ALTER TABLE nodes ADD COLUMN adapter_status TEXT NOT NULL DEFAULT 'unknown'
    CHECK (adapter_status IN (
        'unknown', 'compatible', 'incompatible', 'unavailable', 'not_configured'
    ));
ALTER TABLE nodes ADD COLUMN adapter_version TEXT NOT NULL DEFAULT '';
ALTER TABLE nodes ADD COLUMN adapter_error_code TEXT NOT NULL DEFAULT '';
ALTER TABLE nodes ADD COLUMN adapter_last_probed_at INTEGER;
ALTER TABLE nodes ADD COLUMN adapter_last_discovered_at INTEGER;
ALTER TABLE nodes ADD COLUMN sui_target_inbound_ids TEXT NOT NULL DEFAULT '[]'
    CHECK (json_valid(sui_target_inbound_ids));

ALTER TABLE node_user_assignments ADD COLUMN management_mode TEXT NOT NULL DEFAULT 'managed'
    CHECK (management_mode IN ('read_only', 'managed'));
ALTER TABLE node_user_assignments ADD COLUMN remote_client_id INTEGER
    CHECK (remote_client_id IS NULL OR remote_client_id > 0);

CREATE UNIQUE INDEX node_user_assignments_sui_remote_unique
    ON node_user_assignments(node_id, remote_client_id)
    WHERE remote_client_id IS NOT NULL;

CREATE TABLE sui_discovered_inbounds (
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    remote_id INTEGER NOT NULL CHECK (remote_id > 0),
    tag TEXT NOT NULL,
    type TEXT NOT NULL,
    listen TEXT NOT NULL DEFAULT '',
    listen_port INTEGER NOT NULL CHECK (listen_port BETWEEN 0 AND 65535),
    observed_at INTEGER NOT NULL,
    PRIMARY KEY(node_id, remote_id)
);

CREATE TABLE sui_discovered_clients (
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    remote_id INTEGER NOT NULL CHECK (remote_id > 0),
    name TEXT NOT NULL,
    enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
    inbound_ids TEXT NOT NULL CHECK (json_valid(inbound_ids)),
    upload_bytes INTEGER NOT NULL CHECK (upload_bytes >= 0),
    download_bytes INTEGER NOT NULL CHECK (download_bytes >= 0),
    expires_at INTEGER NOT NULL DEFAULT 0 CHECK (expires_at >= 0),
    online INTEGER NOT NULL CHECK (online IN (0, 1)),
    client_group TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    observed_at INTEGER NOT NULL,
    PRIMARY KEY(node_id, remote_id)
);

CREATE INDEX sui_discovered_clients_node_name_idx
    ON sui_discovered_clients(node_id, name COLLATE NOCASE);
