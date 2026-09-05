import { useQuery } from "@tanstack/react-query";
import { Card, CardContent, Spinner } from "@heroui/react";

import { PageHeader } from "@/components/PageHeader";
import { useMe, can } from "@/lib/auth";
import { Audit, Backups, Cluster, Servers } from "@/lib/endpoints";
import type { Backup, ClusterStats, ClusterView } from "@/types";

export function DashboardPage() {
  const { data: me } = useMe();
  const canAudit = can(me, "audit:read");

  const { isLoading: serversLoading } = useQuery({
    queryKey: ["servers"],
    queryFn: () => Servers.list(),
    refetchInterval: 5_000,
  });
  const { isLoading: statsLoading } = useQuery({
    queryKey: ["cluster-stats"],
    queryFn: () => Cluster.stats().catch(() => ({}) as ClusterStats),
    staleTime: 30_000,
  });
  const { isLoading: clusterLoading } = useQuery({
    queryKey: ["cluster"],
    queryFn: () => Cluster.view().catch(() => ({}) as ClusterView),
    staleTime: 30_000,
  });
  const { isLoading: backupsLoading } = useQuery({
    queryKey: ["backups"],
    queryFn: () => Backups.list().catch(() => ({ items: [] as Backup[] })),
    staleTime: 30_000,
  });
  const { isLoading: auditLoading } = useQuery({
    queryKey: ["audit", "dashboard"],
    queryFn: () => Audit.page(8, 0),
    enabled: canAudit,
    staleTime: 30_000,
  });

  const isLoading = serversLoading || statsLoading || clusterLoading || backupsLoading || (canAudit && auditLoading);

  // TODO(slice-2+): Render stat cards (Running, Players online, vCPUs, Storage, Nodes ready),
  // Fleet status card with bar + legend + needs-attention, Cluster resources with Meter + nodes list,
  // Recent activity rows, Recent backups empty state.

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Dashboard"
        subtitle="At-a-glance health of your Gameplane cluster."
      />

      {isLoading ? (
        <Card className="flex items-center justify-center p-8">
          <CardContent>
            <Spinner />
          </CardContent>
        </Card>
      ) : (
        <Card className="p-8">
          <CardContent className="text-center text-muted">
            {/* TODO(slice-2+): Add dashboard content */}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
