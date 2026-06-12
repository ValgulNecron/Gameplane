import { Link } from "@tanstack/react-router";
import { ArrowUpCircle, Download, ExternalLink, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { GameIcon } from "@/components/ui/game-icon";
import { cn } from "@/lib/utils";
import type { CatalogEntry } from "@/types";

interface ModuleCardProps {
  entry: CatalogEntry;
  onInstall: (entry: CatalogEntry) => void;
  onUpgrade: (entry: CatalogEntry) => void;
  onUninstall: (entry: CatalogEntry) => void;
  busy?: boolean;
}

// ModuleCard is the catalog grid tile. The right-hand action depends
// on installation state:
//   - not installed     → "Install"
//   - installed, current → "Deploy" (navigate to /servers/new)
//   - upgrade available → "Upgrade"
//   - phase != Ready    → render the phase + spinner (no actions)
export function ModuleCard({ entry, onInstall, onUpgrade, onUninstall, busy }: ModuleCardProps) {
  const upgradeAvailable =
    entry.installed &&
    entry.installedVersion &&
    entry.latestVersion &&
    entry.latestVersion !== entry.installedVersion;

  const inFlight = entry.phase && entry.phase !== "Ready" && entry.installed;

  const versionLabel = entry.installed
    ? `v${entry.installedVersion ?? "?"} installed${
        upgradeAvailable ? ` · v${entry.latestVersion} available` : ""
      }`
    : entry.latestVersion
    ? `v${entry.latestVersion}`
    : "no published versions";

  return (
    <Card className="flex flex-col gap-3 p-4">
      <div className="flex items-start justify-between gap-3">
        <GameIcon game={entry.game ?? entry.name} size="md" />
        <StatusPill entry={entry} />
      </div>
      <div>
        <div className="font-medium text-fg">
          {entry.displayName ?? entry.name}
        </div>
        <div className="pt-0.5 font-mono text-[11px] text-muted">{versionLabel}</div>
      </div>
      <p className="line-clamp-3 flex-1 text-xs text-muted">
        {entry.summary ?? "No summary."}
      </p>
      {entry.lastError && (
        <div className="rounded border border-danger/40 bg-danger/10 px-2 py-1 text-[11px] text-danger">
          {entry.lastError}
        </div>
      )}
      <div className="mt-1 flex flex-wrap items-center justify-between gap-2 text-[11px] text-muted">
        <span className="font-mono">
          {entry.sources.length === 1
            ? `${entry.sources[0].name} (${entry.sources[0].type})`
            : `${entry.sources.length} sources`}
        </span>
        <div className="flex items-center gap-1">
          {entry.installed && entry.phase === "Ready" && entry.moduleName && (
            <Button size="sm" variant="outline" asChild>
              <Link
                to="/servers/new"
                search={{ template: entry.moduleName } as never}
              >
                <ExternalLink className="h-3.5 w-3.5" /> Deploy
              </Link>
            </Button>
          )}
          {upgradeAvailable && entry.phase === "Ready" && (
            <Button size="sm" onClick={() => onUpgrade(entry)} disabled={busy}>
              <ArrowUpCircle className="h-3.5 w-3.5" />
              Upgrade
            </Button>
          )}
          {!entry.installed && (
            <Button size="sm" onClick={() => onInstall(entry)} disabled={busy}>
              {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Download className="h-3.5 w-3.5" />}
              Install
            </Button>
          )}
          {entry.installed && (
            <Button
              size="sm"
              variant="outline"
              onClick={() => onUninstall(entry)}
              disabled={busy || !!inFlight}
            >
              Uninstall
            </Button>
          )}
        </div>
      </div>
    </Card>
  );
}

function StatusPill({ entry }: { entry: CatalogEntry }) {
  let label = "available";
  let cls = "bg-muted/20 text-muted";
  if (entry.installed) {
    if (entry.phase === "Ready") {
      label = "installed";
      cls = "bg-success/15 text-success";
    } else if (entry.phase === "Failed") {
      label = "failed";
      cls = "bg-danger/15 text-danger";
    } else {
      label = (entry.phase ?? "pending").toLowerCase();
      cls = "bg-warning/15 text-warning";
    }
  }
  return (
    <span
      className={cn(
        "rounded-full px-2 py-0.5 font-mono text-[10px] uppercase",
        cls,
      )}
    >
      {label}
    </span>
  );
}
