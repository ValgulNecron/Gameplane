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
vi.mock("@/lib/endpoints", () => ({
  Modules: {
    catalog: () => catalog(),
    install: (body: unknown) => install(body),
    upgrade: (name: string, version: string) => upgrade(name, version),
    uninstall: (name: string) => uninstall(name),
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
  sources: ["default"],
  versions: ["1.1.0", "1.0.0"],
  latestVersion: "1.1.0",
  installed: false,
};

const VALHEIM_INSTALLED: CatalogEntry = {
  name: "valheim",
  displayName: "Valheim",
  sources: ["default"],
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
  sources: ["default"],
  versions: ["1.1.0", "1.0.0"],
  latestVersion: "1.1.0",
  installed: true,
  installedVersion: "1.0.0",
  installedFrom: "default",
  moduleName: "terraria",
  phase: "Ready",
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
});
