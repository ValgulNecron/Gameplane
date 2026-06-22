import { test, expect, type Page } from "@playwright/test";
import { LoginPage } from "../pages/LoginPage";

// Players tab e2e. Renders /players + /players/banned and posts to the
// kick / ban / unban endpoints when the matching action is taken.

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

test.describe("players tab", () => {
  test.skip(
    process.env.GAMEPLANE_E2E_TARGET === "live",
    "kick/ban actions on a live agent mutate real player state; mock mode is the safe path",
  );

  test.beforeEach(async ({ page }) => {
    await loginIfNeeded(page);
  });

  test("renders the online list from /players", async ({ page }) => {
    await page.goto("/servers/alpha");
    await page.waitForLoadState("domcontentloaded");

    const tabNav = page.locator("header nav.scrollbar-thin");
    await tabNav.getByRole("button", { name: /^players$/i }).click();

    // The MSW makePlayers() seed has at least one player. The tab
    // renders a list/table — wait for any content to mount before
    // asserting deeper.
    await page.waitForTimeout(500);
    // Any role=button under the players panel proves the action menu
    // rendered for at least one player. We don't assert a specific name
    // (mock factory may evolve).
    const actionButtons = page.getByRole("button", { name: /kick|ban|unban/i });
    await expect(actionButtons.first()).toBeVisible({ timeout: 10_000 });
  });

  test("Kick action POSTs to /players/kick", async ({ page }) => {
    await page.goto("/servers/alpha");
    await page.waitForLoadState("domcontentloaded");
    const tabNav = page.locator("header nav.scrollbar-thin");
    await tabNav.getByRole("button", { name: /^players$/i }).click();

    // Wait for the player list to load — the per-player Kick action is an
    // icon button whose accessible name comes from its title="Kick".
    const kickBtn = page.getByRole("button", { name: /^kick$/i }).first();
    await expect(kickBtn).toBeVisible({ timeout: 10_000 });

    const kicked = page.waitForRequest(
      (req) =>
        /\/servers\/alpha\/players\/kick$/.test(req.url()) && req.method() === "POST",
    );
    await kickBtn.click();
    // The Kick button opens an inline confirm panel (a plain div, not a
    // role=dialog). Its confirm button is the only button with the
    // visible text "Kick" — the trigger above is icon-only. Clicking it
    // fires the POST.
    await page.getByRole("button").filter({ hasText: /^Kick$/ }).click();
    await kicked;
  });
});
