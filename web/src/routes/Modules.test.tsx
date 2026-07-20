import { afterEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import { ModulesPage } from "./Modules";
import type { CatalogEntry } from "@/types";

// Modules.tsx calls Modules.catalog/install/upgrade/uninstall — stub
// the entire endpoints module so the test asserts on the wire calls
// without spinning up fetch mocks.
const catalog = vi.fn();
const install = vi.fn();
const upgrade = vi.fn();
const uninstall = vi.fn();
const removeUpload = vi.fn();
const listSources = vi.fn();
vi.mock("@/lib/endpoints", () => ({
  Modules: {
    catalog: () => catalog(),
    install: (body: unknown) => install(body),
    upgrade: (name: string, version: string) => upgrade(name, version),
    uninstall: (name: string) => uninstall(name),
  },
  ModuleSources: {
    list: () => listSources(),
    removeUpload: (source: string, module: string) => removeUpload(source, module),
  },
}));

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, ...rest }: { children: ReactNode } & Record<string, unknown>) => (
    <a {...rest}>{children}</a>
  ),
}));

const MINECRAFT: CatalogEntry = {
  name: "minecraft-java",
  displayName: "Minecraft (Java)",
  summary: "Vanilla / Paper / Forge",
  game: "minecraft-java",
  sources: [{ name: "default", type: "oci" }],
  versions: ["1.1.0", "1.0.0"],
  latestVersion: "1.1.0",
  installed: false,
};

const VALHEIM_INSTALLED: CatalogEntry = {
  name: "valheim",
  displayName: "Valheim",
  sources: [{ name: "default", type: "oci" }],
  versions: ["0.9.0"],
  latestVersion: "0.9.0",
  installed: true,
  installedVersion: "0.9.0",
  installedFrom: "default",
  moduleName: "valheim",
  phase: "Ready",
};

const TERRARIA_UPGRADE: CatalogEntry = {
  name: "terraria",
  displayName: "Terraria",
  sources: [{ name: "default", type: "oci" }],
  versions: ["1.1.0", "1.0.0"],
  latestVersion: "1.1.0",
  installed: true,
  installedVersion: "1.0.0",
  installedFrom: "default",
  moduleName: "terraria",
  phase: "Ready",
};

const CUSTOM_UPLOADED: CatalogEntry = {
  name: "custom-game",
  displayName: "Custom Game",
  summary: "Custom uploaded module",
  game: "custom",
  sources: [{ name: "uploads", type: "upload" }],
  versions: ["1.0.0"],
  latestVersion: "1.0.0",
  installed: false,
};

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <ModulesPage />
    </QueryClientProvider>,
  );
}

afterEach(() => {
  catalog.mockReset();
  install.mockReset();
  upgrade.mockReset();
  uninstall.mockReset();
  removeUpload.mockReset();
  listSources.mockReset();
});

describe("ModulesPage", () => {
  it("renders the catalog with installation state per entry", async () => {
    catalog.mockResolvedValue({ items: [MINECRAFT, VALHEIM_INSTALLED, TERRARIA_UPGRADE] });
    renderPage();

    expect(await screen.findByText("Minecraft (Java)")).toBeInTheDocument();
    expect(screen.getByText("Valheim")).toBeInTheDocument();
    expect(screen.getByText("Terraria")).toBeInTheDocument();

    // Minecraft is not installed → version label shows the latest tag.
    expect(screen.getByText(/v1\.1\.0$/)).toBeInTheDocument();
    // Valheim is installed → "v0.9.0 installed".
    expect(screen.getByText(/v0\.9\.0 installed/)).toBeInTheDocument();
    // Terraria has an upgrade available.
    expect(screen.getByText(/v1\.0\.0 installed.*v1\.1\.0 available/)).toBeInTheDocument();
  });

  it("filters the catalog by game category (multi-select)", async () => {
    const minecraftEntry = { ...MINECRAFT, categories: ["Sandbox", "Survival"] };
    const valheimEntry = { ...VALHEIM_INSTALLED, game: "valheim", categories: ["Survival"] };
    catalog.mockResolvedValue({
      items: [minecraftEntry, valheimEntry],
    });
    renderPage();
    expect(await screen.findByText("Minecraft (Java)")).toBeInTheDocument();
    expect(screen.getByText("Valheim")).toBeInTheDocument();

    // Both declared categories should have chips. Click Sandbox to filter.
    const sandboxBtn = screen.getByRole("button", { name: "Sandbox" });
    expect(sandboxBtn).toBeInTheDocument();
    await userEvent.click(sandboxBtn);
    expect(screen.getByText("Minecraft (Java)")).toBeInTheDocument();
    expect(screen.queryByText("Valheim")).toBeNull();

    // Click Survival to show modules in either Sandbox or Survival
    const survivalBtn = screen.getByRole("button", { name: "Survival" });
    await userEvent.click(survivalBtn);
    expect(screen.getByText("Minecraft (Java)")).toBeInTheDocument();
    expect(screen.getByText("Valheim")).toBeInTheDocument();
  });

  it("shows modules matching any of the selected categories (multi-select)", async () => {
    const minecraftEntry = { ...MINECRAFT, categories: ["Sandbox", "Survival"] };
    const valheimEntry = { ...VALHEIM_INSTALLED, game: "valheim", categories: ["Survival"] };
    const terraria = { ...TERRARIA_UPGRADE, game: "terraria", categories: ["Sandbox"] };
    catalog.mockResolvedValue({
      items: [minecraftEntry, valheimEntry, terraria],
    });
    renderPage();
    expect(await screen.findByText("Minecraft (Java)")).toBeInTheDocument();
    expect(screen.getByText("Valheim")).toBeInTheDocument();
    expect(screen.getByText("Terraria")).toBeInTheDocument();

    // Click Sandbox: shows Minecraft and Terraria
    await userEvent.click(screen.getByRole("button", { name: "Sandbox" }));
    expect(screen.getByText("Minecraft (Java)")).toBeInTheDocument();
    expect(screen.queryByText("Valheim")).not.toBeInTheDocument();
    expect(screen.getByText("Terraria")).toBeInTheDocument();

    // Click Survival: now shows all three (Sandbox OR Survival)
    await userEvent.click(screen.getByRole("button", { name: "Survival" }));
    expect(screen.getByText("Minecraft (Java)")).toBeInTheDocument();
    expect(screen.getByText("Valheim")).toBeInTheDocument();
    expect(screen.getByText("Terraria")).toBeInTheDocument();

    // Click Survival again to deselect it: shows only Sandbox (Minecraft and Terraria)
    await userEvent.click(screen.getByRole("button", { name: "Survival" }));
    expect(screen.getByText("Minecraft (Java)")).toBeInTheDocument();
    expect(screen.queryByText("Valheim")).not.toBeInTheDocument();
    expect(screen.getByText("Terraria")).toBeInTheDocument();

    // Click Sandbox to deselect it: shows all (no filters)
    await userEvent.click(screen.getByRole("button", { name: "Sandbox" }));
    expect(screen.getByText("Minecraft (Java)")).toBeInTheDocument();
    expect(screen.getByText("Valheim")).toBeInTheDocument();
    expect(screen.getByText("Terraria")).toBeInTheDocument();
  });

  it("opens the install dialog and POSTs /modules with the chosen version", async () => {
    catalog.mockResolvedValue({ items: [MINECRAFT] });
    install.mockResolvedValue({});
    renderPage();

    await screen.findByText("Minecraft (Java)");
    await userEvent.click(screen.getByRole("button", { name: /install/i }));

    // Dialog appears with "Install minecraft-java" title.
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    // Confirm install.
    const confirms = screen.getAllByRole("button", { name: /^install$/i });
    await userEvent.click(confirms[confirms.length - 1]);

    await waitFor(() => expect(install).toHaveBeenCalledTimes(1));
    expect(install.mock.calls[0][0]).toMatchObject({
      source: "default",
      module: "minecraft-java",
      name: "minecraft-java",
      version: "1.1.0",
    });
  });

  it("upgrades an installed module to the latest version", async () => {
    catalog.mockResolvedValue({ items: [TERRARIA_UPGRADE] });
    upgrade.mockResolvedValue({});
    renderPage();

    await screen.findByText("Terraria");
    await userEvent.click(screen.getByRole("button", { name: /upgrade/i }));

    await waitFor(() => expect(upgrade).toHaveBeenCalledWith("terraria", "1.1.0"));
  });

  it("uninstalls after confirmation", async () => {
    catalog.mockResolvedValue({ items: [VALHEIM_INSTALLED] });
    uninstall.mockResolvedValue(undefined);
    renderPage();

    await screen.findByText("Valheim");
    await userEvent.click(screen.getByRole("button", { name: /uninstall/i }));
    // Confirm dialog button (label "Uninstall" again, in the dialog).
    const buttons = await screen.findAllByRole("button", { name: /uninstall/i });
    await userEvent.click(buttons[buttons.length - 1]);

    await waitFor(() => expect(uninstall).toHaveBeenCalledWith("valheim"));
  });

  it("shows the upload action only when an upload-type source exists", async () => {
    catalog.mockResolvedValue({ items: [MINECRAFT] });
    listSources.mockResolvedValue({
      items: [
        { metadata: { name: "default" }, spec: { type: "oci" } },
        { metadata: { name: "uploads" }, spec: { type: "upload" } },
      ],
    });
    renderPage();

    await screen.findByText("Minecraft (Java)");
    const uploadBtn = await screen.findByRole("button", { name: /upload module/i });
    await userEvent.click(uploadBtn);
    expect(await screen.findByText(/Choose a bundle archive/)).toBeInTheDocument();
  });

  it("hides the upload action without upload sources", async () => {
    catalog.mockResolvedValue({ items: [MINECRAFT] });
    listSources.mockResolvedValue({
      items: [{ metadata: { name: "default" }, spec: { type: "oci" } }],
    });
    renderPage();
    await screen.findByText("Minecraft (Java)");
    expect(screen.queryByRole("button", { name: /upload module/i })).not.toBeInTheDocument();
  });

  it("shows 'Remove upload' button only on upload-type entries", async () => {
    catalog.mockResolvedValue({ items: [MINECRAFT, CUSTOM_UPLOADED] });
    renderPage();

    await screen.findByText("Minecraft (Java)");

    // There should be exactly one "Remove upload" button (for CUSTOM_UPLOADED)
    expect(screen.getByRole("button", { name: /remove upload/i })).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: /remove upload/i })).toHaveLength(1);
  });

  it("removes an uploaded module after confirmation", async () => {
    catalog.mockResolvedValue({ items: [CUSTOM_UPLOADED] });
    removeUpload.mockResolvedValue(undefined);
    renderPage();

    await screen.findByText("Custom Game");
    await userEvent.click(screen.getByRole("button", { name: /remove upload/i }));

    // Confirm dialog button (label "Remove upload" in the dialog).
    const buttons = await screen.findAllByRole("button", { name: /remove upload/i });
    await userEvent.click(buttons[buttons.length - 1]);

    await waitFor(() =>
      expect(removeUpload).toHaveBeenCalledWith("uploads", "custom-game"),
    );
  });
});
