-- Add a cluster dimension to user_role_bindings. Existing rows backfill to
-- 'local' (the default cluster) — NOT '*', which would silently grant every
-- existing binding on any second cluster registered later.
--
-- SQLite can't ALTER a PRIMARY KEY, so rebuild the table; the create/copy/
-- drop/rename sequence is portable to Postgres too. Nothing FK-references
-- this table, so the drop is clean on both drivers.
CREATE TABLE user_role_bindings_new (
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_name  TEXT NOT NULL REFERENCES roles(name) ON DELETE RESTRICT,
    cluster    TEXT NOT NULL DEFAULT 'local',
    namespace  TEXT NOT NULL DEFAULT '*',
    PRIMARY KEY (user_id, role_name, cluster, namespace)
);

INSERT INTO user_role_bindings_new(user_id, role_name, cluster, namespace)
    SELECT user_id, role_name, 'local', namespace FROM user_role_bindings;

DROP TABLE user_role_bindings;

ALTER TABLE user_role_bindings_new RENAME TO user_role_bindings;

CREATE INDEX idx_user_role_bindings_user ON user_role_bindings(user_id);
