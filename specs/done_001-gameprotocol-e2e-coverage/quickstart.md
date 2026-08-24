# Phase 1 Validation Guide: Game Protocol E2E Coverage

This guide validates that every shipped game module has a committed join-protocol E2E test, classified by coverage status, and that the classification is machine-verified rather than silent assumptions. It demonstrates that real protocol handshakes are achieved, not mocked, and that failures are caught immediately when upstream versions drift.

**Prerequisite assumptions**: git submodule `modules/` is initialized (`git submodule update --init`); an operator-provided cluster context is specified via `GAMEPLANE_E2E_CONTEXT` for any hands-on run (CI provisions its own ephemeral kind cluster — you do not); docker is available for inspecting probe images.

---

## Scenario 1: Verify the coverage artifact is consistent

Runs the static verifier locally to check that `docs/game-coverage.md`, `test/e2e/buckets.sh`, and module test file definitions stay in sync. This is a shell-only check with no cluster required; the authoritative verification runs in CI's `e2e bucket coverage` job.

**Prerequisites**: repo root, modules/ initialized.

**Command**:

```bash
./test/e2e/joincoverage.sh verify
```

**Expected success**:

```
✓ all modules accounted for
✓ coverage record: 2 covered-in-ci, 0 covered-deferred, 12 blocked-doc, 2 out-of-scope-by-design
✓ bucket membership: bot-fast holds exactly 2 JOINED tests
✓ test functions matched: minecraft-java → TestGameServer_MinecraftJavaBot_Joined, terraria → TestGameServer_TerrariaBot_Joined, ...
```

**Expected failure example** (a module exists under `modules/` but has no record in `docs/game-coverage.md`):

```
✗ FAIL: module "new-game" has no entry in coverage record
✗ Coverage incomplete: 16 modules exist but only 15 are tracked
```

**Note on scope**: The verifier is a static POSIX shell script (`test/e2e/joincoverage.sh verify`) that needs no cluster. It runs in CI as part of the existing `e2e bucket coverage` job (`.github/workflows/ci.yaml`), which is the authoritative gate. Running it locally is a convenience for fast feedback during development, not evidence of merge readiness; the CI run is the proof.

---

## Scenario 2: Prove the bucket gate still holds

Confirms that every test in `test/e2e/*_test.go` belongs to exactly one bucket and no test is silently missing.

**Prerequisites**: repo root, test/e2e populated.

**Command**:

```bash
./test/e2e/buckets.sh verify
```

**Expected success**:

```
(exits 0, no output)
```

**Expected failure example** (a new test was added but not assigned to a bucket):

```
FAIL: in the suite but in no bucket (add to a bucket in ./test/e2e/buckets.sh):
TestGameServer_NewGameBot_Joined
```

**Why this matters**: The bucket verifier is the guardrail that prevents a test from being accidentally committed without being routed through CI. Without passing this gate, no game test can be added.

---

## Scenario 3: Prove the verifier itself can fail

The verifier is only trustworthy if it can catch real defects. This scenario shows that each validation rule has a fixture in `test/e2e/testdata/joincoverage/` that makes the verifier exit non-zero.

**Prerequisites**: repo root.

**Shell commands to inspect fixtures**:

```bash
# Fixture: module exists but not in coverage record
ls -la test/e2e/testdata/joincoverage/case-missing-module/

# Fixture: deferred test with no lastVerified date
ls -la test/e2e/testdata/joincoverage/case-deferred-without-lastverified/

# Fixture: test in bucket but module marked with wrong status
ls -la test/e2e/testdata/joincoverage/case-covered-without-test/
```

See `./contracts/verifier.md` for the full set of validation rule fixtures (including `case-stray-module`, `case-duplicate-module`, `case-blocked-without-blocker`, and others).

**Principle**: Every gate must have a fixture that proves it can fail. If a rule has never been seen to reject an invalid input, it is not a gate — it is dead code. The fixtures ensure the verifier stays live across maintenance refactors and give reviewers concrete scenarios to reason about.

**No need to run tests here**: the test/e2e/testdata/ files are consulted by the verifier's own unit tests (to be added in the implementation phase), which run in CI. This scenario simply documents their existence and role.

---

## Scenario 4: Run the default-CI join coverage (bot_fast bucket)

Executes the minimal join-protocol test set that runs in CI by default. Includes both positive (server accepting login) and negative (server refusing connection) controls.

**Prerequisites**:
- An operator-provided Kubernetes cluster reachable via `KUBECONFIG` and `GAMEPLANE_E2E_CONTEXT`

**What CI executes** (the authoritative run):

The CI pipeline runs the following command in the `test-e2e-bot-fast` job:

```bash
make test-e2e-bucket BUCKET=bot-fast
```

This command is **not intended for local execution**. It assumes a CI-provisioned kind cluster and network isolation. Running this on your own machine will not provide evidence of merge readiness.

**Running against an operator-provided cluster** (hands-on validation):

If you have access to an operator-provided cluster (e.g., `~/kubelab.yaml`), you can validate the same tests locally:

```bash
cd test/e2e && \
  GAMEPLANE_E2E_REUSE_CLUSTER=1 \
  GAMEPLANE_E2E_CONTEXT=<context-name> \
  KUBECONFIG=<path-to-kubeconfig> \
  GAMEPLANE_E2E_GAME_BOT=1 \
  go test -tags=e2e -timeout 35m -v -run "^(TestGameServer_MinecraftJavaBot_Joined|TestGameServer_TerrariaBot_Joined)$" ./...
```

**Expected output** (truncated):

```
=== RUN   TestGameServer_MinecraftJavaBot_Joined
    gamebot_e2e_test.go:NNN: ✓ minecraft-java GameServer is Ready
    gameprobe_job.go:NNN: minecraft-java probe passed against gs.game.svc:25565 (depth=JOINED):
      [probe log] Connecting to server...
      [probe log] Handshake OK, protocol version 757
      [probe log] Login success for user bot#1
    gamebot_e2e_test.go:NNN: ✓ negative control: minecraft-java probe correctly fails against dead port
--- PASS: TestGameServer_MinecraftJavaBot_Joined (45s)

=== RUN   TestGameServer_TerrariaBot_Joined
    gameprobe_job.go:NNN: terraria probe passed against gs.game.svc:7777 (depth=JOINED):
      [probe log] Connecting to server...
      [probe log] WorldData frame received
    gamebot_e2e_test.go:NNN: ✓ negative control: terraria probe correctly fails against dead port
--- PASS: TestGameServer_TerrariaBot_Joined (38s)
```

**Expected failure signal**: If a join depth that was previously reported as `JOINED` starts reporting `QUERY` (e.g., due to upstream server/client version drift), the test fails:

```
--- FAIL: TestGameServer_MinecraftJavaBot_Joined
    gameprobe_job.go:NNN: minecraft-java probe failed — gs.game.svc:25565 never reached depth JOINED:
      [probe log] Connected but server rejected: "Unknown protocol version"
```

**Why negative control matters** (spec FR-002): Each game's probe is run twice in its test:
1. Against the real GameServer (expecting `JOINED`)
2. Against a guaranteed-dead address like `127.0.0.1:1` (expecting failure)

If the negative control passes (i.e., the probe falsely reports success on a dead port), the test fails. This ensures no probe can accidentally report false positives by skipping network I/O or misinterpreting errors.

---

## Scenario 5: Run one game in isolation for iteration

Useful when developing or fixing a single game's protocol client. Requires an operator-provided cluster that is already running.

**Prerequisites**:
- An operator-provided Kubernetes cluster reachable via `KUBECONFIG` and `GAMEPLANE_E2E_CONTEXT`, with `gameplane` namespace deployed

**Example command** (minecraft-java only):

```bash
cd test/e2e && \
  GAMEPLANE_E2E_REUSE_CLUSTER=1 \
  GAMEPLANE_E2E_CONTEXT=<context-name> \
  KUBECONFIG=<path-to-kubeconfig> \
  GAMEPLANE_E2E_GAMES=minecraft-java \
  GAMEPLANE_E2E_GAME_BOT=1 \
  go test -tags=e2e -timeout 10m -v -run "^TestGameServer_MinecraftJavaBot_Joined$" ./...
```

**Expected output**: Same as Scenario 4, but for one game only.

**Iteration workflow**:
1. Ensure your operator-provided cluster is up and has `gameplane` namespace with the latest code deployed
2. Make a code change to the probe client (e.g., `test/e2e/internal/protocol/mcproto/client.go`)
3. Run the command above with `-run "^TestGameServer_MinecraftJavaBot_Joined$"`
4. Read probe logs; adjust the client based on the server's actual packet stream
5. Rebuild + re-run (no cluster restart needed)
6. When passing, commit + push to trigger full CI

---

## Scenario 6: Run a deferred/heavy game on an operator-provided cluster

Game modules whose servers consume multi-GB images or sustained CPU/RAM (e.g., `ark-survival-ascended`, `satisfactory`) are excluded from the default CI bucket but remain runnable on demand via a real cluster.

**Prerequisites**:
- An existing Kubernetes cluster reachable via `KUBECONFIG` and `GAMEPLANE_E2E_CONTEXT` (e.g., the operator-provided kubelab k3s instance at `~/kubelab.yaml`)
- The cluster has `gameplane` namespace deployed (run `make dev-install` or equivalent)
- Enough disk/memory for the heavy server to boot (~5–15 minutes per game)

**Example command** (run Satisfactory):

```bash
cd test/e2e && \
  GAMEPLANE_E2E_REUSE_CLUSTER=1 \
  GAMEPLANE_E2E_CONTEXT=default \
  KUBECONFIG=~/kubelab.yaml \
  GAMEPLANE_E2E_GAMES=satisfactory \
  GAMEPLANE_E2E_GAME_BOT=1 \
  go test -tags=e2e -timeout 60m -v -run "^TestGameServer_SatisfactoryBot" ./...
```

**Expected outcome**:
- If the test passes: update the `lastVerified` date in `docs/game-coverage.md` for that module in the same commit
- If the test fails: the probe logs show exactly why the join broke (missing protocol packet, version mismatch, etc.)

**Updating the Last Verified date**: After a successful heavy-game run, open `docs/game-coverage.md`, find the module's row, and update its `lastVerified` field to today's ISO 8601 date (e.g., `2026-08-16`). Commit both the test run (if code changes happened) and the date update together.

**Why this matters** (spec FR-007, FR-004): Deferred tests would otherwise bitrot unseen. Recording when each was last run — and updating it after each on-demand validation — keeps the coverage artifact honest and lets reviewers spot a test that has gone untouched for months (a signal to re-run it or validate it's still reachable).

---

## Scenario 7: Prove a probe cannot false-positive

Validates that no probe's verdict can be faked by accident or timing quirks. This is the runtime proof of spec FR-002.

**Prerequisites**: one game's probe binary available (e.g., `/probe/minecraft-java` from the gameprobe image).

**The negative control mechanism**:

The probe is run twice per game in each test:
1. **Positive control**: Probe runs against the real game server with `-expect-depth JOINED`, expecting to reach full join depth.
2. **Negative control**: Probe runs against a guaranteed-closed address (e.g., `127.0.0.1:1`) with the SAME `-expect-depth JOINED` argument, PLUS the `-expect-fail` flag, expecting the dial to fail.

**Example** (minecraft-java):

```bash
# Positive: should succeed
docker run --rm gameplane-test/gameprobe:e2e \
  /probe/minecraft-java \
  -addr gs.game.svc:25565 \
  -deadline 30s \
  -expect-depth JOINED
# Expected exit: 0 (probe succeeded in reaching JOINED)

# Negative: must fail on a closed port
docker run --rm gameplane-test/gameprobe:e2e \
  /probe/minecraft-java \
  -addr 127.0.0.1:1 \
  -deadline 10s \
  -expect-depth JOINED \
  -expect-fail
# Expected exit: 0 (probe correctly failed to dial, satisfying -expect-fail)
```

**Exit code semantics** (see `./contracts/probe-cli.md` for complete reference):
- Exit 0: probe succeeded (or succeeded at failing when `-expect-fail` is set)
- Exit 1: probe internal error (bug in probe code)
- Exit 2: probe reached an earlier depth than expected (e.g., got QUERY when expecting JOINED)
- Exit 3: transport error (cannot dial, connection reset, timeout)

If the negative control exits with 2 or 1 (instead of 0), the test fails: the probe is either falsely reporting success on a dead port, or crashing. This is checked at test-assertion time: see `gamebot_e2e_test.go` which calls `runNegativeControl()` after the positive join.

**Reference** for complete exit-code contract: `./contracts/probe-cli.md`.

---

## Scenario 8: Acceptance Checklist

Each Success Criterion (SC) from the specification maps to one or more scenarios above. A reviewer can use this table to confirm all spec requirements are demonstrated:

| Spec Criterion | Validated By | Evidence |
|---|---|---|
| **SC-001**: 100% of shipped game modules have a committed, real protocol-join E2E test | Scenario 1 (`joincoverage.sh verify`) + Scenario 2 (`buckets.sh verify`) | Coverage record lists all 16 modules; no module is missing a test function |
| **SC-002**: 100% of modules excluded from CI have a documented, reviewable reason | Scenario 1 + Scenario 6 | Exclusions are in `docs/game-coverage.md` with status set to `blocked-doc` or `out-of-scope-by-design`; heavy-module tests are in `bot-heavy` bucket with comments in `buckets.sh` explaining why they're excluded |
| **SC-003**: Zero modules in undocumented status; all are covered-in-ci, covered-deferred, blocked-doc, or out-of-scope-by-design | Scenario 1 + Scenario 2 | `joincoverage.sh verify` returns no "unknown status" errors; every record has a status field from the canonical four |
| **SC-004**: Maintainer can read full coverage status in under 5 minutes from one artifact | Scenario 1 | `docs/game-coverage.md` is readable as a markdown table; no git history or chat logs required |
| **SC-005**: A join test failing due to upstream protocol/version drift is caught immediately | Scenario 4 + Scenario 6 | Test fails with clear probe logs showing the protocol deviation (e.g., "unknown protocol version"); not discovered via user report |

---

## Running the Full Validation Suite in Sequence

A maintainer's validation flow before shipping breaks into two phases:

**Phase 1: Static checks** (run anywhere, no cluster required):

```bash
# 1. Initialize modules if not already done
git submodule update --init

# 2. Verify static consistency
./test/e2e/buckets.sh verify
./test/e2e/joincoverage.sh verify
```

**Phase 2: Runtime validation** (CI is the authoritative venue):

- **Scenarios 4–6** (join tests + negative controls) run in CI via the `test-e2e-*` jobs, which provision ephemeral kind clusters and report results. This is the source of truth for merge readiness.
- If you have access to an operator-provided cluster, you may run Scenarios 4 and 5 hands-on using the `GAMEPLANE_E2E_REUSE_CLUSTER=1` flow (see Scenario 4 and Scenario 5 respectively).
- Scenario 6 (heavy games) is typically run on-demand against an operator cluster when a deferred test needs re-validation.
- **Do not spin up a local kind cluster to run the test suite.** Local test runs are not evidence of merge readiness; only CI results are authoritative.

If Phase 1 (static checks) passes and CI Phase 2 (all test jobs) exits 0, the feature is working end-to-end.
