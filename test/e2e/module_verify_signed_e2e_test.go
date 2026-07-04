//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestModule_VerifySignedBundleInstalls is the positive counterpart to
// TestModule_VerifyRejectsUnsignedBundle: it proves the operator's keyed,
// offline cosign verification ACCEPTS a bundle that carries a valid signature,
// installs it, and materializes the GameTemplate.
//
// The signature is produced in-cluster by a cosign Job (keyed, no Rekor) using
// a key pair cosign writes straight into the e2e-sign-keypair Secret — whose
// cosign.pub the operator then verifies against via ModuleSource.spec.verify.
// This exercises the real sign->verify seam end to end, which the operator's
// envtest suite only covers with a fake verifier and which the release pipeline
// (modules/build.sh --sign) relies on.
func TestModule_VerifySignedBundleInstalls(t *testing.T) {
	t.Parallel()
	// Shares the fixed-name oras-push Job with the other module tests and
	// additionally owns the cosign keypair Secret + sign Job — serialize
	// against the other module tests (see ociPushMu).
	ociPushMu.Lock()
	defer ociPushMu.Unlock()

	parent := t
	ctx := context.Background()

	// Registry + unsigned bundle first (idempotent; shared with the other
	// Module e2e tests, which run sequentially in the same cluster).
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

	// Sign the pushed bundle in-cluster. Clear any prior keypair Secret + Job
	// first so cosign generate-key-pair starts clean (it errors on an existing
	// Secret, and the Job runs with backoffLimit 0 / restartPolicy Never).
	bg := metav1.DeletePropagationBackground
	_ = envInstance.K8s.BatchV1().Jobs("gameplane-system").
		Delete(ctx, "cosign-sign-test-game", metav1.DeleteOptions{PropagationPolicy: &bg})
	_ = envInstance.K8s.CoreV1().Secrets("gameplane-system").
		Delete(ctx, "e2e-sign-keypair", metav1.DeleteOptions{})
	envInstance.ApplyYAML(t, "cosign-sign-job.yaml")
	parent.Cleanup(func() {
		c := context.Background()
		_ = envInstance.K8s.BatchV1().Jobs("gameplane-system").
			Delete(c, "cosign-sign-test-game", metav1.DeleteOptions{PropagationPolicy: &bg})
		_ = envInstance.K8s.CoreV1().Secrets("gameplane-system").
			Delete(c, "e2e-sign-keypair", metav1.DeleteOptions{})
	})
	envInstance.Eventually(t, 2*time.Minute, func() (bool, string) {
		j, err := envInstance.K8s.BatchV1().Jobs("gameplane-system").
			Get(ctx, "cosign-sign-test-game", metav1.GetOptions{})
		if err != nil {
			return false, "get sign job: " + err.Error()
		}
		if j.Status.Succeeded > 0 {
			return true, ""
		}
		if j.Status.Failed > 0 {
			out, _ := envInstance.Kubectl("logs", "-n", "gameplane-system",
				"job/cosign-sign-test-game", "--all-containers", "--tail=200")
			return false, "sign job failed:\n" + out
		}
		return false, "sign job not done yet"
	})

	const (
		sourceName = "e2e-verify-signed-source"
		moduleCR   = "e2e-verify-signed-module"
	)

	// A verify-enabled OCI source keyed to the cosign-generated public key
	// (the keypair Secret exposes cosign.pub, which is what the operator reads).
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
			"verify":          map[string]any{"key": map[string]any{"name": "e2e-sign-keypair"}},
			"refreshInterval": "10m",
		},
	}}
	if _, err := envInstance.Dyn.Resource(moduleSourceGVR).
		Create(ctx, src, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create signed-verify modulesource: %v", err)
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

	// Installing must pass the signature gate and reach Ready.
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
		t.Fatalf("create signed-verify module: %v", err)
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
		if phase != "Ready" {
			lastErr, _, _ := unstructured.NestedString(got.Object, "status", "lastError")
			return false, "module phase=" + phase + " lastError=" + lastErr
		}
		return true, ""
	})

	// A verified bundle materializes its GameTemplate (named after the Module).
	tmpl, err := envInstance.Dyn.Resource(gameTemplateGVR).Get(ctx, moduleCR, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get materialized template for signed bundle: %v", err)
	}
	if image, _, _ := unstructured.NestedString(tmpl.Object, "spec", "image"); image != "busybox:1.36" {
		t.Errorf("template.spec.image=%q want busybox:1.36", image)
	}
}
