import { test, expect, type Page, type Locator } from "@playwright/test";
import path from "path";
import { fileURLToPath } from "node:url";

// T137 (specs/014-heroui-web-rebuild/tasks.md): Slice 3 — Onboarding flow
// (Create Server wizard steps 1-5), Modules Catalog, and Backups
// (Index/Schedules/Restores + detail drawer/restore dialog/list item).
// Screenshot verification tests for the HeroUI rebuild, capturing the 12
// design frames listed in design-export/MANIFEST.md's "Incremental export
// 2026-09-05 — Slice 3 design wave" section at 1440px, for comparison
// against design-export/screenshots/<id>.png per
// specs/014-heroui-web-rebuild/contracts/screen-verification.md.
//
// Mirrors slice2a.spec.ts's structure (viewport, capture() helper,
// id-named PNGs, GAMEPLANE_SCREENSHOTS=1 gating via playwright.config.ts's
// grep/grepInvert on the @screenshots tag) and reuses the same
// "screenshots" MSW dataset (web/src/test/screenshotData.ts,
// buildScreenshotHandlers() in web/src/test/handlers.ts) so sample data
// (server names, phases, counts) matches the redrawn frames.
//
// One additive fixture change backs this spec: the minecraft-java entry in
// screenshotTemplates (web/src/test/screenshotData.ts) gained a
// spec.versions catalog, because no existing screenshot template declared
// one and the wizard's "Version" step (CreateServer.tsx's stepsFor()) only
// appears when a template has versions — see the comment at that fixture.

async function capture(page: Page, id: string): Promise<void> {
  const here = path.dirname(fileURLToPath(import.meta.url));
  const screenshotPath = path.join(here, `${id}.png`);
  await page.screenshot({ path: screenshotPath, fullPage: true });
}

async function captureLocator(locator: Locator, id: string): Promise<void> {
  const here = path.dirname(fileURLToPath(import.meta.url));
  const screenshotPath = path.join(here, `${id}.png`);
  await locator.screenshot({ path: screenshotPath });
}

// Selects the enriched screenshot dataset before the app's first fetch.
async function useScreenshotDataset(page: Page): Promise<void> {
  await page.addInitScript(() => {
    try {
      window.localStorage.setItem("gameplane-e2e-dataset", "screenshots");
    } catch {
      // localStorage unavailable (sandboxed) — falls back to default handlers.
    }
  });
}

async function clickTab(page: Page, name: string): Promise<void> {
  const tab = page.getByRole("tab", { name: new RegExp(`^${name}$`, "i") });
  await expect(tab).toBeVisible({ timeout: 10_000 });
  await tab.click();
}

test.describe("Slice 3: Create Server, Modules, Backups (Desktop — 1440x900) @screenshots", () => {
  test.use({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 2,
    colorScheme: "dark",
  });

  test.beforeEach(async ({ page }) => {
    await useScreenshotDataset(page);
  });

  test("W8idqY: Create Server — Step 1 (Template)", async ({ page }) => {
    await page.goto("/servers/new");
    await expect(page.getByText(/new game server/i)).toBeVisible({ timeout: 10_000 });
    // screenshotTemplates seeds 8 templates; the first is Minecraft Java Edition.
    await expect(
      page.getByRole("button", { name: /Minecraft Java Edition/i }),
    ).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "W8idqY");
  });

  test("nNL3E: Create Server — Step 2 Version", async ({ page }) => {
    await page.goto("/servers/new");
    await page.getByRole("button", { name: /Minecraft Java Edition/i }).click();
    await page.getByRole("button", { name: /continue/i }).click();
    // Only reachable because the minecraft-java fixture now declares
    // spec.versions (T137 additive fixture change) — stepsFor() inserts the
    // "version" step for a template with a version catalog.
    await expect(page.getByText(/choose a version/i)).toBeVisible({ timeout: 10_000 });
    await expect(page.getByRole("button", { name: /1\.21 \(Vanilla\)/i })).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "nNL3E");
  });

  test("vUqMl: Create Server — Step 3 Configure", async ({ page }) => {
    await page.goto("/servers/new");
    await page.getByRole("button", { name: /Minecraft Java Edition/i }).click();
    await page.getByRole("button", { name: /continue/i }).click();
    await page.getByRole("button", { name: /1\.21 \(Vanilla\)/i }).click();
    await page.getByRole("button", { name: /continue/i }).click();
    await expect(page.getByText(/server name/i)).toBeVisible({ timeout: 10_000 });
    await expect(page.getByPlaceholder(/mc-hardcore/i)).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "vUqMl");
  });

  test("f1Vga: Create Server — Step 4 Network", async ({ page }) => {
    await page.goto("/servers/new");
    await page.getByRole("button", { name: /Minecraft Java Edition/i }).click();
    await page.getByRole("button", { name: /continue/i }).click();
    await page.getByRole("button", { name: /1\.21 \(Vanilla\)/i }).click();
    await page.getByRole("button", { name: /continue/i }).click();
    await page.getByPlaceholder(/mc-hardcore/i).fill("e2e-screenshot-srv");
    await page.getByRole("button", { name: /continue/i }).click();
    await expect(page.getByText(/^expose$/i)).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "f1Vga");
  });

  test("UMJli: Create Server — Step 5 Review", async ({ page }) => {
    await page.goto("/servers/new");
    await page.getByRole("button", { name: /Minecraft Java Edition/i }).click();
    await page.getByRole("button", { name: /continue/i }).click();
    await page.getByRole("button", { name: /1\.21 \(Vanilla\)/i }).click();
    await page.getByRole("button", { name: /continue/i }).click();
    await page.getByPlaceholder(/mc-hardcore/i).fill("e2e-screenshot-srv");
    await page.getByRole("button", { name: /continue/i }).click();
    await page.getByRole("button", { name: /continue/i }).click();
    await expect(page.getByRole("button", { name: /create server/i })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByText("e2e-screenshot-srv")).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "UMJli");
  });

  test("kK8Ji: Modules Catalog", async ({ page }) => {
    await page.goto("/modules");
    await expect(page.getByRole("heading", { name: /^modules$/i })).toBeVisible({
      timeout: 10_000,
    });
    // The catalog fixture (buildScreenshotHandlers' /modules/catalog) seeds
    // an uninstalled Minecraft entry and an installed Valheim entry.
    const grid = page.locator('[data-testid="modules-grid"]');
    await expect(grid.getByText("Minecraft (Vanilla)", { exact: true })).toBeVisible({
      timeout: 10_000,
    });
    await expect(grid.getByText("Valheim", { exact: true })).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "kK8Ji");
  });

  test("DPrYX: Backups — Index", async ({ page }) => {
    await page.goto("/backups");
    await expect(page.getByRole("heading", { name: /^backups$/i })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByText("alpha-2026-05-07")).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText("alpha-2026-05-06")).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "DPrYX");
  });

  test("fK8Bi: Backups — Schedules", async ({ page }) => {
    await page.goto("/backups");
    await expect(page.getByRole("heading", { name: /^backups$/i })).toBeVisible({
      timeout: 10_000,
    });
    await clickTab(page, "Schedules");
    // Screenshot dataset's schedules (screenshotSchedules), not the
    // "alpha-daily" default-dataset fixture used by e2e/specs/backups.spec.ts.
    await expect(page.getByText("test-server-01-daily")).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "fK8Bi");
  });

  test("tTSdi: Backups — Restores", async ({ page }) => {
    await page.goto("/backups");
    await expect(page.getByRole("heading", { name: /^backups$/i })).toBeVisible({
      timeout: 10_000,
    });
    await clickTab(page, "Restores");
    await expect(page.getByText("restore-test-server-01-1")).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "tTSdi");
  });

  test("zhLZN: Backup Detail Drawer", async ({ page }) => {
    await page.goto("/backups");
    const nameCell = page.getByText("alpha-2026-05-07", { exact: true });
    await expect(nameCell).toBeVisible({ timeout: 10_000 });
    await nameCell.click();
    await expect(page.getByText(/backup details/i)).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "zhLZN");
  });

  test("E9EEv0: Restore Backup dialog", async ({ page }) => {
    await page.goto("/backups");
    const row = page.getByRole("row", { name: /alpha-2026-05-07/i });
    await expect(row).toBeVisible({ timeout: 10_000 });
    // alpha-2026-05-07 (default makeBackup()) is Succeeded with a
    // snapshotID, so BackupRow's "Restore" action is enabled on it.
    await row.getByRole("button", { name: /^restore$/i }).click();
    await expect(page.getByRole("dialog")).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText(/restore backup/i)).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "E9EEv0");
  });

  test("DMnEi: Backup List Item", async ({ page }) => {
    await page.goto("/backups");
    const row = page.getByRole("row", { name: /alpha-2026-05-07/i });
    await expect(row).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    // Component-level crop (not a full-page capture): DMnEi is the reusable
    // backup list item (BackupRow), not a routed screen — see
    // design-export/MANIFEST.md's "Components/Dialogs" table for this id.
    await captureLocator(row, "DMnEi");
  });
});
