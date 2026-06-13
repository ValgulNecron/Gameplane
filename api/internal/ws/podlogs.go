package ws

import (
	"bufio"
	"context"
	"net/http"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"

	"github.com/kestrel-gg/kestrel/api/internal/httperr"
	"github.com/kestrel-gg/kestrel/api/internal/kube"
	"github.com/kestrel-gg/kestrel/api/internal/scope"
)

// mountPodLogs streams the game container's stdout/stderr over a
// WebSocket via the Kubernetes pod-log API. Unlike the agent's
// /logs/tail — which tails the configured game log file — this surfaces
// everything the container prints, including the binary/asset/mod
// download and config output during startup, before the game's own log
// file exists. Like the PTY attach it uses the API's in-cluster
// kubeconfig, so it works even when agent mTLS isn't configured.
//
// Each log line is delivered as a text WS frame, matching the agent
// /logs/tail protocol so the dashboard's Logs tab renders both
// identically.
func mountPodLogs(r chi.Router, k *kube.Client) {
	h := &podLogProxy{k: k}
	r.Get("/ws/servers/{name}/logs/pod", h.handle)
}

type podLogProxy struct {
	k *kube.Client
}

func (h *podLogProxy) handle(w http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")
	ns, err := scope.Resolve(req)
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	// StatefulSet replica naming — the operator pins replicas=1, so -0 is
	// the only pod that ever exists for a given GameServer.
	podName := name + "-0"

	opts := &corev1.PodLogOptions{Container: "game", Follow: true}
	// from=start streams the whole log (the default — best for watching a
	// server come up); from=end tails only the recent lines then follows.
	if req.URL.Query().Get("from") == "end" {
		tail := int64(200)
		opts.TailLines = &tail
	}

	conn, err := websocket.Accept(w, req, nil)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()

	stream, err := h.k.Typed.CoreV1().Pods(ns).GetLogs(podName, opts).Stream(ctx)
	if err != nil {
		// Pod not created yet, container not started, or transient API
		// error. The dashboard reconnects, so close cleanly rather than
		// erroring — the next attempt succeeds once the pod is up.
		_ = conn.Close(websocket.StatusTryAgainLater, "pod logs unavailable")
		return
	}
	defer stream.Close()

	// Drain the read side so a client-initiated close cancels ctx and
	// tears the log stream down promptly.
	go func() {
		for {
			if _, _, rerr := conn.Read(ctx); rerr != nil {
				cancel()
				return
			}
		}
	}()

	sc := bufio.NewScanner(stream)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if ctx.Err() != nil {
			return
		}
		// Copy the scanner's buffer — it's reused on the next Scan — and
		// re-add the newline streamFile/ReadString preserves, so the web
		// client's line splitter behaves the same for both sources.
		line := append([]byte(nil), sc.Bytes()...)
		line = append(line, '\n')
		if werr := conn.Write(ctx, websocket.MessageText, line); werr != nil {
			return
		}
	}
}
