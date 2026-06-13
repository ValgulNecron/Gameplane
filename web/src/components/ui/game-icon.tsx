import { cn } from "@/lib/utils";

// Legacy per-game palette, kept as a fallback for templates that don't
// declare an accentColor yet. New modules carry spec.accentColor, so the
// dashboard tints from data instead of this table — see GameIcon below.
const legacyPalette: Record<string, string> = {
  "minecraft-java":    "bg-success/20 text-success",
  "minecraft-bedrock": "bg-success/20 text-success",
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

// Small colored tile per game. When the template declares an
// accentColor, the tile is tinted from that value (background at ~20%
// alpha, text at full); otherwise it falls back to the legacy palette
// keyed off the game/template name.
export function GameIcon({
  game,
  accentColor,
  size = "md",
}: {
  game?: string;
  accentColor?: string;
  size?: "sm" | "md" | "lg";
}) {
  const g = (game ?? "?").toLowerCase();
  const dims = { sm: "h-7 w-7 text-xs", md: "h-9 w-9 text-sm", lg: "h-12 w-12 text-base" }[size];
  const base = "flex shrink-0 items-center justify-center rounded-md font-mono uppercase";

  if (accentColor && hex6.test(accentColor)) {
    return (
      <div
        className={cn(base, dims)}
        style={{ backgroundColor: `${accentColor}33`, color: accentColor }}
      >
        {g.slice(0, 2)}
      </div>
    );
  }
  const color = legacyPalette[g] ?? "bg-muted/30 text-muted";
  return <div className={cn(base, color, dims)}>{g.slice(0, 2)}</div>;
}
