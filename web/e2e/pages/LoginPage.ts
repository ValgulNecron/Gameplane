import type { Locator, Page } from "@playwright/test";

// loginIfNeeded handles both run modes:
//   - mock: /users/me always returns a user, so visiting / stays in-app.
//   - live: /users/me returns 401, AppLayout redirects to /login, we sign in.
// Used by both shell and live specs to reduce duplication while preserving
// credential fallbacks and waitForURL guards.
//
// PRECONDITION: The caller must navigate to a page and wait for it to settle
// (page.waitForLoadState("domcontentloaded")) before calling this helper.
// The helper inspects the current URL and only logs in if already on /login;
// it does not navigate itself. Callers that need to navigate to a specific
// route must do so first (e.g., page.goto("/servers")), or those that need
// a default route must navigate to "/" (e.g., page.goto("/")).
export async function loginIfNeeded(page: Page): Promise<void> {
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

// Page Object for /login. Encapsulates locator details so spec files
// stay readable when the form structure shifts (and so a renamed
// placeholder doesn't break ten tests).
export class LoginPage {
  readonly page: Page;
  readonly username: Locator;
  readonly password: Locator;
  readonly submit: Locator;
  readonly error: Locator;

  constructor(page: Page) {
    this.page = page;
    this.username = page.getByRole("textbox", { name: /email or username/i });
    this.password = page.locator('input[name="password"]');
    this.submit = page.getByRole("button", { name: /sign in/i });
    this.error = page.getByRole("alert");
  }

  async goto(): Promise<void> {
    await this.page.goto("/login");
  }

  async login(username: string, password: string): Promise<void> {
    await this.username.fill(username);
    await this.password.fill(password);
    await this.submit.click();
  }
}
