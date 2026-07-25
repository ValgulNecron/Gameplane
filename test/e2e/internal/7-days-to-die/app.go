package main

import (
	"context"
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
		return probeSevenDaysToDay(ctx, flags.Addr)
	})
}

// probeSevenDaysToDay attempts to measure join depth on a 7 Days to Die server.
//
// 7 Days to Die (Unity engine) declares multiple ports:
//   - TCP 26900: primary game port
//   - UDP 26900: game port (redundant, same number as TCP)
//   - UDP 26901: secondary protocol port
//   - UDP 26902: tertiary protocol port
//   - TCP 8081: telnet console (no password wiring possible in this template)
//
// node-gamedig's "sdtd" game definition documents A2S (Valve/Source Engine
// Query) support at `port: 26900, port_query_offset: 1` — the query port is
// the game port PLUS ONE (26901/UDP), not the game port itself. Its "sdtd"
// protocol implementation is literally `class sdtd extends Valve`: the same
// A2S_INFO/A2S_PLAYER wire format as every other Source-family game, with an
// optional telnet enrichment step layered on top that this probe does not use
// (the telnet password is not wired in this template).
// Reference: https://github.com/gamedig/node-gamedig
// (lib/games.js "sdtd" entry; protocols/sdtd.js).
//
// An earlier version of this probe sent A2S_INFO to the game port (26900)
// itself — the wrong port per the above, which would never succeed against a
// real server. It also raced its own TCP fallback out of any retry budget:
// probe.Retry consumes the ENTIRE remaining deadline retrying A2S before
// returning an error, so by the time control reached the TCP fallback the
// shared context was already expired and the fallback's own probe.Retry call
// returned immediately without ever dialing. This version queries 26901 for
// A2S and gives the TCP fallback a real, separately-budgeted chance to run.
//
// This probe:
//  1. Attempts A2S_INFO on the query port (26901/UDP), using up to half of
//     whatever deadline remains.
//  2. If A2S never succeeds, falls back to a plain TCP dial on the declared
//     primary game port (26900/TCP), using the rest of the deadline — a real
//     TCP accept is a falsifiable signal a live server actually grants, even
//     without understanding 7DtD's join protocol.
//  3. Without joining the game (credentials unknown), QUERY is the maximum
//     measurable depth either way.
func probeSevenDaysToDay(ctx context.Context, addr string) (probe.Depth, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("parse addr: %w", err)
	}
	queryAddr := net.JoinHostPort(host, "26901")

	// Give A2S up to half of whatever deadline remains, so the TCP fallback
	// below is guaranteed a genuine share of the budget rather than starting
	// with an already-expired context. If ctx somehow carries no deadline
	// (probe.Main always sets one in practice), fall back to a fixed budget
	// so this phase still can't run forever.
	a2sBudget := 90 * time.Second
	if dl, ok := ctx.Deadline(); ok {
		if remaining := time.Until(dl); remaining > 0 {
			a2sBudget = remaining / 2
		}
	}
	a2sCtx, cancel := context.WithTimeout(ctx, a2sBudget)
	var info *a2s.Info
	a2sErr := probe.Retry(a2sCtx, "a2s-query", 15*time.Second, func(actx context.Context) error {
		var qerr error
		info, qerr = a2s.QueryInfo(actx, queryAddr)
		return qerr
	})
	cancel()

	if a2sErr == nil {
		log.Printf("a2s: server=%q map=%q players=%d/%d", info.Name, info.Map, info.Players, info.MaxPlayers)
		return probe.Query, nil
	}
	log.Printf("a2s-query: never succeeded on %s (%v); falling back to tcp connectivity on %s", queryAddr, a2sErr, addr)

	// A2S never succeeded. Fall back to plain TCP connectivity on the
	// declared primary game port, using whatever budget remains on the
	// ORIGINAL ctx (not the expired a2sCtx sub-budget) — a real TCP accept is
	// a falsifiable signal a live server actually grants.
	tcpErr := probe.Retry(ctx, "tcp-connectivity", 15*time.Second, func(actx context.Context) error {
		d := net.Dialer{}
		conn, derr := d.DialContext(actx, "tcp", addr)
		if derr != nil {
			return fmt.Errorf("tcp dial: %w", derr)
		}
		defer conn.Close()
		log.Printf("tcp-connectivity: connection established to %s", addr)
		return nil
	})
	if tcpErr == nil {
		// TCP connectivity succeeded, but A2S did not: the server is
		// listening but did not answer A2S_INFO within this run. Without
		// official protocol documentation for anything deeper, or
		// credentials for a real join, QUERY depth is all this establishes.
		log.Printf("tcp-connectivity: server is listening, but A2S query on %s was not answered", queryAddr)
		return probe.Query, nil
	}

	return "", fmt.Errorf("7dtd server not reachable (a2s query on %s failed: %v; tcp dial to %s failed: %w)",
		queryAddr, a2sErr, addr, tcpErr)
}
