//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

const helperLockName = "helper.lock"

func acquireHelperLock(ctx context.Context, stateDir string) (func() error, error) {
	return acquireHelperLockOwnedBy(ctx, stateDir, 0)
}

func acquireHelperLockOwnedBy(
	ctx context.Context,
	stateDir string,
	expectedUID uint32,
) (func() error, error) {
	if stateDir == "" || !filepath.IsAbs(stateDir) || filepath.Clean(stateDir) != stateDir {
		return nil, errors.New("operations state directory is invalid")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("wait for operations helper lock: %w", err)
	}
	directoryFD, err := unix.Open(
		stateDir,
		unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("open operations state directory: %w", err)
	}
	defer unix.Close(directoryFD)

	var directoryStat unix.Stat_t
	if err := unix.Fstat(directoryFD, &directoryStat); err != nil {
		return nil, fmt.Errorf("inspect operations state directory: %w", err)
	}
	if directoryStat.Mode&unix.S_IFMT != unix.S_IFDIR ||
		directoryStat.Uid != expectedUID || directoryStat.Mode&0o077 != 0 {
		return nil, errors.New("operations state directory is not owner-only")
	}

	lockFD, err := unix.Openat(
		directoryFD,
		helperLockName,
		unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW,
		0o600,
	)
	if err != nil {
		return nil, fmt.Errorf("open operations helper lock: %w", err)
	}
	closeLock := func() { _ = unix.Close(lockFD) }

	var lockStat unix.Stat_t
	if err := unix.Fstat(lockFD, &lockStat); err != nil {
		closeLock()
		return nil, fmt.Errorf("inspect operations helper lock: %w", err)
	}
	if lockStat.Mode&unix.S_IFMT != unix.S_IFREG || lockStat.Nlink != 1 ||
		lockStat.Uid != expectedUID || lockStat.Mode&0o077 != 0 {
		closeLock()
		return nil, errors.New("operations helper lock is not an owner-only regular file")
	}

	for {
		if err := ctx.Err(); err != nil {
			closeLock()
			return nil, fmt.Errorf("wait for operations helper lock: %w", err)
		}
		err = unix.Flock(lockFD, unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			closeLock()
			return nil, fmt.Errorf("lock operations helper: %w", err)
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			closeLock()
			return nil, fmt.Errorf("wait for operations helper lock: %w", ctx.Err())
		case <-timer.C:
		}
	}

	return func() error {
		unlockErr := unix.Flock(lockFD, unix.LOCK_UN)
		closeErr := unix.Close(lockFD)
		return errors.Join(unlockErr, closeErr)
	}, nil
}
