import { test, expect, type Page } from "@playwright/test";
import path from "path";
import { fileURLToPath } from "node:url";

// T191 (specs/014-heroui-web-rebuild/tasks.md): Slice 5 — Share links screen
// captures, for comparison against design-export/screenshots/<id>.png per
// contracts/screen-verification.md. Mirrors slice2a.spec.ts's structure
// (viewport, capture() helper, id-named PNGs, dark @screenshots-tagged
// describe block) and is selected the same way — only when
// GAMEPLANE_SCREENSHOTS=1 (via playwright.config.ts's grep/grepInvert on
// the @screenshots tag).
//
// Ten screen ids were designed for this slice
// (specs/014-heroui-web-rebuild/contracts/share-link-ui.md): five Settings ·
// Share links frames (xCJlu, dQV9N, atqRh, VM7ro, S7SCDc) and five public
// /share/$token page states (C2LQE4, q31B6w, qFLfB, EcoGD, epZO2). Only the
// public-page five are captured here — ShareLinksSection
// (web/src/routes/tabs/settings/ShareLinks.tsx) is built but not yet
// mounted into Settings.tsx's SECTIONS (T179 is deferred) and no
// standalone route reaches it either, so there is nothing at any URL to
// screenshot for the other five yet. The skipped placeholders below name
// them so `grep -n T191 tasks.md` and this file agree on what remains;
// un-skip them (dropping the fixture-URL TODO) once T179 mounts the
// section.
//
// The public route answers a single GET /shares/{token} (and POST
// …/start) with no per-token branching in the shared MSW handlers
// (src/test/handlers.ts always answers "Running"), and editing those
// shared handlers is out of scope for this file — so, like slice5.spec.ts
// (the mock functional spec), each test below installs its own
// window.fetch shim via addInitScript to script the resolve/start
// responses for its one token. See that file's mockShareResolve doc
// comment for why this is the correct technique (page.route cannot see
// traffic MSW's service worker answers in mock mode).

interface ScriptedResponse {
  status: number;
  body?: unknown;
}

async function mockShareResolve(
  page: Page,
  responses: Record<string, ScriptedResponse | ScriptedResponse[]>,
): Promise<void> {
  await page.addInitScript((responsesJson: string) => {
    const table = JSON.parse(responsesJson) as Record<
      string,
      { status: number; body?: unknown } | { status: number; body?: unknown }[]
    >;
    const calls: Record<string, number> = {};
    const originalFetch = window.fetch.bind(window);
    window.fetch = (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString();
      const match = /\/shares\/([^/?]+)(\/start)?/.exec(url);
      if (match) {
        const token = decodeURIComponent(match[1]);
        const key = match[2] ? `${token}:start` : token;
        const entry = table[key];
        if (entry) {
          const steps = Array.isArray(entry) ? entry : [entry];
          const i = Math.min(calls[key] ?? 0, steps.length - 1);
          calls[key] = (calls[key] ?? 0) + 1;
          const step = steps[i];
          return Promise.resolve(
            new Response(step.body !== undefined ? JSON.stringify(step.body) : null, {
              status: step.status,
              headers: { "Content-Type": "application/json" },
            }),
          );
        }
      }
      return originalFetch(input, init);
    };
  }, JSON.stringify(responses));
}

async function capture(page: Page, id: string): Promise<void> {
  const here = path.dirname(fileURLToPath(import.meta.url));
  const screenshotPath = path.join(here, `${id}.png`);
  await page.screenshot({ path: screenshotPath, fullPage: true });
}

test.describe("Slice 5: Share links — Settings surfaces (Desktop — 1440x900) @screenshots", () => {
  test.use({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 2,
    colorScheme: "dark",
  });

  // Blocked on T179 mounting ShareLinksSection into Settings.tsx (or a
  // standalone route) — there is no reachable fixture URL yet. See this
  // file's header comment.
  test.skip("xCJlu: Settings — Share links (list)", async () => {});
  test.skip("dQV9N: Settings — Share links (empty state)", async () => {});
  test.skip("atqRh: Settings — Share links (create dialog)", async () => {});
  test.skip("VM7ro: Settings — Share links (created dialog)", async () => {});
  test.skip("S7SCDc: Settings — Share links (revoke dialog)", async () => {});
});

test.describe("Slice 5: Share links — public page (Desktop — 1440x900) @screenshots", () => {
  test.use({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 2,
    colorScheme: "dark",
  });

  test("C2LQE4: Share page — Up", async ({ page }) => {
    await mockShareResolve(page, {
      "shot-up": {
        status: 200,
        body: {
          serverName: "mc-survival",
          status: "Running",
          address: { host: "play.gameplane.example", port: 25565 },
          playersOnline: 3,
        },
      },
    });
    await page.goto("/share/shot-up");
    await expect(page.getByRole("heading", { name: "mc-survival" })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByText("Online")).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "C2LQE4");
  });

  test("q31B6w: Share page — Asleep, can start", async ({ page }) => {
    await mockShareResolve(page, {
      "shot-asleep-start": {
        status: 200,
        body: { serverName: "mc-survival", status: "Suspended" },
      },
    });
    await page.goto("/share/shot-asleep-start");
    await expect(page.getByRole("heading", { name: "mc-survival" })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByRole("button", { name: /start server/i })).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "q31B6w");
  });

  test("qFLfB: Share page — Asleep, view-only", async ({ page }) => {
    await mockShareResolve(page, {
      "shot-asleep-view": [
        { status: 200, body: { serverName: "mc-survival", status: "Suspended" } },
        { status: 200, body: { serverName: "mc-survival", status: "Suspended" } },
      ],
      "shot-asleep-view:start": { status: 202, body: {} },
    });
    await page.goto("/share/shot-asleep-view");
    const startButton = page.getByRole("button", { name: /start server/i });
    await expect(startButton).toBeVisible({ timeout: 10_000 });
    await startButton.click();
    await expect(page.getByText(/check back later/i)).toBeVisible({ timeout: 15_000 });
    await page.waitForTimeout(200);
    await capture(page, "qFLfB");
  });

  test("EcoGD: Share page — Starting", async ({ page }) => {
    await mockShareResolve(page, {
      "shot-starting": { status: 200, body: { serverName: "mc-survival", status: "Starting" } },
    });
    await page.goto("/share/shot-starting");
    await expect(page.getByRole("heading", { name: "mc-survival" })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByText("Starting...").first()).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "EcoGD");
  });

  test("epZO2: Share page — Invalid/expired", async ({ page }) => {
    await mockShareResolve(page, { "shot-gone": { status: 404 } });
    await page.goto("/share/shot-gone");
    await expect(page.getByRole("heading", { name: /link not available/i })).toBeVisible({
      timeout: 10_000,
    });
    await page.waitForTimeout(200);
    await capture(page, "epZO2");
  });
});
