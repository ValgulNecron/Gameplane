import { Switch } from "@/components/ui/switch";
import { Input } from "@/components/ui/input";
import { Select, type SelectOption } from "@/components/ui/select";
import { RetentionFields, buildRetention, type RetentionForm } from "@/components/backups/RetentionFields";
import { useBackupDestinations } from "@/lib/destinations";
import type { InlineBackupPolicy } from "@/types";
import { Field } from "./Field";
import type { SectionProps } from "./types";

export function BackupsSection({ draft, onChange }: SectionProps) {
  const policy = draft.spec.backupPolicy;
  const { data: destinations = [] } = useBackupDestinations();

  const setPolicy = (next: InlineBackupPolicy | undefined) => {
    onChange({
      ...draft,
      spec: {
        ...draft.spec,
        backupPolicy: next,
      },
    });
  };

  const setPolicyField = <K extends keyof InlineBackupPolicy>(
    key: K,
    value: InlineBackupPolicy[K],
  ) => {
    if (!policy) return;
    setPolicy({ ...policy, [key]: value });
  };

  const setRetention = (retention: RetentionForm) => {
    if (!policy) return;
    const built = buildRetention(retention);
    setPolicyField("retention", built);
  };

  const destinationOptions: SelectOption[] = destinations.map((d) => ({
    value: d.name,
    label: d.name,
  }));

  const retentionForm: RetentionForm = policy?.retention ?? {};

  return (
    <div className="space-y-6">
      <Field
        label="Enable scheduled backups"
        hint="Automatically create backups on a cron schedule. The operator manages the BackupSchedule CR."
      >
        <div className="flex items-center gap-3 pt-1">
          <Switch
            checked={!!policy}
            onCheckedChange={(enabled) => {
              if (enabled) {
                // Seed with defaults
                setPolicy({
                  schedule: "0 */6 * * *",
                  repoRef: {
                    name: destinations[0]?.name ?? "",
                    key: "repo",
                  },
                });
              } else {
                setPolicy(undefined);
              }
            }}
            aria-label="Enable scheduled backups"
          />
          <span className="text-sm text-muted">
            {policy ? "Enabled" : "Disabled"}
          </span>
        </div>
      </Field>

      {policy && (
        <>
          <Field
            label="Schedule (cron)"
            hint="When to run backups. Example: '0 */6 * * *' for every 6 hours."
          >
            <Input
              value={policy.schedule}
              onChange={(e) => setPolicyField("schedule", e.target.value)}
              placeholder="0 */6 * * *"
              spellCheck={false}
            />
          </Field>

          <Field label="Destination">
            <Select
              value={policy.repoRef.name}
              options={destinationOptions}
              onValueChange={(name) => {
                setPolicyField("repoRef", { name, key: "repo" });
              }}
            />
          </Field>

          <div>
            <div className="mb-3 text-sm font-medium text-fg">Retention policy</div>
            <RetentionFields value={retentionForm} onChange={setRetention} />
          </div>

          <Field
            label="Suspend schedule"
            hint="When enabled, scheduled backups will not run, but the schedule is preserved."
          >
            <div className="flex items-center gap-3 pt-1">
              <Switch
                checked={policy.suspend ?? false}
                onCheckedChange={(v) => setPolicyField("suspend", v)}
                aria-label="Suspend schedule"
              />
              <span className="text-sm text-muted">
                {policy.suspend ? "Suspended" : "Active"}
              </span>
            </div>
          </Field>
        </>
      )}
    </div>
  );
}
