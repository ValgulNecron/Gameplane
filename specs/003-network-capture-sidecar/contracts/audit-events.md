# Audit Events Contract for Network Captures

**Feature**: 003-network-capture-sidecar
**Phase**: 1 (Contracts)
**Status**: PLANNED — nothing described below has been built, run, or tested.

This contract replaces an earlier version of this file that invented a MySQL
`audit_events` table (`AUTO_INCREMENT`, `LONGTEXT payload`, a `result` enum
column). None of that exists. Everything here is checked against the real
schema and code, cited by file:line.

---

## 1. The real audit primitive

### 1.1 `Event` struct

`api/internal/audit/audit.go:727-736`:

```go
type Event struct {
    ID     int64  `json:"id"`
    TS     string `json:"ts"`
    Actor  string `json:"actor"`
    Method string `json:"method"`
    Path   string `json:"path"`
    Target string `json:"target,omitempty"`
    Status int    `json:"status"`
    IP     string `json:"ip,omitempty"`
}
```

There is no `payload`, no `result`, no `operation` field. `Status` is the
HTTP response status code (int), not a success/failure enum.

### 1.2 Table shape

`api/internal/db/migrations/001_init.sql` (audit_events block):

```sql
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
```

`api/internal/db/migrations/005_audit_chain.sql` later adds two columns for
tamper evidence: `prev_hash` and `hash`, computed by
`Auditor.insertChained` (`api/internal/audit/audit.go:348-393`) — every
insert chains to the previous row's hash, and the "head" anchor is upserted
in the same transaction via `writeConfigTx` into the `config` table
(`headConfigKey`, referenced at `audit.go:270`). Migrations are append-only
and run at startup. The directory today holds, in order: `001_init.sql`,
`002_config.sql`, `003_roles.sql`, `004_cluster_rbac.sql`,
`005_audit_chain.sql`, `006_share_links.sql` — six files, verified by
listing `api/internal/db/migrations/` directly rather than assumed. This
feature's new migration (§2) is therefore `007_`, not `007_` guessed from
an out-of-date recollection.

`id` is `INTEGER PRIMARY KEY AUTOINCREMENT` under SQLite / a Postgres
serial-equivalent under pgx — never MySQL `BIGINT AUTO_INCREMENT`, and there
is no `LONGTEXT` anywhere in this codebase's DB layer (that's a MySQL
type). Never restate those in future revisions of this doc.

### 1.3 How rows get written today

`Middleware` (`api/internal/audit/audit.go:579-657`) wraps every mutating
route once, in `api/cmd/main.go` (not re-derived here beyond confirming it's
mounted once, per the RBAC ground truth's description of `Middleware`
mounting). Per request it:

1. Runs the wrapped handler to completion (`next.ServeHTTP(rw, req)`,
   `audit.go:589`) — the response has **already been written** to the
   underlying `ResponseWriter` by this point, since `rw.WriteHeader`
   forwards immediately.
2. Resolves `actor` from the request-scoped `ActorHolder`
   (`auth.WithActorHolder`, `audit.go:586`, feeding `holder.Name()` /
   `auth.UserFromContext` at `audit.go:593-602`), falling back to
   `"anonymous"`.
3. Resolves `target` as `req.URL.Query().Get("name")` — **a query-string
   parameter, not a path parameter** (`audit.go:604`).
4. Calls `a.insertChained(...)` (`audit.go:632`) with a
   `context.WithoutCancel` context so a client disconnect can't punch a hole
   in the trail.
5. On insert failure, logs `slog.Warn("audit insert failed", ...)`
   (`audit.go:635-636`) and **does not fail the request** — by
   construction it can't, because the response was already sent in step 1.
   The stdout sink, webhook sink, and S3 sink (if configured) are best-effort
   fan-out after that, unaffected by the DB write's success or failure
   (`audit.go:638-654`).

Two consequences follow directly from this, and both matter for capture
endpoints:

- **The `target` auto-fill does not work for capture routes.** Per
  `contracts/rest-api.md`, every capture route puts the server name (and,
  where relevant, the capture ID) in the URL path or a `:verb` suffix — e.g.
  `POST /servers/{name}:capture-start`, `GET /servers/{name}:capture?id={id}`
  — never in a `?name=` query string. If capture handlers rely on the
  generic `Middleware` alone, `Target` lands empty for every capture event.
  Capture handlers MUST set the audit target themselves rather than depend
  on the generic query-string extraction.
- **The generic middleware writes *after* the response is already on the
  wire**, not before. That is fine for the existing best-effort model (an
  audit failure can never affect a response that's already been sent) but
  it is not what FR-006 asks for — see §4.
- **The generic middleware never runs for `GET`/`HEAD`/`OPTIONS` at all.**
  `shouldLog` (`audit.go:659-663`) returns `false` for those methods
  unconditionally, before any path-specific check. Every capture `GET`
  route (list, get-status, download) is therefore invisible to the generic
  middleware no matter what `Target` it would have produced — see §3.1 and
  §3.2 for how this contract resolves that against FR-006's mandate that
  download specifically be audited.

---

## 2. The FR-006 gap, and the decided fix

**FR-006** (spec.md:126): "Every capture operation MUST generate an audit
event recording: user identity, operation type (start/stop/download/delete),
target GameServer name, capture ID, timestamp, and result (success or
failure reason). Audit events MUST be written before the operation response
is returned to the user."

`Event` has exactly: `Actor`, `Method`, `Path`, `Target` (one string),
`Status` (one int), `TS`, `IP`. It has no field for "operation type" beyond
whatever `Method`+`Path` already imply, no field for "capture ID" distinct
from `Target`, and no field for a textual failure reason — only an HTTP
status code.

The gap is sharper than "one field is missing": **`Method`+`Path` alone
cannot distinguish enable-capture from disable-capture** if both shared a
route — they don't, under the routing this contract now uses (§3), but even
with distinct `:capture-enable`/`:capture-disable` suffixes, nothing in
`Method`, `Path`, or a bare HTTP `Status` code lets a reader of the audit
log recover *why* a start/stop/download/delete failed. A 409 for "capture
already running" and a 409 for "GameServer not found" are indistinguishable
from the status code alone.

### Decision: append-only migration adding a structured `reason` column

This is the chosen design, not one option among several. Add
`api/internal/db/migrations/007_audit_reason.sql`, the next file after the
six confirmed to exist today (§1.2), following the append-only convention
`005_audit_chain.sql` itself established (new columns, old rows keep
`NULL`, no backfill required):

```sql
-- Add a nullable structured-reason column to audit_events. Existing
-- best-effort audit writers (the generic request middleware) never set it
-- and it stays NULL for their rows; only call sites needing a
-- machine-readable reason beyond the HTTP status code (e.g. network
-- capture operations, see specs/003-network-capture-sidecar) populate it.
-- Append-only per convention: no backfill, no rewrite of existing rows.
ALTER TABLE audit_events ADD COLUMN reason TEXT;
```

`Event` gains one field:

```go
type Event struct {
    ID     int64  `json:"id"`
    TS     string `json:"ts"`
    Actor  string `json:"actor"`
    Method string `json:"method"`
    Path   string `json:"path"`
    Target string `json:"target,omitempty"`
    Status int    `json:"status"`
    IP     string `json:"ip,omitempty"`
    Reason string `json:"reason,omitempty"` // NEW — machine-readable, e.g. "capture_already_running"
}
```

`insertChained`'s signature, its `INSERT` statement, and `Page`/`Stream`'s
`SELECT` lists (`audit.go:745`, `797`) all need the extra column and
argument — this is a real, non-trivial change to `audit.go`, not a doc-only
one, and is called out here as required implementation work, not assumed
complete.

Operation type and capture ID are still not separate columns: they ride in
`Target` (composite, see §3) because adding a column per future caller's
bespoke identifier does not scale, and the existing schema already treats
`Target` as "the one freeform identifier for whatever this event is about"
(`Target string`, no further structure, used today for a bare server name).
`Reason` closes the one gap that's actually costly to work around by
encoding into `Target`: a *human/machine-readable cause*, independent of the
coarse HTTP status.

**Why this and not encoding everything into `Target`/`Path`**: a capture
audit trail exists specifically so an admin can answer "who captured what,
and did it work" without cross-referencing stdout logs; collapsing every
non-2xx outcome into "the HTTP status was 409" fails that job the first
time two different failure modes share a status code. The migration is
small, additive, and matches the project's own append-only convention
exactly as `005_audit_chain.sql` did.

### 2.1 Hash-chain interaction with `005_audit_chain.sql` (critical)

`005_audit_chain.sql` hash-chains every row: `insertChained` reads the
previous row's `hash` inside a transaction guarded by `chainMu`
(`audit.go:340-347`) and computes this row's `hash` from the full `Event`
it just built (`computeHash(prevHash, e)`, `audit.go:364`, over the `Event`
literal assembled at `audit.go:363`). This is integrity-sensitive code
shared by *every* audited operation, not just capture, so the new column
must be reasoned about explicitly rather than left implicit:

- **The `reason` column MUST participate in `computeHash`'s input.**
  Leaving it out of the hash while it is a security-relevant field would
  mean `Verify` (`audit.go:454`) reports a row as intact even if its
  `reason` text were altered after the fact — the chain would "verify" a
  tampered row. `computeHash` and the `Event` literal built inside
  `insertChained` (currently `Event{TS: ts, Actor: actor, Method: method,
  Path: path, Target: target, Status: status, IP: ip}`, `audit.go:363`)
  both need to carry `Reason` through before the hash is computed, not
  after.
- **Rows written before this migration ships have no `reason` value and
  were hashed without one** — their `hash` was computed over the seven-field
  `Event` this codebase has today, and that computation is exactly what
  `Verify` must keep reproducing for those rows. Concretely: `computeHash`
  must treat an absent/`NULL` `reason` the same way it treated the field's
  total absence before this migration (e.g. an empty-string placeholder in
  the hashed representation, matching what a zero-value `Reason` already
  produces in Go) — not a new, different placeholder — or every
  pre-migration row fails `Verify` retroactively the moment the hash
  function changes shape. This contract does not itself specify
  `computeHash`'s exact serialization (that lives in the function body,
  unread in this pass); it states the constraint the implementation must
  satisfy: **verifying a pre-migration row must reproduce the pre-migration
  hash bit-for-bit**, which in practice means the new field's zero-value
  encoding has to be indistinguishable, in `computeHash`'s input, from the
  field not existing at all.
- This requirement is called out here, not deferred, because it is easy to
  get backwards: an implementer adding `Reason` to the `Event` struct and
  then mechanically adding it to `computeHash` risks silently reformatting
  the hash input for all 6+ months of pre-existing rows, which would surface
  as a mass `Verify` failure with no actual tampering having occurred.

---

## 3. Per-operation audit rows

The six operations below are the exact routes specified in
`contracts/rest-api.md`, not the earlier `PATCH /servers/{name}` draft —
that route is matched by the `servers:write` catch-all
(`api/internal/rbac/rbac.go:184`) the `operator` role already holds
(granted at `api/internal/db/migrations/003_roles.sql:48`), which is
precisely the FR-005 hole `rest-api.md`'s routing rewrite exists to close.
Every operation now uses a fixed `:verb` suffix so the RBAC rule table can
match it ahead of that catch-all.

`Target` uses a composite encoding, matching `rest-api.md`'s own Audit
Event lines for each endpoint: **`{name}` alone when there is no capture ID
yet (enable/disable), or `{name}:{captureId}` once a capture ID exists** —
colon-separated, not slash-separated. A reader recovers the parts by
splitting on the first `:`: the part before is the GameServer name, the
part after (if present) is the NetworkCapture CR name (the capture ID; the
CRD's `.metadata.name`, e.g. `cap-8f7d3c1a` per `rest-api.md`'s examples).

`Method` and `Path` are the literal request method/path template from
`contracts/rest-api.md` (that document, not this one, is the source of
truth for exact routes; cited here only to show which row belongs to which
operation).

| Operation | Method | Path | Target | Status (success) | Status (failure, example) | Reason (failure, example) |
|---|---|---|---|---|---|---|
| Enable capture | POST | `/servers/{name}:capture-enable` | `{name}` | 200 | 403/404/409/500 | `already_enabled`, `terminating` |
| Disable capture | POST | `/servers/{name}:capture-disable` | `{name}` | 200 | 403/404/409 | `not_enabled` |
| Start capture | POST | `/servers/{name}:capture-start` | `{name}:{captureId}` (or `{name}` if creation failed before an ID was minted) | 202 | 400/403/404/409/500 | `capture_already_in_progress`, `invalid_filter` |
| Stop capture | POST | `/servers/{name}:capture-stop` | `{name}:{captureId}` | 200 | 403/404/409 | `capture_not_found`, `conflict` |
| Download capture | GET | `/servers/{name}:capture-file?id={id}` | `{name}:{id}` | 200 | 403/404/409/502 | `capture_expired`, `capture_not_completed` |
| Delete capture | DELETE | `/servers/{name}:capture?id={id}` | `{name}:{id}` | 200 | 403/404/409 | `capture_still_running` |

On success, `Reason` is empty (`""`, omitted via `omitempty`) — a present
`Status` in the 2xx range already says "it worked"; `Reason` exists only to
carry *why not* on failure. `Actor` is always the authenticated username
resolved the same way `Middleware` resolves it today
(`auth.WithActorHolder`/`auth.UserFromContext`, `audit.go:593-602`);
`IP` is the client IP the same way (`middleware.GetClientIP`, falling back
to `RemoteAddr`'s host, `audit.go:612-624`).

Exact enumerated failure-`Reason` strings are **UNVERIFIED** beyond the
examples above — they depend on error handling not yet written in the
capture handlers, which don't exist yet. This table fixes the *shape*
(`Target` encoding, one `Reason` string per failure class) so handler
authors have a contract to write against; it does not fix a closed
enumeration of every reason string, since that would require inventing
handler internals that haven't been designed.

### 3.1 List is a `GET`, but IS audited (per `rest-api.md`)

`GET /servers/{name}:captures` is not one of FR-006's named six operations
("start, stop, download, delete" plus the enable/disable pair from
FR-001/FR-005), but `rest-api.md`'s own Audit Event line for that route
states it is audited anyway, on the grounds that the route carries
`captures:manage` (not a read-only permission) rather than the generic
`servers:read`. Because `shouldLog` (`audit.go:659-663`) unconditionally
returns `false` for `GET`, the generic `Middleware` cannot produce that row
no matter what permission the route carries — the same explicit
handler-side write path required for download (§3.2) is required here too,
if list is to be audited at all. This contract records the requirement
`rest-api.md` states; whether list-auditing is worth the extra explicit
write on every listing call, versus scoping FR-006 strictly to its six
named operations, is a product decision outside this file's scope — it is
flagged here as a discrepancy between what FR-006 requires and what
`rest-api.md` currently promises, not silently resolved.

`GET /servers/{name}:capture?id={id}` (get single capture status) is
likewise a `GET` outside FR-006's six named operations and outside the
generic middleware's reach for the same reason. Unlike download, FR-006
does not name "get status" as a required audit event, so this contract
does **not** require an explicit handler-side write for it — it may be
left unaudited, or audited at the same cost/benefit call as list above.

### 3.2 Download MUST be audited via an explicit handler-side write

FR-006 explicitly lists "download" among the operations that must produce
an audit event, and download is a `GET`
(`GET /servers/{name}:capture-file?id={id}`, per `rest-api.md`). Since
`shouldLog` returns `false` for every `GET` before any path is even
inspected, the generic `Middleware` structurally cannot produce this row —
there is no path-based carve-out to add to `shouldLog` that would fix this
without also auditing every other `GET` route in the API, which is out of
scope for this feature.

**Resolution, in favor of FR-006**: the capture-file download handler MUST
call the same synchronous, exported audit-write path specified for
start/stop/enable/disable/delete in §4 — directly from handler code, not
through `Middleware` — immediately before streaming the response body (or
immediately before writing the error status, on failure). This is the one
`GET` route in this contract that gets a mandatory handler-side write
regardless of the list/get discussion in §3.1, because FR-006 names it
explicitly and the other two `GET` routes are not named.

---

## 4. Ordering guarantee vs. "must not fail" — how they coexist

FR-006: *"Audit events MUST be written before the operation response is
returned to the user."*

spec.md's edge case (spec.md:112): *"Capture operations should not fail if
audit logging is down, but a warning or notification should be raised."*

These two requirements are only in tension if "written before the response"
is read as "the write must succeed, and block the response, before anything
is returned" — that reading would mean an audit-DB outage takes down every
capture operation, which the edge case explicitly forbids.

The existing generic `Middleware` resolves an analogous tension by writing
*after* `next.ServeHTTP` returns (`audit.go:589, 632`) — the response is
already on the wire by the time the DB write happens or fails, so a DB
failure structurally cannot affect a response the client already received.
That satisfies "never fails the operation" perfectly but does not satisfy
FR-006's literal ordering.

**Recommendation**: capture handlers (all six operations, including
download and, if adopted, list — see §3.1/§3.2) perform the audit write
themselves, synchronously, immediately before calling
`w.WriteHeader`/writing the final response body — i.e. attempt-write-then-
respond rather than respond-then-write. Treat a write failure as non-fatal
to the operation outcome exactly as `Middleware` already does (`slog.Warn`,
continue) — the operation's real result (its actual `Status`) still gets
returned to the caller, just without a durable audit row for the write that
failed, and the warning is what lets an operator notice the trail has a
hole (mirroring `audit.go:633-634`'s own comment: "a dropped security-audit
write must not be silent"). This satisfies both requirements: the DB
insert is attempted before any response bytes are written (FR-006), and its
failure is caught and logged rather than surfaced to the capture caller as
an operation failure (the edge case).

This requires capture handlers to call a *synchronous* write path with the
same chained-hash semantics as `insertChained` — today that method is
unexported and only called from `Middleware` (`audit.go:348, 632`; no other
call site exists per `grep -n "^func (a \*Auditor)"`). Exposing a
synchronous, exported equivalent callable from handler code (not just from
the generic post-hoc middleware), and threading the new `Reason` parameter
through it (§2), is required implementation work this contract surfaces but
does not itself perform — flagged here as PLANNED, not built.

The generic `Middleware` may still run for the capture routes that are
`POST`/`DELETE` (for the stdout/webhook/S3 fan-out and RBAC's own coarse
"something happened here" signal), but its query-string-derived `Target`
will be empty for these routes (§1.3) — the handler-driven write above is
what actually satisfies FR-006, not the generic middleware pass. For the
`GET` routes (§3.1/§3.2), the generic `Middleware` never runs at all
(`shouldLog` returns `false` before it would), so the handler-driven write
is the *only* source of an audit row.

---

## 5. Error response bodies are plain text, not JSON

Capture-endpoint audit rows record `Status` as an HTTP status code; they do
not record a response body. For completeness (and because a prior revision
of this doc set assumed otherwise): `api/internal/httperr` writes error
bodies as **plain text** via `http.Error` (`httperr.go`'s `WriteCode`/
`Write` functions), and `rbac.Middleware` writes a literal plain string for
authorization failures before any handler runs — there is no
`{"code":"...","message":"..."}` or `{"error":"...","message":"..."}` JSON
envelope anywhere in this codebase's error path. This feature introduces no
new JSON error envelope; audit rows and error responses are governed by the
same "no JSON body" fact independently of each other.

---

## 6. Summary of required (not yet built) changes

Everything below is PLANNED, not implemented:

1. `api/internal/db/migrations/007_audit_reason.sql` — adds nullable
   `audit_events.reason` (§2), decided (not proposed).
2. `Event` struct (`audit.go:727-736`) gains `Reason string
   \`json:"reason,omitempty"\``.
3. `insertChained` (`audit.go:348`), the `Event` literal built inside it
   (`audit.go:363`), its `INSERT` statement, and `Page`/`Stream`'s
   `SELECT` column lists (`audit.go:745`, `797`), extended for the new
   column.
4. `computeHash`'s field coverage decision for `Reason` (§2.1), implemented
   so that pre-migration rows keep verifying against their original hash —
   not merely "stated," since §2.1 requires bit-for-bit reproducibility for
   rows written before this migration.
5. A new exported, synchronous write method on `Auditor` (name TBD by the
   implementer) that capture handlers — including the download handler,
   mandatorily (§3.2) — call directly, before writing their HTTP response,
   using the `Target` composite encoding in §3.
6. Handler-side audit calls for all six operations named in
   `contracts/rest-api.md`'s route table, at the exact `Method`/`Path`
   values in §3's table (not the retired `PATCH /servers/{name}` shape).

No claim is made that any of the above compiles, passes review, or has been
attempted.
