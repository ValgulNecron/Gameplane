# API HTTP Contract: Install-Time Configuration

## 1. Existing Endpoint: Get All Config

**Endpoint**: `GET /admin/config`

**RBAC Permission**: `config:read`

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
        "name": "helm",
        "kind": "oidc",
        "displayName": "Single sign-on",
        "enabled": true
      },
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
    ]
  },
  "notifications": { ... },
  "telemetry": { ... },
  "modRegistries": { ... },
  "installTimeSettings": {
    "gameDataStorageClass": "fast-nvme",
    "oidcHelmProvider": {
      "groupsClaim": "groups",
      "roleMappings": {
        "admin": ["gameplane-admins"],
        "operator": ["gameplane-operators"],
        "viewer": []
      },
      "defaultRole": "viewer"
    }
  }
}
```

**Changes** (NEW field in response):

| Field | Type | Presence | Semantics |
|-------|------|----------|-----------|
| `installTimeSettings` | object | NEW, optional | Read-only install-time configuration snapshot; omitted if not available |
| `installTimeSettings.gameDataStorageClass` | string | Present if `installTimeSettings` exists | StorageClass name passed via `operator.gameDataStorage.storageClassName` Helm value to the API's `--game-data-storage-class` CLI flag; empty string if using cluster default. This field is report-only on the API (the operator is the consumer). |
| `installTimeSettings.oidcHelmProvider` | object | Present if OIDC Helm config exists | Synthetic "helm" provider's role-mapping configuration (read-only; sourced from CLI flags, not database) |
| `installTimeSettings.oidcHelmProvider.groupsClaim` | string | Present if `oidcHelmProvider` exists | Claim name (e.g., "groups", "roles"); empty if default |
| `installTimeSettings.oidcHelmProvider.roleMappings` | object | Present if `oidcHelmProvider` exists | Group-to-role mappings; mirrors structure of `auth.providers[].roleMappings` |
| `installTimeSettings.oidcHelmProvider.roleMappings.admin` | array[string] | Present if `roleMappings` exists | IdP group names granting "admin" role |
| `installTimeSettings.oidcHelmProvider.roleMappings.operator` | array[string] | Present if `roleMappings` exists | IdP group names granting "operator" role |
| `installTimeSettings.oidcHelmProvider.roleMappings.viewer` | array[string] | Present if `roleMappings` exists | IdP group names granting "viewer" role |
| `installTimeSettings.oidcHelmProvider.defaultRole` | string | Present if `oidcHelmProvider` exists | Default role when no group matches ("viewer", "operator", "admin", "deny", or empty) |

**Error Responses**:

| Status | Reason | Body |
|--------|--------|------|
| `401 Unauthorized` | No valid session | `{"error":"unauthorized"}` |
| `403 Forbidden` | Authenticated but lacks `config:read` permission | `{"error":"forbidden"}` |
| `500 Internal Server Error` | Server error fetching config or operator info | `{"error":"internal error"}` |

**Caching Directive**: `Cache-Control: no-cache` (data may change via helm upgrade or admin mutation)

---

## 2. Existing Endpoint: Update Config Section (FR-015 client-side guard)

**Endpoint**: `PUT /admin/config/{section}`

**RBAC Permission**: `config:manage`

**Authentication**: Required

**Path Parameters**:
- `section` (string): One of `"general"`, `"auth"`, `"notifications"`, `"telemetry"`, `"modRegistries"`

**Request**:
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

**Client-Side Confirmation** (FR-015 — dashboard-level guard, not API-level):
- When saving a provider mapping whose `roleMappings.admin` is non-empty (grants admin role to one or more IdP groups), the dashboard MUST display an advisory warning (from research.md Decision 9) explaining the security implications of granting admin access via group membership.
- The user MUST explicitly confirm the admin mapping before the PUT is sent to the server.
- This is a client-side guard only; the API does not enforce or track the confirmation — it accepts the PUT if validation passes.

**Error Responses**:

| Status | Reason | Body |
|--------|--------|------|
| `400 Bad Request` | Validation failed (reserved name "helm", invalid defaultRole, etc.) | `{"error":"validation failed","details":"..."}` |
| `401 Unauthorized` | No valid session | `{"error":"unauthorized"}` |
| `403 Forbidden` | Lacks `config:manage` permission | `{"error":"forbidden"}` |
| `500 Internal Server Error` | Failed to write config to database | `{"error":"internal error"}` |

**Audit Event**: Recorded on successful write (status 200) with:
- Method: `PUT`
- Path: `/admin/config/auth`
- Target: (empty)
- Reason: `""` (empty on success)
- Status: `200`

---

## 3. Configuration Storage (Backend)

**Database Table**: `config` (`api/internal/db/migrations/002_config.sql`)

**Schema**:
```sql
CREATE TABLE config (
    key        TEXT PRIMARY KEY,      -- section name: "general", "auth", "notifications", "telemetry", "modRegistries"
    value      TEXT NOT NULL,         -- JSON-encoded config (string)
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

**Row for Auth Section** (example):
```sql
INSERT INTO config (key, value) VALUES ('auth', '{"providers":[...]}');
```

**Note**: `installTimeSettings` is NOT stored in the database; it is computed at request time from:
- Operator process flags (passed via Helm / CLI)
- Registry of Helm-configured OIDC provider (synthesized in memory)

---

## 4. Audit Event Recording for OIDC Role Assignment (FR-014)

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

## 5. Audit Endpoint: List Audit Events

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
      "id": 43,
      "ts": "2026-08-25T14:35:00Z",
      "actor": "bob@ex.com",
      "method": "POST",
      "path": "/auth/oidc/callback",
      "target": "bob@ex.com",
      "status": 200,
      "ip": "192.0.2.1",
      "reason": "oidc role assigned: provider=helm matched=gameplane-admins from=new_user to=admin"
    }
  ],
  "total": 1234
}
```

---

## 6. No New HTTP Endpoints

**Summary**: Feature 006 does NOT introduce new REST endpoints. It extends:
- `GET /admin/config` response (adds `installTimeSettings` field)
- `PUT /admin/config/auth` validation (rejects reserved name "helm")
- Audit event recording (captures OIDC role assignments with new `reason` field)

All changes are backward compatible; clients unaware of `installTimeSettings` simply ignore the new field.

---

## 7. CORS & Content Negotiation

**Content-Type**: All endpoints return `application/json`

**CORS**: Admin endpoints (`/admin/*`) are same-origin only; no Access-Control-Allow-Origin header for CORS clients.

**Accept-Language**: Dashboard sends preferred language in `Accept-Language` header; server ignores for config (returns en-US defaults). Localization is client-side (React i18n).
