# T045 agent chunk: independent verification (opus)

This file is held off-git under OD-019. It covers the three candidates in `held/review-agent.md`. The wording is defensive: for each candidate it names the control and where it lives, states the correct behaviour, and says how a maintainer confirms that the control holds. It contains no misuse walkthroughs.

## Method

I checked each candidate against master `13a859ff`. None of the cited files differ between this branch (`3de03ab0`) and master.

Agent code I read in full:
- `agent/internal/files/files.go`
- `agent/internal/console/console.go`
- `agent/internal/auth/auth.go`
- `agent/cmd/main.go`
- `agent/specs.md`

Code I read in part:
- The WebRcon dial path: `agent/internal/rcon/websocket.go:261-320`.
- The error handling in `players.go`, `status.go` and `actions.go`.
- `mods.redactURLErr`, `ConfinePath` and `ConfineRelPath`.
- The mods ledger comment in `manifest.go`.
- The operator's agent container and its volume mounts: `gameserver_controller.go:2095-2200` and `gameserver_rcon.go:110-141`.

Library code I read to see what ends up in a dial error's text:
- coder/websocket `v1.8.15` `dial.go`.
- `net/http` `stripPassword` (`client.go:1064-1070`) and `url.Error.Error` in the local Go 1.27 toolchain.

I also checked `held/verification-netguard.md` and `audit/findings.md` for duplicates. `go build ./...` in `agent/` passes. I ran no tests or linters and nothing against a cluster.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| H-agent-01 | kept | S3 | Confirmed by reading the code. WebRcon puts the escaped secret in the URL path. When the handshake request fails, the error text includes that URL: `stripPassword` redacts userinfo only. The console handler forwards `err.Error()` to the client, and three other handlers write the same text to the agent log. The `rust` module (`rcon.protocol: websocket`) reaches this whenever its RCON listener is down, for example during startup. Only users who already have console access to that server, or who can read the agent log, see the text. But the RCON secret is a separate credential, and the code comments promise never to reflect it. So S3. |
| H-agent-02 | kept | S3 | Confirmed by reading the code. `upload` confines only the target directory. `savePart` opens `dir/<base name>` with `os.Create`, which follows a symlink that already exists at that name. `/files/write` and the mods upload check the full path. The precondition is an out-of-root symlink already in the data volume, which the file API and the mods archive handling can't create. The agent's read-only root filesystem also limits where such a write can land. So S3 rather than S1. |
| H-agent-03 | kept | S4 | All six spec statements confirmed as contradicted by the code. Items 1, 2 and 4 fail closed, or the code is the safer side. Item 3 is a promise with no enforcement behind it. Item 5 is a startup-validation gap that affects only hand-run agents. Item 6 is a description error. Overlap: the `SECURITY_AUDIT.md:155` part of item 1 is the same line that H-netguard-01 corrects (kept in `held/verification-netguard.md`), so fix them together. The `agent/specs.md:153` part is new. |

### H-agent-01

**Location**: `agent/internal/rcon/websocket.go:283-285` (the URL is built with the escaped secret as its path) and `:303-307` (a dial failure is wrapped with `%w`); `agent/internal/console/console.go:66-70` (`err.Error()` becomes the `err` envelope body). Logging sites for the same error: `agent/internal/players/players.go:133`, `agent/internal/status/status.go:119`, `agent/internal/actions/actions.go:160`.

**Control**: RCON error text never leaves the agent, because it can carry addresses or secrets. `players.go:131-132` and `actions.go:157-159` state this rule, and both handlers reply with a generic "upstream unavailable". The console handler does not follow the rule.

**Repro / observation** (by reading master):
1. The WebRcon protocol takes the secret in the URL path. `ensureLocked` builds `ws://<host>:<port>/<url.PathEscape(secret)>` (`websocket.go:283-285`).
2. When the handshake request fails at the transport level (the game's RCON listener isn't up yet, or the game is restarting), `http.Client.Do` returns a `*url.Error`. Its text is `Op "URL": err`, and its URL has passed only through `stripPassword`, which redacts userinfo and leaves the path. coder/websocket wraps this error (`dial.go:223-225`, with the "failed to WebSocket dial" prefix), and `websocket.go:307` wraps it again with `%w`.
3. `console.serve` sends `err.Error()` as the body of the `err` envelope (`console.go:66-70`). The API's console proxy forwards agent frames unchanged (`api/internal/ws/dialer.go:224-227`). The players, status and actions handlers keep the text out of their responses, but log it with `slog`.
4. The shipped module that uses this protocol is `rust` (`rcon.protocol: websocket`). Its console mode is `rcon`, so the Console tab is shown and talks to the agent.

**Expected**: The RCON secret never appears in an error string. The WebRcon client redacts the URL path from dial errors, just as `mods.redactURLErr` (`mods.go:751-760`) strips query strings. The console handler returns a generic message for transport failures, as `players.go` and `actions.go` already do, and logs detail only after redaction.

**Actual**: The dial error text includes the path-escaped secret. The console handler sends it to the client, and three handlers log it.

**How a maintainer confirms it holds**: Add a unit test in `agent/internal/rcon` that points `NewWebSocket` at a closed local port with a `PassFn` returning a unique sentinel, and assert that neither the sentinel nor its `url.PathEscape` form appears in the error text. Add a `console` test with a fake `Rcon` whose error contains a sentinel, and assert that the `err` envelope doesn't carry it. Neither test exists today (`websocket_test.go`, `console_test.go`).

### H-agent-02

**Location**: `agent/internal/files/files.go:237-245` (only the upload directory goes through `resolve`) and `:302-311` (`savePart` joins `filepath.Base(filename)` onto that directory and calls `os.Create`).

**Control**: Every file operation is confined to `--data-root`, and symlinks can't lead outside it. `agent/specs.md:14` ("reject path traversal and symlink escape"), `:201` ("all I/O is confined") and `SECURITY_AUDIT.md:150` all state this. `/files/write` enforces it on the full path, because `resolve` runs `EvalSymlinks` on an existing target and rejects anything outside the root (`files.go:71-74`). The mods upload enforces it through `ConfinePath` (`mods.go:362`).

**Repro / observation** (by reading master):
1. `upload` calls `resolve` on the directory from `?path=` (`:237`) and never on the final path.
2. `savePart` removes directory components with `filepath.Base` and rejects `.` and `..`. It then calls `os.Create(dir/name)`, which opens with O_CREAT|O_TRUNC and follows a symlink that already exists at that name. The resolved target is never compared with the root.
3. The precondition is a symlink in the data volume that points outside the root. The file API has no way to create one. The mods archive handling rejects symlink entries (`mods.go:588-598`). So the link would have to come from another writer of the volume, such as the game container.
4. The agent container runs with a read-only root filesystem (`gameserver_controller.go:2101,2185`), and its only other mounts are the ones `agentVolumeMounts` adds (`gameserver_rcon.go:125-141`). That limits the reach of such a write.

**Expected**: The upload destination gets the same full-path confinement as `/files/write` (`resolve` or `ConfinePath` on `dir/name`), and/or the part is written to a temporary file and renamed into place. `rename` replaces a link instead of following it. The same temp-and-rename change also fixes the notes candidate C-agent-01.

**Actual**: The final upload path is never confined, and an existing symlink at that name is followed.

**How a maintainer confirms it holds**: Add a unit test in `agent/internal/files`. Create a root that contains a symlink to a file outside the root, then POST a multipart upload whose part filename matches the link's name. Assert a 4xx response, and that the outside file's content is unchanged.

### H-agent-03

**Location**: `agent/specs.md:92`, `:112-114`, `:144`, `:152`, `:153`, `:199`, `:201`, `:205`; `SECURITY_AUDIT.md:155`.

**Control**: `agent/specs.md` and `SECURITY_AUDIT.md` are the documented description of the agent's security controls: authentication, egress policy and path confinement. Reviewers and operators rely on them.

**Repro / observation** (by reading master; each item gives the spec text, then what the code enforces):
1. `specs.md:153`: WebRcon dials "using `netguard.IsPublic()` … cannot reach private/loopback addresses". The code uses `netguard.IsAllowed` (`websocket.go:302`), which permits loopback and private ranges. The code is right: the agent always dials the in-pod game at the `--rcon-host` default `127.0.0.1` (`main.go:65`), and `IsPublic` would refuse it. `SECURITY_AUDIT.md:155` says "`IsAllowed` (operator) and `IsPublic` (agent)". The agent uses both: `IsPublic` for mod downloads and `IsAllowed` for WebRcon.
2. `specs.md:112-114`, `:144` and `:205` call `/healthz` and `/metrics` "Public (unauthenticated)". The operator always passes `--tls-cert/--tls-key/--tls-client-ca` (`gameserver_controller.go:2104-2106`). `auth.ServerTLS` then sets `ClientAuth: tls.RequireAndVerifyClientCert` (`auth.go:111`), which requires a client certificate at the handshake for every path. This fails closed. The agent container has no probes that would depend on it (`gameserver_controller.go:2174-2190`). The failed PodMonitor scrape that follows from it is tracked as C-charts-gameplane-05. Notes candidate C-agent-07 raised the same thing and was rejected as a duplicate.
3. `specs.md:201`: the `files` package "rejects … dotfile access". `resolve` (`files.go:57-98`) has no dotfile rule, so list, read, write and delete all accept dot-prefixed names. That includes the mods ledger `.gameplane-mods.json`, whose comment (`manifest.go:12-14`) says it is "out of reach of client-supplied names". That holds only for the mods API, where `safeName` rejects dotfiles.
4. `specs.md:199`: private registries "are rejected unless explicitly whitelisted". Mod downloads dial through `netguard.IsPublic` (`mods.go:843,850`), whatever `allowedHosts` contains, and nothing can override that. The code is stricter than the sentence. `docs/security.md:343-350` describes the real behaviour correctly.
5. `specs.md:92`: `--tls-cert` "if set, requires `--tls-key` and enables HTTPS + mTLS". TLS is enabled only when both flags are set (`main.go:185`). With `--tls-cert` alone, the listener starts on plain HTTP without an error. In mTLS mode this fails closed: every protected route answers 401, because `req.TLS` is nil (`auth.go:66-70`). Token mode is documented for plain HTTP anyway. The operator always passes both flags, so only hand-run agents are affected.
6. `specs.md:152`: `ConfinePath` is "the single point of validation for all filesystem operations on untrusted paths", including archive extraction. The `files` package uses its own `resolve` (`files.go:57`), and archive extraction uses `ConfineRelPath` (`mods.go:575-579`, `confinement.go:158`).

**Expected**: `agent/specs.md` and `SECURITY_AUDIT.md` state what the code enforces. For items 3 and 5, the maintainer decides whether the spec wording is the intended contract and the code should change instead: rejecting dotfiles in `resolve`, or refusing to start when `--tls-cert` is given without `--tls-key`.

**Actual**: The six statements above don't match the code.

**How a maintainer confirms it holds**: Read the cited lines against the corrected text. For item 2: a TLS handshake to an agent pod without a client certificate is refused, and one with the API's client certificate succeeds. For item 3, if the dotfile rule is kept as the contract: a unit test that reads or writes `/.gameplane-mods.json` through `/files/*` expects 4xx. For item 5, if the spec wording is kept: a startup test that passes `--tls-cert` without `--tls-key` expects a non-zero exit.
