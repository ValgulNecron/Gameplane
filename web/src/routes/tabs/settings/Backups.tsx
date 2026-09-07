import { useState } from "react";
import { ChevronDown } from "lucide-react";
import {
  Switch,
  Input,
  Popover,
  PopoverTrigger,
  PopoverContent,
  ListBox,
  ListBoxItem,
} from "@heroui/react";
import { RetentionFields, buildRetention, type RetentionForm } from "@/components/backups/RetentionFields";
import { useBackupDestinations } from "@/lib/destinations";
import type { InlineBackupPolicy } from "@/types";
import { cn } from "@/lib/utils";
import { Field } from "./Field";
import type { SectionProps } from "./types";

export function BackupsSection({ draft, onChange }: SectionProps) {
  const policy = draft.spec.backupPolicy;
  const { data: destinations = [] } = useBackupDestinations();
  const [destPopoverOpen, setDestPopoverOpen] = useState(false);

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

  const retentionForm: RetentionForm = policy?.retention ?? {};
  const selectedDestName = policy?.repoRef.name;

  return (
    <div className="space-y-6">
      <Field
        label="Enable scheduled backups"
        hint="Automatically create backups on a cron schedule. The operator manages the BackupSchedule CR."
      >
        <div className="flex items-center gap-3 pt-1">
          <Switch
            isSelected={!!policy}
            isDisabled={!policy && destinations.length === 0}
            onChange={(enabled) => {
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
        {!policy && destinations.length === 0 && (
          <div className="pt-1 text-xs text-muted">
            Configure a backup destination first.
          </div>
        )}
      </Field>

      {policy && (
        <>
          <Field
            label="Schedule (cron)"
            hint="When to run backups. Example: '0 */6 * * *' for every 6 hours."
          >
            <Input
              type="text"
              value={policy.schedule}
              onChange={(e) => setPolicyField("schedule", e.target.value)}
              placeholder="0 */6 * * *"
              spellCheck={false}
            />
          </Field>

          <Field label="Destination">
            <Popover isOpen={destPopoverOpen} onOpenChange={setDestPopoverOpen}>
              <PopoverTrigger
                className={cn(
                  "flex items-center justify-between gap-2 rounded-lg border border-border bg-card px-3 py-2 text-sm",
                  "hover:bg-surface transition-colors cursor-pointer",
                )}
              >
                <span className={selectedDestName ? "text-fg" : "text-muted"}>
                  {selectedDestName || "Select destination…"}
                </span>
                <ChevronDown className="h-4 w-4 text-muted shrink-0" />
              </PopoverTrigger>
              <PopoverContent className="min-w-[200px]">
                <ListBox
                  aria-label="Backup destination"
                  onSelectionChange={(selected) => {
                    setPolicyField("repoRef", { name: String(selected), key: "repo" });
                    setDestPopoverOpen(false);
                  }}
                >
                  {destinations.map((d) => (
                    <ListBoxItem key={d.name} id={d.name}>
                      {d.name}
                    </ListBoxItem>
                  ))}
                </ListBox>
              </PopoverContent>
            </Popover>
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
                isSelected={policy.suspend ?? false}
                onChange={(v) => setPolicyField("suspend", v)}
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
