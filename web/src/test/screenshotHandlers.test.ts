import { beforeEach, describe, expect, it } from "vitest";
import { Fleet } from "@/lib/fleet";
import { server } from "./server";
import { buildScreenshotHandlers } from "./handlers";
import { getScreenshotData } from "./screenshotData";

describe("screenshot dataset fleet adapters", () => {
  beforeEach(() => {
    // Browser screenshot mode uses this handler set exclusively. Keep defaults
    // out of this test so a missing adapter cannot silently use ordinary data.
    server.resetHandlers(...buildScreenshotHandlers());
  });

  it("returns the enriched screenshot resources through every fleet list", async () => {
    const data = getScreenshotData();
    const [servers, backups, schedules, restores] = await Promise.all([
      Fleet.servers(), Fleet.backups(), Fleet.schedules(), Fleet.restores(),
    ]);
    const results = [servers, backups, schedules, restores];
    const expected = [
      data.servers.map((item) => item.metadata.name),
      ["mc-survival-nightly-0713", "mc-survival-nightly-0712"],
      data.schedules.map((item) => item.metadata.name),
      data.restores.map((item) => item.metadata.name),
    ];
    for (const [index, result] of results.entries()) {
      expect(result.partial).toBe(false);
      expect(result.items.map((item) => item.target.name)).toEqual(expected[index]);
      for (const item of result.items) {
        expect(item.target.cluster).toBe("local");
        expect(item.target.namespace).toBe(item.resource.metadata.namespace);
        expect(item.target.uid).toBeTruthy();
        expect(item.permissions).toContain("*");
      }
    }
  });

  it("registers inventory discovery, placement and exact server access", async () => {
    const data = getScreenshotData();
    const [inventory, placements, accessResponse] = await Promise.all([
      Fleet.inventory(), Fleet.placements(),
      fetch("/servers/mc-survival/access?namespace=gameplane-games"),
    ]);
    expect(inventory.partial).toBe(false);
    expect(inventory.items).toHaveLength(1);
    expect(inventory.items[0].view.nodes?.map((node) => node.name)).toEqual(data.clusterView().nodes?.map((node) => node.name));
    expect(placements.items).toHaveLength(1);
    expect(placements.items[0].cluster).toBe("local");
    expect(placements.items[0].namespace).toBe("gameplane-games");
    expect(placements.items[0].templates.map((item) => item.metadata.name)).toEqual(data.templates.map((item) => item.metadata.name));
    expect(accessResponse.ok).toBe(true);
    const access = await accessResponse.json();
    expect(access.target).toMatchObject({ cluster: "local", namespace: "gameplane-games", name: "mc-survival" });
    expect(access.isOwner).toBe(true);
    expect(access.canWrite).toBe(true);
  });
});
