// Package gameproto holds wire-protocol parsing for game server handshakes:
// Minecraft (Java Edition) and Terraria. Both are used by the operator's idle
// auto-sleep feature to classify an inbound connection on first read and decide
// whether to wake a dormant server:
//
//   - Join: a player is attempting to connect (wake the server).
//   - Status: a server-list ping or query (answer it without waking).
//   - Unknown: unrecognized bytes (do not wake; let the connection fail).
//
// Primary API: The Classifier interface and registry pattern (gameproto/classifier.go
// and gameproto/registry.go) provide the canonical API for protocol-agnostic handshake
// classification, response building, and status-ping capability detection. This pattern
// enables new protocols to be added without modifying shared code (gameproto/gameproto.go
// or sentinel/main.go).
//
// Deprecated facades: Functions ClassifyMinecraft, ClassifyTerraria, BuildMinecraftStatusResponse,
// BuildMinecraftLoginDisconnect, and BuildTerrariaDisconnect, along with types
// MinecraftClassifyResult and TerrariaClassifyResult, are deprecated in favor of the
// Classifier interface. They are retained as compatibility wrappers to support equivalence
// tests that validate byte-for-byte behavior preservation during the transition from
// facades to the Classifier registry pattern.
//
// Replay contract: the caller must create a *bufio.Reader and pass it to the
// classifier. The classifier reads the handshake and returns it in the Consumed
// field. Any pipelined data (e.g., Handshake followed immediately by Login Start)
// remains in the *bufio.Reader's internal buffer. The caller then owns that buffer
// and must continue reading from the same *bufio.Reader for the rest of the
// connection. The complete original stream is exactly: Consumed + everything
// remaining in the caller's *bufio.Reader.
//
// All parsing is defensive: bounded reads, explicit max packet sizes, no unbounded
// allocations from length prefixes, and no panics on hostile input.
//
// Example usage (deprecated — use Classifier registry instead):
//
//	br := bufio.NewReader(conn)
//	kind, result, err := ClassifyMinecraft(br)
//	if err != nil {
//		return err
//	}
//	// Replay the handshake and forward the rest of the connection:
//	_, err = upstream.Write(result.Consumed)
//	_, err = io.Copy(upstream, br)  // br still has pipelined bytes
//
// The Consumed field holds the actual bytes read: length-prefix VarInt + frame
// data for Minecraft, or 3-byte header + payload for Terraria.
package gameproto

import (
	"bufio"
	"errors"
	"fmt"
)

// ErrStatusPingUnsupported is returned when a protocol does not support
// out-of-band status pings and BuildStatusResponse is called on it.
var ErrStatusPingUnsupported = errors.New("status ping unsupported for this protocol")

// Kind classifies the reason for a connection attempt.
type Kind int

const (
	// Join means a player is attempting to connect to the server.
	// The server should wake if dormant.
	Join Kind = iota

	// Status means the client is pinging the server for status information
	// (server list entry). The server should answer without waking.
	Status

	// Unknown means the bytes did not parse as either join or status.
	// The connection should be left to fail naturally.
	Unknown
)

func (k Kind) String() string {
	switch k {
	case Join:
		return "Join"
	case Status:
		return "Status"
	case Unknown:
		return "Unknown"
	default:
		return fmt.Sprintf("Kind(%d)", k)
	}
}

// MinecraftClassifyResult holds the result of classifying a Minecraft handshake.
//
// Deprecated: Use Classifier interface and MinecraftDetail struct instead.
// This type is retained for equivalence tests validating behavior preservation.
type MinecraftClassifyResult struct {
	// ProtocolVersion is the version field from the handshake packet.
	ProtocolVersion int32

	// NextState is the requested next protocol state after handshake.
	// 1 = Status, 2 = Login.
	NextState int32

	// ServerAddr is the server address (host:port) the client sent in the handshake.
	ServerAddr string

	// Consumed holds the bytes read from the input to parse this handshake.
	// Can be replayed: len(consumed) + len(remaining) == len(original).
	Consumed []byte
}

// ClassifyMinecraft reads a Minecraft handshake from the connection and
// classifies it as Join (next state = 2 / login) or Status (next state = 1 /
// status ping), or Unknown if the bytes don't parse. It reads at most one
// complete handshake packet and returns any error encountered (including EOF
// on truncated input). Hostile input like a huge length prefix is rejected
// before allocation. The caller must provide a *bufio.Reader and continue
// using it for the rest of the connection to avoid losing pipelined data.
//
// Deprecated: Use registry.Lookup("minecraft") to obtain a MinecraftClassifier,
// then call its Classify method instead. This function is retained for
// equivalence tests validating behavior preservation.
func ClassifyMinecraft(br *bufio.Reader) (Kind, *MinecraftClassifyResult, error) {
	return classifyMinecraftHandshake(br)
}

// TerrariaClassifyResult holds the result of classifying a Terraria connection.
//
// Deprecated: Use Classifier interface and TerrariaDetail struct instead.
// This type is retained for equivalence tests validating behavior preservation.
type TerrariaClassifyResult struct {
	// Version is the protocol version string from the ConnectRequest.
	Version string

	// Consumed holds the bytes read from the input to parse this request.
	// Can be replayed: len(consumed) + len(remaining) == len(original).
	Consumed []byte
}

// ClassifyTerraria reads a Terraria ConnectRequest and classifies it as Join.
// Terraria has no out-of-band status ping, only player connections, so any
// recognized ConnectRequest is classified as Join. Unknown messages or
// truncated input return Unknown. The caller must provide a *bufio.Reader and
// continue using it for the rest of the connection to avoid losing pipelined data.
//
// Deprecated: Use registry.Lookup("terraria") to obtain a TerrariaClassifier,
// then call its Classify method instead. This function is retained for
// equivalence tests validating behavior preservation.
func ClassifyTerraria(br *bufio.Reader) (Kind, *TerrariaClassifyResult, error) {
	return classifyTerrariaConnect(br)
}

// BuildMinecraftStatusResponse builds a JSON status response packet that
// the server can send without waking. It takes the JSON (e.g. a server list
// entry payload) and returns the framed packet bytes ready to write.
//
// Deprecated: Use registry.Lookup("minecraft") to obtain a MinecraftClassifier,
// then call its BuildStatusResponse method instead. This function is retained
// for equivalence tests validating behavior preservation.
func BuildMinecraftStatusResponse(jsonPayload string) ([]byte, error) {
	return buildMinecraftStatusResponse(jsonPayload)
}

// BuildMinecraftLoginDisconnect builds a Login Disconnect packet containing
// a chat-JSON reason string.
//
// Deprecated: Use registry.Lookup("minecraft") to obtain a MinecraftClassifier,
// then call its BuildDisconnect method instead. This function is retained
// for equivalence tests validating behavior preservation.
func BuildMinecraftLoginDisconnect(reason string) ([]byte, error) {
	return buildMinecraftLoginDisconnect(reason)
}

// BuildTerrariaDisconnect builds a Terraria Disconnect message containing
// a reason string.
//
// Deprecated: Use registry.Lookup("terraria") to obtain a TerrariaClassifier,
// then call its BuildDisconnect method instead. This function is retained
// for equivalence tests validating behavior preservation.
func BuildTerrariaDisconnect(reason string) ([]byte, error) {
	return buildTerrariaDisconnect(reason)
}
