import { lazy, Suspense, useEffect, useRef, useState } from "react";
import { useNavigate, useParams, useSearch } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  Loader2,
  Play,
  RotateCw,
  Square,
  Sunrise,
  Terminal,
} from "lucide-react";
import { Servers, Templates, type LifecycleVerb } from "@/lib/endpoints";
import { resolveConsoleMode, serverHasMods, serverHasModpacks } from "@/lib/capabilities";
import { PhaseBadge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { GameIcon } from "@/components/ui/game-icon";
import { capitalize, cn, formatUptime } from "@/lib/utils";
import { ServerActionsMenu } from "@/components/server/ServerActionsMenu";

import { OverviewTab } from "./tabs/Overview";
import { EventsTab } from "./tabs/Events";
import { LogsTab } from "./tabs/Logs";
import { ModsTab } from "./tabs/Mods";
import { ModpacksTab } from "./tabs/Modpacks";
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

type TabKey =
  | "overview" | "events" | "console" | "logs" | "files" | "mods" | "modpacks" | "players" | "backups" | "settings";

const tabs: Array<{ key: TabKey; label: string }> = [
  { key: "overview", label: "Overview" },
  { key: "events",   label: "Events" },
  { key: "console",  label: "Console" },
  { key: "logs",     label: "Logs" },
  { key: "files",    label: "Files" },
  { key: "mods",     label: "Mods" },
  { key: "modpacks", label: "Modpacks" },
  { key: "players",  label: "Players" },
  { key: "backups",  label: "Backups" },
  { key: "settings", label: "Settings" },
];

export function ServerDetailPage() {
  const { name } = useParams({ from: "/app-layout/servers/$name" });
  const { ns } = useSearch({ from: "/app-layout/servers/$name" });
  const [tab, setTab] = useState<TabKey>("overview");
  const [settingsDirty, setSettingsDirty] = useState(false);
  const qc = useQueryClient();
  const nav = useNavigate();

  const { data: gs } = useQuery({
    queryKey: ["server", name, ns],
    queryFn: () => Servers.get(name, ns),
    refetchInterval: settingsDirty ? false : 5_000,
  });

  const templateName = gs?.spec.templateRef.name;
  const { data: tmpl } = useQuery({
    queryKey: ["template", templateName],
    queryFn: () => Templates.get(templateName as string),
    enabled: !!templateName,
  });

  const act = useMutation({
    mutationFn: (verb: LifecycleVerb) => Servers.lifecycle(name, verb, ns),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["server", name, ns] }),
  });

  const phase = gs?.status?.phase;
  const asleep = gs?.status?.idle?.asleep === true;
  const running = phase === "Running";
  // While provisioning, the operator refines the Progressing condition
  // with what the pod is doing (pulling image, installing server files,
  // waiting for the agent) — surface it under the phase badge.
  const provisioning = phase === "Starting" || phase === "Pending";
  const progressMessage = gs?.status?.conditions?.find(
    (c) => c.type === "Progressing",
  )?.message;
  // When startup terminally fails the operator sets phase=Failed and puts
  // the reason on the Ready condition (image pull, crash-loop, exit). Both
  // the operator's escalation and a build-error setPhase write Ready, so
  // read the failure message from there.
  const failed = phase === "Failed";
  const failureMessage = gs?.status?.conditions?.find(
    (c) => c.type === "Ready",
  )?.message;
  // Gate lifecycle actions on phase so they aren't fired during a
  // transition (Starting/Stopping/Pending): Start only from a stopped
  // state, Stop/Restart only while Running. act.isPending blocks
  // duplicate mutations from a double-click. When asleep, Start is a silent
  // no-op (spec.suspend is already false) — only :wake clears the operator's
  // sleep marker.
  const canStart = (phase === "Stopped" || phase === "Suspended" || phase === "Failed") && !asleep;
  const version = gs?.status?.agent?.gameVersion;
  const uptime = formatUptime(gs?.status?.startedAt);

  // Tab visibility is driven by the template: a game with no console
  // (consoleMode none / no RCON) hides the Console tab. Logs is always
  // available — the tab streams the container's stdout (install/startup
  // output) via the pod-log API, which needs no logPath; the configured
  // game-log file is just an extra source the tab offers when logPath is
  // set. While the template is still loading we show everything.
  const consoleAvailable = !tmpl || resolveConsoleMode(tmpl) !== "none";
  // Mods only appears when this server actually has a mod directory — the
  // template declares the capability AND (for the per-loader model) the
  // active version's loader maps to one. Hidden for e.g. vanilla servers.
  const modsAvailable = serverHasMods(tmpl, gs);
  // Modpacks appears when the template offers modpacks AND the active
  // version's loader can run one — hidden for vanilla and plugin loaders
  // (e.g. Paper), which can't load a Modrinth/Forge modpack.
  const modpacksAvailable = serverHasModpacks(tmpl, gs);
  const visibleTabs = tabs.filter((t) => {
    if (t.key === "console") return consoleAvailable;
    if (t.key === "mods") return modsAvailable;
    if (t.key === "modpacks") return modpacksAvailable;
    return true;
  });

  // If the active tab gets hidden once the template resolves, fall back
  // to Overview so the content area never goes blank.
  useEffect(() => {
    if (!visibleTabs.some((t) => t.key === tab)) setTab("overview");
  }, [visibleTabs, tab]);

  // A freshly-created server is provisioning — land the user on Logs so
  // they can watch the install stream, rather than an empty Overview.
  // "Never been Running" = Pending/Starting with no startedAt (the
  // operator sets startedAt only on first reaching Running). Fires once,
  // after gs first loads, and never fights a manual tab click.
  const autoTabApplied = useRef(false);
  useEffect(() => {
    if (autoTabApplied.current || !gs) return;
    autoTabApplied.current = true;
    const st = gs.status;
    if ((st?.phase === "Pending" || st?.phase === "Starting") && !st.startedAt) {
      setTab("logs");
    }
  }, [gs]);

  return (
    <div className="flex h-full flex-col">
      <header className="border-b border-border bg-background px-6 pb-0 pt-4">
        <div className="flex flex-wrap items-start justify-between gap-4 pb-4">
          <div className="flex items-start gap-4">
            <GameIcon
              game={gs?.spec.templateRef.name}
              accentColor={tmpl?.spec.accentColor}
              size="lg"
            />
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-3">
                <h1 className="truncate font-mono text-2xl font-semibold text-fg">{name}</h1>
                <PhaseBadge phase={phase} asleep={asleep} />
              </div>
              {provisioning && progressMessage && (
                <div className="pt-1 flex items-center gap-1.5 text-xs text-warning">
                  <Loader2 className="h-3 w-3 animate-spin" />
                  <span>{capitalize(progressMessage)}</span>
                </div>
              )}
              {failed && (
                <div className="pt-1 flex items-center gap-1.5 text-xs text-danger">
                  <AlertTriangle className="h-3 w-3" />
                  <span>
                    {failureMessage ? capitalize(failureMessage) : "The server failed to start."}
                  </span>
                </div>
              )}
              <div className="pt-1 flex flex-wrap items-center gap-2 text-xs text-muted">
                {gs?.spec.templateRef.name && <span>{gs.spec.templateRef.name}</span>}
                {version && <Dot />}{version && <span>{version}</span>}
                {gs?.metadata.namespace && <><Dot /><span>ns: {gs.metadata.namespace}</span></>}
                {uptime !== "—" && <><Dot /><span>up {uptime}</span></>}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              onClick={() => act.mutate("restart")}
              disabled={!running || act.isPending}
            >
              <RotateCw className="h-4 w-4" /> Restart
            </Button>
            <Button
              variant="outline"
              onClick={() => act.mutate("stop")}
              disabled={!running || act.isPending}
            >
              <Square className="h-4 w-4" /> Stop
            </Button>
            {asleep && (
              <Button onClick={() => act.mutate("wake")} disabled={act.isPending}>
                <Sunrise className="h-4 w-4" /> Wake
              </Button>
            )}
            {canStart && (
              <Button variant="outline" onClick={() => act.mutate("start")} disabled={act.isPending}>
                <Play className="h-4 w-4" /> Start
              </Button>
            )}
            {consoleAvailable && (
              <Button onClick={() => setTab("console")}>
                <Terminal className="h-4 w-4" /> Open console
              </Button>
            )}
            {gs && (
              <ServerActionsMenu
                gs={gs}
                onDeleted={() => void nav({ to: "/servers" })}
              />
            )}
          </div>
        </div>

        <nav className="-mb-px flex overflow-x-auto scrollbar-thin">
          {visibleTabs.map((t) => (
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
          {tab === "overview" && (
            <OverviewTab gs={gs} name={name} tmpl={tmpl} ns={ns} onViewAllEvents={() => setTab("events")} />
          )}
          {tab === "events"   && <EventsTab name={name} ns={ns} gs={gs} />}
          {tab === "console"  && <ConsoleTab name={name} ns={ns} />}
          {tab === "logs"     && (
            <LogsTab
              name={name}
              ns={ns}
              logPath={tmpl?.spec.logPath}
              phase={phase}
              progressMessage={progressMessage}
            />
          )}
          {tab === "files"    && <FilesTab   name={name} ns={ns} />}
          {tab === "mods"     && <ModsTab    name={name} ns={ns} tmpl={tmpl} gs={gs} />}
          {tab === "modpacks" && <ModpacksTab name={name} ns={ns} tmpl={tmpl} gs={gs} />}
          {tab === "players"  && <PlayersTab name={name} ns={ns} />}
          {tab === "backups"  && <BackupsTab name={name} ns={ns} />}
          {tab === "settings" && (
            <SettingsTab gs={gs} name={name} ns={ns} onDirtyChange={setSettingsDirty} />
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
