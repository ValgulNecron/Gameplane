import { Switch } from "@/components/ui/switch";
import { Input } from "@/components/ui/input";
import { Field } from "./Field";
import { GRACE_PERIOD_ANNOTATION, type SectionProps } from "./types";

export function LifecycleSection({ draft, onChange, template }: SectionProps) {
  const annotations = draft.metadata.annotations ?? {};
  const autoRestart = !(draft.spec.suspend ?? false);
  const grace = annotations[GRACE_PERIOD_ANNOTATION] ?? "";
  const probesAvailable = !!template?.spec.rcon || !!template?.spec.consoleMode;

  const setSuspend = (suspend: boolean) => {
    onChange({
      ...draft,
      spec: { ...draft.spec, suspend },
    });
  };

  const setGrace = (value: string) => {
    const next = { ...annotations };
    if (value) next[GRACE_PERIOD_ANNOTATION] = value;
    else delete next[GRACE_PERIOD_ANNOTATION];
    onChange({
      ...draft,
      metadata: {
        ...draft.metadata,
        annotations: Object.keys(next).length ? next : undefined,
      },
    });
  };

  const graceNumber = Number(grace);
  const graceInvalid = grace !== "" && (!Number.isFinite(graceNumber) || graceNumber < 0);

  return (
    <div className="space-y-6">
      <Field
        label="Auto-restart"
        hint="When off, the StatefulSet is scaled to zero (Suspended). Data is preserved."
      >
        <div className="flex items-center gap-3 pt-1">
          <Switch
            checked={autoRestart}
            onCheckedChange={(v) => setSuspend(!v)}
            aria-label="Auto-restart"
          />
          <span className="text-sm text-muted">
            {autoRestart ? "Pod is desired Running" : "Pod is suspended"}
          </span>
        </div>
      </Field>

      <Field
        label="Termination grace period"
        hint="Seconds the pod has to flush state before SIGKILL. Persisted as an annotation; read by the operator on next reconcile."
      >
        <div className="flex items-center gap-2">
          <Input
            value={grace}
            onChange={(e) => setGrace(e.target.value.replace(/[^0-9]/g, ""))}
            placeholder="120"
            inputMode="numeric"
            className={graceInvalid ? "border-danger focus:ring-danger" : "max-w-32"}
          />
          <span className="text-xs text-muted">seconds</span>
        </div>
        {graceInvalid && (
          <div className="pt-1 text-xs text-danger">Must be a non-negative integer.</div>
        )}
      </Field>

      <Field
        label="Liveness probe"
        hint={
          probesAvailable
            ? "Liveness defaults come from the GameTemplate. Per-server overrides will arrive in a future iteration."
            : "Template does not declare probes."
        }
      >
        <div className="rounded border border-dashed border-border bg-surface/20 px-3 py-2 text-xs text-muted">
          Editing probes per-server is not yet supported.
        </div>
      </Field>
    </div>
  );
}
