# Changelog

All notable changes to Gameplane are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims
to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html) once it
reaches `1.0.0`. Pre-1.0 minor versions may contain breaking changes.

## [Unreleased]

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

[Unreleased]: https://github.com/ValgulNecron/gameplane/compare/v0.2.0-beta.3...HEAD
[0.2.0-beta.3]: https://github.com/ValgulNecron/gameplane/compare/v0.2.0-beta.2...v0.2.0-beta.3
[0.2.0-beta.2]: https://github.com/ValgulNecron/gameplane/compare/v0.2.0-beta.1...v0.2.0-beta.2
[0.2.0-beta.1]: https://github.com/ValgulNecron/gameplane/releases/tag/v0.2.0-beta.1
