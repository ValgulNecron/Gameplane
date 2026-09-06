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
  ListBox,
  ListBoxItem,
  Popover,
  PopoverTrigger,
  PopoverContent,
} from "@heroui/react";
import { ChevronDown } from "lucide-react";
import { Servers, Users } from "@/lib/endpoints";
import { errorText } from "@/lib/errors";
import { OWNER_ANNOTATION } from "@/lib/annotations";
import { cn } from "@/lib/utils";

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
  const [popoverOpen, setPopoverOpen] = useState(false);
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

  const selectedUser = users.find((u) => String(u.id) === userId);

  return (
    <Modal isOpen={open} onOpenChange={onOpenChange}>
      <ModalBackdrop isDismissable={!transfer.isPending} isKeyboardDismissDisabled={transfer.isPending} />
      <ModalContainer>
        <ModalDialog>
          <ModalHeader>
            <ModalHeading>Transfer {name}</ModalHeading>
          </ModalHeader>

          <ModalBody>
            <div className="space-y-4">
              <div className="text-sm text-muted">
                Current owner: {currentOwner || "unassigned"}.
              </div>

              {usersError ? (
                <div className="text-xs text-danger">
                  You need permission to list users to pick a recipient.
                </div>
              ) : (
                <Popover isOpen={popoverOpen} onOpenChange={setPopoverOpen}>
                  <PopoverTrigger
                    className={cn(
                      "flex items-center justify-between gap-2 rounded-lg border border-border bg-card px-3 py-2 text-sm",
                      "hover:bg-surface transition-colors cursor-pointer",
                    )}
                  >
                    <span className={selectedUser ? "text-fg" : "text-muted"}>
                      {selectedUser?.username || "Select a user…"}
                    </span>
                    <ChevronDown className="h-4 w-4 text-muted shrink-0" />
                  </PopoverTrigger>
                  <PopoverContent className="min-w-[200px]">
                    <ListBox
                      aria-label="Transfer to"
                      onSelectionChange={(selected) => {
                        setUserId(String(selected));
                        setPopoverOpen(false);
                      }}
                    >
                      {users.map((u) => (
                        <ListBoxItem key={u.id} id={String(u.id)}>
                          {u.username}
                        </ListBoxItem>
                      ))}
                    </ListBox>
                  </PopoverContent>
                </Popover>
              )}

              {transfer.isError && (
                <div className="text-xs text-danger">
                  {errorText(transfer.error, "transfer failed")}
                </div>
              )}
            </div>
          </ModalBody>

          <ModalFooter className="flex items-center justify-end gap-2">
            <Button
              variant="secondary"
              size="sm"
              isDisabled={transfer.isPending}
              onPress={() => onOpenChange(false)}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              isDisabled={!userId || transfer.isPending}
              onPress={() => transfer.mutate()}
            >
              {transfer.isPending ? "Transferring…" : "Transfer"}
            </Button>
          </ModalFooter>
        </ModalDialog>
      </ModalContainer>
    </Modal>
  );
}
