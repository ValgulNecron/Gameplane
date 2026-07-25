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
		return probeDontStarveTogether(ctx, flags.Addr)
	})
}

// probeDontStarveTogether attempts to reach the deepest join depth on a Don't
// Starve Together server.
//
// DST is a Steamworks GameServer title (Steam AppID 322330): like the other
// four games in this bucket, its Steamworks GameServer integration answers
// Valve's A2S (Source Engine Query) protocol on a dedicated Steam query port,
// independent of the game's own ENet-based join protocol on the game port.
// node-gamedig's "dst" game definition confirms this: `port: 10999,
// port_query: 27016, protocol: 'valve'` — plain A2S, no per-game protocol
// override. The community-documented cluster.ini [STEAM] section corroborates
// the port number: a DST master shard's `master_server_port` defaults to
// 27016.
// References:
//   - https://github.com/gamedig/node-gamedig (lib/games.js, "dst" entry)
//   - https://dontstarve.wiki.gg/wiki/Guides/Don%E2%80%99t_Starve_Together_Dedicated_Servers
//     (cluster.ini [STEAM] master_server_port)
//
// An earlier version of this probe sent a single invented byte (0x00) to the
// game port (10999) and required any reply. That is not falsifiable: DST's
// game port speaks ENet, a reliable-UDP layer that does not answer arbitrary
// unrecognized datagrams — the same failure mode proven empirically against
// Factorio's query port (it silently drops unrecognized packets, so a probe
// requiring a response to a made-up packet fails permanently against a
// healthy server). A2S_INFO on the Steam query port is the actual documented
// request a real server answers, so it is used here instead.
//
// CAVEAT (unverified): unlike the other four games in this bucket, port 27016
// is not mentioned anywhere in the jamesits/dst-server-docker image's own
// documentation (which calls out only 10999/11000 for players to open), and
// this probe has never been run against a real server — DST is in the heavy
// set (opt-in only, never runs in CI; see spec.md "Measured connectivity").
// If a real run shows the port never responds — e.g. because this image's
// entrypoint never initializes the Steamworks GameServer object without a
// Steam cluster token, which CI does not set — this assertion needs to be
// revisited. See spec.md for what to do if that happens.
//
// Depth measurement: Returns Query if the Steam query port answers A2S_INFO.
// Returns a fatal error otherwise, since DST declares no TCP port at all to
// fall back to.
func probeDontStarveTogether(ctx context.Context, addr string) (probe.Depth, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("parse addr: %w", err)
	}

	// Steam query port is 27016 (per node-gamedig's port_query and DST's
	// documented cluster.ini [STEAM] master_server_port default).
	queryAddr := net.JoinHostPort(host, "27016")

	var info *a2s.Info
	err = probe.Retry(ctx, "a2s-info", 15*time.Second, func(actx context.Context) error {
		var qerr error
		info, qerr = a2s.QueryInfo(actx, queryAddr)
		return qerr
	})
	if err == nil {
		log.Printf("a2s query succeeded: server=%q map=%q players=%d/%d",
			info.Name, info.Map, info.Players, info.MaxPlayers)
		return probe.Query, nil
	}

	// A2S never succeeded before the deadline. Send one bounded, best-effort
	// diagnostic probe at the game port purely for evidence — it cannot
	// change the outcome below. Silence is itself a measurement: future
	// readers need to know a raw probe was attempted and what (if anything)
	// came back.
	diagCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	logRawDiagnostic(diagCtx, net.JoinHostPort(host, "10999"))

	return "", fmt.Errorf("a2s query on steam query port %s never succeeded: %w", queryAddr, err)
}

// logRawDiagnostic sends a minimal, non-protocol UDP probe and logs whatever
// comes back, or explicitly logs that nothing did. It is purely diagnostic:
// its outcome is never used to determine probe success or failure, and it
// runs with its own short, independent timeout rather than the (by this
// point, likely expired) probe deadline.
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

	buf := make([]byte, 4096)
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
