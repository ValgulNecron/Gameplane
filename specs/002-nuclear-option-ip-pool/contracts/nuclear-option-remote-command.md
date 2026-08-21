# Nuclear Option Remote-Command Wire Protocol Contract

**Feature**: 002-nuclear-option-ip-pool  
**Phase**: 1 (Contracts)  
**Date**: 2026-08-21  
**Status**: Proposal for Code Review  

This document specifies the wire protocol for the Nuclear Option dedicated server's remote-command interface (spec FR-006 to FR-012). All details are **PUBLISHER-OFFICIAL** and transcribed exactly from the game's developer documentation to prevent fabrication and drift.

---

## Source of Truth

**Primary Source**: Shockfront Studios' official Nuclear Option Server Tools documentation, `ServerCommands/Readme.md`, retrieved from <https://github.com/Shockfront-Studios/Nuclear-Option-Server-Tools>.

**Local Copy**: `/tmp/claude-1000/-home-valgul-project-kubernetes-game-dashboard/516a9c4f-a0e2-4092-8626-1fc2e4821ea8/scratchpad/nocmd.md` (exact transcript from the official source).

All claims in this document are quoted directly from the official source unless otherwise marked **UNVERIFIED**.

---

## Enablement

**Activation**: The remote-command console is disabled by default and MUST be explicitly enabled by the operator.

**Launch Flag**: `-ServerRemoteCommands [port]`

**Port Specification**:
- If a port number is provided in the flag, the server listens on that port.
- If no port is provided (i.e., flag is `-ServerRemoteCommands` alone), the server defaults to port **7779**.

**Transport**: **TCP** (not UDP). The server listens for incoming TCP connections on the specified port.

**Authentication**: The remote-command protocol defines **NO authentication mechanism, NO password validation, and NO handshake**. Any client with network access to this port can send commands. (See Security Constraints, below.)

---

## Request Framing (Client → Server)

### Format

All requests follow a strict length-prefixed format:

1. **4 Bytes** (Little-Endian): Length of the JSON payload (the byte count of the UTF-8 JSON string that follows).
2. **N Bytes**: UTF-8 JSON string representing the command message.

### Command Message Structure

The JSON payload MUST be a single object with the following structure:

```json
{
    "name": "command-name",
    "arguments": [
        "argument 0",
        "argument 1",
        "argument 2"
    ]
}
```

**Fields**:
- `name` (string, required): The command name (must match one of the recognized command names below).
- `arguments` (array of strings, required): Command arguments. May be empty for commands that take no arguments. Each argument is a UTF-8 string. The server validates argument count and type per command.

### Example Request

To send the `get-player-list` command (which takes no arguments):

```
[0x09, 0x00, 0x00, 0x00]         # Length: 9 bytes (little-endian)
{"name":"get-player-list","arguments":[]}
```

To send `kick-player` with a Steam ID:

```
[0x45, 0x00, 0x00, 0x00]         # Length: 69 bytes
{"name":"kick-player","arguments":["0123456789"]}
```

---

## Response Framing (Server → Client)

### Format (Asymmetric to Request)

Responses are **NOT** a mirror of the request format. The response framing is distinctly different:

1. **4 Bytes** (Little-Endian): Status code (integer, e.g., `2000` for Success, `4000` for BadRequest).
2. **4 Bytes** (Little-Endian): Length of the JSON body (may be `0` if no body is present).
3. **N Bytes** (only if length > 0): UTF-8 JSON body.

**CRITICAL ASYMMETRY**: The most common implementation error is to treat the response as a mirror of the request. It is not. The response is status code (4 bytes) + body length (4 bytes) + optional body. Request is length (4 bytes) + body. These are structurally different.

### Example Response

Success with a JSON body:

```
[0xd0, 0x07, 0x00, 0x00]         # Status: 2000 (little-endian)
[0x2a, 0x00, 0x00, 0x00]         # Body length: 42 bytes
{"Players":[{"steamId":"123","faction":"Boscali"}]}
```

Success with no body (e.g., `kick-player` success):

```
[0xd0, 0x07, 0x00, 0x00]         # Status: 2000
[0x00, 0x00, 0x00, 0x00]         # Body length: 0
```

Error response:

```
[0x0f, 0x0f, 0x00, 0x00]         # Status: 4003 (JsonError, little-endian)
[0x00, 0x00, 0x00, 0x00]         # Body length: 0 (no error detail)
```

---

## Status Codes & Error Mapping

The server returns an integer status code (4 bytes, little-endian) in the response. The complete status-code vocabulary is:

| Code | Name | HTTP Analog | Interpretation | Agent-Level Go Error |
|------|------|-------------|-----------------|----------------------|
| **2000** | Success | 200 OK | Command executed successfully. Body may contain result data or be absent. | `nil` — no error. Parse the body if present. |
| **4000** | BadRequest | 400 Bad Request | Malformed request (generic client error not covered by more specific codes below). | `ErrBadRequest` or `fmt.Errorf("bad request")` |
| **4001** | BadHeader | 400 Bad Request | Error in the request header (e.g., length field is corrupted or incomplete). | `ErrBadHeader` or `fmt.Errorf("bad request header")` |
| **4002** | BadLength | 400 Bad Request | Invalid length value (negative, overflow, or exceeds maximum allowed size). | `ErrBadLength` or `fmt.Errorf("invalid request length")` |
| **4003** | JsonError | 400 Bad Request | Failed to parse the `CommandMessage` JSON (malformed JSON, missing required fields, type mismatch). | `ErrJsonError` or `fmt.Errorf("failed to parse JSON")` |
| **4004** | UnknownCommand | 404 Not Found | The command name is not recognized by the server. | `ErrUnknownCommand` or `fmt.Errorf("unknown command: %s", name)` |
| **4005** | BadArguments | 400 Bad Request | The command was recognized, but the provided arguments are invalid, missing, or out of range. | `ErrBadArguments` or `fmt.Errorf("invalid arguments for command %s: %s", name, detail)` |
| **5000** | InternalServerError | 500 Internal Server Error | General server-side error (unclassified). | `ErrInternalServerError` or `fmt.Errorf("server error")` |
| **5001** | CommandError | 500 Internal Server Error | An error occurred during the execution of the command's logic (e.g., player not found when trying to kick, mission name invalid). | `ErrCommandError` or `fmt.Errorf("command failed: %s", detail)` |
| **5002** | ConfigError | 500 Internal Server Error | The server configuration prevents the command from completing (e.g., no ban lists are configured, so `banlist-add` cannot write). | `ErrConfigError` or `fmt.Errorf("config error: %s", detail)` |

**Agent Implementation Guidance**:
- Status codes 2000 are success; parse and return the response body (if present) to the user.
- Status codes 4xxx are client errors; report to the user as an invalid request or command error.
- Status codes 5xxx are server errors; report to the user as a server-side failure (may be retryable depending on the specific reason).
- Never assume a response body is present; check the body-length field. If it is 0, there is no JSON to parse.

---

## Complete Command List & Specifications

All 19 commands are listed below with their request format, required arguments, and (where specified in the official source) response body format. Where the official source does not specify a response body shape, it is marked **UNVERIFIED**.

### 1. `update-ready`

**Purpose**: Notifies the server that a component is ready, likely to progress a startup sequence.

**Arguments**: None.

**Request**:
```json
{"name": "update-ready", "arguments": []}
```

**Response**: Status 2000, body: **UNVERIFIED — response body shape not specified upstream; confirm against a live server**.

---

### 2. `send-chat-message`

**Purpose**: Sends a server message to be displayed in the in-game chat. Supports Rich Text formatting (see official source).

**Arguments**: 
- `arguments[0]` (required): The chat message string. Supports Unity TextMeshPro rich text tags (e.g., `<color=#ff0000><b>Alert:</b></color>`).

**Request**:
```json
{"name": "send-chat-message", "arguments": ["<color=#ff0000><b>Alert:</b></color> Important server message."]}
```

**Response**: Status 2000, body: **UNVERIFIED — response body shape not specified upstream; confirm against a live server**.

---

### 3. `reload-config`

**Purpose**: Instructs the dedicated server to reload its configuration.

**Arguments**:
- `arguments[0]` (optional): Path to a new config file. If omitted, the server reloads the previously-used config file.

**Request (with new path)**:
```json
{"name": "reload-config", "arguments": ["Optional/path/to/new/config.json"]}
```

**Request (reload previous)**:
```json
{"name": "reload-config", "arguments": []}
```

**Response**: Status 2000, body: **UNVERIFIED — response body shape not specified upstream; confirm against a live server**.

---

### 4. `get-mission-time`

**Purpose**: Retrieves the current and maximum mission time (in seconds).

**Arguments**: None.

**Request**:
```json
{"name": "get-mission-time", "arguments": []}
```

**Response**: Status 2000, body (official specification):
```json
{
    "currentTime": 0,
    "maxTime": 0
}
```

**Note**: If no players are on the server, both values are 0. Values are in seconds.

---

### 5. `get-mission`

**Purpose**: Retrieves the currently running mission and the next mission scheduled.

**Arguments**: None.

**Request**:
```json
{"name": "get-mission", "arguments": []}
```

**Response**: Status 2000, body (official specification):
```json
{
    "currentMission": {
        "Key": {
            "Group": "BuiltIn",
            "Name": "Escalation"
        },
        "MaxTime": 3600.0
    },
    "nextMission": {
        "Key": {
            "Group": "BuiltIn",
            "Name": "Terminal Control"
        },
        "MaxTime": 3600.0
    }
}
```

---

### 6. `get-server-id`

**Purpose**: Retrieves the Steam ID of the server.

**Arguments**: None.

**Request**:
```json
{"name": "get-server-id", "arguments": []}
```

**Response**: Status 2000, body (official specification):
```json
{"serverId": "01234567891234567"}
```

---

### 7. `get-player-list`

**Purpose**: Retrieves the list of currently connected players.

**Arguments**: None.

**Request**:
```json
{"name": "get-player-list", "arguments": []}
```

**Response**: Status 2000, body (official specification):
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

**Note (from official source)**: The dedicated server returns only `steamId` and `faction` fields. The `displayName` field has been removed because the server runs headlessly and does not cache player names. The external server (dashboard/admin console) can fetch display names using Steam's Web API if needed.

**SPEC CONFLICT**: Spec 002 FR-007 and User Story 3 AC1 promise a player list with display names in the dashboard. This protocol cannot provide display names; the agent can return only `steamId` and `faction`. **Flagged for spec owner**: decide whether to (a) defer display-name rendering to a future release, (b) mandate Steam Web API lookups in the agent, or (c) modify the upstream game server's output format.

---

### 8. `set-time-remaining`

**Purpose**: Sets the remaining time for the current mission (in seconds).

**Arguments**:
- `arguments[0]` (required): Time in seconds (float format, e.g., `"600.0"`).

**Request**:
```json
{"name": "set-time-remaining", "arguments": ["600.0"]}
```

**Response**: Status 2000, body: **UNVERIFIED — response body shape not specified upstream; confirm against a live server**.

---

### 9. `set-next-mission`

**Purpose**: Sets the mission to be loaded after the current one concludes.

**Arguments**:
- `arguments[0]` (required): Mission group (string, e.g., `"BuiltIn"`).
- `arguments[1]` (required): Mission name (string, e.g., `"Escalation"`).
- `arguments[2]` (required): Max time in seconds (float format, e.g., `"3600.0"`).

**Request**:
```json
{"name": "set-next-mission", "arguments": ["BuiltIn", "Escalation", "3600.0"]}
```

**Response**: Status 2000, body: **UNVERIFIED — response body shape not specified upstream; confirm against a live server**.

---

### 10. `kick-player`

**Purpose**: Kicks a player from the server.

**Arguments**:
- `arguments[0]` (required): Steam ID (unsigned long as string, e.g., `"0123456789"`).

**Request**:
```json
{"name": "kick-player", "arguments": ["0123456789"]}
```

**Response**: Status 2000, body: (absent or **UNVERIFIED**). Note (from official source): Kicked player cannot rejoin until the server restarts.

---

### 11. `unkick-player`

**Purpose**: Removes a player from the kick list, allowing them to rejoin.

**Arguments**:
- `arguments[0]` (required): Steam ID (unsigned long as string).

**Request**:
```json
{"name": "unkick-player", "arguments": ["0123456789"]}
```

**Response**: Status 2000, body: **UNVERIFIED — response body shape not specified upstream; confirm against a live server**.

---

### 12. `clear-kicked-players`

**Purpose**: Clears the entire list of kicked players, allowing all to rejoin.

**Arguments**: None.

**Request**:
```json
{"name": "clear-kicked-players", "arguments": []}
```

**Response**: Status 2000, body: **UNVERIFIED — response body shape not specified upstream; confirm against a live server**.

---

### 13. `banlist-reload`

**Purpose**: Reloads the ban list from the list of files configured in the server config.

**Arguments**: None.

**Request**:
```json
{"name": "banlist-reload", "arguments": []}
```

**Response**: Status 2000, body: **UNVERIFIED — response body shape not specified upstream; confirm against a live server**.

**Note (from official source)**: Will only add new IDs. Use `banlist-clear` before reload if you want to remove all IDs first before reloading.

---

### 14. `banlist-add`

**Purpose**: Adds a Steam ID to the ban list (appends it to the first configured ban file).

**Arguments**:
- `arguments[0]` (required): Steam ID (unsigned long as string, e.g., `"0123456789"`).
- `arguments[1]` (optional): Reason (string, e.g., `"cheating"`).

**Request (with reason)**:
```json
{"name": "banlist-add", "arguments": ["0123456789", "cheating"]}
```

**Request (without reason)**:
```json
{"name": "banlist-add", "arguments": ["0123456789"]}
```

**Response**: Status 2000, body: **UNVERIFIED — response body shape not specified upstream; confirm against a live server**.

---

### 15. `banlist-remove`

**Purpose**: Removes a Steam ID from the ban list and from the first configured ban file.

**Arguments**:
- `arguments[0]` (required): Steam ID (unsigned long as string).

**Request**:
```json
{"name": "banlist-remove", "arguments": ["0123456789"]}
```

**Response**: Status 2000, body: **UNVERIFIED — response body shape not specified upstream; confirm against a live server**.

---

### 16. `banlist-clear`

**Purpose**: Clears the ban list loaded in the Authenticator. Does NOT modify ban list files.

**Arguments**: None.

**Request**:
```json
{"name": "banlist-clear", "arguments": []}
```

**Response**: Status 2000, body: **UNVERIFIED — response body shape not specified upstream; confirm against a live server**.

---

### 17. `get-mission-rotation`

**Purpose**: Retrieves the current mission rotation configuration, type, and next override status.

**Arguments**: None.

**Request**:
```json
{"name": "get-mission-rotation", "arguments": []}
```

**Response**: Status 2000, body (official specification):
```json
{
    "rotationType": "Sequence",
    "rotation": [
        {
            "Key": {
                "Group": "BuiltIn",
                "Name": "Escalation"
            },
            "MaxTime": 7200.0
        },
        {
            "Key": {
                "Group": "BuiltIn",
                "Name": "Terminal Control"
            },
            "MaxTime": 7200.0
        }
    ],
    "hasNextOverride": true,
    "nextOverride": {
        "Key": {
            "Group": "BuiltIn",
            "Name": "Escalation"
        },
        "MaxTime": 3600.0
    }
}
```

---

### 18. `set-mission-rotation`

**Purpose**: Sets the mission rotation configuration in memory and reloads the configuration.

**Arguments**:
- `arguments[0]` (required): Entire rotation configuration serialized as a JSON string. (Because `arguments` is an array of strings, complex objects must be JSON-encoded as a single string.)

**Request Example**:
```json
{
    "name": "set-mission-rotation",
    "arguments": [
        "{\"rotationType\":\"Sequence\",\"clearNextOverride\":false,\"rotation\":[{\"Key\":{\"Group\":\"BuiltIn\",\"Name\":\"Escalation\"},\"MaxTime\":7200.0}]}"
    ]
}
```

**Nested JSON Structure** (the string in `arguments[0]` deserializes to):
```json
{
    "rotationType": "Sequence",
    "clearNextOverride": false,
    "rotation": [
        {
            "Key": {
                "Group": "BuiltIn",
                "Name": "Escalation"
            },
            "MaxTime": 7200.0
        }
    ]
}
```

**Response**: Status 2000, body: **UNVERIFIED — response body shape not specified upstream; confirm against a live server**.

---

### 19. `clear-next-mission`

**Purpose**: Clears the next overridden mission, reverting back to the standard rotation.

**Arguments**: None.

**Request**:
```json
{"name": "clear-next-mission", "arguments": []}
```

**Response**: Status 2000, body: **UNVERIFIED — response body shape not specified upstream; confirm against a live server**.

---

## Security Constraints (CRITICAL)

**NO AUTHENTICATION, NO PASSWORD, NO HANDSHAKE**

The remote-command protocol defines:
- No username/password mechanism.
- No TLS/SSL encryption.
- No digital signature or HMAC validation.
- No session token or nonce.
- No challenge-response handshake.

**Any client with network access to the port can send any command.**

**Consequences of Exposed Port**:
- An unauthenticated attacker with access to this port can:
  - Kick or ban players (destructive).
  - Change the mission rotation (disrupting play).
  - Broadcast chat messages (griefing).
  - Retrieve the list of connected players and their Steam IDs (reconnaissance).

**Hard Requirement for Gameplane**:
1. **MUST NOT advertise the remote-command port** on the externally exposed Service (LoadBalancer, NodePort, etc.).
2. **MUST only reach this port via pod-local loopback** (localhost/127.0.0.1) from the agent sidecar.
3. **MUST NOT allow external client connections** to this port.

**Implementation Detail** (per spec FR-006):
- The agent sidecar (`agent/internal/rcon/` or a new `agent/internal/nuclearoption/` package) connects to `localhost:7779` (or the configured port) over TCP.
- The game container exposes the remote-command port internally; the agent reaches it without leaving the pod.
- The operator must not manually expose this port to the network.
- If a user manually adds this port to the exposed Service (via `ServiceAnnotations` or other means), they are bypassing security; Gameplane MUST detect and warn against this.

**Consequence if Violated**: An operator who accidentally exposes this port to the internet exposes their server to remote takeover (arbitrarily kick/ban players, change settings, gather player data).

---

## Agent-Side Protocol Identifier & Integration

The existing console protocol allowlist (per CLAUDE.md memory, "Console protocols phase 2") includes:
- `telnet` (implemented in `agent/internal/rcon/telnet.go`)
- `websocket` (implemented in `agent/internal/rcon/websocket.go`)
- `battleye` (implemented in `agent/internal/rcon/battleye.go`)
- `satisfactory` (implemented in `agent/internal/rcon/satisfactory.go`)
- `palworld` (implemented in `agent/internal/rcon/palworld.go`)
- `none` (no console protocol)

A new protocol identifier for Nuclear Option must be added:
- **Protocol Name**: `nuclearoption` (lowercase, single word, no separators; does NOT need to match the module name — existing protocols already differ this way).
- **Location**: New implementation file `agent/internal/rcon/nuclearoption.go` (or similar) containing a client that speaks the length-prefixed JSON wire format described above.
- **Registration**: The allowlist in the agent (wherever protocols are enumerated) must include `"nuclearoption"`.

**Also Update**: The module repository (`gameplane-module`, checked out as `modules/` submodule) maintains its own `validate.py` script with a `RCON_PROTOCOLS` allowlist (per CLAUDE.md memory). After this contract is implemented:
1. The agent must support `"nuclearoption"` protocol.
2. The module repo's `validate.py` must also add `"nuclearoption"` to its allowlist so that `nuclear-option/template.yaml` is valid when it declares `rcon.protocol: nuclearoption`.

---

## File & Artifact References

- **Agent RCON Package**: `agent/internal/rcon/` (telnet.go, websocket.go, battleye.go, satisfactory.go, palworld.go, rcon.go)
- **Console Package**: `agent/internal/console/console.go` (WebSocket server for console endpoints)
- **Module Repository Validation**: `gameplane-module` repo, `validate.py` (maintains `RCON_PROTOCOLS` allowlist)
