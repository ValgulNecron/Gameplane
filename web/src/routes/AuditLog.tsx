import { useMemo, useState, type ReactNode } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { Download, RefreshCw } from "lucide-react";
import type { AuditEvent } from "@/types";
import { Audit } from "@/lib/endpoints";
import { PageHeader } from "@/components/PageHeader";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn, formatRelative } from "@/lib/utils";

const PAGE_SIZE = 100;

type StatusClass = "all" | "2xx" | "4xx" | "5xx";
type MethodFilter = "all" | "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

export function AuditLogPage() {
  const [statusClass, setStatusClass] = useState<StatusClass>("all");
  const [methodFilter, setMethodFilter] = useState<MethodFilter>("all");
  const [actorQ, setActorQ] = useState("");

  const query = useInfiniteQuery({
    queryKey: ["audit"],
    queryFn: ({ pageParam }: { pageParam: number }) =>
      Audit.page(PAGE_SIZE, pageParam),
    initialPageParam: 0,
    getNextPageParam: (last) =>
      last.length === PAGE_SIZE ? last[last.length - 1].id : undefined,
  });

  const all: AuditEvent[] = useMemo(
    () => query.data?.pages.flat() ?? [],
    [query.data],
  );

  const filtered = useMemo(() => {
    const a = actorQ.trim().toLowerCase();
    return all.filter((e) => {
      if (statusClass !== "all" && statusBucket(e.status) !== statusClass) return false;
      if (methodFilter !== "all" && e.method !== methodFilter) return false;
      if (a && !e.actor.toLowerCase().includes(a)) return false;
      return true;
    });
  }, [all, statusClass, methodFilter, actorQ]);

  const totals = useMemo<Record<StatusClass, number>>(() => {
    const r: Record<StatusClass, number> = { all: all.length, "2xx": 0, "4xx": 0, "5xx": 0 };
    for (const e of all) {
      const b = statusBucket(e.status);
      if (b !== "other") r[b]++;
    }
    return r;
  }, [all]);

  return (
    <div className="space-y-5 p-6">
      <PageHeader
        title="Audit log"
        subtitle="Mutating control-plane requests, newest first."
        actions={
          <>
            <Button
              variant="outline"
              onClick={() => exportCsv(filtered)}
              disabled={filtered.length === 0}
            >
              <Download className="h-4 w-4" /> Export CSV
            </Button>
            <Button
              onClick={() => query.refetch()}
              disabled={query.isFetching && !query.isFetchingNextPage}
            >
              <RefreshCw
                className={cn(
                  "h-4 w-4",
                  query.isFetching && !query.isFetchingNextPage && "animate-spin",
                )}
              />
              Refresh
            </Button>
          </>
        }
      />

      <div className="flex flex-wrap items-center gap-3">
        <div className="flex gap-1 rounded-md border border-border bg-surface/40 p-1">
          {(["all", "2xx", "4xx", "5xx"] as StatusClass[]).map((s) => (
            <button
              key={s}
              onClick={() => setStatusClass(s)}
              className={cn(
                "rounded px-3 py-1 text-xs font-medium",
                statusClass === s
                  ? "bg-primary/15 text-primary"
                  : "text-muted hover:text-fg",
              )}
            >
              {labelFor(s)} · {totals[s] ?? 0}
            </button>
          ))}
        </div>

        <select
          value={methodFilter}
          onChange={(e) => setMethodFilter(e.target.value as MethodFilter)}
          className="h-9 rounded-md border border-border bg-surface px-2 text-sm text-fg"
        >
          <option value="all">All methods</option>
          {(["GET", "POST", "PUT", "PATCH", "DELETE"] as const).map((m) => (
            <option key={m} value={m}>{m}</option>
          ))}
        </select>

        <Input
          placeholder="Filter by actor…"
          value={actorQ}
          onChange={(e) => setActorQ(e.target.value)}
          className="w-64"
        />

        <div className="ml-auto text-xs text-muted">
          {filtered.length} of {all.length} loaded
          {query.hasNextPage && " · more available"}
        </div>
      </div>

      <Card className="overflow-hidden p-0">
        <div className="max-h-[70vh] overflow-auto scrollbar-thin">
          <table className="w-full text-xs">
            <thead className="sticky top-0 z-10 bg-surface/95 text-left uppercase tracking-wider text-muted backdrop-blur">
              <tr>
                <Th className="w-[160px]">Time</Th>
                <Th className="w-[140px]">Actor</Th>
                <Th>Action</Th>
                <Th className="w-[80px]">Method</Th>
                <Th className="w-[90px]">Access</Th>
                <Th className="w-[120px]">IP</Th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border font-mono">
              {filtered.length === 0 && !query.isLoading && (
                <tr>
                  <td colSpan={6} className="px-5 py-12 text-center text-muted">
                    {all.length === 0
                      ? "No audit events yet."
                      : "No events match the active filters."}
                  </td>
                </tr>
              )}
              {filtered.map((e) => {
                const access = accessOutcome(e.status);
                return (
                  <tr key={e.id} className="hover:bg-surface/40">
                    <td className="px-5 py-2 text-muted" title={e.ts}>
                      {formatRelative(e.ts)}
                    </td>
                    <td className="px-5 py-2 text-fg">{e.actor}</td>
                    <td className="px-5 py-2">
                      <span
                        className="block max-w-[480px] truncate text-fg"
                        title={`${e.method} ${e.path}`}
                      >
                        {auditAction(e)}
                      </span>
                    </td>
                    <td className="px-5 py-2">
                      <MethodPill method={e.method} />
                    </td>
                    <td className={cn("px-5 py-2", access.tone)} title={`HTTP ${e.status}`}>
                      {access.label}
                    </td>
                    <td className="px-5 py-2 text-muted">{e.ip || "—"}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>

        <div className="flex items-center justify-between border-t border-border px-5 py-3">
          <div className="text-xs text-muted">
            Page size {PAGE_SIZE}. Older events load on demand.
          </div>
          <Button
            variant="outline"
            onClick={() => query.fetchNextPage()}
            disabled={!query.hasNextPage || query.isFetchingNextPage}
          >
            {query.isFetchingNextPage
              ? "Loading…"
              : query.hasNextPage
                ? "Load more"
                : "End of log"}
          </Button>
        </div>
      </Card>
    </div>
  );
}

function Th({ children, className }: { children: ReactNode; className?: string }) {
  return <th className={cn("px-5 py-2 font-medium", className)}>{children}</th>;
}

// auditAction renders an audit row's method+path as a human-readable action
// ("Started server alpha") instead of raw HTTP, falling back to a generic
// "<verb> <resource>" when the route isn't specially known.
const VERB: Record<string, string> = {
  POST: "Created", PUT: "Updated", PATCH: "Updated", DELETE: "Deleted", GET: "Viewed",
};
export function auditAction(e: { method: string; path: string; target?: string }): string {
  // Strip a leading /api/v<n> prefix so route matching is version-agnostic.
  const p = e.path.replace(/^\/api\/v\d+/, "").replace(/\/+$/, "") || "/";
  const t = e.target ? ` ${e.target}` : "";
  // Lifecycle verbs are encoded as POST /servers/{name}:<verb>.
  const colon = /\/servers\/[^/]+:([a-z-]+)$/.exec(p);
  if (colon) {
    const verb = colon[1];
    const nice: Record<string, string> = {
      start: "Started", stop: "Stopped", restart: "Restarted", clone: "Cloned",
      "wipe-data": "Wiped data on",
    };
    return `${nice[verb] ?? verb} server${t}`;
  }
  const m = (s: string) => p === s || p.startsWith(s + "/");
  if (m("/servers")) return `${VERB[e.method] ?? e.method} server${t}`;
  if (m("/backups")) return `${VERB[e.method] ?? e.method} backup${t}`;
  if (m("/restores")) return e.method === "POST" ? `Restored backup${t}` : `${VERB[e.method]} restore${t}`;
  if (m("/schedules")) return `${VERB[e.method] ?? e.method} backup schedule${t}`;
  if (m("/users")) return `${VERB[e.method] ?? e.method} user${t}`;
  if (m("/roles")) return `${VERB[e.method] ?? e.method} role${t}`;
  if (m("/modules")) return `${VERB[e.method] ?? e.method} module${t}`;
  if (m("/modules/sources")) return `${VERB[e.method] ?? e.method} module source${t}`;
  if (m("/destinations") || m("/backup-destinations")) return `${VERB[e.method] ?? e.method} backup destination${t}`;
  if (m("/admin/config")) return "Updated settings";
  if (m("/auth/login")) return "Signed in";
  if (m("/cluster")) return `${VERB[e.method] ?? e.method} cluster${t}`;
  // Fallback: verb + last path segment.
  const seg = p.split("/").filter(Boolean).pop() ?? "resource";
  return `${VERB[e.method] ?? e.method} ${seg}`;
}

// accessOutcome maps an HTTP status to the allow/deny framing the audit log
// presents (403 = explicitly denied; other 4xx/5xx = failed; 2xx/3xx = ok).
function accessOutcome(status: number): { label: string; tone: string } {
  if (status === 403) return { label: "Denied", tone: "text-danger" };
  if (status >= 500) return { label: "Error", tone: "text-danger" };
  if (status >= 400) return { label: "Failed", tone: "text-warning" };
  return { label: "Allowed", tone: "text-success" };
}

function MethodPill({ method }: { method: string }) {
  const tone = methodTone(method);
  return (
    <span
      className={cn(
        "inline-flex h-5 min-w-[44px] items-center justify-center rounded px-1.5 text-[10px] font-semibold uppercase",
        tone,
      )}
    >
      {method}
    </span>
  );
}

function methodTone(m: string): string {
  switch (m) {
    case "GET":    return "bg-muted/30 text-muted";
    case "POST":   return "bg-success/15 text-success";
    case "PUT":
    case "PATCH":  return "bg-warning/15 text-warning";
    case "DELETE": return "bg-danger/15 text-danger";
    default:       return "bg-muted/30 text-muted";
  }
}

function statusBucket(s: number): "2xx" | "4xx" | "5xx" | "other" {
  if (s >= 200 && s < 300) return "2xx";
  if (s >= 400 && s < 500) return "4xx";
  if (s >= 500 && s < 600) return "5xx";
  return "other";
}

function labelFor(s: StatusClass): string {
  if (s === "all") return "All";
  return s;
}

function exportCsv(events: AuditEvent[]) {
  const head = ["id", "ts", "actor", "method", "path", "status", "ip", "target"];
  const rows = events.map((e) => [
    e.id, e.ts, e.actor, e.method, e.path, e.status, e.ip ?? "", e.target ?? "",
  ]);
  const csv = [head, ...rows]
    .map((r) => r.map(csvCell).join(","))
    .join("\n");
  const blob = new Blob([csv], { type: "text/csv" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `gameplane-audit-${new Date().toISOString().replace(/[:.]/g, "-")}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}

function csvCell(v: string | number): string {
  const s = String(v);
  if (/[",\n]/.test(s)) return `"${s.replace(/"/g, '""')}"`;
  return s;
}
