import { cn } from "@/lib/utils";
import type { GameServerPhase } from "@/types";

const phaseClass: Record<GameServerPhase, string> = {
  Pending:   "bg-muted/20 text-muted",
  Starting:  "bg-warning/20 text-warning",
  Running:   "bg-success/20 text-success",
  Stopping:  "bg-warning/20 text-warning",
  Stopped:   "bg-muted/20 text-muted",
  Suspended: "bg-muted/20 text-muted",
  Failed:    "bg-danger/20 text-danger",
};

export function PhaseBadge({ phase }: { phase?: GameServerPhase }) {
  const p = phase ?? "Pending";
  return (
    <span className={cn("inline-flex h-5 items-center rounded px-2 text-xs font-mono", phaseClass[p])}>
      {p}
    </span>
  );
}
