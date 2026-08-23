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

	// DeleteCaptureFile instructs the sidecar to remove a capture's backing
	// PCAPNG file from its emptyDir, without altering the NetworkCapture CRD
	// (the retention pass deletes the CR itself once this returns). Called
	// once a capture's TTL has elapsed (FR-007). Treated as best-effort by
	// callers: an error here (e.g. the sidecar/pod is already gone, so there
	// is no file left to remove) does not block the CR from being deleted.
	DeleteCaptureFile(ctx context.Context, namespace, serverName, captureID string) error
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

// capturePodUIDAnnotation records, on the NetworkCapture object itself, the
// UID of the game Pod observed at the moment the capture transitioned to
// Running — so a later Running-phase reconcile can tell a genuine pod
// recreation (evicted/rescheduled; same name "<gs>-0", new UID) apart from
// the pod merely being briefly unready. Stored as an annotation rather than
// a new status field to avoid a CRD schema/codegen change for what is
// purely internal reconciler bookkeeping, never read by any other client.
const capturePodUIDAnnotation = "gameplane.local/capture-pod-uid"

// defaultCaptureRetentionSeconds is the fallback TTL (seconds) applied to a
// terminal capture whose spec.ttlSecondsAfterFinished is nil or
// non-positive, and maxCaptureRetentionSeconds is the cluster-wide ceiling —
// both match the ratified values in
// specs/003-network-capture-sidecar/data-model.md (24-hour default, 7-day
// max via capture.defaultRetentionSeconds/capture.maxRetentionSeconds Helm
// values) and NetworkCaptureSpec.TTLSecondsAfterFinished's own
// kubebuilder:validation:Minimum=60/Maximum=604800. Used only as a fallback
// when the reconciler's CaptureDefaultRetentionSeconds/
// CaptureMaxRetentionSeconds fields are left at their zero value (e.g. not
// yet wired from a Helm-sourced flag in cmd/main.go, or in a test that
// doesn't care about the exact bound).
const (
	defaultCaptureRetentionSeconds int32 = 86400
	maxCaptureRetentionSeconds     int32 = 604800
)

// retentionPollInterval is the periodic backstop requeue for the retention
// pass, used only when a terminal capture has no anchor timestamp to compute
// an exact remaining-TTL requeue from — normal Completed/Failed transitions
// always stamp CompletionTime, so this should not fire in practice. Matches
// research.md's proposed interval, aligned with BackupSchedule's cadence.
const retentionPollInterval = 60 * time.Second

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

	// CaptureDefaultRetentionSeconds and CaptureMaxRetentionSeconds configure
	// the cluster-wide retention default/ceiling (data-model.md's
	// capture.defaultRetentionSeconds / capture.maxRetentionSeconds Helm
	// values). Zero (unconfigured) falls back to this file's
	// defaultCaptureRetentionSeconds/maxCaptureRetentionSeconds constants —
	// see effectiveRetentionSeconds.
	CaptureDefaultRetentionSeconds int32
	CaptureMaxRetentionSeconds     int32
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

	// Terminal phases need no further lifecycle reconciliation, but Completed/
	// Failed captures still carry a retention window (FR-007): Kubernetes has
	// no built-in TTL GC for custom CRDs, so this reconciler is the only thing
	// that expires and cleans them up. An object that is already Expired here
	// means a previous expiry attempt's CR deletion didn't land (e.g. an
	// apiserver hiccup) — retry it directly.
	if nc.Status.Phase == gameplanev1alpha1.CapturePhaseCompleted ||
		nc.Status.Phase == gameplanev1alpha1.CapturePhaseFailed {
		return r.reconcileRetention(ctx, &nc)
	}
	if nc.Status.Phase == gameplanev1alpha1.CapturePhaseExpired {
		return r.expireCapture(ctx, &nc)
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

		// Concurrency lock: exactly one Pending/Running capture may exist per
		// GameServer at a time, and this reconciler is the sole authority for
		// that invariant — api/internal/handlers/capture.go's own check
		// (hasActiveCapture) is a best-effort fast path only, since two
		// concurrent create requests can both pass it before either CR
		// exists. Enforce the lock by determining the earliest-created
		// Pending/Running capture for this server (by CreationTimestamp,
		// then Name to break an exact tie — two objects can land within the
		// same timestamp resolution, especially under envtest) and failing
		// every other one. nc's own entry in the list snapshot may still
		// show an empty phase (its first-ever reconcile hasn't persisted
		// Pending yet — see the status-subresource note above), so nc is
		// always treated as active regardless of what List returned for it.
		var captures gameplanev1alpha1.NetworkCaptureList
		if err := r.List(ctx, &captures, client.InNamespace(nc.Namespace)); err != nil {
			return ctrl.Result{}, fmt.Errorf("list captures for concurrency check: %w", err)
		}

		var earliest *gameplanev1alpha1.NetworkCapture
		for i := range captures.Items {
			other := &captures.Items[i]
			if other.Spec.ServerRef.Name != nc.Spec.ServerRef.Name {
				continue
			}
			active := other.Name == nc.Name ||
				other.Status.Phase == gameplanev1alpha1.CapturePhasePending ||
				other.Status.Phase == gameplanev1alpha1.CapturePhaseRunning
			if !active {
				continue
			}
			switch {
			case earliest == nil:
				earliest = other
			case other.CreationTimestamp.Before(&earliest.CreationTimestamp):
				earliest = other
			case other.CreationTimestamp.Equal(&earliest.CreationTimestamp) && other.Name < earliest.Name:
				earliest = other
			}
		}
		if earliest != nil && earliest.Name != nc.Name {
			return r.failWithReason(ctx, &nc, "capture_already_in_progress",
				fmt.Sprintf("another capture (%s) is already in progress on this gameserver", earliest.Name))
		}

		// Inject ephemeral container if not already present.
		var pod corev1.Pod
		if err := r.Get(ctx, types.NamespacedName{
			Namespace: nc.Namespace,
			Name:      fmt.Sprintf("%s-0", gs.Name),
		}, &pod); err != nil {
			return ctrl.Result{}, fmt.Errorf("get game pod for ephemeral container injection: %w", err)
		}

		if !hasCaptureEphemeralContainer(&pod) {
			// Fallback injection — see injectCaptureContainer's doc comment
			// for why this is normally already done by the time we get here.
			if err := r.injectCaptureContainer(ctx, &pod); err != nil {
				// The Pod read above comes from the manager's cache, which can
				// lag a subresource write. A requeue that races ahead of cache
				// propagation would see hasCaptureEphemeralContainer == false
				// a second time and try to inject again; the apiserver
				// rejects that as invalid (duplicate ephemeral container
				// name) or reports a conflict on the stale object. Either
				// way, injection has already happened (or will land from the
				// earlier write) — treat it as done rather than erroring out.
				if !apierrors.IsInvalid(err) && !apierrors.IsConflict(err) {
					return ctrl.Result{}, fmt.Errorf("inject capture ephemeral container: %w", err)
				}
			}
		}

		// Record the pod UID observed right now, so the Running-phase health
		// check below can later tell a genuine pod recreation (new UID,
		// same name) apart from the pod simply being briefly unready.
		// Annotations live on metadata, not the status subresource, so this
		// needs its own Update call distinct from the Status().Update below.
		if nc.Annotations == nil {
			nc.Annotations = map[string]string{}
		}
		if nc.Annotations[capturePodUIDAnnotation] != string(pod.UID) {
			nc.Annotations[capturePodUIDAnnotation] = string(pod.UID)
			if err := r.Update(ctx, &nc); err != nil {
				return ctrl.Result{}, fmt.Errorf("record pod UID for capture %s: %w", nc.Name, err)
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
		if err := r.patchGameServerActiveCapture(ctx, &gs, &nc, nil); err != nil {
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
		// Retention safety net: a capture that never reaches a terminal phase
		// (e.g. the sidecar stops reporting without actually finishing) would
		// otherwise poll forever and never expire, since TTL is normally
		// anchored to completionTime. Anchor to startTime instead for a
		// still-Running capture, and force it to a stop once that window
		// elapses too, rather than skipping retention for it entirely.
		if nc.Status.StartTime != nil {
			ttl := effectiveRetentionSeconds(nc.Spec.TTLSecondsAfterFinished, r.CaptureDefaultRetentionSeconds, r.CaptureMaxRetentionSeconds)
			if expiresAt := captureExpiresAt(nc.Status.StartTime, ttl); captureIsExpired(expiresAt, time.Now()) {
				return r.expireStuckRunningCapture(ctx, &gs, &nc)
			}
		}

		// Failure detection: a dead sidecar can't answer GetCaptureStatus at
		// all, and a same-named replacement pod (StatefulSet pod restart
		// keeps the name "<gs>-0") would make the sidecar client report a
		// fresh, misleading "capture not found" rather than the real cause.
		// Check pod health directly before trusting the sidecar's status.
		var pod corev1.Pod
		podErr := r.Get(ctx, types.NamespacedName{
			Namespace: nc.Namespace,
			Name:      fmt.Sprintf("%s-0", gs.Name),
		}, &pod)
		switch {
		case apierrors.IsNotFound(podErr):
			return r.failWithReason(ctx, &nc, "PodRestarted", "game pod was deleted while the capture was running")
		case podErr != nil:
			return ctrl.Result{}, fmt.Errorf("get game pod for capture health check: %w", podErr)
		}
		if observedUID := nc.Annotations[capturePodUIDAnnotation]; observedUID != "" && observedUID != string(pod.UID) {
			return r.failWithReason(ctx, &nc, "PodRestarted", "game pod was deleted and recreated while the capture was running")
		}
		if cs := captureEphemeralContainerStatus(&pod); cs != nil && cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
			return r.failWithReason(ctx, &nc, "SidecarCrashed",
				fmt.Sprintf("capture sidecar exited unexpectedly (exit code %d, reason %s)",
					cs.State.Terminated.ExitCode, cs.State.Terminated.Reason))
		}

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

			// Clear the GameServer's active capture pointer and record this
			// as the most recent terminal capture.
			if err := r.patchGameServerActiveCapture(ctx, &gs, nil, nc.Status.CompletionTime); err != nil {
				return ctrl.Result{}, fmt.Errorf("clear gameserver active capture: %w", err)
			}

			if err := r.Status().Update(ctx, &nc); err != nil {
				return ctrl.Result{}, fmt.Errorf("update capture status to Completed: %w", err)
			}
			return ctrl.Result{}, nil

		case "failed":
			nc.Status.Phase = gameplanev1alpha1.CapturePhaseFailed
			nc.Status.CompletionTime = &metav1.Time{Time: time.Now()}

			// Clear the GameServer's active capture pointer and record this
			// as the most recent terminal capture.
			if err := r.patchGameServerActiveCapture(ctx, &gs, nil, nc.Status.CompletionTime); err != nil {
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

// clampRetention bounds requested to (0, clusterMax]: a non-positive
// requested value falls back to clusterDefault, and a value above
// clusterMax is clamped down to it. Never returns a value the caller
// couldn't use — the retention pass must always have *some* TTL to work
// with, even for a legacy/hand-applied object whose spec value bypassed
// admission validation.
func clampRetention(requested, clusterMax, clusterDefault int32) int32 {
	if requested <= 0 {
		requested = clusterDefault
	}
	if requested > clusterMax {
		return clusterMax
	}
	return requested
}

// effectiveRetentionSeconds resolves the TTL to enforce for a capture: the
// spec value when set and positive, else clusterDefault, clamped to
// clusterMax either way. clusterDefault/clusterMax of zero (unconfigured on
// the reconciler) fall back to this file's
// defaultCaptureRetentionSeconds/maxCaptureRetentionSeconds constants.
func effectiveRetentionSeconds(specTTL *int32, clusterDefault, clusterMax int32) int32 {
	if clusterDefault <= 0 {
		clusterDefault = defaultCaptureRetentionSeconds
	}
	if clusterMax <= 0 {
		clusterMax = maxCaptureRetentionSeconds
	}
	requested := clusterDefault
	if specTTL != nil {
		requested = *specTTL
	}
	return clampRetention(requested, clusterMax, clusterDefault)
}

// captureExpiresAt returns the instant a capture's retention window elapses,
// anchored to anchorTime (status.completionTime for a terminal capture, or
// status.startTime as the Running-phase safety-net anchor — see the
// retention safety net in the Running branch above). Returns the zero Time
// when anchorTime is nil; callers must check IsZero() before treating the
// result as meaningful, since a zero Time compares as "before" everything.
func captureExpiresAt(anchorTime *metav1.Time, ttlSeconds int32) time.Time {
	if anchorTime == nil {
		return time.Time{}
	}
	return anchorTime.Add(time.Duration(ttlSeconds) * time.Second)
}

// captureIsExpired reports whether now has moved past expiresAt. The
// boundary is exclusive: now == expiresAt is NOT yet expired, matching
// data-model.md's documented transition condition "completionTime + ttl <
// now()" (strict less-than). A zero expiresAt (no anchor) is never expired.
func captureIsExpired(expiresAt, now time.Time) bool {
	if expiresAt.IsZero() {
		return false
	}
	return expiresAt.Before(now)
}

// reconcileRetention checks a Completed/Failed NetworkCapture's TTL and,
// once elapsed, expires and deletes it (FR-007). Not yet expired: requeues
// at the exact remaining TTL rather than a blind fixed poll, so expiry lands
// close to on time without a hot requeue loop.
func (r *NetworkCaptureReconciler) reconcileRetention(ctx context.Context, nc *gameplanev1alpha1.NetworkCapture) (ctrl.Result, error) {
	ttl := effectiveRetentionSeconds(nc.Spec.TTLSecondsAfterFinished, r.CaptureDefaultRetentionSeconds, r.CaptureMaxRetentionSeconds)
	expiresAt := captureExpiresAt(nc.Status.CompletionTime, ttl)
	if expiresAt.IsZero() {
		// No completionTime recorded — shouldn't happen for a capture this
		// reconciler itself transitioned to Completed/Failed, but defend
		// against a hand-applied or legacy object rather than never expiring
		// it: fall back to the periodic backstop poll.
		return ctrl.Result{RequeueAfter: retentionPollInterval}, nil
	}

	now := time.Now()
	if !captureIsExpired(expiresAt, now) {
		return ctrl.Result{RequeueAfter: expiresAt.Sub(now)}, nil
	}
	return r.expireCapture(ctx, nc)
}

// expireStuckRunningCapture handles the Running-phase retention safety net:
// a capture whose retention window elapses with no completionTime set,
// because it never reached a terminal phase on its own (e.g. the sidecar
// stopped reporting without actually finishing). Force-stops it on the
// sidecar and clears the GameServer's active-capture pointer before falling
// through the same expireCapture path (file cleanup + CR deletion) the
// terminal-phase retention pass uses, rather than silently skipping it.
func (r *NetworkCaptureReconciler) expireStuckRunningCapture(
	ctx context.Context,
	gs *gameplanev1alpha1.GameServer,
	nc *gameplanev1alpha1.NetworkCapture,
) (ctrl.Result, error) {
	stopTime := &metav1.Time{Time: time.Now()}

	// Best-effort: the sidecar may already be gone (pod restarted), in which
	// case there is nothing left to stop.
	if err := r.SidecarClient.StopCapture(ctx, nc.Namespace, nc.Spec.ServerRef.Name, nc.Name); err != nil {
		meta.SetStatusCondition(&nc.Status.Conditions, metav1.Condition{
			Type:               "RetentionStopFailed",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: nc.Generation,
			Reason:             "sidecar_stop_failed",
			Message:            err.Error(),
			LastTransitionTime: metav1.Now(),
		})
	}

	if err := r.patchGameServerActiveCapture(ctx, gs, nil, stopTime); err != nil {
		return ctrl.Result{}, fmt.Errorf("clear gameserver active capture for retention-expired running capture %s: %w", nc.Name, err)
	}

	nc.Status.CompletionTime = stopTime
	nc.Status.Message = "capture retention window elapsed while running; force-stopped"
	return r.expireCapture(ctx, nc)
}

// expireCapture transitions nc to Expired (persisting first if it isn't
// already), best-effort deletes its backing file via the sidecar, then
// deletes the CR itself. File deletion is best-effort: the pod (and its
// emptyDir) may already be gone by the time retention fires, which is not
// itself an error worth failing the reconcile over — the CR still gets
// deleted either way.
func (r *NetworkCaptureReconciler) expireCapture(ctx context.Context, nc *gameplanev1alpha1.NetworkCapture) (ctrl.Result, error) {
	if nc.Status.Phase != gameplanev1alpha1.CapturePhaseExpired {
		nc.Status.Phase = gameplanev1alpha1.CapturePhaseExpired
		if nc.Status.CompletionTime == nil {
			nc.Status.CompletionTime = &metav1.Time{Time: time.Now()}
		}
		if err := r.Status().Update(ctx, nc); err != nil {
			return ctrl.Result{}, fmt.Errorf("mark capture %s expired: %w", nc.Name, err)
		}
	}

	if err := r.SidecarClient.DeleteCaptureFile(ctx, nc.Namespace, nc.Spec.ServerRef.Name, nc.Name); err != nil {
		meta.SetStatusCondition(&nc.Status.Conditions, metav1.Condition{
			Type:               "FileCleanupFailed",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: nc.Generation,
			Reason:             "delete_failed",
			Message:            err.Error(),
			LastTransitionTime: metav1.Now(),
		})
		_ = r.Status().Update(ctx, nc) // Best-effort; the CR delete below still proceeds.
	}

	if err := r.Delete(ctx, nc); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("delete expired capture %s: %w", nc.Name, err)
	}
	return ctrl.Result{}, nil
}

// injectCaptureContainer patches the pod's ephemeralContainers list to add
// the capture container. This is the idempotent fallback path: the common
// path is that GameServerReconciler's reconcileCapture (gameserver_controller.go)
// already injected it eagerly when spec.capture.enabled was set — this only
// fires when that hasn't landed yet (e.g. this reconciler's cache is ahead
// of the GameServer reconciler's write). Uses the same
// buildCaptureEphemeralContainer definition GameServerReconciler does, so
// the two injection paths can never produce different container specs.
func (r *NetworkCaptureReconciler) injectCaptureContainer(ctx context.Context, pod *corev1.Pod) error {
	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, buildCaptureEphemeralContainer(r.CaptureSidecarImage))

	// spec.ephemeralContainers is only mutable through the pods/ephemeralcontainers
	// subresource — a plain Update on the main pod resource is rejected by the
	// API server ("pod updates may not change fields other than ...").
	return r.SubResource("ephemeralcontainers").Update(ctx, pod)
}

// patchGameServerActiveCapture updates the GameServer's status.capture
// pointer fields. If nc is nil, clears status.capture.activeCapture;
// otherwise sets it to nc.Name. terminalTime, when non-nil, additionally
// stamps status.capture.lastCaptureTime — passed only from the
// Completed/Failed transitions below (and from fail()), never from the
// Pending→Running transition that sets the active pointer.
func (r *NetworkCaptureReconciler) patchGameServerActiveCapture(
	ctx context.Context,
	gs *gameplanev1alpha1.GameServer,
	nc *gameplanev1alpha1.NetworkCapture,
	terminalTime *metav1.Time,
) error {
	if gs.Status.Capture == nil {
		gs.Status.Capture = &gameplanev1alpha1.CaptureStatus{}
	}

	if nc == nil {
		gs.Status.Capture.ActiveCapture = nil
	} else {
		gs.Status.Capture.ActiveCapture = ptrTo(nc.Name)
	}

	if terminalTime != nil {
		gs.Status.Capture.LastCaptureTime = terminalTime
	}

	return r.Status().Update(ctx, gs)
}

// fail marks a NetworkCapture as Failed with the given message and the
// generic "failed" condition reason. Most failure paths (gameserver not
// found, capture disabled, sidecar start error, sidecar unreachable) don't
// need a more specific reason a caller would act on differently — see
// failWithReason for the ones that do (capture_already_in_progress,
// PodRestarted, SidecarCrashed).
func (r *NetworkCaptureReconciler) fail(ctx context.Context, nc *gameplanev1alpha1.NetworkCapture, message string) (ctrl.Result, error) {
	return r.failWithReason(ctx, nc, "failed", message)
}

// failWithReason marks a NetworkCapture as Failed with the given message,
// recording reason on the "Failed" condition so callers (dashboard, e2e
// assertions) can distinguish why without parsing the free-text message.
func (r *NetworkCaptureReconciler) failWithReason(ctx context.Context, nc *gameplanev1alpha1.NetworkCapture, reason, message string) (ctrl.Result, error) {
	nc.Status.Phase = gameplanev1alpha1.CapturePhaseFailed
	nc.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	nc.Status.Message = message

	// Add a condition.
	cond := metav1.Condition{
		Type:               "Failed",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: nc.Generation,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
	meta.SetStatusCondition(&nc.Status.Conditions, cond)

	// Try to clear the GameServer's active capture pointer and record this
	// as the most recent terminal capture.
	var gs gameplanev1alpha1.GameServer
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: nc.Namespace,
		Name:      nc.Spec.ServerRef.Name,
	}, &gs); err == nil {
		_ = r.patchGameServerActiveCapture(ctx, &gs, nil, nc.Status.CompletionTime) // Ignore error; we'll fail the capture anyway.
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
