# Probe CLI Contract

Every per-game probe binary at `/probe/<module>` (e.g., `/probe/minecraft-java`, `/probe/terraria`) built into the `gameplane-test/gameprobe` image must satisfy this CLI contract exactly. The shared harness (`test/e2e/gameprobe_job.go`) invokes all probes identically; games that cannot express their protocol or join depth must use `-expect-depth` flags to declare what CI can measure, and communicate the reason in their specs.md.

## Invocation Form

Each probe binary is invoked with the following structure:

```
/probe/<module> [FLAGS]
```

The binary reads flags, dials the game server, measures join depth, and exits with a code that distinguishes the outcome type:
- Exit 0 if it reached exactly the expected depth (or under -expect-fail, correctly did NOT reach it)
- Exit 2 if it connected to a live listener but did not reach the expected depth
- Exit 3 if it encountered a transport failure (nothing listening, connection refused, DNS failure, timeout)
- Exit 1 if it encountered an internal error (bad flags, panic, unusable environment)
- Exit codes MUST signal the *cause* of failure, not just a binary pass/fail

## Flag Table

Every probe must accept these flags (in any order, per Go's `flag` package):

| Flag | Type | Required? | Default | Semantics | Notes |
|---|---|---|---|---|---|
| `-addr` | string | yes | (none) | `host:port` of the game server to probe | In-cluster: `<servicename>.<namespace>.svc.cluster.local:<port>`. Must support both DNS and direct IP addresses. |
| `-deadline` | duration | no | `4m` | Total elapsed time allowed before giving up | Hard cap on `ctx.WithTimeout()`. Probe retries internally within this deadline; no external retry loop. Format per Go `time.ParseDuration`: e.g., `4m`, `240s`, `4m0s`. |
| `-expect-depth` | string | no | `JOINED` | Expected join depth: one of `QUERY`, `PARTIAL`, or `JOINED` | Probe measures the depth it actually reaches, then asserts it equals this flag. If they differ, exit 2 (connected but wrong depth) or 3 (transport error). Exact-depth assertion prevents silent regressions (e.g., credential gate moving from PARTIAL → JOINED). |
| `-expect-fail` | bool | no | false | If set, probe must NOT reach `-expect-depth`; exits 0 on correct failure, exits 1 or 2 if it somehow succeeded | Negative control: used to verify that a probe can fail on dead addresses and does not always report success. If `-expect-fail` is set and the probe reaches `-expect-depth`, exit 1 or 2 (probe succeeded when it should not have). If the probe does not reach `-expect-depth` (transport error exit 3, wrong depth exit 2, or internal error exit 1), exit 0. |

## Per-Game Extra Flags

Some games may require additional flags, e.g.:
- Minecraft: `-user` (username for the login handshake, e.g., `-user gameplane-bot`)
- Others as needed for per-server configuration

**Constraint**: Extra flags must be:
1. **Additive and namespaced**: defined AFTER the standard flags in `flag.FlagSet`, so the shared harness can parse standard flags first
2. **Fully optional**: the game must boot and the probe must reach at least a minimal depth (e.g., TCP connect or QUERY) without them
3. **Documented in the per-game `specs.md`** with their purpose and default behavior

The harness (`test/e2e/gameprobe_job.go`, `GameProbe.Args`) passes extra flags via the `Args` slice:

```go
p := GameProbe{
    Game:        "minecraft-java",
    Args:        []string{"-user", "bot"},  // appended after standard flags
    ExpectDepth: "JOINED",
    // ...
}
```

## Exit Code Contract

The probe's exit code MUST signal exactly what happened, distinguishing these cases:

| Outcome | Exit Code | Semantics | Required? |
|---|---|---|---|
| Success: reached exactly `-expect-depth` (or `-expect-fail` and did NOT reach it) | `0` | Probe succeeded; depth matches the expectation (positive or negative control). | YES |
| Failure: connected but wrong depth | `2` | Probe reached a live listener and measured a depth, but it does not match `-expect-depth`. E.g., measured QUERY but expected JOINED, or measured PARTIAL but expected JOINED. Example: server sent Encryption Request (PARTIAL) but the test expected JOINED. | YES |
| Failure: transport/connection error, nothing listening, or deadline expired | `3` | Probe could not dial, connection refused, DNS failure, no response, or deadline elapsed with zero bytes exchanged. This is the "negative control signal": the automatic control runs the probe against a dead address and asserts exit 3 to prove the probe fails because *nothing is listening*, not for an unrelated reason. | YES |
| Internal error: bad flags, panic, or unusable environment | `1` | Probe encountered an error in its own logic: invalid flag, nil dereference, environment misconfiguration. MUST NOT be used for protocol-level outcomes (wrong depth or transport failure), precisely so a broken probe cannot masquerade as a correct negative result by internally panicking. | YES |

**Why codes must be distinct**: The automatic negative control verifies probe correctness by running it against a dead address and asserting exit 3. If every failure — including a probe's own internal error — returned the same code, a probe broken in an unrelated way (typo in address parsing, panic on a nil field) would pass the negative control for entirely the wrong reason, and the gate would prove nothing. Exit codes must signal the *cause* of failure: is it the transport, the depth, or the probe itself?

## Stdout and Stderr Grammar

### Verdict Line (Machine-Readable)

The probe MUST emit exactly one machine-readable verdict line to stdout before exiting. This line MUST be parseable by the harness and by human review.

**Format** (exact order, tab-separated fields):

```
VERDICT	<result>	<depth>	<depth_evidence>
```

Where:
- `<result>` = one of: `PASS`, `FAIL_WRONG_DEPTH`, `FAIL_TRANSPORT`, `FAIL_INTERNAL_ERROR`, `FAIL_NEGATIVE_CONTROL_REACHED` (if `-expect-fail` was set and the probe unexpectedly reached the depth)
- `<depth>` = the depth the probe measured (must be one of `QUERY`, `PARTIAL`, `JOINED`, or `UNKNOWN` if transport failure). Under `-expect-fail`, `UNKNOWN` indicates the probe correctly failed to reach any depth.
- `<depth_evidence>` = a concrete server-originated artifact or observation proving the depth, or a description of the transport-level event if `UNKNOWN`

**Depth evidence rules:**

| Depth | Evidence Examples | Requirement |
|---|---|---|
| `JOINED` | `Login Success packet (0x02)`, `Player ID assigned: 42`, `Server accepted username "bot"` | MUST cite the exact server-originated artifact (packet type, field value, message) that proves the join succeeded. Invented observations do NOT count. |
| `PARTIAL` | `Server sent Encryption Request (0x01); offline-mode unavailable`, `Password required prompt received` | MUST name the specific credential gate that was encountered (Mojang auth, Steam login, custom auth, password field). Exit code 2; depth was measured (something on wire received), but insufficient for the expected depth. |
| `QUERY` | `Server List Ping: 5 players online`, `A2S_INFO response received`, `UDP port accepts packets` | MUST cite the query-protocol response that was received. Bare TCP connect or UDP dial is NOT sufficient for QUERY; a real query protocol must be spoken. Exit code 2; depth was measured. |
| `UNKNOWN` (transport failure only) | `Dial timeout after 30s`, `Connection reset by peer`, `No response to ping attempt (5 retries)` | MUST describe what connection-level event occurred, not why. Exit code 3; the probe never received a server response or connection succeeded but dropped before any protocol exchange. This is the transport-failure case. |

**Example verdict lines:**

```
VERDICT	PASS	JOINED	Login Success packet (0x02); username "gameplane-bot" accepted
VERDICT	FAIL_WRONG_DEPTH	QUERY	Server List Ping: 0 players; expected JOINED (exit 2)
VERDICT	FAIL_TRANSPORT	UNKNOWN	Dial timeout after 4m against 127.0.0.1:25565; connection never established (exit 3)
VERDICT	FAIL_WRONG_DEPTH	PARTIAL	Encryption Request (0x01) sent; Mojang auth required (offline-mode unavailable); expected JOINED (exit 2)
VERDICT	FAIL_INTERNAL_ERROR	UNKNOWN	Bad flag: -expect-depth must be one of QUERY, PARTIAL, JOINED (exit 1)
```

### Free-Form Logs

Everything else goes to stdout and/or stderr, one line per log entry, with timestamps (Go's `log` package default format: `HH:MM:SS ...` or similar). Logs must be human-readable but are not machine-parsed by the harness. They document the retry sequence and help diagnose failures.

**Example log sequence:**

```
2024-08-16T14:23:45Z Dial attempt 1/15: connecting to minecraft-java.gameplane-games.svc.cluster.local:25565
2024-08-16T14:23:47Z Handshake sent (protocol version -1, next state 1)
2024-08-16T14:23:47Z Status Request sent
2024-08-16T14:23:47Z Status Response received: 0 players online, 20 max
2024-08-16T14:23:47Z Login Handshake sent
2024-08-16T14:23:49Z Login Response: Start encryption (0x01) — Mojang auth required
2024-08-16T14:23:49Z Depth: PARTIAL (server gated by Mojang auth; offline-mode unavailable)
```

## The `-expect-fail` Contract

When the harness runs the negative control (verifying the probe can fail), it passes `-expect-fail` on the same command line as normal. The probe's behavior changes:

```bash
# Positive test: probe dials a real server, expects to reach JOINED
/probe/minecraft-java -addr real.server:25565 -expect-depth JOINED

# Negative test: probe dials a dead address, expects to fail
/probe/minecraft-java -addr 127.0.0.1:54321 -expect-fail -expect-depth JOINED
```

**Semantics:**

1. If `-expect-fail` is set, the probe MUST attempt to dial and measure depth exactly as normal.
2. If the probe successfully reaches `-expect-depth`, exit 1 or 2 (the negative control failed — the probe should not have succeeded). Specifically: exit 2 if it connected and reached the depth, or exit 1 if an internal error occurred. Both are test failures.
3. If the probe fails to reach `-expect-depth` for any reason (transport error exit 3, wrong depth exit 2, or internal error exit 1), the probe MUST exit 0 (the negative control passed — the probe correctly failed).
4. The verdict line MUST still be emitted with a result field that names what actually happened, allowing harness and human readers to understand which failure mode occurred:

```
/probe/minecraft-java -addr 127.0.0.1:54321 -expect-fail -expect-depth JOINED
# Output when transport fails (correct negative control):
# VERDICT	PASS	UNKNOWN	Dial timeout after 4m; expected to fail, and did (exit 0)
#
# (or, if wrong depth is encountered instead):
# VERDICT	PASS	PARTIAL	Encryption Request (0x01); expected to fail, and did (exit 0)
#
# (or, if the probe somehow reached the expected depth anyway — test failure):
# VERDICT	FAIL_NEGATIVE_CONTROL_REACHED	JOINED	Server accepted login; expected to fail but succeeded (exit 1 or 2)
```

The VERDICT line's result and depth fields distinguish the cause of failure (transport vs. wrong depth), even when exit code is 0. This redundancy ensures both the harness and human readers have clear, machine-parseable evidence of what happened.

## Required Properties

Every probe implementation MUST satisfy these properties, verified by inspection of the source and by empirical testing (one pass, one fail):

1. **Idempotent**: Running the same probe twice against the same server must produce the same depth and verdict (modulo log timestamps). No side effects on the server (no state created, modified, or deleted).

2. **No writes to server persistent state**: The probe dials, measures, and disconnects. It never calls commands that modify the server's world, configuration, or player lists.

3. **Honors `-deadline` as a hard cap**: The probe's overall `context.WithTimeout(ctx, deadline)` bounds all I/O. No operation can outlast the deadline. Retries happen inside the deadline; a timeout at the deadline-level is fatal (not retried).

4. **Retries internally but never masks a definitive rejection as a timeout**: If the server sends a Disconnect packet (Minecraft), a Password Required prompt (Terraria), or an explicit rejection, the probe MUST NOT retry — it should report that depth (PARTIAL or QUERY) immediately. Retries are for "not ready yet" (TCP dial backoff, query protocol no response), not for "server said no".

5. **Wraps errors with `%w`**: Every error returned from the probe (before exiting) must use `fmt.Errorf("...: %w", err)` to preserve the cause. Log lines and the verdict line must not strip the underlying error details.

6. **Handles truncated/malformed/out-of-order packets gracefully**: If the server sends garbage, the probe MUST either (a) retry or (b) report QUERY depth if query protocol is unreliable, never panic or crash.

## Worked Examples

### Example 1: Positive Control — Minecraft Probe Against a Real Server

**Invocation:**
```bash
/probe/minecraft-java -addr minecraft-java.gameplane-games.svc.cluster.local:25565 -deadline 4m -expect-depth JOINED -user gameplane-bot
```

**Expected stdout/stderr:**
```
2024-08-16T14:23:45Z Dial attempt 1/15: connecting to minecraft-java.gameplane-games.svc.cluster.local:25565
2024-08-16T14:23:46Z Handshake sent (protocol version -1, next state 1)
2024-08-16T14:23:46Z Status Request sent
2024-08-16T14:23:46Z Status Response received: version 1.21.4, protocol 767, 0/20 players
2024-08-16T14:23:47Z Login Handshake sent
2024-08-16T14:23:47Z Login Response: Start encryption (0x01)
2024-08-16T14:23:47Z Offline-mode override: no encryption, sending Login Start
2024-08-16T14:23:48Z Login Success (0x02) received: UUID 550e8400-e29b-41d4-a716-446655440000, username "gameplane-bot"
2024-08-16T14:23:48Z Depth: JOINED
VERDICT	PASS	JOINED	Login Success packet (0x02); username "gameplane-bot" accepted
```

**Exit code:** `0`

**Why this passes:**
- Probe reached JOINED (confirmed by Login Success packet).
- `-expect-depth JOINED` matches the measured depth.
- Verdict line cites the concrete server-originated artifact (Login Success 0x02).
- All errors wrapped with `%w` (visible in detailed logs).

---

### Example 2: Negative Control — Minecraft Probe Against a Dead Address

**Invocation:**
```bash
/probe/minecraft-java -addr 127.0.0.1:54321 -deadline 4m -expect-depth JOINED -user gameplane-bot -expect-fail
```

**Expected stdout/stderr:**
```
2024-08-16T14:23:45Z Dial attempt 1/15: connecting to 127.0.0.1:54321
2024-08-16T14:23:45Z connection refused
2024-08-16T14:23:48Z Dial attempt 2/15: connecting to 127.0.0.1:54321
2024-08-16T14:23:48Z connection refused
2024-08-16T14:23:51Z Dial attempt 3/15: ... (continues until deadline)
2024-08-16T14:27:45Z Deadline reached; no response from server
VERDICT	PASS	UNKNOWN	Dial timeout after 4m against 127.0.0.1:54321; connection never established; expected to fail, and did
```

**Exit code:** `0`

**Why this passes:**
- Probe encountered a transport failure (connection refused on every attempt until deadline).
- `-expect-fail` was set, so the probe's failure IS the success condition.
- Probe correctly exited 0 because it did NOT reach the expected depth.
- Exit 3 would have signaled the transport failure, but under `-expect-fail`, we invert it: exit 0 means "failure was correct, test passes".
- Verdict line documents the transport-level event (Dial timeout, connection never established) so the harness and human readers have clear evidence why the negative control passed.

---

### Example 3: Regression Detection — Minecraft Probe Regresses to Wrong Depth

**Scenario**: A previous release measured `JOINED`. Now `-expect-depth JOINED` is set. But the server configuration changed, and the probe hits an Encryption Request (online-mode enabled) instead of offline-mode success.

**Invocation:**
```bash
/probe/minecraft-java -addr minecraft-java.gameplane-games.svc.cluster.local:25565 -deadline 4m -expect-depth JOINED -user gameplane-bot
```

**Expected stdout/stderr:**
```
2024-08-16T14:23:45Z Dial attempt 1/15: connecting to minecraft-java.gameplane-games.svc.cluster.local:25565
2024-08-16T14:23:46Z Handshake sent (protocol version -1, next state 1)
2024-08-16T14:23:46Z Status Request sent
2024-08-16T14:23:46Z Status Response received: version 1.21.4, protocol 767, 0/20 players
2024-08-16T14:23:47Z Login Handshake sent
2024-08-16T14:23:47Z Login Response: Start encryption (0x01)
2024-08-16T14:23:47Z online-mode enabled; Mojang auth required
2024-08-16T14:23:47Z Depth: PARTIAL
VERDICT	FAIL_WRONG_DEPTH	PARTIAL	Encryption Request (0x01) sent; Mojang auth required (online-mode enabled)
```

**Exit code:** `2`

**Why this fails:**
- Probe reached PARTIAL (Encryption Request indicates online-mode) — the probe connected to a live listener and received a server response.
- `-expect-depth JOINED` does not match PARTIAL.
- Exit code 2 (connected but wrong depth) signals that the probe succeeded in reaching the server, but the depth regressed.
- Exact-depth assertion caught the regression; test fails loudly, alerting the maintainer that server config changed.

---

### Example 4: Internal Error — Minecraft Probe with Bad Flag

**Scenario**: A probe implementation has a bug where it fails to validate the `-expect-depth` flag before using it, leading to a nil dereference.

**Invocation:**
```bash
/probe/minecraft-java -addr minecraft-java.gameplane-games.svc.cluster.local:25565 -deadline 4m -expect-depth INVALID
```

**Expected stdout/stderr:**
```
2024-08-16T14:23:45Z Starting probe
2024-08-16T14:23:45Z Parsing flags
2024-08-16T14:23:45Z Error: invalid value for -expect-depth: INVALID; must be one of QUERY, PARTIAL, JOINED
2024-08-16T14:23:45Z Fatal: probe initialization failed
VERDICT	FAIL_INTERNAL_ERROR	UNKNOWN	Bad flag: -expect-depth=INVALID must be one of QUERY, PARTIAL, JOINED
```

**Exit code:** `1`

**Why this fails:**
- Probe encountered an internal error (bad flag) before attempting to dial.
- Exit code 1 (internal error) signals that the probe's own logic is broken, NOT that the server failed or the depth was wrong.
- This is why exit codes 1 and 2/3 must be distinct: if a probe with a bug exited 3 (the same as "nothing listening"), the automatic negative control would pass incorrectly. By reserving 1 for the probe's own errors, we ensure the negative control gate proves what it claims: the probe fails *because nothing is listening*, not for an unrelated reason.

