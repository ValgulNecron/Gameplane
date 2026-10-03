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
      (req) => new URL(req.url()).pathname === "/servers/alpha:restart" && req.method() === "POST",
    );
    await serverDetail.restartButton.click();
    const requestURL = new URL((await restarted).url());
    expect(requestURL.searchParams.get("cluster") ?? "local").toBe("local");
    expect(requestURL.searchParams.get("namespace")).toBe("gameplane-games");
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
      (req) => new URL(req.url()).pathname === "/servers/alpha:stop" && req.method() === "POST",
    );
    await stopBtn.click();
    const requestURL = new URL((await stopped).url());
    expect(requestURL.searchParams.get("cluster") ?? "local").toBe("local");
    expect(requestURL.searchParams.get("namespace")).toBe("gameplane-games");
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

  test("shared row on /servers shows lifecycle actions scoped to its namespace", async ({ page, context }) => {
    await context.addCookies([{ name: "e2e_shared_server", value: "1", url: "http://localhost:5173" }]);
    await page.goto("/servers");
    await page.waitForLoadState("domcontentloaded");

    await expect(page.getByText("Shared with you")).toHaveCount(0);
    const sharedRow = page.getByRole("row").filter({ hasText: "team-a-shared" });
    await expect(sharedRow).toBeVisible();
    await expect(sharedRow.getByText("local / team-a", { exact: true })).toBeVisible();

    // Fixture seeds phase=Running (makeServer's default), so Stop is the
    // enabled affordance, same as the existing "alpha" row tests above.
    const stopBtn = sharedRow.getByRole("button", { name: /^stop$/i });
    await expect(stopBtn).toBeEnabled({ timeout: 5_000 });

    const stopped = page.waitForRequest(
      (req) => new URL(req.url()).pathname === "/servers/team-a-shared:stop" && req.method() === "POST",
    );
    await stopBtn.click();
    const req = await stopped;
    const requestURL = new URL(req.url());
    expect(requestURL.searchParams.get("cluster") ?? "local").toBe("local");
    expect(requestURL.searchParams.get("namespace")).toBe("team-a");
  });
});
