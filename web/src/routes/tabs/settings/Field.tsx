import type { ReactNode } from "react";

export function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="grid grid-cols-1 items-start gap-1.5 sm:grid-cols-[200px_1fr] sm:gap-4">
      <div className="sm:pt-2">
        <div className="text-sm text-fg">{label}</div>
        {hint && <div className="pt-1 text-xs text-muted">{hint}</div>}
      </div>
      <div>{children}</div>
    </div>
  );
}
