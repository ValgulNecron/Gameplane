package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModuleSpec is the user-facing install request for a module. The
// reconciler pulls the referenced OCI artifact and materializes a
// GameTemplate owned by this Module.
type ModuleSpec struct {
	// Source references the ModuleSource this module is pulled from.
	// +kubebuilder:validation:Required
	Source corev1.LocalObjectReference `json:"source"`

	// Name is the module's logical name within the source. Must match
	// one of Source.spec.modules[].name. The created GameTemplate is
	// always named after the Module's metadata.name (not this field), so
	// a single module can be installed twice under different names.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Version pins a specific bundle version (semver). When empty the
	// reconciler tracks the source's LatestVersion and re-pulls when a
	// new version appears.
	// +optional
	Version string `json:"version,omitempty"`
}

// ModuleStatus reports the state of a module install.
type ModuleStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase is a coarse, UI-friendly summary.
	//
	//   - Pending: source not yet indexed, or version unresolved
	//   - Pulling: artifact pull in progress
	//   - Ready:   GameTemplate is current with the desired bundle
	//   - Failed:  see Conditions / LastError
	// +kubebuilder:validation:Enum=Pending;Pulling;Ready;Failed
	// +optional
	Phase string `json:"phase,omitempty"`

	// AppliedVersion is the bundle version currently materialized into
	// the GameTemplate. May lag spec.version during reconcile.
	// +optional
	AppliedVersion string `json:"appliedVersion,omitempty"`

	// AppliedDigest is the OCI manifest digest of the bundle that
	// produced the current GameTemplate, for auditability.
	// +optional
	AppliedDigest string `json:"appliedDigest,omitempty"`

	// AppliedTemplate is the name of the GameTemplate this Module owns.
	// Equal to Module.metadata.name on success.
	// +optional
	AppliedTemplate string `json:"appliedTemplate,omitempty"`

	// LastError is a short human-readable message for the most recent
	// failed reconcile. Cleared on success.
	// +optional
	LastError string `json:"lastError,omitempty"`

	// Conditions tracks Ready, Pulling, and any uninstall blockers.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchMergeKey:"type" patchStrategy:"merge"`
}

// Module phases.
const (
	ModulePhasePending = "Pending"
	ModulePhasePulling = "Pulling"
	ModulePhaseReady   = "Ready"
	ModulePhaseFailed  = "Failed"
)

// Module condition types.
const (
	ModuleConditionReady   = "Ready"
	ModuleConditionPulling = "Pulling"
)

// ModuleFinalizer guards delete while a GameServer references the
// module's GameTemplate.
const ModuleFinalizer = "kestrel.gg/module-finalizer"

// Labels stamped on the materialized GameTemplate so the API + UI can
// distinguish module-managed templates from manually-applied ones.
const (
	LabelManagedBy     = "kestrel.gg/managed-by"
	LabelModuleName    = "kestrel.gg/module-name"
	LabelModuleVersion = "kestrel.gg/module-version"
	LabelModuleDigest  = "kestrel.gg/module-digest"
	LabelModuleSource  = "kestrel.gg/module-source"

	// ManagedByModule is the value of LabelManagedBy when a Module owns
	// a GameTemplate.
	ManagedByModule = "Module"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=mod
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.source.name`
// +kubebuilder:printcolumn:name="Module",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.status.appliedVersion`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:subresource:status

// Module is an installed instance of a Kestrel module bundle. The
// reconciler maintains a GameTemplate owned by this object.
type Module struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModuleSpec   `json:"spec,omitempty"`
	Status ModuleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ModuleList is a list of Modules.
type ModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Module `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Module{}, &ModuleList{})
}
