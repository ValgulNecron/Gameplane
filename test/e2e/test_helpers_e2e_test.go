//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GVRs for the Gameplane CRDs the new tests touch. The existing
// gameserver_e2e_test.go owns gameTemplateGVR and gameServerGVR; we
// extend the set here.
var (
	backupGVR         = schema.GroupVersionResource{Group: "gameplane.gg", Version: "v1alpha1", Resource: "backups"}
	backupScheduleGVR = schema.GroupVersionResource{Group: "gameplane.gg", Version: "v1alpha1", Resource: "backupschedules"}
	restoreGVR        = schema.GroupVersionResource{Group: "gameplane.gg", Version: "v1alpha1", Resource: "restores"}
	moduleGVR         = schema.GroupVersionResource{Group: "gameplane.gg", Version: "v1alpha1", Resource: "modules"}
	moduleSourceGVR   = schema.GroupVersionResource{Group: "gameplane.gg", Version: "v1alpha1", Resource: "modulesources"}
)

// applyBusyboxTemplate creates (or reuses) a cluster-scoped GameTemplate
// named tmplName with a tiny busybox spec. Test registers a Cleanup to
// delete it.
//
// We don't wait for the template to be picked up by anything — it's
// just a static blueprint, and the GameServer reconciler reads it on
// demand.
func applyBusyboxTemplate(t *testing.T, tmplName string) {
	t.Helper()
	ctx := context.Background()
	tmpl := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.gg/v1alpha1",
		"kind":       "GameTemplate",
		"metadata":   map[string]any{"name": tmplName},
		"spec": map[string]any{
			"displayName": "E2E busybox (" + tmplName + ")",
			"game":        "busybox",
			"version":     "1",
			"image":       "busybox:1.36",
			"command":     []any{"sh", "-c", "sleep 100000"},
			"ports": []any{
				map[string]any{"name": "noop", "containerPort": int64(12345), "advertise": true, "protocol": "TCP"},
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

// applyBusyboxGameServer creates a namespaced GameServer pointing at
// tmplName. Caller is responsible for asserting whatever it cares
// about (PVC, StatefulSet, status). Cleanup is registered on t.
func applyBusyboxGameServer(t *testing.T, ns, gsName, tmplName string) {
	t.Helper()
	ctx := context.Background()
	gs := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.gg/v1alpha1",
		"kind":       "GameServer",
		"metadata":   map[string]any{"name": gsName, "namespace": ns},
		"spec": map[string]any{
			"templateRef": map[string]any{"name": tmplName},
		},
	}}
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Create(ctx, gs, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create gameserver %s/%s: %v", ns, gsName, err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Delete(context.Background(), gsName, metav1.DeleteOptions{})
	})
}

// waitPVCBound polls until the PVC reaches Bound (or timeout). The
// kind default storage class binds on first claim, so this typically
// resolves within ~10s of the StatefulSet pod being scheduled.
func waitPVCBound(t *testing.T, ns, pvcName string, timeout time.Duration) {
	t.Helper()
	ctx := context.Background()
	envInstance.Eventually(t, timeout, func() (bool, string) {
		pvc, err := envInstance.K8s.CoreV1().PersistentVolumeClaims(ns).
			Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			return false, "get pvc: " + err.Error()
		}
		if pvc.Status.Phase == "Bound" {
			return true, ""
		}
		return false, "pvc " + pvcName + " phase=" + string(pvc.Status.Phase)
	})
}

// createBackup applies a Backup CR and registers cleanup on t. The
// caller chooses the Backup name explicitly so a parent test can refer
// to it in subsequent assertions (e.g. matching a Restore.spec.backupRef).
//
// The repoSecretName/repoKey pair must already exist in ns — typically
// the e2e-restic-creds Secret installed by fixtures/backup-restic-secret.yaml.
func createBackup(t *testing.T, ns, name, gsName, repoSecretName, repoKey string) *unstructured.Unstructured {
	t.Helper()
	ctx := context.Background()
	bk := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.gg/v1alpha1",
		"kind":       "Backup",
		"metadata":   map[string]any{"name": name, "namespace": ns},
		"spec": map[string]any{
			"serverRef": map[string]any{"name": gsName},
			"repoRef":   map[string]any{"name": repoSecretName, "key": repoKey},
			"strategy":  "restic-snapshot",
			"quiesce":   false,
		},
	}}
	created, err := envInstance.Dyn.Resource(backupGVR).Namespace(ns).
		Create(ctx, bk, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create backup %s/%s: %v", ns, name, err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(backupGVR).Namespace(ns).
			Delete(context.Background(), name, metav1.DeleteOptions{})
	})
	return created
}

// createRestore applies a Restore CR and registers cleanup on t.
func createRestore(t *testing.T, ns, name, gsName, backupName string) *unstructured.Unstructured {
	t.Helper()
	ctx := context.Background()
	rs := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.gg/v1alpha1",
		"kind":       "Restore",
		"metadata":   map[string]any{"name": name, "namespace": ns},
		"spec": map[string]any{
			"backupRef": map[string]any{"name": backupName},
			"serverRef": map[string]any{"name": gsName},
		},
	}}
	created, err := envInstance.Dyn.Resource(restoreGVR).Namespace(ns).
		Create(ctx, rs, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create restore %s/%s: %v", ns, name, err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(restoreGVR).Namespace(ns).
			Delete(context.Background(), name, metav1.DeleteOptions{})
	})
	return created
}

// waitBackupSucceeded polls until the Backup reaches phase=Succeeded and
// returns the resolved snapshotID. Fails the test on timeout or on a
// terminal Failed phase.
func waitBackupSucceeded(t *testing.T, ns, name string, timeout time.Duration) string {
	t.Helper()
	ctx := context.Background()
	var snapshotID string
	envInstance.Eventually(t, timeout, func() (bool, string) {
		got, err := envInstance.Dyn.Resource(backupGVR).Namespace(ns).
			Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, "get backup: " + err.Error()
		}
		phase, _, _ := unstructured.NestedString(got.Object, "status", "phase")
		switch phase {
		case "Succeeded":
			snapshotID, _, _ = unstructured.NestedString(got.Object, "status", "snapshotID")
			if snapshotID == "" {
				return false, "phase=Succeeded but snapshotID empty"
			}
			return true, ""
		case "Failed":
			msg, _, _ := unstructured.NestedString(got.Object, "status", "message")
			t.Fatalf("backup %s/%s failed: %s", ns, name, msg)
			return false, ""
		default:
			return false, "phase=" + phase
		}
	})
	return snapshotID
}

// waitRestoreSucceeded polls until the Restore reaches phase=Succeeded.
// Fails the test on timeout or on a terminal Failed phase.
func waitRestoreSucceeded(t *testing.T, ns, name string, timeout time.Duration) {
	t.Helper()
	ctx := context.Background()
	envInstance.Eventually(t, timeout, func() (bool, string) {
		got, err := envInstance.Dyn.Resource(restoreGVR).Namespace(ns).
			Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, "get restore: " + err.Error()
		}
		phase, _, _ := unstructured.NestedString(got.Object, "status", "phase")
		switch phase {
		case "Succeeded":
			return true, ""
		case "Failed":
			msg, _, _ := unstructured.NestedString(got.Object, "status", "message")
			t.Fatalf("restore %s/%s failed: %s", ns, name, msg)
			return false, ""
		default:
			return false, "phase=" + phase
		}
	})
}

// waitStatefulSetReplicas polls until ss.Spec.Replicas equals want.
func waitStatefulSetReplicas(t *testing.T, ns, name string, want int32, timeout time.Duration) {
	t.Helper()
	ctx := context.Background()
	envInstance.Eventually(t, timeout, func() (bool, string) {
		ss, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, "get ss: " + err.Error()
		}
		if ss.Spec.Replicas != nil && *ss.Spec.Replicas == want {
			return true, ""
		}
		got := int32(-1)
		if ss.Spec.Replicas != nil {
			got = *ss.Spec.Replicas
		}
		return false, "want replicas=" + itoa(int(want)) + " got=" + itoa(int(got))
	})
}

// waitPodReady polls until the named pod has Ready=True. The agent
// sidecar takes a noticeable while on first run because the operator
// has to mint mTLS certs and mount them; allow the caller to set a
// generous timeout.
func waitPodReady(t *testing.T, ns, podName string, timeout time.Duration) {
	t.Helper()
	ctx := context.Background()
	envInstance.Eventually(t, timeout, func() (bool, string) {
		ok, err := envInstance.PodIsReady(ctx, ns, podName)
		if err != nil {
			return false, "pod " + podName + ": " + err.Error()
		}
		if ok {
			return true, ""
		}
		return false, "pod " + podName + " not ready"
	})
}

// getStatefulSetPod returns the canonical pod-0 for a StatefulSet.
func getStatefulSetPod(t *testing.T, ns, ssName string) *corev1.Pod {
	t.Helper()
	ctx := context.Background()
	pod, err := envInstance.K8s.CoreV1().Pods(ns).Get(ctx, ssName+"-0", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get pod %s-0 in %s: %v", ssName, ns, err)
	}
	return pod
}

// applyBusyboxPTYTemplate creates (or reuses) a cluster-scoped GameTemplate
// with consoleMode=pty and command=["sh"]. The operator surfaces this as
// tty=true + stdin=true on the game container, which is what /ws/servers/
// {name}/console-pty needs to attach over the K8s pod-attach API.
//
// Distinct from applyBusyboxTemplate so the existing tests keep their
// "sleep forever" command — a /bin/sh entrypoint exits immediately under
// non-PTY mode (no controlling tty), which would crashloop the pod.
func applyBusyboxPTYTemplate(t *testing.T, tmplName string) {
	t.Helper()
	ctx := context.Background()
	tmpl := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.gg/v1alpha1",
		"kind":       "GameTemplate",
		"metadata":   map[string]any{"name": tmplName},
		"spec": map[string]any{
			"displayName": "E2E busybox PTY (" + tmplName + ")",
			"game":        "busybox",
			"version":     "1",
			"image":       "busybox:1.36",
			"command":     []any{"sh"},
			"consoleMode": "pty",
			"ports": []any{
				map[string]any{"name": "noop", "containerPort": int64(12345), "advertise": true, "protocol": "TCP"},
			},
		},
	}}
	if _, err := envInstance.Dyn.Resource(gameTemplateGVR).
		Create(ctx, tmpl, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create pty template %s: %v", tmplName, err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameTemplateGVR).
			Delete(context.Background(), tmplName, metav1.DeleteOptions{})
	})
}

// applyBusyboxLogTickerTemplate creates a GameTemplate that appends the
// given marker line to /data/game.log every second and declares that
// file as spec.logPath, so the agent sidecar tails it. Used by the WS
// log-tail test — the marker is what the test grep'es for in the
// streamed frames.
func applyBusyboxLogTickerTemplate(t *testing.T, tmplName, marker string) {
	t.Helper()
	ctx := context.Background()
	tmpl := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.gg/v1alpha1",
		"kind":       "GameTemplate",
		"metadata":   map[string]any{"name": tmplName},
		"spec": map[string]any{
			"displayName": "E2E busybox log-ticker (" + tmplName + ")",
			"game":        "busybox",
			"version":     "1",
			"image":       "busybox:1.36",
			"command":     []any{"sh", "-c", "while true; do echo " + marker + " >> /data/game.log; sleep 1; done"},
			"logPath":     "/data/game.log",
			"ports": []any{
				map[string]any{"name": "noop", "containerPort": int64(12345), "advertise": true, "protocol": "TCP"},
			},
		},
	}}
	if _, err := envInstance.Dyn.Resource(gameTemplateGVR).
		Create(ctx, tmpl, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create log-ticker template %s: %v", tmplName, err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameTemplateGVR).
			Delete(context.Background(), tmplName, metav1.DeleteOptions{})
	})
}

// dialAuthedWS opens a WebSocket against the API client's port-forward,
// reusing its session cookies and CSRF token. Returns the connection
// and a stop func that closes it cleanly.
//
// The chosen path must already be registered on the API router (e.g.
// "/ws/servers/foo/console-pty"). Caller is responsible for the wire
// protocol on top.
func dialAuthedWS(t *testing.T, cli *APIClient, path string) (*websocket.Conn, func()) {
	t.Helper()
	wsURL := strings.Replace(cli.BaseURL, "http://", "ws://", 1) + path

	// websocket.Dial uses the supplied HTTPClient's Jar for cookies, so
	// borrowing the APIClient's jar gives us the gameplane_session cookie
	// without manual scraping.
	dialClient := &http.Client{Jar: cli.HTTP.Jar, Timeout: 0}
	header := http.Header{}
	header.Set("X-Gameplane-CSRF", cli.CSRF)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	wsConn, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPClient: dialClient,
		HTTPHeader: header,
	})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial ws %s: %v", path, err)
	}
	stop := func() {
		_ = wsConn.Close(websocket.StatusNormalClosure, "test done")
	}
	return wsConn, stop
}

// waitBackupCount polls until exactly want Backups owned by the named
// BackupSchedule are present in the namespace, or fails the test on
// timeout. Used by retention-trim assertions where the count itself is
// the contract under test.
func waitBackupCount(t *testing.T, ns, schedName string, want int, timeout time.Duration) {
	t.Helper()
	ctx := context.Background()
	envInstance.Eventually(t, timeout, func() (bool, string) {
		bks, err := envInstance.Dyn.Resource(backupGVR).Namespace(ns).
			List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, "list backups: " + err.Error()
		}
		// Count only Succeeded backups: retention's contract is about
		// completed snapshots — with a one-minute cron there is almost
		// always an in-flight Backup alongside the kept one, and the
		// trimmer never touches in-flight backups by design.
		got := 0
		for _, item := range bks.Items {
			phase, _, _ := unstructured.NestedString(item.Object, "status", "phase")
			if phase != "Succeeded" {
				continue
			}
			for _, owner := range item.GetOwnerReferences() {
				if owner.Kind == "BackupSchedule" && owner.Name == schedName {
					got++
					break
				}
			}
		}
		if got == want {
			return true, ""
		}
		return false, "succeeded owned Backups got=" + itoa(got) + " want=" + itoa(want)
	})
}

// itoa is a tiny strconv.Itoa shim — saves an import in the helper file
// where it's the only usage.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	n := len(buf)
	for i > 0 {
		n--
		buf[n] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		n--
		buf[n] = '-'
	}
	return string(buf[n:])
}
