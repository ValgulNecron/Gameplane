import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { renderWithQuery } from "@/test/render";
import { ModpacksTab } from "./Modpacks";
import type { GameServer, GameTemplate, RegistryProject } from "@/types";

const fetchMock = vi.fn();
beforeEach(() => vi.stubGlobal("fetch", fetchMock));
afterEach(() => {
  fetchMock.mockReset();
  vi.unstubAllGlobals();
});

function jsonRes(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const operator = {
  id: 1, username: "u", displayName: "U", email: "", role: "operator",
  permissions: { "*": ["servers:read", "servers:write"] },
};
const viewer = { ...operator, role: "viewer", permissions: { "*": ["servers:read"] } };

type Modpacks = { refEnv?: string; env?: { name: string; value?: string }[] };

interface Routes {
  me?: typeof operator;
  packs?: RegistryProject[];
  deps?: { filename: string; downloadUrl: string }[];
  providers?: { provider: string; available: boolean; modpacks: boolean }[];
  onModpack?: (body: { ref: string }, provider: string | null) => void;
  onInstallMod?: (body: { url: string; name?: string }) => void;
}

function route(r: Routes) {
  fetchMock.mockImplementation((url: string, opts?: { method?: string; body?: string }) => {
    const method = opts?.method ?? "GET";
    if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(r.me ?? operator));
    if (url.includes("/mods/registry/providers"))
      return Promise.resolve(jsonRes(r.providers ?? [{ provider: "modrinth", available: true, modpacks: true }]));
    if (url.includes("/mods/registry/search")) return Promise.resolve(jsonRes(r.packs ?? []));
    if (url.includes("/mods/registry/projects/") && url.includes("/modpack"))
      return Promise.resolve(jsonRes(r.deps ?? []));
    if (url.includes("/modpack") && method === "POST") {
      const provider = new URL("http://x" + url.slice(url.indexOf("/"))).searchParams.get("provider");
      r.onModpack?.(JSON.parse(opts?.body ?? "{}"), provider);
      return Promise.resolve(jsonRes({ ok: true }));
    }
    if (url.includes("/mods/install")) {
      const body = JSON.parse(opts?.body ?? "{}");
      r.onInstallMod?.(body);
      return Promise.resolve(jsonRes({ name: body.name ?? "x", size: 1 }));
    }
    return Promise.resolve(jsonRes({}));
  });
}

function tmpl(modpacks: Modpacks, provider = "modrinth"): GameTemplate {
  return {
    metadata: { name: "t" },
    spec: {
      displayName: "T", game: "g", version: "1", image: "img",
      capabilities: {
        mods: { loaders: { l: { path: "mods" } }, registry: { providers: [{ provider: provider as "modrinth", modpacks }] } },
      },
    },
  };
}

const providersOf = (provider: string) => [{ provider, available: true, modpacks: true }];

function gs(env?: { name: string; value?: string }[]): GameServer {
  return { metadata: { name: "s1" }, spec: { templateRef: { name: "t" }, ...(env ? { env } : {}) } };
}

const pack: RegistryProject = {
  id: "cobblemon", slug: "cobblemon", title: "Cobblemon", author: "cobblemon",
  downloads: 9_100_000, provider: "modrinth",
};

describe("ModpacksTab", () => {
  it("env-mode: installs by setting the modpack ref", async () => {
    const calls: { ref: string }[] = [];
    route({ packs: [pack], onModpack: (b) => calls.push(b) });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({ refEnv: "MODRINTH_MODPACK" })} gs={gs()} />);

    await screen.findByText("Cobblemon");
    const install = await screen.findByRole("button", { name: /install/i });
    fireEvent.click(install);
    await waitFor(() => expect(calls).toEqual([{ ref: "cobblemon" }]));
    expect(await screen.findByText(/Set modpack Cobblemon/)).toBeInTheDocument();
  });

  it("env-mode: shows the active modpack banner", async () => {
    route({ packs: [] });
    renderWithQuery(
      <ModpacksTab name="s1" tmpl={tmpl({ refEnv: "MODRINTH_MODPACK" })} gs={gs([{ name: "MODRINTH_MODPACK", value: "cobblemon" }])} />,
    );
    expect(await screen.findByText("Active modpack:")).toBeInTheDocument();
    expect(screen.getByText("cobblemon")).toBeInTheDocument();
  });

  it("deps-mode: resolves and installs each dependency", async () => {
    const installed: Array<{ url: string; name?: string }> = [];
    route({
      providers: providersOf("thunderstore"),
      packs: [{ ...pack, title: "MegaPack", id: "packer-MegaPack" }],
      deps: [
        { filename: "a.zip", downloadUrl: "https://cdn/a.zip" },
        { filename: "b.zip", downloadUrl: "https://cdn/b.zip" },
      ],
      onInstallMod: (b) => installed.push(b),
    });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({}, "thunderstore")} gs={gs()} />);

    fireEvent.click(await screen.findByRole("button", { name: /install/i }));
    await waitFor(() => expect(installed).toHaveLength(2));
    expect(installed[0]).toEqual({ url: "https://cdn/a.zip", name: "a.zip" });
    expect(await screen.findByText(/Installed MegaPack — 2 mods/)).toBeInTheDocument();
  });

  it("disables install for viewers", async () => {
    route({ me: viewer, packs: [pack] });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({ refEnv: "MODRINTH_MODPACK" })} gs={gs()} />);
    await screen.findByText("Cobblemon");
    expect(screen.getByRole("button", { name: /install/i })).toBeDisabled();
  });

  it("filters by category chip and changes sort", async () => {
    const urls: string[] = [];
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(operator));
      if (url.includes("/mods/registry/providers"))
        return Promise.resolve(jsonRes(providersOf("modrinth")));
      if (url.includes("/mods/registry/search")) {
        urls.push(url);
        return Promise.resolve(jsonRes([pack]));
      }
      return Promise.resolve(jsonRes({}));
    });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({ refEnv: "MODRINTH_MODPACK" })} gs={gs()} />);
    await screen.findByText("Cobblemon");

    fireEvent.click(screen.getByRole("button", { name: "Tech" }));
    await waitFor(() => expect(urls.some((u) => u.includes("category=technology"))).toBe(true));

    fireEvent.change(screen.getByLabelText("Sort"), { target: { value: "updated" } });
    await waitFor(() => expect(urls.some((u) => u.includes("sort=updated"))).toBe(true));
    expect(urls.every((u) => u.includes("type=modpack"))).toBe(true);
  });

  it("switches providers", async () => {
    const urls: string[] = [];
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(operator));
      if (url.includes("/mods/registry/providers"))
        return Promise.resolve(
          jsonRes([
            { provider: "modrinth", available: true, modpacks: true },
            { provider: "thunderstore", available: true, modpacks: true },
          ]),
        );
      if (url.includes("/mods/registry/search")) {
        urls.push(url);
        return Promise.resolve(jsonRes([pack]));
      }
      return Promise.resolve(jsonRes({}));
    });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({ refEnv: "MODRINTH_MODPACK" })} gs={gs()} />);
    await screen.findByText("Cobblemon");
    await waitFor(() => expect(urls.some((u) => u.includes("provider=modrinth"))).toBe(true));

    fireEvent.click(screen.getByRole("button", { name: "Thunderstore" }));
    await waitFor(() => expect(urls.some((u) => u.includes("provider=thunderstore"))).toBe(true));
  });

  it("pages with load more", async () => {
    const page1 = Array.from({ length: 24 }, (_, i) => ({ ...pack, id: `p${i}`, title: `Pack ${i}` }));
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(operator));
      if (url.includes("/mods/registry/providers"))
        return Promise.resolve(jsonRes(providersOf("modrinth")));
      if (url.includes("/mods/registry/search")) {
        const off = new URL("http://x" + url.slice(url.indexOf("/"))).searchParams.get("offset");
        return Promise.resolve(jsonRes(off ? [{ ...pack, id: "last", title: "Last Pack" }] : page1));
      }
      return Promise.resolve(jsonRes({}));
    });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({ refEnv: "MODRINTH_MODPACK" })} gs={gs()} />);
    await screen.findByText("Pack 0");
    fireEvent.click(await screen.findByRole("button", { name: "Load more" }));
    expect(await screen.findByText("Last Pack")).toBeInTheDocument();
  });

  it("shows the unavailable state when no provider is usable", async () => {
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(operator));
      // No usable provider → the browser shows the unavailable state.
      if (url.includes("/mods/registry/providers")) return Promise.resolve(jsonRes([]));
      return Promise.resolve(jsonRes({}));
    });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({})} gs={gs()} />);
    expect(await screen.findByText(/isn’t available/)).toBeInTheDocument();
  });

  it("surfaces a search error", async () => {
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(operator));
      if (url.includes("/mods/registry/providers"))
        return Promise.resolve(jsonRes(providersOf("modrinth")));
      if (url.includes("/mods/registry/search")) return Promise.resolve(jsonRes({ error: "boom" }, 502));
      return Promise.resolve(jsonRes({}));
    });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({})} gs={gs()} />);
    expect(await screen.findByText("boom")).toBeInTheDocument();
  });

  it("env-mode: surfaces install error", async () => {
    fetchMock.mockImplementation((url: string, opts?: { method?: string }) => {
      if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(operator));
      if (url.includes("/mods/registry/providers"))
        return Promise.resolve(jsonRes(providersOf("modrinth")));
      if (url.includes("/mods/registry/search")) return Promise.resolve(jsonRes([pack]));
      if (url.includes("/modpack") && opts?.method === "POST")
        return Promise.resolve(jsonRes({ error: "failed to restart server" }, 500));
      return Promise.resolve(jsonRes({}));
    });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({ refEnv: "MODRINTH_MODPACK" })} gs={gs()} />);

    await screen.findByText("Cobblemon");
    fireEvent.click(await screen.findByRole("button", { name: /install/i }));
    expect(await screen.findByText("failed to restart server")).toBeInTheDocument();
  });

  it("deps-mode: surfaces install error for first mod", async () => {
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(operator));
      if (url.includes("/mods/registry/providers"))
        return Promise.resolve(jsonRes(providersOf("thunderstore")));
      if (url.includes("/mods/registry/search"))
        return Promise.resolve(jsonRes([{ ...pack, title: "MegaPack", id: "packer-MegaPack" }]));
      if (url.includes("/mods/registry/projects"))
        return Promise.resolve(
          jsonRes([
            { filename: "a.zip", downloadUrl: "https://cdn/a.zip" },
            { filename: "b.zip", downloadUrl: "https://cdn/b.zip" },
          ]),
        );
      if (url.includes("/mods/install"))
        return Promise.resolve(jsonRes({ error: "network timeout" }, 500));
      return Promise.resolve(jsonRes({}));
    });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({}, "thunderstore")} gs={gs()} />);

    fireEvent.click(await screen.findByRole("button", { name: /install/i }));
    expect(await screen.findByText("network timeout")).toBeInTheDocument();
  });

  it("dismisses the error banner when dismiss is clicked", async () => {
    fetchMock.mockImplementation((url: string, opts?: { method?: string }) => {
      if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(operator));
      if (url.includes("/mods/registry/providers"))
        return Promise.resolve(jsonRes(providersOf("modrinth")));
      if (url.includes("/mods/registry/search")) return Promise.resolve(jsonRes([pack]));
      if (url.includes("/modpack") && opts?.method === "POST")
        return Promise.resolve(jsonRes({ error: "install failed" }, 500));
      return Promise.resolve(jsonRes({}));
    });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({ refEnv: "MODRINTH_MODPACK" })} gs={gs()} />);

    await screen.findByText("Cobblemon");
    fireEvent.click(await screen.findByRole("button", { name: /install/i }));
    expect(await screen.findByText("install failed")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "dismiss" }));
    await waitFor(() => expect(screen.queryByText("install failed")).not.toBeInTheDocument());
  });

  it("install button stays disabled while one is being installed", async () => {
    let delayResolve: () => void;
    const delayedPromise = new Promise<void>((resolve) => {
      delayResolve = resolve;
    });

    fetchMock.mockImplementation((url: string, opts?: { method?: string }) => {
      if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(operator));
      if (url.includes("/mods/registry/providers"))
        return Promise.resolve(jsonRes(providersOf("modrinth")));
      if (url.includes("/mods/registry/search"))
        return Promise.resolve(jsonRes([pack, { ...pack, id: "pack2", title: "Pack 2" }]));
      if (url.includes("/modpack") && opts?.method === "POST")
        return delayedPromise.then(() => jsonRes({ ok: true }));
      return Promise.resolve(jsonRes({}));
    });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({ refEnv: "MODRINTH_MODPACK" })} gs={gs()} />);

    await screen.findByText("Cobblemon");
    const buttons = screen.getAllByRole("button", { name: /install/i });
    expect(buttons).toHaveLength(2);

    // Click first install
    fireEvent.click(buttons[0]);
    await waitFor(() => expect(buttons[0]).toHaveTextContent("Installing…"));

    // Second button should also be disabled
    expect(buttons[1]).toBeDisabled();

    delayResolve!();
  });

  it("shows active modpack when none is set", async () => {
    route({ packs: [] });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({ refEnv: "MODRINTH_MODPACK" })} gs={gs()} />);

    // The active modpack banner should not appear
    expect(screen.queryByText("Active modpack:")).not.toBeInTheDocument();
  });

  it("handles APIError with JSON error message", async () => {
    fetchMock.mockImplementation((url: string, opts?: { method?: string }) => {
      if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(operator));
      if (url.includes("/mods/registry/providers"))
        return Promise.resolve(jsonRes(providersOf("modrinth")));
      if (url.includes("/mods/registry/search")) return Promise.resolve(jsonRes([pack]));
      if (url.includes("/modpack") && opts?.method === "POST")
        return Promise.resolve(jsonRes({ error: "specific error from API" }, 500));
      return Promise.resolve(jsonRes({}));
    });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({ refEnv: "MODRINTH_MODPACK" })} gs={gs()} />);

    await screen.findByText("Cobblemon");
    fireEvent.click(await screen.findByRole("button", { name: /install/i }));
    expect(await screen.findByText("specific error from API")).toBeInTheDocument();
  });

  it("handles 403 error with role message", async () => {
    fetchMock.mockImplementation((url: string, opts?: { method?: string }) => {
      if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(operator));
      if (url.includes("/mods/registry/providers"))
        return Promise.resolve(jsonRes(providersOf("modrinth")));
      if (url.includes("/mods/registry/search")) return Promise.resolve(jsonRes([pack]));
      if (url.includes("/modpack") && opts?.method === "POST")
        return Promise.resolve(new Response("forbidden", { status: 403 }));
      return Promise.resolve(jsonRes({}));
    });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({ refEnv: "MODRINTH_MODPACK" })} gs={gs()} />);

    await screen.findByText("Cobblemon");
    fireEvent.click(await screen.findByRole("button", { name: /install/i }));
    expect(await screen.findByText("Your role does not allow managing this server.")).toBeInTheDocument();
  });

  it("shows success banner for env-mode install", async () => {
    route({ packs: [pack] });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({ refEnv: "MODRINTH_MODPACK" })} gs={gs()} />);

    await screen.findByText("Cobblemon");
    fireEvent.click(await screen.findByRole("button", { name: /install/i }));
    expect(await screen.findByText(/Set modpack Cobblemon/)).toBeInTheDocument();
  });

  it("shows success banner with mod count for deps-mode install", async () => {
    route({
      providers: providersOf("thunderstore"),
      packs: [{ ...pack, title: "Single Mod", id: "packer-Single" }],
      deps: [{ filename: "mod.zip", downloadUrl: "https://cdn/mod.zip" }],
    });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({}, "thunderstore")} gs={gs()} />);

    fireEvent.click(await screen.findByRole("button", { name: /install/i }));
    expect(await screen.findByText(/Installed Single Mod — 1 mod\./)).toBeInTheDocument();
  });

  it("pluralizes mod count in success message", async () => {
    route({
      providers: providersOf("thunderstore"),
      packs: [{ ...pack, title: "Multi", id: "packer-Multi" }],
      deps: [
        { filename: "a.zip", downloadUrl: "https://cdn/a.zip" },
        { filename: "b.zip", downloadUrl: "https://cdn/b.zip" },
        { filename: "c.zip", downloadUrl: "https://cdn/c.zip" },
      ],
    });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({}, "thunderstore")} gs={gs()} />);

    fireEvent.click(await screen.findByRole("button", { name: /install/i }));
    expect(await screen.findByText(/Installed Multi — 3 mods\./)).toBeInTheDocument();
  });

  it("shows installing state with project id match", async () => {
    let delayResolve: () => void;
    const delayedPromise = new Promise<void>((resolve) => {
      delayResolve = resolve;
    });

    fetchMock.mockImplementation((url: string, opts?: { method?: string }) => {
      if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(operator));
      if (url.includes("/mods/registry/providers"))
        return Promise.resolve(jsonRes(providersOf("modrinth")));
      if (url.includes("/mods/registry/search")) return Promise.resolve(jsonRes([pack]));
      if (url.includes("/modpack") && opts?.method === "POST")
        return delayedPromise.then(() => jsonRes({ ok: true }));
      return Promise.resolve(jsonRes({}));
    });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({ refEnv: "MODRINTH_MODPACK" })} gs={gs()} />);

    const button = await screen.findByRole("button", { name: /install/i });
    fireEvent.click(button);

    expect(await screen.findByRole("button", { name: "Installing…" })).toBeInTheDocument();
    delayResolve!();
  });
});
