# agent — Specification

**Status:** beta (v0.2.0-beta.7)  
**Module / package:** `github.com/ValgulNecron/gameplane/agent`

## Purpose

The agent is a per-pod HTTP/HTTPS sidecar that runs inside every game pod to expose the game server's operations to the control plane and dashboard. It translates dashboard requests into game-protocol actions (RCON, file I/O, container logs, player queries) and reports liveness, resource usage, and player metrics back to the operator via Kubernetes API patches, decoupling game-specific behavior from the cluster control plane.

## Responsibilities

- **Request authentication**: Verify incoming requests via mTLS (preferred) or shared-secret bearer token.
- **Console & RCON**: Duplex WebSocket forwarding user commands to the game via RCON and echoing responses; supports Valve/Source, Telnet, WebSocket, BattlEye, Satisfactory, and Palworld protocols.
- **File I/O**: List, read, download, upload, write, mkdir, delete within the `/data` volume; reject path traversal and symlink escape.
- **Logs**: Tail the game container's log file over WebSocket (text frames per line); supports streaming from start or end.
- **Players**: Query online count, names, ban lists, and moderation actions (kick, ban, unban) via RCON; capabilities advertise per-game support.
- **Quiesce**: Pause auto-saves and flush in-flight state before snapshots; run module-declared sequences over RCON and handle games that don't support it gracefully.
- **Lifecycle**: Run module-declared stop sequences over RCON before the operator scales the server to zero.
- **Actions**: Execute module-declared operator actions (templated RCON commands with user parameters) without agent code changes.
- **Status metrics**: Query live game metrics (TPS, world time, etc.) via RCON and regex extraction; module-declared in `capabilities.status.metrics[]`.
- **Mods**: List, install, and uninstall game mods from a registry; track per-volume metadata; guard downloads with strict SSRF egress validation.
- **Heartbeat**: Periodically patch the owning GameServer's status with lastHeartbeat, playersOnline, playersMax, gameVersion, and resource usage (CPU, memory, disk).

## Non-goals / boundaries

- The agent does **not** expose a PTY or attach the game container directly. Games with consoleMode "pty" (e.g., Unity servers) are handled by the Gameplane API bridging the browser WebSocket to Kubernetes pod-attach; the agent is uninvolved.
- The agent does **not** manage game processes or containers — all process/container control is the operator's job (scaling, restarts, resource limits).
- The agent does **not** validate RCON responses for correctness. It forwards raw game output to the UI, so game-specific parsing happens on the dashboard (e.g., player-list regex rendering).
- The agent does **not** require a cluster metrics pipeline (no Prometheus scrape, no metrics-server dependency). All resource usage is sourced in-pod from `/proc` or cgroups.
- The agent does **not** implement game-specific business logic. All protocol handlers are per-game and declared in the module's template (`spec.capabilities`); new games require no agent code change.

## Directory & package layout

```
agent/
├── cmd/main.go              # Entry point: flag parsing, auth setup, chi router, heartbeat goroutine
├── internal/
│   ├── actions/             # Module-declared operator actions (templated RCON)
│   ├── auth/                # Request authenticator (mTLS + bearer token)
│   ├── caps/                # Mirrors GameTemplate.spec.capabilities schema (JSON unmarshaling)
│   ├── console/             # Duplex WebSocket console bridge (RCON stdin/stdout)
│   ├── files/               # File-browser HTTP API (list, read, write, upload, mkdir, delete)
│   ├── heartbeat/           # GameServer status patcher (lastHeartbeat, players, version, usage)
│   ├── lifecycle/           # Stop sequence execution before scale-to-zero
│   ├── logs/                # Log file streaming over WebSocket (tail from start or end)
│   ├── mods/                # Mod list, install, uninstall (manifest tracking, SSRF guard)
│   ├── players/             # Player count, names, ban lists, moderation (kick, ban, unban)
│   ├── quiesce/             # Pause auto-saves and flush state before snapshots
│   ├── rcon/                # RCON wire protocols (source, telnet, websocket, battleye, satisfactory, palworld)
│   ├── status/              # Live metrics extraction via RCON + regex
│   └── usage/               # Resource usage reader (/proc or cgroups; disk via statfs)
├── openapi.yaml             # Partial machine-readable HTTP contract (subset of routes)
└── .testcoverage.yml        # Coverage gate (90% total, unit-only)
```

Per-package roles:

- **`actions`**: Renders module-declared RCON command templates with user parameters; validates via `gameaction` package.
- **`auth`**: Two modes: mTLS (agent listens TLS, requires client cert signed by `--tls-client-ca`), or shared-secret bearer token (fallback for dev).
- **`caps`**: Unmarshals JSON capabilities blob from `GAMEPLANE_CAPABILITIES` env; exposes `Spec` with `Players`, `Quiesce`, `Lifecycle`, `Actions`, `Status`, `Mods`.
- **`console`**: Accepts `{ kind: "cmd", body: "<rcon cmd>" }` JSON over WebSocket, runs it via RCON, replies with `{ kind: "out"|"err", body: "<response>" }`.
- **`files`**: Walks the filesystem under `--data-root`, enforces path-traversal protection (no `..`, no symlinks escaping the root), handles multipart uploads.
- **`heartbeat`**: Runs a background goroutine that every 20 seconds patches `gameservers/<name>/status` with `agent.lastHeartbeat`, `status.playersOnline`, `status.playersMax`, `status.gameVersion`, and resource usage via the pod's ServiceAccount.
- **`lifecycle`**: HTTP handler for the operator's `/lifecycle/stop` call; runs module-declared stop commands over RCON before the game process terminates.
- **`logs`**: Tails a game log file (path from `--game-log-path`) over WebSocket; supports streaming from end (default, "live") or start ("backlog").
- **`mods`**: Tracks installed mods in a per-volume manifest (`.gameplane-mods.json`); downloads from registry with strict egress validation via `netguard.IsPublic`.
- **`players`**: Queries player count, names, ban lists, and runs moderation actions over RCON; game-specific `commander` implementations (Minecraft, Satisfactory, Palworld, etc.) report capabilities.
- **`quiesce`**: Runs module-declared sequences (e.g., Minecraft's `save-off` + `save-all flush`) over RCON; responds `quiesced: false` + reason when unsupported (not an error).
- **`rcon`**: Factory pattern for wire-protocol clients (`Valve/Source`, `Telnet`, `WebSocket`, `BattlEye`, `Satisfactory`, `Palworld`, `Disabled`); `Exec(cmd) (string, error)` interface.
- **`status`**: Runs module-declared metrics queries over RCON; each metric specifies a command and a regex with named group `"value"` for extraction.
- **`usage`**: Reads CPU/memory from `/proc` (proc mode, default) or cgroup v2; disk via `statfs`; exposes `Sample` with `Known` flags so callers distinguish "unknown" from "zero".

## External interface / contracts

### Entry point

Entry: `agent/cmd/main.go`  
Mode: In-pod HTTP/HTTPS sidecar (runs as a container sidecar or as a pod share-process-namespace helper)

### Command-line flags

| Flag | Default | Env var | Purpose |
|------|---------|---------|---------|
| `--addr` | `:8090` | — | HTTP listen address (e.g., `:8090` for all interfaces) |
| `--data-root` | `/data` | — | Root path for file operations (agent restricts all I/O here) |
| `--rcon-host` | `127.0.0.1` | — | RCON server host (loopback in-pod) |
| `--rcon-port` | `25575` | — | RCON server port (game-specific default) |
| `--rcon-password-file` | `` | — | Path to file holding the RCON password |
| `--rcon-enabled` | `true` (from env) | `GAMEPLANE_RCON_ENABLED` | Whether the game exposes RCON; `false` degrades RCON-backed endpoints gracefully |
| `--rcon-protocol` | `source` (from env) | `GAMEPLANE_RCON_PROTOCOL` | RCON wire protocol: `source` (Valve/Minecraft), `telnet` (7 Days to Die), `websocket` (Rust), `battleye` (DayZ/Arma), `satisfactory` (Satisfactory), `palworld` (Palworld); unrecognized falls back to `source` |
| `--game-log-path` | `` | — | Path to the game container's log file for `/logs/tail` |
| `--tls-cert` | `` | — | Server TLS cert (PEM); if set, requires `--tls-key` and enables HTTPS + mTLS |
| `--tls-key` | `` | — | Server TLS key (PEM) |
| `--tls-client-ca` | `` | — | CA bundle that signs API client certs (required if `--tls-cert` is set) |
| `--api-token-file` | `` | — | Fallback shared-secret auth (used when TLS is not configured); file contents become the bearer token |
| `--server-name` | `` | `GAMEPLANE_SERVER_NAME` | Owning GameServer name (for status patches) |
| `--template` | `` | `GAMEPLANE_TEMPLATE` | GameTemplate name |
| `--game` | `` | `GAMEPLANE_GAME` | Game identifier (e.g., `minecraft`, `rust`, `satisfactory`) |
| `--capabilities` | `` | `GAMEPLANE_CAPABILITIES` | Declared game capabilities (JSON, from `GameTemplate.spec.capabilities`) |
| `--log-level` | `info` (from env) | `GAMEPLANE_LOG_LEVEL` | Log verbosity: `debug`, `info`, `warn`, `error` |

Resource usage env vars (set by the operator):

| Env var | Purpose |
|---------|---------|
| `GAMEPLANE_USAGE_PROC` | When `"1"`, enables proc mode (reads game process CPU/memory from `/proc`); otherwise falls back to cgroup mode |
| `GAMEPLANE_CPU_LIMIT_MILLICORES` | Game container's CPU limit (for usage percentage calculation) |
| `GAMEPLANE_MEM_LIMIT_BYTES` | Game container's memory limit |

### Endpoint groups (from mounted routes in cmd/main.go; openapi.yaml documents a subset)

**Public (unauthenticated):**
- `GET /healthz` — Liveness probe; returns `200 ok`
- `GET /metrics` — Prometheus metrics exposition

**Protected (all require mTLS cert or bearer token):**

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/files/list` | GET | List directory entries; query param `path` (default `/`) |
| `/files/read` | GET | Read file inline (small files only); query param `path` |
| `/files/download` | GET | Download file as attachment; query param `path` |
| `/files/write` | POST | Overwrite or create file; query param `path`; body is raw file content |
| `/files/upload` | POST | Upload one or more files to a directory; query param `path`; body is `multipart/form-data` with `files[]` |
| `/files/mkdir` | POST | Create directory (recursive); query param `path` |
| `/files/delete` | DELETE | Delete file or directory; query param `path`, optional `recursive` (boolean) |
| `/logs/tail` | GET | Tail game log over WebSocket; query param `from` (enum: `start`, `end`, default `end`) |
| `/console` | GET | Duplex console over WebSocket; client sends `{ kind: "cmd", body: "<command>" }` (JSON), server replies `{ kind: "out"\|"err", body: "<response>" }` |
| `/players` | GET | Current online player count and names; response: `{ online, max, players[], asOf, capabilities }` |
| `/players/kick` | POST | Kick a player; request: `{ name, reason? }`; response: `{ ok, raw? }`; 501 if unsupported |
| `/players/ban` | POST | Ban a player; request: `{ name, reason? }`; response: `{ ok, raw? }`; 501 if unsupported |
| `/players/unban` | POST | Lift a ban; request: `{ name }`; response: `{ ok, raw? }`; 501 if unsupported |
| `/players/banned` | GET | Currently banned players; response: `[ { name, reason?, source? } ]`; 502 if RCON unavailable |
| `/quiesce` | POST | Pause auto-saves before snapshot; response: `{ quiesced, reason? }` (boolean, reason optional); unsupported games return `quiesced: false` with a reason |
| `/unquiesce` | POST | Resume auto-saves after snapshot; response: `{ quiesced, reason? }` |
| `/lifecycle/stop` | POST | Run stop sequence before scale-to-zero; RCON connection drop is the expected outcome |
| `/actions/run` | POST | Execute module-declared operator action; request: `{ name, params }` |
| `/status` | GET | Live game metrics (module-declared); response: `{ metrics[] }` |
| `/mods` | GET | List installed mods; response: `{ mods[] }` |
| `/mods/install` | POST | Install a mod; request: `{ name, version, ... }` |
| `/mods` | DELETE | Uninstall a mod; request: `{ name }` |
| `/mods/upload` | POST | Upload a mod archive; body is multipart/form-data |

All endpoints (except `/healthz` and `/metrics`) return `401 Unauthorized` if the request lacks a valid cert or token.

## Key invariants

- **Every request is authenticated**: The `auth` package gates all protected routes with either mTLS verification or bearer-token matching.
- **RCON is a lower-trust boundary**: The agent uses `netguard.IsPublic()` (strict, permissive only for well-known registries) for mod-install downloads, assuming modules are less trusted than the operator.
- **Gameaction validation is independent**: Both the API (stdin pod-attach) and the agent (RCON) call `gameaction.Resolve()` independently to validate action inputs (no control characters, 512-char cap, required-ness checks, etc.). Neither trusts the other.
- **No persistent storage**: The agent has no database. GameServer status patches flow through the operator; all transient state (WebSocket streams, RCON sessions) is in-memory.
- **Path traversal is blocked**: `files` package rejects `..`, symlinks, and any path component starting with `.`; all I/O is rooted under `--data-root`.
- **Resource usage is in-pod**: The `usage` package reads from `/proc` or cgroups; no external metrics pipeline required. Cgroup mode is a fallback for older clusters; proc mode (default in production) requires the operator to set `ShareProcessNamespace: true`.
- **Module capabilities drive behavior**: Every game-specific handler (players, quiesce, lifecycle, status, actions) reads its config from `--capabilities` (JSON unmarshaled into `caps.Spec`). New games require no agent code change.
- **RCON connection errors are graceful**: A lost RCON connection does not crash the agent. `console`, `players`, `quiesce`, and `lifecycle` handlers catch connection errors and return appropriate HTTP status (e.g., `502 Bad Gateway`).
- **Log streams are tail-only**: The `logs` package does not support random-access reads. It streams from the current end (live mode) or from file start (backlog mode); clients must handle partial output and reconnection.

## Dependencies

### Internal

- **`netguard`** (sibling module): SSRF dial-guard for egress validation. Agent uses `IsPublic()` (strict policy for mod downloads). Operator uses `IsAllowed()` (permissive for git/http ModuleSource fetches).
- **`gameaction`** (sibling module): Console-injection guard and command-template renderer. Agent calls `Resolve()` on RCON action inputs independently.

### External

| Module | Version | Purpose |
|--------|---------|---------|
| `github.com/go-chi/chi/v5` | v5.1.0 | HTTP router |
| `github.com/coder/websocket` | v1.8.12 | WebSocket library for console, logs, player queries |
| `k8s.io/apimachinery` | **v0.31.1** | Kubernetes types for status patches |
| `k8s.io/client-go` | **v0.31.1** | Kubernetes client for heartbeat (GameServer status patches) |
| `github.com/prometheus/client_golang` | v1.20.5 | Prometheus metrics (`/metrics` endpoint) |
| `golang.org/x/sys` | v0.22.0 | System-level utilities (used by client-go) |

**Note:** The agent is pinned to Kubernetes v0.31.1 (Kubernetes 1.31) while other modules (operator, api) use v0.35.0 (Kubernetes 1.35). This is intentional to maintain compatibility with a broader range of cluster versions. The older k8s version does not restrict the agent's functionality in the current scope.

## Data & persistence

- **Game data volume** (`--data-root`, typically `/data`): A PVC mounted read-write. The agent's file I/O operations are confined here; no access to system paths or other volumes.
- **GameServer status**: Patched periodically by the `heartbeat` goroutine via the Kubernetes API. The agent holds no local copy; the operator is the source of truth.
- **Mod install manifest** (`.gameplane-mods.json`): Stored per-volume under `--data-root`. Tracks metadata for each installed mod (name, version, registry source, checksum, etc.). Unmarshaled on startup and written on install/uninstall.
- **RCON sessions**: In-memory only. Each RCON protocol implementation holds a connection pool or singleton (e.g., Source uses a single TCP stream with request ID sequencing; BattlEye uses UDP).
- **WebSocket streams** (console, logs): In-memory buffering. No replay; clients reconnect to resume.

## Security considerations

- **mTLS + token auth**: All protected endpoints require either a valid mTLS client cert (signed by `--tls-client-ca`) or a bearer token (from `--api-token-file`).
- **SSRF guard on mod downloads**: `netguard.IsPublic()` enforces a strict allowlist for registry hostnames. Private registries on loopback or non-routable addresses are rejected unless explicitly whitelisted.
- **Console-injection guard on RCON actions**: `gameaction.Resolve()` validates action inputs independently on the agent side; control characters, oversized inputs, and required-parameter validation prevent blind command injection.
- **Path-traversal protection**: The `files` package rejects `..`, symlinks escaping `--data-root`, and dotfile access; all I/O is confined.
- **Low privilege within pod**: The agent is a sidecar container (not privileged, not root unless the game container is). It reads `/proc` only for the game process and the pause process; it cannot access other pods' data.
- **No unauthenticated data exposure**: `/healthz` and `/metrics` are public; all game data (`/files`, `/logs`, `/console`, `/players`) requires authentication.
- **Network context**: The agent runs inside the game pod and is reached via service DNS (e.g., `gameserver-pod-0.gameplane-games.svc.cluster.local:8090`). The pod's NetworkPolicy may restrict egress (e.g., games namespace has default-deny-egress); the agent's mod downloads and heartbeat calls must be compatible with that policy.

## Testing & coverage

- **Unit tests**: All packages have `*_test.go` files covering happy paths, error cases, and protocol edge cases (e.g., RCON packet framing, BattlEye CRC32, Satisfactory HTTP auth).
- **e2e tests**: The `api-agent` bucket in `test/e2e/` runs real agent instances on a kind cluster, testing console duplex, file I/O, moderation, quiesce/lifecycle, and heartbeat against live game servers (Minecraft, Satisfactory, Palworld).
- **Coverage gate**: `agent/.testcoverage.yml` enforces **90% total line coverage** (re-baselined down from 91% after the SSRF dial guard moved to `netguard`). Excluded: `cmd/` (flag/signal wiring) and `proto/` (if present). The remaining ~10% gap is in `heartbeat.Run()`'s in-cluster `rest.InClusterConfig` path (only meaningful in a real pod with ServiceAccount) and a few bookkeeping branches in `logs.streamFile` (sleep/return logic), both exercised by the e2e tier.

## References

- **`agent/openapi.yaml`** — Partial machine-readable HTTP contract documenting a subset of the routes (security schemes, endpoint paths, request/response schemas).
- **`docs/architecture.md`** — System overview, data flow, and the "operator is authoritative" design principle.
- **`docs/security.md`** — Auth model, threat boundaries, pod security defaults, and the module-trust relationship.
- **`go.mod`** (agent) — Dependency versions and workspace references.

