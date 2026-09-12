//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestGameServer_DayZ_Workshop verifies that Steam Workshop mod IDs configured under
// GameServer.spec.mods.ids are projected into the MOD_LIST environment variable
// for the DayZ game container, per capabilities.mods.idList in modules/dayz/template.yaml.
//
// Retargeted from Garry's Mod per maintainer ruling in OPEN-DECISIONS.md §9 (T094, T096).
// DayZ is in the HEAVY game set (40Gi storage, SteamCMD download).
// This test DELIBERATELY NEVER RUNS IN CI. It is validated only by maintainer hand-run:
//
//	GAMEPLANE_E2E_REUSE_CLUSTER=1 GAMEPLANE_E2E_CONTEXT=<context> GAMEPLANE_E2E_GAME_BOT=1 GAMEPLANE_E2E_GAMES=dayz make test-e2e-keep
func TestGameServer_DayZ_Workshop(t *testing.T) {
	skipUnlessGameInScope(t, "dayz")
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"
	uid := time.Now().UnixNano() % 100000
	tmplName := fmt.Sprintf("e2e-dayz-workshop-%d", uid)
	gsName := fmt.Sprintf("dayz-workshop-%d", uid)
	const mountPath = "/data"

	tmpl := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameTemplate",
		"metadata":   map[string]any{"name": tmplName},
		"spec": map[string]any{
			"displayName": "E2E DayZ Workshop",
			"game":        "dayz",
			"version":     "1.1.1",
			"image":       "registry.godbleak.dev/godbleak/serverz:latest@sha256:5e8757beae763c862d08a9c08587c35211cda74ce399f7419492d7520adab4fe",
			"env": []any{
				map[string]any{"name": "PORT", "value": "2302"},
				map[string]any{"name": "STEAM_QUERY_PORT", "value": "27015"},
				map[string]any{"name": "BE_PORT", "value": "2305"},
				map[string]any{"name": "BE_IP", "value": "127.0.0.1"},
				map[string]any{"name": "BE_PASSWORD", "value": "secret-rcon-password"},
			},
			"ports": []any{
				map[string]any{"name": "game", "containerPort": int64(2302), "protocol": "UDP", "advertise": true},
				map[string]any{"name": "query", "containerPort": int64(27015), "protocol": "UDP", "advertise": false},
				map[string]any{"name": "rcon", "containerPort": int64(2305), "protocol": "UDP", "advertise": false},
			},
			"storage": map[string]any{
				"size":      "40Gi",
				"mountPath": mountPath,
			},
			"resources": map[string]any{
				"requests": map[string]any{"cpu": "2", "memory": "6Gi"},
				"limits":   map[string]any{"cpu": "4", "memory": "12Gi"},
			},
			"rcon": map[string]any{
				"protocol":    "battleye",
				"port":        int64(2305),
				"passwordEnv": "BE_PASSWORD",
			},
			"consoleMode": "rcon",
			"capabilities": map[string]any{
				"mods": map[string]any{
					"idList": map[string]any{
						"env":       "MOD_LIST",
						"separator": ",",
						"mode":      "replace",
					},
					"registry": map[string]any{
						"providers": []any{
							map[string]any{
								"provider":   "steam",
								"steamAppID": int64(221100),
							},
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
				"size": "40Gi",
			},
			"mods": map[string]any{
				"ids": []any{
					map[string]any{"id": "1559212036"}, // Community Framework (CF)
					map[string]any{"id": "1564026768"}, // Community-Online-Tools
				},
			},
		},
	}}

	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).Create(ctx, gs, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create gameserver %s: %v", gsName, err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).Delete(context.Background(), gsName, metav1.DeleteOptions{})
	})

	waitPVCBound(t, ns, gsName+"-data", 90*time.Second)

	// Verify the game pod has the projected MOD_LIST env var containing both workshop IDs joined with comma
	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		podList, err := envInstance.K8s.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("gameplane.local/gameserver=%s", gsName),
		})
		if err != nil {
			return false, "list pods: " + err.Error()
		}
		if len(podList.Items) == 0 {
			return false, "no pod found yet"
		}
		pod := podList.Items[0]
		var gameContainer *corev1.Container
		for i := range pod.Spec.Containers {
			if pod.Spec.Containers[i].Name == "game" {
				gameContainer = &pod.Spec.Containers[i]
				break
			}
		}
		if gameContainer == nil {
			return false, "game container not found in pod"
		}

		var modListVal string
		found := false
		for _, e := range gameContainer.Env {
			if e.Name == "MOD_LIST" {
				modListVal = e.Value
				found = true
				break
			}
		}
		if !found {
			return false, "MOD_LIST env var not found in game container"
		}

		expected := "1559212036,1564026768"
		if !strings.Contains(modListVal, expected) {
			return false, fmt.Sprintf("MOD_LIST value %q does not contain expected %q", modListVal, expected)
		}
		return true, ""
	})
}
