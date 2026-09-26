# Review: telemetry-receiver

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `telemetry-receiver/specs.md`, `telemetry-receiver/README.md`, `docs/install.md#telemetry`, `docs/architecture.md` (API → telemetry-receiver bullet), `charts/gameplane/values.yaml` (`api.telemetry.*`)

## Scope reviewed

Read in full:
- `telemetry-receiver/main.go`
- `telemetry-receiver/main_test.go`
- `telemetry-receiver/specs.md`
- `telemetry-receiver/README.md`
- `telemetry-receiver/Dockerfile`
- `telemetry-receiver/go.mod`
- `telemetry-receiver/.testcoverage.yml`
- `telemetry-receiver/.gitignore`

Not read: `telemetry-receiver/go.sum` (checksums only).

Read for cross-reference (only the relevant sections):
- `charts/gameplane/templates/telemetry-receiver.yaml` (full)
- `charts/gameplane/templates/servicemonitors.yaml` (full)
- `charts/gameplane/templates/networkpolicies.yaml` (lines 1-140 in full; the rest by policy name)
- `charts/gameplane/templates/api.yaml` (telemetry endpoint and `GAMEPLANE_TELEMETRY_AUTH` wiring)
- `charts/gameplane/values.yaml` (`api.telemetry` block lines ~219-237; `networkPolicies.enabled` line 316; `serviceMonitors` line 417)
- `api/internal/telemetry/telemetry.go` (the reporter that POSTs here) and `api/cmd/main.go:479-483` (flags)
- `docs/install.md` lines 384-412 (Telemetry section)
- `docs/architecture.md` lines 288-320
- `web/src/routes/AdminSettings.tsx:1499-1509` (toggle label named in the README)
- `SECURITY_AUDIT.md` item 6 (receiver ingress NetworkPolicy; tracked, see C-telemetry-receiver-01 for the new aspect)

Compile check: `go build ./...` in `telemetry-receiver/` succeeds. No tests or linters were run.

## Method

- Compared the handler, validation, metrics and lifecycle in `main.go` against the route table, payload contract, metric names/buckets and invariants in `specs.md` and the README.
- Checked the producer (`api/internal/telemetry/telemetry.go` `payload`) against the receiver's `DisallowUnknownFields` contract, so that a field added on the API side would be caught.
- Followed the chart path an operator uses (`api.telemetry.receiver.enabled=true`) to check that the documented outputs (log line, `/metrics`) can actually be reached.
- Checked the spec's testing claims against `main_test.go`.

## Observations (no finding)

- The API's report struct (`api/internal/telemetry/telemetry.go`, `payload{Version, Servers, Templates}` with the same JSON tags) matches the receiver's `payload` exactly. `DisallowUnknownFields` doesn't reject real reports.
- Metric names, label, and histogram buckets `0,1,2,5,10,25,50,100,250` (+Inf) match `specs.md:62-70` and `main.go:86-101`. The metrics use a private registry, so no Go runtime or process collectors are mixed in. That's consistent with "three in-memory Prometheus metrics".
- The version-label sanitiser (`main.go:43`, `:141-144`) matches the regex quoted in `specs.md:15` and `:55`, and the log line uses the sanitised value.
- Auth: the whole `Authorization` value is compared in constant time, and only when `AUTH_TOKEN` is non-empty. The chart sources both `AUTH_TOKEN` (receiver) and `GAMEPLANE_TELEMETRY_AUTH` (API) from `api.telemetry.authSecretRef` (`telemetry-receiver.yaml:38-48`, `api.yaml:361-367`), as the README says.
- `serve` wraps the listen error with `%w` (`main.go:174`), so `main`'s `errors.Is(err, http.ErrServerClosed)` check works. That check is effectively never true, because `serve` only returns `ListenAndServe`'s error on the error path, before any `Shutdown`, and it doesn't hurt anything. Not reported as dead code.
- Go 1.22 method patterns (`"GET /healthz"`, `"GET /metrics"`, `"POST /ingest"`) return 405 for other methods, as specs.md says. `HEAD` is also accepted on the GET routes, which is standard.
- The README's toggle path "Admin Settings → Telemetry → Send anonymous usage metrics" matches the UI (`AdminSettings.tsx:1509`).

## Candidate findings

### C-telemetry-receiver-01: With the chart defaults, the bundled receiver's /metrics can't be scraped: its NetworkPolicy admits only API pods and no ServiceMonitor exists for it

- **Location**: `charts/gameplane/templates/telemetry-receiver.yaml:72-93`, `charts/gameplane/templates/servicemonitors.yaml` (no receiver entry), `charts/gameplane/values.yaml:316`
- **Category**: correctness
- **Suggested severity**: S3 (workaround: the operator adds their own NetworkPolicy and scrape config)
- **Tracked-item note**: `SECURITY_AUDIT.md` item 6 records adding this NetworkPolicy. That item is not re-reported here. The new aspect is the functional side effect: the policy also blocks the receiver's only output channel, `/metrics`.
- **Observation / repro**:
  1. Install with `api.telemetry.receiver.enabled=true` and leave `networkPolicies.enabled` at its default `true` (`values.yaml:316`).
  2. The chart renders NetworkPolicy `gameplane-telemetry-receiver` with `policyTypes: [Ingress]` and a single rule: `from: podSelector app.kubernetes.io/name: gameplane-api`, TCP 8080 (`telemetry-receiver.yaml:74-92`). The rule has no `namespaceSelector`, so it only matches API pods in the release namespace.
  3. A Prometheus in any other namespace (for example `monitoring`), or any non-API pod in the release namespace, is refused on `:8080/metrics`.
  4. Setting `serviceMonitors.enabled=true` doesn't help: `servicemonitors.yaml` creates monitors only for the operator and API and a PodMonitor for agents. There is no scrape object for the receiver and no `prometheus.io/*` annotations.
- **Expected**: `telemetry-receiver/README.md:13-15` says "The receiver validates it, logs it as a structured line, and aggregates it into Prometheus metrics". `docs/install.md:393-397` says: "deploy the bundled telemetry-receiver ... It logs each report and exposes aggregate Prometheus metrics (`gameplane_telemetry_reports_total` by version, fleet-size histograms) on its `/metrics`." `main.go:5-6` gives the purpose as "so an operator can chart adoption". An operator who enables the bundled receiver should be able to scrape `/metrics`, or the docs should say what extra policy and scrape config that needs.
- **Actual**: In a default install the metrics are produced but can't be reached by a scraper, so the only usable output is the per-report log line. In the other direction, `values.yaml:225-226` and `docs/install.md:403-404` say the receiver's "Service is reachable by other in-cluster pods". That is only true when `networkPolicies.enabled=false`.

### C-telemetry-receiver-02: specs.md states Go version 1.25; the module declares go 1.26.0

- **Location**: `telemetry-receiver/specs.md:5`, `telemetry-receiver/go.mod:3`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:5`: "**Go version:** 1.25".
  2. `go.mod:3`: `go 1.26.0`. The Dockerfile builds with `golang:1.27-alpine`.
- **Expected**: The spec states the module's actual Go requirement.
- **Actual**: It is one minor version behind. The same drift is in several other `specs.md` files; see C-audit-syslog-bridge-04 for the list.

### C-telemetry-receiver-03: specs.md claims tests for method guards on /metrics and /healthz that don't exist

- **Location**: `telemetry-receiver/specs.md:105`, `telemetry-receiver/main_test.go:120-134`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:105`: "**HTTP method guards**: non-POST to `/ingest`, non-GET to `/metrics`/`/healthz` return 405".
  2. `main_test.go` has one method test, `TestIngestMethodNotAllowed` (GET `/ingest`). No test sends a non-GET to `/metrics` or `/healthz`.
- **Expected**: The Testing section lists only what the suite exercises.
- **Actual**: The `/metrics` and `/healthz` method guards are listed as tested but have no test. The behaviour itself is correct (see Observations).

### C-telemetry-receiver-04: specs.md describes the accepted version charset as "printable ASCII", but the regex accepts only letters, digits and `._+-`

- **Location**: `telemetry-receiver/specs.md:55`, `telemetry-receiver/main.go:43`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:55`: "sanitized to `"invalid"` if it doesn't match `^[A-Za-z0-9][A-Za-z0-9._+-]{0,31}$` (printable ASCII, at most 32 chars)".
  2. Report `"version":"1.0 rc1"` or `"version":"1.0~rc1"`. Space and `~` are printable ASCII, but neither is in `[A-Za-z0-9._+-]`, so the report is counted under `version="invalid"`.
- **Expected**: The parenthetical matches the regex. The README's wording ("bad charset, > 32 chars", `README.md:33-34`) is accurate.
- **Actual**: The parenthetical describes a wider set than the code accepts. The regex quoted next to it is correct.

## Questions (not findings)

- **First report timing.** The API reporter's `Run` uses a ticker only (`api/internal/telemetry/telemetry.go`, `time.NewTicker(r.interval)`), so the first report arrives one full interval (24h) after API start. Every API restart resets that clock. This is API scope, not the receiver's. Is it intended that a frequently restarted API never reports?
- **Count failures sent as zero.** When the reporter's `count` fails, the error is dropped and `0` is sent for servers/templates (`telemetry.go`, `reportOnce`), so the receiver's histograms record a real-looking `0`. This is API scope. Should a failed count skip the report instead?
- **Kubelet probes and the NetworkPolicy.** This is the same question as for the audit bridge: the receiver's NetworkPolicy has no kubelet/probe allowance, unlike game pods' `allow-kubelet-probes`. Has the `/healthz` probe been checked on the CNIs that motivated `networkPolicies.kubeletCIDRs`?

held candidates: 2 (see OD-019)
