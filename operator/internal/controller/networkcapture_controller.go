package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

// SidecarCaptureClient abstracts the sidecar's :9091 HTTP control endpoint.
// Used by the reconciler to start/stop/poll captures. Tests inject a stub.
type SidecarCaptureClient interface {
	// StartCapture instructs the sidecar to begin capturing.
	// Returns error on failure.
	StartCapture(ctx context.Context, namespace, serverName, captureID string, filter *string, maxDurationSeconds, maxSizeBytes int64) error

	// StopCapture instructs the sidecar to stop an active capture.
	StopCapture(ctx context.Context, namespace, serverName, captureID string) error

	// GetCaptureStatus polls the sidecar for a capture's current status.
	// Returns the phase, packets written, bytes written, message, and error.
	// A non-nil error also signals "the sidecar has no record of this capture ID"
	// (e.g. it was never started, or the sidecar restarted) — the reconciler
	// relies on this to decide whether StartCapture still needs to be called.
	GetCaptureStatus(ctx context.Context, namespace, serverName, captureID string) (phase string, packetsWritten int64, bytesWritten int64, message string, err error)
}

// SidecarStoppedCondition marks that the reconciler has already told the
// sidecar to stop capturing for this NetworkCapture, so a Completed phase
// set from outside (the API's user-requested stop) triggers exactly one
// StopCapture call rather than one per reconcile.
const SidecarStoppedCondition = "SidecarStopped"

// userStoppedMessage is the status.message the API's StopNetworkCapture
// writes (api/internal/kube/capture.go) when a user stops a capture early.
// The reconciler matches on this exact string to distinguish a user-driven
// stop (sidecar still capturing, needs to be told to stop) from a capture
// that finished on its own (sidecar already stopped itself). There is no
// spec-level "stop requested" field to reconcile against instead — see the
// accepted design in research.md, "Capture lifecycle".
const userStoppedMessage = "stopped by user request"

// NetworkCaptureReconciler drives a NetworkCapture's lifecycle: Pending → Running → Completed/Failed.
// It injects the capture sidecar as an ephemeral container, calls the sidecar's control endpoint
// over mTLS through the <gs>-agent Service, and monitors completion.
type NetworkCaptureReconciler struct {
	client.Client
	Scheme                           *runtime.Scheme
	SidecarClient                    SidecarCaptureClient
	CaptureEnabled                   bool
	CaptureSidecarImage              string
	CaptureDefaultMaxDurationSeconds int64
	CaptureDefaultMaxSizeBytes       int64
}

// +kubebuilder:rbac:groups=gameplane.local,resources=networkcaptures,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gameplane.local,resources=networkcaptures/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gameplane.local,resources=gameservers,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gameplane.local,resources=gameservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/ephemeralcontainers,verbs=get;list;watch;patch;update

func (r *NetworkCaptureReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var nc gameplanev1alpha1.NetworkCapture
	if err := r.Get(ctx, req.NamespacedName, &nc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// The API's StopNetworkCapture (api/internal/kube/capture.go) sets
	// phase=Completed directly via a status patch — there is no reconciler
	// step in between. Before treating that as terminal, make sure the
	// sidecar is actually told to stop; otherwise it keeps writing packets
	// until it hits its own max-duration/max-size limit. Guarded by a
	// condition so this fires exactly once per user-requested stop.
	if nc.Status.Phase == gameplanev1alpha1.CapturePhaseCompleted &&
		nc.Status.Message == userStoppedMessage &&
		!meta.IsStatusConditionTrue(nc.Status.Conditions, SidecarStoppedCondition) {
		if err := r.SidecarClient.StopCapture(ctx, nc.Namespace, nc.Spec.ServerRef.Name, nc.Name); err != nil {
			return ctrl.Result{}, fmt.Errorf("stop capture %s on sidecar: %w", nc.Name, err)
		}
		meta.SetStatusCondition(&nc.Status.Conditions, metav1.Condition{
			Type:               SidecarStoppedCondition,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: nc.Generation,
			Reason:             "stopped",
			Message:            "sidecar told to stop capturing",
			LastTransitionTime: metav1.Now(),
		})
		if err := r.Status().Update(ctx, &nc); err != nil {
			return ctrl.Result{}, fmt.Errorf("record sidecar stop for capture %s: %w", nc.Name, err)
		}
		return ctrl.Result{}, nil
	}

	// Terminal phases need no further reconciliation.
	if nc.Status.Phase == gameplanev1alpha1.CapturePhaseCompleted ||
		nc.Status.Phase == gameplanev1alpha1.CapturePhaseFailed ||
		nc.Status.Phase == gameplanev1alpha1.CapturePhaseExpired {
		return ctrl.Result{}, nil
	}

	// Verify the target GameServer exists.
	var gs gameplanev1alpha1.GameServer
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: nc.Namespace,
		Name:      nc.Spec.ServerRef.Name,
	}, &gs); err != nil {
		if apierrors.IsNotFound(err) {
			return r.fail(ctx, &nc, fmt.Sprintf("gameserver %s not found: %v", nc.Spec.ServerRef.Name, err))
		}
		return ctrl.Result{}, fmt.Errorf("get gameserver %s: %w", nc.Spec.ServerRef.Name, err)
	}

	// Backfill the owner reference so this NetworkCapture is garbage-collected
	// with its GameServer. The API's CreateNetworkCapture (api/internal/kube)
	// only has the dynamic client and a bare server name at creation time, so
	// this reconciler — which already has the live GameServer object — is
	// where the reference actually gets set.
	if err := r.ensureOwnerReference(ctx, &nc, &gs); err != nil {
		return ctrl.Result{}, fmt.Errorf("set owner reference on capture %s: %w", nc.Name, err)
	}

	// Verify capture is enabled on the server.
	if gs.Spec.Capture == nil || !gs.Spec.Capture.Enabled {
		return r.fail(ctx, &nc, "capture not enabled on gameserver")
	}

	// The status subresource discards any status set in the same call as
	// object creation, regardless of which client created the CR (typed or
	// dynamic/unstructured) — so a freshly-created NetworkCapture always
	// lands with status.phase == "" no matter what the creator intended.
	// Treat that the same as Pending so it falls into the branch below
	// instead of the no-op default case at the bottom of this function.
	if nc.Status.Phase == "" {
		nc.Status.Phase = gameplanev1alpha1.CapturePhasePending
	}

	// Pending: transition to Running if no other capture is Running.
	if nc.Status.Phase == gameplanev1alpha1.CapturePhasePending {
		// Cluster-wide kill switch: refuse to start any capture when the
		// operator was started with --capture-enabled=false.
		if !r.CaptureEnabled {
			return r.fail(ctx, &nc, "network capture disabled on this operator")
		}

		// Resolve the max duration/size to enforce. Both fields are
		// +kubebuilder:validation:Required on the CRD, so the apiserver
		// should never persist a NetworkCapture without them — but a CRD
		// applied out of sync with an older revision could still let a nil
		// through, and Duration.Seconds()/Quantity.Value() would panic the
		// whole manager rather than just this reconcile. Fall back to the
		// operator's configured defaults and fail cleanly if neither is set.
		maxDurationSeconds := r.CaptureDefaultMaxDurationSeconds
		if nc.Spec.MaxDuration != nil {
			maxDurationSeconds = int64(nc.Spec.MaxDuration.Seconds())
		}
		maxSizeBytes := r.CaptureDefaultMaxSizeBytes
		if nc.Spec.MaxSize != nil {
			maxSizeBytes = nc.Spec.MaxSize.Value()
		}
		if maxDurationSeconds <= 0 || maxSizeBytes <= 0 {
			return r.fail(ctx, &nc, "capture has no maxDuration/maxSize and the operator has no configured default")
		}

		// Check for concurrent Running captures.
		var captures gameplanev1alpha1.NetworkCaptureList
		if err := r.List(ctx, &captures, client.InNamespace(nc.Namespace)); err != nil {
			return ctrl.Result{}, fmt.Errorf("list captures for concurrency check: %w", err)
		}

		for _, other := range captures.Items {
			if other.Spec.ServerRef.Name == nc.Spec.ServerRef.Name &&
				other.Status.Phase == gameplanev1alpha1.CapturePhaseRunning &&
				other.Name != nc.Name {
				return r.fail(ctx, &nc, "another capture is already running on this gameserver")
			}
		}

		// Inject ephemeral container if not already present.
		var pod corev1.Pod
		if err := r.Get(ctx, types.NamespacedName{
			Namespace: nc.Namespace,
			Name:      fmt.Sprintf("%s-0", gs.Name),
		}, &pod); err != nil {
			return ctrl.Result{}, fmt.Errorf("get game pod for ephemeral container injection: %w", err)
		}

		ephemeralExists := false
		for _, ec := range pod.Spec.EphemeralContainers {
			if ec.Name == "capture" {
				ephemeralExists = true
				break
			}
		}

		if !ephemeralExists {
			// Inject the capture ephemeral container.
			// NOTE: This is simplified for T041; the actual injection logic
			// is normally in gameserver_controller.go's ephemeral container injection,
			// but for T041 we create it inline here. In a full implementation,
			// this would be delegated to a shared helper.
			if err := r.injectCaptureContainer(ctx, &pod); err != nil {
				// The Pod read above comes from the manager's cache, which can
				// lag a subresource write. A requeue that races ahead of cache
				// propagation would see ephemeralExists == false a second time
				// and try to inject again; the apiserver rejects that as
				// invalid (duplicate ephemeral container name) or reports a
				// conflict on the stale object. Either way, injection has
				// already happened (or will land from the earlier write) —
				// treat it as done rather than erroring out.
				if !apierrors.IsInvalid(err) && !apierrors.IsConflict(err) {
					return ctrl.Result{}, fmt.Errorf("inject capture ephemeral container: %w", err)
				}
			}
		}

		// Idempotency: this branch may re-run after a successful StartCapture
		// whose subsequent Status().Update failed (a conflict, not exotic).
		// Ask the sidecar whether it already knows about this capture ID
		// before issuing another :start — a duplicate start against an
		// already-running capture would restart the pcap and lose packets.
		_, existingPackets, existingBytes, _, statusErr := r.SidecarClient.GetCaptureStatus(
			ctx, nc.Namespace, gs.Name, nc.Name,
		)
		if statusErr != nil {
			// The sidecar has no record of this capture yet; start it.
			if err := r.SidecarClient.StartCapture(
				ctx,
				nc.Namespace,
				gs.Name,
				nc.Name,
				nc.Spec.Filter,
				maxDurationSeconds,
				maxSizeBytes,
			); err != nil {
				return r.fail(ctx, &nc, fmt.Sprintf("failed to start capture on sidecar: %v", err))
			}
		} else {
			// Already running (or already finished) on the sidecar from a
			// prior attempt; reflect what it reports instead of starting again.
			nc.Status.PacketsWritten = existingPackets
			nc.Status.BytesWritten = resource.NewQuantity(existingBytes, resource.BinarySI)
		}

		// Transition to Running and record startTime.
		nc.Status.Phase = gameplanev1alpha1.CapturePhaseRunning
		nc.Status.StartTime = &metav1.Time{Time: time.Now()}
		nc.Status.Message = "capture running"

		// Update GameServer's active capture pointer.
		if err := r.patchGameServerActiveCapture(ctx, &gs, &nc); err != nil {
			return ctrl.Result{}, fmt.Errorf("update gameserver active capture: %w", err)
		}

		if err := r.Status().Update(ctx, &nc); err != nil {
			return ctrl.Result{}, fmt.Errorf("update capture status to Running: %w", err)
		}

		// Requeue quickly to poll status.
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Running: poll sidecar status and transition when complete/failed.
	if nc.Status.Phase == gameplanev1alpha1.CapturePhaseRunning {
		phase, packets, bytesWritten, message, err := r.SidecarClient.GetCaptureStatus(
			ctx,
			nc.Namespace,
			gs.Name,
			nc.Name,
		)
		if err != nil {
			// Sidecar unreachable; maybe it crashed. Fail the capture.
			return r.fail(ctx, &nc, fmt.Sprintf("sidecar unreachable: %v", err))
		}

		nc.Status.PacketsWritten = packets
		nc.Status.BytesWritten = resource.NewQuantity(bytesWritten, resource.BinarySI)
		nc.Status.Message = message

		switch strings.ToLower(strings.TrimSpace(phase)) {
		case "completed":
			nc.Status.Phase = gameplanev1alpha1.CapturePhaseCompleted
			nc.Status.CompletionTime = &metav1.Time{Time: time.Now()}

			// Clear the GameServer's active capture pointer.
			if err := r.patchGameServerActiveCapture(ctx, &gs, nil); err != nil {
				return ctrl.Result{}, fmt.Errorf("clear gameserver active capture: %w", err)
			}

			if err := r.Status().Update(ctx, &nc); err != nil {
				return ctrl.Result{}, fmt.Errorf("update capture status to Completed: %w", err)
			}
			return ctrl.Result{}, nil

		case "failed":
			nc.Status.Phase = gameplanev1alpha1.CapturePhaseFailed
			nc.Status.CompletionTime = &metav1.Time{Time: time.Now()}

			// Clear the GameServer's active capture pointer.
			if err := r.patchGameServerActiveCapture(ctx, &gs, nil); err != nil {
				return ctrl.Result{}, fmt.Errorf("clear gameserver active capture: %w", err)
			}

			if err := r.Status().Update(ctx, &nc); err != nil {
				return ctrl.Result{}, fmt.Errorf("update capture status to Failed: %w", err)
			}
			return ctrl.Result{}, nil
		}

		// Still running; requeue for another status poll.
		if err := r.Status().Update(ctx, &nc); err != nil {
			return ctrl.Result{}, fmt.Errorf("update capture status (running update): %w", err)
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// ensureOwnerReference sets a controller owner reference from nc to gs, if
// not already present, so the NetworkCapture is garbage-collected when its
// GameServer is deleted (matching backup_controller.go's SetControllerReference
// pattern for the Jobs it creates).
func (r *NetworkCaptureReconciler) ensureOwnerReference(
	ctx context.Context,
	nc *gameplanev1alpha1.NetworkCapture,
	gs *gameplanev1alpha1.GameServer,
) error {
	for _, ref := range nc.OwnerReferences {
		if ref.UID == gs.UID {
			return nil
		}
	}
	if err := controllerutil.SetControllerReference(gs, nc, r.Scheme); err != nil {
		return fmt.Errorf("set controller reference: %w", err)
	}
	return r.Update(ctx, nc)
}

// injectCaptureContainer patches the pod's ephemeralContainers list to add the capture container.
// This is a simplified version; full implementation would be in gameserver_controller.go.
func (r *NetworkCaptureReconciler) injectCaptureContainer(ctx context.Context, pod *corev1.Pod) error {
	image := r.CaptureSidecarImage
	if image == "" {
		image = DefaultCaptureSidecarImage
	}

	// Append the capture ephemeral container to the pod spec.
	captureContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:  "capture",
			Image: image,
			SecurityContext: &corev1.SecurityContext{
				RunAsNonRoot: ptrTo(true),
				// AllowPrivilegeEscalation must stay true: the capture binary
				// relies on Linux file capabilities (CAP_NET_RAW/CAP_NET_ADMIN
				// via setcap on the executable, not a running-as-root process)
				// to open a raw socket, and the kernel only honors those
				// capabilities on exec when no_new_privs is off. Combined with
				// RunAsNonRoot and Capabilities.Drop: ["ALL"] below, the
				// container still starts as an unprivileged user with no
				// capabilities of its own — only the setcap'd binary gains
				// NET_RAW/NET_ADMIN at exec time. capabilities.add is
				// deliberately never used here; see the ratified design.
				AllowPrivilegeEscalation: ptrTo(true),
				ReadOnlyRootFilesystem:   ptrTo(true),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"ALL"},
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					// Matches the "captures" emptyDir volume gameserver_controller.go
					// provisions unconditionally on every game pod template.
					Name:      "captures",
					MountPath: "/tmp/captures",
				},
				{
					Name:      "agent-tls",
					MountPath: "/etc/tls",
					ReadOnly:  true,
				},
			},
			Env: []corev1.EnvVar{
				{Name: "TLS_CERT_FILE", Value: "/etc/tls/tls.crt"},
				{Name: "TLS_KEY_FILE", Value: "/etc/tls/tls.key"},
				{Name: "TLS_CA_FILE", Value: "/etc/tls/ca.crt"},
			},
		},
		TargetContainerName: "game", // Target the game container for sharing pid/network/ipc.
	}

	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, captureContainer)

	// spec.ephemeralContainers is only mutable through the pods/ephemeralcontainers
	// subresource — a plain Update on the main pod resource is rejected by the
	// API server ("pod updates may not change fields other than ...").
	return r.SubResource("ephemeralcontainers").Update(ctx, pod)
}

// patchGameServerActiveCapture updates the GameServer's status.capture.activeCapture pointer.
// If nc is nil, clears the pointer.
func (r *NetworkCaptureReconciler) patchGameServerActiveCapture(
	ctx context.Context,
	gs *gameplanev1alpha1.GameServer,
	nc *gameplanev1alpha1.NetworkCapture,
) error {
	if gs.Status.Capture == nil {
		gs.Status.Capture = &gameplanev1alpha1.CaptureStatus{}
	}

	if nc == nil {
		gs.Status.Capture.ActiveCapture = nil
	} else {
		gs.Status.Capture.ActiveCapture = ptrTo(nc.Name)
	}

	return r.Status().Update(ctx, gs)
}

// fail marks a NetworkCapture as Failed with the given message.
func (r *NetworkCaptureReconciler) fail(ctx context.Context, nc *gameplanev1alpha1.NetworkCapture, message string) (ctrl.Result, error) {
	nc.Status.Phase = gameplanev1alpha1.CapturePhaseFailed
	nc.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	nc.Status.Message = message

	// Add a condition.
	cond := metav1.Condition{
		Type:               "Failed",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: nc.Generation,
		Reason:             "failed",
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
	meta.SetStatusCondition(&nc.Status.Conditions, cond)

	// Try to clear the GameServer's active capture pointer.
	var gs gameplanev1alpha1.GameServer
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: nc.Namespace,
		Name:      nc.Spec.ServerRef.Name,
	}, &gs); err == nil {
		if gs.Status.Capture == nil {
			gs.Status.Capture = &gameplanev1alpha1.CaptureStatus{}
		}
		gs.Status.Capture.ActiveCapture = nil
		_ = r.Status().Update(ctx, &gs) // Ignore error; we'll fail the capture anyway.
	}

	if err := r.Status().Update(ctx, nc); err != nil {
		return ctrl.Result{}, fmt.Errorf("fail capture %s: %w", nc.Name, err)
	}
	return ctrl.Result{}, nil
}

// mapPodToNetworkCaptures maps a game Pod event to the Running NetworkCapture(s)
// targeting that Pod's GameServer, so a pod crash/recreation while a capture is
// active re-triggers reconciliation instead of leaving the capture to poll on
// its own 5s requeue alone. The game Pod is owned by its StatefulSet, never by
// a NetworkCapture, so Owns(&corev1.Pod{}) would never fire — this explicit
// label-based lookup is the real mechanism.
func (r *NetworkCaptureReconciler) mapPodToNetworkCaptures(ctx context.Context, obj client.Object) []reconcile.Request {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil
	}
	serverName := pod.Labels["app.kubernetes.io/instance"]
	if serverName == "" {
		return nil
	}

	var captures gameplanev1alpha1.NetworkCaptureList
	if err := r.List(ctx, &captures, client.InNamespace(pod.Namespace)); err != nil {
		return nil
	}

	var reqs []reconcile.Request
	for _, c := range captures.Items {
		if c.Spec.ServerRef.Name == serverName && c.Status.Phase == gameplanev1alpha1.CapturePhaseRunning {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{Namespace: c.Namespace, Name: c.Name},
			})
		}
	}
	return reqs
}

func (r *NetworkCaptureReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gameplanev1alpha1.NetworkCapture{}).
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(r.mapPodToNetworkCaptures)).
		Complete(r)
}

// ptrTo returns a pointer to the given value.
func ptrTo[T any](v T) *T {
	return &v
}
