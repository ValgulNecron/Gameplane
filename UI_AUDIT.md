# Kestrel dashboard — UI audit (design ↔ implementation)

**Date:** 2026-06-14
**Scope:** Every web dashboard screen, compared against the source-of-truth designs in `design.pen`, plus runtime bug hunting (console errors, broken states).
**Goal:** Findings doc to feed into plan mode for fixing.

---

## How this was checked (reproduce)

The live app was driven in Chrome against the project's **MSW mock mode** (deterministic, fully-populated, no cluster needed) — the same harness `web` Playwright E2E uses:

```sh
cd web && VITE_E2E_MOCK=true npx vite --mode mock --port 5174 --strictPort
# open http://localhost:5174/login, sign in with any non-empty creds (mock accepts all)
```

Each screen was compared 3 ways: **live screenshot** + **`design.pen` frame screenshot** + **component source** (`web/src/routes/**`). Designs were read via the `pencil` MCP (node IDs cited per finding).

> **Caveats baked into the mock:** zeros/blanks below that are *data-driven* (CPU 0 %, empty events, host `—`, 1 cluster node, 2 templates, 2 backups) are MSW limitations, **not bugs** — flagged as `INFO` where relevant. They should be re-checked against a live cluster.

**Severity:** `P1` functional bug / missing core capability · `P2` notable design divergence or missing UI feature · `P3` minor/cosmetic · `INFO` not-a-bug context.

---

## Screen ↔ design map

| Route | `design.pen` frame(s) | Verdict |
|---|---|---|
| `/login` | `N1GkB` | divergent (card, links) + design has privacy issue |
| `/` (nav "Dashboard") | `joEfX` Dashboard Home | **not implemented** (shows servers list) |
| `/servers` (nav "Servers") | — (no dedicated design) | duplicate of `/` |
| `/servers/$name` Overview | `3AL5u` | mostly matches; no sparklines |
| · Console | `IH0A9` | divergent + **runtime bug** |
| · Logs | `Jo9Fx` | heavily simplified |
| · Files | `Cw94C` | matches |
| · Players | `RgHTM` | heavily simplified |
| · Backups | `4avx0` | no summary tiles |
| · Settings | `5gdHG` | matches |
| `/servers/new` | `v2AEV`,`CSdAH`,`GqrKS`,`AVJRp`,`wdVEa` | steps 3 & 4 incomplete |
| `/modules` | `kK8Ji`, `EOqzy` | matches; category filter not impl |
| `/cluster` | `0cdM6` | matches |
| `/users` | `Nf6Up`, `f3OG1` | matches; fewer columns |
| `/admin` | `GnLZY` | matches |
| `/admin/audit` | `KSoe7` | functional; semantics differ |
| `/backups` | `EHR5h` | matches |

---

## Runtime bugs

### B1 `P1` — Console tab throws a recurring xterm exception
On the **Server Detail → Console** tab, the console logs an uncaught exception (observed 4×, on mount / resize / WS reconnect):

```
TypeError: Cannot read properties of undefined (reading 'dimensions')
  at get dimensions (@xterm/xterm … :1885)
  at Viewport.syncScrollArea (@xterm/xterm … :831)
```

`web/src/routes/tabs/Console.tsx` calls `fit.fit()` immediately after `term.open()` (both `RconConsole` and `PtyConsole`), and on every `window.resize`. The fit/scroll-sync runs before the xterm renderer has dimensions (or after the terminal is being disposed on tab switch / Strict-Mode double-mount), so `_renderService.dimensions` is undefined. Needs: guard `fit()` behind a mounted/ready check (e.g. rAF after layout, ResizeObserver, or `if (term.element?.isConnected)`).

### B2 `P2` — Console reconnect storm
With no reachable agent, the console socket loops `— connected —` → `agent unreachable` → `— disconnected —` continuously (visible as repeated lines filling the terminal). `openWS` (`web/src/lib/ws.ts`) appears to reconnect with no backoff/cap on this path. Risk: hammers the API/WS endpoint and produces an unreadable console. Needs exponential backoff + a "disconnected, retrying…" UI state instead of spamming the buffer. (Likely related to B1's repeated firing.)

### B3 `P2` — "Dashboard" and "Servers" are the same page
`web/src/router/tree.tsx` maps **both** `/` and `/servers` to `DashboardPage` (the servers list). Result: the sidebar "Dashboard" item is highlighted while the page heading reads "Servers", and there are two nav entries that go to identical screens. The designed **Dashboard Home** overview (see M1) is absent. Decide: build Dashboard Home at `/` and keep the list at `/servers`, or drop the duplicate nav item.

---

## Major design mismatches (missing UI vs `design.pen`)

### M1 `P2` — Dashboard Home (`joEfX`) not implemented
The design's landing page is a **fleet overview**: 4 summary stats, a **Fleet health** stacked bar (running/starting/stopped/errored), a **Needs attention** panel, a **Recent activity** feed, and a **Quick actions** card. The live `/` shows the servers table instead. None of those four panels exist. (See B3.)

### M2 `P2` — Server Detail · Logs (`Jo9Fx`) heavily simplified
Design = rich log viewer: **log-volume histogram**, **level filter pills (INFO/WARN/ERROR/DEBUG)**, **time-range selector ("Last 1 hour")**, and **structured rows** (timestamp · colored level badge · source · message).
Live (`tabs/Logs.tsx`) = raw virtualized text tail with a *Container output / Game log* source toggle + a free-text filter + line count + download. No histogram, no level parsing, no time range, no structured rows.

### M3 `P2` — Server Detail · Players (`RgHTM`) heavily simplified
Design = **5 summary stat tiles** (online/unique/whitelisted/banned/total), **Online / Whitelisted / Banned tabs**, **whitelist management** ("Add to whitelist"), and a table with **ROLE / PING / LOCATION / SESSION** columns.
Live (`tabs/Players.tsx`) = simple "X / Y online" header + online list with kick/ban + a "show banned" toggle. No stat tiles, no whitelist concept, no role/ping/location/session columns. *(Some columns may be infeasible over RCON — confirm scope; the whitelist tab and stat tiles are the clear gaps.)*

### M4 `P2` — Server Detail · Backups tab (`4avx0`) missing summary
Design = **4 summary stat tiles** (last backup / next backup / count / total size) + a schedule banner, then the backups table.
Live (`tabs/Backups.tsx`) = Schedules / Backups / Restores sections with full functionality but **no summary tiles / schedule banner**.

### M5 `P2` — Create Server · Step 3 Network (`GqrKS`) incomplete
Design = Expose type cards **+ a Port-mappings table** (per-port protocol TCP/UDP + enable toggles) **+ an IP allowlist (CIDR)** field.
Live (`CreateServer.tsx` → `Network`) = only the 3 Expose cards + an optional Hostname input. **No port-mappings table, no CIDR allowlist.**

### M6 `P2` — Create Server · Step 4 Review (`AVJRp`) incomplete
Design = "Review & Launch" with **per-section Edit links**, a **dry-run / "Schedulable" validation banner**, and an **"I accept the EULA for this game" checkbox**, alongside the generated-spec YAML.
Live (`Review`) = a plain key/value summary table + a JSON dump of template config. **No Edit links, no dry-run banner, no EULA checkbox.**

### M7 `P2` — Create Server · Step 1 Template (`CSdAH`) divergent
Design = **search field + category filter pills (All / Survival / Sandbox / Shooter)** above the template grid.
Live = no search, no category pills; instead adds a **YAML-preview + "Memory tip" right panel** that isn't in the design. (Template count 2-vs-8 is mock data — `INFO`.)

### M8 `P2` — Server Detail · Console (`IH0A9`) chrome missing
Design = terminal **header toolbar** (● LIVE indicator, latency, clear / download / fullscreen actions) **+ a dedicated command-input bar** ("Type a command…").
Live (`tabs/Console.tsx`) = bare xterm in a bordered box; input is typed directly into the terminal; **no header toolbar, no separate command input.**

### M9 `P3` — Server Detail · Overview (`3AL5u`) metric tiles flat
Design CPU/Network tiles show **sparkline mini-charts**. Live `MetricTile` (`tabs/Overview.tsx`) renders a flat progress bar only; the **Network tile has no bar at all** (no `progress` prop). Otherwise the Overview layout (header, 2-column, Connection/Players/Status/Actions cards, Recent events) matches.

### M10 `P3` — Modules (`kK8Ji`) category filter not implemented
Design filters by **game category** (Survival/Sandbox/Shooter…). Live (`Modules.tsx`) filters by **source name** (All sources / upstream). Card layout, status badges (Available/Installed), and Install/Deploy/Uninstall all match. *(The "Upload module" button exists in code but is gated on an upload-type source — see I3.)*

### M11 `P3` — Users & RBAC (`Nf6Up`) fewer columns + missing quick link
Design header has an **"Audit log" quick button** (live has only "Invite user"). Design table shows **email under each name, a Last-seen column, and status (Online/pending/suspended)**; live shows USER (name + role text) / ROLE / PROVIDER / CREATED only. Tabs, role badges, and Invite flow match.

### M12 `P3` — Audit log (`KSoe7`) semantics differ
Design presents **human-readable actions** with **actor avatars + email** and **Allowed/Denied** status. Live (`AuditLog.tsx`) presents the raw HTTP view: TIME / ACTOR (text) / METHOD / PATH / HTTP status code / IP. Live *adds* method + actor filters and 2xx/4xx/5xx pills (good). Consider mapping method+path → friendly action labels.

---

## Minor / cosmetic

- **C1 `P3` — Login card missing.** Design `N1GkB`/`APD3Q` wraps the form in an elevated, bordered `--card` panel. Live (`Login.tsx`) renders the form directly on `bg-background` with no card framing.
- **C2 `P3` — Login affordances.** Design shows a **"Forgot?"** link by the password label and a **show/hide (eye)** icon in the password field. Live has neither. *(Confirm whether password reset exists at all.)*
- **C3 `P3` — Brand mark.** App-wide logo is a lucide **ShieldCheck** icon; the design uses an orange **"K"** mark (login, sidebar). Pick one.
- **C4 `P3` — Wizard step 1 button.** Live shows a disabled **"Back"**; design shows **"Cancel"** on step 1.
- **C5 `P3` — Admin Settings layout.** Live uses a left sub-nav showing **one section at a time**; design (`GnLZY`) reads as a **single scrolling page** with all sections stacked. Live also adds a **"Module sources"** sub-section not in the design.
- **C6 `P3` — Backups "Back up now".** Live renders it as an inline section (server dropdown + Run snapshot); design puts a **"Back up now"** button top-right.

---

## INFO — context, not bugs

- **I1 — `design.pen` Login violates the project's own pre-auth privacy rule.** Frame `N1GkB` puts the cluster name **"homelab-01"**, version **"v0.1.0-alpha"**, and hostname **"api.kestrel.homelab-01"** on the login screen (subtitle "Welcome back to homelab-01" and footer). The live `Login.tsx` *correctly* omits all of these (and has a comment enforcing it). **Action: fix the design, not the code** — and don't "restore" these strings when closing M1/M-series gaps.
- **I2 — Mock-data blanks.** CPU/Memory/Network 0, Connection host/port `—`, empty Recent events, single cluster node, 2 templates/modules, 2 backups — all MSW fixtures. Re-verify metrics, sparkline data, and the servers-table CPU/MEM/NODE columns (all `—` in mock) against a live cluster.
- **I3 — Modules "Upload module".** Present in `Modules.tsx` but only rendered when an `upload`-type module source exists; the mock only has OCI sources, so it's hidden — not missing. The `EOqzy` design (Sources & Upload dialogs) should be cross-checked once an upload source exists.
- **I4 — Cluster count mismatch.** Header reads "3/3 nodes healthy" but only **one** node card renders — mock inconsistency between `cluster/stats` and `cluster` (node list). Confirm on live data.
- **I5 — Hidden "Mods" tab.** `ServerDetail.tsx` defines a `mods` tab not present in the design; it's filtered out unless the template advertises mods capability. Confirm whether a Mods design is owed.

---

## Verified in parity (no action)

- **Server Detail · Settings** — all 7 design sub-sections present (General / Resources / Networking / Env vars / Lifecycle / Access / Danger zone).
- **Server Detail · Files** — file navigation, Monaco editor, New file / New folder / Upload / Download / Delete / Save, dirty-state + confirm dialogs. *(Minor: live uses cwd folder-navigation vs the design's expandable tree.)*
- **Create Server · Step 2 Configure** — name, description, CPU/Mem, storage, node placement, + dynamic template-config fields. Matches `v2AEV`.
- **Create Server error states** (`wdVEa`) — name-conflict, not-permitted, create-failed all handled (`errorMessage()`).
- **Cluster** — header + Download kubeconfig / Add node; node card stats + CPU/Mem bars; per-node Cordon/Drain actions exist in `Cluster.tsx`.
- **Backups index**, **Admin Settings sub-nav**, **Users tabs + role badges**, global **sidebar/topbar** shell.

---

## Suggested fix priority for planning

1. **B1 / B2** — Console crash + reconnect storm (real, user-visible runtime bugs).
2. **B3 / M1** — Decide Dashboard Home vs Servers list; build the missing overview or de-duplicate nav.
3. **M5 / M6** — Create Server completeness (port mappings, CIDR allowlist, EULA, dry-run, edit links) — these block correct server creation UX.
4. **M2 / M3 / M4** — Server Detail Logs / Players / Backups feature parity (histograms, level filters, whitelist, summary tiles).
5. **M7–M12, C1–C6** — Search/filters, console chrome, sparklines, login card, brand, audit semantics.
6. **I1** — Correct the `design.pen` login frame to remove pre-auth cluster/version/host leakage.

---

### Notes on test method reliability
- Live screenshots are sound; capture is reliable only when the tab is foregrounded immediately before each shot (pattern used: a `navigate` precedes every screenshot in a batch). DOM/structure was cross-checked with `read_page` (focus-independent) and component source, so findings don't rely on pixels alone.
- Responsive/small-viewport behavior was **not** exercised (all captures at 1440-wide). Worth a separate pass.
- Only the mock API was used; anything requiring a live agent (real console I/O, live metrics/sparklines, multi-node cluster) is explicitly called out as needing live-cluster verification.
