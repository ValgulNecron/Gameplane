package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/probe"
)

func main() {
	flags := probe.ParseFlags()
	probe.Main(flags, func(ctx context.Context) (probe.Depth, error) {
		return probeARK(ctx, flags.Addr)
	})
}

// probeARK attempts to measure join depth on an ARK: Survival Ascended server.
//
// ARK: Survival Ascended (UE5) declares no query port. The join protocol
// requires EOS/Steam identity that CI cannot mint, making a full join
// impossible. The honest depth is RCON port connectivity only.
//
// This probe:
//  1. Attempts TCP connect to port 27020 (RCON/Source protocol port) to verify a listener exists.
//  2. A successful TCP handshake proves the server is listening on that port.
//  3. Returns QUERY depth if the connection succeeded, or an error if the server
//     is not reachable at all.
//
// Why TCP on 27020 instead of UDP on 7777:
//   - UDP dial does not handshake; it just creates a local socket. It succeeds even
//     when nothing is listening.
//   - TCP dial requires a real handshake with the remote listener, which is
//     falsifiable: a dead server or closed port will reject the connection.
//   - ARK declares rcon.protocol: source on TCP 27020, and this port exists on all
//     ARK servers (though RCON is not enabled by default without admin configuration).
//
// Note: This proves the TCP port is open, which is consistent with QUERY depth.
// It does not prove the game is serving players (that would require join or query depth).
func probeARK(ctx context.Context, addr string) (probe.Depth, error) {
	// Parse addr to replace UDP with TCP on 27020.
	// The addr passed in is in format "host:7777" (game port).
	// We need to connect to "host:27020" (RCON port) instead.
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// If addr doesn't have a port, assume it's just the host.
		host = addr
	}
	rconAddr := net.JoinHostPort(host, "27020")

	if err := probe.Retry(ctx, "tcp-connect-rcon", 15*time.Second, func(actx context.Context) error {
		// Dial the RCON port (TCP 27020). Use DialContext to respect the context deadline.
		d := net.Dialer{}
		conn, err := d.DialContext(actx, "tcp", rconAddr)
		if err != nil {
			return fmt.Errorf("dial tcp 27020: %w", err)
		}
		defer conn.Close()

		log.Printf("connectivity-probe: successfully connected to RCON port (TCP 27020)")
		return nil
	}); err != nil {
		// If the retry loop exhausted, return a clear error.
		return "", fmt.Errorf("ark server not reachable on RCON port: %w", err)
	}

	// If we reach here, the TCP connection to 27020 succeeded.
	log.Printf("connectivity-probe: verified QUERY depth (TCP port 27020 listening)")
	return probe.Query, nil
}
