// Package main is the entry point for the Gameplane operator.
package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
	"github.com/ValgulNecron/gameplane/operator/internal/agent"
	"github.com/ValgulNecron/gameplane/operator/internal/controller"
	"github.com/ValgulNecron/gameplane/operator/internal/modsrc"
)

var scheme = runtime.NewScheme()

// cidrListFlag implements flag.Value for a repeatable
// --game-ingress-from-cidr flag: each occurrence appends a CIDR to the
// list, rather than the stdlib flag package's default last-one-wins
// behavior for a flag registered more than once.
type cidrListFlag []string

func (f *cidrListFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *cidrListFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

// errUnknownAddressManager is returned for an --address-manager value outside
// the supported set. Startup fails on it rather than silently degrading to
// "none": a cluster admin who names a pool expects the request translated, and
// a typo'd flavor would otherwise leave every pool preference unapplied while
// the address manager quietly hands out default-pool addresses.
var errUnknownAddressManager = errors.New("unknown cluster address-manager flavor")

// validateAddressManager accepts only the flavors reconcileService knows how
// to translate a pool/address preference into.
func validateAddressManager(flavor string) error {
	switch flavor {
	case "metallb", "cilium", "none":
		return nil
	default:
		return fmt.Errorf("%w %q: want metallb, cilium or none", errUnknownAddressManager, flavor)
	}
}

// Version is the operator build version, overridden at build time via
// -ldflags. Compared against a module bundle's gameplaneMinVersion to refuse
// modules that need a newer operator. Mirrors api/cmd and agent/cmd.
var Version = "dev"

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(gameplanev1alpha1.AddToScheme(scheme))
	// CSI VolumeSnapshot types — backed by the volume-snapshot backup
	// strategy (BackupReconciler creates VolumeSnapshots; RestoreReconciler
	// reads them to seed a new server's data PVC).
	utilruntime.Must(snapshotv1.AddToScheme(scheme))
}

func main() {
	var (
		metricsAddr                      string
		probeAddr                        string
		enableLeaderElection             bool
		agentImage                       string
		agentImagePullPolicy             string
		configInitImage                  string
		resticImage                      string
		sentinelImage                    string
		tunnelFrpImage                   string
		tunnelTailscaleImage             string
		tunnelPlayitImage                string
		agentLogLevel                    string
		agentCABundle                    string
		agentClientCert                  string
		agentClientKey                   string
		agentCASecretName                string
		agentCASecretNamespace           string
		moduleNamespace                  string
		moduleLocalRoot                  string
		controlPlaneNamespace            string
		addressManager                   string
		metalLBNamespace                 string
		gameIngressPolicy                bool
		gameIngressFromCIDR              cidrListFlag
		captureEnabled                   bool
		captureDefaultRetention          int64
		captureMaxRetention              int64
		captureSidecarImage              string
		captureDefaultMaxDurationSeconds int64
		captureDefaultMaxSizeBytes       int64
	)
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "Address the metrics endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "Address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election.")
	flag.StringVar(&agentImage, "agent-image", "ghcr.io/valgulnecron/gameplane/agent:dev",
		"Image to use for the Gameplane agent sidecar injected into game pods.")
	flag.StringVar(&agentImagePullPolicy, "agent-image-pull-policy", "",
		"ImagePullPolicy for the agent sidecar container (Always, IfNotPresent, or Never). "+
			"Empty (default) leaves it unset so Kubernetes applies its usual default, preserving "+
			"today's behavior. Set explicitly when --agent-image tracks a floating tag (e.g. :edge): "+
			"Kubernetes defaults ImagePullPolicy to IfNotPresent for any non-\":latest\" tag, so without "+
			"this flag a node reuses whatever agent image it already cached and game pods run a stale "+
			"agent indefinitely.")
	flag.StringVar(&configInitImage, "config-init-image", controller.DefaultConfigInitImage,
		"Image for the init container that copies rendered config files onto the data volume. "+
			"Point at a private registry mirror for air-gapped installs.")
	flag.StringVar(&resticImage, "restic-image", controller.DefaultResticImage,
		"Image for the restic backup/restore Jobs. "+
			"Point at a private registry mirror for air-gapped installs.")
	flag.StringVar(&sentinelImage, "sentinel-image", controller.DefaultSentinelImage,
		"Image for the wake sentinel pod that holds advertised ports while a server is asleep. "+
			"Point at a private registry mirror for air-gapped installs.")
	flag.StringVar(&tunnelFrpImage, "tunnel-frp-image", controller.DefaultTunnelFrpImage,
		"Image for the frp tunnel relay pod that routes players through an frp server. "+
			"Point at a private registry mirror for air-gapped installs.")
	flag.StringVar(&tunnelTailscaleImage, "tunnel-tailscale-image", controller.DefaultTunnelTailscaleImage,
		"Image for the Tailscale tunnel relay pod that routes players through a Tailscale tailnet. "+
			"Point at a private registry mirror for air-gapped installs.")
	flag.StringVar(&tunnelPlayitImage, "tunnel-playit-image", controller.DefaultTunnelPlayitImage,
		"Image for the Playit tunnel relay pod that routes players through a Playit tunnel. "+
			"Point at a private registry mirror for air-gapped installs.")
	flag.BoolVar(&captureEnabled, "capture-enabled", false,
		"Enable the network capture feature cluster-wide. When false (the default), "+
			"the capture capability is disabled and cannot be enabled per-GameServer.")
	flag.Int64Var(&captureDefaultRetention, "capture-default-retention-seconds", 86400,
		"Default retention period for completed network captures, in seconds. "+
			"Defaults to 86400 (24 hours). Applies when a GameServer's spec.capture.retentionSeconds is not set.")
	flag.Int64Var(&captureMaxRetention, "capture-max-retention-seconds", 604800,
		"Maximum retention period for network captures, in seconds. "+
			"Defaults to 604800 (7 days). Clamps any higher retention request to this value.")
	flag.StringVar(&captureSidecarImage, "capture-sidecar-image", controller.DefaultCaptureSidecarImage,
		"Image for the network capture sidecar container injected when capture is enabled on a GameServer. "+
			"Point at a private registry mirror for air-gapped installs.")
	flag.Int64Var(&captureDefaultMaxDurationSeconds, "capture-default-max-duration-seconds", 300,
		"Default maximum duration for a single network capture, in seconds. "+
			"A capture requested without an explicit maxDuration uses this limit; the sidecar stops the capture "+
			"automatically when the duration is reached. Defaults to 300 (5 minutes).")
	flag.Int64Var(&captureDefaultMaxSizeBytes, "capture-default-max-size-bytes", 5368709120,
		"Default maximum size for a single network capture file, in bytes. "+
			"A capture requested without an explicit maxSize uses this limit; the sidecar stops the capture "+
			"automatically when the file reaches this size. Defaults to 5368709120 (5 GiB).")
	flag.StringVar(&agentLogLevel, "agent-log-level", "",
		"Log level (debug, info, warn, or error) injected into agent sidecars as GAMEPLANE_LOG_LEVEL. "+
			"Empty injects nothing (the agent defaults to info) and avoids rolling existing pods.")
	flag.StringVar(&moduleNamespace, "module-namespace", "gameplane-system",
		"Namespace where ModuleSource credential Secrets live.")
	flag.StringVar(&moduleLocalRoot, "module-local-root", "",
		"Base directory that local-type ModuleSources resolve their paths under. Empty disables local sources.")
	flag.StringVar(&agentCABundle, "agent-ca-bundle", "",
		"CA bundle that signs agent server certs (for operator → agent calls).")
	flag.StringVar(&agentClientCert, "agent-client-cert", "",
		"Client cert presented when calling the agent over mTLS.")
	flag.StringVar(&agentClientKey, "agent-client-key", "",
		"Client key for the agent client cert.")
	flag.StringVar(&agentCASecretName, "agent-ca-secret-name", "gameplane-agent-ca",
		"Name of the Secret holding the agent CA cert+key used to sign per-GameServer agent server certs.")
	flag.StringVar(&agentCASecretNamespace, "agent-ca-secret-namespace", "gameplane-system",
		"Namespace of the agent CA Secret.")
	controlPlaneNamespaceDefault := os.Getenv("POD_NAMESPACE")
	if controlPlaneNamespaceDefault == "" {
		controlPlaneNamespaceDefault = "gameplane-system"
	}
	flag.StringVar(&controlPlaneNamespace, "control-plane-namespace", controlPlaneNamespaceDefault,
		"Namespace where the operator runs and where cluster kubeconfig Secrets are stored.")
	addressManagerDefault := os.Getenv("GAMEPLANE_ADDRESS_MANAGER")
	if addressManagerDefault == "" {
		addressManagerDefault = "none"
	}
	flag.StringVar(&addressManager, "address-manager", addressManagerDefault,
		"Cluster load-balancer address manager (metallb, cilium, or none) the operator translates a "+
			"GameServer's spec.networking.addressPool/address preference for. metallb writes the "+
			"metallb.io annotations; cilium writes the gameplane.local/lb-pool label plus the "+
			"lbipam.cilium.io/ips annotation. none (the default) mutates no Service and instead reports "+
			"the unhonored request on the GameServer's AddressAssignment condition, so a pool preference "+
			"never silently falls back to the default pool. Also settable as GAMEPLANE_ADDRESS_MANAGER.")
	flag.StringVar(&metalLBNamespace, "metallb-namespace", "metallb-system",
		"Namespace MetalLB's IPAddressPool custom resources (metallb.io/v1beta1, namespaced) live in. "+
			"Only used when --address-manager=metallb: the operator GETs a requested pool directly there "+
			"to report PoolNotFound without waiting for a MetalLB event. \"metallb-system\" is MetalLB's "+
			"own install convention, not a contract, hence configurable.")
	flag.BoolVar(&gameIngressPolicy, "game-ingress-policy", true,
		"Reconcile a per-GameServer ingress NetworkPolicy admitting player traffic to the template's "+
			"advertised ports. When false, the operator ensures the policy is absent instead of merely "+
			"skipping it, so toggling this off converges existing GameServers.")
	flag.Var(&gameIngressFromCIDR, "game-ingress-from-cidr",
		"Source CIDR admitted to advertised game ports by the ingress NetworkPolicy. Repeatable. "+
			"Defaults to 0.0.0.0/0 (games are meant to be publicly reachable) when not supplied.")

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	setupLog := ctrl.Log.WithName("setup")

	if err := validateAddressManager(addressManager); err != nil {
		setupLog.Error(err, "invalid --address-manager value")
		os.Exit(1)
	}

	// Games are meant to be publicly reachable, so an unset
	// --game-ingress-from-cidr defaults to wide-open rather than an empty
	// (and therefore permission-less) list.
	if len(gameIngressFromCIDR) == 0 {
		gameIngressFromCIDR = cidrListFlag{"0.0.0.0/0"}
	}
	for i, cidr := range gameIngressFromCIDR {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			setupLog.Error(err, "invalid --game-ingress-from-cidr value", "cidr", cidr)
			os.Exit(1)
		}
		// ParseCIDR succeeds even when the address has host bits set (e.g.
		// "10.0.0.1/8"), returning the parsed network alongside it — use
		// that canonical form (e.g. "10.0.0.0/8") rather than the raw flag
		// value, so IPBlock.CIDR never carries host bits the apiserver
		// might reject or interpret surprisingly.
		gameIngressFromCIDR[i] = ipnet.String()
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "gameplane-operator.gameplane.local",
		// On a resource-constrained node (the homelab target, and the CI
		// runner) a hammered apiserver can push an informer's initial sync —
		// e.g. the backup controller's VolumeSnapshot watch — past the default
		// 2m CacheSyncTimeout. The manager then exits ("problem running
		// manager: failed to wait for ... caches to sync") and crash-loops,
		// stalling all reconciliation. A larger window lets it ride out the
		// slowness and start cleanly.
		Controller: config.Controller{CacheSyncTimeout: 5 * time.Minute},
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	agentClient, err := agent.New(agent.Config{
		CABundle:   agentCABundle,
		ClientCert: agentClientCert,
		ClientKey:  agentClientKey,
	})
	if err != nil {
		setupLog.Error(err, "unable to build agent client")
		os.Exit(1)
	}

	if err := (&controller.GameServerReconciler{
		Client:                 mgr.GetClient(),
		APIReader:              mgr.GetAPIReader(),
		Scheme:                 mgr.GetScheme(),
		AgentImage:             agentImage,
		AgentImagePullPolicy:   agentImagePullPolicy,
		ConfigInitImage:        configInitImage,
		SentinelImage:          sentinelImage,
		TunnelFrpImage:         tunnelFrpImage,
		TunnelTailscaleImage:   tunnelTailscaleImage,
		TunnelPlayitImage:      tunnelPlayitImage,
		AgentLogLevel:          agentLogLevel,
		AgentCASecretName:      agentCASecretName,
		AgentCASecretNamespace: agentCASecretNamespace,
		AgentClient:            agentClient,
		PodAttacher: &controller.StopAttachClient{
			Config:    mgr.GetConfig(),
			Clientset: kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		},
		AddressManager:                   addressManager,
		MetalLBNamespace:                 metalLBNamespace,
		GameIngressPolicyEnabled:         gameIngressPolicy,
		GameIngressFromCIDRs:             gameIngressFromCIDR,
		CaptureEnabled:                   captureEnabled,
		CaptureDefaultRetention:          captureDefaultRetention,
		CaptureMaxRetention:              captureMaxRetention,
		CaptureDefaultMaxDurationSeconds: captureDefaultMaxDurationSeconds,
		CaptureDefaultMaxSizeBytes:       captureDefaultMaxSizeBytes,
		CaptureSidecarImage:              captureSidecarImage,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "GameServer")
		os.Exit(1)
	}
	if err := (&controller.GameTemplateReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "GameTemplate")
		os.Exit(1)
	}

	if err := (&controller.BackupReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		Clientset:   kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		AgentClient: agentClient,
		ResticImage: resticImage,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "Backup")
		os.Exit(1)
	}
	if err := (&controller.BackupScheduleReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "BackupSchedule")
		os.Exit(1)
	}
	if err := (&controller.RestoreReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		ResticImage: resticImage,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "Restore")
		os.Exit(1)
	}

	// Network capture reconciler (manages NetworkCapture CRD lifecycle and sidecar interaction).
	captureClient := agent.NewCaptureClient(agentClient)
	if err := (&controller.NetworkCaptureReconciler{
		Client:                           mgr.GetClient(),
		Scheme:                           mgr.GetScheme(),
		SidecarClient:                    captureClient,
		CaptureEnabled:                   captureEnabled,
		CaptureSidecarImage:              captureSidecarImage,
		CaptureDefaultMaxDurationSeconds: captureDefaultMaxDurationSeconds,
		CaptureDefaultMaxSizeBytes:       captureDefaultMaxSizeBytes,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "NetworkCapture")
		os.Exit(1)
	}

	fetchOptions := modsrc.Options{LocalRoot: moduleLocalRoot}
	if err := (&controller.ModuleSourceReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		Namespace:    moduleNamespace,
		FetchOptions: fetchOptions,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "ModuleSource")
		os.Exit(1)
	}
	if err := (&controller.ModuleReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Namespace:       moduleNamespace,
		OperatorVersion: Version,
		FetchOptions:    fetchOptions,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "Module")
		os.Exit(1)
	}
	if err := (&controller.ClusterStatusReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		Namespace: controlPlaneNamespace,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "Cluster")
		os.Exit(1)
	}

	// Fleet metrics: report how many GameServers and Backups sit in each phase,
	// served on the manager's existing /metrics endpoint. The collectors read
	// the shared cache (populated by the controllers' watches above) at scrape
	// time, so registration order relative to Start doesn't matter.
	metrics.Registry.MustRegister(
		controller.NewGameServerCollector(mgr.GetClient()),
		controller.NewGameServerIdleCollector(mgr.GetClient()),
		controller.NewBackupCollector(mgr.GetClient()),
	)

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
