package rcon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// satisfactoryWireRequest decodes just enough of an inbound request to
// route the fake server's handler and inspect the function-specific data.
type satisfactoryWireRequest struct {
	Function string          `json:"function"`
	Data     json.RawMessage `json:"data"`
}

func decodeSatisfactoryRequest(t *testing.T, r *http.Request) satisfactoryWireRequest {
	t.Helper()
	var req satisfactoryWireRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	return req
}

// TestSatisfactoryLoginThenCommand covers required case 1: the first
// request is PasswordLogin with the correct lowercase fields, the
// RunCommand call carries the returned bearer token, and Exec returns
// commandResult.
func TestSatisfactoryLoginThenCommand(t *testing.T) {
	const testToken = "tok-abc123"
	const testPassword = "s3cret"

	var mu sync.Mutex
	var order []string
	var loginData struct {
		MinimumPrivilegeLevel string `json:"minimumPrivilegeLevel"`
		Password              string `json:"password"`
	}
	var sawAuthHeader string

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		req := decodeSatisfactoryRequest(t, r)

		mu.Lock()
		order = append(order, req.Function)
		mu.Unlock()

		switch req.Function {
		case "PasswordLogin":
			if err := json.Unmarshal(req.Data, &loginData); err != nil {
				t.Fatalf("decode login data: %v", err)
			}
			_, _ = w.Write([]byte(`{"data":{"authenticationToken":"` + testToken + `"}}`))
		case "RunCommand":
			sawAuthHeader = r.Header.Get("Authorization")
			var cmdData struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal(req.Data, &cmdData); err != nil {
				t.Fatalf("decode command data: %v", err)
			}
			if cmdData.Command != "say hi" {
				t.Errorf("command = %q, want %q", cmdData.Command, "say hi")
			}
			_, _ = w.Write([]byte(`{"data":{"commandResult":"hi said"}}`))
		default:
			t.Fatalf("unexpected function %q", req.Function)
		}
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewSatisfactory(host, port, func() (string, error) { return testPassword, nil })
	defer func() { _ = client.Close() }()

	out, err := client.Exec("say hi")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if out != "hi said" {
		t.Errorf("Exec = %q, want %q", out, "hi said")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(order) < 2 || order[0] != "PasswordLogin" || order[1] != "RunCommand" {
		t.Errorf("request order = %v, want [PasswordLogin RunCommand ...]", order)
	}
	if loginData.MinimumPrivilegeLevel != "Administrator" {
		t.Errorf("minimumPrivilegeLevel = %q, want Administrator", loginData.MinimumPrivilegeLevel)
	}
	if loginData.Password != testPassword {
		t.Errorf("password = %q, want %q", loginData.Password, testPassword)
	}
	if sawAuthHeader != "Bearer "+testToken {
		t.Errorf("Authorization header = %q, want %q", sawAuthHeader, "Bearer "+testToken)
	}
}

// TestSatisfactory204NoContentIsSuccess covers required case 2 — the key
// ambiguity test. The runCommand operation's documented success response
// is 204 with no body; a naive implementation that treats any non-200 as
// an error would misreport this as a failure.
func TestSatisfactory204NoContentIsSuccess(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeSatisfactoryRequest(t, r)
		switch req.Function {
		case "PasswordLogin":
			_, _ = w.Write([]byte(`{"data":{"authenticationToken":"tok"}}`))
		case "RunCommand":
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewSatisfactory(host, port, func() (string, error) { return "pw", nil })
	defer func() { _ = client.Close() }()

	out, err := client.Exec("save")
	if err != nil {
		t.Fatalf("Exec on 204 No Content must succeed, got: %v", err)
	}
	if out != "" {
		t.Errorf("Exec on 204 No Content = %q, want empty string", out)
	}
}

// TestSatisfactoryTokenReused covers required case 3: two Exec calls
// produce exactly one PasswordLogin request.
func TestSatisfactoryTokenReused(t *testing.T) {
	var loginCount int32

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeSatisfactoryRequest(t, r)
		switch req.Function {
		case "PasswordLogin":
			atomic.AddInt32(&loginCount, 1)
			_, _ = w.Write([]byte(`{"data":{"authenticationToken":"tok"}}`))
		case "RunCommand":
			_, _ = w.Write([]byte(`{"data":{"commandResult":"ok"}}`))
		}
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewSatisfactory(host, port, func() (string, error) { return "pw", nil })
	defer func() { _ = client.Close() }()

	if _, err := client.Exec("cmd1"); err != nil {
		t.Fatalf("first Exec failed: %v", err)
	}
	if _, err := client.Exec("cmd2"); err != nil {
		t.Fatalf("second Exec failed: %v", err)
	}

	if got := atomic.LoadInt32(&loginCount); got != 1 {
		t.Errorf("login count = %d, want 1 (token should be reused, not re-fetched per command)", got)
	}
}

// TestSatisfactory401TriggersOneRelogin covers required case 4: a 401 on
// RunCommand triggers exactly one re-login and retry, which then
// succeeds.
func TestSatisfactory401TriggersOneRelogin(t *testing.T) {
	var loginCount int32
	var cmdCount int32
	var mu sync.Mutex
	var retryAuthHeader string

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeSatisfactoryRequest(t, r)
		switch req.Function {
		case "PasswordLogin":
			n := atomic.AddInt32(&loginCount, 1)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"authenticationToken":"tok-%d"}}`, n)))
		case "RunCommand":
			n := atomic.AddInt32(&cmdCount, 1)
			if n == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			mu.Lock()
			retryAuthHeader = r.Header.Get("Authorization")
			mu.Unlock()
			_, _ = w.Write([]byte(`{"data":{"commandResult":"ok"}}`))
		}
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewSatisfactory(host, port, func() (string, error) { return "pw", nil })
	defer func() { _ = client.Close() }()

	out, err := client.Exec("status")
	if err != nil {
		t.Fatalf("Exec should succeed after one re-login+retry: %v", err)
	}
	if out != "ok" {
		t.Errorf("Exec = %q, want %q", out, "ok")
	}
	if got := atomic.LoadInt32(&loginCount); got != 2 {
		t.Errorf("login count = %d, want exactly 2 (initial + one re-login)", got)
	}
	if got := atomic.LoadInt32(&cmdCount); got != 2 {
		t.Errorf("RunCommand count = %d, want exactly 2 (initial + one retry)", got)
	}
	// The retry must carry the FRESH token (tok-2), not the stale one that
	// was just rejected — otherwise the re-login bought nothing.
	mu.Lock()
	defer mu.Unlock()
	if retryAuthHeader != "Bearer tok-2" {
		t.Errorf("retry Authorization = %q, want %q", retryAuthHeader, "Bearer tok-2")
	}
}

func TestSatisfactoryServerErrorWithEnvelopeIsNotAuth(t *testing.T) {
	// A 5xx that happens to carry an error envelope is a server problem, not
	// a rejected password. It must surface as a plain error — never ErrAuth —
	// and must NOT arm the auth cooldown, or a transient outage would freeze
	// the pollers and read as a bad credential.
	var reqCount int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&reqCount, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"errorCode":"server_busy","errorMessage":"try later"}`))
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewSatisfactory(host, port, func() (string, error) { return "pw", nil })
	client.authFailureCooldown = time.Hour // would block the 2nd Exec if armed
	defer func() { _ = client.Close() }()

	_, err := client.Exec("status")
	if err == nil {
		t.Fatal("expected an error from a 503 response")
	}
	if errors.Is(err, ErrAuth) {
		t.Errorf("a 5xx must not be reported as an auth failure: %v", err)
	}

	// The cooldown must NOT have been armed: a second Exec must still reach
	// the server rather than short-circuiting on a phantom auth failure.
	before := atomic.LoadInt32(&reqCount)
	_, _ = client.Exec("status")
	if atomic.LoadInt32(&reqCount) == before {
		t.Error("second Exec issued no request — the cooldown was wrongly armed by a 5xx")
	}
}

// TestSatisfactoryPersistent401ReturnsErrAuth covers required case 5: a
// 401 that persists even after the single re-login surfaces as ErrAuth
// without looping.
func TestSatisfactoryPersistent401ReturnsErrAuth(t *testing.T) {
	var loginCount int32
	var cmdCount int32

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeSatisfactoryRequest(t, r)
		switch req.Function {
		case "PasswordLogin":
			atomic.AddInt32(&loginCount, 1)
			_, _ = w.Write([]byte(`{"data":{"authenticationToken":"tok"}}`))
		case "RunCommand":
			atomic.AddInt32(&cmdCount, 1)
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewSatisfactory(host, port, func() (string, error) { return "pw", nil })
	defer func() { _ = client.Close() }()

	_, err := client.Exec("status")
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("expected errors.Is(err, ErrAuth), got %v", err)
	}

	if got := atomic.LoadInt32(&loginCount); got != 2 {
		t.Errorf("login count = %d, want exactly 2 (bounded — no infinite re-login loop)", got)
	}
	if got := atomic.LoadInt32(&cmdCount); got != 2 {
		t.Errorf("RunCommand count = %d, want exactly 2 (bounded)", got)
	}
}

// TestSatisfactoryBadPasswordAtLogin covers required case 6: PasswordLogin
// itself returns an error envelope, and Exec never even reaches
// RunCommand.
func TestSatisfactoryBadPasswordAtLogin(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeSatisfactoryRequest(t, r)
		if req.Function != "PasswordLogin" {
			t.Fatalf("unexpected function %q; should never reach RunCommand on a rejected login", req.Function)
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errorCode":"wrong_password","errorMessage":"Incorrect password"}`))
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewSatisfactory(host, port, func() (string, error) { return "wrongpw", nil })
	defer func() { _ = client.Close() }()

	_, err := client.Exec("status")
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("expected errors.Is(err, ErrAuth), got %v", err)
	}
}

// TestSatisfactory200WithErrorCodeIsError covers required case 7: the
// spec documents "200 Ok - Error", so a 2xx status never proves success.
// A naive implementation would return the raw error body as if it were
// command output.
func TestSatisfactory200WithErrorCodeIsError(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeSatisfactoryRequest(t, r)
		switch req.Function {
		case "PasswordLogin":
			_, _ = w.Write([]byte(`{"data":{"authenticationToken":"tok"}}`))
		case "RunCommand":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"errorCode":"unknown_command","errorMessage":"no such command"}`))
		}
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewSatisfactory(host, port, func() (string, error) { return "pw", nil })
	defer func() { _ = client.Close() }()

	out, err := client.Exec("bogus")
	if err == nil {
		t.Fatalf("expected an error, got success with output %q", out)
	}
	if errors.Is(err, ErrAuth) {
		t.Errorf("a 200 errorCode body is not an auth failure, got: %v", err)
	}
	if !strings.Contains(err.Error(), "unknown_command") {
		t.Errorf("expected the errorCode in the error message, got: %v", err)
	}
	if out != "" {
		t.Errorf("output on a 200-with-errorCode response must be empty, got %q", out)
	}
}

// TestSatisfactoryAuthFailureCooldown covers required case 8: after a
// rejected password, a second Exec inside the cooldown window must not
// issue another HTTP request at all.
func TestSatisfactoryAuthFailureCooldown(t *testing.T) {
	var requestCount int32

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errorCode":"wrong_password","errorMessage":"nope"}`))
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewSatisfactory(host, port, func() (string, error) { return "wrongpw", nil })
	client.authFailureCooldown = 200 * time.Millisecond
	defer func() { _ = client.Close() }()

	_, err1 := client.Exec("cmd1")
	if !errors.Is(err1, ErrAuth) {
		t.Fatalf("first Exec expected ErrAuth, got %v", err1)
	}
	afterFirst := atomic.LoadInt32(&requestCount)
	if afterFirst != 1 {
		t.Fatalf("expected exactly 1 request after first Exec, got %d", afterFirst)
	}

	_, err2 := client.Exec("cmd2")
	if !errors.Is(err2, ErrAuth) {
		t.Fatalf("second Exec (within cooldown) expected ErrAuth, got %v", err2)
	}
	afterSecond := atomic.LoadInt32(&requestCount)
	if afterSecond != afterFirst {
		t.Errorf("expected NO new HTTP request during the cooldown window, count went from %d to %d", afterFirst, afterSecond)
	}
}

// TestSatisfactoryConnectionRefused covers part of required case 9: a
// closed port must fail fast with a plain (non-auth) error, not hang.
func TestSatisfactoryConnectionRefused(t *testing.T) {
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

	client := NewSatisfactory(host, port, func() (string, error) { return "pw", nil })
	client.dialTimeout = 2 * time.Second
	client.requestTimeout = 2 * time.Second
	defer func() { _ = client.Close() }()

	start := time.Now()
	_, err = client.Exec("status")
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

// TestSatisfactoryMalformedJSONBody covers part of required case 9: a
// non-JSON RunCommand response body must error, not panic, and must not
// be misclassified as an auth failure.
func TestSatisfactoryMalformedJSONBody(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeSatisfactoryRequest(t, r)
		switch req.Function {
		case "PasswordLogin":
			_, _ = w.Write([]byte(`{"data":{"authenticationToken":"tok"}}`))
		case "RunCommand":
			_, _ = w.Write([]byte(`this is not json`))
		}
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewSatisfactory(host, port, func() (string, error) { return "pw", nil })
	defer func() { _ = client.Close() }()

	_, err := client.Exec("status")
	if err == nil {
		t.Fatal("expected an error for a malformed JSON response body")
	}
	if errors.Is(err, ErrAuth) {
		t.Errorf("a malformed body is not proof of a bad password, got: %v", err)
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected 'unmarshal' in the error message, got: %v", err)
	}
}

// TestSatisfactoryLoginMalformedJSON exercises the same unmarshal-error
// path on the PasswordLogin response.
func TestSatisfactoryLoginMalformedJSON(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewSatisfactory(host, port, func() (string, error) { return "pw", nil })
	defer func() { _ = client.Close() }()

	_, err := client.Exec("status")
	if err == nil {
		t.Fatal("expected an error for a malformed login response body")
	}
	if errors.Is(err, ErrAuth) {
		t.Errorf("a malformed login response is not proof of a bad password, got: %v", err)
	}
}

// TestSatisfactoryNon2xxStatusWithoutErrorEnvelope covers the remainder of
// required case 9: a bare 500 on RunCommand (no error envelope at all)
// must surface as a plain error, not a misread success or an auth error.
func TestSatisfactoryNon2xxStatusWithoutErrorEnvelope(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeSatisfactoryRequest(t, r)
		switch req.Function {
		case "PasswordLogin":
			_, _ = w.Write([]byte(`{"data":{"authenticationToken":"tok"}}`))
		case "RunCommand":
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewSatisfactory(host, port, func() (string, error) { return "pw", nil })
	defer func() { _ = client.Close() }()

	_, err := client.Exec("status")
	if err == nil {
		t.Fatal("expected an error for a bare 500 response")
	}
	if errors.Is(err, ErrAuth) {
		t.Errorf("a bare 500 must not be classified as ErrAuth, got: %v", err)
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected the status code in the error message, got: %v", err)
	}
}

// TestSatisfactoryLoginServerError mirrors the above for PasswordLogin
// itself: a bare 500 is a server problem, not proof of a bad password, so
// it must not arm the auth-failure cooldown.
func TestSatisfactoryLoginServerError(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewSatisfactory(host, port, func() (string, error) { return "pw", nil })
	defer func() { _ = client.Close() }()

	_, err := client.Exec("status")
	if err == nil {
		t.Fatal("expected an error for a bare 500 login response")
	}
	if errors.Is(err, ErrAuth) {
		t.Errorf("a bare 500 during login is a server problem, not a bad password: %v", err)
	}
}

// TestSatisfactoryPasswordFunctionError proves a failing password
// resolver propagates without ever dialing the server.
func TestSatisfactoryPasswordFunctionError(t *testing.T) {
	client := NewSatisfactory("127.0.0.1", 1, func() (string, error) {
		return "", fmt.Errorf("password retrieval failed")
	})
	defer func() { _ = client.Close() }()

	_, err := client.Exec("status")
	if err == nil {
		t.Fatal("expected an error when the password function fails")
	}
	if !strings.Contains(err.Error(), "password") {
		t.Errorf("expected 'password' in the error message, got: %v", err)
	}
}

// TestSatisfactoryCloseNeverConnected proves Close is safe to call on a
// client that never issued a request.
func TestSatisfactoryCloseNeverConnected(t *testing.T) {
	client := NewSatisfactory("127.0.0.1", 9999, func() (string, error) { return "pw", nil })
	if err := client.Close(); err != nil {
		t.Errorf("Close() on a never-connected client returned an error: %v", err)
	}
}

func TestIsLoopbackHost(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"127.0.0.1", true},
		{"localhost", true},
		{"LocalHost", true},
		{"::1", true},
		{"127.0.0.53", true},
		{"10.0.0.5", false},
		{"192.168.1.1", false},
		{"game.default.svc.cluster.local", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isLoopbackHost(tc.host); got != tc.want {
			t.Errorf("isLoopbackHost(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

func TestSatisfactoryLoopbackTrustsSelfSignedCert(t *testing.T) {
	// httptest binds loopback and presents a self-signed cert. The client
	// must accept it, proving the loopback path skips verification.
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeSatisfactoryRequest(t, r)
		switch req.Function {
		case "PasswordLogin":
			_, _ = w.Write([]byte(`{"data":{"authenticationToken":"t"}}`))
		default:
			_, _ = w.Write([]byte(`{"data":{"commandResult":"ok"}}`))
		}
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	client := NewSatisfactory(host, port, func() (string, error) { return "pw", nil })
	defer func() { _ = client.Close() }()
	if _, err := client.Exec("x"); err != nil {
		t.Fatalf("loopback client should trust the test server's self-signed cert: %v", err)
	}
}

func TestSatisfactoryNonLoopbackVerifiesTLS(t *testing.T) {
	// A non-loopback host must NOT skip verification: constructed against a
	// non-loopback name, the transport keeps verification on, so a self-signed
	// cert is rejected. We redirect the dial to the loopback test server so
	// only the TLS decision — not reachability — is under test.
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"authenticationToken":"t"}}`))
	}))
	defer server.Close()
	host, port := parseHostPort(t, server.URL)

	client := NewSatisfactory("example.invalid", port, func() (string, error) { return "pw", nil })
	client.requestTimeout = 2 * time.Second
	defer func() { _ = client.Close() }()

	tr := client.httpClient.Transport.(*http.Transport)
	if tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("non-loopback host must not set InsecureSkipVerify")
	}
	tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(host, fmt.Sprint(port)))
	}

	_, err := client.Exec("x")
	if err == nil {
		t.Fatal("expected a TLS verification error against a self-signed cert on a non-loopback host")
	}
	if errors.Is(err, ErrAuth) {
		t.Errorf("a TLS failure must not be reported as an auth failure: %v", err)
	}
}
