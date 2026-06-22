import type { APIRequestContext, APIResponse, Page } from "@playwright/test";
import { LoginPage } from "../../pages/LoginPage";

// Shared helpers for the live-mode data-screen specs. These talk to the
// REAL Kestrel API (through vite's proxy onto the kubectl port-forward
// globalSetup spawns) using the admin session cookies in storageState.
//
// Why seed through the API instead of reusing the Go e2e suite's fixtures:
// the Go suite registers a t.Cleanup for everything it creates, so by the
// time the Playwright job runs those CRs are already gone. Each live spec
// therefore creates the real objects it asserts on and deletes them again,
// which keeps it deterministic and independent of Go-suite test ordering.

// loginIfNeeded handles both run modes:
//   - mock: /users/me always returns a user, so visiting / stays in-app.
//   - live: /users/me returns 401, AppLayout redirects to /login, we sign in.
// (Kept here so the live specs don't each re-declare it.)
export async function loginIfNeeded(page: Page): Promise<void> {
  await page.goto("/");
  await page.waitForLoadState("domcontentloaded");
  if (new URL(page.url()).pathname.startsWith("/login")) {
    const login = new LoginPage(page);
    const username =
      process.env.ADMIN_USERNAME ?? process.env.KESTREL_E2E_ADMIN_USERNAME ?? "e2e-admin";
    const password =
      process.env.ADMIN_PASSWORD ?? process.env.KESTREL_E2E_ADMIN_PASSWORD ?? "any-non-empty";
    await login.login(username, password);
    await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 10_000 });
  }
}

// seedHeaders builds the headers a seed mutation needs:
//
//   - X-Gameplane-CSRF — the API enforces double-submit CSRF on every mutating
//     request (the header must equal the gameplane_csrf cookie; api/internal/
//     auth/sessions.go). The browser app echoes it automatically (lib/api.ts);
//     an APIRequestContext does not, so we read the token out of storageState
//     and set it ourselves. Without this every seed POST/DELETE would 403.
//   - Accept: application/json — vite's dev proxy serves the SPA shell for
//     text/html navigations on collision paths like /servers, and forwards
//     everything else to the API. Pinning Accept keeps the seed request on
//     the API side of that bypass regardless of Playwright's default.
async function seedHeaders(request: APIRequestContext): Promise<Record<string, string>> {
  const state = await request.storageState();
  const token = state.cookies.find((c) => c.name === "gameplane_csrf")?.value ?? "";
  return { "X-Gameplane-CSRF": token, Accept: "application/json" };
}

// expectSeedOk throws (failing the spec) unless the seed request succeeded.
// 409 Conflict is tolerated: a previous run that died before cleanup may
// have leaked the object, and an existing object is just as usable here.
async function expectSeedOk(res: APIResponse, what: string): Promise<void> {
  if (!res.ok() && res.status() !== 409) {
    throw new Error(`${what} failed: ${res.status()} ${await res.text().catch(() => "")}`);
  }
}

export interface Seeded {
  name: string;
  // Best-effort teardown — safe to call even if the object is already gone.
  // Takes a live request context: Playwright disposes the beforeAll request
  // before afterAll runs, so the cleanup must use afterAll's own `request`
  // rather than closing over the (now-closed) one used to seed.
  cleanup: (request: APIRequestContext) => Promise<void>;
}

// seedTemplate creates a minimal, schema-valid busybox GameTemplate
// (mirrors test/e2e's applyBusyboxTemplate). `sleep` keeps the pod alive so
// agent-backed screens have a live sidecar to read from. Cluster-scoped, so
// no namespace is needed.
export async function seedTemplate(request: APIRequestContext, name: string): Promise<Seeded> {
  const res = await request.post("/templates", {
    headers: await seedHeaders(request),
    data: {
      apiVersion: "gameplane.gg/v1alpha1",
      kind: "GameTemplate",
      metadata: { name },
      spec: {
        displayName: `E2E live ${name}`,
        game: "busybox",
        version: "1",
        image: "busybox:1.36",
        command: ["sh", "-c", "sleep 100000"],
        ports: [{ name: "noop", containerPort: 12345, advertise: true, protocol: "TCP" }],
      },
    },
  });
  await expectSeedOk(res, `seed template ${name}`);
  return {
    name,
    cleanup: async (req: APIRequestContext) => {
      await req
        .delete(`/templates/${name}`, { headers: await seedHeaders(req) })
        .catch(() => undefined);
    },
  };
}

export interface SeedServerOpts {
  name: string;
  template: string;
  description?: string;
}

// seedServer creates a namespaced GameServer referencing `template`. The
// API defaults the namespace to scope.DefaultNamespace ("kestrel-games") —
// the same default the dashboard reads — so seeded and rendered data line up
// without passing ?namespace=. The description annotation mirrors what the
// create-server wizard writes (gameplane.gg/description).
export async function seedServer(request: APIRequestContext, opts: SeedServerOpts): Promise<Seeded> {
  const res = await request.post("/servers", {
    headers: await seedHeaders(request),
    data: {
      apiVersion: "gameplane.gg/v1alpha1",
      kind: "GameServer",
      metadata: {
        name: opts.name,
        ...(opts.description
          ? { annotations: { "gameplane.gg/description": opts.description } }
          : {}),
      },
      spec: { templateRef: { name: opts.template } },
    },
  });
  await expectSeedOk(res, `seed server ${opts.name}`);
  return {
    name: opts.name,
    cleanup: async (req: APIRequestContext) => {
      await req
        .delete(`/servers/${opts.name}`, { headers: await seedHeaders(req) })
        .catch(() => undefined);
    },
  };
}
