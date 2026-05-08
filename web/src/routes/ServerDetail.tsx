import { lazy, Suspense, useState } from "react";
import { useParams } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  MoreHorizontal,
  Play,
  RotateCw,
  Square,
  Terminal,
} from "lucide-react";
import { Servers, type LifecycleVerb } from "@/lib/endpoints";
import { PhaseBadge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { GameIcon } from "@/components/ui/game-icon";
import { cn, formatUptime } from "@/lib/utils";

import { OverviewTab } from "./tabs/Overview";
import { LogsTab } from "./tabs/Logs";
import { PlayersTab } from "./tabs/Players";
import { BackupsTab } from "./tabs/Backups";
import { SettingsTab } from "./tabs/Settings";

// Console (xterm) and Files (Monaco) bring in heavy vendor deps; load on demand.
const ConsoleTab = lazy(() =>
  import("./tabs/Console").then((m) => ({ default: m.ConsoleTab })),
);
const FilesTab = lazy(() =>
  import("./tabs/Files").then((m) => ({ default: m.FilesTab })),
);

type TabKey = "overview" | "console" | "logs" | "files" | "players" | "backups" | "settings";

const tabs: Array<{ key: TabKey; label: string }> = [
  { key: "overview", label: "Overview" },
  { key: "console",  label: "Console" },
  { key: "logs",     label: "Logs" },
  { key: "files",    label: "Files" },
  { key: "players",  label: "Players" },
  { key: "backups",  label: "Backups" },
  { key: "settings", label: "Settings" },
];

export function ServerDetailPage() {
  const { name } = useParams({ from: "/app-layout/servers/$name" });
  const [tab, setTab] = useState<TabKey>("overview");
  const [settingsDirty, setSettingsDirty] = useState(false);
  const qc = useQueryClient();

  const { data: gs } = useQuery({
    queryKey: ["server", name],
    queryFn: () => Servers.get(name),
    refetchInterval: settingsDirty ? false : 5_000,
  });

  const act = useMutation({
    mutationFn: (verb: LifecycleVerb) => Servers.lifecycle(name, verb),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["server", name] }),
  });

  const phase = gs?.status?.phase;
  const running = phase === "Running";
  const version = gs?.status?.agent?.gameVersion;
  const uptime = formatUptime(gs?.status?.startedAt);

  return (
    <div className="flex h-full flex-col">
      <header className="border-b border-border bg-background px-6 pb-0 pt-4">
        <div className="flex flex-wrap items-start justify-between gap-4 pb-4">
          <div className="flex items-start gap-4">
            <GameIcon game={gs?.spec.templateRef.name} size="lg" />
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-3">
                <h1 className="truncate font-mono text-2xl font-semibold text-fg">{name}</h1>
                <PhaseBadge phase={phase} />
              </div>
              <div className="pt-1 flex flex-wrap items-center gap-2 text-xs text-muted">
                {gs?.spec.templateRef.name && <span>{gs.spec.templateRef.name}</span>}
                {version && <Dot />}{version && <span>{version}</span>}
                {gs?.metadata.namespace && <><Dot /><span>ns: {gs.metadata.namespace}</span></>}
                {uptime !== "—" && <><Dot /><span>up {uptime}</span></>}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" onClick={() => act.mutate("restart")}>
              <RotateCw className="h-4 w-4" /> Restart
            </Button>
            <Button variant="outline" onClick={() => act.mutate("stop")} disabled={!running}>
              <Square className="h-4 w-4" /> Stop
            </Button>
            {!running && (
              <Button variant="outline" onClick={() => act.mutate("start")}>
                <Play className="h-4 w-4" /> Start
              </Button>
            )}
            <Button onClick={() => setTab("console")}>
              <Terminal className="h-4 w-4" /> Open console
            </Button>
            <Button variant="ghost" size="icon" title="More">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </div>
        </div>

        <nav className="-mb-px flex overflow-x-auto scrollbar-thin">
          {tabs.map((t) => (
            <button
              key={t.key}
              onClick={() => setTab(t.key)}
              className={cn(
                "whitespace-nowrap border-b-2 px-4 py-3 text-sm transition-colors",
                tab === t.key
                  ? "border-primary text-fg"
                  : "border-transparent text-muted hover:text-fg",
              )}
            >
              {t.label}
            </button>
          ))}
        </nav>
      </header>

      <div className="flex-1 overflow-auto scrollbar-thin">
        <Suspense fallback={<TabFallback />}>
          {tab === "overview" && <OverviewTab gs={gs} name={name} />}
          {tab === "console"  && <ConsoleTab name={name} />}
          {tab === "logs"     && <LogsTab    name={name} />}
          {tab === "files"    && <FilesTab   name={name} />}
          {tab === "players"  && <PlayersTab name={name} />}
          {tab === "backups"  && <BackupsTab name={name} />}
          {tab === "settings" && (
            <SettingsTab gs={gs} name={name} onDirtyChange={setSettingsDirty} />
          )}
        </Suspense>
      </div>
    </div>
  );
}

function Dot() {
  return <span className="text-muted/50">·</span>;
}

function TabFallback() {
  return (
    <div className="flex h-full items-center justify-center text-sm text-muted">
      Loading…
    </div>
  );
}
