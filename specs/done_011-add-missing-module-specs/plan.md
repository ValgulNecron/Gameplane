# Implementation Plan: Module Specifications Completion & Compliance Check

**Branch**: `011-add-missing-module-specs` | **Date**: 2026-09-01 | **Spec**: [./spec.md](./spec.md)

**Input**: Feature specification from `specs/011-add-missing-module-specs/spec.md`

## Summary

Complete the specification framework for all Gameplane modules by authoring the missing `specs.md` files for `svcutil` and `tunnel`, then codify automated compliance enforcement via a shell-based verification script integrated into the existing `make lint` target. This addresses Constitution Principle IV's mandate that every module maintain an authoritative specification document. The check validates that all 14 Go modules in `go.work` plus `web/` have valid, non-empty `specs.md` files, providing ongoing assurance that no future refactors introduce undocumented modules.

## Technical Context

**Language/Version**: Markdown documentation + POSIX shell/bash script; Go 1.26 (subjects of spec documentation)

**Primary Dependencies**: `coreutils` (POSIX sh/bash utilities: grep, find, wc, sed); Make (existing build system integration)

**Storage**: N/A (documentation and scripts only)

**Testing**: CI runs the positive check (all modules have non-empty specs.md) on every lint run via a step in `.github/workflows/ci.yaml` (gated to `matrix.module == 'netguard'`) that invokes `make check-specs`. The negative path (missing/empty/whitespace-only specs.md → check exits non-zero) is verified locally per D6 and quickstart.md Scenarios 3-4, and is not simulated in CI.

**Target Platform**: Linux CI runners + developer machines (any POSIX sh/bash environment)

**Project Type**: Documentation + repository hygiene tooling (no runtime component)

**Performance Goals**: Specification check completes in under 2 seconds (SC-002)

**Constraints**: No network access, no container runtime required, POSIX sh/bash + coreutils only, no new CI job (a dedicated step, gated `if: matrix.module == 'netguard'`, is added to the existing `lint` job per D5)

**Scale/Scope**: 14 Go modules in `go.work` + `web/` directory; 2 new specification files (`svcutil/specs.md`, `tunnel/specs.md`); 1 new shell script (`hack/check-specs.sh`); existing `Makefile` and `docs/module-authoring.md` receive targeted updates

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Justification |
|-----------|--------|---------------|
| **I. E2E-Tested Delivery** | PASS-WITH-JUSTIFICATION | This feature adds documentation and a repo-hygiene script with no user-facing runtime path (operator/user experience unchanged). Principle I's core mandate—verifiable end-to-end coverage—cannot literally apply. Verification: the check's positive path (all modules have non-empty specs.md, exit 0) is exercised in CI via the lint job on every run; the negative path (missing/empty specs.md → non-zero exit) is verified locally per D6 and quickstart.md Scenarios 3-4. |
| **II. Design-First for User-Facing Change** | N/A | No UI/dashboard changes; documentation only. |
| **III. Language & Ecosystem Best Practice** | PASS | Shell script follows POSIX sh/bash idioms; Markdown specs follow existing formatting conventions established across `sentinel/specs.md`, `capture-sidecar/specs.md`, `api/specs.md`, and others. POSIX/bash correctness is verified by the tier+1 (sonnet) review pass and by CI actually executing the script (a syntax error would surface as the job failing), not by any dedicated bash linter. |
| **IV. Spec-Driven Development** | PASS | This feature IS the remediation of Principle IV's per-module `specs.md` requirement. Completing missing specs for `svcutil` and `tunnel` directly fulfills the mandate; the compliance check enforces it going forward. |
| **V. Delegate to Workflows & Subagents** | PASS | Implementation delegated to haiku workers for `svcutil/specs.md` and `tunnel/specs.md` authorship, with mandatory tier-up (sonnet) review before acceptance. Fixes from review applied via re-launched small agents per workflow pattern. Main loop performs only decomposition, orchestration, and acceptance verification. |
| **VI. CI Bears the Heavy Lifting** | PASS | Specification check is verified in CI via a dedicated step (gated `if: matrix.module == 'netguard'`) in the existing lint job — CI does not invoke `make lint` itself (D5). No local test or lint suite execution required; developers push to branch, watch CI run the check, and fix failures with follow-up commits. Check itself runs on CI runners only (local `make check-specs`/`hack/check-specs.sh` is a permitted pre-flight exception per D6; `make lint` remains CI-only). |

**Post-Design Re-check**: Principles I–VI remain satisfied after Phase 1 design.

## Project Structure

### Documentation (this feature)

```text
specs/011-add-missing-module-specs/
├── plan.md                      # This file
├── research.md                  # Phase 0 research output
├── data-model.md                # Phase 1 data model (module audit results, check design)
├── quickstart.md                # Phase 1 quickstart (running the check, interpreting output)
├── contracts/
│   ├── check-specs.md           # Contract for hack/check-specs.sh
│   └── specs-md-structure.md    # Reference structure for canonical specs.md format
├── OPEN-DECISIONS.md            # Open questions from research (svcutil coverage clarification, modules/<game>/specs.md intent)
└── tasks.md                     # Phase 2 implementation tasks (created by /speckit-tasks, not here)
```

### Source Code (repository root)

```text
svcutil/
├── specs.md                     # NEW: Specification for stdlib-only utility module

tunnel/
├── specs.md                     # NEW: Specification for relay supervisor module

hack/
└── check-specs.sh               # NEW: Automated compliance verification script

Makefile
├── target: check-specs          # NEW: Runs hack/check-specs.sh
└── target: lint                 # MODIFIED: now depends on check-specs

docs/
└── module-authoring.md          # MODIFIED: adds guideline for modules/<game>/specs.md requirement

CLAUDE.md
└── [Lint section]               # MODIFIED: brief mention of make check-specs integration
```

**Structure Decision**: 

Two new specification files (`svcutil/specs.md`, `tunnel/specs.md`) follow the canonical structure established across existing module specs (Purpose, Responsibilities, Non-goals/boundaries, Directory & package layout, External interface/Configuration, Key Invariants, Dependencies, Security considerations, Testing & coverage, References). 

The compliance verification script (`hack/check-specs.sh`) is a POSIX shell script invoked by `make check-specs` and integrated as a dependency of the existing `make lint` target. This approach:

1. Reuses the existing CI infrastructure (a new step is added to the existing CI `lint` job's per-module matrix, e.g. gated to `matrix.module == 'netguard'`, that invokes `make check-specs`)
2. Requires no new CI job (per D5 maintainer ruling, which supersedes D1's CI half)
3. Validates all 14 Go modules from `go.work` plus `web/` in under 2 seconds
4. Detects missing or empty `specs.md` files with clear error reporting
5. Runs offline with only POSIX utilities (per D4)

The `docs/module-authoring.md` update codifies the guideline (not enforcement) for `modules/<game>/specs.md` files, per D2 maintainer ruling — enforcement of the `modules/` tree is out of scope here and belongs in the gameplane-module repo's own CI.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Principle I exception: no E2E test | Feature adds docs + tooling with no runtime path; user/operator behavior unchanged. Verification: CI runs the positive check on every lint run; the negative path (missing/empty specs.md) is verified locally per D6 and quickstart.md Scenarios 3-4, and is not simulated in CI. | Cannot invent an artificial E2E test on a feature with no user-facing deliverable; would violate Principle I's spirit (testing real paths, not test infrastructure). The positive check in CI + local verification of negative paths provides genuine reproducibility. |

## Phase Summary

- **research.md**: Detailed audit of existing module specifications across 13 files (structure, formatting, conventions, coverage gates, open questions about svcutil coverage gate and modules/<game>/specs.md requirements).
- **data-model.md**: Specification audit findings, check design (module enumeration from go.work + web/, empty file detection, error reporting), and contract definitions for the check script and reference specs.md format.
- **quickstart.md**: Step-by-step walkthrough of running `make check-specs`, interpreting success vs. failure output, and fixing common missing-file scenarios.
- **contracts/check-specs.md**: Formal contract for `hack/check-specs.sh` (inputs: none; outputs: exit 0 for compliance, non-zero + diagnostic for missing/empty specs; runtime <2s; POSIX-only dependencies).
- **contracts/specs-md-structure.md**: Reference template and canonical section ordering for new and existing module specifications (Title, Purpose, Responsibilities, Non-goals/boundaries, Directory & package layout, External interface/Configuration, Key Invariants, Dependencies, Security considerations, Testing & coverage, References).

---

**Constitution Re-check (post-Phase 1)**: Principles I–VI remain satisfied. Principle I's E2E exception is justified in Complexity Tracking above. No further violations.
