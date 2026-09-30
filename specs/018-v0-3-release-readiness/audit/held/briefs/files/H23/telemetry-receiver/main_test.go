package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testServer(t *testing.T, cfg config) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(newServer(cfg).routes())
	t.Cleanup(srv.Close)
	return srv
}

func post(t *testing.T, srv *httptest.Server, body string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/ingest", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	return resp
}

func metrics(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/metrics", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get metrics: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func TestIngestCountsReport(t *testing.T) {
	srv := testServer(t, config{})
	resp := post(t, srv, `{"version":"0.2.0-beta.5","servers":3,"templates":7}`, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	m := metrics(t, srv)
	if !strings.Contains(m, `gameplane_telemetry_reports_total{version="0.2.0-beta.5"} 1`) {
		t.Fatalf("reports counter missing from metrics:\n%s", m)
	}
	if !strings.Contains(m, `gameplane_telemetry_servers_bucket{le="5"} 1`) {
		t.Fatalf("servers histogram missing:\n%s", m)
	}
	if !strings.Contains(m, `gameplane_telemetry_templates_sum 7`) {
		t.Fatalf("templates histogram missing:\n%s", m)
	}
}

func TestIngestSanitizesVersionLabel(t *testing.T) {
	srv := testServer(t, config{})
	// A hostile version string must not become a raw label value.
	resp := post(t, srv, `{"version":"<script>alert(1)</script>","servers":0,"templates":0}`, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	resp2 := post(t, srv, `{"version":"`+strings.Repeat("x", 200)+`","servers":0,"templates":0}`, nil)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNoContent {
		t.Fatalf("long version: status not 204")
	}
	m := metrics(t, srv)
	if !strings.Contains(m, `gameplane_telemetry_reports_total{version="invalid"} 2`) {
		t.Fatalf("sanitized counter missing:\n%s", m)
	}
	if strings.Contains(m, "script") {
		t.Fatal("hostile version string leaked into metrics output")
	}
}

func TestIngestRejects(t *testing.T) {
	srv := testServer(t, config{})
	cases := []struct {
		name string
		body string
		want int
	}{
		{"bad json", `{not json`, http.StatusBadRequest},
		{"unknown field", `{"version":"1","servers":1,"templates":1,"hostname":"leaky"}`, http.StatusBadRequest},
		{"negative servers", `{"version":"1","servers":-1,"templates":0}`, http.StatusBadRequest},
		{"negative templates", `{"version":"1","servers":0,"templates":-2}`, http.StatusBadRequest},
		{"oversized", `{"version":"` + strings.Repeat("a", maxBody+1) + `"}`, http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := post(t, srv, tc.body, nil)
			defer resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}
	// Nothing above must have been counted.
	if m := metrics(t, srv); strings.Contains(m, "gameplane_telemetry_reports_total{") {
		t.Fatalf("rejected reports were counted:\n%s", m)
	}
}

func TestIngestMethodNotAllowed(t *testing.T) {
	srv := testServer(t, config{})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/ingest", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
}

func TestIngestAuth(t *testing.T) {
	srv := testServer(t, config{authToken: "Bearer s3cret"})
	body := `{"version":"1.0.0","servers":1,"templates":1}`
	resp := post(t, srv, body, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no auth: status = %d, want 401", resp.StatusCode)
	}
	resp = post(t, srv, body, map[string]string{"Authorization": "Bearer wrong"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad auth: status = %d, want 401", resp.StatusCode)
	}
	resp = post(t, srv, body, map[string]string{"Authorization": "Bearer s3cret"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("good auth: status = %d, want 204", resp.StatusCode)
	}
}

func TestHealthz(t *testing.T) {
	srv := testServer(t, config{})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/healthz", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(b) != "ok" {
		t.Fatalf("healthz = %d %q", resp.StatusCode, b)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg := loadConfig()
	if cfg.listen != ":8080" || cfg.authToken != "" {
		t.Fatalf("defaults = %+v", cfg)
	}
	t.Setenv("LISTEN_ADDR", ":9999")
	t.Setenv("AUTH_TOKEN", "tok")
	cfg = loadConfig()
	if cfg.listen != ":9999" || cfg.authToken != "tok" {
		t.Fatalf("env override = %+v", cfg)
	}
}

func TestServeShutsDownOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, config{listen: "127.0.0.1:0"}) }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve returned %v after cancel, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not shut down after context cancel")
	}
}

func TestServeBadListenAddr(t *testing.T) {
	err := serve(context.Background(), config{listen: "not-an-addr"})
	if err == nil {
		t.Fatal("expected listen error")
	}
}

// Only a single JSON object carrying all three required fields is accepted;
// anything else gets 400 and is not counted.
func TestIngestRequiresCompleteSingleReport(t *testing.T) {
	srv := testServer(t, config{})
	valid := `{"version":"1.0.0","servers":1,"templates":1}`
	cases := []struct {
		name string
		body string
	}{
		{"missing version", `{"servers":1,"templates":1}`},
		{"missing servers", `{"version":"1.0.0","templates":1}`},
		{"missing templates", `{"version":"1.0.0","servers":1}`},
		{"empty object", `{}`},
		{"null field", `{"version":"1.0.0","servers":null,"templates":1}`},
		{"null body", `null`},
		{"array body", `[` + valid + `]`},
		{"number body", `42`},
		{"empty body", ``},
		{"trailing garbage", valid + `xyz`},
		{"second object", valid + valid},
		{"trailing value", valid + ` 1`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := post(t, srv, tc.body, nil)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
	if m := metrics(t, srv); strings.Contains(m, "gameplane_telemetry_reports_total{") {
		t.Fatalf("rejected reports were counted:\n%s", m)
	}
}

// A complete report with zero counts, or followed only by whitespace, is
// still accepted.
func TestIngestAcceptsZeroCountsAndTrailingWhitespace(t *testing.T) {
	srv := testServer(t, config{})
	for _, body := range []string{
		`{"version":"1.0.0","servers":0,"templates":0}`,
		"{\"version\":\"1.0.0\",\"servers\":2,\"templates\":3}\n",
		"  {\"version\":\"1.0.0\",\"servers\":2,\"templates\":3}  \r\n\t",
	} {
		resp := post(t, srv, body, nil)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("body %q: status = %d, want 204", body, resp.StatusCode)
		}
	}
	if m := metrics(t, srv); !strings.Contains(m, `gameplane_telemetry_reports_total{version="1.0.0"} 3`) {
		t.Fatalf("accepted reports not counted:\n%s", m)
	}
}

// The 16 KiB limit applies to the whole body, including anything after the
// report, and a rejected body is not counted.
func TestIngestBodyLimitCoversWholeBody(t *testing.T) {
	srv := testServer(t, config{})
	valid := `{"version":"1.0.0","servers":1,"templates":1}`
	for name, body := range map[string]string{
		"padding after report":  valid + strings.Repeat(" ", maxBody),
		"content after report":  valid + strings.Repeat("x", maxBody),
		"exactly one byte over": valid + strings.Repeat(" ", maxBody-len(valid)+1),
	} {
		t.Run(name, func(t *testing.T) {
			resp := post(t, srv, body, nil)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusRequestEntityTooLarge {
				t.Fatalf("status = %d, want 413", resp.StatusCode)
			}
		})
	}
	// A body exactly at the limit is still read and judged on its content.
	atLimit := valid + strings.Repeat(" ", maxBody-len(valid))
	resp := post(t, srv, atLimit, nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("body at the limit: status = %d, want 204", resp.StatusCode)
	}
	if m := metrics(t, srv); !strings.Contains(m, `gameplane_telemetry_reports_total{version="1.0.0"} 1`) {
		t.Fatalf("expected only the at-limit report to be counted:\n%s", m)
	}
}
