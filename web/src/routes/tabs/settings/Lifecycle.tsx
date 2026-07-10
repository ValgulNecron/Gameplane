import { Switch } from "@/components/ui/switch";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import type { Probe, ProbeKind, ProbeSet } from "@/types";
import { Field } from "./Field";
import { GRACE_PERIOD_ANNOTATION, type SectionProps } from "./types";

const PROBE_FIELDS: { key: keyof Probe; label: string }[] = [
  { key: "initialDelaySeconds", label: "Initial delay" },
  { key: "periodSeconds", label: "Period" },
  { key: "timeoutSeconds", label: "Timeout" },
  { key: "failureThreshold", label: "Failure threshold" },
  { key: "successThreshold", label: "Success threshold" },
];

const PROBE_KINDS: ProbeKind[] = ["readiness", "liveness", "startup"];

export function LifecycleSection({ draft, onChange, template }: SectionProps) {
  const annotations = draft.metadata.annotations ?? {};
  const autoRestart = !(draft.spec.suspend ?? false);
  // Read the real spec field; fall back to the legacy annotation for servers
  // created before the migration so their value still shows (display only —
  // we don't mutate the draft on mount, so this doesn't mark the form dirty).
  const grace =
    draft.spec.stopGracePeriodSeconds !== undefined
      ? String(draft.spec.stopGracePeriodSeconds)
      : (annotations[GRACE_PERIOD_ANNOTATION] ?? "");
  const templateProbes = template?.spec.probes;
  const overrides = draft.spec.probes;

  // setProbe writes a per-server override for `kind`, seeded from the
  // template probe so the action (httpGet/tcpSocket/exec) is preserved.
  const setProbe = (kind: ProbeKind, field: keyof Probe, raw: string) => {
    const base = overrides?.[kind] ?? templateProbes?.[kind] ?? {};
    const next: Probe = { ...base };
    if (raw === "") delete next[field];
    else next[field] = Number(raw);
    const probes: ProbeSet = { ...overrides, [kind]: next };
    onChange({ ...draft, spec: { ...draft.spec, probes } });
  };

  const resetProbe = (kind: ProbeKind) => {
    if (!overrides) return;
    const probes: ProbeSet = { ...overrides };
    delete probes[kind];
    onChange({
      ...draft,
      spec: {
        ...draft.spec,
        probes: Object.keys(probes).length ? probes : undefined,
      },
    });
  };

  const setSuspend = (suspend: boolean) => {
    onChange({
      ...draft,
      spec: { ...draft.spec, suspend },
    });
  };

  const setGrace = (value: string) => {
    // Write the field the operator actually reads, and migrate off the dead
    // legacy annotation on first edit.
    const spec = { ...draft.spec };
    if (value === "") delete spec.stopGracePeriodSeconds;
    else spec.stopGracePeriodSeconds = Number(value);
    const nextAnnotations = { ...annotations };
    delete nextAnnotations[GRACE_PERIOD_ANNOTATION];
    onChange({
      ...draft,
      spec,
      metadata: {
        ...draft.metadata,
        annotations: Object.keys(nextAnnotations).length ? nextAnnotations : undefined,
      },
    });
  };

  const graceNumber = Number(grace);
  const graceInvalid =
    grace !== "" &&
    (!Number.isFinite(graceNumber) || graceNumber < 0 || graceNumber > 600);

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
        hint="Seconds the operator waits for the template's stop sequence to finish before scaling the pod down. Range 0–600; defaults to 30. No effect when the template declares no stop sequence."
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
          <div className="pt-1 text-xs text-danger">
            Must be an integer between 0 and 600.
          </div>
        )}
      </Field>

      <Field
        label="Health probes"
        hint="Per-server overrides for the template's readiness/liveness/startup probe timing. The probe action is inherited from the template."
      >
        <div className="space-y-4">
          {PROBE_KINDS.map((kind) => {
            const tmplProbe = templateProbes?.[kind];
            const override = overrides?.[kind];
            const effective = override ?? tmplProbe;
            if (!tmplProbe && !override) {
              return (
                <div key={kind} className="text-xs text-muted">
                  <span className="font-medium capitalize text-fg">{kind}</span>: not
                  defined by the template.
                </div>
              );
            }
            return (
              <div key={kind} className="rounded border border-border p-3">
                <div className="flex items-center justify-between pb-2">
                  <span className="text-sm font-medium capitalize">{kind}</span>
                  {override ? (
                    <Button
                      variant="ghost"
                      className="h-6 px-2 text-[11px]"
                      onClick={() => resetProbe(kind)}
                    >
                      Reset to template
                    </Button>
                  ) : (
                    <span className="text-[11px] text-muted">from template</span>
                  )}
                </div>
                <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
                  {PROBE_FIELDS.map((f) => (
                    <label key={String(f.key)} className="text-[11px] text-muted">
                      {f.label}
                      <Input
                        className="mt-0.5"
                        inputMode="numeric"
                        value={String((effective?.[f.key] as number | undefined) ?? "")}
                        onChange={(e) =>
                          setProbe(kind, f.key, e.target.value.replace(/[^0-9]/g, ""))
                        }
                        placeholder="—"
                        aria-label={`${kind} ${f.label}`}
                      />
                    </label>
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      </Field>
    </div>
  );
}
