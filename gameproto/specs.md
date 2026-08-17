# gameproto — Specification

**Status:** beta (v0.2.0-beta.8)  
**Module / package:** `github.com/ValgulNecron/gameplane/gameproto`  
**Dependencies:** stdlib only (Go 1.25+)

## Purpose

Shared wire-protocol parser for game server handshakes (Minecraft Java Edition and Terraria) that classifies inbound connections to determine whether they are join attempts (player connecting) or status queries (server-list ping). Used by the sentinel wake-on-connect daemon to distinguish a real player from a server-list probe without corrupting the connection stream, so the sentinel can answer status pings immediately without waking the server while holding join attempts open until the real game pod comes up.

## Responsibilities

1. Parse Minecraft Java Edition handshake frames (VarInt-framed packets) and classify them by next-state (Status vs. Login).
2. Parse Terraria connection frames and identify ConnectRequest messages as join attempts.
3. Preserve consumed bytes for replay to the real game pod, via a `Consumed` field that tracks exactly what was read, enabling lossless hand-off once the upstream connection is established.
4. Distinguish a genuine join from a server-list status ping or other probe without consuming more of the connection than necessary.
5. Defend against hostile input: bounded reads, explicit max packet sizes, no unbounded allocations from length prefixes, no panics.
6. Build protocol-native responses (Minecraft status replies, disconnect messages) for the sentinel to send without waking the server.

## Non-goals / boundaries

- Does **not** execute joins — only parses handshakes and classifies them. Execution (gameplay, RCON, etc.) is outside the scope.
- Does **not** validate player credentials or enforceRBAC — classification is protocol-only, not business logic.
- Does **not** persist state or maintain connections — parsing and response building are pure, stateless functions.
- Does **not** interact with Kubernetes objects directly — the sentinel and its callers own the operator integration.
- Does **not** support join-attempt forwarding or address rewriting — replies and consumed bytes are protocol-native, unmodified.

## Directory & package layout

```
gameproto/
├── gameproto.go        # Public API (Kind, classifiers, response builders)
├── minecraft.go        # Minecraft handshake codec (VarInt frame + packet parser + response builders)
├── terraria.go         # Terraria message codec (7-bit-encoded strings, NetworkText frames)
├── minecraft_test.go   # Minecraft codec tests
├── terraria_test.go    # Terraria codec tests
├── gameproto_test.go   # Public API integration tests
├── go.mod             # (module, stdlib-only)
└── .testcoverage.yml  # 90% coverage gate
```

Single package; no subdirectories.

## External interface / contracts

### Classification

**`Kind`** — Enum classifying a connection attempt:

```go
type Kind int

const (
  Join    Kind = iota  // Player is attempting to connect; server should wake
  Status               // Server-list ping or query; server should answer without waking
  Unknown              // Bytes did not parse as either join or status
)
```

### Classifiers

**`ClassifyMinecraft(br *bufio.Reader) (Kind, *MinecraftClassifyResult, error)`**

Parses the Handshake packet from a Minecraft Java Edition connection and classifies it.

- `br`: A `*bufio.Reader` wrapping the inbound connection. The reader is used directly, so pipelined data (Handshake followed by Login Start, etc.) remains in the reader's buffer for the caller to continue reading.
- **Returns:** `Kind` (Join for next-state=2/Login, Status for next-state=1/Status, Unknown otherwise), a result struct with parsed fields (protocol version, server address, port, consumed bytes), and any parse error.
- **Error handling:** Returns `Unknown` and no error for truncated input (EOF mid-packet). Returns `Unknown` and a wrapped error for malformed frames (invalid VarInts, length prefix out of range, missing required fields). Does **not** panic on hostile input.

**MinecraftClassifyResult:**
```go
type MinecraftClassifyResult struct {
  ProtocolVersion int32   // Version field from handshake
  NextState       int32   // 1 = Status, 2 = Login (determines Kind)
  ServerAddr      string  // "hostname:port" sent by client
  Consumed        []byte  // Exact bytes read to parse this handshake (VarInt frame + packet data)
}
```

**`ClassifyTerraria(br *bufio.Reader) (Kind, *TerrariaClassifyResult, error)`**

Parses the first Terraria message frame and classifies it. Terraria has no out-of-band status ping (all recognized messages are player-driven), so only ConnectRequest (message type 1) classifies as Join; anything else or unrecognized data returns Unknown.

- `br`: A `*bufio.Reader` wrapping the inbound connection, used directly to preserve pipelined bytes.
- **Returns:** `Kind` (Join only for ConnectRequest; Unknown otherwise), a result struct with the version string and consumed bytes, and any parse error.
- **Error handling:** Returns `Unknown` and no error for truncated input or unrecognized message types. Returns `Unknown` and a wrapped error for parsing failures.

**TerrariaClassifyResult:**
```go
type TerrariaClassifyResult struct {
  Version  string // Protocol version string from ConnectRequest
  Consumed []byte // Exact bytes read: 2-byte LE length + 1-byte type + payload
}
```

### Replay Contract

**The `Consumed` field and why it exists:**

The sentinel must classify a connection without consuming more bytes than necessary, so it can replay them to the real game pod once it comes up. Both classifiers accept a `*bufio.Reader` and read from it directly. Any bytes they consume are returned in the `Consumed` field; anything left in the reader's internal buffer (pipelined data) stays there for the caller to consume.

**Replay sequence:**
1. Call `ClassifyMinecraft` or `ClassifyTerraria` with a fresh `*bufio.Reader` wrapping the client connection.
2. Receive the classified `Kind` and the `Consumed` bytes.
3. Once the upstream (game pod) connection is ready, write `Consumed` to it first.
4. Then continue copying from the same `*bufio.Reader` (for pipelined bytes) and the client connection (for new data).

This contract ensures: `Consumed + remaining(bufio.Reader) == original stream`, so the game pod receives exactly what the client sent, in order, with no data loss or duplication.

### Response Builders

**`BuildMinecraftStatusResponse(jsonPayload string) ([]byte, error)`**

Builds a Minecraft Status Response packet (sent in response to a Status ping without waking the server).

- Input: A JSON object (e.g., `{"version":{"name":"Asleep","protocol":0},"players":{"max":0,"online":0},"description":{"text":"Server is asleep"}}`).
- Returns: The framed packet bytes ready to write to the client.
- **Error handling:** Returns an error if the JSON payload is too long (string > 32KB, a Minecraft protocol limit).

**`BuildMinecraftLoginDisconnect(reason string) ([]byte, error)`**

Builds a Minecraft Login Disconnect packet (sent to reject or bounce a joining player).

- Input: A plain-text reason string (e.g., "Server is waking up, try again in a moment.").
- Returns: The framed packet bytes ready to write.
- **Error handling:** Escapes the reason for JSON safety (quotes, backslashes, control chars). Returns an error if the escaped JSON exceeds protocol limits.

**`BuildTerrariaDisconnect(reason string) ([]byte, error)`**

Builds a Terraria Disconnect message (sent to reject or bounce a connecting player).

- Input: A plain-text reason string.
- Returns: The framed message bytes (3-byte header + NetworkText-encoded reason) ready to write.
- **Error handling:** Returns an error if the message exceeds the 16-bit length limit.

## Handshake Codecs

### Minecraft Java Edition

**Handshake packet structure:**
```
[VarInt] packet_length
  [VarInt] packet_id (0x00 for Handshake)
  [VarInt] protocol_version
  [String] server_address (VarInt length + UTF-8 bytes)
  [UInt16 BE] server_port
  [VarInt] next_state (1 = Status, 2 = Login)
```

**Parsing:** Read the frame length (VarInt, 5 bytes max), reject if > 512 (minecraftMaxPacketSize), read the frame data, then parse packet fields inside. Strings are bounded at 32KB (Minecraft protocol limit). The packet ID must be 0x00; any other value returns Unknown.

**Classification:** next_state == 2 → Join; next_state == 1 → Status; anything else → Unknown.

### Terraria

**Message frame structure:**
```
[UInt16 LE] total_length (includes the 2-byte length field itself)
[UInt8] message_type
[payload...]
```

**ConnectRequest payload:**
```
[7-bit-encoded-int] version_string_length
[UTF-8] version_string
```

**Parsing:** Read 3-byte header (2-byte length LE + 1-byte type), validate length (>= 3, <= 65535), read payload (length - 3), then parse the message type. Only ConnectRequest (type 1) indicates a join; all other types or parse errors return Unknown.

**Classification:** message_type == 1 → Join; otherwise → Unknown.

### VarInt and 7-bit-encoded-int Codecs

**Minecraft VarInt (signed 32-bit, big-endian continuation bits):**
- 1–5 bytes; high bit (0x80) set = continuation, clear = final byte.
- Each byte contributes 7 bits; result is cast from uint32 to int32.
- Rejects malformed sequences: overflow (6+ bytes) or values that set bits beyond bit 31.

**Terraria 7-bit-encoded-int (signed 32-bit, little-endian continuation bits):**
- Similar principle but different bit layout (MSB is continuation, LSBs are payload).
- 1–5 bytes; rejects overflow (6+ bytes).

Both defend against untrusted length prefixes: they bound the decoder loop to 5 iterations and reject any continuation bit on the 5th byte (signaling a 6th byte is needed).

## Kind Classification Invariants

1. **Only JOINED depth satisfies a covered-* status.** Classification alone (without joining) is PARTIAL; only test cases that establish a game connection to verify handshake replay satisfy JOINED depth.
2. **Status and Unknown are distinct.** Status means the sentinel can and should answer with a protocol-native reply (Minecraft status JSON). Unknown means the bytes did not parse; the sentinel closes without replying or waking.
3. **Protocol-native only.** The classifiers operate on wire bytes only; no game state, player database, or server configuration is consulted. Join vs. Status is determined entirely by the handshake.
4. **Classification is deterministic.** Given the same wire bytes, ClassifyMinecraft and ClassifyTerraria always return the same Kind (barring bugs).
5. **Parsing is read-safe.** The frame length limits, bounded VarInt loops, and max-packet-size gates ensure parsing terminates and doesn't allocate based on untrusted input.

## Bounded-Read Defences

1. **Frame length limits:**
   - Minecraft: 512 bytes max (handshakes are much smaller; this prevents unbounded allocation from a huge length prefix).
   - Terraria: 65535 bytes max (16-bit length field limit; reasonable for a single message).

2. **String length bounds:**
   - Minecraft: 32KB max (Minecraft protocol limit for string payloads).
   - Terraria: 32KB max (derived from the 7-bit-encoded int max for a single message).

3. **VarInt decoder loops:**
   - Both Minecraft and Terraria VarInt decoders loop exactly 5 times. If the 5th byte has the continuation bit set, the decoder rejects the input (overflow), rather than reading a 6th byte.

4. **io.ReadFull** (not io.Read):
   - The classifiers use `io.ReadFull` to read frame data and payloads. This ensures either the full count is read or an error is returned; partial reads are treated as errors (truncated input).

5. **Metadata hostname detection:**
   - Not present in gameproto itself, but the caller (sentinel) can use response builders to craft safe replies. The builders escape JSON (Minecraft) and validate payload sizes (Terraria).

## Invariants the Sentinel Depends On

1. **Consumed bytes are correct and complete.** The sentinel replays them to the game pod as-is; if they're truncated, duplicated, or corrupted, the join fails.

2. **The bufio.Reader is not consumed beyond the handshake.** The sentinel re-uses the same reader to forward pipelined data (e.g., Login Start after Handshake) and later game protocol traffic. If ClassifyMinecraft or ClassifyTerraria reads beyond the consumed frame, pipelined bytes are lost.

3. **Status replies are protocol-native and don't wake the server.** BuildMinecraftStatusResponse returns bytes the client expects; sending them doesn't trigger any game-logic side effects.

4. **Disconnect messages bounce the client cleanly.** BuildMinecraftLoginDisconnect and BuildTerrariaDisconnect are protocol-native rejection messages; the client receives them, displays the reason, and closes the connection.

5. **Unknown classification doesn't assume anything.** If ClassifyMinecraft returns Unknown (e.g., unknown next-state), the sentinel closes without waking or replying. The client is left to time out naturally.

## Testing & Coverage

**Test structure:**

- **`minecraft_test.go`:** Table-driven tests for VarInt codec, string parsing, handshake classification (various protocol versions, next-states, edge cases), response building (JSON escaping, large payloads), error cases (truncated input, oversized frames).
- **`terraria_test.go`:** Similar coverage for 7-bit-encoded-int, Terraria message framing, ConnectRequest parsing, various message types (Join, Unknown, etc.), Disconnect message building.
- **`gameproto_test.go`:** Integration tests verifying classifiers work end-to-end, including replay contract (Consumed bytes can be re-read without duplication).

**Key test scenarios:**

- **Replay contract verification:** Build a handshake, classify it, replay the Consumed bytes to a mock upstream, verify the game pod receives the exact bytes.
- **Pipelined data preservation:** Classify a handshake with additional Login Start bytes following; verify the bufio.Reader still has those bytes available.
- **Bounds enforcement:** Oversized frame lengths, oversized strings, malformed VarInts, truncated input — all return errors or Unknown without panicking.
- **Response builders:** Status JSON escaping, chat-component encoding, Terraria NetworkText framing.

**Coverage gate:** 90% (enforced by `.testcoverage.yml`). The small gap is in error paths (e.g., malformed VarInts that exceed bounds) and less-critical response-builder edge cases.

## Non-goals (What Gameproto Does NOT Do)

- **Does not execute joins.** Classification only; the sentinel decides whether to wake or reply.
- **Does not validate server addresses or ports.** The ServerAddr field in MinecraftClassifyResult is the raw string the client sent; the sentinel doesn't validate it, only replays it.
- **Does not interact with DNS or network sockets.** Parsing is pure; callers provide a bufio.Reader wrapping a real connection.
- **Does not implement the full game protocol.** Handshake parsing only; full Login, Status, play, etc. phases are the game's job, not gameproto's.
- **Does not authenticate players.** The classified Kind is protocol-based, not player-database-based.
- **Does not provide telemetry or logging.** Callers (sentinel, tests) own logging classified Kind and any parse errors.

## Dependencies

**Stdlib only:**
- `bufio` — buffered readers for connection streams.
- `bytes` — byte buffers for packet assembly and frame capture.
- `encoding/binary` — big-endian (Minecraft) and little-endian (Terraria) integer encoding.
- `errors` — error wrapping and identity.
- `fmt` — error formatting.
- `io` — io.Reader, io.ReadFull, io.ByteReader interfaces.
- `strconv` — integer parsing (port numbers).
- `strings` — string operations (handshake address parsing).

No external modules.

## Security Considerations

1. **Denial of Service (unbounded allocation):** The frame length limits (512 and 65535) and VarInt loop bounds (5 iterations) prevent a malicious client from causing unbounded memory allocation. The sentinel can safely call the classifiers on untrusted inbound connections.

2. **Stream corruption (pipelined bytes):** The Consumed field tracks exactly what was read, enabling lossless replay. If the sentinel forwards the bufio.Reader's remaining bytes without re-reading Consumed, the game pod receives the correct stream in order.

3. **Status spoofing (information disclosure):** BuildMinecraftStatusResponse returns exactly what the sentinel provides (no added metadata). The sentinel is responsible for populating the JSON with safe values (no version strings, cluster names, online player counts, etc., per CLAUDE.md login privacy rule).

4. **Response injection (protocol-native escaping):** BuildMinecraftLoginDisconnect escapes the reason string for JSON safety (quotes, control chars). The sentinel relies on this; if it bypassed the builder, it could send malformed packets.

## References

- **`sentinel/main.go`** — usage: `ClassifyMinecraft`, `ClassifyTerraria`, response builders for hold-and-forward logic.
- **`sentinel/main_test.go`** — test doubles: `encodeMinecraftHandshake`, `encodeTerrariaConnectRequest` show how to construct wire bytes for testing.
- **`test/e2e/tests/bot_*.go`** — e2e tests that launch real game bots to verify handshake replay doesn't corrupt joins.
- **`go.work`** — workspace linking gameproto to sentinel, operator, api, and other Go modules.
- **`docs/architecture.md`** — overview of sentinel's wake-on-connect feature and gameproto's role.
