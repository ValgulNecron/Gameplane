// Default MSW handlers — the baseline API surface a route test can rely
// on to render its happy-path UI without each test having to declare
// every endpoint. Tests override individual routes via `server.use(...)`.

import { http, HttpResponse } from "msw";
import {
  makeAudit,
  makeBackup,
  makeBannedPlayer,
  makeCatalog,
  makeClusterStats,
  makeClusterView,
  makeConfig,
  makeDestination,
  makeFileEntry,
  makeModule,
  makeModuleSource,
  makePlayers,
  makeRestore,
  makeSchedule,
  makeServer,
  makeTemplate,
  makeUser,
} from "./factories";

export const handlers = [
  // Auth
  http.get("/users/me", ({ cookies }) => {
    // e2e affordance: a 401 on /users/me must bounce the SPA to /login,
    // but the browser-mode service worker answers before Playwright's
    // page.route can inject a status. A cookie the test sets (and which
    // survives the navigation) lets it force the unauthorized path
    // deterministically. No-op for the unit suite (no cookie set).
    if (cookies.e2e_force_401 === "1") {
      return new HttpResponse("unauthorized\n", { status: 401 });
    }
    return HttpResponse.json(makeUser());
  }),
  // Mock-mode Playwright login: accept any non-empty credentials and set
  // the CSRF cookie the SPA's mutation path reads on every later POST.
  // Real backend rate-limiting and argon2id verification are out of
  // scope for browser-side mock — those are exercised in api_auth_e2e.
  http.post("/auth/login", async ({ request }) => {
    const body = (await request.json().catch(() => null)) as {
      username?: string;
      password?: string;
    } | null;
    if (!body?.username || !body?.password) {
      return new HttpResponse("invalid credentials\n", { status: 401 });
    }
    return HttpResponse.json(
      { user: makeUser({ username: body.username }), csrf: "mock-csrf-token" },
      {
        headers: {
          // Mirror the real cookie shape: csrf cookie is JS-readable so
          // the SPA can echo it back in X-Kestrel-CSRF.
          "Set-Cookie":
            "kestrel_csrf=mock-csrf-token; Path=/; SameSite=Lax",
        },
      },
    );
  }),
  http.post("/auth/logout", () => new HttpResponse(null, { status: 204 })),
  // Pre-auth login providers. Mock mode advertises both local + OIDC so
  // the full login UI is exercised; real installs gate OIDC on config.
  http.get("/auth/providers", () =>
    HttpResponse.json({
      providers: [
        { kind: "local", label: "Local account" },
        { kind: "oidc", label: "OIDC" },
      ],
    }),
  ),

  // Cluster
  http.get("/cluster", () => HttpResponse.json(makeClusterView())),
  http.get("/cluster/info", () =>
    HttpResponse.json({ clusterName: "homelab", version: "v1.31.0" }),
  ),
  http.get("/cluster/stats", () => HttpResponse.json(makeClusterStats())),

  // Servers
  http.get("/servers", () =>
    HttpResponse.json({
      items: [
        makeServer(),
        makeServer({ metadata: { name: "beta", namespace: "kestrel-games" } }),
      ],
    }),
  ),
  http.get("/servers/:name", ({ params }) =>
    HttpResponse.json(makeServer({ metadata: { name: String(params.name) } })),
  ),
  http.post("/servers", async ({ request }) => {
    const body = (await request.json().catch(() => null)) as {
      metadata?: { name?: string };
    } | null;
    return HttpResponse.json(
      makeServer({
        metadata: { name: body?.metadata?.name ?? "new-server", namespace: "kestrel-games" },
      }),
    );
  }),
  http.put("/servers/:name", ({ params }) =>
    HttpResponse.json(makeServer({ metadata: { name: String(params.name) } })),
  ),
  http.delete("/servers/:name", () => new HttpResponse(null, { status: 204 })),

  // Lifecycle: chi uses `:verb` literal-colon URL syntax, which standard
  // URL pattern matchers don't parse — fall back to regex per verb.
  http.post(/\/servers\/[^/]+:start$/, () => new HttpResponse(null, { status: 202 })),
  http.post(/\/servers\/[^/]+:stop$/, () => new HttpResponse(null, { status: 202 })),
  http.post(/\/servers\/[^/]+:restart$/, () => new HttpResponse(null, { status: 202 })),
  http.post(/\/servers\/[^/]+:clone$/, async ({ request }) => {
    const body = (await request.json().catch(() => null)) as {
      newName?: string;
    } | null;
    return HttpResponse.json(
      makeServer({
        metadata: { name: body?.newName ?? "cloned-server", namespace: "kestrel-games" },
      }),
    );
  }),

  // Templates
  http.get("/templates", () =>
    HttpResponse.json({
      items: [
        makeTemplate(),
        makeTemplate({
          metadata: { name: "valheim-default" },
          spec: {
            displayName: "Valheim",
            game: "valheim",
            version: "0.218",
            image: "ghcr.io/kestrel/valheim:0.218",
          },
        }),
      ],
    }),
  ),
  http.get("/templates/:name", ({ params }) =>
    HttpResponse.json(makeTemplate({ metadata: { name: String(params.name) } })),
  ),

  // Backups
  http.get("/backups", () =>
    HttpResponse.json({
      items: [
        makeBackup(),
        makeBackup({
          metadata: { name: "alpha-2026-05-06", namespace: "kestrel-games" },
          status: {
            phase: "Failed",
            startTime: "2026-05-06T03:00:00Z",
            completionTime: "2026-05-06T03:00:30Z",
          },
        }),
      ],
    }),
  ),
  http.get("/backups/:name", ({ params }) =>
    HttpResponse.json(makeBackup({ metadata: { name: String(params.name) } })),
  ),
  http.post("/backups", async ({ request }) => {
    const body = (await request.json().catch(() => null)) as {
      metadata?: { name?: string; generateName?: string };
      spec?: { serverRef?: { name?: string } };
    } | null;
    const name =
      body?.metadata?.name ??
      `${body?.spec?.serverRef?.name ?? "alpha"}-manual-${Date.now()}`;
    return HttpResponse.json(makeBackup({ metadata: { name, namespace: "kestrel-games" } }));
  }),
  http.delete("/backups/:name", () => new HttpResponse(null, { status: 204 })),

  http.get("/schedules", () => HttpResponse.json({ items: [makeSchedule()] })),
  http.get("/schedules/:name", ({ params }) =>
    HttpResponse.json(makeSchedule({ metadata: { name: String(params.name) } })),
  ),
  http.post("/schedules", async ({ request }) => {
    const body = (await request.json().catch(() => null)) as {
      metadata?: { name?: string; generateName?: string };
      spec?: { serverRef?: { name?: string }; schedule?: string };
    } | null;
    return HttpResponse.json(
      makeSchedule({
        metadata: {
          name: body?.metadata?.name ?? `${body?.spec?.serverRef?.name ?? "alpha"}-sched-1`,
          namespace: "kestrel-games",
        },
      }),
    );
  }),
  http.put("/schedules/:name", ({ params }) =>
    HttpResponse.json(makeSchedule({ metadata: { name: String(params.name) } })),
  ),
  http.delete("/schedules/:name", () => new HttpResponse(null, { status: 204 })),

  // Restores
  http.get("/restores", () => HttpResponse.json({ items: [] })),
  http.post("/restores", async ({ request }) => {
    const body = (await request.json().catch(() => null)) as {
      metadata?: { name?: string; generateName?: string };
      spec?: { backupRef?: { name?: string }; serverRef?: { name?: string } };
    } | null;
    return HttpResponse.json(
      makeRestore({
        metadata: {
          name: body?.metadata?.name ?? `restore-${Date.now()}`,
          namespace: "kestrel-games",
        },
        spec: {
          backupRef: { name: body?.spec?.backupRef?.name ?? "alpha-2026-05-07" },
          serverRef: { name: body?.spec?.serverRef?.name ?? "alpha" },
        },
      }),
    );
  }),
  http.delete("/restores/:name", () => new HttpResponse(null, { status: 204 })),

  // Backup destinations
  http.get("/backup-destinations", () =>
    HttpResponse.json({ items: [makeDestination()] }),
  ),
  http.post("/backup-destinations", async ({ request }) => {
    const body = (await request.json().catch(() => null)) as {
      name?: string;
      url?: string;
    } | null;
    return HttpResponse.json(
      makeDestination({ name: body?.name ?? "default", url: body?.url ?? "s3://x" }),
    );
  }),
  http.delete("/backup-destinations/:name", () => new HttpResponse(null, { status: 204 })),

  // Modules: legacy /modules handler is preserved (some existing tests
  // hit it); the dashboard's actual paths are /modules/catalog and
  // /modules/sources, served below.
  http.get("/modules", () => HttpResponse.json({ items: [makeModule()] })),
  http.get("/modules/catalog", () =>
    HttpResponse.json({
      items: [
        makeCatalog(),
        makeCatalog({
          name: "valheim-default",
          displayName: "Valheim",
          game: "valheim",
          installed: true,
          installedVersion: "0.218",
          moduleName: "valheim-default",
          phase: "Ready",
          versions: ["0.218", "0.217"],
          latestVersion: "0.218",
        }),
      ],
    }),
  ),
  http.get("/modules/sources", () =>
    HttpResponse.json({ items: [makeModuleSource()] }),
  ),
  http.get("/modules/:name", ({ params }) =>
    HttpResponse.json(makeModule({ metadata: { name: String(params.name) } })),
  ),
  http.post("/modules", async ({ request }) => {
    const body = (await request.json().catch(() => null)) as {
      module?: string;
      name?: string;
    } | null;
    return HttpResponse.json(
      makeModule({ metadata: { name: body?.name ?? body?.module ?? "minecraft-vanilla" } }),
    );
  }),
  http.patch("/modules/:name", ({ params }) =>
    HttpResponse.json(makeModule({ metadata: { name: String(params.name) } })),
  ),
  http.delete("/modules/:name", () => new HttpResponse(null, { status: 204 })),

  // Legacy path retained for backwards compatibility with earlier tests.
  http.get("/module-sources", () =>
    HttpResponse.json({ items: [makeModuleSource()] }),
  ),

  // Players
  http.get("/servers/:name/players", () => HttpResponse.json(makePlayers())),
  http.get("/servers/:name/players/banned", () =>
    HttpResponse.json([makeBannedPlayer()]),
  ),
  http.post(/\/servers\/[^/]+\/players\/(kick|ban|unban)$/, () =>
    HttpResponse.json({ ok: true }),
  ),

  // Files (agent proxy)
  http.get("/servers/:name/files/list", () =>
    HttpResponse.json([
      makeFileEntry({ name: "server.properties", path: "/data/server.properties", size: 412 }),
      makeFileEntry({ name: "world", path: "/data/world", size: 0, dir: true }),
    ]),
  ),
  http.get("/servers/:name/files/read", () => new HttpResponse("# mock file body\n", { status: 200 })),
  http.post("/servers/:name/files/write", () => new HttpResponse(null, { status: 204 })),
  http.post("/servers/:name/files/mkdir", () => new HttpResponse(null, { status: 204 })),
  http.delete("/servers/:name/files/delete", () => new HttpResponse(null, { status: 204 })),

  // Users (canonical path per endpoints.ts; /admin/users below is legacy).
  http.get("/users", () =>
    HttpResponse.json([
      makeUser(),
      makeUser({ id: 2, username: "operator-bob", role: "operator" }),
      makeUser({ id: 3, username: "viewer-carol", role: "viewer" }),
    ]),
  ),
  http.post("/users", async ({ request }) => {
    const body = (await request.json().catch(() => null)) as {
      username?: string;
      role?: "admin" | "operator" | "viewer";
    } | null;
    return HttpResponse.json(
      makeUser({
        id: 99,
        username: body?.username ?? "new-user",
        role: body?.role ?? "viewer",
      }),
    );
  }),
  http.patch("/users/:id", ({ params }) =>
    HttpResponse.json(makeUser({ id: Number(params.id) })),
  ),
  http.delete("/users/:id", () => new HttpResponse(null, { status: 204 })),
  http.post("/users/:id/reset-password", () => new HttpResponse(null, { status: 204 })),

  // Roles + permission catalog (custom RBAC).
  http.get("/roles", () =>
    HttpResponse.json([
      { name: "admin", description: "Full access.", builtin: true, permissions: ["*"] },
      {
        name: "operator",
        description: "Manage servers, backups, templates.",
        builtin: true,
        permissions: ["servers:read", "servers:write"],
      },
      { name: "viewer", description: "Read-only.", builtin: true, permissions: ["servers:read"] },
    ]),
  ),
  http.get("/roles/permissions", () =>
    HttpResponse.json({
      groups: [
        {
          resource: "servers",
          label: "Game servers",
          permissions: [
            { key: "servers:read", label: "View servers", namespaced: true },
            { key: "servers:write", label: "Manage servers", namespaced: true },
          ],
        },
        {
          resource: "users",
          label: "Users",
          permissions: [{ key: "users:manage", label: "Manage users", namespaced: false }],
        },
      ],
    }),
  ),
  http.post("/roles", async ({ request }) => {
    const body = (await request.json().catch(() => null)) as {
      name?: string;
      description?: string;
      permissions?: string[];
    } | null;
    return HttpResponse.json(
      {
        name: body?.name ?? "new-role",
        description: body?.description ?? "",
        builtin: false,
        permissions: body?.permissions ?? [],
      },
      { status: 201 },
    );
  }),
  http.patch("/roles/:name", async ({ params, request }) => {
    const body = (await request.json().catch(() => null)) as {
      description?: string;
      permissions?: string[];
    } | null;
    return HttpResponse.json({
      name: String(params.name),
      description: body?.description ?? "",
      builtin: false,
      permissions: body?.permissions ?? [],
    });
  }),
  http.delete("/roles/:name", () => new HttpResponse(null, { status: 204 })),
  http.get("/users/:id/bindings", () => HttpResponse.json([])),
  http.post("/users/:id/bindings", async ({ request }) => {
    const body = (await request.json().catch(() => null)) as {
      roleName?: string;
      namespace?: string;
    } | null;
    return HttpResponse.json(
      { roleName: body?.roleName ?? "viewer", namespace: body?.namespace ?? "team-a" },
      { status: 201 },
    );
  }),
  http.delete("/users/:id/bindings/:role/:namespace", () => new HttpResponse(null, { status: 204 })),

  // Admin
  http.get("/admin/audit", ({ request }) => {
    const url = new URL(request.url);
    const limit = Number(url.searchParams.get("limit") ?? "50");
    const out: ReturnType<typeof makeAudit>[] = [];
    for (let i = 0; i < Math.min(limit, 5); i++) {
      out.push(
        makeAudit({
          id: i + 1,
          method: i % 2 === 0 ? "POST" : "GET",
          path: `/api/v1/servers/alpha`,
          status: i === 4 ? 403 : 200,
        }),
      );
    }
    return HttpResponse.json(out);
  }),
  http.get("/admin/config", () => HttpResponse.json(makeConfig())),
  http.put("/admin/config/:section", () => new HttpResponse(null, { status: 204 })),

  // Legacy admin/users path retained in case any existing test still
  // points at it; canonical /users handler above takes precedence for
  // paths the dashboard actually fetches.
  http.get("/admin/users", () => HttpResponse.json({ items: [makeUser()] })),
];
