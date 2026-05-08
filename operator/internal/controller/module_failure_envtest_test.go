//go:build envtest

package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
)

// TestModule_SourceNotFound — referencing a non-existent ModuleSource
// pushes the Module to Phase=Failed (markFailed branch).
func TestModule_SourceNotFound(t *testing.T) {
	_ = newNamespace(t)
	fake := newFakeOCI()
	startMgr(t, "kestrel-system", withModuleReconciler(fake))

	modName := uniqueName("mod-no-src")
	mod := &kestrelv1alpha1.Module{
		ObjectMeta: metav1.ObjectMeta{Name: modName},
		Spec: kestrelv1alpha1.ModuleSpec{
			Source: corev1.LocalObjectReference{Name: "ghost-source"},
			Name:   "minecraft-java",
		},
	}
	if err := k8sClient.Create(context.Background(), mod); err != nil {
		t.Fatalf("create module: %v", err)
	}
	deleteCleanup(t, mod)

	eventually(t, func() (bool, string) {
		got := getModule(t, modName)
		if got.Status.Phase != kestrelv1alpha1.ModulePhaseFailed {
			return false, "phase=" + got.Status.Phase
		}
		return true, ""
	})
}

// TestModule_VersionNotInCatalog — pinning a version the source doesn't
// publish must end in Phase=Failed (markFailed for VersionUnavailable).
func TestModule_VersionNotInCatalog(t *testing.T) {
	_ = newNamespace(t)
	fake := newFakeOCI()
	startMgr(t, "kestrel-system", withModuleReconciler(fake))

	const ref = "local/test/minecraft-java"
	srcName := uniqueName("modsrc")
	createIndexedSource(t, srcName, "local/test", fake, []kestrelv1alpha1.ModuleEntry{{
		Name:          "minecraft-java",
		Reference:     ref,
		Versions:      []string{"1.0.0"},
		LatestVersion: "1.0.0",
	}})

	modName := uniqueName("mod-bad-ver")
	mod := &kestrelv1alpha1.Module{
		ObjectMeta: metav1.ObjectMeta{Name: modName},
		Spec: kestrelv1alpha1.ModuleSpec{
			Source:  corev1.LocalObjectReference{Name: srcName},
			Name:    "minecraft-java",
			Version: "9.9.9",
		},
	}
	if err := k8sClient.Create(context.Background(), mod); err != nil {
		t.Fatalf("create module: %v", err)
	}
	deleteCleanup(t, mod)

	eventually(t, func() (bool, string) {
		got := getModule(t, modName)
		if got.Status.Phase != kestrelv1alpha1.ModulePhaseFailed {
			return false, "phase=" + got.Status.Phase
		}
		return true, ""
	})
}

// TestModule_WaitingForCatalog — the source exists but doesn't (yet)
// list the requested module, so the reconciler markPending and waits.
func TestModule_WaitingForCatalog(t *testing.T) {
	_ = newNamespace(t)
	fake := newFakeOCI()
	startMgr(t, "kestrel-system", withModuleReconciler(fake))

	srcName := uniqueName("modsrc-empty")
	// Source's catalog has only an unrelated module — the one the
	// Module spec requests below is not (yet) indexed.
	createIndexedSource(t, srcName, "local/test", fake, []kestrelv1alpha1.ModuleEntry{{
		Name:          "other",
		Reference:     "local/test/other",
		Versions:      []string{"1.0.0"},
		LatestVersion: "1.0.0",
	}})

	modName := uniqueName("mod-pending")
	mod := &kestrelv1alpha1.Module{
		ObjectMeta: metav1.ObjectMeta{Name: modName},
		Spec: kestrelv1alpha1.ModuleSpec{
			Source: corev1.LocalObjectReference{Name: srcName},
			Name:   "not-yet-indexed",
		},
	}
	if err := k8sClient.Create(context.Background(), mod); err != nil {
		t.Fatalf("create module: %v", err)
	}
	deleteCleanup(t, mod)

	eventually(t, func() (bool, string) {
		got := getModule(t, modName)
		if got.Status.Phase != kestrelv1alpha1.ModulePhasePending {
			return false, "phase=" + got.Status.Phase
		}
		return true, ""
	})
}
