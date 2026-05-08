# `valheim` module

Kestrel GameTemplate for Valheim dedicated servers.

## Install

```sh
kubectl apply -f modules/valheim/template.yaml
```

## Ports

| Name   | Port | Protocol | Advertised |
| ------ | ---- | -------- | ---------- |
| game   | 2456 | UDP      | yes        |
| game2  | 2457 | UDP      | yes        |
| game3  | 2458 | UDP      | yes        |
| status | 80   | TCP      | no         |

All three UDP game ports must be open on the client network — Steam's
discovery protocol hits all of them. `expose: NodePort` + `hostPort` is
the usual pattern for homelab setups.

## Storage

Default PVC is 5 GiB at `/config`. World saves live under
`/config/worlds_local`.

## Backups

`BACKUPS=false` is the default so the Kestrel Backup CRD is the
authoritative snapshot path. Backups of the whole `/config` volume
restore cleanly onto a fresh pod.
