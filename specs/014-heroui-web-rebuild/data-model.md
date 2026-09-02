# Data model — feature 014

This feature adds no persisted data. The entities below are the design and delivery objects the plan, tasks and reviews reason about. Where a rule is an exception to a spec requirement's blanket wording, it is marked **Exception** (rule 15).

## Design screen

A top-level `Screen/…` frame in `design.pen`.

| Field | Value |
|---|---|
| `id` | Pencil node id (e.g. `N1GkB`) |
| `name` | `Screen/<Area> — <State>` |
| `width × height` | 1440 × 900 desktop; 390 × 844 mobile (`tooKB`, `SeizD`) |
| `theme` | today `{"c:Mode":"Dark"}` (lunaris); after redraw the HeroUI semantic mode, dark |
| `slice` | 0–5 per `plan.md` |
| `export` | `design-export/json/<id>.json` (depth ≥ 12, no `"..."`), `design-export/screenshots/<id>.png` |
| `code surface` | the route/tab/component file(s) that implement it (`research.md` R-06) |

Count: 79. Invariant: the count does not change in this feature; no screen is added or removed, only redrawn. **Exception**: the appearance toggle (OD-2) may add one state frame to an existing shell screen rather than a new screen.

States: `lunaris` → `redrawn` (Pencil edit saved) → `exported` (JSON + PNG refreshed, MANIFEST row) → `translated` (code merged) → `verified` (screenshot comparison accepted). A screen may not be `translated` before it is `exported` (Constitution II).

## Design component

A reusable definition in `design.pen`. Three families:

| Family | Naming | Count | Fate |
|---|---|---|---|
| Gameplane composition | `Gameplane/…` | 57 | redrawn on HeroUI definitions (FR-002) |
| HeroUI definition | unprefixed, inside `LtgNm` | 192 named | consumed; exported in slice 0; never edited except to set variable values |
| lunaris primitive | `c:…` inside `c:frame-1761929672442` | 100 | retired: zero references from any screen or Gameplane composition at completion (FR-008). The definitions themselves stay in the file until the maintainer approves deleting them (a design edit). |

Relationship: a composition references definitions; a screen references compositions and definitions. Rebuild order is leaf-first: atoms (slice 0) → shell compositions (slice 1) → per-area compositions in their slice.

## Theme token

A named value shared between the design file and the shipped CSS. Full mapping in `contracts/theme-tokens.md`.

| Field | Value |
|---|---|
| `heroui name` | e.g. `--accent`, `--surface`, `--danger-soft` |
| `pencil variable` | e.g. `$accent/accent`, `$surface/surface` |
| `light value` / `dark value` | oklch, derived from today's HSL brand values |
| `legacy alias` | the `--color-*` name old screens still use, defined as `var(--<heroui name>)` until slice 5 |

Invariant: the set of legacy aliases only shrinks; slice 5 deletes it. `--color-violet` is a permanent Gameplane extra, not an alias.

## Dashboard surface

A shipped page, tab, settings section, dialog or drawer.

| Field | Value |
|---|---|
| `file` | path under `web/src/` |
| `route` / `tab` / `section` | from `web/src/router/tree.tsx`, `ServerDetail.tsx`, `tabs/Settings.tsx` |
| `permission` | unchanged by this feature |
| `family` | `old` (imports `@/components/ui/` or Radix) or `hero` (imports `@heroui/react` or `@/components/hero/`) — **never both** (FR-012) |
| `tests` | co-located `*.test.tsx`; count per file may only rise |
| `screens` | the design screens it implements |

Two surfaces are new rather than rebuilt (share-link settings section, public share page); see `contracts/share-link-ui.md`.

## Slice

| Field | Value |
|---|---|
| `id` | 0–5 |
| `branch` | `014a-…` … `014g-…` per `plan.md` |
| `screens`, `components`, `surfaces` | as listed in `plan.md` |
| `gates` | design commit with export; `web`, `web-e2e-mock`, `e2e-web-live` green; screenshot verification accepted; `web/specs.md` updated; labels applied |
| `state` | `planned` → `designed` → `coded` → `reviewed` → `green` → `merged` |

Invariant: slice N+1's branch is cut from `master` after slice N merges; FR-008 deletion happens only in slice 5.

## Share link (types added to `web/src/types.ts` in slice 5)

Mirrors the API's `share_links` row and public resolve response; fields are taken from `api/internal/handlers/shares.go` at implementation time, not invented here. Public responses must carry nothing beyond what the link exposes (FR-005).
