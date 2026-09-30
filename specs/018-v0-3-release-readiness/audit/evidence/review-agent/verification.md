# T045 agent chunk: independent verification (opus)

- **Date**: 2026-09-24
- **Input**: `audit/evidence/review-agent/notes.md` (13 candidates, C-agent-01 to C-agent-13). The notes also record 3 held candidates (OD-019). Those are verified in the held folder and are not covered here.

## Method

I checked every candidate against branch `018-v0-3-release-readiness` at `3de03ab0`. There, `agent/`, `api/`, `operator/`, `charts/`, `web/`, `modules/` and `docs/` have no diff against `origin/master` (`13a859ff`), so the lines cited below are the same on master.

Agent code I read:
- All of `agent/cmd/main.go` and the `files`, `console`, `logs`, `heartbeat`, `players` and `quiesce` packages, plus `rcon/rcon.go`.
- The parts of `mods`, `usage`, `status`, `actions` and `auth` that the candidates cite.
- The tests that pin the behaviour: `files_gaps_test.go`, `files_test.go`, `rcon_extra_test.go`, `heartbeat_test.go`, `quiesce_declared_test.go` and `web/src/routes/tabs/Players.test.tsx`.

I followed each path to its other end:
- The API's body-limit and timeout middleware (`api/cmd/main.go:602-700`) and its agent proxy (`api/internal/ws/dialer.go`).
- The web pod's nginx (`web/nginx.conf.template`) and the ingress (`charts/gameplane/templates/ingress.yaml`, `values.yaml:288-311`).
- The operator's agent container (`gameserver_controller.go:2085-2200`) and its phase derivation (`gameserver_status.go`).
- The Backup quiesce path (`backup_controller.go:270-300`, `:535-560`).
- The dashboard components that render agent output.

I also read the library code the candidates depend on: chi `v5.3.2` `middleware/timeout.go` and coder/websocket `v1.8.15` `conn.go`. I checked `audit/findings.md` (none of these candidates is tracked there) and searched the other chunks' notes for duplicates. `go build ./...` in `agent/` passes. I ran no tests or linters.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-agent-01 | kept | S1 | Confirmed in the agent. `write` truncates the target before copying, and `savePart` deletes a file that already existed when its copy fails. The notes' repro is wrong for a default install. In that path, `web/nginx.conf.template` sets no `client_max_body_size`, so nginx's 1 MiB default returns 413 before the API or the agent sees the body, and the ingress's `64m` limit stops the over-64-MiB upload even earlier. The defect is still deterministic in three cases: with `web.enabled=false`, where the ingress routes to the API; through a port-forward to the API; and against the agent's own 64 MiB cap. On a default install, a full volume or an API or agent restart during the transfer triggers it. The result is the loss of a file that already existed, so S1. A side observation for the web and chart chunks: the same nginx default means the dashboard can't save or upload any file over 1 MiB. `review-api/notes.md:257` already raises this as a question. |
| C-agent-02 | kept | S3 | Confirmed. chi `Timeout` gives every request context a 30 s deadline. coder/websocket closes the connection when the read context expires, so the Console and file-log streams drop 30 s after they connect, and mod URL downloads are limited to 30 s. The dashboard reconnects on its own, so S3. Not a duplicate: the API's fix `3221df76` changed only `api/cmd/main.go`. |
| C-agent-03 | kept | S3 | Confirmed, with the threshold corrected. The size field includes 10 bytes of framing, so the check at `rcon.go:300` rejects any reply packet whose body is 4087 bytes or more. Minecraft sends chunks of up to 4096 characters, so its size field reaches 4106, and more for non-ASCII text. `rcon_extra_test.go:65-81` only pins size 0. |
| C-agent-04 | kept | S3 | Confirmed. `gameVersion` carries `tmpl.Spec.Game` (for example `minecraft-java`), and the dashboard shows it as the version in two places. The CRD documents the field as the running game's version. Nothing else reads it, so the impact is wrong information on screen. |
| C-agent-05 | rejected | n/a | Speculative, and no reproducible path is shown. The notes say the timing was derived from the code, not observed. The scenario needs RCON auth to succeed while commands go unanswered for 30 s or more, again and again, plus a concurrent poller. Among the shipped protocols, only Minecraft handles auth on a separate thread, and its watchdog stops the server once one tick passes 60 s. Without contention the heartbeat still lands inside the 60 s window: `sendOnce` blocks for about 30 s, and the tick buffered by the Go ticker starts the next send at once, so `lastHeartbeat` stays between 30 and 60 s old. |
| C-agent-06 | kept | S3 | Confirmed, both halves. The 11 modules with `rcon.protocol: none` show "-1" and "-1 online" on the Players tab, which is always visible. The 10 Source-protocol modules without `capabilities.players` show "0 / 0 online" and "Nobody online." whoever is connected. The heartbeat reports the same case as `null`. |
| C-agent-07 | rejected | n/a | Duplicate of **C-charts-gameplane-05** (`review-charts-gameplane/notes.md:96-104`). The defect and the fix location are the same (the chart's PodMonitor), and that item also covers the NetworkPolicy side. The facts are confirmed here. The operator always passes `--tls-cert/--tls-key/--tls-client-ca` (`gameserver_controller.go:2104-2106`). `auth.ServerTLS` requires a verified client certificate (`auth.go:111`). The PodMonitor sets no scheme and no TLS config (`servicemonitors.yaml:46-60`). If the charts-chunk verifier rejects C-charts-gameplane-05, this item needs another look. |
| C-agent-08 | kept | S4 | Kept as documentation drift only. `docs/module-authoring.md:1109-1111` promises that any quiesce command error, or a `failurePattern` match, triggers a best-effort `unquiesce`. On purpose, the code skips it when the first command fails (`quiesce.go:128`), and `TestDeclaredQuiescer_FirstCommandErrorSkipsRollback` pins that. The operator doesn't cover the gap. The runtime scenario in the notes (a `save-off` that times out but still runs) was not reproduced, because it needs the game's main thread to stall for 30 s or more without the watchdog stopping the server. Any change to the code needs sign-off, because a pinned test would change. |
| C-agent-09 | kept | S3 | Confirmed. `resolve` returns the `EvalSymlinks` target for an existing path, and `del` removes that path. Deleting a symlink inside the root deletes the file it points to and leaves a dangling link. The file API can't create a symlink, and I found no shipped module that creates one inside the data volume. So the trigger is narrow, but the result is data loss. |
| C-agent-10 | rejected | n/a | Unreachable. The early `return` without `Unlock` (`usage.go:157-159`) needs a CPU counter delta above 2^63 µs (about 292,000 years). No behaviour can be reproduced. This is code hygiene, worth fixing in any change that touches `usage.go`. |
| C-agent-11 | kept | S4 | All 9 items confirmed against `specs.md`, `openapi.yaml` and the mounted handlers. Not a duplicate of C-api-14, which covers `api/specs.md`. |
| C-agent-12 | kept | S4 | Confirmed. One location correction: the `go.mod` require block is at lines 18-23 (k8s.io at 22-23), not 18-25. The operator and the API are also on k8s.io `v0.37.0`, so the "intentional pin" note at `specs.md:179` is false. F-035 covers `docs/dependencies.md`, not this file. |
| C-agent-13 | kept | S4 | All 5 items confirmed. |

### C-agent-01

**Location**: `agent/internal/files/files.go:202-224` (`write`: `os.Create` at `:213` truncates the target before the `io.Copy` at `:219`); `agent/internal/files/files.go:302-331` (`savePart`: `os.Create` at `:309`, then `os.Remove(dstPath)` in the deferred cleanup at `:315-319` on any error). Promises: `agent/specs.md:154`, `:202`.

**Repro / observation**:
1. By reading master: `write` opens the target with `os.Create`, which truncates it, and only then copies the body. After `:213`, any error leaves the target truncated or partly rewritten, and the handler returns 500 (`httpErr`). That covers a body that ends early, the agent's own 64 MiB `MaxBytesReader` at `:219`, and ENOSPC.
2. `savePart` does the same for each upload part. Its deferred cleanup deletes `dstPath` on any error. If a file already existed at `dstPath`, the cleanup deletes the user's previous file, not a partial file the handler "just created" (`specs.md:154`).
3. Agent-only, no cluster needed:
   - Write: use the `newServer` helper from `files_gaps_test.go`. Write `big.txt` with known content, then POST `maxWriteBytes+1` bytes to `/files/write?path=/big.txt`, as `TestWrite_BodyTooLarge` does. The response isn't 204, and `big.txt` now holds the first 64 MiB of the new body.
   - Upload: create `x.txt` in a temp dir and call `savePart(dir, "x.txt", errReader{}, 16)`, as `TestSavePart_CopyError` does. `x.txt` no longer exists.
4. Live, on a supported configuration:
   - Install with `web.enabled=false`. The ingress then routes to `gameplane-api` directly (`charts/gameplane/templates/ingress.yaml:24`), with the default `proxy-body-size: 64m` (`values.yaml:307`).
   - Put a 1.5 MiB file in a server's data volume.
   - With a valid session, `POST /servers/<name>/files/write?path=/<file>` with a 1.5 MiB body.
   - The API wraps the body in a 1 MiB `MaxBytesReader` (`api/cmd/main.go:254`, `:607-616`); `/files/write` is not exempt (`:620-623`). The proxy streams the body upstream with no length set (`api/internal/ws/dialer.go:283-286`), so the upstream request aborts after about 1 MiB.
   - The call fails, and the file now holds about 1 MiB of the new content. The same happens through `kubectl port-forward` to the API Service.
5. A correction to the notes' repro. On a default install (`web.enabled: true`), neither of the notes' dashboard triggers reaches the agent. `web/nginx.conf.template` (`location @backend`) sets no `client_max_body_size`, so nginx's 1 MiB default returns 413 for the 1.5 MiB save, and the file stays intact. The over-64-MiB upload is rejected even earlier, by the ingress's `64m` limit. On a default install, the remaining triggers are failures during the transfer: the data volume fills up, or the API or agent restarts while a file is being written.

**Expected**: A failed write or upload leaves the previous file untouched. One way: write to a temporary file in the same directory and rename it over the target only on success, as `mods.go:321-383` and `:461-503` already do.

**Actual**: A failed `/files/write` leaves the target truncated or partly rewritten. A failed `/files/upload` deletes the file that was already there.

### C-agent-02

**Location**: `agent/cmd/main.go:160` (`r.Use(middleware.Timeout(30 * time.Second))` on the root router, so it applies to every route). Affected handlers: `agent/internal/console/console.go:54-61`, `agent/internal/logs/logs.go:69-78`, `agent/internal/mods/mods.go:228,230`.

**Repro / observation**:
1. chi's `Timeout` (`chi/v5@v5.3.2/middleware/timeout.go`) replaces the request context with `context.WithTimeout(r.Context(), 30s)`. After the handler returns, it writes 504 if the deadline has passed.
2. `console.serve` reads with `wsjson.Read(ctx, …)` on that context. For every read, coder/websocket `v1.8.15` arms `context.AfterFunc(ctx, … c.close())` (`conn.go:188-198`). So the connection closes 30 s after the upgrade, whether or not it is in use. The error is `context.DeadlineExceeded`, not `context.Canceled`, so the handler also sends a close with `StatusInternalError` (`console.go:57-60`).
3. Live: open the Console tab of a running RCON server (for example `minecraft-java`) and leave it open. About 30 s after the "connected" line, the terminal prints the disconnect line and reconnects (`web/src/routes/tabs/useConsoleTerminal.ts:113-128`). This repeats every 30 s.
4. The Logs tab's file source (`logs.go:69-78`, through `streamFile`) is cut the same way. With `from=end`, lines written during the reconnect gap are never shown.
5. `mods.install` passes `req.Context()` to `installArchive` and `download` (`mods.go:228,230`). A mod download that takes longer than 30 s fails with 502, although the HTTP client allows 2 minutes (`mods.go:850`).
6. On the API side, `3221df76` fixed the same defect (`api/cmd/main.go:662-700`, `isStreamingRequest`). The agent line dates from the initial commit and was never changed.

**Expected**: The WebSocket routes are exempt from the request deadline, as they are in the API. Mod URL installs are bounded by their own download timeout, not by the router's 30 s.

**Actual**: Console and log streams drop every 30 s, and mod URL installs fail after 30 s.

### C-agent-03

**Location**: `agent/internal/rcon/rcon.go:300` (`if size < 10 || size > 4096`), reached from `Exec` at `:160-175`.

**Repro / observation**:
1. `readPacket` rejects any packet whose size field is above 4096 (`:300`). The size field counts the id, the type, the body and the two trailing nulls, so any reply packet with a body of 4087 bytes or more fails. The check runs before `gotResponse` is set, so `Exec` drops the connection and returns an error (`:174-175`).
2. Minecraft splits long replies into chunks of up to 4096 characters and writes each chunk's size as its UTF-8 byte length + 10. The first chunk of a long reply therefore has a size field of 4106, or more if the text isn't ASCII. The 4096 cap is right for outbound packets (`:281`), but not for replies.
3. Unit-level, no cluster needed: use `net.Pipe` as `TestReadPacket_MalformedSize` does (`rcon_extra_test.go:65-81`), and write a well-formed packet with size 4106. `readPacket` returns `rcon: malformed packet size 4106`. The existing test covers only size 0.
4. Live: on a `minecraft-java` server, ban enough players (about 80) that `banlist players` (`modules/minecraft-java/template.yaml:320`) prints more than 4086 bytes. The Players tab's ban list then gets 502 from `/players/banned` (`players.go:285-288`). Running `banlist players` in the Console tab returns an `err` envelope.

**Expected**: Replies of up to at least 4106 bytes (a 4096-byte body + 10) are accepted. Non-ASCII text can make a Minecraft chunk larger, so the documented bound should allow for that too.

**Actual**: Any Minecraft reply of 4087 bytes or more fails, together with the RCON connection it was read on.

### C-agent-04

**Location**: `agent/internal/heartbeat/heartbeat.go:129` (`"gameVersion": cfg.Game`). `cfg.Game` comes from `--game` / `GAMEPLANE_GAME` (`agent/cmd/main.go:86,208`), which the operator sets to `tmpl.Spec.Game` (`operator/internal/controller/gameserver_controller.go:2130`).

**Repro / observation**:
1. Start a server from `minecraft-java` (`game: minecraft-java`, `modules/minecraft-java/template.yaml:24`) and wait one heartbeat interval (20 s).
2. `kubectl get gameserver <name> -o jsonpath='{.status.agent.gameVersion}'` prints `minecraft-java`.
3. The CRD documents the field as "the version string the running game reports" (`operator/api/v1alpha1/gameserver_types.go:654`). The dashboard shows it as the version in the page header (`web/src/routes/ServerDetail.tsx:115,194`) and in the "Version" row of the status card (`web/src/components/server/ServerStatusCard.tsx:60-63`).
4. `heartbeat_test.go:60,75` feeds `Game: "minecraft-1.20"`, a value the operator never sets, so the test passes.

**Expected**: The field holds the running game's version (or the selected version token), or it is left empty.

**Actual**: The field holds the template's game identifier (`minecraft-java`, `palworld` and so on), and the dashboard labels that as the version.

### C-agent-06

**Location**: `agent/internal/players/players.go:118-129` (RCON disabled: `online: -1, max: -1`) and `:367-371` (`list` output in neither Minecraft format: `online: 0, max: 0`). Renderer: `web/src/routes/tabs/Players.tsx:102,115,195`.

**Repro / observation**:
1. RCON disabled:
   - These 11 modules declare `rcon.protocol: none`: 7-days-to-die, arma-reforger, beammp, dont-starve-together, enshrouded, euro-truck-simulator-2, garrys-mod, mount-and-blade-2-bannerlord, terraria, tmodloader and valheim. For them the agent uses `rcon.Disabled`, and `/players` answers `online: -1, max: -1`.
   - The Players tab is always shown: `ServerDetail.tsx:133-138` filters only Console, Mods and Modpacks.
   - `Players.tsx:102,115` renders "-1" and "-1 online", and `:195` adds "Nobody online.".
   - `Players.test.tsx:60-69` covers only `max: -1`.
2. No player list declared:
   - These 10 source-protocol modules declare no `capabilities.players`: ark-survival-ascended, ark-survival-evolved, cs2, hell-let-loose, left-4-dead-2, project-zomboid, squad, team-fortress-2, the-isle and v-rising. For them the agent sends `list`.
   - A reply that matches neither Minecraft format, such as an unknown-command message, gives `online: 0, max: 0` (`players.go:367-371`). The tab shows "0 / 0 online" and "Nobody online." whoever is connected.
3. For the same reply, when it contains no digits, the heartbeat patches `playersOnline: null` (`heartbeat.go:149-168`). So the status card shows "—" while the Players tab shows 0.
4. Neither `agent/specs.md:129` nor `agent/openapi.yaml:43-51` defines a value for "unknown".

**Expected**: One documented representation of "unknown" (for example `online: null`) for both cases, rendered as "—".

**Actual**: Two different values, `-1/-1` and `0/0`, and the dashboard renders both as real counts.

### C-agent-08

**Location**: `docs/module-authoring.md:1109-1111`, compared with `agent/internal/quiesce/quiesce.go:118-135` (the `if i > 0` at `:128`). Operator side: `operator/internal/controller/backup_controller.go:286-296` and `:542-549`.

**Repro / observation**:
1. `module-authoring.md:1109-1111` says: "any command error — or output matching `failurePattern` (case-insensitive) — aborts the backup and best-effort runs `unquiesce` so the game is never left paused."
2. `declaredQuiescer.Quiesce` runs `unquiesce` only when a later command fails (`i > 0`). If the first command returns an error, or its output matches `failurePattern`, the function returns without running `unquiesce`. `TestDeclaredQuiescer_FirstCommandErrorSkipsRollback` (`quiesce_declared_test.go:61-70`, "Nothing was paused yet") pins this on purpose.
3. The operator doesn't fill the gap. On any error other than `ErrUnsupported`, it fails the Backup (`:292`) without setting the `quiesce-attempted` annotation, and `maybeUnquiesce` returns early when that annotation is empty (`:547`).
4. Not reproduced: the notes' runtime case, where a first command times out but still runs later.

**Expected**: The doc and the code agree. Either the doc says `unquiesce` runs only after an earlier command has succeeded, or the agent runs a best-effort `unquiesce` on any quiesce failure. The second option needs sign-off because it changes a pinned test.

**Actual**: The doc promises a rollback on every failure, and the code skips it when the first command fails.

### C-agent-09

**Location**: `agent/internal/files/files.go:71-75` (`resolve` returns the `EvalSymlinks` result for an existing path) and `:346-368` (`del` removes the resolved path). The listing is at `:125-138`.

**Repro / observation**:
1. Inside a server's data volume, create `logs/a.log` and a relative symlink `latest.log -> logs/a.log`. The file API can't create a symlink, so use a unit test with `os.Symlink` under the `newServer` root, or `kubectl exec` into the game container.
2. `/files/list` shows `latest.log` with mode `L…` and `dir: false`, because `DirEntry.Info` reports the link itself.
3. Delete `latest.log` in the Files tab. The dashboard sends `DELETE /servers/<name>/files/delete?path=/latest.log` with no `recursive` flag, because `Files.tsx:137` passes `entry.dir`, which is `false`.
4. `resolve` returns `<root>/logs/a.log`, and `os.Remove` deletes it. `latest.log` remains and now dangles. For a link to a directory, `os.Remove` either fails (non-empty) or removes the empty target directory.
5. No existing test covers a link that stays inside the root. The symlink tests in `files_test.go` cover only links that escape it.

**Expected**: Delete acts on the entry that was named. For example: confine and resolve the parent directory, then remove the final component without following it.

**Actual**: The link's target is deleted, and the link is left behind.

### C-agent-11

**Location**: `agent/specs.md:63`, `:82-100`, `:110-142`; `agent/openapi.yaml:52-63`.

**Repro / observation** (by reading master; each item gives the spec text, then the code):
1. `specs.md:137`: `/actions/run` request `{ name, params }`. The code decodes `{ "id", "params" }` (`actions.go:110-113`).
2. `specs.md:138`: `/status` response `{ metrics[] }`. The code writes a bare JSON array (`status.go:104,114,130`).
3. `specs.md:139`: `GET /mods` response `{ mods[] }`. The code writes a bare array (`mods.go:110,115,157`).
4. `specs.md:140`: `/mods/install` request `{ name, version, ... }`. The code reads `{ url, name?, replaces?, meta? }` and has no `version` field (`mods.go:160-169`).
5. `specs.md:141`: `DELETE /mods` request `{ name }`. The code reads the `?name=` query parameter (`mods.go:665`).
6. The table at `specs.md:110`, described as "from mounted routes", omits `GET /logs/download` (`logs.go:32`) and `GET /players/whitelist` with `POST /players/whitelist/{add,remove}` (`players.go:102-104`). The API proxies all four (`dialer.go:43,67-69`).
7. The flag table (`specs.md:82-100`) omits `--cli-pipe` / `GAMEPLANE_CLI_PIPE` (`main.go:77-78`).
8. `specs.md:63` says the heartbeat patches `status.playersOnline`, `status.playersMax` and `status.gameVersion`. The code patches `status.agent.*` (`heartbeat.go:126-130,202-204`).
9. The `openapi.yaml` `Capabilities` schema (`:52-63`) lists `kick`, `ban` and `unban`. The agent also returns `whitelist` (`players.go:54`).

**Expected**: The spec and the OpenAPI file describe the mounted contract.

**Actual**: They differ from the code as listed.

### C-agent-12

**Location**: `agent/specs.md:170-179`, compared with `agent/go.mod:18-23`.

**Repro / observation**:
1. The spec table lists chi v5.1.0, coder/websocket v1.8.12, apimachinery and client-go v0.31.1, client_golang v1.20.5 and x/sys v0.22.0. `go.mod:18-23` has v5.3.2, v1.8.15, v0.37.0, v0.37.0, v1.24.1 and v0.48.0.
2. `specs.md:179` says "The agent is pinned to Kubernetes v0.31.1 … while other modules (operator, api) use v0.35.0 … This is intentional". The agent, `operator/go.mod:26-27` and `api/go.mod:39-40` all use k8s.io `v0.37.0`.

**Expected**: The table matches `go.mod`, and the pin note is removed.

**Actual**: Both are stale.

### C-agent-13

**Location**: `agent/internal/players/players.go:6-8`, `agent/specs.md:67`, `agent/internal/rcon/rcon.go:14-16`, `agent/internal/heartbeat/heartbeat.go:2-3`, `agent/internal/rcon/satisfactory.go:64-66`.

**Repro / observation** (by reading master):
1. `players.go:6-8` says moderation is dispatched "through a small commander strategy keyed off the agent's --game flag". `pickCommander` ignores the game and builds only from declared capabilities (`commander.go:28-37`).
2. `specs.md:67` promises "game-specific `commander` implementations (Minecraft, Satisfactory, Palworld, etc.)". Only `templateCommander` and `unsupportedCommander` exist (`commander.go:47,222`).
3. `rcon.go:14-16` says replies are assembled with "the Valve-documented 'empty-cmd sentinel' trick". `rcon.go:147-156` says the code deliberately doesn't use that trick and uses a grace window instead.
4. `heartbeat.go:2-3` says the heartbeat reports "the agent's own cpu/memory/disk usage". In proc mode, which the operator always enables, the values are the game processes' usage, with the agent's own subtree excluded (`usage.go:8-13,306-321`). On the operator side, the CRD comment repeats "the agent's own cgroup CPU usage" (`gameserver_types.go:658`).
5. `satisfactory.go:64-66` says RCON clients "never leave the pod, see the package doc comment on rcon.go". That package doc (`rcon.go:1-16`) says nothing on the subject.

**Expected**: The comments and the spec describe the current design.

**Actual**: The descriptions are stale, as listed.
