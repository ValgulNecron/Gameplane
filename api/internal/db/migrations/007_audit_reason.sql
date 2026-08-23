-- Add a nullable structured-reason column to audit_events. Existing
-- best-effort audit writers (the generic request middleware) never set it
-- and it stays NULL for their rows; only call sites needing a
-- machine-readable reason beyond the HTTP status code (e.g. network
-- capture operations, see specs/003-network-capture-sidecar) populate it.
-- Append-only per convention: no backfill, no rewrite of existing rows.
ALTER TABLE audit_events ADD COLUMN reason TEXT;
