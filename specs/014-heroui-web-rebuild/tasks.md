# Tasks: Rebuild the dashboard on the HeroUI design system

**Input**: Design documents from `/specs/014-heroui-web-rebuild/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/, quickstart.md, OPEN-DECISIONS.md

**Tests**: The spec requires tests to be updated in the same change as the screen they cover (FR-010), new tests for the new share-link surfaces (US5), Playwright live coverage of the P1 flows (FR-011) and a per-screen screenshot comparison (FR-009). Those appear as tasks below; no separate TDD-first test phase is generated. Per OD-3 (Settled 2026-09-03), the `web/e2e/specs/live/` Playwright suite (CI job `e2e-web-live`) is this feature's E2E tier; no test is added to the Go `test/e2e/` suite or its bucket registration.

**Organization**: Tasks are grouped by user story, which maps one-to-one onto the delivery slices in plan.md (slice 0 = Setup + Foundational, slice 1 = US1, slices 2a/2b = US2, slice 3 = US3, slice 4 = US4, slice 5 = US5 + Polish). Each slice is its own branch and PR, cut from `master` after the previous slice merges (plan.md "Delivery slices").

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1..US5)
- Include exact file paths in descriptions

## Path Conventions

- Dashboard code: `web/src/…` (routes, components, lib, router, styles, test), Playwright: `web/e2e/…`
- Design source: `design.pen` (Pencil MCP only, never read or edited directly); plain-file snapshot: `design-export/{json,screenshots}/<id>.{json,png}` + `design-export/MANIFEST.md`
- Open decisions OD-1..OD-7 live in `specs/014-heroui-web-rebuild/OPEN-DECISIONS.md`; all seven were settled by the maintainer on 2026-09-03, and every task below that depended on one cites it as "OD-n (Settled 2026-09-03)" with the ruled value

---

## Phase 1: Setup (Shared Infrastructure)

**Slice**: 0 Foundation (branch 014a-foundation), first half

**Goal**: Set up the shared infrastructure for HeroUI migration — dependencies, CSS imports, design exports, and directory structure — without editing design screens or building components yet.

**Independent Test**: tsc --noEmit passes with HeroUI types installed and @heroui/styles import, design-export/MANIFEST.md documents the HeroUI frame export, web/src/components/hero/ directory exists, web/e2e/screenshots/ directory exists with a template spec covering slice 1 screens at 1440px viewport, and npm ci succeeds.

### Pre-flight Design Verification

- [X] T001 Open /home/valgul/project/Gameplane/design.pen in the Pencil GUI and verify that the LtgNm frame (HeroUI: Design System Components) contains 192 named component definitions without missing or unresolved nodes; document the frame id and child count, then close without saving (maintainer task — no changes yet).

### Design Export (HeroUI Component Library)

- [X] T002 Export the LtgNm frame and all 192 HeroUI component definitions from design.pen to design-export/json/LtgNm.json (via Pencil MCP Get at depth ≥ 14, zero "..." elision markers, includePathGeometry: true) and design-export/screenshots/LtgNm.png (2x scale via export_nodes).
- [X] T003 Add a new 'HeroUI Frame' section to design-export/MANIFEST.md documenting the export: timestamp, export method (depth 14 with includePathGeometry), file counts (1 JSON + 1 PNG), validation (json.loads + grep for a HeroUI-unique term), and note that this clears the pre-existing export debt for the HeroUI definitions already in design.pen since 2026-09-02.

### Design cleanup (OD-4, OD-5)

- [X] T004 Per OD-4 (Settled 2026-09-03), delete the imported HeroUI sample frame `dashboard-utility` (`bJ2cg`) and its prompt node `OFfAu` from design.pen via the Pencil MCP (mcp__pencil__execute); confirm neither id remains in a subsequent Get on the document root, then ask the maintainer to save in the Pencil GUI as part of the same design commit as the rest of slice 0.
- [X] T005 Per OD-5 (Settled 2026-09-03), add `heroUI template.pen` to `.gitignore` alongside the existing `design.pen.bak` entry, since design.pen already contains everything the template has (research.md R-05) and the file stays untracked.

### Dependencies and CSS Setup

- [X] T006 Add to web/package.json dependencies: @heroui/react@3.2.4, @heroui/styles@3.2.4, react-aria@^3.51.0, react-aria-components@^1.20.0, @react-aria/i18n@^3.13.1, @react-aria/ssr@^3.10.1, @react-aria/utils@^3.34.1; then run 'npm ci' from web/ to verify install succeeds.
- [X] T007 Add '@import "@heroui/styles";' as the first line in web/src/styles/globals.css (before the existing `:root` block and legacy `@import` statements) so Tailwind base, HeroUI components, and utilities are available to all rebuilt screens; the token mapping and HeroUI variable configuration (T017, slice 0 second half) will follow as a separate change.

### Directory Structure Scaffold

- [X] T008 Create web/src/components/hero/ directory with an empty .gitkeep file to scaffold the location for rebuilt Gameplane compositions on HeroUI definitions (no component files yet, just the directory structure per OD-7 (Settled 2026-09-03): `web/src/components/hero/` during the transition, renamed to `components/ui/` in slice 5).
- [X] T009 Create web/e2e/screenshots/ directory and add a skeleton spec file web/e2e/screenshots/slice0.spec.ts that imports test and expect from @playwright/test, sets viewport to 1440×900, deviceScaleFactor to 2, colorScheme to 'dark', and includes placeholder test cases for each slice 0 atom id (N1GkB, jmoi3, ljdA5 for desktop; tooKB, SeizD for mobile per screen-verification.md); include a helper function to capture PNG screenshots at 2x scale into web/e2e/screenshots/ directory.

### Verification and Commit Readiness

- [X] T010 Run 'npm ci && npx tsc --noEmit' from web/ directory to verify that TypeScript compilation succeeds with HeroUI types installed, @heroui/styles CSS import resolves, and no type errors are introduced by the dependency changes.

**Checkpoint**: Dependencies installed, HeroUI frame exported, `tsc --noEmit` clean; no screen changed

---

## Phase 2: Foundational (Blocking Prerequisites)

**Slice**: 0 Foundation (branch 014a-foundation), second half

**Goal**: Establish the HeroUI atom layer and theme tokens, enabling all subsequent slices to build screens from HeroUI component compositions rather than lunaris primitives.

**Independent Test**: Phase 2 (Foundation, second half) is complete when: (1) all 24 atoms are redrawn on HeroUI definitions in design.pen and re-exported to design-export/json/<id>.json and design-export/screenshots/<id>.png with zero elision markers (grep -l '"..."' returns nothing for all 24 ids), (2) web/src/styles/globals.css maps Gameplane brand tokens to HeroUI semantic tokens (--accent, --surface, --background, --foreground, --border, --muted, --danger, --warning, --success) with the --gp-* legacy alias block and @theme legacy bindings per contracts/theme-tokens.md, (3) a Vitest test in web/src/__tests__/theme.test.tsx verifies computed values of --accent, --surface, --background, --foreground, --border, --muted in both .dark and .light modes match the table in contracts/theme-tokens.md, (4) web/src/components/hero/ contains StatCard, PhaseChip, PageHeader, ConfirmDialog, DropdownMenu, FilterPopover, LoadingCard, ErrorCard, ErrorBanner, Meter, Sparkline, and GameIcon, each with a co-located .test.tsx file, (5) web/specs.md includes a "HeroUI component layer" section describing the new atom components per FR-013, and (6) web cd web && npx tsc --noEmit passes without errors.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

### Design wave

- [X] T011 Redraw design.pen Button and form-atom nodes tpKRk rNhll LMIom XoX7L z9ShNE d5N3W3 from HeroUI definitions in LtgNm per contracts/component-map.md, swapping $c: variable references to HeroUI variables, keeping layout and node names intact.
- [X] T012 Redraw design.pen nodes J09iP IU7OG D0cDM qvQPg Lmaf1 AT7ya (button and form-atom definitions) from HeroUI definitions in LtgNm, swapping $c: variables to HeroUI variables per contracts/theme-tokens.md.
- [X] T013 Redraw design.pen nodes hl7R3 rh2QH k38Uta ZWcwn xCDF7 x3beP (form atoms, Card, StatCard, PageHeader, Modal) from HeroUI definitions in LtgNm, updating variable references and composition structure.
- [X] T014 Redraw design.pen nodes WwNlX BPEpm FyV6E w4ntSc zzx8f igj2U (ConfirmDialog, DropdownMenu, FilterPopover, LoadingCard, ErrorCard, ErrorBanner) as compositions from HeroUI definitions in LtgNm.
- [X] T015 Establish the FR-007 import procedure that every later slice's design wave must follow: when a HeroUI component a screen needs is absent from the LtgNm frame, import it from `heroUI template.pen` into LtgNm through the Pencil MCP before it is used (FR-007, research.md R-05); never recreate the component by hand and never substitute a lunaris `c:` primitive. Record this procedure in design-export/MANIFEST.md's HeroUI Frame section (added in T003) so every later design-wave agent sees it.

### Export and design closure

- [X] T016 Re-export all 24 redrawn atom nodes (tpKRk rNhll LMIom XoX7L z9ShNE d5N3W3 J09iP IU7OG D0cDM qvQPg Lmaf1 AT7ya hl7R3 rh2QH k38Uta ZWcwn xCDF7 x3beP WwNlX BPEpm FyV6E w4ntSc zzx8f igj2U) to design-export/json/<id>.json at depth >=12 with zero "..." elision markers, design-export/screenshots/<id>.png, and update design-export/MANIFEST.md with one row per id, verifying grep -l '"..."' design-export/json/*.json finds none of these 24 files.

### Token mapping and verification

- [X] T017 Rewrite web/src/styles/globals.css per contracts/theme-tokens.md: add @import '@heroui/styles'; replace Tailwind import; define `:root` and `.dark` blocks with HeroUI token values (--accent, --surface, --background, --foreground, --surface-secondary, --overlay, --default, --success, --warning, --danger, --muted, --border); add legacy --gp-* aliases (--gp-bg, --gp-surface, --gp-card, --gp-border, --gp-muted, --gp-fg, --gp-primary, --gp-primary-fg); add @theme block with --color-* aliases pointing to HeroUI tokens; keep --color-violet (hsl 258 90% 66%) for operator chips.
- [X] T018 Add web/src/__tests__/theme.test.tsx with a Vitest test that renders a probe element, applies the theme CSS, and asserts computed values of --accent (hsl 22 95% 53% light / #F97316 dark), --surface, --background, --foreground, --border, --muted equal the values in contracts/theme-tokens.md table, testing both .dark and light modes.

### Code wave — atom components

- [X] T019 [P] Create web/src/components/hero/StatCard.tsx composing HeroUI Card with value/label/trend display per design node ZWcwn, and co-located StatCard.test.tsx with at least 3 test cases; keep all tests from any migrated predecessor.
- [X] T020 [P] Create web/src/components/hero/PhaseChip.tsx composing HeroUI Chip with phase-to-colour map (running→green, idle→yellow, asleep→blue, never-sleeps→grey, failed→red) for server state badges, and co-located PhaseChip.test.tsx.
- [X] T021 [P] Create web/src/components/hero/PageHeader.tsx composing HeroUI title/breadcrumbs/actions per design node xCDF7 used by every dashboard page, with co-located PageHeader.test.tsx.
- [X] T022 [P] Create web/src/components/hero/ConfirmDialog.tsx composing HeroUI AlertDialog for destructive actions (danger style per design node WwNlX) with title, description, cancel, and confirm buttons, co-located ConfirmDialog.test.tsx.
- [X] T023 [P] Create web/src/components/hero/DropdownMenu.tsx composing HeroUI Dropdown + Menu per design node BPEpm, supporting item groups, dividers, and disabled states, with co-located DropdownMenu.test.tsx.
- [X] T024 [P] Create web/src/components/hero/FilterPopover.tsx composing HeroUI Popover + form controls per design node FyV6E for server-list phase/template/namespace filters, with co-located FilterPopover.test.tsx.
- [X] T025 [P] Create web/src/components/hero/LoadingCard.tsx composing HeroUI Card with Spinner per design node w4ntSc, shown while async data loads, with co-located LoadingCard.test.tsx.
- [X] T026 [P] Create web/src/components/hero/ErrorCard.tsx composing HeroUI Alert (danger) + Card per design node zzx8f for failed-load states, with error icon, title, and dismiss button, co-located ErrorCard.test.tsx.
- [X] T027 [P] Create web/src/components/hero/ErrorBanner.tsx composing HeroUI Alert (danger) per design node igj2U for inline error messages (e.g. failed save, PVC provisioning failure), with co-located ErrorBanner.test.tsx.
- [X] T028 [P] Create web/src/components/hero/Meter.tsx as a pure-SVG progress/radial meter component (no HeroUI counterpart) kept from current ui/meter.tsx, migrating any tests to co-located Meter.test.tsx and ensuring no test is deleted.
- [X] T029 [P] Create web/src/components/hero/Sparkline.tsx as a pure-SVG mini line chart component (no HeroUI counterpart) kept from current ui/sparkline.tsx, migrating any tests to co-located Sparkline.test.tsx.
- [X] T030 [P] Create web/src/components/hero/GameIcon.tsx displaying game logos (Minecraft, Valheim, Terraria) as SVG or img elements per design specifications, migrating from current ui/game-icon.tsx with co-located GameIcon.test.tsx.

### Spec and documentation

- [X] T031 Add a 'HeroUI component layer' section to web/specs.md (after any existing architecture description) describing the 12 slice-0 atom components (StatCard, PhaseChip, PageHeader, ConfirmDialog, DropdownMenu, FilterPopover, LoadingCard, ErrorCard, ErrorBanner, Meter, Sparkline, GameIcon) as the foundation for all rebuilt screens, noting that they compose @heroui/react primitives on Gameplane brand tokens, and reference contracts/theme-tokens.md for token mapping per FR-013.
- [X] T032 Apply PR labels to the slice 0 (014a-foundation) pull request via the REST API per CLAUDE.md rule 14 (`gh pr edit` does not work on this repo): `gh api -X POST repos/ValgulNecron/Gameplane/issues/<pr-number>/labels -f "labels[]=type: refactor" -f "labels[]=area: web"`; verify with `gh api repos/ValgulNecron/Gameplane/issues/<pr-number>/labels -q '[.[].name]|join(", ")'`.

### Theme Implementation (added 2026-09-04)

- [X] T032a Apply the 2026-09-04 palette to design.pen HeroUI variables (both modes) and re-point every screen/definition from lunaris $c:--* and legacy hex to HeroUI tokens per the token tables in contracts/theme-tokens.md (light: accent #DB2777, surface #FFF7FB, background #FFFFFF, foreground #2A0F1E, etc.; dark: accent #FF4FA3, surface #1C1A20, background #121114, foreground #F5F3F7, etc.); add five light-mode preview frames (gX7um Login (Light), oyoTs Dashboard Home (Light), zFiOW Servers (Light), sSISK Server Detail Overview (Light), DWztv Mobile Servers (Light)) to show both modes side by side in Pencil per maintainer decision 2026-09-04.
- [X] T032b Add light-mode preview frames to design.pen per OD-specific decision: gX7um Login (Light) (restored 2026-09-05 after an out-of-scope deletion, refreshed final 2026-09-05), oyoTs Dashboard Home (Light) (re-created 2026-09-05 after the bare-frame card fix wave, refreshed final 2026-09-05), zFiOW Servers (Light), sSISK Server Detail Overview (Light), DWztv Mobile Servers (Light) (refreshed final 2026-09-05), so both light and dark modes are visible simultaneously without switching appearance in Pencil; note that the original screens remain in dark mode as designed.
- [X] T032c Re-export design-export/ after the 2026-09-04 theme changes: slice-0 atoms, definitions, and the five light previews (gX7um, oyoTs, zFiOW, sSISK, DWztv as of 2026-09-05 final refresh) exported to design-export/json/<id>.json and design-export/screenshots/<id>.png at depth ≥12 with zero "..." elision markers; no $c: reference remains in the exports of the ids re-pointed in that wave; other screens keep $c: bindings until their own slice (see the T033 note on zFiOW); update design-export/MANIFEST.md with one row per id clarifying the HeroUI token re-point and noting that lunaris library variables are read-only in Pencil.
- [X] T032d Update web/src/styles/globals.css :root/.dark blocks and web/src/__tests__/theme.test.tsx to the contracts/theme-tokens.md light and dark values (currently carrying legacy orange values from before the re-theme); this is a code follow-up to T032a/T032b/T032c, verified by CI; note: design-export was updated in T032c but code values lag pending this task completion.
- [ ] T032e Decide OD-9 (purple stat-icon token): verify that StatCard icon background color uses the settled purple #8B5CF6 per the 2026-09-04 decision (was amber before), and ensure theme.test.tsx covers this token value in both light and dark modes; if the token is not yet settled in OPEN-DECISIONS.md, add the decision with citation to maintainer 2026-09-04 ruling.

**Checkpoint**: Foundation ready - slice 0 PR green on CI and merged; every later slice is a pure translation job

---

## Phase 3: User Story 1 - Sign in and navigate the shell (Priority: P1) 🎯 MVP

**Slice**: 1 Shell + login (branch 014b-shell-login)

**Goal**: Deliver User Story 1 (P1 Shell + login) with the application shell (sidebar, top bar, page headers, cluster selector, search, notifications, theme toggle) and login surfaces rebuilt from HeroUI components, fully testable with login privacy preserved and every sidebar destination reachable and gated by role.

**Independent Test**: Sign in with a local admin account and SSO-only login variant; confirm the three login states (default, invalid credentials, SSO-only) and app-loading state render from HeroUI components without revealing internal details; then click every sidebar entry and confirm each page loads with the new shell. Fully testable by a viewer, operator, and admin session, delivering the rebuilt frame for every page.

### Design wave — redraw shell, login, and mobile screens

- [X] T033 [US1] Redraw Screen/Login — Default (N1GkB), Screen/Login — Invalid Credentials (jmoi3), and Screen/Login — SSO Only (ljdA5) in design.pen from HeroUI Button, Input, Alert definitions per contracts/component-map.md, swapping lunaris $c: variables to HeroUI theme variables (e.g., $c:--accent to $accent/accent).
  - Note 2026-09-05: zFiOW (Screen/Servers (Light)) still carries lunaris refs (c:KbyBJ, c:BdBJJ, c:X6nwq, c:dOLzc) and $c:--font-* bindings — Servers is a slice-2a screen (F9pUrx); it is redrawn and its light copy re-created in slice 2a, not here.
- [X] T034 [US1] Redraw Screen/App Loading (N13Xud) and Screen/Dashboard — Home (j24cXg) in design.pen from HeroUI Spinner, Card, and composition definitions, preserving layout and copy, swapping $c: variables to HeroUI tokens.
- [X] T035 [US1] Redraw Screen/Servers — Mobile (tooKB) and Screen/Navigation Drawer — Mobile (SeizD) in design.pen from HeroUI Drawer, ListBox, Link, and other component definitions for mobile viewport (390px).
- [X] T036 [US1] Redraw Gameplane/App Sidebar (kKFX9), Gameplane/Top Bar (gu5WY), Gameplane/Cluster Selector (aI9PL), Gameplane/Notifications Panel (hboVw), and Gameplane/Search Results (IdaU7) composition definitions in design.pen from HeroUI Button, Link, ListBox, Dropdown, Menu, Avatar, Popover, and SearchField per contracts/component-map.md.
- [X] T037 [US1] Add appearance-toggle design frame (light/dark/system state) to the sidebar profile footer next to logout, mirrored in the mobile navigation drawer, per OD-2 (Settled 2026-09-03), composing from HeroUI Switch and ensuring variants for both dark and light mode.
- [X] T038 [US1] Export screens N1GkB, jmoi3, ljdA5, N13Xud, j24cXg, tooKB, SeizD and compositions kKFX9, gu5WY, aI9PL, hboVw, IdaU7 to design-export/json/<id>.json and screenshots/<id>.png at depth ≥12 with zero "..." elision markers, then update design-export/MANIFEST.md with row entries for each exported id.
- [X] T039 [US1] Confirm maintainer has saved design.pen in Pencil GUI, then create a single design commit with all design.pen changes, design-export/ updates, and MANIFEST.md row additions, before proceeding to code wave.

### Code wave — shell and layout infrastructure

- [X] T040 [US1] Rebuild web/src/routes/Login.tsx from exported login screens (N1GkB, jmoi3, ljdA5) using HeroUI TextField, Button, Alert, and Spinner components; preserve error handling for invalid credentials and SSO-only state detection; update Login.test.tsx with new selectors and maintain ≥ baseline test count; ensure tsc --noEmit passes.
- [X] T041 [US1] Rebuild web/src/routes/Dashboard.tsx from exported dashboard screen (j24cXg) using HeroUI Card and Spinner, displaying app-loading state and empty dashboard frame; update Dashboard.test.tsx; ensure tsc --noEmit passes.
- [X] T042 [P] [US1] Rebuild web/src/components/AppLayout.tsx to compose hero/AppShell and hero/TopBar, implementing theme toggle via HeroUI useTheme hook syncing to localStorage and index.html data-theme attribute per OD-2 (Settled 2026-09-03: sidebar profile footer next to logout, mirrored in the mobile drawer); update AppLayout.test.tsx with new component structure; ensure tsc --noEmit passes.
- [X] T043 [P] [US1] Rebuild web/src/components/PageHeader.tsx from exported composition using HeroUI Breadcrumbs, Heading, and layout components; update PageHeader.test.tsx; ensure tsc --noEmit passes.
- [X] T044 [P] [US1] Rebuild web/src/components/ClusterSelector.tsx (composition aI9PL) using HeroUI Select and Button components, preserving multi-cluster context switching; update ClusterSelector.test.tsx; ensure tsc --noEmit passes.

### Code wave — new hero components

- [X] T045 [P] [US1] Create web/src/components/hero/AppShell.tsx and AppShell.test.tsx composing hero/Sidebar, hero/TopBar, and Breadcrumbs with a main content slot; export from @heroui/react and @/components/hero; ensure tsc --noEmit passes.
- [X] T046 [P] [US1] Create web/src/components/hero/Sidebar.tsx and Sidebar.test.tsx from exported Gameplane composition (kKFX9) using HeroUI Link, ListBox, Separator, Avatar, and Drawer components; implement navigation with active-state highlighting, permission-based visibility gating (viewer hides admin-only entries), and mobile drawer variant; ensure tsc --noEmit passes.
- [X] T047 [P] [US1] Create web/src/components/hero/TopBar.tsx and TopBar.test.tsx from exported Gameplane composition (gu5WY) using HeroUI Button, Avatar, Dropdown, Popover components; include cluster selector slot and user menu; ensure tsc --noEmit passes.
- [X] T048 [P] [US1] Create web/src/components/hero/Breadcrumbs.tsx and Breadcrumbs.test.tsx using HeroUI Breadcrumbs component for page navigation hierarchy; ensure tsc --noEmit passes.
- [X] T049 [P] [US1] Create web/src/components/hero/NotificationsPanel.tsx and NotificationsPanel.test.tsx from exported Gameplane composition (hboVw) using HeroUI Popover, Button, and Card components for notification display and actions; ensure tsc --noEmit passes.
- [X] T050 [P] [US1] Create web/src/components/hero/GlobalSearch.tsx and GlobalSearch.test.tsx from exported Gameplane composition (IdaU7) using HeroUI SearchField and Popover components; implement search results rendering and navigation; ensure tsc --noEmit passes.

### Styling and theme integration

- [X] T051 [US1] Verify the slice-0 token mapping in web/src/styles/globals.css (written in T017 per contracts/theme-tokens.md) is complete for the shell and login surfaces and that no `$c:` variable remains in compiled CSS output; do not redefine the token blocks.
- [X] T052 [US1] Add appearance-toggle switch to hero/Sidebar.tsx profile footer next to logout, mirrored in the mobile navigation drawer, per OD-2 (Settled 2026-09-03), using HeroUI Switch and useTheme hook to toggle between 'light', 'dark', and 'system' modes, syncing selection to localStorage and index.html data-theme attribute, and update Sidebar.test.tsx with toggle interaction tests covering both the sidebar footer and the mobile drawer instance.
- [X] T053 [P] [US1] Update web/index.html to include data-theme attribute (initialized to 'dark' default) and add <script> to apply saved theme preference from localStorage on page load, supporting dark/light/system modes per HeroUI useTheme integration.

### Testing and verification

- [X] T054 [US1] Create web/e2e/screenshots/slice1.spec.ts screenshot spec covering login screens (N1GkB, jmoi3, ljdA5) at 1440px, app-loading screen (N13Xud) at 1440px, dashboard (j24cXg) at 1440px, and mobile screens (tooKB, SeizD) at 390px with MSW fixtures for each state (unsigned, invalid credentials, SSO-only, loading, admin logged-in, viewer logged-in, mobile), extracting screenshots at deviceScaleFactor 2 in dark mode for comparison against design-export/screenshots/<id>.png per contracts/screen-verification.md.
- [X] T055 [P] [US1] Update selectors and queries in web/e2e/specs/login.spec.ts for login form fields, error alerts, and submit button to match new HeroUI TextField, Button, and Alert markup; update web/e2e/specs/shell.spec.ts (or create if missing) for sidebar, top bar, cluster selector, and breadcrumbs to match HeroUI Link, ListBox, Select, and Breadcrumbs components.
- [X] T056 [US1] Add live Playwright specs web/e2e/specs/live/login-and-shell.spec.ts (this is the feature's E2E tier per OD-3, Settled 2026-09-03 — no corresponding Go test/e2e/ test is added) exercising login with local admin credentials and SSO-only variant per Acceptance Scenarios 1–2, appearance-toggle interaction in light/dark/system modes, sidebar navigation through all admin destinations (Dashboard, Servers, Modules, Backups, Cluster, Users, Audit log, System logs, Admin settings) per Scenario 3, permission-based visibility gating with a viewer session per Scenario 4, and mobile drawer navigation at 390px viewport per Scenario 5; ensure tests call t.Parallel() and use unique resource names per e2e conventions.
- [X] T057 [P] [US1] Verify FR-005 login privacy by inspecting rendered HTML and error messages on login screens (N1GkB, jmoi3, ljdA5, N13Xud) to confirm no internal metrics, cluster names, version strings, user-enumeration hints, or hostnames are displayed; only brand, form, and neutral error copy ('invalid credentials') appear.

### Documentation and final checks

- [X] T058 [P] [US1] Update web/specs.md to add a Slice 1 section describing the HeroUI component layer for the shell and login surfaces: AppShell, Sidebar, TopBar, Breadcrumbs, NotificationsPanel, GlobalSearch, theme integration via useTheme, brand token mapping per contracts/theme-tokens.md, and note that old lunaris primitives remain on other screens until Slice 5; document FR-010 test coverage maintenance and FR-012 import rule.
- [X] T059 [P] [US1] Confirm TypeScript compilation succeeds with tsc --noEmit in web/; verify test-file count in web/src/**/*.test.tsx is ≥ master baseline; verify coverage thresholds (lines 92%, functions 76%, branches 82%, statements 92%) are met; verify no imports from @radix-ui/*, @/components/ui/, or class-variance-authority appear in any rebuilt file (Login.tsx, Dashboard.tsx, AppLayout.tsx, PageHeader.tsx, ClusterSelector.tsx, hero/*.tsx).
- [X] T060 [P] [US1] Add PR description entries: list all touched design node ids (N1GkB, jmoi3, ljdA5, N13Xud, j24cXg, tooKB, SeizD, kKFX9, gu5WY, aI9PL, hboVw, IdaU7), design commit hash, CSS bundle size delta (informational only — OD-6 (Settled 2026-09-03) sets no budget), screen verification verdicts (match/layout/colour/component-family/content mismatch per contract), keyboard-navigation walk (Tab order through login and sidebar with no focus trap), and confirm FR-005 login privacy check passed.
- [X] T061 Apply PR labels to the slice 1 (014b-shell-login) pull request via the REST API per CLAUDE.md rule 14: `gh api -X POST repos/ValgulNecron/Gameplane/issues/<pr-number>/labels -f "labels[]=type: refactor" -f "labels[]=area: web"`; verify with `gh api repos/ValgulNecron/Gameplane/issues/<pr-number>/labels -q '[.[].name]|join(", ")'`.

**Checkpoint**: US1 fully functional, slice PR green on the `web`, `web-e2e-mock` and `e2e-web-live` jobs, screenshot comparison accepted, merged before the next slice is cut

---

## Phase 4: User Story 2 - Manage servers day to day (Priority: P1)

**Slice**: 2a Servers + core tabs (branch 014c-servers-core) THEN 2b Mods, backups tab, capture, settings (branch 014d-server-settings)

**Goal**: Rebuild User Story 2 (Manage servers day to day) — the Servers list, Server Detail page with all tabs (Overview through Players, then Mods/Modpacks/Backups/Settings), and all Settings sub-sections — on the HeroUI design system across two sub-slices: 2a (Servers + core tabs) then 2b (Mods, backups, capture, settings), verifying each screen against its design frame and ensuring all existing behavior (filters, actions, forms, confirm dialogs, state preservation) is preserved.

**Independent Test**: On the test cluster with ≥1 running and ≥1 stopped server, walk the Servers list (apply filters, verify status chips, open per-row actions menu), click every tab in Server Detail (Overview, Events, Console, Logs, Files, Players in 2a; Mods, Modpacks, Backups, Settings in 2b), open every Settings sub-section, save and discard a settings change, trigger one confirm-dialog flow (e.g., Wipe world via the Danger zone), and verify each surface matches its design frame and behaves as before; CI must pass (web/web-e2e-mock/e2e-web-live all green) with web coverage gates held, and screenshot comparisons must find no layout/colour/family mismatch per contracts/screen-verification.md.

### Slice 2a - Design wave

- [ ] T062 [US2] Redraw server list and server overview (idle/asleep/never-sleeps/PVC-provisioning-failure states) screens from HeroUI definitions in design.pen, swapping lunaris fills/text to HeroUI variables; keep layout, copy and node names; design.pen node ids iGBIs EZFW0 mQ1zB IzuY2 TE2jI o4LH8W
- [ ] T063 [US2] Redraw Events, Console, Logs, Files, Players tab screens from HeroUI definitions in design.pen, swapping lunaris to HeroUI variables; keep layout and copy; design.pen node ids P08Uw Xn5ns kPmoo FtdkI Burtr dPP50
- [ ] T064 [US2] Redraw Server Detail Header composition and Server Detail Tabs composition in design.pen from HeroUI definitions, and redraw dialog compositions (Clone, Transfer, Wipe World) from HeroUI AlertDialog definitions, swapping lunaris to HeroUI variables; design.pen node ids S4k0x I9kvlZ T1LzpU rdlrx t9irnv (dialogs also include New Folder I9W8z and New File JLaGB from code wave)
- [ ] T065 [US2] Export all redrawn Slice 2a screens and dialogs to design-export/json/ (depth ≥ 12, zero '...' markers) and design-export/screenshots/, update design-export/MANIFEST.md with rows for each touched node id, verify no '$c:' variable remains in exports, then save design.pen in the Pencil GUI (do not commit; the maintainer will save and commit the design change)

### Slice 2a - Code wave

- [ ] T066 [P] [US2] Rebuild web/src/routes/Servers.tsx to render the server table, phase chips, filter popover and per-row actions from @heroui/react (Table, Chip, Popover, Dropdown/Menu) and @/components/hero/ (PhaseChip, FilterPopover), update web/src/routes/Servers.test.tsx to the new markup keeping all it() cases, and verify tsc --noEmit
- [ ] T067 [P] [US2] Rebuild web/src/routes/ServerDetail.tsx to render the server detail shell, tab bar and breadcrumbs from HeroUI and hero/ compositions, update ServerDetail.test.tsx keeping all it() cases, and verify tsc --noEmit
- [ ] T068 [P] [US2] Rebuild web/src/routes/tabs/Overview.tsx to render the overview card layout (status chips, resource cards, lifecycle state alerts) from @heroui/react Card/Chip/Alert and @/components/hero/ (StatCard, PhaseChip) per the designed layout for each server state (running, idle armed, asleep, never sleeps, PVC provisioning failed), update Overview.test.tsx keeping all it() cases, and verify tsc --noEmit
- [ ] T069 [P] [US2] Rebuild web/src/routes/tabs/Events.tsx to render the events table and filters from @heroui/react (Table, Input, Select) and @/components/hero/, update Events.test.tsx keeping all it() cases, and verify tsc --noEmit
- [ ] T070 [P] [US2] Rebuild web/src/routes/tabs/Console.tsx to keep the xterm engine and terminal wrapper unchanged and rebuild only the chrome (tab bar, toolbar, status chips, connection state) from HeroUI components, update Console.test.tsx keeping all it() cases, and verify tsc --noEmit
- [ ] T071 [P] [US2] Rebuild web/src/routes/tabs/Logs.tsx to keep the virtualised log view engine unchanged and rebuild only the chrome (tab bar, toolbar, status chips, failed-state alert, search) from HeroUI components, update Logs.test.tsx keeping all it() cases, and verify tsc --noEmit
- [ ] T072 [P] [US2] Rebuild web/src/routes/tabs/Files.tsx to render the file browser (table, breadcrumb, action buttons, new-file/new-folder dialogs) from HeroUI Table/Breadcrumbs/Button and @/components/hero/ dialogs, update Files.test.tsx keeping all it() cases, and verify tsc --noEmit
- [ ] T073 [P] [US2] Rebuild web/src/routes/tabs/Players.tsx to render the players table and filters from @heroui/react (Table, Input, Select) and @/components/hero/, update Players.test.tsx keeping all it() cases, and verify tsc --noEmit
- [ ] T074 [P] [US2] Rebuild web/src/components/server/ServerStatusCard.tsx to render server state, phase chip and lifecycle actions from HeroUI Card/Chip/Button and @/components/hero/ (PhaseChip, StatCard), update ServerStatusCard.test.tsx keeping all it() cases, and verify tsc --noEmit
- [ ] T075 [P] [US2] Rebuild web/src/components/server/ServerSleepCard.tsx to render the sleep state control (sleep/wake buttons, armed/asleep display) from HeroUI Button/Card/Switch and @/components/hero/, update ServerSleepCard.test.tsx keeping all it() cases, and verify tsc --noEmit
- [ ] T076 [P] [US2] Rebuild web/src/components/server/ServerActionsCard.tsx to render action buttons (start/stop/restart/pause/resume) from HeroUI Button and dropdown menus from HeroUI Dropdown/Menu, update ServerActionsCard.test.tsx keeping all it() cases, and verify tsc --noEmit
- [ ] T077 [P] [US2] Rebuild web/src/components/server/ServerActionsMenu.tsx from HeroUI Dropdown/Menu and add a co-located ServerActionsMenu.test.tsx (none exists today) covering the rebuilt markup, and verify tsc --noEmit
- [ ] T078 [P] [US2] Rebuild web/src/components/server/CloneServerDialog.tsx to render the clone form (name input, description, template selector) from HeroUI TextField/Input/Select inside a Modal and add a co-located CloneServerDialog.test.tsx (none exists today) covering the rebuilt markup, and verify tsc --noEmit
- [ ] T079 [P] [US2] Rebuild web/src/components/server/TransferServerDialog.tsx to render the transfer form (destination address/port fields) from HeroUI inputs inside a Modal and add a co-located TransferServerDialog.test.tsx (none exists today) covering the rebuilt markup, and verify tsc --noEmit
- [ ] T080 [P] [US2] Rebuild web/src/components/server/WipeServerDialog.tsx to render the destructive wipe confirmation from HeroUI AlertDialog/Danger with confirmation checkbox and buttons and add a co-located WipeServerDialog.test.tsx (none exists today) covering the rebuilt markup, and verify tsc --noEmit
- [ ] T081 [P] [US2] Rebuild web/src/components/server/DeleteServerDialog.tsx to render the destructive delete confirmation from HeroUI AlertDialog/Danger and add a co-located DeleteServerDialog.test.tsx (none exists today) covering the rebuilt markup, and verify tsc --noEmit
- [ ] T082 [P] [US2] Rebuild web/src/components/server/EventList.tsx to render the events list (cards/rows, pagination) from HeroUI components and add a co-located EventList.test.tsx (none exists today) covering the rebuilt markup, and verify tsc --noEmit
- [ ] T083 [P] [US2] Rebuild web/src/components/server/PortOverridesEditor.tsx to render the port override form (table/rows, add/remove buttons, input validation) from HeroUI components, update PortOverridesEditor.test.tsx keeping all it() cases, and verify tsc --noEmit

### Slice 2a - Verification

- [ ] T084 [US2] Update web/e2e/specs/ Playwright page object matchers and selectors to target HeroUI component roles (button[role='button'], [role='menuitem'], [role='table'], etc.) instead of old class-based queries; verify all mock specs still pass with updated selectors
- [ ] T085 [US2] Add Playwright live specs for Slice 2a flows to web/e2e/specs/live/ (this is the feature's E2E tier per OD-3, Settled 2026-09-03 — no corresponding Go test/e2e/ test is added): sign in, navigate to Servers list, apply phase/template/namespace filters, click row actions, open ServerDetail (all tabs: Overview, Events, Console, Logs, Files, Players), exercise one server action (start/stop), trigger one confirm dialog (Wipe world); specs must drive a real cluster server list and detail tab per the contract
- [ ] T086 [US2] Add screenshot spec entries to web/e2e/screenshots/ (or similar tracking file per contracts/screen-verification.md) for each Slice 2a screen at 1440px reference width: Servers list, ServerDetail/Overview (running), ServerDetail/Overview (idle armed), ServerDetail/Overview (asleep), ServerDetail/Overview (never sleeps), ServerDetail/Overview (PVC provisioning failed), Events, Console, Logs, Files, Players tabs; specs must capture the rendered page for comparison with design-export/screenshots/
- [ ] T087 [US2] Update web/specs.md to describe the HeroUI component layer for Slice 2a screens: replace references to old primitives with HeroUI definitions (Table, Card, Button, Chip, Modal, Dropdown), name the hero/ compositions (ServerStatusCard, PhaseChip, FilterPopover), and note that Console/Logs keep their engines; this update precedes the PR opening
- [ ] T088 [US2] Verify that no rebuilt file in Slice 2a imports from @/components/ui/ or @radix-ui/ (only from @heroui/react and @/components/hero/); grep -rl '@/components/ui/|@radix-ui' web/src --include='*.tsx' must list zero files touched by Slice 2a code commits
- [ ] T089 Apply PR labels to the slice 2a (014c-servers-core) pull request via the REST API per CLAUDE.md rule 14: `gh api -X POST repos/ValgulNecron/Gameplane/issues/<pr-number>/labels -f "labels[]=type: refactor" -f "labels[]=area: web"`; verify with `gh api repos/ValgulNecron/Gameplane/issues/<pr-number>/labels -q '[.[].name]|join(", ")'`.

**Checkpoint**: US2 (slice 2a) fully functional, slice PR green on the `web`, `web-e2e-mock` and `e2e-web-live` jobs, screenshot comparison accepted, merged before the next slice is cut

### Slice 2b - Design wave

- [ ] T090 [US2] Redraw Mods browse, install, and upload screens from HeroUI definitions in design.pen (Table for browse, Modal for dialogs), swapping lunaris to HeroUI variables; keep layout and copy; design.pen node ids sZtDi GayoL KhYNc
- [ ] T091 [US2] Redraw Modpacks and Settings tab screens from HeroUI definitions in design.pen, swapping lunaris to HeroUI variables; keep layout and copy; design.pen node ids Ss0Yr V1VhGE
- [ ] T092 [US2] Redraw Backups tab and backup detail drawer from HeroUI definitions in design.pen (Table/Drawer), swapping lunaris to HeroUI variables; keep layout and copy; design.pen node ids tY6RD pssCT
- [ ] T093 [US2] Redraw Settings sub-pages from HeroUI definitions in design.pen: General, Version, Resources, Networking, Environment (EnvVars), Lifecycle, Scheduled backups, Network capture, Placement, RBAC & access, Danger zone; keep form layout and control arrangement; design.pen node ids Bbnga dBILX xvlB6 m5kOm4 O08uaD b4eaUf hLB9Z swxkJ E0ypH J5pjJ3 ugDSa i1bLR (12 settings screens)
- [ ] T094 [US2] Redraw Capture warning banner and Mods install/upload dialogs (compositions only) from HeroUI definitions in design.pen, swapping lunaris to HeroUI variables; design.pen node ids KaRFX RodrS (additional; note: install/upload dialogs also in slice 2b code)
- [ ] T095 [US2] Redraw remaining Slice 2b screens and verify completeness; design.pen node ids Y5cmvI VfB0Y i8wib f0s9zG KrREo BX0XM (KrREo: Gameplane/Modules — Install Module dialog, BX0XM: Upload Module dialog, per contracts/component-map.md)
- [ ] T096 [US2] Export all redrawn Slice 2b screens, dialogs and compositions — including KrREo (Install Module dialog), which plan.md's slice 2b node-id list carries — to design-export/json/ (depth ≥ 12, zero '...' markers) and design-export/screenshots/, update design-export/MANIFEST.md with rows for each touched node id, verify no '$c:' variable remains in exports, then save design.pen in the Pencil GUI (do not commit; the maintainer will save and commit)

### Slice 2b - Code wave

- [ ] T097 [P] [US2] Rebuild web/src/routes/tabs/Mods.tsx to render the mods browse table (name, version, installed, actions) from HeroUI Table/Button/Dropdown and @/components/hero/, wire the install dialog, update Mods.test.tsx keeping all it() cases, and verify tsc --noEmit
- [ ] T098 [P] [US2] Rebuild web/src/routes/tabs/Modpacks.tsx to render modpack selection/management from HeroUI components, update Modpacks.test.tsx keeping all it() cases, and verify tsc --noEmit
- [ ] T099 [P] [US2] Rebuild web/src/routes/tabs/Backups.tsx to render the backups table (name, date, status, actions) from HeroUI Table/Button and backup detail drawer trigger, update Backups.test.tsx keeping all it() cases, and verify tsc --noEmit
- [ ] T100 [P] [US2] Rebuild web/src/routes/tabs/Settings.tsx to render the settings sub-section tabs (Access, General, Version, Resources, Networking, Environment, Lifecycle, Backups, Network Capture, Placement, Danger) as a Tabs component, update Settings.test.tsx keeping all it() cases, and verify tsc --noEmit
- [ ] T101 [P] [US2] Rebuild web/src/routes/tabs/settings/Access.tsx to render RBAC controls from HeroUI form components, keep state hooks/handlers verbatim and swap only rendered controls (input/select/switch), update test keeping all it() cases, and verify tsc --noEmit
- [ ] T102 [P] [US2] Rebuild web/src/routes/tabs/settings/General.tsx to render server name, description, template fields from HeroUI TextField/Input, keep state handlers verbatim, update test keeping all it() cases, and verify tsc --noEmit
- [ ] T103 [P] [US2] Rebuild web/src/routes/tabs/settings/Version.tsx to render version selector and rollback controls from HeroUI Select/Button, keep state handlers verbatim, update test keeping all it() cases, and verify tsc --noEmit
- [ ] T104 [P] [US2] Rebuild web/src/routes/tabs/settings/Resources.tsx to render CPU/memory/storage sliders and inputs from HeroUI Slider/NumberField/TextField, keep state handlers verbatim, update test keeping all it() cases, and verify tsc --noEmit
- [ ] T105 [P] [US2] Rebuild web/src/routes/tabs/settings/Networking.tsx to render network config (ports, address pool, firewall rules) from HeroUI Table/Input/Button, keep state handlers verbatim, update test keeping all it() cases, and verify tsc --noEmit
- [ ] T106 [P] [US2] Rebuild web/src/routes/tabs/settings/EnvVars.tsx to render environment variable editor (key/value table, add/remove) from HeroUI Table/TextField/Button, keep state handlers verbatim, update test keeping all it() cases, and verify tsc --noEmit
- [ ] T107 [P] [US2] Rebuild web/src/routes/tabs/settings/Lifecycle.tsx to render lifecycle policy controls (auto-pause, quiesce strategy, idle thresholds) from HeroUI Switch/Select/Slider, keep state handlers verbatim, update test keeping all it() cases, and verify tsc --noEmit
- [ ] T108 [P] [US2] Rebuild web/src/routes/tabs/settings/Backups.tsx to render backup configuration (retention, retention-days input) from HeroUI TextField/Select, keep state handlers verbatim, update test keeping all it() cases, and verify tsc --noEmit
- [ ] T109 [P] [US2] Rebuild web/src/routes/tabs/settings/NetworkCapture.tsx to render capture config (enabled toggle, filter input, retention policy) from HeroUI Switch/TextField/Select, keep state handlers verbatim, update test keeping all it() cases, and verify tsc --noEmit
- [ ] T110 [P] [US2] Rebuild web/src/routes/tabs/settings/Placement.tsx to render node affinity controls from HeroUI form components, keep state handlers verbatim, update test keeping all it() cases, and verify tsc --noEmit
- [ ] T111 [P] [US2] Rebuild web/src/routes/tabs/settings/Danger.tsx to render destructive action buttons (Delete, Wipe world, Transfer, Revoke backups) from HeroUI Button/variant='danger', wire confirm dialogs, update test keeping all it() cases, and verify tsc --noEmit
- [ ] T112 [P] [US2] Rebuild web/src/components/CaptureWidget.tsx to render capture status, warning banner (from @/components/hero/ CaptureWarningBanner), and download/stop buttons from HeroUI components, update test keeping all it() cases, and verify tsc --noEmit
- [ ] T113 [P] [US2] Rebuild web/src/components/registry-browser.tsx to render a registry/image browser (list/search/filter) from HeroUI Table/SearchField/Chip, update test keeping all it() cases, and verify tsc --noEmit
- [ ] T114 [P] [US2] Rebuild web/src/components/modules/InstallDialog.tsx to render the module install form (name, version selector, confirm button) inside HeroUI Modal, update InstallDialog.test.tsx keeping all it() cases, and verify tsc --noEmit
- [ ] T115 [P] [US2] Rebuild web/src/components/modules/UploadModuleDialog.tsx to render the file upload form (file input, version field) inside HeroUI Modal, update UploadModuleDialog.test.tsx keeping all it() cases, and verify tsc --noEmit

### Slice 2b - Verification

- [ ] T116 [US2] Update web/e2e/specs/ Playwright selectors for Slice 2b components (Mods table, Backups drawer, Settings tabs/forms) to target HeroUI roles and semantic queries; verify all updated specs still pass
- [ ] T117 [US2] Add Playwright live specs for Slice 2b flows to web/e2e/specs/live/ (this is the feature's E2E tier per OD-3, Settled 2026-09-03 — no corresponding Go test/e2e/ test is added): open server detail, navigate through Mods/Modpacks/Backups tabs, open all Settings sub-sections, save one settings field (e.g., server name), discard a change, and re-open to verify state; specs must drive the real cluster
- [ ] T118 [US2] Add screenshot spec entries to web/e2e/screenshots/ for each Slice 2b screen at 1440px reference width: Mods tab, Modpacks tab, Backups tab, Settings/General, Settings/Version, Settings/Resources, Settings/Networking, Settings/EnvVars, Settings/Lifecycle, Settings/Backups, Settings/NetworkCapture, Settings/Placement, Settings/Access, Settings/Danger, backup detail drawer (open state); specs must capture for comparison with design-export/screenshots/
- [ ] T119 [US2] Update web/specs.md to describe Slice 2b HeroUI components and compositions: replace old primitive references with HeroUI Table/Modal/Drawer/Form components, note registry-browser and CaptureWidget chrome rebuild, name the module dialogs and settings form structure, and describe the per-section state preservation rule; this update precedes the PR opening
- [ ] T120 [US2] Verify that no rebuilt file in Slice 2b imports from @/components/ui/ or @radix-ui/; grep -rl '@/components/ui/|@radix-ui' web/src --include='*.tsx' must list zero files touched by Slice 2b code commits
- [ ] T121 Apply PR labels to the slice 2b (014d-server-settings) pull request via the REST API per CLAUDE.md rule 14: `gh api -X POST repos/ValgulNecron/Gameplane/issues/<pr-number>/labels -f "labels[]=type: refactor" -f "labels[]=area: web"`; verify with `gh api repos/ValgulNecron/Gameplane/issues/<pr-number>/labels -q '[.[].name]|join(", ")'`.

**Checkpoint**: US2 (slice 2b) fully functional, slice PR green on the `web`, `web-e2e-mock` and `e2e-web-live` jobs, screenshot comparison accepted, merged before the next slice is cut

---

## Phase 5: User Story 3 - Create a server and manage modules and backups (Priority: P2)

**Slice**: 3 Create, modules, backups (branch 014e-create-modules-backups)

**Goal**: Rebuild the Create Server wizard, Modules catalog, and Backups surfaces from HeroUI components following their design frames (12 screens: Create 5 steps, Modules 3 catalog screens, Backups 4 sub-pages), preserving all form validation, wizard flow, dialog completions, and backup operations (US3, P2).

**Independent Test**: Create a server through all five wizard steps including address-pool fields on Network step, install one module from catalog and upload one, add a module source, create a backup schedule, and open one backup's detail drawer and restore dialog; each screen matches its design frame and each flow completes end-to-end.

### Design pass — Create Server, Modules, Backups

- [ ] T122 [US3] Redraw Create Server wizard steps 1–3 (Template, Version, Configure) in design.pen from HeroUI components per contracts/component-map.md, swap lunaris $c: variables to HeroUI variables ($accent/*, $surface/*, $foreground/*, $danger/*, etc.), set HeroUI variables to Gameplane brand values in light and dark semantic modes (per contracts/theme-tokens.md), re-export JSON at depth ≥12 with zero '...' markers and PNG at 2× to design-export/ for node ids W8idqY, nNL3E, vUqMl; maintainer saves design.pen in Pencil GUI after this commit lands.
- [ ] T123 [US3] Redraw Create Server wizard steps 4–5 (Network, Review) in design.pen from HeroUI components, including address-pool and requested-address optional inputs and alerts on step 4 per spec.md User Story 3 acceptance scenario 1 (Network step address-pool fields), swap variables to HeroUI set, re-export JSON depth ≥12 (zero '...') and PNG for node ids f1Vga, UMJli.
- [ ] T124 [US3] Redraw Modules catalog list, install dialog, and upload dialog screens in design.pen from HeroUI components (Modals, tables, cards, buttons, chips per contracts/component-map.md), swap variables to HeroUI, re-export JSON depth ≥12 (zero '...') and PNG for node ids kK8Ji, DPrYX, fK8Bi.
- [ ] T125 [US3] Redraw Backups Index, Schedules, Restores, and detail drawer screens in design.pen from HeroUI components (tables, filters, modals, forms per contracts/component-map.md), swap variables to HeroUI, re-export JSON depth ≥12 (zero '...') and PNG for node ids tTSdi, zhLZN, E9EEv0, DMnEi; update design-export/MANIFEST.md to add or verify rows for all 12 slice-3 screen ids (W8idqY, nNL3E, vUqMl, f1Vga, UMJli, kK8Ji, DPrYX, fK8Bi, tTSdi, zhLZN, E9EEv0, DMnEi) with non-empty JSON/PNG files and zero '"..."' markers verified by grep.

### Code wave — Routes

- [ ] T126 [US3] Rebuild web/src/routes/CreateServer.tsx on @heroui/react, importing only from @heroui/react and @/components/hero/ (never @/components/ui/ or @radix-ui/*), replace hand-rolled inputs/selects/buttons/dialogs with HeroUI TextField, Select, Button, Modal, Alert, Checkbox per component-map.md, preserve five-step wizard validation logic and progress-blocking on each step, update co-located CreateServer.test.tsx, CreateServer_more.test.tsx, CreateServer_tunnel.test.tsx to match new markup (no tests deleted, counts may only rise), end with tsc --noEmit.
- [ ] T127 [US3] Rebuild web/src/routes/Modules.tsx on @heroui/react, importing from @heroui/react and @/components/hero/, replace table/card/dialog markup with HeroUI equivalents (Table, Card, Modal per component-map.md), preserve catalog browsing, install/upload/source-add flows, update Modules.test.tsx (no tests deleted), run tsc --noEmit.
- [ ] T128 [US3] Rebuild web/src/routes/Backups.tsx (main Backups index page) on @heroui/react, importing from @heroui/react and @/components/hero/, replace table/filter/action markup with HeroUI components, preserve multi-cluster context via query params and cluster state, update Backups.test.tsx and Backups_flows.test.tsx (no tests deleted), run tsc --noEmit. (web/src/routes/tabs/Backups.tsx, the server-detail Backups TAB, is a distinct file already rebuilt in slice 2b's T099 per plan.md — this task does not touch it.)

### Code wave — Modules components

- [ ] T129 [US3] Rebuild web/src/components/modules/ModuleCard.tsx on @heroui/react, importing from @heroui/react and @/components/hero/, replace card/button/action markup with HeroUI Card, Button, Dropdown per component-map.md, update ModuleCard.test.tsx (no tests deleted), run tsc --noEmit.
- [ ] T130 [US3] Rebuild web/src/components/modules/ModuleSourcesPanel.tsx on @heroui/react, importing from @heroui/react and @/components/hero/, replace panel/list/button markup with HeroUI components, update ModuleSourcesPanel.test.tsx (no tests deleted), run tsc --noEmit.
- [ ] T131 [US3] Rebuild web/src/components/modules/SourceDialog.tsx on @heroui/react, importing from @heroui/react and @/components/hero/, replace dialog/input/button markup with HeroUI Modal, TextField, Button per component-map.md, preserve dialog form validation and cancel/submit flows, update SourceDialog.test.tsx (no tests deleted), run tsc --noEmit. (web/src/components/modules/{InstallDialog,UploadModuleDialog}.tsx are already rebuilt in slice 2b's T114/T115 per plan.md — this slice consumes the slice-2b hero versions and does not re-rebuild them.)

### Code wave — Backups components

- [ ] T132 [US3] Rebuild web/src/components/backups/BackupRow.tsx on @heroui/react, importing from @heroui/react and @/components/hero/, replace table row/button/status markup with HeroUI TableCell, Button, Chip per component-map.md, update BackupRow.test.tsx (no tests deleted), run tsc --noEmit.
- [ ] T133 [US3] Rebuild web/src/components/backups/BackupFilters.tsx on @heroui/react, importing from @heroui/react and @/components/hero/, replace filter popover/input/button markup with HeroUI Popover, Input, Button, Checkbox per component-map.md, preserve filter state and reset logic, update BackupFilters.test.tsx (no tests deleted), run tsc --noEmit.
- [ ] T134 [US3] Rebuild web/src/components/backups/RestoreDialog.tsx on @heroui/react, importing from @heroui/react and @/components/hero/, replace modal/select/button markup with HeroUI Modal, Select, Button, Alert per component-map.md, preserve form validation and restore flow, update RestoreDialog.test.tsx (no tests deleted), run tsc --noEmit.
- [ ] T135 [US3] Rebuild web/src/components/backups/BackupDetailDrawer.tsx on @heroui/react, importing from @heroui/react and @/components/hero/, replace drawer/card/button markup with HeroUI Drawer (or Modal as left-anchored per component-map.md), Card, Button components, preserve drawer state and close logic, update BackupDetailDrawer.test.tsx (no tests deleted), run tsc --noEmit.
- [ ] T136 [US3] Rebuild web/src/components/backups/ScheduleForm.tsx, RetentionFields.tsx, ErrorBanner.tsx on @heroui/react, importing from @heroui/react and @/components/hero/, replace form/input/alert markup with HeroUI TextField, Select, Checkbox, Alert per component-map.md, preserve form state and validation, update all three test files (no tests deleted), run tsc --noEmit.

### Verification and compliance

- [ ] T137 [US3] Create web/e2e/screenshots/slice-3.spec.ts Playwright screenshot spec covering all 12 slice-3 screens (Create Server wizard steps 1–5 via /servers/new, Modules catalog at /modules, Backups Index/Schedules/Restores/detail at /backups and /servers/{name}:backups) at 1440×900 viewport, dark appearance, MSW fixtures that reproduce design frame sample data (server names, phases, counts); upload Playwright artifacts on every run including screenshots for design-export comparison per contracts/screen-verification.md.
- [ ] T138 [US3] Update web/specs.md to describe HeroUI-based Create Server wizard (form controls, validation per step), Modules catalog (browsing, install/upload/source dialogs), and Backups surfaces (index table, filters, detail drawer, restore dialog, schedule form), documenting the component family change from lunaris primitives + Radix to HeroUI per FR-013.
- [ ] T139 [US3] Verify slice-3 compliance: grep -rl '@/components/ui/|@radix-ui' web/src/routes/CreateServer.tsx web/src/routes/Modules.tsx web/src/routes/Backups.tsx web/src/components/modules web/src/components/backups --include='*.tsx' | grep -v test must return zero matches (web/src/routes/tabs/Backups.tsx is out of scope for this slice — it was rebuilt in slice 2b); design-export files must verify via python3 -c 'import json,sys; [json.load(open(f)) for f in sys.argv[1:]]' on all 12 slice-3 JSON ids and grep -l '"..."' must print nothing; MANIFEST.md must carry a row per screen id with non-empty JSON/PNG file sizes.
- [ ] T140 Apply PR labels to the slice 3 (014e-create-modules-backups) pull request via the REST API per CLAUDE.md rule 14: `gh api -X POST repos/ValgulNecron/Gameplane/issues/<pr-number>/labels -f "labels[]=type: refactor" -f "labels[]=area: web"`; verify with `gh api repos/ValgulNecron/Gameplane/issues/<pr-number>/labels -q '[.[].name]|join(", ")'`.

**Checkpoint**: US3 fully functional, slice PR green on the `web`, `web-e2e-mock` and `e2e-web-live` jobs, screenshot comparison accepted, merged before the next slice is cut

---

## Phase 6: User Story 4 - Administer the installation (Priority: P3)

**Slice**: 4 Admin (branch 014f-admin)

**Goal**: Complete Phase 6 implementation: rebuild the Admin, Users, Audit, and Cluster routes and their components from HeroUI design system components, with all design screens exported and all tests updated in-task.

**Independent Test**: As an admin, open every admin route (/admin/settings, /admin/users, /admin/audit, /admin/logs, /admin/cluster) and each sub-page/section within them, verify the UI renders from HeroUI components (no old primitives imported), create an invite, edit a role, add and remove an OIDC role-mapping override (hitting the admin-mapping confirm dialog and the save-rejected state), verify the audit chain, and download system logs — all operations must complete successfully with no behavioral regression from today.

### Design Wave: Redraw admin screens from HeroUI definitions

- [ ] T141 [US4] Redraw design screens WZdnw uMiwd nNGDX QgW58 zqzr4 RC3Kf g5mEpx (Admin Settings main page and sub-pages) from HeroUI definitions per contracts/component-map.md, swap all $c: fills/text to HeroUI variables, re-export JSON at depth ≥12 with zero "..." markers to design-export/json/<id>.json and PNG to design-export/screenshots/<id>.png, then update design-export/MANIFEST.md.
- [ ] T142 [US4] Redraw design screens Wj0V4 n6Xlo uoxQW M2sA4u zM0VF bYDHC (Users, RBAC, role dialogs) from HeroUI definitions per contracts/component-map.md, re-export JSON and PNG, update design-export/MANIFEST.md.
- [ ] T143 [US4] Redraw design screens e9lV4 TBvTC Dpb9f DxKOh (Audit Log, System Logs, Cluster pages) from HeroUI definitions per contracts/component-map.md, re-export JSON and PNG, update design-export/MANIFEST.md.
- [ ] T144 [US4] Redraw design screens Bq2Yg j9W8A dxdEi kIxaJ CqaSq NLDDv t3IY3u MaoHP Kp48V uw0dB XL5ZU vStkb R65Xyx Rwnu3 BV5ei (dialogs, modals, badges, chips, banners for admin features) from HeroUI definitions per contracts/component-map.md, re-export JSON and PNG, update design-export/MANIFEST.md.
- [ ] T145 [US4] Save design.pen in Pencil GUI after all design edits are complete (required before design commit per Constitution II).

### Composition & Component Work: Build hero/ compositions

- [ ] T146 [US4] Create web/src/components/hero/AuditIntegrityBanner.tsx implementing the Gameplane/Audit Integrity Banner composition (node kIxaJ) from HeroUI Alert/Card components, per contracts/component-map.md, with co-located AuditIntegrityBanner.test.tsx covering the component's rendered output.
- [ ] T147 [US4] Create web/src/components/hero/RoleEditorModal.tsx implementing the Gameplane/Role Editor Modal composition (node CqaSq) using HeroUI Modal/AlertDialog components with form controls, per contracts/component-map.md, with co-located RoleEditorModal.test.tsx.
- [ ] T148 [US4] Create web/src/components/hero/admin/ directory with InviteUserDialog.tsx (node NLDDv), EditUserDialog.tsx (node t3IY3u), and ResetPasswordDialog.tsx (node MaoHP) as HeroUI Modal compositions with form controls, each with co-located .test.tsx files.
- [ ] T149 [US4] Create web/src/components/hero/ConfirmAdminMappingDialog.tsx implementing the Gameplane/Confirm Admin Mapping composition (node Kp48V) using HeroUI AlertDialog, per contracts/component-map.md, with co-located ConfirmAdminMappingDialog.test.tsx.
- [ ] T150 [US4] Create web/src/components/hero/RemovableGroupChip.tsx implementing the Gameplane/Removable Group Chip composition (nodes uw0dB XL5ZU vStkb) using HeroUI Chip components with close button, per contracts/component-map.md, with co-located RemovableGroupChip.test.tsx.
- [ ] T151 [US4] Create web/src/components/hero/ProvenanceBadge.tsx implementing the Gameplane/Provenance Badge composition (nodes R65Xyx Rwnu3 BV5ei) as HeroUI Chip/Badge variants showing data provenance (manual, OIDC, etc.), per contracts/component-map.md, with co-located ProvenanceBadge.test.tsx.

### Code Wave: Rebuild route components from HeroUI

- [ ] T152 [US4] Rewrite web/src/routes/AdminSettings.tsx to import from @heroui/react and @/components/hero/ (never @/components/ui/ or @radix-ui/*), render all sections and forms using HeroUI Input/Select/Switch/Card/Modal components per contracts/component-map.md, preserve all state and handlers verbatim, and update web/src/routes/AdminSettings.test.tsx to the new markup without deleting any it() block.
- [ ] T153 [US4] Rewrite the Authentication, Backup Destinations, Module Sources, Mod Registries, Notifications, Telemetry, Updates, and About sub-sections in place inside web/src/routes/AdminSettings.tsx — plan.md lists only this single file for slice 4, so no sub-section files are created or split out — using HeroUI Card/Tabs/form components, and update web/src/routes/AdminSettings.test.tsx to cover each sub-section without deleting any it() block.
- [ ] T154 [US4] Rewrite web/src/routes/Users.tsx to import from @heroui/react and @/components/hero/, render users table, roles list, invite/edit/reset-password dialogs using HeroUI Modal/Table/Button components per contracts/component-map.md, preserve permission gates and RBAC logic, and update web/src/routes/Users.test.tsx without deleting tests.
- [ ] T155 [US4] Update web/src/routes/Users_invite.test.tsx and web/src/routes/Users_tabs.test.tsx to work with the new HeroUI Modal and form components in Users.tsx, preserving test count per FR-010.
- [ ] T156 [US4] Rewrite web/src/routes/AuditLog.tsx to import from @heroui/react and @/components/hero/, render audit table and integrity banner using HeroUI Table/Alert/Pagination components per contracts/component-map.md, preserve export/verify actions and pagination, and update web/src/routes/AuditLog.test.tsx without deleting tests.
- [ ] T157 [US4] Rewrite web/src/routes/AdminLogs.tsx to import from @heroui/react and @/components/hero/, render system logs interface using HeroUI Table/Spinner/Card/Button components per contracts/component-map.md, preserve log streaming and download actions, and update web/src/routes/AdminLogs.test.tsx without deleting tests.
- [ ] T158 [US4] Rewrite web/src/routes/Cluster.tsx to import from @heroui/react and @/components/hero/, render cluster settings form using HeroUI Input/Card/Button components per contracts/component-map.md, preserve save/discard behavior and conflict detection, and update web/src/routes/Cluster.test.tsx without deleting tests.

### Verification & Playwright Updates

- [ ] T159 [US4] Update web/e2e/specs/screenshots.spec.ts to capture HeroUI-rebuilt admin screens at 1440×900 for every screen id in Slice 4 (WZdnw uMiwd nNGDX QgW58 zqzr4 RC3Kf g5mEpx Wj0V4 n6Xlo uoxQW M2sA4u zM0VF bYDHC e9lV4 TBvTC Dpb9f DxKOh Bq2Yg j9W8A dxdEi kIxaJ CqaSq NLDDv t3IY3u MaoHP Kp48V uw0dB XL5ZU vStkb R65Xyx Rwnu3 BV5ei), using MSW fixtures to reproduce each state, per contracts/screen-verification.md.
- [ ] T160 [US4] Verify FR-012 by confirming no file under web/src/routes or web/src/components rebuilt in this slice imports from @/components/ui/ or @radix-ui/*; any import violation fails the review.
- [ ] T161 [US4] Run cd web && npx tsc --noEmit to verify TypeScript compilation of all rebuilt admin files.

### Documentation & Closeout

- [ ] T162 [US4] Update web/specs.md to document the rebuilt admin component layer (replacing old primitives with HeroUI definitions for Admin Settings, Users, Audit, System Logs, and Cluster routes), reflecting the new family per FR-013.
- [ ] T163 Apply PR labels to the slice 4 (014f-admin) pull request via the REST API per CLAUDE.md rule 14: `gh api -X POST repos/ValgulNecron/Gameplane/issues/<pr-number>/labels -f "labels[]=type: refactor" -f "labels[]=area: web"`; verify with `gh api repos/ValgulNecron/Gameplane/issues/<pr-number>/labels -q '[.[].name]|join(", ")'`.

**Checkpoint**: US4 fully functional, slice PR green on the `web`, `web-e2e-mock` and `e2e-web-live` jobs, screenshot comparison accepted, merged before the next slice is cut

---

## Phase 7: User Story 5 - Use a public share link (Priority: P3)

**Slice**: 5 Share links (branch 014g-share-links-retire), new construction half

**Goal**: Implement User Story 5: public share-link surfaces (Settings section with create/list/revoke dialogs, public page in five states) built directly on HeroUI, per contracts/share-link-ui.md, in branch 014g-share-links-retire.

**Independent Test**: Create a share link for a server, open it signed out in each of the five states (Up, Asleep with start available, Asleep view-only, Starting, Invalid or expired), and confirm each matches its design frame per contracts/screen-verification.md; then revoke the link and verify the page renders the neutral invalid state.

### Design: Settings section and dialogs

- [ ] T164 [US5] Redraw ShareLinks settings section and empty state from HeroUI definitions in design.pen (node ids xCJlu dQV9N), importing table/dialog/chip/button definitions from LtgNm frame; keep table layout with link label, created date, expiry, capabilities, status columns; empty state per dQV9N design.
- [ ] T165 [US5] Redraw Create Share Link modal dialog from HeroUI (node id atqRh): label input, expiry select/toggle, allow-start switch; follow Modal/Primary definition; set HeroUI variables to Gameplane tokens (dark mode shown).
- [ ] T166 [US5] Redraw Share Link Created and Revoke dialogs from HeroUI (node ids VM7ro S7SCDc): VM7ro as Modal/Primary showing full URL with copy-button action and warning text; S7SCDc as AlertDialog/Danger with revoke confirmation copy.
- [ ] T167 [US5] Export share-link design screens to design-export/json/{xCJlu,dQV9N,atqRh,VM7ro,S7SCDc}.json (depth ≥ 12, no '...' markers) and design-export/screenshots/{xCJlu,dQV9N,atqRh,VM7ro,S7SCDc}.png; update design-export/MANIFEST.md with one row per id; then save design.pen in the Pencil GUI before proceeding to code.

### Design: Public share page states

- [ ] T168 [US5] Redraw public share page Up state (node id C2LQE4) from HeroUI definitions: server status, address/port display, players list if exposed; use Card/Default, Chip, Badge from LtgNm; no cluster/namespace/version shown per FR-005.
- [ ] T169 [US5] Redraw public share page Asleep states (node ids q31B6w qFLfB): q31B6w showing server asleep with Start button, qFLfB as view-only variant without button; both from HeroUI Button/Primary, Card, Alert definitions.
- [ ] T170 [US5] Redraw public share page Starting and Invalid states (node ids EcoGD epZO2): EcoGD showing Starting with spinner and polling indicator (use Spinner definition), epZO2 as Invalid/expired with neutral copy and no detail beyond 'link unavailable'.
- [ ] T171 [US5] Export public share page frames to design-export/json/{C2LQE4,q31B6w,qFLfB,EcoGD,epZO2}.json (depth ≥ 12, no '...' markers) and design-export/screenshots/{C2LQE4,q31B6w,qFLfB,EcoGD,epZO2}.png; update design-export/MANIFEST.md with one row per id.

### Foundation: Types and API layer

- [ ] T172 [P] [US5] Add ShareLink type (id, label, createdAt, expiresAt, allowStart, status, token) and ShareLinkPublic response type to web/src/types.ts mirroring api/internal/handlers/shares.go response shapes; add ShareLinkCreateRequest for the create endpoint payload.
- [ ] T173 [P] [US5] Add Shares namespace to web/src/lib/endpoints.ts with create(server, cluster?), list(server, cluster?), revoke(server, id, cluster?), resolve(token), start(token) endpoint paths matching the API handler routes.
- [ ] T174 [P] [US5] Add Shares namespace to web/src/lib/api.ts with create/list/revoke functions (authenticated, use mutation helpers) and resolve/start functions (public, use query/mutation with rate-limit error mapping to neutral state); write web/src/lib/api.test.ts cases for all Shares functions including error states per FR-005.

### UI: Settings section with create, created, and revoke dialogs

- [ ] T175 [US5] Create web/src/routes/tabs/settings/ShareLinks.tsx as a new settings section importing from @heroui/react, @/components/hero/, rendering link table (label, created, expires, capabilities, status, Revoke action), empty state, and Create button opening the create dialog; handle list query, delete mutation, dialog state; update co-located ShareLinks.test.tsx testing all states, empty case, delete flow with matching test count.
- [ ] T176 [US5] Implement Create Share Link dialog inside ShareLinks.tsx as a HeroUI Modal with label Input, expiry Select, allow-start Switch, and Create button; submit via the create mutation; show validation errors; update ShareLinks.test.tsx with dialog open/close and submit tests.
- [ ] T177 [US5] Implement Share Link Created confirmation dialog inside ShareLinks.tsx as HeroUI Modal displaying the full URL with a copy-to-clipboard button and 'won't be shown again' warning; include test for copy action in ShareLinks.test.tsx.
- [ ] T178 [US5] Implement Revoke Share Link dialog inside ShareLinks.tsx as HeroUI AlertDialog/Danger confirming revocation; trigger via table row action; call revoke mutation; update ShareLinks.test.tsx with revoke confirmation and after-delete table refresh test.
- [ ] T179 [US5] Add ShareLinks section to web/src/routes/tabs/Settings.tsx between Access and Danger sections, rendering the ShareLinks component conditionally on write permission; import ShareLinks from './settings/ShareLinks'; update Settings.test.tsx with ShareLinks section rendering test.
- [ ] T180 [US5] Run tsc --noEmit in web/ directory to verify all ShareLinks types and Settings integration compile without errors.

### UI: Public share page with five states

- [ ] T181 [US5] Create web/src/routes/Share.tsx as a public route outside the authenticated layout with no sidebar or top bar, importing from @heroui/react and @/components/hero/; fetch the share token from URL params; call resolve endpoint; render states: Up (address/port/players), Asleep-can-start (Start button), Asleep-view-only, Starting (with polling), Invalid/expired (neutral copy).
- [ ] T182 [US5] Implement Up state in Share.tsx rendering Card with server status, address, port, optional players list from the resolve response; use Chip for status, Link for address if clickable per design; respect FR-005 privacy (no cluster/namespace/version/user names).
- [ ] T183 [US5] Implement Asleep states in Share.tsx: one with Start button (async, transitions to Starting with polling) and one view-only without button; both show server name and 'asleep' message; use Button/Primary for Start, Spinner during async transition.
- [ ] T184 [US5] Implement Starting state in Share.tsx with Spinner and 'server starting' message; poll the resolve endpoint until server is up or link expires, then transition to Up or Invalid; implement polling cancellation on unmount.
- [ ] T185 [US5] Implement Invalid/expired state in Share.tsx showing neutral copy 'This link is not available' without revealing whether the token was valid, revoked, or expired; map all error states (404, rate-limit 429, any auth error) to the same neutral message per FR-005.
- [ ] T186 [US5] Add honor-appearance-preference logic to Share.tsx reading the stored appearance from localStorage (light/dark/system) and applying it via the stored data-theme/class on html, matching the AppLayout behavior but without exposing a toggle; add Share.test.tsx testing all five states, error cases, and polling behavior with 15+ cases.
- [ ] T187 [US5] Run tsc --noEmit in web/ directory to verify Share.tsx and all type references compile without errors.

### Router: Register public share route

- [ ] T188 [US5] Add public share route to web/src/router/tree.tsx with path /share/$token per OD-1 (Settled 2026-09-03); route points to Share component; place outside the authenticated layout group so it never redirects to login; verify path does not collide with existing routes.

### Testing: Playwright mock and live specs

- [ ] T189 [US5] Create web/e2e/specs/slice5.spec.ts for mock mode covering ShareLinks section: list state, create dialog open/close/submit, created dialog showing URL, revoke dialog and confirmation; and public page states (Up, Asleep-can-start with Start action, Asleep-view-only, Starting with polling, Invalid/expired); use MSW fixtures matching design data.
- [ ] T190 [US5] Add share-link live E2E specs to web/e2e/specs/live/ (this is the feature's E2E tier per OD-3, Settled 2026-09-03 — no corresponding Go test/e2e/ test is added) covering: create a link, open it signed out in all reachable states, revoke it on the authenticated side, reopen signed out and verify Invalid state; include start action on a test server if available; file may be 'shareLinksFlow.spec.ts'.
- [ ] T191 [US5] Add all seven share-link screen ids (xCJlu, dQV9N, atqRh, VM7ro, S7SCDc, C2LQE4, q31B6w, qFLfB, EcoGD, epZO2) to web/e2e/specs/screenshots.spec.ts with routes and fixture data to reach each state; set viewport 1440×900, deviceScaleFactor: 2, dark appearance; update existing Playwright screenshot project to always() upload artifacts.

### Verification and module spec

- [ ] T192 [US5] Compare Playwright screenshots from web/e2e/specs/screenshots.spec.ts against design-export/screenshots/{xCJlu,dQV9N,atqRh,VM7ro,S7SCDc,C2LQE4,q31B6w,qFLfB,EcoGD,epZO2}.png per contracts/screen-verification.md; record one verdict per screen in PR description (match/layout-mismatch/colour-mismatch/component-family-mismatch/content-mismatch); accept only match or content-mismatch.
- [ ] T193 [US5] Update web/specs.md: add Share links entry under Routing section (path, public access, no auth required, five states) and under Settings sub-sections (ShareLinks tab, create/list/revoke, permission gate); remove any reference to old share-link primitives if any exist.
- [ ] T194 [US5] Verify FR-005 privacy rule: the public Share.tsx page renders no cluster name, namespace, version string, user names, counts, or server enumeration hints; review the Invalid state and all error mappings in the code match the contract.
- [ ] T195 [US5] Verify FR-012 import rule: grep -rl '@/components/ui/\|@radix-ui' web/src/routes/Share.tsx web/src/routes/tabs/settings/ShareLinks.tsx web/src/lib/api.ts web/src/types.ts must return nothing; all new files import from @heroui/react or @/components/hero/ only.
- [ ] T196 [US5] Record in PR description: Pencil node ids touched (xCJlu dQV9N C2LQE4 q31B6w qFLfB EcoGD epZO2 atqRh VM7ro S7SCDc), screen verification verdicts table, FR-005 privacy check result, FR-012 import check result, test-count verification (ShareLinks.test.tsx, Share.test.tsx, api.test.ts updated, no test files deleted).

**Checkpoint**: US5 fully functional, slice PR green on the `web`, `web-e2e-mock` and `e2e-web-live` jobs, screenshot comparison accepted, merged before the next slice is cut

---

## Phase 8: Polish & Cross-Cutting Concerns (retirement, slice 5 second half)

**Slice**: 5 Retirement (same branch 014g-share-links-retire)

**Goal**: Complete the retirement phase of the HeroUI rebuild by removing all old primitives, verifying zero consumers remain, and delivering the feature with updated documentation and verified test coverage.

**Independent Test**: Verify slice 5 is mergeable: (1) grep web/src for @/components/ui/ and @radix-ui imports returns no results, (2) web/src/components/ui/ (old) directory is deleted, (3) web/package.json has no @radix-ui or class-variance-authority entries, (4) web/specs.md contains zero references to old primitives (grep for lunaris, Radix, badge.tsx, button.tsx, card.tsx, etc.), (5) design-export/MANIFEST.md updated for all slice 5 screens, (6) web/src/components/hero/ is renamed to web/src/components/ui/ per OD-7 (Settled 2026-09-03), (7) keyboard-only navigation of login, servers list, and server settings completes without focus traps or unlabeled controls, (8) Playwright and Vitest suites pass on CI with coverage at or above 92/76/82/92, (9) PR carries type: refactor + area: web + type: feature labels.

### Search and Deletion

- [ ] T197 Search web/src/ for remaining imports from @/components/ui/ or @radix-ui/*; log file paths and line counts for review before deletion.
- [ ] T198 Delete web/src/components/ui/ directory and all its contents (37 files: button.tsx, card.tsx, input.tsx, select.tsx, textarea.tsx, switch.tsx, tabs.tsx, confirm-dialog.tsx, dropdown-menu.tsx, badge.tsx, stat.tsx, meter.tsx, sparkline.tsx, game-icon.tsx, slack-icon.tsx, slider.tsx, resource-input.tsx, password-input.tsx, field.tsx, plus all .test.tsx files).
- [ ] T199 Update web/package.json to remove @radix-ui/react-dialog, @radix-ui/react-dropdown-menu, @radix-ui/react-label, @radix-ui/react-slot, @radix-ui/react-tabs, react-toast, and class-variance-authority from dependencies; run npm ci and verify no errors.

### Configuration and Rename

- [ ] T200 Check web/tailwind.config.ts; if Tailwind 4 CSS-first @theme blocks in globals.css make this file redundant, delete it; otherwise leave in place per plan.
- [ ] T201 Per OD-7 (Settled 2026-09-03), rename web/src/components/hero/ to web/src/components/ui/ to become the new primitive layer (this runs only after T198 deletes the old web/src/components/ui/); update all imports across web/src to reference @/components/ui/ instead of @/components/hero/.

### Documentation Updates

- [ ] T202 Update web/specs.md: remove every mention of lunaris primitives, Radix, and old component files (badge.tsx, button.tsx, card.tsx, confirm-dialog.tsx, dropdown-menu.tsx, input.tsx, select.tsx, slider.tsx, stat.tsx, switch.tsx, tabs.tsx, textarea.tsx); add a section naming HeroUI as the new component layer; note the new Share links section in Settings and public Share route.
- [ ] T203 Update docs/architecture.md to replace any mention of the old lunaris-based primitives with HeroUI as the component foundation; list the component families and their roles (HeroUI base + Gameplane compositions in @/components/ui/).
- [ ] T204 Update docs/contributing.md to replace component-building guidance from lunaris/Radix to HeroUI; direct contributors to @heroui/react docs and the Gameplane compositions in web/src/components/ui/.

### Verification: Consumers and Coverage

- [ ] T205 Run grep -rE '@/components/ui/|@radix-ui' web/src --include='*.tsx' against the codebase; confirm zero results to verify SC-002 (no consumers of old primitives remain).
- [ ] T206 Run git diff master --stat -- 'web/src/**/*.test.ts*' and verify no test file was deleted; list counts of it() cases in each modified test file and confirm counts did not decrease (SC-005).
- [ ] T207 Manually navigate the login page, servers list, and server settings using only keyboard (Tab, Enter, arrow keys, Escape) to verify no focus traps, all controls are labeled, and every interactive element is reachable without a mouse (SC-007 keyboard-only pass).

### Quickstart and Housekeeping

- [ ] T208 Run the quickstart.md validation steps (section 1–4): verify design commit precedes code commits, check JSON exports have no '...' elision markers, run tsc --noEmit, confirm web/e2e-mock and e2e-web-live CI jobs pass. (OD-4's `dashboard-utility` (bJ2cg) / OFfAu deletion and OD-5's `.gitignore` entry were both settled and applied in slice 0 — T004, T005 — so no further action on either is needed here.)

### PR Labeling

- [ ] T209 Add labels to the slice 5 PR via gh api: type: refactor, area: web, and type: feature (only slice 5 carries type: feature); use the REST API per CLAUDE.md rule 14 since gh pr edit does not work on this repo.

**Checkpoint**: Feature complete: zero consumers of the old primitives, all gates green, spec folder ready for the `done_` rename (CLAUDE.md rule 16)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies; starts after the maintainer opens `design.pen` in the Pencil GUI (plan.md Risks, R-05)
- **Foundational (Phase 2)**: depends on Setup; same branch `014a-foundation`; BLOCKS every user story
- **User stories (Phases 3-7)**: strictly sequential in priority order because each slice branch is cut from `master` after the previous slice merges (plan.md "Delivery slices"): US1 → US2 (2a then 2b) → US3 → US4 → US5. Stories are NOT run in parallel on this feature; parallelism is inside a slice.
- **Polish (Phase 8)**: same branch as US5; the old primitives are deleted only after every screen is rebuilt (FR-008)

### User Story Dependencies

- **US1 (P1)**: needs Foundational; provides the shell every later screen renders inside
- **US2 (P1)**: needs US1 (shell, PageHeader, ConfirmDialog); slice 2b needs slice 2a merged (shared ServerDetail tab bar)
- **US3 (P2)**: needs US1; independent of US2 except the shared hero atoms from slice 0
- **US4 (P3)**: needs US1; independent of US2/US3
- **US5 (P3)**: new construction on the finished component layer; needs US2 (Settings tab host for the Share links section) and is the only slice allowed to delete `components/ui/`

### Within Each Slice

- Design wave (Pencil MCP) → maintainer saves in the GUI → export + MANIFEST → design commit → code wave → tests updated in the same task → `tsc --noEmit` → push → CI (`web`, `web-e2e-mock`, `e2e-web-live`) → screenshot comparison per `contracts/screen-verification.md` → review → PR labels (`type: refactor`, `area: web`; slice 5 also `type: feature`)
- Per CLAUDE.md rule 13 each wave is a Workflow: haiku design agents (one per 4-6 screens), haiku code agents (one per file, worktree isolation), one sonnet review, then a haiku fix wave in a second Workflow

### Parallel Opportunities

- Setup: dependency, export and scaffold tasks marked [P] run together
- Foundational: design tasks per node group run in parallel through the MCP; hero atoms are one file each and run in parallel with worktree isolation
- Every story: design tasks per node group in parallel; code tasks per route/tab/component file in parallel (worktree isolation); Playwright and screenshot tasks after the code wave
- Never parallel: anything touching the same file; export before code; deletion of `components/ui/` after every consumer is gone

---

## Parallel Example: US1

```bash
Task: "T042: Rebuild web/src/components/AppLayout.tsx to compose hero/AppShell and hero/TopBar, implementing theme toggle via HeroUI useTheme hook syncing to localStorage and index.html data-theme attribute per OD-2 (Settled 2026-09-03: sidebar profile footer next to logout, mirrored in the mobile drawer); update AppLayout.test.tsx with new component structure; ensure tsc --noEmit passes."
Task: "T043: Rebuild web/src/components/PageHeader.tsx from exported composition using HeroUI Breadcrumbs, Heading, and layout components; update PageHeader.test.tsx; ensure tsc --noEmit passes."
Task: "T044: Rebuild web/src/components/ClusterSelector.tsx (composition aI9PL) using HeroUI Select and Button components, preserving multi-cluster context switching; update ClusterSelector.test.tsx; ensure tsc --noEmit passes."
Task: "T045: Create web/src/components/hero/AppShell.tsx and AppShell.test.tsx composing hero/Sidebar, hero/TopBar, and Breadcrumbs with a main content slot; export from @heroui/react and @/components/hero; ensure tsc --noEmit passes."
```

## Parallel Example: US2

```bash
Task: "T066: Rebuild web/src/routes/Servers.tsx to render the server table, phase chips, filter popover and per-row actions from @heroui/react (Table, Chip, Popover, Dropdown/Menu) and @/components/hero/ (PhaseChip, FilterPopover), update web/src/routes/Servers.test.tsx to the new markup keeping all it() cases, and verify tsc --noEmit"
Task: "T067: Rebuild web/src/routes/ServerDetail.tsx to render the server detail shell, tab bar and breadcrumbs from HeroUI and hero/ compositions, update ServerDetail.test.tsx keeping all it() cases, and verify tsc --noEmit"
Task: "T068: Rebuild web/src/routes/tabs/Overview.tsx to render the overview card layout (status chips, resource cards, lifecycle state alerts) from @heroui/react Card/Chip/Alert and @/components/hero/ (StatCard, PhaseChip) per the designed layout for each server state (running, idle armed, asleep, never sleeps, PVC provisioning failed), update Overview.test.tsx keeping all it() cases, and verify tsc --noEmit"
Task: "T069: Rebuild web/src/routes/tabs/Events.tsx to render the events table and filters from @heroui/react (Table, Input, Select) and @/components/hero/, update Events.test.tsx keeping all it() cases, and verify tsc --noEmit"
```

## Parallel Example: US5

```bash
Task: "T172: Add ShareLink type (id, label, createdAt, expiresAt, allowStart, status, token) and ShareLinkPublic response type to web/src/types.ts mirroring api/internal/handlers/shares.go response shapes; add ShareLinkCreateRequest for the create endpoint payload."
Task: "T173: Add Shares namespace to web/src/lib/endpoints.ts with create(server, cluster?), list(server, cluster?), revoke(server, id, cluster?), resolve(token), start(token) endpoint paths matching the API handler routes."
Task: "T174: Add Shares namespace to web/src/lib/api.ts with create/list/revoke functions (authenticated, use mutation helpers) and resolve/start functions (public, use query/mutation with rate-limit error mapping to neutral state); write web/src/lib/api.test.ts cases for all Shares functions including error states per FR-005."
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1 + Phase 2 (slice 0, branch `014a-foundation`) → PR → maintainer merge
2. Complete Phase 3 (slice 1, branch `014b-shell-login`) → PR → maintainer merge
3. **STOP and VALIDATE**: login states, sidebar navigation, mobile drawer, screenshot comparison; old page bodies render inside the new shell

### Incremental Delivery

1. Slice 0 → foundation merged
2. Slice 1 (US1) → MVP: new shell around old bodies
3. Slice 2a then 2b (US2) → servers and every tab rebuilt
4. Slice 3 (US3) → create wizard, modules, backups
5. Slice 4 (US4) → admin surfaces
6. Slice 5 (US5 + Polish) → share links built new, old primitives deleted, docs and `web/specs.md` final

### Team / Agent Strategy

- Slices are sequential; inside a slice the design wave, code wave and verification are Workflows of haiku agents reviewed at sonnet (plan.md "Per-slice execution").
- A `fable` agent is never launched without explicit maintainer authorization.

---

## Notes

- [P] tasks = different files, no dependencies
- Commit per logical unit (rule 11): the design commit precedes the code commits in every slice
- No local test/lint runs (rule 8); `tsc --noEmit` only
- A test edit is justified only by a markup change in the same slice; test files never decrease (SC-005)
- A rebuilt file importing `@/components/ui/` fails review (FR-012)
- All seven open decisions (OD-1..OD-7) were settled by the maintainer on 2026-09-03; tasks that depend on one cite it as "OD-n (Settled 2026-09-03)" with the ruled value
