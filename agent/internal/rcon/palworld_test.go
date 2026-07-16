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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestPalworldBasicAuthHeader covers required case 1: every request carries
// HTTP Basic auth with username "admin" and the resolved password, decoded
// exactly, not just prefix-matched.
func TestPalworldBasicAuthHeader(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewPalworld(host, port, func() (string, error) { return "s3cret", nil })
	defer func() { _ = client.Close() }()

	if _, err := client.Exec("save"); err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	const prefix = "Basic "
	if !strings.HasPrefix(gotAuth, prefix) {
		t.Fatalf("Authorization header = %q, want prefix %q", gotAuth, prefix)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(gotAuth, prefix))
	if err != nil {
		t.Fatalf("decode Authorization header: %v", err)
	}
	if string(decoded) != "admin:s3cret" {
		t.Errorf("decoded Authorization = %q, want %q", decoded, "admin:s3cret")
	}
}

// TestPalworldAnnounceFullMessage covers required case 2: the FULL
// multi-word message survives into the request body (the deprecated RCON
// this replaces truncated at the first space).
func TestPalworldAnnounceFullMessage(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody palworldMessagePayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewPalworld(host, port, func() (string, error) { return "pw", nil })
	defer func() { _ = client.Close() }()

	if _, err := client.Exec("announce Hello world everyone"); err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/v1/api/announce" {
		t.Errorf("path = %q, want /v1/api/announce", gotPath)
	}
	if gotBody.Message != "Hello world everyone" {
		t.Errorf("message = %q, want %q", gotBody.Message, "Hello world everyone")
	}
}

// TestPalworldSave covers required case 3: save POSTs to /v1/api/save with
// no body.
func TestPalworldSave(t *testing.T) {
	var gotPath, gotMethod string
	var bodyLen int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		b, _ := io.ReadAll(r.Body)
		bodyLen = len(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewPalworld(host, port, func() (string, error) { return "pw", nil })
	defer func() { _ = client.Close() }()

	out, err := client.Exec("save")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if out != "" {
		t.Errorf("Exec(save) = %q, want empty string", out)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/v1/api/save" {
		t.Errorf("path = %q, want /v1/api/save", gotPath)
	}
	if bodyLen != 0 {
		t.Errorf("save request body length = %d, want 0", bodyLen)
	}
}

// TestPalworldPlayersReturnsRawBody covers required case 4: the players GET
// returns the raw JSON body verbatim, so a regex-based capability can parse
// it exactly like Rust's playerlist output.
func TestPalworldPlayersReturnsRawBody(t *testing.T) {
	const rawJSON = `{"players":[{"name":"Alice","accountName":"alice","playerId":"1","userId":"steam_1","ip":"1.2.3.4","ping":10}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v1/api/players" {
			t.Errorf("path = %q, want /v1/api/players", r.URL.Path)
		}
		_, _ = w.Write([]byte(rawJSON))
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewPalworld(host, port, func() (string, error) { return "pw", nil })
	defer func() { _ = client.Close() }()

	out, err := client.Exec("players")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if out != rawJSON {
		t.Errorf("Exec(players) = %q, want raw body %q", out, rawJSON)
	}
}

// TestPalworldInfoMetricsSettingsReturnRawBody exercises the other three GET
// verbs the same way as players.
func TestPalworldInfoMetricsSettingsReturnRawBody(t *testing.T) {
	cases := []struct {
		cmd  string
		path string
		body string
	}{
		{"info", "/v1/api/info", `{"version":"v1.0","servername":"My Server","description":"","worldguid":"abc"}`},
		{"metrics", "/v1/api/metrics", `{"serverfps":60,"currentplayernum":1,"maxplayernum":32,"uptime":100,"days":1}`},
		{"settings", "/v1/api/settings", `{"Difficulty":"None"}`},
	}
	for _, tc := range cases {
		t.Run(tc.cmd, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("method = %s, want GET", r.Method)
				}
				if r.URL.Path != tc.path {
					t.Errorf("path = %q, want %q", r.URL.Path, tc.path)
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			host, port := parseHostPort(t, server.URL)
			client := NewPalworld(host, port, func() (string, error) { return "pw", nil })
			defer func() { _ = client.Close() }()

			out, err := client.Exec(tc.cmd)
			if err != nil {
				t.Fatalf("Exec(%q) failed: %v", tc.cmd, err)
			}
			if out != tc.body {
				t.Errorf("Exec(%q) = %q, want %q", tc.cmd, out, tc.body)
			}
		})
	}
}

// TestPalworldShutdown covers required case 5: waittime is encoded as an
// int (not a string), and the message keeps its spaces.
func TestPalworldShutdown(t *testing.T) {
	var rawBody []byte
	var gotBody palworldShutdownPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		rawBody = b
		if err := json.Unmarshal(b, &gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewPalworld(host, port, func() (string, error) { return "pw", nil })
	defer func() { _ = client.Close() }()

	if _, err := client.Exec("shutdown 30 Server going down"); err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if gotBody.Waittime != 30 {
		t.Errorf("waittime = %d, want 30", gotBody.Waittime)
	}
	if gotBody.Message != "Server going down" {
		t.Errorf("message = %q, want %q", gotBody.Message, "Server going down")
	}
	if !strings.Contains(string(rawBody), `"waittime":30`) {
		t.Errorf("raw body = %s, want waittime encoded as a bare JSON int (30), not a string", rawBody)
	}
}

// TestPalworldShutdownInvalidWaittime proves a non-numeric waittime is
// rejected before any HTTP request is made.
func TestPalworldShutdownInvalidWaittime(t *testing.T) {
	var reqCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&reqCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewPalworld(host, port, func() (string, error) { return "pw", nil })
	defer func() { _ = client.Close() }()

	_, err := client.Exec("shutdown notanumber Server going down")
	if err == nil {
		t.Fatal("expected an error for a non-numeric waittime")
	}
	if got := atomic.LoadInt32(&reqCount); got != 0 {
		t.Errorf("invalid waittime must not issue an HTTP request, got %d", got)
	}
}

// TestPalworldKickBanUnban exercises the three moderation verbs together
// and checks path + body for each.
func TestPalworldKickBanUnban(t *testing.T) {
	type gotReq struct {
		path string
		body []byte
	}
	var mu sync.Mutex
	var reqs []gotReq

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		reqs = append(reqs, gotReq{path: r.URL.Path, body: b})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewPalworld(host, port, func() (string, error) { return "pw", nil })
	defer func() { _ = client.Close() }()

	if _, err := client.Exec("kick steam_1 griefing"); err != nil {
		t.Fatalf("kick Exec failed: %v", err)
	}
	if _, err := client.Exec("ban steam_2 cheating with mods"); err != nil {
		t.Fatalf("ban Exec failed: %v", err)
	}
	if _, err := client.Exec("unban steam_2"); err != nil {
		t.Fatalf("unban Exec failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(reqs) != 3 {
		t.Fatalf("got %d requests, want 3", len(reqs))
	}

	var kickBody palworldKickBanPayload
	if err := json.Unmarshal(reqs[0].body, &kickBody); err != nil {
		t.Fatalf("decode kick body: %v", err)
	}
	if reqs[0].path != "/v1/api/kick" || kickBody.UserID != "steam_1" || kickBody.Message != "griefing" {
		t.Errorf("kick request = path %q body %+v, want path /v1/api/kick userid steam_1 message griefing", reqs[0].path, kickBody)
	}

	var banBody palworldKickBanPayload
	if err := json.Unmarshal(reqs[1].body, &banBody); err != nil {
		t.Fatalf("decode ban body: %v", err)
	}
	if reqs[1].path != "/v1/api/ban" || banBody.UserID != "steam_2" || banBody.Message != "cheating with mods" {
		t.Errorf("ban request = path %q body %+v, want path /v1/api/ban userid steam_2 message %q", reqs[1].path, banBody, "cheating with mods")
	}

	var unbanBody palworldUnbanPayload
	if err := json.Unmarshal(reqs[2].body, &unbanBody); err != nil {
		t.Fatalf("decode unban body: %v", err)
	}
	if reqs[2].path != "/v1/api/unban" || unbanBody.UserID != "steam_2" {
		t.Errorf("unban request = path %q body %+v, want path /v1/api/unban userid steam_2", reqs[2].path, unbanBody)
	}
}

// TestPalworldMissingArgsNoRequest proves every "requires a ..." parse error
// in Exec returns before any HTTP request is issued.
func TestPalworldMissingArgsNoRequest(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
	}{
		{"empty command", ""},
		{"blank command", "   "},
		{"announce no message", "announce"},
		{"shutdown no args", "shutdown"},
		{"kick no userid", "kick"},
		{"ban no userid", "ban"},
		{"unban no userid", "unban"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var reqCount int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&reqCount, 1)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			host, port := parseHostPort(t, server.URL)
			client := NewPalworld(host, port, func() (string, error) { return "pw", nil })
			defer func() { _ = client.Close() }()

			_, err := client.Exec(tc.cmd)
			if err == nil {
				t.Fatalf("Exec(%q) expected an error", tc.cmd)
			}
			if got := atomic.LoadInt32(&reqCount); got != 0 {
				t.Errorf("Exec(%q) issued %d HTTP requests, want 0", tc.cmd, got)
			}
		})
	}
}

// TestPalworldUnknownVerb covers required case 8: an unrecognized command
// errors without ever dialing the server.
func TestPalworldUnknownVerb(t *testing.T) {
	var reqCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&reqCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewPalworld(host, port, func() (string, error) { return "pw", nil })
	defer func() { _ = client.Close() }()

	_, err := client.Exec("nonsense")
	if err == nil {
		t.Fatal("expected an error for an unknown command")
	}
	if got := atomic.LoadInt32(&reqCount); got != 0 {
		t.Errorf("unknown command issued %d HTTP requests, want 0", got)
	}
}

// TestPalworld401IsAuthAndArmsCooldown covers required case 6: a 401 wraps
// ErrAuth, and a second Exec inside the cooldown window issues no new
// request at all.
func TestPalworld401IsAuthAndArmsCooldown(t *testing.T) {
	var reqCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&reqCount, 1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewPalworld(host, port, func() (string, error) { return "wrongpw", nil })
	client.authFailureCooldown = time.Hour
	defer func() { _ = client.Close() }()

	_, err := client.Exec("save")
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("expected errors.Is(err, ErrAuth), got %v", err)
	}
	afterFirst := atomic.LoadInt32(&reqCount)
	if afterFirst != 1 {
		t.Fatalf("expected exactly 1 request after the first Exec, got %d", afterFirst)
	}

	_, err2 := client.Exec("save")
	if !errors.Is(err2, ErrAuth) {
		t.Fatalf("expected errors.Is(err2, ErrAuth) for the cooldown-cached failure, got %v", err2)
	}
	afterSecond := atomic.LoadInt32(&reqCount)
	if afterSecond != afterFirst {
		t.Errorf("expected NO new HTTP request during the cooldown window, count went from %d to %d", afterFirst, afterSecond)
	}
}

// TestPalworld500IsNotAuthAndDoesNotArmCooldown covers required case 7: a
// 500 must not be classified as ErrAuth and must not arm the cooldown — the
// exact bug this package has hit three times before with other clients.
func TestPalworld500IsNotAuthAndDoesNotArmCooldown(t *testing.T) {
	var reqCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&reqCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewPalworld(host, port, func() (string, error) { return "pw", nil })
	client.authFailureCooldown = time.Hour // would block the 2nd Exec if wrongly armed
	defer func() { _ = client.Close() }()

	_, err := client.Exec("save")
	if err == nil {
		t.Fatal("expected an error from a 500 response")
	}
	if errors.Is(err, ErrAuth) {
		t.Errorf("a 500 must not be reported as an auth failure: %v", err)
	}

	before := atomic.LoadInt32(&reqCount)
	_, _ = client.Exec("save")
	if atomic.LoadInt32(&reqCount) == before {
		t.Error("second Exec issued no request — the cooldown was wrongly armed by a 500")
	}
}

// TestPalworldConnectionRefused covers required case 9: a dead port fails
// fast with a wrapped, non-auth error.
func TestPalworldConnectionRefused(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve a port: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("close reserved listener: %v", err)
	}

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host:port: %v", err)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}

	client := NewPalworld(host, port, func() (string, error) { return "pw", nil })
	client.dialTimeout = 2 * time.Second
	client.requestTimeout = 2 * time.Second
	defer func() { _ = client.Close() }()

	start := time.Now()
	_, err = client.Exec("save")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error connecting to a closed port")
	}
	if errors.Is(err, ErrAuth) {
		t.Errorf("connection refused must not be classified as ErrAuth, got: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("connection refused took too long to fail: %v", elapsed)
	}
}

// TestPalworldPasswordFunctionError proves a failing password resolver
// propagates without ever dialing the server.
func TestPalworldPasswordFunctionError(t *testing.T) {
	client := NewPalworld("127.0.0.1", 1, func() (string, error) {
		return "", fmt.Errorf("password retrieval failed")
	})
	defer func() { _ = client.Close() }()

	_, err := client.Exec("save")
	if err == nil {
		t.Fatal("expected an error when the password function fails")
	}
	if !strings.Contains(err.Error(), "password") {
		t.Errorf("expected 'password' in the error message, got: %v", err)
	}
}

// TestPalworldCloseNeverConnected proves Close is safe to call on a client
// that never issued a request.
func TestPalworldCloseNeverConnected(t *testing.T) {
	client := NewPalworld("127.0.0.1", 9999, func() (string, error) { return "pw", nil })
	if err := client.Close(); err != nil {
		t.Errorf("Close() on a never-used client returned an error: %v", err)
	}
}

// TestPalworldDefaultPort proves port 0 falls back to 8212.
func TestPalworldDefaultPort(t *testing.T) {
	client := NewPalworld("127.0.0.1", 0, func() (string, error) { return "pw", nil })
	defer func() { _ = client.Close() }()
	const want = "http://127.0.0.1:8212"
	if client.baseURL != want {
		t.Errorf("baseURL = %q, want %q", client.baseURL, want)
	}
}
