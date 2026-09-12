//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/protocol/joindepth"
)

// TestGameServer_ArmaReforgerBot_Query boots an Arma Reforger dedicated server
// and performs an A2S_INFO query against the query port to verify QUERY depth readiness.
//
// Arma Reforger is in the HEAVY game set due to Enfusion engine size and SteamCMD download.
// This test DELIBERATELY NEVER RUNS IN CI. It is validated only by maintainer hand-run:
//
//	GAMEPLANE_E2E_REUSE_CLUSTER=1 GAMEPLANE_E2E_CONTEXT=<context> GAMEPLANE_E2E_GAME_BOT=1 GAMEPLANE_E2E_GAMES=arma-reforger make test-e2e-keep
func TestGameServer_ArmaReforgerBot_Query(t *testing.T) {
	skipUnlessGameInScope(t, "arma-reforger")

	runGameBotTest(t, gameBotSpec{
		Game:        "arma-reforger",
		Template:    "e2e-arma-reforger-bot",
		DisplayName: "E2E Arma Reforger",
		Image:       "ghcr.io/valgulnecron/gameplane/arma-reforger:latest@sha256:0000000000000000000000000000000000000000000000000000000000000000",
		Env: map[string]string{
			"SERVER_NAME": "Gameplane E2E Arma Reforger",
		},
		Ports: []gamePort{
			{Name: "game", Port: 2001, Protocol: "UDP"},
			{Name: "query", Port: 17777, Protocol: "UDP"},
		},
		StorageSize: "25Gi",
		MountPath:   "/home/steam/.local/share/ArmaReforgerServer",
		Resources: gameResources{
			ReqCPU: "2", ReqMem: "8Gi",
			LimCPU: "6", LimMem: "16Gi",
		},
		ReadyTimeout:  10 * time.Minute,
		ProbePort:     17777,
		ProbeDeadline: 4 * time.Minute,
		ExpectDepth:   joindepth.QUERY,
		RCON: map[string]any{
			"protocol": "none",
		},
	})
}
