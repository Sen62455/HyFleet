//go:build !linux

package agent

import (
	"context"
	"runtime"
	"time"

	"github.com/hyfleet/hyfleet/internal/protocol"
)

type genericCollector struct {
	started time.Time
}

func NewCollector() Collector {
	return &genericCollector{started: time.Now()}
}

func (collector *genericCollector) Facts() HostFacts {
	return HostFacts{OS: runtime.GOOS, Architecture: runtime.GOARCH}
}

func (collector *genericCollector) Sample(_ context.Context) (protocol.HostMetrics, error) {
	return protocol.HostMetrics{
		UptimeSeconds: int64(time.Since(collector.started).Seconds()),
	}, nil
}

func (collector *genericCollector) ServiceRunning(_ context.Context, _ string) bool {
	return false
}
