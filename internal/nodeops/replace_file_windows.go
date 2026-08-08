//go:build windows

package nodeops

import (
	"fmt"
	"os"
	"path/filepath"
)

func replaceHelperFile(source, destination string) error {
	if err := os.Remove(destination); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(source, destination)
}

func replaceHelperDirectory(source, destination string) error {
	displaced, err := os.MkdirTemp(filepath.Dir(destination), ".hyfleet-previous-*")
	if err != nil {
		return err
	}
	if err := os.Remove(displaced); err != nil {
		return err
	}
	if err := os.Rename(destination, displaced); err != nil {
		return err
	}
	if err := os.Rename(source, destination); err != nil {
		_ = os.Rename(displaced, destination)
		return err
	}
	if err := os.RemoveAll(displaced); err != nil {
		return fmt.Errorf("remove replaced configuration directory: %w", err)
	}
	return nil
}
