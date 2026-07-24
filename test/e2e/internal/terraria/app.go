package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/probe"
	"github.com/ValgulNecron/gameplane/test/e2e/internal/terraria/protocol"
)

func main() {
	flags := probe.ParseFlags()
	probe.Main(flags, func(ctx context.Context) (probe.Depth, error) {
		return probeTerraria(ctx, flags.Addr)
	})
}

// probeTerraria implements the Terraria join depth check: connects to the
// server, requests world data, and returns the join depth.
func probeTerraria(ctx context.Context, addr string) (probe.Depth, error) {
	// Retry the handshake at 15 seconds per attempt.
	var conn *protocol.Conn
	if err := probe.Retry(ctx, "handshake", 15*time.Second, func(actx context.Context) error {
		var err error
		conn, _, err = protocol.Connect(actx, addr)
		if errors.Is(err, protocol.ErrPasswordRequired) {
			// Password prompt proves the protocol works but the server is misconfigured
			// (this template sets no password, so this is a fatal error).
			return fmt.Errorf("%w: unexpected password prompt (template sets no password)", probe.ErrFatal)
		}
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return "", err
	}

	// After successful Connect, request world data with its own timeout.
	defer conn.Close()
	worldDataCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := conn.RequestWorldData(worldDataCtx); err != nil {
		return "", err
	}

	return probe.Joined, nil
}
