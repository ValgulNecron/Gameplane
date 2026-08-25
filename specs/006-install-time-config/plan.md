# Implementation Plan: Install-Time Configuration

**Branch**: `006-install-time-config` | **Date**: 2026-08-25 | **Spec**: ./spec.md

## Summary

This feature enables operators to configure two critical Gameplane settings at install time via Helm values, eliminating post-deployment manual steps and solving the OIDC-only bootstrap problem. First (P1): OIDC role mapping via group claims, so the first OIDC user logs in with the correct dashboard role without running bootstrap-admin. Second (P2): default storage class for game-data volumes, so operators specify once at install time instead of editing every GameTemplate afterward.

**Technical approach**: Operator accepts two new Helm values—`operator.gameDataStorage.storageClassName` (propagated to operator flag `--game-data-storage-class`) and API-consumed Helm-supplied OIDC config (via existing `--oidc-*` flags expanded with `--oidc-groups-claim`, `--oidc-default-role`, and three per-role `--oidc-role-mapping-{admin,operator,viewer}` flags, each with `GAMEPLANE_OIDC_*` env fallbacks). Storage class defaults are injected after the PVC precedence chain (GameServer override > GameTemplate default > install-time default > cluster default). OIDC mappings are synthesized as a read-only "helm" provider at API startup, coexisting with dashboard-managed providers. Error states (missing StorageClass, misconfigured OIDC) are surfaced in GameServer status conditions and the dashboard config view. Install-time settings are viewable (read-only) in the admin config interface.

## Technical Context

**Language/Version**: Go 1.26 (via go.work modules: netguard, gameaction, gameproto, operator, api, agent, audit-syslog-bridge, telemetry-receiver, sentinel, capture-sidecar, mcp-server, svcutil, tunnel, test/e2e); TypeScript 5.6 strict, React 18.3, Vite 5.4 for the dashboard.

**Primary Dependencies**: `controller-runtime` v0.19.0, `client-go` v0.35.0 (operator reconciliation); `chi` v5, `coder/websocket` v1.8.12 (API); `coreos/go-oidc/v3` (OIDC token verification); Helm 3.13+ for chart templating and values binding.

**Storage**: SQLite (modernc.org/sqlite, tested) or PostgreSQL (pgx/v5, experimental, build-time selected via `postgres` tag). Game-data state persists via Kubernetes PVCs (referencing StorageClass). API config stored in database `config` table (key "auth", value JSON with provider list including Helm-synthesized provider).

**Testing**: `go test` + `testing/quick` (unit), envtest 1.31 (integration), `kind` v0.24+ with e2e suite (test/e2e/, bucketed per buckets.sh). Vitest 2.1 for frontend. All test tiers run in CI only (GitHub Actions), not locally.

**Target Platform**: Kubernetes 1.28+ (API minimum); tested on kind clusters and kubelab live cluster. CRDs are cluster-scoped (Module, ModuleSource, Cluster) and namespaced (GameTemplate, GameServer, Backup, BackupSchedule, Restore).

**Project Type**: Kubernetes control plane (operator + CRDs) + REST/WebSocket API gateway + React SPA dashboard + in-pod sidecar agents. Multi-module Go workspace shared via go.work.

**Performance Goals/Constraints** (from spec SC-###):
- SC-002: Error surfacing for missing StorageClass within 30 seconds of pod scheduling.
- SC-005: User role re-evaluation on OIDC login within 5 minutes (dashboard refresh cycle).
- SC-007: Admin config changes take effect on next login without API restart.
- Standard Gameplane budgets: API login rate limiter (5/min per IP, 3/min per user, burst 6), operator reconciliation <5s per PVC creation.

**Scale/Scope**: 14 Go modules + 1 React codebase. This feature touches: operator (PVC reconciliation), API (OIDC provider synthesis + config surface), web (read-only admin config display), Helm chart (new Helm values), and test/e2e (3 operator + 3 API e2e tests, 6 envtest cases).

## Constitution Check

| Principle | Status | Justification |
|-----------|--------|---------------|
| **I. E2E-Tested Delivery** | PASS | 6 new e2e tests required: 3 in `operator` bucket (storage class precedence, nonexistent class error, explicit override); 3 in `api-auth` bucket (OIDC role assignment at first login, re-evaluation on group change, mapping with no match defaults to viewer). All 6 tests must be registered in `test/e2e/buckets.sh` before merge (CI gate enforces no unbucketed tests). 4 envtest cases added to operator and api integration suites. |
| **II. Design-First for User-Facing Change** | PASS with note | Backend-only changes (operator PVC reconciliation, API OIDC synthesis) are exempt per the Constitution's exemption for operator-only and API-only changes. However, the web dashboard slice adds a new section to `/web/src/routes/AdminSettings.tsx` (installTimeSettings display + FR-015 admin-mapping warning), which is a user-facing visual change and therefore requires a design.pen pass via the pencil MCP server plus matching design-export re-export (design-export/json/<id>.json + design-export/screenshots/<id>.png) in the same commit. |
| **III. Language & Ecosystem Best Practice** | PASS | Go errors wrap with `%w` throughout (operator PVC validation, API OIDC role mapping). TypeScript strict (no `any` without comment). ESLint/gofmt enforced in CI; no suppressions. CRD types unchanged (existing GameServer.Spec.Storage.StorageClassName already present; no `make generate`/`make manifests` required). |
| **IV. Spec-Driven Development** | PASS | Spec at ./spec.md (complete, verified claims in research wave). This plan (here) governs implementation. Each of 14 Go modules and the web tree maintain specs.md; updates required to `api/specs.md` (CLI flags + OIDC flow + audit events), `operator/specs.md` (PVC materialization), and `web/specs.md` (installTimeSettings display + admin-mapping warning); docs/install.md, docs/security.md, docs/oidc.md updated post-implementation. Module specs updated in same commit as behavior change. |
| **V. Delegate to Workflows & Subagents** | PASS | Main loop writes a single Workflow orchestrating 7 concurrent subagents (haiku): 2 operator (PVC reconciliation + status), 2 API (OIDC provider + config view), 1 web (admin settings display), 1 Helm chart values, 1 test authoring. Sonnet reviews combined output (phase-wise), then haiku fix wave on any findings. No Fable involvement (explicit user auth required, not requested). |
| **VI. CI Bears the Heavy Lifting** | PASS | All builds, tests, lint, coverage, envtest, e2e run on GitHub Actions only. No local test execution. Operator-provided kubelab cluster (remote) may be used for manual validation after CI-green, not as substitute for CI. Change considered done only after all CI checks pass green, including e2e buckets. |

## Project Structure

### Documentation (this feature)

```text
specs/006-install-time-config/
├── spec.md              # Feature specification (requirements, user stories, scope)
├── plan.md              # This file (implementation breakdown)
├── research.md          # Phase 0 research findings (helm-storage, pvc-path, pvc-errors, oidc, config-admin, tests, specs-md)
├── data-model.md        # Phase 1: CRD/storage/config schema (NOT CREATED YET)
├── quickstart.md        # Phase 1: quickstart walkthrough (NOT CREATED YET)
├── contracts/           # Phase 1: REST API + wire contract specs (NOT CREATED YET)
└── tasks.md             # Phase 2: per-slice task breakdown (NOT CREATED YET)
```

### Source Code (repository root)

```text
charts/gameplane/
├── values.yaml                          # ADD: operator.gameDataStorage.storageClassName
├── templates/operator.yaml              # ADD: --game-data-storage-class flag to operator Deployment args
└── templates/api.yaml                   # EDIT: ADD conditional --game-data-storage-class flag to api serve Deployment args (read-only, reporting only)

operator/
├── api/v1alpha1/
│   ├── gameserver_types.go              # UNMODIFIED (Storage.StorageClassName already present)
│   └── gametemplate_types.go            # UNMODIFIED (Storage.StorageClassName already present)
├── cmd/main.go                          # ADD: --game-data-storage-class flag + env var binding
├── internal/controller/
│   ├── gameserver_controller.go         # EDIT: reconcilePVC() — inject install-time default after nil-check (line 526)
│   ├── gameserver_extravolumes.go       # EDIT: reconcileExtraPVCs() loop — inject default (line 129)
│   ├── gameserver_version.go            # EDIT: reconcileModPVC() — inject default (line 212)
│   ├── gameserver_status.go             # EDIT: reconcileStatus() — add PVC provisioning check + condition
│   └── gameserver_storage_envtest_test.go  # NEW: envtest cases for PVC storage class selection
└── specs.md                             # UPDATE: document storage class reconciliation + operator flags

api/
├── cmd/main.go                          # ADD: --oidc-groups-claim, --oidc-default-role, --oidc-role-mapping-{admin,operator,viewer} flags with GAMEPLANE_OIDC_* env fallbacks; ADD --game-data-storage-class flag (read-only reporting)
├── internal/auth/
│   ├── oidc.go                          # EDIT: thread audit recorder into OIDC login path to emit FR-014 audit event on first assignment and role re-evaluation
│   ├── registry.go                      # UNMODIFIED (HelmProviderName == "helm" already reserved)
│   └── oidc_rolemap_test.go             # ADD: unit test cases for Helm provider binding + role re-evaluation + audit event emission
├── internal/handlers/
│   ├── config.go                        # EDIT: getAll() handler — add InstallTimeSettings to response
│   └── config_envtest_test.go           # ADD: test InstallTimeSettings is readable, not writable
└── specs.md                             # UPDATE: document OIDC Helm-provider flow + CLI flags + audit event emission

web/
├── specs.md                             # UPDATE: document installTimeSettings display + OIDC mapping warning UI pattern
├── src/lib/config.ts                    # ADD: InstallTimeSettings interface to AllConfig
├── src/routes/AdminSettings.tsx         # EDIT: add read-only section displaying gameDataStorageClass + oidcHelmProvider; EDIT: add warning + confirm step for admin-role OIDC mappings (FR-015)
└── src/routes/AdminSettings.test.tsx    # ADD: test that InstallTimeSettings section renders (if present); ADD: test warning and confirm step for admin mappings

test/e2e/
├── api_auth_e2e_test.go                 # ADD: 3 tests — OIDC mapping at install, first login, re-eval on group change
├── gameserver_e2e_test.go               # ADD: 3 tests — storage class default, nonexistent class error, explicit override
├── buckets.sh                           # UPDATE: register 3 tests in api-auth bucket, 3 in operator bucket
└── e2e_suite_test.go                    # UNMODIFIED (bucket registration in buckets.sh, not test file)

docs/
├── install.md                           # UPDATE: add Helm values for operator.gameDataStorage.storageClassName + OIDC role mappings
├── security.md                          # UPDATE: document OIDC role-mapping security model + Helm-vs-runtime coexistence
├── oidc.md                              # NEW or EXTENDED: per-IdP (Okta, Azure, Keycloak) walkthrough for role mappings
└── architecture.md                      # UNMODIFIED (high-level overview, no feature-specific detail needed)

Makefile                                 # UNMODIFIED (existing targets cover all build/test/lint steps)
```

**Structure Decision**: This feature spans two primary code layers — operator (storage class reconciliation, PVC validation) and API (OIDC provider synthesis, config display) — with supporting changes to the dashboard (read-only settings view) and Helm chart (new Helm values propagation). No new packages or modules are created; the feature is integrated into existing packages via new functions and fields. The test changes (new e2e/envtest cases, plus bucket registration) are isolated to test/e2e/ and per-module test files.

## Complexity Tracking

No constitutional violations; this section is intentionally empty.

## Implementation Sequence

The following 6 slices are independently committable (each is a logical unit of work that can be reviewed and merged separately) and ordered by dependency and user story priority (P1 OIDC before P2 storage class).

### Slice 1: Operator Storage Class Flag & PVC Injection (Foundation, Dependency)
- **What**: Add operator flag `--game-data-storage-class` (env var fallback `GAMEPLANE_GAME_DATA_STORAGE_CLASS`), add field to GameServerReconciler struct, inject the default into PVCs after the existing precedence chain (GameServer override > GameTemplate default > nil).
- **Touch Points**: `operator/cmd/main.go` (flag def + field binding), `operator/internal/controller/gameserver_controller.go` (reconcilePVC), `gameserver_extravolumes.go` (reconcileExtraPVCs loop), `gameserver_version.go` (reconcileModPVC). Add 4 envtest cases.
- **Why first**: Storage class handling is independent and foundational. OIDC features depend on nothing from this slice, so it can be merged separately.
- **Priority**: P2 (lower user impact; operational convenience but has a workaround).
- **Commit message pattern**: `feat(operator): add --game-data-storage-class install-time default for game PVCs`

### Slice 2: GameServer Status Condition for PVC Provisioning Errors
- **What**: In GameServerReconciler, after fetching the PVC, check if it's stuck Pending due to missing StorageClass. Add a helper to extract the error reason and set a Ready condition with reason "PVCProvisioningFailed" and a detailed message (e.g., "StorageClass 'fast-nvme' not found").
- **Touch Points**: `operator/internal/controller/gameserver_status.go` (reconcileStatus, add PVC check + condition), modeling after the existing MetalLB IPAddressPool check pattern (lines 125-151). Add 2 envtest cases for error detection.
- **Why second**: Depends on Slice 1 (needs the PVC to exist with a potentially missing StorageClass). Independent from OIDC. Enables SC-002 (error surfacing within 30s).
- **Priority**: P2.
- **Commit message pattern**: `feat(operator): detect and report StorageClass not found in GameServer status`

### Slice 3: Helm Values for Storage Class & Propagate to Operator
- **What**: Add Helm value `operator.gameDataStorage.storageClassName: ""` to `charts/gameplane/values.yaml` (defaults to empty, cluster default). Update `charts/gameplane/templates/operator.yaml` to conditionally pass `--game-data-storage-class=<value>` to the operator Deployment args (only if non-empty, matching the pattern for optional flags like `--agent-log-level`).
- **Touch Points**: `charts/gameplane/values.yaml` (new key after line 100), `charts/gameplane/templates/operator.yaml` (add conditional args block, matching existing pattern for optional flags).
- **Why third**: Depends on Slice 1 (operator must accept the flag). Can ship before API/OIDC work, allowing operators to test storage class configuration independently.
- **Priority**: P2.
- **Commit message pattern**: `feat(charts): add operator.gameDataStorage.storageClassName Helm value`

### Slice 4: OIDC Helm Provider Synthesis & API Flag Binding (Foundation for OIDC, Dependency)
- **What**: Extend `api/cmd/main.go` with five new flags: `--oidc-groups-claim` (string, env fallback `GAMEPLANE_OIDC_GROUPS_CLAIM`, defaults to "groups"), `--oidc-default-role` (string, env fallback `GAMEPLANE_OIDC_DEFAULT_ROLE`, defaults to "viewer"), `--oidc-role-mapping-admin` (string, env fallback `GAMEPLANE_OIDC_ROLE_MAPPING_ADMIN`, comma-separated claim values), `--oidc-role-mapping-operator` (string, env fallback `GAMEPLANE_OIDC_ROLE_MAPPING_OPERATOR`, comma-separated claim values), `--oidc-role-mapping-viewer` (string, env fallback `GAMEPLANE_OIDC_ROLE_MAPPING_VIEWER`, comma-separated claim values). Bind these to the existing OIDC instance (or create a new OIDCPolicy struct if needed). Modify the API's auth registry to synthesize a read-only "helm" provider containing this config. Thread the audit recorder into the OIDC login path (api/internal/auth/oidc.go) to emit an audit event (FR-014) on role assignment with the reason format: `"oidc role assigned: provider=helm matched=<claimValue|none> from=<oldRole> to=<newRole>"`. Add unit tests for flag parsing, provider synthesis, and audit event emission on first assignment and role re-evaluation.
- **Touch Points**: `api/cmd/main.go` (flag defs with env fallbacks + binding to OIDC config), `api/internal/auth/registry.go` (Enabled() method, add Helm provider synthesis), `api/internal/auth/oidc.go` (ensure role mapping logic uses the policy's groupsClaim if set; thread audit recorder for FR-014 event on first assignment and re-evaluation). Add 3 unit test cases covering audit emission.
- **Why fourth**: Foundation for OIDC role assignment. Independent from storage class. Includes FR-014 audit event implementation. Must run before API startup tests.
- **Priority**: P1 (critical for OIDC-only installs).
- **Commit message pattern**: `feat(api): synthesize Helm-configured OIDC provider with role mappings and FR-014 audit events`

### Slice 5: API Config Endpoint Returns Install-Time Settings (Viewability, Dependency on Slice 4)
- **What**: Extend `api/cmd/main.go` with a new flag `--game-data-storage-class` (string, env fallback `GAMEPLANE_GAME_DATA_STORAGE_CLASS`, defaults to empty), wired to a field in the API config struct. This flag is purely informational and read-only — the API does not use it for any operational purpose. Extend the `GET /admin/config` handler's response to include a new optional top-level field `installTimeSettings` containing `gameDataStorageClass` (from the new flag) and `oidcHelmProvider` (with groupsClaim, roleMappings, defaultRole from Slice 4). The field is populated from environment variables or runtime config (read-only; cannot be written via `PUT /admin/config`). Add envtest cases verifying the field is present and correct, and that attempting to write it is a no-op or silently ignored.
- **Touch Points**: `api/cmd/main.go` (add --game-data-storage-class flag with env fallback + struct field + Helm template binding), `api/internal/handlers/config.go` (getAll handler, add InstallTimeSettings struct + population logic), `web/src/lib/config.ts` (extend AllConfig interface + type InstallTimeSettings). Add 3 envtest test cases.
- **Why fifth**: Depends on Slice 4 (OIDC config must exist) and Slice 3 (storage class flag must be defined in Helm values). The operator-side plumbing (Slices 1–3) injects the storage class into PVCs; the API-side plumbing here reports it for operator transparency. Enables SC-006 (operators can view settings via admin interface).
- **Priority**: P1 (transparency requirement).
- **Commit message pattern**: `feat(api): expose install-time settings (storage class + OIDC mappings) in GET /admin/config`

### Slice 6: Web Dashboard Read-Only Display of Install-Time Settings (Dashboard, Dependency on Slice 5)
- **First step (Design pass)**: Update `design.pen` via the `pencil` MCP server to add (or refine) the AdminSettings install-time-settings display section. Export the touched node(s) via `pencil` MCP to `design-export/json/<id>.json` and `design-export/screenshots/<id>.png` in the same commit.
- **What**: Translate the design into React. Add a new conditional section to `/web/src/routes/AdminSettings.tsx` (read-only card/pane) displaying `installTimeSettings.gameDataStorageClass` and `installTimeSettings.oidcHelmProvider` if present in the config response. Style as informational labels (no edit controls), matching the Pencil design. Add a small note explaining these are set at install time via Helm and cannot be changed through the dashboard. Add vitest case verifying the section renders when data is present.
- **Touch Points**: (Design export) `design-export/json/` and `design-export/screenshots/` (re-export after Pencil edit); `web/src/routes/AdminSettings.tsx` (add new section to sections array + conditional render), `web/src/routes/AdminSettings.test.tsx` (test section renders), `web/specs.md` (document the new section UI pattern).
- **Why sixth**: Depends on Slice 5 (config endpoint must return the data). Design pass is required per Principle II (Constitution Check). Final frontend touch; can ship independently after Slice 5 merges.
- **Priority**: P1 (transparency UI).
- **Commit message pattern**: `feat(web): display install-time settings in AdminSettings (design + implementation)`

### Slice 7: OIDC Role Mapping Admin Warning & Confirm (Dashboard, Dependency on Slice 4)
- **What**: In the existing OIDC provider role-mapping editor in `/web/src/routes/AdminSettings.tsx`, add a warning + confirm dialog when an operator attempts to save a mapping whose target role is "admin". The warning must state: "Mapping users to the admin role grants full cluster control. Ensure the mapped group contains only authorized personnel. This action cannot be reversed through the dashboard." Require an explicit confirmation click to proceed. Since Gameplane cannot enumerate IdP group membership, the warning is unconditional on all admin mappings (not conditional on size). Add vitest case verifying the warning appears and the confirm step works.
- **Touch Points**: `web/src/routes/AdminSettings.tsx` (add warning + confirm modal to the OIDC mapping edit flow), `web/src/routes/AdminSettings.test.tsx` (test warning and confirm UX), `web/specs.md` (document the warning pattern).
- **Why seventh**: Depends on Slice 4 (OIDC provider + role mappings exist). This is a UX safety enhancement independent from install-time settings display (Slice 6). Addresses FR-015 requirement.
- **Priority**: P1 (security UX).
- **Commit message pattern**: `feat(web): add warning and confirm step for admin OIDC role mappings (FR-015)`

### Slice 8: E2E Tests for Storage Class (Operator Bucket)
- **What**: Add 3 e2e tests to `test/e2e/gameserver_e2e_test.go` in the `operator` bucket: (1) test that GameServer PVC is created with the Helm-configured default storage class; (2) test that a nonexistent storage class results in the GameServer entering Pending and the dashboard showing the error; (3) test that an explicit GameServer or GameTemplate override takes precedence over the default. All tests use unique resource names, call `t.Parallel()`. Register tests in `test/e2e/buckets.sh` operator bucket (lines 36–87).
- **Touch Points**: `test/e2e/gameserver_e2e_test.go` (3 new test functions), `test/e2e/buckets.sh` (register 3 tests).
- **Why eighth**: Depends on Slices 1–3 (operator + Helm chart changes must be in place). No login pressure (zero admin logins, pure operator reconciliation).
- **Priority**: P2 (E2E verification of storage class feature).
- **Commit message pattern**: `test(e2e): add storage class configuration and error-handling e2e tests`

### Slice 9: E2E Tests for OIDC Role Mapping (API-Auth Bucket)
- **What**: Add 3 e2e tests to `test/e2e/api_auth_e2e_test.go` in the `api-auth` bucket: (1) bootstrap install with OIDC role mappings configured, user logs in and is assigned the correct role (admin, operator, viewer); (2) user's role is re-evaluated on second login if their OIDC group has changed; (3) user with no matching group receives the default role. Tests reuse the bootstrap session from TestAPI_BootstrapAndLogin to conserve login budget (~1–2 additional logins per test, within the ~7-login budget per ci job). Fake OIDC issuer via newFakeIDP(t, ...) helper. Register tests in `test/e2e/buckets.sh` api-auth bucket (lines 89–98).
- **Touch Points**: `test/e2e/api_auth_e2e_test.go` (3 new test functions), `test/e2e/buckets.sh` (register 3 tests).
- **Why ninth**: Depends on Slice 4 (OIDC Helm provider must exist). Can be merged after Slice 4 but grouped with Slice 8 for review.
- **Priority**: P1 (critical feature coverage, OIDC-only scenario).
- **Commit message pattern**: `test(e2e): add OIDC role mapping assignment and re-evaluation e2e tests`

### Slice 10: Documentation & Module Specs.md Updates
- **What**: (1) Create or extend `docs/oidc.md` with per-IdP (Okta, Azure AD, Keycloak) walkthrough for configuring role mappings and claim names. (2) Update `docs/install.md` to document the new Helm values `operator.gameDataStorage.storageClassName` and OIDC-related flags. (3) Update `docs/security.md` to explain the OIDC role-mapping security model and Helm-vs-runtime coexistence. (4) Update `api/specs.md` to detail the new OIDC CLI flags, provider synthesis, and audit event emission (FR-014). (5) Update `operator/specs.md` to document PVC StorageClass selection and the install-time default injection. (6) Update `web/specs.md` to document the installTimeSettings display section and the admin-mapping warning UI pattern (FR-015).
- **Touch Points**: `docs/oidc.md` (new or extended), `docs/install.md`, `docs/security.md`, `api/specs.md`, `operator/specs.md`, `web/specs.md`.
- **Why last**: Depends on Slices 1–9 (all features must be implemented before documentation is final). Can be authored in parallel with preceding slices, merged last.
- **Priority**: P1 (user-facing docs required for feature adoption).
- **Commit message pattern**: `docs: document install-time storage class, OIDC role mapping, and audit trail configuration`

Each slice is independently testable and committable. Slices 1–3 form the storage class feature (P2), Slices 4–6 form the OIDC feature (P1), Slice 7 is the FR-015 warning UX, and Slices 8–10 provide verification and documentation. Dependencies flow: 1→2→3 (storage), 4→5→6 (OIDC + display), 7 (warning, depends on 4), 8 (e2e storage, depends on 1–3), 9 (e2e OIDC, depends on 4), 10 (docs, depends on all).
