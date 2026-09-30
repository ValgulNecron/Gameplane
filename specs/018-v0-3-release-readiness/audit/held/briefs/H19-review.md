# H19 review: fix/018-harden-syslog-bridge-delivery

Reviewer: opus (tier-up review), 2026-09-24
Commit reviewed: `aea96c3fbff2d6de124070c0f4da485623ef4384` on base `origin/master` = `61f50265`
Worktree: `/tmp/claude-1000/-home-valgul-project-Gameplane/0cbfe030-6e64-40ca-9a18-d291dbf5b40a/scratchpad/wt-H19`

## Verdict: PASS

No blocking issues. Wording is clean.

## Fidelity to the brief

- `main.go`, `bridge_test.go`, `specs.md` and `README.md` match the staged files in `held/briefs/files/H19/` byte for byte (`cmp`).
- The CHANGELOG hunk matches the brief: a new `### Security hardening` heading at the end of `## [Unreleased]`, directly above `## [0.3.0-rc.1]` (line 150). The bullet is `- **audit-syslog-bridge:** hardened collector delivery reporting and intake time bounds.`, which follows the required format.
- Diff touches exactly the 5 files in brief section 5. Nothing under `specs/*/audit/held/` is committed and `SECURITY_AUDIT.md` is untouched.
- Commit subject matches the brief. The commit is signed off and has the Co-Authored-By and Claude-Session trailers.

## Compile checks (run by the reviewer)

In `audit-syslog-bridge/`: `gofmt -l .` printed nothing, and `go build ./...` and `go vet ./...` both passed. The module uses Go 1.26, so the builtin `min` and `httptest.NewRequestWithContext` are fine.

## Does it close the gaps?

### Delivery reported for a record written to a closed peer (F-194)

`send` now calls `alive()` before it reuses a connection. `alive()` does a 1-byte read with a 5 ms deadline:
- A timeout means the connection is open, so it is reused.
- EOF, RST or any other error means the collector closed it, so the bridge closes its side, sets it to nil, and redials.
- If the redial fails, the existing error path returns 502.

That matches the finding's Expected: the record either reaches a fresh connection or gets a 502.

- **TLS path checked.** `f.conn` is a `*tls.Conn` when `SYSLOG_TLS=true`. In `crypto/tls/conn.go:633,678`, a read error only becomes sticky when `!Temporary()`. A deadline error is `poll.DeadlineExceededError`, whose `Temporary()` returns true, so the probe timeout does not break the TLS conn. A `close_notify` returns `io.EOF`, which triggers a redial. TLS 1.3 post-handshake tickets are consumed inside `Read`, and the call then times out, which correctly reads as alive.
- **UDP** is unchanged: `alive()` returns true early. `newServer` only allows `tcp` or `udp`, so no `tcp4`/`tcp6` variant can slip past the check.
- **Byte consumption.** If a collector ever writes, the probe discards one byte. Syslog (RFC 6587) collectors never write, so this is harmless.
- **Residual (by design, per the brief's held notes).** A close that lands between the probe and the write can still lose one frame. Only an application-level acknowledgement would close that fully. This is not in git, and a post-merge note is enough.

### Body read has no time bound (F-199)

`newHTTPServer` sets:
- `ReadTimeout` 15 s (config field, with the default applied when zero or negative)
- `ReadHeaderTimeout` = min(10 s, read timeout)
- `IdleTimeout` 120 s

`serve` uses it. That closes the body-phase gap described against `specs.md:64`.

### Legitimate callers

The API webhook sink (`api/internal/audit/audit.go:117`) uses `http.Client{Timeout: 5s}` on DefaultTransport, which has a 90 s idle timeout.
- A 15 s read bound cannot cut off a request that the client already abandons at 5 s.
- The 120 s idle bound is longer than the client's 90 s, so the client retires idle connections first. This avoids a non-retried POST landing on a connection the server is closing.
- `/healthz` probes are unaffected.

The sender is a single worker draining a 1024-event channel. The extra 5 ms per TCP send caps one bridge at about 200 records/s. That is far above audit-event rates, so no change is needed. It is noted here in case the bridge is ever used for high-volume sources, which the README says it is schema-agnostic enough to be.

## Tests

No existing test was modified. I checked that the existing tests which reuse a connection or never drain it still behave under the probe:
- `TestForwarder_ReusesConnection`: the collector never writes, so the probe times out and the connection is reused.
- The write-deadline and forward-failure cases: same reasoning.

New tests, and whether each fails before the fix:

1. **`TestForwarder_RecordAfterCollectorCloseReachesNewConnection`.** Before the fix, the second write succeeds locally on the closed c1, c2 is never accepted, and the 2 s wait fails. After the fix, the probe sees EOF or RST, the bridge redials, and "two" arrives on c2. Either FIN or RST from the collector's close leads to a redial, so the test does not depend on which one arrives. **Real.**
2. **`TestHandle_CollectorGoneAfterCloseIs502`.** Before the fix, the second POST gets 204. After the fix, the probe sees the close, the redial is refused because the listener is closed, and the POST gets 502. **Real.** Closing the listener twice (once in the goroutine, once in the defer) is harmless.
3. **`TestIntake_BodyReadIsTimeBounded`.** Headers announce a 64-byte body and none is sent. With the 300 ms bound, the handler's body read fails, it responds, and the connection closes. `io.ReadAll` returns nil well under 3 s. On the base server, which had only `ReadHeaderTimeout`, the body read would block, and the client's 5 s deadline makes `ReadAll` return an error, so the test fails. **Real.**
4. **`TestNewHTTPServer_BoundsEveryPhase`.** Structural check of the defaults and the header bound being capped at the read bound. Fine.

Flake risk: both reconnect tests sleep 50 ms on loopback for the FIN to arrive. That is acceptable, and the brief records the choice.

E2E: none needed, since no bucket covers the bridge. `buckets.sh` and the login budget are therefore unaffected.

## Docs and specs

- `specs.md` Responsibilities, Behavior, DoS mitigation and Testing lines describe the probe, the 15 s / 10 s / 120 s bounds and the new test coverage accurately.
- `README.md` gains two accurate sentences.
- Neither doc claims the delivery guarantee is complete, which is correct given the residual described above.

## Wording-leak scan

I grepped every added git-bound line and the commit message for `F-NNN`, `Hxx`, `OD-`, attack, exploit, slowloris, repro, vulnerab, finding and held. There were **no hits**.
- The `alive()` comment ("a write would still succeed locally while the frame is lost") describes a reliability mechanism, not a way to use the gap. It came verbatim from the brief. Acceptable.
- Test names are statements of the control they check.

## Tracker notes (held only, not a fix for this branch)

- `fix-plan-held.md` H19 "Overlap" says the same tests close F-195/F-196. The brief is more precise: F-195 is closed by the two reconnect tests, but **F-196 (dedicated write-deadline test) is not**. When the `Fix` column is updated at T055/T056, record F-196 as still open.
- CHANGELOG heading conflict with other hardening branches: whichever branch merges second keeps a single `### Security hardening` heading on rebase.
