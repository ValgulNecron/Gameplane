import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as Dialog from "@radix-ui/react-dialog";
import { Button } from "@/components/ui/button";
import { Restores, Servers } from "@/lib/endpoints";
import type { Backup } from "@/types";
import { ErrorBanner } from "./ErrorBanner";
import { FieldLabel } from "@/components/ui/field";

interface Props {
  backup: Backup | null;
  defaultServer?: string;
  onClose: () => void;
}

export function RestoreDialog({ backup, defaultServer, onClose }: Props) {
  const qc = useQueryClient();
  const open = backup !== null;
  // Volume-snapshot backups can't be restored in place — they provision a
  // brand-new server seeded from the CSI snapshot. So the dialog collects a
  // NEW (unused) server name instead of an overwrite target.
  const isVolumeSnapshot = backup?.spec.strategy === "volume-snapshot";
  const { data: servers } = useQuery({
    queryKey: ["servers"],
    queryFn: () => Servers.list(),
    enabled: open,
  });
  const [target, setTarget] = useState("");

  // Reset the target whenever a different backup is opened: restic restores
  // default to the in-place server; volume-snapshot restores suggest a fresh
  // "<original>-restored" name.
  useEffect(() => {
    if (!backup) return;
    setTarget(
      backup.spec.strategy === "volume-snapshot"
        ? `${backup.spec.serverRef.name}-restored`
        : (defaultServer ?? ""),
    );
  }, [backup, defaultServer]);

  const nameTaken =
    isVolumeSnapshot &&
    (servers?.items ?? []).some((s) => s.metadata.name === target);
  const invalid = !target || nameTaken;

  const create = useMutation({
    mutationFn: () =>
      Restores.create({
        backupRef: { name: backup!.metadata.name },
        serverRef: { name: target },
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["restores"] });
      onClose();
    },
  });

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-[480px] max-w-[calc(100vw-2rem)] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-border bg-card p-5 text-fg shadow-2xl">
          <Dialog.Title className="text-base font-semibold">
            Restore from backup
          </Dialog.Title>
          <Dialog.Description className="pt-1 text-sm text-muted">
            {isVolumeSnapshot
              ? "A new server will be provisioned from this snapshot. The original server is left untouched."
              : "The target server will be suspended, the volume restored from the snapshot, then resumed."}
          </Dialog.Description>
          <div className="space-y-4 pt-4">
            <FieldLabel label="Source backup">
              <span className="inline-flex items-center rounded-full border border-border bg-surface/40 px-3 py-1 font-mono text-xs">
                {backup?.metadata.name}
              </span>
            </FieldLabel>

            {isVolumeSnapshot ? (
              <FieldLabel label="New server name">
                <input
                  className="h-9 w-full rounded-md border border-border bg-surface px-3 font-mono text-sm"
                  value={target}
                  spellCheck={false}
                  autoComplete="off"
                  placeholder="my-restored-server"
                  onChange={(e) => setTarget(e.target.value)}
                />
                {nameTaken && (
                  <p className="pt-1 text-xs text-danger">
                    A server named “{target}” already exists — choose a new
                    name.
                  </p>
                )}
              </FieldLabel>
            ) : (
              <FieldLabel label="Target game server">
                <select
                  className="h-9 w-full rounded-md border border-border bg-surface px-3 text-sm"
                  value={target}
                  onChange={(e) => setTarget(e.target.value)}
                >
                  <option value="" disabled>
                    Select a server…
                  </option>
                  {(servers?.items ?? []).map((s) => (
                    <option key={s.metadata.name} value={s.metadata.name}>
                      {s.metadata.name}
                    </option>
                  ))}
                </select>
              </FieldLabel>
            )}

            {isVolumeSnapshot ? (
              <div className="rounded-md border border-border bg-surface/40 p-3 text-xs text-muted">
                The new server copies {backup?.spec.serverRef.name}&apos;s
                configuration and starts with the snapshot&apos;s data. Nothing
                on the original server changes.
              </div>
            ) : (
              <div className="rounded-md border border-danger/60 bg-danger/10 p-3 text-xs text-danger">
                This will overwrite all data on the target server. Players will
                be disconnected during the restore.
              </div>
            )}
            {create.error && <ErrorBanner err={create.error} />}
          </div>
          <div className="flex items-center justify-end gap-2 pt-5">
            <Button
              variant="ghost"
              size="sm"
              onClick={onClose}
              disabled={create.isPending}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant={isVolumeSnapshot ? "default" : "danger"}
              onClick={() => create.mutate()}
              disabled={invalid || create.isPending}
            >
              {create.isPending
                ? "Starting…"
                : isVolumeSnapshot
                  ? "Restore to new server"
                  : "Restore"}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
