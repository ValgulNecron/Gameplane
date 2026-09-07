import { test, expect, type Page } from "@playwright/test";
import path from "path";
import { fileURLToPath } from "node:url";

// T159 (specs/014-heroui-web-rebuild/tasks.md): Slice 4 — Admin Settings,
// Users & RBAC, Audit Log, System Logs, and Cluster Settings. Screenshot
// verification tests for the HeroUI rebuild, capturing the design frames
// listed in specs/014-heroui-web-rebuild/contracts/component-map.md's
// "Slice 4" entry / design-export/MANIFEST.md, at 1440px, for comparison
// against design-export/screenshots/<id>.png per
// contracts/screen-verification.md.
//
// Mirrors slice2a.spec.ts's structure (viewport, capture() helper,
// id-named PNGs, the useScreenshotDataset() localStorage switch) and is
// selected the same way — only when GAMEPLANE_SCREENSHOTS=1 (via
// playwright.config.ts's grep/grepInvert on the @screenshots tag).
//
// Several ids on the manifest are component crops (removable group chip
// variants, provenance badge variants) rather than distinct screens. Per
// the contract, "the comparison is visual, at reference width" — not a
// pixel diff — so each of those is captured as the same full-page state
// that renders the component, under its own id, exactly like the
// full-screen ids: uMiwd's Authentication section renders the
// "Overridden" provenance badge (R65Xyx) with an orange chip (XL5ZU) on
// its admin row and the "From Helm" badge (Rwnu3) with violet/secondary
// chips (vStkb/uw0dB) on its operator/viewer rows, all in one capture.
//
// screenshotConfig() (web/src/test/screenshotData.ts) was extended
// additively to back this: a dashboard override on the admin role
// mapping plus a Helm-seeded oidcHelmProvider with role mappings on all
// three roles, and a set game-data storage class. The "no OIDC mappings
// yet" (nNGDX/BV5ei), "save rejected" (QgW58), and "default storage
// class" (dxdEi) variants are one-off states produced per-test via
// page.route, not dataset fixtures — each is a single test's fixture,
// not something other tests should see.

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

async function clickSection(page: Page, name: string): Promise<void> {
  const btn = page.getByRole("button", { name, exact: true });
  await expect(btn).toBeVisible({ timeout: 10_000 });
  await btn.click();
}

test.describe("Slice 4: Admin, Users, Audit, System logs, Cluster (Desktop — 1440x900) @screenshots", () => {
  test.use({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 2,
    colorScheme: "dark",
  });

  test.beforeEach(async ({ page }) => {
    await useScreenshotDataset(page);
  });

  test("WZdnw: Admin Settings — General", async ({ page }) => {
    await page.goto("/admin");
    await expect(page.getByRole("heading", { name: /admin settings/i })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByText(/instance name/i)).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "WZdnw");
  });

  test("uMiwd: Admin Settings — Authentication", async ({ page }) => {
    await page.goto("/admin");
    await clickSection(page, "Authentication");
    // Helm OIDC Provider Card (read-only, groups claim + default role).
    await expect(page.getByText("Helm-seeded OIDC provider")).toBeVisible({ timeout: 10_000 });
    // Role Mapping Overrides Card — admin row is dashboard-overridden.
    await expect(page.getByText("Role mapping overrides")).toBeVisible();
    await expect(page.getByText("ops-leads")).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "uMiwd");
  });

  test("R65Xyx: Provenance Badge — Overridden", async ({ page }) => {
    await page.goto("/admin");
    await clickSection(page, "Authentication");
    await expect(page.getByText("Overridden in dashboard")).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "R65Xyx");
  });

  test("Rwnu3: Provenance Badge — From Helm", async ({ page }) => {
    await page.goto("/admin");
    await clickSection(page, "Authentication");
    await expect(page.getByText("From Helm values").first()).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "Rwnu3");
  });

  test("XL5ZU: Removable Group Chip — Orange (admin)", async ({ page }) => {
    await page.goto("/admin");
    await clickSection(page, "Authentication");
    await expect(page.getByText("ops-leads")).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "XL5ZU");
  });

  test("vStkb: Removable Group Chip — Violet (operator)", async ({ page }) => {
    await page.goto("/admin");
    await clickSection(page, "Authentication");
    await expect(page.getByText("ops-team")).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "vStkb");
  });

  test("uw0dB: Removable Group Chip — Secondary (viewer)", async ({ page }) => {
    await page.goto("/admin");
    await clickSection(page, "Authentication");
    await expect(page.getByText("everyone")).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "uw0dB");
  });

  test("nNGDX: Admin Settings — Authentication (No OIDC mappings)", async ({ page }) => {
    // Empty-state variant: Helm-seeded provider exists but declares no
    // role mappings at all, and there's no dashboard override either.
    await page.route(/\/admin\/config$/, async (route) => {
      if (route.request().method() !== "GET") return route.fallback();
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          general: {
            instanceName: "My Gameplane Cluster",
            externalURL: "https://gameplane-demo.local",
            defaultNamespace: "default",
          },
          auth: { providers: [{ name: "local", kind: "local", enabled: true }] },
          modRegistries: { registries: [] },
          installTimeSettings: {
            gameDataStorageClass: "fast-nvme",
            oidcHelmProvider: { groupsClaim: "groups", defaultRole: "viewer" },
          },
        }),
      });
    });
    await page.goto("/admin");
    await clickSection(page, "Authentication");
    await expect(page.getByText("No OIDC role mappings yet")).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "nNGDX");
  });

  test("BV5ei: Provenance Badge — Not configured", async ({ page }) => {
    await page.route(/\/admin\/config$/, async (route) => {
      if (route.request().method() !== "GET") return route.fallback();
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          general: {
            instanceName: "My Gameplane Cluster",
            externalURL: "https://gameplane-demo.local",
            defaultNamespace: "default",
          },
          auth: { providers: [{ name: "local", kind: "local", enabled: true }] },
          modRegistries: { registries: [] },
          installTimeSettings: {
            gameDataStorageClass: "fast-nvme",
            oidcHelmProvider: { groupsClaim: "groups", defaultRole: "viewer" },
          },
        }),
      });
    });
    await page.goto("/admin");
    await clickSection(page, "Authentication");
    await expect(page.getByText("Not configured").first()).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "BV5ei");
  });

  test("QgW58: Admin Settings — Authentication (Save rejected)", async ({ page }) => {
    // NOTE: the design frame additionally shows a field-level red-stroked
    // input for the blank group name; the current form has no client-side
    // blank-name validation to trigger that state; this test reproduces
    // the card-level error banner half of the frame (the 400 response
    // surfaced via SaveStatus) by rejecting the section's PUT outright.
    await page.route(/\/admin\/config\/auth$/, async (route) => {
      if (route.request().method() !== "PUT") return route.fallback();
      await route.fulfill({
        status: 400,
        contentType: "application/json",
        body: JSON.stringify({
          error:
            "helmOverride.roleMappings.admin must not contain blank group names. No changes were saved.",
        }),
      });
    });
    await page.goto("/admin");
    await clickSection(page, "Authentication");
    await expect(page.getByText("Role mapping overrides")).toBeVisible({ timeout: 10_000 });
    await page.getByRole("button", { name: /save role mappings/i }).click();
    await expect(page.getByText(/must not contain blank group names/i)).toBeVisible({
      timeout: 10_000,
    });
    await page.waitForTimeout(200);
    await capture(page, "QgW58");
  });

  test("zqzr4: Admin Settings — Authentication (Admin mapping warning)", async ({ page }) => {
    // NOTE: the design frame (design-export/MANIFEST.md) describes an
    // inline AdminGroupsInlineWarning appearing under this field as soon
    // as an admin group is typed; the current AddProviderForm only warns
    // via ConfirmAdminMappingDialog on submit (see Kp48V) — the inline
    // banner isn't wired up yet. This capture reflects the shipped
    // markup (expanded form with an admin group entered) so the
    // screenshot gate has a real comparison point; the missing inline
    // warning is a content/behavior gap to flag separately, not something
    // this screenshot-only task adds.
    await page.goto("/admin");
    await clickSection(page, "Authentication");
    await page.getByRole("button", { name: /^add provider$/i }).click();
    await expect(page.getByText("Add identity provider")).toBeVisible({ timeout: 10_000 });
    await page.getByLabel("Admin groups").fill("ops-leads");
    await expect(page.getByLabel("Admin groups")).toHaveValue("ops-leads");
    await page.waitForTimeout(200);
    await capture(page, "zqzr4");
  });

  test("Kp48V: Confirm Admin Mapping dialog", async ({ page }) => {
    await page.goto("/admin");
    await clickSection(page, "Authentication");
    await page.getByRole("button", { name: /^add provider$/i }).click();
    await expect(page.getByText("Add identity provider")).toBeVisible({ timeout: 10_000 });
    await page.getByLabel("Name").fill("corp-sso");
    await page.getByLabel("Issuer URL").fill("https://idp.example.com");
    await page.getByLabel("Client ID").fill("abc123");
    await page.getByLabel("Client secret").fill("supersecret");
    await page.getByLabel("Admin groups").fill("ops-leads");
    await page.getByRole("button", { name: /^add provider$/i }).click();
    await expect(page.getByText("Confirm admin role mapping?")).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "Kp48V");
  });

  test("RC3Kf: Admin Settings — Backup destinations", async ({ page }) => {
    await page.goto("/admin");
    await clickSection(page, "Backup destinations");
    await expect(page.getByText(/restic repositories/i)).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "RC3Kf");
  });

  test("g5mEpx: Admin Settings — Module sources", async ({ page }) => {
    await page.goto("/admin");
    await clickSection(page, "Module sources");
    await expect(page.getByText("upstream")).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "g5mEpx");
  });

  test("Wj0V4: Admin Settings — Mod registries", async ({ page }) => {
    await page.goto("/admin");
    await clickSection(page, "Mod registries");
    await expect(page.getByText("CurseForge")).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "Wj0V4");
  });

  test("n6Xlo: Admin Settings — Notifications", async ({ page }) => {
    await page.goto("/admin");
    await clickSection(page, "Notifications");
    await expect(page.getByText(/no notification sinks configured/i)).toBeVisible({
      timeout: 10_000,
    });
    await page.waitForTimeout(200);
    await capture(page, "n6Xlo");
  });

  test("uoxQW: Admin Settings — Telemetry", async ({ page }) => {
    await page.goto("/admin");
    await clickSection(page, "Telemetry");
    await expect(page.getByText(/send anonymous usage metrics/i)).toBeVisible({
      timeout: 10_000,
    });
    await page.waitForTimeout(200);
    await capture(page, "uoxQW");
  });

  test("M2sA4u: Admin Settings — Updates", async ({ page }) => {
    await page.goto("/admin");
    await clickSection(page, "Updates");
    await expect(page.getByText(/upgraded via helm/i)).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "M2sA4u");
  });

  test("zM0VF: Admin Settings — About", async ({ page }) => {
    await page.goto("/admin");
    await clickSection(page, "About");
    await expect(page.getByText(/agpl-3\.0/i)).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "zM0VF");
  });

  test("bYDHC: Users & RBAC", async ({ page }) => {
    await page.goto("/users");
    await expect(page.getByRole("heading", { name: /users.*rbac/i })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByText("operator-01")).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "bYDHC");
  });

  test("e9lV4: Users & RBAC — Roles", async ({ page }) => {
    await page.goto("/users");
    await page.getByRole("tab", { name: /^roles/i }).click();
    await expect(page.getByText(/full access to all resources/i)).toBeVisible({
      timeout: 10_000,
    });
    await page.waitForTimeout(200);
    await capture(page, "e9lV4");
  });

  test("TBvTC: Users & RBAC — Service accounts", async ({ page }) => {
    await page.goto("/users");
    await page.getByRole("tab", { name: /^service accounts$/i }).click();
    await expect(page.getByText(/tracked for v1\.1/i)).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "TBvTC");
  });

  test("Dpb9f: Users & RBAC — Identity providers", async ({ page }) => {
    await page.goto("/users");
    await page.getByRole("tab", { name: /^identity providers$/i }).click();
    await expect(page.getByText(/oidc identity providers/i)).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "Dpb9f");
  });

  test("CqaSq: Role Editor Modal", async ({ page }) => {
    await page.goto("/users");
    await page.getByRole("tab", { name: /^roles/i }).click();
    await expect(page.getByRole("button", { name: /new role/i })).toBeVisible({
      timeout: 10_000,
    });
    await page.getByRole("button", { name: /new role/i }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible({ timeout: 10_000 });
    await expect(dialog.getByText("New role")).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "CqaSq");
  });

  test("NLDDv: Dialog — Invite User", async ({ page }) => {
    await page.goto("/users");
    await page.getByRole("button", { name: /invite user/i }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible({ timeout: 10_000 });
    await expect(dialog.getByText("Invite user")).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "NLDDv");
  });

  test("t3IY3u: Dialog — Edit User", async ({ page }) => {
    await page.goto("/users");
    await expect(page.getByText("operator-01").first()).toBeVisible({ timeout: 10_000 });
    await page.getByRole("button", { name: /^actions for operator-01$/i }).click();
    await page.getByRole("menuitem", { name: /edit user/i }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible({ timeout: 10_000 });
    await expect(dialog.getByText("Edit user")).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "t3IY3u");
  });

  test("MaoHP: Dialog — Reset Password", async ({ page }) => {
    await page.goto("/users");
    await expect(page.getByText("operator-01").first()).toBeVisible({ timeout: 10_000 });
    await page.getByRole("button", { name: /^actions for operator-01$/i }).click();
    await page.getByRole("menuitem", { name: /reset password/i }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible({ timeout: 10_000 });
    await expect(dialog.getByText(/reset password for/i)).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "MaoHP");
  });

  test("DxKOh: Audit Log", async ({ page }) => {
    await page.goto("/admin/audit");
    await expect(page.getByRole("heading", { name: /audit log/i })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByText("audit chain verified", { exact: false })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByText("admin-demo").first()).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "DxKOh");
  });

  test("kIxaJ: Audit Integrity Banner (failed)", async ({ page }) => {
    await page.route(/\/admin\/audit\/verify$/, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          ok: false,
          checked: 42,
          firstBadId: 17,
          message: "Integrity check failed — chain breaks at event #17",
        }),
      });
    });
    await page.goto("/admin/audit");
    await expect(page.getByRole("heading", { name: /audit log/i })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByText(/chain breaks at event #17/i)).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(200);
    await capture(page, "kIxaJ");
  });

  test("Bq2Yg: Admin — System Logs", async ({ page }) => {
    await page.goto("/admin/logs");
    await expect(page.getByRole("heading", { name: /system logs/i })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByRole("button", { name: /^api server$/i })).toBeVisible();
    // Mocked plaintext stream from /admin/system-logs/:component
    // (screenshotSystemLogLines).
    await expect(page.getByText(/starting gameplane-api/i)).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(500);
    await capture(page, "Bq2Yg");
  });

  test("j9W8A: Cluster Settings", async ({ page }) => {
    await page.goto("/cluster");
    await expect(page.getByRole("heading", { name: /cluster/i }).first()).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByText("Storage")).toBeVisible();
    await expect(page.getByText("fast-nvme")).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "j9W8A");
  });

  test("dxdEi: Cluster Settings — Default storage class", async ({ page }) => {
    await page.route(/\/admin\/config$/, async (route) => {
      if (route.request().method() !== "GET") return route.fallback();
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          general: {
            instanceName: "My Gameplane Cluster",
            externalURL: "https://gameplane-demo.local",
            defaultNamespace: "default",
          },
          auth: { providers: [{ name: "local", kind: "local", enabled: true }] },
          modRegistries: { registries: [] },
          installTimeSettings: { gameDataStorageClass: "" },
        }),
      });
    });
    await page.goto("/cluster");
    await expect(page.getByText("Storage")).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText("Cluster default")).toBeVisible();
    await page.waitForTimeout(200);
    await capture(page, "dxdEi");
  });
});
