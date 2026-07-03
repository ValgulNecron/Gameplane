# Changelog

All notable changes to Gameplane are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims
to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html) once it
reaches `1.0.0`. Pre-1.0 minor versions may contain breaking changes.

## [Unreleased]

### Added

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
