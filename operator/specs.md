# operator — Specification

**Status:** beta (v0.2.0-beta.8)  
**Module / package:** github.com/ValgulNecron/gameplane/operator  
**Go version:** 1.25.0

## Purpose

The operator is a Kubernetes controller-runtime-based process that reconciles Gameplane Custom Resource Definitions (CRDs) into operational Kubernetes objects. It is the authoritative owner of all state transformations: a user must be able to `kubectl apply` a GameServer and achieve the same outcome as creating it via the dashboard API. The operator drives the full lifecycle of game servers, backups, restores, and module installation through a set of specialized reconcilers watching CRD status.

## Responsibilities

- **GameServer lifecycle:** reconcile GameServer CRs into StatefulSets, Services, PVCs, and pod-injected agent sidecars; manage pod affinity, RCON/console integration, file I/O, and graceful stop sequences.
- **Module installation:** pull OCI module bundles from remote or local sources, verify signatures (cosign), and materialize GameTemplate CRs; refresh source indexes periodically.
- **Backup & restore:** coordinate application-level quiesce with agent, spawn restic backup/restore Jobs, track snapshot IDs, and enforce retention policies.
- **Scheduled backups:** parse cron expressions, create Backup objects on schedule, apply retention rules (keep-daily, keep-hourly, etc.), and report schedule validity and retention status.
- **Remote cluster health:** periodically probe Kubernetes clusters reachable via kubeconfig Secrets, report connectivity and API-server health.
- **Agent orchestration:** generate per-GameServer mTLS certificates, bind RBAC to agents, expose operator-to-agent RPC for quiesce/unquiesce during backup.
- **Fleet observability:** expose Prometheus metrics summarizing GameServer and Backup phases across the entire fleet.
- **CRD codegen:** curate operator/api/v1alpha1 type definitions; `make generate && make manifests` regenerates deepcopy, RBAC, and CRD YAML whenever types change.

## Non-goals / boundaries

The operator **does not** handle:
- User authentication or authorization (RBAC is Kubernetes role-based, not application-level).
- REST API, WebSocket, or dashboard concerns (the API server, not the operator, owns the UX layer).
- Module source credential management beyond reading Secrets passed by the API (credentials are user-supplied via the API and stored as Secrets; the operator only reads them).

**House rule — "the operator is authoritative":** All business logic flows through the reconcilers. The API server writes CRs and watches status; it does **not** embed reconciliation logic. A user deleting a GameServer via `kubectl delete gameserver <name>` must trigger identical cleanup as a dashboard delete, because both routes lead to CRD deletion and the operator's cleanup finalizers. Changes that bypass the CRD (e.g., direct edits to StatefulSets) are not supported — the operator's next reconcile will overwrite them.

## Directory & package layout

```
operator/
├── cmd/main.go
│   Entry point; parses CLI flags (metrics/probe addresses, image refs, certificate paths,
│   CIDR allowlists), constructs manager and controller-runtime objects, starts reconcilers.
│
├── api/v1alpha1/
│   CRD type definitions (gameserver_types.go, gametemplate_types.go, backup_types.go,
│   backupschedule_types.go, restore_types.go, module_types.go, modulesource_types.go,
│   cluster_types.go) and generated deepcopy helpers (zz_generated.deepcopy.go).
│   Edited manually; regenerate deepcopy + YAML after any type edit.
│
├── config/
│   ├── crd/              Generated CRD YAML (8 kinds). Do not hand-edit.
│   ├── rbac/             Generated ServiceAccount, Role, RoleBinding, ClusterRole,
│   │                     ClusterRoleBinding. Do not hand-edit.
│   ├── manager/          Manager deployment, StatefulSet, and RBAC references.
│   └── samples/          Sample GameServer, GameTemplate, Backup, etc. CRs for testing.
│
├── internal/
│   ├── controller/       Reconciler implementations (8 controllers + co-located envtest).
│   │   ├── gameserver_controller.go        Reconciles GameServer → StatefulSet+Service+PVC
│   │   ├── gameserver_*.go                 Split concerns: config, modcreds, rcon, restart,
│   │   │                                   wipe, version, stop_attach, node, status, extravolumes
│   │   ├── gametemplate_controller.go      Maintains template.status.inUseCount
│   │   ├── backup_controller.go            Drives Backup → restic Job + quiesce orchestration
│   │   ├── backup_volumesnapshot.go        CSI snapshot tracking for backup strategy
│   │   ├── backupschedule_controller.go    Cron scheduler + retention trimming
│   │   ├── restore_controller.go           Drives Restore → restic Job with GameServer suspend
│   │   ├── restore_volumesnapshot.go       Snapshot recovery for restore strategy
│   │   ├── modulesource_controller.go      Indexes module sources, refreshes catalog
│   │   ├── module_controller.go            Pulls OCI bundle, verifies signature, creates GameTemplate
│   │   ├── cluster_controller.go           Health checks remote clusters via kubeconfig
│   │   ├── agent_certs.go                  Generates per-GameServer agent mTLS certs
│   │   ├── agent_rbac.go                   Creates per-GameServer ServiceAccount + Role
│   │   ├── metrics.go                      Prometheus collectors for GameServer/Backup phases
│   │   ├── retention.go                    Backup retention logic (keep-daily, keep-hourly, etc.)
│   │   ├── restic_summary.go               Parses restic JSON output from container logs
│   │   ├── semver.go                       Semantic versioning helpers
│   │   ├── helpers.go                      Utility functions shared across reconcilers
│   │   └── *_envtest_test.go               Co-located integration tests (envtest tier)
│   │
│   ├── agent/
│   │   Typed HTTP client for operator → agent mTLS calls during backup quiesce.
│   │   Builds TLS config from certificate paths; disabled when no mTLS material configured.
│   │
│   ├── modsrc/
│   │   Module source fetchers (bundle.go, dir.go, git.go, http.go, local.go, oci.go, upload.go).
│   │   Abstraction layer: each source type (OCI, git, HTTP, local directory) implements
│   │   a Fetcher interface; operator uses modsrc.ForSource to route to the right impl.
│   │
│   ├── oci/
│   │   OCI registry client wrapper + authentication helpers (auth.go, client.go, testregistry_test.go).
│   │   Thin layer over go-containerregistry for image pushing/pulling.
│   │
│   └── verify/
│       cosign signature verification for OCI module bundles (keyed + keyless).
│       Returns Verifier interface; operator refuses unsigned bundles if spec.verify declared.
```

## Custom Resource Definitions

Eight CRD kinds under `gameplane.local/v1alpha1`:

### Cluster-scoped (4)

| Kind | File | Purpose |
|------|------|---------|
| **GameTemplate** | `gametemplate_types.go` | Reusable blueprint for a game server (image, ports, probes, RCON protocol, mod loaders, etc.). Cluster-wide catalog; instances are created via GameServer references. Status tracks in-use count. |
| **Module** | `module_types.go` | User request to pull and install a module (OCI bundle) from a ModuleSource. Creates a child GameTemplate on success; delete cascades to the template via owner reference. |
| **ModuleSource** | `modulesource_types.go` | Registry of module bundles (OCI, git, HTTP, or local). Declares source location, refresh interval, optional credentials Secret, and optional cosign verification policy. Status lists available modules. |
| **Cluster** | `cluster_types.go` | Metadata for a remote Kubernetes cluster (displayName, kubeconfig Secret ref). Cluster-scoped so multiple control planes can discover each other. Status tracks health (Unknown/Healthy/Unhealthy), lastCheckTime, conditions. |

### Namespaced (5)

| Kind | File | Purpose |
|------|------|---------|
| **GameServer** | `gameserver_types.go` | Instance of a game server. References a GameTemplate for defaults; Spec declares desired replica count, suspend flag, stop grace period, module customizations, backup trigger, **optional capture configuration**. Status tracks phase (Pending/Starting/Running/Stopping/Stopped/Suspended/Failed), pod readiness, agent heartbeat, **capture sidecar readiness and active capture**. |
| **Backup** | `backup_types.go` | One-shot backup of a GameServer's data. Spec declares gameServer ref, optional quiesce preferences, strategy (restic or volume snapshot). Status tracks phase (Pending/Running/Succeeded/Failed), snapshot ID, restic output summary. |
| **BackupSchedule** | `backupschedule_types.go` | Recurring backup schedule for a GameServer. Spec declares cron expression, retention rules (keep-daily, keep-weekly, keep-monthly, keep-yearly), optional suspend. Status reports next firing time, last fire time, retention condition. |
| **Restore** | `restore_types.go` | One-shot restore of a Backup into a GameServer (typically a fresh copy). Spec refs the source Backup and target GameServer. Status tracks phase (Pending/Suspending/Running/Resuming/Succeeded/Failed), snapshot ID. Coordinates suspend → restic restore Job → resume. |
| **NetworkCapture** | `networkcapture_types.go` | Opt-in packet-capture session for a GameServer. Spec declares gameServer ref, optional pcap filter, max duration, max file size, and optional TTL. Status tracks phase (Pending/Running/Completed/Failed/Expired), start/completion times, packet/byte counts, and error messages. Owned by its parent GameServer via ownerReferences. |

**Verification:** CRD YAML in `config/crd/` generated from types via `make manifests`. All 9 kinds present and scopes correct (verified against `gameplane.local_*.yaml` files): 4 cluster-scoped (GameTemplate, Module, ModuleSource, Cluster) + 5 namespaced (GameServer, Backup, BackupSchedule, Restore, NetworkCapture).

## Reconcilers

Primary reconcilers register with the manager in `cmd/main.go` and handle CRD lifecycle:

### GameServerReconciler
- **Responsibility:** Reconcile GameServer → StatefulSet, Service, Config ConfigMap, PVC, NetworkPolicy.
- **Key functions:**
  - Pod spec assembly: inject agent sidecar (image + pull policy + env), mount agent certs + logs volume.
  - Config & credentials: render game startup config (template vars, port mappings, mod credentials).
  - RCON integration: create agent RBAC, mTLS certs; expose RCON port if template declares it.
  - Graceful stop: if template has Lifecycle.Stop sequence, orchestrate via agent before scaling down.
  - Node affinity: honor spec.nodeSelector and spec.affinity preferences.
  - Ingress NetworkPolicy: enforce per-template advertised ports, admit CIDRs from `--game-ingress-from-cidr`.
  - Load-balancer address management: translate spec.networking.addressPool / .address preferences onto the Service based on the cluster's address-manager flavor (MetalLB, Cilium, or none), report assignment status via the AddressAssignment condition.
  - **Network capture foundation** (**Phase 2 Foundational**): Pre-provision a `captures` emptyDir (1 GiB) unconditionally on every game pod's StatefulSet template (required because pod.spec.volumes is immutable on running pods; a rolling restart of all existing game pods is incurred once on upgrade). Extend the `<gs>-agent` Service with a second numeric ServicePort 9091 for the capture sidecar's control endpoint. RBAC markers grant `pods/ephemeralcontainers` access (get, list, watch, patch, update) and full CRUD on `networkcaptures` resources. The actual capture sidecar injection as an ephemeral container is **planned for Phase 2 Implementation** (T030+).
- **Split concerns:**
  - `gameserver_config.go`: render config files, template variable substitution.
  - `gameserver_modcreds.go`: mount mod credentials Secrets.
  - `gameserver_rcon.go`: RCON port exposure, lifecycle sequences.
  - `gameserver_restart.go`: restart action, pod deletion.
  - `gameserver_wipe.go`: data wipe sequence (delete PVC, recreate).
  - `gameserver_version.go`: track/propagate game version.
  - `gameserver_node.go`: node affinity, pod anti-affinity.
  - `gameserver_status.go`: phase computation from StatefulSet/Pod state + agent heartbeat, AddressAssignment condition.
  - `gameserver_stop_attach.go`: pod exec attachment for graceful stop commands.
  - `gameserver_extravolumes.go`: user-supplied additional volume mounts.
- **Capture configuration:**
  - **Spec fields (spec.capture):**
    - `Enabled bool`: Optional flag to enable/disable the capture sidecar injection on this GameServer. When false or omitted, no sidecar is injected and the server is unchanged. When true, the operator injects the capture sidecar as an ephemeral container into the running game pod, live and without restarting the game container.
    - `RetentionSeconds *int32`: Optional per-GameServer retention override, in seconds. Clamped to the cluster maximum (via `--capture-max-retention-seconds` flag) at API tier; defaults to cluster default (via `--capture-default-retention-seconds` flag) when omitted. Applied to all captures on this server unless overridden at capture-creation time.
  - **Status fields (status.capture):**
    - `Ready bool`: Whether the capture sidecar is currently running and listening on its :9091 port, able to accept new capture requests. True only when the ephemeral container has reached Running state and the sidecar has bound its listener; False before injection, during startup, if the container crashed, or if spec.capture.enabled is false.
    - `ActiveCapture *string`: Name of the NetworkCapture currently in Pending or Running phase for this GameServer, if any. Set by NetworkCaptureReconciler when transitioning a capture to Running; cleared when that capture transitions to a terminal phase (Completed, Failed, or Expired). Allows finding the in-progress capture without listing all NetworkCaptures in the namespace.
    - `LastCaptureTime *metav1.Time`: Timestamp of the most recent capture reaching a terminal phase (Completed or Failed) on this GameServer. Updated only on terminal transitions; not updated for Pending/Running/Expired transitions.
    - `SidecarRestarts int32`: Count of restarts observed for the capture ephemeral container. NOTE: Kubernetes does not restart ephemeral containers (RestartPolicy does not apply to them). This field is retained as a stable hook for future implementations and to surface any unexpected restarts via node-level mechanisms; expected to stay 0 in practice.
  - **Implementation status — Built:**
    - `captures` emptyDir (1 GiB) pre-provisioned unconditionally on every game pod's StatefulSet template (persistent across pod restarts, immutable per house rule). Volume is mounted only on the capture ephemeral container when capture is enabled; never on the agent or game container.
    - `<gs>-agent` Service extended with second numeric port "capture" on 9091.
    - RBAC markers on GameServerReconciler grant `pods/ephemeralcontainers` (get, list, watch, patch, update) and `networkcaptures` (full CRUD).
    - Sidecar injection as ephemeral container (conditional on spec.capture.enabled + cluster-wide `--capture-enabled` flag). Injection is live and restart-free.
    - Sidecar lifecycle reconciliation (NetworkCaptureReconciler manages Pending → Running → Completed/Failed state machine).
  - **Implementation status — Planned:**
    - TTL-based auto-deletion (NetworkCapture deletion when `status.completionTime + spec.ttlSecondsAfterFinished` elapses; US4).
    - Edge cases and advanced scenarios (US5).
    - Dashboard UI for starting/stopping captures and browsing history (Phase 8).
- **Status phases:** Pending, Starting, Running, Suspended, Stopping, Stopped, Failed.
- **Address pool & assignment:**
  - **Inputs:** spec.networking.addressPool (pool name) and spec.networking.address (explicit IP).
  - **Manager flavor:** Selected via operator CLI flag `--address-manager`, one of `metallb` | `cilium` | `none` (default). Validated at startup; invalid flavors fail the operator.
  - **Service translation per flavor:**
    - MetalLB: addressPool → `metallb.io/address-pool` annotation; address → `metallb.io/loadBalancerIPs` annotation.
    - Cilium: addressPool → `gameplane.local/lb-pool` label (Gameplane convention; admin mirrors it in `CiliumLoadBalancerIPPool.spec.serviceSelector`); address → `lbipam.cilium.io/ips` annotation.
    - None: records the request but mutates no Service metadata; operator flags the ignored preference on AddressAssignment.
  - **Managed metadata:** All address-manager-applied annotations and labels are tracked and pruned when the preference is unset, ensuring no stale metadata lingers.
  - **AddressAssignment condition:** Reports the outcome of a pool/address request with one of eight reason codes:
    - `Assigned`: request honored; address manager assigned an address (True status).
    - `AssignmentPending`: LoadBalancer Service exists but no address assigned yet (False).
    - `ServiceNotReady`: Service doesn't exist or isn't type LoadBalancer (False).
    - `IgnoredForExposureMode`: request ignored because Expose ≠ LoadBalancer (False; the request is meaningless for ClusterIP/NodePort/Hostport).
    - `NoAddressManagerConfigured`: request recorded but no address manager in this cluster to act on it, so default-pool address will be used — explicitly reported to avoid silent misunderstanding (False).
    - `PoolNotFound`: the address manager reported the requested pool does not exist (False). Derived best-effort from Warning events on the Service (`extractAddressFailureFromEvents`), so it is not guaranteed to fire — an address manager that emits no matching event leaves the condition on `AssignmentPending`.
    - `AllocationFailed`: the address manager reported failure to allocate an address (False) — includes exhausted pools, quota failures, or other allocation errors. Same best-effort event derivation, and the same caveat, as `PoolNotFound`.
    - `AddressInUse`: the requested explicit address is already taken (False). Two sources: a direct check (`findAddressConflict`) — this path names the conflicting server in the message and takes precedence — and, failing that, the same best-effort event derivation, which catches clashes with non-Gameplane Services or hosts on the network. `findAddressConflict` considers every other GameServer that either carries the address in `status.endpoints` ("holds" it) or merely requests it via `spec.networking.address` ("requests" it), as a single candidate set resolved by ONE ordered comparison rather than two independent checks: candidates are sorted by a strict total order that ranks an actual holder ahead of a mere requester *regardless of creation order*, falling back to `(creationTimestamp, namespace, name)` only to break ties among candidates with the same hold status — and a conflict is reported iff the minimum of that sorted set sorts strictly before the server being reconciled, in which case that minimum is named (as `namespace/name` if from a different namespace). The server's own hold status is folded into the same comparison as every candidate's, which is what keeps this from reintroducing the old livelock: a requester's own reconcile ranks the actual holder first (so the requester yields), while the holder's own reconcile — computing its own hold status fresh — ranks itself first (so it never names the requester back); only a pure holder (no explicit `spec.networking.address` of its own) is exempt from ever running this comparison in the first place (see below). This guarantees *at least one* server in any contending set is told "no conflict" (breaking a livelock two independent checks could produce: a holder and an earlier-created mere-requester permanently reporting each other) and that the reported name is deterministic regardless of List's return order — it is not *exactly* one, because a pure holder is also told "no conflict" via its own early return (see below) independently of whatever the set's minimum resolves to. "Holding" is judged independently of "requesting" — a pool-assigned server with no explicit `spec.networking.address` still counts as holding the address it was assigned, and outranks any requester for that address regardless of which was created first, which is what catches the primary real-world case: a server assigned an address from a pool racing against another server that explicitly requests that same address. Such a pure holder can never itself report a conflict — its own `spec.networking.address` is empty, so it hits `findAddressConflict`'s early return before it ever lists other servers or runs this comparison — so this precedence rule cannot produce a mutual pair (a holder that *also* requests the address in spec does run the comparison and can report, but only when it is itself outranked by a different, higher-priority holder). Two discriminators keep a `status.endpoints` match from false-positiving on an address that was never a real LoadBalancer assignment: `GameServerEndpoint.TunnelProvider` is set on every tunnel-sourced endpoint that reaches `status.endpoints` through a code path that stamps it (frp, tailscale, and both the success and validation-failure branches of playit), so only a `TunnelProvider == ""` endpoint is eligible; and `endpointsFromService` falls back to `svc.Spec.ClusterIP` as `Host` when the Service has no LoadBalancer ingress yet, so a `Host` match only counts as holding when the candidate's own `spec.networking.expose` is `LoadBalancer` — this narrows but does not fully close the false-positive, since an LB-exposed candidate still awaiting ingress assignment carries its ClusterIP as `Host`, and a requested address that happened to fall inside the Service CIDR would still read as "holding" it; `GameServerEndpoint.Pool` cannot be used to close this gap, since it is stamped only for a translated pool request, not for an explicit-address request served off a real LB ingress. A candidate with a non-nil `metadata.deletionTimestamp` is excluded from the set entirely — a terminating server can no longer legitimately hold or claim an address, so letting it win the tiebreak would permanently block a live server behind one already on its way out; a non-LoadBalancer candidate is still eligible via the requests path, since it may legitimately be requesting an address it will use once re-exposed.
    - Unset (condition absent): no pool or address requested; no condition emitted.
    - Precedence when several apply: `IgnoredForExposureMode` → `NoAddressManagerConfigured` → direct-conflict `AddressInUse` → event-derived failure → `ServiceNotReady` → `AssignmentPending` → `Assigned`.
  - **Endpoint population:** `endpointsFromService()` populates `GameServerEndpoint.pool` only when a pool request was actually translated (outcome == Assigned and Manager supports it); never claims a pool for the ClusterIP address shown while assignment is pending, for an ignored request, or for a cluster without an address manager.

### GameTemplateReconciler
- **Responsibility:** Lightweight; only maintains status.inUseCount (how many GameServers ref this template).
- **Watches:** GameServer creations/deletions to recompute count on every reconcile.

### BackupReconciler
- **Responsibility:** Drive Backup to completion: coordinate quiesce (if agent available), spawn restic Job, track snapshot ID.
- **Key functions:**
  - Quiesce orchestration: call agent Quiesce before Job, Unquiesce after completion.
  - Restic Job: mount data PVC, run restic backup to configured destination (S3, B2, rest-server, local fs).
  - Snapshot tracking: parse restic JSON output from container logs to extract snapshot ID.
  - Status: report phase (Pending/Running/Succeeded/Failed), error details.
- **Integrations:** Agent client (quiesce), Kubernetes Pod logs (restic output), VolumeSnapshot API (CSI backups).

### BackupScheduleReconciler
- **Responsibility:** Cron scheduler + retention enforcement.
- **Key functions:**
  - Parse spec.Schedule cron expression; report ScheduleValid condition.
  - Compute next firing time; create Backup CR when due.
  - Retention trimming: list Backups, apply keep-hourly/keep-daily/keep-weekly/keep-monthly/keep-yearly rules, delete excess.
  - Report RetentionTrimmed condition (success) or retention failure (TrimFailed).

### RestoreReconciler
- **Responsibility:** Drive Restore to completion: suspend GameServer, run restic restore Job, resume.
- **Key functions:**
  - Pin snapshot ID at observation (immutable during restore).
  - Suspend: set spec.suspend=true on target GameServer, wait for pods to scale to 0.
  - Restore Job: mount snapshot (from VolumeSnapshot or restic), run restic restore.
  - Resume: clear suspend flag, wait for GameServer to reach Running phase.
  - Status: report phase (Pending/Suspending/Running/Resuming/Succeeded/Failed).

### ModuleSourceReconciler
- **Responsibility:** Index module sources and surface available modules into status.
- **Key functions:**
  - Refresh on interval (spec.refreshInterval, default 1 hour, minimum 1 minute).
  - Fetch module metadata (module.yaml) from source (OCI, git, HTTP, local dir).
  - Report status.modules[] with name, version, digest, description.
  - Handle credentials: read optional secret referenced in spec.auth, pass to fetcher.
  - Report IndexFailed condition if fetch fails.

### ModuleReconciler
- **Responsibility:** Materialize Module → GameTemplate.
- **Key functions:**
  - Resolve module name/version via ModuleSource status.modules catalog.
  - Fetch OCI bundle (oras pull).
  - Verify cosign signature if ModuleSource.spec.verify declared.
  - Extract module.yaml + template.yaml from bundle.
  - Create GameTemplate CR with owner reference to Module (delete Module → delete template).
  - Validate operator version against bundle's gameplaneMinVersion.
  - Report InstallFailed condition with root cause (signature mismatch, version too old, fetch failed, etc.).

### ClusterStatusReconciler
- **Responsibility:** Periodic health checks on remote clusters.
- **Key functions:**
  - Guard: reserve "local" cluster name (returns Unhealthy + "NameReserved").
  - Read kubeconfig from Secret (spec.kubeconfigSecret).
  - Discover cluster version, ping API server.
  - Set status.phase (Unknown/Healthy/Unhealthy), status.message, status.lastCheckTime.
  - Report Healthy condition.
  - Requeue on interval (2 minutes).

### NetworkCaptureReconciler
- **Status:** Implemented (Phase 2 Foundational + Phase 2 Implementation); TTL-based auto-deletion planned (US4), dashboard UI planned (Phase 8).
- **Responsibility:** Manage NetworkCapture CRD lifecycle — transition Pending → Running → Completed/Failed, inject capture sidecar, call sidecar control endpoint, monitor progress.
- **Key dependencies:**
  - `SidecarCaptureClient interface` — abstracted mTLS client to sidecar's `:9091` HTTP endpoints; injected for testability; production impl in `operator/internal/agent/sidecar_capture.go`
- **Reconcile state machine:**
  - **Terminal phases** (Completed, Failed, Expired): no further reconciliation, immediate return
  - **Pending → Running:** Verify target GameServer exists and has `spec.capture.enabled = true`; check for concurrent Running captures on same server (reject with phase=Failed, message="another capture is already running on this gameserver"); inject capture sidecar ephemeral container if not already present; call `SidecarClient.StartCapture()` with spec filter/maxDuration/maxSize; transition to Running; requeue every 5s to poll sidecar
  - **Running:** Poll `SidecarClient.GetCaptureStatus()` every 5s; update `status.packetsWritten`, `status.bytesWritten`, `status.message`; once sidecar reports phase=Completed or phase=Failed, transition NetworkCapture phase and record `status.completionTime`
  - **Failure detection:** If GetCaptureStatus returns error (sidecar unreachable, pod deleted mid-capture), fail capture with descriptive message; if status.phase=Failed from sidecar, transition to Failed immediately
  - **User-requested stop:** When API's StopNetworkCapture sets phase=Completed with message="stopped by user request", tell the sidecar to stop (once per user-stop request via `SidecarStoppedCondition` guard)
- **Ephemeral container injection:** Inject into game pod's spec.ephemeralContainers subresource with:
  - **Name:** "capture"
  - **Image:** operator CLI flag `--capture-sidecar-image` (default `ghcr.io/valgulnecron/gameplane/capture-sidecar:dev`)
  - **TargetContainerName:** "game" (shares pid/network/ipc namespaces with the game container for packet capture)
  - **VolumeMounts:**
    - `captures` (emptyDir pre-provisioned by gameserver_controller.go) mounted at `/tmp/captures` (read-write)
    - `agent-tls` (Secret holding agent mTLS cert+key+CA) mounted at `/etc/tls` (read-only); certs passed via env vars below
  - **Environment:**
    - `TLS_CERT_FILE=/etc/tls/tls.crt` — client cert for mTLS to agent
    - `TLS_KEY_FILE=/etc/tls/tls.key` — private key for mTLS  
    - `TLS_CA_FILE=/etc/tls/ca.crt` — CA bundle for verifying agent server cert
  - **SecurityContext:**
    - `RunAsNonRoot: true` — container runs as unprivileged user (no root)
    - `AllowPrivilegeEscalation: true` — **CRITICAL:** Must be true for file capabilities (CAP_NET_RAW/CAP_NET_ADMIN via setcap on the capture binary) to work. Kernel only honors setcap'd capabilities on exec when no_new_privs is off; combined with Capabilities.Drop: ["ALL"] below, the container starts with zero capabilities but the setcap'd binary gains them at exec time
    - `ReadOnlyRootFilesystem: true` — mounts /tmp/captures and /etc/tls as the only writable paths
    - `Capabilities: Drop: ["ALL"]` — no ambient capabilities; sidecar binary gains CAP_NET_RAW via file capabilities at exec, never through container capabilities
  - **No imagePullPolicy set** — inherits from pod spec (usually IfNotPresent or Always per deployment)
- **Concurrency enforcement:** List all NetworkCaptures in same namespace; reject Pending→Running transition if any other NetworkCapture with same `spec.serverRef.name` and `status.phase=Running` exists; prevents multiple simultaneous captures per server; API tier also enforces this before creating the NetworkCapture
- **mTLS client reuse:** `operator/internal/agent/sidecar_capture.go`'s `CaptureClient` reuses the agent's existing mTLS http.Client (same CA cert/client cert/private key material) and addresses the sidecar via Service DNS: `https://<gs>-agent.<ns>.svc.cluster.local:9091`
- **Not implemented (planned for later phases):**
  - TTL-based auto-deletion (NetworkCapture deletion when `status.completionTime + spec.ttlSecondsAfterFinished` elapses) — US4
  - Edge cases (pod restart during capture, network partition recovery) — US5
  - Dashboard UI for starting/stopping captures and browsing history — Phase 8

### Sidecar client (operator/internal/agent/sidecar_capture.go)

- **`SidecarCaptureClient interface`** — injected into NetworkCaptureReconciler; abstracts mTLS communication with capture sidecar's `:9091` HTTP control endpoint (testable; tests inject a stub)
  - `StartCapture(ctx, namespace, serverName, captureID string, filter *string, maxDurationSeconds, maxSizeBytes int64) error` — POST `/captures/{id}:start` on sidecar
  - `StopCapture(ctx, namespace, serverName, captureID string) error` — POST `/captures/{id}:stop` on sidecar
  - `GetCaptureStatus(ctx, namespace, serverName, captureID string) (phase string, packetsWritten, bytesWritten int64, message string, err error)` — GET `/captures/{id}/status` on sidecar; polls current state during Running phase
- **`CaptureClient struct`** — production implementation of SidecarCaptureClient
  - `NewCaptureClient(agent *Client) *CaptureClient` — factory; reuses agent's existing mTLS http.Client (same CA cert/client cert/private key material)
  - `Disabled bool` flag — set to true if no mTLS material provided; all methods return nil on Disabled=true (graceful degradation)
  - URLs built from `https://<serverName>-agent.<namespace>.svc.cluster.local:9091/captures/<captureID>:start|:stop` (service DNS, cluster-internal mTLS endpoint)
  - All errors wrapped with `%w` per CLAUDE.md rule 6

### Helper Reconcilers & Utilities

- **agent_certs.go:** Generate per-GameServer CA-signed mTLS server cert for agent sidecar (operator CA cert/key injected via flags).
- **agent_rbac.go:** Create per-GameServer ServiceAccount + Role for the agent sidecar (used to verify token).
- **internal/kube/capture.go:** Kubernetes client helper functions for NetworkCapture CRD operations (API-tier use):
  - `CreateNetworkCapture(ctx, ns, captureID, serverName string, filter *string, maxDuration *metav1.Duration, maxSize *resource.Quantity, ttlSecondsAfterFinished *int32) (*NetworkCapture, error)` — creates NetworkCapture CR with Pending phase; sets ownerRef for cascade delete
  - `GetNetworkCapture(ctx, ns, name string) (*NetworkCapture, error)` — fetches by namespace/name; returns nil if not found
  - `ListNetworkCaptures(ctx, ns, serverName string) ([]NetworkCapture, error)` — lists all captures in namespace; filters by spec.serverRef.name on client side
  - `DeleteNetworkCapture(ctx, ns, name string) error` — deletes NetworkCapture by namespace/name
- **metrics.go:** Prometheus collectors — GameServerCollector (count by phase), BackupCollector (count by phase).
- **retention.go:** Backup retention logic — parse keep-* rules, identify excess backups for deletion.
- **restic_summary.go:** Parse restic container logs, extract final JSON summary (snapshot ID, duration, size).
- **semver.go:** Semantic versioning utilities (compare version strings against gameplaneMinVersion).
- **helpers.go:** Shared utilities (updateCondition, upsertCondition, phase transition helpers).

## External interface / contracts

### Entry point: `cmd/main.go`

**CLI flags:**

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--metrics-bind-address` | string | `:8080` | Prometheus metrics endpoint address. |
| `--health-probe-bind-address` | string | `:8081` | Liveness/readiness probes address. |
| `--leader-elect` | bool | `false` | Enable leader election (multi-replica deployments). |
| `--agent-image` | string | `ghcr.io/valgulnecron/gameplane/agent:dev` | Container image for injected agent sidecar. |
| `--agent-image-pull-policy` | string | `` | ImagePullPolicy override (Always/IfNotPresent/Never); empty leaves unset. |
| `--config-init-image` | string | `controller.DefaultConfigInitImage` | Init container for rendering config files onto data volume. |
| `--restic-image` | string | `controller.DefaultResticImage` | restic backup/restore Job image. |
| `--agent-log-level` | string | `` | Log level (debug/info/warn/error) injected into agent as GAMEPLANE_LOG_LEVEL; empty skips. |
| `--module-namespace` | string | `gameplane-system` | Namespace where ModuleSource credential Secrets live. |
| `--module-local-root` | string | `` | Base directory for local-type ModuleSources; empty disables local sources. |
| `--agent-ca-bundle` | string | `` | PEM-encoded CA cert signing agent server certs (operator → agent mTLS). |
| `--agent-client-cert` | string | `` | PEM-encoded client cert for operator → agent calls. |
| `--agent-client-key` | string | `` | PEM-encoded private key for agent client cert. |
| `--agent-ca-secret-name` | string | `gameplane-agent-ca` | Name of Secret holding agent CA cert+key for signing per-GameServer certs. |
| `--agent-ca-secret-namespace` | string | `gameplane-system` | Namespace of agent CA Secret. |
| `--control-plane-namespace` | string | `gameplane-system` or `POD_NAMESPACE` env | Namespace where operator runs and where cluster kubeconfig Secrets live. |
| `--game-ingress-policy` | bool | `true` | Reconcile per-GameServer ingress NetworkPolicy. |
| `--game-ingress-from-cidr` | strings | `0.0.0.0/0` | Source CIDR(s) admitted to game ports; repeatable; canonical form enforced. |
| `--address-manager` | string | `none` | Load-balancer address-manager flavor (metallb, cilium, or none). Validated at startup; controls how spec.networking.addressPool / .address preferences are translated onto the Service. |
| `--capture-enabled` | bool | `false` | Enable the network capture feature cluster-wide. When false, capture capability is disabled and cannot be enabled per-GameServer. |
| `--capture-default-retention-seconds` | int64 | `86400` | Default retention period (seconds) for completed captures; applied when spec.capture.retentionSeconds is not set. 24-hour default. |
| `--capture-max-retention-seconds` | int64 | `604800` | Maximum retention period (seconds) for captures; clamps any higher retention request. 7-day default, a storage-limitation-informed constraint. |
| `--capture-sidecar-image` | string | `ghcr.io/valgulnecron/gameplane/capture-sidecar:dev` | Container image for the network capture sidecar injected when capture is enabled. |

**Manager configuration:**
- CacheSyncTimeout: 5 minutes (extended from default 2m to tolerate slow apiservers on resource-constrained nodes).
- Scheme: includes Kubernetes types, gameplane.local/v1alpha1 types, and CSI VolumeSnapshot types.

### Codegen invariants

After any edit to `operator/api/v1alpha1/*_types.go`:

```sh
make generate && make manifests
```

Regenerates and commits atomically:
- `operator/api/v1alpha1/zz_generated.deepcopy.go` — struct deepcopy methods (includes CaptureConfiguration, CaptureStatus, NetworkCapture, NetworkCaptureList, NetworkCaptureSpec, NetworkCaptureStatus deepcopy functions).
- `operator/config/crd/gameplane.local_*.yaml` — 9 CRD manifests (includes gameplane.local_networkcaptures.yaml; gameplane.local_gameservers.yaml extended with capture spec/status schemas).
- `operator/config/rbac/*.yaml` — ServiceAccount, Roles, RoleBindings, ClusterRoles, ClusterRoleBindings (includes pods/ephemeralcontainers and networkcaptures CRUD permissions).
- `charts/gameplane/crds/*.yaml` — copy of CRDs for Helm integration (Helm `crds/` directory + pre-upgrade hook for `kubectl apply --server-side`).

Forgetting codegen leaves the YAML out of sync with types — CI's `make manifests` verify gate will catch it, but envtest runs will fail mysteriously first.

## Key invariants

1. **Operator is authoritative.** All business logic lives in reconcilers; API is a UX layer that writes CRs. Users can `kubectl apply` and get the same outcome as the dashboard.

2. **No CEL budget overruns.** CRD validation rules (XValidation) in unbounded maps/arrays must include maxProperties/maxItems + maxLength caps, or the apiserver rejects the CRD at install time and envtest panics.

3. **Codegen is mandatory after CRD type edits.** Generated deepcopy + YAML must ship in the same commit as type changes.

4. **CRDs are owned by the control plane, not Helm.** Helm's `crds/` is applied only on first install; updates come from a pre-upgrade hook running `kubectl apply --server-side --server-side-apply-manager=gameplane` on every `helm upgrade`. CRDs are never owned or deleted by Helm.

5. **Agent mTLS is optional but recommended.** Operator boots without `--agent-ca-bundle`/`--agent-client-cert`/`--agent-client-key` (client.Disabled=true); Agent methods silently no-op. Production installs should supply all three.

6. **Module sources are immutable once pulled.** Digest pinning (spec.digest) defeats tag moves; version pinning (spec.version) tracks a specific semver. Floating (unset version/digest) tracks latest and re-pulls on ModuleSource refresh.

7. **Backup quiesce is best-effort.** If agent unavailable, backup proceeds raw (no pause). If quiesce unsupported (agent returns ErrUnsupported), backup continues degraded (success-with-note).

8. **Remote cluster health checks are non-blocking.** A Cluster with health Unhealthy does not prevent GameServer creation on the local cluster; it surfaces the issue so operators can intervene.

9. **Capture volume is immutable and pre-provisioned.** Every game pod carries a `captures` emptyDir (1 GiB) unconditionally, because Kubernetes pod.spec.volumes is immutable on running pods. This incurs a rolling restart of all existing game pods once on upgrade, regardless of whether capture is ever used. The volume is mounted only on the capture sidecar ephemeral container (injected conditionally based on spec.capture.enabled and cluster-wide `--capture-enabled`), never on the agent or game container.

10. **Capture sidecar uses ephemeral containers, not init/sidecar containers.** Ephemeral containers are added live without restarting the game pod or agent. They have no imagePullPolicy, volumeMounts, or named containerPorts; the capture sidecar's port (9091) is exposed via the numeric TargetPort on the <gs>-agent Service. Disabling capture (spec.capture.enabled: false) does not remove the ephemeral container from running pods (Kubernetes API does not support removal); it stops accepting new captures.

11. **Capture TTL-based expiry is planned (Phase 2 Implementation).** The NetworkCaptureReconciler currently reconciles Pending → Running → Completed/Failed states. Auto-deletion based on `spec.ttlSecondsAfterFinished` (delete when `status.completionTime + ttl` elapses) is not yet implemented and is deferred to Phase 2 Implementation (T067). For Phase 2 Foundational, the API tier validates and clamps TTL at CRD creation time; completed captures persist until manually deleted or until Phase 2 Implementation adds TTL garbage collection.

## Dependencies

**Direct (from go.mod):**

| Module | Version | Purpose |
|--------|---------|---------|
| k8s.io/api | v0.35.0 | Kubernetes core types (Pod, StatefulSet, Service, Job, etc.). |
| k8s.io/apimachinery | v0.35.0 | Kubernetes API machinery (metav1, runtime.Scheme, etc.). |
| k8s.io/client-go | v0.35.0 | Kubernetes client (for exec, logs, discovery). |
| sigs.k8s.io/controller-runtime | v0.19.0 | Reconciler framework (Manager, Builder, Reconciler interface). |
| github.com/ValgulNecron/gameplane/netguard | local | SSRF dial guard (permissive policy for module fetches from private registries). |
| github.com/go-git/go-git/v5 | latest | Git operations (clone, fetch) for ModuleSources. |
| github.com/go-git/go-billy/v5 | latest | VCS filesystem abstraction for go-git. |
| github.com/google/go-containerregistry | v0.20.7 | OCI image operations (push, pull, digest). |
| github.com/kubernetes-csi/external-snapshotter/client/v8 | v8.0.0 | VolumeSnapshot API types. |
| github.com/opencontainers/go-digest | v1.0.0 | OCI digest parsing. |
| github.com/opencontainers/image-spec | v1.1.1 | OCI image spec types. |
| github.com/prometheus/client_golang | v1.23.2 | Prometheus metrics registration & exposition. |
| github.com/robfig/cron/v3 | v3.0.1 | Cron expression parsing (BackupSchedule). |
| github.com/sigstore/cosign/v2 | v2.6.3 | cosign signature verification (module bundles). |
| github.com/sigstore/sigstore | v1.10.8 | sigstore primitives (Fulcio roots, certificate chains). |
| golang.org/x/crypto | v0.50.0 | Cryptographic primitives (TLS, X.509). |
| golang.org/x/mod | v0.35.0 | Semantic versioning (semver package). |
| oras.land/oras-go/v2 | v2.6.0 | OCI artifact pull (module bundles). |
| sigs.k8s.io/yaml | v1.6.0 | YAML marshaling (CRD manifests). |

**Indirect:** Transitively pulled by the above (go-logr, crypto libraries, etc.).

## Data & persistence

**State location:** Entirely in CRD status subresources and Kubernetes objects created by the operator.

**No external database.** All persistent state lives in:
- CRD status fields (gameserver.status.phase, backup.status.snapshotID, etc.)
- Created child objects:
  - StatefulSet, Service, PVC (GameServer)
  - ConfigMap (game startup config)
  - Secret (game credentials, RCON password, agent certs)
  - Job (backup/restore restic Jobs)
  - NetworkPolicy (per-GameServer ingress rules)
  - VolumeSnapshot (CSI backups)
  - ServiceAccount, Role, RoleBinding (agent RBAC)

**Backup data:** Stored outside the cluster (S3, B2, restic rest-server, local filesystem). Operator creates the restic Job; destination and credentials are supplied by GameTemplate.backup.* spec.

## Security considerations

1. **cosign signature verification:** ModuleSource.spec.verify declares keyed (public key Secret) or keyless (Rekor + transparency log) verification. Operator refuses to install bundles with invalid/missing signatures if verify is declared.

2. **SSRF dial guard (netguard):** ModuleSource fetch (git clone, HTTP download) uses netguard's permissive IsAllowed policy — allows self-hosted registries on private addresses (10.0.0.0/8, etc.), but blocks obvious metadata-service endpoints (169.254.169.254). Agent module install (`capabilities.mods.install`) uses strict IsPublic policy, rejecting private IPs.

3. **Agent mTLS:** Operator → agent calls are over HTTPS with client+server certs. Server certs are per-GameServer, signed by an operator-held CA. Operator's CA cert, client cert, and private key injected via CLI flags (never in YAML).

4. **Agent RBAC:** Each GameServer gets a unique ServiceAccount + Role (verb:exec on that Pod only). Agent token is bound to that SA and verified by the operator before accepting quiesce/unquiesce calls.

5. **Network policies:** Per-GameServer ingress NetworkPolicy admits only the advertised game ports from declared CIDR(s) (default 0.0.0.0/0, customizable via `--game-ingress-from-cidr`).

6. **Finalizers:** Controllers use ownership and finalizers to ensure cleanup (e.g., Module deletion cascades to GameTemplate; Backup deletion removes associated Jobs).

## Testing & coverage

**Envtest tier (controller-runtime envtest, real Kubernetes API):** Co-located `*_envtest_test.go` files in `operator/internal/controller/`. Tests create CRs, advance reconcilers, assert status fields and child objects. Covers controller logic without kind-cluster overhead.

**Unit tests:** Smaller scope, testing individual functions (e.g., cron parsing, retention logic, restic log parsing).

**Coverage gate:** `operator/.testcoverage.yml` enforces **72% total coverage** (unit + envtest merged). Excludes:
- `cmd/` — main.go + flag/signal wiring.
- `api/v1alpha1/` — mostly generated deepcopy.
- `hack/` — codegen helpers not shipped.

CI run: `make cover` generates merged profile (unit + envtest), `make cover-ratchet` shows per-module headroom.

## References

- **`docs/architecture.md`** — full system architecture, data flow, security boundaries.
- **`docs/security.md`** — auth, RBAC, threat model, pod security.
- **`docs/module-authoring.md`** — OCI bundle format, module.yaml schema, template.yaml spec.
- **`CLAUDE.md`** — rules 7 (codegen), 9 (K8s-native), 10 (operator is authoritative).
- **`Makefile`** — canonical source of build, test, lint commands.
- **`.golangci.yml`** — linter rule set (no nolint directives without cause; fix the code).
- **`.editorconfig`** — indentation: tabs in Go, 2 spaces in YAML; LF line endings.
- **`go.mod` / `go.sum`** — dependency lock files.
- **CRD YAML:** `operator/config/crd/gameplane.local_*.yaml` (generated from types).
- **RBAC YAML:** `operator/config/rbac/*.yaml` (generated, defines operator ServiceAccount + permissions).
- **Helm integration:** `charts/gameplane/crds/` + `charts/gameplane/templates/crd-apply-hook.yaml` (pre-upgrade CRD sync).
