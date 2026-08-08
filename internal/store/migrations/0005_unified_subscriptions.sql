ALTER TABLE nodes ADD COLUMN public_host TEXT NOT NULL DEFAULT '';
ALTER TABLE nodes ADD COLUMN public_port INTEGER NOT NULL DEFAULT 443
    CHECK (public_port BETWEEN 1 AND 65535);
ALTER TABLE nodes ADD COLUMN sni TEXT NOT NULL DEFAULT '';
ALTER TABLE nodes ADD COLUMN tls_insecure INTEGER NOT NULL DEFAULT 0
    CHECK (tls_insecure IN (0, 1));

CREATE TABLE subscription_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL DEFAULT '',
    token_hash BLOB NOT NULL UNIQUE CHECK (length(token_hash) = 32),
    token_prefix TEXT NOT NULL,
    allowed_formats TEXT NOT NULL,
    expires_at INTEGER,
    last_used_at INTEGER,
    revoked_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX subscription_tokens_user_idx
    ON subscription_tokens(user_id, created_at DESC);
CREATE INDEX subscription_tokens_expiry_idx
    ON subscription_tokens(expires_at)
    WHERE revoked_at IS NULL;
