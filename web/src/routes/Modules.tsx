import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Search, Settings2 } from "lucide-react";
import { Link } from "@tanstack/react-router";

import { ModuleCard } from "@/components/modules/ModuleCard";
import { InstallDialog } from "@/components/modules/InstallDialog";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { PageHeader } from "@/components/PageHeader";
import { Modules } from "@/lib/endpoints";
import { APIError } from "@/lib/api";
import type { CatalogEntry } from "@/types";
import { cn } from "@/lib/utils";

// ModulesPage renders the catalog merged across every ModuleSource. The
// install/upgrade/uninstall actions all hit /modules — install creates
// a Module CR, the operator's Module reconciler does the OCI pull and
// materializes the GameTemplate.
export function ModulesPage() {
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({
    queryKey: ["modules-catalog"],
    queryFn: () => Modules.catalog(),
    refetchInterval: 5_000, // pick up phase transitions promptly
  });

  const [q, setQ] = useState("");
  const [sourceFilter, setSourceFilter] = useState<string>("all");
  const [installTarget, setInstallTarget] = useState<CatalogEntry | null>(null);
  const [uninstallTarget, setUninstallTarget] = useState<CatalogEntry | null>(null);
  const [pageError, setPageError] = useState<string | null>(null);

  const items = useMemo(() => data?.items ?? [], [data]);
  const sources = useMemo(() => {
    const out = new Set<string>();
    for (const it of items) for (const s of it.sources) out.add(s.name);
    return ["all", ...Array.from(out).sort()];
  }, [items]);

  const visible = items.filter((e) => {
    if (sourceFilter !== "all" && !e.sources.some((s) => s.name === sourceFilter)) return false;
    if (q && !(e.displayName ?? e.name).toLowerCase().includes(q.toLowerCase())) {
      return false;
    }
    return true;
  });

  const installMutation = useMutation({
    mutationFn: (args: { source: string; module: string; name: string; version: string }) =>
      Modules.install(args),
    onSuccess: async () => {
      setInstallTarget(null);
      setPageError(null);
      await qc.invalidateQueries({ queryKey: ["modules-catalog"] });
    },
    onError: (err: Error) => setPageError(formatErr(err)),
  });

  const upgradeMutation = useMutation({
    mutationFn: (args: { name: string; version: string }) =>
      Modules.upgrade(args.name, args.version),
    onSuccess: async () => {
      setPageError(null);
      await qc.invalidateQueries({ queryKey: ["modules-catalog"] });
    },
    onError: (err: Error) => setPageError(formatErr(err)),
  });

  const uninstallMutation = useMutation({
    mutationFn: (name: string) => Modules.uninstall(name),
    onSuccess: async () => {
      setUninstallTarget(null);
      setPageError(null);
      await qc.invalidateQueries({ queryKey: ["modules-catalog"] });
    },
    onError: (err: Error) => setPageError(formatErr(err)),
  });

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Modules"
        subtitle="Pre-packaged game-server templates pulled from your configured OCI registries."
        actions={
          <Button variant="outline" asChild>
            <Link to="/admin" hash="modules">
              <Settings2 className="h-4 w-4" /> Manage sources
            </Link>
          </Button>
        }
      />

      <div className="flex flex-wrap items-center gap-3">
        <div className="inline-flex gap-1 rounded-md border border-border bg-card p-1">
          {sources.map((s) => (
            <button
              key={s}
              onClick={() => setSourceFilter(s)}
              className={cn(
                "rounded px-3 py-1.5 text-xs transition-colors",
                sourceFilter === s
                  ? "bg-primary/15 text-primary"
                  : "text-muted hover:text-fg",
              )}
            >
              {s === "all" ? "All sources" : s}
            </button>
          ))}
        </div>
        <div className="relative ml-auto w-64">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" />
          <Input
            className="pl-9"
            placeholder="Search modules…"
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
        </div>
      </div>

      {pageError && (
        <div className="rounded border border-danger/40 bg-danger/10 px-3 py-2 text-xs text-danger">
          {pageError}
        </div>
      )}

      <div
        className="grid gap-4 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4"
        data-testid="modules-grid"
      >
        {visible.map((entry) => (
          <ModuleCard
            key={entry.name}
            entry={entry}
            onInstall={setInstallTarget}
            onUpgrade={(e) => {
              if (!e.moduleName || !e.latestVersion) return;
              upgradeMutation.mutate({ name: e.moduleName, version: e.latestVersion });
            }}
            onUninstall={setUninstallTarget}
            busy={installMutation.isPending || upgradeMutation.isPending}
          />
        ))}
        {!isLoading && visible.length === 0 && (
          <div className="col-span-full rounded-lg border border-dashed border-border bg-card/40 p-12 text-center text-sm text-muted">
            {items.length === 0
              ? "No modules in any catalog yet — check ModuleSource sync status under Admin → Module sources."
              : "No modules match the current filter."}
          </div>
        )}
      </div>

      <InstallDialog
        open={!!installTarget}
        onOpenChange={(o) => !o && setInstallTarget(null)}
        entry={installTarget}
        busy={installMutation.isPending}
        onConfirm={async ({ source, version, name }) => {
          if (!installTarget) return;
          await installMutation.mutateAsync({
            source,
            module: installTarget.name,
            name,
            version,
          });
        }}
      />

      <ConfirmDialog
        open={!!uninstallTarget}
        onOpenChange={(o) => !o && setUninstallTarget(null)}
        title={`Uninstall ${uninstallTarget?.displayName ?? uninstallTarget?.name ?? ""}?`}
        description={
          <>
            <p>
              Deletes the Module resource and its managed GameTemplate. Game
              servers using this template must be removed first.
            </p>
          </>
        }
        confirmLabel="Uninstall"
        destructive
        busy={uninstallMutation.isPending}
        onConfirm={() => {
          if (!uninstallTarget?.moduleName) return;
          uninstallMutation.mutate(uninstallTarget.moduleName);
        }}
      />
    </div>
  );
}

function formatErr(err: Error): string {
  if (err instanceof APIError) {
    return err.body || err.message;
  }
  return err.message;
}
