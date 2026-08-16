# Verifier CLI Contract: test/e2e/joincoverage.sh

The verifier is a POSIX shell script that validates the consistency and completeness of game protocol E2E coverage across three sources of truth: the set of game modules under `modules/`, the test registry in `test/e2e/buckets.sh`, and the tracked coverage artifact `docs/game-coverage.md`. It is the machine-enforced gate that prevents coverage status from drifting when modules are added, renamed, or removed.

## Subcommands and Exit Codes

The script supports the following subcommands:

```
joincoverage.sh [SUBCOMMAND]
```

### `verify` (default, no argument)

```bash
test/e2e/joincoverage.sh verify
# or simply:
test/e2e/joincoverage.sh
```

**Purpose**: Run all consistency checks and exit with a status indicating success or the type of failure.

**Exit codes**:

| Code | Meaning | Failure is Hard (blocks CI)? | Example Condition |
|---|---|---|---|
| `0` | All checks passed; coverage tracking is in sync | N/A (success) | Every module in `modules/` is listed in docs/game-coverage.md exactly once, all tests exist, all depths are consistent. |
| `1` | One or more hard checks failed | YES | Uninitialized `modules/` submodule, a module listed twice, missing blocker, wrong status/depth combination. |
| `2` | Warning only; checks passed but coverage is stale | NO (warning, not failure) | A deferred test's Last Verified date is > 90 days old. CI should note but not block. |

Exit code 2 is a warning, not a failure; it reports stale coverage but does not prevent merge. Exit code 1 is a hard failure; the CI job must block on it.

### Subcommand: `diagnose [module-name]` (future, optional)

For maintainer debugging (not required for Phase 1). If implemented later:

```bash
test/e2e/joincoverage.sh diagnose minecraft-java
```

Would print detailed info about a single module's state (status, test name, bucket, why blocked, etc.). Not required by the contract.

## Verification Checks (Hard Failures, Exit Code 1)

**Summary**: 16 hard checks (numbered 1–16, exit 1 on violation) plus 1 warning (W1, exit 0 or 2). Every hard check has exactly one fixture. The script MUST implement all of them.

### Check 1: Modules Submodule Must Be Initialized

**Rule**: If `modules/` is an empty directory or does not exist, the verifier MUST exit 1 with a distinct error message.

**Failure message** (exact or very close):
```
ERROR: modules/ is not initialized (run: git submodule update --init)
```

**Why**: An empty `modules/` directory silently reports "0 modules, all covered" and masks the true state. This is worse than no test.

**Verification**: Check if `modules/*/` has at least one real directory (not `.gitmodules`). If not, fail immediately.

---

### Check 2: Every Module in `modules/` Must Be Listed in docs/game-coverage.md

**Rule**: For every directory `modules/<name>` (where `<name>` is not `.gitmodules` or `.git`), there MUST be exactly one row in the coverage table with Module = `` `<name>` ``.

**Failure message** (exact or very close):
```
ERROR: module 'factorio' found in modules/ but not listed in docs/game-coverage.md
```

**Verification**: 
1. Read the list of real module directories from `modules/`.
2. Parse the coverage table from `docs/game-coverage.md` (first table only).
3. For each module directory, verify a row exists with Module = `` `<name>` `` (exact match, case-sensitive, backticks required).

---

### Check 3: No Duplicate Modules in Coverage Table

**Rule**: Every module MUST be listed exactly once. If a module appears twice, fail.

**Failure message** (exact or very close):
```
ERROR: module 'minecraft-java' listed twice in docs/game-coverage.md (row 5 and row 12)
```

**Verification**: Parse the table; count rows per module. If any count > 1, fail.

---

### Check 4: No Stray Modules in Coverage Table

**Rule**: Every row in the coverage table MUST correspond to a directory in `modules/`. Rows for non-existent modules are typos or deletions that were not reflected in the table.

**Failure message** (exact or very close):
```
ERROR: docs/game-coverage.md lists module 'minecraft-oldversion' which does not exist in modules/ (deleted or renamed?)
```

**Verification**: Parse the table. For each Module, check if `modules/<module>/` exists. If not, fail.

---

### Check 5: Status and Depth Must Be Consistent

**Rule**: Certain status/depth combinations are forbidden (spec FR-006 and related):

- If Status is `covered-in-ci`, Depth MUST be `JOINED` exactly (not `PARTIAL`, not `QUERY`, not `—`).
- If Status is `covered-deferred`, Depth MUST be `JOINED` exactly (not `PARTIAL`, not `QUERY`, not `—`).
- If Status is `blocked-doc` or `out-of-scope-by-design`, Depth MAY be `QUERY`, `PARTIAL`, or `—`. A blocked row may have a real test that does not constitute join coverage (e.g., a `_Query` reachability test).

**Failure message** (exact or very close):
```
ERROR: module 'garrys-mod' has Status 'covered-in-ci' but Depth 'QUERY' (only JOINED depth counts as CI coverage, per spec FR-006)
```

**Verification**: For each row, check the Status/Depth pair against the rules above. For covered-* rows, Depth must be JOINED.

---

### Check 6: Covered Modules Must Have a Test

**Rule**: If Status is `covered-in-ci` or `covered-deferred`, the Test column MUST NOT be `—`.

**Failure message** (exact or very close):
```
ERROR: module 'minecraft-java' has Status 'covered-in-ci' but Test is empty (no test listed)
```

**Verification**: For each `covered-*` row, ensure Test is not `—`.

---

### Check 7: Test Name Must Be Valid

**Rule**: If Test is not `—`, the listed Go function name MUST be found in `test/e2e/*_test.go` via grep. This applies to covered rows and blocked rows alike. Typos in test names are caught here.

**Failure message** (exact or very close):
```
ERROR: module 'minecraft-java' lists test 'TestGameServer_MinecraftJavaBot_Join' which was not found in test/e2e/*_test.go (typo or renamed?)
```

**Verification**:
```bash
grep -l "^func $testname" test/e2e/*_bot_e2e_test.go >/dev/null 2>&1 || fail
```

---

### Check 8: Covered Modules Must Have a Bucket

**Rule**: If Status is `covered-in-ci` or `covered-deferred`, the Bucket column MUST be one of the valid buckets recognized in `test/e2e/buckets.sh`: `operator`, `api-auth`, `api-roles`, `api-rbac`, `api-agent`, `api-mods`, `ratelimit`, `bot-fast`, `bot-heavy`, `multicluster`, `upgrade`, or `unbucketed()` (with comment explaining the exclusion). A bucket is NOT required if the test is explicitly marked as `unbucketed()` in the coverage table with a comment.

**Failure message** (exact or very close):
```
ERROR: module 'minecraft-java' has Status 'covered-in-ci' but Bucket is empty (must be one of: operator, api-auth, api-roles, api-rbac, api-agent, api-mods, ratelimit, bot-fast, bot-heavy, multicluster, upgrade, or unbucketed() with comment)
```

**Verification**: For each `covered-*` row, ensure Bucket is one of the valid values or `unbucketed()` with a note.

---

### Check 9: Bucket Name Must Be Valid When Present

**Rule**: If Bucket is not `—`, it MUST be one of the values recognized by `test/e2e/buckets.sh` (currently: `operator`, `api-auth`, `api-roles`, `api-rbac`, `api-agent`, `api-mods`, `ratelimit`, `bot-fast`, `bot-heavy`, `multicluster`, `upgrade`), or the special `unbucketed()` marker with a required comment. This applies to any row where a Bucket is specified, whether covered or blocked.

**Failure message** (exact or very close):
```
ERROR: module 'minecraft-java' has Bucket 'bot-invalid' which is not recognized (valid: operator, api-auth, api-roles, api-rbac, api-agent, api-mods, ratelimit, bot-fast, bot-heavy, multicluster, upgrade, or unbucketed() with comment)
```

**Verification**: For each row where Bucket is not `—`, check it against the known bucket list or `unbucketed()`.

---

### Check 10: Covered-in-CI Cannot Use Bot-Heavy Bucket

**Rule**: If Status is `covered-in-ci`, Bucket MUST NOT be `bot-heavy` (bot-heavy tests are bucketed but not executed in the default CI run).

**Failure message** (exact or very close):
```
ERROR: module 'minecraft-java' has Status 'covered-in-ci' but Bucket 'bot-heavy' (bot-heavy does not run in default CI, use bot-fast or another default-executed bucket)
```

**Verification**: For each `covered-in-ci` row, ensure Bucket is not `bot-heavy`.

---

### Check 11: Covered Modules Must Have a Last Verified Date

**Rule**: If Status is `covered-in-ci` or `covered-deferred`, Last Verified MUST be a date (YYYY-MM-DD), not `—`.

**Failure message** (exact or very close):
```
ERROR: module 'minecraft-java' has Status 'covered-in-ci' but Last Verified is empty (must be YYYY-MM-DD)
```

**Verification**: For each `covered-*` row, parse Last Verified as YYYY-MM-DD and fail if it is `—` or malformed.

---

### Check 12: Blocked Modules Must Have a Blocker

**Rule**: If Status is `blocked-doc` or Status is `out-of-scope-by-design`, Blocker MUST NOT be `—`, and Blocker Class MUST be present. For `blocked-doc`, Blocker Class MUST be `documentation`. For `out-of-scope-by-design`, Blocker Class MUST be `architectural`.

**Failure message** (exact or very close):
```
ERROR: module '7-days-to-die' has Status 'blocked-doc' but Blocker is empty
```

**Verification**: For each blocked row, ensure Blocker is present and Blocker Class matches the status (`documentation` for `blocked-doc`, `architectural` for `out-of-scope-by-design`).

---

### Check 13: Blocked-Doc Modules Must Name a Concrete Unblocking Artifact

**Rule**: If Status is `blocked-doc`, the Blocker cell MUST name a concrete unblocking artifact. The verifier uses keyword matching (not semantic NLP) to verify this: the Blocker text MUST contain at least one of the recognized artifact-description keywords: `packet capture`, `field map`, `reverse-engineer`, `vendor documentation`, `protocol spec`, `documentation`, or `specification`. This is a heuristic guard against empty or hand-wavy blockers like "unknown" or "TODO" — it catches unmotivated entries and signals incomplete research. The keyword list is maintained in the verifier script and is extended when a genuinely new artifact kind is discovered. This check applies ONLY to `blocked-doc`; `out-of-scope-by-design` rows are permanent by definition and are validated by Check 14.

**Failure message** (exact or very close):
```
ERROR: module 'factorio' has Status 'blocked-doc' but Blocker does not name a concrete artifact (use one of: packet capture, field map, reverse-engineer, vendor documentation, protocol spec, documentation, specification, or similar)
```

**Verification**: For each `blocked-doc` row, check if Blocker text matches at least one of the artifact keywords. If none match, fail.

---

### Check 14: Out-of-Scope-by-Design Must Have Architectural Blocker

**Rule**: If Status is `out-of-scope-by-design`, Blocker Class MUST be `architectural`.

**Failure message** (exact or very close):
```
ERROR: module 'valheim' has Status 'out-of-scope-by-design' but Blocker Class is 'documentation' (should be 'architectural')
```

**Verification**: For each `out-of-scope-by-design` row, ensure Blocker Class = `architectural`.

---

### Check 15: Test and Bucket Cross-References Must Match

**Rule**: If both Test and Bucket are specified (not `—`), the Test function MUST be listed in that bucket's definition in `test/e2e/buckets.sh`. This ensures consistency even for blocked rows that carry a real test (e.g., a `_Query` reachability test that does not constitute join coverage).

**Failure message** (exact or very close):
```
ERROR: module 'minecraft-java' lists test 'TestGameServer_MinecraftJavaBot_Query' in Bucket 'bot-fast' but this test is not found in buckets.sh's bot-fast definition
```

**Verification**: For each row where both Test and Bucket are not `—`, verify that the test is listed in the bucket's definition.

---

### Check 16: Bot Test Names Must Agree With Depth

**Rule**: A bot test function whose name ends in `_Joined` MUST assert Depth = `JOINED`. A bot test whose name ends in `_Query` MUST assert Depth = `QUERY`. This naming convention prevents depth assertions from drifting silently.

**Failure message** (exact or very close):
```
ERROR: module 'minecraft-java' lists test 'TestGameServer_MinecraftJavaBot_Joined' but Depth is 'QUERY' (test name ends in _Joined, must have JOINED depth)
```

Or:

```
ERROR: module 'minecraft-java' lists test 'TestGameServer_MinecraftJavaBot_Query' but Depth is 'JOINED' (test name ends in _Query, must have QUERY depth)
```

**Verification**: For each row where Test is not `—`, check if the test name ends in `_Joined` or `_Query`. If it ends in `_Joined`, assert Depth == `JOINED`. If it ends in `_Query`, assert Depth == `QUERY`.

---

## Staleness Check (Warning Only, Exit Code 2)

### Check W1: Last Verified Dates Must Not Be Too Old

**Rule**: If a module has Status `covered-deferred` (not CI), and Last Verified is present and > 90 days old, emit a warning but do not fail (exit 0, not 1, but report the warning).

**Warning message** (informational):
```
WARNING: module 'factorio' (Status 'covered-deferred') last verified 120 days ago (2026-05-18). Consider re-running this test on a real cluster to ensure the protocol is still correct.
```

**Why**: Deferred tests can bit-rot if they are never run. A stale date is not a hard failure (the test may be correct), but it is visible so a maintainer knows which deferred tests need re-validation.

**Threshold**: 90 days. If `now - Last Verified > 90 days`, emit a warning.

**Exit behavior**: Warnings do not change the exit code (still 0 if all hard checks pass; still 1 if any hard check fails). They are printed to stdout/stderr as `WARNING:` lines.

---

## CI Integration

The verifier MUST be wired into CI as follows:

### Existing Job

The verifier runs in the existing **`e2e bucket coverage`** CI job (defined in `.github/workflows/ci.yaml`). This job is triggered on:

- Every push to `main`
- Every PR
- Any change to `test/e2e/`, `docs/game-coverage.md`, or the `modules/` submodule path (per path filters in ci.yaml)

### Required Wiring: Path Filter Configuration

For the verifier to run on submodule changes, `.github/workflows/ci.yaml` MUST be updated to include path filters for:

1. The `modules/` submodule path (e.g., `'modules/**'`)
2. The `docs/game-coverage.md` file

These filters MUST be added to the `dorny/paths-filter` step and folded into the gating boolean for the **`e2e` output** that controls whether the "e2e bucket coverage" job executes. Without this wiring, a submodule-pointer-only bump will not trigger the verifier, and coverage drift will go undetected.

Example path-filter configuration:
```yaml
- uses: dorny/paths-filter@v7
  id: changes
  with:
    filters: |
      modules:
        - 'modules/**'
      docs_coverage:
        - 'docs/game-coverage.md'
      # ... existing filters (go, web, charts, e2etree, ci)
      e2e:
        - 'modules/**'
        - 'docs/game-coverage.md'
        - 'test/e2e/**'
        - 'deploy/kind/**'
        - 'docker-bake.hcl'
        - '.github/workflows/ci.yaml'
```

### Invocation

Add a step to the job:

```yaml
- name: Verify game protocol coverage
  run: test/e2e/joincoverage.sh verify
  # If this exits 1, the job fails and the PR is blocked.
  # If this exits 2 (staleness warning), the job still passes but shows the warning.
```

### Failure Handling

- Exit 0: Job passes (green checkmark in GitHub).
- Exit 1: Job fails; PR is blocked until the inconsistency is fixed.
- Exit 2: Job passes but warnings are visible in the log; maintainer can review them.

### Coverage Record Change Trigger

The verifier runs when the **only change is a `modules/` submodule pointer bump** (e.g., a new module added to the submodule, or an existing module renamed). This ensures coverage status is updated in lockstep with module changes. This behavior is enabled by wiring the path filter correctly (see "Required Wiring" above).

---

## Shell Fixture Strategy: test/e2e/testdata/joincoverage/

The verifier's own correctness is proven by fixtures: shell test data files that cause the verifier to fail in specific ways. Each hard check has exactly one fixture. There are 16 fixtures (one per hard check 1–16) plus the staleness warning (W1, tested separately).

### Fixture Directory Structure

The canonical fixture names under `test/e2e/testdata/joincoverage/` are (in order, mapped to their respective checks):

```
test/e2e/testdata/joincoverage/
├── case-uninitialized-submodule/      # check 1: modules/ is empty or uninitialized
│   └── (modules/ exists but empty)
├── case-missing-module/               # check 2: module in modules/ but not in coverage
│   ├── modules/factorio/
│   └── coverage.md
├── case-stray-module/                 # check 3: module in coverage but not in modules/
│   ├── modules/minecraft-java/
│   └── coverage.md
├── case-duplicate-module/              # check 4: module listed twice in coverage
│   ├── modules/minecraft-java/
│   └── coverage.md
├── case-covered-with-query-depth/      # check 5: covered-in-ci with QUERY depth
│   ├── modules/garrys-mod/
│   └── coverage.md
├── case-covered-without-test/          # check 6: covered-in-ci with empty Test
│   ├── modules/minecraft-java/
│   └── coverage.md
├── case-invalid-test-name/             # check 7: test listed but not found in test/e2e/*_test.go
│   ├── modules/minecraft-java/
│   └── coverage.md
├── case-covered-without-bucket/        # check 8: covered-deferred with empty Bucket
│   ├── modules/minecraft-java/
│   └── coverage.md
├── case-bad-bucket-name/               # check 9: bucket name not recognized
│   ├── modules/minecraft-java/
│   └── coverage.md
├── case-covered-in-ci-in-bot-heavy/    # check 10: covered-in-ci assigned to bot-heavy bucket
│   ├── modules/minecraft-java/
│   └── coverage.md
├── case-deferred-without-lastverified/ # check 11: covered-deferred with no Last Verified date
│   ├── modules/minecraft-java/
│   └── coverage.md
├── case-blocked-without-blocker/       # check 12: blocked-doc with empty Blocker
│   ├── modules/7-days-to-die/
│   └── coverage.md
├── case-blocked-doc-without-artifact/  # check 13: blocked-doc with non-artifact Blocker
│   ├── modules/factorio/
│   └── coverage.md
├── case-architectural-not-out-of-scope/ # check 14: out-of-scope-by-design with wrong Blocker Class
│   ├── modules/valheim/
│   └── coverage.md
├── case-test-bucket-mismatch/          # check 15: test listed but not in bucket definition
│   ├── modules/minecraft-java/
│   └── coverage.md
└── case-bot-test-depth-mismatch/       # check 16: bot test name ends in _Joined but Depth is QUERY
    ├── modules/minecraft-java/
    └── coverage.md
```

### Fixture Contract

Each fixture directory contains:

1. **`modules/`**: A minimal directory structure with one or more empty module directories (just `mkdir modules/<name>` is sufficient; no actual content needed).
2. **`coverage.md`**: A malformed or inconsistent coverage table designed to trigger the specific rule.
3. **`buckets.sh`** (optional): A minimal buckets.sh for rule 13 and related checks.

The verifier is pointed at each fixture directory and MUST exit with the expected code and print the expected error message (or at least match the pattern).

### Legitimate Homes for Deferred Tests

A `covered-deferred` test (one that is verified on a live cluster but not in CI) may be bucketed in one of two ways:

1. **`bot-heavy` bucket**: The test is listed in `test/e2e/buckets.sh`'s `bot-heavy` bucket definition. These tests are bucketed and visible but deliberately excluded from the default CI job (e.g., because they require substantial resources or time). Bucket membership and default execution are orthogonal — being in a bucket does not mean the test runs in the default CI run.

2. **`unbucketed()` escape hatch**: The test is marked with `unbucketed()` in the Bucket column, with a required comment in the coverage table explaining why it is excluded. This is for edge cases where a test cannot fit into an existing bucket or for temporary exclusions awaiting a permanent solution.

A `covered-in-ci` test (one that runs in the default CI job) CANNOT use `bot-heavy`; it must use one of the default-executed buckets.

### Why Fixtures Are Essential

A verifier that has never failed is not a verifier; it is a rubber stamp. Each rule above has a fixture that makes the verifier fail, because "a gate never proven to fail is not a gate" — this is the same principle that the negative control enforces for probes (spec SC-002). Without fixtures, a bug in the verifier could silently pass bad coverage records through to production, defeating the entire purpose of the gate.

### Testing Fixtures (integration test)

A shell-based integration test (e.g., `test/e2e/joincoverage_test.sh`) runs the verifier against each of the 16 fixtures and asserts:

```bash
# example for check 2
run_verifier case-missing-module
expect_exit_code 1
expect_stderr "module 'factorio' found in modules/ but not listed"

# example for check 13
run_verifier case-blocked-doc-without-artifact
expect_exit_code 1
expect_stderr "module 'factorio' has Status 'blocked-doc' but Blocker does not name a concrete artifact"
```

This integration test runs as part of `make test-go` (build tag `e2e`, or in a separate `test/e2e/joincoverage_test.go` with shell invocation) so every CI run verifies the verifier itself.

---

## Robustness & Error Recovery

### Markdown Table Parsing

The verifier MUST be robust to:

1. **Trailing whitespace**: Each cell is trimmed of leading/trailing spaces.
2. **Multiple spaces between pipes**: `|  Module  |` is parsed as `Module`.
3. **Em-dash variants**: Only U+2014 (em-dash) is recognized as `—`; other dashes are rejected as typos.
4. **Missing backticks on module names**: If a module is listed without backticks (e.g., `minecraft-java` instead of `` `minecraft-java` ``), the verifier MUST fail with a clear message: `ERROR: module name 'minecraft-java' on row N not wrapped in backticks`.

### Error Messages

Every error message MUST:

1. Include `ERROR:` or `WARNING:` prefix (machine-parseable by CI).
2. State which module (or row, or file) is affected.
3. State what the rule requires and what was found.
4. Suggest a fix if applicable.

Example:

```
ERROR: module 'minecraft-java' has Status 'covered-in-ci' but Depth 'QUERY'.
Reason: QUERY depth (query-only protocol) does not count as join coverage per spec FR-006.
Fix: Change Status to 'blocked-*' or 'covered-deferred', or measure a deeper join protocol (PARTIAL or JOINED).
```

### Silent Failures Are Not Tolerated

If the verifier encounters an unrecoverable parse error (e.g., the coverage table is completely malformed), it MUST exit 1 with an error message, not silently succeed or skip the check.

---

## Invocation Examples

### CI Job Step

```bash
set -euo pipefail
cd test/e2e
./joincoverage.sh verify
# or
bash -c 'cd test/e2e && ./joincoverage.sh verify'
```

### Local Maintainer Run

```bash
# From repo root:
cd test/e2e
./joincoverage.sh verify

# From anywhere:
test/e2e/joincoverage.sh verify
```

### Against a Fixture (for testing the verifier itself)

```bash
# Temporarily replace docs/game-coverage.md and modules/ with a fixture
cp test/e2e/testdata/joincoverage/case-missing-module/coverage.md docs/game-coverage.md
# ... run the verifier against this broken state
test/e2e/joincoverage.sh verify
# expect: exit 1, error about missing module
```

---

## Success Criteria for the Verifier Itself

The verifier is production-ready when:

1. **All hard checks (1–16) are implemented** and exit 1 on violation.
2. **The staleness check (W1) is implemented** and emits warnings (exit 0) when dates are old.
3. **Every hard-check rule has exactly one corresponding fixture** in `test/e2e/testdata/joincoverage/` that reliably triggers the failure (16 fixtures total).
4. **An integration test runs each fixture** and asserts the expected exit code and error message.
5. **The verifier is wired into the CI job** and blocks PRs on exit 1 (but not on warnings).
6. **Maintainer can run it locally** without special setup: `cd test/e2e && ./joincoverage.sh verify`.

