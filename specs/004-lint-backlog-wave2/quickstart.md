# Phase 1 Validation Guide: Lint Backlog Wave 2 — Static Analysis Gate for api, agent, test/e2e

This guide validates that three of the project's largest modules—`api`, `agent`, and `test/e2e`—have been brought under the uniform lint gate, that all linting findings have been fixed (not suppressed), and that the configuration cannot silently regress. It demonstrates that the zero-suppression property is preserved and that every module in the workspace is equally subject to static analysis.

**Critical distinction**: This project does NOT run build, test, or lint commands locally. The project's standing rule (Constitution VI, "CI Bears the Heavy Lifting") forbids it; CI is the only oracle. Therefore, **every runtime validation in this guide is expressed as: push to a branch → read the CI job → interpret the result**. Static checks that are pure text inspection (grep over the tree, reading YAML configuration) ARE allowed locally and are explicitly distinguished in each scenario. This distinction is non-negotiable and represents the single most important constraint for validating this feature.

**Prerequisite assumptions**: `git` command-line tools are available; `gh` CLI is installed and authenticated to the GitHub repository (required only for CI log reading); the git repo is cloned and on a branch.

---

## Scenario 1: Verify every go.work member is in the lint matrix

Confirms that the CI golangci-lint job's module matrix includes all 13 members of the `go.work` workspace, with no modules listed as exempted or pending cleanup. This is a static, local check (no CI execution needed).

**Prerequisites**: repo root, `.github/workflows/ci.yaml` and `go.work` readable.

**Commands**:

```bash
# Extract the list of modules from go.work (one per line, sorted)
awk '/use \\(/{flag=1;next} /\\)/{flag=0} flag {gsub(/^\t|\/\/.*$/,""); if (NF) print}' go.work | sort

# Extract the list of modules from the CI lint matrix (line ~180 in ci.yaml)
grep -A 1 'matrix:' .github/workflows/ci.yaml | grep 'module:' | \
  sed 's/.*module: //' | tr ',' '\n' | sed 's/\[//;s/\]//;s/ //g' | sort
```

**Expected success**:

```
# go.work lists:
agent
api
audit-syslog-bridge
gameaction
gameproto
mcp-server
netguard
operator
sentinel
svcutil
telemetry-receiver
test/e2e
tunnel

# lint matrix should also list all 13 of the above (with api, agent, test/e2e added by Wave 2 work)
```

**Expected failure example** (a module was added to go.work but not to the lint matrix):

```
# go.work lists: ..., newmodule, ...
# lint matrix lists: ..., tunnel  (missing: newmodule)
# => newmodule is in the workspace but not gated
```

**What this validates**: SC-001 (100% of go.work members are in the lint matrix); SC-004 (readability of the configuration); FR-004 (the matrix is explicit and maintainable). This is the guardrail against accidental omission of a module from the gate.

---

## Scenario 2: Verify zero suppression directives

Confirms that no in-source linting suppression directives exist in the tree; only the eight authorized config-level exclusions in `.golangci.yml` (inventoried in `contracts/exclusion-policy.md`), including the pre-existing gosec G115 exclusion in the Minecraft codec, are permitted. This is a static, local check.

**Prerequisites**: repo root, all Go source files present.

**Commands**:

```bash
# Search for all suppression directive patterns across the tree (excluding .git)
# False positives may appear: strings like "noslint" in test fixture names. Filter by hand.

echo "=== Searching for //nolint directives ==="
git grep -n '//nolint' -- '*.go' || echo "(none found)"

echo "=== Searching for #nosec directives (alternate format) ==="
git grep -n '#nosec' -- '*.go' || echo "(none found)"

echo "=== Searching for //#nosec (with slashes) ==="
git grep -n '//#nosec' -- '*.go' || echo "(none found)"

echo "=== Searching for lint:ignore directives ==="
git grep -n 'lint:ignore' -- '*.go' || echo "(none found)"

# One of the eight authorized config-level exclusions (Minecraft VarInt codec);
# see contracts/exclusion-policy.md for the full inventory:
echo "=== Authorized exception check (gameproto/minecraft.go) ==="
git grep -A 2 -B 2 'G115' gameproto/minecraft.go 2>/dev/null || echo "(not found — expected to be there)"
```

**Expected success**:

```
(No output for any suppression search EXCEPT the G115 check should show:)
# gameproto/minecraft.go contains a docstring comment explaining the G115 exclusion
# (The gosec exclusion is configured in .golangci.yml, not inline, so no //go:build comment appears in the file itself)
```

**Expected failure example** (a suppression directive was added):

```
api/internal/handlers/foo.go:42: //nolint:errcheck
agent/internal/console/bar.go:100: //#nosec
```

**Why this matters**: The zero-suppression property (FR-002, FR-005) is a quality signal and a constraint: linters are trusted, findings are addressable, and suppression directives are not used as a shortcut. This check must pass before, during, and after Wave 2 work. It is a regression test.

**Warning about false positives**: A test file or string constant that merely contains the substring "nolint" or "nosec" (e.g., a fixture named `testdata/noslint-example.txt` or a test assertion on the string `"nolint was here"`) will match this grep. Read the actual line to confirm it is a real directive (on the code line itself, right before a declaration), not a string literal or filename.

---

## Scenario 3: Verify the build tags are actually passed to the linter

Confirms that the CI configuration explicitly passes `--build-tags=envtest` for `api` and `--build-tags=e2e` for `test/e2e`, so that build-tag-conditional files are actually analyzed. This is a static, local check.

**Prerequisites**: repo root, `.github/workflows/ci.yaml` readable.

**Commands**:

```bash
# Check that operator gets --build-tags=envtest
echo "=== Operator build tags ==="
grep -A 5 'name: lint (operator' .github/workflows/ci.yaml | grep -i 'build-tags'

# After Wave 2, api should ALSO get --build-tags=envtest
echo "=== API build tags (post-Wave-2 only) ==="
grep -A 10 'matrix.module == .api' .github/workflows/ci.yaml | grep -i 'build-tags' || echo "(not yet; expected after Wave 2)"

# test/e2e should get --build-tags=e2e
echo "=== test/e2e build tags (post-Wave-2 only) ==="
grep -A 10 'matrix.module == .test/e2e' .github/workflows/ci.yaml | grep -i 'build-tags' || echo "(not yet; expected after Wave 2)"

# Count how many build-tag-conditional files would otherwise go unanalyzed
echo "=== Files gated by envtest build tag in api/ ==="
grep -r '//go:build envtest' api --include='*.go' | wc -l

echo "=== Files gated by e2e build tag in test/e2e/ ==="
grep -r '//go:build e2e' test/e2e --include='*.go' | wc -l
```

**Expected success**:

```
=== Operator build tags ===
args: --build-tags=envtest

=== API build tags (post-Wave-2 only) ===
args: --build-tags=envtest

=== test/e2e build tags (post-Wave-2 only) ===
args: --build-tags=e2e

=== Files gated by envtest build tag in api/ ===
7

=== Files gated by e2e build tag in test/e2e/ ===
51
```

**Expected failure example** (build tags omitted):

```
=== API build tags ===
(no output; build tag not passed)
=> 7 envtest-tagged files in api/ are silently skipped
```

**What this validates**: FR-001 (the matrix includes api and agent with correct tags); FR-007 (no code is hidden from analysis by selective tag passing); spec edge case (build-tag-conditional compilation is handled correctly). The counts (7 and 51) document which files would otherwise go unanalyzed.

---

## Scenario 4: Enumerate the backlog via CI and measure findings

Runs the matrix-enablement commit through CI to measure the current state of findings across the three modules. This is a runtime, CI-only check. The lint job will fail deliberately; a RED run here is the expected measurement, not a failure of validation.

**Prerequisites**: A branch with the Wave 2 PR that adds api, agent, and test/e2e to the lint matrix (even if fixes are incomplete). CI must run on push.

**Steps**:

1. Push the branch to GitHub (or ensure a PR is open).
2. Observe the CI lint job in the Actions tab or via the gh CLI:

```bash
# List recent runs on your branch
gh run list --branch <your-branch> --workflow ci.yaml --limit 5

# Get the run ID (most recent)
RUN_ID=$(gh run list --branch <your-branch> --workflow ci.yaml --limit 1 --json databaseId --query '.[0].databaseId' -q)

# View high-level status
gh run view "$RUN_ID" --json status,conclusion

# Get the full set of jobs in that run (to find the lint job IDs)
gh api repos/ValgulNecron/Gameplane/actions/runs/"$RUN_ID"/jobs \
  --jq '.jobs[] | select(.name|test("lint")) | {id, name, conclusion}'
```

3. Retrieve logs for each module's lint job. The job names follow the pattern `lint (module)`:

```bash
# For a specific module (e.g., api), find its job ID and fetch the log
API_JOB_ID=$(gh api repos/ValgulNecron/Gameplane/actions/runs/"$RUN_ID"/jobs \
  --jq '.jobs[] | select(.name|test("lint.*api")) | .id' -r)

# Download the full log
gh api repos/ValgulNecron/Gameplane/actions/jobs/"$API_JOB_ID"/logs > /tmp/api-lint.log

# Extract the finding summary (line count and module summary)
tail -100 /tmp/api-lint.log | grep -E 'Found|findings? in|Found'
```

4. Repeat for agent and test/e2e by changing the module name in the `select(.name|test(...))` filter.

**Expected output** (from API lint job):

```
Found 150 issues in 24 files (api module).
...
[golangci-lint summary detailing which linters found findings]
```

**What this proves**: This scenario does not "fix" anything — it is a measurement step. The counts and linters that report findings form the baseline for Wave 2 work. A RED CI run here is expected and correct; the findings must be read from the logs (not from a passed job). This validates FR-001 (the modules are in the matrix) and provides the data for the acceptance criterion SC-003 (zero findings after fixes).

**Note on log retrieval**: `gh run view --log` returns EMPTY for this repository; the working method is `gh api repos/ValgulNecron/Gameplane/actions/jobs/<job_id>/logs` as shown above. This is non-obvious and documented here to avoid repeated rediscovery.

---

## Scenario 5: Verify a module's findings are cleared

After fix commits have been pushed, confirms that golangci-lint returns zero findings for a single module. This is a runtime, CI-only check.

**Prerequisites**: A branch with Wave 2 fixes committed and pushed. CI runs on push.

**Steps**:

1. Get the latest run and a specific module's job ID (same method as Scenario 4):

```bash
RUN_ID=$(gh run list --branch <your-branch> --workflow ci.yaml --limit 1 --json databaseId --query '.[0].databaseId' -q)

# Find the api lint job
API_JOB_ID=$(gh api repos/ValgulNecron/Gameplane/actions/runs/"$RUN_ID"/jobs \
  --jq '.jobs[] | select(.name|test("lint.*api")) | .id' -r)

# Check the job's conclusion (should be "success" if all findings are gone)
gh api repos/ValgulNecron/Gameplane/actions/jobs/"$API_JOB_ID" --jq '.conclusion'
```

2. If the conclusion is `success`, the module is clear. If `failure`:

```bash
# Fetch the log to see which linters still have findings
gh api repos/ValgulNecron/Gameplane/actions/jobs/"$API_JOB_ID"/logs > /tmp/api-lint.log
tail -50 /tmp/api-lint.log
```

**Expected success**:

```
# api-lint job conclusion: "success"
# (no findings reported)
```

**Expected failure** (findings remain):

```
# api-lint job conclusion: "failure"
# Log shows:
# staticcheck: 3 issues found
# errcheck: 7 issues found
# ...
```

**What this validates**: FR-003 (findings are fixed, not suppressed); SC-003 (zero findings in each module after fixes). This scenario is run once per module to confirm all three (`api`, `agent`, `test/e2e`) reach success.

---

## Scenario 6: Verify no collateral breakage in other gates

After fixes land, confirms that the coverage gates and bucket mapping are not accidentally broken by refactoring. This is a runtime, CI-only check.

**Prerequisites**: Wave 2 fixes merged to a branch or main. CI runs on every push.

**Steps**:

1. Get all checks for the current commit/PR:

```bash
# For a PR
gh pr checks <pr-number>

# OR for a specific commit
gh api repos/ValgulNecron/Gameplane/commits/<commit-sha>/check-runs \
  --jq '.check_runs[] | {name, conclusion}' | grep -E 'coverage|bucket|e2e'
```

2. Confirm these specific checks pass:
   - `coverage/api` (must stay ≥ 80%)
   - `coverage/agent` (must stay ≥ 90%)
   - `e2e bucket coverage` (must verify test names map to buckets correctly)

3. If any coverage check is RED:

```bash
# Retrieve the coverage report from the job artifact or commit status
gh api repos/ValgulNecron/Gameplane/commits/<commit-sha>/statuses \
  --jq '.[] | select(.context|test("coverage")) | {context, description}'
```

**Expected success**:

```
coverage/api: success (Lines 82% / Statements 82% / ...)
coverage/agent: success (Lines 91% / ...)
e2e bucket coverage: success
```

**Expected failure** (a refactor broke the boundary of a frozen surface):

```
coverage/api: failure (Lines 79% < 80% threshold)
e2e bucket coverage: failure (TestGameServer_FooBot_Joined not in any bucket — bucket verifier caught a rename)
```

**Why this matters**: Fixing linting findings sometimes requires refactoring or renaming internal functions. The coverage gate and the e2e bucket name mapping are frozen surfaces (FR-006) that must not be broken. If a refactor accidentally:
- Lowers coverage below the threshold, or
- Renames an e2e test without updating `test/e2e/buckets.sh`

…then those gates will fail and must be fixed (or coverage/bucket assignments re-examined). This scenario confirms no such collateral damage occurred.

---

## Scenario 7: Verify the gate cannot silently regress

Confirms that the CI configuration is explicit, maintainable, and guards against accidental omission of a module from the lint matrix. This is a static, local check.

**Prerequisites**: repo root, `.github/workflows/ci.yaml` readable.

**Commands**:

```bash
# Check that the lint matrix is explicit (not dynamic or auto-generated)
echo "=== Lint matrix definition ==="
sed -n '169,181p' .github/workflows/ci.yaml

# Confirm there is no "pending cleanup" comment or continue-on-error on the lint step
echo "=== Checking for conditional lint exemptions ==="
grep -A 20 'name: lint (' .github/workflows/ci.yaml | grep -i 'continue\|pending\|skip' || echo "(none found — good)"

# Verify the lint matrix does NOT have `|| true` or similar that would hide failures
echo "=== Checking for silent failure suppression ==="
sed -n '169,202p' .github/workflows/ci.yaml | grep -E '\\|\\| true||| true' || echo "(none found — good)"
```

**Expected success**:

```
=== Lint matrix definition ===
strategy:
  fail-fast: false
  matrix:
    module: [netguard, gameaction, gameproto, operator, api, agent, sentinel, audit-syslog-bridge, telemetry-receiver, mcp-server, svcutil, tunnel, test/e2e]

=== Checking for conditional lint exemptions ===
(none found — good)

=== Checking for silent failure suppression ===
(none found — good)
```

**Expected failure** (a hidden exemption):

```
# Example (WRONG):
- name: lint (${{ matrix.module }})
  if: matrix.module != 'agent'  # => agent is silently exempted
  # OR
  continue-on-error: true  # => failures don't fail CI
  # OR
  run: ... || true  # => command failure is hidden
```

**What this validates**: SC-004 (reviewability in under 2 minutes) and FR-004 (the configuration is explicit and guards against omission). A reviewer should be able to read the lint matrix and instantly see all 13 modules, with no hidden conditionals.

---

## Scenario 8: Acceptance Checklist

Each Success Criterion (SC) from the specification maps to one or more scenarios above. A reviewer can use this table to confirm all spec requirements are demonstrated before approving the PR:

| Spec Criterion | Validated By | Validation Type | Evidence |
|---|---|---|---|
| **SC-001**: 100% of go.work members (13 total) are in the lint matrix; api, agent, and test/e2e are now included | Scenario 1 + Scenario 7 | Static/Local | go.work lists all 13; lint matrix includes all 13 with no exemptions; no `if:` conditionals hide any module |
| **SC-002**: Zero in-source suppression directives exist; only the eight authorized config-level `.golangci.yml` exclusions are present (contracts/exclusion-policy.md) | Scenario 2 | Static/Local | Grep over tree for //nolint, #nosec, lint:ignore returns no matches; all eight exclusions (incl. G115) are in .golangci.yml config, not inline |
| **SC-003**: golangci-lint reports zero findings for api, agent, and test/e2e after fixes are applied | Scenario 4 (baseline measurement) + Scenario 5 (per-module confirmation) | Runtime/CI-only | Each module's lint job concludes `success`; log tails show no findings for any linter |
| **SC-004**: A maintainer can read the CI config and identify all linted modules within 2 minutes, with no external docs required | Scenario 1 + Scenario 7 | Static/Local | Explicit matrix; no dynamic evaluation; matrix is self-documenting; no "pending cleanup" comments |
| **SC-005**: Frozen surfaces (audit fields, migrations, e2e test names, protocol layouts, thresholds, metrics) remain semantically identical | Scenario 6 | Runtime/CI-only | coverage/api and coverage/agent checks pass (no field renames broke exports); e2e bucket coverage passes (no test renames broke bucket.sh mapping) |

**Validation flow before merge**:

1. **Static checks (Scenarios 1, 2, 3, 7)**: Run locally before pushing. Each should complete in under 1 minute.
2. **Baseline measurement (Scenario 4)**: Push to a branch; let CI run; read the lint job logs to see finding counts per module. Record these numbers (they are the baseline for acceptance criterion SC-003).
3. **Fix and verify (Scenario 5)**: Commit fixes; push; wait for CI; confirm each module's lint job shows `success`.
4. **Collateral check (Scenario 6)**: On the same CI run, confirm coverage gates and e2e bucket mapping still pass.
5. **Finalize (Scenario 8, this checklist)**: Before merge, ensure all four criteria above are satisfied.

If all scenarios pass, Wave 2 is ready to merge.

---

## Quick-Reference Command Kit

Commands from scenarios above, in a single convenient block:

```bash
# Scenario 1: Compare go.work vs lint matrix
awk '/use \\(/{flag=1;next} /\\)/{flag=0} flag {gsub(/^\t|\/\/.*$/,""); if (NF) print}' go.work | sort

# Scenario 2: Search for suppression directives
git grep -n '//nolint' -- '*.go'
git grep -n '#nosec' -- '*.go'

# Scenario 3: Verify build tags in config
grep -A 5 'name: lint (operator' .github/workflows/ci.yaml | grep 'build-tags'
grep -r '//go:build envtest' api --include='*.go' | wc -l
grep -r '//go:build e2e' test/e2e --include='*.go' | wc -l

# Scenario 4–5: Read CI logs (requires latest run ID)
RUN_ID=$(gh run list --branch <branch> --workflow ci.yaml --limit 1 --json databaseId --query '.[0].databaseId' -q)
JOB_ID=$(gh api repos/ValgulNecron/Gameplane/actions/runs/"$RUN_ID"/jobs \
  --jq '.jobs[] | select(.name|test("lint.*api")) | .id' -r)
gh api repos/ValgulNecron/Gameplane/actions/jobs/"$JOB_ID"/logs > /tmp/lint.log
tail -50 /tmp/lint.log

# Scenario 6: Check all jobs on a commit
gh pr checks <pr-number>

# Scenario 7: Verify gate is explicit
sed -n '169,202p' .github/workflows/ci.yaml
grep -E 'continue|pending|true.*fail' .github/workflows/ci.yaml
```

---

## Notes for Reviewers

- **Scenario 4 is not a failure**: When you first push the matrix-enablement PR (before fixes are applied), the lint jobs for api, agent, and test/e2e will fail. This is expected and correct — it surfaces the backlog. Read the logs to measure; do NOT assume the backlog has been cleared until Scenario 5 shows all three modules succeeding.

- **Test name stability (Scenario 6)**: The `e2e bucket coverage` check depends on test names in `test/e2e/*_test.go` matching exactly the entries in `test/e2e/buckets.sh`. If a linting fix requires renaming a test to satisfy the revive linter, update `buckets.sh` in the same commit.

- **Log retrieval is non-obvious**: `gh run view --log` does not work on this repo. Use `gh api repos/ValgulNecron/Gameplane/actions/jobs/<job_id>/logs` instead. The exact incantation is in Scenario 4 and the command kit above.

- **False positives in grep**: A test fixture or string constant that happens to contain "nolint" or "nosec" as a substring will match grep. Always examine the actual line to confirm it is a real linting directive (immediately preceding a declaration) versus a string literal.

- **Frozen surfaces are real**: audit event field names, database migration file structure, e2e test names in `buckets.sh`, game protocol byte layouts, rate-limit constants, and Prometheus metric names cannot change. If a linting fix requires touching one of these, refactor outside the frozen API instead (e.g., extract logic to a helper; rename an internal variable, not a public field).
