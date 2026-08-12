//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
)

func openHelperConnection(ctx context.Context, file *os.File) (io.ReadWriteCloser, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return nil, errors.New("operations helper input requires a deadline")
	}
	connection, err := net.FileConn(file)
	if err != nil {
		return nil, fmt.Errorf("open operations helper socket input: %w", err)
	}
	if err := connection.SetDeadline(deadline); err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("set operations helper socket deadline: %w", err)
	}
	return connection, nil
}
