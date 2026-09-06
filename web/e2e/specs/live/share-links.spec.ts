import { test, expect } from "@playwright/test";
import type { APIRequestContext, Page } from "@playwright/test";
import { loginIfNeeded, seedServer, seedTemplate } from "./_seed";

// T190 (specs/014-heroui-web-rebuild/tasks.md): live share-link coverage —
// this is the feature's E2E tier per OD-3 (Settled 2026-09-03); no
// corresponding test/e2e/ Go test is added for share links.
//
// ShareLinksSection (web/src/routes/tabs/settings/ShareLinks.tsx) is built
// but not yet mounted into Settings.tsx's SECTIONS (T179 is deferred), so
// there is no dashboard UI to click through to create/revoke a link. This
// spec drives the authenticated create/revoke calls directly via
// page.request (same-session cookies, so no extra login) against the real
// API — the same technique _seed.ts uses for template/server fixtures —
// and reserves the UI-driven half of the flow for whenever T179 lands the
// Settings section. The public /share/$token route (registered in
// web/src/router/tree.tsx) IS real UI and is exercised as such, in a
// separate, unauthenticated browser context to genuinely test "signed
// out" rather than merely a page an admin session happens to be able to
// see.
//
// One seeded template + one seeded GameServer, one real login (via
// storageState / loginIfNeeded's no-op safety net) covers the whole file,
// mirroring servers-core.spec.ts's login-budget discipline.

test.describe("live: share links", () => {
  test.skip(
    process.env.GAMEPLANE_E2E_TARGET !== "live",
    "the mock spec (web/e2e/specs/slice5.spec.ts) covers share-link states against a scripted fetch shim",
  );

  const stamp = Date.now().toString(36);
  const tmplName = `e2e-pw-share-tmpl-${stamp}`;
  const serverName = `e2e-pw-share-${stamp}`;
  let cleanups: Array<(request: APIRequestContext) => Promise<void>> = [];

  test.beforeAll(async ({ request }) => {
    const tmpl = await seedTemplate(request, tmplName);
    const server = await seedServer(request, {
      name: serverName,
      template: tmplName,
      description: "Live share-link probe",
    });
    cleanups = [server.cleanup, tmpl.cleanup];
  });

  test.afterAll(async ({ request }) => {
    for (const c of cleanups) await c(request);
  });

  test.beforeEach(async ({ page }) => {
    await page.goto("/");
    await page.waitForLoadState("domcontentloaded");
    await loginIfNeeded(page);
  });

  // page.request shares the authenticated browser context's cookies, but
  // mutating calls still need the CSRF header the SPA's own fetch wrapper
  // adds automatically (api/internal/auth/sessions.go's double-submit
  // check) — read it out of the same cookie jar, mirroring _seed.ts's
  // seedHeaders (not exported, so reimplemented locally here).
  async function csrfHeaders(p: Page): Promise<Record<string, string>> {
    const cookies = await p.context().cookies();
    const token = cookies.find((c) => c.name === "gameplane_csrf")?.value ?? "";
    return { "X-Gameplane-CSRF": token, Accept: "application/json" };
  }

  test("create a link, resolve it signed out, revoke it, and see Invalid signed out", async ({
    page,
    browser,
  }) => {
    // Create the link as the authenticated admin (server owner) —
    // api/internal/handlers/shares.go: "only the server owner can manage
    // shares", and seedServer created this one as this same admin.
    const createRes = await page.request.post(`/servers/${serverName}:shares`, {
      headers: await csrfHeaders(page),
      data: { canStart: true, expiresIn: "24h" },
    });
    expect(createRes.ok(), await createRes.text().catch(() => "")).toBeTruthy();
    const created = (await createRes.json()) as { id: string; token: string };
    expect(created.token).toBeTruthy();
    expect(created.id).toBeTruthy();

    // Resolve signed out: a genuinely separate, storageState-free context
    // — no admin session cookie ever exists here — so this is the same
    // request an anonymous link recipient would make.
    const anonContext = await browser.newContext();
    const anonPage = await anonContext.newPage();
    try {
      await anonPage.goto(`/share/${created.token}`);
      await anonPage.waitForLoadState("domcontentloaded");

      // Every valid state (Up / Asleep-can-start / Starting) renders the
      // server's own name as the page's <h1>; only Invalid does not. A
      // freshly-created GameServer may be in any of those three phases
      // depending on how far the operator has gotten, so assert on the
      // name rather than a specific status chip.
      const nameHeading = anonPage.getByRole("heading", { name: serverName });
      await expect(nameHeading).toBeVisible({ timeout: 20_000 });
      await expect(anonPage.getByRole("heading", { name: /link not available/i })).toHaveCount(0);

      // Best-effort start action: only meaningful if the server happened
      // to resolve into the Asleep-can-start state during the window
      // above. Not asserted as required — this is a bonus check per
      // T190, not the spec's core flow.
      const startButton = anonPage.getByRole("button", { name: /start server/i });
      if (await startButton.isVisible().catch(() => false)) {
        await startButton.click();
        await expect(anonPage.getByText(/starting|online/i).first()).toBeVisible({
          timeout: 15_000,
        });
      }

      // Revoke as the admin, in the original authenticated page.
      const revokeRes = await page.request.delete(`/servers/${serverName}/shares/${created.id}`, {
        headers: await csrfHeaders(page),
      });
      expect(revokeRes.ok(), await revokeRes.text().catch(() => "")).toBeTruthy();

      // Reopen signed out: the same token now resolves to Invalid. FR-005:
      // the copy must not hint that the link ever existed.
      await anonPage.goto(`/share/${created.token}`);
      await expect(
        anonPage.getByRole("heading", { name: /link not available/i }),
      ).toBeVisible({ timeout: 20_000 });
      await expect(
        anonPage.getByText(/this link may be invalid, expired, or revoked/i),
      ).toBeVisible();
    } finally {
      await anonContext.close();
    }
  });
});
