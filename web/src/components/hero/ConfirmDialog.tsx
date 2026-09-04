import { useState, type ReactNode } from "react";
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
} from "@heroui/react";
import { Button, Input } from "@heroui/react";
import { AlertCircle } from "lucide-react";

export interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: ReactNode;
  /** When set, user must type this value exactly to enable the confirm button. */
  confirmPhrase?: string;
  confirmLabel?: string;
  destructive?: boolean;
  onConfirm: () => void;
  busy?: boolean;
}

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmPhrase,
  confirmLabel = "Confirm",
  destructive,
  onConfirm,
  busy,
}: ConfirmDialogProps) {
  const [typed, setTyped] = useState("");
  // Clear the typed confirmation whenever the dialog transitions to open.
  // Adjusted directly during render (not in an effect) — the React-endorsed
  // pattern for resetting state on a prop change, gated on the previous
  // `open` value tracked in state.
  const [prevOpen, setPrevOpen] = useState(open);
  if (open !== prevOpen) {
    setPrevOpen(open);
    if (open) setTyped("");
  }

  const matches = !confirmPhrase || typed === confirmPhrase;

  return (
    <AlertDialog isOpen={open} onOpenChange={onOpenChange}>
      <AlertDialogBackdrop isDismissable={!busy} isKeyboardDismissDisabled={busy} />
      <AlertDialogContainer>
        <AlertDialogDialog>
          <AlertDialogHeader>
            <div className="flex items-start gap-4">
              <AlertDialogIcon
                status={destructive ? "danger" : "accent"}
                className="shrink-0"
              >
                <AlertCircle className="h-5 w-5" />
              </AlertDialogIcon>
              <AlertDialogHeading>{title}</AlertDialogHeading>
            </div>
          </AlertDialogHeader>

          <AlertDialogBody>
            <div className="space-y-4">
              <div className="text-sm text-muted">{description}</div>

              {confirmPhrase && (
                <div>
                  <label className="block pb-2 text-xs text-muted">
                    Type <span className="font-mono text-foreground">{confirmPhrase}</span> to
                    confirm
                  </label>
                  <Input
                    autoFocus
                    value={typed}
                    onChange={(e) => setTyped(e.target.value)}
                    spellCheck={false}
                    placeholder="Type to confirm"
                  />
                </div>
              )}
            </div>
          </AlertDialogBody>

          <AlertDialogFooter className="flex items-center justify-end gap-2">
            <Button
              variant="secondary"
              size="sm"
              isDisabled={busy}
              onPress={() => onOpenChange(false)}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant={destructive ? "danger" : "primary"}
              isDisabled={!matches || busy}
              onPress={onConfirm}
            >
              {busy ? "Working…" : confirmLabel}
            </Button>
          </AlertDialogFooter>
        </AlertDialogDialog>
      </AlertDialogContainer>
    </AlertDialog>
  );
}
