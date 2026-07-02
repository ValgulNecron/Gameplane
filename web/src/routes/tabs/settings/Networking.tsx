import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { X } from "lucide-react";
import { useState } from "react";
import type { Expose, GameServerNetworking } from "@/types";
import { PortOverridesEditor } from "@/components/server/PortOverridesEditor";
import { Field } from "./Field";
import type { SectionProps } from "./types";

const EXPOSE_OPTIONS: { value: Expose; label: string }[] = [
  { value: "ClusterIP", label: "ClusterIP (in-cluster only)" },
  { value: "NodePort", label: "NodePort" },
  { value: "LoadBalancer", label: "LoadBalancer" },
  { value: "Hostport", label: "Hostport" },
];

export function NetworkingSection({ draft, onChange }: SectionProps) {
  const net = draft.spec.networking ?? {};

  const setNet = (next: GameServerNetworking) => {
    const cleaned: GameServerNetworking = {};
    if (next.expose) cleaned.expose = next.expose;
    if (next.hostname) cleaned.hostname = next.hostname;
    if (next.serviceAnnotations && Object.keys(next.serviceAnnotations).length) {
      cleaned.serviceAnnotations = next.serviceAnnotations;
    }
    if (next.portOverrides && next.portOverrides.length) {
      cleaned.portOverrides = next.portOverrides;
    }
    if (next.sourceRanges && next.sourceRanges.length) {
      cleaned.sourceRanges = next.sourceRanges;
    }
    onChange({
      ...draft,
      spec: {
        ...draft.spec,
        networking: Object.keys(cleaned).length ? cleaned : undefined,
      },
    });
  };

  return (
    <div className="space-y-6">
      <Field label="Expose" hint="Service type fronting the game pod.">
        <Select
          value={net.expose ?? "ClusterIP"}
          options={EXPOSE_OPTIONS}
          onValueChange={(v) => setNet({ ...net, expose: v as Expose })}
        />
      </Field>

      <Field label="Hostname" hint="Optional DNS name for ingress / external-dns annotations.">
        <Input
          value={net.hostname ?? ""}
          onChange={(e) => setNet({ ...net, hostname: e.target.value || undefined })}
          placeholder="mc.example.com"
          spellCheck={false}
        />
      </Field>

      <Field
        label="Service annotations"
        hint="Merged into the Service. Useful for cloud LB or external-dns config."
      >
        <KVEditor
          values={net.serviceAnnotations ?? {}}
          onChange={(v) => setNet({ ...net, serviceAnnotations: v })}
          keyPlaceholder="service.beta.kubernetes.io/…"
          valuePlaceholder="value"
        />
      </Field>

      <Field
        label="Port overrides"
        hint="Pin a NodePort or override the Service port for a named template port."
      >
        <PortOverridesEditor
          values={net.portOverrides ?? []}
          onChange={(v) => setNet({ ...net, portOverrides: v })}
        />
      </Field>

      {net.expose === "LoadBalancer" && (
        <Field
          label="LoadBalancer IP allow-list"
          hint="One CIDR per line. Restricts which clients reach the LoadBalancer; empty allows all."
        >
          <textarea
            className="block w-full rounded-md border border-border bg-surface px-3 py-2 font-mono text-sm"
            rows={3}
            value={(net.sourceRanges ?? []).join("\n")}
            onChange={(e) =>
              setNet({
                ...net,
                sourceRanges: e.target.value
                  .split(/[\n,]/)
                  .map((s) => s.trim())
                  .filter(Boolean),
              })
            }
            placeholder={"203.0.113.0/24\n10.0.0.0/8"}
            aria-label="LoadBalancer IP allow-list"
          />
        </Field>
      )}
    </div>
  );
}

function KVEditor({
  values,
  onChange,
  keyPlaceholder,
  valuePlaceholder,
}: {
  values: Record<string, string>;
  onChange: (v: Record<string, string>) => void;
  keyPlaceholder: string;
  valuePlaceholder: string;
}) {
  const [draftKV, setDraftKV] = useState({ key: "", value: "" });
  const add = () => {
    const key = draftKV.key.trim();
    if (!key) return;
    onChange({ ...values, [key]: draftKV.value });
    setDraftKV({ key: "", value: "" });
  };
  return (
    <div className="space-y-2">
      {Object.entries(values).map(([k, v]) => (
        <div
          key={k}
          className="flex items-center gap-2 rounded border border-border bg-surface/40 px-2 py-1"
        >
          <span className="flex-1 truncate font-mono text-xs">{k}</span>
          <span className="text-muted">=</span>
          <span className="flex-1 truncate font-mono text-xs">{v}</span>
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6"
            title="Remove"
            onClick={() => {
              const next = { ...values };
              delete next[k];
              onChange(next);
            }}
          >
            <X className="h-3 w-3" />
          </Button>
        </div>
      ))}
      <div className="flex items-center gap-2">
        <Input
          value={draftKV.key}
          onChange={(e) => setDraftKV({ ...draftKV, key: e.target.value })}
          placeholder={keyPlaceholder}
          className="flex-1"
        />
        <Input
          value={draftKV.value}
          onChange={(e) => setDraftKV({ ...draftKV, value: e.target.value })}
          placeholder={valuePlaceholder}
          className="flex-1"
        />
        <Button size="sm" variant="outline" onClick={add} disabled={!draftKV.key.trim()}>
          Add
        </Button>
      </div>
    </div>
  );
}

