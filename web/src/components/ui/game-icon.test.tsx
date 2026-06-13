import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import { GameIcon } from "./game-icon";

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
});
