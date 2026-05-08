package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RestorePhase is a high-level state for a Restore job.
// +kubebuilder:validation:Enum=Pending;Suspending;Running;Resuming;Succeeded;Failed
type RestorePhase string

const (
	RestorePhasePending    RestorePhase = "Pending"
	RestorePhaseSuspending RestorePhase = "Suspending"
	RestorePhaseRunning    RestorePhase = "Running"
	RestorePhaseResuming   RestorePhase = "Resuming"
	RestorePhaseSucceeded  RestorePhase = "Succeeded"
	RestorePhaseFailed     RestorePhase = "Failed"
)

// RestoreSpec is the desired state of a one-shot restore.
type RestoreSpec struct {
	// BackupRef is the source Backup. Must live in the same namespace
	// and have status.phase=Succeeded with a non-empty snapshotID.
	BackupRef LocalObjectRef `json:"backupRef"`

	// ServerRef is the target GameServer. Its data volume is
	// overwritten from the backup. May differ from the Backup's
	// original serverRef (restoring into a fresh server).
	ServerRef LocalObjectRef `json:"serverRef"`
}

// RestoreStatus is the observed state of a Restore.
type RestoreStatus struct {
	// +optional
	Phase RestorePhase `json:"phase,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// SnapshotID is the restic snapshot used. Resolved from the
	// referenced Backup at Pending → Suspending transition and pinned
	// here so retention can't surprise us mid-run.
	// +optional
	SnapshotID string `json:"snapshotID,omitempty"`

	// StartTime / CompletionTime bracket the actual restore Job.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Message is a human-readable reason when Phase=Failed.
	// +optional
	Message string `json:"message,omitempty"`

	// Conditions carry detailed state transitions.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=rs
// +kubebuilder:printcolumn:name="Backup",type=string,JSONPath=`.spec.backupRef.name`
// +kubebuilder:printcolumn:name="Server",type=string,JSONPath=`.spec.serverRef.name`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Completed",type=date,JSONPath=`.status.completionTime`
// +kubebuilder:subresource:status

// Restore represents a one-shot restore of a Backup's snapshot into a
// GameServer's data volume. The target server is suspended for the
// duration of the restore Job and resumed on success.
type Restore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RestoreSpec   `json:"spec,omitempty"`
	Status RestoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RestoreList is a list of Restores.
type RestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Restore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Restore{}, &RestoreList{})
}
