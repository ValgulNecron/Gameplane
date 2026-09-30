# Held findings (OD-019, off-git)

| ID | Title | Component | Origin | Severity | Status | Fix | Re-verified | Note |
|----|-------|-----------|--------|----------|--------|-----|-------------|------|
| F-053 | Fixed-name deletes run without ownership check | operator | review:operator | S4 | open | | | |
| F-064 | OCI manifest body unverified, breaking signature guarantee | operator | review:operator | S1 | fixing | #427 | | |
| F-065 | Module spec.digest ignored in convergence check | operator | review:operator | S3 | fixing | #427 | | |
| F-066 | OCI and cosign fetches bypass netguard guard | operator | review:operator | S3 | fixing | #427 | | |
| F-067 | Capture with empty filter always fails | operator | review:operator | S3 | open | | | |
| F-068 | Playit tunnel egress and address reporting incomplete | operator | review:operator | S3 | open | | | |
| F-069 | Tunnel credential docs show unowned Secret creation | operator | review:operator | S3 | open | | | |
| F-070 | Volume-snapshot restore never finishes with owned Secrets | operator | review:operator | S3 | open | | | |
| F-071 | Operator RBAC docs contradict split design | operator | review:operator | S4 | open | | | |
| F-072 | Operator dev RBAC manifests bind nonexistent roles | operator | review:operator | S4 | open | | | |
| F-073 | ModuleSource spec.oci.insecure doc promises TLS-skip that code forbids | operator | review:operator | S4 | fixing | #427 | | |
| F-078 | WriteSync enqueues reason; webhook/S3/CSV sinks drop it | api/ | review:api | S3 | open | | | |
| F-079 | User delete leaves orphaned oidc_links on SQLite (no cascade) | api/ | review:api | S3 | open | | | |
| F-080 | Dashboard-managed OIDC providers skip audit on role assignment | api/ | review:api | S3 | open | | | |
| F-084 | Tunnel credentials merge-patch leaves stale key on provider switch | api/ | review:api | S4 | open | | | |
| F-090 | MountRegistry handlers lack rejectRemoteCluster guard; RBAC bypass | api/ | review:api | S1 | fixing | #430 | | |
| F-091 | ClientIPFromXFF never checks RemoteAddr; per-IP limits collapse | api/ | review:api | S3 | open | | | |
| F-092 | OIDC role sync keyed to Helm base only; override-only mappings skip re-eval | api/ | review:api | S3 | open | | | |
| F-093 | /events requires servers:read only; streams all object kinds | api/ | review:api | S3 | open | | | |
| F-094 | Share-link revoke WHERE clause doesn't match ownership check | api/ | review:api | S4 | open | | | |
| F-095 | User delete doesn't cascade oidc_links/preferences on SQLite | api/ | review:api | S3 | open | | | |
| F-096 | enableLocalLogin drops helmOverride from auth config silently | api/ | review:api | S3 | open | | | |
| F-097 | --force password reset doesn't evict sessions | api/ | review:api | S3 | open | | | |
| F-098 | captureDownload proxies to home cluster even for remote requests | api/ | review:api | S3 | fixing | #430 | | |
| F-099 | Spec claims netguard on registry fetches; code uses plain http.Client | api/ | review:api | S4 | open | | | |
| F-100 | Module archive extraction has no total decompressed budget | api/ | review:api | S4 | open | | | |
| F-101 | DeleteFunc ignores tombstones; cluster revocation waits for watcher | api/ | review:api | S4 | open | | | |
| F-112 | WebRcon dial error leaks RCON secret in error text | agent | review:agent | S3 | open | | | |
| F-113 | Upload destination symlink escape possible via existing link | agent | review:agent | S3 | open | | | |
| F-114 | Six spec statements contradict code security controls | agent | review:agent | S4 | open | | | |
| F-142 | Secret removal race with unsaved config | web | review:web | S3 | fixing | #431 | | |
| F-143 | Role mapping reset doesn't persist | web | review:web | S2 | fixing | #431 | | |
| F-144 | Installed module verification badge overstates confidence | web | review:web | S3 | open | | | |
| F-145 | Permission gates don't match API enforcement | web | review:web | S3 | open | | | |
| F-146 | netguard usage poorly documented | netguard/ | review:netguard | S4 | open | | | |
| F-147 | IsAllowed permits AWS/Alibaba cloud instance metadata addresses | netguard/ | review:netguard | S3 | open | | | |
| F-148 | IsPublic accepts reserved/special-purpose addresses and IPv4-compatible form | netguard/ | review:netguard | S4 | open | | | |
| F-169 | SteamCMD preset default image runs as root | gp-module | review:gp-module | S3 | open | | | |
| F-177 | FRP config server address and port names not escaped | tunnel | review:tunnel | S3 | open | | | |
| F-178 | Tailscale tags not applied at device registration | tunnel | review:tunnel | S3 | open | | | |
| F-194 | Record lost after peer closes connection | audit-syslog-bridge | review:audit-syslog-bridge | S3 | open | | | |
| F-199 | Body read has no time deadline | audit-syslog-bridge | review:audit-syslog-bridge | S4 | open | | | |
| F-202 | Missing required fields accepted without validation | telemetry-receiver | review:telemetry-receiver | S4 | open | | | |
| F-203 | Body size limit and trailing content not validated | telemetry-receiver | review:telemetry-receiver | S4 | open | | | |
| F-211 | Package boundary read-only guarantee overstated | mcp-server | review:mcp-server | S4 | open | | | |
| F-227 | Pre-auth /metrics endpoint exposed through public ingress | charts/gameplane/ | review:charts/gameplane | S3 | open | | | |
| F-228 | insecure: true means plain HTTP, not just skip verification | charts/gameplane/ | review:charts/gameplane | S3 | open | | | |
| F-229 | Empty groupsClaim doc says disabled; code defaults to 'groups' | charts/gameplane/ | review:charts/gameplane | S4 | open | | | |
| F-230 | Bring-your-own CA Secret path fails at install, unused spec fields | charts/gameplane/ | review:charts/gameplane | S3 | open | | | |
| F-231 | NetworkPolicy allows all private ports 443/6443; anti-SSRF hole | charts/gameplane/ | review:charts/gameplane | S3 | open | | | |
| F-235 | ingress-nginx manifest fetched from floating main branch, not pinned | deploy/ | review:deploy | S4 | open | | | |
| F-246 | Image push precedes signature check, leaving unsigned window | .github/workflows/ | review:github-workflows | S3 | open | | | |
| F-247 | Signing-secret confinement relies on convention only | .github/workflows/ | review:github-workflows | S3 | open | | | |
| F-248 | DELETE /clusters deletes arbitrary control-plane Secret | api/ | review:api | S3 | open | | | |
| F-249 | roles.update can strip users:manage with no last-user-manager guard | api/ | review:api | S3 | open | | | |
| F-250 | docs/security.md misstates game container privilege posture | docs/ | review:api | S4 | open | | | |
| F-256 | Owner-only server operations open to every namespace servers:write holder | api/ | maintainer:HQ-001 | S2 | open | | | S2, not S1: docs/security.md:133-134, the servers:write catalog label and the dashboard hint document this as allowed; HQ-001 (2026-09-24) rules it unintended. Maintainer may raise to S1 |

## Details

### F-064

**Repro / observation (defensive: confirms whether the control holds)**
1. Read `oci/client.go:Pull`. The returned digest is `manifestDesc.Digest` from `FetchReference(ctx, <tag>)`, and the manifest bytes come from `io.ReadAll` on the response body. Nothing computes `digest.FromBytes(manifestBytes)` or compares it.
2. Read `readBlob`: `Blobs().Fetch` followed by `io.ReadAll`, with no digest verification of the body.
3. Read the vendored oras-go: for a tag reference, `generateDescriptor` takes the digest from the response header when one is present and hashes the body only when the header is absent. `blobStore.Fetch` compares only header values. oras-go leaves body verification to the caller.
4. Read `module_controller.go:130`: `verifier.Verify(ctx, entry.Reference, bundle.Digest)` checks the signature for that digest. The template is then materialised from the unverified bytes.

**Expected:** Every byte the operator consumes is bound to the digest that cosign verified.

**Actual:** The manifest body and the layer bodies are not hashed. The signature therefore guarantees nothing about the content that is applied.

**Evidence:** [verification-operator.md#h-operator-01](verification-operator.md#h-operator-01)

### F-065

**Repro / observation (defensive)**
1. Read `:100-102` in module_controller.go. The condition compares `AppliedVersion`, `AppliedTemplate`, `Phase` and the catalog digest. `mod.Spec.Digest` does not appear in it.
2. On a `Ready` Module, a `spec.digest` edit that leaves `spec.version` unchanged returns at `:106`. The pin check at `:136` is never reached.

**Expected:** When `spec.digest` is set and differs from `status.appliedDigest`, the Module is not treated as converged.

**Actual:** The Module stays `Ready` on content that does not match the pin.

**Evidence:** [verification-operator.md#h-operator-02](verification-operator.md#h-operator-02)

### F-066

**Repro / observation (defensive)**
1. `grep -n netguard operator/internal/modsrc/*.go operator/internal/oci/*.go operator/internal/verify/*.go` finds hits only in `modsrc/git.go` and `modsrc/http.go`.
2. `oci.New` builds `retry.NewTransport(&http.Transport{})` with no guarded `DialContext`. `baseCheckOpts` passes only auth and context to go-containerregistry, so its default transport is used.
3. `docs/architecture.md` and `README.md` include OCI in the guard's scope, while `docs/security.md` lists git and http only.

**Expected:** Either the OCI client and the cosign registry client dial through netguard, or `docs/architecture.md` and `README.md:146` are narrowed to match `docs/security.md`.

**Actual:** The OCI and cosign fetch paths have no dial-time guard, and two documents say they do.

**Evidence:** [verification-operator.md#h-operator-03](verification-operator.md#h-operator-03)

### F-067

**Repro / observation (defensive)**
1. The dashboard's filter field starts empty and sends `undefined` when it is left blank. The API then stores `spec.filter: nil`.
2. The operator passes `nc.Spec.Filter` (nil) to `StartCapture`, so the sidecar receives an empty filter and answers 400. `IsTransientError` treats a 4xx as permanent, and the capture goes `Failed`.
3. `grep -rn` across `operator/` and `api/` finds no code that builds a filter from `tmpl.Spec.Ports`.

**Expected:** When `spec.filter` is nil, the operator builds the default filter from the GameTemplate's advertised ports and protocols and sends that.

**Actual:** A capture started without a filter always fails.

**Evidence:** [verification-operator.md#h-operator-04](verification-operator.md#h-operator-04)

### F-068

**Repro / observation (defensive)**
1. Read the `playit` case in gameserver_tunnel.go. It adds no ports, although its comment says "we permit all ports". The resulting rule lists DNS (53/UDP and 53/TCP) plus the template's advertised container ports.
2. `grep -n "gameplane-tunnel" charts/gameplane/templates/networkpolicies.yaml` finds nothing, so no chart policy adds egress for tunnel pods.
3. Read the TODOs: nothing writes a playit address to status yet.

**Expected:** The maintainer decides the intended playit egress scope and the docs describe the actual state of address reporting.

**Actual:** Under the chart defaults, a playit tunnel pod cannot reach its relay (fails closed). The docs also promise an address on status that nothing writes.

**Evidence:** [verification-operator.md#h-operator-05](verification-operator.md#h-operator-05)

### F-069

**Repro / observation (defensive)**
1. The three setup sections in `docs/tunnels.md` create the Secret with `kubectl create secret generic`, which sets no ownerReference.
2. `reconcileTunnel` returns an error for such a Secret, and `Reconcile` returns before the Service, StatefulSet and status steps. The only signal is the operator log line.

**Expected:** The docs describe the supported way to supply credentials: the dashboard or API endpoint, or a Secret created with an ownerReference to the GameServer. The refusal also appears on the GameServer's status or conditions.

**Actual:** Following the docs leaves a GameServer that never gets its Service or StatefulSet, and nothing on its status says why.

**Evidence:** [verification-operator.md#h-operator-06](verification-operator.md#h-operator-06)

### F-070

**Repro / observation (defensive)**
1. A volume-snapshot Restore creates the new server by copying the original's whole spec, including Secret and ConfigMap references.
2. The referenced objects are owned by the original server, so the new server's reconcile is refused before its status is written, and its phase stays empty.
3. `awaitRestoredServer` treats an empty phase as "still starting" and requeues every 10 s with no deadline. The Restore stays `Running`.

**Expected:** The volume-snapshot restore gives the new server objects it owns (copied or re-created with the new owner), or drops or rewrites the inherited references, or fails the Restore with a clear message.

**Actual:** A volume-snapshot restore of any server that uses env Secret or ConfigMap refs, or a tunnel credential Secret, never finishes.

**Evidence:** [verification-operator.md#h-operator-07](verification-operator.md#h-operator-07)

### F-071

**Repro / observation (defensive)**
1. `agent_rbac.go` creates a Role `<sa>-heartbeat` with a single rule: `get` and `patch` on `gameservers/status`, restricted to `resourceNames: [<gs>]`.
2. `agent/client.go` builds an mTLS client. The operator calls the agent. The operator receives no calls from the agent and verifies no agent token.
3. The chart's ClusterRole is read-only for workloads and Secrets. Workload writes sit in the namespaced Role.

**Expected:** `operator/specs.md` and `docs/architecture.md` state the real grants and the real direction and mechanism of authentication.

**Actual:** Both documents describe a different model.

**Evidence:** [verification-operator.md#h-operator-08](verification-operator.md#h-operator-08)

### F-072

**Repro / observation (defensive)**
1. `grep -rn "gameplane-operator-manager\|gameplane-operator-leader-election" operator/config` finds only the binding file. Neither role is defined there.
2. List `role.yaml`'s rules. Write verbs appear cluster-wide on workloads, volumes and CRDs. They come from write verbs in several controllers' kubebuilder markers.
3. The marker comment says the ClusterRole "keeps a compromised operator token from reading Secrets cluster-wide". Both `role.yaml` and the chart's ClusterRole grant `get/list/watch` on `secrets` cluster-wide.
4. `role_namespace.yaml` lacks grants the controllers use.

**Expected:** The dev RBAC manifests reference roles that exist. The kubebuilder markers declare only the cluster-wide reads plus the few cluster-scoped writes, and the namespaced Role matches the chart's.

**Actual:** The dev manifests bind nothing usable, and the generated ClusterRole is broader than the design it documents.

**Evidence:** [verification-operator.md#h-operator-09](verification-operator.md#h-operator-09)

### F-073

**Repro / observation (defensive)**
1. `oci/client.go` sets only `r.PlainHTTP = c.insecure`, and its doc comment says the client "deliberately does not skip TLS verification".
2. `grep -rn InsecureSkipVerify operator | grep -v _test` returns nothing.
3. The CRD description of `spec.oci.insecure` says it "skips TLS verification". The description of the neighbouring git/http field already states the correct behaviour.

**Expected:** The CRD doc, and the regenerated CRD description, say "plain HTTP only; TLS verification is never skipped".

**Actual:** The documentation promises a weaker behaviour than the code implements.

**Evidence:** [verification-operator.md#h-operator-10](verification-operator.md#h-operator-10)

### F-090

**Repro / observation (by reading master `13a859ff`)**
1. `api/cmd/main.go:331` mounts `handlers.MountRegistry(…)` with the home cluster client.
2. `api/internal/handlers/registry.go:66-207`: the five MountRegistry routes (providers, search, versions, modpackDeps, installModpack) never call `rejectRemoteCluster`.
3. By contrast, `mod_ids.go:73` and `:93` call `rejectRemoteCluster` on sibling routes with the same home client.

**Expected:** every MountRegistry route answers 404 for `?cluster=<registered-non-local>`, or dispatches through the registry to that cluster.

**Actual:** no guard on any of the five routes.

**How a maintainer confirms the control holds:** add a handler test that calls each MountRegistry route with `?cluster=<registered-non-local>` and asserts 404 with no call to the Kubernetes client. Add a structural check that every handler with a bare `*kube.Client` under namespaced RBAC is wrapped by `rejectRemoteCluster`. After the fix, multicluster e2e should show that a user bound only to a remote cluster cannot change a home-cluster GameServer through `/servers/{name}/modpack`.

**Evidence:** [verification-api.md#h-api-01](verification-api.md#h-api-01)

### F-091

**Repro / observation (by reading master `13a859ff`)**
1. `client_ip.go:92-117`: ClientIPFromXFF walks the header right to left but never reads RemoteAddr.
2. `cmd/main.go:522-523`: the default trusted list is loopback, all of RFC 1918, link-local and ULA.
3. On a LAN install where the client IP is in RFC 1918, the result depends on the path in front of the API and either lacks identity or takes an identity the proxies didn't add.

**Expected:** the TCP peer is checked against the trusted set before any XFF entry is used.

**Actual:** the peer is not checked. Per-IP limiting and audit collapse for private-range clients.

**How a maintainer confirms the control holds:** unit tests on the middleware chain. (a) RemoteAddr outside the trusted set plus an XFF header: recorded client IP is the RemoteAddr host. (b) RemoteAddr is a trusted proxy and XFF ends in a private-range client address: recorded IP is that client address. (c) Two such clients get different limiter buckets. Update docs/security.md to match the fixed behaviour.

**Evidence:** [verification-api.md#h-api-02](verification-api.md#h-api-02)

### F-092

**Repro / observation (by reading master `13a859ff`)**
1. `oidc.go:447-454`: first-login role is resolved from merged policy (Helm + override).
2. `oidc.go:469`: syncRole is keyed to `o.policy != nil && o.policy.RoleMappings != nil` — the Helm base.
3. With no `--oidc-role-mapping-*` flags, RoleMappings is nil in the Helm base, so syncRole is false.
4. On a later login, the role is never re-evaluated, even if an override with role mappings exists.

**Expected:** re-evaluation runs whenever the effective, post-merge policy has role mappings.

**Actual:** re-evaluation is keyed on Helm mappings only.

**How a maintainer confirms the control holds:** a unit test with a Helm policy whose RoleMappings is nil and an override that maps group G to admin. First login in G gives admin. A second login without G must change the role (and emit the FR-014 audit event), except when the last-user-manager guard applies.

**Evidence:** [verification-api.md#h-api-03](verification-api.md#h-api-03)

### F-093

**Repro / observation (by reading master `13a859ff`)**
1. `rbac.go:243`: /events requires only servers:read.
2. `events.go:63-110`: the handler watches all entries of kube.GVRs (servers, templates, backups, schedules, restores) and streams each full object.
3. A custom role with servers:read but without backups:read receives all backups in the stream.

**Expected:** the stream includes only kinds the caller may read in the resolved namespace.

**Actual:** every kind is sent to any holder of servers:read.

**How a maintainer confirms the control holds:** an SSE handler test with a custom role that holds only servers:read asserts that no event with a kind other than servers arrives. Include a namespace-scoped binding case.

**Evidence:** [verification-api.md#h-api-04](verification-api.md#h-api-04)

### F-094

**Repro / observation (by reading master `13a859ff`)**
1. `handlers/shares.go:251-258`: ownership check is on {name}.
2. `db/shares.go:289-300`: revoke SQL matches `id` and `cluster` only, not the path's namespace or server name.

**Expected:** revocation matches cluster, namespace, server_name and id.

**Actual:** only id and cluster are matched. A mismatch affects no row; impact is small because ids are random and appear only in owner-only listings.

**How a maintainer confirms the control holds:** a store test where revoking with a mismatched namespace or server name returns a not-found sentinel and leaves the row untouched, and a handler test that expects 404 in that case.

**Evidence:** [verification-api.md#h-api-05](verification-api.md#h-api-05)

### F-095

**Repro / observation (by reading master `13a859ff`)**
1. `006_share_links.sql:20`: `created_by INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE`.
2. `values.yaml:129`: the shipped DSN doesn't enable SQLite foreign keys, so the cascade never fires.
3. `users.go:464-471`: delete removes only the users row and bindings.
4. LookupShareLink checks revocation and expiry only, not whether the creator exists.

**Expected:** deleting a user revokes or deletes the share links that user created, on both SQLite and Postgres.

**Actual:** on SQLite the links stay valid indefinitely.

**How a maintainer confirms the control holds:** a handler test that creates a link as user U, deletes U, and asserts that the token now gets the uniform not-found response from `GET /shares/{token}`.

**Evidence:** [verification-api.md#h-api-06](verification-api.md#h-api-06)

### F-096

**Repro / observation (by reading master `13a859ff`)**
1. `bootstrap.go:165-170`: enableLocalLogin decodes into `struct{ Providers []map[string]any }`.
2. `:183-195`: it marshals that struct back, dropping any top-level key outside `providers`.
3. helmOverride is lost; the Helm-seeded role mappings apply again on next OIDC logins.

**Expected:** the command preserves everything else in the auth config row, per docs/security.md:58-59.

**Actual:** helmOverride is erased and unaudited.

**How a maintainer confirms the control holds:** a bootstrap test that seeds an auth row with providers and helmOverride.roleMappings, runs enableLocalLogin, and asserts that the override is unchanged and the local provider is enabled.

**Evidence:** [verification-api.md#h-api-07](verification-api.md#h-api-07)

### F-097

**Repro / observation (by reading master `13a859ff`)**
1. `bootstrap.go:114-124` (--force path): updates pw_hash and role, but leaves sessions alone.
2. Compare `handlers/users.go:651-658` (dashboard reset): evicts sessions explicitly.
3. Sessions last 12h, so the new credential takes up to 12h to take effect on existing sessions.

**Expected:** break-glass reset evicts the account's sessions like the dashboard reset does.

**Actual:** existing sessions stay valid for up to 12h.

**How a maintainer confirms the control holds:** a bootstrap test that inserts a session for the user, runs the --force path, and asserts that `SELECT COUNT(*) FROM sessions WHERE user_id = ?` is 0.

**Evidence:** [verification-api.md#h-api-08](verification-api.md#h-api-08)

### F-098

**Repro / observation (by reading master `13a859ff`)**
1. `capture.go:943`: resolves k from ?cluster=.
2. `:1065-1070`: builds `<name>-agent.<ns>.svc.cluster.local` and `:1086-1094` sends with home-cluster mTLS.
3. On a remote cluster, the downstream resolves the home cluster's sidecar.

**Expected:** the download either dispatches to the target cluster's sidecar or answers 404 for a non-local cluster, like the other home-only proxies.

**Actual:** a request for a remote cluster goes to the home cluster's same-named sidecar.

**How a maintainer confirms the control holds:** a handler test with `?cluster=<registered-non-local>` asserts either 404 before any upstream call, or an upstream host that belongs to the target cluster, and never the home-cluster service name.

**Evidence:** [verification-api.md#h-api-09](verification-api.md#h-api-09)

### F-099

**Repro / observation (by reading master `13a859ff`)**
1. `api/specs.md:430` and `:548`: netguard is described as the "SSRF dial-guard for … module registry fetches".
2. `registry.go:12-15`: package doc explains every host is a fixed provider hostname.
3. `registry.go:208-209`: NewSet uses `&http.Client{Timeout: 15s}` with no netguard dialer.

**Expected:** the spec and code agree on whether netguard guards registry fetches.

**Actual:** the spec claims a control that the code doesn't implement.

**How a maintainer confirms the control holds:** if netguard is wired in, a unit test that points a provider's base URL at a link-local address expects `netguard.ErrBlockedAddr`. Otherwise, the spec diff is the fix.

**Evidence:** [verification-api.md#h-api-10](verification-api.md#h-api-10)

### F-100

**Repro / observation (by reading master `13a859ff`)**
1. `module_upload.go:78-87`: the request body is capped at 900 KiB (compressed).
2. `:303`: the number of distinct member paths is capped at 256.
3. `:306-312`: each member is capped at 900 KiB.
4. No running total is kept, so decompressed set can reach about 256 × 900 KiB before parseUploadedBundle keeps only four files.

**Expected:** the extractor enforces a total decompressed budget while it extracts.

**Actual:** the total isn't bounded.

**How a maintainer confirms the control holds:** a unit test feeds extractUploadArchive an archive whose members together exceed the budget, while each stays under the per-member cap, and asserts an error before full expansion.

**Evidence:** [verification-api.md#h-api-11](verification-api.md#h-api-11)

### F-101

**Repro / observation (by reading master `13a859ff`)**
1. `watch.go:57-61`: DeleteFunc returns when obj isn't `*unstructured.Unstructured`, so `cache.DeletedFinalStateUnknown` is ignored.
2. `clusters.go:192-236`: DELETE /clusters/{name} deletes the CR and Secret but never calls `reg.Remove`.
3. Removal depends only on the watcher, which can miss the delete and deliver it as a tombstone.

**Expected:** a deleted Cluster registration always removes its client, including tombstones.

**Actual:** the tombstone path is ignored; the client stays registered until API restart.

**How a maintainer confirms the control holds:** a watcher unit test delivers `cache.DeletedFinalStateUnknown{Obj: <cluster>}` to the handler and asserts that the id is gone from `reg.IDs()`. Optionally, also call `reg.Remove` directly in the `DELETE /clusters/{name}` handler, so revocation doesn't depend on the watch at all.

**Evidence:** [verification-api.md#h-api-12](verification-api.md#h-api-12)

### F-112

**Repro / observation (defensive — confirms the control is in place; no payload)**
1. The WebRcon protocol takes the secret in the URL path. `ensureLocked` builds `ws://<host>:<port>/<url.PathEscape(secret)>`.
2. When the handshake request fails at the transport level, `http.Client.Do` returns a `*url.Error`. Its text is `Op "URL": err`, and its URL has passed only through `stripPassword`, which redacts userinfo and leaves the path.
3. `console.serve` sends `err.Error()` as the body of the `err` envelope. The players, status and actions handlers log it with `slog`.
4. The shipped module that uses this protocol is `rust` (`rcon.protocol: websocket`). Its console mode is `rcon`, so the Console tab is shown and talks to the agent.

**Control**: RCON error text never leaves the agent, because it can carry addresses or secrets. `players.go:131-132` and `actions.go:157-159` state this rule, and both handlers reply with a generic "upstream unavailable". The console handler does not follow the rule.

**Expected**: The RCON secret never appears in an error string. The WebRcon client redacts the URL path from dial errors, just as `mods.redactURLErr` strips query strings. The console handler returns a generic message for transport failures and logs detail only after redaction.

**Actual**: The dial error text includes the path-escaped secret. The console handler sends it to the client, and three handlers log it.

**How a maintainer confirms it holds**: Add a unit test in `agent/internal/rcon` that points `NewWebSocket` at a closed local port with a `PassFn` returning a unique sentinel, and assert that neither the sentinel nor its `url.PathEscape` form appears in the error text. Add a `console` test with a fake `Rcon` whose error contains a sentinel, and assert that the `err` envelope doesn't carry it.

**Evidence:** [verification-agent.md#h-agent-01](verification-agent.md#h-agent-01)

### F-113

**Repro / observation (defensive — confirms the control is in place; no payload)**
1. `upload` calls `resolve` on the directory from `?path=` and never on the final path.
2. `savePart` removes directory components with `filepath.Base` and rejects `.` and `..`. It then calls `os.Create(dir/name)`, which opens with O_CREAT|O_TRUNC and follows a symlink that already exists at that name.
3. The precondition is a symlink in the data volume that points outside the root. The file API has no way to create one. The mods archive handling rejects symlink entries. So the link would have to come from another writer of the volume.
4. The agent container runs with a read-only root filesystem, and its only other mounts limit the reach of such a write.

**Control**: Every file operation is confined to `--data-root`, and symlinks can't lead outside it. `agent/specs.md:14`, `:201` and `SECURITY_AUDIT.md:150` all state this. `/files/write` enforces it on the full path. The mods upload enforces it through `ConfinePath`.

**Expected**: The upload destination gets the same full-path confinement as `/files/write` (`resolve` or `ConfinePath` on `dir/name`), and/or the part is written to a temporary file and renamed into place.

**Actual**: The final upload path is never confined, and an existing symlink at that name is followed.

**How a maintainer confirms it holds**: Add a unit test in `agent/internal/files`. Create a root that contains a symlink to a file outside the root, then POST a multipart upload whose part filename matches the link's name. Assert a 4xx response, and that the outside file's content is unchanged.

**Evidence:** [verification-agent.md#h-agent-02](verification-agent.md#h-agent-02)

### F-114

**Repro / observation (defensive — confirms the control holds; each item gives the spec text, then what the code enforces)**
1. `specs.md:153`: WebRcon dials "using `netguard.IsPublic()` … cannot reach private/loopback addresses". The code uses `netguard.IsAllowed`, which permits loopback and private ranges. The code is right: the agent always dials the in-pod game at the `--rcon-host` default `127.0.0.1`. `SECURITY_AUDIT.md:155` says "`IsAllowed` (operator) and `IsPublic` (agent)". The agent uses both: `IsPublic` for mod downloads and `IsAllowed` for WebRcon.
2. `specs.md:112-114`, `:144` and `:205`: call `/healthz` and `/metrics` "Public (unauthenticated)". The operator always passes `--tls-cert/--tls-key/--tls-client-ca`. `auth.ServerTLS` then sets `ClientAuth: tls.RequireAndVerifyClientCert`, which requires a client certificate at the handshake for every path. This fails closed. The agent container has no probes that would depend on it.
3. `specs.md:201`: the `files` package "rejects … dotfile access". `resolve` has no dotfile rule, so list, read, write and delete all accept dot-prefixed names. That includes the mods ledger `.gameplane-mods.json`, whose comment says it is "out of reach of client-supplied names". That holds only for the mods API.
4. `specs.md:199`: private registries "are rejected unless explicitly whitelisted". Mod downloads dial through `netguard.IsPublic`, whatever `allowedHosts` contains. The code is stricter than the sentence. `docs/security.md:343-350` describes the real behaviour correctly.
5. `specs.md:92`: `--tls-cert` "if set, requires `--tls-key` and enables HTTPS + mTLS". TLS is enabled only when both flags are set. With `--tls-cert` alone, the listener starts on plain HTTP without an error. In mTLS mode this fails closed. Only hand-run agents are affected.
6. `specs.md:152`: `ConfinePath` is "the single point of validation for all filesystem operations on untrusted paths", including archive extraction. The `files` package uses its own `resolve`, and archive extraction uses `ConfineRelPath`.

**Control**: `agent/specs.md` and `SECURITY_AUDIT.md` are the documented description of the agent's security controls. Reviewers and operators rely on them.

**Expected**: `agent/specs.md` and `SECURITY_AUDIT.md` state what the code enforces. For items 3 and 5, the maintainer decides whether the spec wording is the intended contract and the code should change instead: rejecting dotfiles in `resolve`, or refusing to start when `--tls-cert` is given without `--tls-key`.

**Actual**: The six statements above don't match the code.

**How a maintainer confirms it holds**: Read the cited lines against the corrected text. For item 2: a TLS handshake to an agent pod without a client certificate is refused, and one with the API's client certificate succeeds. For item 3, if the dotfile rule is kept as the contract: a unit test that reads or writes `/.gameplane-mods.json` through `/files/*` expects 4xx. For item 5, if the spec wording is kept: a startup test that passes `--tls-cert` without `--tls-key` expects a non-zero exit.

**Evidence:** [verification-agent.md#h-agent-03](verification-agent.md#h-agent-03)

### F-142

**Location:** `web/src/routes/AdminSettings.tsx:307-315` (identity provider removal), `:974-982` (mod-registry key removal), `:1460-1468` (notification sink removal); `useSectionForm` at `:197-224`.

**Control:** The lifecycle of the API-managed Secrets behind dashboard-managed identity providers, keyed mod registries and notification sinks. A Secret and the config row that references it should change together.

**Repro / observation** (defensive: confirms whether the control holds):
1. Each trash button does two things: calls `deleteSecret` at once, and changes only the section's local draft.
2. `useSectionForm.save` is the only path that PUTs the section config. Leaving the page or a failed validation leaves the stored config unchanged.
3. The add forms work the other way round: they write the Secret before the row exists in the saved config.

**Expected:** A Secret is deleted only as part of, or after, a successful save of the config change that stops referencing it. A cancelled add doesn't leave a Secret.

**Actual:** Removing a row and then leaving without saving keeps the row in the stored config while its Secret is already gone. The dependent feature stops working until the secret is entered again.

**Evidence:** [verification-web.md#hc-web-01](verification-web.md#hc-web-01)

### F-143

**Location:** `web/src/routes/AdminSettings.tsx:197-199` (`useSectionForm` seeds its draft once), `:130` (`AuthSection` is not keyed), `:1748-1757` (`handleReset`); `api/internal/handlers/config.go:102-156` (`put`), `:159-250` (`resetRoleMapping`).

**Control:** The OIDC role-mapping overrides, which decide which IdP groups get admin, operator or viewer. "Reset to Helm default" should remove an override for good.

**Repro / observation** (defensive: confirms whether the control holds):
1. `handleReset` calls `DELETE /admin/config/auth/role-mappings/{role}`, and on success the API removes the override.
2. `useSectionForm` ignores new `initial` values (`useState(initial)`), so `f.draft.helmOverride` still holds the removed mapping.
3. Any later "Save changes" in the Authentication card PUTs `f.draft`, and `config.go:put` stores it, putting the override back. The reset only logs to console on failure.

**Expected:** After a reset, the card and the draft show the Helm value. No later save brings the override back.

**Actual:** The reset looks as if nothing happened, and the next save of the Authentication section re-applies the override the admin just reset, including an admin-group mapping. This is an unintended change to authorization config.

**Evidence:** [verification-web.md#hc-web-02](verification-web.md#hc-web-02)

### F-144

**Location:** `web/src/lib/verify.ts:48-51` (`verifyForEntry`, installed branch), `:22-24` (the contract comment); `web/src/components/modules/ModuleCard.tsx:249-265` (`VerifyBadge`); `operator/internal/controller/module_controller.go:100-107` (the early return for a converged module).

**Control:** The cosign signature-verification posture that the Modules catalog shows for installed modules. A solid "verified" chip should mean that the installed digest was signature-checked.

**Repro / observation** (defensive: confirms whether the control holds):
1. For an installed entry, `verifyForEntry` returns `enforced: mode !== "none"`, where `mode` comes from the `installedFrom` source's *current* `spec.verify`.
2. `VerifyBadge` renders the solid "verified" chip whenever `enforced` is true.
3. The Module reconciler returns early when the module is converged. A verify policy added to a source after install doesn't make the operator pull or verify the installed bundle again. Nothing on the Module status records whether a verification actually ran.

**Expected:** The solid "verified" state appears only when the operator has actually verified the installed digest, as recorded on the Module status. A policy added later shows as the softer "policy" state.

**Actual:** A module installed while its source had no verify policy shows "verified" as soon as a policy is added to that source.

**Evidence:** [verification-web.md#hc-web-03](verification-web.md#hc-web-03)

### F-145

**Location:** (a) `web/src/routes/ServerDetail.tsx:133-138` (the Capture tab is always visible), `web/src/components/CaptureWidget.tsx:180-203`. (b) `web/src/routes/tabs/Mods.tsx:68`, `web/src/lib/auth.ts:34-42`. For comparison, `web/src/routes/tabs/settings/NetworkCapture.tsx:53` and `web/src/components/server/ServerActionsMenu.tsx:37-39` pass a namespace.

**Control:** The dashboard's permission gating. The API is still the enforcer. The UI should show a control exactly when the API would accept the action, using the same permission and namespace.

**Repro / observation** (defensive: confirms whether the control holds):
1. (a) `CaptureWidget` renders "Enable Capture", "Start Capture" and "Delete" with no `can(me, "captures:manage", ns)` check. The Settings → Network capture section does gate on it.
2. (b) `can(me, perm)` with no namespace checks only `perms["*"]`. A user whose `servers:write` comes only from a binding in `gameplane-games` gets `false`, even for a server in `gameplane-games`, which the API authorizes against that namespace's binding.

**Expected:** Each UI gate uses the permission and namespace the API checks for that action: `captures:manage` in the server's namespace for capture controls, and `servers:write` in the server's namespace for mods and actions.

**Actual:** (a) shows capture controls to users the API will refuse; the refusal is displayed as an error. (b) disables mod and action controls for namespace-scoped operators whose actions the API would accept.

**Evidence:** [verification-web.md#hc-web-04](verification-web.md#hc-web-04)

### F-147

**Location:** `netguard/netguard.go:95-110` (`IsAllowed`); the promise is at `netguard/netguard.go:1-4`, `netguard/specs.md:8,76` and `docs/security.md:308-313`.

**Control:** `IsAllowed` is the permissive dial-time address policy for admin-configured git/http ModuleSource fetches, notification-sink delivery, and the agent's loopback WebSocket RCON. It is meant to refuse cloud instance-metadata endpoints while keeping RFC 1918, ULA, loopback and CGNAT reachable for self-hosted endpoints.

**Repro / observation** (defensive; confirms whether the control holds):
1. Read `netguard/netguard.go:99-108`. `IsAllowed` refuses unspecified, multicast, interface-local multicast, link-local unicast and link-local multicast addresses, plus `64:ff9b::/96` and `2002::/16`. Nothing else is refused.
2. `fd00:ec2::254` (AWS EC2 IPv6 instance-metadata address, inside ULA) and `100.100.100.200` (Alibaba Cloud ECS metadata address, inside CGNAT) both return `true` from `IsAllowed`.
3. `IsPublic` correctly refuses both addresses. Only the permissive policy is affected.

**Expected:** `IsAllowed` refuses known instance-metadata addresses by exact match: at least `169.254.169.254` (already refused via link-local), `fd00:ec2::254` and `100.100.100.200`. RFC 1918, ULA, loopback and the rest of CGNAT stay allowed. `SECURITY_AUDIT.md` should say only `IsPublic` refuses CGNAT.

**Actual:** Both addresses pass `IsAllowed`, so on those clouds the policy doesn't enforce its stated metadata refusal for admin-configured ModuleSources and notification sinks. Whether a given pod can reach those endpoints depends on the cloud's IMDS settings and the cluster's CNI.

**How a maintainer confirms the control holds:** Add `{"fd00:ec2::254", false}` and `{"100.100.100.200", false}` to the `TestIsAllowed` table (`netguard_test.go:14-35`) and let CI run it. Today both rows fail. The fix holds when they pass and the existing `10.0.0.1`, `fc00::1` and `127.0.0.1` → `true` rows still pass. Then read `SECURITY_AUDIT.md` and check that it no longer says `IsAllowed` refuses CGNAT.

**Evidence:** [verification-netguard.md#h-netguard-01](verification-netguard.md#h-netguard-01)

### F-148

**Location:** `netguard/netguard.go:117-132` (`IsPublic`), `:77-89` (`reservedBlocks`), `:56-61` (`normalize`), and `:65-68` (`blockedV6Prefixes`); the promise is at `netguard/specs.md:41-42`, `netguard/netguard.go:14-18` and `docs/security.md:344`.

**Control:** `IsPublic` is the strict dial-time policy for agent mod downloads (after the `allowedHosts` check) and the API's Steam resolver, documented to allow "only globally routable unicast addresses". It is built as a denylist.

**Repro / observation** (defensive; confirms whether the control holds):
1. Read `netguard/netguard.go:121-131`. Go's `IsUnspecified` matches only `0.0.0.0`, and `reservedBlocks` has no `0.0.0.0/8` entry, so `IsPublic(0.0.0.1)` returns `true`.
2. Read `netguard/netguard.go:56-61`. `normalize` relies on `To4()`, which unwraps only `::ffff:0:0/96`. IPv4-compatible addresses (`::7f00:1`, `::a9fe:a9fe`) stay 16 bytes and `IsPublic` returns `true`.
3. `reservedBlocks` (`:85`) and `blockedV6Prefixes` (`:66`) list only well-known NAT64 prefix `64:ff9b::/96`. The RFC 8215 local-use prefix `64:ff9b:1::/48` is not listed, so `IsPublic(64:ff9b:1::a9fe:a9fe)` returns `true`. Also `100::/64` (RFC 6666 discard-only) is not listed, so `IsPublic(100::1)` returns `true`.
4. Reachability: mod downloads must first pass `allowedHosts`, and each redirect is checked again. No path to an internal host was shown in a default Linux netns.

**Expected:** `IsPublic` refuses every IANA special-purpose block marked not globally reachable: at least `0.0.0.0/8`, `::/96`, `64:ff9b:1::/48` and `100::/64`. `IsAllowed` also gets `64:ff9b:1::/48` and an unwrap-and-recheck of `::/96`. Alternatively, narrow the "only globally routable" wording to the ranges that are actually refused.

**Actual:** Each listed address passes `IsPublic`, and the `::/96` and `64:ff9b:1::/48` forms also pass `IsAllowed`. No path to an internal host was shown in a default install, so this is a gap between the documented contract and the denylist.

**How a maintainer confirms the control holds:** Add `0.0.0.1`, `::7f00:1`, `::a9fe:a9fe`, `64:ff9b:1::a9fe:a9fe` and `100::1` to the `blocked` list in `TestIsPublic` (`netguard_test.go:54-66`). Add `{"64:ff9b:1::a9fe:a9fe", false}` and `{"::a9fe:a9fe", false}` to `TestIsAllowed` (`:14-35`). CI must pass with these rows. Existing public rows (`8.8.8.8`, `2606:4700:4700::1111`) must stay allowed, and `10.0.0.1`, `fc00::1` and `127.0.0.1` must stay allowed by `IsAllowed`.

**Evidence:** [verification-netguard.md#h-netguard-02](verification-netguard.md#h-netguard-02)

### F-169

**Location:** `gp-module/internal/archetypes/archetypes.go:121-122` (the `steamcmd` preset's `Description` and `DefaultImage`); `gp-module/internal/scaffold/scaffold.go` (the rendered `spec` never includes `security`).

**Control:** Privilege defaults for scaffolded modules. The `steamcmd` preset is documented as producing a template that runs the game as a non-root user.

**Repro / observation (defensive; confirms whether the control holds):**
1. Read `archetypes.go:121`: the description ends "(Valve UDP ports, save volume, non-root user)". `specs/010-easy-module-building/contracts/archetypes-contract.md:13` lists "non-root security defaults" as a primary characteristic of `steamcmd`.
2. Read `archetypes.go:122`: `DefaultImage` is `cm2network/steamcmd:root@sha256:4d830b…`. The tag names the upstream image's root variant.
3. In a scratch directory, run `gp-module init x --archetype steamcmd -y` and read the generated `template.yaml`. It uses that image and has no `spec.security` block.
4. Read `operator/internal/controller/gameserver_controller.go:1981-1990`. With no `spec.security`, `gameContainerSecurityContext` returns nil, so the pod runs the game as whatever user the image declares.

**Expected:** The `steamcmd` preset produces a template whose game container does not run as root. Either the default image is a non-root variant, or the template carries a `spec.security` block with a non-root uid and gid. Then the preset description and the spec contract are true.

**Actual:** The preset's default is the root image variant and it emits no `spec.security` block, so the documented non-root default is not delivered.

**Evidence:** [verification-gp-module.md#h-gp-module-01](verification-gp-module.md#h-gp-module-01)

### F-177

**Location:** `tunnel/main.go:304-309` (`serverAddr = "%s"` gets `cfg.FrpServerAddr` unescaped); `tunnel/main.go:325-332` (`name = "%s"` gets the proxy name unescaped); `tunnel/main.go:424-432` (`escapeTomlString`, applied only to the token); CRD markers `operator/api/v1alpha1/gameserver_types.go:406-409,426-429` (length-only).

**Control:** The frpc config renderer must produce a file that holds exactly the keys the supervisor intends, whatever the GameServer spec contains.

**Repro / observation (defensive; confirms whether the control holds, no payload):**
1. Read `tunnel/main.go:304-309`. Of the three string values in the header, only `auth.token` goes through `escapeTomlString`. `serverAddr` is inserted as is.
2. Read `tunnel/main.go:313-332`. Each `BACKING_SERVICE_PORT` entry's name part is inserted into `name = "%s"` unescaped.
3. Read `operator/api/v1alpha1/gameserver_types.go:404-432`. `ServerAddr` and `RemotePortMapping.Name` carry only length markers, with no `Pattern`.
4. Read `api/internal/handlers/resources.go:544-554`. Only `credentialsSecretRef` is validated. `loadConfig` (`tunnel/main.go:141-162`) checks only that the values are non-empty.
5. So a `serverAddr` or `remotePorts[].name` containing a double quote and a line break passes every layer, and the rendered file can gain keys the operator never generated.

**Expected:** Every value written into the frpc TOML is either escaped or rejected before rendering. `serverAddr` parses as a hostname or IP, and proxy names are DNS labels, enforced by a CRD `Pattern` and/or `loadConfig`. The rendered file then holds exactly the generated keys.

**Actual:** Only the token is escaped. The server address and proxy names reach the TOML file unescaped and unvalidated beyond their length.

**Evidence:** [verification-tunnel.md#h-tunnel-01](verification-tunnel.md#h-tunnel-01)

### F-178

**Location:** `tunnel/main.go:169` (`TAILSCALE_TAGS` read), `tunnel/main.go:291` (only hostname and auth key passed), `tunnel/main.go:378-397` (config holds only `version`, `authKey` and `hostname`), `tunnel/main.go:478-482` (no tag flag); `docs/tunnels.md:183-184`; `operator/api/v1alpha1/gameserver_types.go:447`.

**Control:** Tailnet authorization of the tunnel device. The docs and the CRD tell admins that `spec.networking.tunnel.tailscale.tags` become the device's ACL tags at registration, so tailnet ACL rules written against those tags govern who can reach the device.

**Repro / observation (defensive; confirms whether the control holds):**
1. `operator/internal/controller/gameserver_tunnel.go:260-282` joins `spec.networking.tunnel.tailscale.tags` into `TAILSCALE_TAGS`.
2. `tunnel/main.go:169` stores it in `cfg.TailscaleTags`. `grep -n TailscaleTags tunnel/main.go` finds only the struct field and that assignment.
3. `renderTailscaleConfig` writes only `version`, `authKey` and `hostname`, and `buildCommand` passes no tag argument. So tailscaled registers with whatever identity the auth key carries.
4. `tunnel/specs.md:63` and `:119` list this as a known gap. `docs/tunnels.md:183-184` and the CRD field doc do not.

**Expected:** Either the configured tags reach tailscaled at registration and the device shows them, or `docs/tunnels.md:183-184` and the CRD field doc say the tags are not applied yet and tell admins to put the tags on the auth key instead.

**Actual:** The tags are accepted by the CRD and passed to the pod, then dropped. The device carries only the auth key's identity, while the guide and the CRD doc say it carries the configured tags.

**Evidence:** [verification-tunnel.md#h-tunnel-02](verification-tunnel.md#h-tunnel-02)

### F-199

**Location:** `audit-syslog-bridge/main.go:283-287` (the `http.Server` literal in `serve`) and `main.go:148` (`io.ReadAll(http.MaxBytesReader(...))` in `handle`), measured against `audit-syslog-bridge/specs.md:64`.

**Control:** the DoS bound on the bridge's HTTP intake. The spec promises three limits: a 64 KiB body cap, a read deadline and a write deadline.

**Repro / observation (how to confirm the control holds):**
1. Read `serve` in `main.go:283-287`. The server sets `ReadHeaderTimeout: 10 * time.Second` and nothing else: there is no `ReadTimeout` and no `IdleTimeout`.
2. Read `handle` in `main.go:138-168`. The body is read with `io.ReadAll` on a `MaxBytesReader`, which bounds size but not time, and nothing sets a per-request read deadline before the read. By contrast, the collector write sets a deadline (`main.go:263`).
3. Start the handler under `httptest.NewServer` with the production `http.Server` settings, open a raw connection, send complete headers with a non-zero `Content-Length`, then send no body. Assert that the server closes the connection within the configured read bound. On the current tree the connection stays open past the 10 s header timeout.

**Expected:** Per `specs.md:64`, a request whose body is not fully received within a bounded time is closed, for example by `http.Server.ReadTimeout` or by `http.NewResponseController(w).SetReadDeadline` before the body is read. That bounds the number and lifetime of intake goroutines.

**Actual:** Only the header phase is time-bounded. The spec's "read deadlines" have no matching code for the body phase. Exposure is limited by the chart's API-only NetworkPolicy when `networkPolicies.enabled=true`.

**Evidence:** [verification-audit-syslog-bridge.md#h-audit-syslog-bridge-01](verification-audit-syslog-bridge.md#h-audit-syslog-bridge-01)

### F-202

**Location:** `telemetry-receiver/main.go:125-140` (decode, then check only for negative counts), measured against `telemetry-receiver/specs.md:55-57`.

**Control:** input validation on `POST /ingest`. The spec says only a payload with exactly the three required fields is accepted and counted.

**Repro / observation (how to confirm the control holds):**
1. Read `main.go:125-140`. `json.Decoder` with `DisallowUnknownFields` fills a `payload` struct of plain `string`/`int` fields, and the only check after that is `Servers < 0 || Templates < 0`. The code never distinguishes an absent field from its zero value. A top-level `null` decodes into the zero struct without error.
2. POST bodies with one or more required fields missing, and a body that is the JSON literal `null`. On the current tree each gets `204`, and `/metrics` counts it.

**Expected:** A payload that lacks any of the three fields, or isn't a JSON object, is rejected with `400` and not counted, as `specs.md:45` says for malformed payloads and `specs.md:55-57` says for required fields.

**Actual:** Such payloads get `204` and are counted, which skews the fleet-size histograms towards `0`.

**Evidence:** [verification-telemetry-receiver.md#h-telemetry-receiver-01](verification-telemetry-receiver.md#h-telemetry-receiver-01)

### F-203

**Location:** `telemetry-receiver/main.go:125-136` (a single `dec.Decode`, with no end-of-input check), measured against `telemetry-receiver/specs.md:14`, `:45` and `telemetry-receiver/README.md:21`.

**Control:** input validation and the 16 KiB body-size limit on `POST /ingest`.

**Repro / observation (how to confirm the control holds):**
1. Read `main.go:125-136`. `json.Decoder.Decode` returns as soon as the first complete JSON value has been read. `http.MaxBytesReader` raises its error only when a read goes past the limit, and for a small leading object the decoder never reads that far. Nothing after `Decode` checks that the body is exhausted.
2. POST a valid report followed by non-whitespace bytes, and a valid report followed by padding that takes the body over 16 KiB. On the current tree both get `204` and both are counted. A body whose oversized content sits inside the JSON value still gets `413`.

**Expected:** After decoding, the handler confirms the body holds nothing more, for example with a second `Decode` that must return `io.EOF`, reading far enough that `MaxBytesError` still maps to `413`. Trailing content gets `400`, and any body over 16 KiB gets `413`, as the spec and README say.

**Actual:** Only the first JSON value is validated. Trailing content, including content past the 16 KiB cap, is accepted with `204`.

**Evidence:** [verification-telemetry-receiver.md#h-telemetry-receiver-02](verification-telemetry-receiver.md#h-telemetry-receiver-02)

### F-211

**Location:** `mcp-server/main.go:8-17` (the package doc, layer 1), `mcp-server/specs.md:77-91` (the "Package boundary enforcement" bullet at `:91`) and `mcp-server/README.md:12-25`. The code that bounds the claim is `main.go:106-115` (`runServe` holds the `*rest.Config`) and `internal/kube/client.go:145-147` (the exported `NewFrom`).

**Control:** the MCP server's read-only guarantee. It has two layers: (1) tool handlers receive only a `*kube.Client`, whose exported methods are all List/Get-shaped; (2) a ClusterRole that grants only `get`/`list`/`watch`, plus `get` on `pods/log`.

**Repro / observation (how to confirm the control holds):**
1. RBAC, the authoritative layer: `kubectl auth can-i --list --as=system:serviceaccount:<release-ns>:gameplane-mcp-server` shows only `get`/`list`/`watch` on the seven `gameplane.local` resources and on core `pods` and `events`, and `get` on `pods/log`.
2. Code: a grep of `mcp-server/` for `.Create(`, `.Update(`, `.Patch(`, `.Delete(`, `.Apply(`, `UpdateStatus` and `DeleteCollection` finds nothing. `kube.Client` exports only `ListCRD`, `GetCRD`, `ListPods`, `GetPod`, `ListEvents` and `PodLogs`.
3. Scope of layer 1: read `main.go:106-115`. `runServe` itself calls `ctrl.GetConfig()`, so `package main` holds a full `*rest.Config`. Layer 1 therefore depends on the convention that handlers receive only the `*kube.Client` built from it. The unexported fields are not the only route to a clientset.

**Expected:** The layer-1 description says what the package boundary actually enforces, for example "tool handlers receive only a `*kube.Client`, which exposes no mutating method". RBAC is described as the only layer that stops mutation by any code in the process. `docs/architecture.md:306-316` already phrases it that way.

**Actual:** `main.go:13-15`, `specs.md:91` and `README.md:19-22` describe layer 1 as making mutation impossible for `package main` "even by mistake". That is broader than the boundary enforces. The guarantee holds today because of RBAC and the absence of mutating call sites, not because of the package boundary alone.

**Evidence:** [verification-mcp-server.md#h-mcp-server-01](verification-mcp-server.md#h-mcp-server-01)

### F-227

**Repro / observation (defensive; confirms whether the control holds)**
1. Read `api/cmd/main.go:260` and `web/nginx.conf.template:94-115`. `/metrics` sits on the root router with no auth, and nginx proxies every non-HTML request to it.
2. On a default install, request `/metrics` on the ingress host with no session. The response is Prometheus text format, not 401/403/404.

**Expected:** The public host does not serve Prometheus metrics to unauthenticated clients. The control is either an nginx rule (`location = /metrics { return 404; }`) or a separate metrics listener the ingress doesn't expose.

**Actual:** Nothing in the chart or nginx config keeps `/metrics` off the public host, so it is proxied to the API.

**Evidence:** [verification-charts-gameplane.md#c-charts-gameplane-h01](verification-charts-gameplane.md#c-charts-gameplane-h01)

### F-228

**Repro / observation (by reading code)**
1. `api/internal/audit/s3.go:71-75` sets `Secure: !cfg.Insecure` with no custom transport.
2. In minio-go v7.3.0, `getEndpointURL` picks `http` when `secure` is false.
3. `api/internal/audit/s3_test.go` tests only with `Insecure: true` against plain-HTTP servers.
4. Docs say the option "skips certificate verification"; the code means "plain HTTP".

**Expected:** The option does what the docs say (keep TLS, skip verification) or the docs say plainly "plain HTTP only".

**Actual:** The option disables TLS entirely, while docs describe it as skipping certificate verification only.

**Evidence:** [verification-charts-gameplane.md#c-charts-gameplane-h02](verification-charts-gameplane.md#c-charts-gameplane-h02)

### F-229

**Repro / observation (by reading code)**
1. `install.md:131-134` and `:147-148` say empty `groupsClaim` disables group mapping.
2. `api/internal/auth/oidc.go:110-116` shows empty claim becomes `"groups"`, so mapping is active whenever mappings are configured.
3. `values.yaml:247-248` and `security.md:699-700` state the real behaviour.

**Expected:** All docs say the same thing: empty `groupsClaim` defaults to reading the `groups` claim, and mapping is active when mappings exist.

**Actual:** `install.md` alone says empty `groupsClaim` disables mapping.

**Evidence:** [verification-charts-gameplane.md#c-charts-gameplane-h03](verification-charts-gameplane.md#c-charts-gameplane-h03)

### F-230

**Repro / observation (by reading code)**
1. `mtls.yaml` has no `helm.sh/hook` annotation; both Secrets are normal release resources.
2. Both Secrets always render under the fixed names `gameplane-agent-ca` and `gameplane-agent-client`, whatever `api.agentMTLS.*.name` says.
3. `grep -n agentMTLS charts/gameplane/templates/*.yaml` finds only the `.name` fields. The `.key` fields are never read.
4. Docs promise a bring-your-own path that fails at `helm install` with an ownership error.

**Expected:** The chart skips generating Secrets whose names were overridden, or docs describe the Helm ownership metadata needed.

**Actual:** The documented bring-your-own path fails at install, and four of six `api.agentMTLS` fields do nothing.

**Evidence:** [verification-charts-gameplane.md#c-charts-gameplane-h04](verification-charts-gameplane.md#c-charts-gameplane-h04)

### F-231

**Repro / observation (by rendering)**
1. `allow-agent-to-apiserver` (the default) admits RFC1918 ranges on TCP 443/6443 for `gameplane-game` pods.
2. `allow-game-public-egress` excludes private ranges from public egress.
3. For TCP 443/6443, the private-range exception is cancelled; game containers reach any private/link-local address on those ports.

**Expected:** Game pods reach only the apiserver on private addresses 443/6443. Either `apiServerCIDRs` gets a narrower default (e.g., resolved from the `kubernetes` Endpoints), or docs plainly say the default allows game containers to reach any private address on those ports.

**Actual:** With defaults, the anti-SSRF exception doesn't hold for TCP 443/6443.

**Evidence:** [verification-charts-gameplane.md#c-charts-gameplane-h05](verification-charts-gameplane.md#c-charts-gameplane-h05)

### F-235

**Repro / observation (by reading code)**
1. `up.sh:196` applies `https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml`.
2. `up.sh:28-31` forbids floating refs for MetalLB and pins `v0.14.9`.
3. CI is not affected (e2e.sh doesn't install ingress-nginx).

**Expected:** The ingress-nginx manifest URL names an immutable release tag (e.g. `controller-vX.Y.Z`), held in a variable next to `METALLB_VERSION`.

**Actual:** Each fresh `make dev-up` applies whatever upstream `main` holds at that moment.

**Evidence:** [verification-deploy.md#c-deploy-h01](verification-deploy.md#c-deploy-h01)

### F-246

**Location:**
- `.github/workflows/release.yaml:57-68` (build-push with `push: true` and every metadata tag), `:70-77` (key-presence check) and `:90-107` (keyed sign with 3 attempts). `:19-36` is the images `strategy`, which has no `fail-fast: false`.
- The same push-then-check order appears in `.github/workflows/images.yaml:61-75` and `:76-84` (common base, sign at `:94`), in `images.yaml:224-240` and `:242-250` (game images, sign at `:260`), and in `.github/workflows/publish-edge.yaml:87-99` and `:104-111` (sign at `:144`).

**Control:** Official tags point only at signed artifacts. It is stated at `release.yaml:68-69` ("Signing is mandatory for an official release: fail rather than publish unsigned images") and at `docs/contributing.md:146-148` ("Signing is **mandatory and fail-closed**").

**Repro / observation** (how to confirm the control holds):
1. Read `release.yaml:46-68`. The build step pushes every tag `docker/metadata-action` produced (`:54-56`). For a final tag such as `v0.3.0` that is `v0.3.0`, `0.3.0`, `0.3` and `latest`. The push happens before the "require signing key" step (`:70`) and the "sign image" step (`:90`) run.
2. Read `release.yaml:19-20`. The `strategy` block sets only `matrix`, so GitHub's default `fail-fast: true` applies: when one leg fails, the in-progress legs are cancelled, whatever step they are on.
3. From steps 1 and 2: a run that stops between push and sign leaves the tags on a digest with no signature until the job is re-run. That happens when signing exhausts its retries (`:97-107`) or when a sibling leg fails. The job still fails, so the gap is not silent, but publication comes before the check.
4. After every release run, including failed or cancelled ones, confirm with `cosign verify --key cosign.pub ghcr.io/valgulnecron/gameplane/<component>:<tag>` for all 12 components (`release.yaml:21-36`) and every tag that run created. Also confirm `cosign verify --key cosign.pub ghcr.io/valgulnecron/charts/gameplane:<version>` for the chart. Do the same for `:edge` after a failed `publish-edge` run.
5. After a fix, confirm by reading the order: the key check runs before any push step, and tags are attached only to a digest that has already been signed.

**Expected:** An official tag only ever resolves to a digest whose signature already exists. One approach is to push by digest, sign, and then attach tags. At minimum, the key check should run before any push, and the matrix should use `fail-fast: false` with a documented re-run policy. `docs/contributing.md:146-148` then holds at every point of a run, not just at its end.

**Actual:** Every image job fails closed, but only after it has published. The unsigned window lasts until someone re-runs the job. I did not observe this live, since that needs a failing release run. The digest in the window is still the project's own CI build, and a consumer who verifies gets a verification failure, not a false pass.

**Evidence:** [verification-github-workflows.md#h-github-workflows-01](verification-github-workflows.md#h-github-workflows-01)

### F-247

**Location:**
- `.github/workflows/publish-edge.yaml:30` (`workflow_dispatch: {}`), `.github/workflows/republish-modules.yaml:10-11` (`on: workflow_dispatch`) and `.github/workflows/images.yaml:4` (`workflow_dispatch:`). None of their jobs has an `environment:` or an `if: github.ref == …` guard (grep over `.github/workflows/`).
- Repository settings: Actions secrets `COSIGN_PRIVATE_KEY` and `COSIGN_PASSWORD` are repository-scoped, the only environment is `copilot`, and the only ruleset is `18692396` ("protect main", target `branch`).

**Control:** Signing-secret confinement. Spec 008 `spec.md:176` (Assumptions) says: "Production signing secrets (`COSIGN_PRIVATE_KEY`) remain strictly restricted to master branch and release tag workflows (`publish-edge.yaml`, `release.yaml`, `images.yaml`, `republish-modules.yaml`)". This complements the protected-master ruleset (one human approval, no self-approval).

**Repro / observation** (how to confirm the control holds):
1. Read the `on:` blocks of the three files. Each accepts `workflow_dispatch`, and GitHub lets the person dispatching choose the ref.
2. Run `grep -n 'environment:\|github.ref ==' .github/workflows/*.yaml`. It finds no job-level ref guard or environment on any signing job.
3. `gh api repos/ValgulNecron/Gameplane/actions/secrets --jq '.secrets[].name'` lists `COSIGN_PASSWORD` and `COSIGN_PRIVATE_KEY` at repository scope. `gh api repos/ValgulNecron/Gameplane/environments` lists only `copilot`. Repository-scoped secrets are not tied to a ref, so adding a ref guard to these three files is necessary but not sufficient. The setting that actually confines the key is environment scoping with a deployment policy.
4. `gh api repos/ValgulNecron/Gameplane/rulesets` returns a single branch-target ruleset. Nothing restricts who can create `v*` tags, and `release.yaml` signs whatever commit such a tag points at.
5. After a fix, confirm the following:
   - The `COSIGN_*` secrets are gone from the repository list and exist only in an environment (for example `release-signing`) whose deployment policy allows `refs/heads/master` and `refs/tags/v*` only (Settings → Environments).
   - Every signing job declares that environment.
   - A tag ruleset limits who can create `v*` tags.

**Expected:** The signing key is available only to runs on `master` or on a `v*` release tag, and this is enforced by repository configuration, as spec 008's assumption states.

**Actual:** Confinement relies on convention. Any account with write access can run the signing workflows against a ref other than master, and those runs get the key. Fork pull requests are unaffected, because they receive no secrets.

**Evidence:** [verification-github-workflows.md#h-github-workflows-02](verification-github-workflows.md#h-github-workflows-02)

### F-248

**Location:** `api/internal/handlers/clusters.go:192-236` (`delete`): the Secret name comes from the Cluster CR at `:213-220`, and the delete at `:230-233` has no label check. Create path for comparison: `clusters.go:130-145`, which sets `gameplane.local/cluster-kubeconfig=true`. Load-path guard: `api/internal/kube/loader.go:38-41`. Stated rule: `charts/gameplane/templates/api.yaml:129-144`, where the API Role grants `delete` on every Secret in the control-plane namespace and the comment names code as the real boundary. Documentation: `docs/security.md` ("Kubeconfig Secret handling", label-guard bullet, around `:609-620`), and `docs/install.md:442-492` (Path 1, kubectl apply with a user-chosen Secret name).

**Control:** the API's code-level boundary on control-plane Secrets. RBAC can't filter by label, so the API may delete only Secrets it is entitled to manage. A Cluster CR's reference must not make the API delete an arbitrary control-plane Secret: for example an auth-provider client secret, a notification-sink credential or a mod-registry key.

**Repro / observation** (defensive: confirms whether the control holds; by reading `213bdaa7`):
1. Read `clusters.go:203-220`. The handler loads the Cluster CR and takes `spec.kubeconfigSecret.name` as it is.
2. Read `clusters.go:230-233`. It deletes that name in `h.namespace` (the control-plane namespace) and ignores the result. It never reads the Secret's labels.
3. Read `kube/loader.go:38-41`. Loading the same reference refuses a Secret without `gameplane.local/cluster-kubeconfig=true`, with a comment saying the check exists "to prevent loading arbitrary secrets".
4. Read `charts/gameplane/templates/api.yaml:129-141`. For the other Secret-managing features, deletes also require `gameplane.local/managed-by=gameplane-api`, "so kubectl-/GitOps-created Secrets are never deleted over HTTP". The cluster create path (`clusters.go:133-139`) sets only the kubeconfig label, not `managed-by`.
5. How to confirm on a test install, as a `cluster:manage` holder, without touching real credentials. Create a throwaway Secret `audit018-unrelated` without the kubeconfig label in the control-plane namespace. Create a Cluster CR `audit018-probe` whose `spec.kubeconfigSecret.name` is `audit018-unrelated`. Call `DELETE /clusters/audit018-probe`. With the control in place, `audit018-unrelated` still exists afterwards. Clean up both objects.

**Expected:** `DELETE /clusters/{name}` deletes the referenced Secret only when it carries `gameplane.local/cluster-kubeconfig=true`, the same check the load path applies. Otherwise the CR is removed and the Secret is left in place. The maintainer decides whether a kubeconfig Secret created with kubectl or GitOps (Path 1) should be deleted over HTTP at all. If not, create should also set `managed-by=gameplane-api`, and delete should require it, as the other API-managed Secrets do.

**Actual:** the handler deletes whichever control-plane Secret the Cluster CR names, labelled or not, and discards any error.

**Evidence:** [verification-followup.md#followup-3](verification-followup.md#followup-3)

### F-249

**Location:** `api/internal/handlers/roles.go:160-238` (`update`). Only the built-in `admin` role is refused (`:171-176`). The permission set is replaced at `:205-220` with no user-management check. The guards this path doesn't apply are in `api/internal/handlers/users.go`: self-demotion `:517-524`, last user manager on role change `:525-542`, last user manager on delete `:449-463`. Counting: `api/internal/db/rbac.go:63-106` (`RoleGrantsUserManagement`, `UserManagesUsers` and `UserManagerCount` all join a user's primary role to `role_permissions`). Route gate: `api/internal/rbac/rbac.go:176-177` (`roles:manage`). Documentation: `docs/security.md:121-122` ("The API refuses to demote or delete the last user who can manage users, and refuses self-demotion below `users:manage`"). The OIDC re-evaluation path applies the same guard (`docs/security.md:712-718`).

**Control:** the user-administration lockout guards. No API action may leave the install with zero users who hold `users:manage`, and a caller may not remove their own `users:manage` access.

**Repro / observation** (defensive: confirms whether the control holds; by reading `213bdaa7`):
1. `users.go:511-542` checks, on a primary-role change, whether the new role still grants user management. It refuses self-demotion, and it refuses when the target is the last user manager.
2. `roles.go:183-220` validates only that each permission is in the catalog and isn't `*`. It then deletes and re-inserts the role's permissions. It never calls `RoleGrantsUserManagement`, `UserManagesUsers` or `UserManagerCount`.
3. `db/rbac.go:95-106` counts user managers through their primary role's current permissions. Removing `users:manage` from a role therefore removes every user whose primary role it is from the count at once. The `admin` role (`*`) can't be edited, so the count reaches zero only when no user has `admin` as their primary role. `users.go:517-524` allows that state, because a user may switch to a custom role that still grants `users:manage`.
4. How to confirm on a test install with throwaway objects only. Create a custom role `audit018-um` with `users:manage` and `roles:manage`, and a throwaway user `audit018-um-user` with that primary role. As that user, `PUT /roles/audit018-um` with a permission list that drops `users:manage`. With the control in place, the API answers 400, the same as the self-demotion refusal. The same request is also expected to fail whenever it would leave `UserManagerCount` at zero. Delete the throwaway user and role afterwards.

**Expected:** `roles.update` (and any future bulk role edit) applies the same lockout guards as `users.update`. It refuses a permission change that removes `users:manage` from the caller's own primary role. It also refuses one that would leave no user whose primary role grants `users:manage`.

**Actual:** a `roles:manage` holder can remove `users:manage` from any editable role, including their own primary role, and the change is applied even when it leaves no user who can manage users. Recovery then needs the `bootstrap-admin` break-glass command.

**Evidence:** [verification-followup.md#followup-4](verification-followup.md#followup-4)

### F-250

**Location:** `docs/security.md:271-274` (Network Capture Security Exception: "The game container retains its unprivileged posture: `runAsNonRoot: true`, `allowPrivilegeEscalation: false`, and no elevated capabilities. Only the capture sidecar holds `CAP_NET_RAW`; exploit of game code cannot grant packet-capture ability."). Code: `operator/internal/controller/gameserver_controller.go:1967` (game container `SecurityContext`), `:1971-1990` (`gameContainerSecurityContext` and its rationale), `:1503` and `:2074-2084` (`gamePodSecurityContext`, `fsGroup` only). For comparison: the agent container's fixed hardening at `:2183-2188`, and the capture sidecar's at `:1580-1611`. Related text: `docs/security.md:226-238` (managed pods are hardened; game pods are shaped per template; `podSecurity.enforceRestricted=true` is recommended for untrusted modules).

**Control:** the privilege posture of the game container, and the capture feature's documented isolation claim that only the sidecar can capture packets in a game pod. Operators rely on this text when they decide whether to enable capture and whether untrusted modules need extra admission policy.

**Repro / observation** (defensive: confirms whether the control holds; by reading `213bdaa7`):
1. Read `gameserver_controller.go:1981-1990`. With no `spec.security`, or one without `runAsUser`/`runAsGroup`, the game container gets no `securityContext`. With one, it gets only `runAsUser` and `runAsGroup`. Neither case sets `runAsNonRoot`, `allowPrivilegeEscalation: false` or a capability drop.
2. Read `:2078-2084`. The pod-level context carries only `fsGroup`, so no pod-level `runAsNonRoot` fills the gap.
3. Read `:1971-1980`. Leaving these fields unset is deliberate, so that arbitrary third-party game images keep working.
4. The shipped templates without a `spec.security` block (`dayz`, `dont-starve-together`, `enshrouded`, `garrys-mod`, `minecraft-java`, `rust`, `valheim`, `v-rising`) run the game as the image's own user.
5. How to confirm on a test install. For a throwaway GameServer from one of those templates, read the game container's `securityContext` with `kubectl get pod <name>-0 -o jsonpath` and expect it to be empty today. Then read `/proc/1/status` (`Uid`, `CapEff`, `CapBnd`, `NoNewPrivs`) in the game container. After the fix, either the docs match what these show, or the values show a non-root uid, `NoNewPrivs: 1` and no `NET_RAW` in the bounding set.
6. Related check for the maintainer, not rated here. The `restricted` Pod Security profile that `docs/security.md:236-238` recommends requires `allowPrivilegeEscalation: false`, `capabilities.drop: [ALL]`, `runAsNonRoot: true` and a seccomp profile on every container. Confirm on a test install whether game pods are admitted with `podSecurity.enforceRestricted=true`.

**Expected:** `docs/security.md:271-274` states what the operator actually sets. The game container runs as the template's `spec.security` uid/gid, or else as the image's default user. The operator adds no `runAsNonRoot`, no `allowPrivilegeEscalation: false` and no capability drop. The packet-capture isolation claim is limited to what that posture supports. Alternatively, if the maintainer wants the sentence to be the contract, the operator sets at least `allowPrivilegeEscalation: false` and drops `NET_RAW` on the game container, and the docs name which templates run non-root.

**Actual:** the docs claim `runAsNonRoot: true`, `allowPrivilegeEscalation: false` and no elevated capabilities for every game container. The operator sets none of these, and templates without `spec.security` run as whatever user the image declares.

**Evidence:** [verification-followup.md#followup-7](verification-followup.md#followup-7)

### F-256

**Location:** `api/internal/rbac/rbac.go:109-144`. The owner/collaborator fallback, with its owner-only check at `:128-136`, runs only when the namespace permission check at `:109` fails. A caller who holds the permission is served at `:144` without any ownership check. Related: the package comment at `:11-15`, and the rule at `:214` (`servers:write` for every non-GET `/servers` path). Role seed: `api/internal/db/migrations/003_roles.sql:40,48`, where the built-in `operator` role holds `servers:write`; no later migration changes that. Handlers that do no ownership check of their own: `api/internal/handlers/ownership.go:79-120` (`transfer`), `:125-230` (`setCollaborators`), `api/internal/handlers/lifecycle.go:57-92` (`wipeDataHandler`) and `api/internal/handlers/resources.go:291-322` (`deleteHandler`). For comparison, the share-link handlers do check ownership themselves (`isServerOwner`, `api/internal/handlers/shares.go:106,205,259,555-572`). Text that currently grants the permission: `docs/security.md:124-135` ("Only the owner and users holding the namespace `servers:write` permission can perform owner-only operations", `:133-134`), `api/internal/rbac/catalog.go:27` (`servers:write`: "Create, edit, delete, and control servers"), `api/specs.md:404-407`, and the dashboard hint "Requires owner or operator role" (`web/src/components/server/ServerActionsMenu.tsx:35-39,62-85`).

**Control:** per-server ownership. Four operations are owner-only: transferring ownership, editing the collaborator list, wiping data and deleting the server. The maintainer's HQ-001 answer says only the server's owner or an admin may perform them. Collaborators keep the access they have today, which excludes these four operations. Holding the namespace `servers:write` permission, through the built-in operator role or a custom role, is not enough on its own.

**Repro / observation** (defensive: confirms whether the control holds; by reading `c44cb179`, the RC-TAG-1 target, where these files are unchanged since `13a859ff`):
1. Read `rbac.go:109-143`. The owner-only check (`isOwnerOnly`, `:129-130`) sits inside `if !allow(...)`, so it runs only when the caller lacks the rule's permission.
2. Read `rbac.go:213-214` and `003_roles.sql:47-49`. `:transfer`, `:collaborators`, `:wipe-data` and `DELETE /servers/{name}` all require `servers:write`. The built-in operator role holds it cluster-wide by default: the migration backfills a `*` binding, and `POST /users` mirrors the primary role as a `*` binding.
3. Read the four handlers. Each one resolves the cluster and namespace, then patches or deletes the GameServer. None reads `gameplane.local/owner-id` or checks the caller's role. `setCollaborators` does fetch the server (`ownership.go:142-148`), but only to drop the owner's ID from the new list.
4. So a caller with namespace `servers:write` passes the middleware at `:109` and reaches these handlers for any server in that namespace, whether or not they own it.
5. To confirm on a test install, use throwaway objects only. Never use `audit018-rbac-test`: these calls are destructive when the control is missing.
   1. As `audit018-admin`, create a throwaway GameServer `audit018-hq001`. The admin is stamped as its owner.
   2. As `audit018-operator`, which is neither owner nor collaborator, make these calls in order:
      - `PUT /servers/audit018-hq001:collaborators` with its own user ID
      - `POST /servers/audit018-hq001:transfer` with its own user ID
      - `POST /servers/audit018-hq001:wipe-data` with `{"confirm":"audit018-hq001"}`
      - `DELETE /servers/audit018-hq001`
   3. With the control in place, each call answers 403, and the server's `gameplane.local/owner-id` and `gameplane.local/collaborators` annotations are unchanged. The same calls made as `audit018-admin` succeed.
   4. Delete the throwaway server afterwards.
6. Related check for the maintainer, not rated here (D18 in `fix-plan-held.md`): other namespace-gated writes can also reach a server the caller doesn't own. These include `PUT /servers/{name}`, lifecycle verbs, console and files, and a Restore onto the server with `backups:restore`. `docs/security.md:135` says backups and restores stay namespace-gated in this release.

**Expected:** `:transfer`, `:collaborators`, `:wipe-data` and `DELETE /servers/{name}` succeed only for the server's owner or an admin, whatever namespace permission the caller holds. A server with no owner annotation, for example one created with kubectl or GitOps, can then be managed this way only by an admin. `docs/security.md:124-135`, `api/specs.md:404-407`, the `servers:write` catalog label and the dashboard gates and hints all state that rule. D18 settles who counts as an admin, and whether the rule should cover more than these four operations.

**Actual:** any caller with namespace `servers:write` can transfer, change the collaborators of, wipe or delete any server in that namespace, whether or not they own it. This includes every holder of the built-in operator role. The docs, the catalog label and the dashboard currently describe this as allowed.

**Evidence:** [questions.md#hq-001-server-ownership-vs-the-operator-role-2026-09-24](questions.md#hq-001-server-ownership-vs-the-operator-role-2026-09-24)

### F-053

**Repro / observation**
1. When the feature is off, each function calls `r.Delete` on a fixed name with no cache read and no `IsControlledBy` check.
2. Take a server with no password config fields, no `configFiles`, no `backupPolicy` and no operator-managed RCON. With apiserver audit logging on, every agent heartbeat produces four `DELETE` calls that return 404.
3. If an object with one of those names exists and is not controlled by the GameServer, it is deleted.

**Expected:** The pattern already used in `reconcileNetworkPolicy`: read from the cache, and delete only when `metav1.IsControlledBy`.

**Actual:** Four live, unowned deletes per reconcile.

**Evidence:** [evidence/review-operator/verification.md#c-operator-10](evidence/review-operator/verification.md#c-operator-10)

### F-078

**Repro / observation**
1. Read `api/internal/audit/audit.go:728-739`: WriteSync enqueues Event{…, Reason: reason} to webhook/S3 sinks.
2. Read `audit.go:221-229` and `s3.go:231-247`: both drop the reason field; CSV export header (handlers/audit.go:76) has no reason column.
3. Log in via OIDC with a role mapping as a new user. The OIDC role assignment reason carries only in the DB row and JSON export, not webhook/S3.

**Expected:** webhook, S3 and stdout sinks all carry reason, per api/specs.md:338.

**Actual:** only stdout and JSON export carry it. For role-assignment events, reason is the only field recording which role was granted, so external consumers can't see privilege grants.

**Evidence:** [evidence/review-api/verification.md#c-api-05](evidence/review-api/verification.md#c-api-05)

### F-079

**Repro / observation**
1. An SSO user logs in and creates users, oidc_links, bindings and preferences rows.
2. Admin deletes the user via DELETE /users/{id}. Handler deletes users and bindings only; oidc_links and user_preferences survive.
3. The person logs in through IdP again. The join finds no user, tries to insert into oidc_links, hits the (issuer, subject) primary key, and gets 500 "login failed".

**Expected:** a deleted SSO user can be provisioned again on next login. users.go deletes a user's rows explicitly.

**Actual:** that IdP subject is locked out until someone deletes the orphaned oidc_links row by hand.

**Evidence:** [evidence/review-api/verification.md#c-api-06](evidence/review-api/verification.md#c-api-06)

### F-080

**Repro / observation**
1. Read `api/cmd/main.go:228-231`: AttachAuditWriteSyncFunc is called only on oidcAuth (Helm flag provider).
2. Read `api/internal/auth/registry.go:308-342`: providers from the auth config row are built with no audit function attached.
3. Add a dashboard-managed OIDC provider with role mapping admin: [ops]. Log in as a new user in group ops. GET /admin/audit shows no role assignment row.

**Expected:** FR-014 requires role assignments from OIDC mappings to be recorded, regardless of provider.

**Actual:** only Helm-flag OIDC providers are audited.

**Evidence:** [evidence/review-api/verification.md#c-api-07](evidence/review-api/verification.md#c-api-07)

### F-084

**Repro / observation**
1. Read `api/internal/handlers/tunnelcreds.go:136-141`: put merge-patches stringData, keeping the old provider's key.
2. Read `:225-231`: get ranges the tunnelProviderKeys map and returns the first match (random in Go).
3. Set frp credentials, then tailscale. The Secret now has both token and authKey. GET returns either one randomly.

**Expected:** switching provider doesn't leave the old provider's credential behind.

**Actual:** both keys stay in the Secret, and get returns random one.

**Evidence:** [evidence/review-api/verification.md#c-api-11](evidence/review-api/verification.md#c-api-11)

### F-146

**Repro / observation**
1. Read `netguard/specs.md:8` and `netguard/netguard.go:6-21`: both state "Used by both the operator and the agent via two deliberately different policies".
2. Grep for all netguard call sites in Go code outside the module itself: operator git/http module sources use `IsAllowed`; agent mod downloads use `IsPublic`; agent WebSocket RCON uses `IsAllowed` with loopback target; API notification sinks use `IsAllowed`; API Steam resolver uses `IsPublic`.
3. Check `CLAUDE.md:56,250` and `docs/dependencies.md:333-338`: all describe the split as operator → `IsAllowed`, agent → `IsPublic`, but they omit the API's use of both and the agent's `IsAllowed` call site.

**Expected:** The netguard spec (Purpose and References), package doc, `CLAUDE.md:56,250` and `docs/dependencies.md` name every importer and which policy it uses:
- operator git/http module sources: `IsAllowed`
- agent mod downloads: `IsPublic`
- agent WebSocket RCON: `IsAllowed`
- API notification sinks: `IsAllowed`
- API Steam resolver: `IsPublic`

**Actual:** The docs describe a strict two-way split. The code has two more call-site groups, with the API importing both policies and the agent using both.

**Evidence:** [evidence/review-netguard/verification.md#c-netguard-01](evidence/review-netguard/verification.md#c-netguard-01)

### F-194

**Repro / observation**
1. Build the bridge: `cd audit-syslog-bridge && go build -o /tmp/bridge .`.
2. Start a TCP listener on `127.0.0.1:25614`. On its first connection it reads one frame and then closes the connection. After that it keeps accepting new connections and prints every frame.
3. Run `LISTEN_ADDR=127.0.0.1:28614 SYSLOG_ADDR=127.0.0.1:25614 /tmp/bridge`. TCP is the default network.
4. Once `/healthz` answers, POST events about 0.5 s apart: `curl -s -o /dev/null -w '%{http_code}\n' -X POST --data '{"event":N}' http://127.0.0.1:28614/`.
5. Observed: the first delivered event arrived on connection 1, and the listener then closed that connection. The next POST returned `204`, but its frame never arrived anywhere. The POST after that also returned `204` and arrived on connection 2. The bridge logged nothing about the lost record or the reconnect.

**Expected:** README.md:32-35 says "TCP surfaces a dead collector as a `502` (→ API `failed`), which is the signal you want". A record sent on a connection the peer has already closed should either reach a fresh connection or get a `502`, so the API counts it as `failed`.

**Actual:** The first write after the peer closes succeeds locally, and the peer answers with RST. One record is lost every time the collector closes the connection, and both the bridge (`204`) and the API (`sent`) report it as delivered. Audit events are infrequent, so long idle gaps are normal, and the idle-reap case the comment is written for is exactly the one that loses a record.

**Evidence:** [evidence/review-audit-syslog-bridge/verification.md#c-audit-syslog-bridge-01](evidence/review-audit-syslog-bridge/verification.md#c-audit-syslog-bridge-01)
