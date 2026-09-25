package handlers

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/ValgulNecron/gameplane/api/internal/kube"
	"github.com/ValgulNecron/gameplane/api/internal/scope"
)

// These tests cover the race between a user-requested stop (which sets
// phase=Completed on the NetworkCapture straight away) and the operator
// telling the capture sidecar to stop (which it records afterwards as
// SidecarStopped=True). The e2e suite hit it as a 409 from the download
// right after the capture read Completed; the download must instead wait
// for the sidecar to be stopped.

type captureRoundTripFunc func(*http.Request) (*http.Response, error)

func (f captureRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// newUserStoppedNetworkCapture builds a capture exactly as the API's
// StopNetworkCapture leaves it, before the operator has reconciled it.
func newUserStoppedNetworkCapture(t *testing.T, name, serverName string) *unstructured.Unstructured {
	t.Helper()
	nc := newCompletedNetworkCapture(t, name, serverName, time.Second, 86400)
	if err := unstructured.SetNestedField(nc.Object, kube.CaptureUserStoppedMessage, "status", "message"); err != nil {
		t.Fatalf("seed status.message: %v", err)
	}
	return nc
}

// markSidecarStopped does what the operator's user-stop branch does: record
// SidecarStopped=True on the capture's status.
func markSidecarStopped(ctx context.Context, t *testing.T, k *kube.Client, name string) {
	t.Helper()
	res := k.Dynamic.Resource(kube.GVRNetworkCapture).Namespace(scope.DefaultNamespace)
	u, err := res.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("get capture: %v", err)
		return
	}
	cond := map[string]any{
		"type":               kube.CaptureSidecarStoppedCondition,
		"status":             "True",
		"reason":             "stopped",
		"message":            "sidecar told to stop capturing",
		"lastTransitionTime": time.Now().UTC().Format(time.RFC3339),
	}
	if err := unstructured.SetNestedSlice(u.Object, []any{cond}, "status", "conditions"); err != nil {
		t.Errorf("set conditions: %v", err)
		return
	}
	if _, err := res.Update(ctx, u, metav1.UpdateOptions{}); err != nil {
		t.Errorf("update capture: %v", err)
	}
}

// newFinalizeTestRouter mounts only the download route on a handler whose
// sidecar transport is stubbed. The stub answers the way the real sidecar
// does: 409 while the capture is still running on it, 200 once stopped.
func newFinalizeTestRouter(t *testing.T, k *kube.Client, sidecarStopped *atomic.Bool, calls *atomic.Int32, wait time.Duration) *chi.Mux {
	t.Helper()
	reg := kube.NewRegistry(scope.DefaultCluster)
	reg.Set(scope.DefaultCluster, k)
	h := &captureHandler{
		reg:     reg,
		auditor: newCaptureAuditor(t),
		cfg:     captureTestCfg,
		tlsClient: &http.Client{Transport: captureRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls.Add(1)
			if !sidecarStopped.Load() {
				return &http.Response{
					StatusCode: http.StatusConflict,
					Body:       io.NopCloser(strings.NewReader("capture is still running")),
					Header:     http.Header{},
					Request:    r,
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("pcapng-bytes")),
				Header:     http.Header{},
				Request:    r,
			}, nil
		})},
		finalizeWait: wait,
		finalizePoll: 10 * time.Millisecond,
	}
	r := chi.NewRouter()
	r.Get("/servers/{name}:capture-file", h.captureDownload)
	return r
}

// TestCaptureDownload_UserStoppedWaitsForSidecarStop: a download issued
// right after a user stop, before the operator has stopped the sidecar,
// must wait for that and then succeed, never surface the sidecar's 409.
func TestCaptureDownload_UserStoppedWaitsForSidecarStop(t *testing.T) {
	k := fakeCaptureClient(newCaptureServerObj("dl-race", true), newUserStoppedNetworkCapture(t, "cap-race", "dl-race"))
	var stopped atomic.Bool
	var calls atomic.Int32
	r := newFinalizeTestRouter(t, k, &stopped, &calls, 5*time.Second)

	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(100 * time.Millisecond)
		// The sidecar finishes its :stop before the operator records the
		// condition, matching the reconciler's ordering.
		stopped.Store(true)
		markSidecarStopped(context.Background(), t, k, "cap-race")
	}()

	rr := do(t, r, http.MethodGet, "/servers/dl-race:capture-file?id=cap-race", nil)
	<-done
	if rr.Code != http.StatusOK {
		t.Fatalf("download status = %d, want 200 once the sidecar is stopped; body=%s", rr.Code, rr.Body)
	}
	if got := rr.Body.String(); got != "pcapng-bytes" {
		t.Errorf("download body = %q, want the sidecar's file", got)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("sidecar called %d times, want exactly 1 (only after the stop was recorded)", n)
	}
}

// TestCaptureDownload_UserStoppedNeverFinalizedConflicts: if the operator
// never records the sidecar stop, the download gives up with a 409 of its
// own and never proxies to the still-running sidecar.
func TestCaptureDownload_UserStoppedNeverFinalizedConflicts(t *testing.T) {
	k := fakeCaptureClient(newCaptureServerObj("dl-stall", true), newUserStoppedNetworkCapture(t, "cap-stall", "dl-stall"))
	var stopped atomic.Bool
	var calls atomic.Int32
	r := newFinalizeTestRouter(t, k, &stopped, &calls, 100*time.Millisecond)

	rr := do(t, r, http.MethodGet, "/servers/dl-stall:capture-file?id=cap-stall", nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("download status = %d, want 409; body=%s", rr.Code, rr.Body)
	}
	if !strings.Contains(rr.Body.String(), "being finalized") {
		t.Errorf("body = %q, want the not-finalized message", rr.Body.String())
	}
	if n := calls.Load(); n != 0 {
		t.Errorf("sidecar called %d times, want 0", n)
	}
}

// TestCaptureDownload_SidecarCompletedDoesNotWait: a capture the sidecar
// completed itself (duration/size limit) has no user-stop message, so the
// download proxies immediately.
func TestCaptureDownload_SidecarCompletedDoesNotWait(t *testing.T) {
	k := fakeCaptureClient(newCaptureServerObj("dl-auto", true), newCompletedNetworkCapture(t, "cap-auto", "dl-auto", time.Second, 86400))
	var stopped atomic.Bool
	stopped.Store(true)
	var calls atomic.Int32
	r := newFinalizeTestRouter(t, k, &stopped, &calls, 5*time.Second)

	start := time.Now()
	rr := do(t, r, http.MethodGet, "/servers/dl-auto:capture-file?id=cap-auto", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("download status = %d, want 200; body=%s", rr.Code, rr.Body)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("download took %s, want no finalize wait", elapsed)
	}
}
