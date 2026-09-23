# Implementation Plan: v0.3 Release Readiness Audit

**Branch**: `018-v0-3-release-readiness` | **Date**: 2026-09-23 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/018-v0-3-release-readiness/spec.md`

## Summary

Take Gameplane from `v0.2.0-beta.8` to `v0.3.0`, its first release without a beta suffix, and back the release with evidence. The work has four parts:

1. Record the release criteria.
2. Build a feature inventory and a component list from the code.
3. Test every inventory item live on kubelab, and review every component.
4. Fix every finding, then re-verify each fix live. This repeats in rounds until the go/no-go report shows zero open findings.

Each audit round runs against a public release candidate (`v0.3.0-rc.N`) published by the existing `release.yaml` tag pipeline. Each publication needs the maintainer's approval first. The candidate is deployed to kubelab by a `helm upgrade` that changes only the image and chart version. All audit records are Markdown files under `specs/018-v0-3-release-readiness/audit/`. Fixes go through the normal branch → PR → human review → CI process. This branch carries only the audit records and the final status-wording change.

## Technical Context

**Language/Version**: No new product code of its own. Fixes land in the existing stack: Go 1.25 (`go.work`, 15 modules including `gp-module` and `test/e2e`), TypeScript strict / React 18 / Vite (`web/`), and Helm 3 (`charts/gameplane`).

**Primary Dependencies**: Existing release pipeline `.github/workflows/release.yaml` (GHCR images, OCI Helm chart, cosign signing, prerelease flag set for any tag containing `-`). Also `kubectl` and `helm` against `~/kubelab.yaml`, `curl` for API probes, and Chrome MCP for dashboard walkthroughs.

**Storage**: Audit records are Markdown files in the spec folder (FR-020). Evidence is small text logs and PNG screenshots under `audit/evidence/`. The kubelab API database is SQLite on a PVC. It is snapshotted before any upgrade.

**Testing**: CI (GitHub Actions) stays the verification authority for the unit, envtest, lint and E2E suites (FR-017, Constitution VI). Live procedures run on kubelab only. The constitution's exception for operator-provided infrastructure covers that, and kubelab is named in `Makefile:19,25`. Every fix PR carries a regression test, and an E2E test when the fix touches a user- or operator-facing path (Constitution I).

**Target Platform**: kubelab, the maintainer's three-node k3s cluster (control plane plus two workers), reached through `REMOTE_KUBECONFIG=~/kubelab.yaml`, context `default`.

**Project Type**: Release-engineering audit across a Kubernetes-native web platform (operator, API, agent, dashboard, auxiliary services, Helm chart).

**Performance Goals**: None of the product's own. Audit goal: a cold reader can reach the go/no-go conclusion from `audit/report.md` in under 15 minutes (SC-008).

**Constraints**:
- Pre-existing kubelab workloads, accounts and data must not change (FR-009, SC-006).
- Login rate limits: IP burst 10 (5/min), user burst 6 (3/min).
- Every RC tag publication needs maintainer approval (FR-019).
- No local test or lint suites (CLAUDE.md Rule 8).
- Dashboard visual fixes go through `design.pen` first (Constitution II).
- The `modules/` submodule must be initialised (`git submodule update --init`) before module work.

**Scale/Scope**: 15 Go workspace modules, `web/`, the Helm chart, `deploy/`, `hack/`, `.github/workflows/`, `docs/`, and root docs. Also 16 dashboard routes, about 25 API route groups, 9 CRDs, 11 agent capabilities, 6 auxiliary components, about 50 Helm toggles, and the bundled game modules. Consistency-only review for the `modules/` and `website/` submodules. The scout counts are in [research.md](research.md#r2-inventory-and-component-sources). The real inventory is enumerated from code during implementation.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Applies how | Status |
|---|---|---|
| I. E2E-Tested Delivery | The audit adds no feature. Each **fix** must ship with a regression test at the right layer: E2E in a `test/e2e/buckets.sh` bucket for user- or operator-facing paths. FR-012 automation proposals feed new E2E tests. The upgrade bucket baseline moves from `0.2.0-beta.5` to `0.2.0-beta.8` (`deploy/kind/upgrade.sh:36`, `.github/workflows/ci.yaml:970`). | PASS (by design) |
| II. Design-First | Any finding whose fix changes the dashboard's visual surface is done in `design.pen` via Pencil MCP, then exported to `design-export/`, before React changes. Non-visual web fixes are exempt. On the audit machine Pencil screenshots don't work. Design edits are made blind and logged in `OPEN-DECISIONS.md` for maintainer review, and no React change starts until the maintainer approves the design. | PASS (by design) |
| III. Language best practice | Fixes follow `%w` wrapping and strict TS, with no suppression directives. A lint-silencing "fix" is rejected in review. | PASS |
| IV. Spec-Driven | This plan, `tasks.md`, and the audit records live in the spec folder. Fixes that change module behaviour update that module's `specs.md` in the same PR. The `**Status:** beta (v0.2.0-beta.8)` lines in each `*/specs.md` are part of the FR-016 wording change. | PASS |
| V. Delegate to Workflows | The main loop orchestrates. Inventory enumeration, component reviews, live-procedure runs and fix waves run as `Workflow` fan-outs with an explicit `model:` on every call. They start at haiku and are reviewed one tier up. Browser smoke runs on sonnet. | PASS (by design) |
| VI. CI heavy lifting | Live runs on kubelab use the operator-provided-infrastructure exception, which does not cover the agent's local machine. CI green is still required for merge. A live pass never substitutes for CI (FR-017). | PASS |

No violations, so Complexity Tracking stays empty.

**Post-design re-check (after Phase 1)**: PASS. The contracts add no product code, no new abstractions, and no lint or test weakening. Two items are left open for the maintainer, not settled here: OD-005 (upgrade baseline if kubelab runs a build newer than beta.8), OD-006 (real node-loss test on a long-lived cluster), OD-007 (kubelab can't be reached from the audit machine) and OD-008 (live versus copy-based audit tamper test). See [OPEN-DECISIONS.md](OPEN-DECISIONS.md).

## Project Structure

### Documentation (this feature)

```text
specs/018-v0-3-release-readiness/
├── spec.md
├── OPEN-DECISIONS.md
├── plan.md              # this file
├── research.md          # Phase 0
├── data-model.md        # Phase 1: audit entities, fields, states
├── quickstart.md        # Phase 1: how to run and validate one audit round
├── contracts/
│   ├── audit-records.md     # file formats: inventory, findings, coverage, criteria, report
│   ├── test-resources.md    # naming, isolation, baseline snapshot, cleanup
│   ├── rc-deploy.md         # RC publish + kubelab deploy/restore procedure
│   └── status-wording.md    # v0.3.0 wording + every location to change
├── checklists/requirements.md
├── audit/               # created during implementation (FR-020)
│   ├── release-criteria.md
│   ├── kubelab-baseline.md
│   ├── inventory.md
│   ├── coverage.md
│   ├── findings.md
│   ├── rounds.md
│   ├── report.md
│   ├── procedures/<area>.md
│   └── evidence/<ID>/…
└── tasks.md             # /speckit-tasks
```

### Source Code (repository root)

The audit touches, but does not restructure, the existing layout:

```text
operator/ api/ agent/ web/ netguard/ gameaction/ gameproto/ gp-module/ svcutil/
sentinel/ capture-sidecar/ tunnel/ audit-syslog-bridge/ telemetry-receiver/ mcp-server/
test/e2e/                     # regression E2E for fixes; upgrade baseline bump
charts/gameplane/             # Chart.yaml version/appVersion → 0.3.0
deploy/kind/upgrade.sh        # FROM_VERSION default → 0.2.0-beta.8
.github/workflows/            # release.yaml (RC + final tags), ci.yaml (upgrade baseline)
docs/ README.md CLAUDE.md CHANGELOG.md   # status wording (contracts/status-wording.md)
hack/check-doc-versions.sh    # keeps version markers consistent
```

**Structure Decision**: There's no new source tree. Audit records live under the spec folder's `audit/` directory. Fixes land in each finding's owning component, on per-finding (or per-cluster) branches off `master`. The final wording and version change is one commit on this branch after the go decision.

## Complexity Tracking

No constitution violations to justify.
