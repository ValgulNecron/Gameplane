import { test, expect } from "@playwright/test";
import { loginIfNeeded, seedServer, seedTemplate } from "./_seed";

// Live: prove the data-bearing dashboard screens render REAL backend data,
// not MSW fixtures. The 17 main specs all skip in live mode and assert
// against MSW; this spec is the inverse — it seeds real CRDs/DB rows through
// the API and asserts the UI shows exactly those, end-to-end against the
// kestrel-e2e cluster.
//
// Screens with no setup cost (Cluster, Users/Roles, Modules) read whatever
// the live install already has. Screens that need a workload (Servers list,
// Server Settings) use a seeded GameServer; no running pod is required —
// the CR existing is enough for these to render real data. Agent-backed
// screens (Overview/Files/Players) need a live sidecar and live in
// liveAgentScreens.spec.ts.

test.describe("live: data screens render real backend data", () => {
  test.skip(
    process.env.KESTREL_E2E_TARGET !== "live",
    "live-only — the mock specs cover these screens against MSW",
  );

  const stamp = Date.now().toString(36);
  const tmplName = `e2e-pw-data-tmpl-${stamp}`;
  const serverName = `e2e-pw-data-${stamp}`;
  let cleanups: Array<() => Promise<void>> = [];

  test.beforeAll(async ({ request }) => {
    const tmpl = await seedTemplate(request, tmplName);
    const server = await seedServer(request, {
      name: serverName,
      template: tmplName,
      description: "Live data-screen probe",
    });
    // Delete the server before the template it references.
    cleanups = [server.cleanup, tmpl.cleanup];
  });

  test.afterAll(async () => {
    for (const c of cleanups) await c();
  });

  test.beforeEach(async ({ page }) => {
    await loginIfNeeded(page);
  });

  test("cluster page shows real node health", async ({ page }) => {
    await page.goto("/cluster");
    await expect(page).toHaveURL(/\/cluster$/);
    await expect(page.getByRole("heading", { name: /cluster/i }).first()).toBeVisible();
    // "X/Y nodes healthy" is computed from the live ClusterView (real K8s
    // node list) — the kind cluster always has at least its control-plane.
    await expect(page.getByText(/nodes healthy/i)).toBeVisible();
  });

  test("users page lists the bootstrapped admin and real builtin roles", async ({ page }) => {
    await page.goto("/users");
    await expect(page.getByRole("heading", { name: /users.*rbac/i })).toBeVisible();
    // The live admin is the account the Go e2e suite bootstrapped.
    await expect(page.getByText("e2e-admin").first()).toBeVisible();

    await page.getByRole("button", { name: /^roles/i }).first().click();
    // Builtin role descriptions seeded by migration 003_roles.sql.
    await expect(page.getByText(/full access to all resources/i)).toBeVisible();
    await expect(page.getByText(/read-only access/i)).toBeVisible();
  });

  test("servers list shows the seeded GameServer", async ({ page }) => {
    await page.goto("/servers");
    await expect(page.getByRole("heading", { name: /^servers$/i })).toBeVisible();
    // The list polls every 5s; the seeded server's row links to its detail.
    await expect(page.getByRole("link", { name: serverName })).toBeVisible({ timeout: 15_000 });
    // The Game column renders the real templateRef.
    await expect(page.getByText(tmplName).first()).toBeVisible();
  });

  test("server settings reflect the real CR spec", async ({ page }) => {
    await page.goto(`/servers/${serverName}`);
    await expect(page.getByRole("heading", { name: serverName })).toBeVisible({ timeout: 20_000 });

    const tabNav = page.locator("header nav.scrollbar-thin");
    await tabNav.getByRole("button", { name: /^Settings$/ }).click();

    // The General sub-section's disabled Template input carries the
    // templateRef we POSTed — proving the form is bound to the live CR, not a
    // fixture. Scope to the field's grid block (Field renders the label as a
    // div, not an associated <label>), then assert the input's value.
    const templateField = page.locator("div.grid", {
      has: page.getByText("Template", { exact: true }),
    });
    await expect(templateField.locator("input")).toHaveValue(tmplName, { timeout: 15_000 });
  });

  test("audit log shows the seeding mutations", async ({ page }) => {
    await page.goto("/admin/audit");
    await expect(page).toHaveURL(/\/admin\/audit$/);
    await expect(page.getByRole("heading", { name: /audit log/i })).toBeVisible();

    // Seeding issued POST /templates and POST /servers as e2e-admin, so the
    // log is non-empty and carries those real rows.
    await expect(page.getByText("No audit events yet.")).toHaveCount(0);
    await expect(page.getByText("e2e-admin").first()).toBeVisible();
    await expect(page.getByText("/servers").first()).toBeVisible();
  });

  test("modules page renders the real catalog state", async ({ page }) => {
    await page.goto("/modules");
    await expect(page.getByRole("heading", { name: /^modules$/i })).toBeVisible();
    // Either the merged catalog has entries (a source filter chip renders)
    // or it's genuinely empty — both are the page faithfully reflecting real
    // ModuleSource state, never a fabricated list.
    await expect(
      page.getByText(/no modules in any catalog yet/i).or(page.getByText(/all sources/i)),
    ).toBeVisible();
  });
});
