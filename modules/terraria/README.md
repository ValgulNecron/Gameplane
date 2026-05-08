# Terraria

Terraria dedicated server, packaged as a Kestrel module.

**Image:** [`ryshe/terraria:latest`](https://github.com/ryshe/terraria-docker)

## What's included

- World generation on first start (configurable size + difficulty)
- Autosave every 10 minutes (image default)
- Console attached to the container's stdin/stdout — type `help` in the
  dashboard's Console tab to see the command list

## Ports

- `7777/tcp` — game

## Storage

A `/world` PVC is mounted into the game container; the world file lives
there. Size defaults to 2 GiB which is plenty for a medium world.

## Backups

Standard Kestrel `Backup` / `BackupSchedule` works — restic snapshots the
PVC contents. Suspend / resume the server only when you need a fully-
quiesced backup; live snapshots are also fine for Terraria since the
server flushes the world file on every autosave.

## Notes

- No RCON. Use the Console tab (pty) for live commands.
- The server announces port via UPnP if your network supports it; in
  Kubernetes you'll usually expose it via `Service` (set on the
  GameServer spec) or LoadBalancer / NodePort instead.
