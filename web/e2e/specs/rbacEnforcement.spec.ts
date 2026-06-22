import { test, expect, type Page } from "@playwright/test";
import { LoginPage } from "../pages/LoginPage";

// RBAC enforcement at the UI layer. The dashboard reads the caller's
// role from /users/me and conditionally renders admin-only affordances
// (Invite user button, /admin routes, etc.). We override /users/me per
// test using page.route to assert each role's UI surface.

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

async function stubMeRole(page: Page, role: "admin" | "operator" | "viewer"): Promise<void> {
  // Intercept /users/me at the network layer so the SPA observes the
  // requested role from the moment it loads.
  await page.route(/\/users\/me$/, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        id: 1,
        username: `e2e-${role}`,
        displayName: `E2E ${role}`,
        email: "",
        role,
        provider: "local",
        createdAt: "2026-01-01T00:00:00Z",
      }),
    });
  });
}

test.describe("RBAC enforcement", () => {
  test.skip(
    process.env.GAMEPLANE_E2E_TARGET === "live",
    "RBAC must be tested against the real backend; live coverage is in the Go RBAC matrix",
  );

  test("admin sees Invite user on /users", async ({ page }) => {
    await stubMeRole(page, "admin");
    await loginIfNeeded(page);
    await page.goto("/users");
    await page.waitForLoadState("domcontentloaded");

    await expect(page.getByRole("button", { name: /invite user/i })).toBeVisible();
  });

  test("viewer does NOT see Invite user on /users", async ({ page }) => {
    await stubMeRole(page, "viewer");
    await loginIfNeeded(page);
    await page.goto("/users");
    await page.waitForLoadState("domcontentloaded");

    // Either the page redirects, or the button is hidden. Both are
    // acceptable — the contract is "viewer can't trigger invite".
    if (new URL(page.url()).pathname.startsWith("/users")) {
      await expect(page.getByRole("button", { name: /invite user/i })).toHaveCount(0);
    }
  });

  test("operator does NOT see Invite user on /users", async ({ page }) => {
    await stubMeRole(page, "operator");
    await loginIfNeeded(page);
    await page.goto("/users");
    await page.waitForLoadState("domcontentloaded");

    if (new URL(page.url()).pathname.startsWith("/users")) {
      await expect(page.getByRole("button", { name: /invite user/i })).toHaveCount(0);
    }
  });
});
