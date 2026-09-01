# svcutil — Specification

**Status:** Stable  
**Module / command:** `github.com/ValgulNecron/gameplane/svcutil`  
**Dependencies:** stdlib only (Go 1.26+)

## Purpose

Shared stdlib-only service helpers for environment parsing and graceful HTTP server shutdown. Reduces code duplication and enforces consistent startup and shutdown behavior across operator, api, agent, audit-syslog-bridge, and telemetry-receiver. All functions degrade gracefully on invalid inputs; no startup crashes from bad configuration.

## Responsibilities

- Parse environment variables with fallback semantics, distinguishing unset from empty-but-set.
- Parse log-level strings to `slog.Level` enums with case-insensitive recognition and graceful default on unknown values.
- Parse integer-valued environment variables with non-crashing fallback to defaults.
- Start HTTP servers and manage graceful shutdown on context cancellation with bounded timeout.
- Map shutdown success (`http.ErrServerClosed`) to nil so callers see clean completion.

## Non-goals / boundaries

- Does not provide configuration file parsing; only environment-variable and command-line inputs.
- Does not enforce configuration schema or validation; callers own business-logic constraints.
- Does not handle signal forwarding; callers that need signal → context.Done bridging use `signal.NotifyContext`.
- Does not manage connection pooling or middleware; `RunHTTP` runs a stock `http.Server` as-is.

## Directory & package layout

```
svcutil/
├── env.go             # Or, OrInt, ParseLogLevel environment helpers
├── env_test.go        # Unit tests for environment parsing (13 subtests)
├── server.go          # RunHTTP graceful shutdown helper
├── server_test.go     # Unit tests for server lifecycle (5 functions)
├── go.mod             # Module declaration (stdlib-only)
└── .testcoverage.yml  # 90% coverage gate
```

Single package; no subdirectories or internal structure.

## External interface / contracts

### Exported Functions

| Function | Signature | Behavior |
|---|---|---|
| **`Or`** | `Or(key, fallback string) string` | Returns environment variable `key` if set (via `os.LookupEnv`); otherwise returns `fallback`. Distinguishes unset from empty-but-set: empty string in env returns `""`, not `fallback`. |
| **`OrInt`** | `OrInt(key string, def int) int` | Parses environment variable `key` as int via `strconv.Atoi`; returns `def` if unset or unparseable. Non-crashing: invalid values degrade to default rather than panicking. |
| **`ParseLogLevel`** | `ParseLogLevel(s string) slog.Level` | Maps case-insensitive log-level string to `slog.Level`. Recognized case-insensitively: debug, warn, error; anything else (including "info" or empty) defaults to `slog.LevelInfo`. Non-crashing fallback semantics. |
| **`RunHTTP`** | `RunHTTP(ctx context.Context, srv *http.Server, shutdownTimeout time.Duration) error` | Starts HTTP server; on context cancellation, performs graceful shutdown with deadline bounded by `shutdownTimeout`. Maps `http.ErrServerClosed` to `nil`. Returns listen error immediately or shutdown result on cancellation. |

## Key invariants

1. **Graceful fallback.** `OrInt` and `ParseLogLevel` never panic on invalid input; they degrade to sensible defaults (`def` for OrInt, `slog.LevelInfo` for ParseLogLevel). This is a guarantee: invalid manifest values must not crash the service at startup.

2. **Empty-string semantics.** `Or` distinguishes unset (`os.LookupEnv` returns `ok=false`) from empty-but-set (`os.LookupEnv` returns `v=""` with `ok=true`). Unset uses fallback; empty-but-set returns empty string (no fallback). This preserves user intent when an env var is intentionally cleared.

3. **Bounded shutdown.** `RunHTTP` calls `srv.Shutdown` with a deadline-bounded context derived from `shutdownTimeout`. If the timeout expires, the deadline context is cancelled, causing `srv.Shutdown` to return and the goroutine to exit promptly. Long-running in-flight requests are interrupted; no indefinite hangs.

4. **Success mapping.** `RunHTTP` explicitly maps `http.ErrServerClosed` (the success path from `srv.Shutdown`) to `nil`, so callers see nil on clean shutdown and a non-nil error only on failure.

5. **Context propagation.** `RunHTTP` selects on both listen errors (returned immediately) and context cancellation (triggers shutdown). If the context is pre-cancelled or becomes cancelled before the server listens, shutdown proceeds normally.

## Dependencies

**Internal:** None.  
**External:** stdlib only (`context`, `errors`, `log/slog`, `net/http`, `os`, `strconv`, `strings`, `time`). No external modules in `go.mod`.

## Security considerations

- **No secrets in env vars.** This package reads environment variables for configuration only; credentials or sensitive values must be injected via files or mounts, not environment variables (which are visible in process listings and pod describe).
- **Graceful defaults prevent misconfiguration crashes.** Services that fail to start due to invalid log levels or port numbers in the environment are difficult to debug in containerized environments. Non-crashing fallbacks reduce incident severity (service starts, logs at info level, operator can adjust).
- **Bounded shutdown prevents hanging.** A service hung in shutdown — waiting indefinitely for in-flight requests — can leave resources locked or cause deployment failures. `shutdownTimeout` bounds the wait; Kubernetes `terminationGracePeriodSeconds` (usually 30 seconds) provides the outermost bound.

## Testing & coverage

**Test structure:**
- Unit tests in `env_test.go` cover `Or`, `OrInt`, `ParseLogLevel` with table-driven cases for set/unset/invalid/empty/default scenarios.
- Unit tests in `server_test.go` cover `RunHTTP` with cases for clean shutdown, listen errors, pre-cancelled context, shutdown timeout, and `http.ErrServerClosed` mapping.

**Coverage gate:** 90% (`.testcoverage.yml`, total threshold).

**Uncovered paths:** The small 10% gap accounts for hard-to-trigger graceful-shutdown race conditions:
- Context timeout during `srv.Shutdown` (difficult to race reliably in tests).
- Platform-specific signal handling edge cases in goroutine scheduling.
- Listener errors that are difficult to synthesize without system-level faults.

These edge cases are acceptable because:
1. They occur only during service shutdown/restart, not during steady-state operation.
2. The outer Kubernetes `terminationGracePeriodSeconds` ensures eventual termination even if shutdown races occur.
3. Triggering them in a test would require flaky timing assumptions or mocking system calls.

**Key test cases:**
- **Environment parsing:** Unset, set, empty, invalid values; fallback semantics; case-insensitivity.
- **Server lifecycle:** Clean shutdown, listen error, pre-cancelled context, shutdown timeout, ErrServerClosed mapping.

## References

- **Consumers:** `operator/cmd/main.go`, `api/cmd/main.go`, `agent/cmd/main.go`, `audit-syslog-bridge/cmd/main.go`, `telemetry-receiver/cmd/main.go`, `capture-sidecar/cmd/main.go` — each uses `RunHTTP` for graceful startup/shutdown and environment parsing (Or, OrInt, ParseLogLevel).
- **Architecture:** `docs/architecture.md` § "svcutil" — shared helpers for consistent service lifecycle.
- **Go workspace:** `go.work` — svcutil linked as a workspace module, imported by operator, api, agent, and sidecar components.
