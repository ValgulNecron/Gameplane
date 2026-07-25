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
		return probeGarrysMod(ctx, flags.Addr)
	})
}

// probeGarrysMod attempts to reach the deepest join depth on a Garry's Mod server.
// It queries A2S info (establishes QUERY depth).
// It attempts source protocol connection as diagnostic only (never changes the returned depth).
// Depth measurement: A2S success proves QUERY. Source protocol is optional instrumentation.
func probeGarrysMod(ctx context.Context, addr string) (probe.Depth, error) {
	// First: Query A2S to verify the server is alive and responding to queries.
	// This establishes QUERY depth and gives us server metadata.
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

	// Depth is now QUERY because A2S succeeded.
	// The source protocol attempt below is purely diagnostic and will not change the returned depth.

	// Second: Attempt source protocol challenge and connect.
	// This is instrumentation only; the server may or may not speak source protocol,
	// and the connect may succeed or fail. We log the outcome for evidence (raw bytes, etc.)
	// but neither success nor failure changes our QUERY depth measurement.
	// Future CI runs can escalate to PARTIAL or JOINED once the connect format is verified.
	var connectResult *source.ConnectResult
	if err := probe.Retry(ctx, "source-connect", 15*time.Second, func(actx context.Context) error {
		// Get a challenge token.
		challenge, err := source.Challenge(actx, addr)
		if err != nil {
			// Challenge failed; don't proceed to connect.
			// Log this as diagnostic but don't fail the probe.
			log.Printf("connect-probe: challenge failed: %v", err)
			return probe.ErrFatal // Signal to stop retrying without failing the probe itself.
		}

		// Attempt to connect using the challenge.
		// Use a generic bot name; sv_lan 1 in the template should allow joins without Steam auth.
		result, err := source.Connect(actx, addr, challenge, "gameplane-e2e-bot", source.ProtocolSource1)
		if err != nil {
			// Connect failed; log as diagnostic.
			log.Printf("connect-probe: connect error: %v", err)
			return probe.ErrFatal // Signal to stop retrying without failing the probe itself.
		}
		connectResult = result
		return nil
	}); err != nil {
		// Source protocol attempt failed (network, protocol error, etc.).
		// This is NOT fatal; A2S already proved QUERY depth.
		// No additional logging needed here; errors were logged in the retry callback.
		return probe.Query, nil
	}

	// Connect succeeded; decode and log the response for evidence.
	// Extract response type (byte 4, after the 0xFFFFFFFF header) for diagnosis.
	var respType string
	if len(connectResult.Raw) >= 5 {
		respType = fmt.Sprintf("0x%02x", connectResult.Raw[4])
	} else {
		respType = "truncated"
	}

	// Log outcome: either accepted or rejection reason.
	if connectResult.Accepted {
		log.Printf("connect-probe: response type %s — ACCEPTED (new client)", respType)
	} else if connectResult.RejectMsg != "" {
		// Log rejection reason; if it contains #GameUI_ token, extract and highlight it.
		log.Printf("connect-probe: response type %s — REJECTED: %s", respType, connectResult.RejectMsg)
	} else {
		log.Printf("connect-probe: response type %s — rejected (no reason provided)", respType)
	}

	// Also log raw hex for diagnosis, bounded to ~256 bytes to prevent log flooding.
	rawHex := hex.EncodeToString(connectResult.Raw)
	if len(rawHex) > 512 {
		rawHex = rawHex[:512] + "... (truncated)"
	}
	log.Printf("connect-probe: raw response (%d bytes): %s", len(connectResult.Raw), rawHex)

	// Regardless of connect outcome, return QUERY because that is what A2S proved.
	// The connect attempt was diagnostic; evidence is in the logs above.
	return probe.Query, nil
}
