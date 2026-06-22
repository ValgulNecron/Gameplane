import { test, expect, type Page } from "@playwright/test";
import { LoginPage } from "../pages/LoginPage";

// Server detail tabs. Mock mode walks each tab and asserts no
// uncaught console errors fire during navigation. Per-tab content
// checks are intentionally light — the unit/component tests already
// cover render output. This spec proves the tab routing and lazy
// imports survive a real browser.

async function loginIfNeeded(page: Page): Promise<void> {
  await page.goto("/");
  await page.waitForLoadState("domcontentloaded");
  if (new URL(page.url()).pathname.startsWith("/login")) {
    const login = new LoginPage(page);
    const username =
      process.env.ADMIN_USERNAME ?? process.env.GAMEPLANE_E2E_ADMIN_USERNAME ?? "e2e-admin";
    const password =
      process.env.ADMIN_PASSWORD ?? process.env.GAMEPLANE_E2E_ADMIN_PASSWORD ?? "any-non-empty";
    await login.login(username, password);
    await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 10_000 });
  }
}

test.describe("server detail tabs", () => {
  test.skip(
    process.env.GAMEPLANE_E2E_TARGET === "live",
    "tab navigation against live data depends on a running server; mock mode is the deterministic path",
  );

  test.beforeEach(async ({ page }) => {
    await loginIfNeeded(page);
  });

  test("renders header and switches between tabs without errors", async ({ page }) => {
    const errors: string[] = [];
    page.on("pageerror", (err) => errors.push(err.message));
    page.on("console", (msg) => {
      if (msg.type() === "error") errors.push(msg.text());
    });

    await page.goto("/servers/alpha");
    await page.waitForLoadState("domcontentloaded");

    // Header shows the server name.
    await expect(page.getByRole("heading", { name: "alpha" })).toBeVisible();

    // The header contains two <nav> elements: a breadcrumb and the
    // tab strip. The tab strip carries the `scrollbar-thin` class so
    // locator selection stays unambiguous.
    const tabNav = page.locator("header nav.scrollbar-thin");
    await expect(tabNav).toBeVisible();

    // Visit each tab in sequence. Console and Files lazy-load via
    // React.Suspense; allow time for the chunk to settle.
    const labels = ["Overview", "Console", "Logs", "Files", "Players", "Backups", "Settings"];
    for (const label of labels) {
      await tabNav.getByRole("button", { name: new RegExp(`^${label}$`) }).click();
      // Tab content swap doesn't change URL — just await DOM stability.
      await page.waitForTimeout(200);
    }

    expect(errors.filter((e) => !isExpectedDevWarning(e))).toEqual([]);
  });

  test("settings sub-tabs render in sequence", async ({ page }) => {
    const errors: string[] = [];
    page.on("pageerror", (err) => errors.push(err.message));

    await page.goto("/servers/alpha");
    await page.waitForLoadState("domcontentloaded");

    const tabNav = page.locator("header nav.scrollbar-thin");
    await tabNav.getByRole("button", { name: /^Settings$/ }).click();

    // Settings has an inner tab strip (TabBar) — sub-tabs differ in
    // styling but are also <button> elements. Match by their text and
    // require visibility before clicking.
    const subTabs = ["General", "Networking", "Resources", "Environment", "Lifecycle", "Access", "Danger"];
    for (const t of subTabs) {
      const btn = page.getByRole("button", { name: new RegExp(`^${t}$`, "i") }).first();
      // Some sub-tabs may not be present depending on settings module
      // shape; tolerate that gracefully — but if visible, clicking
      // must not throw.
      if (await btn.isVisible().catch(() => false)) {
        await btn.click();
        await page.waitForTimeout(150);
      }
    }

    expect(errors.filter((e) => !isExpectedDevWarning(e))).toEqual([]);
  });
});

// isExpectedDevWarning filters out noisy dev-mode warnings that aren't
// regressions: HMR notices, React strict-mode double-render notes, and
// WebSocket-handshake failures from the Console/Logs tabs (mock mode has
// no WS server). These are genuine environment artifacts of mock mode.
//
// Note: the xterm.js FitAddon "reading 'dimensions'" error used to be
// allow-listed here as a mock-mode artifact. It is no longer — the Console
// tab now guards fit() (useConsoleTerminal: deferred rAF fit, a disposed
// flag, an isConnected check, and a host-scoped ResizeObserver), so the
// crash must not fire. Walking the tabs is now a live regression guard for
// it; do not re-add it.
function isExpectedDevWarning(text: string): boolean {
  const lower = text.toLowerCase();
  return (
    lower.includes("[vite]") ||
    lower.includes("downloadable font") ||
    lower.includes("[mocks]") ||
    lower.includes("hydration") ||
    lower.includes("websocket connection to") ||
    lower.includes("ws://localhost")
  );
}
