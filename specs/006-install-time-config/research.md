# Research: Design Decisions for Install-Time Configuration (Feature 006)

This document synthesizes codebase analysis into design decisions for Gameplane's install-time configuration feature (storage class defaults and OIDC role mappings). Each decision cites exact file:line evidence from the verified research phase.

---

## Verification of Spec Claims

### Claim 1: Helm chart value structure for storage class

**Status: VERIFIED**

The Helm chart already follows an established pattern for persistent storage configuration. Review of `charts/gameplane/values.yaml` (lines 126–128) documents:

```yaml
api:
  storage:
    size: 2Gi
    storageClassName: ""  # empty = cluster default
```

This pattern shows that storage-class defaults are nested under the component name (api) with two keys: `size` and `storageClassName`. Following this convention, the operator (which manages game-server storage) should define a parallel structure at `operator.gameDataStorage.storageClassName`.

**Evidence**: `charts/gameplane/values.yaml:126–128` (api storage config); `charts/gameplane/templates/operator.yaml:247–308` (operator args binding pattern). Existing operator flags use kebab-case convention (`--leader-elect`, `--agent-image`, `--zap-log-level`, etc.).

**Verification complete**: The value key `operator.gameDataStorage.storageClassName` mirrors the `api.storage.storageClassName` precedent and is the correct location.

---

### Claim 2: Helm-configured provider flow and immutability

**Status: VERIFIED**

The codebase already implements a reserved OIDC provider pattern via `HelmProviderName`. Review of `api/internal/auth/registry.go` (lines 30, 199–203) confirms:

- A synthetic provider named `"helm"` is synthesized at runtime from CLI flags and never persisted to the database.
- The provider is listed as read-only and cannot be edited or deleted through the dashboard.
- Validation in `api/internal/handlers/config.go` (lines 268–269) explicitly rejects any database-managed provider with name `"helm"`, preventing namespace collisions.

The guard prevents accidental shadowing of the Helm-configured provider and enforces immutability: dashboard-managed providers can have any name except `"helm"`.

**Evidence**: `api/internal/auth/registry.go:30` (constant), `:200–202` (synthesis logic); `api/internal/handlers/config.go:268–269` (validation guard); `api/internal/auth/oidc.go:374–411` (syncUserRole logic ensuring Helm-configured role mappings work correctly).

**Verification complete**: The reserved `"helm"` provider name is guarded and cannot be accidentally created via the dashboard. Immutability is enforced structurally.

---

### Claim 3: OIDC group claim name configurability

**Status: VERIFIED**

The OIDC implementation is fully flexible on claim names. Review of `api/internal/auth/oidc.go` (lines 244–254) shows:

```go
claimName := ""
if o.policy != nil {
    claimName = o.policy.GroupsClaim  // Admin-configurable
}
role, deny := computeRole(extractGroups(rawClaims, claimName), o.policy)
```

The group claim name is configurable per OIDC provider via `Provider.GroupsClaim` (defined in `api/internal/auth/registry.go:72–74`). If empty, it defaults to `"groups"`. Admins can set this via the admin configuration interface at `/admin/config` → `auth` → `providers[i].groupsClaim`.

**Evidence**: `api/internal/auth/oidc.go:250–252` (claim name extraction); `api/internal/auth/registry.go:72–74` (Provider struct); `api/internal/handlers/config.go:203–208` (validation of groupsClaim).

**Verification complete**: The group claim name is fully configurable with a sensible default of `"groups"`. No code changes are required to support different claim names.

---

## Design Decisions

### Decision 1: Helm Value Key and Operator Flag for Game-Data Storage Class Default

**Decision**: 
- **Helm key**: `operator.gameDataStorage.storageClassName` (nested under operator config, empty string as default)
- **Operator Deployment flag**: `--game-data-storage-class=""` (accepts any string; empty means cluster default; used to materialize PVCs)
- **API Deployment flag** (D2): `--game-data-storage-class=""` (NEW; report-only flag on the `api serve` subcommand, passed to `/admin/config` response as `installTimeSettings.gameDataStorageClass`)
- **Single source of truth**: Both deployments receive the same Helm value from a single key (`operator.gameDataStorage.storageClassName`), ensuring consistency across components.

**Rationale**:
- **Consistency with existing API storage pattern**: `api.storage.storageClassName` already provides a storage class default for the API's SQLite database PVC. Mirroring this structure (`operator.gameDataStorage.storageClassName`) ensures operators understand the precedence immediately.
- **Kebab-case flag naming**: All existing operator flags use kebab-case (`--agent-image`, `--leader-elect`, `--zap-log-level`, `--capture-default-retention-seconds`). The new flag `--game-data-storage-class` follows this convention (`operator/cmd/main.go:150–245` defines 30 explicit flags).
- **Nested structure for future expansion**: Placing the storage class under `operator.gameDataStorage.*` (rather than a flat `operator.storageClassName`) allows future extensions like `operator.gameDataStorage.size` or `operator.gameDataStorage.accessModes` without restructuring.
- **Empty string means cluster default**: Kubernetes convention treats an unset or empty `storageClassName` as "use the cluster's default StorageClass." This preserves backward compatibility: operators who do not set this value get the current behavior.
- **Two-consumer Helm value (D2 rationale)**: The operator and API are separate binaries in separate pods. Both must receive the same Helm value to stay synchronized. The API flag is read-only (not used for business logic) and exists purely to report the install-time setting to admins via the config API endpoint. This avoids duplication and ensures single-source-of-truth semantics. Helm sets both via the same `operator.gameDataStorage.storageClassName` key (N2).

**Alternatives considered**:
- **Flat key `operator.storageClassName`** (rejected): Conflicts semantically with potential future API storage class config and creates ambiguity about which component the setting applies to.
- **Storing the default in an API config table at runtime** (rejected): The value is install-time immutable (sourced from Helm/environment flags), not a runtime admin-configurable setting. Storing it in the database would create confusion about whether it can be changed post-install; placing it in Helm values makes immutability explicit.
- **Environment-variable-only configuration** (rejected): Operators already work with Helm values; adding a second configuration path (env vars) increases cognitive load and makes the install-time setting less discoverable.

---

### Decision 2: Precedence Chain for StorageClassName

**Decision**: 
The following precedence applies when materializing a PVC for a GameServer:

1. **GameServer.Spec.Storage.StorageClassName** (if set; highest priority)
2. **GameTemplate.Spec.Storage.StorageClassName** (if set; template-level override)
3. **Operator flag `--game-data-storage-class`** (install-time default)
4. **Nil → Kubernetes cluster default** (if all above are unset; lowest priority)

**Rationale**:
- **Most-specific-first principle**: GameServers can override templates, which override cluster defaults. This gives operators maximal flexibility.
- **Install-time default fills the gap**: The operator flag provides a cluster-wide default (tier 3) that applies when neither GameServer nor GameTemplate have explicit settings. This is the intended use case for FR-001/FR-002.
- **Backward compatibility preserved**: If the operator flag is not set (empty), the behavior is unchanged from today: explicit GameTemplate/GameServer settings are honored, and unset values fall through to the cluster default. No existing deployments are affected.
- **Evidence from codebase**: PVC creation sites (`operator/internal/controller/gameserver_controller.go:515–526`, `gameserver_extravolumes.go:122–129`, `gameserver_version.go:201–212`) already implement tiers 1–2. Tier 3 (operator flag) is inserted after the nil-check, before the PVC is submitted to the API.

**Alternatives considered**:
- **Operator flag overrides everything** (rejected): Would break existing GameTemplate-level precedence and make explicit template settings non-functional — a breaking change for existing users.
- **No install-time default; Helm values only at chart level** (rejected): Operators need to pass this to the operator Deployment's args, not just the chart values.yaml. The operator itself must receive and enforce the setting.

---

### Decision 3: CRD Type Changes Required

**Decision**: 
**No CRD type changes are required.** The storage class feature is implemented purely in operator flag plumbing, not in CRD types.

**Rationale**:
- **StorageClassName already exists in CRDs**: Both `GameServer` and `GameTemplate` have `Spec.Storage.StorageClassName` fields (type `*string`, omitempty). These were added in prior work and are fully schema-compatible. No new fields need to be added to `gameserver_types.go` or `gametemplate_types.go`.
- **Citation from codebase**: `operator/api/v1alpha1/gametemplate_types.go:900` defines `GameStorageSpec` with `StorageClassName *string`, used by both GameServer and GameTemplate. Both CRD types are already generated and include this field.
- **Consequence**: `make generate` and `make manifests` are NOT required for this feature. The CRD YAML in `charts/gameplane/crds/` does not change.

**Alternatives considered**:
- **Add a new field like `Spec.StorageClassOverride`** (rejected): Unnecessary and confusing; the existing `Spec.Storage.StorageClassName` already serves this purpose.
- **Modify the GameStorageSpec struct** (rejected): Already has the field; no modifications needed.

---

### Decision 4: Detection and Reporting of Nonexistent StorageClass

**Decision**:
- **Primary detection**: Direct Kubernetes API GET of the StorageClass by name during PVC creation in `reconcilePVC()`. If 404 NotFound, fail the PVC creation and report the error to the status.
- **Secondary detection**: Pod event inspection during status reconciliation. If a Pod's `FailedScheduling` or `FailedAttachVolume` event mentions "StorageClass not found", capture it as a supplementary signal.
- **Reporting**: Set the GameServer's Ready condition to False with Reason `"PVCProvisioningFailed"` and a message like "StorageClass 'fast-nvme' not found on cluster" in the condition's `Message` field. This renders in the dashboard via `ServerDetail.tsx:179–186`.

**Rationale**:
- **Prior art in codebase**: The MetalLB IPAddressPool validation pattern (`operator/internal/controller/gameserver_status.go:125–151`) already performs a direct resource GET to detect pool nonexistence without waiting for events. Events are transient (TTL ~1 hour) and may never be emitted by some CSI provisioners. A direct GET provides immediate, reliable feedback.
- **Early detection**: Failing in `reconcilePVC()` (line 505) before the Pod is created surfaces the error quickly. The operator requeue loop (default ~5s) ensures the error is re-evaluated frequently, and the dashboard polls every 5s (ServerDetail.tsx:67), so the error appears in the UI within ~5–10 seconds (meeting SC-002's requirement).
- **Reusable components**: The condition-setting logic in `computeConditions()` (gameserver_status.go:286–379) and `upsertCondition()` already handle Ready condition updates. Pod event extraction via `listServiceEvents()` (line 2138+) is a proven pattern that can be adapted to Pod events.
- **Dashboard integration**: The ServerDetail route already renders a Ready condition with message as `failureMessage` (line 179–186), displaying it under an AlertTriangle icon. No new UI code is required.

**Alternatives considered**:
- **Wait for a PVC event** (rejected): Many CSI provisioners never emit an event; some only emit after 5+ minutes. Gameplane must detect the failure immediately, not wait for an eventual event.
- **Poll the PVC status indefinitely** (rejected): The PVC will remain Pending forever if the StorageClass doesn't exist. Polling won't recover the error; only a direct StorageClass check will.
- **Silent failure with a generic "Pending" message** (rejected): This is the current behavior and is the exact problem FR-005 aims to solve. Users are left confused with no actionable feedback.

---

### Decision 5: OIDC Role Mappings — Helm-Synthesized vs. Database vs. Both

**Decision**:
**Layered approach**: OIDC role mappings reside in TWO independent locations:

1. **Helm-configured provider** (`"helm"` synthetic provider): Immutable at runtime. Sourced from Helm values / CLI flags at startup. Configured by operators via Helm chart values at install time. Never edited through the admin UI.
2. **Database-managed providers**: Mutable at runtime. Stored in the `config` table. Configured by admins through `/admin/config` page. Can be created, edited, and deleted at runtime. Do NOT include the reserved name `"helm"`.

Both paths coexist independently. The Helm provider's group claims are evaluated first (if configured); database providers are evaluated second. Admins can manage secondary (database-stored) OIDC providers for runtime flexibility while the install-time Helm provider remains immutable.

**Rationale**:
- **Helm immutability requirement** (FR-007): Operators must be able to specify role mappings at install time without a bootstrap-admin step. If role mappings are mutable in the database and stored there by default, then the operator still needs to run bootstrap-admin (or manually edit the database) to set the initial mappings — creating a chicken-and-egg problem. By sourcing mappings from Helm/environment, the first OIDC login can immediately apply role mappings without any admin account existing.
- **Runtime flexibility requirement** (SC-007): Admins must be able to update role mappings post-install via the admin UI without re-running Helm or restarting the API. The database-stored providers path satisfies this.
- **Layering avoids duplication**: A single unified config in the database would require operators to seed it (still needing bootstrap-admin or direct database edits). Layering avoids this by letting Helm drive install-time defaults and the database drive runtime mutations.
- **Reserved name guard prevents collision**: The synthetic `"helm"` provider name is reserved and validated at `config.go:268–269`. This prevents accidental shadowing or deletion of the Helm-configured provider.
- **Evidence from codebase**: `api/internal/auth/registry.go:143–202` (Enabled method) already constructs and returns the Helm provider at runtime from CLI flags. `api/internal/auth/oidc.go:237–254` evaluates OIDC token claims and applies the mapping. The logic is already present to support both sources.

**Alternatives considered**:
- **Helm-only, no runtime mutations** (rejected): Violates SC-007 (admins cannot update mappings without Helm/API restart).
- **Database-only, no Helm pre-configuration** (rejected): Violates FR-007 (requires bootstrap-admin even with OIDC-only install).
- **Helm and database merged into a single config key** (rejected): Creates ambiguity about precedence and makes it unclear which source is authoritative. The layering approach is explicit: Helm is install-time immutable, database is runtime mutable.

---

### Decision 6: Group Claim Name Configurability

**Decision**:
**Fully configurable per provider.** The group claim name is specified via `Provider.GroupsClaim` (a string field in the provider config). If empty, it defaults to `"groups"`. Admins set this via `/admin/config` → `auth` → `providers[i].groupsClaim` for database-managed providers. For the Helm-configured provider, operators set it via Helm values (e.g., `api.oidc.groupsClaim` per N1).

**Rationale**:
- **OIDC providers use different claim names**: Okta uses `"groups"`, Azure AD uses `"roles"`, some custom providers use `"membership"`. Gameplane cannot assume a single claim name; it must be configurable.
- **Existing implementation supports this**: `api/internal/auth/oidc.go:250–252` reads `o.policy.GroupsClaim` and passes it to `extractGroups()`. The logic is already in place.
- **Validation already present**: `api/internal/handlers/config.go:203–208` validates that `groupsClaim` is not blank if set. No additional validation is needed.
- **Default is sensible**: "groups" is the most common claim name and matches OIDC standards. Operators who do not specify a custom claim name get this default automatically.

**Alternatives considered**:
- **Hardcoded claim name** (rejected): Would require re-coding per OIDC provider, defeating the purpose of a configurable OIDC flow.
- **Enumerated list of "approved" claim names** (rejected): Too restrictive; custom OIDC providers and organizational configurations use non-standard claim names.

---

### Decision 7: Role Re-Evaluation on Login and Interaction with Manually-Assigned Roles

**Decision**:

**Role re-evaluation occurs if and only if the OIDC provider has RoleMappings configured.**

- **If provider has RoleMappings** (e.g., a Helm-configured provider with admin/operator/viewer mappings): On each login, the user's OIDC token groups are compared against the mappings. The user's role is updated to match the mapping result. A guard prevents demotion: if the new role would remove the last user capable of managing users (e.g., last admin), the update is skipped and the previous role is retained (applied=false in `syncUserRole`).
- **If provider has NO RoleMappings** (e.g., a secondary OIDC provider added via the admin UI with no mappings): The user's role is assigned once at first login (based on `DefaultRole`, typically "viewer") and is never re-evaluated on subsequent logins. Manually-assigned roles (e.g., via admin grant or bootstrap-admin) persist indefinitely.

**Rationale**:
- **Explicit intent**: If an admin configures role mappings, they intend for role assignment to be automatic and dynamic. If they do not configure mappings, role assignment is manual and static. This is the principle of least surprise.
- **Bootstrap-admin coexistence** (FR-013): A bootstrap-admin-created admin account must not be clobbered by OIDC re-evaluation. If no OIDC role mappings are configured (`syncRole=false`), the bootstrap-admin role is preserved indefinitely. If mappings are configured later (e.g., Helm upgrade with new mappings), existing users are re-evaluated on next login; bootstrap-admin can set `DefaultRole="deny"` to prevent this.
- **Prevents lockout** (edge case "Misconfigured OIDC role mappings leave no admin"): The demotion guard ensures that if role mappings are misconfigured such that the sole admin is demoted, the admin is NOT demoted. This preserves access to the admin UI to fix the mappings. An admin can always escape by running bootstrap-admin as a fallback.
- **Evidence from codebase**: `api/internal/auth/oidc.go:262–311` (login handler) checks `syncRole := o.policy != nil && o.policy.RoleMappings != nil`. If true, `syncUserRole()` is called; if false, stored role is kept. `syncUserRole()` (lines 374–411) includes the demotion guard at lines 379–393.

**Alternatives considered**:
- **Always re-evaluate on login** (rejected): Violates the principle of explicit intent and could clobber manually-assigned roles.
- **Never re-evaluate on login** (rejected): Violates FR-011 (users' roles must be updated if their group membership changes).

---

### Decision 8: Reserved Provider Name Guard for "helm"

**Decision**:
**Enforce at the validation layer** in `api/internal/handlers/config.go:268–269`. When a dashboard user or API client attempts to create or update a provider with `Name=="helm"`, the API returns a validation error: `"providers[i].name 'helm' is reserved for the Helm-configured provider"`.

The validation occurs:
1. On provider creation via `/admin/config` PUT (handlers/config.go validateAuth)
2. On provider update (same validation)
3. Never on the synthetic Helm provider itself (it is not stored in the database and cannot be mutated)

**Rationale**:
- **Prevents accidental shadowing**: If an admin creates a database provider named "helm", it would shadow or conflict with the synthetic Helm-configured provider. The guard prevents this collision.
- **Enforces immutability**: The synthetic provider is read-only (not in the database). Rejecting any database provider with that name keeps the boundaries clear.
- **Simple to implement**: A one-line check (`if p.Name == "helm" { return error }`). No complex logic required.
- **Evidence from codebase**: `api/internal/handlers/config.go:268–269` (existing guard); `api/internal/auth/registry.go:30` (constant HelmProviderName).

**Alternatives considered**:
- **Silent shadowing** (rejected): Confusing for admins; unclear which provider is active.
- **Allow collision and pick one** (rejected): Ambiguous; creates race conditions if both coexist in the config.
- **Rename the synthetic provider** (rejected): Would break existing Helm deployments and documentation.

---

### Decision 9: Over-Broad Mapping Warning (FR-015)

**Decision**:
**No backend validation.** Instead, provide a **UI-level warning and explicit confirmation** in the admin configuration interface (`web/src/routes/AdminSettings.tsx`), specifically when editing OIDC provider role mappings.

**Mechanism**:

1. **Advisory Callout** (rendered unconditionally for every mapping):
   When an admin views or edits an OIDC role mapping rule (e.g., "group 'employees' → admin role"), display an informational callout:

   > "Warning: This mapping grants [role] to any user in the '[group]' group. If this group has many members, verify that you intend to grant this role to all of them. Gameplane cannot enumerate group membership from the OIDC provider."

2. **Explicit Confirmation Step** (when target role is "admin"):
   If the admin is saving a mapping whose target role is `"admin"`, show an additional confirmation dialog or checkbox before the save completes:

   > "Confirm: This mapping will grant admin access to all users in '[group]'. This cannot be revoked automatically. Click 'I understand' to proceed, or edit the mapping."

   The mapping is NOT saved until the admin explicitly confirms. This satisfies SC-007 (runtime mutation safety).

3. **Non-blocking for non-admin roles**:
   For operator and viewer roles, the advisory callout is shown, but no explicit confirmation is required (the admin can save directly).

**Rationale**:
- **Gameplane cannot enumerate IdP groups**: OIDC tokens carry the claims issued by the provider; Gameplane does not query the IdP's group directory. It cannot determine how many users are in a group.
- **Trust boundary**: The OIDC provider is authoritative. If an operator misconfigures a mapping, that is an operator error, not a Gameplane bug. Gameplane can warn and require confirmation, but cannot enforce.
- **FR-015 is satisfied**: The spec asks for "validation or confirmation when a single mapping covers a non-trivial number of users." An unconditional warning + explicit confirmation for admin mappings satisfies this by notifying the admin of the risk and forcing a deliberate acknowledgment.
- **Proportionate friction**: Confirmation is required only for the most dangerous role (admin); lower-privilege roles incur no friction.
- **Keep implementation lean**: No IdP API integrations, no enumeration, no complex heuristics.

**Delivery**: This is implemented as its own slice in the dashboard implementation, separate from core role-mapping functionality.

**Alternatives considered**:
- **Query the OIDC provider's group API** (rejected): Out-of-band IdP queries introduce latency, flakiness, and trust boundary issues. Some IdPs don't expose group membership queries; others require elevated credentials.
- **Reject mappings with "suspicious" group names** (rejected): Arbitrary heuristics (e.g., "reject if the group name contains 'org'" or "employees") are unreliable and culturally insensitive.
- **Advisory only, no confirmation** (rejected): Does not fully satisfy FR-015's call for confirmation. An explicit checkpoint is more effective than a passive warning.

---

### Decision 10: Test Strategy and E2E Bucket Assignments

**Decision**:

**Unit/Envtest Tier**:
- Unit tests for role-mapping logic (`api/internal/auth/oidc_rolemap_test.go`): Extend existing `TestComputeRole()` with new cases for Helm provider configuration, "deny" default role, and edge cases.
- New envtest file (`operator/internal/controller/gameserver_storage_envtest_test.go`): Test PVC creation with explicit/template/install-time storage class settings and error cases (nonexistent StorageClass).

**E2E Tier** (in CI kind clusters):
- **`api-auth` bucket**: Add 3 OIDC tests: `TestAPI_OIDCRoleMappingAtInstallTime`, `TestAPI_OIDCRoleMappingFirstLogin`, `TestAPI_OIDCRoleMappingReEvalOnLogin`. Each test uses a fake OIDC issuer (in-process, following the pattern from `oidc_rolemap_test.go`). Total new logins: ~2–3 (within the api-auth bucket's per-user burst of 6 logins + retries).
- **`operator` bucket**: Add 3 storage class tests: `TestGameServer_StorageClassFromHelmDefault`, `TestGameServer_StorageClassNotFound`, `TestGameServer_StorageClassExplicitOverride`. All parallel-safe, zero login pressure.

**OIDC E2E feasibility**:
- **VERIFIED as feasible**: The `newFakeIDP()` pattern from `api/internal/auth/oidc_rolemap_test.go:135` can be ported to e2e. A fake OIDC issuer running in-process is the proven approach; there is NO shared test IdP in CI, and tests deliberately use an unreachable issuer (`https://e2e-idp.invalid`) for listing/routing without dialing (api_auth_e2e_test.go:278–279).

**Audit event testing**: Audit events record OIDC role assignments per N4, with reason format: `oidc role assigned: provider=<name> matched=<claimValue|none> from=<oldRole> to=<newRole>`. Tests verify the event is recorded correctly in the audit_events table with columns: id, ts, actor, method, path, target, status, ip, reason.

**Rationale**:
- **No external IdP dependency**: A fake issuer keeps tests isolated and deterministic. External providers (Okta, Keycloak) introduce flakiness and are out-of-scope.
- **Login budget constraint**: Each test in the api-auth bucket consumes part of a per-user rate limit (3/min, burst 6). Adding ~2–3 logins fits comfortably within the budget; re-using bootstrap-admin login across tests minimizes overhead.
- **Parallel safety**: Storage class tests are pure reconciliation with zero admin UI involvement; they run in parallel and do not contend for resources. OIDC tests in api-auth run either sequentially or re-use shared sessions to avoid rate-limit exhaustion.
- **Coverage impact**: New unit tests add ~5–10% coverage to api/auth and operator controller without triggering rebaselines (both modules are at 80% and 72% respectively).

**Alternatives considered**:
- **Skip OIDC e2e testing** (rejected): Does not verify that a real login works end-to-end. Unit tests alone are insufficient for a user-facing login flow.
- **Use a real external IdP** (rejected): Introduces CI flakiness, trust boundaries issues, and test isolation violations.
- **All tests in one bucket** (rejected): Exceeds the api-auth bucket's login budget (7 logins, and new tests would add 2–3 more, risking rate-limit overflows).

---

## Implementation & Design Requirements

### Pencil Design Pass (Constitutional Principle II)

This feature includes a **visual-surface change** to the dashboard: a new section in the AdminSettings page displaying install-time settings (storage class default) and the FR-015 OIDC role-mapping warning with confirmation step. Per Constitution Principle II, this requires:

1. A **design.pen pass** (first step of the dashboard implementation slice):
   - Update `design.pen` via the Pencil MCP server to add/revise the AdminSettings install-time settings section and the OIDC mapping confirmation flow.
   - Export the touched nodes to `design-export/json/<id>.json` and `design-export/screenshots/<id>.png`.
2. **Touch points**: Includes `web/specs.md` (specifications for the dashboard component).

---

## Conclusion

These 10 design decisions establish a coherent path forward for install-time configuration. The decisions are grounded in codebase review and follow Gameplane's existing conventions:

1. Helm value key and operator flag naming mirror existing storage-class patterns (api.storage). Canonical names per N1/N2/N3: `api.oidc.*` (not api.auth.oidc), `operator.gameDataStorage.storageClassName`, flags `--oidc-*` and `--game-data-storage-class`.
2. PVC precedence is explicit and backward-compatible.
3. No CRD changes are required; existing fields suffice.
4. StorageClass error detection reuses proven MetalLB validation patterns.
5. OIDC role mapping is layered: Helm-immutable + database-mutable, avoiding chicken-and-egg install problems.
6. Group claim names are already configurable; no code changes needed.
7. Role re-evaluation is explicit (controlled by presence of RoleMappings) with demotion guards.
8. The reserved "helm" provider name is guarded at the validation layer.
9. Over-broad mapping risks are communicated via UI warnings with explicit confirmation for admin mappings (no backend enumeration). Satisfies SC-007 (runtime mutations with safety).
10. Tests are feasible with fake OIDC issuers and fit within existing CI bucket constraints. Audit event validation per N4 (single reason format).
11. Dashboard visual changes require a Pencil design pass (Constitutional Principle II) as the first step of the dashboard implementation slice.
