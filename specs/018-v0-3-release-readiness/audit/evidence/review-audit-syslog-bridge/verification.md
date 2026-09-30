# T045 aux chunk: independent verification (opus)

Verifier input: `notes.md` in this directory (opus reviewer), candidates C-audit-syslog-bridge-02 to -05 (see held item C-audit-syslog-bridge-01, OD-019). This component's held candidates are verified separately, off-git (OD-019).

**Method.** I tried to refute each candidate against the code on `master` (`13a859ff`). The component, chart, API and operator files on this branch are byte-identical to `master` (`git diff master HEAD` on those paths is empty). I read `audit-syslog-bridge/main.go`, `bridge_test.go`, `specs.md`, `README.md`, `go.mod` and `Dockerfile`, the chart wiring (`charts/gameplane/templates/audit-syslog-bridge.yaml`, `api.yaml:283`, `values.yaml:185-195`), the producer (`api/internal/audit/audit.go:86-235`) and the API router (`api/cmd/main.go:197`, `:297`, `:647-652`). I checked `audit/findings.md`, `OPEN-DECISIONS.md` and the other components' review notes for duplicates and exemptions. None of the kept items is tracked: F-016 covers only the bridge's ingress NetworkPolicy. `go build ./...` passes in `audit-syslog-bridge/`. For (see held item C-audit-syslog-bridge-01, OD-019) and C-05 I built the bridge from this tree into the session scratchpad and ran it on `127.0.0.1` against a throwaway Python TCP listener. I ran no test or lint suite, and I changed no repo file other than this one. Severity follows research R3.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-audit-syslog-bridge-01 | kept | held (OD-019) | held (OD-019) |
| C-audit-syslog-bridge-02 | kept | S4 | Confirmed for `specs.md:75`. It names `api/internal/handlers/audit.go`, which has only `MountAudit` and `parseStreamFilter` (the read, stream and export routes), and `api/internal/notify/`, which is the separate notification system. The real producer is `WebhookSink` in `api/internal/audit/audit.go:96-216`. `git grep syslog-bridge-svc` matches only this line, and the chart wires `http://gameplane-audit-syslog-bridge.<ns>.svc:8514/` (`api.yaml:283`). **Location narrowed**: I dropped the `specs.md:63` sub-claim. Audit events come only from mutating requests, and every authenticated POST/PUT/PATCH/DELETE goes through `mutationRateLimit` (`api/cmd/main.go:297`, `:650`), so "the API rate-limits webhook submissions" holds indirectly. |
| C-audit-syslog-bridge-03 | kept | S4 | Confirmed. `bridge_test.go` has 15 `Test*` functions. `grep -n -i "deadline\|reconnect\|redial"` finds nothing. `TestForwarder_ReusesConnection` (`:265-316`) sends two frames over one healthy connection. `TestHandle_ForwardFailureIs502` (`:254-263`) fails the first dial. Neither test reaches the reconnect-and-retry branch (`main.go:242-254`) or fires the write deadline (`main.go:263`). Yet `specs.md:26` and `:68` list both as tested. (see held item C-audit-syslog-bridge-01, OD-019) |
| C-audit-syslog-bridge-04 | kept | S4 | Confirmed. `specs.md:55` says "**Go 1.25**", `go.mod:3` says `go 1.26.0`, and `Dockerfile:1` builds with `golang:1.27-alpine`. **Location narrowed** to this component's own file. The other copies the note lists belong to other components (`telemetry-receiver/specs.md:5` is C-telemetry-receiver-02 in this chunk; the operator, gameaction and gameproto copies are in their own reviews). F-026, the doc-version checker, matches only Gameplane release strings, so it doesn't cover this. |
| C-audit-syslog-bridge-05 | kept | S4 | Reproduced. With `APP_NAME="gameplane audit"`, the bridge starts without error and sends `69 <134>1 2026-09-24T14:44:09.718Z dev gameplane audit - - - {"event":1}`. Every header field after APP-NAME then shifts by one. `newServer` checks FACILITY and SEVERITY against fixed maps (`main.go:106-113`) but copies `appName` and `hostname` unchecked (`:114-124`). The chart exposes `appName` as a free-form value (`values.yaml:191`, `audit-syslog-bridge.yaml:43`). Only an operator-supplied value can trigger this, so S4. |

### C-audit-syslog-bridge-01: held (OD-019)

### C-audit-syslog-bridge-02

**Location (narrowed):** `audit-syslog-bridge/specs.md:75`.

**Repro / observation** (by reading files on `master`):
1. `sed -n 75p audit-syslog-bridge/specs.md` prints: "API audit webhook sink (`api/internal/handlers/audit.go`, `api/internal/notify/`) — calls `POST http://syslog-bridge-svc:8514/` with audit events".
2. `grep -n "^func " api/internal/handlers/audit.go` lists only `MountAudit` and `parseStreamFilter`. Nothing in that file POSTs to a webhook. `api/internal/notify/` holds the notification sinks, which are unrelated to the bridge.
3. The component that POSTs to the bridge is `WebhookSink` in `api/internal/audit/audit.go:96-216`. `api/cmd/main.go:197` builds it from `--audit-webhook-url`.
4. `git grep -n "syslog-bridge-svc"` matches only `specs.md:75`. The chart's Service is `gameplane-audit-syslog-bridge` (`charts/gameplane/templates/audit-syslog-bridge.yaml:72`), and the chart gives the API `http://gameplane-audit-syslog-bridge.<namespace>.svc:8514/` (`charts/gameplane/templates/api.yaml:283`).

**Expected:** The References entry names `api/internal/audit/audit.go` (`WebhookSink`) and the real Service DNS name.

**Actual:** It names two wrong files and a Service that doesn't exist.

### C-audit-syslog-bridge-03

**Location:** `audit-syslog-bridge/specs.md:26` and `:68`, compared with `audit-syslog-bridge/bridge_test.go` (whole file).

**Repro / observation** (by reading files on `master`):
1. `specs.md:26` says `bridge_test.go` covers "... TCP/UDP forwarding, auth, reconnection".
2. `specs.md:68` says: "Tests cover ... connection reuse, write deadline enforcement, forward-failure 502, ...".
3. `grep -n "func Test" bridge_test.go` lists 15 tests. `grep -n -i "deadline\|reconnect\|redial" bridge_test.go` finds nothing.
4. `TestForwarder_ReusesConnection` (`:265-316`) sends two frames over one healthy connection. `TestHandle_ForwardFailureIs502` (`:254-263`) targets a closed port, so the first dial fails. Neither reaches the reconnect-and-retry branch (`main.go:242-254`) or makes the `SetWriteDeadline` at `main.go:263` fire.

**Expected:** The spec lists only behaviour the tests exercise, or there are tests for the reconnect path and the write deadline.

**Actual:** Two of the test areas the spec lists have no test. (see held item C-audit-syslog-bridge-01, OD-019)

### C-audit-syslog-bridge-04

**Location (narrowed):** `audit-syslog-bridge/specs.md:55`, compared with `audit-syslog-bridge/go.mod:3`.

**Repro / observation** (by reading files on `master`):
1. `sed -n 55p audit-syslog-bridge/specs.md` prints: "**Go 1.25** (workspace-linked to `go.work` ...)".
2. `sed -n 3p audit-syslog-bridge/go.mod` prints `go 1.26.0`. `audit-syslog-bridge/Dockerfile:1` builds with `golang:1.27-alpine`.

**Expected:** The spec gives the Go version the module requires, 1.26.

**Actual:** It says 1.25.

### C-audit-syslog-bridge-05

**Location:** `audit-syslog-bridge/main.go:70` and `:73` (read `APP_NAME` and `SYSLOG_HOSTNAME`), `:114-124` (copied into `server` without a check), and `:176-186` (`buildSyslog`). The chart exposes the value at `charts/gameplane/values.yaml:191` and passes it at `charts/gameplane/templates/audit-syslog-bridge.yaml:43`.

**Repro / observation** (local, no cluster needed):
1. Build the bridge (see held item C-audit-syslog-bridge-01, OD-019), and start a TCP listener that prints what it receives on `127.0.0.1:25714`.
2. Run `APP_NAME="gameplane audit" LISTEN_ADDR=127.0.0.1:28714 SYSLOG_ADDR=127.0.0.1:25714 /tmp/bridge`. On a cluster, the same value comes from `--set api.audit.webhook.syslogBridge.appName="gameplane audit"`.
3. The bridge starts without error and logs `app="gameplane audit"`.
4. POST `{"event":1}`. The response is `204`, and the listener receives `69 <134>1 2026-09-24T14:44:09.718Z dev gameplane audit - - - {"event":1}`.
5. RFC 5424 §6 defines APP-NAME as 1*48 PRINTUSASCII, which excludes SP. A collector that splits the header on SP reads APP-NAME=`gameplane`, PROCID=`audit`, MSGID=`-` and STRUCTURED-DATA=`-`, and the MSG starts with a stray `- `.

**Expected:** `specs.md:51` promises RFC 5424 output. An `APP_NAME` or `SYSLOG_HOSTNAME` that can't be a valid RFC 5424 field is rejected at startup, the way an unknown FACILITY or SEVERITY already is (`main.go:106-113`), or it is sanitised.

**Actual:** The bridge accepts the value without complaint, and every record it sends has shifted header fields.
