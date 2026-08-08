package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hyfleet/hyfleet/internal/protocol"
)

type Node struct {
	ID                  string
	Name                string
	Provider            string
	Region              string
	AdapterType         string
	Enabled             bool
	Status              string
	StatusReason        string
	DesiredVersion      int64
	AppliedVersion      int64
	AgentInstallationID string
	AgentVersion        string
	ProtocolVersion     int
	OSName              string
	OSVersion           string
	Architecture        string
	CoreName            string
	CoreVersion         string
	CoreRunning         bool
	UptimeSeconds       int64
	CPUPercent          float64
	MemoryUsedBytes     int64
	MemoryTotalBytes    int64
	DiskUsedBytes       int64
	DiskTotalBytes      int64
	NetworkRXBPS        int64
	NetworkTXBPS        int64
	Load1               float64
	Load5               float64
	Load15              float64
	LastSeenAt          *time.Time
	LastAppliedAt       *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type NewNode struct {
	ID          string
	Name        string
	Provider    string
	Region      string
	AdapterType string
	Enabled     bool
	Now         time.Time
}

type UpdateNode struct {
	Name        string
	Provider    string
	Region      string
	AdapterType string
	Enabled     bool
	Now         time.Time
}

const nodeColumns = `
	id, name, provider, region, adapter_type, enabled, status, status_reason,
	desired_version, applied_version, COALESCE(agent_installation_id, ''),
	agent_version, protocol_version, os_name, os_version, architecture,
	core_name, core_version, core_running, uptime_seconds, cpu_percent,
	memory_used_bytes, memory_total_bytes, disk_used_bytes, disk_total_bytes,
	network_rx_bps, network_tx_bps, load_1, load_5, load_15,
	last_seen_at, last_applied_at, created_at, updated_at
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanNode(row rowScanner) (Node, error) {
	var node Node
	var enabled, coreRunning int
	var lastSeen, lastApplied sql.NullInt64
	var created, updated int64
	err := row.Scan(
		&node.ID, &node.Name, &node.Provider, &node.Region, &node.AdapterType,
		&enabled, &node.Status, &node.StatusReason, &node.DesiredVersion,
		&node.AppliedVersion, &node.AgentInstallationID, &node.AgentVersion,
		&node.ProtocolVersion, &node.OSName, &node.OSVersion, &node.Architecture,
		&node.CoreName, &node.CoreVersion, &coreRunning, &node.UptimeSeconds,
		&node.CPUPercent, &node.MemoryUsedBytes, &node.MemoryTotalBytes,
		&node.DiskUsedBytes, &node.DiskTotalBytes, &node.NetworkRXBPS,
		&node.NetworkTXBPS, &node.Load1, &node.Load5, &node.Load15,
		&lastSeen, &lastApplied, &created, &updated,
	)
	if err != nil {
		return Node{}, err
	}
	node.Enabled = enabled == 1
	node.CoreRunning = coreRunning == 1
	node.LastSeenAt = nullableTime(lastSeen)
	node.LastAppliedAt = nullableTime(lastApplied)
	node.CreatedAt = unixTime(created)
	node.UpdatedAt = unixTime(updated)
	return node, nil
}

func (s *Store) CreateNode(ctx context.Context, input NewNode) (Node, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, fmt.Errorf("begin create node: %w", err)
	}
	status := "pending"
	if !input.Enabled {
		status = "disabled"
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO nodes(
			id, name, provider, region, adapter_type, enabled, status,
			desired_version, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)
	`, input.ID, input.Name, input.Provider, input.Region, input.AdapterType,
		boolInt(input.Enabled), status, input.Now.UnixMilli(), input.Now.UnixMilli())
	if err != nil {
		_ = tx.Rollback()
		return Node{}, fmt.Errorf("insert node: %w", err)
	}
	if err := insertSnapshot(ctx, tx, input.ID, input.AdapterType, 1, input.Now); err != nil {
		_ = tx.Rollback()
		return Node{}, err
	}
	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("commit create node: %w", err)
	}
	return s.GetNode(ctx, input.ID)
}

func (s *Store) ListNodes(ctx context.Context) ([]Node, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+nodeColumns+" FROM nodes WHERE archived_at IS NULL ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	defer rows.Close()
	nodes := make([]Node, 0)
	for rows.Next() {
		node, err := scanNode(rows)
		if err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nodes: %w", err)
	}
	return nodes, nil
}

func (s *Store) GetNode(ctx context.Context, id string) (Node, error) {
	node, err := scanNode(s.db.QueryRowContext(ctx,
		"SELECT "+nodeColumns+" FROM nodes WHERE id = ? AND archived_at IS NULL", id,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Node{}, ErrNotFound
		}
		return Node{}, fmt.Errorf("get node: %w", err)
	}
	return node, nil
}

func (s *Store) UpdateNode(ctx context.Context, id string, input UpdateNode) (Node, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, fmt.Errorf("begin update node: %w", err)
	}
	var version int64
	var currentAdapter, installationID string
	err = tx.QueryRowContext(ctx,
		`SELECT desired_version, adapter_type, COALESCE(agent_installation_id, '')
		 FROM nodes WHERE id = ? AND archived_at IS NULL`, id,
	).Scan(&version, &currentAdapter, &installationID)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return Node{}, ErrNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return Node{}, fmt.Errorf("read node version: %w", err)
	}
	if installationID != "" && input.AdapterType != currentAdapter {
		_ = tx.Rollback()
		return Node{}, ErrConflict
	}
	version++
	status := "pending"
	if !input.Enabled {
		status = "disabled"
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE nodes SET name = ?, provider = ?, region = ?, adapter_type = ?,
			enabled = ?, status = ?, status_reason = '', desired_version = ?, updated_at = ?
		WHERE id = ? AND archived_at IS NULL
	`, input.Name, input.Provider, input.Region, input.AdapterType,
		boolInt(input.Enabled), status, version, input.Now.UnixMilli(), id)
	if err != nil {
		_ = tx.Rollback()
		return Node{}, fmt.Errorf("update node: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE node_snapshots SET superseded_at = ? WHERE node_id = ? AND superseded_at IS NULL",
		input.Now.UnixMilli(), id,
	); err != nil {
		_ = tx.Rollback()
		return Node{}, fmt.Errorf("supersede snapshot: %w", err)
	}
	if err := insertSnapshot(ctx, tx, id, input.AdapterType, version, input.Now); err != nil {
		_ = tx.Rollback()
		return Node{}, err
	}
	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("commit update node: %w", err)
	}
	return s.GetNode(ctx, id)
}

func (s *Store) ArchiveNode(ctx context.Context, id string, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE nodes SET enabled = 0, status = 'disabled', archived_at = ?, updated_at = ?
		WHERE id = ? AND archived_at IS NULL
	`, now.UnixMilli(), now.UnixMilli(), id)
	if err != nil {
		return fmt.Errorf("archive node: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read archive result: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func insertSnapshot(ctx context.Context, tx *sql.Tx, nodeID, adapter string, version int64, now time.Time) error {
	snapshot := protocol.DesiredSnapshot{
		SchemaVersion: 1,
		NodeID:        nodeID,
		Version:       version,
		Adapter:       adapter,
		Users:         []protocol.DesiredUser{},
		GeneratedAt:   now.UTC(),
	}
	canonical, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("encode desired snapshot: %w", err)
	}
	hash := sha256.Sum256(canonical)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO node_snapshots(node_id, version, canonical_json, sha256, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, nodeID, version, canonical, hash[:], now.UnixMilli())
	if err != nil {
		return fmt.Errorf("insert desired snapshot: %w", err)
	}
	return nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
