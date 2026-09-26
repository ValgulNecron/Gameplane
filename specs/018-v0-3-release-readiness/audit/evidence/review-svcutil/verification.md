# T045 libs chunk: independent verification (opus)

Method: I tried to refute the two candidates in `notes.md` (reviewer: opus) against master `13a859ff`. None of the cited files differ between this branch and master. I read every file in `svcutil/` (`env.go`, `env_test.go`, `server.go`, `server_test.go`, `specs.md`, `go.mod`). I grepped the whole workspace for Go imports of `github.com/ValgulNecron/gameplane/svcutil`, for `svcutil.` call sites, and for `svcutil` in every `go.mod`. I read the helper copies the spec's consumers keep (`api/cmd/main.go:533-590`, `agent/cmd/main.go:248-270`, `audit-syslog-bridge/main.go:79`, `telemetry-receiver/main.go:57`) and the leftover wiring in `capture-sidecar/go.mod:11-13`, `capture-sidecar/Dockerfile:4-7` and `capture-sidecar/specs.md:58,88`. I counted the table cases in `env_test.go` with a script. I checked `docs/architecture.md` for a svcutil section and checked `audit/findings.md` for existing entries. F-035 and F-036 name svcutil, but only as missing from `docs/dependencies.md` and `docs/contributing.md`, so neither candidate is a duplicate. I ran no test or lint suite. There is no held file for svcutil (`audit/held/verification-svcutil.md` records that).

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-svcutil-01 | kept | S4 | Confirmed. No Go file in the workspace imports svcutil, and no `go.mod` requires it. The one `svcutil.` hit outside the module is a comment at `capture-sidecar/cmd/main.go:85`. None of the six consumers that `specs.md:101` names imports it, and four of them (api, agent, audit-syslog-bridge, telemetry-receiver) keep their own `envOr`/`parseLogLevel`/`envOrInt`. `capture-sidecar` still carries a `replace` directive with no matching `require`, and a Dockerfile `COPY svcutil/`. Two of the consumer paths the spec cites (`audit-syslog-bridge/cmd/main.go`, `telemetry-receiver/cmd/main.go`) don't exist. Only documentation and dead wiring are affected, so S4. Whether svcutil should be adopted or removed is the maintainer's call (see the Questions section in `notes.md`). |
| C-svcutil-02 | kept | S4 | Confirmed. `docs/architecture.md` has no svcutil section. `env_test.go` has 21 table subtests (3 + 6 + 12), not 13. `TestRunHTTPShutdownTimeout` never sends a request, so it would pass even if the timeout were ignored, which contradicts the spec's own "Uncovered paths" note. The package parses no command-line input. |

### C-svcutil-01

**Location:** `svcutil/specs.md:9` (Purpose), `:101` (Consumers), `:103` (Go workspace); `svcutil/env.go:3-4` (package doc); `capture-sidecar/go.mod:11-13`, `capture-sidecar/Dockerfile:4-7`, `capture-sidecar/specs.md:58,88`. (`README.md:147,183` describe what svcutil does and make no claim about consumers, so they are accurate.)

**Repro / observation:**
1. `grep -rln 'gameplane/svcutil' --include='*.go' .` returns no files.
2. `grep -rn 'svcutil\.[A-Z]' --include='*.go' . | grep -v '^./svcutil/'` returns one line, a comment at `capture-sidecar/cmd/main.go:85` explaining why `svcutil.RunHTTP` "must not be used here".
3. `grep -rn svcutil --include=go.mod .` finds only `svcutil/go.mod` itself and the `replace … => ../svcutil` at `capture-sidecar/go.mod:11-13`, which has no matching `require`. `capture-sidecar/Dockerfile:4-7` still copies `svcutil/` into the build context "so `go mod download` and the build resolve it".
4. The named consumers keep their own copies: `api/cmd/main.go:533` (`envOr`), `:548` (`parseLogLevel`), `:564` (`envOrInt`); `agent/cmd/main.go:248` (`envOr`), `:257` (`parseLogLevel`); `audit-syslog-bridge/main.go:79` and `telemetry-receiver/main.go:57` (`envOr`). `operator/cmd/main.go` defines none of these and doesn't import svcutil either.
5. `ls audit-syslog-bridge/cmd telemetry-receiver/cmd` fails for both. Their entry points are `main.go` at the module root.
6. The docs say the opposite. `specs.md:101`: "each uses `RunHTTP` for graceful startup/shutdown and environment parsing". `specs.md:103`: "imported by operator, api, agent, and sidecar components". `env.go:3-4`: "Used across operator, api, agent, audit-syslog-bridge, and telemetry-receiver". `capture-sidecar/specs.md:88` lists `svcutil v0.0.0` as a dependency.

**Expected:** The docs match the tree. Either the listed binaries import svcutil (the purpose in `specs.md:9`), or `specs.md`, the package doc and `capture-sidecar/specs.md` say the module has no consumers yet, and the stale `replace` and Dockerfile `COPY` in `capture-sidecar` are removed.

**Actual:** No binary imports svcutil. Four binaries keep their own copies of the helpers, and three documents (`svcutil/specs.md`, the package doc and `capture-sidecar/specs.md`) describe an adoption that never happened.

### C-svcutil-02

**Location:** `svcutil/specs.md:21`, `:31`, `:83`, `:102`; `svcutil/server_test.go:148-193`.

**Repro / observation:**
1. `specs.md:102` cites `docs/architecture.md` § "svcutil". `grep -n -i svcutil docs/architecture.md` returns nothing.
2. `specs.md:31` says `env_test.go` has "13 subtests". The three tables run their cases through `t.Run`: `TestOr` has 3 cases, `TestOrInt` 6 and `TestParseLogLevel` 12, so 21 in total (`env_test.go:9-55`, `:57-130`, `:132-197`).
3. `specs.md:83` says `TestRunHTTPShutdownTimeout` shows that the "shutdown timeout mechanism bounds `RunHTTP`'s wait time". The test (`server_test.go:148-193`) installs a 30-second handler but never sends a request. No connection is active when `cancel()` runs, so `srv.Shutdown` returns at once whatever the timeout is, and the test would pass if `RunHTTP` ignored `shutdownTimeout`. `specs.md:88` says correctly that "no test currently holds a request open past the timeout", which contradicts `:83`.
4. `specs.md:21` says the package handles "only environment-variable and command-line inputs". Nothing in `env.go` or `server.go` reads command-line input.

**Expected:** The spec's cross-reference, test count, test descriptions and scope statement match the tree.

**Actual:** As listed above: a dead section reference, a wrong subtest count, a test described as covering behaviour it never exercises, and a command-line claim with no code behind it.
