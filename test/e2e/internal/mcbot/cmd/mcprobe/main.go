// Command mcprobe is a small CLI around the mcbot package: it pings a
// Minecraft: Java Edition server and (optionally) attempts a login, printing
// what it found. Handy for manually checking that a Kestrel-managed server is
// actually reachable and playable.
//
//	go run ./internal/mcbot/cmd/mcprobe -addr 127.0.0.1:25565 -login
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/mcbot"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:25565", "Minecraft server host:port")
	login := flag.Bool("login", false, "attempt an (offline) login after the ping")
	user := flag.String("user", "kestrel-probe", "username to log in with")
	timeout := flag.Duration("timeout", 30*time.Second, "overall timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	st, err := mcbot.Ping(ctx, *addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ping failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("ping ok: version=%q protocol=%d players=%d/%d\n",
		st.Version.Name, st.Version.Protocol, st.Players.Online, st.Players.Max)

	if !*login {
		return
	}
	res, err := mcbot.Login(ctx, *addr, st.Version.Protocol, *user)
	if err != nil {
		fmt.Fprintf(os.Stderr, "login failed: %v\n", err)
		os.Exit(1)
	}
	switch res.Outcome {
	case mcbot.Success:
		fmt.Printf("login ok: server accepted %q\n", res.Detail)
	case mcbot.NeedsAuth:
		fmt.Printf("login needs auth: %s\n", res.Detail)
	case mcbot.Disconnected:
		fmt.Printf("login disconnected: %s\n", res.Detail)
		os.Exit(2)
	}
}
