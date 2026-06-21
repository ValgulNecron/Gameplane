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
	// server query). -1 means "unknown / not supported by this game".
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
