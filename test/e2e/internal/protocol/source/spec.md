# Source Engine Connectionless Protocol

**Status:** Beta (implemented, not yet CI-verified)  
**Module / package:** `test/e2e/internal/protocol/source`  
**Dependencies:** stdlib only (Go 1.25+)

## Purpose

Implement the Valve Source engine's connectionless UDP protocol (`0xFFFFFFFF` header) for headless player-join probing. This is distinct from the in-game protocol layer; it handles the initial handshake before a client is admitted to the game.

The implementation focuses on two critical packets:

- **A2S_GETCHALLENGE ('q')** — query the server for a challenge number.
- **C2S_CONNECT ('k')** — attempt to join using the challenge, player name, and protocol version.

This package's `Connect` path exists to **elicit and log a real server's response** so the packet format can be corrected from evidence. It must not be used to assert a join depth until a real CI run has confirmed its behavior against a live server.

## Responsibilities

1. Perform `A2S_GETCHALLENGE` to obtain a server challenge number.
2. Send `C2S_CONNECT` with the challenge, protocol version (17 for Source 1, 18+ for Source 2), player name, and LAN-mode auth settings.
3. Interpret the server's response as acceptance (0x03), rejection with a message (0x63 + reason string), or inconclusive (other packet types).
4. Measure join depth: JOINED if the server accepts the connect, PARTIAL if the server rejects with a specific message (e.g., Steam ticket gate), or QUERY if only the challenge query works.
5. Return raw response bytes on every outcome — accepted, rejected, or error — for diagnosis.

## Non-goals / boundaries

- Does not implement the full in-game protocol (entity updates, world state, player input).
- Does not handle stream-based protocols or fragmentation beyond single UDP packets.
- Does not attempt to generate or validate Steam authentication tokens; the test is whether the server accepts a connect *without* a token (LAN mode).
- No automatic retry on lost UDP packets; a single lost request/response times out (matching real player behavior).

## Directory & package layout

```
test/e2e/internal/protocol/source/
├── source.go          # Challenge() and Connect() functions
├── source_test.go     # Unit tests
└── spec.md            # (this file)
```

Single package; no subdirectories.

## External interface / contracts

### Functions

**`Challenge(ctx context.Context, addr string) (uint32, error)`**  
Sends A2S_GETCHALLENGE and returns the server's challenge number. The challenge is required in the subsequent C2S_CONNECT attempt.

**Packets:**
- Sent: 0xFFFFFFFF + 'q' (0x71) + 0x00000000 (4 null bytes)
- Expected reply: 0xFFFFFFFF + 'A' (0x41) + challenge (4 bytes LE)

Returns an error if the dial fails, the response doesn't match the expected format, or the context deadline is exceeded.

**`Connect(ctx context.Context, addr string, challenge uint32, name string, protocol uint32) (*ConnectResult, error)`**  
Sends C2S_CONNECT with the supplied challenge and player name, attempting to join the server. Returns the server's response.

**Packet layout (best-effort, unverified against live servers):**
```
Offset  Size   Field
0       4      0xFFFFFFFF (LE)
4       1      'k' (0x6B)
5       4      Challenge (LE uint32, from S2C_CHALLENGE)
9       4      Protocol version (LE uint32)
13      4      Authentication protocol (LE uint32; 0 for LAN mode)
17      var    Player name (null-terminated UTF-8 string)
17+N+1  var    Cvars string (null-terminated UTF-8; often empty)
```

**Protocol version:**
- `17` — Source 1 (Half-Life 2, Garry's Mod, GoldSrc)
- `18` — Source 2 (CS:GO post-2018, CS2)

Servers reject connect attempts with a version mismatch; a clear disconnect message is returned.

**Server response (best-effort, unverified):**

1. **Acceptance (0x03):**  
   Indicates the server accepted the connect and will proceed with the in-game protocol. The join depth is **JOINED**. Raw packet format: `0xFFFFFFFF + 0x03 + [in-game protocol data]`.

2. **Rejection / Disconnect (0x63):**  
   The server rejected the connect attempt. The reason string describes why (e.g., "server is full", "You must have a Steam account to play", etc.). Raw packet format: `0xFFFFFFFF + 0x63 + [null-terminated UTF-8 reason string]`. The join depth is **PARTIAL** if the reason indicates a credential gate (Steam, GSLT, password), or could indicate other errors (server misconfigured, maximum players, etc.).

3. **Other/Inconclusive:**  
   If the server sends a packet type we don't recognize (e.g., an in-game protocol packet out of order), we cannot determine acceptance vs. rejection. The join depth is inconclusive and the raw packet is returned for diagnosis.

### Types

**`ConnectResult` struct:**
```go
type ConnectResult struct {
    Accepted  bool     // Server accepted the connection attempt
    RejectMsg string   // Server's rejection reason (if Accepted is false)
    Raw       []byte   // Raw response packet for diagnosis (includes 0xFFFFFFFF header and packet type)
}
```

The `Raw` field is always populated on success or on read error (including timeouts), allowing the caller to diagnose unexpected responses.

### Constants

- **`ProtocolSource1`** = 17 (protocol version for Garry's Mod, Half-Life 2, GoldSrc)
- **`ProtocolSource2`** = 18 (protocol version for CS2, modern Source 2)

## Key invariants

1. **This package's C2S_CONNECT layout is unverified.** Real servers (CS2, Garry's Mod, Half-Life 2, etc.) may silently drop a malformed connect rather than reply with an error. No depth assertion may be built on the connect path until measured against a live server in CI.

2. **UDP is connectionless and lossy.** A single lost request or response causes a timeout. There is no automatic retry; the caller supplies the deadline context.

3. **Source 1 and Source 2 diverge.** GoldSrc, Half-Life 2 (Source 1), and CS2 (Source 2) use different engines with different historical formats. Protocol version 17 (Source 1) and 18+ (Source 2) are exchanged in C2S_CONNECT; a mismatch causes immediate rejection. Packet layout may differ between versions; this implementation provides generic functions and delegates game-specific details to callers.

4. **Wire format is little-endian (LE).** All multi-byte integers in the Source protocol are encoded little-endian (`0xFFFFFFFF` header, challenge uint32, protocol version uint32).

5. **Strings are null-terminated (C-string style).** No length prefix; end on the first 0x00 byte.

6. **LAN mode assumption is untested.** This implementation sends a connect attempt without a Steam ticket and inspects the reply to determine the auth gate. The assumption is that servers running with `sv_lan 1` do not require Steam authentication and will accept the connect. This is currently unverified—real CI runs will settle it.

## Dependencies

**Internal:** None.  
**External:** stdlib only (`context`, `encoding/binary`, `errors`, `fmt`, `io`, `net`, `time`). Go 1.25+.

## Security considerations

1. **No authentication.** Connection probes are unauthenticated and sent in plaintext. Do not embed secrets in packet payloads.

2. **DoS-safe.** The implementation respects context deadlines and does not loop indefinitely on malformed responses. Oversized or garbage datagrams are handled gracefully.

3. **No DNS rebinding risk (limited).** UDP queries provide limited surface for DNS rebinding compared to HTTP, but the dial is still to a potentially user-supplied address. Callers should validate addresses before calling if this is a concern (the probe harness runs inside the cluster).

## Testing & coverage

**Unit tests** (`source_test.go`) — untagged, run on every `make test-go`:

- **Challenge round-trip** (`TestChallenge_RoundTrip`) — send 'q', receive 'A' + challenge.
- **Challenge error handling** (`TestChallenge_BadHeader`, `TestChallenge_BadPacketType`, `TestChallenge_TruncatedResponse`) — bad/truncated responses.
- **Connect acceptance** (`TestConnect_Accepted`) — send 'k', receive 0x03. Raw bytes preserved.
- **Connect rejection** (`TestConnect_RejectedWithMessage`, `TestConnect_RejectedNoMessage`) — send 'k', receive 0x63 + reason. Raw bytes preserved.
- **Connect error handling** (`TestConnect_TruncatedResponse`, `TestConnect_GarbageResponse`) — truncated/garbage responses, no panic.
- **Payload integrity** (`TestConnect_PayloadIntegrity`) — verify challenge, protocol version, and player name are correctly encoded in outgoing packet.
- **Protocol version handling** (`TestConnect_ProtocolVersions`) — both protocol 17 and 18 supported.
- **Non-loopback peers** (`TestChallenge_NonLoopbackServer`, `TestConnect_NonLoopbackServer`) — verify client works against non-loopback peers on the same host. **Do NOT catch a loopback-only bind regression** (all local addresses are reachable from a loopback-bound socket; the real bug required an off-host, routed destination like a Kubernetes ClusterIP).

All tests use a fake UDP server and run in parallel with 2-second deadlines.

**Self-confirming tests:** Tests that use this package's own wire encoder to produce server responses and then verify with its own decoder (e.g., `TestConnect_Accepted`, `TestConnect_RejectedWithMessage`, `TestConnect_PayloadIntegrity`) prove internal consistency, not correctness against real servers. These were converted to use explicit literal byte slices annotated field-by-field to verify the actual wire format being sent and received.

**Regression guard (loopback-bind class of defects):** This package's unit tests cannot catch a loopback-only bind bug because all addresses on the same host are locally reachable; the bug requires an off-host routed destination (Kubernetes Service ClusterIP). This defect class is guarded by the e2e tier: the e2e game bot CI job dials a real Service ClusterIP and did catch this bug once in production before the fix.

## Next steps (CI verification)

1. **Run Garry's Mod probe against a LAN-mode server:**
   - If `sv_lan 1` server accepts the probe's C2S_CONNECT without a Steam ticket, depth is JOINED.
   - If the server rejects with a Steam-auth-related message, depth is PARTIAL and the LAN-mode assumption must be reconsidered (or the server is misconfigured).

2. **Run CS2 probe against a LAN-mode server:**
   - Same as Garry's Mod; protocol version 18 is used instead of 17.
   - If CS2's response format differs from Source 1, add a CS2-specific wrapper or update the packet layout documentation.

3. **Record the measured depth** in the game-specific probe specs (`internal/garrys-mod/spec.md` and `internal/cs2/spec.md`).

## References

- **Valve Developer Community - Server queries:** https://developer.valvesoftware.com/wiki/Server_queries (primary reference for A2S packet types and formats)
- **Valve Developer Community - Master Server Query Protocol:** https://developer.valvesoftware.com/wiki/Master_Server_Query_Protocol (upstream registration)
- **E2E harness spec:** `test/e2e/internal/specs.md`
- **Probe pattern:** `test/e2e/internal/<game>/app.go`
