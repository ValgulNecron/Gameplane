package main

import (
	"flag"
	"os"
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
		metricsAddr            string
		probeAddr              string
		enableLeaderElection   bool
		agentImage             string
		agentCABundle          string
		agentClientCert        string
		agentClientKey         string
		agentCASecretName      string
		agentCASecretNamespace string
		moduleNamespace        string
		moduleLocalRoot        string
	)
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "Address the metrics endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "Address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election.")
	flag.StringVar(&agentImage, "agent-image", "ghcr.io/valgulnecron/gameplane/agent:dev",
		"Image to use for the Gameplane agent sidecar injected into game pods.")
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

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	setupLog := ctrl.Log.WithName("setup")

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
		Scheme:                 mgr.GetScheme(),
		AgentImage:             agentImage,
		AgentCASecretName:      agentCASecretName,
		AgentCASecretNamespace: agentCASecretNamespace,
		AgentClient:            agentClient,
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
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "Restore")
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

	// Fleet metrics: report how many GameServers and Backups sit in each phase,
	// served on the manager's existing /metrics endpoint. The collectors read
	// the shared cache (populated by the controllers' watches above) at scrape
	// time, so registration order relative to Start doesn't matter.
	metrics.Registry.MustRegister(
		controller.NewGameServerCollector(mgr.GetClient()),
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
