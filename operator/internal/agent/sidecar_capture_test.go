package agent

import (
	"context"
	"errors"
	"testing"
)

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
}

// TestCaptureClient_DisabledNoOp verifies that a disabled CaptureClient no-ops without dialing.
func TestCaptureClient_DisabledNoOp(t *testing.T) {
	c := &CaptureClient{Disabled: true}
	ctx := context.Background()

	if err := c.StartCapture(ctx, "ns", "server", "cap-1", nil, 60, 1024); err != nil {
		t.Fatalf("StartCapture disabled: %v", err)
	}
	if err := c.StopCapture(ctx, "ns", "server", "cap-1"); err != nil {
		t.Fatalf("StopCapture disabled: %v", err)
	}
	phase, _, _, _, err := c.GetCaptureStatus(ctx, "ns", "server", "cap-1")
	if err == nil || phase != "unknown" {
		t.Fatalf("GetCaptureStatus disabled: phase=%s err=%v", phase, err)
	}
	if err := c.DeleteCaptureFile(ctx, "ns", "server", "cap-1"); err != nil {
		t.Fatalf("DeleteCaptureFile disabled: %v", err)
	}
}
