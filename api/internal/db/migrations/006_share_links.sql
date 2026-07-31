-- Share links: signed, expiring, revocable tokens for unauthenticated access to
-- a single GameServer's status and connection address, optionally with start capability.
--
-- The token itself is never stored; only its SHA-256 hash is persisted. A raw
-- token is returned exactly once at creation and is never recoverable afterwards.
-- Lookup is by hash index for O(1) equality; expiry and revocation are both
-- mandatory auditable checks. Unknown, expired, and revoked tokens are
-- indistinguishable to the caller.
CREATE TABLE share_links (
    id           TEXT PRIMARY KEY,
    namespace    TEXT NOT NULL,
    server_name  TEXT NOT NULL,
    created_by   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    can_start    INTEGER NOT NULL DEFAULT 0,
    token_hash   TEXT NOT NULL UNIQUE,
    expires_at   TEXT NOT NULL,
    revoked_at   TEXT,
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    last_used    TEXT
);

CREATE INDEX idx_share_links_token ON share_links(token_hash);
CREATE INDEX idx_share_links_server ON share_links(namespace, server_name);
