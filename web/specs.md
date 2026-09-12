# web — Specification

**Status:** beta (v0.2.0-beta.8)  
**Module / package:** @gameplane/web  
**Build:** Vite 5.4 + React 18.3 + TypeScript 5.6 (strict)

## Purpose

The Gameplane dashboard is a React SPA providing a UI layer over the Gameplane API. It exposes Kubernetes GameServer/GameTemplate/Backup/Module operations through browser-based workflows: server creation and lifecycle, mod/player management, file browsing, backup scheduling, multi-cluster routing, RBAC configuration, and audit trails. All persistence and reconciliation logic lives in the operator and API; the dashboard is a pure UX adapter.

## Responsibilities

- Render all authenticated Gameplane CRDs and operations as React pages and interactive forms
- Handle user authentication (local + OIDC) and session management (cookies + CSRF tokens)
- Fetch and cache data via TanStack Query; invalidate caches on Kubernetes watch events (SSE)
- Stream console input/output (WebSocket) and pod/game logs (WebSocket) to the Console and Logs tabs
- Wrap the API client in three layers: thin fetch wrapper (`api<T>()`), typed endpoint namespaces (`Servers`, `Templates`, `Cluster`, etc.), and domain helpers (validations, RCON mode resolution, mod/modpack capability detection)
- Thread multi-cluster context through all API requests via `?cluster=` query params (local cluster omitted for back-compat)
- Enforce role-based access control (RBAC) on frontend routes via `<RequirePermission>` middleware
- Validate form inputs against declared server/template schema before submit
- Display real-time resource status (CPU, memory, uptime) and live action metrics from RCON/agent

## Non-goals / Boundaries

**Design-first rule:** Any change to the dashboard's visual surface (new page, new form field, layout shift, color/icon change) originates in `design.pen` (Pencil MCP server), not in React code. The Pencil file is the source of truth for all UI design; code-led redesigns are reverted. See docs/architecture.md and CLAUDE.md rule 1.

**API-only:** The dashboard never speaks directly to the operator or agent. All reads/writes flow through the Kubernetes API server and the REST/WebSocket API gateway (`api/`).

**Pre-auth privacy:** The login page and any unauthenticated screen must not leak internal state: no hostnames, cluster names, server counts, version strings, user lists, or "user not found" errors. See docs/security.md "pre-auth privacy" and CLAUDE.md rule 3.

**Local-cluster WebSocket:** Console and Logs streams currently route only to the local cluster (`?cluster=` param is not threaded through WebSocket paths). Cross-cluster WebSocket support is deferred; see `docs/roadmap.md`.

## HeroUI Component Layer

The dashboard's visual surface composes twelve foundational atom components built on `@heroui/react` 3.2.4 primitives and Gameplane brand design tokens. These atoms reside in `web/src/components/hero/` and form the basis for all rebuilt screens:

- **StatCard** — displays a metric with label, value, and optional trend indicator
- **PhaseChip** — status badge rendering server phase (Running/Stopped/Pending/Failed) with semantic color
- **PageHeader** — page title, description, and action button slot
- **ConfirmDialog** — high-stakes confirmation modal with warning styling
- **DropdownMenu** — contextual menu with keyboard navigation and icon support
- **FilterPopover** — filter/search popover with form controls
- **LoadingCard** — skeleton placeholder during data fetch
- **ErrorCard** — error state card with message and recovery action slot
- **ErrorBanner** — top-of-page alert for form/request errors
- **Meter** — horizontal progress indicator with percentage display
- **Sparkline** — mini inline chart for resource trends (CPU, memory)
- **GameIcon** — cached game-specific icon with fallback

Each atom composes HeroUI's headless react-aria-components (Adobe React Aria) and applies Gameplane's brand palette — orange accent, dark default mode, light mode supported — via HeroUI's semantic token layer. Token mapping from Gameplane brand values to HeroUI variables is documented in `specs/014-heroui-web-rebuild/contracts/theme-tokens.md` (FR-013).

During the multi-slice rebuild transition, the previous Radix-based primitives remain in `web/src/components/ui/` and are used only by un-rebuilt screens. **No single screen mixes the HeroUI (`hero/`) and legacy (`ui/`) component families** — this is enforced mechanically at review (FR-012: any rebuilt file importing from `components/ui/` fails).

## T057 — Login Privacy Compliance (FR-005)

**Verdict: PASS.** The login page (`web/src/routes/Login.tsx`) and all pre-auth hero components comply with FR-005 (login privacy).

**Audit findings:**

1. **Login.tsx pre-auth surface (lines 26–233):**
   - ✅ No cluster name, version string, or hostname rendered
   - ✅ No server count, deployment metrics, or internal status
   - ✅ No user enumeration signals (error copy at line 94 is neutral: "Invalid credentials" or "Network error" only)
   - ✅ Marketing panel (lines 196–231) contains static product copy only (no internal metrics)
   - ✅ Explicit comment (lines 23–25) guards this rule at source

2. **HeroUI components in pre-auth context:**
   - Only `Input`, `Label`, `InputGroup` from `@heroui/react` are used before auth (lines 17–18)
   - No other hero/* components are rendered pre-auth (AppLayout, Sidebar, TopBar, etc. all require `useMe()`, which 401-redirects unauthenticated users)
   - All HeroUI imports follow FR-012 (from "@heroui/react" only; no radix-ui/ui/CVA leakage)

3. **SSO button labels (SSOButtons, lines 249–267; MarketingRow, lines 269–278):**
   - Provider display names come from the pre-auth `Auth.providers()` API response (`p.label`, line 262 in SSOButtons)
   - Never issuer URLs or internal identifiers
   - Compliant with the login-privacy rule

**Conclusion:** No metrics, hostnames, versions, cluster names, or enumeration hints leak on the unauthenticated surface. Error copy is neutral and user-facing. The pre-auth privacy invariant (CLAUDE.md rule 3) is satisfied.

## T058 — Slice 1: Shell + Login Architecture

**Scope:** Slice 1 (tasks T040–T053, feature 014) rebuilds the authenticated shell (AppLayout, Sidebar, TopBar) and the login page on HeroUI, establishing the visual layer for all downstream screens. Old Radix-based primitives remain in `web/src/components/ui/` until slices 2–5 migrate their respective screens.

### Components Added (T045–T052)

**New composition root:**
- **`AppLayout.tsx` (T042)** — layout orchestrator: composes `AppShell` (layout), `Sidebar` (fixed or drawer variant), `TopBar` (breadcrumbs + cluster selector + search + notifications), and the authenticated page outlet. Owns `useMe()`, 401-redirect logic, permission gating for nav items (admin/operator/viewer), `useClusterInfo()` (cluster stats cache), and `useTheme()` integration for appearance mode.

**New layout atoms (T045–T048, T050, T052):**
- **`hero/AppShell.tsx` (T045)** — pure layout wrapper: renders sidebar fixed-width, topbar + main content in flex column. No mobile logic (owned by Sidebar's drawer variant).
- **`hero/Sidebar.tsx` (T046, T052)** — left navigation: fixed variant (always visible on desktop) or drawer variant (mobile off-canvas). Renders nav groups + items, active route highlighting, appearance toggle footer, and user-info footer with logout. Permission gating is computed by AppLayout; Sidebar renders whatever nav items it receives.
- **`hero/TopBar.tsx` (T047)** — horizontal header: hamburger (mobile), breadcrumb slot, cluster selector slot, global search slot, notifications slot, user menu (avatar + logout). All four slots are ReactNode — each slot component fetches its own data (no centralized fetching in TopBar).
- **`hero/Breadcrumbs.tsx` (T048)** — route-hierarchy breadcrumbs: renders the tree path (gameplane / Servers / my-server) using `buildCrumbs` logic kept from the old AppLayout. Distinct from `hero/PageHeader.tsx`'s internal page-level breadcrumbs.
- **`hero/NotificationsPanel.tsx` (T049)** — bell icon + popover + SSE notification list: owns the `openEventStream` subscription and local state (same as today's `Notifications()` in AppLayout). Fetching moved into the component, not centralized in AppLayout.
- **`hero/GlobalSearch.tsx` (T050)** — search field + results popover: owns the `useQuery` call for servers list and client-side filter. Kept from today's AppLayout.
- **`hero/AppearanceToggle.tsx` (T052)** — three-state appearance mode selector (light/dark/system): dispatches `onChange` to parent (AppLayout), which owns the `useTheme()` hook. Uses HeroUI `ToggleButtonGroup` or fallback three-button set.

**Refactored on HeroUI:**
- **`PageHeader.tsx` (T043)** — route-level page header: thin wrapper around `hero/PageHeader` (slice-0 atom), passing through `title/subtitle/actions/breadcrumbs` unchanged. Called by ~30+ route pages; no changes required in call sites.
- **`ClusterSelector.tsx` (T044)** — multi-cluster dropdown: refactored from `DropdownMenu` to HeroUI `Select`, keeping all permission/state logic, phase color mapping, and "Add cluster" action.
- **`Login.tsx` (T040)** — login form: refactored from raw DOM to HeroUI `TextField`/`Label`/`Input`/`InputGroup`, `Alert` for errors, `Button` for actions. Kept all state, error handling, SSO provider rendering, and marketing panel. Verified compliant with FR-005 (login privacy).
- **`Dashboard.tsx` (T041)** — landing page loading + empty state: narrowed scope to loading skeleton and empty frame only (full content deferred to slice 2+). Keeps the same cache keys and queries so downstream slices reuse prepared data.

### Design Import Rule (FR-012)

Every file touched in slice 1 imports **only** from:
- `@heroui/react` (HeroUI components)
- `@/components/hero/` (new slice-1 atom components)

**Forbidden imports:**
- `@/components/ui/*` (legacy Radix-based primitives)
- `@radix-ui/*` (raw Radix)
- `class-variance-authority` (replaced by HeroUI's variant system)

This rule is enforced by lint and review: any rebuilt file importing from forbidden sources fails CI.

### Theme Integration & Boot Script (T053)

**Appearance selection:** Two mechanisms work together to avoid theme flicker on page load:

1. **Boot script in `index.html`** (T053, lines ~398–411):
   - Runs as the first script in `<head>` (before Google Fonts)
   - Reads `localStorage.getItem("gameplane-theme")` — must match the key HeroUI's `useTheme()` persists
   - Resolves system theme via `window.matchMedia("(prefers-color-scheme: dark)")`
   - Sets both `document.documentElement.classList` (add/remove "dark" and "light") and `document.documentElement.dataset.theme` (for HeroUI token fallback)
   - Gracefully handles localStorage unavailability; keeps the dark default from markup

2. **`AppearanceToggle.tsx` + `AppLayout.tsx` (T052):**
   - AppLayout calls `useAppearance()` (custom hook) to get theme state and setter
   - Passes `theme` and `setTheme` as props to Sidebar
   - Sidebar renders `AppearanceToggle` with `value` and `onChange` callbacks
   - On toggle, AppearanceToggle calls the `onChange` callback (wired to `setTheme`)
   - `setTheme` syncs to localStorage; `useAppearance()` calls `applyTheme()` via useEffect on every theme change
   - `applyTheme()` (web/src/components/AppLayout.tsx lines 48–57) explicitly sets both `document.documentElement.classList` (adds/removes dark/light) and `document.documentElement.dataset.theme = resolved`, ensuring consistent theme application without requiring a separate hook from HeroUI

**Shipped default:** `<html class="dark" data-theme="dark">` in markup (dark theme as the no-JavaScript fallback; overridden by boot script if a stored preference exists).

### Test Count Rule (FR-010)

Each rewritten test file must maintain or exceed the original test count:
- `Login.test.tsx`: ✅ ported all cases (new file)
- `AppLayout.test.tsx`: ✅ 29 tests (old file had 28), porting every case including permission gating for nav items and 401-redirect behavior

Selectors updated to query by role (`getByRole("textbox", { name: /username/i })`, `getByRole("button", { name: /sign in/i })`, `getByRole("alert")`) since HeroUI markup changes internal DOM structure. No `data-testid` added unless HeroUI components genuinely lack accessible role/name.

### Playwright Specs TypeScript Check

**Status:** Clean — `npx tsc --noEmit` passes with no errors across all Playwright specs (`e2e/specs/*.spec.ts`, `e2e/globalSetup.ts`, `e2e/globalTeardown.ts`, `e2e/pages/*.ts`).

### Legacy Primitives Still in Use

Until slices 2–5 migrate their screens, the old Radix-based `ui/` components remain:
- `web/src/components/ui/button.tsx`
- `web/src/components/ui/card.tsx`
- `web/src/components/ui/input.tsx`
- `web/src/components/ui/dialog.tsx`
- `web/src/components/ui/select.tsx`
- `web/src/components/ui/tabs.tsx`
- (and ~20+ others)

These are used only by Servers, ServerDetail, Modules, Cluster, Users, AdminSettings, AuditLog, AdminLogs, Backups pages — which remain un-rebuilt until their respective slices. **No single screen mixes both families** (enforced by import rule FR-012).

### Deviation Notes

1. **AppearanceToggle three-state vs. binary Switch (OD-7, decision #5):** Task T052 text suggests "using HeroUI Switch", but the contract specifies a three-state control (light/dark/system). Implementation uses HeroUI `ToggleButtonGroup` if available; if not, falls back to three `Button` elements with `isSelected` state. Binary `Switch` would discard the system-preference state — flagged as a deviation in the PR if justification is needed.

2. **Dashboard.tsx narrowed scope (decision #6):** Task T041's literal wording ("displaying app-loading state and empty dashboard frame") narrows this slice's work to loading + empty states only. Full stat/table dashboard content is deferred; a `// TODO(slice-2+)` comment marks where it would be added. Flagged in PR description since this is a visible reduction from today's 470-line Dashboard.tsx.

## T066–T088 — Slice 2a: Servers List and ServerDetail Screens

**Scope:** Slice 2a (tasks T066–T088, `specs/014-heroui-web-rebuild/tasks.md`) rebuilds two core screens — the Servers list page and the ServerDetail (full server view with tabbed interface) — on HeroUI components. This is the primary user-facing surface after login, showing all game server instances and their live status. The slice composes existing hero/ atoms from slice 0 with new HeroUI form/table/dialog components.

### Screens and Routes

1. **Servers Page** (`web/src/routes/Servers.tsx`, 651 lines)
   - List all GameServers in a responsive table (desktop ≥768px) or stacked cards (mobile <768px)
   - Renders server phase, game icon, player count, resource usage, and uptime via StatCard compositions
   - Filter by phase (Running/Stopped/Pending/Failed), game template, and namespace via FilterPopover
   - Inline lifecycle buttons (play/stop/restart) using HeroUI Button variants
   - Server count summary via StatCard at the top; search box using HeroUI Input
   - Action menu (clone, transfer, wipe, delete) via ServerActionsMenu composition (uses DropdownMenu)

2. **ServerDetail Page** (`web/src/routes/ServerDetail.tsx`, 289 lines)
   - Full server view with header showing phase chip, uptime, player count, and action buttons
   - Tabbed interface (HeroUI Tabs) routing to nine sub-views: Overview, Events, Console, Logs, Files, Mods, Modpacks, Players, Backups, Capture, Settings
   - Phase chip from hero/PhaseChip; game icon from hero/GameIcon
   - Lifecycle action buttons (start/stop/restart) gated by phase state

### Design-Imported Compositions

**Slice 2a uses the following hero/ atom compositions from slice 0** (line numbers below are the `import` line in each file, verified by grep against the current tree):

- **StatCard** — metric display (players online, server uptime, resource usage gauges) in Servers list summary and Overview tab (`web/src/routes/tabs/Overview.tsx` line 7, `web/src/routes/Servers.tsx` line 23)
- **PhaseChip** — server phase badge (Running/Stopped/Pending/Failed) in Servers list rows and ServerDetail header (`web/src/routes/Servers.tsx` line 24, `web/src/routes/ServerDetail.tsx` line 16)
- **FilterPopover** — filter controls in Servers list (phase, template, namespace) (`web/src/routes/Servers.tsx` line 25)
- **GameIcon** — cached game-specific icon in Servers list rows and ServerDetail header (`web/src/routes/Servers.tsx` line 26, `web/src/routes/ServerDetail.tsx` line 17)
- **DropdownMenu** (`DropdownMenu`, `DropdownMenuContent`, `DropdownMenuItem`, `DropdownMenuSeparator`, `DropdownMenuTrigger`) — context menu (clone/transfer/wipe/delete) via ServerActionsMenu (`web/src/components/server/ServerActionsMenu.tsx` lines 12–17)
- **ConfirmDialog** — used in **DeleteServerDialog.tsx** (`web/src/components/server/DeleteServerDialog.tsx` line 2) and the file-delete confirmation in Files (`web/src/routes/tabs/Files.tsx` line 35). CloneServerDialog, TransferServerDialog and WipeServerDialog do **not** use hero/ConfirmDialog — they build their own dialogs directly from HeroUI `Modal`/`AlertDialog` primitives (see Server Component Helpers below).
- **ErrorBanner** — form/request error display in Files and Players tabs (`web/src/routes/tabs/Files.tsx` line 36, `web/src/routes/tabs/Players.tsx` line 16)
- **ErrorCard** — error state display in Console tab ("No console available") (`web/src/routes/tabs/Console.tsx` line 5)
- **LoadingCard** — skeleton placeholder in Console tab while data loads (`web/src/routes/tabs/Console.tsx` line 4)
- **Sparkline** — mini inline CPU/memory/disk charts in Overview tab (`web/src/routes/tabs/Overview.tsx` line 8)

**HeroUI Components imported directly** (component-name lists below are taken verbatim from each file's `@heroui/react` import, not restated from memory):

- **Button** — from `@heroui/react`, imported in Servers.tsx (line 22), ServerDetail.tsx (line 4), Events.tsx (line 3), Logs.tsx (line 4), Players.tsx (line 14), ServerActionsMenu.tsx (line 9), and the dialog/card files below
- **Card, CardHeader, CardTitle, CardContent, CardFooter, Alert** — from `@heroui/react`, imported together in Overview tab (`web/src/routes/tabs/Overview.tsx` line 6)
- **Input** — from `@heroui/react` (search box in Servers list, player filter in Players tab, file/folder name fields in Files) (`web/src/routes/Servers.tsx` line 22, `web/src/routes/tabs/Players.tsx` line 14, `web/src/routes/tabs/Logs.tsx` line 4, `web/src/routes/tabs/Files.tsx` line 17)
- **Chip, Tabs, Tab, Table** — from `@heroui/react` (`web/src/routes/Servers.tsx` line 22: `Button, Card, Input, Chip, Tabs, Tab, Table`; `web/src/routes/ServerDetail.tsx` line 4: `Button, Tabs, Tab`)
- **Modal, ModalBackdrop, ModalContainer, ModalDialog, ModalHeader, ModalBody, ModalFooter** — from `@heroui/react` (file create/folder dialogs in Files tab, `web/src/routes/tabs/Files.tsx` lines 9–19; also used, with `ModalHeading`/`Label`/`Description`/`FieldError` added, in `CloneServerDialog.tsx` lines 4–17, `TransferServerDialog.tsx` lines 3–16 with `ListBox`/`ListBoxItem`/`Popover`/`PopoverTrigger`/`PopoverContent` instead of form fields, and `ServerActionsCard.tsx` lines 20–39)
- **AlertDialog, AlertDialogBackdrop, AlertDialogContainer, AlertDialogDialog, AlertDialogHeader, AlertDialogHeading, AlertDialogBody, AlertDialogFooter, AlertDialogIcon, Checkbox** — from `@heroui/react` (`WipeServerDialog.tsx` lines 3–14)
- **Alert** — from `@heroui/react`, also imported in `ServerSleepCard.tsx` (line 2) for the idle-state summary

### Tab Components (Slice 2a)

Six of the nine ServerDetail tabs are rebuilt in this slice:

1. **Overview** (`web/src/routes/tabs/Overview.tsx`, 411 lines)
   - Composes `ServerStatusCard`, `ServerSleepCard`, `ServerActionsCard`, `EventList` (imports at lines 9–12) for status, sleep state, lifecycle actions, and recent pod events
   - Live metrics: CPU %, memory %, disk % via StatCard + Sparkline (`Sparkline` used at line 332)
   - Uses HeroUI Card/CardHeader/CardTitle/CardContent/CardFooter/Alert (line 6); hero/ StatCard, Sparkline (lines 7–8)

2. **Events** (`web/src/routes/tabs/Events.tsx`, 86 lines)
   - Kubernetes events (image pull, scheduling, crash-loops, agent startup) rendered via `EventList` (`web/src/components/server/EventList.tsx`, imported line 6)
   - Filter state (all/info/warnings) via HeroUI Button
   - Uses HeroUI Button, Card (line 3)

3. **Console** (`web/src/routes/tabs/Console.tsx`, 157 lines)
   - Interactive RCON/PTY terminal (xterm.js, lazy-loaded, NOT rebuilt — uses existing engine)
   - LoadingCard during template resolution (line 28); ErrorCard if no console available (line 35)
   - Uses hero/ LoadingCard, ErrorCard only (lines 4–5); console I/O engine unchanged

4. **Logs** (`web/src/routes/tabs/Logs.tsx`, 299 lines)
   - Pod stdout or configured game log file stream (WebSocket, NOT rebuilt — uses existing engine)
   - Search/filter input via HeroUI Input; download button
   - Virtualized log viewer (TanStack Virtual, NOT rebuilt)
   - Uses HeroUI Button, Input only (line 4); log streaming engine unchanged

5. **Files** (`web/src/routes/tabs/Files.tsx`, 602 lines)
   - File browser and editor (Monaco, lazy-loaded, NOT rebuilt)
   - Create folder/file dialogs via HeroUI Modal/ModalBackdrop/ModalContainer/ModalDialog/ModalHeader/ModalBody/ModalFooter (lines 9–19)
   - Delete confirmation via hero/ ConfirmDialog (line 35, used at line 408)
   - Error display via hero/ ErrorBanner (line 36, used at line 261)
   - Monaco editor unchanged

6. **Players** (`web/src/routes/tabs/Players.tsx`, 386 lines)
   - Online player snapshot, ban list, whitelist management
   - Player count summary via hero/ StatCard (line 15)
   - Kick/ban/unban actions with reason input via HeroUI Input (line 14)
   - Error display via hero/ ErrorBanner (line 16, used at line 131)

**Three tabs remain un-rebuilt (used only from legacy primitives or not yet touched):**

- **Mods** — unchanged from slice 0 (not in scope)
- **Modpacks** — unchanged from slice 0 (not in scope)
- **Backups** — unchanged from slice 0 (not in scope)
- **Settings** — unchanged from slice 0 (not in scope)

### Console and Logs Engines

Console input/output and Logs streaming use existing bidirectional WebSocket (console) and read-only WebSocket/SSE (logs) plumbing via `web/src/lib/ws.ts` and `web/src/lib/sse.ts`. These engines are **not rebuilt** in any slice; only the loading/error UI wrapper changes (LoadingCard, ErrorCard in slice 2a). The consumer-facing API (`openWS()`, `openEventStream()`) remains unchanged.

### Server Component Helpers

**Slice 2a adds/updates helper components in `web/src/components/server/`** (line counts are the file's current total, since internal structure shifts with every edit and precision there is not load-bearing):

- **ServerActionsMenu.tsx** (116 lines) — Dropdown menu context menu (clone/transfer/wipe/delete) using hero/ DropdownMenu (import lines 12–17, usage lines 48–86); wires the four dialogs below
- **CloneServerDialog.tsx** (138 lines) — Clone form (name, description, template selector) built directly from HeroUI `Modal`/`ModalBackdrop`/`ModalContainer`/`ModalDialog`/`ModalHeader`/`ModalHeading`/`ModalBody`/`ModalFooter`/`Button`/`Input`/`Label`/`Description`/`FieldError` (import lines 4–17) — not hero/ConfirmDialog
- **TransferServerDialog.tsx** (143 lines) — Transfer form (destination picker) from the HeroUI `Modal` family plus `ListBox`/`ListBoxItem`/`Popover`/`PopoverTrigger`/`PopoverContent` (import lines 3–16) — not hero/ConfirmDialog
- **WipeServerDialog.tsx** (123 lines) — Wipe-world confirmation from the HeroUI `AlertDialog` family plus `Checkbox` (import lines 3–14) — not hero/ConfirmDialog
- **DeleteServerDialog.tsx** (52 lines) — Delete server confirmation via hero/ ConfirmDialog (line 2)
- **ServerStatusCard.tsx** (73 lines) — Status summary card in Overview tab, built on HeroUI Card (line 3)
- **ServerActionsCard.tsx** (487 lines) — Lifecycle action card in Overview tab; HeroUI Button/Card/CardHeader/CardContent/Modal family/Input/Label/Select/ListBox/ListBoxItem/Checkbox/Description/FieldError (import lines 20–39)
- **ServerSleepCard.tsx** (170 lines) — Server sleep/idle state summary; HeroUI Card/Alert (line 2) plus a `Chip` re-exported from hero/PhaseChip (line 6)
- **EventList.tsx** (43 lines) — Kubernetes event list renderer, used from both Events.tsx (line 6) and Overview.tsx (line 12)
- **PortOverridesEditor.tsx** (80 lines) — Port configuration helper; HeroUI Input/Button (line 1) (Settings tab, deferred)

### Design Import Rule (FR-012)

Every file in slice 2a follows the import rule:

- ✅ Imports **only** from `@heroui/react` and `@/components/hero/` (no `@radix-ui/*`, no `@/components/ui/*`)
- ✅ No mixed families within a single file
- ✅ Verified by grep: `grep -rE '@/components/ui/|@radix-ui' web/src/routes/{Servers,ServerDetail}.tsx web/src/routes/tabs/{Overview,Events,Console,Logs,Files,Players}.tsx web/src/components/server/ 2>/dev/null` returns **zero results**

**Mechanics:** Lint and review enforce the rule; any rebuilt file importing from forbidden sources fails CI.

### Test Count Rule (FR-010)

Each rewritten test file maintains or exceeds the original test count:

- `Servers.test.tsx`: ported filter state, search, lifecycle action, and phase-counting cases
- `ServerDetail.test.tsx`: ported tab switching and lifecycle action cases
- `Overview.test.tsx`: ported metric display and event summary cases
- `Events.test.tsx`: ported event filter and list cases
- `Console.test.tsx`: ported loading/error state cases
- `Logs.test.tsx`: ported log filtering and line rendering cases
- `Players.test.tsx`: ported player action and list cases
- `Files.test.tsx`: ported file browser and editor dialog cases

Selectors updated to query by role (`getByRole("button", { name: /clone/i })`, `getByRole("table")`, `getByRole("tablist")`) since HeroUI markup changes internal DOM structure. No `data-testid` added unless HeroUI components genuinely lack accessible role/name.

### Deviation Notes

CloneServerDialog, TransferServerDialog and WipeServerDialog do not route through hero/ConfirmDialog the way DeleteServerDialog and the Files delete confirmation do — each builds its own dialog directly from HeroUI `Modal`/`AlertDialog` primitives, since their forms need bespoke fields (name/description/template picker, destination picker, wipe checkbox) that hero/ConfirmDialog's fixed layout does not support. This is a real, verified divergence from the atom-reuse framing above, not a workaround pending cleanup — no forbidden-import (`@/components/ui/`, `@radix-ui`) is involved, per the Design Import Rule grep above.

## Directory & Package Layout

```
src/
  main.tsx                  # React entry point; bootstraps QueryClient, Router, MSW
  types.ts                  # TypeScript types mirroring Gameplane CRDs (GameServer, GameTemplate, Backup, etc.)
  router/tree.tsx           # TanStack Router route tree (root, login, app-layout, all pages)
  routes/                   # Page components: Login, Dashboard, Servers, ServerDetail, CreateServer,
                            # Modules, Cluster, Users, AdminSettings, AuditLog, AdminLogs, Backups;
                            # ServerDetail sub-pages (tabs): Overview, Events, Console, Logs, Files,
                            # Mods, Modpacks, Players, Backups, Settings;
                            # Settings sub-sections: General, Version, Resources, Networking,
                            # Environment, Lifecycle, Backups (scheduled), Placement, Access (RBAC), Danger
  components/
    ui/                     # Radix + shadcn-style primitives: button, card, input, select, tabs,
                            # switch, slider, textarea, dialog, confirm-dialog, stat, etc.
    server/                 # Server-detail helpers: ServerActionsMenu, tab components
    backups/                # Backup-flow components: restore wizard, destination selector
    modules/                # Module catalog, install flow, upload preview
    AppLayout.tsx           # Main nav shell, cluster selector, user menu, breadcrumbs
    PageHeader.tsx          # Standardized page title + action buttons
    ClusterSelector.tsx     # Multi-cluster dropdown; threads ?cluster= through API calls
    RequireRole.tsx         # Permission gate middleware; wraps routes needing specific perms
    registry-browser.tsx    # Shared mod registry search/browse UI (Mods + Modpacks tabs)
  lib/
    api.ts                  # Thin fetch wrapper: api<T>(), APIError, csrfHeaders(), cluster threading
    endpoints.ts            # Typed URL-builder namespaces: Servers, Templates, Cluster(s),
                            # Backups, Schedules, Restores, BackupDestinations, Players,
                            # Users, Roles, Auth, AuthProviders, Audit, Notifications,
                            # ModRegistries, Files, Logs, Modules, ModuleSources
    ws.ts                   # openWS(): reconnecting WebSocket helper, exponential backoff
    sse.ts                  # openEventStream(): Server-Sent Events client for /events watch
    cluster.ts              # getCurrentCluster() / setCurrentCluster() for multi-cluster state
    auth.ts                 # Local user session, OIDC provider detection, logout
    capabilities.ts         # Server capability resolution: resolveConsoleMode(), serverHasMods(),
                            # serverHasModpacks() — drives tab visibility
    servers.ts              # Server validation, phase state helpers
    games.ts                # Game-specific logic: icon URLs, console protocol detection
    config.ts               # Global config: API base URL, feature flags, registry provider keys
    errors.ts               # Custom error types
    media.ts                # Image/icon URL helpers
    quantity.ts             # K8s quantity parsing (CPU, memory)
    events.ts               # Event severity / reason formatting
    annotations.ts          # K8s annotation key constants
    verify.ts               # Audit trail verification helpers (hash chain validation)
    destinations.ts         # Backup destination type detection
    validation.ts           # Form field validators (domain names, port ranges, resource specs)
    utils.ts                # capitalize(), formatUptime(), cn(), clsx helpers
  styles/
    globals.css             # Tailwind directives, CSS variables for theme, layout resets
  test/
    setup.ts                # Vitest setup: DOM matchers, MSW worker initialization
    browser-msw.ts          # MSW request handlers for mock e2e tests
    [*.test.tsx]            # Co-located unit + integration tests
e2e/
  [*.spec.ts]               # Playwright tests (mock mode and live mode)
  globalSetup.ts            # Live mode: spawn kubectl port-forward, bootstrap admin, save auth state
  globalTeardown.ts         # Live mode: kill port-forward
  .auth/storage.json        # Session + CSRF cookies persisted for live-mode tests

vite.config.ts              # Build config; dev proxy rules (Accept-header bypass for SPA routes)
tsconfig.json               # TS strict, noUnusedLocals, noUnusedParameters, noFallthroughCasesInSwitch
vitest.config.ts            # Coverage gates (lines 92 / functions 76 / branches 82 / statements 92)
playwright.config.ts        # Mock + live test modes, serial execution (login state is shared)
eslint.config.js            # Flat config: @typescript-eslint (strict), react-hooks, no-floating-promises
package.json                # @gameplane/web v0.2.0-beta.8; dev: vite, npm scripts for build/test/lint
```

## Routing & Pages

**Router:** TanStack Router v1.75 in `src/router/tree.tsx` defines a two-level structure:
- Root route → `<Outlet>`
- Login route (`/login`) → public, unauthenticated
- App layout (`/app-layout`) → contains all authenticated pages

**Top-level Pages:**

1. **Login** (`/login`) → `LoginPage`
   - Public, pre-auth, unauthenticated
   - Local username/password form + OIDC provider buttons
   - No internal metrics/hostnames (privacy rule)

2. **Dashboard** (`/`) → `DashboardPage`
   - Landing page for authenticated users
   - Server count summary, recent events, admin shortcuts

3. **Servers** (`/servers`) → `ServersPage`
   - List all GameServers in a table
   - Filter by namespace, phase (Running/Stopped/Pending/Failed), template
   - Create + clone + delete actions

4. **ServerDetail** (`/servers/$name`) → `ServerDetailPage`
   - Full server view with query param `?ns=<namespace>` support
   - Lifecycle buttons (start/stop/restart) gated on phase state
   - Tabbed interface (below)

5. **CreateServer** (`/servers/new`) → `CreateServerWizard`
   - Multi-step form (template select, version pick, name, config, storage, networking, resources)
   - Supports query param `?template=<name>` to pre-select from Modules page Deploy link
   - Validation on each step
   - If tunnel is enabled with credentials entered at create time, the mutation saves the GameServer first, then saves tunnel credentials to a Kubernetes Secret. If credential-save fails after the server is created, the error is rethrown with `{ cause: err }` to preserve the original error chain, and includes remediation text (the server exists; credential can be set from Networking settings)

6. **Modules** (`/modules`) → `ModulesPage`
   - Merged catalog from all registered ModuleSources + installed Module CRs
   - Browse by game, install from catalog, manage installations, bulk upload
   - **Module Builder** (`BuildModuleDialog.tsx`): 3-step modal wizard for creating, validating, simulating, and packaging custom game modules directly from the dashboard:
     - **Step 1 (Preset & Metadata)**: Choose an archetype preset (`steamcmd`, `java`, `generic`), enter DNS-1123 module name with live validation, display title, summary, and canonical category chips.
     - **Step 2 (Container & Ports)**: Configure container image ref (with digest pinning verification badge), custom TCP/UDP ports with advertise flags, and persistent volume size and mount path.
     - **Step 3 (Review & Export)**: Dual-pane view with syntax-editable YAML manifests (`module.yaml`, `template.yaml`, `README.md`), live offline validation diagnostics, memory scaling simulation (`autoFromMemoryLimit`), and action buttons to download a `.tar.gz` archive or install directly into a cluster upload `ModuleSource`.

7. **Cluster** (`/cluster`) → `ClusterPage` (gated by `servers:write` permission)
   - Cluster health, node list, kubeconfig download
   - Node join credential generation (admin-only, when clusterOps enabled)

8. **Users** (`/users`) → `UsersPage` (gated by `users:manage` permission)
   - Create/edit/delete users and OIDC links
   - Manage role bindings per namespace

9. **AdminSettings** (`/admin`) → `AdminSettingsPage` (gated by `config:manage` permission)
   - Sections: General (version, telemetry), Authentication (OIDC providers), Mod registries (API keys),
     Notification sinks (Discord/Slack/SMTP/webhook), Backup destinations

### Install-Time Settings Display & OIDC Role Mapping Overrides (AdminSettings & Cluster split)

**Pattern:** Install-time cluster configuration (set via Helm values at deploy time) is read-only in the dashboard. These settings are **never dashboard-editable**; they originate from `installTimeSettings` injected via the API's `GET /admin/config` response.

**Data source:** `GET /admin/config` returns an optional `installTimeSettings` object (absent when there is nothing to report) containing:
- `gameDataStorageClass` — the `operator.gameDataStorage.storageClassName` Helm value (a plain string; never null when the object is present)
- `oidcHelmProvider` — OIDC provider metadata: `groupsClaim` (string), `defaultRole` (string), and `roleMappings` (the **Helm-seeded** role mappings as `{ admin?, operator?, viewer? }`, each role key optionally present)

**Split display across two routes (not one):**
1. **Cluster.tsx** displays the **storage class card** — `gameDataStorageClass` value or "Cluster default" badge if unset; rendered with a contextual hint when unset
2. **AdminSettings.tsx** displays the **OIDC provider and role mappings overrides** — all auth-related configuration

**Why the split:**  
A StorageClass is a cluster infrastructure concern, not an authentication concern. Cluster.tsx is the natural home for infrastructure settings; AdminSettings.tsx owns auth-only config. This separation keeps concerns aligned with where users expect to find them.

**Permission gating:**  
- **Route guard (web/src/router/tree.tsx):** The `/admin` route itself is guarded by `config:manage` permission (admin and operator roles only).
- **AdminSettings.tsx** (OIDC/auth section): The `RoleMappingOverridesCard` component is gated by `config:manage` permission check at render time — since it is an editing surface for role mappings (not read-only display). The route-level and component-level gates are now aligned.
- **Cluster.tsx** (storage card): Is **not** already admin-only, so it requires an explicit `can(me, "config:read")` permission check on both the query (to avoid 403 errors) and the rendered card (to hide it from viewers lacking permission). Precedent: Dashboard.tsx:51 and Users.tsx:688.

**OIDC Role Mapping Overrides (AdminSettings.tsx):**

The `helmOverride` object in the API's `AuthCfg` response (`auth.helmOverride.roleMappings`) allows dashboard users to override the Helm-seeded role mappings **per role**. Each role (admin/operator/viewer) is independently optional:

- **Key present** (including empty list `[]`): an override exists, stored in the dashboard database, superseding the Helm value
- **Key absent**: no override; the role uses the Helm-seeded value
- **Empty list `[]`**: distinct from absent; means "nobody maps to this role" (intentionally empty)

**Provenance is derived client-side from key presence**, not from a server-provided `source` field:
- If `helmOverride.roleMappings[role]` is present → "Overridden in dashboard"
- If absent and Helm-seeded value exists → "From Helm values"
- If absent and no Helm-seeded value → "Not configured"

**Per-role UI in RoleMappingOverridesCard (AdminSettings.tsx):**

For each role (admin/operator/viewer), the card displays:
1. **Effective mapping:** the list of IdP group names currently mapped (from override if present, else from Helm-seeded)
2. **Provenance badge:** one of the three states above
3. **Edit controls:** add/remove IdP group names
4. **Reset button:** "Reset to Helm default" shown only when an override exists (allows reverting to Helm values)
5. **Empty state:** a visual indicator when zero groups are mapped

**FR-012 — Empty state & remediation:**  
When no role mappings exist (neither overridden nor Helm-seeded), the card displays an empty state. Two remediation paths:
1. If the operator *is* configured (Helm `oidc.enabled`): add role mappings via the overrides editor
2. If the operator *is not* configured: the Helm provider panel shows a banner; no override editor is available (no mappings to configure)

**FR-015 — Admin-role mapping confirmation modal:**

Mapping users to the admin role grants full cluster control. The dashboard enforces a two-step confirmation:

- **Read-only Helm-seeded path (HelmOIDCProviderCard):** If Helm-seeded role mappings include admin mappings, show a warning banner (no confirm step; the admin already made this decision at deploy time)
- **Editable override path (RoleMappingOverridesCard):** When a user tries to **add** an admin role mapping via the overrides editor, show `AdminMappingConfirmDialog`

The confirm dialog displays:
- A fixed warning banner with exact copy: "Mapping users to the admin role grants full cluster control. Ensure the mapped group contains only authorized personnel. Anyone in these groups gets full admin access from their next login."
- The group name(s) being mapped
- Requires explicit user confirmation before the override is saved

**Warning unconditional on group size:**  
The warning is shown regardless of how many groups are being added (single or multiple). Gameplane cannot enumerate IdP membership at the dashboard layer, so it cannot verify that a group contains only authorized personnel — the warning applies uniformly.

**Implementation:** Uses existing form patterns (`useSectionForm`, `useUpdateConfigSection`, `useResetRoleMapping` hook for the reset DELETE request) for unified save/cancel. Overrides are written via the existing `PUT /admin/config` endpoint's `AuthCfg` round-trip (no separate endpoint). Resets use `DELETE /admin/config/auth/role-mappings/{role}`.

10. **AuditLog** (`/admin/audit`) → `AuditLogPage` (gated by `audit:read` permission)
    - Paginated audit event table (action, actor, resource, result, timestamp)
    - Verify audit trail hash chain, export to CSV

11. **AdminLogs** (`/admin/logs`) → `AdminLogsPage` (gated by `*` wildcard permission)
    - API and system pod logs (tail, download)
    - Diagnostic endpoint for cluster issues

12. **Backups** (`/backups`) → `BackupsPage`
    - List Backups, Schedules, Restores (three sub-tabs)
    - Manual backup trigger, schedule create/edit/suspend, restore from backup

## ServerDetail Tabs

Visible tab set depends on server template + active version:

1. **Overview** — GameServer status, phase, uptime, restart count; live metrics from RCON (if available); recent pod events
2. **Events** — Kubernetes events: image pulls, scheduling, crash-loop, agent startup
3. **Console** — Interactive RCON/WebSocket terminal (hidden if template has no console, or consoleMode=none)
4. **Logs** — Live pod stdout OR configured game log file (agent-provided via mTLS; see gameplane.local/logPath annotation)
5. **Files** — Browser and editor for server data files (config, save games, logs)
6. **Mods** — List/install/remove mods; browse by registry provider (Modrinth, CurseForge, etc.) if template declares one
7. **Modpacks** — Install modpacks (only if template + active version supports loader with modpack capability)
8. **Players** — Online player snapshot, ban list, whitelist, kick/ban/unban actions
9. **Backups** — Per-server backup list, schedule management, restore trigger
10. **Capture** — a `CaptureWidget` component driving start/stop of packet captures and a table of past captures for this server, gated on the `captures:manage` permission. Sits between the Backups and Settings tabs per `design-export/json` node `O08uaD`/`b4eaUf` (start-capture modal) and `m5kOm4` (capture list). `CaptureWidget.tsx` with `Captures` client namespace (`web/src/lib/api.ts:127-175`) and router/tab wiring (`ServerDetail.tsx:278`) are implemented in `web/src`.
11. **Settings** — Grouped form with sub-sections (below); changes are draft-until-save; conflict detection on reload

## ServerDetail Settings Sub-sections

Settings tab (`SettingsTab`, `web/src/routes/tabs/Settings.tsx`) displays 11 sections in a left sidebar (`SECTIONS` array):

1. **General** — Server name, description
2. **Version** — Template version selector (triggers container restart)
3. **Resources** — CPU request/limit, memory request/limit (Kubernetes resource specs)
4. **Networking** — Service type (ClusterIP/NodePort/LoadBalancer), LoadBalancer hostname, address pool / explicit address request, port overrides. Tunnel validation (`tunnel.enabled`, provider-specific config, credentials) is computed during render and reported via `onValidityChange` callback in an effect; local field state for `addressPool` and `address` is held in `useState` — so consecutive edits within one render are cumulative rather than each recomputing against the same stale `net` snapshot — and is re-seeded from props by two complementary mechanisms: on identity change, via a parent `key` remount; and in-render, whenever the incoming values differ by value from the last synced pair (so a save round-trip or a reload that returns changed values for the *same* server is picked up).
5. **Environment** — Custom env var key=value pairs
6. **Lifecycle** — Pre/post-start/stop scripts, quiesce grace period
7. **Scheduled backups** — Backup schedule CRUD (daily/weekly/cron), retention policy
8. **Network capture** — `NetworkCaptureSection` (`web/src/routes/tabs/settings/NetworkCapture.tsx`). Enable/disable switch for `spec.capture.enabled` (the opt-in ephemeral-container sidecar); states plainly that disabling stops new captures immediately but the already-injected sidecar container stays in the pod, idle, until the pod is next recreated (Kubernetes has no API to remove an ephemeral container); admin-access + unredacted-data warning; a Retention Window control (numeric value + seconds/minutes/hours/days unit select) writing `spec.capture.retentionSeconds`, validated client-side against the cluster ceiling of 604,800 seconds (7 days — a storage-limitation-informed engineering default, explicitly not a legal requirement) mirroring the CRD's `+kubebuilder:validation:Maximum=604800` on `CaptureConfiguration.RetentionSeconds` (`operator/api/v1alpha1/gameserver_types.go`). Controls are disabled, with an explanatory note, for a session lacking the `captures:manage` permission (`api/internal/rbac/catalog.go`) — the API is still the real enforcer (FR-005: non-admin capture operations get 403).
9. **Placement** — Node selector labels, pod affinity/anti-affinity rules (lazy-loaded)
10. **RBAC & access** — Server owner + collaborator list, permission inheritance. The `setCollaborators` mutation runs unconditionally at component render (not gated by an early return), so hook invocation order is consistent. The mutation's namespace is derived from the GameServer's `metadata.namespace` (or `gameplane-games` as fallback); when no server is loaded, the mutation returns early without calling the API. On success, the mutation invalidates the `["server", gs.metadata.name]` query cache, clears input state, and resets errors; on error, it sets a locally-rendered error message and does not clear input, allowing retry.
11. **Danger zone** — Clone, transfer owner, wipe data (confirm-dialog), delete server

**Capture types (`src/types.ts`, built):** `CaptureConfiguration` (`{ enabled?, retentionSeconds? }`, mirrors `spec.capture` — `retentionSeconds` is bounded by the CRD's authoritative `+kubebuilder:validation:Minimum=1 / Maximum=604800` on `operator/api/v1alpha1/gameserver_types.go`'s `CaptureConfiguration.RetentionSeconds`, omit to use cluster default (86400)), `CaptureStatus` (mirrors `status.capture`: `ready`, `activeCapture`/`lastCaptureTime` typed nullable since the API's `formatOptionalTime` never omits the key), `CapturePhase` (`"Pending" | "Running" | "Completed" | "Failed" | "Expired"`), `NetworkCapture` (one capture record — merges the API's start/stop/list/get response shapes) and `NetworkCaptureList` (the `:captures` list envelope). All are implemented in the tree; the `CaptureWidget` component and `Captures` endpoint namespace are live (see Tabs and API Client sections above).

## External Interface / API Client

**Three-layer client:**

### Layer 1: Thin Fetch Wrapper (`lib/api.ts`)

```typescript
api<T>(path: string, opts?: Options): Promise<T>
```

- Base URL: relative paths (Vite proxy in dev, same-origin in prod)
- CSRF: reads `gameplane_csrf` cookie, injects `X-Gameplane-CSRF` header on POST/PUT/PATCH
- Cluster threading: appends `?cluster=<clusterId>` when non-local cluster is selected
- Error: throws `APIError(status, body)` on !ok; TanStack Query treats it uniformly
- 204 No Content: returns `undefined as T`
- Credentials: `include` (send cookies)

**Helpers:**
- `csrfHeaders()` — returns `{ "X-Gameplane-CSRF": token }` for raw fetch (multipart, plaintext)
- `getCurrentCluster()` / `setCurrentCluster()` — global cluster context

### Layer 2: Typed Endpoint Namespaces (`lib/endpoints.ts`)

Each namespace is an object of typed functions building and fetching URLs:

- **Servers** — `list()`, `get(name, ns?)`, `create(body)`, `update(name, body, ns?)`, `remove(name, ns?)`, `lifecycle(name, verb, ns?)` (start/stop/restart), `clone(name, newName, ns?)`, `wipeData(name, confirm, ns?)`, `transfer(name, userId, ns?)`, `setCollaborators(name, ns, body)`, `getMyServers()`, `status(name, ns?)`, `events(name, ns?)`, `runAction(name, body, ns?)`, `mods(name, ns?)`, `installMod(name, body, ns?)`, `removeMod(name, mod, ns?)`, `modUpdates(name, ns?)`, `uploadMod(name, file, ns?)` (FormData), `registryProviders(name, ns?)`, `searchRegistry(name, opts?, ns?)`, `modVersions(name, project, provider?, ns?)`, `modpackDeps(name, project, provider?, ns?)`, `installModpack(name, body, provider?, ns?)`, `modIDs(name, ns?)`, `setModIDs(name, ids, ns?)`. **ServerCreate request shape:** `create(body)` accepts a `ServerCreate` object with optional `networking` sub-field carrying `expose`, `hostname`, `sourceRanges`, `portOverrides`, `addressPool` (load-balancer pool name), and `address` (requested IP); these are threaded into the `spec.networking` of the created GameServer. **Server response / GameServerEndpoint shape:** Each server's `status.endpoints` is an array of `GameServerEndpoint` objects, carrying: `name` (endpoint identifier), `host`, `port`, `protocol`, `private` (true for tailnet-only addresses), `tunnelProvider` (non-empty for tunnel-routed endpoints like frp/tailscale/playit), and `pool` (the load-balancer address pool the address was allocated from; set by the reconciler when a pool request is honored, absent for other address sources).

- **Templates** — `list()`, `get(name)`

- **Cluster** — `info()`, `stats()`, `view()`, `addNode()` (POST), `kubeconfig()` (blob download)

- **Clusters** — `list()` (multi-cluster registry)

- **Backups** — `list()`, `get(name)`, `create(opts)`, `remove(name)`

- **Schedules** — `list()`, `get(name)`, `create(opts)`, `patchSpec(name, patch)` (suspend toggle), `remove(name)`

- **Restores** — `list()`, `create(opts)`, `remove(name)`

- **BackupDestinations** — `list()`, `get(name)`, `upsert(body)` (POST), `remove(name)`

- **Players** — `snapshot(server, ns?)`, `banned(server, ns?)`, `moderate(server, action, body, ns?)` (kick/ban/unban), `whitelist(server, ns?)`, `whitelistAdd(server, name, ns?)`, `whitelistRemove(server, name, ns?)`

- **Users** — `me()`, `list()`, `create(body)`, `update(id, body)`, `remove(id)`, `resetPassword(id, password)`, `bindings(id)`, `addBinding(id, body)`, `removeBinding(id, roleName, namespace)`

- **Roles** — `list()`, `catalog()` (permission groups), `create(body)`, `update(name, body)`, `remove(name)`

- **Auth** — `login(body)` (local), `logout()`, `oidcStartURL(name?)`, `providers()` (pre-auth public)

- **AuthProviders** — `putSecret(name, body)` (clientSecret), `deleteSecret(name)` (admin)

- **Audit** — `page(limit, before)` (pagination), `verify()` (hash chain), `exportCsv(filter?)` (blob)

- **Notifications** — `test(name)`, `putSecret(name, body)` (sink credentials), `deleteSecret(name)`

- **ModRegistries** — `putSecret(provider, apiKey)`, `deleteSecret(provider)`

- **Files** — `list(server, path, ns?)`, `read(server, path, ns?)`, `write(server, path, content, ns?)`, `mkdir(server, path, ns?)`, `remove(server, path, recursive?, ns?)`, `upload(server, dir, files, ns?)`, `downloadURL(server, path, ns?)`

- **Logs** — `downloadURL(server, ns?)`, `fileStreamPath(server, ns?)` (WebSocket), `podStreamPath(server, ns?)` (WebSocket)

- **Modules** — `catalog()`, `list()`, `get(name)`, `install(body)`, `upgrade(name, version)`, `uninstall(name)`

- **ModuleSources** — `list()`, `create(name, spec)`, `update(name, spec)`, `remove(name)`, `upload(source, file, opts?)` (blob), `removeUpload(source, module)`

- **Captures** (`web/src/lib/api.ts:127-175`) — wraps the 8 REST routes `api/internal/handlers/capture.go` mounts under `MountCapture` (all gated by the `captures:manage` permission, `api/internal/rbac/catalog.go`): `POST /servers/{name}:capture-enable`, `POST /servers/{name}:capture-disable`, `POST /servers/{name}:capture-start`, `POST /servers/{name}:capture-stop`, `GET /servers/{name}:captures` (list), `GET /servers/{name}:capture?id=` (status), `GET /servers/{name}:capture-file?id=` (download), `DELETE /servers/{name}:capture?id=`. Per `specs/done_003-network-capture-sidecar/research.md`'s "Decision 1: Download Path" (T007, resolved): the download handler streams directly from the capture sidecar's `:9091 GET /captures/{id}/file` through the existing `<gs>-agent` ClusterIP Service's second port — not proxied through the agent's general `/files/*` file-browser surface (which is single-rooted at `--data-root` and cannot serve the capture emptyDir). The dashboard API client is expected to call through that one `:capture-file` indirection point rather than reconstructing the sidecar path itself.

**Helper:** `withNS(path, ns?)` appends `?namespace=<ns>` when provided; `withCluster(path)` appends `?cluster=<clusterId>` when non-local.

### Layer 3: Domain Helpers (`lib/*.ts`)

- **capabilities.ts** — `resolveConsoleMode(template)`, `serverHasMods(template, server)`, `serverHasModpacks(template, server)` — drive tab visibility
- **servers.ts** — `isServerRunning(phase)`, phase → string formatters
- **games.ts** — game icon URLs, console protocol detection (RCON/Satisfactory/Battleye)
- **auth.ts** — `getCurrentUser()`, OIDC provider list, logout flow
- **cluster.ts** — `getCurrentCluster()` / `setCurrentCluster()` (localStorage-backed)
- **validation.ts** — domain, port, K8s resource validation
- **events.ts** — event severity / reason → display strings
- **quantity.ts** — parse/format K8s quantities (500m → 0.5, 1Gi → 1073741824 bytes)

## Realtime (WebSocket & SSE)

### WebSocket (`lib/ws.ts`)

```typescript
openWS(path: string, opts: WSOptions)
  → { send(data), close() }
```

**Status states:** `connecting` | `open` | `reconnecting` | `closed` (emitted via `onStatus` callback)

**Behavior:**
- Auto-reconnect with exponential backoff: 500ms × 2^attempt, capped at 30s
- Attempt counter increments on each failed reconnection; resets to 0 on open
- `reconnect=false` option disables auto-reconnect (e.g., for intentional closes)
- Protocol detection: `wss://` on HTTPS, `ws://` on HTTP

**Used by:**
- Console tab: streams RCON/stdin input/output (bidirectional)
- Logs tab: streams container stdout and/or game log file (read-only)

**Local-cluster limitation:** WebSocket paths (`/ws/servers/{name}/logs`, `/ws/servers/{name}/logs/pod?from=start`) do not thread `?cluster=` param; multi-cluster WebSocket support is deferred.

### Server-Sent Events (`lib/sse.ts`)

```typescript
openEventStream(opts: EventStreamOptions)
  → () => void  // disposer
```

**Behavior:**
- Connects to `/events` (EventSource, auto-reconnect on transient close)
- Each frame is a Kubernetes watch event: `{ kind, eventType, object }`
- Frames are parsed as JSON; malformed frames are silently dropped
- Manual reconnect on onerror (after 3s backoff) if the browser closed the stream
- No-op fallback if EventSource is undefined (jsdom, ancient browser)

**Used by:**
- Global event listener in `main.tsx` or a provider component
- Invalidates TanStack Query caches on MODIFIED/DELETED (watches servers, templates, backups, schedules, restores)
- Powers notifications panel (shows recent activity)

**Note:** SSE does not thread cluster context (local only, for now).

## Key Invariants

1. **TypeScript strict mode** — `tsconfig.json` sets `strict: true`, `noUnusedLocals`, `noUnusedParameters`, `noFallthroughCasesInSwitch`
2. **No unjustified `any`** — `@typescript-eslint/no-explicit-any: error`; any real `any` must be a comment explaining why
3. **No floating promises** — `@typescript-eslint/no-floating-promises: error`; either `await` or `void` prefix every Promise
4. **Design-first for visuals** — all UI/layout changes start in `design.pen`, not React code
5. **Pre-auth privacy** — login page and unauthenticated screens leak no internal state (rule 3, CLAUDE.md)
6. **Fix, not silence** — ESLint / TypeScript flags are fixed at source, never suppressed inline (rule 4, CLAUDE.md)
7. **Operator is authoritative** — business logic lives in the operator; the dashboard is a pure view layer
8. **Generic, template-driven rendering** — Game configuration (Create Server steps, Settings tab) and status (Overview metrics) render via the template's declared schema (`spec.configSchema`, `spec.capabilities.status.metrics`, etc.), with no per-game branching; the surface works identically for every game type (FR-024)

## Dependencies

**Runtime:**
- `react@18.3.1` — React library
- `react-dom@18.3.1` — DOM renderer
- `@tanstack/react-router@1.75.0` — file-based routing (via tree.tsx)
- `@tanstack/react-query@5.59.0` — data fetching, caching, invalidation
- `@tanstack/react-virtual@3.10.8` — virtualized lists for large tables
- `@radix-ui/*` — dialog, dropdown-menu, label, slot, tabs, toast (headless, unstyled)
- `clsx@2.1.1` — conditional classNames
- `tailwind-merge@2.5.2` — Tailwind class conflict resolution
- `class-variance-authority@0.7.0` — component variant system
- `tailwindcss@3.4.13` — utility-first CSS framework
- `lucide-react@0.445.0` — SVG icon library
- `@monaco-editor/react@4.6.0` — code editor (lazy-loaded, file/config edit tabs)
- `@xterm/xterm@5.5.0` — terminal emulator (Console tab)
- `@xterm/addon-fit@0.10.0` — xterm fit-to-container addon

**Dev:**
- `typescript@5.6.2` — strict type checking
- `vite@5.4.8` — build + dev server
- `@vitejs/plugin-react@4.3.1` — React JSX transform + fast refresh
- `vitest@2.1.1` — unit test runner (Jest-like API, Vite-integrated)
- `@vitest/coverage-v8@2.1.1` — V8 coverage reporter
- `@testing-library/react@16.0.1` — component test utilities
- `@testing-library/jest-dom@6.5.0` — DOM matchers
- `@testing-library/user-event@14.6.1` — user interaction simulation
- `msw@2.14.4` — Mock Service Worker for request interception (e2e mock mode)
- `vitest-websocket-mock@0.4.0` — WebSocket mock for unit tests
- `@playwright/test@1.59.1` — browser e2e testing (mock + live modes)
- `jsdom@25.0.1` — DOM implementation for unit tests
- `eslint@9.11.1` — JavaScript linter
- `@typescript-eslint/eslint-plugin@8.7.0` — TypeScript linting rules
- `@typescript-eslint/parser@8.7.0` — TypeScript AST parser for ESLint
- `eslint-plugin-react@7.37.0` — React-specific rules
- `eslint-plugin-react-hooks@5.0.0` — React Hooks rules
- `tailwindcss@3.4.13` + `postcss@8.4.47` + `autoprefixer@10.4.20` — CSS processing
- `@tanstack/router-devtools@1.75.0` — TanStack Router dev tools (optional, for debugging)

## Security considerations

- **Pre-auth privacy:** The login page and unauthenticated screens leak no internal state: no hostnames, cluster names, server counts, version strings, or user-enumeration signals. Errors are neutral ("invalid credentials" only) — see CLAUDE.md rule 3 and docs/security.md.
- **CSRF protection:** Mutating requests carry a double-submit token; `gameplane_csrf` cookie is read and echoed as the `X-Gameplane-CSRF` header by `lib/api.ts` on POST/PUT/PATCH.
- **XSS surface:** User-controlled content rendered in the Monaco editor, xterm console, and log/event views is treated as untrusted. React's automatic escaping and avoiding `dangerouslySetInnerHTML` are the primary defenses.
- **No secrets in the bundle:** The SPA holds no API keys, credentials, or tokens; all privileged actions flow through the authenticated API. Session auth is cookie-based (credentials: include).
- **Session handling:** Auth is cookie-based (credentials: include); the dashboard never stores tokens in localStorage and relies on secure, HttpOnly cookies set by the API.

## Testing & Coverage

**Framework:** Vitest 2.1 + Testing Library (React) + jsdom

**Test files:** Co-located with source (`src/**/*.test.tsx`, `src/**/*.test.ts`)

**Mock server:** MSW 2 in mock e2e mode (Playwright); real API in live e2e mode

**Coverage thresholds** (`vitest.config.ts`):
- Lines: 92%
- Functions: 76%
- Branches: 82%
- Statements: 92%

**Exclusions from coverage:**
- `src/main.tsx` — bootstrapping only
- `src/router/**` — route tree is configuration, not logic
- `src/**/*.d.ts` — type definitions
- `src/types.ts` — type mirrors only
- `src/test/**` — test infrastructure
- `src/styles/**` — CSS
- `src/lib/config.ts` — config-only, no logic

**E2E testing:** Playwright in two modes:

1. **Mock mode** (`GAMEPLANE_E2E_TARGET=mock`, npm run test:e2e:mock)
   - Vite runs with `--mode mock`, loading `.env.mock` (sets `VITE_E2E_MOCK=true`)
   - Dashboard dynamically imports MSW browser worker at bootstrap
   - MSW intercepts every fetch; no cluster needed
   - Fast, deterministic, no side effects

2. **Live mode** (`GAMEPLANE_E2E_TARGET=live`, npm run test:e2e:live)
   - Tests run against a real Kubernetes cluster (gameplane-e2e, via kubectl port-forward)
   - globalSetup spawns port-forward, logs in as admin, saves session cookies
   - Tests inherit auth state from `.auth/storage.json`
   - Slower, flaky, full integration

**Execution:** Serial (workers: 1) because login state is shared across tests; retries: 0 local, 1 in CI.

## References

- **docs/architecture.md** — component overview, data flow, security boundaries, "operator is authoritative" rationale
- **docs/security.md** — auth model, RBAC, threat model, pod security, pre-auth privacy rule
- **docs/installing.md** — Helm values, K8s prerequisites, OIDC setup (for deployment contexts)
- **CLAUDE.md rule 1** — Design-first: visual changes originate in design.pen, not code
- **CLAUDE.md rule 3** — Pre-auth privacy: login page must not leak internal metrics/hostnames/versions
- **CLAUDE.md rule 4** — Fix, don't silence: linter/type flags are fixed at source
- **CLAUDE.md rule 5** — TS strict; no unjustified `any`; no floating promises
- **design.pen** — Source of truth for all UI/layout (Pencil MCP server; do not edit as text)
- **README.md** — Project pitch, quickstart, architecture overview
