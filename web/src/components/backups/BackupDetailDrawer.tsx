import type { ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Drawer, Button } from "@heroui/react";
import { RotateCcw, Trash2 } from "lucide-react";
import { Backups } from "@/lib/endpoints";
import { formatRelative } from "@/lib/utils";
import type { Backup } from "@/types";
import { PhaseChip } from "@/components/ui/PhaseChip";
import { ErrorBanner } from "./ErrorBanner";

interface Props {
  name: string | null;
  ns?: string;
  onClose: () => void;
  onRestore: (backup: Backup) => void;
}

export function BackupDetailDrawer({ name, ns, onClose, onRestore }: Props) {
  const qc = useQueryClient();
  const open = name !== null;
  const { data: backup, error } = useQuery({
    queryKey: ["backup", name, ns],
    queryFn: () => Backups.get(name!, ns),
    enabled: open,
    refetchInterval: 5000,
  });
  const remove = useMutation({
    mutationFn: () => Backups.remove(name!, ns),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["backups", ns] });
      onClose();
    },
  });

  const restorable =
    backup?.status?.phase === "Succeeded" && Boolean(backup.status.snapshotID);

  return (
    <Drawer
      isOpen={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <Drawer.Backdrop>
        <Drawer.Content placement="right" className="me-4">
          <Drawer.Dialog className="flex flex-col w-[440px] h-[760px] p-0">
            <Drawer.Header className="shrink-0 border-b border-border p-5 flex flex-row items-center justify-between gap-3">
              <div className="space-y-1">
                <Drawer.Heading className="text-base font-semibold">Backup details</Drawer.Heading>
                <div className="font-mono text-xs text-muted">{name}</div>
              </div>
              <Button
                variant="ghost"
                size="sm"
                isDisabled={!restorable}
                onPress={() => backup && onRestore(backup)}
                className="gap-1.5 shrink-0"
                aria-label="Restore backup"
              >
                <RotateCcw className="h-4 w-4" />
                Restore
              </Button>
            </Drawer.Header>

            <Drawer.Body className="min-h-0 space-y-4 overflow-y-auto p-5 text-sm">
              {error && <ErrorBanner err={error} />}
              {backup && (
                <>
                  <Field label="Phase">
                    <PhaseChip phase={backup.status?.phase} />
                  </Field>
                  <Field label="Server">{backup.spec.serverRef.name}</Field>
                  <Field label="Snapshot ID">
                    <span className="font-mono text-xs">{backup.status?.snapshotID ?? "—"}</span>
                  </Field>
                  <Field label="Size">{backup.status?.size ?? "—"}</Field>
                  <Field label="Started">
                    {formatRelative(backup.status?.startTime)}
                    {backup.status?.startTime && (
                      <span className="pl-2 font-mono text-xs text-muted">
                        {backup.status.startTime}
                      </span>
                    )}
                  </Field>
                  <Field label="Completed">
                    {formatRelative(backup.status?.completionTime)}
                    {backup.status?.completionTime && (
                      <span className="pl-2 font-mono text-xs text-muted">
                        {backup.status.completionTime}
                      </span>
                    )}
                  </Field>
                </>
              )}
              {remove.error && <ErrorBanner err={remove.error} />}
            </Drawer.Body>

            <Drawer.Footer className="shrink-0 flex items-center justify-end gap-2 border-t border-border p-4">
              <Button
                variant="danger"
                size="sm"
                isDisabled={!backup || remove.isPending}
                onPress={() => remove.mutate()}
              >
                <Trash2 className="h-3.5 w-3.5" />
                {remove.isPending ? "Deleting…" : "Delete"}
              </Button>
              <Button
                variant="primary"
                size="sm"
                isDisabled={!restorable}
                onPress={() => backup && onRestore(backup)}
              >
                Restore
              </Button>
            </Drawer.Footer>
          </Drawer.Dialog>
        </Drawer.Content>
      </Drawer.Backdrop>
    </Drawer>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="space-y-1">
      <div className="text-xs uppercase tracking-wide text-muted">{label}</div>
      <div className="text-sm">{children}</div>
    </div>
  );
}
