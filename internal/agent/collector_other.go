//go:build !linux

package agent

import (
	"context"
	"os"
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
	hostname, _ := os.Hostname()
	return HostFacts{OS: runtime.GOOS, Architecture: runtime.GOARCH, Hostname: hostname, CPUCores: runtime.NumCPU()}
}

func (collector *genericCollector) Sample(_ context.Context) (protocol.HostMetrics, error) {
	return protocol.HostMetrics{
		Hostname: collector.Facts().Hostname, CPUCores: runtime.NumCPU(),
		UptimeSeconds: int64(time.Since(collector.started).Seconds()),
	}, nil
}

func (collector *genericCollector) ServiceRunning(_ context.Context, _ string) bool {
	return false
}
