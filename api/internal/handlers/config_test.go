package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ValgulNecron/gameplane/api/internal/db"
)

// newTestStore opens an in-memory SQLite store and runs migrations.
// Each test gets its own store so PUT/GET cases don't bleed.
func newTestStore(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.Open(context.Background(), "sqlite", "file::memory:?_pragma=journal_mode(WAL)&cache=shared")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store
}

func newConfigServer(t *testing.T) (*httptest.Server, *db.Store) {
	t.Helper()
	store := newTestStore(t)
	r := chi.NewRouter()
	MountConfig(r, store)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, store
}

func doReq(t *testing.T, method, url string, body any) (int, []byte) {
	t.Helper()
	var buf io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, url, buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

func TestConfig_GetEmpty(t *testing.T) {
	srv, _ := newConfigServer(t)
	status, body := doReq(t, "GET", srv.URL+"/admin/config", nil)
	if status != 200 {
		t.Fatalf("status = %d, want 200; body=%s", status, body)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if len(got) != 0 {
		t.Fatalf("want empty map, got %v", got)
	}
}

func TestConfig_PutThenGet(t *testing.T) {
	srv, _ := newConfigServer(t)

	in := generalCfg{
		InstanceName:     "homelab-01",
		ExternalURL:      "https://kestrel.example.dev",
		DefaultNamespace: "kestrel-games",
	}
	status, body := doReq(t, "PUT", srv.URL+"/admin/config/general", in)
	if status != 200 {
		t.Fatalf("PUT status = %d, want 200; body=%s", status, body)
	}

	status, body = doReq(t, "GET", srv.URL+"/admin/config", nil)
	if status != 200 {
		t.Fatalf("GET status = %d", status)
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(body, &all); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	raw, ok := all["general"]
	if !ok {
		t.Fatalf("missing general key; got %v", all)
	}
	var got generalCfg
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal general: %v", err)
	}
	if got != in {
		t.Fatalf("got %+v want %+v", got, in)
	}
}

func TestConfig_PutUpsert(t *testing.T) {
	srv, _ := newConfigServer(t)
	first := telemetryCfg{SendMetrics: true}
	if s, b := doReq(t, "PUT", srv.URL+"/admin/config/telemetry", first); s != 200 {
		t.Fatalf("first PUT status %d body=%s", s, b)
	}
	second := telemetryCfg{SendMetrics: false}
	if s, b := doReq(t, "PUT", srv.URL+"/admin/config/telemetry", second); s != 200 {
		t.Fatalf("second PUT status %d body=%s", s, b)
	}
	_, body := doReq(t, "GET", srv.URL+"/admin/config", nil)
	var all map[string]telemetryCfg
	_ = json.Unmarshal(body, &all)
	if all["telemetry"].SendMetrics {
		t.Fatalf("expected upsert to flip SendMetrics to false; got %+v", all["telemetry"])
	}
}

func TestConfig_PutUnknownSection(t *testing.T) {
	srv, _ := newConfigServer(t)
	status, _ := doReq(t, "PUT", srv.URL+"/admin/config/wat", map[string]any{"x": 1})
	if status != 400 {
		t.Fatalf("status = %d, want 400", status)
	}
}

func TestConfig_PutBadJSON(t *testing.T) {
	srv, _ := newConfigServer(t)
	req, _ := http.NewRequestWithContext(t.Context(), "PUT", srv.URL+"/admin/config/general", bytes.NewReader([]byte("{not json")))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 422 {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
}

func TestConfig_PutValidationFailure(t *testing.T) {
	srv, _ := newConfigServer(t)
	cases := []struct {
		name    string
		section string
		body    any
	}{
		{"general missing instanceName", "general", generalCfg{DefaultNamespace: "kestrel-games"}},
		{"general bad URL", "general", generalCfg{InstanceName: "x", DefaultNamespace: "kestrel-games", ExternalURL: "not a url"}},
		{"general bad namespace", "general", generalCfg{InstanceName: "x", DefaultNamespace: "Bad_Name"}},
		{"updates bad channel", "updates", updatesCfg{Channel: "wat"}},
		{"auth bad kind", "auth", authCfg{Providers: []authProvider{{Name: "x", Kind: "wat"}}}},
		{"auth duplicate names", "auth", authCfg{Providers: []authProvider{{Name: "x", Kind: "local"}, {Name: "x", Kind: "oidc"}}}},
		{"notif bad kind", "notifications", notifCfg{Sinks: []notifSink{{Name: "x", Kind: "telegram"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := doReq(t, "PUT", srv.URL+"/admin/config/"+tc.section, tc.body)
			if status != 422 {
				t.Fatalf("status = %d, want 422; body=%s", status, body)
			}
		})
	}
}

func TestConfig_PutEmptyBody(t *testing.T) {
	srv, _ := newConfigServer(t)
	req, _ := http.NewRequestWithContext(t.Context(), "PUT", srv.URL+"/admin/config/general", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestConfig_GetSkipsUnknownSections(t *testing.T) {
	srv, store := newConfigServer(t)
	// Insert a row with a key the API doesn't expose anymore (e.g. a
	// removed section). GET should skip it rather than return it as
	// json.RawMessage.
	_, err := store.DB.Exec(`INSERT INTO config(key, value) VALUES('legacy', '{"foo":1}')`)
	if err != nil {
		t.Fatalf("insert legacy: %v", err)
	}
	_, body := doReq(t, "GET", srv.URL+"/admin/config", nil)
	var all map[string]json.RawMessage
	_ = json.Unmarshal(body, &all)
	if _, ok := all["legacy"]; ok {
		t.Fatalf("legacy section should be filtered out; got %v", all)
	}
}
