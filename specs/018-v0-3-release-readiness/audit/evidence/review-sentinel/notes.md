# Review: sentinel

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `sentinel/specs.md`; `docs/architecture.md` (lines 284-294); `docs/roadmap.md` ("Wake-on-connect for idle auto-sleep", lines 131-165); `docs/install.md` (lines 192-199); operator wiring in `operator/internal/controller/gameserver_sentinel.go`

held candidates: 0 (see OD-019)

## Scope reviewed

Read in full:
- `sentinel/main.go`
- `sentinel/specs.md`
- `sentinel/Dockerfile`, `sentinel/.testcoverage.yml`, `sentinel/go.mod` (require block)
- `operator/internal/controller/gameserver_sentinel.go` (the operator wiring: Deployment, RBAC, game-direct Service, `planSentinel`)

Read in part:
- `sentinel/main_test.go`: the test function list, lines 20-340 (config, parse and UDP heuristic tests), and lines 1100-1240 (`run`/`serveTCP` tests). I skipped the handler and proxy test bodies.
- `operator/internal/controller/gameserver_controller.go`: lines 340-376 (reconcile order), 1925-1958 (game container probes), 1545-1553 (default images)
- `operator/internal/controller/gameserver_idle.go`: the wake annotation constants and handling (grep only)
- `operator/api/v1alpha1/gametemplate_types.go`: lines 866-889 (`WakeProtocol` enum and default)
- `gameproto/registry.go`, `gameproto/demo.go`, `gameproto/minecraft.go` (lines 40-135, 290-340), `gameproto/terraria.go` (lines 235-290), for the classifier contract the sentinel relies on
- `modules/{minecraft-java,terraria,tmodloader}/template.yaml`: readiness probes and `wakeProtocol` (grep only)
- `test/e2e/wake_on_connect_e2e_test.go`: lines 200-260 (the login-wake assertions)
- `charts/gameplane/templates/networkpolicies.yaml`: the sentinel policy names and comments (grep only)

Not reviewed: the gameproto parsers beyond what the sentinel calls (another chunk covers them); the chart NetworkPolicy bodies.

Compile check: `go build ./...` in `sentinel/` succeeds.

## Method

I read `main.go` line by line against every behavioural statement in `specs.md`, then traced the operator side: how the sentinel Deployment is created and deleted (`planSentinel` → `reconcileSentinel`), which env it gets (`buildSentinelPortConfig`), the RBAC Role, and the `<gs>-game-direct` Service. For each spec statement I checked whether the code does what it says. I also traced every error path (listen, accept, read, patch, close) to where it ends up. No tests or linters were run.

## Observations (no finding)

- The RBAC Role grants exactly `get` and `patch` on `gameservers` with `resourceNames: [<gs>]` (`gameserver_sentinel.go:400-405`), as `specs.md:69-73` says.
- The wake patch is a JSON merge patch that sets only `gameplane.local/idle-wake-requested` (`main.go:359-372`). The constant matches `IdleWakeRequestedAnnotation` in `gameserver_idle.go:35`.
- Burst coalescing (`main.go:342-357`) behaves as `specs.md` invariant 6 describes.
- Startup validation of `wakeProtocol` against the gameproto registry (`main.go:262-267`) matches `specs.md:58` and `:102`.
- The TCP semaphore is shared across all TCP listeners (`main.go:387`, `:454-464`). One accepted connection can wait for a slot outside the semaphore, which is consistent with the code comment.
- The `*bufio.Reader` is reused for proxying (`main.go:511`, `:617`, `:667`), which preserves pipelined bytes as invariant 2 requires.
- Hostport shutdown: on SIGTERM, `waitForUpstream` returns the ctx error, so `handleJoin` sends the bounce (`main.go:597-602`). This matches the operator's Hostport design (`gameserver_sentinel.go:95-101`).
- The pod security context is non-root with all capabilities dropped and `allowPrivilegeEscalation: false` (`gameserver_sentinel.go:215-252`).
- The operator never sets `WAKE_DEADLINE`, `UDP_*`, `MAX_CONNECTIONS` or `WAKE_PATCH_INTERVAL`, so production always runs on the defaults in `main.go:153-160`.
- The e2e test `TestGameServer_WakeOnConnect_LoginWakes` checks only the annotation and the wake. It never checks that a held connection is handed through to the game (`test/e2e/wake_on_connect_e2e_test.go:203-254`).

## Candidate findings

### C-sentinel-01: The hand-through connection is killed when the operator deletes the sentinel at game-pod Ready
- **Location**: `sentinel/main.go:101-108` and `:683-689`; `operator/internal/controller/gameserver_sentinel.go:128-135` and `:168-169`
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. Use a GameServer with `idle.enabled`, `idle.wakeOnConnect` and Expose `ClusterIP`, `NodePort` or `LoadBalancer`, on the `terraria` module. Its readiness is `tcpSocket` on the game port with `initialDelaySeconds: 30` and `periodSeconds: 10` (`modules/terraria/template.yaml:93-99`). Let the server go to sleep.
  2. A player connects. The sentinel classifies Join, patches the wake annotation, and polls `<gs>-game-direct` every 250 ms. That Service has `PublishNotReadyAddresses: true` (`gameserver_sentinel.go:306`), so the dial succeeds as soon as the game process binds its port, before the pod is Ready. The sentinel replays the handshake and starts `proxyBidirectional`.
  3. The readiness probe passes and `ss.Status.ReadyReplicas` becomes 1. `planSentinel` returns `wantSentinel=false` (`gameserver_sentinel.go:132-133`), and `reconcileSentinel` deletes the `<gs>-waker` Deployment (`:168-169`).
  4. The sentinel pod receives SIGTERM. `main` cancels ctx (`main.go:104-108`), and `proxyBidirectional` takes the `ctx.Done()` branch, which closes both the upstream and downstream connections immediately (`main.go:683-689`). There is no drain.
  5. A player who reconnects in the window between the game port binding and Ready is proxied the same way and then dropped as well, because the game Service keeps routing to the sentinel until Ready.
- **Expected**: `gameserver_sentinel.go:90-94` says: "the sentinel … is kept alive — and the Service kept routed to it — until the game pod itself reports Ready. A player connection the sentinel is holding open can then be handed straight through instead of being dropped mid-wake." `sentinel/specs.md:233` (invariant 3) says: "The sentinel doesn't close the downstream connection until both the upstream→downstream and downstream→upstream copies have reached EOF. If it closed early, data in flight would be dropped."
- **Actual**: A hand-through session lasts only until the sentinel pod is torn down, and that happens right after the game pod reports Ready. The player who woke the server is disconnected shortly after joining.
- **Note for the verifier**: `docs/roadmap.md:161` says "Service-backed modes (ClusterIP/NodePort/LoadBalancer) proxy through until Ready." That wording may be an intended description of this limit. If it is, the finding becomes docs drift against `gameserver_sentinel.go:93-94` and `specs.md:233`.

### C-sentinel-02: A listener error on one port while other ports are open is swallowed, and `run` blocks
- **Location**: `sentinel/main.go:418-421` (together with `:447-451`, `:709-713`, `:744-748`)
- **Category**: error-handling
- **Suggested severity**: S3
- **Observation / repro**:
  1. Use `PORTS_CONFIG=25565:TCP:minecraft,19132:UDP:generic` with UDP 19132 unavailable for binding. Any non-timeout `ReadFrom` error, or any `Accept` error such as EMFILE on the TCP port, takes the same path.
  2. `serveUDP` sends `listen udp :19132: …` to `errCh` and returns (`main.go:709-713`).
  3. `run` receives it in `case err := <-errCh:` and calls `wg.Wait()` (`main.go:419-420`). The TCP goroutine is still blocked in `Accept` and exits only when ctx is cancelled (`main.go:436-439`). `run` does not own a cancel, so `wg.Wait()` blocks until SIGTERM.
  4. Nothing is logged. The failed port is not served. The sentinel Deployment has no readiness probe, so the pod stays Ready and the game Service stays routed to it (`gameserver_sentinel.go:110-114`). The error is printed only at shutdown (`main.go:110-112`).
- **Expected**: The `run` doc comment (`main.go:383-385`) says it "blocks until ctx is cancelled or a listener reports a fatal error. It returns once every listener goroutine has exited."
- **Actual**: When other listeners are healthy, `run` blocks forever after a listener error. The port stays dead with no log line. The existing `TestRunReturnsErrorOnListenFailure` covers only the synchronous path: a single TCP port that fails in `lc.Listen` (`main_test.go:1149-1169`).

### C-sentinel-03: The process exits with status 0 after a fatal listener error
- **Location**: `sentinel/main.go:110-114`
- **Category**: error-handling
- **Suggested severity**: S4
- **Observation / repro**:
  1. `PORTS_CONFIG` names a TCP port that is already bound in the pod, which is the synchronous failure path at `main.go:407-410`.
  2. `run` returns `listen tcp :N: …`, and `main` logs `sentinel exiting: …`. Then `main` returns normally, so the exit code is 0.
- **Expected**: A fatal startup error exits non-zero, as the `config:` and `kubernetes client:` paths do at `main.go:90` and `:95` (`log.Fatalf`).
- **Actual**: The container terminates with reason `Completed` (exit 0) instead of `Error`. The kubelet still restarts it under the Deployment's `restartPolicy: Always`, but the pod status hides the failure.

### C-sentinel-04: Every proxied session logs two spurious "use of closed network connection" errors
- **Location**: `sentinel/main.go:478-482`, `:604-608`, `:688-689`
- **Category**: error-handling
- **Suggested severity**: S4
- **Observation / repro**:
  1. A join is proxied successfully and ends normally.
  2. `proxyBidirectional` closes both conns (`main.go:688-689`).
  3. `handleJoin`'s defer calls `upstream.Close()` again and logs `close upstream connection: close tcp …: use of closed network connection` (`:604-608`).
  4. `handleTCPConnection`'s defer calls `conn.Close()` again and logs `close connection: close tcp …: use of closed network connection` (`:478-482`).
- **Expected**: Close errors are logged only for real failures.
- **Actual**: Two error-looking log lines for every successful hand-through, including every shutdown of an active proxy.

### C-sentinel-05: specs.md says Hostport "Works as above", but the operator gives Hostport no hold window
- **Location**: `sentinel/specs.md:143`; `operator/internal/controller/gameserver_sentinel.go:95-101` and `:120-123`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:143`: "**Hostport:** Sentinel listens on port N inside the pod; the kubelet binds the host's port N to the pod's port N. Works as above (the pod sees normal port binding; the kubelet handles the host-level binding)." "Above" means the hold-and-poll and proxy path.
  2. `gameserver_sentinel.go:95-101`: "Hostport: the sentinel and the game pod bind the exact same host port, so the sentinel cannot outlive the wake decision … There is no hold-open window in this mode." Lines 120-123 delete the sentinel as soon as the server is awake.
- **Expected**: `specs.md` describes the Hostport asymmetry, as `docs/roadmap.md:158-160` does.
- **Actual**: `specs.md` says Hostport behaves like the Service-backed modes. In Hostport mode every held join ends in a bounce.

### C-sentinel-06: specs.md says UDP sources are tracked per IP; the code keys on IP:port and does not count packets during cooldown
- **Location**: `sentinel/main.go:754` and `:816-822`; `sentinel/specs.md:19`, `:116`, `:149`, `:158`, `:162`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:19`: "Track UDP sources by IP". `:116`: "extract the remote IP address". `:162`: "bounded at `maxSources` (default 4096) distinct IPs".
  2. `main.go:754` calls `heuristic.shouldWake(addr.String(), …)`. For a `*net.UDPAddr` that string is `ip:port`, so two sockets on the same host are two sources.
  3. `specs.md:158`: "Additional packets from that source within the cooldown are counted but don't trigger another `RequestWake`". `main.go:816-822` returns before the packet is appended, and the code comment says "suppress further counting". `st.packets` was reset to `nil` at wake time (`:838`).
- **Expected**: `specs.md` matches the keying and the cooldown counting, or the code keys by IP as documented.
- **Actual**: Sources are per `ip:port`, and packets are not counted during cooldown.

### C-sentinel-07: Behaviour statements in specs.md that the code contradicts
- **Location**: `sentinel/specs.md:51`, `:82`, `:94`, `:235`; `sentinel/main.go:232-235`, `:624-641`, `:66`, `:560-569`/`:668-673`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:51`: "**`PORTS_CONFIG`** (required)". `parsePortsConfig("")` returns `nil, nil` (`main.go:232-235`), `loadConfig` accepts it, and `run` then waits on ctx with no listeners. `TestParsePortsConfig` asserts the empty case succeeds (`main_test.go:32`).
  2. `specs.md:82`: "The sentinel dials this address with exponential backoff / polling (default interval: 250ms)". `waitForUpstream` uses a fixed `time.After(pollInterval)` (`main.go:635-639`). `specs.md:125` itself says "fixed interval".
  3. `specs.md:94`: the example status JSON is `{"version":{"name":"Asleep","protocol":0},"players":{"max":0,"online":0,"description":{"text":"Asleep — joining wakes it"}}`, which has unbalanced braces and puts `description` inside `players`. The code sends `…"players":{"max":0,"online":0,"sample":[]},"description":{…}}` (`main.go:66`).
  4. `specs.md:235`: "Generic protocol (no protocol-native disconnect) doesn't half-close (no-op)." Generic TCP goes through the same `handleJoin` → `proxyBidirectional` path (`main.go:560-569`), which calls `closeWrite` on both sides (`:668`, `:673`).
- **Expected** / **Actual**: `specs.md` should state what the code does. It doesn't in any of these four places.

### C-sentinel-08: Stale dependency, test-name and reference entries in specs.md
- **Location**: `sentinel/specs.md:5`, `:284-285`, `:311`, `:315`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `specs.md:5`: "**Dependencies:** `controller-runtime` libs, Kubernetes client-go, gameproto". `go.mod` does not require controller-runtime, and `main.go:293-296` says the dynamic client is used so the module "doesn't need controller-runtime".
  2. `specs.md:284-285` names `TestHandleRegistryProtocolLookupSuccess`, `TestHandleRegistryProtocolStatusPing`, `TestHandleRegistryProtocolJoinAndWake`, `TestHandleRegistryProtocolUnknown`, `TestParsePortsConfigUnknownProtocol` and `TestParsePortsConfigValidRegisteredProtocols`. None of them exist in `main_test.go`. The actual tests are `TestRegistryProtocolDispatch*`, `TestParsePortsConfigRejectsUnknownProtocol` and `TestRegistryProtocolReplayConsumedBytes` (`main_test.go:1327-1529`).
  3. `specs.md:311` references `test/e2e/tests/bot_*.go`. That directory does not exist; the files are `test/e2e/*_bot_e2e_test.go`, and the wake tests are in `test/e2e/wake_on_connect_e2e_test.go`.
  4. `specs.md:315` references `CLAUDE.md` ("Wake-on-connect") "project context and design rationale". `CLAUDE.md` has no such section; the string appears only in the repo-map, coverage and architecture table rows.
- **Expected** / **Actual**: The references should resolve, and they don't.

## Questions (not findings)

- Hold time vs `WAKE_DEADLINE`: `handleJoin` calls `RequestWake(ctx)` synchronously, and the patch has no timeout of its own, before `waitForUpstream` starts the deadline (`main.go:593-597`). A slow apiserver extends the real hold beyond `WAKE_DEADLINE`, which `specs.md:60` ties to the client's ~30 s timeout. Should the deadline include the patch?
- Failed patches: `lastPatch` is stamped even when the patch fails (`main.go:352-357`), and callers coalesced during an in-flight patch get `nil`. A transient apiserver error on the only join therefore leaves the server asleep for that attempt, with no retry inside the 25 s hold. Is that intended? `specs.md:124` only says "fails but logged".
- UDP ports with a registered `wakeProtocol`: the CRD allows `wakeProtocol: minecraft` on a UDP port. `run` sends every UDP port to `serveUDP`, which applies the generic heuristic whatever the `wakeProtocol` (`main.go:397-403`). Should that combination be rejected or documented?
- The gameproto registry includes `demo` (`gameproto/registry.go:16`), so `parsePortsConfig` accepts `wakeProtocol: demo` and lists it in its "registered protocols" error. The CRD enum blocks it on the operator path. Is `demo` meant to be accepted by the production binary?
- `minecraftAsleepStatusJSON` is passed to `BuildStatusResponse` of any classifier that supports status pings (`main.go:522-523`). Only Minecraft does today, but the dispatcher is described as protocol-agnostic (`specs.md:98`).
- The sentinel Deployment has no readiness probe, so `sentinelIsReady` becomes true once the container runs, possibly before its listeners bind (`gameserver_sentinel.go:356-363`). Is that sub-second gap accepted?
