package joindepth

import (
	"fmt"
	"strings"
)

// JoinDepth represents the depth a probe reached in a game's join handshake.
//
// QUERY: Out-of-band status query (A2S_INFO, RCON dial, bare socket dial) proving reachability only.
// PARTIAL: Server accepted the client handshake intent but the exchange was deliberately not completed.
//   Example: sentinel wake-on-connect tests assert this on an armed server.
// JOINED: Client completed the real protocol login/join handshake and observed a server-originated post-join artifact
//   (e.g., Minecraft Login Success packet, Terraria WorldData packet).
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
