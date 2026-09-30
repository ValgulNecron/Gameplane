# Review: agent

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `agent/specs.md`, `agent/openapi.yaml`, `docs/architecture.md` (agent flows, lines 76-160 and 265-290), `docs/security.md` (API → Agent, runtime mod installs), `docs/install.md` (Metrics, lines 250-285), `docs/module-authoring.md` (Capabilities, lines 1046-1200)

held candidates: 3 (see OD-019)

## Scope reviewed

Read in full (33 agent files):
- `agent/cmd/main.go`
- `agent/internal/actions/actions.go`, `auth/auth.go`, `caps/caps.go`, `console/console.go`, `files/files.go`, `heartbeat/heartbeat.go`, `httpjson/httpjson.go`, `lifecycle/lifecycle.go`, `logs/logs.go`, `metrics/metrics.go`, `mods/{mods,confinement,manifest}.go`, `players/{players,commander}.go`, `quiesce/quiesce.go`, `rcon/{rcon,websocket,telnet,battleye,satisfactory,palworld,nuclearoption,rest,cli}.go`, `status/status.go`, `usage/usage.go`
- `agent/specs.md`, `agent/openapi.yaml`, `agent/.testcoverage.yml`, `agent/Dockerfile`, `agent/go.mod`

Read in part, to follow a contract across components:
- `api/cmd/main.go` (`requestTimeout`, `bodyLimit`, `isUploadPath`: lines 246-260, 600-690), `api/internal/ws/dialer.go` (routes and `httpProxyLimit`: lines 1-90, 240-300), `api/internal/ws/actions.go` (lines 40-140)
- `operator/internal/controller/gameserver_controller.go` (`buildAgentContainer`: lines 2085-2200), `backup_controller.go` (`maybeQuiesce`, `maybeUnquiesce`), `gameserver_status.go` (`heartbeatFreshness`, `derivePhase`), `operator/api/v1alpha1/gameserver_types.go` (`AgentStatus`), `gametemplate_types.go` (capability JSON tags)
- `charts/gameplane/templates/servicemonitors.yaml`, `networkpolicies.yaml` (lines 70-140), `charts/gameplane/values.yaml` (`serviceMonitors`)
- `web/src/routes/tabs/Files.tsx`, `web/src/routes/tabs/Players.tsx` (lines 90-120), `web/src/routes/ServerDetail.tsx` (lines 100-200), `web/src/components/server/ServerStatusCard.tsx` (lines 50-70), `web/src/routes/tabs/useConsoleTerminal.ts` (lines 100-150), `web/src/lib/endpoints.ts` (`Files`)
- `modules/*/template.yaml`: `rcon.protocol`, `capabilities.players`, `capabilities.quiesce` and `extract` blocks only
- `SECURITY_AUDIT.md` (to avoid re-reporting tracked items)

Not read, or only grepped:
- Agent `*_test.go` files: grepped only (quiesce rollback test, heartbeat `gameVersion` test, RCON packet-size tests)
- `netguard/` and `gameaction/` internals: reviewed in their own chunks
- Game-level semantics of the `rest`, `cli` and `nuclearoption` protocols: already tracked under the spec 015 RCON protocol questions

## Method

1. Built the route and flag inventory from `cmd/main.go` and each `Mount` function, then diffed it against the `specs.md` endpoint and flag tables and against `openapi.yaml`.
2. Traced every handler's error path: what reaches the client, what gets logged, and whether sentinel errors (`rcon.ErrDisabled`, `rcon.ErrAuth`, `errTooLarge`, `netguard.ErrBlockedAddr`) survive to an `errors.Is` check.
3. Followed each contract to the other side (operator env and args, API proxy, dashboard renderer) wherever the agent's output is consumed.
4. Checked the doc sentences that describe agent behaviour against the code.
5. `go build ./...` in `agent/` passes. No tests or linters were run (Rule 8).

## Observations (no finding)

- Every route except `/healthz` and `/metrics` is mounted inside the authenticated group (`main.go:167-178`).
- The RCON clients wrap transport errors with `%w`. Handlers classify them with `errors.Is(err, rcon.ErrDisabled)` (`players.go:118,221,256,281`, `actions.go:152`, `status.go:111`). `caps.Parse` wraps with `%w`.
- The BattlEye client's split between `mu` and `ioMu`, its `failAll`/`dropLocked` ordering, its fragment reassembly and its keepalive all look consistent. No deadlock path was found.
- Satisfactory, Palworld and REST each classify auth failures and arm the cooldown only on 401/403, or on a 2xx error envelope for Satisfactory, as their doc comments say.
- Mod URL installs and mod uploads write through a dot-temp file and then rename (`mods.go:321-383`, `461-503`). Archive entries are confined, and symlink entries are rejected (`mods.go:579-598`).
- The `caps` JSON tags match the operator's `CapabilitiesSpec` tags field for field.
- The operator always passes `--tls-cert/--tls-key/--tls-client-ca`, `--data-root` and `GAMEPLANE_USAGE_PROC=1` (`gameserver_controller.go:2104-2113,2143`). So the spec's "proc mode (default in production)" holds.
- The coverage statement in `specs.md:212` matches `.testcoverage.yml` (total 90, `^cmd/` excluded).

## Candidate findings

### C-agent-01: `/files/write` and `/files/upload` overwrite in place, so a failed transfer destroys the file that was already there

- **Location**: `agent/internal/files/files.go:213-222` (`write`: `os.Create` truncates first, then `io.Copy`), `agent/internal/files/files.go:309-320` (`savePart`: `os.Create`, then `os.Remove(dstPath)` on any error)
- **Category**: correctness
- **Suggested severity**: S1 (data loss). Both triggers below are deterministic, not only client aborts.
- **Observation / repro**:
  1. Path A, `/files/write`. The data volume holds a 1.5 MiB text file (for example `config/big.json`). The agent serves it inline because the cap is 2 MiB (`files.go:158`), and the Files tab opens it without a size check (`web/src/routes/tabs/Files.tsx:79`).
  2. Edit the file and Save. The dashboard sends `POST /servers/{name}/files/write` (`web/src/lib/endpoints.ts:679`).
  3. The API wraps every body except `/files/upload` in `MaxBytesReader(1 MiB)` (`api/cmd/main.go:254,607-620`). The proxy streams the body upstream with no length set (`api/internal/ws/dialer.go:283-286`), so the upstream request aborts at about 1 MiB.
  4. By then the agent has already run `os.Create` on the target (`files.go:213`), which truncated the original. `io.Copy` (`files.go:219`) then fails, the handler returns 500, and the file is left holding the first ~1 MiB.
  5. Path B, `/files/upload`. A directory already holds `world.zip`. Upload a replacement `world.zip` larger than 64 MiB, or upload a new 40 MiB file together with a 30 MiB `world.zip` in one request. The 64 MiB budget is shared by all parts (`files.go:227-231`; the API proxy applies the same 64 MiB cap at `dialer.go:247`).
  6. `savePart` truncates the existing `world.zip` with `os.Create` (`files.go:309`). The copy fails when the body cap trips, and the deferred cleanup runs `os.Remove(dstPath)` (`files.go:318`), which deletes it.
  7. `specs.md:154` describes this cleanup as removing "the destination file it just created". When the destination already existed, the handler didn't create it; it truncated the file and then deleted it.
- **Expected**: A failed write or upload leaves the previous file untouched: write to a dot-temp file in the same directory, then rename it over the target only on success. `mods.go` already does this.
- **Actual**: Path A leaves the file truncated to about 1 MiB. Path B deletes the pre-existing file. Both return an error.
- Note: the 1 MiB cap on `/files/write` is itself API-side (the comment at `dialer.go:243-245` claims 64 MiB). The API reviewer may report that separately.

### C-agent-02: A 30 s request timeout applies to the WebSocket routes and to mod URL installs

- **Location**: `agent/cmd/main.go:160` (`r.Use(middleware.Timeout(30 * time.Second))` on the root router); affected handlers: `console/console.go:54-61`, `logs/logs.go:69-78`, `mods/mods.go:228,230`
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. chi's `Timeout` puts a 30 s deadline on every request context (`chi/v5@v5.3.2/middleware/timeout.go:33-47`).
  2. Open the Console tab. Thirty seconds after connecting, `wsjson.Read(ctx, …)` returns `context.DeadlineExceeded`. That isn't `context.Canceled`, so the handler closes with `StatusInternalError` (`console.go:57-61`).
  3. The Logs tab (file source) behaves the same way. `streamFile` returns `DeadlineExceeded`, and `logs.go:76-77` closes with "context deadline exceeded". The dashboard reconnects (`useConsoleTerminal.ts:126-128` prints a disconnect line each time). A `from=end` tail loses whatever was written during the gap.
  4. A mod URL install passes `req.Context()` into the download (`mods.go:228,230`), so any download slower than 30 s fails with "could not download the mod" (502). That happens even though the netguard client allows 2 minutes (`mods.go:850`) and the size cap is 256 MiB (`mods.go:40`).
  5. After a WebSocket handler returns, chi's deferred `WriteHeader(504)` also fires on the hijacked connection, which adds log noise.
- **Expected**: Long-lived and streaming routes are exempt. The API fixed the same defect for itself in `3221df76` ("fix(api): exempt WebSocket and SSE routes from the global 60s request timeout"; `api/cmd/main.go:662-688`, `requestTimeout`/`isStreamingRequest`).
- **Actual**: Console and log streams are cut every 30 s, and mod installs are capped at 30 s.

### C-agent-03: Source RCON rejects Minecraft reply packets larger than 4096 bytes

- **Location**: `agent/internal/rcon/rcon.go:300` (`if size < 10 || size > 4096`)
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. Valve's Source servers cap the packet size field at 4096. Minecraft's server does not: it splits long replies into chunks with bodies of up to 4096 bytes, so the size field (body + 10) can reach 4106. This matches the wiki.vg RCON page and gorcon/rcon's `MaxPacketSize = 4096 + MinPacketSize`. It wasn't re-verified against a live server here.
  2. On `minecraft-java` (protocol `source`), run any command whose UTF-8 output is 4096 bytes or more. Examples: `banlist players` with about 60 or more entries, a long `whitelist list`, or `help` on a plugin server.
  3. The first reply packet's size is 4106. `readPacket` returns "rcon: malformed packet size 4106" before `gotResponse` is set, so `Exec` fails and drops the connection (`rcon.go:160-175`).
  4. The Console shows an `err` envelope, and `/players/banned` and `/players/whitelist` return 502 (`players.go:285-288,260-262`).
- **Expected**: Reply packets up to 4106 bytes (4096-byte body + 10) are accepted. The 4096 cap stays on outbound packets (`rcon.go:281`).
- **Actual**: Any Minecraft reply of 4096 bytes or more fails.
- **Verification**: A unit test with a fake server that sends a size-4106 packet reproduces it without a cluster.

### C-agent-04: The heartbeat reports the template's game identifier as `status.agent.gameVersion`

- **Location**: `agent/internal/heartbeat/heartbeat.go:129` (`"gameVersion": cfg.Game`); `cfg.Game` is `GAMEPLANE_GAME` (`main.go:86,208`), which the operator sets to `tmpl.Spec.Game` (`operator/internal/controller/gameserver_controller.go:2130`)
- **Category**: correctness
- **Suggested severity**: S3 (the field never carries a version; the dashboard shows the wrong value). S4 would also be defensible if it's treated as display-only.
- **Observation / repro**:
  1. Start any server, for example from `minecraft-java`.
  2. `status.agent.gameVersion` becomes `minecraft-java`.
  3. The CRD documents the field as "the version string the running game reports" (`operator/api/v1alpha1/gameserver_types.go:654-656`). `specs.md:22,63` lists `gameVersion` among the reported values.
  4. The dashboard renders it as the version: in the page header (`web/src/routes/ServerDetail.tsx:115,194`) and in the "Version" row (`web/src/components/server/ServerStatusCard.tsx:60-63`).
  5. The unit test feeds `Game: "minecraft-1.20"` (`heartbeat_test.go:60,75`), a value the operator never produces, so the test doesn't catch this.
- **Expected**: The field holds the running game's version (or the selected `GameVersion` token), or it is left unset.
- **Actual**: The field holds the game identifier (`minecraft-java`, `palworld`, and so on).

### C-agent-05: Heartbeat freshness depends on RCON latency, so a stalled game flips the phase to `Starting`

- **Location**: `agent/internal/heartbeat/heartbeat.go:127` (timestamp taken before the query), `:149` (blocking `queryPlayerCounts`), `:209-211` (patch); `agent/internal/rcon/rcon.go:95,122-124` (30 s exec deadline, one mutex shared by console, players, status, quiesce, lifecycle and heartbeat)
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. Take a `source` RCON game whose server thread is stalled: auth succeeds but commands get no reply. Minecraft runs RCON commands on the main thread, so a long tick does this.
  2. Keep the Players tab open. Each `GET /players` holds the RCON mutex for up to 30 s.
  3. The heartbeat's `Exec("list")` first waits for the mutex (up to 30 s) and then its own 30 s deadline. The patch lands carrying a `lastHeartbeat` taken at the start of `sendOnce`, so it is already 60 s or more old.
  4. The operator's freshness window is 60 s (`gameserver_status.go:27`). A stale heartbeat with a ready pod derives `Starting` (`gameserver_status.go:287-292`).
  5. The dashboard allows Stop and Restart only while the phase is `Running` (`ServerDetail.tsx:108-113`), so a wedged server can't be restarted from the UI while this lasts.
  6. `docs/architecture.md:84-88`: "That signals the pod and sidecar are healthy — **not** that the game protocol (RCON / server query) is responsive."
- **Expected**: Heartbeat liveness is independent of RCON. For example, the player-count query gets a short bound or runs separately from the timestamped patch.
- **Actual**: A slow or contended RCON delays or ages the heartbeat past the freshness window.
- **Verification**: The timing was derived from the code, not observed.

### C-agent-06: `GET /players` has no consistent "unknown" value (`-1/-1` in one case, `0/0` in another), and the dashboard shows both as real counts

- **Location**: `agent/internal/players/players.go:118-129` (RCON disabled: `online:-1, max:-1`), `players.go:367-371` (unrecognized `list` output with no `entryRegex`: `online:0, max:0`)
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. For any of the 11 modules with `rcon.protocol: none` (for example `valheim`, `terraria`), `/players` returns `online:-1, max:-1`. `Players.tsx:102,115` renders `data.max >= 0 ? … : ${data.online}`, so the Players tab reads "-1" and "-1 online".
  2. The Source-RCON modules that declare no `capabilities.players.list` are `ark-survival-ascended`, `ark-survival-evolved`, `cs2`, `hell-let-loose`, `left-4-dead-2`, `project-zomboid`, `squad`, `team-fortress-2`, `the-isle` and `v-rising`. For them the agent sends `list`. Any reply that doesn't match the two Minecraft formats yields `online:0, max:0`, which renders as "0 / 0 online" and "Nobody online." whoever is connected.
  3. For that same reply the heartbeat patches `playersOnline: null` (`heartbeat.go:141-168`) because its parse fails. So the Players tab and the server list disagree.
  4. Neither `specs.md:129` nor `openapi.yaml:43-51` defines an unknown value. The heartbeat's own comment rejects `-1` sentinels (`heartbeat.go:141-148`).
- **Expected**: One documented representation for "unknown" (for example `online: null`) in both cases, rendered as "—".
- **Actual**: Two different sentinels, both shown as numbers.

### C-agent-07: The shipped PodMonitor scrapes the agent over plain HTTP, but every operator-built agent listens only on TLS

- **Location**: `charts/gameplane/templates/servicemonitors.yaml:46-60` (`podMetricsEndpoints: - { port: agent, interval: 30s }`, which defaults to `http`); `agent/cmd/main.go:185-192,230-231`; `operator/internal/controller/gameserver_controller.go:2104-2107`
- **Category**: correctness
- **Suggested severity**: S3 (agent metrics can't be collected with the shipped config)
- **Observation / repro**:
  1. Install with `serviceMonitors.enabled: true` (`values.yaml:418`: "a PodMonitor for agent sidecars on port 8090").
  2. The operator always starts the agent with `--tls-cert/--tls-key`, so `srv.TLSConfig` is set and the agent calls `ListenAndServeTLS`.
  3. Prometheus scrapes `http://<pod>:8090/metrics`. Go's TLS server answers a plain-HTTP request with `400 Client sent an HTTP request to an HTTPS server`, so the target is down and no `gameplane_agent_*` series appear.
  4. `docs/install.md:263-264`: "Agent per-server metrics (scraped from port 8090 in each game pod when `serviceMonitors.enabled: true`)".
- **Expected**: The documented scrape works end to end.
- **Actual**: The scrape fails. Changing the PodMonitor scheme alone may not be enough; verify the scrape end to end after any fix.

### C-agent-08: Quiesce skips rollback when the first command fails, although that command may already have paused the game

- **Location**: `agent/internal/quiesce/quiesce.go:124-131` (`if i > 0 { _ = q.Unquiesce(rc) }`); operator side: `operator/internal/controller/backup_controller.go:286-294` (a hard quiesce failure fails the Backup without the `quiesce-attempted` annotation) and `:541-546` (`maybeUnquiesce` returns when that annotation is empty)
- **Category**: correctness / docs-drift
- **Suggested severity**: S3
- **Observation / repro**:
  1. `minecraft-java` declares `quiesce: ["save-off", "save-all flush"]`.
  2. `/quiesce` sends `save-off`. The server is mid-stall, so the reply takes more than the 30 s exec deadline (`rcon.go:95`). `Exec` returns a timeout (`rcon.go:174-175`), but the command still runs once the main thread catches up.
  3. With `i == 0`, no `save-on` is sent, and the agent returns 502.
  4. The operator fails the Backup without recording that quiesce was attempted, so it never unquiesces. Auto-save stays off until someone runs `save-on` by hand or the server restarts. If the server crashes before then, progress since `save-off` is lost.
  5. `docs/module-authoring.md:1109-1111`: "any command error … aborts the backup and best-effort runs `unquiesce` so the game is never left paused."
  6. `quiesce_declared_test.go:61-69` (`TestDeclaredQuiescer_FirstCommandErrorSkipsRollback`, "Nothing was paused yet") pins the current behaviour. Its premise, that an `Exec` error means the command didn't run, doesn't hold for timeouts.
- **Expected**: A best-effort unquiesce on any quiesce failure, whether in the agent or in the operator's hard-failure path. Otherwise the doc should state the exception.
- **Actual**: No rollback when the first command errors.
- Note: changing the pinned test needs maintainer sign-off (CLAUDE.md rule 1).

### C-agent-09: Deleting a symlink in the Files tab deletes the link's target instead of the link

- **Location**: `agent/internal/files/files.go:71-75` (`resolve` returns the `EvalSymlinks` result for existing paths), `files.go:346-368` (`del` removes that resolved path)
- **Category**: correctness
- **Suggested severity**: S3 (data loss, but only where in-root symlinks exist)
- **Observation / repro**:
  1. The data volume has `latest.log -> logs/2026-09-24.log`, both inside `--data-root`.
  2. `/files/list` shows `latest.log` from lstat info: mode `L…`, `dir:false` (`files.go:126-138`).
  3. Delete it from the Files tab. The request is `DELETE /files/delete?path=/latest.log`; `recursive` is false because `entry.dir` is false.
  4. `resolve` returns `/data/logs/2026-09-24.log`, and `os.Remove` deletes that file. The link stays behind, dangling.
  5. For a link to a directory, `os.Remove` on the target either fails (500, ENOTEMPTY) or removes an empty directory.
- **Expected**: Delete acts on the named entry. Confine and resolve the parent directory, then `lstat` and remove the final component itself.
- **Actual**: The target is deleted.

### C-agent-10: An unreachable branch in `usage.readCPU` returns while still holding `r.mu`

- **Location**: `agent/internal/usage/usage.go:147-168` (the `return` at `:157-159` skips the `r.mu.Unlock()` at `:168`)
- **Category**: dead-code
- **Suggested severity**: S4
- **Observation / repro**:
  1. The branch runs only if a CPU counter delta exceeds `math.MaxInt64` µs (about 292,000 years), so it can't be reached in practice.
  2. If it ever were reached, the mutex would stay locked. Every later `Read()` would block, the heartbeat goroutine would hang, and the phase would drop to `Starting`.
- **Expected**: Unlock before returning, or delete the branch.
- **Actual**: A latent lock leak in dead code.

### C-agent-11: `specs.md` endpoint, flag and field contracts drift from the mounted code

- **Location**: `agent/specs.md:63,82-100,110-142`; `agent/openapi.yaml:52-63`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation**: Each item quotes the spec sentence first, then the code that contradicts it.
  1. `specs.md:137`: `/actions/run` "request: `{ name, params }`". The code decodes `{ "id", "params" }` (`actions.go:110-113`), and the API rejects bodies without `id` (`api/internal/ws/actions.go:79-82`).
  2. `specs.md:138`: `/status` "response: `{ metrics[] }`". The code writes a bare JSON array of `Result` (`status.go:104,114,130`).
  3. `specs.md:139`: `GET /mods` "response: `{ mods[] }`". The code writes a bare array (`mods.go:110,115,157`).
  4. `specs.md:140`: `/mods/install` "request: `{ name, version, ... }`". The body is `{ url (required), name?, replaces?, meta? }` with no `version` (`mods.go:160-169,182-185`).
  5. `specs.md:141`: `DELETE /mods` "request: `{ name }`". The name comes from the `?name=` query parameter, and there is no body (`mods.go:665`).
  6. The route table is described as "from mounted routes in cmd/main.go" (`specs.md:110`), but it omits `GET /logs/download` (`logs.go:32`), `GET /players/whitelist` and `POST /players/whitelist/{add,remove}` (`players.go:102-104`). The API proxies all four (`dialer.go:43,67-69`).
  7. The flag table (`specs.md:82-100`) omits `--cli-pipe` / `GAMEPLANE_CLI_PIPE` (`main.go:77-78`).
  8. `specs.md:63`: the heartbeat patches "`status.playersOnline`, `status.playersMax`, and `status.gameVersion`". The code patches them under `status.agent.*` (`heartbeat.go:126-130,202-204`).
  9. The `openapi.yaml` `Capabilities` schema (`:52-63`) lists `kick`, `ban` and `unban`, but the agent also returns `whitelist` (`players.go:54`).
- **Expected**: The spec and OpenAPI describe the mounted contract.
- **Actual**: They don't match the code, as listed above.

### C-agent-12: The `specs.md` dependency table and the "pinned to Kubernetes v0.31.1" note are stale

- **Location**: `agent/specs.md:170-179` vs `agent/go.mod:18-25`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation**:
  1. The spec table lists chi v5.1.0, coder/websocket v1.8.12, apimachinery and client-go **v0.31.1**, client_golang v1.20.5 and x/sys v0.22.0. `go.mod` has v5.3.2, v1.8.15, v0.37.0, v1.24.1 and v0.48.0.
  2. `specs.md:179`: "The agent is pinned to Kubernetes v0.31.1 (Kubernetes 1.31) while other modules (operator, api) use v0.35.0 … This is intentional". The agent, the operator and the API all use k8s.io v0.37.0 (`agent/go.mod:24-25`, `operator/go.mod`, `api/go.mod`).
- **Expected**: The table matches `go.mod`, and the "intentional pin" note is removed.
- **Actual**: Both are stale. This is dependency versions, not the Gameplane version patterns that `hack/check-doc-versions.sh` covers.

### C-agent-13: In-code package docs and `specs.md` describe per-game dispatch and protocol handling that the code doesn't do

- **Location**: `agent/internal/players/players.go:6-8`, `agent/specs.md:67`, `agent/internal/rcon/rcon.go:14-16`, `agent/internal/heartbeat/heartbeat.go:2-3`, `agent/internal/rcon/satisfactory.go:64-66`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation**:
  1. `players.go:6-8` says moderation commands "are dispatched through a small commander strategy keyed off the agent's --game flag". `pickCommander` ignores the game and builds only from declared capabilities: "with no per-game special-casing in the agent" (`commander.go:28-37`).
  2. `specs.md:67` promises "game-specific `commander` implementations (Minecraft, Satisfactory, Palworld, etc.)". Only `templateCommander` and `unsupportedCommander` exist (`commander.go:47,222`).
  3. `rcon.go:14-16` says "we assemble them with the Valve-documented 'empty-cmd sentinel' trick". But `rcon.go:147-156` says "We deliberately do NOT use the Valve 'empty-cmd sentinel' trick", and it uses a grace window instead.
  4. `heartbeat.go:2-3` says it reports "the agent's own cpu/memory/disk usage". In proc mode, which the operator always enables, the values are the game processes' usage, with the agent's own subtree excluded (`usage.go:8-13,306-321`). The CRD field comments repeat "the agent's own cgroup CPU usage" (`operator/api/v1alpha1/gameserver_types.go:658-660`), which is operator-side.
  5. `satisfactory.go:64-66` says RCON clients "never leave the pod, see the package doc comment on rcon.go". `rcon.go`'s package doc (`:1-16`) says nothing about this.
- **Expected**: Comments and spec describe the current design.
- **Actual**: Stale descriptions, as listed above.

## Questions (not findings)

- Telnet client: 7 Days to Die's telnet console mirrors its log to every connected client. Unsolicited output waiting in the socket buffer between commands becomes the start of the next command's reply, because `Exec` writes without draining first (`telnet.go:133-147`). Combined with the heartbeat's "first number in the reply" parse (`heartbeat.go:251-262`), a leading log timestamp could become `playersOnline`. No shipped module enables `protocol: telnet` today (`7-days-to-die` has `rcon.protocol: none`). Should `Exec` drain before writing?
- `validateName` (`players.go:300`) limits moderation targets to `^[A-Za-z0-9_]{1,32}$` for every protocol. That fits Minecraft, and Steam64 or Palworld user IDs. Is it intended for future modules whose moderation keys contain `-`, `.` or spaces (for example EOS or display names)?
- The heartbeat's first patch waits a full interval (20 s) after start, because the ticker has no immediate send (`heartbeat.go:111-122`). So the operator's `Running` transition lags pod readiness by at least 20 s. Is that intended?
- `tailLoop` sends a partial trailing line as its own frame when it reaches EOF mid-line (`logs.go:123-128`). The xterm renderer concatenates frames, so nothing visible breaks, but `specs.md:15` says "text frames per line". Is the wording or the behaviour the one to fix?
