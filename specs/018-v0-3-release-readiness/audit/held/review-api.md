# Held: review-api security candidates (OD-019, off-git)

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `api/specs.md`, `docs/security.md`, `specs/done_006-install-time-config/spec.md`, `SECURITY_AUDIT.md` (none of the items below is listed there)
- **Companion notes**: `audit/evidence/review-api/notes.md` (public notes; it only states the count of held items)

Each item below names the control, where it lives, what the correct behaviour is, and how a maintainer confirms the control holds once it is fixed. Wording is defensive on purpose.

### H-api-01: The cluster-gating guard is missing on the mod-registry routes
- **Location**: `api/internal/handlers/registry.go:41-48` (mount), `:171-207` (`installModpack`, which writes the GameServer at `:202`); compare `api/internal/handlers/resources.go:90-108` (`rejectRemoteCluster`, whose comment names only MountModIDs and MountModUpdates) and `api/internal/rbac/rbac.go:97-109`
- **Category**: correctness (authorization: cluster gating)
- **Suggested severity**: S2
- **Observation / repro**:
  1. `rbac.Middleware` authorizes namespaced permissions against the cluster named in `?cluster=` (spec invariants 4 and 12).
  2. `MountRegistry`'s five routes use the bare home-cluster `*kube.Client` and never apply `rejectRemoteCluster`, unlike `mod_ids.go:73,93`, `mod_updates.go:119` and every `ws/` route.
  3. The result is that permission granted on a registered remote cluster is accepted for requests these handlers then serve against the local cluster's same-named GameServer, and `POST /servers/{name}/modpack` changes that GameServer's `spec.env`.
- **Expected**: the mod-registry routes return 404 for a non-local `?cluster=` (the same guard as the sibling mod routes), or dispatch through the registry to the named cluster.
- **Actual**: no guard.
- **How to confirm the fix holds**: add a handler test that calls each `MountRegistry` route with `?cluster=<non-local>` and asserts 404, and a structural check that every handler mounted with a bare `*kube.Client` under a namespaced RBAC rule is wrapped in `rejectRemoteCluster`.

### H-api-02: Client-IP extraction trusts X-Forwarded-For from any peer, and the default trusted ranges include LAN addresses
- **Location**: `api/cmd/main.go:251`, `:522-525`; `charts/gameplane/values.yaml:125`; consumers `api/internal/auth/ratelimit.go:85-97` and `api/internal/audit/audit.go:633-645`; wired middleware `chi/v5@v5.3.2/middleware/client_ip.go:92-117`
- **Category**: docs-drift (network exposure / rate limiting / audit integrity)
- **Suggested severity**: S3
- **Observation / repro**:
  1. `middleware.ClientIPFromXFF` walks X-Forwarded-For from the right, skips entries inside the trusted prefixes, and never looks at `RemoteAddr`.
  2. `docs/security.md:72-74` says "The API **only** trusts `X-Forwarded-For` from requests originating within the configured CIDR blocks, defeating IP spoofing", and `:76-78` says that when the API is directly exposed "`X-Forwarded-For` is ignored, and rate limiting uses the TCP peer's address". The wired middleware does neither.
  3. Because the default trusted list covers all RFC1918 ranges, a client whose real address is on a private LAN (the usual homelab case) is itself "trusted". Its identity is then either taken from an earlier, client-supplied XFF entry or not found at all, in which case every such client is keyed on the proxy pod's address.
- **Expected**: the documented behaviour. XFF is honoured only when the TCP peer is a trusted proxy, and a trusted-proxy set that matches the real client network does not remove per-client identity.
- **Actual**: the per-IP login limiter (`LoginLimiter`, `OIDCCallbackLimiter`, `ShareLimiter`, `MutationLimiter`) and the audit `ip` column rely on a value the client can influence or that collapses to the proxy address. The per-username limiter is unaffected.
- **How to confirm the fix holds**: a unit test where `RemoteAddr` is outside the trusted set and XFF carries another address must record the `RemoteAddr` host. A test with a trusted proxy peer and a private-range client must record the client's own address. Update the `docs/security.md` wording to whatever the fixed behaviour is.

### H-api-03: OIDC role re-evaluation ignores override-only mappings (FR-011)
- **Location**: `api/internal/auth/oidc.go:445-452` (effective policy), `:469` (`syncRole := o.policy != nil && o.policy.RoleMappings != nil`), the doc comment at `:612-613`; `api/internal/handlers/config.go:659-664` (accepts `helmOverride` whether or not Helm seeded any mappings)
- **Category**: correctness (authorization: role assignment)
- **Suggested severity**: S3
- **Observation / repro**:
  1. For the Helm provider, first login uses the merged effective policy (`resolvePolicy`). The decision to re-sync an existing user's role uses the Helm base policy.
  2. When Helm set `--oidc-issuer` without any `--oidc-role-mapping-*` flags, and an admin later adds `helmOverride.roleMappings`, a user's first login assigns a role from the overrides, but later logins never re-evaluate it. A user removed from the mapped IdP group keeps the role granted at first login.
- **Expected**: `spec.md` FR-011 in `specs/done_006-install-time-config`: "On each subsequent login, the user's role MUST be re-evaluated against the current mappings". The code comment at `oidc.go:612-613` says re-evaluation runs when "the effective policy (post-helmOverride merge) has RoleMappings configured".
- **Actual**: re-evaluation is keyed on the base (Helm-seeded) mappings only.
- **How to confirm the fix holds**: a test with a Helm policy that has `RoleMappings == nil`, an override that grants admin to group G, a first login in G (admin), then a second login without G. The role must change, subject only to the last-user-manager guard.

### H-api-04: The `/events` SSE stream bypasses per-resource read permissions
- **Location**: `api/internal/handlers/events.go:63-71`; `api/internal/rbac/rbac.go:242-243`; `api/internal/rbac/catalog.go:30-42`
- **Category**: correctness (authorization)
- **Suggested severity**: S3
- **Observation / repro**:
  1. `/events` is gated only by `servers:read` in the resolved namespace.
  2. The handler watches every entry in `kube.GVRs`, which covers servers, backups, schedules, restores and templates (templates cluster-wide), and streams the full objects.
  3. A custom role holding `servers:read` without `backups:read`, `schedules:read` or `templates:read` therefore receives those objects.
- **Expected**: the catalog says `backups:read` gates "View backups and restores", `schedules:read` gates "View schedules" and `templates:read` gates "View templates". The stream should include only the kinds the caller may read.
- **Actual**: every kind is sent to any holder of `servers:read`.
- **How to confirm the fix holds**: an SSE test with a custom role that has only `servers:read` asserts that no `kind` other than `servers` arrives. Include a namespace-scoped binding case so the cluster-wide template watch is covered.

### H-api-05: Share-link revocation is not bound to the server in the path
- **Location**: `api/internal/handlers/shares.go:232-271` (`:264-265`); `api/internal/db/shares.go:287-300`
- **Category**: correctness (authorization: object ownership)
- **Suggested severity**: S4
- **Observation / repro**:
  1. `revokeShareHandler` checks that the caller owns `{name}`, then calls `RevokeShareLink(cluster, id)`, whose SQL matches only `id` and `cluster`.
  2. The caller must already hold a link id, and ids appear only in owner-only list responses (12 random bytes). So the exposure is limited to someone who once owned a server, for example before an ownership transfer, or who otherwise learned an id.
- **Expected**: `MountShareLinks` says "only the server owner can manage shares". Revocation must also match `namespace` and `server_name` from the path.
- **Actual**: an owner of any server in the cluster can revoke a link on another server if the link id is known.
- **How to confirm the fix holds**: a store test where revoking with a mismatched namespace or server name affects no row, and a handler test that expects 404 in that case.
- **Also observed during the F-014 check** (review F-001..F-030 (opus), 2026-09-24; same code on master `13a859ff`; no new candidate filed).

### H-api-06: Share links outlive the user who created them (SQLite)
- **Location**: `api/internal/handlers/users.go:438-473`; `api/internal/db/shares.go:114-197`; `api/internal/db/migrations/006_share_links.sql:20` (`created_by … REFERENCES users(id) ON DELETE CASCADE`); `003_roles.sql` header ("The API layer is authoritative … clean up bindings on user delete")
- **Category**: correctness (access revocation)
- **Suggested severity**: S3
- **Observation / repro**:
  1. SQLite runs with foreign keys off (`api/specs.md:510`), so the schema's cascade never fires there.
  2. The user-delete handler removes only the `users` row and the role bindings. Share links created by the deleted user stay valid, and `LookupShareLink` does not check that the creator still exists, so their public tokens keep resolving status and address and, with `canStart`, waking the server.
- **Expected**: the same outcome as the schema's declared cascade. Deleting a user revokes, or deletes, that user's share links on both drivers.
- **Actual**: the links stay active on SQLite until their expiry, possibly never.
- **How to confirm the fix holds**: a handler test that creates a link as user U, deletes U, and asserts the token then returns the uniform 404.

### H-api-07: The break-glass `--enable-local-login` silently deletes the admin's OIDC role-mapping overrides
- **Location**: `api/cmd/bootstrap.go:156-199` (the struct at `:165-167` decodes only `providers`, and `:183` re-marshals only that)
- **Category**: correctness (authorization configuration)
- **Suggested severity**: S3
- **Observation / repro**:
  1. `enableLocalLogin` decodes the `auth` row into `struct{ Providers []map[string]any }` and writes it back.
  2. Any top-level key other than `providers`, notably `helmOverride.roleMappings`, is dropped. After a break-glass run, the next OIDC logins use the Helm-seeded mappings again, which can re-grant a role (for example admin) that an override had removed.
- **Expected**: `docs/security.md:58-59`: "It force-enables the local provider in the auth config row (preserving everything else)".
- **Actual**: `helmOverride` (and any other top-level key) is erased, and no audit row records it.
- **How to confirm the fix holds**: a bootstrap test that seeds an auth row with `helmOverride.roleMappings`, runs `--enable-local-login`, and asserts the override is byte-for-byte unchanged.

### H-api-08: `bootstrap-admin --force` resets the password without ending existing sessions
- **Location**: `api/cmd/bootstrap.go:111-126`; compare `api/internal/handlers/users.go:651-658` (the reset path calls `DeleteForUser`)
- **Category**: correctness (authentication: session revocation)
- **Suggested severity**: S4
- **Observation / repro**:
  1. The dashboard password reset deletes all of the target's sessions, and its comment says "we want reset to actually evict".
  2. The break-glass reset updates `pw_hash` and the role but leaves every `sessions` row, so an existing session for that account stays valid for up to 12 h after the operator rotates the credential.
- **Expected**: the break-glass reset evicts sessions the same way the dashboard reset does.
- **Actual**: the sessions survive.
- **How to confirm the fix holds**: a bootstrap test that creates a session for the user, runs `--force`, and asserts `SELECT COUNT(*) FROM sessions WHERE user_id = ?` is 0.

### H-api-09: Capture download on a non-local cluster is proxied to the local cluster's sidecar address
- **Location**: `api/internal/handlers/capture.go:985-992`, `:1065-1070`, `:1086-1094`
- **Category**: correctness (cluster boundary)
- **Suggested severity**: S3
- **Observation / repro**:
  1. `captureDownload` resolves the GameServer and NetworkCapture on the cluster named by `?cluster=`, then proxies to `<name>-agent.<ns>.svc.cluster.local:9091`, a name that resolves in the API's own (home) cluster. It uses the home cluster's mTLS client material.
  2. On a remote cluster the download therefore never reaches the capture's own sidecar. The only thing that keeps the request from reaching another cluster's server is the random capture id.
- **Expected**: capture endpoints are described as cluster-dispatch (`api/specs.md:170`). The download either dispatches correctly or 404s a non-local cluster, as the `ws/` proxies do (`ws/dialer.go:114-140`).
- **Actual**: a cross-cluster request reaches the home cluster's same-named sidecar.
- **How to confirm the fix holds**: a handler test with `?cluster=<non-local>` asserts either 404 or an upstream host that belongs to the target cluster, never the home-cluster DNS name.

### H-api-10: The spec says mod-registry fetches go through the SSRF guard, but they do not
- **Location**: `api/specs.md:430` ("netguard — SSRF dial-guard for outbound HTTP (module registry fetches)") and `:548`; `api/internal/registry/registry.go:12-15`, `:208-209`
- **Category**: docs-drift (network egress)
- **Suggested severity**: S4
- **Observation / repro**:
  1. `registry.NewSet` builds a plain `&http.Client{Timeout: 15s}` with no netguard dialer. The package doc says it "carries no per-request SSRF guard of its own" because every host is fixed.
  2. Today the hosts are hard-coded, so the practical exposure is low. Still, the spec states a control the code does not implement, and the GitHub provider takes `owner/repo` from template content.
- **Expected**: either wire netguard into the registry client or correct `api/specs.md:430,548`.
- **Actual**: spec and code disagree.
- **How to confirm the fix holds**: if netguard is wired, a test that points a provider base URL at a link-local address expects `netguard.ErrBlockedAddr`. Otherwise, a spec diff.

### H-api-11: Module-bundle upload does not bound the total decompressed size
- **Location**: `api/internal/handlers/module_upload.go:290-315` (the comment at `:291-292` says "Size is pre-capped by the caller, so only the file count needs guarding here")
- **Category**: correctness (input validation / resource bounds)
- **Suggested severity**: S4
- **Observation / repro**:
  1. The compressed body is capped at 900 KiB. The extractor then caps each member at 900 KiB and the member count at 256, but never the sum, so a small, highly compressible archive expands to roughly 256 × 900 KiB in memory before `parseUploadedBundle` discards the non-bundle files.
  2. The route requires `modules:manage` (admin-tier), which limits who can trigger this.
- **Expected**: the extractor enforces a running total (for example `maxUploadBundleBytes` across all members) while it extracts.
- **Actual**: the total is unbounded up to about 225 MiB.
- **How to confirm the fix holds**: a unit test that feeds `extractUploadArchive` an archive whose members together exceed the bundle cap, and asserts an error before full expansion.

### H-api-12: The cluster watcher drops tombstoned deletes, so an unregistered cluster can stay dispatchable
- **Location**: `api/internal/kube/watch.go:57-69`
- **Category**: correctness (access revocation)
- **Suggested severity**: S4
- **Observation / repro**:
  1. `DeleteFunc` type-asserts `obj.(*unstructured.Unstructured)` and returns when that fails. An informer passes `cache.DeletedFinalStateUnknown` when it missed the delete event and relisted.
  2. In that case `reg.Remove` never runs, and the deleted cluster's client stays in the registry, so `?cluster=<deleted>` keeps validating and dispatching until the API restarts. `notify/watch.go` already uses `cache.DeletionHandlingMetaNamespaceKeyFunc` for this case.
- **Expected**: deleting a Cluster registration always removes its client.
- **Actual**: the tombstone path is ignored.
- **How to confirm the fix holds**: a watcher test that delivers a `DeletedFinalStateUnknown{Obj: cluster}` and asserts the id is gone from `reg.IDs()`.

## Questions (not findings)

- `/metrics` is mounted before authentication (`api/cmd/main.go:260`), and the web nginx proxies any non-HTML request to the API, so it is reachable unauthenticated through the public ingress. It exposes `go_info` (Go version), process metrics and the audit and notify counters. `api/specs.md:102` lists it as public, but CLAUDE.md rule 3 says unauthenticated views must not expose versions. Should the ingress or nginx block `/metrics`, or should it require a token?
- A `users:manage` holder can raise their own primary role to `admin` through `PATCH /users/{self}`. The guard at `api/internal/handlers/users.go:511-543` only blocks self-demotion. Is `users:manage` meant to be admin-equivalent? If so, the catalog label ("Create, edit, and delete users and their role bindings") could say so.
- `ws.AgentClient.GetJSON` builds `https://<name>-agent.<ns>.svc.cluster.local:8090<path>` without the DNS-1123 check that `ws/dialer.go` applies (`api/internal/ws/agent_client.go:46-47`; caller `handlers/mod_updates.go:133`). The host suffix is fixed and the namespace is allow-listed, so no practical path is evident, but spec invariant 13 describes that check as the load-bearing property. Should it apply here too?
- `DELETE /clusters/{name}` deletes whatever Secret the Cluster CR names in the control-plane namespace, without the `gameplane.local/cluster-kubeconfig=true` label check that `kube/loader.go:39` applies on load (`api/internal/handlers/clusters.go:213-233`). A Cluster CR created outside the API (kubectl or GitOps) could name an unrelated Secret. Should delete require the label too?

## Moved from audit/evidence/review-api/notes.md (OD-019, 2026-09-24)

### C-api-03 (partial: audit half only; the reset half stays in the git-bound file as F-076)

- Heading clause: "and the first override audit"
- Repro item:
  3. On the same fresh install, call `PUT /admin/config/auth` with a `helmOverride.roleMappings` body. `detectAuthAuditEvents` returns nil on the error path, so no `oidc role mapping override set: …` row is written.
- Expected sentence: Spec line 255 says "Set/change operations are audited with reason 'oidc role mapping override set: role=<role> groups=<groups>'".
- Actual clause: "and the first override set gets only the generic middleware row with no structured reason"

### C-api-05: The webhook and S3 audit sinks (and the CSV export) drop the `reason` field
- **Location**: `api/internal/audit/audit.go:183-186`, `:221-229` (`webhookPayload` has no Reason); `api/internal/audit/s3.go:234-246`; `api/internal/handlers/audit.go:76-81`
- **Category**: docs-drift
- **Suggested severity**: S3
- **Observation / repro**:
  1. `WriteSync` enqueues `Event{…, Reason: reason}` to the webhook and S3 sinks (`audit.go:729-740`).
  2. `WebhookSink.post` marshals `webhookPayload{TS, Actor, Method, Path, Target, Status, IP}` without Reason. `S3Sink.encodeNDJSON` builds a map without `reason`. The CSV export header is `id,ts,actor,method,path,target,status,ip`.
- **Expected**: spec line 338: "WriteSync passes the reason field to all configured external sinks (webhook, S3, stdout) via the Event struct … enabling external audit systems to capture structured failure reasons alongside the HTTP status code".
- **Actual**: only the stdout sink and the JSON export carry `reason`. Capture failure reasons (`invalid_filter`, `capture_in_progress`, …) and OIDC role-assignment reasons never reach webhook or S3 consumers, including the syslog bridge.

### C-api-06: After an OIDC user is deleted, that identity can never log in again
- **Location**: `api/internal/handlers/users.go:464-471`; `api/internal/auth/oidc.go:520-524`, `:572-577`; `api/internal/db/migrations/011_user_theme_preferences.sql:12`
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. An SSO user logs in once, which creates rows in `users`, `oidc_links(issuer, subject)` and `user_role_bindings`.
  2. An admin deletes that user (`DELETE /users/{id}`). The handler deletes `users` and the bindings only. SQLite runs with foreign keys off (spec line 510), so the `oidc_links` and `user_preferences` rows remain.
  3. The same person logs in through the IdP again. The existing-user query joins `users` and finds no row, so the first-login path runs, and `INSERT INTO oidc_links …` violates the `(issuer, subject)` primary key.
- **Expected**: a deleted SSO user can be re-provisioned on the next login. Migration 011's comment also claims "users.go deletes a user's rows explicitly anyway".
- **Actual**: `resolveOrLinkUser` returns the constraint error, and every later login by that subject gets 500 "login failed" until someone edits the DB by hand. `user_preferences` rows are orphaned too, which contradicts the 011 comment.

### C-api-07: Role changes from dashboard-managed OIDC providers are never audited (FR-014)
- **Location**: `api/cmd/main.go:228-231`; `api/internal/auth/registry.go:325-338`; `api/internal/auth/oidc.go:285-289`
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. `AttachAuditWriteSyncFunc` is called only on the Helm-flag provider (`main.go:229`).
  2. `Registry.build` builds each dashboard-managed provider, which can carry `roleMappings` (`config.go:578-590`), and attaches the store and Helm overrides but no audit function.
  3. Configure a DB provider with `roleMappings.admin: ["ops"]`, then log in as a new user in group `ops`. `emitRoleAssignmentAudit` returns at `if o.auditWriteSync == nil`.
- **Expected**: `specs/done_006-install-time-config/spec.md` FR-014: "Role assignments and changes to a user's role arising from OIDC mappings MUST be recorded as auditable events … including the user's name, new role, and the mapping rule". The requirement is not limited to the Helm provider.
- **Actual**: no role-assignment event is written for any DB-managed provider. The callback is a GET, which `shouldLog` skips, so there is no generic row either.

### C-api-11: After a provider switch, the tunnel-credentials GET reports the provider at random
- **Location**: `api/internal/handlers/tunnelcreds.go:136-141`, `:225-231`
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. `PUT /servers/x:tunnel-credentials` `{"provider":"frp","values":{"token":"a"}}` creates `x-tunnel-auth` with `token`.
  2. `PUT …` `{"provider":"tailscale","values":{"authKey":"b"}}` merge-patches `stringData`, so the Secret now holds both `token` and `authKey`.
  3. `GET /servers/x:tunnel-credentials` ranges over the `tunnelProviderKeys` map and returns the first provider whose keys are all present.
- **Expected**: `keys` reflects the provider last configured.
- **Actual**: Go map order is random, so `keys` is `["token"]` or `["authKey"]` from one call to the next, and the stale frp token stays in the Secret.

### Q-api-audit-login

- `shouldLog` (`audit.go:753-756`) says "Login events are audited only on success via the session creation path", but `sessions.Create` writes no audit row, so a successful OIDC login with no role change leaves no audit trace. Is that intended?

### Q-api-rbac-last-manager

- `roles.update` lets `users:manage` or `roles:manage` holders strip `users:manage` from a custom role without the last-user-manager guard that `users.go` applies. With no admin-role users left, this could lock out user administration. Has this been considered?
