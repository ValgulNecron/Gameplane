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
)

// TestGameServer_TModLoader_Mods boots a tModLoader server with capabilities.mods configured,
// installs a mod into the mods directory via the agent API, and asserts the mod is properly
// listed and present on the filesystem.
//
// tModLoader is classified under the heavy test suite due to SteamCMD and .NET runtime size.
// This test DELIBERATELY NEVER RUNS IN CI. It is validated only by maintainer hand-run:
//
//	GAMEPLANE_E2E_REUSE_CLUSTER=1 GAMEPLANE_E2E_CONTEXT=<context> GAMEPLANE_E2E_GAME_BOT=1 GAMEPLANE_E2E_GAMES=tmodloader make test-e2e-keep
func TestGameServer_TModLoader_Mods(t *testing.T) {
	skipUnlessGameInScope(t, "tmodloader")
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"
	uid := time.Now().UnixNano() % 100000
	tmplName := fmt.Sprintf("e2e-tmodloader-mods-%d", uid)
	gsName := fmt.Sprintf("tmodloader-mods-%d", uid)
	const mountPath = "/data"

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	cli := envInstance.APIClient(t, adminUsername, adminPassword)
	defer cli.Close()

	tmpl := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameTemplate",
		"metadata":   map[string]any{"name": tmplName},
		"spec": map[string]any{
			"displayName": "E2E tModLoader Mods",
			"game":        "tmodloader",
			"version":     "1.0.0",
			"image":       "passivelemon/terraria-docker:tmodloader-latest@sha256:3f2d8703421159f1037084bd2c0901a3a63b85a00801cf36f6f928e8b666b44e",
			"env": []any{
				map[string]any{"name": "HOME", "value": "/data"},
			},
			"security": map[string]any{
				"runAsUser":  int64(1000),
				"runAsGroup": int64(1000),
				"fsGroup":    int64(1000),
			},
			"ports": []any{
				map[string]any{"name": "game", "containerPort": int64(7777), "protocol": "TCP", "advertise": true},
			},
			"storage": map[string]any{
				"size":      "5Gi",
				"mountPath": mountPath,
			},
			"resources": map[string]any{
				"requests": map[string]any{"cpu": "1", "memory": "2Gi"},
				"limits":   map[string]any{"cpu": "2", "memory": "4Gi"},
			},
			"rcon": map[string]any{
				"protocol": "none",
			},
			"consoleMode": "pty",
			"capabilities": map[string]any{
				"lifecycle": map[string]any{
					"stop": []any{"exit"},
				},
				"mods": map[string]any{
					"path":       "Mods",
					"extensions": []any{".tmod", ".zip"},
					"install": map[string]any{
						"allowedHosts": []any{"raw.githubusercontent.com", "github.com", ".githubusercontent.com"},
						"maxSizeMB":    int64(512),
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
				"size": "5Gi",
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
	requireAgentReady(t, ns, gsName)

	// Wait for the agent to report mods readiness
	envInstance.Eventually(t, 30*time.Second, func() (bool, string) {
		resp, body, err := cli.Get("/servers/" + gsName + "/mods")
		if err != nil {
			return false, "GET /mods: " + err.Error()
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return false, fmt.Sprintf("status=%d body=%s", resp.StatusCode, string(body))
		}
		return true, ""
	})

	// Install a test mod file into the configured Mods directory
	modName := "testmod.tmod"
	resp, body, err := cli.Post("/servers/"+gsName+"/mods/install", map[string]any{
		"url":  modDownloadURL,
		"name": modName,
		"meta": map[string]any{
			"provider":      "custom",
			"projectId":     "e2e-tmodloader-test",
			"versionId":     "v1",
			"versionNumber": "1.0.0",
		},
	})
	if err != nil {
		t.Fatalf("POST /mods/install: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("install expected 200, got %d body=%q", resp.StatusCode, string(body))
	}

	mods := listServerMods(t, cli, gsName)
	found := false
	for _, m := range mods {
		if m.Name == modName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected mod %q to be listed after install, got: %+v", modName, mods)
	}
}
