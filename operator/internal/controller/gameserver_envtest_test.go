//go:build envtest

package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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

// TestGameServer_AgentServiceAlwaysClusterIP — the dedicated `<gs>-agent`
// Service exists with port 8090 and stays ClusterIP even when the game's
// own Service is externally exposed via spec.networking.expose.
func TestGameServer_AgentServiceAlwaysClusterIP(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("factorio"))
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "smp", tmpl.Name)
	gs.Spec.Networking.Expose = "NodePort"
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var svc corev1.Service
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp-agent"}, &svc); err != nil {
			return false, "agent service: " + err.Error()
		}
		if svc.Spec.Type != corev1.ServiceTypeClusterIP {
			return false, "agent service type = " + string(svc.Spec.Type)
		}
		if len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0].Port != 8090 {
			return false, "agent service ports unexpected"
		}
		if !svc.Spec.PublishNotReadyAddresses {
			return false, "agent service must publish not-ready addresses"
		}
		if svc.Spec.Selector["app.kubernetes.io/instance"] != "smp" {
			return false, "agent service selector missing instance label"
		}
		var game corev1.Service
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &game); err != nil {
			return false, "game service: " + err.Error()
		}
		if game.Spec.Type != corev1.ServiceTypeNodePort {
			return false, "game service should be NodePort, got " + string(game.Spec.Type)
		}
		return true, ""
	})
}

// TestGameServer_AgentHeartbeatRBAC — the per-GameServer SA, Role, and
// RoleBinding exist, the Role is resourceNames-scoped to this server's
// status subresource, and the pod template runs as the SA (unless
// spec.serviceAccountName overrides it).
func TestGameServer_AgentHeartbeatRBAC(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("terraria"))
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	if err := k8sClient.Create(context.Background(), buildGameServer(ns, "smp", tmpl.Name)); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var sa corev1.ServiceAccount
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp-agent"}, &sa); err != nil {
			return false, "serviceaccount: " + err.Error()
		}
		var role rbacv1.Role
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp-agent-heartbeat"}, &role); err != nil {
			return false, "role: " + err.Error()
		}
		if len(role.Rules) != 1 {
			return false, "expected 1 rule"
		}
		rule := role.Rules[0]
		if len(rule.ResourceNames) != 1 || rule.ResourceNames[0] != "smp" {
			return false, "role not resourceNames-scoped to the GameServer"
		}
		if len(rule.Resources) != 1 || rule.Resources[0] != "gameservers/status" {
			return false, "role grants more than gameservers/status"
		}
		var rb rbacv1.RoleBinding
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp-agent-heartbeat"}, &rb); err != nil {
			return false, "rolebinding: " + err.Error()
		}
		if len(rb.Subjects) != 1 || rb.Subjects[0].Name != "smp-agent" {
			return false, "rolebinding subject is not the agent SA"
		}
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &ss); err != nil {
			return false, "statefulset: " + err.Error()
		}
		if got := ss.Spec.Template.Spec.ServiceAccountName; got != "smp-agent" {
			return false, "pod SA = " + got
		}
		return true, ""
	})
}

// TestGameServer_ServiceAccountOverrideWins — spec.serviceAccountName
// replaces the operator-managed default on the pod template.
func TestGameServer_ServiceAccountOverrideWins(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("astroneer"))
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "smp", tmpl.Name)
	gs.Spec.ServiceAccountName = "my-own-sa"
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &ss); err != nil {
			return false, "statefulset: " + err.Error()
		}
		if got := ss.Spec.Template.Spec.ServiceAccountName; got != "my-own-sa" {
			return false, "pod SA = " + got
		}
		return true, ""
	})
}

// TestGameServer_ConfigMaterializesAsEnv — spec.config resolved against
// the template's configSchema lands on the game container with the
// documented precedence: template env < config < spec.env.
func TestGameServer_ConfigMaterializesAsEnv(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("minecraft"))
	tmpl.Spec.Env = []corev1.EnvVar{
		{Name: "EULA", Value: "TRUE"},
		{Name: "TYPE", Value: "VANILLA"}, // config below must override this
	}
	tmpl.Spec.ConfigSchema = []kestrelv1alpha1.ConfigField{
		{Name: "TYPE", Type: "enum", Enum: []string{"VANILLA", "PAPER"}, Default: "VANILLA", Required: true},
		{Name: "VERSION", Type: "string", Default: "LATEST"},
		{Name: "MAX_PLAYERS", Type: "int", Default: "20"},
	}
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "smp", tmpl.Name)
	gs.Spec.Config = map[string]string{"TYPE": "PAPER"}
	gs.Spec.Env = []corev1.EnvVar{{Name: "MAX_PLAYERS", Value: "64"}} // explicit env beats config default
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &ss); err != nil {
			return false, "statefulset: " + err.Error()
		}
		var game *corev1.Container
		for i := range ss.Spec.Template.Spec.Containers {
			if ss.Spec.Template.Spec.Containers[i].Name == "game" {
				game = &ss.Spec.Template.Spec.Containers[i]
			}
		}
		if game == nil {
			return false, "no game container"
		}
		// Later duplicates win in the kubelet; assert on the last
		// occurrence of each name.
		got := map[string]string{}
		for _, e := range game.Env {
			got[e.Name] = e.Value
		}
		want := map[string]string{
			"EULA":        "TRUE",   // template env untouched
			"TYPE":        "PAPER",  // config overrides template env
			"VERSION":     "LATEST", // schema default applied
			"MAX_PLAYERS": "64",     // spec.env overrides config
		}
		for k, v := range want {
			if got[k] != v {
				return false, "env " + k + " = " + got[k] + ", want " + v
			}
		}
		if ss.Spec.Template.Annotations["kestrel.gg/config-hash"] == "" {
			return false, "config-hash annotation missing"
		}
		return true, ""
	})
}

// TestGameServer_InvalidConfigFailsThenRecovers — a config violating the
// schema flips phase to Failed with a pointed message and creates no
// StatefulSet; fixing spec.config materializes the server.
func TestGameServer_InvalidConfigFailsThenRecovers(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("minecraft"))
	tmpl.Spec.ConfigSchema = []kestrelv1alpha1.ConfigField{
		{Name: "MODE", Type: "enum", Enum: []string{"survival", "creative"}},
	}
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "smp", tmpl.Name)
	gs.Spec.Config = map[string]string{"MODE": "hardcore"}
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		got := getGameServer(t, ns, "smp")
		if got.Status.Phase != kestrelv1alpha1.GameServerPhaseFailed {
			return false, "phase = " + string(got.Status.Phase)
		}
		for _, c := range got.Status.Conditions {
			if c.Type == "Ready" && strings.Contains(c.Message, "MODE") {
				return true, ""
			}
		}
		return false, "Ready condition does not mention the offending field"
	})

	var ss appsv1.StatefulSet
	if err := k8sClient.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: "smp"}, &ss); !apierrors.IsNotFound(err) {
		t.Fatalf("StatefulSet should not exist while config is invalid, get err = %v", err)
	}

	gs = getGameServer(t, ns, "smp")
	gs.Spec.Config["MODE"] = "creative"
	if err := k8sClient.Update(context.Background(), gs); err != nil {
		t.Fatalf("update gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &ss); err != nil {
			return false, "statefulset: " + err.Error()
		}
		got := getGameServer(t, ns, "smp")
		if got.Status.Phase == kestrelv1alpha1.GameServerPhaseFailed {
			return false, "phase still Failed after fixing config"
		}
		return true, ""
	})
}

// TestGameServer_PasswordConfigStoredInSecret — password-type config
// lands in the owned `<gs>-config` Secret and the pod spec carries only
// a SecretKeyRef; dropping the password deletes the Secret again.
func TestGameServer_PasswordConfigStoredInSecret(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("valheim"))
	tmpl.Spec.ConfigSchema = []kestrelv1alpha1.ConfigField{
		{Name: "SERVER_PASS", Type: "password"},
	}
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "smp", tmpl.Name)
	gs.Spec.Config = map[string]string{"SERVER_PASS": "hunter22"}
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var sec corev1.Secret
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp-config"}, &sec); err != nil {
			return false, "config secret: " + err.Error()
		}
		if string(sec.Data["SERVER_PASS"]) != "hunter22" {
			return false, "secret does not hold the password"
		}
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &ss); err != nil {
			return false, "statefulset: " + err.Error()
		}
		for _, c := range ss.Spec.Template.Spec.Containers {
			for _, e := range c.Env {
				if e.Value == "hunter22" {
					return false, "password appears inline in pod spec env " + e.Name
				}
				if c.Name == "game" && e.Name == "SERVER_PASS" {
					if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil ||
						e.ValueFrom.SecretKeyRef.Name != "smp-config" {
						return false, "SERVER_PASS env is not a SecretKeyRef into smp-config"
					}
				}
			}
		}
		return true, ""
	})

	// Dropping the password must clean the Secret up.
	gs = getGameServer(t, ns, "smp")
	gs.Spec.Config = nil
	if err := k8sClient.Update(context.Background(), gs); err != nil {
		t.Fatalf("update gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var sec corev1.Secret
		err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp-config"}, &sec)
		if err == nil {
			return false, "config secret still exists"
		}
		if !apierrors.IsNotFound(err) {
			return false, err.Error()
		}
		return true, ""
	})
}

// TestGameServer_ConfigChangeRollsPodTemplate — editing a config value
// must change the pod template (env + hash annotation) so the
// StatefulSet rolls the pod.
func TestGameServer_ConfigChangeRollsPodTemplate(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("minecraft"))
	tmpl.Spec.ConfigSchema = []kestrelv1alpha1.ConfigField{
		{Name: "DIFFICULTY", Type: "string", Default: "easy"},
	}
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "smp", tmpl.Name)
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	firstHash := ""
	eventually(t, func() (bool, string) {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &ss); err != nil {
			return false, err.Error()
		}
		firstHash = ss.Spec.Template.Annotations["kestrel.gg/config-hash"]
		return firstHash != "", "config-hash annotation missing"
	})

	gs = getGameServer(t, ns, "smp")
	gs.Spec.Config = map[string]string{"DIFFICULTY": "hard"}
	if err := k8sClient.Update(context.Background(), gs); err != nil {
		t.Fatalf("update gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &ss); err != nil {
			return false, err.Error()
		}
		hash := ss.Spec.Template.Annotations["kestrel.gg/config-hash"]
		if hash == firstHash {
			return false, "config-hash unchanged"
		}
		var game *corev1.Container
		for i := range ss.Spec.Template.Spec.Containers {
			if ss.Spec.Template.Spec.Containers[i].Name == "game" {
				game = &ss.Spec.Template.Spec.Containers[i]
			}
		}
		got := map[string]string{}
		for _, e := range game.Env {
			got[e.Name] = e.Value
		}
		if got["DIFFICULTY"] != "hard" {
			return false, "DIFFICULTY = " + got["DIFFICULTY"]
		}
		return true, ""
	})
}
