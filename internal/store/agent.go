package store

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hyfleet/hyfleet/internal/cryptoutil"
	"github.com/hyfleet/hyfleet/internal/protocol"
)

type EnrollmentToken struct {
	ID        string
	NodeID    string
	Token     string
	ExpiresAt time.Time
}

type AgentIdentity struct {
	NodeID         string
	InstallationID string
	AdapterType    string
	Enabled        bool
}

type EnrollmentFacts struct {
	InstallationID string
	RequestID      string
	AgentVersion   string
	OSName         string
	OSVersion      string
	Architecture   string
	AdapterType    string
	CoreName       string
}

func (s *Store) CreateEnrollmentToken(
	ctx context.Context,
	nodeID, adminID string,
	now time.Time,
	lifetime time.Duration,
) (EnrollmentToken, error) {
	token, err := cryptoutil.RandomToken(32)
	if err != nil {
		return EnrollmentToken{}, err
	}
	result := EnrollmentToken{
		ID:        cryptoutil.NewID(),
		NodeID:    nodeID,
		Token:     token,
		ExpiresAt: now.Add(lifetime),
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return EnrollmentToken{}, fmt.Errorf("begin enrollment token: %w", err)
	}
	var exists int
	if err := tx.QueryRowContext(ctx,
		"SELECT 1 FROM nodes WHERE id = ? AND archived_at IS NULL", nodeID,
	).Scan(&exists); err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return EnrollmentToken{}, ErrNotFound
		}
		return EnrollmentToken{}, fmt.Errorf("find enrollment node: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM node_enrollment_tokens
		WHERE node_id = ? AND consumed_at IS NULL
	`, nodeID); err != nil {
		_ = tx.Rollback()
		return EnrollmentToken{}, fmt.Errorf("invalidate enrollment tokens: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO node_enrollment_tokens(
			id, node_id, token_hash, expires_at, created_by, created_at
		) VALUES (?, ?, ?, ?, ?, ?)
	`, result.ID, nodeID, cryptoutil.TokenHash(token), result.ExpiresAt.UnixMilli(),
		adminID, now.UnixMilli()); err != nil {
		_ = tx.Rollback()
		return EnrollmentToken{}, fmt.Errorf("insert enrollment token: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return EnrollmentToken{}, fmt.Errorf("commit enrollment token: %w", err)
	}
	return result, nil
}

func (s *Store) EnrollAgent(
	ctx context.Context,
	token string,
	facts EnrollmentFacts,
	masterKey []byte,
	now time.Time,
) (protocol.EnrollResponse, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return protocol.EnrollResponse{}, fmt.Errorf("begin agent enrollment: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var enrollmentID, nodeID, adapterType string
	var expiresAt int64
	var consumedAt, responseExpires sql.NullInt64
	var boundInstallation, boundRequest sql.NullString
	var ciphertext []byte
	err = tx.QueryRowContext(ctx, `
		SELECT t.id, t.node_id, n.adapter_type, t.expires_at, t.consumed_at,
		       t.bound_installation_id, t.bound_request_id,
		       t.response_ciphertext, t.response_expires_at
		FROM node_enrollment_tokens t
		JOIN nodes n ON n.id = t.node_id
		WHERE t.token_hash = ? AND n.archived_at IS NULL
	`, cryptoutil.TokenHash(token)).Scan(
		&enrollmentID, &nodeID, &adapterType, &expiresAt, &consumedAt,
		&boundInstallation, &boundRequest, &ciphertext, &responseExpires,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return protocol.EnrollResponse{}, ErrUnauthorized
	}
	if err != nil {
		return protocol.EnrollResponse{}, fmt.Errorf("load enrollment token: %w", err)
	}
	if consumedAt.Valid {
		if !boundInstallation.Valid || !boundRequest.Valid ||
			boundInstallation.String != facts.InstallationID || boundRequest.String != facts.RequestID ||
			!responseExpires.Valid || responseExpires.Int64 < now.UnixMilli() || len(ciphertext) == 0 {
			return protocol.EnrollResponse{}, ErrConflict
		}
		plaintext, err := cryptoutil.Open(masterKey, ciphertext,
			enrollmentAAD(enrollmentID, nodeID, facts.InstallationID, facts.RequestID))
		if err != nil {
			return protocol.EnrollResponse{}, fmt.Errorf("open enrollment replay capsule: %w", err)
		}
		var response protocol.EnrollResponse
		if err := json.Unmarshal(plaintext, &response); err != nil {
			return protocol.EnrollResponse{}, fmt.Errorf("decode enrollment replay capsule: %w", err)
		}
		return response, nil
	}
	if expiresAt < now.UnixMilli() {
		return protocol.EnrollResponse{}, ErrExpired
	}
	if adapterType != facts.AdapterType {
		return protocol.EnrollResponse{}, ErrConflict
	}
	secret, err := cryptoutil.RandomToken(32)
	if err != nil {
		return protocol.EnrollResponse{}, err
	}
	credential := "hya_" + nodeID + "." + secret
	response := protocol.EnrollResponse{
		NodeID:         nodeID,
		NodeCredential: credential,
		Protocol:       protocol.MajorVersion,
		Polling: protocol.PollingPolicy{
			HeartbeatSeconds: 15,
			DesiredSeconds:   10,
		},
		ServerTime: now.UTC(),
	}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return protocol.EnrollResponse{}, fmt.Errorf("encode enrollment response: %w", err)
	}
	ciphertext, err = cryptoutil.Seal(masterKey, responseJSON,
		enrollmentAAD(enrollmentID, nodeID, facts.InstallationID, facts.RequestID))
	if err != nil {
		return protocol.EnrollResponse{}, fmt.Errorf("seal enrollment response: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE nodes SET agent_installation_id = ?, agent_credential_hash = ?,
			agent_version = ?, protocol_version = ?, os_name = ?, os_version = ?,
			architecture = ?, core_name = ?, status = 'pending', updated_at = ?
		WHERE id = ?
	`, facts.InstallationID, cryptoutil.TokenHash(credential), facts.AgentVersion,
		protocol.MajorVersion, facts.OSName, facts.OSVersion, facts.Architecture,
		facts.CoreName, now.UnixMilli(), nodeID); err != nil {
		return protocol.EnrollResponse{}, fmt.Errorf("bind agent to node: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE node_enrollment_tokens SET consumed_at = ?, bound_installation_id = ?,
			bound_request_id = ?, response_ciphertext = ?, response_expires_at = ?
		WHERE id = ? AND consumed_at IS NULL
	`, now.UnixMilli(), facts.InstallationID, facts.RequestID, ciphertext,
		now.Add(5*time.Minute).UnixMilli(), enrollmentID); err != nil {
		return protocol.EnrollResponse{}, fmt.Errorf("consume enrollment token: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return protocol.EnrollResponse{}, fmt.Errorf("commit agent enrollment: %w", err)
	}
	return response, nil
}

func enrollmentAAD(enrollmentID, nodeID, installationID, requestID string) []byte {
	return []byte(strings.Join([]string{enrollmentID, nodeID, installationID, requestID}, "\x00"))
}

func (s *Store) AuthenticateAgent(ctx context.Context, credential string) (AgentIdentity, error) {
	if !strings.HasPrefix(credential, "hya_") {
		return AgentIdentity{}, ErrUnauthorized
	}
	nodePart, _, found := strings.Cut(strings.TrimPrefix(credential, "hya_"), ".")
	if !found || nodePart == "" {
		return AgentIdentity{}, ErrUnauthorized
	}
	var identity AgentIdentity
	var expected []byte
	var enabled int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(agent_installation_id, ''), adapter_type, enabled,
		       agent_credential_hash
		FROM nodes WHERE id = ? AND archived_at IS NULL
	`, nodePart).Scan(
		&identity.NodeID, &identity.InstallationID, &identity.AdapterType, &enabled, &expected,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return AgentIdentity{}, ErrUnauthorized
	}
	if err != nil {
		return AgentIdentity{}, fmt.Errorf("authenticate agent: %w", err)
	}
	actual := cryptoutil.TokenHash(credential)
	if len(expected) != sha256.Size || subtle.ConstantTimeCompare(actual, expected) != 1 {
		return AgentIdentity{}, ErrUnauthorized
	}
	identity.Enabled = enabled == 1
	return identity, nil
}

func (s *Store) RecordHeartbeat(
	ctx context.Context,
	identity AgentIdentity,
	heartbeat protocol.HeartbeatRequest,
	now time.Time,
) (int64, error) {
	if identity.InstallationID != heartbeat.InstallationID {
		return 0, ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin heartbeat: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	status := "online"
	if !identity.Enabled {
		status = "disabled"
	}
	var desiredVersion int64
	err = tx.QueryRowContext(ctx, `
		UPDATE nodes SET status = ?, status_reason = '', agent_version = ?,
			protocol_version = ?, core_name = ?, core_version = ?, core_running = ?,
			uptime_seconds = ?, cpu_percent = ?, memory_used_bytes = ?,
			memory_total_bytes = ?, disk_used_bytes = ?, disk_total_bytes = ?,
			network_rx_bps = ?, network_tx_bps = ?, load_1 = ?, load_5 = ?, load_15 = ?,
			last_seen_at = ?, updated_at = ?
		WHERE id = ? AND agent_installation_id = ?
		RETURNING desired_version
	`, status, heartbeat.Agent.Version, heartbeat.Agent.Protocol,
		heartbeat.Core.Name, heartbeat.Core.Version, boolInt(heartbeat.Core.Running),
		heartbeat.Host.UptimeSeconds, heartbeat.Host.CPUPercent,
		heartbeat.Host.MemoryUsedBytes, heartbeat.Host.MemoryTotalBytes,
		heartbeat.Host.DiskUsedBytes, heartbeat.Host.DiskTotalBytes,
		heartbeat.Host.NetworkRXBPS, heartbeat.Host.NetworkTXBPS,
		heartbeat.Host.Load1, heartbeat.Host.Load5, heartbeat.Host.Load15,
		now.UnixMilli(), now.UnixMilli(), identity.NodeID, identity.InstallationID,
	).Scan(&desiredVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrConflict
	}
	if err != nil {
		return 0, fmt.Errorf("update heartbeat: %w", err)
	}
	bucket := heartbeat.SampledAt.UTC().Truncate(time.Minute).UnixMilli()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO node_metric_samples(
			node_id, bucket_at, cpu_percent, memory_used_bytes, memory_total_bytes,
			disk_used_bytes, disk_total_bytes, network_rx_bps, network_tx_bps,
			load_1, load_5, load_15, sampled_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(node_id, bucket_at) DO UPDATE SET
			cpu_percent = excluded.cpu_percent,
			memory_used_bytes = excluded.memory_used_bytes,
			memory_total_bytes = excluded.memory_total_bytes,
			disk_used_bytes = excluded.disk_used_bytes,
			disk_total_bytes = excluded.disk_total_bytes,
			network_rx_bps = excluded.network_rx_bps,
			network_tx_bps = excluded.network_tx_bps,
			load_1 = excluded.load_1,
			load_5 = excluded.load_5,
			load_15 = excluded.load_15,
			sampled_at = excluded.sampled_at
	`, identity.NodeID, bucket, heartbeat.Host.CPUPercent,
		heartbeat.Host.MemoryUsedBytes, heartbeat.Host.MemoryTotalBytes,
		heartbeat.Host.DiskUsedBytes, heartbeat.Host.DiskTotalBytes,
		heartbeat.Host.NetworkRXBPS, heartbeat.Host.NetworkTXBPS,
		heartbeat.Host.Load1, heartbeat.Host.Load5, heartbeat.Host.Load15,
		heartbeat.SampledAt.UnixMilli()); err != nil {
		return 0, fmt.Errorf("upsert metric sample: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE node_enrollment_tokens
		SET response_ciphertext = NULL, response_expires_at = NULL
		WHERE node_id = ? AND bound_installation_id = ? AND response_ciphertext IS NOT NULL
	`, identity.NodeID, identity.InstallationID); err != nil {
		return 0, fmt.Errorf("clear enrollment replay capsule: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit heartbeat: %w", err)
	}
	return desiredVersion, nil
}

func (s *Store) GetDesiredSnapshot(
	ctx context.Context,
	nodeID string,
	version int64,
) (protocol.DesiredEnvelope, error) {
	var canonical, hash []byte
	var createdAt int64
	err := s.db.QueryRowContext(ctx, `
		SELECT canonical_json, sha256, created_at
		FROM node_snapshots WHERE node_id = ? AND version = ?
	`, nodeID, version).Scan(&canonical, &hash, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return protocol.DesiredEnvelope{}, ErrNotFound
	}
	if err != nil {
		return protocol.DesiredEnvelope{}, fmt.Errorf("get desired snapshot: %w", err)
	}
	var snapshot protocol.DesiredSnapshot
	if err := json.Unmarshal(canonical, &snapshot); err != nil {
		return protocol.DesiredEnvelope{}, fmt.Errorf("decode desired snapshot: %w", err)
	}
	return protocol.DesiredEnvelope{
		Snapshot:  snapshot,
		SHA256:    base64.RawURLEncoding.EncodeToString(hash),
		CreatedAt: unixTime(createdAt),
	}, nil
}

func (s *Store) AcknowledgeDesired(
	ctx context.Context,
	identity AgentIdentity,
	version int64,
	hash []byte,
	status, errorCode, message string,
	now time.Time,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin desired acknowledgement: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var expected []byte
	var desiredVersion int64
	err = tx.QueryRowContext(ctx, `
		SELECT s.sha256, n.desired_version
		FROM node_snapshots s JOIN nodes n ON n.id = s.node_id
		WHERE s.node_id = ? AND s.version = ? AND n.archived_at IS NULL
	`, identity.NodeID, version).Scan(&expected, &desiredVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrVersionConflict
	}
	if err != nil {
		return fmt.Errorf("read desired acknowledgement target: %w", err)
	}
	if version != desiredVersion || subtle.ConstantTimeCompare(hash, expected) != 1 {
		return ErrVersionConflict
	}
	if status == "applied" {
		resultStatus := "online"
		if !identity.Enabled {
			resultStatus = "disabled"
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE nodes SET applied_version = ?, status = ?, status_reason = '',
				last_applied_at = ?, updated_at = ? WHERE id = ?
		`, version, resultStatus, now.UnixMilli(), now.UnixMilli(), identity.NodeID)
	} else {
		if len(message) > 240 {
			message = message[:240]
		}
		reason := errorCode
		if message != "" {
			reason += ": " + message
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE nodes SET status = 'degraded', status_reason = ?, updated_at = ?
			WHERE id = ?
		`, reason, now.UnixMilli(), identity.NodeID)
	}
	if err != nil {
		return fmt.Errorf("update desired acknowledgement: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit desired acknowledgement: %w", err)
	}
	return nil
}
