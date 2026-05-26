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

// TestModuleSourceAndModule covers the whole bundle-discovery path
// end-to-end against an in-cluster registry:
//
//  1. Bring up a registry:2 pod inside the kestrel-system namespace.
//  2. Push a tiny module bundle (configmap + oras Job) at it.
//  3. Apply a ModuleSource pointing at the registry; expect the
//     indexer to populate status.modules with the pushed entry.
//  4. Apply a Module CR; expect the operator to materialize a
//     cluster-scoped GameTemplate with the published spec.
//
// The whole chain runs as one parent test with t.Run subtests so the
// expensive setup (registry + push) is paid once.
//
// Cleanup ordering: t.Cleanup registered inside a t.Run runs at the END
// of that subtest, NOT at the end of the parent. The Module subtest
// depends on the ModuleSource still existing when it reconciles, so we
// register cleanup for both on the parent test's t (`parent` aliased
// below) — that way the source survives across the subtest boundary
// and is cleaned up when the whole TestModuleSourceAndModule returns.
func TestModuleSourceAndModule(t *testing.T) {
	parent := t
	ctx := context.Background()

	// 1. Registry. Apply, then wait for the deployment to land at
	// least one Ready pod.
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

	// 2. Push. OCIPush manages the Job lifecycle (delete-then-apply
	// for idempotence, then wait for success).
	envInstance.OCIPush(t, "kestrel-system", "oras-push-test-game")

	const (
		sourceName = "e2e-test-source"
		moduleCR   = "e2e-test-game"
	)

	// 3. ModuleSource indexes the registry.
	t.Run("ModuleSourceIndexesFromOCIRegistry", func(t *testing.T) {
		src := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "kestrel.gg/v1alpha1",
			"kind":       "ModuleSource",
			"metadata":   map[string]any{"name": sourceName},
			"spec": map[string]any{
				"url":      "kestrel-test-registry.kestrel-system.svc:5000",
				"insecure": true,
				"modules":  []any{map[string]any{"name": "e2e-test-game"}},
				// Default refreshInterval is 1h; for tests we want a
				// fast first reconcile, which controller-runtime gives
				// us regardless of this value.
				"refreshInterval": "10m",
			},
		}}
		if _, err := envInstance.Dyn.Resource(moduleSourceGVR).
			Create(ctx, src, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			t.Fatalf("create modulesource: %v", err)
		}
		parent.Cleanup(func() {
			_ = envInstance.Dyn.Resource(moduleSourceGVR).
				Delete(context.Background(), sourceName, metav1.DeleteOptions{})
		})

		envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
			got, err := envInstance.Dyn.Resource(moduleSourceGVR).
				Get(ctx, sourceName, metav1.GetOptions{})
			if err != nil {
				return false, "get modulesource: " + err.Error()
			}
			modules, _, _ := unstructured.NestedSlice(got.Object, "status", "modules")
			if len(modules) == 0 {
				return false, "status.modules still empty"
			}
			// First (and only) entry's name should match what we pushed.
			m, ok := modules[0].(map[string]any)
			if !ok {
				return false, "status.modules[0] not a map"
			}
			if m["name"] != "e2e-test-game" {
				return false, "status.modules[0].name=" + asString(m["name"])
			}
			lastSync, _, _ := unstructured.NestedString(got.Object, "status", "lastSync")
			if lastSync == "" {
				return false, "status.lastSync still empty"
			}
			return true, ""
		})
	})

	// 4. Module → GameTemplate. Depends on subtest 3 having registered
	// the source's catalog. We don't share state via Cleanup ordering
	// (subtest cleanup runs at end of parent), so the source still
	// exists here.
	t.Run("ModuleMaterializesGameTemplate", func(t *testing.T) {
		mod := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "kestrel.gg/v1alpha1",
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
			t.Fatalf("create module: %v", err)
		}
		parent.Cleanup(func() {
			_ = envInstance.Dyn.Resource(moduleGVR).
				Delete(context.Background(), moduleCR, metav1.DeleteOptions{})
		})

		envInstance.Eventually(t, 2*time.Minute, func() (bool, string) {
			got, err := envInstance.Dyn.Resource(moduleGVR).
				Get(ctx, moduleCR, metav1.GetOptions{})
			if err != nil {
				return false, "get module: " + err.Error()
			}
			phase, _, _ := unstructured.NestedString(got.Object, "status", "phase")
			if phase != "Ready" {
				lastErr, _, _ := unstructured.NestedString(got.Object, "status", "lastError")
				return false, "module phase=" + phase + " lastError=" + lastErr
			}
			applied, _, _ := unstructured.NestedString(got.Object, "status", "appliedTemplate")
			if applied != moduleCR {
				return false, "appliedTemplate=" + applied + " want " + moduleCR
			}
			return true, ""
		})

		// Verify the GameTemplate the operator created has the spec
		// fields the bundle promised.
		tmpl, err := envInstance.Dyn.Resource(gameTemplateGVR).
			Get(ctx, moduleCR, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get materialized template: %v", err)
		}
		image, _, _ := unstructured.NestedString(tmpl.Object, "spec", "image")
		if image != "busybox:1.36" {
			t.Errorf("template.spec.image=%q want busybox:1.36", image)
		}
		// LabelManagedBy should be set to "Module" so the API can
		// distinguish module-managed templates from hand-applied ones.
		labels := tmpl.GetLabels()
		if labels["kestrel.gg/managed-by"] != "Module" {
			t.Errorf("template missing managed-by=Module label, got %q", labels["kestrel.gg/managed-by"])
		}

		// Sanity-check that the bundle's port + display metadata round-
		// tripped through the OCI artifact and into the live CR. A
		// regression in the bundle decoder (e.g. dropping fields not in
		// a minimal whitelist) would silently leave these empty.
		ports, _, _ := unstructured.NestedSlice(tmpl.Object, "spec", "ports")
		if len(ports) == 0 {
			t.Errorf("template.spec.ports empty — bundle decoder dropped ports?")
		}
		displayName, _, _ := unstructured.NestedString(tmpl.Object, "spec", "displayName")
		if displayName == "" {
			t.Errorf("template.spec.displayName empty — bundle decoder dropped displayName?")
		}
	})

	// 5. The materialized template should be usable as a real
	// GameTemplate — i.e. a GameServer referencing it must drive the
	// operator through its full materialize path (StatefulSet/Service/PVC)
	// just like a hand-applied template does. Without this assertion,
	// the previous subtest could pass even if the template were missing
	// the controller-required fields (image, command, etc.).
	t.Run("MaterializedTemplateIsUsable", func(t *testing.T) {
		ns := "kestrel-games"
		gs := "e2e-test-game-via-module"

		gsObj := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "kestrel.gg/v1alpha1",
			"kind":       "GameServer",
			"metadata":   map[string]any{"name": gs, "namespace": ns},
			"spec": map[string]any{
				"templateRef": map[string]any{"name": moduleCR},
			},
		}}
		if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Create(ctx, gsObj, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			t.Fatalf("create gameserver via module template: %v", err)
		}
		t.Cleanup(func() {
			_ = envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
				Delete(context.Background(), gs, metav1.DeleteOptions{})
		})

		envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
			ss, err := envInstance.K8s.AppsV1().StatefulSets(ns).
				Get(ctx, gs, metav1.GetOptions{})
			if err != nil {
				return false, "get ss: " + err.Error()
			}
			// Find the game container and assert it uses the bundle's
			// image. A regression where the operator falls back to a
			// default image would otherwise pass the materialize check.
			for _, c := range ss.Spec.Template.Spec.Containers {
				if c.Name == "game" {
					if c.Image == "" {
						return false, "game container has empty image"
					}
					if c.Image != "busybox:1.36" {
						return false, "game container image=" + c.Image + " want busybox:1.36"
					}
					return true, ""
				}
			}
			return false, "no 'game' container in StatefulSet pod template"
		})
	})
}

// asString formats an arbitrary value for error messages without
// pulling in fmt.Sprintf("%v", ...) at call sites — the test files
// already import fmt selectively.
func asString(v any) string {
	if v == nil {
		return "<nil>"
	}
	if s, ok := v.(string); ok {
		return s
	}
	return "<non-string>"
}
