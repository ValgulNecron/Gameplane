# API CLI Flags Contract: Install-Time Configuration

## 1. API Serve Subcommand: OIDC Role-Mapping Flags

**Entry Point**: `api/cmd/main.go`, `serve` subcommand

**New Flags** (install-time OIDC configuration, used only when `--oidc-issuer` is set):

### Flag: `--oidc-groups-claim`

**Type**: `string`

**Default**: `""` (empty; defaults to `"groups"`)

**Syntax**:
```bash
--oidc-groups-claim="groups"
--oidc-groups-claim="roles"
--oidc-groups-claim="membership"
```

**Semantics**: Name of the OIDC ID token claim containing group/role memberships. If empty, defaults to `"groups"`.

**Example**:
```bash
./api serve \
  --oidc-issuer="https://idp.example.com" \
  --oidc-client-id="..." \
  --oidc-groups-claim="organization_roles"
```

**Requirement**: FR-005, FR-006

---

### Flag: `--oidc-default-role`

**Type**: `string`

**Default**: `""` (empty; interpreted as `"viewer"`)

**Syntax**:
```bash
--oidc-default-role="viewer"
--oidc-default-role="operator"
--oidc-default-role="admin"
--oidc-default-role="deny"
```

**Valid Values**: `""`, `"viewer"`, `"operator"`, `"admin"`, `"deny"`

**Semantics**: 
- Role assigned to new OIDC users when their group membership does not match any mapping.
- `""` or `"viewer"`: Grant viewer role (read-only).
- `"operator"`: Grant operator role (start/stop servers, manage backups).
- `"admin"`: Grant admin role (manage configuration, users).
- `"deny"`: Reject the login (do not create user; return 403 Forbidden).

**Only meaningful if `--oidc-role-mapping-*` is set** (i.e., role mapping is enabled).

**Example**:
```bash
./api serve \
  --oidc-issuer="https://idp.example.com" \
  --oidc-client-id="..." \
  --oidc-role-mapping-admin="idp-admins" \
  --oidc-default-role="viewer"
```

**Requirement**: FR-007

---

### Flag: `--oidc-role-mapping-admin` (repeatable)

**Type**: `string` (repeatable)

**Default**: None (empty list)

**Syntax** (repeatable; one group per flag):
```bash
--oidc-role-mapping-admin="idp-admins"
--oidc-role-mapping-admin="ops-team"
--oidc-role-mapping-admin="admins"
```

**Semantics**: IdP group names that grant the "admin" dashboard role. Flag may be repeated to specify multiple groups.

**Example**:
```bash
./api serve \
  --oidc-issuer="https://idp.example.com" \
  --oidc-client-id="..." \
  --oidc-groups-claim="groups" \
  --oidc-role-mapping-admin="gameplane-admins" \
  --oidc-role-mapping-admin="devops-team" \
  --oidc-role-mapping-operator="gameplane-operators" \
  --oidc-role-mapping-viewer="gameplane-viewers"
```

**Requirement**: FR-007

---

### Flag: `--oidc-role-mapping-operator` (repeatable)

**Type**: `string` (repeatable)

**Default**: None (empty list)

**Syntax** (repeatable):
```bash
--oidc-role-mapping-operator="idp-operators"
--oidc-role-mapping-operator="ops-support"
```

**Semantics**: IdP group names that grant the "operator" dashboard role. Flag may be repeated.

**Requirement**: FR-007

---

### Flag: `--oidc-role-mapping-viewer` (repeatable)

**Type**: `string` (repeatable)

**Default**: None (empty list)

**Syntax** (repeatable):
```bash
--oidc-role-mapping-viewer="idp-viewers"
--oidc-role-mapping-viewer="readonly-users"
```

**Semantics**: IdP group names that grant the "viewer" dashboard role. Flag may be repeated.

**Requirement**: FR-007

---

## 2. API Serve Subcommand: Game Data Storage Class Flag

**Entry Point**: `api/cmd/main.go`, `serve` subcommand

**New Flag** (install-time configuration passthrough, for `GET /admin/config` reporting only):

### Flag: `--game-data-storage-class`

**Type**: `string`

**Default**: `""` (empty)

**Syntax**:
```bash
--game-data-storage-class="fast-nvme"
--game-data-storage-class="standard"
```

**Semantics**: StorageClass name passed from the Helm value `operator.gameDataStorage.storageClassName` and used by the operator as the install-time default for game server data volumes. The API stores this value report-only and returns it in `GET /admin/config` under `installTimeSettings.gameDataStorageClass`. The API itself never uses this value; it is purely for dashboard transparency.

**Example**:
```bash
./api serve \
  --game-data-storage-class="fast-nvme"
```

**Requirement**: FR-006

**Note**: This flag is passed from the same Helm value (`operator.gameDataStorage.storageClassName`) used by the operator, ensuring both control plane components report the same configured storage class.

---

## 3. OIDC Flag Registration Pattern

**Code Pattern** (in `api/cmd/main.go`, alongside existing `--oidc-*` flags):

```go
var (
    // Existing flags
    oidcIssuer       = flag.String("oidc-issuer", os.Getenv("GAMEPLANE_OIDC_ISSUER"), "...")
    oidcClientID     = flag.String("oidc-client-id", os.Getenv("GAMEPLANE_OIDC_CLIENT_ID"), "...")
    oidcRedirectURL  = flag.String("oidc-redirect-url", os.Getenv("GAMEPLANE_OIDC_REDIRECT_URL"), "...")
    oidcDisplayName  = flag.String("oidc-display-name", os.Getenv("GAMEPLANE_OIDC_DISPLAY_NAME"), "...")

    // NEW flags for role mapping
    oidcGroupsClaim      = flag.String("oidc-groups-claim", os.Getenv("GAMEPLANE_OIDC_GROUPS_CLAIM"), 
        "OIDC claim name containing group memberships; empty defaults to \"groups\"")
    oidcDefaultRole      = flag.String("oidc-default-role", os.Getenv("GAMEPLANE_OIDC_DEFAULT_ROLE"), 
        "Default role for new OIDC users when no group matches: \"viewer\" (default), \"operator\", \"admin\", or \"deny\"")
)

// NEW for repeatable flags (arrays)
var oidcRoleMappingAdmin    arrayFlag  // custom flag type for repeatable
var oidcRoleMappingOperator arrayFlag
var oidcRoleMappingViewer   arrayFlag

func init() {
    flag.Var(&oidcRoleMappingAdmin, "oidc-role-mapping-admin",
        "IdP group(s) granting admin role (repeatable)")
    flag.Var(&oidcRoleMappingOperator, "oidc-role-mapping-operator",
        "IdP group(s) granting operator role (repeatable)")
    flag.Var(&oidcRoleMappingViewer, "oidc-role-mapping-viewer",
        "IdP group(s) granting viewer role (repeatable)")
}
```

**Flag Parsing** (in `api/cmd/main.go` `serve` subcommand):
```go
flag.Parse()

// Build role mappings struct
roleMappings := &auth.RoleMappings{
    Admin:    oidcRoleMappingAdmin.values,
    Operator: oidcRoleMappingOperator.values,
    Viewer:   oidcRoleMappingViewer.values,
}
```

---

## 4. Helm Template Integration

**File**: `charts/gameplane/templates/api.yaml` (Deployment spec)

**Template Snippet** (in `spec.containers[].args`):

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

**Behavior**:
- If `api.oidc.issuer` is empty/nil, ALL OIDC flags are omitted (no Helm OIDC setup).
- If `issuer` is set but `groupsClaim` is empty, the `--oidc-groups-claim` flag is omitted (uses CLI default "groups").
- If `issuer` is set but `roleMappings.admin` is empty, the admin loop produces no flags (empty array).
- Repeating the loop over `roleMappings.admin|operator|viewer` generates one `--oidc-role-mapping-*` flag per group.

---

## 5. Environment Variables (Optional Fallback)

**Convention** (optional, for development or container-to-CLI integration):

| Environment Variable | Flag | Example |
|----------------------|------|---------|
| `GAMEPLANE_OIDC_GROUPS_CLAIM` | `--oidc-groups-claim` | `groups` or `roles` |
| `GAMEPLANE_OIDC_DEFAULT_ROLE` | `--oidc-default-role` | `viewer`, `admin`, `deny` |
| `GAMEPLANE_OIDC_ROLE_MAPPING_ADMIN` | `--oidc-role-mapping-admin` | (repeatable) |
| `GAMEPLANE_OIDC_ROLE_MAPPING_OPERATOR` | `--oidc-role-mapping-operator` | (repeatable) |
| `GAMEPLANE_OIDC_ROLE_MAPPING_VIEWER` | `--oidc-role-mapping-viewer` | (repeatable) |

**Usage** (example with env vars):
```bash
export GAMEPLANE_OIDC_ISSUER="https://idp.example.com"
export GAMEPLANE_OIDC_CLIENT_ID="client-123"
export GAMEPLANE_OIDC_GROUPS_CLAIM="roles"
export GAMEPLANE_OIDC_DEFAULT_ROLE="viewer"
export GAMEPLANE_OIDC_ROLE_MAPPING_ADMIN="idp-admins"
export GAMEPLANE_OIDC_ROLE_MAPPING_OPERATOR="idp-ops"

./api serve
```

**Note**: Helm-managed deployments use CLI flags directly (via Deployment `args:`); env vars are optional for manual testing.

---

## 6. List-Valued Mapping Syntax (Helm to CLI)

### Array Input (Helm values.yaml)
```yaml
api:
  oidc:
    roleMappings:
      admin:
        - "gameplane-admins"
        - "devops-team"
      operator:
        - "gameplane-operators"
      viewer:
        - "gameplane-viewers"
        - "readonly-users"
```

### Generated CLI Flags (Helm template output)
```bash
./api serve \
  --oidc-issuer="https://idp.example.com" \
  --oidc-client-id="..." \
  --oidc-groups-claim="groups" \
  --oidc-role-mapping-admin="gameplane-admins" \
  --oidc-role-mapping-admin="devops-team" \
  --oidc-role-mapping-operator="gameplane-operators" \
  --oidc-role-mapping-viewer="gameplane-viewers" \
  --oidc-role-mapping-viewer="readonly-users"
```

### Direct CLI Invocation
```bash
./api serve \
  --oidc-issuer="https://idp.example.com" \
  --oidc-client-id="..." \
  --oidc-groups-claim="groups" \
  --oidc-role-mapping-admin="gameplane-admins" \
  --oidc-role-mapping-admin="devops-team" \
  --oidc-role-mapping-operator="gameplane-operators" \
  --oidc-role-mapping-viewer="gameplane-viewers"
```

---

## 7. Error Handling & Validation

**Validation** (performed at API startup):

1. **Invalid `--oidc-default-role`**: Must be one of `""`, `"viewer"`, `"operator"`, `"admin"`, `"deny"`
   - Error: Fatal exit with message `"invalid OIDC default role: must be one of '', 'viewer', 'operator', 'admin', 'deny'"`

2. **Empty group in `--oidc-role-mapping-admin|operator|viewer`**: Spaces-only or empty string rejected
   - Error: Warning logged; group skipped or fatal depending on policy

3. **No flags set** (issuer but no mappings/groupsClaim): Warning logged; OIDC works but all users get default role

---

## 8. Backward Compatibility

- **Existing installs without role mappings**: All new flags are optional and default to empty. Behavior unchanged (new OIDC users default to "viewer", roles never re-evaluated).
- **Existing CLI invocations**: No required flags added; opt-in only.
- **Helm values**: All new keys optional; empty/omitted values preserve legacy behavior.

---

## 9. Testing & Verification

**Manual Testing** (development):
```bash
# Direct CLI with role mappings
./api serve \
  --oidc-issuer="https://idp.example.com" \
  --oidc-client-id="test-client" \
  --oidc-groups-claim="org_roles" \
  --oidc-role-mapping-admin="admins" \
  --oidc-role-mapping-operator="ops" \
  --oidc-role-mapping-viewer="users" \
  --oidc-default-role="deny"
```

**Helm Testing** (chart):
```bash
helm install gameplane ./charts/gameplane \
  --set api.oidc.issuer="https://idp.example.com" \
  --set api.oidc.clientID="test" \
  --set api.oidc.groupsClaim="groups" \
  --set 'api.oidc.roleMappings.admin={idp-admins,devops}' \
  --set 'api.oidc.roleMappings.operator={idp-ops}'
```

**Verification** (in cluster):
```bash
kubectl logs -l app=gameplane,component=api | grep "OIDC"
# Should show OIDC config loaded with group mappings
```
