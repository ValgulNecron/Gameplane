import { Switch } from "@/components/ui/switch";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import type { IdleSpec, Probe, ProbeKind, ProbeSet } from "@/types";
import { cn } from "@/lib/utils";
import { Field } from "./Field";
import { GRACE_PERIOD_ANNOTATION, type SectionProps } from "./types";

// Mirrors the CRD's structural guard on IdleSpec.WakeWindows: a five (or six,
// with seconds) whitespace-separated field cron expression, at least 9
// characters. A blank or malformed entry is flagged inline rather than
// filtered out — filtering would delete the row out from under a user still
// mid-typing it.
const WAKE_WINDOW_PATTERN = /^\S+\s+\S+\s+\S+\s+\S+\s+\S+(\s+\S+)?$/;
function isValidWakeWindow(w: string): boolean {
  return w.length >= 9 && WAKE_WINDOW_PATTERN.test(w);
}

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

  const setIdle = (idle?: IdleSpec) => {
    const spec = { ...draft.spec };
    if (!idle || !idle.enabled) {
      delete spec.idle;
    } else {
      spec.idle = idle;
    }
    onChange({
      ...draft,
      spec,
    });
  };

  const addWakeWindow = () => {
    const idle = draft.spec.idle ?? { enabled: true };
    const windows = idle.wakeWindows ?? [];
    if (windows.length < 8) {
      setIdle({
        ...idle,
        wakeWindows: [...windows, ""],
      });
    }
  };

  const removeWakeWindow = (index: number) => {
    const idle = draft.spec.idle ?? { enabled: true };
    const windows = idle.wakeWindows ?? [];
    setIdle({
      ...idle,
      wakeWindows: windows.filter((_, i) => i !== index),
    });
  };

  const setWakeWindow = (index: number, value: string) => {
    const idle = draft.spec.idle ?? { enabled: true };
    // Copy before writing: `windows` aliases the draft's own array, and
    // mutating it in place would edit state React never sees change.
    const windows = [...(idle.wakeWindows ?? [])];
    windows[index] = value;
    setIdle({
      ...idle,
      wakeWindows: windows,
    });
  };

  const setIdleEnabled = (enabled: boolean) => {
    const idle = draft.spec.idle;
    if (!enabled) {
      // Keep the user's windows/threshold — `enabled: false` is exactly
      // what the operator reads as disabled (idleDecide's first branch),
      // and re-enabling must not start the user back from scratch.
      onChange({
        ...draft,
        spec: { ...draft.spec, idle: idle ? { ...idle, enabled: false } : undefined },
      });
      return;
    }
    setIdle({ enabled: true, afterMinutes: idle?.afterMinutes ?? 30, wakeWindows: idle?.wakeWindows ?? [] });
  };

  const setIdleAfterMinutes = (value: string) => {
    const idle = draft.spec.idle ?? { enabled: true };
    const minutes = value === "" ? undefined : Number(value);
    setIdle({
      ...idle,
      afterMinutes: minutes,
    });
  };

  const graceNumber = Number(grace);
  const graceInvalid =
    grace !== "" &&
    (!Number.isFinite(graceNumber) || graceNumber < 0 || graceNumber > 600 || !Number.isInteger(graceNumber));

  // Mirrors graceInvalid: the CRD enforces 5–1440 at admission, so a value
  // outside it here would otherwise sail past Save and fail the whole PUT,
  // discarding every other edit in the draft.
  const idleMinutes = draft.spec.idle?.afterMinutes;
  const idleMinutesText = idleMinutes !== undefined ? String(idleMinutes) : "";
  const idleInvalid =
    draft.spec.idle?.enabled === true &&
    idleMinutes !== undefined &&
    (!Number.isInteger(idleMinutes) || idleMinutes < 5 || idleMinutes > 1440);

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

      <Field
        label="Idle auto-sleep"
        hint="When enabled, scales the server to zero after it reports zero players for the configured duration. A wake window cron or an explicit :wake brings it back."
      >
        <div className="space-y-3">
          <div className="flex items-center gap-3 pt-1">
            <Switch
              checked={draft.spec.idle?.enabled ?? false}
              onCheckedChange={setIdleEnabled}
              aria-label="Enable idle auto-sleep"
            />
            <span className="text-sm text-muted">
              {draft.spec.idle?.enabled ? "Idle auto-sleep enabled" : "Disabled"}
            </span>
          </div>

          {draft.spec.idle?.enabled && (
            <div className="space-y-3 rounded border border-border p-3">
              <div>
                <label className="text-xs font-medium text-muted">
                  Sleep after
                  <Input
                    value={idleMinutesText}
                    onChange={(e) =>
                      setIdleAfterMinutes(e.target.value.replace(/[^0-9]/g, ""))
                    }
                    placeholder="30"
                    inputMode="numeric"
                    aria-label="Idle sleep after"
                    className={cn("mt-0.5", idleInvalid && "border-danger focus:ring-danger")}
                  />
                </label>
                <span className="text-xs text-muted">minutes of zero players (5–1440)</span>
                {idleInvalid && (
                  <div className="pt-1 text-xs text-danger">
                    Must be an integer between 5 and 1440.
                  </div>
                )}
              </div>

              <div>
                <div className="mb-2 flex items-center justify-between">
                  <label className="text-xs font-medium text-muted">Wake windows</label>
                  <Button
                    variant="ghost"
                    className="h-6 px-2 text-[11px]"
                    onClick={addWakeWindow}
                    disabled={(draft.spec.idle?.wakeWindows?.length ?? 0) >= 8}
                  >
                    Add wake window
                  </Button>
                </div>
                <div className="space-y-2">
                  {(draft.spec.idle?.wakeWindows ?? []).map((cron, i) => {
                    const invalid = !isValidWakeWindow(cron);
                    return (
                      <div key={i} className="space-y-1">
                        <div className="flex gap-2">
                          <Input
                            value={cron}
                            onChange={(e) => setWakeWindow(i, e.target.value)}
                            placeholder="0 9 * * 1-5"
                            aria-label={`Wake window ${i + 1}`}
                            className={cn("font-mono text-xs", invalid && "border-danger focus:ring-danger")}
                          />
                          <Button
                            variant="ghost"
                            className="h-8 px-2 text-[11px]"
                            onClick={() => removeWakeWindow(i)}
                          >
                            Remove
                          </Button>
                        </div>
                        {invalid && (
                          <div className="text-xs text-danger">
                            Must be a five-field cron expression (e.g. 0 9 * * 1-5).
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
                <span className="text-xs text-muted">Standard five-field cron. Max 8 entries.</span>
              </div>

              <div className="space-y-1 rounded bg-surface/40 p-2">
                <div className="text-xs text-muted">
                  A game that reports no player count will never sleep.
                </div>
                <div className="text-xs text-muted">
                  A wake window never restarts a server you stopped by hand.
                </div>
              </div>
            </div>
          )}
        </div>
      </Field>
    </div>
  );
}
