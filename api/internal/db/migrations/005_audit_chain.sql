-- Add a hash chain to audit_events so a DB-level UPDATE or DELETE against a
-- row is detectable: each row records the hash of the row before it
-- (prev_hash) and its own content hash (hash), computed by the API at
-- insert time (see api/internal/audit.Auditor.insertChained). Rows written
-- before this migration keep NULL in both columns — the chain simply starts
-- at the first row inserted after this ships; Verify treats that boundary as
-- a fresh genesis rather than a break.
ALTER TABLE audit_events ADD COLUMN prev_hash TEXT;
ALTER TABLE audit_events ADD COLUMN hash TEXT;
