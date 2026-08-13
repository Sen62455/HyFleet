package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Alert struct {
	ID              string
	NodeID          string
	NodeName        string
	Type            string
	Severity        string
	Status          string
	Message         string
	OccurrenceCount int
	FirstSeenAt     time.Time
	LastSeenAt      time.Time
	AcknowledgedBy  string
	AcknowledgedAt  *time.Time
	ResolvedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

const alertColumns = `
	a.id, a.node_id, n.name, a.type, a.severity, a.status, a.message,
	a.occurrence_count, a.first_seen_at, a.last_seen_at,
	COALESCE(a.acknowledged_by, ''), a.acknowledged_at, a.resolved_at,
	a.created_at, a.updated_at
`

func scanAlert(row rowScanner) (Alert, error) {
	var alert Alert
	var firstSeenAt, lastSeenAt, createdAt, updatedAt int64
	var acknowledgedAt, resolvedAt sql.NullInt64
	if err := row.Scan(
		&alert.ID, &alert.NodeID, &alert.NodeName, &alert.Type, &alert.Severity,
		&alert.Status, &alert.Message, &alert.OccurrenceCount, &firstSeenAt,
		&lastSeenAt, &alert.AcknowledgedBy, &acknowledgedAt, &resolvedAt,
		&createdAt, &updatedAt,
	); err != nil {
		return Alert{}, err
	}
	alert.FirstSeenAt = unixTime(firstSeenAt)
	alert.LastSeenAt = unixTime(lastSeenAt)
	alert.AcknowledgedAt = nullableTime(acknowledgedAt)
	alert.ResolvedAt = nullableTime(resolvedAt)
	alert.CreatedAt = unixTime(createdAt)
	alert.UpdatedAt = unixTime(updatedAt)
	return alert, nil
}

func (s *Store) ListAlerts(ctx context.Context, status string, limit int) ([]Alert, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	where := "a.resolved_at IS NULL"
	switch status {
	case "", "active":
	case "resolved":
		where = "a.resolved_at IS NOT NULL"
	case "all":
		where = "1 = 1"
	default:
		return nil, ErrUnsupported
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+alertColumns+`
		FROM alerts a JOIN nodes n ON n.id = a.node_id
		WHERE `+where+` AND n.archived_at IS NULL
		ORDER BY CASE a.severity WHEN 'critical' THEN 0 ELSE 1 END,
		         a.last_seen_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()
	alerts := make([]Alert, 0)
	for rows.Next() {
		alert, err := scanAlert(rows)
		if err != nil {
			return nil, fmt.Errorf("scan alert: %w", err)
		}
		alerts = append(alerts, alert)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alerts: %w", err)
	}
	return alerts, nil
}

func (s *Store) AcknowledgeAlert(
	ctx context.Context,
	alertID, adminID string,
	now time.Time,
) (Alert, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE alerts SET status = 'acknowledged', acknowledged_by = ?,
			acknowledged_at = ?, updated_at = ?
		WHERE id = ? AND resolved_at IS NULL
	`, adminID, now.UnixMilli(), now.UnixMilli(), alertID)
	if err != nil {
		return Alert{}, fmt.Errorf("acknowledge alert: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return Alert{}, fmt.Errorf("read alert acknowledgement: %w", err)
	}
	if count == 0 {
		var exists int
		if err := s.db.QueryRowContext(ctx,
			"SELECT EXISTS(SELECT 1 FROM alerts WHERE id = ?)", alertID,
		).Scan(&exists); err != nil {
			return Alert{}, fmt.Errorf("check acknowledged alert: %w", err)
		}
		if exists == 0 {
			return Alert{}, ErrNotFound
		}
		return Alert{}, ErrConflict
	}
	alert, err := scanAlert(s.db.QueryRowContext(ctx, `
		SELECT `+alertColumns+`
		FROM alerts a JOIN nodes n ON n.id = a.node_id WHERE a.id = ?
	`, alertID))
	if err != nil {
		return Alert{}, fmt.Errorf("read acknowledged alert: %w", err)
	}
	return alert, nil
}

type alertCondition struct {
	nodeID                      string
	enabled                     bool
	status                      string
	statusReason                string
	installed                   bool
	coreRunning                 bool
	lastActivityAt              time.Time
	usageEnabled                bool
	usageErrorCode              string
	desiredVersion              int64
	appliedVersion              int64
	desiredCreatedAt            time.Time
	failedAssignments           bool
	unrecoveredOperationFailure bool
	trafficLimitBytes           int64
	trafficUsedBytes            int64
}

func (s *Store) ReconcileAlerts(
	ctx context.Context,
	now time.Time,
	offlineAfter, syncStuckAfter time.Duration,
) error {
	if offlineAfter <= 0 {
		offlineAfter = 90 * time.Second
	}
	if syncStuckAfter <= 0 {
		syncStuckAfter = 5 * time.Minute
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin alert reconciliation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		UPDATE node_operations
		SET status = 'expired', completed_at = ?, updated_at = ?,
			error_code = 'operation_expired', error_message = 'operation expired before completion'
		WHERE status = 'queued' AND expires_at <= ?
	`, now.UnixMilli(), now.UnixMilli(), now.UnixMilli()); err != nil {
		return fmt.Errorf("expire operations during alert reconciliation: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT n.id, n.enabled, n.status, n.status_reason,
		       (COALESCE(n.agent_installation_id, '') <> ''), n.core_running,
		       COALESCE(n.last_seen_at, n.updated_at), n.usage_enabled,
		       n.usage_error_code, n.desired_version, n.applied_version,
		       COALESCE(s.created_at, n.updated_at),
		       EXISTS(
		           SELECT 1 FROM node_user_assignments a
		           WHERE a.node_id = n.id AND a.state = 'failed'
		       ),
		       EXISTS(
		           SELECT 1 FROM node_operations failed
		           WHERE failed.node_id = n.id AND failed.status IN ('failed', 'expired')
		             AND NOT EXISTS (
		                 SELECT 1 FROM node_operations recovered
		                 WHERE recovered.node_id = failed.node_id
		                   AND recovered.type = failed.type
		                   AND recovered.sequence > failed.sequence
		                   AND recovered.status = 'succeeded'
		             )
		       ),
		       n.traffic_limit_bytes,
		       CASE
		         WHEN n.traffic_calibration_bytes IS NULL OR n.traffic_calibration_proxy_bytes IS NULL
		           THEN n.traffic_cycle_upload_bytes + n.traffic_cycle_download_bytes
		         ELSE n.traffic_calibration_bytes + MAX(
		           0,
		           n.traffic_cycle_upload_bytes + n.traffic_cycle_download_bytes -
		           n.traffic_calibration_proxy_bytes
		         )
		       END
		FROM nodes n
		LEFT JOIN node_snapshots s
		  ON s.node_id = n.id AND s.version = n.desired_version
		WHERE n.archived_at IS NULL
	`)
	if err != nil {
		return fmt.Errorf("query alert conditions: %w", err)
	}
	conditions := make([]alertCondition, 0)
	for rows.Next() {
		var condition alertCondition
		var enabled, installed, coreRunning, usageEnabled int
		var lastActivityAt, desiredCreatedAt int64
		var failedAssignments, operationFailure int
		if err := rows.Scan(
			&condition.nodeID, &enabled, &condition.status, &condition.statusReason,
			&installed, &coreRunning, &lastActivityAt, &usageEnabled,
			&condition.usageErrorCode, &condition.desiredVersion,
			&condition.appliedVersion, &desiredCreatedAt, &failedAssignments,
			&operationFailure, &condition.trafficLimitBytes, &condition.trafficUsedBytes,
		); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan alert condition: %w", err)
		}
		condition.enabled = enabled == 1
		condition.installed = installed == 1
		condition.coreRunning = coreRunning == 1
		condition.usageEnabled = usageEnabled == 1
		condition.failedAssignments = failedAssignments == 1
		condition.unrecoveredOperationFailure = operationFailure == 1
		condition.lastActivityAt = unixTime(lastActivityAt)
		condition.desiredCreatedAt = unixTime(desiredCreatedAt)
		conditions = append(conditions, condition)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate alert conditions: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close alert conditions: %w", err)
	}
	for _, condition := range conditions {
		recent := now.Sub(condition.lastActivityAt) < offlineAfter
		trafficWarning := condition.trafficLimitBytes > 0 &&
			condition.trafficUsedBytes < condition.trafficLimitBytes &&
			condition.trafficUsedBytes >= condition.trafficLimitBytes-condition.trafficLimitBytes/5
		trafficExhausted := condition.trafficLimitBytes > 0 &&
			condition.trafficUsedBytes >= condition.trafficLimitBytes
		checks := []struct {
			alertType string
			severity  string
			active    bool
			message   string
		}{
			{"offline", "critical", condition.enabled && condition.installed && !recent, "Agent heartbeat is overdue"},
			{"degraded", "warning", condition.enabled && condition.status == "degraded", condition.statusReason},
			{"core_down", "critical", condition.enabled && condition.installed && recent && !condition.coreRunning, "core service is not running"},
			{"usage_error", "warning", condition.enabled && condition.usageEnabled && condition.usageErrorCode != "", condition.usageErrorCode},
			{"sync_failed", "warning", condition.enabled && condition.failedAssignments, "one or more desired assignments failed"},
			{
				"sync_stuck", "warning",
				condition.enabled && condition.installed && condition.desiredVersion > condition.appliedVersion &&
					now.Sub(condition.desiredCreatedAt) >= syncStuckAfter,
				"desired state has not been applied within the expected window",
			},
			{"operation_failed", "warning", condition.unrecoveredOperationFailure, "a node operation failed or expired"},
			{"traffic_quota_warning", "warning", trafficWarning, "node traffic has reached 80% of the configured monthly allowance"},
			{"traffic_quota_exhausted", "critical", trafficExhausted, "node traffic has reached the configured monthly allowance"},
		}
		for _, check := range checks {
			if check.active {
				if check.message == "" {
					check.message = check.alertType
				}
				if err := upsertAlertTx(
					ctx, tx, condition.nodeID, check.alertType, check.severity,
					check.message, now,
				); err != nil {
					return err
				}
			} else if err := resolveAlertTx(ctx, tx, condition.nodeID, check.alertType, now); err != nil {
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit alert reconciliation: %w", err)
	}
	return nil
}

func upsertAlertTx(
	ctx context.Context,
	tx *sql.Tx,
	nodeID, alertType, severity, message string,
	now time.Time,
) error {
	if len(message) > 512 {
		message = message[:512]
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO alerts(
			id, node_id, type, severity, status, message,
			first_seen_at, last_seen_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, 'open', ?, ?, ?, ?, ?)
		ON CONFLICT(node_id, type) WHERE resolved_at IS NULL DO UPDATE SET
			severity = excluded.severity,
			message = excluded.message,
			last_seen_at = excluded.last_seen_at,
			updated_at = excluded.updated_at
	`, uuid.NewString(), nodeID, alertType, severity, message, now.UnixMilli(),
		now.UnixMilli(), now.UnixMilli(), now.UnixMilli()); err != nil {
		return fmt.Errorf("upsert alert: %w", err)
	}
	return nil
}

func resolveAlertTx(ctx context.Context, tx *sql.Tx, nodeID, alertType string, now time.Time) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE alerts SET status = 'resolved', resolved_at = ?, updated_at = ?
		WHERE node_id = ? AND type = ? AND resolved_at IS NULL
	`, now.UnixMilli(), now.UnixMilli(), nodeID, alertType); err != nil {
		return fmt.Errorf("resolve alert: %w", err)
	}
	return nil
}

func (s *Store) GetAlert(ctx context.Context, alertID string) (Alert, error) {
	alert, err := scanAlert(s.db.QueryRowContext(ctx, `
		SELECT `+alertColumns+`
		FROM alerts a JOIN nodes n ON n.id = a.node_id WHERE a.id = ?
	`, alertID))
	if errors.Is(err, sql.ErrNoRows) {
		return Alert{}, ErrNotFound
	}
	if err != nil {
		return Alert{}, fmt.Errorf("get alert: %w", err)
	}
	return alert, nil
}
