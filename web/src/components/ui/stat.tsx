import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export function StatCard({
  label,
  value,
  sub,
  icon,
  accent,
  className,
}: {
  label: string;
  value: ReactNode;
  sub?: ReactNode;
  icon?: ReactNode;
  accent?: "primary" | "success" | "warning" | "danger" | "violet";
  className?: string;
}) {
  const accentClass = {
    primary: "text-primary",
    success: "text-success",
    warning: "text-warning",
    danger: "text-danger",
    violet: "text-violet",
  }[accent ?? "primary"];
  return (
    <div className={cn("rounded-lg border border-border bg-card p-4", className)}>
      <div className="flex items-center justify-between text-xs uppercase tracking-wide text-muted">
        <span>{label}</span>
        {icon && <span className={accentClass}>{icon}</span>}
      </div>
      <div className="pt-2 font-mono text-2xl text-fg">{value}</div>
      {sub && <div className="pt-1 text-xs text-muted">{sub}</div>}
    </div>
  );
}
