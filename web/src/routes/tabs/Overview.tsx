import type { ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Activity,
  Copy,
  Cpu,
  MemoryStick,
  Users as UsersIcon,
} from "lucide-react";
import type { GameServer, GameTemplate, PlayersResp } from "@/types";
import { Players } from "@/lib/endpoints";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { ServerActionsCard } from "@/components/server/ServerActionsCard";
import { ServerStatusCard } from "@/components/server/ServerStatusCard";
import { formatBytes, formatRelative } from "@/lib/utils";

interface Event {
  id: string;
  ts: string;
  kind: "info" | "warn" | "error";
  message: string;
  source?: string;
}

export function OverviewTab({
  gs,
  name,
  tmpl,
}: {
  gs?: GameServer;
  name: string;
  tmpl?: GameTemplate;
}) {
  const { data: roster } = useQuery({
    queryKey: ["players", name, "overview"],
    queryFn: () => Players.snapshot(name),
    enabled: !!name && (gs?.status?.phase === "Running"),
    refetchInterval: 10_000,
    retry: false,
  });

  if (!gs) return <div className="p-6 text-muted">Loading…</div>;

  const status = gs.status ?? {};
  const running = status.phase === "Running";
  const agent = status.agent;
  const cpu = (status as unknown as { cpuPercent?: number }).cpuPercent ?? 0;
  const cpuCores = (status as unknown as { cpuCores?: number }).cpuCores ?? 2;
  const memUsed = (status as unknown as { memoryBytes?: number }).memoryBytes ?? 0;
  const memTotal = (status as unknown as { memoryTotalBytes?: number }).memoryTotalBytes ?? 0;
  const netRx = (status as unknown as { networkRxBps?: number }).networkRxBps ?? 0;
  const netTx = (status as unknown as { networkTxBps?: number }).networkTxBps ?? 0;
  // null/undefined playersOnline means "unknown" (RCON unavailable or a
  // stale heartbeat the API blanked) — render "—", not a misleading 0.
  const playersKnown = typeof agent?.playersOnline === "number" && agent.playersOnline >= 0;
  const players = playersKnown ? (agent?.playersOnline as number) : 0;
  const playersMax = agent?.playersMax ?? 0;
  const events: Event[] = (status as unknown as { recentEvents?: Event[] }).recentEvents ?? [];
  const endpoints = status.endpoints ?? [];
  const primary = endpoints[0];

  return (
    <div className="grid gap-5 p-6 lg:grid-cols-[1fr_320px]">
      <div className="space-y-5">
        <div className="grid gap-4 md:grid-cols-4">
          <MetricTile
            label="CPU"
            icon={<Cpu className="h-4 w-4" />}
            primary={`${cpu.toFixed(0)}%`}
            secondary={`of ${cpuCores} cores`}
            progress={cpu}
            accent="primary"
          />
          <MetricTile
            label="Memory"
            icon={<MemoryStick className="h-4 w-4" />}
            primary={`${memTotal ? Math.round((memUsed / memTotal) * 100) : 0}%`}
            secondary={`${formatBytes(memUsed)} / ${memTotal ? formatBytes(memTotal) : "—"}`}
            progress={memTotal ? (memUsed / memTotal) * 100 : 0}
            accent="violet"
          />
          <MetricTile
            label="Network"
            icon={<Activity className="h-4 w-4" />}
            primary={`${formatBytes(netRx)}/s`}
            secondary={`↓ ${formatBytes(netRx)} / ↑ ${formatBytes(netTx)}`}
            accent="success"
          />
          <MetricTile
            label="Players"
            icon={<UsersIcon className="h-4 w-4" />}
            primary={playersKnown ? `${players}` : "—"}
            secondary={playersMax ? `of ${playersMax} slots` : "—"}
            progress={playersKnown && playersMax ? (players / playersMax) * 100 : 0}
            accent="warning"
          />
        </div>

        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <CardTitle>Recent events</CardTitle>
            </div>
            <button className="text-xs text-primary hover:underline">View all</button>
          </CardHeader>
          <CardContent className="px-0">
            {events.length === 0 && (
              <div className="px-6 pb-6 text-sm text-muted">
                No events yet. Lifecycle, backup, and agent activity will appear here.
              </div>
            )}
            <ul className="divide-y divide-border">
              {events.map((e) => (
                <li key={e.id} className="flex items-start gap-3 px-6 py-3">
                  <EventDot kind={e.kind} />
                  <div className="min-w-0 flex-1">
                    <div className="text-sm text-fg">{e.message}</div>
                    <div className="pt-0.5 text-xs text-muted">
                      {e.source ?? "system"}
                    </div>
                  </div>
                  <div className="text-xs text-muted">{formatRelative(e.ts)}</div>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      </div>

      <div className="space-y-5">
        <Card>
          <CardHeader><CardTitle>Connection</CardTitle></CardHeader>
          <CardContent>
            <div className="space-y-3 text-sm">
              <InfoRow label="Host">
                <span className="truncate font-mono">{primary?.host ?? "—"}</span>
                {primary?.host && (
                  <button
                    className="rounded p-1 text-muted hover:bg-border hover:text-fg"
                    onClick={() => navigator.clipboard?.writeText(primary.host)}
                    title="Copy"
                  >
                    <Copy className="h-3.5 w-3.5" />
                  </button>
                )}
              </InfoRow>
              <InfoRow label="Port">
                <span className="font-mono">{primary?.port ?? "—"}</span>
              </InfoRow>
            </div>
          </CardContent>
        </Card>

        <PlayersCard roster={roster} fallbackOnline={players} />

        <ServerStatusCard name={name} tmpl={tmpl} running={running} />

        <ServerActionsCard name={name} tmpl={tmpl} />
      </div>
    </div>
  );
}

function MetricTile({
  label,
  icon,
  primary,
  secondary,
  progress,
  accent,
}: {
  label: string;
  icon: ReactNode;
  primary: string;
  secondary?: string;
  progress?: number;
  accent?: "primary" | "success" | "warning" | "violet";
}) {
  const accentClass = {
    primary: "bg-primary",
    success: "bg-success",
    warning: "bg-warning",
    violet:  "bg-violet",
  }[accent ?? "primary"];
  return (
    <Card className="p-4">
      <div className="flex items-center justify-between text-xs uppercase tracking-wide text-muted">
        <span>{label}</span>
        <span className="text-muted">{icon}</span>
      </div>
      <div className="pt-2 font-mono text-2xl text-fg">{primary}</div>
      {progress !== undefined && (
        <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-surface">
          <div
            className={`h-full ${accentClass}`}
            style={{ width: `${Math.min(100, Math.max(0, progress))}%` }}
          />
        </div>
      )}
      {secondary && <div className="pt-2 text-xs text-muted">{secondary}</div>}
    </Card>
  );
}

function EventDot({ kind }: { kind: Event["kind"] }) {
  const color = {
    info:  "bg-primary",
    warn:  "bg-warning",
    error: "bg-danger",
  }[kind];
  return <span className={`mt-1.5 inline-block h-2 w-2 rounded-full ${color}`} />;
}

function InfoRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-center gap-2 rounded-md border border-border bg-surface/60 px-3 py-2">
      <span className="w-16 shrink-0 text-xs text-muted">{label}</span>
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
    <Card>
      <CardHeader>
        <CardTitle>Players online · {online}</CardTitle>
        {names.length > 0 && (
          <span className="text-xs text-muted">{names.length} listed</span>
        )}
      </CardHeader>
      <CardContent>
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
                <div className="flex h-6 w-6 items-center justify-center rounded-full bg-surface font-mono text-[10px] text-muted">
                  {n.slice(0, 2).toUpperCase()}
                </div>
                <span className="font-mono">{n}</span>
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

