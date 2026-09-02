---
name: e2e-test-authoring
description: Use when adding or modifying a test in test/e2e/ — encodes bucket registration, parallelism rules, and the login rate-limit budget.
---

# E2E Test Authoring

## Procedure

### 1. Choose a bucket and verify availability

The e2e suite is divided into buckets by **login pressure**, not feature area. Every new test MUST be added to exactly one bucket in `test/e2e/buckets.sh` or the CI job `e2e bucket coverage` will fail.

**Current buckets** (from `bucket_names()` in `buckets.sh`):
- `operator`: zero logins, runs with `t.Parallel()`
- `api-auth`: OIDC + password + session lifecycle (ceiling: ~8 admin logins per job)
- `api-roles`: custom roles + role mappings (ceiling: ~5 admin logins)
- `api-rbac`: permission matrix + scope binding (no login budget constraints documented)
- `api-agent`: agent endpoint proxying (agent in-pod, shared restic repo)
- `api-mods`: module installation + confinement (shared oras-push Job via `ociPushMu`)
- `ratelimit`: deliberately exhausts login limiter (runs non-parallel, last, in its own `go test` invocation)
- `bot-fast`: game server bots with modest resource footprint (runs parallel)
- `bot-heavy`: excluded from default CI — games ≥5Gi storage or heavy downloads (documented in comments)
- `multicluster`: requires dual-cluster setup (separate CI job)
- `upgrade`: requires previous-release chart (separate CI job)

**Login budget per bucket**: The API rate limiter is per-IP (5/min, burst 10) + per-username (3/min, burst 6). All tests in a job share one client IP through the kubectl port-forward, so admin logins — not CPU — bound wall-clock time. Keep api-auth ≤ ~8 logins, api-roles ≤ ~5. Check `buckets.sh` comments for current budget positions before committing.

### 2. Name the test and resources consistently

**Test function name**: `TestBucket_Feature` (e.g., `TestAPI_AgentFilesRoundTrip`). Match the bucket in the name (`TestAPI_*` for api buckets, `TestGameServer_*` for operator, etc.).

**Resource names**: Prefix all K8s objects (GameServer, GameTemplate, Service, ConfigMap, etc.) with `e2e-<name>-<kind>` to ensure per-test uniqueness and aid debugging. Examples:
```go
ns := "gameplane-games"
tmpl := "e2e-my-feature-tmpl"
gs := "e2e-my-feature-gs"
job := "e2e-my-feature-backup-job"
```

### 3. Declare parallelism and shared-state guards

**Default**: Add `t.Parallel()` at the start of your test. Most tests are parallel-safe if they use unique resource names (rule 2 above).

**Non-parallel cases**:
- Tests observing raw login status codes (e.g., testing 429 responses) — `ratelimit` bucket only, deliberately serial.
- Other true shared-state cases (rare) — leave `t.Parallel()` out.

**Shared-state guards** (use these if your test fits the pattern):

**Module/bundle tests**: If your test performs OCI bundle operations (pushing modules via `oras`), guard the fixed-name `oras-push` Job with the global `ociPushMu` mutex:
```go
ociPushMu.Lock()
defer ociPushMu.Unlock()
// ... perform OCI push ...
```

**Backup/restore tests**: If your test uses restic backups, call `ensureResticRepo(t)` at setup. It idempotently initializes the shared restic repository:
```go
ensureResticRepo(t)
// ... perform backup/restore ...
```

Do not run multiple backup tests in parallel against the same restic repo without this guard.

### 4. Build tag and file location

- **Build tag**: Add `//go:build e2e` at the top of the file.
- **Location**: `test/e2e/<name>_e2e_test.go` (e.g., `api_agent_e2e_test.go`, `operator_gameserver_e2e_test.go`).
- **Package**: Always `package e2e`.
- **Lint**: CI runs `golangci-lint` with `--build-tags=e2e` so your test is analyzed as part of the linting suite.

### 5. Add the test to buckets.sh

After writing the test, edit `test/e2e/buckets.sh` and add the test name to the appropriate bucket function. **Example**: adding `TestAPI_MyNewFeature` to `api-agent`:

```bash
bucket_api_agent() { cat <<'EOF'
TestAPI_AgentFilesRoundTrip
TestAPI_AgentPlayers
TestAPI_AgentUnreachable
TestAPI_ConsolePTYRoundTrip
TestAPI_LogsTailWS
TestAPI_LifecycleStartStop
TestAPI_LifecycleRestart
TestAPI_MyNewFeature
EOF
}
```

Run `test/e2e/buckets.sh verify` locally to confirm your addition is valid (no duplicates, test exists, no unlisted tests).

### 6. Do not run the suite locally (Rule 8)

**Never** run `make test-e2e`, `make test-e2e-keep`, or `go test -tags=e2e ./...` locally. CI is the source of truth. Instead:
1. Write the code.
2. Commit with a conventional prefix (`test: add e2e test for X`), signed.
3. Push to a feature branch.
4. CI runs the full suite on GitHub Actions — watch via `gh run view`.
5. Fix failures with follow-up commits; do not retry locally.

A quick **compile check** (`tsc --noEmit` in web, `go build ./...` in test/e2e) is fine to avoid obviously broken pushes — that is not a test run.

### 7. Verify before finishing

After writing your test, confirm:

- [ ] Test function name matches bucket pattern (e.g., `TestAPI_*` for api bucket, `TestGameServer_*` for operator).
- [ ] All K8s resource names prefixed with `e2e-<feature>-<kind>`.
- [ ] `t.Parallel()` present (unless a legitimate non-parallel reason in the code).
- [ ] If using modules: `ociPushMu.Lock()/Unlock()` guards OCI operations.
- [ ] If using backups: `ensureResticRepo(t)` called at setup.
- [ ] `//go:build e2e` tag at top of file.
- [ ] File in `test/e2e/<name>_e2e_test.go`.
- [ ] Test name added to bucket in `buckets.sh`.
- [ ] `test/e2e/buckets.sh verify` exits cleanly.
- [ ] Commit signed (`git commit -s`), conventional-commit prefix.
- [ ] Pushed to feature branch (never directly to `master`, per rule 12 in CLAUDE.md).

### 8. Handling bucket ceiling overflows

If your test requires admin logins and the target bucket is at its ceiling (e.g., api-auth at 8):

1. Check `buckets.sh` for comments naming the per-bucket budget.
2. Either:
   - Reduce logins in your test (defer auth to a later batch, use OIDC if available, etc.).
   - Move to a different bucket with headroom.
   - Add a comment in `buckets.sh` noting the overflow and adjusting the ceiling comment if appropriate (rare; needs maintainer sign-off).

3. Update the comment in `buckets.sh` for the affected bucket to reflect new position.

### 9. For operator-only tests

Operator tests (no API logins) have no budget constraints and run with full parallelism. Verify:
- Test name in `bucket_operator()` in `buckets.sh`.
- `t.Parallel()` present.
- Unique resource names per test.

---

## Reference

- **buckets.sh**: `/home/valgul/project/Gameplane/test/e2e/buckets.sh` — source of truth for bucket membership and login budgets.
- **CLAUDE.md rule 8**: "Never run test/lint locally — CI is the source of truth."
- **CLAUDE.md test tiers**: `make test-e2e` (10–20 min) is the CI e2e tier; `make test-integration` is operator/api envtest.
- **Lint**: `make lint` includes a check that every Go module in `go.work` plus `web/` has a non-empty `specs.md`.

---

## Common helpers (from existing tests)

- `envInstance.BootstrapAdmin(t, username, password)` — create admin user.
- `envInstance.APIClient(t, username, password)` — log in, return *APIClient with CSRF token and BaseURL.
- `applyBusyboxTemplate(t, name)` and `applyBusyboxGameServer(t, ns, name, template)` — shorthand for test setup.
- `requireAgentReady(t, ns, gsName)` — wait for agent sidecar to reach Ready without restarts (90s timeout).
- `waitAgentReachable(t, cli, gs)` — poll agent `/players` endpoint through API proxy (30s timeout).
- `waitPVCBound(t, ns, pvcName, timeout)` — wait for PVC to bind.
- `envInstance.Eventually(t, duration, func())` — retry predicate until true or timeout.
- `envInstance.Kubectl(ctx, args...)` — run kubectl, return stdout + stderr.
