-- Custom roles + granular permissions.
--
-- Roles are named sets of permissions; users are bound to roles per
-- namespace ('*' = cluster-wide). The three built-in roles are seeded
-- so their cluster-wide bindings reproduce the pre-existing
-- admin/operator/viewer matrix exactly, and every existing user gets a
-- ('*') binding to their current role on upgrade (backfill below) — so
-- behaviour is unchanged at migrate time.
--
-- NOTE: modernc-sqlite runs with foreign_keys OFF, so the REFERENCES /
-- ON DELETE clauses are enforced only under Postgres. The API layer is
-- authoritative for the lockout guards (can't delete an in-use role,
-- clean up bindings on user delete).

CREATE TABLE roles (
    name        TEXT PRIMARY KEY,
    description TEXT NOT NULL DEFAULT '',
    builtin     INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE role_permissions (
    role_name  TEXT NOT NULL REFERENCES roles(name) ON DELETE CASCADE,
    permission TEXT NOT NULL,
    PRIMARY KEY (role_name, permission)
);

CREATE TABLE user_role_bindings (
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_name  TEXT NOT NULL REFERENCES roles(name) ON DELETE RESTRICT,
    namespace  TEXT NOT NULL DEFAULT '*',
    PRIMARY KEY (user_id, role_name, namespace)
);

CREATE INDEX idx_user_role_bindings_user ON user_role_bindings(user_id);

INSERT INTO roles(name, description, builtin) VALUES
    ('admin', 'Full access to all resources, including users, roles, and global config.', 1),
    ('operator', 'Manage game servers, backups, templates, and schedules.', 1),
    ('viewer', 'Read-only access across the control panel.', 1);

INSERT INTO role_permissions(role_name, permission) VALUES
    ('admin', '*');

INSERT INTO role_permissions(role_name, permission) VALUES
    ('operator', 'servers:read'),
    ('operator', 'servers:write'),
    ('operator', 'servers:console'),
    ('operator', 'backups:read'),
    ('operator', 'backups:write'),
    ('operator', 'backups:restore'),
    ('operator', 'schedules:read'),
    ('operator', 'schedules:write'),
    ('operator', 'templates:read'),
    ('operator', 'templates:write'),
    ('operator', 'modules:read'),
    ('operator', 'destinations:read'),
    ('operator', 'cluster:read'),
    ('operator', 'roles:read');

INSERT INTO role_permissions(role_name, permission) VALUES
    ('viewer', 'servers:read'),
    ('viewer', 'backups:read'),
    ('viewer', 'schedules:read'),
    ('viewer', 'templates:read'),
    ('viewer', 'modules:read'),
    ('viewer', 'destinations:read'),
    ('viewer', 'cluster:read'),
    ('viewer', 'roles:read');

INSERT INTO user_role_bindings(user_id, role_name, namespace)
    SELECT id, role, '*' FROM users WHERE role IN ('admin', 'operator', 'viewer');
