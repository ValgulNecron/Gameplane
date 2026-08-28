# Feature Specification: Complete Module Specifications & Compliance Verification

**Feature Branch**: `011-add-missing-module-specs`

**Created**: 2026-08-28

**Status**: Draft

**Input**: User description: "Some folder are missing specs.md"

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Author Missing Module Specifications for `svcutil` and `tunnel` (Priority: P1)

A developer, operator, or AI agent working on Gameplane needs authoritative, up-to-date specifications for every module in the repository to understand responsibilities, API contracts, invariants, configuration parameters, and boundaries. When inspecting `svcutil` or `tunnel`, they consult standard `specs.md` files located directly inside each module folder (`svcutil/specs.md` and `tunnel/specs.md`), structured identically to existing module specifications across the repository (e.g., `sentinel/specs.md`, `capture-sidecar/specs.md`, `audit-syslog-bridge/specs.md`).

**Why this priority**: Gameplane Constitution Principle IV explicitly mandates that every module folder (each Go module in `go.work` and the `web/` tree) maintain an authoritative `specs.md`. Currently, `svcutil` and `tunnel` lack `specs.md` files, creating documentation gaps and violating project constitution rules.

**Independent Test**: Can be tested independently by reviewing `svcutil/specs.md` and `tunnel/specs.md` to ensure they contain all required sections (Purpose, Responsibilities, Non-goals/boundaries, Directory layout, External interface/Configuration, Key Invariants, Dependencies, Security considerations, Testing & coverage, References) and accurately reflect the current implementations.

**Acceptance Scenarios**:

1. **Given** the `svcutil` utility module, **When** a user or agent opens `svcutil/specs.md`, **Then** the document comprehensively details its purpose, public functions (`Or`, `OrInt`, `ParseLogLevel`, `RunHTTP`), stdlib-only constraint, non-crashing fallback semantics, graceful shutdown mechanics, and test coverage requirements.
2. **Given** the `tunnel` relay supervisor module, **When** a user or agent opens `tunnel/specs.md`, **Then** the document comprehensively details its architecture, supported tunnel providers (`frp`, `tailscale`, `playit`), environment variables, read-only secret credential mounting, config rendering, child process supervision, signal forwarding, and operator contracts.
3. **Given** both new specification files, **When** evaluated against existing module specifications (`sentinel/specs.md`, `capture-sidecar/specs.md`), **Then** their section structures, formatting, and technical depth are consistent with repository standards.

---

### User Story 2 - Comprehensive Repository Audit for Module Specification Coverage (Priority: P1)

A maintainer wants to ensure that all directories across the repository (including Go workspace modules, frontend trees, Helm charts, and module directories) are audited for compliance with Gameplane Constitution Principle IV, ensuring no other active modules or components are missing required specification documents.

**Why this priority**: A complete audit provides assurance that all existing and future module categories have documented specifications and clear boundary guidelines.

**Independent Test**: Can be tested independently by running an inventory across all root and sub-tree directories in the repository, checking each against Constitution Principle IV rules, and verifying that every identified module folder has a valid `specs.md`.

**Acceptance Scenarios**:

1. **Given** all 14 Go modules in `go.work` plus `web/`, **When** the audit is performed, **Then** 100% of these modules are confirmed to have a dedicated `specs.md` file.
2. **Given** the `modules/` directory for game modules, **When** the audit evaluates game module requirements, **Then** the specification guidelines for game module directories (`modules/<name>/specs.md`) are clearly codified.
3. **Given** infrastructure directories like `charts/gameplane`, **When** the audit evaluates them, **Then** their documentation status and boundaries are cataloged.

---

### User Story 3 - Automated Specification Completeness Verification (Priority: P2)

A contributor, reviewer, or automated CI workflow executes a validation check to verify that all modules defined in `go.work` and the `web/` directory maintain a valid, non-empty `specs.md`. If any module is missing `specs.md`, the check fails fast with a clear diagnostic message pointing to the missing file.

**Why this priority**: Automated enforcement guarantees ongoing compliance with Constitution Principle IV, preventing future PRs or refactors from introducing new modules without accompanying specification files.

**Independent Test**: Can be tested independently by running the specification check command against the codebase (verifying it exits with code 0), and temporarily simulating a missing `specs.md` in a module (verifying the check exits with code 1 and identifies the missing file).

**Acceptance Scenarios**:

1. **Given** all workspace modules contain valid `specs.md` files, **When** the specification validation check is run, **Then** it outputs a success summary with 0 errors and exits with code 0.
2. **Given** a module directory in `go.work` that lacks a `specs.md` file or has an empty file, **When** the specification validation check is run, **Then** it outputs an error identifying the offending module and exits with a non-zero code.
3. **Given** standard developer tooling, **When** running project verification (`make lint` or dedicated target), **Then** the specification check runs swiftly without external network dependencies.

---

### Edge Cases

- **Empty or Whitespace-Only `specs.md`**: A module directory containing a 0-byte or whitespace-only `specs.md` must be treated as missing a specification.
- **Nested / Internal Sub-Packages**: Sub-packages within a Go module (e.g., `agent/internal/caps`) are covered by their parent module's `specs.md` unless explicitly designed as an independent module in `go.work`.
- **Placeholder Directories**: Empty placeholder directories like `modules/` (with no game subdirectories) do not require a root `specs.md` until specific game directories (`modules/<game>/`) are created.
- **Spec Drift during Code Changes**: When module functionality or configuration changes in future work, the module's `specs.md` must be updated in the same change per Constitution Principle IV.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide an authoritative, comprehensive specification file at `svcutil/specs.md`.
- **FR-002**: `svcutil/specs.md` MUST document the module's purpose, exported utility functions (`Or`, `OrInt`, `ParseLogLevel`, `RunHTTP`), stdlib-only architecture, fallback behavior, graceful shutdown mechanics, test coverage gate (70%), and key invariants.
- **FR-003**: The system MUST provide an authoritative, comprehensive specification file at `tunnel/specs.md`.
- **FR-004**: `tunnel/specs.md` MUST document the module's purpose, supervisor architecture, supported tunnel types (`frp`, `tailscale`, `playit`), environment variable schemas, credentials mounting at `/etc/gameplane/tunnel-auth`, rendered config paths, subprocess supervision and signal forwarding, test coverage gate (70%), and key invariants.
- **FR-005**: All created `specs.md` files MUST follow the standard structure established by existing module specifications (Purpose, Responsibilities, Non-goals/boundaries, Directory layout, Configuration/Interface, Key Invariants, Dependencies, Security considerations, Testing & coverage, References).
- **FR-006**: The system MUST provide an automated verification script or command (e.g., in `hack/` or `Makefile`) to validate that every Go module in `go.work` and `web/` contains a valid, non-empty `specs.md`.
- **FR-007**: The automated verification tool MUST report the list of checked modules and clearly flag any missing or empty `specs.md` files with a non-zero exit code.

### Key Entities *(include if feature involves data)*

- **Module Specification (`specs.md`)**: A Markdown document residing at the root of a module folder defining its purpose, architecture, configuration parameters, runtime contracts, dependencies, and invariants.
- **Workspace Module Registry**: The set of active workspace modules derived from `go.work` and designated subsystem trees (`web/`, `modules/*`).

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of active modules in `go.work` (all 14 modules) and `web/` have a complete, compliant `specs.md` file.
- **SC-002**: Automated specification compliance check executes and validates all modules in under 2 seconds.
- **SC-003**: Automated check reliably detects and reports 100% of missing or empty `specs.md` files in testing.
- **SC-004**: 0 unresolved discrepancies between module specifications and current codebase implementations.

---

## Assumptions

- `svcutil` and `tunnel` are existing, fully functional Go modules whose implementations in `svcutil/` and `tunnel/` are the authoritative source for their specification content.
- Sub-packages within a Go module (e.g. `agent/internal/*` or `operator/internal/*`) are represented by their top-level module `specs.md`, except where explicit sub-specs exist (e.g. `test/e2e/internal/specs.md`).
- Helm charts under `charts/` and deployment manifests under `deploy/` are deployment infrastructure rather than standalone Go modules, but their relationships and contracts are referenced where relevant.
- The automated verification tool runs locally and in CI without requiring external network access or container runtimes.

