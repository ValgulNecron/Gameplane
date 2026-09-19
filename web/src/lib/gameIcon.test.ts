import { describe, expect, it } from "vitest";
import { assignGameCodes, assignGameCodesForTemplates } from "./gameIcon";

describe("assignGameCodes", () => {
  it("gives the oldest template its first-choice letter pair, and a later one with the same first letter the next available pair", () => {
    const codes = assignGameCodes([
      { name: "minecraft-java", creationTimestamp: "2026-01-01T00:00:00Z" },
      { name: "minecraft-bedrock", creationTimestamp: "2026-02-01T00:00:00Z" },
    ]);
    // "minecraft" -> M + i,n,e,c,r,a,f,t: java (oldest) claims "MI" first.
    expect(codes.get("minecraft-java")).toBe("MI");
    // bedrock's own subsequent letters are i,n,e,c,r,a,f,t,b,e,d,r,o,c,k;
    // "MI" is taken, so it falls through to the next unused candidate, "MN".
    expect(codes.get("minecraft-bedrock")).toBe("MN");
  });

  it("does not depend on input array order — only on creationTimestamp", () => {
    const older = { name: "minecraft-java", creationTimestamp: "2026-01-01T00:00:00Z" };
    const newer = { name: "minecraft-bedrock", creationTimestamp: "2026-02-01T00:00:00Z" };
    const forward = assignGameCodes([older, newer]);
    const reversed = assignGameCodes([newer, older]);
    expect(forward.get("minecraft-java")).toBe(reversed.get("minecraft-java"));
    expect(forward.get("minecraft-bedrock")).toBe(reversed.get("minecraft-bedrock"));
    expect(reversed.get("minecraft-java")).toBe("MI");
  });

  it("treats a missing creationTimestamp as installed after every template that has one", () => {
    // Listed first in the array, but with no timestamp — should still lose
    // its preferred code to the template that has a real (later) timestamp.
    const codes = assignGameCodes([
      { name: "valheim" }, // no timestamp
      { name: "valheim-modded", creationTimestamp: "2026-03-01T00:00:00Z" },
    ]);
    expect(codes.get("valheim-modded")).toBe("VA");
    expect(codes.get("valheim")).not.toBe("VA");
  });

  it("falls back to first-letter+digit once every letter pair for a name is exhausted", () => {
    // Two templates that reduce to the identical letter sequence "at":
    // the first claims "AT"; the second has no other subsequent letter to
    // try, so it falls back to the digit sequence starting at 2.
    const codes = assignGameCodes([
      { name: "at", creationTimestamp: "2026-01-01T00:00:00Z" },
      { name: "at-2", creationTimestamp: "2026-01-02T00:00:00Z" },
    ]);
    expect(codes.get("at")).toBe("AT");
    expect(codes.get("at-2")).toBe("A2");
  });

  it("uses the digit fallback directly for a single-letter name (no subsequent letter to pair with)", () => {
    const codes = assignGameCodes([{ name: "x", creationTimestamp: "2026-01-01T00:00:00Z" }]);
    expect(codes.get("x")).toBe("X2");
  });

  it("assignGameCodesForTemplates reads metadata.name and metadata.creationTimestamp from GameTemplate objects", () => {
    const templates = [
      {
        metadata: { name: "terraria", creationTimestamp: "2026-01-01T00:00:00Z" },
        spec: { displayName: "Terraria", game: "terraria", version: "1", image: "x" },
      },
    ] as Parameters<typeof assignGameCodesForTemplates>[0];
    const codes = assignGameCodesForTemplates(templates);
    expect(codes.get("terraria")).toBe("TE");
  });

  it("falls back to '?' as the first letter when the name has no letters at all", () => {
    const codes = assignGameCodes([{ name: "123", creationTimestamp: "2026-01-01T00:00:00Z" }]);
    expect(codes.get("123")).toBe("?2");
  });

  it("the later entry's computed code wins when two inputs share the same name", () => {
    const codes = assignGameCodes([
      { name: "dup", creationTimestamp: "2026-01-01T00:00:00Z" },
      { name: "dup", creationTimestamp: "2026-01-02T00:00:00Z" },
    ]);
    expect(codes.size).toBe(1);
    expect(codes.get("dup")).toBe("DP");
  });
});
