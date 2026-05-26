import { test, expect, type Page } from "@playwright/test";
import { LoginPage } from "../pages/LoginPage";

// Error-path UI behavior. The SPA must:
//   - Redirect to /login on a 401 from a protected route.
//   - Surface a usable error UI on a 500 (no blank screen).
//   - Make NO authenticated GETs before the user submits the login form
//     (CLAUDE.md §3 pre-auth privacy contract).

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

test.describe("error handling", () => {
  test.skip(
    process.env.KESTREL_E2E_TARGET === "live",
    "error injection requires controlled responses; mock mode is the deterministic path",
  );

  test("401 on a protected route bounces to /login", async ({ page }) => {
    // First, log in normally so we have a session.
    await loginIfNeeded(page);

    // Now arm the next /servers fetch to come back as 401.
    await page.route(/\/servers$/, async (route) => {
      await route.fulfill({ status: 401, body: "unauthorized\n" });
    });
    // Same for /users/me — the bounce-to-login logic typically lives
    // there. If the 401 surfaces on either, the SPA must redirect.
    await page.route(/\/users\/me$/, async (route) => {
      await route.fulfill({ status: 401, body: "unauthorized\n" });
    });

    await page.goto("/servers");
    // Either the page route handler or the global 401 hook fires.
    // We allow up to 10s for the redirect.
    await page.waitForURL((u) => u.pathname.startsWith("/login"), { timeout: 10_000 });
  });

  test("500 on /cluster shows an error UI rather than a blank screen", async ({ page }) => {
    await page.route(/\/cluster$/, async (route) => {
      await route.fulfill({ status: 500, body: "boom\n" });
    });
    await page.route(/\/cluster\/stats$/, async (route) => {
      await route.fulfill({ status: 500, body: "boom\n" });
    });
    await loginIfNeeded(page);
    await page.goto("/cluster");
    await page.waitForLoadState("domcontentloaded");

    // The page must render *something* — heading or any visible text —
    // not a blank page. This is a coarse but high-signal check; a
    // detailed error-card assertion lives in the Cluster component test.
    const visibleText = await page.locator("body").innerText();
    expect(visibleText.length).toBeGreaterThan(0);
  });

  test("pre-auth /login does not call /users or /cluster", async ({ page }) => {
    const forbidden: string[] = [];
    page.on("request", (req) => {
      const u = new URL(req.url());
      if (
        u.pathname === "/users" ||
        u.pathname === "/cluster" ||
        u.pathname === "/cluster/info" ||
        u.pathname === "/cluster/stats"
      ) {
        forbidden.push(`${req.method()} ${u.pathname}`);
      }
    });

    const login = new LoginPage(page);
    await login.goto();
    // Wait for any initial fetches to settle.
    await page.waitForLoadState("networkidle");

    expect(
      forbidden,
      "/login must not GET cluster/user data before the form is submitted",
    ).toEqual([]);
  });
});
