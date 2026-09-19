# Tasks: User Theme Customization

**Input**: Design documents from `specs/016-user-theme-customization/`  
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`  
**Feature Branch**: `016-user-theme-customization`

---

## Format: `- [ ] [TaskID] [P?] [Story?] Description with file path`

- **[P]**: Parallelizable task (different files, no blocking dependencies)
- **[US1]..[US4]**: User Story label mapping to `spec.md` user journeys

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Database schema definition, client types, and API endpoint bindings.

- [ ] T001 Create database migration file for user theme preferences in `api/internal/db/migrations/010_user_theme_preferences.sql`
- [ ] T002 [P] Define TypeScript interfaces and types for theme preferences in `web/src/types.ts`
- [ ] T003 [P] Register API client methods for user preferences in `web/src/lib/endpoints.ts`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core backend persistence, API handlers, and React state hooks that MUST be complete before user stories.

- [ ] T004 Implement database queries and store methods for user preferences in `api/internal/db/preferences.go`
- [ ] T005 [P] Add unit tests for user preferences DB methods and queries in `api/internal/db/preferences_test.go`
- [ ] T006 Mount and implement `/users/me/preferences` endpoints and extend `/users/me` in `api/internal/handlers/users.go`
- [ ] T007 [P] Add unit tests for preferences endpoints, validation, and error handling in `api/internal/handlers/users_preferences_test.go`
- [ ] T008 Implement `useThemePreferences` React hook for loading and mutating theme preferences in `web/src/hooks/useThemePreferences.ts`

**Checkpoint**: Backend persistence and client query/mutation hooks are operational. User story implementation can now begin.

---

## Phase 3: User Story 1 - Select and Switch Between Default Preset Themes (Priority: P1) 🎯 MVP

**Goal**: Enable users to switch between the modern Pink preset and the classic Legacy Orange preset without page reload, supporting both Dark and Light appearance modes.

**Independent Test**: Open the appearance menu, select "Legacy", observe orange accents (`#F97316`) and dark neutral surfaces apply across the application; select "Pink", observe modern pink accents (`#FF4FA3`) apply.

- [ ] T009 [P] [US1] Add token definitions for `[data-theme-preset="legacy"]` in dark and light modes to `web/src/styles/globals.css`
- [ ] T010 [P] [US1] Update synchronous boot script in `web/index.html` to apply `data-theme-preset` from localStorage
- [ ] T011 [US1] Build `ThemeSettingsModal` preset selection cards and appearance selector in `web/src/components/hero/ThemeSettingsModal.tsx`
- [ ] T012 [P] [US1] Add unit test for preset token assertion (Pink vs Legacy) in `web/src/__tests__/theme.test.tsx`
- [ ] T013 [P] [US1] Add unit test for boot script preset initialization in `web/src/__tests__/themeBoot.test.ts`
- [ ] T014 [US1] Add "Theme & Appearance" trigger item to user avatar menu in `web/src/components/hero/TopBar.tsx`
- [ ] T015 [US1] Add theme settings button trigger to sidebar footer in `web/src/components/hero/Sidebar.tsx`
- [ ] T016 [P] [US1] Add unit tests for `ThemeSettingsModal` preset selection in `web/src/components/hero/ThemeSettingsModal.test.tsx`

**Checkpoint**: Users can select between Modern Pink and Legacy Orange presets with full dark/light support. MVP is fully functional.

---

## Phase 4: User Story 2 - User Theme Migration and Preference Persistence (Priority: P1)

**Goal**: Ensure existing users are migrated to the Legacy theme via database migration, newly created accounts default to Pink, preferences persist across devices, and unauthenticated pages render Pink default with no custom CSS.

**Independent Test**: Sign in with an account created prior to migration and verify default Legacy theme; sign in with a new account and verify default Pink theme; change theme, reload or switch browsers and observe persisted theme; open `/login` and verify Pink theme preset is rendered.

- [ ] T017 [US2] Implement migration logic in `api/internal/db/migrations/010_user_theme_preferences.sql` to seed pre-existing users with `preset_id = 'legacy'` and default new accounts to `pink`
- [ ] T018 [P] [US2] Add migration verification test in `api/internal/db/preferences_migration_test.go`
- [ ] T019 [US2] Integrate backend preferences hydration and localStorage synchronization in `web/src/components/AppLayout.tsx`
- [ ] T020 [P] [US2] Enforce unauthenticated page default theme (Pink preset) and custom CSS exclusion on `/login` in `web/src/routes/Login.tsx`
- [ ] T021 [P] [US2] Enforce unauthenticated page default theme (Pink preset) and custom CSS exclusion on `/share/:token` in `web/src/routes/Share.tsx`
- [ ] T022 [P] [US2] Add unit tests verifying unauthenticated theme isolation in `web/src/routes/Login.test.tsx` and `web/src/routes/Share.test.tsx`

**Checkpoint**: User migration is automated, preferences persist across devices, and unauthenticated surfaces remain secure and consistent.

---

## Phase 5: User Story 3 - Personalize Dashboard with a Simple Custom Color Scheme (Priority: P2)

**Goal**: Provide a code-free color picker allowing users to configure primary accent color and background surface tone with automated WCAG AA contrast calculation.

**Independent Test**: Select custom accent (e.g. Emerald `#10B981`) and surface tone in Theme Settings, verify primary buttons and active markers update in real time with high contrast text, and verify "Reset to Default Preset" restores the preset.

- [ ] T023 [P] [US3] Create color math and contrast calculation utility in `web/src/lib/theme-derivation.ts`
- [ ] T024 [P] [US3] Add unit tests for contrast calculations and token derivation in `web/src/lib/theme-derivation.test.ts`
- [ ] T025 [US3] Implement Custom Colors tab (accent color picker, surface selector, live preview) in `web/src/components/hero/ThemeSettingsModal.tsx`
- [ ] T026 [US3] Implement dynamic custom variables injector element (`#gameplane-custom-theme-vars`) in `web/src/components/AppLayout.tsx`
- [ ] T027 [P] [US3] Add unit test for custom color scheme application in `web/src/components/hero/ThemeSettingsModal.test.tsx`

**Checkpoint**: Simple custom color schemes can be configured, previewed live, applied across the app, and reset on demand.

---

## Phase 6: User Story 4 - Apply and Manage Custom CSS (Priority: P3)

**Goal**: Provide a custom CSS editor for advanced users with 64KB length cap, HTML sanitization, isolated DOM injection, and Safe Mode recovery via `?safe-theme=1`.

**Independent Test**: Enter custom CSS (e.g. `body { border: 2px solid red; }`), save, verify styling applies; trigger Safe Mode with `?safe-theme=1`, verify custom CSS is disabled and recovery banner allows resetting styles.

- [ ] T028 [US4] Implement Custom CSS tab (code textarea, character counter, forbidden tag validation) in `web/src/components/hero/ThemeSettingsModal.tsx`
- [ ] T029 [US4] Implement custom CSS `<style id="gameplane-custom-css">` injection with HTML sanitization in `web/src/components/AppLayout.tsx`
- [ ] T030 [P] [US4] Create `SafeModeBanner` component with recovery actions in `web/src/components/hero/SafeModeBanner.tsx`
- [ ] T031 [US4] Add URL query param `?safe-theme=1` detection and banner mount in `web/src/components/AppLayout.tsx`
- [ ] T032 [P] [US4] Add unit tests for Safe Mode activation and custom CSS injection in `web/src/components/hero/SafeModeBanner.test.tsx`

**Checkpoint**: Custom CSS can be written and applied safely with guaranteed recovery through Safe Mode.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: End-to-end browser test automation, design documentation sync, and module specs update.

- [ ] T033 Create automated Playwright E2E test suite in `web/e2e/specs/theme-customization.spec.ts`
- [ ] T034 Design `ThemeSettingsModal` and appearance states in `design.pen` via Pencil MCP and export assets to `design-export/`
- [ ] T035 [P] Update web module specification documentation in `web/specs.md`
- [ ] T036 [P] Update API module specification documentation in `api/specs.md`

---

## Dependencies & Execution Graph

```mermaid
graph TD
    subgraph Phase 1: Setup
        T001[T001: SQL Migration]
        T002[T002: TypeScript Types]
        T003[T003: API Client Endpoints]
    end

    subgraph Phase 2: Foundational
        T004[T004: DB Store Methods]
        T005[T005: DB Tests]
        T006[T006: User Handler Preferences]
        T007[T007: Handler Tests]
        T008[T008: useThemePreferences Hook]
    end

    subgraph Phase 3: User Story 1 (MVP)
        T009[T009: Legacy CSS Tokens]
        T010[T010: Boot Script Preset Init]
        T011[T011: ThemeSettingsModal UI]
        T012[T012: Theme Token Tests]
        T013[T013: Boot Script Tests]
        T014[T014: TopBar Avatar Menu Trigger]
        T015[T015: Sidebar Footer Trigger]
        T016[T016: Modal Preset Tests]
    end

    subgraph Phase 4: User Story 2 (Persistence & Migration)
        T017[T017: Existing User Migration SQL]
        T018[T018: Migration Test]
        T019[T019: AppLayout Hydration Sync]
        T020[T020: Login Unauthenticated Isolation]
        T021[T021: Share Link Isolation]
        T022[T022: Unauthenticated Route Tests]
    end

    subgraph Phase 5: User Story 3 (Custom Colors)
        T023[T023: Color Derivation Utility]
        T024[T024: Derivation Tests]
        T025[T025: Custom Colors Tab]
        T026[T026: Dynamic Vars Injector]
        T027[T027: Custom Colors Tests]
    end

    subgraph Phase 6: User Story 4 (Custom CSS)
        T028[T028: Custom CSS Tab]
        T029[T029: Custom CSS Injection]
        T030[T030: SafeModeBanner Component]
        T031[T031: Safe Mode Query Detection]
        T032[T032: Safe Mode Tests]
    end

    subgraph Phase 7: Polish
        T033[T033: Playwright E2E Suite]
        T034[T034: Pencil MCP Design Export]
        T035[T035: web/specs.md Update]
        T036[T036: api/specs.md Update]
    end

    T001 --> T004
    T002 --> T003
    T004 --> T005
    T004 --> T006
    T006 --> T007
    T003 --> T008
    T006 --> T008

    T008 --> T011
    T009 --> T011
    T010 --> T011
    T009 --> T012
    T010 --> T013
    T011 --> T014
    T011 --> T015
    T011 --> T016

    T001 --> T017
    T017 --> T018
    T008 --> T019
    T019 --> T020
    T019 --> T021
    T020 --> T022
    T021 --> T022

    T011 --> T025
    T023 --> T024
    T023 --> T025
    T025 --> T026
    T025 --> T027

    T011 --> T028
    T028 --> T029
    T030 --> T031
    T029 --> T031
    T030 --> T032

    T016 --> T033
    T022 --> T033
    T027 --> T033
    T032 --> T033
    T033 --> T034
    T033 --> T035
    T033 --> T036
```

---

## Parallel Execution Opportunities

- **Setup Phase**: T002 (`web/src/types.ts`) and T003 (`web/src/lib/endpoints.ts`) can execute in parallel with T001 (`api/internal/db/migrations/010_user_theme_preferences.sql`).
- **Foundational Phase**: T005 (DB tests) can run in parallel with T007 (Handler tests).
- **User Story 1**: T009 (`globals.css`), T010 (`index.html`), T012 (`theme.test.tsx`), and T013 (`themeBoot.test.ts`) can all run concurrently before assembling `ThemeSettingsModal.tsx`.
- **User Story 2**: T020 (`Login.tsx`), T021 (`Share.tsx`), and T022 (Route tests) can execute in parallel with T018 (migration test).
- **User Story 3**: T023 (`theme-derivation.ts`) and T024 (`theme-derivation.test.ts`) are completely self-contained pure TypeScript functions and can run in parallel with UI tasks.
- **User Story 4**: T030 (`SafeModeBanner.tsx`) and T032 (`SafeModeBanner.test.tsx`) can be developed in parallel with T028 (`ThemeSettingsModal.tsx` CSS tab).
- **Polish Phase**: T035 (`web/specs.md`) and T036 (`api/specs.md`) can run concurrently.

---

## Implementation Strategy & MVP Scope

- **MVP Scope**: **Phase 1, Phase 2, and Phase 3 (User Story 1)** deliver the primary user requirement: immediate, in-app switching between the modern Pink preset and the legacy Orange & Dark preset with full Light and Dark mode support.
- **Incremental Releases**:
  1. *Slice 1 (MVP)*: Default presets switching + token architecture (US1).
  2. *Slice 2*: Database persistence & automated user migration to Legacy (US2).
  3. *Slice 3*: Simple custom color scheme derivation & editor (US3).
  4. *Slice 4*: Custom CSS editor, isolated injection, and Safe Mode recovery (US4).
  5. *Slice 5*: Playwright E2E coverage, Pencil design export, and spec updates.
