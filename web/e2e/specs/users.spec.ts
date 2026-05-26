import { test, expect, type Page } from "@playwright/test";
import { LoginPage } from "../pages/LoginPage";

// Users / RBAC / Audit-log flows. Covers:
//   - The Users page list rendering from /users.
//   - The Invite dialog driving POST /users.
//   - Tab switching to Roles and back.
//   - The Audit log page rendering rows from /admin/audit.

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

test.describe("users page", () => {
  test.skip(
    process.env.KESTREL_E2E_TARGET === "live",
    "user create/delete in live mode would persist to the API DB; mock mode is the safe path",
  );

  test.beforeEach(async ({ page }) => {
    await loginIfNeeded(page);
  });

  test("lists users from /users", async ({ page }) => {
    await page.goto("/users");
    await page.waitForLoadState("domcontentloaded");

    await expect(page.getByRole("heading", { name: /users.*rbac/i })).toBeVisible();

    // The MSW seed returns three rows: admin, operator-bob, viewer-carol.
    await expect(page.getByText("admin").first()).toBeVisible();
    await expect(page.getByText("operator-bob")).toBeVisible();
    await expect(page.getByText("viewer-carol")).toBeVisible();
  });

  test("invite dialog submits POST /users", async ({ page }) => {
    await page.goto("/users");
    await page.waitForLoadState("domcontentloaded");

    await page.getByRole("button", { name: /invite user/i }).click();

    // Modal opens via Radix Dialog. Fill required fields. The dialog
    // has three text inputs whose placeholders all contain "alice"
    // ("alice", "Alice Operator", "alice@example.com"); anchor the
    // regex so we only match the username field's exact placeholder.
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await dialog.getByPlaceholder(/^alice$/i).fill("e2e-invitee");
    // Password field: needs ≥12 chars to pass client-side validation.
    await dialog.locator('input[type="password"]').fill("e2e-mock-pw-1234");

    const created = page.waitForRequest(
      (req) => req.url().endsWith("/users") && req.method() === "POST",
    );
    await dialog.getByRole("button", { name: /create user/i }).click();
    const req = await created;
    const body = req.postDataJSON() as { username?: string; role?: string };
    expect(body.username).toBe("e2e-invitee");
    expect(body.role).toBeTruthy();
  });

  test("switches between Users and Roles tabs", async ({ page }) => {
    await page.goto("/users");

    // The TabBar component renders sub-tabs; click "Roles" and verify
    // the role cards render.
    await page.getByRole("button", { name: /^roles/i }).first().click();
    await expect(page.getByText(/full access to all resources/i)).toBeVisible();

    // Back to Users.
    await page.getByRole("button", { name: /^users/i }).first().click();
    await expect(page.getByText("operator-bob")).toBeVisible();
  });
});

test.describe("audit log page", () => {
  test.skip(
    process.env.KESTREL_E2E_TARGET === "live",
    "audit log on a fresh live cluster has variable rows; mock mode is the deterministic path",
  );

  test.beforeEach(async ({ page }) => {
    await loginIfNeeded(page);
  });

  test("renders audit rows from /admin/audit", async ({ page }) => {
    await page.goto("/admin/audit");
    await page.waitForLoadState("domcontentloaded");

    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    // MSW seeds five audit events; at least one POST and one 403 row
    // appear if filters are off.
    await expect(page.getByText(/api\/v1\/servers\/alpha/).first()).toBeVisible();
  });
});
