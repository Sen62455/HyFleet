package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type NodeMetricSample struct {
	BucketAt                time.Time
	CPUPercent              float64
	MemoryUsedBytes         int64
	MemoryTotalBytes        int64
	SwapUsedBytes           int64
	SwapTotalBytes          int64
	DiskUsedBytes           int64
	DiskTotalBytes          int64
	DiskReadBytesPerSecond  int64
	DiskWriteBytesPerSecond int64
	NetworkRXBPS            int64
	NetworkTXBPS            int64
	Load1                   float64
	Load5                   float64
	Load15                  float64
	SampledAt               time.Time
}

func (s *Store) ListNodeMetricSamples(
	ctx context.Context,
	nodeID string,
	since, until time.Time,
	maxPoints int,
) ([]NodeMetricSample, time.Duration, error) {
	if maxPoints < 30 || maxPoints > 1000 || !since.Before(until) {
		return nil, 0, ErrUnsupported
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, `
		SELECT 1 FROM nodes WHERE id = ? AND archived_at IS NULL
	`, nodeID).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return nil, 0, ErrNotFound
	} else if err != nil {
		return nil, 0, fmt.Errorf("find metric node: %w", err)
	}
	step := time.Minute
	rangeDuration := until.Sub(since)
	if rangeDuration > time.Duration(maxPoints)*step {
		// The time range is inclusive at both ends. Use maxPoints-1 intervals so
		// aligned boundary samples cannot produce a maxPoints+1 result.
		minutes := (int64(rangeDuration/time.Minute) + int64(maxPoints) - 2) / int64(maxPoints-1)
		step = time.Duration(minutes) * time.Minute
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT MIN(bucket_at), AVG(cpu_percent),
		       CAST(AVG(memory_used_bytes) AS INTEGER), MAX(memory_total_bytes),
		       CAST(AVG(swap_used_bytes) AS INTEGER), MAX(swap_total_bytes),
		       CAST(AVG(disk_used_bytes) AS INTEGER), MAX(disk_total_bytes),
		       CAST(AVG(disk_read_bytes_per_second) AS INTEGER),
		       CAST(AVG(disk_write_bytes_per_second) AS INTEGER),
		       CAST(AVG(network_rx_bps) AS INTEGER), CAST(AVG(network_tx_bps) AS INTEGER),
		       AVG(load_1), AVG(load_5), AVG(load_15), MAX(sampled_at)
		FROM node_metric_samples
		WHERE node_id = ? AND bucket_at >= ? AND bucket_at <= ?
		GROUP BY (bucket_at / ?)
		ORDER BY MIN(bucket_at)
	`, nodeID, since.UnixMilli(), until.UnixMilli(), step.Milliseconds())
	if err != nil {
		return nil, 0, fmt.Errorf("query node metrics: %w", err)
	}
	defer rows.Close()
	samples := make([]NodeMetricSample, 0, maxPoints)
	for rows.Next() {
		var sample NodeMetricSample
		var bucketAt, sampledAt int64
		if err := rows.Scan(
			&bucketAt, &sample.CPUPercent, &sample.MemoryUsedBytes, &sample.MemoryTotalBytes,
			&sample.SwapUsedBytes, &sample.SwapTotalBytes, &sample.DiskUsedBytes,
			&sample.DiskTotalBytes, &sample.DiskReadBytesPerSecond,
			&sample.DiskWriteBytesPerSecond, &sample.NetworkRXBPS, &sample.NetworkTXBPS,
			&sample.Load1, &sample.Load5, &sample.Load15, &sampledAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan node metrics: %w", err)
		}
		sample.BucketAt = unixTime(bucketAt)
		sample.SampledAt = unixTime(sampledAt)
		samples = append(samples, sample)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate node metrics: %w", err)
	}
	return samples, step, nil
}

func (s *Store) PruneNodeMetricSamples(ctx context.Context, before time.Time, limit int) (int64, error) {
	if limit < 1 || limit > 10000 {
		limit = 5000
	}
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM node_metric_samples WHERE rowid IN (
			SELECT rowid FROM node_metric_samples WHERE bucket_at < ? LIMIT ?
		)
	`, before.UnixMilli(), limit)
	if err != nil {
		return 0, fmt.Errorf("prune node metrics: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read pruned metric count: %w", err)
	}
	return count, nil
}
