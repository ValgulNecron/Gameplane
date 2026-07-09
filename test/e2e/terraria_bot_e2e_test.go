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

// TestGameServer_TerrariaBotConnects boots a REAL Terraria server
// (passivelemon/terraria-docker — the same image the shipped terraria
// module uses) through the operator, waits for it to generate a world and
// reach Running, then runs a minimal Terraria-protocol bot inside the cluster:
// ConnectRequest → ContinueConnecting, then a world-data request the server
// must answer. That proves the module's template boots a genuinely joinable
// server, not merely a pod that accepts TCP.
//
// Terraria is the one non-Minecraft shipped game where this is practical:
// the server ships in the image (no steamcmd download) and speaks TCP.
// Valheim/Palworld boot via multi-GB steamcmd downloads over proprietary
// UDP protocols, and Factorio is UDP-only — none are bot-testable in CI.
//
// Like the Minecraft bot test, it is opt-in (GAMEPLANE_E2E_GAME_BOT=1) and
// runs in the bot bucket. Deliberately NOT t.Parallel(): two real game
// servers booting concurrently OOM-starves a single kind node.
func TestGameServer_TerrariaBotConnects(t *testing.T) {
	if os.Getenv("GAMEPLANE_E2E_GAME_BOT") == "" {
		t.Skip("heavy: set GAMEPLANE_E2E_GAME_BOT=1 to run the real-Terraria bot test")
	}
	ctx := context.Background()
	ns := "gameplane-games"

	// A trimmed Terraria GameTemplate: same image and port as the shipped
	// modules/terraria template, generating the smallest world so first
	// boot stays fast. Env names match passivelemon/terraria-docker.
	tmplName := "e2e-terraria"
	tmpl := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameTemplate",
		"metadata":   map[string]any{"name": tmplName},
		"spec": map[string]any{
			"displayName": "E2E Terraria",
			"game":        "terraria",
			"version":     "1",
			"image":       "passivelemon/terraria-docker:terraria-latest",
			"env": []any{
				map[string]any{"name": "WORLDNAME", "value": "e2e"},
				map[string]any{"name": "AUTOCREATE", "value": "1"}, // small world
				map[string]any{"name": "DIFFICULTY", "value": "0"},
				map[string]any{"name": "MAXPLAYERS", "value": "4"},
				map[string]any{"name": "SECURE", "value": "0"},
			},
			"ports": []any{
				map[string]any{"name": "game", "containerPort": int64(7777), "advertise": true, "protocol": "TCP"},
			},
			"storage": map[string]any{"size": "2Gi", "mountPath": "/opt/terraria/config"},
			"resources": map[string]any{
				"requests": map[string]any{"cpu": "200m", "memory": "512Mi"},
				"limits":   map[string]any{"cpu": "1", "memory": "1536Mi"},
			},
			"rcon":        map[string]any{"protocol": "none"},
			"consoleMode": "pty",
			"probes": map[string]any{
				"readiness": map[string]any{
					"tcpSocket":           map[string]any{"port": "game"},
					"initialDelaySeconds": int64(30),
					"periodSeconds":       int64(10),
					"failureThreshold":    int64(30),
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

	gsName := "e2e-terraria-bot"
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

	// First boot pulls the image, extracts the server, and generates a
	// small world — allow several minutes to reach Running.
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

	// Drive the bot from inside the cluster: ConnectRequest →
	// ContinueConnecting, then a world-data request the server must answer.
	//
	// gameprobe retries the handshake internally — the readiness probe is a TCP
	// check, so the pod can be Ready a moment before the server answers the
	// protocol. A password prompt would also prove the protocol, but this
	// template sets none, so the probe treats one as a hard failure.
	envInstance.RunGameProbe(t, ns, gsName, "terraria", 7777, 4*time.Minute)
}
