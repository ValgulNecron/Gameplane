//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/protocol/joindepth"
)

// TestGameServer_FiveMBot_Query boots a FiveM dedicated server and probes it
// via the dynamic.json endpoint to verify QUERY depth readiness.
//
// FiveM is in the HEAVY game set due to container size and embedded database.
// This test DELIBERATELY NEVER RUNS IN CI. It is validated only by maintainer hand-run:
//
//	GAMEPLANE_E2E_REUSE_CLUSTER=1 GAMEPLANE_E2E_CONTEXT=<context> GAMEPLANE_E2E_GAME_BOT=1 GAMEPLANE_E2E_GAMES=fivem make test-e2e-keep
func TestGameServer_FiveMBot_Query(t *testing.T) {
	skipUnlessGameInScope(t, "fivem")

	runGameBotTest(t, gameBotSpec{
		Game:        "fivem",
		Template:    "e2e-fivem-bot",
		DisplayName: "E2E FiveM",
		Image:       "ghcr.io/valgulnecron/gameplane/fivem:latest@sha256:0000000000000000000000000000000000000000000000000000000000000000",
		Env: map[string]string{
			"CFX_LICENSE_KEY": "test-cfx-license-key",
			"SV_HOSTNAME":     "Gameplane E2E FiveM",
		},
		Ports: []gamePort{
			{Name: "game", Port: 30120, Protocol: "UDP"},
			{Name: "game-tcp", Port: 30120, Protocol: "TCP"},
			{Name: "txadmin", Port: 40120, Protocol: "TCP"},
		},
		StorageSize: "10Gi",
		MountPath:   "/server-data",
		Resources: gameResources{
			ReqCPU: "1", ReqMem: "2Gi",
			LimCPU: "4", LimMem: "4Gi",
		},
		ReadyTimeout:  10 * time.Minute,
		ProbePort:     30120,
		ProbeDeadline: 4 * time.Minute,
		ExpectDepth:   joindepth.QUERY,
		RCON: map[string]any{
			"protocol": "rest",
			"port":     int64(40120),
		},
	})
}
