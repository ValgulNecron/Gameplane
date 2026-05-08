# Kestrel

A Kubernetes-native game server control panel. Open-source alternative to
[CubeCoders AMP](https://cubecoders.com/AMP) with a K8s backend instead of
Docker — scales from a single-node k3s homelab to multi-node production
clusters without changing the operational model.

> Status: **pre-alpha**. CRDs, operator, and dashboard are under active
> development. Not yet suitable for production workloads.

## Why

AMP is great, but it's bound to a single host running Docker. If you want:

- a spare PC running one Minecraft server, **and**
- a 5-node cluster hosting a dozen games across a club or small hosting shop,

the existing options force you to pick a side. Kestrel uses standard
Kubernetes primitives (CRDs, operators, StatefulSets, PVCs) so the same
control plane handles both.

## Feature goals

- **Lifecycle**: create, start, stop, restart, clone, delete game servers
- **Console**: live stdout/stderr over WebSocket, RCON stdin
- **Logs**: historical log viewer with filtering and download
- **Files**: browse, edit, upload, download server files (Monaco editor in-browser)
- **Players**: per-server player list with kick/ban where the game protocol supports it
- **Backups**: scheduled + on-demand snapshots to S3-compatible storage (restic), with restore back into a server
- **Modules**: versioned game templates distributed as OCI artifacts
- **Users & RBAC**: local accounts + OIDC (Keycloak, Google, GitHub)
- **Multi-cluster**: single dashboard can target multiple clusters (roadmap)

## Architecture

```
┌────────────────────────────────────────────────────────────────┐
│  Browser: React + TypeScript + Vite + shadcn/ui                │
└────────────────────────────────────────────────────────────────┘
                            │  HTTPS / WSS
┌────────────────────────────────────────────────────────────────┐
│  API (Go):  REST + WebSocket, auth, RBAC, aggregates CRD state │
└────────────────────────────────────────────────────────────────┘
                            │  K8s API
┌────────────────────────────────────────────────────────────────┐
│  Operator (Go, controller-runtime):                            │
│    reconciles GameServer / GameTemplate / Backup /             │
│    BackupSchedule / Restore CRDs into StatefulSet, Service,    │
│    PVC, and restic Jobs                                        │
└────────────────────────────────────────────────────────────────┘
                            │
┌────────────────────────────────────────────────────────────────┐
│  GameServer pod:                                               │
│    ├── game container (minecraft, valheim, ...)                │
│    └── agent sidecar (Go): RCON, file ops, log tail, metrics   │
└────────────────────────────────────────────────────────────────┘
```

### Components

| Path         | Language | Purpose                                                           |
| ------------ | -------- | ----------------------------------------------------------------- |
| `operator/`  | Go       | Reconciles CRDs into K8s objects. Built with controller-runtime.  |
| `api/`       | Go       | Front-end-facing REST + WebSocket gateway. chi, nhooyr/websocket. |
| `agent/`     | Go       | Sidecar running in each game pod. RCON, file ops, PTY console.   |
| `web/`       | TS+React | Dashboard UI. Vite, TanStack Query, xterm.js, Monaco.             |
| `modules/`   | YAML     | Per-game `GameTemplate` bundles (Minecraft, Valheim, …).          |
| `charts/`    | Helm     | `kestrel` install chart for operator + API + optional ingress.    |
| `deploy/`    | Shell    | Local dev env (kind/k3d) bootstrap scripts.                       |

### CRDs (`kestrel.gg/v1alpha1`)

- **GameTemplate** — reusable blueprint for a game (image, ports, env, volumes, defaults)
- **GameServer** — an instance of a GameTemplate with user-specific config
- **Backup** — a one-shot snapshot job
- **BackupSchedule** — a cron-like recurring backup policy
- **Restore** — a one-shot restore of a Backup snapshot into a GameServer's data volume

## Repo layout

```
.
├── operator/     # controller-runtime operator
│   ├── api/v1alpha1/     # CRD Go types
│   ├── internal/controller/
│   ├── cmd/              # operator main.go
│   └── config/{crd,rbac,samples}
├── api/          # REST + WS gateway
├── agent/        # in-pod sidecar
├── web/          # React dashboard
├── modules/      # game-module bundles (template.yaml + module.yaml)
│   ├── minecraft-java/
│   ├── valheim/
│   ├── terraria/
│   └── build.sh  # OCI bundle builder/pusher (uses oras)
├── charts/kestrel/       # Helm chart
├── deploy/kind/          # local dev cluster
├── docs/
└── design.pen    # Pencil design source (do not delete)
```

## Quickstart (local dev)

Requires: Go 1.22+, Node 20+, Docker, kind, kubectl, helm,
[oras](https://oras.land/docs/installation) (>= 1.2.0).

```sh
# spin up a local kind cluster with Kestrel preinstalled
make dev-up

# in another shell, run the web app against the in-cluster API
make web-dev

# tear down
make dev-down
```

The `make dev-up` target:

1. creates a kind cluster from `deploy/kind/cluster.yaml` and a local
   OCI registry at `localhost:5001` (reachable from cluster pods as
   `kind-registry:5000`),
2. loads locally-built operator/api/agent images,
3. pushes every directory under `modules/` (minecraft-java, valheim,
   terraria) to the local registry as an OCI module bundle,
4. installs the Helm chart from `charts/kestrel/` with a default
   `ModuleSource` pointing at the local registry — the operator
   indexes it within seconds and the modules show up in the dashboard's
   Modules page.

See [`docs/module-authoring.md`](docs/module-authoring.md) for the
bundle format and how to author additional modules.

## Contributing

Design changes go through `design.pen` (Pencil) before any UI code is
written. See `docs/contributing.md` (coming soon).

## License

[AGPL-3.0-or-later](./LICENSE). Any network-accessible deployment of a
modified version must make its source available to users.
