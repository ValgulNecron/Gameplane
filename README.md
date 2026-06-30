# Gameplane

A Kubernetes-native game server control panel. Open-source alternative to
[CubeCoders AMP](https://cubecoders.com/AMP) with a K8s backend instead of
Docker — scales from a single-node k3s homelab to multi-node production
clusters without changing the operational model.

> Status: **beta** (`v0.2.0-beta.3`). The operator, API, agent, and dashboard
> are feature-complete for the v1 scope and stabilized for external testing.
> See [Beta status & known limitations](#beta-status--known-limitations) before
> running it for anything you can't afford to lose.

## Beta status & known limitations

Gameplane is in **beta**: the core workflows — deploy a game server, console,
files, backups/restore, modules, RBAC — work end to end and are covered by
unit, integration (envtest), and kind-based e2e suites. Before you rely on it,
know that the following are **deferred past the first beta**:

- **Per-GameServer (owner-based) RBAC** — authorization is namespace-scoped
  today; server ownership is informational only.
- **Multi-cluster** — one target cluster per dashboard.
- **Audit-log push sink** — audit events are stored in the database, pruned on a
  configurable retention window (`api.audit.retentionDays`), exportable in full
  via `GET /admin/audit/export` (CSV/JSON), and mirrored to stdout as structured
  JSON for a cluster log aggregator (`api.audit.stdout`); a direct *push* sink
  (S3/syslog) is not built in yet.

CI runs the full suite (unit, envtest, and kind e2e) on every PR. The kind
e2e jobs can occasionally flake under resource pressure on the self-hosted
runner; re-running the job clears transient infrastructure failures.

## Why

AMP is great, but it's bound to a single host running Docker. If you want:

- a spare PC running one Minecraft server, **and**
- a 5-node cluster hosting a dozen games across a club or small hosting shop,

the existing options force you to pick a side. Gameplane uses standard
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
| `api/`       | Go       | Front-end-facing REST + WebSocket gateway. chi, coder/websocket. |
| `agent/`     | Go       | Sidecar running in each game pod. RCON, file ops, PTY console.   |
| `web/`       | TS+React | Dashboard UI. Vite, TanStack Query, xterm.js, Monaco.             |
| `modules/`   | YAML     | Per-game `GameTemplate` bundles (Minecraft, Valheim, …).          |
| `charts/`    | Helm     | `gameplane` install chart for operator + API + optional ingress.    |
| `deploy/`    | Shell    | Local dev env (kind/k3d) bootstrap scripts.                       |

### CRDs (`gameplane.local/v1alpha1`)

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
├── modules/      # git submodule → gameplane-module repo (OCI bundles)
│   ├── minecraft-java/
│   ├── valheim/
│   ├── terraria/
│   └── build.sh  # OCI bundle builder/pusher (uses oras)
├── charts/gameplane/       # Helm chart
├── deploy/kind/          # local dev cluster
├── docs/
└── design.pen    # Pencil design source (do not delete)
```

## Install on a cluster

The Helm chart and component images are published to the GitHub Container
Registry as OCI artifacts — no `helm repo add` required:

```sh
helm upgrade --install gameplane oci://ghcr.io/valgulnecron/charts/gameplane \
  --version <version> \
  --namespace gameplane-system --create-namespace \
  --set ingress.host=gameplane.your-domain.test
```

The chart pins matching `ghcr.io/valgulnecron/gameplane/{operator,api,agent}`
images by `appVersion`. To track the rolling beta instead of a tagged release,
add `--set image.tag=edge`. Then seed an admin user and log in — see
[`docs/install.md`](docs/install.md) for the full flow, OIDC, Postgres, and
values reference.

## Quickstart (local dev)

Requires: Go 1.22+, Node 20+, Docker, kind, kubectl, helm,
[oras](https://oras.land/docs/installation) (>= 1.2.0).

The game modules live in the separate `gameplane-module` repo, wired in here
as the `modules/` submodule — clone with submodules (or initialize them after):

```sh
git clone --recurse-submodules <repo-url>
# already cloned? populate the submodule:
git submodule update --init
```

```sh
# spin up a local kind cluster with Gameplane preinstalled
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
4. installs the Helm chart from `charts/gameplane/` with a default
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
