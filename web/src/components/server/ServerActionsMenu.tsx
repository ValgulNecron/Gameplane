import { useState } from "react";
import {
  ArrowRightLeft,
  Copy,
  Eraser,
  MoreHorizontal,
  Trash2,
} from "lucide-react";
import type { GameServer } from "@/types";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useMe, can } from "@/lib/auth";
import { CloneServerDialog } from "./CloneServerDialog";
import { DeleteServerDialog } from "./DeleteServerDialog";
import { TransferServerDialog } from "./TransferServerDialog";
import { WipeServerDialog } from "./WipeServerDialog";

const OWNER_ID_ANNOTATION = "gameplane.local/owner-id";

interface Props {
  gs: GameServer;
  onDeleted?: () => void;
  onTransferred?: () => void;
}

export function ServerActionsMenu({ gs, onDeleted, onTransferred }: Props) {
  const { data: me } = useMe();
  const ns = gs.metadata.namespace;
  const ann = gs.metadata.annotations ?? {};
  const ownerID = ann[OWNER_ID_ANNOTATION];

  const canClone = can(me, "servers:write", ns);
  const canManage =
    ownerID === String(me?.id) || can(me, "servers:write", ns ?? "gameplane-games");

  const [cloneOpen, setCloneOpen] = useState(false);
  const [transferOpen, setTransferOpen] = useState(false);
  const [wipeOpen, setWipeOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon" aria-label="Server actions">
            <MoreHorizontal className="h-4 w-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem
            icon={<Copy className="h-4 w-4" />}
            label="Clone server"
            onSelect={() => setCloneOpen(true)}
            disabled={!canClone}
            hint={canClone ? undefined : "Requires operator role"}
          />
          <DropdownMenuItem
            icon={<ArrowRightLeft className="h-4 w-4" />}
            label="Transfer ownership"
            onSelect={() => setTransferOpen(true)}
            disabled={!canManage}
            hint={canManage ? undefined : "Requires owner or operator role"}
          />
          <DropdownMenuItem
            icon={<Eraser className="h-4 w-4" />}
            label="Wipe world data"
            onSelect={() => setWipeOpen(true)}
            disabled={!canManage}
            hint={canManage ? undefined : "Requires owner or operator role"}
            destructive
          />
          <DropdownMenuSeparator />
          <DropdownMenuItem
            icon={<Trash2 className="h-4 w-4" />}
            label="Delete server"
            onSelect={() => setDeleteOpen(true)}
            disabled={!canManage}
            destructive
          />
        </DropdownMenuContent>
      </DropdownMenu>

      <CloneServerDialog
        open={cloneOpen}
        onOpenChange={setCloneOpen}
        sourceName={gs.metadata.name}
        ns={ns}
      />
      <TransferServerDialog
        name={gs.metadata.name}
        ns={ns}
        open={transferOpen}
        onOpenChange={setTransferOpen}
        onTransferred={onTransferred}
      />
      <WipeServerDialog
        name={gs.metadata.name}
        ns={ns}
        open={wipeOpen}
        onOpenChange={setWipeOpen}
      />
      <DeleteServerDialog
        name={gs.metadata.name}
        ns={ns}
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        onDeleted={onDeleted}
      />
    </>
  );
}
