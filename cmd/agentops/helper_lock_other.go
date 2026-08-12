//go:build !linux

package main

import (
	"context"
	"errors"
)

func acquireHelperLock(context.Context, string) (func() error, error) {
	return nil, errors.New("operations helper locking requires Linux")
}
