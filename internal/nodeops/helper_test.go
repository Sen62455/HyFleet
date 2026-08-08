package nodeops

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hyfleet/hyfleet/internal/protocol"
)

func testHelper(t *testing.T, configBody string) (*Helper, string) {
	t.Helper()
	root := t.TempDir()
	configPath := filepath.Join(root, "etc", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	return &Helper{
		ServiceUnit: "hysteria-server.service", CoreConfigPath: configPath,
		BackupDir: filepath.Join(root, "backups"), LedgerDir: filepath.Join(root, "ledger"),
		Now: func() time.Time { return now },
	}, configPath
}

func testDirectoryHelper(t *testing.T) (*Helper, string) {
	t.Helper()
	root := t.TempDir()
	configPath := filepath.Join(root, "etc", "sing-box", "conf")
	if err := os.MkdirAll(filepath.Join(configPath, "nested"), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	files := map[string]string{
		"00_log.json":                "{\"log\":{\"level\":\"info\"}}\n",
		"12_hysteria2_inbounds.json": "{\"inbounds\":[{\"type\":\"hysteria2\"}]}\n",
		"nested/route.json":          "{\"route\":{}}\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(configPath, filepath.FromSlash(name)), []byte(body), 0o640); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	return &Helper{
		ServiceUnit: "sing-box.service", CoreConfigPath: configPath,
		BackupDir: filepath.Join(root, "backups"), LedgerDir: filepath.Join(root, "ledger"),
		Now: func() time.Time { return now },
	}, configPath
}

func TestNewHelperRejectsOptionLikeUnitAndUnsupportedConfigPath(t *testing.T) {
	if _, err := NewHelper("--help", "/etc/hysteria/config.yaml"); err == nil {
		t.Fatal("NewHelper() accepted an option-like service unit")
	}
	if _, err := NewHelper("hysteria-server.service", "/etc/other/config.yaml"); err == nil {
		t.Fatal("NewHelper() accepted a config path outside writable systemd directories")
	}
	if _, err := NewHelper("sing-box.service", "/etc/sing-box/config.json"); err != nil {
		t.Fatalf("NewHelper() rejected supported sing-box config: %v", err)
	}
}

func TestHelperRestartFailureRestoresPreviousBackupAndIsIdempotent(t *testing.T) {
	helper, configPath := testHelper(t, "known-good: true\n")
	backupOperation := protocol.NodeOperation{
		ID: uuid.NewString(), Sequence: 1, Type: "backup_config", Attempt: 1,
	}
	backup := helper.Handle(t.Context(), HelperRequest{Operation: backupOperation})
	if backup.Status != "succeeded" || backup.Backup == nil {
		t.Fatalf("initial backup response = %#v", backup)
	}
	if err := os.WriteFile(configPath, []byte("broken: true\n"), 0o600); err != nil {
		t.Fatalf("write broken config: %v", err)
	}
	commands := 0
	helper.RunCommand = func(_ context.Context, name string, arguments ...string) ([]byte, error) {
		commands++
		if name != "systemctl" {
			t.Fatalf("unexpected command: %s %v", name, arguments)
		}
		switch commands {
		case 1:
			return []byte("restart rejected"), errors.New("exit status 1")
		case 2:
			return []byte("inactive"), errors.New("exit status 3")
		case 3:
			return nil, nil
		case 4:
			return []byte("active"), nil
		default:
			t.Fatalf("unexpected command count %d", commands)
			return nil, nil
		}
	}
	restartOperation := protocol.NodeOperation{
		ID: uuid.NewString(), Sequence: 2, Type: "restart_core", Attempt: 1,
	}
	result := helper.Handle(t.Context(), HelperRequest{Operation: restartOperation})
	if result.Status != "failed" || result.ErrorCode != "core_restart_failed" || !result.RolledBack {
		t.Fatalf("restart result = %#v", result)
	}
	restored, err := os.ReadFile(configPath)
	if err != nil || string(restored) != "known-good: true\n" {
		t.Fatalf("restored config = %q, error = %v", restored, err)
	}
	replayed := helper.Handle(t.Context(), HelperRequest{Operation: restartOperation})
	if commands != 4 || replayed.Status != result.Status || !replayed.RolledBack {
		t.Fatalf("helper replay executed side effect: commands=%d replay=%#v", commands, replayed)
	}
}

func TestHelperDirectoryBackupAndRestartRollback(t *testing.T) {
	helper, configPath := testDirectoryHelper(t)
	backupOperation := protocol.NodeOperation{
		ID: uuid.NewString(), Sequence: 1, Type: "backup_config", Attempt: 1,
	}
	backup := helper.Handle(t.Context(), HelperRequest{Operation: backupOperation})
	if backup.Status != "succeeded" || backup.Backup == nil ||
		!strings.HasSuffix(backup.Backup.LocalPath, "-conf.tar.gz") || backup.Backup.SizeBytes < 1 {
		t.Fatalf("directory backup response = %#v", backup)
	}
	if err := os.WriteFile(
		filepath.Join(configPath, "12_hysteria2_inbounds.json"),
		[]byte("{\"broken\":true}\n"), 0o640,
	); err != nil {
		t.Fatalf("write broken config: %v", err)
	}
	if err := os.Remove(filepath.Join(configPath, "00_log.json")); err != nil {
		t.Fatalf("remove known config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configPath, "unexpected.json"), []byte("{}\n"), 0o640); err != nil {
		t.Fatalf("write unexpected config: %v", err)
	}
	commands := 0
	helper.RunCommand = func(_ context.Context, name string, arguments ...string) ([]byte, error) {
		commands++
		if name != "systemctl" {
			t.Fatalf("unexpected command: %s %v", name, arguments)
		}
		switch commands {
		case 1:
			return []byte("restart rejected"), errors.New("exit status 1")
		case 2:
			return []byte("inactive"), errors.New("exit status 3")
		case 3:
			return nil, nil
		case 4:
			return []byte("active"), nil
		default:
			t.Fatalf("unexpected command count %d", commands)
			return nil, nil
		}
	}
	restartOperation := protocol.NodeOperation{
		ID: uuid.NewString(), Sequence: 2, Type: "restart_core", Attempt: 1,
	}
	result := helper.Handle(t.Context(), HelperRequest{Operation: restartOperation})
	if result.Status != "failed" || !result.RolledBack || result.Backup == nil {
		t.Fatalf("directory restart result = %#v", result)
	}
	restored, err := os.ReadFile(filepath.Join(configPath, "12_hysteria2_inbounds.json"))
	if err != nil || string(restored) != "{\"inbounds\":[{\"type\":\"hysteria2\"}]}\n" {
		t.Fatalf("restored directory config = %q, error = %v", restored, err)
	}
	if _, err := os.Stat(filepath.Join(configPath, "00_log.json")); err != nil {
		t.Fatalf("removed configuration was not restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configPath, "unexpected.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected configuration survived rollback: %v", err)
	}
}

func TestHelperDirectoryBackupRejectsSymlinksAndOversizedTrees(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		helper, configPath := testDirectoryHelper(t)
		outsidePath := filepath.Join(t.TempDir(), "outside.json")
		if err := os.WriteFile(outsidePath, []byte("secret\n"), 0o600); err != nil {
			t.Fatalf("write outside file: %v", err)
		}
		if err := os.Symlink(outsidePath, filepath.Join(configPath, "linked.json")); err != nil {
			t.Skipf("symlink is unavailable on this platform: %v", err)
		}
		operation := protocol.NodeOperation{
			ID: uuid.NewString(), Sequence: 1, Type: "backup_config", Attempt: 1,
		}
		result := helper.Handle(t.Context(), HelperRequest{Operation: operation})
		if result.Status != "failed" || result.ErrorCode != "config_backup_failed" ||
			!strings.Contains(result.ErrorMessage, "symbolic link") {
			t.Fatalf("symlink backup response = %#v", result)
		}
	})

	t.Run("size", func(t *testing.T) {
		helper, configPath := testDirectoryHelper(t)
		oversized, err := os.OpenFile(
			filepath.Join(configPath, "oversized.json"), os.O_CREATE|os.O_WRONLY, 0o600,
		)
		if err != nil {
			t.Fatalf("create oversized file: %v", err)
		}
		if err := oversized.Truncate(maxConfigBackupBytes + 1); err != nil {
			_ = oversized.Close()
			t.Fatalf("truncate oversized file: %v", err)
		}
		if err := oversized.Close(); err != nil {
			t.Fatalf("close oversized file: %v", err)
		}
		operation := protocol.NodeOperation{
			ID: uuid.NewString(), Sequence: 1, Type: "backup_config", Attempt: 1,
		}
		result := helper.Handle(t.Context(), HelperRequest{Operation: operation})
		if result.Status != "failed" || result.ErrorCode != "config_backup_failed" ||
			!strings.Contains(result.ErrorMessage, "size limit") {
			t.Fatalf("oversized backup response = %#v", result)
		}
	})
}

func TestDirectoryRestoreRejectsArchiveTraversal(t *testing.T) {
	helper, configPath := testDirectoryHelper(t)
	if err := helper.prepareBackupDir(); err != nil {
		t.Fatalf("prepareBackupDir() error = %v", err)
	}
	backupPath := filepath.Join(helper.BackupDir, "malicious-conf.tar.gz")
	archive, err := os.OpenFile(backupPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("create malicious archive: %v", err)
	}
	gzipWriter := gzip.NewWriter(archive)
	tarWriter := tar.NewWriter(gzipWriter)
	body := []byte("escaped\n")
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: "../escaped.json", Mode: 0o600, Size: int64(len(body)), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("write malicious header: %v", err)
	}
	if _, err := tarWriter.Write(body); err != nil {
		t.Fatalf("write malicious body: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close malicious archive: %v", err)
	}
	if err := helper.restoreBackup(backupPath); err == nil {
		t.Fatal("restoreBackup() accepted archive traversal")
	}
	escapedPath := filepath.Join(filepath.Dir(configPath), "escaped.json")
	if _, err := os.Stat(escapedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("archive traversal wrote outside restore directory: %v", err)
	}
}

func TestHelperLogOutputIsBoundedAndRedacted(t *testing.T) {
	helper, _ := testHelper(t, "config: true\n")
	helper.RunCommand = func(_ context.Context, name string, arguments ...string) ([]byte, error) {
		if name != "journalctl" || len(arguments) < 5 {
			t.Fatalf("unexpected log command: %s %v", name, arguments)
		}
		return []byte(
			"authorization: Bearer private-value\npassword=private-password\n" +
				strings.Repeat("bounded log line\n", 250),
		), nil
	}
	operation := protocol.NodeOperation{
		ID: uuid.NewString(), Sequence: 1, Type: "tail_core_log", MaxLines: 100, Attempt: 1,
	}
	result := helper.Handle(t.Context(), HelperRequest{Operation: operation})
	if result.Status != "succeeded" || len(result.Output) > MaxOutputSize ||
		strings.Count(result.Output, "\n") >= 100 ||
		strings.Contains(result.Output, "private-value") ||
		strings.Contains(result.Output, "private-password") {
		t.Fatalf("bounded log result = %#v", result)
	}
}
