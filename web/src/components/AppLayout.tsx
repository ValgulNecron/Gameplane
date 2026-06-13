import { Outlet, Link, useLocation, useMatches } from "@tanstack/react-router";
import type { ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Archive,
  Bell,
  ChevronDown,
  LayoutDashboard,
  LogOut,
  Package,
  ScrollText,
  Search,
  Server,
  Settings,
  ShieldCheck,
  Users,
} from "lucide-react";
import { APIError } from "@/lib/api";
import { Auth, Cluster as ClusterAPI, Servers } from "@/lib/endpoints";
import { useMe } from "@/lib/auth";
import type { ClusterInfo, User } from "@/types";
import { cn } from "@/lib/utils";
import { openEventStream, queryKeyForKind, type KestrelEvent } from "@/lib/sse";
import { useEffect, useState } from "react";

function useClusterInfo() {
  return useQuery({
    queryKey: ["cluster-info"],
    queryFn: () => ClusterAPI.info().catch(() => ({} as ClusterInfo)),
    retry: false,
    staleTime: 60_000,
  });
}

export function AppLayout() {
  const { data: me, error } = useMe();
  const { data: cluster } = useClusterInfo();

  useEffect(() => {
    if (error instanceof APIError && error.status === 401) {
      location.assign("/login");
    }
  }, [error]);

  return (
    <div className="flex h-full bg-background text-fg">
      <Sidebar role={me?.role} me={me} clusterName={cluster?.clusterName} />
      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar user={me} />
        <main className="flex-1 overflow-auto scrollbar-thin">
          <Outlet />
        </main>
      </div>
    </div>
  );
}

interface NavItem {
  to: string;
  label: string;
  icon: typeof LayoutDashboard;
}

function Sidebar({ role, me, clusterName }: { role?: User["role"]; me?: User; clusterName?: string }) {
  const general: NavItem[] = [
    { to: "/",        label: "Dashboard", icon: LayoutDashboard },
    { to: "/servers", label: "Servers",   icon: Server },
    { to: "/modules", label: "Modules",   icon: Package },
    { to: "/backups", label: "Backups",   icon: Archive },
  ];
  const admin: NavItem[] = [];
  if (role === "admin" || role === "operator") {
    admin.push({ to: "/cluster", label: "Cluster", icon: Server });
  }
  if (role === "admin") {
    admin.push(
      { to: "/users",       label: "Users & RBAC", icon: Users },
      { to: "/admin/audit", label: "Audit log",    icon: ScrollText },
      { to: "/admin",       label: "Settings",     icon: Settings },
    );
  }

  return (
    <aside className="flex w-[260px] shrink-0 flex-col border-r border-border bg-surface/60">
      <div className="flex items-center gap-2 px-5 py-4">
        <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary/15">
          <ShieldCheck className="h-4 w-4 text-primary" />
        </div>
        <div className="leading-tight">
          <div className="font-mono text-base font-semibold text-fg">kestrel</div>
          <div className="text-[11px] text-muted">{clusterName || "—"}</div>
        </div>
      </div>

      <nav className="flex-1 overflow-auto px-3 py-2 scrollbar-thin">
        <SectionLabel>General</SectionLabel>
        <NavGroup items={general} />
        {admin.length > 0 && (
          <>
            <div className="h-3" />
            <SectionLabel>Admin</SectionLabel>
            <NavGroup items={admin} />
          </>
        )}
      </nav>

      <ProfileFooter me={me} />
    </aside>
  );
}

function SectionLabel({ children }: { children: ReactNode }) {
  return (
    <div className="px-3 pb-2 pt-3 text-[10px] font-semibold uppercase tracking-widest text-muted">
      {children}
    </div>
  );
}

function NavGroup({ items }: { items: NavItem[] }) {
  return (
    <ul className="flex flex-col gap-0.5">
      {items.map(({ to, label, icon: Icon }) => {
        // A parent link (e.g. /admin "Settings") must match exactly when a
        // sibling links to a sub-path (e.g. /admin/audit), otherwise
        // prefix-matching lights up both. Links with no sibling sub-path
        // keep prefix matching, so e.g. Servers stays active on
        // /servers/:name.
        const exact = to === "/" || items.some((o) => o.to !== to && o.to.startsWith(to + "/"));
        return (
          <li key={to}>
            <Link
              to={to}
              className={cn(
                "group flex items-center gap-3 rounded-md px-3 py-2 text-sm text-muted transition-colors",
                "hover:bg-border/60 hover:text-fg",
                "[&.active]:bg-primary/10 [&.active]:text-primary",
              )}
              activeProps={{ className: "active" }}
              activeOptions={{ exact }}
            >
              <Icon className="h-[18px] w-[18px]" />
              <span>{label}</span>
            </Link>
          </li>
        );
      })}
    </ul>
  );
}

function ProfileFooter({ me }: { me?: User }) {
  const name = me?.displayName || me?.username || "guest";
  const initials = name.slice(0, 2).toUpperCase();
  return (
    <div className="border-t border-border px-3 py-3">
      <div className="group flex items-center gap-3 rounded-md px-2 py-1.5">
        <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/20 font-mono text-xs text-primary">
          {initials}
        </div>
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm text-fg">{name}</div>
          <div className="truncate text-[11px] text-muted">{me?.role ?? "—"}</div>
        </div>
        <button
          title="Sign out"
          className="rounded p-1 text-muted hover:bg-border/60 hover:text-fg"
          onClick={async () => {
            await Auth.logout().catch(() => {});
            location.assign("/login");
          }}
        >
          <LogOut className="h-4 w-4" />
        </button>
      </div>
    </div>
  );
}

function Topbar({ user }: { user?: User }) {
  const { pathname } = useLocation();
  const matches = useMatches();

  const crumbs = buildCrumbs(pathname, matches);
  const name = user?.displayName || user?.username || "guest";
  const initials = name.slice(0, 2).toUpperCase();

  return (
    <header className="flex h-14 items-center justify-between gap-4 border-b border-border bg-background px-6">
      <Breadcrumbs items={crumbs} />
      <div className="flex items-center gap-3">
        <GlobalSearch />
        <Notifications />
        <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/20 font-mono text-xs text-primary">
          {initials}
        </div>
      </div>
    </header>
  );
}

interface Notice {
  id: number;
  text: string;
  at: string;
}

// Notifications opens the /events SSE stream once for the app: each watch
// event invalidates the matching TanStack Query cache (so views refresh
// without waiting for the next poll) and is buffered into a dropdown
// panel. The badge shows the unread count.
function Notifications() {
  const qc = useQueryClient();
  const [notices, setNotices] = useState<Notice[]>([]);
  const [open, setOpen] = useState(false);
  const [unread, setUnread] = useState(0);

  useEffect(() => {
    let seq = 0;
    const dispose = openEventStream({
      onEvent: (ev: KestrelEvent) => {
        const key = queryKeyForKind(ev.kind);
        if (key) void qc.invalidateQueries({ queryKey: key });
        const name = ev.object?.metadata?.name ?? "";
        const verb = ev.eventType.toLowerCase();
        seq += 1;
        const notice: Notice = {
          id: seq,
          text: `${verb} ${ev.kind.replace(/s$/, "")} ${name}`.trim(),
          at: new Date().toLocaleTimeString(),
        };
        setNotices((prev) => [notice, ...prev].slice(0, 50));
        setUnread((u) => u + 1);
      },
    });
    return dispose;
  }, [qc]);

  return (
    <div className="relative">
      <button
        type="button"
        aria-label="Notifications"
        title="Notifications"
        onClick={() => {
          setOpen((o) => !o);
          setUnread(0);
        }}
        className="relative rounded-md p-2 text-muted hover:bg-surface hover:text-fg"
      >
        <Bell className="h-[18px] w-[18px]" />
        {unread > 0 && (
          <span className="absolute right-1 top-1 flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-primary px-1 text-[9px] font-medium text-primary-fg">
            {unread > 9 ? "9+" : unread}
          </span>
        )}
      </button>
      {open && (
        <div className="absolute right-0 z-30 mt-1 w-72 overflow-hidden rounded-md border border-border bg-background shadow-lg">
          <div className="border-b border-border px-3 py-2 text-xs font-medium text-muted">
            Recent activity
          </div>
          {notices.length === 0 ? (
            <div className="px-3 py-4 text-sm text-muted">No recent activity.</div>
          ) : (
            <ul className="max-h-80 overflow-auto">
              {notices.map((n) => (
                <li key={n.id} className="flex items-center justify-between gap-2 px-3 py-2 text-sm">
                  <span className="truncate font-mono text-xs">{n.text}</span>
                  <span className="shrink-0 text-[10px] text-muted">{n.at}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}

// GlobalSearch is the header search box. It filters the (cached) server
// list by name and shows a dropdown of matches; selecting one navigates
// to its detail page. Enter jumps to the first match.
function GlobalSearch() {
  const [q, setQ] = useState("");
  const [open, setOpen] = useState(false);
  const { data } = useQuery({
    queryKey: ["servers"],
    queryFn: () => Servers.list(),
    staleTime: 10_000,
  });
  const query = q.trim().toLowerCase();
  const matches =
    query.length > 0
      ? (data?.items ?? [])
          .filter((s) => s.metadata.name.toLowerCase().includes(query))
          .slice(0, 6)
      : [];

  return (
    <div className="relative hidden w-72 md:block">
      <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" />
      <input
        type="search"
        aria-label="Search servers"
        placeholder="Search servers…"
        value={q}
        onChange={(e) => {
          setQ(e.target.value);
          setOpen(true);
        }}
        onFocus={() => setOpen(true)}
        // Delay so a result click registers before the dropdown unmounts.
        onBlur={() => setTimeout(() => setOpen(false), 120)}
        onKeyDown={(e) => {
          if (e.key === "Escape") setOpen(false);
          if (e.key === "Enter" && matches.length > 0) {
            location.assign(`/servers/${matches[0].metadata.name}`);
          }
        }}
        className="h-9 w-full rounded-md border border-border bg-surface pl-9 pr-3 text-sm text-fg placeholder:text-muted focus:border-primary focus:outline-none"
      />
      {open && query.length > 0 && (
        <ul className="absolute z-20 mt-1 w-full overflow-hidden rounded-md border border-border bg-background shadow-lg">
          {matches.length === 0 ? (
            <li className="px-3 py-2 text-sm text-muted">No servers match.</li>
          ) : (
            matches.map((s) => (
              <li key={s.metadata.name}>
                <Link
                  to="/servers/$name"
                  params={{ name: s.metadata.name }}
                  onClick={() => {
                    setOpen(false);
                    setQ("");
                  }}
                  className="flex items-center gap-2 px-3 py-2 text-sm text-fg hover:bg-surface"
                >
                  <Server className="h-3.5 w-3.5 text-muted" />
                  <span className="truncate font-mono">{s.metadata.name}</span>
                </Link>
              </li>
            ))
          )}
        </ul>
      )}
    </div>
  );
}

interface Crumb {
  label: string;
  to?: string;
}

function buildCrumbs(pathname: string, _matches: unknown[]): Crumb[] {
  // Map route paths → human labels. Keeps breadcrumbs simple without
  // requiring per-route loader data.
  const crumbs: Crumb[] = [{ label: "kestrel", to: "/" }];
  const parts = pathname.split("/").filter(Boolean);
  const labels: Record<string, string> = {
    servers: "Servers",
    modules: "Modules",
    cluster: "Cluster",
    users:   "Users & RBAC",
    admin:   "Settings",
    audit:   "Audit log",
    backups: "Backups",
    new:     "New",
  };
  let acc = "";
  for (const p of parts) {
    acc += "/" + p;
    crumbs.push({ label: labels[p] ?? p, to: acc });
  }
  if (parts.length === 0) crumbs.push({ label: "Servers" });
  return crumbs;
}

function Breadcrumbs({ items }: { items: Crumb[] }) {
  return (
    <nav className="flex min-w-0 items-center gap-2 text-sm text-muted">
      {items.map((c, i) => {
        const last = i === items.length - 1;
        return (
          <span key={c.to ?? c.label} className="flex min-w-0 items-center gap-2">
            {c.to && !last ? (
              <Link to={c.to} className="hover:text-fg truncate">
                {c.label}
              </Link>
            ) : (
              <span className={cn("truncate", last && "text-fg")}>{c.label}</span>
            )}
            {!last && <ChevronDown aria-hidden="true" className="h-3 w-3 -rotate-90 text-muted" />}
          </span>
        );
      })}
    </nav>
  );
}
