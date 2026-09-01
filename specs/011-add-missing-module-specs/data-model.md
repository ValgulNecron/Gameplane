# Data Model: Module Specification Compliance

## Module Specification (`specs.md`)

**Definition**: A Markdown document that serves as the authoritative reference for a single Go module or subsystem, residing at the root of that module's directory (e.g., `agent/specs.md`, `api/specs.md`, `web/specs.md`).

### Location Rule

- **Required path**: `<module-root>/specs.md` (e.g., `netguard/specs.md`, `svcutil/specs.md`, `web/specs.md`)
- **Scope**: One per Go module listed in `go.work` and the `web/` tree
- **Invalid locations**: Sub-packages (e.g., `agent/internal/console/specs.md`) are NOT standalone module specs; they are covered by their parent module's specs.md (exception: `test/e2e/internal/specs.md` is explicit, see Exceptions section E2)

### Required Sections (Canonical Order)

Every module's `specs.md` MUST include the following sections in this order:

1. **Title** — format: `# <module-name> — Specification` with **Status:**, **Module / command:**, and **Dependencies:** metadata on the next lines
2. **Purpose** — concise statement of the module's role and why it exists
3. **Responsibilities** — detailed list of what the module is responsible for (e.g., "reconcile GameServer CRDs", "render command templates", "supervise relay processes")
4. **Non-goals / Boundaries** — explicit "does NOT" statements about what is intentionally outside scope
5. **Directory & Package Layout** — file/function organization, exported vs. internal APIs, export lists
6. **External Interface / Configuration** — contracts with callers, environment variables, configuration parameters, wire protocols, API surfaces
7. **Key Invariants** — properties that MUST hold at all times (e.g., "credentials are always read-only", "every backoff retry increments by 2^n")
8. **Dependencies** — internal (within go.work or web/) and external (third-party + stdlib)
9. **[Optional] Data & Persistence** — state storage, databases, caches (optional for stateless modules; required if module manages state)
10. **Security Considerations** — threat model, authentication/authorization, SSRF/egress constraints, mTLS, sensitive data handling, DoS mitigations
11. **Testing & Coverage** — test structure (unit/integration/e2e breakdown), test doubles, key test cases, coverage gate value and threshold, uncovered paths, CI/Makefile commands
12. **References** — importers (file paths, brief description), cross-references to related specs.md files, external docs (docs/)

### Validity Rule

A `specs.md` file is **valid** (non-missing) if and only if:

- The file exists at `<module-root>/specs.md`
- **AND** the file is **non-empty after whitespace trimming** — a file containing only whitespace (spaces, tabs, newlines) counts as **missing** per maintainer ruling D3

A file that is 0 bytes or contains only whitespace is treated as **missing** for compliance purposes.

### Update-in-Same-Change Rule

Per Constitution Principle IV: whenever a module's behavior or configuration changes, its `specs.md` MUST be updated in the same commit/PR. A behavior change without a matching specs.md update is incomplete.

---

## Workspace Module Registry

**Definition**: The set of active modules in the repository that MUST have valid `specs.md` files per Constitution Principle IV.

### Registry Membership

The registry is derived from:

1. **Go modules in `go.work`** (14 modules, lines 4–17 of go.work):
   - `agent/`
   - `api/`
   - `audit-syslog-bridge/`
   - `capture-sidecar/`
   - `gameaction/`
   - `gameproto/`
   - `mcp-server/`
   - `netguard/`
   - `operator/`
   - `sentinel/`
   - `svcutil/`
   - `telemetry-receiver/`
   - `test/e2e/`
   - `tunnel/`

2. **Web subsystem**:
   - `web/` (React + TypeScript dashboard)

**Total registry size**: 15 modules

### Exclusions (with Rationale per Maintainer Ruling D2)

- **`modules/<game>/*`** — game module directories (e.g., `modules/minecraft-java/`) are **NOT** checked by the automated compliance script (hack/check-specs.sh). Game module specs.md requirements are codified as a written guideline in `docs/module-authoring.md` and enforced in the separate `gameplane-module` repository's own CI. They are out of scope for this feature.

- **Sub-packages** — packages nested inside a workspace module (e.g., `agent/internal/console/`, `operator/internal/controller/`) are covered by their parent module's `specs.md` and do NOT need standalone specs.md files. Exception: `test/e2e/internal/specs.md` is explicit and documents the game-bot harness contract (see Exceptions section E2).

- **Infrastructure directories** — `charts/gameplane/`, `deploy/kind/`, and other infrastructure-as-code directories are deployment concerns, not workspace modules, and are not subject to this rule.

---

## Compliance Check Result

**Definition**: The output of the automated specification completeness verification script (`hack/check-specs.sh`, invoked by `make check-specs`).

### Per-Module Status

The check assigns one of three internal statuses to each module in the Workspace Module Registry:

| Status | Meaning | Implication |
|--------|---------|-------------|
| **present** | Module directory exists; `specs.md` file exists and is non-empty (after whitespace trim) | Module is compliant |
| **missing** | Module directory exists; `specs.md` file does not exist | Module is non-compliant; check exits 1 |
| **empty** | Module directory exists; `specs.md` file exists but is empty or contains only whitespace | Module is non-compliant per D3; check exits 1 |

**Note on output emission**: The `present` status is **not** emitted as per-module output. Compliant modules are counted only in the aggregate summary line (`✓ Checked N modules: all have non-empty specs.md`), while `missing` and `empty` statuses each produce one diagnostic line (`✗ <path>: <reason>`) with reason strings per `contracts/check-specs.md` (reason is `missing` for absent files, or `empty (0 bytes)` / `empty (whitespace only)` for empty/whitespace-only files).

### Exit Code Semantics

| Exit Code | Condition | Interpretation |
|-----------|-----------|-----------------|
| **0** | All modules in the registry report `present` | Compliance verified; all modules have valid specs.md files |
| **1** | At least one module reports missing or empty, OR go.work is absent/unreadable/yields no use paths | Non-compliance detected; at least one module lacks a valid specs.md |

### Output Format

The check script's output format, including success and failure cases, diagnostic line structure, and exact text strings, is authoritatively specified in **`contracts/check-specs.md`** (section "Outputs"). The data model does not duplicate that specification here.

### Performance Constraint

Per maintainer ruling D4, the check MUST complete in under 2 seconds, using only POSIX sh/bash + coreutils, without requiring network access or container runtimes.

---

## Validation Rules

### Rule V1: File Existence

For each module in the registry:
- Check if `<module-root>/specs.md` exists on the filesystem.
- If it does not exist → Report `missing`.

### Rule V2: Non-Empty Content (D3)

For each module where `specs.md` exists:
- Read the file and strip all leading/trailing whitespace.
- If the stripped content is empty (0 bytes after whitespace removal) → Report `empty`.

### Rule V3: Update in Same Change

**Enforcement scope**: Code review and maintainer judgment (not automated by the check script).
- When a module's behavior, configuration, or contract changes → Verify that the change includes an update to that module's `specs.md`.
- A behavior change without a specs.md update is marked incomplete in review and must be fixed before merge.

---

## State / Decision Table: Module Compliance Status

This table shows the decision logic for determining a module's compliance status:

| File Exists? | File Non-Empty? | Status | Exit Code |
|---|---|---|---|
| No | — | **missing** | 1 |
| Yes | No | **empty** | 1 |
| Yes | Yes | **present** | 0 (if all modules OK) |

**Aggregate Decision**: 
- If ALL modules are `present` → Exit 0 (compliance verified).
- If ANY module is `missing` or `empty` → Exit 1 (non-compliance detected).

---

## Exceptions

This section records all blanket-rule exceptions this feature carries, per Constitution Principle IV and CLAUDE.md rule 15.

### E1: `modules/<game>/` Excluded from Automated Check

**Rule**: Game module directories under `modules/` are **NOT** subject to the automated compliance check run by `hack/check-specs.sh`.

**Rationale**: Maintainer ruling D2 specifies that the check covers exactly the Go modules in `go.work` plus `web/`, explicitly excluding `modules/*`. Game module specs.md requirements are codified as a written guideline in `docs/module-authoring.md` and enforced in the separate `gameplane-module` repository's own CI, which is outside the scope of this feature (spec 011).

**Implication**: A game module (e.g., `modules/minecraft-java/`) that lacks `specs.md` does NOT cause the lint job (which runs `make check-specs`) to fail. Enforcement happens upstream in the gameplane-module repo's CI.

### E2: `test/e2e/internal/` Has Multiple Module-Level Specs

**Rule**: The `test/e2e/internal/` directory contains multiple `specs.md` files documenting different harness components (e.g., `test/e2e/internal/specs.md` for the probe harness, `test/e2e/internal/<game>/spec.md` for per-game implementations, `test/e2e/internal/protocol/<family>/spec.md` for wire protocols). These are NOT subject to Rule V1–V4.

**Rationale**: `test/e2e/internal/` is internal infrastructure for the `test/e2e/` module, not a workspace module itself. Its internal specs.md files document the test harness contract and are exempt from the single-module-one-specs.md convention. The parent module's specs.md at `test/e2e/specs.md` is the single authoritative reference for the `test/e2e/` module as a whole.

**Implication**: The compliance check does NOT validate `test/e2e/internal/specs.md` structure or presence. Only `test/e2e/specs.md` (at the module root) is subject to the automated check.

### E3: Principle I E2E-Tested Delivery — Exception for Documentation & Scripts

**Rule**: Per Constitution Principle I, every feature MUST be verifiable end-to-end before completion. This feature (spec 011) adds documentation (`svcutil/specs.md`, `tunnel/specs.md`) and a repo-hygiene script (`hack/check-specs.sh`) with no user-facing or operator-facing runtime path.

**Rationale**: An E2E test in `test/e2e/` cannot exercise documentation content (there is no user/operator interaction with the `.md` files themselves). The feature's E2E verification is achieved through:
- Negative test: simulating a missing or empty `specs.md` in a test environment, verifying the check exits with code 1 and identifies the missing file.
- CI integration: the lint job runs `make check-specs` (which invokes the script) on every push, ensuring ongoing compliance.

**Implication**: No new E2E test is added to `test/e2e/`. The check script itself is verified by CI and by manual negative testing during implementation.

### E4: `svcutil/` and `tunnel/` Are Remediation Targets

**Rule**: The automated check initially fails on `svcutil/` (no `specs.md` currently exists) and on any other module lacking a valid `specs.md`.

**Rationale**: As of 2026-09-01, `svcutil/specs.md` does not exist, and `tunnel/specs.md` was added recently. These modules are the primary targets of spec 011. Once the implementation wave completes and `svcutil/specs.md` and `tunnel/specs.md` are authored and merged, the check will pass (exit 0) for all 15 modules in the registry.

**Implication**: Until both `svcutil/specs.md` and `tunnel/specs.md` are merged into `master`, the lint job's `make check-specs` step will report failures. This is the expected intermediate state during feature development.

### E5: Sub-Packages Inherit Parent Module's Specs

**Rule**: Internal sub-packages (e.g., `agent/internal/console/`, `operator/internal/controller/`) are NOT required to have their own `specs.md` files. They are documented as part of their parent module's `specs.md` under the "Directory & Package Layout" section.

**Rationale**: Gameplane modules are atomic units; internal packages are implementation details. A single `specs.md` per module (not per package) keeps documentation maintainable and prevents specs proliferation.

**Implication**: The compliance check does NOT enumerate or validate sub-packages. It checks only the module root (e.g., `agent/specs.md`, not `agent/internal/console/specs.md`).

---

## Data Integrity & Staleness Prevention

### Specs.md as Single Source of Truth

Each module's `specs.md` is the authoritative reference for that module's behavior, configuration, and contracts. Code is the implementation; specs.md is the intent.

### Update Obligation

Per Constitution Principle IV: whenever a module's behavior changes, its `specs.md` MUST be updated in the same commit. A behavior change without a specs.md update is treated as incomplete work and will not be accepted in review.

### Staleness Detection

The compliance check does NOT validate whether `specs.md` content matches current code (that is a human code-review responsibility). It validates only:
- File existence (Rule V1)
- Non-empty content (Rule V2)

---

## References

- **Constitution Principle IV**: `/.specify/memory/constitution.md` (Spec-Driven Development rule)
- **CLAUDE.md rule 15**: `docs` section — "A feature's intent is the whole `specs/<feature>/` folder, not just spec/plan/tasks"
- **CLAUDE.md rule 16**: `docs` section — "A finished feature's spec folder is renamed `done_<NNN>-<slug>`"
- **Maintainer Rulings**: Feature 011 data model (2026-09-01)
  - D1: Script location (hack/check-specs.sh) and integration (make check-specs, lint depends on it)
  - D2: Coverage scope (go.work + web/, excluding modules/*)
  - D3: Empty/whitespace-only counts as missing
  - D4: Performance and tooling constraints
- **Research: specs-structure** — canonical section list, formatting conventions, examples
- **Research: repo-audit** — go.work module count, existing compliance gaps, CI integration points
