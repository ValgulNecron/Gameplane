package ws

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ValgulNecron/gameplane/api/internal/httperr"
	"github.com/ValgulNecron/gameplane/api/internal/kube"
	"github.com/ValgulNecron/gameplane/api/internal/scope"
)

// gameContainer is the StatefulSet's main game container name (set by the
// operator). Init containers run before it.
const gameContainer = "game"

// podLogPollInterval is how often we re-check pod status while waiting for
// the next container in the startup sequence to begin emitting logs.
const podLogPollInterval = time.Second

// mountPodLogs streams a server's startup-through-runtime output over a
// WebSocket via the Kubernetes pod-log API. Unlike the agent's /logs/tail
// — which tails the configured game log file — this surfaces everything
// the pod prints. Like the PTY attach it uses the API's in-cluster
// kubeconfig, so it works even when agent mTLS isn't configured.
//
// from=start (the default, used while a server provisions) stitches the
// full timeline: each init/setup container's logs in order, then the game
// container — so the install/setup step is visible (and a setup *failure*
// shows its output) instead of a blank panel. from=end tails only the
// running game container's recent lines.
//
// Each log line is delivered as a text WS frame, matching the agent
// /logs/tail protocol so the dashboard's Logs tab renders both sources
// identically.
func mountPodLogs(r chi.Router, k *kube.Client) {
	h := &podLogProxy{k: k}
	// rejectRemoteCluster: this reads pod logs via the API's own in-cluster
	// kubeconfig, so it can only ever reach the LOCAL cluster — see its
	// doc comment in dialer.go for why a non-local `?cluster=` must 404.
	r.Get("/ws/servers/{name}/logs/pod", rejectRemoteCluster(h.handle))
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
	fromEnd := req.URL.Query().Get("from") == "end"

	conn, err := websocket.Accept(w, req, nil)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()

	// Drain the read side so a client-initiated close cancels ctx and tears
	// the log stream(s) down promptly.
	go func() {
		for {
			if _, _, rerr := conn.Read(ctx); rerr != nil {
				cancel()
				return
			}
		}
	}()

	if fromEnd {
		// Tail the running game container only — replaying setup logs makes
		// no sense for "follow a running server".
		tail := int64(200)
		if serr := h.streamContainer(ctx, conn, ns, podName, gameContainer, &tail); serr != nil && ctx.Err() == nil {
			_ = conn.Close(websocket.StatusTryAgainLater, "pod logs unavailable")
		}
		return
	}

	h.streamTimeline(ctx, conn, ns, podName)
}

// streamTimeline streams every init/setup container's logs in pod order,
// then the game container, over the one connection.
func (h *podLogProxy) streamTimeline(ctx context.Context, conn *websocket.Conn, ns, podName string) {
	pod, err := h.k.Typed.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		// Pod not created yet / transient API error. The dashboard
		// reconnects, so close cleanly rather than erroring.
		_ = conn.Close(websocket.StatusTryAgainLater, "pod unavailable")
		return
	}

	type step struct {
		name   string
		isInit bool
	}
	steps := make([]step, 0, len(pod.Spec.InitContainers)+1)
	for i := range pod.Spec.InitContainers {
		steps = append(steps, step{name: pod.Spec.InitContainers[i].Name, isInit: true})
	}
	steps = append(steps, step{name: gameContainer})
	// Phase markers only help when there's more than one phase to label.
	multi := len(steps) > 1

	for _, s := range steps {
		if !h.waitForStart(ctx, ns, podName, s.name, s.isInit) {
			return // ctx cancelled or pod vanished
		}
		if multi {
			h.writeMarker(ctx, conn, s.name)
		}
		// Follow to EOF: an init container's stream ends when it terminates;
		// the game container's follows until exit or client disconnect.
		if serr := h.streamContainer(ctx, conn, ns, podName, s.name, nil); serr != nil && ctx.Err() != nil {
			return
		}
		if s.isInit && h.containerFailed(ctx, ns, podName, s.name) {
			// A setup step failed → the game container will never start, so
			// stop here. The user sees the failing setup output, not a hang.
			return
		}
	}
}

// streamContainer copies one container's log lines to the WebSocket. A nil
// tail streams from the start; a non-nil tail limits to the recent lines
// before following.
func (h *podLogProxy) streamContainer(ctx context.Context, conn *websocket.Conn, ns, podName, container string, tail *int64) error {
	opts := &corev1.PodLogOptions{Container: container, Follow: true, TailLines: tail}
	stream, err := h.k.Typed.CoreV1().Pods(ns).GetLogs(podName, opts).Stream(ctx)
	if err != nil {
		return err
	}
	defer stream.Close()

	sc := bufio.NewScanner(stream)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Copy the scanner's buffer — it's reused on the next Scan — and
		// re-add the newline streamFile preserves, so the web client's line
		// splitter behaves the same for both sources.
		line := append([]byte(nil), sc.Bytes()...)
		line = append(line, '\n')
		if werr := conn.Write(ctx, websocket.MessageText, line); werr != nil {
			return werr
		}
	}
	return sc.Err()
}

// waitForStart blocks until the named container has begun emitting logs
// (Running or Terminated), polling pod status. It returns false when ctx
// is cancelled or the pod can't be read.
func (h *podLogProxy) waitForStart(ctx context.Context, ns, podName, container string, isInit bool) bool {
	for {
		if ctx.Err() != nil {
			return false
		}
		pod, err := h.k.Typed.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false
		}
		if started, _ := containerLogState(pod, container, isInit); started {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(podLogPollInterval):
		}
	}
}

// containerFailed reports whether the named container has terminated with a
// non-zero exit code (best-effort; false on read error).
func (h *podLogProxy) containerFailed(ctx context.Context, ns, podName, container string) bool {
	pod, err := h.k.Typed.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	_, failed := containerLogState(pod, container, true)
	return failed
}

// writeMarker emits a thin section header so the user can tell the setup
// phases apart in the stitched stream.
func (h *podLogProxy) writeMarker(ctx context.Context, conn *websocket.Conn, name string) {
	_ = conn.Write(ctx, websocket.MessageText, []byte(fmt.Sprintf("── %s ──\n", name)))
}

// containerLogState reports whether the named container's logs are
// available (it has started) and whether it already terminated non-zero.
func containerLogState(pod *corev1.Pod, name string, isInit bool) (started, failed bool) {
	statuses := pod.Status.ContainerStatuses
	if isInit {
		statuses = pod.Status.InitContainerStatuses
	}
	for i := range statuses {
		if statuses[i].Name != name {
			continue
		}
		switch st := statuses[i].State; {
		case st.Running != nil:
			return true, false
		case st.Terminated != nil:
			return true, st.Terminated.ExitCode != 0
		default:
			return false, false // waiting to start
		}
	}
	return false, false
}
