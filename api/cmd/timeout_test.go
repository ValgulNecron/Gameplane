package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIsStreamingRequest(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		header string
		want   bool
	}{
		{"websocket upgrade", "GET", "/ws/servers/alpha/console", "websocket", true},
		{"websocket upgrade case-insensitive", "GET", "/ws/servers/alpha/console", "WebSocket", true},
		{"events GET", "GET", "/events", "", true},
		{"events GET with query", "GET", "/events?namespace=gameplane-games", "", true},
		{"events wrong method", "POST", "/events", "", false},
		{"unrelated path", "GET", "/servers/alpha", "", false},
		{"unrelated upgrade value", "GET", "/servers/alpha", "h2c", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.header != "" {
				req.Header.Set("Upgrade", tc.header)
			}
			if got := isStreamingRequest(req); got != tc.want {
				t.Errorf("isStreamingRequest(%s %s, Upgrade=%q) = %v, want %v",
					tc.method, tc.path, tc.header, got, tc.want)
			}
		})
	}
}

// TestRequestTimeout_NormalRequestCarriesDeadline proves a normal route
// still gets chi's Timeout middleware — the request context has a
// deadline, matching the real DoS protection this middleware exists for.
func TestRequestTimeout_NormalRequestCarriesDeadline(t *testing.T) {
	mw := requestTimeout(30 * time.Second)
	var sawDeadline bool
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, sawDeadline = r.Context().Deadline()
	}))
	req := httptest.NewRequest(http.MethodGet, "/servers/alpha", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !sawDeadline {
		t.Fatal("normal request context has no deadline, want chi.Timeout applied")
	}
}

// TestRequestTimeout_NormalRequestExpires proves the deadline is real: a
// handler that outlives it gets the standard 504, same as bare
// middleware.Timeout would produce.
func TestRequestTimeout_NormalRequestExpires(t *testing.T) {
	mw := requestTimeout(10 * time.Millisecond)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(200 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/servers/alpha", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusGatewayTimeout {
		t.Fatalf("got %d, want 504 (deadline should have fired)", rr.Code)
	}
}

// TestRequestTimeout_StreamingRequestHasNoDeadline proves the two
// streaming shapes (WebSocket upgrade, /events) bypass chi.Timeout
// entirely — no deadline is imposed on their request context at all, so a
// long-lived connection is never force-closed by this middleware.
func TestRequestTimeout_StreamingRequestHasNoDeadline(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		header string
	}{
		{"websocket upgrade", "GET", "/ws/servers/alpha/console", "websocket"},
		{"events SSE", "GET", "/events", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mw := requestTimeout(10 * time.Millisecond)
			var hasDeadline bool
			h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, hasDeadline = r.Context().Deadline()
			}))
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.header != "" {
				req.Header.Set("Upgrade", tc.header)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if hasDeadline {
				t.Fatal("streaming request context has a deadline, want none")
			}
		})
	}
}

// TestRequestTimeout_StreamingRequestOutlivesTimeout proves a streaming
// handler that runs well past the configured deadline is never force
// gateway-timed-out — the exact regression this middleware fixes (SSE
// connections killed every 60s).
func TestRequestTimeout_StreamingRequestOutlivesTimeout(t *testing.T) {
	mw := requestTimeout(10 * time.Millisecond)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond) // well past the configured deadline
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (streaming route must not be cut off at the deadline)", rr.Code)
	}
}
