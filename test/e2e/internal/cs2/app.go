package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/probe"
	"github.com/ValgulNecron/gameplane/test/e2e/internal/protocol/a2s"
	"github.com/ValgulNecron/gameplane/test/e2e/internal/protocol/source"
)

func main() {
	flags := probe.ParseFlags()
	probe.Main(flags, func(ctx context.Context) (probe.Depth, error) {
		return probeCS2(ctx, flags.Addr)
	})
}

// probeCS2 implements the CS2 join depth check: queries the server via A2S
// to verify connectivity and protocol support. The A2S query succeeds determines
// the depth (QUERY). The Source protocol handshake attempt (Challenge + Connect)
// is purely diagnostic — it is logged richly but never changes the returned depth
// or fails the probe.
func probeCS2(ctx context.Context, addr string) (probe.Depth, error) {
	// Query the server via A2S to verify it is listening and responding to queries.
	// CS2 is Source 2, which still supports the Source A2S query protocol.
	var info *a2s.Info
	if err := probe.Retry(ctx, "A2S query", 15*time.Second, func(actx context.Context) error {
		var err error
		info, err = a2s.QueryInfo(actx, addr)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return "", fmt.Errorf("A2S QueryInfo failed: %w", err)
	}

	log.Printf("A2S query succeeded: server=%q players=%d/%d map=%q", info.Name, info.Players, info.MaxPlayers, info.Map)

	// A2S succeeded; the depth is QUERY. Now run the Source protocol handshake
	// diagnostically (Challenge + Connect). This is purely for evidence: does the
	// server speak Source protocol? Where does it fail? Log the outcome richly,
	// but never fail the probe or change the returned depth based on this attempt.
	connectProbe(ctx, addr)

	// Depth determined by A2S success alone.
	return probe.Query, nil
}

// connectProbe attempts a Source protocol Challenge + Connect handshake
// and logs the outcome richly for debugging. It is purely diagnostic and
// never affects the probe outcome.
func connectProbe(ctx context.Context, addr string) {
	var challenge uint32
	var challengeOk bool
	err := probe.Retry(ctx, "Source challenge", 15*time.Second, func(actx context.Context) error {
		var err error
		challenge, err = source.Challenge(actx, addr)
		if err != nil {
			return err
		}
		challengeOk = true
		return nil
	})

	if err != nil {
		log.Printf("connect-probe: challenge failed (server does not speak Source): %v", err)
		return
	}

	if !challengeOk {
		log.Printf("connect-probe: challenge did not complete")
		return
	}

	log.Printf("connect-probe: challenge obtained")

	// Attempt to connect with the challenge. The bot name is fixed.
	var result *source.ConnectResult
	err = probe.Retry(ctx, "Source connect", 15*time.Second, func(actx context.Context) error {
		r, err := source.Connect(actx, addr, challenge, "gameplane-e2e-bot", source.ProtocolSource2)
		if err != nil {
			return err
		}
		result = r
		return nil
	})

	if err != nil {
		log.Printf("connect-probe: connect attempt failed: %v", err)
		return
	}

	if result == nil {
		log.Printf("connect-probe: no response")
		return
	}

	// Log the connect response bytes for debugging.
	if result.Raw != nil && len(result.Raw) > 0 {
		hexStr := hex.EncodeToString(result.Raw)
		// Bound to avoid huge logs; truncate if necessary.
		if len(hexStr) > 256 {
			hexStr = hexStr[:256] + "..."
		}
		log.Printf("connect-probe: response received (length=%d, hex=%s)", len(result.Raw), hexStr)
	}

	if !result.Accepted {
		// Connection was rejected.
		log.Printf("connect-probe: connect rejected: %s", result.RejectMsg)
		return
	}

	log.Printf("connect-probe: connect accepted")
}
