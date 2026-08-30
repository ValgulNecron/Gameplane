# api — Specification

**Status:** beta (v0.2.0-beta.8)  
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

- **handlers:** 23+ route groups (Audit, AuthProviderSecrets, Capture, Cluster, ClusterActions, Clusters, Config, Destinations, Events, Lifecycle, ModIDs, ModSources, Modules, ModUpdates, Notifications, Ownership, PodEvents, Registry, RegistrySecrets, Resources, Roles, SystemLogs, Users, WebSocket Mount)
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
   - **Core flags:** `--addr`, `--db-driver`, `--db-dsn`, `--log-level`
   - **OIDC flags** (install-time, Helm-seeded): `--oidc-issuer`, `--oidc-client-id`, `--oidc-client-secret`, `--oidc-redirect-url`, `--oidc-display-name` (login button label, no hostname — pre-auth surface), `--oidc-groups-claim` (configurable claim name, defaults to "groups"), `--oidc-default-role` (default for unmapped users: "", "viewer", "operator", "admin", or "deny"), `--oidc-role-mapping-admin` (comma-separated group names for admin role), `--oidc-role-mapping-operator`, `--oidc-role-mapping-viewer`. All have `GAMEPLANE_OIDC_*` env fallbacks (preferred over flags for credentials).
   - **Storage class (report-only):** `--game-data-storage-class` — echoed in `GET /admin/config`'s `installTimeSettings.gameDataStorageClass`, read-only, unaffected by overrides.
   - **Other flags:** `--audit-*`, `--agent-*`, `--namespace`, `--cluster-ops`, `--update-channel`, `--curseforge-api-key`, `--telemetry-*`, `--capture-*` (default retention, max retention, max duration, max size)
   - Env overrides via GAMEPLANE_* vars (credentials come from env only, never flags)
   - Initialize: database + migrations, K8s client, auth (local + OIDC with Helm-seeded provider synthesis), audit (sinks), notifier, cluster watch, telemetry, session GC, capture config
   - Routes all mounted at startup; chi router with security middleware (secure headers, body limit, audit, session auth, RBAC, rate limiting); MountCapture wires capture endpoints

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
- **[PLANNED, Phase 8 Dashboard]** `/servers/{name}:capture-enable`, `:capture-disable` — sidecar lifecycle actions (endpoints stubbed; handlers not yet implemented; RBAC rules already in place for when implemented)
- `/servers/{name}:capture-start` — POST: start a network packet capture (creates NetworkCapture CR, transitions to Pending)
- `/servers/{name}:capture-stop` — POST: stop an active capture (updates NetworkCapture status to Completed)
- `/servers/{name}:captures` — GET: list all NetworkCaptures (active and historical) for a server, excluding Expired captures
- `/servers/{name}:capture` — GET: fetch a single capture's metadata and status; query param `id={captureId}`; 404 if not found or Expired
- `/servers/{name}:capture-file` — GET: download completed PCAPNG file from the capture sidecar; query param `id={captureId}` (proxied to sidecar over mTLS; 409 if still running)
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

### Network capture endpoints

**Mounting & configuration:**
- `MountCapture(r chi.Router, reg *kube.Registry, auditor *audit.Auditor, cfg CaptureConfig, agentCABundle, agentClientCert, agentClientKey string)` — wires 7 implemented capture endpoints on a chi.Router with cluster-dispatch and mTLS client pool for sidecar communication
- `type CaptureConfig struct { FeatureEnabled bool; DefaultRetentionSeconds, MaxRetentionSeconds int64; DefaultMaxDurationSecs int; DefaultMaxSizeBytes int64 }` — cluster-wide capture feature flag, defaults, and size/duration limits; passed from `cmd/main.go`'s flag parsing
- Registered in api/cmd/main.go line 281 with CaptureConfig fields bound from CLI flags `--capture-*` (feature-enabled, default-retention-seconds, max-retention-seconds, default-max-duration-secs, default-max-size-bytes)

**Implemented endpoints (7 routes, all cluster-dispatch via `?cluster=`, all require `captures:manage` RBAC permission, all authenticate via session, audit all writes synchronously before response):**

- **POST `/servers/{name}:capture-start`** — Create a NetworkCapture CR and transition to Pending; request body: `{filter?: string, maxDurationSeconds: int, maxSizeBytes: int64, ttlSecondsAfterFinished?: int64}`; response: `{captureId, phase, serverName, filter, maxDurationSeconds, maxSizeBytes, ttlSecondsAfterFinished, createdAt, startedAt?, completedAt?, bytesWritten, packetsWritten}` (HTTP 202 Accepted)
  - Verifies server exists and `spec.capture.enabled = true`; returns 400 if capture not enabled on server
  - Validates pcap-filter expression before CRD creation (FR-003; character whitelist + length check, no full BPF compile); returns 400 on invalid filter (e.g., control chars, >1024 chars)
  - Enforces maximum one Pending/Running capture per server (rejects with 409 Conflict if one exists); checks both `status.capture.activeCapture` and scans all NetworkCaptures as a guard against eventual consistency lag
  - Validates duration (1..3600s) and size (1..cluster-max bytes); returns 400 if out of range
  - Clamps TTL to cluster maximum at request time; returns 400 if exceeds max (cluster maximum is 604800s / 7 days by default, storage-limitation-informed, not a legal requirement)
  - Creates NetworkCapture CR with ownerReference to GameServer (cascade delete on server deletion)
  - Audit: `WriteSync()` before response body is sent (FR-006); reason field records "server_not_found", "capture_not_enabled", "invalid_filter", "invalid_duration", "invalid_size", "capture_in_progress", "ttl_exceeded", "create_failed", or "" on success

- **POST `/servers/{name}:capture-stop`** — Transition a NetworkCapture from Pending/Running to Completed; request body: `{captureId: string}`; response: `{captureId, phase, serverName, filter, createdAt, startedAt?, completedAt?, stoppingReason, bytesWritten, packetsWritten}` (HTTP 200 OK)
  - Returns 400 if `captureId` is missing or empty in request body
  - Returns 404 if capture not found or doesn't belong to this server
  - Returns 409 if capture is not in Pending or Running phase (already Completed/Failed/Expired)
  - Sets capture status to Completed, records `completionTime`, and sets `message="stopped by user request"` so operator's reconciler tells the sidecar to stop (once per request, guarded by reconciler-side condition)
  - Audit: `WriteSync()` before response; reason field records "missing_id", "not_found", "not_running", "stop_failed", or "" on success

- **GET `/servers/{name}:captures`** — List all NetworkCaptures for a server (active and historical); response: `{captures: [...], total: int, limit: 100, offset: 0}`
  - Filters to captures whose `spec.serverRef.name` matches the server
  - Excludes Expired captures (phase == Expired)
  - Includes Pending/Running/Completed/Failed; client can filter by phase
  - Each item includes metadata, phase, timestamps, packet/byte counts, and computed `expiresAt` field (when the capture will auto-delete if TTL is set)
  - Audit: `WriteSync()` before response (non-GET auditing, rare but required for capture list due to FR-006 gap on GET operations)

- **GET `/servers/{name}:capture`** — Fetch a single capture's metadata, status, and counts; query param `id={captureId}`; response: capture object with full details (HTTP 200)
  - Returns 400 if `id` is missing or empty
  - Returns 404 if capture not found, doesn't belong to this server, or phase == Expired
  - Response includes phase, timestamps (createdAt/startedAt/completedAt), filter, limits, byte/packet counts, and computed `expiresAt`
  - Audit: `WriteSync()` before response; reason "not_found", "expired", or ""

- **GET `/servers/{name}:capture-file`** — Download completed PCAPNG file from sidecar; query param `id={captureId}` (HTTP 200 on success)
  - Returns 400 if `id` is missing or empty
  - Returns 404 if capture not found or doesn't belong to this server
  - Returns 404 if capture phase == Expired (TTL window elapsed)
  - Returns 409 if capture phase == Pending or Running (still recording)
  - Proxies from sidecar's `https://<gs>-agent.<ns>.svc.cluster.local:9091/captures/{id}/file` over mTLS
  - **CRITICAL (FR-006):** Audit `WriteSync()` with status code BEFORE streaming starts (on both success and error paths); if audit write fails, returns 500 and stops download entirely (audit failure fails the operation)
  - Sets response headers: `Content-Type: application/vnd.tcpdump.pcap`, `Content-Disposition: attachment; filename="capture-{id}.pcapng"`
  - Streams file without buffering via `io.Copy(responseWriter, sidecarResponse.Body)` so large captures don't accumulate in memory
  - On sidecar error (non-2xx), classifies via `writeUpstreamError` (timeout→504, other transport error→502); error message is safe generic text to client, full error logged server-side
  - Audit reason field: "not_found", "not_running", "expired", "invalid_host", "download_failed", or "" on success; a second audit row (with "download_failed") is written post-stream if sidecar returned non-2xx (to correct the initial optimistic row)

- **Error responses:** All errors are plain text via `httperr.WriteCode()`, no JSON envelope. Status codes: 400 (validation), 404 (not found), 409 (conflict/wrong state), 503 (sidecar unavailable), 500 (internal error)

**Fully implemented endpoints (all routing registered, RBAC gated, handlers complete):**

- **POST `/servers/{name}:capture-enable`** — Enable capture sidecar injection on a GameServer (sets `spec.capture.enabled = true`, triggers live injection)
  - Patches GameServer spec directly; operator watches and injects sidecar as ephemeral container live into running pod (rule 10: operator is authoritative)
  - Gated by `captures:manage` permission
  - Audit: synchronous write before response with reason "feature_disabled", "server_not_found", "terminating", "patch_failed", or "" on success

- **POST `/servers/{name}:capture-disable`** — Disable capture on a GameServer (sets `spec.capture.enabled = false`)
  - Patches GameServer spec directly; operator stops any running capture and rejects new ones, but the ephemeral container remains in place until the next pod recreation (Kubernetes design constraint: ephemeral containers cannot be removed without pod recreation)
  - **ASYMMETRY (US2 central design point):** Disabling stops accepting new capture-start requests and stops any running capture but the container lingers until the pod is next recreated (e.g., on node drain, replica restart, or manual delete)
  - Gated by `captures:manage` permission
  - Audit: synchronous write before response with reason "feature_disabled", "server_not_found", "terminating", "patch_failed", or "" on success

### OIDC role mapping and Helm-seeded provider (install-time configuration)

**Helm-seeded provider synthesis & coexistence:**
- **Startup:** API synthesizes a read-only "helm" provider when `--oidc-issuer` is set (Helm-seeded OIDC configuration exists). This synthetic provider coexists with:
  - Dashboard-managed OIDC providers (stored in the "auth" config row, can be created/edited/deleted via API)
  - Local login (always available unless explicitly disabled)
  - Legacy fallback paths for backward compat (legacy provider mirrors Helm config as identity)
- **Provenance:** The "helm" provider's groupsClaim, defaultRole, and roleMappings come from CLI flags (`--oidc-groups-claim`, `--oidc-default-role`, `--oidc-role-mapping-*`) only. It is not a database row; it is synthesized at runtime from flags and the optional `helmOverride.roleMappings` overlay (see below).
- **Per-login re-evaluation:** Role mappings are read at login time from the current "auth" config row, so override changes take effect immediately on the next login without restart or Helm upgrade.

**Group-claim extraction & role assignment:**
- **Claim name:** Defaults to "groups" when `--oidc-groups-claim` is empty or unset. Otherwise uses the configured name. Extraction honors the configured (or default) name on every login (not cached).
- **First-match precedence:** Roles are assigned in strict precedence order: admin → operator → viewer. A user whose IdP groups match any group in the admin mapping gets the admin role, even if they also match operator/viewer groups. If no tier matches, the policy's DefaultRole applies ("", "viewer", "operator", "admin", or "deny" to refuse login).
- **Per-login re-evaluation:** Role assignment is computed fresh on every successful OIDC login, so a user's role changes immediately if their IdP groups change (or if the group-claim name in config changes).
- **Two safety guards:**
  1. Role assignment only applies when mappings are explicitly configured (when overrides exist or when Helm seeded them). No automatic role assignment from bare group names without explicit mapping.
  2. Demotion guard: A user who is the **only user able to manage users** (sole admin, or sole admin-equivalent) cannot be demoted or removed from the admin role by the login-time role assignment flow. This prevents accidental lockout: if a user is currently the only admin and a role remapping would remove their admin status, the remapping is skipped (logged as warn), leaving them as admin. The guard applies only on login re-evaluation; the override API (PATCH /admin/config/auth) does not enforce it (an explicit admin action).

**The helmOverride overlay:**
- **Storage:** Lives in the "auth" config row as `helmOverride.roleMappings.{admin, operator, viewer}`. No separate table, no migration beyond the existing config table. The entire overlay is optional.
- **Shape:** Each role key (admin/operator/viewer) is independently optional:
  - **Absent:** use the Helm-seeded value (from `--oidc-role-mapping-*` flags)
  - **Non-nil array (including `[]`):** override with this array; `[]` means "nobody maps to this role"
  - Provenance is **derived from key presence** — no explicit `source` field exists. A key's presence alone means DB-overridden; absence means Helm-seeded.
- **Merge semantics:** Non-nil replaces the Helm seed in full (not a merge); `[]` is distinct from absent.
- **Reset route:** `DELETE /admin/config/auth/role-mappings/{role}` removes a single role's key, restoring the Helm-seeded value for that role.
- **API:** Writes go through `PUT /admin/config` with the full auth config (including the helmOverride overlay). Reads via `GET /admin/config` return both the current override state (helmOverride) and the original Helm seed (installTimeSettings.oidcHelmProvider) as separate fields, never merged.
- **Audit:** Set/change operations are audited with reason "oidc role mapping override set: role=<role> groups=<groups>". Reset operations are audited with reason "oidc role mapping override reset: role=<role>".

**installTimeSettings response object:**
- **Returned by:** `GET /admin/config` (requires `config:read` permission)
- **Content:** Read-only snapshot of Helm-seeded values captured at startup:
  - `gameDataStorageClass` (string, the value of `--game-data-storage-class` flag)
  - `oidcHelmProvider` (object with `groupsClaim`, `defaultRole`, `roleMappings.{admin, operator, viewer}` — the original Helm seed, never merged with overrides)
- **Immutability:** Never changes after startup, unaffected by any override (helmOverride.roleMappings changes do not appear here). Allows clients to always see what was Helm-configured vs. what was overridden.
- **Presence:** Absent entirely when no install-time values are reportable (empty storage class, no Helm OIDC).

### Capture operation auditing

**Scope — which operations are audited** (per T010, specs/done_003-network-capture-sidecar/research.md):

- **REQUIRED (FR-006):** All capture write/delete operations are audited synchronously:
  - POST `:capture-enable` (enable) — recorded before response
  - POST `:capture-disable` (disable) — recorded before response
  - POST `:capture-start` (start) — recorded before response
  - POST `:capture-stop` (stop) — recorded before response
  - GET `:capture-file` (download) — recorded before response (despite being a GET, because FR-006 explicitly names "download" as a required audit event)
  - DELETE `:capture` (delete, future Phase 2 task) — recorded before response
  
- **RECOMMENDED (product decision, not FR-006-mandated):** GET `:captures` (list) audit is recommended but not required by FR-006

- **NOT REQUIRED:** GET `:capture` (get-status) audit is not required by FR-006 and no handler-side write is performed for this endpoint

**Audit implementation pattern** (api/internal/audit/audit.go and api/internal/handlers/capture.go):

- **Synchronous writes:** All capture handlers call `Auditor.WriteSync(ctx, method, path, target, reason, status)` directly before writing the HTTP response body to the client
- **Signature:** `WriteSync(ctx context.Context, method, path, target, reason string, status int) error` (api/internal/audit/audit.go:689)
- **Timing:** The audit row is inserted into the database **before WriteHeader is called**, so if the audit write fails, the handler returns HTTP 500 (internal error) without sending the response body — **a failed audit write fails the entire capture operation (FR-006)**
- **Error handling:** WriteSync returns an error if the database write fails; the handler must check the return value and bail if it's non-nil. This is enforced by a helper method `auditWriteOrFail` in capture.go, which returns false on write failure (the handler then returns 500 without proceeding)

**Audit reason field** (migration 007_audit_reason.sql and audit.go):

- **Column:** `audit_events.reason` (nullable TEXT, added by migration 007_audit_reason.sql, api/internal/db/migrations/)
- **Purpose:** Structured, machine-readable reason codes for operations that need fault reporting beyond HTTP status — e.g., capture start may fail with reasons "server_not_found", "capture_not_enabled", "invalid_filter", "invalid_duration", "invalid_size", "capture_in_progress", "ttl_exceeded", "create_failed", or "" on success
- **Hash chain compatibility** (api/internal/audit/audit.go:309-331):
  - The `reason` field **participates in the hash chain ONLY when non-empty** (api/internal/audit/audit.go computeHash function, lines 329-332)
  - Pre-migration rows (written before 007_audit_reason.sql shipped) have NULL/empty reason and were hashed without a reason field; their stored hash remains valid and is verified bit-for-bit
  - Post-migration rows with non-empty reason include it in their content hash; two rows differing only in reason hash differently
  - The Verify function (api/internal/audit/audit.go:455+) correctly reconstructs this at each row: pre-migration rows are hashed without reason (as originally computed), post-migration rows with non-empty reason include it
  - This preserves backward compatibility: the hash chain is never invalidated by the migration, and pre-existing audit rows remain verifiable

**Fan-out to external sinks:**

- WriteSync passes the reason field to all configured external sinks (webhook, S3, stdout) via the Event struct (api/internal/audit/audit.go:722-731), enabling external audit systems to capture structured failure reasons alongside the HTTP status code

**Captures:manage permission model** (api/internal/rbac/ and api/internal/db/migrations/):

- **Permission key:** `captures:manage` (defined in api/internal/rbac/catalog.go:71, namespaced)
- **Label:** "Enable, start, stop, download, and delete packet captures"
- **Seeding:** Only the **admin role** has this permission, via the wildcard admin permission `*` (admin holds `perm: "*"` in 003_roles.sql:48, which covers all permissions including captures:manage)
- **Migration 008 (008_captures_rbac.sql):** This migration deliberately does NOT explicitly INSERT captures:manage into role_permissions for the admin role, because admin's permission set must remain exactly `['*']` as verified by the test TestRoles_ListIncludesBuiltins
- **Grantability:** captures:manage is a normal catalog permission that *could technically* be granted to custom roles via the roles API (it has no structural restriction like the `*` wildcard does). However, the default seeding keeps it admin-only; future phases may decide to allow operator or custom-role grantability (T012 in research.md left that as an open decision)
- **Ownership fallback limitation:** The owner/collaborator fallback in rbac.go (lines 111-127) is explicitly restricted to `servers:read`, `servers:write`, and `servers:console` permissions. Even a GameServer owner without the `captures:manage` permission will get HTTP 403 Forbidden on all capture endpoints

**RBAC rule-table ordering (critical security constraint)**:

- **Constraint:** All 8 capture permission checks in `api/internal/rbac/rbac.go` (lines 189-196) MUST precede the `servers:write` catch-all rule (line 201)
- **Why it matters:** Path matching is done on the first segment, stripped of any verb suffix ("servers"). So `/servers/{name}:capture-start` matches segment "servers" just like `/servers/{name}` (ordinary CRUD). An unordered insertion of capture rules after the `servers:write` catch-all (which the operator role holds via 003_roles.sql:48) would silently grant all 8 capture endpoints to the operator role, **breaking the security requirement that only admin has capture permissions (FR-005/SC-005) with no test failure, no log line, no observable error — just silent escalation**
- **What breaks if reordered:**
  - Any HTTP verb on any path matching `/servers/{name}:capture-*` would resolve to `servers:write` permission instead of `captures:manage`
  - The operator role holds `servers:write`, so operators would gain full capture access, undermining admin-only isolation
  - Capture audit entries would still flow through the audit middleware, but the RBAC denial that should have fired never happens
  - No CI test currently detects this regression
- **Regression test guards** (api/internal/handlers/capture_test.go and rbac_test.go):
  - Test TestCapture_OnlyAdminCanAccess (or similar) verifies that operator role gets 403 on all 8 capture endpoints
  - Test TestRBACRuleOrdering (or similar) validates that capture rules appear before servers:write in the rule table (structural assertion on rule slice ordering)
  - These tests prevent accidental reordering in future edits

**Security & invariants:**

- **Asymmetry clarity:** Enable is live and restart-free. Disable is similarly live but not truly "off"  — the container remains in the pod until recreation. Spec.md documents this precisely; dashboa will surface it to users (e.g., "Capture disabled but sidecar remains until pod restart").
- **RBAC ordering (critical):** All 8 capture permission checks in `api/internal/rbac/rbac.go` (lines 189-196 in rbac.go) MUST precede the `servers:write` catch-all. Path matching is done on segment "servers" (via chi's `{name}` segment stripping), so a /servers/{name}:verb verb routes as "servers" segment. An unordered insertion after servers:write would silently grant capture access to the operator role (which holds servers:write), breaking the security requirement that only admin has capture permissions (FR-005/SC-005). CI does not currently detect this regression; future edits must preserve order or add a structural test.
- **Concurrent capture limit (API + operator):** API enforces at creation time (rejects 409); operator enforces again at reconciliation time. Two independent gates prevent race conditions and eventual-consistency lag.
- **Sidecar authentication:** mTLS only; both client cert+key and server CA are passed from cmd/main.go to MountCapture and built into an http.Client. No sidecar auth tokens in URLs (SSRF risk), no secrets echoed in logs.

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
captures:manage (namespaced)
cluster:read, cluster:manage (cluster-scoped)
users:read, users:manage (cluster-scoped)
roles:read, roles:manage (cluster-scoped)
audit:read, config:read, config:manage (cluster-scoped)
```

**Network capture permissions:**
- `captures:manage` — enables, starts, stops, downloads, and (future) deletes packet captures (namespaced, scoped to GameServer within namespace)
  - Required for all 7 implemented capture endpoints (`:capture-enable`, `:capture-disable`, `:capture-start`, `:capture-stop`, `:captures`, `:capture`, `:capture-file`)
  - Future Phase 2 task: DELETE `:capture` endpoint for explicit capture deletion (8th rule, also gated by captures:manage)
  - Seeded to **admin role only** (migration 008 and admin's `*` wildcard; no explicit custom-role or operator-role grant)
  - RBAC rule table ordering: All 8 capture rules (including the future DELETE rule) MUST precede the `servers:write` catch-all to prevent silent operator-role escalation (see invariant 12)

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
5. **Append-only migrations:** database schema mutations are irreversible (migrations 001-008); no down-migrations
6. **Login rate limiting:** per-IP (burst 10, 5/min) + per-user (burst 6, 3/min) on `/auth/login` + OIDC callback
7. **Audit hash-chain:** each audit_events row includes hash of previous row (prev_hash) + its own content hash (hash); detects DB-level UPDATE/DELETE tampering
8. **Audit pagination is bounded:** The `Auditor.Page(ctx, limit)` method clamps the untrusted `limit` parameter to a maximum of 500 entries (`MaxAuditPageSize`). Clamping occurs at both the API handler layer (api/internal/handlers/audit.go line 25) and the store layer (api/internal/audit/audit.go lines 820–822) so untrusted input is bounded at the earliest opportunity and again at use time. The allocated slice is always created with capacity within the bound (`make([]Event, 0, limit)` after clamping), guaranteeing that no untrusted client input can cause unbounded memory allocation regardless of how the limit value flows through the system.
9. **Session CSRF protection:** CSRF token paired with session token; validated on state-changing requests. The CSRF cookie is deliberately **not** `HttpOnly` — the SPA reads it via JS and echoes it back as the `X-Gameplane-CSRF` header (double-submit pattern); making it `HttpOnly` would break the protection it's providing. The session cookie itself stays `HttpOnly`. Cookie *clearing* (logout) always sends `HttpOnly` regardless of the original cookie, since a delete carries no value for a script to read and the browser matches the clear on Name/Domain/Path alone. This is why the `.golangci.yml` gosec G124 exclusion is scoped narrowly to `api/internal/auth/sessions.go`.
10. **Cluster-watch logging is privacy-preserving:** The `kube.Watch` goroutine logs cluster synchronization events using only the cluster name and error details, never secret material. While cluster credentials are held in memory and used to construct clients, logging never includes kubeconfig bytes, API tokens, or other sensitive data. The taint-source variable naming (`kubeconfigField` for field names like "kubeconfig") is distinct from the actual credential bytes flowing through the client, preserving clarity for future readers and static analysis.
11. **Secure error handling:** internal errors (DB failures, K8s API errors) logged in full; safe generic messages sent to clients
12. **Cluster dispatch validation:** `?cluster=` matched against registered Cluster CRDs via registry; unknown cluster is a 400
13. **WebSocket/HTTP proxy path validation:** `api/internal/ws/dialer.go` takes the namespace and pod name from the request path before building the agent's upstream URL. Both are validated as DNS-1123 labels (`isDNS1123Label`) and rejected with a 400 before any URL is constructed — gosec's taint analysis doesn't model a custom validator as a sanitizer, hence the scoped G704 exclusion on that file.
14. **Capture rule-table ordering:** All 8 capture permission checks (POST `:capture-enable`, `:capture-disable`, `:capture-start`, `:capture-stop`; GET `:captures`, `:capture`, `:capture-file`; DELETE `:capture`) **MUST precede** the `servers:write` catch-all rule in `api/internal/rbac/rbac.go` lines 189-196 before line 211. Because all `/servers/{name}:verb` paths match the segment "servers" (chi's `{name}` segment strips the verb suffix), an unordered insertion after `servers:write` (which the operator role holds) would **silently grant all 8 capture endpoints to the operator role**, breaking the security requirement that only admin has capture permissions (FR-005/SC-005). This regression is a structural bug CI does not currently catch — moving the capture rules after servers:write is a one-line security break. Any future RBAC edits must preserve this order; consider a structural test to prevent silent reordering.
15. **Helm-seeded OIDC role mappings:** The "helm" provider is synthesized from CLI flags and the optional helmOverride overlay. No database migration carries the Helm seed; it lives only in flags. The helmOverride.roleMappings lives on the existing "auth" config row, per-role independently optional (key presence = overridden, absence = use Helm seed). Re-evaluated at login time so changes take effect without restart. Demotion guard prevents removing the last user able to manage users.

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

### Schema (migrations 001-008)

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

**006_share_links.sql:** (unauthenticated server access tokens)
- Creates `share_links` table: signed, expiring, revocable tokens for unauthenticated access to a single GameServer's status and connection address, optionally with start capability
- Token never stored; only SHA-256 hash persisted and indexed for O(1) lookup
- Pre-existing; not part of the Phase 2 Foundational feature scope

**007_audit_reason.sql:** (Phase 2 Foundational: capture operation auditing)
- Adds nullable `reason TEXT` column to `audit_events`
- Used by synchronous audit writes (`Auditor.WriteSync`) to record machine-readable failure reasons for operations like network captures
- Existing rows retain NULL `reason`; the audit hash-chain boundary preserves backward compatibility (see below)

**008_captures_rbac.sql:** (Phase 2 Foundational: capture permissions)
- Seeds `captures:manage` permission to admin role via `INSERT INTO role_permissions(role_name, permission) VALUES ('admin', 'captures:manage')`
- Only admin role grants capture access in Phase 2 Foundational; future phases determine operator role grantability

All foreign keys are enforced only on Postgres (modernc-sqlite runs with FK OFF); API layer is authoritative.

## Security considerations

### Authentication
- **Local:** argon2id (Argon2id13, m=64MiB, t=3, p=2) with per-user random salt; ~200ms per check
- **OIDC:** `coreos/go-oidc/v3` discovery + `go-jose/v4` JWT validation; claims mapping (email, groups, roles)
- **Sessions:** cryptographically random token + paired CSRF token; memory store + DB persistence; expiry at midnight UTC
- **CSRF cookie is JS-readable by design:** unlike the session cookie (`HttpOnly`), the CSRF cookie is set `HttpOnly: false` so the SPA can read its value and echo it back as `X-Gameplane-CSRF` on mutating requests — the standard double-submit pattern (see `docs/security.md`). Logout's cookie-clear always sends `HttpOnly: true` regardless, since a MaxAge<0 delete carries no value and the browser matches it on Name/Domain/Path alone.
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
- **Webhook sink context threading:** `WebhookSink.Start` runs the delivery worker until its context is cancelled; on cancellation it calls `drain`, which best-effort-ships whatever's still buffered within a short (2s) deadline so a wedged endpoint can't stall process exit. Both `drain` and the normal per-event `post` path detach from the worker's context via `context.WithoutCancel` before making the HTTP call — the `Start` select loop can still pick a buffered event right after cancellation, and a cancelled context would fail that delivery even though the event could still be shipped. Because `WithoutCancel` strips the deadline along with the cancellation, `post` re-applies the caller's own deadline (if any) onto the detached context, so `drain`'s 2s budget survives the trip through `post` instead of being silently discarded.

### Audit Reason field (Phase 2 Foundational)

- **Column:** `audit_events.reason` (nullable TEXT, added by migration `007_audit_reason.sql`)
- **Purpose:** Machine-readable failure reason for operations that need structured fault reporting beyond HTTP status codes (e.g., network capture start/stop operations)
- **Middleware (generic requests):** Always passes empty string `""` as reason, so pre-migration rows have NULL reason
- **Synchronous writes (handler-direct):** Routes that need immediate audit writes before sending the response call `Auditor.WriteSync(ctx, method, path, target, reason, status)` directly, providing the reason
  - Method signature: `WriteSync(ctx context.Context, method, path, target, reason string, status int) error`
  - Extracts actor from context (set by auth middleware)
  - Extracts client IP from context (set by ClientIPFromXFF middleware)
  - Generates RFC3339 timestamp
  - Returns error if DB write fails; error is **non-fatal** to the operation (log it, but don't fail the capture operation)
  - Fan-outs to external sinks (webhook, S3, stdout) with reason included
- **Hash-chain boundary (backward compatibility):** The `reason` field is included in the canonical hash **only when non-empty**. This preserves the hash computation of pre-migration rows (which have NULL/empty reason) bit-for-bit identical. A post-migration row with a non-empty reason hashes differently, providing integrity protection. Rows differing only in reason will hash differently; Verify still walks the chain correctly across both pre- and post-migration rows.

### Outbound safety
- **netguard SSRF guard:** on module registry fetches (HTTP/HTTPS); permissive allowlist (modular, self-hosted registries on loopback OK)
- **mTLS to agent:** client cert + key validate console operations
- **Agent proxy URL construction:** `api/internal/ws/dialer.go`'s ws and http proxy handlers build the upstream agent URL from the namespace and pod name taken off the request path. In both handlers the namespace and pod name are validated as DNS-1123 labels via `isDNS1123Label` first, and a request with an invalid namespace or pod name is rejected with `400 Bad Request` before any URL is built. Only after that validation do the two paths diverge in how they construct the URL: the HTTP proxy path assembles it with `url.URL`, while the WebSocket proxy path concatenates the already-validated host onto a fixed `wss://` scheme and the fixed agent path (`upstream := "wss://" + host + agentPath`). The load-bearing safety property — validation strictly precedes URL construction — holds for both paths regardless of which one then uses string concatenation.

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
