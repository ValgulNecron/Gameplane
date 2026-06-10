package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
)

// GameServerReconciler reconciles a GameServer object into a StatefulSet,
// Service, and PVC. The agent sidecar is injected at pod-spec build time.
type GameServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// AgentImage is the container image used for the sidecar agent
	// injected into every game pod. Set from an operator flag so the
	// deployer can pin the agent version independently of the game.
	AgentImage string

	// AgentCASecretName / AgentCASecretNamespace point at the cluster-
	// wide Secret holding `ca.crt` + `ca.key` used to sign the
	// per-GameServer agent server cert. Provisioned by the chart
	// (charts/kestrel/templates/mtls.yaml).
	AgentCASecretName      string
	AgentCASecretNamespace string
}

// RBAC markers below describe only the CLUSTER-wide permissions the
// operator needs. Writes to workload primitives (StatefulSets, Services,
// PVCs, Secrets, ConfigMaps, Jobs) are scoped per-namespace via a
// hand-managed Role bound in the games namespace(s) — see
// operator/config/rbac/role_namespace.yaml and the Helm chart. This
// keeps a compromised operator token from reading Secrets cluster-wide.
//
// +kubebuilder:rbac:groups=kestrel.gg,resources=gameservers,verbs=get;list;watch
// +kubebuilder:rbac:groups=kestrel.gg,resources=gameservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kestrel.gg,resources=gameservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=kestrel.gg,resources=gametemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=services;persistentvolumeclaims;configmaps;secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods;pods/log,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

func (r *GameServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var gs kestrelv1alpha1.GameServer
	if err := r.Get(ctx, req.NamespacedName, &gs); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Resolve the template this GameServer points at. Templates are
	// cluster-scoped so no namespace is needed.
	var tmpl kestrelv1alpha1.GameTemplate
	if err := r.Get(ctx, types.NamespacedName{Name: gs.Spec.TemplateRef.Name}, &tmpl); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, r.setPhase(ctx, &gs, kestrelv1alpha1.GameServerPhaseFailed,
				fmt.Sprintf("GameTemplate %q not found", gs.Spec.TemplateRef.Name))
		}
		return ctrl.Result{}, err
	}

	if err := r.reconcilePVC(ctx, &gs, &tmpl); err != nil {
		logger.Error(err, "reconcile PVC")
		return ctrl.Result{}, err
	}
	if err := r.reconcileService(ctx, &gs, &tmpl); err != nil {
		logger.Error(err, "reconcile Service")
		return ctrl.Result{}, err
	}
	if err := r.reconcileAgentService(ctx, &gs); err != nil {
		logger.Error(err, "reconcile agent Service")
		return ctrl.Result{}, err
	}
	if err := r.reconcileAgentTLS(ctx, &gs); err != nil {
		logger.Error(err, "reconcile agent TLS")
		return ctrl.Result{}, err
	}
	if err := r.reconcileAgentRBAC(ctx, &gs); err != nil {
		logger.Error(err, "reconcile agent RBAC")
		return ctrl.Result{}, err
	}
	if err := r.reconcileStatefulSet(ctx, &gs, &tmpl); err != nil {
		logger.Error(err, "reconcile StatefulSet")
		return ctrl.Result{}, err
	}
	if err := r.reconcileBackupSchedule(ctx, &gs); err != nil {
		logger.Error(err, "reconcile BackupSchedule")
		return ctrl.Result{}, err
	}

	requeue, err := r.reconcileStatus(ctx, &gs)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: requeue}, nil
}

func (r *GameServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kestrelv1alpha1.GameServer{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&kestrelv1alpha1.BackupSchedule{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Complete(r)
}

// --- sub-reconcilers (skeletons) ---

func (r *GameServerReconciler) reconcilePVC(
	ctx context.Context, gs *kestrelv1alpha1.GameServer, tmpl *kestrelv1alpha1.GameTemplate,
) error {
	size := resource.MustParse("10Gi")
	if gs.Spec.Storage != nil && !gs.Spec.Storage.Size.IsZero() {
		size = gs.Spec.Storage.Size
	} else if !tmpl.Spec.Storage.Size.IsZero() {
		size = tmpl.Spec.Storage.Size
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: gs.Name + "-data", Namespace: gs.Namespace},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, pvc, func() error {
		if pvc.CreationTimestamp.IsZero() {
			pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
			pvc.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: size}
			if gs.Spec.Storage != nil && gs.Spec.Storage.StorageClassName != nil {
				pvc.Spec.StorageClassName = gs.Spec.Storage.StorageClassName
			} else if tmpl.Spec.Storage.StorageClassName != nil {
				pvc.Spec.StorageClassName = tmpl.Spec.Storage.StorageClassName
			}
		}
		return controllerutil.SetControllerReference(gs, pvc, r.Scheme)
	})
	return err
}

func (r *GameServerReconciler) reconcileService(
	ctx context.Context, gs *kestrelv1alpha1.GameServer, tmpl *kestrelv1alpha1.GameTemplate,
) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: gs.Name, Namespace: gs.Namespace},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec.Type = corev1.ServiceTypeClusterIP
		switch gs.Spec.Networking.Expose {
		case "NodePort":
			svc.Spec.Type = corev1.ServiceTypeNodePort
		case "LoadBalancer":
			svc.Spec.Type = corev1.ServiceTypeLoadBalancer
		}
		svc.Spec.Selector = map[string]string{
			"app.kubernetes.io/name":     "kestrel-game",
			"app.kubernetes.io/instance": gs.Name,
		}
		svc.Spec.Ports = svcPortsFromTemplate(tmpl, gs)
		if svc.Annotations == nil {
			svc.Annotations = map[string]string{}
		}
		for k, v := range gs.Spec.Networking.ServiceAnnotations {
			svc.Annotations[k] = v
		}
		return controllerutil.SetControllerReference(gs, svc, r.Scheme)
	})
	return err
}

// reconcileAgentService maintains a dedicated ClusterIP Service
// (`<gs>-agent`) fronting the agent sidecar on port 8090. The game's
// own Service follows spec.networking.expose and may be NodePort or
// LoadBalancer; the agent must never ride along on an externally
// exposed Service, so it gets its own, always cluster-internal one.
// The API and operator dial the agent through this Service
// (api/internal/ws/dialer.go, operator/internal/agent/client.go) —
// per-pod DNS only resolves under headless Services, which the game
// Service is not.
func (r *GameServerReconciler) reconcileAgentService(
	ctx context.Context, gs *kestrelv1alpha1.GameServer,
) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: gs.Name + "-agent", Namespace: gs.Namespace},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec.Type = corev1.ServiceTypeClusterIP
		// The agent must be reachable while the game container is still
		// starting (console/files/logs during long world generation), so
		// don't gate endpoints on whole-pod readiness — the game's
		// readiness probe would otherwise hold the agent hostage.
		svc.Spec.PublishNotReadyAddresses = true
		svc.Spec.Selector = map[string]string{
			"app.kubernetes.io/name":     "kestrel-game",
			"app.kubernetes.io/instance": gs.Name,
		}
		svc.Spec.Ports = []corev1.ServicePort{{
			Name:       "agent",
			Port:       8090,
			TargetPort: intstr.FromInt32(8090),
			Protocol:   corev1.ProtocolTCP,
		}}
		return controllerutil.SetControllerReference(gs, svc, r.Scheme)
	})
	return err
}

func svcPortsFromTemplate(
	tmpl *kestrelv1alpha1.GameTemplate, gs *kestrelv1alpha1.GameServer,
) []corev1.ServicePort {
	overrides := map[string]kestrelv1alpha1.PortOverride{}
	for _, o := range gs.Spec.Networking.PortOverrides {
		overrides[o.Name] = o
	}
	out := make([]corev1.ServicePort, 0, len(tmpl.Spec.Ports))
	for _, p := range tmpl.Spec.Ports {
		if !p.Advertise {
			continue
		}
		port := p.ContainerPort
		nodePort := int32(0)
		if o, ok := overrides[p.Name]; ok {
			if o.ServicePort != 0 {
				port = o.ServicePort
			}
			nodePort = o.NodePort
		}
		sp := corev1.ServicePort{
			Name:       p.Name,
			Port:       port,
			TargetPort: intstr.FromInt32(p.ContainerPort),
			Protocol:   p.Protocol,
			NodePort:   nodePort,
		}
		if sp.Protocol == "" {
			sp.Protocol = corev1.ProtocolTCP
		}
		out = append(out, sp)
	}
	return out
}

func (r *GameServerReconciler) reconcileStatefulSet(
	ctx context.Context, gs *kestrelv1alpha1.GameServer, tmpl *kestrelv1alpha1.GameTemplate,
) error {
	replicas := int32(1)
	if gs.Spec.Suspend {
		replicas = 0
	}
	image := tmpl.Spec.Image
	if gs.Spec.Image != "" {
		image = gs.Spec.Image
	}

	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: gs.Name, Namespace: gs.Namespace},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, ss, func() error {
		labels := map[string]string{
			"app.kubernetes.io/name":     "kestrel-game",
			"app.kubernetes.io/instance": gs.Name,
			"kestrel.gg/template":        tmpl.Name,
		}
		ss.Spec.Replicas = &replicas
		ss.Spec.ServiceName = gs.Name
		ss.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		ss.Spec.Template.ObjectMeta.Labels = labels
		ss.Spec.Template.Spec.Containers = []corev1.Container{
			buildGameContainer(gs, tmpl, image),
			buildAgentContainer(gs, tmpl, r.AgentImage),
		}
		ss.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: gs.Name + "-data",
					},
				},
			},
			{
				// Per-GameServer Secret with tls.crt, tls.key, ca.crt.
				// Reconciled by reconcileAgentTLS before this StatefulSet.
				Name: "agent-tls",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: agentTLSSecretName(gs),
					},
				},
			},
		}
		if gs.Spec.NodeSelector != nil {
			ss.Spec.Template.Spec.NodeSelector = gs.Spec.NodeSelector
		}
		ss.Spec.Template.Spec.Tolerations = gs.Spec.Tolerations
		ss.Spec.Template.Spec.Affinity = gs.Spec.Affinity
		// Default to the per-GameServer SA so the agent's heartbeat can
		// patch gameservers/status (see reconcileAgentRBAC); an explicit
		// spec.serviceAccountName still wins.
		ss.Spec.Template.Spec.ServiceAccountName = agentServiceAccountName(gs)
		if gs.Spec.ServiceAccountName != "" {
			ss.Spec.Template.Spec.ServiceAccountName = gs.Spec.ServiceAccountName
		}
		return controllerutil.SetControllerReference(gs, ss, r.Scheme)
	})
	return err
}

func buildGameContainer(
	gs *kestrelv1alpha1.GameServer, tmpl *kestrelv1alpha1.GameTemplate, image string,
) corev1.Container {
	mountPath := "/data"
	if tmpl.Spec.Storage.MountPath != "" {
		mountPath = tmpl.Spec.Storage.MountPath
	}
	ports := make([]corev1.ContainerPort, 0, len(tmpl.Spec.Ports))
	for _, p := range tmpl.Spec.Ports {
		ports = append(ports, corev1.ContainerPort{
			Name: p.Name, ContainerPort: p.ContainerPort, Protocol: p.Protocol,
		})
	}
	env := append([]corev1.EnvVar{}, tmpl.Spec.Env...)
	env = append(env, gs.Spec.Env...)

	res := tmpl.Spec.Resources
	if gs.Spec.Resources != nil {
		res = *gs.Spec.Resources
	}

	c := corev1.Container{
		Name:         "game",
		Image:        image,
		Command:      tmpl.Spec.Command,
		Args:         tmpl.Spec.Args,
		Env:          env,
		Ports:        ports,
		VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: mountPath}},
		Resources:    res,
	}
	if tmpl.Spec.Probes != nil {
		c.ReadinessProbe = tmpl.Spec.Probes.Readiness
		c.LivenessProbe = tmpl.Spec.Probes.Liveness
		c.StartupProbe = tmpl.Spec.Probes.Startup
	}
	// PTY console mode requires the kubelet to allocate a TTY for the
	// container at start time. These fields are immutable once the pod
	// exists, so changing ConsoleMode forces a pod recreate (handled by
	// StatefulSet's normal rollout when the template hash changes).
	if EffectiveConsoleMode(tmpl) == "pty" {
		c.TTY = true
		c.Stdin = true
	}
	return c
}

func buildAgentContainer(
	gs *kestrelv1alpha1.GameServer, tmpl *kestrelv1alpha1.GameTemplate, fallbackImage string,
) corev1.Container {
	image := fallbackImage
	res := corev1.ResourceRequirements{}
	if tmpl.Spec.Agent != nil {
		if tmpl.Spec.Agent.Image != "" {
			image = tmpl.Spec.Agent.Image
		}
		res = tmpl.Spec.Agent.Resources
	}
	mountPath := "/data"
	if tmpl.Spec.Storage.MountPath != "" {
		mountPath = tmpl.Spec.Storage.MountPath
	}
	nonRoot := true
	roRootFS := true
	noPrivEsc := false
	uid := int64(65532)
	return corev1.Container{
		Name:  "agent",
		Image: image,
		Args: []string{
			"--tls-cert=/etc/kestrel/agent-tls/tls.crt",
			"--tls-key=/etc/kestrel/agent-tls/tls.key",
			"--tls-client-ca=/etc/kestrel/agent-tls/ca.crt",
		},
		Env: []corev1.EnvVar{
			{Name: "KESTREL_SERVER_NAME", Value: gs.Name},
			{Name: "KESTREL_TEMPLATE", Value: tmpl.Name},
			{Name: "KESTREL_GAME", Value: tmpl.Spec.Game},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "data", MountPath: mountPath},
			{Name: "agent-tls", MountPath: "/etc/kestrel/agent-tls", ReadOnly: true},
		},
		Ports:     []corev1.ContainerPort{{Name: "agent", ContainerPort: 8090}},
		Resources: res,
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             &nonRoot,
			RunAsUser:                &uid,
			ReadOnlyRootFilesystem:   &roRootFS,
			AllowPrivilegeEscalation: &noPrivEsc,
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
			SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		},
	}
}

func (r *GameServerReconciler) reconcileBackupSchedule(
	ctx context.Context, gs *kestrelv1alpha1.GameServer,
) error {
	name := gs.Name + "-auto"
	bs := &kestrelv1alpha1.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: gs.Namespace},
	}

	if gs.Spec.BackupPolicy == nil {
		err := r.Delete(ctx, bs)
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, bs, func() error {
		bs.Spec.ServerRef = kestrelv1alpha1.LocalObjectRef{Name: gs.Name}
		bs.Spec.Schedule = gs.Spec.BackupPolicy.Schedule
		bs.Spec.RepoRef = gs.Spec.BackupPolicy.RepoRef
		bs.Spec.Retention = gs.Spec.BackupPolicy.Retention
		bs.Spec.Suspend = gs.Spec.BackupPolicy.Suspend
		return controllerutil.SetControllerReference(gs, bs, r.Scheme)
	})
	return err
}

func (r *GameServerReconciler) setPhase(
	ctx context.Context, gs *kestrelv1alpha1.GameServer, phase kestrelv1alpha1.GameServerPhase, msg string,
) error {
	gs.Status.Phase = phase
	gs.Status.Conditions = upsertCondition(gs.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionFalse,
		Reason:  string(phase),
		Message: msg,
	})
	return r.Status().Update(ctx, gs)
}
