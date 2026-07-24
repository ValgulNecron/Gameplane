package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

const (
	sentinelLabel = "app.kubernetes.io/name"
	sentinelValue = "gameplane-waker"
)

// reconcileSentinel maintains a 1-replica Deployment `<gs>-waker` that
// serves as the wake sentinel while the server is asleep. The sentinel holds
// the advertised ports and wakes the server when a player connects.
// It is created only when idle.wakeOnConnect is true, the server is asleep,
// and at least one advertised port has a non-"none" wakeProtocol.
func (r *GameServerReconciler) reconcileSentinel(
	ctx context.Context, gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate,
	idle idleState,
) error {
	if !shouldHaveSentinel(gs, tmpl, idle) {
		// Delete the sentinel if it exists.
		return r.deleteSentinel(ctx, gs.Namespace, gs.Name)
	}

	// Create or update the sentinel Deployment.
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gs.Name + "-waker",
			Namespace: gs.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		dep.Spec.Replicas = pointer(int32(1))
		dep.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				sentinelLabel:                sentinelValue,
				"app.kubernetes.io/instance": gs.Name,
			},
		}
		dep.Spec.Template.ObjectMeta.Labels = map[string]string{
			sentinelLabel:                sentinelValue,
			"app.kubernetes.io/instance": gs.Name,
		}

		// Container ports: advertised ports only, mirroring buildGameContainer.
		ports := make([]corev1.ContainerPort, 0)
		hostPort := gs.Spec.Networking.Expose == "Hostport"
		for _, p := range tmpl.Spec.Ports {
			if !p.Advertise {
				continue
			}
			cp := corev1.ContainerPort{
				Name:          p.Name,
				ContainerPort: p.ContainerPort,
				Protocol:      p.Protocol,
			}
			// Mirror buildGameContainer: only bind advertised ports on the host
			// in Hostport mode.
			if hostPort {
				cp.HostPort = p.ContainerPort
			}
			ports = append(ports, cp)
		}

		// Security context: matching the wipe Job's hardened context.
		uid := int64(65532)
		nonRoot := true
		noPrivEsc := false
		dep.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsNonRoot:   &nonRoot,
			RunAsUser:      &uid,
			RunAsGroup:     &uid,
			FSGroup:        &uid,
			SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		}

		// Sentinel container passes config via env or args. For now,
		// pass the minimal set: GameServer name/namespace and per-port config.
		image := r.SentinelImage
		if image == "" {
			image = DefaultSentinelImage
		}

		// Build the per-port config env var (comma-separated list).
		// Format: "port:protocol:wakeProtocol,..." e.g. "25565:TCP:minecraft,19133:UDP:generic"
		portConfig := ""
		for i, p := range tmpl.Spec.Ports {
			if !p.Advertise {
				continue
			}
			if i > 0 {
				portConfig += ","
			}
			proto := p.Protocol
			if proto == "" {
				proto = corev1.ProtocolTCP
			}
			portConfig += fmt.Sprintf("%d:%s:%s", p.ContainerPort, proto, p.WakeProtocol)
		}

		dep.Spec.Template.Spec.Containers = []corev1.Container{{
			Name:  "sentinel",
			Image: image,
			Env: []corev1.EnvVar{
				{Name: "GAMESERVER_NAME", Value: gs.Name},
				{Name: "GAMESERVER_NAMESPACE", Value: gs.Namespace},
				{Name: "PORTS_CONFIG", Value: portConfig},
			},
			Ports: ports,
			SecurityContext: &corev1.SecurityContext{
				RunAsNonRoot:             &nonRoot,
				RunAsUser:                &uid,
				AllowPrivilegeEscalation: &noPrivEsc,
				Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
			},
		}}

		return controllerutil.SetControllerReference(gs, dep, r.Scheme)
	})
	return err
}

// reconcileGameDirectService maintains a ClusterIP Service `<gs>-game-direct`
// that always selects the game pod labels. This Service allows the sentinel
// to reach the game pod when the main game Service's selector is flipped to
// the sentinel.
//
// The sentinel cannot proxy through the game's own Service — while asleep
// the sentinel IS that Service's endpoint. A separate per-pod DNS Service
// lets the sentinel reach the game pod even while it is that pod's external
// endpoint. Mirrors reconcileAgentService's reasoning: per-pod DNS only
// resolves under headless Services (or Services with PublishNotReadyAddresses),
// which the game Service is not; the agent and sentinel each need their own
// internal Service to be reachable by name.
//
// Note: tmpl is passed nil here so we only pass it during reconcileGameDirectService
// call. The caller must pass the template when calling this.
func (r *GameServerReconciler) reconcileGameDirectServiceFromTemplate(
	ctx context.Context, gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate,
) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: gs.Name + "-game-direct", Namespace: gs.Namespace},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec.Type = corev1.ServiceTypeClusterIP
		// Allow the sentinel to reach the game pod while the game pod is
		// starting (before readiness). Match the agent Service's
		// PublishNotReadyAddresses so the sentinel connects even if the game
		// is warming up.
		svc.Spec.PublishNotReadyAddresses = true
		svc.Spec.Selector = map[string]string{
			"app.kubernetes.io/name":     "gameplane-game",
			"app.kubernetes.io/instance": gs.Name,
		}
		// Expose advertised template ports only, mirroring svcPortsFromTemplate.
		svc.Spec.Ports = svcPortsFromTemplate(tmpl, gs)
		return controllerutil.SetControllerReference(gs, svc, r.Scheme)
	})
	return err
}

// updateServiceSelector updates the game Service selector based on idle state.
// While the sentinel is running, the Service points at sentinel labels.
// Otherwise, it points at the game pod labels.
// Returns true if a requeue is needed (waiting for sentinel to be ready).
func (r *GameServerReconciler) updateServiceSelector(
	ctx context.Context, gs *gameplanev1alpha1.GameServer, idle idleState, sentinelReady bool,
) (bool, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: gs.Name, Namespace: gs.Namespace},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		// Decide the selector based on idle state and sentinel readiness.
		// Into sleep: sentinel must be ready before flipping.
		// Out of sleep: flip back to game pod once game is ready.
		if idle == idleAsleep && sentinelReady {
			svc.Spec.Selector = map[string]string{
				sentinelLabel:                sentinelValue,
				"app.kubernetes.io/instance": gs.Name,
			}
		} else {
			svc.Spec.Selector = map[string]string{
				"app.kubernetes.io/name":     "gameplane-game",
				"app.kubernetes.io/instance": gs.Name,
			}
		}
		return nil
	})
	return false, err
}

// shouldHaveSentinel reports whether the server should have a sentinel Deployment.
func shouldHaveSentinel(gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate, idle idleState) bool {
	// Only if idle is enabled and wakeOnConnect is armed.
	if gs.Spec.Idle == nil || !gs.Spec.Idle.Enabled || !gs.Spec.Idle.WakeOnConnect {
		return false
	}
	// Only while the server is actually asleep.
	if idle != idleAsleep {
		return false
	}
	// Only if at least one advertised port has a non-"none" wakeProtocol.
	for _, p := range tmpl.Spec.Ports {
		if p.Advertise && p.WakeProtocol != "none" {
			return true
		}
	}
	return false
}

// sentinelIsReady reports whether the sentinel Deployment has a ready replica.
func (r *GameServerReconciler) sentinelIsReady(ctx context.Context, namespace, gsName string) (bool, error) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gsName + "-waker",
			Namespace: namespace,
		},
	}
	err := r.Get(ctx, client.ObjectKeyFromObject(dep), dep)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get sentinel deployment: %w", err)
	}
	return dep.Status.ReadyReplicas > 0, nil
}

// deleteSentinel removes the sentinel Deployment if it exists.
func (r *GameServerReconciler) deleteSentinel(ctx context.Context, namespace, gsName string) error {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gsName + "-waker",
			Namespace: namespace,
		},
	}
	err := r.Get(ctx, client.ObjectKeyFromObject(dep), dep)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get sentinel deployment: %w", err)
	}
	policy := metav1.DeletePropagationBackground
	return client.IgnoreNotFound(r.Delete(ctx, dep, &client.DeleteOptions{PropagationPolicy: &policy}))
}

// reconcileSentinelRBAC ensures the sentinel ServiceAccount and Roles exist,
// granting permission to get and patch the GameServer's annotations.
func (r *GameServerReconciler) reconcileSentinelRBAC(ctx context.Context, gs *gameplanev1alpha1.GameServer) error {
	// ServiceAccount: <gs>-waker-sa
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gs.Name + "-waker-sa",
			Namespace: gs.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, sa, func() error {
		return controllerutil.SetControllerReference(gs, sa, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconcile sentinel serviceaccount: %w", err)
	}

	// Role: <gs>-waker-role, granting get + patch on this GameServer's status.
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gs.Name + "-waker-role",
			Namespace: gs.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, role, func() error {
		role.Rules = []rbacv1.PolicyRule{{
			APIGroups:     []string{"gameplane.local"},
			Resources:     []string{"gameservers"},
			Verbs:         []string{"get", "patch"},
			ResourceNames: []string{gs.Name},
		}}
		return controllerutil.SetControllerReference(gs, role, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconcile sentinel role: %w", err)
	}

	// RoleBinding: <gs>-waker-rb, binding the SA to the role.
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gs.Name + "-waker-rb",
			Namespace: gs.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, rb, func() error {
		rb.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
		}
		rb.Subjects = []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      sa.Name,
			Namespace: gs.Namespace,
		}}
		return controllerutil.SetControllerReference(gs, rb, r.Scheme)
	})
	return err
}

// pointer returns a pointer to the given value.
func pointer[T any](v T) *T {
	return &v
}
