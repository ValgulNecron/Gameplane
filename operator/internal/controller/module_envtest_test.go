//go:build envtest

package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	kestrelv1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
	"github.com/ValgulNecron/gameplane/operator/internal/verify"
)

// fakeVerifier stands in for the cosign verifier in envtests.
type fakeVerifier struct{ err error }

func (f fakeVerifier) Verify(context.Context, string, string) error { return f.err }

func withModuleReconcilerVerifier(fake *fakeOCI, v verify.Verifier) setupReconciler {
	return func(mgr manager.Manager) error {
		return (&ModuleReconciler{
			Client:     mgr.GetClient(),
			Scheme:     mgr.GetScheme(),
			NewFetcher: fakeOCIFetcher(fake),
			NewVerifier: func(context.Context, *kestrelv1alpha1.ModuleSource) (verify.Verifier, error) {
				return v, nil
			},
		}).SetupWithManager(mgr)
	}
}

func withModuleReconciler(fake *fakeOCI) setupReconciler {
	return func(mgr manager.Manager) error {
		return (&ModuleReconciler{
			Client:     mgr.GetClient(),
			Scheme:     mgr.GetScheme(),
			NewFetcher: fakeOCIFetcher(fake),
		}).SetupWithManager(mgr)
	}
}

func withModuleReconcilerVersion(fake *fakeOCI, version string) setupReconciler {
	return func(mgr manager.Manager) error {
		return (&ModuleReconciler{
			Client:          mgr.GetClient(),
			Scheme:          mgr.GetScheme(),
			OperatorVersion: version,
			NewFetcher:      fakeOCIFetcher(fake),
		}).SetupWithManager(mgr)
	}
}

// createIndexedSource is a test helper: creates a ModuleSource and
// directly seeds its status.modules so the Module reconciler doesn't
// have to wait for a separate ModuleSourceReconciler in this test.
func createIndexedSource(t *testing.T, name, urlPrefix string, _ *fakeOCI, modules []kestrelv1alpha1.ModuleEntry) *kestrelv1alpha1.ModuleSource {
	t.Helper()
	src := &kestrelv1alpha1.ModuleSource{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: kestrelv1alpha1.ModuleSourceSpec{
			Type: kestrelv1alpha1.ModuleSourceTypeOCI,
			OCI: &kestrelv1alpha1.OCISourceSpec{
				URL:     urlPrefix,
				Modules: []kestrelv1alpha1.ModuleRef{},
			},
		},
	}
	for _, e := range modules {
		src.Spec.OCI.Modules = append(src.Spec.OCI.Modules, kestrelv1alpha1.ModuleRef{Name: e.Name})
	}
	if err := k8sClient.Create(context.Background(), src); err != nil {
		t.Fatalf("create modulesource: %v", err)
	}
	deleteCleanup(t, src)

	// Re-fetch to get a UID/RV.
	if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: name}, src); err != nil {
		t.Fatalf("re-get modulesource: %v", err)
	}
	src.Status.Modules = modules
	src.Status.Conditions = []metav1.Condition{{
		Type:               kestrelv1alpha1.ModuleSourceConditionSynced,
		Status:             metav1.ConditionTrue,
		Reason:             "TestSeeded",
		LastTransitionTime: metav1.Now(),
	}}
	if err := k8sClient.Status().Update(context.Background(), src); err != nil {
		t.Fatalf("seed status: %v", err)
	}
	return src
}

// TestModule_PullsAndMaterializesGameTemplate — happy path: a Module
// referencing an indexed source pulls the bundle, creates a labeled
// GameTemplate, and reaches Phase=Ready.
func TestModule_PullsAndMaterializesGameTemplate(t *testing.T) {
	_ = newNamespace(t)
	fake := newFakeOCI()
	startMgr(t, "kestrel-system", withModuleReconciler(fake))

	const ref = "local/test/minecraft-java"
	fake.putBundle(ref, "1.0.0", fixtureBundle("minecraft-java", "1.0.0", "Minecraft (Java)"))

	srcName := uniqueName("modsrc")
	createIndexedSource(t, srcName, "local/test", fake, []kestrelv1alpha1.ModuleEntry{{
		Name:          "minecraft-java",
		DisplayName:   "Minecraft (Java)",
		Reference:     ref,
		Versions:      []string{"1.0.0"},
		LatestVersion: "1.0.0",
	}})

	modName := uniqueName("mod")
	mod := &kestrelv1alpha1.Module{
		ObjectMeta: metav1.ObjectMeta{Name: modName},
		Spec: kestrelv1alpha1.ModuleSpec{
			Source: corev1.LocalObjectReference{Name: srcName},
			Name:   "minecraft-java",
		},
	}
	if err := k8sClient.Create(context.Background(), mod); err != nil {
		t.Fatalf("create module: %v", err)
	}
	deleteCleanup(t, mod)

	eventually(t, func() (bool, string) {
		got := getModule(t, modName)
		if got.Status.Phase != kestrelv1alpha1.ModulePhaseReady {
			return false, "phase=" + got.Status.Phase + " err=" + got.Status.LastError
		}
		if got.Status.AppliedVersion != "1.0.0" {
			return false, "appliedVersion=" + got.Status.AppliedVersion
		}
		return true, ""
	})

	// GameTemplate should exist with management labels.
	tmpl := getTemplateByName(t, modName)
	if tmpl.Labels[kestrelv1alpha1.LabelManagedBy] != kestrelv1alpha1.ManagedByModule {
		t.Errorf("template missing managed-by label: %v", tmpl.Labels)
	}
	if tmpl.Labels[kestrelv1alpha1.LabelModuleVersion] != "1.0.0" {
		t.Errorf("template module-version = %q", tmpl.Labels[kestrelv1alpha1.LabelModuleVersion])
	}
	if tmpl.Spec.Game != "minecraft-java" {
		t.Errorf("template spec.game = %q", tmpl.Spec.Game)
	}
}

// TestModule_DeletionBlockedByGameServer — a Module with an
// in-use template stays Failed/InUse until the GameServer is removed.
func TestModule_DeletionBlockedByGameServer(t *testing.T) {
	ns := newNamespace(t)
	fake := newFakeOCI()
	startMgr(t, ns, withModuleReconciler(fake))

	const ref = "local/test/valheim"
	fake.putBundle(ref, "0.9.0", fixtureBundle("valheim", "0.9.0", "Valheim"))

	srcName := uniqueName("modsrc")
	createIndexedSource(t, srcName, "local/test", fake, []kestrelv1alpha1.ModuleEntry{{
		Name:          "valheim",
		Reference:     ref,
		Versions:      []string{"0.9.0"},
		LatestVersion: "0.9.0",
	}})

	modName := uniqueName("vh")
	mod := &kestrelv1alpha1.Module{
		ObjectMeta: metav1.ObjectMeta{Name: modName},
		Spec: kestrelv1alpha1.ModuleSpec{
			Source: corev1.LocalObjectReference{Name: srcName},
			Name:   "valheim",
		},
	}
	if err := k8sClient.Create(context.Background(), mod); err != nil {
		t.Fatalf("create module: %v", err)
	}

	// Wait for materialization.
	eventually(t, func() (bool, string) {
		got := getModule(t, modName)
		return got.Status.Phase == kestrelv1alpha1.ModulePhaseReady, "phase=" + got.Status.Phase
	})

	// Create a GameServer referencing the materialized template.
	gs := buildGameServer(ns, "smp", modName)
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gs: %v", err)
	}

	// Initiate Module delete.
	if err := k8sClient.Delete(context.Background(), mod); err != nil {
		t.Fatalf("delete module: %v", err)
	}

	// Module should NOT disappear — finalizer blocks while gs in use.
	consistently(t, 2*time.Second, func() (bool, string) {
		var still kestrelv1alpha1.Module
		err := k8sClient.Get(context.Background(), types.NamespacedName{Name: modName}, &still)
		if err != nil {
			return false, fmt.Sprintf("Module gone too early: %v", err)
		}
		return true, ""
	})

	// Now delete the GameServer; the finalizer should release.
	if err := k8sClient.Delete(context.Background(), gs); err != nil {
		t.Fatalf("delete gs: %v", err)
	}
	eventually(t, func() (bool, string) {
		var still kestrelv1alpha1.Module
		err := k8sClient.Get(context.Background(), types.NamespacedName{Name: modName}, &still)
		if apierrors.IsNotFound(err) {
			return true, ""
		}
		return false, fmt.Sprintf("module still present: phase=%s err=%s",
			still.Status.Phase, still.Status.LastError)
	})

	// And the template should have been deleted along with it.
	var tmpl kestrelv1alpha1.GameTemplate
	err := k8sClient.Get(context.Background(), types.NamespacedName{Name: modName}, &tmpl)
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected GameTemplate deleted; got err=%v", err)
	}
}

func getModule(t *testing.T, name string) *kestrelv1alpha1.Module {
	t.Helper()
	var m kestrelv1alpha1.Module
	if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: name}, &m); err != nil {
		t.Fatalf("get module: %v", err)
	}
	return &m
}
