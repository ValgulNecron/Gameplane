import type { ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Drawer, Button } from "@heroui/react";
import { Backups } from "@/lib/endpoints";
import { formatRelative } from "@/lib/utils";
import type { Backup } from "@/types";
import { PhaseChip } from "@/components/hero/PhaseChip";
import { ErrorBanner } from "./ErrorBanner";

interface Props {
  name: string | null;
  onClose: () => void;
  onRestore: (backup: Backup) => void;
}

export function BackupDetailDrawer({ name, onClose, onRestore }: Props) {
  const qc = useQueryClient();
  const open = name !== null;
  const { data: backup, error } = useQuery({
    queryKey: ["backup", name],
    queryFn: () => Backups.get(name!),
    enabled: open,
    refetchInterval: 5000,
  });
  const remove = useMutation({
    mutationFn: () => Backups.remove(name!),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["backups"] });
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
      <Drawer.Backdrop />
      <Drawer.Content placement="right" className="w-[440px]">
        <Drawer.Dialog className="flex flex-col h-full">
          <Drawer.Header className="flex items-start justify-between border-b border-border p-5">
            <div className="space-y-1">
              <h2 className="text-base font-semibold">Backup details</h2>
              <div className="font-mono text-xs text-muted">{name}</div>
            </div>
            <Button
              isIconOnly
              variant="ghost"
              size="sm"
              onPress={onClose}
              aria-label="Close backup details"
            >
              ✕
            </Button>
          </Drawer.Header>

          <Drawer.Body className="flex-1 space-y-4 overflow-y-auto p-5 text-sm">
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

          <Drawer.Footer className="flex items-center justify-end gap-2 border-t border-border p-4">
            <Button
              variant="danger"
              size="sm"
              isDisabled={!backup || remove.isPending}
              onPress={() => remove.mutate()}
            >
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
