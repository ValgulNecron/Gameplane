import type { ReactNode } from "react";

export function FieldLabel({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block space-y-1.5">
      <span className="text-xs font-medium text-fg">{label}</span>
      {children}
    </label>
  );
}
