-- One-off cleanup of rows that belong to users who no longer exist.
--
-- modernc-sqlite runs with foreign_keys OFF, so the ON DELETE CASCADE
-- clauses on these tables never fired on that driver, and a user delete
-- left the rows below in place. db.Store.DeleteUser now removes them in the
-- same transaction as the user; this migration clears what earlier deletes
-- left. Share links are revoked rather than deleted, which keeps their audit
-- trail (revoked_at uses the RFC3339 UTC format shares.go writes and parses).
-- On Postgres the cascades already removed these rows, so every statement
-- matches nothing there.
DELETE FROM oidc_links WHERE user_id NOT IN (SELECT id FROM users);

DELETE FROM user_preferences WHERE user_id NOT IN (SELECT id FROM users);

DELETE FROM sessions WHERE user_id NOT IN (SELECT id FROM users);

DELETE FROM api_tokens WHERE user_id NOT IN (SELECT id FROM users);

DELETE FROM user_role_bindings WHERE user_id NOT IN (SELECT id FROM users);

UPDATE share_links SET revoked_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
    WHERE revoked_at IS NULL AND created_by NOT IN (SELECT id FROM users);
