-- hyfleet:foreign-keys-off

CREATE TABLE alerts_new (
    id TEXT PRIMARY KEY,
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (
        type IN (
            'offline', 'degraded', 'core_down', 'usage_error',
            'sync_failed', 'sync_stuck', 'operation_failed',
            'traffic_quota_warning', 'traffic_quota_exhausted'
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

INSERT INTO alerts_new(
    id, node_id, type, severity, status, message, occurrence_count,
    first_seen_at, last_seen_at, acknowledged_by, acknowledged_at,
    resolved_at, created_at, updated_at
)
SELECT
    id, node_id, type, severity, status, message, occurrence_count,
    first_seen_at, last_seen_at, acknowledged_by, acknowledged_at,
    resolved_at, created_at, updated_at
FROM alerts;

DROP TABLE alerts;
ALTER TABLE alerts_new RENAME TO alerts;

CREATE UNIQUE INDEX alerts_active_type_unique
    ON alerts(node_id, type)
    WHERE resolved_at IS NULL;
CREATE INDEX alerts_status_last_seen_idx
    ON alerts(status, last_seen_at DESC);
