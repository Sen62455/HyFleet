package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	_ "modernc.org/sqlite"
)

func TestBackupDatabaseCapturesWALAndValidatesCopy(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.db")
	backupPath := filepath.Join(root, "backup", "server.db")
	database, err := Open(ctx, sourcePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	if _, err := database.DB().ExecContext(ctx, `
		CREATE TABLE backup_probe (id INTEGER PRIMARY KEY, value TEXT NOT NULL);
		INSERT INTO backup_probe(value) VALUES ('before-backup');
	`); err != nil {
		t.Fatalf("seed backup probe: %v", err)
	}
	if err := BackupDatabase(ctx, sourcePath, backupPath); err != nil {
		t.Fatalf("BackupDatabase() error = %v", err)
	}
	if err := CheckDatabase(ctx, backupPath); err != nil {
		t.Fatalf("CheckDatabase() error = %v", err)
	}
	if _, err := database.DB().ExecContext(
		ctx, "INSERT INTO backup_probe(value) VALUES ('after-backup')",
	); err != nil {
		t.Fatalf("insert after backup: %v", err)
	}
	backup, err := sql.Open("sqlite", backupPath)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer backup.Close()
	var count int
	if err := backup.QueryRowContext(ctx, "SELECT COUNT(*) FROM backup_probe").Scan(&count); err != nil {
		t.Fatalf("read backup probe: %v", err)
	}
	if count != 1 {
		t.Fatalf("backup probe count = %d, want 1", count)
	}
	info, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("stat backup: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("backup permissions = %o, want no group/world bits", info.Mode().Perm())
	}
}

func TestBackupDatabaseRejectsExistingDestinationAndInvalidDatabase(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.db")
	destinationPath := filepath.Join(root, "destination.db")
	database, err := Open(ctx, sourcePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := os.WriteFile(destinationPath, []byte("keep"), 0o600); err != nil {
		t.Fatalf("write destination: %v", err)
	}
	if err := BackupDatabase(ctx, sourcePath, destinationPath); err == nil {
		t.Fatal("BackupDatabase() overwrote an existing destination")
	}
	invalidPath := filepath.Join(root, "invalid.db")
	if err := os.WriteFile(invalidPath, []byte("not sqlite"), 0o600); err != nil {
		t.Fatalf("write invalid database: %v", err)
	}
	if err := CheckDatabase(ctx, invalidPath); err == nil {
		t.Fatal("CheckDatabase() accepted invalid data")
	}
}
