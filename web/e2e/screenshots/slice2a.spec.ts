import { test, expect, type Page } from "@playwright/test";
import path from "path";
import { fileURLToPath } from "node:url";

// T086 (specs/014-heroui-web-rebuild/tasks.md): Slice 2a — Servers list +
// Server Detail core tabs. Screenshot verification tests for the HeroUI
// rebuild, capturing the design frames listed in
// specs/014-heroui-web-rebuild/contracts/component-map.md /
// design-export/MANIFEST.md's "Incremental export — Slice 2a" section at
// 1440px, for comparison against design-export/screenshots/<id>.png per
// contracts/screen-verification.md.
//
// Mirrors slice1.spec.ts's structure (viewport, capture() helper, id-named
// PNGs) and is selected the same way — only when GAMEPLANE_SCREENSHOTS=1
// (via playwright.config.ts's grep/grepInvert on the @screenshots tag).
//
// The five Overview state variants (idle armed / asleep / never sleeps /
// PVC provisioning failed) and the running baseline need named, stateful
// GameServer fixtures — screenshotServers (web/src/test/screenshotData.ts)
// is the enriched dataset the mock worker serves when
// localStorage["gameplane-e2e-dataset"] === "screenshots" (see
// browser-msw.ts). Three of those fixtures (test-server-06/07/08) were
// added by this task — additively — because no existing fixture covered
// idle-armed, never-sleeps, or PVC-provisioning-failed.

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

test.describe("Slice 2a: Servers + core tabs (Desktop — 1440x900) @screenshots", () => {
  test.use({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 2,
    colorScheme: "dark",
  });

  test.beforeEach(async ({ page }) => {
    await useScreenshotDataset(page);
  });

  test("F9pUrx: Servers — list", async ({ page }) => {
    await page.goto("/servers");
    await expect(page.getByRole("heading", { name: /^servers$/i })).toBeVisible();
    // The enriched dataset's 8 servers render as table rows.
    await expect(page.getByRole("link", { name: "test-server-01" })).toBeVisible({
      timeout: 10_000,
    });
    await page.waitForTimeout(200);
    await capture(page, "F9pUrx");
  });

  test("EZFW0: Server Detail — Overview (running)", async ({ page }) => {
    await page.goto("/servers/test-server-01");
    await expect(page.getByRole("heading", { name: "test-server-01" })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByText("Connection")).toBeVisible();
    await expect(page.getByText("Recent events")).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "EZFW0");
  });

  test("Hy9r0: Server Detail — Overview (Idle armed)", async ({ page }) => {
    await page.goto("/servers/test-server-06");
    await expect(page.getByRole("heading", { name: "test-server-06" })).toBeVisible({
      timeout: 10_000,
    });
    // ServerSleepCard's "counting down" state (spec.idle.enabled, status.idle.emptySince set).
    await expect(page.getByText("Counting down")).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText("Empty since")).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "Hy9r0");
  });

  test("TE2jI: Server Detail — Overview (Asleep)", async ({ page }) => {
    await page.goto("/servers/test-server-05");
    await expect(page.getByRole("heading", { name: "test-server-05" })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByText("Asleep", { exact: true }).first()).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText("Asleep since")).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "TE2jI");
  });

  test("IzuY2: Server Detail — Overview (Never sleeps)", async ({ page }) => {
    await page.goto("/servers/test-server-07");
    await expect(page.getByRole("heading", { name: "test-server-07" })).toBeVisible({
      timeout: 10_000,
    });
    // ServerSleepCard's neverSleeps warning banner.
    await expect(page.getByText("Will never sleep")).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "IzuY2");
  });

  test("o4LH8W: Server Detail — Overview (PVC Provisioning Failed)", async ({ page }) => {
    await page.goto("/servers/test-server-08");
    await expect(page.getByRole("heading", { name: "test-server-08" })).toBeVisible({
      timeout: 10_000,
    });
    await clickTab(page, "Overview");
    // Overview.tsx's provisioning-failure warning banner (matches condition
    // reason "PVCProvisioningFailed" against /Provisioning|Storage|PVC/).
    await expect(page.getByText(/waiting on storage/i)).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText(/PVCProvisioningFailed/)).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "o4LH8W");
  });

  test("P08Uw: Server Detail — Events", async ({ page }) => {
    await page.goto("/servers/test-server-01");
    await clickTab(page, "Events");
    await expect(page.getByText(/pulled|scheduled|started/i).first()).toBeVisible({
      timeout: 10_000,
    });
    await page.waitForTimeout(200);
    await capture(page, "P08Uw");
  });

  test("Xn5ns: Server Detail — Console", async ({ page }) => {
    await page.goto("/servers/test-server-01");
    await clickTab(page, "Console");
    // Mocked RCON/PTY WebSocket stream (screenshotConsoleOutput).
    await expect(page.getByText("joined the game").first()).toBeVisible({ timeout: 15_000 });
    await page.waitForTimeout(200);
    await capture(page, "Xn5ns");
  });

  test("kPmoo: Server Detail — Logs", async ({ page }) => {
    await page.goto("/servers/test-server-01");
    await clickTab(page, "Logs");
    // Mocked pod-log WebSocket stream (screenshotLogLines).
    await expect(page.getByText(/players online/i).first()).toBeVisible({ timeout: 15_000 });
    await page.waitForTimeout(200);
    await capture(page, "kPmoo");
  });

  test("FtdkI: Server Detail — Logs (Failed)", async ({ page }) => {
    // test-server-04 (phase Failed, no startedAt) — Logs is where the
    // ServerDetail auto-tab lands a server that never reached Running,
    // and the phase/warning header context differs from the healthy kPmoo capture.
    await page.goto("/servers/test-server-04");
    await expect(page.getByRole("heading", { name: "test-server-04" })).toBeVisible({
      timeout: 10_000,
    });
    await clickTab(page, "Logs");
    await page.waitForTimeout(500);
    await capture(page, "FtdkI");
  });

  test("Burtr: Server Detail — Files", async ({ page }) => {
    await page.goto("/servers/test-server-01");
    await clickTab(page, "Files");
    await expect(page.getByText("server.properties")).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "Burtr");
  });

  test("dPP50: Server Detail — Players", async ({ page }) => {
    await page.goto("/servers/test-server-01");
    await clickTab(page, "Players");
    // makePlayers() fixture's roster renders once the tab's query resolves.
    await expect(page.getByRole("heading", { name: /online/i })).toBeVisible({
      timeout: 10_000,
    });
    await page.waitForTimeout(200);
    await capture(page, "dPP50");
  });
});
