import {
  RootRoute, Route, Outlet,
} from "@tanstack/react-router";
import { AppLayout } from "@/components/AppLayout";
import { RequirePermission } from "@/components/RequireRole";
import { LoginPage } from "@/routes/Login";
import { DashboardPage } from "@/routes/Dashboard";
import { ServersPage } from "@/routes/Servers";
import { ServerDetailPage } from "@/routes/ServerDetail";
import { ModulesPage } from "@/routes/Modules";
import { ClusterPage } from "@/routes/Cluster";
import { UsersPage } from "@/routes/Users";
import { AdminSettingsPage } from "@/routes/AdminSettings";
import { CreateServerWizard } from "@/routes/CreateServer";
import { BackupsPage } from "@/routes/Backups";
import { AuditLogPage } from "@/routes/AuditLog";
import { AdminLogsPage } from "@/routes/AdminLogs";

const rootRoute = new RootRoute({ component: Outlet });

const loginRoute = new Route({
  getParentRoute: () => rootRoute,
  path: "/login",
  component: LoginPage,
});

const appLayoutRoute = new Route({
  getParentRoute: () => rootRoute,
  id: "app-layout",
  component: AppLayout,
});

const dashboardRoute = new Route({
  getParentRoute: () => appLayoutRoute,
  path: "/",
  component: DashboardPage,
});

const serversRoute = new Route({
  getParentRoute: () => appLayoutRoute,
  path: "/servers",
  component: ServersPage,
});

const serverDetailRoute = new Route({
  getParentRoute: () => appLayoutRoute,
  path: "/servers/$name",
  component: ServerDetailPage,
  validateSearch: (search: Record<string, unknown>): { ns?: string } => ({
    ns: typeof search.ns === "string" && search.ns !== "" ? search.ns : undefined,
  }),
});

const createServerRoute = new Route({
  getParentRoute: () => appLayoutRoute,
  path: "/servers/new",
  component: CreateServerWizard,
  // Lets the Modules catalog "Deploy" link pre-select a template via
  // /servers/new?template=<name>.
  validateSearch: (search: Record<string, unknown>): { template?: string } => ({
    template: typeof search.template === "string" ? search.template : undefined,
  }),
});

const modulesRoute = new Route({
  getParentRoute: () => appLayoutRoute,
  path: "/modules",
  component: ModulesPage,
});

const clusterRoute = new Route({
  getParentRoute: () => appLayoutRoute,
  path: "/cluster",
  component: () => (
    <RequirePermission perm="servers:write">
      <ClusterPage />
    </RequirePermission>
  ),
});

const usersRoute = new Route({
  getParentRoute: () => appLayoutRoute,
  path: "/users",
  component: () => (
    <RequirePermission perm="users:manage">
      <UsersPage />
    </RequirePermission>
  ),
});

const adminRoute = new Route({
  getParentRoute: () => appLayoutRoute,
  path: "/admin",
  component: () => (
    <RequirePermission perm="config:manage">
      <AdminSettingsPage />
    </RequirePermission>
  ),
});

const auditLogRoute = new Route({
  getParentRoute: () => appLayoutRoute,
  path: "/admin/audit",
  component: () => (
    <RequirePermission perm="audit:read">
      <AuditLogPage />
    </RequirePermission>
  ),
});

// The API guards /admin/system-logs with the admin wildcard permission
// ("*"), so the page gates on the same — not a narrower named permission.
const adminLogsRoute = new Route({
  getParentRoute: () => appLayoutRoute,
  path: "/admin/logs",
  component: () => (
    <RequirePermission perm="*">
      <AdminLogsPage />
    </RequirePermission>
  ),
});

const backupsRoute = new Route({
  getParentRoute: () => appLayoutRoute,
  path: "/backups",
  component: BackupsPage,
});

export const routeTree = rootRoute.addChildren([
  loginRoute,
  appLayoutRoute.addChildren([
    dashboardRoute,
    createServerRoute,
    serversRoute,
    serverDetailRoute,
    modulesRoute,
    clusterRoute,
    usersRoute,
    adminRoute,
    auditLogRoute,
    adminLogsRoute,
    backupsRoute,
  ]),
]);
