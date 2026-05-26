import { useQuery } from "@tanstack/react-query";
import { BackupDestinations } from "@/lib/endpoints";
import type { BackupDestination } from "@/types";

// useBackupDestinations is the single read path for backup destinations.
// Several places need to know whether 0/1/many destinations exist before
// they can wire a "Run snapshot" / "Schedule" button. Sharing one query
// key keeps that state coherent across pages.
export function useBackupDestinations() {
  return useQuery({
    queryKey: ["backup-destinations"],
    queryFn: () => BackupDestinations.list(),
    select: (resp): BackupDestination[] => resp.items,
  });
}
