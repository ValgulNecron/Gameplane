import type { Locator, Page } from "@playwright/test";

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
    this.error = page.locator(".text-danger");
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
