//go:build !linux

package main

import (
	"context"
	"errors"
	"io"
	"os"
)

func openHelperConnection(context.Context, *os.File) (io.ReadWriteCloser, error) {
	return nil, errors.New("operations helper socket connection requires Linux")
}
