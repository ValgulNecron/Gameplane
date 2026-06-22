import { test, expect, type Page } from "@playwright/test";
import { LoginPage } from "../pages/LoginPage";

// loginIfNeeded handles both run modes:
//   - mock: /users/me always returns a user, so visiting / stays on /servers.
//   - live: /users/me returns 401, AppLayout redirects to /login, we sign in.
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

test.describe("authenticated navigation", () => {
  test.beforeEach(async ({ page }) => {
    await loginIfNeeded(page);
  });

  test("dashboard at / renders the overview", async ({ page }) => {
    await page.goto("/");
    // The "/" route renders the dashboard overview (the server list lives
    // at /servers). We don't pin overview content — the AppLayout sidebar
    // always renders the "Modules" nav link once the SPA has hydrated,
    // which is a stable signal the dashboard mounted.
    await expect(page).toHaveURL(/\/(servers)?$/);
    await expect(page.getByRole("link", { name: /modules/i }).first()).toBeVisible();
  });

  test("modules page renders", async ({ page }) => {
    await page.goto("/modules");
    await expect(page).toHaveURL(/\/modules$/);
    // The catalog page always has a heading, regardless of how many
    // modules are installed.
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  });

  test("backups page renders", async ({ page }) => {
    await page.goto("/backups");
    await expect(page).toHaveURL(/\/backups$/);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  });

  test("admin audit log renders for admin role", async ({ page }) => {
    await page.goto("/admin/audit");
    // RequireRole sends non-admins back to /. If this URL holds, the
    // logged-in user is admin (which is the case in both mock and live
    // — the bootstrapped account is admin).
    await expect(page).toHaveURL(/\/admin\/audit$/);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  });
});
