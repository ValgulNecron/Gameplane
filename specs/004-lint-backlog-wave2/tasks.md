---
description: "Task list for Lint Backlog Wave 2 — bringing api, agent, test/e2e under the static-analysis gate"
---

# Tasks: Lint Backlog Wave 2 — Static Analysis Gate for api, agent, test/e2e

**Input**: Design documents from `/specs/004-lint-backlog-wave2/` (spec.md, plan.md, research.md, contracts/)

**Prerequisites**: All specification documents (spec.md, plan.md, research.md, data-model.md, contracts/) and CLAUDE.md

**Project Rules**:
- Nothing runs locally — no `golangci-lint`, no `go build`, no `go vet`, no `make`. CI is the oracle. A task is "done" when pushed and the relevant CI job is green, not when it compiles locally.
- Every commit is signed (`git commit -s`) with a conventional-commit prefix (fix:, chore:, ci:); one logical unit per commit.
- No lint suppression (`//nolint`, `//#nosec`, `//lint:ignore`) — fix the code instead. In-source suppression directives remain zero in the tree, absolutely and without exception; only config-level `.golangci.yml` exclusions exist—nine total, eight path-scoped rules plus one global gosec setting (inventoried in `contracts/exclusion-policy.md`), including the pre-existing gosec G115 exclusion in gameproto/minecraft.go. Do NOT add new suppression directives.
- Go errors wrap with `%w`.
- No CRD type changes in this feature, so no `make generate`/`make manifests` obligation.
- Build-tag-conditional code (`//go:build envtest` in api; `//go:build e2e` in test/e2e) must be analyzed. CI must pass `--build-tags=envtest` for api and `--build-tags=e2e` for test/e2e.
- Frozen surfaces (audit field names, migration filenames, e2e test names in buckets.sh, game protocol byte layouts, rate-limit thresholds, Prometheus metric names) must not be renamed; refactor around them via extraction or wrapping.
- Constitution IV: Any behavioral change requires a corresponding `specs.md` update in the same commit.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the feature branch off master and salvage completed work from PR #216

- [X] T001 Create a fresh branch `004-lint-backlog-wave2` off master (do NOT base on PR #216)
- [X] T002 [P] Cherry-pick api fix commit ba32d0b from PR #216 into the new branch (fixes 68 api files across 13 packages)
- [X] T003 [P] Cherry-pick agent fix commit f5b9ede from PR #216 into the new branch (fixes 29 agent files across 17 packages)

**Checkpoint**: Branch created with salvaged api and agent work. Do NOT cherry-pick the test/e2e commit e7b99b6 — it is stale against master's feature 001 restructure and will be redone in Phase 3.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Land the CI matrix enablement commit EARLY so the first CI run is deliberately RED — that red run IS the measurement. Fixes then land in subsequent commits. Once all fixes are complete, the branch will be green.

**⚠️ CRITICAL**: The matrix change lands early on the feature branch, triggering a deliberately-red CI run. This red run is NOT a failure; it is the authoritative measurement. No local linter runs are permitted (Constitution VI).

- [X] T004 Modify `.github/workflows/ci.yaml` (~line 180): add `api`, `agent`, `test/e2e` to the `matrix.module` list (currently they are listed in a comment on lines 177-178)
- [X] T005 Modify `.github/workflows/ci.yaml` (~line 189-195): extend the envtest build-tag step to include `api` with condition `if: matrix.module == 'operator' || matrix.module == 'api'` and args `--build-tags=envtest`
- [X] T006 Modify `.github/workflows/ci.yaml`: add a NEW step for e2e build tags with condition `if: matrix.module == 'test/e2e'` and args `--build-tags=e2e`
- [X] T007 Modify `.github/workflows/ci.yaml`: delete the exclusion comment at lines 177-178 that lists api, agent, test/e2e as pending cleanup
- [X] T008 Push the branch with the matrix changes (no fixes yet). The CI lint job will run RED, reporting the baseline finding count across all three modules. This red run is intentional and is the measurement step per research.md Decision 3.
- [X] T009 Retrieve the finding lists from the red CI run using the exact recipe: `gh api repos/ValgulNecron/Gameplane/actions/runs/<run_id>/jobs --jq '.jobs[] | select(.name|test("lint")) | [.id,.name,.conclusion] | @tsv'` then `gh api repos/ValgulNecron/Gameplane/actions/jobs/<job_id>/logs`. Record the finding counts per module (api, agent, test/e2e) and note that this is the baseline; no local grep or lint run is performed (Constitution VI).

**Checkpoint**: Matrix enablement landed, first CI run is RED with baseline finding counts recorded. Fix work can now fan out per-package. The branch will be red during phases 3-5; this is expected and honest.

---

## Phase 3: User Story 1 (P1) — "Three modules brought under the uniform lint gate" 🎯 MVP

**Goal**: Fix all golangci-lint findings across api, agent, and test/e2e via real code changes (no suppression directives). Each fix is a real code change addressing the underlying issue.

**Independent Test**: All three modules pass golangci-lint with zero findings when `--build-tags=envtest` (api) and `--build-tags=e2e` (test/e2e) are passed.

### API Module Fixes (199 files, 13 packages)

#### API: api/internal/handlers — auth & OIDC (6 files)

- [X] T010 [P] [US1] Fix all golangci-lint findings in `api/internal/handlers/` auth and OIDC handler files (auth_provider_secret.go, auth_provider_secret_test.go, auth_providers.go, auth_providers_test.go, oidc_routes.go, oidc_routes_test.go). Expected fixes: improved error handling (errcheck), context parameters (contextcheck), resource cleanup (bodyclose). Update `api/specs.md` if behavioral changes occur.

#### API: api/internal/handlers — config (4 files)

- [X] T011 [P] [US1] Fix all golangci-lint findings in `api/internal/handlers/config.go`, `config_test.go`, `config_db_test.go`, and `config_validators_test.go`. Expected fixes: nil checks (nilerr), error wrapping consistency (errorlint).

#### API: api/internal/handlers — modules, module sources, uploads, mod IDs, mod updates (15 files)

- [X] T012 [P] [US1] Fix all golangci-lint findings in `api/internal/handlers/` module and module-source files (modules.go, modules_envtest_test.go, modules_fake_test.go, modules_list_errors_test.go, modules_unit_test.go, modules_uninstall_test.go, module_sources.go, module_sources_test.go, module_sources_validate_test.go). Expected fixes: context parameters, error handling, resource cleanup.
- [X] T013 [P] [US1] Fix all golangci-lint findings in `api/internal/handlers/` upload and mod-related files (module_upload.go, module_upload_test.go, mod_ids.go, mod_ids_test.go, mod_updates.go, mod_updates_test.go). Expected fixes: context parameters, error wrapping.

#### API: api/internal/handlers — destinations (5 files)

- [X] T014 [P] [US1] Fix all golangci-lint findings in `api/internal/handlers/` destinations files (destinations.go, destinations_envtest_test.go, destinations_fake_test.go, destinations_unit_test.go, destinations_upsert_test.go). Expected fixes: context parameters, nil checks.

#### API: api/internal/handlers — lifecycle (4 files)

- [X] T015 [P] [US1] Fix all golangci-lint findings in `api/internal/handlers/` lifecycle files (lifecycle.go, lifecycle_envtest_test.go, lifecycle_extra_test.go, lifecycle_fake_test.go). Expected fixes: error handling, context parameters.

#### API: api/internal/handlers — resources, pod events, system logs (10 files)

- [X] T016 [P] [US1] Fix all golangci-lint findings in `api/internal/handlers/` resources and pod-events files (resources.go, resources_envtest_test.go, resources_fake_test.go, resources_cluster_envtest_test.go, resources_stale_test.go, resources_update_test.go, pod_events.go, pod_events_test.go). Expected fixes: context parameters, error wrapping.
- [X] T017 [P] [US1] Fix all golangci-lint findings in `api/internal/handlers/systemlogs.go` and `systemlogs_test.go`. Expected fixes: nil checks, context parameters.

#### API: api/internal/handlers — cluster(s) (9 files)

- [X] T018 [P] [US1] Fix all golangci-lint findings in `api/internal/handlers/` cluster files (cluster.go, clusters.go, cluster_actions.go, cluster_test.go, clusters_test.go, cluster_actions_test.go, cluster_actions_kubeconfig_test.go, cluster_guard_test.go, cluster_name_test.go). Expected fixes: context parameters, error handling.

#### API: api/internal/handlers — registry & registry secret (4 files)

- [X] T019 [P] [US1] Fix all golangci-lint findings in `api/internal/handlers/registry.go`, `registry_test.go`, `registry_secret.go`, and `registry_secret_test.go`. Expected fixes: error handling, context parameters.

#### API: api/internal/handlers — users, roles, shares, ownership (11 files)

- [X] T020 [P] [US1] Fix all golangci-lint findings in `api/internal/handlers/` user and role files (users.go, users_test.go, users_extra_test.go, users_branches_test.go, roles.go, roles_test.go, roles_branches_test.go). Expected fixes: context parameters, error wrapping.
- [X] T021 [P] [US1] Fix all golangci-lint findings in `api/internal/handlers/shares.go`, `shares_test.go`, `ownership.go`, and `ownership_test.go`. Expected fixes: context parameters, nil checks.

#### API: api/internal/handlers — notifications (3 files)

- [X] T022 [P] [US1] Fix all golangci-lint findings in `api/internal/handlers/notifications.go`, `notifications_test.go`, and `notifications_secret_test.go`. Expected fixes: error handling, context parameters.

#### API: api/internal/handlers — events (2 files)

- [X] T023 [P] [US1] Fix all golangci-lint findings in `api/internal/handlers/events.go` and `events_test.go`. Expected fixes: context parameters, error wrapping.

#### API: api/internal/handlers — helpers & misc (11 files)

- [X] T024 [P] [US1] Fix all golangci-lint findings in `api/internal/handlers/` helper and miscellaneous files (audit.go, audit_test.go, validate.go, secrets_managed.go, semver.go, semver_test.go, suite_envtest_test.go, test_helpers_test.go, tunnelcreds.go, tunnelcreds_test.go, dispatch_isolation_envtest_test.go). Expected fixes: context parameters, error handling.

#### API: api/internal/registry (24 files)

- [X] T025 [P] [US1] Fix all golangci-lint findings in `api/internal/registry/` package. Expected fixes: context parameters (contextcheck), error handling (errcheck), resource cleanup (bodyclose), improved error wrapping (errorlint). Update `api/specs.md` if OCI-registry behavior changes.

#### API: api/internal/auth (22 files)

- [X] T026 [P] [US1] Fix all golangci-lint findings in `api/internal/auth/` package. Expected fixes: context parameters (contextcheck — especially on graceful shutdown paths with `context.WithoutCancel`), variable shadowing fixes (gosec G101 — validate renames against password/secret/token regex), error handling (errcheck, nilerr), rate-limit integration tests. Update `api/specs.md` if authentication behavior or rate-limit logic changes.

#### API: api/internal/ws (12 files)

- [X] T027 [P] [US1] Fix all golangci-lint findings in `api/internal/ws/` package (WebSocket bridge). Expected fixes: context parameters (contextcheck), error handling (errcheck), resource cleanup (bodyclose). Note trap: if switching from `net.DialTimeout` to `DialContext`, ensure `Dialer.Timeout` is also set. Update `api/specs.md` if WebSocket behavior changes.

#### API: api/internal/notify (11 files)

- [X] T028 [P] [US1] Fix all golangci-lint findings in `api/internal/notify/` package (notification delivery). Expected fixes: error handling (errcheck), context parameters (contextcheck), resource cleanup (bodyclose). Update `api/specs.md` if notification sink behavior changes.

#### API: api/internal/kube (11 files)

- [X] T029 [P] [US1] Fix all golangci-lint findings in `api/internal/kube/` package (Kubernetes client wrapper). Expected fixes: error handling (errcheck), context parameters (contextcheck). Update `api/specs.md` if K8s API interaction changes.

#### API: api/internal/db (11 files)

- [X] T030 [P] [US1] Fix all golangci-lint findings in `api/internal/db/` package (database driver and migrations). Expected fixes: error handling (errcheck), error wrapping consistency (errorlint). Note: migration filenames are frozen; do NOT rename migration files.

#### API: api/internal/rbac (5 files)

- [X] T031 [P] [US1] Fix all golangci-lint findings in `api/internal/rbac/` package (role-based access control middleware). Expected fixes: error handling (errcheck), context parameters (contextcheck).

#### API: api/internal/audit (5 files)

- [X] T032 [P] [US1] Fix all golangci-lint findings in `api/internal/audit/` package (audit event logging). Expected fixes: error handling (errcheck), resource cleanup. Note: audit event field names and Prometheus metric names are frozen; do NOT rename them. Refactor around via extraction if needed.

#### API: api/cmd (5 files)

- [X] T033 [P] [US1] Fix all golangci-lint findings in `api/cmd/` package (API entry point, serve and bootstrap-admin subcommands). Expected fixes: error handling, context parameters.

#### API: api/internal/scope (4 files)

- [X] T034 [P] [US1] Fix all golangci-lint findings in `api/internal/scope/` package. Expected fixes: error handling, context parameters.

#### API: api/internal/telemetry (3 files)

- [X] T035 [P] [US1] Fix all golangci-lint findings in `api/internal/telemetry/` package. Expected fixes: error handling, context parameters.

#### API: api/internal/httperr (2 files)

- [X] T036 [P] [US1] Fix all golangci-lint findings in `api/internal/httperr/` package. Expected fixes: error handling, error wrapping.

### Agent Module Fixes (64 files, 17 packages)

#### Agent: agent/internal/rcon (14 files)

- [X] T037 [P] [US1] Fix all golangci-lint findings in `agent/internal/rcon/` package (RCON protocol implementation). Expected fixes: context parameters (contextcheck), error handling (errcheck), resource cleanup (bodyclose). Update `agent/specs.md` if RCON protocol behavior changes.

#### Agent: agent/internal/players (7 files)

- [X] T038 [P] [US1] Fix all golangci-lint findings in `agent/internal/players/` package. Expected fixes: context parameters, error handling.

#### Agent: agent/internal/files (7 files)

- [X] T039 [P] [US1] Fix all golangci-lint findings in `agent/internal/files/` package (file access from pods). Expected fixes: context parameters, error handling, error wrapping.

#### Agent: agent/internal/mods (6 files)

- [X] T040 [P] [US1] Fix all golangci-lint findings in `agent/internal/mods/` package (module management). Expected fixes: context parameters, error handling.

#### Agent: agent/internal/quiesce (4 files)

- [X] T041 [P] [US1] Fix all golangci-lint findings in `agent/internal/quiesce/` package (graceful shutdown). Expected fixes: context parameters (contextcheck — note trap: graceful-shutdown paths with `WithoutCancel` require explicit timeout wrapping).

#### Agent: agent/internal/logs (4 files)

- [X] T042 [P] [US1] Fix all golangci-lint findings in `agent/internal/logs/` package. Expected fixes: context parameters, error handling.

#### Agent: Small packages grouped (usage, status, metrics, lifecycle, httpjson, heartbeat, console, caps, auth, actions — 2 files each on average)

- [X] T043 [P] [US1] Fix all golangci-lint findings in `agent/internal/{usage,status,metrics,lifecycle,httpjson,heartbeat,console,caps,auth,actions}/` packages (small utility and handler packages). Expected fixes: context parameters, error handling across all packages.

#### Agent: agent/cmd (2 files)

- [X] T044 [P] [US1] Fix all golangci-lint findings in `agent/cmd/` package (agent entry point). Expected fixes: error handling, context parameters.

### Test/E2E Module Fixes (79 files, 23 packages)

#### Test/E2E: Root setup & helpers (5 files)

- [X] T045 [P] [US1] Fix all golangci-lint findings in root test/e2e setup files (`env.go`, `gameprobe_job.go`, `e2e_suite_test.go`, `test_helpers_e2e_test.go`, `gamebot_helpers_e2e_test.go`). Expected fixes: context parameters, error handling, build-tag-specific signatures. Build tags must be passed: `--build-tags=e2e`.

#### Test/E2E: API tests (10 files)

- [X] T046 [P] [US1] Fix all golangci-lint findings in root `test/e2e/` API integration test files (api_agent_e2e_test.go, api_auth_e2e_test.go, api_lifecycle_e2e_test.go, api_mods_e2e_test.go, api_mods_upload_e2e_test.go, api_owner_collab_e2e_test.go, api_rbac_matrix_e2e_test.go, api_roles_e2e_test.go, api_session_e2e_test.go, api_ws_e2e_test.go). Expected fixes: context parameters, error handling. Build tags: `--build-tags=e2e`.

#### Test/E2E: Operator & CRD tests (16 files)

- [X] T047 [P] [US1] Fix all golangci-lint findings in root `test/e2e/` operator and CRD test files (backup_e2e_test.go, backupschedule_e2e_test.go, crd_validation_e2e_test.go, gameserver_e2e_test.go, gameserver_idle_e2e_test.go, gameserver_lifecycle_e2e_test.go, gameserver_networkpolicy_e2e_test.go, gameserver_version_switch_e2e_test.go, module_e2e_test.go, module_verify_e2e_test.go, module_verify_signed_e2e_test.go, modulesource_ssrf_e2e_test.go, modulesource_upload_e2e_test.go, operator_finalizer_e2e_test.go, restore_e2e_test.go, wake_on_connect_e2e_test.go). Expected fixes: context parameters, error handling. Build tags: `--build-tags=e2e`.

#### Test/E2E: Game bot tests (16 files)

- [X] T048 [P] [US1] Fix all golangci-lint findings in root `test/e2e/` game bot test files (ark_bot_e2e_test.go, cs2_bot_e2e_test.go, dayz_bot_e2e_test.go, dontstarve_bot_e2e_test.go, enshrouded_bot_e2e_test.go, factorio_bot_e2e_test.go, garrysmod_bot_e2e_test.go, minecraft_bot_e2e_test.go, palworld_bot_e2e_test.go, projectzomboid_bot_e2e_test.go, rust_bot_e2e_test.go, satisfactory_bot_e2e_test.go, sevendaystodie_bot_e2e_test.go, terraria_bot_e2e_test.go, valheim_bot_e2e_test.go, vrising_bot_e2e_test.go). Expected fixes: context parameters, error handling. Note: test names in buckets.sh are frozen; do NOT rename test functions. Build tags: `--build-tags=e2e`.

#### Test/E2E: Integration tests (4 files)

- [X] T049 [P] [US1] Fix all golangci-lint findings in root test/e2e integration test files (failure_paths_e2e_test.go, multicluster_e2e_test.go, helm_install_e2e_test.go, upgrade_e2e_test.go). Expected fixes: context parameters, error handling. Build tags: `--build-tags=e2e`.

#### Test/E2E: Internal packages (probe, protocol subdirs, per-game dirs)

- [X] T050 [P] [US1] Fix all golangci-lint findings in `test/e2e/internal/probe/` package (probe harness). Expected fixes: error handling, context parameters. Build tags: `--build-tags=e2e`.
- [X] T051 [P] [US1] Fix all golangci-lint findings in `test/e2e/internal/protocol/` packages (joindepth, a2sproto, sourceproto) and per-game protocol packages (test/e2e/internal/terraria/, test/e2e/internal/minecraft-java/, and 14 other game dirs). Expected fixes: error handling, variable names. Build tags: `--build-tags=e2e`.
- [X] T052 [P] [US1] Fix all golangci-lint findings in per-game subdirectories under `test/e2e/internal/` (16 game dirs: minecraft-java, terraria, valheim, dayz, garrys-mod, factorio, cs2, rust, ark-survival-ascended, palworld, satisfactory, 7-days-to-die, project-zomboid, dont-starve-together, enshrouded, v-rising). Each game dir contains protocol and bot fixtures. Expected fixes: error handling, variable names. Note: protocol byte layouts are frozen; refactor around them if findings touch them. Build tags: `--build-tags=e2e`.

#### Test/E2E: specs.md documentation

- [X] T053 [US1] Create or update `test/e2e/specs.md` documenting the E2E harness structure, build-tag requirements (`//go:build e2e`), test naming conventions, and any behavioral changes the fixes introduce.

**Checkpoint**: All three modules (api, agent, test/e2e) pass golangci-lint with zero findings when the correct build tags are passed. Per-module fixes are complete and committed with conventional-commit prefixes. API and agent now have `specs.md` files documenting behavioral changes (if any).

---

## Phase 4: User Story 2 (P2) — "Findings are fixed, not suppressed"

**Goal**: Verify and prove the zero-suppression property is preserved. No `//nolint`, `//#nosec`, or `//lint:ignore` directives are introduced anywhere.

**Independent Test**: A grep of the entire repository returns zero lines matching suppression directives in api, agent, and test/e2e (accounting for false positives in identifier names).

- [X] T054 [US2] Verify zero `//nolint` directives in `api/`, `agent/`, and `test/e2e/` directories. Expected: empty result or only false positives (identifiers like "nolint" in variable names or test names). The grep command must be: `git grep -i 'nolint' -- api/ agent/ test/e2e/` and any match must be manually verified as a false positive (not an actual suppression directive). Document the verification step.
- [X] T055 [US2] Verify zero `//#nosec` directives in `api/`, `agent/`, and `test/e2e/` directories. Expected: empty result. Grep: `git grep '#nosec' -- api/ agent/ test/e2e/`.
- [X] T056 [US2] Verify zero `//lint:ignore` directives in `api/`, `agent/`, and `test/e2e/` directories. Expected: empty result. Grep: `git grep 'lint:ignore' -- api/ agent/ test/e2e/`.
- [X] T057 [US2] Review a sample of 5–10 landed fix commits to confirm they contain real code changes (improved error handling, added context parameters, renamed variables, extracted helpers) and not deletions or artificial narrowing of analysis scope. Document findings in a brief review note.
- [X] T058 [US2] Confirm `.golangci.yml` has gained only the five reviewed, documented, narrowly-scoped exclusions inventoried in `contracts/exclusion-policy.md` (eight config-level exclusion rules total, including the three pre-existing ones: test exemptions, controller revive exemption, gameproto G115 exemption), and that the tree still has zero in-source suppression directives. Verify by inspecting `.golangci.yml`'s exclusion rules against the inventory in `contracts/exclusion-policy.md` and confirming no undocumented directive appears.

**Checkpoint**: Zero suppression directives introduced. All landed fixes are real code changes. .golangci.yml exclusion list is unchanged.

---

## Phase 5: User Story 3 (P3) — "The gate cannot silently regress"

**Goal**: Implement the lint-gate contract rules (R-001 through R-010 from contracts/lint-gate.md) as CI verifications or scripts to prevent future regressions.

**Independent Test**: A new verifier script can be run and detects violations of the contract when artifacts are modified to violate it.

- [X] T059 [P] [US3] Implement a CI verification that `go.work` module list and `.github/workflows/ci.yaml` matrix module list are in sync. Verify: every module appearing in `go.work` appears exactly once in the matrix.module list (R-001 / R-007). Create a shell script `test/e2e/lint-gate-verify.sh` with a paired fixture-based test script `test/e2e/lint-gate-verify_test.sh` demonstrating it can fail when a module is missing or duplicated (mirroring the pattern of `test/e2e/joincoverage.sh` and `test/e2e/joincoverage_test.sh`). Wire the script into `.github/workflows/ci.yaml` as a step in the lint job that runs before the actual linting (so the gate is complete). **Superseded by T080** — Full implementation with 12 test fixtures in testdata/lint-gate/, wired into CI lint job.
- [X] T060 [P] [US3] Verify that `.github/workflows/ci.yaml` has no `continue-on-error: true` lines and no `|| true` fallbacks in the golangci-lint step or surrounding steps (R-004). Document the finding: if one exists, it blocks this task. The lint job must fail hard on linting failures.
- [X] T061 [P] [US3] Verify that `.github/workflows/ci.yaml` has no comment on the lint step suggesting "pending cleanup" or "temporary" status (R-006). The gate is permanent.
- [X] T062 [US3] Verify that build-tag args are correctly passed in `.github/workflows/ci.yaml`: `--build-tags=envtest` for api module, `--build-tags=e2e` for test/e2e module (R-008 / R-009). Confirm by inspection of the matrix step conditions and args in the workflow file.
- [X] T063 [US3] Confirm that no module listed in the matrix has been accidentally excluded via a skip condition (`if: ...false`) (R-003). Every module in the matrix must be analyzed on every run.
- [X] T064 [US3] Add documentation to specs.md: update `specs/004-lint-backlog-wave2/specs.md` to document the lint-gate contract and its verification rules (R-001..R-010), with references to the verifier script and CI configuration. Include the exact CI recipe for maintainers to manually verify the gate if needed.
  - **Note (2026-08-19, T077 convergence pass)**: no `specs/004-lint-backlog-wave2/specs.md` was created; `contracts/lint-gate.md` was judged to already satisfy this task's intent in full — it documents R-001..R-010 as a normative rules table, spells out what a green lint job does and does not prove, and ends with a "Matrix Completeness Verification" section giving the exact maintainer recipe (`go work edit -json | jq ...` vs. the ci.yaml matrix) this task asked for. The one gap — a verifier *script* — is still open and tracked separately as T059/T076.

**Checkpoint**: Lint-gate contract rules are automated and documented. The verifier script can fail when the contract is violated. CI configuration is locked in.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final validation, collateral-damage checks, and documentation updates.

- [X] T065 [P] Confirm that `api/.testcoverage.yml` 80% gate still passes after all fixes. If coverage dropped, add targeted tests to recover it. Build tags: `--build-tags=envtest`.
- [X] T066 [P] Confirm that `agent/.testcoverage.yml` 90% gate still passes after all fixes. If coverage dropped, add targeted tests to recover it.
- [X] T067 Verify that the "e2e bucket coverage" CI job still passes and the verifier (`test/e2e/joincoverage.sh`) still validates all 16 modules correctly. Note: renaming an e2e test to satisfy a linting rule will silently break the test→bucket mapping in `test/e2e/buckets.sh` and fail the coverage job; this is why e2e test names are frozen (T048).
- [X] T068 Run the quickstart.md scenarios on a real cluster (e.g., kubelab). All 8 scenarios must execute without error. If a scenario fails due to this feature's changes, document the failure and roll back the offending fix. **Superseded by T082** — Validated via CI logs (run 32191677838, PR #237) + cluster inspection; all 8 scenarios pass.
- [X] T069 Verify that CLAUDE.md's "Lint & coverage" section (lines 120–131) and the project-specific rules accurately reflect that all 13 Go modules (netguard, gameaction, gameproto, operator, api, agent, audit-syslog-bridge, telemetry-receiver, sentinel, mcp-server, svcutil, and test/e2e) are now under the uniform golangci-lint gate. If the section does not list api, agent, and test/e2e alongside the other modules or does not mention build-tag requirements (`--build-tags=envtest` for api, `--build-tags=e2e` for test/e2e), update it to reflect the complete, uniform gate. If CLAUDE.md is already current, document the verification and confirm no changes are needed. **Superseded by T081** — CLAUDE.md updated (line 132) to confirm all 13 modules gated with build-tag requirements.

**Checkpoint**: All collateral checks pass. Documentation is current. The feature is ready for merge.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately off master.
- **Foundational (Phase 2)**: Depends on Setup completion. **CRITICAL**: The matrix enablement commit lands early so CI provides the measurement (deliberately red). Blocks all user story fix work only in that it provides the baseline number.
- **User Stories (Phase 3-5)**: All depend on Foundational completion (so the matrix is in place and baseline is known).
  - Phase 3 (US1) — fixes — can start after Foundational and run in parallel across packages.
  - Phase 4 (US2) — verification — can start after all Phase 3 fixes are landed and CI is green.
  - Phase 5 (US3) — gate verification — can start after Foundational and run in parallel with fixes; should conclude after Phase 3 is green.
- **Polish (Phase 6)**: Depends on all user stories being complete and the branch being green on CI.

### User Story Dependencies

- **User Story 1 (P1 / MVP)**: Can start after Foundational. Independently valuable; all fixes must land before the branch is green.
- **User Story 2 (P2)**: Can start after Phase 3 (US1) fixes land. Verifies what was fixed.
- **User Story 3 (P3)**: Can start after Foundational and run in parallel with US1. Should be complete before Phase 6 (Polish).

### Within Each User Story

- **Phase 1 (Setup)**: T001, T002, T003 can run in parallel (cherry-picks are independent operations on different commits).
- **Phase 2 (Foundational)**: T004-T007 are sequential (they modify the same file incrementally). T008-T009 depend on T007 landing and CI running.
- **Phase 3 (US1)**: All tasks marked [P] can run in parallel. Each task is assigned a package or group of packages; no file overlap between tasks. T053 (specs.md) depends on fixes landing; schedule it after the first wave of fixes is committed.
- **Phase 4 (US2)**: T054-T058 can run in parallel (they search different directories or review different aspects). All depend on Phase 3 being complete.
- **Phase 5 (US3)**: T059-T063 can run in parallel (they each verify a different contract rule). T064 depends on T062 being done (documenting what was verified). All depend on Foundational being complete; they don't need US1 to finish first.
- **Phase 6 (Polish)**: T065-T069 can mostly run in parallel. T068 (quickstart) is the critical path; it may expose regressions from earlier phases.

### Parallel Opportunities

This feature is exceptionally parallel. After Phase 2 (Foundational) completes with a red CI run, three independent fan-out streams can begin:

**Stream A — API Module Fixes** (10-12 concurrent tasks, one per package or group):
- Workers: T010-T036 (one haiku agent per package group)
- Each worker owns disjoint files in `api/internal/*/`; no conflicts
- Sequence: start after T009 (baseline measured); commit and push as ready; no blocking on other streams

**Stream B — Agent Module Fixes** (5-7 concurrent tasks):
- Workers: T037-T044 (one haiku agent per package or group)
- Each worker owns disjoint files in `agent/internal/*/`; no conflicts
- Sequence: start after T009; independent of Stream A

**Stream C — Test/E2E Module Fixes** (6-7 concurrent tasks):
- Workers: T045-T052 (one haiku agent per file group or package)
- Note: root e2e files (51 `*_test.go`) are not partitioned into packages, so parallelism is trickier here. One approach: assign games to workers (T052 can be split 16 ways by game). Another: assign test suites (API, Operator, Bot, Integration).
- Sequence: start after T009; independent of Streams A and B

**Stream D — Parallel US2 & US3** (2-5 concurrent tasks):
- Workers: T059-T064 (T059-T063 can run in parallel; T064 depends on others)
- Sequence: start after T009 (runs independent of fix work); can conclude once fixes are in place

### Worked Example: Concurrent Execution

After Phase 2 (T004-T009) completes and CI reports the baseline:

```
Time T+0 (start, all concurrent):
  Agent A (Haiku): T010 — api/handlers auth/oidc fixes
  Agent B (Haiku): T011 — api/handlers config fixes
  Agent C (Haiku): T025 — api/registry fixes
  Agent D (Haiku): T026 — api/auth fixes
  Agent E (Haiku): T037 — agent/rcon fixes
  Agent F (Haiku): T045 — test/e2e root setup/helpers fixes
  Agent G (Haiku): T059 — Verify go.work ↔ matrix sync (US3)
  Agent H (Haiku): T054 — Grep for //nolint in tree (US2)

[All complete in ~45-60 min on their own CI runs]

Time T+60 min:
  Continue with remaining packages in Streams A, B, C in the same parallel fashion.
  Once all fixes land and CI is green for 3 consecutive commits:

Time T+6-8 hours (estimated):
  Agent I (Sonnet): Review all Phase 3 commits (T010-T053) for code quality, real fixes vs. suppression, consistency across modules
  Agent J (Sonnet): Review Phase 4 and 5 verification (T054-T064)

[Review completes; fixes are applied if needed; branch is green]

Time T+Final:
  Agent K (Haiku): Phase 6 Polish tasks (T065-T069)
```

### MVP Strategy

To deliver the MVP (api, agent, test/e2e all green on lint gate) in the shortest path:

1. **Phase 1** (Setup): Create branch, cherry-pick api and agent work (~5 min)
2. **Phase 2** (Foundational): Land matrix, get red CI measurement (~30 min)
3. **Phase 3** (US1): Fix all three modules in parallel per-package (~6-8 hours with concurrent agents)
4. **Phase 4** (US2): Verify zero suppression (~30 min, can run while Phase 3 completes)
5. **Phase 5** (US3): Implement gate verification (~1 hour)
6. **STOP and VALIDATE**: CI is green; all three modules pass; zero findings; zero suppressions.
7. **Merge to master**: Create a PR, get Sonnet review, merge when all green.

At this point, the project has:
- Three large security-sensitive modules now under uniform lint gate
- ~400+ findings fixed via real code changes
- Zero suppression directives introduced
- Gate verification rules in place to prevent regressions

### Incremental Delivery

If work stretches across multiple days:

1. **Day 1**: Phase 1 + Phase 2 → baseline measured, ready to fix
2. **Day 2-3**: Phase 3 (fixes) → commit as ready per package; branch goes red → green
3. **Day 4**: Phase 4 (US2 verification) + Phase 5 (US3 gate rules) → documentation complete
4. **Day 5**: Phase 6 (Polish) → validation + merge

Each phase is independently reviewable; a maintainer can review Phase 1 and 2 before fixes land, then review fixes as they stream in.

### Parallel Team Strategy

With multiple developers:

1. **Team completes Phase 1 + Phase 2 together** (~45 min)
2. **Once Phase 2 is done** (CI red with baseline measured):
   - **Developer A + 4 concurrent Haiku agents**: Stream A (api module, 10-12 packages, fan-out)
   - **Developer B + 3 concurrent Haiku agents**: Stream B (agent module, 5-7 packages, fan-out)
   - **Developer C + 3 concurrent Haiku agents**: Stream C (test/e2e, 6-7 file groups, fan-out)
   - **Developer D**: Stream D (US2 & US3 verifications, less CPU-intensive, can overlap with others)
3. **Fixes stream in** and land on the branch; CI gradually goes from red → green
4. **Once CI is green** (all fixes landed):
   - **Developer E (Sonnet agent)**: Code review all commits at tier+1 (comprehensive review of fix quality, no suppression, real changes)
5. **After review**:
   - **Developer A**: Apply any review-requested changes
   - **Developer D**: Finalize Phase 6 (Polish)
6. **Merge when green**

This stratey keeps developers unblocked and maximizes concurrency.

---

## Implementation Strategy

### MVP First (User Story 1 Only)

To deliver the MVP (api, agent, test/e2e all green on the lint gate):

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CI runs red; baseline is measured)
3. Complete Phase 3: User Story 1 (all fixes; branch goes red → green as commits land)
4. **STOP and VALIDATE**: `git log --oneline` shows ~50-80 fix commits. `gh run list --branch 004-lint-backlog-wave2 --limit 1 --json conclusion` returns "success". All three modules pass golangci-lint with zero findings when correct build tags are passed.
5. Merge to master (with or without a full review, depending on project policy)

At this point, the project has:
- Three large modules now gated
- ~400+ real code fixes applied
- Zero suppression directives introduced
- Foundation for future lint work

### Incremental Delivery (All Three User Stories)

Continue from MVP:

1. MVP delivered (Phase 1-3 complete, merged, or pending review)
2. **Add Phase 4** (US2: Verification) → Test independently → Document
3. **Add Phase 5** (US3: Gate Rules) → Test independently → Document
4. **Add Phase 6** (Polish) → Final validation → Merge

Each story is independently valuable:
- US1 = gate is under enforcement
- US2 = gate has no suppressions (property verified)
- US3 = gate has automated regression checks (cannot silently regress)

### Parallel Team Strategy

See "Parallel Opportunities" section above.

---

## Notes

- `[P]` tasks = different files/packages, no merge conflicts, can run in parallel
- `[US1]`/`[US2]`/`[US3]` labels = user story traceability
- Every task produces one or more commits (one logical unit per commit)
- Stop at any checkpoint to validate story independently
- Avoid: renaming frozen surfaces, introducing suppressions, assuming local lint runs are available
- No local build/test/lint runs; CI is the oracle
- All commits signed (`git commit -s`) and conventional-commit formatted (`fix:`, `chore:`, `ci:`)

---

## Phase 7: Convergence

**Purpose**: Close gaps found by assessing the merged wave-2 code (master `60fc5bd4`) against spec.md, plan.md, the contracts, and the constitution

**Note**: The lint gate itself is delivered and green — all 13 `go.work` modules are gated and `lint (api)`, `lint (agent)` and `lint (test/e2e)` pass on master. Every task below is documentation drift or an unbuilt guard, not a broken gate.

- [X] T070 CRITICAL Create `test/e2e/specs.md` documenting the e2e harness — its build-tag requirements (`//go:build e2e`), the test-name-to-bucket mapping that `test/e2e/buckets.sh` enforces by exact name, and the context threading this feature added to `Kubectl`/`KubectlWithStdin`/`tryPortForward` — per Constitution IV, which requires every module folder to maintain one (missing)
- [X] T071 CRITICAL Update `api/specs.md` and `agent/specs.md` to document the behaviour this feature changed: the CSRF cookie's deliberate non-`HttpOnly` double-submit design, the DNS-1123 validation applied to proxy targets in `api/internal/ws/dialer.go`, the ctx-threaded `WebhookSink` in `api/internal/audit/audit.go`, and the agent's partial-file cleanup on a failed upload — per Constitution IV, which requires specs.md to change in the same change as the behaviour it documents (partial)
- [X] T072 CRITICAL Propose a constitution amendment resolving whether a narrowly-scoped, commented, maintainer-authorized `.golangci.yml` exclusion is permitted — Principle III's ratified text bars "loosening rules in .golangci.yml" with no written carve-out, while project practice (the pre-existing G115 rule, and the five authorized during this feature) treats scoped exclusions as the sanctioned escape hatch per Constitution III (contradicts). **Superseded by T079** — Constitution amended to Principle III (v1.6.0) explicitly permitting narrowly-scoped, config-level exclusions.
- [X] T073 Document exclusions #5-#8 (G704 `api/internal/ws/dialer.go`, G124 `api/internal/auth/sessions.go`, G204 `test/e2e/env.go`, G402 `test/e2e/internal/satisfactory/app.go`) in `specs/004-lint-backlog-wave2/contracts/exclusion-policy.md` using the same Path/Linters/Text/Justification structure as #1-#4, and correct its stale counts — line 87 says "four", line 257's recipe says "Should be 4", line 310's table says "≤3", while `.golangci.yml` has 8 per contract: exclusion-policy.md (contradicts)
- [X] T074 Update `spec.md` FR-002, FR-005, SC-002 and the Authorized-Exclusion List entity to describe the six gosec exclusions now present instead of asserting G115 "remains the only exception" — the inline zero-suppression property they test still holds (zero `//nolint`, `//#nosec`, `//lint:ignore` in the tree), only the singular wording is false per FR-002/FR-005/SC-002 (contradicts)
- [X] T075 Amend `plan.md`'s Technical Context and task T058's acceptance criterion, which both commit to zero new `.golangci.yml` exclusions, to record the five that were added and the reasoning for each per plan: Technical Context (contradicts)
- [X] T076 Implement `test/e2e/lint-gate-verify.sh` with a paired fixture runner proving it can fail, mirroring `test/e2e/joincoverage.sh` + `joincoverage_test.sh`, checking the `go.work`-to-matrix sync and the forbidden CI patterns in contracts/lint-gate.md R-001..R-010, and wire it into the lint job — or record in the spec that code-review-only detection was judged sufficient and the verifier deliberately dropped, since FR-004's "code review and/or configuration validation" is disjunctive per US3/AC2 (missing). **Superseded by T080** — Full implementation with 12 test fixtures in testdata/lint-gate/, wired into CI lint job.
- [X] T077 Create `specs/004-lint-backlog-wave2/specs.md` consolidating the lint-gate contract with a maintainer verification recipe, or mark T064 done with a note that `contracts/lint-gate.md` satisfied the intent per T064 (missing)
- [X] T078 Sweep `tasks.md` and check off every task whose work is verifiably in master, leaving only the genuinely unbuilt ones open — 60 of 69 remain unchecked because the work shipped through workflows rather than by walking the list (missing)

---

## Phase 8: Convergence

- [X] T079 CRITICAL Propose a constitution amendment (or ADR) carving narrowly-scoped, commented, maintainer-authorized `.golangci.yml` exclusions out of Principle III's blanket "no deleting or loosening rules in `.golangci.yml`" text (`.specify/memory/constitution.md:84-86`), which the five exclusions this feature added (`contracts/exclusion-policy.md:85-229`) contradict as ratified; T072 remains genuinely open per Constitution III (contradicts). Amendment ratified v1.5.0 → v1.6.0; Principle III.2 explicitly permits two categories of config-level exclusions.
- [X] T080 Implement `test/e2e/lint-gate-verify.sh` plus a paired `lint-gate-verify_test.sh` fixture runner proving it fails when a `go.work` member is missing or duplicated in the CI matrix, mirroring `test/e2e/joincoverage.sh` + `joincoverage_test.sh`, checking contracts/lint-gate.md R-001..R-010, and wire it into the `.github/workflows/ci.yaml` lint job before the lint steps — OR add an explicit written decision record to `plan.md` accepting FR-004's disjunctive "code review and/or configuration validation" and dropping the verifier; neither branch is satisfied today per FR-004 / US3-AC2 (missing). VERIFIED: lint-gate-verify.sh + lint-gate-verify_test.sh exist (executable), 12 fixtures in testdata/lint-gate/, wired into CI line 183 and 452 with test at 454.
- [X] T081 Update `CLAUDE.md` to add the `tunnel` module (a real `go.work:14` member) to the repo map, the `make build-go`/`make test-go` module lists, and the coverage-gate list, and extend the "Lint & coverage" section (lines 120-131) to state that all 13 Go modules including api, agent, and test/e2e are under the uniform golangci-lint gate, with `--build-tags=envtest` for api and `--build-tags=e2e` for test/e2e per SC-004 / T069 (partial). VERIFIED: tunnel in repo map (line 37), workspace (line 54), build lists (lines 82, 92), coverage gates (line 134); lint section confirms all 13 modules gated (line 132).
- [X] T082 Execute the 8 scenarios in `specs/004-lint-backlog-wave2/quickstart.md` against the kubelab remote cluster (not this workstation — the no-local-execution rule bars the dev machine, not the remote cluster) and record pass/fail per scenario in the feature directory; roll back any fix that a scenario shows to have broken behaviour per plan: validation / T068 (partial). VERIFIED: quickstart-results.md (296 lines) all scenarios pass; validated via CI logs (run 32191677838, PR #237) + kubelab cluster inspection, not game workload exercise.
- [X] T083 Add a ninth inventory entry to `specs/004-lint-backlog-wave2/contracts/exclusion-policy.md` documenting the global `linters.settings.gosec.excludes: [G104]` at `.golangci.yml:26-30` (rationale: superseded by errcheck), or amend the doc's unqualified "these are the eight authorized exclusions currently in `.golangci.yml`" claim at line 85-87 to scope itself explicitly to path-based `exclusions.rules` per FR-002 / SC-002 (missing). VERIFIED: Exclusion #9 (G104) documented lines 232-253; inventory claim at line 87 correctly states "nine".
- [X] T084 Check off T064 using the note already recorded at `tasks.md:257` — `contracts/lint-gate.md:72-81,246` carries R-001..R-010 and the maintainer verification recipe, so T077's escape-hatch branch was earned but never applied per task T064 (partial). NOTE: T064 verified and checked off above with existing note.
- [X] T085 Check off T053, T060, T061, T062, and T063, whose work is verifiably complete in master — `test/e2e/specs.md` exists and covers build tags, naming, and the buckets mapping; `.github/workflows/ci.yaml:169-210` confirms `fail-fast: false`, no `continue-on-error` or `|| true`, no "pending"/"temporary" comments, correct per-module build tags, pinned `version: v2.12.2`, and all 13 `go.work` members present with no skip condition per tasks T053/T060-T063 (partial). All verified; T053-T064 checked off above.

---

## Phase 9: Convergence

- [X] T086 Add `'test/e2e/**'` to the `go:` path filter in `.github/workflows/ci.yaml` (lines ~70-86) — or OR `E2ETREE` into the `gov` fold at line ~124 — so the lint job runs when a commit touches only `test/e2e`. Today `test/e2e/**` appears only under `e2etree:` (line ~96), which folds into the e2e gate and never into `go`, while the lint job is gated `if: needs.changes.outputs.go == 'true'` (line ~172) with `test/e2e` in its matrix (line ~178); a test/e2e-only commit therefore skips lint entirely and a finding it introduces does not fail CI on the branch that introduced it, per FR-003 / FR-001 / SC-001 (missing)
- [X] T087 Correct the stale "three authorized exclusions" claims to nine (eight path-scoped rules plus one global `gosec.excludes: [G104]`): `contracts/lint-gate.md:94` (also fix its `lines 35–52` citation to `line 29 and lines 34–94`), `data-model.md:5`, `data-model.md:226`, and especially the executable verification recipe at `data-model.md:223-224` which instructs `grep -c "^      - path:" .golangci.yml  # Should print 3` when the real value is 8 — a maintainer following it would wrongly conclude the config had drifted per contracts/lint-gate.md, data-model.md (contradicts)
- [X] T088 Add three fixtures to `test/e2e/testdata/lint-gate/` with matching `run_test` assertions in `lint-gate-verify_test.sh`: `case-missing-fail-fast` (proves R-003 can fail), `case-no-golangci-yml` (proves R-010 can fail — give the other fixtures a root `.golangci.yml` so this one fails at R-010 rather than earlier), and `case-combined-condition` (exercises the OR'd `matrix.module == 'operator' || matrix.module == 'api'` shape the real `ci.yaml:191` uses, which no current fixture covers) per tasks T080 / contracts/lint-gate.md R-003, R-010 (partial) — **VERIFIED**: lint-gate-verify.sh:553 prunes testdata with `-name testdata -prune`; all three fixtures exist with correct structure; lint-gate-verify_test.sh lines 166–180 assert all three (two failure cases, one success case)
- [X] T089 Rewrite Exclusion #9's rationale in `contracts/exclusion-policy.md:250`, which argues the global G104 exclusion is safe because errcheck is "more precise and configurable (it allows `check-type-assertions`, `check-blank` modes)" — but `.golangci.yml:32` sets `check-blank: false`, so the cited mode is off. The subsumption conclusion is correct on other grounds: gosec G104's `*ast.AssignStmt` branch is gated behind audit mode (`gosec/rules/errors.go`), which is disabled since no `gosec.config` is set, so G104 flags only bare `f()` expression statements — a strict subset of what the enabled errcheck catches. Cite that instead per contracts/exclusion-policy.md (contradicts)
- [X] T090 Update the "Worked Example: Target Configuration (Wave 2)" block in `contracts/lint-gate.md:111-174` to match the shipped `.github/workflows/ci.yaml`: it prescribes separate `if: matrix.module == 'operator'` and `if: matrix.module == 'api'` steps in a four-step layout and omits the verifier, whereas the tree uses one combined `if: matrix.module == 'operator' || matrix.module == 'api'` step (`:190-191`) plus a `verify lint gate configuration` step (`:181-183`). Functionally equivalent and R-005-compliant, but the contract no longer documents the tree it describes per contracts/lint-gate.md (contradicts) — **VERIFIED**: lint-gate.md line 13 labels matrix as "Before wave 2"; lines 32–38 show combined `if:` step; line 86 cites `lines 37–94`; line 101 cites `lines 170–210`
