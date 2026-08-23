package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CapturePhase is a high-level state for a NetworkCapture session.
// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed;Expired
type CapturePhase string

// CapturePhase constants define the possible values for a NetworkCapture's phase.
const (
	// CapturePhasePending: NetworkCapture CRD created; awaiting operator to transition to Running.
	CapturePhasePending CapturePhase = "Pending"

	// CapturePhaseRunning: sidecar is actively capturing packets.
	CapturePhaseRunning CapturePhase = "Running"

	// CapturePhaseCompleted: capture finished successfully (manually stopped by user,
	// or max-duration/max-size limit reached).
	CapturePhaseCompleted CapturePhase = "Completed"

	// CapturePhaseFailed: capture terminated abnormally (sidecar crash, disk full, invalid filter, pod restart mid-capture, etc.).
	CapturePhaseFailed CapturePhase = "Failed"

	// CapturePhaseExpired: capture has been auto-deleted by operator because TTL window elapsed.
	CapturePhaseExpired CapturePhase = "Expired"
)

// NetworkCaptureSpec is the desired state of a packet capture session.
type NetworkCaptureSpec struct {
	// ServerRef is the GameServer being captured. Immutable once created. Required.
	// +kubebuilder:validation:Required
	ServerRef corev1.LocalObjectReference `json:"serverRef"`

	// Filter is a pcap-filter(7) expression restricting captured packets.
	// Examples: "tcp port 8080", "host 192.168.1.5 and udp", "tcp[tcpflags] & tcp-syn != 0"
	// Optional. If omitted, a default filter restricts capture to the GameServer's own advertised ports.
	// Immutable once created (filter is set at capture start time, cannot change mid-capture).
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=1024
	Filter *string `json:"filter,omitempty"`

	// MaxDuration is the maximum runtime for the capture, expressed as a Go duration.
	// If the capture runs for longer than this duration, it stops automatically.
	// Required. Must be > 0.
	// +kubebuilder:validation:Required
	MaxDuration *metav1.Duration `json:"maxDuration"`

	// MaxSize is the maximum file size for the capture (e.g. 100Mi, 5Gi).
	// If the capture file grows larger than this size, it stops automatically.
	// Required. Must be > 0.
	// +kubebuilder:validation:Required
	MaxSize *resource.Quantity `json:"maxSize"`

	// TTLSecondsAfterFinished is the auto-expiration window after the capture completes,
	// in seconds. After status.completionTime + TTLSecondsAfterFinished, the NetworkCapture
	// is deleted by the operator. Optional; if omitted, the cluster default
	// (from Helm value capture.defaultRetentionSeconds) is used. The cluster-wide maximum
	// (capture.maxRetentionSeconds) constrains this field; requests exceeding the max are
	// rejected at API tier with HTTP 400. The cluster maximum of 604800 seconds (7 days) is
	// a storage-limitation-informed default, not a legal requirement.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=60
	// +kubebuilder:validation:Maximum=604800
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`
}

// NetworkCaptureStatus is the observed state of a packet capture session.
type NetworkCaptureStatus struct {
	// Phase is the current lifecycle phase of the capture.
	// Valid values: Pending, Running, Completed, Failed, Expired.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed;Expired
	Phase CapturePhase `json:"phase"`

	// StartTime is the RFC3339 timestamp when the capture started
	// (phase transitioned from Pending to Running). Populated by the operator when transitioning to Running.
	// +kubebuilder:validation:Optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the RFC3339 timestamp when the capture stopped
	// (phase transitioned to Completed or Failed). Populated by the operator when transitioning to a terminal phase.
	// +kubebuilder:validation:Optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// PacketsWritten is the number of packets captured and written to the output file.
	// Updated as the capture progresses; final value recorded when capture completes.
	// +kubebuilder:validation:Optional
	PacketsWritten int64 `json:"packetsWritten,omitempty"`

	// BytesWritten is the total size of the captured file, in bytes.
	// Updated as the capture progresses; final value recorded when capture completes.
	// +kubebuilder:validation:Optional
	BytesWritten *resource.Quantity `json:"bytesWritten,omitempty"`

	// Message is a human-readable status or error message.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=512
	Message string `json:"message,omitempty"`

	// Conditions contains structured reasons for current phase state.
	// +kubebuilder:validation:Optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=cap;caps
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Server",type=string,JSONPath=`.spec.serverRef.name`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Packets",type=integer,JSONPath=`.status.packetsWritten`
// +kubebuilder:printcolumn:name="Size",type=string,JSONPath=`.status.bytesWritten`
// +kubebuilder:printcolumn:name="Start",type=date,JSONPath=`.status.startTime`
// +kubebuilder:rbac:groups=gameplane.local,resources=networkcaptures,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gameplane.local,resources=networkcaptures/status,verbs=get;update;patch

// NetworkCapture represents a single packet-capture session initiated by an admin user.
// It tracks the capture's lifecycle (Pending, Running, Completed, Failed), configuration
// (filter, max-duration, max-size), and observed state (packets written, bytes written, timestamps, error reasons).
// Each NetworkCapture is owned by its parent GameServer via ownerReferences.
type NetworkCapture struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkCaptureSpec   `json:"spec,omitempty"`
	Status NetworkCaptureStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NetworkCaptureList is a list of NetworkCaptures.
type NetworkCaptureList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkCapture `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkCapture{}, &NetworkCaptureList{})
}
