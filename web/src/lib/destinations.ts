import { useQuery } from "@tanstack/react-query";
import { useResourceClient, useResourceTarget, resourceKey, type ResourceScope } from "./resourceTarget";
import type { BackupDestination } from "@/types";

export function useBackupDestinations(scope?: ResourceScope) {
  const target = useResourceTarget({ name: "" }, scope ? { ...scope, name: "" } : undefined);
  const client = useResourceClient(target);
  return useQuery({
    queryKey: resourceKey(target, "backup-destinations"),
    queryFn: ({ signal }) => client.withSignal(signal).BackupDestinations.list(),
    select: (resp): BackupDestination[] => resp.items,
  });
}
