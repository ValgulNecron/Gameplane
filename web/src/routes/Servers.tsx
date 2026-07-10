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
  Users as UsersIcon,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge, PhaseBadge } from "@/components/ui/badge";
import { StatCard } from "@/components/ui/stat";
import { TabBar } from "@/components/ui/tabs";
import { GameIcon } from "@/components/ui/game-icon";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuSeparator,
  DropdownMenuCheckboxItem,
} from "@/components/ui/dropdown-menu";
import { PageHeader } from "@/components/PageHeader";
import { Card } from "@/components/ui/card";
import { cn, formatBytes } from "@/lib/utils";
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
  }, [servers, myServers?.items]);
  const counts = useMemo(() => countByState(servers), [servers]);
  const vcpus = (clusterView?.nodes ?? []).reduce((s, n) => s + (n.cpu?.capacity ?? 0), 0);

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
          <Button asChild>
            <Link to="/servers/new"><Plus className="h-4 w-4" /> Create server</Link>
          </Button>
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
          label="Storage"
          icon={<HardDrive className="h-4 w-4" />}
          value={formatBytes(cluster?.usedStorageBytes ?? 0)}
          sub={
            cluster?.totalStorageBytes
              ? `of ${formatBytes(cluster.totalStorageBytes)}`
              : "—"
          }
          accent="violet"
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
        <TabBar
          items={[
            { key: "all",     label: "All",     count: servers.length },
            { key: "running", label: "Running", count: counts.running },
            { key: "stopped", label: "Stopped", count: counts.stopped },
          ]}
          value={filter}
          onChange={setFilter}
        />
        <div className="ml-auto flex items-center gap-2">
          <div className="relative w-64">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" />
            <Input
              className="pl-9"
              placeholder="Search servers…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </div>
          <DropdownMenu open={isFilterOpen} onOpenChange={handleOpenFilterChange}>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" size="default" className="relative">
                <Filter className="h-4 w-4" />
                Filter
                {appliedFacetCount > 0 && (
                  <Badge variant="primary" className="ml-1.5">
                    {appliedFacetCount}
                  </Badge>
                )}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-[280px]">
              {/* Game section */}
              <div className="px-2 py-1.5 text-xs font-semibold text-muted">Game</div>
              {distinctGames.map((game) => (
                <DropdownMenuCheckboxItem
                  key={game}
                  checked={draftGames.has(game)}
                  onSelect={(e) => {
                    e.preventDefault();
                    handleToggleDraftGame(game);
                  }}
                >
                  {game}
                </DropdownMenuCheckboxItem>
              ))}

              <DropdownMenuSeparator />

              {/* Namespace section */}
              <div className="px-2 py-1.5 text-xs font-semibold text-muted">Namespace</div>
              {distinctNamespaces.map((ns) => (
                <DropdownMenuCheckboxItem
                  key={ns}
                  checked={draftNamespaces.has(ns)}
                  onSelect={(e) => {
                    e.preventDefault();
                    handleToggleDraftNamespace(ns);
                  }}
                >
                  {ns}
                </DropdownMenuCheckboxItem>
              ))}

              <DropdownMenuSeparator />

              {/* Footer buttons */}
              <div className="flex items-center justify-between gap-2 px-2 py-1.5">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handleClearFilter}
                  className="h-7"
                >
                  Clear
                </Button>
                <Button
                  variant="default"
                  size="sm"
                  onClick={handleApplyFilter}
                  className="h-7"
                >
                  Apply
                </Button>
              </div>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {isMobile ? (
        <div className="space-y-3">
          {isLoading && (
            <Card className="p-10 text-center text-sm text-muted">Loading…</Card>
          )}
          {!isLoading && visible.length === 0 && visibleShared.length === 0 && (
            <Card className="p-12 text-center text-sm text-muted">No servers match.</Card>
          )}
          {visible.map((gs) => (
            <ServerCard key={gs.metadata.name} gs={gs} onAct={act.mutate} />
          ))}

          {visibleShared.length > 0 && (
            <>
              <div className="flex items-center gap-2 px-1 pt-2 text-xs font-semibold uppercase tracking-wider text-muted">
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
        <div className="overflow-hidden rounded-lg border border-border bg-card">
          <table className="w-full text-sm">
            <thead className="bg-surface/70 text-left text-[11px] uppercase tracking-wider text-muted">
              <tr>
                <th className="px-4 py-3">Name</th>
                <th className="px-4 py-3">Game</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3">CPU</th>
                <th className="px-4 py-3">Memory</th>
                <th className="px-4 py-3">Players</th>
                <th className="px-4 py-3">Node</th>
                <th className="px-4 py-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {isLoading && (
                <tr><td className="px-4 py-10 text-center text-muted" colSpan={8}>Loading…</td></tr>
              )}
              {!isLoading && visible.length === 0 && visibleShared.length === 0 && (
                <tr><td className="px-4 py-12 text-center text-muted" colSpan={8}>
                  No servers match.
                </td></tr>
              )}
              {visible.map((gs) => <ServerRow key={gs.metadata.name} gs={gs} onAct={act.mutate} />)}

              {visibleShared.length > 0 && (
                <>
                  <tr className="bg-surface/20">
                    <td colSpan={8} className="px-4 py-3">
                      <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-muted">
                        <Share2 className="h-4 w-4" />
                        Shared with you
                      </div>
                    </td>
                  </tr>
                  {visibleShared.map((gs) => <ServerRow key={`shared-${gs.metadata.namespace ?? ""}-${gs.metadata.name}`} gs={gs} onAct={act.mutate} />)}
                </>
              )}
            </tbody>
          </table>
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

  return { phase, node, isSharedNonDefault, cpuLabel, memLabel, playersLabel };
}

// ServerLifecycleActions is the Start/Stop/Restart/menu cluster shared by
// the desktop table row and the mobile card.
function ServerLifecycleActions({
  gs,
  phase,
  onAct,
}: {
  gs: GameServer;
  phase?: GameServerPhase;
  onAct: (args: { name: string; verb: LifecycleVerb }) => void;
}) {
  const qc = useQueryClient();
  const invalidate = () => {
    void qc.invalidateQueries({ queryKey: ["servers"] });
    void qc.invalidateQueries({ queryKey: ["my-servers"] });
  };
  return (
    <div className="inline-flex items-center">
      <ActionButton
        title="Start"
        disabled={phase === "Running" || phase === "Starting"}
        onClick={() => onAct({ name: gs.metadata.name, verb: "start" })}
      >
        <Play className="h-4 w-4" />
      </ActionButton>
      <ActionButton
        title="Stop"
        disabled={phase === "Stopped" || phase === "Suspended"}
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

function ServerRow({
  gs,
  onAct,
}: {
  gs: GameServer;
  onAct: (args: { name: string; verb: LifecycleVerb }) => void;
}) {
  const { phase, node, isSharedNonDefault, cpuLabel, memLabel, playersLabel } = serverRowData(gs);

  return (
    <tr className="hover:bg-surface/40">
      <td className="px-4 py-3">
        <div className="flex items-center gap-3">
          <GameIcon game={gs.spec.templateRef.name} size="sm" />
          <div className="min-w-0">
            <Link
              to="/servers/$name"
              params={{ name: gs.metadata.name }}
              search={isSharedNonDefault ? { ns: gs.metadata.namespace } : {}}
              className="truncate font-mono text-sm text-fg hover:text-primary"
            >
              {gs.metadata.name}
            </Link>
            <div className="text-[11px] text-muted">
              {gs.metadata.namespace ?? "gameplane-games"}
            </div>
          </div>
        </div>
      </td>
      <td className="px-4 py-3 text-muted">{gs.spec.templateRef.name}</td>
      <td className="px-4 py-3"><PhaseBadge phase={phase} /></td>
      <td className="px-4 py-3 font-mono">{cpuLabel}</td>
      <td className="px-4 py-3 font-mono">{memLabel}</td>
      <td className="px-4 py-3 font-mono">{playersLabel}</td>
      <td className="px-4 py-3 font-mono text-muted">{node ?? "—"}</td>
      <td className="px-4 py-3 text-right">
        {!isSharedNonDefault && <ServerLifecycleActions gs={gs} phase={phase} onAct={onAct} />}
      </td>
    </tr>
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
  const { phase, node, isSharedNonDefault, cpuLabel, memLabel, playersLabel } = serverRowData(gs);

  return (
    <Card className="p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <GameIcon game={gs.spec.templateRef.name} size="sm" />
          <div className="min-w-0">
            <Link
              to="/servers/$name"
              params={{ name: gs.metadata.name }}
              search={isSharedNonDefault ? { ns: gs.metadata.namespace } : {}}
              className="block truncate font-mono text-sm text-fg hover:text-primary"
            >
              {gs.metadata.name}
            </Link>
            <div className="truncate text-[11px] text-muted">
              {gs.spec.templateRef.name} · {gs.metadata.namespace ?? "gameplane-games"}
            </div>
          </div>
        </div>
        <PhaseBadge phase={phase} />
      </div>

      <div className="mt-3 flex flex-wrap gap-2">
        <StatChip icon={<Cpu className="h-3 w-3" />} label="CPU" value={cpuLabel} />
        <StatChip icon={<HardDrive className="h-3 w-3" />} label="Mem" value={memLabel} />
        <StatChip icon={<UsersIcon className="h-3 w-3" />} label="Players" value={playersLabel} />
        <StatChip icon={<ServerIcon className="h-3 w-3" />} label="Node" value={node ?? "—"} />
      </div>

      {!isSharedNonDefault && (
        <div className="mt-3 flex items-center justify-end border-t border-border pt-3">
          <ServerLifecycleActions gs={gs} phase={phase} onAct={onAct} />
        </div>
      )}
    </Card>
  );
}

function StatChip({ icon, label, value }: { icon: ReactNode; label: string; value: ReactNode }) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-md bg-surface px-2 py-1 text-[11px] text-muted">
      {icon}
      {label}
      <span className="font-mono text-fg">{value}</span>
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
    <button
      title={title}
      onClick={onClick}
      disabled={disabled}
      className={cn(
        "rounded p-1.5 text-muted transition-colors hover:bg-border/60 hover:text-fg",
        "disabled:opacity-40 disabled:pointer-events-none",
      )}
    >
      {children}
    </button>
  );
}
