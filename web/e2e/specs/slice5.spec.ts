import { test, expect, type Page } from "@playwright/test";

// T189 (specs/014-heroui-web-rebuild/tasks.md): Slice 5 — Share links, mock
// mode. Contract: specs/014-heroui-web-rebuild/contracts/share-link-ui.md.
//
// ShareLinksSection (web/src/routes/tabs/settings/ShareLinks.tsx) is built
// but not yet mounted into Settings.tsx's SECTIONS (T179 is deferred — see
// the contract and ShareLinks.tsx's own header comment) and web/src/router/
// tree.tsx registers no standalone route for it either. There is therefore
// no reachable URL to drive the Settings-side create/list/revoke flow from
// here. Once T179 mounts the section, its own dialog-flow coverage belongs
// in this file as a new test.describe block — this file, for now, covers
// only the public /share/$token page (route registered in tree.tsx), which
// is the half of the contract that is actually reachable.
//
// The public route's states all come from a single GET /shares/{token}
// (and POST …/start) pair with no per-token branching in the shared MSW
// handlers (src/test/handlers.ts always answers "Running" for any token) —
// editing those shared handlers is out of scope for this file. Instead,
// each test installs its own window.fetch shim via addInitScript, matching
// slice1.spec.ts's documented technique for MSW-unreachable network
// control (its N13Xud test explains why page.route can't see traffic MSW's
// service worker answers). The shim only intercepts /shares/ URLs and
// falls through to the real fetch — and therefore to MSW — for everything
// else, so login/session calls made by other tests are unaffected (this
// page makes none of its own).

interface ScriptedResponse {
  status: number;
  body?: unknown;
}

// Installs a window.fetch shim, before any app script runs, that answers
// GET /shares/:token and POST /shares/:token/start from a scripted table
// keyed by "<token>" and "<token>:start" respectively. Each key's value may
// be a single response (repeated for every call) or an array consumed in
// order (holding on the last entry once exhausted) — the latter is what
// lets a test script the Starting → polling → Up transition.
//
// For array-based responses, a mutable phase variable tracks the current state.
// The phase advances only when a polling call arrives after a Start click,
// preventing StrictMode double-renders from skipping states.
async function mockShareResolve(
  page: Page,
  responses: Record<string, ScriptedResponse | ScriptedResponse[]>,
): Promise<void> {
  await page.addInitScript((responsesJson: string) => {
    const table = JSON.parse(responsesJson) as Record<
      string,
      { status: number; body?: unknown } | { status: number; body?: unknown }[]
    >;
    // Track phase index for each token with array-based responses
    const phases: Record<string, number> = {};
    // Track whether a start was just called, so next resolve advances the phase
    const pendingAdvance: Record<string, boolean> = {};
    const originalFetch = window.fetch.bind(window);
    window.fetch = (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString();
      const match = /\/shares\/([^/?]+)(\/start)?/.exec(url);
      if (match) {
        const token = decodeURIComponent(match[1]);
        const isStart = !!match[2];
        if (isStart) {
          // Start call: answer with the start response and mark for phase advance on next resolve
          const startKey = `${token}:start`;
          const startEntry = table[startKey];
          if (startEntry) {
            const step = Array.isArray(startEntry) ? startEntry[0] : startEntry;
            pendingAdvance[token] = true;
            return Promise.resolve(
              new Response(step.body !== undefined ? JSON.stringify(step.body) : null, {
                status: step.status,
                headers: { "Content-Type": "application/json" },
              }),
            );
          }
        } else {
          // Resolve call: answer with response at current phase
          const entry = table[token];
          if (entry) {
            if (Array.isArray(entry)) {
              // Initialize phase if needed
              if (!(token in phases)) {
                phases[token] = 0;
              }
              const currentPhase = phases[token];
              // Advance phase on resolve calls after start was called: advance first,
              // then return the response at the new phase. On initial (pre-Start) calls
              // when pendingAdvance[token] is undefined, return entry[0] without advancing.
              if (pendingAdvance[token] !== undefined) {
                phases[token] = Math.min(currentPhase + 1, entry.length - 1);
                pendingAdvance[token] = false; // Reset flag after first post-Start advance
              }
              const step = entry[Math.min(phases[token], entry.length - 1)];
              return Promise.resolve(
                new Response(step.body !== undefined ? JSON.stringify(step.body) : null, {
                  status: step.status,
                  headers: { "Content-Type": "application/json" },
                }),
              );
            } else {
              // Single response: return as-is
              const step = entry;
              return Promise.resolve(
                new Response(step.body !== undefined ? JSON.stringify(step.body) : null, {
                  status: step.status,
                  headers: { "Content-Type": "application/json" },
                }),
              );
            }
          }
        }
      }
      return originalFetch(input, init);
    };
  }, JSON.stringify(responses));
}

// FR-005 (rule 3): the public page must never reveal cluster name,
// namespace, version, user names, or server counts. Checked against the
// full rendered body text on every state below.
async function expectNoPrivacyLeak(page: Page): Promise<void> {
  const text = (await page.locator("body").innerText()).toLowerCase();
  for (const forbidden of ["namespace", "cluster", "gameplane-games", "v0.", "beta"]) {
    expect(text).not.toContain(forbidden);
  }
}

test.describe("Slice 5: Share links — public page (mock mode)", () => {
  test.skip(
    process.env.GAMEPLANE_E2E_TARGET === "live",
    "share-link states are scripted via a fetch shim; live coverage is web/e2e/specs/live/share-links.spec.ts",
  );

  test("Up: server name, Online, connect address, player count", async ({ page }) => {
    await mockShareResolve(page, {
      "tok-up": {
        status: 200,
        body: {
          serverName: "mc-survival",
          status: "Running",
          address: { host: "play.gameplane.example", port: 25565 },
          playersOnline: 3,
        },
      },
    });
    await page.goto("/share/tok-up");
    await expect(page.getByRole("heading", { name: "mc-survival" })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByText("Online", { exact: true })).toBeVisible();
    await expect(page.getByText("play.gameplane.example:25565")).toBeVisible();
    await expect(page.getByText("3 players online")).toBeVisible();
    await expectNoPrivacyLeak(page);
  });

  test("Up: no exposed address renders 'Not exposed' instead of blank", async ({ page }) => {
    await mockShareResolve(page, {
      "tok-up-noaddr": { status: 200, body: { serverName: "hidden-server", status: "Running" } },
    });
    await page.goto("/share/tok-up-noaddr");
    await expect(page.getByRole("heading", { name: "hidden-server" })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByText("Not exposed")).toBeVisible();
    await expectNoPrivacyLeak(page);
  });

  test("Asleep and can start: shows a Start button", async ({ page }) => {
    await mockShareResolve(page, {
      "tok-asleep-start": { status: 200, body: { serverName: "mc-survival", status: "Suspended" } },
    });
    await page.goto("/share/tok-asleep-start");
    await expect(page.getByRole("heading", { name: "mc-survival" })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByText("Asleep", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: /start server/i })).toBeVisible();
    await expectNoPrivacyLeak(page);
  });

  test("Starting: clicking Start transitions to Starting, then polls through to Up", async ({
    page,
  }) => {
    await mockShareResolve(page, {
      "tok-wake": [
        { status: 200, body: { serverName: "mc-survival", status: "Suspended" } }, // initial resolve
        { status: 200, body: { serverName: "mc-survival", status: "Pending" } }, // first poll: still waking
        {
          status: 200,
          body: {
            serverName: "mc-survival",
            status: "Running",
            address: { host: "play.gameplane.example", port: 25565 },
          },
        }, // second poll: up
      ],
      // Body {} rather than none: api.ts's api<T>() calls res.json() for any
      // non-204 success status, which throws on a genuinely empty body in a
      // real browser (the live API's POST …/start does return 202 with no
      // body — api/internal/handlers/shares.go — a latent parse-error risk
      // in api.ts noted here rather than fixed, since api.ts is out of this
      // file's scope; giving the mock a body keeps this test about SharePage's
      // own state machine, not about that separate issue).
      "tok-wake:start": { status: 202, body: {} },
    });
    await page.goto("/share/tok-wake");
    const startButton = page.getByRole("button", { name: /start server/i });
    await expect(startButton).toBeVisible({ timeout: 10_000 });
    await startButton.click();

    await expect(page.getByText("Starting...").first()).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText(/waking up/i)).toBeVisible();

    // Polling runs every 2s in SharePage; allow a few intervals to land.
    await expect(page.getByText("Online")).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText("play.gameplane.example:25565")).toBeVisible();
  });

  test("Starting: reached directly from an in-progress resolve (no click needed)", async ({
    page,
  }) => {
    await mockShareResolve(page, {
      "tok-starting": { status: 200, body: { serverName: "mc-survival", status: "Starting" } },
    });
    await page.goto("/share/tok-starting");
    await expect(page.getByRole("heading", { name: "mc-survival" })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByText("Starting...").first()).toBeVisible();
    await expect(page.getByText(/waking up/i)).toBeVisible();
    await expectNoPrivacyLeak(page);
  });

  test("Asleep view-only: reached when a Start attempt polls back to still-asleep", async ({
    page,
  }) => {
    // SharePage.tsx always shows the Start button on the initial Asleep
    // resolve; the view-only copy only appears once a Start attempt's
    // polling still finds the server asleep (see the component's own
    // comment on this — there is no direct "no permission" signal on the
    // resolve response itself).
    await mockShareResolve(page, {
      "tok-no-perm": [
        { status: 200, body: { serverName: "mc-survival", status: "Suspended" } },
        { status: 200, body: { serverName: "mc-survival", status: "Suspended" } },
      ],
      "tok-no-perm:start": { status: 202, body: {} }, // see the tok-wake test's comment on why body: {}
    });
    await page.goto("/share/tok-no-perm");
    const startButton = page.getByRole("button", { name: /start server/i });
    await expect(startButton).toBeVisible({ timeout: 10_000 });
    await startButton.click();

    await expect(page.getByText(/check back later/i)).toBeVisible({ timeout: 15_000 });
    await expect(page.getByRole("button", { name: /start server/i })).toHaveCount(0);
    await expectNoPrivacyLeak(page);
  });

  test("Invalid/expired: a 404 resolve renders the neutral copy", async ({ page }) => {
    await mockShareResolve(page, { "tok-gone": { status: 404 } });
    await page.goto("/share/tok-gone");
    await expect(page.getByRole("heading", { name: /link not available/i })).toBeVisible({
      timeout: 10_000,
    });
    await expect(
      page.getByText(/this link may be invalid, expired, or revoked/i),
    ).toBeVisible();
    await expectNoPrivacyLeak(page);
  });

  test("Invalid/expired: a rate-limited (429) resolve renders the identical neutral copy", async ({
    page,
  }) => {
    // FR-005: invalid, expired, and rate-limited must be indistinguishable.
    await mockShareResolve(page, { "tok-limited": { status: 429 } });
    await page.goto("/share/tok-limited");
    await expect(page.getByRole("heading", { name: /link not available/i })).toBeVisible({
      timeout: 10_000,
    });
    await expect(
      page.getByText(/this link may be invalid, expired, or revoked/i),
    ).toBeVisible();
    await expectNoPrivacyLeak(page);
  });

  test("honors a stored dark appearance preference with no toggle exposed", async ({ page }) => {
    await mockShareResolve(page, {
      "tok-theme": { status: 200, body: { serverName: "mc-survival", status: "Running" } },
    });
    await page.addInitScript(() => {
      try {
        window.localStorage.setItem("gameplane-theme", "dark");
      } catch {
        // localStorage unavailable — not the behavior under test here.
      }
    });
    await page.goto("/share/tok-theme");
    await expect(page.getByRole("heading", { name: "mc-survival" })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
    // No appearance control is rendered on the public page.
    await expect(page.getByRole("button", { name: /appearance|theme|dark mode|light mode/i })).toHaveCount(
      0,
    );
  });
});
