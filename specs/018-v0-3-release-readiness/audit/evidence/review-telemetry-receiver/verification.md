# T045 aux chunk: independent verification (opus)

Verifier input: `notes.md` in this directory (opus reviewer), candidates C-telemetry-receiver-01 to -04. This component's held candidates are verified separately, off-git (OD-019).

**Method.** I tried to refute each candidate against the code on `master` (`13a859ff`). The component and chart files on this branch are byte-identical to `master`. I read `telemetry-receiver/main.go`, `main_test.go`, `specs.md`, `README.md`, `go.mod` and `Dockerfile`, the chart (`charts/gameplane/templates/telemetry-receiver.yaml`, `servicemonitors.yaml`, `values.yaml:219-237`), `docs/install.md:390-400`, and the API reporter's payload struct (`api/internal/telemetry/telemetry.go:59-63`). I checked `audit/findings.md`, `OPEN-DECISIONS.md` and the other components' review notes for duplicates. F-019 covers only adding the receiver's NetworkPolicy, and C-01 turned out to be the same finding as a charts candidate. `go build ./...` passes in `telemetry-receiver/`. For C-03 I built the receiver from this tree into the session scratchpad, ran it on `127.0.0.1`, and sent requests with `curl`. I ran no test or lint suite, and I changed no repo file other than this one. Severity follows research R3.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-telemetry-receiver-01 | rejected (duplicate) | (S3 if reinstated) | The defect is real, but it is the same finding, at the same location, as **C-charts-gameplane-06** (`evidence/review-charts-gameplane/notes.md:107-116`, `charts/gameplane/templates/telemetry-receiver.yaml:72-93`, S3). The chart owns the file. I confirmed the defect independently. The NetworkPolicy has one ingress rule, `podSelector app.kubernetes.io/name: gameplane-api` on TCP 8080, with no `namespaceSelector`. The same port serves `GET /metrics` (`main.go:112`). `servicemonitors.yaml` defines only the operator and API ServiceMonitors and the agent PodMonitor, so nothing scrapes the receiver. Meanwhile, `docs/install.md:393-397` and `values.yaml:230-233` present `/metrics` as the receiver's output. I rejected it here only to avoid counting it twice. If the charts verification drops C-charts-gameplane-06, reinstate this one at S3. |
| C-telemetry-receiver-02 | kept | S4 | Confirmed. `specs.md:5` says "**Go version:** 1.25", `go.mod:3` says `go 1.26.0`, and `Dockerfile:1` builds with `golang:1.27-alpine`. It is the same kind of drift as C-audit-syslog-bridge-04, but in this component's own file, so it is not a duplicate. |
| C-telemetry-receiver-03 | kept | S4 | Confirmed. `specs.md:105` lists "non-GET to `/metrics`/`/healthz` return 405" as tested. `main_test.go` has one method-guard test, `TestIngestMethodNotAllowed` (`:120-134`, `GET /ingest`). Every request to `/metrics` (`:38`) and `/healthz` (`:158`) uses `GET`. The behaviour itself is correct: the running receiver answers `PUT`, `POST` and `DELETE` on both routes with `405`. Only the testing claim is wrong. |
| C-telemetry-receiver-04 | rejected | n/a | Wording preference. `specs.md:55` quotes the exact regex `^[A-Za-z0-9][A-Za-z0-9._+-]{0,31}$` in the same sentence as "(printable ASCII, at most 32 chars)". Every character the regex accepts is printable ASCII, and it accepts at most 32 characters, so the parenthetical is a true upper bound. It doesn't claim that every printable character is accepted, and the authoritative pattern is right beside it (and again at `specs.md:15`). |

### C-telemetry-receiver-02

**Location:** `telemetry-receiver/specs.md:5`, compared with `telemetry-receiver/go.mod:3`.

**Repro / observation** (by reading files on `master`):
1. `sed -n 5p telemetry-receiver/specs.md` prints: "**Go version:** 1.25".
2. `sed -n 3p telemetry-receiver/go.mod` prints `go 1.26.0`. `telemetry-receiver/Dockerfile:1` builds with `golang:1.27-alpine`.

**Expected:** The spec gives the module's actual minimum Go version, 1.26.

**Actual:** It says 1.25. `audit-syslog-bridge/specs.md:55` has the same drift (C-audit-syslog-bridge-04), and both can be fixed in one PR.

### C-telemetry-receiver-03

**Location:** `telemetry-receiver/specs.md:105`, compared with `telemetry-receiver/main_test.go:120-134`, the only method-guard test.

**Repro / observation:**
1. `sed -n 105p telemetry-receiver/specs.md` prints: "**HTTP method guards**: non-POST to `/ingest`, non-GET to `/metrics`/`/healthz` return 405".
2. `grep -n "http.Method" telemetry-receiver/main_test.go` shows that the only request with a disallowed method is `GET /ingest` in `TestIngestMethodNotAllowed`. The `/metrics` helper (`:38`) and `TestHealthz` (`:158`) send only `GET`.
3. The behaviour is correct. To check, run `LISTEN_ADDR=127.0.0.1:28180 ./telemetry-receiver`, then send `curl -s -o /dev/null -w '%{http_code}' -X PUT http://127.0.0.1:28180/metrics`, and the same with `POST` and `DELETE` on `/metrics` and `/healthz`. Each prints `405`, because each route pattern includes its method (`main.go:109-113`).

**Expected:** The Testing section lists only what the suite exercises, or a test covers the `/metrics` and `/healthz` guards.

**Actual:** Two of the three guards the spec lists as tested have no test. A regression that registered those routes without a method, for example `mux.Handle("/metrics", ...)`, would still pass the suite.
