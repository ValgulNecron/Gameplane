package ws

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/kestrel-gg/kestrel/api/internal/kube"
)

// TestPodLogs_StreamsContainerStdout dials the pod-log WS and asserts the
// game container's stdout is delivered as text frames. The fake clientset
// serves "fake logs" over the pods/log subresource.
func TestPodLogs_StreamsContainerStdout(t *testing.T) {
	k := &kube.Client{Typed: fake.NewSimpleClientset()}
	r := chi.NewRouter()
	mountPodLogs(r, k)
	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/servers/alpha/logs/pod"
	cli, resp, err := websocket.Dial(ctx, url, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer cli.Close(websocket.StatusNormalClosure, "")

	mt, data, err := cli.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if mt != websocket.MessageText {
		t.Fatalf("frame kind = %v, want text", mt)
	}
	if !strings.Contains(string(data), "fake logs") {
		t.Fatalf("frame = %q, want it to contain %q", data, "fake logs")
	}
}

// TestPodLogs_TailsFromEnd exercises the from=end branch (TailLines set).
func TestPodLogs_TailsFromEnd(t *testing.T) {
	k := &kube.Client{Typed: fake.NewSimpleClientset()}
	r := chi.NewRouter()
	mountPodLogs(r, k)
	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/servers/alpha/logs/pod?from=end"
	cli, resp, err := websocket.Dial(ctx, url, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer cli.Close(websocket.StatusNormalClosure, "")

	if _, _, err := cli.Read(ctx); err != nil {
		t.Fatalf("read: %v", err)
	}
}
