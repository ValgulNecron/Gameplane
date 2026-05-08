CREATE TABLE users (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    username     TEXT NOT NULL UNIQUE,
    email        TEXT,
    display_name TEXT,
    pw_hash      TEXT,
    role         TEXT NOT NULL DEFAULT 'viewer',
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE sessions (
    token       TEXT PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    csrf_token  TEXT NOT NULL,
    expires_at  TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_sessions_user ON sessions(user_id);

CREATE TABLE oidc_links (
    user_id  INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    issuer   TEXT NOT NULL,
    subject  TEXT NOT NULL,
    email    TEXT,
    PRIMARY KEY (issuer, subject)
);

CREATE TABLE audit_events (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    ts         TEXT NOT NULL DEFAULT (datetime('now')),
    actor      TEXT NOT NULL,
    method     TEXT NOT NULL,
    path       TEXT NOT NULL,
    target     TEXT,
    status     INTEGER NOT NULL,
    ip         TEXT
);

CREATE INDEX idx_audit_ts ON audit_events(ts DESC);

CREATE TABLE api_tokens (
    token       TEXT PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    last_used   TEXT
);

CREATE INDEX idx_api_tokens_user ON api_tokens(user_id);
