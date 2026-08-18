// Package joindepth defines the JoinDepth scale (QUERY/PARTIAL/JOINED) and
// the ProbeVerdict wire contract that per-game e2e probes use to report how
// far a real client got against a running game server.
package joindepth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
)

// JoinDepth represents the depth a probe reached in a game's join handshake.
//
// QUERY: Out-of-band status query (A2S_INFO, RCON dial, bare socket dial) proving reachability only.
// PARTIAL: Server accepted the client handshake intent but the exchange was deliberately not completed.
// Example: sentinel wake-on-connect tests assert this on an armed server.
// JOINED: Client completed the real protocol login/join handshake and observed a server-originated post-join artifact
// (e.g., Minecraft Login Success packet, Terraria WorldData packet).
//
// Ordering is QUERY < PARTIAL < JOINED.
//
// ONLY JOINED constitutes join coverage (per FR-006 of the game-protocol E2E coverage spec).
// PARTIAL proves the server parsed a join handshake but does not count as a completed join.
// QUERY is out-of-band reachability only.
//
// Tests assert an exact expected depth, not a minimum. An unexpected upgrade to a higher depth
// is a test failure signal and indicates a correctness defect (e.g., an assertion of QUERY
// when the probe unexpectedly reached JOINED).
//
// Wire encoding: stable uppercase string names ("QUERY", "PARTIAL", "JOINED"), used across
// process boundaries in the probe CLI contract (probe-cli.md).
type JoinDepth int

const (
	// QUERY represents an out-of-band status query that proves reachability only.
	QUERY JoinDepth = iota
	// PARTIAL represents a server that accepted the client's handshake intent but the exchange
	// was deliberately not completed (e.g., sentinel wake-on-connect tests).
	PARTIAL
	// JOINED represents a completed protocol login/join handshake with server-originated evidence.
	JOINED
)

// String returns the uppercase string encoding of the JoinDepth.
func (jd JoinDepth) String() string {
	switch jd {
	case QUERY:
		return "QUERY"
	case PARTIAL:
		return "PARTIAL"
	case JOINED:
		return "JOINED"
	default:
		return "UNKNOWN"
	}
}

// Parse parses a string token into a JoinDepth, returning an error if the token is unrecognized.
func Parse(s string) (JoinDepth, error) {
	switch strings.ToUpper(s) {
	case "QUERY":
		return QUERY, nil
	case "PARTIAL":
		return PARTIAL, nil
	case "JOINED":
		return JOINED, nil
	case "UNKNOWN":
		return -1, nil
	default:
		return -1, fmt.Errorf("invalid join depth: %q; must be one of QUERY, PARTIAL, JOINED, or UNKNOWN", s)
	}
}

// Less returns true if jd < other, implementing a total order over the three depths.
// Ordering is QUERY < PARTIAL < JOINED.
func (jd JoinDepth) Less(other JoinDepth) bool {
	return jd < other
}

// LessOrEqual returns true if jd <= other.
func (jd JoinDepth) LessOrEqual(other JoinDepth) bool {
	return jd <= other
}

// Greater returns true if jd > other.
func (jd JoinDepth) Greater(other JoinDepth) bool {
	return jd > other
}

// GreaterOrEqual returns true if jd >= other.
func (jd JoinDepth) GreaterOrEqual(other JoinDepth) bool {
	return jd >= other
}

// IsTransportError reports whether err represents a failure to establish or maintain
// the connection at all, as opposed to a live server that rejected or did not complete
// the exchange.
//
// The wrong-depth-on-transport-failure bug hit five probes simultaneously because each
// classifier had to be independently correct and five were not. Centralising this function
// makes that bug class structurally impossible rather than repeatedly fixable.
//
// IsTransportError prefers structural checks over string matching where the standard library
// allows it (errors.Is against net.ErrClosed, os.ErrDeadlineExceeded, context.DeadlineExceeded,
// errors.As to *net.OpError, *net.DNSError, and syscall.ECONNREFUSED / ECONNRESET /
// EHOSTUNREACH / ENETUNREACH), falling back to keyword matching only for what those cannot catch.
func IsTransportError(err error) bool {
	if err == nil {
		return false
	}

	// Structural checks first: standard library error types.
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Check for *net.OpError (covers dial, read, write failures).
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	// Check for *net.DNSError (DNS resolution failures).
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	// Check for common syscall errors.
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	if errors.Is(err, syscall.EHOSTUNREACH) {
		return true
	}
	if errors.Is(err, syscall.ENETUNREACH) {
		return true
	}

	// Fall back to keyword matching for errors that don't fit the structural checks above.
	// This union covers all 16 probes' vocabularies.
	errMsg := err.Error()
	transportKeywords := []string{
		"connection refused",
		"Dial timeout",
		"timeout",
		"connection never established",
		"DNS failure",
		"No response",
		"connection reset",
		"no such host",
		"dial",
		"dial tcp",
		"dial udp",
		"EOF",
		"refused",
		"i/o timeout",
		"network is unreachable",
		"context deadline exceeded",
		"cannot assign requested address",
		"deadline exceeded",
		"name resolution failed",
		"temporary failure",
		"connection refused by peer",
		"address in use",
		"never succeeded before the deadline",
	}

	for _, keyword := range transportKeywords {
		if strings.Contains(errMsg, keyword) {
			return true
		}
	}

	return false
}
