import { useEffect, useState } from "react";
import { Switch } from "@/components/ui/switch";
import { Input } from "@/components/ui/input";
import { Select, type SelectOption } from "@/components/ui/select";
import { useMe, can } from "@/lib/auth";
import type { CaptureConfiguration } from "@/types";
import { Field } from "./Field";
import type { SectionProps } from "./types";

// Cluster ceiling on the per-server retention override (operator/api/v1alpha1
// CaptureConfiguration.RetentionSeconds: kubebuilder Minimum=1/Maximum=604800,
// default 86400). 7 days is a storage-limitation-informed engineering default
// (GDPR Art. 5(1)(e) informed the choice) — NOT a legal requirement, and the
// UI must not claim otherwise.
const CLUSTER_MAX_RETENTION_SECONDS = 604800;
const DEFAULT_RETENTION_SECONDS = 86400;

type RetentionUnit = "seconds" | "minutes" | "hours" | "days";

const UNIT_SECONDS: Record<RetentionUnit, number> = {
  seconds: 1,
  minutes: 60,
  hours: 3600,
  days: 86400,
};

const UNIT_OPTIONS: SelectOption[] = [
  { value: "seconds", label: "seconds" },
  { value: "minutes", label: "minutes" },
  { value: "hours", label: "hours" },
  { value: "days", label: "days" },
];

// Picks the coarsest unit that displays the given second count as a whole
// number, so "86400" reads back as "24 hours" instead of "86400 seconds".
function bestUnit(seconds: number): RetentionUnit {
  if (seconds > 0 && seconds % 86400 === 0) return "days";
  if (seconds > 0 && seconds % 3600 === 0) return "hours";
  if (seconds > 0 && seconds % 60 === 0) return "minutes";
  return "seconds";
}

export function NetworkCaptureSection({ draft, onChange, onValidityChange }: SectionProps) {
  const { data: me } = useMe();
  const namespace = draft.metadata.namespace ?? "gameplane-games";
  const canManage = can(me, "captures:manage", namespace);

  const capture = draft.spec.capture;
  const enabled = capture?.enabled ?? false;
  const storedSeconds = capture?.retentionSeconds;

  const [unit, setUnit] = useState<RetentionUnit>(() =>
    bestUnit(storedSeconds ?? DEFAULT_RETENTION_SECONDS),
  );
  const [rawValue, setRawValue] = useState<string>(() =>
    storedSeconds === undefined ? "" : String(storedSeconds / UNIT_SECONDS[unit]),
  );
  const [retentionError, setRetentionError] = useState<string | null>(null);

  useEffect(() => {
    onValidityChange?.(retentionError === null);
  }, [retentionError, onValidityChange]);

  const setCaptureField = <K extends keyof CaptureConfiguration>(
    key: K,
    value: CaptureConfiguration[K],
  ) => {
    const next: CaptureConfiguration = { ...(draft.spec.capture ?? {}), [key]: value };
    onChange({ ...draft, spec: { ...draft.spec, capture: next } });
  };

  const applyRetention = (valueStr: string, u: RetentionUnit) => {
    if (valueStr.trim() === "") {
      setRetentionError(null);
      setCaptureField("retentionSeconds", undefined);
      return;
    }
    const n = Number(valueStr);
    if (!Number.isFinite(n) || n <= 0) {
      setRetentionError("Enter a positive number.");
      return;
    }
    const seconds = Math.round(n * UNIT_SECONDS[u]);
    if (seconds > CLUSTER_MAX_RETENTION_SECONDS) {
      setRetentionError(
        `Exceeds the cluster maximum of 7 days (${CLUSTER_MAX_RETENTION_SECONDS.toLocaleString()} seconds).`,
      );
      return;
    }
    setRetentionError(null);
    setCaptureField("retentionSeconds", seconds);
  };

  const disabled = !canManage;

  return (
    <div className="space-y-6">
      <Field
        label="Enable Capture"
        hint="Records raw network protocol traffic from a sidecar container for later download and analysis."
      >
        <div className="space-y-3">
          <div className="flex items-center gap-3">
            <Switch
              checked={enabled}
              disabled={disabled}
              onCheckedChange={(v) => setCaptureField("enabled", v)}
              aria-label="Enable Capture"
            />
            <span className="text-sm text-muted">{enabled ? "Enabled" : "Disabled"}</span>
          </div>
          <p className="text-xs leading-relaxed text-muted">
            Turning this off stops any running capture and blocks new ones immediately. The
            capture container itself stays in the pod, idle, until the pod is next recreated —
            Kubernetes has no API to remove an ephemeral container.
          </p>
          <p className="text-xs leading-relaxed text-muted">
            Network packet capture requires admin access. Captures contain real player data
            (IP addresses, chat, credentials) and are not redacted.
          </p>
          {disabled && (
            <p className="text-xs leading-relaxed text-warning">
              You don&apos;t have permission to change capture settings for this server.
            </p>
          )}
        </div>
      </Field>

      <Field
        label="Retention Window"
        hint="Captures are automatically deleted after this window. Leave blank to use the cluster default."
      >
        <div className="space-y-2">
          <div className="flex items-center gap-2.5">
            <Input
              className="w-24"
              inputMode="numeric"
              disabled={disabled}
              value={rawValue}
              placeholder={String(DEFAULT_RETENTION_SECONDS / UNIT_SECONDS[unit])}
              onChange={(e) => {
                setRawValue(e.target.value);
                applyRetention(e.target.value, unit);
              }}
              aria-label="Retention window value"
            />
            <Select
              className="w-36"
              value={unit}
              disabled={disabled}
              options={UNIT_OPTIONS}
              onValueChange={(v) => {
                const nextUnit = v as RetentionUnit;
                setUnit(nextUnit);
                applyRetention(rawValue, nextUnit);
              }}
              aria-label="Retention window unit"
            />
          </div>
          {retentionError ? (
            <div className="text-xs text-danger">{retentionError}</div>
          ) : (
            <div className="text-xs text-muted">
              Cluster maximum: 7 days ({CLUSTER_MAX_RETENTION_SECONDS.toLocaleString()} seconds)
              — a storage-limitation-informed default, not a legal requirement.
            </div>
          )}
        </div>
      </Field>
    </div>
  );
}
