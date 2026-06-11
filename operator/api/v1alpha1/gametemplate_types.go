package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GameTemplateSpec defines a reusable blueprint for a game server.
// Users instantiate a GameTemplate by creating a GameServer that
// references it.
// +kubebuilder:validation:XValidation:rule="!has(self.consoleMode) || self.consoleMode != 'rcon' || (has(self.rcon) && (!has(self.rcon.protocol) || self.rcon.protocol != 'none'))",message="consoleMode 'rcon' requires spec.rcon with a protocol other than 'none'"
type GameTemplateSpec struct {
	// DisplayName is a human-friendly label shown in the dashboard.
	// +kubebuilder:validation:MinLength=1
	DisplayName string `json:"displayName"`

	// Game is the canonical game identifier (e.g. "minecraft-java",
	// "valheim", "factorio"). Used as a grouping key in the UI catalog.
	// +kubebuilder:validation:MinLength=1
	Game string `json:"game"`

	// Version is the template revision (e.g. "1.0.0"). Bump when changing
	// defaults in ways that existing GameServers should opt into.
	// +kubebuilder:validation:MinLength=1
	Version string `json:"version"`

	// Icon is an optional URL or data URI shown in the catalog.
	// +optional
	Icon string `json:"icon,omitempty"`

	// Description is free-form markdown describing the template.
	// +optional
	Description string `json:"description,omitempty"`

	// Image is the default container image (e.g.
	// "itzg/minecraft-server:2025.1.0"). Can be overridden by GameServer.
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Command / Args override the container image entrypoint when set.
	// +optional
	Command []string `json:"command,omitempty"`
	// +optional
	Args []string `json:"args,omitempty"`

	// Env is the default environment for the game container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Ports declares exposed ports on the game container.
	// +optional
	Ports []GamePort `json:"ports,omitempty"`

	// Storage describes the default persistent storage layout.
	// +optional
	Storage GameStorageSpec `json:"storage,omitempty"`

	// Resources are the default compute resources for the game container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// RCON declares the remote-console protocol used by this game, if any.
	// +optional
	RCON *RCONSpec `json:"rcon,omitempty"`

	// ConsoleMode controls how the dashboard's Console tab attaches to
	// the running game.
	//
	//   - "rcon": send line-based RCON commands (default when RCON is set
	//     and its protocol is not "none").
	//   - "pty":  attach to the game container's stdin/stdout via the
	//     Kubernetes pod-attach API. Requires the container to be started
	//     with tty=true and stdin=true (the operator sets these). Switching
	//     to pty after the pod exists requires a pod recreate.
	//   - "none": disable the Console tab for this game.
	//
	// When unset, the operator defaults to "rcon" if RCON.Protocol is set
	// to anything other than "none", and "none" otherwise.
	// +kubebuilder:validation:Enum=rcon;pty;none
	// +optional
	ConsoleMode string `json:"consoleMode,omitempty"`

	// LogPath is the file holding the game's primary log, as seen from
	// inside the pod (e.g. "/data/logs/latest.log"). It must live under
	// the shared data volume (Storage.MountPath) so the agent sidecar
	// can tail it for the dashboard's Logs tab. Leave empty for games
	// that only log to stdout — the Logs tab is then unavailable.
	// +optional
	LogPath string `json:"logPath,omitempty"`

	// Probes are the default readiness/liveness/startup probes for the
	// game container. Operator supplies sane defaults when unset.
	// +optional
	Probes *GameProbesSpec `json:"probes,omitempty"`

	// ConfigSchema declares user-tunable fields surfaced in the Create
	// Server wizard. The operator resolves GameServer.spec.config
	// against this schema (applying defaults, validating types/enums)
	// and sets each resolved value as an env var on the game container.
	// Fields with target=file are not implemented yet and fail the
	// GameServer if a value is supplied.
	// +optional
	ConfigSchema []ConfigField `json:"configSchema,omitempty"`

	// Agent tunes the sidecar deployed alongside the game container.
	// +optional
	Agent *AgentSpec `json:"agent,omitempty"`
}

// GamePort is a single exposed port.
type GamePort struct {
	// Name is a DNS-label port identifier (e.g. "game", "query", "rcon").
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// ContainerPort is the port the game listens on inside the pod.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ContainerPort int32 `json:"containerPort"`

	// Protocol is one of TCP, UDP. Defaults to TCP.
	// +kubebuilder:default=TCP
	// +kubebuilder:validation:Enum=TCP;UDP
	// +optional
	Protocol corev1.Protocol `json:"protocol,omitempty"`

	// Advertise controls whether this port is exposed to users via
	// GameServer.Spec.Networking. RCON and query ports are typically
	// not advertised publicly.
	// +kubebuilder:default=true
	// +optional
	Advertise bool `json:"advertise,omitempty"`
}

// GameStorageSpec describes the persistent storage layout for a game.
type GameStorageSpec struct {
	// Size is the default PVC size (e.g. "10Gi").
	// +optional
	Size resource.Quantity `json:"size,omitempty"`

	// StorageClassName, when set, pins the PVC to a specific StorageClass.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`

	// MountPath is where the game persistent volume is mounted inside
	// the game container (e.g. "/data").
	// +kubebuilder:default=/data
	// +optional
	MountPath string `json:"mountPath,omitempty"`
}

// RCONSpec describes the remote-console protocol used by the game.
type RCONSpec struct {
	// Protocol is the wire protocol. "source" is the Valve/Minecraft RCON
	// protocol; other games use different mechanisms (telnet, HTTP, etc.)
	// and are implemented by module-specific code paths.
	// +kubebuilder:default=source
	// +kubebuilder:validation:Enum=source;telnet;http;none
	// +optional
	Protocol string `json:"protocol,omitempty"`

	// Port is the TCP port RCON listens on inside the pod.
	// +optional
	Port int32 `json:"port,omitempty"`

	// PasswordSecretRef references a Secret+key containing the RCON
	// password. If unset, the operator generates a password and stores
	// it in an auto-managed Secret.
	// +optional
	PasswordSecretRef *SecretKeySelector `json:"passwordSecretRef,omitempty"`
}

// GameProbesSpec are the default probes for the game container.
type GameProbesSpec struct {
	// +optional
	Readiness *corev1.Probe `json:"readiness,omitempty"`
	// +optional
	Liveness *corev1.Probe `json:"liveness,omitempty"`
	// +optional
	Startup *corev1.Probe `json:"startup,omitempty"`
}

// AgentSpec tunes the Kestrel sidecar that runs alongside the game.
type AgentSpec struct {
	// Image overrides the default agent image. Normally set by the
	// operator to the image matching its own build.
	// +optional
	Image string `json:"image,omitempty"`

	// Resources overrides agent resources.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// ConfigField is a single user-tunable setting surfaced in the wizard.
type ConfigField struct {
	// Name is the field identifier (also used as an env var when
	// Target is "env").
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// DisplayName is shown in the UI.
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// Description explains the field to end users.
	// +optional
	Description string `json:"description,omitempty"`

	// Type controls the input widget in the wizard.
	// +kubebuilder:validation:Enum=string;int;bool;enum;password
	// +kubebuilder:default=string
	Type string `json:"type"`

	// Default is the default value (as a string) rendered in the wizard.
	// +optional
	Default string `json:"default,omitempty"`

	// Enum restricts valid values when Type=enum.
	// +optional
	Enum []string `json:"enum,omitempty"`

	// Required, when true, blocks wizard submission if unset.
	// +optional
	Required bool `json:"required,omitempty"`

	// Target controls where the value is applied. "env" (default) sets
	// an env var on the game container; "file" writes to a file rendered
	// from a template (module-specific).
	// +kubebuilder:validation:Enum=env;file
	// +kubebuilder:default=env
	// +optional
	Target string `json:"target,omitempty"`
}

// SecretKeySelector references a key inside a namespaced Secret. Mirrors
// corev1.SecretKeySelector but narrowed to the fields we support.
type SecretKeySelector struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// GameTemplateStatus is intentionally minimal — templates are mostly
// static configuration. The operator populates ObservedGeneration so
// controllers reading templates can detect spec updates.
type GameTemplateStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// InUseCount is the number of GameServers currently referencing
	// this template.
	// +optional
	InUseCount int32 `json:"inUseCount,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=gtmpl;gametmpl
// +kubebuilder:printcolumn:name="Game",type=string,JSONPath=`.spec.game`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.version`
// +kubebuilder:printcolumn:name="In Use",type=integer,JSONPath=`.status.inUseCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:subresource:status

// GameTemplate is a cluster-scoped blueprint for a specific game. A
// GameTemplate is distributed as part of a Kestrel module (OCI artifact)
// and instantiated via GameServer.
type GameTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GameTemplateSpec   `json:"spec,omitempty"`
	Status GameTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GameTemplateList is a list of GameTemplates.
type GameTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GameTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GameTemplate{}, &GameTemplateList{})
}
