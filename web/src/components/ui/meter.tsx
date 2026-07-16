// Meter renders a labelled horizontal usage bar (CPU, memory, storage, …).
// Extracted from the cluster node cards so the Cluster page and the
// Dashboard overview share one bar. `pct` is clamped to 0–100 for the fill.

import { cn } from "@/lib/utils";

const accentClass = {
  primary: "bg-primary",
  violet: "bg-violet",
  success: "bg-success",
  warning: "bg-warning",
  danger: "bg-danger",
} as const;

export type MeterAccent = keyof typeof accentClass;

export function Meter({
  label,
  pct,
  sub,
  accent = "primary",
  unknown = false,
}: {
  label: string;
  pct: number;
  sub?: string;
  accent?: MeterAccent;
  // unknown renders "—" and an empty track instead of a percentage, for
  // when there's no measurement to show (e.g. no metrics-server) — a
  // meter reading 0% otherwise reads as "measured idle", not "unmeasured".
  unknown?: boolean;
}) {
  return (
    <div>
      <div className="flex items-center justify-between text-[11px]">
        <span className="text-muted">{label}</span>
        <span className={cn("font-mono", unknown ? "text-muted" : "text-fg")}>
          {unknown ? "—" : `${Math.round(pct)}%`}
        </span>
      </div>
      <div className="mt-1 h-1.5 overflow-hidden rounded-full bg-surface">
        {!unknown && (
          <div
            className={`h-full ${accentClass[accent]}`}
            style={{ width: `${Math.min(100, Math.max(0, pct))}%` }}
          />
        )}
      </div>
      {sub && <div className="pt-1 text-[10px] text-muted">{sub}</div>}
    </div>
  );
}
