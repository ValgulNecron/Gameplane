package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/minecraft-java/minecraftproto"
	"github.com/ValgulNecron/gameplane/test/e2e/internal/protocol/joindepth"
)

func main() {
	// Register -user and -mode flags before parsing standard flags.
	user := flag.String("user", "gameplane-bot",
		"username the minecraft bot logs in with (Minecraft allows at most 16 characters)")
	mode := flag.String("mode", "join",
		"probe mode: join (ping + login, full join, expect-depth JOINED) | "+
			"ping (server-list ping only, never logs in, expect-depth QUERY) | "+
			"wake (ping + a single non-retried login attempt, tolerant of the "+
			"connection being dropped mid-response — for exercising a "+
			"wake-on-connect sentinel's handshake parser, expect-depth PARTIAL)")

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
			ReachedDepth: joindepth.QUERY, // dummy depth for error case
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

	// Run the probe based on mode.
	var reached joindepth.JoinDepth
	var evidence string
	var transportErr error

	switch *mode {
	case "ping":
		reached, evidence, transportErr = probeMinecraftPing(ctx, *addr)
	case "wake":
		reached, evidence, transportErr = probeMinecraftWake(ctx, *addr, *user)
	default:
		reached, evidence, transportErr = probeMinecraft(ctx, *addr, *user)
	}

	// Build the verdict.
	verdict := joindepth.ProbeVerdict{
		ReachedDepth: reached,
		Detail:       evidence,
		Err:          transportErr,
	}

	// Determine exit code based on the contract.
	exitCode := joindepth.ExitCodeFromVerdict(&verdict, parsedExpect, *expectFail)
	emitVerdictAndExit(&verdict, parsedExpect, *expectFail, exitCode)
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

// probeMinecraft pings the server for its protocol version, then completes an
// offline-mode login. Only Login Success proves the world is serving players.
// Returns (depth, evidence, error).
func probeMinecraft(ctx context.Context, addr, user string) (joindepth.JoinDepth, string, error) {
	var st *minecraftproto.Status

	// Retry the server-list ping until the server answers or the deadline passes.
	err := retry(ctx, "server-list ping", probeAttempt, func(c context.Context) error {
		s, err := minecraftproto.Ping(c, addr)
		if err != nil {
			return err
		}
		st = s
		return nil
	})
	if err != nil {
		// Transport failure before reaching any depth.
		evidence := err.Error()
		return joindepth.JoinDepth(-1), evidence, fmt.Errorf("server-list ping: %w", err)
	}

	log.Printf("ping ok: version=%q protocol=%d players=%d/%d",
		st.Version.Name, st.Version.Protocol, st.Players.Online, st.Players.Max)

	// The server answers pings while it is still preparing the spawn area but
	// rejects logins until the world is ready, so the login is retried too.
	var loginResult *minecraftproto.LoginResult
	err = retry(ctx, "login", loginAttempt, func(c context.Context) error {
		r, err := minecraftproto.Login(c, addr, st.Version.Protocol, user)
		if err != nil {
			return err
		}
		loginResult = r
		switch r.Outcome {
		case minecraftproto.Success:
			log.Printf("login ok: server accepted %q", r.Detail)
			return nil
		case minecraftproto.NeedsAuth:
			// ONLINE_MODE was not disabled: no amount of retrying lets an
			// unauthenticated bot in.
			return errFatal{fmt.Errorf("server is in online-mode: %s", r.Detail)}
		default:
			return fmt.Errorf("login refused: %s", r.Detail)
		}
	})
	if err != nil {
		if _, ok := err.(errFatal); ok {
			// Non-retryable error: server is in online-mode. Report as PARTIAL.
			evidence := fmt.Sprintf("Encryption Request (0x01) sent; %s", loginResult.Detail)
			return joindepth.PARTIAL, evidence, nil
		}
		return joindepth.QUERY, "", fmt.Errorf("login: %w", err)
	}

	// Success: server sent Login Success.
	evidence := fmt.Sprintf("Login Success packet (0x02); username %q accepted", user)
	return joindepth.JOINED, evidence, nil
}

// probeMinecraftPing issues a server-list ping and nothing else. It is used
// to prove that reaching a wake-on-connect sentinel with a status query does
// NOT trigger a wake — unlike probeMinecraft, it never attempts a login.
// Returns (depth, evidence, error).
func probeMinecraftPing(ctx context.Context, addr string) (joindepth.JoinDepth, string, error) {
	var st *minecraftproto.Status

	err := retry(ctx, "server-list ping", probeAttempt, func(c context.Context) error {
		s, err := minecraftproto.Ping(c, addr)
		if err != nil {
			return err
		}
		st = s
		return nil
	})
	if err != nil {
		// Transport failure before reaching QUERY.
		evidence := err.Error()
		return joindepth.JoinDepth(-1), evidence, fmt.Errorf("server-list ping: %w", err)
	}

	log.Printf("ping ok: version=%q protocol=%d players=%d/%d",
		st.Version.Name, st.Version.Protocol, st.Players.Online, st.Players.Max)

	evidence := fmt.Sprintf("Server List Ping: %d/%d players online", st.Players.Online, st.Players.Max)
	return joindepth.QUERY, evidence, nil
}

// probeMinecraftWake pings the server, then makes exactly ONE login attempt —
// enough to exercise a wake-on-connect sentinel's handshake parser, which is
// expected to read the login packet, patch the wake-request annotation, and
// then drop the connection without completing a real join. Unlike
// probeMinecraft's join, the login response is NOT retried and any outcome
// (Login Success, an auth demand, a Disconnect packet, or a read error from
// the dropped connection) is accepted here: what actually proves the wake
// fired is the GameServer's annotation/status, which the e2e test asserts
// via the K8s API — nothing observable on the wire settles it.
// Returns (depth, evidence, error).
func probeMinecraftWake(ctx context.Context, addr, user string) (joindepth.JoinDepth, string, error) {
	var st *minecraftproto.Status

	err := retry(ctx, "server-list ping", probeAttempt, func(c context.Context) error {
		s, err := minecraftproto.Ping(c, addr)
		if err != nil {
			return err
		}
		st = s
		return nil
	})
	if err != nil {
		// Transport failure before reaching QUERY.
		evidence := err.Error()
		return joindepth.JoinDepth(-1), evidence, fmt.Errorf("server-list ping: %w", err)
	}

	loginCtx, cancel := context.WithTimeout(ctx, loginAttempt)
	defer cancel()
	res, err := minecraftproto.Login(loginCtx, addr, st.Version.Protocol, user)
	if err != nil {
		log.Printf("wake login attempt closed (expected once the sentinel wakes the server): %v", err)
		// For wake probes, a connection drop or timeout is expected and counts as PARTIAL.
		evidence := "Connection accepted; sentinel dropped mid-handshake"
		return joindepth.PARTIAL, evidence, nil
	}

	log.Printf("wake login attempt: outcome=%v detail=%s", res.Outcome, res.Detail)
	evidence := fmt.Sprintf("Received %v response: %s", res.Outcome, res.Detail)
	return joindepth.PARTIAL, evidence, nil
}

// errFatal wraps an error to indicate it's non-retryable.
type errFatal struct {
	err error
}

func (e errFatal) Error() string {
	return e.err.Error()
}

const (
	probeAttempt = 15 * time.Second // per-attempt timeout for ping
	loginAttempt = 20 * time.Second // per-attempt timeout for login
	retryInterval = 3 * time.Second  // pause between attempts
)

// retry calls fn until it succeeds, ctx expires, or fn reports a fatal error.
// It's similar to probe.Retry but adapted for the new contract.
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
		if _, ok := err.(errFatal); ok {
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
