# T045 api chunk: independent verification (opus)

Held under OD-019, off-git. This file covers the 12 candidates in `held/review-api.md`. Its wording is defensive: each item names the control, where it lives, the correct behaviour, and how a maintainer confirms that the control holds. It contains no misuse walkthroughs.

Method: I checked each candidate against the code. `git diff --stat master -- api/` is empty, so `api/` on this branch is identical to master `13a859ff`. I read every cited location and its callers and callees: `api/cmd/main.go` (router, middleware order, flag defaults, mounts), `api/cmd/bootstrap.go`, `api/internal/rbac/rbac.go`, `api/internal/handlers/{registry,mod_ids,resources,events,shares,users,capture,module_upload,clusters,config}.go`, `api/internal/db/shares.go` and migrations `001`, `006`, `011`, `api/internal/auth/{oidc,registry,ratelimit,sessions}.go`, `api/internal/audit/audit.go`, `api/internal/kube/{client,watch}.go`, `api/internal/registry/registry.go`, the vendored `chi/v5@v5.3.2/middleware/client_ip.go`, `docs/security.md`, `api/specs.md`, `charts/gameplane/values.yaml`, `web/nginx.conf.template` and `web/src/routes/AdminSettings.tsx`. I checked `audit/findings.md` and the other held files for duplicates. None of these items is tracked. The only overlap is a *question* in `held/review-charts-gameplane.md:62` about the same trusted-proxy default as H-api-02, and it isn't filed as a candidate. Partway through the session, shell commands were blocked by the session's command classifier, so most of this pass was done by reading files. I ran no build, test, lint suite or live request, and I changed no repo file other than this one and its public sibling.

Severity follows research R3. H-api-01 is raised to S1 because it breaks the cluster authorization boundary. That matches the S1 given to the fixed cross-cluster share-link finding F-014. H-api-08 is raised from S4 to S3 because it is a behavioural session-revocation gap, not wording.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| H-api-01 | kept | S1 | Confirmed. `MountRegistry` is mounted with the bare home-cluster client (`cmd/main.go:331`), and none of its five handlers applies `rejectRemoteCluster` (`handlers/registry.go:41-48`, `:66-207`). The sibling handlers that hold the same bare client do apply it (`mod_ids.go:73,93`, and `resources.go:90-108` names the pattern). `rbac.Middleware` evaluates namespaced permissions, including the owner/collaborator fallback, against the `?cluster=` cluster (`rbac.go:97-139`). So a grant that is valid only on a registered remote cluster is accepted for a handler that reads and writes the home cluster. `installModpack` then updates the home cluster's GameServer (`registry.go:202`). This affects every install with a registered remote cluster. Raised from S2: it is a security-boundary break. |
| H-api-02 | kept | S3 | Confirmed. The wired `ClientIPFromXFF` never consults `RemoteAddr` (`client_ip.go:92-117`). `docs/security.md:72-78` promises that XFF is honoured only from trusted peers and ignored on direct exposure. The default trust list covers every RFC 1918 range (`cmd/main.go:522-523`, `values.yaml:125`), so on a LAN install the client's own address is "trusted". The per-IP limiters and the audit `ip` column (`ratelimit.go:85-97`, `audit.go:633-645`) then either collapse onto the proxy pod address or take a value the client can influence, depending on the path in front of the API. The per-username limiter still applies, so S3. |
| H-api-03 | kept | S3 | Confirmed. First login resolves the role from the merged policy (`oidc.go:447-454`), but `syncRole` is keyed on the Helm base policy (`oidc.go:469`). The Helm base has `RoleMappings == nil` whenever no `--oidc-role-mapping-*` flag is set (`cmd/main.go:169-176`). The override editor is shown to every `config:manage` holder (`AdminSettings.tsx:413-420`), and `api/specs.md:243` names override-only mapping as a supported configuration. FR-011 requires re-evaluation on every login, and `oidc.go:612-613` describes the intended behaviour. Workaround: an admin demotes the user by hand. |
| H-api-04 | kept | S3 | Confirmed. `/events` requires only `servers:read` (`rbac.go:243`), and the handler watches every entry of `kube.GVRs`: servers, templates (cluster-wide), backups, schedules and restores (`kube/client.go:34-40`, `events.go:63-110`). A custom role with `servers:read` but without `backups:read`, `schedules:read` or `templates:read` receives those objects in full. The exposure is metadata. It stays inside namespaces the caller may already read servers in, plus templates cluster-wide. |
| H-api-05 | kept | S4 | Confirmed. The ownership check is on `{name}` (`handlers/shares.go:251-258`), but the revoke SQL matches only `id` and `cluster` (`db/shares.go:289-300`). Practical impact is small: it needs a link id (96 random bits) that only appears in owner-only listings, and the outcome is revoking a link, not gaining access. |
| H-api-06 | kept | S3 | Confirmed. User delete removes only the `users` row and the bindings (`handlers/users.go:464-471`). The declared `ON DELETE CASCADE` on `share_links.created_by` (`migrations/006_share_links.sql:20`) never fires on the shipped SQLite DSN, and `LookupShareLink` doesn't check the creator (`db/shares.go:114-197`). On Postgres the same delete removes the links, so the two drivers disagree. Workaround: the server's owner can still see and revoke the links in the Share Links list, because the list is per server, not per creator. |
| H-api-07 | kept | S3 | Confirmed. `enableLocalLogin` decodes the `auth` row into a struct that has only `providers` and writes it back (`cmd/bootstrap.go:165-195`), so `helmOverride` and any other top-level key are dropped. `docs/security.md:58-59` says the command preserves everything else. Dropping the override silently brings back the Helm-seeded role mappings on the next OIDC logins, and nothing is audited. |
| H-api-08 | kept | S3 | Confirmed. The break-glass `--force` path updates `pw_hash` and the role (`cmd/bootstrap.go:114-124`) but leaves `sessions` alone. The dashboard reset evicts sessions and says why (`handlers/users.go:651-658`). Raised from S4: this is a behavioural gap in a credential-rotation path, not wording. Workaround: rotate through the dashboard reset. |
| H-api-09 | kept | S3 | Confirmed. `captureDownload` resolves the GameServer and the NetworkCapture on the `?cluster=` cluster (`capture.go:943-1030`), then proxies to a `*.svc.cluster.local` name that resolves in the API's home cluster, using home-cluster mTLS material (`capture.go:1065-1070`, `:1086-1094`). On a remote cluster the download therefore can't reach the right sidecar. Kept at S3 rather than S1: reaching another cluster's capture needs `captures:manage` (admin-only) plus the capture's random id. The main effect is that remote-cluster downloads are broken. |
| H-api-10 | kept | S4 | Confirmed. `api/specs.md:430` and `:548` say registry fetches go through netguard, but `registry.NewSet` uses a plain `&http.Client{Timeout: 15s}` (`registry/registry.go:208-209`), and the package doc (`:12-15`) explains why: every host is a fixed provider hostname. The two statements disagree, but no exposure was shown. |
| H-api-11 | kept | S4 | Confirmed. The comment at `module_upload.go:290-292` assumes the caller's size cap bounds extraction. That cap applies to the compressed body (`:78-87`). The extractor caps each member (`:306-312`) and the number of distinct paths (`:303`), but not the running total. The route needs `modules:manage` (admin tier), which limits who can trigger it, so this is hardening. |
| H-api-12 | kept | S4 | Confirmed. `DeleteFunc` returns early when `obj` isn't `*unstructured.Unstructured` (`kube/watch.go:57-61`), so a `cache.DeletedFinalStateUnknown` tombstone never reaches `reg.Remove`. `DELETE /clusters/{name}` relies on the watcher to evict the client (`handlers/clusters.go:192-236` never calls `reg.Remove`). It needs the informer to miss the delete event, which is rare. An API restart clears it. |

### H-api-01

**Location:** `api/internal/handlers/registry.go:41-48` (`MountRegistry`), handlers at `:66-207` (write at `:202`); mount at `api/cmd/main.go:331`. Reference guard: `api/internal/handlers/resources.go:90-108` (`rejectRemoteCluster`), applied at `mod_ids.go:73,93` and in `ws/dialer.go:114-140`. Authorization context: `api/internal/rbac/rbac.go:97-139`.

**Control:** handlers that hold the bare home-cluster `*kube.Client` must refuse a non-local `?cluster=` selector, because `rbac.Middleware` authorizes against the cluster the selector names. The guard keeps a permission on one cluster from being used against a same-named object on another.

**Repro / observation** (by reading master `13a859ff`):
1. `api/cmd/main.go:331` mounts `handlers.MountRegistry(p, k8s, regSet)` with the home client `k8s`, not the registry `reg`.
2. `api/internal/handlers/registry.go:66-207`: `providers`, `search`, `versions`, `modpackDeps` and `installModpack` start with `resolveNS` and never call `rejectRemoteCluster`. By contrast, `mod_ids.go:73` and `:93` call it first.
3. `api/internal/rbac/rbac.go:100-107` resolves `cl` from `?cluster=`, and `:109-139` evaluates the permission and the owner/collaborator fallback against `cl`.

**Expected:** every `MountRegistry` route answers 404 for a non-local `?cluster=` (the same guard as the sibling mod routes), or dispatches through the registry to the named cluster.

**Actual:** no guard on any of the five routes.

**How a maintainer confirms the control holds:** add a handler test that calls each `MountRegistry` route with `?cluster=<registered-non-local>` and asserts 404 with no call to the Kubernetes client. Add a structural check, for example a table test over the `main.go` mounts, asserting that every handler constructed with a bare `*kube.Client` under a namespaced RBAC rule is wrapped by `rejectRemoteCluster`. After the fix, the multicluster e2e bucket should show that a user bound only to a remote cluster cannot change a home-cluster GameServer through `/servers/{name}/modpack`.

### H-api-02

**Location:** `api/cmd/main.go:251` (`middleware.ClientIPFromXFF(cfg.trustedProxies...)`), `api/cmd/main.go:517-530` (default trusted list); `charts/gameplane/values.yaml:125`; consumers `api/internal/auth/ratelimit.go:80-105` and `api/internal/audit/audit.go:630-645`; `chi/v5@v5.3.2/middleware/client_ip.go:92-117`. Promise: `docs/security.md:63-81`.

**Control:** client-IP extraction feeds the per-IP limiters (`LoginLimiter`, `OIDCCallbackLimiter`, `ShareLimiter`, `MutationLimiter`) and the audit `ip` column. The documented rule is that `X-Forwarded-For` is honoured only when the TCP peer is a trusted proxy, and ignored otherwise.

**Repro / observation** (by reading master `13a859ff`):
1. `client_ip.go:92-117`: `ClientIPFromXFF` walks the header right to left, skips entries inside the trusted prefixes, and never reads `RemoteAddr`. Its own doc comment (`:140-142` for the sibling variant) warns that the network layer must guarantee only proxies can reach the server.
2. `cmd/main.go:522-523`: the default trusted list is loopback, all of RFC 1918, link-local, ULA and `fe80::/10`.
3. `ratelimit.go:85-97` and `audit.go:633-645`: when no client IP is set, both fall back to `RemoteAddr`, which behind the web nginx is the web pod's address.
4. So the result depends on the path in front of the API. A client whose real address is inside the trusted ranges (the usual LAN or homelab case) either has no extracted identity, and shares one bucket and one audit address with every such client, or has an identity taken from an entry the trusted hops did not add.

**Expected:** the behaviour `docs/security.md:72-78` describes. The TCP peer is checked against the trusted set before any XFF entry is used. A client on a private network keeps its own identity when the proxy in front of the API adds it.

**Actual:** the peer is not checked. Per-IP limiting and the audit address are only as reliable as every hop in front of the API, and they collapse for private-range clients. The per-username login limiter (`ratelimit.go:117`) is unaffected.

**How a maintainer confirms the control holds:** unit tests on the middleware chain. (a) `RemoteAddr` outside the trusted set plus an XFF header: the recorded client IP is the `RemoteAddr` host. (b) `RemoteAddr` is a trusted proxy and XFF ends in a private-range client address: the recorded IP is that client address. (c) Two such clients get different limiter buckets. Update `docs/security.md:63-81` to match the fixed behaviour, and consider a narrower chart default than all of RFC 1918.

### H-api-03

**Location:** `api/internal/auth/oidc.go:445-454` (effective policy for first-login resolution), `api/internal/auth/oidc.go:469` (`syncRole := o.policy != nil && o.policy.RoleMappings != nil`), doc comment `api/internal/auth/oidc.go:612-613`; `api/cmd/main.go:165-182` (the base policy has nil `RoleMappings` without mapping flags); `api/internal/handlers/config.go:659-664` (accepts `helmOverride`).

**Control:** OIDC role re-evaluation. On every login the user's role is recomputed from the current mappings, so removing someone from a mapped IdP group removes the role (FR-011 in `specs/done_006-install-time-config/spec.md`).

**Repro / observation** (by reading master `13a859ff`):
1. With `--oidc-issuer` set and no `--oidc-role-mapping-*` flags, `main.go:169-176` leaves `RoleMappings` nil in the Helm base policy.
2. `oidc.go:447-452` merges `helmOverride` into `resolvePolicy` for role computation, but `oidc.go:469` decides whether to re-sync an existing user from `o.policy` (the base). That is false here.
3. `resolveOrLinkUser` therefore returns the stored role unchanged for existing users (`oidc.go:525-543`), whatever the current group membership and overrides are.

**Expected:** re-evaluation runs whenever the effective, post-merge policy has role mappings, as `oidc.go:612-613` says. It remains subject only to the last-user-manager guard.

**Actual:** re-evaluation is keyed on the Helm-seeded mappings only. With override-only mappings, a role assigned at first login is never re-evaluated.

**How a maintainer confirms the control holds:** a unit test with a Helm policy whose `RoleMappings` is nil and an override that maps group G to admin. First login in G gives admin. A second login without G must change the role (and emit the FR-014 audit event), except when the last-user-manager guard applies.

### H-api-04

**Location:** `api/internal/handlers/events.go:63-110`; `api/internal/rbac/rbac.go:242-243`; `api/internal/kube/client.go:34-40`; catalog `api/internal/rbac/catalog.go:30-42`.

**Control:** per-resource read permissions (`backups:read`, `schedules:read`, `templates:read`) gate the REST reads of those kinds (`rbac.go:215-221`). The SSE stream should honour the same split.

**Repro / observation** (by reading master `13a859ff`):
1. `rbac.go:243` gates `GET /events` with `servers:read` only.
2. `events.go:63-110` opens a watch on every entry of `kube.GVRs` (servers, templates, backups, schedules, restores; templates cluster-wide) and streams each full object with its `kind`.
3. The handler makes no per-kind permission check.

**Expected:** the stream includes only the kinds the caller may read in the resolved namespace, and templates only for callers who hold `templates:read`.

**Actual:** every kind is sent to any holder of `servers:read`. Built-in roles aren't affected in practice, because viewer and operator hold all the reads. Custom roles are.

**How a maintainer confirms the control holds:** an SSE handler test with a custom role that holds only `servers:read` asserts that no event with a `kind` other than `servers` arrives. Include a namespace-scoped binding case so the cluster-wide template watch is covered.

### H-api-05

**Location:** `api/internal/handlers/shares.go:232-271` (ownership check `:251-258`, revoke `:263-267`); `api/internal/db/shares.go:289-300`.

**Control:** share-link management is owner-only per server ("only the server owner can manage shares", `MountShareLinks`).

**Repro / observation** (by reading master `13a859ff`):
1. The handler proves ownership of `{name}` in the resolved namespace.
2. `RevokeShareLink(ctx, cluster, id)` runs `UPDATE share_links SET revoked_at = ? WHERE id = ? AND cluster = ?`. The path's namespace and server name are not part of the match.

**Expected:** revocation matches `cluster`, `namespace`, `server_name` and `id`. A mismatch affects no row and returns 404.

**Actual:** only `id` and `cluster` are matched. Impact is small: ids are random and appear only in owner-only listings, and the effect is a revocation.

**How a maintainer confirms the control holds:** a store test where revoking with a mismatched namespace or server name returns a not-found sentinel and leaves the row untouched, and a handler test that expects 404 in that case. That test also needs C-api-09's 404 mapping.

### H-api-06

**Location:** `api/internal/handlers/users.go:438-473`; `api/internal/db/shares.go:114-197`; `api/internal/db/migrations/006_share_links.sql:20`; `api/specs.md:510` (SQLite runs with foreign keys off).

**Control:** revoking a user's access when the user is deleted. The schema declares that share links cascade with their creator.

**Repro / observation** (by reading master `13a859ff`):
1. `006_share_links.sql:20`: `created_by INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE`.
2. The shipped DSN (`values.yaml:129`) doesn't enable SQLite foreign keys, so the cascade never runs there. `users.go:464-471` deletes only the `users` row and the bindings.
3. `LookupShareLink` checks revocation and expiry only (`db/shares.go:185-194`), not whether the creator still exists.

**Expected:** deleting a user revokes or deletes the share links that user created, on both drivers, the same outcome as the declared cascade.

**Actual:** on SQLite the links stay valid until they expire, and a link created with "never expires" stays valid indefinitely. The server's owner can still revoke them from the server's Share Links list.

**How a maintainer confirms the control holds:** a handler test that creates a link as user U, deletes U, and asserts that the token now gets the uniform not-found response from `GET /shares/{token}`.

### H-api-07

**Location:** `api/cmd/bootstrap.go:153-199` (`enableLocalLogin`; the struct at `:165-167`, the write at `:183-195`). Promise: `docs/security.md:58-59`.

**Control:** the break-glass `--enable-local-login` re-enables the local provider and, per the docs, preserves everything else in the `auth` config row. That includes the admin's role-mapping overrides.

**Repro / observation** (by reading master `13a859ff`):
1. `bootstrap.go:165-170` decodes the row into `struct{ Providers []map[string]any }`.
2. `:183-195` marshals that struct back and upserts it. Top-level keys other than `providers`, notably `helmOverride`, are not carried over.
3. No audit row is written for this change.

**Expected:** the command changes only the local provider's `enabled` flag and leaves every other key byte-for-byte unchanged.

**Actual:** `helmOverride` is erased. The Helm-seeded role mappings then apply again on the next OIDC logins, which can undo a deliberate restriction.

**How a maintainer confirms the control holds:** a bootstrap test that seeds an `auth` row with `providers` and `helmOverride.roleMappings`, runs `enableLocalLogin`, and asserts that the override is unchanged and the local provider is enabled. Decoding into `map[string]any` at the top level is one robust way to get there.

### H-api-08

**Location:** `api/cmd/bootstrap.go:111-126` (`--force` path); compare `api/internal/handlers/users.go:651-658` (`DeleteForUser` on dashboard reset).

**Control:** session revocation on credential rotation. The dashboard reset evicts all of the target's sessions so the new credential actually takes effect.

**Repro / observation** (by reading master `13a859ff`):
1. `bootstrap.go:114-124` updates `pw_hash`, `role` and profile fields, and rebinds admin.
2. Nothing deletes from `sessions` for that user. Sessions last 12 h (`auth/sessions.go:19`).

**Expected:** the break-glass reset evicts the account's sessions the same way the dashboard reset does.

**Actual:** existing sessions stay valid for up to 12 h after the rotation.

**How a maintainer confirms the control holds:** a bootstrap test that inserts a session for the user, runs the `--force` path, and asserts that `SELECT COUNT(*) FROM sessions WHERE user_id = ?` is 0.

### H-api-09

**Location:** `api/internal/handlers/capture.go:942-1050` (`captureDownload`), `:1065-1070` (`newValidatedHost`), `:1086-1094` (upstream request).

**Control:** cluster boundary for agent and sidecar proxies. Routes that can only reach home-cluster pods must refuse a non-local cluster, as the `ws/` proxies do (`ws/dialer.go:114-140`).

**Repro / observation** (by reading master `13a859ff`):
1. `capture.go:943` resolves `k` from `?cluster=`, and `:964` and `:994` read the GameServer and NetworkCapture on that cluster.
2. `:1065-1070` builds `<name>-agent.<ns>.svc.cluster.local:9091`, which resolves through the API pod's own cluster DNS, and `:1086-1094` sends the request with the home-cluster mTLS client.

**Expected:** the download either dispatches to the target cluster's sidecar or answers 404 for a non-local cluster, like the other home-only proxies.

**Actual:** a request for a remote cluster goes to the home cluster's same-named sidecar. For legitimate users, remote-cluster capture downloads don't work. Anything beyond that needs the admin-only `captures:manage` and a matching random capture id.

**How a maintainer confirms the control holds:** a handler test with `?cluster=<registered-non-local>` asserts either 404 before any upstream call, or an upstream host that belongs to the target cluster, and never the home-cluster service name.

### H-api-10

**Location:** `api/specs.md:430` and `:548`; `api/internal/registry/registry.go:12-15` (package doc) and `:208-209` (`NewSet` client).

**Control:** the SSRF dial guard (`netguard`) on outbound HTTP.

**Repro / observation** (by reading master `13a859ff`):
1. `api/specs.md:430`: netguard is described as the "SSRF dial-guard for outbound HTTP (module registry fetches)". `:548` repeats it.
2. `registry.go:208-209` builds a plain `&http.Client{Timeout: 15 * time.Second}` with no netguard dialer. The package doc at `:12-15` explains that every host is a fixed provider hostname.

**Expected:** the spec and the code agree. Either the registry client dials through netguard, or `api/specs.md:430,548` names the real netguard users (notification sinks, the Steam resolver) and states that registry hosts are fixed.

**Actual:** the spec claims a control on this path that the code doesn't implement. With today's fixed hosts, no exposure was shown.

**How a maintainer confirms the control holds:** if netguard is wired in, a unit test that points a provider's base URL at a link-local address expects `netguard.ErrBlockedAddr`. Otherwise, the spec diff is the fix.

### H-api-11

**Location:** `api/internal/handlers/module_upload.go:290-315` (`extractUploadArchive`; the comment at `:290-292`), caller `:78-89`.

**Control:** resource bounds on uploaded module bundles.

**Repro / observation** (by reading master `13a859ff`):
1. `:78-87` caps the request body, which is the compressed archive, at `maxUploadBundleBytes` (900 KiB).
2. `:303` caps the number of distinct member paths at 256, and `:306-312` caps each member at 900 KiB. No running total is kept, so the decompressed set can reach about 256 × 900 KiB in memory before `parseUploadedBundle` keeps only the four canonical files.
3. The route requires `modules:manage` (`rbac.go:231`).

**Expected:** the extractor enforces a total decompressed budget, for example `maxUploadBundleBytes` summed across members, while it extracts, and the comment says what is actually bounded.

**Actual:** the total isn't bounded. Only admin-tier users can reach the route, so this is hardening.

**How a maintainer confirms the control holds:** a unit test feeds `extractUploadArchive` an archive whose members together exceed the budget, while each stays under the per-member cap, and asserts an error before full expansion.

### H-api-12

**Location:** `api/internal/kube/watch.go:57-69` (`DeleteFunc`); `api/internal/handlers/clusters.go:192-236` (delete relies on the watcher). Reference pattern: `api/internal/notify/watch.go` uses `cache.DeletionHandlingMetaNamespaceKeyFunc`.

**Control:** access revocation when a remote Cluster registration is removed. The registry must drop the cluster's client, so `?cluster=<removed>` stops validating and dispatching.

**Repro / observation** (by reading master `13a859ff`):
1. `watch.go:58-61`: `DeleteFunc` returns when `obj` isn't `*unstructured.Unstructured`. An informer delivers `cache.DeletedFinalStateUnknown` when it missed the delete and relisted.
2. `clusters.go:192-236` deletes the CR and the kubeconfig Secret, but never calls `reg.Remove`. Removal depends only on the watcher.

**Expected:** a deleted Cluster registration always removes its client, including when the delete arrives as a tombstone.

**Actual:** the tombstone path is ignored, and the client stays registered until the API restarts. This needs a missed watch event, which is rare.

**How a maintainer confirms the control holds:** a watcher unit test delivers `cache.DeletedFinalStateUnknown{Obj: <cluster>}` to the handler and asserts that the id is gone from `reg.IDs()`. Optionally, also call `reg.Remove` directly in the `DELETE /clusters/{name}` handler, so revocation doesn't depend on the watch at all.

## Moved from audit/evidence/review-api/verification.md (OD-019, 2026-09-24)

### C-api-03 (partial: audit half only; the reset half stays in the git-bound file as F-076)

- Table-row sentence: The audit half affects only the first save on an install whose `auth` row does not yet exist, and the generic middleware row for `PUT /admin/config/auth` is still written.
- Section step:
4. On the same kind of install (Helm OIDC configured, auth section never saved), set a role-mapping override from Admin Settings, which sends `PUT /admin/config/auth`. `detectAuthAuditEvents` returns nil, so no `oidc role mapping override set: …` row is written. Only the generic middleware row for the PUT exists.
- Expected sentence: `api/specs.md:255` says every override set is audited with the structured reason.
- Actual clause: "and the first override saved on such an install gets no structured audit reason"

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-api-05 | kept | S3 | Confirmed. `WriteSync` enqueues `Reason` (`audit/audit.go:728-739`), but `webhookPayload` (`:221-229`) and `S3Sink.encodeNDJSON` (`s3.go:231-247`) drop it, and so does the CSV export header (`handlers/audit.go:76`). `api/specs.md:338` says the reason reaches every external sink. This matters beyond wording: the OIDC role-assignment event carries its role details only in `reason` (`auth/oidc.go:312-321`), so a webhook, S3 or syslog-bridge consumer receives `POST /auth/oidc/callback 200` with no record of which role was granted. The DB row and the JSON export keep the reason, which is the workaround. |
| C-api-06 | kept | S3 | Confirmed. `users.go:438-473` deletes the `users` row and the bindings only. The shipped DSN (`values.yaml:129`, `cmd/main.go:463`) does not turn foreign keys on, so `oidc_links` survives. `resolveOrLinkUser` finds no joined user (`auth/oidc.go:520-524`), takes the first-login path, and the `INSERT INTO oidc_links` violates the `(issuer, subject)` primary key (`migrations/001_init.sql:22-28`). A primary key is enforced whatever the FK pragma is. The callback answers 500 "login failed" (`oidc.go:471-475`) on every later attempt. `users.id` is `AUTOINCREMENT`, so orphaned links can't be re-attached to a new user. Workaround: hand-edit the DB. |
| C-api-07 | kept | S3 | Confirmed. `AttachAuditWriteSyncFunc` is called only for the Helm-flag provider (`cmd/main.go:228-231`). `Registry.build` (`auth/registry.go:308-342`) attaches the store and the Helm override function to each dashboard-managed provider, but no audit function, so `emitRoleAssignmentAudit` returns at `oidc.go:286`. The callback is a GET under `/auth/oidc/`, which `shouldLog` skips (`audit.go:744-758`), so no generic row is written either. FR-014 (`done_006/spec.md:83`) covers role assignments "arising from OIDC mappings" with no provider restriction, and `plan.md:83` threads the recorder into "the OIDC login path". It is a gap in audit coverage of privilege grants. Whether that makes it a security finding to hold under OD-019 is a maintainer call. It is left here because the reviewer filed it publicly. |
| C-api-11 | kept | S4 | Confirmed. `put` merge-patches `stringData` (`tunnelcreds.go:136-141`), so switching provider keeps the old provider's key in the Secret, and `get` returns the first match from a map iteration (`:225-231`), which is random in Go. The dashboard reads only `configured` (`web/src/routes/tabs/settings/Networking.tsx:113,246`), so it is not affected. |

### C-api-05

**Location:** `api/internal/audit/audit.go:183-186` (`WebhookSink.post` builds the payload), `api/internal/audit/audit.go:221-229` (`webhookPayload` has no `reason` field), `api/internal/audit/s3.go:231-247` (`encodeNDJSON`), `api/internal/handlers/audit.go:76` (CSV header). Promise: `api/specs.md:338`.

**Repro / observation**
1. Read `audit.go:728-739`: `WriteSync` enqueues `Event{…, Reason: reason}` to the webhook and S3 sinks.
2. Read `audit.go:183-186` and `:221-229`: the webhook body is built from `TS, Actor, Method, Path, Target, Status, IP` only. `s3.go:234-245` builds a map without `reason`, and the CSV export header is `id,ts,actor,method,path,target,status,ip`.
3. On a cluster with `api.audit.webhook.url` pointing at a request bin (or with the syslog bridge enabled), log in through an OIDC provider with a role mapping as a new user. The DB row (`GET /admin/audit`) has `reason: "oidc role assigned: provider=… matched=… from=new_user to=…"`. The webhook body for the same event has no `reason` key.

**Expected:** webhook, S3 and stdout sinks all carry `reason` (spec line 338), so external audit systems see capture failure reasons and OIDC role grants.

**Actual:** only the stdout sink and the JSON export carry it. For the role-assignment event, `reason` is the only field that records which role was granted, so external consumers can't see privilege grants.

### C-api-06

**Location:** `api/internal/handlers/users.go:438-473` (`del`); `api/internal/auth/oidc.go:520-524` (existing-user join) and the first-login insert that follows (`INSERT INTO oidc_links …`); schema `api/internal/db/migrations/001_init.sql:22-28`; the claim in `api/internal/db/migrations/011_user_theme_preferences.sql:11-13`.

**Repro / observation**
1. An SSO user logs in once. This creates rows in `users`, `oidc_links(issuer, subject)`, `user_role_bindings` and, later, `user_preferences`.
2. An admin deletes that user from Users (`DELETE /users/{id}`). The handler deletes the `users` row (`users.go:464`) and the role bindings (`:469`), nothing else. The default DSN (`values.yaml:129`) doesn't enable foreign keys, so the `ON DELETE CASCADE` on `oidc_links` never fires.
3. The same person logs in through the IdP again. The join at `oidc.go:520-524` finds no user, the first-login path creates a new `users` row, and the `oidc_links` insert fails on the `(issuer, subject)` primary key.
4. The browser gets 500 "login failed", and the API log shows `oidc resolveOrLinkUser … UNIQUE constraint failed` (`oidc.go:471-475`). Every later attempt fails the same way.

**Expected:** a deleted SSO user can be provisioned again on their next login. `011`'s comment says `users.go` deletes a user's rows explicitly.

**Actual:** that IdP subject is locked out until someone deletes the orphaned `oidc_links` row by hand. `user_preferences` rows are also left behind.

### C-api-07

**Location:** `api/cmd/main.go:228-231` (audit function attached only to `oidcAuth`, the Helm-flag provider); `api/internal/auth/registry.go:308-342` (`Registry.build` for dashboard-managed providers attaches no audit function); `api/internal/auth/oidc.go:285-288` (`emitRoleAssignmentAudit` returns when none is attached).

**Repro / observation**
1. Read `main.go:228-231`: `AttachAuditWriteSyncFunc` and `SetProviderName` are called only on `oidcAuth`.
2. Read `registry.go:308-342`: providers from the `auth` config row are built with `NewOIDCWithPolicy(... RoleMappings: p.RoleMappings ...)`, then get `AttachStore` and `AttachHelmRoleOverridesFunc`, but no audit function.
3. On a cluster, add a dashboard-managed OIDC provider with `roleMappings.admin: ["ops"]` (Admin Settings, Authentication), then log in as a new IdP user in group `ops`. That user becomes admin.
4. `GET /admin/audit` has no row for that login: `emitRoleAssignmentAudit` returns early, and `shouldLog` skips `GET /auth/oidc/…` (`audit.go:744-758`), so the middleware writes no generic row either.

**Expected:** FR-014 (`specs/done_006-install-time-config/spec.md:83`): role assignments and changes from OIDC mappings are recorded with the user, the new role and the matching rule. The requirement doesn't restrict this to the Helm provider.

**Actual:** role assignments and role changes through dashboard-managed providers leave no audit trace.

### C-api-11

**Location:** `api/internal/handlers/tunnelcreds.go:136-141` (merge patch of `stringData`), `api/internal/handlers/tunnelcreds.go:225-231` (provider inferred by ranging over the `tunnelProviderKeys` map).

**Repro / observation**
1. `PUT /servers/x:tunnel-credentials` `{"provider":"frp","values":{"token":"a"}}` creates `x-tunnel-auth` with `token`.
2. `PUT /servers/x:tunnel-credentials` `{"provider":"tailscale","values":{"authKey":"b"}}` merge-patches it, so the Secret now holds both `token` and `authKey`.
3. Call `GET /servers/x:tunnel-credentials` several times. `keys` flips between `["token"]` and `["authKey"]`, because Go randomizes map iteration order.

**Expected:** `keys` reflects the provider that was configured last, and switching provider doesn't leave the old provider's credential behind.

**Actual:** `keys` is random from one call to the next, and the stale frp token stays in the Secret. The dashboard reads only `configured`, so it doesn't show the problem.
