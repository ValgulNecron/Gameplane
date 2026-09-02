import { test, expect, type Page } from "@playwright/test";
import path from "path";
import { fileURLToPath } from "url";
import fs from "fs";
import { LoginPage } from "../pages/LoginPage";

const __filename = fileURLToPath(import.meta.url);
const OUT = path.resolve(path.dirname(__filename), "../../../docs/img");

test.describe("@screenshots dashboard gallery", () => {
  test.use({
    viewport: { width: 1920, height: 1080 },
    deviceScaleFactor: 1,
    timezoneId: "UTC",
    colorScheme: "light",
  });

  // Helper to capture screenshot at 1920×1080 JPEG
  async function shoot(page: Page, name: string): Promise<void> {
    await page.waitForLoadState("load");
    await page.waitForTimeout(400);
    await page.screenshot({
      path: path.join(OUT, name + ".jpg"),
      type: "jpeg",
      quality: 80,
      fullPage: false,
    });
  }

  // Ensure output directory exists before any test runs
  test.beforeAll(async () => {
    fs.mkdirSync(OUT, { recursive: true });
  });

  test("capture gallery", async ({ page }) => {
    // Enable screenshot dataset BEFORE any navigation
    // This sets the localStorage flag that triggers MSW to load screenshot handlers
    await page.addInitScript(() => {
      try {
        localStorage.setItem("gameplane-e2e-dataset", "screenshots");
      } catch {}
    });

    // ============================================================
    // 1. LOGIN SCREEN (before authentication)
    // ============================================================
    // Force 401 on /users/me so the SPA doesn't redirect to dashboard before
    // we can screenshot the login page. The cookie survives the navigation and
    // is needed before visiting /login because mock mode answers /users/me
    // immediately upon page load.
    await page.context().addCookies([
      { name: "e2e_force_401", value: "1", url: "http://localhost:5173" },
    ]);
    await page.goto("/login");
    await expect(
      page.getByRole("textbox", { name: /Email or username/i })
    ).toBeVisible();
    await shoot(page, "login");

    // ============================================================
    // 2. AUTHENTICATE
    // ============================================================
    // Clear the force-401 cookie so login.login() succeeds
    await page.context().clearCookies();
    const login = new LoginPage(page);
    const username =
      process.env.ADMIN_USERNAME ?? process.env.GAMEPLANE_E2E_ADMIN_USERNAME ?? "e2e-admin";
    const password =
      process.env.ADMIN_PASSWORD ?? process.env.GAMEPLANE_E2E_ADMIN_PASSWORD ?? "any-non-empty";
    await login.login(username, password);
    await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 10_000 });

    // ============================================================
    // 3. DASHBOARD (/): Fleet health overview
    // ============================================================
    await page.goto("/");
    // Wait for the sidebar navigation to render (Modules link is always visible after hydration)
    await expect(page.getByRole("link", { name: /modules/i }).first()).toBeVisible();
    await page.waitForTimeout(250); // Layout settle
    await shoot(page, "dashboard");

    // ============================================================
    // 4. SERVERS LIST (/servers): Game server table
    // ============================================================
    await page.goto("/servers");
    // Wait for page heading or table to be visible
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await page.waitForTimeout(250);
    await shoot(page, "servers-list");

    // ============================================================
    // 5. SERVER OVERVIEW (/servers/test-server-01)
    // ============================================================
    await page.goto("/servers/test-server-01");
    // Click Overview tab to ensure we're on the right tab
    await page.getByRole("button", { name: "Overview" }).click();
    // Wait for overview tab panel or a distinctive metric/heading
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await page.waitForTimeout(250);
    await shoot(page, "server-overview");

    // ============================================================
    // 6. SERVER MODS TAB (/servers/test-server-01)
    // ============================================================
    await page.goto("/servers/test-server-01");
    // Click Mods tab to navigate to the correct tab
    await page.getByRole("button", { name: "Mods" }).click();
    // Wait for mods grid or a heading indicating mods are loaded
    await expect(page.locator('[role="tabpanel"]')).toBeVisible();
    await page.waitForTimeout(250);
    await shoot(page, "mods-registry-browse");

    // ============================================================
    // 7. SERVER CONSOLE TAB (/servers/test-server-01)
    // ============================================================
    await page.goto("/servers/test-server-01");
    // Click Console tab to navigate to the correct tab
    await page.getByRole("button", { name: "Console" }).click();
    // Wait for actual console output from the WebSocket mock stream
    await expect(page.getByText("joined the game").first()).toBeVisible({
      timeout: 15_000,
    });
    await shoot(page, "server-console");

    // ============================================================
    // 8. CREATE SERVER WIZARD - TEMPLATE SELECTION (/servers/new)
    // ============================================================
    await page.goto("/servers/new");
    // Wait for template grid or wizard heading
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await page.waitForTimeout(250);
    await shoot(page, "create-server-template-select");

    // ============================================================
    // 9. ADMIN SETTINGS - MOD REGISTRIES (/admin)
    // ============================================================
    await page.goto("/admin");
    // Click Mod registries button to navigate to the correct section
    await page.getByRole("button", { name: "Mod registries" }).click();
    // Wait for admin settings section to load
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await page.waitForTimeout(250);
    await shoot(page, "admin-mod-registries");

    // ============================================================
    // 10. SERVER EVENTS TAB (test-server-04 with FAILED phase)
    // ============================================================
    await page.goto("/servers/test-server-04");
    // Click Events tab to navigate to the correct tab
    await page.getByRole("button", { name: "Events" }).click();
    // Wait for events tab panel or event list
    await expect(page.locator('[role="tabpanel"]')).toBeVisible();
    await page.waitForTimeout(250);
    await shoot(page, "server-detail-events");

    // ============================================================
    // 11. ADMIN SETTINGS - GENERAL (/admin)
    // ============================================================
    await page.goto("/admin");
    // Wait for admin general form or heading (General is the default section)
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await page.waitForTimeout(250);
    await shoot(page, "admin-settings-general");

    // ============================================================
    // 12. CLUSTER PAGE (/cluster): Node list and stats
    // ============================================================
    await page.goto("/cluster");
    // Wait for cluster heading or node list
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await page.waitForTimeout(250);
    await shoot(page, "cluster-nodes");

    // ============================================================
    // 13. SERVER LOGS TAB (/servers/test-server-01)
    // ============================================================
    await page.goto("/servers/test-server-01");
    // Click Logs tab to navigate to the correct tab
    await page.getByRole("button", { name: "Logs" }).click();
    // Wait for actual log output from the WebSocket mock stream
    await expect(page.getByText("joined the game").first()).toBeVisible({
      timeout: 15_000,
    });
    await shoot(page, "server-detail-logs");
  });
});
