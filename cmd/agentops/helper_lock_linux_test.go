//go:build linux

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHelperLockSerializesProcessesAndHonorsContext(t *testing.T) {
	stateDir := t.TempDir()
	if err := os.Chmod(stateDir, 0o700); err != nil {
		t.Fatalf("secure state directory: %v", err)
	}
	expectedUID := uint32(os.Geteuid())
	firstRelease, err := acquireHelperLockOwnedBy(t.Context(), stateDir, expectedUID)
	if err != nil {
		t.Fatalf("acquire first helper lock: %v", err)
	}

	waitContext, cancel := context.WithTimeout(t.Context(), 75*time.Millisecond)
	defer cancel()
	if _, err := acquireHelperLockOwnedBy(waitContext, stateDir, expectedUID); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("contended helper lock error = %v", err)
	}
	if err := firstRelease(); err != nil {
		t.Fatalf("release first helper lock: %v", err)
	}

	release, err := acquireHelperLockOwnedBy(t.Context(), stateDir, expectedUID)
	if err != nil {
		t.Fatalf("reacquire helper lock: %v", err)
	}
	if err := release(); err != nil {
		t.Fatalf("release reacquired helper lock: %v", err)
	}
}

func TestHelperLockDoesNotAcquireAfterCancellation(t *testing.T) {
	stateDir := t.TempDir()
	if err := os.Chmod(stateDir, 0o700); err != nil {
		t.Fatalf("secure state directory: %v", err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := acquireHelperLockOwnedBy(cancelled, stateDir, uint32(os.Geteuid())); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled helper lock error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(stateDir, helperLockName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled helper lock created state: %v", err)
	}
}

func TestHelperLockRejectsSymlinkAndPermissiveState(t *testing.T) {
	expectedUID := uint32(os.Geteuid())

	t.Run("symlink lock", func(t *testing.T) {
		stateDir := t.TempDir()
		if err := os.Chmod(stateDir, 0o700); err != nil {
			t.Fatalf("secure state directory: %v", err)
		}
		target := filepath.Join(t.TempDir(), "target")
		if err := os.WriteFile(target, nil, 0o600); err != nil {
			t.Fatalf("create target: %v", err)
		}
		if err := os.Symlink(target, filepath.Join(stateDir, helperLockName)); err != nil {
			t.Fatalf("create lock symlink: %v", err)
		}
		if _, err := acquireHelperLockOwnedBy(t.Context(), stateDir, expectedUID); err == nil {
			t.Fatal("helper lock accepted a symbolic link")
		}
	})

	t.Run("permissive directory", func(t *testing.T) {
		stateDir := t.TempDir()
		if err := os.Chmod(stateDir, 0o755); err != nil {
			t.Fatalf("make state directory permissive: %v", err)
		}
		if _, err := acquireHelperLockOwnedBy(t.Context(), stateDir, expectedUID); err == nil {
			t.Fatal("helper lock accepted a group-readable state directory")
		}
	})
}
