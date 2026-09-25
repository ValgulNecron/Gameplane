//go:build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// TestModule_DigestPinEnforcedAfterInstall checks Module.spec.digest
// against a real OCI registry. A Module installs unpinned and reports a
// sha256 manifest digest; a pin that does not match the source content,
// set while the Module is Ready and with the version unchanged, moves it
// to Failed with reason DigestMismatch. The content it already applied,
// and its GameTemplate, stay in place.
func TestModule_DigestPinEnforcedAfterInstall(t *testing.T) {
	t.Parallel()
	// Shares the fixed-name oras-push Job with the other module tests —
	// serialize against them (see ociPushMu).
	ociPushMu.Lock()
	defer ociPushMu.Unlock()

	parent := t
	ctx := context.Background()

	// Registry + bundle (idempotent; shared with the other Module e2e
	// tests, which run sequentially in the same cluster).
	envInstance.ApplyYAML(t, "oci-registry.yaml")
	envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
		dep, err := envInstance.K8s.AppsV1().Deployments("gameplane-system").
			Get(ctx, "gameplane-test-registry", metav1.GetOptions{})
		if err != nil {
			return false, "get registry deploy: " + err.Error()
		}
		if dep.Status.ReadyReplicas >= 1 {
			return true, ""
		}
		return false, "registry not ready yet"
	})
	envInstance.OCIPush(t, "gameplane-system", "oras-push-test-game")

	const (
		sourceName = "e2e-digest-pin-source"
		moduleCR   = "e2e-digest-pin-module"
	)

	src := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "ModuleSource",
		"metadata":   map[string]any{"name": sourceName},
		"spec": map[string]any{
			"type": "oci",
			"oci": map[string]any{
				"url":      "gameplane-test-registry.gameplane-system.svc:5000",
				"insecure": true,
				"modules":  []any{map[string]any{"name": "e2e-test-game"}},
			},
			"refreshInterval": "10m",
		},
	}}
	if _, err := envInstance.Dyn.Resource(moduleSourceGVR).
		Create(ctx, src, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create digest-pin modulesource: %v", err)
	}
	parent.Cleanup(func() {
		_ = envInstance.Dyn.Resource(moduleSourceGVR).
			Delete(context.Background(), sourceName, metav1.DeleteOptions{})
	})

	envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
		got, err := envInstance.Dyn.Resource(moduleSourceGVR).Get(ctx, sourceName, metav1.GetOptions{})
		if err != nil {
			return false, "get modulesource: " + err.Error()
		}
		modules, _, _ := unstructured.NestedSlice(got.Object, "status", "modules")
		if len(modules) == 0 {
			return false, "status.modules still empty"
		}
		return true, ""
	})

	mod := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "Module",
		"metadata":   map[string]any{"name": moduleCR},
		"spec": map[string]any{
			"source":  map[string]any{"name": sourceName},
			"name":    "e2e-test-game",
			"version": "0.1.0",
		},
	}}
	if _, err := envInstance.Dyn.Resource(moduleGVR).
		Create(ctx, mod, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create digest-pin module: %v", err)
	}
	parent.Cleanup(func() {
		_ = envInstance.Dyn.Resource(moduleGVR).
			Delete(context.Background(), moduleCR, metav1.DeleteOptions{})
	})

	// Unpinned install reaches Ready with a sha256 manifest digest.
	var appliedDigest string
	envInstance.Eventually(t, 2*time.Minute, func() (bool, string) {
		got, err := envInstance.Dyn.Resource(moduleGVR).Get(ctx, moduleCR, metav1.GetOptions{})
		if err != nil {
			return false, "get module: " + err.Error()
		}
		phase, _, _ := unstructured.NestedString(got.Object, "status", "phase")
		if phase != "Ready" {
			lastErr, _, _ := unstructured.NestedString(got.Object, "status", "lastError")
			return false, "module phase=" + phase + " lastError=" + lastErr
		}
		appliedDigest, _, _ = unstructured.NestedString(got.Object, "status", "appliedDigest")
		if !strings.HasPrefix(appliedDigest, "sha256:") {
			return false, "appliedDigest=" + appliedDigest + " (want sha256:...)"
		}
		return true, ""
	})

	// Pin a digest the source content does not have. The version is
	// unchanged, so the pin check is what must move the Module off Ready.
	const otherDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	patch := []byte(`{"spec":{"digest":"` + otherDigest + `"}}`)
	if _, err := envInstance.Dyn.Resource(moduleGVR).
		Patch(ctx, moduleCR, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		t.Fatalf("pin module digest: %v", err)
	}

	envInstance.Eventually(t, 2*time.Minute, func() (bool, string) {
		got, err := envInstance.Dyn.Resource(moduleGVR).Get(ctx, moduleCR, metav1.GetOptions{})
		if err != nil {
			return false, "get module: " + err.Error()
		}
		phase, _, _ := unstructured.NestedString(got.Object, "status", "phase")
		if phase != "Failed" {
			return false, "module phase=" + phase + " (want Failed)"
		}
		conds, _, _ := unstructured.NestedSlice(got.Object, "status", "conditions")
		for _, c := range conds {
			cm, ok := c.(map[string]any)
			if !ok || cm["type"] != "Ready" {
				continue
			}
			if cm["reason"] != "DigestMismatch" {
				return false, "Ready reason=" + asString(cm["reason"])
			}
			return true, ""
		}
		return false, "no Ready condition"
	})

	// The refused pin leaves the applied content and its GameTemplate in place.
	got, err := envInstance.Dyn.Resource(moduleGVR).Get(ctx, moduleCR, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get module: %v", err)
	}
	if d, _, _ := unstructured.NestedString(got.Object, "status", "appliedDigest"); d != appliedDigest {
		t.Errorf("status.appliedDigest=%q, want %q", d, appliedDigest)
	}
	if _, err := envInstance.Dyn.Resource(gameTemplateGVR).Get(ctx, moduleCR, metav1.GetOptions{}); err != nil {
		t.Errorf("GameTemplate %q should still exist after a refused pin: %v", moduleCR, err)
	}
}
