# Implementation Plan: User Theme Customization

**Branch**: `016-user-theme-customization` | **Date**: 2026-09-19 | **Spec**: ./spec.md

**Input**: Feature specification from `specs/016-user-theme-customization/spec.md`

## Summary

This feature delivers user-customizable visual themes to Gameplane, providing two curated presets ("Modern Pink" and "Legacy Orange"), a simple code-free color scheme editor, and an advanced custom CSS injection mode.

Key architecture points:
1. **User Migration & Backend Persistence**: A new migration (`api/internal/db/migrations/010_user_theme_preferences.sql`) creates `user_preferences` and migrates all pre-existing user accounts to the `legacy` orange & dark theme. New users default to the modern `pink` theme. Preferences sync across devices via `GET/PUT /api/v1/users/me/preferences` and hydrate directly within `useMe()`.
2. **CSS Token System**: `web/src/styles/globals.css` extends HeroUI semantic tokens with `data-theme-preset="legacy"`, mapping the orange accent (`#F97316`) and dark neutrals (`#0F0F0F`, `#171717`, `#1C1C1C`) to HeroUI's base tokens.
3. **Simple Custom Colors**: A TypeScript derivation utility calculates accessible semantic tokens from user-selected Primary Accent and Surface tones.
4. **Safe Custom CSS**: Injected into the document via `<style id="gameplane-custom-css">` with a 64 KB cap and HTML tag stripping. An escape hatch via `?safe-theme=1` bypasses custom CSS and presents a recovery banner.
5. **UI & Design**: A dedicated `ThemeSettingsModal` built on HeroUI primitives is triggered from the TopBar user avatar dropdown and Sidebar appearance footer.

---

## Technical Context

**Language/Version**: Go 1.25 (backend API), TypeScript 6.0.3 strict (frontend web), React 19.2.8.

**Primary Dependencies**:
- Backend: `chi/v5`, standard library `database/sql`, `modernc.org/sqlite`.
- Frontend: `@heroui/react` 3.2.4, `@heroui/styles` 3.2.4, `@tanstack/react-query`, `lucide-react`.

**Storage**: SQLite and PostgreSQL (opt-in) via `user_preferences` table with cascading foreign key to `users(id)`. Client-side caching in `localStorage` under `gameplane-theme-prefs`.

**Testing**:
- Go unit tests: `internal/db` (migration verification), `internal/handlers` (preferences endpoints).
- Web unit tests: Vitest 4.1.11 (`theme.test.tsx`, `themeBoot.test.ts`, `ThemeSettingsModal.test.tsx`).
- E2E tests: Playwright (`e2e/specs/theme-customization.spec.ts`).

**Target Platform**: Evergreen desktop (1440px) and mobile (390px) browsers served by the Gameplane API.

**Project Type**: Full-stack web application (Go API service + React web frontend).

**Performance Goals**:
- Client-side theme switching completes in < 100ms without full page reload.
- Zero Flash of Unstyled Content (FOUC) on application boot via synchronous `index.html` boot script.

**Constraints**:
- Constitution Principle I: E2E coverage for theme switching and safe-mode recovery.
- Constitution Principle II: Theme Settings Modal visual surface designed in `design.pen` via Pencil MCP server before code implementation.
- Constitution Principle III: Strict TypeScript, Go `%w` error wrapping, no `//nolint` or `// @ts-ignore`.
- FR-003: 100% of pre-existing accounts migrated to Legacy theme.
- FR-011: Unauthenticated public pages (login and share links) strictly render with the default Pink theme preset and never execute custom CSS.

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | How this plan satisfies it |
|---|---|---|
| **I. E2E-Tested Delivery** | PASS | Playwright E2E tests (`web/e2e/specs/theme-customization.spec.ts`) verify preset switching, persistence across reloads, custom color application, and safe-mode recovery on live/mock cluster. |
| **II. Design-First** | PASS | The `ThemeSettingsModal` and appearance controls will be created in `design.pen` via the Pencil MCP server and exported to `design-export/` before React code is merged. |
| **III. Language & Ecosystem** | PASS | Strict TypeScript enabled; error wrapping with `%w`; zero in-source linter suppressions (`//nolint`, `// @ts-ignore`). Coverage gates remain intact. |
| **IV. Spec-Driven** | PASS | Specification, plan, research, data model, contracts, and quickstart guide precede implementation. `web/specs.md` and `api/specs.md` will be updated upon implementation. |
| **V. Delegate to Workflows** | PASS | Implementation tasks will be fanned out across independent slices (API/DB slice, CSS token slice, UI modal slice, E2E test slice) with tier-appropriate review. |
| **VI. CI Bears the Heavy Lifting** | PASS | All unit tests, migrations, linting, and Playwright E2E suites run on GitHub Actions CI; only local `go build` and `tsc --noEmit` checks. |

*Post-design re-check (after Phase 1): All principles continue to pass. No architectural violations.*

---

## Project Structure

### Documentation (this feature)

```text
specs/016-user-theme-customization/
├── spec.md                  # Feature specification with clarification resolutions
├── plan.md                  # This implementation plan
├── research.md              # Phase 0: Technical decisions (R-01 to R-05)
├── data-model.md            # Phase 1: Entity model, DDL, TypeScript interfaces
├── quickstart.md            # Phase 1: Runnable end-to-end verification guide
├── contracts/
│   ├── user-preferences-api.md  # REST API contract for /users/me/preferences
│   ├── theme-tokens-v2.md       # Semantic token mappings for Pink and Legacy presets
│   └── theme-ui.md              # UI contract for ThemeSettingsModal and triggers
├── checklists/
│   └── requirements.md      # Specification quality checklist (validated)
└── tasks.md                 # Phase 2 output (/speckit-tasks command)
```

### Source Code Layout

```text
api/
├── internal/
│   ├── db/
│   │   ├── migrations/
│   │   │   └── 010_user_theme_preferences.sql  # NEW: table & existing user migration
│   │   ├── preferences.go                      # NEW: DB queries for user_preferences
│   │   └── preferences_test.go                 # NEW: unit tests for migration and queries
│   └── handlers/
│       ├── users.go                            # UPDATE: mount /users/me/preferences & extend /users/me
│       └── users_preferences_test.go           # NEW: tests for preferences endpoints

web/
├── index.html                                  # UPDATE: boot script reads cached preset & theme type
├── src/
│   ├── types.ts                                # UPDATE: add UserThemePreferences & extend User interface
│   ├── styles/
│   │   └── globals.css                         # UPDATE: add [data-theme-preset="legacy"] tokens
│   ├── lib/
│   │   ├── endpoints.ts                        # UPDATE: add Users.getPreferences / updatePreferences
│   │   ├── theme-derivation.ts                 # NEW: derive custom color tokens from accent & surface
│   │   └── theme-derivation.test.ts            # NEW: unit tests for contrast and color math
│   ├── hooks/
│   │   └── useThemePreferences.ts              # NEW: hook for reading & updating theme state
│   ├── components/
│   │   ├── hero/
│   │   │   ├── TopBar.tsx                      # UPDATE: add "Theme & Appearance" item in user dropdown
│   │   │   ├── Sidebar.tsx                     # UPDATE: add settings trigger button in footer
│   │   │   ├── ThemeSettingsModal.tsx          # NEW: HeroUI modal with Presets, Colors, CSS tabs
│   │   │   ├── ThemeSettingsModal.test.tsx     # NEW: unit tests for modal interactions
│   │   │   └── SafeModeBanner.tsx              # NEW: floating banner when ?safe-theme=1 is active
│   │   └── AppLayout.tsx                       # UPDATE: integrate theme provider & safe mode banner
│   └── __tests__/
│       ├── theme.test.tsx                      # UPDATE: assert Pink vs Legacy token values
│       └── themeBoot.test.ts                   # UPDATE: assert preset initialization from localStorage
└── e2e/
    └── specs/
        └── theme-customization.spec.ts         # NEW: Playwright tests for preset & safe-mode flows
```

---

## Complexity Tracking

*No constitutional or architectural violations. No entry required.*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|---|---|---|
| None | N/A | N/A |
