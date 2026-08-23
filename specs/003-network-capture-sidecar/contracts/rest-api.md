# Network Capture REST API Contract

**Feature**: 003-network-capture-sidecar
**Phase**: 1 (Contracts)
**Date**: 2026-08-23
**Status**: Proposal for Code Review — REVISED (error-body, status-code, and citation corrections)

> **Revision note (this pass)**: this revision corrects four things left wrong
> by the prior RBAC-mechanism rewrite, per explicit human decision:
> (1) every endpoint below invented a `{"code", "message"}` JSON error
> envelope that does not exist anywhere in this codebase — the real shape is
> plain text via `api/internal/httperr`, shown below; (2) the "capture
> feature disabled cluster-wide" response used 500, which is wrong for a
> deliberate configuration state — changed to 501, matching the existing
> `clusterOps.enabled` precedent; (3) the retention default drifted to 7
> days in places — reverted to the authoritative 24 hours; (4) this document
> and `contracts/audit-events.md` used two different separators
> (`{name}:{captureId}` here, `{name}/{captureId}` there) for the audit
> `Target` field — settled on the colon form, see "RBAC and Error Handling"
> → "Audit Target encoding" below. The RBAC mechanism section itself
> (rule-table ordering, migration, mandatory tests) was already verified
> correct in the prior pass and is preserved here with only citation
> corrections (see item 7 of the wave's task list — `rbac.go:237`,
> `003_roles.sql:43-44`, `rbac.go:153-224`).

This document specifies the REST API endpoints for capture operations on
GameServers. All endpoints require the new `captures:manage` permission,
which is granted to the built-in `admin` role only — never to `operator` or
`viewer`. All requests generate audit events per FR-006.

**Open FR-005 gap (must be ratified by the human, not silently assumed
closed)**: see "Grantability of `captures:manage`" under "RBAC and Error
Handling" below. Adding `captures:manage` to the permission catalog makes
it a valid key for `ValidPermission` (`api/internal/rbac/catalog.go:86-89`),
and nothing in the existing custom-role machinery
(`api/internal/handlers/roles.go`) refuses a specific catalog key — the only
structurally non-grantable permission is the `"*"` wildcard itself. So
FR-005's "admin-only" is enforced only as a *default seed*
(migration grants it to `admin` alone); any holder of `roles:manage` can
today mint a custom role carrying `captures:manage` and bind it to any
user. This contract does not invent a new non-grantable-permission
mechanism (none exists in the codebase to extend) — it flags the gap and
defers the decision to the human, per the task instructions for this wave.

---

## Overview

The capture API provides a RESTful interface for admins to manage packet
captures on GameServers:
- Enable/disable capture capability on a GameServer (opt-in, spec-level
  change; injects/schedules removal of an ephemeral capture container).
- Start a capture (create a NetworkCapture CRD, trigger sidecar).
- Stop a capture (mark as completed, clean up sidecar).
- List captures for a server.
- Get capture status.
- Download the captured pcap file.
- Delete an expired or failed capture.

Every endpoint below requires `captures:manage`. Requests from a caller who
lacks it receive HTTP 403 Forbidden — including an **operator**-role token,
which is the case that actually matters here (see RBAC section). All
responses (success and failure) are audited per FR-006.

---

## Error Response Shape (applies to every endpoint below)

**Correction (DECISION 3, this pass)**: the prior version of this contract
showed every error as a JSON body — `{"code": "...", "message": "..."}`.
That envelope does not exist anywhere in this codebase and must not be
implemented. The real, verified shape:

- `api/internal/httperr` (`httperr.go`) is the only error-writing surface
  used by handlers. `WriteCode` (`httperr.go:35-45`) and `Write`
  (`httperr.go:50-60`) both call `http.Error(w, msg, status)` — Go's
  stdlib helper, which writes `Content-Type: text/plain; charset=utf-8`
  and a body of exactly `msg + "\n"`. There is no JSON, no `code` field,
  no envelope of any kind.
- For a 4xx status, `msg` is `err.Error()` verbatim — by
  `httperr.go`'s own doc comment (lines 25-34), every existing 4xx caller
  has already built a message safe to show the caller. For a `>=500`
  status, `msg` is the generic `http.StatusText(status)` — the real error
  is logged server-side only, never echoed to the client
  (`httperr.go:36-40`).
- **The 403 case is special and does not go through `httperr` at all.**
  Every capture route in this contract is gated by the RBAC rule table, so
  a permission failure is rejected by `rbac.Middleware` **before any
  handler runs** — `rbac.go:81` and `rbac.go:132` both call
  `http.Error(w, "forbidden", http.StatusForbidden)` directly, with the
  literal string `"forbidden"`, not a handler-composed message. A
  capture handler never gets the chance to write its own 403 body for a
  permission denial; any "403 response body" shown per-endpoint below that
  differs from the literal string `forbidden` is describing a body that is
  **unreachable** for this feature and must not be implemented as such.

So, per-endpoint error tables below list **status code**, **condition**,
and a **plain-text message** (what `err.Error()` returns, i.e. what the
raw HTTP body is — no braces, no quoting beyond what appears literally in
the response). The 403 row is always the same across every endpoint:
status 403, body `forbidden` (plain text, written by the RBAC middleware,
not by the handler).

---

## Base Path and Routing Conventions

All endpoints are under the API root. Capture operations follow the
existing Kubernetes-style custom-action convention already used for
lifecycle verbs — e.g. `r.Post("/servers/{name}:start", ...)` and
`r.Post("/servers/{name}:wipe-data", ...)` in
`api/internal/handlers/lifecycle.go:40-45` — extended here to `:capture-*`
suffixes.

**Routing change from the prior version of this contract**: the prior draft
put "enable capture" on the generic `PATCH /servers/{name}` route, and put
list/get/download/delete under nested paths like
`GET /servers/{name}/captures/{id}`. Both are RBAC-unsafe under the real
rule table:

- `PATCH /servers/{name}` is the same route every other spec edit uses, and
  is already matched by the `servers:write` catch-all (`rbac.go:184`) that
  the operator role holds. It cannot be given a stricter permission without
  also re-gating ordinary server edits.
- `match` (`rbac.go:237`) resolves a rule from only three things: HTTP
  method, the path's **first** segment (`firstSegment`, `rbac.go:228-234`,
  which strips a trailing `:verb` but nothing else), and a fixed
  `prefix`/`suffix` string. It has no notion of a path template with a
  variable segment in the middle. `/servers/{name}/captures/{id}` has a
  variable component (`{id}`) between two fixed substrings, so no
  `prefix`/`suffix` combination can match it without also matching
  unrelated `/servers/{name}/...` paths whose middle segment isn't
  `captures`.

The fix applied here: every capture endpoint — including list, get, and
delete, which previously carried a variable capture ID as a path segment —
is moved onto a `:verb` suffix with the capture ID passed as a query
parameter instead. This keeps every capture path's suffix fixed and
matchable, following the same shape the lifecycle verbs already use. Start
and stop already used this shape in the prior draft and are unchanged.

---

## Endpoint: Enable Capture on GameServer

**Functional Requirement**: FR-001 (opt-in sidecar)
**Required permission**: `captures:manage`

### Request

```
POST /servers/{name}:capture-enable
```

No request body. (Changed from the prior `PATCH /servers/{name}` shape —
see "Routing Conventions" above.)

**Path Parameters**:
- `{name}` (string): GameServer name, resolved to a namespace the same way
  every other `/servers/*` route does (existing `scope.Resolve`, unchanged
  by this feature).

**Preconditions**:
- Caller holds `captures:manage`.
- GameServer must exist and not be terminating.

**Mechanism** (per the ephemeral-container decision — not open for
re-litigation): enabling capture patches the GameServer's
`pods/ephemeralcontainers` subresource to inject the capture sidecar into
the running pod. No game-container restart occurs. (The capture
`emptyDir` volume itself is pre-provisioned unconditionally on every game
pod's StatefulSet template — see DECISION 1's consequences, which apply to
this feature globally and are not repeated per endpoint. Enabling capture
does not add the volume; the volume is already present, opted in or not.)

### Response

**Success (200 OK)**:
```json
{
  "name": "my-game",
  "status": {
    "capture": {
      "ready": false,
      "activeCapture": null,
      "lastCaptureTime": null,
      "sidecarRestarts": 0
    }
  }
}
```

**`status.capture` fields** (canonical nested shape — matches the CRD's own
`status.capture.{...}` convention; there is no `status.captureReady` or
`status.captureMessage`):
- `ready` (boolean): `true` once the injected ephemeral container reaches
  Running. `false` immediately after enable while injection is in flight.
- `activeCapture` (string, nullable): capture ID of the currently running
  capture, or `null` if none.
- `lastCaptureTime` (RFC3339 timestamp, nullable): completion time of the
  most recent capture, or `null` if none has run.
- `sidecarRestarts` (integer): observed restart count of the ephemeral
  capture container (ephemeral containers cannot be restarted by
  Kubernetes itself, so a nonzero value here reflects the container
  exiting and Kubernetes leaving it in a terminal state, not an automatic
  restart).

**Error Responses**:

| Status | Condition | Response Body (plain text) |
|--------|-----------|---|
| **404** | GameServer does not exist | `not found` |
| **403** | Caller lacks `captures:manage` | `forbidden` (written by `rbac.Middleware`, not this handler) |
| **409** | Pod is terminating | `server is terminating; cannot enable capture` |
| **501** | Capture feature disabled cluster-wide | `capture feature is disabled on this cluster` |

**Capture-feature-disabled status code (this pass, item 2)**: the prior
draft used 500 here. A cluster operator deliberately not enabling the
capture feature is a configuration state, not a server failure — 500 is
reserved for unexpected internal errors (see "Error Response Shape" above:
`>=500` never echoes a caller-composed message, which would be the wrong
signal for a condition the caller can act on by asking the operator to
enable the feature). This mirrors an existing, directly analogous
precedent in this codebase: `clusterOps.enabled` gates cluster-join/leave
operations and returns exactly **501 Not Implemented** when disabled
(`api/internal/handlers/cluster_actions.go:49-55`,
`httperr.WriteCode(w, req, http.StatusNotImplemented, errors.New("cluster operations are not enabled (set clusterOps.enabled)"))`).
Capture-disabled-cluster-wide follows the same shape.

**Audit Event**: one row per `audit.Event` (`api/internal/audit/audit.go:727-736`
fields — `Actor`, `Method`, `Path`, `Target`, `Status`, `IP`; there is no
payload/result column) with `Method="POST"`,
`Path="/servers/{name}:capture-enable"`, `Target={name}`, `Status`=the HTTP
status returned.

---

## Endpoint: Disable Capture on GameServer

**Functional Requirement**: FR-001 (opt-in; disable does **not** remove a
running ephemeral container — see amendment below)
**Required permission**: `captures:manage`

### Request

```
POST /servers/{name}:capture-disable
```

No request body.

**Preconditions**:
- Caller holds `captures:manage`.
- GameServer must exist.
- Capture must currently be enabled on the server.

### Response

**Success (200 OK)**:
```json
{
  "name": "my-game",
  "status": {
    "capture": {
      "ready": false,
      "activeCapture": null,
      "lastCaptureTime": "2026-08-23T14:31:00Z",
      "sidecarRestarts": 0
    }
  }
}
```

**Response description (US2 acceptance scenario 4, amended)**: disabling
capture stops any active capture and clears the capability **immediately**
— no new capture can be started, and `ready` drops to `false` in this same
response. It does **not** remove the ephemeral container from the running
pod: Kubernetes provides no API to delete an ephemeral container once
injected. The container is only removed when the pod is next recreated
(next scheduling, rollout, or manual delete) — until then it remains
visible in `pod.status.ephemeralContainerStatuses`, idle. Callers must not
infer "container gone" from a 200 response here. (The pre-provisioned
capture `emptyDir` volume is never removed by disable, either — see
DECISION 1.)

**Error Responses**:

| Status | Condition | Response Body (plain text) |
|--------|-----------|---|
| **404** | GameServer does not exist | `not found` |
| **403** | Caller lacks `captures:manage` | `forbidden` (written by `rbac.Middleware`, not this handler) |
| **409** | Capture is already disabled | `capture is not enabled on this server` |

**Audit Event**: `Method="POST"`, `Path="/servers/{name}:capture-disable"`,
`Target={name}`, `Status`=HTTP status.

---

## Endpoint: Start a Capture

**Functional Requirement**: FR-001 (user-initiated capture), FR-002
(max-duration and max-size limits), FR-003 (filter validation at API
tier), FR-005 (admin-only — see the grantability gap flagged at the top of
this document), FR-006 (audit), FR-012 (one capture at a time).
**Required permission**: `captures:manage`

### Request

```
POST /servers/{name}:capture-start
Content-Type: application/json

{
  "filter": "tcp port 8080",
  "maxDurationSeconds": 300,
  "maxSizeBytes": 5368709120,
  "ttlSecondsAfterFinished": 86400
}
```

**Path Parameters**:
- `{name}` (string): GameServer name.

**Request Body Fields**:

| Field | Type | Required | Range/Validation | Description |
|-------|------|----------|---|---|
| `filter` | string | NO | Valid pcap-filter(7) syntax | Optional packet filter. If omitted, a default filter is applied (restricts to the game server's advertised ports). If provided, the API attempts to compile it before creating any CRD. |
| `maxDurationSeconds` | integer | YES | 1..3600 | Max capture runtime in seconds; capped by cluster default. |
| `maxSizeBytes` | integer | YES | 1..cluster-max | Max file size in bytes; capped by cluster default. |
| `ttlSecondsAfterFinished` | integer | NO | 0..cluster-max | Retention after completion; **defaults to 86400 (24 hours, FR-007)** and is capped by cluster config. |

**Retention default (DECISION 4, this pass)**: the default and the example
above were drifted to 7 days (`604800`) in an earlier revision without
authorization. `spec.md` FR-007 and its "Out of Scope (Architectural
Constraints Already Decided)" section fix the default retention at **24
hours (86400 seconds)**, and that section is explicitly closed to
re-discussion. Every occurrence in this contract has been reverted to
`86400` / 24 hours.

> **Dependency risk (unresolved, must be flagged to the reviewer, not
> silently assumed)**: filter compilation is intended to use
> `github.com/packetcap/go-pcap`'s `filter` package (`Compile`, confirmed
> to exist via the GitHub API). That module has **no tagged release** — the
> Go module proxy resolves it only to the pseudo-version
> `v0.0.0-20260731105150-c86974bbfbcd`. Pinning a filter-syntax validator to
> an untagged commit is a real supply-chain/stability risk (no semver
> guarantee, no changelog, upstream can force-push its history) and should
> be called out as an open risk in this contract, not presented as settled.

**Preconditions**:
- Caller holds `captures:manage`.
- GameServer must exist and have capture enabled (`ready` observed `true`
  is not required at request time, but the capability must be enabled).
- No other capture on this GameServer may have `phase=Running` (FR-012).
- All numeric fields within range; filter (if provided) compiles.

**Validation order**: (1) filter compiles, before any CRD is created; (2)
concurrency check — reject if `status.capture.activeCapture` is non-null or
any owned NetworkCapture has `phase=Running`; (3) TTL against cluster max
(24h default, see above).

### Response

**Success (202 Accepted)**:
```json
{
  "captureId": "cap-8f7d3c1a",
  "phase": "Pending",
  "serverName": "my-game",
  "filter": "tcp port 8080",
  "maxDurationSeconds": 300,
  "maxSizeBytes": 5368709120,
  "ttlSecondsAfterFinished": 86400,
  "createdAt": "2026-08-23T14:30:00Z",
  "startedAt": null,
  "completedAt": null,
  "bytesWritten": 0,
  "packetsWritten": 0
}
```

**Status Code**: 202 Accepted, not 201 — the capture starts `Pending` and
the operator reconciler transitions it to `Running`.

**Error Responses**:

| Status | Condition | Response Body (plain text) |
|--------|-----------|---|
| **404** | GameServer does not exist | `not found` |
| **400** | Capture not enabled on server | `capture is not enabled on this server` |
| **400** | Filter expression is invalid (FR-003) | `invalid filter: <syntax error detail>` |
| **400** | TTL exceeds cluster max (24h default) | `requested retention 90000s exceeds cluster maximum 86400s` |
| **400** | `maxDurationSeconds` out of range | `maxDurationSeconds must be 1..3600` |
| **400** | `maxSizeBytes` out of range | `maxSizeBytes must be 1..<cluster-max>` |
| **403** | Caller lacks `captures:manage` (FR-005) | `forbidden` (written by `rbac.Middleware`, not this handler) |
| **409** | Capture already in progress (FR-012) | `capture already in progress on this server` |
| **501** | Capture feature disabled cluster-wide | `capture feature is disabled on this cluster` (see the Enable-endpoint's 501 rationale above) |

**Audit Event**: `Method="POST"`, `Path="/servers/{name}:capture-start"`,
`Target={name}:{captureId}` (or `{name}` if creation failed before an ID
was minted), `Status`=HTTP status. See "Audit Target encoding" below for
the separator convention.

---

## Endpoint: Stop a Capture

**Functional Requirement**: FR-001, FR-006 (audit).
**Required permission**: `captures:manage`

### Request

```
POST /servers/{name}:capture-stop
Content-Type: application/json

{
  "captureId": "cap-8f7d3c1a"
}
```

**Request Body Fields**:
- `captureId` (string, required): the capture to stop.

**Preconditions**:
- Caller holds `captures:manage`.
- GameServer and the named capture must exist and belong to this server.
- Capture must be `Pending` or `Running`.

### Response

**Success (200 OK)**:
```json
{
  "captureId": "cap-8f7d3c1a",
  "phase": "Completed",
  "serverName": "my-game",
  "filter": "tcp port 8080",
  "createdAt": "2026-08-23T14:30:00Z",
  "startedAt": "2026-08-23T14:30:05Z",
  "completedAt": "2026-08-23T14:31:00Z",
  "stoppingReason": "user_requested",
  "bytesWritten": 524288,
  "packetsWritten": 1024
}
```

**`stoppingReason`**: one of `user_requested`, `max_duration_reached`,
`max_size_reached`, `pod_restarted`, `error`.

**Error Responses**:

| Status | Condition | Response Body (plain text) |
|--------|-----------|---|
| **404** | GameServer does not exist | `not found` |
| **404** | Capture does not exist, or belongs to a different server | `capture 'cap-...' not found` |
| **403** | Caller lacks `captures:manage` | `forbidden` (written by `rbac.Middleware`, not this handler) |
| **409** | Capture is already stopped | `capture is not running` |

**Audit Event**: `Method="POST"`, `Path="/servers/{name}:capture-stop"`,
`Target={name}:{captureId}`, `Status`=HTTP status.

---

## Endpoint: List Captures for a Server

**Functional Requirement**: FR-001, FR-007 (listing excludes expired
captures).
**Required permission**: `captures:manage`

### Request

```
GET /servers/{name}:captures
```

(Changed from the prior `GET /servers/{name}/captures` — see "Routing
Conventions" above; the fixed `:captures` suffix is required for the RBAC
rule to match this route and not the `servers:read` catch-all.)

**Query Parameters** (all optional):
- `phase` (string, enum): `Pending`, `Running`, `Completed`, `Failed`.
- `limit` (integer, default 100).
- `offset` (integer, default 0).

### Response

**Success (200 OK)**:
```json
{
  "captures": [
    {
      "captureId": "cap-8f7d3c1a",
      "phase": "Completed",
      "serverName": "my-game",
      "filter": "tcp port 8080",
      "createdAt": "2026-08-23T14:30:00Z",
      "startedAt": "2026-08-23T14:30:05Z",
      "completedAt": "2026-08-23T14:31:00Z",
      "bytesWritten": 524288,
      "packetsWritten": 1024,
      "expiresAt": "2026-08-24T14:31:00Z"
    }
  ],
  "total": 1,
  "limit": 100,
  "offset": 0
}
```

Note: `expiresAt` above is `completedAt` + 24 hours (the FR-007 default
retention, DECISION 4), not 7 days as an earlier drifted revision showed.

**Expired Capture Handling (FR-007)**: captures outside the retention
window are not returned; the operator's retention reconciler deletes them
in the background.

**Error Responses**:

| Status | Condition | Response Body (plain text) |
|--------|-----------|---|
| **404** | GameServer does not exist | `not found` |
| **403** | Caller lacks `captures:manage` | `forbidden` (written by `rbac.Middleware`, not this handler) |

**Audit Event**: `Method="GET"`, `Path="/servers/{name}:captures"`,
`Target={name}`, `Status`=HTTP status. See "Download auditing and the GET
gap" below — this is one of the GET routes the generic audit middleware
cannot log, so an explicit handler-side write is required here too, not
only for download.

---

## Endpoint: Get Capture Status

**Functional Requirement**: FR-001, FR-007 (404 for expired captures).
**Required permission**: `captures:manage`

### Request

```
GET /servers/{name}:capture?id={id}
```

(Changed from `GET /servers/{name}/captures/{id}` — the capture ID moves
to a query parameter so the path's suffix, `:capture`, stays fixed and
matchable. See "Routing Conventions".)

**Query Parameters**:
- `id` (string, required): capture ID, e.g. `cap-8f7d3c1a`.

### Response

**Success (200 OK)**:
```json
{
  "captureId": "cap-8f7d3c1a",
  "phase": "Running",
  "serverName": "my-game",
  "filter": "tcp port 8080",
  "maxDurationSeconds": 300,
  "maxSizeBytes": 5368709120,
  "createdAt": "2026-08-23T14:30:00Z",
  "startedAt": "2026-08-23T14:30:05Z",
  "completedAt": null,
  "bytesWritten": 12288,
  "packetsWritten": 32,
  "expiresAt": "2026-08-24T14:31:00Z"
}
```

**Error Responses**:

| Status | Condition | Response Body (plain text) |
|--------|-----------|---|
| **404** | GameServer does not exist | `not found` |
| **404** | Capture does not exist, belongs to a different server, or has expired (FR-007) | `capture 'cap-...' not found or has expired` |
| **400** | `id` query parameter missing | `id is required` |
| **403** | Caller lacks `captures:manage` | `forbidden` (written by `rbac.Middleware`, not this handler) |

**Audit Event**: `Method="GET"`, `Path="/servers/{name}:capture"`,
`Target={name}:{id}`, `Status`=HTTP status. Same GET-audit gap as List
above — see "Download auditing and the GET gap".

---

## Endpoint: Download Capture File

**Functional Requirement**: FR-004 (valid pcap/pcapng, readable by
standard tools), FR-006 (audit — explicitly names download as an operation
that MUST be audited), FR-007 (404 for expired captures), FR-011 (filtered
packets only).
**Required permission**: `captures:manage`

### Request

```
GET /servers/{name}:capture-file?id={id}
Accept: application/octet-stream
```

**Query Parameters**:
- `id` (string, required): capture ID.

**Preconditions**:
- Capture must exist, belong to this server, be `Completed`, and not have
  expired.

**UNVERIFIED — internal proxy path**: the prior draft split this into a
small-file path proxied through the agent and a large-file path (`>64
MiB`) proxied via a second route under `ws.Mount`
(`api/cmd/main.go:281`). That split was not re-verified against the real
`ws` package during this revision — `ws.Mount`'s registered routes were
not read, so whether it supports a plain (non-websocket-upgrade) binary
stream is unconfirmed. This contract therefore specifies **one** route for
all capture sizes; whether the handler internally proxies through the
agent or streams from the sidecar is an implementation choice, not a
second RBAC-relevant route.

One half of that choice is, however, already closed by evidence. If
proxying through the agent is chosen, it CANNOT reuse the agent's file
browser: that browser is rooted at exactly one path (`--data-root`) and
rejects any resolved path outside it, so a capture volume mounted as a
second root would be unreachable to it — see the doc comment at
`operator/internal/controller/gameserver_rcon.go:111-119`, which states
that serving a second location "requires teaching the agent to serve
multiple roots, not just adding a VolumeMount here". Proxying therefore
means a dedicated agent endpoint, not the existing files surface. See
`capture-sidecar.md` ("Addressing"), which specifies the path: the
existing `<gs>-agent` ClusterIP Service, given a second, numerically
targeted port 9091, dialed by its DNS name over mTLS — a name the
agent cert's SANs already cover. Before implementation, verify against
`api/internal/ws/` whether a second route is actually needed, and if so
add a corresponding `{method: "GET", segment: "ws", suffix:
":capture-file", perm: "captures:manage"}` rule ahead of the existing `ws`
catch-all at `rbac.go:219`.

### Response (Success)

```
HTTP/1.1 200 OK
Content-Type: application/vnd.tcpdump.pcap
Content-Disposition: attachment; filename="capture-cap-8f7d3c1a.pcapng"
Content-Length: 524288

[Binary PCAPNG file data]
```

**Response Headers**:
- `Content-Type: application/vnd.tcpdump.pcap`
- `Content-Disposition: attachment; filename="capture-{captureId}.pcapng"`
- `Content-Length`: exact file size in bytes.

**Response Body**: binary PCAPNG (the sidecar writes via
`pcapgo.NgWriter` per research.md), containing only packets matching the
capture's filter (FR-011).

### Error Responses

| Status | Condition | Response Body (plain text) |
|--------|-----------|---|
| **404** | GameServer does not exist | `not found` |
| **404** | Capture not found, wrong server, or expired | `capture 'cap-...' not found or has expired` |
| **409** | Capture is not `Completed` | `capture is still running or has failed` |
| **403** | Caller lacks `captures:manage` | `forbidden` (written by `rbac.Middleware`, not this handler) |
| **502** | Sidecar/agent unreachable | `capture sidecar is unreachable` |

### Download auditing and the GET gap

**This section must agree with `contracts/audit-events.md` — it previously
did not.** `audit.shouldLog` (`api/internal/audit/audit.go:659-663`)
returns `false` for `GET`/`HEAD`/`OPTIONS`, so the generic per-request
audit `Middleware` **never fires** for this route, or for the List/Get GET
routes above. FR-006 nonetheless explicitly requires download to be
audited ("Every capture operation MUST generate an audit event ... audit
events MUST be written before the operation response is returned").

Those two facts are only reconcilable one way: **the download handler
must call an explicit, audited write itself**, not rely on the generic
middleware. Today no such call site exists — `insertChained`
(`api/internal/audit/audit.go:348`) is unexported, and nothing outside
`audit.go` calls it (verified: no `Auditor.Record`/`Auditor.Log`-shaped
exported method exists, and no handler package imports `audit` to write
an event directly today). This is real, uncompleted implementation work,
not something this contract can describe as already covered:

- `audit` must gain a new exported method (e.g. `Auditor.RecordExplicit`
  or similarly named — exact name is an implementation choice for
  capture-sidecar.md / the eventual PR, not fixed by this contract) that
  wraps `insertChained` for callers outside the package.
- The download handler calls it **before** writing the response body to
  the client, satisfying FR-006's "before the operation response is
  returned" — this is the one capture route where that ordering is not
  automatically satisfied by the generic middleware's after-the-fact model
  (see `contracts/audit-events.md` §1.3 on the generic middleware writing
  *after* the response is already on the wire).
- The same explicit-write requirement applies to the List and Get
  endpoints above, for the same `shouldLog` reason — they are noted at
  their own Audit Event lines rather than repeated in full here.

**Audit Event**: `Method="GET"`, `Path="/servers/{name}:capture-file"`,
`Target={name}:{id}`, `Status`=HTTP status — written by the explicit
handler-side call described above, not the generic middleware.

---

## Endpoint: Delete a Capture

**Functional Requirement**: FR-001, FR-006 (audit), FR-007 (can delete
expired captures).
**Required permission**: `captures:manage`

### Request

```
DELETE /servers/{name}:capture?id={id}
```

**Preconditions**:
- Capture must exist and belong to this server.
- Capture must be `Completed`, `Failed`, or `Expired` (a running capture
  must be stopped first).

### Response

**Success (200 OK)**:
```json
{
  "deleted": true,
  "captureId": "cap-8f7d3c1a"
}
```

**Error Responses**:

| Status | Condition | Response Body (plain text) |
|--------|-----------|---|
| **404** | GameServer does not exist | `not found` |
| **404** | Capture does not exist or belongs to a different server | `capture 'cap-...' not found` |
| **403** | Caller lacks `captures:manage` | `forbidden` (written by `rbac.Middleware`, not this handler) |
| **409** | Capture is running | `cannot delete a running capture; stop it first` |

**Audit Event**: `Method="DELETE"`, `Path="/servers/{name}:capture"`,
`Target={name}:{id}`, `Status`=HTTP status. `DELETE` is a mutating method,
so `shouldLog` returns `true` and the generic middleware covers this route
— unlike List/Get/Download above, no explicit handler-side write is
required here.

---

## RBAC and Error Handling

### The real mechanism (not `rbac.RequireRole`)

Authorization is a first-match-wins path-pattern rule table
(`rbac.rules`, `api/internal/rbac/rbac.go:153-224`). Each incoming request
is matched by `match(method, path)` (`rbac.go:237`) against `{method,
segment, prefix, suffix}` tuples, in table order; the first rule whose
fields all match wins, and its `perm` is the permission checked via
`u.Can` (`rbac.go:364`, backed by `api/internal/auth/perms.go:93`). There
is no per-route decorator, no `RequireRole` call site anywhere in the
handlers — the table *is* the enforcement.

`firstSegment` (`rbac.go:228-234`) strips only a trailing `:verb` from the
**first** path segment; it does not know anything about later segments.
Every route below `/servers/...` — including every capture route in this
contract — therefore has `segment == "servers"`, identical to ordinary
server CRUD. Two existing rules already claim that segment:

```go
// rbac.go:183-184 (existing, unchanged)
{method: "GET", segment: "servers", perm: "servers:read"},
{segment: "servers", perm: "servers:write"},
```

`servers:write` is granted to `operator`
(`api/internal/db/migrations/003_roles.sql:48`). If capture routes are
added anywhere *after* these two lines without a more specific match of
their own, `match` never reaches a capture-specific rule — the generic
servers rule wins first, and every capture route silently becomes
available to any `operator`-role token. This is the exact hole FR-005 and
SC-005 must not ship with, and it is **operator-shaped**, not
viewer-shaped: a viewer holds neither `servers:read` at write time nor
`servers:write`, so a viewer-only test would pass even with the hole
present. **A test asserting an `operator`-role token gets 403 on every
capture route is mandatory; a "non-admin" or viewer-only test is not
sufficient coverage for FR-005.**

### New permission

Add to `api/internal/rbac/catalog.go` (entry shape per `catalog.go:27`,
inside `Catalog`):

```go
{Resource: "captures", Label: "Network captures", Permissions: []Permission{
    {Key: "captures:manage", Label: "Enable, start, stop, download, and delete packet captures", Namespaced: true},
}},
```

`Namespaced: true` because captures are scoped to a GameServer within a
namespace, matching `servers:*`'s shape.

### Grantability of `captures:manage`

**This is an FR-005 amendment that the human must ratify, not a settled
design choice.** Adding the entry above makes `captures:manage` pass
`ValidPermission` (`api/internal/rbac/catalog.go:86-89`), which is the only
gate `api/internal/handlers/roles.go` applies when a `roles:manage` holder
creates or edits a custom role. The catalog's own doc comment
(`catalog.go:4-5`) states only the `"*"` wildcard is structurally
non-grantable; there is no existing per-permission "admin-only, never
custom-grantable" mechanism to reuse, and this contract does not invent
one.

Concretely, FR-005 ("Only admin can access capture endpoints (per-server
admin or global admin)") holds **only as a default seed**: the migration
below grants `captures:manage` to `admin` and nobody else, so out of the
box no `operator`/`viewer` token has it. But it is not enforced as an
invariant — any `roles:manage` holder can mint a custom role carrying
`captures:manage` and bind it to any user, at which point that user has
full capture access without being `admin`. Two ways forward, left for the
human to choose (not chosen here):

1. Accept the weaker reading of FR-005 — "holders of `captures:manage`,
   which is granted to `admin` by default" — and amend FR-005's text to
   say that explicitly, so the spec stops implying a stronger guarantee
   the code does not provide.
2. Extend the RBAC package with an actual non-grantable-permission
   mechanism (e.g. a `Grantable bool` field on `Permission`, checked by
   `ValidPermission` or by `roles.go`'s create/update path, with a test
   enforcing that `captures:manage` cannot appear in a custom role's
   grant set) — a real code change beyond this contract's scope, called
   out here rather than assumed.

### New migration (append-only; `api/internal/db/migrations/007_captures_rbac.sql`)

Grants `captures:manage` to `admin` only:

```sql
INSERT INTO role_permissions(role_name, permission) VALUES
    ('admin', 'captures:manage');
```

**Migration-number coordination note**: `api/internal/db/migrations/`
currently ends at `006_share_links.sql`, so `007_` is the next free
number as of this revision. `contracts/audit-events.md` (DECISION 2, a
sibling contract in this same feature) separately proposes its own
`007_audit_reason.sql` for the structured audit-reason column. Both
proposals cannot both be `007`; whichever migration is implemented second
must renumber to the next free integer at implementation time. This
contract does not resolve the collision unilaterally since it does not own
`audit-events.md`.

Note: the built-in `admin` role already holds the `*` wildcard
(`003_roles.sql:43-44`), which `u.Can` treats as matching any permission
(`perms.go:91-92`) — so admin has effective access to captures routes even
without this row. The explicit grant is still added for two reasons: (1)
`catalog.go`'s own doc comment requires every RBAC-referenced key to be
cross-checked against seeded roles in tests, and an explicit row is what
that cross-check exercises; (2) it keeps the grant legible without relying
on a reader knowing the wildcard's semantics. **Do not** add a
corresponding row for `operator` or `viewer` — that is precisely the grant
this feature must not make (subject to the grantability gap noted above,
which is about *custom* roles, not this seed migration).

### New rule-table entries (`api/internal/rbac/rbac.go`)

Every capture rule MUST be inserted **before** line 183
(`{method: "GET", segment: "servers", perm: "servers:read"}`) for the GET
routes, and before line 184
(`{segment: "servers", perm: "servers:write"}`) for the POST/DELETE
routes — both, since either existing rule would otherwise shadow a
capture rule placed after it:

```go
// Network capture (feature 003-network-capture-sidecar): admin-only via
// captures:manage. MUST precede the servers:read rule (next block) and
// the servers:write catch-all (below) — firstSegment makes every
// /servers/{name}:verb path match segment "servers" same as ordinary
// server CRUD, so an unordered insertion here falls through to
// servers:write, which the operator role already holds.
{method: "POST", segment: "servers", suffix: ":capture-enable", perm: "captures:manage"},
{method: "POST", segment: "servers", suffix: ":capture-disable", perm: "captures:manage"},
{method: "POST", segment: "servers", suffix: ":capture-start", perm: "captures:manage"},
{method: "POST", segment: "servers", suffix: ":capture-stop", perm: "captures:manage"},
{method: "GET", segment: "servers", suffix: ":captures", perm: "captures:manage"},
{method: "GET", segment: "servers", suffix: ":capture-file", perm: "captures:manage"},
{method: "GET", segment: "servers", suffix: ":capture", perm: "captures:manage"},
{method: "DELETE", segment: "servers", suffix: ":capture", perm: "captures:manage"},

{method: "GET", segment: "servers", perm: "servers:read"},   // rbac.go:183, unchanged, now after the above
{segment: "servers", perm: "servers:write"},                  // rbac.go:184, unchanged, now after the above
```

No suffix collision: `strings.HasSuffix` requires the *entire* trailing
substring to match, so `:capture` does not match a path ending in
`:capture-enable`, `:capture-disable`, `:capture-start`, `:capture-stop`,
or `:capture-file` — each of those ends in a longer, distinct suffix.
Placing `:capture` after the more specific `:capture-*` rules is done here
for readability only; it is not required for correctness given
`HasSuffix`'s exact-tail semantics, but keep them ordered that way so a
future reader isn't left to re-derive it.

### Audit Target encoding

**Settled in this pass**: this contract uses a **colon** separator for the
composite `Target` field wherever a capture ID accompanies the server
name — `{name}:{captureId}`, e.g. `my-game:cap-8f7d3c1a` — consistently
across every endpoint above. `contracts/audit-events.md` independently
used a slash (`{server}/{captureId}`, nested inside an operation-prefixed
string like `capture:start:{server}/{captureId}`) before this pass; that
document should be reconciled to the colon form used here so the two
contracts agree on one wire format for `Target`. This document does not
edit `audit-events.md` directly (out of scope for this pass — see the
migration-numbering note above for the same boundary), but states the
choice explicitly so that reconciliation is a direct search-and-replace,
not a fresh design decision.

### Mandatory test coverage for this rewrite

- **Operator-403 test** (required, not optional): seed a user bound only
  to the built-in `operator` role and assert HTTP 403 on every route
  listed above. This is the test that would have caught the original hole
  — an operator holds `servers:write`, so this is the token that must be
  proven *not* to work, not merely "some non-admin token."
- Viewer-403 test: same shape, for completeness, but does not substitute
  for the operator test above.
- Admin-200/202 happy path per endpoint.
- A `catalog.go` cross-check test (mirroring existing catalog tests) that
  `captures:manage` is a valid key referenced consistently by
  `rbac.rules` and `003`-series/`007_captures_rbac.sql`.
- Rule-order regression test: assert that `match("POST",
  "/servers/x:capture-start")` and `match("GET", "/servers/x:captures")`
  resolve to `captures:manage`, not `servers:write`/`servers:read` — this
  is the specific regression the ordering requirement above exists to
  prevent, and it is easy for a future edit to silently break by adding a
  new capture-unrelated rule between these lines and line 183/184.
- A grantability test if the human picks option 2 above ("Grantability of
  `captures:manage`"): assert a custom role cannot be created/updated to
  carry `captures:manage`. Not required if the human picks option 1
  (amend FR-005's wording instead).

### Common Error Codes

There is no `code` field in any response (see "Error Response Shape"
above) — this table exists only as an internal naming reference for the
condition each status/message pair represents, not a wire-format value:

| Internal name | Status | Meaning |
|------|------|---------|
| not found | 404 | Server or capture does not exist, or (per FR-007) the capture has expired |
| forbidden | 403 | Caller lacks `captures:manage`; body is the literal string `forbidden` written by `rbac.Middleware` |
| conflict | 409 | Conflicting state: capture already running, server terminating, capture not in the required phase for the operation |
| invalid filter | 400 | Filter expression is invalid (FR-003) |
| invalid ttl | 400 | TTL exceeds cluster maximum (24h default) |
| invalid duration | 400 | Duration limit out of range |
| invalid size | 400 | Size limit out of range |
| invalid request | 400 | Missing/malformed query parameter (e.g. missing `id`) |
| capture disabled | 400 | Capture is not enabled on this server |
| capture not found | 404 | Capture ID does not exist for this server |
| feature disabled | 501 | Capture feature disabled cluster-wide (see the Enable-endpoint's 501 rationale) |
| sidecar unreachable | 502 | Sidecar/agent unreachable |

---

## Functional Requirement Traceability

| FR | Requirement | Contract Coverage |
|----|---|---|
| FR-001 | Opt-in sidecar per GameServer; add/remove without modifying game container | **Enable/Disable Capture** (`POST /servers/{name}:capture-enable`/`:capture-disable`) via ephemeral-container injection; disable does not remove a running container (documented explicitly in the Disable response description). |
| FR-002 | Hard limits on duration and size | **Start Capture**: `maxDurationSeconds` (1..3600), `maxSizeBytes` (1..cluster-max); enforcement itself is in capture-sidecar.md, not this contract. |
| FR-003 | Filter expressions optional; invalid filters rejected before capture starts | **Start Capture**: `filter` optional, validated before CRD creation; see the flagged go-pcap untagged-dependency risk above — coverage is contingent on that risk being resolved, not settled. |
| FR-004 | Downloaded file is valid pcap/pcapng | **Download Capture File**: `Content-Type`/`Content-Disposition` specified; the >64 MiB internal-proxy split from the prior draft is marked UNVERIFIED above pending a read of `api/internal/ws/`. |
| FR-005 | Admin-only access; non-admin gets 403 | **RBAC and Error Handling**: `captures:manage`, seeded only to `admin`, with rule-table entries ordered ahead of the `servers:write`/`servers:read` catch-alls; mandatory operator-403 test specified. **Custom-role grantability gap is open** — see "Grantability of `captures:manage`", not yet ratified by the human. |
| FR-006 | Audit events for all operations, written before response | Each endpoint's Audit Event line, against the real `audit.Event` shape (`Actor`/`Method`/`Path`/`Target`/`Status`/`IP` — no payload/result column). List/Get/Download are GET routes `shouldLog` excludes from the generic middleware (`audit.go:659-663`); those three require a new explicit handler-side audit write, specified under "Download auditing and the GET gap" — this is required implementation work, not yet built. |
| FR-007 | Auto-deletion after retention; expired captures not downloadable/gettable; default retention 24 hours | **List/Get/Download** all return 404/omit on expiry; default `ttlSecondsAfterFinished` is 86400 (24h, DECISION 4); enforcement (the retention reconciler) lives in capture-sidecar.md, not this contract. |
| FR-008 | Sidecar never modifies game container | **Not covered by this contract** — verify in capture-sidecar.md / operator reconciliation. |
| FR-009 | Server remains playable during active capture | **Not covered by this contract** — a performance property, not an API shape; would need an e2e test, not named here. |
| FR-010 | Graceful failure on pod restart; orphaned resources cleaned up | **Not covered by this contract** — operator/reconciler responsibility; see capture-sidecar.md. |
| FR-011 | Packets not matching filter excluded from file | **Download Capture File**: response body description states filtered-only content; enforcement is in the sidecar, not this contract. |
| FR-012 | One concurrent capture per GameServer; second request rejected 409 | **Start Capture**: concurrency check ordered before CRD creation, 409 on an existing `Pending`/`Running` capture. |

**Requirements with no contract coverage, named rather than implied**:
FR-008, FR-009, FR-010 have no REST-API-shaped coverage in this document —
they are operator/sidecar behavioral requirements and belong in
capture-sidecar.md and/or an e2e test plan, not here.

---

## Related Contracts

- **capture-sidecar.md**: wire protocol and lifecycle for sidecar control
  and monitoring.
- **audit-events.md**: audit event schema and payload details. This
  revision settles the `Target` separator as `{name}:{captureId}` (colon)
  and flags the migration-number and GET-audit-gap coordination points
  above; `audit-events.md` should be reconciled to match rather than
  independently re-deciding either.

---

**Document compiled**: 2026-08-23
**Status**: Error-body shape, capture-disabled status code, retention
default, Target separator, and file:line citations corrected against the
verified codebase; still nothing built, run, or tested — every "coverage"
statement above describes a plan, not a verified behavior.
**Next step**: implement `catalog.go`/`rbac.go`/migration changes, then the
mandatory operator-403 test, before any handler code is written. The
human must also ratify one of the two options under "Grantability of
`captures:manage`" before FR-005 can be considered fully specified.
