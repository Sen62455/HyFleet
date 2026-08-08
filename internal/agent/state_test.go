package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
)

func TestStatePersistsAcrossRestartAndReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "agent-state.json")
	state, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState(missing) error = %v", err)
	}
	if _, err := uuid.Parse(state.InstallationID); err != nil {
		t.Fatalf("installation ID %q is not a UUID: %v", state.InstallationID, err)
	}
	state.NodeID = uuid.NewString()
	state.NodeCredential = "hya_test.secret"
	state.AppliedVersion = 1
	if err := SaveState(path, state); err != nil {
		t.Fatalf("SaveState(first) error = %v", err)
	}

	state.AppliedVersion = 2
	state.PendingAckVersion = 2
	state.PendingAckHash = "snapshot-hash"
	if err := SaveState(path, state); err != nil {
		t.Fatalf("SaveState(replace) error = %v", err)
	}
	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState(saved) error = %v", err)
	}
	if loaded.InstallationID != state.InstallationID || loaded.NodeCredential != state.NodeCredential ||
		loaded.AppliedVersion != 2 || loaded.PendingAckVersion != 2 {
		t.Fatalf("loaded state = %#v, want %#v", loaded, state)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("os.Stat() error = %v", err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("state mode = %o, want 600", info.Mode().Perm())
		}
	}
}

func TestLoadStateRejectsMissingInstallationID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-state.json")
	if err := os.WriteFile(path, []byte(`{"node_id":"test"}`), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if _, err := LoadState(path); err == nil {
		t.Fatal("LoadState() accepted state without installation ID")
	}
}
