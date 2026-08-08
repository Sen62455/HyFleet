CREATE TABLE node_operations (
    id TEXT PRIMARY KEY,
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL CHECK (sequence >= 1),
    type TEXT NOT NULL CHECK (
        type IN ('probe_core', 'restart_core', 'tail_core_log', 'backup_config')
    ),
    status TEXT NOT NULL DEFAULT 'queued' CHECK (
        status IN ('queued', 'running', 'succeeded', 'failed', 'expired')
    ),
    retry_of TEXT REFERENCES node_operations(id) ON DELETE SET NULL,
    attempt INTEGER NOT NULL DEFAULT 1 CHECK (attempt >= 1),
    max_lines INTEGER NOT NULL DEFAULT 0 CHECK (max_lines BETWEEN 0 AND 200),
    output TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    rolled_back INTEGER NOT NULL DEFAULT 0 CHECK (rolled_back IN (0, 1)),
    requested_by TEXT NOT NULL REFERENCES admins(id),
    expires_at INTEGER NOT NULL,
    started_at INTEGER,
    completed_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE(node_id, sequence)
);

CREATE INDEX node_operations_node_created_idx
    ON node_operations(node_id, created_at DESC);
CREATE INDEX node_operations_pending_idx
    ON node_operations(node_id, sequence)
    WHERE status IN ('queued', 'running');
CREATE INDEX node_operations_retry_idx ON node_operations(retry_of);

CREATE TABLE config_backups (
    id TEXT PRIMARY KEY,
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    operation_id TEXT NOT NULL REFERENCES node_operations(id) ON DELETE CASCADE,
    local_path TEXT NOT NULL,
    sha256 TEXT NOT NULL CHECK (length(sha256) = 64),
    size_bytes INTEGER NOT NULL CHECK (size_bytes >= 0),
    created_at INTEGER NOT NULL,
    UNIQUE(operation_id)
);

CREATE INDEX config_backups_node_created_idx
    ON config_backups(node_id, created_at DESC);

CREATE TABLE alerts (
    id TEXT PRIMARY KEY,
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (
        type IN (
            'offline', 'degraded', 'core_down', 'usage_error',
            'sync_failed', 'sync_stuck', 'operation_failed'
        )
    ),
    severity TEXT NOT NULL CHECK (severity IN ('warning', 'critical')),
    status TEXT NOT NULL DEFAULT 'open' CHECK (
        status IN ('open', 'acknowledged', 'resolved')
    ),
    message TEXT NOT NULL,
    occurrence_count INTEGER NOT NULL DEFAULT 1 CHECK (occurrence_count >= 1),
    first_seen_at INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    acknowledged_by TEXT REFERENCES admins(id),
    acknowledged_at INTEGER,
    resolved_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX alerts_active_type_unique
    ON alerts(node_id, type)
    WHERE resolved_at IS NULL;
CREATE INDEX alerts_status_last_seen_idx
    ON alerts(status, last_seen_at DESC);
