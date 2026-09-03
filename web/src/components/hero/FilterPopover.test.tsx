import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FilterPopover } from "./FilterPopover";

describe("FilterPopover", () => {
  it("renders the trigger button and popover content when open", () => {
    const onOpenChange = vi.fn();
    render(
      <FilterPopover
        isOpen={true}
        onOpenChange={onOpenChange}
        games={["minecraft-java", "valheim"]}
        selectedGames={new Set()}
        onToggleGame={vi.fn()}
        namespaces={["gameplane-games", "gameplane-extra"]}
        selectedNamespaces={new Set()}
        onToggleNamespace={vi.fn()}
        onApply={vi.fn()}
        onClear={vi.fn()}
        contentClassName="test-content"
      >
        Filter
      </FilterPopover>
    );

    // Verify the trigger button is rendered
    expect(screen.getByRole("button", { name: /^Filter/i })).toBeInTheDocument();

    // Verify section headers are rendered
    expect(screen.getByText("Game")).toBeInTheDocument();
    expect(screen.getByText("Namespace")).toBeInTheDocument();

    // Verify all game and namespace items are rendered
    expect(screen.getByText("minecraft-java")).toBeInTheDocument();
    expect(screen.getByText("valheim")).toBeInTheDocument();
    expect(screen.getByText("gameplane-games")).toBeInTheDocument();
    expect(screen.getByText("gameplane-extra")).toBeInTheDocument();

    // Verify footer buttons are rendered
    expect(screen.getByRole("button", { name: "Clear all filters" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Apply filters" })).toBeInTheDocument();
  });

  it("calls onToggleGame when a game checkbox is toggled", async () => {
    const user = userEvent.setup();
    const onToggleGame = vi.fn();

    render(
      <FilterPopover
        isOpen={true}
        onOpenChange={vi.fn()}
        games={["minecraft-java"]}
        selectedGames={new Set()}
        onToggleGame={onToggleGame}
        namespaces={[]}
        selectedNamespaces={new Set()}
        onToggleNamespace={vi.fn()}
        onApply={vi.fn()}
        onClear={vi.fn()}
      >
        Filter
      </FilterPopover>
    );

    // Find and click the checkbox for minecraft-java
    const gameCheckbox = screen.getByRole("checkbox", { name: "minecraft-java" });
    await user.click(gameCheckbox);

    // Verify the callback was called with the correct game name
    expect(onToggleGame).toHaveBeenCalledWith("minecraft-java");
  });

  it("calls onToggleNamespace when a namespace checkbox is toggled", async () => {
    const user = userEvent.setup();
    const onToggleNamespace = vi.fn();

    render(
      <FilterPopover
        isOpen={true}
        onOpenChange={vi.fn()}
        games={[]}
        selectedGames={new Set()}
        onToggleGame={vi.fn()}
        namespaces={["gameplane-games"]}
        selectedNamespaces={new Set()}
        onToggleNamespace={onToggleNamespace}
        onApply={vi.fn()}
        onClear={vi.fn()}
      >
        Filter
      </FilterPopover>
    );

    // Find and click the checkbox for gameplane-games
    const nsCheckbox = screen.getByRole("checkbox", { name: "gameplane-games" });
    await user.click(nsCheckbox);

    // Verify the callback was called with the correct namespace name
    expect(onToggleNamespace).toHaveBeenCalledWith("gameplane-games");
  });

  it("calls onApply when the Apply button is clicked", async () => {
    const user = userEvent.setup();
    const onApply = vi.fn();

    render(
      <FilterPopover
        isOpen={true}
        onOpenChange={vi.fn()}
        games={["minecraft-java"]}
        selectedGames={new Set(["minecraft-java"])}
        onToggleGame={vi.fn()}
        namespaces={[]}
        selectedNamespaces={new Set()}
        onToggleNamespace={vi.fn()}
        onApply={onApply}
        onClear={vi.fn()}
      >
        Filter
      </FilterPopover>
    );

    const applyButton = screen.getByRole("button", { name: "Apply filters" });
    await user.click(applyButton);

    expect(onApply).toHaveBeenCalledOnce();
  });

  it("calls onClear when the Clear button is clicked", async () => {
    const user = userEvent.setup();
    const onClear = vi.fn();

    render(
      <FilterPopover
        isOpen={true}
        onOpenChange={vi.fn()}
        games={["minecraft-java"]}
        selectedGames={new Set(["minecraft-java"])}
        onToggleGame={vi.fn()}
        namespaces={["gameplane-games"]}
        selectedNamespaces={new Set(["gameplane-games"])}
        onToggleNamespace={vi.fn()}
        onApply={vi.fn()}
        onClear={onClear}
      >
        Filter
      </FilterPopover>
    );

    const clearButton = screen.getByRole("button", { name: "Clear all filters" });
    await user.click(clearButton);

    expect(onClear).toHaveBeenCalledOnce();
  });

  it("displays checked checkboxes for selected items", () => {
    render(
      <FilterPopover
        isOpen={true}
        onOpenChange={vi.fn()}
        games={["minecraft-java", "valheim"]}
        selectedGames={new Set(["minecraft-java"])}
        onToggleGame={vi.fn()}
        namespaces={["gameplane-games", "gameplane-extra"]}
        selectedNamespaces={new Set(["gameplane-games"])}
        onToggleNamespace={vi.fn()}
        onApply={vi.fn()}
        onClear={vi.fn()}
      >
        Filter
      </FilterPopover>
    );

    // Verify that selected items have checked checkboxes
    const minecraftCheckbox = screen.getByRole("checkbox", { name: "minecraft-java" }) as HTMLInputElement;
    expect(minecraftCheckbox.checked).toBe(true);

    const valheimCheckbox = screen.getByRole("checkbox", { name: "valheim" }) as HTMLInputElement;
    expect(valheimCheckbox.checked).toBe(false);

    const gameplazeGamesCheckbox = screen.getByRole("checkbox", { name: "gameplane-games" }) as HTMLInputElement;
    expect(gameplazeGamesCheckbox.checked).toBe(true);

    const gameplazeExtraCheckbox = screen.getByRole("checkbox", { name: "gameplane-extra" }) as HTMLInputElement;
    expect(gameplazeExtraCheckbox.checked).toBe(false);
  });

  it("handles empty games and namespaces gracefully", () => {
    render(
      <FilterPopover
        isOpen={true}
        onOpenChange={vi.fn()}
        games={[]}
        selectedGames={new Set()}
        onToggleGame={vi.fn()}
        namespaces={[]}
        selectedNamespaces={new Set()}
        onToggleNamespace={vi.fn()}
        onApply={vi.fn()}
        onClear={vi.fn()}
      >
        Filter
      </FilterPopover>
    );

    // Verify footer buttons are still rendered
    expect(screen.getByRole("button", { name: "Clear all filters" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Apply filters" })).toBeInTheDocument();

    // Verify no section headers are rendered
    expect(screen.queryByText("Game")).not.toBeInTheDocument();
    expect(screen.queryByText("Namespace")).not.toBeInTheDocument();
  });

  it("calls onOpenChange when the popover trigger is clicked", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();

    render(
      <FilterPopover
        isOpen={false}
        onOpenChange={onOpenChange}
        games={[]}
        selectedGames={new Set()}
        onToggleGame={vi.fn()}
        namespaces={[]}
        selectedNamespaces={new Set()}
        onToggleNamespace={vi.fn()}
        onApply={vi.fn()}
        onClear={vi.fn()}
      >
        <span>Filter</span>
      </FilterPopover>
    );

    const trigger = screen.getByRole("button", { name: "Filter" });
    await user.click(trigger);

    // The onOpenChange callback should have been called
    expect(onOpenChange).toHaveBeenCalled();
  });

  it("renders multiple games and namespaces in correct sections", () => {
    const games = ["minecraft-java", "valheim", "terraria"];
    const namespaces = ["gameplane-games", "gameplane-extra", "custom-ns"];

    render(
      <FilterPopover
        isOpen={true}
        onOpenChange={vi.fn()}
        games={games}
        selectedGames={new Set()}
        onToggleGame={vi.fn()}
        namespaces={namespaces}
        selectedNamespaces={new Set()}
        onToggleNamespace={vi.fn()}
        onApply={vi.fn()}
        onClear={vi.fn()}
      >
        Filter
      </FilterPopover>
    );

    // Verify all games are rendered
    games.forEach((game) => {
      expect(screen.getByText(game)).toBeInTheDocument();
    });

    // Verify all namespaces are rendered
    namespaces.forEach((ns) => {
      expect(screen.getByText(ns)).toBeInTheDocument();
    });

    // Verify that each item has an associated checkbox
    games.forEach((game) => {
      expect(screen.getByRole("checkbox", { name: game })).toBeInTheDocument();
    });

    namespaces.forEach((ns) => {
      expect(screen.getByRole("checkbox", { name: ns })).toBeInTheDocument();
    });
  });
});
