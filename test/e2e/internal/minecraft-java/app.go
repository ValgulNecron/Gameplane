package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/minecraft-java/protocol"
	"github.com/ValgulNecron/gameplane/test/e2e/internal/probe"
)

func main() {
	// Register -user flag before calling probe.ParseFlags.
	user := flag.String("user", "gameplane-bot",
		"username the minecraft bot logs in with (Minecraft allows at most 16 characters)")

	// ParseFlags registers -addr, -deadline, -expect-depth and calls flag.Parse.
	flags := probe.ParseFlags()

	// Main runs the probe and exits with the appropriate code.
	probe.Main(flags, func(c context.Context) (probe.Depth, error) {
		return probeMinecraft(c, flags.Addr, *user)
	})
}

// probeMinecraft pings the server for its protocol version, then completes an
// offline-mode login. Only Login Success proves the world is serving players.
func probeMinecraft(ctx context.Context, addr, user string) (probe.Depth, error) {
	var st *protocol.Status

	// Retry the server-list ping until the server answers or the deadline passes.
	err := probe.Retry(ctx, "server-list ping", probeAttempt, func(c context.Context) error {
		s, err := protocol.Ping(c, addr)
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
		r, err := protocol.Login(c, addr, st.Version.Protocol, user)
		if err != nil {
			return err
		}
		switch r.Outcome {
		case protocol.Success:
			log.Printf("login ok: server accepted %q", r.Detail)
			return nil
		case protocol.NeedsAuth:
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

const (
	probeAttempt = 15 * time.Second // per-attempt timeout for ping
	loginAttempt = 20 * time.Second // per-attempt timeout for login
)
