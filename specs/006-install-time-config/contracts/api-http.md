# API HTTP Contract: Install-Time Configuration

> **This file was rewritten to match the settled mechanism** verified against the live source tree
> (`api/internal/auth/registry.go`, `api/internal/handlers/config.go`, `api/internal/rbac/rbac.go`,
> `api/internal/audit/audit.go`) and recorded in `data-model.md` (Entities 3a, 3b, 4, 4b, 6). A
> previous draft invented a `role-mappings:helm` DB row, a `default-role` override, and a parallel
> `/admin/config/role-mappings` resource — all wrong; none of that appears below. See
> `data-model.md` for the full entity-level detail this file summarizes.

## 0. Maintainer Decision: Hybrid Role-Mapping Resolution (binding)

FR-007 (Helm seeds role mappings at install time) and SC-007 (admins manage mappings through the
dashboard, no `helm upgrade`) are both satisfied by a **hybrid**, layered entirely on the *existing*
admin auth-config endpoint — not a new resource, not a new table:

- Helm values (`api.oidc.roleMappings.*`) SEED the per-role group lists used to resolve the
  synthetic `helm` provider (`auth.HelmProviderName`, `api/internal/auth/registry.go:30`).
  `api.oidc.groupsClaim` and `api.oidc.defaultRole` seed the claim name and fallback role.
- Admins can then set a **DB override** for one or more of the three roles (`admin`, `operator`,
  `viewer` — **not** `groupsClaim`, **not** `defaultRole`, which stay Helm-only in v1). The override
  lives as a new `helmOverride` field alongside `providers` in the *existing* `"auth"` config row
  (`config` table, key `"auth"` — `registry.go:175`, `179-185`). **No new table, no new migration**
  (`api/internal/db/migrations/` stays at `008_captures_rbac.sql`).
- A written override takes effect on that role's very next OIDC login — no API restart, no `helm
  upgrade`. **This is not the row-hash cache** — that cache governs DB providers only. For the
  `"helm"` provider, `Registry.OIDCFor` short-circuits *before* `snapshot()`/the hash is even
  computed: `if name == HelmProviderName { ... return r.legacy, nil }`
  (`registry.go:224-232`). `r.legacy` is a single `*OIDC` built once at process startup
  (`api/cmd/main.go:127`) and held for the process lifetime, so no per-request cache invalidation
  ever runs on this path. Instead, the override takes effect because it is **read at login time**:
  role resolution for the helm provider reads `helmOverride` from the `"auth"` config row on every
  login attempt and merges it per-role over the Helm-seeded lists (the new `effectiveHelmPolicy`
  helper) before calling the existing, unmodified `computeRole`. `registry.go:152-153` already calls
  `legacy.AttachStore(store)` when the registry is built, so the helm provider already has the store
  handle this read needs — no new caching layer, no new store wiring. Because the read happens fresh
  on every login attempt, an admin's dashboard edit lands on the next login with no API restart and
  no `helm upgrade`; that per-login read is what delivers SC-007, not any cache.
- **Resolution precedence, per role, independently**: DB override (if present, even `[]`) replaces
  the Helm-seeded list for that role in full; otherwise the Helm-seeded list stands. This merge runs
  *before* the existing, unmodified `computeRole(groups, pol)` (`api/internal/auth/oidc.go:121`) —
  see the worked example in §2.3.
- **On a later `helm upgrade`**: only the Helm-seeded lists change. Any role carrying a DB override
  keeps that override; the upgrade never clobbers it. Returning a role to its Helm-declared value is
  the explicit "reset" action in §2.2 below — there is no flag-only way to force it.
- The synthetic `"helm"` provider record itself (`GET /admin/config` → `auth.providers[].name ==
  "helm"`) is **unchanged and still immutable** through `PUT /admin/config/auth` — `validateAuth`
  still rejects `providers[].name == "helm"`, exactly as today (§3). `helmOverride` is a sibling
  field next to `providers`, never a `Provider` entry, and is never merged into the stored
  `Provider`/`ProviderPolicy` struct for `"helm"` — only into a policy value derived at
  login-resolution time.
- `groupsClaim` and `defaultRole` have **no DB override path in v1**: they are Helm-only, read only
  from the CLI-flag-built policy. `helmOverride` never carries either field, and no endpoint accepts
  a write to either.

---

## 1. Existing Endpoint: Get All Config

**Endpoint**: `GET /admin/config`

**RBAC Permission**: `config:read` (`api/internal/rbac/rbac.go:169` —
`{method: "GET", segment: "admin", prefix: "/admin/config", perm: "config:read"}`)

**Authentication**: Required (authenticated session cookie)

**Request**:
```
GET /admin/config HTTP/1.1
Cookie: gameplane_session=<token>; gameplane_csrf=<token>
```

**Response** (200 OK):

```json
{
  "general": {
    "instanceName": "My Game Cluster",
    "externalURL": "https://gameplane.example.com",
    "defaultNamespace": "games"
  },
  "auth": {
    "providers": [
      {
        "name": "github-oauth",
        "kind": "oidc",
        "displayName": "GitHub",
        "enabled": true,
        "issuer": "https://accounts.google.com",
        "clientID": "...",
        "configRef": "secret/github-oidc-config",
        "groupsClaim": "teams",
        "roleMappings": {
          "admin": ["org:admins"],
          "operator": ["org:ops"],
          "viewer": ["org:members"]
        },
        "defaultRole": "viewer"
      }
    ],
    "helmOverride": {
      "roleMappings": {
        "admin": ["gameplane-admins", "infrastructure-team"],
        "viewer": []
      }
    }
  },
  "notifications": { ... },
  "telemetry": { ... },
  "modRegistries": { ... },
  "installTimeSettings": {
    "gameDataStorageClass": "fast-nvme",
    "oidcHelmProvider": {
      "groupsClaim": "teams",
      "defaultRole": "viewer",
      "roleMappings": {
        "admin": ["gameplane-admins"],
        "operator": ["gameplane-ops"],
        "viewer": ["gameplane-members"]
      }
    }
  }
}
```

**Changes**:

| Field | Type | Presence | Semantics |
|-------|------|----------|-----------|
| `auth.helmOverride` | object | NEW, optional | Absent entirely means no role currently has a DB override — every role of the synthetic `"helm"` provider falls through to its Helm-seeded value. See §2.1 for shape rules. |
| `auth.helmOverride.roleMappings.{admin\|operator\|viewer}` | array[string] | NEW, each independently optional | **Key present** (including `[]`) means that role is DB-overridden — this array is the override list, and its presence is itself the provenance signal the dashboard uses to render a "reset to Helm default" action for that role (§0, §2.2). **Key absent/`null`** means no override — the role's effective mapping is the Helm-seeded one, which the dashboard reads from `installTimeSettings.oidcHelmProvider.roleMappings` (see note below and §1's field table entry for `installTimeSettings.oidcHelmProvider`). |
| `installTimeSettings` | object | NEW, optional | Install-time configuration snapshot; omitted if not available |
| `installTimeSettings.gameDataStorageClass` | string | Present if `installTimeSettings` exists | StorageClass name passed via `operator.gameDataStorage.storageClassName` Helm value to the API's `--game-data-storage-class` CLI flag; empty string if using cluster default. Report-only — the API never uses this value; unrelated to the role-mapping hybrid and not DB-overridable. |
| `installTimeSettings.oidcHelmProvider` | object | Present if Helm OIDC is configured | Read-only snapshot of the Helm-seeded values for the synthetic `"helm"` provider: `groupsClaim`, `defaultRole`, and `roleMappings.{admin\|operator\|viewer}` as passed via `api.oidc.groupsClaim`/`api.oidc.defaultRole`/`api.oidc.roleMappings.*`. This is the raw seed, not merged with `auth.helmOverride` — the dashboard combines the two client-side to render the effective mapping per role (FR-017, SC-006). |

**Provenance note (M7)**: `helmOverride.roleMappings` is deliberately the *entire* provenance
signal — there is no separate `source: "db"|"helm"` field anywhere in this response. A role key's
presence (even as `[]`) means "DB overlay active for this role"; its absence means "Helm-seeded
value stands." The dashboard reads the Helm-seeded list from `installTimeSettings.oidcHelmProvider`
and combines it client-side with `auth.helmOverride` to render the effective mapping and offer reset
— the API itself never computes or returns a merged "effective" view.

**Error Responses**:

| Status | Reason | Body |
|--------|--------|------|
| `401 Unauthorized` | No valid session | `{"error":"unauthorized"}` |
| `403 Forbidden` | Authenticated but lacks `config:read` permission | `{"error":"forbidden"}` |
| `500 Internal Server Error` | Server error fetching config | `{"error":"internal error"}` |

**Caching Directive**: `Cache-Control: no-cache` (data may change via helm upgrade or admin mutation)

---

## 2. Admin-Managed Helm Role-Mapping Overrides

This is **not** a parallel resource. It extends the *existing* `/admin/config` mount
(`api/internal/handlers/config.go:30`, `MountConfig`) — but that mount's signature changes, because
the audit events in §2.4 must be written from inside `config.go`, and `configHandler` has no
`*audit.Auditor` today. The in-tree precedent for injecting one into a handler-mount function is
`MountCapture(r chi.Router, reg *kube.Registry, auditor *audit.Auditor, cfg CaptureConfig, ...)`
(`api/internal/handlers/capture.go:55`). `MountConfig` gains the same kind of parameter:

```go
func MountConfig(r chi.Router, store *db.Store, auditor *audit.Auditor, helmOIDCPresent bool) {
	h := &configHandler{db: store, auditor: auditor, validators: newValidators(helmOIDCPresent)}
	r.Route("/admin/config", func(r chi.Router) {
		r.Get("/", h.getAll)
		r.Put("/{section}", h.put)
		r.Delete("/auth/role-mappings/{role}", h.resetRoleMapping) // NEW
	})
}
```

**Call site changes** (`api/cmd/main.go:245`, currently `handlers.MountConfig(p, store, oidcAuth !=
nil)` — no auditor passed): becomes `handlers.MountConfig(p, store, auditor, oidcAuth != nil)`. The
`auditor` value is already constructed earlier in `main.go` (used by `handlers.MountAudit(p,
auditor)` and `MountCapture`'s own call site) — this feature does not construct a new one, only
threads the existing one through to `MountConfig`.

and inherits the RBAC rules already in place for that prefix (`api/internal/rbac/rbac.go:169-170`)
with **no new `rbac.go` rule needed**:

```go
{method: "GET", segment: "admin", prefix: "/admin/config", perm: "config:read"},
{segment: "admin", prefix: "/admin/config", perm: "config:manage"},
```

### 2.1 `PUT /admin/config/auth` — EXISTING route, EXTENDED body

**RBAC Permission**: `config:manage`

**Authentication**: Required

This is the same route, same handler, same "PUT replaces the whole section value" semantics as
today (`config.go:70-103`, `put`, dispatching to `validateAuth(helmOIDCPresent)`). It is how an
admin sets, changes, or clears one or more roles' overrides — there is no separate "create override"
endpoint.

**Request** — `providers` unchanged; `helmOverride` is the new optional sibling field (§0):
```json
PUT /admin/config/auth HTTP/1.1
Content-Type: application/json
Cookie: gameplane_session=<token>; gameplane_csrf=<token>

{
  "providers": [ ... unchanged Provider[] ... ],
  "helmOverride": {
    "roleMappings": {
      "admin": ["gameplane-admins", "platform-team"],
      "viewer": []
    }
  }
}
```

Omitting `helmOverride` entirely, or omitting a given role's key within
`helmOverride.roleMappings`, leaves that role's override (or lack of one) exactly as it was before
the write — `PUT` replaces the whole `"auth"` row's JSON, so the dashboard must round-trip
`helmOverride` from the last `GET` if it is not changing it in this request (same requirement that
already applies to `providers`).

**Response** (200 OK) — echoes the canonicalized body, same envelope as today:
```json
{
  "section": "auth",
  "value": {
    "providers": [ ... ],
    "helmOverride": { "roleMappings": { "admin": ["gameplane-admins", "platform-team"], "viewer": [] } }
  }
}
```

**Validation Rules** (extends `validateAuth`, `api/internal/handlers/config.go`):
- All existing `providers[]` rules are unchanged (§3 below), **including** that `providers[].name ==
  "helm"` stays rejected — `helmOverride` does not relax that guard.
- `helmOverride.roleMappings.{admin|operator|viewer}`, when the key is present: array of non-blank
  strings, same rule `validateProviderMapping` already applies to
  `providers[].roleMappings.admin|operator|viewer`. `[]` is valid and meaningful (§0).
- `helmOverride` (or `helmOverride.roleMappings`) present-but-empty is accepted and is equivalent to
  omitting it — no role key means no override for any role.
- Requires Helm OIDC to be configured for a *non-empty* `helmOverride` to be meaningful, but the API
  does not reject setting `helmOverride` when `--oidc-issuer` is unset — it simply has no effect
  until the `"helm"` provider exists, mirroring how the field carries no immediate consequence
  either way.

**Error Responses**: same table as §3 below (`400`/`401`/`403`/`500`), `helmOverride` validation
failures surface as `400 Bad Request` with `{"error":"validation failed","details":"..."}`.

**Audit Event**: see §2.3.

---

### 2.2 `DELETE /admin/config/auth/role-mappings/{role}` — NEW route (the only new route)

This is the **one** new HTTP route this feature adds. It resets a single role's override, restoring
the Helm-seeded value for that role — the "reset to Helm default" action referenced throughout §0.

**Mount** (added inside the same `r.Route("/admin/config", ...)` block as the two routes above,
nested under the `auth` section the same way the existing routes nest under `/admin/config`):
```go
r.Route("/admin/config", func(r chi.Router) {
	r.Get("/", h.getAll)
	r.Put("/{section}", h.put)
	r.Delete("/auth/role-mappings/{role}", h.resetRoleMapping) // NEW
})
```

**RBAC Permission**: `config:manage` — same gate as `PUT /admin/config/{section}` above, inherited
automatically since this route also falls under the `/admin/config` prefix rule
(`rbac.go:170`); no new `rbac.go` entry.

**Authentication**: Required

**Path Parameters**:
- `role` (string): one of `admin`, `operator`, `viewer`. Any other value (including the removed
  `default-role`, which is **not** a valid value — `defaultRole` has no override path per §0/M5) is
  `400 Bad Request`.

**Request**:
```
DELETE /admin/config/auth/role-mappings/admin HTTP/1.1
Cookie: gameplane_session=<token>; gameplane_csrf=<token>
```

**Effect**: removes that one role's key from `helmOverride.roleMappings` in the `"auth"` row,
leaving the other two roles' overrides (if any) untouched, and re-persists the row. If the role had
no override, this is a no-op that still returns `200` (idempotent reset — matches the "delete of
something already absent succeeds" convention used elsewhere in this handler family, e.g.
`registry_secret.go`'s delete).

**Response** (200 OK) — the updated `"auth"` section body, same envelope shape as `put`'s
`{"section": ..., "value": ...}`, so the dashboard can refresh from the response without a follow-up
`GET`:
```json
{
  "section": "auth",
  "value": {
    "providers": [ ... unchanged ... ],
    "helmOverride": {
      "roleMappings": { "viewer": [] }
    }
  }
}
```
(Example shows `admin`'s key removed while a pre-existing `viewer: []` override survives.)

**Error Responses**:

| Status | Reason | Body |
|--------|--------|------|
| `400 Bad Request` | Invalid `role` path segment (not `admin`/`operator`/`viewer`) | `{"error":"validation failed","details":"..."}` |
| `401 Unauthorized` | No valid session | `{"error":"unauthorized"}` |
| `403 Forbidden` | Lacks `config:manage` | `{"error":"forbidden"}` |
| `500 Internal Server Error` | Failed to persist the updated row | `{"error":"internal error"}` |

**Audit Event**: see §2.3.

---

### 2.3 Worked Example — Merge Precedence (M3/M4, stated explicitly)

Because the per-role merge (§0) runs *before* the existing, unmodified `computeRole` evaluates its
admin > operator > viewer tie-break, **the most privileged match still wins across both sources**:

- Helm seed: `roleMappings.admin = ["gameplane-admins"]`
- DB override: `helmOverride.roleMappings.viewer = ["gameplane-admins"]` (an admin decided this
  particular group should now *also* — or instead — grant viewer, and wrote that override)
- A user whose OIDC groups include `gameplane-admins` still resolves to **admin**, not viewer — the
  override replaced the *viewer* role's list, it did not remove `gameplane-admins` from the
  (unmodified) Helm-seeded *admin* list, and `computeRole` checks admin before viewer regardless of
  which layer each match came from.

This is the correct, intended behavior (an earlier draft of this contract stated it backwards).

---

### 2.4 Audit Events for Role-Mapping Overrides (M8)

Every write that changes `helmOverride` — a `PUT /admin/config/auth` whose body sets, changes, or
removes a role's override list, and every `DELETE /admin/config/auth/role-mappings/{role}` — is
recorded via the existing `WriteSync` call (`api/internal/audit/audit.go:689`):

```go
func (a *Auditor) WriteSync(ctx context.Context, method, path, target, reason string, status int) error
```

`audit_events` has exactly the columns `id, ts, actor, method, path, target, status, ip, reason` —
**no** `action`/`metadata`/`message` column (`api/internal/db/migrations/001_init.sql` +
`007_audit_reason.sql`). Every detail rides in the free-text `reason` string, in **one consistent
format** used for both operations (and matching the shape Entity 5 / §5 below already uses):

```
oidc role mapping override set: role=<role> groups=<comma_joined_or_none>
oidc role mapping override reset: role=<role>
```

- `role=<role>`: one of `admin`, `operator`, `viewer` — the single role this row's write/reset
  affected. A `PUT /admin/config/auth` that changes more than one role's override in the same
  request emits **one audit row per changed role** (each with its own `target`/`reason`), so `target`
  always names exactly one role.
- `groups=<comma_joined_or_none>`: the role's override list *after* the write, comma-joined (e.g.
  `gameplane-admins,infra-team`), or the literal `none` for an empty-list override (`[]`). Omitted
  from the `reset` template — a reset has no new list, it restores the Helm-seeded one.
- `actor` (already populated by `WriteSync` from the authenticated session) is the admin who made
  the change — not duplicated in `reason`.

| Operation | `method` | `path` | `target` | `reason` (status 200) |
|-----------|----------|--------|----------|-------------------------------------|
| `PUT /admin/config/auth` (role override set/changed) | `PUT` | `/admin/config/auth` | `<role>` | `"oidc role mapping override set: role=<role> groups=<comma_joined_or_none>"` |
| `DELETE /admin/config/auth/role-mappings/{role}` | `DELETE` | `/admin/config/auth/role-mappings/{role}` | `<role>` | `"oidc role mapping override reset: role=<role>"` |

Example rows:
```
PUT    /admin/config/auth                          → target: admin  → reason: "oidc role mapping override set: role=admin groups=gameplane-admins,platform-team"
PUT    /admin/config/auth                          → target: viewer → reason: "oidc role mapping override set: role=viewer groups=none"
DELETE /admin/config/auth/role-mappings/admin      → target: admin  → reason: "oidc role mapping override reset: role=admin"
```

---

## 3. Existing Endpoint: Update Config Section (FR-015 client-side guard)

**Endpoint**: `PUT /admin/config/{section}`

**RBAC Permission**: `config:manage`

**Authentication**: Required

**Path Parameters**:
- `section` (string): One of `"general"`, `"auth"`, `"notifications"`, `"telemetry"`, `"modRegistries"`
  (unchanged closed set — `helmOverride` is a field *within* the `"auth"` section body, not a new
  section name).

**Request** (example targeting `section = auth`, showing an ordinary provider edit — `helmOverride`
handling is covered separately in §2.1):
```json
PUT /admin/config/auth HTTP/1.1
Content-Type: application/json
Cookie: gameplane_session=<token>; gameplane_csrf=<token>

{
  "providers": [
    {
      "name": "okta-oidc",
      "kind": "oidc",
      "issuer": "https://dev-1234567.okta.com",
      "clientID": "0oa...",
      "configRef": "secret/okta-config",
      "displayName": "Okta",
      "groupsClaim": "groups",
      "roleMappings": {
        "admin": ["okta-admins"],
        "operator": ["okta-ops"],
        "viewer": []
      },
      "defaultRole": "viewer"
    }
  ]
}
```

**Response** (200 OK):
```json
{
  "section": "auth",
  "value": { ... same as request body ... }
}
```

**Validation Rules** (enforced at API, defined in `api/internal/handlers/config.go:validateAuth`):
- `providers[].name`: Not equal to `"helm"` (reserved for Helm-configured provider)
- `providers[].groupsClaim`: If set, must not be blank
- `providers[].defaultRole`: Must be one of `""`, `"viewer"`, `"operator"`, `"admin"`, `"deny"`
- `providers[].roleMappings.admin|operator|viewer`: Arrays must not contain empty strings
- `providers[].defaultRole` with value other than `""` requires `providers[].roleMappings` to be present
- `providers[].configRef`: Must reference an existing Secret in the operator namespace (validated via K8s API)
- `helmOverride.roleMappings.admin|operator|viewer` (new, §2.1): when the key is present, array of
  non-blank strings; no other new validation is added for `section = "auth"`.

**Client-Side Confirmation** (FR-015 — dashboard-level guard, not API-level):
- When saving a provider mapping whose `roleMappings.admin` is non-empty (grants admin role to one or more IdP groups), the dashboard MUST display an advisory warning (from research.md Decision 9) explaining the security implications of granting admin access via group membership.
- The same guard applies to setting `helmOverride.roleMappings.admin` to a non-empty list.
- The user MUST explicitly confirm the admin mapping before the PUT is sent to the server.
- This is a client-side guard only; the API does not enforce or track the confirmation — it accepts the PUT if validation passes.

**Error Responses**:

| Status | Reason | Body |
|--------|--------|------|
| `400 Bad Request` | Validation failed (reserved name "helm", invalid defaultRole, invalid `helmOverride`, etc.) | `{"error":"validation failed","details":"..."}` |
| `401 Unauthorized` | No valid session | `{"error":"unauthorized"}` |
| `403 Forbidden` | Lacks `config:manage` permission | `{"error":"forbidden"}` |
| `500 Internal Server Error` | Failed to write config to database | `{"error":"internal error"}` |

**Audit Event**: for a plain provider edit that does not touch `helmOverride`, recorded as before —
`method: PUT`, `path: /admin/config/auth`, `target: ""`, `reason: ""`, `status: 200`. When the same
write also changes `helmOverride`, the role-specific rows described in §2.4 are recorded *in
addition* to (not instead of) this generic one.

---

## 4. Configuration Storage (Backend)

**Database Table**: `config` (`api/internal/db/migrations/002_config.sql`) — **unchanged schema, no
new migration** (M1):
```sql
CREATE TABLE config (
    key        TEXT PRIMARY KEY,      -- section name: "general", "auth", "notifications", "telemetry", "modRegistries"
    value      TEXT NOT NULL,         -- JSON-encoded config (string)
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

**Row for Auth Section** (example, with an override present):
```sql
INSERT INTO config (key, value) VALUES (
  'auth',
  '{"providers":[...],"helmOverride":{"roleMappings":{"admin":["gameplane-admins","platform-team"],"viewer":[]}}}'
);
```

**Note**: `installTimeSettings.gameDataStorageClass` is NOT stored in the database; it is computed
at request time from the API process's `--game-data-storage-class` CLI flag (report-only, unrelated
to the role-mapping hybrid). `helmOverride` **is** stored in the database, as the `"auth"` row's new
sibling field described above — no separate table, no separate key.

---

## 5. Audit Event Recording for OIDC Role Assignment (FR-014)

**Event Trigger**: User logs in via OIDC and their role is assigned or re-evaluated.

**Requirement**: FR-014 — The OIDC role assignment MUST emit an audit event recording the provider name, matched group claim value, previous role, and new role in the `reason` field.

**Audit Event Details**:

| Field | Value |
|-------|-------|
| `ts` | Request timestamp (RFC3339) |
| `actor` | Username (from OIDC claim, typically email) |
| `method` | `POST` (login is a session-creation mutation) |
| `path` | `/auth/oidc/callback` (or `/auth/oidc/callback?provider=helm`) |
| `target` | Username (same as actor; identifies the subject) |
| `status` | `200` (login succeeded) or `400`/`401`/`403` (login failed) |
| `ip` | Client IP address |
| `reason` | Structured string: `"oidc role assigned: provider=<name> matched=<claimValue\|none> from=<oldRole> to=<newRole>"` (on status 200); or empty on failure |

**Stored in**: `audit_events` table (`api/internal/db/migrations/001_init.sql` + `007_audit_reason.sql`; reason column is TEXT, nullable)

**Reason Format** (on successful login, status 200):
```
oidc role assigned: provider=helm matched=gameplane-admins from=viewer to=admin
oidc role assigned: provider=okta matched=none from=viewer to=viewer
oidc role assigned: provider=github matched=org-ops from=viewer to=operator
```

- `provider`: The IdP provider name (e.g., "helm" for Helm-configured OIDC, or custom provider name from the database)
- `matched`: The IdP group claim value that matched a role mapping, or `"none"` if no group matched and the default role was applied
- `from`: The user's previous role (first login shows "new_user"; re-evaluation shows the prior role)
- `to`: The role assigned on this login

When the login resolves against the synthetic `"helm"` provider, the role mapping consulted for the
match is the **effective** (DB-override-merged) one from §0/§2.3 — `provider=helm` in the reason
string does not distinguish whether the matched group came from a DB override or the Helm seed; that
distinction is only visible via `GET /admin/config`'s `helmOverride` field (§1), not in this
login-time audit row.

**Example Query** (audit log for OIDC logins):
```sql
SELECT id, ts, actor, status, reason FROM audit_events
WHERE path LIKE '/auth/oidc%' AND status = 200
ORDER BY ts DESC;
```

**Example Response** (3 audit events):
```
| id | ts                      | actor        | status | reason                                              |
|----|-------------------------|--------------|--------|-----------------------------------------------------|
| 42 | 2026-08-25T14:30:00Z    | alice@ex.com | 200    | oidc role assigned: provider=helm matched=gameplane-admins from=new_user to=admin |
| 43 | 2026-08-25T14:35:00Z    | bob@ex.com   | 200    | oidc role assigned: provider=helm matched=none from=new_user to=viewer |
| 44 | 2026-08-25T14:40:00Z    | alice@ex.com | 200    | oidc role assigned: provider=helm matched=gameplane-operators from=admin to=operator |
```

**Audit Event Mutation** (required change to `api/internal/auth/oidc.go`):
- After user is created/updated with new role on successful login (status 200), record the event with the structured reason as shown above.
- Method: Call `a.WriteSync(ctx, "POST", "/auth/oidc/callback", username, reason, 200)` where reason follows the format `"oidc role assigned: provider=... matched=... from=... to=..."`

---

## 6. Audit Endpoint: List Audit Events

**Endpoint**: `GET /admin/audit` (existing; no changes)

**RBAC Permission**: `audit:read`

**Query Parameters**:
- `offset` (int, optional): Start position; default 0
- `limit` (int, optional): Max results per page; default 50, max 1000
- `actor` (string, optional): Filter by username
- `path` (string, optional): Filter by request path (substring match)

**Response** (200 OK):
```json
{
  "events": [
    {
      "id": 44,
      "ts": "2026-08-25T14:40:00Z",
      "actor": "alice@ex.com",
      "method": "POST",
      "path": "/auth/oidc/callback",
      "target": "alice@ex.com",
      "status": 200,
      "ip": "192.0.2.1",
      "reason": "oidc role assigned: provider=helm matched=gameplane-operators from=admin to=operator"
    },
    {
      "id": 51,
      "ts": "2026-08-26T09:12:00Z",
      "actor": "admin@example.com",
      "method": "PUT",
      "path": "/admin/config/auth",
      "target": "admin",
      "status": 200,
      "ip": "192.0.2.10",
      "reason": "oidc role mapping override set: role=admin groups=gameplane-admins,platform-team"
    }
  ],
  "total": 1234
}
```

---

## 7. Summary of New/Changed HTTP Surface

**NEW route** (the only one this feature adds):
- `DELETE /admin/config/auth/role-mappings/{role}` — reset one role (`admin`|`operator`|`viewer`)
  to its Helm-seeded value, removing its `helmOverride.roleMappings` entry (§2.2)

**Extended** (no route added or renamed):
- `GET /admin/config` response — `auth.helmOverride` (new, optional) reports per-role DB overrides,
  itself the provenance signal (§1); `installTimeSettings.gameDataStorageClass` (new, optional,
  report-only, unrelated to the role-mapping hybrid); `installTimeSettings.oidcHelmProvider` (new,
  optional) reports the raw Helm-seeded `groupsClaim`/`defaultRole`/`roleMappings`, which the
  dashboard needs to compute the effective per-role mapping client-side (FR-017, SC-006) — the API
  does not merge or return an effective view itself
- `PUT /admin/config/auth` — body may now include `helmOverride.roleMappings`; validation extended
  accordingly (§2.1, §3); the reserved-name guard on `providers[].name == "helm"` is unchanged
- Audit event recording — captures both OIDC role assignments (§5, FR-014, unchanged shape) and,
  newly, admin writes/resets of `helmOverride` (§2.4, M8)

**Deleted from the prior (wrong) draft of this file**: `GET /admin/config/role-mappings`,
`PUT /admin/config/role-mappings/{role}`, `DELETE /admin/config/role-mappings/{role}` (as a
top-level resource), any `role=default-role` path value, any `role-mappings:helm` config-table key,
any `source: "db"|"helm"` response field. None of these exist in the settled design.

**Backward compatibility**: purely additive. `GET /admin/config`'s `auth` object gains one optional
field (`helmOverride`) that absent-by-default installs never see; existing `providers[]` shape and
every other section are byte-for-byte unchanged.

---

## 8. CORS & Content Negotiation

**Content-Type**: All endpoints return `application/json`

**CORS**: Admin endpoints (`/admin/*`) are same-origin only; no Access-Control-Allow-Origin header for CORS clients.

**Accept-Language**: Dashboard sends preferred language in `Accept-Language` header; server ignores for config (returns en-US defaults). Localization is client-side (React i18n).
