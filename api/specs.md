# api — Specification

**Status:** beta (v0.2.0-beta.7)  
**Module / package:** github.com/ValgulNecron/gameplane/api

## Purpose

The api module is the REST + WebSocket gateway for Gameplane, serving the web dashboard and external integrations. It exposes the Kubernetes game-server CRDs (GameServer, GameTemplate, Backup, BackupSchedule, Restore, Module, ModuleSource, Cluster) and operator-reconciled state through a type-safe HTTP surface with multi-cluster support, role-based access control, and audit logging.

## Responsibilities

- **REST gateway:** expose CRUD operations on game resources (servers, templates, backups, schedules, destinations, clusters) and administrative surfaces (users, roles, config, audit, notifications, mod registries)
- **WebSocket bridge:** streaming console (RCON/PTY), pod logs, system logs, and real-time events (SSE) — multiplexed per namespace and cluster
- **Authentication:** local argon2id + OIDC via `coreos/go-oidc/v3`; session-based with CSRF; per-IP + per-user rate limiting on login endpoints
- **Authorization:** three built-in roles (admin/operator/viewer) plus custom roles; granular per-GameServer owner/collaborator fallback; cluster and namespace dimensions on permissions
- **Audit:** structured audit log (database + external sinks: webhook, S3, syslog bridge, stdout) with hash-chain integrity; per-user and per-IP tracking
- **Notifications:** watch CRD status transitions (server health, backup outcomes) and dispatch to admin-configured sinks (Discord, Slack, SMTP, webhook)
- **Module registries:** pluggable providers (CurseForge, Modrinth, Spigot, Hangar, Nexus, Steam, Thunderstore, Factorio, GitHub, UMod); live lookup + caching
- **Multi-cluster dispatch:** `?cluster=` selector routes requests to remote clusters via registered kubeconfigs; home cluster is `local`
- **Telemetry:** opt-in anonymous daily usage metrics (version, server count, template count) POSTed to admin-configured endpoint

## Non-goals / boundaries

The API is a **UX layer only** — it reads and writes CRDs but does **not** contain business logic that belongs in the operator-reconciler:
- Do NOT implement "when GameServer is created, also create a default Backup" as an API-side action; the operator owns that as a reconciler side-effect
- Do NOT compute derived state in the handler (e.g., "server is healthy if all containers are running"); the operator computes status and the API reads it
- Do NOT add custom storage to supplement CRDs; operator-authoritative means every mutation goes through K8s objects and the operator reconciles the outcome

## Directory & package layout

```
api/
├── cmd/
│   ├── main.go           # entry point; subcommands: serve (default) + bootstrap-admin
│   ├── bootstrap.go      # bootstrap-admin implementation
│   └── bootstrap_test.go
├── internal/
│   ├── handlers/         # REST + WebSocket route handlers (lifecycle, users, modules, destinations, config, audit, events, resources, etc.)
│   ├── auth/             # authentication (local argon2id, OIDC, sessions, rate limiting)
│   ├── rbac/             # authorization (role catalog, middleware, permission -> rule mapping)
│   ├── db/               # database driver (sqlite/postgres), schema migrations, query layer
│   ├── kube/             # Kubernetes client wrapper, registry (multi-cluster), server/template helpers
│   ├── audit/            # audit logger (database, webhook sink, S3 sink)
│   ├── notify/           # notification delivery (Discord/Slack/SMTP/webhook watch -> dispatch)
│   ├── ws/               # WebSocket (console RCON/PTY, pod logs, agent client)
│   ├── registry/         # mod registry providers (CurseForge, Modrinth, Spigot, Hangar, Nexus, Steam, Thunderstore, Factorio, GitHub, UMod)
│   ├── scope/            # namespace + cluster resolution from request context
│   ├── telemetry/        # optional anonymous usage metrics collection
│   ├── httperr/          # error -> HTTP status classification (safe message for client, full error logged)
│   └── [test files]
└── go.mod, go.sum
```

**Key packages and responsibilities:**

- **handlers:** 22+ route groups (Audit, AuthProviderSecrets, Cluster, ClusterActions, Clusters, Config, Destinations, Events, Lifecycle, ModIDs, ModSources, Modules, ModUpdates, Notifications, Ownership, PodEvents, Registry, RegistrySecrets, Resources, Roles, SystemLogs, Users, WebSocket Mount)
- **auth:** SessionStore (CSRF + expiry), Local (argon2id password check), OIDC (provider registry + claim mapping), Registry (auth provider discovery per request)
- **rbac:** Middleware (namespace/cluster-scoped permission check + owner/collaborator fallback), rule table (method/path -> permission), catalog (permission definitions)
- **db:** driver-selectable (modernc.org/sqlite or pgx/v5 via postgres build tag), migrations (001-005), Store (query interface)
- **kube:** Client (K8s API wrapper), Registry (per-cluster clients from Cluster CRDs), watch (cluster-config sync)
- **audit:** Auditor (insert to DB + distribute to sinks), webhook sink (POST JSON to URL), S3 sink (object storage), hash-chain (detect tampering)
- **notify:** Notifier (watch GameServer/Backup/Restore status, format + deliver to sinks), sinks (Discord, Slack, SMTP, webhook)
- **ws:** Mount (WebSocket router), agent-client (mTLS to agent), actions (RCON/PTY execution), attach (SPDY proxy), podlogs (live pod logs)
- **registry:** provider types (each implements Search, Details, Manifest, Download), Set (versioned provider pool with key fallback)
- **scope:** ResolveNamespace (extract from path or default), ResolveCluster (validate `?cluster=` against registry)
- **httperr:** classify error type to safe HTTP status + message; preserve full error server-side
- **telemetry:** daily metrics reporter (version, server/template counts)

## External interface / contracts

### Entry point: api/cmd/main.go

Two subcommands:

1. **`serve` (default)** — starts the HTTP server
   - Flags: `--addr`, `--db-driver`, `--db-dsn`, `--log-level`, `--oidc-*`, `--audit-*`, `--agent-*`, `--namespace`, `--cluster-ops`, `--update-channel`, `--curseforge-api-key`, `--telemetry-*`
   - Env overrides via GAMEPLANE_* vars (credentials come from env only, never flags)
   - Initialize: database + migrations, K8s client, auth (local + OIDC), audit (sinks), notifier, cluster watch, telemetry, session GC
   - Routes all mounted at startup; chi router with security middleware (secure headers, body limit, audit, session auth, RBAC, rate limiting)

2. **`bootstrap-admin`** — seed or reset the initial admin user
   - Flags: `--db-driver`, `--db-dsn`, `--username`, `--password`, `--password-stdin`, `--email`, `--display-name`, `--force`, `--enable-local-login`
   - Runs schema migrations like the serve path; password hashed with argon2id
   - Break-glass: `--enable-local-login` alone re-enables local auth in the config row (for OIDC-lockout recovery)

### REST surface (domain-level)

The HTTP server listens on `:8000` (configurable) with these route groups:

**Public (pre-auth):**
- `/auth/providers` — GET: list enabled login methods (no version/host/count, login privacy)
- `/auth/login` — POST: argon2id auth + session creation (rate-limited per IP + user)
- `/auth/logout` — POST: session deletion
- `/auth/oidc/{provider}/start` — GET: IdP authorization flow start
- `/auth/oidc/{provider}/callback` — GET: IdP token exchange (rate-limited)
- `/auth/oidc/start` (legacy) — GET: single helm-provider start
- `/auth/oidc/callback` (legacy) — GET: single helm-provider callback
- `/healthz` — GET: liveness probe
- `/metrics` — GET: Prometheus metrics (openmetrics format)

**Protected (authenticated + RBAC):**
- `/servers/{name}` — CRUD for GameServer CRDs; cluster-dispatch via `?cluster=`; multiplexed console/files
- `/servers/{name}/console` — WebSocket: RCON/exec; cluster-dispatch
- `/servers/{name}:start`, `:stop`, `:restart` — actions (operator-handled)
- `/servers/{name}:collaborators`, `:transfer` — GameServer owner/collaborator management
- `/servers/{name}/files/*` — file browser, upload, download (proxied to agent); cluster-dispatch
- `/templates/{name}` — CRUD for GameTemplate (cluster-scoped)
- `/backups/{name}` — CRUD for Backup (namespaced, cluster-dispatch)
- `/schedules/{name}` — CRUD for BackupSchedule (namespaced, cluster-dispatch)
- `/restores/{name}` — CRUD for Restore (namespaced, cluster-dispatch)
- `/backup-destinations/{name}` — CRUD for restic repo Secrets (namespaced, cluster-dispatch)
- `/modules` — GET: list installed Module CRDs; POST: upload/install
- `/modules/{name}` — CRUD (cluster-scoped)
- `/modules/{name}:uninstall` — action
- `/module-sources` — CRUD for ModuleSource (cluster-scoped)
- `/registry/{provider}/search`, `/{id}` — live mod registry queries (CurseForge, Modrinth, Spigot, etc.)
- `/mod-updates/{name}` — GET: available updates for a mod
- `/mod-ids/{name}` — PATCH: ID-managed mods (ARK CurseForge IDs, Project Zomboid MOD_IDs, Steam Workshop lists)
- `/cluster` — GET: version, nodes, storage; POST: credential-minting (add node, kubeconfig)
- `/cluster/actions` — credential-minting ops (cluster ops flag gated)
- `/clusters` — multi-cluster: list remote Cluster CRDs; create/delete cluster registrations
- `/events` — SSE: real-time K8s events (multiplexed per namespace + cluster)
- `/pod-events` — SSE: pod-level events
- `/users/me` — GET: own profile
- `/users/me/servers` — GET: own GameServers (owner/collaborator)
- `/users/{id}` — CRUD for users (admin only)
- `/users/{id}/role-bindings` — PATCH: role assignments (per namespace + cluster)
- `/roles` — GET catalog and custom roles; POST/PATCH/DELETE custom roles
- `/admin/audit` — GET: audit log (searchable, hash-chain verifiable)
- `/admin/config` — GET/PATCH: global settings (OIDC, notifications, telemetry, module upload limits, etc.)
- `/admin/notifications` — PATCH config + test-send to sinks
- `/admin/auth` — PATCH identity-provider secrets
- `/admin/registries/{provider}/secret` — PATCH mod-registry API keys
- `/admin/system-logs` — GET: control-plane pod logs
- `/admin/cluster/{op}` — cluster operations (add node, etc.)
- `/ws/servers/{name}/console` — WebSocket: RCON command execution (write-capable)
- `/ws/servers/{name}/console-pty` — WebSocket: PTY command execution (write-capable)
- `/ws/servers/{name}/logs` — WebSocket: game/agent log file stream (read-only)
- `/ws/servers/{name}/logs/pod` — WebSocket: pod stdout stream (read-only)

All cluster-dispatch routes accept `?cluster={name}` (validates against registered Cluster CRDs; default is `local`).

### WebSocket bridge

- **`/ws/servers/{name}/console` (GET upgrade)** — RCON to game pod via agent; write-capable
- **`/ws/servers/{name}/console-pty` (GET upgrade)** — PTY/exec to game pod; write-capable
- **`/ws/servers/{name}/logs` (GET upgrade)** — game/agent log file stream via agent; read-only
- **`/ws/servers/{name}/logs/pod` (GET upgrade)** — pod stdout stream via Kubernetes watch; read-only
- All authenticate via session + mTLS to agent (for console routes)
- Multiplexed per `?cluster=` + namespace

### RBAC roles (three built-in + custom)

**Built-in roles:**
- **admin:** wildcard permission `*`; full access to all resources and config
- **operator:** read/write servers, backups, schedules, templates, modules; read destinations, cluster, roles
- **viewer:** read-only across servers, backups, schedules, templates, modules, destinations, cluster, roles

**Permissions (granular, per resource and action):**
```
servers:read, servers:write, servers:console (namespaced)
backups:read, backups:write, backups:restore (namespaced)
schedules:read, schedules:write (namespaced)
templates:read, templates:write (cluster-scoped)
modules:read, modules:manage (cluster-scoped)
destinations:read, destinations:manage (namespaced)
cluster:read, cluster:manage (cluster-scoped)
users:read, users:manage (cluster-scoped)
roles:read, roles:manage (cluster-scoped)
audit:read, config:read, config:manage (cluster-scoped)
```

**Binding dimensions:**
- Per-user + per-role (many-to-many)
- Per-namespace (namespaced permissions; `*` = cluster-wide)
- Per-cluster (multi-cluster; `*` = all clusters, but typically scoped to specific cluster)

**Owner/collaborator fallback:**
- When namespace permission is denied and request targets a GameServer, check if caller is owner or collaborator
- Owner-only operations (`:transfer`, `:collaborators`, `:wipe-data`, bare DELETE) deny collaborators
- Fetch GameServer from `?cluster=` (cluster-gated in middleware)

## Key invariants

1. **Operator-authoritative:** every mutating request goes through K8s API (Create/Patch/Delete on CRDs); API waits for operator to reconcile status
2. **Every mutating request audited:** audit middleware logs actor, method, path, target, status, IP to database + external sinks
3. **Three-role baseline RBAC:** admin/operator/viewer roles reproduce historical permission matrix exactly
4. **Multi-dimensional RBAC:** namespace + cluster + owner/collaborator dimensions; cluster gating prevents cross-cluster privilege escalation
5. **Append-only migrations:** database schema mutations are irreversible (migrations 001-005); no down-migrations
6. **Login rate limiting:** per-IP (burst 10, 5/min) + per-user (burst 6, 3/min) on `/auth/login` + OIDC callback
7. **Audit hash-chain:** each audit_events row includes hash of previous row (prev_hash) + its own content hash (hash); detects DB-level UPDATE/DELETE tampering
8. **Session CSRF protection:** CSRF token paired with session token; validated on state-changing requests
9. **Secure error handling:** internal errors (DB failures, K8s API errors) logged in full; safe generic messages sent to clients
10. **Cluster dispatch validation:** `?cluster=` matched against registered Cluster CRDs via registry; unknown cluster is a 400

## Dependencies

**Internal (same workspace via go.work):**
- `github.com/ValgulNecron/gameplane/netguard` — SSRF dial-guard for outbound HTTP (module registry fetches)
- `github.com/ValgulNecron/gameplane/gameaction` — console-injection guard + command-template renderer

**External (go.mod):**
- `github.com/go-chi/chi/v5` v5.1.0 — HTTP router, middleware
- `github.com/coder/websocket` v1.8.12 — WebSocket upgrade + streaming
- `github.com/coreos/go-oidc/v3` v3.11.0 — OIDC provider discovery + token validation
- `github.com/go-jose/go-jose/v4` v4.0.2 — OIDC JWT parsing (transitive via go-oidc)
- `github.com/jackc/pgx/v5` v5.5.4 — PostgreSQL driver (build tag: postgres, experimental)
- `github.com/minio/minio-go/v7` v7.2.1 — S3-compatible client (audit sink)
- `github.com/prometheus/client_golang` v1.23.2 — Prometheus metrics
- `modernc.org/sqlite` v1.34.1 — SQLite driver (production, tested)
- `golang.org/x/crypto` v0.53.0 — argon2id password hashing
- `golang.org/x/oauth2` v0.30.0 — OAuth2 token exchange (OIDC flow)
- `k8s.io/api` v0.35.0, `k8s.io/apimachinery` v0.35.0, `k8s.io/client-go` v0.35.0 — Kubernetes API
- `sigs.k8s.io/controller-runtime` v0.23.3 — K8s client (dynamic unstructured access)

Verify from `/api/go.mod`.

## Data & persistence

### Database driver selection

- **Production (default):** `modernc.org/sqlite` — file-based, WAL mode, tested
- **Experimental:** PostgreSQL via `jackc/pgx/v5` — compile with `-tags=postgres`
- Driver selected at startup via `--db-driver` (sqlite|postgres) + `--db-dsn`
- Migrations run automatically on startup (`store.Migrate(ctx)`)

### Schema (migrations 001-005)

**001_init.sql:**
- `users` — username (unique), email, display_name, pw_hash (argon2id), role (legacy, now via role_bindings), created_at, updated_at
- `sessions` — token (PK), user_id (FK), csrf_token, expires_at
- `oidc_links` — (issuer, subject) -> user_id (many-to-one); email claim
- `audit_events` — ts, actor, method, path, target, status, ip
- `api_tokens` — token (PK), user_id, name, last_used

**002_config.sql:**
- `config` — key (PK), value, updated_at

**003_roles.sql:** (custom roles + granular permissions)
- `roles` — name (PK), description, builtin flag
- `role_permissions` — (role_name, permission) junction
- `user_role_bindings` — (user_id, role_name, namespace) junction; seeded with pre-existing user roles on '*' namespace

**004_cluster_rbac.sql:** (multi-cluster support)
- Alters `user_role_bindings` primary key to (user_id, role_name, cluster, namespace); backfill to 'local' cluster

**005_audit_chain.sql:** (hash-chain integrity)
- Adds `prev_hash` and `hash` columns to `audit_events` for tamper detection

All foreign keys are enforced only on Postgres (modernc-sqlite runs with FK OFF); API layer is authoritative.

## Security considerations

### Authentication
- **Local:** argon2id (Argon2id13, m=64MiB, t=3, p=2) with per-user random salt; ~200ms per check
- **OIDC:** `coreos/go-oidc/v3` discovery + `go-jose/v4` JWT validation; claims mapping (email, groups, roles)
- **Sessions:** cryptographically random token + paired CSRF token; memory store + DB persistence; expiry at midnight UTC
- **Bootstrap:** `bootstrap-admin` subcommand hashes password same way as API

### Authorization
- **RBAC middleware:** intercepts all protected routes; namespace + cluster gating
- **Owner/collaborator fallback:** fallback only when RBAC denies AND GameServer is explicitly named; fail-closed on malformed paths
- **Cluster dispatch validation:** `?cluster=` against registry; unknown cluster is a 400 (malformed request, not 403 forbidden)

### Audit
- **Scope:** every mutating request (POST/PATCH/DELETE); reads excluded
- **Sinks:** database (table audit_events) + webhook (POST JSON) + S3 (object storage) + syslog bridge (HTTP->syslog relay) + stdout (structured logs)
- **Hash-chain:** prev_hash + hash computed at insert time; Verify tool detects tampering
- **Retention:** optional daily prune (--audit-retention-days)

### Outbound safety
- **netguard SSRF guard:** on module registry fetches (HTTP/HTTPS); permissive allowlist (modular, self-hosted registries on loopback OK)
- **mTLS to agent:** client cert + key validate console operations

### Error handling
- **httperr package:** internal errors (K8s 404, DB constraint, FS path) mapped to safe HTTP status + generic message
- **500+ errors:** never echoed to client; full error logged at Error level server-side
- **4xx errors:** hand-crafted safe messages by handlers (e.g., "ref is required", already classified)

### Login privacy (rule 3)
- `/auth/providers` omits version, cluster name, server count, hostnames
- `/login` error is always "invalid credentials" (never "wrong password" vs "unknown user")
- No internal metrics visible pre-auth

## Testing & coverage

### Test tiers

1. **Unit tests** (localhost, no K8s)
   - Mocked auth, DB (in-memory sqlite)
   - Package-level tests (`*_test.go` in each internal package)
   - Fast (<1 min total)

2. **Envtest integration** (K8s envtest assets 1.31)
   - Real K8s API server (in-memory)
   - Real database (file-backed sqlite)
   - Coverage merged with unit tests
   - Handlers + CRD reconciliation flows
   - ~2 min

3. **E2E (kind + Helm)**
   - Real kind cluster (1.31)
   - Real Helm chart installation
   - Real agents + console/file operations
   - Bucket sharding (api-auth, api-rbac, api-agent, ratelimit, bot, multicluster)
   - ~10-20 min total (parallel buckets on CI)

### Coverage gate

**Target:** 80% (api/.testcoverage.yml)

Excluded:
- `cmd/` (main.go + flag/signal wiring)
- `internal/db/db_postgres.go` (Postgres driver, build-tag gated, needs Docker/testcontainers; tracked separately on nightly)

Final 20% gap concentrated in:
- `ws/attach.go` handle() (SPDY exec proxy, exercised by e2e)
- `auth/oidc.go` last-mile error paths (id_token claim parse failures)
- `events.go` SSE pump (live kube-watch, also e2e)

## References

- **`docs/architecture.md`** — components, data flow, security boundaries, operator-authoritative rationale
- **`docs/security.md`** — detailed threat model, auth/RBAC/audit design, pre-auth privacy
- **`docs/notifications.md`** — notification sink formats + test delivery
- **`docs/oidc.md`** — OIDC provider setup, claim mapping, role inference
- **`docs/module-authoring.md`** — module registry bundle format, ModuleSource CRD
- **`CLAUDE.md`** rule 10 — "The operator is authoritative" principle (API is UX layer only)
- **`api/go.mod`** — dependency versions (source of truth for go.mod)
- **Makefile** — `make test-go`, `make cover`, `make lint-go`, `make images`; CI runs via GitHub Actions
