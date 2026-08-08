package nodeops

import (
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
