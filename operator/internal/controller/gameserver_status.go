package controller

import (
	"context"
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
)

// heartbeatFreshness defines how long an agent heartbeat is trusted.
// Reported values (playersOnline, etc.) are kept across short dropouts;
// the Healthy condition flips false once this window elapses.
const heartbeatFreshness = 60 * time.Second

// reconcileStatus derives phase / conditions / endpoints / startedAt
// from observed StatefulSet, Service, and the agent heartbeat. It's a
// pure computation — no child objects are mutated here.
func (r *GameServerReconciler) reconcileStatus(
	ctx context.Context, gs *kestrelv1alpha1.GameServer,
) (time.Duration, error) {
	// base captures the object as fetched so we can issue a JSON merge
	// patch of only the fields this reconciler owns. The agent sidecar
	// concurrently patches status.agent (its heartbeat); a full
	// Status().Update would carry the stale agent value we read at the
	// start of reconcile and revert a fresher heartbeat (and race it for
	// the resourceVersion). MergeFrom touches only changed fields, so
	// status.agent is left untouched and there is nothing to conflict on.
	base := gs.DeepCopy()
	orig := gs.Status.DeepCopy()

	var ss appsv1.StatefulSet
	ssErr := r.Get(ctx, types.NamespacedName{Name: gs.Name, Namespace: gs.Namespace}, &ss)
	if ssErr != nil && !apierrors.IsNotFound(ssErr) {
		return 0, ssErr
	}
	ssExists := ssErr == nil

	var svc corev1.Service
	svcErr := r.Get(ctx, types.NamespacedName{Name: gs.Name, Namespace: gs.Namespace}, &svc)
	if svcErr != nil && !apierrors.IsNotFound(svcErr) {
		return 0, svcErr
	}
	svcExists := svcErr == nil

	phase := derivePhase(gs, ssExists, ss.Status.ReadyReplicas > 0, heartbeatFresh(gs))

	gs.Status.Phase = phase
	gs.Status.ObservedGeneration = gs.Generation
	gs.Status.Conditions = computeConditions(gs, phase)
	if svcExists {
		gs.Status.Endpoints = endpointsFromService(&svc)
	}
	if phase == kestrelv1alpha1.GameServerPhaseRunning && gs.Status.StartedAt == nil {
		now := metav1.Now()
		gs.Status.StartedAt = &now
	}
	if phase == kestrelv1alpha1.GameServerPhaseStopped || phase == kestrelv1alpha1.GameServerPhaseSuspended {
		gs.Status.StartedAt = nil
	}

	if !reflect.DeepEqual(orig, &gs.Status) {
		if err := r.Status().Patch(ctx, gs, client.MergeFrom(base)); err != nil {
			return 0, err
		}
	}

	// Re-check when heartbeat is about to go stale so Healthy flips promptly.
	if phase == kestrelv1alpha1.GameServerPhaseRunning {
		return heartbeatFreshness, nil
	}
	return 15 * time.Second, nil
}

func derivePhase(
	gs *kestrelv1alpha1.GameServer, ssExists, ssReady, hbFresh bool,
) kestrelv1alpha1.GameServerPhase {
	if gs.Spec.Suspend {
		if ssExists && ssReady {
			return kestrelv1alpha1.GameServerPhaseStopping
		}
		return kestrelv1alpha1.GameServerPhaseSuspended
	}
	if !ssExists {
		return kestrelv1alpha1.GameServerPhasePending
	}
	if !ssReady {
		return kestrelv1alpha1.GameServerPhaseStarting
	}
	if !hbFresh {
		// Pod is ready but the agent isn't reporting — treat as
		// Starting until the first heartbeat. A long timeout here
		// could escalate to Failed; for now, optimistic.
		return kestrelv1alpha1.GameServerPhaseStarting
	}
	return kestrelv1alpha1.GameServerPhaseRunning
}

func heartbeatFresh(gs *kestrelv1alpha1.GameServer) bool {
	if gs.Status.Agent == nil || gs.Status.Agent.LastHeartbeat == nil {
		return false
	}
	return time.Since(gs.Status.Agent.LastHeartbeat.Time) < heartbeatFreshness
}

func computeConditions(
	gs *kestrelv1alpha1.GameServer, phase kestrelv1alpha1.GameServerPhase,
) []metav1.Condition {
	conds := gs.Status.Conditions

	var ready, progressing, healthy metav1.Condition
	ready = metav1.Condition{Type: "Ready", ObservedGeneration: gs.Generation}
	progressing = metav1.Condition{Type: "Progressing", ObservedGeneration: gs.Generation}
	healthy = metav1.Condition{Type: "Healthy", ObservedGeneration: gs.Generation}

	switch phase {
	case kestrelv1alpha1.GameServerPhaseRunning:
		ready.Status = metav1.ConditionTrue
		ready.Reason = "Running"
		ready.Message = "server is ready and the agent is reporting heartbeats"
		progressing.Status = metav1.ConditionFalse
		progressing.Reason = "Stable"
		healthy.Status = metav1.ConditionTrue
		healthy.Reason = "AgentFresh"
	case kestrelv1alpha1.GameServerPhaseStarting:
		ready.Status = metav1.ConditionFalse
		ready.Reason = "Starting"
		progressing.Status = metav1.ConditionTrue
		progressing.Reason = "Starting"
		healthy.Status = metav1.ConditionFalse
		healthy.Reason = "AgentStale"
	case kestrelv1alpha1.GameServerPhaseStopping:
		ready.Status = metav1.ConditionFalse
		ready.Reason = "Stopping"
		progressing.Status = metav1.ConditionTrue
		progressing.Reason = "Stopping"
		healthy.Status = metav1.ConditionFalse
		healthy.Reason = "Stopping"
	case kestrelv1alpha1.GameServerPhaseSuspended:
		ready.Status = metav1.ConditionFalse
		ready.Reason = "Suspended"
		progressing.Status = metav1.ConditionFalse
		progressing.Reason = "Suspended"
		healthy.Status = metav1.ConditionFalse
		healthy.Reason = "Suspended"
	case kestrelv1alpha1.GameServerPhaseFailed:
		ready.Status = metav1.ConditionFalse
		ready.Reason = "Failed"
		progressing.Status = metav1.ConditionFalse
		progressing.Reason = "Failed"
		healthy.Status = metav1.ConditionFalse
		healthy.Reason = "Failed"
	default:
		ready.Status = metav1.ConditionUnknown
		ready.Reason = "Unknown"
		progressing.Status = metav1.ConditionUnknown
		progressing.Reason = "Unknown"
		healthy.Status = metav1.ConditionUnknown
		healthy.Reason = "Unknown"
	}

	conds = upsertCondition(conds, ready)
	conds = upsertCondition(conds, progressing)
	conds = upsertCondition(conds, healthy)
	return conds
}

// endpointsFromService lists the per-port externally reachable address
// for a Service. For ClusterIP we report the cluster IP; for NodePort
// we report the Service's declared port (the node IP is left to the API
// layer since the operator doesn't know which node a user will hit).
func endpointsFromService(svc *corev1.Service) []kestrelv1alpha1.GameServerEndpoint {
	out := make([]kestrelv1alpha1.GameServerEndpoint, 0, len(svc.Spec.Ports))
	host := svc.Spec.ClusterIP
	if svc.Spec.Type == corev1.ServiceTypeLoadBalancer && len(svc.Status.LoadBalancer.Ingress) > 0 {
		ing := svc.Status.LoadBalancer.Ingress[0]
		if ing.Hostname != "" {
			host = ing.Hostname
		} else if ing.IP != "" {
			host = ing.IP
		}
	}
	for _, p := range svc.Spec.Ports {
		ep := kestrelv1alpha1.GameServerEndpoint{
			Name:     p.Name,
			Host:     host,
			Port:     p.Port,
			Protocol: p.Protocol,
		}
		if svc.Spec.Type == corev1.ServiceTypeNodePort && p.NodePort != 0 {
			ep.Port = p.NodePort
		}
		out = append(out, ep)
	}
	return out
}
