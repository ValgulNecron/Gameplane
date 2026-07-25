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
		return probeEnshrouded(ctx, flags.Addr)
	})
}

// probeEnshrouded attempts to reach the deepest join depth on an Enshrouded server.
// Enshrouded has no console, no RCON, and no remote control of any kind (the
// shipped template declares rcon.protocol: none and consoleMode: none), so the
// query port is the only measurement surface available at all.
//
// The query port (UDP 15637) speaks Valve's A2S (Source Engine Query) protocol.
// Enshrouded's developers do not publish this directly, but it is confirmed by
// node-gamedig — a widely used, actively maintained open-source game-server
// query library: its "enshrouded" game definition declares
// `port: 15636, port_query: 15637, protocol: 'valve'`, i.e. plain A2S on the
// query port, using the exact same generic Source-query implementation it uses
// for hundreds of Source-family and Source-adjacent (Steamworks GameServer)
// titles — no per-game protocol override exists for Enshrouded, unlike e.g.
// 7 Days to Die's telnet-enriched variant.
// Reference: https://github.com/gamedig/node-gamedig (lib/games.js,
// "enshrouded" entry; GAMES_LIST.md lists it under "Valve Protocol").
//
// An earlier version of this probe sent a single invented byte (0x01) to the
// query port and required any reply. That is not falsifiable: a real server
// that only answers well-formed A2S requests would silently drop an
// unrecognized packet — the same failure mode proven empirically against
// Factorio's query port (it drops unrecognized UDP datagrams with no reply at
// all, so a probe requiring a response to a made-up packet fails permanently
// against a perfectly healthy server). A2S_INFO is the actual documented
// request this port answers, so it is used here instead.
//
// Depth measurement: Returns Query if the query port answers A2S_INFO. Returns
// a fatal error if it never does before the deadline, since Enshrouded has no
// other control or query surface to fall back to.
func probeEnshrouded(ctx context.Context, addr string) (probe.Depth, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("parse addr: %w", err)
	}

	// Query port is 15637 (per the module template and node-gamedig's port_query).
	queryAddr := net.JoinHostPort(host, "15637")

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
	// diagnostic probe purely for evidence — it cannot change the outcome
	// below (which is already a failure). Silence is itself a measurement:
	// future readers need to know a raw probe was attempted and what (if
	// anything) came back, in case A2S support turns out to be wrong.
	diagCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	logRawDiagnostic(diagCtx, queryAddr)

	return "", fmt.Errorf("a2s query on query port %s never succeeded: %w", queryAddr, err)
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
