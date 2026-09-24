package main

import (
	"bufio"
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// -----------------------------------------------------------------------
// Fake playitd IPC server
//
// Speaks the v1.0.10 wire format (see the header of playit_reporter.go):
// newline-delimited JSON, hello frame first, then one response per request.
// -----------------------------------------------------------------------

type fakePlayitd struct {
	path string
	ln   net.Listener

	mu       sync.Mutex
	hello    string // raw hello line; default is a valid v2 hello
	respond  func(req map[string]any) []string
	requests []map[string]any
}

// shortSocketDir returns a short temp dir: Unix socket paths are capped at
// ~108 bytes and t.TempDir() can exceed that.
func shortSocketDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "pit")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func newFakePlayitd(t *testing.T, respond func(req map[string]any) []string) *fakePlayitd {
	t.Helper()
	path := filepath.Join(shortSocketDir(t), "p.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	f := &fakePlayitd{
		path:    path,
		ln:      ln,
		hello:   `{"message_kind":"hello","data":{"protocol":{"ipc_version":2,"capabilities":["lifecycle_state"]}}}`,
		respond: respond,
	}
	t.Cleanup(func() { _ = ln.Close() })
	go f.serve()
	return f
}

func (f *fakePlayitd) serve() {
	for {
		conn, err := f.ln.Accept()
		if err != nil {
			return
		}
		go f.handle(conn)
	}
}

func (f *fakePlayitd) handle(conn net.Conn) {
	defer conn.Close()
	f.mu.Lock()
	hello := f.hello
	f.mu.Unlock()
	if _, err := io.WriteString(conn, hello+"\n"); err != nil {
		return
	}
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		var req map[string]any
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			return
		}
		f.mu.Lock()
		f.requests = append(f.requests, req)
		respond := f.respond
		f.mu.Unlock()
		for _, line := range respond(req) {
			if _, err := io.WriteString(conn, line+"\n"); err != nil {
				return
			}
		}
	}
}

func (f *fakePlayitd) setHello(h string) {
	f.mu.Lock()
	f.hello = h
	f.mu.Unlock()
}

func (f *fakePlayitd) setRespond(r func(req map[string]any) []string) {
	f.mu.Lock()
	f.respond = r
	f.mu.Unlock()
}

func stateResponse(lifecycle string) func(req map[string]any) []string {
	return func(req map[string]any) []string {
		id, _ := req["request_id"].(float64)
		b, _ := json.Marshal(map[string]any{
			"message_kind": "response",
			"data": map[string]any{
				"ipc_version": 2,
				"request_id":  int(id),
				"response":    map[string]any{"type": "state", "data": json.RawMessage(lifecycle)},
			},
		})
		return []string{string(b)}
	}
}

const runningTwoTunnels = `{"state":"running","data":{"version":"1.0.10","tunnels":[
 {"display_address":"alpha.gl.joinmc.link:31337","destination":"10.43.0.10:25565","is_disabled":false,"disabled_reason":null},
 {"display_address":"147.185.221.5:40000","destination":"10.43.0.10:19132","is_disabled":false,"disabled_reason":null}
],"pending_tunnels":[],"notices":[],"account_status":"verified","agent_id":"a","login_link":null,"start_time":0}}`

// -----------------------------------------------------------------------
// queryPlayitState
// -----------------------------------------------------------------------

func TestQueryPlayitStateRunning(t *testing.T) {
	f := newFakePlayitd(t, stateResponse(runningTwoTunnels))

	st, err := queryPlayitState(context.Background(), f.path)
	if err != nil {
		t.Fatalf("queryPlayitState: %v", err)
	}
	if st.Phase != "running" || len(st.Tunnels) != 2 {
		t.Fatalf("state = %+v, want running with 2 tunnels", st)
	}
	if st.Tunnels[0].DisplayAddress != "alpha.gl.joinmc.link:31337" || st.Tunnels[0].Destination != "10.43.0.10:25565" {
		t.Errorf("tunnel[0] = %+v", st.Tunnels[0])
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(f.requests))
	}
	req := f.requests[0]
	if req["ipc_version"] != float64(2) || req["request_id"] != float64(1) {
		t.Errorf("request envelope = %v", req)
	}
	inner, _ := req["request"].(map[string]any)
	if inner["type"] != "get_state" {
		t.Errorf("request type = %v, want get_state", inner["type"])
	}
}

func TestQueryPlayitStateStarting(t *testing.T) {
	f := newFakePlayitd(t, stateResponse(`{"state":"starting"}`))
	st, err := queryPlayitState(context.Background(), f.path)
	if err != nil {
		t.Fatalf("queryPlayitState: %v", err)
	}
	if st.Phase != "starting" || len(st.Tunnels) != 0 {
		t.Errorf("state = %+v, want starting with no tunnels", st)
	}
}

func TestQueryPlayitStateErrorLifecycle(t *testing.T) {
	// Error-bearing lifecycle states carry a ServiceError in "data"; they
	// decode as that phase with no tunnels rather than failing.
	f := newFakePlayitd(t, stateResponse(`{"state":"has_invalid_secret","data":{"code":"invalid_secret","message":"bad","retryable":false,"details":null}}`))
	st, err := queryPlayitState(context.Background(), f.path)
	if err != nil {
		t.Fatalf("queryPlayitState: %v", err)
	}
	if st.Phase != "has_invalid_secret" || len(st.Tunnels) != 0 {
		t.Errorf("state = %+v", st)
	}
}

func TestQueryPlayitStateSkipsEvents(t *testing.T) {
	f := newFakePlayitd(t, func(req map[string]any) []string {
		ev := `{"message_kind":"event","data":{"ipc_version":2,"event":{"type":"stats","data":{"bytes_in":0,"bytes_out":0,"active_tcp":0,"active_udp":0}}}}`
		return append([]string{ev}, stateResponse(`{"state":"stopping"}`)(req)...)
	})
	st, err := queryPlayitState(context.Background(), f.path)
	if err != nil {
		t.Fatalf("queryPlayitState: %v", err)
	}
	if st.Phase != "stopping" {
		t.Errorf("phase = %q, want stopping", st.Phase)
	}
}

func TestQueryPlayitStateErrors(t *testing.T) {
	tests := []struct {
		name    string
		hello   string
		respond func(req map[string]any) []string
		want    string
	}{
		{
			name:  "wrong ipc version",
			hello: `{"message_kind":"hello","data":{"protocol":{"ipc_version":3,"capabilities":[]}}}`,
			want:  "IPC version 3",
		},
		{
			name:  "first frame not hello",
			hello: `{"message_kind":"event","data":{}}`,
			want:  "want hello",
		},
		{
			name:  "garbage hello",
			hello: `not json`,
			want:  "read playitd hello",
		},
		{
			name: "service error response",
			respond: func(map[string]any) []string {
				return []string{`{"message_kind":"response","data":{"ipc_version":2,"request_id":1,"response":{"type":"error","data":{"code":"unsupported_protocol","message":"nope","retryable":false}}}}`}
			},
			want: "playitd error unsupported_protocol: nope",
		},
		{
			name: "unexpected response type",
			respond: func(map[string]any) []string {
				return []string{`{"message_kind":"response","data":{"ipc_version":2,"request_id":1,"response":{"type":"status","data":{}}}}`}
			},
			want: `unexpected "status" response`,
		},
		{
			name: "mismatched request id",
			respond: func(map[string]any) []string {
				return []string{`{"message_kind":"response","data":{"ipc_version":2,"request_id":7,"response":{"type":"state","data":{"state":"starting"}}}}`}
			},
			want: "response id 7",
		},
		{
			name: "duplicate hello",
			respond: func(map[string]any) []string {
				return []string{`{"message_kind":"hello","data":{"protocol":{"ipc_version":2}}}`}
			},
			want: `unexpected "hello" frame`,
		},
		{
			name: "state without tag",
			respond: func(map[string]any) []string {
				return []string{`{"message_kind":"response","data":{"ipc_version":2,"request_id":1,"response":{"type":"state","data":{}}}}`}
			},
			want: "no state",
		},
		{
			name: "blank response frame",
			respond: func(map[string]any) []string {
				return []string{""}
			},
			want: "read get_state response",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			respond := tt.respond
			if respond == nil {
				respond = stateResponse(`{"state":"starting"}`)
			}
			f := newFakePlayitd(t, respond)
			if tt.hello != "" {
				f.setHello(tt.hello)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, err := queryPlayitState(ctx, f.path)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestQueryPlayitStateNoSocket(t *testing.T) {
	_, err := queryPlayitState(context.Background(), filepath.Join(shortSocketDir(t), "missing.sock"))
	if err == nil || !strings.Contains(err.Error(), "dial playitd socket") {
		t.Errorf("err = %v, want dial error", err)
	}
}

func TestQueryPlayitStateTimesOut(t *testing.T) {
	// A server that sends hello then never answers must not hang forever:
	// the caller's context bounds the read.
	f := newFakePlayitd(t, func(map[string]any) []string { return nil })
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := queryPlayitState(ctx, f.path)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if time.Since(start) > 3*time.Second {
		t.Errorf("query took %v, want bounded by ctx", time.Since(start))
	}
}

// -----------------------------------------------------------------------
// Mapping
// -----------------------------------------------------------------------

func TestParseBackingPorts(t *testing.T) {
	got, err := parseBackingPorts("java:25565, bedrock:19132,")
	if err != nil {
		t.Fatalf("parseBackingPorts: %v", err)
	}
	want := []backingPort{{"java", 25565}, {"bedrock", 19132}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	for _, bad := range []string{"java", ":25565", "java:0", "java:70000", "java:abc"} {
		if _, err := parseBackingPorts(bad); err == nil {
			t.Errorf("parseBackingPorts(%q) = nil error, want error", bad)
		}
	}
}

func TestPlayitEndpointsMatchesByDestinationPort(t *testing.T) {
	ports := []backingPort{{"java", 25565}, {"bedrock", 19132}}
	tunnels := []playitTunnel{
		{DisplayAddress: "alpha.gl.joinmc.link:31337", Destination: "10.43.0.10:25565"},
		{DisplayAddress: "147.185.221.5:40000", Destination: "10.43.0.10:19132"},
		{DisplayAddress: "disabled.example:1", Destination: "10.43.0.10:25565", IsDisabled: true},
		{DisplayAddress: "web.example", Destination: "10.43.0.10 (http: 80, https: 443)"},
		{DisplayAddress: "other.example:5000", Destination: "10.43.0.10:9999"},
	}
	got := playitEndpoints(tunnels, ports)
	want := []tunnelEndpoint{
		{Name: "bedrock", Host: "147.185.221.5", Port: 40000, TunnelProvider: "playit"},
		{Name: "java", Host: "alpha.gl.joinmc.link", Port: 31337, TunnelProvider: "playit"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestPlayitEndpointsSinglePortFallbackAndPortlessAddress(t *testing.T) {
	// One advertised port, one enabled tunnel pointed at a different local
	// port, and a display address with no port (SRV-style hostname).
	ports := []backingPort{{"game", 25565}}
	tunnels := []playitTunnel{{DisplayAddress: "beta.joinmc.link", Destination: "127.0.0.1:1234"}}
	got := playitEndpoints(tunnels, ports)
	want := []tunnelEndpoint{{Name: "game", Host: "beta.joinmc.link", Port: 25565, TunnelProvider: "playit"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestPlayitEndpointsDedupAndBadPort(t *testing.T) {
	ports := []backingPort{{"game", 25565}, {"query", 25566}}
	tunnels := []playitTunnel{
		{DisplayAddress: "a.example:1000", Destination: "x:25565"},
		{DisplayAddress: "b.example:2000", Destination: "x:25565"},  // duplicate name, dropped
		{DisplayAddress: "c.example:99999", Destination: "x:25566"}, // bad public port, dropped
	}
	got := playitEndpoints(tunnels, ports)
	want := []tunnelEndpoint{{Name: "game", Host: "a.example", Port: 1000, TunnelProvider: "playit"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got := playitEndpoints(nil, ports); len(got) != 0 {
		t.Errorf("no tunnels: got %+v, want empty", got)
	}
}

func TestPlayitEndpointsCappedAtCRDMax(t *testing.T) {
	var ports []backingPort
	var tunnels []playitTunnel
	for i := 0; i < maxTunnelEndpoints+5; i++ {
		p := int32(20000 + i)
		name := "p" + string(rune('a'+i/26)) + string(rune('a'+i%26))
		ports = append(ports, backingPort{name, p})
		tunnels = append(tunnels, playitTunnel{
			DisplayAddress: "h.example:" + strconv.Itoa(int(p)),
			Destination:    "x:" + strconv.Itoa(int(p)),
		})
	}
	if got := playitEndpoints(tunnels, ports); len(got) != maxTunnelEndpoints {
		t.Errorf("len = %d, want %d", len(got), maxTunnelEndpoints)
	}
}

// -----------------------------------------------------------------------
// kubeStatusPatcher
// -----------------------------------------------------------------------

func TestKubeStatusPatcherPatches(t *testing.T) {
	var gotPath, gotCT, gotAuth, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotCT, gotAuth = r.Header.Get("Content-Type"), r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte("tok-123\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := &kubeStatusPatcher{
		baseURL: srv.URL, namespace: "games", name: "mc",
		tokenPath: filepath.Join(dir, "token"), client: srv.Client(),
	}
	eps := []tunnelEndpoint{{Name: "java", Host: "a.example", Port: 1000, TunnelProvider: "playit"}}
	if err := p.PatchTunnelEndpoints(context.Background(), eps); err != nil {
		t.Fatalf("PatchTunnelEndpoints: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %s", gotMethod)
	}
	if gotPath != "/apis/gameplane.local/v1alpha1/namespaces/games/gameservers/mc/status" {
		t.Errorf("path = %s", gotPath)
	}
	if gotCT != "application/merge-patch+json" {
		t.Errorf("content-type = %s", gotCT)
	}
	if gotAuth != "Bearer tok-123" {
		t.Errorf("authorization = %q", gotAuth)
	}
	status, _ := gotBody["status"].(map[string]any)
	list, _ := status["tunnelEndpoints"].([]any)
	if len(list) != 1 {
		t.Fatalf("body = %v", gotBody)
	}
	ep, _ := list[0].(map[string]any)
	if ep["name"] != "java" || ep["host"] != "a.example" || ep["port"] != float64(1000) || ep["tunnelProvider"] != "playit" {
		t.Errorf("endpoint = %v", ep)
	}

	// Empty set clears the field with an explicit null.
	if err := p.PatchTunnelEndpoints(context.Background(), nil); err != nil {
		t.Fatalf("PatchTunnelEndpoints(nil): %v", err)
	}
	status, _ = gotBody["status"].(map[string]any)
	if v, ok := status["tunnelEndpoints"]; !ok || v != nil {
		t.Errorf("empty patch body = %v, want tunnelEndpoints: null", gotBody)
	}
}

func TestKubeStatusPatcherErrors(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenPath, []byte("t"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := &kubeStatusPatcher{baseURL: srv.URL, namespace: "g", name: "n", tokenPath: tokenPath, client: srv.Client()}
	err := p.PatchTunnelEndpoints(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Errorf("err = %v, want HTTP 403", err)
	}

	p.tokenPath = filepath.Join(dir, "missing")
	err = p.PatchTunnelEndpoints(context.Background(), nil)
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err = %v, want wrapped ErrNotExist", err)
	}
}

func TestNewInClusterStatusPatcher(t *testing.T) {
	dir := t.TempDir()
	if _, err := newInClusterStatusPatcher(dir, "g", "n"); err == nil {
		t.Error("missing CA: want error")
	}
	if err := os.WriteFile(filepath.Join(dir, "ca.crt"), []byte("not a pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := newInClusterStatusPatcher(dir, "g", "n"); err == nil {
		t.Error("invalid CA: want error")
	}

	srv := httptest.NewTLSServer(http.NotFoundHandler())
	defer srv.Close()
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(filepath.Join(dir, "ca.crt"), certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := newInClusterStatusPatcher(dir, "g", "n")
	if err != nil {
		t.Fatalf("valid CA: %v", err)
	}
	if p.baseURL != kubeAPIServerURL || p.tokenPath != filepath.Join(dir, "token") {
		t.Errorf("patcher = %+v", p)
	}
}

// -----------------------------------------------------------------------
// runPlayitReporter
// -----------------------------------------------------------------------

type recordingPatcher struct {
	mu    sync.Mutex
	calls [][]tunnelEndpoint
	fail  int // number of initial calls to fail
	ch    chan struct{}
}

func (r *recordingPatcher) PatchTunnelEndpoints(_ context.Context, eps []tunnelEndpoint) error {
	r.mu.Lock()
	defer func() {
		r.mu.Unlock()
		select {
		case r.ch <- struct{}{}:
		default:
		}
	}()
	if r.fail > 0 {
		r.fail--
		return errors.New("apiserver unavailable")
	}
	r.calls = append(r.calls, eps)
	return nil
}

func (r *recordingPatcher) snapshot() [][]tunnelEndpoint {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][]tunnelEndpoint(nil), r.calls...)
}

var fastTiming = reporterTiming{minBackoff: 5 * time.Millisecond, maxBackoff: 20 * time.Millisecond, steady: 10 * time.Millisecond}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within 3s")
}

func TestRunPlayitReporterReportsOnceAndOnChange(t *testing.T) {
	f := newFakePlayitd(t, stateResponse(`{"state":"starting"}`))
	ports := []backingPort{{"java", 25565}, {"bedrock", 19132}}
	rp := &recordingPatcher{fail: 1, ch: make(chan struct{}, 1)}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		runPlayitReporter(ctx, f.path, ports, rp, fastTiming)
	}()

	// While starting, nothing is patched.
	time.Sleep(30 * time.Millisecond)
	if n := len(rp.snapshot()); n != 0 {
		t.Fatalf("patched %d times while starting, want 0", n)
	}

	// Running: the first patch fails, the retry succeeds.
	f.setRespond(stateResponse(runningTwoTunnels))
	waitFor(t, func() bool { return len(rp.snapshot()) == 1 })

	// Unchanged state: no further patches across several steady polls.
	time.Sleep(50 * time.Millisecond)
	if n := len(rp.snapshot()); n != 1 {
		t.Fatalf("patched %d times with unchanged state, want 1", n)
	}

	// Address change: patched again.
	f.setRespond(stateResponse(`{"state":"running","data":{"tunnels":[{"display_address":"new.example:1","destination":"x:25565","is_disabled":false}]}}`))
	waitFor(t, func() bool { return len(rp.snapshot()) == 2 })
	calls := rp.snapshot()
	want := []tunnelEndpoint{{Name: "java", Host: "new.example", Port: 1, TunnelProvider: "playit"}}
	if !reflect.DeepEqual(calls[1], want) {
		t.Errorf("second patch = %+v, want %+v", calls[1], want)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("reporter did not stop after ctx cancel")
	}
}

func TestRunPlayitReporterReportsEmptySetOnFirstRun(t *testing.T) {
	// A running agent with no matching tunnels still patches once (with
	// null) so stale endpoints from a previous pod are cleared.
	f := newFakePlayitd(t, stateResponse(`{"state":"running","data":{"tunnels":[]}}`))
	rp := &recordingPatcher{ch: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runPlayitReporter(ctx, f.path, []backingPort{{"java", 25565}}, rp, fastTiming)
	waitFor(t, func() bool { return len(rp.snapshot()) == 1 })
	if got := rp.snapshot()[0]; len(got) != 0 {
		t.Errorf("first patch = %+v, want empty", got)
	}
}

func TestRunPlayitReporterStopsWhenSocketMissing(t *testing.T) {
	rp := &recordingPatcher{ch: make(chan struct{}, 1)}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		runPlayitReporter(ctx, filepath.Join(shortSocketDir(t), "none.sock"), nil, rp, fastTiming)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("reporter did not stop at ctx deadline")
	}
	if n := len(rp.snapshot()); n != 0 {
		t.Errorf("patched %d times without a socket, want 0", n)
	}
}

func TestStartPlayitReporterDisabledOutsideCluster(t *testing.T) {
	// Outside a cluster the ServiceAccount mount is absent: the reporter is
	// skipped and the returned wait function returns immediately. The
	// context is pre-cancelled so that, should the test host happen to have
	// a ServiceAccount mount, the started reporter exits at once too.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	wait := startPlayitReporter(ctx, Config{
		GameServerName: "g", GameServerNamespace: "n", BackingServicePorts: "java:25565",
	})
	wait()

	wait = startPlayitReporter(ctx, Config{BackingServicePorts: "bad"})
	wait()
}
