//go:build e2e

package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestGameServer_MinecraftBotConnects is the most end-to-end test in the suite:
// it stands up a REAL Minecraft server (itzg/minecraft-server) through the
// operator, waits for it to actually boot, and runs a headless protocol bot
// inside the cluster that pings the server and completes a login — proving the
// server is genuinely playable, not merely "Running" in Kubernetes.
//
// The bot runs as an in-cluster Job (see Env.RunGameProbe) rather than dialing
// a `kubectl port-forward`: that tunnel corrupts the login handshake under CI
// load, which is what used to make this job advisory.
//
// Unlike the other GameServer tests (which use a busybox "fake game" and never
// wait for a Ready pod), this pulls a large external image and boots a JVM, so
// it is opt-in (set GAMEPLANE_E2E_GAME_BOT=1) and runs on its own CI job with a
// generous timeout. The mcbot client is also exercised against the shipped
// minecraft-java template on a real cluster; here we use a trimmed vanilla
// template so it boots fast and fits a single kind node.
//
// Terraria has its own bespoke protocol bot (terraria_bot_e2e_test.go).
// Headless clients for the remaining shipped games are not viable:
//   - Valheim uses a proprietary, password-gated UDP protocol — no open
//     client — and boots via a multi-GB steamcmd download; same for Palworld.
//   - Factorio's game traffic is UDP-only and our module runs RCON off, so
//     there is no assertable control channel.
func TestGameServer_MinecraftBotConnects(t *testing.T) {
	if os.Getenv("GAMEPLANE_E2E_GAME_BOT") == "" {
		t.Skip("heavy: set GAMEPLANE_E2E_GAME_BOT=1 to run the real-Minecraft bot test")
	}
	ctx := context.Background()
	ns := "gameplane-games"

	// A trimmed Minecraft GameTemplate: the same itzg image and game port as the
	// shipped modules/minecraft-java template, but vanilla with a small JVM heap
	// and a superflat world so it boots fast and fits a single kind node.
	// ONLINE_MODE=FALSE lets an unauthenticated headless bot complete a login.
	tmplName := "e2e-minecraft"
	tmpl := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameTemplate",
		"metadata":   map[string]any{"name": tmplName},
		"spec": map[string]any{
			"displayName": "E2E Minecraft",
			"game":        "minecraft-java",
			"version":     "1",
			"image":       "itzg/minecraft-server:java21",
			"env": []any{
				map[string]any{"name": "EULA", "value": "TRUE"},
				map[string]any{"name": "TYPE", "value": "VANILLA"},
				map[string]any{"name": "VERSION", "value": "1.21.4"},
				map[string]any{"name": "ONLINE_MODE", "value": "FALSE"},
				map[string]any{"name": "INIT_MEMORY", "value": "512M"},
				map[string]any{"name": "MAX_MEMORY", "value": "1G"},
				map[string]any{"name": "USE_AIKAR_FLAGS", "value": "false"},
				// Superflat + tiny view distance keeps first-boot world-gen
				// cheap so the JVM stays under the container memory limit.
				map[string]any{"name": "LEVEL_TYPE", "value": "FLAT"},
				map[string]any{"name": "VIEW_DISTANCE", "value": "4"},
				map[string]any{"name": "SPAWN_PROTECTION", "value": "0"},
			},
			"ports": []any{
				map[string]any{"name": "game", "containerPort": int64(25565), "advertise": true, "protocol": "TCP"},
			},
			"storage": map[string]any{"size": "2Gi", "mountPath": "/data"},
			"resources": map[string]any{
				"requests": map[string]any{"cpu": "250m", "memory": "1Gi"},
				"limits":   map[string]any{"cpu": "2", "memory": "1536Mi"},
			},
			"probes": map[string]any{
				"readiness": map[string]any{
					"exec":                map[string]any{"command": []any{"mc-health"}},
					"initialDelaySeconds": int64(30),
					"periodSeconds":       int64(10),
					"failureThreshold":    int64(60),
				},
			},
		},
	}}
	if _, err := envInstance.Dyn.Resource(gameTemplateGVR).
		Create(ctx, tmpl, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create template: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameTemplateGVR).
			Delete(context.Background(), tmplName, metav1.DeleteOptions{})
	})

	gsName := "e2e-mc-bot"
	gs := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameServer",
		"metadata":   map[string]any{"name": gsName, "namespace": ns},
		"spec": map[string]any{
			"templateRef": map[string]any{"name": tmplName},
		},
	}}
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Create(ctx, gs, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create gameserver: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Delete(context.Background(), gsName, metav1.DeleteOptions{})
	})

	// Wait for the server to actually boot. The operator reports Running once
	// the pod is Ready (mc-health passes) and the agent heartbeat is fresh.
	// First boot pulls the image, downloads the vanilla jar, and generates the
	// world, so allow several minutes.
	envInstance.Eventually(t, 10*time.Minute, func() (bool, string) {
		obj, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).Get(ctx, gsName, metav1.GetOptions{})
		if err != nil {
			return false, "get gs: " + err.Error()
		}
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		if phase == "Running" {
			return true, ""
		}
		return false, "phase=" + phase
	})

	// Drive the bot from inside the cluster: it pings the server for its
	// protocol version and then completes a real login. ONLINE_MODE=FALSE means
	// the server skips encryption and answers Login Success for our offline bot.
	//
	// gameprobe retries both steps internally — a Minecraft server answers
	// server-list pings while it is still preparing the spawn area but rejects
	// logins until the world is ready, so a single attempt races world
	// generation.
	envInstance.RunGameProbe(t, ns, gsName, "minecraft", 25565, 4*time.Minute)
}
