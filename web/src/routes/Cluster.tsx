import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Download, Plus, Server as ServerIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Meter } from "@/components/ui/meter";
import { PageHeader } from "@/components/PageHeader";
import { cn, formatBytes, formatUptime } from "@/lib/utils";
import { APIError } from "@/lib/api";
import type { ClusterInfo, ClusterNode, ClusterView, NodeJoinInfo } from "@/types";
import { Cluster } from "@/lib/endpoints";

// opMessage turns a cluster-op error into user copy — 501 means the
// operator hasn't enabled clusterOps.
function opMessage(e: unknown): string {
  if (e instanceof APIError && e.status === 501) {
    return "Cluster operations aren't enabled on this install (set clusterOps.enabled).";
  }
  if (e instanceof APIError && e.status === 403) return "You don't have permission to do that.";
  return "That operation failed. Check the API logs.";
}

export function ClusterPage() {
  const { data } = useQuery({
    queryKey: ["cluster"],
    queryFn: () => Cluster.view().catch(() => ({} as ClusterView)),
    refetchInterval: 15_000,
  });
  const { data: info } = useQuery({
    queryKey: ["cluster-info"],
    queryFn: () => Cluster.info().catch(() => ({} as ClusterInfo)),
  });
  // Only an explicit false disables the buttons — an older API (or a
  // failed info fetch) leaves them active, and the 501 opMessage still
  // catches the disabled case as a backstop.
  const opsDisabled = info?.clusterOps === false;

  const nodes = data?.nodes ?? [];
  const [joinInfo, setJoinInfo] = useState<NodeJoinInfo | null>(null);
  const [opError, setOpError] = useState<string | null>(null);

  const addNode = useMutation({
    mutationFn: () => Cluster.addNode(),
    onSuccess: (info) => {
      setOpError(null);
      setJoinInfo(info);
    },
    onError: (e) => setOpError(opMessage(e)),
  });

  async function downloadKubeconfig() {
    setOpError(null);
    try {
      const blob = await Cluster.kubeconfig();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "gameplane-kubeconfig.yaml";
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch (e) {
      setOpError(opMessage(e));
    }
  }

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
          <div className="flex flex-col items-end gap-1">
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                onClick={() => void downloadKubeconfig()}
                disabled={opsDisabled}
                title={opsDisabled ? "Cluster operations are disabled on this install." : undefined}
              >
                <Download className="h-4 w-4" /> Download kubeconfig
              </Button>
              <Button
                onClick={() => addNode.mutate()}
                disabled={addNode.isPending || opsDisabled}
                title={opsDisabled ? "Cluster operations are disabled on this install." : undefined}
              >
                <Plus className="h-4 w-4" /> Add node
              </Button>
            </div>
            {opsDisabled && (
              <p className="text-xs text-muted">
                Enable <code className="font-mono">clusterOps.enabled</code> in the Helm values
                to mint node-join tokens and kubeconfigs.
              </p>
            )}
          </div>
        }
      />

      {opError && (
        <Card className="border-danger/40 bg-danger/5 p-4 text-sm text-danger">{opError}</Card>
      )}

      {joinInfo && (
        <Card className="space-y-2 p-4">
          <div className="flex items-center justify-between">
            <div className="text-sm font-medium">Join a node</div>
            <button
              type="button"
              onClick={() => setJoinInfo(null)}
              className="text-xs text-muted hover:text-fg"
            >
              Dismiss
            </button>
          </div>
          <p className="text-xs text-muted">
            Run this on the machine you want to join (token expires {joinInfo.expiresAt}):
          </p>
          <div className="flex items-center gap-2">
            <code className="flex-1 overflow-auto rounded bg-[#0b0b0d] px-3 py-2 font-mono text-xs text-fg">
              {joinInfo.command}
            </code>
            <Button
              variant="outline"
              size="sm"
              onClick={() => void navigator.clipboard?.writeText(joinInfo.command)}
            >
              Copy
            </Button>
          </div>
        </Card>
      )}

      {nodes.length === 0 && (
        <Card className="flex flex-col items-center justify-center gap-3 p-12 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 text-primary">
            <ServerIcon className="h-5 w-5" />
          </div>
          <div>
            <div className="font-medium">No node data yet.</div>
            <div className="pt-1 text-sm text-muted">
              No nodes are reporting yet — check the API and operator connection.
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
  const cpuKnown = node.cpu?.used !== undefined;
  const memKnown = node.memory?.used !== undefined;
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
        <Stat label="Pods" value={node.pods?.used !== undefined ? `${node.pods.used}/${node.pods.capacity ?? 0}` : node.pods?.capacity ? `of ${node.pods.capacity}` : "—"} />
        <Stat label="CPU" value={node.cpu?.capacity ? `${node.cpu.capacity} cores` : "—"} />
      </div>

      <div className="space-y-2">
        <Meter label="CPU" pct={cpuPct} unknown={!cpuKnown} accent="primary" />
        <Meter
          label="Memory"
          pct={memPct}
          unknown={!memKnown}
          sub={
            node.memory?.capacity
              ? memKnown
                ? `${formatBytes(node.memory.used ?? 0)} / ${formatBytes(node.memory.capacity)}`
                : `of ${formatBytes(node.memory.capacity)}`
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

function pct(u?: number, cap?: number): number {
  if (u === undefined || !cap) return 0;
  return (u / cap) * 100;
}
