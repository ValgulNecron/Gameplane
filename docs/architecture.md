# Architecture

Gameplane is split across long-lived components — dashboard, API, operator,
and optional audit-syslog-bridge / telemetry-receiver satellites — plus a
short-lived per-pod agent sidecar.

```
┌───────────────────────────────────────────────────────┐
│  Browser (web/)                                       │
│    React + TanStack Router/Query                      │
│    xterm.js · Monaco · Tailwind                       │
└───────────────────────────────────────────────────────┘
                    │ HTTPS / WSS (same-origin)
┌───────────────────────────────────────────────────────┐
│  API (api/)                                           │
│    REST + WebSocket gateway                           │
│    Local + OIDC auth · RBAC · audit log               │
│    SQLite (default) or Postgres for user store        │
│    mTLS to agent                                      │
└───────────────────────────────────────────────────────┘
                    │
       ┌────────────┴────────────┐
       ▼                         ▼
┌──────────────┐         ┌─────────────────────┐
│  K8s API     │         │  In-pod agent       │
│              │         │  (sidecar in every  │
│  CRD reads   │         │   GameServer pod)   │
│  CRD writes  │         │  RCON · files       │
└──────────────┘         │  logs · console     │
       │                 │  heartbeat          │
       ▼                 └─────────────────────┘
┌────────────────────────────────────────────┐
│  Operator (operator/)                      │
│    controller-runtime reconcilers          │
│    GameServer   → StatefulSet+Service+PVC  │
│    Backup       → Job (restic)             │
│    BackupSchedule → Backups + retention    │
│    GameTemplate → inUseCount bookkeeping   │
└────────────────────────────────────────────┘
```

## Why two control planes?

The operator handles everything a cluster admin expects from a K8s
controller: declarative reconciliation, owner refs, leader election,
event emission. The API handles everything the browser expects from a
SaaS dashboard: session auth, CSRF, per-user RBAC beyond K8s RBAC,
audit trails.

Keeping them separate lets advanced users bypass the API entirely and
manage Gameplane with `kubectl apply` — the operator is authoritative.

## Data flow examples

### Start a server

1. Dashboard → `POST /servers/foo:start`
2. API → `PATCH spec.suspend=false` on GameServer via k8s API
3. Operator observes GameServer update, scales StatefulSet to 1
4. StatefulSet starts the game pod (game container + agent sidecar)
5. Agent starts heartbeating → sets `status.agent.lastHeartbeat`
6. Operator observes heartbeat → sets `status.phase=Running`
7. Dashboard SSE stream receives the status update, re-renders

> **What `Running` means.** The phase flips to `Running` once the pod is
> Ready *and* the agent heartbeat is fresh. That signals the pod and
> sidecar are healthy — **not** that the game protocol (RCON / server
> query) is responsive. `status.agent.playersOnline` stays unknown until
> the game answers a player-count query, and the Console is unavailable
> until RCON is provisioned. Games needing stricter, protocol-level
> readiness should express it through the template's readiness probe.
>
> The operator updates only the phase/conditions/endpoints it owns via a
> JSON merge patch, while the agent independently patches `status.agent`;
> the two never clobber each other.

### Tail logs

1. Dashboard opens WS `/ws/servers/foo/logs`
2. API verifies session, RBAC → dials `wss://foo-0.foo.gameplane-games:8090/logs/tail` using mTLS
3. Agent tails the game container's log file and streams each line as a text WS frame
4. API proxies frames back to the browser; xterm.js renders them

The Logs tab can also stream the game container's stdout directly via the
Kubernetes pod-log API (`/ws/servers/foo/logs/pod`, no agent mTLS needed).
This is the default source and surfaces download/config output during
startup, before the game's own log file exists.

### Back up

1. Dashboard → `POST /backups` with inline `spec`
2. API creates the Backup CRD
3. Operator creates a Job running `restic backup /data` against the PVC
4. Job completes; operator mirrors Job status into Backup.status
5. (Agent, during the Job, optionally issues an RCON `save-all` to quiesce)

A `Backup` is only usable once its `status.snapshotID` has been read out of
the restic Job's pod logs — a backup with no snapshot id cannot be restored,
so parking it at `Succeeded` would be misleading. The operator retries the
scrape, and if the id still can't be read a grace period after the Job
finished (the pod logs were rotated or garbage-collected, or the Job itself
is gone) it transitions the Backup to `Failed`.

Before going terminal it always releases the game world: a Backup with
`spec.quiesce` left the game with auto-save off waiting for the post-backup
unquiesce, and a `Failed` Backup is never reconciled again — so the unquiesce
runs first, and while the agent is unreachable the operator requeues rather
than failing.

## Module system

A game module is a bundle of `module.yaml` (catalog metadata) +
`template.yaml` (a GameTemplate spec) + optional README/icon. Modules
can be loaded from anywhere:

- **ModuleSource** (cluster-scoped CRD) declares one store via a typed
  spec: `oci` (registry artifacts, explicit module list), `git`
  (auto-discovered module dirs at a ref), `http` (a tar.gz/zip
  archive), `local` (a directory mounted into the operator —
  `--module-local-root`, Helm `operator.localModules`), or `upload`
  (ConfigMaps labeled `gameplane.local/module-upload=true` in the operator
  namespace, written by the dashboard's upload endpoint or applied by
  hand).
- The **ModuleSource controller** indexes each source through a
  `modsrc.Fetcher` (operator/internal/modsrc — one implementation per
  type over a shared fs scanner) and caches the catalog into
  `status.modules`, each entry carrying a content digest (OCI manifest
  digest, git commit, or sha256 over the module dir).
- A **Module** CR installs one entry: the controller pulls the bundle
  and materializes an owned **GameTemplate**; the digest comparison
  re-applies bundles whose content changed behind an unchanged version
  (moving git branch, re-uploaded ConfigMap). A finalizer blocks
  uninstall while GameServers reference the template.
- Templates can also be `kubectl apply`'d directly — module-managed
  ones are distinguished by the `gameplane.local/managed-by=Module` label.
- `template.yaml#spec.capabilities` declares per-game console commands
  (player moderation, backup quiesce); the operator serializes it onto
  the agent sidecar (`GAMEPLANE_CAPABILITIES`), which interprets the
  commands at runtime — new games get full feature support without
  agent code changes.

The API's `/modules` surface (catalog merge, install, source CRUD,
bundle upload) only reads and writes these CRs/ConfigMaps; the
operator owns all reconciliation, so the dashboard and kubectl always
converge on the same outcome. Format spec: `docs/module-authoring.md`.

## Multi-cluster (federation)

Gameplane scales across multiple Kubernetes clusters through a
federation model: each target cluster runs its own operator and agents,
while the API server (on the control-plane cluster) holds a pool of
Kubernetes clients keyed by cluster ID and dispatches requests via a
`?cluster=` URL parameter. The built-in "local" cluster targets the
same cluster as the API; any additional cluster is registered through
a cluster-scoped CRD.

**Cluster registration and health:**

- A `Cluster` CRD (group `gameplane.local/v1alpha1`) is the source of
  truth for additional clusters. Each `Cluster` references a
  `kubeconfig` Secret via a selector, and the API watches `Cluster`
  updates to populate its client pool live.
- The kubeconfig Secret **must** be labelled
  `gameplane.local/cluster-kubeconfig=true` in the control-plane
  namespace — the label guard prevents pointing at arbitrary Secrets
  (see "Kubeconfig Secret handling" in `docs/security.md`).
- The operator health-checks each registered `Cluster` and reconciles
  `status.phase` (Unknown/Healthy/Unhealthy), so cluster connectivity
  is visible in the dashboard.

**Deployment model:**

- The **target cluster** runs its own operator instance (deployed via
  Helm to manage GameServer, Backup, and other CRDs on that cluster).
  The same agent sidecar image is used in all clusters.
- The **control-plane cluster** hosts the API and dashboard; it may or
  may not have game pods itself (if `cluster=local`).
- A **request** made to the API with `?cluster=<name>` is dispatched to
  the named cluster's Kubernetes client. Omitting the selector targets
  the built-in "local" cluster.

**RBAC and permissions:**

Registering an additional cluster grants **no implicit RBAC** on it.
Each cluster's role bindings are independent; existing bindings
created before federation was enabled are pinned to "local". To grant
a user access to resources on a newly registered cluster, create
matching role bindings on that target cluster or add cluster-scoped
permissions through the API. See `docs/install.md#registering-an-additional-cluster`
for the registration flow.

## Security boundaries

- **Browser → API**: HTTPS, session cookie + CSRF header, OIDC or local login.
- **API → Agent**: mTLS; client cert signed by operator-managed CA mounted into API pod.
- **Agent → K8s**: in-pod ServiceAccount, scoped to updating its owning GameServer's status.
- **Operator → K8s**: cluster-wide CRUD on Gameplane CRDs + workload primitives it manages.
- **Operator/Agent → external fetches**: a shared dial-time SSRF guard
  (`netguard/`) refuses cloud-metadata and other unroutable-for-the-caller
  addresses — permissive for the operator's admin-configured ModuleSource
  fetches, strict for the agent's user-triggered mod-install downloads.
- **API → audit-syslog-bridge (optional)**: plaintext or TLS syslog forward
  for the audit trail, enabled via `api.audit.webhook.syslogBridge.enabled`.
- **API → telemetry-receiver (optional)**: the anonymous daily usage report
  (admin-toggle gated), auto-wired via `api.telemetry.receiver.enabled` or
  aimed at an external URL via `api.telemetry.endpoint`.
- **mcp-server (optional)**: a standalone, strictly read-only MCP server —
  `get`/`list`/`watch` ClusterRole only, no create/update/patch/delete
  anywhere in its RBAC or its tool set — that talks directly to the
  Kubernetes API (not through the API server), gated via `mcpServer.enabled`
  and reachable only via `kubectl exec` (stdio transport, no network port).
  See `mcp-server/README.md`.

See `docs/security.md` for the threat model.
