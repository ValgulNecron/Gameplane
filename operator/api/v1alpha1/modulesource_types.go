package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModuleSourceSpec configures one OCI registry that hosts Kestrel
// modules. The operator periodically lists tags for each ModuleRef and
// caches the resulting catalog into status.modules.
type ModuleSourceSpec struct {
	// URL is the registry/repository prefix that holds module bundles
	// (e.g. "ghcr.io/kestrel-gg/modules"). Each Module in this source is
	// pushed under "<URL>/<name>:<version>".
	// +kubebuilder:validation:MinLength=1
	URL string `json:"url"`

	// Modules is the explicit list of modules served by this source. v1
	// keeps the listing explicit; anonymous discovery (catalog index
	// artifact) is reserved for future extensions.
	// +kubebuilder:validation:MinItems=1
	Modules []ModuleRef `json:"modules"`

	// PullSecretRef references a kubernetes.io/dockerconfigjson Secret
	// in the operator namespace. Used for private registries.
	// +optional
	PullSecretRef *corev1.LocalObjectReference `json:"pullSecretRef,omitempty"`

	// Insecure allows plain HTTP and skips TLS verification. Intended
	// for local kind/k3d registries; do not enable on production
	// clusters.
	// +optional
	Insecure bool `json:"insecure,omitempty"`

	// RefreshInterval is how often the controller re-indexes the
	// registry. Defaults to 1h. Minimum 1m.
	// +kubebuilder:default="1h"
	// +optional
	RefreshInterval metav1.Duration `json:"refreshInterval,omitempty"`
}

// ModuleRef identifies a module within a ModuleSource by its logical
// name. The full OCI reference is reconstructed as
// "<source.url>/<name>".
type ModuleRef struct {
	// Name is the module identifier within this source. Must match the
	// "name" field in the bundle's module.yaml.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// ModuleSourceStatus is populated by the operator from a successful
// registry index.
type ModuleSourceStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// LastSync is the time of the most recent successful index pull.
	// +optional
	LastSync *metav1.Time `json:"lastSync,omitempty"`

	// Conditions tracks Synced (last index attempt succeeded) and Ready
	// (a complete catalog is available).
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchMergeKey:"type" patchStrategy:"merge"`

	// Modules is the cached catalog for this source.
	// +optional
	Modules []ModuleEntry `json:"modules,omitempty"`
}

// ModuleEntry is one module's catalog snapshot from the registry. The
// operator fills these from each module's module.yaml + tag list.
type ModuleEntry struct {
	// Name is the logical module name (matches ModuleRef.Name).
	Name string `json:"name"`

	// DisplayName is the human-readable label from module.yaml.
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// Summary is the one-line catalog blurb from module.yaml.
	// +optional
	Summary string `json:"summary,omitempty"`

	// Game is the game-family identifier from module.yaml.
	// +optional
	Game string `json:"game,omitempty"`

	// Icon is either a relative filename (resolved against the bundle)
	// or a URL/data URI passed straight through to the UI.
	// +optional
	Icon string `json:"icon,omitempty"`

	// Reference is the registry/repo path for this module without a
	// tag (e.g. "ghcr.io/kestrel-gg/modules/minecraft-java").
	Reference string `json:"reference"`

	// Versions is the list of tags found in the registry, semver-sorted
	// in descending order. Non-semver tags are excluded.
	// +optional
	Versions []string `json:"versions,omitempty"`

	// LatestVersion is Versions[0] when populated.
	// +optional
	LatestVersion string `json:"latestVersion,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=msrc;modsource
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.spec.url`
// +kubebuilder:printcolumn:name="Synced",type=string,JSONPath=`.status.conditions[?(@.type=="Synced")].status`
// +kubebuilder:printcolumn:name="Last Sync",type=date,JSONPath=`.status.lastSync`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:subresource:status

// ModuleSource is a registry that Kestrel pulls module bundles from.
type ModuleSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModuleSourceSpec   `json:"spec,omitempty"`
	Status ModuleSourceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ModuleSourceList is a list of ModuleSources.
type ModuleSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ModuleSource `json:"items"`
}

// Condition types reported on ModuleSource.status.
const (
	// ModuleSourceConditionSynced is True when the most recent index
	// pull succeeded.
	ModuleSourceConditionSynced = "Synced"
	// ModuleSourceConditionReady is True when at least one full catalog
	// pull has completed successfully.
	ModuleSourceConditionReady = "Ready"
)

func init() {
	SchemeBuilder.Register(&ModuleSource{}, &ModuleSourceList{})
}
