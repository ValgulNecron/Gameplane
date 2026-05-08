import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { X } from "lucide-react";
import { useState } from "react";
import { DESCRIPTION_ANNOTATION, type SectionProps } from "./types";
import { Field } from "./Field";

export function GeneralSection({ draft, onChange }: SectionProps) {
  const annotations = draft.metadata.annotations ?? {};
  const labels = draft.metadata.labels ?? {};
  const description = annotations[DESCRIPTION_ANNOTATION] ?? "";
  const [labelDraft, setLabelDraft] = useState({ key: "", value: "" });

  const setAnnotation = (key: string, value: string | undefined) => {
    const next = { ...annotations };
    if (value && value.length > 0) next[key] = value;
    else delete next[key];
    onChange({
      ...draft,
      metadata: {
        ...draft.metadata,
        annotations: Object.keys(next).length ? next : undefined,
      },
    });
  };

  const setLabels = (next: Record<string, string>) => {
    onChange({
      ...draft,
      metadata: {
        ...draft.metadata,
        labels: Object.keys(next).length ? next : undefined,
      },
    });
  };

  const addLabel = () => {
    const key = labelDraft.key.trim();
    const value = labelDraft.value.trim();
    if (!key) return;
    setLabels({ ...labels, [key]: value });
    setLabelDraft({ key: "", value: "" });
  };

  return (
    <div className="space-y-6">
      <Field label="Name" hint="Pinned to the GameServer UID; cannot change.">
        <Input value={draft.metadata.name} disabled />
      </Field>

      <Field label="Template">
        <Input value={draft.spec.templateRef.name} disabled />
      </Field>

      <Field label="Description" hint="Shown in the dashboard server list.">
        <Textarea
          value={description}
          onChange={(e) => setAnnotation(DESCRIPTION_ANNOTATION, e.target.value)}
          maxLength={1024}
          placeholder="Long-standing survival realm. Invite-only."
        />
      </Field>

      <Field label="Image override" hint="Pin a specific game-container image. Leave blank to use the template default.">
        <Input
          value={draft.spec.image ?? ""}
          onChange={(e) =>
            onChange({
              ...draft,
              spec: { ...draft.spec, image: e.target.value || undefined },
            })
          }
          placeholder="itzg/minecraft-server:2025.1.0"
          spellCheck={false}
        />
      </Field>

      <Field label="Labels" hint="Free-form Kubernetes labels.">
        <div className="space-y-2">
          {Object.entries(labels).map(([k, v]) => (
            <div key={k} className="flex items-center gap-2 rounded border border-border bg-surface/40 px-2 py-1">
              <span className="font-mono text-xs text-muted">{k}</span>
              <span className="text-muted">=</span>
              <span className="font-mono text-xs text-fg">{v}</span>
              <Button
                variant="ghost"
                size="icon"
                className="ml-auto h-6 w-6"
                title="Remove label"
                onClick={() => {
                  const next = { ...labels };
                  delete next[k];
                  setLabels(next);
                }}
              >
                <X className="h-3 w-3" />
              </Button>
            </div>
          ))}
          <div className="flex items-center gap-2">
            <Input
              value={labelDraft.key}
              onChange={(e) => setLabelDraft({ ...labelDraft, key: e.target.value })}
              placeholder="key"
              className="flex-1"
            />
            <Input
              value={labelDraft.value}
              onChange={(e) => setLabelDraft({ ...labelDraft, value: e.target.value })}
              placeholder="value"
              className="flex-1"
            />
            <Button size="sm" variant="outline" onClick={addLabel} disabled={!labelDraft.key.trim()}>
              Add
            </Button>
          </div>
        </div>
      </Field>
    </div>
  );
}

