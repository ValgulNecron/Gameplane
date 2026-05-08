// Default MSW handlers — the baseline API surface a route test can rely
// on to render its happy-path UI without each test having to declare
// every endpoint. Tests override individual routes via `server.use(...)`.

import { http, HttpResponse } from "msw";
import {
  makeAudit,
  makeBackup,
  makeCatalog,
  makeClusterStats,
  makeClusterView,
  makeDestination,
  makeModuleSource,
  makePlayers,
  makeSchedule,
  makeServer,
  makeTemplate,
  makeUser,
} from "./factories";

export const handlers = [
  // Auth
  http.get("/users/me", () => HttpResponse.json(makeUser())),

  // Cluster
  http.get("/cluster", () => HttpResponse.json(makeClusterView())),
  http.get("/cluster/info", () =>
    HttpResponse.json({ clusterName: "homelab", version: "v1.31.0" }),
  ),
  http.get("/cluster/stats", () => HttpResponse.json(makeClusterStats())),

  // Servers
  http.get("/servers", () => HttpResponse.json({ items: [makeServer()] })),
  http.get("/servers/:name", ({ params }) =>
    HttpResponse.json(makeServer({ metadata: { name: String(params.name) } })),
  ),

  // Templates
  http.get("/templates", () => HttpResponse.json({ items: [makeTemplate()] })),

  // Backups
  http.get("/backups", () => HttpResponse.json({ items: [makeBackup()] })),
  http.get("/schedules", () => HttpResponse.json({ items: [makeSchedule()] })),
  http.get("/restores", () => HttpResponse.json({ items: [] })),
  http.get("/backup-destinations", () =>
    HttpResponse.json({ items: [makeDestination()] }),
  ),

  // Modules
  http.get("/modules", () => HttpResponse.json({ items: [makeCatalog()] })),
  http.get("/module-sources", () => HttpResponse.json({ items: [makeModuleSource()] })),

  // Players
  http.get("/servers/:name/players", () => HttpResponse.json(makePlayers())),
  http.get("/servers/:name/players/banned", () => HttpResponse.json([])),

  // Admin
  http.get("/admin/audit", () => HttpResponse.json([makeAudit()])),
  http.get("/admin/config", () => HttpResponse.json({})),
  http.get("/admin/users", () => HttpResponse.json({ items: [makeUser()] })),
];
