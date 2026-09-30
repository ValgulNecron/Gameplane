# Review: api

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `api/specs.md`; `specs/done_006-install-time-config/spec.md` (FR-011, FR-014); `specs/done_016-user-theme-customization/contracts/user-preferences-api.md`; `docs/security.md`; `docs/oidc.md`; `docs/notifications.md`; `docs/install.md` / `docs/roadmap.md` (Postgres status); `charts/gameplane/values.yaml` (api block); `web/nginx.conf.template` (only to check how API paths are reached)

## Scope reviewed

Read in full:
- `api/cmd/main.go`, `api/cmd/bootstrap.go`
- `api/internal/handlers/`: every non-test file (`audit.go`, `auth_provider_secret.go`, `auth_providers.go`, `capture.go`, `cluster.go`, `cluster_actions.go`, `clusters.go`, `config.go`, `destinations.go`, `events.go`, `lifecycle.go`, `mod_ids.go`, `mod_updates.go`, `module_sources.go`, `module_upload.go`, `modules.go`, `modules_builder.go`, `notifications.go`, `oidc_routes.go`, `ownership.go`, `pod_events.go`, `registry.go`, `registry_secret.go`, `resources.go`, `roles.go`, `secrets_managed.go`, `semver.go`, `shares.go`, `systemlogs.go`, `tunnelcreds.go`, `users.go`, `validate.go`)
- `api/internal/auth/`: `actor.go`, `local.go`, `oidc.go`, `password.go`, `perms.go`, `ratelimit.go`, `registry.go`, `sessions.go`, `testparams.go`
- `api/internal/rbac/rbac.go`, `catalog.go`; `api/internal/scope/scope.go`, `cluster.go`; `api/internal/httperr/httperr.go`
- `api/internal/db/`: `db.go`, `db_postgres.go`, `db_nopostgres.go`, `config.go`, `rbac.go`, `shares.go`, `preferences.go`, and all 11 migrations (`001`–`011`)
- `api/internal/kube/`: `client.go`, `registry.go`, `watch.go`, `loader.go`, `capture.go`, `stdin.go`, `server_template.go`
- `api/internal/audit/audit.go`, `s3.go`
- `api/internal/notify/`: `notify.go`, `events.go`, `sinks.go`, `deliver.go`, `watch.go`; `format.go` first 60 lines only
- `api/internal/ws/`: `dialer.go`, `attach.go`, `podlogs.go`, `actions.go`, `agent_client.go`
- `api/internal/registry/registry.go`, `keys.go`
- `api/internal/telemetry/telemetry.go`
- `api/specs.md`, `api/go.mod` (require block), `api/Dockerfile`, `api/.testcoverage.yml`
- `api/cmd/timeout_test.go` (to see what the timeout tests pin)
- The external `chi/v5@v5.3.2` `middleware/timeout.go` and `middleware/client_ip.go` in the module cache, to confirm what the wired middleware does

Read partly or at grep level only (said plainly):
- `api/internal/registry/`: `factorio.go` and `thunderstore.go` skimmed (catalog fetch and pagination); `curseforge.go`, `github.go`, `hangar.go`, `modrinth.go`, `nexus.go`, `spigot.go`, `steam.go`, `umod.go` checked with grep only (HTTP call sites, body close, response caps, error wrapping, key placement). I did not read their search and version mapping logic.
- `api/internal/steam/` (4 files): read the package header and the exported symbols only, then grepped for importers (there are none; see C-api-13).
- Test files: not read except targeted greps (`rbac_test.go` capture rows, `config_envtest_test.go` role-mapping DELETE cases, `destinations*_test.go` status assertions, `api/cmd/main_test.go` for `isUploadPath`, `test/e2e/api_mods_confinement_e2e_test.go` upload size).
- `go.sum`: not read.

## Method

Per research R12, for every file read: (1) behaviour against `api/specs.md` and the docs that describe it, (2) error handling (`%w` where callers use `errors.Is`/`As`, dropped errors, unclosed bodies), (3) dead or unreachable code, (4) docs drift. Then I cross-checked each route that `main.go` mounts against the RBAC rule table and against the `?cluster=` handling of the handler that serves it. `go build ./...` in `api/` passes. I ran no test or lint suites.

## Observations (no finding)

- RBAC rule ordering: all 8 capture rules (`rbac.go:202-209`) come before `servers:read`/`servers:write` (`:213-214`). `rbac_test.go:193+` pins viewer and operator denial on every capture route, so reordering the rules would fail CI (this contradicts a line in the spec; see C-api-15).
- Every agent-proxy, PTY and pod-log route in `ws/` is wrapped in `rejectRemoteCluster`. `dialer.go` checks namespace and pod as DNS-1123 labels before it builds any URL, as spec invariant 13 says.
- `httperr.WriteCode` never echoes the error text for a status of 500 or above. Registry transport errors go through `sanitizeUpstreamErr`, which strips the Steam key from the query string. Notify strips `*url.Error` before surfacing sink errors.
- Login privacy: `local.go:139-148` pays the argon2 cost on every 401 branch. `/auth/providers` returns only name, kind and label.
- Share tokens: only the SHA-256 is stored, the raw token appears only in the create response, and the audit path redacts it (`audit.go:761-778`).
- Capture handlers call `auditWriteOrFail` before every response write, and a failed audit write returns 500 (FR-006), matching spec line 322.
- The hash chain, checkpoint and head anchors in `audit.go` behave as the spec and `docs/security.md` describe. `Verify` closes its rows before `verifyHead` to avoid the single-connection deadlock.
- `docs/notifications.md` (sinks, events, defaults, test-send route) matches `notify/` and `handlers/notifications.go`.
- Migrations are append-only, `001`–`011`, one transaction each.
- Postgres cannot work as shipped: `001_init.sql` uses `AUTOINCREMENT` and every query uses `?` placeholders. This is already disclosed as work in progress (`docs/roadmap.md:228-234`, `docs/install.md:120`), so it is not filed. The spec's "tracked separately on nightly" claim is filed under C-api-17.
- `kube/client.go:24-25`: the doc comment on `GetServer` ("Returns nil if not found") is false, and `kube/capture.go:118-121` already calls this out. The RBAC fallback checks `err == nil && obj != nil`, so no behaviour depends on it.

## Candidate findings

held candidates: 12 (see OD-019)

### C-api-01: The global 60 s request timeout cuts every long upload and download
- **Location**: `api/cmd/main.go:253` (`requestTimeout(60 * time.Second)`), `api/cmd/main.go:695-700` (`isStreamingRequest`), `api/internal/ws/dialer.go:283-285`, `api/internal/handlers/capture.go:1094`, `api/internal/handlers/audit.go:77,97`
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. `isStreamingRequest` exempts only WebSocket upgrades and `GET /events`. Every other route runs under chi `middleware.Timeout(60s)`, which cancels the request context at 60 s (`chi/v5@v5.3.2/middleware/timeout.go`).
  2. The agent proxy builds its upstream request with `req.Context()` (`dialer.go:283-285`). So do the capture download (`capture.go:1094`) and the audit export stream (`audit.go:77,97`).
  3. Upload a 60 MiB file through `POST /servers/{name}/files/upload` (exempt from the 1 MiB body cap), or download a multi-GiB capture with `GET /servers/{name}:capture-file` (default max size 5 GiB, `main.go:511`), on a link that cannot finish within 60 s.
- **Expected**: these routes stream for as long as the transfer takes, the way `main.go:662-675` says streaming routes must.
- **Actual**: the transfer is cut at 60 s. An upload fails with 504 "agent unreachable". A download that has already sent its 200 header is truncated, and the capture audit row still records 200, because the corrective row only fires on a non-2xx status (`capture.go:1045-1050`). `/servers/{name}/logs/download`, `/files/download`, `/mods/upload` and `/admin/audit/export` behave the same way.

### C-api-02: Mod uploads are capped at 1 MiB, so the 512 MiB cap in dialer.go never applies
- **Location**: `api/cmd/main.go:607-623` (`bodyLimit`, `isUploadPath`), `api/internal/ws/dialer.go:87`
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. `isUploadPath` matches only `strings.HasSuffix(path, "/files/upload")`. The comment above it says the global `MaxBytesReader` "is *authoritative*: once wrapped at 1 MiB, re-wrapping at 64 MiB downstream doesn't raise the ceiling".
  2. `dialer.go:87` mounts `POST /servers/{name}/mods/upload` with `httpProxyLimit(..., 512<<20)`. The comment says the upload "gets its own body cap matching the largest module install policy".
  3. POST a 3 MiB mod `.jar` to `/servers/{name}/mods/upload` (the dashboard does, `web/src/lib/endpoints.ts:172`).
- **Expected**: uploads up to 512 MiB are proxied to the agent.
- **Actual**: the body is already wrapped at 1 MiB, so reading past it fails mid-proxy, `p.http.Do` returns an error, and the client gets 502 "agent unreachable" (`dialer.go:297-300`, `313-322`). The e2e tests that use this route send only tiny archives (`test/e2e/api_mods_confinement_e2e_test.go:136`).

### C-api-03: `err.Error() != "sql: no rows"` never matches, which breaks role-mapping reset
- **Location**: `api/internal/handlers/config.go:179`, `api/internal/handlers/config.go:272`
- **Category**: error-handling
- **Suggested severity**: S3
- **Observation / repro**:
  1. `database/sql.ErrNoRows.Error()` is `"sql: no rows in result set"`. Both sites compare against `"sql: no rows"`, which never matches, and neither uses `errors.Is`.
  2. On an install with no `auth` config row (a fresh install), call `DELETE /admin/config/auth/role-mappings/admin`. `resetRoleMapping` falls into `httperr.Write(w, req, err)`, and `classify` maps `sql.ErrNoRows` to 500 "internal error".
  3. (see held item C-api-03, OD-019)
- **Expected**: the handler doc comment at `config.go:161` says "It is idempotent: returns 200 even if the role had no override".
- **Actual**: the DELETE returns 500 until some PUT has created the row. The tests always PUT first (`config_envtest_test.go:495-512`), so neither path is covered.

### C-api-04: Validation errors are passed to `httperr.Write` and come back as 500 "internal error"
- **Location**: `api/internal/handlers/destinations.go:132`, `:136`, `:177`; `api/internal/handlers/modules.go:176`, `:184`
- **Category**: error-handling
- **Suggested severity**: S4
- **Observation / repro**:
  1. `POST /backup-destinations` with `{"name":"Bad_Name","url":"s3:x","password":"p"}`.
  2. `httperr.Write(w, req, errors.New("name must be a DNS label …"))` goes through `classify`, which has no case for a plain error and falls to `500 "internal error"`, logged at Error level.
  3. The same happens for missing url or password (`:136`), for a clash with a non-Gameplane Secret (`:177`, which should be 409), and for `POST /modules` with an empty source/module or a bad name (`modules.go:176,184`).
- **Expected**: 400 (or 409) with the hand-written message, as spec line 555 describes ("4xx errors: hand-crafted safe messages by handlers").
- **Actual**: 500 "internal error". The tests only assert "non-200" (`destinations_upsert_test.go:47`).

### C-api-05: held (OD-019)

### C-api-06: held (OD-019)

### C-api-07: held (OD-019)

### C-api-08: "Download kubeconfig" and "Add node" hand out the in-cluster ClusterIP address and a kubeadm command
- **Location**: `api/internal/handlers/cluster_actions.go:101-104`, `:207`
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. `main.go:138` builds `restCfg` with `ctrl.GetConfig()`. In-cluster, `Host` is `https://$KUBERNETES_SERVICE_HOST:443`, the `kubernetes` Service ClusterIP (for example `10.43.0.1`).
  2. With `clusterOps.enabled=true`, call `POST /cluster/kubeconfig`. The rendered kubeconfig has `server: https://10.43.0.1:443` (`renderKubeconfig(h.k.Config.Host, …)`).
  3. Call `POST /cluster/nodes:join`. The command is `kubeadm join 10.43.0.1:443 --token … --discovery-token-ca-cert-hash …`.
- **Expected**: `values.yaml:386-390` says Download kubeconfig mints "a short-lived client cert", and the dashboard offers it as a usable kubeconfig. It should point at an address reachable from outside the cluster, and the join command should work on the distribution the project targets (k3s first).
- **Actual**: a workstation outside the pod network cannot reach the ClusterIP, and a node that has not joined yet has no kube-proxy route to it. The command is also kubeadm syntax on k3s installs. Nothing in `docs/` sets a different API server address for these flows.

### C-api-09: Revoking an unknown share-link id returns 500
- **Location**: `api/internal/db/shares.go:311-313`; `api/internal/handlers/shares.go:265-267`
- **Category**: error-handling
- **Suggested severity**: S4
- **Observation / repro**:
  1. As the server owner, call `DELETE /servers/{name}/shares/doesnotexist`.
  2. `RevokeShareLink` returns `fmt.Errorf("share link not found: %w", errors.New("unknown id"))`, which is not a sentinel and not a k8s error.
  3. `httperr.Write` classifies it as 500 "internal error" and logs it at Error level.
- **Expected**: 404.
- **Actual**: 500. A link on another cluster (the `cluster = ?` mismatch) returns 500 the same way.

### C-api-10: 201 responses are sent without `Content-Type: application/json`
- **Location**: `api/internal/handlers/modules.go:203-204`, `module_sources.go:263-264`, `module_upload.go:138-139`, `modules_builder.go:417-418`, `clusters.go:184-185`, `users.go:772-773`, `roles.go:151-152`
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. Each site calls `w.WriteHeader(http.StatusCreated)` and then `writeJSON(w, …)`, and `writeJSON` sets `Content-Type` after that.
  2. net/http snapshots the headers at `WriteHeader`, so the later `Set` has no effect and the body is content-sniffed.
- **Expected**: `Content-Type: application/json`, like every other JSON response (for example `resources.go:205-207`, which sets it before `WriteHeader`).
- **Actual**: `Content-Type: text/plain; charset=utf-8` on the 201 responses of POST /modules, /modules/sources, bundle upload, builder export (install), /clusters, /users/{id}/bindings and /roles.

### C-api-11: held (OD-019)

### C-api-12: Users backfilled by migration 011 get a non-RFC3339 `updatedAt`
- **Location**: `api/internal/db/migrations/011_user_theme_preferences.sql` (column `updated_at … DEFAULT (datetime('now'))` and the backfill `INSERT … SELECT`); `api/internal/db/preferences.go:47-50`
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. Upgrade an install that has users from a release before migration 011. The backfill takes the column default, SQLite `datetime('now')`, which is `2026-09-24 08:00:00`.
  2. `GET /users/me/preferences` (or `/users/me`, or the login response) returns `"updatedAt":"2026-09-24 08:00:00"` until that user's first PUT.
- **Expected**: spec line 279: "`updated_at` TEXT NOT NULL (application-generated RFC3339 UTC in Go)". The contract examples use `"2026-09-19T14:32:00Z"`.
- **Actual**: the SQLite `YYYY-MM-DD HH:MM:SS` format for every backfilled user.

### C-api-13: Dead or unwired code
- **Location**: `api/internal/kube/loader.go:57-72` (`IsKubeNotFound`, `IsKubeAlreadyExists`); `api/internal/rbac/rbac.go:287-293` (`serverNameFromPath`); `api/internal/kube/registry.go:37-43` (`Registry.Default`); `api/internal/auth/oidc.go:103-107` (`NewOIDC`); `api/internal/db/migrations/001_init.sql` (`api_tokens` table); `api/internal/steam/` (whole package); `api/internal/handlers/destinations.go:253-255`
- **Category**: dead-code
- **Suggested severity**: S4
- **Observation / repro**:
  1. Grepping the api module for callers: `IsKubeNotFound(` and `IsKubeAlreadyExists(` have none, tests included. `serverNameFromPath(`, `.Default()` and `NewOIDC(` are called only from `_test.go` files.
  2. No Go code reads or writes `api_tokens`, yet spec line 465 lists it as a live table.
  3. No file in the repo imports `api/internal/steam`. It is the spec 002 Steam display-name resolver (tasks T081+ are still unchecked), so it is work in progress, but it ships unwired, and `api/specs.md`'s directory layout does not list it.
  4. `var _ = scope.ErrForbiddenNamespace` exists only to keep an import alive.
- **Expected**: production code has live callers, or unused schema and planned packages are marked as such in `api/specs.md`.
- **Actual**: as listed. Low risk, but the unwired `steam/` package and the `api_tokens` table are easy to mistake for live features.

### C-api-14: The api/specs.md route list does not match the mounted routes
- **Location**: `api/specs.md:105-146`, `:152`, `:161`; `api/specs.md:93-102` (public routes)
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**: spec lines compared with the actual mounts:
  1. `:124` "`/module-sources` — CRUD": the routes are `/modules/sources[/{name}]` (`modules.go:41-46`).
  2. `:125` "`/registry/{provider}/search`, `/{id}`": the routes are `GET /servers/{name}/mods/registry/{providers,search,projects/{project}/versions,projects/{project}/modpack}` and `POST /servers/{name}/modpack` (`registry.go:43-47`).
  3. `:126-127` "`/mod-updates/{name}` … `/mod-ids/{name}` — PATCH": the routes are `GET /servers/{name}/mods/updates` and `GET/PUT /servers/{name}/mods/ids`.
  4. `:128-129` "`/cluster` … POST: credential-minting", "`/cluster/actions`", and `:146` "`/admin/cluster/{op}`": the routes are `GET /cluster`, `/cluster/info`, `/cluster/stats`, `POST /cluster/nodes:join`, `POST /cluster/kubeconfig`. There is no `/cluster/actions` and no `/admin/cluster/{op}`.
  5. `:132` "`/pod-events` — SSE: pod-level events": the route is `GET /servers/{name}/events`, a JSON snapshot (`pod_events.go:16-27`).
  6. `:138` "`/users/{id}/role-bindings` — PATCH": the routes are `GET/POST /users/{id}/bindings` and `DELETE /users/{id}/bindings/{role}/{namespace}`.
  7. `:141-145`: config is `GET /admin/config` plus `PUT /admin/config/{section}` (not PATCH). Notifications, auth and registries secrets are `PUT/DELETE …/secret` (not PATCH). System logs is `/admin/system-logs/{component}`. Line 244 says "PATCH /admin/config/auth" and line 254 says "PUT /admin/config"; both are the same `PUT /admin/config/auth`.
  8. `:106`, `:109`, `:161` call console, files and WS "cluster-dispatch" / "Multiplexed per `?cluster=`", but every one of those routes 404s a non-local `?cluster=` (`ws/dialer.go:132-140`).
  9. `:93-102` (Public) leaves out `GET /shares/{token}` and `POST /shares/{token}/start` (`shares.go:38-43`), which are unauthenticated.
  10. Not listed at all: `:wake`, `:clone`, `:wipe-data`, `:shares`, `:tunnel-credentials`, `/players/*`, `/actions/run`, `/status`, `/logs/download`, `/admin/audit/verify`, `/admin/audit/export`, `/modules/catalog`, `/modules/builder/*`, `/users/{id}/reset-password`, `/roles/permissions`, `/admin/notifications/sinks/{name}/test`.
- **Expected**: the spec's "External interface / contracts" section lists the real surface.
- **Actual**: as above.

### C-api-15: The capture sections of api/specs.md contradict themselves and the code
- **Location**: `api/specs.md:110`, `:166`, `:199`, `:205`, `:211`, `:226`, `:312`, `:316`, `:322`, `:351`, `:357`, `:366`, `:424`, `:543`; `api/internal/audit/audit.go:692-696`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `:110` says `:capture-enable`/`:capture-disable` are "[PLANNED …] (endpoints stubbed; handlers not yet implemented …)", while `:215-226` says "Fully implemented". The code implements them (`capture.go:321-470`).
  2. `:166` "wires 7 implemented capture endpoints", and `:312` / `:395` describe `DELETE :capture` as a "future Phase 2 task". `MountCapture` mounts 8 routes, including `captureDelete` (`capture.go:67-74`, `850-938`).
  3. `:199` says GET `:capture` does "Audit: `WriteSync()` before response". `:316` says "no handler-side write is performed for this endpoint", and the code does no audit (`capture.go:768-776`).
  4. `:322` says "a failed audit write fails the entire capture operation", but `:543` says "error is **non-fatal** to the operation (log it, but don't fail the capture operation)", and the `WriteSync` doc comment (`audit.go:692-696`) says handlers "must treat audit-write failures as non-fatal". The code is fatal (`capture.go:1151-1157`).
  5. `:205`: 409 only for Pending/Running. The code returns 409 for every phase except Completed, Failed included (`capture.go:1024-1030`). `:211` lists reason "not_running", but the code writes "not_completed" (and also "missing_id", "server_not_found", "error").
  6. `:226`: the disable reasons include "feature_disabled" and "terminating". Disable checks neither, and it writes "already_disabled" / "stop_failed" (`capture.go:425-441`).
  7. `:351`, `:366`, `:424` give rule line numbers 189-196 / 201 / 211. The rules are now at `rbac.go:202-209` and `:214`. `rbac.go:196`'s own comment still says "(line 184)".
  8. `:357` "No CI test currently detects this regression" and `:366` "CI does not currently detect this regression". `rbac_test.go:193+` asserts operator denial on every capture route, and `:358-361` of the same spec says such tests exist.
- **Expected**: one consistent description that matches `capture.go`.
- **Actual**: as above.

### C-api-16: api/specs.md is wrong about sessions, rate limits, audit paging and seeded roles
- **Location**: `api/specs.md:517`, `:416`, `:418`, `:374`, `:492`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `:517` "Sessions: … memory store + DB persistence; expiry at midnight UTC". Sessions live only in the DB and expire 12 h after creation (`sessions.go:19`, `:58-61`).
  2. `:416` invariant 6: "per-IP (burst 10, 5/min) + per-user (burst 6, 3/min) on `/auth/login` + OIDC callback". The OIDC callback has only a per-IP bucket of 10/min with burst 10 (`ratelimit.go:122`, `main.go:278-285`), and no per-user limit.
  3. `:418` invariant 8: "clamps the untrusted `limit` parameter to a maximum of 500 entries". A limit above 500 is replaced with the default of 100, not 500 (`handlers/audit.go:26-28`, `audit.go:830-832`). The cited line numbers (`audit.go` 820–822) are stale.
  4. `:374` "operator: read/write servers, backups, schedules, templates, modules". The seeded operator has only `modules:read` for modules (`003_roles.sql:46-60`; `:57` is the only operator modules row).
  5. `:492` "Seeds `captures:manage` … via `INSERT INTO role_permissions … VALUES ('admin', 'captures:manage')`". `008_captures_rbac.sql` contains only comments and says it deliberately does not insert; spec `:345` says the same.
- **Expected**: the spec matches the code.
- **Actual**: as above.

### C-api-17: api/specs.md is wrong about flags, dependencies, layout and Postgres CI
- **Location**: `api/specs.md:79`, `:168`, `:17`/`:62`, `:64`, `:429-447`, `:29-52`, `:591`; `api/.testcoverage.yml:28`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `:79` and `:168` describe `--capture-*` flags "(feature-enabled, default-retention-seconds, max-retention-seconds, default-max-duration-secs, default-max-size-bytes)". Only `--capture-enabled` and `--capture-default-max-duration` exist. Retention and size come from the environment only (`main.go:505-511`). `:168` "Registered in api/cmd/main.go line 281": it is line 344.
  2. `:17` and `:62` list notification sinks "(Discord, Slack, SMTP, webhook)". The code also has `ntfy` (`config.go:685`, `deliver.go:70-78`).
  3. `:64` says registry providers each implement "Search, Details, Manifest, Download". The interface is `Search`, `Versions` and `ModpackDeps` (`registry.go:121-129`).
  4. `:429-447` Dependencies: internal deps omit `gp-module`, which `go.mod` requires and `modules_builder.go:17-21` imports. Every listed version is stale (chi v5.1.0 vs v5.3.2, go-oidc v3.11.0 vs v3.21.0, pgx v5.5.4 vs v5.11.0, k8s v0.35.0 vs v0.37.0, and so on).
  5. `:29-52` layout omits `internal/steam/` and the module builder.
  6. `:591` and `.testcoverage.yml:28`: Postgres coverage is "tracked separately on nightly". No workflow in `.github/workflows/` mentions postgres or has a `schedule:` trigger.
- **Expected**: the spec matches `main.go`, `go.mod` and CI.
- **Actual**: as above.

## Questions (not findings)

- The web nginx (`web/nginx.conf.template`) sets no `client_max_body_size`, so nginx's 1 MiB default applies to every proxied request. This is a web-chunk question, but it bears on C-api-01 and C-api-02: are uploads over 1 MiB through the dashboard supposed to work today at all?
- `telemetry.Run` (`telemetry.go:72-83`) sends its first report only after one full interval (24 h), and the ticker restarts with every pod. An API pod that restarts more often than daily never reports. Is that intended for "daily usage metrics"?
- `validateAndProtectGameServer` rejects every `spec.env` secret or ConfigMap reference on create, admin included, because the new object has no UID yet (`resources.go:517-523`, `564-586`). So `:clone` of any server that uses a server-owned secret env returns 403. That is secure, but is it the intended UX?
- Q-api-audit-login: held (OD-019)
- Q-api-rbac-last-manager: held (OD-019)
