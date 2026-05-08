import { useEffect, useState, type ReactNode } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

interface ConfirmDialogProps {
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

  useEffect(() => {
    if (open) setTyped("");
  }, [open]);

  const matches = !confirmPhrase || typed === confirmPhrase;

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-[440px] max-w-[calc(100vw-2rem)] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-border bg-card p-5 text-fg shadow-2xl">
          <Dialog.Title className="text-base font-semibold">{title}</Dialog.Title>
          <Dialog.Description asChild>
            <div className="pt-2 text-sm text-muted">{description}</div>
          </Dialog.Description>

          {confirmPhrase && (
            <div className="pt-4">
              <label className="block pb-1 text-xs text-muted">
                Type <span className="font-mono text-fg">{confirmPhrase}</span> to confirm
              </label>
              <Input
                autoFocus
                value={typed}
                onChange={(e) => setTyped(e.target.value)}
                spellCheck={false}
              />
            </div>
          )}

          <div className="flex items-center justify-end gap-2 pt-5">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onOpenChange(false)}
              disabled={busy}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant={destructive ? "danger" : "default"}
              disabled={!matches || busy}
              onClick={onConfirm}
            >
              {busy ? "Working…" : confirmLabel}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
