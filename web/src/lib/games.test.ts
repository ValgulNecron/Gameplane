import { describe, expect, it } from "vitest";
import { OTHER_CATEGORY, categoryFilters, gameCategory, resolveCategories } from "./games";

describe("resolveCategories", () => {
  it("returns the declared categories", () => {
    expect(resolveCategories(["Sandbox", "Survival"], "minecraft-java")).toEqual([
      "Sandbox",
      "Survival",
    ]);
  });

  it("falls back to the game-slug heuristic when none are declared", () => {
    expect(resolveCategories(undefined, "valheim")).toEqual(["Survival"]);
    expect(resolveCategories([], "minecraft-java")).toEqual(["Sandbox"]);
  });

  it("falls back to Other for an unknown game", () => {
    expect(resolveCategories(undefined, "some-unknown-game")).toEqual([OTHER_CATEGORY]);
  });

  it("drops blank entries and trims", () => {
    expect(resolveCategories(["  Sandbox  ", "", "   "], "minecraft-java")).toEqual(["Sandbox"]);
  });

  it("falls back to the heuristic when every declared entry is blank", () => {
    expect(resolveCategories(["", "  "], "valheim")).toEqual(["Survival"]);
  });
});

describe("categoryFilters", () => {
  it("flattens per-module lists, sorts named categories, and puts Other last", () => {
    expect(
      categoryFilters([
        ["Sandbox", "Survival"],
        ["Survival", "Co-op"],
        [OTHER_CATEGORY],
      ]),
    ).toEqual(["all", "Co-op", "Sandbox", "Survival", OTHER_CATEGORY]);
  });

  it("omits Other when no module falls into it", () => {
    expect(categoryFilters([["Shooter"], ["PvP"]])).toEqual(["all", "PvP", "Shooter"]);
  });

  it("returns just all for an empty catalog", () => {
    expect(categoryFilters([])).toEqual(["all"]);
  });

  it("dedupes case-insensitively, keeping the first spelling", () => {
    expect(categoryFilters([["Survival"], ["survival"]])).toEqual(["all", "Survival"]);
  });
});

describe("gameCategory", () => {
  it("still classifies known slugs", () => {
    expect(gameCategory("valheim")).toBe("Survival");
    expect(gameCategory("minecraft-java")).toBe("Sandbox");
    expect(gameCategory("cs2")).toBe("Shooter");
    expect(gameCategory("nothing-like-this")).toBe(OTHER_CATEGORY);
  });
});
