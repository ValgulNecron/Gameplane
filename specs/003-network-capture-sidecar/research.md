# Research Consolidation: Network Capture Sidecar (Spec 003)

Phase 0 decision document consolidating findings from 9 parallel research probes: operator pod building, Kubernetes container injection mechanisms, admin API patterns, API-agent communication, CRD lifecycle patterns, packet capture technology options, e2e test strategy, optional component wiring, and security posture.

---

## Sidecar Injection Mechanism: Ephemeral Containers

**Decision (ACCEPTED 2026-08-23)**: Use Kubernetes ephemeral containers (K8s 1.28+) to add the capture sidecar to running pods without pod template modification. Disabling capture does NOT itself trigger a pod restart: it stops any active capture and marks the capability off immediately, but the ephemeral container is only actually removed the next time the pod is recreated for some other, independent reason (a normal Kubernetes constraint — ephemeral containers cannot be removed in place). Until that next recreation, the (now-inert) ephemeral container simply lingers in the pod without affecting the game container.

**Rationale**:
- **Add without restart (US2 criterion 2)**: Ephemeral containers are the ONLY Kubernetes 1.28+ mechanism to add containers to a running pod without modifying the StatefulSet pod template (which triggers pod recreation). Ephemeral containers are created via `POST /pods/{name}/ephemeralcontainers` subresource and added to the pod immediately; the game container never stops. Per spec.md's actual US2 scenario 2, this is stated as: "Given a GameServer with capture disabled, When an admin enables capture, Then the capture sidecar is injected as an ephemeral container into the running pod without restarting the game container." ✓ ACHIEVES THIS.
- **Disable does not restart the pod (US2 criterion 4)**: Kubernetes 1.28 design explicitly forbids removal of ephemeral containers without pod recreation. "Ephemeral containers may not be removed or restarted" (K8s docs). spec.md's actual US2 scenario 4 does NOT claim disable triggers a restart — it explicitly defers removal to whenever the pod is next recreated for unrelated reasons, and is explicit that the game container is unaffected in the meantime: "Given a GameServer with capture enabled, When an admin disables capture, Then any active capture is stopped immediately, the capture capability is marked off, and the ephemeral container is removed on the next pod recreation—with the game container unaffected in the meantime." FR-001 states the same constraint: "Disabling capture MUST stop any active capture and mark the capability off immediately; the ephemeral container is removed on the next pod recreation without affecting the running game container in the meantime." No amendment to spec.md was made or is needed here — this is simply the requirement as already written. A prior draft of this research document mistakenly asserted that disable "triggers a controlled pod restart" and that this was an approved spec amendment; that assertion was fabricated (no such text exists in spec.md, no such amendment was ever applied) and is corrected here. ✓ ACCEPTABLE OPERATIONAL OVERHEAD (a stale ephemeral container carries no elevated capability once the capture it belonged to is stopped) — but it is a lazy, incidental cleanup, not an admin-triggered restart.
- **Pod-template sidecar alternative**: Regular container in `spec.containers` requires StatefulSet pod template change → pod recreates → game container restarts → violates US2 criterion 2. ✗ NOT VIABLE.
- **Native sidecar alternative** (initContainers with `restartPolicy: Always`, K8s 1.28+): Not yet stable; requires pod template change → pod restart anyway. Same as regular container. ✗ NOT VIABLE.

**Key finding** (from Kubernetes Design research): "Ephemeral containers cannot be removed without pod recreation. This is a fundamental Kubernetes design constraint."

**Alternatives considered**:
1. **Regular pod.spec.containers sidecar**: Modifying pod.spec.containers requires pod template change (StatefulSet update) → rolling update → game pod recreates → game container restarts. Violates US2's "without restarting game container" requirement. Ruled out.
2. **Separate network-sharing pod (sidecar Deployment)**: Run a separate pod in the games namespace that intercepts game pod traffic via NetworkPolicy or shared network namespace. Would require Kubernetes network namespace sharing (not standard isolation) and complex network plumbing. Operationally complex, fragile, deferred.
3. **Init container with persistent process**: initContainers run once and exit; converting one to long-lived (restartPolicy: Always) requires pod template change, same problem as regular sidecar. Ruled out.

---

## Capture Engine: gopacket/afpacket with pcapgo and go-pcap/filter for BPF Validation

**Decision**: Use `github.com/google/gopacket/afpacket` (AF_PACKET with MMap'd kernel buffers) for live packet capture, `github.com/google/gopacket/pcapgo` (NgWriter) for writing PCAPNG files, and `github.com/packetcap/go-pcap/filter` for pure-Go BPF filter expression compilation and validation at the API tier.

**Rationale**:
- **CGO requirement**: AF_PACKET is pure Go (no cgo required, buildable with `CGO_ENABLED=0`). Distroless-compatible. ✓ MATCHES FR-008.
- **Libpcap requirement**: AF_PACKET does NOT depend on libpcap at runtime. BPF filter compilation happens via `go-pcap/filter` in the API tier (separate process), not in the sidecar. ✓ KEEPS SIDECAR MINIMAL.
- **Image base**: Can use `gcr.io/distroless/static:nonroot` (existing operator/api/agent pattern). ✓ CONSISTENT WITH CODEBASE.
- **Performance**: AF_PACKET with MMap'd TPacket buffers is battle-tested on high-packet-rate workloads (Linux kernel feature, widely used in network monitoring). MMap'd buffers reduce copy overhead vs simple socket reads. ✓ ADEQUATE FOR GAME SERVER TRAFFIC.
- **Filter compilation**: `packetcap/go-pcap/filter` (pure Go, no libpcap) provides the only pure-Go BPF expression compiler. Validates filters at API request time (early rejection, FR-003 compliance) rather than in sidecar (which has no way to compile filters without libpcap). ✓ ENABLES EARLY VALIDATION.
- **PCAP writing**: `pcapgo.NgWriter` handles PCAPNG format (nanosecond precision, interface metadata, modern tool support). Does NOT auto-truncate packets to snaplen; sidecar must do it (manual truncation ~5 lines of code). ✓ ACCEPTABLE TRADE-OFF.

**Technology trade-offs** (from research):
- `gopacket/pcap` (alternative): Uses libpcap cgo bindings. Requires non-distroless base image (Alpine with libc + libpcap). Ruled out due to image size and security posture (cgo in distroless is already unusual; capture sidecar adds unnecessary complexity).
- `tcpdump` (alternative): Third-party binary. Adds ~10MB (tcpdump) + ~500KB (libpcap) to image. Requires subprocess management and careful signal handling. Less precise control; filters are passed as strings to tcpdump, which is simpler but less integrated. Ruled out in favor of direct Go library (smaller, simpler, single process).
- `packetcap/go-pcap` (sole dependency trade-off): Newer library, less proven in production. However, no other pure-Go BPF filter compiler exists; risk is acceptable for MVP with caveats in "Open Risks" below.

**Alternatives considered**:
1. **gopacket/pcap + static libpcap link**: Requires non-distroless build image, statically linked libpcap in the binary (larger artifact), complex cgo setup. Trade-off: distroless compatibility for build complexity. Ruled out; pure-Go path is cleaner.
2. **AF_PACKET + tcpdump for BPF compilation**: AF_PACKET for capture (good), tcpdump for filter validation (subprocess + shell invocation). Trade-off: introduces tcpdump binary dependency just for filter compilation. Use go-pcap/filter instead (pure Go, simpler).
3. **No BPF filter compilation, restrict to whitelist**: Accept only a hardcoded set of filters (e.g., "by_host", "by_port") to avoid filter compiler dependency. Trade-off: reduces flexibility for protocol reverse-engineering (FR-003 demands flexible filter language). Ruled out; users need arbitrary filters.

---

## Filter Language and Validation: API-Tier Compilation with go-pcap/filter

**Decision**: Accept user-provided filter expressions in standard pcap-filter syntax (as defined in pcap-filter(7) man page, e.g., `tcp port 8080`, `host 192.168.1.5 and udp`). Validate filter syntax at the API tier by attempting to compile it with `packetcap/go-pcap/filter` before starting the capture. Return HTTP 400 Bad Request if compilation fails, satisfying FR-003 ("invalid filter expressions MUST be rejected before the capture starts").

**Rationale**:
- **Validation timing** (FR-003): "Invalid filter expressions MUST be rejected before the capture starts." API-tier validation rejects invalid filters at request time (HTTP 400), before the sidecar is involved. ✓ SATISFIES FR-003.
- **Decouples responsibility**: The API server (stateless, horizontally scalable) owns filter validation; the sidecar (resource-constrained) owns capture execution. Clean separation of concerns.
- **Syntax familiarity**: pcap-filter syntax is industry-standard (tcpdump, Wireshark, tshark use it). Users can copy filters from existing tools.
- **Pure-Go compilation**: `go-pcap/filter` package compiles filter expressions without calling out to external libpcap. No binary dependency, no shell invocation, deterministic errors.
- **Bytecode passing**: Once validated, the sidecar receives the compiled BPF bytecode (or filter string if the sidecar re-compiles), ensuring the filter is known-valid before execution.

**Unverified**: `packetcap/go-pcap/filter` handles all pcap-filter(7) syntax edge cases (TCP flags like `tcp[tcpflags] & tcp-syn != 0`, IPv6, VLAN tags). This should be tested with a suite of known-valid and known-invalid expressions before commitment. Current confidence: HIGH (filter syntax is well-standardized), but practical verification recommended.

**Alternatives considered**:
1. **Sidecar-tier validation** (compile filter in capture process): Defers validation to sidecar start. If compilation fails, sidecar returns HTTP error, and the operator sees a failed capture. Trade-off: slower feedback, harder debugging, capture ID issued and later fails. Ruled out; API-tier validation is cleaner.
2. **No validation, fail silently**: Sidecar silently drops packets that don't match a malformed filter. Trade-off: operator has no idea the filter is wrong. Ruled out; unacceptable for FR-003.
3. **Restrict to whitelist** (e.g., only `by_host`, `by_port`, `all`): Avoids complex filter language. Trade-off: limited expressiveness. Ruled out; users need arbitrary filters for protocol reverse-engineering (FR-003 mandates flexibility, not restriction).

---

## Capture State Storage: New Namespaced NetworkCapture CRD Owned by GameServer

**Decision**: Introduce a new **namespaced NetworkCapture CRD** (v1alpha1, gameplane.local group) to track capture state, lifecycle (Pending/Running/Completed/Failed), and metadata. Each NetworkCapture is owned by its parent GameServer via `SetControllerReference`, enabling automatic cleanup and audit trails. Status fields track start time, completion time, phase, captured packet count, and error conditions.

**Rationale**:
- **CLAUDE.md rule 10** ("The operator is authoritative"): NetworkCapture is a first-class resource, materialized and reconciled by the operator. Users can `kubectl get captures` or inspect via dashboard. kubectl apply will produce the same outcome as the API (GitOps-compatible).
- **CRD vs database row trade-off**: Captures are not just ephemeral sessions; they have lifecycle, audit trails, retention policy, and discoverable state. A database row (API-only state) would violate rule 10 and make captures invisible to kubectl users. CRD is the Gameplane pattern. ✓ MATCHES EXISTING ARCHITECTURE.
- **Ownership and garbage collection**: NetworkCapture owned by GameServer means deleting a GameServer cascades to delete its captures (Kubernetes automatic cleanup). ✓ AVOIDS ORPHANED RESOURCES.
- **Short-lived operation semantics**: Unlike Backup (weeks-long retention), captures are ephemeral (hours to days). Status.phase transitions (Pending → Running → Completed/Failed → Expired/Deleted) map cleanly to CRD status fields. ✓ FITS CRD MODEL.
- **Audit trail**: Each NetworkCapture change triggers an audit event (spec.enabled change, phase transition). Audit middleware (api/internal/audit/audit.go) captures Method/Path/Target/Actor/Status for every write. ✓ ENABLES COMPREHENSIVE AUDIT LOGGING (FR-006).
- **Analogy**: Backup CRD (operator/api/v1alpha1/backup_types.go:1-145) uses the same pattern: Spec (user intent), Status (observed state, phases, timestamps), conditions (reasons for failures). NetworkCapture can follow Backup's structure.

**Spec design outline**:
```go
type NetworkCapture struct {
    Spec NetworkCaptureSpec
    Status NetworkCaptureStatus
}

type NetworkCaptureSpec struct {
    ServerRef corev1.LocalObjectRef  // Reference to GameServer
    Filter string                    // pcap-filter expression (validated by API)
    MaxDuration *metav1.Duration     // Max runtime (e.g., 5m)
    MaxSize resource.Quantity        // Max file size (e.g., 100Mi)
    TTLSecondsAfterFinished *int32   // Auto-expire after N seconds
}

type NetworkCaptureStatus struct {
    Phase CapturePhase  // Pending, Running, Completed, Failed
    StartTime *metav1.Time
    CompletionTime *metav1.Time
    PacketsWritten int64
    BytesWritten resource.Quantity
    Message string  // Error reason
    Conditions []metav1.Condition
}
```

**Alternatives considered**:
1. **API database rows only** (no CRD): Captures stored in `api/internal/db/migrations/` config table. Trade-off: invisible to kubectl, violates rule 10, state not reconciled by operator. Ruled out.
2. **Embedded in GameServer spec** (spec.captures []NetworkCaptureRef): Store captures as sub-objects in GameServer spec. Trade-off: clutters GameServer spec, makes captures harder to query independently, retention cleaning is harder (owner-ref cleanup doesn't work for embedded objects). Ruled out.
3. **Cluster-scoped CRD**: NetworkCapture as cluster-scoped (like Module, ModuleSource). Trade-off: namespace isolation is lost (one namespace's captures visible to all). Ruled out; game servers are namespaced, captures should inherit that isolation.

---

## PCAP File Storage: Pre-Provisioned emptyDir on Every Game Pod — **RESOLVED 2026-08-23 (human decision)**

**Decision**: The capture `emptyDir` volume is added to the StatefulSet pod template **unconditionally, for every game pod**, whether or not that GameServer has opted into capture. It is not added lazily when capture is first enabled.

**Rationale — why pre-provisioning is required, not optional**:
- The `pods/ephemeralcontainers` subresource can only add a *container* to a running pod; it CANNOT add a *volume*. `pod.spec.volumes` is immutable on a running pod (Kubernetes rejects a volume-list change on a live pod).
- Therefore, if the capture volume does not already exist in the pod template before the ephemeral container is created, there is no way to attach one at enable-time without recreating the pod — which would defeat the entire premise of US2 (enable without restarting the game container).
- Pre-provisioning the volume in the pod template (present from pod creation, regardless of opt-in) is the only way to keep "enable capture" restart-free, because the ephemeral container injected at enable-time can then mount a volume that was already declared.

**Consequences that MUST be stated plainly wherever this feature is documented, not glossed over**:
- **(a) One-time rolling restart on upgrade.** Adding this volume to the pod template is itself a pod-template change. Every existing game server rolls once, on the release that ships this feature — this is an upgrade-time event, not a per-opt-in event, and it affects GameServers that will never use capture. This MUST be called out as an upgrade note the operator sees before upgrading (release notes / `docs/install.md` upgrade section), not left implicit.
- **(b) FR-001 and SC-007's "byte-identical" wording is no longer literally true and must be amended to a weaker, true claim.** A non-opted-in GameServer's pod DOES carry an empty, unused capture `emptyDir` volume (declared in `pod.spec.volumes`, not mounted into the game container, holding zero bytes). What it does NOT carry is the capture sidecar container — no ephemeral container is ever added, no CAP_NET_RAW capability exists anywhere in that pod, and (per FR-008) the game container's own image/runtime/filesystem/network stack is untouched. See the amendment recorded under "Requirements Needing Spec Amendment" item 5 below (SC-007) and the FR-001 discussion above — both must say "no capture *component* attached," not "byte-identical" without qualification, and `kubectl diff` between a non-capturing and capturing server now shows the sidecar/ephemeral-container addition and (for pre-upgrade vs. post-upgrade servers) the empty-volume addition, not zero diff.
- **(c) The emptyDir SHOULD carry a `sizeLimit`.** This gives a hard disk guard backing FR-002's max-size enforcement and the disk-full edge case discussed elsewhere in spec.md — without a `sizeLimit`, an emptyDir can silently consume node ephemeral storage up to whatever the node allows, undermining the "hard max-size limit" FR-002 promises. The sizeLimit should be set generously above the per-capture `maxSizeBytes` default (see the Cluster-Wide Defaults section) since it is a node-disk backstop, not the primary size enforcement mechanism (the sidecar's own max-size check, enforced during capture, is primary).
- **(d) Whether the volume is also mounted on the agent container depends on the download path, and the agent's existing single-root constraint applies.** `agentVolumeMounts`'s doc comment (`operator/internal/controller/gameserver_rcon.go:105-119`) explains that `spec.storage.extra` volumes are deliberately NOT mounted on the agent: "The agent's file browser (agent/internal/files) is rooted at exactly one path — --data-root ... and rejects any resolved path outside it; it has no notion of a second root ... If a future change wants those directories visible in the Files tab, that requires teaching the agent to serve multiple roots, not just adding a VolumeMount here." The same constraint applies to a capture emptyDir: mounting it on the agent at a path outside `--data-root` would add an unreachable mount with no benefit through the existing `/files/download` endpoint, exactly as it would for an extra volume. Serving captures through the agent's existing file surface therefore requires ONE of: (i) teaching the agent to serve multiple roots (the change the doc comment says would be needed, not yet done), (ii) mounting the capture volume inside the existing data root (rejected below — mixes ephemeral diagnostics with persistent game data), or (iii) not using the agent's general file browser at all and instead exposing a capture-specific download path (on the capture sidecar itself, or a small dedicated agent endpoint scoped only to the capture directory, not the general `/files/*` surface). This document does not resolve which of (i)/(iii) implementation should take — that is an implementation-phase decision — but it must NOT silently mount the capture emptyDir under the agent's `--data-root` alongside game data (see "Persist to game PVC" alternative below, ruled out for the same commingling reason), and must NOT assume `/files/download` "just works" for a volume mounted outside the data root, because it does not.

**Alternatives considered**:
1. **Lazily add the volume only when capture is first enabled**: Would require a pod-template change (and therefore a pod recreation) at the moment of enabling capture — directly violating US2 criterion 2 (enable without restarting the game container). Ruled out; this is precisely the problem pre-provisioning solves.
2. **Persist to game PVC** (write captures to `/data/captures/`): Avoids a second volume, and trivially satisfies the agent's single-root constraint (point (d) above) since it's already inside `--data-root`. Trade-off: bloats the game PVC with ephemeral diagnostics, uses disk space that could be game data, and complicates retention cleanup (captures mixed with game data, and the PVC has no independent `sizeLimit`-style guard the way an emptyDir does). Ruled out.
3. **External S3/object storage**: Sidecar uploads captures to S3 or GCS instead of local emptyDir. Trade-off: requires cloud credentials, network egress out of cluster, adds latency, complicates offline/air-gapped deployments, and does nothing to solve the ephemeral-container-cannot-add-a-volume problem for the *local* working file the sidecar writes while capturing. Deferred to Phase 2 (if captures become long-term compliance artifacts).

---

## Retention and Expiry: Kubernetes TTL with Operator Cleanup, Cluster Max Enforced at API Tier

**Decision**: Use Kubernetes-native `spec.ttlSecondsAfterFinished` field on NetworkCapture CRD for auto-expiry. Default TTL: **24 hours** (cluster-configurable via Helm value `capture.defaultRetentionSeconds`) — this is the value spec.md's FR-007 and its "Out of Scope (Architectural Constraints Already Decided)" section fix as the default, and that section is explicitly not open for re-discussion. A prior version of this research document (and downstream artifacts) drifted to a 7-day default without authorization; that drift is reverted here. Cluster-wide **hard max**: **90 days** (Helm value `capture.maxRetentionSeconds`) — spec.md does not fix a specific max value, only that one must exist and cap any per-server setting, so 90 days remains this document's proposed number, distinct from the mandated 24-hour default. API validation rejects capture requests with `ttlSecondsAfterFinished > maxRetentionSeconds`. Operator reconciler (`NetworkCaptureReconciler`) periodically checks `status.completionTime + ttl < now()` and deletes expired captures (job runs every 1 minute, aligned with `BackupSchedule`'s pattern).

**Rationale**:
- **Simplicity over complexity**: Unlike BackupRetention (which has hourly/daily/weekly/monthly buckets for selective keeping), captures are not long-term archives. A simple TTL is sufficient. ✓ MVP-APPROPRIATE.
- **Kubernetes native**: `spec.ttlSecondsAfterFinished` is standard Kubernetes field (used by Jobs, also supported via custom CRDs). Operators familiar with K8s understand it. ✓ IDIOMATIC.
- **Automatic cleanup**: Operator's garbage collector automatically removes objects with expired TTL (if `ttlSecondsAfterFinished` is implemented). No manual cleanup loop needed (though explicit reconciliation can be more reliable). ✓ SELF-HEALING.
- **Cluster max enforcement** (FR-007, "cluster-wide maximum retention cap constrains per-server setting"): API validates at request time that requested TTL ≤ cluster max. Example:
  ```go
  if userTTL > clusterMaxRetention {
      return http.StatusBadRequest, "requested retention exceeds cluster max"
  }
  ```
  ✓ PREVENTS STORAGE BLOAT.
- **Audit trail**: Each expired capture deletion is a Kubernetes DELETE event (auditable via kubectl API server audit log). ✓ COMPLIES WITH FR-006.

**Defaults** (Helm values):
- `capture.defaultRetentionSeconds: 86400` (24 hours)
- `capture.maxRetentionSeconds: 7776000` (90 days)

**Alternatives considered**:
1. **Manual deletion only**: Operator never auto-deletes captures; admins must clean them up manually. Trade-off: captures accumulate, PVC bloats, no safety net. Ruled out; unacceptable for long-running clusters.
2. **BackupRetention-style bucketing** (hourly/daily/weekly): Keep N most recent, N per-day, N per-week, etc. Trade-off: much more complex reconciliation logic, unnecessary for ephemeral captures. Ruled out; overkill for MVP.
3. **Per-GameServer retention override**: Let each GameServer or game template set its own TTL. Trade-off: requires per-template config, complicates API validation (must check each GameServer's TTL against cluster max). Deferred; use cluster-wide default for v1.
4. **Streaming uploads to external storage with local TTL=immediate**: Sidecar uploads capture to S3 immediately, deletes local copy. Trade-off: requires external storage setup, complicates offline use. Deferred to Phase 2.

---

## Concurrency Control: CRD Status Phase-Based Serialization, One Capture Running Per GameServer

**Decision**: Enforce one capture at a time per GameServer via CRD status.phase. The operator's `NetworkCaptureReconciler` examines all captures for a given GameServer; if any capture has phase=Running, new capture requests are rejected (either at API tier with HTTP 409 Conflict, or by the operator marking them Failed with reason "another capture in progress"). Once a capture completes (phase=Completed or Failed), the next queued capture (if any) can start.

**Rationale**:
- **Lock-free via CRD state**: Use the CRD's etcd-backed status.phase as the coordination primitive. No explicit lock object needed; the phase field IS the lock. Example flow:
  1. User requests capture → API creates NetworkCapture with phase=Pending.
  2. Operator's reconciler checks: is there a Running capture on this GameServer? If yes, do nothing (mark request as Conflict). If no, transition to Running.
  3. User stops capture → API patches NetworkCapture to phase=Completed.
  4. Operator notices completion, cleans up sidecar, transitions to Completed.
  5. Next Pending capture (if any) is now eligible to start.
  ✓ SIMPLE, RACES ARE SAFE (etcd is single-writer consistent).
- **FR-012 compliance** ("one capture at a time per server"): Phase transition is atomic via Kubernetes PATCH. ✓ NO RACE CONDITIONS.
- **Matches existing pattern**: GameServer's idle reconciler (operator/internal/controller/gameserver_reconciler.go lines 296-299) uses phase-like logic (status fields with timestamps) to coordinate long-running operations. ✓ CONSISTENT WITH CODEBASE.
- **Backpressure handling**: If a second capture is requested while one is Running, the operator can either (a) mark it Failed with reason "capture already in progress", or (b) leave it Pending and let the user poll for when it transitions to Running. Option (a) is clearer UX.

**Serialization at API tier** (optional but recommended for better UX):
- Before creating a NetworkCapture CRD, check if another capture is already Running on that GameServer.
- If yes, return HTTP 409 Conflict immediately (fail fast, no orphaned CRD).
- If no, proceed to create the CRD.

**Alternatives considered**:
1. **Mutex or Lease object**: Maintain a separate `NetworkCaptureLock` Lease resource per GameServer. Trade-off: extra object, extra RBAC, introduces another coordination primitive. Ruled out; overkill when CRD status fields are sufficient.
2. **Allow concurrent captures, serialize in sidecar**: Accept multiple NetworkCapture requests, let the sidecar handle queueing. Trade-off: complex sidecar logic, hard to expose queue status in CRD, user has no visibility into wait time. Ruled out; better to fail fast at API tier.
3. **Timestamp-based fallback**: If two captures are requested simultaneously, use creation timestamp to pick the one with the earlier timestamp. Trade-off: still requires careful comparison logic, doesn't prevent concurrent starts during network delays. Ruled out; phase-based is clearer.

---

## Cluster-Wide Defaults Mechanism: Helm Values for Immutable Defaults, Optional API Config Table for Tuning

**Decision**: Define cluster-wide capture defaults via Helm values (immutable at cluster install time). Optional: allow runtime tuning via the API's `/admin/config` database table (like audit retention). Specific Helm values:
- `capture.enabled: true` — feature flag (allow/disallow captures cluster-wide)
- `capture.defaultRetentionSeconds: 86400` (24 hours)
- `capture.maxRetentionSeconds: 7776000` (90 days)
- `capture.defaultMaxDurationSeconds: 300` (5 minutes, max per-capture duration)
- `capture.defaultMaxSizeBytes: 5368709120` (5 GiB, max per-capture file size)

Pass these to the operator and API via `--capture-*` flags (api/cmd/main.go pattern, similar to `--audit-retention-days`). Store in memory or query at startup.

**Rationale**:
- **Helm values** (immutable, cluster-install time): Used for infrastructure settings that don't change frequently. Audit retention uses this pattern (docs/install.md:134). ✓ CONSISTENT WITH PRECEDENT.
- **Decouples from operator**: Defaults are set at API / operator startup, not reconciled per-GameServer. Simpler mental model.
- **Optional API config table** (runtime-tunable): If an operator needs to adjust defaults without redeploy, an advanced feature can add `/admin/config?section=capture` endpoint (api/internal/handlers/config.go pattern). Not required for v1. ✓ FUTURE-PROOF.
- **Per-GameServer overrides** (future): Can be added to NetworkCapture spec or GameTemplate spec later, allowing fine-grained control without Helm re-run. ✓ EXTENSIBLE.

**Defaults rationale**:
- **5-minute default max duration**: Typical protocol reverse-engineering session (player joins, exchanges a few packets, leaves). Enough to capture a full client join sequence for most games.
- **5 GiB default max size**: Balances safety (prevents runaway captures filling disk) with flexibility (full 5-minute capture on typical game traffic is ≤ GiB easily). Configurable per-request via NetworkCapture spec if needed.
- **24-hour default TTL**: Captures are disposable diagnostics. Operators have one day to download and archive captures they want to keep, per spec.md FR-007 and its Out of Scope/Architectural Constraints section (mandated, not a research recommendation).
- **90-day cluster max**: Hard cap prevents accidental year-long storage of sensitive player data. Balance between safety and operational flexibility.

**Alternatives considered**:
1. **API database config table only** (no Helm): All defaults tunable at runtime. Trade-off: no immutable baseline (cluster admin's intent not visible in Helm values, harder to audit). Ruled out; Helm values should be the source of truth.
2. **Per-GameTemplate defaults** (spec.captureDefaults): Each template can set its own retention. Trade-off: complicates API validation (must merge cluster max + template defaults), scattered configuration. Ruled out; cluster-wide defaults first, per-template later if needed.
3. **Hardcoded defaults** (no Helm): Fixed 24-hour retention, 5 GiB max, etc. Trade-off: no flexibility for different clusters (edge cases: air-gapped clusters might want stricter limits; internal labs might want longer retention). Ruled out; Helm values enable flexibility.

---

## Security Posture: CAP_NET_RAW Sidecar Only, Game Container Unchanged, Audit Logging Mandatory

**Decision**: Capture sidecar runs with **CAP_NET_RAW capability only** (drop all others, including CAP_NET_ADMIN) in its own container's securityContext. Game container retains its existing unprivileged security posture (no elevated capabilities). All capture operations (start, stop, download, delete) are logged to the audit table with actor, timestamp, operation type, and result (FR-006). Captures contain real player data (IP addresses, game chat, credentials); access is admin-only (FR-005); default retention is 24 hours (spec.md FR-007, data sensitivity). Non-opted-in GameServers have no capture component attached — no capture sidecar, no CAP_NET_RAW anywhere in the pod — but, per the storage decision above, they DO carry the pre-provisioned, empty, unmounted capture volume (FR-008; SC-007 as amended).

**Rationale**:
- **Capability separation** (FR-008, SC-007): Only the capture sidecar has CAP_NET_RAW; the game container has no elevated capabilities. Exploit of the game code cannot lead to packet capture ability. ✓ PRESERVES SECURITY BOUNDARY.
- **CAP_NET_RAW sufficiency**: CAP_NET_RAW permits raw socket access (AF_PACKET) for read-only packet capture. CAP_NET_ADMIN is not required for capture-only use (it's broader: network namespace admin, device config, etc.). ✓ LEAST PRIVILEGE.
- **No capture component, not "byte-identical"** (SC-007, amended): GameServers without `spec.capture.enabled=true` have no capture sidecar container, no ephemeral container, no service entry. They do carry the empty capture `emptyDir` volume declared in the pod template (see the storage decision above) — that volume exists on every game pod, opted in or not, because the ephemeral-container mechanism cannot add a volume later. The claim is "no capture component attached," not literal byte-identity of the pod spec. ✓ SATISFIES SC-007 AS AMENDED, NOT AS ORIGINALLY WORDED.
- **Audit logging** (FR-006): Automatically captured by api/internal/audit middleware (audit.go:579-657). Every capture operation (POST/DELETE) emits an event with Actor/Method/Path/Target/Status. ✓ COMPREHENSIVE TRAIL.
- **Data sensitivity** (SC-007): Captures contain binary game protocols (headers, payloads, IP addresses, sometimes in-game chat or credentials). Default 24-hour TTL and admin-only access reduce exposure window. ✓ DEFENSIBLE RETENTION.

**RBAC enforcement** (FR-005): The API enforces capture access control via the rule-table mechanism in `api/internal/rbac/rbac.go:237` (func match; the preceding line, `rbac.go:236`, is only its doc comment). A new permission key `captures:manage` is added to `api/internal/rbac/catalog.go:27` and granted to admin role only in a new append-only migration to `api/internal/db/migrations/` (see the "Audit Reason Column" decision below for that migration's number). Capture routes (e.g., `POST /servers/{name}:capture-start`) MUST be inserted into the rule table BEFORE the catch-all rule at `rbac.go:184` ({segment: "servers", perm: "servers:write"}), ensuring first-match-wins ordering directs capture routes to `captures:manage` rather than `servers:write`. The operator role holds `servers:write` (migrations/003_roles.sql:48), so rule ordering is load-bearing for security: captures are admin-only via rule precedence, not inherited permission. Any capture operation by non-admin users returns HTTP 403 Forbidden.

**Verification (test recommendation)**: Before implementation, run a proof-of-concept ephemeral container with CAP_NET_RAW on the target K8s version to confirm AF_PACKET can create raw sockets without CAP_NET_ADMIN.

**Alternatives considered**:
1. **CAP_NET_RAW + CAP_NET_ADMIN**: Broader permissions, safer for unknown use cases. Trade-off: violates least privilege, CAP_NET_ADMIN includes network namespace and device admin rights (overkill for read-only capture). Ruled out.
2. **Unprivileged capture** (no capabilities, use eBPF or other mechanism): Avoids capability requirement entirely. Trade-off: eBPF is newer (Linux 5.8+), more complex, less proven on diverse game workloads. Deferred to Phase 2 if CAP_NET_RAW is unacceptable.
3. **Shared capture pod** (separate pod with network namespace sharing): Isolate capture to a different pod, no sidecar capability. Trade-off: Kubernetes doesn't natively support network namespace sharing between pods; would require custom plumbing. Operationally complex. Ruled out.

**Pod Security Standards compliance**: If a cluster runs `podSecurity.enforceRestricted=true` (restricted admission profile), pods with CAP_NET_RAW will be rejected. Game pods are not locked to restricted by default (docs/security.md:214-216), but an operator who does so will need to exempt the games namespace or disable capture. Document this trade-off in FR-009 (capture feature availability depends on PodSecurity admission level).

---

## Capability Mechanism: File Capabilities via `setcap`, Not Root — **RESOLVED 2026-08-23 (human decision)**

**Decision**: The capture container acquires `CAP_NET_RAW` via **file capabilities on its binary** (`setcap cap_net_raw+ep <binary>`), applied at image-build time. It does **not** run as root, and it does not rely on `securityContext.capabilities.add: ["NET_RAW"]` on a non-root `runAsUser` to grant the capability at runtime.

**Rationale — why `capabilities.add` alone does not work**: Kubernetes does not set ambient capabilities on a container's process. `securityContext.capabilities.add: ["NET_RAW"]` combined with a non-root `runAsUser` grants nothing at `execve()`: without the ambient set, a non-root process's added capabilities are cleared across the exec of the container's entrypoint binary. File capabilities (an xattr on the binary itself, consulted by the kernel at `execve()` independent of ambient state) are the mechanism that actually survives exec for a non-root process.

**Consequences that MUST be stated plainly wherever the capture sidecar's securityContext appears (docs, plan.md, quickstart.md, the actual manifest/injection code)**:
- **(a) `allowPrivilegeEscalation: true` is required.** File capabilities are ignored by the kernel when `no_new_privs` is set, and Kubernetes sets `no_new_privs` whenever `allowPrivilegeEscalation: false`. This container therefore MUST set `allowPrivilegeEscalation: true`. This is an explicit, accepted trade-off: `runAsNonRoot: true` is preserved (the process still runs as an unprivileged UID), but `allowPrivilegeEscalation` is given up in exchange. Do not present this container's securityContext as fully hardened without naming this trade plainly.
- **(b) PodSecurity `restricted` forbids this.** The `restricted` Pod Security Standards profile disallows `allowPrivilegeEscalation: true` outright. A cluster enforcing `restricted` on the games namespace will reject this pod at admission. This is the same namespace-exemption obligation already noted under "Pod Security Standards compliance" above (and Open Risk 9), but the cause is now specifically `allowPrivilegeEscalation`, not just "has a capability." This MUST be recorded as a `docs/security.md` obligation: document that capture requires baseline (or an explicit `restricted` exemption) admission on the games namespace, and why.
- **(c) UNVERIFIED BUILD RISK — file capabilities may not survive image build.** File capabilities are stored as an xattr (`security.capability`) on the binary's inode. This repo has **already hit COPY-time file-mode problems** building game images (see `fix(images): set entrypoint mode at COPY time instead of chmod`, recent history on this repo). Whether a multi-stage Docker/Buildx `COPY` into a distroless/scratch final stage preserves that xattr is genuinely unverified for this build pipeline — some overlay/union filesystem and `COPY` implementations drop xattrs, and distroless/scratch base images have no `setcap` tooling available at container-build time to reapply it in the final stage. **This document does not assert that the capability survives the image build.** It MUST be proven empirically in CI (build the actual multi-stage image, run a container from it, inspect `getcap` on the binary and/or attempt an actual `AF_PACKET` socket open as the non-root user) before this approach is trusted for implementation. See Open Risk 10 below.

**Alternatives considered**:
1. **Run the capture container as root**: Root has `CAP_NET_RAW` (and everything else) by default; simplest to get working. Trade-off: violates least-privilege far more severely than a single scoped capability, and contradicts the project's non-root-by-default posture for every other component. Rejected by explicit human decision (D-SETCAP).
2. **`capabilities.add: ["NET_RAW"]` with non-root `runAsUser`**: The naive Kubernetes-native approach. Trade-off: does not work — see Rationale above; Kubernetes does not set ambient capabilities, so the effective set is empty after `execve()`. Ruled out as non-functional, not merely undesirable.
3. **File capabilities via `setcap` at build time (chosen)**: Works with a non-root `runAsUser`, at the cost of `allowPrivilegeEscalation: true` and unverified survival through the image build pipeline. Chosen as the human-decided trade-off, with the build-survival question left explicitly open (Open Risk 10).

---

## E2E Test Strategy and Bucket Placement: Operator Bucket, Structural Testing

**Decision**: Add capture testing to the **operator** e2e bucket (test/e2e/buckets.sh:36-82). Test name: `TestGameServer_NetworkCaptureEphemeralContainer`. Test validates **structural correctness** only: operator injects the capture sidecar ephemeral container, wires volumes/mounts, and pod spec reflects the expected configuration. No functional testing (no real game traffic through capture) for v1; functional validation deferred to gamebot bucket if capture output verification is required.

**Rationale**:
- **Operator bucket characteristics** (buckets.sh:24-34): 45 existing tests, 8-way parallel (no login bottleneck), 35min wall-clock timeout, fast per-test (no large image pulls, no long-running game servers). ✓ MATCHES CAPTURE TEST PROFILE.
- **Zero logins required**: Capture testing does not need APIClient or authentication (no dashboard interaction). Pure operator reconciliation testing. ✓ KEEPS LOGIN BUDGET UNAFFECTED.
- **Structural testing scope** (operator reconciliation):
  1. Create a GameServer with `spec.capture.enabled=true`.
  2. Wait for pod Running.
  3. Verify StatefulSet spec.template.spec.ephemeralContainers contains the capture sidecar (name, image, securityContext with CAP_NET_RAW, volume mounts for emptyDir).
  4. Verify emptyDir volume exists and is mounted on both game and capture sidecar.
  5. Verify sidecar has no inherited network policies (inherits game pod's default-deny-egress).
  ✓ TESTS OPERATOR BEHAVIOR, NOT GAME BEHAVIOR.
- **No game traffic**: Avoids e2e complexity (no game image pull, no real join client, no protocol parsing). Fast feedback on operator correctness.
- **Follows operator bucket pattern**: Existing tests like `TestGameServer_OperatorMaterializesChildren` (gameserver_e2e_test.go:31) create GameServers and inspect materialized objects (StatefulSet, Services, PVCs). Capture test is analogous.

**Test structure**:
```go
func TestGameServer_NetworkCaptureEphemeralContainer(t *testing.T) {
    t.Parallel()
    // 1. Create GameTemplate
    // 2. Create GameServer with spec.capture.enabled=true
    // 3. Wait for pod Running
    // 4. Fetch pod; verify ephemeralContainers[0] matches capture sidecar spec
    // 5. Assert securityContext.capabilities.add = [NET_RAW]
    // 6. Assert volumeMounts include emptyDir
    // 7. Create GameServer without capture; verify NO ephemeralContainers
}
```

**Functional testing** (future, optional):
- If the feature requires verification that captured packets are correctly written and downloadable, add a test to the gamebot bucket (bot-fast) that:
  1. Starts a real game server.
  2. Initiates a capture.
  3. Sends traffic (via in-cluster probe) matching a filter.
  4. Stops the capture.
  5. Downloads the pcap file.
  6. Verifies the file is valid PCAPNG and contains expected packets.
  This is deferred to Phase 1 (implementation phase) if needed.

**Bucket placement decision rationale**:
- gamebot bucket (bot-fast) is for testing game server functionality (join, RCON, players, mods). Capture's functional path (capture real traffic) could go there, but only if output verification is required.
- operator bucket is for testing operator reconciliation (spec → materialized objects). Capture's sidecar injection is pure reconciliation, fits naturally.
- Start with operator bucket (v0 testing); if downstream e2e needs capture functional verification, add gamebot test in v1.

**Alternatives considered**:
1. **New bucket for capture**: Create a dedicated capture bucket. Trade-off: expensive (new kind cluster, 35–50min job), unnecessary overhead for MVP. Ruled out.
2. **api-agent bucket**: If capture is API-driven (not operator-injected). Trade-off: would require login overhead. Ruled out; capture is operator-injected, not API-only.
3. **Functional testing only (no structural)**: Skip the operator test, jump to gamebot functional validation. Trade-off: slower debugging if structural injection fails; gamebot tests are heavier (game images, longer boot). Ruled out; fast structural test catches operator bugs early.

---

## CRD Type Definition and Codegen: NetworkCapture in operator/api/v1alpha1, run `make generate && make manifests`

**Decision**: Define NetworkCapture CRD types in `/home/valgul/project/kubernetes-game-dashboard/operator/api/v1alpha1/networkcapture_types.go` (new file) with kubebuilder markers. Run `make generate && make manifests` to regenerate RBAC and CRD YAML. Commit the regenerated files (`operator/config/rbac/*.yaml`, `operator/config/crd/*.yaml`, `charts/gameplane/crds/` copies) in the same commit as the type definitions (CLAUDE.md rule 7).

**Rationale** (from CRD Patterns research):
- **Codegen contract** (Makefile, CLAUDE.md rule 7): Any edit to `operator/api/v1alpha1/*_types.go` must be followed by `make generate && make manifests` in the same commit.
- **File location**: Namespaced CRD types go in `operator/api/v1alpha1/`. Backup and Restore are in the same directory (backup_types.go, restore_types.go). ✓ CONSISTENT.
- **Kubebuilder markers**: Add markers like:
  ```go
  // +kubebuilder:object:root=true
  // +kubebuilder:resource:shortName=cap
  // +kubebuilder:subresource:status
  // +kubebuilder:printcolumn:name="Server",type=string,JSONPath=`.spec.serverRef.name`
  // +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
  // +kubebuilder:rbac:groups=gameplane.local,resources=gamecaptures,verbs=get;list;watch;create;update;patch;delete
  ```
- **RBAC auto-generation**: The `// +kubebuilder:rbac:*` markers feed into `make manifests`, which regenerates `operator/config/rbac/*.yaml` with the required rules.
- **Chart sync**: `make manifests` copies regenerated CRD YAML from `operator/config/crd/` to `charts/gameplane/crds/` and `charts/gameplane/crd-manifests/` (pre-upgrade hook location).

**Operator reconciler** (new file `operator/internal/controller/networkcapture_controller.go`):
- Reconciles NetworkCapture CRD status (Pending → Running → Completed/Failed → Expired).
- Watches GameServer to validate serverRef and enforce concurrency (one Running per GameServer).
- Calls the API to start/stop sidecar capture via a new capture controller (separate from game controller).

**Alternatives considered**:
1. **Embed in GameServer spec** (spec.captures []NetworkCaptureRef): No separate CRD. Trade-off: no independent RBAC/audit for captures, harder to query. Ruled out; CRD-first is cleaner.
2. **Cluster-scoped CRD**: Deviates from namespaced pattern. Ruled out; game servers are namespaced.

---

## API Routes and Sidecar Communication: RESTful Capture Lifecycle Endpoints

**Decision**: Expose the following REST endpoints for capture management (all admin-only, RBAC-gated by `captures:manage` permission):

- **POST `/servers/{name}:capture-start`** → Request body: `{filter: "tcp port 8080", maxDurationSeconds: 300, maxSizeBytes: 5368709120}` → Response: `{captureId: "cap-<uuid>", phase: "Pending"}`
- **POST `/servers/{name}:capture-stop`** → Request body: `{captureId: "cap-<uuid>"}` → Response: `{phase: "Completed"}`
- **GET `/servers/{name}:captures`** → List all captures for the server (running, completed, expired). Response: `[{id: "cap-<uuid>", phase: "Running", startTime: "...", bytesWritten: 0, ...}]`
- **GET `/servers/{name}:capture-file?id={id}`** → Download the pcap file. Proxied to the capture sidecar's HTTP endpoint (`GET /captures/{id}/file`, see below).
- **DELETE `/servers/{name}:capture?id={id}`** → Delete an expired or failed capture. Response: `{deleted: true}`

**Superseded shape — retracted here**: an earlier version of this list specified the list/download/delete routes with a variable capture ID as a path segment — `GET /servers/{name}/captures`, `GET /servers/{name}/captures/{id}/file` (proxied via `/ws/servers/{name}/captures/{id}/file`, WebSocket then HTTP fallback), and `DELETE /servers/{name}/captures/{id}`. That nested shape is retracted: `rbac.match` (`api/internal/rbac/rbac.go:228-237`) matches routes by a fixed method+segment+suffix triple and cannot match a variable middle path segment, so under the nested shape these three routes would silently fall through to the generic `servers:read`/`servers:write` rules instead of `captures:manage` — reopening the FR-005 authorization hole. The authoritative routes are the fixed-suffix `:captures`/`:capture-file`/`:capture` forms above, with the capture ID passed as a query parameter; see `contracts/rest-api.md` ("Routing Conventions" and the per-endpoint sections) for the full rationale and RBAC rule table.

**Sidecar HTTP endpoints** (capture service, listening on `:9091` inside the pod):
- **POST `/captures/{id}:start`** → Start capture, receive filter and config from request body. Response: `{status: "running", bytesWritten: 0}`
- **POST `/captures/{id}:stop`** → Stop capture. Response: `{status: "completed", bytesWritten: N, packetsWritten: M}`
- **GET `/captures/{id}/file`** → Stream the captured pcap file. Headers: `Content-Type: application/vnd.tcpdump.pcap`, `Content-Disposition: attachment; filename="capture-<id>.pcapng"`

**Rationale**:
- **REST (vs gRPC, vs WebSocket)**: REST endpoints are easier to test, align with existing API patterns (handlers/lifecycle.go, handlers/modules.go), and are compatible with the audit middleware (HTTP Method/Path are logged). ✓ MATCHES CODEBASE STYLE.
- **POST with `:action` suffix** (`POST /servers/{name}:capture-start`): Follows Kubernetes-style custom actions (e.g., `/servers/{name}:quiesce`, `/servers/{name}:unquiesce` in quiesce.go). ✓ FAMILIAR PATTERN.
- **Sidecar API listening on `:9091`** (separate port from agent `:8090`): Allows independent RBAC/firewall rules and cleaner endpoint separation.
- **mTLS between API and sidecar**: Sidecar runs with Agent TLS cert/key, validates against same client CA as agent. ✓ NO NEW TLS SETUP NEEDED.

**Audit logging** (automatic):
- Each endpoint is an HTTP handler mounted in `api/cmd/main.go`.
- Audit middleware (api/internal/audit/audit.go:579) logs every write (POST, DELETE) with Actor/Method/Path/Target/Status.
- Example: `POST /servers/my-game:capture-start` from user=admin → audit event with actor=admin, method=POST, path=/servers/my-game:capture-start, target=my-game, status=202 (Accepted).
- **Audit schema constraint** (audit.go:727-736): The audit.Event struct contains {ID, TS, Actor, Method, Path, Target, Status, IP}; there is NO payload/result/operation/reason column. **RESOLVED 2026-08-23 (human decision)**: see the "Audit Reason: Structured Column via New Migration" decision below — a new `reason` column is added via a new append-only migration, not encoded into Target/Path. That decision also covers the risk this raises for the existing hash-chain migration (`005_audit_chain.sql`).

**Alternatives considered**:
1. **WebSocket for all operations** (single stream for control + data): Reduces round-trips, can stream capture events live. Trade-off: more complex client code, harder to test with curl/Postman. Ruled out; REST is simpler for MVP.
2. **GraphQL mutation**: Trade-off: adds GraphQL complexity, not aligned with existing chi REST API. Ruled out.
3. **Command-line only** (no REST endpoints): Operators manage captures via `kubectl patch NetworkCapture`. Trade-off: dashboard has no UI, poor UX. Ruled out; REST endpoints enable dashboard integration.

---

## Sidecar Addressing: Existing `<gs>-agent` Service, Numerically-Targeted Second Port — **RESOLVED 2026-08-23 (human decision)**

**Decision**: The API/operator dials the capture sidecar's `:9091` HTTP endpoint through the **existing `<gs>-agent` ClusterIP Service**, by adding a second port (numeric target `9091`) to that Service. It does **not** dial the pod IP directly, and it does not add any IP SAN or new certificate.

**Correction of a prior hand edit**: An earlier version of this document claimed "no Service can front an ephemeral container." That claim is **false** and is retracted here. A Kubernetes Service selects **Pods** (via label selector), not containers — a Service has no notion of which container within a matched pod owns a given port. The real, narrower constraint is only this: an ephemeral container cannot declare a named `containerPort` (the `ephemeralcontainers` subresource does not support `ports` the way `spec.containers` does), so a Service fronting a port the capture sidecar opens must target that port **numerically** (`targetPort: 9091`), not by name.

**Verified facts this decision rests on**:
- There is already a dedicated `<gs>-agent` ClusterIP Service, per the doc comment on `agentDNSNames` (`operator/internal/controller/agent_certs.go:201-206`): "The canonical path is the dedicated `<gs>-agent` ClusterIP Service (api/internal/ws/dialer.go, operator/internal/agent/client.go)."
- The agent's TLS cert SANs (`agentDNSNames`, `operator/internal/controller/agent_certs.go:207-223`) already include `<gs>-agent`, `<gs>-agent.<ns>`, `<gs>-agent.<ns>.svc`, and `<gs>-agent.<ns>.svc.cluster.local` (lines 219-222).
- **Verified by direct inspection**: `agent_certs.go` contains **no `IPAddresses` field anywhere** (grepped for `IPAddresses`, zero matches) — the cert has no IP SAN at all. Dialing a pod IP directly over this mTLS setup would therefore **fail certificate verification** (no SAN covers a bare IP).
- The SAN list also does **not** contain a bare `<pod-name>.<namespace>.svc.cluster.local` form (a pod hostname combined directly with the namespace, skipping the governing/service name) — every pod-based SAN entry (lines 211-218) interposes a service name (`gs.Name` or `agentSvc`) between the pod hostname and the namespace, e.g. `<pod>.<gs.Name>.<ns>.svc.cluster.local`, not `<pod>.<ns>.svc.cluster.local`.

**Consequence**: Because the existing `<gs>-agent` Service's DNS names are already covered by the agent cert's SANs, adding a second, numerically-targeted port (9091) to that same Service and dialing it by its existing DNS name requires **no new certificate, no IP SAN, and no `ServerName` override** on the client side. This is strictly less new surface than either minting a new Service (with its own DNS name requiring a SAN addition) or dialing the pod IP (which cannot pass certificate verification today).

**Scope note**: Adding the second port to the `<gs>-agent` Service definition is **operator work** (the Service object is materialized by the operator, per the same reconciler that owns the agent Service today) — it is not an API-tier or sidecar-tier change.

**Alternatives considered**:
1. **Dial the pod IP directly**: Simplest in theory (no Service indirection). Ruled out: the agent cert has no IP SAN (verified above), so this fails TLS verification outright; adding an IP SAN would mean re-issuing every agent cert and is a materially larger change than adding a Service port.
2. **A brand-new, capture-specific Service**: Cleanly separates concerns. Trade-off: a new DNS name needs a new SAN entry (or a new cert), and a second Service object to reconcile/own/clean up for no benefit over reusing the existing agent Service. Ruled out in favor of extending the Service that already exists and is already trusted.
3. **Named `containerPort` on the ephemeral container + Service targeting the name**: Not available — the `ephemeralcontainers` subresource does not support declaring `ports`/`containerPort` the way regular containers do (this is the actual constraint the retracted "no Service can front an ephemeral container" claim was groping toward). A Service must therefore target the port numerically.

---

## Audit Reason: Structured Reason Column via New Migration — **RESOLVED 2026-08-23 (human decision)**

**Decision**: Add a structured `reason` column to `audit_events` via a **new append-only migration**, rather than encoding capture-specific detail (filter used, stop reason, capture ID) into the existing `Target`/`Path` fields.

**Numbering**: Migrations live in `api/internal/db/migrations/` and are append-only, numbered sequentially. The directory currently holds `001_init.sql`, `002_config.sql`, `003_roles.sql`, `004_cluster_rbac.sql`, `005_audit_chain.sql`, and `006_share_links.sql` — the highest existing number is 006, so the new migration for this feature is **`007_audit_reason.sql`** (or similar name), not `006` as a stale reading of the directory might suggest, and not any number chosen without re-checking the directory at implementation time (it may have gained further migrations by then).

**Critical risk — the hash chain**: `005_audit_chain.sql` adds `prev_hash`/`hash` columns to `audit_events` so that a DB-level UPDATE or DELETE against a row is detectable: each row records the hash of the row before it (`prev_hash`) and its own content hash (`hash`), computed by the API at insert time (see `api/internal/audit.Auditor.insertChained`, referenced in that migration's own header comment). Adding a new column to this table is therefore **not a capture-only concern** — it touches integrity-sensitive code shared by every audited operation in the system, not just capture. This MUST be treated with the same care as touching the chain computation itself, and reviewed as such rather than as an incidental schema tweak.

**What this document resolves explicitly**: the new `reason` column **DOES participate in the hash computation** for every row written after this migration ships — `insertChained` must include `reason` in the content hash it computes, the same as it includes `Method`/`Path`/`Target`/etc. today. The alternative (excluding `reason` from the hash) would let an attacker with DB write access alter capture reasons undetected while every other field stayed chain-verified, which defeats the purpose of the chain for exactly the events (capture start/stop/download/delete) this feature cares most about auditing.

**Consequence for pre-existing rows**: Rows written before this migration ships have no `reason` value (NULL) and were never hashed with a `reason` field in the first place — their existing `hash`/`prev_hash` values were computed over the pre-migration column set and remain valid as-is; `insertChained` must NOT attempt to retroactively rehash them. `Verify` (the chain-walking function referenced in `005_audit_chain.sql`'s own comment as treating the pre-005 boundary as "a fresh genesis rather than a break") needs the equivalent treatment at this boundary: verification of rows written before `007_audit_reason.sql` must hash them without a `reason` field (as they were originally computed), and rows written after must include it. Getting this wrong either breaks `Verify` for every pre-existing row in the database (false tamper reports) or silently accepts a hash computed without `reason` for rows that should include it (a verification gap). This is implementation-critical, not a documentation nicety.

**Alternatives considered**:
1. **Encode into Target/Path** (no schema change): `target=<serverName>/<captureId>`, stop reason appended to the path. Trade-off: no hash-chain risk, but limited to short opaque strings, awkward to query, and conflates capture identity with capture reason in one field. Ruled out in favor of a real column now that the hash-chain implications are understood and addressed above.
2. **Separate `capture_audit_metadata` table**: Keeps `audit_events` untouched, avoids the hash-chain question entirely. Trade-off: two audit trails to correlate, and capture-specific detail falls outside the tamper-evident chain altogether (an attacker could edit `capture_audit_metadata` freely with no detection). Ruled out; the whole point of FR-006 is a complete, trustworthy record of capture operations, which argues for extending the chain-covered table, not creating an uncovered sibling.

---

## Error Response Format: Existing `httperr` Plain-Text Convention — **RESOLVED 2026-08-23 (human decision)**

**Decision**: This feature introduces NO new JSON error envelope. All capture-endpoint error responses conform to the existing `api/internal/httperr` package, which is verified (by reading `httperr.go`) to write **plain text** via `http.Error`, not JSON.

**Verified behavior**:
- `httperr.WriteCode` writes a specific HTTP status with a plain-text message via `http.Error`: for a >=500 status it writes the generic `http.StatusText(status)` (never echoing the underlying error to the client, only logging it server-side via `slog.Error`); for a 4xx it echoes `err.Error()` verbatim, by convention only for errors the handler has already classified as safe to show.
- `rbac.Middleware` writes a literal plain string for auth failures before any handler runs — not a JSON body.
- Neither writes `{"code": ..., "message": ...}` or `{"error": ..., "message": ...}` or any other JSON envelope.

**Correction**: earlier drafts of this feature's design artifacts (`rest-api.md`, `quickstart.md`) invented two different, mutually inconsistent JSON error envelopes for capture endpoints. Both are wrong and must be removed wherever they appear (outside the scope of this document's own edit, but noted here since this document is the canonical decision record). Capture endpoint errors — 400 on invalid filter, 403 on non-admin access, 409 on concurrent capture, 404/expired on a retention-expired download — all go through the same plain-text `httperr.WriteCode`/`httperr.Write` path every other handler in `api/internal/handlers/` uses. No handler-specific exception is introduced.

---

## Open Risks

### 1. go-pcap/filter Library Maturity, Missing Tagged Release, and Edge Cases
- **Risk**: `packetcap/go-pcap/filter` exists and exports filter.Compile, but has NO TAGGED RELEASE — only pseudo-version v0.0.0-20260731105150-c86974bbfbcd from the module proxy. This untagged-module dependency carries supply-chain risk: no stable point to pin to; unforeseen upstream changes could break the build; vendoring is complex without a tag. Additionally, it is a newer library with less proven production usage than libpcap, and edge cases (TCP flags, IPv6, VLAN tags, complex expressions) may not be fully compatible with pcap-filter(7) spec.
- **Mitigation**: (1) **Vendoring**: Pin the exact pseudo-version in go.mod and vendor the module with `go mod vendor` (document the pseudo-version pin in comments for clarity). (2) **Testing**: Before implementation, test the filter compiler against a suite of known-valid and known-invalid expressions (at least 50 test cases covering edge cases). (3) **Upstream tracking**: Monitor the packetcap/go-pcap repository for a tagged release; if one becomes available during implementation, upgrade and re-test. If no tagged release materializes before v1 release, document this as a known supply-chain limitation in release notes.
- **Timeline**: Vendoring and verification recommended in Phase 0 (before commitment) or early Phase 1. Low risk if tested and vendored; high risk if not.

### 2. AF_PACKET Performance and Packet Loss Under High Load
- **Risk**: At very high packet rates (e.g., 100k packets/sec on a heavily-played game server), AF_PACKET's MMap'd buffers may not keep up, leading to kernel buffer overflow and dropped packets.
- **Mitigation**: Benchmark AF_PACKET throughput on a realistic game server workload (e.g., Minecraft with 20 concurrent players) before Phase 1 implementation. If performance is inadequate, consider fallback to simple socket-based capture (less efficient but simpler) or tcpdump (tested, proven, but heavier image).
- **Timeline**: Benchmarking recommended in Phase 0 or early Phase 1.

### 3. Distroless + CAP_NET_RAW Interaction
- **Risk**: Unconfirmed whether distroless/static:nonroot can successfully acquire raw sockets with CAP_NET_RAW capability only (no CAP_NET_ADMIN). Linux kernel behavior varies slightly by version; K8s versions have different security defaults.
- **Mitigation**: Run a quick PoC on the target K8s version (e.g., on the e2e kind cluster or kubelab) before implementation. Create an ephemeral container with CAP_NET_RAW in a distroless base and attempt to create an AF_PACKET socket. If successful, risk is mitigated. If it fails (requires CAP_NET_ADMIN), escalate to that capability or use unprivileged capture (eBPF).
- **Timeline**: Test recommended in Phase 0 before design finalization. No-blocker if confirmed; blocker if AF_PACKET fails without CAP_NET_ADMIN.

### 4. Ephemeral Container Lifecycle on Pod Template Updates
- **Risk**: When a GameServer's pod template is updated (e.g., game code deploy, new operator release), the StatefulSet triggers a rolling update, the old pod is deleted, and a new pod is created. Ephemeral containers are NOT replicated; they are lost. If a user has an active capture running, it is abruptly terminated without graceful cleanup.
- **Mitigation**: Operator must handle pod restart by detecting it (watch pod events) and cleaning up the associated NetworkCapture CRD (mark as Failed with reason "pod restarted"). Dashboard should warn users that active captures are lost on pod restart.
- **Timeline**: Mitigation needed in Phase 1 (implementation). Not a deal-breaker; expected behavior for ephemeral containers.

### 5. Snaplen Selection and Game Protocol Safety
- **Risk**: If snaplen is too low (e.g., 128 bytes), some game protocols with unconventional headers may be truncated, losing important information for reverse-engineering.
- **Mitigation**: Document recommended default snaplen (likely 65535 for full packet) and educate users that snaplen is configurable. Provide UI hints in dashboard (capture advanced options). Default to full-packet capture (snaplen=65535) unless storage is a concern.
- **Timeline**: Documentation recommended in Phase 1. Not a blocker; conservative default (full packet) is safe.

### 6. Filter Compilation Syntax Mismatch Between go-pcap/filter and libpcap
- **Risk**: If API validates with go-pcap/filter but the sidecar later uses libpcap (or tcpdump), they may accept/reject filters differently, leading to silent failures or unexpected behavior.
- **Mitigation**: If the sidecar uses libpcap (e.g., via tcpdump), ensure API validation uses the same library (statically linked libpcap) or verifies filters twice (API + sidecar). For pure-Go sidecar using AF_PACKET, no secondary library is needed; go-pcap/filter is the sole source of truth.
- **Timeline**: Relevant only if sidecar design changes away from pure-Go. Deferred to Phase 1.

### 7. Capture File Corruption on Pod Eviction
- **Risk**: If a pod is evicted or deleted while a capture is writing to a file, the file may be left in a partially-written, corrupted state. pcapgo's Writer doesn't have explicit "flush" or "finalize" semantics; the caller must close the file properly.
- **Mitigation**: Sidecar must use `defer file.Close()` to ensure files are closed even if the process is killed. For reliability, consider writing a `.incomplete` suffix to partial captures and cleaning them up on pod restart. Document this edge case (Assumption 4 in spec.md).
- **Timeline**: Mitigation needed in Phase 1. Not a deal-breaker if cleanup is documented.

### 8. Cluster Max Retention Enforcement Edge Cases
- **Risk**: If a user attempts to set `spec.ttlSecondsAfterFinished` to a value > cluster max, the API rejects it. But if the cluster max is lowered after a capture is created with the old higher TTL, the old capture may not auto-expire until its original TTL fires. Inconsistency risk.
- **Mitigation**: On cluster max change (via Helm upgrade), run a reconciliation pass on all NetworkCaptures to update their TTL to min(current TTL, new max). Or document that lowering the max does not retroactively expire existing captures.
- **Timeline**: Low-priority; edge case. Deferred to Phase 2 or documented as known limitation in Phase 1.

### 9. PodSecurity Restricted Admission Incompatibility
- **Risk**: If a cluster operator enables PodSecurity admission level `restricted` on the games namespace, pods with CAP_NET_RAW will be rejected by the admission controller, and capture sidecar injection will fail (pod never starts).
- **Mitigation**: Document that capture requires baseline or privileged admission level. If restricted is required, either exempt the games namespace, disable capture, or add eBPF-based unprivileged capture (Phase 2). Operator should receive clear error message when attempting to enable capture in restricted namespace.
- **Timeline**: Mitigation needed in Phase 1 (API validation + clear error message). Document trade-off in FR-009.

### 10. File Capabilities May Not Survive Multi-Stage Image Build (UNVERIFIED)
- **Risk**: The capture container's `CAP_NET_RAW` is granted via `setcap cap_net_raw+ep` file capabilities on its binary (see the Capability Mechanism decision above), which requires `allowPrivilegeEscalation: true` and is incompatible with PodSecurity `restricted`. Whether the `security.capability` xattr `setcap` writes actually survives a multi-stage `COPY` into the final distroless/scratch image stage is **unverified** for this build pipeline — some `COPY`/overlay-filesystem implementations drop xattrs, and this repo has already hit COPY-time file-mode problems building game images (`fix(images): set entrypoint mode at COPY time instead of chmod`). Distroless/scratch final stages also have no `setcap` binary available to reapply the capability after copying in, so if the xattr is dropped there is no in-final-stage fallback.
- **Mitigation**: Before implementation is trusted, CI must build the actual multi-stage capture image, run a container from it as the non-root user, and confirm the capability survived — via `getcap` on the binary and/or an actual successful `AF_PACKET` raw-socket open. Do not assert in any downstream artifact (plan.md, tasks.md, PR description) that file capabilities work in this image until that CI proof exists.
- **Timeline**: Must be proven in CI in Phase 0/early Phase 1, before this mechanism is relied upon for the capture container's design. Blocker if the capability does not survive: escalate to root (rejected by D-SETCAP as a default, but would need re-litigating) or find a base image stage that can `setcap` post-copy.

---

## Requirements Needing Spec Amendment

### 1. User Story 2, Acceptance Criterion 4: No Amendment Needed — Disable Does NOT Restart the Pod — **CORRECTED**

**Status**: NO AMENDMENT WAS APPLIED, AND NONE IS NEEDED. A prior version of this research document fabricated a quote claiming spec.md was amended on 2026-08-23 to say disable "triggers a controlled pod restart." That quote does not exist anywhere in spec.md and no such amendment was ever made. It is retracted here.

**Actual current text** (spec.md line 50, US2 Acceptance Scenario 4):
> "Given a GameServer with capture enabled, When an admin disables capture, Then any active capture is stopped immediately, the capture capability is marked off, and the ephemeral container is removed on the next pod recreation—with the game container unaffected in the meantime."

**Actual current text** (spec.md line 121, FR-001, last sentence):
> "Disabling capture MUST stop any active capture and mark the capability off immediately; the ephemeral container is removed on the next pod recreation without affecting the running game container in the meantime."

**Finding**: Kubernetes 1.28's ephemeral container design **forbids removal without pod recreation** — that part of the earlier research was correct. But the spec does not resolve this by having disable *trigger* a restart. Instead it accepts that the stale ephemeral container lingers, inert, until the pod is next recreated for some independent reason (an image update, a node drain, an operator upgrade, etc.); disable itself is restart-free, and the game container is explicitly called out as unaffected "in the meantime." Implementation must not add code that proactively restarts the pod on disable — that would be stricter than, and inconsistent with, the accepted requirement.

**Justification**: Disabling capture is defined as an immediate, restart-free action (stop the capture, flip the flag off). The lingering ephemeral container is a cosmetic/resource-cleanup concern for the *next* natural pod recreation, not a scenario the disable operation itself needs to force.

### 2. User Story 2, Acceptance Criterion 2: No Amendment Needed — Already States the Ephemeral-Container Mechanism — **CORRECTED**

**Status**: NO AMENDMENT WAS APPLIED, AND NONE IS NEEDED. A prior version of this research document fabricated a claim that spec.md was amended on 2026-08-23 to add ephemeral-container detail to this scenario. spec.md already stated the ephemeral-container mechanism in its original text; nothing here needed rewriting.

**Actual current text** (spec.md line 47, US2 Acceptance Scenario 2):
> "Given a GameServer with capture disabled, When an admin enables capture, Then the capture sidecar is injected as an ephemeral container into the running pod without restarting the game container."

**Finding**: This requirement is directly achievable via ephemeral containers: the game container is not restarted, and the mechanism (ephemeral container injection into the running pod) is named explicitly in the requirement text itself, not left implicit.

**Justification**: No further clarification is required; implementers should read this scenario, plus FR-001's second sentence ("Enabling capture for a GameServer MUST inject the capture sidecar into the running pod without modifying the game container itself"), as the complete statement of the enable-path contract.

### 3. User Story 4: Retention Window Must Be Enforced in Code, Not Just Documented — largely already covered

**Current requirement** (spec.md line 71):
> "Captures are not permanent. Each capture is assigned an auto-expiration time based on a configured retention window (e.g., 24 hours). Once a capture expires, it is deleted from storage and subsequent download attempts are rejected."

**Finding**: Implementation must enforce the retention window with code (e.g., operator reconciler checks `status.completionTime + ttl < now()` and deletes expired captures). Simply documenting retention is insufficient for security. Note that spec.md's US4 already has Acceptance Scenarios 2–4 (spec.md lines 80–82) covering expired-download-rejected, expired-capture-not-listed, and running-capture-terminated-at-expiry — a prior draft of this section understated how much US4 already specifies. What remains implementation guidance (not a spec gap) is that "deleted from storage" must mean the operator reconciler actively performs the deletion (checking `status.completionTime + ttl < now()`), not that retention is merely advisory.

**Recommended clarification** (implementation note, not a required spec amendment):
> The operator's NetworkCapture reconciler MUST actively delete the CRD and its backing pcap file once `status.completionTime + ttl < now()`; passive/manual cleanup does not satisfy US4 Acceptance Scenarios 2–3.

**Justification**: Security requirement. Captures contain player data; automatic cleanup is mandatory. The existing scenarios already state the observable behavior; this note only pins down that the mechanism must be active, not documented-only.

### 4. FR-003: Filter Validation Timing Must Be Explicit

**Current requirement** (spec.md line 124, FR-003):
> "Capture filter expressions MUST be a first-class input to the capture start request. The filter is OPTIONAL; when omitted, a default filter is applied that restricts the capture to the game server's own advertised ports. Custom filters MUST restrict captured traffic by at least packet criteria (source/destination IP, source/destination port, protocol). Invalid filter expressions MUST be rejected before the capture starts."

**Finding**: Implementable as either API-tier validation (at request time) or sidecar-tier validation (when sidecar receives the start command). The spec doesn't clarify timing, leaving ambiguity about where validation happens.

**Recommended amendment**:
> "FR-003 (addendum): Validation occurs at the API tier (when the user submits a capture request); invalid filters result in HTTP 400 Bad Request, and no NetworkCapture CRD is created. Sidecar-tier validation is secondary (defense-in-depth); if API validation is bypassed (e.g., via direct CRD creation), the sidecar must also reject invalid filters."

**Justification**: Clarifies responsibility and prevents ambiguity during implementation.

### 5. SC-007: AMENDMENT ALREADY APPLIED — no action

**Status**: AMENDMENT ALREADY APPLIED, NO FURTHER ACTION NEEDED. A prior version of this research document quoted a stale, pre-amendment SC-007 ("...does not have a capture sidecar and is byte-identical to a server today...") and proposed amending it. That quote no longer matches spec.md: SC-007 was already amended to account for the pre-provisioned-emptyDir storage decision. The quote and its "Recommended amendment" below are retracted.

**Actual current text** (spec.md line 155, SC-007):
> "A GameServer with capture disabled or not opted in does not have a capture container (it may carry an empty, unused capture-storage volume — see FR-001); a `kubectl diff` between a non-capturing and capturing server shows only the sidecar addition, not any change to the game container."

**Finding**: This already states exactly the outcome the pre-provisioned-emptyDir decision requires: no capture *container* (sidecar or ephemeral) on a non-opted-in server, an allowed empty/unused capture-storage volume, and a `kubectl diff` that shows only the sidecar addition with no change to the game container. No further spec amendment is needed here. The real SC-007 does not mention a capture ServiceAccount, capture RBAC rules, or capture audit entries — those were inventions of the earlier, now-retracted "Recommended amendment" and must not be treated as spec requirements.

**Justification**: The spec already reflects the storage architecture; implementers should read spec.md line 155 as the governing text, not any older draft quoted in an earlier revision of this document.

### 6. FR-007: Cluster Max Retention Enforcement Mechanism Could Be More Explicit

**Current requirement** (spec.md line 128, FR-007):
> "Captures MUST be automatically deleted after a configured retention window (default: 24 hours). Per-server retention settings override the cluster default; a cluster-configured maximum retention window caps any per-server setting to prevent servers from extending retention beyond cluster policy. Completed captures outside the retention window MUST not be listed or downloadable. Expired captures MUST be cleaned from storage."

**Finding**: The default (24 hours) is already fixed by the spec and is not open for re-discussion (see the Retention decision above — a prior draft of this research incorrectly used 7 days here). What the spec does not fix is the cluster-wide maximum's specific value, or the precise enforcement mechanism (HTTP status code, config surface) — those remain implementation choices this document may recommend but not mandate.

**Recommended amendment (addendum, not a change to the 24-hour default)**:
> "FR-007 (addendum): The API enforces the cluster max at request time: if a user attempts to create a capture with `ttlSecondsAfterFinished` exceeding the cluster max, the request is rejected with HTTP 400 Bad Request and a descriptive error message (plain text, per the httperr decision below — no JSON envelope). The cluster max is configured via Helm value `capture.maxRetentionSeconds`, proposed default 90 days, and cannot be exceeded by any capture or GameTemplate."

**Justification**: Clarity on enforcement mechanism prevents ambiguity during implementation, without touching the already-decided 24-hour default.

### 7. User Story 5 / FR-010: Pod-Restart-Mid-Capture Behavior — Already an Explicit Acceptance Criterion

**Status**: ALREADY PRESENT, NO AMENDMENT NEEDED. A prior draft of this research document cited this behavior as living only in "spec.md line 96, Assumption 4" and recommended promoting it to an explicit acceptance criterion. That citation was wrong on two counts: line 96 is not Assumption 4 (Assumption 4, spec.md line 165, is about hard duration/size limits, unrelated to pod restart), and the pod-restart behavior is not merely an assumption — it is already User Story 5's first Acceptance Scenario.

**Actual current text** (spec.md line 96, User Story 5, Acceptance Scenario 1):
> "Given a running capture, When the GameServer pod is restarted, Then the capture is terminated, the partial capture file is cleaned up (or marked as incomplete if preserved for debugging), and the server returns to a playable state."

**Actual current text** (spec.md line 131, FR-010):
> "The capture system MUST fail gracefully when the pod is restarted, evicted, or deleted. Orphaned capture processes and partial files MUST be cleaned up. The game server MUST return to a playable state."

**Finding**: Both the acceptance scenario and the functional requirement already exist in the form this research document previously proposed adding. No further spec amendment is needed here; implementers should treat spec.md lines 96 and 131 as the governing text.

---

## Summary of Architectural Decisions

| Aspect | Decision |
|--------|----------|
| **Sidecar injection** | Ephemeral containers (K8s 1.28+); disable is restart-free — the stale ephemeral container just lingers until the next unrelated pod recreation, per spec.md FR-001 / US2 scenario 4 |
| **Capture engine** | gopacket/afpacket (AF_PACKET) + pcapgo (NgWriter) + go-pcap/filter (BPF validation) |
| **Filter validation** | API tier (HTTP 400 on invalid filter) using go-pcap/filter |
| **Capture state** | NetworkCapture CRD (namespaced, owned by GameServer) |
| **Storage** | Capture `emptyDir` (with `sizeLimit`) pre-provisioned on EVERY game pod's StatefulSet template, opted-in or not (human Decision 1) — required because ephemeral containers cannot add a volume; one-time rolling restart on upgrade; download path (agent proxy vs. sidecar endpoint) still open, constrained by the agent's single-root file browser |
| **Retention** | spec.ttlSecondsAfterFinished (K8s native); 24-hour default (spec.md FR-007, mandated — not a research choice), 90-day cluster max (research proposal) |
| **Concurrency** | CRD phase-based serialization; one Running capture per GameServer |
| **Defaults** | Helm values (immutable); optional API config table for tuning |
| **Security** | CAP_NET_RAW sidecar only; admin-only access; audit logging mandatory; non-opted-in pods carry no capture component but do carry the empty pre-provisioned volume (SC-007 — already amended in spec.md, no further action) |
| **Capability mechanism** | File capabilities (`setcap cap_net_raw+ep`) on the sidecar binary, non-root `runAsUser`; requires `allowPrivilegeEscalation: true`, incompatible with PodSecurity `restricted`; xattr survival through the multi-stage image build is UNVERIFIED (D-SETCAP, human decision) |
| **Sidecar addressing** | Existing `<gs>-agent` ClusterIP Service gets a second, numerically-targeted port (9091); dialed by its existing DNS name, already covered by the agent cert's SANs; no IP SAN, no new cert (D-ADDRESSING, human decision) |
| **Audit reason** | New `reason` column on `audit_events` via new append-only migration (007+); participates in the `005_audit_chain.sql` hash chain for rows written after it ships (human Decision 2) |
| **Error responses** | Existing plain-text `api/internal/httperr` (`http.Error`); no new JSON error envelope (human Decision 3) |
| **E2E testing** | Operator bucket; structural testing (no game traffic) for v1 |
| **CRD codegen** | operator/api/v1alpha1/networkcapture_types.go; `make generate && make manifests` |
| **API routes** | REST endpoints (`POST /servers/{name}:capture-start`, etc.); sidecar on `:9091`, reached via the `<gs>-agent` Service |

---

## Citation Index

**Kubernetes and Container Design:**
- Kubernetes 1.28+ ephemeral container design: [https://kubernetes.io/docs/concepts/workloads/pods/ephemeral-containers/]
- AF_PACKET socket design: [https://man7.org/linux/man-pages/man7/packet.7.html]

**Gameplane Codebase Citations:**
- GameServer container assembly: `/home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_controller.go:1255-1391, 1461-1561, 1596-1708`
- Agent sidecar security context: `/home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_controller.go:1609-1612, 1691-1698`
- Pod template hash annotation: `/home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_controller.go:1274-1285`
- Backup CRD pattern: `/home/valgul/project/kubernetes-game-dashboard/operator/api/v1alpha1/backup_types.go:1-145`
- Backup controller phase transitions: `/home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/backup_controller.go:173-257, 316-441`
- Retention cleanup logic: `/home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/retention.go:14-148`
- Agent TLS provisioning: `/home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/agent_certs.go:37-108` (Secret name is `<gs>-agent-tls`, not `<gs>-agent-cert`; the CA ships inside that same Secret as `ca.crt`)
- Agent DNS SANs / `<gs>-agent` Service addressing (no IP SAN; no bare `<pod>.<ns>.svc.cluster.local` form): `/home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/agent_certs.go:201-223`
- Agent volume-mount single-root constraint (why extra/capture volumes are NOT mounted on the agent without further work): `/home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_rcon.go:105-136`
- Agent image injection: `/home/valgul/project/kubernetes-game-dashboard/operator/cmd/main.go:112-129, 248-259`
- Audit middleware: `/home/valgul/project/kubernetes-game-dashboard/api/internal/audit/audit.go:579-657`
- Audit event schema and hash chain: `/home/valgul/project/kubernetes-game-dashboard/api/internal/audit/audit.go:727-736`; `/home/valgul/project/kubernetes-game-dashboard/api/internal/db/migrations/005_audit_chain.sql`
- Migrations directory (append-only, numbered sequentially; highest existing is 006 as of this writing — re-check before naming a new one): `/home/valgul/project/kubernetes-game-dashboard/api/internal/db/migrations/`
- Plain-text error responses (no JSON envelope): `/home/valgul/project/kubernetes-game-dashboard/api/internal/httperr/httperr.go`
- Files endpoint and path security: `/home/valgul/project/kubernetes-game-dashboard/agent/internal/files/files.go:47-184, 352-353`
- RBAC enforcement: `/home/valgul/project/kubernetes-game-dashboard/api/internal/rbac/rbac.go:40-70, 153-179, 237, 318-345`
- Admin-only route patterns: `/home/valgul/project/kubernetes-game-dashboard/api/internal/handlers/audit.go:22-32`
- WebSocket proxying: `/home/valgul/project/kubernetes-game-dashboard/api/internal/ws/dialer.go:162-214, 238-290`
- E2E test buckets and conventions: `/home/valgul/project/kubernetes-game-dashboard/test/e2e/buckets.sh:1-250`
- GameServer helpers: `/home/valgul/project/kubernetes-game-dashboard/test/e2e/test_helpers_e2e_test.go:42-92`
- Optional component pattern (sentinel): `/home/valgul/project/kubernetes-game-dashboard/operator/internal/controller/gameserver_sentinel.go:228-230`
- Helm values and feature gating: `/home/valgul/project/kubernetes-game-dashboard/charts/gameplane/values.yaml:58, 221-225, 357-362`
- Pod security posture: `/home/valgul/project/kubernetes-game-dashboard/docs/security.md:203-216`
- CRD codegen contract: `/home/valgul/project/kubernetes-game-dashboard/CLAUDE.md:rule 7, 32-35, 84-85, Makefile`

**Go Libraries:**
- gopacket/afpacket: [https://pkg.go.dev/github.com/google/gopacket/afpacket]
- gopacket/pcapgo: [https://pkg.go.dev/github.com/google/gopacket/pcapgo]
- packetcap/go-pcap/filter: [https://pkg.go.dev/github.com/packetcap/go-pcap/filter]

---

**Document compiled**: 2026-08-23
**Status**: Ready for Phase 1 (Planning). Six architectural decisions (capture storage pre-provisioning, audit reason column, error response format, 24-hour retention default, capability mechanism via `setcap` (D-SETCAP), sidecar addressing via the `<gs>-agent` Service (D-ADDRESSING)) are RESOLVED by explicit human decision as recorded above; "Requirements Needing Spec Amendment" items 1, 2, 5, and 7 require no further spec.md change (the cited text already exists as quoted); items 3, 4, and 6 remain open clarifications/amendments for stakeholder review. The `setcap`-survives-image-build question (Open Risk 10) remains explicitly UNVERIFIED and must be proven in CI before implementation relies on it.
**Next step**: Stakeholder review of the remaining open clarifications (items 3, 4, 6 above) and the Open Risks section (especially Risk 10); proceed to planning phase per constitution.
