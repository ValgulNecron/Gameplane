-- Grant the captures:manage permission to the admin role only.
-- This permission controls all capture operations: enable/disable/start/stop/delete/download.
-- Only the admin role grants this permission per FR-005; the captures:manage catalog entry
-- will resolve its grantability model per T012.
INSERT INTO role_permissions(role_name, permission) VALUES
    ('admin', 'captures:manage');
