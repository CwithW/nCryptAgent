package keyman

import (
	"crypto"
	"crypto/rsa"
	"fmt"
	"golang.org/x/crypto/cryptobyte"
	"golang.org/x/crypto/cryptobyte/asn1"
	"io"
	"log"
	"math/big"
	"ncryptagent/ncrypt"
	"sync"
	"time"
)

type Signer struct {
	algorithmGroup string
	publicKey      crypto.PublicKey
	providerName   string
	containerName  string

	mu              sync.Mutex
	timeout         time.Duration
	timer           *time.Timer
	timerGeneration uint64
	pinCacheActive  bool
	hwnd            uintptr
	closed          bool
	worker          signerWorker
	workerFactory   signerWorkerFactory
}

func newNCryptSigner(kh uintptr, timeout int, providerName string, containerName string) (crypto.Signer, error) {
	pub, err := getPublicKey(kh)
	if err != nil {
		return nil, fmt.Errorf("unable to get public key: %w", err)
	}

	algGroup, err := ncrypt.NCryptGetPropertyStr(kh, ncrypt.NCRYPT_ALGORITHM_GROUP_PROPERTY)
	if err != nil {
		return nil, fmt.Errorf("unable to get NCRYPT_ALGORITHM_GROUP_PROPERTY: %w", err)
	}

	return newSigner(
		algGroup,
		pub,
		providerName,
		containerName,
		time.Duration(timeout)*time.Second,
		newProcessSignerWorker,
	), nil
}

func newSigner(
	algorithmGroup string,
	publicKey crypto.PublicKey,
	providerName string,
	containerName string,
	timeout time.Duration,
	workerFactory signerWorkerFactory,
) *Signer {
	return &Signer{
		algorithmGroup: algorithmGroup,
		publicKey:      publicKey,
		providerName:   providerName,
		containerName:  containerName,
		timeout:        timeout,
		workerFactory:  workerFactory,
	}
}

func (s *Signer) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	if _, isRSAPSS := opts.(*rsa.PSSOptions); isRSAPSS {
		return nil, fmt.Errorf("RSA-PSS signing is not supported")
	}

	var hashAlgorithm string
	switch s.algorithmGroup {
	case "ECDSA":
	case "RSA":
		hashFunction := opts.HashFunc()
		var ok bool
		hashAlgorithm, ok = ncrypt.HashAlgorithms[hashFunction]
		if !ok {
			return nil, fmt.Errorf("unsupported RSA hash algorithm %v", hashFunction)
		}
	default:
		return nil, fmt.Errorf("unsupported algorithm group %v", s.algorithmGroup)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureWorkerLocked(); err != nil {
		return nil, err
	}

	signatureBytes, err := s.worker.Sign(digest, hashAlgorithm, s.hwnd)
	if err != nil {
		s.pinCacheActive = false
		if closeErr := s.stopWorkerLocked(); closeErr != nil {
			log.Printf("Unable to stop failed NCrypt signer subprocess: %v\n", closeErr)
		}
		return nil, fmt.Errorf("NCryptSignHash failed: %w", err)
	}

	if s.algorithmGroup == "ECDSA" {
		if len(signatureBytes) < len(digest)*2 {
			s.pinCacheActive = false
			if closeErr := s.stopWorkerLocked(); closeErr != nil {
				log.Printf("Unable to stop NCrypt signer subprocess after malformed signature: %v\n", closeErr)
			}
			return nil, fmt.Errorf("signatureBytes not long enough to encode ASN signature")
		}

		sigR := signatureBytes[:len(digest)]
		sigS := signatureBytes[len(digest):]

		var builder cryptobyte.Builder
		builder.AddASN1(asn1.SEQUENCE, func(builder *cryptobyte.Builder) {
			builder.AddASN1BigInt(new(big.Int).SetBytes(sigR))
			builder.AddASN1BigInt(new(big.Int).SetBytes(sigS))
		})
		signatureBytes, err = builder.Bytes()
		if err != nil {
			s.pinCacheActive = false
			if closeErr := s.stopWorkerLocked(); closeErr != nil {
				log.Printf("Unable to stop NCrypt signer subprocess after signature encoding failure: %v\n", closeErr)
			}
			return nil, err
		}
	}

	s.handlePINCacheAfterSignLocked()
	return signatureBytes, nil
}

func (s *Signer) ensureWorkerLocked() error {
	if s.closed {
		return fmt.Errorf("NCrypt signer is closed")
	}
	if s.worker != nil {
		return nil
	}
	if s.workerFactory == nil {
		return fmt.Errorf("NCrypt signer worker factory is unavailable")
	}

	worker, err := s.workerFactory(s.providerName, s.containerName)
	if err != nil {
		return fmt.Errorf("unable to start NCrypt signer subprocess: %w", err)
	}
	s.worker = worker
	return nil
}

func (s *Signer) handlePINCacheAfterSignLocked() {
	s.cancelPINCacheTimerLocked()

	switch {
	case s.timeout < 0:
		s.pinCacheActive = true
		log.Printf("PIN cache timeout is permanent; keeping NCrypt signer subprocess running\n")
	case s.timeout == 0:
		s.pinCacheActive = false
		if err := s.stopWorkerLocked(); err != nil {
			log.Printf("Unable to stop NCrypt signer subprocess after signing: %v\n", err)
		}
	default:
		s.pinCacheActive = true
		generation := s.timerGeneration
		log.Printf("Starting PIN cache expiry timer: %s\n", s.timeout)
		s.timer = time.AfterFunc(s.timeout, func() {
			s.expirePINCache(generation)
		})
	}
}

func (s *Signer) expirePINCache(generation uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed || generation != s.timerGeneration {
		return
	}

	s.timer = nil
	s.pinCacheActive = false
	if err := s.stopWorkerLocked(); err != nil {
		log.Printf("Unable to stop NCrypt signer subprocess after PIN cache expiry: %v\n", err)
		return
	}

	log.Printf("PIN cache expired; NCrypt signer subprocess stopped\n")
}

func (s *Signer) cancelPINCacheTimerLocked() {
	s.timerGeneration++
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}

func (s *Signer) stopWorkerLocked() error {
	if s.worker == nil {
		return nil
	}

	worker := s.worker
	s.worker = nil
	return worker.Close()
}

func (s *Signer) Public() crypto.PublicKey {
	return s.publicKey
}

func (s *Signer) PINCacheActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pinCacheActive
}

func (s *Signer) SetPINTimeout(timeout int) {
	s.setPINTimeout(time.Duration(timeout) * time.Second)
}

func (s *Signer) setPINTimeout(timeout time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cancelPINCacheTimerLocked()
	s.timeout = timeout

	if !s.pinCacheActive {
		return
	}

	switch {
	case s.timeout < 0:
		return
	case s.timeout == 0:
		s.pinCacheActive = false
		if err := s.stopWorkerLocked(); err != nil {
			log.Printf("Unable to stop NCrypt signer subprocess while disabling PIN cache: %v\n", err)
		}
	default:
		generation := s.timerGeneration
		s.timer = time.AfterFunc(s.timeout, func() {
			s.expirePINCache(generation)
		})
	}
}

func (s *Signer) SetHWND(hwnd uintptr) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hwnd = hwnd
	return nil
}

func (s *Signer) DeleteKey() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cancelPINCacheTimerLocked()
	if err := s.ensureWorkerLocked(); err != nil {
		return err
	}

	deleteErr := s.worker.Delete(s.hwnd)
	s.pinCacheActive = false
	closeErr := s.stopWorkerLocked()
	if deleteErr != nil {
		return deleteErr
	}
	if closeErr != nil {
		log.Printf("Unable to stop NCrypt signer subprocess after deleting key: %v\n", closeErr)
	}

	s.closed = true
	return nil
}

func (s *Signer) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	s.cancelPINCacheTimerLocked()
	s.pinCacheActive = false
	if err := s.stopWorkerLocked(); err != nil {
		log.Printf("Unable to close NCrypt signer subprocess: %v\n", err)
	}
	s.closed = true
}
