//go:build !windows

package agent

import "os"

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
