import { useState, type ReactNode } from "react";
import { useNavigate } from "@tanstack/react-router";
import { Button } from "@/components/ui/button";
import { DeleteServerDialog } from "@/components/server/DeleteServerDialog";
import { WipeServerDialog } from "@/components/server/WipeServerDialog";
import { TransferServerDialog } from "@/components/server/TransferServerDialog";

interface Props {
  name: string;
  ns?: string;
}

export function DangerSection({ name, ns }: Props) {
  const navigate = useNavigate();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [wipeOpen, setWipeOpen] = useState(false);
  const [transferOpen, setTransferOpen] = useState(false);

  return (
    <div className="space-y-3">
      <Row
        title="Wipe world data"
        body="Suspends the server and deletes the contents of its persistent volume, then you can restart into a fresh install. Keeps the GameServer."
        action={
          <Button variant="outline" size="sm" onClick={() => setWipeOpen(true)}>
            Wipe world…
          </Button>
        }
      />
      <Row
        title="Transfer ownership"
        body="Hands the server over to another user. Ownership is informational."
        action={
          <Button variant="outline" size="sm" onClick={() => setTransferOpen(true)}>
            Transfer…
          </Button>
        }
      />
      <Row
        title="Delete server"
        body="Removes the GameServer resource and its persistent volume. This cannot be undone."
        action={
          <Button variant="danger" size="sm" onClick={() => setConfirmOpen(true)}>
            Delete server…
          </Button>
        }
      />
      <WipeServerDialog name={name} ns={ns} open={wipeOpen} onOpenChange={setWipeOpen} />
      <TransferServerDialog
        name={name}
        ns={ns}
        open={transferOpen}
        onOpenChange={setTransferOpen}
      />
      <DeleteServerDialog
        name={name}
        ns={ns}
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        onDeleted={() => navigate({ to: "/servers" })}
      />
    </div>
  );
}

function Row({
  title,
  body,
  action,
}: {
  title: string;
  body: string;
  action: ReactNode;
}) {
  return (
    <div className="flex items-start justify-between gap-4 rounded border border-border bg-surface/30 p-4">
      <div className="min-w-0">
        <div className="text-sm font-medium text-fg">{title}</div>
        <div className="pt-1 text-xs text-muted">{body}</div>
      </div>
      {action}
    </div>
  );
}
