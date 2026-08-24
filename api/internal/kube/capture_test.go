package kube

import (
	"context"
	"errors"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

// newCaptureFake builds a dynamic fake client that knows the NetworkCapture
// GVR, mirroring handlers/capture_test.go's fakeCaptureClient setup.
func newCaptureFake(objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scm := runtime.NewScheme()
	gvkr := map[schema.GroupVersionResource]string{
		GVRNetworkCapture: "NetworkCaptureList",
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scm, gvkr, objs...)
}

func createTestCapture(t *testing.T, dyn *dynamicfake.FakeDynamicClient) (*NetworkCapture, error) {
	t.Helper()
	c := &Client{Dynamic: dyn}
	dur := &metav1.Duration{Duration: time.Minute}
	size := resource.NewQuantity(1048576, resource.BinarySI)
	ttl := int32(86400)
	return c.CreateNetworkCapture(context.Background(),
		"gameplane-games", "cap-test", "alpha", "uid-1", nil, dur, size, &ttl)
}

// TestCreateNetworkCapture_RetriesOnConflict covers the arm64 CI failure
// this retry exists for: the reconciler writes the freshly-created
// NetworkCapture's status between our Create and our UpdateStatus, so the
// first UpdateStatus 409s. The call must recover rather than surface the
// conflict to the handler.
func TestCreateNetworkCapture_RetriesOnConflict(t *testing.T) {
	dyn := newCaptureFake()
	var updates int
	dyn.PrependReactor("update", "networkcaptures", func(k8stesting.Action) (bool, runtime.Object, error) {
		updates++
		if updates == 1 {
			return true, nil, apierrors.NewConflict(
				GVRNetworkCapture.GroupResource(), "cap-test", nil)
		}
		return false, nil, nil
	})

	got, err := createTestCapture(t, dyn)
	if err != nil {
		t.Fatalf("create network capture: %v", err)
	}
	if got == nil {
		t.Fatal("returned capture is nil with a nil error")
	}
	if got.Status.Phase != CapturePhasePending {
		t.Fatalf("phase = %q, want %q", got.Status.Phase, CapturePhasePending)
	}
	if updates != 2 {
		t.Fatalf("update attempts = %d, want 2 (one conflict, one success)", updates)
	}
}

// TestCreateNetworkCapture_ConflictKeepsReconcilerPhase asserts the retry
// never writes Pending over a phase the reconciler already advanced past:
// re-driving a Running capture through the operator's Pending branch would
// start the capture on the sidecar twice.
func TestCreateNetworkCapture_ConflictKeepsReconcilerPhase(t *testing.T) {
	dyn := newCaptureFake()
	var updates int
	dyn.PrependReactor("update", "networkcaptures", func(k8stesting.Action) (bool, runtime.Object, error) {
		updates++
		return true, nil, apierrors.NewConflict(
			GVRNetworkCapture.GroupResource(), "cap-test", nil)
	})
	dyn.PrependReactor("get", "networkcaptures", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "gameplane.local/v1alpha1",
			"kind":       "NetworkCapture",
			"metadata": map[string]any{
				"name":      "cap-test",
				"namespace": "gameplane-games",
			},
			"status": map[string]any{"phase": string(CapturePhaseRunning)},
		}}, nil
	})

	got, err := createTestCapture(t, dyn)
	if err != nil {
		t.Fatalf("create network capture: %v", err)
	}
	if got.Status.Phase != CapturePhaseRunning {
		t.Fatalf("phase = %q, want the reconciler's %q left intact",
			got.Status.Phase, CapturePhaseRunning)
	}
	if updates != 1 {
		t.Fatalf("update attempts = %d, want 1 (no write over the reconciler's phase)", updates)
	}
}

// TestCreateNetworkCapture_NonConflictErrorPropagates guards the nil-result
// path: a non-conflict UpdateStatus failure must return an error and never
// dereference the unset result.
func TestCreateNetworkCapture_NonConflictErrorPropagates(t *testing.T) {
	dyn := newCaptureFake()
	dyn.PrependReactor("update", "networkcaptures", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewInternalError(errors.New("boom"))
	})

	got, err := createTestCapture(t, dyn)
	if err == nil {
		t.Fatalf("want an error, got capture %+v", got)
	}
	if got != nil {
		t.Fatalf("want a nil capture alongside the error, got %+v", got)
	}
}
