import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { renderWithQuery } from "@/test/render";
import { ModpacksTab } from "./Modpacks";
import type { GameServer, GameTemplate, ModRegistryDecl, RegistryProject } from "@/types";

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

interface Routes {
  me?: typeof operator;
  packs?: RegistryProject[];
  deps?: { filename: string; downloadUrl: string }[];
  onModpack?: (body: { ref: string }) => void;
  onInstallMod?: (body: { url: string; name?: string }) => void;
}

function route(r: Routes) {
  fetchMock.mockImplementation((url: string, opts?: { method?: string; body?: string }) => {
    const method = opts?.method ?? "GET";
    if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(r.me ?? operator));
    if (url.includes("/mods/registry/search")) return Promise.resolve(jsonRes(r.packs ?? []));
    if (url.includes("/mods/registry/projects/") && url.includes("/modpack"))
      return Promise.resolve(jsonRes(r.deps ?? []));
    if (url.endsWith("/modpack") && method === "POST") {
      r.onModpack?.(JSON.parse(opts?.body ?? "{}"));
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

function tmpl(modpacks: ModRegistryDecl["modpacks"], provider = "modrinth"): GameTemplate {
  return {
    metadata: { name: "t" },
    spec: {
      displayName: "T", game: "g", version: "1", image: "img",
      capabilities: { mods: { loaders: { l: { path: "mods" } }, registry: { provider: provider as "modrinth", modpacks } } },
    },
  };
}

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

    const card = await screen.findByText("Cobblemon");
    const install = within(card.closest("div")!.parentElement!).getByRole("button", { name: /install/i });
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

  it("pages with load more", async () => {
    const page1 = Array.from({ length: 24 }, (_, i) => ({ ...pack, id: `p${i}`, title: `Pack ${i}` }));
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(operator));
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

  it("shows the unavailable state on 501", async () => {
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(operator));
      if (url.includes("/mods/registry/search")) return Promise.resolve(jsonRes({ error: "no registry" }, 501));
      return Promise.resolve(jsonRes({}));
    });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({})} gs={gs()} />);
    expect(await screen.findByText(/isn’t available/)).toBeInTheDocument();
  });

  it("surfaces a search error", async () => {
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(operator));
      if (url.includes("/mods/registry/search")) return Promise.resolve(jsonRes({ error: "boom" }, 502));
      return Promise.resolve(jsonRes({}));
    });
    renderWithQuery(<ModpacksTab name="s1" tmpl={tmpl({})} gs={gs()} />);
    expect(await screen.findByText("boom")).toBeInTheDocument();
  });
});
