import { useQueryClient } from "@tanstack/react-query";
import { getCurrentCluster, setCurrentCluster } from "@/lib/cluster";

/** End the old context before publishing the new one to route/query subscribers. */
export function useClusterSelection() {
  const client = useQueryClient();
  return async (clusterId: string): Promise<void> => {
    if (getCurrentCluster() === clusterId) return;
    await client.cancelQueries();
    client.clear();
    setCurrentCluster(clusterId);
  };
}
