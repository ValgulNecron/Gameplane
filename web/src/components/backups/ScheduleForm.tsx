import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Schedules } from "@/lib/endpoints";
import { useBackupDestinations } from "@/lib/destinations";
import { ErrorBanner } from "./ErrorBanner";
import { FieldLabel } from "@/components/ui/field";

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
    keepLast: 7,
  });
  const isVolumeSnapshot = form.strategy === "volume-snapshot";
  // Default the destination to the first one once the list resolves.
  // The user can still pick a different one if more than one exists.
  useEffect(() => {
    if (!form.repoName && destinations.length > 0) {
      setForm((f) => ({ ...f, repoName: destinations[0].name }));
    }
  }, [destinations, form.repoName]);

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
        retention: { keepLast: form.keepLast },
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["schedules"] });
      onClose();
    },
  });

  return (
    <Card className="space-y-3 p-4">
      <div className="text-sm font-medium">New backup schedule</div>
      <p className="text-xs text-muted">
        {isVolumeSnapshot
          ? "Cron-based recurring CSI volume snapshot. No restic repository needed."
          : "Cron-based recurring snapshot, stored in a configured restic repository."}
      </p>
      <FieldLabel label="Backup type">
        <select
          className="h-9 w-full rounded-md border border-border bg-surface px-3 text-sm"
          value={form.strategy}
          onChange={(e) =>
            setForm({ ...form, strategy: e.target.value as typeof form.strategy })
          }
          aria-label="Backup type"
        >
          <option value="restic-snapshot">Restic snapshot (to a repository)</option>
          <option value="volume-snapshot">Volume snapshot (CSI)</option>
        </select>
      </FieldLabel>
      <div className="grid gap-3 md:grid-cols-[1fr_140px]">
        <FieldLabel label="Schedule (cron)">
          <Input
            value={form.schedule}
            onChange={(e) => setForm({ ...form, schedule: e.target.value })}
            placeholder="0 */6 * * *"
          />
        </FieldLabel>
        <FieldLabel label="Keep last">
          <Input
            type="number"
            min={1}
            value={form.keepLast}
            onChange={(e) => setForm({ ...form, keepLast: Number(e.target.value) })}
          />
        </FieldLabel>
        {!isVolumeSnapshot && (
          <>
            <FieldLabel label="Destination">
              <select
                className="h-9 w-full rounded-md border border-border bg-surface px-3 text-sm"
                value={form.repoName}
                onChange={(e) => setForm({ ...form, repoName: e.target.value })}
                aria-label="Destination"
              >
                {destinations.length === 0 && (
                  <option value="" disabled>No destinations configured</option>
                )}
                {destinations.map((d) => (
                  <option key={d.name} value={d.name}>{d.name}</option>
                ))}
              </select>
            </FieldLabel>
            <FieldLabel label="Repo secret · key">
              <Input
                value={form.repoKey}
                onChange={(e) => setForm({ ...form, repoKey: e.target.value })}
              />
            </FieldLabel>
          </>
        )}
      </div>
      {create.error && <ErrorBanner err={create.error} />}
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>Cancel</Button>
        <Button
          onClick={() => create.mutate()}
          disabled={
            !form.schedule || (!isVolumeSnapshot && !form.repoName) || create.isPending
          }
        >
          {create.isPending ? "Creating…" : "Create schedule"}
        </Button>
      </div>
    </Card>
  );
}
