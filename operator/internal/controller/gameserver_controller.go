package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
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

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
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

	// AgentLogLevel, when non-empty, is injected into every agent sidecar
	// as GAMEPLANE_LOG_LEVEL. Empty injects nothing — the agent defaults
	// to info and existing StatefulSets don't roll on operator upgrade.
	AgentLogLevel string

	// ConfigInitImage is the image for the init container that copies
	// operator-rendered config files onto the data volume. Set from an
	// operator flag so air-gapped installs can point it at a private
	// registry mirror instead of Docker Hub. Empty falls back to
	// DefaultConfigInitImage.
	ConfigInitImage string

	// AgentCASecretName / AgentCASecretNamespace point at the cluster-
	// wide Secret holding `ca.crt` + `ca.key` used to sign the
	// per-GameServer agent server cert. Provisioned by the chart
	// (charts/gameplane/templates/mtls.yaml).
	AgentCASecretName      string
	AgentCASecretNamespace string

	// AgentClient runs the module-declared in-game stop sequence over RCON
	// during a soft stop. May be nil (or a disabled client) in dev clusters,
	// in which case the operator falls back to a timed scale-to-zero.
	AgentClient AgentStopper
}

// AgentStopper issues the module-declared graceful stop sequence to a game's
// agent. Satisfied by *operator/internal/agent.Client.
type AgentStopper interface {
	Stop(ctx context.Context, namespace, server string) error
}

const (
	// stopRequestedAtAnnotation records (RFC3339) when the operator issued the
	// in-game stop sequence, so the soft-stop wait survives reconciles and the
	// command is issued only once.
	stopRequestedAtAnnotation = "gameserver.gameplane.local/stop-requested-at"

	// defaultStopGracePeriod bounds the soft-stop wait when the GameServer
	// leaves spec.stopGracePeriodSeconds unset.
	defaultStopGracePeriod = 30 * time.Second
)

// RBAC markers below describe only the CLUSTER-wide permissions the
// operator needs. Writes to workload primitives (StatefulSets, Services,
// PVCs, Secrets, ConfigMaps, Jobs) are scoped per-namespace via a
// hand-managed Role bound in the games namespace(s) — see
// operator/config/rbac/role_namespace.yaml and the Helm chart. This
// keeps a compromised operator token from reading Secrets cluster-wide.
//
// +kubebuilder:rbac:groups=gameplane.local,resources=gameservers,verbs=get;list;watch
// +kubebuilder:rbac:groups=gameplane.local,resources=gameservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gameplane.local,resources=gameservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=gameplane.local,resources=gametemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=core,resources=services;persistentvolumeclaims;configmaps;secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods;pods/log,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

func (r *GameServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var gs gameplanev1alpha1.GameServer
	if err := r.Get(ctx, req.NamespacedName, &gs); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Resolve the template this GameServer points at. Templates are
	// cluster-scoped so no namespace is needed.
	var tmpl gameplanev1alpha1.GameTemplate
	if err := r.Get(ctx, types.NamespacedName{Name: gs.Spec.TemplateRef.Name}, &tmpl); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, r.setPhase(ctx, &gs, gameplanev1alpha1.GameServerPhaseFailed,
				fmt.Sprintf("GameTemplate %q not found", gs.Spec.TemplateRef.Name))
		}
		return ctrl.Result{}, err
	}

	// Resolve spec.config against the template's configSchema before
	// touching any children: invalid config must fail loudly (mirroring
	// the missing-template path) instead of materializing a pod that
	// silently ignores what the user asked for.
	mc, err := materializeConfig(&gs, &tmpl)
	if err != nil {
		return ctrl.Result{}, r.setPhase(ctx, &gs, gameplanev1alpha1.GameServerPhaseFailed,
			fmt.Sprintf("invalid config: %v", err))
	}

	// Resolve the selected game version (image/env + per-loader mod
	// volume). A spec.version that names no catalog entry fails loudly,
	// like an invalid config, rather than silently falling back.
	ver, err := resolveVersion(&gs, &tmpl)
	if err != nil {
		return ctrl.Result{}, r.setPhase(ctx, &gs, gameplanev1alpha1.GameServerPhaseFailed, err.Error())
	}

	if err := r.reconcilePVC(ctx, &gs, &tmpl); err != nil {
		logger.Error(err, "reconcile PVC")
		return ctrl.Result{}, err
	}
	if err := r.reconcileModPVC(ctx, &gs, &tmpl, ver); err != nil {
		logger.Error(err, "reconcile mod PVC")
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
	if err := r.reconcileConfigSecret(ctx, &gs, mc); err != nil {
		logger.Error(err, "reconcile config Secret")
		return ctrl.Result{}, err
	}
	if err := r.reconcileFilesSecret(ctx, &gs, mc); err != nil {
		logger.Error(err, "reconcile files Secret")
		return ctrl.Result{}, err
	}
	if err := r.reconcileRCONSecret(ctx, &gs, &tmpl); err != nil {
		logger.Error(err, "reconcile rcon Secret")
		return ctrl.Result{}, err
	}
	replicas, stopRequeue, err := r.desiredReplicas(ctx, &gs, &tmpl)
	if err != nil {
		logger.Error(err, "compute desired replicas")
		return ctrl.Result{}, err
	}
	if err := r.reconcileStatefulSet(ctx, &gs, &tmpl, ver, mc, replicas); err != nil {
		logger.Error(err, "reconcile StatefulSet")
		return ctrl.Result{}, err
	}
	// Node placement is a cosmetic annotation for the dashboard; a transient
	// pod-get hiccup shouldn't stall the rest of reconciliation, so log and go.
	if err := r.reconcileNodePlacement(ctx, &gs); err != nil {
		logger.Error(err, "reconcile node placement")
	}
	if err := r.reconcileBackupSchedule(ctx, &gs); err != nil {
		logger.Error(err, "reconcile BackupSchedule")
		return ctrl.Result{}, err
	}
	if err := r.reconcileWipe(ctx, &gs, &tmpl); err != nil {
		logger.Error(err, "reconcile data wipe")
		return ctrl.Result{}, err
	}

	requeue, err := r.reconcileStatus(ctx, &gs)
	if err != nil {
		return ctrl.Result{}, err
	}
	// While a soft stop is mid-flight, requeue at the (sooner) grace deadline
	// so we scale to zero even if no pod event arrives first.
	if stopRequeue > 0 && (requeue == 0 || stopRequeue < requeue) {
		requeue = stopRequeue
	}
	return ctrl.Result{RequeueAfter: requeue}, nil
}

func (r *GameServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gameplanev1alpha1.GameServer{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&gameplanev1alpha1.BackupSchedule{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Complete(r)
}

// --- sub-reconcilers (skeletons) ---

func (r *GameServerReconciler) reconcilePVC(
	ctx context.Context, gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate,
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
			// Seed the volume from a CSI VolumeSnapshot when requested
			// (this is how volume-snapshot Restores stand up a new server).
			// DataSource is immutable once the PVC binds, so it only ever
			// takes effect on this first-creation path. The copied storage
			// size is >= the snapshot's restoreSize by construction (the
			// snapshot came from a PVC of that size).
			if gs.Spec.Storage != nil && gs.Spec.Storage.DataSource != nil {
				apiGroup := "snapshot.storage.k8s.io"
				pvc.Spec.DataSource = &corev1.TypedLocalObjectReference{
					APIGroup: &apiGroup,
					Kind:     gs.Spec.Storage.DataSource.Kind,
					Name:     gs.Spec.Storage.DataSource.Name,
				}
			}
		}
		return controllerutil.SetControllerReference(gs, pvc, r.Scheme)
	})
	return err
}

func (r *GameServerReconciler) reconcileService(
	ctx context.Context, gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate,
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
		// loadBalancerSourceRanges is only valid on LoadBalancer Services;
		// clear it otherwise so a later Expose change doesn't leave a stale
		// (and rejected) allow-list behind.
		if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			svc.Spec.LoadBalancerSourceRanges = gs.Spec.Networking.SourceRanges
		} else {
			svc.Spec.LoadBalancerSourceRanges = nil
		}
		svc.Spec.Selector = map[string]string{
			"app.kubernetes.io/name":     "gameplane-game",
			"app.kubernetes.io/instance": gs.Name,
		}
		svc.Spec.Ports = svcPortsFromTemplate(tmpl, gs)
		applyManagedServiceAnnotations(svc, desiredServiceAnnotations(gs))
		return controllerutil.SetControllerReference(gs, svc, r.Scheme)
	})
	return err
}

// managedServiceAnnotationsKey records which annotation keys the operator
// applied from spec.networking.serviceAnnotations on the previous reconcile.
const managedServiceAnnotationsKey = "gameplane.local/managed-service-annotations"

// externalDNSHostnameAnnotation is the de-facto-standard external-dns key the
// operator stamps onto the game Service from spec.networking.hostname so an
// installed external-dns controller publishes the record. The operator does
// not create the DNS record itself (see GameServerNetworking.Hostname).
const externalDNSHostnameAnnotation = "external-dns.alpha.kubernetes.io/hostname"

// desiredServiceAnnotations is the full set of annotations the operator wants
// to manage on the game Service: the user's spec.networking.serviceAnnotations
// plus, when spec.networking.hostname is set, the external-dns hostname hint.
// Set unconditionally on hostname (not gated on Expose type) — external-dns
// decides what to publish from its own source config. The typed hostname field
// is applied last so it wins over a same-key entry in serviceAnnotations: it is
// the explicit, validated, UI-backed field and therefore authoritative.
func desiredServiceAnnotations(gs *gameplanev1alpha1.GameServer) map[string]string {
	desired := make(map[string]string, len(gs.Spec.Networking.ServiceAnnotations)+1)
	for k, v := range gs.Spec.Networking.ServiceAnnotations {
		desired[k] = v
	}
	if h := gs.Spec.Networking.Hostname; h != "" {
		desired[externalDNSHostnameAnnotation] = h
	}
	return desired
}

// applyManagedServiceAnnotations reconciles the user's desired
// serviceAnnotations onto svc so the Service converges when keys are removed
// from spec, without clobbering annotations written by other controllers
// (cloud load balancer, external-dns). It prunes keys the operator set last
// time but that are gone from spec now, applies the desired set, and records
// the managed keys in a sentinel annotation for the next pass. (Merging
// alone, as before, left removed annotations active on the Service.)
func applyManagedServiceAnnotations(svc *corev1.Service, desired map[string]string) {
	if svc.Annotations == nil {
		svc.Annotations = map[string]string{}
	}
	if prev := svc.Annotations[managedServiceAnnotationsKey]; prev != "" {
		for _, k := range strings.Split(prev, ",") {
			if _, keep := desired[k]; !keep {
				delete(svc.Annotations, k)
			}
		}
	}
	keys := make([]string, 0, len(desired))
	for k, v := range desired {
		svc.Annotations[k] = v
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		delete(svc.Annotations, managedServiceAnnotationsKey)
		return
	}
	sort.Strings(keys)
	svc.Annotations[managedServiceAnnotationsKey] = strings.Join(keys, ",")
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
	ctx context.Context, gs *gameplanev1alpha1.GameServer,
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
			"app.kubernetes.io/name":     "gameplane-game",
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
	tmpl *gameplanev1alpha1.GameTemplate, gs *gameplanev1alpha1.GameServer,
) []corev1.ServicePort {
	overrides := map[string]gameplanev1alpha1.PortOverride{}
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

// desiredReplicas decides the StatefulSet replica count. It brings the server
// down (gracefully, via the soft stop) on a spec.suspend or while a restart
// drains, and back up otherwise. A restart is an operator-owned scale-down →
// scale-up: the pod is recycled only once it is confirmed gone, so the request
// survives coalesced reconciles. The second return value is a requeue hint (>0
// while a soft stop or restart drain is in progress).
func (r *GameServerReconciler) desiredReplicas(
	ctx context.Context, gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate,
) (int32, time.Duration, error) {
	// A pending restart (stamped by the API) drives a transient scale-down →
	// scale-up entirely operator-side, so the request can't be lost to a
	// coalesced reconcile the way a client-issued suspend/resume pair can.
	restart, err := r.restartPhase(ctx, gs)
	if err != nil {
		return 1, 0, err
	}
	if restart == restartComplete {
		// The pod is gone (Status.Replicas == 0 / StatefulSet absent), so a
		// scale back to 1 yields a fresh pod identity. Ack so the same token
		// never re-runs, then return to the spec'd power state.
		if err := r.ackRestart(ctx, gs); err != nil {
			return 1, 0, err
		}
		if gs.Spec.Suspend {
			return 0, 0, nil // an explicit suspend outlives the restart
		}
		return 1, 0, nil
	}

	stopping := gs.Spec.Suspend || restart == restartDraining
	if !stopping {
		// Running: drop any stale soft-stop bookkeeping from a prior stop.
		return 1, 0, r.clearStopAnnotation(ctx, gs)
	}

	replicas, requeue, err := r.softStop(ctx, gs, tmpl)
	// While a restart drains, keep requeuing until the pod is actually gone —
	// the StatefulSet watch already wakes us on pod deletion; this is a backstop.
	if err == nil && restart == restartDraining && requeue == 0 {
		requeue = restartDrainPoll
	}
	return replicas, requeue, err
}

// softStop computes the replica count while a server is being brought down —
// via spec.suspend or a draining restart. It drives the module-declared
// graceful stop over the agent and holds the pod up while the game saves, then
// scales to zero once the game goes not-ready (or the grace deadline elapses).
// Templates with no stop sequence scale straight to zero.
func (r *GameServerReconciler) softStop(
	ctx context.Context, gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate,
) (int32, time.Duration, error) {
	declared := tmpl.Spec.Capabilities != nil &&
		tmpl.Spec.Capabilities.Lifecycle != nil &&
		len(tmpl.Spec.Capabilities.Lifecycle.Stop) > 0
	if !declared || r.AgentClient == nil {
		return 0, 0, nil // hard scale-down (no graceful stop available)
	}

	// Is the game still up? If the StatefulSet is gone or has no ready
	// replica, there's nothing to gracefully stop — finish the scale-down.
	var ss appsv1.StatefulSet
	switch err := r.Get(ctx, types.NamespacedName{Namespace: gs.Namespace, Name: gs.Name}, &ss); {
	case apierrors.IsNotFound(err):
		return 0, 0, nil
	case err != nil:
		return 0, 0, err
	}
	if ss.Status.ReadyReplicas == 0 {
		return 0, 0, nil
	}

	grace := defaultStopGracePeriod
	if gs.Spec.StopGracePeriodSeconds != nil {
		grace = time.Duration(*gs.Spec.StopGracePeriodSeconds) * time.Second
	}

	// First pass: stamp the start of the grace clock, then issue the stop
	// sequence. Stamping first means an update conflict retries cleanly
	// without re-issuing the command.
	if _, ok := gs.Annotations[stopRequestedAtAnnotation]; !ok {
		if err := r.setStopAnnotation(ctx, gs, time.Now().UTC().Format(time.RFC3339)); err != nil {
			return 1, 0, err
		}
		if err := r.AgentClient.Stop(ctx, gs.Namespace, gs.Name); err != nil {
			// Best-effort: a failed/unreachable agent must not wedge the stop.
			// The grace deadline (and readiness) still scale us down.
			log.FromContext(ctx).Info("soft stop: agent stop call failed; falling back to timed scale-down", "err", err)
		}
		return 1, grace, nil // keep running; requeue at the grace deadline
	}

	// Backstop: the game is still ready, so wait out the remaining grace
	// (readiness going to zero, handled above, scales us down sooner).
	requestedAt, perr := time.Parse(time.RFC3339, gs.Annotations[stopRequestedAtAnnotation])
	if perr != nil {
		return 0, 0, nil // unparseable stamp — don't hang, just scale down
	}
	if remaining := grace - time.Since(requestedAt); remaining > 0 {
		return 1, remaining, nil
	}
	return 0, 0, nil
}

func (r *GameServerReconciler) setStopAnnotation(ctx context.Context, gs *gameplanev1alpha1.GameServer, val string) error {
	if gs.Annotations == nil {
		gs.Annotations = map[string]string{}
	}
	gs.Annotations[stopRequestedAtAnnotation] = val
	return r.Update(ctx, gs)
}

func (r *GameServerReconciler) clearStopAnnotation(ctx context.Context, gs *gameplanev1alpha1.GameServer) error {
	if _, ok := gs.Annotations[stopRequestedAtAnnotation]; !ok {
		return nil
	}
	delete(gs.Annotations, stopRequestedAtAnnotation)
	return r.Update(ctx, gs)
}

func (r *GameServerReconciler) reconcileStatefulSet(
	ctx context.Context, gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate,
	ver *gameplanev1alpha1.GameVersion, mc *materializedConfig, replicas int32,
) error {
	image := resolveImage(gs, tmpl, ver)

	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: gs.Name, Namespace: gs.Namespace},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, ss, func() error {
		labels := map[string]string{
			"app.kubernetes.io/name":     "gameplane-game",
			"app.kubernetes.io/instance": gs.Name,
			"gameplane.local/template":   tmpl.Name,
		}
		ss.Spec.Replicas = &replicas
		ss.Spec.ServiceName = gs.Name
		ss.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		ss.Spec.Template.ObjectMeta.Labels = labels
		// Stamp (or clear) the config fingerprint without touching
		// annotations other actors may have set on the pod template.
		ann := ss.Spec.Template.ObjectMeta.Annotations
		if mc.hash != "" {
			if ann == nil {
				ann = map[string]string{}
			}
			ann[configHashAnnotation] = mc.hash
		} else {
			delete(ann, configHashAnnotation)
		}
		ss.Spec.Template.ObjectMeta.Annotations = ann
		ss.Spec.Template.Spec.Containers = []corev1.Container{
			buildGameContainer(gs, tmpl, image, ver, mc),
			buildAgentContainer(gs, tmpl, ver, r.AgentImage, r.AgentLogLevel),
		}
		volumes := []corev1.Volume{
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
		// Mount the resolved RCON password (operator-generated or the
		// template's referenced Secret) so the agent sidecar can read it
		// via --rcon-password-file. Added only when the game exposes RCON
		// and doesn't use a game-managed password file.
		if rc := resolveRCON(gs, tmpl); rc.enabled && rc.passwordFile == "" {
			volumes = append(volumes, corev1.Volume{
				Name: "rcon-password",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: rc.secretName,
						Items:      []corev1.KeyToPath{{Key: rc.secretKey, Path: "password"}},
					},
				},
			})
		}
		// Volumes and InitContainers are assigned wholesale so removing
		// configFiles from the template strips them on the next reconcile.
		if len(mc.files) > 0 {
			items := make([]corev1.KeyToPath, 0, len(mc.files))
			for _, f := range mc.files {
				items = append(items, corev1.KeyToPath{Key: f.key, Path: f.path})
			}
			volumes = append(volumes, corev1.Volume{
				Name: "config-files",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: filesSecretName(gs),
						Items:      items,
					},
				},
			})
			ss.Spec.Template.Spec.InitContainers = []corev1.Container{buildConfigInitContainer(r.ConfigInitImage, tmpl)}
		} else {
			ss.Spec.Template.Spec.InitContainers = nil
		}
		// Per-(version+loader) mod volume, nested at storage.mountPath/<path>
		// so the game image reads mods from its usual dir while they persist
		// on their own PVC. Assigned wholesale with the rest, so switching to
		// a loaderless version drops the mount on the next reconcile.
		if v := modVolume(gs, tmpl, ver); v != nil {
			volumes = append(volumes, *v)
		}
		// Mod-portal credential volumes for each configured provider. When a
		// provider's CredentialsSecretRef is set, its Secret is mounted
		// read-only at /etc/gameplane/mod-creds/<provider>/ so the agent can
		// inject credentials on install. Assigned wholesale so removing the
		// secret ref drops the mount on the next reconcile.
		volumes = append(volumes, modCredsVolumes(resolveModCreds(tmpl))...)
		ss.Spec.Template.Spec.Volumes = volumes
		// Assign unconditionally so clearing spec.nodeSelector also clears the
		// pod template's selector (nil resets it); the previous nil-guard left
		// a removed scheduling pin active on the StatefulSet.
		ss.Spec.Template.Spec.NodeSelector = gs.Spec.NodeSelector
		ss.Spec.Template.Spec.Tolerations = gs.Spec.Tolerations
		ss.Spec.Template.Spec.Affinity = gs.Spec.Affinity
		// Default to the per-GameServer SA so the agent's heartbeat can
		// patch gameservers/status (see reconcileAgentRBAC); an explicit
		// spec.serviceAccountName still wins.
		ss.Spec.Template.Spec.ServiceAccountName = agentServiceAccountName(gs)
		if gs.Spec.ServiceAccountName != "" {
			ss.Spec.Template.Spec.ServiceAccountName = gs.Spec.ServiceAccountName
		}
		// Share the pod's PID namespace so the agent sidecar can read the
		// game process's CPU/memory from /proc. cgroup v2 files are
		// per-container, so without this the agent only sees its own
		// (idle) usage. The pod stays non-privileged; /proc/<pid>/stat and
		// /statm are world-readable, so no extra capabilities are needed.
		shareProcess := true
		ss.Spec.Template.Spec.ShareProcessNamespace = &shareProcess
		return controllerutil.SetControllerReference(gs, ss, r.Scheme)
	})
	return err
}

// effectiveMountPath is where the game's data volume is mounted,
// defaulting to /data when the template doesn't say.
func effectiveMountPath(tmpl *gameplanev1alpha1.GameTemplate) string {
	if tmpl.Spec.Storage.MountPath != "" {
		return tmpl.Spec.Storage.MountPath
	}
	return "/data"
}

// configFilesStagingPath is where the `<gs>-files` Secret is mounted
// inside the config-init container before being copied onto the data
// volume.
const configFilesStagingPath = "/etc/gameplane/config-files"

// DefaultConfigInitImage is the small shell image the operator uses for the
// utility containers it injects into game workloads: the config-init container
// that seeds rendered config files, and the wipe Job that clears a data volume.
// Pinned like the restic image in backup_controller.go; the agent image can't do
// either job (distroless, no shell or cp). Overridable via the operator's
// --config-init-image flag for air-gapped installs.
const DefaultConfigInitImage = "busybox:1.37.0"

// configInitImageOrDefault resolves the configured shell image, falling back to
// the pin when the operator wasn't given a --config-init-image.
func configInitImageOrDefault(image string) string {
	if image == "" {
		return DefaultConfigInitImage
	}
	return image
}

// buildConfigInitContainer copies the rendered config files onto the
// data volume on every pod start — operator-rendered files always win
// over in-place edits (e.g. via the dashboard Files tab). image is the
// operator-configured config-init image; empty falls back to the pin.
func buildConfigInitContainer(image string, tmpl *gameplanev1alpha1.GameTemplate) corev1.Container {
	image = configInitImageOrDefault(image)
	mountPath := effectiveMountPath(tmpl)
	return corev1.Container{
		Name:    "config-init",
		Image:   image,
		Command: []string{"/bin/sh", "-c"},
		// -L dereferences the kubelet's per-key symlinks; the * glob
		// skips the ..data/..<timestamp> dot-entries of the Secret mount.
		Args: []string{"cp -RL " + configFilesStagingPath + "/* '" + mountPath + "/'"},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "config-files", MountPath: configFilesStagingPath, ReadOnly: true},
			{Name: "data", MountPath: mountPath},
		},
	}
}

// effectiveResources resolves the game container's compute resources:
// the template's defaults, replaced wholesale by spec.resources when set.
func effectiveResources(
	gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate,
) corev1.ResourceRequirements {
	if gs.Spec.Resources != nil {
		return *gs.Spec.Resources
	}
	return tmpl.Spec.Resources
}

func buildGameContainer(
	gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate, image string,
	ver *gameplanev1alpha1.GameVersion, mc *materializedConfig,
) corev1.Container {
	mountPath := effectiveMountPath(tmpl)
	// expose: Hostport binds each container port directly on the node so the
	// game is reachable at <node>:<port> without a NodePort/LoadBalancer
	// Service (the Service stays ClusterIP). Suits single-node k3s/homelab
	// installs where one pod owns the host port.
	hostPort := gs.Spec.Networking.Expose == "Hostport"
	ports := make([]corev1.ContainerPort, 0, len(tmpl.Spec.Ports))
	for _, p := range tmpl.Spec.Ports {
		cp := corev1.ContainerPort{Name: p.Name, ContainerPort: p.ContainerPort, Protocol: p.Protocol}
		if hostPort {
			cp.HostPort = p.ContainerPort
		}
		ports = append(ports, cp)
	}
	// Later entries win on duplicate names: template defaults, then the
	// selected version's env (e.g. itzg TYPE/VERSION), then schema-resolved
	// config, then explicit spec.env overrides.
	env := append([]corev1.EnvVar{}, tmpl.Spec.Env...)
	if ver != nil {
		env = append(env, ver.Env...)
	}
	env = append(env, mc.env...)
	env = append(env, gs.Spec.Env...)
	// The operator-managed RCON password wins, so the game and the agent
	// sidecar always agree on it.
	if e := rconGameEnv(gs, tmpl); e != nil {
		env = append(env, *e)
	}

	res := effectiveResources(gs, tmpl)

	mounts := []corev1.VolumeMount{{Name: "data", MountPath: mountPath}}
	if m := modVolumeMount(tmpl, ver); m != nil {
		mounts = append(mounts, *m)
	}

	c := corev1.Container{
		Name:         gameContainerName,
		Image:        image,
		Command:      tmpl.Spec.Command,
		Args:         tmpl.Spec.Args,
		Env:          env,
		Ports:        ports,
		VolumeMounts: mounts,
		Resources:    res,
	}
	if tmpl.Spec.Probes != nil {
		c.ReadinessProbe = tmpl.Spec.Probes.Readiness
		c.LivenessProbe = tmpl.Spec.Probes.Liveness
		c.StartupProbe = tmpl.Spec.Probes.Startup
	}
	// Per-server probe overrides win over the template, one probe at a time.
	if p := gs.Spec.Probes; p != nil {
		if p.Readiness != nil {
			c.ReadinessProbe = p.Readiness
		}
		if p.Liveness != nil {
			c.LivenessProbe = p.Liveness
		}
		if p.Startup != nil {
			c.StartupProbe = p.Startup
		}
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
	gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate,
	ver *gameplanev1alpha1.GameVersion, fallbackImage, logLevel string,
) corev1.Container {
	image := fallbackImage
	res := corev1.ResourceRequirements{}
	if tmpl.Spec.Agent != nil {
		if tmpl.Spec.Agent.Image != "" {
			image = tmpl.Spec.Agent.Image
		}
		res = tmpl.Spec.Agent.Resources
	}
	mountPath := effectiveMountPath(tmpl)
	nonRoot := true
	roRootFS := true
	noPrivEsc := false
	uid := int64(65532)
	args := []string{
		"--tls-cert=/etc/gameplane/agent-tls/tls.crt",
		"--tls-key=/etc/gameplane/agent-tls/tls.key",
		"--tls-client-ca=/etc/gameplane/agent-tls/ca.crt",
	}
	if tmpl.Spec.LogPath != "" {
		args = append(args, "--game-log-path="+tmpl.Spec.LogPath)
	}
	rc := resolveRCON(gs, tmpl)
	if rc.enabled {
		if rc.passwordFile != "" {
			args = append(args, "--rcon-password-file="+path.Join(mountPath, rc.passwordFile))
		} else {
			args = append(args, "--rcon-password-file="+rconPasswordPath+"/password")
		}
		args = append(args, "--rcon-port="+strconv.FormatInt(int64(rc.port), 10))
	}
	env := []corev1.EnvVar{
		{Name: "GAMEPLANE_SERVER_NAME", Value: gs.Name},
		{Name: "GAMEPLANE_TEMPLATE", Value: tmpl.Name},
		{Name: "GAMEPLANE_GAME", Value: tmpl.Spec.Game},
		// Games without RCON (consoleMode pty/none) must not have the
		// agent dialing a console port that doesn't exist — players
		// and moderation endpoints degrade instead.
		{Name: "GAMEPLANE_RCON_ENABLED", Value: strconv.FormatBool(templateHasRCON(tmpl))},
		// The pod shares its PID namespace (ShareProcessNamespace), so the
		// agent reports the GAME process's CPU/memory from /proc rather than
		// its own per-container cgroup (which shows only the idle sidecar).
		{Name: "GAMEPLANE_USAGE_PROC", Value: "1"},
	}
	// Only when explicitly configured — the env change rolls every game
	// StatefulSet, so an unset flag must not differ from the old pod spec.
	if logLevel != "" {
		env = append(env, corev1.EnvVar{Name: "GAMEPLANE_LOG_LEVEL", Value: logLevel})
	}
	// In proc mode the agent can't read the game container's cgroup limit, so
	// pass the resolved limits through as the denominator for the dashboard's
	// usage bars. Mirrors buildGameContainer's resource resolution.
	gameRes := effectiveResources(gs, tmpl)
	if cpu := gameRes.Limits.Cpu(); cpu != nil && !cpu.IsZero() {
		env = append(env, corev1.EnvVar{
			Name: "GAMEPLANE_CPU_LIMIT_MILLICORES", Value: strconv.FormatInt(cpu.MilliValue(), 10),
		})
	}
	if mem := gameRes.Limits.Memory(); mem != nil && !mem.IsZero() {
		env = append(env, corev1.EnvVar{
			Name: "GAMEPLANE_MEM_LIMIT_BYTES", Value: strconv.FormatInt(mem.Value(), 10),
		})
	}
	// Declared capability commands travel to the agent as one JSON blob;
	// the env change rolls the StatefulSet, so capability edits apply on the
	// next pod rollout like every other template change. resolveCapabilities
	// collapses the per-loader mods map into the active version's concrete
	// Mods.Path, so the agent stays loader-agnostic (no agent code change).
	if caps := resolveCapabilities(tmpl, ver); caps != nil {
		if b, err := json.Marshal(caps); err == nil {
			env = append(env, corev1.EnvVar{Name: "GAMEPLANE_CAPABILITIES", Value: string(b)})
		}
	}
	return corev1.Container{
		Name:         "agent",
		Image:        image,
		Args:         args,
		Env:          env,
		VolumeMounts: agentVolumeMounts(gs, tmpl, ver, mountPath),
		Ports:        []corev1.ContainerPort{{Name: "agent", ContainerPort: 8090}},
		Resources:    res,
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
	ctx context.Context, gs *gameplanev1alpha1.GameServer,
) error {
	name := gs.Name + "-auto"
	bs := &gameplanev1alpha1.BackupSchedule{
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
		bs.Spec.ServerRef = gameplanev1alpha1.LocalObjectRef{Name: gs.Name}
		bs.Spec.Schedule = gs.Spec.BackupPolicy.Schedule
		bs.Spec.RepoRef = &gs.Spec.BackupPolicy.RepoRef
		bs.Spec.Retention = gs.Spec.BackupPolicy.Retention
		bs.Spec.Suspend = gs.Spec.BackupPolicy.Suspend
		return controllerutil.SetControllerReference(gs, bs, r.Scheme)
	})
	return err
}

func (r *GameServerReconciler) setPhase(
	ctx context.Context, gs *gameplanev1alpha1.GameServer, phase gameplanev1alpha1.GameServerPhase, msg string,
) error {
	// Patch (not Update) so we don't carry/revert the agent's concurrently
	// written status.agent — see reconcileStatus for the full rationale.
	base := gs.DeepCopy()
	gs.Status.Phase = phase
	gs.Status.Conditions = upsertCondition(gs.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionFalse,
		Reason:  string(phase),
		Message: msg,
	})
	return r.Status().Patch(ctx, gs, client.MergeFrom(base))
}
