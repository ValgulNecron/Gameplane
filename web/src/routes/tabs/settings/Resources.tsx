import { Input } from "@/components/ui/input";
import { Slider } from "@/components/ui/slider";
import type { ResourceRequirements } from "@/types";
import { isValidQuantity } from "@/lib/validation";
import { Field } from "./Field";
import type { SectionProps } from "./types";

const CPU_MIN = 0.5;
const CPU_MAX = 16;
const CPU_STEP = 0.5;
const MEM_MIN = 0.5;
const MEM_MAX = 64;
const MEM_STEP = 0.5;

// CPU cores → quantity string. ".5" → "500m", whole → "<n>".
function cpuToQty(cores: number): string {
  return Number.isInteger(cores) ? String(cores) : `${Math.round(cores * 1000)}m`;
}

function qtyToCpu(qty?: string): number | null {
  if (!qty) return null;
  if (qty.endsWith("m")) {
    const n = Number(qty.slice(0, -1));
    return Number.isFinite(n) ? n / 1000 : null;
  }
  const n = Number(qty);
  return Number.isFinite(n) ? n : null;
}

function memToQty(gib: number): string {
  return Number.isInteger(gib) ? `${gib}Gi` : `${Math.round(gib * 1024)}Mi`;
}

function qtyToMem(qty?: string): number | null {
  if (!qty) return null;
  if (qty.endsWith("Gi")) {
    const n = Number(qty.slice(0, -2));
    return Number.isFinite(n) ? n : null;
  }
  if (qty.endsWith("Mi")) {
    const n = Number(qty.slice(0, -2));
    return Number.isFinite(n) ? n / 1024 : null;
  }
  return null;
}

export function ResourcesSection({ draft, onChange }: SectionProps) {
  const res = draft.spec.resources ?? {};
  const cpu = qtyToCpu(res.limits?.cpu) ?? qtyToCpu(res.requests?.cpu) ?? 2;
  const mem = qtyToMem(res.limits?.memory) ?? qtyToMem(res.requests?.memory) ?? 4;

  const setResources = (next: ResourceRequirements) => {
    const cleaned = pruneResources(next);
    onChange({
      ...draft,
      spec: { ...draft.spec, resources: cleaned },
    });
  };

  const setCpu = (cores: number) => {
    const q = cpuToQty(cores);
    setResources({
      ...res,
      requests: { ...res.requests, cpu: q },
      limits: { ...res.limits, cpu: q },
    });
  };

  const setMem = (gib: number) => {
    const q = memToQty(gib);
    setResources({
      ...res,
      requests: { ...res.requests, memory: q },
      limits: { ...res.limits, memory: q },
    });
  };

  const storage = draft.spec.storage ?? {};

  const setStorage = (next: typeof storage) => {
    const cleaned: typeof storage = {};
    if (next.size) cleaned.size = next.size;
    if (next.storageClassName) cleaned.storageClassName = next.storageClassName;
    if (next.mountPath) cleaned.mountPath = next.mountPath;
    onChange({
      ...draft,
      spec: {
        ...draft.spec,
        storage: Object.keys(cleaned).length ? cleaned : undefined,
      },
    });
  };

  const sizeValid = !storage.size || isValidQuantity(storage.size);

  return (
    <div className="space-y-6">
      <Field
        label="CPU cores"
        hint="Sets requests=limits to the same value (Guaranteed QoS)."
      >
        <div className="flex items-center gap-3">
          <Slider
            value={cpu}
            min={CPU_MIN}
            max={CPU_MAX}
            step={CPU_STEP}
            onValueChange={setCpu}
            aria-label="CPU cores"
          />
          <div className="w-16 text-right font-mono text-sm">{cpu}</div>
        </div>
      </Field>

      <Field label="Memory (GiB)" hint="Sets requests=limits to the same value.">
        <div className="flex items-center gap-3">
          <Slider
            value={mem}
            min={MEM_MIN}
            max={MEM_MAX}
            step={MEM_STEP}
            onValueChange={setMem}
            aria-label="Memory"
          />
          <div className="w-16 text-right font-mono text-sm">{mem} Gi</div>
        </div>
      </Field>

      <Field
        label="Storage size"
        hint={
          <>
            <div>K8s quantity (e.g. 10Gi, 200Gi).</div>
            <div className="pt-1">
              Resizing requires a StorageClass with{" "}
              <span className="font-mono">allowVolumeExpansion: true</span>.
            </div>
          </>
        }
      >
        <Input
          value={storage.size ?? ""}
          onChange={(e) => setStorage({ ...storage, size: e.target.value || undefined })}
          placeholder="10Gi"
          className={sizeValid ? "" : "border-danger focus:border-danger focus:ring-danger"}
        />
        {!sizeValid && (
          <div className="pt-1 text-xs text-danger">Invalid quantity (e.g. &quot;10Gi&quot;).</div>
        )}
      </Field>

      <Field label="StorageClass" hint="Leave blank to use the cluster default.">
        <Input
          value={storage.storageClassName ?? ""}
          onChange={(e) =>
            setStorage({ ...storage, storageClassName: e.target.value || undefined })
          }
          placeholder="fast-ssd"
        />
      </Field>
    </div>
  );
}

function pruneResources(r: ResourceRequirements): ResourceRequirements | undefined {
  const requests = pruneRecord(r.requests);
  const limits = pruneRecord(r.limits);
  if (!requests && !limits) return undefined;
  const out: ResourceRequirements = {};
  if (requests) out.requests = requests;
  if (limits) out.limits = limits;
  return out;
}

function pruneRecord(r: Partial<Record<"cpu" | "memory", string>> | undefined) {
  if (!r) return undefined;
  const out: Partial<Record<"cpu" | "memory", string>> = {};
  if (r.cpu) out.cpu = r.cpu;
  if (r.memory) out.memory = r.memory;
  return Object.keys(out).length ? out : undefined;
}
