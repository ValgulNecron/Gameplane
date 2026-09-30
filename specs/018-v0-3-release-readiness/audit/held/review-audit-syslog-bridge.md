# Held review candidates: audit-syslog-bridge (OD-019, off-git)

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `audit-syslog-bridge/specs.md` (Security considerations), `audit-syslog-bridge/README.md`
- **Companion notes**: `audit/evidence/review-audit-syslog-bridge/notes.md` (non-security candidates)

## Held candidates

### H-audit-syslog-bridge-01: Inbound request bodies have no read deadline, though the spec says read deadlines bound handler goroutines

- **Control**: resource-exhaustion hardening on the bridge's HTTP intake (network exposure).
- **Location**: `audit-syslog-bridge/main.go:283-287` (the `http.Server` sets only `ReadHeaderTimeout: 10 * time.Second`); body read at `main.go:148`
- **Category**: correctness
- **Suggested severity**: S4 (hardening. With `networkPolicies.enabled`, only API pods can reach the Service, as tracked in `SECURITY_AUDIT.md` item 3, which limits exposure.)
- **Spec statement**: `specs.md:64`: "**DoS mitigation:** inbound body capped at 64 KiB; read and write deadlines bound goroutines so a misbehaving client/collector cannot wedge the process".
- **Observation**: The body size cap (64 KiB, `http.MaxBytesReader`) and the per-write collector deadline (`main.go:262-266`) are both present. On the inbound side, the only time bound is on headers. Neither `ReadTimeout` nor a per-request body deadline is set, so the time a handler goroutine spends reading the body isn't bounded. The forwarder mutex isn't held during the body read, so this doesn't block the forwarding path, but the "read deadlines" part of the spec sentence has no matching code.
- **Correct behaviour**: A request whose body isn't fully received within a bounded time is closed, for example with `http.Server.ReadTimeout` or an `http.ResponseController.SetReadDeadline` before `io.ReadAll`, so the number and lifetime of intake goroutines are bounded as the spec says.
- **How a maintainer confirms it holds**: Check that the `http.Server` in `serve` sets a read timeout (or that `handle` sets a read deadline before reading the body), and add a unit test with an `httptest` server and a client that sends headers and then stalls the body; assert that the server closes the connection within the configured bound.
- **Tracked-item note**: This isn't covered by `SECURITY_AUDIT.md` item 3, which concerns who can reach the bridge, not how long a request may take.

## Questions (not findings)

None.

## Moved from audit/evidence/review-audit-syslog-bridge/notes.md (OD-019, 2026-09-24)

### C-audit-syslog-bridge-01: First audit record after the collector drops the TCP connection is lost but reported as delivered (204)

- **Location**: `audit-syslog-bridge/main.go:242-254` (reconnect only when `f.write` returns an error), `main.go:262-266`
- **Category**: correctness
- **Suggested severity**: S3 (the external mirror silently loses records. The API database remains the source of truth, so this is a degraded audit mirror, not data loss in Gameplane itself.)
- **Observation / repro**:
  1. Run the bridge with `SYSLOG_NETWORK=tcp` (the default and the recommended setting) against a collector, and POST one event. `forwarder.conn` is now an established, reused connection.
  2. Make the collector end the connection: restart the collector process, let an idle timeout on the collector or on an intermediate LB/NAT close it, or stop the collector.
  3. POST a second event. `f.conn` is non-nil, so `send` goes straight to `f.write(frame)` (`main.go:242`). On a TCP socket whose peer has already sent FIN/closed, the first `Write` succeeds: the bytes are accepted by the local kernel and the peer answers with RST. `write` returns `nil`, `send` returns `nil`, and the handler returns `204` (`main.go:167`). The API's `WebhookSink.post` counts `gameplane_audit_webhook_events_total{result="sent"}` (`api/internal/audit/audit.go:215`).
  4. The collector never receives the second record. Only the third POST gets `EPIPE`/`ECONNRESET` and goes through the reconnect-and-retry branch at `main.go:243-253`.
- **Expected**: The code comment at `main.go:243` states the intent ("The collector may have dropped a long-idle connection; reconnect once."), and README line 32-35 says "TCP surfaces a dead collector as a `502` (→ API `failed`), which is the signal you want." A record written to a connection the peer has already closed should either reach a fresh connection or produce a 502/`failed`.
- **Actual**: Each time the collector closes the connection (restart, idle reap, crash with FIN), one record is lost, and both the bridge and the API report it as delivered. Audit events are low-rate, so idle periods between them are normal, and the "long-idle connection" case the comment targets is exactly the one that loses the first record.

### Cross-reference replaced in C-audit-syslog-bridge-03 (Actual)

The reconnect branch is also the code path involved in C-audit-syslog-bridge-01.
