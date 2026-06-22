//go:build envtest

package controller

import (
	"context"
	"sync"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

// fakeStopper records calls to the agent's soft-stop endpoint.
type fakeStopper struct {
	mu    sync.Mutex
	calls int
}

func (f *fakeStopper) Stop(_ context.Context, _, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return nil
}

func (f *fakeStopper) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func withGameServerReconcilerStopper(t *testing.T, ns string, stop AgentStopper) setupReconciler {
	t.Helper()
	seedAgentCA(t, ns, "agent-ca")
	return func(mgr manager.Manager) error {
		return (&GameServerReconciler{
			Client:                 mgr.GetClient(),
			Scheme:                 mgr.GetScheme(),
			AgentImage:             "ghcr.io/valgulnecron/gameplane/agent:test",
			AgentCASecretName:      "agent-ca",
			AgentCASecretNamespace: ns,
			AgentClient:            stop,
		}).SetupWithManager(mgr)
	}
}

// setSSReady fakes a running (or stopped) game in envtest, which has no kubelet
// to populate StatefulSet status.
func setSSReady(t *testing.T, ns, name string, ready int32) {
	t.Helper()
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: name}, &ss); err != nil {
			return err
		}
		ss.Status.Replicas = ready
		ss.Status.ReadyReplicas = ready
		return k8sClient.Status().Update(context.Background(), &ss)
	}); err != nil {
		t.Fatalf("set ss ready=%d: %v", ready, err)
	}
}

func ssReplicas(t *testing.T, ns, name string) int32 {
	t.Helper()
	var ss appsv1.StatefulSet
	if err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: name}, &ss); err != nil {
		// Used inside eventually() polls, including before the reconciler has
		// created the StatefulSet — report "not there" rather than failing.
		return -1
	}
	if ss.Spec.Replicas == nil {
		return -1
	}
	return *ss.Spec.Replicas
}

// TestGameServer_SoftStopRunsStopThenScalesDown verifies the soft-stop path:
// suspending a server whose template declares a stop sequence issues that
// sequence over the agent and keeps the pod up while the game saves, then
// scales to zero once the game goes not-ready.
func TestGameServer_SoftStopRunsStopThenScalesDown(t *testing.T) {
	ns := newNamespace(t)
	stopper := &fakeStopper{}
	startMgr(t, ns, withGameServerReconcilerStopper(t, ns, stopper))

	tmpl := buildGameTemplate(uniqueName("minecraft"))
	if tmpl.Spec.Capabilities == nil {
		tmpl.Spec.Capabilities = &gameplanev1alpha1.CapabilitiesSpec{}
	}
	tmpl.Spec.Capabilities.Lifecycle = &gameplanev1alpha1.LifecycleSpec{Stop: []string{"stop"}}
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "smp", tmpl.Name)
	// Long grace so the scale-down in this test is driven by the game going
	// not-ready, not by the backstop deadline firing mid-assertion.
	grace := int32(120)
	gs.Spec.StopGracePeriodSeconds = &grace
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	// Wait for the initial create to land replicas=1, then fake a running game.
	eventually(t, func() (bool, string) {
		return ssReplicas(t, ns, "smp") == 1, "replicas not yet 1"
	})
	setSSReady(t, ns, "smp", 1)

	// Suspend. The reconciler should issue the stop sequence and keep the pod
	// running while it waits for the game to go down.
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		gs = getGameServer(t, ns, "smp")
		gs.Spec.Suspend = true
		return k8sClient.Update(context.Background(), gs)
	}); err != nil {
		t.Fatalf("suspend: %v", err)
	}

	eventually(t, func() (bool, string) {
		if stopper.count() == 0 {
			return false, "stop sequence not issued yet"
		}
		g := getGameServer(t, ns, "smp")
		if _, ok := g.Annotations[stopRequestedAtAnnotation]; !ok {
			return false, "stop-requested annotation not set"
		}
		if r := ssReplicas(t, ns, "smp"); r != 1 {
			return false, "pod scaled down before the game went not-ready"
		}
		return true, ""
	})

	// The game finishes saving and goes not-ready: the operator scales to zero.
	setSSReady(t, ns, "smp", 0)
	eventuallyWith(t, 15*time.Second, func() (bool, string) {
		return ssReplicas(t, ns, "smp") == 0, "replicas not yet zero"
	})

	// Resuming clears the soft-stop bookkeeping and scales back up.
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		gs = getGameServer(t, ns, "smp")
		gs.Spec.Suspend = false
		return k8sClient.Update(context.Background(), gs)
	}); err != nil {
		t.Fatalf("resume: %v", err)
	}
	eventually(t, func() (bool, string) {
		g := getGameServer(t, ns, "smp")
		if _, ok := g.Annotations[stopRequestedAtAnnotation]; ok {
			return false, "stop annotation not cleared on resume"
		}
		return ssReplicas(t, ns, "smp") == 1, "not scaled back to 1"
	})
}
