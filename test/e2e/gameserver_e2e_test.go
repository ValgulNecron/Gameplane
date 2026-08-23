//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// gameTemplateGVR / gameServerGVR / networkCaptureGVR — typed clients
// aren't generated for the test/e2e module; we use the dynamic client.
var (
	gameTemplateGVR   = schema.GroupVersionResource{Group: "gameplane.local", Version: "v1alpha1", Resource: "gametemplates"}
	gameServerGVR     = schema.GroupVersionResource{Group: "gameplane.local", Version: "v1alpha1", Resource: "gameservers"}
	networkCaptureGVR = schema.GroupVersionResource{Group: "gameplane.local", Version: "v1alpha1", Resource: "networkcaptures"}
)

// TestGameServer_OperatorMaterializesChildren — apply a tiny template
// + a GameServer that references it. The operator must produce a
// StatefulSet, Service, and PVC. We do NOT wait for pods to reach
// Ready — that requires a real game image and the kind node may not
// be able to pull large external images. The test asserts the operator
// shaped the right cluster objects, which is the contract that
// matters at the operator layer.
func TestGameServer_OperatorMaterializesChildren(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"

	// Use a busybox-based "fake game" so the operator can construct a
	// pod spec that won't fail to render. Image is never actually
	// pulled here — we don't wait for the pod.
	tmplName := "e2e-busybox"
	tmpl := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameTemplate",
		"metadata":   map[string]any{"name": tmplName},
		"spec": map[string]any{
			"displayName": "E2E busybox",
			"game":        "busybox",
			"version":     "1",
			"image":       "busybox:1.36",
			"command":     []any{"sh", "-c", "sleep 100000"},
			"ports": []any{
				map[string]any{"name": "noop", "containerPort": int64(12345), "advertise": true, "protocol": "TCP"},
			},
			// Exercise spec.config materialization end to end: a value
			// set explicitly, a default the operator must fill in, and a
			// password that must land in a Secret instead of the pod spec.
			"configSchema": []any{
				map[string]any{"name": "DIFFICULTY", "type": "enum", "enum": []any{"easy", "hard"}, "default": "easy"},
				map[string]any{"name": "MAX_PLAYERS", "type": "int", "default": "16"},
				map[string]any{"name": "SERVER_PASS", "type": "password"},
				map[string]any{"name": "MOTD", "type": "string", "default": "hello e2e", "target": "file"},
			},
			// Exercise target:file materialization: the rendered file must
			// land in the <gs>-files Secret and be wired to the pod via the
			// config-init container, never into env.
			"configFiles": []any{
				map[string]any{
					"path":     "cfg/server.cfg",
					"template": "motd={{ .Values.MOTD }}\npass={{ .Values.SERVER_PASS }}\n",
				},
			},
		},
	}}
	if _, err := envInstance.Dyn.Resource(gameTemplateGVR).
		Create(ctx, tmpl, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create template: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameTemplateGVR).
			Delete(context.Background(), tmplName, metav1.DeleteOptions{})
	})

	gsName := "e2e-test-srv"
	gs := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameServer",
		"metadata":   map[string]any{"name": gsName, "namespace": ns},
		"spec": map[string]any{
			"templateRef": map[string]any{"name": tmplName},
			"config": map[string]any{
				"DIFFICULTY":  "hard",
				"SERVER_PASS": "e2e-secret",
			},
		},
	}}
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Create(ctx, gs, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create gameserver: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Delete(context.Background(), gsName, metav1.DeleteOptions{})
	})

	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		// StatefulSet
		if _, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, gsName, metav1.GetOptions{}); err != nil {
			return false, "ss: " + err.Error()
		}
		// Service
		if _, err := envInstance.K8s.CoreV1().Services(ns).Get(ctx, gsName, metav1.GetOptions{}); err != nil {
			return false, "svc: " + err.Error()
		}
		// PVC named <gs>-data
		if _, err := envInstance.K8s.CoreV1().PersistentVolumeClaims(ns).Get(ctx, gsName+"-data", metav1.GetOptions{}); err != nil {
			return false, "pvc: " + err.Error()
		}
		return true, ""
	})

	// Sanity-check the StatefulSet's pod template has the agent sidecar.
	ss, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, gsName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get statefulset: %v", err)
	}
	names := []string{}
	for _, c := range ss.Spec.Template.Spec.Containers {
		names = append(names, c.Name)
	}
	if !contains(names, "agent") {
		t.Errorf("sidecar missing — container names: %s", strings.Join(names, ","))
	}
	if !contains(names, "game") {
		t.Errorf("game container missing — container names: %s", strings.Join(names, ","))
	}

	// spec.config must be materialized: explicit value, schema default,
	// and the password routed through the <gs>-config Secret.
	for _, c := range ss.Spec.Template.Spec.Containers {
		if c.Name != "game" {
			continue
		}
		env := map[string]string{}
		var passRef string
		for _, e := range c.Env {
			env[e.Name] = e.Value
			if e.Value == "e2e-secret" {
				t.Errorf("password appears inline in pod spec env %s", e.Name)
			}
			if e.Name == "SERVER_PASS" && e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
				passRef = e.ValueFrom.SecretKeyRef.Name
			}
		}
		if env["DIFFICULTY"] != "hard" {
			t.Errorf("DIFFICULTY = %q, want hard", env["DIFFICULTY"])
		}
		if env["MAX_PLAYERS"] != "16" {
			t.Errorf("MAX_PLAYERS = %q, want default 16", env["MAX_PLAYERS"])
		}
		if passRef != gsName+"-config" {
			t.Errorf("SERVER_PASS SecretKeyRef = %q, want %s-config", passRef, gsName)
		}
		if _, ok := env["MOTD"]; ok {
			t.Errorf("file-target field MOTD leaked into game env")
		}
	}
	sec, err := envInstance.K8s.CoreV1().Secrets(ns).Get(ctx, gsName+"-config", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get config secret: %v", err)
	}
	if string(sec.Data["SERVER_PASS"]) != "e2e-secret" {
		t.Errorf("config secret does not hold the password")
	}

	// target:file config must be rendered into the <gs>-files Secret and
	// reach the pod through the config-init container.
	filesSec, err := envInstance.K8s.CoreV1().Secrets(ns).Get(ctx, gsName+"-files", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get files secret: %v", err)
	}
	if got := string(filesSec.Data["file-0"]); got != "motd=hello e2e\npass=e2e-secret\n" {
		t.Errorf("rendered file-0 = %q", got)
	}
	inits := ss.Spec.Template.Spec.InitContainers
	if len(inits) != 1 || inits[0].Name != "config-init" {
		t.Fatalf("expected a config-init init container, got %+v", inits)
	}
	foundVol := false
	for _, v := range ss.Spec.Template.Spec.Volumes {
		if v.Name != "config-files" {
			continue
		}
		foundVol = true
		if v.Secret == nil || v.Secret.SecretName != gsName+"-files" {
			t.Errorf("config-files volume not backed by %s-files: %+v", gsName, v)
		} else if len(v.Secret.Items) != 1 || v.Secret.Items[0].Key != "file-0" ||
			v.Secret.Items[0].Path != "cfg/server.cfg" {
			t.Errorf("config-files items do not map file-0 to cfg/server.cfg: %+v", v.Secret.Items)
		}
	}
	if !foundVol {
		t.Errorf("config-files volume missing from pod spec")
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// TestGameServer_PVCSurvivesPodDelete: deleting pod-0 must not destroy
// the persistent <gs>-data PVC. The StatefulSet's volumeClaimTemplate
// guarantees this in K8s, but a regression in how the operator scopes
// the PVC's owner references could tie its lifetime to the pod.
//
// We delete pod-0 and assert the StatefulSet recreates a pod with a
// different UID, while the PVC keeps the same UID throughout.
func TestGameServer_PVCSurvivesPodDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"
	tmpl := "e2e-pvc-survive-tmpl"
	gs := "e2e-pvc-survive-gs"

	applyBusyboxTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)
	waitPVCBound(t, ns, gs+"-data", 90*time.Second)
	waitStatefulSetReplicas(t, ns, gs, 1, 90*time.Second)

	// Wait for pod-0 to be present so we can capture its UID.
	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		_, err := envInstance.K8s.CoreV1().Pods(ns).Get(ctx, gs+"-0", metav1.GetOptions{})
		if err != nil {
			return false, "get pod: " + err.Error()
		}
		return true, ""
	})

	pvcPre, err := envInstance.K8s.CoreV1().PersistentVolumeClaims(ns).
		Get(ctx, gs+"-data", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get pvc pre-delete: %v", err)
	}
	pvcUID := pvcPre.UID

	podPre, err := envInstance.K8s.CoreV1().Pods(ns).Get(ctx, gs+"-0", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get pod pre-delete: %v", err)
	}
	oldPodUID := podPre.UID

	if err := envInstance.K8s.CoreV1().Pods(ns).
		Delete(ctx, gs+"-0", metav1.DeleteOptions{}); err != nil {
		t.Fatalf("delete pod-0: %v", err)
	}

	// StatefulSet recreates pod-0 with a fresh UID.
	envInstance.Eventually(t, 2*time.Minute, func() (bool, string) {
		pod, err := envInstance.K8s.CoreV1().Pods(ns).Get(ctx, gs+"-0", metav1.GetOptions{})
		if err != nil {
			return false, "get pod: " + err.Error()
		}
		if pod.UID == oldPodUID {
			return false, "pod still has old UID"
		}
		return true, ""
	})

	// PVC UID is unchanged — it must NOT have been recreated.
	pvcPost, err := envInstance.K8s.CoreV1().PersistentVolumeClaims(ns).
		Get(ctx, gs+"-data", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get pvc post-delete: %v", err)
	}
	if pvcPost.UID != pvcUID {
		t.Errorf("PVC UID changed after pod delete (pre=%s, post=%s) — pod ownership leaked into PVC lifetime",
			pvcUID, pvcPost.UID)
	}
}

// TestGameServer_HeartbeatReachesRunning: with the per-GameServer
// ServiceAccount, the heartbeat RBAC, and the agent->apiserver egress
// policy in place, the agent's status heartbeat must land and the
// operator must derive phase Running. Before those existed, no chart
// install could ever leave Starting — this is the regression guard.
func TestGameServer_HeartbeatReachesRunning(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"
	tmpl := "e2e-hb-tmpl"
	gs := "e2e-hb-gs"

	applyBusyboxTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)
	waitPVCBound(t, ns, gs+"-data", 90*time.Second)
	requireAgentReady(t, ns, gs)

	// Heartbeat interval is 20s and the status reconciler requeues every
	// ~15s, so a couple of minutes is comfortable without being flaky.
	envInstance.Eventually(t, 3*time.Minute, func() (bool, string) {
		obj, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Get(ctx, gs, metav1.GetOptions{})
		if err != nil {
			return false, "get gameserver: " + err.Error()
		}
		hb, _, _ := unstructured.NestedString(obj.Object, "status", "agent", "lastHeartbeat")
		if hb == "" {
			return false, "no heartbeat yet"
		}
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		if phase != "Running" {
			return false, "heartbeat present but phase=" + phase
		}
		return true, ""
	})
}

// TestGameServer_NetworkCaptureStartStopDownload — opt a GameServer into
// capture (spec.capture.enabled=true; the :capture-enable/:capture-disable
// HTTP routes are US2 and not yet built, so this is set directly on the
// spec rather than through the API), start a capture with a BPF filter
// restricted to a single advertised port, generate both filter-matching
// and non-matching traffic, stop the capture, download the file, and
// assert:
//
//   - SC-001 (third-party readability): the download parses cleanly as
//     structurally valid PCAPNG via gopacket's pcapgo.NewNgReader — a
//     real third-party parser, independent of the sidecar's own writer,
//     so a truncated or malformed file (right magic bytes, broken
//     structure) is caught rather than rubber-stamped. tshark/capinfos
//     are NOT installed anywhere in this repo's CI workflows (no step
//     in .github/workflows/*.yaml installs wireshark-common or
//     tshark, and the ubuntu-latest/ubuntu-24.04-arm runner images
//     don't ship them by default), so pcapgo is the actual third-party
//     verification that runs here — this comment is deliberately
//     explicit about that rather than claiming tshark coverage the
//     runner can't provide.
//   - SC-008 (filter correctness): every packet decoded from the file
//     carries the filter-matching TCP port, at least one such packet
//     exists (guards against a silently-empty-but-valid file passing
//     by coincidence), and zero packets carry the non-matching port.
func TestGameServer_NetworkCaptureStartStopDownload(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"
	tmpl := "e2e-cap-tmpl"
	gsName := "e2e-cap-gs"
	trafficPodName := "e2e-cap-traffic-gen"

	const (
		matchPort    = 12345 // advertised port from applyBusyboxTemplate
		nonMatchPort = 54321
	)

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	cli := envInstance.APIClient(t, adminUsername, adminPassword)
	defer cli.Close()

	applyBusyboxTemplate(t, tmpl)
	// spec.capture.enabled=true at creation time — see
	// applyBusyboxGameServerWithCapture's doc comment for why this
	// bypasses the (unbuilt) :capture-enable route. The operator
	// injects the capture sidecar as an ephemeral container as soon as
	// it observes the flag.
	applyBusyboxGameServerWithCapture(t, ns, gsName, tmpl)
	waitPVCBound(t, ns, gsName+"-data", 90*time.Second)
	requireAgentReady(t, ns, gsName)

	// The ephemeral capture container must reach Running
	// (status.capture.ready=true on the GameServer CRD) before we start
	// a capture against it — starting against an unready sidecar would
	// race the sidecar's own startup and could silently produce an
	// empty file.
	envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
		obj, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Get(ctx, gsName, metav1.GetOptions{})
		if err != nil {
			return false, "get gameserver: " + err.Error()
		}
		ready, _, _ := unstructured.NestedBool(obj.Object, "status", "capture", "ready")
		if !ready {
			return false, "status.capture.ready still false"
		}
		return true, ""
	})

	// Start a capture with a filter restricted to matchPort.
	startReq := map[string]any{
		"filter":                  fmt.Sprintf("tcp port %d", matchPort),
		"maxDurationSeconds":      300,
		"maxSizeBytes":            5368709120,
		"ttlSecondsAfterFinished": 86400,
	}
	startHTTPResp, startBody, err := cli.Post("/servers/"+gsName+":capture-start", startReq)
	if err != nil {
		t.Fatalf("start capture: %v", err)
	}
	defer func() { _ = startHTTPResp.Body.Close() }()
	if startHTTPResp.StatusCode != http.StatusAccepted {
		t.Fatalf("start capture: status=%s body=%s", startHTTPResp.Status, startBody)
	}
	var startedCapture struct {
		CaptureID string `json:"captureId"`
	}
	if err := json.Unmarshal(startBody, &startedCapture); err != nil {
		t.Fatalf("parse capture-start response %q: %v", startBody, err)
	}
	captureID := startedCapture.CaptureID
	if captureID == "" {
		t.Fatalf("capture-start response had no captureId: %s", startBody)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(networkCaptureGVR).Namespace(ns).
			Delete(context.Background(), captureID, metav1.DeleteOptions{})
	})

	// Wait for the operator to reconcile the NetworkCapture CRD from
	// Pending to Running — the sidecar isn't actually recording packets
	// until then, so generating traffic before this races capture start.
	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		obj, err := envInstance.Dyn.Resource(networkCaptureGVR).Namespace(ns).
			Get(ctx, captureID, metav1.GetOptions{})
		if err != nil {
			return false, "get networkcapture: " + err.Error()
		}
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		if phase != "Running" {
			return false, "phase=" + phase
		}
		return true, ""
	})

	// Get the pod IP to generate traffic against.
	pod, err := envInstance.K8s.CoreV1().Pods(ns).Get(ctx, gsName+"-0", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get pod: %v", err)
	}
	podIP := pod.Status.PodIP
	if podIP == "" {
		t.Fatalf("pod has no IP yet")
	}

	// The games namespace ships a default-deny-egress NetworkPolicy
	// (podSelector: {}, DNS-only) and the operator's own per-server
	// ingress policy admits only the advertised game port. Left as-is,
	// the traffic pod's connects to nonMatchPort would never leave its
	// own netns and the "zero packets on the non-matching port"
	// assertion below would pass because the CNI dropped the traffic,
	// not because BPF filtered it — proving nothing about SC-008.
	// NetworkPolicies for a given pod are additive (OR across every
	// policy that selects it), so adding explicit allow rules here
	// makes the traffic genuinely reach the game pod's netns without
	// having to touch (or fight) the always-on cluster policies.
	trafficLabels := map[string]string{"gameplane.local/e2e-role": "capture-traffic"}
	gamePodLabels := map[string]string{
		"app.kubernetes.io/name":     "gameplane-game",
		"app.kubernetes.io/instance": gsName,
	}
	tcpPorts := func() []networkingv1.NetworkPolicyPort {
		proto := corev1.ProtocolTCP
		matchP := intstr.FromInt(matchPort)
		nonMatchP := intstr.FromInt(nonMatchPort)
		return []networkingv1.NetworkPolicyPort{
			{Protocol: &proto, Port: &matchP},
			{Protocol: &proto, Port: &nonMatchP},
		}
	}
	ingressAllow := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: gsName + "-e2e-allow-capture-traffic-in", Namespace: ns},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: gamePodLabels},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				From:  []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: trafficLabels}}},
				Ports: tcpPorts(),
			}},
		},
	}
	egressAllow := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: trafficPodName + "-e2e-allow-capture-traffic-out", Namespace: ns},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: trafficLabels},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				To:    []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: gamePodLabels}}},
				Ports: tcpPorts(),
			}},
		},
	}
	if _, err := envInstance.K8s.NetworkingV1().NetworkPolicies(ns).Create(ctx, ingressAllow, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create ingress-allow networkpolicy: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.K8s.NetworkingV1().NetworkPolicies(ns).
			Delete(context.Background(), ingressAllow.Name, metav1.DeleteOptions{})
	})
	if _, err := envInstance.K8s.NetworkingV1().NetworkPolicies(ns).Create(ctx, egressAllow, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create egress-allow networkpolicy: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.K8s.NetworkingV1().NetworkPolicies(ns).
			Delete(context.Background(), egressAllow.Name, metav1.DeleteOptions{})
	})

	// A helper pod sends TCP connections to both the filter-matching and
	// non-matching ports. Neither port has a real listener behind it
	// (busybox never accepts connections), but the SYN/RST exchange is
	// captured at the network layer regardless of whether anything
	// answers. Uses `nc -w <secs> <ip> <port> </dev/null` rather than
	// `-zv`: busybox's nc applet only supports -z/-v when built with
	// NC_110_COMPAT, which the common (e.g. Alpine) build does not
	// enable — `-w` plus redirecting stdin from /dev/null is universally
	// supported and behaves the same way (connect, then exit).
	trafficPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      trafficPodName,
			Namespace: ns,
			Labels:    trafficLabels,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:  "traffic-gen",
					Image: "busybox:1.36",
					Command: []string{
						"sh", "-c",
						fmt.Sprintf(
							"(nc -w 2 %s %d </dev/null 2>&1; echo done-match) & "+
								"(nc -w 2 %s %d </dev/null 2>&1; echo done-nonmatch) & wait",
							podIP, matchPort, podIP, nonMatchPort,
						),
					},
				},
			},
		},
	}
	if _, err := envInstance.K8s.CoreV1().Pods(ns).Create(ctx, trafficPod, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create traffic pod: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.K8s.CoreV1().Pods(ns).Delete(context.Background(), trafficPodName, metav1.DeleteOptions{})
	})

	// Poll for the traffic pod to finish rather than a bare sleep — a
	// loaded CI runner can blow past a fixed sleep before scheduling
	// even starts the pod.
	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		p, err := envInstance.K8s.CoreV1().Pods(ns).Get(ctx, trafficPodName, metav1.GetOptions{})
		if err != nil {
			return false, "get traffic pod: " + err.Error()
		}
		if p.Status.Phase != corev1.PodSucceeded && p.Status.Phase != corev1.PodFailed {
			return false, "traffic pod phase=" + string(p.Status.Phase)
		}
		return true, ""
	})

	// Log the traffic pod's output before any packet-count assertion —
	// if the capture later comes back empty, this is what tells us
	// whether the traffic generator actually ran (vs. a capture/CNI
	// problem further down the chain).
	if trafficLogs, err := envInstance.Kubectl(ctx, "logs", "-n", ns, trafficPodName); err != nil {
		t.Logf("traffic pod logs: (failed to fetch: %v)", err)
	} else {
		t.Logf("traffic pod logs:\n%s", trafficLogs)
	}

	// Stop the capture. The contract takes captureId in the JSON body,
	// not a query parameter.
	stopHTTPResp, stopBody, err := cli.Post("/servers/"+gsName+":capture-stop", map[string]any{
		"captureId": captureID,
	})
	if err != nil {
		t.Fatalf("stop capture: %v", err)
	}
	defer func() { _ = stopHTTPResp.Body.Close() }()
	if stopHTTPResp.StatusCode != http.StatusOK {
		t.Fatalf("stop capture: status=%s body=%s", stopHTTPResp.Status, stopBody)
	}

	// The capture must reach a terminal phase before the file is
	// guaranteed downloadable.
	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		obj, err := envInstance.Dyn.Resource(networkCaptureGVR).Namespace(ns).
			Get(ctx, captureID, metav1.GetOptions{})
		if err != nil {
			return false, "get networkcapture: " + err.Error()
		}
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		if phase != "Completed" {
			return false, "phase=" + phase
		}
		return true, ""
	})

	// Download the capture file.
	fileHTTPResp, fileBody, err := cli.Get("/servers/" + gsName + ":capture-file?id=" + captureID)
	if err != nil {
		t.Fatalf("download capture file: %v", err)
	}
	defer func() { _ = fileHTTPResp.Body.Close() }()
	if fileHTTPResp.StatusCode != http.StatusOK {
		t.Fatalf("download capture file: status=%s", fileHTTPResp.Status)
	}
	if len(fileBody) == 0 {
		t.Fatalf("capture file is empty")
	}

	// SC-001: parse with gopacket's pcapgo reader — a genuine
	// third-party PCAPNG parser distinct from the sidecar's own writer.
	reader, err := pcapgo.NewNgReader(bytes.NewReader(fileBody), pcapgo.DefaultNgReaderOptions)
	if err != nil {
		t.Fatalf("capture file is not valid PCAPNG: %v", err)
	}

	// SC-008: every decoded packet must carry matchPort, and at least
	// one packet must be present.
	matchedPackets := 0
	for {
		data, _, err := reader.ReadPacketData()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("read packet from capture: %v", err)
		}
		packet := gopacket.NewPacket(data, reader.LinkType(), gopacket.Default)
		tcpLayer := packet.Layer(layers.LayerTypeTCP)
		if tcpLayer == nil {
			t.Fatalf("captured packet has no TCP layer despite filter %q: %s", startReq["filter"], packet.String())
		}
		tcp, ok := tcpLayer.(*layers.TCP)
		if !ok {
			t.Fatalf("TCP layer type assertion failed for packet: %s", packet.String())
		}
		srcPort := uint16(tcp.SrcPort)
		dstPort := uint16(tcp.DstPort)
		if srcPort != matchPort && dstPort != matchPort {
			t.Errorf("captured packet on neither src nor dst port %d: src=%d dst=%d", matchPort, srcPort, dstPort)
		}
		if srcPort == nonMatchPort || dstPort == nonMatchPort {
			t.Errorf("captured a packet on the non-matching port %d: src=%d dst=%d", nonMatchPort, srcPort, dstPort)
		}
		matchedPackets++
	}
	if matchedPackets == 0 {
		t.Fatalf("capture file contains zero packets — filter correctness (SC-008) cannot be verified against an empty capture")
	}
	t.Logf("capture file size=%d bytes, packets=%d, all matched filter %q (SC-001+SC-008 verified)",
		len(fileBody), matchedPackets, startReq["filter"])
}

// findContainerByName returns the container named `name` from cs, or nil.
func findContainerByName(cs []corev1.Container, name string) *corev1.Container {
	for i := range cs {
		if cs[i].Name == name {
			return &cs[i]
		}
	}
	return nil
}

// findContainerStatusByName returns the container status named `name`
// from css, or nil.
func findContainerStatusByName(css []corev1.ContainerStatus, name string) *corev1.ContainerStatus {
	for i := range css {
		if css[i].Name == name {
			return &css[i]
		}
	}
	return nil
}

// TestGameServer_NetworkCaptureEphemeralContainer — the structural
// counterpart to TestGameServer_NetworkCaptureStartStopDownload: no real
// packet traffic here, just the injection/removal-refusal contract for
// the capture ephemeral container.
//
// FR-001 as amended: enabling capture must not perturb the running game
// workload — no game-container restart, no image/securityContext/mount
// change. We snapshot the game container before enabling and diff it
// against the same fields after.
//
// The disable half asserts the documented asymmetry explicitly: disable
// stops the capability (status.capture.ready flips false, new
// :capture-start calls are refused) but Kubernetes provides no API to
// remove an already-injected ephemeral container, so its entry in
// pod.status.ephemeralContainerStatuses (and pod.spec.ephemeralContainers)
// legitimately lingers. That lingering is the ratified platform
// behavior — assert it stays present, don't "fix" the test if it does.
func TestGameServer_NetworkCaptureEphemeralContainer(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"
	tmpl := "e2e-cap-ec-tmpl"
	gsName := "e2e-cap-ec-gs"

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	cli := envInstance.APIClient(t, adminUsername, adminPassword)
	defer cli.Close()

	applyBusyboxTemplate(t, tmpl)
	// Created WITHOUT capture enabled — this test drives enable/disable
	// entirely through the :capture-enable/:capture-disable HTTP routes.
	applyBusyboxGameServer(t, ns, gsName, tmpl)
	waitPVCBound(t, ns, gsName+"-data", 90*time.Second)
	requireAgentReady(t, ns, gsName)

	podPre, err := envInstance.K8s.CoreV1().Pods(ns).Get(ctx, gsName+"-0", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get pod pre-enable: %v", err)
	}
	if len(podPre.Spec.EphemeralContainers) != 0 {
		t.Fatalf("pod already has ephemeral containers before capture was ever enabled: %+v",
			podPre.Spec.EphemeralContainers)
	}
	gameContainerPre := findContainerByName(podPre.Spec.Containers, "game")
	if gameContainerPre == nil {
		t.Fatalf("no game container in pre-enable pod spec")
	}
	gameStatusPre := findContainerStatusByName(podPre.Status.ContainerStatuses, "game")
	if gameStatusPre == nil {
		t.Fatalf("no game container status in pre-enable pod")
	}
	preImage := gameContainerPre.Image
	preSecurityContext := gameContainerPre.SecurityContext
	preVolumeMounts := gameContainerPre.VolumeMounts
	preRestartCount := gameStatusPre.RestartCount

	// Enable capture.
	enableHTTPResp, enableBody, err := cli.Post("/servers/"+gsName+":capture-enable", nil)
	if err != nil {
		t.Fatalf("enable capture: %v", err)
	}
	defer func() { _ = enableHTTPResp.Body.Close() }()
	if enableHTTPResp.StatusCode != http.StatusOK {
		t.Fatalf("enable capture: status=%s body=%s", enableHTTPResp.Status, enableBody)
	}

	// The ephemeral container appears on the pod spec as soon as the
	// operator observes spec.capture.enabled=true — no pod recreation.
	var capEC corev1.EphemeralContainer
	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		pod, err := envInstance.K8s.CoreV1().Pods(ns).Get(ctx, gsName+"-0", metav1.GetOptions{})
		if err != nil {
			return false, "get pod: " + err.Error()
		}
		for _, ec := range pod.Spec.EphemeralContainers {
			if ec.Name == "capture" {
				capEC = ec
				return true, ""
			}
		}
		return false, "no capture ephemeral container in pod spec yet"
	})

	if !strings.Contains(capEC.Image, "capture-sidecar") {
		t.Errorf("capture ephemeral container image = %q, want it to contain capture-sidecar", capEC.Image)
	}
	mountsCaptures := false
	for _, m := range capEC.VolumeMounts {
		if m.Name == "captures" {
			mountsCaptures = true
			break
		}
	}
	if !mountsCaptures {
		t.Errorf("capture ephemeral container does not mount the captures volume: %+v", capEC.VolumeMounts)
	}
	sc := capEC.SecurityContext
	if sc == nil {
		t.Fatalf("capture ephemeral container has no securityContext")
	}
	if sc.AllowPrivilegeEscalation == nil || !*sc.AllowPrivilegeEscalation {
		t.Errorf("capture ephemeral container allowPrivilegeEscalation = %v, want true", sc.AllowPrivilegeEscalation)
	}
	if sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
		t.Errorf("capture ephemeral container runAsNonRoot = %v, want true", sc.RunAsNonRoot)
	}
	if sc.Capabilities != nil && len(sc.Capabilities.Add) != 0 {
		t.Errorf("capture ephemeral container securityContext.capabilities.add = %v, want none (file capabilities only)",
			sc.Capabilities.Add)
	}

	// The game container itself must be untouched: same image, same
	// securityContext, same mounts, no restart.
	podPostEnable, err := envInstance.K8s.CoreV1().Pods(ns).Get(ctx, gsName+"-0", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get pod post-enable: %v", err)
	}
	gameContainerPost := findContainerByName(podPostEnable.Spec.Containers, "game")
	if gameContainerPost == nil {
		t.Fatalf("no game container in post-enable pod spec")
	}
	if gameContainerPost.Image != preImage {
		t.Errorf("game container image changed after enabling capture: pre=%q post=%q", preImage, gameContainerPost.Image)
	}
	if !reflect.DeepEqual(gameContainerPost.SecurityContext, preSecurityContext) {
		t.Errorf("game container securityContext changed after enabling capture: pre=%+v post=%+v",
			preSecurityContext, gameContainerPost.SecurityContext)
	}
	if !reflect.DeepEqual(gameContainerPost.VolumeMounts, preVolumeMounts) {
		t.Errorf("game container volumeMounts changed after enabling capture: pre=%+v post=%+v",
			preVolumeMounts, gameContainerPost.VolumeMounts)
	}
	gameStatusPost := findContainerStatusByName(podPostEnable.Status.ContainerStatuses, "game")
	if gameStatusPost == nil {
		t.Fatalf("no game container status in post-enable pod")
	}
	if gameStatusPost.RestartCount != preRestartCount {
		t.Errorf("game container restart count changed after enabling capture: pre=%d post=%d",
			preRestartCount, gameStatusPost.RestartCount)
	}

	// Wait for the sidecar to actually come up before disabling — this
	// mirrors TestGameServer_NetworkCaptureStartStopDownload's rationale:
	// disabling before the sidecar was ever observed ready would still
	// exercise the asymmetry, but waiting first proves injection worked
	// end to end, not just that the pod spec mutation landed.
	envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
		obj, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Get(ctx, gsName, metav1.GetOptions{})
		if err != nil {
			return false, "get gameserver: " + err.Error()
		}
		ready, _, _ := unstructured.NestedBool(obj.Object, "status", "capture", "ready")
		if !ready {
			return false, "status.capture.ready still false"
		}
		return true, ""
	})

	// Disable capture.
	disableHTTPResp, disableBody, err := cli.Post("/servers/"+gsName+":capture-disable", nil)
	if err != nil {
		t.Fatalf("disable capture: %v", err)
	}
	defer func() { _ = disableHTTPResp.Body.Close() }()
	if disableHTTPResp.StatusCode != http.StatusOK {
		t.Fatalf("disable capture: status=%s body=%s", disableHTTPResp.Status, disableBody)
	}

	// Half one of the asymmetry: status.capture.ready flips false
	// immediately (the operator does not wait for the ephemeral container
	// to actually stop — it can't force that anyway).
	envInstance.Eventually(t, 30*time.Second, func() (bool, string) {
		obj, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Get(ctx, gsName, metav1.GetOptions{})
		if err != nil {
			return false, "get gameserver: " + err.Error()
		}
		ready, _, _ := unstructured.NestedBool(obj.Object, "status", "capture", "ready")
		if ready {
			return false, "status.capture.ready still true"
		}
		active, found, _ := unstructured.NestedString(obj.Object, "status", "capture", "activeCapture")
		if found && active != "" {
			return false, "status.capture.activeCapture still set to " + active
		}
		return true, ""
	})

	// Half one, continued: a new capture is refused once disabled.
	startHTTPResp, startBody, err := cli.Post("/servers/"+gsName+":capture-start", map[string]any{
		"filter":             "tcp port 12345",
		"maxDurationSeconds": 60,
		"maxSizeBytes":       1048576,
	})
	if err != nil {
		t.Fatalf("capture-start after disable: %v", err)
	}
	defer func() { _ = startHTTPResp.Body.Close() }()
	if startHTTPResp.StatusCode != http.StatusBadRequest {
		t.Errorf("capture-start after disable: status=%s body=%s, want 400 (capture not enabled)",
			startHTTPResp.Status, startBody)
	}

	// Half two of the asymmetry, the one nobody should ever "fix": the
	// ephemeral container Kubernetes already injected cannot be removed
	// through any Kubernetes API (there is no pods/ephemeralcontainers
	// delete/remove operation), so both its spec entry and its
	// status.ephemeralContainerStatuses entry legitimately remain on the
	// pod after disable. This is documented, ratified platform behavior
	// (see FR-001's amendment and US2 acceptance scenario 4 in spec.md) —
	// asserting its absence here would be asserting a capability
	// Kubernetes does not offer.
	podPostDisable, err := envInstance.K8s.CoreV1().Pods(ns).Get(ctx, gsName+"-0", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get pod post-disable: %v", err)
	}
	specStillPresent := false
	for _, ec := range podPostDisable.Spec.EphemeralContainers {
		if ec.Name == "capture" {
			specStillPresent = true
			break
		}
	}
	if !specStillPresent {
		t.Errorf("capture ephemeral container vanished from pod.spec.ephemeralContainers after disable — " +
			"Kubernetes has no API to remove an ephemeral container; this entry should linger until the pod " +
			"is recreated (see FR-001's disable amendment)")
	}
	statusStillPresent := false
	for _, cs := range podPostDisable.Status.EphemeralContainerStatuses {
		if cs.Name == "capture" {
			statusStillPresent = true
			break
		}
	}
	if !statusStillPresent {
		t.Errorf("capture entry vanished from pod.status.ephemeralContainerStatuses after disable — " +
			"Kubernetes has no API to remove an ephemeral container; this entry should linger until the pod " +
			"is recreated (see FR-001's disable amendment)")
	}
}
