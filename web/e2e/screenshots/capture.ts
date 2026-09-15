import { expect, type Locator, type Page } from "@playwright/test";
import path from "node:path";
import { fileURLToPath } from "node:url";
import fs from "node:fs";
import sharp from "sharp";

const here = path.dirname(fileURLToPath(import.meta.url));
const SCREENSHOTS_DIR = here;
const DESIGN_EXPORT_DIR = path.resolve(here, "../../../design-export/screenshots");

/**
 * Capture a screenshot matching the reference frame dimensions and save to web/e2e/screenshots/<id>.png
 *
 * Automatically sizes the viewport to match the reference design frame's dimensions (at 2x scale)
 * so scrollable tall pages (e.g. 1440x1300 in Pencil -> 2880x2600 PNG) and mobile viewports
 * render and capture with exact dimension alignment without scrollbar gutter artifacts.
 */
export async function capture(page: Page, id: string): Promise<void> {
  const screenshotPath = path.join(SCREENSHOTS_DIR, `${id}.png`);
  const refPath = path.join(DESIGN_EXPORT_DIR, `${id}.png`);

  if (fs.existsSync(refPath)) {
    const meta = await sharp(refPath).metadata();
    if (meta.width && meta.height) {
      // Pencil exports frames at 2x scale. Set viewport to match the reference frame.
      const targetWidth = Math.round(meta.width / 2);
      const targetHeight = Math.round(meta.height / 2);
      const current = page.viewportSize();
      if (!current || current.width !== targetWidth || current.height !== targetHeight) {
        await page.setViewportSize({ width: targetWidth, height: targetHeight });
        await page.waitForTimeout(200);
      }
    }
  }

  await page.evaluate(() => {
    const el = document.activeElement as HTMLElement | null;
    if (el && !el.closest('[role="dialog"],[role="listbox"],[role="menu"],[data-open="true"]')) el.blur();
  });
  await page.screenshot({ path: screenshotPath, animations: "disabled" });
}

/**
 * Capture a screenshot of a single element (dialog, drawer, badge, chip, ...) rather
 * than the full page, and save to web/e2e/screenshots/<id>.png.
 *
 * Use this instead of `capture()` when the reference design frame is a tight crop of
 * one component rather than a full viewport — comparing a small element crop against
 * a full-page capture otherwise reads as a near-total diff (mismatched canvas sizes),
 * not a real rendering difference.
 *
 * Waits for web fonts to finish loading before waiting for the target to be visible,
 * scrolls it into view, then screenshots just that element with animations disabled
 * and the text caret hidden so the capture is deterministic.
 */
export async function captureLocator(page: Page, id: string, locator: Locator): Promise<void> {
  const screenshotPath = path.join(SCREENSHOTS_DIR, `${id}.png`);

  await page.evaluate(() => document.fonts.ready.then(() => undefined));
  await expect(locator).toBeVisible();
  await locator.scrollIntoViewIfNeeded();
  await page.evaluate(() => {
    const el = document.activeElement as HTMLElement | null;
    if (el && !el.closest('[role="dialog"],[role="listbox"],[role="menu"],[data-open="true"]')) el.blur();
  });
  await locator.screenshot({ path: screenshotPath, animations: "disabled", caret: "hide" });
}
