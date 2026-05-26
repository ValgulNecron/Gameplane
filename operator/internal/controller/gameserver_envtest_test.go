//go:build envtest

package controller

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
)

// TestGameServer_CreatesStatefulSetServicePVC — minimal happy path.
// One GameServer + GameTemplate ⇒ StatefulSet, Service, PVC all exist
// in the test namespace.
func TestGameServer_CreatesStatefulSetServicePVC(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("minecraft"))
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "smp", tmpl.Name)
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &ss); err != nil {
			return false, "statefulset: " + err.Error()
		}
		var svc corev1.Service
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &svc); err != nil {
			return false, "service: " + err.Error()
		}
		var pvc corev1.PersistentVolumeClaim
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp-data"}, &pvc); err != nil {
			return false, "pvc: " + err.Error()
		}
		return true, ""
	})
}

// TestGameServer_AgentSidecarInjected — pod template has 2 containers,
// agent has the documented identity env vars.
func TestGameServer_AgentSidecarInjected(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("valheim"))
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	if err := k8sClient.Create(context.Background(), buildGameServer(ns, "smp", tmpl.Name)); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &ss); err != nil {
			return false, err.Error()
		}
		cs := ss.Spec.Template.Spec.Containers
		if len(cs) != 2 {
			return false, "expected 2 containers, got " + sprintArgs(containerNames(cs))
		}
		var agent *corev1.Container
		for i := range cs {
			if cs[i].Name == "agent" {
				agent = &cs[i]
				break
			}
		}
		if agent == nil {
			return false, "no agent sidecar container"
		}
		want := map[string]string{
			"KESTREL_SERVER_NAME": "smp",
			"KESTREL_TEMPLATE":    tmpl.Name,
			"KESTREL_GAME":        tmpl.Spec.Game,
		}
		got := map[string]string{}
		for _, e := range agent.Env {
			got[e.Name] = e.Value
		}
		for k, v := range want {
			if got[k] != v {
				return false, "env " + k + " = " + got[k] + ", want " + v
			}
		}
		// Sidecar should drop ALL caps and run as non-root.
		if agent.SecurityContext == nil {
			return false, "agent has no SecurityContext"
		}
		if agent.SecurityContext.RunAsNonRoot == nil || !*agent.SecurityContext.RunAsNonRoot {
			return false, "agent RunAsNonRoot != true"
		}
		if agent.SecurityContext.Capabilities == nil ||
			len(agent.SecurityContext.Capabilities.Drop) == 0 ||
			agent.SecurityContext.Capabilities.Drop[0] != "ALL" {
			return false, "agent does not drop ALL caps"
		}
		return true, ""
	})
}

// TestGameServer_TemplateNotFound_PhaseFailed — referencing a missing
// template flips Status.Phase to Failed with a reasonable message.
func TestGameServer_TemplateNotFound_PhaseFailed(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	if err := k8sClient.Create(context.Background(), buildGameServer(ns, "orphan", "does-not-exist")); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		gs := getGameServer(t, ns, "orphan")
		if gs.Status.Phase != kestrelv1alpha1.GameServerPhaseFailed {
			return false, "phase = " + string(gs.Status.Phase)
		}
		return true, ""
	})
}

// TestGameServer_SuspendScalesToZero — Spec.Suspend=true ⇒ underlying
// StatefulSet replicas=0.
func TestGameServer_SuspendScalesToZero(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("minecraft"))
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "smp", tmpl.Name)
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	// Wait for initial create to land replicas=1.
	eventually(t, func() (bool, string) {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &ss); err != nil {
			return false, err.Error()
		}
		return ss.Spec.Replicas != nil && *ss.Spec.Replicas == 1, ""
	})

	// Flip Suspend=true.
	gs = getGameServer(t, ns, "smp")
	gs.Spec.Suspend = true
	if err := k8sClient.Update(context.Background(), gs); err != nil {
		t.Fatalf("update gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &ss); err != nil {
			return false, err.Error()
		}
		if ss.Spec.Replicas == nil || *ss.Spec.Replicas != 0 {
			r := int32(-1)
			if ss.Spec.Replicas != nil {
				r = *ss.Spec.Replicas
			}
			return false, "replicas not yet zero (got " + sprintArgs([]string{intToStr(r)}) + ")"
		}
		return true, ""
	})
}

// TestGameServer_BackupPolicyMaterializesSchedule — adding an inline
// BackupPolicy to a GameServer materializes a managed BackupSchedule.
func TestGameServer_BackupPolicyMaterializesSchedule(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("minecraft"))
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "smp", tmpl.Name)
	gs.Spec.BackupPolicy = &kestrelv1alpha1.InlineBackupPolicy{
		Schedule: "0 */6 * * *",
		RepoRef:  kestrelv1alpha1.SecretKeySelector{Name: "repo", Key: "url"},
	}
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var bs kestrelv1alpha1.BackupSchedule
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp-auto"}, &bs); err != nil {
			return false, "get schedule: " + err.Error()
		}
		if bs.Spec.Schedule != "0 */6 * * *" {
			return false, "schedule = " + bs.Spec.Schedule
		}
		ok := false
		for _, ref := range bs.OwnerReferences {
			if ref.Kind == "GameServer" && ref.Name == "smp" && ref.Controller != nil && *ref.Controller {
				ok = true
				break
			}
		}
		if !ok {
			return false, "schedule not owned by GameServer"
		}
		return true, ""
	})
}

// TestGameServer_BackupPolicyRemovedDeletesSchedule — clearing
// Spec.BackupPolicy deletes the managed schedule.
func TestGameServer_BackupPolicyRemovedDeletesSchedule(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("minecraft"))
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "smp", tmpl.Name)
	gs.Spec.BackupPolicy = &kestrelv1alpha1.InlineBackupPolicy{
		Schedule: "0 0 * * *",
		RepoRef:  kestrelv1alpha1.SecretKeySelector{Name: "repo", Key: "url"},
	}
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var bs kestrelv1alpha1.BackupSchedule
		err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp-auto"}, &bs)
		return err == nil, "waiting for managed schedule"
	})

	// Now drop the policy.
	gs = getGameServer(t, ns, "smp")
	gs.Spec.BackupPolicy = nil
	if err := k8sClient.Update(context.Background(), gs); err != nil {
		t.Fatalf("update gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var bs kestrelv1alpha1.BackupSchedule
		err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp-auto"}, &bs)
		if apierrors.IsNotFound(err) {
			return true, ""
		}
		if err != nil {
			return false, err.Error()
		}
		return false, "schedule still exists"
	})
}

// TestGameServer_StorageOverride — Spec.Storage.Size overrides the
// template default and lands on the PVC.
func TestGameServer_StorageOverride(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("minecraft"))
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "smp", tmpl.Name)
	want := resource.MustParse("25Gi")
	gs.Spec.Storage = &kestrelv1alpha1.GameStorageSpec{Size: want}
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var pvc corev1.PersistentVolumeClaim
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp-data"}, &pvc); err != nil {
			return false, err.Error()
		}
		got := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		if got.Cmp(want) != 0 {
			return false, "PVC size = " + got.String() + ", want " + want.String()
		}
		return true, ""
	})

	// Sanity: PVC is immutable on size, so a re-reconcile shouldn't fail.
	consistently(t, 1*time.Second, func() (bool, string) {
		var pvc corev1.PersistentVolumeClaim
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp-data"}, &pvc); err != nil {
			return false, err.Error()
		}
		return true, ""
	})
}

// TestGameServer_ConsoleMode_PTY — when the GameTemplate selects PTY
// console, the rendered StatefulSet's "game" container must have
// tty=true and stdin=true. These fields are immutable post-create, so
// getting them right at first reconcile is critical.
func TestGameServer_ConsoleMode_PTY(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("ptygame"))
	tmpl.Spec.ConsoleMode = "pty"
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	if err := k8sClient.Create(context.Background(), buildGameServer(ns, "ptysrv", tmpl.Name)); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "ptysrv"}, &ss); err != nil {
			return false, err.Error()
		}
		var game *corev1.Container
		for i := range ss.Spec.Template.Spec.Containers {
			if ss.Spec.Template.Spec.Containers[i].Name == "game" {
				game = &ss.Spec.Template.Spec.Containers[i]
				break
			}
		}
		if game == nil {
			return false, "no game container yet"
		}
		if !game.TTY {
			return false, "game.TTY = false, want true"
		}
		if !game.Stdin {
			return false, "game.Stdin = false, want true"
		}
		return true, ""
	})
}

// TestGameServer_ConsoleMode_RCONNoTTY — the default (RCON) console
// must NOT set tty/stdin on the game container. Setting them
// gratuitously would still work, but it changes how the container
// runtime hooks up stdio, and is a behavior the dashboard would not
// expect for the rcon path.
func TestGameServer_ConsoleMode_RCONNoTTY(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("rcongame"))
	tmpl.Spec.RCON = &kestrelv1alpha1.RCONSpec{Protocol: "source", Port: 25575}
	// ConsoleMode left empty — defaults to "rcon" via EffectiveConsoleMode.
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	if err := k8sClient.Create(context.Background(), buildGameServer(ns, "rconsrv", tmpl.Name)); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "rconsrv"}, &ss); err != nil {
			return false, err.Error()
		}
		var game *corev1.Container
		for i := range ss.Spec.Template.Spec.Containers {
			if ss.Spec.Template.Spec.Containers[i].Name == "game" {
				game = &ss.Spec.Template.Spec.Containers[i]
				break
			}
		}
		if game == nil {
			return false, "no game container yet"
		}
		if game.TTY {
			return false, "game.TTY = true, want false"
		}
		if game.Stdin {
			return false, "game.Stdin = true, want false"
		}
		return true, ""
	})
}

func containerNames(cs []corev1.Container) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Name)
	}
	return out
}

func intToStr(n int32) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	buf := [12]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = digits[n%10]
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
