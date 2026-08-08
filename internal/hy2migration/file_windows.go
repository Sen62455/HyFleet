//go:build windows

package hy2migration

import (
	"os"

	"golang.org/x/sys/windows"
)

func applyOwnership(_ string, _ os.FileInfo) error { return nil }

func replaceFile(source, destination string) error {
	sourcePath, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	destinationPath, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(
		sourcePath,
		destinationPath,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	)
}

func syncParentDirectory(_ string) error { return nil }
