# Architecture

Kestrel is split across four long-lived components and a short-lived
per-pod sidecar.

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
manage Kestrel with `kubectl apply` — the operator is authoritative.

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
2. API verifies session, RBAC → dials `wss://foo-0.foo.kestrel-games:8090/logs/tail` using mTLS
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

## Module system

A game module is a bundle of `module.yaml` (catalog metadata) +
`template.yaml` (a GameTemplate spec) + optional README/icon. Modules
can be loaded from anywhere:

- **ModuleSource** (cluster-scoped CRD) declares one store via a typed
  spec: `oci` (registry artifacts, explicit module list), `git`
  (auto-discovered module dirs at a ref), `http` (a tar.gz/zip
  archive), `local` (a directory mounted into the operator —
  `--module-local-root`, Helm `operator.localModules`), or `upload`
  (ConfigMaps labeled `kestrel.gg/module-upload=true` in the operator
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
  ones are distinguished by the `kestrel.gg/managed-by=Module` label.
- `template.yaml#spec.capabilities` declares per-game console commands
  (player moderation, backup quiesce); the operator serializes it onto
  the agent sidecar (`KESTREL_CAPABILITIES`), which interprets the
  commands at runtime — new games get full feature support without
  agent code changes.

The API's `/modules` surface (catalog merge, install, source CRUD,
bundle upload) only reads and writes these CRs/ConfigMaps; the
operator owns all reconciliation, so the dashboard and kubectl always
converge on the same outcome. Format spec: `docs/module-authoring.md`.

## Security boundaries

- **Browser → API**: HTTPS, session cookie + CSRF header, OIDC or local login.
- **API → Agent**: mTLS; client cert signed by operator-managed CA mounted into API pod.
- **Agent → K8s**: in-pod ServiceAccount, scoped to updating its owning GameServer's status.
- **Operator → K8s**: cluster-wide CRUD on Kestrel CRDs + workload primitives it manages.

See `docs/security.md` for the threat model.
