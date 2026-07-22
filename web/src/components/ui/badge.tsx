import { cn } from "@/lib/utils";

// Generic badge for counts and simple labels.
export function Badge({ children, variant = "default", className }: { children: React.ReactNode; variant?: "default" | "primary"; className?: string }) {
  const variantClass = {
    default: "bg-muted/20 text-muted",
    primary: "bg-primary/20 text-primary",
  };
  return (
    <span
      className={cn(
        "inline-flex h-5 items-center rounded px-2 text-xs font-mono",
        variantClass[variant],
        className,
      )}
    >
      {children}
    </span>
  );
}

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

export function PhaseBadge({ phase, asleep }: { phase?: string; asleep?: boolean }) {
  // Asleep is not a real CRD phase (the CRD says Suspended); it's a derived
  // display state layered on top by checking status.idle.asleep.
  const p = phase ?? "Pending";
  const label = asleep ? "Asleep" : p;
  const tone = asleep ? "bg-violet/20 text-violet" : (phaseClass[p] ?? "bg-muted/20 text-muted");
  return (
    <span className={cn("inline-flex h-5 items-center rounded px-2 text-xs font-mono", tone)}>
      {label}
    </span>
  );
}
