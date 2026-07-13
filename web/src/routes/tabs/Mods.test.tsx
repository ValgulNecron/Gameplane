import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { renderWithQuery } from "@/test/render";
import { ModsTab } from "./Mods";
import type {
  GameServer,
  GameTemplate,
  InstalledMod,
  ModID,
  ModUpdatesResponse,
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
  updates?: ModUpdatesResponse;
  registry?: RegistryProject[];
  versions?: RegistryVersion[];
  providers?: { provider: string; available: boolean; modpacks: boolean }[];
  onInstall?: (body: { url: string; name?: string; replaces?: string; meta?: unknown }) => void;
  onUpload?: (body: unknown) => void;
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
    if (url.includes("/mods/registry/providers")) {
      return Promise.resolve(jsonRes(r.providers ?? [{ provider: "modrinth", available: true, modpacks: false }]));
    }
    if (url.includes("/mods/registry/search")) {
      return Promise.resolve(jsonRes(r.registry ?? []));
    }
    if (url.includes("/mods/registry/projects")) {
      return Promise.resolve(jsonRes(r.versions ?? []));
    }
    if (url.includes("/mods/updates")) {
      return Promise.resolve(jsonRes(r.updates ?? { checkedAt: new Date().toISOString(), updates: [] }));
    }
    if (url.includes("/mods/upload")) {
      r.onUpload?.(opts?.body);
      return Promise.resolve(jsonRes({ name: "uploaded.jar", size: 7, meta: { provider: "upload" } }));
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
  registry: { providers: [{ provider: "modrinth" }] },
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

  it("offers upload-only install when the module declares no install policy", async () => {
    route({ mods: [] });
    renderWithQuery(<ModsTab name="s1" tmpl={tmpl(listOnly)} />);
    await screen.findByText("No mods installed.");
    // Uploads carry no SSRF risk, so the install page stays reachable —
    // but only the Upload mode is offered (no URL/browse).
    const open = screen.getByRole("button", { name: /install mod/i });
    await waitFor(() => expect(open).not.toBeDisabled());
    fireEvent.click(open);
    expect(await screen.findByText(/Drop a mod file here/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "From URL" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Browse registry" })).not.toBeInTheDocument();
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

    // Registry installs record provenance in the agent's manifest.
    await waitFor(() =>
      expect(installs).toEqual([
        {
          url: "https://cdn.modrinth.com/sodium/0.6.0/sodium-fabric-0.6.0.jar",
          name: "sodium-fabric-0.6.0.jar",
          meta: {
            provider: "modrinth",
            projectId: "sodium",
            projectName: "Sodium",
            versionId: "v1",
            versionNumber: "0.6.0",
          },
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

  const managedSodium: InstalledMod = {
    name: "sodium-0.6.9.jar",
    size: 1024,
    meta: { provider: "modrinth", projectId: "sodium", versionId: "v-old", versionNumber: "0.6.9" },
  };
  const sodiumUpdate = {
    name: "sodium-0.6.9.jar",
    provider: "modrinth",
    projectId: "sodium",
    projectName: "Sodium",
    installedVersionId: "v-old",
    installedVersionNumber: "0.6.9",
    latestVersionId: "v-new",
    latestVersionNumber: "0.6.13",
    file: {
      filename: "sodium-0.6.13.jar",
      downloadUrl: "https://cdn.modrinth.com/sodium/0.6.13/sodium-0.6.13.jar",
      primary: true,
    },
  };

  it("badges managed mods with their provider and marks unmanaged files", async () => {
    route({ mods: [managedSodium, { name: "handmade.jar", size: 5, meta: null }] });
    renderWithQuery(<ModsTab name="s1" tmpl={tmpl(withInstall)} />);
    expect(await screen.findByText("modrinth · 0.6.9")).toBeInTheDocument();
    expect(await screen.findByText("unmanaged")).toBeInTheDocument();
  });

  it("checks for updates and applies one in place", async () => {
    const installs: unknown[] = [];
    route({
      mods: [managedSodium],
      updates: { checkedAt: new Date().toISOString(), updates: [sodiumUpdate] },
      onInstall: (b) => installs.push(b),
    });
    renderWithQuery(<ModsTab name="s1" tmpl={tmpl(withInstall)} />);

    const check = await screen.findByRole("button", { name: /check updates/i });
    await waitFor(() => expect(check).not.toBeDisabled());
    fireEvent.click(check);

    // The row gains an update pill + Update button.
    expect(await screen.findByText("0.6.13 available")).toBeInTheDocument();
    expect(await screen.findByText(/1 update available/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Update" }));
    await waitFor(() =>
      expect(installs).toEqual([
        {
          url: "https://cdn.modrinth.com/sodium/0.6.13/sodium-0.6.13.jar",
          name: "sodium-0.6.13.jar",
          replaces: "sodium-0.6.9.jar",
          meta: {
            provider: "modrinth",
            projectId: "sodium",
            projectName: "Sodium",
            versionId: "v-new",
            versionNumber: "0.6.13",
          },
        },
      ]),
    );
  });

  it("updates every outdated mod via Update all", async () => {
    const installs: Array<{ replaces?: string }> = [];
    const secondUpdate = {
      ...sodiumUpdate,
      name: "lithium-0.12.jar",
      projectId: "lithium",
      projectName: "Lithium",
      file: { filename: "lithium-0.13.jar", downloadUrl: "https://cdn.modrinth.com/lithium.jar" },
    };
    route({
      mods: [managedSodium, { ...managedSodium, name: "lithium-0.12.jar" }],
      updates: { checkedAt: new Date().toISOString(), updates: [sodiumUpdate, secondUpdate] },
      onInstall: (b) => installs.push(b as { replaces?: string }),
    });
    renderWithQuery(<ModsTab name="s1" tmpl={tmpl(withInstall)} />);

    // The check button stays disabled until the mod list has loaded —
    // clicking early is a silent no-op and the test would hang.
    const check = await screen.findByRole("button", { name: /check updates/i });
    await waitFor(() => expect(check).not.toBeDisabled());
    fireEvent.click(check);
    const all = await screen.findByRole("button", { name: /update all \(2\)/i });
    fireEvent.click(all);

    await waitFor(() => expect(installs).toHaveLength(2));
    expect(installs.map((i) => i.replaces)).toEqual(["sodium-0.6.9.jar", "lithium-0.12.jar"]);
  });

  it("uploads a local file from the Upload mode", async () => {
    const uploads: unknown[] = [];
    route({ mods: [], onUpload: (b) => uploads.push(b) });
    renderWithQuery(<ModsTab name="s1" tmpl={tmpl(withInstall)} />);

    const open = await screen.findByRole("button", { name: /install mod/i });
    await waitFor(() => expect(open).not.toBeDisabled());
    fireEvent.click(open);
    fireEvent.click(await screen.findByRole("button", { name: "Upload file" }));

    const file = new File(["JAR"], "custom.jar", { type: "application/java-archive" });
    const input = document.getElementById("mod-file") as HTMLInputElement;
    fireEvent.change(input, { target: { files: [file] } });

    const submit = screen.getByRole("button", { name: "Upload" });
    await waitFor(() => expect(submit).not.toBeDisabled());
    fireEvent.click(submit);

    await waitFor(() => expect(uploads).toHaveLength(1));
    expect(uploads[0]).toBeInstanceOf(FormData);
    expect((uploads[0] as FormData).get("file")).toBeInstanceOf(File);
    expect(await screen.findByText(/Uploaded uploaded.jar/)).toBeInTheDocument();
  });

  it("hands requiresAuth registry files off to the URL form", async () => {
    route({
      mods: [],
      registry: [
        { id: "flib", title: "Factorio Library", provider: "factorio" },
      ],
      versions: [
        {
          id: "0.16.2",
          versionNumber: "0.16.2",
          files: [
            {
              filename: "flib_0.16.2.zip",
              downloadUrl: "https://mods.factorio.com/download/flib/xyz",
              primary: true,
              requiresAuth: true,
            },
          ],
        },
      ],
    });
    renderWithQuery(<ModsTab name="s1" tmpl={tmpl(withBrowse)} />);

    const open = await screen.findByRole("button", { name: /install mod/i });
    await waitFor(() => expect(open).not.toBeDisabled());
    fireEvent.click(open);
    fireEvent.change(await screen.findByPlaceholderText("Search mods…"), {
      target: { value: "flib" },
    });
    fireEvent.click(await screen.findByText("Factorio Library"));

    // No one-click Install for credential-gated downloads — a hand-off
    // button switches to the URL form with the portal URL prefilled.
    const handoff = await screen.findByRole("button", { name: "Use URL form" });
    expect(screen.queryByRole("button", { name: "Install" })).not.toBeInTheDocument();
    fireEvent.click(handoff);

    const urlInput = await screen.findByLabelText<HTMLInputElement>(/download url/i);
    expect(urlInput.value).toBe("https://mods.factorio.com/download/flib/xyz");
    expect(screen.getByRole("button", { name: "From URL" })).toHaveAttribute("aria-pressed", "true");
  });
});

// ---------------------------------------------------------------------
// Id-managed mods (spec.capabilities.mods.idList): games whose server
// downloads its own mods given a list of ids (ARK's CurseForge ids,
// Project Zomboid's MOD_IDS, Steam Workshop lists). The template's idList
// declaration is the signal to render this editor instead of the
// file-based list/install/upload flow exercised above.
describe("ModsTab — id-managed mods (capabilities.mods.idList)", () => {
  function idTmpl(opts: { registry?: boolean } = {}): GameTemplate {
    return {
      metadata: { name: "ark-survival-ascended" },
      spec: {
        displayName: "ARK: Survival Ascended",
        game: "ark-survival-ascended",
        version: "1",
        image: "img",
        capabilities: {
          mods: {
            idList: { env: "ASA_START_PARAMS", separator: ",", format: " -mods={{ids}}", mode: "append" },
            ...(opts.registry ? { registry: { providers: [{ provider: "curseforge" as const }] } } : {}),
          },
        },
      },
    };
  }

  function routeIds(opts: {
    ids?: ModID[];
    onPut?: (body: ModID[]) => void;
    providers?: { provider: string; available: boolean; modpacks: boolean }[];
    search?: RegistryProject[];
  } = {}) {
    const ids = opts.ids ?? [];
    fetchMock.mockImplementation((url: string, o?: { method?: string; body?: string }) => {
      const method = o?.method ?? "GET";
      if (url.endsWith("/users/me")) {
        return Promise.resolve(
          jsonRes({
            id: 1,
            username: "u",
            displayName: "U",
            email: "",
            role: "operator",
            permissions: { "*": ["servers:read", "servers:write"] },
          }),
        );
      }
      if (url.includes("/mods/registry/providers")) {
        return Promise.resolve(
          jsonRes(opts.providers ?? [{ provider: "curseforge", available: true, modpacks: false }]),
        );
      }
      if (url.includes("/mods/registry/search")) {
        return Promise.resolve(jsonRes(opts.search ?? []));
      }
      if (url.includes("/mods/ids") && method === "PUT") {
        const body = JSON.parse(o?.body ?? "[]") as ModID[];
        opts.onPut?.(body);
        return Promise.resolve(jsonRes(body));
      }
      if (url.includes("/mods/ids")) {
        return Promise.resolve(jsonRes(ids));
      }
      return Promise.resolve(jsonRes({}));
    });
  }

  it("renders the id editor, not the file list, for an id-managed template", async () => {
    routeIds({ ids: [{ id: "889745", name: "Structures Plus (S+)" }] });
    renderWithQuery(<ModsTab name="s1" tmpl={idTmpl()} />);

    expect(await screen.findByText("Structures Plus (S+)")).toBeInTheDocument();
    // The file-based UI never mounts: no "Install mod" button, no
    // installed-file empty state, no per-mod file size/mtime line.
    expect(screen.queryByRole("button", { name: /install mod/i })).not.toBeInTheDocument();
    expect(screen.queryByText("No mods installed.")).not.toBeInTheDocument();
  });

  it("adds a mod by id and saves exactly one PUT with the full list", async () => {
    const puts: ModID[][] = [];
    routeIds({ ids: [{ id: "889745", name: "Structures Plus (S+)" }], onPut: (b) => puts.push(b) });
    renderWithQuery(<ModsTab name="s1" tmpl={idTmpl()} />);

    await screen.findByText("Structures Plus (S+)");
    fireEvent.change(screen.getByPlaceholderText(/paste a.*mod id/i), { target: { value: "895711" } });
    const addBtn = screen.getByRole("button", { name: "Add" });
    expect(addBtn).not.toBeDisabled();
    fireEvent.click(addBtn);

    // The new row shows the "Added" pending chip immediately, before Save.
    expect(await screen.findByText("895711")).toBeInTheDocument();
    expect(screen.getByText("Added")).toBeInTheDocument();

    const saveBtn = screen.getByRole("button", { name: "Save changes" });
    expect(saveBtn).not.toBeDisabled();
    fireEvent.click(saveBtn);

    await waitFor(() => expect(puts).toHaveLength(1));
    expect(puts[0]).toEqual([{ id: "889745", name: "Structures Plus (S+)" }, { id: "895711" }]);
  });

  it("marks a removal as pending and only applies it on save", async () => {
    const puts: ModID[][] = [];
    routeIds({ ids: [{ id: "889745", name: "Structures Plus (S+)" }], onPut: (b) => puts.push(b) });
    renderWithQuery(<ModsTab name="s1" tmpl={idTmpl()} />);

    await screen.findByText("Structures Plus (S+)");
    fireEvent.click(screen.getByRole("button", { name: "Remove" }));

    // Still listed (dimmed, with Undo) — not actually dropped until Save.
    expect(await screen.findByText("Marked for removal")).toBeInTheDocument();
    expect(screen.getByText("Structures Plus (S+)")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Undo" })).toBeInTheDocument();
    expect(puts).toHaveLength(0);

    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(puts).toEqual([[]]));
  });

  it("discard reverts pending edits without saving", async () => {
    const puts: ModID[][] = [];
    routeIds({ ids: [{ id: "889745", name: "Structures Plus (S+)" }], onPut: (b) => puts.push(b) });
    renderWithQuery(<ModsTab name="s1" tmpl={idTmpl()} />);

    await screen.findByText("Structures Plus (S+)");
    fireEvent.click(screen.getByRole("button", { name: "Remove" }));
    expect(await screen.findByText("Marked for removal")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Discard" }));
    await waitFor(() => expect(screen.queryByText("Marked for removal")).not.toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    expect(puts).toHaveLength(0);
  });

  it("disables Save changes and Discard when there are no pending edits", async () => {
    routeIds({ ids: [{ id: "889745", name: "Structures Plus (S+)" }] });
    renderWithQuery(<ModsTab name="s1" tmpl={idTmpl()} />);

    await screen.findByText("Structures Plus (S+)");
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Discard" })).toBeDisabled();
  });

  it("adds a mod from registry browse instead of installing a file", async () => {
    routeIds({
      ids: [],
      search: [{ id: "889745", title: "Structures Plus (S+)", provider: "curseforge", author: "orionsun" }],
    });
    renderWithQuery(<ModsTab name="s1" tmpl={idTmpl({ registry: true })} />);

    await screen.findByText("0 selected");
    fireEvent.click(await screen.findByRole("button", { name: /browse curseforge/i }));

    // The card's Add button adds to the pending list; it never calls an
    // install endpoint (there is none to call in id-managed mode).
    const cardAdd = await screen.findByRole("button", { name: "Add" });
    fireEvent.click(cardAdd);
    expect(await screen.findByText("Added")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /selected mods/i }));
    expect(await screen.findByText("Structures Plus (S+)")).toBeInTheDocument();
    expect(screen.getByText("Added")).toBeInTheDocument();
  });

  it("still offers Add-by-ID when no registry provider is declared", async () => {
    routeIds({ ids: [] });
    renderWithQuery(<ModsTab name="s1" tmpl={idTmpl()} />);

    await screen.findByText("0 selected");
    expect(screen.queryByRole("button", { name: /browse/i })).not.toBeInTheDocument();
    expect(screen.getByPlaceholderText(/paste a mod id/i)).toBeInTheDocument();
  });

  it("still renders the file-based list for a template with no idList (no regression)", async () => {
    route({ mods: [{ name: "sodium.jar", size: 1024 }] });
    renderWithQuery(<ModsTab name="s1" tmpl={tmpl(withInstall)} />);
    expect(await screen.findByText("sodium.jar")).toBeInTheDocument();
    expect(screen.queryByText(/selected$/)).not.toBeInTheDocument();
  });
});
