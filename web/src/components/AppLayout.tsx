import { Outlet, useLocation } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import {
  Archive,
  LayoutDashboard,
  Package,
  ScrollText,
  Server,
  Settings,
  ShieldCheck,
  Terminal,
  Users,
} from "lucide-react";
import { Skeleton } from "@heroui/react";
import { APIError } from "@/lib/api";
import { Cluster as ClusterAPI, Auth } from "@/lib/endpoints";
import { useMe, can } from "@/lib/auth";
import type { ClusterInfo } from "@/types";
import { useEffect, useState } from "react";
import { ClusterSelector } from "@/components/ClusterSelector";
import { AppShell } from "@/components/hero/AppShell";
import { Sidebar, type SidebarNavGroup } from "@/components/hero/Sidebar";
import { TopBar } from "@/components/hero/TopBar";
import { Breadcrumbs, buildCrumbs } from "@/components/hero/Breadcrumbs";
import { GlobalSearch } from "@/components/hero/GlobalSearch";
import { NotificationsPanel } from "@/components/hero/NotificationsPanel";
import type { AppearanceMode } from "@/components/hero/AppearanceToggle";

// The localStorage key the theme boot script in index.html reads before
// React mounts — must stay in sync (see index.html and theme-tokens.md).
// Note: HeroUI's own `useTheme()` hook hardcodes a different key
// ("heroui-theme"), which would silently diverge from the boot script, so
// theme state is owned here rather than via that hook (deviation from the
// contract's preferred approach, flagged for the maintainer).
const THEME_STORAGE_KEY = "gameplane-theme";

function readStoredTheme(): AppearanceMode {
  try {
    const saved = localStorage.getItem(THEME_STORAGE_KEY);
    if (saved === "light" || saved === "dark" || saved === "system") return saved;
  } catch {
    // localStorage unavailable — fall back to system default.
  }
  return "system";
}

function applyTheme(mode: AppearanceMode) {
  const resolved =
    mode === "system"
      ? window.matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light"
      : mode;
  document.documentElement.classList.remove("dark", "light");
  document.documentElement.classList.add(resolved);
  document.documentElement.dataset.theme = resolved;
}

function useAppearance(): [AppearanceMode, (mode: AppearanceMode) => void] {
  const [theme, setThemeState] = useState<AppearanceMode>(readStoredTheme);

  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  const setTheme = (mode: AppearanceMode) => {
    setThemeState(mode);
    try {
      localStorage.setItem(THEME_STORAGE_KEY, mode);
    } catch {
      // localStorage unavailable — theme still applies for this session.
    }
  };

  return [theme, setTheme];
}

function useClusterInfo() {
  return useQuery({
    queryKey: ["cluster-info"],
    queryFn: () => ClusterAPI.info().catch(() => ({} as ClusterInfo)),
    retry: false,
    staleTime: 60_000,
  });
}

export function AppLayout() {
  const { data: me, error, isLoading } = useMe();
  const { data: cluster } = useClusterInfo();
  const { pathname } = useLocation();
  const [theme, setTheme] = useAppearance();
  // Below `lg`, the fixed sidebar becomes an off-canvas drawer toggled by
  // the TopBar's hamburger button. Desktop (`lg`+) keeps the always-on
  // sidebar and never mounts the drawer.
  const [drawerOpen, setDrawerOpen] = useState(false);

  useEffect(() => {
    if (error instanceof APIError && error.status === 401) {
      location.assign("/login");
    }
  }, [error]);

  if (isLoading) return <AppShellSkeleton />;

  const onLogout = async () => {
    await Auth.logout().catch(() => {});
    location.assign("/login");
  };

  const navItems: SidebarNavGroup[] = [
    {
      label: "General",
      items: [
        { to: "/", label: "Dashboard", icon: LayoutDashboard },
        { to: "/servers", label: "Servers", icon: Server },
        { to: "/modules", label: "Modules", icon: Package },
        { to: "/backups", label: "Backups", icon: Archive },
      ],
    },
    {
      label: "Admin",
      items: [
        ...(can(me, "servers:write")
          ? [{ to: "/cluster", label: "Cluster", icon: Server }]
          : []),
        ...(can(me, "users:manage")
          ? [{ to: "/users", label: "Users & RBAC", icon: Users }]
          : []),
        ...(can(me, "audit:read")
          ? [{ to: "/admin/audit", label: "Audit log", icon: ScrollText }]
          : []),
        // System logs match the API's /admin/system-logs guard: the admin
        // wildcard permission, not a narrower named one.
        ...(can(me, "*")
          ? [{ to: "/admin/logs", label: "System logs", icon: Terminal }]
          : []),
        ...(can(me, "config:manage")
          ? [{ to: "/admin", label: "Settings", icon: Settings }]
          : []),
      ],
    },
  ];

  const crumbs = buildCrumbs(pathname);

  return (
    <>
      <AppShell
        sidebar={
          <Sidebar
            navItems={navItems}
            clusterName={cluster?.clusterName}
            user={me}
            variant="fixed"
            onLogout={onLogout}
            theme={theme}
            onThemeChange={setTheme}
          />
        }
        topBar={
          <TopBar
            breadcrumbs={<Breadcrumbs items={crumbs} />}
            clusterSelector={<ClusterSelector />}
            search={<GlobalSearch />}
            notifications={<NotificationsPanel />}
            user={me}
            onMenuClick={() => setDrawerOpen(true)}
          />
        }
      >
        <Outlet />
      </AppShell>

      {/* Mobile off-canvas drawer — rendered outside AppShell so it never
          nests inside the <main> landmark; HeroUI's Drawer portals its
          content regardless, but keeping it a sibling here avoids
          confusing the accessibility tree in source order too. */}
      <Sidebar
        navItems={navItems}
        clusterName={cluster?.clusterName}
        user={me}
        variant="drawer"
        isOpen={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        onNavigate={() => setDrawerOpen(false)}
        onLogout={onLogout}
        theme={theme}
        onThemeChange={setTheme}
      />
    </>
  );
}

function AppShellSkeleton() {
  return (
    <div className="flex h-full bg-background text-fg" aria-busy="true" aria-label="Loading">
      {/* Desktop sidebar — always rendered, hidden on mobile */}
      <div className="hidden lg:flex">
        <aside className="flex w-[260px] shrink-0 flex-col border-r border-border bg-surface/60">
          {/* Brand block — real wordmark, skeleton cluster line */}
          <div className="flex items-center gap-2 px-5 py-4">
            <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary/15">
              <ShieldCheck className="h-4 w-4 text-primary" />
            </div>
            <div className="min-w-0 flex-1 leading-tight">
              <div className="font-mono text-base font-semibold text-fg">gameplane</div>
              <Skeleton className="h-3 w-20 rounded" />
            </div>
          </div>

          {/* Nav area with skeleton items */}
          <nav className="flex-1 overflow-auto px-3 py-2 scrollbar-thin">
            <div className="px-3 pb-2 pt-3 text-[10px] font-semibold uppercase tracking-widest text-muted">
              General
            </div>
            <ul className="flex flex-col gap-0.5">
              {Array.from({ length: 4 }).map((_, i) => (
                <li key={`general-${i}`}>
                  <div className="flex items-center gap-3 rounded-md px-3 py-2">
                    <Skeleton className="h-[18px] w-[18px] shrink-0 rounded" />
                    <Skeleton className="h-3 flex-1 rounded" />
                  </div>
                </li>
              ))}
            </ul>

            <div className="h-3" />
            <div className="px-3 pb-2 pt-3 text-[10px] font-semibold uppercase tracking-widest text-muted">
              Admin
            </div>
            <ul className="flex flex-col gap-0.5">
              {Array.from({ length: 3 }).map((_, i) => (
                <li key={`admin-${i}`}>
                  <div className="flex items-center gap-3 rounded-md px-3 py-2">
                    <Skeleton className="h-[18px] w-[18px] shrink-0 rounded" />
                    <Skeleton className="h-3 flex-1 rounded" />
                  </div>
                </li>
              ))}
            </ul>
          </nav>

          {/* Profile footer skeleton */}
          <div className="border-t border-border px-3 py-3">
            <div className="flex items-center gap-3 rounded-md px-2 py-1.5">
              <Skeleton className="h-8 w-8 shrink-0 rounded-full" />
              <div className="min-w-0 flex-1 space-y-1">
                <Skeleton className="h-3 rounded" />
                <Skeleton className="h-2 w-16 rounded" />
              </div>
            </div>
          </div>
        </aside>
      </div>

      {/* Main column */}
      <div className="flex min-w-0 flex-1 flex-col">
        {/* TopBar skeleton */}
        <header className="flex h-14 items-center justify-between gap-4 border-b border-border bg-background px-3 sm:px-6">
          <div className="flex min-w-0 items-center gap-2">
            <Skeleton className="h-3 w-20 rounded" />
          </div>
          <div className="flex shrink-0 items-center gap-3">
            <div className="hidden md:flex">
              <Skeleton className="h-9 w-72 rounded-md" />
            </div>
            <Skeleton className="h-8 w-8 shrink-0 rounded-full" />
            <Skeleton className="h-8 w-8 shrink-0 rounded-full" />
            <Skeleton className="h-8 w-8 shrink-0 rounded-full" />
          </div>
        </header>

        {/* Main content skeleton */}
        <main className="flex-1 overflow-auto scrollbar-thin p-6">
          <Skeleton className="mb-6 h-4 w-48 rounded" />

          <div className="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={`stat-${i}`} className="rounded-md border border-border bg-surface/60 p-4">
                <Skeleton className="mb-2 h-3 w-24 rounded" />
                <Skeleton className="h-6 w-12 rounded" />
              </div>
            ))}
          </div>

          <div className="rounded-md border border-border">
            <div className="flex border-b border-border px-4 py-3">
              <Skeleton className="h-3 flex-1 rounded" />
              <Skeleton className="ml-4 h-3 w-20 rounded" />
            </div>
            {Array.from({ length: 5 }).map((_, i) => (
              <div key={`row-${i}`} className="flex border-b border-border px-4 py-3 last:border-b-0">
                <Skeleton className="h-3 flex-1 rounded" />
                <Skeleton className="ml-4 h-3 w-20 rounded" />
              </div>
            ))}
          </div>
        </main>
      </div>
    </div>
  );
}
