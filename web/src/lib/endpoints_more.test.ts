import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  Auth,
  BackupDestinations,
  Backups,
  Cluster,
  Logs,
  Modules,
  ModuleSources,
  Restores,
  Roles,
  Schedules,
  Servers,
  Templates,
  Users,
} from "./endpoints";

const fetchMock = vi.fn();

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
  fetchMock.mockImplementation(
    async () =>
      new Response(JSON.stringify({ items: [], spec: {} }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
  );
});

afterEach(() => {
  fetchMock.mockReset();
  vi.unstubAllGlobals();
});

interface FetchInit {
  method?: string;
  body?: BodyInit;
}

function last(): { url: string; init: FetchInit } {
  const c = fetchMock.mock.calls.at(-1);
  expect(c).toBeDefined();
  return { url: c![0] as string, init: (c![1] ?? {}) as FetchInit };
}

// Asserts the most recent fetch hit `url` with `method` (GET when omitted).
async function expectCall(p: Promise<unknown>, url: string, method = "GET") {
  await p;
  const c = last();
  expect(c.url).toBe(url);
  expect(c.init.method ?? "GET").toBe(method);
}

describe("Servers endpoints (uncovered)", () => {
  it("get / update / remove", async () => {
    await expectCall(Servers.get("s1"), "/servers/s1");
    await expectCall(Servers.update("s1", {} as never), "/servers/s1", "PUT");
    await expectCall(Servers.remove("s1"), "/servers/s1", "DELETE");
  });
  it("wipeData / transfer carry confirm/userId bodies", async () => {
    await Servers.wipeData("s1", "s1");
    expect(last().url).toBe("/servers/s1:wipe-data");
    expect(last().init.body).toBe(JSON.stringify({ confirm: "s1" }));
    await Servers.transfer("s1", 7);
    expect(last().url).toBe("/servers/s1:transfer");
    expect(last().init.body).toBe(JSON.stringify({ userId: 7 }));
  });
  it("status / runAction / mods family", async () => {
    await expectCall(Servers.status("s1"), "/servers/s1/status");
    await expectCall(Servers.runAction("s1", { id: "save" }), "/servers/s1/actions/run", "POST");
    await expectCall(Servers.mods("s1"), "/servers/s1/mods");
    await expectCall(Servers.installMod("s1", { url: "u" }), "/servers/s1/mods/install", "POST");
    await expectCall(Servers.removeMod("s1", "a b"), "/servers/s1/mods?name=a%20b", "DELETE");
  });
});

describe("Templates / Cluster", () => {
  it("Templates.list", async () => {
    await expectCall(Templates.list(), "/templates");
  });
  it("Cluster info / stats / view / addNode", async () => {
    await expectCall(Cluster.info(), "/cluster/info");
    await expectCall(Cluster.stats(), "/cluster/stats");
    await expectCall(Cluster.view(), "/cluster");
    await expectCall(Cluster.addNode(), "/cluster/nodes:join", "POST");
  });
  it("Cluster.kubeconfig POSTs and returns a blob", async () => {
    fetchMock.mockImplementation(async () => new Response("kubeconfig-bytes", { status: 200 }));
    const blob = await Cluster.kubeconfig();
    // (instanceof Blob is unreliable across the undici/jsdom realm boundary;
    // assert on the content instead.)
    expect(await blob.text()).toBe("kubeconfig-bytes");
    expect(last().url).toBe("/cluster/kubeconfig");
    expect(last().init.method).toBe("POST");
  });
});

describe("Backups / Schedules / Restores / Destinations", () => {
  it("Backups list/get/create/remove", async () => {
    await expectCall(Backups.list(), "/backups");
    await expectCall(Backups.get("b1"), "/backups/b1");
    await expectCall(Backups.create({ serverRef: { name: "s1" } }), "/backups", "POST");
    await expectCall(Backups.remove("b1"), "/backups/b1", "DELETE");
  });
  it("Schedules list/get/create/remove and patchSpec read-modify-write", async () => {
    await expectCall(Schedules.list(), "/schedules");
    await expectCall(Schedules.get("sc1"), "/schedules/sc1");
    await expectCall(
      Schedules.create({ serverRef: { name: "s1" }, schedule: "0 3 * * *", repoRef: { name: "r", key: "repo" } }),
      "/schedules",
      "POST",
    );
    // patchSpec GETs then PUTs; the last call is the PUT.
    await expectCall(Schedules.patchSpec("sc1", { suspend: true }), "/schedules/sc1", "PUT");
    await expectCall(Schedules.remove("sc1"), "/schedules/sc1", "DELETE");
  });
  it("Restores list/create/remove", async () => {
    await expectCall(Restores.list(), "/restores");
    await expectCall(
      Restores.create({ backupRef: { name: "b1" }, serverRef: { name: "s1" } }),
      "/restores",
      "POST",
    );
    await expectCall(Restores.remove("r1"), "/restores/r1", "DELETE");
  });
  it("BackupDestinations list/get/upsert/remove", async () => {
    await expectCall(BackupDestinations.list(), "/backup-destinations");
    await expectCall(BackupDestinations.get("d1"), "/backup-destinations/d1");
    await expectCall(
      BackupDestinations.upsert({ name: "d1", url: "s3:x", password: "pw" }),
      "/backup-destinations",
      "POST",
    );
    await expectCall(BackupDestinations.remove("d1"), "/backup-destinations/d1", "DELETE");
  });
});

describe("Users / Roles / Auth", () => {
  it("Users me/list/create/update/resetPassword/bindings", async () => {
    await expectCall(Users.me(), "/users/me");
    await expectCall(Users.list(), "/users");
    await expectCall(Users.create({ username: "u", role: "viewer" }), "/users", "POST");
    await expectCall(Users.update(3, { role: "operator" }), "/users/3", "PATCH");
    await expectCall(Users.resetPassword(3, "longenoughpw1"), "/users/3/reset-password", "POST");
    await expectCall(Users.bindings(3), "/users/3/bindings");
    await expectCall(
      Users.addBinding(3, { roleName: "operator", namespace: "team-a" }),
      "/users/3/bindings",
      "POST",
    );
    await expectCall(Users.removeBinding(3, "operator", "team-a"), "/users/3/bindings/operator/team-a", "DELETE");
  });
  it("Roles list/catalog/create/update/remove", async () => {
    await expectCall(Roles.list(), "/roles");
    await expectCall(Roles.catalog(), "/roles/permissions");
    await expectCall(Roles.create({ name: "support", permissions: ["servers:read"] }), "/roles", "POST");
    await expectCall(Roles.update("support", { permissions: [] }), "/roles/support", "PATCH");
    await expectCall(Roles.remove("support"), "/roles/support", "DELETE");
  });
  it("Auth logout / providers / oidcStartURL", async () => {
    await expectCall(Auth.logout(), "/auth/logout", "POST");
    await expectCall(Auth.providers(), "/auth/providers");
    expect(Auth.oidcStartURL()).toBe("/auth/oidc/start");
  });
});

describe("Modules / ModuleSources / Logs", () => {
  it("Modules catalog/list/get/install/upgrade/uninstall", async () => {
    await expectCall(Modules.catalog(), "/modules/catalog");
    await expectCall(Modules.list(), "/modules");
    await expectCall(Modules.get("m1"), "/modules/m1");
    await expectCall(Modules.install({ source: "up", module: "mc" }), "/modules", "POST");
    await expectCall(Modules.upgrade("m1", "1.2"), "/modules/m1", "PATCH");
    await expectCall(Modules.uninstall("m1"), "/modules/m1", "DELETE");
  });
  it("ModuleSources list/create/update/remove/removeUpload", async () => {
    await expectCall(ModuleSources.list(), "/modules/sources");
    await expectCall(
      ModuleSources.create("up", { type: "oci" } as never),
      "/modules/sources",
      "POST",
    );
    await expectCall(ModuleSources.update("up", { type: "oci" } as never), "/modules/sources/up", "PUT");
    await expectCall(ModuleSources.remove("up"), "/modules/sources/up", "DELETE");
    await expectCall(ModuleSources.removeUpload("up", "mc"), "/modules/sources/up/upload/mc", "DELETE");
  });
  it("ModuleSources.upload POSTs the bundle (dry-run query)", async () => {
    const file = new Blob(["bundle"]);
    await ModuleSources.upload("up", file, { dryRun: true });
    expect(last().url).toBe("/modules/sources/up/upload?dryRun=true");
    expect(last().init.method).toBe("POST");
    await ModuleSources.upload("up", file);
    expect(last().url).toBe("/modules/sources/up/upload");
  });
  it("Logs stream paths are server-encoded", () => {
    expect(Logs.fileStreamPath("a b")).toBe("/ws/servers/a%20b/logs");
    expect(Logs.podStreamPath("a b")).toBe("/ws/servers/a%20b/logs/pod?from=start");
  });
});
