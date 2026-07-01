import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { X } from "lucide-react";
import type { PortOverride } from "@/types";

// Row editor for GameServer.spec.networking.portOverrides — shared between
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
        <div className="grid grid-cols-[1fr_120px_120px_32px] items-center gap-2 text-xs text-muted">
          <span>Port name</span>
          <span>Service port</span>
          <span>NodePort</span>
          <span />
        </div>
      )}
      {values.map((p, idx) => (
        <div key={idx} className="grid grid-cols-[1fr_120px_120px_32px] items-center gap-2">
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
            size="icon"
            className="h-8 w-8"
            title="Remove"
            onClick={() => remove(idx)}
          >
            <X className="h-3 w-3" />
          </Button>
        </div>
      ))}
      <Button size="sm" variant="outline" onClick={add}>
        Add override
      </Button>
    </div>
  );
}
