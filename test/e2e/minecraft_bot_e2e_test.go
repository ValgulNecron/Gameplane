//go:build e2e

package e2e

import (
	"testing"
	"time"
)

// TestGameServer_MinecraftJavaBot_Joined is the most end-to-end test in the
// suite: it stands up a REAL Minecraft server (itzg/minecraft-server) through
// the operator, waits for it to actually boot, and runs a headless protocol bot
// inside the cluster that pings the server and completes a login — proving the
// server is genuinely playable, not merely "Running" in Kubernetes.
//
// The bot runs as an in-cluster Job (see `runGameBotTest`) and dials the game
// Service directly rather than through a `kubectl port-forward` tunnel.
//
// Unlike the other GameServer tests (which use a busybox "fake game" and never
// wait for a Ready pod), this pulls a large external image and boots a JVM, so
// it is opt-in (set GAMEPLANE_E2E_GAME_BOT=1) and runs on its own CI job with a
// generous timeout. The protocol client is also exercised against the shipped
// minecraft-java template on a real cluster; here we use a trimmed vanilla
// template so it boots fast and fits a single kind node.
//
// Terraria has its own bespoke protocol bot (terraria_bot_e2e_test.go).
// Headless clients for the remaining shipped games are not viable:
//   - Valheim uses a proprietary, password-gated UDP protocol — no open
//     client — and boots via a multi-GB steamcmd download; same for Palworld.
//   - Factorio's game traffic is UDP-only and our module runs RCON off, so
//     there is no assertable control channel.
func TestGameServer_MinecraftJavaBot_Joined(t *testing.T) {
	skipUnlessGameInScope(t, "minecraft-java")

	runGameBotTest(t, gameBotSpec{
		Game:        "minecraft-java",
		Template:    "e2e-minecraft",
		DisplayName: "E2E Minecraft",
		Image:       "itzg/minecraft-server:java21",
		Env: map[string]string{
			"EULA":              "TRUE",
			"TYPE":              "VANILLA",
			"VERSION":           "1.21.4",
			"ONLINE_MODE":       "FALSE",
			"INIT_MEMORY":       "512M",
			"MAX_MEMORY":        "1G",
			"USE_AIKAR_FLAGS":   "false",
			"LEVEL_TYPE":        "FLAT",
			"VIEW_DISTANCE":     "4",
			"SPAWN_PROTECTION":  "0",
		},
		Ports: []gamePort{
			{Name: "game", Port: 25565, Protocol: "TCP"},
		},
		StorageSize:   "2Gi",
		MountPath:     "/data",
		Resources: gameResources{
			ReqCPU: "250m",
			ReqMem: "1Gi",
			LimCPU: "2",
			LimMem: "1536Mi",
		},
		ReadyTimeout:  10 * time.Minute,
		ProbePort:     25565,
		ProbeDeadline: 4 * time.Minute,
		ExpectDepth:   "JOINED",
		ProbeArgs:     []string{"-user", "gameplane-bot"},
		Probes: map[string]any{
			"readiness": map[string]any{
				"exec":                map[string]any{"command": []any{"mc-health"}},
				"initialDelaySeconds": int64(30),
				"periodSeconds":       int64(10),
				"failureThreshold":    int64(60),
			},
		},
	})
}
