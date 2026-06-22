# `valheim` module

Gameplane GameTemplate for Valheim dedicated servers.

## Install

```sh
kubectl apply -f modules/valheim/template.yaml
```

## Version (channel) choice

The New Server wizard's version picker offers **Stable** and **Public Test
(beta)**, which set the `PUBLIC_TEST` env. Both run the same
[`lloesche/valheim-server`](https://github.com/lloesche/valheim-server-docker)
image; the channel decides which Steam build SteamCMD installs. To pin a
specific image build, set `GameServer.spec.image` (Settings → Image override).

## Mod manager

Enable **BepInEx mods** (`BEPINEX`) when creating the server, then manage
`.dll` plugins from the **Mods** tab. The mod volume mounts at
`/config/bepinex/plugins` — its own per-channel PVC, so a public-test mod set
never leaks into stable. Installs are allowed from Thunderstore and GitHub
(max 256 MiB), subject to the agent's SSRF guard. Mods only load when
`BEPINEX` is enabled.

The Mods tab's **Search registry** mode browses the
[Thunderstore Valheim community](https://thunderstore.io/c/valheim/) so you
can find and one-click install BepInEx plugins by name. **From URL** install
remains available for anything not on Thunderstore.

## Logs (including install)

The server installs via SteamCMD on start; that output (and all runtime
logging) goes to stdout — watch it in the **Logs** tab's *Container output*
source. There is no persistent logfile, so the *Game log* toggle is not
offered.

## Console

No RCON. The **Console** tab attaches to the container's stdin/stdout (pty).

## Ports

| Name   | Port | Protocol | Advertised |
| ------ | ---- | -------- | ---------- |
| game   | 2456 | UDP      | yes        |
| game2  | 2457 | UDP      | yes        |
| game3  | 2458 | UDP      | yes        |
| status | 80   | TCP      | no         |

All three UDP game ports must be open on the client network — Steam's
discovery protocol hits all of them. `expose: NodePort` is the usual pattern
for homelab setups.

## Storage

Default PVC is 5 GiB at `/config`; world saves live under
`/config/worlds_local`. The BepInEx mod volume is a separate per-channel PVC.

## Backups

`BACKUPS=false` is the default so the Gameplane Backup CRD is the authoritative
snapshot path. A backup of the whole `/config` volume restores cleanly onto a
fresh pod.
