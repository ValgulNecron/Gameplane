# Review: svcutil

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `svcutil/specs.md`; package doc `svcutil/env.go:1-4`; `README.md:147,183`; `CLAUDE.md:76,150,253`; `docs/agent-architecture.md:31`; `docs/architecture.md` (searched for the `§ "svcutil"` section that `specs.md:102` cites)

## Scope reviewed

Read in full: `svcutil/env.go`, `svcutil/env_test.go`, `svcutil/server.go`, `svcutil/server_test.go`, `svcutil/specs.md`, `svcutil/go.mod`, `svcutil/.testcoverage.yml`. That is every file in the module.

To check the spec's consumer list I also read: `capture-sidecar/go.mod`, `capture-sidecar/cmd/main.go:75-100`, `capture-sidecar/specs.md:58,88`, `api/cmd/main.go:530-590`, `agent/cmd/main.go:245-280`, the `envOr` definitions in `audit-syslog-bridge/main.go:79` and `telemetry-receiver/main.go:57`, `specs/002-nuclear-option-ip-pool/tasks.md:252`, and a repo-wide grep for `svcutil` across `*.go`, `go.mod`, `*.md`, `Makefile`, `*.yml`/`*.yaml` and Dockerfiles.

Nothing was skipped.

## Method

- Compared each exported function and each invariant in `specs.md` with the code.
- Grepped the whole repo for Go imports of `github.com/ValgulNecron/gameplane/svcutil` and for calls to `svcutil.<Name>` to check the consumer claims.
- Counted the table cases in `env_test.go` and compared them with the spec's test description.
- Did not run tests or linters (Rule 8).

## Observations (no finding)

- `Or`, `OrInt` and `ParseLogLevel` behave as `specs.md:44-48` and invariants 1-2 describe: `os.LookupEnv` semantics, the fallback on unparseable ints, and case-insensitive `debug`/`warn`/`error` with `info` as the default.
- `RunHTTP` (`server.go:19-37`) does what invariants 3-5 say. `http.ErrServerClosed` from the listen goroutine is filtered out (`server.go:23`), so a Shutdown called elsewhere yields `nil`. Shutdown uses `context.WithoutCancel(ctx)` plus `shutdownTimeout` (`server.go:33`), so the deadline isn't already expired when shutdown begins. `srv.Shutdown`'s deadline error comes back unwrapped, so `errors.Is(err, context.DeadlineExceeded)` works for callers.
- With a context that is already cancelled, the `select` almost always takes `ctx.Done()`, so a concurrent bind error from the goroutine is dropped. `specs.md:61` describes exactly that ("shutdown proceeds normally"), so it is not a finding.
- `go.mod` has no requirements, which matches "stdlib only". `.testcoverage.yml` sets 90%, which matches `specs.md:85` and `CLAUDE.md:150`.

## Candidate findings

### C-svcutil-01: No Go module imports svcutil. The spec, package doc and README name six consumers, and each of them has its own copy of the helpers

- **Location**: `svcutil/specs.md:101` (Consumers), `svcutil/specs.md:9`, `svcutil/specs.md:103`, `svcutil/env.go:3-4`; related: `capture-sidecar/go.mod:11-13`, `capture-sidecar/specs.md:58,88`
- **Category**: docs-drift (also dead-code: the module ships, gets linted and is coverage-gated, but no binary uses it)
- **Suggested severity**: S4
- **Observation / repro**:
  1. `grep -rln 'gameplane/svcutil' --include='*.go' .` returns no files. `grep -rn 'svcutil\.[A-Z]' --include='*.go'` outside `svcutil/` finds one hit, a comment at `capture-sidecar/cmd/main.go:85` saying `svcutil.RunHTTP` "must not be used here".
  2. No `go.mod` has a `require` for svcutil. `capture-sidecar/go.mod:11-13` has only a leftover `replace github.com/ValgulNecron/gameplane/svcutil => ../svcutil` with no matching `require`, and `capture-sidecar/Dockerfile:4-7` still copies `svcutil/` into the build for it.
  3. The consumers the spec names keep their own copies: `api/cmd/main.go:533` (`envOr`), `:548` (`parseLogLevel`), `:564` (`envOrInt`); `agent/cmd/main.go:248` (`envOr`), `:257` (`parseLogLevel`); `audit-syslog-bridge/main.go:79` and `telemetry-receiver/main.go:57` (`envOr`). `specs/002-nuclear-option-ip-pool/tasks.md:252` confirms it: "`api` does **not** import `svcutil` today … this feature must not make it the first".
  4. The spec says the opposite. `specs.md:101`: "**Consumers:** `operator/cmd/main.go`, `api/cmd/main.go`, `agent/cmd/main.go`, `audit-syslog-bridge/cmd/main.go`, `telemetry-receiver/cmd/main.go`, `capture-sidecar/cmd/main.go` — each uses `RunHTTP` for graceful startup/shutdown and environment parsing (Or, OrInt, ParseLogLevel)." `specs.md:103` says it is "imported by operator, api, agent, and sidecar components". `env.go:3-4` says it is "Used across operator, api, agent, audit-syslog-bridge, and telemetry-receiver". `capture-sidecar/specs.md:88` lists `svcutil v0.0.0` as a dependency. Two of the cited paths don't exist either: `audit-syslog-bridge/cmd/main.go` and `telemetry-receiver/cmd/main.go` (both binaries have `main.go` at the module root).
- **Expected**: The spec's purpose (`specs.md:9`: "Reduces code duplication and enforces consistent startup and shutdown behavior across operator, api, agent, audit-syslog-bridge, and telemetry-receiver") holds: the listed binaries import svcutil. Otherwise the spec, package doc, README and capture-sidecar docs say the module has no consumers.
- **Actual**: Nothing imports svcutil. The duplication the module exists to remove is still in five binaries, and the docs describe an adoption that never happened.

### C-svcutil-02: specs.md cites an architecture section that doesn't exist, miscounts the tests, and says a test checks behaviour it never exercises

- **Location**: `svcutil/specs.md:31`, `svcutil/specs.md:83` vs `:88`, `svcutil/specs.md:102`, `svcutil/specs.md:21`; `svcutil/server_test.go:148-193`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:102`: "**Architecture:** `docs/architecture.md` § "svcutil"". `grep -n -i svcutil docs/architecture.md` returns nothing, so the section doesn't exist.
  2. `specs.md:31`: "`env_test.go` … (13 subtests)". The tables have 3 (`TestOr`), 6 (`TestOrInt`) and 12 (`TestParseLogLevel`) cases, so 21 subtests (`env_test.go:17-41, 65-116, 138-197`).
  3. `specs.md:83`: "`TestRunHTTPShutdownTimeout`: shutdown timeout mechanism bounds `RunHTTP`'s wait time." The test (`server_test.go:148-193`) installs a 30-second `blockingHandler` but never sends it a request. No connection is active at `cancel()`, so `srv.Shutdown` returns immediately whatever `shutdownTimeout` is, and the test would pass if the timeout were ignored. That contradicts `specs.md:88`, which says correctly that "no test currently holds a request open past the timeout".
  4. `specs.md:21`: "Does not provide configuration file parsing; only environment-variable and command-line inputs." The package parses no command-line input.
- **Expected**: The spec's references, counts and test descriptions match the tree.
- **Actual**: As listed above.

## Questions (not findings)

- Should svcutil be adopted by the binaries its spec names, or removed together with the leftover `capture-sidecar` replace directive and Dockerfile copy? `specs/002-nuclear-option-ip-pool/tasks.md:252` tells new work not to import it, which suggests removal. That choice belongs to the maintainer.
