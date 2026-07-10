import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import * as Dialog from "@radix-ui/react-dialog";
import { Servers } from "@/lib/endpoints";
import { APIError } from "@/lib/api";
import { isValidK8sName } from "@/lib/validation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  sourceName: string;
  ns?: string;
  onCloned?: () => void;
}

function cloneErrorMessage(err: unknown, name: string): string {
  if (err instanceof APIError) {
    if (err.status === 409) return `A server named ${name} already exists.`;
    if (err.status === 403) return "Your role does not allow cloning servers.";
    return err.body.slice(0, 240) || `Clone failed (${err.status}).`;
  }
  return err instanceof Error ? err.message : "Unknown error";
}

export function CloneServerDialog({
  open,
  onOpenChange,
  sourceName,
  ns,
  onCloned,
}: Props) {
  const qc = useQueryClient();
  const nav = useNavigate();
  const [newName, setNewName] = useState("");

  useEffect(() => {
    if (open) setNewName(`${sourceName.slice(0, 58)}-copy`);
  }, [open, sourceName]);

  const clone = useMutation({
    mutationFn: () => Servers.clone(sourceName, newName, ns),
    onSuccess: async (created) => {
      await qc.invalidateQueries({ queryKey: ["servers"] });
      await qc.invalidateQueries({ queryKey: ["my-servers"] });
      onOpenChange(false);
      onCloned?.();
      await nav({ to: "/servers/$name", params: { name: created.metadata.name } });
    },
  });

  const valid = isValidK8sName(newName);

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-[440px] max-w-[calc(100vw-2rem)] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-border bg-card p-5 text-fg shadow-2xl">
          <Dialog.Title className="text-base font-semibold">Clone server</Dialog.Title>
          <Dialog.Description asChild>
            <div className="pt-2 text-sm text-muted">
              Creates a new server with the same configuration. World data is not copied.
            </div>
          </Dialog.Description>

          <div className="pt-4">
            <label className="block pb-1 text-xs text-muted" htmlFor="clone-new-name">
              New name
            </label>
            <Input
              id="clone-new-name"
              autoFocus
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              spellCheck={false}
            />
            {!valid && (
              <p className="pt-1 text-xs text-danger">
                Name must be lowercase letters, digits, dashes (max 63)
              </p>
            )}
            {clone.isError && (
              <p className="pt-1 text-xs text-danger">
                {cloneErrorMessage(clone.error, newName)}
              </p>
            )}
          </div>

          <div className="flex items-center justify-end gap-2 pt-5">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onOpenChange(false)}
              disabled={clone.isPending}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              disabled={!valid || clone.isPending}
              onClick={() => clone.mutate()}
            >
              {clone.isPending ? "Cloning…" : "Clone server"}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
