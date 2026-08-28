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

### `installTimeSettings` Response Shape (Two Fields, Not One)

**Orchestrator decision (binding)**: `installTimeSettings` carries **both** of the following. It is
not `gameDataStorageClass` alone — FR-017 and SC-006 require an operator to be able to view the
active OIDC role mappings in the admin interface, and the dashboard's client-side effective-value
rendering (see Entity 3a's key-presence provenance) needs the Helm-seeded values to compare the
`helmOverride` overlay against.

```json
{
  "installTimeSettings": {
    "gameDataStorageClass": "local-nvme",
    "oidcHelmProvider": {
      "groupsClaim": "groups",
      "defaultRole": "viewer",
      "roleMappings": {
        "admin": ["gameplane-admins", "infrastructure-team"],
        "operator": ["gameplane-operators"],
        "viewer": []
      }
    }
  }
}
```

**Fields**:

- **`gameDataStorageClass`** (string): the operator's install-time default StorageClass name, as
  described above. Empty string means "use cluster default."
- **`oidcHelmProvider`** (object): a **read-only snapshot of the Helm-seeded OIDC role-mapping
  policy** — the same values used to build the synthetic `"helm"` provider's `ProviderPolicy`
  (Entity 4), reported here purely for display. It is never itself writable through
  `installTimeSettings`; the only way to change effective role assignment is the `helmOverride`
  overlay on the `"auth"` config row (Entity 3a) via `PUT /admin/config/auth` and
  `DELETE /admin/config/auth/role-mappings/{role}` (Entity 4b, M7).
  - `groupsClaim` (string): the Helm-configured claim name (`api.oidc.groupsClaim`); `""` displayed
    as `"groups"` per the existing default.
  - `defaultRole` (string): the Helm-configured fallback role (`api.oidc.defaultRole`); Helm-only,
    no override path in v1 (M5).
  - `roleMappings` (object): the Helm-seeded `admin`/`operator`/`viewer` group lists
    (`api.oidc.roleMappings.*`), exactly as passed via the `--oidc-role-mapping-*` flags.
  - Omitted entirely (or `null`) when OIDC is not configured in Helm at all — mirrors
    `helmOIDCPresent` being false.
- **Why the dashboard needs this**: provenance in the `helmOverride` response is key-presence only
  (Entity 3a, V3) — there is no server-computed "effective" view and no `source` field anywhere in
  this feature. To render each role's *effective* mapping (Helm-seeded value, or the override that
  replaces it), the dashboard combines `installTimeSettings.oidcHelmProvider.roleMappings` (the seed)
  with `helmOverride.roleMappings` (the overlay) entirely client-side: a role key present in
  `helmOverride.roleMappings` means overridden (show the override's list, offer "reset"); a role key
  absent falls back to `oidcHelmProvider.roleMappings` for that role (show the seed, no reset
  action). No new server-side merge logic is introduced for this display — `effectiveHelmPolicy`
  (Entity 3b) exists only for login-time role resolution, not for building an HTTP response.

**Source**: `api/internal/handlers/config.go` (`getAll` handler, or equivalent install-time-settings
assembly point); populated from the same CLI-flag-derived values used to build the operator's PVC
defaults and the synthetic `"helm"` provider's `ProviderPolicy`.

**Cite FR-001, FR-002, FR-017, SC-006, D2, M2, M5**.

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

## MAINTAINER DECISION: Hybrid Role-Mapping Layering (Helm seeds, DB overrides)

> Binding resolution of the FR-007 / SC-007 tension. Supersedes any prior "Helm provider is the
> only source of role mappings" framing below. See H1–H8 in the task brief. The synthetic `"helm"`
> provider (Entity 4) is unchanged and still read-only (H4) — the overlay described here is a
> **separate field on the existing `"auth"` config row**, not an edit to that provider.
>
> **No new table, no new migration (M1).** The overlay is carried inside the `config` table's
> existing `"auth"` row (`api/internal/auth/registry.go:175`, `179-185`), as one optional sibling
> field next to `"providers"`. `api/internal/db/migrations/` stays at `008_captures_rbac.sql` —
> this feature adds zero files there.

### Entity 3a: `helmOverride` — DB-Persisted Role-Mapping Overlay (admin-managed)

**Storage**: the existing `config` table, row `key = "auth"` (no schema change). The JSON value
gains one optional top-level sibling to `"providers"`:

```json
{
  "providers": [ /* ... unchanged Provider[] ... */ ],
  "helmOverride": {
    "roleMappings": {
      "admin": ["gameplane-admins", "infrastructure-team"],
      "operator": ["gameplane-operators"],
      "viewer": []
    }
  }
}
```

**Shape rules (M2)**:

- `helmOverride` (object, optional): entirely absent means no override anywhere — every role falls
  through to the Helm-seeded value. Present-but-empty (`{"helmOverride": {}}` or
  `{"helmOverride": {"roleMappings": {}}}`) is equivalent to absent.
- `helmOverride.roleMappings` (object, optional): the three keys `admin`, `operator`, `viewer` are
  **each independently optional**.
  - **Key absent / JSON `null`** for a role: no override for that role — the Helm-seeded list for
    that role stands.
  - **Key present as an array** (including `[]`): that array **replaces** the Helm-seeded list for
    that role. `[]` is a valid, meaningful override: "nobody maps to this role from any group"
    (M3) — it is not the same as "no override" and is not coalesced with `null`/absent.
- `helmOverride` is **not** a `Provider` entry. It is never appended to, matched against, or
  validated by the `providers` array logic — the `"helm"` reserved-name guard at
  `api/internal/handlers/config.go` (`validateAuth`, the `p.Name == "helm"` check) is unchanged and
  is not relaxed or bypassed by this field.
- `groupsClaim` and `defaultRole` have **no override path in v1** (M5): they are Helm-only, read
  only from the synthetic `"helm"` provider's `ProviderPolicy`. `helmOverride` never carries either
  field, and no endpoint accepts them.

**Cite M1, M2, M5, FR-007, SC-007**.

---

### Entity 3b: Merged Role-Resolution Policy (runtime Go shape)

**Verified existing `ProviderPolicy`** (`api/internal/auth/oidc.go:29-34`; this is the real type —
there is no `auth.OIDCPolicy`):

```go
type ProviderPolicy struct {
	Scopes       []string
	GroupsClaim  string
	RoleMappings *RoleMappings
	DefaultRole  string
}
```

**Verified existing `RoleMappings`** (`api/internal/auth/registry.go:53-57`):

```go
type RoleMappings struct {
	Admin    []string `json:"admin,omitempty"`
	Operator []string `json:"operator,omitempty"`
	Viewer   []string `json:"viewer,omitempty"`
}
```

Both structs are unchanged — `ProviderPolicy` still represents the Helm-seeded policy exactly as
before. `computeRole` (`api/internal/auth/oidc.go:121`) is **not modified** (M4): its existing
admin > operator > viewer tie-break, its existing fallback to `DefaultRole`, and its existing tests
stay exactly as they are.

**New helper signature** (M4 — signature only, no body; `api/internal/auth`):

```go
// effectiveHelmPolicy merges the DB overlay's per-role list replacements (M3)
// onto the Helm-seeded base policy, producing the *ProviderPolicy that
// computeRole is then called with, unmodified.
func effectiveHelmPolicy(base *ProviderPolicy, ov *RoleMappings) *ProviderPolicy
```

**Parameters**:

- `base` (`*ProviderPolicy`): the Helm-seeded policy for the synthetic `"helm"` provider, exactly as
  built from CLI flags today.
- `ov` (`*RoleMappings`): the `helmOverride.roleMappings` value decoded from the `"auth"` config row
  (Entity 3a); `nil` when `helmOverride` is absent.

**Return**: a `*ProviderPolicy` with `RoleMappings` merged per role (M3) and `Scopes`, `GroupsClaim`,
`DefaultRole` carried through unchanged from `base` (M5 — those fields are never overridden).

**Call-site semantics**: `effectiveHelmPolicy` runs once per login attempt against the `"helm"`
provider, and its result is passed to the existing, unmodified `computeRole(groups, pol)`. No new
call path for DB-stored (non-Helm) OIDC providers is introduced — the overlay applies only to the
synthetic `"helm"` provider's policy, consistent with M2's storage shape living beside `providers`
rather than inside one.

**Cite M3, M4, FR-010, FR-011, FR-012**.

---

### Merge Algorithm (M3 — per-role list replacement, not per-claim-value lookup)

`effectiveHelmPolicy` merges **whole lists per role**, independently for each of the three roles:

1. For each role in `{admin, operator, viewer}`: if `helmOverride.roleMappings` supplies a
   **non-nil** list for that role, that list **replaces** the Helm-seeded list for that role in
   full (not a union, not a per-value lookup).
2. Otherwise (the role's key is absent or `null`), the Helm-seeded list for that role stands
   unchanged.
3. An empty, non-nil list (`[]`) is a valid override meaning "nobody maps to this role" — it still
   replaces (to an empty set), it does not fall back to the Helm-seeded list.
4. The three roles are resolved **independently** — overriding `viewer` has no effect on whether
   `admin` or `operator` are overridden.

**Worked case (M4 — most-privileged-match-still-wins, stated explicitly)**: because the per-role
merge happens *before* `computeRole` runs, a user whose groups match an **overridden viewer** group
*and* a **Helm-seeded admin** group still resolves to **admin** — `computeRole`'s admin > operator >
viewer tie-break is evaluated against the *merged* policy, so the most privileged match across both
sources wins, exactly as it would if both lists had come from Helm alone. An earlier draft of this
document stated this backwards (claiming the override always wins regardless of which role
matched); that framing is wrong and is corrected here.

**Cite M3, M4, FR-010, FR-011, FR-012**.

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

- **defaultRole** (string, optional): Role assigned to users whose group does not match any mapping. Allowed values: `""` (empty, interpreted as "viewer"), `"viewer"`, `"operator"`, `"admin"`, `"deny"` (deny refuses login). Defaults to `""` (viewer). **Helm-only in v1 (M5)**: `defaultRole` has no DB override path — it is never a field of `helmOverride` (Entity 3a), and no endpoint accepts a write to it. The same is true of `groupsClaim`.

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

**H4 — unchanged under the hybrid model**: the hybrid layering (Entity 3a/3b) does NOT make this
provider editable. `helmOverride` is a sibling field of the `"auth"` config row's JSON (M2), merged
into a *derived* `*ProviderPolicy` by `effectiveHelmPolicy` (M4) only at resolution time — it never
writes to, reads from, or is merged into the stored `Provider`/`ProviderPolicy` struct for `"helm"`
itself, and the synthesized `Provider{Name: "helm", ...}` record is unchanged by this feature. The
dashboard's new editing surface (M7) writes `helmOverride`, not fields on the `"helm"` provider
record. Do not describe the helm provider itself as becoming editable.

**Cite FR-007, H4, Assumption 3**.

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

// envOr sits in the default-value position, exactly as every existing
// flag registration does (verified pattern, api/cmd/main.go:391-395 —
// e.g. fs.StringVar(&c.oidcIssuer, "oidc-issuer", envOr("GAMEPLANE_OIDC_ISSUER", ""), "OIDC issuer URL")).
// It is never concatenated into the usage string.
fs.StringVar(&oidcGroupsClaim, "oidc-groups-claim", envOr("GAMEPLANE_OIDC_GROUPS_CLAIM", ""),
    "OIDC token claim name for groups/roles")
fs.StringVar(&oidcDefaultRole, "oidc-default-role", envOr("GAMEPLANE_OIDC_DEFAULT_ROLE", ""),
    "Default role for users without a group match")
fs.StringVar(&oidcRoleMappingAdmin, "oidc-role-mapping-admin", envOr("GAMEPLANE_OIDC_ROLE_MAPPING_ADMIN", ""),
    "Comma-separated groups that map to admin role")
fs.StringVar(&oidcRoleMappingOperator, "oidc-role-mapping-operator", envOr("GAMEPLANE_OIDC_ROLE_MAPPING_OPERATOR", ""),
    "Comma-separated groups that map to operator role")
fs.StringVar(&oidcRoleMappingViewer, "oidc-role-mapping-viewer", envOr("GAMEPLANE_OIDC_ROLE_MAPPING_VIEWER", ""),
    "Comma-separated groups that map to viewer role")

// Parse and build the OIDC policy
var roleMappings *auth.RoleMappings
if oidcRoleMappingAdmin != "" || oidcRoleMappingOperator != "" || oidcRoleMappingViewer != "" {
    roleMappings = &auth.RoleMappings{
        Admin:    strings.Split(strings.TrimSpace(oidcRoleMappingAdmin), ","),
        Operator: strings.Split(strings.TrimSpace(oidcRoleMappingOperator), ","),
        Viewer:   strings.Split(strings.TrimSpace(oidcRoleMappingViewer), ","),
    }
}

oidcPolicy := &auth.ProviderPolicy{
    GroupsClaim:  oidcGroupsClaim,    // "" -> defaults to "groups" in handler
    RoleMappings: roleMappings,        // nil if no mappings provided
    DefaultRole:  oidcDefaultRole,     // "" -> defaults to "viewer" in computeRole
}
// Initialize the OIDC authenticator with the policy
oidcAuth := auth.NewOIDCWithPolicy(cfg.oidcIssuer, ..., oidcPolicy)
```

**Cite FR-007, FR-008**.

---

## Entity 4b: HTTP Surface for the `helmOverride` Overlay (M7)

### Existing Route — Extended, Not Replaced

**Verified current mount** (`api/internal/handlers/config.go:30-36`, `MountConfig`):

```go
func MountConfig(r chi.Router, store *db.Store, helmOIDCPresent bool) {
    h := &configHandler{db: store, validators: newValidators(helmOIDCPresent)}
    r.Route("/admin/config", func(r chi.Router) {
        r.Get("/", h.getAll)
        r.Put("/{section}", h.put)
    })
}
```

Called today as `handlers.MountConfig(p, store, oidcAuth != nil)` (`api/cmd/main.go:245`), with no
auditor — but Entity 6's audit events (M8) must be written from inside `config.go` (the `put` handler
for `"auth"` and the new reset handler below), so `MountConfig` **gains an `*audit.Auditor`
parameter**, and its call site in `api/cmd/main.go` changes accordingly. The in-tree precedent for
this exact pattern is `MountCapture(r chi.Router, reg *kube.Registry, auditor *audit.Auditor, cfg
CaptureConfig, ...)` (`api/internal/handlers/capture.go:55`), which already takes an auditor the same
way:

```go
func MountConfig(r chi.Router, store *db.Store, auditor *audit.Auditor, helmOIDCPresent bool) {
    h := &configHandler{db: store, auditor: auditor, validators: newValidators(helmOIDCPresent)}
    r.Route("/admin/config", func(r chi.Router) {
        r.Get("/", h.getAll)
        r.Put("/{section}", h.put)
        r.Delete("/auth/role-mappings/{role}", h.resetRoleMapping)
    })
}
```

```go
// api/cmd/main.go:245, updated call site:
handlers.MountConfig(p, store, auditor, oidcAuth != nil)
```

- `GET /admin/config` — returns every recognized section as `map[string]json.RawMessage`, keyed by
  section name (`config.go:43-68`, `getAll`).
- `PUT /admin/config/{section}` — validates the body against `newValidators()[section]` and upserts
  the canonicalized JSON into `config` (`config.go:70-103`, `put`). `"auth"` is one of the closed
  set of section names (`config.go:112`, `newValidators`), validated by `validateAuth(helmOIDCPresent)`
  (`config.go:235`).
- **RBAC**: `MountConfig` is mounted under `p` in `api/cmd/main.go:245`, inside a router group that
  applies `rbac.Middleware(reg)` (`main.go:235`); the `config:manage` permission
  (`api/internal/rbac/catalog.go:68`, `{Resource: "config", ...}`) is the existing gate for writes
  to this resource.

**No new resource is invented.** `helmOverride` (Entity 3a) rides inside the same `"auth"` section
body that `providers` already lives in:

- **`GET /admin/config`** — unchanged route, unchanged shape at the top level; the `"auth"` entry's
  JSON now includes `helmOverride` when set (M2). No new query param, no new response envelope.
- **`PUT /admin/config/auth`** — unchanged route; `validateAuth` is extended to also parse and
  validate the optional `helmOverride.roleMappings` field (each present role list: no empty/blank
  elements, same rule `validateProviderMapping` already applies to `providers[*].roleMappings[*]`).
  A write to this route is how an admin sets or changes a role's override.

### New Route — Reset One Role's Override (M7, the only new route in this feature)

```go
r.Delete("/auth/role-mappings/{role}", h.resetRoleMapping)
```

Mounted alongside the existing two routes, inside the same `r.Route("/admin/config", ...)` block and
under the same `rbac.Middleware` + `config:manage` gate as the `PUT /admin/config/{section}` route
above — matching the existing path convention of nesting under `/admin/config/{section}/...` rather
than a top-level resource.

- **Method**: `DELETE`
- **Path**: `DELETE /admin/config/auth/role-mappings/{role}`, `{role}` one of `admin`, `operator`,
  `viewer` (400 on any other value — mirrors the closed-enum handling `put` already does for
  `{section}`).
- **Effect**: removes that one role's key from `helmOverride.roleMappings` in the `"auth"` row
  (leaving the other two roles' overrides, if any, untouched) and re-persists the row. If the role
  had no override, this is a no-op that still returns success (idempotent reset).
- **RBAC**: `config:manage`, same as `PUT /admin/config/{section}`.
- **Response**: the updated `"auth"` section body (same envelope shape as `put`'s
  `{"section": ..., "value": ...}`), so the dashboard can refresh from the response without a
  follow-up `GET`.

### GET Response — Provenance Per Role (M7)

`GET /admin/config` must let the dashboard show, **per role**, whether the effective list came from
the DB overlay or the Helm seed — so it can render a "reset" action only where an override exists.
The `"auth"` section's `helmOverride.roleMappings` object itself carries this: a role key **present**
(even as `[]`) means DB-overridden; a role key **absent** means Helm-seeded. The dashboard derives
provenance directly from presence/absence of each key — no separate provenance field is added to the
response, since `helmOverride`'s own shape (Entity 3a, M2) already encodes it unambiguously.

**Cite M7, FR-007, SC-007**.

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

## Entity 6: Audit Events for `helmOverride` Writes and Resets (M8)

### Event Recording on Override Write / Reset

**Function** (verified, `api/internal/audit/audit.go:689`):

```go
func (a *Auditor) WriteSync(ctx context.Context, method, path, target, reason string, status int) error
```

**Trigger**: every admin-gated write that changes `helmOverride` — a `PUT /admin/config/auth` whose
body sets, changes, or removes a role's override list, and the dedicated
`DELETE /admin/config/auth/role-mappings/{role}` reset route (M7, M9) — is audited via `WriteSync`.

**RBAC gate**: both routes require the `config:manage` permission — the existing catalog entry for
`{Resource: "config", ...}` (`api/internal/rbac/catalog.go:68`, "Change global settings"); overriding
role mappings is global auth config, so no new catalog permission is introduced.

**`audit_events` columns** (verified, `api/internal/db/migrations/001_init.sql:30-39` plus `reason`
added in `007_audit_reason.sql`): `id, ts, actor, method, path, target, status, ip, reason`. There is
**no** `action`/`metadata`/`message` column — every detail rides in the free-text `reason` string
(M8), so this feature does not add or need one.

**Event Fields** (via `WriteSync`):

| Field | Value | Example |
|-------|-------|---------|
| **Method** | HTTP method of the mutating request | `"PUT"` (override write) / `"DELETE"` (reset) |
| **Path** | Request path | `"/admin/config/auth"` / `"/admin/config/auth/role-mappings/admin"` |
| **Target** | The affected role | `"admin"` |
| **Reason** | Concrete reason string identifying the change | See format below |
| **Status** | HTTP status code | `200` |

**Reason String Format** (one consistent format, used everywhere this feature writes an audit row —
including Entity 5's login-time role assignment, which already uses this same
`"<subject>: <k>=<v> <k>=<v> ..."` shape):

```
oidc role mapping override set: role=<role> groups=<comma_joined_or_none>
oidc role mapping override reset: role=<role>
```

- `role=<role>`: one of `admin`, `operator`, `viewer` — the single role this write/reset affected.
  A `PUT /admin/config/auth` that changes more than one role's override in the same request emits
  one audit row per changed role (each with its own `target`/`reason`), so every row still names
  exactly one role — consistent with `target` carrying a single value.
- `groups=<comma_joined_or_none>`: the role's new override list after the write, comma-joined (e.g.
  `"gameplane-admins,infra-team"`), or the literal `"none"` for an empty-list override (`[]` — "nobody
  maps to this role", per M3). Omitted entirely from the `reset` template since a reset has no new
  list — it restores the Helm-seeded one.
- The `actor` column (already populated by `WriteSync` from the request context) carries who made
  the change; the reason string does not duplicate it, unlike Entity 5's format — Entity 5's
  `from=`/`matched=` sentinels exist because a *login* has no separate "who" column meaning other
  than the actor being logged in, whereas here `actor` is unambiguously the admin.

**Example Audit Rows**:

| id | ts | actor | method | path | target | status | ip | reason |
|----|----|----|--------|------|--------|--------|-----|--------|
| 51 | 2026-08-26T09:12:00Z | admin@example.com | PUT | /admin/config/auth | admin | 200 | 192.0.2.10 | oidc role mapping override set: role=admin groups=gameplane-admins,infra-team |
| 52 | 2026-08-26T09:15:00Z | admin@example.com | DELETE | /admin/config/auth/role-mappings/admin | admin | 200 | 192.0.2.10 | oidc role mapping override reset: role=admin |

**Cite M7, M8, M9.**

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

### OIDC Role Resolution Precedence (M3/M4 — per-role merge, then unmodified `computeRole`)

Resolution is **two steps**, not a per-claim-value lookup chain:

1. **Per-role merge** (`effectiveHelmPolicy`, M3): for each of `admin`, `operator`, `viewer`
   independently, `helmOverride.roleMappings` supplying a non-nil list for that role **replaces**
   the Helm-seeded list for that role; an absent/`null` key leaves the Helm-seeded list standing.
2. **Unmodified `computeRole`** (M4) runs once against the *merged* policy: first match wins,
   admin > operator > viewer, exactly as it does today — falling through to `DefaultRole` (Helm-only,
   M5) and finally the built-in `"viewer"` if nothing matches.

| Step | Source | Condition | Effect |
|------|--------|-----------|--------|
| **1a** | `helmOverride.roleMappings.admin` | Key present (incl. `[]`) | Replaces the Helm-seeded admin list in the merged policy |
| **1b** | `helmOverride.roleMappings.operator` | Key present (incl. `[]`) | Replaces the Helm-seeded operator list in the merged policy |
| **1c** | `helmOverride.roleMappings.viewer` | Key present (incl. `[]`) | Replaces the Helm-seeded viewer list in the merged policy |
| **1 (no override)** | Helm-seeded value | Key absent/`null` for that role | That role's Helm-seeded list stands unchanged |
| **2a** | `computeRole` on merged policy | User's group in merged `RoleMappings.Admin` | `"admin"` |
| **2b** | `computeRole` on merged policy | ...group in merged `RoleMappings.Operator` | `"operator"` |
| **2c** | `computeRole` on merged policy | ...group in merged `RoleMappings.Viewer` | `"viewer"` |
| **3** | `computeRole` fallback | No match; mappings configured | `provider.DefaultRole` (Helm-only, M5) |
| **3 (fallback)** | Built-in default | No mappings configured anywhere (merged policy has none, or `pol == nil`) | `"viewer"` |

**Decision Table** (what role is assigned at login):

| User Groups | Admin Override | Operator Override | Viewer Override | Helm Admin Mapping | Helm Operator Mapping | Helm Viewer Mapping | Default Role | Result |
|-------------|-----------------|---------------------|-------------------|---------------------|-------------------------|------------------------|---------------|--------|
| `["gameplane-admins"]` | none (Helm stands) | — | — | includes "gameplane-admins" | — | — | (any) | `"admin"` |
| `["gameplane-admins"]` | `[]` (nobody admin) | — | — | includes "gameplane-admins" (replaced by `[]`) | — | — | `"viewer"` | `"viewer"` (override replaced the list the group would have matched) |
| `["gameplane-ops"]` | — | none (Helm stands) | — | — | includes "gameplane-ops" | — | (any) | `"operator"` |
| `["users"]` | — | — | `["users"]` (same as Helm) | — | — | includes "users" | (any) | `"viewer"` |
| `["unknown"]` | — | — | — | (no match) | (no match) | (no match) | `"operator"` | `"operator"` |
| `[]` (empty groups) | — | — | — | (no match) | (no match) | (no match) | `""` (empty) | `"viewer"` |
| `["any"]` | (no override anywhere) | (no override anywhere) | (no override anywhere) | (mappings nil) | — | — | (ignored) | `"viewer"` |

**Worked M4 case** — overridden viewer group + Helm-seeded admin group, most-privileged match still
wins:

| User Groups | Viewer Override | Admin Override | Helm Admin Mapping | Merged Result | Login Result |
|-------------|-------------------|-------------------|----------------------|----------------|--------------|
| `["dev-team", "gameplane-admins"]` | `["dev-team"]` (replaces Helm viewer list, which happened to also list "dev-team") | none (Helm stands) | includes "gameplane-admins" | merged policy has "dev-team" in `Viewer` AND "gameplane-admins" still in `Admin` (untouched) | `"admin"` — `computeRole` checks `Admin` before `Viewer`; overriding one role's list never demotes a match against a different, non-overridden role. |

**Cite M3, M4, FR-009, FR-010, FR-011, FR-012, SC-003, SC-004, SC-005, SC-007**.

---

### Upgrade-Case Rows (M9 — `helm upgrade` with changed `api.oidc.roleMappings.*` values)

| Scenario | Override Present for This Role? | Helm Value Before Upgrade | Helm Value After Upgrade | Result After Upgrade | Notes |
|----------|-------------------------------------|----------------------------|----------------------------|------------------------|-------|
| Admin previously overrode a role | Yes — `helmOverride.roleMappings.admin = ["gameplane-admins"]` (mapped by the admin to a smaller set than Helm ships) | `roleMappings.admin: [gameplane-admins]` | `roleMappings.admin: [gameplane-admins, new-admins]` (Helm-seeded list changed) | Merged admin list is still just `["gameplane-admins"]` — `new-admins` does **not** gain admin (override still wins, M9) | Helm reasserting on upgrade would silently undo the admin's edit if the override were dropped — deliberately avoided. |
| No admin edit yet for this role | No override key present for `viewer` | `roleMappings.viewer: [gameplane-viewers]` | `roleMappings.viewer: [gameplane-viewers, new-viewers]` (Helm-seeded list changed) | Merged viewer list picks up `new-viewers` immediately (Helm-seeded value applies) | With no override, the Helm-seeded layer is free to change on upgrade — the intended seeding behavior. |
| Admin resets to Helm default | Was present, admin calls `DELETE /admin/config/auth/role-mappings/operator` (M7) → key removed from `helmOverride.roleMappings` | `roleMappings.operator: [gameplane-ops]` | `roleMappings.operator: [gameplane-ops]` (unchanged in this example) | `"operator"` role now sourced from the Helm-seeded list again | Explicit dashboard action (M7/M9); audited per Entity 6's `reset` reason template. |

**Cite M9, FR-009, SC-007.**

---

### Role Re-Evaluation on Subsequent Logins

**Condition**: User logs in for the second or later time.

**Behavior** — for the synthetic `"helm"` provider, the row-hash cache does **not** apply and is not
what delivers SC-007: `Registry.OIDCFor` short-circuits on `name == HelmProviderName` and returns
`r.legacy` immediately, before `snapshot()` is called and before any row hash is computed
(`api/internal/auth/registry.go:224-232`). `r.legacy` is a single `*OIDC` built once at process
startup (`api/cmd/main.go:127`) and held for the process lifetime — it is never rebuilt from a
hash-invalidated cache entry. The row-hash cache (`registry.go:172-186`, `rebuildTTL`) governs only
DB-stored (non-Helm) providers.

Instead, SC-007 for the helm provider is delivered by reading `helmOverride` at **login time**: the
helm provider already has the store attached (`legacy.AttachStore(store)`, wired once in
`NewRegistry`, `registry.go:152-153`), so role resolution for the `"helm"` provider reads the
`"auth"` config row's `helmOverride` field on every login attempt, merges it per-role via
`effectiveHelmPolicy` (M3/M4) onto the Helm-seeded base policy, and calls the existing, unmodified
`computeRole` with the merged result. Because this read happens fresh on each login rather than from
a cached `*OIDC`/`*ProviderPolicy`, an admin's edit to `helmOverride` lands on the very next login
with no API restart and no `helm upgrade` — that per-login read is the mechanism, not any caching
layer, and no new caching layer is introduced by this feature.

- **If any role has an override, or a Helm role mapping applies** (`helmOverride.roleMappings` has a
  non-nil list for at least one role, OR `provider.RoleMappings != nil`):
  - Re-evaluate the user's groups against `effectiveHelmPolicy`'s merged policy (M3), then
    `computeRole`, unchanged (M4).
  - Update `users.role` and `user_role_bindings` to the newly computed role.
  - **Guard**: If the new role would remove the last user with user-management permission (admin or operator), skip the update and leave the stored role unchanged. Log a warning.

- **If no override exists for any role and no Helm role mapping applies**
  (`helmOverride` absent/empty AND `provider.RoleMappings == nil`):
  - Leave the user's stored role unchanged. Do not re-evaluate.
  - (This preserves role stability for OIDC-only installs that manually set up roles via `/admin/config` after first login.)

**Cite FR-011, SC-005, SC-007, M3, M4**.

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

**Compatibility Guarantee**: Gameplane installs with OIDC enabled but NO mappings configured
anywhere — neither Helm-seeded (`api.oidc.roleMappings.*` unset) nor DB-persisted (`helmOverride`
absent from the `"auth"` config row) — put every user on the configured default role, which is
`"viewer"` when `defaultRole` is also unset (SC-008). This still holds under the hybrid model: the
overlay (Entity 3a) is additive and only participates in resolution when at least one role key is
present; an absent/empty `helmOverride` is a strict no-op, falling straight through to the
pre-existing Helm/default-role/built-in-viewer chain via `effectiveHelmPolicy` returning the base
policy unchanged.

| Change | Impact | Migration Path | Verification |
|--------|--------|-----------------|--------------|
| New Helm values `api.oidc.groupsClaim`, `api.oidc.roleMappings.*`, `api.oidc.defaultRole` | Optional; if not set, OIDC behaves as before (no role mappings, all users get viewer). Existing installs see no change. | No migration needed; Helm values are optional and default to nil/empty. Existing installs without these values behave identically. | OIDC users logging in without role mappings configured receive viewer role, same as before. Role mappings take effect only if explicitly configured via new Helm values. |
| New API CLI flags `--oidc-groups-claim`, `--oidc-role-mapping-admin`, `--oidc-role-mapping-operator`, `--oidc-role-mapping-viewer`, `--oidc-default-role` | Optional; empty/null values mean no role mappings, identical to pre-feature behavior. | If operators previously manually configured role mappings via `/admin/config`, those configurations persist in the database. The new Helm-based config is a separate, synthetic provider ("helm") and does not interact with database-stored providers. | Existing OIDC users' roles are not retroactively changed; role mappings apply on next login. If a new mapping is configured at install time, it takes effect for all future logins (including existing users on their next login). |
| Database provider synthesis at runtime | New synthetic "helm" provider is listed alongside database-stored providers; it is read-only and cannot be edited/deleted via the dashboard (M2, H4, unchanged by the hybrid model). | No database migration needed; the synthetic provider is constructed at runtime from CLI flags/env vars. | Dashboard `/admin/config/auth` lists the Helm provider as a separate, non-editable entry; operators can still manage other (database-stored) OIDC providers via the dashboard. |
| **`helmOverride` sibling field on the existing `"auth"` config row (Entity 3a, M1/M2) — NO new table, NO new migration** | Additive; a fresh install or an upgrade from a pre-feature version starts with `helmOverride` absent from the row. An absent/empty `helmOverride` has zero effect on resolution — `effectiveHelmPolicy` returns the Helm-seeded policy verbatim, and resolution falls through to the pre-existing Helm/default-role/viewer chain exactly as before this feature existed. | No migration; the field is parsed opportunistically from existing JSON (an old row simply has no `helmOverride` key, which unmarshals to the zero value). No backfill needed. | `effectiveHelmPolicy(base, nil)` returns `base` unchanged; a login against it produces byte-identical results to calling `computeRole(groups, base)` directly, pre-feature. |
| Dashboard editing surface for the overlay (M7) | New UI; does not remove or gate any existing read-only display of Helm-seeded mappings. | No migration; UI-only addition, requires its own `design.pen` pass per constitution Principle II. | Admins with `config:manage` can write/reset role overrides via the extended `PUT /admin/config/auth` and the new `DELETE /admin/config/auth/role-mappings/{role}`; admins without `config:manage` see a read-only view (or no access), matching the existing `/admin/config` RBAC pattern. |

**Cite SC-007, SC-008, M1, M2, M5, M7, M9, Assumption 3, Assumption 5, Assumption 6**.

---

### Bootstrap-Admin Coexistence

**Compatibility Guarantee**: bootstrap-admin and OIDC-mapped users coexist peacefully; both are valid ways to be admin.

**Scenario**: Operator runs `bootstrap-admin` to create a local admin account on a cluster with OIDC role mappings already configured.

| Step | Entity | State | Notes |
|------|--------|-------|-------|
| 1 | OIDC role mappings (Helm) | Configured | `api.oidc.roleMappings` (M10 canonical key path — there is no `auth` level under `api.oidc.*`) set at install time. |
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

- `api/internal/handlers/config_test.go`: `validateAuth` extended for `helmOverride.roleMappings` shape validation; new `DELETE /admin/config/auth/role-mappings/{role}` handler test.
- `api/internal/auth/oidc_rolemap_test.go`: new `TestEffectiveHelmPolicy()` covering per-role replacement (M3) and the M4 worked case; `computeRole`'s existing tests are untouched (M4).
- `test/e2e/api_auth_e2e_test.go`: `TestAPI_OIDCRoleMappingAtInstallTime`, `TestAPI_OIDCRoleMappingFirstLogin`, `TestAPI_OIDCRoleMappingReEvalOnLogin`, `TestAPI_OIDCRoleMappingResetRoute` (api-auth bucket, ~2-3 logins total).
- `test/e2e/gameserver_e2e_test.go`: `TestGameServer_StorageClassFromHelmDefault`, `TestGameServer_StorageClassNotFound`, `TestGameServer_StorageClassExplicitOverride` (operator bucket, zero logins).

**Register new tests** in `test/e2e/buckets.sh` before merge.

---

## Summary

This data model defines:

1. **Storage Class Resolution**: Helm values + operator CLI flags → precedence-ordered PVC materialization across three PVC types (data, extra volumes, mods).
2. **Status Reporting**: Existing `Conditions[]` mechanism used; no new status fields. Error messages rendered on `ServerDetail` UI.
3. **OIDC Role Mapping (Hybrid — Helm seeds, DB overrides, per M0-M10)**: Helm values + CLI flags
   seed the synthetic `"helm"` provider (immutable, read-only at runtime, M2, H4, unchanged). **No
   new table, no new migration (M1)**: a `helmOverride` field on the existing `"auth"` config row
   (Entity 3a) lets admins replace individual roles' group lists through the dashboard's extended
   `PUT /admin/config/auth` (M7); the write takes effect on next login with no restart and no
   `helm upgrade`, because the helm provider path (`OIDCFor` short-circuit, `registry.go:224-232`,
   bypassing the row-hash cache that governs only DB providers) reads `helmOverride` fresh on every
   login attempt. Resolution is two steps: a per-role
   merge (`effectiveHelmPolicy`, Entity 3b, M3/M4) that replaces whole role lists (never a
   per-claim-value lookup), followed by the unchanged `computeRole` — so the most privileged match
   still wins even when only one of two matched roles is overridden (M4's worked case). `groupsClaim`
   and `defaultRole` have no override path in v1 (M5). On `helm upgrade` with changed values, an
   existing per-role override wins over the new Helm value for that role (M9); the new
   `DELETE /admin/config/auth/role-mappings/{role}` route (M7, the only new route) resets one role to
   its Helm-seeded value.
4. **Audit Trail**: OIDC login role assignments (Entity 5) and `helmOverride` writes/resets
   (Entity 6, M8) are both recorded as audit events via the real `WriteSync(ctx, method, path,
   target, reason string, status int)`, gated by the `config:manage` permission for the latter; one
   consistent reason-string format per operation, detail carried entirely in the free-text `reason`
   column (`audit_events` has no action/metadata/message column).
5. **Backward Compatibility**: All changes are optional (empty/nil defaults preserve pre-feature behavior); existing GameServers unaffected by install-time defaults; bootstrap-admin and OIDC coexist; an absent/empty `helmOverride` is a strict no-op, so SC-008 (default viewer with no mappings anywhere) still holds.

All entities resolve in consistent precedence order; see decision tables for role (per-role merge then `computeRole`, M3/M4) and storage class.

