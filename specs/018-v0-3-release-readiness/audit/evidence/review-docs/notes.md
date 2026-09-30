# Review: docs/ + root docs

- **Date**: 2026-09-23
- **Reviewer tier**: sonnet (verified by opus)
- **Checked against**: product behaviour in code (api/cmd/main.go, operator/api/v1alpha1/*, charts/gameplane/values.yaml, web/src/router/tree.tsx, go.work)

## Scope reviewed

- Root: `README.md`, `CHANGELOG.md` (first ~120 lines in full, skimmed rest), `SECURITY_AUDIT.md`, `CLAUDE.md`
- `docs/agent-architecture.md`, `docs/architecture.md`, `docs/comparison-sources.md` (partial — row a/b/c/g sections read closely, rest skimmed), `docs/contributing.md`, `docs/dependencies.md` (opening + section headers; not every per-dependency paragraph), `docs/game-coverage.md`, `docs/install.md`, `docs/key-rotation.md`, `docs/module-authoring.md` (targeted sections: config schema, module count references), `docs/networking.md`, `docs/notifications.md`, `docs/oidc.md`, `docs/roadmap.md`, `docs/security.md`, `docs/tunnels.md`
- `design-export/json/*.json` (365 files) — extracted `name`/`type` fields for all frames and diffed the `Screen/*` list against `web/src/router/tree.tsx`
- Cross-checked against: `api/cmd/main.go`, `api/internal/handlers/{config,audit,ownership,notifications}.go`, `api/internal/rbac/catalog.go`, `api/internal/db/shares.go`, `api/internal/audit/audit.go`, `api/internal/auth/ratelimit.go`, `api/internal/notify/{deliver,events,format,notify}.go`, `api/internal/registry/*.go`, `operator/api/v1alpha1/{gameserver_types,gametemplate_types}.go`, `operator/internal/controller/gameserver_status.go`, `operator/internal/controller/gameserver_controller.go`, `charts/gameplane/values.yaml`, `go.work`, `modules/*/` directory listing, `cosign.pub`/`cosign-legacy.pub`/`cosign.pub.legacy-sig`

Not read: `docs/img/` (binary screenshots, only referenced by filename), `docs/superpowers/`, `docs/design/` (excluded per instructions), `AGENT.md`/`IDEA.md`/`reddit.md` (not in the given scope list), full text of `docs/module-authoring.md` (1579 lines — read the config-schema section and grepped the rest for count claims), full text of `docs/dependencies.md`/`docs/comparison-sources.md`/`CHANGELOG.md` past the sections cited above.

## Method

For each in-scope claim that names a concrete number, route, Helm key, env var, CRD field, or file, searched the corresponding code/config and compared literally. Did not run `go build` or `tsc --noEmit` (not required by the task and no code was changed).

## Observations (no finding)

- `PUT /admin/config/auth`, `DELETE /admin/config/auth/role-mappings/{role}` (CHANGELOG, security.md, oidc.md) — routes exist exactly as documented (`api/internal/handlers/config.go:55` etc.).
- `rcon.protocol` enum `source;telnet;websocket;battleye;satisfactory;palworld;nuclearoption;rest;cli;none` (CHANGELOG) — matches `operator/api/v1alpha1/gametemplate_types.go:997` kubebuilder enum exactly.
- CLI flags `--oidc-groups-claim`, `--oidc-role-mapping-{admin,operator,viewer}` (oidc.md:21) — all present in `api/cmd/main.go`.
- AddressAssignment condition reasons (networking.md) — all 8 reasons (`Assigned`, `AssignmentPending`, `PoolNotFound`, `AllocationFailed`, `AddressInUse`, `ServiceNotReady`, `IgnoredForExposureMode`, `NoAddressManagerConfigured`) found verbatim in `operator/internal/controller/gameserver_status.go` / `gameserver_controller.go`.
- Notification retry backoff "2s, then 8s" and rate limit "~12/min" (notifications.md) — match `api/internal/notify/deliver.go:27` (`{0, 2s, 8s}`) and `api/internal/auth/ratelimit.go:128` (`12.0/60.0`) exactly.
- 10 mod registries claimed in README — `api/internal/registry/` has exactly 10 non-test provider files (curseforge, factorio, github, hangar, modrinth, nexus, spigot, steam, thunderstore, umod).
- `GET /admin/audit/verify`, `GET /admin/audit/export`, `PUT /servers/{name}:collaborators`, `redactShareToken`, `Referrer-Policy: no-referrer` (secureHeaders), `api/internal/rbac/catalog.go`, `api/internal/db/shares.go` + `006_share_links.sql` — all exist exactly as security.md describes.
- Tunnel CRD fields (`GameServerTunnel`, `FrpTunnelSpec`, `TailscaleTunnelSpec`, `PlayitTunnelSpec` and their sub-fields) match docs/tunnels.md's field names.
- Design screens vs routes: every named route in `web/src/router/tree.tsx` (`/login`, `/`, `/servers`, `/servers/$name`, `/servers/new`, `/modules`, `/cluster`, `/users`, `/admin`, `/settings/theme`, `/admin/audit`, `/admin/logs`, `/backups`, `/share/$token`) has a corresponding `Screen/*` frame (or set of tab-variant frames) in `design-export/json/`; no orphaned route or obviously-orphaned top-level screen found.
- `cosign.pub`, `cosign-legacy.pub`, `cosign.pub.legacy-sig` referenced by key-rotation.md all exist at the repo root.
- Helm values referenced in install.md (`operator.gameDataStorage.storageClassName`, `api.oidc.{groupsClaim,defaultRole,roleMappings}`, `clusterOps.enabled`, `mcpServer.enabled`, `updates.channel`, `podSecurity.enforceRestricted`, `defaultModuleSource.*`, `uploadModuleSource.*`, `operator.localModules`, `serviceMonitors.enabled`, `operator.sentinelImage`, `operator.addressManager`, `capture.*`) all exist in `charts/gameplane/values.yaml` under the exact key paths documented.

## Candidate findings

### C-docs-01: Module/template count ("16") is stale across README, comparison-sources, and roadmap — actual shipped catalog is 30 modules

- **Location**: `README.md:71`, `README.md:85`, `README.md:142`, `README.md:178`, `README.md:259`; `docs/comparison-sources.md:69`; `docs/roadmap.md:164`, `docs/roadmap.md:266`
- **Category**: docs-drift
- **Suggested severity**: S3
- **Observation / repro**: `ls -d modules/*/ | wc -l` → 30 directories, every one with a `module.yaml` (verified). `docs/game-coverage.md` (itself current) lists all 30 in its coverage table and its own summary at the bottom says "2 modules... 26 modules... blocked... 2 modules... out-of-scope" (well, the *table* has 26 `blocked-doc` rows — see next finding). Yet:
  - README.md:85: "**OCI Game Modules**: 16 ready-to-use game templates packaged as OCI artifacts"
  - README.md:142: "`modules/` | YAML | 16 pre-packaged game templates ... |"
  - README.md:178: "`modules/` # Submodule: Game templates (16 games shipped)"
  - README.md:259: "pushes every directory under `modules/` (16 games at last count ...)"
  - README.md:71 (comparison table): "16 ready-to-use templates shipped."
  - docs/comparison-sources.md:69: "16 ready-to-use templates in gameplane-module repository."
  - docs/roadmap.md:164: "the other 14 shipped modules use generic" (implies 16 total: 2 real-parsing + 14 generic)
  - docs/roadmap.md:266: "**12 modules** are blocked on undocumented or partially-documented wire-protocol formats" — `docs/game-coverage.md`'s own table has 26 rows with Status `blocked-doc`, not 12.
- **Expected**: A count consistent with the actual `modules/` directory (30) or with `docs/game-coverage.md`'s own up-to-date table (2 covered-in-ci + 26 blocked-doc + 2 out-of-scope-by-design = 30).
- **Actual**: Five sentences in README.md, one in comparison-sources.md, and two in roadmap.md all repeat a stale "16"/"14"/"12" figure that undercounts the real catalog by roughly half. A prospective user comparing Gameplane's game coverage against Pterodactyl/AMP/Agones (the explicit purpose of the README comparison table this number sits in) is given a materially wrong number.

### C-docs-02: docs/security.md says `capture.enabled` "default is true"; actual default is `false`, contradicting install.md, architecture.md, and values.yaml itself

- **Location**: `docs/security.md:282`
- **Category**: security / docs-drift
- **Suggested severity**: S3
- **Observation / repro**: `charts/gameplane/values.yaml:527` → `capture:\n  enabled: false`. `docs/install.md:216-217` correctly states `capture.enabled` "(default `false`)". `docs/architecture.md:304` correctly states "Disabled by default". But `docs/security.md:281-283` (in the "Trade-off with PodSecurity `restricted`" section, option 1) reads: *"**Disable capture** — leave the cluster's capture feature disabled via Helm value `capture.enabled: false` (default is true)."*
- **Expected**: "(default is false)" to match values.yaml, install.md, and architecture.md.
- **Actual**: The sentence is self-contradictory (tells the admin to "leave it disabled via `capture.enabled: false`" while parenthetically claiming the default is `true`) and disagrees with two other docs describing the exact same Helm key. This sits inside a security-tradeoff discussion (whether relaxing Pod Security `restricted` is needed for capture), so an admin doing threat assessment from this doc alone could wrongly conclude the capture sidecar (which requires `allowPrivilegeEscalation: true`) is already active out of the box.

### C-docs-03: docs/tunnels.md example GameServer YAML uses a non-existent `spec.template` field instead of the required `spec.templateRef.name`

- **Location**: `docs/tunnels.md:125`, `docs/tunnels.md:166`, `docs/tunnels.md:213`
- **Category**: correctness / docs-drift
- **Suggested severity**: S3
- **Observation / repro**: All three worked examples (frp, Tailscale, playit) contain:
  ```yaml
  spec:
    template: minecraft-java
    networking:
      ...
  ```
  `operator/api/v1alpha1/gameserver_types.go:35-37` defines the actual field: `TemplateRef GameTemplateRef \`json:"templateRef"\`` with doc comment "References a GameTemplate ... Required." and no `omitempty` — there is no `template` field on `GameServerSpec` at all. Under the CRD's structural schema, an unrecognized `template` property is pruned and the required `templateRef` is then missing.
- **Expected**: `templateRef:\n  name: minecraft-java` (as correctly shown in `docs/networking.md:64-65`'s own example, in the same repo).
- **Actual**: A user who copies any of these three tunnel YAML examples verbatim and runs `kubectl apply -f` gets a validation error (missing required field `templateRef`) rather than a working tunneled GameServer.

### C-docs-04: docs/comparison-sources.md cites line numbers in CLAUDE.md that don't exist (CLAUDE.md is 311 lines; cited as 368/372)

- **Location**: `docs/comparison-sources.md:31`, `docs/comparison-sources.md:40`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**: `wc -l CLAUDE.md` → 311 lines. `docs/comparison-sources.md:31` cites "**Evidence**: ...; CLAUDE.md:368" and line 40 cites "**Evidence**: CLAUDE.md:372; ...". Both line numbers are past the end of the file.
- **Expected**: A citation this document's own stated purpose ("documents the sources and verification dates for every claim in the README comparison table") requires to be checkable.
- **Actual**: The citations cannot be verified because the referenced lines don't exist — CLAUDE.md has likely been trimmed/reorganized since this evidence file was last updated (2026-09-02 per its own header), and the line numbers were never refreshed.

### C-docs-05: docs/dependencies.md's own module-count claim is wrong, and the report omits 5 real Go workspace modules (sentinel, capture-sidecar, gameproto, svcutil, tunnel) entirely

- **Location**: `docs/dependencies.md:4-8`
- **Category**: docs-drift
- **Suggested severity**: S3
- **Observation / repro**: Lines 4-8 read: *"It covers the 9 Go modules that share `go.work` (`netguard`, `gameaction`, `operator`, `api`, `agent`, `audit-syslog-bridge` [optional], `telemetry-receiver` [optional], `mcp-server` [optional], `test/e2e`)..."* `go.work` actually lists 15 `use` entries: `agent, api, audit-syslog-bridge, capture-sidecar, gameaction, gameproto, gp-module, mcp-server, netguard, operator, sentinel, svcutil, telemetry-receiver, test/e2e, tunnel`. The doc's "At a glance" table (`## Shared Go modules` / `## Services` sections, lines 45-220) never mentions `sentinel/`, `capture-sidecar/`, `gameproto/`, `svcutil/`, or `tunnel/` at all — not even as a "0 deps, stdlib-only" row (the pattern used for `netguard`/`gameaction`, which also have 0 direct deps but are listed). `sentinel/go.mod` requires `k8s.io/apimachinery`, `k8s.io/client-go` (real third-party deps); `capture-sidecar/go.mod` requires `github.com/gopacket/gopacket` and `github.com/packetcap/go-pcap` (a packet-capture library) — none of these appear anywhere in the report.
- **Expected**: A dependency inventory whose stated goal is "component-by-component inventory of Gameplane's third-party dependencies and why each one is there" (line 3) covering all Go modules in `go.work`, or explicitly scoping/footnoting the omission.
- **Actual**: 5 of 14 non-submodule Go workspace modules are undocumented, two of which (`sentinel`, `capture-sidecar`) pull in real third-party packages (including a raw-packet-capture library) that get no "why it exists" justification anywhere in the repo's dependency report.

### C-docs-06: docs/contributing.md's "Per-component" test list has the same 5-module gap as C-docs-05

- **Location**: `docs/contributing.md:74-83`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**: The `## Testing` → "Per-component" block lists `cd` commands for `netguard`, `gameaction`, `operator`, `api`, `agent`, `audit-syslog-bridge`, `telemetry-receiver`, `mcp-server`, `web` — the identical 9-module set as C-docs-05, again omitting `sentinel`, `capture-sidecar`, `gameproto`, `svcutil`, `tunnel`.
- **Expected**: A `cd <module> && go test ./...` line for every Go module a contributor might touch.
- **Actual**: A contributor following this list literally would never run tests for the 5 omitted modules, none of which are hypothetical — they're documented, shipped components with their own README/CLAUDE.md references elsewhere in the repo (e.g. README.md's own component table lists `sentinel/`, `tunnel/`, `capture-sidecar/`).

## Questions (not findings)

- `docs/contributing.md:15` claims "18 designed screens" as the source of truth for the dashboard. `design-export/json/` has far more than 18 `Screen/*` frames, but many are state variants of the same logical screen (e.g. 6+ `Server Detail — Settings · *` sub-screens, several `(Light)` theme variants). Grouping variants back into logical screens plausibly lands near 18, but this review didn't have a reliable way to enumerate "logical screens" vs "variants" from the flat JSON export, so this is left as a question rather than a finding.
- `docs/comparison-sources.md:40`'s evidence also cites `charts/gameplane/values.yaml:58`, which at that line is a comment about `operator.addressManager`, not obviously about the frp/Tailscale/playit relay row it's attached to. Didn't chase this further — could be an off-by-a-few-lines citation drift like C-docs-04, or could be referencing the broader block; not concrete enough to file as its own finding.
