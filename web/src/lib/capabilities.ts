import type { GameServer, GameTemplate, GameVersion } from "@/types";

export type ConsoleMode = "rcon" | "pty" | "none";

// resolveConsoleMode mirrors operator.controller.EffectiveConsoleMode so
// the dashboard agrees with the server on which transport to open (and
// whether to show the Console tab at all). Keep in sync with the operator
// helper if its rules change.
export function resolveConsoleMode(tmpl: GameTemplate | undefined): ConsoleMode {
  if (!tmpl) return "rcon";
  if (tmpl.spec.consoleMode) return tmpl.spec.consoleMode;
  const proto = tmpl.spec.rcon?.protocol;
  if (proto && proto !== "none") return "rcon";
  return "none";
}

// rconAvailable reports whether the template exposes a live RCON
// connection — required for module actions and status metrics, which run
// over RCON.
export function rconAvailable(tmpl: GameTemplate | undefined): boolean {
  const proto = tmpl?.spec.rcon?.protocol;
  return !!proto && proto !== "none";
}

// activeVersion mirrors the operator's resolveVersion: the server's chosen
// version, else the template's default, else the first, else undefined when
// the template declares no version catalog.
export function activeVersion(
  tmpl: GameTemplate | undefined,
  gs: GameServer | undefined,
): GameVersion | undefined {
  const versions = tmpl?.spec.versions;
  if (!versions || versions.length === 0) return undefined;
  const chosen = gs?.spec.version;
  if (chosen) return versions.find((v) => v.id === chosen);
  return versions.find((v) => v.default) ?? versions[0];
}

export interface ActiveModVolume {
  path: string;
  displayName?: string;
  loader?: string;
  versionLabel?: string;
}

// resolveModVolume mirrors the operator's resolveCapabilities: it returns the
// mod directory this server actually manages, or undefined when the server has
// no mod manager — no mods block, or a per-loader mods map whose active loader
// has no entry (e.g. a vanilla Minecraft/Terraria server). For the legacy
// single-path model it returns that path.
export function resolveModVolume(
  tmpl: GameTemplate | undefined,
  gs: GameServer | undefined,
): ActiveModVolume | undefined {
  const mods = tmpl?.spec.capabilities?.mods;
  if (!mods) return undefined;
  const loaders = mods.loaders;
  const ver = activeVersion(tmpl, gs);
  if (loaders && Object.keys(loaders).length > 0) {
    const entry = ver?.loader ? loaders[ver.loader] : undefined;
    if (!entry) return undefined;
    return { path: entry.path, displayName: entry.displayName, loader: ver?.loader, versionLabel: ver?.displayName };
  }
  if (mods.path) {
    return { path: mods.path, loader: ver?.loader, versionLabel: ver?.displayName };
  }
  return undefined;
}

// serverHasMods reports whether this server should show the Mods tab. An
// idList template (see ModIDListDecl) has no mods directory at all — it
// installs mods by id instead — so resolveModVolume returns undefined for
// it; the tab still shows, rendering the id-managed editor instead of the
// file list (see ModsTab).
export function serverHasMods(
  tmpl: GameTemplate | undefined,
  gs: GameServer | undefined,
): boolean {
  if (tmpl?.spec.capabilities?.mods?.idList) return true;
  return resolveModVolume(tmpl, gs) !== undefined;
}

// Modrinth/CurseForge modpacks bring their own mod loader (Fabric/Forge/…),
// so they only apply to true mod loaders — never vanilla or the Bukkit
// plugin family (paper/spigot/bukkit/purpur). Keep in sync with the loaders
// that map to a mods/ (not plugins/) directory in module templates.
const MODPACK_LOADERS = new Set(["fabric", "forge", "neoforge", "quilt"]);

// serverHasModpacks reports whether this server should show the Modpacks
// tab: the template declares a registry provider that offers modpacks AND —
// for the per-loader model — the active version's loader can actually run
// one. A modpack replaces the server type (e.g. itzg TYPE=MODRINTH), so it's
// meaningless for vanilla or plugin-only loaders like Paper.
export function serverHasModpacks(
  tmpl: GameTemplate | undefined,
  gs: GameServer | undefined,
): boolean {
  const providers = tmpl?.spec.capabilities?.mods?.registry?.providers ?? [];
  if (!providers.some((p) => p.modpacks !== undefined)) return false;
  const loaders = tmpl?.spec.capabilities?.mods?.loaders;
  // Per-loader model (e.g. Minecraft): gate on a modpack-capable loader.
  if (loaders && Object.keys(loaders).length > 0) {
    const loader = activeVersion(tmpl, gs)?.loader;
    return loader !== undefined && MODPACK_LOADERS.has(loader);
  }
  // No per-loader model → keep the prior template-level behavior.
  return true;
}
