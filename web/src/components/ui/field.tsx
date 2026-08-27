import type { ReactNode } from "react";

// `hint` is deliberately rendered OUTSIDE the <label> rather than as a
// trailing child inside it. A wrapping <label> exposes the concatenated
// text of every non-form-control descendant as its accessible name (see
// testing-library's getLabelContent/getTextContent) — a hint sentence
// inside the label makes the field's accessible name "Name<hint text>"
// instead of "Name", so `getByLabelText("Name")` (and a screen reader's
// exact-name interaction) can never match it. Keeping the field's own
// label+control pairing on a bare <label> and moving descriptive copy to
// a sibling preserves the same layout while keeping the accessible name
// exactly `label`.
export function FieldLabel({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="space-y-1.5">
      <label className="block space-y-1.5">
        <span className="text-xs font-medium text-fg">{label}</span>
        {children}
      </label>
      {hint && <span className="block text-[11px] text-muted">{hint}</span>}
    </div>
  );
}
