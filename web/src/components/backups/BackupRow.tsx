import { Button } from "@/components/ui/button";
import { formatRelative } from "@/lib/utils";
import type { Backup } from "@/types";
import { PhaseBadge } from "./PhaseBadge";

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
    <tr
      className="cursor-pointer hover:bg-surface/40"
      onClick={() => onSelect(backup)}
    >
      <td className="px-4 py-3 font-mono text-xs">{backup.metadata.name}</td>
      {showServer && (
        <td className="px-4 py-3 text-muted">{backup.spec.serverRef.name}</td>
      )}
      <td className="px-4 py-3">
        <PhaseBadge phase={backup.status?.phase} />
      </td>
      <td className="px-4 py-3 font-mono">{backup.status?.size ?? "—"}</td>
      <td className="px-4 py-3 text-muted">
        {formatRelative(backup.status?.completionTime)}
      </td>
      <td
        className="px-4 py-3 text-right"
        onClick={(e) => e.stopPropagation()}
      >
        <Button
          size="sm"
          variant="outline"
          disabled={!restorable}
          onClick={() => onRestore(backup)}
        >
          Restore
        </Button>
      </td>
    </tr>
  );
}
