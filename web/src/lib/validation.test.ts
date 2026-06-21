import { describe, expect, it } from "vitest";
import {
  defaultVersionId,
  isValidK8sName,
  isValidQuantity,
  isValidVersion,
  validateConfig,
  type ConfigField,
} from "./validation";
import type { GameTemplate } from "@/types";

function tmplWithVersions(versions?: GameTemplate["spec"]["versions"]): GameTemplate {
  return {
    metadata: { name: "minecraft-java" },
    spec: {
      displayName: "Minecraft",
      game: "minecraft-java",
      version: "2.0.0",
      image: "itzg/minecraft-server:java21",
      versions,
    },
  };
}

describe("isValidK8sName", () => {
  it.each([
    ["mc", true],
    ["mc-hardcore", true],
    ["x".repeat(63), true],
    ["a1b2c3", true],
  ])("accepts %s", (s, expected) => {
    expect(isValidK8sName(s)).toBe(expected);
  });

  it.each([
    "",
    "MC",
    "mc test",
    "-mc",
    "mc-",
    "mc_test",
    "x".repeat(64),
    "1.2.3",
  ])("rejects %s", (s) => {
    expect(isValidK8sName(s)).toBe(false);
  });
});

describe("isValidQuantity", () => {
  it.each(["50Gi", "1Ti", "100Mi", "500m", "2", "1.5Gi"])("accepts %s", (s) => {
    expect(isValidQuantity(s)).toBe(true);
  });

  it.each(["", "50G ", " 50Gi", "50 Gi", "50gi", "Gi", "abc", "-5Gi"])(
    "rejects %s",
    (s) => {
      expect(isValidQuantity(s)).toBe(false);
    },
  );

  it("accepts plain integers (e.g. CPU count or millicores)", () => {
    expect(isValidQuantity("2")).toBe(true);
    expect(isValidQuantity("500m")).toBe(true);
  });
});

describe("validateConfig", () => {
  const schema: ConfigField[] = [
    { name: "TYPE", displayName: "Server type", type: "enum", enum: ["VANILLA", "PAPER"], required: true, default: "VANILLA" },
    { name: "VERSION", type: "string", required: true },
    { name: "DIFFICULTY", type: "enum", enum: ["peaceful", "easy", "normal", "hard"] },
    { name: "MOTD", type: "string" },
  ];

  it("returns no errors when required fields have values", () => {
    expect(validateConfig(schema, { VERSION: "1.21" })).toEqual([]);
  });

  it("flags missing required fields", () => {
    const errs = validateConfig(schema, {});
    expect(errs.map((e) => e.name)).toEqual(["VERSION"]);
  });

  it("flags enum mismatch", () => {
    const errs = validateConfig(schema, { VERSION: "1.21", DIFFICULTY: "extreme" });
    expect(errs).toHaveLength(1);
    expect(errs[0].name).toBe("DIFFICULTY");
    expect(errs[0].message).toContain("peaceful");
  });

  it("treats default as a satisfying value for required enum fields", () => {
    expect(validateConfig(schema, { VERSION: "1.21" })).toEqual([]);
  });
});

describe("isValidVersion", () => {
  const versions = [
    { id: "1.21.4-paper", displayName: "1.21.4 Paper", loader: "paper", default: true },
    { id: "1.21.4-forge", displayName: "1.21.4 Forge", loader: "forge" },
  ];

  it("is true when the template declares no versions", () => {
    expect(isValidVersion(tmplWithVersions(undefined), undefined)).toBe(true);
    expect(isValidVersion(tmplWithVersions([]), "anything")).toBe(true);
  });

  it("requires a value when the template declares versions", () => {
    expect(isValidVersion(tmplWithVersions(versions), undefined)).toBe(false);
    expect(isValidVersion(tmplWithVersions(versions), "")).toBe(false);
  });

  it("accepts a matching id and rejects an unknown one", () => {
    expect(isValidVersion(tmplWithVersions(versions), "1.21.4-forge")).toBe(true);
    expect(isValidVersion(tmplWithVersions(versions), "9.9-bogus")).toBe(false);
  });
});

describe("defaultVersionId", () => {
  it("returns undefined without a catalog", () => {
    expect(defaultVersionId(tmplWithVersions(undefined))).toBeUndefined();
    expect(defaultVersionId(tmplWithVersions([]))).toBeUndefined();
  });

  it("prefers the entry marked default", () => {
    expect(
      defaultVersionId(
        tmplWithVersions([
          { id: "a", displayName: "A" },
          { id: "b", displayName: "B", default: true },
        ]),
      ),
    ).toBe("b");
  });

  it("falls back to the first entry when none is marked default", () => {
    expect(
      defaultVersionId(
        tmplWithVersions([
          { id: "a", displayName: "A" },
          { id: "b", displayName: "B" },
        ]),
      ),
    ).toBe("a");
  });
});
