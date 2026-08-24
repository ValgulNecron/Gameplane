# Implementation Plan: Game Protocol E2E Coverage

**Branch**: `001-gameprotocol-e2e-coverage` | **Date**: 2026-08-16 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/done_001-gameprotocol-e2e-coverage/spec.md`

## Summary

Today the E2E suite boots a GameServer for all 16 shipped modules, but only **two**
(`minecraft-java`, `terraria`) prove a real protocol join. The other 14 assert
`ExpectDepth: "QUERY"` — an A2S server-list query, an RCON TCP dial, or a bare packet
exchange — which the spec's FR-006 explicitly rules out as join coverage. Nothing in the
repo records that gap in one place: it is spread across `PACKET_CAPTURE_NEEDED.md`, a
throwaway line in `docs/roadmap.md`, and the `bot_heavy` bucket list.

The plan closes the gap in four moves, in dependency order:

1. **Make the depth contract enforceable.** Promote the existing informal
   `JOINED | PARTIAL | QUERY` probe vocabulary into a typed, single-source-of-truth
   status model, and make "a join test exists" a machine-checkable property rather than
   a claim in prose.
2. **Make every probe two-sided.** FR-002 requires each probe to be proven to fail on a
   dead address *and* succeed on a real listener. Today that is a manual, one-off
   ritual. The harness will run the negative control automatically as part of every
   game's test, so a probe that cannot fail is a test failure, not an oversight.
3. **Close the joins that are actually reachable**, using the protocol work already
   in-tree (`gameproto/`, `test/e2e/internal/protocol/sourceproto`, `a2sproto`), and
   classify the rest honestly.
4. **Publish one tracked artifact** — `docs/game-coverage.md` — generated-and-verified
   against `modules/`, `test/e2e/buckets.sh`, and the test sources, so it cannot drift
   when a module is added, renamed, or removed. It supersedes `PACKET_CAPTURE_NEEDED.md`.

The honest scope boundary: this plan does **not** promise 16 live joins. Reverse-engineering
a dozen undocumented proprietary UDP protocols is not a planning-phase deliverable. What it
promises is that every module ends in exactly one *defensible, verified* state, and that no
module can quietly sit in an unknown one. See [Scope Reality](#scope-reality-what-100-can-mean-here).

## Technical Context

**Language/Version**: Go 1.25 (the `test/e2e` module in `go.work`, `//go:build e2e`), plus
POSIX shell for the bucket/coverage verifier and Markdown for the tracked artifact.

**Primary Dependencies**: existing in-repo only — `gameproto/` (Minecraft/Terraria codecs),
`test/e2e/internal/protocol/{a2sproto,sourceproto}`, the `RunGameProbe` in-cluster Job
runner (`test/e2e/gameprobe_job.go`), the `runGameBotTest` harness
(`test/e2e/gamebot_helpers_e2e_test.go`), `controller-runtime`/`client-go` for the E2E env,
kind + Helm for the cluster. **No new Go module, no new third-party dependency.**

**Storage**: N/A — no CRD, database, or persisted state is added. The coverage status is
derived from files in the tree, never stored as mutable state.

**Testing**: `test/e2e` (build tag `e2e`), executed as per-bucket CI jobs defined in
`test/e2e/buckets.sh`; the new coverage gate runs alongside the existing
`e2e bucket coverage` job in `.github/workflows/ci.yaml`.

**Target Platform**: Linux; Kubernetes 1.28+; kind on `ubuntu-latest` in CI, and the
operator-provided k3s cluster (`~/kubelab.yaml`) for deferred/heavy runs via
`GAMEPLANE_E2E_REUSE_CLUSTER=1` + `GAMEPLANE_E2E_CONTEXT`.

**Project Type**: test infrastructure + documentation inside the existing monorepo. No
dashboard, API, CRD, or operator surface changes.

**Performance Goals**: the default-CI bot bucket's wall clock must not regress beyond its
current budget — the negative control is a sub-second dial against a closed port and adds
no server boot. Any newly promoted JOINED test must boot inside the existing
`bot_fast` job budget or it belongs in the deferred set instead.

**Constraints**:
- GitHub Actions runner disk (~14 GB) is the hard gate on which modules may run in CI —
  the existing fast/heavy split is retained as-is, not re-litigated (spec Assumption 5).
- Nothing runs on the developer's machine (Constitution VI); CI is the oracle, with the
  operator-provided cluster as the named exception for deferred-test runs.
- API login rate limits bound the bot buckets; the added negative control performs **zero**
  API logins, so per-bucket login budgets are unaffected.
- `modules/` is a git submodule — the coverage verifier must handle and explicitly fail on
  an uninitialized submodule rather than silently reporting "0 modules, all covered".

**Scale/Scope**: 16 game modules today; 19 bot tests; 12 buckets. The design must stay
correct without edits when module #17 is added — the verifier's failure on an untracked
module is the mechanism.

## Constitution Check

*GATE: evaluated before Phase 0, re-checked after Phase 1 design. Result: **PASS**, no
violations to justify.*

| Principle | Assessment |
|---|---|
| **I. E2E-Tested Delivery** (non-negotiable) | This feature *is* the principle's enforcement mechanism. New/renamed tests keep `t.Parallel()`, per-test unique names, and existing shared-state guards. The heavy-game exclusion path the principle mandates (author + commit, exclude from buckets, comment why, keep on-demand runnable) becomes machine-checked rather than convention. The coverage verifier itself is a shell gate in the lint tier, following the existing `buckets.sh verify` precedent — it is a static consistency check with no cluster to exercise, so it correctly has no E2E test of its own; its own correctness is covered by shell unit fixtures (see contracts). |
| **II. Design-First for User-Facing Change** | **Exempt.** No dashboard or public-website *screen* changes. `docs/game-coverage.md` is a docs page; surfacing coverage status in the dashboard UI is explicitly out of scope. If that is ever wanted, it re-enters via `design.pen` first. |
| **III. Language & Ecosystem Best Practice** | New Go in the probe clients wraps errors with `%w`. No `//nolint`, no `eslint-disable`, no rule loosening. No CRD type edits, so no `make generate`/`make manifests` obligation. |
| **IV. Spec-Driven Development** | This plan follows `/speckit-specify` → `/speckit-plan`. The per-game wire-protocol record required by FR-005 **already exists** as `test/e2e/internal/<game>/spec.md` for all 16 games — this plan extends those files with a status header rather than inventing a second location. Two protocol-owning Go modules, `gameproto/` and `sentinel/`, are missing the `specs.md` the constitution requires; this plan adds them. The separate gap of a `specs.md` per `modules/<game>/` directory (describing the *template*, not the protocol) is real but belongs to the module repo and is **out of scope here** — flagged, not silently absorbed. |
| **V. Delegate to Workflows & Subagents** | Implementation fans out per game module — 16 near-independent units, ideal for parallel small-tier agents with a tier-up review pass. Research for this plan was already fanned out four ways. |
| **VI. CI Bears the Heavy Lifting** | No local suite runs. Default-CI coverage is proven by the `bot_fast` job; deferred coverage is proven on the operator-provided cluster, which is exactly the constitution's named exception, and the artifact records *when* that last happened. |

**Post-Phase-1 re-check**: unchanged — the Phase 1 design adds no new module, dependency,
persisted state, or user-facing surface, so no gate moves.

## Scope Reality: what "100%" can mean here

SC-001/SC-003 demand every module be covered or explicitly classified. Cross-referencing
`PACKET_CAPTURE_NEEDED.md` with the module list, the 16 modules fall into three genuinely
different situations, and the plan treats them differently rather than pretending they are
one problem:

- **Reachable now** — a real join is achievable with in-tree protocol work
  (`minecraft-java` ✅ and `terraria` ✅ already; `garrys-mod` is the closest open
  candidate — the challenge exchange is decoded and `sv_lan 1` removes the Steam gate;
  the blocker is field offsets in `C2S_CONNECT`, not authentication).
- **Blocked on documentation** — the protocol is undocumented upstream and needs a packet
  capture or reverse-engineering session before a client can exist at all
  (`factorio`, `7-days-to-die`, `v-rising`, `project-zomboid`, `dont-starve-together`,
  `enshrouded`, `rust`, `cs2`, `ark-survival-ascended`, `palworld`, `satisfactory`).
  These are **temporary** and each carries a named next artifact.
- **Architecturally blocked** — no headless client can ever complete the join
  (`valheim`: joins route through Steam Datagram Relay; `dayz`: BattlEye anti-cheat gates
  the join). These are marked **out-of-scope-by-design** per FR-008 and stop being open
  items.

The plan's deliverable is that all 16 sit in a verified, non-"unknown" state with the
evidence attached — not that all 16 execute a live join. This is a faithful reading of
FR-008 and Assumption 3, and it is the boundary reviewers should hold the implementation to.

## Project Structure

### Documentation (this feature)

```text
specs/done_001-gameprotocol-e2e-coverage/
├── plan.md              # This file
├── research.md          # Phase 0 output — decisions + rejected alternatives
├── data-model.md        # Phase 1 output — status vocabulary + coverage record schema
├── quickstart.md        # Phase 1 output — how to run/verify each tier
├── contracts/           # Phase 1 output — probe CLI, harness, verifier contracts
│   ├── probe-cli.md
│   ├── coverage-record.md
│   └── verifier.md
├── checklists/
│   └── requirements.md  # existing
└── tasks.md             # Phase 2 — created by /speckit-tasks, NOT by this command
```

### Source Code (repository root)

```text
test/e2e/
├── gamebot_helpers_e2e_test.go     # MODIFY: negative control per game; depth is typed
├── gameprobe_job.go                # MODIFY: -expect-fail mode; structured probe verdict
├── buckets.sh                      # MODIFY: bot buckets renamed by depth; commented exclusions
├── joincoverage.sh                 # NEW: verifier — modules/ x buckets x tests x doc
├── testdata/joincoverage/          # NEW: shell fixtures proving the verifier fails correctly
├── <game>_bot_e2e_test.go          # MODIFY (x16): one canonical test per module, depth-explicit
└── internal/
    ├── specs.md                    # MODIFY: document the depth contract + negative control
    ├── <game>/spec.md              # MODIFY (x16): all 16 already exist — add the status
    │                               # header (status, depth, blocker, last verified) that
    │                               # docs/game-coverage.md indexes
    └── protocol/
        ├── a2sproto/               # unchanged — query only, never counts as join
        ├── sourceproto/            # EXTEND: finish C2S_CONNECT for garrys-mod
        └── joindepth/              # NEW: shared depth type + verdict encoding

gameproto/
└── specs.md                        # NEW: required by Constitution IV, currently missing

sentinel/
└── specs.md                        # NEW: required by Constitution IV, currently missing

docs/
├── game-coverage.md                # NEW: the single tracked artifact (FR-007)
└── roadmap.md                      # MODIFY: point at game-coverage.md, drop the stale line

PACKET_CAPTURE_NEEDED.md            # DELETE — content migrates into docs/game-coverage.md
                                    # and the per-game test/e2e/internal/<game>/spec.md files

.github/workflows/ci.yaml           # MODIFY: run joincoverage.sh in the existing
                                    # "e2e bucket coverage" job
```

**Structure Decision**: everything lands in the existing `test/e2e` Go module and the
existing `docs/` tree. No new Go module is created — the project has an explicit rule
against pulling new test modules into the workspace, and the shared depth type is small
enough to live in `test/e2e/internal/protocol/joindepth/`. Critically, **no new per-game
documentation location is invented**: `test/e2e/internal/<game>/spec.md` already exists for
all 16 games, co-located with that game's probe client, which is exactly where a wire
protocol record belongs. `docs/game-coverage.md` is an index over those files, not a
replacement for them. The `modules/` submodule is **not touched** by this feature.

## Implementation Phasing

Sequenced so each step is independently reviewable and lands green.

1. **Depth contract + negative control** (`joindepth`, `gameprobe_job.go`,
   `gamebot_helpers_e2e_test.go`). No status changes yet; existing tests keep passing.
   This is the step that makes FR-002 and FR-009 structural.
2. **Codify the existing naming, don't redo it.** The tests are *already* named by depth —
   `TestGameServer_MinecraftJavaBot_Joined` and `..._TerrariaBot_Joined` versus fourteen
   `..._Query` tests. That convention is correct and needs no mass rename; what it lacks is
   enforcement, so nothing stops a future `..._Joined` test from quietly asserting QUERY.
   This step adds the check that a test's name suffix must agree with the depth it asserts,
   and comments `buckets.sh` so the bot-heavy bucket's never-run-by-default status is
   explicit rather than folklore.
3. **The verifier + the artifact** (`joincoverage.sh`, `docs/game-coverage.md`, CI wiring).
   At the end of this step the tree is honest: it reports 2 covered, 14 not, and CI enforces
   that the number is real.
4. **Per-game spec fan-out** — add the status header to each existing
   `test/e2e/internal/<game>/spec.md` (16 near-independent units) and write the two missing
   `specs.md` files for `gameproto/` and `sentinel/`. `PACKET_CAPTURE_NEEDED.md` is deleted
   in this step, not before — its per-game blocker detail migrates into the matching
   `test/e2e/internal/<game>/spec.md` first.
5. **Close what is closable** — `garrys-mod` `C2S_CONNECT` first, as the highest-confidence
   candidate with measured evidence already in-tree. Each additional module promoted from
   blocked → covered is its own branch, its own PR, its own coverage-artifact diff.

Steps 1–4 are the plan's committed scope. Step 5 is open-ended by nature and proceeds
per-game, which is why the artifact and gate come first: they make step 5's progress
visible one module at a time instead of as a single all-or-nothing push.

## Last Verified Discipline

Updating a module's **Last Verified** date in `docs/game-coverage.md` is licensed only by evidence of a successful protocol-join test execution. The discipline works as follows:

1. A deferred or on-demand test (one that does not run in the default CI job) can only update its `Last Verified` date via a successful run on either:
   - The operator-provided cluster (invoked with `GAMEPLANE_E2E_REUSE_CLUSTER=1 GAMEPLANE_E2E_CONTEXT=<name> GAMEPLANE_E2E_GAME_BOT=1`)
   - A on-demand CI job (a workflow dispatch or manual trigger)

2. The `Last Verified` update must be committed in the **same change** as (or immediately after) the successful run, with a reference to the run (CI job URL, local run date, or proof artifact).

3. The test harness records the depth result in its exit code and logs; a successful exit signals that the measured depth matches the expected depth in the test name and `docs/game-coverage.md`.

4. For default-CI tests (currently `minecraft-java` and `terraria`), `Last Verified` is updated on every successful default CI run. The CI job that runs these tests can automatically update the date to today's date as part of its pass logic.

5. The covered-deferred status (reserved for modules that have a real join implementation but are excluded from fast CI for performance reasons) entails a documented schedule: "Last Verified annually" or "Last Verified every 6 months", so a maintainer knows when the next on-demand run is due. When that run succeeds, the date is updated together with any blocking PRs.

This discipline prevents silent decay of coverage claims: a `Last Verified` date older than the documented schedule is visible proof that the deferred test needs a fresh run before the module can be relied upon.

## Complexity Tracking

> No Constitution Check violations. Nothing to justify.
