import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { Plus } from "lucide-react";
import { Backups, Restores, Schedules, Servers } from "@/lib/endpoints";
import { useBackupDestinations } from "@/lib/destinations";
import {
  Button,
  Card,
  Select,
  ListBox,
  ListBoxItem,
  Tabs,
  Tab,
  Table,
  Modal,
  ModalBackdrop,
  ModalContainer,
  ModalDialog,
  ModalHeader,
  ModalHeading,
  ModalBody,
  ModalFooter,
} from "@heroui/react";
import { Switch } from "@/components/ui/switch";
import { PageHeader } from "@/components/PageHeader";
import { formatRelative } from "@/lib/utils";
import { PhaseChip } from "@/components/hero/PhaseChip";
import { ErrorBanner } from "@/components/backups/ErrorBanner";
import { ScheduleForm } from "@/components/backups/ScheduleForm";
import { RestoreDialog } from "@/components/backups/RestoreDialog";
import { BackupDetailDrawer } from "@/components/backups/BackupDetailDrawer";
import { BackupRow } from "@/components/backups/BackupRow";
import { BackupFilters } from "@/components/backups/BackupFilters";
import { ConfirmDialog } from "@/components/hero/ConfirmDialog";
import type { Backup } from "@/types";

type TabKey = "backups" | "schedules" | "restores";
const TABS: { id: TabKey; label: string }[] = [
  { id: "backups", label: "Backups" },
  { id: "schedules", label: "Schedules" },
  { id: "restores", label: "Restores" },
];

const BACKUP_PHASES = ["Pending", "Running", "Succeeded", "Failed"];
const RESTORE_PHASES = ["Pending", "Suspending", "Running", "Resuming", "Succeeded", "Failed"];

function readTab(): TabKey {
  const v = new URLSearchParams(window.location.search).get("tab");
  return v === "schedules" || v === "restores" ? v : "backups";
}

export function BackupsPage() {
  const [tab, setTab] = useState<TabKey>(() => readTab());
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
          <Button onPress={() => setBackupNow(true)}>
            <Plus className="h-4 w-4" /> Back up now
          </Button>
        }
      />
      {backupNow && <BackupNowDialog onClose={() => setBackupNow(false)} />}
      <Tabs selectedKey={tab} onSelectionChange={(key) => setTab(key as TabKey)}>
        <Tabs.List>
          {TABS.map((t) => (
            <Tab key={t.id} id={t.id}>
              {t.label}
            </Tab>
          ))}
        </Tabs.List>
      </Tabs>
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

      <div className="rounded-lg border border-border bg-card overflow-hidden">
        <Table.Root>
          <Table.ScrollContainer>
            <Table.Content aria-label="Backups">
              <Table.Header>
                <Table.Column key="name" isRowHeader>
                  Name
                </Table.Column>
                <Table.Column key="server">Server</Table.Column>
                <Table.Column key="phase">Phase</Table.Column>
                <Table.Column key="size">Size</Table.Column>
                <Table.Column key="completed">Completed</Table.Column>
                <Table.Column key="actions" className="text-end" />
              </Table.Header>
              <Table.Body
                renderEmptyState={() => (
                  <>
                    {items.length === 0
                      ? 'No backups yet. Use "Back up now" to create the first one.'
                      : "No backups match the current filters."}
                  </>
                )}
              >
                {filtered.map((b) => (
                  <BackupRow
                    key={b.metadata.name}
                    backup={b}
                    showServer
                    onSelect={(x) => setSelectedBackup(x.metadata.name)}
                    onRestore={setRestoringBackup}
                  />
                ))}
              </Table.Body>
            </Table.Content>
          </Table.ScrollContainer>
        </Table.Root>
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
// dialog launched from the page header. The query and mutation logic is unchanged;
// the dialog closes once the snapshot starts.
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
  // still pick a different one if more than one exists. Adjusted directly
  // during render (not in an effect) — the condition becomes false as soon
  // as createDest is set, so this can't loop.
  if (!createDest && destinations.length > 0) {
    setCreateDest(destinations[0].name);
  }

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
    <Modal isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <ModalBackdrop isDismissable={!createNow.isPending} isKeyboardDismissDisabled={createNow.isPending} />
      <ModalContainer>
        <ModalDialog>
          <ModalHeader>
            <ModalHeading>Back up now</ModalHeading>
          </ModalHeader>
          <ModalBody className="gap-4">
            <p className="text-sm text-foreground/60">Run a one-off snapshot outside any schedule.</p>
            <div className="space-y-4">
              <div>
                <Select
                  value={createServer}
                  onChange={(v) => setCreateServer(v as string)}
                  placeholder="Select a server…"
                  aria-label="Server"
                  className="mt-1"
                >
                  <Select.Trigger>
                    <Select.Value />
                    <Select.Indicator className="ml-auto h-4 w-4" />
                  </Select.Trigger>
                  <Select.Popover>
                    <ListBox aria-label="Server options">
                      {(serversList?.items ?? []).map((s) => (
                        <ListBoxItem key={s.metadata.name} id={s.metadata.name} textValue={s.metadata.name}>
                          {s.metadata.name}
                        </ListBoxItem>
                      ))}
                    </ListBox>
                  </Select.Popover>
                </Select>
              </div>
              {destinations.length > 1 && (
                <div>
                  <Select
                    value={createDest}
                    onChange={(v) => setCreateDest(v as string)}
                    placeholder="Select a destination…"
                    aria-label="Destination"
                    className="mt-1"
                  >
                    <Select.Trigger>
                      <Select.Value />
                      <Select.Indicator className="ml-auto h-4 w-4" />
                    </Select.Trigger>
                    <Select.Popover>
                      <ListBox aria-label="Destination options">
                        {destinations.map((d) => (
                          <ListBoxItem key={d.name} id={d.name} textValue={d.name}>{d.name}</ListBoxItem>
                        ))}
                      </ListBox>
                    </Select.Popover>
                  </Select>
                </div>
              )}
              {noDestinations && (
                <p className="text-xs text-foreground/60">
                  No backup destinations configured. Add one in{" "}
                  <Link to="/admin" className="text-primary hover:underline">
                    admin settings
                  </Link>{" "}
                  to enable snapshots.
                </p>
              )}
              {createNow.error && <ErrorBanner err={createNow.error} />}
            </div>
          </ModalBody>
          <ModalFooter>
            <Button variant="secondary" onPress={onClose} isDisabled={createNow.isPending}>
              Cancel
            </Button>
            <Button
              variant="primary"
              isDisabled={!createServer || !createDest || createNow.isPending || noDestinations}
              onPress={() => createNow.mutate()}
            >
              {createNow.isPending ? "Starting…" : "Run snapshot"}
            </Button>
          </ModalFooter>
        </ModalDialog>
      </ModalContainer>
    </Modal>
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
          <div className="flex-1">
            <label className="text-xs font-medium">New schedule for</label>
            <Select
              value={creatingFor}
              onChange={(v) => setCreatingFor(v as string)}
              placeholder="Select a server…"
              aria-label="Select a server"
              className="mt-1"
            >
              <Select.Trigger>
                <Select.Value />
                <Select.Indicator className="ml-auto h-4 w-4" />
              </Select.Trigger>
              <Select.Popover>
                <ListBox aria-label="Server options">
                  {(serversList?.items ?? []).map((s) => (
                    <ListBoxItem key={s.metadata.name} id={s.metadata.name} textValue={s.metadata.name}>
                      {s.metadata.name}
                    </ListBoxItem>
                  ))}
                </ListBox>
              </Select.Popover>
            </Select>
          </div>
        </div>
      </Card>

      {creatingFor && (
        <ScheduleForm serverName={creatingFor} onClose={() => setCreatingFor("")} />
      )}
      {toggleSuspend.error && <ErrorBanner err={toggleSuspend.error} />}
      {remove.error && <ErrorBanner err={remove.error} />}

      <div className="rounded-lg border border-border bg-card overflow-hidden">
        <Table.Root>
          <Table.ScrollContainer>
            <Table.Content aria-label="Schedules">
              <Table.Header>
                <Table.Column key="name" isRowHeader>
                  Name
                </Table.Column>
                <Table.Column key="server">Server</Table.Column>
                <Table.Column key="cron">Cron</Table.Column>
                <Table.Column key="lastrun">Last run</Table.Column>
                <Table.Column key="nextrun">Next run</Table.Column>
                <Table.Column key="active">Active</Table.Column>
                <Table.Column key="actions" className="text-end" />
              </Table.Header>
              <Table.Body renderEmptyState={() => <>No schedules configured yet.</>}>
                {items.map((s) => (
                  <Table.Row key={s.metadata.name}>
                    <Table.Cell>
                      <span className="font-mono text-xs">{s.metadata.name}</span>
                    </Table.Cell>
                    <Table.Cell>{s.spec.serverRef.name}</Table.Cell>
                    <Table.Cell>
                      <span className="font-mono text-xs">{s.spec.schedule}</span>
                    </Table.Cell>
                    <Table.Cell className="text-foreground/60">
                      {formatRelative(s.status?.lastSuccessfulTime)}
                    </Table.Cell>
                    <Table.Cell className="text-foreground/60">
                      {s.spec.suspend ? "—" : formatRelative(s.status?.nextScheduleTime)}
                    </Table.Cell>
                    <Table.Cell>
                      <Switch
                        checked={!s.spec.suspend}
                        onCheckedChange={(checked) =>
                          toggleSuspend.mutate({ name: s.metadata.name, suspend: !checked })
                        }
                        aria-label="Schedule active"
                      />
                    </Table.Cell>
                    <Table.Cell>
                      <Button
                        size="sm"
                        variant="ghost"
                        onPress={() => setDeleting(s.metadata.name)}
                      >
                        Delete
                      </Button>
                    </Table.Cell>
                  </Table.Row>
                ))}
              </Table.Body>
            </Table.Content>
          </Table.ScrollContainer>
        </Table.Root>
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
      <div className="rounded-lg border border-border bg-card overflow-hidden">
        <Table.Root>
          <Table.ScrollContainer>
            <Table.Content aria-label="Restores">
              <Table.Header>
                <Table.Column key="name" isRowHeader>
                  Name
                </Table.Column>
                <Table.Column key="backup">Backup</Table.Column>
                <Table.Column key="target">Target</Table.Column>
                <Table.Column key="phase">Phase</Table.Column>
                <Table.Column key="completed">Completed</Table.Column>
                <Table.Column key="message">Message</Table.Column>
              </Table.Header>
              <Table.Body
                renderEmptyState={() => (
                  <>
                    {items.length === 0
                      ? "No restores have been run."
                      : "No restores match the current filters."}
                  </>
                )}
              >
                {filtered.map((r) => (
                  <Table.Row key={r.metadata.name}>
                    <Table.Cell>
                      <span className="font-mono text-xs">{r.metadata.name}</span>
                    </Table.Cell>
                    <Table.Cell>
                      <span className="font-mono text-xs">{r.spec.backupRef.name}</span>
                    </Table.Cell>
                    <Table.Cell>{r.spec.serverRef.name}</Table.Cell>
                    <Table.Cell>
                      <PhaseChip phase={r.status?.phase} />
                    </Table.Cell>
                    <Table.Cell className="text-foreground/60">
                      {formatRelative(r.status?.completionTime)}
                    </Table.Cell>
                    <Table.Cell className="text-xs text-foreground/60">
                      {r.status?.message ?? "—"}
                    </Table.Cell>
                  </Table.Row>
                ))}
              </Table.Body>
            </Table.Content>
          </Table.ScrollContainer>
        </Table.Root>
      </div>
    </div>
  );
}
