# T045 edge chunk: independent verification (opus)

- **Date**: 2026-09-24
- **Input**: `audit/evidence/review-sentinel/notes.md`, 8 candidates. There is no `audit/held/review-sentinel.md`, and the notes record "held candidates: 0".

## Method

I tried to refute each candidate against branch `018-v0-3-release-readiness` at `3de03ab0`. On that branch, `sentinel/`, `operator/internal/`, `modules/` and `charts/` have no diff against `origin/master`, so the line numbers below also hold on master. I read all of `sentinel/main.go`, `sentinel/specs.md`, `sentinel/go.mod` and `operator/internal/controller/gameserver_sentinel.go`. I also read the reconcile order at `operator/internal/controller/gameserver_controller.go:352-390`, the test list in `sentinel/main_test.go` and the `GamePort` CRD validation (`operator/api/v1alpha1/gametemplate_types.go:850-889`). In `modules/`, I read the readiness probes of the three modules that use a handshake parser and the advertised ports of all 30. For the design side, I compared the code with `docs/roadmap.md:131-165`, `docs/architecture.md:284-294` and the original design, `docs/superpowers/specs/2026-07-24-wake-on-connect-design.md`. I searched `specs/*/`, including every `OPEN-DECISIONS.md`, for a decision that allows any of this behaviour and found none. `audit/findings.md` (F-001 to F-043) tracks nothing for sentinel, and no other chunk's notes repeat these candidates. `go build ./...` in `sentinel/` succeeds. I ran no tests or linters.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-sentinel-01 | kept | S3 | Confirmed by reading. The planning step and the delete happen in the same reconcile pass. When the game pod turns Ready, `planSentinel` returns an empty plan (`gameserver_sentinel.go:132-133`). Reconcile then deletes `<gs>-waker` (`gameserver_controller.go:360-369`, `gameserver_sentinel.go:168-169`). SIGTERM cancels ctx (`main.go:104-108`), and `proxyBidirectional` closes both sockets on `ctx.Done()` (`main.go:682-689`). The three modules with a handshake parser (`minecraft-java`, `terraria`, `tmodloader`) all have readiness `initialDelaySeconds: 30`, and the hold lasts at most 25 s. So Ready always fires after a handed-through session has started, and the session is always cut. The original design says held connections "survive the flip-back" (`docs/superpowers/specs/2026-07-24-wake-on-connect-design.md:80`), and the operator comment says the connection is "handed straight through instead of being dropped" (`gameserver_sentinel.go:90-94`). `docs/roadmap.md:160` ("proxy through until Ready") describes routing. It does not accept the disconnect. The workaround is to reconnect, so S3. |
| C-sentinel-02 | kept | S3 | Confirmed by reading. `run` takes the first async listener error and then waits in `wg.Wait()` (`main.go:418-421`). A healthy TCP accept loop exits only on ctx cancel (`:436-452`), so `run` blocks until SIGTERM. The error is not logged, which contradicts the doc comment at `:383-385`. The trigger is narrow but reachable. Five shipped modules mix TCP and UDP advertised ports (7-days-to-die, beammp, fivem, satisfactory, farming-simulator-25), and all of them bind fine. A custom template can advertise a UDP port the sentinel cannot bind: the CRD allows 1-65535, and the sentinel runs as UID 65532 with every capability dropped (`gameserver_sentinel.go:215-252`). A privileged port on a runtime that doesn't enable unprivileged low ports is enough. The result is a dead port with no log line. |
| C-sentinel-03 | kept | S4 | Confirmed. `main.go:110-114` only logs the error from `run` and returns, so the exit code is 0. The other fatal paths use `log.Fatalf` (`:90`, `:95`). The trigger is the same as C-sentinel-02, but on the synchronous TCP path (`:407-410`). The kubelet still restarts the container, so only the reported status is wrong. |
| C-sentinel-04 | kept | S4 | Confirmed. `proxyBidirectional` closes both conns (`:688-689`). The defers at `:604-608` and `:478-482` then close them again, and each logs the `net.ErrClosed` error. Every proxied session produces two error-looking log lines. |
| C-sentinel-05 | kept | S4 | Confirmed docs drift. `specs.md:143` says Hostport "Works as above". The operator deliberately gives Hostport no hold window (`gameserver_sentinel.go:95-101`, `:120-123`), and `docs/roadmap.md:158-160` documents that asymmetry. |
| C-sentinel-06 | kept | S4 | Confirmed docs drift. The heuristic key is `addr.String()`, which is `ip:port` (`main.go:754`), and packets in the cooldown are not counted (`:816-822`, reset at `:838`). `specs.md:19`, `:116`, `:149`, `:158` and `:162` say otherwise. |
| C-sentinel-07 | kept | S4 | All four statements are wrong. `PORTS_CONFIG` is marked "(required)", but an empty value is accepted, and `main_test.go:32` pins that. `:82` says "exponential backoff", but the interval is fixed (`main.go:635-639`, and `specs.md:125` itself). The status JSON at `:94` has unbalanced braces. `:235` says generic doesn't half-close, but it uses the same `proxyBidirectional`. |
| C-sentinel-08 | kept | S4 | Confirmed. `go.mod` has no controller-runtime. The six test names at `specs.md:284-285` don't exist. `test/e2e/tests/` doesn't exist. `CLAUDE.md` has no "Wake-on-connect" section, only table and map rows at `:73`, `:147` and `:259`. |

Rejected: 0.

### C-sentinel-01

**Location**: `operator/internal/controller/gameserver_sentinel.go:128-135` (the plan turns empty at game-pod Ready) and `:168-169` (the Deployment is deleted); `operator/internal/controller/gameserver_controller.go:360-390` (plan, sentinel reconcile and Service reconcile in one pass); `sentinel/main.go:101-108` (SIGTERM cancels ctx) and `:682-689` (the proxy closes both sides on ctx cancel). Context: `gameserver_sentinel.go:90-94` and `:306` (`PublishNotReadyAddresses: true`); `modules/terraria/template.yaml:93-99`.

**Repro / observation**:
1. On a cluster with the chart installed, create a GameServer from the `terraria` module (`tmodloader` and `minecraft-java` behave the same) with `spec.networking.expose: ClusterIP`, `spec.idle.enabled: true` and `spec.idle.wakeOnConnect: true`. NodePort and LoadBalancer behave the same as ClusterIP. Let the server go to sleep, then confirm that `<gs>-waker` is Ready and that the game Service selects `app.kubernetes.io/name=gameplane-waker`.
2. Connect a Terraria client, or the e2e Terraria bot, to the server address. The sentinel classifies the connection as a Join, patches `gameplane.local/idle-wake-requested` and starts polling `<gs>-game-direct`.
3. `<gs>-game-direct` publishes not-ready addresses, so the dial succeeds as soon as the game process binds its port. If that happens within the 25 s hold, the sentinel replays the handshake and the client enters the world through the proxy.
4. Run `kubectl get pods,deploy -n <ns> -w`. Readiness cannot pass until 30 s after the game container starts. When it passes, the operator deletes `<gs>-waker`. `kubectl logs` for the sentinel pod ends with `received signal terminated, shutting down`, and the client is disconnected at that moment.
5. To check by reading on master: `planSentinel` returns `sentinelPlan{}` when `gameReady` (`gameserver_sentinel.go:132-133`). `reconcileSentinel(want=false)` calls `deleteSentinel` (`:168-169`). The sentinel's signal handler calls `cancel()`, and `proxyBidirectional` takes the `ctx.Done()` branch and closes both conns without draining.

**Expected**: A connection that the sentinel hands through survives the switch back to the game pod. The design says "Connections already held in the proxy survive the flip-back" (`docs/superpowers/specs/2026-07-24-wake-on-connect-design.md:80`). The operator comment says the connection is "handed straight through instead of being dropped mid-wake" (`gameserver_sentinel.go:90-94`). `sentinel/specs.md:233` (invariant 3) says the proxy waits for both directions to finish.

**Actual**: A handed-through session lasts only until the game pod reports Ready. The sentinel pod is then deleted, and the player who woke the server is disconnected and must reconnect.

### C-sentinel-02

**Location**: `sentinel/main.go:418-421`. The errors come from `:447-451` (TCP accept), `:709-713` (UDP listen) and `:744-748` (UDP read). The contract is at `:383-385`.

**Repro / observation**:
1. Read `run` on master. After it starts the listeners, `select` takes the first error from `errCh` and calls `wg.Wait()` before it returns (`:419-421`).
2. Read `serveTCP` (`:435-466`) and `serveUDP` (`:705-760`). A healthy listener goroutine returns only when ctx is cancelled or its own socket fails. `run` has no cancel function of its own to stop the other listeners.
3. Take `PORTS_CONFIG=7777:TCP:generic,<P>:UDP:generic`, where the sentinel cannot bind UDP `<P>`. `serveUDP` sends `listen udp :<P>: …` and returns. `run` then blocks in `wg.Wait()` on the TCP goroutine until SIGTERM, and nothing is logged.
4. Live trigger: a custom GameTemplate that advertises a TCP port of 1024 or higher and a UDP port below 1024, on a node whose container runtime leaves `net.ipv4.ip_unprivileged_port_start` at 1024. If the TCP port were privileged too, the synchronous TCP listen would fail first and take the C-sentinel-03 path instead. The sentinel runs as UID 65532 with all capabilities dropped. Its pod stays Running and Ready, and the game Service stays routed to it. The log shows only the start line, and UDP traffic on that port never wakes the server. No shipped module triggers this, because all advertised ports are 2001 or higher.

**Expected**: `run` "blocks until ctx is cancelled or a listener reports a fatal error" (`:383-385`). A fatal listener error is returned or at least logged, so the dead port is visible.

**Actual**: While any other listener is healthy, the error is held silently until shutdown. The failed port is not served, and nothing is logged.

### C-sentinel-03

**Location**: `sentinel/main.go:110-114`, reached from `:407-410`.

**Repro / observation**:
1. Read `main` on master. The error from `run(ctx, cfg, w)` goes only to `log.Printf("sentinel exiting: %v", err)`. `main` then returns, so the exit status is 0.
2. Live: use a custom GameTemplate that advertises a TCP port the sentinel cannot bind, with the same privileged-port trigger as C-sentinel-02. Put the server to sleep with wake-on-connect armed. `kubectl get pod -l app.kubernetes.io/name=gameplane-waker -o jsonpath='{.items[0].status.containerStatuses[0].lastState.terminated}'` shows `exitCode: 0` and `reason: Completed`, and the log shows `sentinel exiting: listen tcp :N: …`.

**Expected**: A fatal startup error exits non-zero, as the `config:` and `kubernetes client:` paths do (`:90`, `:95`), so the container reports `Error`.

**Actual**: The process exits 0, and the pod shows `Completed` while it restart-loops.

### C-sentinel-04

**Location**: `sentinel/main.go:688-689` (the first close), then `:604-608` and `:478-482` (the second close, which is logged).

**Repro / observation**:
1. Read `handleJoin` and `handleTCPConnection` on master. After `proxyBidirectional` returns, both conns are already closed (`:688-689`). The deferred `upstream.Close()` (`:605`) and `conn.Close()` (`:479`) then return `net.ErrClosed`, and each error is logged.
2. Live: wake a sleeping Minecraft or Terraria server with a join that is handed through (see C-sentinel-01), and let the session end. `kubectl logs deploy/<gs>-waker` shows `close upstream connection: close tcp …: use of closed network connection` and `close connection: close tcp …: use of closed network connection`.

**Expected**: A close error is logged only when a close really fails.

**Actual**: Every proxied session, including one cut by shutdown, logs two error lines.

### C-sentinel-05

**Location**: `sentinel/specs.md:143`; `operator/internal/controller/gameserver_sentinel.go:95-101` and `:120-123`.

**Repro / observation**:
1. Read `specs.md:143`: Hostport "Works as above". The hold-and-poll and proxy path it refers to is described at `:122-134`.
2. Read `gameserver_sentinel.go:95-101` ("There is no hold-open window in this mode") and `:120-123`. With `Expose == "Hostport"`, `planSentinel` returns an empty plan as soon as the server is awake, so the sentinel is deleted when the wake is decided.
3. On SIGTERM, `waitForUpstream` returns the ctx error, and `handleJoin` bounces the client (`main.go:597-602`).

**Expected**: `specs.md` describes the Hostport asymmetry, as `docs/roadmap.md:158-160` does: on wake, held joins are bounced and never proxied.

**Actual**: `specs.md` says Hostport behaves like the Service-backed modes.

### C-sentinel-06

**Location**: `sentinel/main.go:754`, `:816-822` and `:838`; `sentinel/specs.md:19`, `:116`, `:149`, `:158` and `:162`.

**Repro / observation**:
1. `main.go:754` passes `addr.String()` to `shouldWake`. For a `*net.UDPAddr` that string is `ip:port`, so two sockets on one host count as two sources.
2. `main.go:816-822` returns `false` during the cooldown before the packet is appended, and `:838` set `st.packets` to nil when the wake fired.
3. Compare with `specs.md:19` ("Track UDP sources by IP"), `:116` ("extract the remote IP address"), `:149` ("per source IP"), `:162` ("distinct IPs") and `:158` ("Additional packets … within the cooldown are counted").

**Expected**: `specs.md` matches how sources are keyed and what happens in the cooldown.

**Actual**: Sources are keyed by `ip:port`, and packets are not counted during the cooldown.

### C-sentinel-07

**Location**: `sentinel/specs.md:51`, `:82`, `:94` and `:235`; `sentinel/main.go:232-235`, `:635-639`, `:66`, `:560-569`, `:668` and `:673`; `sentinel/main_test.go:32`.

**Repro / observation**:
1. `specs.md:51` marks `PORTS_CONFIG` "(required)". `parsePortsConfig("")` returns `nil, nil` (`main.go:232-235`), `loadConfig` accepts it, and `TestParsePortsConfig` pins the empty case as a success (`main_test.go:32`).
2. `specs.md:82` says "exponential backoff / polling". `waitForUpstream` waits a fixed `time.After(pollInterval)` (`main.go:635-639`), and `specs.md:125` itself says "fixed interval".
3. The example JSON at `specs.md:94` has four `{` and three `}`, and it nests `description` inside `players`. The code sends `…"players":{"max":0,"online":0,"sample":[]},"description":{…}}` (`main.go:66`).
4. `specs.md:235` says generic "doesn't half-close". Generic TCP goes through `handleJoin` → `proxyBidirectional` (`main.go:560-569`), which calls `closeWrite` on both sides (`:668`, `:673`).

**Expected**: `specs.md` states what the code does.

**Actual**: All four statements contradict the code.

### C-sentinel-08

**Location**: `sentinel/specs.md:5`, `:284-285`, `:311` and `:315`.

**Repro / observation**:
1. `sentinel/go.mod` requires only `gameproto`, `k8s.io/apimachinery` and `k8s.io/client-go`, with no controller-runtime. `main.go:293-296` says the dynamic client is used so the module "doesn't need controller-runtime".
2. `grep -n '^func Test' sentinel/main_test.go` lists no `TestHandleRegistryProtocol*`, `TestParsePortsConfigUnknownProtocol` or `TestParsePortsConfigValidRegisteredProtocols`. The registry tests are `TestRegistryProtocolDispatch*`, `TestParsePortsConfigRejectsUnknownProtocol` and `TestRegistryProtocolReplayConsumedBytes` (`main_test.go:1327-1529`).
3. `ls test/e2e/tests` fails. The bot tests are `test/e2e/*_bot_e2e_test.go`, and the wake tests are in `test/e2e/wake_on_connect_e2e_test.go`.
4. `grep -n -i 'wake-on-connect' CLAUDE.md` matches only the repo-map line and two table rows (`:73`, `:147`, `:259`). There is no section with that name.

**Expected**: Every dependency, test and reference named in `specs.md` exists.

**Actual**: None of these four references resolves.
