import { cn } from "@/lib/utils";

// Small, deterministic colored tile per game — stand-in for real template
// icons until templates carry an image URL the UI can render.
export function GameIcon({ game, size = "md" }: { game?: string; size?: "sm" | "md" | "lg" }) {
  const g = (game ?? "?").toLowerCase();
  const palette: Record<string, string> = {
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
  const color = palette[g] ?? "bg-muted/30 text-muted";
  const dims = { sm: "h-7 w-7 text-xs", md: "h-9 w-9 text-sm", lg: "h-12 w-12 text-base" }[size];
  return (
    <div className={cn("flex shrink-0 items-center justify-center rounded-md font-mono uppercase", color, dims)}>
      {g.slice(0, 2)}
    </div>
  );
}
