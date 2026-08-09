CREATE TABLE node_telemetry_snapshots (
    node_id TEXT PRIMARY KEY REFERENCES nodes(id) ON DELETE CASCADE,
    sampled_at INTEGER NOT NULL,
    processes_available INTEGER NOT NULL CHECK (processes_available IN (0, 1)),
    processes_error_code TEXT NOT NULL DEFAULT '' CHECK (length(processes_error_code) <= 64),
    processes_total INTEGER NOT NULL DEFAULT 0 CHECK (processes_total >= 0),
    processes_truncated INTEGER NOT NULL DEFAULT 0 CHECK (processes_truncated IN (0, 1)),
    processes_sampled_at INTEGER,
    processes_json BLOB NOT NULL DEFAULT '[]',
    services_available INTEGER NOT NULL CHECK (services_available IN (0, 1)),
    services_error_code TEXT NOT NULL DEFAULT '' CHECK (length(services_error_code) <= 64),
    services_total INTEGER NOT NULL DEFAULT 0 CHECK (services_total >= 0),
    services_truncated INTEGER NOT NULL DEFAULT 0 CHECK (services_truncated IN (0, 1)),
    services_sampled_at INTEGER,
    services_json BLOB NOT NULL DEFAULT '[]',
    received_at INTEGER NOT NULL
);

CREATE INDEX node_telemetry_received_idx ON node_telemetry_snapshots(received_at);
