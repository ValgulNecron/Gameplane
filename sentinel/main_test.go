package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// -----------------------------------------------------------------------
// PORTS_CONFIG parsing
// -----------------------------------------------------------------------

func TestParsePortsConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		want    int
		wantErr bool
	}{
		{name: "empty", config: "", want: 0},
		{name: "single port", config: "25565:TCP:minecraft", want: 1},
		{name: "multiple ports", config: "25565:TCP:minecraft,19133:UDP:generic", want: 2},
		{name: "with spaces", config: "25565: TCP : minecraft , 19133: UDP : generic", want: 2},
		{name: "leading comma", config: ",25565:TCP:minecraft", want: 1},
		{name: "trailing comma", config: "25565:TCP:minecraft,", want: 1},
		{name: "leading and trailing", config: ",25565:TCP:minecraft,", want: 1},
		{name: "doubled comma", config: "25565:TCP:minecraft,,19133:UDP:generic", want: 2},
		{name: "none protocol", config: "25565:TCP:none", want: 1},
		{name: "invalid port number", config: "abc:TCP:minecraft", wantErr: true},
		{name: "invalid protocol", config: "25565:INVALID:minecraft", wantErr: true},
		{name: "invalid wake protocol", config: "25565:TCP:invalid", wantErr: true},
		{name: "wrong format", config: "25565:TCP", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePortsConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePortsConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(got) != tt.want {
				t.Errorf("parsePortsConfig() got %d ports, want %d", len(got), tt.want)
			}
		})
	}
}

func TestParsePortsConfigValues(t *testing.T) {
	ports, err := parsePortsConfig("25565:TCP:minecraft,19133:UDP:generic")
	if err != nil {
		t.Fatalf("parsePortsConfig() error = %v", err)
	}
	if len(ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(ports))
	}

	want := []PortConfig{
		{ContainerPort: 25565, Protocol: "TCP", WakeProtocol: "minecraft"},
		{ContainerPort: 19133, Protocol: "UDP", WakeProtocol: "generic"},
	}
	for i, w := range want {
		if ports[i] != w {
			t.Errorf("port %d = %+v, want %+v", i, ports[i], w)
		}
	}
}

func TestWantsListener(t *testing.T) {
	if wantsListener(PortConfig{WakeProtocol: "none"}) {
		t.Error("wakeProtocol none should not want a listener")
	}
	for _, wp := range []string{"minecraft", "terraria", "generic"} {
		if !wantsListener(PortConfig{WakeProtocol: wp}) {
			t.Errorf("wakeProtocol %s should want a listener", wp)
		}
	}
}

// -----------------------------------------------------------------------
// loadConfig
// -----------------------------------------------------------------------

func stubEnv(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := loadConfig(stubEnv(map[string]string{
		"GAMESERVER_NAME":      "my-server",
		"GAMESERVER_NAMESPACE": "games",
		"PORTS_CONFIG":         "25565:TCP:minecraft",
	}))
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.GameServerName != "my-server" || cfg.GameServerNamespace != "games" {
		t.Errorf("unexpected identity: %+v", cfg)
	}
	if cfg.WakeDeadline != 25*time.Second {
		t.Errorf("WakeDeadline = %v, want 25s", cfg.WakeDeadline)
	}
	if cfg.UDPThreshold != 3 {
		t.Errorf("UDPThreshold = %d, want 3", cfg.UDPThreshold)
	}
	if cfg.UDPWindow != 10*time.Second {
		t.Errorf("UDPWindow = %v, want 10s", cfg.UDPWindow)
	}
	if cfg.UDPMaxSources != 4096 {
		t.Errorf("UDPMaxSources = %d, want 4096", cfg.UDPMaxSources)
	}
	if cfg.MaxConnections != 256 {
		t.Errorf("MaxConnections = %d, want 256", cfg.MaxConnections)
	}
	if cfg.WakePatchInterval != 2*time.Second {
		t.Errorf("WakePatchInterval = %v, want 2s", cfg.WakePatchInterval)
	}
	if len(cfg.Ports) != 1 {
		t.Fatalf("expected 1 port, got %d", len(cfg.Ports))
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	cfg, err := loadConfig(stubEnv(map[string]string{
		"GAMESERVER_NAME":      "my-server",
		"GAMESERVER_NAMESPACE": "games",
		"PORTS_CONFIG":         "25565:TCP:minecraft",
		"WAKE_DEADLINE":        "5s",
		"UDP_PACKET_THRESHOLD": "7",
		"UDP_PACKET_WINDOW":    "1s",
		"UDP_MAX_SOURCES":      "10",
		"MAX_CONNECTIONS":      "4",
		"WAKE_PATCH_INTERVAL":  "500ms",
	}))
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.WakeDeadline != 5*time.Second {
		t.Errorf("WakeDeadline = %v, want 5s", cfg.WakeDeadline)
	}
	if cfg.UDPThreshold != 7 {
		t.Errorf("UDPThreshold = %d, want 7", cfg.UDPThreshold)
	}
	if cfg.UDPWindow != time.Second {
		t.Errorf("UDPWindow = %v, want 1s", cfg.UDPWindow)
	}
	if cfg.UDPMaxSources != 10 {
		t.Errorf("UDPMaxSources = %d, want 10", cfg.UDPMaxSources)
	}
	if cfg.MaxConnections != 4 {
		t.Errorf("MaxConnections = %d, want 4", cfg.MaxConnections)
	}
	if cfg.WakePatchInterval != 500*time.Millisecond {
		t.Errorf("WakePatchInterval = %v, want 500ms", cfg.WakePatchInterval)
	}
}

func TestLoadConfigErrors(t *testing.T) {
	base := map[string]string{"GAMESERVER_NAME": "my-server", "GAMESERVER_NAMESPACE": "games"}

	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "missing name", env: map[string]string{"GAMESERVER_NAMESPACE": "games"}},
		{name: "missing namespace", env: map[string]string{"GAMESERVER_NAME": "my-server"}},
		{name: "bad ports config", env: withKV(base, "PORTS_CONFIG", "not-a-port")},
		{name: "bad wake deadline", env: withKV(base, "WAKE_DEADLINE", "not-a-duration")},
		{name: "bad udp threshold", env: withKV(base, "UDP_PACKET_THRESHOLD", "0")},
		{name: "bad udp window", env: withKV(base, "UDP_PACKET_WINDOW", "nope")},
		{name: "bad udp max sources", env: withKV(base, "UDP_MAX_SOURCES", "-1")},
		{name: "bad max connections", env: withKV(base, "MAX_CONNECTIONS", "nope")},
		{name: "bad wake patch interval", env: withKV(base, "WAKE_PATCH_INTERVAL", "nope")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := loadConfig(stubEnv(tt.env)); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

// withKV returns a copy of base with key=value set, leaving base untouched.
func withKV(base map[string]string, key, value string) map[string]string {
	out := make(map[string]string, len(base)+1)
	for k, v := range base {
		out[k] = v
	}
	out[key] = value
	return out
}

func TestGameDirectAddr(t *testing.T) {
	cfg := Config{GameServerName: "myserver", GameServerNamespace: "games"}
	got := gameDirectAddr(cfg, PortConfig{ContainerPort: 25565})
	want := "myserver-game-direct.games.svc.cluster.local:25565"
	if got != want {
		t.Errorf("gameDirectAddr() = %q, want %q", got, want)
	}
}

// -----------------------------------------------------------------------
// UDP heuristic: counting, window expiry, cooldown, eviction, sweep
// -----------------------------------------------------------------------

func TestUDPHeuristicThresholdAndCooldown(t *testing.T) {
	h := newUDPHeuristic(3, 10*time.Second, 100)
	t0 := time.Now()

	if h.shouldWake("a", t0) {
		t.Error("first packet should not wake")
	}
	if h.shouldWake("a", t0.Add(time.Millisecond)) {
		t.Error("second packet should not wake")
	}
	if !h.shouldWake("a", t0.Add(2*time.Millisecond)) {
		t.Error("third packet should wake")
	}
	if h.shouldWake("a", t0.Add(3*time.Millisecond)) {
		t.Error("fourth packet should not wake (cooldown)")
	}
}

func TestUDPHeuristicWindowExpiry(t *testing.T) {
	h := newUDPHeuristic(3, 100*time.Millisecond, 100)
	t0 := time.Now()

	h.shouldWake("a", t0)
	h.shouldWake("a", t0.Add(10*time.Millisecond))

	// Well past the window: the first two packets should have aged out.
	later := t0.Add(200 * time.Millisecond)
	if h.shouldWake("a", later) {
		t.Error("should not wake: old packets should have aged out of the window")
	}
	if h.shouldWake("a", later.Add(time.Millisecond)) {
		t.Error("should not wake on second packet after reset")
	}
	if !h.shouldWake("a", later.Add(2*time.Millisecond)) {
		t.Error("third packet after reset should wake")
	}
}

func TestUDPHeuristicIndependentSources(t *testing.T) {
	h := newUDPHeuristic(2, 10*time.Second, 100)
	t0 := time.Now()

	h.shouldWake("a", t0)
	if !h.shouldWake("a", t0.Add(time.Millisecond)) {
		t.Error("addr a's second packet should wake")
	}
	if h.shouldWake("b", t0.Add(2*time.Millisecond)) {
		t.Error("addr b's first packet should not wake")
	}
}

func TestUDPHeuristicEviction(t *testing.T) {
	h := newUDPHeuristic(10, time.Hour, 2) // high threshold: nothing should wake here
	t0 := time.Now()

	h.shouldWake("a", t0)
	h.shouldWake("b", t0.Add(time.Second))
	if len(h.sources) != 2 {
		t.Fatalf("expected 2 tracked sources, got %d", len(h.sources))
	}

	// A third distinct source should evict "a" (the least recently active).
	h.shouldWake("c", t0.Add(2*time.Second))
	if len(h.sources) != 2 {
		t.Fatalf("expected map capped at 2 sources, got %d", len(h.sources))
	}
	if _, ok := h.sources["a"]; ok {
		t.Error("expected oldest source 'a' to be evicted")
	}
	if _, ok := h.sources["b"]; !ok {
		t.Error("expected 'b' to survive eviction")
	}
	if _, ok := h.sources["c"]; !ok {
		t.Error("expected new source 'c' to be tracked")
	}
}

func TestUDPHeuristicSweep(t *testing.T) {
	h := newUDPHeuristic(10, time.Second, 100)
	t0 := time.Now()

	h.shouldWake("stale", t0)
	h.shouldWake("fresh", t0.Add(3*time.Second))

	// Sweep well past 2x the window relative to t0, but not relative to the
	// fresh source.
	h.sweepOnce(t0.Add(3*time.Second + 100*time.Millisecond))

	if _, ok := h.sources["stale"]; ok {
		t.Error("expected stale source to be swept")
	}
	if _, ok := h.sources["fresh"]; !ok {
		t.Error("expected fresh source to survive the sweep")
	}
}

func TestUDPHeuristicSweepLoopExitsOnContextCancel(t *testing.T) {
	h := newUDPHeuristic(3, 10*time.Millisecond, 100)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		h.sweepLoop(ctx, 5*time.Millisecond)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sweepLoop did not exit after context cancellation")
	}
}

// -----------------------------------------------------------------------
// waitForUpstream: the deadline/bounce decision
// -----------------------------------------------------------------------

func TestWaitForUpstreamSucceedsImmediately(t *testing.T) {
	lc := &net.ListenConfig{}
	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	go func() {
		c, err := l.Accept()
		if err == nil {
			c.Close()
		}
	}()

	conn, err := waitForUpstream(context.Background(), l.Addr().String(), time.Second, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("waitForUpstream() error = %v", err)
	}
	conn.Close()
}

func TestWaitForUpstreamPollsUntilAvailable(t *testing.T) {
	// Reserve a free port, then release it immediately: Listen+Close
	// without ever accepting a connection releases the port right away (no
	// TIME_WAIT, which only applies to established connections).
	lc := &net.ListenConfig{}
	probe, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := probe.Addr().String()
	probe.Close()

	// Start waiting immediately (nothing listens yet, so the first several
	// dial attempts must fail and retry), then bring the listener up after
	// a delay from a separate goroutine — proving waitForUpstream is
	// actually polling rather than succeeding on a lucky first attempt.
	start := time.Now()
	go func() {
		time.Sleep(80 * time.Millisecond)
		lc := &net.ListenConfig{}
		l, err := lc.Listen(context.Background(), "tcp", addr)
		if err != nil {
			// Environment couldn't reuse the port fast enough; nothing
			// more this goroutine can do. waitForUpstream will then hit
			// its own deadline and the test fails with a clear timeout
			// error below instead of hanging.
			return
		}
		defer l.Close()
		c, err := l.Accept()
		if err == nil {
			c.Close()
		}
	}()

	conn, err := waitForUpstream(context.Background(), addr, 3*time.Second, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("waitForUpstream() error = %v", err)
	}
	defer conn.Close()
	if time.Since(start) < 60*time.Millisecond {
		t.Errorf("expected waitForUpstream to have polled for a while, took %v", time.Since(start))
	}
}

func TestWaitForUpstreamDeadlineExpires(t *testing.T) {
	// Reserve and release a port so nothing is listening on it.
	lc := &net.ListenConfig{}
	probe, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := probe.Addr().String()
	probe.Close()

	start := time.Now()
	_, err = waitForUpstream(context.Background(), addr, 150*time.Millisecond, 20*time.Millisecond)
	if err == nil {
		t.Fatal("expected error from an upstream that never comes up")
	}
	elapsed := time.Since(start)
	if elapsed < 100*time.Millisecond || elapsed > 2*time.Second {
		t.Errorf("waitForUpstream took %v, expected roughly the 150ms deadline", elapsed)
	}
}

func TestWaitForUpstreamRespectsParentContext(t *testing.T) {
	lc := &net.ListenConfig{}
	probe, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := probe.Addr().String()
	probe.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err = waitForUpstream(ctx, addr, 10*time.Second, 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected error when parent context is cancelled")
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("waitForUpstream should have returned promptly on ctx cancellation, took %v", time.Since(start))
	}
}

// -----------------------------------------------------------------------
// proxyBidirectional: waits for both directions, half-closes cleanly
// -----------------------------------------------------------------------

// tcpPipe returns a connected pair of loopback TCP connections for tests
// that need real net.Conn semantics (notably CloseWrite/half-close), which
// net.Pipe's in-memory implementation does not support.
func tcpPipe(t *testing.T) (server, client net.Conn) {
	t.Helper()
	lc := &net.ListenConfig{}
	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()

	serverCh := make(chan net.Conn, 1)
	go func() {
		c, _ := l.Accept()
		serverCh <- c
	}()

	dialer := &net.Dialer{}
	client, err = dialer.DialContext(context.Background(), "tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	server = <-serverCh
	if server == nil {
		t.Fatal("accept failed")
	}
	return server, client
}

func TestProxyBidirectionalWaitsForBothDirections(t *testing.T) {
	upstreamServer, upstreamClient := tcpPipe(t)     // sentinel<->game
	downstreamServer, downstreamClient := tcpPipe(t) // sentinel<->player

	done := make(chan struct{})
	go func() {
		proxyBidirectional(context.Background(), upstreamServer, downstreamServer, bufio.NewReader(downstreamServer))
		close(done)
	}()

	// Player -> game.
	if _, err := downstreamClient.Write([]byte("hello-upstream")); err != nil {
		t.Fatalf("write to downstream client: %v", err)
	}
	// Half-close so the copy loop sees EOF without killing the other
	// direction.
	if tc, ok := downstreamClient.(*net.TCPConn); ok {
		_ = tc.CloseWrite()
	}

	gotUp, err := io.ReadAll(upstreamClient)
	if err != nil {
		t.Fatalf("read from upstream client: %v", err)
	}
	if string(gotUp) != "hello-upstream" {
		t.Errorf("upstream got %q, want %q", gotUp, "hello-upstream")
	}

	// Game -> player, after the client->game direction already finished.
	if _, err := upstreamClient.Write([]byte("hello-downstream")); err != nil {
		t.Fatalf("write to upstream client: %v", err)
	}
	if tc, ok := upstreamClient.(*net.TCPConn); ok {
		_ = tc.CloseWrite()
	}

	gotDown, err := io.ReadAll(downstreamClient)
	if err != nil {
		t.Fatalf("read from downstream client: %v", err)
	}
	if string(gotDown) != "hello-downstream" {
		t.Errorf("downstream got %q, want %q", gotDown, "hello-downstream")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("proxyBidirectional did not return after both directions finished")
	}

	upstreamClient.Close()
	downstreamClient.Close()
}

func TestProxyBidirectionalStopsOnContextCancel(t *testing.T) {
	upstreamServer, upstreamClient := tcpPipe(t)
	downstreamServer, downstreamClient := tcpPipe(t)
	defer upstreamClient.Close()
	defer downstreamClient.Close()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		proxyBidirectional(ctx, upstreamServer, downstreamServer, bufio.NewReader(downstreamServer))
		close(done)
	}()

	// Neither side sends or closes anything: without cancellation this
	// would hang forever.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("proxyBidirectional did not return after context cancellation")
	}
}

// -----------------------------------------------------------------------
// waker: RequestWake against a fake dynamic client
// -----------------------------------------------------------------------

func newFakeGameServerWaker(t *testing.T, name, namespace string, minInterval time.Duration, extraAnnotations map[string]any) (*waker, *dynamicfake.FakeDynamicClient) {
	t.Helper()

	annotations := map[string]any{}
	for k, v := range extraAnnotations {
		annotations[k] = v
	}

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameServer",
		"metadata": map[string]any{
			"name":        name,
			"namespace":   namespace,
			"annotations": annotations,
		},
	}}

	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		gameServerGVR: "GameServerList",
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, obj)

	w := newWaker(client.Resource(gameServerGVR).Namespace(namespace), name, minInterval)
	return w, client
}

func TestWakerRequestWakeSetsAnnotation(t *testing.T) {
	w, client := newFakeGameServerWaker(t, "my-server", "games", time.Millisecond, nil)

	if err := w.RequestWake(context.Background()); err != nil {
		t.Fatalf("RequestWake() error = %v", err)
	}

	got, err := client.Resource(gameServerGVR).Namespace("games").Get(context.Background(), "my-server", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get gameserver: %v", err)
	}
	annotations := got.GetAnnotations()
	if annotations[idleWakeRequestedAnnotation] == "" {
		t.Error("expected idle-wake-requested annotation to be set")
	}
}

func TestWakerRequestWakePreservesOtherAnnotations(t *testing.T) {
	w, client := newFakeGameServerWaker(t, "my-server", "games", time.Millisecond, map[string]any{
		"foo": "bar",
	})

	if err := w.RequestWake(context.Background()); err != nil {
		t.Fatalf("RequestWake() error = %v", err)
	}

	got, err := client.Resource(gameServerGVR).Namespace("games").Get(context.Background(), "my-server", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get gameserver: %v", err)
	}
	annotations := got.GetAnnotations()
	if annotations["foo"] != "bar" {
		t.Errorf("expected unrelated annotation 'foo' to survive the merge patch, got %q", annotations["foo"])
	}
	if annotations[idleWakeRequestedAnnotation] == "" {
		t.Error("expected idle-wake-requested annotation to be set")
	}
}

func TestWakerRequestWakeCoalescesBursts(t *testing.T) {
	w, client := newFakeGameServerWaker(t, "my-server", "games", 100*time.Millisecond, nil)

	if err := w.RequestWake(context.Background()); err != nil {
		t.Fatalf("first RequestWake() error = %v", err)
	}
	first, err := client.Resource(gameServerGVR).Namespace("games").Get(context.Background(), "my-server", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get gameserver: %v", err)
	}
	firstToken := first.GetAnnotations()[idleWakeRequestedAnnotation]

	// Immediately request again: should be coalesced (no-op), token unchanged.
	if err := w.RequestWake(context.Background()); err != nil {
		t.Fatalf("second RequestWake() error = %v", err)
	}
	second, err := client.Resource(gameServerGVR).Namespace("games").Get(context.Background(), "my-server", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get gameserver: %v", err)
	}
	if second.GetAnnotations()[idleWakeRequestedAnnotation] != firstToken {
		t.Error("expected the second immediate RequestWake to be coalesced (token unchanged)")
	}

	// After minInterval passes, a new request should land.
	time.Sleep(150 * time.Millisecond)
	if err := w.RequestWake(context.Background()); err != nil {
		t.Fatalf("third RequestWake() error = %v", err)
	}
	third, err := client.Resource(gameServerGVR).Namespace("games").Get(context.Background(), "my-server", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get gameserver: %v", err)
	}
	if third.GetAnnotations()[idleWakeRequestedAnnotation] == firstToken {
		t.Error("expected the third RequestWake (after minInterval) to patch a fresh token")
	}
}

func TestWakerRequestWakeErrorOnMissingObject(t *testing.T) {
	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		gameServerGVR: "GameServerList",
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)
	w := newWaker(client.Resource(gameServerGVR).Namespace("games"), "does-not-exist", time.Millisecond)

	if err := w.RequestWake(context.Background()); err == nil {
		t.Fatal("expected an error patching a GameServer that does not exist")
	}
}

func TestNewInClusterDynamicClientOutsideCluster(t *testing.T) {
	// This test process is not running in a pod, so in-cluster config must
	// fail cleanly (not panic) and return a wrapped error.
	if _, err := newInClusterDynamicClient(); err == nil {
		t.Skip("unexpectedly running with in-cluster config available")
	}
}

// -----------------------------------------------------------------------
// classify -> action mapping (Minecraft / Terraria / generic)
// -----------------------------------------------------------------------

// countingWaker is a wakeRequester test double that records how many times
// RequestWake was called.
type countingWaker struct {
	mu    sync.Mutex
	calls int
	err   error
}

func newCountingWaker(err error) *countingWaker {
	return &countingWaker{err: err}
}

func (c *countingWaker) RequestWake(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return c.err
}

func (c *countingWaker) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func TestHandleMinecraftStatusDoesNotWake(t *testing.T) {
	server, client := tcpPipe(t)
	defer client.Close()

	cw := newCountingWaker(nil)

	done := make(chan struct{})
	go func() {
		handleMinecraft(context.Background(), server, cw, "127.0.0.1:1", time.Second)
		server.Close() // mirrors handleTCPConnection's defer conn.Close() in production
		close(done)
	}()

	if _, err := client.Write(encodeMinecraftHandshake(-1, "localhost", 25565, 1)); err != nil {
		t.Fatalf("write handshake: %v", err)
	}

	reply := make([]byte, 4096)
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := client.Read(reply)
	if err != nil {
		t.Fatalf("read status reply: %v", err)
	}
	if !bytes.Contains(reply[:n], []byte("Asleep")) {
		t.Errorf("expected status reply to mention Asleep, got %q", reply[:n])
	}

	<-done
	if cw.count() != 0 {
		t.Errorf("expected 0 wake requests for a Status ping, got %d", cw.count())
	}
}

func TestHandleMinecraftJoinWakesAndReplaysHandshake(t *testing.T) {
	lc := &net.ListenConfig{}
	upstream, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer upstream.Close()

	upstreamConnCh := make(chan net.Conn, 1)
	go func() {
		c, err := upstream.Accept()
		if err == nil {
			upstreamConnCh <- c
		}
	}()

	server, client := tcpPipe(t)
	defer client.Close()

	cw := newCountingWaker(nil)

	done := make(chan struct{})
	go func() {
		handleMinecraft(context.Background(), server, cw, upstream.Addr().String(), 2*time.Second)
		server.Close() // mirrors handleTCPConnection's defer conn.Close() in production
		close(done)
	}()

	handshake := encodeMinecraftHandshake(761, "localhost", 25565, 2)
	if _, err := client.Write(handshake); err != nil {
		t.Fatalf("write handshake: %v", err)
	}
	// The simulated player sends nothing further: half-close so the
	// player->game copy direction inside proxyBidirectional can reach EOF
	// and finish, instead of blocking forever waiting for more input.
	if tc, ok := client.(*net.TCPConn); ok {
		_ = tc.CloseWrite()
	}

	var upConn net.Conn
	select {
	case upConn = <-upstreamConnCh:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream never accepted a connection")
	}
	defer upConn.Close()

	replayed := make([]byte, len(handshake))
	_ = upConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(upConn, replayed); err != nil {
		t.Fatalf("read replayed handshake: %v", err)
	}
	if !bytes.Equal(replayed, handshake) {
		t.Errorf("replayed bytes = %x, want %x", replayed, handshake)
	}

	if _, err := upConn.Write([]byte("welcome")); err != nil {
		t.Fatalf("write from upstream: %v", err)
	}
	upConn.Close()

	got, err := io.ReadAll(client)
	if err != nil {
		t.Fatalf("read from client: %v", err)
	}
	if !bytes.Equal(got, []byte("welcome")) {
		t.Errorf("client got %q, want %q", got, "welcome")
	}

	<-done
	if cw.count() != 1 {
		t.Errorf("expected exactly 1 wake request for a Join, got %d", cw.count())
	}
}

func TestHandleMinecraftJoinBouncesOnDeadline(t *testing.T) {
	// Reserve and release a port so nothing is listening on it.
	lc := &net.ListenConfig{}
	probe, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := probe.Addr().String()
	probe.Close()

	server, client := tcpPipe(t)
	defer client.Close()

	cw := newCountingWaker(nil)

	done := make(chan struct{})
	go func() {
		handleMinecraft(context.Background(), server, cw, addr, 150*time.Millisecond)
		server.Close() // mirrors handleTCPConnection's defer conn.Close() in production
		close(done)
	}()

	if _, err := client.Write(encodeMinecraftHandshake(761, "localhost", 25565, 2)); err != nil {
		t.Fatalf("write handshake: %v", err)
	}

	_ = client.SetReadDeadline(time.Now().Add(3 * time.Second))
	got, err := io.ReadAll(client)
	if err != nil {
		t.Fatalf("read bounce message: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected a non-empty bounce message on deadline expiry")
	}

	<-done
	if cw.count() != 1 {
		t.Errorf("expected exactly 1 wake request even though it timed out, got %d", cw.count())
	}
}

func TestHandleTerrariaJoinWakes(t *testing.T) {
	lc := &net.ListenConfig{}
	upstream, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer upstream.Close()

	upstreamConnCh := make(chan net.Conn, 1)
	go func() {
		c, err := upstream.Accept()
		if err == nil {
			upstreamConnCh <- c
		}
	}()

	server, client := tcpPipe(t)
	defer client.Close()

	cw := newCountingWaker(nil)

	done := make(chan struct{})
	go func() {
		handleTerraria(context.Background(), server, cw, upstream.Addr().String(), 2*time.Second)
		server.Close() // mirrors handleTCPConnection's defer conn.Close() in production
		close(done)
	}()

	req := encodeTerrariaConnectRequest("Terraria279")
	if _, err := client.Write(req); err != nil {
		t.Fatalf("write connect request: %v", err)
	}
	// The simulated player sends nothing further: half-close so the
	// player->game copy direction inside proxyBidirectional can reach EOF
	// once the upstream side below also closes.
	if tc, ok := client.(*net.TCPConn); ok {
		_ = tc.CloseWrite()
	}

	select {
	case c := <-upstreamConnCh:
		c.Close()
	case <-time.After(2 * time.Second):
		t.Fatal("upstream never accepted a connection")
	}

	<-done
	if cw.count() != 1 {
		t.Errorf("expected exactly 1 wake request for a Terraria join, got %d", cw.count())
	}
}

func TestHandleTerrariaJoinBouncesOnDeadline(t *testing.T) {
	// Reserve and release a port so nothing is listening on it.
	lc := &net.ListenConfig{}
	probe, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := probe.Addr().String()
	probe.Close()

	server, client := tcpPipe(t)
	defer client.Close()

	cw := newCountingWaker(nil)

	done := make(chan struct{})
	go func() {
		handleTerraria(context.Background(), server, cw, addr, 150*time.Millisecond)
		server.Close() // mirrors handleTCPConnection's defer conn.Close() in production
		close(done)
	}()

	if _, err := client.Write(encodeTerrariaConnectRequest("Terraria279")); err != nil {
		t.Fatalf("write connect request: %v", err)
	}

	_ = client.SetReadDeadline(time.Now().Add(3 * time.Second))
	got, err := io.ReadAll(client)
	if err != nil {
		t.Fatalf("read bounce message: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected a non-empty bounce message on deadline expiry")
	}

	<-done
	if cw.count() != 1 {
		t.Errorf("expected exactly 1 wake request even though it timed out, got %d", cw.count())
	}
}

func TestHandleTerrariaNonJoinDoesNotWake(t *testing.T) {
	server, client := tcpPipe(t)
	defer client.Close()

	cw := newCountingWaker(nil)

	done := make(chan struct{})
	go func() {
		handleTerraria(context.Background(), server, cw, "127.0.0.1:1", time.Second)
		server.Close() // mirrors handleTCPConnection's defer conn.Close() in production
		close(done)
	}()

	// A message type other than ConnectRequest (1) classifies as Unknown.
	if _, err := client.Write(encodeTerrariaMessage(99, []byte("nope"))); err != nil {
		t.Fatalf("write: %v", err)
	}

	<-done
	if cw.count() != 0 {
		t.Errorf("expected 0 wake requests for a non-Join terraria message, got %d", cw.count())
	}
}

func TestHandleGenericWakesOnConnect(t *testing.T) {
	lc := &net.ListenConfig{}
	upstream, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer upstream.Close()

	upstreamConnCh := make(chan net.Conn, 1)
	go func() {
		c, err := upstream.Accept()
		if err == nil {
			upstreamConnCh <- c
		}
	}()

	server, client := tcpPipe(t)
	defer client.Close()

	cw := newCountingWaker(nil)

	done := make(chan struct{})
	go func() {
		handleGeneric(context.Background(), server, cw, upstream.Addr().String(), 2*time.Second)
		server.Close() // mirrors handleTCPConnection's defer conn.Close() in production
		close(done)
	}()

	// Generic has no handshake to send; the simulated player just connects
	// and then, immediately, has nothing further to say.
	if tc, ok := client.(*net.TCPConn); ok {
		_ = tc.CloseWrite()
	}

	select {
	case c := <-upstreamConnCh:
		c.Close()
	case <-time.After(2 * time.Second):
		t.Fatal("upstream never accepted a connection")
	}

	<-done
	if cw.count() != 1 {
		t.Errorf("expected exactly 1 wake request for a generic join, got %d", cw.count())
	}
}

func TestHandleGenericClosesSilentlyOnDeadline(t *testing.T) {
	lc := &net.ListenConfig{}
	probe, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := probe.Addr().String()
	probe.Close()

	server, client := tcpPipe(t)
	defer client.Close()

	cw := newCountingWaker(nil)

	done := make(chan struct{})
	go func() {
		handleGeneric(context.Background(), server, cw, addr, 100*time.Millisecond)
		server.Close() // mirrors handleTCPConnection's defer conn.Close() in production
		close(done)
	}()

	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	got, err := io.ReadAll(client)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no bounce bytes for generic protocol, got %q", got)
	}

	<-done
}

func TestHandleTCPConnectionDispatchesByWakeProtocol(t *testing.T) {
	server, client := tcpPipe(t)
	defer client.Close()

	cw := newCountingWaker(nil)
	cfg := Config{GameServerName: "does-not-exist", GameServerNamespace: "nowhere", WakeDeadline: 50 * time.Millisecond}
	port := PortConfig{ContainerPort: 25565, Protocol: "TCP", WakeProtocol: "unknown-protocol"}

	// A wakeProtocol not in {minecraft,terraria,generic} should just close
	// without doing anything (defensive default in the switch).
	handleTCPConnection(context.Background(), server, port, cw, cfg)
	if cw.count() != 0 {
		t.Errorf("expected no wake requests for an unrecognized wakeProtocol, got %d", cw.count())
	}
}

// -----------------------------------------------------------------------
// run/serveTCP: bounded goroutines and clean shutdown on context cancel
// -----------------------------------------------------------------------

func freeTCPPort(t *testing.T) int {
	t.Helper()
	lc := &net.ListenConfig{}
	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func freeUDPPort(t *testing.T) int {
	t.Helper()
	pc, err := net.ListenUDP("udp", &net.UDPAddr{})
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	defer pc.Close()
	return pc.LocalAddr().(*net.UDPAddr).Port
}

func TestRunShutsDownOnContextCancel(t *testing.T) {
	cfg := Config{
		GameServerName:      "gs",
		GameServerNamespace: "games",
		WakeDeadline:        time.Second,
		UDPThreshold:        3,
		UDPWindow:           time.Second,
		UDPMaxSources:       10,
		MaxConnections:      4,
		WakePatchInterval:   time.Second,
		Ports: []PortConfig{
			{ContainerPort: freeTCPPort(t), Protocol: "TCP", WakeProtocol: "generic"},
			{ContainerPort: freeUDPPort(t), Protocol: "UDP", WakeProtocol: "generic"},
			{ContainerPort: 1, Protocol: "TCP", WakeProtocol: "none"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cw := newCountingWaker(nil)

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, cfg, cw)
	}()

	// Give the listeners a moment to bind.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("run() returned error = %v, want nil on clean shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run() did not return after context cancellation")
	}
}

func TestRunReturnsErrorOnListenFailure(t *testing.T) {
	lc := &net.ListenConfig{}
	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port

	cfg := Config{
		MaxConnections: 4,
		Ports: []PortConfig{
			{ContainerPort: port, Protocol: "TCP", WakeProtocol: "generic"},
		},
	}

	err = run(context.Background(), cfg, newCountingWaker(nil))
	if err == nil {
		t.Fatal("expected an error binding an already-in-use port")
	}
}

// TestServeTCPBoundsConcurrentHandlers proves the semaphore actually gates
// concurrent handler goroutines rather than just being decorative: with no
// upstream listening, every accepted connection's handler blocks for the
// full deadline inside waitForUpstream, so with a semaphore of capacity 1
// a second connection's handler cannot start running (i.e. cannot request
// a wake) until the first one's deadline has elapsed and it has exited.
func TestServeTCPBoundsConcurrentHandlers(t *testing.T) {
	lc := &net.ListenConfig{}
	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	port := PortConfig{ContainerPort: l.Addr().(*net.TCPAddr).Port, Protocol: "TCP", WakeProtocol: "generic"}
	// A short deadline against a game-direct hostname that can never
	// resolve: every handler's waitForUpstream call is guaranteed to run
	// (roughly) the full deadline before giving up, regardless of network
	// availability in the test environment, because DialContext bounds
	// even DNS resolution by the deadline-derived context.
	cfg := Config{GameServerName: "x", GameServerNamespace: "y", WakeDeadline: 300 * time.Millisecond}

	cw := newCountingWaker(nil)
	sem := make(chan struct{}, 1)
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go serveTCP(ctx, l, port, cw, cfg, sem, errCh)

	dialer := &net.Dialer{}
	c1, err := dialer.DialContext(context.Background(), "tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("dial 1: %v", err)
	}
	defer c1.Close()
	c2, err := dialer.DialContext(context.Background(), "tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("dial 2: %v", err)
	}
	defer c2.Close()

	// RequestWake happens immediately on entering handleJoin, before the
	// (possibly slow) upstream dial — so this alone would pass even
	// without the semaphore. It's the second assertion below that proves
	// the bound: the second handler's wake only happens once the first
	// releases the semaphore.
	deadline := time.Now().Add(2 * time.Second)
	for cw.count() < 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := cw.count(); got != 1 {
		t.Fatalf("expected exactly 1 wake request while the semaphore (capacity 1) is held by the first handler, got %d", got)
	}

	// The second connection's handler cannot even start — let alone
	// request a wake — until the first handler's deadline elapses and it
	// releases the semaphore.
	if cw.count() != 1 {
		t.Fatalf("second handler ran before the first released the semaphore")
	}

	deadline = time.Now().Add(3 * time.Second)
	for cw.count() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := cw.count(); got != 2 {
		t.Errorf("expected the second handler to have run once the semaphore freed up, got %d wake requests", got)
	}
}

// -----------------------------------------------------------------------
// test helpers: Minecraft/Terraria handshake encoders
// -----------------------------------------------------------------------

// encodeMinecraftHandshake hand-builds a Minecraft handshake packet
// (VarInt length prefix, packet ID 0x00, VarInt protocol version, string
// server address, uint16 port, VarInt next state). gameproto's own encoder
// is unexported, so this is a small independent re-implementation used only
// to drive the classifier from a test.
func encodeMinecraftHandshake(protocolVersion int32, host string, port uint16, nextState int32) []byte {
	var inner bytes.Buffer
	writeVarInt(&inner, 0x00) // packet ID
	writeVarInt(&inner, protocolVersion)
	writeVarInt(&inner, int32(len(host)))
	inner.WriteString(host)
	inner.WriteByte(byte(port >> 8))
	inner.WriteByte(byte(port))
	writeVarInt(&inner, nextState)

	var out bytes.Buffer
	writeVarInt(&out, int32(inner.Len()))
	out.Write(inner.Bytes())
	return out.Bytes()
}

func writeVarInt(w *bytes.Buffer, v int32) {
	uv := uint32(v)
	for {
		b := byte(uv & 0x7f)
		uv >>= 7
		if uv != 0 {
			b |= 0x80
		}
		w.WriteByte(b)
		if uv == 0 {
			return
		}
	}
}

// encodeTerrariaConnectRequest hand-builds a Terraria ConnectRequest frame
// (2-byte LE total length, message type 1, 7-bit-encoded-length version
// string).
func encodeTerrariaConnectRequest(version string) []byte {
	var payload bytes.Buffer
	write7BitEncodedInt(&payload, int32(len(version)))
	payload.WriteString(version)
	return encodeTerrariaMessage(1, payload.Bytes())
}

// encodeTerrariaMessage hand-builds an arbitrary Terraria message frame
// (2-byte LE total length, 1-byte message type, payload).
func encodeTerrariaMessage(msgType byte, payload []byte) []byte {
	var msg bytes.Buffer
	msg.WriteByte(msgType)
	msg.Write(payload)

	totalLength := msg.Len() + 2
	var out bytes.Buffer
	out.Write(binary.LittleEndian.AppendUint16(nil, uint16(totalLength)))
	out.Write(msg.Bytes())
	return out.Bytes()
}

func write7BitEncodedInt(w *bytes.Buffer, v int32) {
	uv := uint32(v)
	for {
		b := byte(uv & 0x7f)
		uv >>= 7
		if uv != 0 {
			b |= 0x80
		}
		w.WriteByte(b)
		if uv == 0 {
			return
		}
	}
}
