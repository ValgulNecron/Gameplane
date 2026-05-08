import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Schedules } from "@/lib/endpoints";
import { ErrorBanner } from "./ErrorBanner";
import { FieldLabel } from "./FieldLabel";

interface Props {
  serverName: string;
  onClose: () => void;
}

export function ScheduleForm({ serverName, onClose }: Props) {
  const qc = useQueryClient();
  const [form, setForm] = useState({
    schedule: "0 */6 * * *",
    repoName: "kestrel-backup-repo",
    repoKey: "url",
    keepLast: 7,
  });
  const create = useMutation({
    mutationFn: () =>
      Schedules.create({
        serverRef: { name: serverName },
        schedule: form.schedule,
        repoRef: { name: form.repoName, key: form.repoKey },
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
        Cron-based recurring snapshot. Stored in the configured restic repository.
      </p>
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
        <FieldLabel label="Repo secret · name">
          <Input
            value={form.repoName}
            onChange={(e) => setForm({ ...form, repoName: e.target.value })}
          />
        </FieldLabel>
        <FieldLabel label="Repo secret · key">
          <Input
            value={form.repoKey}
            onChange={(e) => setForm({ ...form, repoKey: e.target.value })}
          />
        </FieldLabel>
      </div>
      {create.error && <ErrorBanner err={create.error} />}
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>Cancel</Button>
        <Button
          onClick={() => create.mutate()}
          disabled={!form.schedule || !form.repoName || create.isPending}
        >
          {create.isPending ? "Creating…" : "Create schedule"}
        </Button>
      </div>
    </Card>
  );
}
