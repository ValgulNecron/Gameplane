import { Input, Button } from "@heroui/react";
import { X } from "lucide-react";
import type { PortOverride } from "@/types";

// T083: Row editor for GameServer.spec.networking.portOverrides — shared between
// the server Settings → Networking tab and the Create Server wizard.
export function PortOverridesEditor({
  values,
  onChange,
}: {
  values: PortOverride[];
  onChange: (v: PortOverride[]) => void;
}) {
  const update = (idx: number, patch: Partial<PortOverride>) => {
    onChange(values.map((p, i) => (i === idx ? { ...p, ...patch } : p)));
  };
  const remove = (idx: number) => {
    onChange(values.filter((_, i) => i !== idx));
  };
  const add = () => {
    onChange([...values, { name: "" }]);
  };
  return (
    <div className="space-y-2">
      {values.length > 0 && (
        <div className="hidden gap-2 text-xs text-muted sm:grid sm:grid-cols-[1fr_120px_120px_32px] sm:items-center">
          <span>Port name</span>
          <span>Service port</span>
          <span>NodePort</span>
          <span />
        </div>
      )}
      {values.map((p, idx) => (
        <div
          key={idx}
          className="grid grid-cols-1 gap-2 rounded border border-border bg-surface/30 p-2 sm:grid-cols-[1fr_120px_120px_32px] sm:items-center sm:border-0 sm:bg-transparent sm:p-0"
        >
          <Input
            value={p.name}
            onChange={(e) => update(idx, { name: e.target.value })}
            placeholder="game"
          />
          <Input
            value={p.servicePort?.toString() ?? ""}
            onChange={(e) =>
              update(idx, {
                servicePort: e.target.value ? Number(e.target.value) : undefined,
              })
            }
            inputMode="numeric"
            placeholder="—"
          />
          <Input
            value={p.nodePort?.toString() ?? ""}
            onChange={(e) =>
              update(idx, {
                nodePort: e.target.value ? Number(e.target.value) : undefined,
              })
            }
            inputMode="numeric"
            placeholder="30000-32767"
          />
          <Button
            variant="ghost"
            size="sm"
            isIconOnly
            className="justify-self-start sm:justify-self-auto"
            aria-label="Remove"
            onPress={() => remove(idx)}
          >
            <span title="Remove">
              <X className="h-4 w-4" />
            </span>
          </Button>
        </div>
      ))}
      <Button size="sm" variant="secondary" onPress={add}>
        Add override
      </Button>
    </div>
  );
}
