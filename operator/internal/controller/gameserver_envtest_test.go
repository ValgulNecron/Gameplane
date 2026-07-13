//go:build envtest

package controller

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
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
			"GAMEPLANE_SERVER_NAME": "smp",
			"GAMEPLANE_TEMPLATE":    tmpl.Name,
			"GAMEPLANE_GAME":        tmpl.Spec.Game,
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

// TestGameServer_SecurityContextAppliedToGameContainerAndPod proves the
// full reconcile wiring for GameTemplate.spec.security (added for games
// like ARK: Survival Ascended, whose image requires uid 25000 and can't
// initialise Proton as root): runAsUser/runAsGroup land on the GAME
// container's SecurityContext, fsGroup lands on the pod's SecurityContext
// (it's a pod-level-only field that governs volume ownership), and the
// agent sidecar's own fixed SecurityContext (distroless, uid 65532) is
// untouched by any of it.
func TestGameServer_SecurityContextAppliedToGameContainerAndPod(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	uid := int64(25000)
	gid := int64(25000)
	tmpl := buildGameTemplate(uniqueName("ark-survival-ascended"))
	tmpl.Spec.Security = &gameplanev1alpha1.GameSecuritySpec{
		RunAsUser:  &uid,
		RunAsGroup: &gid,
		FSGroup:    &gid,
	}
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	if err := k8sClient.Create(context.Background(), buildGameServer(ns, "ark", tmpl.Name)); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "ark"}, &ss); err != nil {
			return false, err.Error()
		}
		cs := ss.Spec.Template.Spec.Containers
		var game, agent *corev1.Container
		for i := range cs {
			switch cs[i].Name {
			case gameContainerName:
				game = &cs[i]
			case "agent":
				agent = &cs[i]
			}
		}
		if game == nil {
			return false, "no game container yet"
		}
		if agent == nil {
			return false, "no agent sidecar container"
		}

		// Game container: runAsUser/runAsGroup from spec.security.
		gsc := game.SecurityContext
		if gsc == nil {
			return false, "game container has no SecurityContext"
		}
		if gsc.RunAsUser == nil || *gsc.RunAsUser != uid {
			return false, "game RunAsUser = " + fmt.Sprintf("%v", gsc.RunAsUser) + ", want " + intToStr(int32(uid))
		}
		if gsc.RunAsGroup == nil || *gsc.RunAsGroup != gid {
			return false, "game RunAsGroup = " + fmt.Sprintf("%v", gsc.RunAsGroup) + ", want " + intToStr(int32(gid))
		}

		// Pod level: fsGroup, not on the game container.
		psc := ss.Spec.Template.Spec.SecurityContext
		if psc == nil || psc.FSGroup == nil || *psc.FSGroup != gid {
			return false, "pod SecurityContext.FSGroup = " + fmt.Sprintf("%+v", psc) + ", want " + intToStr(int32(gid))
		}

		// Agent sidecar: untouched — still its own fixed distroless identity,
		// never the game's uid/gid.
		asc := agent.SecurityContext
		if asc == nil {
			return false, "agent has no SecurityContext"
		}
		if asc.RunAsUser == nil || *asc.RunAsUser == uid {
			return false, "agent RunAsUser leaked the game's uid: " + fmt.Sprintf("%v", asc.RunAsUser)
		}
		if asc.RunAsNonRoot == nil || !*asc.RunAsNonRoot {
			return false, "agent RunAsNonRoot != true"
		}
		if asc.ReadOnlyRootFilesystem == nil || !*asc.ReadOnlyRootFilesystem {
			return false, "agent ReadOnlyRootFilesystem != true"
		}
		if asc.Capabilities == nil || len(asc.Capabilities.Drop) == 0 || asc.Capabilities.Drop[0] != "ALL" {
			return false, "agent does not drop ALL caps"
		}
		return true, ""
	})
}

// TestGameServer_SecurityContextUnsetRendersNoSecurityContext is the
// regression guard: a template with no spec.security must render neither
// a game-container SecurityContext nor a pod-level SecurityContext — an
// empty `securityContext: {}` would still change the pod spec (and roll
// every existing game StatefulSet) even though nothing was requested.
func TestGameServer_SecurityContextUnsetRendersNoSecurityContext(t *testing.T) {
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
		if ss.Spec.Template.Spec.SecurityContext != nil {
			return false, "pod SecurityContext = " + fmt.Sprintf("%+v", ss.Spec.Template.Spec.SecurityContext) + ", want nil"
		}
		for _, c := range ss.Spec.Template.Spec.Containers {
			if c.Name == gameContainerName && c.SecurityContext != nil {
				return false, "game container SecurityContext = " + fmt.Sprintf("%+v", c.SecurityContext) + ", want nil"
			}
		}
		return true, ""
	})
}

// TestGameServer_StatusPatchPreservesAgentHeartbeat — the reconciler must
// not clobber status.agent (written by the sidecar's heartbeat) when it
// updates phase/conditions. We reproduce the lost-update race
// deterministically: seed status.agent in the cluster, then drive
// reconcileStatus with a stale in-memory copy whose Agent is nil (as if
// the reconciler had read the object before the heartbeat landed). The
// MergeFrom status patch touches only changed fields, so the seeded
// heartbeat survives; a full Status().Update would have reverted it.
func TestGameServer_StatusPatchPreservesAgentHeartbeat(t *testing.T) {
	ns := newNamespace(t)
	ctx := context.Background()

	tmpl := buildGameTemplate(uniqueName("minecraft"))
	if err := k8sClient.Create(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "smp", tmpl.Name)
	if err := k8sClient.Create(ctx, gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	// Seed status.agent as the in-pod sidecar's heartbeat would.
	var seeded gameplanev1alpha1.GameServer
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "smp"}, &seeded); err != nil {
		t.Fatalf("get for seed: %v", err)
	}
	players := int32(7)
	now := metav1.Now()
	seeded.Status.Agent = &gameplanev1alpha1.AgentStatus{LastHeartbeat: &now, PlayersOnline: &players}
	if err := k8sClient.Status().Update(ctx, &seeded); err != nil {
		t.Fatalf("seed agent status: %v", err)
	}

	// Drive reconcileStatus with a STALE copy whose Agent is nil — the
	// reconciler's pre-heartbeat view. No manager runs, so this is the
	// only writer and the test is deterministic.
	stale := seeded.DeepCopy()
	stale.Status.Agent = nil
	r := &GameServerReconciler{Client: k8sClient, Scheme: scheme}
	if _, err := r.reconcileStatus(ctx, stale); err != nil {
		t.Fatalf("reconcileStatus: %v", err)
	}

	var got gameplanev1alpha1.GameServer
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "smp"}, &got); err != nil {
		t.Fatalf("get after reconcile: %v", err)
	}
	if got.Status.Agent == nil || got.Status.Agent.PlayersOnline == nil {
		t.Fatal("reconciler clobbered status.agent — heartbeat lost")
	}
	if *got.Status.Agent.PlayersOnline != 7 {
		t.Fatalf("playersOnline = %d, want 7 (heartbeat clobbered)", *got.Status.Agent.PlayersOnline)
	}
	// And it still did its own job: phase was derived and written.
	if got.Status.Phase == "" {
		t.Fatal("reconciler did not set phase")
	}
}

// TestGameServer_RCONProvisioning — a template that exposes RCON gets a
// generated <gs>-rcon Secret, the password injected into the game
// container via the declared env var, and the same value mounted for the
// agent sidecar with --rcon-password-file.
func TestGameServer_RCONProvisioning(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("mc"))
	tmpl.Spec.RCON = &gameplanev1alpha1.RCONSpec{Protocol: "source", Port: 25575, PasswordEnv: "RCON_PASSWORD"}
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	if err := k8sClient.Create(context.Background(), buildGameServer(ns, "smp", tmpl.Name)); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var sec corev1.Secret
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp-rcon"}, &sec); err != nil {
			return false, "rcon secret: " + err.Error()
		}
		if len(sec.Data["password"]) == 0 {
			return false, "rcon secret has no password"
		}

		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &ss); err != nil {
			return false, "statefulset: " + err.Error()
		}
		var game, agent *corev1.Container
		for i := range ss.Spec.Template.Spec.Containers {
			c := &ss.Spec.Template.Spec.Containers[i]
			switch c.Name {
			case "game":
				game = c
			case "agent":
				agent = c
			}
		}
		if game == nil || agent == nil {
			return false, "missing containers"
		}
		// Game container: RCON_PASSWORD from the rcon Secret.
		var ok bool
		for _, e := range game.Env {
			if e.Name == "RCON_PASSWORD" && e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil &&
				e.ValueFrom.SecretKeyRef.Name == "smp-rcon" && e.ValueFrom.SecretKeyRef.Key == "password" {
				ok = true
			}
		}
		if !ok {
			return false, "game container missing RCON_PASSWORD secret env"
		}
		// Agent: --rcon-password-file arg + rcon-password mount.
		hasFlag := false
		for _, a := range agent.Args {
			if a == "--rcon-password-file=/etc/gameplane/rcon/password" {
				hasFlag = true
			}
		}
		if !hasFlag {
			return false, "agent missing --rcon-password-file"
		}
		mounted := false
		for _, m := range agent.VolumeMounts {
			if m.Name == "rcon-password" {
				mounted = true
			}
		}
		if !mounted {
			return false, "agent missing rcon-password mount"
		}
		return true, ""
	})
}

// TestGameServer_RCONPort — agent container receives --rcon-port argument
// when RCON is enabled. Defaults to 25575 when spec.rcon.port is unset,
// uses the declared port when set (e.g., 27015 for factorio).
func TestGameServer_RCONPort(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	// Test 1: custom RCON port (e.g., factorio at 27015)
	tmpl := buildGameTemplate(uniqueName("factorio"))
	tmpl.Spec.RCON = &gameplanev1alpha1.RCONSpec{Protocol: "source", Port: 27015, PasswordFile: "config/rconpw"}
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create factorio template: %v", err)
	}
	deleteCleanup(t, tmpl)

	if err := k8sClient.Create(context.Background(), buildGameServer(ns, "factorio-srv", tmpl.Name)); err != nil {
		t.Fatalf("create factorio gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "factorio-srv"}, &ss); err != nil {
			return false, "statefulset: " + err.Error()
		}
		var agent *corev1.Container
		for i := range ss.Spec.Template.Spec.Containers {
			if ss.Spec.Template.Spec.Containers[i].Name == "agent" {
				agent = &ss.Spec.Template.Spec.Containers[i]
				break
			}
		}
		if agent == nil {
			return false, "no agent container"
		}
		// Check for --rcon-port=27015
		hasPort := false
		for _, a := range agent.Args {
			if a == "--rcon-port=27015" {
				hasPort = true
				break
			}
		}
		if !hasPort {
			return false, "agent missing --rcon-port=27015, got: " + strings.Join(agent.Args, " ")
		}
		return true, ""
	})

	// Test 2: default RCON port (25575) when spec.rcon.port is unset
	tmpl2 := buildGameTemplate(uniqueName("minecraft"))
	tmpl2.Spec.RCON = &gameplanev1alpha1.RCONSpec{Protocol: "source", PasswordEnv: "RCON_PASSWORD"}
	// Note: Port is 0/unset, should default to 25575
	if err := k8sClient.Create(context.Background(), tmpl2); err != nil {
		t.Fatalf("create minecraft template: %v", err)
	}
	deleteCleanup(t, tmpl2)

	if err := k8sClient.Create(context.Background(), buildGameServer(ns, "mc-srv", tmpl2.Name)); err != nil {
		t.Fatalf("create minecraft gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "mc-srv"}, &ss); err != nil {
			return false, "statefulset: " + err.Error()
		}
		var agent *corev1.Container
		for i := range ss.Spec.Template.Spec.Containers {
			if ss.Spec.Template.Spec.Containers[i].Name == "agent" {
				agent = &ss.Spec.Template.Spec.Containers[i]
				break
			}
		}
		if agent == nil {
			return false, "no agent container"
		}
		// Check for --rcon-port=25575
		hasPort := false
		for _, a := range agent.Args {
			if a == "--rcon-port=25575" {
				hasPort = true
				break
			}
		}
		if !hasPort {
			return false, "agent missing --rcon-port=25575, got: " + strings.Join(agent.Args, " ")
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
		if gs.Status.Phase != gameplanev1alpha1.GameServerPhaseFailed {
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

	// Flip Suspend=true. Retry on conflict — the reconciler patches
	// status concurrently, racing a bare Get+Update's resourceVersion.
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		gs = getGameServer(t, ns, "smp")
		gs.Spec.Suspend = true
		return k8sClient.Update(context.Background(), gs)
	}); err != nil {
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
	gs.Spec.BackupPolicy = &gameplanev1alpha1.InlineBackupPolicy{
		Schedule: "0 */6 * * *",
		RepoRef:  gameplanev1alpha1.SecretKeySelector{Name: "repo", Key: "url"},
	}
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var bs gameplanev1alpha1.BackupSchedule
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
	gs.Spec.BackupPolicy = &gameplanev1alpha1.InlineBackupPolicy{
		Schedule: "0 0 * * *",
		RepoRef:  gameplanev1alpha1.SecretKeySelector{Name: "repo", Key: "url"},
	}
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var bs gameplanev1alpha1.BackupSchedule
		err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp-auto"}, &bs)
		return err == nil, "waiting for managed schedule"
	})

	// Now drop the policy. Retry on conflict — the reconciler updates the
	// GameServer concurrently, so a bare Get+Update races its resourceVersion
	// (envtest flake: "the object has been modified").
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		gs := getGameServer(t, ns, "smp")
		gs.Spec.BackupPolicy = nil
		return k8sClient.Update(context.Background(), gs)
	}); err != nil {
		t.Fatalf("update gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var bs gameplanev1alpha1.BackupSchedule
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
	gs.Spec.Storage = &gameplanev1alpha1.GameStorageSpec{Size: want}
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
	consistently(t, 500*time.Millisecond, func() (bool, string) {
		var pvc corev1.PersistentVolumeClaim
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp-data"}, &pvc); err != nil {
			return false, err.Error()
		}
		return true, ""
	})
}

// TestGameServer_AgentDataRootMatchesMountPath guards against the agent
// silently rooting all its file ops (Files tab, Mods tab, disk-usage stats)
// at its own /data default when the template mounts the data volume
// somewhere else. The operator must pass --data-root resolved from the same
// value used for the "data" VolumeMount on both containers, so the two can
// never drift apart end-to-end through a real reconcile.
func TestGameServer_AgentDataRootMatchesMountPath(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	// A custom mountPath, as shipped modules like rust (/steamcmd/rust) use.
	tmpl := buildGameTemplate(uniqueName("rust"))
	tmpl.Spec.Storage.MountPath = "/steamcmd/rust"
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
			return false, "statefulset: " + err.Error()
		}
		var agent *corev1.Container
		for i := range ss.Spec.Template.Spec.Containers {
			if ss.Spec.Template.Spec.Containers[i].Name == "agent" {
				agent = &ss.Spec.Template.Spec.Containers[i]
			}
		}
		if agent == nil {
			return false, "no agent sidecar container"
		}
		dataRoot := ""
		hasArg := false
		for _, a := range agent.Args {
			if strings.HasPrefix(a, "--data-root=") {
				dataRoot = strings.TrimPrefix(a, "--data-root=")
				hasArg = true
			}
		}
		if !hasArg {
			return false, "agent missing --data-root arg: " + strings.Join(agent.Args, " ")
		}
		if dataRoot != "/steamcmd/rust" {
			return false, "--data-root=" + dataRoot + ", want /steamcmd/rust"
		}
		mountPath := ""
		mounted := false
		for _, m := range agent.VolumeMounts {
			if m.Name == "data" {
				mountPath = m.MountPath
				mounted = true
			}
		}
		if !mounted {
			return false, "agent missing \"data\" VolumeMount"
		}
		// The invariant that was violated: the flag must match the actual
		// mount, not just some hardcoded expectation, so the two can't drift.
		if dataRoot != mountPath {
			return false, "--data-root=" + dataRoot + " != data VolumeMount path " + mountPath
		}
		return true, ""
	})
}

// TestGameServer_LoadBalancerSourceRanges asserts the CIDR allow-list is
// applied to the fronting Service only when Expose=LoadBalancer.
func TestGameServer_LoadBalancerSourceRanges(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("minecraft"))
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "smp", tmpl.Name)
	gs.Spec.Networking = gameplanev1alpha1.GameServerNetworking{
		Expose:       "LoadBalancer",
		SourceRanges: []string{"203.0.113.0/24", "10.0.0.0/8"},
	}
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var svc corev1.Service
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &svc); err != nil {
			return false, err.Error()
		}
		if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
			return false, "type = " + string(svc.Spec.Type)
		}
		if !equalStrings(svc.Spec.LoadBalancerSourceRanges, []string{"203.0.113.0/24", "10.0.0.0/8"}) {
			return false, "ranges = " + strings.Join(svc.Spec.LoadBalancerSourceRanges, ",")
		}
		return true, ""
	})
}

// TestGameServer_RemovedNetworkingConverges — removing a serviceAnnotation
// and clearing the nodeSelector from the GameServer must converge the child
// Service/StatefulSet rather than leaving the removed settings active.
func TestGameServer_RemovedNetworkingConverges(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("minecraft"))
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "conv", tmpl.Name)
	gs.Spec.NodeSelector = map[string]string{"disktype": "ssd"}
	gs.Spec.Networking = gameplanev1alpha1.GameServerNetworking{
		ServiceAnnotations: map[string]string{"k1": "v1", "k2": "v2"},
	}
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	// Initial: both annotations present and the nodeSelector set.
	eventually(t, func() (bool, string) {
		var svc corev1.Service
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "conv"}, &svc); err != nil {
			return false, err.Error()
		}
		if svc.Annotations["k1"] != "v1" || svc.Annotations["k2"] != "v2" {
			return false, "annotations not applied"
		}
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "conv"}, &ss); err != nil {
			return false, err.Error()
		}
		if ss.Spec.Template.Spec.NodeSelector["disktype"] != "ssd" {
			return false, "nodeSelector not set"
		}
		return true, ""
	})

	// Remove k2 from the annotations and clear the nodeSelector.
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var cur gameplanev1alpha1.GameServer
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "conv"}, &cur); err != nil {
			return err
		}
		cur.Spec.NodeSelector = nil
		cur.Spec.Networking.ServiceAnnotations = map[string]string{"k1": "v1"}
		return k8sClient.Update(context.Background(), &cur)
	}); err != nil {
		t.Fatalf("update gameserver: %v", err)
	}

	// Converged: k2 pruned, k1 kept, nodeSelector cleared.
	eventually(t, func() (bool, string) {
		var svc corev1.Service
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "conv"}, &svc); err != nil {
			return false, err.Error()
		}
		if svc.Annotations["k1"] != "v1" {
			return false, "k1 missing"
		}
		if _, ok := svc.Annotations["k2"]; ok {
			return false, "k2 not pruned"
		}
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "conv"}, &ss); err != nil {
			return false, err.Error()
		}
		if len(ss.Spec.Template.Spec.NodeSelector) != 0 {
			return false, "nodeSelector not cleared"
		}
		return true, ""
	})
}

// TestGameServer_HostnameSetsExternalDNSAnnotation — spec.networking.hostname
// is published as the external-dns hostname annotation on the game Service and
// pruned (via the managed-annotations sentinel) once the hostname is cleared.
func TestGameServer_HostnameSetsExternalDNSAnnotation(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("minecraft"))
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "dns", tmpl.Name)
	gs.Spec.Networking = gameplanev1alpha1.GameServerNetworking{Hostname: "mc.example.com"}
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	// The external-dns hint is stamped on the game Service and recorded as a
	// managed key so a later hostname removal converges.
	eventually(t, func() (bool, string) {
		var svc corev1.Service
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "dns"}, &svc); err != nil {
			return false, err.Error()
		}
		if svc.Annotations[externalDNSHostnameAnnotation] != "mc.example.com" {
			return false, "external-dns annotation not set: " + svc.Annotations[externalDNSHostnameAnnotation]
		}
		if !strings.Contains(svc.Annotations[managedServiceAnnotationsKey], externalDNSHostnameAnnotation) {
			return false, "sentinel missing external-dns key: " + svc.Annotations[managedServiceAnnotationsKey]
		}
		return true, ""
	})

	// Clear the hostname: the annotation drops out of the desired set and the
	// existing prune logic removes it from the Service.
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var cur gameplanev1alpha1.GameServer
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "dns"}, &cur); err != nil {
			return err
		}
		cur.Spec.Networking.Hostname = ""
		return k8sClient.Update(context.Background(), &cur)
	}); err != nil {
		t.Fatalf("update gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var svc corev1.Service
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "dns"}, &svc); err != nil {
			return false, err.Error()
		}
		if _, ok := svc.Annotations[externalDNSHostnameAnnotation]; ok {
			return false, "external-dns annotation not pruned: " + svc.Annotations[externalDNSHostnameAnnotation]
		}
		return true, ""
	})
}

// TestGameServer_HostportSetsContainerHostPort — expose: Hostport binds each
// game container port on the node (HostPort == ContainerPort) while the game
// Service stays ClusterIP.
func TestGameServer_HostportSetsContainerHostPort(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("minecraft"))
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "hp", tmpl.Name)
	gs.Spec.Networking = gameplanev1alpha1.GameServerNetworking{Expose: "Hostport"}
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "hp"}, &ss); err != nil {
			return false, err.Error()
		}
		var game *corev1.Container
		for i := range ss.Spec.Template.Spec.Containers {
			if ss.Spec.Template.Spec.Containers[i].Name == gameContainerName {
				game = &ss.Spec.Template.Spec.Containers[i]
				break
			}
		}
		if game == nil {
			return false, "no game container yet"
		}
		if len(game.Ports) == 0 {
			return false, "game container has no ports"
		}
		for _, p := range game.Ports {
			if p.HostPort != p.ContainerPort {
				return false, "hostPort not set for port " + p.Name
			}
		}
		var svc corev1.Service
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "hp"}, &svc); err != nil {
			return false, err.Error()
		}
		if svc.Spec.Type != corev1.ServiceTypeClusterIP {
			return false, "service type = " + string(svc.Spec.Type)
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
	tmpl.Spec.RCON = &gameplanev1alpha1.RCONSpec{Protocol: "source", Port: 25575}
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
	tmpl.Spec.ConfigSchema = []gameplanev1alpha1.ConfigField{
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
		if ss.Spec.Template.Annotations["gameplane.local/config-hash"] == "" {
			return false, "config-hash annotation missing"
		}
		return true, ""
	})
}

// TestGameServer_AutoMemoryConfigFromLimit — a configSchema field with
// autoFromMemoryLimit resolves to percent% of the server's memory limit
// when the user leaves it empty, and an explicit config value still wins.
func TestGameServer_AutoMemoryConfigFromLimit(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("minecraft"))
	tmpl.Spec.ConfigSchema = []gameplanev1alpha1.ConfigField{
		{Name: "MAX_MEMORY", Type: "string",
			AutoFromMemoryLimit: &gameplanev1alpha1.AutoFromMemoryLimit{Percent: 75}},
		{Name: "INIT_MEMORY", Type: "string",
			AutoFromMemoryLimit: &gameplanev1alpha1.AutoFromMemoryLimit{Percent: 50}},
	}
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "automem", tmpl.Name)
	gs.Spec.Resources = &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("8Gi")},
	}
	gs.Spec.Config = map[string]string{"INIT_MEMORY": "1G"} // explicit value beats auto
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "automem"}, &ss); err != nil {
			return false, "statefulset: " + err.Error()
		}
		got := map[string]string{}
		for _, c := range ss.Spec.Template.Spec.Containers {
			if c.Name != "game" {
				continue
			}
			for _, e := range c.Env {
				got[e.Name] = e.Value
			}
		}
		if got["MAX_MEMORY"] != "6144M" {
			return false, "MAX_MEMORY = " + got["MAX_MEMORY"] + ", want 6144M (75% of 8Gi)"
		}
		if got["INIT_MEMORY"] != "1G" {
			return false, "INIT_MEMORY = " + got["INIT_MEMORY"] + ", want the explicit 1G"
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
	tmpl.Spec.ConfigSchema = []gameplanev1alpha1.ConfigField{
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
		if got.Status.Phase != gameplanev1alpha1.GameServerPhaseFailed {
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

	// Retry on conflict — the reconciler patches status concurrently,
	// racing a bare Get+Update's resourceVersion.
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		gs = getGameServer(t, ns, "smp")
		gs.Spec.Config["MODE"] = "creative"
		return k8sClient.Update(context.Background(), gs)
	}); err != nil {
		t.Fatalf("update gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &ss); err != nil {
			return false, "statefulset: " + err.Error()
		}
		got := getGameServer(t, ns, "smp")
		if got.Status.Phase == gameplanev1alpha1.GameServerPhaseFailed {
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
	tmpl.Spec.ConfigSchema = []gameplanev1alpha1.ConfigField{
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

	// Dropping the password must clean the Secret up. Retry on conflict —
	// the reconciler patches status concurrently, racing a bare
	// Get+Update's resourceVersion.
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		gs = getGameServer(t, ns, "smp")
		gs.Spec.Config = nil
		return k8sClient.Update(context.Background(), gs)
	}); err != nil {
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

// TestGameServer_ConfigFilesMaterialize — target:file config renders
// into the owned `<gs>-files` Secret, wires the config-init container
// and the config-files volume, never leaks into env, and is torn down
// again when the template stops declaring configFiles.
func TestGameServer_ConfigFilesMaterialize(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("terraria"))
	tmpl.Spec.ConfigSchema = []gameplanev1alpha1.ConfigField{
		{Name: "MOTD", Type: "string", Target: "file", Default: "hello"},
		{Name: "SERVER_PASS", Type: "password", Target: "file"},
	}
	tmpl.Spec.ConfigFiles = []gameplanev1alpha1.ConfigFile{{
		Path:     "cfg/server.cfg",
		Template: "motd={{ .Values.MOTD }}\npass={{ .Values.SERVER_PASS }}\n",
	}}
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "smp", tmpl.Name)
	gs.Spec.Config = map[string]string{"SERVER_PASS": "hunter22"}
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	firstHash := ""
	eventually(t, func() (bool, string) {
		var sec corev1.Secret
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp-files"}, &sec); err != nil {
			return false, "files secret: " + err.Error()
		}
		if got := string(sec.Data["file-0"]); got != "motd=hello\npass=hunter22\n" {
			return false, "file-0 content = " + got
		}
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &ss); err != nil {
			return false, "statefulset: " + err.Error()
		}
		inits := ss.Spec.Template.Spec.InitContainers
		if len(inits) != 1 || inits[0].Name != "config-init" {
			return false, "init containers missing config-init"
		}
		var vol *corev1.Volume
		for i := range ss.Spec.Template.Spec.Volumes {
			if ss.Spec.Template.Spec.Volumes[i].Name == "config-files" {
				vol = &ss.Spec.Template.Spec.Volumes[i]
			}
		}
		if vol == nil || vol.Secret == nil || vol.Secret.SecretName != "smp-files" {
			return false, "config-files volume not backed by smp-files"
		}
		if len(vol.Secret.Items) != 1 || vol.Secret.Items[0].Key != "file-0" ||
			vol.Secret.Items[0].Path != "cfg/server.cfg" {
			return false, "volume items do not map file-0 to cfg/server.cfg"
		}
		for _, c := range ss.Spec.Template.Spec.Containers {
			for _, e := range c.Env {
				if e.Name == "MOTD" || e.Name == "SERVER_PASS" || e.Value == "hunter22" {
					return false, "file-target value leaked into env " + e.Name + " of " + c.Name
				}
			}
		}
		firstHash = ss.Spec.Template.Annotations["gameplane.local/config-hash"]
		if firstHash == "" {
			return false, "config-hash annotation missing"
		}
		return true, ""
	})

	// Changing a file-target value must re-render the Secret and roll
	// the pod template hash. Retry on conflict — the reconciler patches
	// status concurrently, so a bare Get+Update races its resourceVersion
	// (envtest flake: "the object has been modified").
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		gs = getGameServer(t, ns, "smp")
		gs.Spec.Config["MOTD"] = "welcome"
		return k8sClient.Update(context.Background(), gs)
	}); err != nil {
		t.Fatalf("update gameserver: %v", err)
	}
	eventually(t, func() (bool, string) {
		var sec corev1.Secret
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp-files"}, &sec); err != nil {
			return false, "files secret: " + err.Error()
		}
		if got := string(sec.Data["file-0"]); got != "motd=welcome\npass=hunter22\n" {
			return false, "file-0 content = " + got
		}
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &ss); err != nil {
			return false, "statefulset: " + err.Error()
		}
		if ss.Spec.Template.Annotations["gameplane.local/config-hash"] == firstHash {
			return false, "config-hash did not roll"
		}
		return true, ""
	})

	// Dropping configFiles from the template must delete the Secret and
	// strip the init container + volume. Template edits don't trigger a
	// GameServer reconcile (no cross-watch), so poke the GameServer.
	// Retry on conflict — reconcilers patch status concurrently, racing a
	// bare Get+Update's resourceVersion.
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		tmplNow := &gameplanev1alpha1.GameTemplate{}
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Name: tmpl.Name}, tmplNow); err != nil {
			return err
		}
		tmplNow.Spec.ConfigSchema = nil
		tmplNow.Spec.ConfigFiles = nil
		return k8sClient.Update(context.Background(), tmplNow)
	}); err != nil {
		t.Fatalf("update template: %v", err)
	}
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		gs = getGameServer(t, ns, "smp")
		gs.Spec.Config = nil
		return k8sClient.Update(context.Background(), gs)
	}); err != nil {
		t.Fatalf("update gameserver: %v", err)
	}
	eventually(t, func() (bool, string) {
		// Re-poke each poll: the reconcile racing the template cache
		// update must not strand the test.
		poke := getGameServer(t, ns, "smp")
		if poke.Annotations == nil {
			poke.Annotations = map[string]string{}
		}
		poke.Annotations["test.gameplane.local/poke"] = time.Now().Format(time.RFC3339Nano)
		_ = k8sClient.Update(context.Background(), poke)

		var sec corev1.Secret
		err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp-files"}, &sec)
		if err == nil {
			return false, "files secret still exists"
		}
		if !apierrors.IsNotFound(err) {
			return false, err.Error()
		}
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &ss); err != nil {
			return false, "statefulset: " + err.Error()
		}
		if len(ss.Spec.Template.Spec.InitContainers) != 0 {
			return false, "config-init container still present"
		}
		for _, v := range ss.Spec.Template.Spec.Volumes {
			if v.Name == "config-files" {
				return false, "config-files volume still present"
			}
		}
		return true, ""
	})
}

// TestGameServer_BadConfigFileTemplateFails — a template that fails to
// render flips the GameServer to Failed with a pointed message and
// creates no StatefulSet; fixing the template recovers.
func TestGameServer_BadConfigFileTemplateFails(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("terraria"))
	tmpl.Spec.ConfigFiles = []gameplanev1alpha1.ConfigFile{{
		Path:     "server.cfg",
		Template: "{{ .Values.NO_SUCH_FIELD }}",
	}}
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "smp", tmpl.Name)
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		got := getGameServer(t, ns, "smp")
		if got.Status.Phase != gameplanev1alpha1.GameServerPhaseFailed {
			return false, "phase = " + string(got.Status.Phase)
		}
		for _, c := range got.Status.Conditions {
			if c.Type == "Ready" && strings.Contains(c.Message, "server.cfg") {
				return true, ""
			}
		}
		return false, "Ready condition does not mention the offending file"
	})

	var ss appsv1.StatefulSet
	if err := k8sClient.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: "smp"}, &ss); !apierrors.IsNotFound(err) {
		t.Fatalf("StatefulSet should not exist while the template is broken, get err = %v", err)
	}
}

// TestGameServer_ConfigChangeRollsPodTemplate — editing a config value
// must change the pod template (env + hash annotation) so the
// StatefulSet rolls the pod.
func TestGameServer_ConfigChangeRollsPodTemplate(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("minecraft"))
	tmpl.Spec.ConfigSchema = []gameplanev1alpha1.ConfigField{
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
		firstHash = ss.Spec.Template.Annotations["gameplane.local/config-hash"]
		return firstHash != "", "config-hash annotation missing"
	})

	// Retry on conflict — the reconciler patches status concurrently,
	// racing a bare Get+Update's resourceVersion.
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		gs = getGameServer(t, ns, "smp")
		gs.Spec.Config = map[string]string{"DIFFICULTY": "hard"}
		return k8sClient.Update(context.Background(), gs)
	}); err != nil {
		t.Fatalf("update gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "smp"}, &ss); err != nil {
			return false, err.Error()
		}
		hash := ss.Spec.Template.Annotations["gameplane.local/config-hash"]
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

// gameContainerEnv fetches the reconciled StatefulSet's "game" container
// env as the raw slice, in declaration order. Deliberately NOT collapsed
// into a name->value map here: a map can't see a duplicate entry (the
// mods-by-id projection once appended a shadow duplicate instead of
// replacing in place — see modIDListEnv), so callers that care about
// "exactly one entry" must use envCount/envValue below on this slice
// instead of losing that information to a map.
//
// It returns an error instead of calling t.Fatal on a miss: callers poll
// this from inside an eventually() condition, and eventually's cond runs
// in the test's own goroutine — a t.Fatal in there would abort the whole
// test on the very first (pre-reconcile) attempt instead of letting the
// poll retry.
func gameContainerEnv(t *testing.T, ns, name string) ([]corev1.EnvVar, error) {
	t.Helper()
	var ss appsv1.StatefulSet
	if err := k8sClient.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: name}, &ss); err != nil {
		return nil, fmt.Errorf("get statefulset: %w", err)
	}
	for _, c := range ss.Spec.Template.Spec.Containers {
		if c.Name == gameContainerName {
			return c.Env, nil
		}
	}
	return nil, nil
}

// envCount returns how many entries in env are named name — used to
// assert "exactly one", a property a name->value map can't express since
// it silently collapses duplicates.
func envCount(env []corev1.EnvVar, name string) int {
	n := 0
	for _, e := range env {
		if e.Name == name {
			n++
		}
	}
	return n
}

// envValue returns the value of the LAST entry named name in env — the
// kubelet's own duplicate-resolution rule — and whether one was found.
func envValue(env []corev1.EnvVar, name string) (string, bool) {
	var val string
	var ok bool
	for _, e := range env {
		if e.Name == name {
			val, ok = e.Value, true
		}
	}
	return val, ok
}

// TestGameServer_ModIDList_ReplaceMode — a template declaring
// capabilities.mods.idList in (default) replace mode gets its env var set
// to the GameServer's selected ids, joined with the default "," separator.
func TestGameServer_ModIDList_ReplaceMode(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("zomboid"))
	tmpl.Spec.Capabilities = &gameplanev1alpha1.CapabilitiesSpec{
		Mods: &gameplanev1alpha1.ModsSpec{
			IDList: &gameplanev1alpha1.ModIDListSpec{Env: "MOD_IDS"},
		},
	}
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "zomboid", tmpl.Name)
	gs.Spec.Mods = &gameplanev1alpha1.GameServerModsSpec{
		IDs: []gameplanev1alpha1.ModRef{{ID: "111"}, {ID: "222"}},
	}
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		got, err := gameContainerEnv(t, ns, "zomboid")
		if err != nil {
			return false, err.Error()
		}
		if v, ok := envValue(got, "MOD_IDS"); !ok || v != "111,222" {
			return false, fmt.Sprintf("MOD_IDS = %q (present=%v), want \"111,222\"", v, ok)
		}
		// Guards the modIDListEnv duplicate-env regression: a stray
		// shadow entry is invisible to a name->value map but must still
		// fail this test.
		if n := envCount(got, "MOD_IDS"); n != 1 {
			return false, fmt.Sprintf("MOD_IDS appears %d times, want exactly 1", n)
		}
		return true, ""
	})
}

// TestGameServer_ModIDList_AppendMode — an ARK-shaped template (append
// mode, a config-schema-provided launch string) keeps the config value
// and gains the rendered mod flag onto the end of it.
func TestGameServer_ModIDList_AppendMode(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("ark"))
	tmpl.Spec.ConfigSchema = []gameplanev1alpha1.ConfigField{
		{Name: "ASA_START_PARAMS", Type: "string", Default: "TheIsland_WP?listen"},
	}
	tmpl.Spec.Capabilities = &gameplanev1alpha1.CapabilitiesSpec{
		Mods: &gameplanev1alpha1.ModsSpec{
			IDList: &gameplanev1alpha1.ModIDListSpec{
				Env:    "ASA_START_PARAMS",
				Format: " -mods={{ids}}",
				Mode:   "append",
			},
		},
	}
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "ark", tmpl.Name)
	gs.Spec.Mods = &gameplanev1alpha1.GameServerModsSpec{
		IDs: []gameplanev1alpha1.ModRef{{ID: "111"}, {ID: "222"}},
	}
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	want := "TheIsland_WP?listen -mods=111,222"
	eventually(t, func() (bool, string) {
		got, err := gameContainerEnv(t, ns, "ark")
		if err != nil {
			return false, err.Error()
		}
		if v, ok := envValue(got, "ASA_START_PARAMS"); !ok || v != want {
			return false, fmt.Sprintf("ASA_START_PARAMS = %q (present=%v), want %q", v, ok, want)
		}
		// This is the exact shape of the live-cluster bug: the
		// config-schema default and the mods-by-id projection share a
		// name (ASA_START_PARAMS). modIDListEnv must merge them into one
		// entry, not append a second one that only the kubelet's
		// last-wins rule happens to paper over.
		if n := envCount(got, "ASA_START_PARAMS"); n != 1 {
			return false, fmt.Sprintf("ASA_START_PARAMS appears %d times, want exactly 1", n)
		}
		return true, ""
	})
}

// TestGameServer_ModIDList_NoIDsProjectsNothing — a template declaring
// idList but a GameServer with no selected mods must leave the target env
// var completely absent, not set to an empty string (an empty
// "-mods=" would break games like ARK that don't tolerate a trailing
// empty flag).
func TestGameServer_ModIDList_NoIDsProjectsNothing(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("zomboid"))
	tmpl.Spec.Capabilities = &gameplanev1alpha1.CapabilitiesSpec{
		Mods: &gameplanev1alpha1.ModsSpec{
			IDList: &gameplanev1alpha1.ModIDListSpec{Env: "MOD_IDS"},
		},
	}
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	// No spec.mods at all — the common case for a server that just never
	// picked any mods.
	gs := buildGameServer(ns, "noids", tmpl.Name)
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	// Wait for the StatefulSet to exist (reconciliation settled) before
	// asserting the negative — otherwise a not-yet-reconciled object
	// would pass a "key absent" check for the wrong reason.
	eventually(t, func() (bool, string) {
		var ss appsv1.StatefulSet
		if err := k8sClient.Get(context.Background(),
			types.NamespacedName{Namespace: ns, Name: "noids"}, &ss); err != nil {
			return false, err.Error()
		}
		return len(ss.Spec.Template.Spec.Containers) > 0, "statefulset not yet populated"
	})

	got, err := gameContainerEnv(t, ns, "noids")
	if err != nil {
		t.Fatalf("get statefulset: %v", err)
	}
	if v, ok := envValue(got, "MOD_IDS"); ok {
		t.Fatalf("MOD_IDS should not be projected with no selected ids, got %q", v)
	}
}

// TestGameServer_ModIDList_CustomSeparator — a non-default Separator is
// honored when joining ids.
func TestGameServer_ModIDList_CustomSeparator(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameServerReconciler(t, ns))

	tmpl := buildGameTemplate(uniqueName("zomboid"))
	tmpl.Spec.Capabilities = &gameplanev1alpha1.CapabilitiesSpec{
		Mods: &gameplanev1alpha1.ModsSpec{
			IDList: &gameplanev1alpha1.ModIDListSpec{Env: "MOD_IDS", Separator: ";"},
		},
	}
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	gs := buildGameServer(ns, "sepzomboid", tmpl.Name)
	gs.Spec.Mods = &gameplanev1alpha1.GameServerModsSpec{
		IDs: []gameplanev1alpha1.ModRef{{ID: "111"}, {ID: "222"}, {ID: "333"}},
	}
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	eventually(t, func() (bool, string) {
		got, err := gameContainerEnv(t, ns, "sepzomboid")
		if err != nil {
			return false, err.Error()
		}
		if v, ok := envValue(got, "MOD_IDS"); !ok || v != "111;222;333" {
			return false, fmt.Sprintf("MOD_IDS = %q (present=%v), want \"111;222;333\"", v, ok)
		}
		if n := envCount(got, "MOD_IDS"); n != 1 {
			return false, fmt.Sprintf("MOD_IDS appears %d times, want exactly 1", n)
		}
		return true, ""
	})
}
