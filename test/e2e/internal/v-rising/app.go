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
		return probeVRising(ctx, flags.Addr)
	})
}

// probeVRising attempts to reach the deepest join depth on a V Rising server.
//
// V Rising is a Steamworks GameServer title: like the other four games in
// this bucket, its Steamworks GameServer integration answers Valve's A2S
// (Source Engine Query) protocol on a dedicated query port — game port + 1 by
// Stunlock's own default configuration (9876 game / 9877 query, matching this
// template). node-gamedig's "vrising" game definition confirms A2S support:
// `protocol: 'valve', port_query_offset: [1, 15]` — i.e. try query-port
// candidates at game-port+1 first, which is exactly the port this template
// exposes.
// Reference: https://github.com/gamedig/node-gamedig (lib/games.js,
// "vrising" entry).
//
// An earlier version of this probe treated A2S as a mere "diagnostic" and, if
// it failed, fell back to a hand-rolled raw UDP probe that also granted QUERY
// depth on any reply. That fallback was not falsifiable (a server that only
// answers well-formed requests would silently drop an unrecognized packet —
// the same failure mode proven empirically against Factorio's query port),
// and in practice it was also dead code: probe.Retry consumes the entire
// remaining deadline retrying A2S before returning, so by the time control
// reached the fallback the shared context was already expired and the
// fallback's own probe.Retry call returned immediately without ever dialing.
// A2S is in fact the genuine, documented protocol here (see above), so it is
// now the sole basis for QUERY depth.
//
// Depth measurement: Returns Query if the query port answers A2S_INFO. Returns
// a fatal error otherwise.
func probeVRising(ctx context.Context, addr string) (probe.Depth, error) {
	var info *a2s.Info
	err := probe.Retry(ctx, "a2s-info", 15*time.Second, func(actx context.Context) error {
		var qerr error
		info, qerr = a2s.QueryInfo(actx, addr)
		return qerr
	})
	if err != nil {
		// A2S never succeeded before the deadline. Send one bounded,
		// best-effort diagnostic probe purely for evidence — it cannot
		// change the outcome below. It runs with its own short, independent
		// timeout (the shared probe deadline is expired by this point).
		// Silence is itself a measurement: future readers need to know a raw
		// probe was attempted and what (if anything) came back.
		diagCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		logRawDiagnostic(diagCtx, addr)
		return "", fmt.Errorf("a2s query on query port %s never succeeded: %w", addr, err)
	}

	log.Printf("a2s query succeeded: server=%q map=%q players=%d/%d",
		info.Name, info.Map, info.Players, info.MaxPlayers)
	return probe.Query, nil
}

// logRawDiagnostic sends a minimal, non-protocol UDP probe and logs whatever
// comes back, or explicitly logs that nothing did. It is purely diagnostic:
// its outcome is never used to determine probe success or failure.
func logRawDiagnostic(ctx context.Context, addr string) {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "udp", addr)
	if err != nil {
		log.Printf("raw-diagnostic: dial %s failed: %v", addr, err)
		return
	}
	defer conn.Close()

	deadline := time.Now().Add(2 * time.Second)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetDeadline(deadline)

	if _, err := conn.Write([]byte{0x00}); err != nil {
		log.Printf("raw-diagnostic: write to %s failed: %v", addr, err)
		return
	}

	buf := make([]byte, 1500)
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("raw-diagnostic: no response from %s within the diagnostic window (%v)", addr, err)
		return
	}

	hexResp := hex.EncodeToString(buf[:n])
	if len(hexResp) > 512 {
		hexResp = hexResp[:512] + "... (truncated)"
	}
	log.Printf("raw-diagnostic: %s replied to a non-protocol probe, len=%d hex=%s", addr, n, hexResp)
}
