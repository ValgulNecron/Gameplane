---
description: "Task list for game protocol E2E coverage feature implementation"
---

# Tasks: Game Protocol E2E Coverage

**Input**: Design documents from `/specs/001-gameprotocol-e2e-coverage/` (spec.md, plan.md, data-model.md, contracts/)

**Prerequisites**: All specification documents (spec.md, plan.md, research.md, data-model.md, contracts/) and quickstart.md

**Project Rules**:
- Tests are NEVER run locally; CI is the oracle
- Every commit is signed (`git commit -s`) with a conventional-commit prefix; one logical unit per commit
- No lint suppression (`//nolint`, eslint-disable) — fix the code instead
- Go errors wrap with `%w`
- No CRD type changes in this feature, so no `make generate`/`make manifests` obligation
- A task is "done" when pushed and CI is green, not when it compiles locally

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create minimal project structure and directory scaffolding

- [ ] T001 Create test/e2e/internal/protocol/joindepth/ directory scaffold with package structure
- [ ] T002 Create test/e2e/testdata/joincoverage/ directory for verifier fixtures

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core typed infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T003 Implement JoinDepth typed value (QUERY/PARTIAL/JOINED) with ordering, stable uppercase wire encoding in test/e2e/internal/protocol/joindepth/depth.go
- [ ] T004 Implement ProbeVerdict type with ReachedDepth, Detail, Error fields and VERDICT-line encode/decode per contracts/probe-cli.md in test/e2e/internal/protocol/joindepth/verdict.go
- [ ] T005 Modify test/e2e/gameprobe_job.go: adopt typed JoinDepth, add -expect-fail mode to GameProbe invocation, surface distinct exit codes (0/2/3/1) via ProbeVerdict
- [ ] T006 Modify test/e2e/gamebot_helpers_e2e_test.go: runGameBotTest runs automatic negative control for every game (second probe against 127.0.0.1:1 with -expect-fail) and fails test if negative control does not fail for transport reason
- [ ] T007 [P] Update all 16 game probe binaries under test/e2e/internal/<game>/ to honour the new exit-code contract and VERDICT-line grammar per contracts/probe-cli.md (note: this is ONE task spanning: minecraft-java, terraria, valheim, dayz, garrys-mod, factorio, cs2, rust, ark-survival-ascended, palworld, satisfactory, 7-days-to-die, project-zomboid, dont-starve-together, enshrouded, v-rising)
- [ ] T008 Update test/e2e/internal/specs.md to document the JoinDepth contract, ProbeVerdict wire format, negative control mechanism, and the exit-code semantics per data-model.md

**Checkpoint**: Foundation ready — user story implementation can now begin in parallel. Verify: test/e2e/internal/protocol/joindepth/ compiles, GameProbe accepts typed ExpectDepth, probes emit VERDICT lines, negative controls run automatically.

---

## Phase 3: User Story 1 - Every shipped game module proves a real join (Priority: P1) 🎯 MVP

**Goal**: Promote minecraft-java and terraria to JOINED status, rename QUERY tests, and document garrys-mod opportunity.

**Independent Test**: Run the fast bot bucket; minecraft-java and terraria report JOINED and their negative controls pass.

- [ ] T009 [US1] Assert JOINED explicitly in test/e2e/minecraft_bot_e2e_test.go with automatic negative control verification
- [ ] T010 [US1] Assert JOINED explicitly in test/e2e/terraria_bot_e2e_test.go with automatic negative control verification
- [ ] T011 [US1] Add an enforcement check in test/e2e/gamebot_helpers_e2e_test.go that validates every bot test's name suffix (_Joined or _Query) matches the JoinDepth it asserts (JOINED vs QUERY), preventing future regressions; note that all 16 functions already satisfy this convention today (TestGameServer_MinecraftJavaBot_Joined, TestGameServer_TerrariaBot_Joined, and fourteen _Query tests for ArkBot, CS2Bot, DayZBot, DontStarveTogetherBot, EnshroudedBot, FactorioBot, GarrysModBot, PalworldBot, ProjectZomboidBot, RustBot, SatisfactoryBot, SevenDaysToDieBot, ValheimBot, VRisingBot)
- [ ] T012 [US1] Extend test/e2e/internal/protocol/sourceproto/ to finish the C2S_CONNECT packet field offsets for garrys-mod (challenge exchange already decoded; sv_lan 1 removes Steam gate). This task is allowed to end in a documented failure that keeps garrys-mod at blocked-doc; that is a legitimate outcome, not a blocked task.
- [ ] T013 [US1] Promote test/e2e/garrysmod_bot_e2e_test.go to JOINED if the C2S_CONNECT client lands in T012, else document the specific field-offset gap and close as blocked-doc
- [ ] T014 [P] [US1] Update status header (status=covered-in-ci, depth=JOINED) and migrate blocker details in test/e2e/internal/minecraft-java/spec.md and test/e2e/internal/terraria/spec.md using the template: `## Coverage Status\n\nStatus: [covered-in-ci|covered-deferred|blocked-doc|out-of-scope-by-design]\nDepth: [JOINED|PARTIAL|QUERY|—]\nBlocker: [reason or —]\nBlockerClass: [documentation|architectural|—]`

**Checkpoint**: minecraft-java and terraria are marked as covered-in-ci with JOINED depth and migration complete. Name-suffix enforcement check in place for all bot tests. garrys-mod status documented.

---

## Phase 4: User Story 2 - Heavy games get a written, on-demand test (Priority: P2)

**Goal**: Document heavy-game exclusions, verify they remain runnable on demand, establish LastVerified discipline.

**Independent Test**: A heavy module's test compiles and is absent from every bucket the default CI job runs; its documented on-demand invocation is accurate.

- [ ] T015 [P] [US2] Add commented exclusion rationale for every heavy module in test/e2e/buckets.sh using inline comments on the bucket_bot_heavy() definition, explaining multi-GB image size, sustained CPU/RAM, or other resource constraints per spec FR-003
- [ ] T016 [P] [US2] Verify that each of the first 7 heavy modules (factorio, cs2, rust, ark-survival-ascended, palworld, satisfactory, dayz) can be run on demand with GAMEPLANE_E2E_REUSE_CLUSTER=1 + GAMEPLANE_E2E_CONTEXT + GAMEPLANE_E2E_GAMES=<game>, and document the exact invocation in each game's test/e2e/internal/<game>/spec.md for on-demand execution. The obligation is to confirm the documented invocation is valid and that the test actually reaches the game server; a protocol-level failure is a legitimate outcome that gets recorded as that module's blocker in docs/game-coverage.md, NOT a reason to leave the task incomplete
- [ ] T017 [P] [US2] Verify that each of the remaining 6 heavy modules (7-days-to-die, project-zomboid, dont-starve-together, enshrouded, valheim, v-rising) can be run on demand with GAMEPLANE_E2E_REUSE_CLUSTER=1 + GAMEPLANE_E2E_CONTEXT + GAMEPLANE_E2E_GAMES=<game>, and document the exact invocation in each game's test/e2e/internal/<game>/spec.md for on-demand execution. The obligation is to confirm the documented invocation is valid and that the test actually reaches the game server; a protocol-level failure is a legitimate outcome that gets recorded as that module's blocker in docs/game-coverage.md, NOT a reason to leave the task incomplete
- [ ] T018 [US2] Establish that a successful on-demand run is what licenses updating the lastVerified date in docs/game-coverage.md (to be created in Phase 5), and document this workflow in plan.md

**Checkpoint**: All 13 heavy modules have a documented, on-demand invocation. Last Verified date discipline is established.

---

## Phase 5: User Story 3 - Blocked and undocumented protocols are tracked (Priority: P3)

**Goal**: Create the single tracked artifact, implement the verifier, ensure every module has exactly one status.

**Independent Test**: joincoverage.sh verify passes on the real tree and fails on every fixture; every module has exactly one status.

- [ ] T019 [US3] Author docs/game-coverage.md with the machine-readable Markdown table per contracts/coverage-record.md, containing exactly 16 rows matching the initial state: 2 covered-in-ci (minecraft-java, terraria), 0 covered-deferred, 12 blocked-doc, 2 out-of-scope-by-design (valheim — Steam Datagram Relay; dayz — BattlEye anti-cheat, both architectural per FR-008); note that every module with a test records Test and Bucket columns truthfully, even when blocked-doc or out-of-scope. Per FR-010, every blocked-doc row must have a Blocker cell naming the specific unblocking artifact required (e.g., a packet capture of a real client's join, a reverse-engineered field map, or vendor documentation) — not a vague "protocol undocumented". The two out-of-scope-by-design rows are exempt; their Blocker names a permanent architectural constraint, not a missing artifact
- [ ] T020 [US3] Implement test/e2e/joincoverage.sh verifier with all 16 hard checks + 1 warning per contracts/verifier.md (Checks 1-16 and W1). The checks validate: modules submodule initialized, every module in coverage record, no duplicates, no strays, status/depth consistency, covered modules have test/bucket/lastVerified, test names exist in source, covered modules have valid buckets, bucket names recognized, covered-in-ci not in bot-heavy, blocked modules have blocker/blockerClass, blocked-doc rows' Blocker field names a specific unblocking artifact per FR-010 (heuristic keyword matching; out-of-scope-by-design rows exempt), test/bucket cross-references match, bot-test names agree with depth. Warning W1 covers staleness for deferred > 90 days old.
- [ ] T021 [P] [US3] Author failure fixtures under test/e2e/testdata/joincoverage/ for checks 1-8: case-uninitialized-submodule/, case-missing-module/, case-stray-module/, case-duplicate-module/, case-covered-with-query-depth/, case-covered-without-test/, case-invalid-test-name/, case-covered-without-bucket/ (each fixture must make the verifier exit non-zero; 8 fixtures total)
- [ ] T022 [P] [US3] Author failure fixtures under test/e2e/testdata/joincoverage/ for checks 9-16: case-bad-bucket-name/, case-covered-in-ci-in-bot-heavy/, case-deferred-without-lastverified/, case-blocked-without-blocker/, case-blocked-doc-without-artifact/, case-architectural-not-out-of-scope/, case-test-bucket-mismatch/, case-bot-test-depth-mismatch/ (each fixture must make the verifier exit non-zero; 8 fixtures total). The case-blocked-doc-without-artifact fixture tests the FR-010 requirement that blocked-doc rows must name a specific unblocking artifact. Note: the staleness check (W1, deferred > 90 days) is a WARNING (not a hard failure) and is exercised inline by the verifier's own tests rather than by a fixture directory
- [ ] T023 [US3] Wire test/e2e/joincoverage.sh verify into .github/workflows/ci.yaml: (1) add a paths filter to the `changes` job in `.github/workflows/ci.yaml` matching `modules/**`, `test/e2e/**`, and `docs/game-coverage.md`; (2) add a conditional output from `changes` to gate the "e2e bucket coverage" job; (3) ensure the verifier runs as a step in the "e2e bucket coverage" job when only the modules/ submodule or docs/game-coverage.md changed
- [ ] T024 [P] [US3] Add status header to test/e2e/internal/<game>/spec.md for the first 7 games (garrys-mod, factorio, cs2, rust, ark-survival-ascended, palworld, satisfactory), migrating each game's blocker detail out of PACKET_CAPTURE_NEEDED.md into the per-game spec.md with the template: `## Coverage Status\n\nStatus: [covered-in-ci|covered-deferred|blocked-doc|out-of-scope-by-design]\nDepth: [JOINED|PARTIAL|QUERY|—]\nBlocker: [reason or —]\nBlockerClass: [documentation|architectural|—]`. For blocked-doc games, per FR-010, the Blocker field must name the specific unblocking artifact (e.g., packet capture, reverse-engineered field map, or vendor documentation), not a vague description
- [ ] T025 [P] [US3] Add status header to test/e2e/internal/<game>/spec.md for the remaining 7 games (7-days-to-die, project-zomboid, dont-starve-together, enshrouded, v-rising, valheim, dayz), migrating each game's blocker detail out of PACKET_CAPTURE_NEEDED.md into the per-game spec.md with the template: `## Coverage Status\n\nStatus: [covered-in-ci|covered-deferred|blocked-doc|out-of-scope-by-design]\nDepth: [JOINED|PARTIAL|QUERY|—]\nBlocker: [reason or —]\nBlockerClass: [documentation|architectural|—]`. For blocked-doc games, per FR-010, the Blocker field must name the specific unblocking artifact (e.g., packet capture, reverse-engineered field map, or vendor documentation), not a vague description
- [ ] T026 [US3] Delete PACKET_CAPTURE_NEEDED.md — ONLY after all migration tasks (T024, T025) are merged and blocker details have migrated into per-game spec.md files

**Checkpoint**: docs/game-coverage.md is the single tracked artifact with full column fidelity. joincoverage.sh verify passes on the real tree and fails on all 16 fixtures. All 16 modules have exactly one status. CI changes job gates e2e-bucket-coverage runs on module/docs changes. PACKET_CAPTURE_NEEDED.md is deleted.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Complete specification, update documentation, validate end-to-end.

- [ ] T027 Write gameproto/specs.md (Constitution IV requirement) documenting the Minecraft and Terraria wire-protocol codecs, their join-depth measurement capabilities, and any version constraints
- [ ] T028 Write sentinel/specs.md (Constitution IV requirement) documenting the sentinel's protocol parsing, wake-on-connect integration, PARTIAL-depth measurement for join attempts, and interaction with gameproto/ codecs
- [ ] T029 Update docs/roadmap.md to point at docs/game-coverage.md as the canonical source of coverage status, and remove the stale line "only Minecraft and Terraria are bot-testable"
- [ ] T030 Run the eight scenarios in specs/001-gameprotocol-e2e-coverage/quickstart.md to validate end-to-end correctness, with explicit venue per scenario: Scenario 1 (static verifier — run as static shell check; authoritative run is CI), Scenario 2 (bucket gate — static shell check; authoritative run is CI), Scenario 3 (verifier can fail — static shell check; authoritative run is CI), Scenario 4 (run bot_fast — execute test suite on CI or operator-provided cluster only, never locally; use GAMEPLANE_E2E_REUSE_CLUSTER=1 + GAMEPLANE_E2E_CONTEXT), Scenario 5 (single-game iteration — execute on CI or operator-provided cluster only; use GAMEPLANE_E2E_REUSE_CLUSTER=1 + GAMEPLANE_E2E_CONTEXT + GAMEPLANE_E2E_GAMES), Scenario 6 (deferred heavy game — execute on CI or operator-provided cluster only; use GAMEPLANE_E2E_REUSE_CLUSTER=1 + GAMEPLANE_E2E_CONTEXT + GAMEPLANE_E2E_GAMES), Scenario 7 (negative control proof — execute on CI or operator-provided cluster only; use GAMEPLANE_E2E_REUSE_CLUSTER=1 + GAMEPLANE_E2E_CONTEXT), Scenario 8 (acceptance checklist — review only, not an execution step)
- [ ] T031 Record the separate, out-of-scope gap: modules/<game>/specs.md files do not exist and belong to the gameplane-module repo (not this one) — flag it in the PR description or add a comment to docs/roadmap.md pointing to the gap in the submodule

**Checkpoint**: All specification documents are complete. Documentation is current. End-to-end validation passes.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3–5)**: All depend on Foundational phase completion
  - Phase 3 (US1) can start after Foundational
  - Phase 4 (US2) can start after Foundational (independent of US1, but may share spec.md updates)
  - Phase 5 (US3) can start after Foundational (independent of US1 and US2)
- **Polish (Phase 6)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1 / MVP)**: Can start after Foundational completion. No dependencies on US2 or US3. US1 owns T014 (minecraft-java and terraria spec.md migration).
- **User Story 2 (P2)**: Can start after Foundational completion. Independent of US1 and US3. T016 and T017 write to different spec.md files (no conflict).
- **User Story 3 (P3)**: Can start after Foundational completion. Independent of US1 and US2. Coordinates with them only at the point where coverage status flows into docs/game-coverage.md (created in T019).

**Critical Note**: US3 is independently valuable and could ship BEFORE US1 completes. The coverage artifact's honest initial state (2 covered, 12 blocked-doc, 2 out-of-scope) does not depend on any join being promoted — it simply records the current truth. When US1 promotes minecraft-java or garrys-mod from blocked to covered, docs/game-coverage.md is updated in a follow-up commit/PR.

### Within Each User Story

- All tests (if any) must be written before implementation (not applicable here; this is a test harness feature)
- Foundational tasks must complete before story-specific implementation
- Story tasks can run in parallel where marked [P]
- Story complete before moving to next priority

### Parallel Opportunities

- **Phase 1**: All tasks can run in sequence (only 2, no parallelism benefit)
- **Phase 2**: T007 is independent of T003–T006; can run T003–T006 first, then T007 in parallel (if T003–T006 are blocking)
- **Phase 3**: T014 [P] can run in parallel with other story tasks.
- **Phase 4**: T015 [P], T016 [P], T017 [P] can all run in parallel (different spec.md files, different heavy modules).
- **Phase 5**: T021–T022 [P] can run in parallel (different fixture directories). T024–T025 [P] can run in parallel (different spec.md files). T019–T020 should run first (they establish the artifact and verifier).
- **Phase 6**: All tasks can run independently after previous phases.

---

## Parallel Example: Phase 3 Parallelism

After Foundational phase (Phase 2) completes:

```
Agent A (Haiku): T012 — Extend sourceproto for garrys-mod
Agent B (Haiku): T013 — Promote garrysmod_bot_e2e_test.go or document blocked-doc closure
Agent C (Haiku): T011 — Add name-suffix enforcement check to gamebot_helpers_e2e_test.go
Agent D (Haiku): T014 — Update minecraft-java + terraria spec.md with status headers and migration

[All complete, then]

Agent E (Sonnet): Review all Phase 3 outputs, verify spec.md format matches, garrys-mod opportunity documented, enforcement check in place
```

Once review passes, create a single PR or sequence of PRs to merge the work.

---

## Parallel Example: Phase 5 Parallelism

After Foundational phase (Phase 2) completes:

```
Agent A (Haiku): T019 — Author docs/game-coverage.md
Agent B (Haiku): T020 — Implement joincoverage.sh verifier

[Both complete, then in parallel:]

Agent C (Haiku): T021–T022 — Author failure fixtures (checks 1-8 + checks 9-16, 16 fixtures total)
Agent D (Haiku): T024 — Add status headers to first 7 spec.md files (garrys-mod through satisfactory)
Agent E (Haiku): T025 — Add status headers to remaining 7 spec.md files (7-days-to-die through dayz)

[All complete, then]

Agent F (Haiku): T023 — Wire joincoverage.sh into CI with changes job filter (depends on T020)
Agent G (Haiku): T026 — Delete PACKET_CAPTURE_NEEDED.md (depends on T024–T025)

[All complete, then]

Agent H (Sonnet): Review all Phase 5 outputs
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

To deliver the MVP (confirmed protocol joins for 2 modules + name-suffix enforcement):

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Run the default-CI bot bucket (`make test-e2e-bucket BUCKET=bot-fast`); confirm minecraft-java and terraria report JOINED with passing negative controls; confirm name-suffix enforcement check passes on all 16 bot tests.
5. Merge and deploy if ready

At this point, the project has:
- Two games with proven real protocol joins
- All bot tests comply with naming convention (suffix matches depth)
- Regression check in place for future bot tests
- Automatic negative control on every game test
- A foundation for future coverage expansion

### Incremental Delivery

Continued delivery with all three stories:

1. Complete Phase 1 + Phase 2 → Foundation ready
2. Add Phase 3 → Test independently → Merge (MVP!)
3. Add Phase 4 → Test independently → Merge (Heavy games documented)
4. Add Phase 5 → Test independently → Merge (Coverage tracking artifact live)
5. Add Phase 6 → Complete documentation → Merge (Specification complete)

Each story adds value without breaking previous stories.

### Parallel Team Strategy

With multiple developers:

1. Team completes Phase 1 + Phase 2 together
2. Once Phase 2 is done:
   - Developer A: Phase 3 (US1)
   - Developer B: Phase 4 (US2, starting after T015 dependencies)
   - Developer C: Phase 5 (US3, starting with T020–T021)
3. Stories complete and integrate independently
4. Developer D: Phase 6 (Polish) — can run after Phase 2, not blocked by stories

---

## Notes

- `[P]` tasks = different files, no merge conflicts, can run in parallel
- `[US1]`/`[US2]`/`[US3]` labels = user story traceability
- Each user story should be independently completable and testable
- Every task produces one or more commits (one logical unit per commit)
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same-file conflicts, cross-story hard dependencies that break independence
- No local tests run; CI is the oracle
- All commits signed and conventional-commit formatted
