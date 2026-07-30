// Package main is the Minecraft Java probe.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/minecraft-java/minecraftproto"
	"github.com/ValgulNecron/gameplane/test/e2e/internal/probe"
)

func main() {
	// Register -user and -mode flags before calling probe.ParseFlags.
	user := flag.String("user", "gameplane-bot",
		"username the minecraft bot logs in with (Minecraft allows at most 16 characters)")
	mode := flag.String("mode", "join",
		"probe mode: join (ping + login, full join, expect-depth JOINED) | "+
			"ping (server-list ping only, never logs in, expect-depth QUERY) | "+
			"wake (ping + a single non-retried login attempt, tolerant of the "+
			"connection being dropped mid-response — for exercising a "+
			"wake-on-connect sentinel's handshake parser, expect-depth PARTIAL)")

	// ParseFlags registers -addr, -deadline, -expect-depth and calls flag.Parse.
	flags := probe.ParseFlags()

	// Main runs the probe and exits with the appropriate code.
	probe.Main(flags, func(c context.Context) (probe.Depth, error) {
		switch *mode {
		case "ping":
			return probeMinecraftPing(c, flags.Addr)
		case "wake":
			return probeMinecraftWake(c, flags.Addr, *user)
		default:
			return probeMinecraft(c, flags.Addr, *user)
		}
	})
}

// probeMinecraft pings the server for its protocol version, then completes an
// offline-mode login. Only Login Success proves the world is serving players.
func probeMinecraft(ctx context.Context, addr, user string) (probe.Depth, error) {
	var st *minecraftproto.Status

	// Retry the server-list ping until the server answers or the deadline passes.
	err := probe.Retry(ctx, "server-list ping", probeAttempt, func(c context.Context) error {
		s, err := minecraftproto.Ping(c, addr)
		if err != nil {
			return err
		}
		st = s
		return nil
	})
	if err != nil {
		return "", err
	}

	log.Printf("ping ok: version=%q protocol=%d players=%d/%d",
		st.Version.Name, st.Version.Protocol, st.Players.Online, st.Players.Max)

	// The server answers pings while it is still preparing the spawn area but
	// rejects logins until the world is ready, so the login is retried too.
	err = probe.Retry(ctx, "login", loginAttempt, func(c context.Context) error {
		r, err := minecraftproto.Login(c, addr, st.Version.Protocol, user)
		if err != nil {
			return err
		}
		switch r.Outcome {
		case minecraftproto.Success:
			log.Printf("login ok: server accepted %q", r.Detail)
			return nil
		case minecraftproto.NeedsAuth:
			// ONLINE_MODE was not disabled: no amount of retrying lets an
			// unauthenticated bot in.
			return fmt.Errorf("%w: server is in online-mode: %s", probe.ErrFatal, r.Detail)
		default:
			return fmt.Errorf("login refused: %s", r.Detail)
		}
	})
	if err != nil {
		return "", err
	}

	return probe.Joined, nil
}

// probeMinecraftPing issues a server-list ping and nothing else. It is used
// to prove that reaching a wake-on-connect sentinel with a status query does
// NOT trigger a wake — unlike probeMinecraft, it never attempts a login.
func probeMinecraftPing(ctx context.Context, addr string) (probe.Depth, error) {
	var st *minecraftproto.Status

	err := probe.Retry(ctx, "server-list ping", probeAttempt, func(c context.Context) error {
		s, err := minecraftproto.Ping(c, addr)
		if err != nil {
			return err
		}
		st = s
		return nil
	})
	if err != nil {
		return "", err
	}

	log.Printf("ping ok: version=%q protocol=%d players=%d/%d",
		st.Version.Name, st.Version.Protocol, st.Players.Online, st.Players.Max)

	return probe.Query, nil
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
func probeMinecraftWake(ctx context.Context, addr, user string) (probe.Depth, error) {
	var st *minecraftproto.Status

	err := probe.Retry(ctx, "server-list ping", probeAttempt, func(c context.Context) error {
		s, err := minecraftproto.Ping(c, addr)
		if err != nil {
			return err
		}
		st = s
		return nil
	})
	if err != nil {
		return "", err
	}

	loginCtx, cancel := context.WithTimeout(ctx, loginAttempt)
	defer cancel()
	res, err := minecraftproto.Login(loginCtx, addr, st.Version.Protocol, user)
	if err != nil {
		log.Printf("wake login attempt closed (expected once the sentinel wakes the server): %v", err)
	} else {
		log.Printf("wake login attempt: outcome=%v detail=%s", res.Outcome, res.Detail)
	}

	return probe.Partial, nil
}

const (
	probeAttempt = 15 * time.Second // per-attempt timeout for ping
	loginAttempt = 20 * time.Second // per-attempt timeout for login
)
