# Implementation Plan: Rebuild the dashboard on the HeroUI design system

**Branch**: `014-heroui-web-rebuild` | **Date**: 2026-09-02 | **Spec**: ./spec.md

**Input**: Feature specification from `specs/014-heroui-web-rebuild/spec.md`

## Summary

Replace the dashboard's component layer — 19 hand-rolled primitives plus six Radix packages, drawn in Pencil on the lunaris `c:` primitives — with HeroUI v3, design first and code second, without changing what any screen does. Every one of the 79 designed screens and 57 `Gameplane/…` compositions is redrawn from the HeroUI definitions already imported into `design.pen` (frame `LtgNm`), re-exported, and then translated to React using `@heroui/react` 3.2.4 with the Gameplane brand written into HeroUI's token set (orange accent, dark default, light supported). Delivery is six priority-ordered slices on their own branches; old and new component families coexist across screens during the transition but never on one screen, and the old primitives are deleted in the final slice. Seven designed-but-unbuilt share-link surfaces are built new in the last slice (maintainer decision). Every slice is implemented by haiku agents in Workflows with a sonnet review, and verified on CI: Vitest with the existing coverage gates, Playwright mock and live, and a per-screen screenshot comparison against the design export.

## Technical Context

**Language/Version**: TypeScript 6.0.3 strict, React 19.2.8 (unchanged; TypeScript 7 remains blocked upstream, see `CLAUDE.md`)

**Primary Dependencies**: `@heroui/react` 3.2.4, `@heroui/styles` 3.2.4, peers `react-aria ^3.51.0`, `react-aria-components ^1.20.0`, `@react-aria/i18n ^3.13.1`, `@react-aria/ssr ^3.10.1`, `@react-aria/utils ^3.34.1`; Tailwind CSS 4.3.3 (kept); TanStack Router / Query, lucide-react, Monaco, xterm (kept). Removed at the end: `@radix-ui/react-dialog`, `react-dropdown-menu`, `react-label`, `react-slot`, `react-tabs`, `react-toast`, `class-variance-authority`.

**Storage**: N/A (no backend change; `localStorage` key for the appearance preference via HeroUI `useTheme`)

**Testing**: Vitest 4.1.11 + Testing Library + MSW (unit, coverage-gated), Playwright 1.62 mock and live (`web/e2e/`), per-screen screenshot comparison (new, `contracts/screen-verification.md`); all on GitHub Actions

**Target Platform**: evergreen desktop browsers at 1440 px reference width and mobile at 390 px; served by the API image from `web/dist`

**Project Type**: web application front end (`web/` npm package) plus Pencil design source (`design.pen`)

**Performance Goals**: no regression in first render of the servers list or server detail

**Constraints**: FR-004 behaviour preservation; FR-005 pre-auth privacy; FR-010 no test deleted and coverage gates hold after every slice; FR-012 no screen mixes families; Constitution II design-first with same-change export; rule 8 no local test runs; rule 13 delegation via Workflows

**Scale/Scope**: 79 screens, 57 compositions, 12 routes, 11 server-detail tabs, 11 settings sections + 1 new, 104 test files (~1,600 cases), 21 Playwright specs, 272 consumer files of the old primitives

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | How this plan satisfies it |
|---|---|---|
| **I. E2E-Tested Delivery** | PASS, OD-3 settled 2026-09-03 | Each slice extends the Playwright live suite (`web/e2e/specs/live/`) for its flows, and the `e2e-web-live` CI job runs them on a kind cluster; FR-011 names the P1 flows. The Go `test/e2e/` suite is API-only and unaffected. The dashboard's E2E tier is `web/e2e/` (live specs); constitution amendment tracked separately. No game-module join coverage is touched. |
| **II. Design-First** | PASS | Every slice starts with a Pencil pass on the repo `design.pen` through the MCP server, redrawing that slice's screens and compositions from the `LtgNm` definitions, followed in the same commit by JSON + PNG re-export and a `MANIFEST.md` update. Slice 0 also clears the pre-existing export debt (the HeroUI frame was imported but never exported). `.pen` files are never read or edited with generic tools. |
| **III. Language & Ecosystem** | PASS | Strict TypeScript, no `any` without a comment, no suppressions; ESLint and coverage gates unchanged; no CRD or Go change, so no codegen. Old primitives are deleted, not silenced. |
| **IV. Spec-Driven** | PASS | Spec, this plan, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`, `OPEN-DECISIONS.md` precede code; `web/specs.md` is updated in each slice (FR-013). |
| **V. Delegate to Workflows** | PASS | Research ran as one Workflow (five haiku, one sonnet review). Each slice runs as Workflows: a design wave (haiku agents drive the Pencil MCP per screen group), a code wave (haiku per route/tab file), a sonnet review, then a haiku fix wave. `model` is set on every `agent()` call. No `fable` agent without explicit authorization. |
| **VI. CI Bears the Heavy Lifting** | PASS | No local test or lint runs; `tsc --noEmit` compile checks only. Each slice PR is verified by the `web`, `web-e2e-mock` and `e2e-web-live` jobs; nothing is reported done before CI is green. |

Post-design re-check (after Phase 1): no violation introduced. OD-3 is settled (web/e2e/ live specs are the E2E tier; constitution amendment tracked separately).

## Project Structure

### Documentation (this feature)

```text
specs/014-heroui-web-rebuild/
├── spec.md                  # what and why; Clarifications section holds the maintainer's rulings
├── plan.md                  # this file
├── research.md              # Phase 0: verified facts R-01…R-08
├── data-model.md            # Phase 1: design screen, design component, theme token, dashboard surface, slice
├── quickstart.md            # Phase 1: how to validate a slice end to end
├── OPEN-DECISIONS.md        # OD-1…OD-7, all settled 2026-09-03
├── contracts/
│   ├── component-map.md     # lunaris/Radix → HeroUI mapping; Gameplane compositions and their node ids
│   ├── theme-tokens.md      # brand values → HeroUI tokens, both modes; legacy alias policy
│   ├── screen-verification.md  # per-screen acceptance procedure (export vs Playwright screenshot)
│   └── share-link-ui.md     # the new share-link surfaces: API consumed, states, privacy rules
├── checklists/requirements.md
└── tasks.md                 # Phase 2 (/speckit-tasks), not created here
```

### Source Code (repository root)

```text
design.pen                                  # EDIT (Pencil MCP only): redraw 79 screens + 57 compositions on LtgNm definitions;
                                            #   set HeroUI variables to Gameplane brand in light and dark semantic modes
design-export/
├── MANIFEST.md                             # UPDATE per slice; slice 0 adds the HeroUI frame section
├── json/<id>.json                          # RE-EXPORT touched screens/components; slice 0 ADDS LtgNm + 192 HeroUI definitions
└── screenshots/<id>.png                    # same

web/
├── package.json                            # slice 0: ADD @heroui/react, @heroui/styles, react-aria peers; slice 5: REMOVE radix, cva
├── index.html                              # keep class="dark"; slice 1 adds data-theme sync via useTheme
├── tailwind.config.ts                      # slice 5: drop if no longer needed (Tailwind 4 is CSS-first)
├── src/styles/globals.css                  # slice 0: @import "@heroui/styles"; brand → HeroUI tokens; legacy --gp-* aliases
├── src/components/
│   ├── hero/                               # NEW (OD-7): rebuilt Gameplane compositions — AppShell, Sidebar, TopBar, Breadcrumbs,
│   │                                       #   PageHeader, StatCard, PhaseChip, ConfirmDialog, FilterPopover, LoadingCard, ErrorCard,
│   │                                       #   ErrorBanner, ProvenanceBadge, RemovableGroupChip, CaptureWarningBanner, Meter, Sparkline, GameIcon
│   ├── ui/                                 # OLD primitives; untouched until slice 5, then DELETED
│   ├── AppLayout.tsx                       # slice 1: REWRITE on hero/AppShell
│   ├── PageHeader.tsx, ClusterSelector.tsx # slice 1: REWRITE
│   ├── CaptureWidget.tsx                   # slice 2b: REWRITE chrome
│   ├── server/*, backups/*, modules/*      # slices 2a/2b/3: REWRITE dialogs and cards
│   └── registry-browser.tsx               # slice 2b
├── src/routes/
│   ├── Login.tsx, Dashboard.tsx            # slice 1
│   ├── Servers.tsx, ServerDetail.tsx       # slice 2a
│   ├── tabs/{Overview,Events,Console,Logs,Files,Players}.tsx        # slice 2a
│   ├── tabs/{Mods,Modpacks,Backups,Settings}.tsx, tabs/settings/*  # slice 2b
│   ├── tabs/settings/ShareLinks.tsx        # slice 5: NEW section
│   ├── CreateServer.tsx, Modules.tsx, Backups.tsx                  # slice 3
│   ├── AdminSettings.tsx, Users.tsx, AuditLog.tsx, AdminLogs.tsx, Cluster.tsx  # slice 4
│   └── Share.tsx                           # slice 5: NEW public route /share/$token
├── src/router/tree.tsx                     # slice 5: ADD public share route
├── src/lib/endpoints.ts, api.ts            # slice 5: ADD Shares namespace (create/list/revoke, public resolve/start)
├── src/types.ts                            # slice 5: ADD ShareLink types
├── src/test/setup.ts                       # unchanged (mocks already present, R-07)
├── src/**/*.test.tsx                       # UPDATE in the same slice as the screen; never deleted
├── e2e/specs/*.spec.ts, e2e/specs/live/*   # UPDATE selectors per slice; slice 5 ADDS share-link specs
├── e2e/screenshots/                        # NEW: per-screen Playwright screenshot spec used by screen-verification.md
└── specs.md                                # UPDATE per slice; final slice removes every mention of the old primitives

docs/
└── architecture.md, contributing.md        # slice 5: mention HeroUI as the component layer where the old primitives were named
```

**Structure Decision**: the feature lives entirely in `web/` and `design.pen`/`design-export/`. The new compositions go in `web/src/components/hero/` so that rebuilt screens never import from `components/ui/`, which is what makes FR-012's "never both families on one screen" mechanically checkable: a rebuilt file importing `@/components/ui/` fails review. Slice 5 deletes `components/ui/` and then, per OD-7 (Settled 2026-09-03), renames `hero/` to `ui/`.

## Delivery slices

Each slice is one branch, one PR, labelled `type: refactor` + `area: web` (slice 5 also `type: feature`). Each slice's branch is cut from `master` after the previous slice merges; the design commit precedes the code commits.

| Slice | Branch suffix | Design objects (node ids) | Code surfaces | Story |
|---|---|---|---|---|
| **0 Foundation** | `014a-foundation` | Set HeroUI variables to brand (light + dark); export `LtgNm` and its 192 definitions; redraw the shared atoms `tpKRk rNhll LMIom XoX7L z9ShNE d5N3W3 J09iP IU7OG D0cDM qvQPg Lmaf1 AT7ya hl7R3 rh2QH k38Uta ZWcwn xCDF7 x3beP WwNlX BPEpm FyV6E w4ntSc zzx8f igj2U` | deps, `globals.css` token mapping, `components/hero/` atoms with tests, `web/specs.md` section, no screen switched yet | — |
| **1 Shell + login** | `014b-shell-login` | `N1GkB jmoi3 ljdA5 N13Xud j24cXg tooKB SeizD kKFX9 gu5WY aI9PL hboVw IdaU7` + new appearance-toggle frame in the sidebar profile footer and mobile drawer (OD-2, Settled 2026-09-03) | `AppLayout.tsx`, `Login.tsx`, `Dashboard.tsx`, `PageHeader.tsx`, `ClusterSelector.tsx`, `hero/{AppShell,Sidebar,TopBar,Breadcrumbs,NotificationsPanel,GlobalSearch}`, theme toggle | US1 |
| **2a Servers + core tabs** | `014c-servers-core` | `iGBIs EZFW0 mQ1zB IzuY2 TE2jI o4LH8W P08Uw Xn5ns kPmoo FtdkI Burtr dPP50 S4k0x I9kvlZ I9W8z JLaGB` | `Servers.tsx`, `ServerDetail.tsx`, tabs Overview/Events/Console/Logs/Files/Players, `components/server/*` cards + dialogs `T1LzpU rdlrx t9irnv` | US2 |
| **2b Mods, backups tab, capture, settings** | `014d-server-settings` | `sZtDi GayoL KhYNc Ss0Yr V1VhGE tY6RD pssCT Bbnga dBILX xvlB6 m5kOm4 O08uaD b4eaUf hLB9Z swxkJ E0ypH J5pjJ3 ugDSa i1bLR KaRFX RodrS Y5cmvI VfB0Y i8wib f0s9zG KrREo BX0XM` | tabs Mods/Modpacks/Backups/Settings, all 11 `tabs/settings/*`, `CaptureWidget.tsx`, `registry-browser.tsx`, `components/modules/{Install,UploadModule}Dialog` | US2 |
| **3 Create, modules, backups** | `014e-create-modules-backups` | `W8idqY nNL3E vUqMl f1Vga UMJli kK8Ji DPrYX fK8Bi tTSdi zhLZN E9EEv0 DMnEi` | `CreateServer.tsx`, `Modules.tsx`, `Backups.tsx`, `components/backups/*`, `components/modules/{ModuleCard,ModuleSourcesPanel,SourceDialog}` | US3 |
| **4 Admin** | `014f-admin` | `WZdnw uMiwd nNGDX QgW58 zqzr4 RC3Kf g5mEpx Wj0V4 n6Xlo uoxQW M2sA4u zM0VF bYDHC e9lV4 TBvTC Dpb9f DxKOh Bq2Yg j9W8A dxdEi kIxaJ CqaSq NLDDv t3IY3u MaoHP Kp48V uw0dB XL5ZU vStkb R65Xyx Rwnu3 BV5ei` | `AdminSettings.tsx`, `Users.tsx`, `AuditLog.tsx`, `AdminLogs.tsx`, `Cluster.tsx` | US4 |
| **5 Share links + retirement** | `014g-share-links-retire` | `xCJlu dQV9N C2LQE4 q31B6w qFLfB EcoGD epZO2 atqRh VM7ro S7SCDc` | NEW `tabs/settings/ShareLinks.tsx`, `routes/Share.tsx`, `lib` Shares namespace, types, router; DELETE `components/ui/`, Radix, cva; final `web/specs.md`, docs | US5, FR-008 |

Ordering rationale: slice 0 makes every later slice a pure translation job; 1 gives every screen its frame; 2a/2b split the largest story so each PR stays reviewable; 5 carries the new build because it is also the only slice allowed to delete the old layer (FR-008 "last step").

### Theme (added 2026-09-04)

Settled by the maintainer 2026-09-04: the slice 0 design pass establishes the brand re-theme across `design.pen`. Light mode palette: white page (#FFFFFF), extremely light pink cards (#FFF7FB) with pink borders, pink sidebar bar (#F8DDE9) with white selected pill, dark text (#2A0F1E). Dark mode: neutral obsidian canvas (#121114), layered charcoal surfaces, hot pink accent (#FF4FA3), zinc muted text (#9E98A6), purple focus ring (#A78BFA), blue links (#7DB4FF). Stat-card icons that were amber are now purple (#8B5CF6); success/warning/danger remain semantic per `contracts/theme-tokens.md`. The lunaris library variables (`$c:--*`) are read-only in Pencil, so every screen and Gameplane definition in `design.pen` is re-pointed to HeroUI semantic tokens instead of `$c:` references. Five light-mode preview frames (jOo7y Login, Qqi8Q Dashboard Home, zFiOW Servers, sSISK Server Detail Overview, vvxCn Mobile Servers) are added so both modes are visible side by side in Pencil. All token tables are finalized in `contracts/theme-tokens.md`; the code side (globals.css, theme.test.tsx, built by T017/T018) still carries the old orange values and is updated in a follow-up code commit of slice 0 (T032d, T032e).

## Per-slice execution (Workflow shape)

1. **Design wave** (haiku, one agent per 4–6 screens, Pencil MCP): redraw from `LtgNm` definitions per `contracts/component-map.md`; swap `$c:` fills/text to HeroUI variables; keep layout, copy and node names; re-export JSON (depth ≥ 12, zero `"..."` markers) and PNG; update `MANIFEST.md`. Then the maintainer saves in the Pencil GUI and the design commit lands.
2. **Code wave** (haiku, one agent per route/tab/component file, worktree isolation): translate the exported frame to React on `@heroui/react` + `components/hero/`, update the file's tests to the new markup, keep every `it()`; `tsc --noEmit` before returning.
3. **Review** (sonnet): diff vs. spec, contracts and exported PNGs; FR-012 import check; test-count check; lists defects and a fix plan.
4. **Fix wave** (haiku) in a second Workflow; then push, watch CI (`web`, `web-e2e-mock`, `e2e-web-live`), fix with follow-up commits, request review only when green.
5. **Screen verification**: the `web/e2e/screenshots` spec renders each rebuilt screen at 1440 (or 390) and the reviewer compares against `design-export/screenshots/<id>.png` per `contracts/screen-verification.md`.

## Risks and mitigations

- **Token collision** between legacy `--surface/--border/--muted` triplets and HeroUI colours: resolved in slice 0 by the `--gp-*` rename and alias policy (`contracts/theme-tokens.md`); a Vitest snapshot of computed `:root` variables guards it.
- **Test churn**: ~1,600 cases assert on DOM. Semantic queries dominate (R-07), so most survive; the 35 `getByTestId` and 14 `querySelector` sites are the ones expected to need edits. Rule: a test edit is justified only by a markup change in the same slice; test files never decrease (SC-005).
- **Behaviour drift** in forms with draft-until-save and conflict detection (Settings): the code wave keeps state hooks and handlers verbatim and swaps only the rendered controls; the live Playwright specs `settingsSubTabs` and `serverLifecycleUI` gate it.
- **Console/Logs engines**: xterm and the virtualised log view stay; only chrome changes.
- **Pencil MCP nested-node resolution** (R-05 caveat): open the repo `design.pen` in the GUI before any design wave; verify with a `Get` on a HeroUI definition id before dispatching agents.

## Complexity Tracking

No constitution violation requires justification. All open decisions are settled 2026-09-03.
