//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// TestGameServer_IdleNeverSleepsWithoutAPlayerCount is the safety property for
// idle auto-sleep, and the one thing envtest genuinely cannot prove: it runs a
// *real* agent sidecar against a real cluster.
//
// The busybox fixture declares no RCON and no player source, so its agent
// reports playersOnline as null — "unknown", the contract from
// agent/internal/heartbeat.sendOnce. Unknown must never be read as zero, or
// every game without a player-count protocol would silently shut itself down
// while people were connected. So: idle enabled, clock backdated well past the
// threshold, and the server must still be running.
//
// The happy path (an empty server actually sleeping and waking) lives in
// operator/internal/controller/gameserver_idle_envtest_test.go rather than
// here. Faking a zero player count in e2e would mean patching status.agent,
// which the live agent overwrites back to null on its next ~20s heartbeat — a
// test that raced the thing it was testing would be worse than no test.
func TestGameServer_IdleNeverSleepsWithoutAPlayerCount(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"
	tmpl := "e2e-idle-tmpl"
	gs := "e2e-idle-nocount"

	applyBusyboxTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)
	waitPVCBound(t, ns, gs+"-data", 90*time.Second)
	requireAgentReady(t, ns, gs)

	// Idle time only accrues from Running, so wait for the real heartbeat to
	// get us there before enabling the policy.
	envInstance.Eventually(t, 3*time.Minute, func() (bool, string) {
		obj, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Get(ctx, gs, metav1.GetOptions{})
		if err != nil {
			return false, "get gameserver: " + err.Error()
		}
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		if phase != "Running" {
			return false, "phase=" + phase
		}
		// Confirm the premise of this test rather than assuming it: the agent
		// must be reporting an absent player count, not a zero one.
		if _, found, _ := unstructured.NestedInt64(obj.Object, "status", "agent", "playersOnline"); found {
			return false, "fixture reports a player count; this test needs one that does not"
		}
		return true, ""
	})

	idlePatch := []byte(`{"spec":{"idle":{"enabled":true,"afterMinutes":5}}}`)
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Patch(ctx, gs, types.MergePatchType, idlePatch, metav1.PatchOptions{}); err != nil {
		t.Fatalf("patch idle: %v", err)
	}

	// Backdate the clock far past the threshold. If the operator were treating
	// unknown as empty, this is exactly what would tip it into sleeping.
	past := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	clockPatch := []byte(`{"status":{"idle":{"emptySince":"` + past + `"}}}`)
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Patch(ctx, gs, types.MergePatchType, clockPatch, metav1.PatchOptions{},
			"status"); err != nil {
		t.Fatalf("backdate idle clock: %v", err)
	}

	// Give the operator several reconciles to do the wrong thing.
	time.Sleep(30 * time.Second)

	obj, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Get(ctx, gs, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get gameserver: %v", err)
	}
	if ann := obj.GetAnnotations(); ann["gameplane.local/idle-asleep-since"] != "" {
		t.Fatal("server slept despite an unknown player count")
	}
	if phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase"); phase != "Running" {
		t.Errorf("phase = %q, want Running", phase)
	}
	// The operator must also have cleared the backdated clock, not merely
	// declined to act on it — a clock left running would sleep the server the
	// instant the game ever did report a count.
	if _, found, _ := unstructured.NestedString(obj.Object, "status", "idle", "emptySince"); found {
		t.Error("idle clock still running on an unknown player count")
	}
	reason, _, _ := unstructured.NestedString(obj.Object, "status", "idle", "reason")
	if reason == "" {
		t.Error("no status.idle.reason; a server that can never sleep must say why")
	}

	ss, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, gs, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get statefulset: %v", err)
	}
	if ss.Spec.Replicas == nil || *ss.Spec.Replicas != 1 {
		t.Errorf("replicas = %v, want 1", ss.Spec.Replicas)
	}
}

// TestCRD_Validation_IdleRejectsBadValues proves the idle bounds are enforced
// by the apiserver in the *shipped* CRD, not just in the Go markers. The
// afterMinutes floor is what stops a server flapping between the last player
// leaving and the next arriving; the cron guard is the same structural check
// BackupSchedule uses, so "every-night" fails at admission rather than silently
// never firing.
func TestCRD_Validation_IdleRejectsBadValues(t *testing.T) {
	t.Parallel()

	t.Run("afterMinutes below the floor", func(t *testing.T) {
		t.Parallel()
		yaml := `apiVersion: gameplane.local/v1alpha1
kind: GameServer
metadata:
  name: e2e-validation-idle-tooshort
  namespace: gameplane-games
spec:
  templateRef:
    name: any-template
  idle:
    enabled: true
    afterMinutes: 1
`
		expectAdmissionRejection(t, yaml, []string{"afterMinutes", "Invalid", "greater than or equal"})
	})

	t.Run("malformed wake window", func(t *testing.T) {
		t.Parallel()
		yaml := `apiVersion: gameplane.local/v1alpha1
kind: GameServer
metadata:
  name: e2e-validation-idle-badcron
  namespace: gameplane-games
spec:
  templateRef:
    name: any-template
  idle:
    enabled: true
    wakeWindows:
      - "every-night"
`
		expectAdmissionRejection(t, yaml, []string{"wakeWindows", "Invalid", "MinLength", "in body"})
	})
}
