import { test, expect } from "@playwright/test";

// Live happy path: create a busybox-templated GameServer through the
// dashboard, navigate around its detail page, and delete it cleanly.
//
// This is the only Playwright spec that mutates real cluster state, so
// it's gated to live mode only. Mock mode is covered by the
// createServer + serverDetail specs.
//
// Prerequisites (all set up by `make e2e-up`):
//   - kestrel-e2e kind cluster is running
//   - Helm chart is installed in kestrel-system
//   - The "e2e-busybox" / "e2e-lifecycle-busybox" GameTemplate exists
//     because the Go E2E suite ran first and created one. If neither
//     exists, the spec falls back to whatever Templates.list() returns,
//     and skips if the catalog is empty.
//
// The spec uses test.afterEach to clean up its own GameServer even if
// an earlier step fails — leaving the cluster clean for subsequent
// runs of `npm run test:e2e:live`.

test.describe("live: create and cleanup a busybox server", () => {
  test.skip(
    process.env.GAMEPLANE_E2E_TARGET !== "live",
    "live-only spec — mock mode covers the wizard",
  );

  // We let each invocation pick its own server name so reruns don't
  // collide with leftover state if cleanup ever fails.
  const serverName = `e2e-pw-live-${Date.now().toString(36)}`;

  test.afterEach(async ({ request }) => {
    // Best-effort cleanup. If the test deleted the server already this
    // is a no-op; if it didn't, this prevents the next run from
    // colliding on the name. Errors here are intentionally swallowed —
    // the test result has already been decided by the time afterEach
    // runs, and a 404 here is the success case.
    try {
      await request.delete(`/servers/${serverName}`);
    } catch {
      // ignore — cleanup is best-effort
    }
  });

  test("creates a busybox server, sees it on detail, deletes it", async ({ page }) => {
    test.setTimeout(180_000); // pod scheduling + image pull on first run

    await page.goto("/servers/new");
    await page.waitForLoadState("domcontentloaded");

    // Step 1: pick the first available template card. The Go E2E suite
    // creates "e2e-busybox" or "e2e-lifecycle-busybox"; if neither is
    // present (the cluster was wiped between runs), skip rather than
    // fail.
    const cards = page.locator("button:has(span)").filter({
      hasText: /busybox|minecraft|valheim|terraria/i,
    });
    const count = await cards.count();
    test.skip(count === 0, "no GameTemplates available on the live cluster");
    await cards.first().click();
    await page.getByRole("button", { name: /continue/i }).click();

    // Step 2: name + defaults.
    await page.getByPlaceholder(/mc-hardcore/i).fill(serverName);

    // Step 2 → 3 → 4 → submit.
    await page.getByRole("button", { name: /continue/i }).click();
    await page.getByRole("button", { name: /continue/i }).click();
    await page.getByRole("button", { name: /create server/i }).click();

    // Detail page renders for our new server.
    await page.waitForURL(new RegExp(`/servers/${serverName}$`), { timeout: 30_000 });
    await expect(page.getByRole("heading", { name: serverName })).toBeVisible({ timeout: 15_000 });

    // The phase badge eventually settles on Running (or Stopped / Stopping
    // if the operator hasn't reconciled yet). We're testing the navigation
    // path — not the operator reconciliation latency — so just ensure SOME
    // phase badge is present within a generous timeout.
    await expect(page.locator("header").getByText(/running|pending|starting|stopped/i).first())
      .toBeVisible({ timeout: 60_000 });

    // Switch to a couple of tabs to confirm they don't crash mid-flight.
    const tabNav = page.locator("header nav");
    for (const label of ["Overview", "Logs", "Settings"]) {
      await tabNav.getByRole("button", { name: new RegExp(`^${label}$`) }).click();
      await page.waitForTimeout(200);
    }

    // Delete the server through Settings → Danger zone. The page object
    // pattern would tidy this up, but the live spec stays self-contained
    // so reading it cold is straightforward.
    await tabNav.getByRole("button", { name: /^Settings$/ }).click();
    const dangerTab = page.getByRole("button", { name: /^danger$/i }).first();
    if (await dangerTab.isVisible().catch(() => false)) {
      await dangerTab.click();
    }
    const deleteBtn = page.getByRole("button", { name: /^delete( server)?$/i }).first();
    await deleteBtn.click();

    // ConfirmDialog requires typing the resource name; supply it.
    const confirmInput = page.getByRole("dialog").getByRole("textbox").first();
    if (await confirmInput.isVisible().catch(() => false)) {
      await confirmInput.fill(serverName);
    }
    const confirmBtn = page.getByRole("dialog").getByRole("button", { name: /^delete/i });
    await confirmBtn.click();

    // After delete, dashboard navigates back to /servers; the deleted
    // server should no longer appear in the list.
    await page.waitForURL((u) => !u.pathname.endsWith(`/servers/${serverName}`), { timeout: 30_000 });
    await expect(page.getByText(serverName)).toHaveCount(0, { timeout: 15_000 });
  });
});
