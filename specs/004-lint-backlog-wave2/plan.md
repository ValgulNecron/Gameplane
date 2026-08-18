# Implementation Plan: Lint Backlog Wave 2 — Static Analysis Gate for api, agent, test/e2e

**Branch**: `004-lint-backlog-wave2` | **Date**: 2026-08-17 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/004-lint-backlog-wave2/spec.md`

## Summary

Three of the project's largest and most security-sensitive Go modules—the public API
gateway (`api`), the in-pod agent (`agent`), and the E2E test suite (`test/e2e`)—are
currently exempted from the static-analysis gate that every other module in `go.work`
must pass. This creates an unequal quality floor: defects in authentication, RCON, file
access, network I/O, and cluster scripting go uncaught by linters, while smaller,
less-critical modules are held to a stricter standard.

The plan brings all three modules under the same gate by:

1. **Measuring the true backlog** via CI on a fresh branch (the starting count for
   `api`, `agent`, and `test/e2e` against the current codebase is unknown and cannot
   be locally enumerated per Constitution VI).
2. **Salvaging completed work** from PR #216 where possible (api and agent fix commits
   cleanly cherry-pick; test/e2e is stale and will be redone against master).
3. **Fixing all findings** via real code changes rather than suppression directives,
   preserving the project's zero-suppression property.
4. **Enabling the matrix** in CI with correct build tags (`--build-tags=envtest` for
   `api`, `--build-tags=e2e` for `test/e2e`), which lands only after all fixes are
   green so the build log stays clean.

## Technical Context

**Language/Version**: Go 1.26 (the `go.work` workspace modules `api`, `agent`, `test/e2e`)

**Primary Dependencies**: golangci-lint v2.12.2 via golangci-lint-action@v9; 14 enabled
linters (bodyclose, errcheck, gosec, govet, ineffassign, staticcheck, unused, misspell,
revive, unparam, nilerr, noctx, errorlint, contextcheck); `.golangci.yml` v2 schema

**Storage**: N/A

**Testing**: The lint gate itself is the deliverable. CI is the only place it runs
(Constitution VI). Verification happens by reading diffs and checking CI logs, not local
execution.

**Target Platform**: Linux; GitHub Actions ubuntu-latest; runs on every push to a
branch and on merge to main

**Project Type**: CI/quality infrastructure inside the existing monorepo. No dashboard,
CRD, API route, or operator surface changes.

**Constraints**:
- The three modules entering the gate total ~342 .go files across 40+ packages
  (`api`: 199 files, 13 packages; `agent`: 64 files, 17 packages; `test/e2e`: 79
  files, 23 packages; partitionable by package directory for parallel work).
- Frozen surfaces (audit field names and chained-hash logic, append-only migrations,
  e2e test names in `buckets.sh`, game protocol byte layouts, rate-limit thresholds,
  Prometheus metric names) must remain unchanged; refactoring is permitted as long as
  the external interface stays identical.
- Build-tag-conditional files (`//go:build envtest` in `api`, `//go:build e2e` in
  `test/e2e`) must be analyzed; CI must pass the corresponding `--build-tags` flag to
  the linter.
- No new in-source suppression directives anywhere; this property remains absolute
  and unweakened. Five new scoped, config-level exclusions in `.golangci.yml` were
  authorized during this work, each narrow (single file/path, single linter rule),
  documented, and reviewed — in addition to the pre-existing gosec G115 exclusion
  (Minecraft VarInt codec): gosec G302 (agent mod extraction — extracted files must
  stay group-readable for the game container's uid); gosec G704 (api ws dialer —
  upstream URL built from request-path values already validated as DNS-1123 labels);
  gosec G124 (api CSRF cookie — deliberately non-`HttpOnly` for the double-submit
  pattern); gosec G204 (test/e2e kubectl helper — args are trusted, in-repo test
  code and the helper rejects shell metacharacters); gosec G402 (test/e2e
  Satisfactory probe — self-signed cert dialed over a pod-local address only).
  See `contracts/exclusion-policy.md` for the full inventory and rationale.

**Scale/Scope**: 13 go.work modules total; 3 newly gated; ~342 .go files in scope;
14 linters; matrix runs in parallel, adding ~3 concurrent jobs to existing CI (no
extra wall-clock).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design. Result:
**PASS**, one note on Principle IV.*

| Principle | Assessment |
|---|---|
| **I. E2E-Tested Delivery** (non-negotiable) | The lint gate itself is not an E2E feature; it is a correctness infrastructure gate. No E2E test suite is required for the feature. Findings uncovered by the gate are fixed via code review and CI validation (Constitution VI), not via end-to-end testing. The gate is not negotiable per se, but its implementation as a CI matrix addition is verified by pushing a branch and reading the CI run log. |
| **II. Design-First for User-Facing Change** | **Exempt.** No dashboard or public-website screen changes. The CI configuration change is infrastructure-only; no visual surface is added. |
| **III. Language & Ecosystem Best Practice** | **Directly Enforced.** This feature implements Principle III itself — it mandates that all findings reported by golangci-lint be fixed, not silenced, across the entire workspace. No `//nolint`, no `// nosec`, no rule removals from `.golangci.yml`. The single authorized gosec G115 exclusion is documented and remains untouched. |
| **IV. Spec-Driven Development** | **Implementation Adds New `specs.md` Requirement.** The fixes applied to `api`, `agent`, and `test/e2e` may alter those modules' observable behavior or documented responsibilities. Any such change MUST be accompanied by a corresponding `specs.md` update in the same commit. `api/specs.md`, `agent/specs.md`, and `test/e2e/specs.md` are either already present or will be created/updated as part of this work to document any behavioral changes the fixes introduce. |
| **V. Delegate to Workflows & Subagents** | Work partitions naturally by package directory within each module (no file overlap between api, agent, test/e2e; package-level granularity within each allows parallel work). The implementation phase fans out per-package/per-module fixes to small agents with a tier-up review before landing the matrix change. |
| **VI. CI Bears the Heavy Lifting** | Finding counts are measured via CI, not locally. The true backlog across all three modules is unknown until the first linter run on a fresh branch against master (this becomes Phase 1's first step). No local build, test, or lint runs occur. All verification is via CI logs and diff review. |

## Project Structure

### Documentation (this feature)

```text
specs/004-lint-backlog-wave2/
├── plan.md              # This file
├── spec.md              # The feature specification
└── checklists/
    └── requirements.md  # existing
```

### Source Code (repository root)

```text
.github/workflows/
└── ci.yaml              # MODIFY: Add api, agent, test/e2e to the golangci-lint matrix
                         # (lines ~180). Add build-tags steps for envtest (api) and e2e
                         # (test/e2e). Remove exclusion comment (lines 177-178).

api/
├── **/*.go              # FIX: Resolve all golangci-lint findings (188 original across
│                        # all files; package-level partitionable work).
├── specs.md             # UPDATE/CREATE: Document any behavioral changes fixes introduce.
└── .testcoverage.yml    # No change (80% gate remains).

agent/
├── **/*.go              # FIX: Resolve all golangci-lint findings (148 original across
│                        # all files; package-level partitionable work).
├── specs.md             # UPDATE/CREATE: Document any behavioral changes fixes introduce.
└── .testcoverage.yml    # No change (90% gate remains).

test/e2e/
├── **/*.go              # FIX: Resolve all golangci-lint findings in current master
│                        # (count unknown until first CI run; package-level partitionable).
├── specs.md             # UPDATE/CREATE: Document any behavioral changes fixes introduce.
├── buckets.sh           # No change (test naming/bucketing is frozen).
└── // (no coverage gate)

.golangci.yml           # Five new scoped exclusions authorized (G302 agent mods,
                        # G704 api ws dialer, G124 api CSRF cookie, G204 test/e2e
                        # kubectl helper, G402 test/e2e Satisfactory probe), plus
                        # the pre-existing G115 exclusion for gameproto/minecraft.go,
                        # unchanged. No in-source suppression directives anywhere.
                        # See contracts/exclusion-policy.md.
```

**Structure Decision**: The fix work is localized to three modules (`api`, `agent`,
`test/e2e`). The CI matrix change lands in `.github/workflows/ci.yaml` as a single
step. No new modules, no new packages, no new linters are introduced. All three modules
already exist in `go.work` and are compilable; they are simply being added to the
existing lint job's matrix.

## Salvage Decision and Scope

**Decision: PARTIAL SALVAGE — cherry-pick api and agent; redo test/e2e against master.**

**Evidence**:
- PR #216 branch `chore/lint-backlog-wave2` has fix commits ba32d0b (api) and f5b9ede
  (agent) that touch zero files modified in master since the fork at b3d5b38.
- These two commits are mechanically reusable and can be cherry-picked onto a fresh
  branch off master without conflict.
- test/e2e fix commit e7b99b6 on the same branch is stale: feature 001
  (gameprotocol-e2e-coverage) merged after the fork and rewrote test/e2e heavily
  (108 files changed, 54 new, 4,279 insertions, structural changes to the probe infrastructure).
- A `git merge-tree` against current master produces no textual conflicts. The case for
  redoing `test/e2e` is not merge difficulty but staleness: master's `test/e2e` has
  moved substantially since the fork (108 files changed, 54 of them new, 4,279 insertions,
  including the new `internal/protocol/joindepth` infrastructure), so the branch's fixes
  were written against code that has since been restructured and would need a fresh lint
  pass regardless.
- PR #216 contains no suppression directives; this decision honors that property and
  redoes only the part that cannot be salvaged.

**Implementation consequence**: The true finding count for test/e2e against current
master is unknown and will be measured in Phase 1 when CI runs the linter for the first
time on the fresh branch.

## Measuring the True Backlog

The original enumeration across the three modules on unfixed code (api 188, agent 148,
test/e2e 152, totaling ~488) is based on runs against older code. The immediate
predecessor CI run on PR #216 (after fixes landed on that branch) showed 91 remaining,
but that branch is now stale relative to master, which received feature 001 and other
changes in the interim.

**Process**:
1. **Phase 1 (Setup)**: Create a fresh branch off master with salvaged api and agent work. No matrix change yet; CI does not yet analyze these three modules.
2. **Phase 2 (Foundational)**: Land the CI matrix enablement commit EARLY, adding api, agent, and test/e2e to the matrix with NO fixes applied. Push to CI. This triggers a deliberately RED run — the red state is intentional and is the authoritative baseline measurement. Retrieve finding counts from CI logs (Constitution VI: no local linter runs).
3. **Phases 3-5 (Fixing & Verification)**: Implement fixes in per-package commits (Phase 3), verify the zero-suppression property (Phase 4), and implement gate-regression contract rules (Phase 5). The branch is red during active fix work, which is expected and the honest state. Once all fixes land and verifications pass, the branch goes green.
4. **Phase 6 (Polish)**: Final validation, collateral checks, documentation.

**Why this ordering**: The matrix lands EARLY (Phase 2) so CI provides measurement mid-work on a deliberately-red run, giving the team visibility into the baseline. Fixing happens against a known measurement, not blindly. The matrix is never reverted; once enabled in Phase 2, it stays as fixes land. This transparency is preferable to measuring blindly or enabling the matrix only after all work is done. Per Constitutional VI, a red state on a feature branch during active development is expected and acceptable.

## Implementation Phasing

Sequenced so each step is independently reviewable and the branch goes red (during fix work) → green (when all fixes and verifications are complete) → merged to master (green). The key insight from research.md Decision 3: the CI matrix enablement lands EARLY on the feature branch (its first run is deliberately red, serving as the baseline measurement), fixes follow, verifications confirm the fix quality, and the branch ends green.

**Phase 1: Setup (Shared Infrastructure)**
- Create a fresh branch `004-lint-backlog-wave2` off master.
- Cherry-pick api fix commit ba32d0b and agent fix commit f5b9ede from PR #216.
- Push to CI. The golangci-lint matrix will not yet include api, agent, or test/e2e,
  so they are not checked by CI.
- This phase establishes the branch and salvaged work, with no measurement yet.

**Phase 2: Foundational (Blocking Prerequisites)**
- Land the CI matrix enablement commit EARLY, adding api, agent, and test/e2e to the
  matrix.module list with NO fixes applied. This commit modifies `.github/workflows/ci.yaml`
  to add the three modules, include conditional build-tag steps (`--build-tags=envtest`
  for api, `--build-tags=e2e` for test/e2e), and remove the exclusion comment.
- Push this commit to CI. This triggers a deliberately RED CI run — the red state is
  intentional and is the authoritative baseline measurement.
- Retrieve finding counts from CI logs. Per Constitution VI, no local linter runs are
  performed; CI is the oracle.
- This phase gives the team visibility into the true current finding count and sets a
  clear baseline. The matrix stays enabled throughout all subsequent phases; it is never
  reverted.

**Phase 3: User Story 1 (P1 MVP) — "Three modules brought under the uniform lint gate"**
- Fix all golangci-lint findings across api, agent, and test/e2e via real code changes
  (improved error handling, added context parameters, variable renames, etc.). No
  suppression directives are introduced.
- Partition work BY PACKAGE DIRECTORY per research.md Decision 4, enabling parallel work
  across all three modules concurrently. Each worker owns one package; no file overlap
  between workers. This scales to package count and avoids merge conflicts.
- Fixes can land per-package as ready; do not wait for full modules to be fixed before
  committing. The branch is red during active fix work, which is expected and honest.
- If a fix requires updating a behavioral contract, update the corresponding `specs.md`
  (`api/specs.md`, `agent/specs.md`, `test/e2e/specs.md`) in the same commit.
- Checkpoint: All three modules pass golangci-lint with zero findings when correct build
  tags are passed. The branch is green.

**Phase 4: User Story 2 (P2) — "Findings are fixed, not suppressed"**
- Verify and prove the zero-suppression property is preserved. No `//nolint`, `//#nosec`,
  or `//lint:ignore` directives are introduced in api, agent, or test/e2e.
- Review a sample of landed fix commits to confirm they contain real code changes, not
  deletions or artificial narrowing of analysis scope.
- Confirm `.golangci.yml` has gained zero new exclusions beyond the three pre-existing
  ones (test exemptions, controller revive exemption, gameproto G115 exemption).
- Checkpoint: Zero suppression directives introduced. All landed fixes are real code
  changes. Configuration exclusion list is unchanged.

**Phase 5: User Story 3 (P3) — "The gate cannot silently regress"**
- Implement the lint-gate contract rules (R-001 through R-010 from contracts/lint-gate.md)
  as CI verifications or scripts to prevent future regressions.
- Create a verifier script (e.g., `test/lint-gate-verify.sh`) that checks contract
  invariants such as: `go.work` module list stays in sync with CI matrix, no
  `continue-on-error: true` in lint steps, no temporary/pending comments on the lint gate,
  build tags are correctly passed.
- Wire the verifier into `.github/workflows/ci.yaml` so it runs before linting.
- Document the lint-gate contract and verification rules in `specs/004-lint-backlog-wave2/specs.md`.
- Checkpoint: Lint-gate contract rules are automated and documented. Regressions are
  detectable.

**Phase 6: Polish & Cross-Cutting Concerns**
- Confirm that `api/.testcoverage.yml` (80% gate) and `agent/.testcoverage.yml` (90%
  gate) still pass after all fixes. If coverage dropped, add targeted tests to recover it.
- Verify that the "e2e bucket coverage" CI job still passes. Note: e2e test names are
  frozen; renaming a test silently breaks the test→bucket mapping in `test/e2e/buckets.sh`.
- Run the quickstart.md scenarios on a real cluster. All 8 scenarios must execute without
  error. If a scenario fails due to this feature's changes, roll back the offending fix.
- Update `CLAUDE.md` if it contains any stale claim that api, agent, or test/e2e are
  "unlinted" or "exempt from the lint gate".
- Checkpoint: All collateral checks pass. Documentation is current. The feature is ready
  for merge.

**Rationale for ordering**: Early measurement (Phase 2) via a deliberately-red CI run gives the team visibility into the baseline. Parallel-by-package fixes (Phase 3) across all three modules concurrently scale far better than serial per-module work, because each package is touched by exactly one worker, avoiding file conflicts (per research.md Decision 4). Verification phases (4 and 5) prove that fixes were real and that the gate has regression guards. Polish phase ensures no collateral damage. The matrix lands in Phase 2 and stays enabled; this transparency is preferable to measuring blindly or enabling only after all work is done.

## Known Implementation Traps

Lessons from Wave 1 (PR #215) that implementers should keep in mind:

1. **net.DialTimeout → DialContext**: Switching from `DialTimeout` to `DialContext`
   silently drops the timeout unless you also set `Dialer.Timeout`. Check both.
2. **contextcheck on graceful shutdown**: Graceful-shutdown code paths often use
   `context.WithoutCancel` to detach the context from cancellation; contextcheck flags
   this as a missing timeout. Fix with `context.WithTimeout(context.WithoutCancel(ctx),
   d)`, do not dismiss as a false positive.
3. **gosec G101 renames**: Renaming a variable flagged by G101 (suspicious naming) must
   avoid every substring matching `(?i)passwd|pass|password|pwd|secret|token|pw|
   apiKey|bearer|cred`. Update the declaration AND every reference together; partial
   renames leave dangling references.
4. **Deliberate fallback vs. hard error**: A naive nilerr fix can turn an intentional
   "if X fails, use the default" pattern into a hard failure. Read the context before
   fixing.
5. **Build-tag-specific call sites**: Signature changes must update call sites inside
   `//go:build envtest` and `//go:build e2e` files. Ordinary tooling skips tag-gated
   files, so these breakages are invisible until a tag-scoped build runs — and under
   this project's rules that only happens in CI. Verify via the tag-scoped CI lint/build
   job output; never run the check locally.

## Complexity Tracking

> No Constitution Check violations. Nothing to justify.
