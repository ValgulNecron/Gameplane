import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { RegistryBrowser, providerLabel, compactNum } from "./registry-browser";
import type { RegistryProject, ModRegistryProvider } from "@/types";

const mockProject = (overrides: Partial<RegistryProject> = {}): RegistryProject => ({
  id: "proj-1",
  title: "Example Mod",
  slug: "example-mod",
  provider: "modrinth",
  downloads: 1500,
  iconUrl: "https://example.com/icon.png",
  ...overrides,
});

describe("RegistryBrowser", () => {
  it("renders no provider UI when only one provider is available", async () => {
    server.use(
      http.get("/servers/test/registry-providers", () =>
        HttpResponse.json([{ provider: "modrinth", available: true, mods: true, modpacks: false }]),
      ),
    );
    renderWithQuery(
      <RegistryBrowser
        name="test"
        type="mod"
        renderItem={(p) => <div>{p.title}</div>}
      />,
    );
    await waitFor(() => {
      expect(screen.queryByRole("button", { name: /modrinth/i })).not.toBeInTheDocument();
    });
  });

  it("renders provider switcher when multiple providers available", async () => {
    server.use(
      http.get("/servers/test/registry-providers", () =>
        HttpResponse.json([
          { provider: "modrinth", available: true, mods: true, modpacks: false },
          { provider: "curseforge", available: true, mods: true, modpacks: true },
        ]),
      ),
    );
    renderWithQuery(
      <RegistryBrowser
        name="test"
        renderItem={(p) => <div>{p.title}</div>}
      />,
    );
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Modrinth" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "CurseForge" })).toBeInTheDocument();
    });
  });

  it("filters providers by type (modpacks only)", async () => {
    server.use(
      http.get("/servers/test/registry-providers", () =>
        HttpResponse.json([
          { provider: "modrinth", available: true, mods: true, modpacks: true },
          { provider: "curseforge", available: true, mods: true, modpacks: false },
        ]),
      ),
    );
    renderWithQuery(
      <RegistryBrowser
        name="test"
        type="modpack"
        renderItem={(p) => <div>{p.title}</div>}
      />,
    );
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Modrinth" })).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "CurseForge" })).not.toBeInTheDocument();
    });
  });

  it("shows 'not available' message when no usable providers", async () => {
    server.use(
      http.get("/servers/test/registry-providers", () =>
        HttpResponse.json([]),
      ),
    );
    renderWithQuery(
      <RegistryBrowser
        name="test"
        renderItem={(p) => <div>{p.title}</div>}
      />,
    );
    await waitFor(() => {
      expect(
        screen.getByText(/In-app browse isn't available/),
      ).toBeInTheDocument();
    });
  });

  it("loads and displays results from default provider", async () => {
    server.use(
      http.get("/servers/test/registry-providers", () =>
        HttpResponse.json([
          { provider: "modrinth", available: true, mods: true, modpacks: false },
        ]),
      ),
      http.get("/servers/test/registry/search", () =>
        HttpResponse.json([mockProject({ title: "First Mod" })]),
      ),
    );
    renderWithQuery(
      <RegistryBrowser
        name="test"
        renderItem={(p) => <div>{p.title}</div>}
      />,
    );
    expect(await screen.findByText("First Mod")).toBeInTheDocument();
  });

  it("debounces search input", async () => {
    let searchCalls = 0;
    server.use(
      http.get("/servers/test/registry-providers", () =>
        HttpResponse.json([
          { provider: "modrinth", available: true, mods: true, modpacks: false },
        ]),
      ),
      http.get("/servers/test/registry/search", () => {
        searchCalls++;
        return HttpResponse.json([mockProject()]);
      }),
    );
    renderWithQuery(
      <RegistryBrowser
        name="test"
        renderItem={(p) => <div>{p.title}</div>}
      />,
    );

    // Initial load
    await screen.findByText("Example Mod");
    const initialCalls = searchCalls;

    // Type fast - should debounce
    const input = screen.getByRole("textbox", { name: /search/i });
    await userEvent.type(input, "abc", { delay: 50 });

    // Wait for debounce
    await waitFor(() => {
      expect(searchCalls).toBeGreaterThan(initialCalls);
    });
  });

  it("disables sort when searching", async () => {
    server.use(
      http.get("/servers/test/registry-providers", () =>
        HttpResponse.json([
          { provider: "modrinth", available: true, mods: true, modpacks: false },
        ]),
      ),
      http.get("/servers/test/registry/search", () =>
        HttpResponse.json([mockProject()]),
      ),
    );
    renderWithQuery(
      <RegistryBrowser
        name="test"
        renderItem={(p) => <div>{p.title}</div>}
      />,
    );

    const sortSelect = screen.getByRole("combobox", { name: /sort/i }) as HTMLSelectElement;
    expect(sortSelect).not.toBeDisabled();

    const input = screen.getByRole("textbox", { name: /search/i });
    await userEvent.type(input, "test");

    // Sort should become disabled while searching
    await waitFor(() => {
      expect(sortSelect).toBeDisabled();
    });
  });

  it("renders category chips only for modrinth", async () => {
    server.use(
      http.get("/servers/test/registry-providers", () =>
        HttpResponse.json([
          { provider: "modrinth", available: true, mods: true, modpacks: false },
        ]),
      ),
      http.get("/servers/test/registry/search", () =>
        HttpResponse.json([mockProject()]),
      ),
    );
    const categories = [
      { value: "magic", label: "Magic" },
      { value: "tech", label: "Tech" },
    ];
    renderWithQuery(
      <RegistryBrowser
        name="test"
        categories={categories}
        renderItem={(p) => <div>{p.title}</div>}
      />,
    );

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "All" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Magic" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Tech" })).toBeInTheDocument();
    });
  });

  it("filters by category", async () => {
    let lastRequest: any = null;
    server.use(
      http.get("/servers/test/registry-providers", () =>
        HttpResponse.json([
          { provider: "modrinth", available: true, mods: true, modpacks: false },
        ]),
      ),
      http.get("/servers/test/registry/search", ({ request }) => {
        lastRequest = Object.fromEntries(new URL(request.url).searchParams);
        return HttpResponse.json([mockProject()]);
      }),
    );
    const categories = [
      { value: "magic", label: "Magic" },
    ];
    renderWithQuery(
      <RegistryBrowser
        name="test"
        categories={categories}
        renderItem={(p) => <div>{p.title}</div>}
      />,
    );

    await screen.findByText("Example Mod");
    const magicBtn = screen.getByRole("button", { name: "Magic" });
    await userEvent.click(magicBtn);

    await waitFor(() => {
      expect(lastRequest?.category).toBe("magic");
    });
  });

  it("shows 'No results' when search finds nothing", async () => {
    server.use(
      http.get("/servers/test/registry-providers", () =>
        HttpResponse.json([
          { provider: "modrinth", available: true, mods: true, modpacks: false },
        ]),
      ),
      http.get("/servers/test/registry/search", () =>
        HttpResponse.json([]),
      ),
    );
    renderWithQuery(
      <RegistryBrowser
        name="test"
        renderItem={(p) => <div>{p.title}</div>}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText(/No results/)).toBeInTheDocument();
    });
  });

  it("shows error on search failure", async () => {
    server.use(
      http.get("/servers/test/registry-providers", () =>
        HttpResponse.json([
          { provider: "modrinth", available: true, mods: true, modpacks: false },
        ]),
      ),
      http.get("/servers/test/registry/search", () =>
        HttpResponse.json({ error: "Invalid search query" }, { status: 400 }),
      ),
    );
    renderWithQuery(
      <RegistryBrowser
        name="test"
        renderItem={(p) => <div>{p.title}</div>}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText(/Invalid search query/)).toBeInTheDocument();
    });
  });

  it("handles pagination with Load more button", async () => {
    const projects = Array.from({ length: 30 }, (_, i) =>
      mockProject({ title: `Mod ${i + 1}` }),
    );
    let callCount = 0;
    server.use(
      http.get("/servers/test/registry-providers", () =>
        HttpResponse.json([
          { provider: "modrinth", available: true, mods: true, modpacks: false },
        ]),
      ),
      http.get("/servers/test/registry/search", ({ request }) => {
        const url = new URL(request.url);
        const offset = parseInt(url.searchParams.get("offset") ?? "0");
        callCount++;
        const page = projects.slice(offset, offset + 24);
        return HttpResponse.json(page);
      }),
    );
    renderWithQuery(
      <RegistryBrowser
        name="test"
        renderItem={(p) => <div>{p.title}</div>}
      />,
    );

    // First page loads
    await screen.findByText("Mod 1");
    expect(screen.getByRole("button", { name: /Load more/i })).toBeInTheDocument();

    // Click load more
    await userEvent.click(screen.getByRole("button", { name: /Load more/i }));

    // Second page should load
    await waitFor(() => {
      expect(screen.getByText("Mod 25")).toBeInTheDocument();
    });
  });

  it("keeps previous results while fetching new search", async () => {
    let isSlowRequest = false;
    server.use(
      http.get("/servers/test/registry-providers", () =>
        HttpResponse.json([
          { provider: "modrinth", available: true, mods: true, modpacks: false },
        ]),
      ),
      http.get("/servers/test/registry/search", async () => {
        if (isSlowRequest) {
          await new Promise((r) => setTimeout(r, 100));
        }
        return HttpResponse.json([mockProject({ title: "Result" })]);
      }),
    );
    renderWithQuery(
      <RegistryBrowser
        name="test"
        renderItem={(p) => <div>{p.title}</div>}
      />,
    );

    await screen.findByText("Result");

    // Trigger a slow search
    isSlowRequest = true;
    const input = screen.getByRole("textbox", { name: /search/i });
    await userEvent.type(input, "new");

    // Old results should still be visible (keepPreviousData)
    expect(screen.getByText("Result")).toBeInTheDocument();
  });

  it("falls back to first available provider when pick is no longer available", async () => {
    const { rerender } = renderWithQuery(
      <RegistryBrowser
        name="test"
        renderItem={(p) => <div>{p.title}</div>}
      />,
    );
    server.use(
      http.get("/servers/test/registry-providers", () =>
        HttpResponse.json([
          { provider: "modrinth", available: true, mods: true, modpacks: false },
          { provider: "curseforge", available: true, mods: true, modpacks: false },
        ]),
      ),
      http.get("/servers/test/registry/search", () =>
        HttpResponse.json([mockProject()]),
      ),
    );

    // Initial render with both providers
    await screen.findByText("Example Mod");

    // Re-render with only one provider
    server.use(
      http.get("/servers/test/registry-providers", () =>
        HttpResponse.json([
          { provider: "modrinth", available: true, mods: true, modpacks: false },
        ]),
      ),
    );
    rerender(
      <RegistryBrowser
        name="test"
        renderItem={(p) => <div>{p.title}</div>}
      />,
    );

    // Should still render without crashing
    expect(screen.getByText("Example Mod")).toBeInTheDocument();
  });

  it("renders renderItem for each project with provider", async () => {
    const renderItem = vi.fn((p) => <div key={p.id}>{p.name}</div>);
    server.use(
      http.get("/servers/test/registry-providers", () =>
        HttpResponse.json([
          { provider: "modrinth", available: true, mods: true, modpacks: false },
        ]),
      ),
      http.get("/servers/test/registry/search", () =>
        HttpResponse.json([
          mockProject({ id: "1", title: "Mod 1" }),
          mockProject({ id: "2", title: "Mod 2" }),
        ]),
      ),
    );
    renderWithQuery(
      <RegistryBrowser
        name="test"
        renderItem={renderItem}
      />,
    );

    await waitFor(() => {
      expect(renderItem).toHaveBeenCalledWith(
        expect.objectContaining({ id: "1", name: "Mod 1" }),
        "modrinth",
      );
      expect(renderItem).toHaveBeenCalledWith(
        expect.objectContaining({ id: "2", name: "Mod 2" }),
        "modrinth",
      );
    });
  });

  it("switches providers and resets pagination", async () => {
    server.use(
      http.get("/servers/test/registry-providers", () =>
        HttpResponse.json([
          { provider: "modrinth", available: true, mods: true, modpacks: false },
          { provider: "curseforge", available: true, mods: true, modpacks: false },
        ]),
      ),
      http.get("/servers/test/registry/search", ({ request }) => {
        const url = new URL(request.url);
        const provider = url.searchParams.get("provider") as ModRegistryProvider;
        return HttpResponse.json([
          mockProject({ provider, title: `${provider} Result` }),
        ]);
      }),
    );
    renderWithQuery(
      <RegistryBrowser
        name="test"
        renderItem={(p) => <div>{p.title}</div>}
      />,
    );

    await screen.findByText("modrinth Result");

    // Switch provider
    await userEvent.click(screen.getByRole("button", { name: "CurseForge" }));

    await waitFor(() => {
      expect(screen.getByText("curseforge Result")).toBeInTheDocument();
    });
  });
});

describe("providerLabel", () => {
  it("returns display name for known providers", () => {
    expect(providerLabel("modrinth")).toBe("Modrinth");
    expect(providerLabel("curseforge")).toBe("CurseForge");
    expect(providerLabel("thunderstore")).toBe("Thunderstore");
    expect(providerLabel("factorio")).toBe("Factorio Mod Portal");
  });

  it("capitalizes unknown providers", () => {
    expect(providerLabel("unknown")).toBe("Unknown");
    expect(providerLabel("custom-provider")).toBe("Custom-provider");
  });
});

describe("compactNum", () => {
  it("formats small numbers", () => {
    expect(compactNum(0)).toBe("0");
    expect(compactNum(100)).toBe("100");
    expect(compactNum(999)).toBe("999");
  });

  it("formats thousands", () => {
    expect(compactNum(1000)).toBe("1K");
    expect(compactNum(1500)).toBe("1.5K");
    expect(compactNum(999999)).toBe("1M");
  });

  it("formats millions", () => {
    expect(compactNum(1000000)).toBe("1M");
    expect(compactNum(1500000)).toBe("1.5M");
  });
});
