//go:build e2e

package e2e

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ptyEnvelope mirrors the wire protocol in api/internal/ws/attach.go.
// Defined locally so the e2e module doesn't import api/internal types.
type ptyEnvelope struct {
	Kind string `json:"kind"`
	Body string `json:"body,omitempty"`
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
}

// TestAPI_ConsolePTYRoundTrip dials /ws/servers/{name}/console-pty,
// sends a known stdin command, and verifies the marker echoes back on
// stdout. Exercises the full path from the dashboard's WS bridge through
// the API's pod-attach helper into a busybox `sh` running with a TTY.
//
// Doesn't depend on the agent sidecar: console-pty attaches via the
// kubelet API, not the agent's mTLS endpoint.
func TestAPI_ConsolePTYRoundTrip(t *testing.T) {
	ctx := context.Background()
	ns := "kestrel-games"
	tmpl := "e2e-ws-pty-tmpl"
	gs := "e2e-ws-pty-gs"

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	cli := envInstance.APIClient(t, adminUsername, adminPassword)
	defer cli.Close()

	applyBusyboxPTYTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)

	// We need the game container actually running with a TTY before the
	// attach can succeed. Wait for the game container in pod-0 to be
	// Ready; the agent sidecar is irrelevant to this attach path.
	envInstance.Eventually(t, 3*time.Minute, func() (bool, string) {
		pod, err := envInstance.K8s.CoreV1().Pods(ns).Get(ctx, gs+"-0", metav1.GetOptions{})
		if err != nil {
			return false, "get pod: " + err.Error()
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name == "game" && cs.Ready {
				return true, ""
			}
		}
		return false, "game container not ready yet"
	})

	wsConn, stop := dialAuthedWS(t, cli, "/ws/servers/"+gs+"/console-pty")
	defer stop()

	const marker = "kestrel-pty-marker-12345"
	stdinCmd := []byte("echo " + marker + "\n")
	envOut := ptyEnvelope{
		Kind: "stdin",
		Body: base64.StdEncoding.EncodeToString(stdinCmd),
	}
	envBytes, err := json.Marshal(envOut)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	writeCtx, writeCancel := context.WithTimeout(ctx, 10*time.Second)
	defer writeCancel()
	if err := wsConn.Write(writeCtx, websocket.MessageText, envBytes); err != nil {
		t.Fatalf("write stdin envelope: %v", err)
	}

	// Read until we either see the marker decoded out of a stdout
	// envelope, or hit a 30s deadline. Busybox `sh` echoes the typed
	// command back AND emits the result, so the marker should appear at
	// least once across the frames we read.
	readCtx, readCancel := context.WithTimeout(ctx, 30*time.Second)
	defer readCancel()
	for {
		_, frame, err := wsConn.Read(readCtx)
		if err != nil {
			t.Fatalf("read frame (no marker seen): %v", err)
		}
		var env ptyEnvelope
		if err := json.Unmarshal(frame, &env); err != nil {
			continue
		}
		if env.Kind == "err" {
			t.Fatalf("server-side error envelope: %s", env.Body)
		}
		if env.Kind != "stdout" {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(env.Body)
		if err != nil {
			continue
		}
		if strings.Contains(string(raw), marker) {
			return
		}
	}
}

// TestAPI_LogsTailWS dials /ws/servers/{name}/logs and verifies a known
// marker emitted by the game container surfaces in the streamed frames.
//
// SKIPS in CI: /ws/servers/{name}/logs is proxied to the agent sidecar
// (which is not yet auth-wired in the e2e helm chart). The test runs
// locally against a manually-provisioned cluster where the agent is
// reachable. requireAgentReady drives the skip.
func TestAPI_LogsTailWS(t *testing.T) {
	ctx := context.Background()
	ns := "kestrel-games"
	tmpl := "e2e-ws-logs-tmpl"
	gs := "e2e-ws-logs-gs"
	const marker = "kestrel-log-marker-67890"

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	cli := envInstance.APIClient(t, adminUsername, adminPassword)
	defer cli.Close()

	applyBusyboxLogTickerTemplate(t, tmpl, marker)
	applyBusyboxGameServer(t, ns, gs, tmpl)

	waitPVCBound(t, ns, gs+"-data", 90*time.Second)
	requireAgentReady(t, ns, gs)

	wsConn, stop := dialAuthedWS(t, cli, "/ws/servers/"+gs+"/logs")
	defer stop()

	readCtx, readCancel := context.WithTimeout(ctx, 30*time.Second)
	defer readCancel()
	for {
		_, frame, err := wsConn.Read(readCtx)
		if err != nil {
			t.Fatalf("read log frame: %v (no marker observed)", err)
		}
		if strings.Contains(string(frame), marker) {
			return
		}
	}
}
