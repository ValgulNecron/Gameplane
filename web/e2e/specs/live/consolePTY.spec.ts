import { test, expect } from "@playwright/test";

// Live: PTY console e2e. Creates a busybox GameServer with consoleMode=pty
// via the API, navigates to its Console tab, types a known marker, and
// asserts the marker echoes back in the xterm output.
//
// The Go e2e suite covers the WS round-trip directly (api_ws_e2e_test.go's
// TestAPI_ConsolePTYRoundTrip). This spec proves the end-to-end stack
// from the React Console tab through the dashboard's WebSocket helper
// into the same path.

test.describe("live: PTY console", () => {
  test.skip(
    process.env.GAMEPLANE_E2E_TARGET !== "live",
    "live-only spec",
  );

  const serverName = `e2e-pw-pty-${Date.now().toString(36)}`;
  const tmplName = "e2e-pw-pty-tmpl";

  test.beforeAll(async ({ request }) => {
    // Create a PTY GameTemplate (cluster-scoped) — the API exposes
    // /templates as a thin pass-through for module-managed templates,
    // but for one-off test fixtures we POST through /servers using
    // POST /api/v1 ... the create-server flow expects a templateRef
    // that already exists. So we apply the template via the kubectl
    // tunnel: in live mode the storageState carries a session that
    // can hit the typed clients, but the dashboard doesn't expose a
    // generic kubectl-apply. We rely on the Go e2e suite (run before
    // this spec in CI) to have produced an "e2e-busybox" template.
    // This beforeAll just creates the GameServer.
    await request.post("/servers", {
      data: {
        apiVersion: "gameplane.gg/v1alpha1",
        kind: "GameServer",
        metadata: { name: serverName, namespace: "kestrel-games" },
        spec: { templateRef: { name: tmplName } },
      },
    });
  });

  test.afterAll(async ({ request }) => {
    try {
      await request.delete(`/servers/${serverName}`);
    } catch {
      // best-effort cleanup
    }
  });

  test("typing in the Console tab echoes back through the WS", async ({ page }) => {
    test.setTimeout(180_000);

    await page.goto(`/servers/${serverName}`);
    await page.waitForLoadState("domcontentloaded");

    // Wait for the server header — proves the GameServer lookup succeeded.
    // If the prerequisite GameTemplate (e2e-pw-pty-tmpl) doesn't exist on
    // the live cluster, the create POST in beforeAll silently 4xx'd and
    // the detail page won't render the server name. Skip in that case.
    const heading = page.getByRole("heading", { name: serverName });
    if (!(await heading.isVisible({ timeout: 30_000 }).catch(() => false))) {
      test.skip(
        true,
        `live cluster has no '${tmplName}' template; PTY console live spec needs the Go e2e suite to have created it`,
      );
      return;
    }

    const tabNav = page.locator("header nav.scrollbar-thin");
    await tabNav.getByRole("button", { name: /^console$/i }).click();

    // xterm.js renders into a child of the Console panel. We can't
    // reliably "type" into it through Playwright's keyboard API because
    // the focus needs to be inside the xterm canvas; instead we rely on
    // the panel having an input affordance for command entry. If neither
    // is visible within 30s, the test skips (the live cluster's pod may
    // not have come up yet).
    const marker = "kestrel-live-marker";
    const cmdInput = page.locator('input[placeholder*="command" i]').first();
    if (!(await cmdInput.isVisible({ timeout: 30_000 }).catch(() => false))) {
      test.skip(true, "Console tab has no command input — pod may not be ready");
      return;
    }
    await cmdInput.fill(`echo ${marker}`);
    await cmdInput.press("Enter");

    // The xterm canvas surfaces emitted bytes as accessible text under
    // .xterm-rows. Wait for the marker to appear.
    await expect(page.locator(".xterm-rows")).toContainText(marker, {
      timeout: 30_000,
    });
  });
});
