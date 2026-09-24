//go:build envtest

package controller

import (
	"context"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

// seedMC catalogs and stocks one "mc" 1.0.0 bundle, returning the source name.
func seedMC(t *testing.T, fake *fakeOCI) (srcName, ref string) {
	t.Helper()
	ref = "local/test/mc"
	fake.putBundle(ref, "1.0.0", fixtureBundle("mc", "1.0.0", "MC"))
	srcName = uniqueName("modsrc")
	createIndexedSource(t, srcName, "local/test", fake, []gameplanev1alpha1.ModuleEntry{{
		Name:          "mc",
		Reference:     ref,
		Versions:      []string{"1.0.0"},
		LatestVersion: "1.0.0",
	}})
	return srcName, ref
}

func createModule(t *testing.T, name, srcName string, mutate func(*gameplanev1alpha1.Module)) {
	t.Helper()
	mod := &gameplanev1alpha1.Module{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: gameplanev1alpha1.ModuleSpec{
			Source: corev1.LocalObjectReference{Name: srcName},
			Name:   "mc",
		},
	}
	if mutate != nil {
		mutate(mod)
	}
	if err := k8sClient.Create(context.Background(), mod); err != nil {
		t.Fatalf("create module: %v", err)
	}
	deleteCleanup(t, mod)
}

func expectModulePhase(t *testing.T, name, phase, lastErrContains string) {
	t.Helper()
	eventually(t, func() (bool, string) {
		got := getModule(t, name)
		if got.Status.Phase != phase {
			return false, "phase=" + got.Status.Phase
		}
		if lastErrContains != "" && !strings.Contains(got.Status.LastError, lastErrContains) {
			return false, "lastError=" + got.Status.LastError
		}
		return true, ""
	})
}

// TestModule_SignatureInvalid — a verifier that rejects the bundle pushes the
// Module to Failed and materializes no GameTemplate.
func TestModule_SignatureInvalid(t *testing.T) {
	_ = newNamespace(t)
	fake := newFakeOCI()
	startMgr(t, "gameplane-system", withModuleReconcilerVerifier(fake, fakeVerifier{err: fmt.Errorf("no matching signatures")}))

	srcName, _ := seedMC(t, fake)
	modName := uniqueName("mod-unsigned")
	createModule(t, modName, srcName, nil)

	expectModulePhase(t, modName, gameplanev1alpha1.ModulePhaseFailed, "no matching signatures")

	var tmpl gameplanev1alpha1.GameTemplate
	if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: modName}, &tmpl); err == nil {
		t.Fatal("GameTemplate should not exist for an unsigned module")
	}
}

// TestModule_SignatureValid — a passing verifier lets the install proceed.
func TestModule_SignatureValid(t *testing.T) {
	_ = newNamespace(t)
	fake := newFakeOCI()
	startMgr(t, "gameplane-system", withModuleReconcilerVerifier(fake, fakeVerifier{}))

	srcName, _ := seedMC(t, fake)
	modName := uniqueName("mod-signed")
	createModule(t, modName, srcName, nil)

	expectModulePhase(t, modName, gameplanev1alpha1.ModulePhaseReady, "")
}

// TestModule_DigestPinMismatch — a spec.digest that doesn't match the
// resolved bundle fails the install.
func TestModule_DigestPinMismatch(t *testing.T) {
	_ = newNamespace(t)
	fake := newFakeOCI()
	startMgr(t, "gameplane-system", withModuleReconciler(fake))

	srcName, _ := seedMC(t, fake)
	modName := uniqueName("mod-badpin")
	createModule(t, modName, srcName, func(m *gameplanev1alpha1.Module) {
		m.Spec.Digest = "sha256:wrong"
	})

	expectModulePhase(t, modName, gameplanev1alpha1.ModulePhaseFailed, "pinned digest")
}

// TestModule_DigestPinMatch — a correct spec.digest installs cleanly.
func TestModule_DigestPinMatch(t *testing.T) {
	_ = newNamespace(t)
	fake := newFakeOCI()
	startMgr(t, "gameplane-system", withModuleReconciler(fake))

	srcName, _ := seedMC(t, fake)
	modName := uniqueName("mod-goodpin")
	createModule(t, modName, srcName, func(m *gameplanev1alpha1.Module) {
		// fixtureBundle stamps digest "sha256:<name>-<version>".
		m.Spec.Digest = "sha256:mc-1.0.0"
	})

	expectModulePhase(t, modName, gameplanev1alpha1.ModulePhaseReady, "")
}

// patchModuleDigest sets a Module's spec.digest, retrying on conflict
// because the reconciler updates status concurrently.
func patchModuleDigest(t *testing.T, name, digest string) {
	t.Helper()
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var mod gameplanev1alpha1.Module
		if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: name}, &mod); err != nil {
			return err
		}
		mod.Spec.Digest = digest
		return k8sClient.Update(context.Background(), &mod)
	}); err != nil {
		t.Fatalf("patch module digest: %v", err)
	}
}

// TestModule_DigestPinCheckedOnReadyModule — a spec.digest changed on a
// Ready Module is checked against the applied bundle, and restoring the
// matching pin brings the Module back to Ready.
func TestModule_DigestPinCheckedOnReadyModule(t *testing.T) {
	_ = newNamespace(t)
	fake := newFakeOCI()
	startMgr(t, "gameplane-system", withModuleReconciler(fake))

	srcName, _ := seedMC(t, fake)
	modName := uniqueName("mod-repin")
	createModule(t, modName, srcName, func(m *gameplanev1alpha1.Module) {
		// fixtureBundle stamps digest "sha256:<name>-<version>".
		m.Spec.Digest = "sha256:mc-1.0.0"
	})
	expectModulePhase(t, modName, gameplanev1alpha1.ModulePhaseReady, "")

	// Same version, different pin: the Module must leave Ready and
	// report the pin check's reason for the new generation.
	patchModuleDigest(t, modName, "sha256:other")
	eventually(t, func() (bool, string) {
		got := getModule(t, modName)
		if got.Status.Phase != gameplanev1alpha1.ModulePhaseFailed {
			return false, "phase=" + got.Status.Phase
		}
		if got.Status.ObservedGeneration != got.Generation {
			return false, fmt.Sprintf("observedGeneration=%d generation=%d",
				got.Status.ObservedGeneration, got.Generation)
		}
		for _, c := range got.Status.Conditions {
			if c.Type == gameplanev1alpha1.ModuleConditionReady {
				if c.Reason != "DigestMismatch" {
					return false, "Ready reason=" + c.Reason
				}
				return true, ""
			}
		}
		return false, "no Ready condition"
	})

	// Restoring the matching pin reconciles back to Ready.
	patchModuleDigest(t, modName, "sha256:mc-1.0.0")
	expectModulePhase(t, modName, gameplanev1alpha1.ModulePhaseReady, "")
}
