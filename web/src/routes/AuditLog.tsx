import { useMemo, useState, type ReactNode } from "react";
import { useInfiniteQuery, useMutation, useQuery, type UseQueryResult } from "@tanstack/react-query";
import { Download, RefreshCw } from "lucide-react";
import type { AuditEvent, AuditVerifyResult } from "@/types";
import { Audit, type AuditExportFilter } from "@/lib/endpoints";
import { Button, Card, Input, Chip, Table } from "@heroui/react";
import { PageHeader } from "@/components/PageHeader";
import { AuditIntegrityBanner } from "@/components/hero/AuditIntegrityBanner";
import { cn, formatRelative } from "@/lib/utils";

const PAGE_SIZE = 100;

type StatusClass = "all" | "2xx" | "4xx" | "5xx";
type MethodFilter = "all" | "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

export function AuditLogPage() {
  const [statusClass, setStatusClass] = useState<StatusClass>("all");
  const [methodFilter, setMethodFilter] = useState<MethodFilter>("all");
  const [actorQ, setActorQ] = useState("");

  const verifyQuery = useQuery({
    queryKey: ["audit-verify"],
    queryFn: () => Audit.verify(),
  });

  const query = useInfiniteQuery({
    queryKey: ["audit"],
    queryFn: ({ pageParam }: { pageParam: number }) =>
      Audit.page(PAGE_SIZE, pageParam),
    initialPageParam: 0,
    getNextPageParam: (last) =>
      last.length === PAGE_SIZE ? last[last.length - 1].id : undefined,
  });

  const exportMutation = useMutation({
    mutationFn: (filter: AuditExportFilter) => Audit.exportCsv(filter),
    onSuccess: (blob) => {
      const filename = "gameplane-audit-" + new Date().toISOString().replace(/[:.]/g, "-") + ".csv";
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = filename;
      a.click();
      URL.revokeObjectURL(url);
    },
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
              onPress={() => {
                const filter: AuditExportFilter = {};
                if (actorQ.trim()) filter.actor = actorQ.trim();
                if (methodFilter !== "all") filter.method = methodFilter;
                if (statusClass !== "all") filter.status = statusClass;
                exportMutation.mutate(filter);
              }}
              isDisabled={exportMutation.isPending}
            >
              <Download className="h-4 w-4" /> Export CSV
            </Button>
            <Button
              onPress={() => void query.refetch()}
              isDisabled={query.isFetching && !query.isFetchingNextPage}
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

      {renderIntegrityBanner(verifyQuery)}

      <div className="flex flex-wrap items-center gap-3">
        <div className="flex gap-1">
          {(["all", "2xx", "4xx", "5xx"] as StatusClass[]).map((s) => (
            <Button
              key={s}
              variant={statusClass === s ? "primary" : "ghost"}
              size="sm"
              onPress={() => setStatusClass(s)}
              className="text-xs"
            >
              {labelFor(s)} · {totals[s] ?? 0}
            </Button>
          ))}
        </div>

        <select
          value={methodFilter}
          onChange={(e) => setMethodFilter(e.target.value as MethodFilter)}
          className="h-9 rounded-md border border-border bg-surface px-2 text-sm text-foreground"
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
          variant="secondary"
        />

        <div className="ml-auto text-xs text-muted">
          {filtered.length} of {all.length} loaded
          {query.hasNextPage && " · more available"}
        </div>
      </div>

      <Card className="overflow-hidden p-0">
        <Table.Root className="bg-transparent text-xs">
          <Table.ScrollContainer className="max-h-[70vh]">
            <Table.Content aria-label="Audit log">
              <Table.Header>
                <Table.Column id="time" className="w-40">Time</Table.Column>
                <Table.Column id="actor" className="w-36">Actor</Table.Column>
                <Table.Column id="action">Action</Table.Column>
                <Table.Column id="method" className="w-20">Method</Table.Column>
                <Table.Column id="access" className="w-24">Access</Table.Column>
                <Table.Column id="ip" className="w-32">IP</Table.Column>
              </Table.Header>
              <Table.Body
                renderEmptyState={() =>
                  <span>
                    {all.length === 0
                      ? "No audit events yet."
                      : "No events match the active filters."}
                  </span>
                }
              >
                {filtered.map((e) => {
                  const access = accessOutcome(e.status);
                  return (
                    <Table.Row key={e.id}>
                      <Table.Cell className="text-muted">
                        <span title={e.ts}>{formatRelative(e.ts)}</span>
                      </Table.Cell>
                      <Table.Cell>{e.actor}</Table.Cell>
                      <Table.Cell>
                        <span
                          className="block max-w-[480px] truncate"
                          title={`${e.method} ${e.path}`}
                        >
                          {auditAction(e)}
                        </span>
                      </Table.Cell>
                      <Table.Cell>
                        <MethodPill method={e.method} />
                      </Table.Cell>
                      <Table.Cell className={access.tone}>
                        <span title={`HTTP ${e.status}`}>{access.label}</span>
                      </Table.Cell>
                      <Table.Cell className="text-muted">{e.ip || "—"}</Table.Cell>
                    </Table.Row>
                  );
                })}
              </Table.Body>
            </Table.Content>
          </Table.ScrollContainer>
        </Table.Root>

        <div className="flex items-center justify-between border-t border-border px-5 py-3">
          <div className="text-xs text-muted">
            Page size {PAGE_SIZE}. Older events load on demand.
          </div>
          <Button
            variant="outline"
            size="sm"
            onPress={() => void query.fetchNextPage()}
            isDisabled={!query.hasNextPage || query.isFetchingNextPage}
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

function renderIntegrityBanner(query: UseQueryResult<AuditVerifyResult>): ReactNode {
  const data = query.data;
  if (query.isLoading) {
    return null;
  }
  if (query.isError) {
    return (
      <div className="flex items-center justify-between rounded-md border border-border bg-surface/50 px-4 py-3">
        <div className="text-sm text-muted">Integrity status unavailable</div>
      </div>
    );
  }
  if (!data) {
    return null;
  }
  if (data.ok) {
    return (
      <div className="flex items-center justify-between rounded-md border border-success bg-success/5 px-4 py-3">
        <div className="text-sm font-medium text-success">Audit chain verified — no tampering detected</div>
        <Button
          size="sm"
          variant="ghost"
          onPress={() => void query.refetch()}
          isDisabled={query.isFetching}
        >
          Re-check
        </Button>
      </div>
    );
  }
  return (
    <AuditIntegrityBanner
      message={data.message || `Integrity check failed — chain breaks at event #${data.firstBadId}`}
    />
  );
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
  const variant = methodVariant(method);
  return (
    <Chip
      size="sm"
      variant="soft"
      className="text-[10px] font-semibold uppercase"
      color={variant}
    >
      {method}
    </Chip>
  );
}

function methodVariant(m: string): "default" | "success" | "warning" | "danger" {
  switch (m) {
    case "GET":    return "default";
    case "POST":   return "success";
    case "PUT":
    case "PATCH":  return "warning";
    case "DELETE": return "danger";
    default:       return "default";
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
