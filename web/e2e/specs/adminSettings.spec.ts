import { test, expect, type Page } from "@playwright/test";
import { LoginPage } from "../pages/LoginPage";

// AdminSettings page e2e. Covers the General section (the simplest
// useSectionForm consumer) submitting a PUT to /admin/config/{section}
// and the section navigation between General/Auth/Telemetry/Updates.
//
// We don't walk every section's form — useSectionForm is shared, so
// proving it on General is enough; the per-section render is exercised
// in component tests.

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

test.describe("admin settings page", () => {
  test.skip(
    process.env.KESTREL_E2E_TARGET === "live",
    "config edits in live mode persist to the API DB; mock mode is the safe path",
  );

  test.beforeEach(async ({ page }) => {
    await loginIfNeeded(page);
  });

  test("renders the page header and General section", async ({ page }) => {
    await page.goto("/admin/settings");
    await page.waitForLoadState("domcontentloaded");

    await expect(page.getByRole("heading", { name: /admin settings/i })).toBeVisible();
    // General tab is selected by default — its inputs are visible.
    await expect(page.getByText(/instance name/i)).toBeVisible();
  });

  test("saving General section sends PUT /admin/config/general", async ({ page }) => {
    await page.goto("/admin/settings");
    await page.waitForLoadState("domcontentloaded");

    // Update the Instance name input. It's the first textbox in the
    // General section per the form order.
    const instanceInput = page.locator("input").first();
    await instanceInput.fill("e2e-mock-instance");

    const saved = page.waitForRequest(
      (req) => /\/admin\/config\/general$/.test(req.url()) && req.method() === "PUT",
    );
    await page.getByRole("button", { name: /save changes/i }).first().click();
    const req = await saved;
    const body = req.postDataJSON() as { instanceName?: string };
    expect(body.instanceName).toBe("e2e-mock-instance");
  });

  test("navigates between sections", async ({ page }) => {
    await page.goto("/admin/settings");
    await page.waitForLoadState("domcontentloaded");

    // Click Telemetry — its toggle row appears.
    await page.getByRole("button", { name: /^telemetry$/i }).click();
    await expect(page.getByText(/send anonymous usage metrics/i)).toBeVisible();

    // Click About — version metadata appears.
    await page.getByRole("button", { name: /^about$/i }).click();
    await expect(page.getByText(/agpl-3\.0/i)).toBeVisible();
  });
});
