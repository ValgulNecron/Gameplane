//go:build envtest

package controller

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

func TestSentinelReconciliation(t *testing.T) {
	t.Run("sentinel created when asleep and armed", func(t *testing.T) {
		ns := newNamespace(t)
		c := startMgr(t, ns, withGameServerReconciler(t, ns))

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		gs := &gameplanev1alpha1.GameServer{
			ObjectMeta: metav1.ObjectMeta{Name: "test-sentinel-create", Namespace: ns},
			Spec: gameplanev1alpha1.GameServerSpec{
				TemplateRef: gameplanev1alpha1.GameTemplateRef{Name: "test-template"},
				Idle: &gameplanev1alpha1.IdleSpec{
					Enabled:       true,
					WakeOnConnect: true,
				},
			},
		}

		tmpl := &gameplanev1alpha1.GameTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "test-template"},
			Spec: gameplanev1alpha1.GameTemplateSpec{
				Ports: []gameplanev1alpha1.GamePort{
					{
						Name:          "game",
						ContainerPort: 25565,
						Protocol:      corev1.ProtocolTCP,
						Advertise:     true,
						WakeProtocol:  "minecraft",
					},
				},
			},
		}

		if err := c.Create(ctx, tmpl); err != nil {
			t.Fatalf("create template: %v", err)
		}
		if err := c.Create(ctx, gs); err != nil {
			t.Fatalf("create gameserver: %v", err)
		}

		// Mark as asleep by adding the annotation
		gs.Annotations = map[string]string{
			IdleAsleepSinceAnnotation: time.Now().Format(time.RFC3339),
		}
		if err := c.Update(ctx, gs); err != nil {
			t.Fatalf("update gameserver: %v", err)
		}

		// Reconcile: should create sentinel
		r := &GameServerReconciler{
			Client: c,
			Scheme: scheme,
		}

		if err := r.reconcileSentinel(ctx, gs, tmpl, idleAsleep); err != nil {
			t.Fatalf("reconcileSentinel: %v", err)
		}

		// Verify sentinel Deployment exists
		var dep appsv1.Deployment
		if err := c.Get(ctx, types.NamespacedName{
			Name:      "test-sentinel-create-waker",
			Namespace: ns,
		}, &dep); err != nil {
			t.Fatalf("get sentinel deployment: %v", err)
		}

		// Verify it has 1 replica
		if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 1 {
			t.Errorf("want 1 replica, got %v", dep.Spec.Replicas)
		}

		// Verify labels
		if dep.Spec.Selector.MatchLabels["app.kubernetes.io/name"] != "gameplane-waker" {
			t.Error("sentinel label not set correctly")
		}
	})

	t.Run("sentinel not created when wakeOnConnect is false", func(t *testing.T) {
		ns := newNamespace(t)
		c := startMgr(t, ns, withGameServerReconciler(t, ns))

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		gs := &gameplanev1alpha1.GameServer{
			ObjectMeta: metav1.ObjectMeta{Name: "test-no-sentinel-disabled", Namespace: ns},
			Spec: gameplanev1alpha1.GameServerSpec{
				TemplateRef: gameplanev1alpha1.GameTemplateRef{Name: "test-template"},
				Idle: &gameplanev1alpha1.IdleSpec{
					Enabled:       true,
					WakeOnConnect: false,
				},
			},
		}

		tmpl := &gameplanev1alpha1.GameTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "test-template"},
			Spec: gameplanev1alpha1.GameTemplateSpec{
				Ports: []gameplanev1alpha1.GamePort{
					{
						Name:          "game",
						ContainerPort: 25565,
						Protocol:      corev1.ProtocolTCP,
						Advertise:     true,
						WakeProtocol:  "minecraft",
					},
				},
			},
		}

		if err := c.Create(ctx, tmpl); err != nil {
			t.Fatalf("create template: %v", err)
		}
		if err := c.Create(ctx, gs); err != nil {
			t.Fatalf("create gameserver: %v", err)
		}

		gs.Annotations = map[string]string{
			IdleAsleepSinceAnnotation: time.Now().Format(time.RFC3339),
		}
		if err := c.Update(ctx, gs); err != nil {
			t.Fatalf("update gameserver: %v", err)
		}

		r := &GameServerReconciler{
			Client: c,
			Scheme: scheme,
		}

		// reconcileSentinel should delete it
		if err := r.reconcileSentinel(ctx, gs, tmpl, idleAsleep); err != nil {
			t.Fatalf("reconcileSentinel: %v", err)
		}

		// Verify sentinel does not exist
		var dep appsv1.Deployment
		err := c.Get(ctx, types.NamespacedName{
			Name:      "test-no-sentinel-disabled-waker",
			Namespace: ns,
		}, &dep)
		if err == nil {
			t.Error("sentinel should not exist when wakeOnConnect is false")
		}
	})

	t.Run("sentinel not created when all ports have wakeProtocol: none", func(t *testing.T) {
		ns := newNamespace(t)
		c := startMgr(t, ns, withGameServerReconciler(t, ns))

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		gs := &gameplanev1alpha1.GameServer{
			ObjectMeta: metav1.ObjectMeta{Name: "test-sentinel-no-protocol", Namespace: ns},
			Spec: gameplanev1alpha1.GameServerSpec{
				TemplateRef: gameplanev1alpha1.GameTemplateRef{Name: "test-template"},
				Idle: &gameplanev1alpha1.IdleSpec{
					Enabled:       true,
					WakeOnConnect: true,
				},
			},
		}

		tmpl := &gameplanev1alpha1.GameTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "test-template"},
			Spec: gameplanev1alpha1.GameTemplateSpec{
				Ports: []gameplanev1alpha1.GamePort{
					{
						Name:          "game",
						ContainerPort: 25565,
						Protocol:      corev1.ProtocolTCP,
						Advertise:     true,
						WakeProtocol:  "none",
					},
				},
			},
		}

		if err := c.Create(ctx, tmpl); err != nil {
			t.Fatalf("create template: %v", err)
		}
		if err := c.Create(ctx, gs); err != nil {
			t.Fatalf("create gameserver: %v", err)
		}

		gs.Annotations = map[string]string{
			IdleAsleepSinceAnnotation: time.Now().Format(time.RFC3339),
		}
		if err := c.Update(ctx, gs); err != nil {
			t.Fatalf("update gameserver: %v", err)
		}

		r := &GameServerReconciler{
			Client: c,
			Scheme: scheme,
		}

		if err := r.reconcileSentinel(ctx, gs, tmpl, idleAsleep); err != nil {
			t.Fatalf("reconcileSentinel: %v", err)
		}

		// Verify sentinel does not exist
		var dep appsv1.Deployment
		err := c.Get(ctx, types.NamespacedName{
			Name:      "test-sentinel-no-protocol-waker",
			Namespace: ns,
		}, &dep)
		if err == nil {
			t.Error("sentinel should not exist when all ports have wakeProtocol: none")
		}
	})

	t.Run("sentinel deleted on wake", func(t *testing.T) {
		ns := newNamespace(t)
		c := startMgr(t, ns, withGameServerReconciler(t, ns))

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		gs := &gameplanev1alpha1.GameServer{
			ObjectMeta: metav1.ObjectMeta{Name: "test-sentinel-delete", Namespace: ns},
			Spec: gameplanev1alpha1.GameServerSpec{
				TemplateRef: gameplanev1alpha1.GameTemplateRef{Name: "test-template"},
				Idle: &gameplanev1alpha1.IdleSpec{
					Enabled:       true,
					WakeOnConnect: true,
				},
			},
		}

		tmpl := &gameplanev1alpha1.GameTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "test-template"},
			Spec: gameplanev1alpha1.GameTemplateSpec{
				Ports: []gameplanev1alpha1.GamePort{
					{
						Name:          "game",
						ContainerPort: 25565,
						Protocol:      corev1.ProtocolTCP,
						Advertise:     true,
						WakeProtocol:  "minecraft",
					},
				},
			},
		}

		if err := c.Create(ctx, tmpl); err != nil {
			t.Fatalf("create template: %v", err)
		}
		if err := c.Create(ctx, gs); err != nil {
			t.Fatalf("create gameserver: %v", err)
		}

		r := &GameServerReconciler{
			Client: c,
			Scheme: scheme,
		}

		// Create sentinel (asleep)
		gs.Annotations = map[string]string{
			IdleAsleepSinceAnnotation: time.Now().Format(time.RFC3339),
		}
		if err := c.Update(ctx, gs); err != nil {
			t.Fatalf("update gameserver: %v", err)
		}
		if err := r.reconcileSentinel(ctx, gs, tmpl, idleAsleep); err != nil {
			t.Fatalf("reconcileSentinel create: %v", err)
		}

		// Remove sleep annotation (wake)
		gs.Annotations = map[string]string{}
		if err := c.Update(ctx, gs); err != nil {
			t.Fatalf("update gameserver: %v", err)
		}

		// Reconcile: should delete sentinel
		if err := r.reconcileSentinel(ctx, gs, tmpl, idleAwake); err != nil {
			t.Fatalf("reconcileSentinel delete: %v", err)
		}

		// Verify sentinel is deleted
		var dep appsv1.Deployment
		err := c.Get(ctx, types.NamespacedName{
			Name:      "test-sentinel-delete-waker",
			Namespace: ns,
		}, &dep)
		if err == nil {
			t.Error("sentinel should be deleted on wake")
		}
	})

	t.Run("game-direct Service created and synced", func(t *testing.T) {
		ns := newNamespace(t)
		c := startMgr(t, ns, withGameServerReconciler(t, ns))

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		gs := &gameplanev1alpha1.GameServer{
			ObjectMeta: metav1.ObjectMeta{Name: "test-game-direct", Namespace: ns},
			Spec: gameplanev1alpha1.GameServerSpec{
				TemplateRef: gameplanev1alpha1.GameTemplateRef{Name: "test-template"},
			},
		}

		tmpl := &gameplanev1alpha1.GameTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "test-template"},
			Spec: gameplanev1alpha1.GameTemplateSpec{
				Ports: []gameplanev1alpha1.GamePort{
					{
						Name:          "game",
						ContainerPort: 25565,
						Protocol:      corev1.ProtocolTCP,
						Advertise:     true,
					},
				},
			},
		}

		if err := c.Create(ctx, tmpl); err != nil {
			t.Fatalf("create template: %v", err)
		}
		if err := c.Create(ctx, gs); err != nil {
			t.Fatalf("create gameserver: %v", err)
		}

		r := &GameServerReconciler{
			Client: c,
			Scheme: scheme,
		}

		if err := r.reconcileGameDirectServiceFromTemplate(ctx, gs, tmpl); err != nil {
			t.Fatalf("reconcileGameDirectServiceFromTemplate: %v", err)
		}

		// Verify Service exists with correct selector
		var svc corev1.Service
		if err := c.Get(ctx, types.NamespacedName{
			Name:      "test-game-direct-game-direct",
			Namespace: ns,
		}, &svc); err != nil {
			t.Fatalf("get game-direct service: %v", err)
		}

		// Verify selector points to game pod
		if svc.Spec.Selector["app.kubernetes.io/name"] != "gameplane-game" {
			t.Error("game-direct Service selector should point to game pod")
		}

		// Verify PublishNotReadyAddresses is set
		if !svc.Spec.PublishNotReadyAddresses {
			t.Error("PublishNotReadyAddresses should be true")
		}
	})

	t.Run("Hostport mode sets HostPort on sentinel container", func(t *testing.T) {
		ns := newNamespace(t)
		c := startMgr(t, ns, withGameServerReconciler(t, ns))

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		gs := &gameplanev1alpha1.GameServer{
			ObjectMeta: metav1.ObjectMeta{Name: "test-sentinel-hostport", Namespace: ns},
			Spec: gameplanev1alpha1.GameServerSpec{
				TemplateRef: gameplanev1alpha1.GameTemplateRef{Name: "test-template"},
				Idle: &gameplanev1alpha1.IdleSpec{
					Enabled:       true,
					WakeOnConnect: true,
				},
				Networking: gameplanev1alpha1.GameServerNetworking{
					Expose: "Hostport",
				},
			},
		}

		tmpl := &gameplanev1alpha1.GameTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "test-template"},
			Spec: gameplanev1alpha1.GameTemplateSpec{
				Ports: []gameplanev1alpha1.GamePort{
					{
						Name:          "game",
						ContainerPort: 25565,
						Protocol:      corev1.ProtocolTCP,
						Advertise:     true,
						WakeProtocol:  "minecraft",
					},
				},
			},
		}

		if err := c.Create(ctx, tmpl); err != nil {
			t.Fatalf("create template: %v", err)
		}
		if err := c.Create(ctx, gs); err != nil {
			t.Fatalf("create gameserver: %v", err)
		}

		gs.Annotations = map[string]string{
			IdleAsleepSinceAnnotation: time.Now().Format(time.RFC3339),
		}
		if err := c.Update(ctx, gs); err != nil {
			t.Fatalf("update gameserver: %v", err)
		}

		r := &GameServerReconciler{
			Client: c,
			Scheme: scheme,
		}

		if err := r.reconcileSentinel(ctx, gs, tmpl, idleAsleep); err != nil {
			t.Fatalf("reconcileSentinel: %v", err)
		}

		// Verify Hostport is set on container port
		var dep appsv1.Deployment
		if err := c.Get(ctx, types.NamespacedName{
			Name:      "test-sentinel-hostport-waker",
			Namespace: ns,
		}, &dep); err != nil {
			t.Fatalf("get sentinel deployment: %v", err)
		}

		if len(dep.Spec.Template.Spec.Containers) == 0 {
			t.Fatal("no containers in sentinel deployment")
		}

		container := dep.Spec.Template.Spec.Containers[0]
		if len(container.Ports) == 0 {
			t.Fatal("no ports in sentinel container")
		}

		port := container.Ports[0]
		if port.HostPort != 25565 {
			t.Errorf("HostPort should be 25565, got %d", port.HostPort)
		}
	})
}
