# Review: audit-syslog-bridge

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `audit-syslog-bridge/specs.md`, `audit-syslog-bridge/README.md`, `docs/install.md#audit-log`, `docs/architecture.md` (API → audit-syslog-bridge bullet), `charts/gameplane/values.yaml` (`api.audit.webhook.*`)

## Scope reviewed

Read in full:
- `audit-syslog-bridge/main.go`
- `audit-syslog-bridge/bridge_test.go`
- `audit-syslog-bridge/specs.md`
- `audit-syslog-bridge/README.md`
- `audit-syslog-bridge/Dockerfile`
- `audit-syslog-bridge/go.mod`
- `audit-syslog-bridge/.testcoverage.yml`
- `audit-syslog-bridge/.gitignore`

Read for cross-reference (only the relevant sections):
- `charts/gameplane/templates/audit-syslog-bridge.yaml` (full)
- `charts/gameplane/templates/api.yaml` (webhook URL auto-wiring, lines ~265-310)
- `charts/gameplane/values.yaml` (`api.audit.webhook` block, lines ~150-190; `networkPolicies`, line ~315)
- `api/internal/audit/audit.go` lines 86-235 (the `WebhookSink` that POSTs to the bridge)
- `docs/install.md` lines 313-345 (Audit log section)
- `docs/architecture.md` lines 288-320
- `docs/security.md` lines 220-240 (Pod security)
- `SECURITY_AUDIT.md` item 3 (bridge ingress NetworkPolicy; tracked, not re-reported)

Compile check: `go build ./...` in `audit-syslog-bridge/` succeeds. No tests or linters were run.

Nothing in scope was skipped.

## Method

- Went through `main.go` handler, forwarder, framing and lifecycle code path by path, comparing each against the Responsibilities, Contracts and Key invariants in `specs.md` and the Behavior/Configuration tables in the README.
- Traced the producer side (`api/internal/audit/audit.go` `WebhookSink.post`) and the chart wiring (URL, `AUTH_HEADER`, NetworkPolicy) to check the end-to-end contract.
- Checked each factual claim in `specs.md` (file references, URLs, versions, test coverage claims) against the tree.
- Reasoned about TCP semantics for the forwarder's reconnect path (Linux: the first `write()` after the peer has closed succeeds locally and draws an RST; only a later write returns `EPIPE`/`ECONNRESET`).

## Observations (no finding)

- The HTTP contract matches the spec: `POST` → 204, non-POST → 405, empty/whitespace body → 400, body over 64 KiB → 400 (`http.MaxBytesReader` error through `io.ReadAll`), auth mismatch → 401 (constant-time compare), forward failure → 502. `/healthz` returns `200 ok`.
- RFC 5424 header format, UTC millisecond timestamp, `-` for empty host/app, CR/LF collapse, and RFC 6587 octet counting for TCP (byte length, since Go `len` counts bytes) all match `specs.md:12-16` and `:51`.
- Config validation (`newServer`, `main.go:96-127`) rejects a missing `SYSLOG_ADDR`, an unknown network, TLS over UDP, and unknown facility or severity, as documented.
- Errors are wrapped with `%w` where they are returned (`dial`, `write syslog`). The dropped errors are intentional and commented: `os.Hostname` falls back to `-`, and `SetWriteDeadline`/`Close` errors are discarded on paths that already return an error.
- `serve` shuts down cleanly. `errCh` is closed after the goroutine exits, so a clean `ErrServerClosed` doesn't leak or block.
- The chart's auto-wired URL `http://gameplane-audit-syslog-bridge.<ns>.svc:8514/` (`api.yaml:283`) matches the Service name and port (`audit-syslog-bridge.yaml:69-78`). `AUTH_HEADER` is sourced from the same Secret the API sends as `Authorization` (`audit-syslog-bridge.yaml:46-55`), which matches README line 40-41.
- The API payload (`webhookPayload`, `audit.go:221-229`) is one JSON object built with `json.Marshal`, so it never contains raw newlines. The bridge's single-line collapse is a no-op for it.
- The write deadline (5s) and dial timeout (5s) are hard-coded (`main.go:75`, `:263`). The spec's "(default 5s)" (`specs.md:16`, `:50`) implies it can be configured, but no env var sets it. This is wording only.
- The Dockerfile builds with `golang:1.27-alpine` while `go.mod` declares `go 1.26.0`. That is consistent: the minimum is lower than the builder.

## Candidate findings

### C-audit-syslog-bridge-01: held (OD-019)

### C-audit-syslog-bridge-02: specs.md References point at the wrong API files and a non-existent Service URL, and claim the API rate-limits webhook submissions

- **Location**: `audit-syslog-bridge/specs.md:75`, `audit-syslog-bridge/specs.md:63`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:75` reads: "API audit webhook sink (`api/internal/handlers/audit.go`, `api/internal/notify/`) — calls `POST http://syslog-bridge-svc:8514/` with audit events".
  2. The sink that POSTs to the bridge is `WebhookSink` in `api/internal/audit/audit.go:96-216`. `api/internal/handlers/audit.go` serves the audit read/export endpoints, and `api/internal/notify/` is the separate notification system (neither sends to the bridge).
  3. The URL the chart gives the API is `http://gameplane-audit-syslog-bridge.<namespace>.svc:8514/` (`charts/gameplane/templates/api.yaml:283`). No `syslog-bridge-svc` Service exists (the Service at `charts/gameplane/templates/audit-syslog-bridge.yaml:70-72` is named `gameplane-audit-syslog-bridge`).
  4. `specs.md:63` reads: "The API rate-limits webhook submissions". `WebhookSink` has no rate limiter. It is a 1024-slot buffered channel (`audit.go:94`, `:118`) drained by a single worker, and it drops events when full (`audit.go:125-131`).
- **Expected**: References name the actual producer (`api/internal/audit/audit.go` `WebhookSink`) and the actual Service DNS name. The trust-boundary bullet describes a bounded queue with a single sequential sender that drops when full.
- **Actual**: Two wrong file references, a Service name that doesn't exist, and a rate-limit claim the API doesn't implement.

### C-audit-syslog-bridge-03: specs.md claims test coverage for reconnection and write-deadline enforcement that bridge_test.go doesn't have

- **Location**: `audit-syslog-bridge/specs.md:26`, `audit-syslog-bridge/specs.md:68`, `audit-syslog-bridge/bridge_test.go` (whole file)
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:26`: "`bridge_test.go` (unit tests covering HTTP handler, syslog framing, config validation, TCP/UDP forwarding, auth, reconnection)".
  2. `specs.md:68`: "Tests cover ... connection reuse, write deadline enforcement, forward-failure 502, ...".
  3. `bridge_test.go` has `TestForwarder_ReusesConnection` (reuse only, `:265-315`) and `TestHandle_ForwardFailureIs502` (dial failure, `:254-263`). No test makes a write fail on an established connection to exercise the reconnect-and-retry branch (`main.go:242-254`). No test stalls a collector to exercise the write deadline (`main.go:262-266`).
- **Expected**: The spec lists only behaviour the tests exercise, or the tests cover reconnection and the write deadline.
- **Actual**: Two of the spec's listed test areas have no test. (see held item C-audit-syslog-bridge-01, OD-019)

### C-audit-syslog-bridge-04: specs.md states Go 1.25; the module declares go 1.26.0

- **Location**: `audit-syslog-bridge/specs.md:55`, `audit-syslog-bridge/go.mod:3`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:55`: "**Go 1.25** (workspace-linked to `go.work` ...)".
  2. `go.mod:3`: `go 1.26.0`. All 14 workspace `go.mod` files declare `go 1.26.0`, and the Dockerfile builds with `golang:1.27-alpine`.
- **Expected**: The spec states the Go version the module requires.
- **Actual**: The spec is one minor version behind. This is cross-cutting: the same "Go 1.25" text is in `telemetry-receiver/specs.md:5`, `gameaction/specs.md`, `gameproto/specs.md`, `operator/specs.md`, `test/e2e/internal/specs.md`, and spec 018's own `plan.md:20`. This doesn't appear to be covered by the tracked `hack/check-doc-versions.sh` item, which only matches Gameplane `0.x.y-beta.N` version strings.

### C-audit-syslog-bridge-05: APP_NAME and SYSLOG_HOSTNAME aren't validated, so a value with a space produces a malformed RFC 5424 header

- **Location**: `audit-syslog-bridge/main.go:70`, `:73`, `:114-118`, `:176-186`
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. Set `APP_NAME="gameplane audit"`. The chart exposes it as `api.audit.webhook.syslogBridge.appName`, and README line 51 documents it.
  2. `newServer` validates FACILITY and SEVERITY against fixed maps, but copies `appName` and `hostname` through unchecked. `buildSyslog` formats `"<%d>1 %s %s %s - - - %s"`.
  3. The emitted header is `<134>1 2026-...Z host gameplane audit - - - {...}`. A collector splits on SP, so it reads APP-NAME=`gameplane`, PROCID=`audit`, MSGID=`-`, STRUCTURED-DATA=`-`, and MSG=`- {...}`. RFC 5424 §6 limits APP-NAME to 1*48 PRINTUSASCII (no SP) and HOSTNAME to 1*255 PRINTUSASCII.
- **Expected**: `specs.md:51` says "RFC 5424 compliance: formats message as `<PRI>1 TIMESTAMP HOSTNAME APP-NAME ...`". Either reject an invalid APP_NAME/SYSLOG_HOSTNAME at startup, as FACILITY/SEVERITY are, or sanitise it.
- **Actual**: A misconfigured value produces records whose fields are shifted, with no startup error.

## Questions (not findings)

- **Any path is accepted.** `routes()` mounts the handler on the catch-all `"/"` (`main.go:134`), so `POST /anything` is forwarded, while specs.md and the README describe only `POST /`. The API always posts to `/`, so this has no functional effect. Is the catch-all intentional?
- **Private-CA collectors.** `SYSLOG_TLS=true` verifies the collector against system roots only (`main.go:221`, no custom CA option). Go on Linux honours `SSL_CERT_FILE`/`SSL_CERT_DIR`, but the chart has no way to mount a CA bundle or set extra env on the bridge. Is a collector with a private CA (common for in-cluster SIEM) meant to be supported?
- **Handler latency vs. API timeout.** In the worst case the handler takes dial 5s + write 5s + redial 5s + write 5s. The API's `WebhookSink` client times out after 5s (`api/internal/audit/audit.go:117`). The redial uses `r.Context()` and aborts once the API disconnects, but a write already in progress runs to its own deadline. I could not build a case where the API counts `failed` for a record that was delivered, so this is only noted.
- **Kubelet probes and the NetworkPolicy.** The chart's bridge NetworkPolicy (`audit-syslog-bridge.yaml:79-100`) admits only `gameplane-api` pods, while game pods get an explicit `allow-kubelet-probes` rule (`networkpolicies.yaml:103-139`) because "Source IP depends on the CNI". Upstream NetworkPolicy semantics always allow node-originated traffic. Has the bridge's HTTP liveness/readiness probe been checked on the CNIs that motivated `kubeletCIDRs`?

held candidates: 1 (see OD-019)
