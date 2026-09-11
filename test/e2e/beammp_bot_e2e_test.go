//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/protocol/joindepth"
)

// TestGameServer_BeamMPBot_Query boots a BeamMP dedicated server
// and connects via the custom TCP protocol to verify QUERY depth readiness.
//
// BeamMP is in the HEAVY game set due to image and vehicle asset size.
// This test DELIBERATELY NEVER RUNS IN CI. It is validated only by maintainer hand-run:
//
//	GAMEPLANE_E2E_REUSE_CLUSTER=1 GAMEPLANE_E2E_CONTEXT=<context> GAMEPLANE_E2E_GAME_BOT=1 GAMEPLANE_E2E_GAMES=beammp make test-e2e-keep
func TestGameServer_BeamMPBot_Query(t *testing.T) {
	skipUnlessGameInScope(t, "beammp")

	runGameBotTest(t, gameBotSpec{
		Game:        "beammp",
		Template:    "e2e-beammp-bot",
		DisplayName: "E2E BeamMP",
		Image:       "ghcr.io/valgulnecron/gameplane/beammp:latest@sha256:0000000000000000000000000000000000000000000000000000000000000000",
		Env: map[string]string{
			"BEAMMP_AUTH_KEY": "test-auth-key",
			"SERVER_NAME":     "Gameplane E2E BeamMP",
		},
		Ports: []gamePort{
			{Name: "game", Port: 30814, Protocol: "UDP"},
			{Name: "auth", Port: 30814, Protocol: "TCP"},
		},
		StorageSize: "5Gi",
		MountPath:   "/server/Root",
		Resources: gameResources{
			ReqCPU: "1", ReqMem: "2Gi",
			LimCPU: "4", LimMem: "4Gi",
		},
		ReadyTimeout:  10 * time.Minute,
		ProbePort:     30814,
		ProbeDeadline: 4 * time.Minute,
		ExpectDepth:   joindepth.QUERY,
		RCON: map[string]any{
			"protocol": "none",
		},
	})
}
