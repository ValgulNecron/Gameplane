# Data Model: Game Protocol E2E Coverage

**Related**: `plan.md` Phase 1 output. Defines the status vocabulary and coverage-record schema for `docs/game-coverage.md` and the coverage verifier.

**Canonical Vocabulary**: This file is the single source of truth for the four status tokens: `covered-in-ci`, `covered-deferred`, `blocked-doc`, `out-of-scope-by-design`. There is deliberately NO `blocked-architectural` status. An architectural blocker transitions directly to `out-of-scope-by-design`; the `blockerClass` field (documentation | architectural) carries the distinction for `blocked-doc` entries only.

## Entity Relationship Summary

```
GameModule (directory under modules/)
  ├─ has one CoverageRecord
  │  ├─ declares status: covered-in-ci | covered-deferred | blocked-doc | out-of-scope-by-design
  │  ├─ references optional Protocol Join Test (test function in test/e2e/)
  │  └─ references optional CI Bucket (from test/e2e/buckets.sh)
  │
  └─ Protocol Join Test (if exists)
     └─ asserts exact JoinDepth via GameProbe
        └─ receives ProbeVerdict from in-cluster probe Job

CoverageRecord status is a function of:
  - test existence (committed test file + function name)
  - test depth assertion (JOINED only counts as coverage, per FR-006)
  - bucket membership + default execution (coverage-in-ci vs deferred)
  - blocker classification (documentation | architectural)
```

---

## JoinDepth

A typed enum expressing the depth a probe reached in a game's join handshake. Defined in `test/e2e/internal/protocol/joindepth/` (to be created).

| Value | Meaning | Coverage Status | Proves |
|-------|---------|-----------------|--------|
| `QUERY` | Out-of-band status query (A2S_INFO, RCON dial, bare socket dial). Reachability only. | Never counts (FR-006) | Server has a listening socket |
| `PARTIAL` | Server parsed and accepted client handshake intent but exchange was deliberately not completed. Example: sentinel wake-on-connect tests assert this on an armed server. | Never counts | Server recognizes client as a join attempt |
| `JOINED` | Client completed the real protocol login/join handshake and observed a server-originated post-join artifact (e.g., Minecraft Login Success packet, Terraria WorldData packet). | **Only value that counts as coverage** | Server accepted and authenticated the client; game is playable |

**Ordering**: QUERY < PARTIAL < JOINED. Tests assert an exact expected depth (not a minimum), so an unexpected upgrade is a test failure signal. An assertion of QUERY when JOINED is reached is a correctness defect.

**Wire Encoding**: Uppercase string names (`"QUERY"`, `"PARTIAL"`, `"JOINED"`), stable across process boundaries. Used by:
- `test/e2e/gameprobe_job.go` GameProbe.ExpectDepth field (line 48) passed as CLI `-expect-depth` flag
- Probe binaries (e.g., `/probe/minecraft-java`) exit code 0 iff actual depth equals expected depth

---

## ProbeVerdict

The structured result a per-game probe binary reports back to the E2E test harness.

| Field | Type | Required | Semantics |
|-------|------|----------|-----------|
| `ReachedDepth` | JoinDepth | Yes | The depth actually achieved before success or failure |
| `Detail` | string | If `ReachedDepth == JOINED` | Human-readable name of the server-originated artifact proving JOINED (e.g., "login success for user bot#1", "WorldData frame received"). Must be concrete evidence, not an inference. |
| `Error` | string | If exit status != 0 | Transport or protocol error. Distinct from "reached a lower depth than expected." |

**Exit Code Encoding** (mandatory for process-boundary safety):
- **exit 0**: Reached the expected depth (or, under `-expect-fail`, correctly failed). Test passes.
- **exit 1**: Internal error in the probe itself (bad flags, panic, unusable environment). Deliberately distinct from protocol and transport outcomes so a broken probe cannot masquerade as a correct negative control by internally failing for an unrelated reason.
- **exit 2**: Connected but did not reach the expected depth. Protocol or join handshake broken. Test fails.
- **exit 3**: Transport failure / nothing listening. Connection refused, timeout, or network error. Negative control passes.

The machine-readable VERDICT line carries the same information redundantly as a fallback.

**Invariants**:
- A probe MUST NOT report `ReachedDepth == JOINED` without a non-empty Detail field naming the exact server-originated artifact observed.
- Transport errors (e.g., "connection refused", "timeout after 30s") are distinct from logic errors (e.g., "connected but server sent rejection packet"). Tests distinguish these to separate "server not up" (negative control passes) from "server up but join broken" (negative control fails). Exit codes 3 vs 2 encode this distinction.
- A probe that times out after its deadline MUST report `ReachedDepth == QUERY` (or lower) and an Error message, not a partial state without timing context.

---

## CoverageRecord

One record per game module, the row schema of `docs/game-coverage.md`. Primary key: `module` (directory name under `modules/`).

| Field | Type | Required | Constraints | Semantics |
|-------|------|----------|-------------|-----------|
| `module` | string | Yes | Must match a directory under `modules/`. Enforced by verifier. | Directory name: `minecraft-java`, `terraria`, etc. |
| `game` | string | Yes | Free text | Display name, e.g., "Minecraft Java Edition" |
| `status` | enum | Yes | One of: `covered-in-ci`, `covered-deferred`, `blocked-doc`, `out-of-scope-by-design` | Join coverage classification |
| `depth` | JoinDepth | If status is `covered-*` | MUST be `JOINED` (both `covered-in-ci` and `covered-deferred`). Sentinel wake-on-connect tests are valuable but assert PARTIAL and do not count as join coverage. | The depth the test asserts via GameProbe.ExpectDepth |
| `test` | string | If status is `covered-*` | Must match a `TestGameServer_<Name>Bot_Joined` or `TestGameServer_<Name>Bot_Query` function in `test/e2e/*_bot_e2e_test.go`, or `—` when no test exists at all | Function name that exists in the test suite, or `"—"` if no test has been authored yet. The Status and Depth columns carry the coverage judgement; this column records what reachability test actually exists. |
| `bucket` | string | If status is `covered-in-ci` | Must match a bucket name from `test/e2e/buckets.sh` | Bucket name, or reason absent (e.g., `"not bucketed"` if status is `covered-deferred`) |
| `lastVerified` | ISO 8601 date | If status is `covered-deferred` | Must be a date on or before today | When the test was last run successfully on a real cluster. Used to detect stale tests (see spec.md Edge Cases section on bit-rot detection). N/A if `covered-in-ci` (CI is continuously the evidence). |
| `blocker` | string | If status is `blocked-doc` or `out-of-scope-by-design` | Free text | Specific obstacle: missing packet capture, anti-cheat gate, platform-relay-only routing, etc. Blank if status is `covered-*` (illegal). |
| `blockerClass` | enum | If status is `blocked-doc` or `out-of-scope-by-design` | One of: `documentation`, `architectural` | `documentation`: a temporary blocker (protocol needs reverse-engineering, packet capture, or vendor docs). `architectural`: a permanent blocker (anti-cheat requiring a real client binary, platform-specific relay, GPU requirement). Illegal if status is `covered-*`. |

**Field Order Note**: The column order (module, game, status, depth, test, bucket, lastVerified, blocker, blockerClass) is load-bearing. Machine parsers and the coverage verifier key off column position, so this order must match the column order of the coverage table in `docs/game-coverage.md`. The authority for this order is `contracts/coverage-record.md`.

**Validation Rules** (enforced by verifier, `test/e2e/joincoverage.sh`):

| Condition | Illegal | Reason |
|-----------|---------|--------|
| `status == covered-in-ci` AND `test == "—"` | Yes | If coverage is proved by CI, a test function must exist |
| `status == covered-in-ci` AND `depth != JOINED` | Yes | CI can only prove JOINED (per FR-006); QUERY or PARTIAL is deferred/blocked |
| `status == covered-in-ci` AND `bucket == empty or absent` | Yes | If in CI, must be bucketed in `test/e2e/buckets.sh` |
| `status == covered-deferred` AND `lastVerified == empty` | Yes | Deferred tests must record when they were last run (edge-case bit-rot detection) |
| `status == covered-deferred` AND `depth != JOINED` | Yes | Deferred tests must assert JOINED. PARTIAL proves the server parsed a handshake, not that a join completed; FR-006 admits only a completed join. |
| `status == covered-deferred` AND `depth == PARTIAL` | Yes | Sentinel wake-on-connect tests are valuable for sentinel verification, but PARTIAL handshakes do not count as module join coverage. |
| `status == blocked-doc or out-of-scope-by-design` AND `blocker == empty` | Yes | Blocked records MUST state why |
| `status == blocked-doc or out-of-scope-by-design` AND `blockerClass == empty` | Yes | Must classify the blocker as documentation or architectural |
| `status == out-of-scope-by-design` AND `blockerClass == documentation` | Yes | Architectural constraint only; documentation blockers are blocked-doc instead |
| `status == covered-*` AND `blocker != empty` | Yes | Covered records have no blocker |
| `status == covered-*` AND `blockerClass != empty` | Yes | Covered records have no blocker classification |
| Module directory exists in `modules/` but not in CoverageRecord | Yes | Verifier fails; every module must have exactly one record |
| CoverageRecord entry references a module directory not in `modules/` | Yes | Verifier fails; stale records are caught at review time |

---

## Status State Transitions

A CoverageRecord transitions through states as its module's join coverage evolves. Allowed transitions:

```
blocked-doc ──→ covered-deferred  (test authored + run once; requires lastVerified)
              ├→ covered-in-ci    (test authored + fits CI budget; depth must be JOINED)
              └→ out-of-scope-by-design  (investigation proved blocker architectural; terminal)

covered-deferred ←→ covered-in-ci  (runner budget changes; maintain lastVerified when deferred)

covered-* ──→ blocked-doc  (protocol drift breaks join; FR-009 path; rare, reviewed)

out-of-scope-by-design  (terminal: no outbound edges without a spec amendment)
```

**Evidence Required for Each Transition**:
- **→ covered-deferred**: A committed test function with `ExpectDepth: "JOINED"`; proof of one successful run on a real cluster (recorded as `lastVerified`).
- **→ covered-in-ci**: A committed test function with `ExpectDepth: "JOINED"`; proof the test fits the existing `bot_fast` or `bot_heavy` job budget (wall-clock time, pod CPU/mem, disk).
- **→ out-of-scope-by-design**: Investigation evidence (packet capture, reverse-engineering, vendor communication, anti-cheat SDK docs) showing the blocker is permanent, not a documentation gap.
- **→ blocked-doc** (from covered-*): A PR comment or issue explaining the protocol drift (version bump, server update, upstream change) that broke the join.

Module lifecycle: adding a directory under `modules/` creates a new CoverageRecord in the same change; removing or renaming one removes or updates the record in the same change (enforced by verifier, per spec edge case).

---

## CI Bucket

An existing entity (defined in `test/e2e/buckets.sh`) that groups E2E tests by login pressure, not feature area. Related to coverage via two independent axes.

| Axis | Meaning | Example |
|------|---------|---------|
| **Bucket membership** | Is the test listed in `test/e2e/buckets.sh`? | `TestGameServer_MinecraftJavaBot_Joined` is in `bucket_bot_fast()` |
| **Default execution** | Does a CI job actually run this bucket by default (on every PR)? | `bot_fast` runs on PRs; `bot_heavy` only on `GAMEPLANE_E2E_GAMES=all` |

**Coverage Signal**: A test can be bucketed and still not run by default. This is exactly what `covered-deferred` means: a committed test that is bucketed but excluded from default CI execution. The verifier confirms:
- Tests with `status == covered-in-ci` are bucketed AND in a bucket that CI runs by default.
- Tests with `status == covered-deferred` are bucketed (or explicitly unbucketed with a comment) but NOT in a bucket CI runs by default.
- No test is silently missing from `buckets.sh` (verifier checks suite-wide completeness via `buckets.sh verify`).

**Budget Context** (from `test/e2e/buckets.sh` lines 23–34): API buckets are cut by login pressure (7 admin logins per job), not CPU. Adding a new game bot test to `bot_fast` requires proof it boots in under the time budget and does not exceed one login per test run. Heavy games are listed in `bucket_bot_heavy()` (never executed by default CI) and comment-documented why they are excluded (e.g., "multi-GB container, 10+ min boot").
