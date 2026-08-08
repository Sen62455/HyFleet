//go:build !windows

package nodeops

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func replaceHelperFile(source, destination string) error {
	info, err := os.Stat(destination)
	if err != nil {
		return err
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		if err := os.Chown(source, int(stat.Uid), int(stat.Gid)); err != nil {
			return fmt.Errorf("preserve configuration ownership: %w", err)
		}
	}
	return os.Rename(source, destination)
}

func replaceHelperDirectory(source, destination string) error {
	info, err := os.Stat(destination)
	if err != nil {
		return fmt.Errorf("inspect rollback destination directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("rollback destination is not a directory")
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		if err := os.Chown(source, int(stat.Uid), int(stat.Gid)); err != nil {
			return fmt.Errorf("preserve configuration directory ownership: %w", err)
		}
	}
	displaced, err := os.MkdirTemp(filepath.Dir(destination), ".hyfleet-previous-*")
	if err != nil {
		return fmt.Errorf("reserve previous configuration path: %w", err)
	}
	if err := os.Remove(displaced); err != nil {
		return fmt.Errorf("prepare previous configuration path: %w", err)
	}
	if err := os.Rename(destination, displaced); err != nil {
		return fmt.Errorf("preserve previous configuration directory: %w", err)
	}
	if err := os.Rename(source, destination); err != nil {
		_ = os.Rename(displaced, destination)
		return fmt.Errorf("activate restored configuration directory: %w", err)
	}
	if err := os.RemoveAll(displaced); err != nil {
		return fmt.Errorf("remove replaced configuration directory: %w", err)
	}
	return nil
}
