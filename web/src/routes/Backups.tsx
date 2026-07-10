import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import * as Dialog from "@radix-ui/react-dialog";
import { Plus } from "lucide-react";
import { Backups, Restores, Schedules, Servers } from "@/lib/endpoints";
import { useBackupDestinations } from "@/lib/destinations";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { FieldLabel } from "@/components/ui/field";
import { PageHeader } from "@/components/PageHeader";
import { cn, formatRelative } from "@/lib/utils";
import { PhaseBadge } from "@/components/ui/badge";
import { ErrorBanner } from "@/components/backups/ErrorBanner";
import { ScheduleForm } from "@/components/backups/ScheduleForm";
import { RestoreDialog } from "@/components/backups/RestoreDialog";
import { BackupDetailDrawer } from "@/components/backups/BackupDetailDrawer";
import { BackupRow } from "@/components/backups/BackupRow";
import { BackupFilters } from "@/components/backups/BackupFilters";
import { Switch } from "@/components/ui/switch";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import type { Backup } from "@/types";

type Tab = "backups" | "schedules" | "restores";
const TABS: { id: Tab; label: string }[] = [
  { id: "backups", label: "Backups" },
  { id: "schedules", label: "Schedules" },
  { id: "restores", label: "Restores" },
];

const BACKUP_PHASES = ["Pending", "Running", "Succeeded", "Failed"];
const RESTORE_PHASES = ["Pending", "Suspending", "Running", "Resuming", "Succeeded", "Failed"];

function readTab(): Tab {
  const v = new URLSearchParams(window.location.search).get("tab");
  return v === "schedules" || v === "restores" ? v : "backups";
}

export function BackupsPage() {
  const [tab, setTab] = useState<Tab>(() => readTab());
  const [backupNow, setBackupNow] = useState(false);
  useEffect(() => {
    const url = new URL(window.location.href);
    if (tab === "backups") url.searchParams.delete("tab");
    else url.searchParams.set("tab", tab);
    window.history.replaceState(null, "", url);
  }, [tab]);

  return (
    <div className="space-y-5 p-6">
      <PageHeader
        title="Backups"
        subtitle="Snapshots, schedules, and restores across all servers in this cluster."
        actions={
          <Button onClick={() => setBackupNow(true)}>
            <Plus className="h-4 w-4" /> Back up now
          </Button>
        }
      />
      {backupNow && <BackupNowDialog onClose={() => setBackupNow(false)} />}
      <div className="flex items-center gap-1 border-b border-border">
        {TABS.map((t) => (
          <button
            key={t.id}
            type="button"
            onClick={() => setTab(t.id)}
            className={cn(
              "h-9 border-b-2 px-3 text-sm transition-colors",
              tab === t.id
                ? "border-primary text-fg"
                : "border-transparent text-muted hover:text-fg",
            )}
          >
            {t.label}
          </button>
        ))}
      </div>
      {tab === "backups" && <BackupsTabPanel />}
      {tab === "schedules" && <SchedulesTabPanel />}
      {tab === "restores" && <RestoresTabPanel />}
    </div>
  );
}

function BackupsTabPanel() {
  const [search, setSearch] = useState("");
  const [server, setServer] = useState("");
  const [phase, setPhase] = useState("");
  const [restoringBackup, setRestoringBackup] = useState<Backup | null>(null);
  const [selectedBackup, setSelectedBackup] = useState<string | null>(null);

  const { data: backups } = useQuery({
    queryKey: ["backups"],
    queryFn: () => Backups.list(),
    refetchInterval: 5000,
  });
  const { data: serversList } = useQuery({
    queryKey: ["servers"],
    queryFn: () => Servers.list(),
  });

  const items = useMemo(() => backups?.items ?? [], [backups]);
  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return items.filter((b) => {
      if (server && b.spec.serverRef.name !== server) return false;
      if (phase && b.status?.phase !== phase) return false;
      if (q) {
        const hay = `${b.metadata.name} ${b.spec.serverRef.name}`.toLowerCase();
        if (!hay.includes(q)) return false;
      }
      return true;
    });
  }, [items, search, server, phase]);

  return (
    <div className="space-y-4">
      <BackupFilters
        search={search}
        onSearchChange={setSearch}
        server={server}
        onServerChange={setServer}
        phase={phase}
        onPhaseChange={setPhase}
        servers={serversList?.items ?? []}
        phases={BACKUP_PHASES}
        trailing={`${filtered.length} of ${items.length} ${items.length === 1 ? "backup" : "backups"}`}
      />

      <div className="overflow-hidden rounded-lg border border-border bg-card">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-surface/70 text-left text-[11px] uppercase tracking-wider text-muted">
              <tr>
                <th className="px-4 py-3">Name</th>
                <th className="px-4 py-3">Server</th>
                <th className="px-4 py-3">Phase</th>
                <th className="px-4 py-3">Size</th>
                <th className="px-4 py-3">Completed</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-10 text-center text-muted">
                    {items.length === 0
                      ? "No backups yet. Use “Back up now” to create the first one."
                      : "No backups match the current filters."}
                  </td>
                </tr>
              )}
              {filtered.map((b) => (
                <BackupRow
                  key={b.metadata.name}
                  backup={b}
                  showServer
                  onSelect={(x) => setSelectedBackup(x.metadata.name)}
                  onRestore={setRestoringBackup}
                />
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <RestoreDialog
        backup={restoringBackup}
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

// The "Back up now" form, relocated from an inline card above the table to a
// dialog launched from the page header (design frame 4avx0). The query and
// mutation logic is unchanged; the dialog closes once the snapshot starts.
function BackupNowDialog({ onClose }: { onClose: () => void }) {
  const qc = useQueryClient();
  const [createServer, setCreateServer] = useState("");
  const [createDest, setCreateDest] = useState("");

  const { data: serversList } = useQuery({
    queryKey: ["servers"],
    queryFn: () => Servers.list(),
  });
  const { data: destinations = [] } = useBackupDestinations();

  // When destinations resolve, default to the first one. The user can
  // still pick a different one if more than one exists.
  useEffect(() => {
    if (!createDest && destinations.length > 0) {
      setCreateDest(destinations[0].name);
    }
  }, [destinations, createDest]);

  const createNow = useMutation({
    mutationFn: () =>
      Backups.create({
        serverRef: { name: createServer },
        repoRef: { name: createDest, key: "repo" },
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["backups"] });
      onClose();
    },
  });

  const noDestinations = destinations.length === 0;

  return (
    <Dialog.Root
      open
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-[480px] max-w-[calc(100vw-2rem)] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-border bg-card p-5 text-fg shadow-2xl">
          <Dialog.Title className="text-base font-semibold">Back up now</Dialog.Title>
          <Dialog.Description className="pt-1 text-sm text-muted">
            Run a one-off snapshot outside any schedule.
          </Dialog.Description>
          <div className="space-y-4 pt-4">
            <FieldLabel label="Server">
              <select
                className="h-9 w-full rounded-md border border-border bg-surface px-3 text-sm"
                value={createServer}
                onChange={(e) => setCreateServer(e.target.value)}
              >
                <option value="" disabled>Select a server…</option>
                {(serversList?.items ?? []).map((s) => (
                  <option key={s.metadata.name} value={s.metadata.name}>
                    {s.metadata.name}
                  </option>
                ))}
              </select>
            </FieldLabel>
            {destinations.length > 1 && (
              <FieldLabel label="Destination">
                <select
                  className="h-9 w-full rounded-md border border-border bg-surface px-3 text-sm"
                  value={createDest}
                  onChange={(e) => setCreateDest(e.target.value)}
                >
                  {destinations.map((d) => (
                    <option key={d.name} value={d.name}>{d.name}</option>
                  ))}
                </select>
              </FieldLabel>
            )}
            {noDestinations && (
              <div className="text-xs text-muted">
                No backup destinations configured. Add one in{" "}
                <Link to="/admin" className="text-primary hover:underline">
                  admin settings
                </Link>{" "}
                to enable snapshots.
              </div>
            )}
            {createNow.error && <ErrorBanner err={createNow.error} />}
          </div>
          <div className="flex items-center justify-end gap-2 pt-5">
            <Button variant="ghost" size="sm" onClick={onClose} disabled={createNow.isPending}>
              Cancel
            </Button>
            <Button
              size="sm"
              disabled={!createServer || !createDest || createNow.isPending || noDestinations}
              onClick={() => createNow.mutate()}
            >
              {createNow.isPending ? "Starting…" : "Run snapshot"}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function SchedulesTabPanel() {
  const qc = useQueryClient();
  const [creatingFor, setCreatingFor] = useState<string>("");
  const [deleting, setDeleting] = useState<string | null>(null);

  const { data: schedules } = useQuery({
    queryKey: ["schedules"],
    queryFn: () => Schedules.list(),
    refetchInterval: 10000,
  });
  const { data: serversList } = useQuery({
    queryKey: ["servers"],
    queryFn: () => Servers.list(),
  });

  const items = schedules?.items ?? [];

  const toggleSuspend = useMutation({
    mutationFn: ({ name, suspend }: { name: string; suspend: boolean }) =>
      Schedules.patchSpec(name, { suspend }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["schedules"] }),
  });

  const remove = useMutation({
    mutationFn: (name: string) => Schedules.remove(name),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["schedules"] });
      setDeleting(null);
    },
  });

  return (
    <div className="space-y-4">
      <Card className="p-4">
        <div className="flex items-end gap-2">
          <div className="flex-1 space-y-1.5">
            <div className="text-xs font-medium text-fg">New schedule for</div>
            <select
              className="h-9 w-full rounded-md border border-border bg-surface px-3 text-sm"
              value={creatingFor}
              onChange={(e) => setCreatingFor(e.target.value)}
            >
              <option value="">Select a server…</option>
              {(serversList?.items ?? []).map((s) => (
                <option key={s.metadata.name} value={s.metadata.name}>
                  {s.metadata.name}
                </option>
              ))}
            </select>
          </div>
        </div>
      </Card>

      {creatingFor && (
        <ScheduleForm serverName={creatingFor} onClose={() => setCreatingFor("")} />
      )}
      {toggleSuspend.error && <ErrorBanner err={toggleSuspend.error} />}
      {remove.error && <ErrorBanner err={remove.error} />}

      <div className="overflow-hidden rounded-lg border border-border bg-card">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-surface/70 text-left text-[11px] uppercase tracking-wider text-muted">
              <tr>
                <th className="px-4 py-3">Name</th>
                <th className="px-4 py-3">Server</th>
                <th className="px-4 py-3">Cron</th>
                <th className="px-4 py-3">Last run</th>
                <th className="px-4 py-3">Next run</th>
                <th className="px-4 py-3">Active</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {items.length === 0 && (
                <tr>
                  <td colSpan={7} className="px-4 py-10 text-center text-muted">
                    No schedules configured yet.
                  </td>
                </tr>
              )}
              {items.map((s) => (
                <tr key={s.metadata.name} className="hover:bg-surface/40">
                  <td className="px-4 py-3 font-mono text-xs">{s.metadata.name}</td>
                  <td className="px-4 py-3">{s.spec.serverRef.name}</td>
                  <td className="px-4 py-3 font-mono text-xs">{s.spec.schedule}</td>
                  <td className="px-4 py-3 text-muted">
                    {formatRelative(s.status?.lastSuccessfulTime)}
                  </td>
                  <td className="px-4 py-3 text-muted">
                    {s.spec.suspend ? "—" : formatRelative(s.status?.nextScheduleTime)}
                  </td>
                  <td className="px-4 py-3">
                    <Switch
                      checked={!s.spec.suspend}
                      onCheckedChange={(active) =>
                        toggleSuspend.mutate({ name: s.metadata.name, suspend: !active })
                      }
                      aria-label="Schedule active"
                    />
                  </td>
                  <td className="px-4 py-3 text-right">
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => setDeleting(s.metadata.name)}
                    >
                      Delete
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(open) => { if (!open) setDeleting(null); }}
        title="Delete schedule?"
        description={
          <>
            <p>
              The cron schedule will be removed. Existing backups remain intact;
              new snapshots on this schedule will stop running until it&apos;s recreated.
            </p>
            {deleting && (
              <p className="pt-2">
                Type <span className="font-mono">{deleting}</span> to confirm.
              </p>
            )}
          </>
        }
        confirmPhrase={deleting ?? undefined}
        confirmLabel="Delete"
        destructive
        busy={remove.isPending}
        onConfirm={() => deleting && remove.mutate(deleting)}
      />
    </div>
  );
}

function RestoresTabPanel() {
  const [server, setServer] = useState("");
  const [phase, setPhase] = useState("");
  const [search, setSearch] = useState("");

  const { data: restores } = useQuery({
    queryKey: ["restores"],
    queryFn: () => Restores.list(),
    refetchInterval: 5000,
  });
  const { data: serversList } = useQuery({
    queryKey: ["servers"],
    queryFn: () => Servers.list(),
  });

  const items = useMemo(() => restores?.items ?? [], [restores]);
  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return items.filter((r) => {
      if (server && r.spec.serverRef.name !== server) return false;
      if (phase && r.status?.phase !== phase) return false;
      if (q) {
        const hay = `${r.metadata.name} ${r.spec.serverRef.name} ${r.spec.backupRef.name}`.toLowerCase();
        if (!hay.includes(q)) return false;
      }
      return true;
    });
  }, [items, search, server, phase]);

  return (
    <div className="space-y-4">
      <BackupFilters
        search={search}
        onSearchChange={setSearch}
        server={server}
        onServerChange={setServer}
        phase={phase}
        onPhaseChange={setPhase}
        servers={serversList?.items ?? []}
        phases={RESTORE_PHASES}
        trailing={`${filtered.length} of ${items.length} ${items.length === 1 ? "restore" : "restores"}`}
      />
      <div className="overflow-hidden rounded-lg border border-border bg-card">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-surface/70 text-left text-[11px] uppercase tracking-wider text-muted">
              <tr>
                <th className="px-4 py-3">Name</th>
                <th className="px-4 py-3">Backup</th>
                <th className="px-4 py-3">Target</th>
                <th className="px-4 py-3">Phase</th>
                <th className="px-4 py-3">Completed</th>
                <th className="px-4 py-3">Message</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-10 text-center text-muted">
                    {items.length === 0
                      ? "No restores have been run."
                      : "No restores match the current filters."}
                  </td>
                </tr>
              )}
              {filtered.map((r) => (
                <tr key={r.metadata.name} className="hover:bg-surface/40">
                  <td className="px-4 py-3 font-mono text-xs">{r.metadata.name}</td>
                  <td className="px-4 py-3 font-mono text-xs">{r.spec.backupRef.name}</td>
                  <td className="px-4 py-3">{r.spec.serverRef.name}</td>
                  <td className="px-4 py-3"><PhaseBadge phase={r.status?.phase} /></td>
                  <td className="px-4 py-3 text-muted">
                    {formatRelative(r.status?.completionTime)}
                  </td>
                  <td className="px-4 py-3 text-xs text-muted">
                    {r.status?.message ?? "—"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
