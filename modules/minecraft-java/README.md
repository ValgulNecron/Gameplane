# `minecraft-java` module

Kestrel GameTemplate for Minecraft: Java Edition.

## Install

```sh
kubectl apply -f modules/minecraft-java/template.yaml
```

The template is cluster-scoped — install once per cluster. The Kestrel
dashboard will then show Minecraft in the catalog.

## Version & loader choice

The New Server wizard shows a version picker populated from `spec.versions`
— e.g. `1.21.4 · Paper`, `1.21.4 · Forge`, `Latest · Vanilla`. Each entry
selects the [`itzg/minecraft-server`](https://github.com/itzg/docker-minecraft-server)
image plus the `TYPE`/`VERSION` env that pick the server software, so you
never set those by hand. To pin an exact image build, set
`GameServer.spec.image` (Settings → Image override).

## Mod manager

Mods and plugins are managed from the **Mods** tab. Each *(version + loader)*
combination has its own volume, so a Paper plugin set and a Forge mod set
never collide and survive switching versions. Plugin loaders
(Paper/Spigot/Bukkit/Purpur) store under `plugins/`; mod loaders
(Forge/Fabric/Quilt) under `mods/`. Vanilla servers have no mod loader, so
the tab is hidden for them. Installs are allowed from Modrinth, CurseForge,
Hangar, and GitHub (max 512 MiB), subject to the agent's SSRF guard.

The Mods tab's **Search registry** mode browses [Modrinth](https://modrinth.com)
filtered to the server's active loader and game version, so you can find and
one-click install a mod or plugin by name without hunting for a URL. The
chosen file downloads through the same allowlist (Modrinth's CDN is listed).
**From URL** install remains available for anything not on Modrinth.

## Logs (including install)

itzg downloads the selected server jar on first boot; that output streams to
the **Logs** tab's *Container output* source, so you can watch the install
even before the game has started. Once running, the persistent logfile at
`/data/logs/latest.log` feeds the *Game log* toggle.

## Ports

| Name | Port  | Protocol | Advertised |
| ---- | ----- | -------- | ---------- |
| game | 25565 | TCP      | yes        |
| rcon | 25575 | TCP      | no         |

RCON stays inside the pod network — the Kestrel agent uses it for the
Console tab, player moderation, backup quiesce, and Overview metrics.

## Storage

Default data PVC is 10 GiB mounted at `/data` (override via
`GameServer.spec.storage`). Each version+loader mod volume is a separate PVC
the operator provisions on demand and retains across version switches.

## Backups

World data lives under `/data/world` (and `world_nether`, `world_the_end`
for vanilla). A backup of the whole `/data` volume restores the world; mod
volumes are separate PVCs.
