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
)

// TestModule_VerifyRejectsUnsignedBundle exercises the cosign verification
// gate end-to-end: a ModuleSource with spec.verify.key set, pulling an
// UNSIGNED bundle from the in-cluster registry, must drive the Module to
// Failed and materialize no GameTemplate.
//
// We assert only the rejection path. Producing a genuinely cosign-signed
// bundle in-cluster is impractical (no cosign/sigstore tooling in CI, and the
// kind clusters are offline w.r.t. Fulcio/Rekor); the signed-success path is
// covered by the operator's envtest fakeVerifier suite. Keyed verification is
// fully offline here (IgnoreTlog/Offline), so an unsigned artifact reaches
// cosign's "no matching signatures" failure, which the operator wraps as
// "cosign verify ...: %w" into Module.status.lastError.
func TestModule_VerifyRejectsUnsignedBundle(t *testing.T) {
	parent := t
	ctx := context.Background()

	// Registry + unsigned bundle (idempotent; shared with the other Module
	// e2e tests, which run sequentially in the same cluster).
	envInstance.ApplyYAML(t, "oci-registry.yaml")
	envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
		dep, err := envInstance.K8s.AppsV1().Deployments("kestrel-system").
			Get(ctx, "kestrel-test-registry", metav1.GetOptions{})
		if err != nil {
			return false, "get registry deploy: " + err.Error()
		}
		if dep.Status.ReadyReplicas >= 1 {
			return true, ""
		}
		return false, "registry not ready yet"
	})
	envInstance.OCIPush(t, "kestrel-system", "oras-push-test-game")

	// Public key the operator verifies against (private half discarded).
	envInstance.ApplyYAML(t, "verify-cosign-pubkey.yaml")
	parent.Cleanup(func() {
		_ = envInstance.K8s.CoreV1().Secrets("kestrel-system").
			Delete(context.Background(), "e2e-cosign-pubkey", metav1.DeleteOptions{})
	})

	const (
		sourceName = "e2e-verify-source"
		moduleCR   = "e2e-verify-module"
	)

	// A verify-enabled OCI source. Indexing only pulls metadata (it does not
	// verify), so the catalog still populates; verification gates install.
	src := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.gg/v1alpha1",
		"kind":       "ModuleSource",
		"metadata":   map[string]any{"name": sourceName},
		"spec": map[string]any{
			"type": "oci",
			"oci": map[string]any{
				"url":      "kestrel-test-registry.kestrel-system.svc:5000",
				"insecure": true,
				"modules":  []any{map[string]any{"name": "e2e-test-game"}},
			},
			"verify":          map[string]any{"key": map[string]any{"name": "e2e-cosign-pubkey"}},
			"refreshInterval": "10m",
		},
	}}
	if _, err := envInstance.Dyn.Resource(moduleSourceGVR).
		Create(ctx, src, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create verify modulesource: %v", err)
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

	// Installing must fail at the signature gate.
	mod := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.gg/v1alpha1",
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
		t.Fatalf("create verify module: %v", err)
	}
	parent.Cleanup(func() {
		_ = envInstance.Dyn.Resource(moduleGVR).
			Delete(context.Background(), moduleCR, metav1.DeleteOptions{})
	})

	envInstance.Eventually(t, 2*time.Minute, func() (bool, string) {
		got, err := envInstance.Dyn.Resource(moduleGVR).Get(ctx, moduleCR, metav1.GetOptions{})
		if err != nil {
			return false, "get module: " + err.Error()
		}
		phase, _, _ := unstructured.NestedString(got.Object, "status", "phase")
		if phase != "Failed" {
			return false, "module phase=" + phase + " (want Failed)"
		}
		lastErr, _, _ := unstructured.NestedString(got.Object, "status", "lastError")
		// Operator wraps the cosign error as "cosign verify <ref>@<digest>: ...".
		if !strings.Contains(lastErr, "cosign verify") {
			return false, "lastError lacks cosign verify: " + lastErr
		}
		return true, ""
	})

	// A rejected bundle must NOT materialize a GameTemplate.
	if _, err := envInstance.Dyn.Resource(gameTemplateGVR).Get(ctx, moduleCR, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected no GameTemplate %q for a rejected bundle, get err=%v", moduleCR, err)
	}
}
