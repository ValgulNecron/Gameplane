package controller

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

// heartbeatFreshness defines how long an agent heartbeat is trusted.
// Reported values (playersOnline, etc.) are kept across short dropouts;
// the Healthy condition flips false once this window elapses.
const heartbeatFreshness = 60 * time.Second

// reconcileStatus derives phase / conditions / endpoints / startedAt
// from observed StatefulSet, Service, and the agent heartbeat. It's a
// pure computation — no child objects are mutated here.
//
// tunnelEndpoints are prepended to the Service endpoints so the dashboard's
// Connection card reads the tunnel address when present.
func (r *GameServerReconciler) reconcileStatus(
	ctx context.Context, gs *gameplanev1alpha1.GameServer,
	idle idleState, idleStatus *gameplanev1alpha1.IdleStatus, tunnelPlan tunnelPlan,
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

	phase := derivePhase(gs, ssExists, ss.Status.ReadyReplicas > 0, heartbeatFresh(gs), idle)

	// While Starting, read the pod's container states to either explain
	// *why* it isn't Running yet (pulling the image, creating the
	// container, installing on first boot, waiting for the agent) — feeding
	// the dashboard's provisioning sub-status — or, if startup has
	// terminally failed, escalate the phase to Failed.
	var prov *provisioningInfo
	if phase == gameplanev1alpha1.GameServerPhaseStarting {
		var pod corev1.Pod
		podErr := r.Get(ctx, types.NamespacedName{Name: gs.Name + "-0", Namespace: gs.Namespace}, &pod)
		switch {
		case apierrors.IsNotFound(podErr):
			prov = &provisioningInfo{reason: "Pending", message: "scheduling the pod"}
		case podErr != nil:
			return 0, fmt.Errorf("get pod %s-0: %w", gs.Name, podErr)
		default:
			if fr, fm, failed := startupFailure(&pod); failed {
				// A terminal startup failure (unpullable image, persistent
				// crash-loop, non-zero exit) — escalate to Failed so the
				// dashboard stops showing a perpetual "Starting". Not sticky:
				// derivePhase re-evaluates every reconcile, so a later
				// recovery returns the phase to Running.
				phase = gameplanev1alpha1.GameServerPhaseFailed
				prov = &provisioningInfo{reason: fr, message: fm}
			} else {
				reason, message := provisioningReason(&pod, heartbeatFresh(gs))
				prov = &provisioningInfo{reason: reason, message: message}
			}
		}
	}

	gs.Status.Phase = phase
	gs.Status.ObservedGeneration = gs.Generation
	gs.Status.Conditions = computeConditions(gs, phase, prov, idle)

	// Fetch tunnel Deployment to compute TunnelReady condition.
	var tunnelDep *appsv1.Deployment
	if tunnelPlan.wantTunnel {
		var dep appsv1.Deployment
		tunnelDepErr := r.Get(ctx, types.NamespacedName{
			Name:      gs.Name + "-tunnel",
			Namespace: gs.Namespace,
		}, &dep)
		if tunnelDepErr == nil {
			tunnelDep = &dep
		}
	}

	// Compute tunnel conditions.
	gs.Status.Conditions = computeTunnelConditions(gs, tunnelPlan, tunnelDep)

	// Folded into this one status patch rather than written by reconcileIdle
	// itself: the agent concurrently patches status.agent, and a second writer
	// here would be a second thing to race. A nil block clears status.idle,
	// which is how disabling the feature drops its stale read model.
	gs.Status.Idle = idleStatus

	// Endpoints: tunnel endpoints come first (when present) so the dashboard's
	// Connection card reads the address players actually use.
	if svcExists {
		gs.Status.Endpoints = endpointsFromService(&svc)
	}
	if tunnelPlan.wantTunnel && len(tunnelPlan.endpoints) > 0 {
		gs.Status.Endpoints = append(tunnelPlan.endpoints, gs.Status.Endpoints...)
	}
	if phase == gameplanev1alpha1.GameServerPhaseRunning && gs.Status.StartedAt == nil {
		now := metav1.Now()
		gs.Status.StartedAt = &now
	}
	if phase == gameplanev1alpha1.GameServerPhaseStopped || phase == gameplanev1alpha1.GameServerPhaseSuspended {
		gs.Status.StartedAt = nil
	}

	if !reflect.DeepEqual(orig, &gs.Status) {
		if err := r.Status().Patch(ctx, gs, client.MergeFrom(base)); err != nil {
			return 0, err
		}
	}

	// Re-check when heartbeat is about to go stale so Healthy flips promptly.
	if phase == gameplanev1alpha1.GameServerPhaseRunning {
		return heartbeatFreshness, nil
	}
	return 15 * time.Second, nil
}

func derivePhase(
	gs *gameplanev1alpha1.GameServer, ssExists, ssReady, hbFresh bool, idle idleState,
) gameplanev1alpha1.GameServerPhase {
	// An idle sleep reports the same phase as an explicit stop. Without this
	// arm a sleeping server would read as Starting — the StatefulSet exists but
	// has no ready replica — and look permanently stuck. The two are told apart
	// by the IdleAsleep condition reason and status.idle, not by the phase.
	if gs.Spec.Suspend || idle == idleAsleep {
		if ssExists && ssReady {
			return gameplanev1alpha1.GameServerPhaseStopping
		}
		return gameplanev1alpha1.GameServerPhaseSuspended
	}
	if !ssExists {
		return gameplanev1alpha1.GameServerPhasePending
	}
	if !ssReady {
		return gameplanev1alpha1.GameServerPhaseStarting
	}
	if !hbFresh {
		// Pod is ready but the agent isn't reporting — treat as
		// Starting until the first heartbeat. A long timeout here
		// could escalate to Failed; for now, optimistic.
		return gameplanev1alpha1.GameServerPhaseStarting
	}
	return gameplanev1alpha1.GameServerPhaseRunning
}

func heartbeatFresh(gs *gameplanev1alpha1.GameServer) bool {
	if gs.Status.Agent == nil || gs.Status.Agent.LastHeartbeat == nil {
		return false
	}
	return time.Since(gs.Status.Agent.LastHeartbeat.Time) < heartbeatFreshness
}

func computeConditions(
	gs *gameplanev1alpha1.GameServer,
	phase gameplanev1alpha1.GameServerPhase,
	prov *provisioningInfo,
	idle idleState,
) []metav1.Condition {
	conds := gs.Status.Conditions

	var ready, progressing, healthy metav1.Condition
	ready = metav1.Condition{Type: "Ready", ObservedGeneration: gs.Generation}
	progressing = metav1.Condition{Type: "Progressing", ObservedGeneration: gs.Generation}
	healthy = metav1.Condition{Type: "Healthy", ObservedGeneration: gs.Generation}

	switch phase {
	case gameplanev1alpha1.GameServerPhaseRunning:
		ready.Status = metav1.ConditionTrue
		ready.Reason = "Running"
		ready.Message = "server is ready and the agent is reporting heartbeats"
		progressing.Status = metav1.ConditionFalse
		progressing.Reason = "Stable"
		healthy.Status = metav1.ConditionTrue
		healthy.Reason = "AgentFresh"
	case gameplanev1alpha1.GameServerPhaseStarting:
		ready.Status = metav1.ConditionFalse
		ready.Reason = "Starting"
		progressing.Status = metav1.ConditionTrue
		progressing.Reason = "Starting"
		// Refine the generic "Starting" with what the pod is actually
		// doing, so the dashboard can show "Pulling image" /
		// "Installing server files" / "Waiting for agent".
		if prov != nil {
			if prov.reason != "" {
				progressing.Reason = prov.reason
			}
			progressing.Message = prov.message
		}
		healthy.Status = metav1.ConditionFalse
		healthy.Reason = "AgentStale"
	case gameplanev1alpha1.GameServerPhaseStopping:
		ready.Status = metav1.ConditionFalse
		ready.Reason = "Stopping"
		progressing.Status = metav1.ConditionTrue
		progressing.Reason = "Stopping"
		healthy.Status = metav1.ConditionFalse
		healthy.Reason = "Stopping"
	case gameplanev1alpha1.GameServerPhaseSuspended:
		ready.Status = metav1.ConditionFalse
		ready.Reason = "Suspended"
		progressing.Status = metav1.ConditionFalse
		progressing.Reason = "Suspended"
		healthy.Status = metav1.ConditionFalse
		healthy.Reason = "Suspended"
		// The phase is shared with an explicit stop, so the reason is the only
		// machine-readable way to tell "the operator parked this because it was
		// empty" from "a human turned it off".
		if idle == idleAsleep {
			ready.Reason = "IdleAsleep"
			ready.Message = "asleep: no players online"
			progressing.Reason = "IdleAsleep"
			healthy.Reason = "IdleAsleep"
		}
	case gameplanev1alpha1.GameServerPhaseFailed:
		ready.Status = metav1.ConditionFalse
		ready.Reason = "Failed"
		progressing.Status = metav1.ConditionFalse
		progressing.Reason = "Failed"
		healthy.Status = metav1.ConditionFalse
		healthy.Reason = "Failed"
		// Carry the specific startup-failure reason (image pull, crash-loop,
		// exit) so the dashboard can explain *why* it failed.
		if prov != nil && prov.reason != "" {
			ready.Reason = prov.reason
			ready.Message = prov.message
			progressing.Reason = prov.reason
			progressing.Message = prov.message
		}
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

// gameContainerName is the name the controller gives the game container in
// every pod (see buildGameContainer). The pod-log proxy keys off the same
// name — keep them in sync.
const gameContainerName = "game"

// provisioningInfo is the human-facing refinement of the Starting phase:
// a short Reason and a sentence-long Message describing what the pod is
// currently doing. It surfaces on the Progressing condition.
type provisioningInfo struct {
	reason  string
	message string
}

// provisioningReason inspects a Starting pod's container states to explain
// why it isn't Running yet — image pull, container creation, first-run
// install (game container up but not Ready), or waiting for the agent's
// first heartbeat. It's a pure function so it can be unit-tested without a
// live kubelet (envtest never runs one). hbFresh is the heartbeat result
// the caller already computed.
func provisioningReason(pod *corev1.Pod, hbFresh bool) (reason, message string) {
	// Init containers (config-init) run before the game container; if one
	// is stuck pulling/creating, surface that first.
	for i := range pod.Status.InitContainerStatuses {
		if w := pod.Status.InitContainerStatuses[i].State.Waiting; w != nil {
			if r, m := waitingReason(w.Reason); r != "" {
				return r, m
			}
		}
	}

	var game *corev1.ContainerStatus
	for i := range pod.Status.ContainerStatuses {
		if pod.Status.ContainerStatuses[i].Name == gameContainerName {
			game = &pod.Status.ContainerStatuses[i]
			break
		}
	}
	if game == nil {
		// Pod scheduled but the kubelet hasn't reported the game container
		// yet — still being created.
		return "ContainerCreating", "creating the container"
	}

	switch {
	case game.State.Waiting != nil:
		if r, m := waitingReason(game.State.Waiting.Reason); r != "" {
			return r, m
		}
		return "ContainerCreating", "creating the container"
	case game.State.Terminated != nil:
		// Exited during startup — a failed install or crash before ready.
		return "ContainerExited", "the container exited during startup; check the logs"
	case game.State.Running != nil:
		if podReady(pod) && !hbFresh {
			return "WaitingForAgent", "server is up; waiting for the agent's first heartbeat"
		}
		// Running but not Ready: the entrypoint is still installing/
		// generating before the readiness probe passes.
		return "InstallingServerFiles", "downloading game files / waiting for readiness"
	default:
		return "Starting", "starting up"
	}
}

// waitingReason maps a container's Waiting.Reason to a Gameplane reason +
// message, or ("", "") if it's not one we specifically explain.
func waitingReason(reason string) (string, string) {
	switch reason {
	case "ImagePullBackOff", "ErrImagePull":
		return "PullingImage", "pulling the game image"
	case "ContainerCreating", "PodInitializing":
		return "ContainerCreating", "creating the container"
	case "CrashLoopBackOff":
		return "CrashLoopBackOff", "the container is crash-looping during startup; check the logs"
	}
	return "", ""
}

// crashLoopFailureThreshold is how many restarts of the game container we
// tolerate during startup before declaring the server Failed. A first boot
// that crashes once or twice and then succeeds stays Starting; a persistent
// crash-loop escalates so the dashboard stops showing a perpetual
// "Starting".
const crashLoopFailureThreshold = 3

// startupFailure reports whether a Starting pod has hit a terminal startup
// failure that will not clear on its own — an unpullable image, a persistent
// crash-loop, or a container that exited non-zero — with a human-facing
// reason and message. It's a pure function (envtest has no kubelet, so the
// container states are supplied by the test). The result is advisory only:
// derivePhase re-evaluates every reconcile, so a pod that later recovers
// returns to Running.
func startupFailure(pod *corev1.Pod) (reason, message string, failed bool) {
	// Init containers (config-init) gate the game container; a stuck image
	// pull there is just as terminal.
	for i := range pod.Status.InitContainerStatuses {
		if r, m, f := containerFailure(&pod.Status.InitContainerStatuses[i]); f {
			return r, m, true
		}
	}
	for i := range pod.Status.ContainerStatuses {
		if pod.Status.ContainerStatuses[i].Name == gameContainerName {
			return containerFailure(&pod.Status.ContainerStatuses[i])
		}
	}
	return "", "", false
}

// containerFailure classifies a single container status as a terminal
// startup failure (true) or a state still worth waiting on (false).
func containerFailure(cs *corev1.ContainerStatus) (reason, message string, failed bool) {
	if w := cs.State.Waiting; w != nil {
		switch {
		case w.Reason == "ImagePullBackOff":
			// The kubelet already retried the pull and is backing off — a
			// bad image reference, not a transient first-attempt blip.
			return "ImagePullFailed", "cannot pull the image — check the image reference", true
		case w.Reason == "CrashLoopBackOff" && cs.RestartCount >= crashLoopFailureThreshold:
			return "CrashLoopBackOff", fmt.Sprintf(
				"the container has crash-looped %d times during startup; check the logs",
				cs.RestartCount), true
		}
	}
	if t := cs.State.Terminated; t != nil && t.ExitCode != 0 {
		return "ContainerExited", fmt.Sprintf(
			"the container exited with code %d during startup; check the logs",
			t.ExitCode), true
	}
	return "", "", false
}

// podReady reports whether the pod's Ready condition is true.
func podReady(pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

// computeTunnelConditions sets the TunnelReady condition based on the tunnel
// Deployment's readiness and configuration state. When the tunnel is disabled,
// the condition is removed from the status.
func computeTunnelConditions(
	gs *gameplanev1alpha1.GameServer,
	plan tunnelPlan,
	tunnelDep *appsv1.Deployment,
) []metav1.Condition {
	conds := gs.Status.Conditions

	// If tunnel is not wanted, remove the TunnelReady condition.
	if !plan.wantTunnel {
		conds = removeCondition(conds, "TunnelReady")
		return conds
	}

	// Tunnel is wanted: determine readiness.
	var tunnelReady metav1.Condition
	tunnelReady.Type = "TunnelReady"
	tunnelReady.ObservedGeneration = gs.Generation

	tunnel := gs.Spec.Networking.Tunnel
	if tunnel == nil {
		// Should not happen if plan.wantTunnel is true, but guard anyway.
		tunnelReady.Status = metav1.ConditionFalse
		tunnelReady.Reason = "InvalidConfig"
		tunnelReady.Message = "tunnel enabled but no tunnel spec"
		conds = upsertCondition(conds, tunnelReady)
		return conds
	}

	// Tunnel Deployment not yet created or not ready.
	if tunnelDep == nil {
		tunnelReady.Status = metav1.ConditionFalse
		tunnelReady.Reason = "DeploymentNotReady"
		tunnelReady.Message = "waiting for tunnel deployment to be created"
		conds = upsertCondition(conds, tunnelReady)
		return conds
	}

	if tunnelDep.Status.ReadyReplicas == 0 {
		tunnelReady.Status = metav1.ConditionFalse
		tunnelReady.Reason = "DeploymentNotReady"
		tunnelReady.Message = "tunnel deployment has no ready replicas"
		conds = upsertCondition(conds, tunnelReady)
		return conds
	}

	// Provider-specific readiness checks.
	switch tunnel.Provider {
	case "frp":
		if tunnel.Frp == nil {
			tunnelReady.Status = metav1.ConditionFalse
			tunnelReady.Reason = "InvalidConfig"
			tunnelReady.Message = "frp provider selected but no frp config"
			conds = upsertCondition(conds, tunnelReady)
			return conds
		}

		// Check for unmapped ports: frp requires explicit RemotePorts mapping.
		if len(plan.noMapping) > 0 {
			tunnelReady.Status = metav1.ConditionFalse
			tunnelReady.Reason = "PortNotMapped"
			tunnelReady.Message = fmt.Sprintf(
				"frp remote ports not configured for: %s",
				strings.Join(plan.noMapping, ", "),
			)
			conds = upsertCondition(conds, tunnelReady)
			return conds
		}

		// All endpoints known: deployment ready and mappings complete.
		tunnelReady.Status = metav1.ConditionTrue
		tunnelReady.Reason = "Ready"
		tunnelReady.Message = "tunnel deployment ready and all ports mapped"

	case "tailscale":
		if tunnel.Tailscale == nil {
			tunnelReady.Status = metav1.ConditionFalse
			tunnelReady.Reason = "InvalidConfig"
			tunnelReady.Message = "tailscale provider selected but no tailscale config"
			conds = upsertCondition(conds, tunnelReady)
			return conds
		}

		// Check if credentials are provided (optional, but if missing, tunnel won't authenticate).
		if tunnel.CredentialsSecretRef == nil {
			tunnelReady.Status = metav1.ConditionFalse
			tunnelReady.Reason = "NoCredentials"
			tunnelReady.Message = "no credentials secret provided; tailscale won't authenticate"
			conds = upsertCondition(conds, tunnelReady)
			return conds
		}

		// Deployment is ready and credentials are configured.
		tunnelReady.Status = metav1.ConditionTrue
		tunnelReady.Reason = "Ready"
		tunnelReady.Message = "tunnel deployment ready and authenticated"

	case "playit":
		if tunnel.Playit == nil {
			tunnelReady.Status = metav1.ConditionFalse
			tunnelReady.Reason = "InvalidConfig"
			tunnelReady.Message = "playit provider selected but no playit config"
			conds = upsertCondition(conds, tunnelReady)
			return conds
		}

		// Check if credentials are provided (playit requires authentication).
		if tunnel.CredentialsSecretRef == nil {
			tunnelReady.Status = metav1.ConditionFalse
			tunnelReady.Reason = "NoCredentials"
			tunnelReady.Message = "no credentials secret provided; playit requires authentication"
			conds = upsertCondition(conds, tunnelReady)
			return conds
		}

		// Deployment is ready; playit endpoint will arrive asynchronously.
		// Report AwaitingAddress while the tunnel pod hasn't yet reported an address.
		// This is optimistic: the condition flips based on whether the tunnel pod
		// has written to status.tunnelEndpoint. The agent or operator reconcile must
		// populate it separately.
		tunnelReady.Status = metav1.ConditionTrue
		tunnelReady.Reason = "Ready"
		tunnelReady.Message = "tunnel deployment ready; waiting for playit endpoint assignment"

	default:
		tunnelReady.Status = metav1.ConditionFalse
		tunnelReady.Reason = "UnknownProvider"
		tunnelReady.Message = fmt.Sprintf("unknown tunnel provider: %s", tunnel.Provider)
	}

	// Report informational conditions when tunnel settings don't apply to tunnel traffic.
	// These are set to True with an informational reason so users understand the limitation.
	var ignoredSettings []string
	if gs.Spec.Networking.Hostname != "" {
		ignoredSettings = append(ignoredSettings, "hostname")
	}
	if len(gs.Spec.Networking.SourceRanges) > 0 {
		ignoredSettings = append(ignoredSettings, "sourceRanges")
	}
	for _, po := range gs.Spec.Networking.PortOverrides {
		if po.NodePort != 0 {
			ignoredSettings = append(ignoredSettings, "nodePort")
			break
		}
	}
	if len(ignoredSettings) > 0 {
		infoCondition := metav1.Condition{
			Type:               "TunnelHostnameIgnored",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: gs.Generation,
			Reason:             "SettingIgnored",
			Message:            fmt.Sprintf("%s apply to the backing Service, not tunnel traffic", strings.Join(ignoredSettings, ", ")),
		}
		conds = upsertCondition(conds, infoCondition)
	}

	conds = upsertCondition(conds, tunnelReady)
	return conds
}

// removeCondition removes a condition by type from the list, or returns the list unchanged.
func removeCondition(conds []metav1.Condition, condType string) []metav1.Condition {
	out := make([]metav1.Condition, 0, len(conds))
	for i := range conds {
		if conds[i].Type != condType {
			out = append(out, conds[i])
		}
	}
	return out
}

// endpointsFromService lists the per-port externally reachable address
// for a Service. For ClusterIP we report the cluster IP; for NodePort
// we report the Service's declared port (the node IP is left to the API
// layer since the operator doesn't know which node a user will hit).
func endpointsFromService(svc *corev1.Service) []gameplanev1alpha1.GameServerEndpoint {
	out := make([]gameplanev1alpha1.GameServerEndpoint, 0, len(svc.Spec.Ports))
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
		ep := gameplanev1alpha1.GameServerEndpoint{
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
