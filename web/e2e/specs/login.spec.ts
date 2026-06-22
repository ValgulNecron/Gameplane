import { test, expect } from "@playwright/test";
import { LoginPage } from "../pages/LoginPage";

// LoginPage E2E. Covers the user-visible login flow and the CLAUDE.md
// §3 pre-auth privacy contract: no cluster-internal data on /login.
//
// In live mode the suite-wide storageState pre-authenticates the
// browser context — login tests need an UN-authenticated start.
// Mock mode has no suite storageState, so we only override when
// targeting live to avoid disturbing the MSW service-worker setup
// (Playwright's storageState load can race with worker registration
// in chromium).
if (process.env.GAMEPLANE_E2E_TARGET === "live") {
  test.use({ storageState: { cookies: [], origins: [] } });
}

test.describe("login", () => {

  test("loads with brand and form, no cluster-internal data leaks", async ({ page }) => {
    const login = new LoginPage(page);
    await login.goto();

    // Static brand surface — proves we're on the login page.
    await expect(page.getByText(/sign in/i).first()).toBeVisible();
    await expect(login.username).toBeVisible();
    await expect(login.password).toBeVisible();

    // Privacy contract: a cluster name like "homelab" is exposed by
    // /cluster/info to authenticated users but must NEVER show up
    // pre-auth. Same for hostnames in the gameplane-system namespace.
    const body = (await page.locator("body").innerText()).toLowerCase();
    const forbidden = ["gameplane-system", "gameplane-games", "kind-gameplane-e2e", "homelab"];
    for (const f of forbidden) {
      expect(body, `pre-auth body must not contain ${f}`).not.toContain(f);
    }
    // Server-count strings ("5 servers", "3 online") must not appear
    // either — they're the canonical "metric leak" anti-example.
    expect(body).not.toMatch(/\d+\s+servers?\s+(online|running|active)/);
  });

  test("valid credentials redirect away from /login", async ({ page }) => {
    const login = new LoginPage(page);
    await login.goto();

    const username =
      process.env.ADMIN_USERNAME ?? process.env.GAMEPLANE_E2E_ADMIN_USERNAME ?? "e2e-admin";
    const password =
      process.env.ADMIN_PASSWORD ?? process.env.GAMEPLANE_E2E_ADMIN_PASSWORD ?? "any-non-empty";

    await login.login(username, password);

    // The Login page hard-redirects via location.assign("/") on success,
    // so the next URL is the root (which the router rewrites to
    // /servers in the dashboard layout).
    await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 10_000 });
    expect(new URL(page.url()).pathname).not.toMatch(/^\/login/);
  });

  test("invalid credentials surface a generic error", async ({ page }) => {
    const login = new LoginPage(page);
    await login.goto();

    // Empty creds in mock mode trip the 401 branch in handlers.ts;
    // in live mode they trip the same path on the real backend.
    await login.login("", "");

    await expect(login.error).toBeVisible();
    const text = (await login.error.textContent())?.toLowerCase() ?? "";

    // Privacy: "invalid credentials" is the only acceptable shape.
    // "user not found" / "wrong password" / "no such user" leak which
    // half of the tuple was wrong.
    expect(text).toMatch(/invalid credentials|network error/);
    expect(text).not.toContain("user not found");
    expect(text).not.toContain("no such user");
    expect(text).not.toContain("wrong password");
  });
});
