package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GameServerPhase is a high-level state for a GameServer, derived from
// the underlying StatefulSet/Pod + agent heartbeat.
// +kubebuilder:validation:Enum=Pending;Starting;Running;Stopping;Stopped;Suspended;Failed
type GameServerPhase string

const (
	GameServerPhasePending   GameServerPhase = "Pending"
	GameServerPhaseStarting  GameServerPhase = "Starting"
	GameServerPhaseRunning   GameServerPhase = "Running"
	GameServerPhaseStopping  GameServerPhase = "Stopping"
	GameServerPhaseStopped   GameServerPhase = "Stopped"
	GameServerPhaseSuspended GameServerPhase = "Suspended"
	GameServerPhaseFailed    GameServerPhase = "Failed"
)

// GameServerSpec is the desired state of a single game server instance.
type GameServerSpec struct {
	// TemplateRef references a GameTemplate that provides defaults for
	// image, ports, probes, etc. Required.
	TemplateRef GameTemplateRef `json:"templateRef"`

	// Suspend, when true, scales the underlying StatefulSet to zero.
	// Data is preserved. Transitions the GameServer to Suspended.
	// +kubebuilder:default=false
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// StopGracePeriodSeconds bounds the soft-stop: when Suspend flips true
	// and the template declares a Lifecycle.Stop sequence, the operator runs
	// it over RCON and waits up to this long for the game to go not-ready
	// before scaling the StatefulSet to zero (it scales early once the game
	// has actually stopped). It has no effect when the template declares no
	// stop sequence. Defaults to 30s.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=600
	// +kubebuilder:default=30
	// +optional
	StopGracePeriodSeconds *int32 `json:"stopGracePeriodSeconds,omitempty"`

	// Image, when set, overrides GameTemplate.Spec.Image (and any image
	// resolved from Version). Useful for pinning a specific build or
	// running a fork.
	// +optional
	Image string `json:"image,omitempty"`

	// Version selects a GameTemplate.spec.versions[].id, pinning that
	// entry's image and appending its env (and provisioning its per-loader
	// mod volume). When the template declares versions and this is set, it
	// must match a catalog id or the server fails. Empty selects the
	// template's default version (the entry with default=true, else the
	// first); when the template declares no versions, this is ignored and
	// the server runs spec.image or the template Image.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=40
	// +optional
	Version string `json:"version,omitempty"`

	// Env is appended to (and overrides) the template's env vars.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Config holds wizard-driven values keyed by ConfigField.Name from
	// the referenced GameTemplate. The operator materializes these as
	// env vars or files per the template's ConfigSchema.
	// +kubebuilder:validation:MaxProperties=64
	// +kubebuilder:validation:XValidation:rule="self.all(k, self[k].size() <= 4096)",message="config values must be at most 4096 characters"
	// +optional
	Config map[string]string `json:"config,omitempty"`

	// Probes overrides the template's readiness/liveness/startup probes.
	// Each set probe replaces the corresponding template probe; unset
	// probes fall back to the template.
	// +optional
	Probes *GameProbesSpec `json:"probes,omitempty"`

	// Resources overrides compute resources from the template.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Storage overrides the template's storage layout.
	// +optional
	Storage *GameStorageSpec `json:"storage,omitempty"`

	// Networking controls how the game is exposed outside the cluster.
	// +optional
	Networking GameServerNetworking `json:"networking,omitempty"`

	// NodeSelector / Tolerations / Affinity are passed through to the
	// pod spec unchanged.
	// +kubebuilder:validation:MaxProperties=32
	// +kubebuilder:validation:XValidation:rule="self.all(k, self[k].size() <= 253)",message="nodeSelector values must be at most 253 characters"
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// BackupPolicy inlines a BackupSchedule for this server. When set,
	// the operator creates/maintains a BackupSchedule owned by this
	// GameServer. Remove to stop scheduled backups.
	// +optional
	BackupPolicy *InlineBackupPolicy `json:"backupPolicy,omitempty"`

	// ServiceAccountName, when set, overrides the SA the pod runs as.
	// By default the operator creates a per-GameServer ServiceAccount
	// (`<name>-agent`) whose only grant is patching this GameServer's
	// status — the agent sidecar's heartbeat needs it. Overriding with
	// an SA that lacks that grant disables heartbeats (the server then
	// never reports Running).
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Mods pins the mods this server loads, for games whose server downloads
	// its own mods by id (see GameTemplate capabilities.mods.idList). Ignored
	// by file-drop games.
	// +optional
	Mods *GameServerModsSpec `json:"mods,omitempty"`

	// Idle configures automatic sleep while nobody is playing. Opt-in.
	// +optional
	Idle *IdleSpec `json:"idle,omitempty"`
}

// IdleSpec configures idle auto-sleep: the operator scales the server down
// once it has reported no online players for AfterMinutes, and brings it back
// on a WakeWindows cron tick or an explicit wake request.
//
// Sleeping reuses the same graceful path as spec.suspend — the module-declared
// stop sequence runs first, so the world is saved — and retains the data
// volume. It is deliberately *not* expressed as spec.suspend: that field is
// the user's own power switch, and conflating the two would let a wake window
// resurrect a server its owner had deliberately turned off.
//
// A server whose game reports no player count can never satisfy the trigger
// (unknown is not zero — see AgentStatus.PlayersOnline). Rather than sleeping
// forever without explanation, the controller surfaces that as an
// IdleEligible=False condition.
type IdleSpec struct {
	// Enabled turns idle auto-sleep on for this server.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// AfterMinutes is how long the server must continuously report zero
	// online players before it is put to sleep. The floor of 5 minutes keeps
	// a server from flapping between the last player leaving and the next
	// arriving.
	// +kubebuilder:validation:Minimum=5
	// +kubebuilder:validation:Maximum=1440
	// +kubebuilder:default=30
	// +optional
	AfterMinutes *int32 `json:"afterMinutes,omitempty"`

	// WakeWindows are cron expressions, in the cluster's timezone, at which a
	// sleeping server is woken back up — the scheduled counterpart to the
	// dashboard's Wake button, so a server can be up before players arrive
	// without anyone holding write access.
	//
	// Same five-field form and structural guard as BackupScheduleSpec.Schedule;
	// the controller parses with robfig/cron/v3 and reports an unparseable
	// entry as a failed condition. A window never wakes a server the user
	// suspended by hand.
	//
	// Bounded (and each entry length-capped) deliberately: an unbounded list
	// carrying a per-item rule blows the apiserver's CEL cost budget and gets
	// the whole CRD rejected at install time.
	// +kubebuilder:validation:MaxItems=8
	// +kubebuilder:validation:items:MinLength=9
	// +kubebuilder:validation:items:MaxLength=64
	// +kubebuilder:validation:items:Pattern=`^\S+\s+\S+\s+\S+\s+\S+\s+\S+(\s+\S+)?$`
	// +listType=atomic
	// +optional
	WakeWindows []string `json:"wakeWindows,omitempty"`
}

// GameServerModsSpec is the set of mods selected for a server whose game
// installs mods by id (see GameTemplate capabilities.mods.idList).
type GameServerModsSpec struct {
	// IDs are the provider-native mod ids, in order.
	// +kubebuilder:validation:MaxItems=200
	// +optional
	IDs []ModRef `json:"ids,omitempty"`
}

// ModRef is one provider-native mod id, optionally labeled for display.
type ModRef struct {
	// ID is the provider-native mod id (e.g. a CurseForge project id).
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9._-]+$`
	ID string `json:"id"`
	// Name is a display label only. It is never projected into the game —
	// it exists so the dashboard can render the list without a registry
	// round-trip.
	// +kubebuilder:validation:MaxLength=128
	// +optional
	Name string `json:"name,omitempty"`
}

// GameTemplateRef identifies a GameTemplate by name.
type GameTemplateRef struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// GameServerNetworking controls service + ingress for the game.
type GameServerNetworking struct {
	// Expose controls the Service type fronting the game pod.
	// - "ClusterIP": reachable only from within the cluster (default)
	// - "NodePort": exposed on a high port on every node
	// - "LoadBalancer": request an external LB (cloud)
	// - "Hostport": advertise a specific host port via HostPort pod spec
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer;Hostport
	// +kubebuilder:default=ClusterIP
	// +optional
	Expose string `json:"expose,omitempty"`

	// Hostname is an optional DNS name the operator may advertise via
	// ingress / external-dns annotations on the Service. Must be an
	// RFC 1123 hostname: dotted labels of 1-63 alphanumeric characters
	// or hyphens, no leading/trailing hyphen, at most 253 characters
	// total. The operator does not create the DNS record itself.
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^([a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?)(\.[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?)*$`
	// +optional
	Hostname string `json:"hostname,omitempty"`

	// ServiceAnnotations are merged into the fronting Service's
	// annotations (useful for LoadBalancer-specific config, external-dns
	// hooks, etc.).
	// +kubebuilder:validation:MaxProperties=32
	// +kubebuilder:validation:XValidation:rule="self.all(k, self[k].size() <= 4096)",message="serviceAnnotations values must be at most 4096 characters"
	// +optional
	ServiceAnnotations map[string]string `json:"serviceAnnotations,omitempty"`

	// PortOverrides lets the user pin a specific NodePort or override
	// the Service port for a named template port.
	// +optional
	PortOverrides []PortOverride `json:"portOverrides,omitempty"`

	// SourceRanges is an IP allow-list (CIDRs) for the fronting Service. It
	// maps to service.spec.loadBalancerSourceRanges and therefore only takes
	// effect when Expose=LoadBalancer (cloud LBs that honor it); it is
	// ignored for ClusterIP/NodePort. Empty allows all clients.
	// +kubebuilder:validation:MaxItems=20
	// +kubebuilder:validation:XValidation:rule="self.all(c, c.contains('/'))",message="each sourceRange must be a CIDR, e.g. 203.0.113.0/24"
	// +listType=atomic
	// +optional
	SourceRanges []string `json:"sourceRanges,omitempty"`
}

// PortOverride pins or remaps one of the template's declared ports.
type PortOverride struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// ServicePort overrides the external Service port.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	ServicePort int32 `json:"servicePort,omitempty"`

	// NodePort pins the NodePort when Networking.Expose is NodePort.
	// +kubebuilder:validation:Minimum=30000
	// +kubebuilder:validation:Maximum=32767
	// +optional
	NodePort int32 `json:"nodePort,omitempty"`
}

// InlineBackupPolicy is the subset of BackupScheduleSpec a user can
// configure when enabling backups directly from GameServer. The
// operator materializes this into a managed BackupSchedule.
type InlineBackupPolicy struct {
	// Schedule is a standard cron expression; same structural guard as
	// BackupScheduleSpec.Schedule so typos are rejected at admission
	// instead of failing inside the reconcile loop of the materialized
	// BackupSchedule.
	// +kubebuilder:validation:MinLength=9
	// +kubebuilder:validation:Pattern=`^\S+\s+\S+\s+\S+\s+\S+\s+\S+(\s+\S+)?$`
	Schedule string `json:"schedule"`

	// RepoRef points at a BackupRepo (cluster-scoped resource, TBD) or
	// inline secret with restic repo + credentials.
	RepoRef SecretKeySelector `json:"repoRef"`

	// +optional
	Retention *BackupRetention `json:"retention,omitempty"`

	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// GameServerStatus is the observed state of a GameServer.
type GameServerStatus struct {
	// +optional
	Phase GameServerPhase `json:"phase,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions carry detailed state transitions. Standard Ready,
	// Progressing, and Healthy conditions are surfaced in the UI.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Endpoints lists the externally reachable addresses advertised for
	// this server (populated once the Service is reconciled).
	// +optional
	Endpoints []GameServerEndpoint `json:"endpoints,omitempty"`

	// Agent reports runtime info sourced from the in-pod agent sidecar.
	// +optional
	Agent *AgentStatus `json:"agent,omitempty"`

	// StartedAt is the wall-clock time the game container was last
	// observed as Ready.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// LastBackupTime is the completion time of the most recent
	// successful backup of this server.
	// +optional
	LastBackupTime *metav1.Time `json:"lastBackupTime,omitempty"`

	// Idle reports the observed state of idle auto-sleep (spec.idle).
	// +optional
	Idle *IdleStatus `json:"idle,omitempty"`
}

// IdleStatus is the observed state of idle auto-sleep. It is a read model:
// the authoritative sleep marker is an operator-owned annotation, so the
// decision survives a status subresource being reset.
type IdleStatus struct {
	// EmptySince is when the server first reported a *fresh* zero player
	// count. It is cleared the moment a player appears or the count goes
	// unknown, so the elapsed time since it is the true continuous-empty
	// duration. Nil while the server is not accumulating idle time.
	// +optional
	EmptySince *metav1.Time `json:"emptySince,omitempty"`

	// Asleep reports whether the operator has scaled this server down for
	// idleness. While true the phase reads Suspended with reason IdleAsleep;
	// it is distinct from spec.suspend, which is the user's own stop.
	// +optional
	Asleep bool `json:"asleep,omitempty"`

	// AsleepSince is when the server was put to sleep.
	// +optional
	AsleepSince *metav1.Time `json:"asleepSince,omitempty"`

	// LastWakeTime is when it was last woken, by a wake window or an
	// explicit request.
	// +optional
	LastWakeTime *metav1.Time `json:"lastWakeTime,omitempty"`

	// Reason explains the current idle state in one short phrase — why the
	// server is not accumulating idle time, or what woke it.
	// +kubebuilder:validation:MaxLength=256
	// +optional
	Reason string `json:"reason,omitempty"`
}

// GameServerEndpoint is a single externally reachable (host, port)
// associated with a named template port.
type GameServerEndpoint struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Port int32  `json:"port"`
	// +optional
	Protocol corev1.Protocol `json:"protocol,omitempty"`
}

// AgentStatus is runtime state the sidecar reports via status updates.
type AgentStatus struct {
	// +optional
	Version string `json:"version,omitempty"`

	// +optional
	LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`

	// PlayersOnline is the count reported by the game protocol (RCON /
	// server query). Null (absent) means "unknown" — the game exposes no
	// player source, or the query failed. Consumers must not treat unknown
	// as zero: idle auto-sleep depends on exactly that distinction, and the
	// dashboard sums this field across servers, which is why the agent
	// patches null rather than a -1 sentinel (see
	// agent/internal/heartbeat.sendOnce).
	// +optional
	PlayersOnline *int32 `json:"playersOnline,omitempty"`

	// PlayersMax is the configured max player count, if known.
	// +optional
	PlayersMax *int32 `json:"playersMax,omitempty"`

	// GameVersion is the version string the running game reports.
	// +optional
	GameVersion string `json:"gameVersion,omitempty"`

	// CPUMillicores is the agent's own cgroup CPU usage averaged over the
	// last heartbeat interval, in millicores. nil means "unknown".
	// +optional
	CPUMillicores *int64 `json:"cpuMillicores,omitempty"`

	// CPULimitMillicores is the pod's cgroup CPU limit in millicores, or
	// nil when unlimited / unknown.
	// +optional
	CPULimitMillicores *int64 `json:"cpuLimitMillicores,omitempty"`

	// MemoryBytes is the pod's current cgroup memory usage in bytes. nil
	// means "unknown".
	// +optional
	MemoryBytes *int64 `json:"memoryBytes,omitempty"`

	// MemoryLimitBytes is the pod's cgroup memory limit in bytes, or nil
	// when unlimited / unknown.
	// +optional
	MemoryLimitBytes *int64 `json:"memoryLimitBytes,omitempty"`

	// DiskUsedBytes is the used space on the game data volume (statfs).
	// nil means "unknown".
	// +optional
	DiskUsedBytes *int64 `json:"diskUsedBytes,omitempty"`

	// DiskTotalBytes is the total size of the game data volume (statfs).
	// nil means "unknown".
	// +optional
	DiskTotalBytes *int64 `json:"diskTotalBytes,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=gs
// +kubebuilder:printcolumn:name="Template",type=string,JSONPath=`.spec.templateRef.name`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Players",type=integer,JSONPath=`.status.agent.playersOnline`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// Asleep distinguishes an idle auto-sleep from a user's own stop, which the
// reused Suspended phase cannot. priority=1 keeps it out of the default table
// (it only shows under `-o wide`) so existing `kubectl get gs` output is
// unchanged.
// +kubebuilder:printcolumn:name="Asleep",type=boolean,JSONPath=`.status.idle.asleep`,priority=1
// +kubebuilder:subresource:status

// GameServer is a single running game server instance.
type GameServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GameServerSpec   `json:"spec,omitempty"`
	Status GameServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GameServerList is a list of GameServers.
type GameServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GameServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GameServer{}, &GameServerList{})
}
