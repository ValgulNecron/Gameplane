import type { ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as Dialog from "@radix-ui/react-dialog";
import { Button } from "@/components/ui/button";
import { Backups } from "@/lib/endpoints";
import { formatRelative } from "@/lib/utils";
import type { Backup } from "@/types";
import { PhaseBadge } from "./PhaseBadge";
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
    <Dialog.Root open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/40" />
        <Dialog.Content
          className="fixed right-0 top-0 z-50 flex h-full w-[440px] max-w-full flex-col border-l border-border bg-card text-fg shadow-2xl"
        >
          <header className="flex items-start justify-between border-b border-border p-5">
            <div className="space-y-1">
              <Dialog.Title className="text-base font-semibold">Backup details</Dialog.Title>
              <Dialog.Description asChild>
                <div className="font-mono text-xs text-muted">{name}</div>
              </Dialog.Description>
            </div>
            <Button variant="ghost" size="sm" onClick={onClose}>Close</Button>
          </header>

          <div className="flex-1 space-y-4 overflow-y-auto p-5 text-sm">
            {error && <ErrorBanner err={error} />}
            {backup && (
              <>
                <Field label="Phase">
                  <PhaseBadge phase={backup.status?.phase} />
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
          </div>

          <footer className="flex items-center justify-end gap-2 border-t border-border p-4">
            <Button
              variant="danger"
              size="sm"
              disabled={!backup || remove.isPending}
              onClick={() => remove.mutate()}
            >
              {remove.isPending ? "Deleting…" : "Delete"}
            </Button>
            <Button
              size="sm"
              disabled={!restorable}
              onClick={() => backup && onRestore(backup)}
            >
              Restore
            </Button>
          </footer>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
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
