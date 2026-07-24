# e2e game-bot harness — Specification

**Status:** beta (v0.2.0-beta.7), in-progress (fast set JOINED, heavy set not yet implemented)  
**Module / package:** `test/e2e/internal/{probe,protocol,<game>}` — per-game protocol clients and the shared probe harness  
**Dependencies:** stdlib only (Go 1.25+) — per-game binaries build against `test/e2e/go.mod` with `GOWORK=off`, dragging zero Kubernetes or external modules into the probe image

## Purpose

Prove that a Gameplane-managed game server is not merely "Running" in Kubernetes but genuinely playable: that it speaks its real game protocol, accepts players (or reaches the exact credential gate the test measures), and does so through the network path a real in-cluster client uses. Each game's headless protocol client (in `internal/<game>/app.go`) runs as a Job *inside* the cluster and dials the game Service directly, removing the `kubectl port-forward` hop and exercising authentic in-cluster networking.

This harness exists in two independent layers:

- **Path A** — through Gameplane — validates the full stack: API console WebSocket, `/players` endpoint, action commands, agent sidecar, and on down to the module's declared control protocol.
- **Path B** — the per-game client — proves the server itself is serving, independent of any Gameplane assumptions. It speaks the real game protocol, shares no code with the agent, and observes only the network facts: what does a real client see?

Neither path alone is sufficient. Path A self-confirms (our agent talking to itself); Path B is the ground truth that the server is actually working.

## Responsibilities

1. Measure the exact join depth (JOINED, PARTIAL, or QUERY) each game's client reaches and assert it matches the expected depth.
2. Reject depth regressions: if a game measured as PARTIAL must fail if it starts JOINED (credential gate moved) or drops to QUERY (protocol access lost).
3. Provide a shared retry harness (`probe.Main`, `probe.Retry`) so every per-game binary avoids re-implementing deadline logic, retry loops, and depth validation.
4. Run per-game probes as Kubernetes Jobs inside the cluster (not via `kubectl port-forward`), using the same network path a real player uses.
5. Build one multi-game probe image (`gameplane-test/gameprobe`) with all per-game binaries at `/probe/<game>`.
6. Define the protocol specifications and shared family implementations (Source A2S, LiteNetLib, etc.) so new games reuse proven code.
7. Support environment-based gating (`GAMEPLANE_E2E_GAME_BOT`, `GAMEPLANE_E2E_GAMES`) so heavy games never run in CI.

## Non-goals / boundaries

- Does not execute game-specific custom logic (mod installation, custom commands). Path A exercises those; Path B proves the server.
- Does not provide a general game-client SDK — each protocol is hand-rolled to exactly what CI can measure, not what the real game client needs.
- Does not attempt to reach the Play state or complete a full game session. Measuring login depth is the entire goal.
- Does not create game accounts or supply external credentials (Steam auth, EOS tokens, PlayFab keys). A PARTIAL depth is the boundary where CI cannot proceed.
- Does not instrument the Gameplane stack itself. Path B is *separate* from the operator, API, and agent code.

## Directory & package layout

```
test/e2e/internal/
├── probe/                          # shared harness
│   ├── probe.go                    # Depth type, Main(), Retry(), ParseFlags()
│   └── probe_test.go               # harness unit tests
├── protocol/                       # shared protocol families
│   ├── <family>/                   # e.g. source_a2s/, liteNetLib/, etc.
│   │   ├── protocol.go             # family impl (read-only, no game-specific state)
│   │   └── protocol_test.go        # family unit tests (run on every make test-go)
│   └── [more families...]
├── minecraft-java/                 # game-specific
│   ├── app.go                      # main(), implementation of probe.Run callback
│   ├── protocol/                   # game-specific protocol quirks (optional)
│   │   └── minecraft.go
│   └── spec.md                     # depth measurement, protocol reference, how CI reaches it
├── terraria/
│   ├── app.go
│   └── spec.md
├── [not yet implemented...]        # factorio/, garrys-mod/, cs2/, etc. — app.go & spec.md follow same pattern
└── [more games...]
```

The `probe` package provides the harness; per-game `app.go` files (one per `internal/<game>/`) build to `/probe/<game>` via the Dockerfile's GOWORK=off build. Each `internal/<game>/spec.md` documents the measured depth and protocol reference. Shared families live once in `protocol/<family>/` with their own spec and unit tests; per-game `protocol/` dirs hold only game-specific deviations (version pins, packet subsets, quirks).

## External interface / contracts

### Shared harness (`probe` package)

**`Depth` type** — how far into a real join a client got:

```go
type Depth string

const (
    Joined  Depth = "JOINED"  // server accepted the client as a player; login succeeded
    Partial Depth = "PARTIAL" // spoke the real protocol, then hit a credential gate CI cannot mint
    Query   Depth = "QUERY"   // even a partial join is impossible; only the query/status protocol works
)
```

**`ParseFlags() Flags`** — parses `-addr`, `-deadline`, `-expect-depth` and any per-game flags. Must be called by every `app.go`:

```go
flags := probe.ParseFlags()
// flags.Addr: "minecraft-java.gameplane-games.svc.cluster.local:25565"
// flags.Deadline: 4 * time.Minute (overall deadline; probe retries within this)
// flags.Expect: probe.Joined (expected depth)
```

**`Main(Flags, RunFunc)`** — never returns; runs `RunFunc(ctx)`, asserts it reached exactly `Expect` depth, and exits 0 only if both succeed. Exits 1 on any other outcome:

```go
Main(flags, func(ctx context.Context) (Depth, error) {
    // app.go implementation: dial, probe, measure depth, return (Joined, nil) or (Partial, nil) or (Query, nil)
})
```

**`Retry(ctx, what, attempt, fn)`** — calls `fn` until success, ctx expires, or `fn` returns `ErrFatal`. Logs every failed attempt. Built into the probe harness for per-game use:

```go
err := probe.Retry(ctx, "minecraft login", 30*time.Second, func(ctx context.Context) error {
    // attempt a login, return nil on success or a retryable error
})
```

### Test-side interface (`gameprobe_job.go`)

**`GameProbe` struct** — describes one probe run:

```go
type GameProbe struct {
    GameNS      string        // namespace holding the GameServer
    GSName      string        // GameServer name (its Service is dialled)
    Game        string        // module dir name; selects the /probe/<Game> binary
    Port        int           // game port (e.g. 25565 for Minecraft)
    Deadline    time.Duration // total time allowed for the probe
    ExpectDepth string        // "JOINED" | "PARTIAL" | "QUERY"
    Args        []string      // extra per-game flags, e.g. []string{"-user", "bot"}
}
```

**`Env.RunGameProbe(t, GameProbe)`** — creates a one-shot Job in the `default` namespace, waits for it to finish or timeout, and fails the test if the probe didn't exit 0. Reads logs on failure. The Job dials the game Service at `<GSName>.<GameNS>.svc.cluster.local:<Port>`.

### Per-game implementation (`app.go`)

Each `internal/<game>/app.go` is a tiny `main()` that:

1. Calls `probe.ParseFlags()` to get the server address and deadline.
2. Implements a callback function matching `func(context.Context) (probe.Depth, error)`.
3. Calls `probe.Main(flags, callback)` and never returns.

The callback dials the server, speaks the real game protocol (or query protocol if that's all that's possible), measures the join depth, and returns `(Joined, nil)`, `(Partial, nil)`, or `(Query, nil)`. If the server is misconfigured, return `(Query, probe.ErrFatal)` to skip retries.

Example shape (Minecraft):

```go
package main

import (
    "context"
    "github.com/ValgulNecron/gameplane/test/e2e/internal/probe"
    "github.com/ValgulNecron/gameplane/test/e2e/internal/minecraft-java/protocol"
)

func main() {
    flags := probe.ParseFlags()
    probe.Main(flags, func(ctx context.Context) (probe.Depth, error) {
        conn, err := net.DialContext(ctx, "tcp", flags.Addr)
        if err != nil {
            return probe.Query, err
        }
        defer conn.Close()
        
        // Measure depth via real protocol
        depth, err := protocol.MeasureDepth(ctx, conn)
        return depth, err
    })
}
```

## Key invariants

1. **Depth is a measured outcome, not a planning input.** The `ExpectDepth` in `GameProbe` is not a guess or a target — it is the *exact depth the real server reaches*, measured via a previous probe run against a known-good server. If a game's depth shifts (credential gate moves, protocol access lost), the test must fail to alert the maintainer that something changed.

2. **Probe binaries import stdlib only.** The Dockerfile builds with `GOWORK=off` against `test/e2e/go.mod`. Any import of `k8s.io/*`, `github.com/ValgulNecron/gameplane/operator`, or external modules not in `test/e2e/go.mod` will compile fine locally and fail silently at docker-build time. Shared protocol families and the `probe` package itself live in `test/e2e/internal/` (OK); everything else must be stdlib.

3. **The probe is the single source of truth for join depth.** A game's `spec.md` documents the depth and cites the protocol reference (e.g., "Minecraft protocol § Login, § Encryption Request"). Tests read `ExpectDepth` from `spec.md`; `spec.md` is *not* a guess but a summary of what the probe actually measured against a reference server.

4. **Exact-depth assertion prevents silent regressions.** If a `PARTIAL` game starts JOINED (e.g., the auth gate code path changed), `probe.Main` will exit 1 because `Joined != Partial`. If it drops to `QUERY`, that also exits 1. This is intentional: the test *must* fail so we notice.

5. **Path A and Path B are independent.** Per-game `app.go` files share no code with `agent/internal/rcon/`, `api/internal/ws/`, or any Gameplane packages. This independence is the entire point — if a per-game client can reach a depth but the agent can't, that's a sign the agent's protocol layer is broken, not the game.

6. **Retry loop is in the probe, not the test.** A game server accepts TCP well before it accepts a login (world generation takes time). The probe has its own deadline-bounded retry loop; `RunGameProbe` doesn't retry the Job, just waits once for a terminal outcome. This keeps the Job a one-shot pass/fail.

7. **The probe runs in the `default` namespace, not `gameplane-games`.** The `gameplane-games` namespace carries a `NetworkPolicy` with `podSelector: {}` and default-deny-egress (only DNS allowed out). Running the probe there would block all game-port egress. Running in `default` (where network policy is unrestricted) mirrors how a real player reaches the game: from outside the games namespace. The game's own `allow-kubelet-probes` policy admits ingress from any RFC1918 pod IP, so this works.

## Dependencies

**Internal to `test/e2e/internal/`:**
- `probe` — shared harness and Depth type
- `protocol/<family>/` — shared protocol implementations (Source A2S, Steam query, LiteNetLib, etc.)

**External:**
- Stdlib only: `context`, `encoding/binary`, `fmt`, `io`, `log`, `net`, `os`, `strconv`, `strings`, `time`, and protocol-specific deps (e.g. `encoding/json` for Minecraft status).
- Go 1.25+ (same as the main repo).

No external modules in the probe image. If a game needs a special library, copy the implementation into `internal/protocol/<family>/` or `internal/<game>/`.

## Security considerations

1. **The probe runs unprivileged.** The Job sets:
   - `runAsNonRoot: true`, `runAsUser: 65532` (nonroot)
   - `readOnlyRootFilesystem: true`
   - `capabilities.drop: ["ALL"]`
   - `allowPrivilegeEscalation: false`
   - `seccompProfile: RuntimeDefault`

2. **No credentials in argv.** Per-game `Args` (e.g., `-user bot`) must never contain passwords or tokens. If a server requires authentication CI cannot mint (Steam auth, EOS PlayFab), that's a `PARTIAL` depth, and the `spec.md` must document the exact packet/step where the gate occurs.

3. **Runs from outside the games namespace.** As noted above, the `default` namespace has no egress NetworkPolicy, but the probe exercises the same network path a real player uses. The game's `allow-kubelet-probes` policy admits ingress, so the probe is a legitimate probe, not an anomaly.

4. **Protocol implementations are read-only.** Shared protocol families in `protocol/<family>/` are observational — they parse packets, measure state, and report depth. They do not mutate shared state, do not log at the application level (only debug-level when tracing a protocol), and do not attempt to modify server state. This is a read-only probe, not a fuzzer or stress tester.

## Testing & coverage

### Per-game protocol packages

Each `internal/protocol/<family>/` carries untagged unit tests that run on every `make test-go`:

- **`TestMeasureDepth_<game>`** — exercises the protocol against pre-recorded server responses or a mock that simulates the exact packets a real server sends.
- **`TestParsePackets`** — validates packet parsing on edge cases (truncated, malformed, out-of-order packets).
- Protocol-family specs live in `internal/protocol/<family>/spec.md` and cite the external protocol reference (Minecraft protocol wiki, Source engine A2S docs, etc.).

### E2E test registration

Each game's e2e test (`TestGameServer_<Game>Bot_<Depth>`) is registered in `test/e2e/buckets.sh`:

- **Fast set** (`bot-fast` bucket) — runs in the `e2e-game-bot` CI job on every PR (amd64 only):
  - `TestGameServer_MinecraftJavaBot_Joined`
  - `TestGameServer_TerrariaBot_Joined`
  - `TestGameServer_GarrysModBot_Query`

- **Heavy set** (`bot-heavy` bucket) — **deliberately never runs in CI** — only on maintainer hand-run with `GAMEPLANE_E2E_GAMES=all`:
  - All other games. Reserved for `KESTREL_E2E_REUSE_CLUSTER=1 GAMEPLANE_E2E_CONTEXT=… GAMEPLANE_E2E_GAMES=all make test-e2e-keep`.

The `buckets.sh verify` step fails CI if any test is unbucketed or double-bucketed, so tests cannot be silently dropped.

### Per-game depth table

| Module | Transport | Port | Set | Status | Depth |
|---|---|---|---|---|---|
| minecraft-java | TCP | 25565 | Fast | Protocol client implemented; CI-verified (PR #194, run 30125535832) | JOINED |
| terraria | TCP | 7777 | Fast | Protocol client implemented; CI-verified (PR #194, run 30125535832) | JOINED |
| factorio | UDP | 34197 | Fast | Not yet implemented | — |
| garrys-mod | UDP | 27015 | Fast | Protocol client implemented; A2S query verified against Valve spec | QUERY |
| cs2 | UDP | 27015 | Heavy | Protocol client implemented; A2S query verified against Valve spec | QUERY |
| 7-days-to-die | UDP | 26900 | Heavy | Not yet implemented | — |
| project-zomboid | UDP | 16261 | Heavy | Not yet implemented | — |
| valheim | UDP | 2456 | Heavy | Not yet implemented | — |
| palworld | UDP | 8211 | Heavy | Not yet implemented | — |
| rust | UDP | 28015 | Heavy | Not yet implemented | — |
| v-rising | UDP | 9876 | Heavy | Not yet implemented | — |
| dayz | UDP | 2302 | Heavy | Not yet implemented | — |
| ark-survival-ascended | UDP | 7777 | Heavy | Not yet implemented | — |
| dont-starve-together | UDP | 10999 | Heavy | Not yet implemented | — |
| enshrouded | UDP | 15636 | Heavy | Not yet implemented | — |
| satisfactory | UDP | 7777 | Heavy | Not yet implemented | — |

The depth column is filled in *only when a client actually measures it* — not before. Status "Not yet implemented" means the `internal/<game>/app.go` file does not exist; once it exists and reaches a measured depth, that depth is recorded in the table and the test enters `bot-fast` or `bot-heavy` accordingly. Entries marked "expected, unverified" have a client implementation but have not yet reached the stated depth on a green CI run; the exact depth is conjectured based on protocol analysis but will not be hardened into the table until CI confirms it.

## Why the fast set is small

Four games cover the essential depth variety:

- **minecraft-java (TCP, Java):** traditional login state machine, tests *JOINED* via Encryption Request rejection (offline-mode override) or Login Success.
- **terraria (TCP, .NET):** different TCP framing (message length prefixes), tests *JOINED* via the initial handshake and player ID assignment.
- **factorio (UDP, Lua):** hand-rolled UDP protocol and *QUERY* depth (no headless join possible — Factorio uses in-game UI for multiplayer, no headless auth).
- **garrys-mod (UDP, Source engine):** shared Source family with cs2, exercises A2S query protocol.

These four, plus their shared protocol families, give coverage: TCP state machines, UDP queries, and the two depth tiers most games hit (JOINED or QUERY). Adding Valheim, Palworld, 7-days-to-die, and Rust would triple CI boot time with no new protocol variety.

## Why the heavy set never runs in CI

Heavy games are not a testing gap — they are a provisioning constraint:

- **Disk:** GitHub-hosted runners have ~14 GB usable disk after the OS, Docker images, and runner overhead. CS2 declares a 60Gi PVC, 7-days-to-die 55Gi across two, DayZ 40Gi, ARK 30Gi. Even one of these exhausts the node.
- **Throughput:** Seven games (CS2, 7DTD, DayZ, ARK, Palworld, Valheim, Rust) run multi-GB SteamCMD or Proton downloads on first boot. CI runners (especially shared GitHub-hosted) have inconsistent upload/download throughput; a 20GB download can take 10+ minutes or fail partway.
- **Duration:** Combining boot time (10–30 min per heavy game) and download throughput means a single CI job would timeout even if disk were not exhausted.

**This is a standing decision, not temporary.** Heavy games' protocol packages and unit tests still ship and still run in `make test-go` on every PR — it is only the real-image boot that is maintainer-initiated. The command:

```bash
GAMEPLANE_E2E_REUSE_CLUSTER=1 GAMEPLANE_E2E_CONTEXT=<kubelab|prod> GAMEPLANE_E2E_GAME_BOT=1 GAMEPLANE_E2E_GAMES=all make test-e2e-keep
```

runs all 16 games against a pre-existing cluster (e.g., the `kubelab` remote k3s). No CI job ever executes `bot-heavy` or sets `GAMEPLANE_E2E_GAMES` to `all` or `heavy`.

## Shared protocol families

Known families (each with its own `internal/protocol/<family>/` dir, spec, and unit tests):

- **A2S / Source A2S query (`protocol/a2s`):** `A2S_GETCHALLENGE` + `A2S_INFO` connectionless query protocol, verified against Valve's specification. Used by cs2, garrys-mod, and others. Returns player count, server name, map, game version. Implementation is confirmed correct and determines QUERY depth.
- **Source protocol (`protocol/source`):** Challenge and Connect handshake for login-phase connectivity. Implementation is best-effort and unverified — packet formats may diverge on Source 2 (CS2) from earlier implementations — so it is currently used only diagnostically for evidence (what is the server's response?) and does not determine depth.
- **Steam A2S (query-only):** Same as above; some servers support both query and a login state.
- **LiteNetLib (Unity):** Connection-oriented UDP with per-message reliability flags, used by 7-days-to-die and possibly v-rising.
- **Minecraft protocol:** Handshaking, Status (Server List Ping), Login states. Shared across Java and potential .NET Bedrock ports.
- **Terraria protocol:** TCP message frames with length prefixes; per-player handshake and state.

Per-game `protocol/` dirs hold only game-specific deviations: Minecraft protocol version pinning (1.20.1, 1.21, etc.), Terraria mod-specific packet IDs, ARK's custom A2S extensions, etc.

## References

- **`gameprobe_job.go`** — `RunGameProbe`, `GameProbe` struct, and in-cluster Job harness.
- **`probe/probe.go`** — `Depth`, `ParseFlags`, `Main`, `Retry` harness.
- **`buckets.sh`** — e2e test bucketing, `bot-fast` / `bot-heavy` definitions, and `GAMEPLANE_E2E_GAME_BOT` / `GAMEPLANE_E2E_GAMES` gating.
- **`.github/workflows/ci.yaml`** — `e2e-game-bot` job (amd64 only, 50-minute timeout, runs `bot-fast` bucket).
- **`Dockerfile`** — builds `gameplane-test/gameprobe` image, per-game probes, `GOWORK=off` constraint.
- **`test/e2e/go.mod`** — minimal Go module for probe binaries (stdlib, probe package, protocol families).
- **`internal/<game>/spec.md`** — per-game depth measurement, protocol reference, how CI reaches the measured depth.
- **Minecraft protocol reference:** https://wiki.vg/Protocol (authoritative source for packet IDs, Encryption Request, Login Success, etc.)
- **Terraria protocol:** community reverse-engineering in `internal/terraria/spec.md` (Appendix: Wire Format section); no official spec.
- **Source engine A2S:** https://developer.valvesoftware.com/wiki/Server_queries; used by garrys-mod and cs2.
- **CLAUDE.md** — "Path A and Path B are independent" and "The operator is authoritative" principles.
