import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Audit, Auth, Players, Servers, Templates, Users } from "./endpoints";

const fetchMock = vi.fn();

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
  fetchMock.mockImplementation(
    async () =>
      new Response(JSON.stringify({ items: [] }), {
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
  headers?: Record<string, string>;
  body?: string;
}

function called(): { url: string; init: FetchInit } {
  const last = fetchMock.mock.calls.at(-1);
  expect(last).toBeDefined();
  return { url: last![0] as string, init: last![1] as FetchInit };
}

describe("endpoints", () => {
  it("Servers.list hits /servers GET", async () => {
    await Servers.list();
    expect(called().url).toBe("/servers");
    expect(called().init.method).toBe("GET");
  });

  it("Servers.lifecycle hits the verb path with POST", async () => {
    await Servers.lifecycle("mc-survival", "restart");
    expect(called().url).toBe("/servers/mc-survival:restart");
    expect(called().init.method).toBe("POST");
  });

  it("Servers.create POSTs JSON body", async () => {
    await Servers.create({
      name: "x",
      templateRef: { name: "minecraft-java" },
    });
    const c = called();
    expect(c.url).toBe("/servers");
    expect(c.init.method).toBe("POST");
    expect(typeof c.init.body).toBe("string");
  });

  it("Templates.get encodes the name", async () => {
    await Templates.get("valheim");
    expect(called().url).toBe("/templates/valheim");
  });

  it("Players endpoints carry server name and action", async () => {
    await Players.snapshot("s1");
    expect(called().url).toBe("/servers/s1/players");
    await Players.banned("s1");
    expect(called().url).toBe("/servers/s1/players/banned");
    await Players.moderate("s1", "kick", { name: "alice" });
    expect(called().url).toBe("/servers/s1/players/kick");
    expect(called().init.method).toBe("POST");
  });

  it("Audit.page sends limit and before in querystring", async () => {
    await Audit.page(50, 0);
    expect(called().url).toBe("/admin/audit?limit=50");
    await Audit.page(50, 999);
    expect(called().url).toBe("/admin/audit?limit=50&before=999");
  });

  it("Auth.login POSTs to /auth/login", async () => {
    await Auth.login({ username: "u", password: "p" });
    expect(called().url).toBe("/auth/login");
    expect(called().init.method).toBe("POST");
  });

  it("Users.remove DELETEs by id", async () => {
    await Users.remove(42);
    expect(called().url).toBe("/users/42");
    expect(called().init.method).toBe("DELETE");
  });
});
