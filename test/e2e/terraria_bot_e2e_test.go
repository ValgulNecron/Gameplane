//go:build e2e

package e2e

import (
	"testing"
	"time"
)

// TestGameServer_TerrariaBot_Joined boots a REAL Terraria server
// (passivelemon/terraria-docker — the same image the shipped terraria
// module uses) through the operator, waits for it to generate a world and
// reach Running, then runs a minimal Terraria-protocol bot inside the cluster
// to verify it reaches JOINED depth (server accepts the client as a player
// and answers WorldData).
//
// Terraria is the one non-Minecraft shipped game where this is practical:
// the server ships in the image (no steamcmd download) and speaks TCP.
// Valheim/Palworld boot via multi-GB steamcmd downloads over proprietary
// UDP protocols, and Factorio is UDP-only — none are bot-testable in CI.
//
// Like the Minecraft bot test, it is opt-in (GAMEPLANE_E2E_GAME_BOT=1) and
// runs in the bot bucket. Deliberately NOT t.Parallel(): two real game
// servers booting concurrently OOM-starves a single kind node.
func TestGameServer_TerrariaBot_Joined(t *testing.T) {
	skipUnlessGameInScope(t, "terraria")

	runGameBotTest(t, gameBotSpec{
		Game:        "terraria",
		Template:    "e2e-terraria",
		DisplayName: "E2E Terraria",
		// Pinned because the bot speaks protocol Terraria279 (1.4.4.x);
		// terraria-latest has moved to 1.4.5.x, whose protocol differs.
		// The server's version-mismatch kick (LegacyMultiplayer.4) names no version,
		// so the bot cannot self-correct. A blocking test must be hermetic.
		Image: "passivelemon/terraria-docker:terraria-1.4.4.9",
		Env: map[string]string{
			"WORLDNAME":  "e2e",
			"AUTOCREATE": "1", // small world
			"DIFFICULTY": "0",
			"MAXPLAYERS": "4",
			"SECURE":     "0",
		},
		Ports: []gamePort{
			{Name: "game", Port: 7777, Protocol: "TCP"},
		},
		StorageSize:   "2Gi",
		MountPath:     "/opt/terraria/config",
		Resources: gameResources{
			ReqCPU: "200m",
			ReqMem: "512Mi",
			LimCPU: "1",
			LimMem: "1536Mi",
		},
		ReadyTimeout:  10 * time.Minute,
		ProbePort:     7777,
		ProbeDeadline: 4 * time.Minute,
		ExpectDepth:   "JOINED",
		RCON:          map[string]any{"protocol": "none"},
		ConsoleMode:   "pty",
		Probes: map[string]any{
			"readiness": map[string]any{
				"tcpSocket":           map[string]any{"port": "game"},
				"initialDelaySeconds": int64(30),
				"periodSeconds":       int64(10),
				"failureThreshold":    int64(30),
			},
		},
	})
}
