import { test, expect, type Page } from "@playwright/test";
import { LoginPage } from "../pages/LoginPage";

// Backups page flows: list rendering, sub-tab switching, and the
// run-snapshot mutation. Restore + schedule modal flows are exercised
// by component-level tests; this spec proves the wiring (list + button
// + mutation) survives a real browser render.

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

test.describe("backups page", () => {
  test.skip(
    process.env.KESTREL_E2E_TARGET === "live",
    "live mode requires a real server and restic-server; mock mode is the deterministic path",
  );

  test.beforeEach(async ({ page }) => {
    await loginIfNeeded(page);
  });

  test("renders the Backups tab with rows from MSW", async ({ page }) => {
    await page.goto("/backups");
    await page.waitForLoadState("domcontentloaded");

    await expect(page.getByRole("heading", { name: /^backups$/i })).toBeVisible();
    // The MSW fixture seeds two backups (one Succeeded, one Failed) on
    // the server "alpha". The table renders one row per backup.
    await expect(page.getByText("alpha-2026-05-07")).toBeVisible();
    await expect(page.getByText("alpha-2026-05-06")).toBeVisible();
  });

  test("switches to Schedules and Restores tabs", async ({ page }) => {
    await page.goto("/backups");

    // Top-level tab strip uses native buttons.
    await page.getByRole("button", { name: /^schedules$/i }).click();
    await expect(page.getByText("alpha-daily")).toBeVisible();

    await page.getByRole("button", { name: /^restores$/i }).click();
    // Empty restores list shows a placeholder row.
    await expect(page.getByText(/no restores have been run/i)).toBeVisible();
  });

  test("run snapshot triggers a POST to /backups", async ({ page }) => {
    await page.goto("/backups");
    await page.waitForLoadState("domcontentloaded");

    // The page contains four native <select>s: the BackupFilters
    // server/phase pair, the Back up now server picker, and the
    // (sometimes-hidden) destination picker. Scope to the
    // "Back up now" card via its label text rather than relying on
    // .nth() index, which is brittle if the filters shape changes.
    const backupNow = page.getByText("Back up now", { exact: true }).locator("..");
    await backupNow.locator("select").first().selectOption("alpha");

    // The button only enables once both the server (we just set) AND
    // the backup destination (auto-set by useEffect once destinations
    // resolve from /backup-destinations) are populated. Wait for that
    // state rather than racing the effect.
    const runBtn = page.getByRole("button", { name: /run snapshot/i });
    await expect(runBtn).toBeEnabled({ timeout: 10_000 });

    const created = page.waitForRequest(
      (req) => req.url().endsWith("/backups") && req.method() === "POST",
    );
    await runBtn.click();
    const req = await created;
    const body = req.postDataJSON() as {
      kind?: string;
      spec?: { serverRef?: { name?: string } };
    };
    expect(body.kind).toBe("Backup");
    expect(body.spec?.serverRef?.name).toBe("alpha");
  });
});
