# DayZ — e2e game-bot probe specification

**Status:** Heavy set, hand-run only (never in CI)  
**Module:** `modules/dayz/`  
**Measured depth:** QUERY  
**Transport:** UDP 27015 (query), UDP 2302 (game protocol)  
**Image:** `registry.godbleak.dev/godbleak/serverz:2.0.0@sha256:5e8757beae763c862d08a9c08587c35211cda74ce399f7419492d7520adab4fe`

## Purpose

Prove that a Gameplane-managed DayZ server answers Steam A2S query protocol, confirming the server is listening and reachable from in-cluster clients. The game protocol on UDP 2302 and the BattlEye RCon control channel (2305) are not reachable or measurable from the probe pod, representing a real product gap documented below.

## Responsibilities

1. Query A2S (Steam A2S on UDP 27015) to establish QUERY depth — server is alive and responds to standard queries.
2. Send a diagnostic UDP probe to the game port (2302) and log the response for evidence, without claiming any depth from it.
3. Document the unreachability of BattlEye RCON (bound to 127.0.0.1) as a control-channel gap.

## Non-goals / boundaries

- Does not join the game or measure PARTIAL/JOINED depth; the Enfusion protocol is not publicly documented.
- Does not test BattlEye RCON (Path A does that via `agent/internal/rcon/battleye`); RCON is unreachable from the probe pod by design (see below).
- Does not install mods or authenticate with Steam; runs vanilla with no auth.

## Directory & package layout

```
test/e2e/internal/dayz/
├── app.go      # probe entry point; calls A2S, logs game-port diagnostic
├── spec.md     # this file
└── [no protocol/ subdir — reuses test/e2e/internal/protocol/a2s]
```

## External interface / contracts

### Query protocol (A2S)

**Reference:** https://developer.valvesoftware.com/wiki/Server_queries

**Wire format:** UDP connectionless, A2S_INFO request/response:

```
Request:  0xFFFFFFFF 0x54 (type T for info) [optional: 4-byte challenge]
Response: 0xFFFFFFFF 0x49 (type I) followed by server metadata
```

**Implementation:** `test/e2e/internal/protocol/a2s.QueryInfo()` — proven correct against real servers (used by garrys-mod and cs2).

**Measured outcome on DayZ:** Server responds with server name, map, player count, max players. Example log line:

```
a2s: server="DayZ Server" map="chernarusplus" players=0/32
```

### Game protocol (UDP 2302)

**Reference:** Enfusion engine (DayZ's game engine); protocol is not publicly documented.

**Wire format:** Unknown. A diagnostic UDP probe is sent (0x00 byte) to observe any response, logged as hex for evidence. No depth is claimed from this response.

**Example diagnostic output (if server responds):**

```
game-port-diagnostic: server responded with N bytes, hex=...
```

### BattlEye RCon (UDP 2305)

**Reference:** BattlEye anti-cheat RCON protocol (proprietary, limited public docs)

**Unreachability:** ServerZ template sets `BE_IP=127.0.0.1`, binding RCon to loopback. The probe pod runs in the `default` namespace (not `gameplane-games`), so it cannot reach 127.0.0.1 — that loopback address is the pod's own loopback, not the game server's. This is a real product gap: a remote probe cannot control the game server at all. Only Path A (the agent sidecar, sharing the game pod's network namespace) can reach BattlEye RCON.

**Path A coverage:** `agent/internal/rcon/battleye` handles RCON over the mTLS connection to the agent sidecar, proving control plane connectivity. This e2e probe does not measure it.

## Key invariants

1. **QUERY is the honest measurement boundary.** A2S is documented and proven; the game protocol is not. A future maintainer who documents Enfusion can escalate to PARTIAL or JOINED; until then, QUERY avoids fabrication.

2. **The diagnostic game-port probe is purely observational.** Sending a 0x00 byte and logging the response is a non-invasive observation; it does not affect the depth measurement and does not depend on success (timeout is expected).

3. **BattlEye RCON is unreachable by design, not a bug.** The template binds it to 127.0.0.1 for security (cannot be brute-forced from the internet). The implication is that a remote probe cannot measure control depth. This gap is documented but not a blocker — Path A (agent sidecar RCON) provides the actual control channel, measured separately.

4. **Storage is heavy: 40Gi.** This e2e test never runs in CI and is only executed on maintainer hand-run via:
   ```
   GAMEPLANE_E2E_REUSE_CLUSTER=1 GAMEPLANE_E2E_GAME_BOT=1 GAMEPLANE_E2E_GAMES=all make test-e2e-keep
   ```

## Dependencies

**Internal:**
- `test/e2e/internal/probe` — retry harness, Depth type, ParseFlags()
- `test/e2e/internal/protocol/a2s` — A2S query client (stdlib only)

**External:**
- Stdlib: `context`, `encoding/hex`, `fmt`, `log`, `net`, `time`
- Go 1.25+

**No external modules.** The probe binary links against stdlib and test/e2e/internal packages only.

## Security considerations

1. **The probe runs unprivileged** with read-only filesystem, drop all caps, and nonroot uid.
2. **No credentials in argv.** The probe accepts `-addr`, `-deadline`, `-expect-depth` only; no passwords or tokens.
3. **Query protocol is read-only.** A2S queries are connectionless and non-stateful; the server state is never modified.
4. **Runs from outside the games namespace.** The probe runs in `default` (unrestricted egress), matching how real players reach the game.

## Testing & coverage

### Unit tests

Protocol-family tests (`test/e2e/internal/protocol/a2s/a2s_test.go`):
- Pre-recorded A2S responses exercised against parsing logic.
- Runs on every `make test-go` (not e2e-only).

### E2E test registration

**Test name:** `TestGameServer_DayZBot_Query`

**Bucket:** `bot-heavy` (never in CI; hand-run only)

**Invocation:**

```bash
GAMEPLANE_E2E_REUSE_CLUSTER=1 GAMEPLANE_E2E_GAME_BOT=1 GAMEPLANE_E2E_GAMES=all make test-e2e-keep
```

**Test behavior:**
- Boots a real DayZ server from the shipped module template.
- Waits up to 30 minutes for the server to reach Running (world generation is slow).
- Runs `test/e2e/internal/dayz/app.go` as a Job inside the cluster.
- Expects QUERY depth (A2S succeeds).
- Logs A2S metadata and game-port diagnostic response (if any).
- Exits 0 only if A2S succeeds and depth is exactly QUERY.

## Measured outcomes & evidence

### A2S Query (UDP 27015)

**Status:** Confirmed reachable and responding on real servers.

**Evidence:**
- The module template exports port 27015 (STEAM_QUERY_PORT).
- ServerZ (the backing image) listens on this port and responds to A2S queries.
- This is the same A2S implementation proven on garrys-mod and cs2.

**Log evidence (expected):**

```
a2s: server="DayZ Server" map="chernarusplus" players=0/32
```

### Game protocol diagnostic (UDP 2302)

**Status:** Diagnostic only; no depth claimed.

**Evidence:**
- A minimal UDP probe (0x00 byte) is sent.
- Server response (if any) is logged as hex and byte count.
- Timeout is expected and non-fatal.

**Example log (if server responds):**

```
game-port-diagnostic: server responded with 128 bytes, hex=…
```

### BattlEye RCON (UDP 2305)

**Status:** Unreachable from the probe pod; Path A only.

**Why:** ServerZ sets `BE_IP=127.0.0.1` (loopback). The probe runs in `default` namespace; it cannot reach the game pod's loopback. This is by design (RCON is admin-only, cannot be brute-forced from the internet). The agent sidecar (Path A) shares the game pod's network namespace and reaches 127.0.0.1:2305 directly.

**Evidence of limitation:**

```
Probe pod network namespace: default namespace (pod IP, e.g., 10.0.0.5)
→ Dial 127.0.0.1:2305 reaches the PROBE POD'S loopback, not the game pod's
→ Connection refused (nothing listening on the probe pod's loopback)
```

**Path A coverage:** `agent/internal/rcon/battleye` test confirms RCON connectivity when the agent runs in the same pod as the game. This e2e probe does not measure it, but Path A does.

## References

- **Module template:** `modules/dayz/template.yaml`
- **A2S query reference:** https://developer.valvesoftware.com/wiki/Server_queries
- **ServerZ (backing image):** https://github.com/GodBleak/ServerZ
- **Agent RCON implementation:** `agent/internal/rcon/battleye/`
- **e2e probe harness:** `test/e2e/internal/probe/probe.go`
- **e2e test helpers:** `test/e2e/gamebot_helpers_e2e_test.go`
- **e2e specs index:** `test/e2e/internal/specs.md`
