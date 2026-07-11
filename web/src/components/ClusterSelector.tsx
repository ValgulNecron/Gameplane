import { ChevronDown, Check, Plus } from "lucide-react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { setCurrentCluster, useCurrentCluster } from "@/lib/cluster";
import { Clusters } from "@/lib/endpoints";
import type { ClusterRegistry } from "@/types";
import { cn } from "@/lib/utils";
import * as Menu from "@radix-ui/react-dropdown-menu";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";

function getPhaseColor(phase: ClusterRegistry["phase"]): string {
  switch (phase) {
    case "Healthy":
      return "bg-success";
    case "Unhealthy":
      return "bg-danger";
    case "Unknown":
    default:
      return "bg-muted";
  }
}

function getDisplayName(cluster: ClusterRegistry | undefined | null): string {
  if (!cluster) return "local";
  return cluster.displayName || cluster.name || "local";
}

export function ClusterSelector() {
  const currentClusterId = useCurrentCluster();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const { data, isLoading, error } = useQuery({
    queryKey: ["clusters"],
    queryFn: () => Clusters.list(),
  });

  const clusters = data?.items ?? [];
  const currentCluster = clusters.find((c) => c.name === currentClusterId);
  const displayName = getDisplayName(currentCluster);
  const phase = currentCluster?.phase ?? "Unknown";
  const phaseColor = getPhaseColor(phase);

  const handleSelectCluster = (clusterId: string): void => {
    setCurrentCluster(clusterId);
    void qc.clear();
  };

  const handleAddCluster = (): void => {
    void navigate({ to: "/cluster" });
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label="Select cluster"
          className={cn(
            "flex items-center gap-1.5 rounded-full border border-border bg-card px-3 py-1.5 text-sm",
            "text-fg hover:bg-surface transition-colors",
          )}
        >
          <span className={cn("h-2 w-2 rounded-full", phaseColor)} />
          <span className="truncate">{displayName}</span>
          <ChevronDown className="h-3.5 w-3.5 text-muted shrink-0" />
        </button>
      </DropdownMenuTrigger>

      <DropdownMenuContent align="end" className="min-w-[200px]">
        {isLoading || error ? (
          <div className="px-2 py-1.5 text-sm text-muted">{isLoading ? "Loading…" : "Error loading clusters"}</div>
        ) : clusters.length === 0 ? (
          <div className="px-2 py-1.5 text-sm text-muted">No clusters available</div>
        ) : (
          <>
            {clusters.map((cluster) => (
              <Menu.Item
                key={cluster.name}
                onSelect={() => handleSelectCluster(cluster.name)}
                className={cn(
                  "flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-sm outline-none",
                  "text-fg data-[highlighted]:bg-surface/70",
                )}
              >
                <span className={cn("h-2 w-2 rounded-full", getPhaseColor(cluster.phase))} />
                <span className="flex-1 text-left">{cluster.displayName || cluster.name}</span>
                {cluster.name === currentClusterId && (
                  <Check className="h-3.5 w-3.5 text-primary shrink-0" />
                )}
              </Menu.Item>
            ))}

            <DropdownMenuSeparator />

            <Menu.Item
              onSelect={handleAddCluster}
              className={cn(
                "flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-sm outline-none",
                "text-fg data-[highlighted]:bg-surface/70",
              )}
            >
              <Plus className="h-3.5 w-3.5 text-muted shrink-0" />
              <span>Add cluster</span>
            </Menu.Item>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
