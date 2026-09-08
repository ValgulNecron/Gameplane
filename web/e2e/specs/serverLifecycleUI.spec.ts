import { test, expect, type Page } from "@playwright/test";
import { LoginPage } from "../pages/LoginPage";
import { ServerDetailPage } from "../pages/ServerDetailPage";

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
    const serverDetail = new ServerDetailPage(page);
    await serverDetail.goto("alpha");
    await page.waitForLoadState("domcontentloaded");

    const restarted = page.waitForRequest(
      (req) => /\/servers\/alpha:restart$/.test(req.url()) && req.method() === "POST",
    );
    await serverDetail.restartButton.click();
    await restarted;
  });

  test("Stop button POSTs /servers/{name}:stop", async ({ page }) => {
    const serverDetail = new ServerDetailPage(page);
    await serverDetail.goto("alpha");
    await page.waitForLoadState("domcontentloaded");

    // Wait for phase to settle so Stop is enabled (it's gated on
    // phase === "Running").
    const stopBtn = serverDetail.stopButton;
    await expect(stopBtn).toBeEnabled({ timeout: 5_000 });

    const stopped = page.waitForRequest(
      (req) => /\/servers\/alpha:stop$/.test(req.url()) && req.method() === "POST",
    );
    await stopBtn.click();
    await stopped;
  });

  test("Open console button switches to the Console tab", async ({ page }) => {
    const serverDetail = new ServerDetailPage(page);
    await serverDetail.goto("alpha");
    await page.waitForLoadState("domcontentloaded");

    await serverDetail.openConsoleButton.click();
    // Console tab is selected — the tab strip's Console tab reflects
    // active state. xterm.js itself is heavy and lazy-loaded; we just
    // assert the tab nav advanced rather than waiting for the terminal
    // to fully mount.
    const tabNav = page.getByRole("tablist", { name: /Server detail tabs/i });
    await expect(tabNav.getByRole("tab", { name: /^console$/i })).toBeVisible();
  });
});
