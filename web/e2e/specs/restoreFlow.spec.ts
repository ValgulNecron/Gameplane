import { test, expect, type Page } from "@playwright/test";
import { LoginPage } from "../pages/LoginPage";

// Restore-flow e2e. The Backups page renders one row per backup with a
// "Restore" button on Succeeded backups. Clicking it opens the
// RestoreDialog (a Radix dialog) which submits POST /restores with
// {backupRef:{name}, serverRef:{name}}.

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

test.describe("restore from backup", () => {
  test.skip(
    process.env.KESTREL_E2E_TARGET === "live",
    "restoring on a live cluster mutates real volume contents; mock mode is the safe path",
  );

  test.beforeEach(async ({ page }) => {
    await loginIfNeeded(page);
  });

  test("opens RestoreDialog from a Succeeded backup row and submits POST /restores", async ({ page }) => {
    await page.goto("/backups");
    await page.waitForLoadState("domcontentloaded");

    // The MSW seed has alpha-2026-05-07 (Succeeded) and alpha-2026-05-06
    // (Failed). The Succeeded one's row carries an enabled Restore
    // button; the Failed one's button is disabled.
    const succeededRow = page
      .locator("tr", { has: page.getByText("alpha-2026-05-07") });
    await expect(succeededRow).toBeVisible();
    await succeededRow.getByRole("button", { name: /^restore$/i }).click();

    // Radix Dialog mounts at the document root.
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText(/restore from backup/i)).toBeVisible();

    // Pick the target server from the select. MSW seeds two servers,
    // alpha and beta — we restore alpha→alpha (the typical case).
    await dialog.locator("select").selectOption("alpha");

    const created = page.waitForRequest(
      (req) => /\/restores$/.test(req.url()) && req.method() === "POST",
    );
    await dialog.getByRole("button", { name: /^restore$/i }).click();
    const req = await created;
    const body = req.postDataJSON() as {
      spec?: { backupRef?: { name?: string }; serverRef?: { name?: string } };
    };
    expect(body.spec?.backupRef?.name).toBe("alpha-2026-05-07");
    expect(body.spec?.serverRef?.name).toBe("alpha");
  });

  test("Restore button is disabled on Failed backups", async ({ page }) => {
    await page.goto("/backups");
    await page.waitForLoadState("domcontentloaded");

    const failedRow = page
      .locator("tr", { has: page.getByText("alpha-2026-05-06") });
    await expect(failedRow).toBeVisible();
    await expect(failedRow.getByRole("button", { name: /^restore$/i })).toBeDisabled();
  });
});
