# Data Model: Install-Time Configuration (Storage Class & OIDC Role Mapping)

**Scope**: This document describes the core data entities, their fields, validation rules, relationships, and state transitions for feature 006-install-time-config.

---

## Entity 1: Install-Time Game-Data Storage Class Setting

### Helm Values Surface

**Key Path**: `operator.gameDataStorage.storageClassName`

**Helm Values YAML**:

```yaml
operator:
  gameDataStorage:
    storageClassName: ""  # empty string = cluster default; set to a named StorageClass name
```

**Type**: `string` (required, non-empty if set; empty string is valid and means use cluster default)

**Default**: `""` (empty string; interpreted as "use cluster default")

**Routing**: This single Helm value is passed to TWO separate components:
1. **Operator Deployment** (`operator/cmd/main.go`): Via `--game-data-storage-class` flag. Used by the GameServerReconciler to materialize PVCs with this default.
2. **API Deployment** (`api/cmd/main.go`): Via `--game-data-storage-class` flag on the `api serve` subcommand. Read-only; reported via `/admin/config` as `installTimeSettings.gameDataStorageClass`.

**Validation Rules**:

- Must be a DNS-1123 subdomain (lowercase alphanumerics + hyphens, max 253 characters) or empty string.
- If non-empty, the named StorageClass must exist on the cluster at pod creation time (validated at PVC materialization, not at Helm install time).
- Case-sensitive; must match the exact StorageClass name in the cluster.

**Source**: `/home/valgul/project/kubernetes-game-dashboard/charts/gameplane/values.yaml` (new key to be added)

**Cite FR-001, FR-002, FR-006, D2**.

---

### Operator CLI Flag

**Flag Name**: `--game-data-storage-class`

**Type**: `string`

**Default**: `""` (empty string)

**Introduced At**: Operator startup (command-line argument, `operator/cmd/main.go` entry point)

**Flag Definition** (pseudo-Go):

```go
var gameDataStorageClass string
flag.StringVar(&gameDataStorageClass, "game-data-storage-class", "",
    "StorageClass for game server data volumes (GameServer spec.storage.storageClassName). "+
        "Empty (default) uses the cluster's default StorageClass. "+
        "Applies when a GameServer does not explicitly set spec.storage.storageClassName.")
```

**Stored In**: `GameServerReconciler.DefaultStorageClassName` (field added to the reconciler struct, set from the parsed flag at initialization)

**Source**: `/home/valgul/project/kubernetes-game-dashboard/operator/cmd/main.go` (new flag to be added)

**Cite FR-001, FR-002**.

---

### API CLI Flag (Report-Only)

**Flag Name**: `--game-data-storage-class`

**Type**: `string`

**Default**: `""` (empty string)

**Introduced At**: API server startup (command-line argument on the `api serve` subcommand, `api/cmd/main.go`)

**Purpose**: Carries the same Helm value as the operator flag, purely for reporting via the admin `/admin/config` endpoint. The API never uses this value for any business logic — it is informational only.

**Flag Definition** (pseudo-Go):

```go
var gameDataStorageClass string
flag.StringVar(&gameDataStorageClass, "game-data-storage-class", "",
    "StorageClass default for game server data volumes (Helm install-time setting). "+
        "Informational only; reported via GET /admin/config as installTimeSettings.gameDataStorageClass. "+
        "Must match the operator's --game-data-storage-class value.")
```

**Stored In**: Config struct used by the `/admin/config` handler to build the response; passed to `installTimeSettings.gameDataStorageClass`

**Source**: `/home/valgul/project/kubernetes-game-dashboard/api/cmd/main.go` (new flag to be added)

**Cite D2 (binding decision), FR-001, FR-002**.

---

### Resolution Function Signature

**Function**: `resolveStorageClass(gs *GameServer, tmpl *GameTemplate, defaultStorageClass string) *string`

**Parameters**:

- `gs` (*GameServer): The game server being reconciled; may have `Spec.Storage.StorageClassName` set (nil if not).
- `tmpl` (*GameTemplate): The template the server is based on; may have `Spec.Storage.StorageClassName` set (nil if not).
- `defaultStorageClass` (string): The operator's install-time default (from `--game-data-storage-class` flag); empty string if not set.

**Return Type**: `*string` (pointer to string, to be assigned to `PersistentVolumeClaim.Spec.StorageClassName`)

**Semantics**:

- Returns the first non-nil value in this precedence order:
  1. `gs.Spec.Storage.StorageClassName` (GameServer explicit override)
  2. `tmpl.Spec.Storage.StorageClassName` (GameTemplate default)
  3. Convert `defaultStorageClass` to `*string` if non-empty; otherwise `nil` (install-time default)
  4. `nil` (cluster default, no StorageClassName set)

**Return Examples**:

- If `gs.Spec.Storage.StorageClassName = "fast-nvme"`, returns `&"fast-nvme"` (regardless of template or install-time default).
- If `gs.Spec.Storage.StorageClassName = nil`, `tmpl.Spec.Storage.StorageClassName = "standard"`, and `defaultStorageClass = "gpu-attached"`, returns `&"standard"` (template wins over install-time).
- If `gs.Spec.Storage.StorageClassName = nil`, `tmpl.Spec.Storage.StorageClassName = nil`, and `defaultStorageClass = "local-nvme"`, returns `&"local-nvme"` (install-time default).
- If `gs.Spec.Storage.StorageClassName = nil`, `tmpl.Spec.Storage.StorageClassName = nil`, and `defaultStorageClass = ""`, returns `nil` (cluster default).

**Cite FR-004, FR-002**.

---

### PVC Creation Integration Points

**Three PVC types use this function**:

1. **Game-Data PVC** (`<gameserver-name>-data`)
   - File: `operator/internal/controller/gameserver_controller.go`
   - Function: `reconcilePVC()` (lines 505-536, approx)
   - Injection point: After line 526 (after the nil-check where StorageClassName is currently set)
   - All storage volumes (game binary, mission data) share this class

2. **Extra Volumes PVC** (`<gameserver-name>-extra-<name>`, one per extra volume)
   - File: `operator/internal/controller/gameserver_extravolumes.go`
   - Function: `reconcileExtraPVCs()` (lines 122-129, approx)
   - Injection point: Inside the loop after line 129 (after each extra volume's StorageClassName is set)
   - Each extra volume individually uses the resolved class

3. **Mod Volume PVC** (`<gameserver-name>-mods-<key>`)
   - File: `operator/internal/controller/gameserver_version.go`
   - Function: `reconcileModPVC()` (lines 201-212, approx)
   - Injection point: After line 212 (after the nil-check where StorageClassName is currently set)
   - Mods (and loaders) volumes use the resolved class

**Cite FR-002, FR-004**.

---

## Entity 2: GameServer Status Failure Reporting for Storage Class

### Existing Status Fields (No New Fields Needed)

**CRD Type**: `GameServerStatus` (file: `operator/api/v1alpha1/gameserver_types.go`, lines 487-537)

**Used Fields**:

| Field | JSON Tag | Type | Purpose |
|-------|----------|------|---------|
| `Phase` | `phase` | `GameServerPhase` enum | High-level state: Pending, Starting, Running, Stopping, Stopped, Suspended, Failed |
| `Conditions` | `conditions` | `[]metav1.Condition` | List of detailed state transitions (patchable, follows Kubernetes conventions) |
| `ObservedGeneration` | `observedGeneration` | `int64` | Which generation this status reflects |

**Condition Structure** (standard `metav1.Condition`, Kubernetes 1.28+ convention):

```go
type Condition struct {
    Type               string      // e.g., "Ready", "Progressing", "Healthy"
    Status             ConditionStatus  // True | False | Unknown
    Reason             string      // Machine-readable short code (e.g., "PVCProvisioningFailed")
    Message            string      // Human-readable sentence (e.g., "StorageClass 'fast-nvme' not found")
    ObservedGeneration int64       // Which generation produced this condition
    LastTransitionTime metav1.Time // When it last changed
}
```

**Cite FR-005**.

---

### Failure Message Rendering

**Dashboard Component**: `web/src/routes/ServerDetail.tsx` (lines 101-104, 179-186)

**Rendering Logic**:

- When `Phase == "Pending"` or a Ready condition has `Status == False`, render the condition's `Reason` + `Message` as the failure cause.
- Icon: AlertTriangle (Lucide icon).
- Example message shown to operator: `"PVCProvisioningFailed: StorageClass 'nonexistent-class' not found on cluster."`

**Detection Timing**:

- Direct StorageClass GET: Reconciler calls `GET /api/v1/storageclasses/{name}` on each reconcile (cluster-local, ~50-100ms).
- Condition set: Within the same reconciliation cycle (~5 seconds from pod creation).
- Dashboard poll: Every 5 seconds (`ServerDetail.tsx` line 67: `refetchInterval: 5_000`).
- **Total latency**: ~5-10 seconds from PVC creation to error visible on dashboard.

**Cite FR-005, SC-002**.

---

### Condition Type & Reason for PVC Storage Class Failures

**Condition Type**: `"Ready"` (existing; no new type needed)

**Reason Code**: `"PVCProvisioningFailed"`

**Message Format**: `"StorageClass '{classname}' not found on cluster."`

**Example Condition JSON**:

```json
{
  "type": "Ready",
  "status": "False",
  "reason": "PVCProvisioningFailed",
  "message": "StorageClass 'fast-nvme' not found on cluster.",
  "observedGeneration": 3,
  "lastTransitionTime": "2026-08-25T10:30:00Z"
}
```

**Cite FR-005**.

---

## Entity 3: OIDC Role Mapping

### Helm Values Surface (Helm-Owned Provider Config)

**Key Path**: `api.oidc.*`

**Helm Values YAML**:

```yaml
api:
  oidc:
    groupsClaim: "groups"          # OIDC token claim name containing groups/roles
    defaultRole: "viewer"          # fallback: "" (empty), "viewer", "operator", "admin", "deny"
    roleMappings:
      admin:
        - "gameplane-admins"
        - "infrastructure-team"
      operator:
        - "gameplane-operators"
      viewer:
        - "gameplane-viewers"
```

**Breakdown**:

- **groupsClaim** (string, optional): The name of the OIDC ID token claim that holds group/role information. Empty string defaults to `"groups"`. Common values: `"groups"`, `"roles"`, `"membership"`, `"resource_access"` (varies by OIDC provider).

- **roleMappings** (object, optional): Maps group names to Gameplane roles. Null or omitted means no role mapping (users get viewer by default, no re-evaluation on subsequent logins). If present, at least one role list must be non-empty.
  - `admin` (array of strings): Group names that map to the admin role.
  - `operator` (array of strings): Group names that map to the operator role.
  - `viewer` (array of strings): Group names that map to the viewer role.

- **defaultRole** (string, optional): Role assigned to users whose group does not match any mapping. Allowed values: `""` (empty, interpreted as "viewer"), `"viewer"`, `"operator"`, `"admin"`, `"deny"` (deny refuses login). Defaults to `""` (viewer).

**Validation Rules**:

- `groupsClaim`: If set, must not be blank or whitespace-only.
- `roleMappings`: If set, at least one of `admin`, `operator`, `viewer` must be a non-empty array.
- `roleMappings[*]`: Arrays must not contain empty strings or whitespace-only elements.
- `defaultRole`: Must be one of `""`, `"viewer"`, `"operator"`, `"admin"`, `"deny"`.
- `defaultRole` and `roleMappings`: If `roleMappings` is null/absent, `defaultRole` is ignored (all users get viewer).

**Source**: Helm values; passed to API via CLI flags or environment variables at startup.

**Cite FR-007, FR-008**.

---

### Runtime Go Struct Representation

**File**: `api/internal/auth/registry.go`

**Struct Definitions** (existing, no changes needed):

```go
// RoleMappings maps IdP groups to dashboard roles.
type RoleMappings struct {
    Admin    []string `json:"admin,omitempty"`
    Operator []string `json:"operator,omitempty"`
    Viewer   []string `json:"viewer,omitempty"`
}

// Provider represents an OIDC provider (database-stored or Helm-synthesized).
type Provider struct {
    Name           string          `json:"name"`                         // unique identifier
    Kind           string          `json:"kind"`                         // "oidc"
    DisplayName    string          `json:"displayName"`                  // user-visible label
    Enabled        bool            `json:"enabled"`                      // always true for Helm provider
    // ... other fields (issuer, clientID, configRef, scopes) ...
    GroupsClaim    string          `json:"groupsClaim,omitempty"`        // "" = "groups"
    RoleMappings   *RoleMappings   `json:"roleMappings,omitempty"`       // nil = no mapping
    DefaultRole    string          `json:"defaultRole,omitempty"`        // "" = viewer
}
```

**Cite FR-007, FR-008**.

---

### Role Assignment Function Signature

**Function**: `computeRole(groups []string, pol *ProviderPolicy) (role string, deny bool)`

**Parameters**:

- `groups` ([]string): List of groups extracted from the OIDC token's group claim. May be nil or empty if the claim is absent.
- `pol` (*ProviderPolicy): The provider's policy configuration, carrying both the role mapping rules and the default role fallback. `ProviderPolicy` is a struct with fields: `Scopes` ([]string), `GroupsClaim` (string), `RoleMappings` (*RoleMappings — nil if no mappings configured), and `DefaultRole` (string). Nil policy means mapping is off.

**Return Values**:

- `role` (string): The computed role: `"admin"`, `"operator"`, `"viewer"`, or `""` (empty, interpreted as "viewer").
- `deny` (bool): True if the computed role is `"deny"` (login refused); false otherwise.

**Semantics**:

1. If `pol == nil` or `pol.RoleMappings == nil`, return `("viewer", false)` (no mapping configured, all users get viewer).
2. If `groups` is empty or nil and `pol.RoleMappings != nil`, skip matching and jump to step 4.
3. **Group matching** (if mappings are set and groups present):
   - Check if any group in `groups` is in `pol.RoleMappings.Admin`. If yes, return `("admin", false)`.
   - Check if any group in `groups` is in `pol.RoleMappings.Operator`. If yes, return `("operator", false)`.
   - Check if any group in `groups` is in `pol.RoleMappings.Viewer`. If yes, return `("viewer", false)`.
4. **Fallback to default role**:
   - Normalize `pol.DefaultRole`: if empty, treat as `"viewer"`.
   - If normalized role is `"deny"`, return `("", true)` (login denied).
   - Otherwise, return `(normalizedRole, false)`.

**Return Examples**:

- `computeRole(["gameplane-admins", "users"], &ProviderPolicy{RoleMappings: &RoleMappings{Admin: ["gameplane-admins"]}, DefaultRole: "viewer"})` → `("admin", false)`.
- `computeRole(["users"], &ProviderPolicy{RoleMappings: &RoleMappings{Admin: ["admins"], Operator: ["ops"]}, DefaultRole: "viewer"})` → `("viewer", false)` (no match, use default).
- `computeRole([], &ProviderPolicy{RoleMappings: &RoleMappings{...}, DefaultRole: "deny"})` → `("", true)` (login denied).
- `computeRole(["users"], nil)` → `("viewer", false)` (no policy/mappings configured).

**Cite FR-010, FR-011, FR-012**.

---

### Group Extraction from OIDC Token

**Function**: `extractGroups(rawClaims map[string]any, claimName string) []string`

**Parameters**:

- `rawClaims` (map[string]any): The raw, untyped OIDC ID token claims.
- `claimName` (string): The claim key to extract groups from (e.g., `"groups"`, `"roles"`). If empty, defaults to `"groups"`.

**Return Type**: `[]string` (slice of group names; nil if claim is absent or not a list)

**Semantics**:

1. If `claimName` is empty, use `"groups"`.
2. Look up `rawClaims[claimName]`.
3. If not found, return `nil`.
4. If found and is a JSON array, extract all string elements (skip non-string elements).
5. If found and is a bare string, return a single-element slice `[]string{value}`.
6. Otherwise (e.g., object, number), return `nil`.

**Return Examples**:

- Token claim `"groups": ["admins", "users"]` → `["admins", "users"]`.
- Token claim `"groups": "admins"` (bare string) → `["admins"]`.
- Token missing `"groups"` claim → `nil`.
- Token claim `"groups": [null, "users", 123, "admins"]` → `["users", "admins"]` (skips non-string).
- Token claim `"groups": {"nested": "value"}` (object) → `nil`.

**Cite FR-010**.

---

### Database Persistence

**Table**: `config` (existing table, no schema changes needed)

**Key**: `"auth"` (string, PRIMARY KEY)

**Value**: JSON blob containing the entire `AuthCfg` struct

**Migration**: No new migration required; OIDC role mapping is stored within the existing `"auth"` config key, same as provider configurations and other auth settings.

**Example Value** (JSON):

```json
{
  "providers": [
    {
      "name": "okta",
      "kind": "oidc",
      "displayName": "Okta",
      "groupsClaim": "groups",
      "roleMappings": {
        "admin": ["okta-admins"],
        "operator": ["okta-operators"]
      },
      "defaultRole": "viewer"
    }
  ]
}
```

**Cite FR-011**.

---

## Entity 4: Synthetic Helm-Owned OIDC Provider

### Provider Synthesis at Runtime

**File**: `api/internal/auth/registry.go` (function `Enabled()`, lines 199-203, approx)

**Constant**: `HelmProviderName = "helm"` (line 30)

**Synthesized Provider Fields**:

| Field | Value | Mutability | Source |
|-------|-------|-----------|--------|
| **Name** | `"helm"` (constant) | Immutable | Reserved name; guards against collision |
| **Kind** | `"oidc"` | Immutable | Provider type |
| **DisplayName** | Config-time value from CLI flag (e.g., `"Single sign-on"`) | Set at startup, read-only at runtime | `--oidc-display-name` flag or config |
| **Enabled** | `true` | Immutable | Always enabled if OIDC is configured in Helm |
| **Issuer** | Empty (stored in separate *OIDC instance) | Read-only | `--oidc-issuer` CLI flag; Helm key `api.oidc.issuer` |
| **ClientID** | Empty (stored in separate *OIDC instance) | Read-only | `--oidc-client-id` CLI flag; Helm key `api.oidc.clientID` |
| **ClientSecret** | Not exposed (stored securely) | Read-only | `--oidc-client-secret` CLI flag; Helm key `api.oidc.clientSecretRef` |
| **Scopes** | Empty (stored in *OIDC policy) | Read-only | Hardcoded or `--oidc-scopes` |
| **GroupsClaim** | From OIDC policy; "" defaults to "groups" | Read-only | Helm key `api.oidc.groupsClaim` (via `--oidc-groups-claim` flag) |
| **RoleMappings** | From OIDC policy; nil if not configured | Read-only | Helm key `api.oidc.roleMappings` (via three `--oidc-role-mapping-*` flags) |
| **DefaultRole** | From OIDC policy; "" defaults to "viewer" | Read-only | Helm key `api.oidc.defaultRole` (via `--oidc-default-role` flag) |

**Immutability Rules**:

1. **Cannot be deleted**: The synthetic provider is listed in `Enabled()` response but has no DELETE endpoint.
2. **Cannot be edited**: Dashboard PUT `/admin/config/auth` cannot target the "helm" provider. Validation in `validateAuth()` (line 268-269, `handlers/config.go`) rejects any `Provider.Name == "helm"`.
3. **Cannot be created**: Dashboard POST cannot create a provider named "helm"; validation rejects it.

**Cite FR-007, Assumption 3**.

---

### Provider Initialization from CLI Flags

**File**: `api/cmd/main.go`

**Flags** (existing):

- `--oidc-issuer` (string): OIDC provider issuer URL (e.g., `https://okta.example.com`).
- `--oidc-client-id` (string): OAuth client ID from OIDC provider.
- `--oidc-client-secret` (string): OAuth client secret.
- `--oidc-redirect-url` (string): Redirect URL after OIDC login (e.g., `https://gameplane.example.com/auth/oidc/callback`).
- `--oidc-display-name` (string): Display label for the OIDC provider in the login UI (e.g., `"Okta Single Sign-On"`).

**Flags (new for role mapping)**:

- `--oidc-groups-claim` (string, optional): OIDC token claim name holding groups/roles. Empty defaults to `"groups"`. Environment variable: `GAMEPLANE_OIDC_GROUPS_CLAIM`.
- `--oidc-default-role` (string, optional): Default role for users without a group match. Allowed values: `""` (empty = viewer), `"viewer"`, `"operator"`, `"admin"`, `"deny"`. Environment variable: `GAMEPLANE_OIDC_DEFAULT_ROLE`.
- `--oidc-role-mapping-admin` (string, optional): Comma-separated list of group names that map to admin role (e.g., `"gameplane-admins,infrastructure-team"`). Environment variable: `GAMEPLANE_OIDC_ROLE_MAPPING_ADMIN`.
- `--oidc-role-mapping-operator` (string, optional): Comma-separated list of group names that map to operator role (e.g., `"gameplane-operators"`). Environment variable: `GAMEPLANE_OIDC_ROLE_MAPPING_OPERATOR`.
- `--oidc-role-mapping-viewer` (string, optional): Comma-separated list of group names that map to viewer role (e.g., `"gameplane-viewers"`). Environment variable: `GAMEPLANE_OIDC_ROLE_MAPPING_VIEWER`.

**Parsing & Initialization** (pseudo-code):

```go
// In api/cmd/main.go startup:
var oidcGroupsClaim string
var oidcDefaultRole string
var oidcRoleMappingAdmin string
var oidcRoleMappingOperator string
var oidcRoleMappingViewer string

flag.StringVar(&oidcGroupsClaim, "oidc-groups-claim", "", 
    "OIDC token claim name for groups/roles. "+envOr("GAMEPLANE_OIDC_GROUPS_CLAIM", ""))
flag.StringVar(&oidcDefaultRole, "oidc-default-role", "", 
    "Default role for users without a group match. "+envOr("GAMEPLANE_OIDC_DEFAULT_ROLE", ""))
flag.StringVar(&oidcRoleMappingAdmin, "oidc-role-mapping-admin", "", 
    "Comma-separated groups that map to admin role. "+envOr("GAMEPLANE_OIDC_ROLE_MAPPING_ADMIN", ""))
flag.StringVar(&oidcRoleMappingOperator, "oidc-role-mapping-operator", "", 
    "Comma-separated groups that map to operator role. "+envOr("GAMEPLANE_OIDC_ROLE_MAPPING_OPERATOR", ""))
flag.StringVar(&oidcRoleMappingViewer, "oidc-role-mapping-viewer", "", 
    "Comma-separated groups that map to viewer role. "+envOr("GAMEPLANE_OIDC_ROLE_MAPPING_VIEWER", ""))

// Parse and build the OIDC policy
var roleMappings *auth.RoleMappings
if oidcRoleMappingAdmin != "" || oidcRoleMappingOperator != "" || oidcRoleMappingViewer != "" {
    roleMappings = &auth.RoleMappings{
        Admin:    strings.Split(strings.TrimSpace(oidcRoleMappingAdmin), ","),
        Operator: strings.Split(strings.TrimSpace(oidcRoleMappingOperator), ","),
        Viewer:   strings.Split(strings.TrimSpace(oidcRoleMappingViewer), ","),
    }
}

oidcPolicy := &auth.OIDCPolicy{
    GroupsClaim:  oidcGroupsClaim,    // "" -> defaults to "groups" in handler
    RoleMappings: roleMappings,        // nil if no mappings provided
    DefaultRole:  oidcDefaultRole,     // "" -> defaults to "viewer" in computeRole
}
// Initialize the OIDC authenticator with the policy
oidcAuth := auth.NewOIDCWithPolicy(cfg.oidcIssuer, ..., oidcPolicy)
```

**Cite FR-007, FR-008**.

---

## Entity 5: Audit Event on OIDC Role Assignment

### Event Recording at Login

**Function**: `WriteSync(ctx context.Context, method string, path string, target string, reason string, status int) error`

**File**: `api/internal/audit/audit.go` (mirrors the audit recording signature used throughout api/internal/handlers/)

**Event Type**: OIDC login with role assignment (via D1 binding decision)

**Mechanism**: When an OIDC user logs in via the callback handler (`api/internal/auth/oidc.go`), if the login succeeds and role mappings are configured (i.e., `syncUserRole` is called), the login handler calls `auditor.WriteSync()` after the role assignment completes. The audit event is emitted once the user's role is persisted.

**Event Fields** (via `WriteSync` signature):

| Field | Value | Example |
|-------|-------|---------|
| **Method** | HTTP method of the callback | `"POST"` |
| **Path** | Request path | `"/auth/oidc/callback"` |
| **Target** | Resource identifier (username, email, or sub) | `"alice@example.com"` or `"alice"` |
| **Reason** | Concrete reason string (single format for all OIDC role assignments) | See format below |
| **Status** | HTTP status code | `200` (login succeeded) |

**Audit Table Columns** (from `api/internal/db/migrations/001_init.sql`, with "reason" added in migration 007):

- `id` (INTEGER PRIMARY KEY AUTOINCREMENT)
- `ts` (TEXT NOT NULL DEFAULT (datetime('now')))
- `actor` (TEXT NOT NULL) — extracted from ID token or system
- `method` (TEXT NOT NULL) — HTTP method
- `path` (TEXT NOT NULL) — request path
- `target` (TEXT) — resource identifier (username)
- `status` (INTEGER NOT NULL) — HTTP status code
- `ip` (TEXT) — client IP
- `reason` (TEXT) — action code and metadata (added in migration 007)

**Reason String Format** (single concrete format for all OIDC role assignments):

```
oidc role assigned: provider=<provider_name> matched=<claim_value_or_none> from=<old_role> to=<new_role>
```

**Format Semantics**:

- `provider=<provider_name>`: The OIDC provider that performed the role assignment (e.g., `"helm"`, `"okta"`).
- `matched=<claim_value_or_none>`: The specific group claim value that matched a mapping rule (e.g., `"gameplane-admins"`), OR the literal string `"none"` if the user's groups did not match any rule and the default role was applied.
- `from=<old_role>`: The user's role before this login. Use the literal string `"new_user"` if this is the user's first login (the canonical sentinel for first login). Otherwise use the previous role: `"viewer"`, `"operator"`, or `"admin"`. **Note**: `from=new_user` is deliberately distinct from `matched=none` to avoid ambiguity — the former signals "no prior role" while the latter signals "no mapping rule matched"; using both sentinels in the same string prevents confusion between these two distinct conditions.
- `to=<new_role>`: The role assigned by the mapping (or default): `"viewer"`, `"operator"`, `"admin"`, or `"denied"` (if default role was set to deny).

**Example Reason Strings**:

- First login, matched admin group: `"oidc role assigned: provider=helm matched=gameplane-admins from=new_user to=admin"`
- First login, no group match, default role applied: `"oidc role assigned: provider=helm matched=none from=new_user to=viewer"`
- Subsequent login, role upgraded from viewer to operator: `"oidc role assigned: provider=helm matched=gameplane-operators from=viewer to=operator"`
- Subsequent login, role unchanged: `"oidc role assigned: provider=helm matched=gameplane-admins from=admin to=admin"`

**Example Audit Rows** (after login with role assignment):

| id | ts | actor | method | path | target | status | ip | reason |
|----|----|----|--------|------|--------|--------|-----|--------|
| 42 | 2026-08-25T10:30:00Z | alice@example.com | POST | /auth/oidc/callback | alice@example.com | 200 | 192.0.2.42 | oidc role assigned: provider=helm matched=gameplane-admins from=new_user to=admin |
| 43 | 2026-08-25T10:35:00Z | alice@example.com | POST | /auth/oidc/callback | alice@example.com | 200 | 192.0.2.42 | oidc role assigned: provider=helm matched=gameplane-operators from=admin to=operator |

**Implementation Notes**:
- The reason string is a single free-text field (no structured metadata columns); all information is captured in this single format.
- The audit_events table has exactly these columns: `id, ts, actor, method, path, target, status, ip, reason` (the "reason" column was added in migration 007; see `api/internal/db/migrations/007_add_reason.sql`).
- Bootstrap-admin-created accounts (with no OIDC link) do NOT trigger OIDC audit events.
- The reason format is consistent across all OIDC role assignments: first login, re-evaluation on subsequent login, and role changes due to updated mappings all use the same template.

**Cite FR-014, D1**.

---

## Precedence & Resolution Tables

### Storage Class Resolution Precedence

**Precedence Order** (highest to lowest):

| Rank | Source | Condition | Value |
|------|--------|-----------|-------|
| **1** | GameServer | `gs.Spec.Storage.StorageClassName != nil` | Use `gs.Spec.Storage.StorageClassName` |
| **2** | GameTemplate | `tmpl.Spec.Storage.StorageClassName != nil` | Use `tmpl.Spec.Storage.StorageClassName` |
| **3** | Operator Install-Time Default | `--game-data-storage-class != ""` | Use `--game-data-storage-class` value |
| **4** | Cluster Default | (final fallback) | Leave `PVC.Spec.StorageClassName` unset (nil); Kubernetes uses cluster default |

**Decision Table** (what `PVC.Spec.StorageClassName` gets set to):

| Scenario | GameServer Override | Template Default | Operator Default | Result |
|----------|-------------------|------------------|------------------|--------|
| Explicit override | `"fast-nvme"` | (any) | (any) | `"fast-nvme"` |
| Template + operator default | `nil` | `"standard"` | `"gpu-attached"` | `"standard"` |
| Operator default only | `nil` | `nil` | `"local-nvme"` | `"local-nvme"` |
| Cluster default | `nil` | `nil` | `""` | `nil` (cluster default) |

**Cite FR-002, FR-004, SC-001, SC-008**.

---

### OIDC Role Resolution Precedence

**Precedence Order** (highest to lowest):

| Rank | Source | Condition | Role Assignment |
|------|--------|-----------|-----------------|
| **1** | Group Membership Match | User's OIDC group in `roleMappings.Admin` | `"admin"` |
| **2** | Group Membership Match | User's OIDC group in `roleMappings.Operator` | `"operator"` |
| **3** | Group Membership Match | User's OIDC group in `roleMappings.Viewer` | `"viewer"` |
| **4** | Default Role Fallback | No group match; role mappings configured | Use `provider.DefaultRole` |
| **5** | Built-In Default | No role mappings configured | `"viewer"` |

**Decision Table** (what role is assigned at login):

| User Groups | Admin Mapping | Operator Mapping | Viewer Mapping | Default Role | Result |
|-------------|--------------|-----------------|----------------|--------------|--------|
| `["gameplane-admins"]` | includes "gameplane-admins" | — | — | (any) | `"admin"` |
| `["gameplane-ops"]` | (no match) | includes "gameplane-ops" | — | (any) | `"operator"` |
| `["users"]` | (no match) | (no match) | includes "users" | (any) | `"viewer"` |
| `["unknown"]` | (no match) | (no match) | (no match) | `"operator"` | `"operator"` |
| `[]` (empty groups) | (no match) | (no match) | (no match) | `""` (empty) | `"viewer"` |
| `["any"]` | (mappings nil) | — | — | (ignored) | `"viewer"` |

**Cite FR-009, FR-010, FR-011, FR-012, SC-003, SC-004, SC-005**.

---

### Role Re-Evaluation on Subsequent Logins

**Condition**: User logs in for the second or later time.

**Behavior**:

- **If provider has role mappings** (`provider.RoleMappings != nil`):
  - Re-evaluate the user's groups against current mappings.
  - Update `users.role` and `user_role_bindings` to the newly computed role.
  - **Guard**: If the new role would remove the last user with user-management permission (admin or operator), skip the update and leave the stored role unchanged. Log a warning.

- **If provider has no role mappings** (`provider.RoleMappings == nil`):
  - Leave the user's stored role unchanged. Do not re-evaluate.
  - (This preserves role stability for OIDC-only installs that manually set up roles via `/admin/config` after first login.)

**Cite FR-011, SC-005**.

---

## Backward Compatibility

### Storage Class Setting (SC-008 Mapping)

**Compatibility Guarantee**: Gameplane installs without an explicit storage class setting default to the cluster's default StorageClass; no breaking changes.

| Change | Impact | Migration Path | Verification |
|--------|--------|-----------------|--------------|
| New Helm value `operator.gameDataStorage.storageClassName` | Optional; empty string (default) means use cluster default. Existing installs (no value set) behave identically to setting the value to `""`. | No migration needed; leave value unset (or set to `""`) to maintain status quo. | Existing GameServers' PVCs continue to use cluster default if no explicit override is set on the template/server. |
| New operator CLI flag `--game-data-storage-class` | Optional; empty string (default) means use cluster default. | If operators previously used environment variables or config files to pass the storage class, they can continue; the new flag is additive. | PVCs created after flag adoption use the flag value (if non-empty); PVCs created before adoption use cluster default as before. |
| New reconciliation logic in operator | Adds fallback to operator default after template/server precedence. | No breaking change; if no operator default is set (flag absent or empty), behavior is identical to pre-feature behavior. | New GameServers use operator default if set; existing GameServers' bindings do not change (PVCs are immutable). |

**Cite SC-008, Assumption 6**.

---

### OIDC Role Mapping Setting (SC-008 Mapping)

**Compatibility Guarantee**: Gameplane installs with OIDC enabled but no role mappings configured default all OIDC users to the viewer role; no breaking changes.

| Change | Impact | Migration Path | Verification |
|--------|--------|-----------------|--------------|
| New Helm values `api.oidc.groupsClaim`, `api.oidc.roleMappings.*`, `api.oidc.defaultRole` | Optional; if not set, OIDC behaves as before (no role mappings, all users get viewer). Existing installs see no change. | No migration needed; Helm values are optional and default to nil/empty. Existing installs without these values behave identically. | OIDC users logging in without role mappings configured receive viewer role, same as before. Role mappings take effect only if explicitly configured via new Helm values. |
| New API CLI flags `--oidc-groups-claim`, `--oidc-role-mapping-admin`, `--oidc-role-mapping-operator`, `--oidc-role-mapping-viewer`, `--oidc-default-role` | Optional; empty/null values mean no role mappings, identical to pre-feature behavior. | If operators previously manually configured role mappings via `/admin/config`, those configurations persist in the database. The new Helm-based config is a separate, synthetic provider ("helm") and does not interact with database-stored providers. | Existing OIDC users' roles are not retroactively changed; role mappings apply on next login. If a new mapping is configured at install time, it takes effect for all future logins (including existing users on their next login). |
| Database provider synthesis at runtime | New synthetic "helm" provider is listed alongside database-stored providers; it is read-only and cannot be edited/deleted via the dashboard. | No database migration needed; the synthetic provider is constructed at runtime from CLI flags/env vars. | Dashboard `/admin/config/auth` lists the Helm provider as a separate, non-editable entry; operators can still manage other (database-stored) OIDC providers via the dashboard. |

**Cite SC-008, Assumption 3, Assumption 5, Assumption 6**.

---

### Bootstrap-Admin Coexistence

**Compatibility Guarantee**: bootstrap-admin and OIDC-mapped users coexist peacefully; both are valid ways to be admin.

**Scenario**: Operator runs `bootstrap-admin` to create a local admin account on a cluster with OIDC role mappings already configured.

| Step | Entity | State | Notes |
|------|--------|-------|-------|
| 1 | OIDC role mappings (Helm) | Configured | `operator.oidc.helmProvider.roleMappings` set at install time. |
| 2 | bootstrap-admin | Executed | Creates a local user with admin role; no OIDC link. |
| 3 | Local admin user | Created in database | User has `users.role = "admin"`; no OIDC link in `oidc_links` table. |
| 4 | OIDC user logs in | Happens after bootstrap-admin | OIDC user's role is computed from mappings (unchanged). |
| 5 | Both admins coexist | Active | Local admin and OIDC-mapped admin(s) both have admin access; no conflict. |

**Cite Assumption 5, FR-013**.

---

### Existing GameServers & PVCs

**Compatibility Guarantee**: Existing GameServers' PVCs are not affected by install-time configuration changes; only new PVCs use the new defaults.

**Scenario**: Operator installs Gameplane, then runs `helm upgrade` to change the default storage class.

| Step | PVC State | Notes |
|------|-----------|-------|
| 1 | Existing GameServers' PVCs | Bound to their original StorageClass (e.g., cluster default). |
| 2 | Helm chart updated | New value for `operator.gameDataStorage.storageClassName` set. |
| 3 | Operator pod restarted | New default is loaded into `--game-data-storage-class` flag. |
| 4 | New GameServer created | Uses the new default StorageClass (because existing GameServers' PVCs are immutable, they do not retarget). |
| 5 | Existing GameServers' PVCs | Still bound to their original class; no retargeting. |

**Cite Assumption 7, SC-008**.

---

## Reference to Verification & Testing

### Test Coverage Map

**Unit/Envtest Tests** (no CI cluster overhead):

- `api/internal/auth/oidc_rolemap_test.go`: Extend `TestComputeRole()` to cover Helm provider policy, "deny" default role, empty groups with admin-mapped fallback.
- `operator/internal/controller/gameserver_storage_envtest_test.go` (new): Test GameServer reconciliation with storage class resolution (default, override, error paths).

**E2E Tests** (in CI):

- `test/e2e/api_auth_e2e_test.go`: `TestAPI_OIDCRoleMappingAtInstallTime`, `TestAPI_OIDCRoleMappingFirstLogin`, `TestAPI_OIDCRoleMappingReEvalOnLogin` (api-auth bucket, ~2-3 logins total).
- `test/e2e/gameserver_e2e_test.go`: `TestGameServer_StorageClassFromHelmDefault`, `TestGameServer_StorageClassNotFound`, `TestGameServer_StorageClassExplicitOverride` (operator bucket, zero logins).

**Register new tests** in `test/e2e/buckets.sh` before merge.

---

## Summary

This data model defines:

1. **Storage Class Resolution**: Helm values + operator CLI flags → precedence-ordered PVC materialization across three PVC types (data, extra volumes, mods).
2. **Status Reporting**: Existing `Conditions[]` mechanism used; no new status fields. Error messages rendered on `ServerDetail` UI.
3. **OIDC Role Mapping**: Helm values + CLI flags → synthetic "helm" provider → immutable, read-only at runtime. Role assignment computed at login, re-evaluated on subsequent logins if mappings configured.
4. **Audit Trail**: OIDC role assignments recorded as audit events; reason codes distinguish initial assignment from updates.
5. **Backward Compatibility**: All changes are optional (empty/nil defaults preserve pre-feature behavior); existing GameServers unaffected by install-time defaults; bootstrap-admin and OIDC coexist.

All entities resolve in consistent precedence order; see decision tables for role and storage class.

