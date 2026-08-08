package protocol

import "time"

const (
	MajorVersion            = 1
	MaxTrafficItemsPerBatch = 1000
)

type ErrorResponse struct {
	Error APIError `json:"error"`
}

type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

type EnrollRequest struct {
	EnrollmentToken string            `json:"enrollment_token"`
	InstallationID  string            `json:"installation_id"`
	RequestID       string            `json:"request_id"`
	AgentVersion    string            `json:"agent_version"`
	OS              string            `json:"os"`
	OSVersion       string            `json:"os_version"`
	Architecture    string            `json:"architecture"`
	Capabilities    []string          `json:"capabilities"`
	Adapter         EnrollmentAdapter `json:"adapter"`
}

type EnrollmentAdapter struct {
	Type     string `json:"type"`
	CoreName string `json:"core_name,omitempty"`
}

type EnrollResponse struct {
	NodeID         string        `json:"node_id"`
	NodeCredential string        `json:"node_credential"`
	Protocol       int           `json:"protocol"`
	Polling        PollingPolicy `json:"polling"`
	ServerTime     time.Time     `json:"server_time"`
}

type PollingPolicy struct {
	HeartbeatSeconds int `json:"heartbeat_seconds"`
	DesiredSeconds   int `json:"desired_seconds"`
}

type HeartbeatRequest struct {
	InstallationID string      `json:"installation_id"`
	AppliedVersion int64       `json:"applied_version"`
	Agent          AgentInfo   `json:"agent"`
	Core           CoreInfo    `json:"core"`
	Host           HostMetrics `json:"host"`
	Usage          UsageInfo   `json:"usage"`
	SampledAt      time.Time   `json:"sampled_at"`
}

type AgentInfo struct {
	Version  string `json:"version"`
	Protocol int    `json:"protocol"`
}

type CoreInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Running bool   `json:"running"`
}

type UsageInfo struct {
	Enabled       bool       `json:"enabled"`
	Available     bool       `json:"available"`
	OutboxBatches int        `json:"outbox_batches"`
	LastSampledAt *time.Time `json:"last_sampled_at,omitempty"`
	LastErrorCode string     `json:"last_error_code,omitempty"`
}

type HostMetrics struct {
	UptimeSeconds    int64   `json:"uptime_seconds"`
	CPUPercent       float64 `json:"cpu_percent"`
	MemoryUsedBytes  int64   `json:"memory_used_bytes"`
	MemoryTotalBytes int64   `json:"memory_total_bytes"`
	DiskUsedBytes    int64   `json:"disk_used_bytes"`
	DiskTotalBytes   int64   `json:"disk_total_bytes"`
	NetworkRXBPS     int64   `json:"network_rx_bps"`
	NetworkTXBPS     int64   `json:"network_tx_bps"`
	Load1            float64 `json:"load_1"`
	Load5            float64 `json:"load_5"`
	Load15           float64 `json:"load_15"`
}

type HeartbeatResponse struct {
	ServerTime     time.Time `json:"server_time"`
	DesiredVersion int64     `json:"desired_version"`
}

type DesiredSnapshot struct {
	SchemaVersion int           `json:"schema_version"`
	NodeID        string        `json:"node_id"`
	Version       int64         `json:"version"`
	Adapter       string        `json:"adapter"`
	Users         []DesiredUser `json:"users"`
	Kicks         []DesiredKick `json:"kicks"`
	GeneratedAt   time.Time     `json:"generated_at"`
}

type DesiredUser struct {
	ID         string            `json:"id"`
	Username   string            `json:"username"`
	Credential DesiredCredential `json:"credential"`
	Enabled    bool              `json:"enabled"`
	ExpiresAt  *time.Time        `json:"expires_at"`
	QuotaState string            `json:"quota_state"`
}

type DesiredCredential struct {
	Ref            string `json:"ref"`
	Fingerprint    string `json:"fingerprint"`
	VerifierSHA256 string `json:"verifier_sha256,omitempty"`
}

type DesiredKick struct {
	UserID     string `json:"user_id"`
	Generation int64  `json:"generation"`
}

type DesiredEnvelope struct {
	Snapshot  DesiredSnapshot `json:"snapshot"`
	SHA256    string          `json:"sha256"`
	CreatedAt time.Time       `json:"created_at"`
}

type DesiredAckRequest struct {
	Status       string `json:"status"`
	SnapshotHash string `json:"snapshot_hash"`
	Adapter      string `json:"adapter"`
	DurationMS   int64  `json:"duration_ms"`
	ErrorCode    string `json:"error_code,omitempty"`
	Message      string `json:"message,omitempty"`
}

type TrafficBatchesRequest struct {
	Batches []TrafficBatch `json:"batches"`
}

type TrafficBatch struct {
	ID             string         `json:"id"`
	InstallationID string         `json:"installation_id"`
	SourceEpoch    string         `json:"source_epoch"`
	Sequence       int64          `json:"sequence"`
	SampledAt      time.Time      `json:"sampled_at"`
	Items          []TrafficDelta `json:"items"`
}

type TrafficDelta struct {
	UserID        string `json:"user_id"`
	UploadBytes   int64  `json:"upload_bytes"`
	DownloadBytes int64  `json:"download_bytes"`
}

type TrafficBatchesResponse struct {
	Results    []TrafficBatchResult `json:"results"`
	ServerTime time.Time            `json:"server_time"`
}

type TrafficBatchResult struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	ErrorCode string `json:"error_code,omitempty"`
}

type OnlineSnapshotRequest struct {
	SnapshotID     string       `json:"snapshot_id"`
	InstallationID string       `json:"installation_id"`
	SampledAt      time.Time    `json:"sampled_at"`
	Users          []OnlineUser `json:"users"`
}

type OnlineUser struct {
	UserID      string `json:"user_id"`
	Connections int    `json:"connections"`
}

type OnlineSnapshotResponse struct {
	Accepted   bool      `json:"accepted"`
	ServerTime time.Time `json:"server_time"`
}
