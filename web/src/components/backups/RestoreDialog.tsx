import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Modal,
  ModalBackdrop,
  ModalContainer,
  ModalDialog,
  ModalHeader,
  ModalHeading,
  ModalBody,
  ModalFooter,
  Button,
  Input,
  Label,
  Description,
  ListBox,
  ListBoxItem,
  Popover,
  PopoverTrigger,
  PopoverContent,
} from "@heroui/react";
import { ChevronDown } from "lucide-react";
import { Restores, Servers } from "@/lib/endpoints";
import { errorText } from "@/lib/errors";
import type { Backup } from "@/types";
import { cn } from "@/lib/utils";

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
  const [popoverOpen, setPopoverOpen] = useState(false);
  // Tracks which backup `target` was last initialized for, so the reset
  // below (adjusted directly during render, not in an effect) fires exactly
  // once per newly-opened backup: restic restores default to the in-place
  // server; volume-snapshot restores suggest a fresh "<original>-restored"
  // name.
  const [initializedFor, setInitializedFor] = useState<Backup | null>(null);
  if (backup && backup !== initializedFor) {
    setInitializedFor(backup);
    setTarget(
      backup.spec.strategy === "volume-snapshot"
        ? `${backup.spec.serverRef.name}-restored`
        : (defaultServer ?? ""),
    );
  }

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

  const selectedServer = servers?.items?.find((s) => s.metadata.name === target);

  return (
    <Modal isOpen={open} onOpenChange={(o) => !o && onClose()}>
      <ModalBackdrop isDismissable={!create.isPending} isKeyboardDismissDisabled={create.isPending} />
      <ModalContainer>
        <ModalDialog>
          <ModalHeader>
            <ModalHeading>Restore backup</ModalHeading>
          </ModalHeader>

          <ModalBody className="gap-4">
            <Description className="text-sm text-muted">
              {isVolumeSnapshot
                ? "A new server will be provisioned from this snapshot. The original server is left untouched."
                : "The target server will be suspended, the volume restored from the snapshot, then resumed."}
            </Description>

            <div className="space-y-4">
              <div>
                <Label htmlFor="source-backup" className="text-xs">
                  Source backup
                </Label>
                <div className="mt-1 flex items-center rounded-lg border border-border bg-surface/40 px-3 py-2 font-mono text-xs text-fg">
                  {backup?.metadata.name}
                </div>
              </div>

              {isVolumeSnapshot ? (
                <div>
                  <Label htmlFor="new-server-name" className="text-xs">
                    New server name
                  </Label>
                  <Input
                    id="new-server-name"
                    autoFocus
                    value={target}
                    onChange={(e) => setTarget(e.target.value)}
                    spellCheck={false}
                    autoComplete="off"
                    placeholder="my-restored-server"
                    className="mt-1"
                  />
                  {nameTaken && (
                    <p role="alert" className="mt-2 text-xs text-danger">
                      A server named "{target}" already exists — choose a new name.
                    </p>
                  )}
                </div>
              ) : (
                <div>
                  <Label htmlFor="target-server" className="text-xs">
                    Target game server
                  </Label>
                  <Popover isOpen={popoverOpen} onOpenChange={setPopoverOpen}>
                    <PopoverTrigger
                      id="target-server"
                      className={cn(
                        "mt-1 flex items-center justify-between gap-2 rounded-lg border border-border bg-card px-3 py-2 text-sm",
                        "hover:bg-surface transition-colors cursor-pointer",
                      )}
                    >
                      <span className={target ? "text-fg" : "text-muted"}>
                        {selectedServer?.metadata.name || "Select a server…"}
                      </span>
                      <ChevronDown className="h-4 w-4 text-muted shrink-0" />
                    </PopoverTrigger>
                    <PopoverContent className="min-w-[200px]">
                      <ListBox
                        aria-label="Select target server"
                        onSelectionChange={(selected) => {
                          setTarget(String(selected));
                          setPopoverOpen(false);
                        }}
                      >
                        {(servers?.items ?? []).map((s) => (
                          <ListBoxItem key={s.metadata.name} id={s.metadata.name}>
                            {s.metadata.name}
                          </ListBoxItem>
                        ))}
                      </ListBox>
                    </PopoverContent>
                  </Popover>
                </div>
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

              {create.isError && (
                <div className="rounded-md border border-danger/60 bg-danger/10 p-2 text-xs text-danger">
                  {errorText(create.error, "Restore failed")}
                </div>
              )}
            </div>
          </ModalBody>

          <ModalFooter className="flex items-center justify-end gap-2">
            <Button
              variant="secondary"
              size="sm"
              onPress={onClose}
              isDisabled={create.isPending}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant={isVolumeSnapshot ? "primary" : "danger"}
              isDisabled={invalid || create.isPending}
              onPress={() => create.mutate()}
            >
              {create.isPending
                ? "Starting…"
                : isVolumeSnapshot
                  ? "Restore to new server"
                  : "Restore"}
            </Button>
          </ModalFooter>
        </ModalDialog>
      </ModalContainer>
    </Modal>
  );
}
