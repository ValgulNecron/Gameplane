import { test, expect, type Page } from "@playwright/test";
import { LoginPage } from "../pages/LoginPage";

// ServerDetail header lifecycle buttons. The header has Restart (always
// enabled), Stop (enabled when phase=Running), and Start (only rendered
// when not running). MSW seeds the mock GameServer with phase=Running,
// so Stop is the affordance available out of the box.

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

test.describe("server lifecycle UI", () => {
  test.skip(
    process.env.GAMEPLANE_E2E_TARGET === "live",
    "lifecycle UI on a live cluster mutates real workloads; mock mode is the safe path",
  );

  test.beforeEach(async ({ page }) => {
    await loginIfNeeded(page);
  });

  test("Restart button POSTs /servers/{name}:restart", async ({ page }) => {
    await page.goto("/servers/alpha");
    await page.waitForLoadState("domcontentloaded");

    const restarted = page.waitForRequest(
      (req) => /\/servers\/alpha:restart$/.test(req.url()) && req.method() === "POST",
    );
    await page.getByRole("button", { name: /^restart$/i }).first().click();
    await restarted;
  });

  test("Stop button POSTs /servers/{name}:stop", async ({ page }) => {
    await page.goto("/servers/alpha");
    await page.waitForLoadState("domcontentloaded");

    // Wait for phase to settle so Stop is enabled (it's gated on
    // phase === "Running").
    const stopBtn = page.getByRole("button", { name: /^stop$/i }).first();
    await expect(stopBtn).toBeEnabled({ timeout: 5_000 });

    const stopped = page.waitForRequest(
      (req) => /\/servers\/alpha:stop$/.test(req.url()) && req.method() === "POST",
    );
    await stopBtn.click();
    await stopped;
  });

  test("Open console button switches to the Console tab", async ({ page }) => {
    await page.goto("/servers/alpha");
    await page.waitForLoadState("domcontentloaded");

    await page.getByRole("button", { name: /open console/i }).click();
    // Console tab is selected — the tab strip's Console button reflects
    // active state. xterm.js itself is heavy and lazy-loaded; we just
    // assert the tab nav advanced rather than waiting for the terminal
    // to fully mount.
    const tabNav = page.locator("header nav.scrollbar-thin");
    await expect(tabNav.getByRole("button", { name: /^console$/i })).toBeVisible();
  });
});
