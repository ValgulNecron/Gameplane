package rcon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)


func TestWebSocketHappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		ctx := context.Background()
		var req WebSocketMessage
		if err := readJSON(ctx, conn, &req); err != nil {
			t.Logf("failed to read request: %v", err)
			return
		}

		// Echo back the response with the same Identifier
		resp := WebSocketMessage{
			Identifier: req.Identifier,
			Message:    "save completed",
			Type:       3,
		}
		_ = writeJSON(ctx, conn, resp)
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewWebSocket(host, port, func() (string, error) { return "testpass", nil })
	defer client.Close()

	result, err := client.Exec("save")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if result != "save completed" {
		t.Errorf("expected 'save completed', got %q", result)
	}
}

func TestWebSocketInterleavedUnsolicited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		ctx := context.Background()
		var req WebSocketMessage
		if err := readJSON(ctx, conn, &req); err != nil {
			t.Logf("failed to read request: %v", err)
			return
		}

		// Send TWO unsolicited frames with different Identifiers
		// The client must DISCARD these and wait for the matching Identifier.
		unsolicited1 := WebSocketMessage{
			Identifier: 0,
			Message:    "[CHAT] Player joined",
			Type:       3,
		}
		_ = writeJSON(ctx, conn, unsolicited1)

		unsolicited2 := WebSocketMessage{
			Identifier: -1,
			Message:    "[LOG] Server tick",
			Type:       3,
		}
		_ = writeJSON(ctx, conn, unsolicited2)

		// NOW send the matching response
		resp := WebSocketMessage{
			Identifier: req.Identifier,
			Message:    "players: alice, bob",
			Type:       3,
		}
		_ = writeJSON(ctx, conn, resp)
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewWebSocket(host, port, func() (string, error) { return "testpass", nil })
	defer client.Close()

	result, err := client.Exec("playerlist")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	// The result must be from the matching frame, not the spam
	if result != "players: alice, bob" {
		t.Errorf("expected 'players: alice, bob', got %q (may have read unsolicited frames)", result)
	}
}

func TestWebSocketPasswordInPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the password is in the path, URL-escaped
		if r.URL.Path != "/s3cret" {
			t.Errorf("expected path /s3cret, got %s", r.URL.Path)
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		ctx := context.Background()
		var req WebSocketMessage
		if err := readJSON(ctx, conn, &req); err != nil {
			return
		}

		resp := WebSocketMessage{
			Identifier: req.Identifier,
			Message:    "ok",
			Type:       3,
		}
		_ = writeJSON(ctx, conn, resp)
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewWebSocket(host, port, func() (string, error) { return "s3cret", nil })
	defer client.Close()

	_, err := client.Exec("test")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
}

func TestWebSocketAuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		// Close immediately — auth failure is signaled by the server
		// closing the connection after accepting the handshake.
		conn.Close(websocket.StatusCode(1006), "")
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewWebSocket(host, port, func() (string, error) { return "wrongpass", nil })
	defer client.Close()

	_, err := client.Exec("test")
	if err == nil {
		t.Fatal("expected an error for early close, got nil")
	}
	if !strings.Contains(err.Error(), "auth") {
		t.Logf("warning: error message doesn't mention auth: %v", err)
	}
}

func TestWebSocketTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		// Accept but never respond
		ctx := context.Background()
		var req WebSocketMessage
		_ = readJSON(ctx, conn, &req)
		// Hang forever — the client should timeout
		select {}
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)

	// Inject a very short timeout for testing
	client := NewWebSocket(host, port, func() (string, error) { return "pass", nil })
	client.execDeadline = 100 * time.Millisecond // Short deadline for test
	defer client.Close()

	start := time.Now()
	_, err := client.Exec("test")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("timeout took too long: %v (should respect execDeadline)", elapsed)
	}
}

func TestWebSocketCommandTooLong(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		ctx := context.Background()
		var req WebSocketMessage
		if err := readJSON(ctx, conn, &req); err != nil {
			return
		}

		resp := WebSocketMessage{
			Identifier: req.Identifier,
			Message:    "ok",
			Type:       3,
		}
		_ = writeJSON(ctx, conn, resp)
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewWebSocket(host, port, func() (string, error) { return "pass", nil })
	defer client.Close()

	// Create a command longer than 1000 chars
	longCmd := strings.Repeat("x", 1001)
	_, err := client.Exec(longCmd)

	if err == nil {
		t.Fatal("expected an error for command > 1000 chars, got nil")
	}
	if !strings.Contains(err.Error(), "too long") || !strings.Contains(err.Error(), "1000") {
		t.Logf("warning: error message doesn't mention the 1000-char limit: %v", err)
	}
}

func TestWebSocketConcurrentExecs(t *testing.T) {
	// Track identifiers seen by the server to ensure they're unique
	var mu sync.Mutex
	seenIDs := make(map[int64]bool)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		ctx := context.Background()
		for {
			var req WebSocketMessage
			if err := readJSON(ctx, conn, &req); err != nil {
				return
			}

			// Track this Identifier
			mu.Lock()
			if seenIDs[req.Identifier] {
				t.Errorf("duplicate Identifier: %d", req.Identifier)
			}
			seenIDs[req.Identifier] = true
			mu.Unlock()

			// Echo back
			resp := WebSocketMessage{
				Identifier: req.Identifier,
				Message:    fmt.Sprintf("result_%d", req.Identifier),
				Type:       3,
			}
			_ = writeJSON(ctx, conn, resp)
		}
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewWebSocket(host, port, func() (string, error) { return "pass", nil })
	defer client.Close()

	// Fire off concurrent Execs
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cmd := fmt.Sprintf("cmd_%d", idx)
			result, err := client.Exec(cmd)
			if err != nil {
				t.Errorf("Exec(%s) failed: %v", cmd, err)
				return
			}
			// Result should have the Identifier (which we can't know here,
			// but the server should have echoed back a unique one)
			if !strings.HasPrefix(result, "result_") {
				t.Errorf("unexpected result format: %s", result)
			}
		}(i)
	}
	wg.Wait()

	// Verify we saw multiple unique Identifiers
	mu.Lock()
	count := len(seenIDs)
	mu.Unlock()
	if count < 3 {
		t.Errorf("expected multiple unique Identifiers, got %d", count)
	}
}

// Helper functions

func readJSON(ctx context.Context, conn *websocket.Conn, v interface{}) error {
	_, data, err := conn.Read(ctx)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func writeJSON(ctx context.Context, conn *websocket.Conn, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, data)
}

func parseHostPort(t *testing.T, serverURL string) (string, int) {
	// serverURL is like "http://127.0.0.1:12345"
	parts := strings.Split(serverURL, "://")
	if len(parts) != 2 {
		t.Fatalf("invalid server URL: %s", serverURL)
	}
	hostPort := parts[1]
	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil {
		t.Fatalf("failed to split host:port: %v", err)
	}
	var port int
	_, _ = fmt.Sscanf(portStr, "%d", &port)
	return host, port
}
