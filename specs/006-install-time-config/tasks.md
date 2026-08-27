---
description: "Task list for install-time configuration feature (storage class & OIDC role mapping)"
---

# Tasks: Install-Time Configuration (Storage Class & OIDC Role Mapping)

**Input**: Design documents from `/specs/006-install-time-config/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), data-model.md, contracts/

**Tests**: All phases include test tasks (unit, envtest, vitest, e2e). Tests are REQUIRED per Constitution Principle I.

**Organization**: Tasks are grouped by phase (Setup → Foundational → User Stories in priority order → Polish).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel — the task's files have no conflict with any other task scheduled alongside it. This is **not** a claim of zero dependencies: a [P] task may still need an earlier task's output to exist (e.g. a route it calls, a struct field it reads) as long as it isn't editing the same file that earlier task edited. Check the Dependencies notes for real ordering; the [P] tag only answers "can two agents/developers touch these files at the same time without merge conflicts."
- **[Story]**: Which user story this task belongs to (e.g., US1, US2) — ONLY for story phases, not setup/foundational/polish
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Helm values and template structure shared by both storage class (US2) and OIDC (US1) stories.

**⚠️ CRITICAL**: These infrastructure tasks MUST complete before any story work begins.

- [X] T001 [P] Add Helm values keys for operator.gameDataStorage.storageClassName and OIDC role-mapping configuration (oidc.groupsClaim, oidc.defaultRole, oidc.roleMapping.admin/operator/viewer) in charts/gameplane/values.yaml
- [X] T002 [P] Add conditional --game-data-storage-class flag binding to operator Deployment args in charts/gameplane/templates/operator.yaml
- [X] T003 [P] Add conditional --oidc-* and --game-data-storage-class flag binding to api Deployment args in charts/gameplane/templates/api.yaml

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: CLI flag definitions in operator and API. These enable story-specific business logic to read Helm-configured values.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T004 [P] Add --game-data-storage-class CLI flag with GAMEPLANE_GAME_DATA_STORAGE_CLASS env fallback and GameServerReconciler.DefaultStorageClassName struct field in operator/cmd/main.go
- [X] T005 [P] Add --oidc-groups-claim, --oidc-default-role, --oidc-role-mapping-admin, --oidc-role-mapping-operator, and --oidc-role-mapping-viewer CLI flags (each a single comma-separated string, not a repeatable flag) with GAMEPLANE_OIDC_* env fallbacks to api/cmd/main.go serve subcommand

**Checkpoint**: Foundation ready — user story implementation can now begin in parallel.

---

## Phase 3: User Story 1 - OIDC Role Mappings at Install Time (Priority: P1) 🎯 MVP

**Goal**: Enable operators to deploy Gameplane with OIDC-only authentication and have users assigned correct roles on first login via Helm values, without running bootstrap-admin. Per the maintainer's hybrid resolution of the FR-007/SC-007 tension (settled mechanism, see plan.md Summary and data-model.md's "MAINTAINER DECISION" section), admins can then manage those role mappings' per-role group lists through the dashboard — via the existing `PUT /admin/config/auth` and a new reset route — with the change taking effect on the next login and no API restart or `helm upgrade` required.

**Independent Test Criteria**:

A fresh Gameplane install with OIDC enabled and role mappings pre-configured (via Helm values) allows:
1. A user whose OIDC group matches an admin mapping to log in and immediately receive admin role.
2. A user whose OIDC group does not match any mapping to receive the default role (viewer).
3. On subsequent logins, users' roles to be re-evaluated against current mappings if their OIDC group membership changes.
4. An admin to view the configured storage class and OIDC role mappings in the dashboard config interface (read-only display).
5. A clear warning and confirmation step when configuring admin-role OIDC mappings to prevent accidental over-broad grants.
6. All role assignments and re-evaluations to be recorded in the audit log with user name, new role, and mapping rule that triggered assignment.
7. An admin to set, change, or reset (to the Helm-seeded default) a role's override group list through the dashboard, with the change taking effect on the next login attempt — no API restart, no `helm upgrade` (SC-007) — and every such mutation recorded in the audit log.

### Slice 4: OIDC Helm Provider Synthesis & Flag Processing

- [X] T006 [US1] Add validation and error handling for OIDC flag values (defined in T005) in api/cmd/main.go: reject invalid --oidc-default-role values (must be 'admin', 'operator', or 'viewer'), reject empty entries in comma-separated role mapping lists
- [X] T007 [P] [US1] Synthesize read-only Helm-configured OIDC provider ("helm") in api/internal/auth/registry.go using the parsed policy from CLI flags; provider is immutable through dashboard
- [X] T008 [US1] Implement groups-claim extraction and first-match role assignment, plus per-login re-evaluation against the Helm-synthesized policy, in api/internal/auth/oidc.go. Must honour the configured claim name (policy.GroupsClaim — never hardcode "groups") for both first-login assignment and every subsequent re-evaluation. Re-evaluation MUST NOT silently demote a locally-granted role: per research.md Decision 7, re-evaluation runs only when the provider's RoleMappings are configured, and the update must be skipped (retaining the prior role, applied=false) when it would remove the last user capable of managing users. Sequential with T009 — both edit this file, and T009's audit event depends on the assignment/re-evaluation outcome this task produces.
- [X] T009 [US1] Thread audit recorder through OIDC login path (depends on T008 landing first; same file) and emit FR-014 audit events on role assignment (first login and role re-evaluation) with format: user name, new role, mapping rule trigger in api/internal/auth/oidc.go
- [X] T010 [P] [US1] Add unit test cases for OIDC flag parsing, role claim extraction, Helm provider synthesis, audit event format, and role re-evaluation logic, including a case asserting a bootstrap-admin local account and an OIDC-mapped admin coexist without either clobbering the other (FR-013), in api/internal/auth/oidc_rolemap_test.go

### Slice 5: `helmOverride` Overlay for Admin-Managed OIDC Role Mapping Overrides (Settled Hybrid Design — no new table, no migration)

- [X] T011 [US1] Add the optional `helmOverride.roleMappings` sibling field to the "auth" config row's JSON: extend the `authCfg`/`authProvider`-adjacent struct and `validateAuth` in api/internal/handlers/config.go so `PUT /admin/config/auth` accepts, canonicalizes, and persists it (each present role list — `admin`/`operator`/`viewer`, independently optional, `[]` meaning "nobody" — validated for non-blank elements with the same rule `validateProviderMapping` already applies to `providers[*].roleMappings[*]`; present-but-empty `helmOverride`/`helmOverride.roleMappings` is accepted as equivalent to absent); `helmOverride` is never a `Provider` entry and the existing `providers[].name == "helm"` reserved-name guard is unchanged. `GET /admin/config` needs no code change to surface it — `getAll()` already returns each section's stored row verbatim, so a role key's presence (including `[]`) in the persisted `helmOverride.roleMappings` is itself the provenance signal the dashboard reads (no separate `source` field is added). Also extend the JSON parsing of the "auth" config row in api/internal/auth/registry.go so the OIDC login path (T012) can read the current `helmOverride.roleMappings` value. Depends on T007 (same file, registry.go — T007's Helm-provider synthesis must land first).
- [X] T012 [US1] Implement `effectiveHelmPolicy(base *ProviderPolicy, ov *RoleMappings) *ProviderPolicy` in api/internal/auth/oidc.go — per-role list replacement: for each of admin/operator/viewer independently, a non-nil override list replaces the Helm-seeded list for that role in full (an empty non-nil list means "nobody maps to this role"), an absent/nil key leaves the Helm-seeded list standing. Wire it into the OIDC login path for the synthetic "helm" provider so the override is read at LOGIN TIME, not cached: `Registry.OIDCFor` short-circuits on `name == HelmProviderName` and returns `r.legacy` immediately, before `snapshot()`, before the row hash, before the row-hash cache (api/internal/auth/registry.go:224-232) — that hash-based cache invalidation governs the DB-backed providers resolved further down `OIDCFor` only, never the helm path. `r.legacy` is a single `*OIDC` built once at API startup (api/cmd/main.go:127) and held for the process lifetime, so no cache entry exists to invalidate for it and no new caching layer is needed or should be added. Because `registry.go:152-153` already calls `legacy.AttachStore(store)`, the helm provider already has the store; role resolution for it must therefore read the current `helmOverride.roleMappings` straight from that store (via T011's registry.go plumbing) on every login attempt, build the effective policy with `effectiveHelmPolicy`, then call the existing, UNMODIFIED `computeRole` once against it. This per-login read — not any cache — is what delivers SC-007: an admin's edit lands on the very next login attempt with no API restart and no `helm upgrade`. `computeRole`'s admin > operator > viewer first-match tie-break and its existing tests are untouched, so a user who matches an overridden viewer group *and* a Helm-seeded admin group still resolves to admin (the most privileged match wins after the merge, not before it). Sequential with T008/T009 (same file, both must land first) and depends on T011 (the override data it merges in).
- [X] T013 [US1] Implement `DELETE /admin/config/auth/role-mappings/{role}` in api/internal/handlers/config.go — the one new route this feature adds — mounted inside the existing `r.Route("/admin/config", ...)` block in `MountConfig` alongside `GET /` and `PUT /{section}`. `MountConfig` gains an `*audit.Auditor` parameter (its signature becomes `MountConfig(r chi.Router, store *db.Store, auditor *audit.Auditor, helmOIDCPresent bool)`) so T014's audit events can be written from inside config.go — precedent: `MountCapture(r chi.Router, reg *kube.Registry, auditor *audit.Auditor, ...)` in api/internal/handlers/capture.go:55. Its call site in api/cmd/main.go:245 (`handlers.MountConfig(p, store, oidcAuth != nil)`) changes accordingly to pass `auditor`. `{role}` must be one of `admin`, `operator`, `viewer` (400 Bad Request on any other value). Removes that one role's key from `helmOverride.roleMappings` in the "auth" row, leaving the other two roles' overrides (if any) untouched, and re-persists the row; idempotent (200 with no change if the role had no override). Response is the updated "auth" section body, same `{"section":...,"value":...}` envelope as `put`. Inherits the existing `config:manage` RBAC gate already covering the `/admin/config` prefix (api/internal/rbac/rbac.go:170) — no new rbac.go rule. Sequential with T011 (same file, config.go); also touches api/cmd/main.go (same file as T006/T017 — all three must be reconciled).
- [X] T014 [US1] Thread `Auditor.WriteSync` audit events into (a) every `PUT /admin/config/auth` that changes `helmOverride` and (b) the new `DELETE /admin/config/auth/role-mappings/{role}` reset route, in api/internal/handlers/config.go, using one consistent reason-string format for both: `"oidc role mapping override set: role=<role> groups=<comma_joined_or_none>"` for a set/change (comma-joined new list, or the literal `none` for an empty-list override) and `"oidc role mapping override reset: role=<role>"` for a reset. A single `PUT` that changes more than one role's override emits one audit row per changed role (`target` is always exactly one role). Sequential with T013 (same file).
- [X] T015 [US1] Extend api/internal/auth/oidc_rolemap_test.go (same file as T010) with unit test cases for `effectiveHelmPolicy` — a non-nil override list replaces the Helm-seeded list for that role; an absent/nil key leaves the Helm-seeded list standing; an empty non-nil list means "nobody"; the three roles resolve independently; the most-privileged-match-still-wins case (an overridden viewer group + a Helm-seeded admin group still resolves to admin because `computeRole` runs on the merged policy); and the upgrade-does-not-clobber case (a changed Helm-seeded list for a role does not overwrite that role's existing override). Also add a case proving `effectiveHelmPolicy(base, nil)` returns `base` unchanged (byte-identical to calling `computeRole` directly, pre-feature — SC-008). Depends on T010 (existing test cases) and T012 (function under test).
- [X] T016 [P] [US1] Add envtest cases for the extended "auth" config endpoint and the new reset route — RBAC gate (an actor lacking `config:manage` rejected on both the `PUT` and the `DELETE`), `PUT /admin/config/auth` persisting a `helmOverride` change and the subsequent `GET /admin/config` reflecting it via role-key presence/absence with no separate provenance field, the override taking effect on the very next login with no API restart (SC-007), `DELETE /admin/config/auth/role-mappings/{role}` reverting that role to its Helm-seeded value and being idempotent when no override exists (400 on an invalid `{role}` value), and an audit event recorded for each `PUT`-with-`helmOverride`-change and each `DELETE` — in api/internal/handlers/config_envtest_test.go (NEW). Depends on T013/T014 (implementation must exist first) but the file is new and unique, so [P] against other files.

### Slice 6: API Config Endpoint Returns Install-Time Settings

- [X] T017 [US1] Extend api/cmd/main.go serve subcommand to accept and store --game-data-storage-class flag (report-only, read from Gameplane installation) with GAMEPLANE_GAME_DATA_STORAGE_CLASS env fallback (API binary — distinct from the operator flag in T004). Sequential with T006 and T013 (same file, api/cmd/main.go — all three must be reconciled).
- [X] T018 [US1] Extend the `getAll()` handler in api/internal/handlers/config.go to add a new `installTimeSettings` object to the `GET /admin/config` response, containing `gameDataStorageClass` (from T017's flag) and `oidcHelmProvider` (`groupsClaim`, `defaultRole` — Helm-only per M5, read directly from the CLI-flag-built policy — and the Helm-seeded `roleMappings`, unaffected by any `helmOverride`). This is a pure Helm-seed snapshot, computed at request time (not stored in the database) and distinct from the "auth" section's `providers[]` "helm" entry and from `auth.helmOverride`'s override view (Slice 5); no provenance field is needed here since nothing in `installTimeSettings` is ever overridden. `installTimeSettings` is read-only: `PUT /admin/config/{section}` has no section named `installTimeSettings`, so it cannot be written. Sequential with T011/T013/T014 (same file, config.go — all must be reconciled).
- [X] T019 [P] [US1] Add `InstallTimeSettings` interface to the `AllConfig` type in web/src/lib/config.ts with `gameDataStorageClass` (string | null) and `oidcHelmProvider` (`{groupsClaim, defaultRole, roleMappings: {admin, operator, viewer}}` — Helm-seeded, read-only) fields matching T018's response; add the optional `helmOverride?: { roleMappings?: { admin?: string[]; operator?: string[]; viewer?: string[] } }` field to the existing `AuthCfg` type (Slice 5's storage shape) — a role key present (including `[]`) means DB-overridden, absent means Helm-seeded; no separate `source` field.
- [X] T020 [US1] Extend api/internal/handlers/config_envtest_test.go (same file created by T016) with cases verifying `installTimeSettings` is readable via `GET /admin/config`, returns the correct Helm-configured `gameDataStorageClass` and `oidcHelmProvider` values (unaffected by any `helmOverride` present), and is not writable via `PUT /admin/config/{section}` (no such section name exists). Sequential with T016 (same file) and depends on T018 (implementation under test).

### Slice 7: Web Dashboard — Install-Time Settings Display & `helmOverride` Role-Mapping Editing Surface

- [X] T021 [P] [US1] Design the install-time settings read-only display section in design.pen via pencil MCP (showing storage class and OIDC Helm-provider info in read-only format) and re-export to design-export/json/<id>.json and design-export/screenshots/<id>.png
- [X] T022 [US1] Design the `helmOverride` role-mapping editing surface in design.pen via pencil MCP — per-role effective-mapping list labeled by provenance (override vs. Helm-seeded), set/update controls, and a "reset to Helm default" action per role, shown only where an override exists — and re-export to design-export/json/<id>.json and design-export/screenshots/<id>.png. This is a third distinct visual element, separate from T021's read-only display and T029's admin-warning modal (Constitution Principle II); it edits the same design.pen document as T021 so it is not tagged [P] against it, but it is its own design.pen pass covering a different node.
- [X] T023 [US1] Implement the OIDC half of the install-time settings read-only section in web/src/routes/AdminSettings.tsx displaying `installTimeSettings.oidcHelmProvider` with explanatory note ("Configured at install time via Helm values"); when `oidcHelmProvider` has no role mappings configured (FR-012), render a distinct empty-state indicator ("OIDC role mappings are not configured") with guidance copy on how to configure them via Helm values, instead of an empty/blank section; when `oidcHelmProvider` contains an admin-role mapping (FR-015), render a warning banner flagging that the Helm-supplied mapping grants admin access, since this read-only display path has no confirm step (unlike the dashboard-editable flow added in Slice 8) — the banner is the only warning available for mappings that arrive via Helm rather than the dashboard. **DECISION SPLIT (Maintainer Decision, 2026-08-27)**: The storage-class half (`installTimeSettings.gameDataStorageClass`) renders separately in web/src/routes/Cluster.tsx (route `/cluster`, design screen j9W8A), not in AdminSettings.tsx. Note: `GET /admin/config` requires the `config:read` permission (api/internal/rbac/rbac.go, line 170), which AdminSettings.tsx users have via admin/operator roles, but viewers in Cluster.tsx do not — Cluster.tsx must gate both the query and the display section with `can(me, "config:read")` from web/src/lib/auth.ts so viewers lacking this permission see neither a loading state nor the value. Depends on T021's design + T018's config data.
- [X] T024 [US1] Implement the `helmOverride` role-mapping editing surface in web/src/routes/AdminSettings.tsx (from T022's design) — lists each role's effective mapping and its provenance (override if `auth.helmOverride.roleMappings` carries that role's key, else the Helm-seeded value from `oidcHelmProvider`), lets an admin set/update a role's override list via the existing `PUT /admin/config/auth` call (through T025's client helper), and offers a "reset to Helm default" action per role via T025's `DELETE` call. Sequential with T023 (same file). Depends on T022's design + T013 (the reset route).
- [X] T025 [US1] Add, to web/src/lib/config.ts, a client call for the new `DELETE /admin/config/auth/role-mappings/{role}` reset route for T024's editing surface to consume; writes that set or change a role's override go through the existing `useUpdateConfigSection("auth")` PUT call (round-tripping the full `AuthCfg` including `helmOverride`, per T019's type) — no separate create/update endpoint exists. Same file as T019 (InstallTimeSettings interface); sequential with it, not [P].
- [X] T026 [US1] Add vitest cases in web/src/routes/AdminSettings.test.tsx verifying the OIDC half of the install-time settings section (oidcHelmProvider display) renders when data is present and updates when config data changes, plus a case verifying the FR-012 empty-state indicator and its configuration guidance copy render when no OIDC role mappings are present, plus a case verifying the FR-015 warning banner renders when `oidcHelmProvider` contains an admin-role mapping. **DECISION SPLIT (Maintainer Decision, 2026-08-27)**: Storage-class display vitest coverage (Cluster.tsx) is handled separately in Cluster.test.tsx as part of the Cluster.tsx implementation.
- [X] T027 [US1] Add vitest cases verifying the `helmOverride` editing surface's set/update actions call `PUT /admin/config/auth` with the expected `helmOverride.roleMappings` body and the reset action calls `DELETE /admin/config/auth/role-mappings/{role}`, and that the displayed mapping updates to reflect the new provenance (override vs. Helm-seeded) afterward, in web/src/routes/AdminSettings.test.tsx. Sequential with T026 (same test file).
- [X] T028 [US1] Document install-time settings display UI pattern, read-only convention, data source (Helm-injected, not dashboard-editable), the `helmOverride` role-mapping editing surface (including reset-to-Helm-default and provenance derivation), and the FR-015 Helm-provider admin-mapping warning banner in web/specs.md

### Slice 7b: OIDC Role Mapping Admin Warning Modal - Design Pass (Design-First, FR-015)

- [X] T029 [US1] Design the admin-role OIDC mapping warning + confirmation modal in design.pen via pencil MCP server and re-export to design-export/json/<id>.json and design-export/screenshots/<id>.png — applies to both the legacy OIDC provider editor and the new `helmOverride` editing surface (T024). Edits the same design.pen document as T021/T022 so it is not tagged [P] against them.

### Slice 8: OIDC Role Mapping Admin Warning & Confirm (Security UX, FR-015)

- [X] T030 [US1] Add warning + confirmation modal for admin-role OIDC mappings, applied to both the legacy OIDC provider role-mapping editor and the new `helmOverride` editing surface (T024), in web/src/routes/AdminSettings.tsx with message: "Mapping users to the admin role grants full cluster control. Ensure the mapped group contains only authorized personnel. Anyone in these groups gets full admin access from their next login." Depends on T029's design + T023/T024 completion (same file).
- [X] T031 [US1] Add vitest case verifying the admin role mapping warning appears with the exact plan.md copy ("Mapping users to the admin role grants full cluster control. Ensure the mapped group contains only authorized personnel. Anyone in these groups gets full admin access from their next login."), confirmation is required, and UI blocks unsafe configuration in both the legacy OIDC editor and the `helmOverride` editing surface, in web/src/routes/AdminSettings.test.tsx (same file as T026/T027)
- [X] T032 [US1] Document admin-role OIDC mapping warning pattern, confirmation step requirement, and security rationale, covering both editors, in web/specs.md (same file as T028)

### Slice 10: E2E Tests for OIDC Role Mapping — Helm-Seeded and Admin-Managed (API-Auth Bucket)

- [X] T033 [P] [US1] Add five e2e tests in test/e2e/api_auth_e2e_test.go: (1) OIDC user assigned correct role on install-time Helm-seeded mapping at first login, (2) user's role re-evaluated on subsequent login if OIDC group membership changes, (3) user with no matching group receives default role (viewer), (4) an admin sets a `helmOverride` role-mapping list for a role via `PUT /admin/config/auth` that differs from that role's Helm-seeded mapping, and a subsequent login resolves the override with no API restart and no `helm upgrade` (SC-007), (5) the admin then calls `DELETE /admin/config/auth/role-mappings/{role}` on that role, and a subsequent login resolves back to the original Helm-seeded value
- [X] T034 [US1] Register five e2e tests (from T033) in api-auth bucket in test/e2e/buckets.sh (~1-2 additional logins per test, per plan.md Slice 10 — within the ~7-login-per-job budget)

**Checkpoint**: User Story 1 (OIDC) is fully functional and independently testable. An operator can deploy with OIDC role mappings configured at install time without bootstrap-admin, and admins can manage overrides through the dashboard without restarting the API or re-running Helm.

---

## Phase 4: User Story 2 - Install-Time Game-Data Storage Class Configuration (Priority: P2)

**Goal**: Enable operators to specify a default StorageClass for game-data PVCs at install time via Helm values, eliminating post-install manual edits and supporting heterogeneous clusters.

**Independent Test Criteria**:

A fresh Gameplane install with a custom default storage class specified (via Helm values) allows:
1. GameServers created from any template to use the Helm-configured storage class for game-data PVCs.
2. Explicit GameServer override to take precedence over template default, which takes precedence over install-time default, which falls back to cluster default (precedence chain).
3. A nonexistent storage class to result in GameServer remaining in Pending phase (not progressing to Failed) with a clear error visible in dashboard status.conditions within 30 seconds (SC-002) — the condition is recoverable (an admin creates the StorageClass and the PVC binds automatically with no action on the GameServer).
4. Existing GameServers' PVCs to remain unchanged when the Helm value is updated post-install (PVC immutability per SC-008).

### Slice 1 & Foundational: Operator Storage Class Flag

The operator's `--game-data-storage-class` CLI flag and the `GameServerReconciler.DefaultStorageClassName` struct field are delivered by **T004** (Phase 2 Foundational) — that task already covers this slice's flag/field, so no separate task is listed here. Slice 1's remaining work is simply this: the tasks below (Slices 2-3) consume `DefaultStorageClassName` directly.

### Slice 2-3: PVC Storage Class Injection Across Reconciliation Paths

- [X] T035 [P] [US2] Inject install-time default storage class after precedence chain (GameServer.Spec.Storage.StorageClassName > GameTemplate default > DefaultStorageClassName flag > nil/cluster default) in reconcilePVC() function in operator/internal/controller/gameserver_controller.go
- [X] T036 [P] [US2] Inject install-time default storage class in reconcileExtraPVCs() loop for each extra volume after applying GameServer/GameTemplate overrides in operator/internal/controller/gameserver_extravolumes.go
- [X] T037 [P] [US2] Inject install-time default storage class after precedence chain in reconcileModPVC() function in operator/internal/controller/gameserver_version.go

### Slice 2-3: Storage Class Precedence & Error Handling Tests

- [X] T038 [P] [US2] Add 4 envtest cases for storage class precedence (GameServer override > GameTemplate default > operator default > cluster default) and immutability in operator/internal/controller/gameserver_storage_envtest_test.go (NEW)
- [X] T039 [US2] Add PVC provisioning error detection in reconcileStatus() by checking PVC conditions and extracting StorageClass-not-found error reason, modeling after MetalLB IPAddressPool pattern in operator/internal/controller/gameserver_status.go; **DECISION (Maintainer Decision, 2026-08-27)**: GameServer remains in Pending phase (not progressing to Failed) with Ready condition set to False and reason PVCProvisioningFailed — the condition is recoverable (an admin creates the StorageClass and the PVC binds automatically), so a terminal Failed phase would misreport the situation.
- [X] T040 [P] [US2] Add 2 envtest cases for PVC provisioning failure detection (nonexistent StorageClass keeps GameServer in Pending phase with Ready condition False and reason PVCProvisioningFailed, not progressing to Failed) in operator/internal/controller/gameserver_status_envtest_test.go (NEW)

### Slice 9: E2E Tests for Storage Class Configuration (Operator Bucket)

- [X] T041 [P] [US2] Add 3 e2e tests in test/e2e/gameserver_e2e_test.go: (1) GameServer PVC created with Helm-configured default storage class, (2) nonexistent storage class keeps the GameServer in Pending phase with Ready=False and reason PVCProvisioningFailed, displaying the error in dashboard status.conditions, (3) explicit GameServer override takes precedence over operator default
- [X] T042 [US2] Register 3 e2e tests (from T041) in operator bucket in test/e2e/buckets.sh

**Checkpoint**: User Story 2 (Storage) is fully functional and independently testable. An operator can deploy with a default storage class configured at install time.

---

## Phase 5: Polish & Cross-Cutting (Slice 11 — Documentation & Module Specs)

**Purpose**: Cross-cutting documentation and specification updates that apply to both stories.

**Note**: `operator/specs.md` (T046) and `api/specs.md` (T047) are listed here for completeness of the documentation sweep, but per Constitution Principle IV (specs.md updated in the same commit as the behavior it documents), they MAY instead land alongside their corresponding implementation tasks — T039/T035-T037 for T046, and T012/T013-T014 for T047 — rather than waiting for this Polish phase. `web/specs.md` already follows this pattern: it is updated in Slice 7/8 (T028, T032), not here.

- [X] T043 [P] Add per-IdP (Okta, Azure AD, Keycloak) OIDC role claim configuration examples and walkthrough, including how to set `helmOverride` role-mapping overrides via the dashboard and reset one to the Helm default, in docs/oidc.md
- [X] T044 [P] Document operator.gameDataStorage.storageClassName and OIDC-related Helm values in docs/install.md (values schema, defaults, examples)
- [X] T045 [P] Add OIDC role-mapping security model, Helm-seeded-vs-`helmOverride` per-role precedence, upgrade semantics (an existing override survives a `helm upgrade` that changes the Helm-seeded value for that role), and audit trail sections in docs/security.md
- [X] T046 [P] Document PVC StorageClass selection precedence chain, install-time default injection logic, and error surfacing (PVCProvisioningFailed) in operator/specs.md
- [X] T047 [P] Document OIDC Helm-provider synthesis at API startup, CLI flag definitions, group claim extraction, the `helmOverride` overlay (storage shape, merge/reset, no new table or migration), role mapping rules, and the FR-014 / override-audit event emission formats in api/specs.md
- [X] T048 [P] Add v0.2.0 release feature entries for install-time storage class configuration and install-time OIDC role mapping (Helm-seeded + `helmOverride` hybrid) in CHANGELOG.md

---

## Dependencies & Execution Order

### Phase Dependencies

```
Phase 1 (Setup) ↓
  ↓ (no dependencies)
Phase 2 (Foundational) ↓
  ↓ (blocks all stories)
┌─────────────────────────┐
│ Phase 3 (US1 - OIDC)    │  (can run parallel with Phase 4)
└─────────────────────────┘
│
Phase 4 (US2 - Storage)    (can run parallel with Phase 3)
│
Phase 5 (Polish)           (can run after all stories complete)
```

### Within-Phase Dependencies

**Phase 1 Setup**: All tasks [P], can run concurrently.

**Phase 2 Foundational**: All tasks [P], can run concurrently (T004 and T005 touch different files).

**Phase 3 (US1 - OIDC)**:
- Slice 4 (T006-T010): T006 is sequential (api/cmd/main.go); T007 is [P] (registry.go, no file conflict); T008 is sequential (api/internal/auth/oidc.go); T009 is sequential (same file as T008, depends on T008 landing first); T010 is [P] (new test file, independent)
- Slice 5 (T011-T016): T011 is sequential (edits api/internal/handlers/config.go and api/internal/auth/registry.go — depends on T007 landing first, same file registry.go); T012 is sequential (api/internal/auth/oidc.go — same file as T008/T009, both of which must land first; also depends on T011's parsed override data); T013 is sequential (api/internal/handlers/config.go, same file as T011, depends on it landing first; also touches api/cmd/main.go for the `MountConfig` auditor-param call-site change — same file as T006/T017, all three must be reconciled); T014 is sequential (same file as T013, depends on T013 landing first); T015 is sequential (extends api/internal/auth/oidc_rolemap_test.go — same file as T010, depends on T010 and T012 landing); T016 is [P] (new envtest file, independent of other files, though it depends on T013-T014 being implemented first)
- Slice 6 (T017-T020): T017 is sequential (api/cmd/main.go — same file as T006 and T013, all three must be reconciled); T018 is sequential (api/internal/handlers/config.go — same file as T011/T013/T014, all must be reconciled); T019 is [P] (web/src/lib/config.ts, independent file); T020 is sequential (extends api/internal/handlers/config_envtest_test.go — same file as T016, depends on T016 and T018)
- Slice 7 (T021-T028): T021 is [P] (design.pen, can start alongside Slice 4); T022 is sequential relative to T021 (same design.pen document, different node — not [P] against T021, but no other code dependency); T023 is sequential (AdminSettings.tsx, depends on T021's design + T018's config data); T024 is sequential (same file as T023, depends on T022's design + T013's reset route via T025); T025 is sequential (web/src/lib/config.ts — same file as T019, depends on T013's route shape); T026 is sequential (AdminSettings.test.tsx, depends on T023); T027 is sequential (same test file as T026, depends on T024 and T026); T028 is sequential (web/specs.md)
- Slice 7b (T029): Design pass for the warning modal, in the same design.pen document as T021/T022 (not [P] against them); sequential prerequisite for Slice 8 implementation
- Slice 8 (T030-T032): T030 is sequential (depends on T029's design + T023/T024 completion, same AdminSettings.tsx); T031 is sequential (same test file as T026/T027); T032 is sequential (same specs.md as T028)
- Slice 10 (T033-T034): T033 is [P] (e2e tests; tests 1-3 depend on Slice 4, tests 4-5 depend on Slice 5); T034 is sequential (depends on T033's test names); independent from the dashboard slices
- **Slice execution order**: Start Slice 4 (T006-T010) + design T021 in parallel; after T006 completes, continue Slice 4 (T007, then T008, then T009 — T010 can run in parallel throughout); once T007 lands, start Slice 5 (T011, then T012, then T013→T014 reset route/audit — T013 also touches api/cmd/main.go for the `MountConfig` auditor param — with T015/T016 tests following); once Slice 4 and T011/T013/T014 land, start Slice 6 (T017-T020, noting T017 must reconcile with T006's and T013's edits to api/cmd/main.go and T018 must reconcile with T011/T013/T014's edits to config.go); once T018 (config data) and T021 (design) are available, start Slice 7's T023; once T022 (design) and T013 (reset route) land, start T024/T025, then T026/T027/T028; once T023/T024 land and T029 (design) is available, start Slice 8 (T030-T032); Slice 10 (T033-T034) can run anytime after Slice 4 (tests 1-3) and Slice 5 (tests 4-5) land.

**Phase 4 (US2 - Storage)**:
- Slice 1: No task of its own — **T004** (Phase 2 Foundational) already delivers the operator flag and `DefaultStorageClassName` field that Slices 2-3 consume.
- Slices 2-3 (T035-T037): All [P], can run concurrently (different controller files)
- Slices 2-3 Tests (T038-T040): T038 is [P] (new precedence/immutability test file, independent of T039/T040); T039 is sequential/no [P] (implementation in gameserver_status.go, depends on T035-T037 landing); T040 is [P] (new envtest file, no file conflict with T038 or T039, though it depends on T039's implementation existing to test against)
- Slice 9 (T041-T042): T041 is [P] (e2e tests), T042 is sequential (depends on T041's test names)
- **Slice execution order**: T004 (Foundational) unblocks Slices 2-3; T035-T037 in parallel, then T038-T040 (T039 must land before T040 is meaningful, even though both carry no shared file conflict with the other); T041-T042 can run anytime after Foundational phase.

**Phase 5 (Polish)**: All tasks [P], can run concurrently (different doc/spec files). T046 and T047 may instead land with their implementation tasks — see the Phase 5 note above.

### Critical Path

```
T001-T005 (Setup + Foundational, 5 tasks)
  ↓
T006-T034 (US1, 29 tasks) OR T035-T042 (US2, 8 tasks) in parallel
  ↓
T043-T048 (Polish, 6 tasks)
```

**Minimum critical path**: T001-T005 → T006/T035 → ... → T034/T042 → T043-T048

---

## Parallel Execution Examples

### Phase 1-2: Parallel Setup & Foundational
```
Parallel group 1: T001, T002, T003 (Helm values + templates)
Parallel group 2: T004, T005 (CLI flags)
  ↓ Wait for all to complete
Proceed to stories
```

### Phase 3 (US1): OIDC Story Parallelization

**Wave 1** (after Foundational completes):
- Sequential: T006 (api/cmd/main.go — OIDC flag validation; carries no [P])
- Parallel (different file, can start alongside T006): T021 (design start)

**Wave 2** (after T006 completes):
- Parallel: T007 (registry.go — Helm provider synthesis; different file from T006/T008)
- Sequential (api/internal/auth/oidc.go): T008 (groups-claim extraction + role assignment/re-evaluation), then T009 (audit threading — depends on T008 landing first, same file)
- Parallel (independent, new test file): T010

**Wave 3** (after T007 and T008/T009 land):
- Sequential (api/internal/handlers/config.go + api/internal/auth/registry.go, depends on T007): T011 (helmOverride storage shape), then T012 (api/internal/auth/oidc.go — effectiveHelmPolicy + wiring, depends on T011 and on T008/T009 having landed first), then T013 (reset route, same file as T011; also adds the `*audit.Auditor` param to `MountConfig` and updates its api/cmd/main.go:245 call site), then T014 (audit, same file as T013)
- Sequential: T015 (extends oidc_rolemap_test.go — same file as T010, depends on T010 and T012)
- Parallel (new envtest file, depends on T013/T014 landing first but no file conflict with anything else): T016

**Wave 4** (after T006 and Wave 3 complete):
- Sequential: T017 (api/cmd/main.go — same-file conflict with T006 and T013; carries no [P])
- Sequential: T018 (api/internal/handlers/config.go — same-file conflict with T011/T013/T014; carries no [P])
- Parallel (different files, independent of T017/T018): T019, T033 (Slice 6 client types + e2e tests)
- Sequential (extends config_envtest_test.go — same file as T016, depends on T016 and T018): T020

**Wave 5** (after T021 design is available, after Wave 4 completes and T013 lands):
- Sequential (design.pen, one node each, not [P] against each other): T022, then T029 (can also be authored alongside T022, both precede their respective implementation waves)
- Sequential: T023 (AdminSettings display), T024 (editing surface), T026, T027 (vitest), T030 (admin warning), T031 (warning vitest), T028, T032 (docs)
- Sequential (web/src/lib/config.ts — same file as T019): T025

### Phase 4 (US2): Storage Story Parallelization

**Wave 1** (after Foundational completes — T004 already delivers the operator flag):
- Parallel: T035, T036, T037 (PVC injection across the three reconciliation paths)
- Parallel (new file, independent of T039/T040): T038 (precedence/immutability envtest)
- Sequential (status.go, no [P], depends on T035-T037 landing): T039 (provisioning-failure detection)
- Parallel (new file, no file conflict — but depends on T039's implementation existing to test against, so schedule it after T039 even though it carries [P]): T040
- Parallel (independent, e2e authoring): T041
- Sequential (after T041, depends on its test names): T042 (register e2e tests)

### Phase 5: Parallel Polish
```
Parallel: T043, T044, T045, T046, T047, T048 (all docs/specs)
```

---

## Implementation Strategy

### MVP Scope (User Story 1 Only)

Minimum shippable increment:

1. Complete Phase 1-2: Setup + Foundational (T001-T005)
2. Complete Phase 3: User Story 1 (T006-T034)
3. Test independently (e2e tests in T033-T034 pass on real cluster)
4. **Deploy/Demo**: OIDC role mappings work at install time without bootstrap-admin, and admins can manage overrides through the dashboard with no restart or `helm upgrade`

**Rationale**: US1 solves a critical setup blocker for OIDC-only deployments (P1) and, via the `helmOverride` overlay, the ongoing management gap SC-007 identified. US2 is a usability feature with an existing workaround (P2).

### Incremental Delivery

```
Day 1: Phases 1-2 (Setup + Foundational, T001-T005)
  ↓ All stories now unblocked

Parallel track (Days 2-4):
  Track A: Phase 3 (US1 - OIDC, T006-T034)
  Track B: Phase 4 (US2 - Storage, T035-T042)

Day 5: Phase 5 (Polish - docs + specs, T043-T048)
```

### Team Parallelization Strategy

With multiple developers:

1. **Developer A**: Phase 1-2 (shared foundation) — T001-T005 (must complete first)
2. Once Foundational completes:
   - **Developer B**: Phase 3 (US1 - OIDC) — T006-T034
   - **Developer C**: Phase 4 (US2 - Storage) — T035-T042
3. **Developer D** (or anyone free): Phase 5 (Polish) — T043-T048 (can start after any story completes)

---

## Open Questions

**SC-007 vs. the read-only Helm provider design — RESOLVED (maintainer decision, hybrid, settled mechanism).** Spec success criterion SC-007 requires that operators be able to manage OIDC role mappings through the administrative configuration interface, with changes taking effect on the next login attempt, without restarting the API or re-running Helm. The design previously adopted in this plan synthesized the Helm-configured OIDC provider as strictly **read-only** at API startup, which left SC-007 with no delivering task.

The maintainer has resolved this tension with a **hybrid** design, recorded in plan.md's Summary and data-model.md's "MAINTAINER DECISION" section:

- Helm values continue to *seed* the role mappings for the synthetic `"helm"` provider — unchanged, still immutable through the dashboard — satisfying FR-007/SC-003/SC-004 (Slice 4, T006-T010, unchanged).
- **No new table, no new migration**: an optional `helmOverride.roleMappings` field on the existing `"auth"` config row (Slice 5, T011) lets admins set, change, or reset per-role overrides through the *existing* `PUT /admin/config/auth` route plus one new `DELETE /admin/config/auth/role-mappings/{role}` reset route (Slice 5's T013; Slice 7's editing surface, T022/T024/T025/T027).
- Resolution is a two-step per-role merge then the unmodified `computeRole`: DB override (if present, even `[]`) replaces the Helm-seeded list for that role; otherwise the Helm-seeded list stands. Implemented by `effectiveHelmPolicy` (T012) and verified against Helm reasserting on upgrade not clobbering an override (T015's upgrade-does-not-clobber case).
- The override takes effect on the very next login — no API restart, no `helm upgrade` — verified by envtest (T016) and by e2e tests #4-5 in Slice 10 (T033-T034).
- An explicit "reset to Helm default" action (T013) lets an admin discard the override and return to the Helm-declared value.
- Every mutation is audited (T014) following one consistent reason-string format.
- `groupsClaim` and `defaultRole` remain Helm-only in v1 — no override path, no endpoint accepts a write to either.

This is now fully delivered by the tasks listed above; no question remains open for this feature.

---

## Requirement Coverage

### Functional Requirements Mapping

| Requirement | Delivered by Task(s) |
|---|---|
| **FR-001** (Storage class configurable via Helm) | T001, T004 (Helm value + CLI flag) |
| **FR-002** (GameServer PVCs use configured class) | T004, T035, T036, T037 (CLI flag/field + PVC injection in reconcilePVC, reconcileExtraPVCs, reconcileModPVC) |
| **FR-003** (Configured class applies only to new PVCs) | T035, T036, T037, T038 (all three injection sites + precedence/immutability envtest) |
| **FR-004** (Explicit override takes precedence) | T004, T035, T036, T037 (CLI flag/field + precedence chain logic in all three injection sites) |
| **FR-005** (Missing StorageClass shows clear error) | T039, T040, T041, T042 (error detection logic + envtest + e2e test #2 + registration) |
| **FR-006** (Operators view storage class in admin config) | T018, T019 (config endpoint + UI in Cluster.tsx); T023 is OIDC-only (AdminSettings.tsx) per Maintainer Decision 2026-08-27 |
| **FR-007** (OIDC role mappings configurable via Helm) | T001, T005 (Helm values + CLI flags) |
| **FR-008** (Helm includes group claim + mappings + default role) | T001, T005, T006 (values + flags + parsing) |
| **FR-009** (User assigned role at first login) | T007, T008, T033, T034 (Helm provider synthesis + groups-claim extraction/first-match assignment + OIDC e2e tests + registration) |
| **FR-010** (Group claim compared to mappings) | T007, T008 (provider synthesis + claim extraction and matching logic) |
| **FR-011** (Role re-evaluated on each login) | T008, T012, T033, T034 (re-evaluation logic + helmOverride-aware resolution + OIDC e2e tests + registration) |
| **FR-012** (Unmapped user gets default role, admin sees warning) | T007, T008, T019, T023, T026, T028 (provider logic + default-role assignment + UI display, including the not-configured empty-state indicator and its guidance copy + vitest coverage of that empty state + docs) |
| **FR-013** (Bootstrap-admin coexists with OIDC mappings) | T007, T008, T010 (provider doesn't block bootstrap + demotion-guard logic preserving locally-granted roles + unit test asserting coexistence) |
| **FR-014** (Role changes audited) | T009, T010, T033, T034 (audit event emission + unit tests + e2e) |
| **FR-015** (Admin role mapping warning + confirmation) | T023 (Helm-path read-only warning banner), T024 (helmOverride editing surface it must also cover), T029, T030, T031, T032 (design + dashboard-editable warning modal covering both editors + vitest + docs) |
| **FR-016** (Documentation of both features) | T028, T032, T043, T044, T045, T046, T047, T048 (web specs (install-time display + editing surface + warning pattern) + OIDC docs + Helm values docs + security docs + operator specs + API specs + CHANGELOG) |
| **FR-017** (View installed config in admin interface) | T018, T019, T023, T028 (config endpoint + UI + docs) |

### Success Criteria Mapping

| Criteria | Delivered by Task(s) |
|---|---|
| **SC-001** (New PVCs use the Helm-specified storage class) | T035, T036, T037 (injection across all three PVC paths), T038 (precedence envtest), T041 (e2e test confirming PVCs target the configured class) |
| **SC-002** (Error visible within 30s) | T039, T040, T041, T042 (error detection logic + envtest + e2e test #2 + registration) |
| **SC-003** (OIDC user with a matching mapping immediately receives admin) | T007, T008, T033 (Helm provider synthesis + first-match role assignment + OIDC e2e test #1) |
| **SC-004** (First user's role matches group membership) | T007, T008, T033 (Helm provider synthesis + groups-claim assignment logic + OIDC e2e test #1) |
| **SC-005** (Role re-evaluation on login) | T008, T012, T033, T034 (re-eval logic + helmOverride-aware resolution + OIDC e2e tests + registration) |
| **SC-006** (View storage class + mappings via the admin interface) | T018, T019 (config endpoint + config.ts type + Cluster.tsx storage display); T023 is OIDC-only (AdminSettings.tsx mappings display) per Maintainer Decision 2026-08-27 |
| **SC-007** (Manage role mappings via admin interface without re-running Helm) | T011, T012, T013, T014 (helmOverride storage shape + resolution precedence + reset route + audit events — no API restart or `helm upgrade` needed); T016 (envtest verifying next-login effect with no restart); T022, T024, T025, T027 (dashboard editing surface, its client calls, its reset action, and vitest coverage); T033, T034 (e2e tests #4-5: an admin override wins over the Helm-seeded value without a restart, and "reset to Helm default" reverts it) |
| **SC-008** (Backward compatibility: no storage value → cluster default; OIDC with no mappings → viewer) | T035, T036, T037, T038 (storage half: precedence chain falls back to nil/cluster default, tested in the precedence/immutability envtest) + T007, T008, T010, T015 (OIDC half: no configured mappings falls back to the policy's default role, tested in the unit tests, including T015's case proving an absent `helmOverride` is a strict no-op) |

### Coverage Summary

**Status**: 17 of 17 FRs are covered by at least one task. 8 of 8 SCs are covered by at least one task.

**Gaps**: None. SC-007 — previously the sole outstanding gap under the read-only-Helm-provider design — is now delivered via the maintainer's hybrid decision: Helm seeds role mappings, a `helmOverride` field on the existing `"auth"` config row (Slice 5, T011-T014) lets admins manage per-role overrides through the dashboard (Slice 7, T022/T024/T025/T027) with no API restart and no `helm upgrade`, verified by envtest (T016) and e2e tests (T033-T034).

---

## Execution Checklist at Completion

- [ ] **Phase 1 & 2 complete**: All CLI flags and Helm values in place; both stories unblocked
- [ ] **Phase 3 complete**: OIDC role mapping at install time works end-to-end, including admin-managed `helmOverride` overrides with no restart/`helm upgrade`; e2e tests pass (api-auth bucket, T033-T034)
- [ ] **Phase 4 complete**: Storage class configuration at install time works end-to-end; e2e tests pass (operator bucket, T041-T042)
- [ ] **Phase 5 complete**: All docs and specs updated with configuration examples
- [ ] **All CI checks green**: lint (gofmt, vet, golangci), test (unit + envtest), e2e (buckets pass), coverage gates met
- [ ] **Requirement traceability verified**: Each FR-00X and SC-00X requirement confirmed delivered by implementation tasks

---

## Notes

- [P] tasks = different files, no file conflict with any task scheduled alongside them. A [P] tag does not mean the task has no dependencies at all — see the Dependencies notes above for real load-bearing ordering (e.g. T016 is [P] because its file is unique, even though it depends on T013-T014 landing first).
- [Story] label maps task to US1 (OIDC) or US2 (Storage); Setup/Foundational/Polish have no story labels
- Each user story should be independently completable and testable (demonstrated via independent e2e tests)
- Both stories can be developed in parallel after Foundational phase completes
- Commit after each task or logical group (per CLAUDE.md rule 11)
- Stop at any checkpoint to validate story independently before moving to next phase
- **Design requirement**: Slice 7 (T021, T022) and Slice 7b (T029) each require their own pencil MCP pass for a distinct AdminSettings visual element (read-only display, `helmOverride` editing surface, and admin-warning modal respectively — Constitution Principle II); re-export JSON + screenshot for each touched node in the same commits (per CLAUDE.md rule 1). All three edit the same design.pen document, so none of T022/T029 are tagged [P] against T021 or each other.
- **New test files**: Three envtest files do not yet exist and will be created as NEW in their respective tasks: api/internal/handlers/config_envtest_test.go (T016, later extended by T020), operator/internal/controller/gameserver_storage_envtest_test.go (T038), operator/internal/controller/gameserver_status_envtest_test.go (T040)
- **No new handler file, no new table, no new migration**: Slice 5's HTTP surface change (the `DELETE` reset route) lives in the existing api/internal/handlers/config.go alongside the existing `GET`/`PUT /admin/config` handlers, not a new file — and the `helmOverride` overlay reuses the existing `"auth"` row of the existing `config` table. `api/internal/db/migrations/` stays at `008_captures_rbac.sql`; this feature adds zero files there.
- **Test-first for e2e**: e2e tests (T033, T041) must be written to FAIL before story implementation begins, then implementation makes them PASS. Note: E2E task IDs appear after implementation tasks in the list, but e2e tests should be authored concurrently with (not after) their story's implementation tasks; task ID order is not strict execution order across independent slices
- **specs.md commit timing**: `operator/specs.md` (T046) and `api/specs.md` (T047) are listed in Phase 5 for the documentation sweep, but per Constitution Principle IV they may instead be committed alongside the implementation task whose behavior they document, rather than deferred to Polish — see the Phase 5 section note.

---

## Phase 6: Convergence

**Purpose**: Close gaps found by `/speckit-converge` between the artifacts (spec.md, plan.md, tasks.md) and the state of the code. Appended without modifying any existing task.

- [ ] T049 CRITICAL: Add the three missing OIDC login-time e2e tests to test/e2e/api_auth_e2e_test.go via a fake OIDC IdP fixture — (a) a user in a Helm-seeded admin group receives the admin role on first login, (b) the role is re-evaluated and updated on a subsequent login after the group changes, (c) a user matching no mapping receives the default role — and register all three in the api-auth bucket in test/e2e/buckets.sh within the ~7-login-per-job budget, per T033/T034 and Constitution Principle I (missing)
- [X] T050 Add the FR-015 admin-role warning banner and confirmation step to `AddProviderForm` in web/src/routes/AdminSettings.tsx, mirroring `RoleMappingOverridesCard`'s `confirmingRole`/`ConfirmDialog` pattern and using the same plan-approved copy, gated on the Admin groups field being non-empty at submit time, per FR-015 / plan.md Slice 8 (partial)
- [X] T051 Add the matching vitest case in web/src/routes/AdminSettings.test.tsx asserting the admin-mapping warning and confirm step appear in the `AddProviderForm` editor (the second surface T031 was to cover), per T031 (partial)
- [x] T052 Assert `status.phase` stays Pending (and never becomes Failed) in TestGameServer_NonexistentStorageClassSurfacesError in test/e2e/gameserver_e2e_test.go and in the matching missing-StorageClass case in operator/internal/controller/gameserver_status_envtest_test.go, replacing the comment-only claim, per plan.md T039 (partial)
