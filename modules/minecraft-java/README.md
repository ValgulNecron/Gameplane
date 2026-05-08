# `minecraft-java` module

Kestrel GameTemplate for Minecraft: Java Edition.

## Install

```sh
kubectl apply -f modules/minecraft-java/template.yaml
```

The template is cluster-scoped — install once per cluster. The Kestrel
dashboard will then show Minecraft in the catalog.

## Usage

Either create a server from the dashboard ("New Server" → "Minecraft
Java Edition"), or apply a GameServer manifest directly:

```sh
kubectl apply -f modules/minecraft-java/samples/gameserver.yaml
```

## Image

Uses [`itzg/minecraft-server`](https://github.com/itzg/docker-minecraft-server)
— the de-facto community image for Minecraft Java servers. Understands
`TYPE`, `VERSION`, `DIFFICULTY`, `MODE`, `MAX_PLAYERS`, `MOTD`, etc.

## Ports

| Name | Port   | Protocol | Advertised |
| ---- | ------ | -------- | ---------- |
| game | 25565  | TCP      | yes        |
| rcon | 25575  | TCP      | no         |

RCON stays inside the pod network — the Kestrel agent sidecar connects
to it locally for console stdin and player queries.

## Storage

Default PVC is 10 GiB mounted at `/data`. Override via `GameServer.spec.storage`.

## Backups

World data lives under `/data/world` (and `world_nether`, `world_the_end`
for vanilla). A `restic-snapshot` backup of the whole `/data` volume is
sufficient to restore.
