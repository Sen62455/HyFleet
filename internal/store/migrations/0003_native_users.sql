CREATE TABLE users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL COLLATE NOCASE,
    display_name TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    expires_at INTEGER,
    archived_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX users_active_username_unique
    ON users(username COLLATE NOCASE)
    WHERE archived_at IS NULL;

CREATE TABLE user_credentials (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    protocol TEXT NOT NULL CHECK (protocol = 'hysteria2'),
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

CREATE INDEX user_credentials_user_node_idx
    ON user_credentials(user_id, node_id);

CREATE TABLE node_user_assignments (
    id TEXT PRIMARY KEY,
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    desired_credential_id TEXT NOT NULL REFERENCES user_credentials(id),
    applied_credential_id TEXT REFERENCES user_credentials(id),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    desired_version INTEGER NOT NULL DEFAULT 0 CHECK (desired_version >= 0),
    applied_version INTEGER NOT NULL DEFAULT 0 CHECK (applied_version >= 0),
    state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'applied', 'failed')),
    last_error_code TEXT NOT NULL DEFAULT '',
    last_error_message TEXT NOT NULL DEFAULT '',
    last_attempt_at INTEGER,
    applied_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE(node_id, user_id)
);

CREATE INDEX node_user_assignments_user_idx
    ON node_user_assignments(user_id);
