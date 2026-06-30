package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBuildSyslog(t *testing.T) {
	ts := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	got := buildSyslog(16*8+6, ts, "host1", "gameplane-audit", `{"actor":"alice"}`)
	want := `<134>1 2026-06-30T12:00:00.000Z host1 gameplane-audit - - - {"actor":"alice"}`
	if got != want {
		t.Errorf("buildSyslog =\n %q\nwant\n %q", got, want)
	}
}

func TestBuildSyslog_DefaultsAndSingleLine(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	got := buildSyslog(13, ts, "", "", "line1\nline2\rline3")
	// Empty host/app become "-"; embedded newlines/CRs collapse to spaces.
	want := `<13>1 2026-01-02T03:04:05.000Z - - - - - line1 line2 line3`
	if got != want {
		t.Errorf("buildSyslog =\n %q\nwant\n %q", got, want)
	}
}

func TestFrameFor(t *testing.T) {
	msg := "<134>1 ... hello"
	if got := string(frameFor("tcp", msg)); got != strconv.Itoa(len(msg))+" "+msg {
		t.Errorf("tcp frame = %q", got)
	}
	if got := string(frameFor("udp", msg)); got != msg {
		t.Errorf("udp frame = %q", got)
	}
}

func TestNewServer_Validation(t *testing.T) {
	base := func() config {
		return config{
			syslogAddr: "127.0.0.1:514", network: "tcp",
			appName: "x", facility: "local0", severity: "info",
			dialTimeout: time.Second,
		}
	}
	t.Run("ok computes pri", func(t *testing.T) {
		s, err := newServer(base())
		if err != nil {
			t.Fatalf("newServer: %v", err)
		}
		if s.pri != 16*8+6 { // local0(16), info(6)
			t.Errorf("pri = %d, want %d", s.pri, 16*8+6)
		}
	})
	t.Run("missing addr", func(t *testing.T) {
		c := base()
		c.syslogAddr = ""
		if _, err := newServer(c); err == nil {
			t.Error("want error for missing syslog addr")
		}
	})
	t.Run("bad network", func(t *testing.T) {
		c := base()
		c.network = "sctp"
		if _, err := newServer(c); err == nil {
			t.Error("want error for bad network")
		}
	})
	t.Run("tls requires tcp", func(t *testing.T) {
		c := base()
		c.network = "udp"
		c.useTLS = true
		if _, err := newServer(c); err == nil {
			t.Error("want error for tls over udp")
		}
	})
	t.Run("bad facility", func(t *testing.T) {
		c := base()
		c.facility = "nope"
		if _, err := newServer(c); err == nil {
			t.Error("want error for bad facility")
		}
	})
	t.Run("bad severity", func(t *testing.T) {
		c := base()
		c.severity = "nope"
		if _, err := newServer(c); err == nil {
			t.Error("want error for bad severity")
		}
	})
}

// tcpSyslogSink starts a one-shot TCP listener that returns the first frame it
// reads. Returns its address and a channel delivering the received bytes.
func tcpSyslogSink(t *testing.T) (string, <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	got := make(chan string, 1)
	go func() {
		defer ln.Close()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		n, _ := conn.Read(buf)
		got <- string(buf[:n])
	}()
	return ln.Addr().String(), got
}

func TestHandle_ForwardsToTCPSyslog(t *testing.T) {
	addr, got := tcpSyslogSink(t)
	s, err := newServer(config{
		syslogAddr: addr, network: "tcp", appName: "gameplane-audit",
		facility: "local0", severity: "info", hostname: "h", dialTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}

	srv := httptest.NewServer(s.routes())
	defer srv.Close()

	body := strings.NewReader(`{"actor":"admin","path":"/api/v1/servers"}`)
	resp, err := http.Post(srv.URL+"/", "application/json", body)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	select {
	case frame := <-got:
		// Octet-counting prefix, RFC 5424 header, and the JSON payload.
		if !strings.Contains(frame, "<134>1 ") {
			t.Errorf("missing PRI/version header: %q", frame)
		}
		if !strings.Contains(frame, "gameplane-audit") {
			t.Errorf("missing app-name: %q", frame)
		}
		if !strings.Contains(frame, `{"actor":"admin","path":"/api/v1/servers"}`) {
			t.Errorf("missing JSON payload: %q", frame)
		}
		prefix := frame[:strings.IndexByte(frame, ' ')]
		if _, err := strconv.Atoi(prefix); err != nil {
			t.Errorf("octet-count prefix not numeric: %q", prefix)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("syslog sink received nothing")
	}
}

func TestHandle_ForwardsToUDPSyslog(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	defer pc.Close()
	got := make(chan string, 1)
	go func() {
		buf := make([]byte, 4096)
		n, _, err := pc.ReadFrom(buf)
		if err == nil {
			got <- string(buf[:n])
		}
	}()

	s, err := newServer(config{
		syslogAddr: pc.LocalAddr().String(), network: "udp", appName: "gp",
		facility: "local0", severity: "info", hostname: "h", dialTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"k":"v"}`))
	s.handle(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
	select {
	case msg := <-got:
		// UDP carries the bare message (no octet-count prefix).
		if !strings.HasPrefix(msg, "<134>1 ") || !strings.Contains(msg, `{"k":"v"}`) {
			t.Errorf("udp message = %q", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("udp sink received nothing")
	}
}

func TestHandle_AuthRequired(t *testing.T) {
	s := &server{authHeader: "Bearer secret", network: "tcp", fwd: newForwarder("tcp", "127.0.0.1:1", false, time.Second)}

	t.Run("missing token", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		s.handle(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rr.Code)
		}
	})
	t.Run("wrong token", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer nope")
		s.handle(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rr.Code)
		}
	})
}

func TestHandle_MethodAndBody(t *testing.T) {
	s := &server{network: "tcp", fwd: newForwarder("tcp", "127.0.0.1:1", false, time.Second)}

	t.Run("GET rejected", func(t *testing.T) {
		rr := httptest.NewRecorder()
		s.handle(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 405", rr.Code)
		}
	})
	t.Run("empty body", func(t *testing.T) {
		rr := httptest.NewRecorder()
		s.handle(rr, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("   ")))
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rr.Code)
		}
	})
}

func TestHandle_ForwardFailureIs502(t *testing.T) {
	// Point at a closed port so the dial fails.
	s := &server{network: "tcp", appName: "x", hostname: "h",
		fwd: newForwarder("tcp", "127.0.0.1:1", false, 200*time.Millisecond)}
	rr := httptest.NewRecorder()
	s.handle(rr, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"k":"v"}`)))
	if rr.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rr.Code)
	}
}

func TestForwarder_ReusesConnAndReconnects(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	received := make(chan string, 4)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if n > 0 {
						received <- string(buf[:n])
					}
					if err != nil {
						return
					}
				}
			}(conn)
		}
	}()
	defer ln.Close()

	f := newForwarder("tcp", ln.Addr().String(), false, time.Second)
	if err := f.send([]byte("one")); err != nil {
		t.Fatalf("send one: %v", err)
	}
	if err := f.send([]byte("two")); err != nil {
		t.Fatalf("send two: %v", err)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-received:
		case <-time.After(2 * time.Second):
			t.Fatal("did not receive both frames")
		}
	}
}

func TestHealthz(t *testing.T) {
	s := &server{network: "tcp", fwd: newForwarder("tcp", "127.0.0.1:1", false, time.Second)}
	rr := httptest.NewRecorder()
	s.routes().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("healthz status = %d", rr.Code)
	}
	b, _ := io.ReadAll(rr.Body)
	if string(b) != "ok" {
		t.Errorf("healthz body = %q", b)
	}
}

func TestEnvOr(t *testing.T) {
	t.Setenv("BRIDGE_TEST_KEY", "set")
	if got := envOr("BRIDGE_TEST_KEY", "fallback"); got != "set" {
		t.Errorf("envOr set = %q", got)
	}
	if got := envOr("BRIDGE_TEST_UNSET_KEY", "fallback"); got != "fallback" {
		t.Errorf("envOr unset = %q", got)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	for _, k := range []string{
		"LISTEN_ADDR", "SYSLOG_ADDR", "SYSLOG_NETWORK", "SYSLOG_TLS",
		"APP_NAME", "FACILITY", "SEVERITY", "SYSLOG_HOSTNAME", "AUTH_HEADER",
	} {
		// t.Setenv registers restoration of the prior value; immediately
		// unsetting then exercises the "env absent → fallback" path.
		t.Setenv(k, "")
		if err := os.Unsetenv(k); err != nil {
			t.Fatalf("unset %s: %v", k, err)
		}
	}
	cfg := loadConfig()
	if cfg.listen != ":8514" || cfg.network != "tcp" || cfg.appName != "gameplane-audit" ||
		cfg.facility != "local0" || cfg.severity != "info" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
}
