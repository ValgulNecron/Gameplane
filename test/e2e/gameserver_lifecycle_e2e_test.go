//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// TestGameServer_SuspendScalesToZeroAndBack verifies that toggling
// spec.suspend on a GameServer drives the underlying StatefulSet's
// replicas to 0 (and back to 1). This is the lifecycle path the
// dashboard's start/stop verbs hit and the operator contract that
// turns a CRD edit into actual scheduling decisions.
//
// The check is on StatefulSet.Spec.Replicas — pod-level Ready isn't
// asserted because that requires the busybox image to land via image
// pull and the agent sidecar to come up, both of which are tested
// elsewhere and add minutes to this case.
func TestGameServer_SuspendScalesToZeroAndBack(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"
	tmpl := "e2e-lifecycle-busybox"
	gs := "e2e-lifecycle-suspend"

	applyBusyboxTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)

	// Wait for the operator to materialize the StatefulSet at all.
	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		_, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, gs, metav1.GetOptions{})
		if err != nil {
			return false, "ss not yet: " + err.Error()
		}
		return true, ""
	})

	// Suspend → replicas: 0
	suspendPatch := []byte(`{"spec":{"suspend":true}}`)
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Patch(ctx, gs, types.MergePatchType, suspendPatch, metav1.PatchOptions{}); err != nil {
		t.Fatalf("patch suspend=true: %v", err)
	}
	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		ss, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, gs, metav1.GetOptions{})
		if err != nil {
			return false, "get ss: " + err.Error()
		}
		if ss.Spec.Replicas != nil && *ss.Spec.Replicas == 0 {
			return true, ""
		}
		return false, fmt.Sprintf("expected replicas=0, got %v", ss.Spec.Replicas)
	})

	// Unsuspend → replicas: 1
	unsuspendPatch := []byte(`{"spec":{"suspend":false}}`)
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Patch(ctx, gs, types.MergePatchType, unsuspendPatch, metav1.PatchOptions{}); err != nil {
		t.Fatalf("patch suspend=false: %v", err)
	}
	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		ss, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, gs, metav1.GetOptions{})
		if err != nil {
			return false, "get ss: " + err.Error()
		}
		if ss.Spec.Replicas != nil && *ss.Spec.Replicas == 1 {
			return true, ""
		}
		return false, fmt.Sprintf("expected replicas=1, got %v", ss.Spec.Replicas)
	})
}
