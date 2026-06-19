# Terraria

Terraria + tModLoader dedicated server, packaged as a Kestrel module.

**Image:** [`passivelemon/terraria-docker`](https://github.com/PassiveLemon/terraria-docker)

## Version choice

The New Server wizard's version picker maps directly to an image tag:
`Vanilla · latest`, `tModLoader · latest`, and `tModLoader · latest preview`.
Vanilla and tModLoader are the same project shipped as different tags — there
is no version env var. To pin a specific build (`terraria-<version>` or
`tmodloader-<version>`), set `GameServer.spec.image` (Settings → Image
override) or add an entry to `spec.versions`.

## Mod manager (tModLoader)

tModLoader servers get a **Mods** tab that manages modpacks under
`/opt/terraria/config/ModPacks` — its own per-(version+loader) volume, so
each tModLoader build keeps its own set. Select the active modpack with the
**Active modpack** (`MODPACK`) config field. Vanilla Terraria has no mods, so
the tab is hidden for vanilla servers. Installs are allowed from GitHub
(max 512 MiB), subject to the agent's SSRF guard.

## Logs (including install)

The server downloads/extracts on first boot and logs to stdout — watch
first-world generation and startup in the **Logs** tab's *Container output*
source. There is no persistent logfile, so the *Game log* toggle is not
offered.

## Console

No RCON. The **Console** tab attaches to the container's stdin/stdout (pty) —
type `help` for the command list.

## Ports

- `7777/tcp` — game

Expose it via `GameServer.spec.networking` (NodePort/LoadBalancer); the
in-container UPnP announce is not used in Kubernetes.

## Storage

Config, worlds, and modpacks live under `/opt/terraria/config` (default
4 GiB) — worlds in `config/Worlds`, modpacks in `config/ModPacks`. The
tModLoader ModPacks volume is a separate PVC per version+loader.

## Backups

Standard Kestrel `Backup` / `BackupSchedule` snapshots the data PVC. Live
snapshots are fine for Terraria since the server flushes the world on every
autosave.

## Migrating from the ryshe/terraria module (1.x)

This 2.0 module switches the image from `ryshe/terraria` to
`passivelemon/terraria-docker`. The data mount moves from `/world` to
`/opt/terraria/config` and several env names change (`WORLD_FILENAME` →
`WORLDNAME`, `MAX_PLAYERS` → `MAXPLAYERS`, plus new `SEED`/`SECURE`/
`LANGUAGE`/`MODPACK`). Treat it as a new template revision: existing running
servers are not auto-migrated — create new servers on 2.0, and copy worlds
across manually if needed.
