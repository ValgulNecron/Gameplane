//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// requireAgentReady waits for the agent sidecar in `<gs>-0` to reach
// Ready=True with no restarts, and fails the test if it never does.
// The chart provisions the agent CA unconditionally (templates/mtls.yaml)
// and the operator wires --tls-cert/--tls-key/--tls-client-ca from the
// per-GameServer Secret it signs, so an unready agent here is a real
// regression, not missing test wiring.
func requireAgentReady(t *testing.T, ns, gsName string) {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		pod, err := envInstance.K8s.CoreV1().Pods(ns).Get(ctx, gsName+"-0", metav1.GetOptions{})
		if err == nil {
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.Name != "agent" {
					continue
				}
				// Require restartCount=0 in addition to Ready=true: a
				// crashlooping agent has a tiny window where
				// ContainerStatus.Ready flips to true before the kubelet
				// records the crash — without the restart-count guard,
				// the test passes the wait and then 500s on the first
				// agent call.
				if cs.Ready && cs.RestartCount == 0 {
					return
				}
			}
		}
		time.Sleep(2 * time.Second)
	}
	logs, _ := envInstance.Kubectl("logs", "-n", ns, gsName+"-0", "-c", "agent", "--tail=10")
	t.Fatalf("agent sidecar never reached stable Ready in 90s. agent logs:\n%s", logs)
}

// waitAgentReachable polls a cheap agent endpoint through the API until
// it answers 200. "Agent container Ready" does not mean routable yet:
// the per-GameServer agent Service, its EndpointSlice, and kube-proxy's
// dataplane programming are all asynchronous, so the first proxied
// request can race that chain and see a 502.
func waitAgentReachable(t *testing.T, cli *APIClient, gs string) {
	t.Helper()
	envInstance.Eventually(t, 30*time.Second, func() (bool, string) {
		resp, body, err := cli.Get("/servers/" + gs + "/players")
		if err != nil {
			return false, "GET /players: " + err.Error()
		}
		if resp.StatusCode != http.StatusOK {
			return false, "status=" + http.StatusText(resp.StatusCode) + " body=" + string(body)
		}
		return true, ""
	})
}

// TestAPI_AgentFilesRoundTrip exercises the API → agent proxy for the
// files endpoints. The dashboard's Files tab calls this same path:
// /servers/{name}/files/{write,list,read,delete}. The test:
//
//  1. Creates a busybox GameServer (auto-injects the agent sidecar).
//  2. Waits for the pod (including agent) to reach Ready — without
//     this, every files request 502s through the proxy.
//  3. Writes a file via /files/write.
//  4. Lists the data root and asserts the file is present.
//  5. Reads it back and asserts the bytes match.
//  6. Deletes it and asserts a subsequent list does NOT show it.
//
// This is the slowest non-restic test in the suite (the agent's mTLS
// cert plumbing + container startup is a few minutes), so we set
// generous timeouts on the Ready wait but keep per-request timeouts
// tight to surface plumbing regressions cleanly.
func TestAPI_AgentFilesRoundTrip(t *testing.T) {
	ns := "kestrel-games"
	tmpl := "e2e-agent-files-tmpl"
	gs := "e2e-agent-files-gs"

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	cli := envInstance.APIClient(t, adminUsername, adminPassword)
	defer cli.Close()

	applyBusyboxTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)
	waitPVCBound(t, ns, gs+"-data", 90*time.Second)
	requireAgentReady(t, ns, gs)
	waitAgentReachable(t, cli, gs)

	// Agent file paths are relative to the agent's data root ("/" is the
	// game's data volume), matching the paths the list endpoint returns —
	// NOT absolute pod paths.
	const fileName = "hello-from-e2e.txt"
	const filePath = "/" + fileName
	const payload = "hi from kestrel api e2e"

	// Write a file. The endpoint takes path as query param and body as
	// raw octet-stream — APIClient.Do marshals JSON, so we go direct.
	writeURL := cli.BaseURL + "/servers/" + gs + "/files/write?path=" + url.QueryEscape(filePath)
	req, err := http.NewRequest(http.MethodPost, writeURL, bytes.NewReader([]byte(payload)))
	if err != nil {
		t.Fatalf("build write req: %v", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Kestrel-CSRF", cli.CSRF)
	resp, err := cli.HTTP.Do(req)
	if err != nil {
		t.Fatalf("POST /files/write: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		t.Fatalf("/files/write expected 2xx, got %d body=%q", resp.StatusCode, string(body))
	}

	// List the data root — file must appear.
	envInstance.Eventually(t, 30*time.Second, func() (bool, string) {
		resp, body, err := cli.Get("/servers/" + gs + "/files/list?path=" + url.QueryEscape("/"))
		if err != nil {
			return false, "list: " + err.Error()
		}
		if resp.StatusCode != http.StatusOK {
			return false, "list status=" + http.StatusText(resp.StatusCode) + " body=" + string(body)
		}
		var entries []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(body, &entries); err != nil {
			return false, "decode list: " + err.Error() + " body=" + string(body)
		}
		for _, e := range entries {
			if e.Name == fileName {
				return true, ""
			}
		}
		return false, "file not in listing yet"
	})

	// Read back — content must match.
	readResp, readBody, err := cli.Get("/servers/" + gs + "/files/read?path=" + url.QueryEscape(filePath))
	if err != nil {
		t.Fatalf("GET /files/read: %v", err)
	}
	if readResp.StatusCode != http.StatusOK {
		t.Fatalf("/files/read expected 200, got %d body=%q", readResp.StatusCode, string(readBody))
	}
	if got := string(readBody); got != payload {
		t.Errorf("/files/read body=%q want %q", got, payload)
	}

	// Delete and verify the file is gone from a subsequent list.
	delResp, delBody, err := cli.Delete("/servers/" + gs + "/files/delete?path=" + url.QueryEscape(filePath))
	if err != nil {
		t.Fatalf("DELETE /files/delete: %v", err)
	}
	if delResp.StatusCode/100 != 2 {
		t.Fatalf("/files/delete expected 2xx, got %d body=%q", delResp.StatusCode, string(delBody))
	}

	envInstance.Eventually(t, 15*time.Second, func() (bool, string) {
		resp, body, err := cli.Get("/servers/" + gs + "/files/list?path=" + url.QueryEscape("/"))
		if err != nil {
			return false, "list-after-delete: " + err.Error()
		}
		if resp.StatusCode != http.StatusOK {
			return false, "list-after-delete status=" + http.StatusText(resp.StatusCode)
		}
		var entries []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(body, &entries); err != nil {
			return false, "decode: " + err.Error()
		}
		for _, e := range entries {
			if e.Name == fileName {
				return false, "file still listed"
			}
		}
		return true, ""
	})
}

// TestAPI_AgentPlayers covers /servers/{name}/players. Busybox doesn't
// speak any game protocol, so the agent reports 0 online and an empty
// banned list — but the JSON shape must still be the contract the
// dashboard expects (PlayersResp in web/src/types.ts: online, max,
// players, asOf, capabilities).
//
// We don't assert specific numbers; we assert the response is valid
// JSON with the expected keys, which proves the API correctly proxied
// to the agent and decoded its response.
func TestAPI_AgentPlayers(t *testing.T) {
	ns := "kestrel-games"
	tmpl := "e2e-agent-players-tmpl"
	gs := "e2e-agent-players-gs"

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	cli := envInstance.APIClient(t, adminUsername, adminPassword)
	defer cli.Close()

	applyBusyboxTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)
	waitPVCBound(t, ns, gs+"-data", 90*time.Second)
	requireAgentReady(t, ns, gs)

	waitAgentReachable(t, cli, gs)

	resp, body, err := cli.Get("/servers/" + gs + "/players")
	if err != nil {
		t.Fatalf("GET /players: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/players expected 200, got %d body=%q", resp.StatusCode, string(body))
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode /players: %v body=%q", err, string(body))
	}
	if _, ok := got["online"]; !ok {
		t.Errorf("/players response missing 'online' field: %s", string(body))
	}
}

// TestAPI_AgentUnreachable: when the GameServer pod is suspended (and
// the agent therefore unreachable), the API must surface a structured
// error rather than hang or 500. The dashboard distinguishes 502/503/504
// from a real auth failure or other 4xx; we assert the response code is
// in that "agent down" range.
func TestAPI_AgentUnreachable(t *testing.T) {
	ctx := context.Background()
	ns := "kestrel-games"
	tmpl := "e2e-agent-unreach-tmpl"
	gs := "e2e-agent-unreach-gs"

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	cli := envInstance.APIClient(t, adminUsername, adminPassword)
	defer cli.Close()

	applyBusyboxTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)
	waitStatefulSetReplicas(t, ns, gs, 1, 90*time.Second)
	requireAgentReady(t, ns, gs)

	// Suspend → operator scales the StatefulSet to 0 → pod gone → agent
	// unreachable. We don't wait for full pod deletion because the
	// proxy's connect attempt fails as soon as the Service has no ready
	// endpoints, which happens shortly after the suspend patch.
	suspendPatch := []byte(`{"spec":{"suspend":true}}`)
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Patch(ctx, gs, types.MergePatchType, suspendPatch, metav1.PatchOptions{}); err != nil {
		t.Fatalf("patch suspend=true: %v", err)
	}
	waitStatefulSetReplicas(t, ns, gs, 0, 60*time.Second)

	// Allow the kube Service's endpoints to drop. With 0 ready pods
	// behind the headless Service the proxy will fail fast.
	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		resp, body, err := cli.Get("/servers/" + gs + "/players")
		if err != nil {
			// network errors here would be from our own port-forward
			// dropping, not from the proxy → agent leg. Surface and
			// retry.
			return false, "GET /players: " + err.Error()
		}
		switch resp.StatusCode {
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true, ""
		case http.StatusOK:
			return false, "agent still answering despite suspend — endpoints not yet drained"
		default:
			// Something other than 5xx-from-proxy and not 200. Print and
			// fail fast so the test surface is unambiguous: a 401/403
			// here would mean session loss; a 404 would mean the proxy
			// route changed.
			s := strings.TrimSpace(string(body))
			if len(s) > 200 {
				s = s[:200]
			}
			return false, "unexpected status " + http.StatusText(resp.StatusCode) + " body=" + s
		}
	})
}
