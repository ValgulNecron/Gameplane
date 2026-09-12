//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/protocol/joindepth"
)

// TestGameServer_ArkSurvivalEvolvedBot_Query boots an ARK: Survival Evolved dedicated server
// and performs an A2S_INFO query against the query port to verify QUERY depth readiness.
//
// ASE is in the HEAVY game set due to massive SteamCMD download and >30Gi storage requirements.
// This test DELIBERATELY NEVER RUNS IN CI. It is validated only by maintainer hand-run:
//
//	GAMEPLANE_E2E_REUSE_CLUSTER=1 GAMEPLANE_E2E_CONTEXT=<context> GAMEPLANE_E2E_GAME_BOT=1 GAMEPLANE_E2E_GAMES=ark-survival-evolved make test-e2e-keep
func TestGameServer_ArkSurvivalEvolvedBot_Query(t *testing.T) {
	skipUnlessGameInScope(t, "ark-survival-evolved")

	runGameBotTest(t, gameBotSpec{
		Game:        "ark-survival-evolved",
		Template:    "e2e-ase-bot",
		DisplayName: "E2E ARK: Survival Evolved",
		Image:       "ghcr.io/valgulnecron/gameplane/ark-survival-evolved:latest@sha256:0000000000000000000000000000000000000000000000000000000000000000",
		Env: map[string]string{
			"SESSION_NAME":  "Gameplane E2E ASE",
			"RCON_PASSWORD": "secret-rcon-password",
		},
		Ports: []gamePort{
			{Name: "game", Port: 7777, Protocol: "UDP"},
			{Name: "query", Port: 27015, Protocol: "UDP"},
			{Name: "rcon", Port: 27020, Protocol: "TCP"},
		},
		StorageSize: "30Gi",
		MountPath:   "/serverdata/ShooterGame/Saved",
		Resources: gameResources{
			ReqCPU: "2", ReqMem: "8Gi",
			LimCPU: "4", LimMem: "16Gi",
		},
		ReadyTimeout:  10 * time.Minute,
		ProbePort:     27015,
		ProbeDeadline: 4 * time.Minute,
		ExpectDepth:   joindepth.QUERY,
		RCON: map[string]any{
			"protocol": "source",
			"port":     int64(27020),
		},
	})
}
