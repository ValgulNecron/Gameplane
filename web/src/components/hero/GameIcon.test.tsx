import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import { GameIcon } from "./GameIcon";

describe("GameIcon", () => {
  it("tints from a declared accentColor", () => {
    const { container } = render(<GameIcon game="minecraft-java" accentColor="#5b9a3e" />);
    const tile = container.firstChild as HTMLElement;
    // Inline color set from the hex; legacy palette class is not used.
    expect(tile.style.color).toBe("rgb(91, 154, 62)");
    expect(tile.className).not.toContain("text-success");
    expect(tile.textContent).toBe("mi");
  });

  it("falls back to the legacy palette without an accentColor", () => {
    const { container } = render(<GameIcon game="valheim" />);
    const tile = container.firstChild as HTMLElement;
    expect(tile.className).toContain("text-warning");
    expect(tile.style.color).toBe("");
  });

  it("ignores a malformed accentColor and uses the fallback", () => {
    const { container } = render(<GameIcon game="valheim" accentColor="red" />);
    const tile = container.firstChild as HTMLElement;
    expect(tile.style.color).toBe("");
    expect(tile.className).toContain("text-warning");
  });

  it("uses the correct size classes based on size prop", () => {
    const { container: smContainer } = render(<GameIcon game="minecraft-java" size="sm" />);
    const smTile = smContainer.firstChild as HTMLElement;
    expect(smTile.className).toContain("h-7");
    expect(smTile.className).toContain("w-7");
    expect(smTile.className).toContain("text-xs");

    const { container: mdContainer } = render(<GameIcon game="minecraft-java" size="md" />);
    const mdTile = mdContainer.firstChild as HTMLElement;
    expect(mdTile.className).toContain("h-9");
    expect(mdTile.className).toContain("w-9");
    expect(mdTile.className).toContain("text-sm");

    const { container: lgContainer } = render(<GameIcon game="minecraft-java" size="lg" />);
    const lgTile = lgContainer.firstChild as HTMLElement;
    expect(lgTile.className).toContain("h-12");
    expect(lgTile.className).toContain("w-12");
    expect(lgTile.className).toContain("text-base");
  });

  it("defaults to md size when size prop is omitted", () => {
    const { container } = render(<GameIcon game="terraria" />);
    const tile = container.firstChild as HTMLElement;
    expect(tile.className).toContain("h-9");
    expect(tile.className).toContain("w-9");
    expect(tile.className).toContain("text-sm");
  });

  it("displays question mark when game is undefined", () => {
    const { container } = render(<GameIcon />);
    const tile = container.firstChild as HTMLElement;
    expect(tile.textContent).toBe("??");
    expect(tile.className).toContain("text-muted");
  });

  it("displays correct text for Terraria and other games in legacy palette", () => {
    const { container } = render(<GameIcon game="terraria" />);
    const tile = container.firstChild as HTMLElement;
    expect(tile.textContent).toBe("te");
    expect(tile.className).toContain("text-success");
  });

  it("respects accentColor even when game has a legacy palette entry", () => {
    const { container } = render(<GameIcon game="ark" accentColor="#ff00ff" />);
    const tile = container.firstChild as HTMLElement;
    // Should use inline style, not the legacy danger color
    expect(tile.style.color).toBe("rgb(255, 0, 255)");
    expect(tile.className).not.toContain("text-danger");
  });
});
