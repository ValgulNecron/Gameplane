package rcon

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func parseHostPort(t *testing.T, s string) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(s)
	if err != nil {
		t.Fatalf("split host port %q: %v", s, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port %q: %v", portStr, err)
	}
	return host, port
}

func staticPass(p string) PassFn {
	return func() (string, error) { return p, nil }
}

func errPass(err error) PassFn {
	return func() (string, error) { return "", err }
}

func TestREST_GenericAdapter_SuccessCases(t *testing.T) {
	tests := []struct {
		name       string
		credential string
		serverResp string
		statusCode int
		wantOutput string
		wantBasic  bool
	}{
		{
			name:       "json output field",
			credential: "my-token",
			serverResp: `{"output": "Server running: 10 players"}`,
			statusCode: http.StatusOK,
			wantOutput: "Server running: 10 players",
		},
		{
			name:       "json result field",
			credential: "user:pass123",
			serverResp: `{"result": "Save complete"}`,
			statusCode: http.StatusOK,
			wantOutput: "Save complete",
			wantBasic:  true,
		},
		{
			name:       "json message field",
			credential: "token",
			serverResp: `{"message": "Broadcast sent"}`,
			statusCode: http.StatusOK,
			wantOutput: "Broadcast sent",
		},
		{
			name:       "json commandResult field",
			credential: "token",
			serverResp: `{"commandResult": "Player kicked"}`,
			statusCode: http.StatusOK,
			wantOutput: "Player kicked",
		},
		{
			name:       "json data field",
			credential: "token",
			serverResp: `{"data": "Status OK"}`,
			statusCode: http.StatusOK,
			wantOutput: "Status OK",
		},
		{
			name:       "plain text response",
			credential: "token",
			serverResp: "Pong\n",
			statusCode: http.StatusOK,
			wantOutput: "Pong",
		},
		{
			name:       "204 no content",
			credential: "token",
			serverResp: "",
			statusCode: http.StatusNoContent,
			wantOutput: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/command" {
					t.Errorf("expected path /api/command, got %s", r.URL.Path)
				}
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}

				if tc.wantBasic {
					expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(tc.credential))
					if r.Header.Get("Authorization") != expectedAuth {
						t.Errorf("expected %s, got %s", expectedAuth, r.Header.Get("Authorization"))
					}
				} else if tc.credential != "" {
					expectedAuth := "Bearer " + tc.credential
					if r.Header.Get("Authorization") != expectedAuth {
						t.Errorf("expected %s, got %s", expectedAuth, r.Header.Get("Authorization"))
					}
					if r.Header.Get("X-TxAdmin-Token") != tc.credential {
						t.Errorf("expected X-TxAdmin-Token %s, got %s", tc.credential, r.Header.Get("X-TxAdmin-Token"))
					}
				}

				body, _ := io.ReadAll(r.Body)
				var reqPayload map[string]string
				if err := json.Unmarshal(body, &reqPayload); err != nil {
					t.Errorf("expected json payload: %v", err)
				}
				if reqPayload["command"] != "test-cmd" {
					t.Errorf("expected command test-cmd, got %s", reqPayload["command"])
				}

				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(tc.serverResp))
			}))
			defer ts.Close()

			host, port := parseHostPort(t, ts.Listener.Addr().String())
			client := NewREST(host, port, staticPass(tc.credential), WithAdapter(&DefaultRESTAdapter{}))
			defer client.Close()

			out, err := client.Exec("test-cmd")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out != tc.wantOutput {
				t.Errorf("Exec() = %q, want %q", out, tc.wantOutput)
			}
		})
	}
}

func TestREST_TxAdminAdapter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/fxserver/commands" {
			t.Errorf("expected path /fxserver/commands, got %s", r.URL.Path)
		}
		if r.Header.Get("X-TxAdmin-Token") != "secret-cfx-token" {
			t.Errorf("expected X-TxAdmin-Token secret-cfx-token, got %s", r.Header.Get("X-TxAdmin-Token"))
		}
		if r.Header.Get("Authorization") != "Bearer secret-cfx-token" {
			t.Errorf("expected Bearer secret-cfx-token, got %s", r.Header.Get("Authorization"))
		}

		body, _ := io.ReadAll(r.Body)
		var reqPayload map[string]string
		if err := json.Unmarshal(body, &reqPayload); err != nil {
			t.Fatalf("bad request json: %v", err)
		}
		if reqPayload["action"] != "console" || reqPayload["parameter"] != "quit" {
			t.Errorf("unexpected payload: %v", reqPayload)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status": "ok", "message": "Server is shutting down..."}`))
	}))
	defer ts.Close()

	host, port := parseHostPort(t, ts.Listener.Addr().String())

	// Test using WithGame("fivem")
	client := NewREST(host, port, staticPass("secret-cfx-token"), WithGame("fivem"))
	defer client.Close()

	if client.adapter.Name() != "txadmin" {
		t.Fatalf("expected adapter name txadmin, got %s", client.adapter.Name())
	}

	out, err := client.Exec("quit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "Server is shutting down..." {
		t.Errorf("Exec() = %q, want %q", out, "Server is shutting down...")
	}
}

func TestREST_FarmingSimulatorAdapter(t *testing.T) {
	expectedPass := "admin:fs25-secret"
	expectedBasic := "Basic " + base64.StdEncoding.EncodeToString([]byte(expectedPass))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/console" {
			t.Errorf("expected path /api/console, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != expectedBasic {
			t.Errorf("expected %s, got %s", expectedBasic, r.Header.Get("Authorization"))
		}

		body, _ := io.ReadAll(r.Body)
		var reqPayload map[string]string
		if err := json.Unmarshal(body, &reqPayload); err != nil {
			t.Fatalf("bad request json: %v", err)
		}
		if reqPayload["command"] != "gsSave" {
			t.Errorf("expected command gsSave, got %s", reqPayload["command"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output": "Game saved successfully"}`))
	}))
	defer ts.Close()

	host, port := parseHostPort(t, ts.Listener.Addr().String())

	// Test passing credential without "admin:" prefix, verifying auto-prefixing
	client := NewREST(host, port, staticPass("fs25-secret"), WithGame("farming-simulator-25"))
	defer client.Close()

	if client.adapter.Name() != "farming-simulator-25" {
		t.Fatalf("expected adapter name farming-simulator-25, got %s", client.adapter.Name())
	}

	out, err := client.Exec("gsSave")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "Game saved successfully" {
		t.Errorf("Exec() = %q, want %q", out, "Game saved successfully")
	}
}

func TestREST_PortBasedDefaultAdapters(t *testing.T) {
	cTx := NewREST("127.0.0.1", 40120, nil)
	if cTx.adapter.Name() != "txadmin" {
		t.Errorf("port 40120 should default to txadmin, got %s", cTx.adapter.Name())
	}

	cFS := NewREST("127.0.0.1", 8080, nil)
	if cFS.adapter.Name() != "farming-simulator-25" {
		t.Errorf("port 8080 should default to farming-simulator-25, got %s", cFS.adapter.Name())
	}

	cGen := NewREST("127.0.0.1", 12345, nil)
	if cGen.adapter.Name() != "generic" {
		t.Errorf("port 12345 should default to generic, got %s", cGen.adapter.Name())
	}
}

func TestREST_AuthFailure_Cooldown(t *testing.T) {
	var callCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Unauthorized"))
	}))
	defer ts.Close()

	host, port := parseHostPort(t, ts.Listener.Addr().String())
	client := NewREST(host, port, staticPass("bad-pass"))
	defer client.Close()

	// First call should reach server and return ErrAuth
	_, err := client.Exec("test")
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("expected ErrAuth, got %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}

	// Second call should immediately fail due to cooldown without reaching server
	_, err = client.Exec("test")
	if err == nil {
		t.Fatal("expected cooldown error, got nil")
	}
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("expected ErrAuth, got %v", err)
	}
	if callCount != 1 {
		t.Fatalf("callCount should still be 1 (cooldown active), got %d", callCount)
	}

	// Shrink cooldown and verify it can dial again
	client.authFailureCooldown = 1 * time.Millisecond
	time.Sleep(5 * time.Millisecond)

	_, _ = client.Exec("test")
	if callCount != 2 {
		t.Fatalf("expected callCount 2 after cooldown expired, got %d", callCount)
	}
}

func TestREST_HTTPErrorStatuses(t *testing.T) {
	tests := []struct {
		status int
		body   string
	}{
		{http.StatusInternalServerError, "Internal server error"},
		{http.StatusBadRequest, "Bad command format"},
		{http.StatusNotFound, "Endpoint not found"},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("status %d", tc.status), func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer ts.Close()

			host, port := parseHostPort(t, ts.Listener.Addr().String())
			client := NewREST(host, port, staticPass("token"))
			defer client.Close()

			_, err := client.Exec("bad")
			if err == nil {
				t.Fatalf("expected error for status %d, got nil", tc.status)
			}
			if !strings.Contains(err.Error(), strconv.Itoa(tc.status)) {
				t.Errorf("expected status %d in error message: %v", tc.status, err)
			}
		})
	}
}

func TestREST_PassFnError(t *testing.T) {
	client := NewREST("127.0.0.1", 8080, errPass(errors.New("secret file missing")))
	defer client.Close()

	_, err := client.Exec("status")
	if err == nil {
		t.Fatal("expected passFn error, got nil")
	}
	if !strings.Contains(err.Error(), "secret file missing") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestREST_LargeResponseCap(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write 2 MiB of data
		chunk := strings.Repeat("A", 1024)
		for i := 0; i < 2048; i++ {
			_, _ = w.Write([]byte(chunk))
		}
	}))
	defer ts.Close()

	host, port := parseHostPort(t, ts.Listener.Addr().String())
	client := NewREST(host, port, staticPass("token"))
	defer client.Close()

	out, err := client.Exec("dumplogs")
	if err != nil {
		t.Fatalf("unexpected error on large response: %v", err)
	}
	if len(out) > restMaxResponseBytes {
		t.Errorf("output length %d exceeded max %d", len(out), restMaxResponseBytes)
	}
}

func TestREST_Options(t *testing.T) {
	client := NewREST("127.0.0.1", 443, nil, WithCustomPath("/custom/api"), WithScheme("https"))
	defer client.Close()

	if client.scheme != "https" {
		t.Errorf("expected https, got %s", client.scheme)
	}
	if client.customPath != "/custom/api" {
		t.Errorf("expected /custom/api, got %s", client.customPath)
	}
}
