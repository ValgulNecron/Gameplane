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
  // Cookie-selected, not page.route: MSW's Service Worker (msw/browser)
  // answers /users/me itself in mock mode, so Playwright can't intercept
  // a request it has already resolved (see handlers.ts's e2e_me_role
  // branch). The cookie survives the page.goto() navigation below.
  await page.context().addCookies([
    { name: "e2e_me_role", value: role, url: "http://localhost:5173" },
  ]);
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
