import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { renderWithQuery } from "@/test/render";
import { ModsTab } from "./Mods";
import type {
  GameServer,
  GameTemplate,
  InstalledMod,
  ModsCapability,
  RegistryProject,
  RegistryVersion,
} from "@/types";

const fetchMock = vi.fn();

beforeEach(() => vi.stubGlobal("fetch", fetchMock));
afterEach(() => {
  fetchMock.mockReset();
  vi.unstubAllGlobals();
});

function jsonRes(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

interface Routes {
  role?: "operator" | "viewer";
  mods?: InstalledMod[];
  registry?: RegistryProject[];
  versions?: RegistryVersion[];
  onInstall?: (body: { url: string; name?: string }) => void;
  onRemove?: (url: string) => void;
}

function route(r: Routes) {
  const role = r.role ?? "operator";
  // Mods management is gated on servers:write; mirror that in the mocked
  // permission set so the role still drives what the UI offers.
  const permissions =
    role === "operator"
      ? { "*": ["servers:read", "servers:write"] }
      : { "*": ["servers:read"] };
  const mods = r.mods ?? [];
  fetchMock.mockImplementation((url: string, opts?: { method?: string; body?: string }) => {
    const method = opts?.method ?? "GET";
    if (url.endsWith("/users/me")) {
      return Promise.resolve(jsonRes({ id: 1, username: "u", displayName: "U", email: "", role, permissions }));
    }
    // Registry browse routes contain "/mods" — match them before the
    // generic /mods list handler below.
    if (url.includes("/mods/registry/search")) {
      return Promise.resolve(jsonRes(r.registry ?? []));
    }
    if (url.includes("/mods/registry/projects")) {
      return Promise.resolve(jsonRes(r.versions ?? []));
    }
    if (url.includes("/mods/install")) {
      const body = JSON.parse(opts?.body ?? "{}") as { url: string; name?: string };
      r.onInstall?.(body);
      return Promise.resolve(jsonRes({ name: body.name ?? "fetched.jar", size: 10 }));
    }
    if (url.includes("/mods") && method === "DELETE") {
      r.onRemove?.(url);
      return Promise.resolve(new Response(null, { status: 204 }));
    }
    if (url.includes("/mods")) {
      return Promise.resolve(jsonRes(mods));
    }
    return Promise.resolve(jsonRes({}));
  });
}

function tmpl(mods: ModsCapability): GameTemplate {
  return {
    metadata: { name: "minecraft-java" },
    spec: {
      displayName: "Minecraft",
      game: "minecraft-java",
      version: "1",
      image: "img",
      capabilities: { mods },
    },
  };
}

const withInstall: ModsCapability = {
  path: "mods",
  extensions: [".jar"],
  install: { allowedHosts: ["cdn.modrinth.com"] },
};
const listOnly: ModsCapability = { path: "mods" };
const withBrowse: ModsCapability = {
  path: "mods",
  extensions: [".jar"],
  install: { allowedHosts: ["cdn.modrinth.com"] },
  registry: { provider: "modrinth" },
};

function versionedTmpl(): GameTemplate {
  return {
    metadata: { name: "minecraft-java" },
    spec: {
      displayName: "Minecraft",
      game: "minecraft-java",
      version: "2",
      image: "img",
      versions: [
        { id: "1.21.4-paper", displayName: "1.21.4 · Paper", loader: "paper", default: true },
        { id: "1.21.4-forge", displayName: "1.21.4 · Forge", loader: "forge" },
      ],
      capabilities: {
        mods: {
          loaders: { paper: { path: "plugins" }, forge: { path: "mods" } },
          install: { allowedHosts: ["cdn.modrinth.com"] },
        },
      },
    },
  };
}

function gsVer(version: string): GameServer {
  return { metadata: { name: "s1" }, spec: { templateRef: { name: "minecraft-java" }, version } };
}

describe("ModsTab", () => {
  it("lists installed mods", async () => {
    route({ mods: [{ name: "sodium.jar", size: 1024 }, { name: "lithium.jar", size: 2048 }] });
    renderWithQuery(<ModsTab name="s1" tmpl={tmpl(withInstall)} />);
    expect(await screen.findByText("sodium.jar")).toBeInTheDocument();
    expect(await screen.findByText("lithium.jar")).toBeInTheDocument();
    expect(await screen.findByText("2 installed")).toBeInTheDocument();
  });

  it("shows an empty state with no mods", async () => {
    route({ mods: [] });
    renderWithQuery(<ModsTab name="s1" tmpl={tmpl(withInstall)} />);
    expect(await screen.findByText("No mods installed.")).toBeInTheDocument();
  });

  it("shows the active version+loader+path header for the per-loader model", async () => {
    route({ mods: [] });
    renderWithQuery(<ModsTab name="s1" tmpl={versionedTmpl()} gs={gsVer("1.21.4-forge")} />);
    expect(await screen.findByTestId("mods-active")).toHaveTextContent("1.21.4 · Forge · mods");
  });

  it("hides Install when the module declares no install policy", async () => {
    route({ mods: [] });
    renderWithQuery(<ModsTab name="s1" tmpl={tmpl(listOnly)} />);
    await screen.findByText("No mods installed.");
    expect(screen.queryByRole("button", { name: /install mod/i })).not.toBeInTheDocument();
  });

  it("installs a mod by URL", async () => {
    const installs: Array<{ url: string; name?: string }> = [];
    route({ mods: [], onInstall: (b) => installs.push(b) });
    renderWithQuery(<ModsTab name="s1" tmpl={tmpl(withInstall)} />);

    const openBtn = await screen.findByRole("button", { name: /install mod/i });
    await waitFor(() => expect(openBtn).not.toBeDisabled());
    fireEvent.click(openBtn);

    const urlInput = await screen.findByLabelText(/download url/i);
    // Install stays disabled until the URL looks valid.
    const submit = screen.getByRole("button", { name: "Install" });
    expect(submit).toBeDisabled();
    fireEvent.change(urlInput, { target: { value: "https://cdn.modrinth.com/x/sodium.jar" } });
    expect(submit).not.toBeDisabled();
    fireEvent.click(submit);

    await waitFor(() =>
      expect(installs).toEqual([{ url: "https://cdn.modrinth.com/x/sodium.jar" }]),
    );
  });

  it("searches a registry and installs the selected file", async () => {
    const installs: Array<{ url: string; name?: string }> = [];
    route({
      mods: [],
      onInstall: (b) => installs.push(b),
      registry: [
        {
          id: "sodium",
          title: "Sodium",
          author: "jellysquid",
          downloads: 30_800_000,
          iconUrl: "https://cdn.modrinth.com/sodium/icon.png",
          provider: "modrinth",
        },
      ],
      versions: [
        {
          id: "v1",
          versionNumber: "0.6.0",
          files: [
            {
              filename: "sodium-fabric-0.6.0.jar",
              downloadUrl: "https://cdn.modrinth.com/sodium/0.6.0/sodium-fabric-0.6.0.jar",
              primary: true,
            },
          ],
        },
      ],
    });
    renderWithQuery(<ModsTab name="s1" tmpl={tmpl(withBrowse)} />);

    const open = await screen.findByRole("button", { name: /install mod/i });
    await waitFor(() => expect(open).not.toBeDisabled());
    fireEvent.click(open);

    // A browse-capable template defaults to Search mode.
    expect(screen.getByRole("button", { name: "Browse registry" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    fireEvent.change(await screen.findByPlaceholderText("Search mods…"), {
      target: { value: "sodium" },
    });

    // Result appears after the debounce + fetch; expand it to load versions.
    fireEvent.click(await screen.findByText("Sodium"));
    const cardInstall = await screen.findByRole("button", { name: "Install" });
    fireEvent.click(cardInstall);

    await waitFor(() =>
      expect(installs).toEqual([
        {
          url: "https://cdn.modrinth.com/sodium/0.6.0/sodium-fabric-0.6.0.jar",
          name: "sodium-fabric-0.6.0.jar",
        },
      ]),
    );
  });

  it("can switch from Search to From URL in a browse-capable template", async () => {
    const installs: Array<{ url: string; name?: string }> = [];
    route({ mods: [], onInstall: (b) => installs.push(b), registry: [] });
    renderWithQuery(<ModsTab name="s1" tmpl={tmpl(withBrowse)} />);

    const open = await screen.findByRole("button", { name: /install mod/i });
    await waitFor(() => expect(open).not.toBeDisabled());
    fireEvent.click(open);
    fireEvent.click(await screen.findByRole("button", { name: "From URL" }));

    fireEvent.change(await screen.findByLabelText(/download url/i), {
      target: { value: "https://cdn.modrinth.com/x.jar" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Install" }));
    await waitFor(() => expect(installs).toEqual([{ url: "https://cdn.modrinth.com/x.jar" }]));
  });

  it("removes a mod after confirmation", async () => {
    const removed: string[] = [];
    route({ mods: [{ name: "old.jar", size: 1 }], onRemove: (u) => removed.push(u) });
    renderWithQuery(<ModsTab name="s1" tmpl={tmpl(withInstall)} />);

    const rowRemove = await screen.findByRole("button", { name: "Remove old.jar" });
    fireEvent.click(rowRemove);
    // Confirm dialog's own Remove button (exact name) commits the delete.
    const confirm = await screen.findByRole("button", { name: "Remove" });
    fireEvent.click(confirm);

    await waitFor(() => expect(removed).toHaveLength(1));
    expect(removed[0]).toContain("name=old.jar");
  });

  const operator = {
    id: 1, username: "u", displayName: "U", email: "", role: "operator",
    permissions: { "*": ["servers:read", "servers:write"] },
  };

  async function openInstall() {
    const open = await screen.findByRole("button", { name: /install mod/i });
    await waitFor(() => expect(open).not.toBeDisabled());
    fireEvent.click(open);
    fireEvent.change(await screen.findByLabelText(/download url/i), {
      target: { value: "https://cdn.modrinth.com/x.jar" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Install" }));
  }

  it("surfaces the agent's install error message", async () => {
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(operator));
      if (url.includes("/mods/install"))
        return Promise.resolve(jsonRes({ error: "download host is not allowed by this module" }, 403));
      if (url.includes("/mods")) return Promise.resolve(jsonRes([]));
      return Promise.resolve(jsonRes({}));
    });
    renderWithQuery(<ModsTab name="s1" tmpl={tmpl(withInstall)} />);
    await openInstall();
    expect(
      await screen.findByText("download host is not allowed by this module"),
    ).toBeInTheDocument();
  });

  it("falls back to a role message on a bare 403", async () => {
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(operator));
      if (url.includes("/mods/install")) return Promise.resolve(new Response("forbidden", { status: 403 }));
      if (url.includes("/mods")) return Promise.resolve(jsonRes([]));
      return Promise.resolve(jsonRes({}));
    });
    renderWithQuery(<ModsTab name="s1" tmpl={tmpl(withInstall)} />);
    await openInstall();
    expect(await screen.findByText("Your role does not allow managing mods.")).toBeInTheDocument();
  });

  it("shows an error when the mod list fails to load", async () => {
    fetchMock.mockImplementation((url: string) => {
      if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(operator));
      if (url.includes("/mods")) return Promise.resolve(new Response("boom", { status: 502 }));
      return Promise.resolve(jsonRes({}));
    });
    renderWithQuery(<ModsTab name="s1" tmpl={tmpl(withInstall)} />);
    expect(await screen.findByText("Couldn’t load mods")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "retry" })).toBeInTheDocument();
  });

  it("surfaces a remove failure and closes the confirm dialog", async () => {
    fetchMock.mockImplementation((url: string, opts?: { method?: string }) => {
      if (url.endsWith("/users/me")) return Promise.resolve(jsonRes(operator));
      if (url.includes("/mods") && (opts?.method ?? "GET") === "DELETE")
        return Promise.resolve(jsonRes({ error: "could not remove mod" }, 500));
      if (url.includes("/mods")) return Promise.resolve(jsonRes([{ name: "old.jar", size: 1 }]));
      return Promise.resolve(jsonRes({}));
    });
    renderWithQuery(<ModsTab name="s1" tmpl={tmpl(withInstall)} />);
    fireEvent.click(await screen.findByRole("button", { name: "Remove old.jar" }));
    fireEvent.click(await screen.findByRole("button", { name: "Remove" }));
    expect(await screen.findByText("could not remove mod")).toBeInTheDocument();
    // The confirm dialog closes on failure.
    await waitFor(() => expect(screen.queryByText("Remove mod")).not.toBeInTheDocument());
  });

  it("disables management for viewers", async () => {
    route({ role: "viewer", mods: [{ name: "x.jar", size: 1 }] });
    renderWithQuery(<ModsTab name="s1" tmpl={tmpl(withInstall)} />);
    const install = await screen.findByRole("button", { name: /install mod/i });
    await waitFor(() => expect(install).toBeDisabled());
    // No per-row remove button is rendered for a viewer.
    expect(screen.queryByRole("button", { name: /^Remove / })).not.toBeInTheDocument();
  });
});
