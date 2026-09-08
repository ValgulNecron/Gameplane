import { useState } from "react";
import { Input } from "@heroui/react";
import type { ResourceRequirements } from "@/types";
import { isValidQuantity } from "@/lib/validation";
import { convertMem, formatCpuQuantity, formatMemQuantity, parseCpuQuantity, parseMemQuantity } from "@/lib/quantity";
import { Field } from "./Field";
import type { SectionProps } from "./types";

function getCpuDisplayValue(quantity: string): string {
  const parsed = parseCpuQuantity(quantity);
  if (!parsed) return quantity;
  const cores = parsed.unit === "cores" ? parsed.value : parsed.value / 1000;
  return String(cores);
}

function getMemoryDisplayValue(quantity: string): string {
  const parsed = parseMemQuantity(quantity);
  if (!parsed) return quantity;
  const gib = convertMem(parsed, "Gi");
  return String(gib.value);
}

export function ResourcesSection({ draft, onChange }: SectionProps) {
  const [cpuBuffer, setCpuBuffer] = useState<string | null>(null);
  const [memBuffer, setMemBuffer] = useState<string | null>(null);
  const res = draft.spec.resources ?? {};

  const setResources = (next: ResourceRequirements) => {
    const cleaned = pruneResources(next);
    onChange({
      ...draft,
      spec: { ...draft.spec, resources: cleaned },
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

  const handleCpuBlur = () => {
    if (cpuBuffer === null) return;
    const num = Number(cpuBuffer);
    if (!Number.isFinite(num)) {
      setCpuBuffer(null);
      return;
    }

    // Parse as cores, clamp to minimum 0.1 (100m), format back to canonical
    const cores = Math.max(0.1, num);
    const formatted = formatCpuQuantity({ value: cores, unit: "cores" });

    setCpuBuffer(null);
    setResources({
      ...res,
      requests: { ...res.requests, cpu: formatted },
      limits: { ...res.limits, cpu: formatted },
    });
  };

  const handleMemoryBlur = () => {
    if (memBuffer === null) return;
    const num = Number(memBuffer);
    if (!Number.isFinite(num)) {
      setMemBuffer(null);
      return;
    }

    // Parse as GiB, format back to canonical
    const formatted = formatMemQuantity({ value: num, unit: "Gi" });

    setMemBuffer(null);
    setResources({
      ...res,
      requests: { ...res.requests, memory: formatted },
      limits: { ...res.limits, memory: formatted },
    });
  };

  return (
    <div className="space-y-6">
      <Field
        label="CPU cores"
        hint="Sets requests=limits to the same value (Guaranteed QoS)."
      >
        <Input
          type="text"
          value={cpuBuffer ?? getCpuDisplayValue(res.limits?.cpu ?? res.requests?.cpu ?? "2")}
          onChange={(e) => setCpuBuffer(e.target.value)}
          onBlur={handleCpuBlur}
          placeholder="2"
          aria-label="CPU cores value"
        />
      </Field>

      <Field label="Memory (GiB)" hint="Sets requests=limits to the same value.">
        <Input
          type="text"
          value={memBuffer ?? getMemoryDisplayValue(res.limits?.memory ?? res.requests?.memory ?? "4Gi")}
          onChange={(e) => setMemBuffer(e.target.value)}
          onBlur={handleMemoryBlur}
          placeholder="4"
          aria-label="Memory (GiB) value"
        />
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
          type="text"
          value={storage.size ?? ""}
          onChange={(e) => setStorage({ ...storage, size: e.target.value || undefined })}
          placeholder="10Gi"
          className={sizeValid ? "" : "border-danger focus:border-danger focus:ring-danger"}
          aria-label="Storage size"
        />
        {!sizeValid && (
          <div className="pt-1 text-xs text-danger">Invalid quantity (e.g. &quot;10Gi&quot;).</div>
        )}
      </Field>

      <Field label="StorageClass" hint="Leave blank to use the cluster default.">
        <Input
          type="text"
          value={storage.storageClassName ?? ""}
          onChange={(e) =>
            setStorage({ ...storage, storageClassName: e.target.value || undefined })
          }
          placeholder="fast-ssd"
          aria-label="StorageClass"
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
