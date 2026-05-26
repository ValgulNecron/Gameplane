import { cn } from "@/lib/utils";

// Phase strings span three CRDs: GameServer (Pending/Starting/Running/
// Stopping/Stopped/Suspended/Failed), Backup (Pending/Running/Succeeded/
// Failed), and Restore (Pending/Suspending/Running/Resuming/Succeeded/
// Failed). One component covers all three; unknown phases fall back to
// muted so a new operator-side phase doesn't render styleless.
const phaseClass: Record<string, string> = {
  Pending:    "bg-muted/20 text-muted",
  Starting:   "bg-warning/20 text-warning",
  Running:    "bg-success/20 text-success",
  Stopping:   "bg-warning/20 text-warning",
  Stopped:    "bg-muted/20 text-muted",
  Suspended:  "bg-muted/20 text-muted",
  Suspending: "bg-warning/20 text-warning",
  Resuming:   "bg-warning/20 text-warning",
  Succeeded:  "bg-success/20 text-success",
  Failed:     "bg-danger/20 text-danger",
};

export function PhaseBadge({ phase }: { phase?: string }) {
  const p = phase ?? "Pending";
  return (
    <span
      className={cn(
        "inline-flex h-5 items-center rounded px-2 text-xs font-mono",
        phaseClass[p] ?? "bg-muted/20 text-muted",
      )}
    >
      {p}
    </span>
  );
}
