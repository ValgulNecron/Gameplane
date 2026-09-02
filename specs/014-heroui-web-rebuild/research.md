# Research: Rebuild the dashboard on HeroUI (feature 014)

**Date**: 2026-09-02. Phase 0 of `/speckit-plan`. Five haiku researchers ran in one Workflow (`wf_e478b954-826`), one sonnet reviewer spot-checked 24 of their claims against the repo and the live HeroUI sources, and the main loop re-verified every load-bearing figure below itself. Two haiku claims were wrong and are corrected here (R-02, R-08). Every number is traceable to a file, a command, or a URL.

## R-01 — What the dashboard is built from today

**Decision**: treat the 19 hand-rolled primitives in `web/src/components/ui/` plus six `@radix-ui/*` packages as the layer being replaced; nothing else in `web/src/lib/` changes.

**Facts** (`web/package.json`, verified 2026-09-02):

| Item | Value |
|---|---|
| React / React DOM | 19.2.8 |
| TypeScript | 6.0.3 (TypeScript 7 is blocked upstream by `@typescript-eslint`, see `CLAUDE.md`) |
| Vite / Vitest | 8.2.2 / 4.1.11 |
| Tailwind CSS | 4.3.3 via `@tailwindcss/postcss` (`web/postcss.config.js`); `web/tailwind.config.ts` still declares `darkMode: "class"` and `content` |
| Primitives | 19 files in `web/src/components/ui/`, imported by 272 non-test consumer files; `button.tsx` 45 consumers, `input.tsx` 31, `card.tsx` 17, `select.tsx` 13, `confirm-dialog.tsx` 9 |
| Radix | `react-dialog` (14 consumers), `react-dropdown-menu` (2), `react-slot` (1, `button.tsx` asChild); `react-label`, `react-tabs`, `react-toast` have zero consumers |
| Shell | `web/src/components/AppLayout.tsx` (612 lines): sidebar, top bar, breadcrumbs, mobile drawer (Radix `Dialog.Portal`), notifications panel and global search (both absolutely-positioned, not portals), cluster selector (Radix dropdown) |
| Theme | `web/src/styles/globals.css`: HSL-triplet tokens `--bg --surface --card --border --muted --fg --primary --primary-fg --success --warning --danger --violet` on `:root` (light) and `.dark`, mapped to Tailwind `--color-*` in an `@theme` block; fonts Geist and JetBrains Mono from Google Fonts (`web/index.html`) |
| Appearance | `<html lang="en" class="dark">` hard-coded in `web/index.html:2`; no light-mode toggle exists anywhere in `web/src` |

**Rationale**: the primitive layer is small and well-bounded; the domain layer (`lib/api.ts`, `endpoints.ts`, `ws.ts`, `sse.ts`, TanStack Query hooks) never touches presentation and is untouched by this feature.

**Alternatives considered**: rewriting `lib/` alongside (rejected — FR-004 preserves behaviour, and the API client is not part of the component family).

## R-02 — HeroUI v3: package, peers, install

**Decision**: adopt `@heroui/react` 3.2.4 and `@heroui/styles` 3.2.4, importing components from the root package, and importing the stylesheet with `@import "@heroui/styles";` in `globals.css`.

**Facts** (from `npm view @heroui/react@3.2.4` and `npm view @heroui/styles@3.2.4`, plus `packages/styles/README.md` and `packages/react/README.md` in `heroui-inc/heroui` on GitHub, read 2026-09-02):

- Latest stable of both packages: **3.2.4**. HeroUI v2 is the previous major and cannot coexist with v3.
- `@heroui/react` peer dependencies, exactly: `react >=19.0.0`, `react-dom >=19.0.0`, `tailwindcss >=4.0.0`, `react-aria ^3.51.0`, `react-aria-components ^1.20.0`, `@react-aria/i18n ^3.13.1`, `@react-aria/ssr ^3.10.1`, `@react-aria/utils ^3.34.1`. These are peers, not bundled; they are added to `web/package.json` as real dependencies. *(Corrects the haiku claim that React Aria is bundled.)*
- `@heroui/styles` peer dependency: `tailwindcss >=4.0.0`. Its root import pulls Tailwind's base, the component styles, the utilities, the default theme variables, and `tw-animate-css`. Granular imports exist (`@heroui/styles/components/button.css` with `layer(components)`, `@heroui/styles/themes/default` with `layer(base)`).
- **One npm package, not per-component packages.** The monorepo's top-level packages are `react`, `standard`, `storybook`, `styles`, `testing`. Subpath exports do exist inside `@heroui/react` (`@heroui/react/button`, `@heroui/react/alert-dialog`, …), so tree-shaking is available without extra packages. *(Corrects the haiku claim of `@heroui/button` et al.)*
- No `<Provider>` wrapper is required (package README, verified). No framer-motion dependency; animations are CSS.
- Components are React Aria Components: compound API (`Card.Header`, `Select.Item`, `Table.Column`) with real ARIA roles, so `getByRole` / `getByLabelText` queries keep working.
- Gameplane's current stack already satisfies every peer range (React 19.2.8, Tailwind 4.3.3).

**Alternatives considered**: granular CSS imports per component (deferred — start with the root import, measure the CSS bundle in slice 1, and switch to per-component `layer(components)` imports only if the bundle grows by more than the slice's stated budget in `contracts/screen-verification.md`).

## R-03 — HeroUI component availability against Gameplane's needs

**Decision**: every surface the dashboard needs maps to a shipped HeroUI component except the sidebar/top-bar shell, which is composed from primitives (maintainer decision, 2026-09-02).

**Facts** (component directory of `packages/react/src/components/` on GitHub, 2026-09-02; ~80 folders): accordion, alert, alert-dialog, autocomplete, avatar, badge, breadcrumbs, button, button-group, calendar, card, checkbox(-group), chip, close-button, combo-box, date-*/time-field, description, disclosure(-group), **drawer**, dropdown, empty-state, error-message, field-error, fieldset, form, header, input, input-group, input-otp, kbd, label, link, list-box, menu, meter, modal, **number-field**, pagination, popover, **progress-bar**, **progress-circle**, radio(-group), scroll-shadow, search-field, select, separator, skeleton, slider, spinner, surface, switch(-group), table, tabs, tag(-group), textarea, textfield, toast, toggle-button(-group), toolbar, tooltip, typography.

Absent: **navbar, sidebar, navigation**. HeroUI Pro sells a Sidebar block; the maintainer chose composition from Link, ListBox/Menu, Separator, Avatar, Breadcrumbs and Drawer instead.

Renames a reader of v2 material must know: Progress → ProgressBar / ProgressCircle; NumberInput → NumberField.

## R-04 — Theming and appearance

**Decision**: Gameplane's brand values are written into HeroUI's own token names (see `contracts/theme-tokens.md`); the legacy `--color-*` aliases stay defined in terms of those tokens during the transition and are deleted in the last slice. Dark stays the default; a light/dark/system toggle is added in slice 1 using HeroUI's `useTheme` hook.

**Facts** (heroui.com theming and dark-mode pages, 2026-09-02):

- Tokens are CSS variables: `--background --foreground --surface --overlay --accent --default --success --warning --danger` each with `-foreground`, `-hover`, `-soft`, `-soft-foreground` variants; `--muted --border --separator --focus --link --backdrop`; field tokens `--field-background --field-foreground --field-placeholder --field-border`; `--surface-secondary/-tertiary`, `--background-secondary/-tertiary/-inverse`; `--radius`, `--field-radius`, `--radius-xs … --radius-4xl`; spacing, border widths, shadows, scrollbar and animation tokens. Values are `oklch()`.
- Dark mode: `.dark` class **or** `data-theme="dark"` on `<html>`; light is `:root`/`.light`. The `useTheme` hook (from `@heroui/react`) persists to `localStorage` and resolves `system` from the OS preference. Gameplane's existing `class="dark"` on `<html>` therefore keeps working unchanged.
- Custom themes are overrides on `:root` / `.dark`, or a scoped `[data-theme="name"]` block imported with `layer(theme)`.
- **Collision**: Gameplane's legacy `--surface`, `--border`, `--muted` are HSL triplets consumed as `hsl(var(--surface))`; HeroUI defines the same names as colours. They cannot both live on `:root`. Resolution in `contracts/theme-tokens.md`: the legacy triplets are renamed `--gp-*` in slice 1 (one file, `globals.css`, no consumer changes because consumers use Tailwind class names), and the `@theme` aliases `--color-card`, `--color-fg`, `--color-primary`, `--color-primary-fg`, `--color-violet`, `--color-surface`, `--color-border`, `--color-muted` are re-pointed at the HeroUI tokens so un-rebuilt screens render in the same colours.
- `violet` has no HeroUI counterpart; it stays a Gameplane-only extra token (`--color-violet`) for the operator-role chips.

## R-05 — Pencil side: what is in `design.pen`

**Decision**: the design pass redraws every screen with the HeroUI definitions in `LtgNm`, switches screen-level fills and text from the lunaris `$c:` variables to the HeroUI variable set, and sets the HeroUI variables to Gameplane's brand values in both semantic modes.

**Facts** (Pencil MCP, 2026-09-02):

- Top-level frame `HeroUI: Design System Components` (`LtgNm`, 3116×3588 at x −1981, y −1285) holds 236 children, 192 named reusable definitions. `heroUI template.pen` at the repo root holds the same frame (`qAjxE`) with the identical 236 children and 192 names, plus the sample `dashboard-utility` and one ref. **The template adds nothing that `design.pen` lacks**; it stays as the fallback only.
- HeroUI definitions use their own variable namespace: `$accent/accent`, `$accent/foreground`, `$surface/surface`, `$background/background`, `$danger/danger`, `$field/background`, `$field/placeholder`, `$foreground/foreground`, `$radius/3xl`, `$radius/xl`, `$spacing/2`, `$typography/font-sans`. The frame's theme is `{"semantic":"light"}`.
- Existing Gameplane screens use the lunaris namespace: `$c:--background`, `$c:--card`, `$c:--border`, `$c:--primary`, `$c:--foreground`, `$c:--muted-foreground`, `$c:--font-primary`, with theme `{"c:Mode":"Dark"}` (e.g. `design-export/json/N1GkB.json`). Reference sizes: desktop 1440×900, mobile 390×844.
- `dashboard-utility` (`bJ2cg`) and the prompt node `OFfAu` are HeroUI sample artefacts (a "PowerGrid" utility dashboard), not Gameplane screens.
- **Export debt**: `design-export/MANIFEST.md` (last updated 2026-08-28) counts 157 components (57 `Gameplane/` + 100 `c:`) and knows nothing of the HeroUI frame, although the committed `design.pen` already contains it. No `LtgNm` or HeroUI-definition JSON/PNG exists under `design-export/`. Slice 0 refreshes this.
- Lunaris usage: 100 `c:` primitives; the heaviest are `Sidebar Item/Active` (70 refs), `Sidebar Item/Default` (69), `Card` (29), `Label/Success` (12), table parts (10–12 each). 22 lunaris primitives have no one-to-one HeroUI definition in the Pencil frame: the Sidebar family, Breadcrumb items, List items, Label / Icon Label colour variants, Progress. All are covered by compositions listed in `contracts/component-map.md`. Only 6 of the 57 `Gameplane/` components reference lunaris primitives directly; the other 51 are built from raw frames, so all 57 are redrawn regardless (FR-002) and the transitive question does not change scope.
- **Tooling caveat**: `mcp__pencil__execute` with an explicit `filePath` resolved top-level nodes in the repo copy but failed on nested ids (`Can't find node 'cb4rt'`) until the same call was made against the editor's active document. Before the design pass, the repo copy `/home/valgul/project/Gameplane/design.pen` must be the document open in the Pencil GUI; the identical copy under `kubernetes-game-dashboard/` is not edited.

## R-06 — Screen-to-code map and what is not built

**Decision**: 72 of the 79 designed screens have shipped code and are rebuilt; 7 are designed but unbuilt and are **built new** in slice 5 (maintainer decision, 2026-09-02).

**Facts** (verified against `web/src/router/tree.tsx`, `web/src/routes/`, `api/internal/handlers/shares.go`):

- Routes: `/login`, `/`, `/servers`, `/servers/new`, `/servers/$name` (11 tabs incl. `capture`, `ServerDetail.tsx:52,278`), `/modules`, `/backups`, `/cluster` (`servers:write`), `/users` (`users:manage`), `/admin` (`config:manage`), `/admin/audit` (`audit:read`), `/admin/logs` (`*`).
- The Capture tab **is** implemented (`web/src/components/CaptureWidget.tsx`, wired at `ServerDetail.tsx:278`). *(Corrects the haiku map that listed the six Capture screens as unbuilt.)*
- Settings sections implemented under `web/src/routes/tabs/settings/`: Access, Backups, Danger, EnvVars, General, Lifecycle, NetworkCapture, Networking, Placement, Resources, Version (11). **No Share links section exists.**
- Share links: the API is complete — `MountShareLinks` (`POST/GET /servers/{name}:shares`, `DELETE /servers/{name}/shares/{id}`) and `MountPublicShares` (`GET /shares/{token}`, `POST /shares/{token}/start`, rate-limited, unauthenticated; `api/cmd/main.go:288`, migration `006_share_links.sql`). `web/src` has no share-link code, endpoint, route or test. The seven designed frames (`xCJlu`, `dQV9N`, `C2LQE4`, `q31B6w`, `qFLfB`, `EcoGD`, `epZO2`) and three dialogs (`atqRh`, `VM7ro`, `S7SCDc`) are therefore new work; see `contracts/share-link-ui.md`.
- Per-slice counts are in `plan.md`.

## R-07 — How the dashboard is verified today

**Decision**: keep all four existing gates and add screenshot comparison per screen (FR-009).

**Facts**:

- Unit: Vitest + Testing Library + MSW, 104 test files, about 1,600 `it()` cases (1,607 by direct grep). `web/src/test/setup.ts` already stubs `ResizeObserver`, `IntersectionObserver`, `matchMedia`, pointer capture and `scrollIntoView` — the mocks HeroUI/React Aria need under jsdom are in place. Queries are overwhelmingly semantic (`findBy*` 1,084, `getByRole` 783, `getByText` 566, `getByLabelText` 111, `getByTestId` 35, `querySelector` 14), which survive a component-family change far better than class-based selectors.
- Coverage gate: `web/vitest.config.ts:37-42` lines 92 / functions 76 / branches 82 / statements 92.
- Browser e2e: Playwright in `web/e2e/` — 17 mock specs in `web/e2e/specs/` plus 4 live specs in `web/e2e/specs/live/`, page object `web/e2e/pages/LoginPage.ts`, scripts `test:e2e:mock` and `test:e2e:live` (`GAMEPLANE_E2E_TARGET`). CI jobs in `.github/workflows/ci.yaml`: `web` (line 502; lint, typecheck, unit, coverage), `web-e2e-mock` (548), `e2e-web-live` (955; kind cluster, helm install, bootstrap admin, Playwright live).
- The Go suite in `test/e2e/` drives the REST API only, never a browser.
- Doc gates: `hack/check-specs.sh` (non-empty `web/specs.md`), `hack/check-links.sh`, `hack/check-doc-versions.sh`.

## R-08 — Corrections to the haiku reports (for the record)

| Claim | Status | Correction |
|---|---|---|
| `@heroui/button`, `@heroui/modal` exist as packages | wrong | single `@heroui/react` with subpath exports |
| React Aria is bundled inside `@heroui/react` | wrong | five React Aria packages are peers |
| Capture tab has no code | wrong | `CaptureWidget.tsx`, wired in `ServerDetail.tsx` |
| `it()` count 1,599 | approximate | 1,607 by direct grep; treat as ~1,600 |
| `web` CI job at line 520 | wrong | line 502 |
| Breadcrumbs are a full HeroUI gap | overstated | `breadcrumbs` component exists in code; only the Pencil frame lacks a definition |

## Unresolved items carried to OPEN-DECISIONS.md

OD-1 public share-link route path; OD-2 where the appearance toggle lives; OD-3 Playwright as the dashboard's E2E tier under Principle I's `test/e2e/` wording; OD-4 fate of the `dashboard-utility` sample frame; OD-5 whether `heroUI template.pen` is committed; OD-6 CSS bundle budget.
