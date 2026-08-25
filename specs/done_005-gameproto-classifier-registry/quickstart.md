# Quickstart: Validation & Testing for the Protocol Classifier Registry Refactor

**Feature**: 005-gameproto-classifier-registry  
**Status**: Validation & Run Guide  
**Scope**: Testing and verification scenarios that prove the refactor is correct

---

## Prerequisites

Before running any scenario below, ensure:

1. **Branch**: You are on the `005-gameproto-classifier-registry` feature branch.
2. **Go workspace**: The go.work file in the repo root links all modules, including `gameproto/` and `sentinel/`.
3. **Submodules**: `git submodule status` should show `modules/` and `website/` initialized (not strictly required for this refactor, but good to verify).
4. **Cluster for E2E**: For Scenario 4 only, the test requires a live Kubernetes cluster. Per Constitution Principle VI, all builds, tests, and linting run on CI (GitHub Actions), not locally. The commands below are documented as runnable by CI and by maintainers pushing to a branch; local execution is not the design.

---

## Scenario 1 — Behavior Equivalence Verification (FR-005, SC-007)

**Purpose**: Verify the refactored classification and response byte-building logic is identical to the current implementation for Minecraft and Terraria.

**Shape of the Comparison Test**:

The refactored `gameproto/` module must include comparison tests in `gameproto/gameproto_test.go` (or a new `comparison_test.go`) that:

1. Define identical test fixtures (saved wire-protocol handshake bytes) for:
   - **Minecraft join**: A valid Minecraft 1.20.1 client handshake (Handshake → Login Start frames).
   - **Minecraft status ping**: A valid Minecraft Status Request → Ping Request sequence.
   - **Minecraft malformed**: Truncated handshake (incomplete packet).
   - **Terraria ConnectRequest**: A valid Terraria connection message (e.g., 0x01 type tag + message body).
   - **Terraria malformed**: Truncated ConnectRequest (incomplete packet).
   - **Non-matching protocol**: Bytes that do not match any known protocol signature.

2. For each fixture:
   - Call **both** the old facade function (e.g., `ClassifyMinecraft()`) and the new Classifier interface (e.g., `MinecraftClassifier.Classify(bufio.Reader)`) with identical input bytes.
   - Assert that:
     - The returned `Kind` (Join/Status/Unknown) matches.
     - The parsed fields (for Minecraft: ProtocolVersion, NextState, ServerAddr; for Terraria: Version string) match.
     - The `Consumed` byte count matches exactly (critical for stream replay).
     - Error/nil status matches (both return an error or both return nil).

3. For response building (status/disconnect messages):
   - Call both `BuildMinecraftStatusResponse()` (old) and `MinecraftClassifier.BuildStatusResponse()` (new) with identical input payloads.
   - Assert byte-for-byte equality of the output packet bytes.
   - Do the same for `BuildMinecraftLoginDisconnect()` and `BuildTerrariaDisconnect()`.

**Expected Outcome**:

- Old implementation and new Classifier produce identical classification results and response bytes for all test fixtures.
- The `Consumed` field correctly reflects the number of bytes parsed from the input stream, enabling lossless replay.
- Malformed input (truncated, non-matching) returns Unknown without consuming extra bytes.

**Acceptance Criteria**:

- ✓ Comparison test compiles and runs on CI (GitHub Actions).
- ✓ All comparison test cases pass (old facade ↔ new Classifier byte equivalence).
- ✓ Stream replay (Consumed count) is exact for all valid handshakes.

---

## Scenario 2 — One-File Protocol Addition (SC-001)

**Purpose**: Prove the registry pattern eliminates the need to edit shared code when adding a new protocol.

**Steps** (documented for CI to execute; no local execution):

1. **Create a new protocol implementation file** — for this refactor, a minimal stub protocol (test-only, not deployed):
   ```bash
   # Create gameproto/demo.go (or use an existing protocol like factorio as a real-world demo)
   cat > gameproto/demo.go <<'EOF'
   package gameproto
   
   import (
       "bufio"
       "fmt"
   )
   
   // DemoClassifier is a minimal stub for validating the registry pattern.
   // In a real protocol, this would parse the actual handshake.
   type DemoClassifier struct{}
   
   func (t *DemoClassifier) Classify(br *bufio.Reader) (*ClassificationResult, error) {
       // Stub: always returns Unknown (or parse real handshake bytes if this were a real protocol)
       return &ClassificationResult{
           Kind:     Unknown,
           Consumed: []byte{},
           Detail:   nil,
       }, nil
   }
   
   func (t *DemoClassifier) SupportsStatusPing() bool {
       return false
   }
   
   func (t *DemoClassifier) BuildStatusResponse(payload string) ([]byte, error) {
       return nil, fmt.Errorf("demo protocol does not support status ping")
   }
   
   func (t *DemoClassifier) BuildDisconnect(reason string) ([]byte, error) {
       return []byte("disconnect"), nil
   }
   EOF
   ```

2. **Register the new protocol in the registry**:
   ```bash
   # Edit gameproto/registry.go: add ONE line to the registry map
   # Example (inside the map literal):
   #   "demo": &DemoClassifier{},
   ```

3. **Verify the diff**:
   ```bash
   # Read-only verification (CI/maintainer): examine the git diff
   git diff --stat
   # Expected: exactly 2 files changed
   #   gameproto/demo.go (new file, ~40 lines)
   #   gameproto/registry.go (1 line added in the map literal)
   ```

4. **Verify no edits to shared code**:
   ```bash
   # Read-only verification: confirm sentinel/main.go is NOT in the diff
   git diff -- sentinel/main.go
   # Expected: no output (file unchanged)
   
   # Verify gameproto/gameproto.go has NO new facade functions
   git diff gameproto/gameproto.go
   # Expected: no new ClassifyTestProto() or BuildTestProtoDisconnect() functions added
   ```

5. **Verify the new protocol is discoverable**:
   ```bash
   # Run the registry audit (Scenario 3) and confirm "demo" appears
   grep "demo" gameproto/registry.go
   # Expected: a single line registering the protocol
   ```

**Acceptance Criteria**:

- ✓ Diff touches exactly 2 files: the new protocol file and registry.go (one-line entry).
- ✓ sentinel/main.go remains unchanged.
- ✓ gameproto/gameproto.go has zero new facade functions added.
- ✓ The protocol is immediately usable by sentinel without additional dispatch logic.

---

## Scenario 3 — Registry Audit (SC-006)

**Purpose**: Verify all registered protocols are visible, correctly formatted, and free of duplicates.

**Command** (read-only, for CI and maintainers):

```bash
# Enumerate all protocol registrations in the registry
grep "\"[a-z]*\"\s*:" gameproto/registry.go | grep -v "^//"
```

**Expected Output**:

```
"minecraft": &MinecraftClassifier{},
"terraria":  &TerrariaClassifier{},
"demo": &DemoClassifier{},    # (added by this refactor as a demo)
```

**Verification Steps** (for CI/maintainer review):

1. **Count registered protocols**:
   ```bash
   grep "\"[a-z]*\"\s*:" gameproto/registry.go | grep -v "^//" | wc -l
   # Expected: 2 (minecraft, terraria) in the base refactor; 3 if demo is included
   ```

2. **Check for duplicate names**:
   ```bash
   grep "\"[a-z]*\"\s*:" gameproto/registry.go | grep -v "^//" | awk -F'"' '{print $2}' | sort | uniq -d
   # Expected: no output (no duplicates)
   ```

3. **Verify completeness** (all expected protocols are present):
   ```bash
   # Manual check: review the grep output above to confirm expected protocols appear
   # For the base refactor: minecraft and terraria MUST both be registered
   ```

**Acceptance Criteria**:

- ✓ `grep` output shows one line per registered protocol.
- ✓ All expected protocols (minecraft, terraria, demo) appear exactly once.
- ✓ No orphaned or commented-out entries (excluding actual comments).
- ✓ A code reviewer can read the registry file and immediately understand which protocols are available.

---

## Scenario 4 — Wake-on-Connect E2E Verification (SC-002, Principle I)

**Purpose**: Verify the refactored code preserves the wake-on-connect behavior end-to-end, proving stream replay (Consumed bytes) works identically.

**Commands** (read-only for CI/maintainer; all execution on GitHub Actions):

```bash
# Run the wake-on-connect tests from the bot-fast bucket
# This requires a live Kubernetes cluster (provided by CI via kind).
make test-e2e-bucket BUCKET=bot-fast
```

**Real Test Names** (from `test/e2e/buckets.sh:128-135`):

The bot-fast bucket includes these three wake-on-connect tests:

1. `TestGameServer_WakeOnConnect_PingDoesNotWake`
   - **What it tests**: A status ping (not a join) does NOT trigger a wake.
   - **Setup**: Boot a sleeping Minecraft server with wake-on-connect enabled.
   - **Action**: Send a Minecraft status ping (Query-like request).
   - **Expected**: Server remains asleep; no join detected.
   - **Refactor validation**: The Classifier must correctly identify status pings vs. joins; the Consumed bytes must be correct so the sentinel can replay a non-matching ping to the generic handler.

2. `TestGameServer_WakeOnConnect_LoginWakes`
   - **What it tests**: A login (join) attempt DOES trigger a wake.
   - **Setup**: Boot a sleeping Minecraft server with wake-on-connect enabled.
   - **Action**: Send a Minecraft login handshake (Handshake → Login Start).
   - **Expected**: Server is woken; connection forwarded to upstream.
   - **Refactor validation**: The Classifier must correctly identify a join; the Consumed bytes must enable the sentinel to forward the exact bytes (no loss, no duplication) to the real server.

3. `TestGameServer_WakeOnConnect_UnarmedNoSentinel`
   - **What it tests**: A server without wake-on-connect armed does NOT spawn sentinel.
   - **Setup**: Boot a Minecraft server with wake-on-connect disabled.
   - **Action**: Verify no sentinel pod is created.
   - **Expected**: Sentinel component is not deployed; server operates normally.
   - **Refactor validation**: The registry and dispatcher must not interfere with the non-sentinel path.

**Cluster Requirements**:

- A live Kubernetes cluster (kind, provided by CI).
- Minecraft Java Edition client available in the test environment (provided by test/e2e).
- Sentinel component deployed and configured per the Helm chart.

**CI Job**:

The bot-fast bucket (including all three wake-on-connect tests) runs on the `e2e-game-bot` CI job:
```
Job: e2e-game-bot
Platform: amd64 only
Timeout: 50m (runs as a separate, serial job; see ci.yaml line 738)
```

**Acceptance Criteria**:

- ✓ All three wake-on-connect tests pass without modification (same test names, same test logic).
- ✓ No E2E test logic change required; the refactoring is transparent to E2E.
- ✓ The Consumed bytes are exact, enabling lossless stream replay to upstream servers.
- ✓ All other bot-fast tests (MinecraftJavaBot_Joined, TerrariaBot_Joined, GarrysModBot_Query) also pass, confirming no behavior regression.

---

## Scenario 5 — Gates Verification (SC-003, SC-004, SC-005)

**Purpose**: Verify coverage thresholds, linting cleanness, and zero suppression directives are maintained.

### 5A. Coverage Gates (SC-003)

**Command** (read-only for CI; executes on GitHub Actions):

```bash
make cover
```

This command runs:
1. Unit tests with coverage profile generation (`cover-go`).
2. Envtest integration tests with coverage profile generation (`cover-go-integration`).
3. Coverage profile merging (`cover-go-merge`).
4. Threshold gate verification (`cover-go-check`).

**Expected Thresholds**:

| Module | Threshold | Config File |
|--------|-----------|-------------|
| gameproto | 90% line | gameproto/.testcoverage.yml |
| sentinel | 70% line | sentinel/.testcoverage.yml |

**Verification** (maintainer review of CI output):

```bash
# CI runs `make cover-go-check` which reports:
# >> threshold check (gameproto)
# >> threshold check (sentinel)
# Both should PASS (exit 0) with no threshold violations reported.
```

**Where This Runs**: GitHub Actions CI job `go` (per module/arch matrix combination; see ci.yaml line 208-220).

### 5B. Linting Gate (SC-005)

**Command** (read-only for CI; executes on GitHub Actions):

```bash
make lint-go
```

This command runs `golangci-lint run` across all Go modules, including `gameproto/` and `sentinel/`.

**Expected Result**:

- Zero findings for `gameproto/gameproto*.go` (all protocol implementations).
- Zero findings for `sentinel/main.go` (protocol dispatcher).
- The authorized G115 suppression for `gameproto/minecraft.go` (Minecraft VarInt encoding/decoding) remains in `.golangci.yml` and is NOT expanded.

**Verification** (CI output):

```bash
# CI runs golangci-lint; expected stderr/stdout:
# - No line-by-line error output for gameproto/ or sentinel/
# - Final exit code: 0 (success)
```

**Configuration** (source of truth: `.golangci.yml`):

The authorized suppression is:
```yaml
- path: (gameproto/)?minecraft\.go$
  linters: [gosec]
  text: "G115"
```

This is the ONLY suppression that applies to gameproto or sentinel. No new suppressions are added by this refactor.

**Where This Runs**: GitHub Actions CI job `lint` (per module matrix combination; see ci.yaml line 169-206).

### 5C. Suppression Directive Audit (SC-004)

**Command** (read-only, for CI and maintainer audit):

```bash
# Enumerate all inline suppression directives in gameproto and sentinel
grep -rn "//nolint\|//#nosec\|//lint:ignore\|//go:pragma" \
  gameproto/ sentinel/ \
  --include="*.go" \
  | grep -v "test.*\.go:"  # Exclude test files (they may have legitimate suppressions)
```

**Expected Output**:

```
(no output)
```

**Rationale**: Per CLAUDE.md rule 4 ("Fix, don't silence"), zero inline suppression directives are permitted in source code. The only exception is the G115 suppression in `.golangci.yml` for Minecraft VarInt handling, which is a config-level (not inline) exception already authorized before this refactor.

**Verification Steps**:

1. **Check for inline suppressions**:
   ```bash
   # If the grep command above returns ANY output, the refactor is incomplete.
   # Example of UNACCEPTABLE output:
   #   gameproto/minecraft.go:50: //nolint:errcheck  ← FAIL
   #   sentinel/main.go:300: //#nosec              ← FAIL
   ```

2. **Confirm config-level suppressions are unchanged**:
   ```bash
   grep -A 5 -B 5 "G115\|gameproto" .golangci.yml
   # Expected: only the Minecraft VarInt suppression appears
   ```

**Acceptance Criteria**:

- ✓ Zero inline suppressions in gameproto/*.go and sentinel/*.go.
- ✓ The config-level G115 suppression in .golangci.yml is unchanged and applies only to minecraft.go.
- ✓ A new suppression is NOT added as a shortcut to fix a linting finding; the underlying code is fixed instead.

---

## Summary: Where This Actually Runs

**Constitution Principle VI** (CI Bears the Heavy Lifting): All verification in the scenarios above runs on **GitHub Actions CI**, not on a developer's or agent's local machine. The commands are documented here for:

1. **CI job execution**: GitHub Actions runs these commands on every push to the feature branch.
2. **Maintainer audit**: A human reviewer can understand and reproduce (on CI) the verification steps.
3. **Local reference only**: A developer may read these commands to understand what CI is checking; they are not meant to be run locally.

**Key CI Jobs** for this refactor:

The table below distinguishes **GitHub Actions job names** (what appears in the checks list) from **commands/targets** the CI job executes. A maintainer reviewing the branch will see the job names in the GitHub Actions checks UI.

| Scenario | GitHub Actions Job Name | What CI Runs | Makefile Target or Command | Duration |
|----------|---|---|---|----------|
| 1 (Behavior Equivalence) | `go (gameproto / amd64)`, `go (gameproto / arm64)` | Unit tests + coverage profile for gameproto | `go test -covermode=atomic -coverpkg=./...` | ~30s per matrix leg |
| 2 (One-File Protocol) | (Manual review, no CI job) | Verification only | `git diff --stat` | ~1s |
| 3 (Registry Audit) | (Manual review, no CI job) | Verification only | `grep ":" gameproto/registry.go` | ~1s |
| 4 (Wake-on-Connect E2E) | `e2e game bot (kind)` | Wake-on-connect E2E tests | `make test-e2e-bucket BUCKET=bot-fast` | ~15–20 min (part of 50m job) |
| 5A (Coverage Gates) | `go (gameproto / amd64)`, `go (gameproto / arm64)` | Coverage threshold gate after unit/envtest tests | `make cover-go-merge && make cover-go-check` | ~2–3 min (part of go job) |
| 5B (Linting) | `lint (gameproto)` | Linting check | `golangci-lint run` | ~20s |
| 5C (Suppression Audit) | (Manual review, part of lint job) | Manual verification | `grep -rn "//nolint\|..."` | ~1s |

**Merge Criteria** (per CLAUDE.md):

A refactor branch is ready to merge when **all CI checks are green**:

- ✓ `go` job passes for all module/arch matrix legs (including Scenario 1 comparison tests for gameproto).
- ✓ `e2e-game-bot` passes (Scenario 4: bot-fast bucket, all three wake-on-connect tests).
- ✓ `go` job's coverage gate passes (Scenario 5A: gameproto 90%, sentinel 70%).
- ✓ `lint` job passes for all modules (Scenario 5B and 5C: zero suppression directives, zero linting findings).
- ✓ All other CI checks (operator, api, web, etc.) pass (no unintended side effects).

No local builds, tests, or linting runs are required or permitted; CI is the oracle.

---

## Final Checklist

Before signaling completion of the refactor:

- [ ] Scenario 1: Comparison tests compiles and passes on CI (old ↔ new byte equivalence).
- [ ] Scenario 2: A third (demo/stub) protocol is added with one new file + one registry entry; no shared-code edits.
- [ ] Scenario 3: `grep` confirms all expected protocols (minecraft, terraria, demo) are registered exactly once.
- [ ] Scenario 4: All bot-fast E2E tests pass, including the three wake-on-connect tests.
- [ ] Scenario 5A: Coverage thresholds maintained (gameproto 90%, sentinel 70%).
- [ ] Scenario 5B: Linting passes with zero findings for gameproto and sentinel.
- [ ] Scenario 5C: Zero inline suppression directives in gameproto and sentinel.
- [ ] All CI jobs green: `go`, `e2e-game-bot`, and `lint` for all matrix legs, plus other suites unaffected.

---

**End of Quickstart**
