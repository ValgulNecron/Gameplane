# Tasks: Complete Module Specifications & Compliance Verification

**Input**: Feature spec from `/specs/011-add-missing-module-specs/`

**Prerequisites**: All artifacts in `/specs/011-add-missing-module-specs/` (spec.md, plan.md, research.md, data-model.md, contracts/, OPEN-DECISIONS.md, quickstart.md)

**Tests**: No unit/E2E tests requested. Verification: negative test in CI (simulated missing specs.md exits 1); script correctness validated in CI lint job step per D6 and quickstart.md Scenario 5.

**Organization**: Tasks grouped by user story (US1, US2, US3), with parallel opportunities marked [P]. Polish phase covers commits and CI verification per CLAUDE.md rules 8, 11.

## Format

- **[P]**: Can run in parallel (different files, no dependencies)
- **[USX]**: Story phase (US1, US2, US3); setup/foundational/polish do not use story tags
- File paths, section lists (from `contracts/specs-md-structure.md`), FRs (FR-001–FR-007), and rulings (D1–D6) named in descriptions

---

## Phase 1: Setup

**Purpose**: Confirm branch context and understand all contracts

- [ ] T001 Confirm checked out on branch `011-add-missing-module-specs`; read all spec artifacts (spec.md, plan.md, research.md, data-model.md, contracts/check-specs.md, contracts/specs-md-structure.md, quickstart.md, OPEN-DECISIONS.md); understand FR-001 through FR-007 and maintainer rulings D1–D6

---

## Phase 2: Foundational

**Purpose**: Establish shared understanding before user stories begin

**Status**: No foundational blocking work. All three user stories can proceed in parallel once Phase 1 completes. They share read-only dependencies on the same contracts (specs-md-structure.md for canonical format; check-specs.md for script behavior; data-model.md for module registry definition). No scaffolding, infrastructure, or prerequisite implementation tasks required.

---

## Phase 3: User Story 1 - Author Missing Module Specifications (Priority: P1) 🎯 MVP

**Goal**: Write authoritative `specs.md` files for `svcutil` and `tunnel` modules, satisfying Constitution Principle IV requirement (FR-001 through FR-005)

**Independent Test**: Both new specs.md follow canonical structure (Purpose, Responsibilities, Non-goals, Directory layout, External interface, Key invariants, Dependencies, Security, Testing & coverage, References per `contracts/specs-md-structure.md`); svcutil documents 4 exported functions + 90% coverage gate per research.md decision; tunnel documents 3 providers (frp, tailscale, playit) + 70% coverage gate per `tunnel/.testcoverage.yml`; both validate against existing module specs for consistency (e.g., sentinel/specs.md, capture-sidecar/specs.md, api/specs.md patterns)

### Implementation for User Story 1

- [ ] T002 [P] [US1] Write `svcutil/specs.md` per canonical structure in `contracts/specs-md-structure.md` (§ "svcutil/specs.md"): document stdlib-only shared utility module, 4 exported functions (`Or`, `OrInt`, `ParseLogLevel`, `RunHTTP`), no-panic fallback semantics, 90% coverage gate per research.md, graceful shutdown race condition gaps, key invariants about environment variable handling; satisfies FR-001, FR-002, FR-005, SC-004

- [ ] T003 [P] [US1] Write `tunnel/specs.md` per canonical structure in `contracts/specs-md-structure.md` (§ "tunnel/specs.md"): document relay client supervisor, 3 providers (frp, tailscale, playit), environment variable schema (TUNNEL_TYPE, BACKING_SERVICE_DNS, provider-specific), credential mounting at `/etc/gameplane/tunnel-auth`, rendered config file paths, exponential backoff retry strategy (base 2^n, capped 5min), SIGTERM/SIGKILL signal forwarding (10s grace), exit code classification (126/127 unrecoverable), 70% coverage gate per `tunnel/.testcoverage.yml`; satisfies FR-003, FR-004, FR-005, SC-004

**Checkpoint**: Both `svcutil/specs.md` and `tunnel/specs.md` complete, follow canonical structure, validate against FR-001–FR-004 and contracts; ready for commit per rule 11

---

## Phase 4: User Story 2 - Comprehensive Repository Audit (Priority: P1)

**Goal**: Audit all modules in `go.work` and `web/` to catalog current specs.md coverage and verify research.md findings (satisfies spec user story 2 acceptance scenarios, implicit FR-005)

**Independent Test**: Audit confirms 12 of 14 go.work modules + `web/` have specs.md (agent, api, audit-syslog-bridge, capture-sidecar, gameaction, gameproto, mcp-server, netguard, operator, sentinel, telemetry-receiver, test/e2e, web); svcutil and tunnel confirmed missing before US1 completes; after US1, 100% compliance verified per SC-001

### Implementation for User Story 2

- [ ] T004 [US2] Verify and record specs.md audit results: re-audit 14 `go.work` modules + `web/` directory per `data-model.md` "Workspace Module Registry"; confirm current state matches research.md audit findings (12 modules have valid specs.md; svcutil, tunnel missing); record findings location per data-model.md; note that `modules/<game>/*` directories are out of scope per D2; catalog documentation status and boundaries of infrastructure directories `charts/gameplane/` and `deploy/kind/` per data-model.md § Exclusions (deployment concerns, not workspace modules); verify after US1 completion that all 15 modules (14 go.work + web) have non-empty specs.md files (no whitespace-only), satisfying SC-001

**Checkpoint**: Audit findings verified and recorded; cleared for US3 automated check implementation

---

## Phase 5: User Story 3 - Automated Specification Completeness Verification (Priority: P2)

**Goal**: Implement automated compliance check script, Makefile target, CI integration, and documentation updates to enforce ongoing specs.md compliance (FR-006, FR-007; satisfies spec user story 3 acceptance scenarios)

**Independent Test**: `make check-specs` invocation outputs success summary ("✓ Checked 15 modules: all have non-empty specs.md") and exits 0 per `contracts/check-specs.md`; negative test (simulated missing/empty specs.md) causes exit 1 with diagnostic error; CI lint job runs check as dedicated step (gated `if: matrix.module == 'netguard'` per D5, ensuring single execution not per-module redundancy); check completes in <2 seconds per SC-002

### Implementation for User Story 3

- [ ] T005 [P] [US3] Implement `hack/check-specs.sh` script per `contracts/check-specs.md`: parse `go.work` to extract 14 module directories (use `awk` to extract `use (...)` block), iterate each module to validate `specs.md` file existence and non-empty content (detect whitespace-only per D3 using `grep -q '[^[:space:]]'`), validate `web/specs.md` separately, output diagnostic errors matching contract § "Failure Case" format or success summary § "Success Case", exit 0 if all modules compliant, exit 1 if any missing/empty; satisfy FR-006, SC-002, SC-003; make script executable (`chmod +x`)

- [ ] T006 [P] [US3] Add Makefile targets in `Makefile` (around line 229 where lint is defined): add `.PHONY: check-specs` target that invokes `hack/check-specs.sh` with no arguments; update existing `lint` target to depend on `check-specs` as first prerequisite (`lint: check-specs lint-go lint-web`); add help comment (`check-specs: ## Verify all modules have valid, non-empty specs.md`); satisfy FR-006

- [ ] T007 [US3] Add CI step to `.github/workflows/ci.yaml` lint job (lines 327–388, insert after line 343 "verify lint gate configuration" step): new step named "check specs compliance" with `if: matrix.module == 'netguard'` gate (single execution per lint job trigger, not per-module per D5), `run: make check-specs` with stdout captured in CI logs; satisfy FR-007, SC-002, D5 ruling

- [ ] T008 [P] [US3] Update `docs/module-authoring.md` to add game module specification guideline: add subsection (e.g., "Module Documentation" or "Specifications") stating that each `modules/<game>/` directory should include `specs.md` documenting purpose, protocol/RCON details, configuration, game-specific notes; reference canonical structure in `contracts/specs-md-structure.md`; note that enforcement is in `gameplane-module` repo's own CI per D2; satisfy D2 ruling (guideline only, not CI-enforced in Gameplane repo)

- [ ] T009 [P] [US3] Update `CLAUDE.md` § Lint section (around line 456 in ### Lint & coverage section) to add single-line mention of `make check-specs` integration: add text like "The `make check-specs` target validates that all workspace modules have non-empty specs.md per Constitution Principle IV. See `hack/check-specs.sh` and `specs/011-add-missing-module-specs/contracts/check-specs.md` for details." per plan.md Lint section requirement

**Checkpoint**: Automated compliance check fully implemented and integrated into CI; US3 ready for commit per rule 11

---

## Phase 6: Polish & Verification

**Purpose**: Commit all changes per logical unit (rule 11), push branch, verify CI green, confirm feature ready for review

- [ ] T010 Commit svcutil and tunnel specifications as single logical unit: `git commit -s` with subject "docs: add svcutil and tunnel module specifications", body: "Add authoritative specs.md for svcutil (stdlib-only utility module, 90% coverage gate) and tunnel (relay supervisor, three providers, 70% coverage gate) per Constitution Principle IV (FR-001, FR-003). Both follow canonical specs.md structure from contracts/specs-md-structure.md and document exported functions, configuration, security considerations, and test coverage." Include `Co-Authored-By` trailer with actual running model name and `Claude-Session` URL per CLAUDE.md rule 11

- [ ] T011 Commit audit verification and module-authoring.md update as single logical unit: `git commit -s` with subject "docs: audit module specs coverage and codify game module guideline", body: "Verify and record specs.md audit findings per US2 acceptance criteria (12 of 14 go.work modules + web have specs.md; svcutil and tunnel are sole gaps before feature completion). Add game module specs guideline to docs/module-authoring.md referencing canonical structure and noting enforcement boundary (gameplane-module repo's CI per D2)." Include trailers per rule 11

- [ ] T012 Commit hack/check-specs.sh and Makefile changes as single logical unit: `git commit -s` with subject "feat: add automated module specs compliance check", body: "Implement hack/check-specs.sh script (POSIX shell, <2 seconds per SC-002) and make check-specs target to validate all 15 workspace modules have non-empty specs.md files per FR-006, FR-007. Script parses go.work, detects missing/empty/whitespace-only specs.md per D3, reports diagnostics per contracts/check-specs.md output contract. Update lint target to depend on check-specs (rule 11 standing order)." Include trailers per rule 11

- [ ] T013 Commit CI integration as single logical unit: `git commit -s` with subject "ci: enforce specs compliance in lint job", body: "Add dedicated step to .github/workflows/ci.yaml lint job (gated if: matrix.module == 'netguard' per D5, single execution per trigger). Specs compliance now enforced in CI on every lint run, preventing future modules from merging without specs.md per Principle VI." Include trailers per rule 11

- [ ] T014 Commit CLAUDE.md Lint section update as separate logical unit: `git commit -s` with subject "docs: document make check-specs in CLAUDE.md", body: "Add mention of make check-specs target to Lint section per plan.md requirement. Specs compliance check is now integrated into standard developer tooling and CI enforcement." Include trailers per rule 11

- [ ] T015 Push branch to remote: `git push origin 011-add-missing-module-specs`; verify push succeeds and branch is visible on GitHub

- [ ] T016 Watch CI lint job run: `gh run watch` (or manually via GitHub Actions UI); verify the "check specs compliance" step in lint job (gated if: matrix.module == 'netguard') executes successfully, outputs "✓ Checked 15 modules: all have non-empty specs.md", and exits 0; verify all other CI jobs pass (golangci-lint across all modules, web ESLint, go tests, web tests, integration tests); **do NOT report work as complete or merge-ready until CI is fully green** per Principle VI and CLAUDE.md rule 8

- [ ] T017 Verify all tasks complete and feature acceptance criteria met: confirm `svcutil/specs.md` and `tunnel/specs.md` exist and match `contracts/specs-md-structure.md` structure checklist (quickstart.md Scenario 1); run `make check-specs` locally (permitted pre-flight compile-check exception per D6) and verify output lists all 15 modules and exits 0; review audit results in research.md/data-model.md; confirm `hack/check-specs.sh` implements `contracts/check-specs.md` output contract exactly (both success and failure cases); confirm Makefile targets, CI step, docs, and CLAUDE.md changes are correct and present; note any remaining gaps; confirm feature branch is in clean state and ready for PR creation and maintainer review

- [ ] T018 Create PR and apply labels: open PR via `gh pr create --title "docs: complete module specifications and automated compliance check" --body "..."` (body per plan.md PR Description), then apply required labels per CLAUDE.md rule 14 using `gh api -X POST repos/ValgulNecron/Gameplane/issues/<n>/labels -f "labels[]=type: docs" -f "labels[]=type: ci" -f "labels[]=area: specs" -f "labels[]=area: shared"` (substitute `<n>` with actual PR number from create output); verify labels applied with `gh api repos/ValgulNecron/Gameplane/issues/<n>/labels -q '[.[].name]|join(", ")'` per rule 14 Mechanics

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies. Can start immediately. BLOCKS all subsequent phases.
- **Foundational (Phase 2)**: Depends on Setup. No actual work; just establishes that no infrastructure tasks block the stories.
- **User Story 1 (Phase 3)**: Depends on Foundational. Can run in parallel with US2 and US3.
- **User Story 2 (Phase 4)**: Depends on Foundational. Can run in parallel with US1 and US3. T004 verification is more meaningful after US1 completes (confirming final 100% state).
- **User Story 3 (Phase 5)**: Depends on Foundational. Should run after US1 and US2 complete (validation targets a complete specs.md baseline).
- **Polish (Phase 6)**: Depends on all user stories (US1, US2, US3) being complete. Commits, pushes, and CI verification.

### User Story Dependencies

- **US1 (Author Specs)**: Independent. Can start after Setup/Foundational.
- **US2 (Audit)**: Independent initially. T004 verification step (verifying 100% compliance) is more meaningful after US1 completes.
- **US3 (Automated Check)**: Should follow US1 and US2 (check validates all modules after new specs exist and audit baseline is recorded).

### Within Each User Story

- **US1 Tasks (T002, T003)**: Marked [P]; can run in parallel (different files `svcutil/specs.md` vs. `tunnel/specs.md`, no dependency between them).
- **US2 Tasks (T004)**: Single task; no internal parallelism.
- **US3 Tasks (T005–T009)**:
  - T005 (script) can run in parallel with T008, T009 (docs changes)
  - T006 (Makefile) can run in parallel with T005, T008, T009
  - T007 (CI step) depends on T006 (Makefile target must exist before CI can invoke it); T007 should follow T006
  - T008, T009 (docs) can run in parallel with all US3 code tasks

### Parallel Opportunities

**Tier 1 (Setup)**: T001 alone.

**Tier 2 (After Setup)**: Can all run in parallel:
- T002, T003 (US1 svcutil/tunnel specs)
- T004 (US2 audit)
- T005, T006, T008, T009 (US3 script, Makefile, docs)

**Tier 3 (After Tier 2)**: 
- T007 (CI step) depends on T006; execute after T006 completes

**Tier 4 (Commits & Push)**: 
- T010–T014 (commits) execute sequentially as natural progression of changes
- T015 (push) after commits
- T016, T017 (verify CI and acceptance) after push

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001)
2. Complete Phase 3: User Story 1 (T002, T003)
3. **STOP and VALIDATE**: Both specs are written, match `contracts/specs-md-structure.md`, validate against FR-001–FR-004
4. At this point, Constitution Principle IV's per-module specs requirement is satisfied for `svcutil` and `tunnel`
5. Commit per rule 11 (T010)

This is the minimal viable feature: the two missing specs exist and follow the project standard.

### Full Feature Delivery (All User Stories)

1. Complete Setup (T001)
2. Complete US1 (T002, T003) and US2 audit (T004) in parallel after Setup
3. Complete US3 automated check (T005–T009) after US1/US2 to validate against a complete baseline
4. Polish & Verification (T010–T017): Commit per unit, push, verify CI green, confirm readiness for review
5. Feature complete and ready for PR creation and maintainer review

### Why US3 Makes It Stick (Why MVP ≠ Just US1)

US1 alone (writing `svcutil` and `tunnel` specs) satisfies Constitution Principle IV **at this moment**, but leaves future modules at risk: a developer could add a new Go module to `go.work` without a `specs.md` and not realize it. US3 (the automated check) prevents that by:

- Running in CI on every lint job (per D5, integrated into existing lint job as a step, not a new job)
- Catching missing/empty/whitespace-only `specs.md` immediately on every push
- Blocking merge until compliance is restored
- Enforcing the rule for all future work, not just this feature

**Therefore, the feature is not production-ready until US3 (automated enforcement) is in place.** US1 is the MVP for Constitution compliance at a point in time; US3 is what makes that compliance stick and scales it to future modules. Together, they ensure the repository stays compliant indefinitely.

---

## Notes

- **Total tasks**: 18 tasks (T001–T018)
- **Per-story task counts**:
  - Setup: 1 task (T001)
  - Foundational: 0 tasks (no work required, just understanding)
  - US1 (P1 - Author Specs): 2 tasks (T002, T003)
  - US2 (P1 - Audit): 1 task (T004)
  - US3 (P2 - Automated Check): 5 tasks (T005–T009)
  - Polish & Verification: 9 tasks (T010–T018, including commits, push, verification, and PR creation)

- **Parallel opportunities**:
  - T002 + T003 (svcutil and tunnel specs) can run in parallel
  - T004 (audit) can run in parallel with US1 or after
  - T005 + T006 + T008 + T009 can start in parallel for US3 (T007 depends on T006)

- **Per-task detail**: Every task specifies exact file path(s), section lists from `contracts/specs-md-structure.md`, FR(s) it satisfies (FR-001–FR-007), and maintainer ruling(s) it obeys (D1–D6).

- **No E2E tests**: Per Principle I exception documented in plan.md Complexity Tracking, this feature (documentation + repository-hygiene tooling, no runtime path) has no E2E test. Verification is the automated check's own negative test (simulated missing/empty specs.md → non-zero exit) in CI, exercised in the lint job per D6.

- **Commit strategy**: Per rule 11, commit after each logical unit (specs authorship as one unit, audit as one, US3 implementation in separate logical units by concern). Signed commits, conventional-commit prefixes (docs:, feat:, ci:), `Co-Authored-By` with actual running model, `Claude-Session` URL. Never amend; create new commits for fixes.

- **CI enforcement**: Per rule 8 and Principle VI, work is verified by pushing to branch and watching CI run **green** (not by local test/lint runs). Local `make check-specs` is permitted as a pre-flight compile-check exception per D6 and CLAUDE.md rule 8.

- **After merge**: Once the PR is approved and merged into `master`, per CLAUDE.md rule 12, delete the branch both remotely (`git push origin --delete 011-add-missing-module-specs`) and locally (`git branch -d 011-add-missing-module-specs`). Per rule 16, the spec folder will be renamed from `specs/011-add-missing-module-specs/` to `specs/done_011-add-missing-module-specs/` in a separate commit after merge, updating all cross-references per rule 16's rename requirement. The done_ rename is **not** a task in this file (happens after merge per rule 16).

- **PR labels** (per CLAUDE.md rule 14, to be added when opening the PR):
  - `type: docs` (svcutil/specs.md, tunnel/specs.md, docs/module-authoring.md, CLAUDE.md updates)
  - `type: ci` (hack/check-specs.sh, Makefile check-specs target, .github/workflows/ci.yaml step)
  - `area: specs` (all spec-related work)
  - `area: shared` (Makefile, CLAUDE.md, hack/, .github/ are repository-wide infrastructure)
