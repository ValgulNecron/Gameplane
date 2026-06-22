# Project Name Proposals

Context: the current project is more than a dashboard. It is a Kubernetes-native
game server control plane with CRDs, an operator, API gateway, per-pod agent,
web UI, modules, backups, RBAC, and Helm packaging.

These names are product-fit proposals only. They are not trademark, domain, or
package-registry availability checks.

## Recommended Shortlist

### 1. Gameplane

**Tagline:** A Kubernetes-native control plane for game servers.

Why it fits:

- Says what the project is: a control plane for game infrastructure.
- Feels native to Kubernetes without using "kube" directly.
- Works well across product, chart, CLI, org, and API naming.

Possible naming:

- Product: `Gameplane`
- Repo: `gameplane`
- Helm chart: `gameplane`
- API group: `gameplane.gg`
- Namespace: `gameplane-system`
- Game namespace: `gameplane-games`

Verdict: strongest overall option.

### 2. Realmplane

**Tagline:** Control plane for self-hosted game realms.

Why it fits:

- Combines game-world language with infrastructure language.
- More distinctive than `Gameplane`.
- Good fit for Minecraft, Valheim, Terraria, and future game modules.

Tradeoff:

- Slightly less obvious than `Gameplane` on first read.

### 3. SpawnGrid

**Tagline:** Spawn and manage game servers on Kubernetes.

Why it fits:

- "Spawn" maps naturally to creating servers.
- "Grid" hints at clusters, scheduling, and distributed infrastructure.
- More playful while still clear.

Tradeoff:

- Strong game tone, weaker enterprise/control-plane tone.

### 4. Serverdeck

**Tagline:** A control deck for multiplayer game servers.

Why it fits:

- Dashboard-friendly and easy to say.
- Clear enough for users who are not deep into Kubernetes.
- Leaves room for the product to grow beyond Kubernetes internals.

Tradeoff:

- Less Kubernetes-native in tone.

### 5. RealmOps

**Tagline:** Operations for game servers and player realms.

Why it fits:

- Strong operations/admin feel.
- Good for a product centered on lifecycle, backups, RBAC, logs, and files.
- Short and memorable.

Tradeoff:

- Sounds more like a service/company than an open-source project.

## Additional Candidates

### Playgrid

Simple, friendly, and cluster-aware. Good if the project wants a lighter
community/homelab tone.

### Gamefleet

Very clear for managing many game servers. Good fit for the dashboard's fleet
health, backups, and lifecycle views.

### ClusterQuest

Memorable and playful. Best if the brand should feel fun and open-source first,
less ideal for serious hosting/operator positioning.

### PodQuest

Kubernetes-flavored and game-flavored. Works for a project aimed at Kubernetes
users, but may feel too cute for production hosting.

### Realmstack

Suggests a full stack for running game realms. Stronger as a platform name than
as a low-level operator name.

### PlayOps

Short and direct. Easy to remember, but likely generic and harder to own.

### GameOps

The clearest descriptive option. Good for search intent, weak for uniqueness.

### Modport

Good if modules and server templates become the primary product identity. Too
narrow if the product remains a full control plane.

### AtlasPlay

Suggests maps, worlds, and multi-cluster scope. More brand-like, less obvious
for infrastructure.

### ControlRealm

Descriptive and accurate, but longer and less clean as a CLI/chart/repo name.

### Ludopod

Uses "ludo" for play plus Kubernetes pods. Distinctive, but the meaning is less
immediate for most users.

## Names I Would Avoid

### Kestrel

The current name is clean, but it has two practical issues:

- It is already strongly associated with the ASP.NET Core Kestrel web server.
- It is deeply embedded in the repo as API groups, labels, media types, cookies,
  image names, chart names, namespaces, and docs, so a rename is a real migration.

### KubeGame, KubePlay, KubeCraft

These are obvious but generic. They also tie the brand too tightly to a `kube`
prefix, which many Kubernetes projects already use.

### GameDash

Too small for the scope. This project is an operator-backed control plane, not
just a dashboard.

### MineOps or CraftOps

Too Minecraft-specific. The repo already supports multiple game modules and is
designed to grow.

## Final Recommendation

Use **Gameplane** if the goal is a serious open-source infrastructure project.
It is accurate, short, and maps cleanly to every technical surface:

- `gameplane`
- `gameplane.gg`
- `charts/gameplane`
- `gameplane-system`
- `gameplane-games`
- `application/vnd.gameplane.module.v1+json`

Use **SpawnGrid** if the brand should feel more playful and game-native.

Use **RealmOps** if the target audience is server admins and small hosting
operators rather than Kubernetes platform engineers.
