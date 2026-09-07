import { test, expect } from "@playwright/test";
import type { APIRequestContext, APIResponse } from "@playwright/test";
import { loginIfNeeded, seedServer } from "./_seed";
import { ServerDetailPage } from "../../pages/ServerDetailPage";

// T117 (specs/014-heroui-web-rebuild/tasks.md): live Playwright coverage for
// the Slice 2b flows (Server Detail — Mods/Modpacks/Backups tabs and every
// Settings sub-tab) against a real cluster — this is the feature's E2E tier
// per OD-3 (Settled 2026-09-03); no corresponding test/e2e/ Go test is added
// for Slice 2b.
//
// Mirrors servers-core.spec.ts's / login-and-shell.spec.ts's structure: one
// describe block, storageState pre-authenticates every test (globalSetup),
// and loginIfNeeded is called only as the documented no-op safety net
// (page.goto then waitForLoadState, THEN loginIfNeeded) — never a second
// explicit login. One seeded template + one seeded GameServer covers every
// flow below.
//
// _seed.ts's seedTemplate() builds a plain busybox template with no mod
// capabilities, so it can't drive the Mods/Modpacks tabs this file needs —
// this file seeds its own template (capabilities.mods with a path + a
// modrinth registry provider declaring `modpacks`, which is what makes both
// serverHasMods and serverHasModpacks — web/src/lib/capabilities.ts —
// return true) via the same request+CSRF pattern _seed.ts's seedTemplate
// uses internally, reusing _seed.ts's seedServer/loginIfNeeded exports for
// everything else.

test.describe("live: server settings (Mods/Modpacks/Backups + Settings)", () => {
  test.skip(
    process.env.GAMEPLANE_E2E_TARGET !== "live",
    "live-only — the mock specs (ServerDetail.test.tsx, tabs/*.test.tsx) cover these flows against MSW",
  );

  const stamp = Date.now().toString(36);
  const tmplName = `e2e-pw-settings-tmpl-${stamp}`;
  const serverName = `e2e-pw-settings-${stamp}`;
  let cleanups: Array<(request: APIRequestContext) => Promise<void>> = [];

  // Mirrors _seed.ts's seedHeaders/expectSeedOk — duplicated locally rather
  // than exported from _seed.ts since this is the only spec that needs a
  // template with mod capabilities.
  async function seedHeaders(request: APIRequestContext): Promise<Record<string, string>> {
    const state = await request.storageState();
    const token = state.cookies.find((c) => c.name === "gameplane_csrf")?.value ?? "";
    return { "X-Gameplane-CSRF": token, Accept: "application/json" };
  }

  async function expectSeedOk(res: APIResponse, what: string): Promise<void> {
    if (!res.ok() && res.status() !== 409) {
      throw new Error(`${what} failed: ${res.status()} ${await res.text().catch(() => "")}`);
    }
  }

  test.beforeAll(async ({ request }) => {
    const tmplRes = await request.post("/templates", {
      headers: await seedHeaders(request),
      data: {
        apiVersion: "gameplane.local/v1alpha1",
        kind: "GameTemplate",
        metadata: { name: tmplName },
        spec: {
          displayName: `E2E live ${tmplName}`,
          game: "busybox",
          version: "1",
          image: "busybox:1.36",
          command: ["sh", "-c", "sleep 100000"],
          ports: [{ name: "noop", containerPort: 12345, advertise: true, protocol: "TCP" }],
          // path + a registry provider with `modpacks` set is what flips
          // serverHasMods AND serverHasModpacks true with no per-loader
          // model involved (capabilities.ts: "No per-loader model → keep
          // the prior template-level behavior. return true").
          capabilities: {
            mods: {
              path: "mods",
              extensions: [".jar"],
              install: { allowedHosts: ["modrinth.com"] },
              registry: { providers: [{ provider: "modrinth", modpacks: {} }] },
            },
          },
        },
      },
    });
    await expectSeedOk(tmplRes, `seed template ${tmplName}`);

    const server = await seedServer(request, {
      name: serverName,
      template: tmplName,
      description: "Live server-settings probe",
    });
    // Delete the server before the template it references.
    cleanups = [
      server.cleanup,
      async (req: APIRequestContext) => {
        await req
          .delete(`/templates/${tmplName}`, { headers: await seedHeaders(req) })
          .catch(() => undefined);
      },
    ];
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

  test("Mods, Modpacks, and Backups tabs render", async ({ page }) => {
    const detail = new ServerDetailPage(page);
    await detail.goto(serverName);
    await expect(page.getByRole("heading", { name: serverName })).toBeVisible({ timeout: 20_000 });

    await detail.clickTab("Mods");
    await expect(detail.tablist.getByRole("tab", { name: /^mods$/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    // No live agent sidecar is running (no seeded pod) — ModsTab still
    // renders its count/error header without crashing (mirrors the Files
    // tab assertion in servers-core.spec.ts).
    await expect(
      page.getByText(/installed|couldn.?t load mods|loading…/i).first(),
    ).toBeVisible({ timeout: 15_000 });

    await detail.clickTab("Modpacks");
    await expect(detail.tablist.getByRole("tab", { name: /^modpacks$/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await expect(page.getByText(/browse modpacks/i)).toBeVisible({ timeout: 10_000 });

    await detail.clickTab("Backups");
    await expect(detail.tablist.getByRole("tab", { name: /^backups$/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    // BackupsTab always renders its "Schedules" and "Backups" section
    // headers, even with none seeded for this server yet.
    await expect(page.getByRole("heading", { name: /^schedules$/i })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByText(/no schedules yet/i)).toBeVisible();
  });

  test("every Settings sub-section opens", async ({ page }) => {
    const detail = new ServerDetailPage(page);
    await detail.goto(serverName);
    await expect(page.getByRole("heading", { name: serverName })).toBeVisible({ timeout: 20_000 });

    await detail.clickTab("Settings");

    // Mirrors Settings.tsx's SECTIONS labels. None of these collide with
    // the top-level Server Detail tab names, so a bare page-level
    // getByRole("tab", ...) unambiguously hits the vertical Settings nav
    // (Settings.tsx's own <Tabs>) without needing to scope past it.
    // "Version" is deliberately excluded — Settings.tsx filters it out
    // for a template with no version catalog (this seeded busybox
    // template has none).
    const sectionLabels = [
      "General",
      "Resources",
      "Networking",
      "Environment",
      "Lifecycle",
      "Scheduled backups",
      "Network capture",
      "Placement",
      "RBAC & access",
      "Danger zone",
    ];
    for (const label of sectionLabels) {
      const tab = page.getByRole("tab", { name: new RegExp(`^${label}$`, "i") });
      await tab.click();
      await expect(tab).toHaveAttribute("aria-selected", "true");
    }
  });

  test("save one settings field, discard a change, and re-open to verify state", async ({
    page,
  }) => {
    const detail = new ServerDetailPage(page);
    await detail.goto(serverName);
    await expect(page.getByRole("heading", { name: serverName })).toBeVisible({ timeout: 20_000 });
    await detail.clickTab("Settings");

    // GeneralSection's Description field is the only free-text field on
    // this template — Name/Template are disabled inputs. Field.tsx has no
    // <label>, so the textarea's accessible name falls back to its
    // placeholder; target it directly instead.
    const description = page.getByPlaceholder(/long-standing survival realm/i);
    await expect(description).toBeVisible({ timeout: 15_000 });

    const saved = `updated description ${stamp}`;
    await description.fill(saved);
    const saveButton = page.getByRole("button", { name: /^save changes$/i });
    await expect(saveButton).toBeEnabled();
    await saveButton.click();
    await expect(page.getByText(/^saved\.$/i)).toBeVisible({ timeout: 10_000 });
    await expect(description).toHaveValue(saved);

    // Make a further change, then discard it — it should revert to the
    // last-saved value, not the pre-save original.
    await description.fill(`${saved} — discard me`);
    await page.getByRole("button", { name: /^discard$/i }).click();
    await expect(description).toHaveValue(saved);

    // Re-open (full reload) and confirm the saved value persisted.
    await page.reload();
    await page.waitForLoadState("domcontentloaded");
    await expect(page.getByRole("heading", { name: serverName })).toBeVisible({ timeout: 20_000 });
    await detail.clickTab("Settings");
    await expect(page.getByPlaceholder(/long-standing survival realm/i)).toHaveValue(saved, {
      timeout: 15_000,
    });
  });
});
