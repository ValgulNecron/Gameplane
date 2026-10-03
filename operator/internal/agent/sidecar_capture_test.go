package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type captureRoundTripFunc func(*http.Request) (*http.Response, error)

func (f captureRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCaptureClientCarriesImmutableIdentityOnLifecycleRequests(t *testing.T) {
	calls := 0
	c := &CaptureClient{http: &http.Client{Transport: captureRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("X-Gameplane-Server-UID") != "server-uid" || r.Header.Get("X-Gameplane-Capture-UID") != "capture-uid" {
			t.Error("missing capture lifecycle identity")
		}
		if strings.HasSuffix(r.URL.Path, "/start") {
			var body startCaptureRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.ServerUID != "server-uid" || body.CaptureUID != "capture-uid" {
				t.Error("missing durable start identity")
			}
		}
		status := 200
		if r.Method == http.MethodDelete {
			status = 204
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"status":"completed"}`))}, nil
	})}}
	if err := c.StartCapture(t.Context(), "games", "alpha", "cap-one", "server-uid", "capture-uid", nil, 10, 1024); err != nil {
		t.Fatal(err)
	}
	if err := c.StopCapture(t.Context(), "games", "alpha", "cap-one", "server-uid", "capture-uid"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := c.GetCaptureStatus(t.Context(), "games", "alpha", "cap-one", "server-uid", "capture-uid"); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteCaptureFile(t.Context(), "games", "alpha", "cap-one", "server-uid", "capture-uid"); err != nil {
		t.Fatal(err)
	}
	if calls != 4 {
		t.Fatal(calls)
	}
}

// TestHTTPError_Error tests string formatting of HTTPError.
func TestHTTPError_Error(t *testing.T) {
	errWithBody := &HTTPError{Op: "start capture", StatusCode: 400, Body: "invalid filter"}
	if got := errWithBody.Error(); got != "capture sidecar: start capture: status 400: invalid filter" {
		t.Errorf("got %q, want %q", got, "capture sidecar: start capture: status 400: invalid filter")
	}

	errWithoutBody := &HTTPError{Op: "get status", StatusCode: 404}
	if got := errWithoutBody.Error(); got != "capture sidecar: get status: status 404" {
		t.Errorf("got %q, want %q", got, "capture sidecar: get status: status 404")
	}
}

// TestIsTransientError tests classification of errors as transient vs permanent.
func TestIsTransientError(t *testing.T) {
	if IsTransientError(nil) {
		t.Error("expected nil to not be transient")
	}

	// Network dial / timeout error
	if !IsTransientError(errors.New("dial tcp 127.0.0.1:9091: connect: connection refused")) {
		t.Error("expected dial failure to be transient")
	}

	// 5xx HTTP errors
	err500 := &HTTPError{Op: "start capture", StatusCode: 500, Body: "internal error"}
	if !IsTransientError(err500) {
		t.Error("expected 500 to be transient")
	}

	err502 := &HTTPError{Op: "start capture", StatusCode: 502, Body: "bad gateway"}
	if !IsTransientError(err502) {
		t.Error("expected 502 to be transient")
	}

	err503 := &HTTPError{Op: "start capture", StatusCode: 503, Body: "service unavailable"}
	if !IsTransientError(err503) {
		t.Error("expected 503 to be transient")
	}

	err429 := &HTTPError{Op: "start capture", StatusCode: 429, Body: "too many requests"}
	if !IsTransientError(err429) {
		t.Error("expected 429 to be transient")
	}

	// 4xx HTTP client errors (permanent)
	err400 := &HTTPError{Op: "start capture", StatusCode: 400, Body: "bad request"}
	if IsTransientError(err400) {
		t.Error("expected 400 to NOT be transient")
	}

	err409 := &HTTPError{Op: "start capture", StatusCode: 409, Body: "conflict"}
	if IsTransientError(err409) {
		t.Error("expected 409 to NOT be transient")
	}

	err404 := &HTTPError{Op: "start capture", StatusCode: 404, Body: "not found"}
	if IsTransientError(err404) {
		t.Error("expected 404 to NOT be transient")
	}

	// 507 Insufficient Storage (F-187 volume budget refusal) is a 5xx status
	// but must be treated as permanent: retrying does not free volume space
	// within a reconcile's retry window, so retrying just repeats a directory
	// scan for no benefit.
	err507 := &HTTPError{Op: "start capture", StatusCode: 507, Body: "capture volume budget exceeded"}
	if IsTransientError(err507) {
		t.Error("expected 507 to NOT be transient")
	}
}

// TestCaptureClient_DisabledNoOp verifies that a disabled CaptureClient no-ops without dialing.
func TestCaptureClient_DisabledNoOp(t *testing.T) {
	c := &CaptureClient{Disabled: true}
	ctx := context.Background()

	if err := c.StartCapture(ctx, "ns", "server", "cap-1", "server-uid", "capture-uid", nil, 60, 1024); err != nil {
		t.Fatalf("StartCapture disabled: %v", err)
	}
	if err := c.StopCapture(ctx, "ns", "server", "cap-1", "server-uid", "capture-uid"); err != nil {
		t.Fatalf("StopCapture disabled: %v", err)
	}
	phase, _, _, _, err := c.GetCaptureStatus(ctx, "ns", "server", "cap-1", "server-uid", "capture-uid")
	if err == nil || phase != "unknown" {
		t.Fatalf("GetCaptureStatus disabled: phase=%s err=%v", phase, err)
	}
	if err := c.DeleteCaptureFile(ctx, "ns", "server", "cap-1", "server-uid", "capture-uid"); err != nil {
		t.Fatalf("DeleteCaptureFile disabled: %v", err)
	}
}
