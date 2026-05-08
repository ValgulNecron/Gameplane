import { cn } from "@/lib/utils";

// Phase strings come from two CRDs: Backup (Pending/Running/Succeeded/Failed)
// and Restore (Pending/Suspending/Running/Resuming/Succeeded/Failed). One
// component covers both since the colour mapping is the same.
const phaseClass: Record<string, string> = {
  Pending:    "bg-muted/20 text-muted",
  Suspending: "bg-warning/20 text-warning",
  Running:    "bg-primary/20 text-primary",
  Resuming:   "bg-warning/20 text-warning",
  Succeeded:  "bg-success/20 text-success",
  Failed:     "bg-danger/20 text-danger",
};

export function PhaseBadge({ phase }: { phase?: string }) {
  const p = phase ?? "Pending";
  return (
    <span
      className={cn(
        "inline-flex h-5 items-center rounded-full px-2 text-[11px] font-mono",
        phaseClass[p] ?? "bg-muted/20 text-muted",
      )}
    >
      {p}
    </span>
  );
}
