# Quickstart: User Theme Customization Validation Guide

**Feature**: `016-user-theme-customization`  
**Date**: 2026-09-19  
**Status**: Ready for Verification  

---

## 1. Overview

This guide provides runnable end-to-end verification procedures to validate that:
1. The database migration correctly defaults existing users to `legacy` and new users to `pink`.
2. The user preferences API (`GET/PUT /api/v1/users/me/preferences`) persists and validates settings.
3. The dashboard UI switches between Pink and Legacy presets without reloading.
4. Custom colors and custom CSS apply dynamically and isolate to the logged-in user.
5. Safe Mode recovers broken custom stylesheets via `?safe-theme=1`.

---

## 2. Prerequisites

- Go 1.25+ installed.
- Node.js 20+ and pnpm/npm installed.
- Repository cloned and current on `016-user-theme-customization` branch.

---

## 3. Database Migration & API Validation

### 3.1 Run Database Migration Test

Verify that migration `010_user_theme_preferences.sql` executes and migrates pre-existing users:

```bash
cd /home/valgul/project/Gameplane-Sec/api
go test -v ./internal/db -run TestUserThemePreferencesMigration
```

**Expected Outcome**:
- `user_preferences` table is created.
- Pre-existing users in the test database hold `preset_id = 'legacy'`.
- Newly inserted users hold default `preset_id = 'pink'`.

### 3.2 Run User Handler Preferences Test

Test preferences API endpoint validation, serialization, and permissions:

```bash
cd /home/valgul/project/Gameplane-Sec/api
go test -v ./internal/handlers -run TestUserPreferences
```

**Expected Outcome**:
- `GET /users/me/preferences` returns the caller's preferences.
- `PUT /users/me/preferences` validates color hex codes, size limits, and disallows HTML tags in CSS.
- Returns `400 Bad Request` on invalid hex `#XYZ` or forbidden `<style>` tag.
- Returns `401 Unauthorized` for unauthenticated requests.

---

## 4. Frontend Component & Unit Validation

### 4.1 Theme Tokens & Preset Tests

Verify token values for both Pink and Legacy presets in light and dark modes:

```bash
cd /home/valgul/project/Gameplane-Sec/web
npm test -- src/__tests__/theme.test.tsx src/__tests__/themeBoot.test.ts
```

**Expected Outcome**:
- Light mode tokens assert `--accent` is `#DB2777` for Pink and `#EA580C` for Legacy.
- Dark mode tokens assert `--accent` is `#FF4FA3` for Pink and `#F97316` for Legacy.
- `themeBoot.test.ts` validates that `index.html` boot script reads `localStorage` and initializes `data-theme-preset`.

### 4.2 Theme Settings Modal Tests

Verify UI interactions in `ThemeSettingsModal.test.tsx`:

```bash
cd /home/valgul/project/Gameplane-Sec/web
npm test -- src/components/hero/ThemeSettingsModal.test.tsx
```

**Expected Outcome**:
- Modal renders tabs: Presets, Custom Colors, Custom CSS.
- Clicking "Legacy" updates DOM attribute `data-theme-preset="legacy"`.
- Clicking "Reset to Defaults" clears custom CSS and colors.

---

## 5. End-to-End Browser Flow Validation

### 5.1 Run Playwright Mock E2E

Run the automated UI test suite for theme customization:

```bash
cd /home/valgul/project/Gameplane-Sec/web
npx playwright test e2e/specs/theme-customization.spec.ts
```

### 5.2 Manual Browser Walkthrough

1. **Sign in as an existing user**:
   - Open browser to `http://localhost:5173/login`.
   - Verify login page renders with modern Pink branding.
   - Sign in with an account created prior to migration.
   - Verify dashboard shell immediately renders in **Legacy Theme** (orange buttons, classic dark background).

2. **Switch to Modern Pink**:
   - Click user avatar in TopBar -> Select **Theme & Appearance**.
   - Select **Modern Pink** -> Click **Save**.
   - Verify buttons shift to vibrant pink and backgrounds shift to deep violet-dark.
   - Refresh page; verify Modern Pink remains active.

3. **Configure Custom Colors**:
   - Reopen **Theme & Appearance** -> Switch to **Custom Colors** tab.
   - Choose an Emerald green accent (`#10B981`) -> Click **Save**.
   - Verify primary buttons and active navigation markers turn emerald green.

4. **Verify Custom CSS & Safe Mode**:
   - Open **Custom CSS** tab in Theme Settings.
   - Enter:
     ```css
     body { opacity: 0.1 !important; }
     ```
   - Click **Save**. Observe the screen dim drastically.
   - Append `?safe-theme=1` to the browser URL and press Enter.
   - Verify opacity restores to 100%, custom CSS is disabled, and the Safe Mode recovery banner is displayed.
   - Click "Open Appearance Settings" from the banner and click **Reset to Defaults**.
   - Custom styling is cleared and application returns to standard default preset.
