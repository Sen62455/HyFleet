package cryptoutil

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func LoadOrCreateKey(path string) ([]byte, bool, error) {
	value, err := os.ReadFile(path)
	if err == nil {
		if len(value) != 32 {
			return nil, false, fmt.Errorf("master key %s must contain exactly 32 bytes", path)
		}
		return value, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, false, fmt.Errorf("read master key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, false, fmt.Errorf("create key directory: %w", err)
	}
	value = make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return nil, false, fmt.Errorf("generate master key: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, false, fmt.Errorf("create master key: %w", err)
	}
	if _, err := file.Write(value); err != nil {
		_ = file.Close()
		return nil, false, fmt.Errorf("write master key: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return nil, false, fmt.Errorf("sync master key: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, false, fmt.Errorf("close master key: %w", err)
	}
	return value, true, nil
}
