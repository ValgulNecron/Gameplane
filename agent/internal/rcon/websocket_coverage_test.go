package rcon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coder/websocket"
)

// TestIsAuthCloseSignal_CloseFrame covers the websocket.CloseError branch
// of isAuthCloseSignal, which the existing TestIsAuthCloseSignal table
// (nil / io.EOF / plain error / ECONNRESET / timeout) does not exercise.
func TestIsAuthCloseSignal_CloseFrame(t *testing.T) {
	closeErr := websocket.CloseError{Code: websocket.StatusPolicyViolation, Reason: "bad password"}
	if !isAuthCloseSignal(closeErr) {
		t.Error("a WebSocket close frame error should be treated as an auth close signal")
	}
}

// TestWebSocketWriteFailsAfterCloseNow deterministically exercises Exec's
// Write-failure path (websocket.go's classifyExecErrLocked call site at
// the Write error return), unlike TestWebSocketWriteFailure/
// TestWebSocketReconnectAfterError, which depend on how fast the server
// closes relative to the client's write and so can land on either the
// Write or the Read error path. Here we run one full happy-path Exec (so
// the connection is proven live), then close the client's own conn
// directly — a local CloseNow always makes the client's own next Write
// fail, with no dependency on server timing at all.
func TestWebSocketWriteFailsAfterCloseNow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		ctx := context.WithoutCancel(r.Context())
		for {
			var req WebSocketMessage
			if err := readJSON(ctx, conn, &req); err != nil {
				return
			}
			resp := WebSocketMessage{Identifier: req.Identifier, Message: "ok", Type: 3}
			if err := writeJSON(ctx, conn, resp); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewWebSocket(host, port, func() (string, error) { return "pw", nil })
	defer client.Close()

	if _, err := client.Exec("first"); err != nil {
		t.Fatalf("initial Exec should succeed: %v", err)
	}

	// Close the client's own conn directly (same package). The next Exec's
	// ensureLocked sees c.conn != nil and skips redialing, so the write
	// below hits the already-closed local conn deterministically.
	client.mu.Lock()
	_ = client.conn.CloseNow()
	client.mu.Unlock()

	_, err := client.Exec("second")
	if err == nil {
		t.Fatal("Exec should fail when the underlying conn was already closed")
	}
}

// TestWebSocketZeroedTimeoutsFallBackToDefaults covers the d<=0 fallback
// in Exec (execDeadline) and the dialTimeout<=0 fallback in ensureLocked,
// both deterministically satisfied by a fake server that answers
// immediately, so no test ever actually waits out the real default
// durations.
func TestWebSocketZeroedTimeoutsFallBackToDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		ctx := context.WithoutCancel(r.Context())
		var req WebSocketMessage
		if err := readJSON(ctx, conn, &req); err != nil {
			return
		}
		resp := WebSocketMessage{Identifier: req.Identifier, Message: "ok", Type: 3}
		_ = writeJSON(ctx, conn, resp)
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewWebSocket(host, port, func() (string, error) { return "pw", nil })
	client.dialTimeout = 0
	client.execDeadline = 0
	defer client.Close()

	got, err := client.Exec("status")
	if err != nil {
		t.Fatalf("Exec with zeroed timeouts should fall back to defaults and succeed: %v", err)
	}
	if got != "ok" {
		t.Errorf("Exec = %q, want %q", got, "ok")
	}
}

// TestNewWebSocketDefaultPort covers NewWebSocket's port==0 fallback to
// defaultWebSocketPort (28016).
func TestNewWebSocketDefaultPort(t *testing.T) {
	client := NewWebSocket("127.0.0.1", 0, func() (string, error) { return "pw", nil })
	defer client.Close()
	if client.port != defaultWebSocketPort {
		t.Errorf("port = %d, want default %d", client.port, defaultWebSocketPort)
	}
	const want = "127.0.0.1:28016"
	if client.baseURL != want {
		t.Errorf("baseURL = %q, want %q", client.baseURL, want)
	}
}
