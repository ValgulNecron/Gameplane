import { useEffect, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { Copy, Cpu, HardDrive, MemoryStick, AlertCircle } from "lucide-react";
import type { GameServer, GameTemplate, PlayersResp } from "@/types";
import { Players, Servers } from "@/lib/endpoints";
import { Card, CardHeader, CardTitle, CardContent, CardFooter, Alert } from "@heroui/react";
import { StatCard } from "@/components/hero/StatCard";
import { Sparkline } from "@/components/hero/Sparkline";
import { ServerActionsCard } from "@/components/server/ServerActionsCard";
import { ServerStatusCard } from "@/components/server/ServerStatusCard";
import { ServerSleepCard } from "@/components/server/ServerSleepCard";
import { EventList } from "@/components/server/EventList";
import { formatBytes } from "@/lib/utils";
import { mapServerEvent, type NormalizedServerEvent } from "@/lib/events";

export function OverviewTab({
  gs,
  name,
  tmpl,
  ns,
  onViewAllEvents,
}: {
  gs?: GameServer;
  name: string;
  tmpl?: GameTemplate;
  ns?: string;
  onViewAllEvents?: () => void;
}) {
  const { data: roster } = useQuery({
    queryKey: ["players", name, "overview", ns],
    queryFn: () => Players.snapshot(name, ns),
    enabled: !!name && (gs?.status?.phase === "Running"),
    refetchInterval: 10_000,
    retry: false,
  });

  // Kubernetes events for the pod/StatefulSet/GameServer — image pull,
  // scheduling, crash-loops. Poll faster while not Running so provisioning
  // diagnostics stay fresh; back off once the server is up.
  const { data: rawEvents } = useQuery({
    queryKey: ["events", name, ns],
    queryFn: () => Servers.events(name, ns),
    enabled: !!name,
    refetchInterval: gs?.status?.phase === "Running" ? 30_000 : 5_000,
    retry: false,
  });

  if (!gs) return <div className="p-6 text-muted">Loading…</div>;

  const status = gs.status ?? {};
  const running = status.phase === "Running";
  const agent = status.agent;
  // Resource usage comes from the agent's heartbeat (cgroup + statfs).
  // null/undefined means "unknown" (unreadable source, or a stale
  // heartbeat the API blanked) — render "—", not a misleading 0.
  const cpuKnown = typeof agent?.cpuMillicores === "number";
  const cpuMilli = cpuKnown ? (agent?.cpuMillicores as number) : 0;
  const cpuLimitMilli =
    typeof agent?.cpuLimitMillicores === "number" ? (agent?.cpuLimitMillicores as number) : 0;
  const cpuPct = cpuKnown && cpuLimitMilli ? (cpuMilli / cpuLimitMilli) * 100 : 0;

  const memKnown = typeof agent?.memoryBytes === "number";
  const memUsed = memKnown ? (agent?.memoryBytes as number) : 0;
  const memLimit =
    typeof agent?.memoryLimitBytes === "number" ? (agent?.memoryLimitBytes as number) : 0;
  const memPct = memKnown && memLimit ? (memUsed / memLimit) * 100 : 0;

  const diskKnown =
    typeof agent?.diskUsedBytes === "number" && typeof agent?.diskTotalBytes === "number";
  const diskUsed = diskKnown ? (agent?.diskUsedBytes as number) : 0;
  const diskTotal = diskKnown ? (agent?.diskTotalBytes as number) : 0;
  const diskPct = diskKnown && diskTotal ? (diskUsed / diskTotal) * 100 : 0;

  // Player count is shown in the Players card (sidebar). null/undefined
  // means "unknown" (RCON unavailable or a stale heartbeat the API
  // blanked); fall back to 0 for the card's header.
  const players =
    typeof agent?.playersOnline === "number" && agent.playersOnline >= 0
      ? (agent.playersOnline as number)
      : 0;
  const events: NormalizedServerEvent[] = (Array.isArray(rawEvents) ? rawEvents : []).map(
    mapServerEvent,
  );
  const endpoints = status.endpoints ?? [];
  const primary = endpoints[0];
  // The operator prepends tunnel endpoints ahead of the cluster-native ones
  // (see gameserver_status.go), so when a tunnel is active `endpoints[0]` is
  // the tunnel endpoint — the cluster address is the first entry after it
  // that isn't itself tunnel-provided.
  const clusterEndpoint = primary?.tunnelProvider
    ? endpoints.find((e) => !e.tunnelProvider)
    : primary;
  const tunnelReady = status.conditions?.find((c) => c.type === "TunnelReady");

  // Check for provisioning failures
  const provisioningFailure = status.conditions?.find(
    (c) => c.reason?.includes("Provisioning") || c.reason?.includes("Storage") || c.reason?.includes("PVC")
  );

  return (
    <div className="space-y-5 p-6">
      {/* Provisioning failure alert */}
      {provisioningFailure && (
        <Alert
          status="warning"
          className="flex items-start gap-3"
        >
          <Alert.Indicator className="mt-0.5">
            <AlertCircle className="h-5 w-5 shrink-0 text-warning" />
          </Alert.Indicator>
          <Alert.Content className="flex flex-1 flex-col gap-1">
            <Alert.Title className="font-semibold text-sm">
              Waiting on storage — the server can't provision yet
            </Alert.Title>
            <Alert.Description className="text-sm">
              Ready condition: {provisioningFailure.reason} — {provisioningFailure.message}
            </Alert.Description>
          </Alert.Content>
        </Alert>
      )}

      <div className="grid gap-5 lg:grid-cols-[1fr_320px]">
        <div className="space-y-5">
          <div className="grid gap-4 md:grid-cols-3">
            <ResourceCard
              label="CPU"
              icon={<Cpu className="h-4 w-4" />}
              value={
                cpuKnown ? (cpuLimitMilli ? `${cpuPct.toFixed(0)}%` : `${(cpuMilli / 1000).toFixed(2)} cores`) : "—"
              }
              sub={
                cpuKnown && cpuLimitMilli
                  ? `${(cpuMilli / 1000).toFixed(1)} / ${(cpuLimitMilli / 1000).toFixed(1)} cores`
                  : undefined
              }
              progress={cpuKnown && cpuLimitMilli ? cpuPct : undefined}
              trend={cpuKnown && cpuLimitMilli ? cpuPct : undefined}
              accent="primary"
            />
            <ResourceCard
              label="Memory"
              icon={<MemoryStick className="h-4 w-4" />}
              value={memKnown ? (memLimit ? `${memPct.toFixed(0)}%` : formatBytes(memUsed)) : "—"}
              sub={
                memKnown ? `${formatBytes(memUsed)} / ${memLimit ? formatBytes(memLimit) : "—"}` : undefined
              }
              progress={memKnown && memLimit ? memPct : undefined}
              trend={memKnown && memLimit ? memPct : undefined}
              accent="violet"
            />
            <ResourceCard
              label="Disk"
              icon={<HardDrive className="h-4 w-4" />}
              value={diskKnown ? (diskTotal ? `${diskPct.toFixed(0)}%` : formatBytes(diskUsed)) : "—"}
              sub={
                diskKnown ? `${formatBytes(diskUsed)} / ${diskTotal ? formatBytes(diskTotal) : "—"}` : undefined
              }
              progress={diskKnown && diskTotal ? diskPct : undefined}
              trend={diskKnown && diskTotal ? diskPct : undefined}
              accent="success"
            />
          </div>

          <Card className="border border-border bg-surface">
            <CardHeader className="flex flex-col gap-3 border-b border-border p-4">
              <div className="flex items-center justify-between">
                <h3 className="text-lg font-semibold text-foreground">Recent events</h3>
                <button
                  className="text-xs font-medium text-accent hover:underline"
                  onClick={onViewAllEvents}
                >
                  View all
                </button>
              </div>
            </CardHeader>
            <CardContent className="px-0">
              <EventList
                events={events}
                emptyMessage="No events yet. Lifecycle, backup, and agent activity will appear here."
              />
            </CardContent>
          </Card>
        </div>

        <div className="space-y-5">
          <Card className="border border-border bg-surface">
            <CardHeader className="border-b border-border px-4 py-3">
              <h3 className="text-lg font-semibold text-foreground">Connection</h3>
            </CardHeader>
            <CardContent className="space-y-3 px-4 py-3 text-sm">
              {/* Show tunnel endpoint first if it exists */}
              {primary?.tunnelProvider && (
                <>
                  <EndpointRow label={`${primary.tunnelProvider} tunnel`}>
                    {primary?.host ? (
                      <>
                        <div className="flex min-w-0 flex-1 items-center gap-2">
                          <span className="truncate font-mono text-foreground">{primary.host}</span>
                          {primary?.private && (
                            <span className="shrink-0 rounded-full bg-warning/20 px-2 py-0.5 text-[10px] font-medium text-warning">
                              Tailnet only
                            </span>
                          )}
                        </div>
                        <button
                          className="rounded p-1 text-muted hover:bg-border hover:text-foreground"
                          onClick={() => navigator.clipboard?.writeText(primary.host)}
                          title="Copy"
                        >
                          <Copy className="h-3.5 w-3.5" />
                        </button>
                      </>
                    ) : (
                      <span className="text-muted italic">
                        {tunnelReady?.message ?? "Waiting for tunnel address…"}
                      </span>
                    )}
                  </EndpointRow>
                  {primary?.port !== undefined && (
                    <EndpointRow label="Port">
                      <span className="font-mono text-foreground">{primary.port}</span>
                    </EndpointRow>
                  )}
                  {/* Show cluster address below */}
                  <div className="border-t border-border pt-3">
                    <div className="text-xs text-muted mb-3">CLUSTER ADDRESS</div>
                    <EndpointRow label="Host">
                      <span className="truncate font-mono text-foreground">{clusterEndpoint?.host ?? "—"}</span>
                    </EndpointRow>
                  </div>
                </>
              )}
              {/* No tunnel: a single address row, annotated with its pool when
                  the operator bound one (there is only one host to show). */}
              {!primary?.tunnelProvider && (
                <>
                  <EndpointRow label="Host">
                    <div className="flex min-w-0 flex-1 flex-col gap-1">
                      <span className="truncate font-mono text-foreground">{primary?.host ?? "—"}</span>
                      {primary?.pool && (
                        <span className="text-xs text-muted">
                          from pool &apos;{primary.pool}&apos;
                        </span>
                      )}
                    </div>
                    {primary?.host && (
                      <button
                        className="rounded p-1 text-muted hover:bg-border hover:text-foreground"
                        onClick={() => navigator.clipboard?.writeText(primary.host)}
                        title="Copy"
                      >
                        <Copy className="h-3.5 w-3.5" />
                      </button>
                    )}
                  </EndpointRow>
                  <EndpointRow label="Port">
                    <span className="font-mono text-foreground">{primary?.port ?? "—"}</span>
                  </EndpointRow>
                </>
              )}
            </CardContent>
          </Card>

          <PlayersCard roster={roster} fallbackOnline={players} />

          <ServerStatusCard name={name} tmpl={tmpl} running={running} />

          <ServerSleepCard gs={gs} />

          <ServerActionsCard name={name} tmpl={tmpl} />
        </div>
      </div>
    </div>
  );
}

// Accumulate a rolling window of a metric's samples so the tile can draw a
// trend. Appends only when the value actually changes (the parent re-renders
// on every poll), capped to the last `max` points.
function useMetricHistory(value: number | undefined, max = 32): number[] {
  const [hist, setHist] = useState<number[]>([]);
  useEffect(() => {
    if (value === undefined || Number.isNaN(value)) return;
    const appendSample = () => {
      setHist((h) => {
        const next = h.concat(value);
        return next.length > max ? next.slice(-max) : next;
      });
    };
    appendSample();
  }, [value, max]);
  return hist;
}

function ResourceCard({
  label,
  icon,
  value,
  sub,
  progress,
  trend,
  accent,
}: {
  label: string;
  icon: ReactNode;
  value: string;
  sub?: string;
  progress?: number;
  trend?: number;
  accent?: "primary" | "success" | "warning" | "violet";
}) {
  const accentClass = {
    primary: "bg-accent",
    success: "bg-success",
    warning: "bg-warning",
    violet: "bg-violet",
  }[accent ?? "primary"];
  const accentText = {
    primary: "text-accent",
    success: "text-success",
    warning: "text-warning",
    violet: "text-violet",
  }[accent ?? "primary"];
  const history = useMetricHistory(trend);
  return (
    <Card className="border border-border bg-surface p-4">
      <div className="flex items-center justify-between text-xs font-semibold uppercase tracking-wider text-muted">
        <span>{label}</span>
        <span className="text-muted">{icon}</span>
      </div>
      <div className="pt-2 font-mono text-2xl font-bold text-foreground">{value}</div>
      <Sparkline data={history} className={`mt-2 ${accentText}`} />
      {progress !== undefined && (
        <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-surface/60">
          <div
            className={`h-full ${accentClass}`}
            style={{ width: `${Math.min(100, Math.max(0, progress))}%` }}
          />
        </div>
      )}
      {sub && <div className="pt-2 text-xs text-muted">{sub}</div>}
    </Card>
  );
}

function EndpointRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-start gap-2 rounded-md border border-border bg-surface/60 px-3 py-2">
      <span className="w-16 shrink-0 pt-1 text-xs font-medium text-muted">{label}</span>
      <div className="flex min-w-0 flex-1 items-center gap-2">{children}</div>
    </div>
  );
}

function PlayersCard({
  roster,
  fallbackOnline,
}: {
  roster?: PlayersResp;
  fallbackOnline: number;
}) {
  const online = roster?.online ?? fallbackOnline;
  const names = roster?.players ?? [];
  const supported =
    roster === undefined || roster.capabilities !== undefined;

  return (
    <Card className="border border-border bg-surface">
      <CardHeader className="border-b border-border px-4 py-3">
        <div className="flex flex-col gap-1">
          <h3 className="text-lg font-semibold text-foreground">
            Players online · <span className="font-mono">{online}</span>
          </h3>
          {names.length > 0 && (
            <span className="text-xs text-muted">{names.length} listed</span>
          )}
        </div>
      </CardHeader>
      <CardContent className="px-4 py-3">
        {!supported ? (
          <p className="text-sm text-muted">
            Player list not supported for this game.
          </p>
        ) : online === 0 ? (
          <p className="text-sm text-muted">No players connected.</p>
        ) : names.length === 0 ? (
          <p className="text-sm text-muted">
            {online} online · names not yet available.
          </p>
        ) : (
          <ul className="space-y-2 text-sm">
            {names.slice(0, 5).map((n) => (
              <li key={n} className="flex items-center gap-2">
                <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-surface font-mono text-[10px] text-muted">
                  {n.slice(0, 2).toUpperCase()}
                </div>
                <span className="truncate font-mono text-foreground">{n}</span>
              </li>
            ))}
            {names.length > 5 && (
              <li className="pl-8 text-xs text-muted">
                + {names.length - 5} more
              </li>
            )}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

