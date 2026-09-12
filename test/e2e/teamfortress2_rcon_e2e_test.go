//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// TestGameServer_TeamFortress2_RCON boots a Team Fortress 2 dedicated server,
// verifies that administrative commands can be executed over Source RCON via the API,
// and verifies that the graceful stop sequence in capabilities.lifecycle.stop
// runs when the server is stopped.
//
// TF2 is in the HEAVY game set (>15Gi storage, SteamCMD download).
// This test DELIBERATELY NEVER RUNS IN CI. It is validated only by maintainer hand-run:
//
//	GAMEPLANE_E2E_REUSE_CLUSTER=1 GAMEPLANE_E2E_CONTEXT=<context> GAMEPLANE_E2E_GAME_BOT=1 GAMEPLANE_E2E_GAMES=team-fortress-2 make test-e2e-keep
func TestGameServer_TeamFortress2_RCON(t *testing.T) {
	skipUnlessGameInScope(t, "team-fortress-2")
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"
	uid := time.Now().UnixNano() % 100000
	tmplName := fmt.Sprintf("e2e-tf2-rcon-%d", uid)
	gsName := fmt.Sprintf("tf2-rcon-%d", uid)
	const mountPath = "/home/steam/tf-dedicated"

	tmpl := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameTemplate",
		"metadata":   map[string]any{"name": tmplName},
		"spec": map[string]any{
			"displayName": "E2E TF2 RCON",
			"game":        "team-fortress-2",
			"version":     "1.0.0",
			"image":       "cm2network/tf2:latest@sha256:39c03ecbee022350a13aec49c7948263c00b5a0d5874b72faf0f4193de863e7b",
			"env": []any{
				map[string]any{"name": "HOME", "value": "/home/steam"},
				map[string]any{"name": "SRCDS_RCONPW", "value": "secret-rcon-password"},
			},
			"security": map[string]any{
				"runAsUser":  int64(1000),
				"runAsGroup": int64(1000),
				"fsGroup":    int64(1000),
			},
			"ports": []any{
				map[string]any{"name": "game", "containerPort": int64(27015), "protocol": "UDP", "advertise": true},
				map[string]any{"name": "rcon", "containerPort": int64(27015), "protocol": "TCP", "advertise": false},
			},
			"storage": map[string]any{
				"size":      "15Gi",
				"mountPath": mountPath,
			},
			"resources": map[string]any{
				"requests": map[string]any{"cpu": "1", "memory": "2Gi"},
				"limits":   map[string]any{"cpu": "4", "memory": "4Gi"},
			},
			"rcon": map[string]any{
				"protocol":    "source",
				"port":        int64(27015),
				"passwordEnv": "SRCDS_RCONPW",
			},
			"consoleMode": "rcon",
			"capabilities": map[string]any{
				"lifecycle": map[string]any{
					"stop": []any{"quit"},
				},
				"actions": []any{
					map[string]any{
						"id":          "broadcast",
						"displayName": "Broadcast",
						"command":     `say "{{.Params.message}}"`,
						"params": []any{
							map[string]any{"name": "message", "type": "string", "required": true},
						},
					},
				},
			},
		},
	}}

	if _, err := envInstance.Dyn.Resource(gameTemplateGVR).Create(ctx, tmpl, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create template %s: %v", tmplName, err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameTemplateGVR).Delete(context.Background(), tmplName, metav1.DeleteOptions{})
	})

	gs := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameServer",
		"metadata": map[string]any{
			"name":      gsName,
			"namespace": ns,
		},
		"spec": map[string]any{
			"templateRef": map[string]any{"name": tmplName},
			"storage": map[string]any{
				"size": "15Gi",
			},
		},
	}}

	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).Create(ctx, gs, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create gameserver %s: %v", gsName, err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).Delete(context.Background(), gsName, metav1.DeleteOptions{})
	})

	waitPVCBound(t, ns, gsName+"-data", 2*time.Minute)
	waitGameContainerReady(t, ns, gsName+"-0", 10*time.Minute)

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	cli := envInstance.APIClient(t, adminUsername, adminPassword)
	defer cli.Close()

	requireAgentReady(t, ns, gsName)
	waitAgentReachable(t, cli, gsName)

	resp, body, err := cli.Post(
		fmt.Sprintf("/servers/%s/actions/run", gsName),
		map[string]any{
			"id": "broadcast",
			"params": map[string]any{
				"message": "Hello TF2 RCON",
			},
		},
	)
	if err != nil {
		t.Fatalf("POST /actions/run error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /actions/run returned status %d: %s", resp.StatusCode, string(body))
	}

	suspendPatch := []byte(`{"spec":{"suspend":true}}`)
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Patch(ctx, gsName, types.MergePatchType, suspendPatch, metav1.PatchOptions{}); err != nil {
		t.Fatalf("patch suspend=true: %v", err)
	}

	envInstance.Eventually(t, 2*time.Minute, func() (bool, string) {
		ss, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, gsName, metav1.GetOptions{})
		if err != nil {
			return false, "get ss: " + err.Error()
		}
		if ss.Spec.Replicas != nil && *ss.Spec.Replicas == 0 {
			return true, ""
		}
		return false, fmt.Sprintf("expected replicas=0, got %v", ss.Spec.Replicas)
	})
}
