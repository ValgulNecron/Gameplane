import { test, expect } from "@playwright/test";

test("mobile dashboard preserves long cluster and node identities without overflow", async ({ page, context }) => {
  test.skip(process.env.GAMEPLANE_E2E_TARGET === "live", "Uses deterministic cross-cluster inventory fixtures.");
  await page.setViewportSize({ width: 375, height: 900 });
  await context.addCookies([
    { name: "e2e_multicluster", value: "1", url: "http://localhost:5173" },
    { name: "e2e_long_node_names", value: "1", url: "http://localhost:5173" },
  ]);
  await page.goto("/");
  const main = page.getByRole("main");
  await expect(main.getByRole("heading", { name: "Dashboard", exact: true })).toBeVisible();
  const sites = ["local", "remote-location-with-a-long-registered-cluster-identifier"];
  for (const site of sites) {
    const identity = `${site} / ${site}-control-plane-with-a-long-unique-node-identifier`;
    const label = main.getByText(identity, { exact: true });
    await expect(label).toBeVisible();
    const geometry = await label.evaluate((element) => {
      const box = element.getBoundingClientRect();
      const styles = getComputedStyle(element);
      return { left: box.left, right: box.right, scroll: element.scrollWidth, width: element.clientWidth, overflow: styles.textOverflow, whitespace: styles.whiteSpace };
    });
    expect(geometry.left).toBeGreaterThanOrEqual(0);
    expect(geometry.right).toBeLessThanOrEqual(375);
    expect(geometry.scroll).toBeLessThanOrEqual(geometry.width);
    expect(geometry.overflow).not.toBe("ellipsis");
    expect(geometry.whitespace).not.toBe("nowrap");
  }
  expect(await main.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});
