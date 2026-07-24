// Package gameproto holds wire-protocol parsing for game server handshakes:
// Minecraft (Java Edition) and Terraria. Both are used by the operator's idle
// auto-sleep feature to classify an inbound connection on first read and decide
// whether to wake a dormant server:
//
//   - Join: a player is attempting to connect (wake the server).
//   - Status: a server-list ping or query (answer it without waking).
//   - Unknown: unrecognized bytes (do not wake; let the connection fail).
//
// Each classifier reads from an io.Reader (safe to feed a live net.Conn) and
// returns the Kind plus parsed context (protocol version, handshake fields)
// that the caller needs to craft a reply. All parsing is defensive: bounded
// reads, explicit max packet sizes, no unbounded allocations from length
// prefixes, and no panics on hostile input.
package gameproto

import (
	"fmt"
	"io"
)

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
type MinecraftClassifyResult struct {
	// ProtocolVersion is the version field from the handshake packet.
	ProtocolVersion int32

	// NextState is the requested next protocol state after handshake.
	// 1 = Status, 2 = Login.
	NextState int32

	// ServerAddr is the server address (host:port) the client sent in the handshake.
	ServerAddr string
}

// ClassifyMinecraft reads a Minecraft handshake from the connection and
// classifies it as Join (next state = 2 / login) or Status (next state = 1 /
// status ping), or Unknown if the bytes don't parse. It reads at most one
// complete handshake packet and returns any error encountered (including EOF
// on truncated input). Hostile input like a huge length prefix is rejected
// before allocation.
func ClassifyMinecraft(r io.Reader) (Kind, *MinecraftClassifyResult, error) {
	return classifyMinecraftHandshake(r)
}

// TerrariaClassifyResult holds the result of classifying a Terraria connection.
type TerrariaClassifyResult struct {
	// Version is the protocol version string from the ConnectRequest.
	Version string

	// MaxPlayers is the max player count if parseable from the request.
	// Not all Terraria clients send it; zero if unavailable.
	MaxPlayers byte
}

// ClassifyTerraria reads a Terraria ConnectRequest and classifies it as Join.
// Terraria has no out-of-band status ping, only player connections, so any
// recognized ConnectRequest is classified as Join. Unknown messages or
// truncated input return Unknown.
func ClassifyTerraria(r io.Reader) (Kind, *TerrariaClassifyResult, error) {
	return classifyTerrariaConnect(r)
}

// BuildMinecraftStatusResponse builds a JSON status response packet that
// the server can send without waking. It takes the JSON (e.g. a server list
// entry payload) and returns the framed packet bytes ready to write.
func BuildMinecraftStatusResponse(jsonPayload string) ([]byte, error) {
	return buildMinecraftStatusResponse(jsonPayload)
}

// BuildMinecraftLoginDisconnect builds a Login Disconnect packet containing
// a chat-JSON reason string.
func BuildMinecraftLoginDisconnect(reason string) ([]byte, error) {
	return buildMinecraftLoginDisconnect(reason)
}

// BuildTerrariaDisconnect builds a Terraria Disconnect message containing
// a reason string.
func BuildTerrariaDisconnect(reason string) ([]byte, error) {
	return buildTerrariaDisconnect(reason)
}
