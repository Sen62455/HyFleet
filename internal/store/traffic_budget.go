package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"
)

func nullableInt64(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func trafficCycleStart(at time.Time, resetDay int) time.Time {
	at = at.UTC()
	if resetDay < 1 {
		resetDay = 1
	} else if resetDay > 31 {
		resetDay = 31
	}
	start := trafficCycleBoundary(at.Year(), at.Month(), resetDay)
	if at.Before(start) {
		previous := at.AddDate(0, -1, 0)
		start = trafficCycleBoundary(previous.Year(), previous.Month(), resetDay)
	}
	return start
}

func trafficCycleNextStart(start time.Time, resetDay int) time.Time {
	next := start.UTC().AddDate(0, 1, 0)
	return trafficCycleBoundary(next.Year(), next.Month(), resetDay)
}

func trafficCycleBoundary(year int, month time.Month, resetDay int) time.Time {
	days := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	day := resetDay
	if day > days {
		day = days
	}
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func trafficCycleTotalsTx(
	ctx context.Context,
	tx *sql.Tx,
	nodeID string,
	start, end time.Time,
) (int64, int64, error) {
	var upload, download int64
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(upload_bytes), 0), COALESCE(SUM(download_bytes), 0)
		FROM traffic_batches
		WHERE node_id = ? AND sampled_at >= ? AND sampled_at < ?
	`, nodeID, start.UnixMilli(), end.UnixMilli()).Scan(&upload, &download); err != nil {
		return 0, 0, fmt.Errorf("sum node traffic cycle: %w", err)
	}
	if upload < 0 || download < 0 {
		return 0, 0, errors.New("node traffic cycle total is invalid")
	}
	return upload, download, nil
}

func EffectiveNodeTrafficUsed(node Node) int64 {
	proxyUsed, ok := checkedAdd(node.TrafficCycleUploadBytes, node.TrafficCycleDownloadBytes)
	if !ok {
		return math.MaxInt64
	}
	if node.TrafficCalibrationBytes == nil || node.TrafficCalibrationProxyBytes == nil {
		return proxyUsed
	}
	delta := proxyUsed - *node.TrafficCalibrationProxyBytes
	if delta < 0 {
		delta = 0
	}
	if *node.TrafficCalibrationBytes > math.MaxInt64-delta {
		return math.MaxInt64
	}
	return *node.TrafficCalibrationBytes + delta
}

func (s *Store) CalibrateNodeTraffic(
	ctx context.Context,
	nodeID string,
	providerUsedBytes int64,
	now time.Time,
) (Node, error) {
	if providerUsedBytes < 0 {
		return Node{}, ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, fmt.Errorf("begin node traffic calibration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var resetDay int
	if err := tx.QueryRowContext(ctx, `
		SELECT traffic_reset_day FROM nodes WHERE id = ? AND archived_at IS NULL
	`, nodeID).Scan(&resetDay); errors.Is(err, sql.ErrNoRows) {
		return Node{}, ErrNotFound
	} else if err != nil {
		return Node{}, fmt.Errorf("read node traffic calibration target: %w", err)
	}
	cycleStart := trafficCycleStart(now, resetDay)
	cycleUpload, cycleDownload, err := trafficCycleTotalsTx(
		ctx, tx, nodeID, cycleStart, trafficCycleNextStart(cycleStart, resetDay),
	)
	if err != nil {
		return Node{}, err
	}
	proxyUsed, ok := checkedAdd(cycleUpload, cycleDownload)
	if !ok {
		return Node{}, errors.New("node traffic cycle exceeds SQLite integer range")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE nodes SET traffic_cycle_started_at = ?, traffic_cycle_upload_bytes = ?,
			traffic_cycle_download_bytes = ?, traffic_calibration_bytes = ?,
			traffic_calibration_proxy_bytes = ?, traffic_calibrated_at = ?, updated_at = ?
		WHERE id = ?
	`, cycleStart.UnixMilli(), cycleUpload, cycleDownload, providerUsedBytes,
		proxyUsed, now.UnixMilli(), now.UnixMilli(), nodeID); err != nil {
		return Node{}, fmt.Errorf("store node traffic calibration: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("commit node traffic calibration: %w", err)
	}
	return s.GetNode(ctx, nodeID)
}
