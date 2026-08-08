//go:build !windows

package hy2migration

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func applyOwnership(path string, info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if err := os.Chown(path, int(stat.Uid), int(stat.Gid)); err != nil {
		return fmt.Errorf("preserve config ownership: %w", err)
	}
	return nil
}

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}

func syncParentDirectory(path string) error {
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("open parent directory for sync: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync parent directory: %w", err)
	}
	return nil
}
