package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/protocol/joindepth"
)

func main() {
	// Parse standard flags
	addr := flag.String("addr", "", "game server host:port (in-cluster Service DNS)")
	deadline := flag.Duration("deadline", 4*time.Minute,
		"overall deadline; the probe retries until the server is reachable or this elapses")
	expectDepth := flag.String("expect-depth", "QUERY",
		"expected join depth: QUERY, PARTIAL, or JOINED")
	expectFail := flag.Bool("expect-fail", false,
		"if set, probe must NOT reach -expect-depth; exits 0 on correct failure, non-zero if it somehow succeeded")
	flag.Parse()

	// Validate inputs
	if *addr == "" {
		emitVerdictLine("FAIL_INTERNAL_ERROR", "UNKNOWN", "Bad flag: -addr is required")
		os.Exit(1)
	}

	expectedDepth, err := joindepth.Parse(*expectDepth)
	if err != nil {
		emitVerdictLine("FAIL_INTERNAL_ERROR", "UNKNOWN",
			fmt.Sprintf("Bad flag: -expect-depth must be one of QUERY, PARTIAL, JOINED; got %q", *expectDepth))
		os.Exit(1)
	}

	log.SetFlags(log.Ltime)

	ctx, cancel := context.WithTimeout(context.Background(), *deadline)
	defer cancel()

	verdict := probeARK(ctx, *addr)

	// Compute exit code and result based on verdict, expectation, and -expect-fail
	var result joindepth.VerdictResult
	var exitCode int

	if *expectFail {
		// Under -expect-fail, the negative control passes if the probe fails to reach expected depth.
		if verdict.ReachedDepth == expectedDepth && verdict.Err == nil {
			// Probe reached the expected depth when it should not have: negative control failed.
			// Exit 2 (connected and reached depth) per the contract.
			exitCode = 2
			result = joindepth.FAIL_NEGATIVE_CONTROL_REACHED
		} else {
			// Probe correctly failed to reach the depth: negative control passed.
			exitCode = 0
			result = joindepth.PASS
		}
	} else {
		// Normal (positive control) case.
		if verdict.ReachedDepth == expectedDepth && verdict.Err == nil {
			// Success: reached the expected depth.
			exitCode = 0
			result = joindepth.PASS
		} else if verdict.Err != nil {
			// Classify the error as transport or internal
			errMsg := verdict.Err.Error()
			if isTransportError(errMsg) {
				exitCode = 3
				result = joindepth.FAIL_TRANSPORT
			} else {
				exitCode = 1
				result = joindepth.FAIL_INTERNAL_ERROR
			}
		} else {
			// Connected but wrong depth.
			exitCode = 2
			result = joindepth.FAIL_WRONG_DEPTH
		}
	}

	// Emit VERDICT line and exit
	emitVerdictLine(string(result), verdict.ReachedDepth.String(), verdict.Detail)
	os.Exit(exitCode)
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
//  3. Returns QUERY depth if the connection succeeded.
//  4. Returns an error with transport-level details if the server is not reachable.
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
func probeARK(ctx context.Context, addr string) *joindepth.ProbeVerdict {
	// Parse addr to replace UDP with TCP on 27020.
	// The addr passed in is in format "host:7777" (game port).
	// We need to connect to "host:27020" (RCON port) instead.
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// If addr doesn't have a port, assume it's just the host.
		host = addr
	}
	rconAddr := net.JoinHostPort(host, "27020")

	// Retry connecting to the RCON port
	var lastErr error
	retryInterval := 3 * time.Second
	for {
		if err := ctx.Err(); err != nil {
			if lastErr == nil {
				lastErr = err
			}
			// Deadline expired: transport failure with UNKNOWN depth
			return &joindepth.ProbeVerdict{
				ReachedDepth: joindepth.JoinDepth(-1), // Renders as UNKNOWN
				Detail:       fmt.Sprintf("Dial timeout after deadline against %s; connection never established", rconAddr),
				Err:          fmt.Errorf("connection never established: %w", lastErr),
			}
		}

		actx, cancel := context.WithTimeout(ctx, 15*time.Second)
		d := net.Dialer{}
		conn, err := d.DialContext(actx, "tcp", rconAddr)
		cancel()

		if err == nil {
			conn.Close()
			log.Printf("connectivity-probe: successfully connected to RCON port (TCP 27020)")
			return &joindepth.ProbeVerdict{
				ReachedDepth: joindepth.QUERY,
				Detail:       "TCP connection to RCON port (27020) accepted; server is listening",
				Err:          nil,
			}
		}

		lastErr = err
		log.Printf("tcp-connect-rcon not ready yet: %v", err)

		select {
		case <-ctx.Done():
			// Deadline expired during retry loop
			return &joindepth.ProbeVerdict{
				ReachedDepth: joindepth.JoinDepth(-1), // Renders as UNKNOWN
				Detail:       fmt.Sprintf("Dial timeout after deadline against %s; connection never established", rconAddr),
				Err:          fmt.Errorf("connection never established: %w", lastErr),
			}
		case <-time.After(retryInterval):
		}
	}
}

// isTransportError returns true if the error message indicates a transport-level failure
// (connection refused, DNS failure, timeout, etc.)
func isTransportError(errMsg string) bool {
	return strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "Dial timeout") ||
		strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "connection never established") ||
		strings.Contains(errMsg, "DNS failure") ||
		strings.Contains(errMsg, "No response") ||
		strings.Contains(errMsg, "connection reset") ||
		strings.Contains(errMsg, "no such host")
}

// emitVerdictLine emits a VERDICT line to stdout in the canonical format:
// VERDICT\t<result>\t<depth>\t<detail>
func emitVerdictLine(result, depth, detail string) {
	line := fmt.Sprintf("VERDICT\t%s\t%s\t%s", result, depth, detail)
	fmt.Println(line)
}
