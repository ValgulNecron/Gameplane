// Package main implements the Palworld join depth probe.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/probe"
	a2s "github.com/ValgulNecron/gameplane/test/e2e/internal/protocol/a2sproto"
)

func main() {
	flags := probe.ParseFlags()
	probe.Main(flags, func(ctx context.Context) (probe.Depth, error) {
		return probePalworld(ctx, flags.Addr)
	})
}

// probePalworld attempts to reach the deepest join depth on a Palworld server.
// It queries A2S info on port 27015 to establish QUERY depth.
// Depth measurement: A2S success on 27015 proves QUERY. The REST API on 8212
// and the game protocol on 8211 are not measured for join depth.
func probePalworld(ctx context.Context, addr string) (probe.Depth, error) {
	// First: Query A2S on the query port to verify the server is alive.
	// Palworld declares QUERY_PORT=27015 in the template; A2S is the standard
	// Steam query protocol used for server discovery.
	// A2S failure is fatal — server is not responding to queries at all.
	var info *a2s.Info
	if err := probe.Retry(ctx, "a2s-info", 15*time.Second, func(actx context.Context) error {
		var err error
		info, err = a2s.QueryInfo(actx, addr)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		// A2S failed; server not even responding to queries. This is fatal.
		return "", fmt.Errorf("a2s query failed: %w", err)
	}

	// Log server metadata from A2S response.
	log.Printf("a2s: server=%q map=%q players=%d/%d", info.Name, info.Map, info.Players, info.MaxPlayers)

	// Depth is QUERY because A2S succeeded.
	// The REST API on 8212 and game protocol on 8211 are not measured for depth:
	// - REST API requires HTTP Basic auth (admin password) we cannot mint.
	// - Game protocol (Unreal Engine) is undocumented and not publicly joinable.
	// A2S on 27015 is the only honest depth measurement.
	return probe.Query, nil
}
