# Quickstart: Validating Module Specification Compliance

This guide walks through validating that the specification check (feature 011) is working correctly. It covers the automated compliance script and CI integration.

## Prerequisites

- Checked out on branch `011-add-missing-module-specs`
- `svcutil/specs.md` and `tunnel/specs.md` exist and are non-empty
- `hack/check-specs.sh` script is present and executable
- `Makefile` has a `check-specs` target that invokes the script
- Existing module specs are in place (e.g., `operator/specs.md`, `api/specs.md`, `web/specs.md`, etc.)

## Scenario 1: New Specs Pass Structure Checklist

**Validation**: Both new `specs.md` files (svcutil and tunnel) conform to the canonical section structure.

**How to verify**:

1. Open `svcutil/specs.md` and `tunnel/specs.md`
2. Confirm each contains sections: Purpose, Responsibilities, Non-goals/boundaries, Directory layout, External interface/Configuration, Key invariants, Dependencies, Security considerations, Testing & coverage, References
3. Compare section ordering and formatting against `contracts/specs-md-structure.md`

**Expected outcome**: Both files follow the established pattern (see research findings in `research.md`); no structural deviations.

## Scenario 2: `make check-specs` Passes

**Validation**: The automated check script exits 0 and lists all required modules.

**Command**:

```bash
make check-specs
```

**Expected output**: One aggregate success line:

```text
✓ Checked 15 modules: all have non-empty specs.md
```

Exit code 0.

See `contracts/check-specs.md` for full output contract.

## Scenario 3: Negative Test — Missing specs.md

**Validation**: The check detects a missing `specs.md` and exits non-zero.

**Commands**:

```bash
cd "$(git rev-parse --show-toplevel)"

# Temporarily move svcutil/specs.md
mv svcutil/specs.md svcutil/specs.md.bak

# Run the check (expect it to fail)
make check-specs
# Expected exit code: 1

# Restore immediately
mv svcutil/specs.md.bak svcutil/specs.md
```

**Expected outcome**: Check fails with:
```text
✗ svcutil/specs.md: missing
✗ 1 module has missing or empty specs.md
```

Exit code 1. After restoring, `make check-specs` passes again.

## Scenario 4: Negative Test — Empty and Whitespace-Only specs.md

**Validation**: The check detects both zero-byte and whitespace-only `specs.md` files and exits non-zero.

**Commands**:

```bash
cd "$(git rev-parse --show-toplevel)"

# Save the current specs.md
cp tunnel/specs.md tunnel/specs.md.bak

# Test 1: Zero-byte file
> tunnel/specs.md

# Run the check (expect it to fail)
make check-specs
# Expected exit code: 1

# Test 2: Whitespace-only file
printf ' \n\t\n' > tunnel/specs.md

# Run the check again (expect it to fail)
make check-specs
# Expected exit code: 1

# Restore
cp tunnel/specs.md.bak tunnel/specs.md
```

**Expected outcome**: 

For Test 1 (zero-byte), check fails with:
```text
✗ tunnel/specs.md: empty (0 bytes)
✗ 1 module has missing or empty specs.md
```

For Test 2 (whitespace-only), check fails with:
```text
✗ tunnel/specs.md: empty (whitespace only)
✗ 1 module has missing or empty specs.md
```

Exit code 1 in both cases. After restoring, `make check-specs` passes.

## Scenario 5: CI Verification

**Validation**: Push the branch to remote and confirm the CI lint job runs the check.

**Command**:

```bash
git push origin 011-add-missing-module-specs
gh run watch
```

**Expected output from CI lint job**:

- The `lint` job matrix runs a single "check specs compliance" step, gated `if: matrix.module == 'netguard'` so it executes exactly once per lint job trigger (not once per module) — per D5
- Output shows all 15 modules confirmed or names any missing/empty specs.md
- Job passes (exit 0)

**Expected outcome**: CI lint job runs green. The check is now enforced on every PR.

---

## Local Execution Note

Running `hack/check-specs.sh` (via `make check-specs` or directly) is permitted as a compile-check exception per maintainer ruling D6 (2026-09-01). The script performs read-only static file inspection only; it qualifies as a pre-flight check equivalent to `go build ./...` or `tsc --noEmit`, not a full test/lint suite. The full `make lint` target remains CI-only per CLAUDE.md rule 8 and constitution Principle VI.

---

## Cleanup

After validation, no branch cleanup is required. The feature branch remains active until the corresponding task/PR is approved and merged, at which point `011-add-missing-module-specs` is deleted per rule 12.
