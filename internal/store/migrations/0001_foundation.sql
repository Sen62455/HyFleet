CREATE TABLE admins (
    id TEXT PRIMARY KEY,
    singleton INTEGER NOT NULL DEFAULT 1 UNIQUE CHECK (singleton = 1),
    username TEXT NOT NULL COLLATE NOCASE UNIQUE,
    password_hash TEXT NOT NULL,
    disabled_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE admin_sessions (
    id TEXT PRIMARY KEY,
    admin_id TEXT NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    token_hash BLOB NOT NULL UNIQUE,
    csrf_token TEXT NOT NULL,
    expires_at INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    revoked_at INTEGER,
    created_at INTEGER NOT NULL
);

CREATE INDEX admin_sessions_admin_idx ON admin_sessions(admin_id);
CREATE INDEX admin_sessions_expiry_idx ON admin_sessions(expires_at);

CREATE TABLE nodes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL COLLATE NOCASE UNIQUE,
    provider TEXT NOT NULL DEFAULT '',
    region TEXT NOT NULL DEFAULT '',
    adapter_type TEXT NOT NULL CHECK (
        adapter_type IN ('native_hysteria2', 'standalone_sing_box', 's_ui')
    ),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (
        status IN ('pending', 'online', 'stale', 'offline', 'degraded', 'disabled')
    ),
    status_reason TEXT NOT NULL DEFAULT '',
    desired_version INTEGER NOT NULL DEFAULT 1,
    applied_version INTEGER NOT NULL DEFAULT 0,
    agent_installation_id TEXT,
    agent_credential_hash BLOB,
    agent_version TEXT NOT NULL DEFAULT '',
    protocol_version INTEGER NOT NULL DEFAULT 0,
    os_name TEXT NOT NULL DEFAULT '',
    os_version TEXT NOT NULL DEFAULT '',
    architecture TEXT NOT NULL DEFAULT '',
    core_name TEXT NOT NULL DEFAULT '',
    core_version TEXT NOT NULL DEFAULT '',
    core_running INTEGER NOT NULL DEFAULT 0 CHECK (core_running IN (0, 1)),
    uptime_seconds INTEGER NOT NULL DEFAULT 0,
    cpu_percent REAL NOT NULL DEFAULT 0,
    memory_used_bytes INTEGER NOT NULL DEFAULT 0,
    memory_total_bytes INTEGER NOT NULL DEFAULT 0,
    disk_used_bytes INTEGER NOT NULL DEFAULT 0,
    disk_total_bytes INTEGER NOT NULL DEFAULT 0,
    network_rx_bps INTEGER NOT NULL DEFAULT 0,
    network_tx_bps INTEGER NOT NULL DEFAULT 0,
    load_1 REAL NOT NULL DEFAULT 0,
    load_5 REAL NOT NULL DEFAULT 0,
    load_15 REAL NOT NULL DEFAULT 0,
    last_seen_at INTEGER,
    last_applied_at INTEGER,
    archived_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE node_enrollment_tokens (
    id TEXT PRIMARY KEY,
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    token_hash BLOB NOT NULL UNIQUE,
    expires_at INTEGER NOT NULL,
    consumed_at INTEGER,
    bound_installation_id TEXT,
    bound_request_id TEXT,
    response_ciphertext BLOB,
    response_expires_at INTEGER,
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    created_by TEXT NOT NULL REFERENCES admins(id),
    created_at INTEGER NOT NULL
);

CREATE INDEX node_enrollment_node_idx ON node_enrollment_tokens(node_id);
CREATE INDEX node_enrollment_expiry_idx ON node_enrollment_tokens(expires_at);

CREATE TABLE node_snapshots (
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    canonical_json BLOB NOT NULL,
    sha256 BLOB NOT NULL,
    created_at INTEGER NOT NULL,
    superseded_at INTEGER,
    PRIMARY KEY (node_id, version)
);

CREATE TABLE node_metric_samples (
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    bucket_at INTEGER NOT NULL,
    cpu_percent REAL NOT NULL,
    memory_used_bytes INTEGER NOT NULL,
    memory_total_bytes INTEGER NOT NULL,
    disk_used_bytes INTEGER NOT NULL,
    disk_total_bytes INTEGER NOT NULL,
    network_rx_bps INTEGER NOT NULL,
    network_tx_bps INTEGER NOT NULL,
    load_1 REAL NOT NULL,
    load_5 REAL NOT NULL,
    load_15 REAL NOT NULL,
    sampled_at INTEGER NOT NULL,
    PRIMARY KEY (node_id, bucket_at)
);
