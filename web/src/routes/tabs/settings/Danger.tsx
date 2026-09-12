import { useState, type ReactNode } from "react";
import { useNavigate } from "@tanstack/react-router";
import { Button, Card, CardContent } from "@heroui/react";
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
        body="Deletes everything on the data volume and restarts the server on a fresh world. Take a backup first."
        action={
          <Button variant="outline" size="sm" onPress={() => setWipeOpen(true)}>
            Wipe world…
          </Button>
        }
      />
      <Row
        title="Transfer ownership"
        body="Hand this server to another user. You keep access only if you stay a collaborator."
        action={
          <Button variant="outline" size="sm" onPress={() => setTransferOpen(true)}>
            Transfer…
          </Button>
        }
      />
      <Row
        title="Delete server"
        body="Removes the GameServer and its persistent volume. This cannot be undone."
        action={
          <Button variant="danger" size="sm" onPress={() => setConfirmOpen(true)}>
            Delete
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
    <Card>
      <CardContent className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="text-sm font-medium">{title}</div>
          <div className="pt-1 text-xs text-default-500">{body}</div>
        </div>
        {action}
      </CardContent>
    </Card>
  );
}
