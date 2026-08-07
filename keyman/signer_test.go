package keyman

import (
	"crypto"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeSignerWorker struct {
	mu            sync.Mutex
	signCount     int
	deleteCount   int
	closeCount    int
	windowHandles []uintptr
	signature     []byte
	signErr       error
	closed        chan struct{}
}

func newFakeSignerWorker() *fakeSignerWorker {
	return &fakeSignerWorker{
		signature: []byte{1, 2, 3},
		closed:    make(chan struct{}),
	}
}

func (w *fakeSignerWorker) Sign(_ []byte, _ string, hwnd uintptr) ([]byte, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.signCount++
	w.windowHandles = append(w.windowHandles, hwnd)
	return w.signature, w.signErr
}

func (w *fakeSignerWorker) Delete(hwnd uintptr) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.deleteCount++
	w.windowHandles = append(w.windowHandles, hwnd)
	return nil
}

func (w *fakeSignerWorker) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closeCount == 0 {
		close(w.closed)
	}
	w.closeCount++
	return nil
}

func (w *fakeSignerWorker) counts() (sign, deleteCount, closeCount int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.signCount, w.deleteCount, w.closeCount
}

func (w *fakeSignerWorker) lastWindowHandle() uintptr {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.windowHandles) == 0 {
		return 0
	}
	return w.windowHandles[len(w.windowHandles)-1]
}

type fakeSignerWorkerFactory struct {
	mu      sync.Mutex
	workers []*fakeSignerWorker
	signErr error
}

func (f *fakeSignerWorkerFactory) create(_ string, _ string) (signerWorker, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	worker := newFakeSignerWorker()
	worker.signErr = f.signErr
	f.workers = append(f.workers, worker)
	return worker, nil
}

func (f *fakeSignerWorkerFactory) allWorkers() []*fakeSignerWorker {
	f.mu.Lock()
	defer f.mu.Unlock()
	workers := make([]*fakeSignerWorker, len(f.workers))
	copy(workers, f.workers)
	return workers
}

func newTestSigner(timeout time.Duration, factory *fakeSignerWorkerFactory) *Signer {
	return newSigner("RSA", nil, "provider", "container", timeout, factory.create)
}

func signForTest(t *testing.T, signer *Signer) {
	t.Helper()
	if _, err := signer.Sign(nil, make([]byte, crypto.SHA256.Size()), crypto.SHA256); err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
}

func waitForWorkerClose(t *testing.T, worker *fakeSignerWorker, timeout time.Duration) {
	t.Helper()
	select {
	case <-worker.closed:
	case <-time.After(timeout):
		t.Fatal("signer worker was not closed")
	}
}

func TestZeroTimeoutStopsWorkerAfterEverySignature(t *testing.T) {
	factory := &fakeSignerWorkerFactory{}
	signer := newTestSigner(0, factory)

	signForTest(t, signer)
	signForTest(t, signer)

	workers := factory.allWorkers()
	if len(workers) != 2 {
		t.Fatalf("worker count = %d, want 2", len(workers))
	}
	for index, worker := range workers {
		signCount, _, closeCount := worker.counts()
		if signCount != 1 || closeCount != 1 {
			t.Fatalf("worker %d counts = sign:%d close:%d, want sign:1 close:1", index, signCount, closeCount)
		}
	}
}

func TestPositiveTimeoutResetsOneExpiryTimer(t *testing.T) {
	factory := &fakeSignerWorkerFactory{}
	signer := newTestSigner(100*time.Millisecond, factory)
	defer signer.Close()

	signForTest(t, signer)
	worker := factory.allWorkers()[0]
	time.Sleep(60 * time.Millisecond)
	signForTest(t, signer)

	select {
	case <-worker.closed:
		t.Fatal("worker closed by stale timer")
	case <-time.After(60 * time.Millisecond):
	}

	waitForWorkerClose(t, worker, 100*time.Millisecond)
	signCount, _, closeCount := worker.counts()
	if signCount != 2 || closeCount != 1 {
		t.Fatalf("counts after timer expiry = sign:%d close:%d, want sign:2 close:1", signCount, closeCount)
	}
}

func TestSignerStartsNewWorkerAfterExpiry(t *testing.T) {
	factory := &fakeSignerWorkerFactory{}
	signer := newTestSigner(20*time.Millisecond, factory)
	defer signer.Close()

	if err := signer.SetHWND(42); err != nil {
		t.Fatalf("SetHWND() error = %v", err)
	}
	signForTest(t, signer)
	firstWorker := factory.allWorkers()[0]
	waitForWorkerClose(t, firstWorker, 200*time.Millisecond)

	signForTest(t, signer)
	workers := factory.allWorkers()
	if len(workers) != 2 {
		t.Fatalf("worker count = %d, want 2", len(workers))
	}
	for index, worker := range workers {
		if hwnd := worker.lastWindowHandle(); hwnd != 42 {
			t.Fatalf("worker %d HWND = %d, want 42", index, hwnd)
		}
	}
}

func TestIndefiniteTimeoutKeepsWorkerRunning(t *testing.T) {
	factory := &fakeSignerWorkerFactory{}
	signer := newTestSigner(30*time.Millisecond, factory)

	signForTest(t, signer)
	worker := factory.allWorkers()[0]
	signer.SetPINTimeout(-1)
	time.Sleep(80 * time.Millisecond)

	_, _, closeCount := worker.counts()
	if closeCount != 0 {
		t.Fatalf("close count = %d, want 0 before Close", closeCount)
	}
	if !signer.PINCacheActive() {
		t.Fatal("PIN cache should remain active for timeout -1")
	}

	signer.Close()
	_, _, closeCount = worker.counts()
	if closeCount != 1 {
		t.Fatalf("close count after signer close = %d, want 1", closeCount)
	}
}

func TestCloseStopsTimerAndWorkerOnce(t *testing.T) {
	factory := &fakeSignerWorkerFactory{}
	signer := newTestSigner(20*time.Millisecond, factory)

	signForTest(t, signer)
	worker := factory.allWorkers()[0]
	signer.Close()
	time.Sleep(60 * time.Millisecond)

	_, _, closeCount := worker.counts()
	if closeCount != 1 {
		t.Fatalf("close count = %d, want 1", closeCount)
	}
}

func TestFailedSignatureStopsWorker(t *testing.T) {
	factory := &fakeSignerWorkerFactory{signErr: errors.New("PIN verification failed")}
	signer := newTestSigner(-1, factory)

	if _, err := signer.Sign(nil, make([]byte, crypto.SHA256.Size()), crypto.SHA256); err == nil {
		t.Fatal("Sign() error = nil, want failure")
	}

	worker := factory.allWorkers()[0]
	_, _, closeCount := worker.counts()
	if closeCount != 1 {
		t.Fatalf("close count = %d, want 1 after failed signature", closeCount)
	}
	if signer.PINCacheActive() {
		t.Fatal("PIN cache must not remain active after failed signature")
	}
}

func TestDeleteKeyRunsInWorkerAndStopsIt(t *testing.T) {
	factory := &fakeSignerWorkerFactory{}
	signer := newTestSigner(-1, factory)
	if err := signer.SetHWND(99); err != nil {
		t.Fatalf("SetHWND() error = %v", err)
	}

	if err := signer.DeleteKey(); err != nil {
		t.Fatalf("DeleteKey() error = %v", err)
	}

	worker := factory.allWorkers()[0]
	_, deleteCount, closeCount := worker.counts()
	if deleteCount != 1 || closeCount != 1 {
		t.Fatalf("counts = delete:%d close:%d, want delete:1 close:1", deleteCount, closeCount)
	}
	if hwnd := worker.lastWindowHandle(); hwnd != 99 {
		t.Fatalf("delete HWND = %d, want 99", hwnd)
	}
}

func TestTimeoutTransitionIndefiniteToPositiveStartsTimer(t *testing.T) {
	factory := &fakeSignerWorkerFactory{}
	signer := newTestSigner(-1, factory)
	defer signer.Close()

	signForTest(t, signer)
	worker := factory.allWorkers()[0]
	signer.setPINTimeout(20 * time.Millisecond)

	waitForWorkerClose(t, worker, 200*time.Millisecond)
}

func TestTimeoutTransitionIndefiniteToZeroStopsWorkerImmediately(t *testing.T) {
	factory := &fakeSignerWorkerFactory{}
	signer := newTestSigner(-1, factory)

	signForTest(t, signer)
	worker := factory.allWorkers()[0]
	signer.setPINTimeout(0)

	_, _, closeCount := worker.counts()
	if closeCount != 1 {
		t.Fatalf("close count = %d, want 1", closeCount)
	}
}

func TestTimeoutTransitionPositiveToIndefiniteCancelsTimer(t *testing.T) {
	factory := &fakeSignerWorkerFactory{}
	signer := newTestSigner(20*time.Millisecond, factory)

	signForTest(t, signer)
	worker := factory.allWorkers()[0]
	signer.setPINTimeout(-1)
	time.Sleep(60 * time.Millisecond)

	_, _, closeCount := worker.counts()
	if closeCount != 0 {
		t.Fatalf("close count = %d, want 0 before Close", closeCount)
	}
	signer.Close()
}

func TestTimeoutTransitionPositiveToZeroStopsWorkerImmediately(t *testing.T) {
	factory := &fakeSignerWorkerFactory{}
	signer := newTestSigner(time.Second, factory)

	signForTest(t, signer)
	worker := factory.allWorkers()[0]
	signer.setPINTimeout(0)

	_, _, closeCount := worker.counts()
	if closeCount != 1 {
		t.Fatalf("close count = %d, want 1", closeCount)
	}
}

func TestTimeoutTransitionZeroToPositiveAppliesToNextWorker(t *testing.T) {
	factory := &fakeSignerWorkerFactory{}
	signer := newTestSigner(0, factory)
	defer signer.Close()

	signer.setPINTimeout(20 * time.Millisecond)
	signForTest(t, signer)
	worker := factory.allWorkers()[0]

	waitForWorkerClose(t, worker, 200*time.Millisecond)
}

func TestTimeoutTransitionZeroToIndefiniteAppliesToNextWorker(t *testing.T) {
	factory := &fakeSignerWorkerFactory{}
	signer := newTestSigner(0, factory)

	signer.setPINTimeout(-1)
	signForTest(t, signer)
	worker := factory.allWorkers()[0]
	time.Sleep(40 * time.Millisecond)

	_, _, closeCount := worker.counts()
	if closeCount != 0 {
		t.Fatalf("close count = %d, want 0 before Close", closeCount)
	}
	signer.Close()
}

func TestTimeoutTransitionPositiveToPositiveRestartsTimer(t *testing.T) {
	factory := &fakeSignerWorkerFactory{}
	signer := newTestSigner(100*time.Millisecond, factory)
	defer signer.Close()

	signForTest(t, signer)
	worker := factory.allWorkers()[0]
	time.Sleep(60 * time.Millisecond)
	signer.setPINTimeout(100 * time.Millisecond)

	select {
	case <-worker.closed:
		t.Fatal("worker closed by the old positive timeout")
	case <-time.After(60 * time.Millisecond):
	}
	waitForWorkerClose(t, worker, 100*time.Millisecond)
}
