import { describe, expect, it } from "vitest";
import { resolveConsoleMode, rconAvailable } from "./capabilities";
import type { GameTemplate } from "@/types";

function tmpl(spec: Partial<GameTemplate["spec"]>): GameTemplate {
  return {
    metadata: { name: "t" },
    spec: { displayName: "T", game: "g", version: "1", image: "img", ...spec },
  };
}

describe("resolveConsoleMode", () => {
  it("defaults to rcon while the template is unknown", () => {
    expect(resolveConsoleMode(undefined)).toBe("rcon");
  });

  it("honors an explicit consoleMode", () => {
    expect(resolveConsoleMode(tmpl({ consoleMode: "pty" }))).toBe("pty");
    expect(resolveConsoleMode(tmpl({ consoleMode: "none" }))).toBe("none");
  });

  it("derives rcon from an rcon protocol when consoleMode is unset", () => {
    expect(resolveConsoleMode(tmpl({ rcon: { protocol: "source" } }))).toBe("rcon");
  });

  it("is none when there is no console and no usable rcon", () => {
    expect(resolveConsoleMode(tmpl({ rcon: { protocol: "none" } }))).toBe("none");
    expect(resolveConsoleMode(tmpl({}))).toBe("none");
  });
});

describe("rconAvailable", () => {
  it("is true only for a non-none rcon protocol", () => {
    expect(rconAvailable(tmpl({ rcon: { protocol: "source" } }))).toBe(true);
    expect(rconAvailable(tmpl({ rcon: { protocol: "none" } }))).toBe(false);
    expect(rconAvailable(tmpl({}))).toBe(false);
    expect(rconAvailable(undefined)).toBe(false);
  });
});
