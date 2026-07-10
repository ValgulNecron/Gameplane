import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as Dialog from "@radix-ui/react-dialog";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { Servers, Users } from "@/lib/endpoints";
import { errorText } from "@/lib/errors";
import { OWNER_ANNOTATION } from "@/lib/annotations";

interface Props {
  name: string;
  ns?: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Called after ownership is reassigned. */
  onTransferred?: () => void;
}

// TransferServerDialog reassigns a server's (informational) owner to another
// user. Self-contained for reuse across the Settings danger zone and the
// server action menus.
export function TransferServerDialog({ name, ns, open, onOpenChange, onTransferred }: Props) {
  const qc = useQueryClient();
  const [userId, setUserId] = useState("");
  const { data: server } = useQuery({
    queryKey: ["server", name, ns],
    queryFn: () => Servers.get(name, ns),
    enabled: open,
  });
  const { data: users = [], error: usersError } = useQuery({
    queryKey: ["users"],
    queryFn: () => Users.list(),
    enabled: open,
  });
  const currentOwner = server?.metadata.annotations?.[OWNER_ANNOTATION];

  const transfer = useMutation({
    mutationFn: () => Servers.transfer(name, Number(userId), ns),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["server", name, ns] });
      onOpenChange(false);
      onTransferred?.();
    },
  });

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-[440px] max-w-[calc(100vw-2rem)] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-border bg-card p-5 text-fg shadow-2xl">
          <Dialog.Title className="text-base font-semibold">Transfer {name}</Dialog.Title>
          <Dialog.Description asChild>
            <div className="pt-1 text-sm text-muted">
              Current owner: {currentOwner || "unassigned"}.
            </div>
          </Dialog.Description>
          <div className="space-y-3 pt-4">
            {usersError ? (
              <div className="text-xs text-danger">
                You need permission to list users to pick a recipient.
              </div>
            ) : (
              <Select
                value={userId}
                onValueChange={setUserId}
                options={[
                  { value: "", label: "Select a user…" },
                  ...users.map((u) => ({ value: String(u.id), label: u.username })),
                ]}
              />
            )}
            {transfer.isError && (
              <div className="text-xs text-danger">
                {errorText(transfer.error, "transfer failed")}
              </div>
            )}
            <div className="flex justify-end gap-2 pt-1">
              <Button variant="ghost" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button disabled={!userId || transfer.isPending} onClick={() => transfer.mutate()}>
                {transfer.isPending ? "Transferring…" : "Transfer"}
              </Button>
            </div>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
