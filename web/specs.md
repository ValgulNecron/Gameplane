# web — Specification

**Status:** beta (v0.2.0-beta.7)  
**Module / package:** @gameplane/web  
**Build:** Vite 5.4 + React 18.3 + TypeScript 5.6 (strict)

## Purpose

The Gameplane dashboard is a React SPA providing a UI layer over the Gameplane API. It exposes Kubernetes GameServer/GameTemplate/Backup/Module operations through browser-based workflows: server creation and lifecycle, mod/player management, file browsing, backup scheduling, multi-cluster routing, RBAC configuration, and audit trails. All persistence and reconciliation logic lives in the operator and API; the dashboard is a pure UX adapter.

## Responsibilities

- Render all authenticated Gameplane CRDs and operations as React pages and interactive forms
- Handle user authentication (local + OIDC) and session management (cookies + CSRF tokens)
- Fetch and cache data via TanStack Query; invalidate caches on Kubernetes watch events (SSE)
- Stream console input/output (WebSocket) and pod/game logs (WebSocket) to the Console and Logs tabs
- Wrap the API client in three layers: thin fetch wrapper (`api<T>()`), typed endpoint namespaces (`Servers`, `Templates`, `Cluster`, etc.), and domain helpers (validations, RCON mode resolution, mod/modpack capability detection)
- Thread multi-cluster context through all API requests via `?cluster=` query params (local cluster omitted for back-compat)
- Enforce role-based access control (RBAC) on frontend routes via `<RequirePermission>` middleware
- Validate form inputs against declared server/template schema before submit
- Display real-time resource status (CPU, memory, uptime) and live action metrics from RCON/agent

## Non-goals / Boundaries

**Design-first rule:** Any change to the dashboard's visual surface (new page, new form field, layout shift, color/icon change) originates in `design.pen` (Pencil MCP server), not in React code. The Pencil file is the source of truth for all UI design; code-led redesigns are reverted. See docs/architecture.md and CLAUDE.md rule 1.

**API-only:** The dashboard never speaks directly to the operator or agent. All reads/writes flow through the Kubernetes API server and the REST/WebSocket API gateway (`api/`).

**Pre-auth privacy:** The login page and any unauthenticated screen must not leak internal state: no hostnames, cluster names, server counts, version strings, user lists, or "user not found" errors. See docs/security.md "pre-auth privacy" and CLAUDE.md rule 3.

**Local-cluster WebSocket:** Console and Logs streams currently route only to the local cluster (`?cluster=` param is not threaded through WebSocket paths). Cross-cluster WebSocket support is deferred; see `docs/roadmap.md`.

## Directory & Package Layout

```
src/
  main.tsx                  # React entry point; bootstraps QueryClient, Router, MSW
  types.ts                  # TypeScript types mirroring Gameplane CRDs (GameServer, GameTemplate, Backup, etc.)
  router/tree.tsx           # TanStack Router route tree (root, login, app-layout, all pages)
  routes/                   # Page components: Login, Dashboard, Servers, ServerDetail, CreateServer,
                            # Modules, Cluster, Users, AdminSettings, AuditLog, AdminLogs, Backups;
                            # ServerDetail sub-pages (tabs): Overview, Events, Console, Logs, Files,
                            # Mods, Modpacks, Players, Backups, Settings;
                            # Settings sub-sections: General, Version, Resources, Networking,
                            # Environment, Lifecycle, Backups (scheduled), Placement, Access (RBAC), Danger
  components/
    ui/                     # Radix + shadcn-style primitives: button, card, input, select, tabs,
                            # switch, slider, textarea, dialog, confirm-dialog, stat, etc.
    server/                 # Server-detail helpers: ServerActionsMenu, tab components
    backups/                # Backup-flow components: restore wizard, destination selector
    modules/                # Module catalog, install flow, upload preview
    AppLayout.tsx           # Main nav shell, cluster selector, user menu, breadcrumbs
    PageHeader.tsx          # Standardized page title + action buttons
    ClusterSelector.tsx     # Multi-cluster dropdown; threads ?cluster= through API calls
    RequireRole.tsx         # Permission gate middleware; wraps routes needing specific perms
    registry-browser.tsx    # Shared mod registry search/browse UI (Mods + Modpacks tabs)
  lib/
    api.ts                  # Thin fetch wrapper: api<T>(), APIError, csrfHeaders(), cluster threading
    endpoints.ts            # Typed URL-builder namespaces: Servers, Templates, Cluster(s),
                            # Backups, Schedules, Restores, BackupDestinations, Players,
                            # Users, Roles, Auth, AuthProviders, Audit, Notifications,
                            # ModRegistries, Files, Logs, Modules, ModuleSources
    ws.ts                   # openWS(): reconnecting WebSocket helper, exponential backoff
    sse.ts                  # openEventStream(): Server-Sent Events client for /events watch
    cluster.ts              # getCurrentCluster() / setCurrentCluster() for multi-cluster state
    auth.ts                 # Local user session, OIDC provider detection, logout
    capabilities.ts         # Server capability resolution: resolveConsoleMode(), serverHasMods(),
                            # serverHasModpacks() — drives tab visibility
    servers.ts              # Server validation, phase state helpers
    games.ts                # Game-specific logic: icon URLs, console protocol detection
    config.ts               # Global config: API base URL, feature flags, registry provider keys
    errors.ts               # Custom error types
    media.ts                # Image/icon URL helpers
    quantity.ts             # K8s quantity parsing (CPU, memory)
    events.ts               # Event severity / reason formatting
    annotations.ts          # K8s annotation key constants
    verify.ts               # Audit trail verification helpers (hash chain validation)
    destinations.ts         # Backup destination type detection
    validation.ts           # Form field validators (domain names, port ranges, resource specs)
    utils.ts                # capitalize(), formatUptime(), cn(), clsx helpers
  styles/
    globals.css             # Tailwind directives, CSS variables for theme, layout resets
  test/
    setup.ts                # Vitest setup: DOM matchers, MSW worker initialization
    browser-msw.ts          # MSW request handlers for mock e2e tests
    [*.test.tsx]            # Co-located unit + integration tests
e2e/
  [*.spec.ts]               # Playwright tests (mock mode and live mode)
  globalSetup.ts            # Live mode: spawn kubectl port-forward, bootstrap admin, save auth state
  globalTeardown.ts         # Live mode: kill port-forward
  .auth/storage.json        # Session + CSRF cookies persisted for live-mode tests

vite.config.ts              # Build config; dev proxy rules (Accept-header bypass for SPA routes)
tsconfig.json               # TS strict, noUnusedLocals, noUnusedParameters, noFallthroughCasesInSwitch
vitest.config.ts            # Coverage gates (lines 92 / functions 76 / branches 82 / statements 92)
playwright.config.ts        # Mock + live test modes, serial execution (login state is shared)
eslint.config.js            # Flat config: @typescript-eslint (strict), react-hooks, no-floating-promises
package.json                # @gameplane/web v0.2.0-beta.7; dev: vite, npm scripts for build/test/lint
```

## Routing & Pages

**Router:** TanStack Router v1.75 in `src/router/tree.tsx` defines a two-level structure:
- Root route → `<Outlet>`
- Login route (`/login`) → public, unauthenticated
- App layout (`/app-layout`) → contains all authenticated pages

**Top-level Pages:**

1. **Login** (`/login`) → `LoginPage`
   - Public, pre-auth, unauthenticated
   - Local username/password form + OIDC provider buttons
   - No internal metrics/hostnames (privacy rule)

2. **Dashboard** (`/`) → `DashboardPage`
   - Landing page for authenticated users
   - Server count summary, recent events, admin shortcuts

3. **Servers** (`/servers`) → `ServersPage`
   - List all GameServers in a table
   - Filter by namespace, phase (Running/Stopped/Pending/Failed), template
   - Create + clone + delete actions

4. **ServerDetail** (`/servers/$name`) → `ServerDetailPage`
   - Full server view with query param `?ns=<namespace>` support
   - Lifecycle buttons (start/stop/restart) gated on phase state
   - Tabbed interface (below)

5. **CreateServer** (`/servers/new`) → `CreateServerWizard`
   - Multi-step form (template select, version pick, name, config, storage, networking, resources)
   - Supports query param `?template=<name>` to pre-select from Modules page Deploy link
   - Validation on each step

6. **Modules** (`/modules`) → `ModulesPage`
   - Merged catalog from all registered ModuleSources + installed Module CRs
   - Browse by game, install from catalog, manage installations, bulk upload

7. **Cluster** (`/cluster`) → `ClusterPage` (gated by `servers:write` permission)
   - Cluster health, node list, kubeconfig download
   - Node join credential generation (admin-only, when clusterOps enabled)

8. **Users** (`/users`) → `UsersPage` (gated by `users:manage` permission)
   - Create/edit/delete users and OIDC links
   - Manage role bindings per namespace

9. **AdminSettings** (`/admin`) → `AdminSettingsPage` (gated by `config:manage` permission)
   - Sections: General (version, telemetry), Authentication (OIDC providers), Mod registries (API keys),
     Notification sinks (Discord/Slack/SMTP/webhook), Backup destinations

10. **AuditLog** (`/admin/audit`) → `AuditLogPage` (gated by `audit:read` permission)
    - Paginated audit event table (action, actor, resource, result, timestamp)
    - Verify audit trail hash chain, export to CSV

11. **AdminLogs** (`/admin/logs`) → `AdminLogsPage` (gated by `*` wildcard permission)
    - API and system pod logs (tail, download)
    - Diagnostic endpoint for cluster issues

12. **Backups** (`/backups`) → `BackupsPage`
    - List Backups, Schedules, Restores (three sub-tabs)
    - Manual backup trigger, schedule create/edit/suspend, restore from backup

## ServerDetail Tabs

Visible tab set depends on server template + active version:

1. **Overview** — GameServer status, phase, uptime, restart count; live metrics from RCON (if available); recent pod events
2. **Events** — Kubernetes events: image pulls, scheduling, crash-loop, agent startup
3. **Console** — Interactive RCON/WebSocket terminal (hidden if template has no console, or consoleMode=none)
4. **Logs** — Live pod stdout OR configured game log file (agent-provided via mTLS; see gameplane.local/logPath annotation)
5. **Files** — Browser and editor for server data files (config, save games, logs)
6. **Mods** — List/install/remove mods; browse by registry provider (Modrinth, CurseForge, etc.) if template declares one
7. **Modpacks** — Install modpacks (only if template + active version supports loader with modpack capability)
8. **Players** — Online player snapshot, ban list, whitelist, kick/ban/unban actions
9. **Backups** — Per-server backup list, schedule management, restore trigger
10. **Settings** — Grouped form with sub-sections (below); changes are draft-until-save; conflict detection on reload

## ServerDetail Settings Sub-sections

Settings tab (`SettingsTab`) displays 10 sections in a left sidebar:

1. **General** — Server name, description
2. **Version** — Template version selector (triggers container restart)
3. **Resources** — CPU request/limit, memory request/limit (Kubernetes resource specs)
4. **Networking** — Service type (ClusterIP/NodePort/LoadBalancer), LoadBalancer hostname, port overrides
5. **Environment** — Custom env var key=value pairs
6. **Lifecycle** — Pre/post-start/stop scripts, quiesce grace period
7. **Scheduled backups** — Backup schedule CRUD (daily/weekly/cron), retention policy
8. **Placement** — Node selector labels, pod affinity/anti-affinity rules (lazy-loaded)
9. **RBAC & access** — Server owner + collaborator list, permission inheritance
10. **Danger zone** — Clone, transfer owner, wipe data (confirm-dialog), delete server

## External Interface / API Client

**Three-layer client:**

### Layer 1: Thin Fetch Wrapper (`lib/api.ts`)

```typescript
api<T>(path: string, opts?: Options): Promise<T>
```

- Base URL: relative paths (Vite proxy in dev, same-origin in prod)
- CSRF: reads `gameplane_csrf` cookie, injects `X-Gameplane-CSRF` header on POST/PUT/PATCH
- Cluster threading: appends `?cluster=<clusterId>` when non-local cluster is selected
- Error: throws `APIError(status, body)` on !ok; TanStack Query treats it uniformly
- 204 No Content: returns `undefined as T`
- Credentials: `include` (send cookies)

**Helpers:**
- `csrfHeaders()` — returns `{ "X-Gameplane-CSRF": token }` for raw fetch (multipart, plaintext)
- `getCurrentCluster()` / `setCurrentCluster()` — global cluster context

### Layer 2: Typed Endpoint Namespaces (`lib/endpoints.ts`)

Each namespace is an object of typed functions building and fetching URLs:

- **Servers** — `list()`, `get(name, ns?)`, `create(body)`, `update(name, body, ns?)`, `remove(name, ns?)`, `lifecycle(name, verb, ns?)` (start/stop/restart), `clone(name, newName, ns?)`, `wipeData(name, confirm, ns?)`, `transfer(name, userId, ns?)`, `setCollaborators(name, ns, body)`, `getMyServers()`, `status(name, ns?)`, `events(name, ns?)`, `runAction(name, body, ns?)`, `mods(name, ns?)`, `installMod(name, body, ns?)`, `removeMod(name, mod, ns?)`, `modUpdates(name, ns?)`, `uploadMod(name, file, ns?)` (FormData), `registryProviders(name, ns?)`, `searchRegistry(name, opts?, ns?)`, `modVersions(name, project, provider?, ns?)`, `modpackDeps(name, project, provider?, ns?)`, `installModpack(name, body, provider?, ns?)`, `modIDs(name, ns?)`, `setModIDs(name, ids, ns?)`

- **Templates** — `list()`, `get(name)`

- **Cluster** — `info()`, `stats()`, `view()`, `addNode()` (POST), `kubeconfig()` (blob download)

- **Clusters** — `list()` (multi-cluster registry)

- **Backups** — `list()`, `get(name)`, `create(opts)`, `remove(name)`

- **Schedules** — `list()`, `get(name)`, `create(opts)`, `patchSpec(name, patch)` (suspend toggle), `remove(name)`

- **Restores** — `list()`, `create(opts)`, `remove(name)`

- **BackupDestinations** — `list()`, `get(name)`, `upsert(body)` (POST), `remove(name)`

- **Players** — `snapshot(server, ns?)`, `banned(server, ns?)`, `moderate(server, action, body, ns?)` (kick/ban/unban), `whitelist(server, ns?)`, `whitelistAdd(server, name, ns?)`, `whitelistRemove(server, name, ns?)`

- **Users** — `me()`, `list()`, `create(body)`, `update(id, body)`, `remove(id)`, `resetPassword(id, password)`, `bindings(id)`, `addBinding(id, body)`, `removeBinding(id, roleName, namespace)`

- **Roles** — `list()`, `catalog()` (permission groups), `create(body)`, `update(name, body)`, `remove(name)`

- **Auth** — `login(body)` (local), `logout()`, `oidcStartURL(name?)`, `providers()` (pre-auth public)

- **AuthProviders** — `putSecret(name, body)` (clientSecret), `deleteSecret(name)` (admin)

- **Audit** — `page(limit, before)` (pagination), `verify()` (hash chain), `exportCsv(filter?)` (blob)

- **Notifications** — `test(name)`, `putSecret(name, body)` (sink credentials), `deleteSecret(name)`

- **ModRegistries** — `putSecret(provider, apiKey)`, `deleteSecret(provider)`

- **Files** — `list(server, path, ns?)`, `read(server, path, ns?)`, `write(server, path, content, ns?)`, `mkdir(server, path, ns?)`, `remove(server, path, recursive?, ns?)`, `upload(server, dir, files, ns?)`, `downloadURL(server, path, ns?)`

- **Logs** — `downloadURL(server, ns?)`, `fileStreamPath(server, ns?)` (WebSocket), `podStreamPath(server, ns?)` (WebSocket)

- **Modules** — `catalog()`, `list()`, `get(name)`, `install(body)`, `upgrade(name, version)`, `uninstall(name)`

- **ModuleSources** — `list()`, `create(name, spec)`, `update(name, spec)`, `remove(name)`, `upload(source, file, opts?)` (blob), `removeUpload(source, module)`

**Helper:** `withNS(path, ns?)` appends `?namespace=<ns>` when provided; `withCluster(path)` appends `?cluster=<clusterId>` when non-local.

### Layer 3: Domain Helpers (`lib/*.ts`)

- **capabilities.ts** — `resolveConsoleMode(template)`, `serverHasMods(template, server)`, `serverHasModpacks(template, server)` — drive tab visibility
- **servers.ts** — `isServerRunning(phase)`, phase → string formatters
- **games.ts** — game icon URLs, console protocol detection (RCON/Satisfactory/Battleye)
- **auth.ts** — `getCurrentUser()`, OIDC provider list, logout flow
- **cluster.ts** — `getCurrentCluster()` / `setCurrentCluster()` (localStorage-backed)
- **validation.ts** — domain, port, K8s resource validation
- **events.ts** — event severity / reason → display strings
- **quantity.ts** — parse/format K8s quantities (500m → 0.5, 1Gi → 1073741824 bytes)

## Realtime (WebSocket & SSE)

### WebSocket (`lib/ws.ts`)

```typescript
openWS(path: string, opts: WSOptions)
  → { send(data), close() }
```

**Status states:** `connecting` | `open` | `reconnecting` | `closed` (emitted via `onStatus` callback)

**Behavior:**
- Auto-reconnect with exponential backoff: 500ms × 2^attempt, capped at 30s
- Attempt counter increments on each failed reconnection; resets to 0 on open
- `reconnect=false` option disables auto-reconnect (e.g., for intentional closes)
- Protocol detection: `wss://` on HTTPS, `ws://` on HTTP

**Used by:**
- Console tab: streams RCON/stdin input/output (bidirectional)
- Logs tab: streams container stdout and/or game log file (read-only)

**Local-cluster limitation:** WebSocket paths (`/ws/servers/{name}/logs`, `/ws/servers/{name}/logs/pod?from=start`) do not thread `?cluster=` param; multi-cluster WebSocket support is deferred.

### Server-Sent Events (`lib/sse.ts`)

```typescript
openEventStream(opts: EventStreamOptions)
  → () => void  // disposer
```

**Behavior:**
- Connects to `/events` (EventSource, auto-reconnect on transient close)
- Each frame is a Kubernetes watch event: `{ kind, eventType, object }`
- Frames are parsed as JSON; malformed frames are silently dropped
- Manual reconnect on onerror (after 3s backoff) if the browser closed the stream
- No-op fallback if EventSource is undefined (jsdom, ancient browser)

**Used by:**
- Global event listener in `main.tsx` or a provider component
- Invalidates TanStack Query caches on MODIFIED/DELETED (watches servers, templates, backups, schedules, restores)
- Powers notifications panel (shows recent activity)

**Note:** SSE does not thread cluster context (local only, for now).

## Key Invariants

1. **TypeScript strict mode** — `tsconfig.json` sets `strict: true`, `noUnusedLocals`, `noUnusedParameters`, `noFallthroughCasesInSwitch`
2. **No unjustified `any`** — `@typescript-eslint/no-explicit-any: error`; any real `any` must be a comment explaining why
3. **No floating promises** — `@typescript-eslint/no-floating-promises: error`; either `await` or `void` prefix every Promise
4. **Design-first for visuals** — all UI/layout changes start in `design.pen`, not React code
5. **Pre-auth privacy** — login page and unauthenticated screens leak no internal state (rule 3, CLAUDE.md)
6. **Fix, not silence** — ESLint / TypeScript flags are fixed at source, never suppressed inline (rule 4, CLAUDE.md)
7. **Operator is authoritative** — business logic lives in the operator; the dashboard is a pure view layer

## Dependencies

**Runtime:**
- `react@18.3.1` — React library
- `react-dom@18.3.1` — DOM renderer
- `@tanstack/react-router@1.75.0` — file-based routing (via tree.tsx)
- `@tanstack/react-query@5.59.0` — data fetching, caching, invalidation
- `@tanstack/react-virtual@3.10.8` — virtualized lists for large tables
- `@radix-ui/*` — dialog, dropdown-menu, label, slot, tabs, toast (headless, unstyled)
- `clsx@2.1.1` — conditional classNames
- `tailwind-merge@2.5.2` — Tailwind class conflict resolution
- `class-variance-authority@0.7.0` — component variant system
- `tailwindcss@3.4.13` — utility-first CSS framework
- `lucide-react@0.445.0` — SVG icon library
- `@monaco-editor/react@4.6.0` — code editor (lazy-loaded, file/config edit tabs)
- `@xterm/xterm@5.5.0` — terminal emulator (Console tab)
- `@xterm/addon-fit@0.10.0` — xterm fit-to-container addon

**Dev:**
- `typescript@5.6.2` — strict type checking
- `vite@5.4.8` — build + dev server
- `@vitejs/plugin-react@4.3.1` — React JSX transform + fast refresh
- `vitest@2.1.1` — unit test runner (Jest-like API, Vite-integrated)
- `@vitest/coverage-v8@2.1.1` — V8 coverage reporter
- `@testing-library/react@16.0.1` — component test utilities
- `@testing-library/jest-dom@6.5.0` — DOM matchers
- `@testing-library/user-event@14.6.1` — user interaction simulation
- `msw@2.14.4` — Mock Service Worker for request interception (e2e mock mode)
- `vitest-websocket-mock@0.4.0` — WebSocket mock for unit tests
- `@playwright/test@1.59.1` — browser e2e testing (mock + live modes)
- `jsdom@25.0.1` — DOM implementation for unit tests
- `eslint@9.11.1` — JavaScript linter
- `@typescript-eslint/eslint-plugin@8.7.0` — TypeScript linting rules
- `@typescript-eslint/parser@8.7.0` — TypeScript AST parser for ESLint
- `eslint-plugin-react@7.37.0` — React-specific rules
- `eslint-plugin-react-hooks@5.0.0` — React Hooks rules
- `tailwindcss@3.4.13` + `postcss@8.4.47` + `autoprefixer@10.4.20` — CSS processing
- `@tanstack/router-devtools@1.75.0` — TanStack Router dev tools (optional, for debugging)

## Security considerations

- **Pre-auth privacy:** The login page and unauthenticated screens leak no internal state: no hostnames, cluster names, server counts, version strings, or user-enumeration signals. Errors are neutral ("invalid credentials" only) — see CLAUDE.md rule 3 and docs/security.md.
- **CSRF protection:** Mutating requests carry a double-submit token; `gameplane_csrf` cookie is read and echoed as the `X-Gameplane-CSRF` header by `lib/api.ts` on POST/PUT/PATCH.
- **XSS surface:** User-controlled content rendered in the Monaco editor, xterm console, and log/event views is treated as untrusted. React's automatic escaping and avoiding `dangerouslySetInnerHTML` are the primary defenses.
- **No secrets in the bundle:** The SPA holds no API keys, credentials, or tokens; all privileged actions flow through the authenticated API. Session auth is cookie-based (credentials: include).
- **Session handling:** Auth is cookie-based (credentials: include); the dashboard never stores tokens in localStorage and relies on secure, HttpOnly cookies set by the API.

## Testing & Coverage

**Framework:** Vitest 2.1 + Testing Library (React) + jsdom

**Test files:** Co-located with source (`src/**/*.test.tsx`, `src/**/*.test.ts`)

**Mock server:** MSW 2 in mock e2e mode (Playwright); real API in live e2e mode

**Coverage thresholds** (`vitest.config.ts`):
- Lines: 92%
- Functions: 76%
- Branches: 82%
- Statements: 92%

**Exclusions from coverage:**
- `src/main.tsx` — bootstrapping only
- `src/router/**` — route tree is configuration, not logic
- `src/**/*.d.ts` — type definitions
- `src/types.ts` — type mirrors only
- `src/test/**` — test infrastructure
- `src/styles/**` — CSS
- `src/lib/config.ts` — config-only, no logic

**E2E testing:** Playwright in two modes:

1. **Mock mode** (`GAMEPLANE_E2E_TARGET=mock`, npm run test:e2e:mock)
   - Vite runs with `--mode mock`, loading `.env.mock` (sets `VITE_E2E_MOCK=true`)
   - Dashboard dynamically imports MSW browser worker at bootstrap
   - MSW intercepts every fetch; no cluster needed
   - Fast, deterministic, no side effects

2. **Live mode** (`GAMEPLANE_E2E_TARGET=live`, npm run test:e2e:live)
   - Tests run against a real Kubernetes cluster (gameplane-e2e, via kubectl port-forward)
   - globalSetup spawns port-forward, logs in as admin, saves session cookies
   - Tests inherit auth state from `.auth/storage.json`
   - Slower, flaky, full integration

**Execution:** Serial (workers: 1) because login state is shared across tests; retries: 0 local, 1 in CI.

## References

- **docs/architecture.md** — component overview, data flow, security boundaries, "operator is authoritative" rationale
- **docs/security.md** — auth model, RBAC, threat model, pod security, pre-auth privacy rule
- **docs/installing.md** — Helm values, K8s prerequisites, OIDC setup (for deployment contexts)
- **CLAUDE.md rule 1** — Design-first: visual changes originate in design.pen, not code
- **CLAUDE.md rule 3** — Pre-auth privacy: login page must not leak internal metrics/hostnames/versions
- **CLAUDE.md rule 4** — Fix, don't silence: linter/type flags are fixed at source
- **CLAUDE.md rule 5** — TS strict; no unjustified `any`; no floating promises
- **design.pen** — Source of truth for all UI/layout (Pencil MCP server; do not edit as text)
- **README.md** — Project pitch, quickstart, architecture overview
