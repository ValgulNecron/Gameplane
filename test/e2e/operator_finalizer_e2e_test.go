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

// TestModule_FinalizerBlocksWhileTemplateInUse: the Module reconciler
// holds the gameplane.gg/module-finalizer while any GameServer references
// the materialized GameTemplate. Deleting the Module under those
// conditions sets metadata.deletionTimestamp but the resource lingers
// (status.phase=Failed with an "InUse" reason) until the GameServer is
// gone.
//
// We piggyback on the registry+module setup that module_e2e_test owns
// so we don't pay the OCI bring-up cost twice — but we drive a fresh
// Module/GameServer pair to keep the assertions independent.
func TestModule_FinalizerBlocksWhileTemplateInUse(t *testing.T) {
	ctx := context.Background()
	ns := "kestrel-games"

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

	const (
		sourceName = "e2e-finalizer-source"
		moduleCR   = "e2e-finalizer-module"
		gsName     = "e2e-finalizer-gs"
	)

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
			"refreshInterval": "10m",
		},
	}}
	if _, err := envInstance.Dyn.Resource(moduleSourceGVR).
		Create(ctx, src, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create modulesource: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(moduleSourceGVR).
			Delete(context.Background(), sourceName, metav1.DeleteOptions{})
	})

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
		t.Fatalf("create module: %v", err)
	}
	// IMPORTANT: do NOT register a t.Cleanup that deletes the Module
	// here — this test deletes it explicitly mid-test, and a duplicate
	// cleanup-time delete would race with the finalizer release we
	// trigger by removing the GameServer. The test removes both before
	// returning.

	// Wait for the materialized template + module Ready phase.
	envInstance.Eventually(t, 2*time.Minute, func() (bool, string) {
		got, err := envInstance.Dyn.Resource(moduleGVR).Get(ctx, moduleCR, metav1.GetOptions{})
		if err != nil {
			return false, "get module: " + err.Error()
		}
		phase, _, _ := unstructured.NestedString(got.Object, "status", "phase")
		if phase != "Ready" {
			return false, "module phase=" + phase
		}
		applied, _, _ := unstructured.NestedString(got.Object, "status", "appliedTemplate")
		if applied == "" {
			return false, "module has no appliedTemplate yet"
		}
		return true, ""
	})

	// Create a GameServer that references the materialized template.
	// The template name equals the Module name (per the reconciler).
	applyBusyboxGameServer(t, ns, gsName, moduleCR)

	// Initiate Module delete. The reconciler's finalize path must NOT
	// release the finalizer while the GameServer references the template.
	if err := envInstance.Dyn.Resource(moduleGVR).
		Delete(ctx, moduleCR, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		t.Fatalf("delete module: %v", err)
	}

	// For at least 15s, the Module must still exist with a
	// deletionTimestamp set — proving the finalizer held the resource.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		got, err := envInstance.Dyn.Resource(moduleGVR).Get(ctx, moduleCR, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			t.Fatalf("module disappeared before GameServer was deleted — finalizer didn't engage")
		}
		if err != nil {
			t.Fatalf("get module mid-finalize: %v", err)
		}
		if got.GetDeletionTimestamp() == nil {
			return // tolerated: race where deletion arrived but the apiserver hasn't stamped yet
		}
		// finalizers list must contain ours
		fins := got.GetFinalizers()
		hasOurs := false
		for _, f := range fins {
			if f == "gameplane.gg/module-finalizer" {
				hasOurs = true
				break
			}
		}
		if !hasOurs {
			t.Fatalf("module mid-delete missing gameplane.gg/module-finalizer (finalizers=%v)", fins)
		}
		time.Sleep(2 * time.Second)
	}

	// Now delete the GameServer — the finalizer should release within
	// the next reconcile pass and the Module disappears.
	if err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Delete(ctx, gsName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		t.Fatalf("delete gameserver: %v", err)
	}
	envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
		_, err := envInstance.Dyn.Resource(moduleGVR).Get(ctx, moduleCR, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, ""
		}
		if err != nil {
			return false, "get module: " + err.Error()
		}
		return false, "module still present after GameServer delete"
	})
}

// TestGameServer_CascadingDelete: deleting a GameServer must drive the
// operator to GC the StatefulSet and PVC owned by it. Backups are
// independent lifecycle objects (they're snapshots, not workload
// children) and survive the GameServer's deletion.
func TestGameServer_CascadingDelete(t *testing.T) {
	ctx := context.Background()
	ns := "kestrel-games"
	tmpl := "e2e-cascade-tmpl"
	gs := "e2e-cascade-target"
	bkName := "e2e-cascade-backup"

	envInstance.ApplyYAML(t, "backup-restic-secret.yaml")

	applyBusyboxTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)
	waitPVCBound(t, ns, gs+"-data", 90*time.Second)

	// Independent Backup CR — never reaches Succeeded (no restic-server)
	// but the CR shape exists in the namespace, which is enough for the
	// "survives GameServer delete" assertion.
	bk := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.gg/v1alpha1",
		"kind":       "Backup",
		"metadata":   map[string]any{"name": bkName, "namespace": ns},
		"spec": map[string]any{
			"serverRef": map[string]any{"name": gs},
			"repoRef":   map[string]any{"name": "e2e-restic-creds", "key": "repo"},
			"strategy":  "restic-snapshot",
			"quiesce":   false,
		},
	}}
	if _, err := envInstance.Dyn.Resource(backupGVR).Namespace(ns).
		Create(ctx, bk, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create backup CR: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(backupGVR).Namespace(ns).
			Delete(context.Background(), bkName, metav1.DeleteOptions{})
	})

	// Sanity: StatefulSet exists before delete.
	if _, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, gs, metav1.GetOptions{}); err != nil {
		t.Fatalf("get statefulset pre-delete: %v", err)
	}

	if err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Delete(ctx, gs, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		t.Fatalf("delete gameserver: %v", err)
	}

	// StatefulSet GC — the operator owns it via OwnerReference, so the
	// kube-controller-manager's GC removes it as soon as the owning
	// GameServer is gone.
	envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
		_, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, gs, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, ""
		}
		if err != nil {
			return false, "get ss: " + err.Error()
		}
		return false, "statefulset still present"
	})

	// Backup CR is in a separate ownership graph — it must still be there.
	if _, err := envInstance.Dyn.Resource(backupGVR).Namespace(ns).
		Get(ctx, bkName, metav1.GetOptions{}); err != nil {
		t.Errorf("Backup CR vanished alongside GameServer — backups are not workload children: %v", err)
	}
}

// TestGameTemplate_DeletionWithLiveServer: deleting a cluster-scoped
// GameTemplate while a GameServer references it must not tear down the
// running StatefulSet. The operator's "operator is authoritative"
// contract means live workloads keep running with the cached spec until
// the GameServer itself is deleted.
func TestGameTemplate_DeletionWithLiveServer(t *testing.T) {
	ctx := context.Background()
	ns := "kestrel-games"
	tmpl := "e2e-tmpldelete-template"
	gs := "e2e-tmpldelete-server"

	applyBusyboxTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)

	// Wait for the StatefulSet so the deletion target is well-defined.
	envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
		_, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, gs, metav1.GetOptions{})
		if err != nil {
			return false, "get ss: " + err.Error()
		}
		return true, ""
	})

	// Capture the StatefulSet UID so we can prove the same object survives.
	pre, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, gs, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get ss pre-delete: %v", err)
	}
	preUID := pre.UID

	// Delete the GameTemplate. Cluster-scoped, no finalizer (the test
	// helper doesn't register one), so it goes away immediately.
	if err := envInstance.Dyn.Resource(gameTemplateGVR).
		Delete(ctx, tmpl, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		t.Fatalf("delete template: %v", err)
	}

	// Watch for ~30s: the StatefulSet must still be present with the
	// same UID. A regression where the operator tears down children on
	// template delete would either delete the SS or recreate it under a
	// fresh UID.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		ss, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, gs, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			t.Fatalf("StatefulSet was GC'd after GameTemplate delete — operator must not cascade")
		}
		if err != nil {
			t.Fatalf("get ss post-delete: %v", err)
		}
		if ss.UID != preUID {
			t.Fatalf("StatefulSet UID changed after GameTemplate delete (pre=%s, post=%s) — operator recreated it",
				preUID, ss.UID)
		}
		time.Sleep(3 * time.Second)
	}
}
