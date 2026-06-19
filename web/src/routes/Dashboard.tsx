import { useMemo } from "react";
import { Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import {
  Activity,
  Archive,
  CheckCircle2,
  Cpu,
  HardDrive,
  Pencil,
  Play,
  Plus,
  RotateCw,
  Server as ServerIcon,
  Square,
  Trash2,
  UserPlus,
  Users as UsersIcon,
  type LucideIcon,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { StatCard } from "@/components/ui/stat";
import { PhaseBadge } from "@/components/ui/badge";
import { Meter } from "@/components/ui/meter";
import { GameIcon } from "@/components/ui/game-icon";
import { PageHeader } from "@/components/PageHeader";
import { cn, formatBytes, formatRelative } from "@/lib/utils";
import { useMe, can } from "@/lib/auth";
import { Audit, Backups, Cluster, Servers } from "@/lib/endpoints";
import { countByState, phaseGroups, type PhaseGroups } from "@/lib/servers";
import type {
  AuditEvent,
  Backup,
  ClusterNode,
  ClusterStats,
  ClusterView,
  GameServer,
} from "@/types";

export function DashboardPage() {
  const { data: me } = useMe();
  const canAudit = can(me, "audit:read");
  const canCluster = can(me, "servers:write");

  const { data: serversData } = useQuery({
    queryKey: ["servers"],
    queryFn: () => Servers.list(),
    refetchInterval: 5_000,
  });
  const { data: stats } = useQuery({
    queryKey: ["cluster-stats"],
    queryFn: () => Cluster.stats().catch(() => ({}) as ClusterStats),
    staleTime: 30_000,
  });
  const { data: clusterView } = useQuery({
    queryKey: ["cluster"],
    queryFn: () => Cluster.view().catch(() => ({}) as ClusterView),
    staleTime: 30_000,
  });
  const { data: backupsData } = useQuery({
    queryKey: ["backups"],
    queryFn: () => Backups.list().catch(() => ({ items: [] as Backup[] })),
    staleTime: 30_000,
  });
  const { data: audit } = useQuery({
    queryKey: ["audit", "dashboard"],
    queryFn: () => Audit.page(8, 0),
    enabled: canAudit,
    staleTime: 30_000,
  });

  const servers = useMemo(() => serversData?.items ?? [], [serversData?.items]);
  const counts = useMemo(() => countByState(servers), [servers]);
  const groups = useMemo(() => phaseGroups(servers), [servers]);
  const recentBackups = useMemo(
    () => sortBackups(backupsData?.items ?? []).slice(0, 6),
    [backupsData?.items],
  );

  const nodes = clusterView?.nodes ?? [];
  const nodesReady = clusterView?.ready ?? nodes.filter((n) => n.status === "Ready").length;
  const nodesTotal = clusterView?.total ?? stats?.nodes ?? nodes.length;
  const cpu = sumUsage(nodes, "cpu");
  const mem = sumUsage(nodes, "memory");
  const vcpus = nodes.reduce((sum, n) => sum + (n.cpu?.capacity ?? 0), 0);
  const storagePct = stats?.totalStorageBytes
    ? ((stats.usedStorageBytes ?? 0) / stats.totalStorageBytes) * 100
    : 0;

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Dashboard"
        subtitle="At-a-glance health of your Kestrel cluster."
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
          sub={`of ${groups.total} total`}
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
          value={formatBytes(stats?.usedStorageBytes ?? 0)}
          sub={stats?.totalStorageBytes ? `of ${formatBytes(stats.totalStorageBytes)}` : "—"}
          accent="violet"
        />
        <StatCard
          label="Nodes ready"
          icon={<ServerIcon className="h-4 w-4" />}
          value={nodesTotal > 0 ? `${nodesReady}/${nodesTotal}` : "—"}
          sub={nodesTotal === 0 ? "no node data" : nodesReady === nodesTotal ? "all healthy" : "needs attention"}
          accent="warning"
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <FleetStatusCard groups={groups} />
        <ClusterResourcesCard
          cpu={cpu}
          mem={mem}
          storagePct={storagePct}
          nodes={nodes}
          nodesReady={nodesReady}
          nodesTotal={nodesTotal}
          canViewCluster={canCluster}
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        {canAudit && <RecentActivityCard events={audit ?? []} />}
        <RecentBackupsCard backups={recentBackups} />
      </div>
    </div>
  );
}

function FleetStatusCard({ groups }: { groups: PhaseGroups }) {
  const stopped = groups.stopped + groups.other;
  const segments = [
    { n: groups.running, cls: "bg-success" },
    { n: stopped, cls: "bg-muted" },
    { n: groups.failed, cls: "bg-danger" },
  ].filter((s) => s.n > 0);
  const attention = groups.attention.slice(0, 4);

  return (
    <Card className="space-y-4 p-5">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-fg">Fleet status</h3>
        <span className="text-xs text-muted">{groups.total} servers</span>
      </div>

      <div className="flex h-2 overflow-hidden rounded-full bg-surface">
        {groups.total > 0 &&
          segments.map((s, i) => (
            <div key={i} className={s.cls} style={{ width: `${(s.n / groups.total) * 100}%` }} />
          ))}
      </div>

      <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-xs">
        <LegendDot cls="bg-success" label={`Running ${groups.running}`} />
        <LegendDot cls="bg-muted" label={`Stopped ${stopped}`} />
        <LegendDot cls="bg-danger" label={`Failed ${groups.failed}`} />
      </div>

      <div className="space-y-3 border-t border-border pt-4">
        <div className="text-[10px] font-medium uppercase tracking-wider text-muted">
          Needs attention
        </div>
        {attention.length === 0 ? (
          <div className="flex items-center gap-2 text-sm text-muted">
            <CheckCircle2 className="h-4 w-4 text-success" /> Everything looks healthy.
          </div>
        ) : (
          attention.map((gs) => <AttentionRow key={gs.metadata.name} gs={gs} />)
        )}
      </div>
    </Card>
  );
}

function AttentionRow({ gs }: { gs: GameServer }) {
  const phase = gs.status?.phase;
  // phaseGroups.attention only ever holds Failed-phase or stale-agent
  // servers, so the reason is one of exactly these two.
  const reason = phase === "Failed" ? "Failed — check logs" : "Agent heartbeat stale";
  return (
    <Link
      to="/servers/$name"
      params={{ name: gs.metadata.name }}
      className="group flex items-center gap-3"
    >
      <GameIcon game={gs.spec.templateRef.name} size="sm" />
      <div className="min-w-0 flex-1">
        <div className="truncate font-mono text-sm text-fg group-hover:text-primary">
          {gs.metadata.name}
        </div>
        <div className="truncate text-[11px] text-muted">{reason}</div>
      </div>
      <PhaseBadge phase={phase} />
    </Link>
  );
}

interface Usage {
  pct: number;
  sub?: string;
}

function ClusterResourcesCard({
  cpu,
  mem,
  storagePct,
  nodes,
  nodesReady,
  nodesTotal,
  canViewCluster,
}: {
  cpu: Usage;
  mem: Usage;
  storagePct: number;
  nodes: ClusterNode[];
  nodesReady: number;
  nodesTotal: number;
  canViewCluster: boolean;
}) {
  return (
    <Card className="space-y-4 p-5">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-fg">Cluster resources</h3>
        {canViewCluster ? (
          <Link to="/cluster" className="text-xs text-primary hover:underline">
            View cluster
          </Link>
        ) : (
          <span className="text-xs text-muted">
            {nodesTotal > 0 ? `${nodesReady}/${nodesTotal} nodes ready` : "—"}
          </span>
        )}
      </div>

      <div className="space-y-3">
        <Meter label="CPU" pct={cpu.pct} sub={cpu.sub} accent="primary" />
        <Meter label="Memory" pct={mem.pct} sub={mem.sub} accent="violet" />
        <Meter label="Storage" pct={storagePct} accent="success" />
      </div>

      {nodes.length > 0 && (
        <div className="space-y-2 border-t border-border pt-4">
          <div className="text-[10px] font-medium uppercase tracking-wider text-muted">Nodes</div>
          {nodes.slice(0, 4).map((n) => <NodeRow key={n.name} node={n} />)}
        </div>
      )}
    </Card>
  );
}

function NodeRow({ node }: { node: ClusterNode }) {
  const ready = node.status === "Ready";
  const cpuPct = pctOf(node.cpu?.used, node.cpu?.capacity);
  const meta = [
    node.pods ? `${node.pods.used ?? 0} pods` : null,
    node.cpu?.capacity ? `cpu ${Math.round(cpuPct)}%` : null,
  ].filter(Boolean).join(" · ");
  return (
    <div className="flex items-center gap-2 text-xs">
      <span className={cn("h-2 w-2 shrink-0 rounded-full", ready ? "bg-success" : "bg-danger")} />
      <span className="flex-1 truncate font-mono text-fg">{node.name}</span>
      <span className="shrink-0 text-muted">{meta || "—"}</span>
    </div>
  );
}

function RecentActivityCard({ events }: { events: AuditEvent[] }) {
  return (
    <Card className="space-y-4 p-5">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-fg">Recent activity</h3>
        <Link to="/admin/audit" className="text-xs text-primary hover:underline">
          View all
        </Link>
      </div>
      {events.length === 0 ? (
        <div className="py-6 text-center text-sm text-muted">No recent activity.</div>
      ) : (
        <div className="space-y-3">
          {events.map((e) => <ActivityRow key={e.id} event={e} />)}
        </div>
      )}
    </Card>
  );
}

function ActivityRow({ event }: { event: AuditEvent }) {
  const { icon: Icon, accent } = actionIcon(event);
  return (
    <div className="flex items-center gap-3">
      <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-surface">
        <Icon className={cn("h-3.5 w-3.5", accent)} />
      </div>
      <div className="min-w-0 flex-1">
        <div className="truncate text-sm text-fg">{describeAudit(event)}</div>
        <div className="truncate font-mono text-[11px] text-muted">
          {event.method} {event.path}
        </div>
      </div>
      <span className="shrink-0 text-xs text-muted">{formatRelative(event.ts)}</span>
    </div>
  );
}

function RecentBackupsCard({ backups }: { backups: Backup[] }) {
  return (
    <Card className="space-y-4 p-5">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-fg">Recent backups</h3>
        <Link to="/backups" className="text-xs text-primary hover:underline">
          View all
        </Link>
      </div>
      {backups.length === 0 ? (
        <div className="py-6 text-center text-sm text-muted">No backups yet.</div>
      ) : (
        <div className="space-y-3">
          {backups.map((b) => <BackupRow key={b.metadata.name} backup={b} />)}
        </div>
      )}
    </Card>
  );
}

function BackupRow({ backup }: { backup: Backup }) {
  const when = backup.status?.completionTime ?? backup.status?.startTime;
  return (
    <div className="flex items-center gap-3">
      <GameIcon game={backup.spec.serverRef.name} size="sm" />
      <div className="min-w-0 flex-1">
        <div className="truncate font-mono text-sm text-fg">{backup.spec.serverRef.name}</div>
        <div className="truncate text-[11px] text-muted">{formatRelative(when)}</div>
      </div>
      {backup.status?.size && (
        <span className="shrink-0 font-mono text-xs text-fg">{backup.status.size}</span>
      )}
      <PhaseBadge phase={backup.status?.phase} />
    </div>
  );
}

function LegendDot({ cls, label }: { cls: string; label: string }) {
  return (
    <span className="flex items-center gap-1.5 text-muted">
      <span className={cn("h-2 w-2 rounded-full", cls)} />
      {label}
    </span>
  );
}

// sumUsage aggregates a resource across nodes into a percentage and a
// human-readable "used / capacity" subtitle (bytes for memory, cores for CPU).
function sumUsage(nodes: ClusterNode[], key: "cpu" | "memory"): Usage {
  let used = 0;
  let cap = 0;
  for (const n of nodes) {
    used += n[key]?.used ?? 0;
    cap += n[key]?.capacity ?? 0;
  }
  const pct = cap > 0 ? (used / cap) * 100 : 0;
  if (cap === 0) return { pct };
  const sub = key === "memory" ? `${formatBytes(used)} / ${formatBytes(cap)}` : `${used} / ${cap} cores`;
  return { pct, sub };
}

function pctOf(used?: number, cap?: number): number {
  if (!used || !cap) return 0;
  return (used / cap) * 100;
}

function sortBackups(items: Backup[]): Backup[] {
  return [...items].sort((a, b) =>
    (b.status?.startTime ?? "").localeCompare(a.status?.startTime ?? ""),
  );
}

// describeAudit turns an audit row into a one-line human summary. Lifecycle
// sub-resource paths (":start" etc.) get a specific verb; otherwise the verb
// derives from the HTTP method.
function describeAudit(e: AuditEvent): string {
  const target = e.target ? ` ${e.target}` : "";
  if (e.path.includes(":start")) return `${e.actor} started${target}`;
  if (e.path.includes(":stop")) return `${e.actor} stopped${target}`;
  if (e.path.includes(":restart")) return `${e.actor} restarted${target}`;
  if (e.path.includes("/backups")) return `${e.actor} backed up${target}`;
  if (e.path.includes("/users")) return `${e.actor} updated a user${target}`;
  const verb =
    e.method === "DELETE"
      ? "deleted"
      : e.method === "POST"
        ? "created"
        : e.method === "PUT" || e.method === "PATCH"
          ? "updated"
          : e.method.toLowerCase();
  return `${e.actor} ${verb}${target}`.trim();
}

function actionIcon(e: AuditEvent): { icon: LucideIcon; accent: string } {
  if (e.path.includes(":start")) return { icon: Play, accent: "text-success" };
  if (e.path.includes(":stop")) return { icon: Square, accent: "text-muted" };
  if (e.path.includes(":restart")) return { icon: RotateCw, accent: "text-warning" };
  if (e.path.includes("/backups")) return { icon: Archive, accent: "text-violet" };
  if (e.path.includes("/users")) return { icon: UserPlus, accent: "text-primary" };
  if (e.method === "DELETE") return { icon: Trash2, accent: "text-danger" };
  if (e.method === "POST") return { icon: Plus, accent: "text-success" };
  return { icon: Pencil, accent: "text-muted" };
}
