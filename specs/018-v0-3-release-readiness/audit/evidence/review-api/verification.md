# T045 api chunk: independent verification (opus)

Verifier input: `notes.md` in this directory (opus reviewer), candidates C-api-01 to C-api-17. The 12 held candidates for this component are verified separately, off-git (OD-019).

Method: I checked each candidate against the code. `git diff --stat master -- api/` is empty, so everything under `api/` on this branch is identical to master `13a859ff`. For every candidate I read the cited file and line, then its callers and callees, the chart and nginx config the request passes through (`charts/gameplane/values.yaml`, `web/nginx.conf.template`, `web/Dockerfile`), the tests that pin the behaviour where the notes cite them, `api/specs.md`, `specs/done_006-install-time-config/{spec,plan}.md`, `specs/done_016-user-theme-customization/contracts/user-preferences-api.md`, `specs/002-nuclear-option-ip-pool/tasks.md` (for the `steam` package), and the vendored `chi/v5@v5.3.2/middleware/timeout.go`. I also checked `audit/findings.md` and the other chunks' notes for duplicates. None of the kept items is tracked there already. C-api-02 shares a trigger with `review-agent` C-agent-01, but the two are different defects in different components. Early in the session I ran read-only shell commands (`grep`, `sed`, `git diff --stat`, `go doc`). Partway through, the session's command classifier blocked further shell use, so the rest was checked by reading files only. I did not run `go build`, any test or lint suite, or anything against a cluster, and I changed no repo file other than this one.

Severity follows research R3. I used S3 when a reachable user-facing function or control is degraded, and S4 when the defect is wording, a status code or header, or a mismatch with negligible practical impact.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-api-01 | kept | S3 | Confirmed. `requestTimeout(60s)` (`cmd/main.go:253`, `:676-700`) exempts only WebSocket upgrades and `GET /events`. chi's `Timeout` cancels the request context at 60 s, and the agent proxy (`ws/dialer.go:283-286`), the capture download (`handlers/capture.go:1094`) and the audit export (`handlers/audit.go:77,97`) all build their upstream work on `req.Context()`. Reachable on a default install for **downloads**: the web nginx proxies with `proxy_buffering off` and a 1 h read timeout (`web/nginx.conf.template:129-131`), so the API's 60 s cut is what ends the transfer. The upload half is masked on a default install: `web/nginx.conf.template` sets no `client_max_body_size`, so nginx's 1 MiB default refuses larger bodies before they reach the API. That nginx cap is a separate web-side issue and isn't filed here. The upload half is reachable when the API is exposed directly. Workaround: `kubectl cp`. |
| C-api-02 | kept | S3 | Confirmed, with a wider location. `bodyLimit` wraps every body except `/servers/*/files/upload` in `MaxBytesReader(1 MiB)` (`cmd/main.go:602-623`), so the 512 MiB cap on `/mods/upload` (`ws/dialer.go:87`) and the default 64 MiB cap in `httpProxy` (`dialer.go:243-248`) never take effect. Past 1 MiB the upstream body read fails, `p.http.Do` errors, and the client gets 502 "agent unreachable" (`dialer.go:297-300`). The same 1 MiB cap hits `POST /files/write` (`dialer.go:55`), which is the trigger for agent finding C-agent-01. Most mod jars are larger than 1 MiB. Workaround: install by URL. |
| C-api-03 | kept | S4 | Confirmed. `sql.ErrNoRows.Error()` is `"sql: no rows in result set"` (`go doc database/sql ErrNoRows`), so the string compares at `handlers/config.go:179` and `:272` never match, and no `auth` config row is seeded anywhere (the only writers are `config.go:134,233`, `cmd/bootstrap.go:188` and `audit.go:302`, for other keys). Downgraded from S3: the dashboard renders the Reset button only when an override exists (`web/src/routes/AdminSettings.tsx:1807-1827`), and an override implies the row exists, so the 500 is reachable only by calling the API directly. |
| C-api-04 | kept | S4 | Confirmed. `destinations.go:132,136,177` and `modules.go:176,184` pass plain `errors.New` values to `httperr.Write`. `classify` (`httperr/httperr.go:63-100`) has no case for them, so they become 500 "internal error". The hand-written 400/409 messages never reach the caller. It is a wrong status and a lost message, with no loss of function. |
| C-api-05 | kept | held (OD-019) | held (OD-019) |
| C-api-06 | kept | held (OD-019) | held (OD-019) |
| C-api-07 | kept | held (OD-019) | held (OD-019) |
| C-api-08 | kept | S3 | Confirmed. `restCfg` comes from `ctrl.GetConfig()` (`cmd/main.go:138`), which in-cluster is `https://$KUBERNETES_SERVICE_HOST:443`, the `kubernetes` Service ClusterIP. `addNode` builds `kubeadm join <that host> …` (`cluster_actions.go:101-107`), and `kubeconfig` renders `server:` from the same host (`:207`). Neither address is reachable from a workstation or a node that hasn't joined yet, and nothing in the chart or `docs/` overrides it. The feature is off by default (`values.yaml:391-392`). Workaround: edit the address by hand. |
| C-api-09 | kept | S4 | Confirmed. `RevokeShareLink` returns a plain wrapped error when no row matches (`db/shares.go:311-313`), and `httperr.Write` maps it to 500 (`handlers/shares.go:264-267`). Re-revoking a link that already exists still matches a row and returns 204, so only unknown ids or ids on another cluster hit this path. |
| C-api-10 | kept | S4 | Confirmed. Seven handlers call `w.WriteHeader(http.StatusCreated)` before `writeJSON` sets `Content-Type` (`resources.go:406-409`): `modules.go:203`, `module_sources.go:263`, `module_upload.go:138`, `modules_builder.go:417`, `clusters.go:184`, `users.go:772`, `roles.go:151`. `net/http` snapshots headers at `WriteHeader`, so the body is content-sniffed. The dashboard reads `res.text()` and doesn't care (`web/src/lib/api.ts:94-102`), so only other API clients notice. |
| C-api-11 | kept | held (OD-019) | held (OD-019) |
| C-api-12 | kept | S4 | Confirmed. The migration 011 backfill takes the column default `datetime('now')` (`migrations/011_user_theme_preferences.sql:23,28-31`), and `GetPreferences` returns it verbatim (`db/preferences.go:46-50`). `api/specs.md:279` requires RFC 3339 written by Go, and `migrations/006_share_links.sql:10-15` states the same convention and why. No dashboard code parses `updatedAt` (`web/src/lib/useThemePreferences.ts:109-113` treats it as opaque), so the impact is only the contract. |
| C-api-13 | rejected | n/a | Cleanup and style, not a defect. `IsKubeNotFound`/`IsKubeAlreadyExists` have no callers, and `NewOIDC` has none either (tests included, which corrects the notes). `serverNameFromPath` and `Registry.Default` are used only from tests, and `var _ = scope.ErrForbiddenNamespace` keeps an import alive. None of these changes behaviour. `api/internal/steam/` is tracked, unfinished work (spec 002 tasks T081–T104, still unchecked in `specs/002-nuclear-option-ip-pool/tasks.md:243+`). `api_tokens` really is a table in the schema (`001_init.sql:43-51`), so `specs.md:465` listing it is true. The only doc gap (the package layout leaves out `steam/`) is already C-api-17 item 5. |
| C-api-14 | kept | S4 | Confirmed by comparing `api/specs.md:92-146` with the mounts: `/modules/sources…` (`modules.go:41-46`), `/servers/{name}/mods/registry/…` and `/servers/{name}/modpack` (`registry.go:43-47`), `/servers/{name}/mods/updates` and `/mods/ids` (`mod_updates.go:40`, `mod_ids.go:64-65`), `GET /cluster{,/info,/stats}` (`cluster.go:29-31`) plus the two `cluster_actions.go` POSTs, `GET /servers/{name}/events` (`pod_events.go:27`), `/users/{id}/bindings` (`users.go:40-42`), `PUT/DELETE …/secret` on notifications, auth and registries, `/admin/system-logs/{component}`, and the public `/shares/{token}` routes. There is no `/cluster/actions` or `/admin/cluster/{op}` route. Documentation only. |
| C-api-15 | kept | S4 | Confirmed. `specs.md:110` says enable/disable are planned stubs, but `:215` and the code (`capture.go:67-74`) say they are implemented. `:166`, `:312` and `:395` say 7 routes with DELETE "future", but `MountCapture` mounts 8. `:322` (audit failure is fatal) contradicts `:543` (non-fatal), and the code is fatal (`capture.go:1151-1157`). `:205` and `:211` give "409 for Pending/Running" and reason `not_running`, but the code returns 409 for every phase except Completed, with reason `not_completed` (`capture.go:1024-1030`). `:351`, `:366` and `:424` cite rule lines 189-196/201, but the rules are at `rbac.go:202-209`/`:214`. `:357`/`:366` say no CI test catches reordering, but `rbac_test.go:193+` does. Documentation only. |
| C-api-16 | kept | S4 | Confirmed. `:517`: sessions are DB-only with a 12 h TTL (`sessions.go:19,58-61`). `:416`: the OIDC callback has only a per-IP bucket of 10/min with burst 10 (`ratelimit.go:122`). `:418`: a limit above 500 falls back to 100 (`handlers/audit.go:26-28`, `audit.go:830-832`). `:374`: the seeded operator has only `modules:read` (`003_roles.sql:57`). `:492` contradicts `:345` and `008_captures_rbac.sql`, which deliberately inserts nothing. Documentation only. |
| C-api-17 | kept | S4 | Confirmed. The only `--capture-*` flags are `--capture-enabled` and `--capture-default-max-duration` (`cmd/main.go:505-510`), against `specs.md:79` and `:168`. The notification sinks include `ntfy` (`config.go:675,685`), missing from `specs.md:17` and `:62`. The provider interface is `Search/Versions/ModpackDeps` (`registry/registry.go:121-129`), not what `specs.md:64` says. `go.mod:18,23` requires `gp-module`, which `specs.md:429-447` leaves out. The layout at `specs.md:29-52` has no `steam/`. `.testcoverage.yml:26-28` and `specs.md:591` claim a nightly Postgres job, but no workflow mentions postgres or has a `schedule:` trigger. Documentation only. |

### C-api-01

**Location:** `api/cmd/main.go:253` (`r.Use(requestTimeout(60 * time.Second))`), `api/cmd/main.go:676-700` (`requestTimeout`, `isStreamingRequest`); upstream work bound to the request context at `api/internal/ws/dialer.go:283-286`, `api/internal/handlers/capture.go:1094` and `api/internal/handlers/audit.go:77,97`. Affected routes: `GET /servers/{name}/files/download` (`dialer.go:54`), `GET /servers/{name}/logs/download` (`dialer.go:42`), `GET /servers/{name}:capture-file`, `GET /admin/audit/export`, and (on direct API exposure) `POST /servers/{name}/files/upload`.

**Repro / observation**
1. Read `chi/v5@v5.3.2/middleware/timeout.go`: `Timeout` wraps the request context in `context.WithTimeout` and, if the deadline fired, calls `WriteHeader(504)` after the handler returns.
2. Read `api/cmd/main.go:695-700`: only `Upgrade: websocket` and `GET /events` bypass it.
3. Read `api/internal/ws/dialer.go:283-305`: the upstream request uses `req.Context()`, and the body is streamed with `io.Copy`. When the context is cancelled mid-stream, the copy stops.
4. On a cluster: put a large file in a server's data volume (for example `kubectl exec <pod> -c <game> -- dd if=/dev/zero of=/data/big.bin bs=1M count=2048`), then download it through the dashboard origin at a rate that needs more than 60 s: `curl -b cookies.txt --limit-rate 10M -o out.bin "https://<dashboard>/servers/<name>/files/download?path=big.bin"`.
5. `curl` ends at about 60 s with "transfer closed with … bytes remaining", and `out.bin` is smaller than 2 GiB. For `:capture-file`, the audit trail keeps a single row with status 200. The corrective row at `capture.go:1044-1050` fires only when the upstream status is not 2xx, and here the upstream answered 200.

**Expected:** download and upload proxies stream for as long as the transfer takes, the same way the WebSocket and SSE routes are exempt (`main.go:662-675` gives the rationale). Other routes keep the 60 s DoS bound.

**Actual:** every non-WebSocket, non-SSE transfer is cut at 60 s. A download that has already sent 200 is truncated silently. An upload fails with 504 "agent unreachable" when the API is reached directly. On a default install, the web nginx's default 1 MiB `client_max_body_size` refuses larger uploads first.

### C-api-02

**Location:** `api/cmd/main.go:254` (`bodyLimit(1 << 20)`), `api/cmd/main.go:602-623` (`bodyLimit`, `isUploadPath`); `api/internal/ws/dialer.go:87` (`/mods/upload` with `httpProxyLimit(..., 512<<20)`), `api/internal/ws/dialer.go:243-248` (default 64 MiB in `httpProxy`, also dead for `/files/write` at `dialer.go:55`).

**Repro / observation**
1. Read `api/cmd/main.go:620-623`: `isUploadPath` exempts only paths that start with `/servers/` and end with `/files/upload`.
2. Read `api/cmd/main.go:602-606`: the comment says that once a body is wrapped at 1 MiB, re-wrapping it higher downstream doesn't raise the ceiling. `dialer.go:285` re-wraps at `maxBody`, which therefore cannot exceed 1 MiB for `/mods/upload` or `/files/write`.
3. On a cluster, reach the API without the web nginx in the path (a headless install with `web.enabled=false`, or `kubectl port-forward` to the API Service). Send `POST /servers/<name>/mods/upload` with a 3 MiB mod file, in the same multipart form the dashboard's Mods tab sends (`web/src/lib/endpoints.ts:172`).
4. The response is 502 "agent unreachable", and the API log has `agent proxy upstream error … http: request body too large`.
5. The same happens for a `POST /servers/<name>/files/write` body over 1 MiB, which is how agent finding C-agent-01 gets truncated writes.

**Expected:** `/mods/upload` accepts up to 512 MiB, as its mount comment says, and `/files/write` accepts up to the 64 MiB `httpProxy` default. The agent enforces the module's own limit.

**Actual:** both are capped at 1 MiB by the global body limit, and the caller gets a 502 that blames the agent.

### C-api-03

**Location:** `api/internal/handlers/config.go:179` (`resetRoleMapping`), `api/internal/handlers/config.go:272` (`detectAuthAuditEvents`).

**Repro / observation**
1. Read `config.go:176-182` and `:269-275`: both compare `err.Error() != "sql: no rows"`. `database/sql.ErrNoRows.Error()` is `"sql: no rows in result set"`, so a missing row is treated as a query error.
2. Confirm that nothing seeds the `auth` row: the only `INSERT INTO config` statements are `config.go:134`, `:233`, `cmd/bootstrap.go:188` and `audit/audit.go:302`. On a fresh install the row doesn't exist until an admin saves the auth section.
3. On such an install, call `DELETE /admin/config/auth/role-mappings/admin` directly. `httperr.Write` classifies `sql.ErrNoRows` as 500 "internal error".
4. (see held item C-api-03, OD-019)

**Expected:** `config.go:161` says the reset is idempotent and returns 200 even when there is no override.

**Actual:** a direct DELETE returns 500 until the row exists. Use `errors.Is(err, sql.ErrNoRows)` at both sites.

### C-api-04

**Location:** `api/internal/handlers/destinations.go:132`, `:136`, `:177`; `api/internal/handlers/modules.go:176`, `:184`.

**Repro / observation**
1. `POST /backup-destinations` with `{"name":"Bad_Name","url":"s3:x","password":"p"}`.
2. `httperr.Write(w, req, errors.New("name must be a DNS label …"))` reaches `classify` (`httperr/httperr.go:63-100`), which has no case for a plain error, so the response is 500 "internal error" and the API logs the error at Error level.
3. The same happens with an empty `url` or `password` (`:136`), for a name clash with a Secret that isn't a Gameplane destination (`:177`, which should be 409), and for `POST /modules` with an empty `source`/`module` or a bad name (`modules.go:176,184`).

**Expected:** 400 (409 for the clash) carrying the hand-written message, through `httperr.WriteCode`, as `api/specs.md:555` describes for 4xx errors.

**Actual:** 500 "internal error", and the caller never sees why the input was rejected.

### C-api-05: held (OD-019)

### C-api-06: held (OD-019)

### C-api-07: held (OD-019)

### C-api-08

**Location:** `api/internal/handlers/cluster_actions.go:101-107` (`addNode` endpoint and `kubeadm join` command), `api/internal/handlers/cluster_actions.go:207` (`renderKubeconfig(h.k.Config.Host, …)`); `api/cmd/main.go:138` (`ctrl.GetConfig()`).

**Repro / observation**
1. Enable `clusterOps.enabled=true` and open the Cluster page as an admin.
2. Click Download kubeconfig (`POST /cluster/kubeconfig`). The file's `server:` is `https://<ClusterIP of the kubernetes Service>:443`, for example `https://10.43.0.1:443` on k3s.
3. Run `kubectl --kubeconfig gameplane-kubeconfig.yaml get pods` from a workstation outside the pod network. It times out.
4. Click Add node (`POST /cluster/nodes:join`). The command is `kubeadm join 10.43.0.1:443 --token … --discovery-token-ca-cert-hash …`. A machine that hasn't joined yet has no route to a ClusterIP, and a k3s node joins with `k3s agent --server https://<server>:6443 --token …`, not `kubeadm`.

**Expected:** the kubeconfig and the join command point at an API server address that is reachable from outside the cluster (configurable, since the pod can't discover it), and the join command matches the distribution (k3s is the primary target).

**Actual:** both use the in-cluster Service address, so neither works from where the user would run it. `values.yaml:386-390` and `docs/install.md:164` describe these as working features.

### C-api-09

**Location:** `api/internal/db/shares.go:311-313`; `api/internal/handlers/shares.go:264-267`.

**Repro / observation**
1. As the owner of server `<name>`, call `DELETE /servers/<name>/shares/doesnotexist`.
2. `RevokeShareLink` returns `fmt.Errorf("share link not found: %w", errors.New("unknown id"))`, which is neither a sentinel nor a Kubernetes error.
3. `httperr.Write` classifies it as 500 "internal error" and logs it at Error level.

**Expected:** 404.

**Actual:** 500. A link id that belongs to another cluster (the `cluster = ?` mismatch) returns 500 the same way.

### C-api-10

**Location:** `api/internal/handlers/modules.go:203-204`, `module_sources.go:263-264`, `module_upload.go:138-139`, `modules_builder.go:417-418`, `clusters.go:184-185`, `users.go:772-773`, `roles.go:151-152`; `writeJSON` at `resources.go:406-409`.

**Repro / observation**
1. `curl -si -b cookies.txt -H 'X-Gameplane-CSRF: <token>' -H 'Content-Type: application/json' -X POST https://<dashboard>/roles -d '{"name":"t1","permissions":["servers:read"]}'`.
2. The response is `201` with `Content-Type: text/plain; charset=utf-8`, because `WriteHeader` ran before `writeJSON` set the header, and `net/http` then sniffed the body.
3. Compare `resources.go:205-207`, which sets `Content-Type` before `WriteHeader(201)` and returns `application/json`.

**Expected:** `Content-Type: application/json` on every JSON response.

**Actual:** `text/plain; charset=utf-8` on the 201 responses of the seven routes listed.

### C-api-11: held (OD-019)

### C-api-12

**Location:** `api/internal/db/migrations/011_user_theme_preferences.sql:23` (`updated_at … DEFAULT (datetime('now'))`) and `:28-31` (backfill); `api/internal/db/preferences.go:46-50`.

**Repro / observation**
1. Upgrade an install that has users from a release before migration 011. The backfill `INSERT … SELECT` leaves `updated_at` to the column default, which is SQLite's `YYYY-MM-DD HH:MM:SS`.
2. Before that user's first theme change, `GET /users/me/preferences` (also embedded in `/users/me` and the login response) returns `"updatedAt":"2026-09-24 08:00:00"`.

**Expected:** `api/specs.md:279`: "`updated_at` TEXT NOT NULL (application-generated RFC3339 UTC in Go)". The contract examples use `2026-09-19T14:32:00Z`, and `migrations/006_share_links.sql:10-15` explains why SQLite's format is avoided.

**Actual:** every backfilled user gets the SQLite format until their first save.

### C-api-14

**Location:** `api/specs.md:92-146` (route lists), `:152` and `:161` (WebSocket bridge).

**Repro / observation** (compare the spec lines with the mounts):
1. `:124` `/module-sources`: the real routes are `/modules/sources[/{name}]` (`handlers/modules.go:41-46`).
2. `:125` `/registry/{provider}/search`: the real routes are `GET /servers/{name}/mods/registry/{providers,search,projects/{project}/versions,projects/{project}/modpack}` and `POST /servers/{name}/modpack` (`registry.go:43-47`).
3. `:126-127`: the real routes are `GET /servers/{name}/mods/updates` and `GET/PUT /servers/{name}/mods/ids`.
4. `:128-129` and `:146`: the real routes are `GET /cluster`, `/cluster/info`, `/cluster/stats` (`cluster.go:29-31`), `POST /cluster/nodes:join` and `POST /cluster/kubeconfig`. There is no `/cluster/actions` and no `/admin/cluster/{op}`.
5. `:132` `/pod-events` SSE: the real route is `GET /servers/{name}/events`, a JSON snapshot (`pod_events.go:27`).
6. `:138` `/users/{id}/role-bindings` PATCH: the real routes are `GET/POST /users/{id}/bindings` and `DELETE /users/{id}/bindings/{role}/{namespace}` (`users.go:40-42`).
7. `:141-145`: config is `PUT /admin/config/{section}`, not PATCH. The secret routes are `PUT/DELETE /admin/{notifications/sinks/{name},auth/providers/{name},registries/{provider}}/secret`, and system logs is `/admin/system-logs/{component}`.
8. `:106`, `:109` and `:161` describe console, files and WS routes as cluster-dispatch, but every one of them 404s a non-local `?cluster=` (`ws/dialer.go:132-140`).
9. `:92-101` (Public) leaves out `GET /shares/{token}` and `POST /shares/{token}/start`.

**Expected:** the "External interface / contracts" section lists the routes the API actually serves.

**Actual:** as listed above.

### C-api-15

**Location:** `api/specs.md:110`, `:166`, `:199`, `:205`, `:211`, `:226`, `:312`, `:316`, `:322`, `:351`, `:357`, `:366`, `:395`, `:424`, `:543`; `api/internal/audit/audit.go` `WriteSync` doc comment.

**Repro / observation**
1. `:110` calls `:capture-enable`/`:capture-disable` planned stubs, while `:215` calls them fully implemented. `capture.go:67-68` mounts them.
2. `:166` says 7 endpoints, and `:312`/`:395` call `DELETE :capture` a future task. `capture.go:74` mounts it.
3. `:199` says GET `:capture` is audited, but `:316` says it isn't. The code doesn't audit it.
4. `:322` says a failed audit write fails the operation, but `:543` says it is non-fatal. The code is fatal (`capture.go:1151-1157`).
5. `:205` and `:211` say 409 only for Pending/Running, with reason `not_running`. The code returns 409 for every phase except Completed, with reason `not_completed` (`capture.go:1024-1030`).
6. `:226` lists disable reasons `feature_disabled`/`terminating`. The code checks neither.
7. `:351`, `:366` and `:424` cite rule lines 189-196/201. The rules are at `rbac.go:202-209` and `:214`, and `rbac.go:196` still says "(line 184)".
8. `:357`/`:366` say no CI test detects rule reordering, but `rbac_test.go:193+` asserts operator denial on every capture route, and `:358` of the same spec says so.

**Expected:** one consistent description that matches `capture.go` and `rbac.go`.

**Actual:** as listed above.

### C-api-16

**Location:** `api/specs.md:374`, `:416`, `:418`, `:492`, `:517`.

**Repro / observation**
1. `:517`: "memory store + DB persistence; expiry at midnight UTC". Sessions are DB rows with a 12 h TTL (`auth/sessions.go:19`, `:58-61`).
2. `:416`: "per-IP … + per-user … on `/auth/login` + OIDC callback". The callback has only `OIDCCallbackLimiter`, per IP, 10/min with burst 10 (`ratelimit.go:122`).
3. `:418`: the limit is clamped to 500. A limit above 500 is replaced by the default of 100 (`handlers/audit.go:26-28`, `audit/audit.go:830-832`), and the cited line numbers are stale.
4. `:374`: the operator gets "read/write … modules". `003_roles.sql:57` grants only `modules:read`.
5. `:492`: "Seeds `captures:manage` … via `INSERT …`". `008_captures_rbac.sql` inserts nothing on purpose, and `specs.md:345` says so.

**Expected:** the spec matches the code.

**Actual:** as listed above.

### C-api-17

**Location:** `api/specs.md:17`, `:29-52`, `:62`, `:64`, `:79`, `:168`, `:429-447`, `:591`; `api/.testcoverage.yml:26-28`.

**Repro / observation**
1. `:79`/`:168` list five `--capture-*` flags. `cmd/main.go:505-510` defines `--capture-enabled` and `--capture-default-max-duration` only. Retention and size come from the environment. `:168`'s "line 281" is now `main.go:344`.
2. `:17`/`:62` list Discord, Slack, SMTP and webhook. `ntfy` is also accepted (`handlers/config.go:675,685`).
3. `:64` gives the provider methods as "Search, Details, Manifest, Download". The interface is `Search`, `Versions` and `ModpackDeps` (`registry/registry.go:121-129`).
4. `:429-447` leaves out `gp-module` (`go.mod:18,23`), and the versions it lists are stale.
5. `:29-52` (layout) leaves out `internal/steam/` and the module builder.
6. `:591` and `.testcoverage.yml:26-28` say Postgres coverage runs on a nightly job. No file in `.github/workflows/` mentions postgres or has a `schedule:` trigger.

**Expected:** the spec matches `main.go`, `go.mod` and CI.

**Actual:** as listed above.
