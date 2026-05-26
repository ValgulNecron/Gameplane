import { test, expect, type Page } from "@playwright/test";
import { LoginPage } from "../pages/LoginPage";

// Modules page: catalog grid renders from /modules/catalog, and the
// install dialog drives a POST /modules through to the server.

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

test.describe("modules page", () => {
  test.skip(
    process.env.KESTREL_E2E_TARGET === "live",
    "live mode requires a populated module catalog; mock mode is the deterministic path",
  );

  test.beforeEach(async ({ page }) => {
    await loginIfNeeded(page);
  });

  test("renders the catalog grid", async ({ page }) => {
    await page.goto("/modules");
    await page.waitForLoadState("domcontentloaded");

    await expect(page.getByRole("heading", { name: /^modules$/i })).toBeVisible();
    const grid = page.locator('[data-testid="modules-grid"]');
    await expect(grid).toBeVisible();
    // MSW seeds two catalog entries: Minecraft (uninstalled) and
    // Valheim (installed). Match by exact display name to avoid
    // strict-mode collisions with "Minecraft" appearing in both the
    // title and the summary text.
    await expect(grid.getByText("Minecraft (Vanilla)", { exact: true })).toBeVisible();
    await expect(grid.getByText("Valheim", { exact: true })).toBeVisible();
  });

  test("install dialog submits POST /modules with source/version/name", async ({ page }) => {
    await page.goto("/modules");
    await page.waitForLoadState("domcontentloaded");

    // Only the (uninstalled) Minecraft card exposes an "Install" button.
    // Valheim is seeded as installed, so its action is "Uninstall" /
    // "Upgrade". `getByRole(name: /^install$/i)` therefore matches
    // exactly one button.
    await page.getByRole("button", { name: /^install$/i }).first().click();

    // Dialog opens with name pre-filled to entry.name.
    await expect(page.getByRole("dialog")).toBeVisible();
    const nameInput = page.getByRole("dialog").getByRole("textbox");
    await expect(nameInput).toHaveValue("minecraft-vanilla");

    // Submit and verify a POST /modules went out with the right body.
    const installed = page.waitForRequest(
      (req) => req.url().endsWith("/modules") && req.method() === "POST",
    );
    await page.getByRole("dialog").getByRole("button", { name: /^install$/i }).click();
    const req = await installed;
    const body = req.postDataJSON() as { source?: string; module?: string; name?: string };
    expect(body.module).toBe("minecraft-vanilla");
    expect(body.source).toBeTruthy();
    expect(body.name).toBeTruthy();
  });
});
