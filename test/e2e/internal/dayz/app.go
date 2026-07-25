package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/probe"
	a2s "github.com/ValgulNecron/gameplane/test/e2e/internal/protocol/a2sproto"
)

func main() {
	flags := probe.ParseFlags()
	probe.Main(flags, func(ctx context.Context) (probe.Depth, error) {
		return probeDayZ(ctx, flags.Addr)
	})
}

// probeDayZ attempts to reach the deepest join depth on a DayZ server.
// It queries A2S info on UDP 27015 (establishes QUERY depth).
// It attempts a diagnostic UDP probe on the game port 2302 to observe
// what the server responds with, but makes no depth claim from it
// (the game protocol is undocumented and BattlEye RCON is unreachable).
//
// Depth measurement: A2S success proves QUERY.
// Game port diagnostic is instrumentation only and does not change depth.
//
// Reference:
// - A2S Query: https://developer.valvesoftware.com/wiki/Server_queries
// - DayZ Enfusion/BattlEye protocol: not publicly documented.
// - BattlEye RCON: bound to 127.0.0.1 (unreachable from probe pod).
func probeDayZ(ctx context.Context, addr string) (probe.Depth, error) {
	// Extract the host from the address (removing :port if present).
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// If SplitHostPort fails, try using the addr as-is.
		host = addr
	}

	// Build the A2S query address on port 27015.
	// The caller provides addr as <service>.<namespace>.svc.cluster.local:27015
	// but we dial A2S on the same host with explicit port.
	a2sAddr := net.JoinHostPort(host, "27015")

	// First: Query A2S to verify the server is alive and responding to queries.
	// This establishes QUERY depth and gives us server metadata.
	// A2S failure is fatal — server is not responding to queries at all.
	var info *a2s.Info
	if err := probe.Retry(ctx, "a2s-info", 15*time.Second, func(actx context.Context) error {
		var err error
		info, err = a2s.QueryInfo(actx, a2sAddr)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		// A2S failed; server not even responding to queries. This is fatal.
		return "", fmt.Errorf("a2s query failed: %w", err)
	}

	// Log server metadata from A2S response.
	log.Printf("a2s: server=%q map=%q players=%d/%d", info.Name, info.Map, info.Players, info.MaxPlayers)

	// Depth is now QUERY because A2S succeeded.
	// The game port diagnostic below is purely observational and will not change the returned depth.

	// Second: Attempt a diagnostic UDP probe on the game port (2302).
	// The game protocol (Enfusion) is not publicly documented, so we cannot
	// speak it. We send a minimal probe to observe what the server responds with,
	// for evidence purposes only. If the server doesn't respond or responds with
	// something we cannot parse, we log it and move on — it does not affect depth.
	if err := probe.Retry(ctx, "game-port-diagnostic", 10*time.Second, func(actx context.Context) error {
		gameAddr := net.JoinHostPort(host, "2302")
		return probeGamePort(actx, gameAddr)
	}); err != nil {
		// Game port probe failed; log it but don't fail the overall probe.
		// A2S already proved QUERY depth.
		log.Printf("game-port-diagnostic: %v (non-fatal; A2S already established QUERY)", err)
	}

	// Return QUERY because that is what A2S proved.
	// The game port attempt was diagnostic; evidence is in the logs above.
	return probe.Query, nil
}

// probeGamePort sends a minimal UDP probe to the game port and logs the response.
// This is purely diagnostic; the Enfusion protocol is undocumented.
func probeGamePort(ctx context.Context, addr string) error {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Set a read deadline so we don't block forever.
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return err
	}

	// Send a minimal probe: a single zero byte to see if the server is listening.
	// This is not a real protocol message, just a diagnostic poke.
	if _, err := conn.Write([]byte{0x00}); err != nil {
		return err
	}

	// Try to read a response (any response is interesting).
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		// Timeout or connection refused is expected for undocumented protocols.
		// Log it as non-fatal.
		return fmt.Errorf("read from game port: %w", err)
	}

	// Log what we received, bound to prevent log flooding.
	hexResp := hex.EncodeToString(buf[:n])
	if len(hexResp) > 256 {
		hexResp = hexResp[:256] + "... (truncated)"
	}
	log.Printf("game-port-diagnostic: server responded with %d bytes, hex=%s", n, hexResp)
	return nil
}
