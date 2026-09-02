# Implementation Plan: Dedicated Server Modules for Top Steam Games

**Branch**: `014-top-steam-game-modules` | **Date**: 2026-09-03 | **Spec**: [./spec.md](./spec.md)

**Input**: Feature specification from `specs/014-top-steam-game-modules/spec.md`

## Summary

Expand Gameplane's first-party game module catalog to support all 26 top-played multiplayer games from the Steam top 100 that support user-hosted dedicated servers. The implementation will author 13 new modules (`fivem`, `team-fortress-2`, `farming-simulator-25`, `euro-truck-simulator-2`, `mount-and-blade-2-bannerlord`, `tmodloader`, `beammp`, `left-4-dead-2`, `the-isle`, `ark-survival-evolved`, `arma-reforger`, `hell-let-loose`, `squad`) and standardize the 13 existing modules (`cs2`, `palworld`, `rust`, `project-zomboid`, `dayz`, `garrys-mod`, `terraria`, `7-days-to-die`, `ark-survival-ascended`, `factorio`, `dont-starve-together`, `valheim`, `satisfactory`). Each module will deliver a complete package (`module.yaml`, `template.yaml`, `README.md`, `specs.md`, `samples/server.yaml`) meeting all static preflight checks in `modules/validate.py` and complying with Gameplane Constitution Principles I, III, and IV.

---

## Technical Context

**Language/Version**: YAML (Kubernetes CRD & JSON Schema), Markdown (`specs.md`, `README.md`), Python 3.9+ (`modules/validate.py`), Go 1.25 (E2E probe clients in `test/e2e/`)

**Primary Dependencies**: Kubernetes `GameTemplate` CRD v1alpha1, `jsonschema` validation suite, OCI container runtimes (SteamCMD, LinuxGSM, cm2network, Wine/Proton)

**Storage**: Kubernetes PersistentVolumeClaims (PVC) mounted to game-specific save paths without shadowing binary entrypoints

**Testing**: Static preflight image & schema validation (`python3 modules/validate.py`), JSON Schema conformance validation, E2E protocol join probes in `test/e2e/` (with heavy server runner budget exclusions in `test/e2e/buckets.sh` per Constitution Principle I)

**Target Platform**: Kubernetes / k3s (Linux amd64/arm64 container host environments)

**Project Type**: Game server orchestration templates & architecture specifications

**Performance Goals**: Pod provisioning & scheduling < 30 seconds; 0% data loss on restarts; validator execution < 5 seconds across all 26 modules

**Constraints**: Immutable sha256 image digest pinning; explicit `runAsUser`/`fsGroup`/`HOME` declarations; non-crashing diagnostic idle for missing registration keys

**Scale/Scope**: 26 game modules total (13 newly created + 13 standardized); 26 `specs.md` architecture specifications; 26 sample manifests; 0 `validate.py` errors

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Justification & Verification Plan |
|---|---|---|
| **I. E2E-Tested Delivery** | **PASS** | Every module specifies wire-protocol join probes in `test/e2e/`. Lightweight servers are included in CI buckets; resource-heavy servers (>8 GB RAM, e.g. ARK SA, Squad, Arma Reforger, DayZ) are authored and committed to `test/e2e/` but annotated with explicit CI runner budget exclusion comments in `test/e2e/buckets.sh` per Principle I. |
| **II. Design-First for User-Facing Change** | **N/A** | Module catalog and CRD definitions do not alter dashboard React layouts or `.pen` design sources. |
| **III. Language & Ecosystem Best Practice** | **PASS** | Strict YAML conformance with JSON schema; Python validator runs without warnings; Go E2E tests adhere to standard error wrapping and linting without inline suppressions. |
| **IV. Spec-Driven Development** | **PASS** | Complete specification lifecycle followed. Every module folder (`modules/<name>/`) will maintain an authoritative `specs.md` adhering to the required 9-section structure. |
| **V. Delegate to Workflows & Subagents** | **PASS** | Implementation fanned out across concurrent subagents by module category, starting at the smallest model tier with mandatory tier-up review. |
| **VI. CI Bears the Heavy Lifting** | **PASS** | Validation and test suites execute in CI pipelines (`.github/workflows/`); local execution limited to standard static preflight checks (`python3 modules/validate.py`). |

**Post-Design Re-check**: All principles remain satisfied following Phase 1 design and contract specifications.

---

## Project Structure

### Documentation (this feature)

```text
specs/014-top-steam-game-modules/
├── plan.md                      # This file
├── research.md                  # Phase 0 research output (complete 26-game matrix)
├── data-model.md                # Phase 1 data model (entity schemas & lifecycle states)
├── quickstart.md                # Phase 1 quickstart guide (validation workflows)
├── checklists/
│   └── requirements.md          # Specification quality checklist
└── contracts/
    ├── module-spec-contract.md  # Contract for module directory layout & specs.md
    ├── gametemplate-contract.md # Contract for GameTemplate CRD & lifecycle stop actions
    └── engine-matrix-contract.md# Contract for ports, protocols, and query mechanics
```

### Source Code (repository root)

```text
modules/
├── .schema/
│   ├── module.schema.json
│   └── gametemplate.schema.json
├── validate.py                  # Module preflight validator
├── build.sh                     # Module packaging script
│
├── [13 Existing Standardized Modules]
│   ├── cs2/
│   ├── palworld/
│   ├── rust/
│   ├── project-zomboid/
│   ├── dayz/
│   ├── garrys-mod/
│   ├── terraria/
│   ├── 7-days-to-die/
│   ├── ark-survival-ascended/
│   ├── factorio/
│   ├── dont-starve-together/
│   ├── valheim/
│   └── satisfactory/
│
└── [13 New Dedicated Server Modules]
    ├── fivem/
    ├── team-fortress-2/
    ├── farming-simulator-25/
    ├── euro-truck-simulator-2/
    ├── mount-and-blade-2-bannerlord/
    ├── tmodloader/
    ├── beammp/
    ├── left-4-dead-2/
    ├── the-isle/
    ├── ark-survival-evolved/
    ├── arma-reforger/
    ├── hell-let-loose/
    └── squad/
        ├── module.yaml
        ├── template.yaml
        ├── README.md
        ├── specs.md
        └── samples/
            └── server.yaml

test/e2e/
├── buckets.sh                   # CI test bucket assignments & heavy exclusions
└── modules_join_test.go         # E2E wire-protocol probe verification
```

---

## Complexity Tracking

| Aspect | Justification | Simpler Alternative Rejected Because |
|---|---|---|
| Single-Pod Auxiliary Services | Bundling txAdmin / embedded DB in FiveM container avoids multi-pod scheduling complexity in homelab clusters | Multi-pod Helm sub-charts require complex PVC sharing and service discovery |
| Heavy Server CI Bucket Exclusion | Committed test suites for >8GB RAM games with documented CI exclusion prevents CI timeouts | Skipping tests entirely violates Principle I; running in standard runners causes OOM kill |
