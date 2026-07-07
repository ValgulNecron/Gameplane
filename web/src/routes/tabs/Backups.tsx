import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Archive, CalendarClock, Clock, HardDrive } from "lucide-react";
import { Button } from "@/components/ui/button";
import { StatCard } from "@/components/ui/stat";
import { Backups, Schedules, Restores } from "@/lib/endpoints";
import { useBackupDestinations } from "@/lib/destinations";
import { formatBytes, formatRelative, formatRelativeFuture, parseQuantityToBytes } from "@/lib/utils";
import { PhaseBadge } from "@/components/ui/badge";
import { ErrorBanner } from "@/components/backups/ErrorBanner";
import { ScheduleForm } from "@/components/backups/ScheduleForm";
import { RestoreDialog } from "@/components/backups/RestoreDialog";
import { BackupDetailDrawer } from "@/components/backups/BackupDetailDrawer";
import type { Backup } from "@/types";

export function BackupsTab({ name, ns: _ns }: { name: string; ns?: string }) {
  const qc = useQueryClient();
  const [creatingSchedule, setCreatingSchedule] = useState(false);
  const [restoringBackup, setRestoringBackup] = useState<Backup | null>(null);
  const [selectedBackup, setSelectedBackup] = useState<string | null>(null);

  const { data: backups } = useQuery({
    queryKey: ["backups"],
    queryFn: () => Backups.list(),
    refetchInterval: 5000,
  });
  const { data: schedules } = useQuery({
    queryKey: ["schedules"],
    queryFn: () => Schedules.list(),
  });
  const { data: restores } = useQuery({
    queryKey: ["restores"],
    queryFn: () => Restores.list(),
    refetchInterval: 5000,
  });
  const { data: destinations = [] } = useBackupDestinations();

  // Multi-destination picking is handled on the dashboard's Backups page;
  // here we only act when there's exactly one configured destination.
  const lone = destinations.length === 1 ? destinations[0] : null;

  const createNow = useMutation({
    mutationFn: () =>
      Backups.create({
        serverRef: { name },
        repoRef: { name: lone!.name, key: "url" },
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["backups"] }),
  });

  const backupNowDisabled = !lone || createNow.isPending;
  const backupNowHint =
    destinations.length === 0
      ? "No backup destination configured. Add one in admin settings."
      : destinations.length > 1
        ? "Multiple destinations configured — use the Backups page to pick one."
        : undefined;

  const serverBackups = backups?.items.filter((b) => b.spec.serverRef.name === name) ?? [];
  const serverSchedules = schedules?.items.filter((s) => s.spec.serverRef.name === name) ?? [];
  const serverRestores = restores?.items.filter((r) => r.spec.serverRef.name === name) ?? [];

  // Summary tiles, derived from the same lists the table renders.
  const succeeded = serverBackups.filter((b) => b.status?.phase === "Succeeded");
  const completions = succeeded
    .map((b) => b.status?.completionTime)
    .filter((t): t is string => Boolean(t))
    .sort();
  const lastBackupAt = completions[completions.length - 1];
  const nextRuns = serverSchedules
    .filter((s) => !s.spec.suspend)
    .map((s) => s.status?.nextScheduleTime)
    .filter((t): t is string => Boolean(t))
    .sort();
  const nextBackupAt = nextRuns[0];
  const totalBytes = succeeded.reduce((acc, b) => acc + parseQuantityToBytes(b.status?.size), 0);

  return (
    <div className="space-y-6 p-6">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <StatCard
          label="Last backup"
          value={formatRelative(lastBackupAt)}
          icon={<Clock size={16} />}
          accent="primary"
        />
        <StatCard
          label="Next backup"
          value={nextBackupAt ? formatRelativeFuture(nextBackupAt) : "—"}
          icon={<CalendarClock size={16} />}
          accent="success"
        />
        <StatCard
          label="Backups"
          value={serverBackups.length}
          icon={<Archive size={16} />}
          accent="violet"
        />
        <StatCard
          label="Total size"
          value={totalBytes ? formatBytes(totalBytes) : "—"}
          icon={<HardDrive size={16} />}
          accent="warning"
        />
      </div>

      <section>
        <div className="flex items-center justify-between pb-3">
          <h2 className="text-sm text-muted">Schedules</h2>
          <Button
            size="sm"
            variant="outline"
            onClick={() => setCreatingSchedule(true)}
            disabled={creatingSchedule}
          >
            New schedule
          </Button>
        </div>
        {creatingSchedule && (
          <ScheduleForm serverName={name} onClose={() => setCreatingSchedule(false)} />
        )}
        <div className="space-y-1 pt-2">
          {serverSchedules.map((s) => (
            <div
              key={s.metadata.name}
              className="flex justify-between rounded border border-border bg-surface/30 px-4 py-2 text-sm"
            >
              <span className="font-mono">{s.spec.schedule}</span>
              <span className="text-muted">
                Next: {formatRelative(s.status?.nextScheduleTime)}
              </span>
            </div>
          ))}
          {serverSchedules.length === 0 && !creatingSchedule && (
            <p className="text-sm text-muted">No schedules yet.</p>
          )}
        </div>
      </section>

      {serverRestores.length > 0 && (
        <section>
          <h2 className="pb-3 text-sm text-muted">Recent restores</h2>
          <div className="space-y-1">
            {serverRestores.map((r) => (
              <div
                key={r.metadata.name}
                className="flex justify-between rounded border border-border bg-surface/30 px-4 py-2 text-sm"
              >
                <span className="font-mono">{r.spec.backupRef.name}</span>
                <span className="text-muted">
                  <PhaseBadge phase={r.status?.phase} />
                  {r.status?.completionTime &&
                    ` · ${formatRelative(r.status.completionTime)}`}
                </span>
              </div>
            ))}
          </div>
        </section>
      )}

      <section>
        <div className="flex items-center justify-between pb-3">
          <h2 className="text-sm text-muted">Backups</h2>
          <Button
            size="sm"
            onClick={() => createNow.mutate()}
            disabled={backupNowDisabled}
            title={backupNowHint}
          >
            {createNow.isPending ? "Starting…" : "Back up now"}
          </Button>
        </div>
        {createNow.error && <ErrorBanner err={createNow.error} />}
        <table className="w-full text-sm">
          <thead className="text-left text-xs uppercase text-muted">
            <tr>
              <th className="py-2">Name</th>
              <th>Phase</th>
              <th>Size</th>
              <th>Completed</th>
              <th />
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {serverBackups.map((b) => {
              const restorable =
                b.status?.phase === "Succeeded" && Boolean(b.status?.snapshotID);
              return (
                <tr
                  key={b.metadata.name}
                  className="cursor-pointer hover:bg-surface/40"
                  onClick={() => setSelectedBackup(b.metadata.name)}
                >
                  <td className="py-2 font-mono">{b.metadata.name}</td>
                  <td>
                    <PhaseBadge phase={b.status?.phase} />
                  </td>
                  <td className="font-mono">{b.status?.size ?? "—"}</td>
                  <td className="font-mono text-muted">
                    {formatRelative(b.status?.completionTime)}
                  </td>
                  <td className="text-right" onClick={(e) => e.stopPropagation()}>
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={!restorable}
                      onClick={() => setRestoringBackup(b)}
                    >
                      Restore
                    </Button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </section>

      <RestoreDialog
        backup={restoringBackup}
        defaultServer={name}
        onClose={() => setRestoringBackup(null)}
      />
      <BackupDetailDrawer
        name={selectedBackup}
        onClose={() => setSelectedBackup(null)}
        onRestore={(b) => {
          setSelectedBackup(null);
          setRestoringBackup(b);
        }}
      />
    </div>
  );
}
