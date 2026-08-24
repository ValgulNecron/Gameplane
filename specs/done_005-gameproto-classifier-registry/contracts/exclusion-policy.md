# Exclusion Policy: Suppressions and Authorized Exclusions (005 Scope)

**Feature**: 005-gameproto-classifier-registry  
**Phase**: 1 (Contracts)  
**Date**: 2026-08-20  
**Status**: Reference Document (SC-004 Requirement)  

This document specifies the linting and suppression policy for the refactored `gameproto/` and `sentinel/` modules. It is scoped to items that are relevant to this refactor and cites the project-wide policy established in the constitution and reflected in the `.golangci.yml` configuration.

---

## Zero-Suppression Rule (Normative)

Per CLAUDE.md rule 4 ("Fix, don't silence") and the Gameplane Constitution Principle III, the codebase MUST NOT contain any in-source suppressions or ignore directives **(FR-008)**.

### Forbidden Patterns in Go

| Pattern | Example | Why It's Forbidden |
|---------|---------|-------------------|
| `//nolint` | `result := foo() //nolint` | Silences all linters; violates "fix, don't silence" policy. |
| `//nolint:linter` | `result := foo() //nolint:errcheck` | Silences a specific linter; enables drawer-filing findings without fixing them. |
| `//#nosec` | `exec.Command(cmd) //#nosec` | Silences gosec (security linter); particularly dangerous for security findings. |
| `//lint:ignore` | `var unused int //lint:ignore U1000` | Ignores specific linter codes; violates policy. |

**Verification in CI**: The lint job MUST NOT report any suppressions in either module. The absence of in-source suppressions is an invariant of the codebase.

---

## Authorized Config-Level Exclusions

The project maintains a small, scoped set of authorized config-level exclusions in `.golangci.yml`. These are rare exceptions, maintainer-approved, and justified with detailed comments explaining why the exclusion is a false positive or unavoidable.

### Distinction: Suppression vs. Exclusion

| Aspect | In-Source Suppression | Config-Level Exclusion |
|--------|----------------------|------------------------|
| **Location** | Inline in source code (`//nolint`) | In `.golangci.yml` configuration file |
| **Scope** | Single line or statement | Path pattern + linter + optional text filter |
| **Governance** | None; violates policy | Rare, maintainer-approved, justified in code comment |
| **Policy** | **Forbidden. Always.** | **Permitted. Rarely. Scoped. Documented.** |

---

## Authorized Exclusions for gameproto/ and sentinel/

The following is the complete inventory of authorized config-level exclusions that apply to either `gameproto/` or `sentinel/` in the refactored code.

### Exclusion #1: Test Files (Applies to Both Modules)

**Config Entry** (in `.golangci.yml`, line 37–38):
```yaml
      - path: _test\.go
        linters: [errcheck, gosec, unparam]
```

| Aspect | Value |
|--------|-------|
| **Path Pattern** | `_test\.go` (all files ending in `_test.go`, including `gameproto/*_test.go` and `sentinel/*_test.go`) |
| **Linters Affected** | errcheck, gosec, unparam |
| **Text Matcher** | None (all findings by these linters in matched files are excluded) |
| **Justification** | Tests often ignore errors deliberately in setup/teardown code (e.g., closing a server that is already closed). Security checks in test code are less critical. Unused parameters are common in test helpers. |
| **Why Authorized** | Narrow scope (only test files). Linters affected are most likely to produce false positives in tests. The exclusion does not disable any linter globally. |
| **Scope for This Refactor** | Both `gameproto/*_test.go` and `sentinel/*_test.go` are covered. The refactored Classifier tests can use this exclusion if test setup needs to ignore errors. |

### Exclusion #2: Minecraft VarInt Two's Complement Reinterpretation (gameproto only)

**Config Entry** (in `.golangci.yml`, line 47–52):
```yaml
      # Minecraft VarInt encoding/decoding requires lossless reinterpretation between uint32
      # and int32 (two's complement). All 32 bits are preserved; length and range overflow
      # are still checked explicitly in the code. G115 remains active everywhere else.
      - path: (gameproto/)?minecraft\.go$
        linters: [gosec]
        text: "G115"
```

| Aspect | Value |
|--------|-------|
| **Path Pattern** | `(gameproto/)?minecraft\.go$` (the file `gameproto/minecraft.go` or any path ending in `minecraft.go`) |
| **Linters Affected** | gosec |
| **Text Matcher** | `"G115"` (gosec's "possible unintended type assertion" / uint32↔int32 cast warning) |
| **Justification** | Minecraft's wire protocol uses variable-length integers (VarInts) that are signed in the spec but unsigned in Go. The parser intentionally reinterprets bytes between uint32 and int32 to decode the spec-correct value. All 32 bits are preserved (no truncation), and overflow checks are explicit in the code. This is safe and necessary. |
| **Why Authorized** | Narrow scope (only `minecraft.go`). Text matcher narrows to the specific gosec code (G115); other gosec findings in the file are still reported. The justification documents the control flow that makes the reinterpretation safe. |
| **Scope for This Refactor** | The MinecraftClassifier's Classify() method inherits this exclusion from the existing minecraft.go implementation. No new exclusions are added for the refactored code. This exclusion remains exactly as-is. |

---

## Inventory of Authorized Exclusions (Project-Wide Reference)

For context, here are the other authorized exclusions in the project. These do NOT apply to gameproto/ or sentinel/:

| # | Path | Linter | Text | Purpose | Modules Affected |
|---|------|--------|------|---------|------------------|
| 1 | `_test\.go` | errcheck, gosec, unparam | (none) | Test file error/security handling | all (including gameproto, sentinel) |
| 2 | `(^|/)internal/controller/` | revive | "exported:" | Operator reconciler builder patterns | operator only |
| 3 | `(gameproto/)?minecraft\.go$` | gosec | "G115" | Minecraft VarInt two's complement | gameproto only |
| 4 | `(^|/)internal/mods/mods\.go$` | gosec | "G302" | Mod extraction file permissions | agent only |
| 5 | `(^|/)internal/ws/dialer\.go$` | gosec | "G704" | Proxy upstream URL construction | api only |
| 6 | `(^|/)internal/auth/sessions\.go$` | gosec | "G124" | CSRF cookie HttpOnly by design | api only |
| 7 | `(^|/)env\.go$` | gosec | "G204" | Kubectl subprocess invocation | test/e2e only |
| 8 | `(^|/)internal/satisfactory/app\.go$` | gosec | "G402" | Satisfactory HTTPS with self-signed cert | test/e2e only |

---

## New Suppressions for This Refactor

**Inventory**: Zero new suppressions are added by this refactor **(SC-004)**.

**Why**: The refactored gameproto/ and sentinel/ code:
- Replaces per-protocol facade functions with a Classifier interface implementation.
- Consolidates sentinel's hardcoded dispatch into a registry-based loop.
- Maintains byte-for-byte equivalence with the current code.
- Preserves all existing test coverage.

No new linting issues are introduced. All Classifier implementations (Minecraft, Terraria, and any test stubs) are written to pass lint without suppressions. This refactor MUST NOT introduce any new in-source suppression directives, and the zero-suppression property of the codebase MUST be preserved **(FR-008, SC-004)**.

**Verification**: CI's lint job will report zero findings in either module. A search for any new suppression patterns in the refactored code (if changes are pushed) will return empty **(SC-004)**.

---

## Verification Recipe for Reviewers

To verify this contract is met during code review:

### 1. Check for In-Source Suppressions

```bash
# Look for any suppressions in gameproto
grep -r '//nolint\|//#nosec\|//lint:ignore' gameproto/ --include='*.go' | grep -v _test.go

# Look for any suppressions in sentinel
grep -r '//nolint\|//#nosec\|//lint:ignore' sentinel/ --include='*.go' | grep -v _test.go

# Expected output: (empty — no matches)
```

If any suppressions are found (except in test files, which are not subject to the zero-suppression rule), the review should be blocked until they are removed and the underlying issue is fixed.

### 2. Verify No New .golangci.yml Exclusions

```bash
# Count exclusions before refactor
git show main:.golangci.yml | grep -c "^      - path:"

# Count exclusions after refactor (on the branch)
grep -c "^      - path:" .golangci.yml

# Expected: same count (no new exclusions added)
```

### 3. Lint Output Check

The lint job in CI must report zero findings for both modules:

```bash
make lint-go  # or the CI equivalent
# Expected output for gameproto/ and sentinel/:
# 0 linter findings reported
```

---

## Coverage Gates (Related Constraint)

Coverage thresholds are also a non-negotiable quality gate, separate from linting. They are mentioned here for completeness.

| Module | Threshold | Enforcer | Status for This Refactor |
|--------|-----------|----------|-------------------------|
| gameproto | 90% line coverage | `.testcoverage.yml` | Must be maintained (may increase due to new Classifier interface tests) |
| sentinel | 70% line coverage | `.testcoverage.yml` | Must be maintained (may shift coverage from handler functions to dispatcher tests) |

The refactored code must maintain or exceed these thresholds. CI will verify.

---

## Decision Log

**Decision (Research Phase, Decision 8)**: "Code currently covered in sentinel/main.go (handleMinecraft, handleTerraria, ~90 lines) will shift coverage from sentinel to gameproto when those handlers are replaced with a generic registry-based dispatcher."

**Decision (Implementation Constraint)**: "This refactor adds NO new in-source suppressions."

**Rationale**: The registry-based refactoring is purely structural. It moves logic from hardcoded dispatch to a Classifier abstraction but preserves all behavior. No new linting issues are introduced; all code is written to pass lint cleanly.

---

## Governance

This exclusion policy is subordinate to the project-wide `.golangci.yml` configuration and the Constitution Principle III ("Language & Ecosystem Best Practice"). Any new suppressions or exclusions for this refactor MUST be justified in the feature branch's plan and approved by a maintainer.

If a linting issue arises during implementation:
1. **Do not suppress it.** Fix the underlying code.
2. **If a genuine false positive is discovered**, propose a new exclusion following the project's authorization procedure (in `.golangci.yml` with a detailed justification comment, reviewed and approved by a maintainer).

This refactor is expected to maintain zero new suppressions.

---

## Summary

- **Zero in-source suppressions** are forbidden and will not be added.
- **Authorized exclusions** are scoped and documented in `.golangci.yml`:
  - Test file exclusions (errcheck, gosec, unparam) apply to both modules.
  - Minecraft G115 exclusion applies to gameproto/minecraft.go and is inherited by the refactored MinecraftClassifier.
- **No new exclusions** are added by this refactor.
- **Coverage gates** (90% gameproto, 70% sentinel) must be maintained.
- **Verification** happens via CI's lint and coverage jobs.
