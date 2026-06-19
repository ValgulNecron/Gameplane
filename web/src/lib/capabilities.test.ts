import { describe, expect, it } from "vitest";
import {
  activeVersion,
  rconAvailable,
  resolveConsoleMode,
  resolveModVolume,
  serverHasMods,
} from "./capabilities";
import type { GameServer, GameTemplate } from "@/types";

function tmpl(spec: Partial<GameTemplate["spec"]>): GameTemplate {
  return {
    metadata: { name: "t" },
    spec: { displayName: "T", game: "g", version: "1", image: "img", ...spec },
  };
}

function gsWith(version?: string): GameServer {
  return { metadata: { name: "s" }, spec: { templateRef: { name: "t" }, version } };
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

const versions = [
  { id: "1.21.4-paper", displayName: "1.21.4 · Paper", loader: "paper", default: true },
  { id: "1.21.4-forge", displayName: "1.21.4 · Forge", loader: "forge" },
  { id: "1.21.4-vanilla", displayName: "1.21.4 · Vanilla", loader: "vanilla" },
];
const loaderMods = {
  loaders: { paper: { path: "plugins" }, forge: { path: "mods" } },
  install: { allowedHosts: ["cdn.modrinth.com"] },
};

describe("activeVersion", () => {
  it("is undefined without a catalog", () => {
    expect(activeVersion(tmpl({}), gsWith())).toBeUndefined();
  });
  it("uses the server's chosen version", () => {
    expect(activeVersion(tmpl({ versions }), gsWith("1.21.4-forge"))?.id).toBe("1.21.4-forge");
  });
  it("falls back to the default, then the first", () => {
    expect(activeVersion(tmpl({ versions }), gsWith())?.id).toBe("1.21.4-paper");
    const noDefault = versions.map((v) => ({ ...v, default: false }));
    expect(activeVersion(tmpl({ versions: noDefault }), gsWith())?.id).toBe("1.21.4-paper");
  });
});

describe("resolveModVolume", () => {
  it("is undefined when no mods block", () => {
    expect(resolveModVolume(tmpl({ versions }), gsWith("1.21.4-paper"))).toBeUndefined();
  });
  it("resolves the active loader's path for the per-loader model", () => {
    const t = tmpl({ versions, capabilities: { mods: loaderMods } });
    expect(resolveModVolume(t, gsWith("1.21.4-forge"))).toMatchObject({ path: "mods", loader: "forge", versionLabel: "1.21.4 · Forge" });
  });
  it("is undefined for a loader with no mapping (e.g. vanilla)", () => {
    const t = tmpl({ versions, capabilities: { mods: loaderMods } });
    expect(resolveModVolume(t, gsWith("1.21.4-vanilla"))).toBeUndefined();
  });
  it("returns the single path for the legacy model", () => {
    const t = tmpl({ capabilities: { mods: { path: "mods" } } });
    expect(resolveModVolume(t, gsWith())).toMatchObject({ path: "mods" });
  });
});

describe("serverHasMods", () => {
  it("tracks resolveModVolume", () => {
    const t = tmpl({ versions, capabilities: { mods: loaderMods } });
    expect(serverHasMods(t, gsWith("1.21.4-paper"))).toBe(true);
    expect(serverHasMods(t, gsWith("1.21.4-vanilla"))).toBe(false);
    expect(serverHasMods(tmpl({}), gsWith())).toBe(false);
  });
});
