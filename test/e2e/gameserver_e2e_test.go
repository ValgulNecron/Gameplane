//go:build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// gameTemplateGVR / gameServerGVR — typed clients aren't generated for
// the test/e2e module; we use the dynamic client.
var (
	gameTemplateGVR = schema.GroupVersionResource{Group: "gameplane.local", Version: "v1alpha1", Resource: "gametemplates"}
	gameServerGVR   = schema.GroupVersionResource{Group: "gameplane.local", Version: "v1alpha1", Resource: "gameservers"}
)

// TestGameServer_OperatorMaterializesChildren — apply a tiny template
// + a GameServer that references it. The operator must produce a
// StatefulSet, Service, and PVC. We do NOT wait for pods to reach
// Ready — that requires a real game image and the kind node may not
// be able to pull large external images. The test asserts the operator
// shaped the right cluster objects, which is the contract that
// matters at the operator layer.
func TestGameServer_OperatorMaterializesChildren(t *testing.T) {
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
