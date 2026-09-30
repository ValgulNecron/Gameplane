# H28 review: dedicated Prometheus metrics listener

- Branch: `fix/018-harden-metrics-endpoint`, commit `68850e19`, base `61f50265` (= brief base, merge-base checked)
- Worktree: `/tmp/claude-1000/-home-valgul-project-Gameplane/0cbfe030-6e64-40ca-9a18-d291dbf5b40a/scratchpad/wt-H28`
- Reviewer verdict: **pass** (two optional, non-blocking notes below)

## 1. Diff vs brief

13 files, exactly the 13 paths listed in brief section 5. Every hunk matches the brief's AFTER blocks verbatim (Edits 1-21 plus new files 2a/2b). No extra files, no `SECURITY_AUDIT.md`, nothing under `specs/*/audit/held/`. Commit is signed off, carries both trailers, message matches brief section 5. Implementer's "deviations: none" and "changedExistingTests: none" are accurate.

## 2. Against the finding and decisions

- F-227 expected result: "the public host does not serve Prometheus metrics to unauthenticated clients ... a separate metrics listener the ingress doesn't expose." D13 (questions.md) = separate listener. Implemented as decided.
- Public router no longer mounts `/metrics` (`api/cmd/main.go:259-260`); `promhttp` import moved to `api/cmd/metrics.go`. The only other `/metrics` reference in `api/` is the audit skip at `internal/audit/audit.go:751` (dead but harmless, as the brief notes).
- Ingress (`templates/ingress.yaml:24-25`) routes by port name `http` only, to `gameplane-web` or `gameplane-api`; the new `metrics` Service port is not reachable through it. With web enabled, nginx proxies non-HTML requests to `gameplane-api` port 80 (`http`), which now 404s `/metrics`.

## 3. Correctness and legitimate callers

- ServiceMonitor for the API now targets `port: metrics` with `path: /metrics`; Service and container both expose `metrics` -> 9090. Operator's own `metrics` port (8080) unaffected.
- NetworkPolicies in `networkpolicies.yaml` select only the games namespace (game pods, waker); nothing restricts ingress to the API pod in `gameplane-system`, so Prometheus and the e2e curl pod reach 9090.
- API pod has a single container; no port 9090 clash. `envOr` uses `LookupEnv`, so `GAMEPLANE_METRICS_ADDR=""` really disables the listener, as the flag help and `api/specs.md` state; the unit test's `t.Setenv` + `os.Unsetenv` pattern is correct.
- Metrics server has `ReadHeaderTimeout` and `MaxHeaderBytes`; shuts down on the same `shutCtx` after the main server.
- `metricsAddr == addr` guard is a plain string compare (would not catch `:8000` vs `0.0.0.0:8000`), but a real collision fails at bind and exits via the listen goroutine anyway. Acceptable.
- `--reuse-values`: the `| default 9090` fallback renders all four lines when the key is absent (checked, see section 5). A custom `api.metricsPort=9191` renders consistently across arg, container port and Service port.

## 4. Tests

- Unit (`api/cmd/metrics_test.go`): `TestMetricsServer_ServesPrometheusMetrics`, `TestMetricsServer_ServesOnlyMetrics`, `TestConfig_MetricsListenerSeparateFromAPIListener`. They do not compile before the fix (no `newMetricsServer`, no `metricsAddr`), so they fail pre-fix. They check the metrics listener, not the public router (built inline in `main()`); the public-port side is covered by the e2e.
- E2E `TestHelmInstall_MetricsNotOnPublicPort` (`test/e2e/helm_install_e2e_test.go:184-231`): before the fix the public port returns 200 + `# HELP` and 9090 does not answer, so it fails (Eventually times out, or the final Fatalf fires). After the fix: chi 404 on port 80, Prometheus text on 9090. `t.Parallel()`, unique pod name per retry, same curl image as `TestHelmInstall_APIHealthz`. Hard-coded 9090 matches the chart default; e2e.sh does not set the new key.
- `buckets.sh`: listed once in `bucket_operator`; `verify` passes (133 tests).
- Login budget (rule 7): no `APIClient`/login; `operator` stays at 5.
- No existing test changed or weakened.

## 5. Compile checks run by the reviewer

- `api`: `gofmt -l ./cmd` empty; `go build ./...` OK; `go vet ./cmd/` OK.
- `test/e2e`: `gofmt -l .` empty; `go vet -tags e2e ./...` OK.
- `bash test/e2e/buckets.sh verify`: `buckets OK: 133 tests, all in exactly one bucket`.
- `helm template ... --set serviceMonitors.enabled=true`: `--metrics-addr=:9090`, `{ name: metrics, containerPort: 9090 }`, `{ name: metrics, port: 9090, targetPort: metrics }`, `{ port: metrics, interval: 30s, path: /metrics }`.
- Same render with values lacking `api.metricsPort` (reuse-values case): identical four lines.
- `--set api.metricsPort=9191`: all three API spots render 9191.

## 6. Docs, specs, CHANGELOG

- `api/specs.md`: flag list, listener paragraph, route list (removes `/metrics` from public routes) and login-privacy line are accurate.
- `docs/install.md`, `docs/dependencies.md`: accurate.
- `CHANGELOG.md`: `### Security hardening` created at the end of `## [Unreleased]`, just before `## [0.3.0-rc.1]`, one line in the required format: `- **api:** hardened Prometheus metrics serving with a dedicated in-cluster listener.`

## 7. Wording leak scan

Scanned the full diff and commit message for `F-NNN`, `Hxx`, `OD-`, `D13`, and exploit terms. No hits in added lines (the two matches, "expose" in `docs/install.md` and "unauthenticated" in `docs/security.md`, are unchanged context lines). Test names state controls. Comments describe the control, not the gap. Clean.

## 8. Optional notes (non-blocking)

1. `docs/security.md:795` says "`/metrics` on the dashboard host answers 404". With `web.enabled=true`, a browser navigation (Accept: text/html) to `/metrics` gets the SPA shell (200 HTML) from nginx's `$serve_shell` path; only non-HTML clients reach the API and get 404. If precision is wanted, change to: "so `/metrics` on the dashboard host never returns metrics." Not a correctness issue in the fix.
2. Release note / label: custom scrape configs that target `gameplane-api:80/metrics` (not the bundled ServiceMonitor) stop working. The brief's held notes already call for an Upgrade Notes line after merge ("The API now serves Prometheus metrics on a dedicated port (`api.metricsPort`, default 9090) instead of its main HTTP port. The bundled ServiceMonitor is updated; custom scrape configs must target the new port."). The maintainer may also want to decide whether this counts as `breaking` under CLAUDE.md rule 14. Held for the maintainer; not required in this branch by the task rules.
