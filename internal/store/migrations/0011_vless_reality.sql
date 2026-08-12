-- hyfleet:foreign-keys-off
-- Both legacy tables have CHECK constraints that cannot be widened in place.
CREATE TABLE nodes_rebuilt_vless_reality (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL COLLATE NOCASE,
    provider TEXT NOT NULL DEFAULT '',
    region TEXT NOT NULL DEFAULT '',
    adapter_type TEXT NOT NULL CHECK (
        adapter_type IN (
            'native_hysteria2', 'standalone_sing_box', 's_ui',
            'sing_box_vless_reality'
        )
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
    updated_at INTEGER NOT NULL,
    usage_enabled INTEGER NOT NULL DEFAULT 0 CHECK (usage_enabled IN (0, 1)),
    usage_available INTEGER NOT NULL DEFAULT 0 CHECK (usage_available IN (0, 1)),
    usage_outbox_batches INTEGER NOT NULL DEFAULT 0 CHECK (usage_outbox_batches >= 0),
    usage_error_code TEXT NOT NULL DEFAULT '',
    usage_sampled_at INTEGER,
    traffic_upload_bytes INTEGER NOT NULL DEFAULT 0 CHECK (traffic_upload_bytes >= 0),
    traffic_download_bytes INTEGER NOT NULL DEFAULT 0 CHECK (traffic_download_bytes >= 0),
    traffic_unattributed_bytes INTEGER NOT NULL DEFAULT 0 CHECK (traffic_unattributed_bytes >= 0),
    traffic_last_report_at INTEGER,
    online_users INTEGER NOT NULL DEFAULT 0 CHECK (online_users >= 0),
    online_connections INTEGER NOT NULL DEFAULT 0 CHECK (online_connections >= 0),
    online_unknown_users INTEGER NOT NULL DEFAULT 0 CHECK (online_unknown_users >= 0),
    online_sampled_at INTEGER,
    online_last_report_at INTEGER,
    public_host TEXT NOT NULL DEFAULT '',
    public_port INTEGER NOT NULL DEFAULT 443 CHECK (public_port BETWEEN 1 AND 65535),
    sni TEXT NOT NULL DEFAULT '',
    tls_insecure INTEGER NOT NULL DEFAULT 0 CHECK (tls_insecure IN (0, 1)),
    adapter_status TEXT NOT NULL DEFAULT 'unknown' CHECK (
        adapter_status IN (
            'unknown', 'compatible', 'incompatible', 'unavailable', 'not_configured'
        )
    ),
    adapter_version TEXT NOT NULL DEFAULT '',
    adapter_error_code TEXT NOT NULL DEFAULT '',
    adapter_last_probed_at INTEGER,
    adapter_last_discovered_at INTEGER,
    sui_target_inbound_ids TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(sui_target_inbound_ids)),
    hostname TEXT NOT NULL DEFAULT '',
    kernel_version TEXT NOT NULL DEFAULT '',
    cpu_cores INTEGER NOT NULL DEFAULT 0,
    swap_used_bytes INTEGER NOT NULL DEFAULT 0,
    swap_total_bytes INTEGER NOT NULL DEFAULT 0,
    disk_read_bytes_per_second INTEGER NOT NULL DEFAULT 0,
    disk_write_bytes_per_second INTEGER NOT NULL DEFAULT 0,
    network_rx_bytes_total INTEGER NOT NULL DEFAULT 0,
    network_tx_bytes_total INTEGER NOT NULL DEFAULT 0,
    tls_cert_fingerprint TEXT NOT NULL DEFAULT '',
    tls_public_key_sha256 TEXT NOT NULL DEFAULT ''
);

INSERT INTO nodes_rebuilt_vless_reality (
    id, name, provider, region, adapter_type, enabled, status, status_reason,
    desired_version, applied_version, agent_installation_id, agent_credential_hash,
    agent_version, protocol_version, os_name, os_version, architecture, core_name,
    core_version, core_running, uptime_seconds, cpu_percent, memory_used_bytes,
    memory_total_bytes, disk_used_bytes, disk_total_bytes, network_rx_bps,
    network_tx_bps, load_1, load_5, load_15, last_seen_at, last_applied_at,
    archived_at, created_at, updated_at, usage_enabled, usage_available,
    usage_outbox_batches, usage_error_code, usage_sampled_at, traffic_upload_bytes,
    traffic_download_bytes, traffic_unattributed_bytes, traffic_last_report_at,
    online_users, online_connections, online_unknown_users, online_sampled_at,
    online_last_report_at, public_host, public_port, sni, tls_insecure,
    adapter_status, adapter_version, adapter_error_code, adapter_last_probed_at,
    adapter_last_discovered_at, sui_target_inbound_ids, hostname, kernel_version,
    cpu_cores, swap_used_bytes, swap_total_bytes, disk_read_bytes_per_second,
    disk_write_bytes_per_second, network_rx_bytes_total, network_tx_bytes_total,
    tls_cert_fingerprint, tls_public_key_sha256
)
SELECT
    id, name, provider, region, adapter_type, enabled, status, status_reason,
    desired_version, applied_version, agent_installation_id, agent_credential_hash,
    agent_version, protocol_version, os_name, os_version, architecture, core_name,
    core_version, core_running, uptime_seconds, cpu_percent, memory_used_bytes,
    memory_total_bytes, disk_used_bytes, disk_total_bytes, network_rx_bps,
    network_tx_bps, load_1, load_5, load_15, last_seen_at, last_applied_at,
    archived_at, created_at, updated_at, usage_enabled, usage_available,
    usage_outbox_batches, usage_error_code, usage_sampled_at, traffic_upload_bytes,
    traffic_download_bytes, traffic_unattributed_bytes, traffic_last_report_at,
    online_users, online_connections, online_unknown_users, online_sampled_at,
    online_last_report_at, public_host, public_port, sni, tls_insecure,
    adapter_status, adapter_version, adapter_error_code, adapter_last_probed_at,
    adapter_last_discovered_at, sui_target_inbound_ids, hostname, kernel_version,
    cpu_cores, swap_used_bytes, swap_total_bytes, disk_read_bytes_per_second,
    disk_write_bytes_per_second, network_rx_bytes_total, network_tx_bytes_total,
    tls_cert_fingerprint, tls_public_key_sha256
FROM nodes;

DROP TABLE nodes;
ALTER TABLE nodes_rebuilt_vless_reality RENAME TO nodes;
CREATE UNIQUE INDEX nodes_active_name_unique
    ON nodes(name COLLATE NOCASE) WHERE archived_at IS NULL;

CREATE TABLE user_credentials_rebuilt_vless (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    protocol TEXT NOT NULL CHECK (protocol IN ('hysteria2', 'vless')),
    secret_ciphertext BLOB NOT NULL,
    verifier_sha256 BLOB NOT NULL CHECK (length(verifier_sha256) = 32),
    secret_fingerprint TEXT NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1 CHECK (key_version >= 1),
    state TEXT NOT NULL CHECK (state IN ('staged', 'applied', 'retired', 'revoked')),
    created_at INTEGER NOT NULL,
    applied_at INTEGER,
    retired_at INTEGER,
    revoked_at INTEGER
);

INSERT INTO user_credentials_rebuilt_vless (
    id, user_id, node_id, protocol, secret_ciphertext, verifier_sha256,
    secret_fingerprint, key_version, state, created_at, applied_at, retired_at,
    revoked_at
)
SELECT
    id, user_id, node_id, protocol, secret_ciphertext, verifier_sha256,
    secret_fingerprint, key_version, state, created_at, applied_at, retired_at,
    revoked_at
FROM user_credentials;

DROP TABLE user_credentials;
ALTER TABLE user_credentials_rebuilt_vless RENAME TO user_credentials;
CREATE INDEX user_credentials_user_node_idx ON user_credentials(user_id, node_id);

CREATE TABLE node_agent_capabilities (
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    capability TEXT NOT NULL CHECK (length(capability) BETWEEN 1 AND 64),
    reported_at INTEGER NOT NULL,
    PRIMARY KEY(node_id, capability)
);

CREATE TABLE node_vless_reality (
    node_id TEXT PRIMARY KEY REFERENCES nodes(id) ON DELETE CASCADE,
    handshake_server TEXT NOT NULL CHECK (length(handshake_server) BETWEEN 1 AND 255),
    handshake_server_port INTEGER NOT NULL DEFAULT 443
        CHECK (handshake_server_port BETWEEN 1 AND 65535),
    desired_key_generation INTEGER NOT NULL DEFAULT 1 CHECK (desired_key_generation >= 1),
    applied_key_generation INTEGER NOT NULL DEFAULT 0 CHECK (applied_key_generation >= 0),
    public_key TEXT NOT NULL DEFAULT '',
    short_id TEXT NOT NULL DEFAULT '',
    material_applied_version INTEGER NOT NULL DEFAULT 0 CHECK (material_applied_version >= 0),
    material_reported_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
