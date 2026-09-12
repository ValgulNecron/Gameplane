//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/protocol/joindepth"
)

// TestGameServer_EuroTruckSimulator2Bot_Query boots a Euro Truck Simulator 2 server
// and performs an A2S_INFO query against the query port to verify QUERY depth readiness.
//
// ETS2 is in the HEAVY game set due to SteamCMD runtime download requirements.
// This test DELIBERATELY NEVER RUNS IN CI. It is validated only by maintainer hand-run:
//
//	GAMEPLANE_E2E_REUSE_CLUSTER=1 GAMEPLANE_E2E_CONTEXT=<context> GAMEPLANE_E2E_GAME_BOT=1 GAMEPLANE_E2E_GAMES=euro-truck-simulator-2 make test-e2e-keep
func TestGameServer_EuroTruckSimulator2Bot_Query(t *testing.T) {
	skipUnlessGameInScope(t, "euro-truck-simulator-2")

	runGameBotTest(t, gameBotSpec{
		Game:        "euro-truck-simulator-2",
		Template:    "e2e-ets2-bot",
		DisplayName: "E2E Euro Truck Simulator 2",
		Image:       "ghcr.io/valgulnecron/gameplane/euro-truck-simulator-2:latest@sha256:0000000000000000000000000000000000000000000000000000000000000000",
		Env: map[string]string{
			"SERVER_LOGON_TOKEN": "test-logon-token",
			"SERVER_NAME":        "Gameplane E2E ETS2",
		},
		Ports: []gamePort{
			{Name: "game", Port: 27015, Protocol: "UDP"},
			{Name: "query", Port: 27016, Protocol: "UDP"},
		},
		StorageSize: "5Gi",
		MountPath:   "/home/steam/.local/share/Euro Truck Simulator 2",
		Resources: gameResources{
			ReqCPU: "1", ReqMem: "2Gi",
			LimCPU: "4", LimMem: "4Gi",
		},
		ReadyTimeout:  10 * time.Minute,
		ProbePort:     27016,
		ProbeDeadline: 4 * time.Minute,
		ExpectDepth:   joindepth.QUERY,
		RCON: map[string]any{
			"protocol": "none",
		},
	})
}
