package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/probe"
	a2s "github.com/ValgulNecron/gameplane/test/e2e/internal/protocol/a2sproto"
)

func main() {
	flags := probe.ParseFlags()
	probe.Main(flags, func(ctx context.Context) (probe.Depth, error) {
		return probeRust(ctx, flags.Addr)
	})
}

// probeRust attempts to reach the deepest join depth on a Rust server.
// Rust's game protocol is RakNet-derived and not publicly documented;
// real joins require Steam identity which CI cannot mint.
// This probe measures QUERY depth via A2S query (if available).
//
// Path B strategy:
//   - Rust servers MAY answer A2S_INFO queries on the game port (28015).
//     This is not guaranteed and depends on server configuration, but many
//     Rust servers do respond to A2S queries for compatibility with query tools.
//   - If A2S succeeds, we establish QUERY depth and log server metadata.
//   - If A2S fails, the server is not responding to standard query protocols,
//     and QUERY depth cannot be established via Path B alone.
//
// We do NOT use WebSocket RCON (port 28016) for depth measurement because:
//   - The RCON password is only known to the agent (it's injected as an env var).
//   - Dialing RCON from the probe would require secretless auth, which is not
//     how CI works — the probe binary is read-only and receives no secrets.
//   - The agent (Path A) will test RCON independently as part of the heartbeat.
//   - Using RCON here would blur the distinction between Path A (Gameplane) and
//     Path B (independent server protocol).
func probeRust(ctx context.Context, addr string) (probe.Depth, error) {
	// Attempt A2S query on the game port.
	// Rust servers may or may not respond to A2S; if they don't, this is not
	// a failure of the server, just a limitation of CI's query depth.
	var info *a2s.Info
	var a2sErr error
	if err := probe.Retry(ctx, "a2s-info", 15*time.Second, func(actx context.Context) error {
		var err error
		info, err = a2s.QueryInfo(actx, addr)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		// A2S query failed. This could mean:
		// 1. The server is not yet ready (boot in progress).
		// 2. The server doesn't answer A2S queries (some Rust configs disable it).
		// 3. Network path is blocked (unlikely in-cluster, but possible).
		// Log the error for diagnostics, but don't fail the probe yet.
		a2sErr = err
		log.Printf("a2s-info failed (non-fatal; Rust may not answer A2S): %v", err)
	}

	// If A2S succeeded, we have QUERY depth.
	if info != nil {
		log.Printf("a2s: server=%q map=%q players=%d/%d version=%q", info.Name, info.Map, info.Players, info.MaxPlayers, info.Version)
		return probe.Query, nil
	}

	// A2S failed. Since Rust's game protocol is undocumented and requires Steam auth,
	// we cannot measure JOINED or PARTIAL depth via Path B.
	// The best we can do is report the A2S failure.
	if a2sErr != nil {
		return "", fmt.Errorf("a2s query failed and no fallback available: %w", a2sErr)
	}

	// This should not be reached, but just in case...
	return "", fmt.Errorf("rust probe: no query method succeeded")
}
