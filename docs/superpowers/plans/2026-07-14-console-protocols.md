# Console Protocols Implementation Plan (Phase 2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Add three remote-console protocols (`websocket`, `battleye`, `satisfactory`) so five more games get a console, players list, quick actions, backup quiesce and graceful stop — plus two free module fixes that need no new code.

**Architecture:** `agent/internal/rcon/` already implements two protocols behind one interface, `Exec(cmd string) (string, error)`. Every consumer (console, players, quiesce, lifecycle, status, actions, heartbeat) depends only on that interface. So each new protocol is: one new file implementing `Exec`, one CRD enum value, one `switch` case in `agent/cmd/main.go`. Nothing downstream changes.

**Tech Stack:** Go 1.25, `coder/websocket` v1.8.12 (already a direct agent dependency — it serves the console WS), stdlib `net` (UDP), `hash/crc32`, `crypto/tls`, `net/http`.

## Global Constraints

- **Phase 2 of** `docs/superpowers/specs/2026-07-14-console-protocols-categories-actions-design.md`. Read §1 first, including the 2026-07-14 correction about Satisfactory's `RunCommand`.
- **Do NOT run tests or linters locally** (CLAUDE.md rule 8). CI is the source of truth. Compile checks ARE allowed and required.
- **`go vet ./...` SKIPS files behind build tags.** After touching operator test files you MUST also run `cd operator && go vet -tags=envtest ./...`. A bad import in an `*_envtest_test.go` passes the plain check and reddens CI — this has already happened once on this feature.
- **Sign commits**: `git -c commit.gpgsign=false commit -s -m "..."` (the `-c` MUST precede the subcommand).
- **Fix, don't silence** (rule 4). **Wrap errors with `%w`** (rule 6). **Regenerate after CRD edits**: `make generate && make manifests`, committed together (rule 7).
- **One branch per protocol**, merged and deleted before the next (rule 12).
- **RCON dials pod-local loopback**, so `netguard` is NOT in the path for any of these clients. Do not add dial guards here.

### The existing interface — do not change it

```go
// agent/internal/rcon — every client satisfies this, and every consumer depends on it.
type Rcon interface { Exec(cmd string) (string, error) }
```

Read `agent/internal/rcon/rcon.go` (Source) and `telnet.go` (line-based) before writing a new client. Match their structure: a `New…()` constructor taking host/port/password-fn, a mutex, lazy connect-and-auth in an `ensureLocked()`, and a `PassFn` for the password so it can be re-read from a file on every reconnect.

---

## Task 1: The two free fixes (no new code)

**Files:** `modules/garrys-mod/template.yaml`, `modules/7-days-to-die/template.yaml` (the `gameplane-module` submodule repo).

These two games have *no* console today purely through misconfiguration, not a missing protocol.

- **garrys-mod** is an srcds (Source-engine) server. srcds has spoken Source RCON natively forever. Its template says `protocol: none`.
- **7-days-to-die** has a telnet console, and `agent/internal/rcon/telnet.go` already implements telnet — but no module references it. Its template comment says it is "blocked by password management".

- [ ] **Step 1: Verify each image actually exposes a password knob**

This is the whole risk. Before editing anything, read each module's `template.yaml` to find the image it runs, then check that image's documentation/entrypoint for the relevant env var:

- garrys-mod: does the image accept an RCON password env (commonly `RCON_PASSWORD`, or an srcds `+rcon_password` arg)? What port does it expose?
- 7-days-to-die: 7DTD's `serverconfig.xml` carries `TelnetEnabled`, `TelnetPort` (default 8081), `TelnetPassword`. Does the image let you set those via env (LinuxGSM-style images often do)?

If an image does NOT expose a knob, **stop** — leave that module at `protocol: none` and record the finding in its template comment. Do not invent an env var name.

- [ ] **Step 2: Set the protocol**

garrys-mod:
```yaml
  rcon:
    protocol: source
    port: 27015
    passwordEnv: <the verified env var>
```

7-days-to-die:
```yaml
  rcon:
    protocol: telnet
    port: 8081
    passwordEnv: <the verified env var>
```

Remove the stale "blocked by password management" comment from 7-days-to-die and the `protocol: none` rationale from garrys-mod.

- [ ] **Step 3: Bump each module's version, commit in the module repo, PR, merge, bump the pointer here.**

Note 7DTD's telnet console is **unauthenticated on some builds** if `TelnetPassword` is empty — never ship it with an empty password. If the image cannot set one, leave the module at `none`.

---

## Task 2: `websocket` — Rust (Facepunch WebRcon)

**Files:**
- Create: `agent/internal/rcon/websocket.go`
- Test: `agent/internal/rcon/websocket_test.go`
- Modify: `operator/api/v1alpha1/gametemplate_types.go` (the protocol enum), `agent/cmd/main.go` (the switch)
- Module: `modules/rust/template.yaml`

**Interfaces:**
- Produces: `rcon.NewWebSocket(host string, port int, pass PassFn) *WebSocket` satisfying `Exec(cmd string) (string, error)`.

### The wire protocol (researched — do not deviate)

- **Connect:** `ws://<host>:<port>/<password>`. The password is the URL **path**, not a header and not a login message. Default port **28016** (the game itself is 28015/udp). Requires the server to run with `+rcon.web 1`.
- **Request frame** (JSON text frame):
  ```json
  {"Identifier": 42, "Message": "playerlist", "Name": "WebRcon"}
  ```
- **Response frame:**
  ```json
  {"Identifier": 42, "Message": "…output…", "Type": 3, "Stacktrace": ""}
  ```
  `Type` is an **int**, not a string.
- **Auth failure:** the WebSocket *handshake succeeds*, then the server closes the connection (observed close code 1006). Rust also rate-limits after a failed login, so do not hot-retry.
- **Commands are capped at 1000 characters.**

### The correlation hazard — read this before implementing

The server pushes **unsolicited** frames (console spam, chat, player join/leave) onto the same socket. Research could **not** confirm what `Identifier` those carry (0? -1?). Do not guess, and do not "read the next frame and return it" — that is how you return a chat line as the output of `save`.

**Design defensively:** assign a unique, monotonically increasing **positive** `Identifier` per request, then read frames in a loop and **discard every frame whose `Identifier` does not equal the one you sent**, until you match or the deadline expires. This is correct regardless of what unsolicited frames actually use, so the unconfirmed detail stops mattering.

- [ ] **Step 1: Write the failing test**

Create `agent/internal/rcon/websocket_test.go`. Stand up a fake server with `httptest` + `coder/websocket` (already a dependency; see how `agent/internal/console/console_test.go` drives a WS). Cover:

1. **Happy path** — client sends `{"Identifier":N,"Message":"save","Name":"WebRcon"}`; server replies with the same `Identifier` and a body; `Exec` returns that body.
2. **Interleaved unsolicited frames** — server first sends two frames with a DIFFERENT `Identifier` (use `0` and `-1`) and only then the matching one. `Exec` must return the matching body, NOT the spam. **This is the regression test for the correlation hazard; it is the most important test in the file.**
3. **Password in the path** — assert the fake server saw `r.URL.Path == "/s3cret"`.
4. **Auth failure** — server accepts the handshake then immediately closes. `Exec` must return an error, not hang.
5. **Timeout** — server accepts but never replies. `Exec` must return an error within the deadline.
6. **Concurrent Exec calls** get distinct Identifiers and each gets its own reply (guard with a mutex like the other clients; a serialized implementation is fine — just prove it doesn't cross wires).

- [ ] **Step 2: Verify it fails** — `cd agent && go vet ./internal/rcon/` → `undefined: NewWebSocket`.

- [ ] **Step 3: Implement `websocket.go`**

Mirror `telnet.go`'s shape: struct with `host`, `port`, `passFn`, a `sync.Mutex`, a lazily-established `*websocket.Conn`, and an atomic counter for the Identifier. `ensureLocked()` dials `ws://host:port/<password>` (URL-escape the password path segment) and, because Rust closes rather than 401s on bad auth, treats an early close as an auth error. `Exec` marshals the request, writes a text frame, then loops reading frames until `Identifier` matches or the context deadline fires. Reject commands over 1000 chars with a clear error. Close and nil the conn on any error so the next `Exec` reconnects.

- [ ] **Step 4: Verify** — `cd agent && go vet ./...` passes.

- [ ] **Step 5: Wire the protocol in**

`operator/api/v1alpha1/gametemplate_types.go`, the `RCONSpec.Protocol` marker — add `websocket`:
```go
// +kubebuilder:validation:Enum=source;telnet;websocket;none
```
(Add `battleye` and `satisfactory` in their own tasks, not now — one protocol per PR.)

Update the field's doc comment to describe the new protocol. Then `make generate && make manifests`.

`agent/cmd/main.go`, in the protocol switch:
```go
case strings.EqualFold(rconProtocol, "websocket"):
    rconClient = rcon.NewWebSocket(rconHost, rconPort, rcon.PasswordFromFile(rconPassFile))
```
Also update the `-rcon-protocol` flag's help text.

- [ ] **Step 6: Compile-check everything**

```sh
cd agent && go vet ./...
cd operator && go vet ./... && go vet -tags=envtest ./...
```

- [ ] **Step 7: Adopt it in the rust module**

`modules/rust/template.yaml` — currently `protocol: none` with `consoleMode: pty`:
```yaml
  rcon:
    protocol: websocket
    port: 28016
    passwordEnv: RCON_PASSWORD   # verify against the image the module actually runs
  consoleMode: rcon
```
Verify the image's env var name and that it passes `+rcon.web 1`. Rust's `playerlist` returns **JSON**, so `capabilities.players` can parse it properly — check how `agent/internal/players/` expects to extract players and wire the list command accordingly. Add quick actions (`say "…"`, `server.save`, `kick`, `ban`, `quit`) in Phase 4, not here.

- [ ] **Step 8: Commit, PR, watch CI to green, merge, delete the branch.**

---

## Task 3: `battleye` — DayZ (BattlEye RCon)

**Files:**
- Create: `agent/internal/rcon/battleye.go`
- Test: `agent/internal/rcon/battleye_test.go`
- Modify: the protocol enum, `agent/cmd/main.go`
- Module: `modules/dayz/template.yaml`

**Interfaces:**
- Produces: `rcon.NewBattlEye(host string, port int, pass PassFn) *BattlEye` satisfying `Exec`.

**This is the hardest of the three** — UDP, checksums, sequence numbers, mandatory keepalive, and a background goroutine. Budget accordingly. Official spec: <https://www.battleye.com/downloads/BERConProtocol.txt>.

### The wire protocol (researched — do not deviate)

**Transport:** UDP. Default port **2305**, separate from the game port. Configured in `beserver_x64.cfg` (`RConPort`, `RConPassword`; password max 24 chars).

**Packet layout** (every packet):
```
0x42 0x45              'B' 'E'
<4 bytes>              CRC32, LITTLE-ENDIAN
0xFF                   terminator
<1 byte>               packet type
<payload…>
```
The **CRC32 is computed over the bytes from the `0xFF` onward** (i.e. `0xFF` + type + payload) — NOT over the whole packet, NOT over just the payload. Standard IEEE CRC32 (`hash/crc32.ChecksumIEEE`, poly 0xEDB88320). Getting this range wrong is the single most likely bug; the test must pin it.

**Packet types:**
- `0x00` **login** — payload is the plaintext password. Server replies `0x00` + `0x01` (success) or `0x00` + `0x00` (failure).
- `0x01` **command** — payload is a 1-byte sequence number then the ASCII command. Reply echoes `0x01` + the same sequence + the response text. Sequence wraps 0xFF → 0x00.
- `0x02` **server message** — the server pushes async events (chat, connects, kills). The client **MUST acknowledge each one within ~10 seconds** by sending back `0x02` + that message's sequence byte. Miss the ack and the server retries 5 times, then **drops the connection**.

**Multi-packet replies:** a fragmented command reply's payload is `seq`, then `0x00`, then `total-pages`, then `page-index`, then the chunk. Pages can arrive **out of order** — reassemble by index, do not just concatenate. A single-packet reply has no such header, so you must distinguish them.

**Keepalive:** the client must send an empty command packet (type `0x01` + seq, no command text) at least every **45 seconds** or the server drops it. Use a ~30s ticker in a background goroutine.

- [ ] **Step 1: Write the failing test**

Create `agent/internal/rcon/battleye_test.go` with a fake UDP server (`net.ListenPacket("udp", "127.0.0.1:0")`). Cover, in this order of importance:

1. **CRC32 is computed over the right byte range.** Build a known packet by hand, assert the exact bytes the client puts on the wire. This pins the one detail most likely to be wrong. Do NOT write this assertion by calling the client's own checksum helper — that would be tautological. Hard-code the expected CRC of a known payload.
2. **Login success and failure** — server replies `0x00 0x01` vs `0x00 0x00`; the latter must surface as an auth error.
3. **Single-packet command reply** — `Exec("players")` returns the text.
4. **Multi-packet reply, delivered OUT OF ORDER** — server sends page 1 then page 0; `Exec` must reassemble in index order, not arrival order.
5. **Server message is acked** — server pushes a `0x02` message; assert the client sends back `0x02` + the same sequence. If it doesn't, a real server would drop us.
6. **Keepalive fires** — with a short interval injected, assert an empty `0x01` packet arrives. (Make the interval a struct field so the test can shorten it; do not `time.Sleep(45s)` in a test.)

- [ ] **Step 2: Verify it fails** — `cd agent && go vet ./internal/rcon/` → `undefined: NewBattlEye`.

- [ ] **Step 3: Implement `battleye.go`**

A `sync.Mutex`-guarded struct over a `*net.UDPConn`, a sequence counter, a map of in-flight multi-packet reassembly buffers, and a background goroutine that (a) reads all inbound packets, (b) acks type `0x02` immediately, (c) routes type `0x01` replies to the waiting `Exec` by sequence, and (d) ticks the keepalive. `Exec` registers a channel for its sequence, writes the command packet, and waits with a deadline.

UDP is lossy: if no reply arrives within a short window, retransmit the command once before giving up. Document that choice in a comment.

- [ ] **Step 4–6: Compile-check, wire the enum + switch (`battleye`), regenerate, adopt in `modules/dayz/template.yaml`** (`protocol: battleye`, `port: 2305`, verify the image's password knob — DayZ carries it in `BEServer_x64.cfg`, so this may need `passwordFile` rather than `passwordEnv`).

- [ ] **Step 7: Commit, PR, CI green, merge, delete branch.**

---

## Task 4: `satisfactory` — Satisfactory HTTPS API

**Files:**
- Create: `agent/internal/rcon/satisfactory.go`
- Test: `agent/internal/rcon/satisfactory_test.go`
- Modify: the protocol enum, `agent/cmd/main.go`
- Module: `modules/satisfactory/template.yaml`

**Interfaces:**
- Produces: `rcon.NewSatisfactory(host string, port int, pass PassFn) *Satisfactory` satisfying `Exec`.

### The wire protocol (researched — note the spec correction)

**Endpoint:** `POST https://<host>:7777/api/v1`, `Content-Type: application/json`. Same port as the game.

**TLS:** the server generates a **self-signed** certificate by default. We dial pod-local loopback (the game container in our own pod), so `InsecureSkipVerify: true` is defensible — but say so in a comment, and scope it to this client only. Do not add a global TLS opt-out.

**Auth:**
```json
{"function": "PasswordLogin", "data": {"MinimumPrivilegeLevel": "Administrator", "Password": "<pw>"}}
```
returns a bearer token, then every later call carries `Authorization: Bearer <token>`.

**Exec maps onto `RunCommand` directly** — this corrects the original spec, which wrongly assumed no free-text console existed:
```json
{"function": "RunCommand", "data": {"command": "<cmd>"}}
  → {"data": {"CommandResult": "<output>"}}
```

**Errors** come back as `{"errorCode": …, "errorMessage": …}`; `401`/`403` mean the token is bad or under-privileged.

**Known limitation — no player enumeration.** `QueryServerState` returns `NumConnectedPlayers` (a count) and there is **no** documented way to list player names or IDs. The satisfactory module therefore must **NOT** declare `capabilities.players`, or the dashboard ships a players tab that can never populate. Record this in the template comment.

- [ ] **Step 1: Write the failing test**

Create `agent/internal/rcon/satisfactory_test.go` using `httptest.NewTLSServer` (its cert is self-signed, which exercises the exact path we need). Cover:

1. **Login then command** — first request is `PasswordLogin`; assert the client then sends `Authorization: Bearer <token>` on the `RunCommand` call and returns `CommandResult`.
2. **Token is reused** — two `Exec` calls produce exactly ONE login request. (Assert a counter; re-logging-in per command would hammer the server.)
3. **401 triggers exactly one re-login and retry**, then succeeds. Assert it does not loop forever on a persistent 401.
4. **Bad password** — login returns an error body; `Exec` surfaces an auth error.
5. **`{"errorCode":…}` response body** surfaces as an error, not as command output.

- [ ] **Step 2–4: Verify red, implement, compile-check.**

Struct holds the base URL, a `*http.Client` with a `tls.Config{InsecureSkipVerify: true}` transport, the cached token, and a mutex. `Exec` lazily logs in, calls `RunCommand`, and on a `401` re-logs-in once and retries.

- [ ] **Step 5: Wire the enum + switch (`satisfactory`), regenerate.**

- [ ] **Step 6: Adopt in `modules/satisfactory/template.yaml`** — currently `protocol: none`, `consoleMode: none`:
```yaml
  rcon:
    protocol: satisfactory
    port: 7777
    passwordEnv: <the image's admin-password env>
  consoleMode: rcon
```
Do **not** add `capabilities.players`. Verify the image's admin-password env var.

- [ ] **Step 7: Commit, PR, CI green, merge, delete branch.**

---

## Task 5: Documentation

- [ ] Update `docs/module-authoring.md` — the `rcon.protocol` section now lists five protocols. For each: which games it suits, the default port, and how the password is supplied. Call out that `satisfactory` cannot enumerate players.
- [ ] Update `CHANGELOG.md` under Unreleased. Note that a **previous** entry there records `rcon.protocol` narrowing from `source;telnet;http;none` to `source;telnet;none` because `http` never had an implementation — this phase re-widens it, and the new `satisfactory` value is the honest, implemented version of what `http` gestured at. Say so, so the two entries don't read as contradictory.

---

## Self-Review

**Spec coverage (§1):** websocket/rust (Task 2), battleye/dayz (Task 3), satisfactory (Task 4), the garrys-mod + 7dtd free fixes (Task 1), enum widened to `source;telnet;websocket;battleye;satisfactory;none` (Tasks 2–4, one value per PR), docs (Task 5). No gaps.

**Risks, stated honestly:**
- **Rust's unsolicited-frame Identifier is unconfirmed.** Mitigated by design (discard non-matching Identifiers) rather than by knowledge, so it cannot bite us. Test 2 in Task 2 is the guard.
- **BattlEye's CRC byte range is the likeliest bug.** Pinned by a hard-coded-expectation test, not a tautological one.
- **The image password knobs for garrys-mod, 7dtd, rust, dayz and satisfactory are all unverified.** Every task says: verify against the real image, and if there is no knob, leave the module at `none` rather than invent an env var. These get smoke-tested on kubelab before the module bundles ship.
- **Satisfactory has no player list.** Design accommodates it (no `capabilities.players`) rather than pretending.
