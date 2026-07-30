// Package main implements the Project Zomboid join depth probe.
package main

import (
	"context"
	"encoding/binary"
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
		return probeProjectZomboid(ctx, flags.Addr)
	})
}

// probeProjectZomboid attempts to reach the deepest join depth on a Project
// Zomboid server.
//
// Project Zomboid is a Steamworks GameServer title: like the other four games
// in this bucket, its Steamworks GameServer integration answers Valve's A2S
// (Source Engine Query) protocol directly on the game port. node-gamedig's
// "projectzomboid" game definition confirms this: `port: 16261, protocol:
// 'valve'` — no separate query port and no per-game protocol override; A2S
// rides the same UDP port as the game.
// Reference: https://github.com/gamedig/node-gamedig (lib/games.js,
// "projectzomboid" entry).
//
// An earlier version of this probe sent a hand-rolled 4-byte zero packet to
// the game port and required any reply. That is not falsifiable: a server
// that only answers well-formed requests would silently drop an unrecognized
// packet — the same failure mode proven empirically against Factorio's query
// port (it drops unrecognized UDP datagrams with no reply at all, so a probe
// requiring a response to a made-up packet fails permanently against a
// perfectly healthy server). A2S_INFO is the actual documented request this
// port answers, so it is used here instead.
//
// Depth measurement: Returns Query if the game port answers A2S_INFO. Returns
// a fatal error otherwise.
func probeProjectZomboid(ctx context.Context, addr string) (probe.Depth, error) {
	var info *a2s.Info
	err := probe.Retry(ctx, "a2s-info", 15*time.Second, func(actx context.Context) error {
		var qerr error
		info, qerr = a2s.QueryInfo(actx, addr)
		return qerr
	})
	if err != nil {
		// A2S never succeeded before the deadline. Send one bounded,
		// best-effort diagnostic probe purely for evidence — it cannot
		// change the outcome below. Silence is itself a measurement: future
		// readers need to know a raw probe was attempted and what (if
		// anything) came back.
		diagCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		logRawDiagnostic(diagCtx, addr)
		return "", fmt.Errorf("a2s query on game port %s never succeeded: %w", addr, err)
	}

	log.Printf("a2s query succeeded: server=%q map=%q players=%d/%d",
		info.Name, info.Map, info.Players, info.MaxPlayers)
	return probe.Query, nil
}

// logRawDiagnostic sends a minimal, non-protocol UDP probe (the same 4-byte
// zero packet this probe used to require a reply to) and logs whatever comes
// back, or explicitly logs that nothing did. It is purely diagnostic: its
// outcome is never used to determine probe success or failure, and it runs
// with its own short, independent timeout rather than the (by this point,
// likely expired) probe deadline.
func logRawDiagnostic(ctx context.Context, addr string) {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "udp4", addr)
	if err != nil {
		log.Printf("raw-diagnostic: dial %s failed: %v", addr, err)
		return
	}
	defer func() { _ = conn.Close() }()

	deadline := time.Now().Add(2 * time.Second)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetDeadline(deadline)

	req := make([]byte, 4)
	binary.LittleEndian.PutUint32(req, 0x00000000)
	if _, err := conn.Write(req); err != nil {
		log.Printf("raw-diagnostic: write to %s failed: %v", addr, err)
		return
	}

	buf := make([]byte, 1024)
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
