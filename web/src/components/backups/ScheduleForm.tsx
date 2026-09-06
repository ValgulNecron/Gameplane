import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Button,
  Card,
  CardContent,
  Input,
  Label,
  Description,
  Select,
  ListBox,
  ListBoxItem,
} from "@heroui/react";
import { Schedules } from "@/lib/endpoints";
import { useBackupDestinations } from "@/lib/destinations";
import { ErrorBanner } from "./ErrorBanner";
import { RetentionFields, buildRetention, type RetentionForm } from "./RetentionFields";

interface Props {
  serverName: string;
  onClose: () => void;
}

export function ScheduleForm({ serverName, onClose }: Props) {
  const qc = useQueryClient();
  const { data: destinations = [] } = useBackupDestinations();
  const [form, setForm] = useState({
    schedule: "0 */6 * * *",
    strategy: "restic-snapshot" as "restic-snapshot" | "volume-snapshot",
    repoName: "",
    repoKey: "repo",
  });
  const [retention, setRetention] = useState<RetentionForm>({ keepLast: 7 });
  const isVolumeSnapshot = form.strategy === "volume-snapshot";
  // Default the destination to the first one once the list resolves. The
  // user can still pick a different one if more than one exists. Adjusted
  // directly during render (not in an effect) — the condition becomes false
  // as soon as repoName is set, so this can't loop.
  if (!form.repoName && destinations.length > 0) {
    setForm((f) => ({ ...f, repoName: destinations[0].name }));
  }

  const create = useMutation({
    mutationFn: () =>
      Schedules.create({
        serverRef: { name: serverName },
        schedule: form.schedule,
        strategy: form.strategy,
        // restic needs a repo; volume-snapshot captures a CSI snapshot.
        ...(isVolumeSnapshot
          ? {}
          : { repoRef: { name: form.repoName, key: form.repoKey } }),
        retention: buildRetention(retention),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["schedules"] });
      onClose();
    },
  });

  return (
    <Card className="space-y-3">
      <CardContent className="space-y-4 pt-6">
        <div>
          <h3 className="text-sm font-medium">New backup schedule</h3>
          <Description className="text-xs">
            {isVolumeSnapshot
              ? "Cron-based recurring CSI volume snapshot. No restic repository needed."
              : "Cron-based recurring snapshot, stored in a configured restic repository."}
          </Description>
        </div>

        <div className="space-y-2">
          <Select
            value={form.strategy}
            onChange={(v) =>
              setForm({ ...form, strategy: v as typeof form.strategy })
            }
          >
            <Label className="text-xs">Backup type</Label>
            <Select.Trigger className="w-full rounded border border-border bg-surface px-3 py-2 text-sm hover:bg-surface/80">
              <Select.Value />
              <Select.Indicator className="ml-auto h-4 w-4" />
            </Select.Trigger>
            <Select.Popover className="rounded border border-border">
              <ListBox aria-label="Backup type">
                <ListBoxItem id="restic-snapshot">Restic snapshot (to a repository)</ListBoxItem>
                <ListBoxItem id="volume-snapshot">Volume snapshot (CSI)</ListBoxItem>
              </ListBox>
            </Select.Popover>
          </Select>
        </div>

        <div className="space-y-4">
          <div className="grid gap-3 md:grid-cols-[1fr_1fr]">
            <div className="space-y-2">
              <Label htmlFor="schedule-input" className="text-xs">Schedule (cron)</Label>
              <Input
                id="schedule-input"
                value={form.schedule}
                onChange={(e) => setForm({ ...form, schedule: e.target.value })}
                placeholder="0 */6 * * *"
                className="h-9"
              />
            </div>
            {!isVolumeSnapshot && (
              <div className="space-y-2">
                <Select
                  value={form.repoName}
                  onChange={(v) => setForm({ ...form, repoName: v as string })}
                >
                  <Label className="text-xs">Destination</Label>
                  <Select.Trigger className="w-full rounded border border-border bg-surface px-3 py-2 text-sm hover:bg-surface/80">
                    <Select.Value />
                    <Select.Indicator className="ml-auto h-4 w-4" />
                  </Select.Trigger>
                  <Select.Popover className="rounded border border-border">
                    <ListBox aria-label="Destination">
                      {destinations.length === 0 ? (
                        <ListBoxItem id="" isDisabled>No destinations configured</ListBoxItem>
                      ) : (
                        destinations.map((d) => (
                          <ListBoxItem key={d.name} id={d.name}>{d.name}</ListBoxItem>
                        ))
                      )}
                    </ListBox>
                  </Select.Popover>
                </Select>
              </div>
            )}
          </div>

          {!isVolumeSnapshot && (
            <div className="space-y-2">
              <Label htmlFor="repo-key-input" className="text-xs">Repo secret · key</Label>
              <Input
                id="repo-key-input"
                value={form.repoKey}
                onChange={(e) => setForm({ ...form, repoKey: e.target.value })}
                className="h-9"
              />
            </div>
          )}

          <div className="space-y-3">
            <Label className="text-xs">Retention policy</Label>
            <RetentionFields value={retention} onChange={setRetention} />
          </div>
        </div>

        {create.error && <ErrorBanner err={create.error} />}

        <div className="flex justify-end gap-2 pt-2">
          <Button variant="secondary" onPress={onClose} size="sm">Cancel</Button>
          <Button
            onPress={() => create.mutate()}
            isDisabled={
              !form.schedule || (!isVolumeSnapshot && !form.repoName) || create.isPending
            }
            size="sm"
          >
            {create.isPending ? "Creating…" : "Create schedule"}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
