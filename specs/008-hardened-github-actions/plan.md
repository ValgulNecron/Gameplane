# Implementation Plan: Hardened GitHub Actions CI/CD, AI Automation & Multi-Module Dependabot

**Branch**: `008-hardened-github-actions` | **Date**: 2026-08-29 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/008-hardened-github-actions/spec.md`

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
logs with no sanitisation at all), and a new `ai-review.yaml` workflow built on the
`pull_request_target`-free `workflow_run` pattern so fork PRs never see a write token.

Enforcement is not left to review discipline: a new `.github/workflows-verify.sh` gate,
wired into the existing `changes`/`report` CI structure, mechanically asserts SC-001
through SC-004 on every run, in the same style as the existing `buckets.sh verify` and
`lint-gate-verify.sh verify` gates.

## Technical Context

**Language/Version**: YAML (GitHub Actions workflow schema) + POSIX `sh`/Bash 5 for the
verifier and composite-action steps. Node 24 for the existing inline `render.cjs` reporter.
No Go or TypeScript source changes.

**Primary Dependencies**: GitHub Actions runners (`ubuntu-latest`, `ubuntu-24.04-arm`);
18 external third-party actions (inventory in [contracts/action-pins.md](./contracts/action-pins.md));
`anthropics/claude-code-action` for User Story 4; `gh` CLI (preinstalled on runners);
`yq`/`python3` (preinstalled) for the verifier's YAML parsing.

**Storage**: N/A — all state is workflow-run artifacts and GitHub's own status/comment APIs.

**Testing**: `.github/workflows-verify.sh verify` (new, runs in CI and locally); existing
`test/e2e/buckets.sh verify`, `test/e2e/lint-gate-verify.sh verify`,
`test/e2e/joincoverage.sh verify`. End-to-end proof is the CI run itself on this branch
(Constitution Principle VI) plus a deliberate-regression check documented in
[quickstart.md](./quickstart.md).

**Target Platform**: GitHub-hosted runners on a public repository (ARM64 runners are free
for public repos, which the existing matrix already relies on).

**Project Type**: CI/CD and repository automation configuration.

**Performance Goals**: No CI wall-clock regression. The verifier job must complete in
< 60 s and run only on `.github/**` changes. SHA pinning is a parse-time change with zero
runtime cost. The AI review workflow must post its comment within 5 minutes of the
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

**Scale/Scope**: 5 workflow files (~97 KB, `ci.yaml` alone is 60 KB / 1400 lines), 4
composite actions, 1 Dependabot config, 26 jobs, 18 distinct external actions, 14 Go
modules, 12 Dockerfiles. Two new files: `.github/workflows/ai-review.yaml`,
`.github/workflows-verify.sh`.

## Constitution Check

*GATE: evaluated before Phase 0 and re-evaluated after Phase 1 design. Result: **PASS** (both passes).*

| Principle | Applies? | Assessment |
|---|---|---|
| **I. E2E-Tested Delivery** (NON-NEGOTIABLE) | Adapted | This feature has no user- or operator-facing runtime path — it changes no CRD, API, agent or dashboard behavior, so there is no `test/e2e/` Go test to write and no bucket to add. The equivalent executable proof is `.github/workflows-verify.sh verify`, a machine gate asserting SC-001…SC-004, wired into CI so a regression reddens a PR exactly as a bucket test would. It is proven the same way a join probe must be: it MUST be shown to fail against a deliberately-regressed workflow before it is trusted (quickstart.md, scenario 6). Existing e2e buckets are untouched and must stay green — that is the second half of the evidence. Recorded in Complexity Tracking. |
| **II. Design-First for User-Facing Change** | No | No dashboard or website visual surface is touched. `design.pen` and `website.pen` are not opened, read, or edited. Explicitly exempt: "Backend-only, API-only, and operator-only changes are exempt" — CI configuration is further from the visual surface still. |
| **III. Language & Ecosystem Best Practice** | Yes | No in-source suppression is introduced anywhere. The hardening moves in the opposite direction — the new verifier makes a class of defect (unpinned action, unbounded job, over-broad token) mechanically uncheckable-around. `.golangci.yml` and `web/eslint.config.js` are not touched. Shell follows the repo's existing `set -euo pipefail` convention from `buckets.sh`. |
| **IV. Spec-Driven Development** | Yes | Following the lifecycle: spec.md exists, this plan is `/speckit-plan`, `/speckit-tasks` next. No `specs.md` module file is affected — `.github/` is not a Go module, `web/`, or a `modules/<game>/` directory, so the per-module `specs.md` requirement does not reach it. |
| **V. Delegate to Workflows & Subagents** | Deferred | An execution-time principle, not a design-time one; it governs `/speckit-implement`, where the task waves in `tasks.md` are fanned out. Note this session's own operator instruction ("do not use workflows or agents unless requested") overrode delegation for the planning step itself — user instruction outranks the constitution's default per the Governance section's deference to explicit direction. Recorded in Complexity Tracking. |
| **VI. CI Bears the Heavy Lifting** | Yes | Load-bearing here: this feature *is* CI. Nothing is validated locally beyond `workflows-verify.sh` (a static parser, not a suite) and YAML syntax checks. Correctness is proven by pushing the branch and watching the run green — which for this feature is both the test and the subject under test. |

**Gate result**: PASS. Two entries in Complexity Tracking, both justified and both narrow.

## Project Structure

### Documentation (this feature)

```text
specs/008-hardened-github-actions/
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
├── workflows-verify.sh           # NEW — the SC-001…SC-004 enforcement gate
├── workflows/
│   ├── ci.yaml                   # MODIFY — job permissions, SHA pins, injection audit
│   ├── images.yaml               # MODIFY — SHA pins, +timeouts, +job permissions
│   ├── publish-edge.yaml         # MODIFY — SHA pins, +timeouts, +job permissions
│   ├── release.yaml              # MODIFY — SHA pins, +timeouts, +job permissions
│   ├── republish-modules.yaml    # MODIFY — SHA pins, +timeouts, +job permissions
│   └── ai-review.yaml            # NEW — fork-safe AI PR review (workflow_run split)
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
scope: everything lives under `.github/`, with one new sibling script
(`.github/workflows-verify.sh`) placed next to what it verifies rather than in `test/e2e/`
— it validates repository configuration, not cluster behavior, and has no Kind or Go
dependency. It follows the `verify` subcommand convention of `test/e2e/buckets.sh` so the
CI wiring and the developer muscle-memory match.

## Phase Sequencing

Ordered by dependency, mapping to the spec's priorities:

1. **Foundation — pins and the verifier** (US1, FR-001…FR-006, SC-001/002). SHA-pin all 18
   actions across 9 files; add missing `timeout-minutes` and per-job `permissions`; write
   `workflows-verify.sh` and wire it into `ci.yaml`. Everything downstream inherits this.
2. **E2E diagnostics hardening** (US2, FR-012…FR-016, SC-005). Redaction filter in
   `dump-cluster-state`; confirm the artifact-reuse and concurrency invariants the spec
   asserts already hold and lock them in the verifier.
3. **Dependabot rewrite** (US3, FR-017…FR-021, SC-003). Independent of 1 and 2 — can land
   in parallel. Directory list is derived mechanically from `go.work` and `find -name
   Dockerfile`, not from the spec's prose list (see research.md D-08 for the correction).
4. **AI review workflow** (US4, FR-022…FR-025, SC-006). Depends on 1 for the pin
   convention and the verifier, which the new file must satisfy on arrival.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|---|---|---|
| **Principle I** satisfied by a static verifier rather than a `test/e2e/` Go test in a bucket | The feature's entire surface is repository configuration evaluated by GitHub's own runner. There is no live control plane, CRD reconciliation, dashboard flow, or wire protocol to exercise — an e2e test would have nothing to connect to. `workflows-verify.sh` provides the same guarantee the principle exists to give: a mechanical, CI-enforced check that reddens on regression, proven to fail before it is trusted. | Writing a Go e2e test that shells out to parse `.github/*.yaml` would add a Kind cluster boot, a bucket slot, and a login budget to a check that needs none of them — strictly more cost for strictly less clarity, and it would misfile a config gate as a cluster test. Relying on human review instead was rejected outright: SC-001 is a 100% claim, and 18-of-18 pins cannot be held by discipline across future PRs. |
| **Principle V** not applied to this planning session | The operator's session-level instruction explicitly directed that no workflows or subagents be used. Constitution Governance defers to explicit human direction, and Principle V itself warns against escalating the delegation rule past its scope. | Delegating anyway would override a direct instruction. The constraint is scoped to this planning session only — `/speckit-implement` fans the `tasks.md` waves out per Principle V as normal, starting at `haiku` with tier-up review. |

## Out of Scope

Named explicitly so `/speckit-tasks` does not widen into them:

- Changing what any test asserts, adding coverage, or moving coverage thresholds.
- Restructuring the e2e bucket split or the `changes` path-filter logic.
- Migrating to reusable workflows (`workflow_call`) or splitting the 1400-line `ci.yaml`.
  Tempting, but it is a refactor with a large blast radius and no security payoff; it
  would also make the pinning diff unreviewable. Note it for a later spec.
- Self-hosted or larger runners.
- Adding new game images or E2E buckets.
- Branch protection rules, rulesets, or org-level Actions policy (not in-repo, not
  reviewable in a PR diff).
