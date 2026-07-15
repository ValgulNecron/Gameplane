# Changelog

All notable changes to Gameplane are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims
to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html) once it
reaches `1.0.0`. Pre-1.0 minor versions may contain breaking changes.

## [Unreleased]

### Changed

- **modules:** official Factorio template now uses RCON (via the operator's
  `rcon.passwordFile` pointed at the image's self-generated `config/rconpw`)
  instead of pty-only; Factorio and Palworld templates gained a `players.list`
  capability (version 1.1.0).
- **api/web/chart:** the Admin Settings → Updates **channel selector is now a
  read-only label**. The old select persisted a channel that nothing consumed
  (Gameplane is upgraded via Helm, not an in-app updater); the channel is now
  the chart's informational `updates.channel` value, passed to the API as
  `--update-channel` and served on `GET /cluster/info` as `updateChannel`.
  `PUT /admin/config/updates` is gone (400 unknown section) and any legacy
  `updates` DB row is ignored.
- **operator:** module-declared stop sequences (`capabilities.lifecycle.stop`)
  now run over telnet RCON and over pod-attach to stdin (for `consoleMode:
  pty` games with no RCON), in addition to Source RCON. **CRD change:**
  `GameTemplate.spec.rcon.protocol` first dropped the never-implemented `http`
  value, then re-widened as real clients landed — it is now
  `source;telnet;websocket;battleye;satisfactory;none` (see Added).
- **modules:** categories are now plural — declare `categories:` as a list
  (a module appears in every category's filter chip), with legacy `category:`
  accepted and normalized. All 16 official modules declare categories explicitly
  instead of the game-slug regex fallback.

### Added

- **agent:** three new remote-console protocols behind the agent's existing
  `Exec` interface, so console / players / quiesce / lifecycle / actions all
  work over them: `websocket` (Rust's Facepunch WebRcon), `battleye` (BattlEye
  RCon — DayZ), and `satisfactory` (Satisfactory's HTTPS function API). The
  `rcon.protocol` enum is now `source;telnet;websocket;battleye;satisfactory;none`.
  Modules adopted them: Rust, DayZ, and Satisfactory each gain a console (plus
  a players list where the protocol exposes one). Satisfactory's admin password
  isn't a server-config value the operator can inject — it needs a one-time
  in-game claim, documented in the module.
- **actions:** quick actions gain `commands` (a sequence run in order,
  mutually exclusive with `command`), `transport` (`rcon` | `stdin`), and
  `group` (labelled sections on the server detail page). Stdin actions run
  over pod-attach from the API, so a `consoleMode: pty` game (Terraria, Don't
  Starve Together) can carry actions — fire-and-forget, the dashboard shows
  "sent". Param validation (the console-injection guard) moved into a shared
  `gameaction` module imported by both the agent and the API, so neither
  transport is trusted blind.
- **mcp-server:** new optional component — a strictly read-only Model
  Context Protocol (MCP) server for Gameplane clusters. Lets an AI assistant
  list/get the 7 Gameplane CRDs, Pods, and Events, fetch pod logs, and get a
  `propose_fix` suggestion (YAML/kubectl as text) — never create, update,
  patch, delete, or apply. Speaks MCP (JSON-RPC 2.0) over stdio only, no
  network port; off by default via `mcpServer.enabled`. See
  `mcp-server/README.md`.
- **api:** native S3 audit sink — batched NDJSON objects to any S3-compatible
  endpoint (`api.audit.s3.*`), alongside the existing stdout/webhook/syslog
  sinks. Events are buffered and flushed when ANY of: 100 events, 1 MiB, or 5
  seconds elapsed. Works with AWS S3, MinIO, Backblaze, Wasabi, etc. Retries 3
  times on transient errors; watch `gameplane_audit_s3_events_total` for
  delivery health.
- **operator:** GameTemplate `rcon.passwordFile` — point the console agent at
  a game-managed password file inside the data volume (e.g., Factorio's
  `config/rconpw`). Precedence: `passwordSecretRef` > `passwordFile` >
  operator-generated Secret. No Secret is created or injected when using
  `passwordFile` mode; the agent reads the file on every connection.
- **api/web/rbac:** per-GameServer access control — server owners and
  collaborators get enforced per-server access on top of namespace role
  bindings. Collaborators are managed from the new Access section (server
  settings) and shared servers appear under "Shared with you"; destructive
  actions (delete, wipe, transfer, collaborator management) stay owner-only.
  (#77)
- **operator/modules:** configurable players-list command — GameTemplate
  capabilities can set the RCON command and an optional per-line entry regex
  for reading the online player list; both the Players tab and the heartbeat
  player counts honor it. (#76)
- **agent/mods:** Factorio mod-portal credentials — mod registry providers
  accept a `credentialsSecretRef` (Secret with `username`/`token` in the
  GameServer's namespace); the agent injects credentials for
  mods.factorio.com downloads, with URL redaction keeping the token out of
  logs. (#78)
- **observability:** log levels are now configurable everywhere — the API and
  agent accept `--log-level`/`GAMEPLANE_LOG_LEVEL` (debug|info|warn|error;
  unknown values degrade to info), the chart wires `api.logLevel` and the
  previously-dead `operator.logLevel` (zap: debug|info|error), and a new
  `operator.agentLogLevel` injects the level into agent sidecars (left empty
  it injects nothing, so upgrades don't roll every game pod).
- **auth:** OIDC providers can now map IdP groups to dashboard roles —
  per-provider extra scopes, a configurable groups claim, group→role
  mappings with most-privileged-wins, a default role (including `deny` to
  refuse unmatched logins), and role re-sync on every login when mappings
  are configured (the IdP becomes the source of truth; providers without
  mappings keep today's viewer-then-promote behavior).
- **api/web:** new admin-only **System Logs** viewer — `GET
  /admin/system-logs/{component}` streams the operator or API pod's logs
  (tail + optional 50s follow window; fixed namespace and label selector
  only), and the new dashboard page (Admin → System logs) renders the
  stream in a scrollback panel with component/tail/follow controls, a pod
  badge, stick-to-bottom auto-scroll, and a download button.
- **operator/modules:** game memory settings can now **size themselves to
  the container memory limit** — a configSchema field may declare
  `autoFromMemoryLimit: {percent}`, and when the user leaves it empty the
  operator computes percent% of the server's effective memory limit
  (e.g. 8Gi × 75% → `6144M`), re-tracking the limit whenever the server
  is resized. The minecraft-java module (2.7.0) adopts it for
  `MAX_MEMORY`/`INIT_MEMORY`: JVM heaps now follow the wizard's memory
  slider instead of a static `2G` default, ending both the OOM kills
  from heaps set at/above the limit and the RAM wasted below it.
  Explicit values still win, and the bundle's `gameplaneMinVersion`
  keeps older operators from silently ignoring the field.
- **telemetry:** a home for the anonymous-usage reports — the new standalone
  **`telemetry-receiver`** component (own Go module + image, like the
  audit-syslog bridge) accepts the API's daily `{version, servers,
  templates}` POST on `/ingest`, logs it structurally, and exposes
  aggregate Prometheus metrics (`gameplane_telemetry_reports_total` by
  version, fleet-size histograms). The chart can deploy it with
  `api.telemetry.receiver.enabled=true` (the API is auto-pointed at it),
  or aim the API at an external receiver via `api.telemetry.endpoint`;
  an optional shared ingest token (`api.telemetry.authSecretRef`) guards
  `/ingest` and rides the API's new `GAMEPLANE_TELEMETRY_AUTH` env.
  Previously the Admin Settings toggle worked but there was nowhere for
  the data to go.
- **auth:** identity providers are now **managed from the dashboard and
  applied live** — Admin Settings → Authentication gained an "Add
  provider" form (generic OIDC, Google preset, GitHub-via-bridge preset):
  issuer + client id go in the auth config, the client secret lands in an
  API-managed Secret (`gameplane-auth-<name>`), and the new
  `/auth/oidc/{provider}/start|callback` routes resolve providers through
  a registry that re-reads the config per request — a saved provider
  works without an API restart, and several OIDC providers can coexist.
  The login page renders one button per enabled provider and hides the
  password form when local login is disabled; disabling local now
  actually gates `/auth/login` (neutral 403). Helm-flag OIDC keeps
  working untouched as the read-only `helm` provider on its legacy
  routes (and its callback now links accounts correctly — the store was
  never attached before, so every Helm-flag OIDC login 500'd). Break-
  glass for a lockout: `bootstrap-admin --enable-local-login`.

- **e2e:** a **Terraria protocol bot** joins the Minecraft one — the opt-in
  bot bucket now also boots a real Terraria server (the same
  passivelemon/terraria-docker image the shipped module uses), completes
  the ConnectRequest → ContinueConnecting handshake with a bespoke minimal
  protocol client (`test/e2e/internal/terrabot`, self-correcting on
  version-mismatch kicks), and asserts the server answers a world-data
  request — proving the module template boots a genuinely joinable server.
  Valheim/Palworld (steamcmd, proprietary UDP) and Factorio (UDP-only, no
  control channel) remain out of scope for CI bots.
- **notifications:** a new **ntfy** sink kind — POSTs each event to an ntfy
  topic URL (ntfy.sh or self-hosted) with the headline in the `Title`
  header, `Priority: high` on failures, and an optional access token.
- **notifications:** the **Add sink form now takes the credential value
  directly** (webhook URL, ntfy topic + token, SMTP settings) instead of
  requiring a pre-created labelled Secret referenced by name. The API
  stores the value as a Secret named `gameplane-notify-<sink>` (labelled
  `gameplane.local/notification-sink=true` +
  `gameplane.local/managed-by=gameplane-api`) and wires the sink's
  `configRef` automatically; deleting the sink cleans the Secret up.
  Secrets created with kubectl/GitOps keep working and are never deleted
  through the dashboard. The cryptic "Name (DNS label)" help text is gone
  too.

### Fixed

- **web:** file browser (list/read/write/upload/delete) now works for servers
  shared from a non-default namespace — the namespace query param was being
  inserted mid-URL, producing 400s and a silently-empty file tree.
- **web:** servers shared from other namespaces are now **navigable** —
  the detail route and namespace-aware API calls (lifecycle, console, files,
  players, and mods) carry the server's namespace through search params and
  query strings, while backups and schedules remain cluster-scoped. Player
  counts no longer render `"-1 online"` when the maximum is unknown (agent
  metric unavailable); they show just the online count instead.
- **api/web:** the Cluster page no longer presents **"Add node" and
  "Download kubeconfig" as click-to-error dead-ends** on installs where
  cluster operations are off (the default). `GET /cluster/info` now reports
  the `clusterOps` flag, and the dashboard disables both buttons with a hint
  pointing at `clusterOps.enabled` in the Helm values.
- **api/web:** the Authentication admin section can no longer save a config
  with **zero enabled identity providers** — a state that would have locked
  everyone out at their next logout. The API rejects such saves (422,
  "at least one identity provider must stay enabled") and the dashboard
  disables the toggle on the last enabled provider with an explanatory hint.
- **web:** the Create Server wizard no longer produces **unschedulable
  servers**. It set only resource *limits*, so Kubernetes defaulted
  requests to match — a 4-core limit then needed a fully-empty node and sat
  `Pending` forever on any partially-used cluster. The wizard now caps CPU /
  memory at the largest single node's capacity (shown as a hint, enforced on
  the field and the step), and sets an explicit modest CPU **request** below
  the limit (memory is still guaranteed at the limit), so a new server
  schedules onto a node with room. Defaults lowered to 2 cores / 4 GiB /
  20 GiB.
- **web:** raw byte counts in Kubernetes event messages (e.g. "Image size:
  333546371 bytes") are now shown human-readable ("318 MB") on the server
  Overview.

### Added

- **mods:** direct **file upload** — a third "Upload file" mode on the Mods
  install page sends a local mod straight to the server (multipart to the
  agent, drop zone + progress in the dialog). The agent applies the same
  name/extension/size checks as URL installs and unpacks archives for
  extract-mode loaders; uploads are recorded as provider `upload` in the
  install manifest and are never update-checked. Because an upload carries
  no SSRF risk, it works even for modules without a download allowlist
  (e.g. installing locally built `.pak` files on a Palworld server).

- **web:** servers can now **switch game version after creation** — a new
  "Version" section in Server Settings (shown when the template declares a
  version catalog) offers the same version+loader picker as the Create
  wizard, with a default badge, a callout explaining that each loader keeps
  its own preserved mod volume, and an image-override warning with one-click
  clear. Saving restarts the server on the new version; the operator side
  (image/env swap, per-loader mod-PVC switching, unknown-id failure) was
  already fully wired and is now also covered by an end-to-end
  version-switch test against a live cluster.
- **web:** the Mods tab understands mod provenance — managed mods show a
  provider + version badge (from the agent's install manifest) while files
  placed outside the panel read "unmanaged"; a **Check updates** button runs
  the batch update check and surfaces per-mod "x.y.z available" pills with
  one-click **Update** (in-place upgrade via `replaces`) and **Update all**;
  registry installs now record their provenance so new installs are managed
  from day one; and credential-gated registry files (Factorio portal) hand
  off to the From-URL form prefilled instead of a one-click install that
  would fail.

- **agent/mods:** installed mods now carry an **install manifest**. Each mod
  volume keeps a hidden `.gameplane-mods.json` ledger recording where every
  panel-installed mod came from (registry provider, project, version, source
  URL, install time); `GET /mods` merges it into listings (`meta` per mod,
  `null` for files placed out-of-band), and `POST /mods/install` accepts
  optional `meta` (registry identity to record) and `replaces` (an existing
  mod to swap out atomically — the new file lands first, so a crash can only
  ever leave a duplicate, not a missing mod). A corrupt manifest degrades to
  "unmanaged" listings and self-heals on the next install/remove. This is the
  foundation for update detection and one-click mod upgrades in the dashboard.
- **api/mods:** batch **mod update detection** — `GET
  /servers/{name}/mods/updates` reads the server's install manifest from the
  agent and checks every managed mod against its registry provider (latest
  release compatible with the active loader + game version), returning the
  update list plus per-mod errors in one call. Results are TTL-cached and
  upstream lookups are concurrency-bounded, so page revisits don't hammer
  Modrinth & co. Upgrades ride the existing install endpoint's `replaces`
  field. A new `api-mods` e2e bucket proves the manifest round-trip
  (install with metadata → listed with metadata → in-place upgrade →
  remove) through the full API → agent → volume stack.
- **registry:** a fifth mod-registry engine — the official **Factorio mod
  portal** (`mods.factorio.com`). Templates can declare
  `capabilities.mods.registry.providers: [{provider: factorio}]` to browse
  and search the portal (keyless, catalog cached with a TTL, filtered by the
  active version's `gameVersion` major token). Portal downloads require the
  player's own factorio.com credentials, which the server must never embed
  in URLs it returns to browsers — files are flagged `requiresAuth: true`
  and the dashboard hands off to the from-URL install form instead of
  one-click install. Groundwork for the upcoming official Factorio module.

- **notifications:** the sinks configured under Admin Settings →
  Notifications now actually **deliver**. The API watches GameServer /
  Backup / Restore status transitions and pushes matching events —
  `server.unhealthy`, `server.recovered`, `backup.failed`,
  `backup.succeeded`, `restore.failed`, `restore.succeeded` — to Discord,
  Slack, SMTP, or generic-webhook sinks (previously the panel persisted
  sinks that nothing read). Sinks grew per-event filters (failures on by
  default) and a `configRef` pointing at a labelled Secret
  (`gameplane.local/notification-sink=true`) holding the webhook URL / SMTP
  credentials; a new `POST /admin/notifications/sinks/{name}/test` endpoint
  test-fires a sink synchronously. Outbound dials are SSRF-guarded
  (`netguard.IsAllowed`), delivery is best-effort with bounded retry, and
  outcomes are counted at `/metrics` as `gameplane_notify_deliveries_total`.
  See [`docs/notifications.md`](docs/notifications.md).

## [0.2.0-beta.5] — 2026-07-03

### Added

- **ci/supply-chain:** the cosign **public key is now published** — committed
  at the repo root as [`cosign.pub`](cosign.pub), exported by CI (job summary +
  artifact on every edge publish), and shipped as a release asset. Both publish
  workflows now **verify each signature right after signing** with the key
  derived from the CI secret, and fail the publish if the committed
  `cosign.pub` drifts from the signing key. README and docs gained
  `cosign verify` instructions (the key is Ed25519; offline/keyed, so
  verification takes `--insecure-ignore-tlog=true`).
- **ci:** tagged releases now get a **GitHub Release page automatically** —
  notes extracted from the tag's CHANGELOG section, prereleases flagged,
  `cosign.pub` attached, idempotent on re-runs. (beta.4's page was backfilled
  by hand.)
- **website:** Gameplane has a public marketing + docs site at
  <https://valgulnecron.github.io/gameplane-website/> — features, game
  showcase, an AMP comparison, and the full documentation set, designed
  screen-first in `design.pen` (18 frames) and built with Astro. The source
  lives in the separate
  [`gameplane-website`](https://github.com/ValgulNecron/gameplane-website)
  repo, mounted here as the `website/` submodule.
- **web:** the last UI-audit parity items — a show/hide toggle on the login
  password field, user emails shown under display names on the Users page,
  port overrides editable in the Create Server wizard's Network step (the CRD
  and Settings tab already supported them), and "Back up now" relocated to a
  header button + dialog on the Backups page, matching the design.

### Changed

- **ci:** the three kind e2e jobs (core / extended / game bot) were collapsed
  into a single job running the same three suites, in the same order, against
  one cluster — they were already fully serialized by the shared concurrency
  group, so the split only cost duplicate image builds, cluster setups, and
  queue slots (~15-20 min per run). Docs-only changes no longer trigger the
  test pipeline at all (`paths-ignore`); no test was removed.

## [0.2.0-beta.4] — 2026-07-01

### Added

- **operator:** fleet metric `gameplane_gameservers{phase=…}` exported on the
  operator's `/metrics` endpoint — one gauge per GameServer lifecycle phase,
  computed from the controller cache at scrape time (so it never drifts from
  reconcile state). The bundled Grafana dashboard gains "GameServers
  running/failed" stats and a stacked "GameServers by phase" graph, and
  `prometheusRules.enabled` gains a `GameplaneGameServerFailed` alert. Until
  now the dashboard and alerts plotted only controller-runtime internals; this
  is the first domain metric.
- **operator:** companion fleet metric `gameplane_backups{phase=…}` (one gauge
  per Backup phase), with a "Backups failed" stat + stacked "Backups by phase"
  graph on the dashboard and a `GameplaneBackupFailed` alert — a silently failed
  backup is a data-loss risk, so it's worth paging on.
- **ci:** the published component images (`:edge` on every `main` push and the
  versioned images on a `v*` release) are now keyed-cosign-signed by digest
  (offline, no Rekor), matching the official module bundles. Verify with
  `cosign verify --key cosign.pub ghcr.io/valgulnecron/gameplane/<component>`.
  Gated on `COSIGN_PRIVATE_KEY`, so publishing still works before the key is set.
  Signing and publishing run only on `main`/tags, never on PRs.
- **api:** server-side audit-log export — `GET /admin/audit/export?format=csv|json`
  (admin-only) streams the **entire** audit trail, optionally bounded by RFC3339
  `since`/`until`. Unlike the dashboard's client-side CSV (only the loaded page),
  this returns the full log in one download for compliance/archival, streamed
  row-by-row so a large table isn't buffered in memory.
- **api:** outbound **audit-log webhook push sink** — set `api.audit.webhook.url`
  and the API POSTs each audit event as JSON to that endpoint (a log aggregator's
  push API, a SIEM, or your own receiver), alongside the database. It mirrors,
  never gates: events always persist, and a slow or unreachable endpoint never
  blocks or fails a request (bounded async buffer, drop-and-count on overflow,
  surfaced at `gameplane_audit_webhook_events_total{result=sent|failed|dropped}`
  on `/metrics`). An optional `Authorization` header is Secret-sourced
  (`api.audit.webhook.authSecretRef`), never a flag.
- **audit-syslog-bridge:** a new, generic HTTP-JSON → **syslog** relay image that
  sits behind the audit webhook sink, so audit events can be forwarded to a
  syslog/SIEM collector (RFC 5424 over TCP, TCP+TLS, or UDP). Deploy the bundled
  relay with `api.audit.webhook.syslogBridge.enabled=true` +
  `…syslogBridge.syslog.addr`, and the chart auto-points the webhook at it (off
  by default). The bridge forwards the request body verbatim, so it works for
  any JSON webhook source, not just Gameplane.

### Changed

- **chart:** the bundled `defaultModuleSource` now defaults to a **git** source
  that indexes the public `gameplane-module` repository, so the official games
  appear in the Modules page out of the box — no OCI registry to publish first.
  Set `defaultModuleSource.type: oci` to pull (optionally cosign-signed) bundles
  from a registry instead. **Breaking values change:** the former top-level
  `url` / `insecure` / `modules` / `pullSecretName` / `verify` keys moved under
  `defaultModuleSource.oci.*`.
- **chart:** the official module-signing public key (ed25519) now ships in
  `defaultModuleSource.oci.verify.cosignPublicKey`, so verifying official bundles
  is just `type: oci` + `oci.verify.enabled: true` — no key to paste. Still off
  by default.

### Fixed

- **operator:** the Servers list "Node" column was always blank — nothing ever
  set the `gameplane.local/node` annotation the dashboard reads. The GameServer
  reconciler now keeps it in sync with the node the game pod is scheduled on
  (and clears it when the server is stopped), so the column shows real placement.

## [0.2.0-beta.3] — 2026-06-27

### Added

- **chart:** bundled observability — `prometheusRules.enabled` ships a
  `PrometheusRule` of operator alerts (reconcile errors, workqueue backlog,
  stuck reconcile) on the standard controller-runtime metrics, and
  `grafanaDashboards.enabled` ships a Grafana dashboard ConfigMap (reconcile
  rate/errors/latency, workqueue depth) for the Grafana sidecar to auto-import.
  Both off by default, alongside the existing `serviceMonitors`.
- **Signed module bundles.** `modules/build.sh` gains an opt-in `--sign` flag
  that keyed-cosign-signs each pushed bundle by digest (offline; no
  transparency-log upload), and the `release.yaml` workflow gains a `modules`
  job that pushes and signs the official `modules/*` bundles on every `v*` tag.
  The job is gated on a `COSIGN_PRIVATE_KEY` secret, so releases still succeed
  before a signing key is provisioned.
- **chart:** `defaultModuleSource.verify` (`enabled` / `cosignPublicKey`, off by
  default) wires the published signing key into a `gameplane-module-cosign-pub`
  Secret and sets `spec.verify.key` on the default source, so installs can
  refuse any unsigned or wrong-key bundle. See
  [`docs/module-authoring.md`](docs/module-authoring.md#signing-official-bundles).

### Changed

- **BREAKING — CRD API group renamed `gameplane.gg` → `gameplane.local`.** Drops
  the `.gg` TLD; all seven CRDs, their labels/annotations (`gameplane.local/*`),
  and RBAC now use the `gameplane.local` group, and manifests are
  `apiVersion: gameplane.local/v1alpha1`. (A CRD group must be a dotted DNS
  subdomain, so a bare `gameplane` is not valid.) No tagged release ever shipped
  the `.gg` group, so this only affects unreleased/edge installs. Recreate CRDs
  after upgrading (`kubectl apply -f charts/gameplane/crds/`) — existing
  `*.gameplane.gg` custom resources are not migrated automatically.
- **Modules moved to their own repo.** The `modules/` tree (the official game
  template bundles + `build.sh`) now lives in the standalone `gameplane-module`
  repository, vendored back here as a git submodule at the same path. Nothing
  changes at runtime — the operator still pulls bundles from an OCI registry —
  but after cloning you must run `git submodule update --init` (or clone with
  `--recurse-submodules`) before `make dev-up` / `make modules-push`.

## [0.2.0-beta.2] — 2026-06-22

### Changed

- **Renamed the project from Kestrel to Gameplane.** This is a breaking rebrand
  that spans every layer:
  - **CRD API group** `kestrel.gg` → `gameplane.gg` (all seven CRDs). Existing
    clusters must be **recreated** — Helm never upgrades CRDs across a group
    change, and there is no in-place migration for `v1alpha1` objects.
  - **OCI module bundles** — media types `application/vnd.kestrel.*` →
    `application/vnd.gameplane.*` and the manifest field `kestrelMinVersion` →
    `gameplaneMinVersion`; existing bundles must be re-pushed
    (`make modules-push`).
  - **Helm chart** `kestrel` → `gameplane`, default namespaces
    `kestrel-system` / `kestrel-games` → `gameplane-system` /
    `gameplane-games`, the image registry, and every `kestrel-*` object name.
  - **Environment variables** `KESTREL_*` → `GAMEPLANE_*`.
  - **Auth cookies and CSRF header** (`kestrel_session`, `kestrel_csrf`,
    `X-Kestrel-CSRF`, …) → `gameplane_*` / `X-Gameplane-CSRF`; active sessions
    are invalidated on upgrade and users must re-login.
  - **Go module path** `github.com/kestrel-gg/kestrel` →
    `github.com/ValgulNecron/gameplane`.
- Synced `web/package.json` to the chart version (it had been left at `0.1.0`).

## [0.2.0-beta.1] — 2026-06-22

First **beta**. The control plane (operator, API, agent) and the dashboard are
feature-complete for the v1 scope and have been stabilized for external
testing. Not yet recommended for unattended production workloads — see
[Beta status & known limitations](README.md#beta-status--known-limitations).

### Highlights (the beta feature surface)

- **GameServers** — create / start / stop / restart / clone / wipe-data, with a
  guided 4-step creation wizard (template → configure → network → review),
  per-server resource limits, node placement, env vars, and a CIDR ingress
  allow-list. Provisioning sub-status surfaces image pull / install / waiting-
  for-agent / crash-loop reasons.
- **Console, logs, files, players** — RCON/PTY console with a command bar and
  resilient reconnect; pod + game-file log views with level filtering; a Monaco
  file editor; and player moderation (kick/ban/unban) plus whitelist management.
- **Backups & restore** — on-demand and scheduled backups (restic-snapshot and
  CSI volume-snapshot strategies), retention policies, and restore-as-new-server.
- **Modules** — game templates distributed as signed OCI bundles, an in-app
  registry browser (Modrinth / CurseForge / Hangar) for mods and modpacks, and
  SSRF-guarded fetching with optional cosign signature verification.
- **RBAC & audit** — admin / operator / viewer roles with a permission catalog,
  local (argon2id) and OIDC login, and a human-readable audit log.
- **Cluster** — node overview with cordon / drain and kubeconfig download.

### Fixed (stabilization for beta)

- **operator:** surface `BackupSchedule` retention-trim failures as a
  `RetentionTrimmed` status condition instead of swallowing them, and persist
  status changes even on dormant/suspended reconciles.
- **operator:** clear a `Module`'s `Pulling` condition on failure so a failed
  module no longer shows "Pulling" and "Failed" at once.
- **api:** `:restart` now reliably recycles the pod — it waits for the
  StatefulSet to drain before resuming, instead of racing the operator's
  reconcile (which left the pod untouched under load).
- **web:** fixed a login password-field accessibility bug (a `<label>` wrapping
  the "Forgot?" button mis-bound the field), a registry browser that collapsed
  expanded results on every search keystroke, and a stale audit-log assertion.
- **web:** added the design's "Audit log" quick-link to the Users header.
- **ci/test:** made the e2e `kubectl port-forward` helper resilient to a loaded
  runner (retry + longer readiness window), eliminating a cascade of flaky
  API e2e failures.

[Unreleased]: https://github.com/ValgulNecron/gameplane/compare/v0.2.0-beta.4...HEAD
[0.2.0-beta.4]: https://github.com/ValgulNecron/gameplane/compare/v0.2.0-beta.3...v0.2.0-beta.4
[0.2.0-beta.3]: https://github.com/ValgulNecron/gameplane/compare/v0.2.0-beta.2...v0.2.0-beta.3
[0.2.0-beta.2]: https://github.com/ValgulNecron/gameplane/compare/v0.2.0-beta.1...v0.2.0-beta.2
[0.2.0-beta.1]: https://github.com/ValgulNecron/gameplane/releases/tag/v0.2.0-beta.1
