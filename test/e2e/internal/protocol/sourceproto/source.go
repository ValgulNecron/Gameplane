// Package source implements the Source engine connectionless protocol for
// player-join probing. This supports both Source 1 (Half-Life 2, Garry's Mod,
// GoldSrc) and Source 2 (CS:GO post-2018, CS2).
//
// The Source engine uses a connectionless UDP handshake (independent of the
// in-game protocol layer) to attempt a connection. This package implements:
//
//   - A2S_GETCHALLENGE ('q'): query for a server challenge number, required
//     before attempting to join. CONFIRMED against real Garry's Mod server
//     (PR #197, e2e game bot job, 2026-07-24).
//   - C2S_CONNECT ('k'): attempt to connect with the challenge, protocol
//     version, and player name. Server parsing CONFIRMED; protocol version 17
//     was rejected as outdated (PR #197, 2026-07-24); attempting version 18 next.
//     Packet layout remains unverified (server rejected at version check before
//     validating the full payload).
//
// Reference: https://developer.valvesoftware.com/wiki/Server_queries
// (primary reference for packet types and formats). See spec.md for measured
// evidence and verification status.
//
// # Steam Auth vs. LAN Mode
//
// By default, Source servers enforce Steam authentication: the server replies
// to C2S_CONNECT with either acceptance or a rejection message (disconnect
// reason). The sv_lan 1 CVAR disables this enforcement, allowing connections
// without valid Steam credentials.
//
// The empirical test is: send C2S_CONNECT and observe the reply.
// On a sv_lan 1 server: acceptance (0xFFFFFFFF + 0x03 for new client) or
// specific rejection (e.g., "server is full").
// On a sv_lan 0 / enforced-auth server: rejection with a message like
// "You must have a Steam account to play on this server" or "Invalid Steam ticket".
//
// This implementation sends a connect attempt without a Steam ticket and
// inspects the reply to determine the gate.
package sourceproto

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// Packet types (Valve Source protocol, the 4-byte 0xFFFFFFFF header is implicit).
const (
	pktChallengeReq  byte = 'q' // A2S_GETCHALLENGE
	pktChallengeResp byte = 'A' // S2C_CHALLENGE
	pktConnectReq    byte = 'k' // C2S_CONNECT

	// pktRejectVersion is observed on protocol version mismatch. Measured against
	// a real Garry's Mod server (PR #197, e2e game bot job, 2026-07-24) rejecting
	// with "game#GameUI_ServerRejectOldVersion\0" when the protocol version did not match.
	pktRejectVersion byte = 0x39 // '9' (confirmed by measurement)

	// pktDisconnect and pktAccept are unconfirmed guesses based on Valve's documentation.
	// A real server has not yet been observed sending these packet types.
	// Do not rely on these constants until they are confirmed by measurement.
	pktDisconnect byte = 'c'  // Disconnect (server rejection with reason text) — unconfirmed
	pktAccept     byte = 0x03 // Server accepted; new client handshake — unconfirmed
)

// Connectionless packet header (0xFFFFFFFF in network byte order).
const connlessHeader = 0xFFFFFFFF

// Protocol version constants.
// ProtocolSource1 = 18 is the current best candidate pending the diagnostic sweep.
// It is based on Source 2013 engine documentation and ceifa/garrysmod:debian's explicit
// rejection of protocol 17 with "GameUI_ServerRejectOldVersion" (PR #197, 2026-07-24).
// The diagnostic sweep in the Garry's Mod probe tests multiple candidates (24, 18, 17)
// in a single CI boot to converge on the correct version. Do not treat this constant
// as measured; it is the hypothesis the sweep will validate or refute.
//
// ProtocolSource2 = 18 is unconfirmed (never measured against a real CS2 server).
const (
	ProtocolSource1 uint32 = 18
	ProtocolSource2 uint32 = 18
)

// ConnectResult reports the outcome of a C2S_CONNECT attempt.
type ConnectResult struct {
	// Accepted indicates the server accepted the connection attempt.
	// If false, RejectMsg contains the server's reason (e.g., "server is full",
	// "Steam ticket expired", etc.).
	Accepted bool

	// RejectMsg contains the server's rejection reason, if Accepted is false.
	// Empty if the server did not provide one or the connection succeeded.
	RejectMsg string

	// Raw is the raw response packet from the server (including the
	// 0xFFFFFFFF header and packet type byte). Useful for diagnosis
	// if the response is unexpected.
	Raw []byte
}

// Challenge performs A2S_GETCHALLENGE and returns the server's challenge number.
// The challenge is required in the subsequent C2S_CONNECT attempt.
//
// Packets:
//   - Sent: 0xFFFFFFFF + 'q' + 0x00000000 (4 null bytes, per Valve documentation)
//   - Expected reply: 0xFFFFFFFF + 'A' + challenge (4 bytes LE)
//
// Returns an error if the dial fails, the response doesn't match the expected
// format, or the context deadline is exceeded.
func Challenge(ctx context.Context, addr string) (uint32, error) {
	conn, err := dialContext(ctx, addr)
	if err != nil {
		return 0, fmt.Errorf("source: dial: %w", err)
	}
	defer conn.Close()

	// Send A2S_GETCHALLENGE.
	req := make([]byte, 9)
	binary.LittleEndian.PutUint32(req[0:4], connlessHeader)
	req[4] = pktChallengeReq
	// The payload is 4 null bytes (0x00000000); Valve's spec suggests this is
	// often omitted or optional, but it's safest to include it.
	binary.LittleEndian.PutUint32(req[5:9], 0)

	if _, err := conn.Write(req); err != nil {
		return 0, fmt.Errorf("source: send challenge request: %w", err)
	}

	// Read response: expect 0xFFFFFFFF + 'A' + 4-byte challenge.
	resp := make([]byte, 9)
	n, err := conn.Read(resp)
	if err != nil {
		return 0, fmt.Errorf("source: read challenge response: %w", err)
	}
	if n < 9 {
		return 0, fmt.Errorf("source: challenge response too short: got %d bytes, expected 9", n)
	}

	if binary.LittleEndian.Uint32(resp[0:4]) != connlessHeader {
		return 0, fmt.Errorf("source: invalid response header: got 0x%08x, expected 0x%08x",
			binary.LittleEndian.Uint32(resp[0:4]), connlessHeader)
	}
	if resp[4] != pktChallengeResp {
		return 0, fmt.Errorf("source: unexpected response packet type: got 0x%02x (%c), expected 0x%02x (%c)",
			resp[4], resp[4], pktChallengeResp, pktChallengeResp)
	}

	challenge := binary.LittleEndian.Uint32(resp[5:9])
	return challenge, nil
}

// Connect sends C2S_CONNECT with the supplied challenge and player name,
// attempting to join the server. Returns the server's response.
//
// The connection attempt includes:
//   - Challenge number (from A2S_GETCHALLENGE)
//   - Protocol version (e.g., 17 for Source 1 / Garry's Mod, 18 for Source 2 / CS2)
//   - Auth protocol (0 for no extra auth, other values for encryption/compression)
//   - Player name (null-terminated UTF-8 string)
//   - Cvars string (additional parameters; often empty)
//
// On success, the server replies with 0x03 (new client). On rejection, it
// replies with 0x63 (disconnect) plus a reason string.
//
// Per Valve's source code (engine/net_ws.cpp), the exact packet layout for
// C2S_CONNECT varies slightly between Source 1 and Source 2, but the core
// (challenge + protocol version + player name) is stable.
//
// Returns the server's raw response in ConnectResult.Raw for diagnosis.
// If the server accepts the connection, ConnectResult.Accepted is true.
// If the server rejects it, ConnectResult.Accepted is false and RejectMsg
// contains the server's reason (if provided).
func Connect(ctx context.Context, addr string, challenge uint32, name string, protocol uint32) (*ConnectResult, error) {
	conn, err := dialContext(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("source: dial: %w", err)
	}
	defer conn.Close()

	// Build C2S_CONNECT packet.
	// Layout (per Valve's net_ws.cpp):
	//   0xFFFFFFFF (4 bytes, LE)
	//   'k' (1 byte)
	//   challenge (4 bytes, LE)
	//   protocol version (4 bytes, LE)
	//   auth protocol (4 bytes, LE) - 0 for LAN mode, no Steam
	//   player name (null-terminated string)
	//   cvars string (usually empty, but may contain encryption flags, etc.)
	//
	// For a simple LAN mode join attempt, auth protocol = 0 and cvars = empty.
	authProto := uint32(0) // LAN mode: no Steam auth
	cvars := ""            // No extra parameters

	req := make([]byte, 0, 64)
	req = append(req, 0xFF, 0xFF, 0xFF, 0xFF) // header
	req = append(req, pktConnectReq)          // 'k'

	// challenge
	chalBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(chalBytes, challenge)
	req = append(req, chalBytes...)

	// protocol version
	protBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(protBytes, protocol)
	req = append(req, protBytes...)

	// auth protocol
	authBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(authBytes, authProto)
	req = append(req, authBytes...)

	// player name (null-terminated)
	req = append(req, []byte(name)...)
	req = append(req, 0x00)

	// cvars (null-terminated; empty string = just null terminator)
	req = append(req, []byte(cvars)...)
	req = append(req, 0x00)

	if _, err := conn.Write(req); err != nil {
		return nil, fmt.Errorf("source: send connect request: %w", err)
	}

	// Read the server's response.
	// It can be:
	//   - 0xFFFFFFFF + 0x03 (new client accepted)
	//   - 0xFFFFFFFF + 0x63 (disconnect with reason)
	//   - Or other packet types in rare cases.
	resp := make([]byte, 1024)
	n, err := conn.Read(resp)
	resp = resp[:n]

	result := &ConnectResult{
		Raw: resp,
	}

	// Preserve the response bytes even on read error (e.g., timeout).
	// This allows the caller to diagnose what happened.
	if err != nil && !errors.Is(err, io.EOF) {
		return result, fmt.Errorf("source: read connect response: %w", err)
	}

	if n < 5 {
		// Truncated; server might have sent too little, or connection dropped.
		return result, nil
	}

	// Check header and packet type.
	if binary.LittleEndian.Uint32(resp[0:4]) != connlessHeader {
		// Not a valid connectionless packet; might be an in-game protocol packet
		// or garbage. Treat as neither accepted nor properly rejected.
		return result, nil
	}

	pktType := resp[4]
	switch pktType {
	case pktAccept:
		// 0x03: server accepted; new client handshake begins (unconfirmed).
		result.Accepted = true
		result.RejectMsg = ""
	case pktDisconnect, pktRejectVersion:
		// Rejection: either 0x63 (unconfirmed) or 0x39 (measured).
		// Both carry a null-terminated UTF-8 reason string at resp[5:].
		result.Accepted = false
		if n > 5 {
			// Find the null terminator.
			msgBytes := resp[5:]
			endIdx := len(msgBytes)
			for i, b := range msgBytes {
				if b == 0x00 {
					endIdx = i
					break
				}
			}
			result.RejectMsg = string(msgBytes[:endIdx])
		}
	default:
		// Unexpected packet type; could be a pre-login protocol packet or noise.
		// Treat as neither accepted nor rejected — inconclusive.
	}

	return result, nil
}

// dialContext is a helper to dial with the context deadline.
func dialContext(ctx context.Context, addr string) (net.Conn, error) {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "udp", addr)
	if err != nil {
		return nil, err
	}

	// Set a read deadline so we don't hang waiting for a response.
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	} else {
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	}

	return conn, nil
}
