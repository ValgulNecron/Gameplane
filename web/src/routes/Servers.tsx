import { useMemo, useState, type ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Activity,
  Cpu,
  Filter,
  HardDrive,
  MoreHorizontal,
  Play,
  Plus,
  RotateCw,
  Search,
  Server as ServerIcon,
  Square,
  Users as UsersIcon,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { PhaseBadge } from "@/components/ui/badge";
import { StatCard } from "@/components/ui/stat";
import { TabBar } from "@/components/ui/tabs";
import { GameIcon } from "@/components/ui/game-icon";
import { PageHeader } from "@/components/PageHeader";
import { cn, formatBytes } from "@/lib/utils";
import type { ClusterStats, ClusterView, GameServer } from "@/types";
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

  const act = useMutation({
    mutationFn: (args: { name: string; verb: LifecycleVerb }) =>
      Servers.lifecycle(args.name, args.verb),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["servers"] }),
  });

  const [filter, setFilter] = useState<FilterKey>("all");
  const [query, setQuery] = useState("");

  const servers = useMemo(() => data?.items ?? [], [data?.items]);
  const counts = useMemo(() => countByState(servers), [servers]);
  const vcpus = (clusterView?.nodes ?? []).reduce((s, n) => s + (n.cpu?.capacity ?? 0), 0);

  const visible = servers.filter((gs) => {
    if (query && !gs.metadata.name.toLowerCase().includes(query.toLowerCase())) return false;
    const phase = gs.status?.phase;
    if (filter === "running") return phase === "Running";
    if (filter === "stopped") return phase === "Stopped" || phase === "Suspended" || phase === "Failed";
    return true;
  });

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
          <Button variant="outline" size="default">
            <Filter className="h-4 w-4" /> Filter
          </Button>
        </div>
      </div>

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
            {!isLoading && visible.length === 0 && (
              <tr><td className="px-4 py-12 text-center text-muted" colSpan={8}>
                No servers match.
              </td></tr>
            )}
            {visible.map((gs) => <ServerRow key={gs.metadata.name} gs={gs} onAct={act.mutate} />)}
          </tbody>
        </table>
      </div>
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
  const phase = gs.status?.phase;
  const players = gs.status?.agent?.playersOnline;
  const maxPlayers = gs.status?.agent?.playersMax;
  const node = (gs.status as unknown as { node?: string })?.node
    ?? gs.metadata.annotations?.["gameplane.local/node"];
  const cpu = (gs.status as unknown as { cpuPercent?: number })?.cpuPercent;
  const mem = (gs.status as unknown as { memoryBytes?: number })?.memoryBytes;

  return (
    <tr className="hover:bg-surface/40">
      <td className="px-4 py-3">
        <div className="flex items-center gap-3">
          <GameIcon game={gs.spec.templateRef.name} size="sm" />
          <div className="min-w-0">
            <Link
              to="/servers/$name"
              params={{ name: gs.metadata.name }}
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
      <td className="px-4 py-3 font-mono">
        {typeof cpu === "number" ? `${cpu.toFixed(1)}%` : "—"}
      </td>
      <td className="px-4 py-3 font-mono">
        {typeof mem === "number" ? formatBytes(mem) : "—"}
      </td>
      <td className="px-4 py-3 font-mono">
        {typeof players === "number" ? `${players}/${maxPlayers ?? "—"}` : "—"}
      </td>
      <td className="px-4 py-3 font-mono text-muted">{node ?? "—"}</td>
      <td className="px-4 py-3 text-right">
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
          <ActionButton title="More">
            <MoreHorizontal className="h-4 w-4" />
          </ActionButton>
        </div>
      </td>
    </tr>
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
