//go:build !windows

package nodeops

import (
	"fmt"
	"os"
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
