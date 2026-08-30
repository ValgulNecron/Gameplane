//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/retry"
)

// captureListTestResp mirrors the envelope returned by GET
// /servers/{name}:captures (see api/internal/handlers/capture.go's
// captureListResp) — {"captures":[{"captureId":...}],"total","limit","offset"}.
type captureListTestResp struct {
	Captures []struct {
		CaptureID string `json:"captureId"`
	} `json:"captures"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// testRetentionCaptureMaxSize is the maximum size for retention testing.
// Must stay at or below charts/gameplane/values.yaml's capture.defaultMaxSizeBytes (900 MiB).
// For retention/TTL tests, the actual file size is immaterial; we use a minimal value.
const testRetentionCaptureMaxSize = 1048576 // 1 MiB

// TestNetworkCapture_RetentionExpiry asserts that a completed capture whose
// TTL has elapsed transitions to Expired phase, is cleaned up by the
// reconciler, and is immediately refused by the API before GC removes the CR.
// The test:
//
//  1. Creates a busybox GameServer with capture enabled.
//  2. Waits for the capture sidecar to be ready.
//  3. Starts and stops a capture to reach Completed phase.
//  4. Backdates status.completionTime well into the past so the TTL is
//     already expired (the CRD enforces Minimum=60 on TTL itself, so we
//     short-circuit via the anchor rather than waiting 60s).
//  5. Asserts the NetworkCapture CR transitions to Expired phase via the
//     reconciler, then is deleted.
//  6. Asserts the API immediately refuses Get/Download (404 with "not found
//     or has expired") and omits it from List, even before GC removes the CR
//     (that window is the whole point of the API-side expiry check).
func TestNetworkCapture_RetentionExpiry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"
	tmpl := "e2e-retention-tmpl"
	gsName := "e2e-retention-gs"

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	cli := envInstance.APIClient(t, adminUsername, adminPassword)
	defer cli.Close()

	applyBusyboxTemplate(t, tmpl)
	applyBusyboxGameServerWithCapture(t, ns, gsName, tmpl)
	waitPVCBound(t, ns, gsName+"-data", 90*time.Second)
	requireAgentReady(t, ns, gsName)

	// The ephemeral capture container must reach Running before we start
	// a capture.
	envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
		obj, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Get(ctx, gsName, metav1.GetOptions{})
		if err != nil {
			return false, "get gameserver: " + err.Error()
		}
		ready, _, _ := unstructured.NestedBool(obj.Object, "status", "capture", "ready")
		if !ready {
			return false, "status.capture.ready still false"
		}
		return true, ""
	})

	// Start a capture.
	startReq := map[string]any{
		"filter":                  "tcp port 12345",
		"maxDurationSeconds":      300,
		"maxSizeBytes":            testRetentionCaptureMaxSize,
		"ttlSecondsAfterFinished": 60,
	}
	startHTTPResp, startBody, err := cli.Post("/servers/"+gsName+":capture-start", startReq)
	if err != nil {
		t.Fatalf("start capture: %v", err)
	}
	defer func() { _ = startHTTPResp.Body.Close() }()
	if startHTTPResp.StatusCode != http.StatusAccepted {
		t.Fatalf("start capture: status=%s body=%s", startHTTPResp.Status, startBody)
	}
	var startedCapture struct {
		CaptureID string `json:"captureId"`
	}
	if err := json.Unmarshal(startBody, &startedCapture); err != nil {
		t.Fatalf("parse capture-start response %q: %v", startBody, err)
	}
	captureID := startedCapture.CaptureID
	if captureID == "" {
		t.Fatalf("capture-start response had no captureId: %s", startBody)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(networkCaptureGVR).Namespace(ns).
			Delete(context.Background(), captureID, metav1.DeleteOptions{})
	})

	// Wait for the capture to reach Running.
	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		obj, err := envInstance.Dyn.Resource(networkCaptureGVR).Namespace(ns).
			Get(ctx, captureID, metav1.GetOptions{})
		if err != nil {
			return false, "get networkcapture: " + err.Error()
		}
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		if phase != "Running" {
			return false, "phase=" + phase
		}
		return true, ""
	})

	// Stop the capture.
	stopHTTPResp, stopBody, err := cli.Post("/servers/"+gsName+":capture-stop", map[string]any{
		"captureId": captureID,
	})
	if err != nil {
		t.Fatalf("stop capture: %v", err)
	}
	defer func() { _ = stopHTTPResp.Body.Close() }()
	if stopHTTPResp.StatusCode != http.StatusOK {
		t.Fatalf("stop capture: status=%s body=%s", stopHTTPResp.Status, stopBody)
	}

	// Wait for the capture to reach Completed.
	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		obj, err := envInstance.Dyn.Resource(networkCaptureGVR).Namespace(ns).
			Get(ctx, captureID, metav1.GetOptions{})
		if err != nil {
			return false, "get networkcapture: " + err.Error()
		}
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		if phase != "Completed" {
			return false, "phase=" + phase
		}
		return true, ""
	})

	// Sanity check the List envelope while the capture is still present:
	// this proves the response actually decodes populated entries, so the
	// later "capture is absent after expiry" assertion can't pass vacuously
	// from a silently-empty parse (e.g. a mismatched field name yielding a
	// zero-value slice either way).
	preListHTTPResp, preListBody, err := cli.Get("/servers/" + gsName + ":captures")
	if err != nil {
		t.Fatalf("list captures before expiry: %v", err)
	}
	defer func() { _ = preListHTTPResp.Body.Close() }()
	if preListHTTPResp.StatusCode != http.StatusOK {
		t.Fatalf("list captures before expiry: status=%s body=%s", preListHTTPResp.Status, preListBody)
	}
	var preResp captureListTestResp
	if err := json.Unmarshal(preListBody, &preResp); err != nil {
		t.Fatalf("parse pre-expiry capture list: %v", err)
	}
	preFound := false
	for _, cap := range preResp.Captures {
		if cap.CaptureID == captureID {
			preFound = true
			break
		}
	}
	if !preFound {
		t.Fatalf("capture %s not found in pre-expiry List response (total=%d, body=%s) — list parsing likely broken",
			captureID, preResp.Total, preListBody)
	}

	// Backdate completionTime to 2 hours in the past so the TTL is already
	// expired — matching the pattern in envtest (TestNetworkCapture_RetentionExpiresCompletedCapture).
	past := metav1.NewTime(time.Now().Add(-2 * time.Hour))
	// Retry on conflict: the operator's reconciler writes status concurrently,
	// so this read-modify-write can race. RetryOnConflict re-fetches the object
	// inside the closure if a conflict occurs.
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		nc, err := envInstance.Dyn.Resource(networkCaptureGVR).Namespace(ns).
			Get(ctx, captureID, metav1.GetOptions{})
		if err != nil {
			return err
		}
		// Convert metav1.Time to RFC3339 string for unstructured API compatibility.
		// The unstructured API cannot deep copy metav1.Time objects. metav1.Time's
		// MarshalJSON renders in UTC with a trailing Z, so use UTC() before Format
		// to match the exact apiserver serialization.
		if err := unstructured.SetNestedField(nc.Object, past.UTC().Format(time.RFC3339), "status", "completionTime"); err != nil {
			return err
		}
		_, err = envInstance.Dyn.Resource(networkCaptureGVR).Namespace(ns).
			UpdateStatus(ctx, nc, metav1.UpdateOptions{})
		return err
	}); err != nil {
		t.Fatalf("patch completionTime to past: %v", err)
	}

	// Assert the capture transitions to Expired phase.
	envInstance.Eventually(t, 30*time.Second, func() (bool, string) {
		obj, err := envInstance.Dyn.Resource(networkCaptureGVR).Namespace(ns).
			Get(ctx, captureID, metav1.GetOptions{})
		if err != nil && apierrors.IsNotFound(err) {
			// The CR has already been deleted — check the API-tier behavior
			// first while it's definitely gone.
			return true, ""
		}
		if err != nil {
			return false, "get networkcapture: " + err.Error()
		}
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		if phase != "Expired" && !apierrors.IsNotFound(err) {
			return false, "phase=" + phase
		}
		return true, ""
	})

	// Assert the API refuses Get with 404 "not found or has expired"
	// (must be true even if the CR hasn't been garbage-collected yet).
	getHTTPResp, getBody, err := cli.Get("/servers/" + gsName + ":capture-file?id=" + captureID)
	if err != nil {
		t.Fatalf("get capture file: %v", err)
	}
	defer func() { _ = getHTTPResp.Body.Close() }()
	if getHTTPResp.StatusCode != http.StatusNotFound {
		t.Errorf("GET capture after expiry: status=%d (want 404), body=%s",
			getHTTPResp.StatusCode, getBody)
	}

	// Assert the API omits the expired capture from List before GC deletes it.
	listHTTPResp, listBody, err := cli.Get("/servers/" + gsName + ":captures")
	if err != nil {
		t.Fatalf("list captures: %v", err)
	}
	defer func() { _ = listHTTPResp.Body.Close() }()
	if listHTTPResp.StatusCode != http.StatusOK {
		t.Fatalf("list captures: status=%s body=%s", listHTTPResp.Status, listBody)
	}
	var resp captureListTestResp
	if err := json.Unmarshal(listBody, &resp); err != nil {
		t.Fatalf("parse capture list: %v", err)
	}
	for _, cap := range resp.Captures {
		if cap.CaptureID == captureID {
			t.Errorf("expired capture %s still in List response", captureID)
		}
	}

	// Eventually the CR is garbage-collected.
	envInstance.Eventually(t, 30*time.Second, func() (bool, string) {
		_, err := envInstance.Dyn.Resource(networkCaptureGVR).Namespace(ns).
			Get(ctx, captureID, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, ""
		}
		if err != nil {
			return false, "get networkcapture: " + err.Error()
		}
		return false, "capture still present in cluster"
	})
}
