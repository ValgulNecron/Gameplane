// Command gameprobe drives a headless protocol bot against a running game
// server and exits 0 only once the server is genuinely playable.
//
// It is built into a small image and run as a Job *inside* the kind cluster by
// the e2e game-bot tests, so the bot reaches the game over the cluster network
// (Service DNS) rather than through a `kubectl port-forward` tunnel. Under CI
// load that tunnel corrupts the Minecraft login handshake and the server drops
// the connection ("Failed to decode packet 'serverbound/minecraft:hello'"),
// which is why the game-bot job used to be advisory.
//
// The probe owns its own retry loop: a game server accepts TCP — and, for
// Minecraft, answers server-list pings — before it will accept a login, so a
// single attempt races world generation. Retrying here (rather than in the
// test) keeps the Job a one-shot pass/fail.
//
//	gameprobe -game minecraft -addr mc.gameplane-games.svc.cluster.local:25565
//	gameprobe -game terraria  -addr tr.gameplane-games.svc.cluster.local:7777
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/mcbot"
	"github.com/ValgulNecron/gameplane/test/e2e/internal/terrabot"
)

// errFatal marks a failure that retrying cannot fix (a misconfigured server
// rather than one that is merely still booting), so the probe gives up at once.
var errFatal = errors.New("fatal")

// retryInterval is the pause between attempts. The overall deadline, not the
// attempt count, bounds the loop.
const retryInterval = 3 * time.Second

func main() {
	log.SetFlags(log.Ltime)
	game := flag.String("game", "", "game protocol to probe: minecraft | terraria")
	addr := flag.String("addr", "", "game server host:port (in-cluster Service DNS)")
	user := flag.String("user", "gameplane-e2e-bot", "username the minecraft bot logs in with")
	deadline := flag.Duration("deadline", 4*time.Minute,
		"overall deadline; the probe retries until the server is playable or this elapses")
	flag.Parse()

	if *addr == "" {
		log.Fatal("gameprobe: -addr is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *deadline)
	defer cancel()

	var err error
	switch *game {
	case "minecraft":
		err = probeMinecraft(ctx, *addr, *user)
	case "terraria":
		err = probeTerraria(ctx, *addr)
	default:
		log.Fatalf("gameprobe: -game must be minecraft or terraria, got %q", *game)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "gameprobe: %s probe failed: %v\n", *game, err)
		os.Exit(1)
	}
	log.Printf("gameprobe: %s server at %s is playable", *game, *addr)
}

// retry calls fn until it succeeds, ctx expires, or fn reports errFatal. Every
// failed attempt is logged so a failed Job's pod logs explain why the server
// never became playable.
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
		if errors.Is(err, errFatal) {
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

// probeMinecraft pings the server for its protocol version, then completes an
// offline-mode login. Only Login Success proves the world is serving players.
func probeMinecraft(ctx context.Context, addr, user string) error {
	var st *mcbot.Status
	err := retry(ctx, "server-list ping", 15*time.Second, func(c context.Context) error {
		s, err := mcbot.Ping(c, addr)
		if err != nil {
			return err
		}
		st = s
		return nil
	})
	if err != nil {
		return err
	}
	log.Printf("ping ok: version=%q protocol=%d players=%d/%d",
		st.Version.Name, st.Version.Protocol, st.Players.Online, st.Players.Max)

	// The server answers pings while it is still preparing the spawn area but
	// rejects logins until the world is ready, so the login is retried too.
	return retry(ctx, "login", 20*time.Second, func(c context.Context) error {
		r, err := mcbot.Login(c, addr, st.Version.Protocol, user)
		if err != nil {
			return err
		}
		switch r.Outcome {
		case mcbot.Success:
			log.Printf("login ok: server accepted %q", r.Detail)
			return nil
		case mcbot.NeedsAuth:
			// ONLINE_MODE was not disabled: no amount of retrying lets an
			// unauthenticated bot in.
			return fmt.Errorf("%w: server is in online-mode: %s", errFatal, r.Detail)
		default:
			return fmt.Errorf("login refused: %s", r.Detail)
		}
	})
}

// probeTerraria completes the join handshake and then asks for the world
// header — the server answering WorldData is the "actually playable" assertion.
func probeTerraria(ctx context.Context, addr string) error {
	var (
		conn *terrabot.Conn
		res  *terrabot.ConnectResult
	)
	err := retry(ctx, "terraria handshake", 15*time.Second, func(c context.Context) error {
		cn, r, err := terrabot.Connect(c, addr)
		if errors.Is(err, terrabot.ErrPasswordRequired) {
			return fmt.Errorf("%w: unexpected password prompt (template sets no password)", errFatal)
		}
		if err != nil {
			return err
		}
		conn, res = cn, r
		return nil
	})
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Printf("terraria handshake ok: slot=%d protocol=%s", res.Slot, res.Version)

	wctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := conn.RequestWorldData(wctx); err != nil {
		return fmt.Errorf("world data request: %w", err)
	}
	log.Print("server answered WorldData — world is being served to clients")
	return nil
}
