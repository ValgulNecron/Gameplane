# operator — Specification

**Status:** beta (v0.2.0-beta.7)  
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

### Namespaced (4)

| Kind | File | Purpose |
|------|------|---------|
| **GameServer** | `gameserver_types.go` | Instance of a game server. References a GameTemplate for defaults; Spec declares desired replica count, suspend flag, stop grace period, module customizations, backup trigger. Status tracks phase (Pending/Starting/Running/Stopping/Stopped/Suspended/Failed), pod readiness, agent heartbeat. |
| **Backup** | `backup_types.go` | One-shot backup of a GameServer's data. Spec declares gameServer ref, optional quiesce preferences, strategy (restic or volume snapshot). Status tracks phase (Pending/Running/Succeeded/Failed), snapshot ID, restic output summary. |
| **BackupSchedule** | `backupschedule_types.go` | Recurring backup schedule for a GameServer. Spec declares cron expression, retention rules (keep-daily, keep-weekly, keep-monthly, keep-yearly), optional suspend. Status reports next firing time, last fire time, retention condition. |
| **Restore** | `restore_types.go` | One-shot restore of a Backup into a GameServer (typically a fresh copy). Spec refs the source Backup and target GameServer. Status tracks phase (Pending/Suspending/Running/Resuming/Succeeded/Failed), snapshot ID. Coordinates suspend → restic restore Job → resume. |

**Verification:** CRD YAML in `config/crd/` generated from types via `make manifests`. All 8 kinds present and scopes correct (verified against `gameplane.local_*.yaml` files).

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
- **Split concerns:**
  - `gameserver_config.go`: render config files, template variable substitution.
  - `gameserver_modcreds.go`: mount mod credentials Secrets.
  - `gameserver_rcon.go`: RCON port exposure, lifecycle sequences.
  - `gameserver_restart.go`: restart action, pod deletion.
  - `gameserver_wipe.go`: data wipe sequence (delete PVC, recreate).
  - `gameserver_version.go`: track/propagate game version.
  - `gameserver_node.go`: node affinity, pod anti-affinity.
  - `gameserver_status.go`: phase computation from StatefulSet/Pod state + agent heartbeat.
  - `gameserver_stop_attach.go`: pod exec attachment for graceful stop commands.
  - `gameserver_extravolumes.go`: user-supplied additional volume mounts.
- **Status phases:** Pending, Starting, Running, Suspended, Stopping, Stopped, Failed.

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

### Helper Reconcilers & Utilities

- **agent_certs.go:** Generate per-GameServer CA-signed mTLS server cert for agent sidecar (operator CA cert/key injected via flags).
- **agent_rbac.go:** Create per-GameServer ServiceAccount + Role for the agent sidecar (used to verify token).
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

**Manager configuration:**
- CacheSyncTimeout: 5 minutes (extended from default 2m to tolerate slow apiservers on resource-constrained nodes).
- Scheme: includes Kubernetes types, gameplane.local/v1alpha1 types, and CSI VolumeSnapshot types.

### Codegen invariants

After any edit to `operator/api/v1alpha1/*_types.go`:

```sh
make generate && make manifests
```

Regenerates and commits atomically:
- `operator/api/v1alpha1/zz_generated.deepcopy.go` — struct deepcopy methods.
- `operator/config/crd/gameplane.local_*.yaml` — 8 CRD manifests.
- `operator/config/rbac/*.yaml` — ServiceAccount, Roles, RoleBindings, ClusterRoles, ClusterRoleBindings.
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
