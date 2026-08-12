//go:build !windows

package nodeops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProductionOwnershipContractsWithFilesystemMetadata(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("root is required to exercise non-root ownership metadata")
	}
	root := t.TempDir()
	parent := filepath.Join(root, "sing-box")
	if err := os.Mkdir(parent, 0o750); err != nil {
		t.Fatalf("Mkdir(parent) error = %v", err)
	}
	config := filepath.Join(parent, "config.json")
	if err := os.WriteFile(config, []byte("{}\n"), 0o640); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	backupDir := filepath.Join(root, "backups")
	if err := os.Mkdir(backupDir, 0o700); err != nil {
		t.Fatalf("Mkdir(backup) error = %v", err)
	}
	backup := filepath.Join(backupDir, "config.bak")
	if err := os.WriteFile(backup, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(backup) error = %v", err)
	}

	parentInfo, _ := os.Lstat(parent)
	configInfo, _ := os.Lstat(config)
	backupDirInfo, _ := os.Lstat(backupDir)
	backupInfo, _ := os.Lstat(backup)
	if !validRealityConfigParentInfo("/etc/sing-box", parentInfo, nil) ||
		!validRealityConfigTargetInfo("/etc/sing-box/hyfleet-reality.json", configInfo) ||
		!validBackupDirectoryInfo("/var/lib/hyfleet-backups", backupDirInfo) ||
		!validBackupFileInfo("/var/lib/hyfleet-backups", backupInfo) {
		t.Fatal("root-owned filesystem metadata did not satisfy production contracts")
	}

	for _, path := range []string{parent, config, backupDir, backup} {
		if err := os.Chown(path, 65534, 65534); err != nil {
			t.Fatalf("Chown(%s) error = %v", path, err)
		}
	}
	parentInfo, _ = os.Lstat(parent)
	configInfo, _ = os.Lstat(config)
	backupDirInfo, _ = os.Lstat(backupDir)
	backupInfo, _ = os.Lstat(backup)
	if validRealityConfigParentInfo("/etc/sing-box", parentInfo, nil) ||
		validRealityConfigTargetInfo("/etc/sing-box/hyfleet-reality.json", configInfo) ||
		validBackupDirectoryInfo("/var/lib/hyfleet-backups", backupDirInfo) ||
		validBackupFileInfo("/var/lib/hyfleet-backups", backupInfo) {
		t.Fatal("non-root filesystem metadata satisfied a production contract")
	}
}
