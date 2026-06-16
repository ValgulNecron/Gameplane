import { lazy, Suspense, useEffect, useRef, useState } from "react";
import { useNavigate, useParams } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import * as Dialog from "@radix-ui/react-dialog";
import {
  Copy,
  Loader2,
  MoreHorizontal,
  Play,
  RotateCw,
  Square,
  Terminal,
} from "lucide-react";
import { Servers, Templates, type LifecycleVerb } from "@/lib/endpoints";
import { APIError } from "@/lib/api";
import { resolveConsoleMode } from "@/lib/capabilities";
import { useMe, can } from "@/lib/auth";
import { isValidK8sName } from "@/lib/validation";
import { PhaseBadge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { GameIcon } from "@/components/ui/game-icon";
import { Input } from "@/components/ui/input";
import { capitalize, cn, formatUptime } from "@/lib/utils";

import { OverviewTab } from "./tabs/Overview";
import { LogsTab } from "./tabs/Logs";
import { ModsTab } from "./tabs/Mods";
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

type TabKey = "overview" | "console" | "logs" | "files" | "mods" | "players" | "backups" | "settings";

const tabs: Array<{ key: TabKey; label: string }> = [
  { key: "overview", label: "Overview" },
  { key: "console",  label: "Console" },
  { key: "logs",     label: "Logs" },
  { key: "files",    label: "Files" },
  { key: "mods",     label: "Mods" },
  { key: "players",  label: "Players" },
  { key: "backups",  label: "Backups" },
  { key: "settings", label: "Settings" },
];

export function ServerDetailPage() {
  const { name } = useParams({ from: "/app-layout/servers/$name" });
  const [tab, setTab] = useState<TabKey>("overview");
  const [settingsDirty, setSettingsDirty] = useState(false);
  const [cloneOpen, setCloneOpen] = useState(false);
  const qc = useQueryClient();
  const { data: me } = useMe();
  const canClone = can(me, "servers:write");

  const { data: gs } = useQuery({
    queryKey: ["server", name],
    queryFn: () => Servers.get(name),
    refetchInterval: settingsDirty ? false : 5_000,
  });

  const templateName = gs?.spec.templateRef.name;
  const { data: tmpl } = useQuery({
    queryKey: ["template", templateName],
    queryFn: () => Templates.get(templateName as string),
    enabled: !!templateName,
  });

  const act = useMutation({
    mutationFn: (verb: LifecycleVerb) => Servers.lifecycle(name, verb),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["server", name] }),
  });

  const phase = gs?.status?.phase;
  const running = phase === "Running";
  // While provisioning, the operator refines the Progressing condition
  // with what the pod is doing (pulling image, installing server files,
  // waiting for the agent) — surface it under the phase badge.
  const provisioning = phase === "Starting" || phase === "Pending";
  const progressMessage = gs?.status?.conditions?.find(
    (c) => c.type === "Progressing",
  )?.message;
  // Gate lifecycle actions on phase so they aren't fired during a
  // transition (Starting/Stopping/Pending): Start only from a stopped
  // state, Stop/Restart only while Running. act.isPending blocks
  // duplicate mutations from a double-click.
  const canStart = phase === "Stopped" || phase === "Suspended" || phase === "Failed";
  const version = gs?.status?.agent?.gameVersion;
  const uptime = formatUptime(gs?.status?.startedAt);

  // Tab visibility is driven by the template: a game with no console
  // (consoleMode none / no RCON) hides the Console tab. Logs is always
  // available — the tab streams the container's stdout (install/startup
  // output) via the pod-log API, which needs no logPath; the configured
  // game-log file is just an extra source the tab offers when logPath is
  // set. While the template is still loading we show everything.
  const consoleAvailable = !tmpl || resolveConsoleMode(tmpl) !== "none";
  // Mods only appears when the template declares the capability — it's an
  // opt-in surface, so hide it until the template resolves.
  const modsAvailable = !!tmpl?.spec.capabilities?.mods;
  const visibleTabs = tabs.filter((t) => {
    if (t.key === "console") return consoleAvailable;
    if (t.key === "mods") return modsAvailable;
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
                <PhaseBadge phase={phase} />
              </div>
              {provisioning && progressMessage && (
                <div className="pt-1 flex items-center gap-1.5 text-xs text-warning">
                  <Loader2 className="h-3 w-3 animate-spin" />
                  <span>{capitalize(progressMessage)}</span>
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
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" aria-label="More actions">
                  <MoreHorizontal className="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent>
                <DropdownMenuItem
                  icon={<Copy className="h-4 w-4" />}
                  label="Clone server"
                  onSelect={() => setCloneOpen(true)}
                  disabled={!canClone}
                  hint={canClone ? undefined : "Requires operator role"}
                />
              </DropdownMenuContent>
            </DropdownMenu>
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
          {tab === "overview" && <OverviewTab gs={gs} name={name} tmpl={tmpl} />}
          {tab === "console"  && <ConsoleTab name={name} />}
          {tab === "logs"     && (
            <LogsTab
              name={name}
              logPath={tmpl?.spec.logPath}
              phase={phase}
              progressMessage={progressMessage}
            />
          )}
          {tab === "files"    && <FilesTab   name={name} />}
          {tab === "mods"     && <ModsTab    name={name} tmpl={tmpl} />}
          {tab === "players"  && <PlayersTab name={name} />}
          {tab === "backups"  && <BackupsTab name={name} />}
          {tab === "settings" && (
            <SettingsTab gs={gs} name={name} onDirtyChange={setSettingsDirty} />
          )}
        </Suspense>
      </div>

      <CloneDialog open={cloneOpen} onOpenChange={setCloneOpen} sourceName={name} />
    </div>
  );
}

function cloneErrorMessage(err: unknown, name: string): string {
  if (err instanceof APIError) {
    if (err.status === 409) return `A server named ${name} already exists.`;
    if (err.status === 403) return "Your role does not allow cloning servers.";
    return err.body.slice(0, 240) || `Clone failed (${err.status}).`;
  }
  return err instanceof Error ? err.message : "Unknown error";
}

function CloneDialog({
  open,
  onOpenChange,
  sourceName,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  sourceName: string;
}) {
  const qc = useQueryClient();
  const nav = useNavigate();
  const [newName, setNewName] = useState("");

  useEffect(() => {
    if (open) setNewName(`${sourceName.slice(0, 58)}-copy`);
  }, [open, sourceName]);

  const clone = useMutation({
    mutationFn: () => Servers.clone(sourceName, newName),
    onSuccess: async (created) => {
      await qc.invalidateQueries({ queryKey: ["servers"] });
      onOpenChange(false);
      await nav({ to: "/servers/$name", params: { name: created.metadata.name } });
    },
  });

  const valid = isValidK8sName(newName);

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-[440px] max-w-[calc(100vw-2rem)] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-border bg-card p-5 text-fg shadow-2xl">
          <Dialog.Title className="text-base font-semibold">Clone server</Dialog.Title>
          <Dialog.Description asChild>
            <div className="pt-2 text-sm text-muted">
              Creates a new server with the same configuration. World data is not copied.
            </div>
          </Dialog.Description>

          <div className="pt-4">
            <label className="block pb-1 text-xs text-muted" htmlFor="clone-new-name">
              New name
            </label>
            <Input
              id="clone-new-name"
              autoFocus
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              spellCheck={false}
            />
            {!valid && (
              <p className="pt-1 text-xs text-danger">
                Name must be lowercase letters, digits, dashes (max 63)
              </p>
            )}
            {clone.isError && (
              <p className="pt-1 text-xs text-danger">
                {cloneErrorMessage(clone.error, newName)}
              </p>
            )}
          </div>

          <div className="flex items-center justify-end gap-2 pt-5">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onOpenChange(false)}
              disabled={clone.isPending}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              disabled={!valid || clone.isPending}
              onClick={() => clone.mutate()}
            >
              {clone.isPending ? "Cloning…" : "Clone server"}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
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
