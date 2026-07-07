# Authoring a Gameplane Module

A **module** is a versioned, reusable game-server blueprint. Gameplane
distributes modules as OCI artifacts so the same registries that hold your
game-server container images also hold the templates that wire them up.

A module is one OCI artifact carrying a `GameTemplate` manifest, machine-
readable metadata, and optional README/icon. Admins point Gameplane at a
registry by creating a `ModuleSource` resource; users install a module by
creating (or clicking Install on) a `Module` resource, which the operator
materializes into an in-cluster `GameTemplate`.

## Source layout

The official modules live in the standalone **`gameplane-module`** repo, which
the main repo checks out as the `modules/` submodule. Whether you work in that
repo directly or through the submodule, a module lives on disk as a directory:

```
modules/<name>/
├── template.yaml   # GameTemplate spec (no metadata.name)
├── module.yaml     # Module metadata (see schema below)
├── README.md       # rendered in the catalog detail drawer
└── icon.png        # optional, 256×256 recommended
```

`modules/build.sh` and `make modules-push` are unchanged by the split — the
submodule mounts the `gameplane-module` repo root at `modules/`, so the paths
below still resolve.

`template.yaml` is the same `GameTemplate` you would write today, with one
difference: omit `metadata.name`. The name is set on install from
`module.yaml#name`, so a single bundle can be installed under different
names if needed.

## `module.yaml` schema

```yaml
apiVersion: gameplane.local/module/v1
name: minecraft-java                       # required, DNS-1123 label
displayName: Minecraft (Java Edition)      # required
version: 1.0.0                             # required, semver, must match the OCI tag
game: minecraft-java                       # required, free-form game family identifier
summary: Vanilla / Paper / Forge / Fabric  # required, one-line description for the card
homepage: https://minecraft.net            # optional
license: MIT                               # optional, SPDX identifier
gameplaneMinVersion: 0.1.0                   # optional, refuse install on older operators
icon: icon.png                             # optional, filename of the icon layer
```

Field rules:

- `name` is the canonical module identifier within a source. Two modules
  with the same `name` in the same `ModuleSource` are an error.
- `version` is the canonical version string and **must** match the OCI tag
  the bundle is pushed under.
- `gameplaneMinVersion` is checked at install time against the operator's
  build version; a module that needs a newer operator fails the `Module`
  reconcile with an `IncompatibleOperator` condition. The check is skipped
  for `dev`/unversioned operator builds.

## Bundle format (OCI artifact)

A module bundle is a single OCI artifact:

| Layer            | Required | Media type                                            |
| ---------------- | -------- | ----------------------------------------------------- |
| `module.yaml`    | yes      | `application/vnd.gameplane.module.metadata.v1+yaml`     |
| `template.yaml`  | yes      | `application/vnd.gameplane.module.template.v1+yaml`     |
| `README.md`      | no       | `application/vnd.gameplane.module.readme.v1+md`         |
| `icon.png`       | no       | `image/png`                                           |

Manifest:

- `mediaType: application/vnd.oci.image.manifest.v1+json`
- `artifactType: application/vnd.gameplane.module.v1+json`
- `config: { mediaType: application/vnd.gameplane.module.config.v1+json,
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
make modules-push REGISTRY=ghcr.io/valgulnecron/gameplane-modules

# push a single module to a local kind registry
modules/build.sh push --registry localhost:5001 --name minecraft-java
```

Under the hood `build.sh` runs:

```sh
oras push \
  --artifact-type application/vnd.gameplane.module.v1+json \
  ghcr.io/valgulnecron/gameplane-modules/minecraft-java:1.0.0 \
  module.yaml:application/vnd.gameplane.module.metadata.v1+yaml \
  template.yaml:application/vnd.gameplane.module.template.v1+yaml \
  README.md:application/vnd.gameplane.module.readme.v1+md \
  icon.png:image/png
```

Private registries: log in once with `oras login <registry>`. The cluster
side uses a `kubernetes.io/dockerconfigjson` secret (referenced from
`ModuleSource.spec.oci.pullSecretRef`) — the same kind kubelet uses for
private images.

## Module sources

`ModuleSource` declares where modules come from. `spec.type` selects
the store; everything except OCI auto-discovers any directory holding
a `module.yaml` (use `spec.allow` to filter by name or glob). Sources
can be managed from the dashboard (admin) or applied as CRs:

```yaml
apiVersion: gameplane.local/v1alpha1
kind: ModuleSource
metadata: { name: community }
spec:
  type: git                  # oci | git | http | local | upload
  git:
    url: https://github.com/example/gameplane-modules
    ref: main                # branch or tag; the resolved commit is the digest
    subPath: modules         # optional scan root inside the repo
    secretRef: { name: gh-creds }   # optional, in the operator namespace
  allow: ["minecraft-*"]     # optional name filter (all types)
  refreshInterval: 30m
```

Per-type config:

| type | config | versioning | auth secret keys |
|---|---|---|---|
| `oci` | `oci.url` + explicit `oci.modules` list | registry tags (semver) | dockerconfigjson via `oci.pullSecretRef` |
| `git` | `git.url`, `ref`, `subPath` | one stream: `module.yaml#version` at the ref; digest = commit | `token` or `username`+`password` (https); `ssh-privatekey` + `known_hosts` (ssh, both required) |
| `http` | `http.url` to a `.tar.gz`/`.zip` | one stream; digest = content hash | `token` (Bearer) or `username`+`password` (Basic) |
| `local` | `local.path` under the operator's `--module-local-root` mount (Helm: `operator.localModules`) | one stream; digest = content hash | — |
| `upload` | none — indexes uploaded bundles | one stream; digest = content hash | — |

**Network safety.** The operator fetches `git`/`http` sources through a
guard that refuses link-local, cloud-metadata (`169.254.169.254`),
unspecified, and multicast destinations — so a source can't be aimed at the
instance-metadata endpoint to steal the operator's credentials. Private and
loopback addresses are allowed, because self-hosted GitLab/Harbor and a
local kind registry legitimately live there.

### Uploaded bundles

`type: upload` sources index ConfigMaps in the operator namespace
labeled `gameplane.local/module-upload: "true"`, each holding one bundle's
files under their canonical names. The dashboard's **Upload module**
flow creates these via `POST /modules/sources/{name}/upload`
(tar.gz/zip, ≤ 900 KiB), but a hand-applied ConfigMap indexes exactly
the same way:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: module-upload-mygame
  namespace: gameplane-system
  labels: { gameplane.local/module-upload: "true" }
binaryData:        # or stringData for plain YAML
  module.yaml: <base64>
  template.yaml: <base64>
```

## Installing a module

Once a `ModuleSource` is configured (Helm chart ships a default one
pointing at `ghcr.io/valgulnecron/gameplane-modules`), modules show up in the
**Modules** page of the dashboard. Click **Install** to create a
`Module` resource:

```yaml
apiVersion: gameplane.local/v1alpha1
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

If a `Module` fails to upgrade (pull error, bad signature, incompatible
operator), the previously-installed `GameTemplate` keeps running unchanged;
`status.previousVersion` records the last-known-good to roll back to by
re-pinning `spec.version`.

### Verifying and pinning bundles

Two independent, opt-in controls harden the supply chain for OCI modules:

**Digest pin** (`Module.spec.digest`) refuses to install unless the resolved
bundle's digest matches exactly — defeating a tag that was moved to point at
new content. Set it to the OCI manifest digest shown in the catalog:

```yaml
spec:
  source: { name: default }
  name: minecraft-java
  version: 1.0.0
  digest: sha256:abc123…        # install fails (DigestMismatch) on any drift
```

**Signature verification** (`ModuleSource.spec.verify`, OCI sources only)
requires every bundle from the source to carry a valid [cosign][cosign]
signature before it is installed. A bundle that is unsigned or signed by the
wrong key/identity fails the install with a `SignatureInvalid` condition.
Keyed:

```yaml
apiVersion: gameplane.local/v1alpha1
kind: ModuleSource
metadata: { name: trusted }
spec:
  type: oci
  oci: { url: ghcr.io/valgulnecron/gameplane-modules, modules: [{ name: minecraft-java }] }
  verify:
    key: { name: cosign-pub }    # Secret in the operator namespace,
                                 # public key under data "cosign.pub"
```

Keyless (Fulcio certificate identity) — for sources whose author signs with a
CI OIDC identity instead of a key. The operator needs outbound access to the
sigstore trust root and Rekor:

```yaml
  verify:
    keyless:
      issuer: https://token.actions.githubusercontent.com
      identity: https://github.com/<org>/<repo>/.github/workflows/release.yaml@refs/tags/v1.0.0
```

The official Gameplane bundles are **keyed**-signed (offline, no Rekor — see
[Signing official bundles](#signing-official-bundles) below), so verify them
with the keyed form above.

`spec.verify` is rejected on non-OCI sources (cosign signatures are an OCI
concept); `git`/`http`/`local`/`upload` rely on the content digest plus a
`Module.spec.digest` pin instead.

[cosign]: https://docs.sigstore.dev/cosign/overview/

### Signing official bundles

The official `modules/*` bundles are signed by the release pipeline so installs
can verify them offline. Signing is **keyed** (an Ed25519 key, no transparency
log) — the operator's keyed verify path needs no Fulcio/Rekor connectivity,
which suits air-gapped and self-hosted clusters.

**One-time key setup (maintainer).** Generate a project signing key and store
the halves in the right places — the private key never leaves CI secrets, the
public key ships to users:

```sh
cosign generate-key-pair                 # writes cosign.key (private) + cosign.pub
# CI secrets (repo settings): paste the contents of each file
#   COSIGN_PRIVATE_KEY = <cosign.key>    COSIGN_PASSWORD = <the passphrase>
# Commit cosign.pub at the repo root — CI drift-checks it against the
# private key on every publish, and it ships as a release asset.
```

The `release.yaml` `modules` job then runs `modules/build.sh push --sign` on
every `v*` tag, pushing each bundle to `ghcr.io/<owner>/gameplane-modules` and
signing it by digest. The job is gated on `COSIGN_PRIVATE_KEY`: until the key
exists the release simply skips module publishing.

**Verifying the official source (operator).** Signing is an OCI concept, so
switch the default source to `type: oci` and turn verification on. The official
module-signing public key already ships in the chart's
`defaultModuleSource.oci.verify.cosignPublicKey`, so you only flip `enabled`:

```yaml
# values.yaml
defaultModuleSource:
  type: oci
  oci:
    verify:
      enabled: true
      # cosignPublicKey ships with the chart (the official Ed25519 key,
      # same as the repo-root cosign.pub);
      # override it only to pin a different signer.
```

The chart writes the key to a `gameplane-module-cosign-pub` Secret in the
operator namespace and sets `spec.verify.key` on the default source.

**Signing your own bundles.** `build.sh --sign` works for any registry; set
`COSIGN_PRIVATE_KEY` (and `COSIGN_PASSWORD` if the key is encrypted) and point a
`ModuleSource.spec.verify.key` Secret at the matching public key:

```sh
export COSIGN_PRIVATE_KEY="$(cat cosign.key)" COSIGN_PASSWORD=…
modules/build.sh push --registry ghcr.io/you/your-modules --sign
```

## Anatomy of a `GameTemplate` spec

The CRD types in `operator/api/v1alpha1/gametemplate_types.go` are the
source of truth; this section documents every author-facing block. The
official modules are the canonical examples: **minecraft-java**
(env-versioned catalog, per-loader mod volumes, three registries,
env-mode modpacks), **valheim** (channel catalog, extract-mode BepInEx
mods, deps-mode Thunderstore packs), **terraria** (tag catalog, pty
console, configFiles), **factorio** (tag catalog, pty console,
wizard-managed server-settings.json, portal registry), **palworld**
(wrapper channels, RCON via a shared admin env, pak mods).

### Branding

```yaml
spec:
  icon: icon.png            # bundle file or URL/data-URI shown in the catalog
  accentColor: "#5b9a3e"    # CSS hex; tints this game's icon + accents
```

`accentColor` is how a module carries its own brand color into the
dashboard — the app no longer hardcodes a per-game palette, so a new
game shows the right color without any frontend change. Omit it for a
neutral default.

### Version catalog (`spec.versions`)

`spec.versions` declares the selectable game versions/flavors. The "New
server" wizard shows the catalog as a picker, and an existing server can
switch entries later from **Settings → Version** (the operator re-renders
the StatefulSet and restarts the pod on the new entry).

```yaml
image: itzg/minecraft-server:java21   # fallback when no version is selected
versions:
  - id: "1.21.4-paper"        # what GameServer.spec.version stores (≤ 40 chars)
    displayName: "1.21.4 · Paper"
    image: itzg/minecraft-server:java21   # full image ref for this entry
    loader: paper             # keys into capabilities.mods.loaders (below)
    gameVersion: "1.21.4"     # upstream token passed to mod registries
    default: true             # pre-selected in the wizard (one entry)
    env:                      # appended when selected
      - { name: TYPE, value: PAPER }
      - { name: VERSION, value: "1.21.4" }
```

How authors express "a version" is up to the image (all three styles are
used by the official modules):

- **env-versioned** — one image, per-entry `env` selects the software
  (minecraft-java: itzg `TYPE`/`VERSION`; the tag only pins the Java
  runtime);
- **tag-versioned** — the image tag *is* the game version (factorio:
  `stable`/`latest`/pinned; terraria: `tmodloader-*`);
- **channel** — entries select an upstream channel via env (valheim:
  `stable` vs `public-test`).

Semantics:

- `GameServer.spec.version` selects an entry by `id`; empty falls back to
  the `default: true` entry (else the first). An id that matches no entry
  **fails the server loudly** (phase `Failed`) instead of silently using
  the template image. `GameServer.spec.image` overrides whatever image
  the entry resolves to.
- Env precedence: template `env` < the entry's `env` < schema-resolved
  config < GameServer `spec.env`.
- **Each (version + loader) combination gets its own mod volume** —
  switching entries mounts that combo's PVC and leaves the others intact,
  so a Paper plugin set survives a detour through Fabric. See §Mods.
- At most 64 entries; `loader` must match a key of
  `capabilities.mods.loaders` (or be absent/`vanilla`-style for entries
  with no mod manager). Use the registry's loader token verbatim (e.g.
  Modrinth's `paper`/`fabric`/`neoforge`) so browse filtering works.

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
  passwordEnv: RCON_PASSWORD   # env the game image reads the password from
```

The operator mints a password secret per GameServer, injects it into the
game container via `passwordEnv`, and mounts the same value for the agent
— the dashboard's console tab (and every RCON-backed capability) then
works automatically. Some images reuse one env for RCON and in-game admin
(Palworld's `ADMIN_PASSWORD`); that's fine — name it here.

#### Password source precedence

When the game image manages its own RCON password file (e.g., Factorio's
`config/rconpw`), use `passwordFile` instead of `passwordEnv`:

```yaml
rcon:
  protocol: source
  port: 25575
  passwordFile: config/rconpw  # path relative to the game data mount
```

In this mode, the operator does not generate a Secret or inject any env var.
The agent reads the password directly from the file on every connection.

The password source is chosen by precedence: `passwordSecretRef` (external
Secret) > `passwordFile` (game-managed) > operator-generated Secret (default).

| Mode | Use case | Secret injected | Env var | Example |
|------|----------|-----------------|---------|---------|
| operator-generated (default) | Operator controls the password | Yes (auto-created) | Via `passwordEnv` | Minecraft with `RCON_PASSWORD` |
| `passwordSecretRef` | Use external credentials | Yes (external) | Via `passwordEnv` | Any game with your own Secret |
| `passwordFile` | Game manages the password | No | No | Factorio with `config/rconpw` |

For games without usable RCON set `protocol: none`. Real cases from the
official modules:

- the game simply has none (Valheim, Terraria);
- the image manages its own RCON password in a file the operator can't
  inject into — use `passwordFile` to read from the data volume.

### Console mode and game log

```yaml
consoleMode: pty              # attach to container stdin/stdout
logPath: /data/logs/latest.log   # agent tails this for the "Game log" view
```

With `rcon.protocol: none`, set `consoleMode: pty` if the server reads
commands on stdin (Terraria, Factorio) — the Console tab then attaches
via the kubelet's pod-attach API instead. Without either, the tab shows
"no live console" and RCON-backed capabilities (players, quiesce,
lifecycle, actions, status) are reported unsupported.

`logPath` is an absolute in-container path pointing the agent at the
game's own logfile. Omit it for games that log only to stdout; the Logs
tab's "Container output" source covers those.

### Capabilities (moderation + backup quiesce)

`spec.capabilities` declares the console commands behind agent
features, so a module adds full Players-tab moderation and safe
backups without any agent code. All commands run over RCON, so this
requires `rcon.protocol` ≠ `none`.

```yaml
capabilities:
  players:
    kick: "kick {{.Player}}{{if .Reason}} {{.Reason}}{{end}}"
    ban: "ban {{.Player}}{{if .Reason}} {{.Reason}}{{end}}"
    unban: "pardon {{.Player}}"
    banList:
      command: "banlist players"
      entryRegex: '^\s*(?P<name>\w+)\s+was banned by\s+(?P<source>[^:]+?)(?::\s*(?P<reason>.*))?\s*$'
    whitelist:                                 # optional allow-list management
      list: "whitelist list"
      add: "whitelist add {{.Player}}"
      remove: "whitelist remove {{.Player}}"
      listRegex: 'whitelisted player[s()]*:\s*(?P<names>.+)$'
  quiesce:
    quiesce: ["save-off", "save-all flush"]   # run before a backup snapshot
    unquiesce: ["save-on"]                    # run after
    failurePattern: "saving failed"           # output regex that fails the step
  lifecycle:
    stop: ["stop"]                            # graceful-stop console sequence
```

- Moderation commands are Go `text/template`s rendered with `.Player`
  and `.Reason` (reason may be empty — guard with `{{if .Reason}}`).
  Unset actions are reported as unsupported and the UI hides them.
- `banList.entryRegex` matches one banned player per output line via
  the named groups `name` (required), `source` and `reason`.
- `whitelist` adds allow-list management to the Players tab: `add` /
  `remove` are `.Player` templates and `listRegex` extracts the
  comma-separated tail of the `list` command's output via the named
  group `names`.
- The quiesce sequence runs in order; any command error — or output
  matching `failurePattern` (case-insensitive) — aborts the backup and
  best-effort runs `unquiesce` so the game is never left paused.
  Games that can't quiesce simply omit the block; backups proceed
  without pausing.
- `lifecycle.stop` runs before the pod is scaled down (stop button,
  restarts): the operator issues the sequence over RCON and waits for
  the server to exit on its own, so a SIGTERM never interrupts an
  in-progress world save (`["Save", "Shutdown 1"]` on Palworld,
  `["stop"]` on Minecraft). Omit it for games that save on SIGTERM.

> The agent has **no per-game special-casing** — every capability above
> (and the two below) comes from this block. A template that declares
> nothing reports those features unsupported and the UI hides them.

#### Actions

`capabilities.actions` declares named operator buttons on the server
detail page. Each runs a console command built from a Go `text/template`
rendered with the parameters the user fills in, sent over RCON.

```yaml
capabilities:
  actions:
    - id: broadcast                 # stable, unique within the template
      displayName: Broadcast message
      description: Send a chat message to everyone.   # shown in the dialog
      icon: megaphone               # optional lucide-react icon name
      command: "say {{.Params.message}}"
      params:
        - name: message             # referenced as {{.Params.message}}
          displayName: Message
          type: string              # string | int | bool | enum
          required: true
    - id: set-weather
      displayName: Set weather
      command: "weather {{.Params.weather}}"
      confirm: false                # require a confirm step in the UI
      danger: false                 # red styling for destructive actions
      params:
        - name: weather
          type: enum
          enum: ["clear", "rain", "thunder"]
          default: clear
    - id: save-world
      displayName: Save world
      command: "save-all"           # no params is fine
```

- Parameter values are validated by `type` and sanitized before
  rendering: CR/LF and other control characters are rejected so a value
  can never chain a second RCON command. `int`/`bool`/`enum` values must
  parse/match; missing optional params fall back to `default`.
- A command template that fails to parse disables only that one action
  (logged), never the whole panel.

#### Status

`capabilities.status.metrics` declares live readouts on the Overview
tab. Each runs an RCON command and extracts a value via a named-group
regex (the group must be `value`).

```yaml
capabilities:
  status:
    metrics:
      - id: seed
        displayName: World seed
        command: "seed"
        regex: 'Seed: \[(?P<value>-?\d+)\]'
      - id: difficulty
        displayName: Difficulty
        command: "difficulty"
        regex: 'The difficulty is (?P<value>\w+)'
        unit: ""                    # optional suffix, e.g. "ms", "TPS"
```

Values are cached for a few seconds. A metric whose command errors or
doesn't match this cycle is shown with an empty value rather than
dropped, so the panel layout stays stable. When the game has no RCON the
endpoint returns nothing and the dashboard omits the panel.

#### Mods

`capabilities.mods` declares where this game's mods/plugins live and how
the dashboard may install new ones. Mods are plain files (or, for
extract-mode loaders, folders) on a volume; the dashboard lists,
installs, uploads, updates, and removes them generically by calling the
agent — no RCON required.

Two layouts, mutually exclusive:

```yaml
capabilities:
  mods:
    # Per-loader map (use with spec.versions): the active version's
    # `loader` selects the directory, and every (version + loader) combo
    # gets its OWN PVC — switching versions never clobbers another
    # combo's mod set. Max 16 keys.
    loaders:
      paper:  { displayName: Plugins (Paper), path: plugins, extensions: [".jar"] }
      fabric: { displayName: Mods (Fabric),   path: mods,    extensions: [".jar"] }
      bepinex:
        path: bepinex/plugins
        extensions: [".zip"]
        extract: true               # unpack archives into per-mod folders
    install:
      allowedHosts:                 # SSRF allowlist (required for URL installs)
        - cdn.modrinth.com
        - ".curseforge.com"         # leading dot → host + subdomains
      maxSizeMB: 512                # default 256, max 4096
    registry:                       # optional in-app browse — see below
      providers:
        - provider: modrinth
```

```yaml
capabilities:
  mods:
    path: mods                      # legacy single directory, relative to
    extensions: [".jar"]            #   storage.mountPath — for games with
    install: { ... }                #   no version/loader dimension
```

- **Listing/removal** work with or without an `install` block. A version
  entry whose `loader` has no map key (e.g. vanilla) gets no Mods tab at
  all.
- **URL install** downloads a user-supplied URL into the active
  directory. It is refused unless the URL's host matches `allowedHosts`
  (exact host or a `.suffix` for domain + subdomains) *and* the resolved
  address is publicly routable — the agent blocks loopback, private
  (RFC1918/ULA), link-local, and metadata addresses so it can't be
  tricked into fetching cluster-internal services. Downloads are
  size-capped and redirects are re-checked against the allowlist.
- **Upload** (the install page's third mode) sends a local file to the
  agent as multipart. Same filename/extension/size checks; works even
  without an `install` block, since an upload carries no SSRF risk —
  handy for locally built mods (e.g. `.pak` files on Palworld).
- **Extract mode** (`extract: true`) unpacks downloaded/uploaded `.zip`
  archives into a per-mod folder (BepInEx-style layouts); listing and
  removal then operate on folders. Zip-slip and total-size are guarded.
- Filenames are sanitized against path traversal; each `path` must be
  relative with no `..`.

**Install manifest.** Every mod volume carries a hidden
`.gameplane-mods.json` ledger the agent maintains: registry installs
record their provider/project/version, uploads record provider
`upload`, and files placed outside the panel show as *unmanaged*. This
powers the Mods tab's provenance badges, the batch update check
(`GET /servers/{name}/mods/updates`), and one-click in-place upgrades —
module authors get all of it for free; there is nothing to declare.

##### Registry browse (`capabilities.mods.registry`)

Declaring `registry.providers` turns the install page's browse mode on:
the dashboard searches the registry filtered to the active version's
`loader` + `gameVersion` and one-click installs the chosen file through
the same allowlisted download path (so keep the registry's CDN hosts in
`allowedHosts`).

```yaml
registry:
  providers:
    - provider: modrinth            # Minecraft mods/plugins (keyless)
      modpacks:                     # optional Modpacks tab — see below
        refEnv: MODRINTH_MODPACK
        env: [{ name: TYPE, value: MODRINTH }]
    - provider: curseforge          # Minecraft; needs an admin API key
    - provider: hangar              # PaperMC plugins (keyless)
    - provider: thunderstore        # BepInEx games (keyless)
      community: valheim            # required: the community slug
      modpacks: {}                  # deps-mode packs (no refEnv)
    - provider: factorio            # official Factorio mod portal
```

- Up to 8 providers; the first is the dashboard's default and a switch
  appears when there's more than one. A provider whose engine needs
  unmet server config (CurseForge without `--curseforge-api-key`) is
  hidden until configured.
- `thunderstore` requires `community`; the others ignore it.
- **`factorio` is browse-only**: the portal's downloads require the
  player's own factorio.com credentials, which Gameplane never stores —
  files are flagged `requiresAuth` and the dashboard hands the user to
  the from-URL form to append `?username=…&token=…` themselves. Keep
  `mods.factorio.com` / `.factorio.com` in `allowedHosts`.

##### Mod-portal credentials (Factorio)

Registry providers can declare a `credentialsSecretRef` to inject
authentication into downloads. Currently used by Factorio:

```yaml
registry:
  providers:
    - provider: factorio
      credentialsSecretRef: { name: factorio-creds }
```

The Secret must live in the GameServer's namespace and contain `username`
and `token` keys:

```yaml
apiVersion: v1
kind: Secret
metadata: { name: factorio-creds, namespace: gameplane-games }
type: Opaque
data:
  username: <base64-encoded-username>
  token: <base64-encoded-api-token>
```

The agent mounts the Secret read-only at `/etc/gameplane/mod-creds/factorio/`
and transparently appends `username` and `token` query parameters to download
URLs during install. Missing or unreadable credential files are handled
gracefully (installs proceed without credentials). The credentials are never
logged or included in error messages.

##### Modpacks (`providers[].modpacks`)

Declaring `modpacks` on a provider adds a Modpacks tab. Two install
modes, chosen by `refEnv`:

- **env-mode** (`refEnv` set): installing a pack pins its slug into the
  named env on the GameServer (plus any fixed `env` listed) and the
  server restarts — the game image installs the pack itself on boot
  (Modrinth packs on the itzg image). One pack active per server.
- **deps-mode** (`modpacks: {}`): the pack is resolved into its
  dependency mods, which install one-by-one through the normal install
  path (Thunderstore/BepInEx packs).

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
apiVersion: gameplane.local/v1alpha1
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
'{"spec":{"version":"<v>"}}'`). Keep `template.yaml`'s `spec.version` in
lockstep with the bundle version — the official modules do, and drift
between the two confuses pinning.

Worth exercising manually for modules that declare `spec.versions` and
per-loader mods:

```sh
# switch the running server to another catalog entry (Settings → Version
# does the same) and watch the StatefulSet re-render
kubectl patch gameserver <gs> -n gameplane-games --type merge \
  -p '{"spec":{"version":"<other-id>"}}'

# each (version+loader) combo gets its own PVC; the previous combo's
# volume must survive the switch
kubectl get pvc -n gameplane-games | grep <gs>-mods-
```

From the dashboard, also try the Mods tab end to end: install one from
the registry (badge should show provider + version), **Check updates**,
upload a local file (badge `upload`), and remove it.
