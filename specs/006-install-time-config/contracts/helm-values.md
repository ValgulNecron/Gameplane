# Helm Values Contract: Install-Time Configuration

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

    # NEW: Group-to-role mapping. Defines which IdP groups map to
    # which dashboard roles. If omitted/nil, role mapping is disabled
    # (new OIDC users get "viewer", existing roles never re-evaluated).
    roleMappings:
      # Array of IdP groups that grant "admin" dashboard role.
      admin: []
      # Example: ["gameplane-admins", "ops-team"]

      # Array of IdP groups that grant "operator" dashboard role.
      operator: []
      # Example: ["gameplane-operators"]

      # Array of IdP groups that grant "viewer" dashboard role.
      viewer: []
      # Example: ["gameplane-viewers", "readonly-users"]

    # NEW: Default role when no group matches.
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
- `--oidc-default-role` **(NEW)**
- `--oidc-role-mapping-admin`, `--oidc-role-mapping-operator`, `--oidc-role-mapping-viewer` **(NEW, repeatable)**

Template snippet (conditional inclusion):
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
  {{- range .Values.api.oidc.roleMappings.admin }}
  - --oidc-role-mapping-admin={{ . }}
  {{- end }}
  {{- range .Values.api.oidc.roleMappings.operator }}
  - --oidc-role-mapping-operator={{ . }}
  {{- end }}
  {{- range .Values.api.oidc.roleMappings.viewer }}
  - --oidc-role-mapping-viewer={{ . }}
  {{- end }}
  {{- end }}
```

---

## 3. Mapping Table: Values → Requirements

| Helm Key | Type | Default | Consumer | Requirement |
|----------|------|---------|----------|-------------|
| `operator.gameDataStorage.storageClassName` | string | `""` | operator binary (flag `--game-data-storage-class`) + api binary (flag `--game-data-storage-class`, report-only) | FR-006 |
| `api.oidc.groupsClaim` | string | `""` | api binary (flag `--oidc-groups-claim`) | FR-005, FR-006 |
| `api.oidc.roleMappings.admin` | array[string] | `[]` | api binary (flags `--oidc-role-mapping-admin`) + audit event emission on role assignment | FR-007, FR-014 |
| `api.oidc.roleMappings.operator` | array[string] | `[]` | api binary (flags `--oidc-role-mapping-operator`) + audit event emission on role assignment | FR-007, FR-014 |
| `api.oidc.roleMappings.viewer` | array[string] | `[]` | api binary (flags `--oidc-role-mapping-viewer`) + audit event emission on role assignment | FR-007, FR-014 |
| `api.oidc.defaultRole` | string | `""` | api binary (flag `--oidc-default-role`) | FR-007 |

---

## 4. Schema File Status

**File**: `charts/gameplane/values.schema.json`

**Status**: Does not exist; no JSON Schema validation is currently in place for Helm values.

**Action**: No schema update required for this feature. If a schema is added in the future, the keys defined above should follow the existing conventions (required vs optional, type hints, default values).

---

## 5. Backward Compatibility

- **Empty `gameDataStorage` block**: Equivalent to `storageClassName: ""` — uses cluster default. Existing clusters unaffected.
- **Omitted `api.oidc.groupsClaim` and `roleMappings`**: Equivalent to no role mapping. Existing Helm OIDC setups continue to work; new OIDC users default to "viewer".
- **No breaking changes**: All new keys default to empty/falsy values, preserving existing behavior.
