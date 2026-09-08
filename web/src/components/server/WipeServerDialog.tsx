import { useState, type ReactNode } from "react";
import { useMutation } from "@tanstack/react-query";
import {
  AlertDialog,
  AlertDialogBackdrop,
  AlertDialogContainer,
  AlertDialogDialog,
  AlertDialogHeader,
  AlertDialogHeading,
  AlertDialogBody,
  AlertDialogFooter,
  AlertDialogIcon,
  Button,
  Checkbox,
} from "@heroui/react";
import { AlertCircle } from "lucide-react";
import { Servers } from "@/lib/endpoints";
import { errorText } from "@/lib/errors";

interface Props {
  name: string;
  ns?: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Called after the world data is wiped — e.g. to refresh server state. */
  onWiped?: () => void;
}

// WipeServerDialog suspends the server and asks the operator to wipe its data
// volume. Self-contained (owns its mutation + error) for reuse across the
// Settings danger zone and the server action menus.
export function WipeServerDialog({ name, ns, open, onOpenChange, onWiped }: Props) {
  const [confirmed, setConfirmed] = useState(false);
  // Track previous open state to reset confirmation when dialog opens
  const [prevOpen, setPrevOpen] = useState(open);
  if (open !== prevOpen) {
    setPrevOpen(open);
    if (open) setConfirmed(false);
  }

  const wipe = useMutation({
    // The API's :wipe-data body echoes the server name as a typed confirmation.
    mutationFn: () => Servers.wipeData(name, name, ns),
    onSuccess: () => {
      onOpenChange(false);
      onWiped?.();
    },
  });

  const description: ReactNode = (
    <div className="space-y-4">
      <div className="text-sm text-muted">
        This permanently deletes the world data for{" "}
        <span className="font-mono text-foreground">{name}</span>. Backups are not
        removed. This cannot be undone.
      </div>
      {wipe.isError && (
        <div className="rounded-lg bg-danger/10 p-3 text-sm text-danger">
          {errorText(wipe.error, "wipe failed")}
        </div>
      )}
    </div>
  );

  return (
    <AlertDialog isOpen={open} onOpenChange={(v) => {
      if (!v) wipe.reset();
      onOpenChange(v);
    }}>
      <AlertDialogBackdrop isDismissable={!wipe.isPending} isKeyboardDismissDisabled={wipe.isPending}>
        <AlertDialogContainer>
          <AlertDialogDialog>
            <AlertDialogHeader>
              <div className="flex items-start gap-4">
                <AlertDialogIcon
                  status="danger"
                  className="shrink-0"
                >
                  <AlertCircle className="h-5 w-5" />
                </AlertDialogIcon>
                <AlertDialogHeading>Wipe world?</AlertDialogHeading>
              </div>
            </AlertDialogHeader>

            <AlertDialogBody>
              <div className="space-y-4">
                {description}

                <Checkbox
                  isSelected={confirmed}
                  onChange={() => setConfirmed(!confirmed)}
                >
                  <Checkbox.Control />
                  <Checkbox.Content className="text-sm text-foreground">
                    I understand this will permanently delete the world data
                  </Checkbox.Content>
                </Checkbox>
              </div>
            </AlertDialogBody>

            <AlertDialogFooter className="flex items-center justify-end gap-2">
              <Button
                variant="secondary"
                size="sm"
                isDisabled={wipe.isPending}
                onPress={() => onOpenChange(false)}
              >
                Cancel
              </Button>
              <Button
                size="sm"
                variant="danger"
                isDisabled={!confirmed || wipe.isPending}
                onPress={() => wipe.mutate()}
              >
                {wipe.isPending ? "Working…" : "Wipe world"}
              </Button>
            </AlertDialogFooter>
          </AlertDialogDialog>
        </AlertDialogContainer>
      </AlertDialogBackdrop>
    </AlertDialog>
  );
}
