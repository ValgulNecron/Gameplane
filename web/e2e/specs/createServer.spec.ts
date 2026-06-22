import { test, expect, type Page } from "@playwright/test";
import { LoginPage } from "../pages/LoginPage";

// Create-server wizard. Mock mode walks the four-step wizard against
// MSW handlers; live mode is skipped because the wizard creates real
// CRDs and we don't want this spec to leave debris on the cluster (the
// dedicated live spec under live/ owns end-to-end create+cleanup).

async function loginIfNeeded(page: Page): Promise<void> {
  await page.goto("/");
  await page.waitForLoadState("domcontentloaded");
  if (new URL(page.url()).pathname.startsWith("/login")) {
    const login = new LoginPage(page);
    const username =
      process.env.ADMIN_USERNAME ?? process.env.GAMEPLANE_E2E_ADMIN_USERNAME ?? "e2e-admin";
    const password =
      process.env.ADMIN_PASSWORD ?? process.env.GAMEPLANE_E2E_ADMIN_PASSWORD ?? "any-non-empty";
    await login.login(username, password);
    await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 10_000 });
  }
}

test.describe("create-server wizard", () => {
  test.skip(
    process.env.GAMEPLANE_E2E_TARGET === "live",
    "live mode is covered by specs/live/createAndCleanupServer.spec.ts",
  );

  test.beforeEach(async ({ page }) => {
    await loginIfNeeded(page);
  });

  test("walks step 1 → step 2 → step 4 and submits", async ({ page }) => {
    await page.goto("/servers/new");
    await page.waitForLoadState("domcontentloaded");
    await expect(page.getByText(/new game server/i)).toBeVisible();

    // Step 1: Pick a template. The first MSW template has displayName
    // "Minecraft (Vanilla)" — clicking the card selects it. Continue
    // unlocks once a card is selected.
    const continueBtn = page.getByRole("button", { name: /continue/i });
    await expect(continueBtn).toBeDisabled();

    await page.getByRole("button", { name: /minecraft \(vanilla\)/i }).click();
    await expect(continueBtn).toBeEnabled();
    await continueBtn.click();

    // Step 2: Configure. Empty name fails validation; the page surfaces
    // the reason in data-testid="step-reason" and disables Continue.
    await expect(page.locator('[data-testid="step-reason"]')).toBeVisible();
    await page.getByPlaceholder(/mc-hardcore/i).fill("e2e-mock-srv");

    // Step 3: Network — defaults are fine. Click through.
    await continueBtn.click(); // configure → network
    await continueBtn.click(); // network → review

    // Step 4: Review shows the row table; submit creates the server and
    // navigates to /servers/<name>.
    const createBtn = page.getByRole("button", { name: /create server/i });
    const apiCall = page.waitForRequest(
      (req) => req.url().endsWith("/servers") && req.method() === "POST",
    );
    await createBtn.click();
    await apiCall;

    // The wizard's onSuccess navigates to /servers/{name}. Some MSW
    // setups can briefly settle on "/" first; assert the eventual URL.
    await page.waitForURL(/\/servers\/e2e-mock-srv$/, { timeout: 10_000 });
  });

  test("rejects an invalid kubernetes name", async ({ page }) => {
    await page.goto("/servers/new");
    await page.getByRole("button", { name: /minecraft \(vanilla\)/i }).click();
    await page.getByRole("button", { name: /continue/i }).click();

    // Uppercase isn't a valid k8s DNS label. Continue must stay disabled
    // and step-reason must explain why.
    await page.getByPlaceholder(/mc-hardcore/i).fill("Bad_Name");
    const reason = page.locator('[data-testid="step-reason"]');
    await expect(reason).toBeVisible();
    await expect(reason).toContainText(/lowercase|name/i);
    await expect(page.getByRole("button", { name: /continue/i })).toBeDisabled();
  });
});
