# Feature Specification: Rebuild the dashboard on the HeroUI design system

**Feature Branch**: `014-heroui-web-rebuild`

**Created**: 2026-09-02

**Status**: Draft

**Input**: User description: "rebuild the web interface using the heroUI design component everything should already be imported inside the design.pen file but heroUI template.pen exist in case you are missing something"

## Context

The Gameplane dashboard's designed screens (79 `Screen/…` frames in `design.pen`) and its 57 `Gameplane/…` reusable components are currently built on the older "lunaris" primitive set (the `c:…` components inside the `lunaris: design system components` frame). The shipped web dashboard mirrors that: a hand-rolled set of primitives (button, card, input, select, tabs, switch, dialog, dropdown, …) styled to match those lunaris designs.

A complete HeroUI component library has since been imported into `design.pen` as the top-level frame `HeroUI: Design System Components` (`LtgNm`, 236 children — roughly 200 named component definitions covering Accordion, Alert, Avatar, Button, Card, Checkbox, Chip, Dropdown/Menu, Input, InputOTP, Link, Modal, AlertDialog, NumberField, Pagination, Radio, SearchField, Select, Slider, Spinner, Switch, Table, Tabs, Textarea, Tooltip). A sample HeroUI dashboard layout (`dashboard-utility`, `bJ2cg`) was imported alongside it as a layout reference. The standalone `heroUI template.pen` file at the repo root is the original import source and is the fallback if any HeroUI piece turns out to be missing from `design.pen`.

This feature rebuilds the dashboard — design first, then code — so that every screen is composed from the HeroUI component set rather than the lunaris primitives, and the shipped web application uses the HeroUI component family end to end. Everything the dashboard *does* today keeps working; what changes is what it is *built from* and how it looks.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Sign in and navigate the shell (Priority: P1)

An operator opens the dashboard, signs in (local password or SSO), and lands on the home page. The application shell — sidebar navigation, top bar with cluster selector, search, notifications and user menu, page headers — is rebuilt from HeroUI components. Every existing destination in the sidebar is still reachable and the current role gating is unchanged.

**Why this priority**: the shell and login are on the path of every other screen; nothing else can be rebuilt or verified until the frame it sits in exists. Delivering only this story already gives a testable product: the old page bodies render inside the new HeroUI shell.

**Independent Test**: sign in with a local admin account and with the SSO-only login variant, confirm the three login states (default, invalid credentials, SSO only) and the app-loading state render from HeroUI components, then click every sidebar entry and confirm each page loads with the new shell. Can be fully tested by a viewer, an operator and an admin session and delivers the new frame for every page.

**Acceptance Scenarios**:

1. **Given** a signed-out browser, **When** the user opens the dashboard, **Then** the login page renders from the HeroUI form components and shows nothing but brand, form and neutral error copy (the pre-auth privacy rule is preserved).
2. **Given** wrong credentials, **When** the user submits, **Then** the "invalid credentials" state renders as a HeroUI alert without distinguishing wrong password from unknown user.
3. **Given** a signed-in admin, **When** they open each sidebar destination (Dashboard, Servers, Modules, Backups, Cluster, Users, Audit log, System logs, Admin settings), **Then** every page loads inside the rebuilt sidebar, top bar and page header, and the active item is highlighted.
4. **Given** a signed-in viewer, **When** they open the sidebar, **Then** entries gated by permissions they lack are hidden exactly as they are today.
5. **Given** a phone-width viewport, **When** the user opens the servers list, **Then** the mobile servers screen and the mobile navigation drawer render from HeroUI components per their design frames.

---

### User Story 2 - Manage servers day to day (Priority: P1)

An operator lists servers, opens one, reads its status, uses the lifecycle actions, and works through every Server Detail tab (Overview, Events, Console, Logs, Files, Mods, Modpacks, Players, Backups, Capture, Settings) and every Settings sub-section. All of these are rebuilt from HeroUI tables, cards, chips, tabs, modals and form controls.

**Why this priority**: server operations are the reason the product exists and carry the largest share of the designed screens (Overview variants including idle/asleep/never-sleeps/PVC-failure, Logs (Failed), Mods browse/install/upload/by-ID, Capture states, eleven Settings sub-pages, share-link settings). Rebuilding these is the bulk of the visible value.

**Independent Test**: with at least one running and one stopped server on the test cluster, walk the Servers list (filters, status chips, actions menu), open a server, click every tab, open every Settings sub-section, exercise one confirm-dialog flow (for example Wipe world), and confirm each surface matches its design frame and its existing behaviour.

**Acceptance Scenarios**:

1. **Given** the Servers page, **When** it loads, **Then** the server table, phase chips, filter popover and per-row actions render from HeroUI components and filtering by phase/template/namespace behaves as before.
2. **Given** a server in each designed Overview state (running, idle armed, asleep, never sleeps, PVC provisioning failed), **When** the Overview tab opens, **Then** the corresponding designed layout is shown using HeroUI cards, chips and alerts.
3. **Given** the Console and Logs tabs, **When** they open, **Then** the terminal and log stream still work and only their surrounding chrome (tab bar, toolbar, status chips, failed-state alert) is rebuilt.
4. **Given** each Settings sub-section (General, Version, Resources, Networking, Environment, Lifecycle, Scheduled backups, Network capture, Placement, RBAC & access, Share links, Danger zone), **When** a value is changed and saved, **Then** the form controls are HeroUI inputs/selects/switches and the save, discard, validation-error and conflict behaviours are unchanged.
5. **Given** any destructive action (delete, wipe, transfer, revoke share link), **When** it is triggered, **Then** a HeroUI alert-dialog styled per the `AlertDialog/Danger` design asks for confirmation before anything happens.

---

### User Story 3 - Create a server and manage modules and backups (Priority: P2)

An operator runs the five-step Create Server wizard, browses and installs modules from the catalog, and works the Backups pages (Index, Schedules, Restores) including the restore dialog and the backup detail drawer.

**Why this priority**: these are the second most-used flows and are self-contained enough to ship after the shell and server surfaces without blocking them.

**Independent Test**: create a server through all five wizard steps (including the address-pool fields on the Network step), install one module from the catalog and upload one, add a module source, create a schedule and open one backup's detail drawer and restore dialog. Each screen must match its design frame and complete its flow.

**Acceptance Scenarios**:

1. **Given** the Create Server wizard, **When** the user moves through Template, Version, Configure, Network and Review, **Then** each step is rebuilt from HeroUI form controls and step-level validation blocks progress exactly as today.
2. **Given** the Modules catalog, **When** the user installs, uploads or adds a source, **Then** the three module dialogs render as HeroUI modals and complete their flows.
3. **Given** the Backups Index, Schedules and Restores sub-pages, **When** they load, **Then** their tables, filters, detail drawer and restore dialog are HeroUI components and the manual backup, schedule and restore actions still work.

---

### User Story 4 - Administer the installation (Priority: P3)

An admin manages users and RBAC (Users, Roles, Service accounts, Identity providers), reviews the Audit log and System logs, and edits Admin settings (Authentication with its OIDC/role-mapping states, Backup destinations, Module sources, Mod registries, Notifications, Telemetry, Updates, About) and Cluster settings.

**Why this priority**: admin-only surfaces are visited least often; they can follow once the shared shell and form components are proven on the P1/P2 screens.

**Independent Test**: as an admin, open every admin route and sub-page, invite a user, edit a role, add and remove an OIDC role-mapping override (hitting the admin-mapping confirm dialog and the save-rejected state), verify the audit chain, and download system logs.

**Acceptance Scenarios**:

1. **Given** the Users & RBAC pages, **When** the invite, edit, reset-password and role-editor dialogs open, **Then** they are HeroUI modals and their flows complete.
2. **Given** Admin Settings — Authentication in each designed state (mappings present, no mappings, admin-mapping warning, save rejected), **When** it loads, **Then** provenance badges, removable group chips, alerts and the confirm dialog render from HeroUI components with their existing copy.
3. **Given** the Audit log, **When** the integrity banner and paginated table load, **Then** pagination and the integrity banner use HeroUI components and export/verify actions still work.

---

### User Story 5 - Use a public share link (Priority: P3)

A player who received a share link opens it without an account and sees the server's state (up, asleep and startable, asleep view-only, starting, invalid/expired), rebuilt from HeroUI components while keeping the page free of any internal detail beyond what the link is meant to expose.

**Why this priority**: a small, isolated, unauthenticated surface; low traffic but must not be left on the old primitives once everything else has moved.

**Independent Test**: create a share link for a server, open it signed out in each of the five states, and confirm each matches its design frame.

**Acceptance Scenarios**:

1. **Given** a valid share link for a running server, **When** it opens, **Then** the "Up" layout renders from HeroUI components.
2. **Given** an expired or revoked link, **When** it opens, **Then** the "Invalid or expired" layout renders and discloses nothing about the server.

---

### Edge Cases

- A screen state that exists in the current code but has no dedicated design frame (for example a loading skeleton or an empty list) must be composed from the HeroUI `Spinner`, `Card` and `Alert` definitions following the nearest designed sibling — it must not be silently dropped.
- A HeroUI component that the design needs but that is missing from the `LtgNm` frame is imported from `heroUI template.pen` and added to the same frame before it is used; the rebuilt screen must not fall back to a lunaris `c:…` primitive.
- The Console and Logs tabs embed a terminal and a virtualised log view that are not design-system components. They keep their engines; only their chrome is rebuilt.
- Dark and light appearance: the current dashboard defaults to dark. The rebuild must render correctly in both appearances with the same brand tokens (orange primary, current surface/border/muted scale) applied to the HeroUI palette.
- Very long content (server names, file paths, audit payloads, log lines) must truncate or wrap inside HeroUI table cells and cards without breaking the layout.
- Keyboard and screen-reader use: every rebuilt control must remain reachable and operable by keyboard, with labels, roles and focus order at least as good as today.
- Existing deep links (`/servers/$name?ns=…&tab=…`, `/servers/new?template=…`, `/backups` sub-tabs, `/admin/*`) must resolve to the same content after the rebuild.
- Existing automated tests that assert on the old markup will break because the underlying components change. Those tests are updated in the same change that rebuilds the screen they cover, never deleted, and coverage does not drop below the current gates.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Every designed dashboard screen (the 79 `Screen/…` frames in `design.pen`, including all state variants) MUST be redesigned in `design.pen` using components from the `HeroUI: Design System Components` frame before its code is rebuilt, and the redesigned frame MUST be re-exported to `design-export/` in the same change (Constitution Principle II).
- **FR-002**: Every `Gameplane/…` reusable component in `design.pen` (57 today: sidebar, top bar, page header, stat card, server detail header/tabs, dropdown menu, confirm dialog, modal, filter popover, buttons, inputs, select, switch, card, notifications panel, search results, backup detail drawer, role editor, error/loading cards, all `Dialog/…` definitions, capture warning banner, removable group chips, provenance badges) MUST be rebuilt as a composition of HeroUI components, and its instances across all screens MUST update accordingly.
- **FR-003**: The shipped dashboard MUST render every page, tab, sub-section, dialog and drawer from the HeroUI component family; no screen may ship on the old primitive set once its story is delivered.
- **FR-004**: The rebuild MUST preserve all existing user-visible behaviour: every route, query parameter, permission gate, form validation, confirm step, realtime stream (console, logs, events), and multi-cluster context behaves exactly as it does today.
- **FR-005**: The login page and every other unauthenticated screen (share link states, app loading) MUST continue to display no internal metrics, hostnames, cluster names, version strings or user-enumeration hints (CLAUDE.md rule 3).
- **FR-006**: The rebuilt interface MUST apply the Gameplane brand tokens (orange primary, existing surface/border/muted/success/warning/danger/violet scale, Geist and JetBrains Mono type) onto the HeroUI theme in both dark and light appearance, dark remaining the default.
- **FR-007**: Any HeroUI component the design needs that is absent from `design.pen` MUST be imported from `heroUI template.pen` into the `HeroUI: Design System Components` frame, not recreated by hand and not substituted with a lunaris primitive.
- **FR-008**: The old lunaris-based primitives (both the `c:…` component definitions' use on Gameplane screens and the hand-rolled primitives in the web code) MUST have no remaining consumers when the feature completes; their removal is the last step of the feature, not the first.
- **FR-009**: Each rebuilt screen MUST be verified against its updated design frame (screenshot comparison at the design's reference width, 1440px for desktop screens and the mobile frames' width for mobile screens) before its story is accepted.
- **FR-010**: Existing automated tests for a rebuilt screen MUST be updated to the new markup in the same change; no test may be deleted, and the web coverage gates (lines 92%, functions 76%, branches 82%, statements 92%) MUST hold after every story.
- **FR-011**: The end-to-end suite MUST exercise the rebuilt dashboard through at least the P1 flows (login, sidebar navigation, server list, server detail tabs, one settings save, one confirm dialog) on the real cluster (Constitution Principle I).
- **FR-012**: The rebuild MUST be delivered as independently mergeable slices ordered by story priority, each leaving the dashboard fully usable, so the old and new component sets may coexist during the transition but never on the same screen.
- **FR-013**: The `web/specs.md` module specification MUST be updated in each slice to describe the new component layer and, at completion, to remove every reference to the old primitive set.

### Key Entities

- **Design screen**: a top-level `Screen/…` frame in `design.pen`; has an id, a name, a state variant, a reference width, and a plain-file export (JSON + PNG) in `design-export/`. 79 exist today; the rebuild changes their contents, not their count, except where a missing state is added.
- **Design component**: a reusable definition in `design.pen`. Three families exist: `Gameplane/…` (product-specific compositions, rebuilt by this feature), HeroUI definitions (the new base set, consumed by this feature), and lunaris `c:…` primitives (the old base set, retired by this feature).
- **Dashboard page / tab / dialog**: a shipped web surface corresponding to one or more design screens; has a route, a permission gate, and a set of automated tests.
- **Theme tokens**: the named colours, radii and fonts shared between the design file and the shipped dashboard.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of the designed dashboard screens (79 frames) and 100% of the `Gameplane/…` components (57) are rebuilt on HeroUI in `design.pen`, with a matching refreshed export for each.
- **SC-002**: 100% of shipped dashboard pages, tabs, settings sub-sections, dialogs and drawers render from HeroUI components; a search of the shipped code for the old primitive set returns zero consumers at completion.
- **SC-003**: Every existing user flow listed in the acceptance scenarios completes successfully on the test cluster after the rebuild, with no regression reported in the P1 and P2 flows during the review of each slice.
- **SC-004**: Each rebuilt screen matches its design frame at the reference width closely enough that a reviewer comparing the screenshot pair finds no layout, colour or component-family mismatch.
- **SC-005**: Dashboard automated-test coverage stays at or above today's gates after every slice, and the number of test files does not decrease.
- **SC-006**: The pre-auth privacy check on the login and share-link pages passes unchanged.
- **SC-007**: Keyboard-only navigation of the login, servers list and server detail settings completes without a trapped focus or an unlabeled control.
- **SC-008**: Each slice is mergeable on its own: at no point between the first and last slice does any screen show a mix of old and new component families.

## Assumptions

- "HeroUI" refers to the component library already imported into `design.pen` as the `HeroUI: Design System Components` frame (`LtgNm`) and, for the code, to the published HeroUI React component family that those designs represent; the dashboard adopts that family as its component layer.
- The `dashboard-utility` frame (`bJ2cg`) and the `OFfAu` prompt node are HeroUI template samples, not Gameplane screens; they serve as a layout reference and are not part of the screen count.
- The information architecture stays as it is: same sidebar destinations, same tabs, same settings sub-sections, same wizard steps. This feature changes the component family and visual treatment, not what the product offers. (See the open question below — if a broader redesign is wanted, the screen inventory grows.)
- Dark appearance stays the default; light appearance is supported with the same tokens.
- The two Pencil documents at `design.pen` and `/home/valgul/project/kubernetes-game-dashboard/design.pen` are byte-identical today; the repo copy is the one edited and exported.
- `heroUI template.pen` stays at the repo root as the import source. Whether it is committed is the maintainer's decision; this spec does not require it in git.
- The Console terminal and the virtualised log view keep their current engines; they are outside the component-family change.
- The existing REST/WebSocket API is unchanged; no backend, operator or agent work is required.
- Delivery is sliced by user-story priority; each slice is its own branch and pull request per CLAUDE.md rules 11 and 12.
