import { test, expect, type Page } from "@playwright/test";
import path from "path";
import { fileURLToPath } from "node:url";

// Slice 1: Shell + Login Screens — Screenshot verification tests for HeroUI rebuild
// These tests capture the 6 design frames from slice 1 at precise viewports with MSW mocks.
// Each test is tagged with @screenshots and will be excluded from normal runs,
// selected only when GAMEPLANE_SCREENSHOTS=1 (via playwright.config.ts grep/grepInvert).

// NOTE: The CI web-e2e-mock job should upload screenshots captured by these tests
// to the GitHub Actions artifacts. If the workflow is not yet configured to do so,
// add a step to upload web/e2e/screenshots/*.png with `if: always()` to ensure
// screenshots are captured regardless of test pass/fail status.
// Example:
//   - name: upload design frame screenshots
//     if: always()
//     uses: actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a
//     with:
//       name: design-frame-screenshots
//       path: web/e2e/screenshots/*.png
//       retention-days: 30
//       if-no-files-found: ignore

/**
 * Capture a full-page screenshot and save it to web/e2e/screenshots/<id>.png
 * @param page The Playwright page object
 * @param id The design frame id for naming the screenshot
 */
async function capture(page: Page, id: string): Promise<void> {
  const here = path.dirname(fileURLToPath(import.meta.url));
  const screenshotPath = path.join(here, `${id}.png`);
  await page.screenshot({ path: screenshotPath, fullPage: true });
}

test.describe("Slice 1: Shell + Login (Desktop — 1440x900) @screenshots", () => {
  // Desktop viewport for all tests in this describe block
  test.use({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 2,
    colorScheme: "dark",
  });

  test("N1GkB: Login — Default", async ({ page }) => {
    // Default login form with both local and SSO providers visible
    // No error message, form is empty and ready for input
    await page.goto("/login");
    await page.waitForLoadState("domcontentloaded");

    // Verify the form is visible
    await expect(page.getByRole("textbox", { name: /username/i })).toBeVisible();
    await expect(page.locator('input[name="password"]')).toBeVisible();
    await expect(page.getByRole("button", { name: /sign in/i })).toBeVisible();

    // Capture the default login state
    await capture(page, "N1GkB");
    expect(true).toBe(true);
  });

  test("jmoi3: Login — Invalid Credentials", async ({ page }) => {
    // Login form with error message visible after failed login attempt
    // Error copy is neutral ("Invalid credentials" per CLAUDE.md §3)
    await page.goto("/login");
    await page.waitForLoadState("domcontentloaded");

    // Submit empty credentials to trigger 401 error
    await page.getByRole("button", { name: /sign in/i }).click();

    // Wait for error message to appear
    await expect(page.getByRole("alert")).toBeVisible({ timeout: 5000 });

    // Capture the error state
    await capture(page, "jmoi3");

    // Verify error message is present and generic
    const errorText = (await page.getByRole("alert").textContent())?.toLowerCase() ?? "";
    expect(errorText).toMatch(/invalid credentials|network error/);
    expect(true).toBe(true);
  });

  test("ljdA5: Login — SSO Only", async ({ page }) => {
    // Mock-mode MSW reads this flag (src/test/browser-msw.ts) and swaps in
    // buildSsoOnlyHandlers(), which reports no local-login provider so
    // Login.tsx renders its SSO-only branch (no username/password form).
    await page.addInitScript(() => {
      localStorage.setItem("gameplane-e2e-dataset", "sso-only");
    });

    await page.goto("/login");
    await page.waitForLoadState("domcontentloaded");

    // Wait for the SSO button to actually render before asserting/capturing —
    // domcontentloaded fires before React mounts and MSW resolves the
    // providers fetch, so screenshotting immediately after it races the
    // paint and captures a blank/black frame.
    const ssoButton = page.getByRole("button", { name: /continue with/i });
    await expect(ssoButton).toBeVisible();

    // The local login form must not render in this state.
    await expect(page.getByRole("textbox", { name: /username/i })).toHaveCount(0);
    await expect(page.locator('input[name="password"]')).toHaveCount(0);

    // Capture the SSO-only scenario.
    await capture(page, "ljdA5");
    expect(true).toBe(true);
  });
});

test.describe("Slice 1: Shell + App (Desktop — 1440x900) @screenshots", () => {
  // Desktop viewport for app loading / dashboard
  test.use({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 2,
    colorScheme: "dark",
  });

  test("N13Xud: App — Loading State", async ({ page }) => {
    // Authenticated user, app is loading (queries pending, showing skeleton UI)
    // This captures the AppShellSkeleton (AppLayout.tsx) that renders while
    // useMe() -> GET /users/me is pending.
    //
    // MSW resolves /users/me fast enough that a bare navigation + short
    // timeout races straight past the skeleton and into the loaded shell —
    // `nav[aria-label="Primary"]` only exists in the *loaded* Sidebar
    // (hero/Sidebar.tsx), never in AppShellSkeleton, so waiting on it was
    // itself waiting for loading to finish. That made this screenshot
    // pixel-identical to j24cXg (Dashboard — Admin View).
    //
    // Delay /users/me so the skeleton is actually on screen when we
    // capture it. MSW's browser worker answers via a registered Service
    // Worker, which Playwright's page.route()/CDP interception cannot see
    // or delay (src/test/handlers.ts documents the same limitation for
    // the 401 case and works around it with a cookie the handler itself
    // reads). So the delay is injected in-page instead: wrap window.fetch
    // before MSW's worker starts, and hold the already-resolved
    // /users/me response for a beat before handing it back to the caller.
    await page.addInitScript(() => {
      const originalFetch = window.fetch.bind(window);
      window.fetch = (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === "string" ? input : input.toString();
        if (/\/users\/me(\?|$)/.test(url)) {
          return originalFetch(input, init).then(
            (res) => new Promise<Response>((resolve) => setTimeout(() => resolve(res), 800)),
          );
        }
        return originalFetch(input, init);
      };
    });

    // Navigate to the app
    await page.goto("/");

    // Wait for the loading skeleton itself (AppShellSkeleton sets
    // aria-busy="true" aria-label="Loading" on its root), not the
    // post-load sidebar.
    await expect(page.locator('[aria-busy="true"][aria-label="Loading"]')).toBeVisible({
      timeout: 5000,
    });

    // Capture the app loading state
    await capture(page, "N13Xud");

    expect(true).toBe(true);
  });

  test("j24cXg: Dashboard — Admin View", async ({ page }) => {
    // Authenticated admin user, app fully loaded, showing empty dashboard
    // Shell (sidebar + topbar + breadcrumbs) is visible
    // Dashboard content is empty (loading/empty state per T041 scope narrowing)

    await page.goto("/");
    await page.waitForLoadState("networkidle");

    // Wait for the layout to fully load
    await expect(page.locator('nav[aria-label="Primary"]')).toBeVisible({ timeout: 5000 });

    // Verify dashboard is rendered
    await expect(page.locator("main")).toBeVisible();

    // Capture the full dashboard view
    await capture(page, "j24cXg");
    expect(true).toBe(true);
  });
});

test.describe("Slice 1: Shell + App (Mobile — 390x844) @screenshots", () => {
  // Mobile viewport for all tests in this describe block
  test.use({
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 2,
    colorScheme: "dark",
  });

  test("tooKB: Servers — Mobile", async ({ page }) => {
    // Mobile view of /servers route with the app shell visible
    // Sidebar is hidden (drawer is closed by default on mobile)
    // TopBar with hamburger menu is visible
    // Servers list is rendered responsively

    await page.goto("/servers");
    await page.waitForLoadState("networkidle");

    // Verify the topbar hamburger is present (mobile navigation trigger)
    await expect(page.getByRole("button", { name: /open navigation/i })).toBeVisible();

    // Verify main content is visible
    await expect(page.locator("main")).toBeVisible();

    // Capture the mobile servers view
    await capture(page, "tooKB");
    expect(true).toBe(true);
  });

  test("SeizD: Navigation Drawer — Mobile", async ({ page }) => {
    // Mobile view with the navigation drawer open
    // Hamburger is clicked, sidebar drawer slides out from the left
    // User menu and navigation items are visible in the drawer
    // Backdrop overlay is visible behind the drawer

    await page.goto("/servers");
    await page.waitForLoadState("networkidle");

    // Click the hamburger menu to open the drawer
    const hamburger = page.getByRole("button", { name: /open navigation/i });
    await hamburger.click();

    // Wait for the drawer to become visible. Its nav carries its own
    // accessible name ("Mobile navigation"), distinct from the fixed
    // sidebar's "Primary" (Sidebar.tsx) — a raw `nav[aria-label="Primary"]`
    // selector would instead resolve to the fixed sidebar, which stays
    // display:none (and therefore never visible) below the `lg` breakpoint.
    await expect(page.locator('nav[aria-label="Mobile navigation"]')).toBeVisible({ timeout: 2000 });

    // Verify drawer is open and visible (look for the close button or nav items)
    await expect(page.getByRole("button", { name: /close navigation/i })).toBeVisible();

    // Capture the mobile drawer state
    await capture(page, "SeizD");
    expect(true).toBe(true);
  });
});
