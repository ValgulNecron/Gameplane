# Valheim e2e probe specification

**Status:** beta (heavy-set, hand-run only)  
**Module:** `test/e2e/internal/valheim/`  
**Depth:** QUERY

## Purpose

Prove that a Gameplane-managed Valheim server (via `lloesche/valheim-server`) is alive and responding to health queries. Valheim's game protocol (UDP 2456+) is proprietary and not publicly documented, so a full player join (JOINED or PARTIAL depth) is not possible in CI. Instead, this probe measures the documented HTTP status endpoint that the image exposes for health checks.

## Responsibilities

Fetch the Valheim server's `/status.json` HTTP endpoint (port 80, `STATUS_HTTP=true`), parse the JSON response, and assert the server is reporting player count, max players, uptime, and version. This proves:

1. The container is running and accepting HTTP connections
2. The Valheim dedicated server process has started and initialized its world
3. The server is in a playable state (the status endpoint is only served once the server is ready to accept players)

The probe does NOT attempt to:
- Connect to UDP ports 2456+ (no client implementation exists)
- Join as a player (would require Steam identity)
- Verify game state beyond what the status endpoint reports

## Non-goals / boundaries

- Does not attempt a player join (JOINED depth)
- Does not measure game-specific protocol details (players list, world name, etc. beyond status.json)
- Does not test BepInEx mod loading or in-game admin commands
- Does not verify network policy or egress (that's Path A via the agent; Path B is connectivity only)

## Directory & package layout

```
test/e2e/internal/valheim/
├── app.go       # main() — fetch /status.json, return QUERY depth
└── spec.md      # this file
```

No `protocol/` subdirectory is needed; HTTP is stdlib and requires no special framing logic.

## External interface / contracts

**Input** (`-addr` flag): host:port of the Valheim server's status port (default 80).

**Output**: Returns `QUERY` depth on success (status.json parsed and servers reports uptime > 0).

**Retries**: Fetches `/status.json` on a 15-second attempt interval, bounded by the overall probe deadline (typically 4 minutes).

## Key invariants

1. **Depth is QUERY, not higher.** The HTTP status endpoint is not a game join; it is a health protocol defined by the lloesche/valheim-server image. Depth measurement stops here because no public documentation or authentication method exists for Valheim's UDP game protocol.

2. **HTTP port is fixed at 80.** The template.yaml sets `STATUS_HTTP_PORT=80`; the probe hardcodes port 80 in the HTTP URL construction.

3. **Retry loop is internal.** The probe retries status.json fetches on failure; a single Job failure means the server never became responsive within the deadline.

4. **No external imports.** `app.go` imports only stdlib: `context`, `encoding/json`, `fmt`, `io`, `net/http`, `time`.

## Dependencies

**Internal:** `probe` package harness (Retry, Main, Depth type).

**External:** stdlib only (`encoding/json`, `io`, `net/http`, `context`, `fmt`, `time`).

## Security considerations

- Probe runs unprivileged in the `default` namespace with a restrictive SecurityContext.
- HTTP request includes a 10-second timeout to prevent hanging on unresponsive servers.
- statusResponse JSON struct is read-only; no fields are mutated.

## Testing & coverage

### Unit tests

The `statusResponse` JSON struct is tested implicitly when the probe runs; no standalone unit tests are needed because JSON unmarshaling is stdlib and a real server response is the ground truth.

If needed, a fake server test could be added under `valheim_protocol_test.go` by starting a fake HTTP server on `127.0.0.1:0` and feeding it a static JSON response. This is deferred until a second Valheim-specific feature is added.

### E2E test registration

The test is registered in `test/e2e/buckets.sh` as part of the `bot-heavy` bucket:

```
TestGameServer_ValheimBot_Query
```

The `bot-heavy` bucket is **deliberately never run in CI**. It is only executed when a maintainer hand-runs the e2e suite with:

```bash
GAMEPLANE_E2E_GAMES=all GAMEPLANE_E2E_GAME_BOT=1 KESTREL_E2E_REUSE_CLUSTER=1 make test-e2e-keep
```

(Valheim is heavy because the `lloesche/valheim-server` image executes SteamCMD on first boot, which downloads a multi-GB game binary — too large and slow for CI runners.)

### Measurements

This game never runs in CI, so no measurements are collected by the CI pipeline. Depth is measured at `QUERY` via manual runs. Expected depth table:

| Metric | Value |
|---|---|
| Depth | QUERY |
| Probe runtime | ~5–30 seconds (after server startup, once world generation completes) |
| Server startup time | ~2–5 minutes (SteamCMD download + Valheim startup) |
| Boot success rate | Should be 100% on reuse-cluster runs (server is idempotent) |

These are unverified estimates; no CI run collects hard numbers.

## References

- **Valheim HTTP status endpoint:** https://github.com/lloesche/valheim-server-docker#status-http-server — official lloesche/valheim-server documentation of the `/status.json` format and port configuration.
- **test/e2e/internal/specs.md** — shared probe harness specification, depth definitions, and fast/heavy set rationale.
- **modules/valheim/template.yaml** — GameTemplate configuration, including `STATUS_HTTP=true` and `STATUS_HTTP_PORT=80`.
- **test/e2e/buckets.sh** — e2e test bucket definitions; Valheim is in `bot-heavy`.
- **test/e2e/gamebot_helpers_e2e_test.go** — `runGameBotTest` harness and `gameBotSpec` struct.
