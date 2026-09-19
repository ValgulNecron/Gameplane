# Research: User Theme Customization

**Feature**: `016-user-theme-customization`  
**Date**: 2026-09-19  
**Status**: Completed  

---

## Executive Summary

This document establishes the technical decisions, architecture, and verification models for multi-theme support, user-specific theme persistence, legacy migration, simple custom color calculation, and safe custom CSS injection in Gameplane.

---

## Research Items

### R-01: User Preferences Database Storage & Migration Design

**Decision**:  
Create a dedicated `user_preferences` table with a 1-to-1 relationship to `users(id)` with cascading deletion. Use migration `010_user_theme_preferences.sql`. During migration execution:
1. Create the `user_preferences` table.
2. Seed rows for all pre-existing users in the `users` table setting `preset_id = 'legacy'`, `theme_type = 'preset'`, and `appearance_mode = 'system'`.
3. In application code, whenever a user profile is queried without a corresponding `user_preferences` row (e.g. newly created users), the system returns the default: `preset_id = 'pink'`, `theme_type = 'preset'`, and `appearance_mode = 'system'`.

**Rationale**:
- **Clean separation of concerns**: Keeps authentication, RBAC, and credential tables focused on security; preferences can evolve without altering `users` table layout.
- **Portability**: Plain SQL `CREATE TABLE` and `INSERT INTO ... SELECT` works identically across SQLite and PostgreSQL drivers without driver-specific JSON parsing functions.
- **Deterministic Migration**: Ensures existing user accounts are immediately assigned `legacy` upon migration execution, fulfilling requirement FR-003 and SC-001.

**DDL Structure**:
```sql
CREATE TABLE user_preferences (
    user_id          INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    theme_type       TEXT NOT NULL DEFAULT 'preset',     -- 'preset' | 'custom_colors' | 'custom_css'
    preset_id        TEXT NOT NULL DEFAULT 'pink',       -- 'pink' | 'legacy'
    appearance_mode  TEXT NOT NULL DEFAULT 'system',     -- 'light' | 'dark' | 'system'
    custom_accent    TEXT,                               -- hex string, e.g. '#3B82F6'
    custom_surface   TEXT,                               -- hex string, e.g. '#1E1E2E'
    custom_css       TEXT,                               -- user CSS rules
    updated_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_user_preferences_user ON user_preferences(user_id);

-- Migrate all pre-existing accounts to legacy theme
INSERT INTO user_preferences (user_id, theme_type, preset_id, appearance_mode)
SELECT id, 'preset', 'legacy', 'system'
FROM users;
```

**Alternatives Considered**:
- *JSON Column on `users` table*: Rejected because SQLite and Postgres handle JSON extraction differently in SQL queries; a dedicated relational table is simpler to type and query across both database engines.
- *Client-side `localStorage` only*: Rejected per user decision in Q1; client-only storage cannot reliably identify existing accounts across devices or after browser cache clear.

---

### R-02: CSS Variable System & Token Mapping for Pink vs Legacy Themes

**Decision**:  
Structure theme tokens in `web/src/styles/globals.css` using `data-theme-preset` attributes on `<html>`:
- `data-theme-preset="pink"` (default): Uses the existing HeroUI semantic tokens approved in `specs/014-heroui-web-rebuild/contracts/theme-tokens.md` (Pink accent `#FF4FA3` dark / `#DB2777` light).
- `data-theme-preset="legacy"`: Rebinds HeroUI semantic variables (`--accent`, `--surface`, `--background`, `--foreground`, `--border`, etc.) to the original Gameplane palette (Orange accent `#F97316`, dark neutral ground `#0F0F0F`, card `#1C1C1C`, border `#292929`).

**Rationale**:
- Rebinding HeroUI semantic variables ensures that **all rebuilt screens automatically adapt** without code changes or conditional class rendering in React components.
- The existing legacy `--gp-*` tokens in `globals.css` already hold the historical orange palette values. Re-pointing `--accent` and `--surface` when `data-theme-preset="legacy"` delivers pixel-accurate restoration of the classic theme.

**Token Mapping for Legacy Theme**:
```css
/* Legacy Dark Theme */
.dark[data-theme-preset="legacy"],
[data-theme="dark"][data-theme-preset="legacy"] {
  --background: oklch(14.5% 0 0);           /* #0F0F0F */
  --foreground: oklch(96.5% 0 0);           /* #F5F5F5 */
  --surface: oklch(18.5% 0 0);              /* #171717 */
  --surface-secondary: oklch(16.5% 0 0);    /* #141414 */
  --surface-tertiary: oklch(15.0% 0 0);     /* #111111 */
  --overlay: oklch(21.0% 0 0);              /* #1C1C1C */
  --accent: oklch(69.11% 0.1944 44.01);     /* #F97316 orange */
  --accent-foreground: oklch(100% 0 0);     /* #FFFFFF */
  --accent-soft: oklch(25.0% 0.06 45.0);
  --accent-soft-foreground: oklch(80.0% 0.15 45.0);
  --default: oklch(21.0% 0 0);              /* #1C1C1C */
  --default-foreground: oklch(96.5% 0 0);   /* #F5F5F5 */
  --border: oklch(26.0% 0 0);               /* #292929 */
  --separator: oklch(26.0% 0 0);            /* #292929 */
  --muted: oklch(65.0% 0 0);                /* #949494 */
  --field-background: oklch(18.5% 0 0);     /* #171717 */
  --field-border: oklch(26.0% 0 0);         /* #292929 */
  --field-placeholder: oklch(50.0% 0 0);    /* #737373 */
  --field-foreground: oklch(96.5% 0 0);     /* #F5F5F5 */
  --focus: oklch(69.11% 0.1944 44.01);      /* #F97316 */
  --link: oklch(72.0% 0.16 45.0);           /* #FB923C */
  --segment: oklch(21.0% 0 0);              /* #1C1C1C */
  --segment-foreground: oklch(96.5% 0 0);   /* #F5F5F5 */
}

/* Legacy Light Theme */
.light[data-theme-preset="legacy"],
[data-theme="light"][data-theme-preset="legacy"] {
  --background: oklch(100% 0 0);            /* #FFFFFF */
  --foreground: oklch(18.0% 0.03 260);      /* #0F172A */
  --surface: oklch(98.5% 0.005 260);        /* #F8FAFC */
  --surface-secondary: oklch(96.0% 0.01 260);/* #F1F5F9 */
  --surface-tertiary: oklch(91.0% 0.02 260); /* #E2E8F0 */
  --overlay: oklch(98.5% 0.005 260);        /* #F8FAFC */
  --accent: oklch(62.0% 0.20 40.0);         /* #EA580C */
  --accent-foreground: oklch(100% 0 0);     /* #FFFFFF */
  --accent-soft: oklch(95.0% 0.04 45.0);
  --accent-soft-foreground: oklch(55.0% 0.20 40.0);
  --border: oklch(90.0% 0.01 260);          /* #E2E8F0 */
  --separator: oklch(90.0% 0.01 260);       /* #E2E8F0 */
  --muted: oklch(50.0% 0.02 260);           /* #64748B */
  --field-background: oklch(100% 0 0);
  --field-border: oklch(90.0% 0.01 260);
  --field-foreground: oklch(18.0% 0.03 260);
  --focus: oklch(62.0% 0.20 40.0);
  --link: oklch(55.0% 0.20 40.0);
}
```

**Alternatives Considered**:
- *Duplicating component styling*: Rejected because hardcoding classnames per theme produces massive bloat; CSS variable swapping is O(1) in CSS bundle size.

---

### R-03: Simple Custom Color Scheme Derivation Algorithm

**Decision**:  
When `theme_type === "custom_colors"`, the user inputs two values:
1. `custom_accent`: A hex color string (e.g. `#10B981` emerald, `#3B82F6` blue).
2. `custom_surface`: A surface tone selector or hex string (defaulting to dark neutral `#141318` or light `#F8FAFC`).

From these two inputs, a pure TypeScript utility (`deriveCustomThemeTokens`) computes all derivative CSS tokens:
- **Accent foreground**: White `#FFFFFF` or Black `#000000` depending on relative luminance (WCAG AA ratio >= 4.5:1).
- **Accent soft**: 15% opacity tint of accent over the surface.
- **Surface layers**: Base surface, secondary (±3% lightness), tertiary (±5% lightness).
- **Borders**: Surface with elevated lightness (+10% in dark mode, -10% in light mode).
- **Muted text**: Intermediate luminance between surface and foreground ensuring >= 4.5:1 contrast against surface.

**Application**:
The derived tokens are injected into a dedicated `<style id="gameplane-custom-theme-vars">` or applied via `document.documentElement.style.setProperty`.

**Alternatives Considered**:
- *Full manual token editor*: Rejected per user decision in Q2; primary accent + surface tone provides 95% of desired customization with zero risk of broken intermediate states.

---

### R-04: Custom CSS Injection & Safe-Mode Recovery Architecture

**Decision**:  
1. **Injection**: Custom CSS is injected into the DOM via a single `<style id="gameplane-custom-css">` element placed at the end of `<head>`.
2. **Sanitization**:
   - Strip any closing `</style>` tags or HTML tags to prevent DOM escape.
   - Restrict maximum CSS length to 64 KB.
3. **Strict Isolation**:
   - Evaluated **only** when a user is authenticated (`me` is loaded).
   - Unauthenticated pages (Login `/login` and Share `/share/:token`) **never** inject user custom CSS.
4. **Safe-Mode Recovery**:
   - If user CSS hides the UI or breaks clicks, users can bypass custom CSS via:
     - URL parameter: `?safe-theme=1` or `?safe_mode=1`.
     - Key sequence: Pressing `Ctrl + Shift + Alt + Escape` or clicking the "Reset Theme" link in the user menu.
   - When Safe Mode is active:
     - Custom CSS injection is completely skipped.
     - A discreet floating banner appears: *"Safe Mode active (Custom CSS suspended). [Edit / Clear Custom CSS]"*.

**Alternatives Considered**:
- *Iframe sandboxing*: Rejected because custom CSS is intended to style the application shell and dashboard screens directly; iframing the entire app would destroy layout and state management.

---

### R-05: Client-Side Hydration, Boot Script & Multi-Device Sync

**Decision**:  
To prevent flash of unstyled content (FOUC):
1. **Local Cache**: The application caches `{ themeType, presetId, appearanceMode, customColors, customCss }` in `localStorage` under key `gameplane-theme-prefs`.
2. **Boot Script (`index.html`)**:
   - Reads `gameplane-theme-prefs`.
   - Sets `data-theme-preset` (`"pink"` | `"legacy"`), `data-theme-type`, and `.dark` / `.light` class synchronously before the first paint.
   - If `customCss` exists and URL does not contain `safe-theme=1`, injects the initial `<style id="gameplane-custom-css">`.
3. **Profile Reconciliation (`AppLayout.tsx`)**:
   - When `useMe()` loads, compare backend preferences with `localStorage`.
   - If backend preferences differ (e.g. user updated theme on another machine), update `localStorage` and smoothly re-apply the DOM attributes.
4. **Mutations**:
   - `useThemePreferences` hook provides `updatePreferences(...)`.
   - Immediately updates DOM and `localStorage` (optimistic UI), then calls `PUT /api/v1/users/me/preferences`.
   - On error, reverts local state and shows an error toast.

---

## Constitution Compliance Analysis

| Constitution Principle | Compliance Assessment |
|---|---|
| **I. E2E-Tested Delivery** | Playwright live specs in `web/e2e/specs/live/theme-customization.spec.ts` will test preset switching, persistence across reloads, custom colors, and custom CSS safe mode. |
| **II. Design-First** | The Theme Settings modal and appearance controls will be mapped and designed in `design.pen` via the Pencil MCP server before code implementation. |
| **III. Language & Ecosystem** | Strict TypeScript; Go handlers wrapped with `%w`; zero suppression directives (`//nolint`, `// @ts-ignore`). |
| **IV. Spec-Driven Development** | Follows spec -> plan -> research -> data-model -> contracts -> quickstart. |
| **V. Delegate to Workflows** | Tasks will be decomposed into independent subagent units. |
| **VI. CI Bears the Heavy Lifting** | Verified on GitHub Actions CI. |
