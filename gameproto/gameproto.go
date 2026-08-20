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
// Example usage:
//
//	br := bufio.NewReader(conn)
//	classifier := registry.Lookup("minecraft")
//	result, err := classifier.Classify(br)
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
