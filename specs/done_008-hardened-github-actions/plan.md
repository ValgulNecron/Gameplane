# Implementation Plan: Hardened GitHub Actions CI/CD, AI Automation & Multi-Module Dependabot

**Branch**: `008-hardened-github-actions` | **Date**: 2026-08-29 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/done_008-hardened-github-actions/spec.md`

## Summary

Harden the repository's entire GitHub Actions surface — 5 workflows, 4 composite actions,
and the Dependabot configuration — against supply-chain and injection attack, and add a
new AI pull-request review workflow that is safe on fork PRs.

The work is configuration-only. No Go, TypeScript, CRD, or dashboard code changes. Four
concrete gaps were measured against the live tree (see [research.md](./research.md)):

| Gap | Measured state | Target state |
|---|---|---|
| Action SHA pinning | 0 / 18 external actions pinned (all float on mutable tags) | 18 / 18 pinned to 40-char SHAs with `# vX.Y.Z` comments |
| Job-level permissions | 24 / 26 jobs inherit workflow-level tokens | every job declares its own minimal `permissions` |
| Job timeouts | 9 / 26 jobs have no `timeout-minutes` (all of `images`, `publish-edge`, `release`, `republish-modules`) | 26 / 26 bounded |
| Dependabot coverage | 1 gomod dir (`/`, covers 0 of 14 modules), 1 docker dir (`/`, matches no Dockerfile) | 14 gomod dirs, 12 docker dirs, npm, github-actions — all grouped |

Plus two additions: a redaction filter in `dump-cluster-state` (currently dumps container
logs with no sanitisation at all), and two new workflows (`ai-review.yaml` and
`ai-review-respond.yaml`) that split the collect/review jobs on the trust boundary so fork
PRs never see a write token and the reviewer's code runs from the base branch, not the PR.

Enforcement is verified through code review and static analysis jobs in CI: `actionlint`
validates schema and expression-injection safety; `zizmor` validates action SHA pins,
permissions declarations, and `pull_request_target` misuse.

## Technical Context

**Language/Version**: YAML (GitHub Actions workflow schema) + POSIX `sh`/Bash 5 for
composite-action steps. Node 24 for the existing inline `render.cjs` reporter.
No Go or TypeScript source changes.

**Primary Dependencies**: GitHub Actions runners (`ubuntu-latest`, `ubuntu-24.04-arm`);
18 external third-party actions (inventory in [contracts/action-pins.md](./contracts/action-pins.md));
`anthropics/claude-code-action` for User Story 4; `gh` CLI (preinstalled on runners);
`actionlint` for workflow validation.

**Storage**: N/A — all state is workflow-run artifacts and GitHub's own status/comment APIs.

**Testing**: Existing `test/e2e/buckets.sh verify`, `test/e2e/lint-gate-verify.sh verify`,
`test/e2e/joincoverage.sh verify`. End-to-end proof is the CI run itself on this branch
(Constitution Principle VI) plus `actionlint` validation and a deliberate-regression check
documented in [quickstart.md](./quickstart.md).

**Target Platform**: GitHub-hosted runners on a public repository (ARM64 runners are free
for public repos, which the existing matrix already relies on).

**Project Type**: CI/CD and repository automation configuration.

**Performance Goals**: No CI wall-clock regression. SHA pinning is a parse-time change with
zero runtime cost. The AI review workflow must post its comment within 5 minutes of the
triggering run completing.

**Constraints**:
- Fork PRs get a read-only `GITHUB_TOKEN` regardless of declared `permissions`; every job
  that writes (statuses, PR comments) must already degrade gracefully — the `report` job
  does this today and is the pattern to follow.
- `pull_request_target` MUST NOT be introduced. It checks out untrusted code with a write
  token; the `workflow_run` split is the safe alternative (research.md, D-05).
- `COSIGN_PRIVATE_KEY` and registry credentials stay confined to `publish-edge.yaml`,
  `release.yaml`, `images.yaml`, `republish-modules.yaml`. No test or review workflow may
  reference them.
- Dependabot's `github-actions` ecosystem updates SHA pins in place and rewrites the
  version comment — so pinning must not fight the bot. Comment format must be exactly
  `# vX.Y.Z` after the SHA.
- Concurrency groups must not cancel `push: master` runs that gate publishing.

**Scale/Scope**: 5 baseline workflow files (~97 KB, `ci.yaml` alone is 60 KB / 1400 lines), 4
composite actions, 1 Dependabot config, 26 baseline jobs, 18 distinct external actions, 14 Go
modules, 12 Dockerfiles. Two new workflow files (splitting User Story 4's collect/review jobs
on the trust boundary): `.github/workflows/ai-review.yaml` (collect, `pull_request` trigger)
and `.github/workflows/ai-review-respond.yaml` (review, `workflow_run` trigger).

## Constitution Check

*GATE: evaluated before Phase 0, re-evaluated after Phase 1 design, and corrected 2026-08-30.*
*Result: **FAIL** — Principle V violated. Recorded, not waived.*

| Principle | Applies? | Assessment |
|---|---|---|
| **I. E2E-Tested Delivery** (NON-NEGOTIABLE) | **UNRESOLVED** | This feature has no user- or operator-facing runtime path — no CRD, API, agent or dashboard behavior changes — so there is no `test/e2e/` Go test to write and no bucket to add. That much is factual. **Needs a ruling**: declare the principle inapplicable to configuration-only work. |
| **II. Design-First for User-Facing Change** | No | No dashboard or website visual surface is touched. `design.pen` and `website.pen` are not opened, read, or edited. Explicitly exempt: "Backend-only, API-only, and operator-only changes are exempt" — CI configuration is further from the visual surface still. |
| **III. Language & Ecosystem Best Practice** | Yes | No in-source suppression is introduced anywhere. `.golangci.yml` and `web/eslint.config.js` are not touched. Shell follows the repo's existing `set -euo pipefail` convention from `buckets.sh`. |
| **IV. Spec-Driven Development** | Yes | Following the lifecycle: spec.md exists, this plan is `/speckit-plan`, `/speckit-tasks` next. No `specs.md` module file is affected — `.github/` is not a Go module, `web/`, or a `modules/<game>/` directory, so the per-module `specs.md` requirement does not reach it. |
| **V. Delegate to Workflows & Subagents** | **VIOLATED** | This planning session ran entirely in the main loop. Principle V requires the main loop to delegate through `Workflow`, and CLAUDE.md rule 13 states the same. There is no exemption for planning work. Recorded in Complexity Tracking as an unjustified violation, not a waiver. |
| **VI. CI Bears the Heavy Lifting** | Yes | Load-bearing here: this feature *is* CI. Correctness is proven by pushing the branch and watching the run green — which for this feature is both the test and the subject under test. |

**Gate result**: FAIL — one violated principle (V) and one unresolved (I). Both are recorded
in Complexity Tracking per the Governance requirement that a violation be stated explicitly
or the change be redesigned. Neither is waived here.

## Project Structure

### Documentation (this feature)

```text
specs/done_008-hardened-github-actions/
├── plan.md                       # This file
├── research.md                   # Phase 0 — 10 decisions, all NEEDS CLARIFICATION resolved
├── data-model.md                 # Phase 1 — the 5 config entities and their validation rules
├── quickstart.md                 # Phase 1 — 7 runnable validation scenarios
├── contracts/
│   ├── action-pins.md            # The 18 action → SHA pin table (authoritative)
│   ├── permissions-matrix.md     # Per-job permission + timeout contract, all 26 jobs
│   ├── dependabot-matrix.md      # Ecosystem × directory × group contract
│   └── ai-review-contract.md     # Trigger model, token posture, prompt-isolation rules
├── checklists/                   # Pre-existing
└── tasks.md                      # Phase 2 — NOT created by /speckit-plan
```

### Source Code (repository root)

```text
.github/
├── dependabot.yml                # REWRITE — 4 entries → 28 entries with groups
├── workflows/
│   ├── ci.yaml                   # MODIFY — job permissions, SHA pins, injection audit
│   ├── images.yaml               # MODIFY — SHA pins, +timeouts, +job permissions
│   ├── publish-edge.yaml         # MODIFY — SHA pins, +timeouts, +job permissions
│   ├── release.yaml              # MODIFY — SHA pins, +timeouts, +job permissions
│   ├── republish-modules.yaml    # MODIFY — SHA pins, +timeouts, +job permissions
│   ├── ai-review.yaml            # NEW — collect job (PR head, untrusted, pull_request trigger)
│   └── ai-review-respond.yaml    # NEW — review job (base branch, privileged, workflow_run trigger)
└── actions/
    ├── build-e2e-images/action.yml   # MODIFY — SHA pins
    ├── dump-cluster-state/action.yml # MODIFY — SHA pins + redaction filter (FR-014)
    ├── e2e-images/action.yml         # MODIFY — SHA pins
    └── go-cache/action.yml           # MODIFY — SHA pins

test/e2e/                         # UNCHANGED — buckets, lint-gate, joincoverage stay as-is
charts/ operator/ api/ agent/ web/ # UNCHANGED — no product code in this feature
```

**Structure Decision**: This is a repository-automation feature, so none of the template's
application layouts (single-project / web / mobile) apply. The tree above is the real
scope: everything lives under `.github/`, with all changes to workflows and actions applied
in-place and two new workflow files (`ai-review.yaml` and `ai-review-respond.yaml`) added.
The split is required by the collect/review trust boundary: a workflow has a single `on:` block
for the entire file, so expressing both a `pull_request` trigger and a `workflow_run` trigger
in one file would require guard clauses that collapse the security boundary.

## Phase Sequencing

Ordered by dependency, mapping to the spec's priorities:

1. **Foundation — pins and timeouts** (US1, FR-001…FR-006, SC-001/002). SHA-pin all 18
   actions across 9 files; add missing `timeout-minutes` and per-job `permissions`; wire
   the workflow-lint gate (actionlint + zizmor) into `ci.yaml` to validate schema, action
   pins, and injection safety. Everything downstream inherits this.
2. **E2E diagnostics hardening** (US2, FR-012…FR-016, SC-005). Redaction filter in
   `dump-cluster-state`; confirm the artifact-reuse and concurrency invariants the spec
   asserts already hold.
3. **Dependabot rewrite** (US3, FR-017…FR-021, SC-003). Independent of 1 and 2 — can land
   in parallel. Directory list is derived mechanically from `go.work` and `find -name
   Dockerfile`, not from the spec's prose list (see research.md D-08 for the correction).
4. **AI review workflow** (US4, FR-022…FR-025, SC-006). Depends on 1 for the pin
   convention and workflow-lint validation (actionlint + zizmor) that the new file must
   satisfy on arrival.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|---|---|---|
| **Principle I** — coverage for configuration-only work | The feature's entire surface is repository configuration evaluated by GitHub's own runner. There is no live control plane, CRD reconciliation, dashboard flow, or wire protocol to exercise — an e2e test would have nothing to connect to. | Writing a Go e2e test that shells out to parse `.github/*.yaml` would add a Kind cluster boot, a bucket slot, and a login budget to a check that needs none of them — strictly more cost for strictly less clarity, and it would misfile a config gate as a cluster test. Relying on human review instead was rejected outright: SC-001 is a 100% claim, and 18-of-18 pins cannot be held by discipline across future PRs. |
| **Principle V** not applied — planning and implementation both ran in the main loop | **Not justified.** A harness default ("do not use workflows unless the user requested it") was treated as outranking rule 13 and Principle V, which are themselves a standing instruction from the maintainer to delegate. The condition "unless the user requested it" was already satisfied. | Nothing. This should have been a Workflow from the start. Left recorded rather than quietly fixed, per the Governance requirement that violations be stated explicitly. |

## Proposed out of scope — NOT RULED ON

These are the agent's suggestions, not decisions. They were originally written as settled
scope exclusions without being asked; see OPEN-DECISIONS.md D-H and the resolution in D-J.
The claim that `actionlint` covers R1/R4/R6 was disproven by ruling D-J: actionlint covers
R6 (injection) only; R1 (pins) is covered by `zizmor`; R4 (timeouts) has no mechanical
enforcement and is code-review-only (per ENFORCEMENT REALITY in D-J).

Suggested for exclusion:

- Changing what any test asserts, adding coverage, or moving coverage thresholds.
- Restructuring the e2e bucket split or the `changes` path-filter logic.
- Migrating to reusable workflows (`workflow_call`) or splitting the 1400-line `ci.yaml`.
  Tempting, but it is a refactor with a large blast radius and no security payoff; it
  would also make the pinning diff unreviewable. Note it for a later spec.
- Self-hosted or larger runners.
- Adding new game images or E2E buckets.
- Branch protection rules, rulesets, or org-level Actions policy (not in-repo, not
  reviewable in a PR diff).
