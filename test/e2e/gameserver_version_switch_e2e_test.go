//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// TestGameServer_VersionSwitch proves the dashboard's version-switch flow
// against a real cluster: patching GameServer.spec.version re-renders the
// StatefulSet to the new catalog entry (env swap for env-versioned images)
// and the rollout converges. This is the operator-side contract the web
// Version settings section relies on; envtest covers the resolution matrix,
// this covers the live apply→rollout loop end to end. Zero API logins —
// it belongs to the operator bucket.
func TestGameServer_VersionSwitch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmplName := "e2e-verswitch-tmpl"
	ns := "gameplane-games"
	gsName := "e2e-verswitch-gs"

	// Busybox-backed template with two env-versioned entries, mirroring how
	// minecraft-java expresses versions (same image, different env).
	tmpl := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameTemplate",
		"metadata":   map[string]any{"name": tmplName},
		"spec": map[string]any{
			"displayName": "E2E version switch",
			"game":        "busybox",
			"version":     "1",
			"image":       "busybox:1.36",
			"command":     []any{"sh", "-c", "sleep 100000"},
			"ports": []any{
				map[string]any{"name": "noop", "containerPort": int64(12345), "advertise": true, "protocol": "TCP"},
			},
			"versions": []any{
				map[string]any{
					"id": "alpha", "displayName": "Alpha", "image": "busybox:1.36",
					"default": true,
					"env":     []any{map[string]any{"name": "CHANNEL", "value": "alpha"}},
				},
				map[string]any{
					"id": "beta", "displayName": "Beta", "image": "busybox:1.36",
					"env": []any{map[string]any{"name": "CHANNEL", "value": "beta"}},
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

	gs := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameServer",
		"metadata":   map[string]any{"name": gsName, "namespace": ns},
		"spec": map[string]any{
			"templateRef": map[string]any{"name": tmplName},
			"version":     "alpha",
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

	gameEnv := func() (map[string]string, error) {
		ss, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, gsName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		out := map[string]string{}
		for _, c := range ss.Spec.Template.Spec.Containers {
			if c.Name != "game" {
				continue
			}
			for _, e := range c.Env {
				out[e.Name] = e.Value
			}
		}
		return out, nil
	}

	envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
		env, err := gameEnv()
		if err != nil {
			return false, "statefulset: " + err.Error()
		}
		if env["CHANNEL"] != "alpha" {
			return false, "CHANNEL=" + env["CHANNEL"] + ", want alpha"
		}
		return true, ""
	})
	waitStatefulSetReplicas(t, ns, gsName, 1, 120*time.Second)

	// Switch versions the way the Settings tab does: patch spec.version.
	patch := []byte(`{"spec":{"version":"beta"}}`)
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Patch(ctx, gsName, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		t.Fatalf("patch spec.version: %v", err)
	}

	envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
		env, err := gameEnv()
		if err != nil {
			return false, "statefulset: " + err.Error()
		}
		if env["CHANNEL"] != "beta" {
			return false, "CHANNEL=" + env["CHANNEL"] + ", want beta"
		}
		return true, ""
	})
	// The rollout itself converges (pod restarts with the new spec).
	waitStatefulSetReplicas(t, ns, gsName, 1, 120*time.Second)
}
