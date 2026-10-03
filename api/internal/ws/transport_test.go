package ws

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
)

func testDirectTransport(srv *httptest.Server) *directAgentTransport {
	transport := newDirectAgentTransport(srv.Client().Transport.(*http.Transport).TLSClientConfig, 0)
	transport.hostFn = func(_, _ string) string { return strings.TrimPrefix(srv.URL, "https://") }
	return transport
}

func TestDirectAgentTransportHTTP(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != "/files/write" || req.URL.Query().Get("path") != "config/server.ini" {
			t.Errorf("unexpected operation: %s %s", req.Method, req.URL)
		}
		if req.Header.Get("Cookie") != "" || req.Header.Get("Authorization") != "" || req.Header.Get("X-Gameplane-CSRF") != "" {
			t.Error("browser credentials reached the agent")
		}
		if req.Header.Get("Content-Type") != "text/plain" {
			t.Error("content type not forwarded")
		}
		body, err := io.ReadAll(req.Body)
		if err != nil || string(body) != "enabled=true" {
			t.Errorf("body = %q, error = %v", body, err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	resp, err := testDirectTransport(srv).Do(t.Context(), agentRequest{
		target: agentTarget{name: "alpha", namespace: "gameplane-games"},
		method: http.MethodPost, path: "/files/write", rawQuery: "path=config%2Fserver.ini",
		header: http.Header{
			"Content-Type": {"text/plain"}, "Cookie": {"session=private"},
			"Authorization": {"Bearer private"}, "X-Gameplane-Csrf": {"private"},
		},
		body: strings.NewReader("enabled=true"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestDirectAgentTransportRejectsInvalidTargets(t *testing.T) {
	transport := newDirectAgentTransport(&tls.Config{}, 0)
	transport.hostFn = func(_, _ string) string { t.Fatal("invalid target reached host resolution"); return "" }
	for _, target := range []agentTarget{
		{name: "../other", namespace: "games"},
		{name: "alpha", namespace: "games.attacker.example"},
		{name: "", namespace: "games"},
	} {
		resp, err := transport.Do(t.Context(), agentRequest{target: target, method: http.MethodGet, path: "/mods"})
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if err == nil {
			t.Errorf("HTTP accepted invalid target %+v", target)
		}
		conn, resp, err := transport.Dial(t.Context(), target, "/console")
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if conn != nil {
			_ = conn.CloseNow()
		}
		if err == nil {
			t.Errorf("WebSocket accepted invalid target %+v", target)
		}
	}
}

func TestDirectAgentTransportDoesNotFollowRedirects(t *testing.T) {
	destination := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("agent redirect reached another destination")
	}))
	defer destination.Close()
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	resp, err := testDirectTransport(source).Do(t.Context(), agentRequest{
		target: agentTarget{name: "alpha", namespace: "games"}, method: http.MethodPost,
		path: "/files/write", body: strings.NewReader("private"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestDirectAgentTransportWebSocket(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/console" {
			t.Errorf("path = %q", req.URL.Path)
		}
		conn, err := websocket.Accept(w, req, nil)
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer conn.CloseNow()
		typ, body, err := conn.Read(req.Context())
		if err != nil {
			t.Errorf("read: %v", err)
			return
		}
		if err := conn.Write(req.Context(), typ, body); err != nil {
			t.Errorf("write: %v", err)
		}
	}))
	defer srv.Close()
	conn, resp, err := testDirectTransport(srv).Dial(t.Context(), agentTarget{name: "alpha", namespace: "games"}, "/console")
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if err := conn.Write(t.Context(), websocket.MessageText, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	_, body, err := conn.Read(t.Context())
	if err != nil || string(body) != "hello" {
		t.Fatalf("body = %q, error = %v", body, err)
	}
}

type recordingAgentTransport struct{ operation agentRequest }

func (r *recordingAgentTransport) Do(_ context.Context, operation agentRequest) (*http.Response, error) {
	r.operation = operation
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("[]"))}, nil
}
func (*recordingAgentTransport) Dial(context.Context, agentTarget, string) (*websocket.Conn, *http.Response, error) {
	panic("unexpected WebSocket operation")
}

func TestHTTPProxyUsesAgentTransport(t *testing.T) {
	transport := &recordingAgentTransport{}
	p := &proxy{transport: transport}
	router := chi.NewRouter()
	router.Get("/servers/{name}/mods", p.httpProxy("/mods"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/servers/alpha/mods?path=plugins", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	got := transport.operation
	if got.target.name != "alpha" || got.target.namespace != "gameplane-games" || got.path != "/mods" || got.rawQuery != "path=plugins" {
		t.Fatalf("operation = %+v", got)
	}
}

func TestAgentClientUsesAgentTransport(t *testing.T) {
	transport := &recordingAgentTransport{}
	client := &AgentClient{transport: transport}
	var out []string
	if err := client.GetJSON(t.Context(), "alpha", "games", "/mods", &out); err != nil {
		t.Fatal(err)
	}
	got := transport.operation
	if got.target.name != "alpha" || got.target.namespace != "games" || got.method != http.MethodGet || got.path != "/mods" {
		t.Fatalf("operation = %+v", got)
	}
}
