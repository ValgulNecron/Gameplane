import { Outlet, useLocation } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import {
  Archive,
  LayoutDashboard,
  Package,
  ScrollText,
  Server,
  Settings,
  Terminal,
  Users,
} from "lucide-react";
import { APIError } from "@/lib/api";
import { Cluster as ClusterAPI, Auth } from "@/lib/endpoints";
import { useMe, can } from "@/lib/auth";
import type { ClusterInfo } from "@/types";
import { useEffect, useState } from "react";
import { ClusterSelector } from "@/components/ClusterSelector";
import { AppShell } from "@/components/ui/AppShell";
import { Sidebar, type SidebarNavGroup } from "@/components/ui/Sidebar";
import { TopBar } from "@/components/ui/TopBar";
import { Breadcrumbs, buildCrumbs } from "@/components/ui/Breadcrumbs";
import { GlobalSearch } from "@/components/ui/GlobalSearch";
import { NotificationsPanel } from "@/components/ui/NotificationsPanel";
import { AppLoadingSkeleton } from "@/components/ui/AppLoadingSkeleton";
import type { AppearanceMode } from "@/components/ui/AppearanceToggle";
import { useDelayedLoading } from "@/lib/useDelayedLoading";

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

    // Subscribe to OS theme changes only in system mode
    if (theme === "system") {
      const mq = window.matchMedia("(prefers-color-scheme: dark)");
      const handleChange = () => applyTheme("system");
      mq.addEventListener("change", handleChange);
      return () => mq.removeEventListener("change", handleChange);
    }
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
  const showSkeleton = useDelayedLoading(isLoading);

  useEffect(() => {
    if (error instanceof APIError && error.status === 401) {
      location.assign("/login");
    }
  }, [error]);

  if (showSkeleton) return <AppLoadingSkeleton />;

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
  // Extract the last breadcrumb label as the mobile title
  const mobileTitle = crumbs.length > 0 ? crumbs[crumbs.length - 1].label : "";

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
            mobileTitle={mobileTitle}
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
