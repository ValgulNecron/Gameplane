package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testAgentClient points an AgentClient at an httptest TLS server, standing
// in for the mTLS-dialed sidecar.
func testAgentClient(srv *httptest.Server) *AgentClient {
	return &AgentClient{
		http:   srv.Client(),
		hostFn: func(_, _ string) string { return strings.TrimPrefix(srv.URL, "https://") },
	}
}

func TestAgentClientGetJSON(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mods" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("accept = %q", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":"a.jar","size":3}]`))
	}))
	defer srv.Close()

	var out []struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}
	if err := testAgentClient(srv).GetJSON(context.Background(), "gs", "ns", "/mods", &out); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if len(out) != 1 || out[0].Name != "a.jar" || out[0].Size != 3 {
		t.Fatalf("out = %+v", out)
	}
}

func TestAgentClientGetJSON_UpstreamStatus(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	}))
	defer srv.Close()
	var out any
	if err := testAgentClient(srv).GetJSON(context.Background(), "gs", "ns", "/mods", &out); err == nil {
		t.Fatal("want error on non-200")
	}
}

func TestAgentClientGetJSON_BadBody(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()
	var out any
	if err := testAgentClient(srv).GetJSON(context.Background(), "gs", "ns", "/mods", &out); err == nil {
		t.Fatal("want decode error")
	}
}

func TestNewAgentClient_MissingMaterial(t *testing.T) {
	if _, err := NewAgentClient("", "", ""); err == nil {
		t.Fatal("want error when mTLS material is missing")
	}
}
