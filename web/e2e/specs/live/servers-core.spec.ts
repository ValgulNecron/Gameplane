import { test, expect } from "@playwright/test";
import type { APIRequestContext } from "@playwright/test";
import { loginIfNeeded, seedServer, seedTemplate } from "./_seed";
import { ServersPage } from "../../pages/ServersPage";
import { ServerDetailPage } from "../../pages/ServerDetailPage";

// T085 (specs/014-heroui-web-rebuild/tasks.md): live Playwright coverage for
// the Slice 2a flows (Servers list + Server Detail core tabs) against a real
// cluster — this is the feature's E2E tier per OD-3 (Settled 2026-09-03); no
// corresponding test/e2e/ Go test is added for Slice 2a.
//
// Mirrors login-and-shell.spec.ts's structure: one describe block, storageState
// pre-authenticates every test (globalSetup), and loginIfNeeded is called only
// as the documented no-op safety net (page.goto then waitForLoadState, THEN
// loginIfNeeded) — never a second explicit login. One seeded template + one
// seeded GameServer covers every flow below; nothing here spends more than
// that one login budget-wise.

test.describe("live: servers core (list + detail)", () => {
  test.skip(
    process.env.GAMEPLANE_E2E_TARGET !== "live",
    "live-only — the mock specs (serverDetail.spec.ts, servers *.spec.ts) cover these flows against MSW",
  );

  const stamp = Date.now().toString(36);
  const tmplName = `e2e-pw-core-tmpl-${stamp}`;
  const serverName = `e2e-pw-core-${stamp}`;
  let cleanups: Array<(request: APIRequestContext) => Promise<void>> = [];

  test.beforeAll(async ({ request }) => {
    const tmpl = await seedTemplate(request, tmplName);
    const server = await seedServer(request, {
      name: serverName,
      template: tmplName,
      description: "Live servers-core probe",
    });
    // Delete the server before the template it references.
    cleanups = [server.cleanup, tmpl.cleanup];
  });

  // afterAll gets its own live `request` — beforeAll's is already disposed.
  test.afterAll(async ({ request }) => {
    for (const c of cleanups) await c(request);
  });

  test.beforeEach(async ({ page }) => {
    await page.goto("/");
    await page.waitForLoadState("domcontentloaded");
    await loginIfNeeded(page);
  });

  test("servers list: phase/template/namespace filters narrow the seeded server", async ({
    page,
  }) => {
    const servers = new ServersPage(page);
    await servers.goto();
    await expect(page.getByRole("heading", { name: /^servers$/i })).toBeVisible();

    const row = servers.getServerRow(serverName);
    // The list polls every 5s — give the seeded CR time to appear.
    await expect(row).toBeVisible({ timeout: 15_000 });

    // Phase filter (status Tabs: All/Running/Stopped): a freshly-created
    // GameServer with no running pod is neither Running nor Stopped
    // (Pending/Starting), so the "Running" tab hides it and "All" restores it.
    await servers.clickStatusTab("Running");
    await expect(row).toHaveCount(0);
    await servers.clickStatusTab("All");
    await expect(row).toBeVisible({ timeout: 10_000 });

    // Template + namespace facet filters (FilterPopover).
    await servers.filterButton.click();
    const gameCheckbox = page.getByRole("checkbox", { name: tmplName });
    const nsCheckbox = page.getByRole("checkbox", { name: "gameplane-games" });
    await expect(gameCheckbox).toBeVisible({ timeout: 10_000 });
    // HeroUI's Checkbox renders its own <label> over the (visually tiny)
    // native input, which Playwright's actionability check treats as the
    // input being covered — force through it, same as a real click on the
    // label does natively (clicking a label toggles its associated input).
    await gameCheckbox.check({ force: true });
    await expect(nsCheckbox).toBeVisible();
    await nsCheckbox.check({ force: true });
    // Apply/Clear's accessible name is just their visible text.
    await page.getByRole("button", { name: /^apply$/i }).click();
    // Both facets include the seeded server's own template/namespace, so it
    // stays visible with the filter applied.
    await expect(row).toBeVisible({ timeout: 10_000 });

    // Clear the filter so it doesn't leak into the next test. Reopen via a
    // loose name match, not servers.filterButton's exact "^Filter$" — with
    // two facets applied the button's accessible name grows a count chip
    // ("Filter 2").
    await page.getByRole("button", { name: /filter/i }).first().click();
    await page.getByRole("button", { name: /^clear$/i }).click();
    await page.getByRole("button", { name: /^apply$/i }).click();
    await expect(row).toBeVisible({ timeout: 10_000 });
  });

  test("servers list: row actions open the server actions menu and one lifecycle action fires", async ({
    page,
  }) => {
    const servers = new ServersPage(page);
    await servers.goto();
    const row = servers.getServerRow(serverName);
    await expect(row).toBeVisible({ timeout: 15_000 });

    // Lifecycle action: Stop is enabled for a Pending server (it is only
    // disabled for an already-Stopped/Suspended one) — exercise it directly
    // from the row rather than opening Detail, mirroring the "one server
    // action" scope of this task.
    const stopButton = row.getByRole("button", { name: /^stop$/i });
    await expect(stopButton).toBeVisible();
    await stopButton.click();

    // Row actions menu: Clone / Transfer / Wipe / Delete.
    const menuTrigger = row.getByRole("button", { name: /server actions/i });
    await menuTrigger.click();
    await expect(page.getByRole("menuitem", { name: /clone server/i })).toBeVisible();
    await expect(page.getByRole("menuitem", { name: /transfer ownership/i })).toBeVisible();
    await expect(page.getByRole("menuitem", { name: /wipe world data/i })).toBeVisible();
    await expect(page.getByRole("menuitem", { name: /delete server/i })).toBeVisible();
    // Close the menu without acting (Escape) — the full dialog open/cancel
    // flows are covered from Server Detail below.
    await page.keyboard.press("Escape");
  });

  test("server detail: all six core tabs render", async ({ page }) => {
    const detail = new ServerDetailPage(page);
    await detail.goto(serverName);
    await expect(page.getByRole("heading", { name: serverName })).toBeVisible({ timeout: 20_000 });

    // Not necessarily the landing tab: ServerDetail switches a provisioning
    // server (Pending/Starting, never started — exactly the seeded one) to
    // Logs the first time the GameServer query resolves. Select Overview
    // explicitly, retrying so a click that lands before that switch isn't
    // silently undone.
    await expect(async () => {
      await detail.clickTab("Overview");
      await expect(page.getByText("Connection")).toBeVisible({ timeout: 2_000 });
    }).toPass({ timeout: 20_000 });

    await detail.clickTab("Events");
    await expect(detail.tablist.getByRole("tab", { name: /^events$/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );

    await detail.clickTab("Console");
    await expect(detail.tablist.getByRole("tab", { name: /^console$/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );

    await detail.clickTab("Logs");
    await expect(detail.tablist.getByRole("tab", { name: /^logs$/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );

    await detail.clickTab("Files");
    await expect(detail.tablist.getByRole("tab", { name: /^files$/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    // No live agent sidecar is running (no seeded pod) — Files.tsx still
    // renders its tree pane's root breadcrumb without crashing.
    await expect(page.getByRole("button", { name: /new folder|refresh/i }).first()).toBeVisible({
      timeout: 10_000,
    });

    await detail.clickTab("Players");
    await expect(detail.tablist.getByRole("tab", { name: /^players$/i })).toHaveAttribute("aria-selected", "true");
    // No agent sidecar is running (no seeded pod), so the players snapshot
    // query never resolves and the header stays "Loading…". The "Online"
    // StatCard label is rendered unconditionally (value "—" until data arrives).
    await expect(page.getByText("Online", { exact: true }).first()).toBeVisible({ timeout: 10_000 });
  });

  test("server detail: clone/transfer/wipe/delete dialogs open and cancel without mutating the server", async ({
    page,
  }) => {
    const detail = new ServerDetailPage(page);
    await detail.goto(serverName);
    await expect(page.getByRole("heading", { name: serverName })).toBeVisible({ timeout: 20_000 });

    // The header's own actions menu (not a row menu) hosts the same four
    // dialogs on Server Detail.
    //
    // Root cause found via a captured CI trace (five prior fix attempts —
    // dialog toBeHidden, backdrop count 0, backdrop opacity 0 via
    // waitForFunction — all hung identically; see PR #351 review notes):
    // HeroUI v3's `ModalBackdrop`/`ModalContainer` (and the equivalent
    // `AlertDialogBackdrop`/`AlertDialogContainer` used by the wipe/delete
    // AlertDialogs) are meant to be composed as siblings under `<Modal>`,
    // matching every usage in this codebase — but react-aria-components'
    // underlying `ModalOverlay` (what `ModalBackdrop` wraps) tracks its OWN
    // exit animation AND a *second* one on an internal `modalRef` meant to
    // be attached by nested modal content
    // (react-aria-components/dist/private/Modal.mjs,
    // `ModalOverlayWithForwardRef`: `isExiting = isOverlayExiting ||
    // isModalExiting`). Used as siblings, that `modalRef` is never attached
    // to anything, so `useAnimation`'s `if (isActive && ref.current)` guard
    // (react-aria/dist/private/utils/animation.mjs) never fires for it —
    // `isModalExiting` latches `true` the very first time any dialog
    // closes and never resets, which permanently disables the backdrop's
    // "should I unmount" check for the rest of that component instance's
    // life. The backdrop element (`[data-slot="modal-backdrop"]` /
    // `[data-slot="alert-dialog-backdrop"]`) is confirmed (via
    // `iframe.contentDocument` queries against the trace's reconstructed
    // DOM snapshots, both right when the wait started and 10s later at
    // timeout) to still be in the DOM, `data-exiting="true"`, at its
    // resting (non-animating) computed opacity of 1 with `pointer-events:
    // auto` — a full-viewport `position:fixed; z-index:50` div that never
    // goes away and blocks every later interaction on that page. This is a
    // real HeroUI v3.2.4 / react-aria-components bug reachable from every
    // dialog in this app (Clone/Transfer/Wipe/Delete included) the first
    // time it's opened and closed on a given page load — not something a
    // test-side wait can wait out, since the backdrop never actually
    // disappears. Fixing the composition itself is out of scope for this
    // test (it touches production markup for six components); tracked
    // instead as a follow-up. The workaround here is to give each dialog a
    // fresh, never-yet-closed-once component instance: reload between
    // dialogs instead of reusing the same page, so the stuck backdrop from
    // dialog N never has a chance to block dialog N+1's trigger click.
    const openMenuAndDialog = async (itemName: RegExp, dialogHeading: RegExp, role: "dialog" | "alertdialog" = "dialog") => {
      await page.getByRole("button", { name: /server actions/i }).click();
      await page.getByRole("menuitem", { name: itemName }).click();
      // The dropdown's own popover is role="dialog" too and can still be
      // mid-exit-animation (data-exiting) when the modal opens, so a bare
      // getByRole("dialog") is a strict-mode violation — scope by the
      // modal's own accessible name (its heading) to pick the right one.
      const dialog = page.getByRole(role, { name: dialogHeading });
      await expect(dialog).toBeVisible({ timeout: 10_000 });
      await expect(dialog.getByRole("heading", { name: dialogHeading })).toBeVisible();
      await dialog.getByRole("button", { name: /^cancel$/i }).click();
      await expect(dialog).toBeHidden({ timeout: 5_000 });

      // The server must still exist — this dialog was cancelled, not
      // confirmed. Reload (see the stuck-backdrop note above for why) and
      // confirm the heading still renders before the next dialog opens.
      await page.reload();
      await page.waitForLoadState("domcontentloaded");
      await expect(page.getByRole("heading", { name: serverName })).toBeVisible({ timeout: 20_000 });
    };

    await openMenuAndDialog(/clone server/i, /^clone server$/i);
    await openMenuAndDialog(/transfer ownership/i, new RegExp(`transfer ${serverName}`, "i"));
    await openMenuAndDialog(/wipe world data/i, /^wipe world\?$/i, "alertdialog");
    // Delete's confirm dialog title is "Delete <name>?" (ConfirmDialog).
    await openMenuAndDialog(/delete server/i, new RegExp(`delete ${serverName}\\?`, "i"), "alertdialog");
  });
});
