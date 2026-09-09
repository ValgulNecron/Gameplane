#!/usr/bin/env node

/**
 * Visual regression comparison script for Gameplane dashboard.
 *
 * Compares Playwright browser captures (web/e2e/screenshots/*.png) against
 * reference baseline screenshots exported from Pencil (design-export/screenshots/*.png).
 *
 * Key features:
 * - Dimension normalization via sharp padding (anchored at top-left, no distorting stretch)
 * - Subpixel & anti-aliasing tolerance via pixelmatch ({ threshold: 0.15, includeAA: false })
 * - Generation of visual diff highlighting (magenta) and 3-panel composite previews
 * - Configurable global & per-screen diff factor thresholds
 * - Output to console, JSON summary, and GitHub Step Summary ($GITHUB_STEP_SUMMARY)
 */

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import process from 'node:process';
import { Buffer } from 'node:buffer';
import sharp from 'sharp';
import pixelmatch from 'pixelmatch';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const REPO_ROOT = path.resolve(__dirname, '../..');
const WEB_ROOT = path.resolve(__dirname, '..');

// Default configurations
const DEFAULT_REF_DIR = path.join(REPO_ROOT, 'design-export/screenshots');
const DEFAULT_CURR_DIR = path.join(WEB_ROOT, 'e2e/screenshots');
const DEFAULT_OUT_DIR = path.join(WEB_ROOT, 'test-results/visual-diff');
const DEFAULT_MAX_DIFF_FACTOR = 0.04; // 4.0% maximum allowed difference
const DEFAULT_PIXEL_THRESHOLD = 0.15; // Color delta sensitivity (0 to 1)

// Per-screen threshold overrides where text density, terminal streams, or
// specific layouts have justified rendering variance.
const SCREEN_THRESHOLD_OVERRIDES = {
  // Console/Logs have high text density and live terminal streaming
  Xn5ns: 0.06, // Server Detail — Console
  kPmoo: 0.06, // Server Detail — Logs
  FtdkI: 0.06, // Server Detail — Logs (Failed)
};

// Parse command line options: --key=value or --flag
function parseArgs() {
  const args = process.argv.slice(2);
  const options = {
    refDir: DEFAULT_REF_DIR,
    currDir: DEFAULT_CURR_DIR,
    outDir: DEFAULT_OUT_DIR,
    maxDiffFactor: DEFAULT_MAX_DIFF_FACTOR,
    pixelThreshold: DEFAULT_PIXEL_THRESHOLD,
    allowMissing: false,
  };

  for (const arg of args) {
    if (arg.startsWith('--ref-dir=')) {
      options.refDir = path.resolve(process.cwd(), arg.split('=')[1]);
    } else if (arg.startsWith('--curr-dir=')) {
      options.currDir = path.resolve(process.cwd(), arg.split('=')[1]);
    } else if (arg.startsWith('--out-dir=')) {
      options.outDir = path.resolve(process.cwd(), arg.split('=')[1]);
    } else if (arg.startsWith('--threshold=')) {
      options.maxDiffFactor = parseFloat(arg.split('=')[1]);
    } else if (arg.startsWith('--pixel-threshold=')) {
      options.pixelThreshold = parseFloat(arg.split('=')[1]);
    } else if (arg === '--allow-missing') {
      options.allowMissing = true;
    }
  }

  // Environment variable overrides
  if (process.env.VISUAL_DIFF_THRESHOLD) {
    const envVal = parseFloat(process.env.VISUAL_DIFF_THRESHOLD);
    if (!isNaN(envVal)) options.maxDiffFactor = envVal;
  }

  return options;
}

/**
 * Normalizes an image to target dimensions by padding at bottom and right (top-left aligned)
 * and returns raw RGBA pixel buffer.
 */
async function loadAndPadImage(filePath, targetWidth, targetHeight) {
  const meta = await sharp(filePath).metadata();
  const padRight = Math.max(0, targetWidth - (meta.width || 0));
  const padBottom = Math.max(0, targetHeight - (meta.height || 0));

  let pipeline = sharp(filePath).ensureAlpha();
  if (padRight > 0 || padBottom > 0) {
    pipeline = pipeline.extend({
      top: 0,
      left: 0,
      right: padRight,
      bottom: padBottom,
      background: { r: 0, g: 0, b: 0, alpha: 0 },
    });
  }

  const { data, info } = await pipeline.raw().toBuffer({ resolveWithObject: true });
  return { data, width: info.width, height: info.height };
}

async function run() {
  const options = parseArgs();

  console.log('\n======================================================');
  console.log('   Gameplane Visual Regression Comparison Engine');
  console.log('======================================================');
  console.log(`Reference Directory : ${path.relative(REPO_ROOT, options.refDir)}`);
  console.log(`Current Directory   : ${path.relative(REPO_ROOT, options.currDir)}`);
  console.log(`Output Directory    : ${path.relative(REPO_ROOT, options.outDir)}`);
  console.log(`Default Threshold   : ${(options.maxDiffFactor * 100).toFixed(2)}%`);
  console.log(`Pixel Sensitivity   : ${options.pixelThreshold}`);
  console.log('------------------------------------------------------\n');

  if (!fs.existsSync(options.currDir)) {
    console.error(`Error: Current screenshots directory not found: ${options.currDir}`);
    console.error('Did you run `npm run screenshots` first?');
    process.exit(1);
  }

  if (!fs.existsSync(options.refDir)) {
    console.error(`Error: Reference screenshots directory not found: ${options.refDir}`);
    process.exit(1);
  }

  fs.mkdirSync(options.outDir, { recursive: true });

  // Discover captured screens in currDir
  const currFiles = fs
    .readdirSync(options.currDir)
    .filter((file) => file.endsWith('.png') && !file.includes('-diff') && !file.includes('-composite'));

  if (currFiles.length === 0) {
    console.warn(`Warning: No PNG screenshots found in ${options.currDir}`);
    process.exit(options.allowMissing ? 0 : 1);
  }

  const results = [];
  let hasFailure = false;

  for (const file of currFiles) {
    const screenId = path.basename(file, '.png');
    const currPath = path.join(options.currDir, file);
    const refPath = path.join(options.refDir, `${screenId}.png`);

    if (!fs.existsSync(refPath)) {
      console.warn(`⚠️  [${screenId}] Reference image missing at ${refPath}`);
      if (!options.allowMissing) {
        hasFailure = true;
        results.push({
          id: screenId,
          status: 'MISSING_REF',
          diffPercentage: 'N/A',
          threshold: 'N/A',
          totalPixels: 0,
          diffPixels: 0,
          dimensions: 'N/A',
        });
      }
      continue;
    }

    // Read metadata of both to determine maximum canvas dimensions
    const refMeta = await sharp(refPath).metadata();
    const currMeta = await sharp(currPath).metadata();

    const maxWidth = Math.max(refMeta.width || 0, currMeta.width || 0);
    const maxHeight = Math.max(refMeta.height || 0, currMeta.height || 0);

    // Standardize both onto maxWidth × maxHeight raw RGBA buffers
    const paddedRef = await loadAndPadImage(refPath, maxWidth, maxHeight);
    const paddedCurr = await loadAndPadImage(currPath, maxWidth, maxHeight);

    const diffData = Buffer.alloc(maxWidth * maxHeight * 4);

    // Run pixelmatch
    const numDiffPixels = pixelmatch(
      paddedRef.data,
      paddedCurr.data,
      diffData,
      maxWidth,
      maxHeight,
      {
        threshold: options.pixelThreshold,
        includeAA: false, // Disregard anti-aliasing subpixel differences
        diffColor: [255, 0, 100], // Highlight diffs in magenta
        aaColor: [255, 255, 0],   // AA pixels in yellow (if detected)
      }
    );

    const totalPixels = maxWidth * maxHeight;
    const diffRatio = numDiffPixels / totalPixels;
    const diffPercentage = (diffRatio * 100).toFixed(2);

    const allowedThreshold = SCREEN_THRESHOLD_OVERRIDES[screenId] ?? options.maxDiffFactor;
    const allowedPercentage = (allowedThreshold * 100).toFixed(2);
    const passed = diffRatio <= allowedThreshold;

    if (!passed) {
      hasFailure = true;
    }

    // Save individual diff image
    const diffFileName = `${screenId}-diff.png`;
    const diffFilePath = path.join(options.outDir, diffFileName);
    const diffPngBuffer = await sharp(diffData, {
      raw: { width: maxWidth, height: maxHeight, channels: 4 },
    })
      .png()
      .toBuffer();
    await fs.promises.writeFile(diffFilePath, diffPngBuffer);

    // Create 3-panel composite preview: [ Reference | Current Browser | Diff ]
    const compositeFileName = `${screenId}-composite.png`;
    const compositeFilePath = path.join(options.outDir, compositeFileName);

    const paddedRefPng = await sharp(paddedRef.data, {
      raw: { width: maxWidth, height: maxHeight, channels: 4 },
    })
      .png()
      .toBuffer();

    const paddedCurrPng = await sharp(paddedCurr.data, {
      raw: { width: maxWidth, height: maxHeight, channels: 4 },
    })
      .png()
      .toBuffer();

    await sharp({
      create: {
        width: maxWidth * 3,
        height: maxHeight,
        channels: 4,
        background: { r: 18, g: 18, b: 20, alpha: 1 },
      },
    })
      .composite([
        { input: paddedRefPng, left: 0, top: 0 },
        { input: paddedCurrPng, left: maxWidth, top: 0 },
        { input: diffPngBuffer, left: maxWidth * 2, top: 0 },
      ])
      .png()
      .toFile(compositeFilePath);

    const statusStr = passed ? '✅ PASS' : '❌ FAIL';
    console.log(
      `${statusStr} [${screenId.padEnd(8)}] Diff: ${diffPercentage.padStart(6)}% (max: ${allowedPercentage}%) | ` +
      `Pixels: ${numDiffPixels.toLocaleString().padStart(9)} / ${totalPixels.toLocaleString()} | Size: ${maxWidth}x${maxHeight}`
    );

    results.push({
      id: screenId,
      status: passed ? 'PASS' : 'FAIL',
      diffRatio,
      diffPercentage,
      threshold: allowedPercentage,
      totalPixels,
      diffPixels: numDiffPixels,
      dimensions: `${maxWidth}x${maxHeight}`,
      diffImage: path.relative(WEB_ROOT, diffFilePath),
      compositeImage: path.relative(WEB_ROOT, compositeFilePath),
    });
  }

  // Write summary JSON
  const summaryJsonPath = path.join(options.outDir, 'summary.json');
  fs.writeFileSync(
    summaryJsonPath,
    JSON.stringify(
      {
        totalScreens: results.length,
        passed: results.filter((r) => r.status === 'PASS').length,
        failed: results.filter((r) => r.status === 'FAIL').length,
        timestamp: new Date().toISOString(),
        results,
      },
      null,
      2
    )
  );

  // Write GitHub Step Summary if environment variable exists
  if (process.env.GITHUB_STEP_SUMMARY) {
    try {
      let markdown = '## 🎨 Visual Regression Test Summary\n\n';
      markdown += `Total screens evaluated: **${results.length}** | `;
      markdown += `Passed: **${results.filter((r) => r.status === 'PASS').length}** | `;
      markdown += `Failed: **${results.filter((r) => r.status === 'FAIL').length}**\n\n`;
      markdown += '| Screen ID | Status | Diff % | Max Allowed | Diff Pixels | Canvas Size |\n';
      markdown += '| :--- | :---: | :---: | :---: | :---: | :---: |\n';

      for (const r of results) {
        const icon = r.status === 'PASS' ? '✅' : '❌';
        markdown += `| \`${r.id}\` | ${icon} ${r.status} | **${r.diffPercentage}%** | ${r.threshold}% | ${r.diffPixels.toLocaleString()} | ${r.dimensions} |\n`;
      }

      markdown += '\n> Diff highlighting: Baseline design (left) vs Current browser (center) vs Diff highlight (right, magenta).\n';
      fs.appendFileSync(process.env.GITHUB_STEP_SUMMARY, markdown);
    } catch (err) {
      console.error('Failed to append to GITHUB_STEP_SUMMARY:', err);
    }
  }

  console.log('\n------------------------------------------------------');
  const passCount = results.filter((r) => r.status === 'PASS').length;
  const failCount = results.filter((r) => r.status === 'FAIL').length;
  console.log(`Summary: ${passCount} passed, ${failCount} failed, ${results.length} total.`);
  console.log(`Artifacts saved in: ${path.relative(REPO_ROOT, options.outDir)}/`);
  console.log('======================================================\n');

  if (hasFailure) {
    console.error('❌ Visual diff check failed: one or more screens exceeded allowed diff threshold.');
    process.exit(1);
  } else {
    console.log('✅ Visual diff check passed: all screens within allowed diff thresholds.');
    process.exit(0);
  }
}

run().catch((err) => {
  console.error('Fatal error during visual comparison:', err);
  process.exit(1);
});
