import { Button, Table } from "@heroui/react";
import { formatRelative } from "@/lib/utils";
import type { Backup } from "@/types";
import { PhaseChip } from "@/components/hero/PhaseChip";

interface Props {
  backup: Backup;
  showServer: boolean;
  onSelect: (b: Backup) => void;
  onRestore: (b: Backup) => void;
}

export function BackupRow({ backup, showServer, onSelect, onRestore }: Props) {
  const restorable =
    backup.status?.phase === "Succeeded" && Boolean(backup.status.snapshotID);
  return (
    <Table.Row
      className="cursor-pointer"
      onClick={() => onSelect(backup)}
    >
      <Table.Cell className="font-mono text-xs">{backup.metadata.name}</Table.Cell>
      <Table.Cell>
        {showServer && backup.spec.serverRef.name}
      </Table.Cell>
      <Table.Cell>
        <PhaseChip phase={backup.status?.phase} />
      </Table.Cell>
      <Table.Cell className="font-mono">{backup.status?.size ?? "—"}</Table.Cell>
      <Table.Cell>
        {formatRelative(backup.status?.completionTime)}
      </Table.Cell>
      <Table.Cell
        className="text-right"
        onClick={(e) => e.stopPropagation()}
      >
        <Button
          size="sm"
          variant="outline"
          isDisabled={!restorable}
          onPress={() => onRestore(backup)}
        >
          Restore
        </Button>
      </Table.Cell>
    </Table.Row>
  );
}
