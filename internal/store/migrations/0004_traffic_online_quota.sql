ALTER TABLE users ADD COLUMN traffic_limit_bytes INTEGER NOT NULL DEFAULT 0
    CHECK (traffic_limit_bytes >= 0);
ALTER TABLE users ADD COLUMN traffic_upload_bytes INTEGER NOT NULL DEFAULT 0
    CHECK (traffic_upload_bytes >= 0);
ALTER TABLE users ADD COLUMN traffic_download_bytes INTEGER NOT NULL DEFAULT 0
    CHECK (traffic_download_bytes >= 0);
ALTER TABLE users ADD COLUMN traffic_used_bytes INTEGER NOT NULL DEFAULT 0
    CHECK (traffic_used_bytes >= 0);
ALTER TABLE users ADD COLUMN quota_state TEXT NOT NULL DEFAULT 'unlimited'
    CHECK (quota_state IN ('unlimited', 'active', 'limited'));
ALTER TABLE users ADD COLUMN last_traffic_at INTEGER;
ALTER TABLE users ADD COLUMN expiry_enforced_at INTEGER;

ALTER TABLE node_user_assignments ADD COLUMN traffic_limit_bytes INTEGER NOT NULL DEFAULT 0
    CHECK (traffic_limit_bytes >= 0);
ALTER TABLE node_user_assignments ADD COLUMN traffic_upload_bytes INTEGER NOT NULL DEFAULT 0
    CHECK (traffic_upload_bytes >= 0);
ALTER TABLE node_user_assignments ADD COLUMN traffic_download_bytes INTEGER NOT NULL DEFAULT 0
    CHECK (traffic_download_bytes >= 0);
ALTER TABLE node_user_assignments ADD COLUMN traffic_used_bytes INTEGER NOT NULL DEFAULT 0
    CHECK (traffic_used_bytes >= 0);
ALTER TABLE node_user_assignments ADD COLUMN quota_state TEXT NOT NULL DEFAULT 'unlimited'
    CHECK (quota_state IN ('unlimited', 'active', 'limited'));
ALTER TABLE node_user_assignments ADD COLUMN last_traffic_at INTEGER;

ALTER TABLE nodes ADD COLUMN usage_enabled INTEGER NOT NULL DEFAULT 0
    CHECK (usage_enabled IN (0, 1));
ALTER TABLE nodes ADD COLUMN usage_available INTEGER NOT NULL DEFAULT 0
    CHECK (usage_available IN (0, 1));
ALTER TABLE nodes ADD COLUMN usage_outbox_batches INTEGER NOT NULL DEFAULT 0
    CHECK (usage_outbox_batches >= 0);
ALTER TABLE nodes ADD COLUMN usage_error_code TEXT NOT NULL DEFAULT '';
ALTER TABLE nodes ADD COLUMN usage_sampled_at INTEGER;
ALTER TABLE nodes ADD COLUMN traffic_upload_bytes INTEGER NOT NULL DEFAULT 0
    CHECK (traffic_upload_bytes >= 0);
ALTER TABLE nodes ADD COLUMN traffic_download_bytes INTEGER NOT NULL DEFAULT 0
    CHECK (traffic_download_bytes >= 0);
ALTER TABLE nodes ADD COLUMN traffic_unattributed_bytes INTEGER NOT NULL DEFAULT 0
    CHECK (traffic_unattributed_bytes >= 0);
ALTER TABLE nodes ADD COLUMN traffic_last_report_at INTEGER;
ALTER TABLE nodes ADD COLUMN online_users INTEGER NOT NULL DEFAULT 0
    CHECK (online_users >= 0);
ALTER TABLE nodes ADD COLUMN online_connections INTEGER NOT NULL DEFAULT 0
    CHECK (online_connections >= 0);
ALTER TABLE nodes ADD COLUMN online_unknown_users INTEGER NOT NULL DEFAULT 0
    CHECK (online_unknown_users >= 0);
ALTER TABLE nodes ADD COLUMN online_sampled_at INTEGER;
ALTER TABLE nodes ADD COLUMN online_last_report_at INTEGER;

CREATE TABLE traffic_batches (
    id TEXT PRIMARY KEY,
    node_id TEXT NOT NULL REFERENCES nodes(id),
    agent_installation_id TEXT NOT NULL,
    source_epoch TEXT NOT NULL,
    sequence INTEGER NOT NULL CHECK (sequence >= 1),
    sampled_at INTEGER NOT NULL,
    received_at INTEGER NOT NULL,
    item_count INTEGER NOT NULL CHECK (item_count >= 1),
    upload_bytes INTEGER NOT NULL CHECK (upload_bytes >= 0),
    download_bytes INTEGER NOT NULL CHECK (download_bytes >= 0),
    payload_sha256 BLOB NOT NULL CHECK (length(payload_sha256) = 32),
    UNIQUE(node_id, agent_installation_id, source_epoch, sequence)
);

CREATE INDEX traffic_batches_node_received_idx
    ON traffic_batches(node_id, received_at);

CREATE TABLE traffic_batch_items (
    batch_id TEXT NOT NULL REFERENCES traffic_batches(id) ON DELETE CASCADE,
    node_id TEXT NOT NULL REFERENCES nodes(id),
    user_id TEXT NOT NULL,
    upload_bytes INTEGER NOT NULL CHECK (upload_bytes >= 0),
    download_bytes INTEGER NOT NULL CHECK (download_bytes >= 0),
    disposition TEXT NOT NULL CHECK (
        disposition IN ('accounted', 'unknown_user', 'unassigned', 'archived_user')
    ),
    PRIMARY KEY(batch_id, user_id)
);

CREATE INDEX traffic_batch_items_user_idx
    ON traffic_batch_items(user_id, node_id);

CREATE TABLE traffic_totals (
    node_id TEXT NOT NULL REFERENCES nodes(id),
    user_id TEXT NOT NULL REFERENCES users(id),
    upload_bytes INTEGER NOT NULL DEFAULT 0 CHECK (upload_bytes >= 0),
    download_bytes INTEGER NOT NULL DEFAULT 0 CHECK (download_bytes >= 0),
    last_batch_id TEXT NOT NULL REFERENCES traffic_batches(id),
    updated_at INTEGER NOT NULL,
    PRIMARY KEY(node_id, user_id)
);

CREATE TABLE node_online_snapshots (
    node_id TEXT PRIMARY KEY REFERENCES nodes(id) ON DELETE CASCADE,
    agent_installation_id TEXT NOT NULL,
    snapshot_id TEXT NOT NULL,
    sampled_at INTEGER NOT NULL,
    received_at INTEGER NOT NULL,
    total_connections INTEGER NOT NULL CHECK (total_connections >= 0),
    known_users INTEGER NOT NULL CHECK (known_users >= 0),
    unknown_users INTEGER NOT NULL CHECK (unknown_users >= 0)
);

CREATE TABLE node_online_users (
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    connections INTEGER NOT NULL CHECK (connections >= 1),
    known_assignment INTEGER NOT NULL CHECK (known_assignment IN (0, 1)),
    sampled_at INTEGER NOT NULL,
    PRIMARY KEY(node_id, user_id)
);

CREATE INDEX node_online_users_user_idx
    ON node_online_users(user_id, sampled_at);

CREATE TABLE node_kick_targets (
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    generation INTEGER NOT NULL CHECK (generation >= 1),
    reason TEXT NOT NULL,
    requested_at INTEGER NOT NULL,
    PRIMARY KEY(node_id, user_id)
);
