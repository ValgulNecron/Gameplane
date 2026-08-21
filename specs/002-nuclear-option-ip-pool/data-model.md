# Data Model: Nuclear Option Module & Load-Balancer IP Pool Override

**Spec Document**: `specs/002-nuclear-option-ip-pool/spec.md`

This document models every entity and CRD change required to ship Nuclear Option as a playable module and to support operator-controlled load-balancer address pool assignment.

---

## Track B: Load-Balancer Address Pool (CRD Changes)

### 1. Proposed GameServerNetworking Additions

**Current struct** (`operator/api/v1alpha1/gameserver_types.go`, lines 225–276):

```go
type GameServerNetworking struct {
	Expose string `json:"expose,omitempty"`                      // Enum: ClusterIP;NodePort;LoadBalancer;Hostport
	Hostname string `json:"hostname,omitempty"`                  // DNS-1123 name, MaxLength 253
	ServiceAnnotations map[string]string `json:"serviceAnnotations,omitempty"` // MaxProperties 32
	PortOverrides []PortOverride `json:"portOverrides,omitempty"`
	SourceRanges []string `json:"sourceRanges,omitempty"`         // MaxItems 20, CEL: each contains '/'
	Tunnel *GameServerTunnel `json:"tunnel,omitempty"`
}
```

**Proposed new fields**:

| Field | Type | Validation | Purpose |
|-------|------|-----------|---------|
| `addressPool` | `string` | `MaxLength: 63`; pattern: `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$` (DNS-1123 label) | Names the load-balancer address pool from which this server's public address should be drawn. Empty means use the cluster's default pool (backward-compatible). Only honored when `Expose == "LoadBalancer"`. |
| `address` | `string` | `MaxLength: 45`; no regex (see rationale below) | Requests a specific IP address (IPv4 or IPv6) instead of or in addition to a pool name. When set, the cluster's address manager will attempt to assign this exact address to the server's public endpoint. Empty means let the pool assign any available address. Only honored when `Expose == "LoadBalancer"`. |

**Kubebuilder markers for `addressPool`**:

```go
// AddressPool is an optional name of a load-balancer address pool from which this
// server's public IP should be drawn. Only honored when Expose=LoadBalancer.
// If unset, the server receives an address from the cluster's default pool.
// +kubebuilder:validation:MaxLength=63
// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`
// +optional
AddressPool string `json:"addressPool,omitempty"`
```

**Kubebuilder markers for `address`**:

```go
// Address is an optional request for a specific IP address (IPv4 or IPv6).
// When set, the cluster's address manager will attempt to assign this exact
// address to this server's public endpoint. Only honored when Expose=LoadBalancer.
// If unset, an address is drawn from the named pool (or default pool if no pool
// is specified). The field accepts both IPv4 (e.g. "192.0.2.1") and IPv6
// (e.g. "2001:db8::1") addresses. No validation is performed at CRD time;
// the address manager's assignment result determines success or failure.
// +kubebuilder:validation:MaxLength=45
// +optional
Address string `json:"address,omitempty"`
```

**Rationale for `address` field design**:

- **No regex pattern**: IPv4 and IPv6 have very different syntax (IPv4 is `\d+\.\d+\.\d+\.\d+`; IPv6 is a complex hex/colon mix). A regex that handles both is error-prone and often fails on valid IPv6 notation (leading zeros, compressed notation, etc.). A simple `MaxLength: 45` (sufficient for any IPv6 address in canonical form) combined with **operator-side DNS/IP parsing** is simpler and more robust.
- **Deferred validation**: The operator reconciler will attempt to apply the address to the load-balancer Service's `status` or through annotations. If the format is invalid or the address is unavailable, the pool-assignment condition will report the failure with a human-readable message (see Status Conditions below), and the operator logs provide the specific parse error.
- **CEL cost**: Avoiding a complex regex saves CEL evaluation cycles and keeps the CRD under the apiserver's cost budgets. This repo has previously had CRDs rejected for unbounded CEL rules (`project_crd_cel_cost_budget.md`); a simple bound-only check is safer.

---

### 2. Status Surface: Assigned Address & Pool

**Decision: Extend `GameServerEndpoint` (not a separate status field)**

**Current `GameServerEndpoint` struct** (`operator/api/v1alpha1/gameserver_types.go`, lines 492–511):

```go
type GameServerEndpoint struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Port int32  `json:"port"`
	Protocol corev1.Protocol `json:"protocol,omitempty"`
	Private bool `json:"private,omitempty"`
	TunnelProvider string `json:"tunnelProvider,omitempty"`
}
```

**Proposed new fields**:

| Field | Type | Purpose |
|-------|------|---------|
| `pool` | `string` | The name of the address pool this endpoint's address came from (e.g., "production-us-east"). Empty if no pool was specified. Read-only; set by the reconciler when populating `status.endpoints` from the Service. |

**Kubebuilder marker**:

```go
// Pool is the name of the load-balancer address pool this endpoint's address came from.
// Empty when no pool preference was set or when Expose is not LoadBalancer.
// This field is read-only and set by the operator during reconciliation.
// +optional
Pool string `json:"pool,omitempty"`
```

**Justification**: 

- **Why extend `GameServerEndpoint` and not add a separate field**: The current `endpointsFromService` function (`operator/internal/controller/gameserver_status.go`, lines 616–644) already populates endpoints from the Service's `.status.loadBalancer.ingress[0]` for LoadBalancer type. Extending `GameServerEndpoint` keeps the pool metadata colocated with the address itself, matching how `TunnelProvider` is already colocated.
- **How populated**: The reconciler reads the pool name from `spec.networking.addressPool` and includes it in each endpoint it writes to status. If the pool name is empty or `Expose != LoadBalancer`, the pool field is omitted (empty string).
- **Dashboard visibility**: The dashboard's networking details will render the assigned pool name alongside the address, providing full visibility into pool assignment (FR-019, FR-025).

---

### 3. Status Condition for Pool Assignment

**New condition type**: `"PoolAssignment"`

This condition tracks the success or failure of assigning an address from the requested pool.

**Condition vocabulary**:

| Reason | Status | Example Message | Scenario |
|--------|--------|-----------------|----------|
| `PoolNotFound` | False | `Pool "unknown-pool" not found in cluster` | The `spec.networking.addressPool` names a pool that does not exist in the cluster. The operator detected this via annotations or config-map lookups (address-manager specific). |
| `PoolExhausted` | False | `Address pool "production-us-east" is exhausted; no addresses available` | The named pool exists but all its addresses are currently assigned to other servers. |
| `AddressInUse` | False | `Requested address 192.0.2.100 is already assigned to gameserver-prod-1 in namespace games` | The `spec.networking.address` field specifies an address that is already assigned to another server. |
| `IgnoredIncompatibleMode` | False | `Pool preference is ignored when exposure mode is not 'Load Balancer' (current: "ClusterIP")` | The operator is not honoring the pool preference because `spec.networking.expose != "LoadBalancer"`. This is informational — the server boots normally, but the pool assignment request has no effect. |
| `Assigned` | True | `Assigned address 192.0.2.150 from pool "production-us-east"` | The server has been successfully assigned an address from the requested pool (or from the default pool if no pool was specified). |

**Kubebuilder marker** (in `GameServerSpec` CEL validation, or as part of reconciliation output):

```
Reason: PoolNotFound | PoolExhausted | AddressInUse | IgnoredIncompatibleMode | Assigned
```

**Integration with `computeConditions`** (`operator/internal/controller/gameserver_status.go`, lines 211–300):

The PoolAssignment condition is computed independently of the phase-based conditions (Ready, Progressing, Healthy). It is managed by the pool-assignment reconciliation logic and upserted into `status.conditions` via `upsertCondition()`, following the same pattern as tunnel conditions (`computeTunnelConditions`, lines 443–550).

---

### 4. Backward Compatibility (FR-017)

**Existing servers with no pool preference remain unchanged**:

- When both `spec.networking.addressPool` and `spec.networking.address` are empty, the behavior is identical to today: the server's Service receives an address from the cluster's default load-balancer pool.
- No migration, data conversion, or operator-side logic changes are required.
- The `PoolAssignment` condition reports `Assigned` with a message like `"Assigned address 192.0.2.1 from default pool"` (or omits the message if the pool name is not discoverable).
- Servers created before the pool-override feature existed will continue to work unchanged.

---

## Track A: Nuclear Option Module (Template Schema)

### 5. Module Metadata and Template Shape

**Module metadata** (`modules/nuclear-option/module.yaml`):

```yaml
apiVersion: gameplane.local/module/v1
name: nuclear-option
displayName: Nuclear Option
version: 1.0.0
game: nuclear-option
categories: [Shooter, PvP, Simulation]
summary: Multiplayer game featuring large-scale destructible environments
homepage: https://store.steampowered.com/app/2168680/Nuclear_Option/
license: proprietary
gameplaneMinVersion: 0.2.0
icon: icon.png
```

**Template skeleton** (`modules/nuclear-option/template.yaml`):

```yaml
apiVersion: gameplane.local/v1alpha1
kind: GameTemplate
metadata:
  name: nuclear-option
  labels:
    gameplane.local/module: nuclear-option

spec:
  displayName: Nuclear Option
  game: nuclear-option
  version: 1.0.0
  categories: [Shooter, PvP, Simulation]
  accentColor: "#ff6b35"  # Orange brand color (example; verify against game assets)

  description: |
    Large-scale multiplayer game with destructible environments.
    Dedicated server with full remote administration via the Remote Console.

  # Base image pinned by digest (required by validate.py).
  # Steam app 3930080 is the dedicated server (see spec Claim 1).
  # Image must be built/provided separately; this template assumes it
  # exists in a registry as <repo>:<version>@sha256:...
  # UNVERIFIED: The exact image source and availability are confirmed
  # during implementation (Claim 1 in spec).
  image: ghcr.io/gameplane/nuclear-option:1.0.0@sha256:placeholder

  # Single port: UDP 7777 for game connections (Claim 2: UNVERIFIED)
  # Query port (7778) and remote-command port (7779) are internal-only.
  # Wake-on-connect is deferred; sentinel falls back to generic UDP packets-in-window heuristic.
  ports:
    - name: game
      containerPort: 7777
      protocol: UDP
      advertise: true
    - name: query
      containerPort: 7778
      protocol: UDP
      advertise: false  # Internal only; used by the operator's join probe
    - name: rcon
      containerPort: 7779
      protocol: TCP
      advertise: false  # Internal-only remote-command port; never advertised

  storage:
    size: 30Gi
    mountPath: /home/steam/NuclearOption

  resources:
    requests:
      cpu: 2
      memory: 8Gi
    limits:
      cpu: 4
      memory: 16Gi

  probes:
    readiness:
      exec:
        command:
          - /bin/sh
          - -c
          - grep -q "Waiting for Players before loading next map" ./logs/server-*.log
      initialDelaySeconds: 45
      periodSeconds: 10
      failureThreshold: 30
      timeoutSeconds: 5
    # The readiness probe checks for the log line "[DedicatedServerManager] Waiting for Players before loading next map"
    # in ./logs/server-<timestamp>.log, which is the publisher-confirmed readiness signal indicating
    # the server is accepting players. The server writes logs to ./logs/server-<timestamp>.log
    # (verified from upstream documentation).

  # Remote-console configuration.
  # UNVERIFIED: Protocol details are third-party documentation only
  # (Claim 3, spec). The protocol is implemented as "nuclearoption" below.
  rcon:
    protocol: nuclearoption
    port: 7779
    # RCON password is generated and stored by the operator.
    # Nuclear Option is not known to read RCON password from environment;
    # the password is applied at the password file path below (in configFiles).

  consoleMode: rcon

  # Configuration schema: fields surfaced in the Create Server wizard
  # and persisted as env/file per target.
  configSchema:
    - name: server_name
      displayName: Server Name
      type: string
      required: true
      default: "Nuclear Option Server"
      # Sanity bound; exact limit depends on game's config parsing.
      # UNVERIFIED during implementation.
      constraints:
        maxLength: 64

    - name: password
      displayName: Server Password
      type: password
      required: true
      default: ""
      # Password for player join; set in config file.
      target: file

    - name: max_players
      displayName: Max Players
      type: int
      required: true
      default: "8"
      constraints:
        min: 1
        max: 64  # UNVERIFIED game limit; adjust based on real server behavior.

    - name: mission_rotation_mode
      displayName: Mission Rotation Mode
      type: enum
      enum: [sequential, random]
      required: true
      default: sequential
      # Mode for cycling through missions: sequential (in order) or random.

    - name: mission_list
      displayName: Mission List
      type: string
      required: true
      # Comma-separated list of mission names available in the game.
      # UNVERIFIED: Exact mission names and availability per version.
      default: "DeathmatchSmall,DeathmatchMedium"
      description: |
        Comma-separated list of mission names to rotate through.
        Available missions depend on the game version installed.
      # Validation: must be a valid mission name (done server-side).

  # Configuration files: renders the DedicatedServerConfig.json and password file.
  configFiles:
    - path: DedicatedServerConfig.json
      # Go text/template rendering the game's JSON config.
      # UNVERIFIED: Exact structure and field names (Claim 3, spec).
      # The FieldOverrides pattern with IsOverride/Value is drawn from
      # third-party hosting-provider documentation.
      template: |
        {
          "ServerName": "{{ .Config.server_name }}",
          "MaxPlayers": {{ .Config.max_players }},
          "MissionRotationMode": "{{ .Config.mission_rotation_mode }}",
          "AvailableMissions": {{ json .Config.mission_list | split "," }},
          "Ports": {
            "Game": {
              "IsOverride": true,
              "Value": 7777
            },
            "Query": {
              "IsOverride": true,
              "Value": 7778
            },
            "RemoteCommand": {
              "IsOverride": true,
              "Value": 7779
            }
          },
          "RemoteCommandPassword": "{{ .Config.rcon_password }}",
          "BanListPaths": ["ban_list.txt"]
        }

    - path: rcon_password
      # Simple plaintext password file read by the server.
      # UNVERIFIED: Whether this is how the game configures the RCON password
      # (Claim 3, spec). If the game reads it from environment instead,
      # this file is removed and rcon.passwordEnv is set instead.
      template: "{{ .Config.password }}"

  # Remote-console capabilities: moderation commands via RCON.
  capabilities:
    actions:
      - id: player-list
        displayName: Player List
        group: Players
        icon: users
        description: Show all connected players
        transport: rcon
        command: "get-player-list"
        # No parameters; the remote command returns the list.

      - id: kick-player
        displayName: Kick Player
        group: Players
        icon: log-out
        description: Disconnect a player from the server
        danger: true
        confirm: true
        transport: rcon
        command: "kick-player {{.Params.steam_id}}"
        params:
          - name: steam_id
            displayName: Steam ID
            type: string
            required: true
            description: The 64-bit Steam ID of the player to kick

      - id: banlist-add
        displayName: Ban Player
        group: Players
        icon: ban
        description: Add a player to the ban list
        danger: true
        confirm: true
        transport: rcon
        command: "banlist-add {{.Params.steam_id}}"
        params:
          - name: steam_id
            displayName: Steam ID
            type: string
            required: true

      - id: banlist-remove
        displayName: Unban Player
        group: Players
        icon: undo
        transport: rcon
        command: "banlist-remove {{.Params.steam_id}}"
        params:
          - name: steam_id
            displayName: Steam ID
            type: string
            required: true

      - id: send-chat-message
        displayName: Send Chat Message
        group: Server
        icon: megaphone
        description: Broadcast a message to all players
        transport: rcon
        command: "send-chat-message {{.Params.message}}"
        params:
          - name: message
            displayName: Message
            type: string
            required: true
            description: Message text (max 256 characters, UNVERIFIED)

      - id: set-next-mission
        displayName: Set Next Mission
        group: World
        icon: target
        description: Set the mission to load after the current one ends
        transport: rcon
        command: "set-next-mission {{.Params.mission_name}}"
        params:
          - name: mission_name
            displayName: Mission Name
            type: string
            required: true
            description: Name of a mission from the mission list
            # Validation: must be a mission in the configured mission_list (server-side check)
```

---

### 6. New RCON Protocol: `nuclearoption`

**Protocol identifier**: `nuclearoption`

**Wire format** (Length-Prefixed JSON over TCP):

This protocol is **UNVERIFIED** and drawn from third-party hosting-provider documentation (spec Claim 3). It must be confirmed against a real running Nuclear Option dedicated server during implementation.

**Request frame**:

```
[4 bytes: little-endian uint32 frame length]
[N bytes: UTF-8 JSON object]
```

The 4-byte little-endian prefix carries the number of UTF-8 bytes in the JSON body only; the prefix itself is never counted. For example, `get-player-list` has a 9-byte JSON body (`{"name": "get-player-list", "arguments": []}`), so the prefix is `0x09000000` (little-endian).

**JSON request body**:

```json
{
  "name": "<command-name>",
  "arguments": [<arg0>, <arg1>, ...]
}
```

Example: `get-player-list` (no arguments):

```json
{"name": "get-player-list", "arguments": []}
```

Example: `kick-player` with Steam ID:

```json
{"name": "kick-player", "arguments": ["76561198012345678"]}
```

**Response frame**:

```
[4 bytes: little-endian uint32 status code]
[4 bytes: little-endian uint32 JSON body length (0 if no body)]
[N bytes: UTF-8 JSON response body (if length > 0)]
```

**Status codes**:

| Code | Meaning | Example Scenario |
|------|---------|------------------|
| 2000 | Success | Command executed; response contains result data or empty `{}` |
| 4000 | Invalid request | Malformed JSON or missing required fields |
| 4001 | Unknown command | Command name not recognized |
| 4002 | Invalid parameters | Parameter count or type mismatch |
| 4003 | Player not found | Kick/ban/unban target Steam ID not on server or ban list |
| 4004 | (reserved) | |
| 4005 | (reserved) | |
| 5000 | Server error | Internal game server error |
| 5001 | Game not ready | Server not accepting commands (still loading, shutting down) |
| 5002 | Command execution failed | Command started but failed to complete |

**Response body shapes** (UNVERIFIED; confirmed during implementation):

- **`get-player-list` (200 OK)**:
  **UNVERIFIED — response body shape not specified upstream; confirm against a live server.**

- **`kick-player` (200 OK)**:
  **UNVERIFIED — response body shape not specified upstream; confirm against a live server.**

- **`banlist-add` (200 OK)**:
  **UNVERIFIED — response body shape not specified upstream; confirm against a live server.**

- **`banlist-remove` (200 OK)**:
  **UNVERIFIED — response body shape not specified upstream; confirm against a live server.**

- **`send-chat-message` (200 OK)**:
  **UNVERIFIED — response body shape not specified upstream; confirm against a live server.**

- **`set-next-mission` (200 OK)**:
  **UNVERIFIED — response body shape not specified upstream; confirm against a live server.**

**Why a new protocol identifier**:

The existing RCON protocol list (`operator/api/v1alpha1/gametemplate_types.go`, RCONSpec) includes `source`, `telnet`, `websocket`, `battleye`, `satisfactory`, `palworld`, `none`. None of these match Nuclear Option's length-prefixed JSON request/response framing with numeric status codes. A new `nuclearoption` protocol ID is required to distinguish it and allow the agent to apply the correct wire-format handling.

**Agent implementation** (not in this data model; implementation detail):

The agent sidecar will recognize `rcon.protocol == "nuclearoption"` and:

1. On first connection, send a length-prefixed JSON frame requesting `{"name": "connect", "arguments": []}` or similar (UNVERIFIED; may not be needed).
2. For each command routed via remote console, format it as a JSON request frame with the given status code handling and parse responses according to the response frame layout above.
3. Map status codes to human-readable error messages for the dashboard console output.

---

### 7. Capabilities: Remote Console Actions (Summary)

See section 5 above for the full `capabilities.actions` list. Each action maps to one of the 19 real server commands:

| Action ID | Server Command | Parameters | Note |
|-----------|----------------|------------|------|
| `player-list` | `get-player-list` | none | Lists connected players; returns list of Player Entries (see section 8) |
| `kick-player` | `kick-player` | steam_id | Disconnects player from server |
| `banlist-add` | `banlist-add` | steam_id | Adds to persistent ban list |
| `banlist-remove` | `banlist-remove` | steam_id | Removes from ban list |
| `send-chat-message` | `send-chat-message` | message | Broadcasts in-game chat |
| `set-next-mission` | `set-next-mission` | mission_name | Changes next mission in rotation |

The following server commands (per spec Claim 3) are **not yet modeled as actions**; they may be added later if dashboard support is needed:

- `update-ready`, `reload-config`, `get-mission-time`, `get-mission`, `get-server-id`, `set-time-remaining`, `clear-next-mission`, `banlist-reload`, `banlist-clear`, `get-mission-rotation`, `set-mission-rotation`, `unkick-player`, `clear-kicked-players`

---

### 8. Runtime Entities (Not CRD Fields)

These entities are returned by remote-console commands and logged/displayed by the dashboard, but do not require CRD changes.

#### Player Entry — modelled as TWO distinct entities

Spec 002 FR-007 and User Story 3 Acceptance Scenario 1 promise operators a player list including a display name. The dedicated server cannot supply one — it runs headlessly and does not cache player names.

**RESOLUTION (decided; not open)**: the spec **stands as written** — FR-007 and US3/AC1 are **not** amended. Display names are hydrated by a **Steam Web API lookup performed in the API server** (`api/internal/...`), layered on top of the unchanged wire response. The previously-open gate on this question is **CLOSED**.

Because of that, "Player Entry" is two different things and must be modelled as two, never merged:

##### 8a. Player Entry (WIRE) — what the game returns, what the agent returns

**Returned by**: `get-player-list` over the `nuclearoption` remote-command protocol.

**Shape** (verified from upstream documentation) — **unchanged, and it must stay unchanged**:

```json
{
  "Players": [
    {
      "steamId": "0123456789",
      "faction": "Boscali"
    },
    {
      "steamId": "9876543210",
      "faction": "Primeva"
    }
  ]
}
```

| Field | Type | Purpose |
|-------|------|---------|
| `steamId` | string | 64-bit Steam ID as a decimal string. The identifier. Used for kick/ban/unban operations. |
| `faction` | string | Team or faction affiliation (e.g., "Boscali", "Primeva", etc.). Field name and enum values per upstream documentation. |

**Hard constraint**: the wire entity has **no `displayName` field**, and none may be added to the protocol codec, the frame parser, or the agent's response struct. The wrapper key is `Players` (capital `P`); the id field is `steamId` (lowercase `d`). The agent returns exactly what the game returns — nothing more. Name hydration is a presentation concern layered on top in the API, consistent with the project rule that the API is a UX layer and the agent mirrors the game.

##### 8b. Player Entry (PRESENTATION) — what the API serves to the dashboard

The API's `/servers/{name}/players` response carries `players` as a flat `string[]` today (`agent/internal/players/players.go:61`, `web/src/types.ts:742`). §8b is therefore a **change** to that contract, gated on the T023 players-capability decision: flat-string entries must keep working unchanged for every other game.

The API server takes the wire entries, resolves names in a batch, and serves an enriched entity:

| Field | Type | Optional? | Purpose |
|-------|------|-----------|---------|
| `steamId` | string | required | Carried through verbatim from the wire entity. **The identifier.** |
| `faction` | string | **OPTIONAL** | Carried through verbatim from the wire entity when the game supplies one. Absent for every game that has no faction concept — the shared `/players` route serves all games. |
| `displayName` | string | **OPTIONAL / nullable** | Steam persona name resolved via the Steam Web API. Absent/null whenever resolution did not succeed. |

`displayName` is modelled as optional/nullable **precisely because resolution can fail** — no key configured, Steam unreachable, request timed out, rate-limited, or the individual id simply not returned by Steam. When it is absent, **the dashboard falls back to rendering the raw Steam ID in the name column**. The player list renders either way.

**Moderation keys on `steamId` only.** Kick, ban, and unban continue to take the Steam ID as their argument, exactly as today. `displayName` is display-only and must never become the identifier used for a moderation action.

**Graceful degradation is mandatory, not best-effort.** Name resolution must never block, fail, or error the player-list response. Spec SC-004 bounds a moderation command result at 5 seconds, so the lookup carries a hard bounded timeout and degrades (drops names) rather than exceeding it.

##### 8c. Steam Name Resolver (API-server component)

Not a persisted entity; the behavioural contract the two entities above depend on.

| Property | Value |
|----------|-------|
| Location | API server, package `api/internal/steam/` — **never the agent sidecar** |
| Why not the agent | `charts/gameplane/templates/networkpolicies.yaml` declares `default-deny-egress` (policy at line 24, `podSelector: {}` at line 28) over every pod in the games namespace; the only outbound path is the opt-in `allow-game-public-egress` policy (line 149) that exists for SteamCMD downloads. A resolver in the agent would need a new egress hole in **every** game pod and the Steam API key distributed to **every** game pod. The API server sits in the control-plane namespace: one egress path, one Secret, one shared cache. |
| Endpoint | Steam Web API `ISteamUser/GetPlayerSummaries/v2` |
| Batching | The endpoint accepts up to **100** steamids per call, so the resolver batches ids from one player list into as few calls as possible — never one request per player. |
| Egress guard | All outbound calls dial through this repo's `netguard` package using the **strict `IsPublic`** policy (Steam is a public internet endpoint; `IsAllowed` is the permissive operator policy and is wrong here). `api/` is **already** a netguard importer — `api/go.mod` lines 5–9 declare the requirement plus the local `replace`, and `api/internal/notify` imports it today — so **no new `go.work`/`go.mod` wiring is required**. What is in scope: `netguard`'s 91% coverage gate applies to any change made inside that package, and the resolver's own tests land under the `api` 80% gate. |
| Failure behaviour | Degrade, never fail: return the wire entries with `displayName` absent. |

##### 8d. Name Resolution Cache

A **small, bounded, in-process RAM cache** inside the API server. It exists for exactly one reason: to stop the dashboard from spamming Steam. Everything else in this data model is backed by a CRD or by the database — **this is not**. There is no table, no migration, no Redis, no file on disk.

###### 8d-i. Cache Entry (the stored value)

One entry per Steam ID. An entry is either **positive** (a name was resolved) or **negative** (resolution failed for this id). **A negative entry is a first-class cached value, not an absence** — it occupies a slot, it has its own expiry, and while it is live the id is *not* re-sent to Steam.

| Field | Type | Purpose |
|-------|------|---------|
| `steamID` (key) | string | The 64-bit Steam ID as a decimal string. The sole cache key — one entry per player, shared across every server and every dashboard session in the install. |
| `displayName` | string | The resolved Steam persona name. Meaningful **only** on a positive entry. On a negative entry it is the empty string and must never be served. |
| `negative` | bool | `false` = positive entry (`displayName` is authoritative and is served to the dashboard). `true` = Steam did not yield a usable name for this id; the dashboard renders the raw Steam ID. |
| `expiresAt` | timestamp | Absolute expiry. Computed at insert as `now + ttl` for a positive entry, `now + negativeTTL` for a negative one. An entry at or past `expiresAt` is treated as a miss and re-resolved on next demand. |

**What produces a negative entry**: Steam returned no `player` object for that id (private profile, deleted/nonexistent account, malformed or non-numeric id). These are *permanent-ish* conditions, and without negative caching **every player-list refresh would retry every unresolvable id forever** — precisely the Steam spam this cache exists to prevent. Negative caching is therefore mandatory, not an optimization.

**What does NOT produce a negative entry**: a transport-level failure of the *call itself* — no API key configured, Steam unreachable, HTTP 5xx, rate-limited (429), or the request exceeding its timeout. Those say nothing about the individual id, so nothing is written and the ids stay plain misses, eligible for the next attempt.

###### 8d-ii. Cache Configuration

Its own small entity. Every knob is bounded by design; none of them can be set to "unlimited". These are the identifiers the implementation uses end to end — `MaxEntries` / `TTL` / `NegativeTTL` / `Timeout` in Go, `maxEntries` / `ttl` / `negativeTTL` / `timeout` under `api.steam.cache` in the chart, and `GAMEPLANE_STEAM_CACHE_MAX_ENTRIES` / `GAMEPLANE_STEAM_CACHE_TTL` / `GAMEPLANE_STEAM_CACHE_NEGATIVE_TTL` / `GAMEPLANE_STEAM_TIMEOUT` in the environment. A zero or negative value falls back to the default; it never disables the bound or the timeout.

| Setting | Default | Meaning |
|---------|---------|---------|
| `maxEntries` | **10000** | Hard upper bound on the **number of entries**, not on bytes. Reaching it evicts the least-recently-used entry to make room. Positive and negative entries share the one budget. A Steam ID plus a persona name is tens of bytes, so even a full cache stays well under a megabyte (§8d-vii). |
| `ttl` | **12 hours** | Lifetime of a resolved name. Deliberately **hours, not minutes**: Steam persona names change rarely, and a stale name is harmless because every moderation action keys on `steamId` and never on the name. Long TTL is what collapses the Steam call volume. |
| `negativeTTL` | **15 minutes** | Lifetime of a negative entry. **Shorter than `ttl`** so a player who unlocks a private profile or fixes their account recovers a name within a reasonable window, while a tight refresh loop still cannot re-ask Steam about them. |
| `timeout` | **2 seconds** | Hard bound on one upstream Steam call, well inside the 5s SC-004 budget (§8b). On expiry the call is abandoned, nothing is cached, and the affected players render as raw Steam IDs. |

###### 8d-iii. Single-Flight De-duplication

Concurrent player-list requests needing the same unresolved ids **must collapse into one upstream Steam call**, not N identical ones. Two dashboards open on the same server, or one dashboard polling while another loads, must not multiply Steam traffic.

`golang.org/x/sync/singleflight` is the idiomatic implementation and is **already available and already in use in this very module**: `golang.org/x/sync v0.22.0` is a direct requirement in `api/go.mod`, and `api/internal/registry/registry.go` imports `golang.org/x/sync/singleflight` (line 38) and holds a `singleflight.Group` (line 195) today. Adopting it here adds **no new dependency** and follows an in-repo precedent. If for any reason the package is not adopted, the *behaviour* above is still required, and a small hand-rolled equivalent (a mutex-guarded map of in-flight keys to a shared result channel) satisfies it without pulling in anything new.

###### 8d-iv. Batching (unchanged, restated here because it is what makes the numbers work)

`ISteamUser/GetPlayerSummaries/v2` accepts up to **100 steamids per call**. A cache miss covering several players is therefore **one request, not one per player** (§8c). Batching and the 12-hour TTL are the two multipliers behind the capacity note below.

###### 8d-v. Invariants

1. **Bounded**: the cache never holds more than `maxEntries` entries. On insert into a full cache the least-recently-used entry is evicted (**LRU**). Memory use has a ceiling that does not depend on player churn, uptime, or number of servers.
2. **Concurrency-safe**: the cache is read and written from concurrent HTTP handlers and must be safe for concurrent use. Reads, writes, expiry checks, and eviction all happen under the cache's own synchronization; callers never coordinate externally.
3. **No durability requirement**: the cache holds nothing that must survive a restart. A cold start is a cold cache; the next player list simply re-resolves. Loss of the entire cache is a performance event, never a correctness event.
4. **Miss degrades, never fails**: a miss, an expired entry, a negative entry, or a timed-out batch yields the raw Steam ID in the name column (§8b). Nothing about the cache may block, error, or fail the player-list response.
5. **Never an identifier**: cached names are display-only. Kick, ban, and unban key on `steamId` only — a stale or wrong cached name can never mis-target a moderation action.
6. **Negative is a value**: a live negative entry suppresses re-querying Steam for that id exactly as a positive entry does. Code must distinguish "cached negative" from "not cached".

###### 8d-vi. Non-Goals (stated explicitly, because everything else here is CRD- or DB-backed)

- **No database table and no migration.** Display names are cosmetic; adding an append-only migration under `api/internal/db/migrations/` for them is explicitly out of scope.
- **No persistence of any kind** — not to disk, not to a PVC, not to a ConfigMap.
- **No Redis, memcached, or any external cache service.**
- **No shared or distributed cache across replicas.** Each API replica keeps its own; with N replicas the worst case is N duplicate Steam calls per TTL window, which is a handful of requests per half-day and is not worth a shared store.
- **Not an API resource.** The cache is never exposed to the browser, never listed, never invalidated through an endpoint.

###### 8d-vii. Capacity note (substantiating "small")

One entry is a Steam ID (17 ASCII digits) plus a short persona name (Steam caps personas at 32 characters) plus a bool and a timestamp — on the order of **tens of bytes**, call it ~100 bytes with Go map and list overhead. Even at the top of the range, **~10,000 entries is well under a megabyte** — a rounding error against the API server's footprint, and far more distinct players than any single install will hold live.

Call volume falls out the same way: a busy 16-player server needs **one batched call per 12-hour TTL window**, not one per dashboard refresh. Steam's documented quota is on the order of **100,000 calls/day per key**, so even hundreds of servers leave an enormous margin.

##### 8e. Steam Web API Key (configuration)

| Property | Value |
|----------|-------|
| Nature | A **credential**. |
| Storage | Provisioned as a Kubernetes **Secret**, surfaced through a Helm value, mounted/injected into the API server only. |
| Optionality | **Optional.** Absent is a supported, first-class configuration — not an error, not a startup failure, not a degraded-health condition. |
| Absent behaviour | The resolver is inert; every player renders with the raw Steam ID. The player list is fully functional; moderation is fully functional. |
| Handling | Never logged (not at any level, not in error strings), never returned to the browser in any API response or config endpoint, never committed to the repo, never placed in a GameServer CR or a game pod. |

#### Ban List Entry

**Stored on disk** in `ban_list.txt` (auto-managed by the server; not directly queried via RCON).

**Shape** (UNVERIFIED):

A single line per entry. Format unknown; likely a space/comma-separated tuple of (SteamID, optional reason, optional timestamp).

The operator never reads or parses this file directly; the server's `banlist-add`, `banlist-remove`, `banlist-clear`, and `banlist-reload` commands are the only management interface.

#### Mission Rotation

**Returned by**: `get-mission-rotation`

**Shape** (UNVERIFIED; command existence assumed):

```json
{
  "missions": ["Mission1", "Mission2", "Mission3"],
  "current": "Mission1",
  "next": "Mission2"
}
```

| Field | Type | Purpose |
|-------|------|---------|
| `missions` | string[] | All available mission names the server can load. Used for validation when the operator calls `set-next-mission`. |
| `current` | string | Name of the currently active mission. |
| `next` | string | Name of the mission queued for the next load. |

#### Remote Console Session

**Lifecycle**: Created by the operator via the dashboard console, lives for the duration of the operator's session, closed when the operator navigates away.

**Shape**:

A session is not a persisted object; it is a live WebSocket connection from the dashboard to the API, which proxies commands to the agent's RCON port. The session state is:

- WebSocket connection status (open/closed)
- Last command sent and its response (buffered for display)
- Connected player count (for UX feedback)

The remote console does not require modeling as a data structure in the CRD; it is a runtime UI feature.

---

## Summary of Changes

### CRD Changes (Track B)

1. **`GameServerNetworking`** (`operator/api/v1alpha1/gameserver_types.go`):
   - Add `addressPool: string` (MaxLength 63, DNS-1123 pattern)
   - Add `address: string` (MaxLength 45, no regex)

2. **`GameServerEndpoint`** (`operator/api/v1alpha1/gameserver_types.go`):
   - Add `pool: string` (read-only, populated by reconciler)

3. **`GameServerStatus`** (`operator/api/v1alpha1/gameserver_types.go`):
   - Condition Type `"PoolAssignment"` with Reasons: `PoolNotFound`, `PoolExhausted`, `AddressInUse`, `IgnoredIncompatibleMode`, `Assigned`

### Module Addition (Track A)

1. **Nuclear Option module** (`modules/nuclear-option/`):
   - `module.yaml`: Metadata (name, displayName, version, categories, etc.)
   - `template.yaml`: Full GameTemplate spec with ports, configSchema, configFiles, RCON (protocol: `nuclearoption`), and actions

2. **New RCON protocol**: `nuclearoption` with length-prefixed JSON request/response framing

### API-Server Addition (display-name hydration)

1. **Steam name resolver** in `api/internal/steam/`: batched `ISteamUser/GetPlayerSummaries/v2` lookups (≤100 ids per call), dialled through `netguard`'s strict `IsPublic` policy, with a hard bounded timeout.
2. **Small bounded in-process RAM cache** keyed on Steam ID (§8d): entry-count bound with LRU eviction (10000 entries), 12h positive TTL (`ttl`), shorter 15m `negativeTTL` so unresolvable ids are not retried forever, a 2s per-call `timeout`, single-flight de-duplication of concurrent lookups (`golang.org/x/sync/singleflight` is already a direct `api/go.mod` requirement and is already imported by `api/internal/registry`), and safe for concurrent use. A miss degrades to the raw Steam ID rather than erroring. **Non-goals: no database table, no migration, no persistence, no shared/distributed cache across replicas.**
3. **Optional Steam Web API key** as a Kubernetes Secret surfaced via a Helm value; absent means degrade (raw Steam IDs), never fail.
4. **No new module wiring**: `api/` already requires and replaces `netguard` (`api/go.mod` lines 5–9, imported by `api/internal/notify`). This is a new *consumer of `netguard.IsPublic`*, not a new importer. The netguard 91% gate still applies to any change made inside that package; the resolver's coverage lands under the api 80% gate.
5. **No change to the wire protocol**: no `displayName` on the wire, in the codec, or in the agent.

### Verification Checklist

- [ ] Claim 1: Dedicated server availability & platform (Steam app 3930080 downloadable, native Linux binary, no base-game ownership required)
- [ ] Claim 2: Network ports (UDP 7777 game, 7778 query, TCP 7779 remote-command confirmed)
- [ ] Claim 3: Remote-command protocol format (JSON request/response, status codes, command names)
- [ ] Claim 4: Readiness signal (log line or API response distinguishing "started" from "accepting players")
- [ ] Claim 5: On-disk log location and format (paths accessible to agent, format parseable)

---

## Notes

- **`address` field rationale**: No regex validation; rely on operator-side parsing and clear error messages from the address manager.
- **`pool` field in endpoints**: Read-only, set by reconciler during Service reconciliation.
- **Protocol identifier `nuclearoption`**: New; enables agent to route RCON frames through the correct wire-format handler.
- **All protocol details marked UNVERIFIED**: Must be confirmed against a real running dedicated server before implementation. The real-protocol E2E join test (spec FR-004) serves as the verification gate.
