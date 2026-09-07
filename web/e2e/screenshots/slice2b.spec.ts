import { test, expect, type Page } from "@playwright/test";
import path from "path";
import { fileURLToPath } from "node:url";

// T118 (specs/014-heroui-web-rebuild/tasks.md): Slice 2b — Mods/Modpacks/
// Backups tabs, every Settings sub-tab, and the backup detail drawer.
// Screenshot verification tests for the HeroUI rebuild, capturing the
// design frames listed in design-export/MANIFEST.md's "Incremental export
// 2026-09-05 — Slice 2b design wave" section at 1440px, for comparison
// against design-export/screenshots/<id>.png per
// specs/014-heroui-web-rebuild/contracts/screen-verification.md.
//
// Mirrors slice2a.spec.ts's structure (viewport, capture() helper,
// useScreenshotDataset, id-named PNGs) and is selected the same way — only
// when GAMEPLANE_SCREENSHOTS=1 (via playwright.config.ts's grep/grepInvert
// on the @screenshots tag).
//
// ID mapping to design-export (from design-export/MANIFEST.md's Slice 2b
// table). Five of the eleven Settings sub-sections have no design export at
// all (checked: `grep -in "Settings.*General\|Version\|Environment\|Access\|
// Danger" design-export/MANIFEST.md` — no hits beyond unrelated Admin
// Settings screens) — those five are captured under a descriptive slug
// instead of a fabricated id:
//   Mods tab          -> sZtDi  (Screen/Server Detail — Mods)
//   Modpacks tab      -> tY6RD  (Screen/Server Detail — Modpacks)
//   Backups tab       -> pssCT  (Screen/Server Detail — Backups)
//   Settings/General       -> settings-general (no design export)
//   Settings/Version       -> settings-version (no design export)
//   Settings/Resources     -> VfB0Y  (Settings · Resource Limits)
//   Settings/Networking    -> J5pjJ3 (Settings · Networking)
//   Settings/EnvVars       -> settings-envvars (no design export)
//   Settings/Lifecycle     -> i1bLR  (Settings · Restart policy — closest
//                              existing export; LifecycleSection covers
//                              restart behavior + idle sleep)
//   Settings/Backups       -> KaRFX  (Settings · Scheduled Backups)
//   Settings/NetworkCapture -> RodrS (Settings · Network Capture)
//   Settings/Placement     -> Y5cmvI (Settings · Placement)
//   Settings/Access        -> settings-access (no design export)
//   Settings/Danger        -> settings-danger (no design export)
//   Backup detail drawer (open) -> zhLZN (Slice 3's Restore Backup — Detail
//                              Drawer export; same component, BackupDetailDrawer)
//
// Two screenshot-dataset fixture gaps blocked this task and were closed
// additively (see their own comments in screenshotData.ts/handlers.ts):
// no template declared a version catalog (Settings/Version never
// rendered), no registry provider declared `modpacks` (Modpacks tab never
// rendered a provider), and the two screenshot-mode /backups fixture
// entries carried spec.serverRef "alpha" despite metadata.name reading
// "test-server-01-…" (BackupsTab filters by serverRef, so the per-server
// Backups tab always rendered empty).

async function capture(page: Page, id: string): Promise<void> {
  const here = path.dirname(fileURLToPath(import.meta.url));
  const screenshotPath = path.join(here, `${id}.png`);
  await page.screenshot({ path: screenshotPath, fullPage: true });
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

test.describe("Slice 2b: Mods/Modpacks/Backups + Settings (Desktop — 1440x900) @screenshots", () => {
  test.use({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 2,
    colorScheme: "dark",
  });

  test.beforeEach(async ({ page }) => {
    await useScreenshotDataset(page);
  });

  test("sZtDi: Server Detail — Mods", async ({ page }) => {
    // test-server-02 (valheim-default) declares capabilities.mods, which is
    // what makes the Mods tab visible (see screenshotData.ts).
    await page.goto("/servers/test-server-02");
    await expect(page.getByRole("heading", { name: "test-server-02" })).toBeVisible({
      timeout: 10_000,
    });
    await clickTab(page, "Mods");
    await expect(page.getByText("ValheimPlus.dll")).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "sZtDi");
  });

  test("tY6RD: Server Detail — Modpacks", async ({ page }) => {
    // Same template's registry.providers[].modpacks (added for this task)
    // is what makes the Modpacks tab visible.
    await page.goto("/servers/test-server-02");
    await clickTab(page, "Modpacks");
    await expect(page.getByText(/browse modpacks/i)).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "tY6RD");
  });

  test("pssCT: Server Detail — Backups", async ({ page }) => {
    await page.goto("/servers/test-server-01");
    await clickTab(page, "Backups");
    await expect(page.getByText("test-server-01-2026-05-07")).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "pssCT");
  });

  test("zhLZN: Backup detail drawer (open)", async ({ page }) => {
    await page.goto("/servers/test-server-01");
    await clickTab(page, "Backups");
    const row = page.getByText("test-server-01-2026-05-07");
    await expect(row).toBeVisible({ timeout: 10_000 });
    await row.click();
    await expect(page.getByRole("heading", { name: /backup details/i })).toBeVisible({
      timeout: 10_000,
    });
    await page.waitForTimeout(200);
    await capture(page, "zhLZN");
  });

  test("VfB0Y: Server Detail — Settings · Resources", async ({ page }) => {
    await page.goto("/servers/test-server-01");
    await clickTab(page, "Settings");
    await clickTab(page, "Resources");
    await expect(page.getByText("Memory (GiB)")).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "VfB0Y");
  });

  test("J5pjJ3: Server Detail — Settings · Networking", async ({ page }) => {
    await page.goto("/servers/test-server-01");
    await clickTab(page, "Settings");
    await clickTab(page, "Networking");
    await expect(page.getByText(/^provider$/i).first()).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "J5pjJ3");
  });

  test("i1bLR: Server Detail — Settings · Lifecycle", async ({ page }) => {
    await page.goto("/servers/test-server-01");
    await clickTab(page, "Settings");
    await clickTab(page, "Lifecycle");
    await expect(page.getByLabel(/enable idle auto-sleep/i)).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "i1bLR");
  });

  test("KaRFX: Server Detail — Settings · Scheduled backups", async ({ page }) => {
    await page.goto("/servers/test-server-01");
    await clickTab(page, "Settings");
    await clickTab(page, "Scheduled backups");
    await expect(page.getByLabel(/enable scheduled backups/i)).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "KaRFX");
  });

  test("RodrS: Server Detail — Settings · Network capture", async ({ page }) => {
    await page.goto("/servers/test-server-01");
    await clickTab(page, "Settings");
    await clickTab(page, "Network capture");
    await expect(page.getByLabel(/enable capture/i)).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "RodrS");
  });

  test("Y5cmvI: Server Detail — Settings · Placement", async ({ page }) => {
    await page.goto("/servers/test-server-01");
    await clickTab(page, "Settings");
    await clickTab(page, "Placement");
    // PlacementSection is lazy-loaded (Suspense) — wait for its Monaco-based
    // editors to actually mount rather than a fixed timeout.
    await expect(page.getByText("Tolerations")).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "Y5cmvI");
  });

  test("settings-general: Server Detail — Settings · General", async ({ page }) => {
    await page.goto("/servers/test-server-01");
    await clickTab(page, "Settings");
    // General is the default-selected section — no click needed, but
    // click it anyway so this test doesn't depend on that default.
    await clickTab(page, "General");
    await expect(page.getByPlaceholder(/long-standing survival realm/i)).toBeVisible({
      timeout: 10_000,
    });
    await page.waitForTimeout(200);
    await capture(page, "settings-general");
  });

  test("settings-version: Server Detail — Settings · Version", async ({ page }) => {
    // test-server-01's template (minecraft-java) declares a version
    // catalog (added for this task) — required for the Version tab to
    // appear at all (Settings.tsx filters it out otherwise).
    await page.goto("/servers/test-server-01");
    await clickTab(page, "Settings");
    await clickTab(page, "Version");
    await expect(page.getByText(/game version/i)).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "settings-version");
  });

  test("settings-envvars: Server Detail — Settings · Environment", async ({ page }) => {
    await page.goto("/servers/test-server-01");
    await clickTab(page, "Settings");
    await clickTab(page, "Environment");
    // test-server-01 has no spec.env fixture — EnvVarsSection renders its
    // empty state (the "Environment variables" table only mounts once
    // env.length > 0).
    await expect(page.getByText(/no environment variables/i)).toBeVisible({
      timeout: 10_000,
    });
    await page.waitForTimeout(200);
    await capture(page, "settings-envvars");
  });

  test("settings-access: Server Detail — Settings · RBAC & access", async ({ page }) => {
    await page.goto("/servers/test-server-01");
    await clickTab(page, "Settings");
    await clickTab(page, "RBAC & access");
    await expect(page.getByText(/^owner$/i)).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "settings-access");
  });

  test("settings-danger: Server Detail — Settings · Danger zone", async ({ page }) => {
    await page.goto("/servers/test-server-01");
    await clickTab(page, "Settings");
    await clickTab(page, "Danger zone");
    await expect(page.getByText("Delete server")).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "settings-danger");
  });
});
