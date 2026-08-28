# Helm Values Contract: Install-Time Configuration

## 0. Maintainer Decision: `api.oidc.roleMappings.*` / `defaultRole` Are Seeds, Not the Sole Source of Truth

**Binding hybrid decision** (full rationale in `api-http.md` §0; CLI-level detail in
`api-cli.md` §0): `api.oidc.roleMappings.{admin,operator,viewer}` and `api.oidc.defaultRole` — the
keys defined in §1 below, unchanged in name and shape — configure the **install-time seed** for
the synthetic `helm` OIDC provider's role mapping. They satisfy FR-007 / SC-003 / SC-004 (an
OIDC-only install works with no admin account and no `bootstrap-admin` run) exactly as before.

What's new is that they are no longer the *only* source: admins can layer a per-role DB override
on top through the dashboard, carried as the `helmOverride.roleMappings` field of the *existing*
`PUT /admin/config/auth` body (`api-http.md` §2.1) and reset via the one new route
`DELETE /admin/config/auth/role-mappings/{role}` (`api-http.md` §2.2) — which satisfies SC-007
(manage mappings through the dashboard, no `helm upgrade`). Resolution precedence, per role,
independently: **DB override (admin-managed), if present for that role > this Helm-seeded value
for that role**. `defaultRole` itself has no DB override path (M5, Helm-only in v1).

**Upgrade semantics (M9)** — the reason this is a "seed" and not authoritative on every
`helm upgrade`: changing one of these values and running `helm upgrade` updates the seed
immediately for any role with **no** DB override, but does **not** clobber a role an admin has
since overridden through the dashboard — that DB override keeps winning until the admin explicitly
resets it (dashboard-only action; no Helm-side way to force it). This is deliberate: the
alternative (Helm reasserting on every upgrade) would silently undo admin edits made through the
UI SC-007 exists to support.

**No key renames, no new Helm keys, no schema changes** result from this decision — everything in
§1–§5 below is unchanged in name and type from a plain "Helm configures OIDC role mapping" design;
only the runtime precedence changes, entirely on the API side (§2's Helm template rendering below
uses a comma-joined single flag per role, not one flag per group — see `api-cli.md` §0/§4 for why).

---

## 1. New & Modified Keys

### Operator Section: Game Data Storage Class

**Location**: `operator.gameDataStorage` (NEW top-level nested structure)

**YAML (commented):**

```yaml
operator:
  # ... existing fields (leaderElect, logLevel, addressManager, etc.) ...

  # NEW: Install-time default for game server data volume storage class.
  # Applies to all GameServer.spec.storage when neither GameServer nor
  # GameTemplate explicitly override it.
  gameDataStorage:
    # empty string (default) = use cluster's default StorageClass
    # "fast-nvme", "gpu-attached", etc. = use named StorageClass
    # If the named class doesn't exist, PVC provisioning fails and GameServer
    # status.conditions surfaces "StorageClassNotFound" reason.
    storageClassName: ""
```

**Placement in values.yaml**: After `operator.localModules.mountPath` (around line 100), before the `api:` section (line 102).

**Type**: `string` (nullable; empty string is valid)

**Default**: `""` (empty = cluster default)

---

### API Section: OIDC Configuration (CLI Flags → Runtime Config)

**Location**: `api.oidc` (MODIFIED; existing structure extended)

**Existing fields** (from research, cli flags):
- `issuer` (string): OIDC issuer URL (from `--oidc-issuer`)
- `clientID` (string): OAuth client ID (from `--oidc-client-id`)
- `clientSecretRef` (object): OAuth client secret reference (from `--oidc-client-secret-ref`)
- `redirectURL` (string): OAuth redirect URL (from `--oidc-redirect-url`)
- `displayName` (string): Label shown on login UI (from `--oidc-display-name`)

**NEW fields** (install-time defaults for role mapping):
```yaml
api:
  oidc:
    # ... existing: issuer, clientID, clientSecretRef, redirectURL, displayName ...

    # NEW: OIDC claim name containing group/role memberships.
    # Default: "groups" if empty or omitted.
    # Examples: "groups", "roles", "membership", "department"
    groupsClaim: ""

    # NEW: Group-to-role mapping, SEEDED at install/upgrade time (§0 — hybrid
    # decision). Defines which IdP groups map to which dashboard roles for the
    # synthetic "helm" provider. If omitted/nil, role mapping is disabled
    # (new OIDC users get "viewer", existing roles never re-evaluated) unless
    # an admin has set a DB override for a role via the dashboard, which wins
    # regardless of what's seeded here (§0).
    roleMappings:
      # Array of IdP groups that seed the "admin" dashboard role.
      # A DB override (dashboard-managed, written via PUT /admin/config/auth's
      # helmOverride.roleMappings.admin field) takes precedence over this
      # value once set, and survives future `helm upgrade`s that change this
      # array — see §0.
      admin: []
      # Example: ["gameplane-admins", "ops-team"]

      # Array of IdP groups that seed the "operator" dashboard role.
      # Same DB-override precedence as "admin" above.
      operator: []
      # Example: ["gameplane-operators"]

      # Array of IdP groups that seed the "viewer" dashboard role.
      # Same DB-override precedence as "admin" above.
      viewer: []
      # Example: ["gameplane-viewers", "readonly-users"]

    # NEW: Default role when no group matches — Helm-only, no DB override
    # path exists or is planned for this field (M5).
    # Accepted values: "" (default to "viewer"), "viewer", "operator", "admin", "deny"
    # "deny" = reject login (no user created/updated).
    # Only meaningful if roleMappings is set.
    defaultRole: ""
```

**Placement in values.yaml**: Within `api.oidc` section (extends existing structure).

**Type**: All strings or arrays of strings.

**Default**: All empty/omitted (backward compatible; no mapping = legacy behavior).

**Validation** (enforced at Helm install time via chart validation or at startup):
- `groupsClaim`: If set, must not be blank (no spaces-only).
- `defaultRole`: Must be one of `""`, `"viewer"`, `"operator"`, `"admin"`, `"deny"`.
- `roleMappings.admin|operator|viewer`: Arrays must not contain empty strings.
- `roleMappings` and `defaultRole` require logical coherence: `defaultRole` only meaningful if `roleMappings` is non-nil.

---

## 2. Helm Template Integration

### Operator Deployment Args Snippet

**File**: `charts/gameplane/templates/operator.yaml` (around line 295-297)

**Pattern** (matching existing conditional flags):

```yaml
args:
  {{- if .Values.operator.leaderElect }}
  - --leader-elect
  {{- end }}
  - --agent-image={{ include "gameplane.agentImage" . }}
  - --zap-log-level={{ .Values.operator.logLevel }}
  # ... other existing flags ...

  {{- if .Values.operator.gameDataStorage.storageClassName }}
  - --game-data-storage-class={{ .Values.operator.gameDataStorage.storageClassName }}
  {{- end }}
```

**Behavior**:
- If `storageClassName` is empty or omitted: the `--game-data-storage-class` flag is NOT passed to the operator.
- If `storageClassName` is set (non-empty string): the flag is passed with that value.

---

### API Deployment: Game Data Storage Class Flag

**File**: `charts/gameplane/templates/api.yaml` (within `spec.containers[].args`, after OIDC flags)

**Pattern** (matching existing flag style):

```yaml
args:
  - serve
  # ... existing flags (db-driver, db-dsn, oidc-*, agent-*, etc.) ...
  
  {{- if .Values.operator.gameDataStorage.storageClassName }}
  - --game-data-storage-class={{ .Values.operator.gameDataStorage.storageClassName }}
  {{- end }}
```

**Behavior**:
- If `operator.gameDataStorage.storageClassName` is empty or omitted: the `--game-data-storage-class` flag is NOT passed to the API.
- If `storageClassName` is set (non-empty string): the flag is passed with that value, purely informational for `GET /admin/config`.

**Note**: The API flag uses the SAME Helm value key as the operator flag (`operator.gameDataStorage.storageClassName`), ensuring both deployments reference the same configured storage class.

---

### API Deployment: OIDC Role-Mapping Flags

**File**: `charts/gameplane/templates/api.yaml` (within `spec.containers[].args`, after OIDC issuer block)

**CLI Flags** (received by `api serve`):
- `--oidc-issuer`
- `--oidc-client-id`
- `--oidc-client-secret-ref`
- `--oidc-redirect-url`
- `--oidc-display-name`
- `--oidc-groups-claim` **(NEW)**
- `--oidc-default-role` **(NEW, Helm-only, no DB override — M5)**
- `--oidc-role-mapping-admin`, `--oidc-role-mapping-operator`, `--oidc-role-mapping-viewer`
  **(NEW — single flag per role, comma-separated value; NOT repeatable, see `api-cli.md` §0/§3/§4)**

Template snippet (conditional inclusion — each mapping array renders as **one** flag via `join`,
never a `range` loop emitting one flag per group):
```yaml
args:
  - serve
  {{- if .Values.api.oidc.issuer }}
  - --oidc-issuer={{ .Values.api.oidc.issuer }}
  - --oidc-client-id={{ .Values.api.oidc.clientID }}
  - --oidc-client-secret-ref={{ .Values.api.oidc.clientSecretRef }}
  - --oidc-redirect-url={{ .Values.api.oidc.redirectURL }}
  - --oidc-display-name={{ .Values.api.oidc.displayName }}
  {{- if .Values.api.oidc.groupsClaim }}
  - --oidc-groups-claim={{ .Values.api.oidc.groupsClaim }}
  {{- end }}
  {{- if .Values.api.oidc.defaultRole }}
  - --oidc-default-role={{ .Values.api.oidc.defaultRole }}
  {{- end }}
  {{- if .Values.api.oidc.roleMappings.admin }}
  - --oidc-role-mapping-admin={{ join "," .Values.api.oidc.roleMappings.admin }}
  {{- end }}
  {{- if .Values.api.oidc.roleMappings.operator }}
  - --oidc-role-mapping-operator={{ join "," .Values.api.oidc.roleMappings.operator }}
  {{- end }}
  {{- if .Values.api.oidc.roleMappings.viewer }}
  - --oidc-role-mapping-viewer={{ join "," .Values.api.oidc.roleMappings.viewer }}
  {{- end }}
  {{- end }}
```

---

## 3. Mapping Table: Values → Requirements

| Helm Key | Type | Default | Consumer | Requirement |
|----------|------|---------|----------|-------------|
| `operator.gameDataStorage.storageClassName` | string | `""` | operator binary (flag `--game-data-storage-class`) + api binary (flag `--game-data-storage-class`, report-only) | FR-006 |
| `api.oidc.groupsClaim` | string | `""` | api binary (flag `--oidc-groups-claim`) | FR-005, FR-006 |
| `api.oidc.roleMappings.admin` | array[string] | `[]` | api binary (flag `--oidc-role-mapping-admin`, comma-joined); SEEDS the "admin" role for the `helm` provider — a DB override set via `PUT /admin/config/auth`'s `helmOverride.roleMappings.admin` (`api-http.md` §2.1), reset via `DELETE /admin/config/auth/role-mappings/admin` (`api-http.md` §2.2), wins over this on every future `helm upgrade` (M9) + audit event emission on role assignment | FR-007, FR-014, SC-007 |
| `api.oidc.roleMappings.operator` | array[string] | `[]` | api binary (flag `--oidc-role-mapping-operator`, comma-joined); SEEDS the "operator" role, same DB-override precedence as `roleMappings.admin` + audit event emission on role assignment | FR-007, FR-014, SC-007 |
| `api.oidc.roleMappings.viewer` | array[string] | `[]` | api binary (flag `--oidc-role-mapping-viewer`, comma-joined); SEEDS the "viewer" role, same DB-override precedence as `roleMappings.admin` + audit event emission on role assignment | FR-007, FR-014, SC-007 |
| `api.oidc.defaultRole` | string | `""` | api binary (flag `--oidc-default-role`); SEEDS the default role. **Helm-only in v1 (M5) — no DB override, no PUT/DELETE endpoint accepts this field.** | FR-007, SC-007 |

---

## 4. Schema File Status

**File**: `charts/gameplane/values.schema.json`

**Status**: Does not exist; no JSON Schema validation is currently in place for Helm values.

**Action**: No schema update required for this feature. If a schema is added in the future, the keys defined above should follow the existing conventions (required vs optional, type hints, default values).

---

## 5. Backward Compatibility

- **Empty `gameDataStorage` block**: Equivalent to `storageClassName: ""` — uses cluster default. Existing clusters unaffected.
- **Omitted `api.oidc.groupsClaim` and `roleMappings`**: Equivalent to no role mapping seed. Existing Helm OIDC setups continue to work; new OIDC users default to "viewer" unless a DB override exists.
- **No breaking changes**: All new keys default to empty/falsy values, preserving existing behavior.
- **`helm upgrade` changing `roleMappings.*` on a role an admin has since overridden through the
  dashboard (M9)**: safe — the new seed value is parsed and held, but does not take effect for that
  role until the admin resets the override (the role's key simply stays present in
  `helmOverride.roleMappings`, per `api-http.md` §1's provenance note — there is no separate
  `source` field). No values-only way exists to force it back in; see §0. `defaultRole` has no
  override at all (M5), so a `helm upgrade` changing it always takes effect immediately.
