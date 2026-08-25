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

**Addendum (maintainer hybrid decision, see Decision 5)**: this immutability claim is about the synthetic `"helm"` *provider object* — its `Kind`, `DisplayName`, issuer/client wiring, and (per this feature) its Helm-seeded `RoleMappings`/`DefaultRole` fields remain read-only and are never written to by the dashboard. The hybrid design adds a `helmOverride` field that admins edit through the dashboard, stored as a sibling of `providers` inside the same `"auth"` config row (`api/internal/auth/registry.go:175–185`); that field is not a mutation of the `"helm"` provider row and does not weaken this claim. See Decision 8 for why the two must not be conflated.

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

### Decision 5: OIDC Role Mappings — Hybrid: Helm Seeds, `helmOverride` (in the Existing Auth Config Row) Overrides

**Status: SUPERSEDES an earlier "Helm-only, read-only" resolution of this decision.** The original design put role mappings entirely in the Helm-synthesized `"helm"` provider and left the database path for *unrelated, secondary* OIDC providers only. That design satisfied FR-007 (install-time mappings, no bootstrap-admin needed) but **failed SC-007**: "An operator can manage role mappings through the administrative configuration interface with changes taking effect for the next login attempt without restarting the API or re-running Helm." A Helm-only mapping can only be changed by editing chart values and running `helm upgrade` (which also restarts the API pod) — there is no in-dashboard path to change a role mapping at all, let alone one that avoids a Helm re-run.

**Decision (maintainer-directed, binding)**: **Hybrid.** Helm values seed the initial role mappings; a database-persisted overlay lets admins override them at runtime, with the override taking effect on the mapped user's next login — no API restart, no `helm upgrade`. **This adds zero new tables and zero new migration files.** It reuses the single existing `"auth"` row in the `config` table verbatim.

1. **Helm-seeded mappings**: Configured via `api.oidc.roleMapping.*` Helm values (canonical keys per the API flags below), passed to the API as `--oidc-role-mapping-{admin,operator,viewer}` (comma-separated group lists) and `--oidc-default-role`. These are read at API startup and held in memory — the same mechanism the pre-hybrid design already used for the synthetic `"helm"` provider's `RoleMappings`/`DefaultRole` fields (`api/internal/auth/registry.go:143–202`). This satisfies FR-007/SC-003/SC-004: a fresh OIDC-only install has working role mappings before any admin account exists.
2. **Database overlay storage**: the admin-managed auth config is already DB-persisted as a single row in the `config` table under key `"auth"`, holding JSON of the shape `{"providers":[...]}` (`api/internal/auth/registry.go:175–185`, verified). The overlay adds **one optional sibling field** next to `"providers"` in that same JSON blob:
   ```json
   {
     "providers": [ /* unchanged */ ],
     "helmOverride": {
       "roleMappings": { "admin": ["..."], "operator": ["..."], "viewer": ["..."] }
     }
   }
   ```
   `helmOverride` is **not** a provider entry — it does not go through the `providers` array or the reserved-name guard at all (Decision 8, Decision 13). Each of the three role keys (`admin`, `operator`, `viewer`) is independently optional; a role with no key falls back to the Helm-seeded list for that role (Decision 7). Admins edit `helmOverride` through the admin configuration interface (H7 — this is a new *editing* surface, see Decision 10), via the *same* `PUT /admin/config/auth` endpoint that already writes the `providers` array (Decision 10 has the exact route). A `helmOverride.roleMappings.<role>` list, once set, **replaces** the Helm-seeded group list for that role in the resolution used at the next login (Decision 7 covers precedence composition).
3. **Both layers coexist permanently** — this is not a one-time migration from Helm into the database. An install can run indefinitely with some roles Helm-seeded and others DB-overridden.
4. **The helm provider bypasses the DB provider cache entirely, so nothing is cached for it to invalidate — the override is simply read fresh on every login.** `OIDCFor` (`api/internal/auth/registry.go:220–227`, verified) short-circuits at the top: `if name == HelmProviderName { ...; return r.legacy, nil }`, returning before `snapshot()` runs and before the SHA-256 row-hash cache (`registry.go:172–186`) is ever consulted. That hash-based cache governs **DB-managed providers only**. `r.legacy` is a single `*OIDC` built once at API startup (`api/cmd/main.go:127`) and held for the process lifetime — there is no per-request rebuild and no cache to invalidate on the helm path. SC-007's "no API restart" for the helm provider is therefore **not** delivered by the registry's cache mechanism at all. It is delivered by reading `helmOverride` directly from the `"auth"` config row **during role resolution, once per login attempt**: `registry.go:152–153` already calls `legacy.AttachStore(store)`, so the helm-flag `*OIDC` has the store handle it needs to read `helmOverride` itself at login time (the same seam Decision 7's `effectiveHelmPolicy` merges into, ahead of the unmodified `computeRole`). Because this read happens fresh on every login attempt — not from any cache — an admin's edit through the dashboard lands on the very next login with no API restart and no `helm upgrade`. **No new caching layer is introduced or required.**

**Rationale**:
- **FR-007 / SC-003 / SC-004 (install-time, no bootstrap-admin)**: Satisfied by the Helm-seeded layer alone — nothing in the hybrid design requires a database row to exist before the first login resolves a role.
- **SC-007 (runtime mutation, no Helm/API restart)**: Satisfied by the database overlay — admins edit `helmOverride` through the dashboard, the write touches the same `"auth"` row, and because the helm provider path (`OIDCFor`, `registry.go:220–227`) never goes through the DB-provider row-hash cache in the first place (point 4 above), there is nothing to invalidate: `helmOverride` is read directly from the row during role resolution on every login attempt, so the change is picked up on the very next login (`resolveOrLinkUser`/`syncUserRole` already re-read policy state per login).
- **Overlay, not an edit to the synthetic provider**: The `"helm"` provider object itself (`HelmProviderName`, `api/internal/auth/registry.go:30`) is unaffected and stays read-only per Claim 2 (H4, Decision 8) — `helmOverride` is a sibling JSON field consulted during role resolution, not a mutation path into the provider registry. This keeps the existing reserved-name guard (Decision 13) meaningful: the `"helm"` provider name is still reserved and un-creatable as a *provider*, while `helmOverride` is intentionally editable and was never subject to that guard in the first place.
- **Evidence from codebase**: `api/internal/auth/registry.go:175–185` (the single `"auth"` config row and its `{"providers":[...]}` shape); `:143–202` (Enabled method constructs the Helm provider's RoleMappings from flags at startup); `:220–227` (`OIDCFor` short-circuits to `r.legacy` for `HelmProviderName`, before `snapshot()` and the row-hash cache — the reason the cache is not, and cannot be, the SC-007 mechanism for the helm provider); `api/cmd/main.go:127` (`r.legacy` is a single `*OIDC` built once at startup and held for the process lifetime — confirming there is no per-request rebuild on that path to invalidate); `api/internal/auth/oidc.go:121–153` (`computeRole`, evaluated once per login from whatever `*ProviderPolicy` is passed in — the natural seam to inject an overlay-adjusted policy, see Decision 7); `api/internal/handlers/config.go:268–269` (existing reserved-name guard, unaffected).

**Alternatives considered**:
- **Pure Helm-only, read-only mappings** (rejected — this was the prior design): Fails SC-007 outright; there is no way to change a mapping without a Helm re-run, contradicting the explicit success criterion.
- **A dedicated new table (e.g. `oidc_role_mappings` or `oidc_role_mapping_overrides`) with one row per role or per group→role rule** (rejected): The existing `"auth"` config row already carries per-provider `RoleMappings` (`registry.go:61–83`) and the helm path already reads that row fresh on every login (point 4 above) — a new table would require its own read path duplicated alongside the existing per-login read of `helmOverride`, and a new migration file, all to store three optional string-list fields that fit trivially as a sibling of `providers` in JSON already being read on every login. It also fragments "the auth config" across two storage locations for no benefit.
- **A per-claim-value schema (a row/entry per individual group→role mapping, e.g. `{group: "gameplane-admins", role: "admin"}`)** (rejected): `computeRole` (Decision 4/Decision 7) already resolves role-tier by list membership, not by walking individual group→role rows; a per-claim-value schema would require a new resolution algorithm to reduce many rows back down to per-role lists before it could even be handed to `computeRole`, duplicating work the existing `RoleMappings{Admin, Operator, Viewer []string}` shape (`registry.go:53–57`) already does for free. It also doesn't match how the Helm-seeded side is expressed (three comma-separated flags, one per role), so the two layers would use structurally different mapping shapes.
- **Pure database-only mappings, no Helm seeding** (rejected): Fails FR-007. A brand-new OIDC-only install has no admin account and no way to reach `/admin/config` to create the first mapping — the chicken-and-egg problem the spec explicitly calls out (spec.md User Story 1: "creating a chicken-and-egg problem for OIDC-only installs"). Seeding from Helm is required precisely so the *first* login already resolves correctly.
- **Helm and database merged into a single config key, DB always wins outright with no seeding relationship** (rejected): Without seeding, this degenerates into the pure-database alternative above and reintroduces the chicken-and-egg problem. Merging into one key also loses the audit-relevant distinction between "operator declared this at install time" and "an admin changed this at runtime" (Decision 9).
- **Helm reasserts on every `helm upgrade`, DB override is only a temporary/soft override** (rejected — this is the shape of the upgrade-semantics question; see Decision 6 for the full rationale). Superficially simpler, but silently undoes admin edits on every chart bump, which is worse than the SC-007 gap this decision exists to close.

---

### Decision 6: Upgrade Semantics — DB Override Wins, With an Explicit Reset-to-Helm-Default Action

**Decision**: On a `helm upgrade` that changes `api.oidc.roleMapping.*` values, **an existing DB override for a role is never clobbered**. A Helm-seeded value is only used to resolve a role when `helmOverride.roleMappings` supplies no (non-nil) list for that role. The dashboard's role-mapping editor provides an explicit **"Reset to Helm default"** action per role. Per Decision 5/M2, there is no per-role row to delete — the action is a `DELETE /admin/config/auth/role-mappings/{role}` request (the one new route added by this feature, Decision 10) that rewrites the same `"auth"` config row, setting `helmOverride.roleMappings.<role>` back to nil/absent for that role only, leaving the other two roles' overrides (if any) untouched. After the reset, the Helm-seeded value (from the *current* running API process's flags) resolves that role again on the next login.

**Rationale**:
- **Why DB-wins-on-upgrade, not Helm-wins-on-upgrade**: The opposite choice — re-applying Helm values over any existing DB override on every `helm upgrade` — was explicitly rejected by the maintainer (H3). An admin who used the dashboard to add or remove a group from a mapping would have that edit silently undone the next time *any* chart value changes, even one unrelated to OIDC (a routine image-tag bump still runs `helm upgrade` and re-renders the operator/API Deployment args). Silent reversion of a deliberate admin action is worse than the alternative of a mapping that "goes stale" relative to Helm values the admin already chose to override.
- **Why an explicit reset action, not automatic reconciliation**: Automatically reconciling would reintroduce exactly the silent-clobber problem above under a different trigger (e.g., "reconcile if the DB row hasn't been touched in N days"). An explicit, admin-initiated action makes "go back to what Helm declares" a deliberate, auditable act (Decision 9) rather than a background side effect of an unrelated upgrade.
- **Consistency with FR-003's PVC-immutability precedent**: The storage-class feature (Decision 2/FR-003) already establishes the pattern that install-time defaults apply going forward, not retroactively, and that reverting requires an explicit action (recreating the PVC) rather than automatic reassertion. The role-mapping upgrade semantics mirror this: a later Helm value is a *new default for what has no override*, not a forced reassertion.
- **No data loss on reset**: Because "reset" only clears one role's key out of `helmOverride.roleMappings` in the shared `"auth"` row (rather than mutating a value in place), the Helm-seeded value is recomputed live from whatever the API process currently has loaded — there is no stale copy to keep in sync, and a subsequent Helm value change is picked up immediately for any role without an override, with no extra plumbing.

**Alternatives considered**:
- **Helm reasserts every DB override on `helm upgrade`** (rejected, H3): Silently undoes admin edits; unacceptable per the maintainer's explicit ruling and inconsistent with SC-007's intent that dashboard edits are durable runtime state, not a value that can be clobbered by an operator's unrelated chart bump.
- **DB override wins forever, no reset path** (rejected): Leaves operators with no way to return to the Helm-declared value short of hand-editing the `"auth"` config row directly in the database (which nothing in this codebase supports doing safely — there is no DB-shell workflow documented anywhere in `docs/`). An explicit UI action (the new DELETE route) is strictly better and costs little.
- **Warn-then-overwrite (dashboard shows a diff and asks the admin to confirm reassertion on next upgrade)** (rejected): Requires the API to detect "this Helm value changed since last seen" across restarts, which means persisting a shadow copy of the last-seen Helm value purely to diff against — extra state for a workflow the explicit reset button already covers with no extra state.

---

### Decision 7: Role Resolution Precedence Chain and Composition with `computeRole`

**Decision**: Role resolution for a given login now has three layers, evaluated in this order:

1. **DB mapping (admin-managed override)** — highest priority.
2. **Helm-seeded mapping** — used for any role that has no DB override.
3. **Configured default role** (`DefaultRole`, from Helm/`--oidc-default-role`) — used only when neither layer's group lists match the user's groups.

Within each of layers 1–2, the existing `computeRole` admin > operator > viewer ordering is preserved unchanged (`api/internal/auth/oidc.go:121–153`, verified): `computeRole` matches groups against `pol.RoleMappings.Admin`, then `.Operator`, then `.Viewer`, returning on the first match — the *first matching role tier*, not the first matching group, wins. **`computeRole` is not modified.** A new, small helper builds the merged policy and `computeRole` is then called exactly once, unmodified, against its output:

```go
// effectiveHelmPolicy merges a helmOverride.roleMappings overlay onto the
// Helm-seeded base policy, per role tier, and returns the merged policy for
// computeRole to evaluate unchanged.
func effectiveHelmPolicy(base *ProviderPolicy, ov *RoleMappings) *ProviderPolicy
```

(Signature only — the body is an implementation detail for the implementer, not this design document.) Per role tier — Admin/Operator/Viewer — the merge is a **list replacement, not a per-claim-value lookup**: if `ov` supplies a non-nil list for that role, that list *replaces* the Helm-seeded list for that role in its entirety; otherwise the Helm-seeded list for that role stands unchanged. An empty-but-non-nil list is a valid override, meaning "no group maps to this role any more" — it is not treated as "no override present." The three roles are resolved independently of one another. `effectiveHelmPolicy`'s result is handed to `computeRole` once; the admin > operator > viewer precedence a reader already knows from `computeRole` continues to be the *only* place that ordering is decided, and the DB/Helm layering is resolved one step earlier, purely as which *group list* feeds each tier.

**Consequence to state explicitly** (an earlier draft of this decision got this backwards): because the per-role merge happens *before* `computeRole` runs, a user who matches an *overridden* `viewer` group **and** a Helm-seeded `admin` group still resolves to **admin** — the most privileged match still wins, exactly as an unmodified two-tier Helm-only policy would have resolved it. Overriding the viewer list does not weaken or bypass the admin tier; it only changes which groups feed the viewer tier.

**Rationale**:
- **`computeRole` is left untouched**: The function already encodes the only precedence ordering the spec requires between *roles* (admin beats operator beats viewer, FR-010/FR-011). Reusing it unmodified — rather than duplicating or reimplementing that ordering inside `effectiveHelmPolicy` — avoids two divergent copies of admin/operator/viewer precedence logic ever existing in the codebase.
- **Per-role (not per-mapping) override granularity**: H5's "first match wins within each layer" is realized here as "the DB layer supplies the group list for a role if present, else Helm's list is used for that role" — a role-by-role merge, not an all-or-nothing swap of the whole mapping set. This lets an admin override just the admin-role mapping (the highest-stakes one) while leaving Helm-seeded operator/viewer mappings alone, matching the "smallest possible admin action" spirit of Decision 9's confirmation-friction design.
- **`groupsClaim` and `defaultRole` are Helm-only, not part of the overlay (M5)**: `helmOverride` carries only `roleMappings`; the default-role fallback (`pol.DefaultRole`, used when no tier matches) and the claim name used to extract groups (`GroupsClaim`) are not overridable through `helmOverride` in v1 — they remain sourced solely from `--oidc-default-role` / `--oidc-groups-claim` (Helm/CLI flags). There is deliberately no `PUT /admin/config/role-mappings/default-role` or equivalent endpoint; any such endpoint is out of scope and must not appear in tasks or scenarios for this feature.

**Alternatives considered**:
- **DB overlay reimplements admin > operator > viewer ordering independently of `computeRole`** (rejected): Duplicate logic that could drift from `computeRole`'s behavior over time (e.g., if `computeRole` gains a new tier or changes DefaultRole handling, the overlay's copy would need a matching, easy-to-forget edit).
- **All-or-nothing override — a DB override for any one role replaces the entire mapping set (all three tiers), including tiers with no override**: (rejected): Forces admins to fully re-specify operator and viewer group lists just to change the admin mapping, and risks an empty/unset tier silently becoming "no group matches" rather than falling back to the Helm value, which is surprising and error-prone.
- **DB overlay evaluated as a fully separate `computeRole` pass, with its result taking precedence over the Helm pass's result** (rejected): Two full `computeRole` evaluations followed by an outer merge is functionally similar to the per-role list-merge for a single admin/operator/viewer mapping, but a user who matches the Helm viewer tier and the DB admin tier would get an ambiguous "which pass wins" question at the *role* level rather than a clean "which list feeds this tier" question at the *group* level; the per-role merge sidesteps the ambiguity entirely since only one merged policy is ever evaluated.

---

### Decision 8: The Synthetic `"helm"` Provider Stays Read-Only; the DB Overlay Is a Separate Consultation, Not an Edit Path

**Decision**: Per H4, the synthetic `"helm"` provider (`HelmProviderName`, `api/internal/auth/registry.go:30`) continues to be synthesized fresh at every API startup from CLI flags, is never persisted to the database, and remains impossible to create, edit, or delete through the dashboard (enforced by the existing reserved-name guard at `api/internal/handlers/config.go:268–269`, unchanged by this feature — see Decision 13). The `helmOverride` field introduced by Decision 5 is **not** a way to edit that provider object. It is a sibling JSON field in the same `"auth"` config row as `providers` — not an entry in the `providers` array, and never written into the provider registry — consulted during role resolution (Decision 7) alongside, not instead of, the `"helm"` provider's Helm-seeded `RoleMappings`.

**Rationale**:
- **Keeps the immutability guarantee legible**: Claim 2 (verified above) states the Helm-configured provider is read-only "structurally." If the hybrid design were described as "the helm provider becomes editable," that claim would need re-verification and the reserved-name guard's purpose would become murky — is `"helm"` reserved because the provider is immutable, or because it can now be edited through a side door? Keeping `helmOverride` conceptually and structurally separate (a distinct top-level field, never merged into or read as a `providers` entry, never written into the provider registry) means the answer stays unambiguous: the provider is still fully read-only; only `helmOverride` is admin-editable.
- **Matches the maintainer's explicit instruction (H4)**: "Do not describe the helm provider as becoming editable." This decision exists specifically to make that instruction durable in the design record, since a future reader skimming Decision 5 in isolation could otherwise conflate "role mappings are now admin-editable" with "the helm provider is now admin-editable."
- **Practical consequence for implementation**: any handler implementing `helmOverride` reads and writes it as a field alongside `providers` in the same `"auth"` row's JSON, with its own validation, and must never let a `helmOverride` write pass through the `providers`-array validation path — so the existing `p.Name == "helm"` guard in `config.go` continues to do exactly what it does today, against exactly the `providers` array, with no new edge case to reason about.

**Alternatives considered**:
- **Model the overlay as a mutable clone of the `"helm"` provider stored in the database** (rejected): Would require either lifting the reserved-name guard for this one special case or inventing a second reserved name, both of which reintroduce the exact ambiguity this decision exists to prevent, and contradict H4 directly.
- **Let admins edit the `"helm"` provider's `RoleMappings` in place, leaving the rest of the provider (issuer, client ID/secret) untouched** (rejected): Would mean "immutable" applies to some fields of the provider but not others, which is a subtler and easier-to-misunderstand invariant than "the provider is fully read-only; a separate overlay supplies overrides."

---

### Decision 9: Audit Requirement for Role-Mapping Overlay Changes (H6)

**Decision**: Every write to `helmOverride` (setting or changing a role's override via `PUT /admin/config/auth`) and every reset of one role's override (via the new `DELETE /admin/config/auth/role-mappings/{role}`, Decision 10) is (a) gated by the existing admin-only RBAC middleware on `/admin/config`-family routes (`api/internal/rbac/rbac.go:170` — the `{segment: "admin", prefix: "/admin/config", perm: "config:manage"}` rule, which prefix-matches any non-GET route under `/admin/config`, including the new reset route with no new rule needed) and (b) recorded synchronously to the audit log via `Auditor.WriteSync` (`api/internal/audit/audit.go:689`), the same convention already used for the FR-014 OIDC role-assignment event and for other admin-configuration mutations (e.g. `api/internal/handlers/capture.go:1152`).

**One consistent pair of `reason` formats, used everywhere this feature writes an audit event** (given `WriteSync`'s signature `ctx, method, path, target, reason string, status int`, with `target` set to `<role>`):

```
set:   oidc role mapping override set: role=<role> groups=<comma_joined_or_none>
reset: oidc role mapping override reset: role=<role>
```

`set` is written by a `PUT /admin/config/auth` request that writes a non-nil `helmOverride.roleMappings.<role>`; `reset` is written by a `DELETE /admin/config/auth/role-mappings/{role}` request. For `set`, `groups=` lists the new effective group list, comma-joined (`none` for a deliberate empty-list override, per Decision 7's "empty is a valid override"). `reset` carries no `groups=` field — there is no new list, the role simply reverts to whatever Helm has seeded. This is in addition to (not a replacement for) the FR-014 per-login role-assignment audit event already covered by N4/Decision 15, which records what happened to an individual *user's* role at login time; this decision covers changes to the *mapping configuration itself*.

**Rationale**:
- **H6 is explicit and unconditional**: "Every create/update/delete of a DB mapping is admin-gated (RBAC) and written to the audit log, following the same WriteSync convention as the FR-014 OIDC role-assignment event." This decision records that requirement precisely, including the correct function signature (verified against `api/internal/audit/audit.go:689` — the earlier draft's implicit assumption of a generic "audit event" needed the precise call site cited so implementers don't invent a different helper).
- **Why config-mutation auditing is distinct from FR-014's per-login auditing**: FR-014 asks for an audit trail of *who got what role and why* at login time — a consequence of the mapping. This decision covers the *cause*: an admin changing what the mapping says. Both matter independently for incident review (e.g., "why did this user become admin" needs the login event; "who changed the mapping that caused it" needs this one) and conflating them into one event type would lose the distinction between an admin action and an automatic login-time computation.
- **Security-sensitive surface**: role-mapping overrides directly control who gets admin access (the exact risk Decision 14/FR-015 already flags for the Helm-seeded case). An unaudited edit path here would be a materially worse gap than the FR-015 warning-and-confirmation UI addresses, since FR-015 only warns at *creation* time — the audit trail is what lets a later reviewer reconstruct *when* and *by whom* a mapping was changed.

**Alternatives considered**:
- **Audit only role-mapping deletions ("reset to Helm default"), not creates/updates** (rejected): H6 says "every create/update/delete," and creates/updates are the higher-risk direction (granting access) compared to resets (which only fall back to an already-Helm-declared, presumably-reviewed value).
- **Asynchronous/best-effort audit write** (rejected): The existing convention for admin-configuration mutations and the FR-014 role-assignment event both use the synchronous `WriteSync` path specifically so a failed audit write is visible to the caller (per `audit.go:684-686`'s documented behavior) rather than silently dropped; deviating to an async path here would be inconsistent with H6's explicit instruction to follow "the same WriteSync convention."

---

### Decision 10: Principle II Consequence — the Editing UI Requires Its Own design.pen Pass

**Decision**: Per H7 and Constitution Principle II, the dashboard's move from a **read-only display** of Helm-seeded role mappings (the pre-hybrid design's scope) to an **editing surface** for `helmOverride` — including the per-role "Reset to Helm default" action from Decision 6 and the FR-015 confirmation flow from Decision 14 — is a visual-surface change and therefore requires its own `design.pen` pass, not an extension of whatever screen the read-only display would have used unchanged.

**HTTP surface backing this UI (M7)**: extends the existing admin auth-config endpoint rather than inventing a parallel resource. The mount point today, verified at `api/internal/handlers/config.go:30`, is:

```go
func MountConfig(r chi.Router, store *db.Store, helmOIDCPresent bool) {
	h := &configHandler{db: store, validators: newValidators(helmOIDCPresent)}
	r.Route("/admin/config", func(r chi.Router) {
		r.Get("/", h.getAll)
		r.Put("/{section}", h.put)
	})
}
```

**`MountConfig` gains an `*audit.Auditor` parameter, and its call site changes.** Decision 9 requires the audit events for `helmOverride` set/reset to be written from inside `config.go`'s handlers, and the handler currently has no auditor to write with. The in-tree precedent for injecting one into a `Mount*` constructor is `MountCapture(r chi.Router, reg *kube.Registry, auditor *audit.Auditor, cfg CaptureConfig, agentCABundle, agentClientCert, agentClientKey string)` (`api/internal/handlers/capture.go:55`, verified). `MountConfig` follows the same pattern:

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

Its call site, `api/cmd/main.go:245` (`handlers.MountConfig(p, store, oidcAuth != nil)`, verified — currently passes no auditor), changes to `handlers.MountConfig(p, store, auditor, oidcAuth != nil)`. Neither the signature nor the call site is unchanged by this feature — both must be edited as part of implementing Decision 9's audit requirement.

- Editing an override is **not** a new endpoint: it rides the existing `PUT /admin/config/auth` request body as the `helmOverride` field alongside `providers` (Decision 5/M2). `validateAuth` (`config.go:235`) gains validation for the new field; no new route.
- **Exactly one new route** is added, for reset (M7): `DELETE /admin/config/auth/role-mappings/{role}`, mounted inside the same `r.Route("/admin/config", ...)` block so it inherits the identical RBAC middleware chain (`sessions.Authenticate` → `mutationRateLimit` → `rbac.Middleware(reg)`, wired in `api/cmd/main.go:228–245`) and the same `{segment: "admin", prefix: "/admin/config", perm: "config:manage"}` rule (`rbac.go:170`) that already gates `PUT /admin/config/{section}` — no new RBAC rule is needed because the rule is prefix-matched, not route-exact.
- **Provenance is key-presence only — there is no separate source field.** The `GET /admin/config` response already returns `helmOverride.roleMappings` verbatim (Decision 5's JSON shape). The dashboard derives provenance for a role purely from whether that role's key is present in `helmOverride.roleMappings`: present means overridden (DB-sourced), absent means the Helm seed applies. No `source`/`origin` field of any kind (e.g. a `auth.effective.roleMappings[<role>].source` string) is added to the response, and "Reset to Helm default" is offered for a role exactly when its key is present in `helmOverride.roleMappings`.

**Concretely, this means**:
1. The `design.pen` update (via the `pencil` MCP server) must model the *editing* affordances explicitly — per-role edit controls for `helmOverride.roleMappings`, the reset-to-Helm-default action, and the admin-role confirmation step — not just the read-only list the earlier scope implied.
2. Touched nodes get re-exported to `design-export/json/<id>.json` and `design-export/screenshots/<id>.png` in the same change, per the repo's standing re-export rule (CLAUDE.md rule 1) — this is a superset of, not a replacement for, the read-only display work already anticipated for the "Implementation & Design Requirements" section below.
3. This is a genuinely new design surface, not a cosmetic tweak to an existing one — under the repo's design-first rule, code implementing the `helmOverride` editing UI must not be written ahead of (or instead of) this Pencil pass.

**Rationale**:
- **H7 is explicit**: "The dashboard gains an EDITING surface, not just the read-only display. Per constitution Principle II this REQUIRES its own design.pen pass plus a design-export re-export." This decision exists to make sure the implementation plan doesn't treat the pre-hybrid read-only-display design work (already noted in "Implementation & Design Requirements" below) as sufficient for the hybrid's larger scope.
- **Editing surfaces carry materially more design risk than read-only displays**: confirmation dialogs, inline validation, destructive actions (delete/reset), and the FR-015 admin-mapping friction step all need their interaction design decided in Pencil, per the repo's design-first convention (CLAUDE.md rule 1) — these are exactly the kind of decisions that get reverted when made code-first.

**Alternatives considered**:
- **Treat the editing UI as a minor extension of the already-planned read-only display, skip a fresh Pencil pass** (rejected): Contradicts H7 directly, and understates the design surface — a read-only list and an edit-with-confirmation editor are different screens with different interaction patterns, not the same screen with a button added.

---

### Decision 11: Group Claim Name Configurability

**Decision**:
**Fully configurable per provider.** The group claim name is specified via `Provider.GroupsClaim` (a string field in the provider config). If empty, it defaults to `"groups"`. Admins set this via `/admin/config` → `auth` → `providers[i].groupsClaim` for database-managed providers. For the Helm-configured provider, operators set it via Helm values (e.g., `api.oidc.groupsClaim` per N1).

**Rationale**:
- **OIDC providers use different claim names**: Okta uses `"groups"`, Azure AD uses `"roles"`, some custom providers use `"membership"`. Gameplane cannot assume a single claim name; it must be configurable.
- **Existing implementation supports this**: `api/internal/auth/oidc.go:250–252` reads `o.policy.GroupsClaim` and passes it to `extractGroups()`. The logic is already in place.
- **Validation already present**: `api/internal/handlers/config.go:203–208` validates that `groupsClaim` is not blank if set. No additional validation is needed.
- **Default is sensible**: "groups" is the most common claim name and matches OIDC standards. Operators who do not specify a custom claim name get this default automatically.
- **Unaffected by the hybrid decision (Decision 5)**: the DB overlay only overrides group→role mapping lists; it does not touch which claim is read to produce a user's group list in the first place. `GroupsClaim` stays a Helm/provider-level setting, not something the overlay layers on top of.

**Alternatives considered**:
- **Hardcoded claim name** (rejected): Would require re-coding per OIDC provider, defeating the purpose of a configurable OIDC flow.
- **Enumerated list of "approved" claim names** (rejected): Too restrictive; custom OIDC providers and organizational configurations use non-standard claim names.

---

### Decision 12: Role Re-Evaluation on Login and Interaction with Manually-Assigned Roles (Updated for the DB Overlay)

**Decision**:

**Role re-evaluation occurs if and only if the effective policy for the login — Helm-seeded mappings merged with any DB overlay per Decision 7 — has RoleMappings configured**, i.e. `syncRole := o.policy != nil && o.policy.RoleMappings != nil` (verified unchanged at `api/internal/auth/oidc.go:262`). Because the DB overlay is merged into the effective `*ProviderPolicy` *before* this check runs (Decision 7), the presence of a DB-only override (with no Helm mapping configured at all, e.g. an install that shipped with `api.oidc.roleMapping.*` unset but an admin has since added overrides through the dashboard) is enough to make `RoleMappings` non-nil and turn re-evaluation on for that login, exactly as a Helm-seeded mapping would.

- **If the effective policy has RoleMappings** (Helm-seeded, DB-overridden, or a mix): On each login, the user's OIDC token groups are compared against the merged mappings (Decision 7). The user's role is updated to match. The demotion guard in `syncUserRole` (`api/internal/auth/oidc.go:374–411`, verified) is unchanged by this feature: if the new role would strip the install's last user capable of managing users (checked via `RoleGrantsUserManagement` → `UserManagesUsers` → `UserManagerCount`), the update is skipped (`applied=false`, no error) and the previously stored role is retained.
- **If the effective policy has no RoleMappings at all** (no Helm seed and no DB override exists for any role): the user's role is assigned once at first login (based on `DefaultRole`) and never re-evaluated on subsequent logins, exactly as before this feature. Manually-assigned roles (via admin grant or bootstrap-admin) persist indefinitely in this case.
- **A DB override interacting with a manually-granted (bootstrap-admin or admin-panel-granted) role**: because `syncUserRole` only ever fires when `syncRole` is true (i.e., some mapping — Helm or DB — exists) and it unconditionally re-points `users.role` to the resolved role subject only to the demotion guard, a manually-granted role on a user who also matches a DB-overridden or Helm-seeded mapping is **not** durable across that user's next OIDC login — the mapping (whichever layer supplied the winning group list) wins, same as it already does for Helm-only mappings today. The demotion guard is the only protection against this for the specific case of "removing the last user-manager"; it does not generally protect a manually-set role from being overwritten by a matching mapping. This is unchanged behavior, now explicitly also true when the winning mapping came from a DB override rather than Helm.

**Rationale**:
- **Explicit intent, extended consistently to the DB layer**: the original principle — "if mappings are configured, role assignment is automatic and dynamic; if not, it's manual and static" — extends naturally once "configured" means "configured in either layer." An admin who adds a DB override to a previously-mapping-free install is *choosing* to turn on dynamic role sync for that role, and should expect the same re-evaluation-and-demotion-guard behavior a Helm-seeded mapping would have given them; treating DB overrides as somehow "softer" than Helm mappings (e.g., not triggering `syncRole`) would create a surprising asymmetry between the two layers Decision 5 otherwise treats as interchangeable inputs to the same `computeRole` call.
- **Demotion guard is unaffected and still fulfills its original purpose**: nothing in the hybrid design changes `syncUserRole`'s lockout protection, and it should not — the guard exists to prevent a *mapping* mistake (in either layer) from locking out the last admin, and a DB-override mistake is exactly as capable of doing that as a Helm-mapping mistake. Verified against the actual guard logic at `oidc.go:374–393`: it checks the *target* role's `RoleGrantsUserManagement`, then (only if the target does not grant management) whether the user currently manages users and whether they're the last one — this logic has no dependency on which layer produced the new role, so it applies identically regardless of whether the demotion was computed from a Helm-seeded or DB-overridden mapping.
- **Bootstrap-admin coexistence (FR-013) still holds**: a bootstrap-admin-created *local* account (not linked to any `oidc_links` row) is never touched by `resolveOrLinkUser`/`syncUserRole` at all — those functions only run for OIDC logins matched by `(issuer, subject)`. The DB overlay changes nothing about that separation; bootstrap-admin remains fully isolated from both the Helm and DB mapping layers.

**Alternatives considered**:
- **DB overrides do not trigger `syncRole` (treated as "advisory" unless a Helm mapping also exists)** (rejected): Would mean an admin who overrides just the admin-role mapping via the dashboard on an install with no Helm mappings at all gets no re-evaluation — the override would be silently inert for any user who doesn't already have a matching Helm mapping, directly undermining SC-007's "changes taking effect for the next login attempt."
- **Always re-evaluate on login regardless of whether any mapping is configured** (rejected, unchanged from the prior version of this decision): Violates the principle of explicit intent and would clobber manually-assigned roles on installs that deliberately have no mappings configured at all.
- **Never re-evaluate on login** (rejected, unchanged): Violates FR-011.
- **Give DB overrides their own separate demotion guard, independent of `syncUserRole`'s existing one** (rejected): Unnecessary duplication — the existing guard already operates on the *resolved* role regardless of which layer produced it, so a second guard would either be redundant or, worse, could disagree with the first under some edge case neither was designed against.

---

### Decision 13: Reserved Provider Name Guard for "helm"

**Decision**:
**Enforce at the validation layer** in `api/internal/handlers/config.go:268–269`. When a dashboard user or API client attempts to create or update a provider with `Name=="helm"`, the API returns a validation error: `"providers[i].name 'helm' is reserved for the Helm-configured provider"`.

The validation occurs:
1. On provider creation via `/admin/config` PUT (handlers/config.go validateAuth)
2. On provider update (same validation)
3. Never on the synthetic Helm provider itself (it is not stored in the database and cannot be mutated)

This guard is **unaffected** by the Decision 5 hybrid design: it protects the `providers` namespace, and the DB role-mapping overlay is deliberately kept out of that namespace (Decision 8), so there is no new interaction to reason about here.

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

### Decision 14: Over-Broad Mapping Warning (FR-015) — Now Also Covers DB Overlay Edits

**Decision**:
**No backend validation.** Instead, provide a **UI-level warning and explicit confirmation** in the admin configuration interface (`web/src/routes/AdminSettings.tsx`), specifically when editing OIDC provider role mappings — and, per Decision 5/Decision 10, this now includes editing `helmOverride.roleMappings`, not only viewing the Helm-seeded mappings the pre-hybrid design assumed were read-only.

**Mechanism**:

1. **Advisory Callout** (rendered unconditionally for every mapping, Helm-seeded or DB-overridden):
   When an admin views or edits an OIDC role mapping rule (e.g., "group 'employees' → admin role"), display an informational callout:

   > "Warning: This mapping grants [role] to any user in the '[group]' group. If this group has many members, verify that you intend to grant this role to all of them. Gameplane cannot enumerate group membership from the OIDC provider."

2. **Explicit Confirmation Step** (when target role is "admin"):
   If the admin is saving a `helmOverride.roleMappings` change via `PUT /admin/config/auth` whose target role is `"admin"`, show an additional confirmation dialog or checkbox before the save completes:

   > "Confirm: This mapping will grant admin access to all users in '[group]'. This cannot be revoked automatically. Click 'I understand' to proceed, or edit the mapping."

   The mapping is NOT saved until the admin explicitly confirms. This satisfies SC-007 (runtime mutation safety) together with the audit trail from Decision 9.

3. **Non-blocking for non-admin roles**:
   For operator and viewer roles, the advisory callout is shown, but no explicit confirmation is required (the admin can save directly).

4. **Reset-to-Helm-default action (Decision 6) is exempt from the confirmation step**: resetting clears the role's key from `helmOverride.roleMappings` and falls back to a Helm-seeded value an operator already declared at install time; it is not a *new* grant of access requiring the same friction as creating or widening an admin mapping. It is still audited (Decision 9).

**Rationale**:
- **Gameplane cannot enumerate IdP groups**: OIDC tokens carry the claims issued by the provider; Gameplane does not query the IdP's group directory. It cannot determine how many users are in a group.
- **Trust boundary**: The OIDC provider is authoritative. If an operator misconfigures a mapping, that is an operator error, not a Gameplane bug. Gameplane can warn and require confirmation, but cannot enforce.
- **FR-015 is satisfied for both layers**: The spec asks for "validation or confirmation when a single mapping covers a non-trivial number of users," without distinguishing Helm-seeded from DB-overridden mappings. Since the hybrid design now lets admins create or widen admin-role mappings directly through the dashboard (not just view Helm-seeded ones), the confirmation step is now load-bearing in a way it was not under the read-only pre-hybrid design — this is precisely why Decision 10 requires a fresh design pass rather than reusing read-only-display mockups.
- **Proportionate friction**: Confirmation is required only for the most dangerous role (admin) and only for grants (create/widen), not for the reset action, which can only narrow access back to an already-declared Helm default.

**Delivery**: This is implemented as its own slice in the dashboard implementation, separate from core role-mapping functionality.

**Alternatives considered**:
- **Query the OIDC provider's group API** (rejected): Out-of-band IdP queries introduce latency, flakiness, and trust boundary issues. Some IdPs don't expose group membership queries; others require elevated credentials.
- **Reject mappings with "suspicious" group names** (rejected): Arbitrary heuristics (e.g., "reject if the group name contains 'org'" or "employees") are unreliable and culturally insensitive.
- **Advisory only, no confirmation** (rejected): Does not fully satisfy FR-015's call for confirmation. An explicit checkpoint is more effective than a passive warning.
- **Require the same confirmation step for the reset-to-default action** (rejected): Reset only narrows access back to an operator-declared default; treating it with the same friction as an admin-role grant would train admins to click through confirmations reflexively, weakening the signal for the cases that actually matter.

---

### Decision 15: Test Strategy and E2E Bucket Assignments (Updated for the DB Overlay)

**Decision**:

**Unit/Envtest Tier**:
- Unit tests for role-mapping logic (`api/internal/auth/oidc_rolemap_test.go`): Extend existing `TestComputeRole()` with new cases for Helm provider configuration, "deny" default role, and edge cases. `computeRole` itself is not modified, so its existing test cases are untouched.
- **New**: unit tests for `effectiveHelmPolicy` (Decision 7's merge helper: `effectiveHelmPolicy(base *ProviderPolicy, ov *RoleMappings) *ProviderPolicy`), covering: an override for one role only (others fall back to Helm), an override present but Helm has no mapping at all for that install, a deliberate empty-but-non-nil override list, and the M4 consequence case (override narrows `viewer` while a Helm-seeded `admin` group still wins after merge + `computeRole`).
- **New**: handler tests for `PUT /admin/config/auth` writing `helmOverride` and for the new `DELETE /admin/config/auth/role-mappings/{role}` reset route, covering the admin RBAC gate (H6, same `config:manage` rule as the existing `PUT /admin/config/{section}`) and the namespace separation from `providers` (Decision 8) — a `helmOverride` write must not be reachable through, or interpreted as, the `providers` validation path guarded at `config.go:268–269`.
- New envtest file (`operator/internal/controller/gameserver_storage_envtest_test.go`): Test PVC creation with explicit/template/install-time storage class settings and error cases (nonexistent StorageClass).

**E2E Tier** (in CI kind clusters):
- **`api-auth` bucket**: Add OIDC tests: `TestAPI_OIDCRoleMappingAtInstallTime`, `TestAPI_OIDCRoleMappingFirstLogin`, `TestAPI_OIDCRoleMappingReEvalOnLogin`, plus **new**: `TestAPI_OIDCRoleMappingOverrideWinsOverHelm` (set a Helm mapping, `PUT` a conflicting `helmOverride`, verify the override's role wins on next login) and `TestAPI_OIDCRoleMappingResetToHelmDefault` (verify `DELETE /admin/config/auth/role-mappings/{role}` clears the override and the Helm value resolves again on next login). Each test uses a fake OIDC issuer (in-process, following the pattern from `oidc_rolemap_test.go`). Budget the added logins against the api-auth bucket's per-user rate limit (burst 6, 3/min) alongside the pre-existing ~7 admin logins already budgeted for that bucket (CLAUDE.md e2e conventions) — the two new tests add roughly 2 more logins, which must be accounted for when the bucket's total is next measured.
- **`operator` bucket**: Add 3 storage class tests: `TestGameServer_StorageClassFromHelmDefault`, `TestGameServer_StorageClassNotFound`, `TestGameServer_StorageClassExplicitOverride`. All parallel-safe, zero login pressure.

**OIDC E2E feasibility**:
- **VERIFIED as feasible**: The `newFakeIDP()` pattern from `api/internal/auth/oidc_rolemap_test.go:135` can be ported to e2e. A fake OIDC issuer running in-process is the proven approach; there is NO shared test IdP in CI, and tests deliberately use an unreachable issuer (`https://e2e-idp.invalid`) for listing/routing without dialing (api_auth_e2e_test.go:278–279).

**Audit event testing**: Audit events record OIDC role assignments per N4, with reason format: `oidc role assigned: provider=<name> matched=<claimValue|none> from=<oldRole> to=<newRole>`. Tests verify the event is recorded correctly in the audit_events table with columns: id, ts, actor, method, path, target, status, ip, reason. **New (Decision 9)**: a separate set of tests verifies the *mapping-mutation* audit events (`helmOverride` set via `PUT`, reset via the new `DELETE`) are recorded via `WriteSync` with the same column set and the single consistent `reason` format from Decision 9, distinct from the per-login role-assignment events above — asserting both event *kinds* can coexist in `audit_events` for the same role/user pair without one overwriting or being confused with the other.

**Rationale**:
- **No external IdP dependency**: A fake issuer keeps tests isolated and deterministic. External providers (Okta, Keycloak) introduce flakiness and are out-of-scope.
- **Login budget constraint**: Each test in the api-auth bucket consumes part of a per-user rate limit (3/min, burst 6). The override and reset tests add real login pressure (each needs at least one login to observe the resolved role) and must be counted against the bucket's existing budget, not assumed free.
- **Parallel safety**: Storage class tests are pure reconciliation with zero admin UI involvement; they run in parallel and do not contend for resources. OIDC tests in api-auth run either sequentially or re-use shared sessions to avoid rate-limit exhaustion. The new `helmOverride` handler tests (`PUT`/`DELETE`) are pure API calls under an already-authenticated admin session and add RBAC-gated HTTP calls, not additional logins, so they do not add login pressure beyond the sessions already budgeted.
- **Coverage impact**: New unit tests add coverage to api/auth and `api/internal/handlers/config.go` without triggering rebaselines (api is at 80%).

**Alternatives considered**:
- **Skip OIDC e2e testing** (rejected): Does not verify that a real login works end-to-end. Unit tests alone are insufficient for a user-facing login flow.
- **Use a real external IdP** (rejected): Introduces CI flakiness, trust boundaries issues, and test isolation violations.
- **All tests in one bucket** (rejected): Risks exceeding the api-auth bucket's login budget once the override and reset tests' logins are added on top of the pre-existing ~7.
- **Skip the mapping-mutation audit tests since the per-login role-assignment audit tests already cover "audit events work for OIDC"** (rejected): The two event kinds are written by different code paths (a login handler vs. an admin-config handler) and asserting one says nothing about the other; H6 is explicit that overlay mutations themselves must be audited, so that path needs its own test.

---

## Implementation & Design Requirements

### Pencil Design Pass (Constitutional Principle II)

This feature includes a **visual-surface change** to the dashboard, and per the hybrid decision (Decision 5, Decision 10 / H7) that surface is larger than originally scoped: not just a read-only display of install-time settings, but an **editing surface** for `helmOverride.roleMappings` (per-role edit, the "Reset to Helm default" action, and the FR-015 admin-mapping confirmation flow), alongside the read-only display of the storage-class default. Per Constitution Principle II, this requires:

1. A **design.pen pass** (first step of the dashboard implementation slice):
   - Update `design.pen` via the Pencil MCP server to add/revise the AdminSettings install-time settings section (storage class default, read-only) **and** the OIDC role-mapping editing surface — per-role `helmOverride` edit controls, the reset-to-Helm-default action, and the admin-mapping confirmation flow (Decision 10).
   - Export the touched nodes to `design-export/json/<id>.json` and `design-export/screenshots/<id>.png`.
2. **Touch points**: Includes `web/specs.md` (specifications for the dashboard component).
3. Per Decision 10, this is a genuinely new design pass for the editing affordances — it must not be treated as a minor extension of a read-only-display mockup.

---

## Conclusion

These 15 design decisions establish a coherent path forward for install-time configuration. The decisions are grounded in codebase review and follow Gameplane's existing conventions:

1. Helm value key and operator flag naming mirror existing storage-class patterns (api.storage). Canonical names per N1/N2/N3: `api.oidc.*` (not api.auth.oidc), `operator.gameDataStorage.storageClassName`, flags `--oidc-*` and `--game-data-storage-class`.
2. PVC precedence is explicit and backward-compatible.
3. No CRD changes are required; existing fields suffice.
4. StorageClass error detection reuses proven MetalLB validation patterns.
5. OIDC role mapping is **hybrid**: Helm seeds initial mappings (satisfying FR-007/SC-003/SC-004, no bootstrap-admin needed), and a `helmOverride` field — a sibling of `providers` in the *same* existing `"auth"` config row (`registry.go:175–185`) — lets admins override them at runtime (satisfying SC-007, no Helm/API restart needed). No new table, no new migration file. This supersedes the earlier Helm-only, read-only resolution, which failed SC-007.
6. Upgrade semantics: a `helmOverride` entry for a role always wins over a changed Helm value for that role; an explicit "Reset to Helm default" action (the new `DELETE /admin/config/auth/role-mappings/{role}` route) is the only way back, avoiding silent reversion of admin edits on routine `helm upgrade`s.
7. Precedence composes per-role (the override's group list if present, else Helm's) via the new `effectiveHelmPolicy` helper into a single effective policy, then defers entirely to the existing, unmodified `computeRole` admin > operator > viewer ordering — no duplicate precedence logic. Because the merge happens before `computeRole` runs, the most privileged match still wins even when a lower tier was the one overridden.
8. The synthetic `"helm"` provider stays fully read-only and un-editable; `helmOverride` is a structurally separate sibling field, never merged into `providers` and never a write path into the provider registry.
9. Every `helmOverride` mutation (set via the existing `PUT /admin/config/auth`, reset via the new `DELETE`) is admin-RBAC-gated (the existing `config:manage` prefix rule) and synchronously audited via `Auditor.WriteSync`, using one consistent `reason` format, distinct from (and in addition to) the existing per-login role-assignment audit event.
10. The `helmOverride` editing UI is a new visual surface requiring its own `design.pen` pass and design-export re-export per Constitutional Principle II — it is not covered by a read-only-display mockup.
11. Group claim names and the default role are Helm-only in v1 (M5) — already configurable via Helm/CLI flags, no code changes needed there, and neither is part of `helmOverride`.
12. Role re-evaluation is controlled by whether the *effective* (Helm-merged-with-override) policy has any RoleMappings, with the existing demotion guard in `syncUserRole` applying unchanged regardless of which layer produced the winning mapping.
13. The reserved "helm" provider name is guarded at the validation layer, unaffected by and cleanly separated from `helmOverride` (Decision 8).
14. Over-broad mapping risks are communicated via UI warnings with explicit confirmation for admin mappings, now covering `helmOverride` edits as the primary editing surface, with the reset action deliberately exempted from the confirmation step.
15. Tests are feasible with fake OIDC issuers and fit within existing CI bucket constraints, extended to cover override-vs-Helm precedence via `effectiveHelmPolicy`, the reset route, and mapping-mutation audit events distinct from per-login role-assignment audit events.
</content>
