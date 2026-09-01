# CLI Contract: `hack/check-specs.sh` — Module Specification Compliance Check

**Status**: Specification Draft  
**Feature Branch**: `011-add-missing-module-specs`  
**Implementers**: Haiku agents in a Workflow (tier+1 review: Sonnet)

---

## Invocation

The check may be invoked in two ways:

1. **Via Makefile target** (primary integration point for CI):
   ```bash
   make check-specs
   ```

2. **Direct script invocation** (for troubleshooting):
   ```bash
   hack/check-specs.sh
   ```

Both forms are equivalent and produce identical output and exit codes.

---

## Inputs

### Required Files

- **`go.work`** (repository root): The Go workspace file that lists active Go module directories. The check parses this file to extract the list of modules requiring validation. If `go.work` is absent or unparsable, the check fails with a diagnostic error (see [Behavior: Absent or Malformed go.work](#behavior-absent-or-malformed-gowork)).

### Environment Variables

No environment variables are required or consumed by the check. The repository structure and `go.work` file are the sole inputs.

---

## Outputs

### Success Case (Exit Code 0)

When all checked modules have a valid, non-empty `specs.md`, the script outputs a single summary line to stdout:

```
✓ Checked 15 modules: all have non-empty specs.md
```

The count includes the 14 Go modules listed in `go.work` (agent, api, audit-syslog-bridge, capture-sidecar, gameaction, gameproto, mcp-server, netguard, operator, sentinel, svcutil, telemetry-receiver, test/e2e, tunnel) plus the `web/` directory tree.

**Exit code**: `0`

### Failure Case (Exit Code 1)

When one or more modules lack a `specs.md` file or have an empty/whitespace-only file, the script outputs:

1. One diagnostic line per offending module, naming the expected path and the specific problem:
   ```
   ✗ svcutil/specs.md: missing
   ✗ tunnel/specs.md: missing (0 bytes)
   ✗ web/specs.md: empty (whitespace only)
   ```

2. A summary line indicating total failures:
   ```
   ✗ 3 modules have missing or empty specs.md
   ```

The format for each offending module is:

```
✗ <path>: <reason>
```

Where `<path>` is the repository-relative path to the expected `specs.md` file, and `<reason>` is one of:

- `missing` — the file does not exist
- `empty (<byte_count> bytes)` — the file exists but is empty or contains only whitespace

All diagnostic output goes to stdout (not stderr) so it can be captured and reviewed in CI logs.

**Exit code**: `1`

---

## Behavior: Absent or Malformed `go.work`

If `go.work` is absent or cannot be parsed:

1. The script outputs an error message to stdout:
   ```
   ✗ Error: go.work not found or unreadable
   ```

2. The script exits with code `1`.

No attempt is made to hardcode a fallback module list; the `go.work` file is the source of truth for the active module set.

---

## Runtime Constraints

- **Duration**: Must complete in under 2 seconds (SC-002 requirement per feature spec).
- **Dependencies**: POSIX `sh` or `bash` only, plus standard GNU coreutils (`grep`, `find`, `wc`, `sed`). No Python, no Go toolchain, no container runtime, no network access.
- **Exit as fast as possible**: The check should terminate and report the first unreadable error (e.g., `go.work` parsing failure) rather than attempting to continue on partial data.

---

## Makefile Integration

### Target Definition

```makefile
.PHONY: check-specs
check-specs:
	hack/check-specs.sh

.PHONY: lint
lint: check-specs lint-go lint-web
	@echo "✓ All lint & spec checks passed"
```

**Breaking down the definition**:

- **`.PHONY: check-specs`** declares that `check-specs` is not a file target, so Make always runs it (never caches based on file timestamps).
- **Target recipe**: Invokes the shell script directly with no arguments.
- **Dependency in `lint`**: The existing `lint` target is updated to depend on `check-specs` first, ensuring specs compliance is verified alongside linting.
- **Help comment**: The target should be documented in the Makefile's help output (following the existing `##` convention):
  ```makefile
  check-specs:          ## Verify all modules have valid, non-empty specs.md
  	hack/check-specs.sh
  ```

### CI Integration

The existing `lint` job in `.github/workflows/ci.yaml` (at line 327+) runs golangci-lint directly via matrix, **not** `make lint`. Per ruling D5 (which supersedes D1's CI half), a new step must be added to the existing `lint` job to invoke `make check-specs` without creating a separate job.

**Concrete CI step to add** (to be inserted after the "verify lint gate configuration" step, around line 343):

```yaml
      - name: check specs compliance
        if: matrix.module == 'netguard'
        run: make check-specs
```

**Rationale for the conditional**: The check is repository-wide (all modules and web/ are validated on every invocation), so running it once per lint job matrix iteration would be wasteful. Gating on `matrix.module == 'netguard'` (the first in the list, so it always runs when the lint job runs at all) ensures the check executes exactly once per lint job trigger, following the existing precedent of the "verify lint gate configuration" step.

---

## Module Scope

The check validates **exactly** the following module directories:

1. **14 Go modules from `go.work`** (parsed from the `use` directives):
   - agent
   - api
   - audit-syslog-bridge
   - capture-sidecar
   - gameaction
   - gameproto
   - mcp-server
   - netguard
   - operator
   - sentinel
   - svcutil
   - telemetry-receiver
   - test/e2e
   - tunnel

2. **The `web/` directory** (as specified in FR-006, a designated subsystem tree)

### Exclusions

- **`modules/` subdirectories** (`modules/minecraft-java/`, `modules/valheim/`, etc.) are **not** checked. Per ruling D2, game module specifications are documented in `docs/module-authoring.md` and enforced in the `gameplane-module` repository's own CI, not here.
- **`charts/gameplane/`**, **`deploy/`**, **`docs/`**, and other non-module directories are not validated — they are deployment infrastructure rather than active Go modules.

---

## Edge Cases & Implementation Notes

### Empty or Whitespace-Only Files

Per spec edge case and ruling D3, a `specs.md` file containing only whitespace characters (spaces, tabs, newlines) is treated as **missing** and triggers a failure diagnostic.

**Implementation approach**: After confirming a file exists (via `test -f` or `[ -f ]`), use `grep -q '[^[:space:]]'` to verify it contains at least one non-whitespace character. If the check fails, report it as `empty (whitespace only)`.

### Module Names with Special Characters

Go workspace module paths may contain forward slashes (e.g., `test/e2e`). The check must handle these correctly when constructing file paths and generating diagnostics.

### Concurrency & Atomicity

The check is read-only and makes no modifications to the repository. It can be safely run concurrently with other CI jobs that also read the workspace structure.

---

## Success Criteria (Acceptance)

- [ ] Script exists at `hack/check-specs.sh` and is executable
- [ ] Script parses `go.work` and extracts all 14 module directories
- [ ] Script validates presence and non-emptiness of each module's `specs.md`
- [ ] Script validates `web/specs.md`
- [ ] On success, script outputs summary line with module count and exits 0
- [ ] On failure, script outputs one diagnostic per missing/empty module and exits 1
- [ ] Script completes in under 2 seconds across typical runs
- [ ] `make check-specs` invocation works
- [ ] `make lint` now depends on `check-specs` and runs it first
- [ ] No external dependencies beyond POSIX shell and coreutils
