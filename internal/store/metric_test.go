package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestListNodeMetricSamplesBoundsAggregationAndPruning(t *testing.T) {
	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "metrics.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	now := time.Now().UTC().Truncate(time.Minute)
	node, err := database.CreateNode(t.Context(), NewNode{
		ID: uuid.NewString(), Name: "metric-node", AdapterType: "native_hysteria2",
		Enabled: true, Now: now,
	})
	if err != nil {
		t.Fatalf("CreateNode() error = %v", err)
	}

	start := now.Add(-24 * time.Hour)
	tx, err := database.DB().BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	for sampledAt := start; !sampledAt.After(now); sampledAt = sampledAt.Add(time.Minute) {
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO node_metric_samples(
				node_id, bucket_at, cpu_percent, memory_used_bytes, memory_total_bytes,
				disk_used_bytes, disk_total_bytes, network_rx_bps, network_tx_bps,
				load_1, load_5, load_15, sampled_at, swap_used_bytes, swap_total_bytes,
				disk_read_bytes_per_second, disk_write_bytes_per_second
			) VALUES (?, ?, 25, 256, 1024, 512, 2048, 8000, 4000, 0.2, 0.1, 0.05, ?, 64, 256, 32, 16)
		`, node.ID, sampledAt.UnixMilli(), sampledAt.UnixMilli()); err != nil {
			t.Fatalf("insert metric sample: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	samples, step, err := database.ListNodeMetricSamples(t.Context(), node.ID, start, now, 360)
	if err != nil {
		t.Fatalf("ListNodeMetricSamples() error = %v", err)
	}
	if len(samples) == 0 || len(samples) > 360 {
		t.Fatalf("metric sample count = %d, want 1..360", len(samples))
	}
	if step != 5*time.Minute {
		t.Fatalf("metric aggregation step = %s, want 5m", step)
	}
	for index, sample := range samples {
		if sample.CPUPercent != 25 || sample.MemoryUsedBytes != 256 || sample.SwapUsedBytes != 64 {
			t.Fatalf("aggregated sample %d = %#v", index, sample)
		}
		if index > 0 && !samples[index-1].BucketAt.Before(sample.BucketAt) {
			t.Fatalf("metric samples are not chronological at %d", index)
		}
	}
	if _, _, err := database.ListNodeMetricSamples(
		t.Context(), uuid.NewString(), start, now, 360,
	); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing node metrics error = %v, want ErrNotFound", err)
	}

	pruned, err := database.PruneNodeMetricSamples(t.Context(), start.Add(time.Hour), 10)
	if err != nil || pruned != 10 {
		t.Fatalf("PruneNodeMetricSamples() = %d, error = %v, want 10", pruned, err)
	}
}
