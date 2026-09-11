//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/ValgulNecron/gameplane/test/e2e/internal/protocol/joindepth"
)

// TestGameServer_TeamFortress2Bot_Query boots a Team Fortress 2 dedicated server
// and performs an A2S_INFO query to verify QUERY depth readiness.
//
// TF2 is in the HEAVY game set due to SteamCMD server download footprint.
// This test DELIBERATELY NEVER RUNS IN CI. It is validated only by maintainer hand-run:
//
//	GAMEPLANE_E2E_REUSE_CLUSTER=1 GAMEPLANE_E2E_CONTEXT=<context> GAMEPLANE_E2E_GAME_BOT=1 GAMEPLANE_E2E_GAMES=team-fortress-2 make test-e2e-keep
func TestGameServer_TeamFortress2Bot_Query(t *testing.T) {
	skipUnlessGameInScope(t, "team-fortress-2")

	runGameBotTest(t, gameBotSpec{
		Game:        "team-fortress-2",
		Template:    "e2e-tf2-bot",
		DisplayName: "E2E Team Fortress 2",
		Image:       "cm2network/tf2:latest@sha256:0000000000000000000000000000000000000000000000000000000000000000",
		Env: map[string]string{
			"SRCDS_TOKEN": "test-srcds-token",
			"START_MAP":   "ctf_2fort",
		},
		Ports: []gamePort{
			{Name: "game", Port: 27015, Protocol: "UDP"},
			{Name: "rcon", Port: 27015, Protocol: "TCP"},
		},
		StorageSize: "15Gi",
		MountPath:   "/home/steam/tf-dedicated",
		Resources: gameResources{
			ReqCPU: "1", ReqMem: "1Gi",
			LimCPU: "2", LimMem: "2Gi",
		},
		ReadyTimeout:  10 * time.Minute,
		ProbePort:     27015,
		ProbeDeadline: 4 * time.Minute,
		ExpectDepth:   joindepth.QUERY,
		RCON: map[string]any{
			"protocol": "source",
			"port":     int64(27015),
		},
	})
}
