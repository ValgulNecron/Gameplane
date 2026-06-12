import { useQuery } from "@tanstack/react-query";
import { CheckCircle2, CircleAlert, Circle, RefreshCcw } from "lucide-react";

import { Card } from "@/components/ui/card";
import { ModuleSources } from "@/lib/endpoints";
import { formatRelative } from "@/lib/utils";
import type { ModuleSource } from "@/types";

// ModuleSourcesPanel is a read-only listing of every ModuleSource. We
// intentionally don't ship add/edit forms in v1 — clusters declare
// their sources via Helm values or `kubectl apply -f` so the catalog
// is reproducible from infra-as-code, not click-through state.
export function ModuleSourcesPanel() {
  const { data, isLoading, isError } = useQuery({
    queryKey: ["module-sources"],
    queryFn: () => ModuleSources.list(),
    refetchInterval: 10_000,
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
      <div>
        <div className="font-medium">Module sources</div>
        <div className="pt-0.5 text-xs text-muted">
          OCI registries the operator pulls module bundles from. Manage
          via Helm <code className="font-mono">defaultModuleSource</code>{" "}
          values or by applying <code className="font-mono">ModuleSource</code>{" "}
          resources directly.
        </div>
      </div>

      {sources.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border bg-card/40 p-6 text-center text-sm text-muted">
          No <code className="font-mono">ModuleSource</code> resources configured.
        </div>
      ) : (
        <ul className="divide-y divide-border">
          {sources.map((s) => (
            <SourceRow key={s.metadata.name} source={s} />
          ))}
        </ul>
      )}
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

function SourceRow({ source }: { source: ModuleSource }) {
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
    </li>
  );
}
