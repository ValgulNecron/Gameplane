import { ChevronDown, Check, Plus } from "lucide-react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { setCurrentCluster, useCurrentCluster } from "@/lib/cluster";
import { Clusters } from "@/lib/endpoints";
import type { ClusterRegistry } from "@/types";
import { cn } from "@/lib/utils";
import {
  Dropdown,
  DropdownTrigger,
  DropdownPopover,
  DropdownMenu,
  DropdownItem,
  DropdownSection,
} from "@heroui/react";

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
    <Dropdown>
      {/* DropdownTrigger itself renders the real `<button>` (react-aria-components'
          Button primitive); a raw `<button>` child here would nest buttons
          (invalid HTML, breaks React's hydration and much of testing-library's
          querying), so the trigger content is a plain `<div>` and the a11y
          label moves onto DropdownTrigger, which forwards it to that button. */}
      <DropdownTrigger
        aria-label="Select cluster"
        className={cn(
          "flex items-center gap-1.5 rounded-full border border-border bg-card px-3 py-1.5 text-sm",
          "text-fg hover:bg-surface transition-colors cursor-pointer",
        )}
      >
        <>
          <span className={cn("h-2 w-2 rounded-full", phaseColor)} />
          <span className="truncate">{displayName}</span>
          <ChevronDown className="h-3.5 w-3.5 text-muted shrink-0" />
        </>
      </DropdownTrigger>

      <DropdownPopover placement="bottom start">
        <DropdownMenu aria-label="Cluster options" className="min-w-[200px]">
          {isLoading || error ? (
            <DropdownItem isDisabled>
              <span className="text-sm text-muted">
                {isLoading ? "Loading…" : "Error loading clusters"}
              </span>
            </DropdownItem>
          ) : clusters.length === 0 ? (
            <DropdownItem isDisabled>
              <span className="text-sm text-muted">No clusters available</span>
            </DropdownItem>
          ) : (
            <>
              <DropdownSection>
                {clusters.map((cluster) => (
                  <DropdownItem
                    key={cluster.name}
                    onPress={() => handleSelectCluster(cluster.name)}
                    textValue={cluster.displayName || cluster.name}
                    className="flex items-center gap-2"
                  >
                    <div className="flex items-center gap-2 flex-1">
                      <span
                        className={cn("h-2 w-2 rounded-full", getPhaseColor(cluster.phase))}
                      />
                      <span>{cluster.displayName || cluster.name}</span>
                    </div>
                    {cluster.name === currentClusterId && (
                      <Check className="h-3.5 w-3.5 text-primary shrink-0" />
                    )}
                  </DropdownItem>
                ))}
              </DropdownSection>

              <DropdownSection>
                <DropdownItem
                  key="add-cluster"
                  onPress={handleAddCluster}
                  textValue="Add cluster"
                  className="flex items-center gap-2"
                >
                  <Plus className="h-3.5 w-3.5 text-muted shrink-0" />
                  <span>Add cluster</span>
                </DropdownItem>
              </DropdownSection>
            </>
          )}
        </DropdownMenu>
      </DropdownPopover>
    </Dropdown>
  );
}
