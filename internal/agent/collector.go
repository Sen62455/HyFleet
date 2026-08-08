package agent

import (
	"context"
	"time"

	"github.com/hyfleet/hyfleet/internal/protocol"
)

type HostFacts struct {
	OS           string
	OSVersion    string
	Architecture string
}

type Collector interface {
	Facts() HostFacts
	Sample(context.Context) (protocol.HostMetrics, error)
	ServiceRunning(context.Context, string) bool
}

type networkSample struct {
	rxBytes int64
	txBytes int64
	at      time.Time
}
