package keyman

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"ncryptagent/ncrypt"
	"os"
	"os/exec"
	"syscall"
)

const (
	ncryptSignerWorkerArgument = "--ncrypt-signer-worker"
	workerOperationSign        = "sign"
	workerOperationDelete      = "delete"
)

type signerWorkerRequest struct {
	Operation     string
	ProviderName  string
	ContainerName string
	Digest        []byte
	HashAlgorithm string
	WindowHandle  uint64
}

type signerWorkerResponse struct {
	Signature []byte
	Error     string
}

type signerWorker interface {
	Sign(digest []byte, hashAlgorithm string, hwnd uintptr) ([]byte, error)
	Delete(hwnd uintptr) error
	Close() error
}

type signerWorkerFactory func(providerName string, containerName string) (signerWorker, error)

type processSignerWorker struct {
	providerName  string
	containerName string
	command       *exec.Cmd
	stdin         io.WriteCloser
	stdout        io.ReadCloser
	encoder       *gob.Encoder
	decoder       *gob.Decoder
	closed        bool
}

func NCryptSignerWorkerRequested(args []string) bool {
	return len(args) == 2 && args[1] == ncryptSignerWorkerArgument
}

func newProcessSignerWorker(providerName string, containerName string) (signerWorker, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("unable to locate nCryptAgent executable: %w", err)
	}

	command := exec.Command(executable, ncryptSignerWorkerArgument)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("unable to create signer worker input pipe: %w", err)
	}

	stdout, err := command.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("unable to create signer worker output pipe: %w", err)
	}

	if err := command.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("unable to start signer worker: %w", err)
	}

	return &processSignerWorker{
		providerName:  providerName,
		containerName: containerName,
		command:       command,
		stdin:         stdin,
		stdout:        stdout,
		encoder:       gob.NewEncoder(stdin),
		decoder:       gob.NewDecoder(stdout),
	}, nil
}

func (w *processSignerWorker) Sign(digest []byte, hashAlgorithm string, hwnd uintptr) ([]byte, error) {
	response, err := w.transact(signerWorkerRequest{
		Operation:     workerOperationSign,
		ProviderName:  w.providerName,
		ContainerName: w.containerName,
		Digest:        digest,
		HashAlgorithm: hashAlgorithm,
		WindowHandle:  uint64(hwnd),
	})
	if err != nil {
		return nil, err
	}

	return response.Signature, nil
}

func (w *processSignerWorker) Delete(hwnd uintptr) error {
	_, err := w.transact(signerWorkerRequest{
		Operation:     workerOperationDelete,
		ProviderName:  w.providerName,
		ContainerName: w.containerName,
		WindowHandle:  uint64(hwnd),
	})
	return err
}

func (w *processSignerWorker) transact(request signerWorkerRequest) (*signerWorkerResponse, error) {
	if w.closed {
		return nil, fmt.Errorf("NCrypt signer worker is closed")
	}

	if err := w.encoder.Encode(&request); err != nil {
		return nil, fmt.Errorf("unable to send request to NCrypt signer worker: %w", err)
	}

	var response signerWorkerResponse
	if err := w.decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("unable to read response from NCrypt signer worker: %w", err)
	}
	if response.Error != "" {
		return nil, errors.New(response.Error)
	}

	return &response, nil
}

func (w *processSignerWorker) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	closeErr := w.stdin.Close()
	waitErr := w.command.Wait()
	_ = w.stdout.Close()

	if waitErr != nil {
		return fmt.Errorf("NCrypt signer worker did not exit cleanly: %w", waitErr)
	}
	if closeErr != nil {
		return fmt.Errorf("unable to close NCrypt signer worker input: %w", closeErr)
	}

	return nil
}

type ncryptSignerWorkerState struct {
	providerName   string
	containerName  string
	providerHandle uintptr
	keyHandle      uintptr
	windowHandle   uintptr
}

func RunNCryptSignerWorker(input io.Reader, output io.Writer) error {
	decoder := gob.NewDecoder(input)
	encoder := gob.NewEncoder(output)
	state := &ncryptSignerWorkerState{}
	defer state.close()

	for {
		var request signerWorkerRequest
		if err := decoder.Decode(&request); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("unable to decode signer worker request: %w", err)
		}

		response := state.handle(request)
		if err := encoder.Encode(&response); err != nil {
			return fmt.Errorf("unable to encode signer worker response: %w", err)
		}
	}
}

func (s *ncryptSignerWorkerState) handle(request signerWorkerRequest) signerWorkerResponse {
	if err := s.ensureKey(request.ProviderName, request.ContainerName, uintptr(request.WindowHandle)); err != nil {
		return signerWorkerResponse{Error: err.Error()}
	}

	switch request.Operation {
	case workerOperationSign:
		signature, err := ncrypt.NCryptSignHash(s.keyHandle, request.Digest, request.HashAlgorithm)
		if err != nil {
			return signerWorkerResponse{Error: err.Error()}
		}
		return signerWorkerResponse{Signature: signature}

	case workerOperationDelete:
		if err := ncrypt.NCryptDeleteKey(s.keyHandle, 0); err != nil {
			return signerWorkerResponse{Error: err.Error()}
		}
		s.keyHandle = 0
		return signerWorkerResponse{}

	default:
		return signerWorkerResponse{Error: fmt.Sprintf("unsupported signer worker operation %q", request.Operation)}
	}
}

func (s *ncryptSignerWorkerState) ensureKey(providerName string, containerName string, hwnd uintptr) error {
	if providerName == "" {
		return fmt.Errorf("NCrypt provider name is empty")
	}
	if containerName == "" {
		return fmt.Errorf("NCrypt container name is empty")
	}
	if s.providerName != "" && (s.providerName != providerName || s.containerName != containerName) {
		return fmt.Errorf("signer worker cannot switch NCrypt keys")
	}

	if s.providerHandle == 0 {
		providerHandle, err := ncrypt.NCryptOpenStorageProvider(providerName)
		if err != nil {
			return fmt.Errorf("unable to open NCrypt provider %q: %w", providerName, err)
		}
		s.providerName = providerName
		s.containerName = containerName
		s.providerHandle = providerHandle
	}

	if s.keyHandle == 0 {
		keyHandle, err := ncrypt.NCryptOpenKey(s.providerHandle, containerName, 0, 0)
		if err != nil {
			return fmt.Errorf("unable to open NCrypt key %q: %w", containerName, err)
		}
		s.keyHandle = keyHandle
	}

	if s.windowHandle != hwnd {
		if err := ncrypt.NCryptSetProperty(s.keyHandle, ncrypt.NCRYPT_WINDOW_HANDLE_PROPERTY, hwnd, 0); err != nil {
			return fmt.Errorf("unable to set NCrypt window handle: %w", err)
		}
		s.windowHandle = hwnd
	}

	return nil
}

func (s *ncryptSignerWorkerState) close() {
	if s.keyHandle != 0 {
		_ = ncrypt.NCryptFreeObject(s.keyHandle)
		s.keyHandle = 0
	}
	if s.providerHandle != 0 {
		_ = ncrypt.NCryptFreeObject(s.providerHandle)
		s.providerHandle = 0
	}
}
