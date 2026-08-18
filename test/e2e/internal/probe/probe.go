// Package probe provides a shared retry loop and harness for per-game probes.
//
// Per-game probe binaries (test/e2e/internal/<game>/app.go) implement the
// probe.run callback to drive a headless protocol bot against a running game
// server. The harness handles retrying, deadline management, and depth
// validation, so every probe binary avoids re-implementing this logic.
//
// The probe owns its own retry loop: a game server accepts TCP — and, for
// some games, answers status queries — before it will accept a login, so a
// single attempt races world generation. Retrying here (rather than in the
// test) keeps the Job a one-shot pass/fail.
package probe

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

// ErrFatal marks a failure that retrying cannot fix (a misconfigured server
// rather than one that is merely still booting), so the probe gives up at once.
var ErrFatal = errors.New("fatal")

// Depth is how far into a real join a client got. See test/e2e/internal/specs.md.
type Depth string

// The three depths a probe can report, from deepest to shallowest.
const (
	Joined  Depth = "JOINED"  // server accepted the client as a player
	Partial Depth = "PARTIAL" // spoke the real protocol, then hit a credential gate CI can't mint
	Query   Depth = "QUERY"   // even a partial join impossible; spoke the query/status protocol
)

// retryInterval is the pause between attempts. The overall deadline, not the
// attempt count, bounds the loop.
const retryInterval = 3 * time.Second

// Flags are the standard flags every per-game probe binary accepts.
type Flags struct {
	Addr     string
	Deadline time.Duration
	Expect   Depth
}

// ParseFlags registers -addr, -deadline and -expect-depth, then calls
// flag.Parse. Per-game binaries may register extra flags BEFORE calling it.
func ParseFlags() Flags {
	addr := flag.String("addr", "", "game server host:port (in-cluster Service DNS)")
	deadline := flag.Duration("deadline", 4*time.Minute,
		"overall deadline; the probe retries until the server is playable or this elapses")
	expectDepth := flag.String("expect-depth", string(Joined),
		"expected join depth: JOINED | PARTIAL | QUERY")
	flag.Parse()

	if *addr == "" {
		log.Fatal("probe: -addr is required")
	}

	return Flags{
		Addr:     *addr,
		Deadline: *deadline,
		Expect:   Depth(*expectDepth),
	}
}

// Retry calls fn until it succeeds, ctx expires, or fn reports ErrFatal.
// Every failed attempt is logged so a failed Job's pod logs explain why the
// server never became playable.
func Retry(ctx context.Context, what string, attempt time.Duration, fn func(context.Context) error) error {
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
		if errors.Is(err, ErrFatal) {
			return err
		}
		last = err
		log.Printf("%s not ready yet: %v", what, err)

		select {
		case <-ctx.Done():
		case <-time.After(retryInterval):
		}
	}
}

// Main runs a game's probe to completion and never returns: it builds a
// context bounded by f.Deadline, calls run, and exits 0 only when run
// succeeded AND reached exactly the depth f.Expect. Any other outcome exits 1.
//
// The exact-depth check is the point: a PARTIAL game that starts JOINing means
// a credential gate moved and we must know, and one that regresses to a bare
// connect must fail just as loudly.
func Main(f Flags, run func(context.Context) (Depth, error)) {
	log.SetFlags(log.Ltime)

	ctx, cancel := context.WithTimeout(context.Background(), f.Deadline)
	defer cancel()

	depth, err := run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "probe: %v\n", err)
		os.Exit(1)
	}

	if depth != f.Expect {
		fmt.Fprintf(os.Stderr, "probe: reached %s, expected %s addr=%s\n", depth, f.Expect, f.Addr)
		os.Exit(1)
	}

	log.Printf("probe: depth=%s expected=%s addr=%s", depth, f.Expect, f.Addr)
}
