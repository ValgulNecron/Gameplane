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

#### Player Entry

**Returned by**: `get-player-list`

**Shape** (verified from upstream documentation):

The dedicated server returns only the `steamId` and `faction` fields. The `displayName` field has been removed since the server runs headlessly and does not cache names. The external server can fetch steam name using Steam's Web API.

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
| `steamId` | string | 64-bit Steam ID as a decimal string. Used for kick/ban operations. |
| `faction` | string | Team or faction affiliation (e.g., "Boscali", "Primeva", etc.). Field name and enum values per upstream documentation. |

**SPEC CONFLICT**: Spec 002 FR-007 and User Story 3 Acceptance Scenario 1 both promise operators a player list including "display name". However, the dedicated server cannot supply display names — it runs headlessly and does not cache player names. Two options for resolution: (1) Resolve names out-of-band via the Steam Web API (new external dependency and new egress path, with SSRF/netguard implications); (2) Amend the spec to drop display name from the player list. This conflict must be resolved before implementation.

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
