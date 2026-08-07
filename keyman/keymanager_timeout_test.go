package keyman

import (
	"crypto"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTimeoutTestKeyManager(t *testing.T, configuredTimeout int, signer *Signer) *KeyManager {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.json")
	cryptoSigner := crypto.Signer(signer)
	return &KeyManager{
		Keys: map[string]*Key{
			"test": {
				Name:   "test",
				Type:   "NCRYPT",
				config: &KeyConfig{Name: "test", Type: "NCRYPT"},
				signer: &cryptoSigner,
			},
		},
		configPath: configPath,
		config: &KeyManagerConfig{
			PinTimeout: configuredTimeout,
		},
	}
}

func TestSetPinTimeoutAppliesImmediatelyAndSavesConfig(t *testing.T) {
	factory := &fakeSignerWorkerFactory{}
	signer := newTestSigner(-1, factory)
	signForTest(t, signer)
	worker := factory.allWorkers()[0]
	km := newTimeoutTestKeyManager(t, -1, signer)

	if err := km.SetPinTimeout(0); err != nil {
		t.Fatalf("SetPinTimeout() error = %v", err)
	}

	_, _, closeCount := worker.counts()
	if closeCount != 1 {
		t.Fatalf("close count = %d, want 1 after transition to zero", closeCount)
	}

	contents, err := os.ReadFile(km.configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var savedConfig KeyManagerConfig
	if err := json.Unmarshal(contents, &savedConfig); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if savedConfig.PinTimeout != 0 {
		t.Fatalf("saved PIN timeout = %d, want 0", savedConfig.PinTimeout)
	}
}

func TestTimedPinTimeoutOverrideExpiresInMemory(t *testing.T) {
	factory := &fakeSignerWorkerFactory{}
	signer := newTestSigner(0, factory)
	km := newTimeoutTestKeyManager(t, 0, signer)
	if err := km.SaveConfig(); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	before, err := os.ReadFile(km.configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if err := km.StartPinTimeoutOverride(30 * time.Millisecond); err != nil {
		t.Fatalf("StartPinTimeoutOverride() error = %v", err)
	}
	signForTest(t, signer)
	worker := factory.allWorkers()[0]
	waitForWorkerClose(t, worker, 200*time.Millisecond)

	status := km.GetPinTimeoutOverrideStatus()
	if status.Active {
		t.Fatal("override should be inactive after expiry")
	}
	after, err := os.ReadFile(km.configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(after) != string(before) {
		t.Fatal("temporary override modified the persisted configuration")
	}
}

func TestLifetimePinTimeoutOverrideEndsOnlyWhenCancelled(t *testing.T) {
	factory := &fakeSignerWorkerFactory{}
	signer := newTestSigner(0, factory)
	km := newTimeoutTestKeyManager(t, 0, signer)

	km.StartPinTimeoutOverrideForLifetime()
	signForTest(t, signer)
	worker := factory.allWorkers()[0]
	time.Sleep(50 * time.Millisecond)

	status := km.GetPinTimeoutOverrideStatus()
	if !status.Active || !status.UntilRestart {
		t.Fatalf("lifetime override status = %+v", status)
	}
	_, _, closeCount := worker.counts()
	if closeCount != 0 {
		t.Fatalf("close count = %d, want 0 before cancellation", closeCount)
	}

	km.CancelPinTimeoutOverride()
	_, _, closeCount = worker.counts()
	if closeCount != 1 {
		t.Fatalf("close count = %d, want 1 after cancellation restores timeout 0", closeCount)
	}
}

func TestConfiguredTimeoutChangeDoesNotEndActiveOverride(t *testing.T) {
	factory := &fakeSignerWorkerFactory{}
	signer := newTestSigner(0, factory)
	km := newTimeoutTestKeyManager(t, 0, signer)

	km.StartPinTimeoutOverrideForLifetime()
	signForTest(t, signer)
	worker := factory.allWorkers()[0]
	if err := km.SetPinTimeout(0); err != nil {
		t.Fatalf("SetPinTimeout() error = %v", err)
	}

	_, _, closeCount := worker.counts()
	if closeCount != 0 {
		t.Fatalf("close count = %d, want 0 while override remains active", closeCount)
	}
	km.CancelPinTimeoutOverride()
	_, _, closeCount = worker.counts()
	if closeCount != 1 {
		t.Fatalf("close count = %d, want 1 after restoring configured timeout", closeCount)
	}
}
