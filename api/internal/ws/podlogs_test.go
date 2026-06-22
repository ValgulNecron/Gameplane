package ws

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/ValgulNecron/gameplane/api/internal/kube"
)

// newPod builds a fake pod for the default namespace with a running game
// container and the given (already-terminated) init containers.
func newPod(name string, initNames []string, initFailed bool) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "kestrel-games"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "game"}}},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "game", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
			},
		},
	}
	for _, n := range initNames {
		code := int32(0)
		if initFailed {
			code = 1
		}
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{Name: n})
		pod.Status.InitContainerStatuses = append(pod.Status.InitContainerStatuses, corev1.ContainerStatus{
			Name:  n,
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: code}},
		})
	}
	return pod
}

func dialPodLogs(t *testing.T, k *kube.Client, query string) (string, error) {
	t.Helper()
	r := chi.NewRouter()
	mountPodLogs(r, k)
	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/servers/alpha/logs/pod" + query
	cli, resp, err := websocket.Dial(ctx, url, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return "", err
	}
	defer cli.Close(websocket.StatusNormalClosure, "")

	var b strings.Builder
	for {
		_, data, rerr := cli.Read(ctx)
		if rerr != nil {
			return b.String(), nil
		}
		b.Write(data)
	}
}

// from=start with only a game container: no phase marker, just the stream.
func TestPodLogs_StreamsGameOnly(t *testing.T) {
	k := &kube.Client{Typed: fake.NewSimpleClientset(newPod("alpha-0", nil, false))}
	out, err := dialPodLogs(t, k, "")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if !strings.Contains(out, "fake logs") {
		t.Errorf("out = %q, want it to contain fake logs", out)
	}
	if strings.Contains(out, "──") {
		t.Errorf("single container should emit no phase marker, got %q", out)
	}
}

// from=start stitches the init container then the game container, each
// labelled with a phase marker.
func TestPodLogs_StitchesInitThenGame(t *testing.T) {
	k := &kube.Client{Typed: fake.NewSimpleClientset(newPod("alpha-0", []string{"config-init"}, false))}
	out, err := dialPodLogs(t, k, "")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if !strings.Contains(out, "── config-init ──") || !strings.Contains(out, "── game ──") {
		t.Errorf("expected both phase markers, got %q", out)
	}
	if !strings.Contains(out, "fake logs") {
		t.Errorf("expected streamed output, got %q", out)
	}
}

// A failed setup container stops the timeline (the game never starts), so
// its output is shown and the game marker is not.
func TestPodLogs_StopsOnFailedInit(t *testing.T) {
	k := &kube.Client{Typed: fake.NewSimpleClientset(newPod("alpha-0", []string{"config-init"}, true))}
	out, err := dialPodLogs(t, k, "")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if !strings.Contains(out, "── config-init ──") {
		t.Errorf("expected config-init marker, got %q", out)
	}
	if strings.Contains(out, "── game ──") {
		t.Errorf("game should not stream after a failed init, got %q", out)
	}
}

// from=start with no pod yet closes cleanly (the client reconnects).
func TestPodLogs_NoPodYet(t *testing.T) {
	k := &kube.Client{Typed: fake.NewSimpleClientset()}
	if _, err := dialPodLogs(t, k, ""); err != nil {
		t.Fatalf("dial: %v", err)
	}
}

// from=end tails the game container directly, without enumerating the pod.
func TestPodLogs_TailsFromEnd(t *testing.T) {
	k := &kube.Client{Typed: fake.NewSimpleClientset()}
	out, err := dialPodLogs(t, k, "?from=end")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if !strings.Contains(out, "fake logs") {
		t.Errorf("out = %q, want fake logs", out)
	}
}
