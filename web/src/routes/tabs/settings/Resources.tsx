import { Input } from "@/components/ui/input";
import { ResourceInput } from "@/components/ui/resource-input";
import type { ResourceRequirements } from "@/types";
import { isValidQuantity } from "@/lib/validation";
import { Field } from "./Field";
import type { SectionProps } from "./types";

export function ResourcesSection({ draft, onChange }: SectionProps) {
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

  return (
    <div className="space-y-6">
      <Field
        label="CPU cores"
        hint="Sets requests=limits to the same value (Guaranteed QoS)."
      >
        <ResourceInput
          kind="cpu"
          value={res.limits?.cpu ?? res.requests?.cpu ?? "2"}
          onChange={(q) =>
            setResources({
              ...res,
              requests: { ...res.requests, cpu: q },
              limits: { ...res.limits, cpu: q },
            })
          }
        />
      </Field>

      <Field label="Memory (GiB)" hint="Sets requests=limits to the same value.">
        <ResourceInput
          kind="memory"
          value={res.limits?.memory ?? res.requests?.memory ?? "4Gi"}
          onChange={(q) =>
            setResources({
              ...res,
              requests: { ...res.requests, memory: q },
              limits: { ...res.limits, memory: q },
            })
          }
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
