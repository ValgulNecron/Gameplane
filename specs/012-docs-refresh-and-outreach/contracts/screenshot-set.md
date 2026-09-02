# Contract: Dashboard Screenshot Deliverable

**Feature**: 012 — Documentation Refresh and Outreach  
**Status**: Specification Draft  
**Applies To**: Dashboard screenshots in `docs/img/` and README.md gallery  
**Requirements Addressed**: FR-015, FR-016, FR-017, FR-018, FR-019, SC-009, SC-010, SC-011

---

## Purpose

This contract specifies the screenshot deliverable: which six existing JPEG files must be refreshed with current UI state, which five mandatory (plus one recommended) new JPEG files must be captured, where they appear in README.md, how to ensure data safety (no real hostnames/IPs), the capture environment, and verification procedures. The screenshot capture itself is PLANNED here; execution is deferred (per OD-3) to implementation.

---

## Scope: Six Existing Screenshots (FR-016)

The following six JPEG files in `docs/img/` are the source of truth for the current dashboard state. These **must be kept** (same filenames) and retaken to show the current web/ codebase as of v0.2.0-beta.8.

### Existing Screenshot Inventory

| Filename | Route | Component | Purpose | Alt Text |
|---|---|---|---|---|
| `dashboard.jpg` | `/` | `DashboardPage` | Fleet health overview, cluster stats, node status, recent activity feed | Fleet health at a glance: running/stopped/failed server counts, cluster CPU/memory/storage, node status, recent activity |
| `servers-list.jpg` | `/servers` | `ServersPage` | Table listing all game servers with status, resource usage, node placement | Every game server, one list: live status, CPU, memory, node placement |
| `server-overview.jpg` | `/servers/$name?tab=overview` | `ServerDetail` Overview tab | CPU/memory/disk usage sparklines, quick actions, sleep status, recent events summary | Server detail Overview: CPU, memory, disk usage, quick actions, connection info |
| `mods-registry-browse.jpg` | `/servers/$name?tab=mods` | `ServerDetail` Mods tab | Game template registry browser showing mod grid with metadata | Mods tab registry browser: grid of Thunderstore mods for Valheim, download counts |
| `server-console.jpg` | `/servers/$name?tab=console` | `ServerDetail` Console tab | Live xterm.js streaming output over WebSocket (game server stdout) | Live streaming console: Terraria server stdout, world-save progress |
| `admin-mod-registries.jpg` | `/admin?section=mod-registries` | `AdminSettings` Mod registries section | Admin configuration for external mod registry API keys (CurseForge, Steam, Nexus, etc.) | Admin Settings Mod registries: CurseForge and Steam Workshop configured, Nexus not configured |

**Evidence**: Route mappings from `/home/user/Gameplane/web/src/router/tree.tsx:5-144` and tab definitions from component files in `web/src/routes/`. Current README.md lines 18-27 link these six files.

---

## Scope: At Least Five New Screenshots, Six Recommended (FR-017)

At least five new JPEG files must be added to `docs/img/`, capturing currently uncovered screens. The table below ranks them by FR-017 priority ("screens new users encounter first"). **The first five are mandatory; a sixth is recommended. If capture cannot occur for a planned screen, substitute a lower-priority one and document why in the audit log (D-F).**

### Recommended New Screenshots (Ranked by FR-017 Priority)

| # | Filename | Route | Component | Purpose | Alt Text Draft | Dummy Data Required |
|---|---|---|---|---|---|---|
| **1** | `login.jpg` | `/login` | `LoginPage` | Authentication entry point; pre-auth surface with no metrics/hostnames (verified CLAUDE.md rule 3 compliance at Login.tsx:19-21) | Sign-in form with local username/password field and SSO provider buttons ("Continue with…"), brand logo, and marketing pitch sidebar describing Gameplane's features | None; login form is static UI. SSO provider button labels are mocked to generic names (e.g., "GitHub", "Keycloak") without issuer URLs |
| **2** | `create-server-template-select.jpg` | `/servers/new` (template selection step) | `CreateServerWizard` template step | Game selection in multi-step create workflow; shows available GameTemplates (Minecraft, Valheim, Terraria, Rust, Palworld, Factorio, CS2, ARK) with icons and descriptions | Game template grid in Create Server wizard: card grid showing game icons, titles (Minecraft Java Edition, Valheim, Terraria, etc.), brief descriptions, and "Select" button per template | 8+ `GameTemplate` resources from MSW fixtures: name, icon URL (data: URI), description. Include both "official" templates and an "example user-uploaded" template to show extensibility |
| **3** | `server-detail-events.jpg` | `/servers/$name?tab=events` | `ServerDetail` Events tab | Kubernetes event timeline for diagnostics; shows scheduling, image pulls, crashes, readiness transitions | Events tab timeline: shows Kubernetes events with reason (ImagePull, Scheduling, Ready), message (brief status), timestamp, and event type indicator; demonstrates failure diagnostics for a previously-failed server |  `GameServer` resource in Pending/Failed phase with `status.conditions` (Progressing=False, Ready=False). Kubernetes events list: mix of normal events (Scheduled, PullImage, Created) and warning events (ImagePullBackOff, CrashLoopBackOff) with realistic timestamps |
| **4** | `admin-settings-general.jpg` | `/admin?section=general` | `AdminSettings` General section | Instance identity configuration; external URL (used for OIDC callbacks), cluster namespace defaults | Admin Settings General section: form fields for Instance Name ("My Gameplane Cluster"), External URL ("https://gameplane.example.com"), Default Namespace dropdown; visible sidebar navigation showing all 9 settings sections | Instance config from MSW handler: `{ instanceName: "My Gameplane Cluster", externalUrl: "https://gameplane.example.com", defaultNamespace: "default" }`. Namespace dropdown with options: "default", "gameplane-production", "test-cluster" |
| **5** | `cluster-nodes.jpg` | `/cluster` | `ClusterPage` | Multi-node cluster management; shows node list with CPU/memory/storage utilization meters | Cluster page with node list: at least 3 nodes displayed, each showing uptime, CPU/memory/storage usage bars (green for available, orange/red for high usage), capacity info, and "Join Node" wizard card | 3+ `ClusterNode` resources with realistic resource metrics: CPU 40–80% utilization, memory 50–90%, storage 20–60% used; mix of "Ready" and "Provisioning" statuses to show operational variety |
| **6** | `server-detail-logs.jpg` | `/servers/$name?tab=logs` | `ServerDetail` Logs tab | Application log streaming view; shows server stdout/stderr lines as they arrive | Logs tab with live application output: shows >20 game server log lines (e.g., world save progress for Terraria, player join notifications for Minecraft) with timestamps; demonstrates scrolling capability; footer shows "tail 100 lines" selector | Streamed log lines (mocked WebSocket in mock mode): realistic game-server output. Example format for Minecraft: `[HH:MM:SS] [main/INFO] [net.minecraft.server.MinecraftServer]: Player alice joined the game`. Example for Terraria: `[HH:MM:SS] Server started in single-player mode.` |

**Evidence**: R4 research findings (screen inventory at lines 9–87, gap analysis at lines 106–211, capture assessment at lines 233–365); spec FR-017 examples (Login, Create Server, Server Detail Events/Logs, Admin Settings, Cluster) all exist and are ranked by user priority.

---

## README.md Gallery Structure (FR-015, SC-010)

The README's `## Screenshots` section (currently lines 18–27) is a 2-column table linking the six existing screenshots. This table MUST be extended to display at least 11 total images (six existing + at least five new).

### Current Structure (lines 18–27)

```markdown
## Screenshots

| | |
|---|---|
| ![<alt-text>](docs/img/dashboard.jpg) | ![<alt-text>](docs/img/servers-list.jpg) |
| <caption> | <caption> |
| ![<alt-text>](docs/img/server-overview.jpg) | ![<alt-text>](docs/img/mods-registry-browse.jpg) |
| <caption> | <caption> |
| ![<alt-text>](docs/img/server-console.jpg) | ![<alt-text>](docs/img/admin-mod-registries.jpg) |
| <caption> | <caption> |
```

### Required Changes

1. **Extend the table** to include all new screenshots (at least 5 new → 11+ total images)
2. **Maintain 2-column layout** (or 3+ columns if preferred; decision left to maintainer)
3. **Keep image rows first**, alt-text row second in each pair, for visual scanning
4. **Preserve existing alt text** from current README.md lines 22, 24, 26 for the six existing images
5. **Add new alt text** for each new image (see below)
6. **File paths** remain `docs/img/<filename>`
7. **Ordering**: Display in recommended priority order (new images first for evaluator impact, or existing images first for backward compatibility; recommend new-first for evaluator prioritization per FR-017)

### Gallery Alt Text Examples

**Format**: One sentence per image. State **purpose** (what the screen shows) and **key UI elements** (buttons, metrics, tabs, lists). Explicitly note if data is mocked where applicable (console logs, player rosters, live streams in mock-mode screenshots).

**Examples**:
- ✅ `Sign-in form with local username/password field and SSO provider buttons, brand logo, and marketing sidebar.`
- ✅ `Game template grid in Create Server wizard: card grid showing game icons, titles, descriptions, and Select buttons.`
- ✅ `Cluster page with node list showing 3 nodes with CPU/memory/storage usage meters, uptime, and capacity info.`
- ✅ `Logs tab showing >20 game server log lines with timestamps (mock mode: realistic game-server output format).`
- ❌ `Shows the login page.` (too vague; does not state key UI elements)
- ❌ `Kubernetes event timeline.` (does not state purpose relative to operator/new user)

**Evidence**: FR-018, SC-010 requirements; R4 alt-text guidance at lines 452–475.

---

## Format Convention: JPEG 1568×773

All screenshots in `docs/img/` follow a consistent format established by the six existing files.

| Property | Specification | Evidence | Notes |
|---|---|---|---|
| **File Format** | JPEG | Six existing files are JPEG; `docs/img/*.jpg` filename convention | Binary transparency not needed (not PNG); JPEG compression reduces file size while maintaining visual quality |
| **Viewport Dimensions** | 1568×773 pixels (width × height) | `identify docs/img/*.jpg` on existing files confirms all six are 1568×773 | This is **NOT a spec value** but a convention inferred from existing files. If a different resolution is preferred, the maintainer must decide before capture (OD-3). Playwright Desktop Chrome default is 1280×720; explicit viewport config required to match |
| **Aspect Ratio** | ~2.03:1 (landscape, slightly wider than 16:9) | Computed from 1568÷773 | Accommodates dashboard left-sidebar + content area + right-sidebar in single view |
| **JPEG Compression Quality** | 75–85 (good quality, 47–74 KB per file) | Existing files measured at 47–74 KB | Lossless/optimized compression acceptable; no visible quality loss vs. files at this size range |
| **Color Space** | sRGB (standard RGB) | Standard JPEG color space | No color management requirements; standard monitors render correctly |
| **Metadata** | Minimal (filename only) | Existing files have no EXIF/XMP metadata | Acceptable to strip metadata; timestamp and capture tool name not required |

### Playwright Viewport Configuration

To capture at 1568×773 with Playwright (mock mode recommended per R4):

```typescript
// In playwright.config.ts or within test:
await page.setViewportSize({ width: 1568, height: 773 });
```

Or set globally in config (preferred):

```typescript
use: {
  viewport: { width: 1568, height: 773 },
  deviceScaleFactor: 1,
}
```

**Evidence**: R4 Appendix C (lines 453–490); playwright.config.ts current setup uses Desktop Chrome preset (1280×720 by default, requires override).

---

## Dummy Data Rule: What NOT to Show (FR-019, SC-011)

Screenshots must not display **any** real user data, infrastructure details, or personally identifiable information that could aid operational reconnaissance.

### Forbidden Patterns (Do NOT Include)

| Category | Forbidden | Reason | Acceptable Substitute |
|---|---|---|---|
| **IPv4 Addresses** | `192.168.1.100`, `10.0.0.5`, `203.0.113.42` | Could expose private network topology or real server IP | `<internal>`, `[redacted]`, or omit field entirely |
| **Public Hostnames** | `prod-cluster-1.company.com`, `gameplane.example.com`, `minecraft.org` | Could identify real infrastructure or operator identity | `test-cluster-01`, `gameplane-demo.local` (use `.local` TLD for clarity) |
| **Real Cluster Names** | `production`, `aws-us-east-1`, `customer-acme-games` | Identifies specific deployments or customers | `test-cluster`, `demo-cluster`, `my-cluster` |
| **Real Usernames** | `alice@company.com`, `gamemaster`, `admin@customer.net` | Violates pre-auth privacy (rule 3) and user privacy | `test-user-01`, `admin-demo`, `operator-01` |
| **Real Email Addresses** | `ops@example.com`, `support@gameplane.io` | PII; could be harvested for social engineering | `test-user-01@local` (or omit, use generic "test user") |
| **Real Game Server Names** | `production-minecraft`, `customer-valheim-private`, `survival-map-v3` | Identifies operational intent | `test-server-01`, `demo-minecraft`, `example-valheim` |
| **Real Player Names / UUIDs** | Minecraft player UUIDs like `550e8400-e29b-41d4-a716-446655440000` or real game usernames | PII for game account holders | Fabricated UUIDs (`00000000-0000-0000-0000-000000000001`) or generic names (`Player-01`, `TestUser`) |
| **Real Module / Template Names** | Names showing customer-specific or copyrighted game variants | Mods tab might show user-uploaded custom modules | Stick to official template names (Minecraft, Valheim, Terraria) or generic examples (`custom-game-01`) |

**Note on Timestamps**: Timestamps are not sensitive data. FR-019 concerns real user data, hostnames, cluster names, and IP addresses only. Render whatever the UI displays at capture time; never back-date or fabricate dates to make documentation appear fresher than it is.

### Allowed Dummy Data Naming Scheme

Consistent test data makes screenshots cohesive and professional:

| Entity Type | Naming Scheme | Examples |
|---|---|---|
| **Servers** | `test-server-NN` or `demo-game-NN` | `test-server-01`, `demo-minecraft`, `example-valheim` |
| **Clusters** | `test-cluster` or `demo-cluster` | `my-cluster`, `test-cluster`, `local-dev` |
| **Namespaces** | `default`, `gameplane-demo`, `test-ns` | Do not use `production`, `prod`, `live` |
| **Nodes** | `node-01`, `node-02`, `node-03` (or auto-assigned node names from K8s) | Do not use cloud provider names like `aws-us-east-1-a` |
| **Users** | `test-user-01`, `admin-demo`, `operator-test` | Do not use names with `.com` or real email domains |
| **API Keys / Tokens** | Truncated or masked (`sk-...`) or omitted entirely | Do not show real secrets; mask with `[redacted]` if UI shows secret field |
| **Timestamps** | Not applicable — render whatever the UI displays at capture time; see Note above | No fabrication or back-dating required |
| **Module Names** | Official Gameplane templates + generic examples | Minecraft, Valheim, Terraria, Rust, Palworld, Factorio, CS2, ARK (shipped templates) or `test-mod-01` (custom) |

**Evidence**: FR-019, SC-011; CLAUDE.md rule 3 (login privacy); spec edge case (lines 82–89); R4 examples (lines 219–226).

---

## Capture Procedure (OD-3)

Screenshot capture is **PLANNED** in this contract; the actual capture method is OPEN and requires a maintainer decision (OD-3). Three options are outlined below with pros/cons; a recommendation is provided, but implementation defers to the maintainer's approval.

### Option A: Playwright Mock Mode (MSW + Vite) — **RECOMMENDED**

**Prerequisites**: None (no cluster required)  
**Setup**: `GAMEPLANE_E2E_TARGET=mock npm run test:e2e:mock` (or equivalent Playwright invocation)  
**Configuration**: Uses existing `web/playwright.config.ts` and MSW handlers in `web/src/test/handlers.ts`

#### How It Works

1. Playwright launches Vite dev server on `localhost:5173` with mock mode enabled
2. MSW intercepts all API fetch requests and returns stubbed responses (realistic data shapes, mocked values)
3. Dashboard renders with mocked data (no live Kubernetes connection, no real servers running)
4. Screenshots capture the UI as it appears with mock data (no cluster provisioning needed)

#### Pros (Why Recommended)

- ✅ **Reproducible**: MSW handlers are deterministic; screenshots are identical on every run
- ✅ **Fast**: No cluster setup; Vite + Playwright screenshot cycle <2 seconds per screen
- ✅ **CI-native**: Runs in standard GitHub Actions runners (no Docker, kubectl, or cluster access needed)
- ✅ **Realistic UI**: MSW responses match production API shapes; UI rendering is identical to production
- ✅ **Data Control**: Test factories (`web/src/test/factories.ts`) can be updated per release without re-provisioning
- ✅ **Isolation**: Each test independent; no shared cluster lock or state drift between runs
- ✅ **Cost**: Zero infra overhead; no compute resources beyond GitHub Actions standard allocation

#### Cons & Trade-offs

- ⚠️ **Console / Logs Tabs**: Live stream (xterm, WebSocket) shows mocked log lines (not real server output). **Acceptable for screenshots documenting "here is the UI"; alt text clarifies "mock mode".** Example alt text: `Logs tab with mocked game-server output lines showing the UI and scrolling capability.`
- ⚠️ **Cluster / Node Stats**: CPU/memory/storage meters show fixed mock values, not real cluster variation. **Acceptable for demo purposes.**
- ⚠️ **Player Rosters**: Player names and UUIDs are fabricated. **Acceptable; reinforces dummy-data compliance.**
- ⚠️ **Mod Registries**: Shows a subset of mods (5–10 per registry, not 100s). **Acceptable; documents the UI layout.**

#### Implementation Steps

1. **Create test spec** at `web/e2e/specs/screenshots.spec.ts` (or similar name)
2. **Seed MSW fixtures** with realistic data:
   - 8+ `GameTemplate` objects (Minecraft, Valheim, Terraria, Rust, Palworld, Factorio, CS2, ARK)
   - 3–5 `GameServer` resources (mix of phases: Running, Pending, Failed)
   - 3+ `ClusterNode` resources with CPU/memory/storage usage
   - 10–20 audit events (mix of methods, status codes)
   - 5–10 mods per registry (Thunderstore, CurseForge, Steam)
   - 3–5 test users with different roles (admin, operator, viewer)
   - Schedule and Restore resources for Backups tab
3. **Authenticate once** at test start (MSW provides stub `/users/me` response)
4. **Navigate to each route** in recommended priority order
5. **Set viewport** to 1568×773 (see Viewport Configuration above)
6. **Capture screenshot** for each route:
   ```typescript
   await page.screenshot({
     path: 'docs/img/<filename>.jpg',
     fullPage: false,  // Capture only visible viewport, not full page
   });
   ```
7. **Post-process JPEG** (optional): Re-encode losslessly to match 47–74 KB range of existing files (e.g., `imagemin`, `mozjpeg`)
8. **Commit screenshots** to git with all filenames matching the table above

#### Testing & Validation

- Run `npm run test:e2e:mock` locally to verify all captures succeed
- Verify generated JPEG files are 1568×773 (use `identify docs/img/<filename>.jpg`)
- Verify no forbidden patterns in alt text or visible captions
- Manual visual inspection: compare new screenshots against running dashboard to ensure layout/styling match

---

### Option B: Playwright Live Mode (kubectl port-forward + make dev-up)

**Prerequisites**: Running `make dev-up` cluster + seeded data  
**Setup**: `GAMEPLANE_E2E_TARGET=live npm run test:e2e:live` after cluster is up  
**Configuration**: Uses existing port-forward from `globalSetup` hook in playwright.config.ts:37–38

#### How It Works

1. Playwright starts a local Kubernetes cluster via `make dev-up`
2. Port-forward to API service (e.g., `:8080`)
3. Seeded GameServers, users, and modules are created in the cluster
4. Dashboard connects to real API; captures render live cluster state
5. If servers are running, Console/Logs tabs show real output; Mod registries show real catalogs

#### Pros

- ✅ Realistic data: Real GameServer resources, real cluster stats, real game output
- ✅ Live streams: Console/Logs tabs show authentic server logs (if servers seeded)
- ✅ Full registries: Mods tab shows complete registry data (if modules pushed)

#### Cons

- ❌ Requires infrastructure: `make dev-up` takes 2–5 minutes; GitHub Actions may not support Docker-in-Docker reliably
- ❌ Complex setup: Must seed GameServers, modules, users, and wait for servers to reach Running phase
- ❌ Non-reproducible: Cluster state drifts per run; timers, cron schedules, and event timing create variance
- ❌ Flaky: If cluster provisioning fails, screenshot capture fails; no fallback
- ❌ Resource-hungry: Kind cluster consumes 4–8 GB RAM; unsuitable for standard GitHub Actions runners
- ❌ Timing brittle: Servers may not reach Running phase before screenshot; captures might show Pending state instead

**Recommendation**: Not recommended for this project due to CI fragility and cost. Use only if Option A proves insufficient.

---

### Option C: Manual Capture (Web Browser on make dev-up)

**Prerequisites**: Running `make dev-up` cluster + seeded data (same as Option B)  
**Method**: Web browser (Chrome/Firefox) at 1568×773 viewport; manual navigation and screenshot tool

#### Pros

- ✅ Full realism (same as live mode)
- ✅ Framing control (crop, zoom, highlight regions)

#### Cons

- ❌ Not reproducible (manual steps, human framing inconsistency)
- ❌ Not automated (blocks on maintainer time)
- ❌ Not CI-gated (no automated check that screenshots stay fresh on PR)
- ❌ Same cluster setup burden as Option B

**Recommendation**: Valid **fallback only** if Options A and B fail. Not recommended as primary method (violates automation principle from spec).

---

### OD-3 Decision: Recommended Path

**Use Option A (Playwright Mock Mode)** for the following reasons:

1. **Reproducibility**: Identical screenshots on every run; diff-able in git
2. **CI Integration**: Integrates into `npm run test:e2e:mock` pipeline; can run on every PR
3. **Speed**: Seconds per screenshot vs. minutes per cluster provisioning
4. **Maintainability**: Data fixtures (`web/src/test/factories.ts`) update per release without cluster re-provisioning
5. **Acceptance**: Trade-off (mocked live streams) is acceptable and documented in alt text

**Awaiting maintainer approval** before implementation proceeds to capture execution.

---

## Common Capture Steps (All Options)

Regardless of which capture method is chosen, all captures follow these common procedures:

### Pre-Capture

1. **Clear browser cache** (or use incognito mode): Ensures CSS/JS loads fresh
2. **Disable browser extensions**: No ad blockers, password managers, or toolbars that might obscure UI
3. **Set timezone to UTC** (or a fixed timezone) if timestamps are visible in UI: Consistent across all screenshots
4. **Set system theme to light** (if applicable): Gameplane dashboard uses light theme by default; no dark-mode variants needed
5. **Verify viewport is exactly 1568×773**: Use DevTools or Playwright to confirm before capture

### Capture

6. **Authenticate** (once at test start, reuse session for all screens)
7. **Navigate to route** per recommended priority table
8. **Wait for content to load** (Playwright: `await page.waitForLoadState('networkidle')`)
9. **Set viewport to 1568×773** if not already set
10. **Screenshot** with `page.screenshot({ path: 'docs/img/<filename>.jpg', fullPage: false })`
11. **Verify file was written** and is 1568×773

### Post-Capture

12. **Optional JPEG re-encode** (if needed): Use `imagemin-mozjpeg` or similar to match 47–74 KB range of existing files
13. **Run `ls -lh docs/img/*.jpg`** to verify all 11+ files exist and are reasonable file sizes
14. **Spot-check alt text** for forbidden patterns (no hostnames, IPs, real usernames)
15. **Git add and commit** all new/refreshed screenshots
16. **Push to branch** and let CI verify screenshot integrity

---

## Substitution Rule (Spec Edge Case, Lines 82–89)

If a planned screenshot cannot be captured because:

- The feature is temporarily unavailable (e.g., a required component crashed or is disabled)
- The UI is not rendering (e.g., a route returns 404 or a page fails to load)
- The capture environment cannot reach the necessary state (e.g., mock mode MSW fixtures are incomplete)

**Then:**

1. **Substitute a lower-priority screen** from the tier list (Tier 2 or Tier 3 in R4, lines 140–211)
2. **Document the substitution** in the audit log (`specs/012-docs-refresh-and-outreach/audit-log.md`, per D-F):
   - Original planned screenshot (filename, route)
   - Reason (feature unavailable, route broken, fixture incomplete, etc.)
   - Substitute chosen (filename, route)
   - Evidence checked (path:line in logs or error message)
   - Resolution (e.g., "feature will be added in v0.2.0-beta.9; defer to next refresh cycle")
3. **Update the screenshot table** in this contract to reflect the substitution (filename, route, purpose)
4. **Commit the substitution decision** with the audit log entry

**Example**:
> **Original**: `admin-settings-authentication.jpg` (/admin?section=authentication) — OIDC provider form  
> **Reason**: Keycloak test fixture integration incomplete in MSW; route renders but form validation hangs  
> **Substitute**: `admin-settings-notifications.jpg` (/admin?section=notifications) — Event sink configuration  
> **Evidence**: web/src/test/handlers.ts:line 214 (authentication route stubbed but incomplete), playwright.config.ts globalSetup timeout logs  
> **Resolution**: Deferred to next minor release when OIDC seeding is available

---

## Verification Checklist

Use this checklist **before merging** the screenshot commits (D-F, per constitution Principle VI — CI is the system of record).

### File Inventory

- [ ] All six existing filenames are present in `docs/img/`:
  - [ ] `dashboard.jpg` (1568×773 JPEG)
  - [ ] `servers-list.jpg` (1568×773 JPEG)
  - [ ] `server-overview.jpg` (1568×773 JPEG)
  - [ ] `mods-registry-browse.jpg` (1568×773 JPEG)
  - [ ] `server-console.jpg` (1568×773 JPEG)
  - [ ] `admin-mod-registries.jpg` (1568×773 JPEG)
- [ ] At least five new filenames are present:
  - [ ] `login.jpg`
  - [ ] `create-server-template-select.jpg`
  - [ ] `server-detail-events.jpg`
  - [ ] `admin-settings-general.jpg`
  - [ ] `cluster-nodes.jpg`
  - [ ] `server-detail-logs.jpg` (recommended sixth)
- [ ] All files are JPEG format (not PNG, not WebP)
- [ ] All files are exactly 1568×773 pixels (run `identify docs/img/*.jpg` or similar)
- [ ] All files are <150 KB (reasonable compression; typical 47–74 KB for existing files)

### README.md Gallery

- [ ] `README.md` lines 18+ contain a `## Screenshots` section
- [ ] Section includes all 11+ images (6 existing + ≥5 new)
- [ ] Each image is in an `![alt text](docs/img/filename.jpg)` Markdown link
- [ ] Each image has alt text (one sentence, purpose + key UI elements)
- [ ] Alt text is in the image markdown, not in a separate caption row (alt text is in square brackets, not in table caption)
- [ ] All filenames match the files in `docs/img/`

### Alt Text Compliance

- [ ] Each alt text is one sentence (under 160 characters ideally)
- [ ] Each alt text states **purpose** (e.g., "authentication entry point", "fleet health overview")
- [ ] Each alt text describes **key UI elements** (e.g., "form fields", "metrics", "buttons", "tabs")
- [ ] No alt text is vague (e.g., "Shows the dashboard" is too vague; "Fleet health overview with running/stopped counts" is good)
- [ ] If data is mocked (mock-mode screenshots), alt text clarifies this (e.g., "with mocked server output")

### Dummy Data Compliance

- [ ] Run a grep search for forbidden patterns in visible text in screenshots:
  ```bash
  grep -riE '(prod|production|customer|acme|example\.com|192\.|10\.|172\.1[6-9]\.|172\.2[0-9]\.|172\.3[01]\.|\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.(com|net|org|io)\b)' docs/img/
  ```
  (This is a heuristic; manual inspection recommended for hostnames and IPs in visible text)
- [ ] Verify all server names follow `test-server-NN` or `demo-*` naming scheme
- [ ] Verify all cluster names are generic (`test-cluster`, `my-cluster`, not `production`, `customer-xyz`)
- [ ] Verify all usernames are generic (`test-user-01`, `admin-demo`, not real emails)
- [ ] Timestamps are not sensitive (FR-019); no action needed — render whatever the UI displays at capture time (see Dummy Data Rule Note)
- [ ] Verify no real player names, UUIDs, or mod names are visible

### Git & Commit

- [ ] All screenshots are committed to git (no large .gitignore exclusions of `docs/img/`)
- [ ] Commit message follows conventional-commit format (e.g., `docs: add six new dashboard screenshots (FR-016, FR-017)`)
- [ ] Commit message references FR-015, FR-016, FR-017 (or applicable FRs)
- [ ] Commit is signed (`git commit -s`)

### Audit Log (D-F)

- [ ] If any substitutions were made, `specs/012-docs-refresh-and-outreach/audit-log.md` includes one row per substitution:
  - [ ] Filename, route
  - [ ] Finding (original intended screenshot)
  - [ ] Evidence (path:line checked)
  - [ ] Resolution (substitute + reason)
  - [ ] Commit hash (link back to this audit-log change)

---

## Open Decisions

The following decisions are **not settled in this contract** and require maintainer ruling before or during implementation:

| OD ID | Question | Options | Recommendation | Status |
|---|---|---|---|---|
| **OD-3** | Screenshot capture environment | (A) Playwright mock mode, (B) Live mode (make dev-up), (C) Manual capture | **Use Option A (mock mode)** for speed, reproducibility, and CI integration | Awaiting approval |
| **OD-3a** | Should viewport remain 1568×773 (inferred from existing files), or should a different resolution be preferred? | (a) Keep 1568×773, (b) Switch to 1280×720 (Desktop Chrome default), (c) Different resolution | **Keep 1568×773** (matches existing, professional aspect ratio) | Pending confirmation |
| **OD-3b** | Should screenshot capture be a Playwright test spec in `web/e2e/` or a separate CI job? | (i) Playwright spec (`web/e2e/specs/screenshots.spec.ts`) run by `npm run test:e2e:mock`, (ii) Standalone CI job | Recommend (i) for integration; (ii) is also acceptable | Pending decision |
| **OD-3d** | Should alt text for live-stream tabs (Console, Logs) explicitly note "mocked data" when using mock-mode screenshots, or is the contract-level disclaimer sufficient? | (a) Explicit in every alt text, (b) Once in README intro, (c) In alt text only for live-stream tabs | Recommend (c): note mock mode in Console/Logs alt text only | Pending maintainer style preference |

---

## References

- **Specification**: `/home/user/Gameplane/specs/012-docs-refresh-and-outreach/spec.md` (FR-015 through FR-019, SC-009 through SC-011)
- **Research**: `/tmp/claude-0/-home-user-Gameplane/*/scratchpad/plan-012/R4-screens.md` (complete inventory, gap analysis, capture assessment)
- **Current README**: `/home/user/Gameplane/README.md` lines 18–27 (existing gallery)
- **Playwright Config**: `/home/user/Gameplane/web/playwright.config.ts` (mock/live mode setup, viewport defaults)
- **MSW Handlers**: `/home/user/Gameplane/web/src/test/handlers.ts` (API response stubs)
- **Test Factories**: `/home/user/Gameplane/web/src/test/factories.ts` (dummy data generation)
- **Route Definition**: `/home/user/Gameplane/web/src/router/tree.tsx` (route list, component mapping)
- **Component Routes**: `/home/user/Gameplane/web/src/routes/*.tsx` (Dashboard, ServerDetail, AdminSettings, etc.)
- **Login Privacy Verification**: `/home/user/Gameplane/web/src/routes/Login.tsx` lines 19–21 (CLAUDE.md rule 3 compliance)
- **CLAUDE.md Rule 3**: `/home/user/Gameplane/CLAUDE.md` (login privacy, pre-auth surface requirements)
- **Existing Screenshots**: `/home/user/Gameplane/docs/img/` (six JPEG files, all 1568×773)
- **Constitution Principle VI**: `/home/user/Gameplane/.specify/memory/constitution.md` lines 249–271 (CI as system of record)
- **Style Reference**: `/home/user/Gameplane/specs/done_011-add-missing-module-specs/contracts/specs-md-structure.md` (document format and tone)

---

## Document Status

**Status**: Specification Draft  
**Version**: 1.0  
**Authored**: 2026-09-01  
**Applies To**: Planning phase (this document); captures deferred to implementation phase (OD-3 decision pending)  
**Maintainer Decision Required**: OD-3, OD-3a, OD-3b, OD-3d before capture execution begins
