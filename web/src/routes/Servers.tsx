import { useMemo, useState, type ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ServerActionsMenu } from "@/components/server/ServerActionsMenu";
import {
  Activity,
  Cpu,
  Filter,
  HardDrive,
  Play,
  Plus,
  RotateCw,
  Search,
  Server as ServerIcon,
  Share2,
  Square,
  Sunrise,
  Users as UsersIcon,
} from "lucide-react";

import { Button, Card, Input, Chip, Tabs, Tab, Table, buttonVariants } from "@heroui/react";
import { StatCard } from "@/components/hero/StatCard";
import { PhaseChip } from "@/components/hero/PhaseChip";
import { FilterPopover } from "@/components/hero/FilterPopover";
import { GameIcon } from "@/components/hero/GameIcon";
import { PageHeader } from "@/components/PageHeader";
import { describeStorageProvisioned, formatBytes, cn } from "@/lib/utils";
import { useMediaQuery } from "@/lib/media";
import type { ClusterStats, ClusterView, GameServer, GameServerPhase } from "@/types";
import { Cluster, Servers, type LifecycleVerb } from "@/lib/endpoints";
import { countByState } from "@/lib/servers";

type FilterKey = "all" | "running" | "stopped";

export function ServersPage() {
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({
    queryKey: ["servers"],
    queryFn: () => Servers.list(),
    refetchInterval: 5_000,
  });

  const { data: cluster } = useQuery({
    queryKey: ["cluster-stats"],
    queryFn: () => Cluster.stats().catch(() => ({} as ClusterStats)),
    staleTime: 30_000,
  });
  const { data: clusterView } = useQuery({
    queryKey: ["cluster"],
    queryFn: () => Cluster.view().catch(() => ({} as ClusterView)),
    staleTime: 30_000,
  });

  const { data: myServers } = useQuery({
    queryKey: ["my-servers"],
    queryFn: () => Servers.getMyServers(),
    refetchInterval: 5_000,
  });

  const act = useMutation({
    mutationFn: (args: { name: string; verb: LifecycleVerb }) =>
      Servers.lifecycle(args.name, args.verb),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["servers"] }),
  });

  // Below `md`, a wide table doesn't fit — render stacked cards instead.
  // Driven by matchMedia (not a responsive CSS class toggle) so only one
  // representation of each server ever mounts; two would duplicate every
  // row's accessible text in the DOM.
  const isMobile = useMediaQuery("(max-width: 767px)");

  const [filter, setFilter] = useState<FilterKey>("all");
  const [query, setQuery] = useState("");
  const [isFilterOpen, setIsFilterOpen] = useState(false);
  const [appliedGames, setAppliedGames] = useState<Set<string>>(new Set());
  const [appliedNamespaces, setAppliedNamespaces] = useState<Set<string>>(new Set());
  const [draftGames, setDraftGames] = useState<Set<string>>(new Set());
  const [draftNamespaces, setDraftNamespaces] = useState<Set<string>>(new Set());

  const servers = useMemo(() => data?.items ?? [], [data?.items]);

  // Compute shared servers (in my-servers but not in the main list)
  const sharedServers = useMemo(() => {
    if (!myServers?.items) return [];
    const serverKeys = new Set(servers.map((s) => `${s.metadata.namespace ?? "gameplane-games"}/${s.metadata.name}`));
    return myServers.items.filter((s) => {
      const key = `${s.metadata.namespace ?? "gameplane-games"}/${s.metadata.name}`;
      return !serverKeys.has(key);
    });
  }, [servers, myServers]);
  const counts = useMemo(() => countByState(servers), [servers]);
  const vcpus = (clusterView?.nodes ?? []).reduce((s, n) => s + (n.cpu?.capacity ?? 0), 0);
  const storage = describeStorageProvisioned(cluster?.usedStorageBytes, cluster?.totalStorageBytes);

  // Derive distinct games and namespaces from servers
  const distinctGames = useMemo(() => {
    const games = new Set<string>();
    servers.forEach((s) => {
      games.add(s.spec.templateRef.name);
    });
    return Array.from(games).sort();
  }, [servers]);

  const distinctNamespaces = useMemo(() => {
    const namespaces = new Set<string>();
    servers.forEach((s) => {
      namespaces.add(s.metadata.namespace ?? "gameplane-games");
    });
    return Array.from(namespaces).sort();
  }, [servers]);

  const appliedFacetCount = appliedGames.size + appliedNamespaces.size;

  const filterServer = (gs: GameServer) => {
    if (query && !gs.metadata.name.toLowerCase().includes(query.toLowerCase())) return false;
    // Facet filters compose with the status tab: evaluate them before the
    // status short-circuit so an active Running/Stopped tab doesn't bypass
    // the applied game/namespace facets.
    if (appliedGames.size > 0 && !appliedGames.has(gs.spec.templateRef.name)) return false;
    const ns = gs.metadata.namespace ?? "gameplane-games";
    if (appliedNamespaces.size > 0 && !appliedNamespaces.has(ns)) return false;
    const phase = gs.status?.phase;
    if (filter === "running") return phase === "Running";
    if (filter === "stopped") return phase === "Stopped" || phase === "Suspended" || phase === "Failed";
    return true;
  };

  const handleOpenFilterChange = (open: boolean) => {
    setIsFilterOpen(open);
    if (open) {
      setDraftGames(new Set(appliedGames));
      setDraftNamespaces(new Set(appliedNamespaces));
    }
  };

  const handleToggleDraftGame = (game: string) => {
    const newSet = new Set(draftGames);
    if (newSet.has(game)) {
      newSet.delete(game);
    } else {
      newSet.add(game);
    }
    setDraftGames(newSet);
  };

  const handleToggleDraftNamespace = (ns: string) => {
    const newSet = new Set(draftNamespaces);
    if (newSet.has(ns)) {
      newSet.delete(ns);
    } else {
      newSet.add(ns);
    }
    setDraftNamespaces(newSet);
  };

  const handleApplyFilter = () => {
    setAppliedGames(new Set(draftGames));
    setAppliedNamespaces(new Set(draftNamespaces));
    setIsFilterOpen(false);
  };

  const handleClearFilter = () => {
    setDraftGames(new Set());
    setDraftNamespaces(new Set());
  };

  const visible = servers.filter(filterServer);
  const visibleShared = sharedServers.filter(filterServer);

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Servers"
        subtitle="Manage game server workloads across your cluster."
        actions={
          <Link to="/servers/new" className={cn(buttonVariants({ variant: "primary" }), "rounded-full")}>
            <Plus className="h-4 w-4" /> Create server
          </Link>
        }
      />

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
        <StatCard
          label="Running"
          icon={<Activity className="h-4 w-4" />}
          value={counts.running}
          sub={`of ${servers.length} total`}
          accent="success"
        />
        <StatCard
          label="Players online"
          icon={<UsersIcon className="h-4 w-4" />}
          value={counts.players}
          sub={`peak ${counts.playersMax}`}
          accent="primary"
        />
        <StatCard
          label="vCPUs"
          icon={<Cpu className="h-4 w-4" />}
          value={vcpus > 0 ? vcpus : "—"}
          sub="cluster cores"
          accent="warning"
        />
        <StatCard
          label="Storage provisioned"
          icon={<HardDrive className="h-4 w-4" />}
          value={storage.valueText}
          sub={storage.subText ?? "—"}
          accent={storage.overcommitted ? "warning" : "violet"}
        />
        <StatCard
          label="Cluster size"
          icon={<ServerIcon className="h-4 w-4" />}
          value={cluster?.nodes ?? "—"}
          sub="nodes ready"
          accent="warning"
        />
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <Tabs
          selectedKey={filter}
          onSelectionChange={(key) => setFilter(key as FilterKey)}
          variant="secondary"
        >
          <Tabs.List aria-label="Server status filter">
            <Tab id="all">
              <span className="inline-flex items-center gap-1.5">
                All
                <span className="rounded-[4px] bg-foreground/10 px-1.5 py-0.5 text-xs leading-none">
                  {servers.length}
                </span>
              </span>
            </Tab>
            <Tab id="running">
              <span className="inline-flex items-center gap-1.5">
                Running
                <span className="rounded-[4px] bg-foreground/10 px-1.5 py-0.5 text-xs leading-none">
                  {counts.running}
                </span>
              </span>
            </Tab>
            <Tab id="stopped">
              <span className="inline-flex items-center gap-1.5">
                Stopped
                <span className="rounded-[4px] bg-foreground/10 px-1.5 py-0.5 text-xs leading-none">
                  {counts.stopped}
                </span>
              </span>
            </Tab>
          </Tabs.List>
        </Tabs>
        <div className="ml-auto flex items-center gap-2">
          <div className="relative w-64">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-foreground/60" />
            <Input
              placeholder="Search servers…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="w-full pl-9"
              aria-label="Search servers"
            />
          </div>
          <FilterPopover
            games={distinctGames}
            selectedGames={draftGames}
            onToggleGame={handleToggleDraftGame}
            namespaces={distinctNamespaces}
            selectedNamespaces={draftNamespaces}
            onToggleNamespace={handleToggleDraftNamespace}
            onApply={handleApplyFilter}
            onClear={handleClearFilter}
            isOpen={isFilterOpen}
            onOpenChange={handleOpenFilterChange}
          >
            <Button
              variant="outline"
              className="rounded-[6px] relative"
              onPress={() => {
                const tabList = document.querySelector('[aria-label="Server status filter"]');
                if (tabList instanceof HTMLElement) {
                  tabList.focus();
                }
              }}
            >
              <Filter className="h-4 w-4" />
              Filter
              {appliedFacetCount > 0 && (
                <Chip size="sm" variant="soft" className="ml-1.5">
                  {appliedFacetCount}
                </Chip>
              )}
            </Button>
          </FilterPopover>
        </div>
      </div>

      {isMobile ? (
        <div className="space-y-3">
          {isLoading && (
            <Card className="p-10 text-center text-sm text-foreground/60">Loading…</Card>
          )}
          {!isLoading && visible.length === 0 && visibleShared.length === 0 && (
            <Card className="p-12 text-center text-sm text-foreground/60">No servers match.</Card>
          )}
          {visible.map((gs) => (
            <ServerCard key={gs.metadata.name} gs={gs} onAct={act.mutate} />
          ))}

          {visibleShared.length > 0 && (
            <>
              <div className="flex items-center gap-2 px-1 pt-2 text-xs font-semibold uppercase tracking-wider text-foreground/60">
                <Share2 className="h-4 w-4" />
                Shared with you
              </div>
              {visibleShared.map((gs) => (
                <ServerCard
                  key={`shared-${gs.metadata.namespace ?? ""}-${gs.metadata.name}`}
                  gs={gs}
                  onAct={act.mutate}
                />
              ))}
            </>
          )}
        </div>
      ) : (
        <div className="rounded-lg border border-border bg-card overflow-hidden">
          <Table.Root className="bg-transparent">
            <Table.ScrollContainer>
              <Table.Content aria-label="Server list">
                <Table.Header>
                  <Table.Column id="name" isRowHeader>Name</Table.Column>
                  <Table.Column id="game">Game</Table.Column>
                  <Table.Column id="status">Status</Table.Column>
                  <Table.Column id="cpu">CPU</Table.Column>
                  <Table.Column id="memory">Memory</Table.Column>
                  <Table.Column id="players">Players</Table.Column>
                  <Table.Column id="node">Node</Table.Column>
                  <Table.Column id="actions" className="text-right">Actions</Table.Column>
                </Table.Header>
                <Table.Body
                  renderEmptyState={() => (
                    <div className="text-center py-10 text-foreground/60">
                      {isLoading ? "Loading…" : "No servers match."}
                    </div>
                  )}
                >
              {visible.map((gs) => (
                <Table.Row key={gs.metadata.name}>
                  <Table.Cell>
                    <div className="flex items-center gap-3">
                      <GameIcon game={gs.spec.templateRef.name} size="sm" />
                      <div className="min-w-0">
                        <Link
                          to="/servers/$name"
                          params={{ name: gs.metadata.name }}
                          search={gs.metadata.namespace && gs.metadata.namespace !== "gameplane-games" ? { ns: gs.metadata.namespace } : {}}
                          className="truncate font-mono text-sm text-foreground hover:text-primary"
                        >
                          {gs.metadata.name}
                        </Link>
                        <div className="text-[11px] text-foreground/60">
                          {gs.metadata.namespace ?? "gameplane-games"}
                        </div>
                      </div>
                    </div>
                  </Table.Cell>
                  <Table.Cell>{gs.spec.templateRef.name}</Table.Cell>
                  <Table.Cell>
                    {(() => {
                      const { phase, asleep } = serverRowData(gs);
                      return <PhaseChip phase={phase} asleep={asleep} />;
                    })()}
                  </Table.Cell>
                  <Table.Cell>
                    {(() => {
                      const { cpuLabel } = serverRowData(gs);
                      return <span className="font-mono">{cpuLabel}</span>;
                    })()}
                  </Table.Cell>
                  <Table.Cell>
                    {(() => {
                      const { memLabel } = serverRowData(gs);
                      return <span className="font-mono">{memLabel}</span>;
                    })()}
                  </Table.Cell>
                  <Table.Cell>
                    {(() => {
                      const { playersLabel } = serverRowData(gs);
                      return <span className="font-mono">{playersLabel}</span>;
                    })()}
                  </Table.Cell>
                  <Table.Cell>
                    {(() => {
                      const { node } = serverRowData(gs);
                      return <span className="font-mono text-foreground/60">{node ?? "—"}</span>;
                    })()}
                  </Table.Cell>
                  <Table.Cell className="text-right">
                    {(() => {
                      const { isSharedNonDefault, phase, asleep } = serverRowData(gs);
                      return !isSharedNonDefault ? <ServerLifecycleActions gs={gs} phase={phase} asleep={asleep} onAct={act.mutate} /> : null;
                    })()}
                  </Table.Cell>
                </Table.Row>
              ))}

              {visibleShared.length > 0 && (
                <>
                  <Table.Row className="bg-surface/20" key="shared-header">
                    <Table.Cell colSpan={8}>
                      <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-foreground/60">
                        <Share2 className="h-4 w-4" />
                        Shared with you
                      </div>
                    </Table.Cell>
                  </Table.Row>
                  {visibleShared.map((gs) => (
                    <Table.Row key={`shared-${gs.metadata.namespace ?? ""}-${gs.metadata.name}`}>
                      <Table.Cell>
                        <div className="flex items-center gap-3">
                          <GameIcon game={gs.spec.templateRef.name} size="sm" />
                          <div className="min-w-0">
                            <Link
                              to="/servers/$name"
                              params={{ name: gs.metadata.name }}
                              search={gs.metadata.namespace && gs.metadata.namespace !== "gameplane-games" ? { ns: gs.metadata.namespace } : {}}
                              className="truncate font-mono text-sm text-foreground hover:text-primary"
                            >
                              {gs.metadata.name}
                            </Link>
                            <div className="text-[11px] text-foreground/60">
                              {gs.metadata.namespace ?? "gameplane-games"}
                            </div>
                          </div>
                        </div>
                      </Table.Cell>
                      <Table.Cell>{gs.spec.templateRef.name}</Table.Cell>
                      <Table.Cell>
                        {(() => {
                          const { phase, asleep } = serverRowData(gs);
                          return <PhaseChip phase={phase} asleep={asleep} />;
                        })()}
                      </Table.Cell>
                      <Table.Cell>
                        {(() => {
                          const { cpuLabel } = serverRowData(gs);
                          return <span className="font-mono">{cpuLabel}</span>;
                        })()}
                      </Table.Cell>
                      <Table.Cell>
                        {(() => {
                          const { memLabel } = serverRowData(gs);
                          return <span className="font-mono">{memLabel}</span>;
                        })()}
                      </Table.Cell>
                      <Table.Cell>
                        {(() => {
                          const { playersLabel } = serverRowData(gs);
                          return <span className="font-mono">{playersLabel}</span>;
                        })()}
                      </Table.Cell>
                      <Table.Cell>
                        {(() => {
                          const { node } = serverRowData(gs);
                          return <span className="font-mono text-foreground/60">{node ?? "—"}</span>;
                        })()}
                      </Table.Cell>
                      <Table.Cell className="text-right">
                        {(() => {
                          const { isSharedNonDefault, phase, asleep } = serverRowData(gs);
                          return !isSharedNonDefault ? <ServerLifecycleActions gs={gs} phase={phase} asleep={asleep} onAct={act.mutate} /> : null;
                        })()}
                      </Table.Cell>
                    </Table.Row>
                  ))}
                </>
              )}
                </Table.Body>
              </Table.Content>
            </Table.ScrollContainer>
          </Table.Root>
        </div>
      )}
    </div>
  );
}

// serverRowData derives the small set of display values ServerRow and
// ServerCard both need (CPU/memory %, player count, shared-read-only flag)
// so the mobile card list can't silently drift from the desktop table's
// formatting.
function serverRowData(gs: GameServer) {
  const phase = gs.status?.phase;
  // The operator reports Stopping (not Suspended) for the whole graceful-
  // drain window even after status.idle.asleep flips true — players may
  // still be connected. Excluding Stopping keeps a draining server's badge
  // honest and keeps Wake from appearing while it's still shutting down.
  const asleep = gs.status?.idle?.asleep === true && phase !== "Stopping";
  const agent = gs.status?.agent;
  const players = agent?.playersOnline;
  const maxPlayers = agent?.playersMax;
  const node = gs.metadata.annotations?.["gameplane.local/node"];
  // Shared rows with non-default namespace are read-only (detail route and
  // lifecycle calls are namespace-blind).
  const isSharedNonDefault =
    !!gs.metadata.namespace && gs.metadata.namespace !== "gameplane-games";

  // Resource usage comes from the agent's heartbeat (cgroup + statfs).
  // null/undefined means "unknown" (unreadable source, or a stale heartbeat
  // the API blanked) — render "—", not a misleading 0. Mirrors Overview.tsx.
  const cpuKnown = typeof agent?.cpuMillicores === "number";
  const cpuMilli = cpuKnown ? (agent?.cpuMillicores as number) : 0;
  const cpuLimitMilli =
    typeof agent?.cpuLimitMillicores === "number" ? (agent?.cpuLimitMillicores as number) : 0;
  const cpuLabel = cpuKnown
    ? cpuLimitMilli
      ? `${((cpuMilli / cpuLimitMilli) * 100).toFixed(0)}%`
      : `${(cpuMilli / 1000).toFixed(2)} cores`
    : "—";

  const memKnown = typeof agent?.memoryBytes === "number";
  const memUsed = memKnown ? (agent?.memoryBytes as number) : 0;
  const memLimit =
    typeof agent?.memoryLimitBytes === "number" ? (agent?.memoryLimitBytes as number) : 0;
  const memLabel = memKnown
    ? memLimit
      ? `${((memUsed / memLimit) * 100).toFixed(0)}%`
      : formatBytes(memUsed)
    : "—";

  const playersLabel =
    typeof players === "number" && players >= 0
      ? typeof maxPlayers === "number" && maxPlayers >= 0
        ? `${players}/${maxPlayers}`
        : `${players}`
      : "—";

  return { phase, asleep, node, isSharedNonDefault, cpuLabel, memLabel, playersLabel };
}

// ServerLifecycleActions is the Start/Stop/Restart/menu cluster shared by
// the desktop table row and the mobile card.
function ServerLifecycleActions({
  gs,
  phase,
  asleep,
  onAct,
}: {
  gs: GameServer;
  phase?: GameServerPhase;
  asleep?: boolean;
  onAct: (args: { name: string; verb: LifecycleVerb }) => void;
}) {
  const qc = useQueryClient();
  const invalidate = () => {
    void qc.invalidateQueries({ queryKey: ["servers"] });
    void qc.invalidateQueries({ queryKey: ["my-servers"] });
  };
  return (
    <div className="inline-flex items-center">
      {asleep ? (
        <ActionButton
          title="Wake"
          onClick={() => onAct({ name: gs.metadata.name, verb: "wake" })}
        >
          <Sunrise className="h-4 w-4" />
        </ActionButton>
      ) : (
        <ActionButton
          title="Start"
          disabled={phase === "Running" || phase === "Starting"}
          onClick={() => onAct({ name: gs.metadata.name, verb: "start" })}
        >
          <Play className="h-4 w-4" />
        </ActionButton>
      )}
      <ActionButton
        title="Stop"
        // An asleep server is phase Suspended, but :stop is still a real
        // action there — it patches spec.suspend=true, which the operator
        // honors immediately, distinct from an idle sleep a wake window
        // would otherwise resurrect.
        disabled={!asleep && (phase === "Stopped" || phase === "Suspended")}
        onClick={() => onAct({ name: gs.metadata.name, verb: "stop" })}
      >
        <Square className="h-4 w-4" />
      </ActionButton>
      <ActionButton
        title="Restart"
        onClick={() => onAct({ name: gs.metadata.name, verb: "restart" })}
      >
        <RotateCw className="h-4 w-4" />
      </ActionButton>
      <ServerActionsMenu gs={gs} onDeleted={invalidate} onTransferred={invalidate} />
    </div>
  );
}


// ServerCard is the mobile (< md) stand-in for a table row: name, game,
// status pill, a row of stat chips, and the same lifecycle actions.
function ServerCard({
  gs,
  onAct,
}: {
  gs: GameServer;
  onAct: (args: { name: string; verb: LifecycleVerb }) => void;
}) {
  const { phase, asleep, node, isSharedNonDefault, cpuLabel, memLabel, playersLabel } = serverRowData(gs);

  return (
    <Card className="border border-border bg-surface p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <GameIcon game={gs.spec.templateRef.name} size="sm" />
          <div className="min-w-0">
            <Link
              to="/servers/$name"
              params={{ name: gs.metadata.name }}
              search={isSharedNonDefault ? { ns: gs.metadata.namespace } : {}}
              className="block truncate font-mono text-sm text-foreground hover:text-primary"
            >
              {gs.metadata.name}
            </Link>
            <div className="truncate text-[11px] text-foreground/60">
              {gs.spec.templateRef.name} · {gs.metadata.namespace ?? "gameplane-games"}
            </div>
          </div>
        </div>
        <PhaseChip phase={phase} asleep={asleep} />
      </div>

      <div className="mt-3 flex flex-wrap gap-2">
        <StatChip icon={<Cpu className="h-3 w-3" />} label="CPU" value={cpuLabel} />
        <StatChip icon={<HardDrive className="h-3 w-3" />} label="Mem" value={memLabel} />
        <StatChip icon={<UsersIcon className="h-3 w-3" />} label="Players" value={playersLabel} />
        <StatChip icon={<ServerIcon className="h-3 w-3" />} label="Node" value={node ?? "—"} />
      </div>

      {!isSharedNonDefault && (
        <div className="mt-3 flex items-center justify-end border-t border-border pt-3">
          <ServerLifecycleActions gs={gs} phase={phase} asleep={asleep} onAct={onAct} />
        </div>
      )}
    </Card>
  );
}

function StatChip({ icon, label, value }: { icon: ReactNode; label: string; value: ReactNode }) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-md bg-default/40 px-2 py-1 text-[11px] text-foreground/60">
      {icon}
      {label}
      <span className="font-mono text-foreground">{value}</span>
    </span>
  );
}

function ActionButton({
  children, title, onClick, disabled,
}: {
  children: ReactNode;
  title: string;
  onClick?: () => void;
  disabled?: boolean;
}) {
  return (
    <Button
      isIconOnly
      variant="ghost"
      aria-label={title}
      onPress={onClick}
      isDisabled={disabled}
      size="sm"
      className="text-foreground/60 hover:text-foreground"
    >
      <span title={title}>{children}</span>
    </Button>
  );
}
