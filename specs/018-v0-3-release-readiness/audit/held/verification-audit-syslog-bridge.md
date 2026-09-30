# T045 aux chunk: independent verification (opus)

Held (OD-019, off-git). Verifier input: `audit/held/review-audit-syslog-bridge.md` (opus reviewer), candidate H-audit-syslog-bridge-01.

**Method.** I read the cited code on `master` (`13a859ff`; this component's files are identical on this branch): `audit-syslog-bridge/main.go` (the `http.Server` in `serve` and the body read in `handle`) and `audit-syslog-bridge/specs.md:64`. I then checked Go's `net/http` server semantics: `ReadHeaderTimeout` covers only the request headers, and when `ReadTimeout` is zero the connection has no read deadline while the body is read. To confirm, I ran the bridge from this tree on `127.0.0.1` and watched whether the server closed a request whose body never finished arriving. It did not close it within 14 s, past the 10 s header timeout. The bridge has no rate or size-in-time bound beyond this, and `findings.md` does not track the item: F-016 is about who can reach the bridge, not how long one request may take. I ran no test or lint suite. Severity follows research R3.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| H-audit-syslog-bridge-01 | kept | S4 | Confirmed. The spec says "read and write deadlines bound goroutines" (`specs.md:64`), but the server sets only `ReadHeaderTimeout` (`main.go:283-287`), so reading a request body has no time bound. The write deadline towards the collector (`main.go:262-266`) and the 64 KiB size cap (`main.go:148`) are both in place. S4 hardening: with the chart's default NetworkPolicy, only API pods can reach the port, and the API's client gives up after 5 s. |

### H-audit-syslog-bridge-01

**Location:** `audit-syslog-bridge/main.go:283-287` (the `http.Server` literal in `serve`) and `main.go:148` (`io.ReadAll(http.MaxBytesReader(...))` in `handle`), measured against `audit-syslog-bridge/specs.md:64`.

**Control:** the DoS bound on the bridge's HTTP intake. The spec promises three limits: a 64 KiB body cap, a read deadline and a write deadline.

**Repro / observation** (how to confirm the control holds):
1. Read `serve` in `main.go:283-287`. The server sets `ReadHeaderTimeout: 10 * time.Second` and nothing else: there is no `ReadTimeout` and no `IdleTimeout`.
2. Read `handle` in `main.go:138-168`. The body is read with `io.ReadAll` on a `MaxBytesReader`, which bounds size but not time, and nothing sets a per-request read deadline before the read. By contrast, the collector write sets a deadline (`main.go:263`).
3. Check with a unit test, not a live cluster: start the handler under `httptest.NewServer` with the production `http.Server` settings, open a raw connection, send complete headers with a non-zero `Content-Length`, then send no body. Assert that the server closes the connection within the configured read bound. On the current tree the connection stays open past the 10 s header timeout. I observed it still open after 14 s.

**Expected:** Per `specs.md:64`, a request whose body is not fully received within a bounded time is closed, for example by `http.Server.ReadTimeout` or by `http.NewResponseController(w).SetReadDeadline` before the body is read. That bounds the number and lifetime of intake goroutines.

**Actual:** Only the header phase is time-bounded. The spec's "read deadlines" have no matching code for the body phase. The forwarder mutex is not held while the body is read, so the forwarding path is unaffected. Exposure is limited by the chart's API-only NetworkPolicy (F-016) when `networkPolicies.enabled=true`.

## Moved from audit/evidence/review-audit-syslog-bridge/verification.md (OD-019, 2026-09-24)

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-audit-syslog-bridge-01 | kept | S3 | Reproduced with the real binary. After the collector closes the TCP connection, the next POST gets `204`, but its record never reaches the collector. The POST after that fails its write, reconnects and gets through. `send` (`main.go:242`) retries only when `Write` returns an error, and on Linux the first write to a socket whose peer has already closed succeeds locally. This contradicts README.md:32-35 and the retry comment at `main.go:243`. S3 rather than S1: only the external mirror loses the record, and the API database still holds every event. |

### C-audit-syslog-bridge-01

**Location:** `audit-syslog-bridge/main.go:242-254` (`forwarder.send` retries only when `write` returns an error) and `main.go:262-266` (`write`). The handler turns a `nil` from `send` into `204` at `main.go:162-167`.

**Repro / observation** (local, no cluster needed):
1. Build the bridge: `cd audit-syslog-bridge && go build -o /tmp/bridge .`.
2. Start a TCP listener on `127.0.0.1:25614`. On its first connection it reads one frame and then closes the connection. After that it keeps accepting new connections and prints every frame. The close stands in for a collector restart, or for an idle timeout on the collector or on a load balancer or NAT in between.
3. Run `LISTEN_ADDR=127.0.0.1:28614 SYSLOG_ADDR=127.0.0.1:25614 /tmp/bridge`. TCP is the default network.
4. Once `/healthz` answers, POST events about 0.5 s apart: `curl -s -o /dev/null -w '%{http_code}\n' -X POST --data '{"event":N}' http://127.0.0.1:28614/`.
5. Observed on this tree: the first delivered event arrived on connection 1, and the listener then closed that connection. The next POST returned `204`, but its frame never arrived anywhere. The POST after that also returned `204` and arrived on connection 2. The bridge logged nothing about the lost record or the reconnect.
6. On a cluster: install with `api.audit.webhook.syslogBridge.enabled=true` and a TCP collector, and make one audited change. Restart the collector, then make two more audited changes. The collector is missing the first record after the restart. The API counts that record in `gameplane_audit_webhook_events_total{result="sent"}` (`api/internal/audit/audit.go:215`).

**Expected:** README.md:32-35 says "TCP surfaces a dead collector as a `502` (→ API `failed`), which is the signal you want". The comment at `main.go:243` states the retry is there because "The collector may have dropped a long-idle connection; reconnect once." A record sent on a connection the peer has already closed should either reach a fresh connection or get a `502`, so the API counts it as `failed`.

**Actual:** The first write after the peer closes succeeds locally, and the peer answers with RST. `write` returns `nil`, so the retry branch never runs. One record is lost every time the collector closes the connection, and both the bridge (`204`) and the API (`sent`) report it as delivered. Audit events are infrequent, so long idle gaps are normal, and the idle-reap case the comment is written for is exactly the one that loses a record. Gameplane's own audit log in the database is unaffected.

### Cross-references replaced in C-audit-syslog-bridge-03

- Table row: This is separate from C-01: fixing C-01 needs a reconnect test, and the write-deadline claim would still be untested.
- Section, Actual: The untested reconnect path is where C-audit-syslog-bridge-01 lives.
