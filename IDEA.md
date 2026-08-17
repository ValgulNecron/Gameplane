# Ideas

Scratch list. Everything here has shipped — see `docs/roadmap.md` for what
remains before a v1 GA.

---

## Shipped (removed from this list)

- Public website mobile pass — the beta.7 refresh had already made most pages
  (landing, features, games, showcase, docs shell with hamburger nav)
  mobile-correct; the one remaining gap was the comparison table (a 720px-wide
  grid that scrolled sideways on phones), now reflowed to stacked cards on
  mobile while keeping the 3-column table on `md:` (gameplane-website PR #3,
  submodule pointer bumped in Gameplane PR #176). The dashboard's own mobile
  pass shipped earlier (PR #106).
- CI is faster — the 9 heavy e2e jobs each rebuilt the same operator/api/agent
  (+gameprobe) images; now a single `build-images` job bakes them once and
  publishes a tar the jobs load (saves ~25–40 runner-minutes/run). A
  `dorny/paths-filter` `changes` gate skips the Go/kind suite on web-only PRs
  and the web jobs on Go-only PRs; the `setup-envtest` binary is cached (PR #174).
- Tests run on arm64 — the Go unit + envtest suite runs on both amd64 and
  `ubuntu-24.04-arm` every PR (arch-qualified caches), and the kind e2e suites
  (e2e-go × 6 buckets, multicluster, web-live) run on both arches too, fed by a
  per-arch build-once. game-bot stays amd64 (the external Terraria image has no
  confirmed arm64 build) (PR #175).
- Module quick-actions expanded — 33 doc-verified console actions added across 9
  modules (ark, cs2, dont-starve-together, factorio, minecraft-java,
  project-zomboid, rust, terraria + dayz's first BattlEye-RCON set) in
  gameplane-module PR #34; 20 new lucide icons wired into the dashboard
  `ServerActionsCard` iconMap and the submodule pointer bumped (PR #173).
  palworld/satisfactory/v-rising were left out — their agent RCON clients accept
  only fixed verb sets, not free-text commands.
- Module categories on the dashboard — the data model was already multi-valued
  (`Categories []string` everywhere); the Modules catalog now shows each
  module's categories as chips on its card and the category filter is
  multi-select (PR #172). Also retired the **mods support** item: all
  mainstream platforms (Modrinth, CurseForge, Thunderstore, Steam Workshop +
  hangar/factorio/spigot/github/umod/nexus) were already implemented with
  per-provider registry engines, and keyed providers (CurseForge/Steam/Nexus)
  already hide in the Mods browser until an API key is configured — only a
  stale `keys.go` comment needed fixing (PR #172).
- Three dashboard defects fixed in web (PR #171): the create-server wizard
  card now stays scrollable when tall (no more preview escaping the modal) and
  its review config preview is bounded; the CPU/RAM sliders are granular (100m
  CPU floor / 50m step, 256Mi RAM); and the Modpacks tab is gated on the active
  version's mod loader, so Minecraft Vanilla and Paper no longer show it.
- Module YAML editor schema — VS Code (redhat.vscode-yaml) was mis-matching
  `template.yaml` against the AWS SAM schema and flagging valid fields. Both
  module YAML files now carry a `# yaml-language-server: $schema=…` modeline
  pointing at JSON schemas under `modules/.schema/`: `gametemplate.schema.json`
  generated from the GameTemplate CRD (carries every enum/pattern, e.g. the
  rcon protocol list) via `make module-schema`, and a hand-written
  `module.schema.json` (module PR #33 + Gameplane PR #158).
- ARK mods bug — CurseForge browse was hardcoded to Minecraft's gameId (432),
  so ARK listed Minecraft mods; templates now declare their own
  `curseforgeGameID` (ARK 83374), required by CEL (Gameplane PR #156 + module
  PR #32). The ARK "mods by ID" tab was realigned to its `design.pen` frame
  (commit `67bf12a`). "No Steam selection" was **not** a bug: ARK dropped Steam
  Workshop in favor of CurseForge, so it correctly has no Steam provider — CS2
  is the only Workshop-capable module and it's already wired.

- CPU/RAM selection — one shared slider + manual numeric input + unit switch
  (cores/mCPU, KiB–TiB); the Create wizard and Settings tab now use the same
  control, unified on Guaranteed QoS (requests == limits) (PR #102).
- Mobile dashboard — responsive app-shell (off-canvas Radix drawer + hamburger),
  Servers rendered as cards, horizontally-scrollable data tables, stacking Files
  panes, and reflowing form grids + toolbars (PR #106). *(The website's own
  mobile pass is still open — see above.)*
- Audit log tamper evidence — `audit_events` hash chain (migration `005`) with
  a per-insert head anchor + retention checkpoint, admin-only
  `GET /admin/audit/verify`, and an honest threat-model section in
  `docs/security.md` (the external sinks are the real append-only record;
  HMAC-keyed chaining noted as future hardening) (PR #103).
- Read-only MCP server — new optional `mcp-server/` component (own `go.mod`,
  distroless image, Helm toggle), structurally read-only (unexported clients in
  `internal/kube`) with a get/list/watch-only ClusterRole; `propose_fix` returns
  text only (PR #105).
- Operator-side restart primitive — was **already implemented** before the sweep
  (`operator/internal/controller/gameserver_restart.go`).
- Module-defined quick actions — **already implemented**
  (`GameTemplateSpec.Capabilities.Actions` → `ServerActionsCard.tsx`).
- Gamelog stuck on "Connecting to the log stream…" — **already fixed** in commit
  `c2db2dd` (`streamFailed` + container-output fallback).
- `make dev-install` now applies CRDs before `helm upgrade` (PR #96).
- Air-gapped installs: `--config-init-image` / `--restic-image` operator flags
  and chart values; the wipe Job no longer hardcodes busybox (PR #96).
- A backup that can't yield a restic snapshot id now fails instead of reporting
  Succeeded, and releases the quiesced world first (PR #96).
- Module-defined categories with dynamic catalog filter chips (PR #97).
- The e2e game bot runs in-cluster and its CI job is blocking (PR #98). It found
  two real bugs the advisory job had hidden: a 17-character bot username
  (Minecraft caps it at 16) and a floating Terraria image tag that drifted past
  the bot's protocol.
- Docs brought back in step with the code, and `docs/roadmap.md` added (PR #99).
- Website games page: all five shipped modules, correct versions
  (gameplane-website PR #1).
