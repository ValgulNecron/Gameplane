//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	minecraftproto "github.com/ValgulNecron/gameplane/test/e2e/internal/minecraft-java/protocol"
)

// TestGameServer_WakeOnConnect_PingDoesNotWake verifies that a server-list ping
// against a sleeping, wake-on-connect-armed server does NOT wake it. The sentinel
// should answer the ping, not the game pod. This is the behaviour most likely to
// regress and the reason the feature parses handshakes at all: protecting against
// "waking on every network probe".
//
// Setup: arm wakeOnConnect, drive to sleep, verify sentinel exists and service
// routes to it. Then: Ping the server. Assert the status response is intact
// AND the server is still asleep afterwards (annotation still present, replicas=0).
func TestGameServer_WakeOnConnect_PingDoesNotWake(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"
	tmpl := "e2e-wake-on-connect-ping"
	gs := "e2e-wake-on-connect-ping-test"

	// Create a Minecraft-like template with wakeOnConnect capability.
	applyMinecraftTemplateWithWake(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)
	waitPVCBound(t, ns, gs+"-data", 90*time.Second)

	// Wait for the server to reach Running.
	envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
		obj, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Get(ctx, gs, metav1.GetOptions{})
		if err != nil {
			return false, "get gameserver: " + err.Error()
		}
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		if phase != "Running" {
			return false, "phase=" + phase
		}
		return true, ""
	})

	// Enable idle auto-sleep with wakeOnConnect armed.
	idlePatch := []byte(`{"spec":{"idle":{"enabled":true,"afterMinutes":5,"wakeOnConnect":true}}}`)
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Patch(ctx, gs, types.MergePatchType, idlePatch, metav1.PatchOptions{}); err != nil {
		t.Fatalf("patch idle+wakeOnConnect: %v", err)
	}

	// Drive the server to sleep by stamping the authoritative sleep marker.
	// (The annotation, not the status field, is what drives the reconciler's decision.)
	past := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	sleepPatch := []byte(`{"metadata":{"annotations":{"gameplane.local/idle-asleep-since":"` + past + `"}}}`)
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Patch(ctx, gs, types.MergePatchType, sleepPatch, metav1.PatchOptions{}); err != nil {
		t.Fatalf("stamp sleep marker: %v", err)
	}

	// Give the operator time to reconcile and create the sentinel.
	time.Sleep(10 * time.Second)

	// Verify the sentinel Deployment was created.
	wakerName := gs + "-waker"
	var waker *appsv1.Deployment
	envInstance.Eventually(t, 30*time.Second, func() (bool, string) {
		var err error
		waker, err = envInstance.K8s.AppsV1().Deployments(ns).Get(ctx, wakerName, metav1.GetOptions{})
		if err != nil {
			return false, "get waker deployment: " + err.Error()
		}
		if waker.Status.ReadyReplicas == 0 {
			return false, "waker not yet ready"
		}
		return true, ""
	})

	// Verify the game Service selector points to the sentinel.
	svc, err := envInstance.K8s.CoreV1().Services(ns).Get(ctx, gs, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get game service: %v", err)
	}
	if svc.Spec.Selector["app.kubernetes.io/name"] != "gameplane-waker" {
		t.Fatalf("service selector does not point to sentinel; selector=%v", svc.Spec.Selector)
	}

	// Ping the sentinel via the service DNS.
	pingAddr := gs + "." + ns + ".svc.cluster.local:25565"
	st, err := minecraftproto.Ping(ctx, pingAddr)
	if err != nil {
		t.Fatalf("ping sentinel: %v", err)
	}
	if st == nil {
		t.Fatal("ping returned nil status")
	}
	t.Logf("ping succeeded: version=%s, protocol=%d", st.Version.Name, st.Version.Protocol)

	// Verify the server is STILL asleep: poll over several seconds to ensure the
	// sleep marker remains. Wake propagation is async (the sentinel patches a
	// request annotation; only the operator's next reconcile clears the sleep marker),
	// so we must allow time for a regression to manifest.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		gsObj, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Get(ctx, gs, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get gameserver: %v", err)
		}
		if ann := gsObj.GetAnnotations(); ann["gameplane.local/idle-asleep-since"] == "" {
			t.Fatal("server woke up after a ping; sleep annotation missing")
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Verify the game StatefulSet is still at 0 replicas.
	ss, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, gs, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get game statefulset: %v", err)
	}
	if ss.Spec.Replicas == nil || *ss.Spec.Replicas != 0 {
		t.Errorf("game statefulset replicas = %v, want 0", ss.Spec.Replicas)
	}
}

// TestGameServer_WakeOnConnect_LoginWakes verifies that a genuine login attempt
// DOES wake a sleeping, wake-on-connect-armed server. The sentinel should detect
// the login handshake, patch the wake annotation, and then the operator should
// bring the game pod back up.
func TestGameServer_WakeOnConnect_LoginWakes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"
	tmpl := "e2e-wake-on-connect-login"
	gs := "e2e-wake-on-connect-login-test"

	// Create a Minecraft-like template with wakeOnConnect capability.
	applyMinecraftTemplateWithWake(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)
	waitPVCBound(t, ns, gs+"-data", 90*time.Second)

	// Wait for the server to reach Running.
	envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
		obj, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Get(ctx, gs, metav1.GetOptions{})
		if err != nil {
			return false, "get gameserver: " + err.Error()
		}
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		if phase != "Running" {
			return false, "phase=" + phase
		}
		return true, ""
	})

	// Enable idle auto-sleep with wakeOnConnect armed.
	idlePatch := []byte(`{"spec":{"idle":{"enabled":true,"afterMinutes":5,"wakeOnConnect":true}}}`)
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Patch(ctx, gs, types.MergePatchType, idlePatch, metav1.PatchOptions{}); err != nil {
		t.Fatalf("patch idle+wakeOnConnect: %v", err)
	}

	// Drive the server to sleep by stamping the authoritative sleep marker.
	// (The annotation, not the status field, is what drives the reconciler's decision.)
	past := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	sleepPatch := []byte(`{"metadata":{"annotations":{"gameplane.local/idle-asleep-since":"` + past + `"}}}`)
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Patch(ctx, gs, types.MergePatchType, sleepPatch, metav1.PatchOptions{}); err != nil {
		t.Fatalf("stamp sleep marker: %v", err)
	}

	// Give the operator time to reconcile and create the sentinel.
	time.Sleep(10 * time.Second)

	// Verify the sentinel Deployment was created and is ready.
	wakerName := gs + "-waker"
	envInstance.Eventually(t, 30*time.Second, func() (bool, string) {
		waker, err := envInstance.K8s.AppsV1().Deployments(ns).Get(ctx, wakerName, metav1.GetOptions{})
		if err != nil {
			return false, "get waker deployment: " + err.Error()
		}
		if waker.Status.ReadyReplicas == 0 {
			return false, "waker not yet ready"
		}
		return true, ""
	})

	// Attempt a login via the sentinel's IP (the sentinel should reject it after
	// waking the server). We expect the sentinel to close the connection once it
	// has patched the wake annotation, and we'll get a protocol error, which is
	// acceptable — the goal is to verify the wake annotation was stamped.
	loginAddr := gs + "." + ns + ".svc.cluster.local:25565"
	st, res, err := minecraftproto.Connect(ctx, loginAddr, "gameplane-bot")
	// We may get an error due to the sentinel closing the connection after
	// patching the wake annotation. That's OK — the point is the annotation
	// should have been stamped.
	if st == nil && res == nil {
		t.Logf("login connection closed (expected if sentinel is waking): %v", err)
	} else if res != nil {
		t.Logf("login attempt: outcome=%v, detail=%s", res.Outcome, res.Detail)
	}

	// Verify the sentinel stamped the wake-request annotation so the wake is
	// pinned to the login-parsing path.
	envInstance.Eventually(t, 10*time.Second, func() (bool, string) {
		obj, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Get(ctx, gs, metav1.GetOptions{})
		if err != nil {
			return false, "get gameserver: " + err.Error()
		}
		if ann := obj.GetAnnotations(); ann["gameplane.local/idle-wake-requested"] == "" {
			return false, "wake-request annotation not stamped"
		}
		return true, ""
	})

	// Give the operator time to react to the wake annotation and bring the
	// server back up.
	envInstance.Eventually(t, 120*time.Second, func() (bool, string) {
		obj, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Get(ctx, gs, metav1.GetOptions{})
		if err != nil {
			return false, "get gameserver: " + err.Error()
		}
		// Check that the sleep annotation is gone.
		if ann := obj.GetAnnotations(); ann["gameplane.local/idle-asleep-since"] != "" {
			return false, "still asleep"
		}
		// Check that the server is back to Running.
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		if phase != "Running" {
			return false, "phase=" + phase
		}
		return true, ""
	})

	// Verify the game StatefulSet is back to 1 replica.
	ss, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, gs, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get game statefulset: %v", err)
	}
	if ss.Spec.Replicas == nil || *ss.Spec.Replicas != 1 {
		t.Errorf("game statefulset replicas = %v, want 1", ss.Spec.Replicas)
	}
}

// TestGameServer_WakeOnConnect_UnarmedNoSentinel verifies that a sleeping server
// with wakeOnConnect DISABLED does not get a sentinel Deployment or have its
// service routed to one. The sentinel infrastructure only exists when explicitly
// armed.
func TestGameServer_WakeOnConnect_UnarmedNoSentinel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"
	tmpl := "e2e-wake-on-connect-unarmed"
	gs := "e2e-wake-on-connect-unarmed-test"

	// Create a Minecraft-like template (it has wakeProtocol) but do NOT arm wakeOnConnect.
	applyMinecraftTemplateWithWake(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)
	waitPVCBound(t, ns, gs+"-data", 90*time.Second)

	// Wait for the server to reach Running.
	envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
		obj, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Get(ctx, gs, metav1.GetOptions{})
		if err != nil {
			return false, "get gameserver: " + err.Error()
		}
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		if phase != "Running" {
			return false, "phase=" + phase
		}
		return true, ""
	})

	// Enable idle auto-sleep WITHOUT wakeOnConnect.
	idlePatch := []byte(`{"spec":{"idle":{"enabled":true,"afterMinutes":5}}}`)
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Patch(ctx, gs, types.MergePatchType, idlePatch, metav1.PatchOptions{}); err != nil {
		t.Fatalf("patch idle without wakeOnConnect: %v", err)
	}

	// Drive the server to sleep by stamping the authoritative sleep marker.
	// (The annotation, not the status field, is what drives the reconciler's decision.)
	past := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	sleepPatch := []byte(`{"metadata":{"annotations":{"gameplane.local/idle-asleep-since":"` + past + `"}}}`)
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Patch(ctx, gs, types.MergePatchType, sleepPatch, metav1.PatchOptions{}); err != nil {
		t.Fatalf("stamp sleep marker: %v", err)
	}

	// Give the operator time to reconcile.
	time.Sleep(10 * time.Second)

	// Verify the sentinel Deployment was NOT created.
	wakerName := gs + "-waker"
	_, err := envInstance.K8s.AppsV1().Deployments(ns).Get(ctx, wakerName, metav1.GetOptions{})
	if err == nil {
		t.Fatal("sentinel waker deployment should not exist when wakeOnConnect is disabled")
	}

	// Verify the game Service selector does NOT point to the sentinel.
	svc, err := envInstance.K8s.CoreV1().Services(ns).Get(ctx, gs, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get game service: %v", err)
	}
	if svc.Spec.Selector["app.kubernetes.io/name"] == "gameplane-waker" {
		t.Fatal("service selector should not point to sentinel when wakeOnConnect is disabled")
	}
}

// applyMinecraftTemplateWithWake creates a cluster-scoped GameTemplate with
// Minecraft-like specs and wakeProtocol=minecraft on its advertised port.
// This template is set up to support wake-on-connect testing.
func applyMinecraftTemplateWithWake(t *testing.T, tmplName string) {
	t.Helper()
	ctx := context.Background()
	tmpl := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameTemplate",
		"metadata":   map[string]any{"name": tmplName},
		"spec": map[string]any{
			"displayName": "E2E Minecraft Wake-On-Connect (" + tmplName + ")",
			"game":        "minecraft-java",
			"version":     "1",
			"image":       "busybox:1.36",
			"command":     []any{"sh", "-c", "sleep 100000"},
			"ports": []any{
				map[string]any{
					"name":          "game",
					"containerPort": int64(25565),
					"advertise":     true,
					"protocol":      "TCP",
					"wakeProtocol":  "minecraft",
				},
			},
		},
	}}
	if _, err := envInstance.Dyn.Resource(gameTemplateGVR).
		Create(ctx, tmpl, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create template %s: %v", tmplName, err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameTemplateGVR).
			Delete(context.Background(), tmplName, metav1.DeleteOptions{})
	})
}
