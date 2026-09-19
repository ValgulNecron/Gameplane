import { cn } from "@/lib/utils";

// Legacy per-game palette, kept as a fallback for templates that don't
// declare an accentColor yet. New modules carry spec.accentColor, so the
// dashboard tints from data instead of this table — see GameIcon below.
const legacyPalette: Record<string, string> = {
  "minecraft-java":    "bg-success/20 text-success",
  "minecraft-bedrock": "bg-success/20 text-success",
  "minecraft-modded":  "bg-success/20 text-success",
  "valheim":           "bg-warning/20 text-warning",
  "factorio":          "bg-primary/20 text-primary",
  "palworld":          "bg-violet/20 text-violet",
  "ark":               "bg-danger/20 text-danger",
  "terraria":          "bg-success/20 text-success",
  "counter-strike-2":  "bg-warning/20 text-warning",
  "rust":              "bg-danger/20 text-danger",
  "7-days-to-die":     "bg-muted/30 text-muted",
  "satisfactory":      "bg-primary/20 text-primary",
};

// hex6 matches a "#rrggbb" color so we only ever inline a value we trust.
const hex6 = /^#[0-9a-fA-F]{6}$/;

// iconSrc matches the two shapes GameTemplate.spec.icon (Q7) is allowed to
// carry — an http(s) URL or a data: URI. Anything else is treated as unset
// so a malformed value can't be injected as an <img src>.
const iconSrc = /^(https?:|data:)/;

// Small colored tile per game. When the template declares an icon (Q7),
// that image is rendered directly. Otherwise the tile shows `code` — a
// unique per-install abbreviation computed by assignGameCodes/
// assignGameCodesForTemplates (lib/gameIcon.ts) from the full template
// list — falling back to the legacy first-two-letters-of-name behavior
// when the caller doesn't have template data to compute one (unchanged
// from before, so existing callers are unaffected). The tile is tinted
// from accentColor when the template declares one; otherwise it falls
// back to the legacy palette keyed off the game/template name.
export function GameIcon({
  game,
  icon,
  code,
  accentColor,
  size = "md",
}: {
  game?: string;
  /** GameTemplate.spec.icon — rendered in place of the letter code when present and well-formed. */
  icon?: string;
  /** Pre-computed abbreviation from lib/gameIcon.ts. Falls back to the legacy first-two-letters behavior when omitted. */
  code?: string;
  accentColor?: string;
  size?: "sm" | "md" | "lg";
}) {
  const g = (game ?? "??").toLowerCase();
  const dims = { sm: "h-7 w-7 text-xs", md: "h-9 w-9 text-sm", lg: "h-12 w-12 text-base" }[size];
  const base = "flex shrink-0 items-center justify-center overflow-hidden rounded-md font-mono uppercase";
  const tint = accentColor && hex6.test(accentColor)
    ? { backgroundColor: `${accentColor}33`, color: accentColor }
    : undefined;
  const color = tint ? undefined : (legacyPalette[g] ?? "bg-muted/30 text-muted");

  if (icon && iconSrc.test(icon)) {
    return (
      <div className={cn(base, color, dims)} style={tint}>
        <img src={icon} alt="" className="h-full w-full object-cover" />
      </div>
    );
  }

  return (
    <div className={cn(base, color, dims)} style={tint}>
      {code ?? g.slice(0, 2)}
    </div>
  );
}
