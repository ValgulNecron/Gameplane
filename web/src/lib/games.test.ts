import { describe, expect, it } from "vitest";
import { OTHER_CATEGORY, categoryFilters, gameCategory, resolveCategories, matchesCategory, matchesAnyCategory } from "./games";

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

  it("handles Other case-insensitively, pinning it last", () => {
    expect(categoryFilters([["Sandbox"], ["other"]])).toEqual(["all", "Sandbox", OTHER_CATEGORY]);
    expect(categoryFilters([["Sandbox"], ["OTHER"]])).toEqual(["all", "Sandbox", OTHER_CATEGORY]);
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

describe("matchesCategory", () => {
  it("matches the all chip", () => {
    expect(matchesCategory(["Survival"], "x", "all")).toBe(true);
    expect(matchesCategory(undefined, "x", "all")).toBe(true);
  });

  it("matches categories case-insensitively", () => {
    expect(matchesCategory(["survival"], "x", "Survival")).toBe(true);
    expect(matchesCategory(["Survival"], "x", "survival")).toBe(true);
    expect(matchesCategory(["SURVIVAL"], "x", "survival")).toBe(true);
  });

  it("returns false for non-matching categories", () => {
    expect(matchesCategory(["Sandbox"], "x", "Survival")).toBe(false);
  });

  it("falls back to game heuristic when categories are undefined", () => {
    expect(matchesCategory(undefined, "valheim", "Survival")).toBe(true);
    expect(matchesCategory(undefined, "minecraft-java", "Sandbox")).toBe(true);
  });

  it("matches heuristic categories case-insensitively", () => {
    expect(matchesCategory(undefined, "valheim", "survival")).toBe(true);
    expect(matchesCategory(undefined, "minecraft-java", "sandbox")).toBe(true);
  });

  it("handles multiple categories", () => {
    expect(matchesCategory(["Sandbox", "Survival"], "x", "Survival")).toBe(true);
    expect(matchesCategory(["Sandbox", "Survival"], "x", "survival")).toBe(true);
    expect(matchesCategory(["Sandbox", "Survival"], "x", "Shooter")).toBe(false);
  });
});

describe("matchesAnyCategory", () => {
  it("returns true for empty selection (show everything)", () => {
    expect(matchesAnyCategory(["Survival"], "x", new Set())).toBe(true);
    expect(matchesAnyCategory(["Sandbox"], "x", new Set())).toBe(true);
    expect(matchesAnyCategory(undefined, "x", new Set())).toBe(true);
  });

  it("returns true when selection has one matching category", () => {
    expect(matchesAnyCategory(["Survival"], "x", new Set(["Survival"]))).toBe(true);
    expect(matchesAnyCategory(["Sandbox", "Survival"], "x", new Set(["Survival"]))).toBe(true);
  });

  it("returns false when selection has no matching categories", () => {
    expect(matchesAnyCategory(["Survival"], "x", new Set(["Sandbox"]))).toBe(false);
    expect(matchesAnyCategory(["Sandbox"], "x", new Set(["Survival", "Shooter"]))).toBe(false);
  });

  it("returns true when selection has one of multiple matching categories", () => {
    expect(matchesAnyCategory(["Sandbox", "Survival", "Co-op"], "x", new Set(["Survival"]))).toBe(
      true
    );
    expect(matchesAnyCategory(["Sandbox", "Survival", "Co-op"], "x", new Set(["Co-op", "Other"]))).toBe(
      true
    );
  });

  it("matches case-insensitively", () => {
    expect(matchesAnyCategory(["Survival"], "x", new Set(["survival"]))).toBe(true);
    expect(matchesAnyCategory(["survival"], "x", new Set(["Survival"]))).toBe(true);
    expect(matchesAnyCategory(["SANDBOX", "survival"], "x", new Set(["sandbox", "SURVIVAL"]))).toBe(
      true
    );
  });

  it("uses game heuristic when categories are undefined", () => {
    expect(matchesAnyCategory(undefined, "valheim", new Set(["Survival"]))).toBe(true);
    expect(matchesAnyCategory(undefined, "minecraft-java", new Set(["Sandbox"]))).toBe(true);
    expect(matchesAnyCategory(undefined, "valheim", new Set(["Sandbox"]))).toBe(false);
  });
});
