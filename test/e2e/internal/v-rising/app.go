package main

import (
	"context"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/protocol/a2sproto"
	"github.com/ValgulNecron/gameplane/test/e2e/internal/protocol/joindepth"
)

func main() {
	// Standard flags per the probe CLI contract.
	addr := flag.String("addr", "", "game server host:port (in-cluster Service DNS)")
	deadline := flag.Duration("deadline", 4*time.Minute,
		"overall deadline; the probe retries until the server is playable or this elapses")
	expectDepth := flag.String("expect-depth", "JOINED",
		"expected join depth: QUERY | PARTIAL | JOINED")
	expectFail := flag.Bool("expect-fail", false,
		"if set, probe must NOT reach -expect-depth; exits 0 on correct failure")

	flag.Parse()

	// Set up logging.
	log.SetFlags(log.Ltime)

	// Validate required flags.
	if *addr == "" {
		verdict := joindepth.ProbeVerdict{
			ReachedDepth: joindepth.QUERY,
			Detail:       "Bad flag: -addr is required",
			Err:          errors.New("-addr is required"),
		}
		emitVerdictAndExit(&verdict, joindepth.QUERY, *expectFail, 1)
	}

	// Parse expected depth.
	parsedExpect, err := joindepth.Parse(*expectDepth)
	if err != nil {
		verdict := joindepth.ProbeVerdict{
			ReachedDepth: joindepth.QUERY,
			Detail:       fmt.Sprintf("Bad flag: %v", err),
			Err:          err,
		}
		emitVerdictAndExit(&verdict, joindepth.QUERY, *expectFail, 1)
	}

	// Create the probe context with deadline.
	ctx, cancel := context.WithTimeout(context.Background(), *deadline)
	defer cancel()

	// Run the probe.
	reached, evidence, transportErr := probeVRising(ctx, *addr)

	// Build the verdict.
	verdict := joindepth.ProbeVerdict{
		ReachedDepth: reached,
		Detail:       evidence,
		Err:          transportErr,
	}

	// Determine exit code based on the contract.
	exitCode := computeExitCode(&verdict, parsedExpect, *expectFail)
	emitVerdictAndExit(&verdict, parsedExpect, *expectFail, exitCode)
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
// and in practice it was also dead code: retry consumes the entire
// remaining deadline retrying A2S before returning, so by the time control
// reached the fallback the shared context was already expired and the
// fallback's own retry call returned immediately without ever dialing.
// A2S is in fact the genuine, documented protocol here (see above), so it is
// now the sole basis for QUERY depth.
//
// Depth measurement: Returns (QUERY, evidence, nil) if the query port answers A2S_INFO.
// Returns (UNKNOWN, "", error) on transport failure. Returns (UNKNOWN, "", error) on protocol error.
// V Rising does not support join verification, so the maximum reachable depth is QUERY.
// Returns (depth, evidence, error).
func probeVRising(ctx context.Context, addr string) (joindepth.JoinDepth, string, error) {
	var info *a2sproto.Info
	err := retry(ctx, "a2s-info", probeAttempt, func(actx context.Context) error {
		var qerr error
		info, qerr = a2sproto.QueryInfo(actx, addr)
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
		return joindepth.JoinDepth(-1), "", fmt.Errorf("a2s query on query port %s: %w", addr, err)
	}

	log.Printf("a2s query succeeded: server=%q map=%q players=%d/%d",
		info.Name, info.Map, info.Players, info.MaxPlayers)
	evidence := fmt.Sprintf("A2S_INFO response: %s, %d/%d players", info.Name, info.Players, info.MaxPlayers)
	return joindepth.QUERY, evidence, nil
}

// computeExitCode returns the appropriate exit code based on the verdict and expectations.
// Exit codes per the contract:
// - 0: reached expected depth (or under -expect-fail, correctly failed)
// - 1: internal error
// - 2: connected but wrong depth
// - 3: transport failure
func computeExitCode(v *joindepth.ProbeVerdict, expectedDepth joindepth.JoinDepth, expectFail bool) int {
	if expectFail {
		// Under -expect-fail, the probe must NOT reach the expected depth.
		if v.ReachedDepth == expectedDepth && v.Err == nil {
			// Probe reached the expected depth when it should not have.
			return 1
		}
		// Probe correctly failed to reach the depth.
		return 0
	}

	// Normal (positive control) case.
	if v.ReachedDepth == expectedDepth && v.Err == nil {
		// Success: reached the expected depth.
		return 0
	}

	// Classify the failure.
	if v.Err != nil {
		// Determine if it's a transport error or internal error.
		if isTransportError(v.Err) {
			return 3 // Transport failure.
		}
		return 1 // Internal error.
	}

	// Connected but wrong depth.
	return 2
}

// isTransportError checks if an error is a transport-level failure.
func isTransportError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	transportKeywords := []string{
		"connection refused",
		"Dial timeout",
		"timeout",
		"connection never established",
		"DNS failure",
		"No response",
		"connection reset",
		"dial",
		"EOF",
		"refused",
	}
	for _, keyword := range transportKeywords {
		if strings.Contains(errMsg, keyword) {
			return true
		}
	}
	return false
}

// emitVerdictAndExit emits the machine-readable VERDICT line and exits.
func emitVerdictAndExit(v *joindepth.ProbeVerdict, expectedDepth joindepth.JoinDepth, expectFail bool, exitCode int) {
	// Encode the verdict line.
	line, err := v.Encode()
	if err != nil {
		// If encoding fails, emit a fallback and exit with internal error.
		fmt.Printf("VERDICT\tFAIL_INTERNAL_ERROR\tUNKNOWN\tVERDICT encoding failed: %v\n", err)
		os.Exit(1)
	}

	// Emit the verdict line to stdout.
	fmt.Println(line)

	// Exit with the computed code.
	os.Exit(exitCode)
}

const (
	probeAttempt  = 15 * time.Second // per-attempt timeout for A2S query
	retryInterval = 3 * time.Second   // pause between attempts
)

// retry calls fn until it succeeds, ctx expires, or fn reports a fatal error.
func retry(ctx context.Context, what string, attempt time.Duration, fn func(context.Context) error) error {
	var last error
	for {
		if err := ctx.Err(); err != nil {
			if last == nil {
				last = err
			}
			return fmt.Errorf("%s never succeeded before the deadline: %w", what, last)
		}

		actx, cancel := context.WithTimeout(ctx, attempt)
		err := fn(actx)
		cancel()
		if err == nil {
			return nil
		}
		last = err
		log.Printf("%s not ready yet: %v", what, err)

		select {
		case <-ctx.Done():
		case <-time.After(retryInterval):
		}
	}
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
