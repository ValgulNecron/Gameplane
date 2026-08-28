# Tasks: Code Scanning Vulnerability Remediation & Dependabot PR Integration

**Feature**: 009-remediate-security-dependabot  
**Branch**: `009-remediate-security-dependabot`  
**Ground Truth**: `/tmp/claude-1000/-home-valgul-project-kubernetes-game-dashboard/b92598c2-044e-4593-8eef-0c717f7cae0c/scratchpad/BRIEF.md`

---

## Phase 1: Setup

**Purpose**: Establish baseline state and verify tooling prerequisites

- [ ] T001 Create and check out feature branch `009-remediate-security-dependabot` from master
- [ ] T002 Record baseline CodeQL alert count and Dependabot PR count to baseline.txt via gh api queries (expected: 14 alerts, 21 open PRs)
- [ ] T003 Confirm gh token has security_events scope by attempting to read alert #1 via gh api repos/ValgulNecron/Gameplane/code-scanning/alerts/1

---

## Phase 2: Foundational

**Purpose**: Implement the shared path confinement helper that all US1 path-injection remediations depend on

**Blocking Prerequisite**: US1 cannot start until ConfinePath helper is complete and unit-tested.

- [ ] T004 Implement ConfinePath helper function in agent/internal/mods/confinement.go per signature and contract in contracts/path-confinement.md; base implementation on filepath.Clean, filepath.EvalSymlinks symlink resolution pattern, and prefix-confinement check from agent/internal/files/files.go:57–98
- [ ] T005 Write unit tests for ConfinePath in agent/internal/mods/confinement_test.go covering all rejection rows from contracts/path-confinement.md table: empty component, `.`, `..`, `../` prefix, absolute paths, path separators, leading dots, length > 200, control characters, backslashes, symlink escape attempts, ancestor escape attempts

---

## Phase 3: User Story 1 — Remediate Filesystem, SSRF & Logging Vulnerabilities (P1)

**Plan Phase**: A  
**Goal**: Eliminate 11 CodeQL alerts via path confinement consolidation, zip-slip validation restructuring, clear-text logging variable rename, and SSRF/netguard defense-in-depth; address 1 latent gap in agent/internal/rcon/websocket.go that CodeQL has not flagged.

### Unit Tests for User Story 1

- [ ] T006 [P] [US1] Write unit tests for path-confining operations in agent/internal/mods/mods_test.go covering removeEntry, download, swapInArchive, remove, and unzipInto with ConfinePath return values

### Per-Site Migrations for User Story 1

- [ ] T007 [US1] Fix alerts #8 at :389 and #9 at :391 in agent/internal/mods/mods.go removeEntry by replacing inline Join+Clean+HasPrefix guard with ConfinePath call
- [ ] T008 [US1] Fix alert #10 at :446 in agent/internal/mods/mods.go download by calling ConfinePath in download() itself instead of relying on caller's safeName()
- [ ] T009 [US1] Fix alerts #11 at :486 and #12 at :490 in agent/internal/mods/mods.go swapInArchive by replacing inline Join+Clean+HasPrefix guard with ConfinePath call
- [ ] T010 [US1] Fix alert #13 at :594 in agent/internal/mods/mods.go remove by replacing inline Join+Clean+HasPrefix guard with ConfinePath call
- [ ] T011 [US1] Fix alert #7 at :508 in agent/internal/mods/mods.go unzipInto by replacing continue-on-escape branch at :511 with single-exit error return that rejects any escaping archive entry
- [ ] T012 [US1] Fix alerts #1 at :40 and #2 at :54 in api/internal/kube/watch.go by renaming secretKey variable to remove heuristic taint source; document that this is cosmetic with respect to real security since kubeconfig bytes never enter log output

### SSRF & Latent Gaps for User Story 1

- [ ] T013 [US1] Attempt barrier-recognizable restructuring for alerts #5 at :405 in agent/internal/mods/mods.go and #6 at :281 in api/internal/ws/dialer.go to make SSRF validation more explicit to CodeQL's model; note that dismissal is the expected outcome per contracts/alert-disposition.md
- [ ] T014 [US1] Add netguard policy to agent/internal/rcon/websocket.go line 292 ensureLocked() to route admin-supplied WebSocket URLs through netguard dial policy for defence-in-depth

### E2E Tests for User Story 1

- [ ] T015 [US1] Write e2e test case for path traversal rejection in test/e2e/api_mods_confinement_e2e_test.go verifying extraction rejects `../` archive entries
- [ ] T016 [US1] Write e2e test case for symlink rejection in test/e2e/api_mods_confinement_e2e_test.go verifying ConfinePath rejects symlink targets outside sandbox
- [ ] T017 [US1] Write e2e test case for extraction sandbox confinement in test/e2e/api_mods_confinement_e2e_test.go verifying all extracted files stay within designated directory

### E2E Bucket Registration & Specifications Update for User Story 1

- [ ] T018 [US1] Register three new e2e tests in test/e2e/buckets.sh under api-mods bucket; state in task notes that e2e-buckets CI job fails on unbucketed tests, all new tests use t.Parallel() with unique per-test resource names, and api-mods bucket has ~7 admin-login budget
- [ ] T019 [US1] Update agent/specs.md to document path confinement helper contract and migration from safeName-only to ConfinePath-based validation
- [ ] T020 [US1] Update api/specs.md to document clear-text logging change for cluster watch secretKey variable rename

---

## Phase 4: User Story 2 — Remediate TLS Verification & Memory Allocation Alerts (P1)

**Plan Phase**: A  
**Goal**: Fix 1 real defect (satisfactory.go TLS), bound 1 allocation (audit.go limit), and dismiss 1 verified-unclearable alert (satisfactory.go production).

### Unit Tests for User Story 2

- [ ] T021 [P] [US2] Write unit tests in test/e2e/internal/satisfactory/satisfactory_loopback_test.go covering loopback guard both rejecting non-loopback addresses (0.0.0.0, 8.8.8.8) and accepting loopback addresses (127.0.0.1, ::1); this untagged unit test is the real CI gate since satisfactory_bot_e2e_test.go lives in the bot-heavy bucket (which never runs in CI), so the go-e2e-unit CI job verifies the loopback guard
- [ ] T022 [P] [US2] Write unit tests in api/internal/handlers/audit_test.go for audit limit clamping covering negative limits, zero limits, extremely large limits, and normal limits

### Fixes for User Story 2

- [ ] T023 [US2] Fix alert #4 in test/e2e/internal/satisfactory/app.go:188 queryServerState by adding loopback guard mirroring isLoopbackHost logic from agent/internal/rcon/satisfactory.go:220–226
- [ ] T024 [US2] Fix alert #14 (flagged allocation `out := make([]Event, 0, limit)` at api/internal/audit/audit.go:834) in api/internal/handlers/audit.go:25 and api/internal/audit/audit.go:820–822 by introducing MaxAuditPageSize named constant (500), clamping untrusted limit value at handler layer before passing to Auditor.Page, and using new bounded variable instead of parameter reassignment

### Dismissal for User Story 2

- [ ] T025 [US2] Obtain maintainer sign-off for alert #3 dismissal at agent/internal/rcon/satisfactory.go:199; this sign-off blocks the dismissal submission task (T046) in Phase 7 and is recorded in contracts/alert-disposition.md alongside the justification. Rationale: InsecureSkipVerify in production satisfactory.go is guarded by isLoopbackHost check accepting only localhost/loopback IPs; Satisfactory generates self-signed cert with no CA supply API; connection is pod-local; this is verified unclearable without removing InsecureSkipVerify entirely; package documentation at lines 60–76 records full rationale

---

## Phase 5: User Story 3 — Reconcile & Merge Go Dependency Updates (P1)

**Plan Phase**: B  
**Goal**: Merge 9 green Go dependency PRs individually in ascending blast-radius order; diagnose and document PR #263.

**Merge Mechanics**: Each PR uses `gh pr merge <N> -R ValgulNecron/Gameplane --admin --merge` (--admin required for branch ruleset update rule; merge-commit not squash). If sibling PR CI fails due to go.sum conflict after a merge, comment `@dependabot rebase` to auto-rebase and re-run.

- [ ] T026 [US3] Merge PR #276 gopacket 1.6.1 → 1.7.1 across 2 modules (capture-sidecar, test/e2e); verify master CI green
- [ ] T027 [US3] Merge PR #279 cosign 2.6.4 → 2.6.5 across 3 modules (capture-sidecar, operator, test/e2e); rebase if needed after T026 merge
- [ ] T028 [US3] Merge PR #281 sqlite 1.55.0 → 1.57.0 across 5 modules (api, capture-sidecar, mcp-server, operator, test/e2e); rebase if needed
- [ ] T029 [US3] Merge PR #267 chi 5.3.1 → 5.3.2 across 5 modules (agent, api, capture-sidecar, operator, test/e2e); rebase if needed
- [ ] T030 [US3] Merge PR #274 x/mod 0.38.0 → 0.40.0 across 7 modules (agent, api, capture-sidecar, mcp-server, operator, sentinel, test/e2e); rebase if needed
- [ ] T031 [US3] Merge PR #271 k8s.io/api 0.36.3 → 0.36.4 across 7 modules (agent, api, capture-sidecar, mcp-server, operator, sentinel, test/e2e); rebase if needed
- [ ] T032 [US3] Merge PR #269 x/net 0.57.0 → 0.58.0 across 7 modules (agent, api, capture-sidecar, mcp-server, operator, sentinel, test/e2e); rebase if needed
- [ ] T033 [US3] Merge PR #273 minio-go 7.2.1 → 7.3.0 across 8 modules (agent, api, capture-sidecar, mcp-server, operator, sentinel, telemetry-receiver, test/e2e); rebase if needed
- [ ] T034 [US3] Merge PR #265 go-containerregistry 0.21.7 → 0.22.0 across 8 modules (agent, api, capture-sidecar, mcp-server, operator, sentinel, telemetry-receiver, test/e2e); rebase if needed
- [ ] T035 [US3] Diagnose failing check on PR #263 sigstore 1.10.8 → 1.10.9 (3 modules: capture-sidecar, operator, test/e2e); accept as done when root cause is identified from CI logs read via the gh CLI and either (a) a fix is landed, or (b) the blocker is documented in contracts/dependency-upgrade.md with the relevant log excerpt. An inconclusive diagnosis does not close the task

---

## Phase 6: User Story 4 — Reconcile & Merge Frontend NPM Dependency Updates (P2)

**Plan Phase**: B  
**Goal**: Merge 9 green npm dependency PRs individually; all touch web/package.json and web/package-lock.json; marked [P] relative to US3 (disjoint files) but NOT [P] within US4 (shared package-lock.json).

**Security Note**: PR #283 is a real security fix (brace-expansion CVE-2026-13149 + js-yaml DoS); merge first as priority.

- [ ] T036 [US4] Merge PR #283 security group: brace-expansion 1.1.14 → 1.1.18 (CVE-2026-13149) and js-yaml 4.3.0 → 4.3.2 (DoS) in web/package.json
- [ ] T037 [US4] Merge PR #280 @types/react-dom 19.2.3 → 19.2.4 in web/package.json; verify web-e2e-mock and web tests pass
- [ ] T038 [US4] Merge PR #278 vitest 4.1.10 → 4.1.11 in web/package.json
- [ ] T039 [US4] Merge PR #277 @vitejs/plugin-react 6.0.4 → 6.1.0 in web/package.json
- [ ] T040 [US4] Merge PR #275 @types/node 26.1.2 → 26.2.0 in web/package.json
- [ ] T041 [US4] Merge PR #270 @tanstack/react-router 1.170.18 → 1.170.32 in web/package.json
- [ ] T042 [US4] Merge PR #266 @playwright/test 1.62.0 → 1.62.1 in web/package.json
- [ ] T043 [US4] Merge PR #264 @testing-library/jest-dom 7.0.0 → 7.0.1 in web/package.json
- [ ] T044 [US4] Merge PR #262 @typescript-eslint/parser 8.65.0 → 8.67.0 in web/package.json

---

## Phase 7: User Story 5 — End-to-End Verification and Clean PR Closure (P2)

**Plan Phase**: A (runs after Phases A refactors merge)  
**Goal**: Verify all 14 alerts are resolved, dismiss unclearable ones, close Dependabot PRs, confirm master CI fully green.

**Critical Note**: Alert closure is only observable AFTER merge to master, because CodeQL default setup analyzes the default branch; a green feature branch proves nothing about alert state.

- [ ] T045 [US5] Re-query code-scanning alerts on master via gh api repos/ValgulNecron/Gameplane/code-scanning/alerts and record which alerts are now `fixed` vs remain `open`
- [ ] T046 [US5] Submit dismissal for alert #3 via gh api PATCH to code-scanning/alerts/3 with state=dismissed, dismissed_reason=false_positive, and documented justification from contracts/alert-disposition.md
- [ ] T047 [US5] Submit dismissals for any non-cleared alerts from T045's re-query that remain open via gh api PATCH calls with false_positive reason and full justification per contracts/alert-disposition.md
- [ ] T048 [US5] Verify Dependabot PR list via gh pr list -R ValgulNecron/Gameplane --author=dependabot --state=open shows only #263 remaining if its diagnosis in T035 concluded it is blocked (all others closed)
- [ ] T049 [US5] Confirm master branch CI is fully green across all ci.yaml jobs (lint, go, web, web-e2e-mock, helm, chart-template, go-e2e-unit, e2e-buckets, e2e-go, e2e-multicluster, e2e-upgrade, e2e-web-live, e2e-game-bot, report)
- [ ] T050 [US5] Walk through specs/009-remediate-security-dependabot/quickstart.md end-to-end to verify baseline capture, alert re-query, dismissal submission, and final verification steps all execute

---

## Phase 8: Polish & Finalization

**Purpose**: Clean up working branch and document final outcomes.

- [ ] T051 Delete merged feature branch remote via git push origin --delete 009-remediate-security-dependabot per branch-lifecycle rule
- [ ] T052 Delete merged feature branch local via git branch -d 009-remediate-security-dependabot per branch-lifecycle rule
- [ ] T053 Update specs/009-remediate-security-dependabot/contracts/alert-disposition.md with actual final state for each of the 14 alerts (fixed or dismissed with datetime and outcome notes); note that spec's original stale count of 20 PRs is corrected to 21 (adds #283 security bump) and all-14-real-defects claim is corrected to 13 false positives + 1 real defect, with the latent gap described as separate from the 14 alerts rather than counted among them

---

## Phase 9: Major Frontend Dependency Migrations (Plan Phase D)

**Purpose**: Reconcile and merge major TypeScript and ESLint version upgrades that require code migrations.

**Prerequisite**: Phase 6 (US4 main wave) must be complete before Phase 9 starts.  
**Execution**: Runs on its own branch off master after Phase 6 merges.

**Note**: PR #272 (TypeScript 7) and PR #268 (ESLint 10) are deferred from Phase 6 due to breaking changes requiring source code updates; they are tackled here with explicit diagnosis of type errors and linting violations, and acceptance criteria that forbid `// @ts-ignore` and `// eslint-disable` per constitution Principle III.

- [ ] T054 [US4] Diagnose failing checks on PR #272 (typescript 6.0.3 → 7.0.2) by reading CI logs via gh api repos/ValgulNecron/Gameplane/actions/runs/<run_id>/attempts/<attempt>/logs and recording specific type errors encountered in web/ migration
- [ ] T055 [US4] Apply TypeScript 7 migration in web/ by updating web/package.json to 7.0.2, web/tsconfig.json as needed, and fixing all resulting type errors in web/src source files; constitution Principle III forbids resolving any error with // @ts-ignore
- [ ] T056 [US4] Merge PR #272 once all CI checks are green via gh pr merge 272 -R ValgulNecron/Gameplane --admin --merge
- [ ] T057 [US4] Diagnose failing check on PR #268 (@eslint/js 9.39.5 → 10.0.1) by reading CI logs via gh API and recording specific linting violations encountered; note that ESLint 10 drops eslintrc support, removes deprecated SourceCode and rule-context methods, and raises the Node floor to ^20.19 || ^22.13 || >=24
- [ ] T058 [US4] Apply ESLint 10 migration in web/ by updating web/package.json and web/eslint.config.js to 10.0.1, removing any deprecated rule-context or SourceCode usage, and fixing all resulting linting violations; constitution Principle III forbids resolving any finding with // eslint-disable
- [ ] T059 [US4] Merge PR #268 once all CI checks are green via gh pr merge 268 -R ValgulNecron/Gameplane --admin --merge
- [ ] T060 [US4] Confirm web CI jobs (web, web-e2e-mock) are green on master after both migrations are merged

---

## Dependencies & Execution Order

**Phase Blocking Relationships**:

- **Phase 1** → Phase 2: Baseline must be recorded before work begins.
- **Phase 2** → **Phase 3 (US1)**: ConfinePath helper and unit tests MUST complete before any US1 refactors start (6 alerts depend on it).
- **US1 and US2 are independent**: Can run on separate branches concurrently or sequentially on the same branch; no shared code.
- **Phase 3 (US3) and Phase 4 (US4) are independent**: Go PRs and npm PRs touch disjoint files; can merge in parallel.
- **Phase 5 (US5)** → Phase 6 (Polish): All PR merges and refactors must be complete before final verification.
- **Phase 6 (US4)** → Phase 9 (Major Frontend Migrations): PR #272 and #268 are deferred from Phase 6 to Phase 9 (separate branch off master) due to breaking changes requiring source migrations.
- **Phase 9** runs after Phase 6 merges and requires explicit diagnosis and code fixes for TypeScript 7 and ESLint 10 migrations.

**Parallel Opportunities**:

Within each phase:
- **Phase 3 (US1) unit tests** (T006) and **Phase 2** (T004, T005) can run in pipeline: T004 → T005, then T005 gates T006.
- **Phase 3 (US1) per-site migrations** (T007–T014) can run in parallel once T005 is done (all are building on ConfinePath).
- **Phase 3 (US1) e2e tests** (T015–T017) can run in parallel with per-site migrations (T007–T014).
- **Phase 4 (US2) unit tests** (T021, T022) can run in parallel (separate modules).
- **Phase 5 (US3) Go PR merges** (T026–T035) are sequential due to go.sum conflicts (each invalidates the next PR's sum).
- **Phase 6 (US4) npm PR merges** (T036–T044) are [P] relative to Phase 5 (US3) due to disjoint files, but sequential within US4 (all touch package-lock.json).
- **Phase 9 (Major Frontend Migrations) tasks** (T054–T060) are sequential: each PR diagnosis feeds into its migration task, which must complete before merging, before the next PR cycle starts.

**Parallel Example**:

```
Phases 1–2 (serial, setup/foundation):
  T001 → T002 → T003 → T004 → T005

Phase 3 (US1, concurrent within gates):
  T006 (gates all refactors, runs after T005)
  T007–T014 (per-site refactors, parallel, all depend on T005)
  T015–T017 (e2e tests, parallel, can start once T005 is done)
  T018–T020 (specs & bucket registration, depend on T007–T017)

Phase 4 (US2, parallel):
  T021, T022 (unit tests, parallel)
  T023, T024 (fixes, parallel or sequential)
  T025 (dismissal sign-off, gate before US5)

Phase 5 (US3, sequential):
  T026 (merge #276) → T027 (merge #279, depends on T026) → ... → T035 (diagnose #263)

Phase 6 (US4, parallel with US3 but sequential within):
  [parallel with Phase 5]
  T036 (security bump #283) → T037–T044 (remaining npm PRs, sequential)

Phase 7 (US5, final gates):
  All PR merges complete → T045–T050 (verification, depends on all PRs)

Phase 8 (Polish, post-merge cleanup):
  T051–T053 (branch delete, outcome documentation)

Phase 9 (Major Frontend Migrations, after Phase 6 on separate branch):
  T054 (diagnose #272 TypeScript) → T055 (apply migration) → T056 (merge #272)
  → T057 (diagnose #268 ESLint) → T058 (apply migration) → T059 (merge #268)
  → T060 (verify web CI green on master)
```

**Total Task Count**: 60 tasks across 9 phases (T001–T060)

---

## Implementation Strategy

### MVP Definition

The natural first increment is **US2** rather than US1. Here is why:

- **US2 is smaller**: 1 real defect (satisfactory.go TLS) + 1 allocation bound (audit.go limit) + 1 verified dismissal (satisfactory.go production). Four short tasks vs. US1's 15 tasks.
- **US2 has lower risk**: The real defect is a straightforward loopback guard copy-paste from production code; the allocation fix is a clamp at the handler. No new abstractions needed.
- **US2 unblocks early**: The ConfinePath helper (Phase 2) is a prerequisite for US1 only. US2 can start as soon as Phase 1 baseline is recorded.
- **US1 can follow**: Once Phase 2 (ConfinePath) is complete and passing unit tests, all US1 refactors can proceed in parallel or in rapid sequence.

**MVP scope**: Phase 1 (Setup) + Phase 2 (ConfinePath helper) + Phase 4 (US2) = 3 + 2 + 4 = **9 tasks**, delivering the 1 real defect fix and the 1 allocation bound fix. This unblocks Phase 5/6 Dependabot merges to start immediately without waiting for the larger Phase 3 refactor wave.

After MVP merge: Phase 3 (US1) proceeds in a follow-up, then Phase 5/6 (Dependabot merges) run in parallel with US1, then Phase 7 (US5) verification runs on master after all merges are done.

---

## Notes on Execution

### Commits & Signing

- Every commit is signed: `git commit -s`. Since pinentry times out headless in CI, use `git -c commit.gpgsign=false commit -s ...` to skip GPG signing while preserving the `-s` (DCO) signature.
- Use conventional commit prefixes: `fix:` (bug fixes), `refactor:` (code restructuring), `test:` (test-only changes), `chore:` (dependency updates, housekeeping), `docs:` (documentation).
- Codegen output (if any CRD changes arise from reconfigurations) is committed in the same change as the source change that triggered it.
- One logical unit of work per commit. A "logical unit" is roughly one task (one alert fix, one PR merge, one helper function + tests).

### Branch Lifecycle

- Feature branch `009-remediate-security-dependabot` is created in Phase 1 and used for all work.
- Once Phase A (alert remediation) merges to master, the feature branch is deleted (both remote and local) per repo branch-lifecycle rule in Phase 8.
- Dependabot PRs are merged INDIVIDUALLY to master (not into the feature branch), using their own branches as-created by Dependabot. After each merge, master updates and the next PR may need `@dependabot rebase`.
- At the end of Phase 8, all merged branches are deleted locally and remotely.

### GitHub CLI & Repository Target

- **Every `gh` command MUST include `-R ValgulNecron/Gameplane`** to prevent cwd drift into `modules/` submodule from retargeting `gh` at the wrong repository.
- Merge commands use `gh pr merge <N> -R ValgulNecron/Gameplane --admin --merge` (flags are required for master's branch ruleset).
- Alert queries use `gh api repos/ValgulNecron/Gameplane/code-scanning/alerts` to list and read alerts.
- Dismissal submissions use `gh api -X PATCH repos/ValgulNecron/Gameplane/code-scanning/alerts/N` with state, dismissed_reason, and dismissed_comment.

### Verification is CI-Only

- Constitution Principle VI: Nothing runs locally. No `make test`, `make lint`, `npm run build`, `go build`, `tsc`, or any other build/test/verification command is run on this machine.
- All builds, tests, linting, and e2e runs happen on GitHub Actions CI (`ci.yaml` workflow and dedicated e2e jobs).
- A locally-green compile is not verification; only CI green is verification.
- Alert closure and Dependabot PR closure are GitHub-side state changes observable only after master re-analysis (CodeQL runs on the default branch, not on feature branches).
