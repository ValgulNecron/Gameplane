# Authoring a Kestrel Module

A **module** is a versioned, reusable game-server blueprint. Kestrel
distributes modules as OCI artifacts so the same registries that hold your
game-server container images also hold the templates that wire them up.

A module is one OCI artifact carrying a `GameTemplate` manifest, machine-
readable metadata, and optional README/icon. Admins point Kestrel at a
registry by creating a `ModuleSource` resource; users install a module by
creating (or clicking Install on) a `Module` resource, which the operator
materializes into an in-cluster `GameTemplate`.

## Source layout

A module lives on disk as a directory:

```
modules/<name>/
├── template.yaml   # GameTemplate spec (no metadata.name)
├── module.yaml     # Module metadata (see schema below)
├── README.md       # rendered in the catalog detail drawer
└── icon.png        # optional, 256×256 recommended
```

`template.yaml` is the same `GameTemplate` you would write today, with one
difference: omit `metadata.name`. The name is set on install from
`module.yaml#name`, so a single bundle can be installed under different
names if needed.

## `module.yaml` schema

```yaml
apiVersion: kestrel.gg/module/v1
name: minecraft-java                       # required, DNS-1123 label
displayName: Minecraft (Java Edition)      # required
version: 1.0.0                             # required, semver, must match the OCI tag
game: minecraft-java                       # required, free-form game family identifier
summary: Vanilla / Paper / Forge / Fabric  # required, one-line description for the card
homepage: https://minecraft.net            # optional
license: MIT                               # optional, SPDX identifier
kestrelMinVersion: 0.1.0                   # optional, refuse install on older operators
icon: icon.png                             # optional, filename of the icon layer
```

Field rules:

- `name` is the canonical module identifier within a source. Two modules
  with the same `name` in the same `ModuleSource` are an error.
- `version` is the canonical version string and **must** match the OCI tag
  the bundle is pushed under.
- `kestrelMinVersion` is checked at install time; mismatches fail the
  `Module` reconcile with a clear `Conditions` entry.

## Bundle format (OCI artifact)

A module bundle is a single OCI artifact:

| Layer            | Required | Media type                                            |
| ---------------- | -------- | ----------------------------------------------------- |
| `module.yaml`    | yes      | `application/vnd.kestrel.module.metadata.v1+yaml`     |
| `template.yaml`  | yes      | `application/vnd.kestrel.module.template.v1+yaml`     |
| `README.md`      | no       | `application/vnd.kestrel.module.readme.v1+md`         |
| `icon.png`       | no       | `image/png`                                           |

Manifest:

- `mediaType: application/vnd.oci.image.manifest.v1+json`
- `artifactType: application/vnd.kestrel.module.v1+json`
- `config: { mediaType: application/vnd.kestrel.module.config.v1+json,
  data: "e30=" }` (the empty JSON object `{}` base64-encoded; we don't use
  the config for now but reserve it for future extensions)
- Each layer carries an `org.opencontainers.image.title` annotation set
  to the filename (`module.yaml`, `template.yaml`, …) so the puller can
  identify layers by name.

Reference shape: `<registry>/<repo>/<name>:<version>`. Push every bundle
with both its semver tag and a moving `latest` tag if it's the default
channel.

## Pushing a bundle

The repo ships `modules/build.sh` which wraps `oras` and pushes a bundle
from a `modules/<name>/` directory. Install `oras` (>= 1.2.0):

```sh
brew install oras                 # macOS
# or download from https://oras.land/docs/installation
```

Then:

```sh
# push every bundle in modules/ to a registry
make modules-push REGISTRY=ghcr.io/kestrel-gg/modules

# push a single module to a local kind registry
modules/build.sh push --registry localhost:5001 --name minecraft-java
```

Under the hood `build.sh` runs:

```sh
oras push \
  --artifact-type application/vnd.kestrel.module.v1+json \
  ghcr.io/kestrel-gg/modules/minecraft-java:1.0.0 \
  module.yaml:application/vnd.kestrel.module.metadata.v1+yaml \
  template.yaml:application/vnd.kestrel.module.template.v1+yaml \
  README.md:application/vnd.kestrel.module.readme.v1+md \
  icon.png:image/png
```

Private registries: log in once with `oras login <registry>`. The cluster
side uses a `kubernetes.io/dockerconfigjson` secret (referenced from
`ModuleSource.spec.pullSecretRef`) — the same kind kubelet uses for
private images.

## Installing a module

Once a `ModuleSource` is configured (Helm chart ships a default one
pointing at `ghcr.io/kestrel-gg/modules`), modules show up in the
**Modules** page of the dashboard. Click **Install** to create a
`Module` resource:

```yaml
apiVersion: kestrel.gg/v1alpha1
kind: Module
metadata:
  name: minecraft-java          # becomes GameTemplate name
spec:
  source:
    name: default               # ModuleSource name
  name: minecraft-java
  version: 1.0.0                # omit to track the source's latest
```

The operator pulls the artifact, parses `module.yaml` + `template.yaml`,
and creates a `GameTemplate` owned by this `Module`. Deleting the
`Module` deletes the `GameTemplate`. The operator refuses to delete the
module while any `GameServer` references the template; the UI surfaces
this as a clear "still in use by N servers" message.

## Anatomy of a `GameTemplate` spec

(Most of what follows is unchanged from the pre-OCI authoring guide.)

### Config schema → wizard

Declared `configSchema` fields become inputs in the "New server"
wizard. At reconcile time the operator resolves the server's
`spec.config` against this schema and sets each resolved value as an
env var on the game container.

```yaml
configSchema:
  - name: MAX_PLAYERS
    displayName: Max players
    type: int
    default: "16"
  - name: DIFFICULTY
    type: enum
    enum: [easy, normal, hard]
    default: normal
  - name: SERVER_PASS
    displayName: Password
    type: password
```

Materialization rules:

- **Defaults apply at reconcile time.** A key absent from
  `spec.config` resolves to its `default`, so `kubectl apply` of a
  bare GameServer behaves exactly like the wizard (which pre-fills
  defaults).
- **Empty optional values are skipped** — no env var is set at all,
  letting the game image fall back to its own default. "Leave blank
  for an open server" means *unset*, not `PASSWORD=""`.
- **Validation is strict.** Unknown keys, enum violations,
  unparseable `int`/`bool` values, and empty `required` fields fail
  the GameServer (phase `Failed`, message on the `Ready` condition)
  instead of materializing a pod that ignores user intent. Fixing
  `spec.config` recovers automatically.
- **`password` fields never appear inline in the pod spec.** Values
  are stored in an owned `<server>-config` Secret and injected via
  `SecretKeyRef`.
- **Precedence**: template `env` < schema-resolved config <
  GameServer `spec.env` — an explicit env override always wins.
- **`target: file` fields feed `configFiles` templates** instead of
  becoming env vars — see the next section.

### Config files

Games that read a config *file* instead of env vars (Terraria's
`serverconfig.txt` is the intended first adopter) declare
`configFiles`: a list of files the operator renders from the resolved
config values and places under `storage.mountPath` before the game
starts.

```yaml
configSchema:
  - name: MOTD
    type: string
    default: Welcome!
    target: file          # consumed by templates below, never an env var
  - name: SERVER_PASS
    type: password
    target: file
configFiles:
  - path: serverconfig.txt          # relative to storage.mountPath
    template: |
      motd={{ .Values.MOTD }}
      {{ if .Values.SERVER_PASS }}password={{ .Values.SERVER_PASS }}{{ end }}
      world={{ .Server.Name }}
```

`template` is a Go [`text/template`](https://pkg.go.dev/text/template)
rendered with:

- **`.Values`** — every `configSchema` field name mapped to its
  resolved value. Unset optional fields are present as `""`, so
  `{{ if .Values.X }}` guards work; referencing a name outside the
  schema fails the GameServer (`missingkey=error`). Env-target values
  are available too — a value may drive both an env var and a file.
- **`.Server`** — `.Name` and `.Namespace` of the GameServer.

Rules and semantics:

- **Paths are relative to `storage.mountPath`.** Absolute paths,
  `..` segments, unclean paths, and duplicates fail the GameServer.
- **Rendered files live in an owned `<server>-files` Secret** —
  always a Secret, never a ConfigMap, because any template may embed
  a password value.
- **Files are copied onto the data volume by a `config-init`
  container on every pod start.** The operator's rendering wins:
  manual edits to these paths (e.g. via the dashboard's Files tab)
  are overwritten on the next restart. Games may freely rewrite the
  files at runtime — the copy is plain data on the PVC, not a
  read-only mount.
- **The copy runs as root (busybox).** Non-root game images that
  rewrite their config need permissions compatible with root-owned
  files.
- **Changes roll the pod.** Rendered contents are part of the config
  hash, so editing a template or a file-target value restarts the
  server with the new file.
- **Failures are strict.** A parse error, a missing key, or a
  file-target value supplied while the template declares no
  `configFiles` fails the GameServer (phase `Failed`, message on the
  `Ready` condition) instead of silently dropping user intent.

### RCON

For Source-protocol games (Minecraft, Valve engine titles) set:

```yaml
rcon:
  protocol: source
  port: 25575
```

The operator mints a password secret per GameServer and mounts it to the
agent. The dashboard's console tab then works automatically.

For games without RCON (e.g. Valheim) set `protocol: none` — the agent
won't try to connect, and the console tab degrades to "server doesn't
support a live console" rather than failing.

### Probes

```yaml
probes:
  readiness:
    httpGet: { path: /health, port: 8080 }
    initialDelaySeconds: 30
    failureThreshold: 10
```

Many game images don't expose HTTP. The `itzg/minecraft-server` image
ships a `mc-health` exec probe; Source images usually expose nothing
useful, so readiness falls back to TCP on the game port.

### Storage layout

`spec.storage.mountPath` is where the PVC is mounted inside the game
container. Pick the path the game writes its world/config files to —
backups snapshot this whole directory.

## Testing a new module locally

```sh
# 1. write modules/<name>/{template.yaml,module.yaml,README.md}
# 2. push to the local kind registry
modules/build.sh push --registry localhost:5001 --name <name>

# 3. wait for the default ModuleSource to pick it up (≤ refreshInterval)
kubectl get modulesource default -o jsonpath='{.status.modules[*].name}'

# 4. install via UI or CR
kubectl apply -f - <<EOF
apiVersion: kestrel.gg/v1alpha1
kind: Module
metadata: { name: <name> }
spec:
  source: { name: default }
  name: <name>
EOF

# 5. verify the GameTemplate showed up
kubectl get gametemplate <name> -o yaml
```

Bump `module.yaml#version`, re-push under the new tag, and the catalog
reports an upgrade available within `refreshInterval`. Click **Upgrade**
in the UI (or `kubectl patch module <name> --type merge -p
'{"spec":{"version":"<v>"}}'`).
