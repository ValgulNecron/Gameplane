# Lint Gate Contract: .github/workflows/ci.yaml lint: job

The CI lint gate must satisfy these normative rules to prevent silent failures and ensure all gated modules are actually verified to be clean.

## Invocation Model

The lint job is parameterized by `matrix.module`, with each matrix entry specifying:
- **Working directory**: `${{ matrix.module }}` (governs where golangci-lint runs and interprets paths relative to)
- **Build tags** (conditional): Passed via `--build-tags=...` only for modules that have tagged test files

### Matrix Configuration (Current and Target)

The matrix.module array (line ~180 in ci.yaml) currently contains 10 entries:

```yaml
matrix:
  module: [netguard, gameaction, gameproto, operator, sentinel, audit-syslog-bridge, telemetry-receiver, mcp-server, svcutil, tunnel]
```

**Wave 2 target** (add three more):

```yaml
matrix:
  module: [netguard, gameaction, gameproto, operator, api, agent, audit-syslog-bridge, telemetry-receiver, sentinel, mcp-server, svcutil, tunnel, test/e2e]
```

**Build-Tag-Conditional Steps** (required for wave 2):

After adding api, agent, and test/e2e to the matrix:

```yaml
  - name: lint (operator - envtest build tags)
    if: matrix.module == 'operator'
    uses: golangci/golangci-lint-action@v9
    with:
      version: v2.12.2
      working-directory: ${{ matrix.module }}
      args: --build-tags=envtest

  - name: lint (api - envtest build tags)
    if: matrix.module == 'api'
    uses: golangci/golangci-lint-action@v9
    with:
      version: v2.12.2
      working-directory: ${{ matrix.module }}
      args: --build-tags=envtest

  - name: lint (test/e2e - e2e build tags)
    if: matrix.module == 'test/e2e'
    uses: golangci/golangci-lint-action@v9
    with:
      version: v2.12.2
      working-directory: ${{ matrix.module }}
      args: --build-tags=e2e

  - name: lint (other modules - no build tags)
    if: matrix.module != 'operator' && matrix.module != 'api' && matrix.module != 'test/e2e'
    uses: golangci/golangci-lint-action@v9
    with:
      version: v2.12.2
      working-directory: ${{ matrix.module }}
```

---

## Normative Rules

These rules MUST be enforced to prevent the gate from becoming silently unreliable.

| # | Rule | Consequence of Violation |
|---|------|--------------------------|
| **R-001** | Every member of `go.work` MUST appear exactly once in `matrix.module`. | A module silently never runs lint; findings in that module are never caught. CI appears green for broken code. |
| **R-002** | A module requiring build tags MUST have a separate conditional step that passes `--build-tags=...`. | Tagged files (e.g., `//go:build envtest`) are silently skipped; linter never analyzes them; hidden findings go undetected. |
| **R-003** | The job MUST use `fail-fast: false` so all matrix entries run even if one fails. | A late-matrix module (e.g., `tunnel`) is never linted if an earlier module fails; multi-module failures are masked. |
| **R-004** | The job MUST NOT use `continue-on-error: true` (on the step or the job). | Findings are reported but the job does not fail; PRs merge with lint failures. |
| **R-005** | A conditional `if:` statement MUST use exact string matching (e.g., `matrix.module == 'operator'`), not regex or prefix matching. | The wrong step runs for a similarly-named module; build tags are applied to the wrong module. |
| **R-006** | No matrix entry MUST be commented with "pending cleanup" or similar deferral language. | The comment implies the module is awaiting future work, but the matrix entry ensures it is linted today. Risk of confusion: a reviewer reads "pending" and assumes the linting is deferred, when in fact it runs on every PR. |
| **R-007** | Adding a new go.work member MUST require adding a matrix entry in the same PR. | A new module is added to the workspace but omitted from the lint matrix. The workspace `go mod tidy` runs against the module (pulling in new dependencies), but lint never touches it. |
| **R-008** | Each module's working-directory MUST exactly match the module path in `go.work` (e.g., `operator`, `api`, `test/e2e`). | If the working-directory is wrong or omitted, paths in lint output become ambiguous or uninterpretable. |
| **R-009** | The golangci-lint version MUST be pinned (currently `v2.12.2`). | Linter behavior changes between versions; an upgrade might report new findings or silence old ones. If the version is `latest`, CI becomes non-deterministic. |
| **R-010** | The linter config file MUST be `.golangci.yml` at the repository root (not module-specific variants). | Each module uses a different ruleset; findings are classified differently per module. Consistency is broken. |

---

## Exit and Failure Semantics

### What the Lint Job Proves (When Green)

When the entire lint job completes with exit code 0:

1. **Every matrix module was analyzed** under the correct working directory and build tags.
2. **Zero findings exist** in any of the gated modules under the configuration in `.golangci.yml`.
3. **All enabled linters ran**: bodyclose, errcheck, gosec, govet, ineffassign, staticcheck, unused, misspell, revive, unparam, nilerr, noctx, errorlint, contextcheck, gofmt.
4. **The three authorized exclusions were applied** (see `.golangci.yml` lines 35–52).
5. **Suppressions do not exist**: no `//nolint`, `//#nosec`, `//lint:ignore` directives were found or applied.

### What It Does NOT Prove

1. **Build-tag gating is complete**: If a module has build tags but the CI step omits `--build-tags`, those files are silently skipped. A green job does not prove they were analyzed.
2. **All code paths are clean**: Code behind unanalyzed build tags is not linted. Example: if `api` is linted without `--build-tags=envtest`, the 7 envtest files are skipped, and their findings are invisible.
3. **Web/TypeScript code is clean**: The lint job is Go-only. `web/` has separate ESLint and TypeScript checks (not covered here).
4. **Coverage gates are met**: Linting and coverage are separate gates (though both run in CI).
5. **Tests pass**: Lint verifies code quality and safety, not correctness. A found error (like an ignored return value) might not cause test failures.

---

## Worked Example: Target Configuration (Wave 2)

Below is the exact YAML after wave 2 changes, based on the current ci.yaml structure (lines 169–201).

```yaml
  lint:
    name: lint (${{ matrix.module }})
    needs: [changes]
    if: needs.changes.outputs.go == 'true'
    runs-on: ubuntu-latest
    timeout-minutes: 15
    strategy:
      fail-fast: false
      # All 13 go.work members are now gated.
      matrix:
        module: [netguard, gameaction, gameproto, operator, api, agent, audit-syslog-bridge, telemetry-receiver, sentinel, mcp-server, svcutil, tunnel, test/e2e]
    steps:
      - uses: actions/checkout@v7
      - uses: ./.github/actions/go-cache
        with:
          key-suffix: lint-${{ matrix.module }}
      - name: go mod download
        working-directory: ${{ matrix.module }}
        run: go mod download

      # Conditional steps for build-tag-gated modules.
      # Each if: checks matrix.module against the exact string.

      - name: lint (operator - envtest build tags)
        if: matrix.module == 'operator'
        uses: golangci/golangci-lint-action@v9
        with:
          version: v2.12.2
          working-directory: ${{ matrix.module }}
          args: --build-tags=envtest

      - name: lint (api - envtest build tags)
        if: matrix.module == 'api'
        uses: golangci/golangci-lint-action@v9
        with:
          version: v2.12.2
          working-directory: ${{ matrix.module }}
          args: --build-tags=envtest

      - name: lint (test/e2e - e2e build tags)
        if: matrix.module == 'test/e2e'
        uses: golangci/golangci-lint-action@v9
        with:
          version: v2.12.2
          working-directory: ${{ matrix.module }}
          args: --build-tags=e2e

      - name: lint (other modules - no build tags)
        if: matrix.module != 'operator' && matrix.module != 'api' && matrix.module != 'test/e2e'
        uses: golangci/golangci-lint-action@v9
        with:
          version: v2.12.2
          working-directory: ${{ matrix.module }}
```

**Key Changes from Current (10 modules) to Target (13 modules)**:

1. **Matrix**: Add `api`, `agent`, `test/e2e`.
2. **Conditional steps**: Replace the single `lint (operator - envtest build tags)` step and the catch-all `lint (other modules - no build tags)` step with four steps:
   - Operator-specific (envtest)
   - API-specific (envtest)
   - test/e2e-specific (e2e)
   - All others (no tags)
3. **No functional change to other jobs**: The `go` job already lists all 12 modules (not test/e2e), and the coverage job remains unchanged.

---

## How This Contract Can Be Violated Silently

These failure modes describe how the lint gate can become ineffective without obvious warning.

### Mode 1: Matrix Entry Omitted

**Symptom**: A new go.work member is added (e.g., a new `networking/` module), but the matrix remains unchanged.

**Effect**: The linter never runs on the new module. Findings in that module are invisible. CI shows green even if the module has errors.

**Detection**: Run `go work edit -json | jq '.Use[].Path' | sort` and compare against the matrix entries. A mismatch means a module is missing.

**Prevention**: Rule R-007 (adding a go.work member requires updating the matrix in the same PR).

### Mode 2: Build Tag Omitted

**Symptom**: A module (e.g., `test/e2e`) is added to the matrix, but the conditional build-tag step is never created. The catch-all step runs instead, omitting `--build-tags=e2e`.

**Effect**: The 51 e2e test files with `//go:build e2e` are silently skipped by golangci-lint. Findings in those files are never reported. A test file with a critical error (e.g., `import "fmt"` but unused) goes undetected.

**Example**: If the step is:

```yaml
- name: lint (all modules)
  uses: golangci/golangci-lint-action@v9
  with:
    version: v2.12.2
    working-directory: ${{ matrix.module }}
    # No --build-tags=... passed
```

Then for test/e2e, all 51 `//go:build e2e` files are excluded from analysis.

**Detection**: Compare `find <module> -name "*.go" | wc -l` (total files) against `find <module> -name "*.go" -exec grep -L "^//go:build" {} \; | wc -l` (files without a build tag). If the latter is zero and the job output doesn't mention the build tag, the tag was not passed.

**Prevention**: Rule R-002 (modules with build tags must have separate conditional steps).

### Mode 3: Scope Narrowed by Working Directory

**Symptom**: The working-directory is set to `${{ matrix.module }}/internal` instead of `${{ matrix.module }}`.

**Effect**: Only the `internal/` subdirectory is linted. Top-level code (e.g., `cmd/main.go`) is skipped.

**Detection**: Check the working-directory in the step against the matrix entry. For module `api`, it should be exactly `api`, not `api/internal`.

**Prevention**: Rule R-008 (working-directory must match the module path exactly).

### Mode 4: Finding Cleared by Deletion (Not Fixing)

**Symptom**: A linter reports "unused import `fmt`" in `api/internal/foo.go`. Instead of removing the import, a contributor deletes the file `api/internal/foo.go` entirely (perhaps because it was a draft or stub).

**Effect**: The finding disappears, the job goes green, but the underlying problem (dead code) is masked. The pattern that caused the unused import is never addressed; a similar mistake in a new file will repeat.

**Detection**: Code review must verify that findings are fixed (import removed, error handled, function called) rather than hidden (file deleted, code restructured to avoid the linter without fixing the issue).

**Prevention**: Rule R-004 (enforce that the job fails on any finding, no suppression). Code review discipline: ensure fixes are semantic, not evasive.

### Mode 5: Configuration Narrowing

**Symptom**: `.golangci.yml` is modified to remove a linter from the `enable:` list (e.g., commenting out `- errorlint`).

**Effect**: Findings from that linter are no longer reported. Existing bugs (e.g., misuse of `errors.Is()`) go undetected in future PRs.

**Prevention**: Rule 4 in CLAUDE.md ("Fix, don't silence") and this contract's authorization procedure for exclusions. Disabling a linter is not an acceptable response to findings; only scoped exclusions (with justification) are allowed.

---

## Matrix Completeness Verification

To prove the matrix is complete and no module is silently dropped:

```bash
# Extract matrix modules from ci.yaml
grep -A 1 "matrix:" .github/workflows/ci.yaml | grep -A 200 "module:" | \
  sed -n '/module:/,/^$/p' | grep -oE '\[.*\]' | tr -d '[]' | tr ',' '\n' | sort

# Extract go.work members
go work edit -json | jq -r '.Use[].Path' | \
  sed 's|./||' | sed 's|/go.mod||' | sort

# They should match exactly (modulo whitespace).
```

If the matrix contains entries the go.work doesn't (or vice versa), the gate is misconfigured.
