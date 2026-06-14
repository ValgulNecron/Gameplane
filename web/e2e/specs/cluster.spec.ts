import { test, expect, type Page } from "@playwright/test";
import { LoginPage } from "../pages/LoginPage";

// Mirrors navigation.spec's helper: works in both mock (always
// authenticated) and live (redirects to /login until we sign in) modes.
async function loginIfNeeded(page: Page): Promise<void> {
  await page.goto("/");
  await page.waitForLoadState("domcontentloaded");
  if (new URL(page.url()).pathname.startsWith("/login")) {
    const login = new LoginPage(page);
    const username =
      process.env.ADMIN_USERNAME ?? process.env.KESTREL_E2E_ADMIN_USERNAME ?? "e2e-admin";
    const password =
      process.env.ADMIN_PASSWORD ?? process.env.KESTREL_E2E_ADMIN_PASSWORD ?? "any-non-empty";
    await login.login(username, password);
    await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 10_000 });
  }
}

test.describe("cluster page", () => {
  // Mock mode serves a deterministic 3-node view. Live mode's cluster data
  // depends on clusterOps being enabled on the e2e cluster, so the node
  // summary isn't guaranteed there — keep this spec deterministic.
  test.skip(
    process.env.KESTREL_E2E_TARGET === "live",
    "cluster view is only deterministic under mock mode",
  );

  test.beforeEach(async ({ page }) => {
    await loginIfNeeded(page);
  });

  test("renders the cluster header and node summary", async ({ page }) => {
    await page.goto("/cluster");
    await expect(page).toHaveURL(/\/cluster$/);
    await expect(page.getByRole("heading", { name: /cluster/i }).first()).toBeVisible();
    // The header subtitle always reports node health ("X/Y nodes healthy"),
    // independent of how many nodes the install actually has.
    await expect(page.getByText(/nodes healthy/i)).toBeVisible();
  });

  test("exposes the cluster operation actions", async ({ page }) => {
    await page.goto("/cluster");
    await expect(page.getByRole("button", { name: /add node/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /download kubeconfig/i })).toBeVisible();
  });
});
