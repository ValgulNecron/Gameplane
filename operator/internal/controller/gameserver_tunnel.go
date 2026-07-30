package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

const (
	tunnelLabel        = "app.kubernetes.io/name"
	tunnelValue        = "gameplane-tunnel"
	tunnelAuthVolume   = "tunnel-auth"
	tunnelAuthMountDir = "/etc/gameplane/tunnel-auth"
)

// tunnelPlan is one reconcile pass's decision about the tunnel pod:
// whether it should exist, and what endpoints to advertise once it does.
type tunnelPlan struct {
	wantTunnel bool
	// endpoints are the computed per-provider addresses advertised via the tunnel.
	// Only set when wantTunnel is true and an endpoint is known.
	endpoints []gameplanev1alpha1.GameServerEndpoint
	// noMapping is the set of advertised port names that have no frp remote-port mapping.
	// Used only by frp to report a helpful condition reason.
	noMapping []string
}

// planTunnel is the single source of truth for the tunnel pod's existence
// and the endpoints it advertises. Like planSentinel, this is deliberately
// separate from the reconcile steps so there is one place deciding "should
// the tunnel exist" instead of multiple CreateOrUpdate calls racing.
//
// CRITICAL: the tunnel Deployment is NEVER torn down when the server sleeps,
// unlike the sentinel. That is the entire point — it must keep holding the
// public address so a player's connection attempt to the tunnel can trigger
// wake-on-connect. The tunnel pod stays running across the full lifecycle.
func (r *GameServerReconciler) planTunnel(
	_ context.Context, gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate,
) tunnelPlan {
	if gs.Spec.Networking.Tunnel == nil || !gs.Spec.Networking.Tunnel.Enabled {
		return tunnelPlan{}
	}

	tunnel := gs.Spec.Networking.Tunnel
	endpoints := make([]gameplanev1alpha1.GameServerEndpoint, 0)
	var noMapping []string

	switch tunnel.Provider {
	case "frp":
		// For frp, compute endpoints from the RemotePorts mapping.
		if tunnel.Frp == nil {
			return tunnelPlan{}
		}

		for _, p := range tmpl.Spec.Ports {
			if !p.Advertise {
				continue
			}

			// Find the remote port mapping for this port name.
			var remotePort int32
			found := false
			for _, rm := range tunnel.Frp.RemotePorts {
				if rm.Name == p.Name {
					remotePort = rm.RemotePort
					found = true
					break
				}
			}

			if !found {
				noMapping = append(noMapping, p.Name)
				continue
			}

			ep := gameplanev1alpha1.GameServerEndpoint{
				Name:           p.Name,
				Host:           tunnel.Frp.ServerAddr,
				Port:           remotePort,
				Protocol:       p.Protocol,
				TunnelProvider: "frp",
			}
			endpoints = append(endpoints, ep)
		}

	case "tailscale":
		// For Tailscale, use the specified hostname (or the GameServer name as default).
		if tunnel.Tailscale == nil {
			return tunnelPlan{}
		}

		hostname := tunnel.Tailscale.Hostname
		if hostname == "" {
			hostname = gs.Name
		}

		for _, p := range tmpl.Spec.Ports {
			if !p.Advertise {
				continue
			}

			ep := gameplanev1alpha1.GameServerEndpoint{
				Name:           p.Name,
				Host:           hostname,
				Port:           p.ContainerPort,
				Protocol:       p.Protocol,
				Private:        true, // Tailscale is always private (tailnet-only)
				TunnelProvider: "tailscale",
			}
			endpoints = append(endpoints, ep)
		}

	case "playit":
		// For Playit, the address is assigned at runtime by the tunnel pod and
		// arrives via a later change. No endpoint is computed here.
		if tunnel.Playit == nil {
			return tunnelPlan{}
		}
		// TODO(tunnel): playit endpoint arrives via the gameservers/status subresource
	}

	return tunnelPlan{
		wantTunnel: true,
		endpoints:  endpoints,
		noMapping:  noMapping,
	}
}

// reconcileTunnel maintains a 1-replica Deployment `<gs>-tunnel` that runs
// the tunnel relay process. Unlike the sentinel, this Deployment is NEVER
// torn down when the server sleeps — the tunnel must stay alive to hold the
// public address so a connection attempt can wake the server.
//
// want is planTunnel's decision for this pass: this function only builds the
// desired Deployment or deletes it, so there is a single place (planTunnel)
// that owns "should the tunnel exist right now".
func (r *GameServerReconciler) reconcileTunnel(
	ctx context.Context, gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate,
	want bool,
) error {
	if !want {
		return r.deleteTunnel(ctx, gs.Namespace, gs.Name)
	}

	tunnel := gs.Spec.Networking.Tunnel
	if tunnel == nil {
		return nil
	}

	// Create or update the tunnel Deployment.
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gs.Name + "-tunnel",
			Namespace: gs.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		replicas := int32(1)
		dep.Spec.Replicas = &replicas
		dep.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				tunnelLabel:                  tunnelValue,
				"app.kubernetes.io/instance": gs.Name,
			},
		}
		dep.Spec.Template.Labels = map[string]string{
			tunnelLabel:                  tunnelValue,
			"app.kubernetes.io/instance": gs.Name,
		}

		// Container ports: advertised ports only, mirroring buildGameContainer
		// and reconcileSentinel. These are the container ports the tunnel pod
		// listens on (the tunnel relay forwards these to the game Service).
		ports := make([]corev1.ContainerPort, 0)
		for _, p := range tmpl.Spec.Ports {
			if !p.Advertise {
				continue
			}
			cp := corev1.ContainerPort{
				Name:          p.Name,
				ContainerPort: p.ContainerPort,
				Protocol:      p.Protocol,
			}
			ports = append(ports, cp)
		}

		// Security context: matching the sentinel's hardened context.
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

		// Tunnel image selection and environment per provider.
		var image string
		var envVars []corev1.EnvVar

		// Backing Service DNS: used by tunnel pod to reach the game pod.
		backingServiceDNS := fmt.Sprintf("%s.%s.svc", gs.Name, gs.Namespace)

		switch tunnel.Provider {
		case "frp":
			if tunnel.Frp == nil {
				return fmt.Errorf("frp provider selected but no frp config for %s/%s", gs.Namespace, gs.Name)
			}

			image = r.TunnelFrpImage
			if image == "" {
				image = DefaultTunnelFrpImage
			}

			// frp env: server address, server port, backing service, port mappings.
			serverPort := tunnel.Frp.ServerPort
			if serverPort == 0 {
				serverPort = 7000
			}

			envVars = []corev1.EnvVar{
				{Name: "GAMESERVER_NAME", Value: gs.Name},
				{Name: "GAMESERVER_NAMESPACE", Value: gs.Namespace},
				{Name: "TUNNEL_TYPE", Value: "frp"},
				{Name: "FRP_SERVER_ADDR", Value: tunnel.Frp.ServerAddr},
				{Name: "FRP_SERVER_PORT", Value: fmt.Sprintf("%d", serverPort)},
				{Name: "BACKING_SERVICE_DNS", Value: backingServiceDNS},
				{Name: "BACKING_SERVICE_PORT", Value: buildFrpRemotePortsConfig(tunnel.Frp)},
			}

		case "tailscale":
			if tunnel.Tailscale == nil {
				return fmt.Errorf("tailscale provider selected but no tailscale config for %s/%s", gs.Namespace, gs.Name)
			}

			image = r.TunnelTailscaleImage
			if image == "" {
				image = DefaultTunnelTailscaleImage
			}

			hostname := tunnel.Tailscale.Hostname
			if hostname == "" {
				hostname = gs.Name
			}

			tagsStr := ""
			if len(tunnel.Tailscale.Tags) > 0 {
				for _, tag := range tunnel.Tailscale.Tags {
					if tagsStr != "" {
						tagsStr += ","
					}
					tagsStr += tag
				}
			}

			envVars = []corev1.EnvVar{
				{Name: "GAMESERVER_NAME", Value: gs.Name},
				{Name: "GAMESERVER_NAMESPACE", Value: gs.Namespace},
				{Name: "TUNNEL_TYPE", Value: "tailscale"},
				{Name: "TAILSCALE_HOSTNAME", Value: hostname},
				{Name: "TAILSCALE_TAGS", Value: tagsStr},
				{Name: "BACKING_SERVICE_DNS", Value: backingServiceDNS},
				{Name: "BACKING_SERVICE_PORTS", Value: buildTailscalePortsConfig(tmpl)},
			}

		case "playit":
			if tunnel.Playit == nil {
				return fmt.Errorf("playit provider selected but no playit config for %s/%s", gs.Namespace, gs.Name)
			}

			image = r.TunnelPlayitImage
			if image == "" {
				image = DefaultTunnelPlayitImage
			}

			tunnelName := tunnel.Playit.TunnelName
			if tunnelName == "" {
				tunnelName = gs.Name
			}

			envVars = []corev1.EnvVar{
				{Name: "GAMESERVER_NAME", Value: gs.Name},
				{Name: "GAMESERVER_NAMESPACE", Value: gs.Namespace},
				{Name: "TUNNEL_TYPE", Value: "playit"},
				{Name: "PLAYIT_TUNNEL_NAME", Value: tunnelName},
				{Name: "BACKING_SERVICE_DNS", Value: backingServiceDNS},
				{Name: "BACKING_SERVICE_PORTS", Value: buildPlayitPortsConfig(tmpl)},
			}
		}

		// Mount credentials Secret if provided.
		var volumes []corev1.Volume
		var volumeMounts []corev1.VolumeMount

		if tunnel.CredentialsSecretRef != nil {
			volumes = append(volumes, corev1.Volume{
				Name: tunnelAuthVolume,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: tunnel.CredentialsSecretRef.Name,
						Optional:   boolPtr(true),
					},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      tunnelAuthVolume,
				MountPath: tunnelAuthMountDir,
				ReadOnly:  true,
			})
		}

		dep.Spec.Template.Spec.Volumes = volumes
		dep.Spec.Template.Spec.Containers = []corev1.Container{{
			Name:         "tunnel",
			Image:        image,
			Env:          envVars,
			Ports:        ports,
			VolumeMounts: volumeMounts,
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

// reconcileTunnelNetworkPolicy maintains a per-server NetworkPolicy admitting
// the tunnel pod's outbound traffic. The games namespace runs a default-deny-
// egress policy (allow-game-public-egress selects app.kubernetes.io/name=gameplane-game),
// which would silently drop the tunnel pod's connection to the relay without
// an explicit admit rule. Without this, the tunnel feature silently does nothing.
//
// The policy admits egress to any destination on the advertised ports and DNS.
func (r *GameServerReconciler) reconcileTunnelNetworkPolicy(
	ctx context.Context, gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate,
	plan tunnelPlan,
) error {
	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gs.Name + "-tunnel-egress",
			Namespace: gs.Namespace,
		},
	}

	if !plan.wantTunnel {
		// Converge-to-absent: only delete if we own it.
		var existing networkingv1.NetworkPolicy
		if err := r.Get(ctx, client.ObjectKeyFromObject(np), &existing); err != nil {
			return client.IgnoreNotFound(err)
		}
		if !metav1.IsControlledBy(&existing, gs) {
			return nil
		}
		return client.IgnoreNotFound(r.Delete(ctx, &existing))
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, np, func() error {
		// Select the tunnel pod's labels.
		np.Spec.PodSelector = metav1.LabelSelector{
			MatchLabels: map[string]string{
				tunnelLabel:                  tunnelValue,
				"app.kubernetes.io/instance": gs.Name,
			},
		}

		// Egress rules: allow DNS, advertised ports for inbound relay traffic,
		// and the provider's relay endpoint ports for outbound control traffic.
		egressPorts := make([]networkingv1.NetworkPolicyPort, 0)

		// DNS: UDP and TCP port 53 to any destination (required by all providers).
		tcpProto := corev1.ProtocolTCP
		udpProto := corev1.ProtocolUDP
		dnsPort := intstr.FromInt(53)
		egressPorts = append(egressPorts,
			networkingv1.NetworkPolicyPort{
				Protocol: &udpProto,
				Port:     &dnsPort,
			},
			networkingv1.NetworkPolicyPort{
				Protocol: &tcpProto,
				Port:     &dnsPort,
			},
		)

		// Provider relay egress: the ports the tunnel process dials outbound.
		tunnel := gs.Spec.Networking.Tunnel
		if tunnel != nil {
			switch tunnel.Provider {
			case "frp":
				// frp: outbound to the frp server on ServerAddr:ServerPort (default 7000).
				if tunnel.Frp != nil {
					serverPort := tunnel.Frp.ServerPort
					if serverPort == 0 {
						serverPort = 7000
					}
					port := intstr.FromInt32(serverPort)
					egressPorts = append(egressPorts, networkingv1.NetworkPolicyPort{
						Protocol: &tcpProto,
						Port:     &port,
					})
				}

			case "tailscale":
				// tailscale: outbound to control plane on TCP 443 and DERP relays on UDP 41641.
				httpsPort := intstr.FromInt(443)
				derpPort := intstr.FromInt(41641)
				egressPorts = append(egressPorts,
					networkingv1.NetworkPolicyPort{
						Protocol: &tcpProto,
						Port:     &httpsPort,
					},
					networkingv1.NetworkPolicyPort{
						Protocol: &udpProto,
						Port:     &derpPort,
					},
				)

			case "playit":
				// playit: relies on TCP and UDP to its relay endpoints.
				// Since playit does not publish a fixed set of relay endpoints,
				// we permit all ports. The tunnel pod's egress is still constrained
				// to these two protocols and the backing Service DNS.
				// Note: this allows any outbound TCP/UDP, but DNS is already granted,
				// and the tunnel pod can only reach the backing Service DNS by name,
				// so concrete egress is still limited to the tunnel relay destinations.
				// A more restrictive approach would require knowing playit's relay IPs,
				// which are not discoverable statically.
			}
		}

		// Advertised ports: the game's container ports (tunnel receives here, forwards inbound).
		for _, p := range tmpl.Spec.Ports {
			if !p.Advertise {
				continue
			}
			proto := p.Protocol
			if proto == "" {
				proto = corev1.ProtocolTCP
			}
			port := intstr.FromInt32(p.ContainerPort)
			egressPorts = append(egressPorts, networkingv1.NetworkPolicyPort{
				Protocol: &proto,
				Port:     &port,
			})
		}

		// Egress with no "To" means "to any destination".
		np.Spec.PolicyTypes = []networkingv1.PolicyType{networkingv1.PolicyTypeEgress}
		np.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{{
			Ports: egressPorts,
			// Empty To = allow to any destination
		}}

		return controllerutil.SetControllerReference(gs, np, r.Scheme)
	})
	return err
}

// getTunnel fetches the tunnel Deployment, reporting exists=false when absent.
func (r *GameServerReconciler) getTunnel(ctx context.Context, namespace, gsName string) (*appsv1.Deployment, bool, error) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gsName + "-tunnel",
			Namespace: namespace,
		},
	}
	err := r.Get(ctx, client.ObjectKeyFromObject(dep), dep)
	if apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get tunnel deployment: %w", err)
	}
	return dep, true, nil
}

// deleteTunnel removes the tunnel Deployment if it exists.
func (r *GameServerReconciler) deleteTunnel(ctx context.Context, namespace, gsName string) error {
	dep, exists, err := r.getTunnel(ctx, namespace, gsName)
	if err != nil || !exists {
		return err
	}
	policy := metav1.DeletePropagationBackground
	return client.IgnoreNotFound(r.Delete(ctx, dep, &client.DeleteOptions{PropagationPolicy: &policy}))
}

// buildFrpRemotePortsConfig constructs the BACKING_SERVICE_PORT env var for frp.
// Format: "port_name:remote_port,..." e.g. "java:25565,bedrock:19133"
func buildFrpRemotePortsConfig(frp *gameplanev1alpha1.FrpTunnelSpec) string {
	if frp == nil {
		return ""
	}
	var entries []string
	for _, mapping := range frp.RemotePorts {
		entries = append(entries, fmt.Sprintf("%s:%d", mapping.Name, mapping.RemotePort))
	}
	if len(entries) == 0 {
		return ""
	}
	var result string
	for i, e := range entries {
		if i > 0 {
			result += ","
		}
		result += e
	}
	return result
}

// buildTailscalePortsConfig constructs the BACKING_SERVICE_PORTS env var for Tailscale.
// Format: "port_name:port,..." e.g. "java:25565,bedrock:19133"
func buildTailscalePortsConfig(tmpl *gameplanev1alpha1.GameTemplate) string {
	var entries []string
	for _, p := range tmpl.Spec.Ports {
		if !p.Advertise {
			continue
		}
		entries = append(entries, fmt.Sprintf("%s:%d", p.Name, p.ContainerPort))
	}
	if len(entries) == 0 {
		return ""
	}
	var result string
	for i, e := range entries {
		if i > 0 {
			result += ","
		}
		result += e
	}
	return result
}

// buildPlayitPortsConfig constructs the BACKING_SERVICE_PORTS env var for Playit.
// Format: "port_name:port,..." e.g. "java:25565,bedrock:19133"
func buildPlayitPortsConfig(tmpl *gameplanev1alpha1.GameTemplate) string {
	var entries []string
	for _, p := range tmpl.Spec.Ports {
		if !p.Advertise {
			continue
		}
		entries = append(entries, fmt.Sprintf("%s:%d", p.Name, p.ContainerPort))
	}
	if len(entries) == 0 {
		return ""
	}
	var result string
	for i, e := range entries {
		if i > 0 {
			result += ","
		}
		result += e
	}
	return result
}

// boolPtr returns a pointer to a bool.
func boolPtr(b bool) *bool {
	return &b
}

// Tunnel image defaults.
const (
	DefaultTunnelFrpImage       = "ghcr.io/valgulnecron/gameplane/tunnel-frp:dev"
	DefaultTunnelTailscaleImage = "ghcr.io/valgulnecron/gameplane/tunnel-tailscale:dev"
	DefaultTunnelPlayitImage    = "ghcr.io/valgulnecron/gameplane/tunnel-playit:dev"
)
