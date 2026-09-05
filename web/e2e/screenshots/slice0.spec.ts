import { test, expect, type Page } from "@playwright/test";
import path from "path";
import { fileURLToPath } from "node:url";

// Slice 0 Foundation: Screenshot verification tests for HeroUI component library atoms
// These are placeholder tests for the design export comparison workflow.
// Each test is tagged with @screenshots and will be excluded from normal runs,
// selected only when GAMEPLANE_SCREENSHOTS=1 (via playwright.config.ts grep/grepInvert).

// Configure desktop viewport for all tests in this file
test.use({
  viewport: { width: 1440, height: 900 },
  deviceScaleFactor: 2,
  colorScheme: "dark",
});

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

test.describe("Slice 0: HeroUI Foundation (Desktop) @screenshots", () => {
  test.skip(true, "slice 1 rebuilds this screen - N1GkB Login Default");

  test("N1GkB: Login — Default", async ({ page }) => {
    // Placeholder: slice 1 rebuilds the login screens
    // Route: /login
    // Fixture: default login state (no error, not SSO-only)
    await page.goto("/login");
    await page.waitForLoadState("domcontentloaded");
    await capture(page, "N1GkB");
    expect(true).toBe(true);
  });
});

test.describe("Slice 0: HeroUI Foundation (Desktop - Login variants) @screenshots", () => {
  test.skip(true, "slice 1 rebuilds this screen - jmoi3 Login Invalid Credentials");

  test("jmoi3: Login — Invalid Credentials", async ({ page }) => {
    // Placeholder: slice 1 rebuilds the login error state
    // Route: /login with error display
    // Fixture: login error message visible
    await page.goto("/login");
    await page.waitForLoadState("domcontentloaded");
    await capture(page, "jmoi3");
    expect(true).toBe(true);
  });
});

test.describe("Slice 0: HeroUI Foundation (Desktop - SSO variant) @screenshots", () => {
  test.skip(true, "slice 1 rebuilds this screen - ljdA5 Login SSO Only");

  test("ljdA5: Login — SSO Only", async ({ page }) => {
    // Placeholder: slice 1 rebuilds the SSO-only variant
    // Route: /login with SSO-only mode
    // Fixture: local login form hidden, SSO button visible
    await page.goto("/login");
    await page.waitForLoadState("domcontentloaded");
    await capture(page, "ljdA5");
    expect(true).toBe(true);
  });
});

test.describe("Slice 0: HeroUI Foundation (Mobile) @screenshots", () => {
  // Mobile tests use 390px viewport per screen-verification.md contract
  test.use({
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 2,
    colorScheme: "dark",
  });

  test.skip(true, "slice 1 rebuilds this screen - tooKB Servers Mobile");

  test("tooKB: Screen/Servers — Mobile", async ({ page }) => {
    // Placeholder: slice 1 rebuilds the servers list on mobile
    // Route: /servers at 390px viewport
    // Fixture: server list rendered for mobile layout
    await page.goto("/servers");
    await page.waitForLoadState("domcontentloaded");
    await capture(page, "tooKB");
    expect(true).toBe(true);
  });
});

test.describe("Slice 0: HeroUI Foundation (Mobile - Navigation) @screenshots", () => {
  // Mobile tests use 390px viewport per screen-verification.md contract
  test.use({
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 2,
    colorScheme: "dark",
  });

  test.skip(true, "slice 1 rebuilds this screen - SeizD Navigation Drawer Mobile");

  test("SeizD: Screen/Navigation Drawer — Mobile", async ({ page }) => {
    // Placeholder: slice 1 rebuilds the mobile navigation drawer
    // Route: /servers or similar with mobile sidebar opened
    // Fixture: mobile drawer/sidebar visible for 390px layout
    await page.goto("/servers");
    await page.waitForLoadState("domcontentloaded");
    await capture(page, "SeizD");
    expect(true).toBe(true);
  });
});
