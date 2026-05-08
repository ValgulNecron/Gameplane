import { Input } from "@/components/ui/input";
import { Field } from "./Field";
import type { SectionProps } from "./types";

export function AccessSection({ draft }: SectionProps) {
  const sa = draft.spec.serviceAccountName ?? "";
  return (
    <div className="space-y-6">
      <Field
        label="Service account"
        hint="Pod runs as this ServiceAccount. Editing is locked while we finalize RBAC scoping."
      >
        <Input value={sa || "default"} disabled />
      </Field>

      <Field label="Access" hint="Managed in Admin → Users.">
        <div className="rounded border border-dashed border-border bg-surface/20 px-3 py-2 text-xs text-muted">
          Per-server role bindings will appear here once the Users tab supports them.
        </div>
      </Field>
    </div>
  );
}
