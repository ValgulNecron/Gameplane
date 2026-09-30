# Held review candidates: agent (OD-019, off-git)

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `agent/specs.md` (lines 12-14, 59, 92, 110-114, 144-153, 196-206); `SECURITY_AUDIT.md:149-155`; `docs/security.md:159-167,335-351`
- **Companion notes**: `audit/evidence/review-agent/notes.md` (which refers here only as "held candidates: 3")

These candidates concern the agent's secret handling, path confinement, authentication and egress-guard descriptions. They are written defensively: the control, where it lives, what correct behaviour is, and how a maintainer confirms it holds. Everything below comes from reading the code and library sources. Nothing was run against a cluster.

## Candidate findings

### H-agent-01: RCON secret material can reach the error text sent to console users and written to agent logs (WebRcon dial path)

- **Location**: `agent/internal/rcon/websocket.go:284-307` (the password is placed in the URL path; a dial failure is wrapped with `%w`); `agent/internal/console/console.go:66-70` (`err.Error()` is forwarded to the WebSocket client as an `err` envelope); logging sites that record the same error: `players.go:133`, `status.go:119`, `actions.go:160`
- **Category**: correctness (secret handling)
- **Suggested severity**: S3. Console users already hold `servers:console`, but the RCON secret is a separate credential that shouldn't leave the agent.
- **Control and its stated promise**:
  - `players.go:131-132`: "err can contain RCON protocol detail (addresses, passwords from poorly-written server mods, etc.). Never reflect it."
  - `actions.go:157-159`: "RCON errors can echo addresses/passwords … never reflect them to the client."
  - The console handler doesn't follow the same rule.
- **Observation**:
  1. The WebRcon protocol requires the password in the URL path. `ensureLocked` builds `ws://host:port/<escaped password>` (`websocket.go:284-285`).
  2. When the handshake request fails (for example, the game is restarting or RCON isn't listening yet), `net/http` returns a `*url.Error` that includes the request URL. `stripPassword` (`net/http/client.go:1064-1070`) redacts only userinfo, not the path. coder/websocket wraps that error (`dial.go:223-225`), and `websocket.go:307` wraps it again with `%w`.
  3. `console.go:68-69` sends `err.Error()` verbatim to the client. The players, status and actions handlers log it through `slog`.
- **Correct behaviour**: The RCON secret never appears in an error string. The WebRcon client redacts the URL path from dial errors (the same idea as `mods.redactURLErr`, `mods.go:751-760`, which strips query-string credentials), and the console handler returns a generic message for transport failures while logging detail only after redaction.
- **How a maintainer confirms it holds**: Add a unit test in `agent/internal/rcon` that points `NewWebSocket` at a closed local port, uses a `PassFn` returning a unique sentinel, and asserts the sentinel doesn't appear in the returned error's text. Add a matching `console` test with a fake `Rcon` whose error contains a sentinel, asserting the sentinel isn't forwarded in the `err` envelope.

### H-agent-02: `/files/upload` doesn't confine the per-part destination name when a symlink already exists at that name

- **Location**: `agent/internal/files/files.go:237-245` (only the target directory goes through `resolve`); `files.go:302-311` (`savePart` joins `filepath.Base(filename)` onto the directory and calls `os.Create`, which follows an existing symlink at that name)
- **Category**: correctness (path confinement)
- **Suggested severity**: S3. It requires a symlink that already exists in the data volume and points outside `--data-root`. The file API can't create one, but the game container, or anything running inside it, can.
- **Control and its stated promise**:
  - `specs.md:14`: "reject path traversal and symlink escape."
  - `specs.md:201`: "rejects … symlinks escaping `--data-root` … all I/O is confined."
  - `specs.md:152`: ConfinePath is "the single point of validation for all filesystem operations on untrusted paths."
  - `SECURITY_AUDIT.md:150`: "file access is confined to the server's data directory … Symlinks can't lead outside it."
- **Observation**: `resolve` (`files.go:57-98`) validates the upload directory, but never the final path. `savePart` opens `dir/name` with `os.Create` without checking whether `name` is an existing symlink, so the write lands at the link's target, which is never compared with the root. Other paths were checked and are not affected: `/files/write` resolves the full path and rejects an out-of-root target (`files.go:71-74`), and the mods upload goes through `ConfinePath` (`mods.go:362`).
- **Correct behaviour**: The upload destination gets the same full-path confinement as `/files/write` (`resolve` or `ConfinePath` on `dir/name`), and/or the write goes through a dot-temp file plus `rename`. `rename` replaces a link rather than following it, and the same temp-plus-rename change also fixes notes finding C-agent-01.
- **How a maintainer confirms it holds**: Add a unit test in `agent/internal/files`. Create a root containing an entry that is a symlink to a file outside the root. POST a multipart upload whose part filename equals that entry's name. Assert a 4xx response and that the outside file's contents are unchanged.

### H-agent-03: Security statements in `specs.md` that the code contradicts

- **Location and observations**: each item quotes the spec, then states what the code does.
  1. `specs.md:153`: "WebRcon dials through netguard … using `netguard.IsPublic()` … cannot reach private/loopback addresses." The code uses `netguard.IsAllowed` (`websocket.go:294-302`), which permits loopback and private addresses. The code is right: the operator always targets the in-pod game at `127.0.0.1` (`main.go:65`), so `IsPublic` would break Rust RCON. The spec sentence, and `SECURITY_AUDIT.md:155`'s "`IsAllowed` (operator) and `IsPublic` (agent)", need to say that the agent uses both policies. The netguard held review (H-netguard-01) already lists "GameServer RCON address" as an `IsAllowed` caller.
  2. `specs.md:112-114,144,205`: `/healthz` and `/metrics` are "Public (unauthenticated)". Whenever TLS is enabled (always, for operator-built pods: `gameserver_controller.go:2104-2107`), `auth.ServerTLS` sets `tls.RequireAndVerifyClientCert` (`auth.go:108-113`). That requires a client certificate at the handshake, for every path. This fails closed (stricter than documented). The functional side, the PodMonitor scrape, is notes finding C-agent-07: no probe or scraper without an API client certificate can reach these paths.
  3. `specs.md:201`: the `files` package "rejects … dotfile access". `resolve` has no dotfile rule: `/files/list`, `/files/read`, `/files/write` and `/files/delete` all accept dot-prefixed names, for example the mods ledger `.gameplane-mods.json`. That file's own comment (`manifest.go:12-14`) says it is "out of reach of client-supplied names", which is true only for the mods API.
  4. `specs.md:199`: "Private registries on loopback or non-routable addresses are rejected unless explicitly whitelisted." `IsPublic` applies at dial time whatever `allowedHosts` contains (`mods.go:843,850`), and there is no whitelist override. The code is stricter than the sentence. `docs/security.md:343-350` describes the actual behaviour correctly.
  5. `specs.md:92`: `--tls-cert` "if set, requires `--tls-key` and enables HTTPS + mTLS." `main.go:185` enables TLS only when both are set. `--tls-cert` alone starts the listener on plain HTTP without an error, so in token mode the bearer token travels in cleartext. The operator always passes both flags, so only hand-run agents are affected.
  6. `specs.md:152`: ConfinePath is "the single point of validation for all filesystem operations on untrusted paths", and "archive extraction … route[s] through ConfinePath". The `files` package uses its own `resolve` (`files.go:57`), and archive extraction uses `ConfineRelPath` (`mods.go:579`).
- **Category**: docs-drift (security controls)
- **Suggested severity**: S4. Items 1, 2 and 4 are fail-closed, or the code is the safer side. Item 3 is a spec promise with no enforcement behind it. Item 5 is a startup-validation gap for manual configurations.
- **Correct behaviour**: `specs.md` states what the code enforces. For items 3 and 5, the maintainer decides whether the spec (dotfile rejection; refusing `--tls-cert` without `--tls-key`) is the intended contract and the code should change instead.
- **How a maintainer confirms it holds**: Read the cited lines. For item 2, check that a TLS handshake to the agent without a client certificate is refused while one with the API's client certificate succeeds. For item 5, a startup test that passes `--tls-cert` without `--tls-key` and expects a non-zero exit, if the spec wording is kept.

## Questions (not findings)

- None beyond the notes file.
