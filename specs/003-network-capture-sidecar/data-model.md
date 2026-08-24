# Data Model: Network Capture Sidecar (Spec 003)

Derived from feature spec `spec.md` and Phase 0 research consolidation `research.md`.

**Status: planned.** Nothing described here has been built, run, or tested. No section below
claims passing tests or verified behaviour — only intended shape, traced to FR numbers.

---

## Overview

The data model consists of three primary entities:

1. **NetworkCapture CRD** — captures the capture session lifecycle, configuration, and status. Namespaced, owned by GameServer.
2. **GameServer spec/status extensions** — opt-in flag and status tracking for capture capability.
3. **CaptureFile** — the PCAPNG output artifact, stored in a capture-only emptyDir volume that is
   pre-provisioned, unconditionally, on every GameServer pod (see "Pre-Provisioned Capture Volume"
   under Entity 3) and referenced by NetworkCapture.

All Kubernetes-native entities follow the CRD pattern: Spec (user intent), Status (observed state), Conditions (structured reasons for state transitions).

---

## Entity 1: NetworkCapture CRD

**Location:** Namespaced CRD, group `gameplane.local`, version `v1alpha1`, kind `NetworkCapture`
**Ownership:** Each NetworkCapture is owned by its parent GameServer via `SetControllerReference` (Kubernetes garbage collection cascades cleanup).
**Scope:** Namespaced (same namespace as the parent GameServer).
**Shortname:** `cap` (for `kubectl get cap` convenience)

### Purpose

Tracks a single packet-capture session initiated by an admin user. Records the capture's lifecycle (Pending, Running, Completed, Failed), configuration (filter, max-duration, max-size), and observed state (packets written, bytes written, start/completion timestamps, error reasons).

### Spec Fields

NetworkCapture.spec defines the user's intent for the capture request.

```go
type NetworkCaptureSpec struct {
    // ServerRef: reference to the GameServer being captured.
    // Immutable once created.
    // Required.
    // +kubebuilder:validation:Required
    ServerRef corev1.LocalObjectReference `json:"serverRef"`

    // Filter: pcap-filter(7) expression restricting captured packets.
    // Examples: "tcp port 8080", "host 192.168.1.5 and udp", "tcp[tcpflags] & tcp-syn != 0"
    // Optional. If omitted, a default filter restricts capture to the GameServer's own advertised ports.
    // Immutable once created (filter is set at capture start time, cannot change mid-capture).
    // Validation: Must be a valid pcap-filter expression, compiled at API tier before NetworkCapture CRD is created (FR-003).
    // +kubebuilder:validation:Optional
    // +kubebuilder:validation:MaxLength=1024
    Filter *string `json:"filter,omitempty"`

    // MaxDuration: maximum runtime for the capture, expressed as a Go
    // duration string (e.g. "5m", "300s"). Unit: wall-clock time.
    // If the capture runs for longer than this duration, it stops automatically and status.phase becomes Completed.
    // Required. Must be > 0.
    // Validation: Enforced by sidecar at capture runtime. Traces to FR-002.
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Type=string
    // +kubebuilder:validation:Pattern='^\d+(ms|s|m|h)$'
    MaxDuration *metav1.Duration `json:"maxDuration"`

    // MaxSize: maximum file size for the capture (e.g. 100Mi, 5Gi). Unit:
    // bytes, expressed as a Kubernetes resource.Quantity.
    // If the capture file grows larger than this size, it stops automatically and status.phase becomes Completed.
    // Required. Must be > 0.
    // Validation: Enforced by sidecar at capture runtime. Traces to FR-002.
    // +kubebuilder:validation:Required
    MaxSize *resource.Quantity `json:"maxSize"`

    // TTLSecondsAfterFinished: auto-expiration window after the capture completes.
    // Unit: seconds.
    // After status.completionTime + TTLSecondsAfterFinished, the NetworkCapture is deleted by the operator.
    // Optional; if omitted, cluster default (from Helm value capture.defaultRetentionSeconds) is used.
    // Cluster-wide maximum: capture.maxRetentionSeconds (from Helm value) constrains this field;
    // requests exceeding the max are rejected at API tier with HTTP 400 Bad Request (FR-007).
    // Default: 86400 (24 hours) — matches spec.md FR-007's "Out of Scope (Architectural
    // Constraints Already Decided)" retention default; do not drift this back to 7 days.
    // Examples: 86400 (24 hours, cluster default), 2592000 (30 days), 7776000 (90 days)
    // Validation: API tier validates that requested TTL <= clusterMax before creating the CRD. Traces to FR-007.
    // +kubebuilder:validation:Optional
    // +kubebuilder:validation:Minimum=60
    // +kubebuilder:validation:Maximum=7776000
    TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`
}
```

### Status Fields

NetworkCapture.status observes the actual state of the capture session.

```go
type NetworkCaptureStatus struct {
    // Phase: current lifecycle phase of the capture.
    // Valid values: Pending, Running, Completed, Failed, Expired
    // Enum definition follows below.
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Enum=Pending;Running;Completed;Failed;Expired
    Phase CapturePhase `json:"phase"`

    // StartTime: RFC3339 timestamp when the capture started (phase transitioned from Pending to Running).
    // Populated by the operator when transitioning to Running.
    // +kubebuilder:validation:Optional
    // +kubebuilder:printcolumn:name="Start",type=date,JSONPath=`.status.startTime`
    StartTime *metav1.Time `json:"startTime,omitempty"`

    // CompletionTime: RFC3339 timestamp when the capture stopped (phase transitioned to Completed or Failed).
    // Populated by the operator when transitioning to a terminal phase (Completed, Failed, Expired).
    // +kubebuilder:validation:Optional
    // +kubebuilder:printcolumn:name="End",type=date,JSONPath=`.status.completionTime`
    CompletionTime *metav1.Time `json:"completionTime,omitempty"`

    // PacketsWritten: number of packets captured and written to the output file.
    // Updated as the capture progresses; final value recorded when capture completes.
    // +kubebuilder:validation:Optional
    PacketsWritten int64 `json:"packetsWritten,omitempty"`

    // BytesWritten: total size of the captured file, in bytes.
    // Updated as the capture progresses; final value recorded when capture completes.
    // +kubebuilder:validation:Optional
    BytesWritten *resource.Quantity `json:"bytesWritten,omitempty"`

    // Message: human-readable status or error message.
    // Examples:
    //   - Phase Running: "capturing packets, 1234 packets written"
    //   - Phase Completed: "capture completed, 1234 packets, 5.2 MiB written"
    //   - Phase Failed: "disk full during capture"
    //   - Phase Expired: "auto-deleted after retention window (24h default) expired"
    // +kubebuilder:validation:Optional
    // +kubebuilder:validation:MaxLength=512
    Message string `json:"message,omitempty"`

    // Conditions: structured reasons for current phase.
    // Each condition tracks a boolean state (e.g., "Captured", "SizeLimitReached", "DurationLimitReached").
    // Examples:
    //   - Type: "SizeLimitReached", Status: "True", Reason: "max_size_bytes_exceeded", Message: "5 GiB limit reached"
    //   - Type: "DurationLimitReached", Status: "True", Reason: "max_duration_exceeded", Message: "5 min limit reached"
    //   - Type: "SidecarCrashed", Status: "True", Reason: "sidecar_crashed", Message: "capture sidecar terminated; capture is unrecoverable (ephemeral containers cannot restart)"
    //   - Type: "PodRestarted", Status: "True", Reason: "pod_recreated", Message: "pod was recreated; capture terminated"
    // +kubebuilder:validation:Optional
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

### Phase Lifecycle Definition

```go
type CapturePhase string

const (
    // CapturePhasePending: NetworkCapture CRD created; awaiting operator to transition to Running.
    // Trigger: API creates NetworkCapture with phase=Pending, operator's reconciler checks for concurrent Running captures.
    // Transition: If no other Running capture on this GameServer, the operator injects the capture
    //             sidecar as an ephemeral container (pods/ephemeralcontainers subresource) if not
    //             already present, then transitions to Running.
    //             If another Running capture exists, mark this as Failed with reason "capture_already_in_progress".
    CapturePhasePending CapturePhase = "Pending"

    // CapturePhaseRunning: sidecar is actively capturing packets.
    // Trigger: Operator transitions from Pending after verifying no other Running capture.
    // State: sidecar process is writing to output file; status.startTime is set.
    // Transition: When sidecar reports completion (either via API call or operator detects sidecar absent), transition to Completed or Failed.
    CapturePhaseRunning CapturePhase = "Running"

    // CapturePhaseCompleted: capture finished successfully (manually stopped by user, or max-duration/max-size limit reached).
    // Trigger: User calls POST /servers/{name}:capture-stop, or sidecar reaches max-duration/max-size and auto-stops.
    // State: Output file is finalized and ready for download. status.completionTime, status.packetsWritten, status.bytesWritten are set.
    //        The ephemeral container itself is NOT removed — Kubernetes provides no API to remove an
    //        ephemeral container from a running pod. It lingers, idle, until the pod is next recreated.
    // Cleanup: emptyDir file persists for download until retention window expires (phase Expired).
    // Transition: When TTL expires, operator transitions to Expired and deletes the CRD.
    CapturePhaseCompleted CapturePhase = "Completed"

    // CapturePhaseFailed: capture terminated abnormally (sidecar crash, disk full, invalid filter, pod restart mid-capture, etc.).
    // Trigger: Sidecar reports error, or operator detects failure condition (pod restarted, sidecar absent, disk error).
    // State: Partial or corrupted output file may exist; marked .incomplete or deleted.
    // Cleanup: emptyDir file is cleaned up; no data available for download.
    // Transition: Phase transitions to Expired after TTL (though no data to preserve). CRD is deleted.
    // Note: Failed captures are not downloadable (FR-007); only Completed captures are downloadable while in retention window.
    CapturePhaseFailed CapturePhase = "Failed"

    // CapturePhaseExpired: capture has been auto-deleted by operator because TTL window elapsed.
    // Trigger: Operator's retention reconciler detects status.completionTime + spec.ttlSecondsAfterFinished < now().
    // State: emptyDir file is deleted; download requests return 404 "capture expired".
    // Transition: CRD is deleted by operator. This does NOT remove the ephemeral container from the
    //             pod — that still requires pod recreation, independent of CRD lifecycle.
    // Note: This phase is transient; the CRD itself is deleted shortly after entering this phase.
    CapturePhaseExpired CapturePhase = "Expired"
)
```

### Kubebuilder Markers

```go
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
// The operator additionally needs pods/ephemeralcontainers RBAC to inject the sidecar; this must
// be added to operator/config/rbac alongside the NetworkCapture markers (see Codegen section).
```

### Example NetworkCapture Instance

```yaml
apiVersion: gameplane.local/v1alpha1
kind: NetworkCapture
metadata:
  name: cap-abc123def456
  namespace: games
  ownerReferences:
  - apiVersion: gameplane.local/v1alpha1
    kind: GameServer
    name: my-minecraft-server
    uid: 12345678-1234-1234-1234-123456789012
    controller: true
spec:
  serverRef:
    name: my-minecraft-server
  filter: "tcp port 25565"
  maxDuration: 5m
  maxSize: 100Mi
  ttlSecondsAfterFinished: 86400
status:
  phase: Running
  startTime: "2026-08-23T10:30:00Z"
  packetsWritten: 1234
  bytesWritten: "5.2Mi"
  message: "capturing packets, 1234 packets written"
  conditions:
  - type: "Captured"
    status: "True"
    lastTransitionTime: "2026-08-23T10:30:00Z"
    reason: "capture_started"
    message: "sidecar successfully started capture"
```

---

## Entity 2: GameServer Extensions

**Location:** GameServer CRD, group `gameplane.local`, version `v1alpha1` (`operator/api/v1alpha1/gameserver_types.go`)
**Changes:** New optional fields in `spec.capture.*` and `status.capture.*` subsections.
**Pattern:** Mirrors existing optional feature subsections (e.g., `spec.idle.*` — see `IdleStatus` at `operator/api/v1alpha1/gameserver_types.go:502`).

**Real field names used elsewhere on this CRD, for reference** (do not confuse with the new
`capture` fields being added here):
- The template a GameServer is created from is `spec.templateRef.name`
  (`GameServerSpec.TemplateRef GameTemplateRef` at gameserver_types.go:37, `GameTemplateRef.Name`
  at gameserver_types.go:229-231). There is no `spec.template` or `spec.template.name`.
- The Service exposure mode is `spec.networking.expose`
  (`GameServerNetworking.Expose string` at gameserver_types.go:244). There is no `exposeMode`
  field anywhere on this type.
- Pod selection uses the label `app.kubernetes.io/instance=<name>`. There is no
  `gameplane.io/gameserver` label anywhere in the controller code — any spec referencing it is
  wrong and must not be reintroduced.

### GameServer Spec Extension

```go
type GameServerSpec struct {
    // ... existing fields, including TemplateRef GameTemplateRef `json:"templateRef"` ...

    // Capture: optional capture configuration for this GameServer.
    // If omitted or capture.enabled=false, no capture capability is provided.
    // When enabled=true, the operator injects the capture sidecar ephemeral container.
    // +kubebuilder:validation:Optional
    Capture *CaptureConfiguration `json:"capture,omitempty"`
}

// CaptureConfiguration: opt-in flag and optional per-server capture retention override.
type CaptureConfiguration struct {
    // Enabled: opt-in flag for packet-capture capability on this GameServer.
    // If false or omitted, the GameServer has no capture CONTAINER and its game container is
    // otherwise unchanged (FR-001, SC-007, as amended). It is NOT byte-identical to a server
    // without the feature: every GameServer pod, opted in or not, carries the pre-provisioned
    // capture-data emptyDir volume (see Entity 3) because pods/ephemeralcontainers cannot add a
    // volume and pod.spec.volumes is immutable on a running pod — the volume must already exist
    // before an ephemeral container can mount it. A `kubectl diff` between a non-opted-in and an
    // opted-in server therefore shows only the sidecar container addition, never a change to the
    // game container, per SC-007 as amended.
    // If true, the operator injects the capture sidecar as an ephemeral container via the
    // pods/ephemeralcontainers subresource, live, without restarting the game container.
    // Changing true→false does NOT remove the sidecar from a running pod — Kubernetes has no API
    // to remove an ephemeral container. It stops accepting new captures (status.capture.ready
    // clears immediately) but the container itself lingers in pod status until the pod is next
    // recreated. See research.md for the ephemeral-container limitation.
    // +kubebuilder:validation:Optional
    // +kubebuilder:validation:Type=boolean
    Enabled bool `json:"enabled,omitempty"`

    // RetentionSeconds: optional per-GameServer override of the cluster-default retention window.
    // Unit: seconds.
    // If omitted, cluster default (Helm value capture.defaultRetentionSeconds) is used.
    // Must not exceed cluster max (Helm value capture.maxRetentionSeconds); API validates this at NetworkCapture creation time.
    // Examples: 86400 (24 hours, cluster default), 2592000 (30 days).
    // +kubebuilder:validation:Optional
    // +kubebuilder:validation:Minimum=60
    // +kubebuilder:validation:Maximum=7776000
    RetentionSeconds *int32 `json:"retentionSeconds,omitempty"`
}
```

### GameServer Status Extension

The canonical status shape is **nested** under `status.capture`, matching the CRD's existing
convention for optional feature status blocks (compare `status.idle` / `IdleStatus`). There is no
top-level `status.captureReady` or `status.captureMessage` field — any artifact using those flat
names is wrong.

```go
type GameServerStatus struct {
    // ... existing fields, including Idle *IdleStatus `json:"idle,omitempty"` ...

    // Capture: observed state of the capture system for this GameServer.
    // +kubebuilder:validation:Optional
    Capture *CaptureStatus `json:"capture,omitempty"`
}

// CaptureStatus is the observed state of the capture system for one GameServer.
// JSON path: status.capture.{ready,activeCapture,lastCaptureTime,sidecarRestarts}
type CaptureStatus struct {
    // Ready: bool. JSON tag `ready,omitempty`.
    // Whether the capture sidecar is currently running and able to accept new capture requests.
    // True only while the ephemeral container is Running and listening on its capture port.
    // False if the sidecar has not been injected yet, has crashed (and therefore cannot restart —
    // see Lifecycle State Machine below), or the GameServer has capture.enabled=false (even if a
    // stale ephemeral container still lingers from a prior enabled period).
    // +kubebuilder:validation:Optional
    Ready bool `json:"ready,omitempty"`

    // ActiveCapture: *string. JSON tag `activeCapture,omitempty`.
    // Name of the NetworkCapture currently in Pending or Running phase for this GameServer, if any.
    // Cleared (set to nil) when that capture transitions to Completed, Failed, or Expired.
    // Lets a caller find the in-progress capture without listing all NetworkCaptures in the namespace.
    // +kubebuilder:validation:Optional
    // +kubebuilder:validation:MaxLength=255
    ActiveCapture *string `json:"activeCapture,omitempty"`

    // LastCaptureTime: *metav1.Time. JSON tag `lastCaptureTime,omitempty`.
    // Timestamp of the most recent capture reaching a terminal phase (Completed or Failed) on this
    // GameServer. Not updated for Pending/Running/Expired transitions.
    // +kubebuilder:validation:Optional
    LastCaptureTime *metav1.Time `json:"lastCaptureTime,omitempty"`

    // SidecarRestarts: int32. JSON tag `sidecarRestarts,omitempty`.
    // Count of container restarts observed for the capture ephemeral container in the pod's
    // container statuses. NOTE: ephemeral containers cannot be restarted by Kubernetes (RestartPolicy
    // does not apply to them) — this field is expected to stay 0 in practice for the ephemeral
    // container itself. It is retained to surface the abnormal case if the runtime ever restarts it
    // (e.g. via an unexpected node-level mechanism) and as a stable field name should the
    // implementation later add a genuinely restartable variant.
    // +kubebuilder:validation:Optional
    SidecarRestarts int32 `json:"sidecarRestarts,omitempty"`
}
```

### Example GameServer with Capture

```yaml
apiVersion: gameplane.local/v1alpha1
kind: GameServer
metadata:
  name: my-minecraft-server
  namespace: games
spec:
  templateRef:
    name: minecraft-java
  capture:
    enabled: true
    retentionSeconds: 86400  # 24 hours (matches cluster default; shown explicitly here as an example override)
status:
  phase: Running
  capture:
    ready: true
    activeCapture: cap-abc123def456
    lastCaptureTime: "2026-08-23T10:30:00Z"
    sidecarRestarts: 0
```

---

## Entity 3: CaptureFile

**Location:** A dedicated `capture-data` emptyDir volume, mounted at `/tmp/captures`, holding
`<capture-id>.pcapng` files.
**Ownership:** Lifecycle tied to the parent pod; lost on pod restart (by design, ephemeral diagnostics).
**Format:** PCAPNG (Packet Capture NG), readable by standard tools (Wireshark, tcpdump, tshark, etc.).
**Constraints:**
  - Size limited by spec.maxSize (FR-002), and backstopped by the volume's own `sizeLimit` (see
    below) so a mis-set or unusually large spec.maxSize can't grow the file past what the node's
    disk can actually hold.
  - Retention limited by TTLSecondsAfterFinished (FR-007); file deleted by operator when CRD is deleted.
  - Only Completed captures are downloadable; Failed/Expired captures have no downloadable file.

### Pre-Provisioned Capture Volume (Decision: unconditional, not enable-time)

The `capture-data` volume is added to **every** GameServer's StatefulSet pod template —
unconditionally, for every pod, whether or not `spec.capture.enabled` is set — alongside the
existing `data` and `agent-tls` volumes built in
`operator/internal/controller/gameserver_controller.go` (the `volumes := []corev1.Volume{...}`
construction at lines 1290–1309, immediately followed by the `extraVolumes(gs, tmpl)` append at
line 1315).

```go
{
    // Pre-provisioned unconditionally, for every GameServer pod, opted into
    // capture or not. Not added at capture-enable time: the
    // pods/ephemeralcontainers subresource cannot add a volume, and
    // pod.spec.volumes is immutable on a running pod, so the volume must
    // already exist before an ephemeral capture container can mount it.
    // This is what keeps "enable capture" restart-free.
    Name: "capture-data",
    VolumeSource: corev1.VolumeSource{
        EmptyDir: &corev1.EmptyDirVolumeSource{
            // Hard disk guard backing FR-002's max-size and the disk-full
            // edge case, independent of any single capture's spec.maxSize.
            SizeLimit: captureVolumeSizeLimit, // from Helm value capture.volumeSizeLimitBytes
        },
    },
},
```

**Consequence — one-time rolling restart on upgrade:** adding this volume to the pod template
changes `spec.template` on every existing GameServer's StatefulSet, which triggers Kubernetes'
normal StatefulSet rolling-update behavior. Every existing game server restarts once, the first
time a cluster upgrades to the release that adds this field — including servers that never opt
into capture. This must be called out as an explicit upgrade note (release notes / `docs/install.md`
upgrade section), not left implicit; it is the direct cost of keeping later capture-enable/disable
toggles restart-free.

**Mount placement — deliberately not on the agent container:** the volume is mounted on the
ephemeral capture sidecar when one is injected. It is **not** mounted on the agent container, and
downloads are not served through the agent's existing file-browser surface. The doc comment on
`agentVolumeMounts` (`operator/internal/controller/gameserver_rcon.go:105-119`) explains why extra
volumes are deliberately excluded from the agent: `agent/internal/files` is rooted at exactly one
path (`--data-root`, i.e. the `data` mount) and rejects any resolved path outside it — it "has no
notion of a second root," and teaching it one is called out there as a distinct future change, not
something to bolt on incidentally. Reusing the agent's existing `/files/download` endpoint for
`/tmp/captures/...` would be exactly that second root, so this data model does not do it (see
"Download Endpoints" below — captures are served exclusively through the sidecar's own endpoint,
proxied by the API, at every size). The volume is also never mounted on the game container, so
`spec.template.spec.containers[]` for the game container itself is unchanged by capture, honoring
FR-008.

### Capture Container securityContext (Decision: file capabilities, not root)

The capture sidecar gets raw-socket access via a **file capability** on its binary
(`setcap cap_net_raw+ep`), not by running the container as root and not via the pod/container
`securityContext.capabilities.add` field granting it directly — `add: ["NET_RAW"]` IS still set
below, but only to preserve NET_RAW in the process's bounding set against `drop: ["ALL"]`; see the
inline comment:

```go
SecurityContext: &corev1.SecurityContext{
    RunAsNonRoot:             ptrTo(true),
    AllowPrivilegeEscalation: ptrTo(true), // required — see below; this is the accepted trade-off
    ReadOnlyRootFilesystem:   ptrTo(true),
    Capabilities: &corev1.Capabilities{
        Drop: []corev1.Capability{"ALL"},      // empty effective set
        Add:  []corev1.Capability{"NET_RAW"},  // keep NET_RAW in bounding set
    },
    // Drop ALL empties the effective capability set, preventing unintended
    // privilege escalation. However, it would also empty the bounding set,
    // which would cause execve of the setcap'd binary to fail with EPERM
    // (the kernel cannot grant a file capability that is not in the bounding
    // set, even if the file has the capability granted via setcap). Re-adding
    // NET_RAW keeps it in the bounding set only — the process still starts
    // with no effective capabilities, but the kernel can now grant NET_RAW
    // when executing the setcap'd binary. The capability is granted at
    // image-build time via `setcap cap_net_raw+ep` on the sidecar binary.
},
```

**Why `AllowPrivilegeEscalation: true` is unavoidable here:** file capabilities are ignored by the
kernel when `no_new_privs` is set, and Kubernetes sets `no_new_privs` precisely when
`allowPrivilegeEscalation: false` (the default under a "restricted" profile). So this container
must set `allowPrivilegeEscalation: true` for the `setcap`'d binary's capability to take effect at
all. The accepted trade-off is: `runAsNonRoot: true` is preserved, `allowPrivilegeEscalation` is
given up. State this plainly wherever this securityContext is documented or implemented — it is
not an oversight to be "fixed" by a future security pass.

**Consequence — PodSecurity exception required:** a "restricted" PodSecurity Standard profile
forbids `allowPrivilegeEscalation: true` outright. Any namespace running capture-enabled
GameServers under "restricted" needs a documented, explicit exception. This must be recorded as a
`docs/security.md` obligation, not left implicit in this data model alone.

**Open build risk — UNVERIFIED, must be proven in CI, not asserted:** file capabilities are stored
as filesystem extended attributes (xattrs), and xattrs are not guaranteed to survive a multi-stage
`COPY` into a distroless/scratch final image — this repo has already hit a COPY-time file-mode bug
(`fix(images): set entrypoint mode at COPY time instead of chmod`) that is exactly this class of
problem for regular file permissions, and xattrs are more fragile than mode bits across `COPY`.
Whether `cap_net_raw+ep` actually survives this image's build must be verified by a CI check (e.g.
`getcap` on the built image) before this approach is trusted; nothing in this document may claim it
already works.

### File Naming and Metadata

- **Filename:** `capture-<capture-id>.pcapng`
- **Content-Type:** `application/vnd.tcpdump.pcap` or `application/pcapng`
- **Content-Disposition:** `attachment; filename="capture-<capture-id>.pcapng"`
- **Checksum:** Not computed or verified (captures are ephemeral, not archival).

### PCAPNG Structure

The sidecar is planned to write PCAPNG format using `gopacket/pcapgo.NgWriter`. PCAPNG allows:
- **Interface description blocks:** one per network interface captured (eth0, for example).
- **Enhanced packet blocks:** one per captured packet, with timestamp (nanosecond precision), truncated-packet flag, packet data (up to snaplen).
- **Options:** interface name, filter expression (for audit), and snaplen in Interface Description Block.

**Capture library risk (UNVERIFIED as a settled dependency):** the candidate BPF-filter compile
path (`github.com/packetcap/go-pcap`, `filter` package, `Compile` function) exists on the module
proxy but has **no tagged release** — only a pseudo-version
(`v0.0.0-20260731105150-c86974bbfbcd`). Pinning to an untagged commit is a real supply-chain and
stability risk (no semver guarantee, no release notes, upstream can force-push its default branch
history). This must be tracked as an open risk in plan.md, not treated as a resolved choice.

### Download Endpoints

Downloads are planned to be proxied by the API, at every file size, through the capture sidecar's
own endpoint — not through the agent's existing file-browser surface (see "Pre-Provisioned Capture
Volume" above for why):
- **All sizes:** via sidecar `/captures/<capture-id>/file` endpoint, proxied by API at
  `GET /servers/{name}:capture-file?id={id}` (`contracts/rest-api.md`). This is the same
  colon-verb + query-parameter shape as `:capture-start`/`:capture-stop`, deliberately — a variable
  middle path segment (the earlier `/servers/{name}/captures/{id}/file` and equally the retired
  `/ws/servers/{name}/captures/{id}/file` form) cannot be matched by the RBAC path-pattern table's
  `match(method, path)` (`api/internal/rbac/rbac.go:228-237`, matching on first-segment plus a
  fixed suffix, not a wildcard middle segment), so a fixed-suffix `:capture-file` route is required
  to let a `captures:manage`-scoped rule apply to it at all. There is no separate small-file tier
  through the agent; the previous two-tier split (small via agent, large via sidecar) is dropped
  because it would have required mounting `capture-data` on the agent and extending its
  single-rooted file browser to a second root, which `agentVolumeMounts`' doc comment defers as a
  distinct future change.

---

## Relationships and Ownership

### Ownership Chain

```
GameServer (parent)
  └── NetworkCapture (owned by GameServer via ownerReferences)
       └── CaptureFile (lifecycle tied to NetworkCapture CRD)
       └── Audit Events (one entry per capture operation)
```

**Cascade Behavior:**
- **Deleting a GameServer:** Cascades delete all NetworkCapture CRDs in that namespace for that server (Kubernetes garbage collection, `ownerReferences` with `controller: true`).
- **Deleting a NetworkCapture:** Operator cleanup logic deletes the associated `/tmp/captures/` file from the pod (if still running).
- **Pod restart:** emptyDir is lost; NetworkCapture CRD is marked Failed if still Running. The
  ephemeral capture container, if any, is also gone (pod recreated) — this is the point at which a
  previously-disabled capture's lingering ephemeral container is finally dropped.

### Foreign Keys and Cross-References

| Field | Type | Target | Constraint |
|-------|------|--------|-----------|
| `NetworkCapture.spec.serverRef` | LocalObjectRef | GameServer | Must exist in same namespace; API validates at creation time |
| `GameServer.status.capture.activeCapture` | *string | NetworkCapture name | Points to capture in Pending or Running phase; cleared on phase transition |

---

## Lifecycle State Machine

This section covers both (a) the ephemeral-container injection/removal lifecycle driven by
`spec.capture.enabled`, and (b) the per-capture NetworkCapture phase machine, including the
spec's edge cases.

### A. Ephemeral Container Lifecycle (driven by `GameServer.spec.capture.enabled`)

| From | To | Trigger | Actor |
|------|----|---------|-------|
| No sidecar | Sidecar Running | `spec.capture.enabled` flips false→true on a running pod | operator (calls the `pods/ephemeralcontainers` subresource; no game-container restart) |
| Sidecar Running | Sidecar Running, not accepting captures | `spec.capture.enabled` flips true→false | operator (stops routing new capture requests to it; sets `status.capture.ready=false` immediately); the container itself is **not removed** — Kubernetes exposes no API to remove an ephemeral container from a live pod |
| Sidecar Running, not accepting captures | No sidecar (fresh pod) | Pod is recreated for any reason (restart, reschedule, node loss) | Kubernetes (pod replacement); operator re-evaluates `spec.capture.enabled` on the new pod and injects fresh only if still true |

US2 acceptance scenario 4 ("disabling capture removes the sidecar") must be read against this: the
running pod's ephemeral container lingers, idle, until the pod is next recreated. Only pod
recreation actually removes it. This is a platform limitation, not an implementation gap.

### B. NetworkCapture Phase Transitions

**From Pending:**
- → Running (if no other Running capture on the same GameServer; ephemeral container is injected
  first if not already present)
- → Failed (if concurrent Running capture detected, or serverRef validation fails)

**From Running:**
- → Completed (user calls stop, or max-duration/max-size limit reached, or sidecar completes successfully)
- → Failed (sidecar crashes, disk full, pod restarted, validation error)

**From Completed:**
- → Expired (TTL window elapsed; operator deletes CRD)

**From Failed:**
- → Expired (TTL window elapsed; operator deletes CRD)

**From Expired:**
- → (deleted) — CRD is removed from etcd; state machine ends

### Transition Triggers and Actors, With Edge Cases

| From | To | Trigger | Actor | Traces to FR |
|------|----|---------|-------|--------------|
| Pending | Running | Operator reconciler checks no concurrent Running capture; injects ephemeral container if absent | operator | FR-012 |
| Pending | Failed | Another Running capture exists, or serverRef validation fails | API tier or operator | FR-012, FR-003 |
| Running | Completed | User calls POST /servers/{name}:capture-stop | API → sidecar | FR-001 |
| Running | Completed | Sidecar detects max-duration limit reached (edge case: duration limit) | sidecar | FR-002 |
| Running | Completed | Sidecar detects max-size limit reached (edge case: size limit) | sidecar | FR-002 |
| Running | Failed | Sidecar process exits/crashes (edge case: sidecar crash) — **capture is lost, not retried**: an ephemeral container cannot be restarted by Kubernetes, so there is no automatic recovery path; a new capture must be started from scratch once a fresh sidecar exists (next pod recreation, or if one was already Running under a separate injection) | operator (detects via reconciliation: container status shows Terminated with no restart) | FR-010 |
| Running | Failed | Pod is restarted mid-capture (edge case: pod restart mid-capture) | operator (watches pod events/deletion) | FR-010, edge case US5.1 |
| Running | Failed | Disk full during capture (edge case: disk full) | sidecar (write error surfaces status) | edge case, FR-009 |
| Completed | Expired | TTL window elapses (completionTime + ttl < now()) | operator reconciler (runs every 60s) | FR-007 |
| Failed | Expired | TTL window elapses | operator reconciler | FR-007 |
| Expired | (deleted) | Operator deletes NetworkCapture CRD | operator | FR-007 |

### Concurrency Semantics (FR-012)

**One capture at a time per GameServer.**

The operator's NetworkCaptureReconciler enforces this:
1. Before transitioning Pending → Running, check: do any other NetworkCaptures for this GameServer have phase=Running?
2. If yes, mark this capture Failed with reason "capture_already_in_progress" and return error.
3. If no, proceed to Running (injecting the ephemeral container first if this is the GameServer's
   first capture).

**Optional API-tier optimization:** Before creating the NetworkCapture CRD, the API tier checks if another capture is already Running on that GameServer. If yes, return HTTP 409 Conflict immediately (fail fast). If no, proceed to create the CRD.

---

## Validation Rules

### At API Tier (HTTP Request Time)

**FR-003: Filter validation**
- Input: `POST /servers/{name}:capture-start` with `filter: "tcp port 8080"`
- Validation: API calls the candidate filter-compile function (see the capture-library risk note
  under Entity 3) to compile the BPF expression.
- On error: Return HTTP 400 Bad Request with descriptive error message.
- On success: Proceed to create NetworkCapture CRD with the filter string.

**FR-007: Cluster max retention enforcement**
- Input: `POST /servers/{name}:capture-start` with `ttlSecondsAfterFinished: 86400000` (1000 days)
- Validation: Compare against cluster max (from Helm value `capture.maxRetentionSeconds`, e.g., 7776000 for 90 days).
- If `ttlSecondsAfterFinished > clusterMax`: Return HTTP 400 Bad Request.
- Otherwise: Use the requested TTL (or cluster default if omitted).

**FR-005 / SC-005: Admin-only access — real mechanism**

The API's authorization is **not** a `RequireRole()`-style call; it is the path-pattern rule table
in `api/internal/rbac/rbac.go`, mounted once via `Middleware` (rbac.go:71). `match(method, path)`
(rbac.go:237) walks `rules []rule` (rbac.go:150) and returns **the first entry whose method,
segment/prefix/suffix match** — order in that slice is load-bearing, not just documentation.

The existing catch-all for anything under `/servers/...` is:

```go
// rbac.go:183-184
{method: "GET", segment: "servers", perm: "servers:read"},
{segment: "servers", perm: "servers:write"},
```

Every capture endpoint under discussion (`POST /servers/{name}:capture-start`,
`POST /servers/{name}:capture-stop`, `GET /servers/{name}:captures`,
`GET /servers/{name}:capture-file?id={id}`, `DELETE /servers/{name}:capture?id={id}`) has `servers`
as its first path segment. If a capture-specific rule is **not** inserted before rbac.go:184 in the
`rules` slice, `match` returns the `servers:write` rule for every one of them instead. Per
`api/internal/db/migrations/003_roles.sql:48`, the built-in **operator** role already holds
`servers:write` — so skipping the ordering requirement silently grants capture access to
operators, not just admins, violating FR-005 and SC-005 without any error, test failure, or log
line to flag it.

**Required implementation shape:**
1. Add a new permission key (e.g. `captures:manage`) to the catalog in
   `api/internal/rbac/catalog.go`, following the existing `Permission{Key, Label, Namespaced}`
   shape shown at catalog.go:27 (namespaced, since a capture belongs to one GameServer's
   namespace).
2. Grant `captures:manage` to the built-in `admin` role only, in a **new, append-only** migration
   file (next sequential number after `006_share_links.sql`, i.e. `007_...sql`) mirroring the
   `INSERT INTO role_permissions(role_name, permission) VALUES (...)` pattern in
   `003_roles.sql:43-46`. Do not grant it to `operator` or `viewer`.
3. Insert the capture path rules into `rbac.go`'s `rules` slice **before** the
   `{segment: "servers", perm: "servers:write"}` line (rbac.go:184), matching on the capture verb
   suffixes/prefixes the same way the existing WS console rules do (rbac.go's
   `{method: "GET", segment: "ws", suffix: "/console", perm: "servers:console"}` pattern is the
   template to follow for verb-specific matching ahead of a broader same-segment rule).
4. Add a unit/table test asserting `match()` returns the new `captures:manage` permission for each
   capture path+method combination, specifically to catch a future reordering regression — this is
   the one thing this data model must not leave to code review alone.

- On unauthorized: Return HTTP 403 Forbidden (same as every other RBAC-denied request —
  `Middleware` writes `http.StatusForbidden` uniformly; there is no separate capture-specific error
  path).

### At Operator Tier (CRD Reconciliation)

**Concurrency (FR-012):**
- When NetworkCapture enters Pending phase, operator lists NetworkCaptures for
  `spec.serverRef.name = X` in the namespace and checks whether any has `status.phase = Running`.
- If one exists: Set phase to Failed, reason "capture_already_in_progress".
- Otherwise: Transition to Running (injecting the ephemeral container first if needed).

**Pod restart detection (edge case US5.1):**
- Operator watches Pod events for the game pod.
- If pod is deleted while a NetworkCapture is Running: Operator marks the capture Failed with reason "pod_restarted".
- Partial file in emptyDir is cleaned up by the pod's termination (emptyDir is ephemeral).
- Any ephemeral capture container is also gone with the old pod; a fresh one is injected only if
  `spec.capture.enabled` is still true when the operator reconciles the new pod.

**TTL enforcement (FR-007):**
- Operator reconciler runs every 60 seconds (configurable).
- For each NetworkCapture in Completed or Failed phase: Check if `status.completionTime + spec.ttlSecondsAfterFinished < now()`.
- If expired: Transition to Expired, then delete the CRD.
- Note: Kubernetes garbage collector does NOT automatically enforce `ttlSecondsAfterFinished` on custom CRDs; operator must explicitly delete.

### At Sidecar Tier (Capture Runtime)

**Max-size limit (FR-002, edge case):**
- Sidecar tracks file size as it writes.
- When file reaches `spec.maxSize`: Close the file, transition status.phase to Completed, set status.message "capture size limit reached".

**Max-duration limit (FR-002, edge case):**
- Sidecar uses a timer for `spec.maxDuration`.
- When timer expires: Stop the AF_PACKET socket, close the file, transition status.phase to Completed, set status.message "capture duration limit reached".

**Sidecar crash (edge case):**
- Because the capture sidecar runs as an ephemeral container, Kubernetes does not restart it on
  exit (RestartPolicy does not apply to ephemeral containers). A crash is therefore terminal for
  that capture: the operator observes the container's terminated status with no restart occurring,
  marks the NetworkCapture Failed with reason "sidecar_crashed", and any partial file is discarded.
  There is no retry-in-place; a new capture (and, after the next pod recreation, a fresh ephemeral
  container) is required to try again.

**Filter compilation (secondary, defense-in-depth):**
- If API-tier validation is bypassed (e.g., direct CRD creation), sidecar attempts to compile the filter.
- On error: Fail the capture with status.phase = Failed, reason "invalid_filter".

---

## Audit Logging (FR-006)

Every capture operation is logged via the API's audit middleware (`api/internal/audit/audit.go`).
The audit schema is the **real** `Event` struct (audit.go:727-735) — there is no `payload`,
`result`, or `operation` column, and `Status` is the HTTP response status code, not a
capture-specific outcome enum:

```go
// api/internal/audit/audit.go:727-735 (current shape, before this feature's migration)
type Event struct {
    ID     int64  `json:"id"`
    TS     string `json:"ts"`
    Actor  string `json:"actor"`
    Method string `json:"method"`
    Path   string `json:"path"`
    Target string `json:"target,omitempty"`
    Status int    `json:"status"`
    IP     string `json:"ip,omitempty"`
}
```

**A structured `Reason` column is added, via a new append-only migration** (Decision: audit reason
column). The existing `Method`/`Path`/`Target`/`Status` fields identify *what* was requested and
its HTTP outcome, but not *why* a non-2xx outcome occurred (e.g. "capture_already_in_progress" vs.
a generic 409, or "invalid_filter" vs. a generic 400) — for capture operations that distinction is
worth recording structurally rather than only in free-text elsewhere.

```go
// api/internal/audit/audit.go — planned addition
type Event struct {
    ID     int64  `json:"id"`
    TS     string `json:"ts"`
    Actor  string `json:"actor"`
    Method string `json:"method"`
    Path   string `json:"path"`
    Target string `json:"target,omitempty"`
    Status int    `json:"status"`
    IP     string `json:"ip,omitempty"`
    Reason string `json:"reason,omitempty"` // NEW: short machine-readable code, e.g. "capture_already_in_progress"
}
```

**Migration:** the existing files are `001_init.sql` … `006_share_links.sql` (highest existing
number, checked directly against `api/internal/db/migrations/`). This feature's RBAC section below
already claims `007_...sql` for the `captures:manage` permission grant, so the audit-reason column
is the next migration after that: `008_capture_audit_reason.sql`, append-only per convention:

```sql
-- 008_capture_audit_reason.sql
-- Adds a structured, machine-readable "reason" column to audit_events, primarily so capture
-- rejections (e.g. concurrent-capture conflicts, invalid filters, RBAC denials) carry a short code
-- alongside the existing method/path/status fields. Existing rows get NULL.
ALTER TABLE audit_events ADD COLUMN reason TEXT;
```

**Critical risk, addressed explicitly (Decision 2): the new column does NOT participate in the hash
chain.** `005_audit_chain.sql` added `prev_hash`/`hash`, computed by `computeHash` /
`canonicalize()` (`api/internal/audit/audit.go`) over exactly seven fields — `TS`, `Actor`,
`Method`, `Path`, `Target`, `Status`, `IP` — for every row that already has a non-NULL `hash`,
which includes every row written between migration 005 and this one, not just rows written before
005. If `Reason` were added to that seven-field byte layout, `Verify()` would recompute a different
hash for every one of those already-hashed rows on its very first run after this ships, and report
the entire pre-existing chain as tampered — a false positive across real history, not a targeted
check, and indistinguishable from an actual attack without redeploying and diffing schema versions.
`canonicalize()`'s field list is therefore left unchanged; `Reason` is stored and returned exactly
like `Target`/`IP` today, but is **not** tamper-evident — a direct `UPDATE` against just that column
would not be caught by `Verify()`. Pre-existing rows are entirely unaffected (their hashes were
computed, and remain verifiable, under the original seven-field layout). If reason integrity is
ever required, that needs a deliberate new hash-format migration that recomputes and re-chains the
whole table, not an in-place column add — this is out of scope here.

**Example audit rows (as `Event` values, one JSON object per row):**
```json
{"id": 1001, "ts": "2026-08-23T10:30:00Z", "actor": "admin@example.com", "method": "POST", "path": "/servers/my-server:capture-start", "target": "my-server", "status": 202, "ip": "192.168.1.1"}
{"id": 1002, "ts": "2026-08-23T10:30:15Z", "actor": "admin@example.com", "method": "GET", "path": "/servers/my-server:capture-file?id=cap-abc", "target": "cap-abc", "status": 200, "ip": "192.168.1.1"}
{"id": 1003, "ts": "2026-08-23T10:30:20Z", "actor": "viewer@example.com", "method": "POST", "path": "/servers/my-server:capture-start", "target": "my-server", "status": 403, "ip": "192.168.1.2", "reason": "rbac_denied"}
{"id": 1004, "ts": "2026-08-23T10:31:00Z", "actor": "admin@example.com", "method": "POST", "path": "/servers/my-server:capture-start", "target": "my-server", "status": 409, "ip": "192.168.1.1", "reason": "capture_already_in_progress"}
```

Row 1002 is a `GET`, and `shouldLog` returns `false` for every `GET` before any path is even
inspected (`api/internal/audit/audit.go:659-663`), so the generic `Middleware` structurally cannot
produce this row. The capture-file download handler must call the same synchronous, exported
audit-write path used for start/stop/enable/disable/delete directly from handler code, not through
`Middleware`, immediately before streaming the response body (or before writing the error status,
on failure) — this is new work, not something the existing middleware wiring provides for free.

---

## Codegen and Manifest Requirements (CLAUDE.md Rule 7)

The NetworkCapture CRD types are planned for a new file:
```
operator/api/v1alpha1/networkcapture_types.go
```

and the `CaptureConfiguration`/`CaptureStatus` additions land in the existing
`operator/api/v1alpha1/gameserver_types.go`.

After editing these files, the following commands MUST be run in the same commit — this task does
NOT run them; it only records what must change:
```bash
make generate
make manifests
```

These commands are expected to touch the following files, which MUST be committed alongside the
type definitions:

1. **Deepcopy methods:**
   ```
   operator/api/v1alpha1/zz_generated.deepcopy.go
   ```
   (regenerated for both the new NetworkCapture types and the GameServer `Capture`/`CaptureStatus`
   additions — this is one file, not two separate regenerations.)

2. **CRD YAML (from kubebuilder markers):**
   ```
   operator/config/crd/*.yaml
   ```
   (a new bases file for `gameplane.local_networkcaptures.yaml`, plus the existing
   `gameplane.local_gameservers.yaml` picking up the new `spec.capture`/`status.capture` schema,
   plus the `kustomization.yaml` listing entry.)

3. **RBAC rules (from kubebuilder markers):**
   ```
   operator/config/rbac/*.yaml
   ```
   (new NetworkCapture editor/viewer role files, the `kustomization.yaml` update, and the
   operator's own ClusterRole gaining `pods/ephemeralcontainers` verbs for the injection path.)

4. **Helm chart CRD copies (synced from operator/config/crd by `make manifests`):**
   ```
   charts/gameplane/crds/*.yaml
   charts/gameplane/crd-manifests/*.yaml
   ```
   (both copies must be kept in sync per the chart's pre-upgrade-hook mechanism described in
   CLAUDE.md rule 7 — `crd-manifests/` exists because Helm hides `crds/` from `.Files`.)

**Verification (manual read of the generated output, not a run of it):**
After codegen runs (in CI or by a human, never by this agent), the generated
`gameplane.local_networkcaptures.yaml` should contain:
- `kind: CustomResourceDefinition`
- `name: networkcaptures.gameplane.local`
- `scope: Namespaced`
- Spec fields: serverRef, filter, maxDuration, maxSize, ttlSecondsAfterFinished
- Status fields: phase, startTime, completionTime, packetsWritten, bytesWritten, message, conditions
- Print columns for kubectl output (Server, Phase, Packets, Size, Start)

and the regenerated `gameplane.local_gameservers.yaml` should show `spec.capture.{enabled,
retentionSeconds}` and `status.capture.{ready, activeCapture, lastCaptureTime, sidecarRestarts}`
under the existing GameServer schema.

---

## Default Values (Helm Configuration)

The cluster-wide defaults are planned to be configured via Helm values (`charts/gameplane/values.yaml`):

```yaml
capture:
  enabled: true                           # Feature flag: allow/disallow captures cluster-wide
  defaultRetentionSeconds: 86400          # 24 hours: default auto-expiration window (spec.md FR-007)
  maxRetentionSeconds: 7776000            # 90 days: hard cap, constrains all per-server overrides
  defaultMaxDurationSeconds: 300          # 5 minutes: default capture runtime limit
  defaultMaxSizeBytes: 5368709120         # 5 GiB: default per-capture file size limit (spec.maxSize default)
  volumeSizeLimitBytes: 10737418240       # 10 GiB: emptyDir.sizeLimit for the pre-provisioned
                                           # capture-data volume (see Entity 3) — a hard,
                                           # cluster-wide disk guard independent of any single
                                           # capture's spec.maxSize; only one capture runs at a
                                           # time per GameServer (FR-012), so this only needs to
                                           # hold one in-flight file.
```

These values would be passed to the operator and API at startup via CLI flags, following the
existing pattern of other Helm-value-to-flag wiring in `operator/cmd/main.go` / `api/cmd/main.go`
(exact flag names UNVERIFIED — no such flags exist yet since this feature is unbuilt).

---

## Summary Table

| Aspect | Decision | Justification |
|--------|----------|---------------|
| **Storage** | NetworkCapture CRD (namespaced, owned by GameServer) | Rule 10 (operator authoritative); supports kubectl; cascading cleanup |
| **File storage** | `capture-data` emptyDir (`/tmp/captures/`), **pre-provisioned unconditionally on every GameServer pod** with an `emptyDir.sizeLimit`, mounted only on the sidecar (never the agent or game container) | Ephemeral container can't add a volume and pod volumes are immutable, so the volume must pre-exist; costs one rolling restart on upgrade (see Entity 3) |
| **Retention** | TTLSecondsAfterFinished-style field, default 24 hours (86400s), enforced by an operator reconciler loop | Matches spec.md FR-007; Kubernetes does NOT auto-enforce this on CRDs, so the operator must delete explicitly every ~60s |
| **Defaults** | Helm values (immutable at cluster install) | Consistent with audit retention pattern in CLAUDE.md |
| **Concurrency** | CRD phase-based serialization | Lock-free, atomic via Kubernetes PATCH, no explicit Lease resource |
| **Sidecar injection** | Ephemeral container via pods/ephemeralcontainers, live | No game-container restart on enable; but no removal API on disable — removal only happens at next pod recreation |
| **Validation** | API tier (FR-003, FR-007); secondary sidecar (defense-in-depth) | Early rejection, better UX, separates concerns |
| **Audit** | Middleware logs every operation (FR-006) via `Event{ID,TS,Actor,Method,Path,Target,Status,IP,Reason}` — `Reason` added by new migration `008_capture_audit_reason.sql` | Automatic via api/internal/audit middleware; no payload/result envelope; `Reason` is deliberately excluded from the hash chain to avoid invalidating pre-existing rows' hashes |
| **RBAC** | New `captures:manage` permission, admin-only grant via new migration, rule inserted before rbac.go:184's `servers:write` catch-all | Path-pattern first-match-wins table (rbac.go); wrong ordering silently grants operator role access (FR-005/SC-005 violation) |
| **Security** | Sidecar gets `NET_RAW` via a file capability (`setcap cap_net_raw+ep`) on its binary, not by running as root; requires `allowPrivilegeEscalation: true` (accepted trade-off, `runAsNonRoot` kept) and a documented PodSecurity "restricted" exception; whether the capability survives the multi-stage image build is UNVERIFIED and must be proven in CI (see Entity 3); admin-only RBAC; auto-deleted files | Least privilege short of root; sensitive data access gated; limited retention window |
| **Codegen** | make generate && make manifests in same commit (not run by this task) | Per CLAUDE.md rule 7; regenerated files kept in sync |
| **Open risk** | `github.com/packetcap/go-pcap` filter-compile dependency has no tagged release (pseudo-version only) | Must be tracked as a supply-chain/stability risk in plan.md, not treated as settled |
