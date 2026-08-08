-- hyfleet:foreign-keys-off
-- SQLite creates an implicit, non-droppable index for a column-level UNIQUE
-- constraint. Rebuild the parent table so archived names can be reused while
-- all child rows continue to reference the same node IDs.
CREATE TABLE nodes_rebuilt (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL COLLATE NOCASE,
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

INSERT INTO nodes_rebuilt SELECT * FROM nodes;
DROP TABLE nodes;
ALTER TABLE nodes_rebuilt RENAME TO nodes;

CREATE UNIQUE INDEX nodes_active_name_unique
    ON nodes(name COLLATE NOCASE)
    WHERE archived_at IS NULL;
