import { type Page } from "@playwright/test";
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

  await page.screenshot({ path: screenshotPath });
}
