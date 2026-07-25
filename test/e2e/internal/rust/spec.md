# Rust Game Bot — Specification

**Status:** heavy set (never runs in CI). Hand-run only via:
```bash
GAMEPLANE_E2E_REUSE_CLUSTER=1 GAMEPLANE_E2E_CONTEXT=<kubelab|prod> GAMEPLANE_E2E_GAMES=all make test-e2e-keep
```

## Purpose

Prove that a Gameplane-managed Rust dedicated server is reachable via the in-cluster game protocol and is accepting queries, even though a full player join requires Steam identity which CI cannot mint.

## Depth Measurement

**Measured Depth:** QUERY (protocol-level query support confirmed)

**Evidence:** A2S (Steam Application Server query) protocol success on UDP port 28015.

Rust servers (didstopia/rust-server) support the A2S query protocol, a standard Valve-originated connectionless UDP query used by game browsers and server tools. A successful A2S_INFO response proves:
- The server process is alive and bound to the network.
- The server is parsing and responding to external queries.
- Server metadata (name, map, player count, version) is obtainable.

This is Path B (independent probe) measurement: the bot speaks the real Rust query protocol, shares no code with the agent, and observes only the network facts.

## Non-Goals / Boundaries

- **JOINED depth not measured:** Rust's game protocol is RakNet-derived and undocumented. A real join requires:
  - Steam identity (SteamID, auth ticket via Steam's authentication API)
  - Valid GSLT (Game Server Login Token) or LAN mode (`+sv_lan 1` in launch args)
  - Protocol version negotiation matching a specific Rust version

  The e2e test uses a dummy RCON password and no Steam credentials, so joins fail at the Steam auth gate.

- **WebSocket RCON not used for depth:** The agent's rcon.websocket protocol is documented and working (Path A will test it). The probe does not use RCON for depth measurement because:
  - RCON requires a password, which is only known to the operator (injected via environment variable).
  - The probe binary is read-only and receives no secrets, so secretless RCON auth is not possible.
  - Using RCON would blur Path A (Gameplane through the operator/agent) and Path B (independent server protocol).

## Directory & Package Layout

```
test/e2e/internal/rust/
├── app.go           # main() and probeRust(); dials A2S on UDP 28015
└── spec.md          # this file
```

## External Interface / Contracts

**Input:** `-addr <host:port>`, `-deadline <duration>`, `-expect-depth QUERY`
- Example: `-addr rust.gameplane-games.svc.cluster.local:28015 -deadline 4m -expect-depth QUERY`

**Output:** Process exits 0 only if A2S query succeeds AND depth == QUERY.

**Protocol:** A2S_INFO query (Valve Source query protocol)
- Reference: https://developer.valvesoftware.com/wiki/Server_queries
- Request: UDP connectionless, 0xFFFFFFFF 0x54 "Source Engine Query\0" [challenge]
- Response: 0xFFFFFFFF 0x49 [server metadata...]

## Key Invariants

1. **A2S is the single source of truth for QUERY depth.** Once A2S succeeds, depth is QUERY regardless of other outcomes.

2. **A2S failure means the server is not answering queries.** If A2S fails after retries exhaust the deadline, the probe exits 1 — the server is not ready or not reachable via the query protocol.

3. **The probe owns its own retry loop.** Game servers accept TCP/UDP well before they answer queries (world generation takes time). The probe retries A2S every 3 seconds until the deadline expires. The test does not retry the Job, so a non-zero exit is fatal to the test.

## Dependencies

**Internal:**
- `test/e2e/internal/probe` — harness (Depth, ParseFlags, Main, Retry, ErrFatal)
- `test/e2e/internal/protocol/a2s` — A2S query implementation

**External:**
- stdlib only (Go 1.25+)

## Security Considerations

1. **The probe is unprivileged:**
   - Runs as `uid=65532` (nonroot), `gid=65532`
   - `readOnlyRootFilesystem: true`
   - `allowPrivilegeEscalation: false`
   - `capabilities.drop: ["ALL"]`
   - `seccompProfile: RuntimeDefault`

2. **No secrets or credentials:** The probe receives no password, GSLT, or Steam tokens. It only dials the public query port.

3. **Read-only observable:** The probe does not mutate any server state. It only sends query packets and reads responses.

## Testing & Coverage

**Unit tests:** None (the A2S family has its own tests in `test/e2e/internal/protocol/a2s/a2s_test.go`).

**E2E test:** `TestGameServer_RustBot_Query` in `test/e2e/rust_bot_e2e_test.go`.

**CI status:** HEAVY SET — never runs in CI. Only runs on maintainer hand-run with:
```bash
GAMEPLANE_E2E_GAMES=all
```

## References

- **Rust module:** `modules/rust/template.yaml` (GameTemplate with WebSocket RCON, game port 28015, RCON port 28016)
- **Rust image:** `didstopia/rust-server:latest` (pinned to specific SHA in template; verify tag exists before boot)
- **A2S protocol:** https://developer.valvesoftware.com/wiki/Server_queries
- **agent/internal/rcon/websocket.go** — the agent's WebSocket RCON client (Path A / not used here)
- **test/e2e/internal/protocol/a2s/** — A2S query family (used by this probe and garrys-mod/cs2)

## Configured Budgets

- **Storage:** 10Gi (per template)
- **CPU request:** 1, limit 4 (per template)
- **Memory request:** 4Gi, limit 8Gi (per template)
- **Probe deadline:** 4 minutes (timeout for the entire probe, including retries)
- **Probe readiness timeout:** 10 minutes (overall time allowed for the pod to reach Running state before probe begins)
- **A2S retry interval:** 3 seconds (pause between failed attempts)

## Measurements (Unmeasured)

Boot time, first response latency, full world generation time — not measured by the probe. The probe focuses on the narrowest question: does the server answer A2S queries?
