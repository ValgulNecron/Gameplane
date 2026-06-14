import { useQuery } from "@tanstack/react-query";
import { Download, Plus, Server as ServerIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { PageHeader } from "@/components/PageHeader";
import { cn, formatBytes, formatUptime } from "@/lib/utils";
import type { ClusterNode, ClusterView } from "@/types";
import { Cluster } from "@/lib/endpoints";

export function ClusterPage() {
  const { data } = useQuery({
    queryKey: ["cluster"],
    queryFn: () => Cluster.view().catch(() => ({} as ClusterView)),
    refetchInterval: 15_000,
  });

  const nodes = data?.nodes ?? [];

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Cluster"
        subtitle={
          data && (data.name || data.version || nodes.length > 0)
            ? `${data.name || "—"} · ${data.version || "—"} · ${data.ready ?? nodes.filter(n => n.status === "Ready").length}/${data.total ?? nodes.length} nodes healthy`
            : "Node inventory and health across the control plane."
        }
        actions={
          <div className="flex items-center gap-2">
            <Button variant="outline">
              <Download className="h-4 w-4" /> Download kubeconfig
            </Button>
            <Button>
              <Plus className="h-4 w-4" /> Add node
            </Button>
          </div>
        }
      />

      {nodes.length === 0 && (
        <Card className="flex flex-col items-center justify-center gap-3 p-12 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 text-primary">
            <ServerIcon className="h-5 w-5" />
          </div>
          <div>
            <div className="font-medium">No node data yet.</div>
            <div className="pt-1 text-sm text-muted">
              Hook up the <code className="font-mono text-xs">/cluster</code> endpoint to
              populate nodes here.
            </div>
          </div>
        </Card>
      )}

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {nodes.map((n) => <NodeCard key={n.name} node={n} />)}
      </div>
    </div>
  );
}

function NodeCard({ node }: { node: ClusterNode }) {
  const cpuPct = pct(node.cpu?.used, node.cpu?.capacity);
  const memPct = pct(node.memory?.used, node.memory?.capacity);
  const ready = node.status === "Ready";
  const uptime = node.uptime ?? formatUptime(node.startedAt);
  return (
    <Card className="p-4 space-y-4">
      <div className="flex items-start justify-between">
        <div className="flex items-start gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-md bg-surface">
            <ServerIcon className="h-4 w-4 text-muted" />
          </div>
          <div>
            <div className="font-mono text-sm font-semibold">{node.name}</div>
            <div className="pt-0.5 text-[11px] text-muted">
              {node.roles?.join(", ") || "worker"}
            </div>
          </div>
        </div>
        <span className={cn(
          "rounded-full px-2 py-0.5 text-[10px] font-mono uppercase",
          ready
            ? "bg-success/15 text-success"
            : "bg-danger/15 text-danger",
        )}>
          ● {node.status ?? "Unknown"}
        </span>
      </div>

      <div className="grid grid-cols-3 gap-3 text-xs">
        <Stat label="Uptime" value={uptime} />
        <Stat label="Pods" value={node.pods ? `${node.pods.used ?? 0}/${node.pods.capacity ?? 0}` : "—"} />
        <Stat label="CPU" value={node.cpu?.capacity ? `${node.cpu.capacity} cores` : "—"} />
      </div>

      <div className="space-y-2">
        <Bar label="CPU" pct={cpuPct} accent="primary" />
        <Bar
          label="Memory"
          pct={memPct}
          sub={
            node.memory?.capacity
              ? `${formatBytes(node.memory.used ?? 0)} / ${formatBytes(node.memory.capacity)}`
              : undefined
          }
          accent="violet"
        />
      </div>

      {node.labels && node.labels.length > 0 && (
        <div className="flex flex-wrap gap-1">
          {node.labels.map((l) => (
            <span key={l} className="rounded bg-surface px-2 py-0.5 text-[10px] font-mono text-muted">
              {l}
            </span>
          ))}
        </div>
      )}
    </Card>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border border-border bg-surface/60 p-2">
      <div className="text-[10px] uppercase tracking-wider text-muted">{label}</div>
      <div className="pt-0.5 font-mono text-xs text-fg">{value}</div>
    </div>
  );
}

function Bar({
  label, pct, sub, accent,
}: { label: string; pct: number; sub?: string; accent: "primary" | "violet" }) {
  const color = accent === "primary" ? "bg-primary" : "bg-violet";
  return (
    <div>
      <div className="flex items-center justify-between text-[11px]">
        <span className="text-muted">{label}</span>
        <span className="font-mono text-fg">{Math.round(pct)}%</span>
      </div>
      <div className="mt-1 h-1.5 overflow-hidden rounded-full bg-surface">
        <div className={`h-full ${color}`} style={{ width: `${Math.min(100, pct)}%` }} />
      </div>
      {sub && <div className="pt-1 text-[10px] text-muted">{sub}</div>}
    </div>
  );
}

function pct(u?: number, cap?: number): number {
  if (!u || !cap) return 0;
  return (u / cap) * 100;
}
