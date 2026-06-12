import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, CircleAlert, Circle, Pencil, Plus, RefreshCcw, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { APIError } from "@/lib/api";
import { ModuleSources } from "@/lib/endpoints";
import { formatRelative } from "@/lib/utils";
import type { ModuleSource, ModuleSourceSpec } from "@/types";
import { SourceDialog } from "./SourceDialog";

// ModuleSourcesPanel lists every ModuleSource and lets admins add,
// edit, and remove them. The same sources can equally be declared via
// Helm values or `kubectl apply` — the dashboard just writes the CRs.
export function ModuleSourcesPanel() {
  const qc = useQueryClient();
  const { data, isLoading, isError } = useQuery({
    queryKey: ["module-sources"],
    queryFn: () => ModuleSources.list(),
    refetchInterval: 10_000,
  });

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<ModuleSource | null>(null);
  const [panelError, setPanelError] = useState<string | null>(null);

  const invalidate = async () => {
    setPanelError(null);
    await qc.invalidateQueries({ queryKey: ["module-sources"] });
    await qc.invalidateQueries({ queryKey: ["modules-catalog"] });
  };

  const saveMutation = useMutation({
    mutationFn: ({ name, spec }: { name: string; spec: ModuleSourceSpec }) =>
      editTarget ? ModuleSources.update(name, spec) : ModuleSources.create(name, spec),
    onSuccess: async () => {
      setDialogOpen(false);
      await invalidate();
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (name: string) => ModuleSources.remove(name),
    onSuccess: invalidate,
    onError: (err: Error) =>
      setPanelError(err instanceof APIError ? err.body || err.message : err.message),
  });

  if (isLoading) {
    return <Card className="p-5 text-sm text-muted">Loading module sources…</Card>;
  }
  if (isError) {
    return (
      <Card className="p-5 text-sm text-danger">
        Failed to load module sources.
      </Card>
    );
  }

  const sources = data?.items ?? [];

  return (
    <Card className="p-5 space-y-3">
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="font-medium">Module sources</div>
          <div className="pt-0.5 text-xs text-muted">
            Registries, git repositories, archives, and uploads the operator
            pulls module bundles from. Equivalent to applying{" "}
            <code className="font-mono">ModuleSource</code> resources directly.
          </div>
        </div>
        <Button
          size="sm"
          onClick={() => {
            setEditTarget(null);
            setDialogOpen(true);
          }}
        >
          <Plus className="h-3.5 w-3.5" /> Add source
        </Button>
      </div>

      {panelError && (
        <div className="rounded border border-danger/40 bg-danger/10 px-3 py-2 text-xs text-danger">
          {panelError}
        </div>
      )}

      {sources.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border bg-card/40 p-6 text-center text-sm text-muted">
          No <code className="font-mono">ModuleSource</code> resources configured.
        </div>
      ) : (
        <ul className="divide-y divide-border">
          {sources.map((s) => (
            <SourceRow
              key={s.metadata.name}
              source={s}
              onEdit={() => {
                setEditTarget(s);
                setDialogOpen(true);
              }}
              onDelete={() => deleteMutation.mutate(s.metadata.name)}
              deleting={deleteMutation.isPending}
            />
          ))}
        </ul>
      )}

      <SourceDialog
        open={dialogOpen}
        onOpenChange={(open) => {
          setDialogOpen(open);
          if (!open) setEditTarget(null);
        }}
        source={editTarget}
        onConfirm={(args) => saveMutation.mutateAsync(args).then(() => undefined)}
        busy={saveMutation.isPending}
      />
    </Card>
  );
}

// sourceLocation renders the human-meaningful "where" of a source,
// which lives in a different nested field per source type.
export function sourceLocation(spec: ModuleSource["spec"]): string {
  switch (spec.type ?? "oci") {
    case "git":
      return `${spec.git?.url ?? ""}${spec.git?.ref ? `@${spec.git.ref}` : ""}`;
    case "http":
      return spec.http?.url ?? "";
    case "local":
      return spec.local?.path ? `local:${spec.local.path}` : "local";
    case "upload":
      return "uploaded bundles";
    default:
      return spec.oci?.url ?? "";
  }
}

function SourceRow({
  source,
  onEdit,
  onDelete,
  deleting,
}: {
  source: ModuleSource;
  onEdit: () => void;
  onDelete: () => void;
  deleting: boolean;
}) {
  const synced = source.status?.conditions?.find((c) => c.type === "Synced");
  const ok = synced?.status === "True";
  const Icon = ok ? CheckCircle2 : synced ? CircleAlert : Circle;
  const tone = ok ? "text-success" : synced ? "text-danger" : "text-muted";
  const refresh = source.spec.refreshInterval ?? "1h";
  const insecure = source.spec.oci?.insecure ?? source.spec.http?.insecure ?? false;
  const moduleCount =
    source.status?.modules?.length ?? source.spec.oci?.modules.length ?? 0;

  return (
    <li className="flex items-center gap-3 py-3">
      <Icon className={`h-4 w-4 ${tone}`} aria-hidden />
      <div className="min-w-0 flex-1">
        <div className="font-medium text-sm text-fg">
          {source.metadata.name}
          <span className="ml-2 rounded bg-card px-1.5 py-0.5 font-mono text-[10px] uppercase text-muted">
            {source.spec.type ?? "oci"}
          </span>
        </div>
        <div className="truncate text-xs text-muted">
          <span className="font-mono">{sourceLocation(source.spec)}</span>
          {insecure && (
            <span className="ml-2 rounded bg-warning/15 px-1.5 py-0.5 font-mono text-[10px] text-warning">
              insecure
            </span>
          )}
        </div>
      </div>
      <div className="text-right text-xs text-muted">
        <div className="flex items-center justify-end gap-1">
          <RefreshCcw className="h-3 w-3" />
          <span className="font-mono">{refresh}</span>
        </div>
        <div>
          {moduleCount} module{moduleCount === 1 ? "" : "s"}
          {source.status?.lastSync && (
            <> · synced {formatRelative(source.status.lastSync)}</>
          )}
        </div>
      </div>
      <div className="flex items-center gap-1">
        <Button size="sm" variant="ghost" onClick={onEdit} aria-label={`Edit ${source.metadata.name}`}>
          <Pencil className="h-3.5 w-3.5" />
        </Button>
        <Button
          size="sm"
          variant="ghost"
          onClick={onDelete}
          disabled={deleting}
          aria-label={`Delete ${source.metadata.name}`}
        >
          <Trash2 className="h-3.5 w-3.5 text-danger" />
        </Button>
      </div>
    </li>
  );
}
