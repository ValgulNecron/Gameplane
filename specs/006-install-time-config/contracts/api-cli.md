# API CLI Flags Contract: Install-Time Configuration

> **This file was rewritten to match the settled mechanism** (see `api-http.md` §0 and
> `data-model.md` Entities 3, 3a–4b for the full rationale). Every `--oidc-role-mapping-*` flag
> below is a **single comma-separated `string` flag** (`flag.StringVar` + `envOr`), matching the
> verified pattern every other `--oidc-*` flag already uses at `api/cmd/main.go:391-395`. A previous
> draft rendered these as repeatable flags (`flag.Var`/`arrayFlag`) — repeatable flags have no
> env-var equivalent and would break that convention; that rendering is wrong and does not appear
> below. `--oidc-default-role` has **no DB override counterpart** — it stays Helm-only in v1 (M5);
> any earlier text suggesting a `PUT .../default-role` endpoint has been removed.

## 0. Maintainer Decision: These Flags Are Seeds, Not the Sole Source of Truth

**Binding hybrid decision** (full rationale in `api-http.md` §0): the flags below configure the
**install-time seed** for the synthetic `helm` OIDC provider's role mapping
(`auth.HelmProviderName`, `api/internal/auth/registry.go:30`). They are no longer the only place a
role mapping can come from — admins can layer a per-role DB override on top through
`PUT /admin/config/auth`'s `helmOverride` field (`api-http.md` §2.1), reset via
`DELETE /admin/config/auth/role-mappings/{role}` (`api-http.md` §2.2), and that override wins at
login-resolution time (`computeRole`, called with the merged policy produced by the new
`effectiveHelmPolicy` helper, `api/internal/auth/oidc.go:121`).

**Flag names, defaults, and semantics below are otherwise unchanged from a plain "Helm configures
OIDC role mapping" design** — the hybrid model doesn't touch the CLI surface itself, only what
happens to the values downstream. What changes is precedence and, critically, **upgrade behavior**:

- **First install** (fresh DB, no override rows): the flag values below are the *only* input —
  role resolution runs exactly on the Helm-seeded policy. This is what satisfies FR-007 / SC-004 for
  an OIDC-only install with no admin account and no `bootstrap-admin` run.
- **`helm upgrade` with changed `--oidc-role-mapping-*` values, on a role an admin has since
  overridden via the dashboard**: the new flag value is parsed and held (it's still what `GET
  /admin/config`'s `helmOverride` reasoning implicitly falls back to for a role with **no**
  override), but it does **not** take effect for an overridden role's login resolution — the DB
  override still wins (M9). This is deliberate: reasserting the Helm value on every upgrade would
  silently undo an admin's dashboard edit. There is no flag-level way to force the Helm value back
  in; that's the dashboard's explicit reset action (`DELETE /admin/config/auth/role-mappings/{role}`,
  `api-http.md` §2.2).
- **`helm upgrade` on a role with no override**: the new flag value takes effect immediately on the
  next login for that role, same as before.

Nothing in this file's flag registration, parsing, or Helm-template-integration sections below needs
to change to support this — the hybrid logic lives entirely in `effectiveHelmPolicy` +
(unmodified) `computeRole`, and in the new `PUT`/`DELETE` handling in `api/internal/handlers/config.go`,
which read these seeded values from the in-memory `auth.RoleMappings`/`auth.ProviderPolicy` the
flags already populate (§3 below).

---

## 1. API Serve Subcommand: OIDC Role-Mapping Flags

**Entry Point**: `api/cmd/main.go`, `serve` subcommand (flags registered in `bindFlags`, alongside
the existing `--oidc-issuer`, `--oidc-client-id`, `--oidc-client-secret`, `--oidc-redirect-url`,
`--oidc-display-name` flags at `api/cmd/main.go:391-395`)

**New Flags** (install-time OIDC configuration, meaningful only when `--oidc-issuer` is set):

### Flag: `--oidc-groups-claim`

**Type**: `string`

**Default**: `""` (empty; defaults to `"groups"` at resolution time)

**Syntax**:
```bash
--oidc-groups-claim="groups"
--oidc-groups-claim="roles"
--oidc-groups-claim="membership"
```

**Semantics**: Name of the OIDC ID token claim containing group/role memberships. If empty, defaults
to `"groups"`. **Helm-only (M5)** — no DB override exists or is planned for this field.

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
- Sets the default role assigned to OIDC users when their group membership does not match any
  mapping. **Helm-only in v1 (M5) — not admin-overridable.** There is no `PUT
  /admin/config/.../default-role` endpoint and no `helmOverride` field for this value; any prior
  draft suggesting one is wrong.
- `""` or `"viewer"`: Grant viewer role (read-only).
- `"operator"`: Grant operator role (start/stop servers, manage backups).
- `"admin"`: Grant admin role (manage configuration, users).
- `"deny"`: Reject the login (do not create user; return 403 Forbidden).

**Only meaningful if at least one `--oidc-role-mapping-*` flag is set** (i.e., role mapping is
enabled).

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

### Flag: `--oidc-role-mapping-admin`

**Type**: `string` — **single flag, comma-separated value** (not repeatable)

**Default**: `""` (empty; no groups)

**Syntax**:
```bash
--oidc-role-mapping-admin="idp-admins"
--oidc-role-mapping-admin="idp-admins,ops-team,admins"
```

**Semantics**: Comma-separated IdP group names that **seed** the "admin" dashboard role mapping
(§0) for the `helm` provider. Values are trimmed of surrounding whitespace after splitting; empty
elements (e.g. from a trailing comma) are dropped. An admin-managed DB override for the "admin"
role, once set via the dashboard, takes precedence over this flag's value until explicitly reset
(§0, `api-http.md` §2.2).

**Example**:
```bash
./api serve \
  --oidc-issuer="https://idp.example.com" \
  --oidc-client-id="..." \
  --oidc-groups-claim="groups" \
  --oidc-role-mapping-admin="gameplane-admins,devops-team" \
  --oidc-role-mapping-operator="gameplane-operators" \
  --oidc-role-mapping-viewer="gameplane-viewers"
```

**Requirement**: FR-007

---

### Flag: `--oidc-role-mapping-operator`

**Type**: `string` — single flag, comma-separated value

**Default**: `""` (empty; no groups)

**Syntax**:
```bash
--oidc-role-mapping-operator="idp-operators"
--oidc-role-mapping-operator="idp-operators,ops-support"
```

**Semantics**: Comma-separated IdP group names that **seed** the "operator" dashboard role mapping
(§0). Same parsing and DB-override precedence as `--oidc-role-mapping-admin` above.

**Requirement**: FR-007

---

### Flag: `--oidc-role-mapping-viewer`

**Type**: `string` — single flag, comma-separated value

**Default**: `""` (empty; no groups)

**Syntax**:
```bash
--oidc-role-mapping-viewer="idp-viewers"
--oidc-role-mapping-viewer="idp-viewers,readonly-users"
```

**Semantics**: Comma-separated IdP group names that **seed** the "viewer" dashboard role mapping
(§0). Same parsing and DB-override precedence as `--oidc-role-mapping-admin` above.

**Requirement**: FR-007

---

## 2. API Serve Subcommand: Game Data Storage Class Flag

**Entry Point**: `api/cmd/main.go`, `serve` subcommand

**New Flag** (install-time configuration passthrough, for `GET /admin/config` reporting only —
unrelated to the OIDC role-mapping hybrid above):

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

**Verified existing pattern** (`api/cmd/main.go:391-395` — every `--oidc-*` flag is a plain
`fs.StringVar` reading an `envOr`-wrapped environment variable, registered inside `(c
*config).bindFlags(fs *flag.FlagSet)`):

```go
fs.StringVar(&c.oidcIssuer, "oidc-issuer", envOr("GAMEPLANE_OIDC_ISSUER", ""), "OIDC issuer URL")
fs.StringVar(&c.oidcClientID, "oidc-client-id", envOr("GAMEPLANE_OIDC_CLIENT_ID", ""), "OIDC client id")
fs.StringVar(&c.oidcClientSecret, "oidc-client-secret", envOr("GAMEPLANE_OIDC_CLIENT_SECRET", ""), "OIDC client secret")
fs.StringVar(&c.oidcRedirectURL, "oidc-redirect-url", envOr("GAMEPLANE_OIDC_REDIRECT_URL", ""), "OIDC redirect URL")
fs.StringVar(&c.oidcDisplayName, "oidc-display-name", envOr("GAMEPLANE_OIDC_DISPLAY_NAME", "Single sign-on"), "label for the OIDC login button (no hostname — shown pre-auth)")
```

**New flags follow the exact same shape** — plain `fs.StringVar`, no custom `flag.Var` type, no
repeatable flag:

```go
fs.StringVar(&c.oidcGroupsClaim, "oidc-groups-claim", envOr("GAMEPLANE_OIDC_GROUPS_CLAIM", ""),
	"OIDC claim name containing group memberships; empty defaults to \"groups\"")
fs.StringVar(&c.oidcDefaultRole, "oidc-default-role", envOr("GAMEPLANE_OIDC_DEFAULT_ROLE", ""),
	"Default role for new OIDC users when no group matches: \"viewer\" (default), \"operator\", \"admin\", or \"deny\". Helm-only — not DB-overridable.")
fs.StringVar(&c.oidcRoleMappingAdmin, "oidc-role-mapping-admin", envOr("GAMEPLANE_OIDC_ROLE_MAPPING_ADMIN", ""),
	"Comma-separated IdP group(s) seeding the admin role")
fs.StringVar(&c.oidcRoleMappingOperator, "oidc-role-mapping-operator", envOr("GAMEPLANE_OIDC_ROLE_MAPPING_OPERATOR", ""),
	"Comma-separated IdP group(s) seeding the operator role")
fs.StringVar(&c.oidcRoleMappingViewer, "oidc-role-mapping-viewer", envOr("GAMEPLANE_OIDC_ROLE_MAPPING_VIEWER", ""),
	"Comma-separated IdP group(s) seeding the viewer role")
```

**Parsing** (building the seeded policy after `fs.Parse` returns, used to construct the `"helm"`
provider's `*auth.ProviderPolicy` via `auth.NewOIDCWithPolicy`, replacing today's plain
`auth.NewOIDC` call at `api/cmd/main.go:127`):

```go
splitGroups := func(csv string) []string {
	if csv == "" {
		return nil
	}
	out := make([]string, 0, 4)
	for _, g := range strings.Split(csv, ",") {
		if g = strings.TrimSpace(g); g != "" {
			out = append(out, g)
		}
	}
	return out
}

var roleMappings *auth.RoleMappings
admin, operator, viewer := splitGroups(c.oidcRoleMappingAdmin), splitGroups(c.oidcRoleMappingOperator), splitGroups(c.oidcRoleMappingViewer)
if len(admin) > 0 || len(operator) > 0 || len(viewer) > 0 {
	roleMappings = &auth.RoleMappings{Admin: admin, Operator: operator, Viewer: viewer}
}

policy := &auth.ProviderPolicy{
	GroupsClaim:  c.oidcGroupsClaim,
	RoleMappings: roleMappings,
	DefaultRole:  c.oidcDefaultRole,
}
```

---

## 4. Helm Template Integration

**File**: `charts/gameplane/templates/api.yaml` (Deployment spec)

**Template Snippet** (in `spec.containers[].args`) — note each mapping key renders as **one flag
with a comma-joined value**, via Helm's `join`, not a `range` loop emitting one flag per group:

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

**Behavior**:
- If `api.oidc.issuer` is empty/nil, ALL OIDC flags are omitted (no Helm OIDC setup).
- If `issuer` is set but `groupsClaim` is empty, the `--oidc-groups-claim` flag is omitted (uses CLI default "groups").
- If `issuer` is set but `roleMappings.admin` is empty, the `--oidc-role-mapping-admin` flag is omitted entirely (rather than emitted with an empty value).
- Each of `roleMappings.admin|operator|viewer`, when non-empty, produces exactly **one** flag whose value is the comma-joined array — never one flag per group.

---

## 5. Environment Variables (Optional Fallback)

**Convention** (optional, for development or container-to-CLI integration — `envOr` is the same
helper every existing flag in `bindFlags` already uses):

| Environment Variable | Flag | Example |
|----------------------|------|---------|
| `GAMEPLANE_OIDC_GROUPS_CLAIM` | `--oidc-groups-claim` | `groups` or `roles` |
| `GAMEPLANE_OIDC_DEFAULT_ROLE` | `--oidc-default-role` | `viewer`, `admin`, `deny` |
| `GAMEPLANE_OIDC_ROLE_MAPPING_ADMIN` | `--oidc-role-mapping-admin` | `idp-admins,ops-team` (comma-separated) |
| `GAMEPLANE_OIDC_ROLE_MAPPING_OPERATOR` | `--oidc-role-mapping-operator` | `idp-ops` (comma-separated) |
| `GAMEPLANE_OIDC_ROLE_MAPPING_VIEWER` | `--oidc-role-mapping-viewer` | `idp-viewers` (comma-separated) |

**Usage** (example with env vars):
```bash
export GAMEPLANE_OIDC_ISSUER="https://idp.example.com"
export GAMEPLANE_OIDC_CLIENT_ID="client-123"
export GAMEPLANE_OIDC_GROUPS_CLAIM="roles"
export GAMEPLANE_OIDC_DEFAULT_ROLE="viewer"
export GAMEPLANE_OIDC_ROLE_MAPPING_ADMIN="idp-admins,platform-team"
export GAMEPLANE_OIDC_ROLE_MAPPING_OPERATOR="idp-ops"

./api serve
```

**Note**: Helm-managed deployments use CLI flags directly (via Deployment `args:`); env vars are the
same `envOr`-fallback convention every other `--oidc-*` flag already uses, and are optional for
manual testing.

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

### Generated CLI Flags (Helm template output — one flag per role, comma-joined)
```bash
./api serve \
  --oidc-issuer="https://idp.example.com" \
  --oidc-client-id="..." \
  --oidc-groups-claim="groups" \
  --oidc-role-mapping-admin="gameplane-admins,devops-team" \
  --oidc-role-mapping-operator="gameplane-operators" \
  --oidc-role-mapping-viewer="gameplane-viewers,readonly-users"
```

### Direct CLI Invocation (same shape — a human writes the comma-joined value directly)
```bash
./api serve \
  --oidc-issuer="https://idp.example.com" \
  --oidc-client-id="..." \
  --oidc-groups-claim="groups" \
  --oidc-role-mapping-admin="gameplane-admins,devops-team" \
  --oidc-role-mapping-operator="gameplane-operators" \
  --oidc-role-mapping-viewer="gameplane-viewers"
```

---

## 7. Error Handling & Validation

**Validation** (performed at API startup):

1. **Invalid `--oidc-default-role`**: Must be one of `""`, `"viewer"`, `"operator"`, `"admin"`, `"deny"`
   - Error: Fatal exit with message `"invalid OIDC default role: must be one of '', 'viewer', 'operator', 'admin', 'deny'"`

2. **Empty element in a comma-separated `--oidc-role-mapping-admin|operator|viewer` value**: A
   trailing/leading/doubled comma (e.g. `"admins,,ops"`) yields dropped empty elements after
   `TrimSpace`, not an error — mirrors how `validateProviderMapping` treats blank entries in the
   DB-stored `providers[].roleMappings.*` arrays as invalid input, but at the CLI layer this is a
   parse-time cleanup, not a rejected flag (there is no interactive feedback loop at startup the way
   there is for a dashboard form submission).

3. **No flags set** (issuer but no mappings/groupsClaim): Warning logged; OIDC works but all users get default role

---

## 8. Backward Compatibility

- **Existing installs without role mappings**: All new flags are optional and default to empty. Behavior unchanged (new OIDC users default to "viewer", roles never re-evaluated).
- **Existing CLI invocations**: No required flags added; opt-in only.
- **Helm values**: All new keys optional; empty/omitted values preserve legacy behavior.
- **Upgrade semantics for admin-overridden roles (M9)**: changing one of these flags' values on a
  `helm upgrade` is safe to do at any time — it never silently reasserts over a role an admin has
  since overridden through the dashboard (§0). There is no flag-only way to force a Helm value back
  onto an overridden role; that requires the dashboard's reset action
  (`DELETE /admin/config/auth/role-mappings/{role}`).

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
