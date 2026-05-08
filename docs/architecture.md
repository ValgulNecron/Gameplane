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

### Tail logs

1. Dashboard opens WS `/ws/servers/foo/logs`
2. API verifies session, RBAC → dials `wss://foo-0.foo.kestrel-games:8090/logs/tail` using mTLS
3. Agent tails the game container's log file and streams each line as a text WS frame
4. API proxies frames back to the browser; xterm.js renders them

### Back up

1. Dashboard → `POST /backups` with inline `spec`
2. API creates the Backup CRD
3. Operator creates a Job running `restic backup /data` against the PVC
4. Job completes; operator mirrors Job status into Backup.status
5. (Agent, during the Job, optionally issues an RCON `save-all` to quiesce)

## Module system

A game module is a directory under `modules/` containing a
`GameTemplate` YAML and (optionally) a README + samples. Templates are
distributed as plain CRDs for v1 — users apply them with `kubectl
apply`. OCI artifact distribution is a v1.1 goal.

## Security boundaries

- **Browser → API**: HTTPS, session cookie + CSRF header, OIDC or local login.
- **API → Agent**: mTLS; client cert signed by operator-managed CA mounted into API pod.
- **Agent → K8s**: in-pod ServiceAccount, scoped to updating its owning GameServer's status.
- **Operator → K8s**: cluster-wide CRUD on Kestrel CRDs + workload primitives it manages.

See `docs/security.md` for the threat model.
