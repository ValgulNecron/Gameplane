# Gameplane

A Kubernetes-native game server control panel. Open-source alternative to
[CubeCoders AMP](https://cubecoders.com/AMP) with a K8s backend instead of
Docker — scales from a single-node k3s homelab to multi-node production
clusters without changing the operational model.

> Status: **beta** (`v0.2.0-beta.8`). The operator, API, agent, and dashboard
> are feature-complete for the v1 scope and stabilized for external testing.
> See [Beta status & known limitations](#beta-status--known-limitations) before
> running it for anything you can't afford to lose.

**Website:** <https://valgulnecron.github.io/gameplane-website/> — features,
docs, and comparisons. Source lives in
[`gameplane-website`](https://github.com/ValgulNecron/gameplane-website),
mounted here as the `website/` submodule.

## Screenshots

| | |
|---|---|
| ![Dashboard showing fleet health: running/stopped/failed server counts, cluster CPU/memory/storage usage, node status, and recent activity](docs/img/dashboard.jpg) | ![Servers list with live status, CPU, memory, and node placement for every game server in the cluster](docs/img/servers-list.jpg) |
| Dashboard — fleet health at a glance | Servers — every game server, one list |
| ![Server detail Overview tab showing CPU, memory, and disk usage plus quick actions and connection info](docs/img/server-overview.jpg) | ![Mods tab registry browser showing a grid of Thunderstore mods for Valheim with download counts](docs/img/mods-registry-browse.jpg) |
| Server detail — Overview | Mods — browsing a registry (Thunderstore) |
| ![Live streaming console output for a Terraria server, showing world-save progress](docs/img/server-console.jpg) | ![Admin Settings Mod registries screen showing CurseForge and Steam Workshop configured, Nexus Mods not configured](docs/img/admin-mod-registries.jpg) |
| Console — live output over WebSocket | Admin Settings — Mod registries |

## Beta status & known limitations

Gameplane is in **beta**: the core workflows — deploy a game server, console,
files, backups/restore, modules, RBAC, relay tunnels, multi-cluster — work end to end
and are covered by unit, integration (envtest), helm upgrade, and kind-based e2e suites.

**Multi-cluster is wired end to end**: register additional clusters from the
**Cluster** page, switch between them from the cluster selector in the
top bar, and RBAC, audit, and e2e coverage all thread the cluster dimension.
One caveat — WebSocket streams (console, logs) stay scoped to the
locally-configured cluster; that's a documented follow-up.

**Idle auto-sleep & Wake-on-connect** is opt-in per server. A woken server still needs its normal
boot time before it accepts connections. Players can wake a sleeping server by
joining (wake-on-connect, opt-in via `spec.idle.wakeOnConnect`, default false),
or via a cron wake window, or by pressing **Wake** on the dashboard. Games whose
agent reports no player count (no RCON or query protocol) never sleep at all; the
server says so on its Overview rather than leaving you guessing. Wake-on-connect
has honest limitations: only Minecraft and Terraria get real protocol parsing; the
other 14 shipped games use a generic heuristic and have no connection to hold, so
they wake-and-drop. Hostport mode also has asymmetric behavior — see
[`docs/roadmap.md`](docs/roadmap.md#wake-on-connect-for-idle-auto-sleep).

**Relay Tunnels** (`frp`, `Tailscale`, `playit`) run as a supervised pod (`tunnel/`).
Credentials are stored in a Kubernetes Secret. For `playit`, port-forward mappings are managed
against the playit.gg account tied to the secret key rather than local config files.

Before you rely on it, know that:

- A handful of production-readiness items are still open: a human-facing documented
  backup/restore drill runbook, resource-limit guidance sized from real workloads, and
  full Postgres driver portability. Automated upgrade testing across releases is fully implemented
  and runs in CI on every PR. None of these are code gaps — see [`docs/roadmap.md`](docs/roadmap.md)
  for the full, current list.

[`docs/roadmap.md`](docs/roadmap.md) tracks everything that stands between beta
and a v1 GA.

CI runs the full suite (unit, envtest, helm upgrade, and kind e2e) on every PR. The kind
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

- **Lifecycle**: create, start, stop, restart, clone, delete game servers. Stop
  and restart run the template's declared shutdown sequence first — over
  Source RCON, telnet RCON, or (for pty-console games with no RCON) a
  pod-attach to stdin — so a restart saves the world instead of just sending
  SIGTERM.
- **Idle auto-sleep & Wake-on-connect** (opt-in): scale a server to zero once it
  has reported no players for a set time (`afterMinutes`), and bring it back on
  a cron wake window, the dashboard's Wake button, or inbound player connections.
  Wake-on-connect is powered by the `sentinel` daemon pod using protocol
  handshake classification (`gameproto`) for Minecraft Java and Terraria, and a
  generic connection heuristic for other games.
- **Relay Tunnels**: expose game servers behind NAT or home networks without
  public IPs or router port-forwarding using built-in relay client supervision
  (`tunnel/`) supporting `frp`, `Tailscale`, and `playit`.
- **Console & Actions**: live stdout/stderr over WebSocket, RCON/stdin terminal,
  and UI-triggered custom admin actions defined by module templates and
  validated with `gameaction` injection guards.
- **Logs**: historical log viewer with filtering and download
- **Files**: browse, edit, upload, download server files (Monaco editor in-browser)
- **Players**: per-server player list with kick/ban where the game protocol supports it
- **Backups**: scheduled + on-demand snapshots to S3-compatible storage (restic), with restore back into a server
- **Modules**: versioned game templates distributed as OCI artifacts — 16
  games shipped today, see [`modules/`](modules/)
- **Mods**: install mods across 10 registries (file-drop or mods-by-ID), gated by API key where the
  registry requires one — see [Mods](#mods) below
- **Users & RBAC**: local accounts + OIDC (Keycloak, Google, GitHub)
- **Multi-cluster**: register and switch between clusters from one dashboard —
  the cluster selector, RBAC, and audit log all carry the cluster dimension

## Mods

Gameplane's mod manager supports two install models, chosen per game template:

- **File-drop** — the agent downloads the mod file straight into a per-loader
  volume (Minecraft's `mods/`/`plugins/`, Valheim's BepInEx `plugins/`, and so
  on). This is the model behind the Mods tab's "Install mod" registry browser.
  Downloads are routed through `netguard` SSRF protection.
- **Mods-by-ID** — for games whose *server itself* downloads mods at launch
  (ARK's CurseForge `-mods=` flag, Project Zomboid, Steam Workshop id lists),
  the operator projects the selected mod IDs into a launch environment
  variable instead of fetching anything itself.

Ten registries are supported: Modrinth, CurseForge, Thunderstore, Hangar, the
Factorio mod portal, Steam Workshop, SpigotMC, GitHub Releases, uMod, and
Nexus Mods. Modrinth, Thunderstore, Hangar, Factorio, SpigotMC, GitHub, and
uMod work with no configuration. CurseForge, Steam Workshop, and Nexus Mods
need an API key — set one in **Settings → Mod registries** and the registry
un-hides itself in the Mods browser; until then it's simply absent, not shown
broken. Keys are stored in a Kubernetes Secret and the API never returns the
raw value back, even to the admin who set it.

Two caveats: **Nexus Mods is browse-only** — its download links are
premium-account- and requester-IP-gated, so Gameplane can't complete a
one-click install; you follow the mod page from there yourself. And
**Factorio mod portal downloads need the user's own factorio.com
username + token**, appended in the install form — the portal ties download
links to the requesting account, so Gameplane can't hold or proxy that
credential on your behalf.

## Architecture

```
┌────────────────────────────────────────────────────────────────┐
│  Browser: React + TypeScript + Vite + shadcn/ui                │
└────────────────────────────────────────────────────────────────┘
                            │  HTTPS / WSS
┌────────────────────────────────────────────────────────────────┐
│  API (Go): REST + WebSocket, auth, RBAC, aggregates CRD state  │
└────────────────────────────────────────────────────────────────┘
                            │  K8s API
┌────────────────────────────────────────────────────────────────┐
│  Operator (Go, controller-runtime):                            │
│    reconciles GameServer / GameTemplate / Backup /             │
│    BackupSchedule / Restore / Module / ModuleSource / Cluster  │
│    CRDs into StatefulSets, Services, PVCs, restic Jobs,        │
│    Sentinel pods, and Tunnel pods                              │
└────────────────────────────────────────────────────────────────┘
        │                            │                        │
┌───────┴───────────────┐ ┌──────────┴──────────┐ ┌───────────┴──────────┐
│ GameServer pod:       │ │ Sentinel pod:       │ │ Tunnel pod:          │
│ ├── game container    │ │ └── daemon (Go):    │ │ └── supervisor (Go): │
│ └── agent sidecar     │ │     wake-on-connect │ │     frp / Tailscale  │
│     (Go): RCON, files │ │     protocol listener│ │     / playit relay   │
└───────────────────────┘ └─────────────────────┘ └──────────────────────┘
```

### Components

| Path         | Language | Purpose                                                           |
| ------------ | -------- | ----------------------------------------------------------------- |
| `agent/`     | Go       | Sidecar running in each game pod. RCON, file ops, PTY console, metrics. |
| `api/`       | Go       | Front-end-facing REST + WebSocket gateway. chi, coder/websocket. |
| `audit-syslog-bridge/` | Go | Optional HTTP-JSON → syslog relay behind the audit webhook sink. |
| `charts/`    | Helm     | `gameplane` install chart for operator, API, sentinel, tunnel, and ingress. |
| `deploy/`    | Shell    | Local dev env (kind/k3d) bootstrap scripts and manifests.         |
| `gameaction/`| Go       | Shared command renderer and console-injection guard for module-declared actions. |
| `gameproto/` | Go       | Shared wire-protocol parser for game handshakes (Minecraft, Terraria) used by sentinel. |
| `mcp-server/` | Go       | Optional strictly read-only MCP server for AI assistants (stdio, no writes). |
| `modules/`   | YAML     | Per-game `GameTemplate` bundles (16 games shipped today, see [`modules/`](modules/)). |
| `netguard/`  | Go       | Shared SSRF dial-guard used by the operator (module fetches) and agent (mod installs). |
| `operator/`  | Go       | Reconciles CRDs into K8s objects. Built with controller-runtime.  |
| `sentinel/`  | Go       | Wake-on-connect daemon running in place of sleeping pods to catch inbound player join requests. |
| `svcutil/`   | Go       | Shared Go utilities for HTTP server lifecycle and environment configuration. |
| `telemetry-receiver/` | Go | Optional collector for the API's anonymous daily usage report. |
| `tunnel/`    | Go       | Relay client supervisor pod managing third-party tunnels (`frp`, `Tailscale`, `playit`). |
| `web/`       | TS+React | Dashboard UI. Vite, TanStack Query, xterm.js, Monaco, shadcn/ui.  |

### CRDs (`gameplane.local/v1alpha1`)

- **GameTemplate** — reusable blueprint for a game (image, ports, env, volumes, defaults, lifecycle commands, custom actions)
- **GameServer** — an instance of a GameTemplate with user-specific config (resources, storage, env, idle/wake settings, networking, relay tunnels, mods, backup policy)
- **Backup** — a one-shot restic snapshot job targeting S3-compatible storage
- **BackupSchedule** — a cron-like recurring backup policy for a GameServer
- **Restore** — a one-shot restore of a Backup snapshot into a GameServer's data volume
- **Module** — an installed module bundle; the operator materializes and owns a GameTemplate from it
- **ModuleSource** — a registered store (OCI, git, http, local, or upload) Gameplane pulls module bundles from
- **Cluster** — a registered remote Kubernetes cluster target for multi-cluster control plane operations

## Repo layout

```
.
├── netguard/             # shared SSRF dial-guard (operator + agent)
├── operator/             # controller-runtime operator
│   ├── api/v1alpha1/     # CRD Go types
│   ├── internal/controller/
│   ├── cmd/              # operator main.go
│   └── config/{crd,rbac,samples}
├── api/                  # REST + WS gateway
├── agent/                # in-pod sidecar
├── sentinel/             # wake-on-connect daemon (Go)
├── tunnel/               # relay client supervisor (frp, Tailscale, playit)
├── gameproto/            # Minecraft & Terraria wire-protocol parser (Go)
├── gameaction/           # console injection guard & action renderer (Go)
├── svcutil/              # shared HTTP server lifecycle & env helpers (Go)
├── audit-syslog-bridge/  # optional HTTP-JSON → syslog relay
├── telemetry-receiver/   # optional daily usage reporting collector
├── mcp-server/           # optional read-only Model Context Protocol server
├── web/                  # React dashboard
├── modules/              # git submodule → gameplane-module repo (OCI bundles)
│   ├── minecraft-java/  valheim/  terraria/  rust/  ...  # 16 games total
│   └── build.sh          # OCI bundle builder/pusher (uses oras)
├── website/              # git submodule → gameplane-website repo (public site)
├── charts/gameplane/     # Helm chart
├── deploy/kind/          # local dev cluster bootstrap scripts
├── docs/                 # documentation and architectural guides
├── specs/                # feature specifications and proposals
├── test/                 # integration and e2e kind test suites
├── cosign.pub            # ECDSA P-256 key for verifying signed artifacts (current)
├── cosign-legacy.pub     # Ed25519 key for verifying pre-rotation artifacts
├── cosign.pub.legacy-sig # cross-signature proving trust continuity
└── design.pen            # Pencil design source (do not delete)
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

All published images, the Helm chart, and official module bundles are signed
with the project's cosign key ([`cosign.pub`](cosign.pub), also baked into
the chart for module verification) and recorded in the public Sigstore Rekor
transparency log:

```sh
cosign verify --key cosign.pub \
  ghcr.io/valgulnecron/gameplane/operator:<version>
```

Pre-rotation releases (v0.2.0-beta.7 and earlier) were signed with the retired
Ed25519 key and do not have transparency log entries — verify them with
`cosign-legacy.pub` and `--insecure-ignore-tlog=true`. See
[`docs/key-rotation.md`](docs/key-rotation.md) for details.

## Quickstart (local dev)

Requires: Go 1.25+, Node 20+, Docker, kind, kubectl, helm,
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
3. pushes every directory under `modules/` (16 games at last count — see
   `modules/` for the current list) to the local registry as an OCI module
   bundle,
4. installs the Helm chart from `charts/gameplane/` with a default
   `ModuleSource` pointing at the local registry — the operator
   indexes it within seconds and the modules show up in the dashboard's
   Modules page.

See [`docs/module-authoring.md`](docs/module-authoring.md) for the
bundle format and how to author additional modules.

## Contributing

Design changes go through `design.pen` (Pencil) before any UI code is
written. See [`docs/contributing.md`](docs/contributing.md) for the
full guide: code style, test tiers, and the PR process.

## License

[AGPL-3.0-or-later](./LICENSE). Any network-accessible deployment of a
modified version must make its source available to users.
