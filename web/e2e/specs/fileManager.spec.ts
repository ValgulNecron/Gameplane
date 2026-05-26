import { test, expect, type Page } from "@playwright/test";
import { LoginPage } from "../pages/LoginPage";

// Files tab e2e. The tab proxies through the API to the agent's
// /files/{list,read,write,mkdir,delete}. MSW seeds the listing with
// "server.properties" and "world/"; this spec proves the wire-up
// without exercising the real agent.

async function loginIfNeeded(page: Page): Promise<void> {
  await page.goto("/");
  await page.waitForLoadState("domcontentloaded");
  if (new URL(page.url()).pathname.startsWith("/login")) {
    const login = new LoginPage(page);
    const username =
      process.env.ADMIN_USERNAME ?? process.env.KESTREL_E2E_ADMIN_USERNAME ?? "e2e-admin";
    const password =
      process.env.ADMIN_PASSWORD ?? process.env.KESTREL_E2E_ADMIN_PASSWORD ?? "any-non-empty";
    await login.login(username, password);
    await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 10_000 });
  }
}

test.describe("file manager tab", () => {
  test.skip(
    process.env.KESTREL_E2E_TARGET === "live",
    "file write/delete on a live agent mutates real volume contents; mock mode is the safe path",
  );

  test.beforeEach(async ({ page }) => {
    await loginIfNeeded(page);
  });

  test("renders the listing from /files/list", async ({ page }) => {
    await page.goto("/servers/alpha");
    await page.waitForLoadState("domcontentloaded");

    const tabNav = page.locator("header nav.scrollbar-thin");
    await tabNav.getByRole("button", { name: /^files$/i }).click();

    // Wait for the listing fetch to complete, then assert seeded entries
    // are visible. The MSW handler returns server.properties + world/.
    await expect(page.getByText("server.properties")).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText(/^world$/)).toBeVisible();
  });

  test("clicking a file fires a /files/read request", async ({ page }) => {
    await page.goto("/servers/alpha");
    await page.waitForLoadState("domcontentloaded");

    const tabNav = page.locator("header nav.scrollbar-thin");
    await tabNav.getByRole("button", { name: /^files$/i }).click();
    await expect(page.getByText("server.properties")).toBeVisible({ timeout: 10_000 });

    const readReq = page.waitForRequest(
      (req) => /\/files\/read/.test(req.url()) && req.method() === "GET",
    );
    await page.getByText("server.properties").click();
    await readReq;
  });
});
