//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/protocol/joindepth"
)

// TestGameServer_HellLetLooseBot_Query boots a Hell Let Loose dedicated server
// and performs an A2S_INFO query against the query port to verify QUERY depth readiness.
//
// Hell Let Loose is in the HEAVY game set due to SteamCMD download and >30Gi storage requirements.
// This test DELIBERATELY NEVER RUNS IN CI. It is validated only by maintainer hand-run:
//
//	GAMEPLANE_E2E_REUSE_CLUSTER=1 GAMEPLANE_E2E_CONTEXT=<context> GAMEPLANE_E2E_GAME_BOT=1 GAMEPLANE_E2E_GAMES=hell-let-loose make test-e2e-keep
func TestGameServer_HellLetLooseBot_Query(t *testing.T) {
	skipUnlessGameInScope(t, "hell-let-loose")

	runGameBotTest(t, gameBotSpec{
		Game:        "hell-let-loose",
		Template:    "e2e-hll-bot",
		DisplayName: "E2E Hell Let Loose",
		Image:       "ghcr.io/valgulnecron/gameplane/hell-let-loose:latest@sha256:0000000000000000000000000000000000000000000000000000000000000000",
		Env: map[string]string{
			"SERVER_NAME":   "Gameplane E2E HLL",
			"RCON_PASSWORD": "secret-rcon-password",
		},
		Ports: []gamePort{
			{Name: "game", Port: 7787, Protocol: "UDP"},
			{Name: "query", Port: 27165, Protocol: "UDP"},
			{Name: "rcon", Port: 22222, Protocol: "TCP"},
		},
		StorageSize: "30Gi",
		MountPath:   "/serverdata/HLL/Saved",
		Resources: gameResources{
			ReqCPU: "2", ReqMem: "6Gi",
			LimCPU: "4", LimMem: "12Gi",
		},
		ReadyTimeout:  10 * time.Minute,
		ProbePort:     27165,
		ProbeDeadline: 4 * time.Minute,
		ExpectDepth:   joindepth.QUERY,
		RCON: map[string]any{
			"protocol": "source",
			"port":     int64(22222),
		},
	})
}
