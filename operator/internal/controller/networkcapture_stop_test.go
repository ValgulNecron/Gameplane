package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

// These tests cover the F-259 stop flow (maintainer decision 2026-09-25,
// operator/specs.md "Network capture stop flow"): the API only sets
// stopRequestedAnnotation, and this reconciler stops the sidecar before it
// marks the capture Completed.

// stopRecordingSidecar is a SidecarCaptureClient that records StopCapture
// calls and fails every other method, so a test notices any sidecar call
// it did not expect.
type stopRecordingSidecar struct {
	stopCalls []string
	stopErr   error
}

func (s *stopRecordingSidecar) StartCapture(context.Context, string, string, string, *string, int64, int64) error {
	return errors.New("unexpected StartCapture")
}

func (s *stopRecordingSidecar) StopCapture(_ context.Context, _, serverName, captureID string) error {
	s.stopCalls = append(s.stopCalls, serverName+"/"+captureID)
	return s.stopErr
}

func (s *stopRecordingSidecar) GetCaptureStatus(context.Context, string, string, string) (string, int64, int64, string, error) {
	return "", 0, 0, "", errors.New("unexpected GetCaptureStatus")
}

func (s *stopRecordingSidecar) DeleteCaptureFile(context.Context, string, string, string) error {
	return errors.New("unexpected DeleteCaptureFile")
}

const stopTestNS = "games"

func stopTestCapture(name string, phase gameplanev1alpha1.CapturePhase, annotated bool) *gameplanev1alpha1.NetworkCapture {
	nc := &gameplanev1alpha1.NetworkCapture{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: stopTestNS},
		Spec:       gameplanev1alpha1.NetworkCaptureSpec{ServerRef: corev1.LocalObjectReference{Name: "srv"}},
		Status:     gameplanev1alpha1.NetworkCaptureStatus{Phase: phase},
	}
	if annotated {
		nc.Annotations = map[string]string{stopRequestedAnnotation: "2026-09-25T10:00:00Z"}
	}
	return nc
}

func stopTestServer(activeCapture string) *gameplanev1alpha1.GameServer {
	gs := &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{Name: "srv", Namespace: stopTestNS},
	}
	if activeCapture != "" {
		gs.Status.Capture = &gameplanev1alpha1.CaptureStatus{ActiveCapture: ptrTo(activeCapture)}
	}
	return gs
}

func reconcileStopTest(t *testing.T, sidecar *stopRecordingSidecar, gs *gameplanev1alpha1.GameServer, nc *gameplanev1alpha1.NetworkCapture) (*gameplanev1alpha1.NetworkCapture, *gameplanev1alpha1.GameServer) {
	t.Helper()
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(gs, nc).
		WithStatusSubresource(&gameplanev1alpha1.GameServer{}, &gameplanev1alpha1.NetworkCapture{}).
		Build()
	r := &NetworkCaptureReconciler{Client: c, Scheme: c.Scheme(), SidecarClient: sidecar, CaptureEnabled: true}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: stopTestNS, Name: nc.Name}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var gotNC gameplanev1alpha1.NetworkCapture
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: stopTestNS, Name: nc.Name}, &gotNC); err != nil {
		t.Fatalf("get capture: %v", err)
	}
	var gotGS gameplanev1alpha1.GameServer
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: stopTestNS, Name: gs.Name}, &gotGS); err != nil {
		t.Fatalf("get gameserver: %v", err)
	}
	return &gotNC, &gotGS
}

func assertUserStopCompleted(t *testing.T, nc *gameplanev1alpha1.NetworkCapture, wantReason string) {
	t.Helper()
	if nc.Status.Phase != gameplanev1alpha1.CapturePhaseCompleted {
		t.Errorf("phase = %q, want Completed", nc.Status.Phase)
	}
	if nc.Status.Message != userStoppedMessage {
		t.Errorf("message = %q, want %q", nc.Status.Message, userStoppedMessage)
	}
	if nc.Status.CompletionTime == nil {
		t.Error("completionTime not set")
	}
	cond := meta.FindStatusCondition(nc.Status.Conditions, SidecarStoppedCondition)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatalf("SidecarStopped condition = %+v, want True", cond)
	}
	if cond.Reason != wantReason {
		t.Errorf("SidecarStopped reason = %q, want %q", cond.Reason, wantReason)
	}
	if _, ok := nc.Annotations[stopRequestedAnnotation]; !ok {
		t.Error("stop-requested annotation should be kept as a record")
	}
}

func TestNetworkCaptureStopRequested_RunningStopsSidecarThenCompletes(t *testing.T) {
	sidecar := &stopRecordingSidecar{}
	nc, gs := reconcileStopTest(t, sidecar, stopTestServer("cap-run"), stopTestCapture("cap-run", gameplanev1alpha1.CapturePhaseRunning, true))

	if len(sidecar.stopCalls) != 1 || sidecar.stopCalls[0] != "srv/cap-run" {
		t.Errorf("StopCapture calls = %v, want exactly [srv/cap-run]", sidecar.stopCalls)
	}
	assertUserStopCompleted(t, nc, "stopped")
	if meta.FindStatusCondition(nc.Status.Conditions, "SidecarStopFailed") != nil {
		t.Error("SidecarStopFailed must not be set when the sidecar stop succeeds")
	}
	if gs.Status.Capture == nil || gs.Status.Capture.ActiveCapture != nil {
		t.Errorf("gameserver activeCapture = %+v, want released (nil)", gs.Status.Capture)
	}
	if gs.Status.Capture != nil && gs.Status.Capture.LastCaptureTime == nil {
		t.Error("gameserver lastCaptureTime should be recorded")
	}
}

func TestNetworkCaptureStopRequested_SidecarStopFailureStillCompletes(t *testing.T) {
	sidecar := &stopRecordingSidecar{stopErr: errors.New("pod gone")}
	nc, _ := reconcileStopTest(t, sidecar, stopTestServer("cap-gone"), stopTestCapture("cap-gone", gameplanev1alpha1.CapturePhaseRunning, true))

	if len(sidecar.stopCalls) != 1 {
		t.Errorf("StopCapture calls = %v, want exactly one", sidecar.stopCalls)
	}
	assertUserStopCompleted(t, nc, "stopped")
	failed := meta.FindStatusCondition(nc.Status.Conditions, "SidecarStopFailed")
	if failed == nil || failed.Status != metav1.ConditionTrue {
		t.Errorf("SidecarStopFailed condition = %+v, want True", failed)
	}
}

func TestNetworkCaptureStopRequested_PendingCompletesWithoutSidecar(t *testing.T) {
	for _, phase := range []gameplanev1alpha1.CapturePhase{"", gameplanev1alpha1.CapturePhasePending} {
		t.Run("phase="+string(phase), func(t *testing.T) {
			sidecar := &stopRecordingSidecar{}
			nc, _ := reconcileStopTest(t, sidecar, stopTestServer(""), stopTestCapture("cap-pend", phase, true))

			if len(sidecar.stopCalls) != 0 {
				t.Errorf("StopCapture calls = %v, want none for a capture that never started", sidecar.stopCalls)
			}
			assertUserStopCompleted(t, nc, "never_started")
		})
	}
}

// TestNetworkCaptureStopRequested_TerminalIgnored: once Completed, the
// annotation no longer drives anything, so a later reconcile (e.g. the
// retention pass) never calls the sidecar's stop again.
func TestNetworkCaptureStopRequested_TerminalIgnored(t *testing.T) {
	sidecar := &stopRecordingSidecar{}
	nc := stopTestCapture("cap-done", gameplanev1alpha1.CapturePhaseCompleted, true)
	now := metav1.NewTime(time.Now())
	nc.Status.CompletionTime = &now
	nc.Status.Message = userStoppedMessage
	meta.SetStatusCondition(&nc.Status.Conditions, metav1.Condition{
		Type: SidecarStoppedCondition, Status: metav1.ConditionTrue, Reason: "stopped", LastTransitionTime: now,
	})

	got, _ := reconcileStopTest(t, sidecar, stopTestServer(""), nc)
	if len(sidecar.stopCalls) != 0 {
		t.Errorf("StopCapture calls = %v, want none for an already-completed capture", sidecar.stopCalls)
	}
	if got.Status.Phase != gameplanev1alpha1.CapturePhaseCompleted {
		t.Errorf("phase = %q, want Completed", got.Status.Phase)
	}
}

// TestNetworkCaptureStopRequested_LegacyFallback: an API from before F-259
// (mid rolling upgrade) writes phase=Completed + userStoppedMessage with no
// annotation. The reconciler must still stop the sidecar once and record
// SidecarStopped.
func TestNetworkCaptureStopRequested_LegacyFallback(t *testing.T) {
	sidecar := &stopRecordingSidecar{}
	nc := stopTestCapture("cap-legacy", gameplanev1alpha1.CapturePhaseCompleted, false)
	now := metav1.NewTime(time.Now())
	nc.Status.CompletionTime = &now
	nc.Status.Message = userStoppedMessage

	got, gs := reconcileStopTest(t, sidecar, stopTestServer("cap-legacy"), nc)
	if len(sidecar.stopCalls) != 1 || sidecar.stopCalls[0] != "srv/cap-legacy" {
		t.Errorf("StopCapture calls = %v, want exactly [srv/cap-legacy]", sidecar.stopCalls)
	}
	if !meta.IsStatusConditionTrue(got.Status.Conditions, SidecarStoppedCondition) {
		t.Errorf("conditions = %+v, want SidecarStopped=True", got.Status.Conditions)
	}
	if gs.Status.Capture == nil || gs.Status.Capture.ActiveCapture != nil {
		t.Errorf("gameserver activeCapture = %+v, want released (nil)", gs.Status.Capture)
	}
}
