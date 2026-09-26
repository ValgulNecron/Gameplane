# Procedures: API

Shared conventions: [conventions.md](conventions.md).

### public-healthz-get

**Preconditions**
None; unauthenticated public endpoint.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s $GP/healthz`

**Expected**
HTTP 200, response body `ok`.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### public-metrics-get

**Preconditions**
None; unauthenticated public endpoint. Prometheus endpoint, may be behind separate authentication in production.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s $GP/metrics | head -20`

**Expected**
HTTP 200, response begins with Prometheus-format lines (HELP, TYPE, metrics).

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### public-auth-providers-list

**Preconditions**
None; unauthenticated public endpoint used by the login page to discover enabled auth methods.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s $GP/auth/providers | jq`

**Expected**
HTTP 200, JSON object with `providers` array. Each entry has `name`, `kind` (local/oidc/google/github), `label`. No version/issuer/client-id exposed.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### public-auth-login

**Preconditions**
Local auth user exists (e.g., `audit018-admin:password`); audit018-admin role is admin.

**Resources created**
none (session cookies only)

**Steps**
1. Login cost: 1 (to audit018-admin). Run: `curl -s -D headers.txt -H "Content-Type: application/json" -d '{"username":"audit018-admin","password":"<password>"}' $GP/auth/login | jq`
2. Capture the session from the response headers — per conventions.md, curl's own cookie jar (-c/-b) silently drops Secure-flagged cookies over plain HTTP, so extract them from -D output instead: `grep -i "^set-cookie:" headers.txt | grep gameplane_session > ~/gameplane-audit-018/session-admin.txt && grep -i "^set-cookie:" headers.txt | grep gameplane_csrf >> ~/gameplane-audit-018/session-admin.txt && chmod 600 ~/gameplane-audit-018/session-admin.txt`

**Expected**
HTTP 200, JSON with user object (id, username, role). Response Set-Cookie headers include `gameplane_session` and `gameplane_csrf` cookies (both HttpOnly, Secure).

**Cleanup**
Remove `headers.txt`. Keep `~/gameplane-audit-018/session-admin.txt` — later procedures in this round reuse it.

**Automatable?**
yes (api-auth)

---

### public-auth-logout

**Preconditions**
Authenticated session established (from public-auth-login).

**Resources created**
none

**Steps**
1. Login cost: 0 (reuse session). Run: `curl -X POST -b "gameplane_session=...;gameplane_csrf=..." -H "X-Gameplane-CSRF: ..." $GP/auth/logout`

**Expected**
HTTP 204 (No Content). Session is invalidated.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### public-auth-oidc-provider-start

**Preconditions**
OIDC provider is configured (via Helm flag or admin settings). Provider name is known (e.g., "helm" for Helm-seeded, or "custom" if dashboard-added).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -L -s $GP/auth/oidc/custom/start 2>&1 | head -20`

**Expected**
If provider exists and is enabled: HTTP 302 redirect to the IdP's authorize endpoint (Location header).
If provider doesn't exist or is disabled: HTTP 404.

**Cleanup**
none

**Automatable?**
yes (api-auth) - note: full OIDC flow requires external IdP; this tests discovery only.

---

### public-auth-oidc-callback

**Preconditions**
OIDC provider configured; external IdP or mock IdP that can issue code/state.

**Resources created**
none

**Steps**
1. Login cost: 0. This is a callback from the IdP; not directly testable without an IdP redirect. Run: `curl -s "$GP/auth/oidc/custom/callback?code=MOCK&state=MOCK" | head -20` (will fail with invalid code)

**Expected**
HTTP 4xx or 5xx (invalid code is expected without real IdP). A real OIDC flow returns HTTP 302 + Set-Cookie on success.

**Cleanup**
none

**Automatable?**
no (blocked: requires external IdP)

---

### public-shares-resolve

**Preconditions**
Run after shares-create and before its Cleanup, which revokes the link and removes `~/gameplane-audit-018/share-create.json`. That off-git file (mode 600) holds the share link creation response, including the 43-character token (32 random bytes in unpadded base64url, `generateShareLinkToken` in `api/internal/db/shares.go`). The link is for audit018-server-from-template and is not expired or revoked. The route is public: no session and no CSRF header.

**Resources created**
none

**Steps**
1. Login cost: 0. Load the token into a shell variable and print only its length. The file path is jq's only argument, so the token is never on a command line: `SHARE_TOKEN=$(jq -r .token ~/gameplane-audit-018/share-create.json); echo "token length ${#SHARE_TOKEN}"` (expect 43).
2. The route takes the token as a path segment (`GET /shares/{token}`). Give curl the URL as config on stdin (`-K -`). `printf` is a shell builtin, so the token stays out of every process's argv and out of shell history: `printf 'url = "%s/shares/%s"\n' "$GP" "$SHARE_TOKEN" | curl -s -K - -w '%{http_code}\n'`
3. `unset SHARE_TOKEN`. Evidence writes the path as `/shares/<token>`, never the value.

**Expected**
HTTP 200 (the last output line, after the body). The body is a JSON object with `serverName` (`audit018-server-from-template`) and `status` (the server's `status.phase`, or `Unknown` when it has none). It also has `address` (`{host, port}` of the first non-private endpoint, tunnel endpoints first) and `playersOnline` (from `status.agent.playersOnline`), each only when the server reports it. Nothing else is returned: no namespace, cluster, owner, collaborators, version or token (`sharePublicResp` in `api/internal/handlers/shares.go`). An unknown, expired or revoked token, or a server that no longer exists, gets HTTP 404 with body `{"error":"not found"}`, so a probe cannot tell these cases apart.

**Cleanup**
none

**Automatable?**
yes (api-roles): OD-015 is resolved, and a test that creates its own server owns it, so it can create a share link (as in shares-create) and resolve it in the same run. No e2e test covers share links yet.

---

### public-shares-start

**Preconditions**
Run after shares-create and before its Cleanup, which revokes the link and removes `~/gameplane-audit-018/share-create.json`. The link in that file was created with `canStart: true` and is not expired or revoked. The route is public: no session and no CSRF header.

**Resources created**
none

**Steps**
1. Login cost: 0. Load the token as in public-shares-resolve, printing only its length: `SHARE_TOKEN=$(jq -r .token ~/gameplane-audit-018/share-create.json); echo "token length ${#SHARE_TOKEN}"` (expect 43).
2. Send the POST with the URL as curl config on stdin, so the token is never on a command line: `printf 'url = "%s/shares/%s/start"\n' "$GP" "$SHARE_TOKEN" | curl -s -o /dev/null -w '%{http_code} %{size_download}\n' -X POST -K -`
3. `unset SHARE_TOKEN`. Evidence writes the path as `/shares/<token>/start`, never the value.

**Expected**
HTTP 202 (Accepted) with an empty body, so the output line is `202 0`. The handler only stamps the `gameplane.local/idle-wake-requested` annotation on audit018-server-from-template (as `:wake` does) and writes no JSON: there is no console URL, dashboard URL or redirect. A link created with `canStart: false` gets HTTP 404 with body `{"error":"not found"}` (the line reads `404 22`), the same response as an unknown, expired or revoked token, so a caller cannot tell whether the link exists.

**Cleanup**
none

**Automatable?**
yes (api-roles): shares-create's link has `canStart: true`, and a test that creates its own server owns it (OD-015 resolved), so it can create the link and start the server through it in the same run. No e2e test covers share links yet.

---

### servers-list

**Preconditions**
Authenticated as audit018-viewer or higher. At least one server exists in the cluster (e.g., one of the pre-existing audit servers: mc-fabric, soak-pool-west, etc.).

**Resources created**
none

**Steps**
1. Login cost: 1. Run: `curl -s -H "Cookie: gameplane_session=...; gameplane_csrf=..." -H "X-Gameplane-CSRF: ..." -b ~/gameplane-audit-018/session-viewer.txt $GP/servers | jq '.items | length'`

**Expected**
HTTP 200, JSON array of GameServer objects. Can filter by `?namespace=...` or `?cluster=...`.

**Cleanup**
none

**Automatable?**
yes (api-rbac)

---

### servers-create

**Preconditions**
Authenticated as audit018-operator or higher. GameTemplate exists (e.g., `mc-fabric`, which is pre-existing). Namespace is `gameplane-games` or audit018-namespace.

**Resources created**
audit018-server-from-template (GameServer in gameplane-games namespace).

**Steps**
1. Login cost: 1. Create JSON payload:
   ```json
   {
     "apiVersion": "gameplane.local/v1alpha1",
     "kind": "GameServer",
     "metadata": {"name": "audit018-server-from-template", "namespace": "gameplane-games"},
     "spec": {
       "templateRef": {"name": "mc-fabric"},
       "suspend": false,
       "storage": {"size": "5Gi"}
     }
   }
   ```
2. Run: `curl -X POST -H "Content-Type: application/json" -d @payload.json -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." $GP/servers | jq '.metadata.name'`

**Expected**
HTTP 201, response contains created GameServer object with name `audit018-server-from-template`.

**Cleanup**
DELETE /servers/audit018-server-from-template

**Automatable?**
yes (api-rbac)

---

### servers-get

**Preconditions**
Authenticated as audit018-viewer. audit018-server-from-template (or any audit server) exists.

**Resources created**
none

**Steps**
1. Login cost: 0 (reuse session). Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." $GP/servers/audit018-server-from-template | jq '.metadata.name'`

**Expected**
HTTP 200, JSON GameServer object with name=`audit018-server-from-template`.

**Cleanup**
none

**Automatable?**
yes (api-rbac)

---

### servers-update

**Preconditions**
Authenticated as audit018-operator. audit018-server-from-template exists. User has servers:write permission on the server or namespace.

**Resources created**
none (in-place mutation)

**Steps**
1. Login cost: 0. GET the current server (servers-get above) to extract current spec.
2. Modify spec.suspend to true, save as updated.json.
3. Run: `curl -X PUT -H "Content-Type: application/json" -d @updated.json -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." $GP/servers/audit018-server-from-template | jq '.spec.suspend'`

**Expected**
HTTP 200, response shows updated object with spec.suspend=true.

**Cleanup**
Update spec.suspend back to false.

**Automatable?**
yes (api-rbac)

---

### templates-list

**Preconditions**
Authenticated as audit018-viewer. At least one template exists (e.g., `mc-fabric` is pre-existing and cluster-scoped).

**Resources created**
none

**Steps**
1. Login cost: 0 (reuse session). Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." $GP/templates | jq '.items | map(.metadata.name)'`

**Expected**
HTTP 200, JSON array of cluster-scoped GameTemplate objects.

**Cleanup**
none

**Automatable?**
yes (api-rbac)

---

### templates-get

**Preconditions**
Authenticated as audit018-viewer. Template `mc-fabric` exists.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." $GP/templates/mc-fabric | jq '.metadata.name'`

**Expected**
HTTP 200, GameTemplate object with name=`mc-fabric`.

**Cleanup**
none

**Automatable?**
yes (api-rbac)

---

### backups-list

**Preconditions**
Authenticated as audit018-viewer. At least one backup exists for a server in the target namespace (or none, which is valid).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." "$GP/backups?namespace=gameplane-games" | jq '.items | length'`

**Expected**
HTTP 200, JSON array of GameBackup objects (may be empty).

**Cleanup**
none

**Automatable?**
yes (api-rbac)

---

### schedules-list

**Preconditions**
Authenticated as audit018-viewer. May or may not have schedules.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." "$GP/schedules?namespace=gameplane-games" | jq '.items | length'`

**Expected**
HTTP 200, JSON array (may be empty).

**Cleanup**
none

**Automatable?**
yes (api-rbac)

---

### restores-list

**Preconditions**
Authenticated as audit018-viewer. May or may not have restores in progress.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." "$GP/restores?namespace=gameplane-games" | jq '.items | length'`

**Expected**
HTTP 200, JSON array (may be empty).

**Cleanup**
none

**Automatable?**
yes (api-rbac)

---

### audit-read-paginated

**Preconditions**
Authenticated as audit018-admin (audit:read permission). At least one audit event exists (e.g., from login above).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." "$GP/admin/audit?limit=10" | jq 'length'`

**Expected**
HTTP 200, JSON array of audit events (newest first, capped at `limit`). Pass the oldest returned event's `id` as `?before=` on the next call to page further back.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### audit-verify-chain

**Preconditions**
Authenticated as audit018-admin. Audit events exist and chain is intact (no tampering).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/admin/audit/verify | jq '.ok'`

**Expected**
HTTP 200, JSON with `ok: true` and `checked` (row count) if the chain is intact, or `ok: false` with `firstBadId` and `message` naming the first broken link.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### audit-export-csv

**Preconditions**
Authenticated as audit018-admin. At least one audit event exists.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." "$GP/admin/audit/export?format=csv" -D /tmp/export.txt | head -5`

**Expected**
HTTP 200, Content-Type: text/csv. File begins with CSV header row: `id,ts,actor,method,path,target,status,ip`.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### audit-export-json

**Preconditions**
Authenticated as audit018-admin. At least one audit event exists.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." "$GP/admin/audit/export?format=json" | jq '.[0].actor'`

**Expected**
HTTP 200, Content-Type: application/json. Response is a JSON array of event objects.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### config-read-all

**Preconditions**
Authenticated as audit018-admin (config:read). Admin settings exist (at minimum, empty sections).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/admin/config | jq 'keys | length'`

**Expected**
HTTP 200, JSON object with config sections (auth, notifications, registries, etc.). May be empty or partially filled.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### config-write-section

**Preconditions**
Authenticated as audit018-admin (config:manage). Testing with a benign section like `notifications` or a custom test section.

**Resources created**
none (mutation of existing config section)

**Steps**
1. Login cost: 0. Create a test config JSON: `{"sinks": []}`
2. Run: `curl -X PUT -H "Content-Type: application/json" -d '{"sinks":[]}' -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/admin/config/notifications | jq '.sinks | length'`

**Expected**
HTTP 200, JSON response confirms the update.

**Cleanup**
Restore previous config or revert the test section.

**Automatable?**
yes (api-auth)

---

### auth-provider-secret-put

**Preconditions**
Authenticated as audit018-admin (config:manage). OIDC provider "test-oidc" does not exist yet (or will be created).

**Resources created**
Secret in gameplane-system namespace: `gameplane-auth-test-oidc` (labeled).

**Steps**
1. Login cost: 0. Run: `curl -X PUT -H "Content-Type: application/json" -d '{"clientSecret":"test-secret-value"}' -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/admin/auth/providers/test-oidc/secret | jq '.name'`

**Expected**
HTTP 200, JSON response with `name: gameplane-auth-test-oidc` and `keys: ["clientSecret"]`.

**Cleanup**
DELETE /admin/auth/providers/test-oidc/secret (to remove the Secret).

**Automatable?**
yes (api-auth)

---

### auth-provider-secret-delete

**Preconditions**
Authenticated as audit018-admin (config:manage). Secret exists (from auth-provider-secret-put).

**Resources created**
none (deletion)

**Steps**
1. Login cost: 0. Run: `curl -X DELETE -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/admin/auth/providers/test-oidc/secret`

**Expected**
HTTP 204 (No Content) or HTTP 200. Secret is deleted.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### notifications-test-sink

**Preconditions**
Authenticated as audit018-admin (config:manage). A notification sink is configured (e.g., webhook, email). **Deferred to T031 for actual delivery testing due to login budget and external dependency.**

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -X POST -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/admin/notifications/sinks/webhook/test | jq '.delivered'`

**Expected**
HTTP 200, JSON `{"delivered": true}` if sink is configured and reachable; HTTP 4xx/5xx otherwise.

**Cleanup**
none

**Automatable?**
deferred to T031 (requires external sink configuration + delivery validation)

---

### notifications-secret-put

**Preconditions**
Authenticated as audit018-admin (config:manage). Sink name is valid (e.g., "webhook", "email").

**Resources created**
Secret: `gameplane-notify-webhook` in gameplane-system.

**Steps**
1. Login cost: 0. Run: `curl -X PUT -H "Content-Type: application/json" -d '{"kind":"webhook","url":"https://example.invalid/audit018-webhook","authorization":"test-webhook-auth"}' -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/admin/notifications/sinks/webhook/secret | jq '.name'`

**Expected**
HTTP 200, JSON with secret metadata.

**Cleanup**
DELETE /admin/notifications/sinks/webhook/secret

**Automatable?**
yes (api-auth)

---

### registry-secret-put

**Preconditions**
Authenticated as audit018-admin (config:manage). Registry provider is known (e.g., "curseforge").

**Resources created**
Secret: `gameplane-modreg-curseforge` in gameplane-system.

**Steps**
1. Login cost: 0. Run: `curl -X PUT -H "Content-Type: application/json" -d '{"apiKey":"test-key-123"}' -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/admin/registries/curseforge/secret | jq '.name'`

**Expected**
HTTP 200, JSON with registry secret reference.

**Cleanup**
DELETE /admin/registries/curseforge/secret

**Automatable?**
yes (api-auth)

---

### capture-enable

**Preconditions**
Authenticated as audit018-admin (captures:manage). Server exists (e.g., mc-fabric). Network capture feature is enabled (--capture-enabled).

**Resources created**
none (annotation on server, sidecar injection by operator)

**Steps**
1. Login cost: 0. Run: `curl -X POST -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/mc-fabric:capture-enable?namespace=gameplane-games" | jq '.phase'`

**Expected**
HTTP 200, JSON response indicates capture feature is enabled on the server.

**Cleanup**
POST /servers/mc-fabric:capture-disable

**Automatable?**
yes (api-rbac) if capture feature is enabled; else 501 Not Implemented.

---

### capture-start

**Preconditions**
Authenticated as audit018-admin (captures:manage). Server is running and capture is enabled.

**Resources created**
PacketCapture CRD in server's namespace; BPF sidecar container deployed.

**Steps**
1. Login cost: 0. Create request: `{"filter":"","maxDurationSeconds":60,"maxSizeBytes":104857600,"ttlSecondsAfterFinished":3600}`
2. Run: `curl -X POST -H "Content-Type: application/json" -d @request.json -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/mc-fabric:capture-start?namespace=gameplane-games" | jq '.captureId'`

**Expected**
HTTP 200, JSON with `captureId`, `phase`, `createdAt`. Capture begins collecting packets.

**Cleanup**
POST /servers/mc-fabric:capture-stop (to stop early) or wait for TTL.

**Automatable?**
deferred to T031 (network capture is optional, requires feature flag and operator support)

---

### capture-list

**Preconditions**
Authenticated as audit018-admin (captures:manage). At least one capture exists for the server.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/mc-fabric:captures?namespace=gameplane-games" | jq '.captures | length'`

**Expected**
HTTP 200, JSON object with a `captures` array (plus `total`, `limit`, `offset`), not a bare `items` list.

**Cleanup**
none

**Automatable?**
deferred to T031

---

### servers-start

**Preconditions**
Authenticated as audit018-operator (servers:write). Server audit018-server-from-template exists and is suspended.

**Resources created**
none (mutation of spec.suspend annotation)

**Steps**
1. Login cost: 0. Run: `curl -s -o /dev/null -w '%{http_code}\n' -X POST -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/audit018-server-from-template:start?namespace=gameplane-games"`
2. Confirm the patch applied: `curl -s -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/audit018-server-from-template?namespace=gameplane-games" | jq '.spec.suspend'`

**Expected**
HTTP 202 (Accepted), empty body — patchSuspend only stamps spec.suspend and returns; there is no response JSON to inspect. The follow-up GET shows spec.suspend=false once the patch applies.

**Cleanup**
none (server is now starting)

**Automatable?**
yes (api-rbac)

---

### servers-stop

**Preconditions**
Authenticated as audit018-operator (servers:write). Server exists and is running.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -o /dev/null -w '%{http_code}\n' -X POST -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/audit018-server-from-template:stop?namespace=gameplane-games"`
2. Confirm the patch applied: `curl -s -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/audit018-server-from-template?namespace=gameplane-games" | jq '.spec.suspend'`

**Expected**
HTTP 202 (Accepted), empty body. The follow-up GET shows spec.suspend=true once the patch applies.

**Cleanup**
none

**Automatable?**
yes (api-rbac)

---

### servers-restart

**Preconditions**
Authenticated as audit018-operator (servers:write). Server exists.

**Resources created**
none (operator-reconciled annotation)

**Steps**
1. Login cost: 0. Run: `curl -s -o /dev/null -w '%{http_code}\n' -X POST -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/audit018-server-from-template:restart?namespace=gameplane-games"`

**Expected**
HTTP 202 (Accepted), empty body — restartHandler only stamps a restart-request annotation and returns. The operator drains and recreates the pod asynchronously.

**Cleanup**
none

**Automatable?**
yes (api-rbac)

---

### servers-clone

**Preconditions**
Authenticated as audit018-operator (servers:write). Source server exists (e.g., mc-fabric). New name does not exist (e.g., audit018-cloned-server).

**Resources created**
audit018-cloned-server (new GameServer, cloned from mc-fabric).

**Steps**
1. Login cost: 0. Create request: `{"newName":"audit018-cloned-server"}`
2. Run: `curl -X POST -H "Content-Type: application/json" -d @request.json -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/mc-fabric:clone?namespace=gameplane-games" | jq '.metadata.name'`

**Expected**
HTTP 200, JSON with new server object (name=audit018-cloned-server).

**Cleanup**
DELETE /servers/audit018-cloned-server

**Automatable?**
yes (api-rbac)

---

### servers-wipe-data

**Preconditions**
Authenticated as audit018-operator (servers:write, owner-only). Server exists.

**Resources created**
none (Job to empty data PVC, initiated by operator).

**Steps**
1. Login cost: 0. Create request: `{"confirm":"audit018-server-from-template"}`
2. Run: `curl -s -o /dev/null -w '%{http_code}\n' -X POST -H "Content-Type: application/json" -d @request.json -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/audit018-server-from-template:wipe-data?namespace=gameplane-games"`

**Expected**
HTTP 202 (Accepted), empty body — wipeDataHandler only stamps a wipe-request annotation and suspends the server. The operator empties the data PVC asynchronously.

**Cleanup**
none

**Automatable?**
yes (api-rbac)

---

### shares-create

**Preconditions**
Authenticated as audit018-operator, the account that created audit018-server-from-template in servers-create. That call stamped the server's `gameplane.local/owner-id` annotation with the operator's user id, and the share handler (`isServerOwner` in `api/internal/handlers/shares.go`) accepts only the user whose id matches it. Role does not count, so even audit018-admin gets 403. Run this before servers-transfer, which moves ownership to audit018-admin.

**Resources created**
One share link row in the API database (`share_links` table), attached to audit018-server-from-template. There is no ServerShare CRD. The API picks the row's random `id`, so it cannot carry an audit018- name.

**Steps**
1. Login cost: 0. Create the request with an expiry 7 days out (the handler rejects a past `expiresAt` with 400): `printf '{"expiresAt":"%s","canStart":true}' "$(date -u -d '+7 days' +%Y-%m-%dT%H:%M:%SZ)" > request.json`
2. Run (the response holds the token, so it goes to an off-git file with mode 600): `(umask 077; curl -s -o ~/gameplane-audit-018/share-create.json -w '%{http_code}\n' -X POST -H "Content-Type: application/json" -d @request.json -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/audit018-server-from-template:shares?namespace=gameplane-games")`
3. Show the result without the token: `jq '{id, createdAt, expiresAt, canStart, tokenLength: (.token | length)}' ~/gameplane-audit-018/share-create.json`. Never print the token or copy it into evidence; write `<token>` instead.

**Expected**
HTTP 200: the handler writes the JSON without setting 201. The object has `id`, `createdAt`, `expiresAt` (7 days out), `canStart: true` and `tokenLength: 43`, because the token is 32 random bytes in unpadded base64url and is returned only in this response. A 403 `forbidden` means the session is not the server's owner (see Preconditions).

**Cleanup**
Run this after public-shares-resolve and public-shares-start, which read the token from `share-create.json`, and before servers-transfer, after which audit018-operator no longer owns the server and the revoke returns 403. As audit018-operator: DELETE /servers/audit018-server-from-template/shares/{id}?namespace=gameplane-games, with `id` from `share-create.json`. It is owner-only and returns 204; it revokes the link, which shares-list still shows. Then `rm ~/gameplane-audit-018/share-create.json`.

**Automatable?**
yes (api-roles): OD-015 is resolved, and the caller owns the server it creates (servers-create stamps `gameplane.local/owner-id`), so a test can create its own server and share link in the same run. No e2e test covers share links yet.

---

### shares-list

**Preconditions**
Authenticated as audit018-operator, owner of audit018-server-from-template. The list handler runs the same owner check as shares-create (`gameplane.local/owner-id` must equal the caller's user id), so run this before servers-transfer. At least one share link exists for that server (from shares-create; a revoked link is still listed).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/audit018-server-from-template:shares?namespace=gameplane-games" | jq 'length, (map(has("token")) | any)'`

**Expected**
HTTP 200, a bare JSON array (no `items` wrapper) of share link objects with `id`, `createdAt`, `expiresAt` and `canStart`. The first printed line is at least 1. The second is `false`, because the list never includes `token`. Revoked links are listed too, with no revoked field. A 403 `forbidden` means the session is not the server's owner.

**Cleanup**
none

**Automatable?**
yes (api-roles): same owner check as shares-create, and OD-015 no longer blocks it, because audit018-operator owns the server it created in servers-create. No e2e test covers share links yet.

---

### servers-collaborators-set

**Preconditions**
Authenticated as owner of audit018-server-from-template, which is audit018-operator (it created the server in servers-create). Run this before servers-transfer, which moves ownership to audit018-admin. Collaborator users exist.

**Resources created**
none (collaborators annotation mutation)

**Steps**
1. Login cost: 0. Create request: `{"userIds":[<user-id>]}`
2. Run: `curl -s -o /dev/null -w '%{http_code}\n' -X PUT -H "Content-Type: application/json" -d @request.json -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/audit018-server-from-template:collaborators?namespace=gameplane-games"`
3. Confirm: `curl -s -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/audit018-server-from-template?namespace=gameplane-games" | jq '.metadata.annotations."gameplane.local/collaborators"'`

**Expected**
HTTP 204 (No Content), empty body — setCollaborators has no response JSON. The follow-up GET shows the collaborators annotation updated.

**Cleanup**
Set collaborators to empty array.

**Automatable?**
yes (api-roles): OD-015 is resolved and audit018-operator owns the server it created in servers-create. TestAPI_OwnerCollaboratorAccess already has the owner set collaborators (expects 204).

---

### servers-transfer

**Preconditions**
Authenticated as owner of audit018-server-from-template (operator role, owner-only). Recipient user exists (e.g., audit018-admin).

**Resources created**
none (ownership annotation mutation)

**Steps**
1. Login cost: 0. Create request: `{"userId": <audit018-admin user id>}`
2. Run: `curl -s -o /dev/null -w '%{http_code}\n' -X POST -H "Content-Type: application/json" -d @request.json -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/audit018-server-from-template:transfer?namespace=gameplane-games"`
3. Confirm: `curl -s -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/audit018-server-from-template?namespace=gameplane-games" | jq '.metadata.annotations."gameplane.local/owner"'`

**Expected**
HTTP 204 (No Content), empty body — the transfer handler has no response JSON. The follow-up GET shows the owner annotation updated.

**Cleanup**
none. servers-delete, the next procedure, deletes audit018-server-from-template as audit018-operator. That DELETE is allowed by the operator role's `servers:write` (`api/internal/db/migrations/003_roles.sql`), not by ownership, so the server does not go back to audit018-operator first. Its share link was already revoked in shares-create's Cleanup.

**Automatable?**
yes (api-roles): OD-015 is resolved and audit018-operator owns the server it created in servers-create. TestAPI_OwnerCollaboratorAccess already covers `:transfer` (expects 204).

### servers-delete

**Preconditions**
Authenticated as audit018-operator. audit018-server-from-template exists. User has servers:write permission (or is owner). Run this after every other procedure that uses audit018-server-from-template (servers-get through servers-transfer), because it removes the server.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -X DELETE -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." $GP/servers/audit018-server-from-template`

**Expected**
HTTP 204 (No Content). Server is deleted.

**Cleanup**
none (server is now gone)

**Automatable?**
yes (api-rbac)

---

### users-list

**Preconditions**
Authenticated as audit018-admin (users:read).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/users | jq 'length'`

**Expected**
HTTP 200, JSON array of user objects.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### users-create

**Preconditions**
Authenticated as audit018-admin (users:manage). `audit018-tmpuser` does not exist. It is a throwaway account used only here and in users-delete. Round setup already creates audit018-collab, which the later users-* procedures use.

**Resources created**
User: audit018-tmpuser in database (role viewer, no password), plus the cluster-wide viewer binding the handler mirrors from its role.

**Steps**
1. Login cost: 0. Create request: `{"username":"audit018-tmpuser","displayName":"Audit temp user","email":"tmpuser@audit.local","role":"viewer"}`
2. Run: `curl -X POST -H "Content-Type: application/json" -d @request.json -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/users | jq '{id, username, role, provider}'`. Keep the printed `id` for users-delete.

**Expected**
HTTP 200 (the create handler writes JSON without setting 201). The user object has `id`, `username: audit018-tmpuser`, `role: viewer` and `provider: pending`, because no password was sent.

**Cleanup**
users-delete removes audit018-tmpuser. If users-delete is not run, send its DELETE as audit018-admin with the `id` from step 2.

**Automatable?**
yes (api-auth)

---

### users-me

**Preconditions**
Authenticated as audit018-viewer.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." $GP/users/me | jq '.username'`

**Expected**
HTTP 200, JSON with current user object including `permissions` map and `preferences`.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### users-preferences-get

**Preconditions**
Authenticated as any user.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." $GP/users/me/preferences | jq '.themeType'`

**Expected**
HTTP 200, JSON with `themeType`, `presetId`, `appearanceMode`, `customColors`, `customCssEnabled`, `customCss`, `updatedAt` (defaults if never customized).

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### users-preferences-put

**Preconditions**
Authenticated as audit018-viewer.

**Resources created**
none (user_preferences row in DB)

**Steps**
1. Login cost: 0. Create request: `{"themeType":"preset","presetId":"pink","appearanceMode":"dark","customCssEnabled":false}`
2. Run: `curl -X PUT -H "Content-Type: application/json" -d @request.json -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." $GP/users/me/preferences | jq '.appearanceMode'`

**Expected**
HTTP 200, JSON shows updated preferences.

**Cleanup**
Reset to defaults (POST /users/me/preferences/reset).

**Automatable?**
yes (api-auth)

---

### users-preferences-reset

**Preconditions**
Authenticated as audit018-viewer. Preferences have been customized.

**Resources created**
none (deletion of user_preferences row)

**Steps**
1. Login cost: 0. Run: `curl -X POST -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." $GP/users/me/preferences/reset | jq '.theme'`

**Expected**
HTTP 200, preferences are reset to system defaults.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### users-get

**Preconditions**
Authenticated as audit018-admin (users:read). audit018-collab exists (from round setup; `<collab-id>` is its id).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/users/<collab-id> | jq '.username'`

**Expected**
HTTP 200, JSON user object.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### users-update

**Preconditions**
Authenticated as audit018-admin (users:manage). audit018-collab exists (from round setup). Before step 1, note its current `role` and `displayName`, which Cleanup restores: `curl -s -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/users/<collab-id> | jq '{role, displayName}'`

**Resources created**
none (mutation)

**Steps**
1. Login cost: 0. Create request: `{"displayName":"Updated Collab","role":"operator"}`
2. Run: `curl -X PATCH -H "Content-Type: application/json" -d @request.json -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/users/<collab-id> | jq '.role'`

**Expected**
HTTP 200, JSON shows role=operator.

**Cleanup**
PATCH `role` and `displayName` back to the values noted in Preconditions.

**Automatable?**
yes (api-auth)

---

### users-reset-password

**Preconditions**
Authenticated as audit018-admin (users:manage). audit018-collab exists (from round setup).

**Resources created**
none (replaces audit018-collab's password hash in the DB and ends that user's sessions; the API generates and returns no password)

**Steps**
1. Login cost: 0. Generate the new password into an off-git env file with mode 600. Never print it or write it anywhere else:
   ```sh
   (umask 077; printf 'AUDIT018_COLLAB_PASSWORD=%s\n' "$(openssl rand -hex 16)" > ~/gameplane-audit-018/collab.env)
   chmod 600 ~/gameplane-audit-018/collab.env
   ```
2. Send it from that variable as the JSON body `{"password": "<new password>"}` that the handler requires (`resetPasswordReq` in `api/internal/handlers/users.go`). `printf` is a shell builtin, so the value is never on a process command line:
   ```sh
   . ~/gameplane-audit-018/collab.env
   printf '{"password":"%s"}' "$AUDIT018_COLLAB_PASSWORD" | curl -s -o /dev/null -w '%{http_code}\n' -X POST -H "Content-Type: application/json" -d @- -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/users/<collab-id>/reset-password
   unset AUDIT018_COLLAB_PASSWORD
   ```

**Expected**
HTTP 204 (No Content) with an empty body: the API returns no password. The generated value is 32 hex characters, above `auth.MinPasswordLen` (12). A shorter one gets 400 `password too short`, and an unknown id gets 404 `user not found`. Every existing audit018-collab session is ended, so a later collab login (login cost 1) uses the password from `collab.env`.

**Cleanup**
Keep `~/gameplane-audit-018/collab.env` while audit018-collab exists. Remove it when that account is deleted.

**Automatable?**
yes (api-auth): the reset is a single POST that returns 204. TestAPI_PasswordResetInvalidatesSession already covers it, including ending the user's existing sessions.

---

### users-bindings-list

**Preconditions**
Authenticated as audit018-admin (users:manage). audit018-collab exists (from round setup).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/users/<collab-id>/bindings | jq 'length'`

**Expected**
HTTP 200, JSON array of role bindings (may be empty; user has primary role).

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### users-bindings-add

**Preconditions**
Authenticated as audit018-admin (users:manage). audit018-collab exists (from round setup). Namespace `gameplane-games` exists. Role `operator` exists.

**Resources created**
RoleBinding: audit018-collab → operator role in gameplane-games namespace.

**Steps**
1. Login cost: 0. Create request: `{"roleName":"operator","namespace":"gameplane-games"}`
2. Run: `curl -X POST -H "Content-Type: application/json" -d @request.json -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/users/<collab-id>/bindings | jq '.roleName'`

**Expected**
HTTP 201, JSON with binding.

**Cleanup**
DELETE /users/{id}/bindings/{role}/{namespace}

**Automatable?**
yes (api-auth)

---

### users-delete

**Preconditions**
Authenticated as audit018-admin (users:manage). audit018-tmpuser exists (from users-create), with the `id` users-create printed. Only users-create and this procedure use audit018-tmpuser. audit018-collab from round setup is not deleted here.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -X DELETE -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/users/<tmpuser-id>`

**Expected**
HTTP 204.

**Cleanup**
none. audit018-tmpuser has no password, so there is no env file. `~/gameplane-audit-018/collab.env` stays while audit018-collab exists (users-reset-password Cleanup).

**Automatable?**
yes (api-auth)

---

### roles-list

**Preconditions**
Authenticated as audit018-admin (roles:read).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/roles | jq 'length'`

**Expected**
HTTP 200, JSON array of roles (at least: admin, operator, viewer).

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### roles-permissions-catalog

**Preconditions**
Authenticated as audit018-admin (roles:read).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/roles/permissions | jq '.groups | keys | length'`

**Expected**
HTTP 200, JSON with permission catalog organized by groups (servers, audit, config, etc.).

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### cluster-view

**Preconditions**
Authenticated as audit018-viewer (cluster:read).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." $GP/cluster | jq '.nodes | length'`

**Expected**
HTTP 200, JSON with cluster info: nodes, version, ready count, etc.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### cluster-info

**Preconditions**
Authenticated as audit018-viewer (cluster:read).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." $GP/cluster/info | jq '.version'`

**Expected**
HTTP 200, JSON with cluster name, Kubernetes version, Gameplane version.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### cluster-stats

**Preconditions**
Authenticated as audit018-viewer (cluster:read).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." $GP/cluster/stats | jq '.totalStorageBytes'`

**Expected**
HTTP 200, JSON with `nodes` (count), `totalStorageBytes`, and `usedStorageBytes` (capacity of Bound PVCs, not live disk usage). No CPU/memory fields here — those are per-node, under GET /cluster's `nodes[].cpu`/`.memory`.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### cluster-join-node

**Preconditions**
Authenticated as audit018-admin (cluster:manage). Cluster ops enabled (--cluster-ops=true). At least one control-plane node exists.

**Resources created**
Bootstrap token Secret in kube-system namespace (TTL: 24h).

**Steps**
1. Login cost: 0. Run: `curl -X POST -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/cluster/nodes:join | jq '.command' | head -c 50`

**Expected**
HTTP 200, JSON with `command`, `token`, `caCertHash`, `endpoint`, `expiresAt`.

**Cleanup**
none (token expires after 24h)

**Automatable?**
yes (api-auth)

---

### cluster-download-kubeconfig

**Preconditions**
Authenticated as audit018-admin (cluster:manage). Cluster ops enabled. Current kubeconfig is valid.

**Resources created**
none (temporary credentials, not persisted)

**Steps**
1. Login cost: 0. Run: `curl -X POST -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/cluster/kubeconfig | head -c 100`

**Expected**
HTTP 200, Content-Type: application/yaml. Body is a plaintext kubeconfig YAML document (not JSON), carrying a short-lived (1h) client cert.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### clusters-list

**Preconditions**
Authenticated as audit018-viewer (cluster:read). Multi-cluster is enabled or default single cluster.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." $GP/clusters | jq '.items | length'`

**Expected**
HTTP 200, JSON array (may be empty or have local cluster info in single-cluster setup).

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### modules-list-installed

**Preconditions**
Authenticated as audit018-viewer (modules:read). At least one module installed (or none, which is valid).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." $GP/modules | jq '.items | length'`

**Expected**
HTTP 200, JSON array of installed Module CRs.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### modules-list-sources

**Preconditions**
Authenticated as audit018-viewer (modules:read). ModuleSources exist (at least "default").

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." $GP/modules/sources | jq '.items | length'`

**Expected**
HTTP 200, JSON array of ModuleSource objects.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### modules-catalog

**Preconditions**
Authenticated as audit018-viewer (modules:read). At least one ModuleSource has a catalog.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." $GP/modules/catalog | jq '.items | length'`

**Expected**
HTTP 200, JSON array of available modules (merged across all sources).

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### backup-destinations-list

**Preconditions**
Authenticated as audit018-viewer (destinations:read). At least one destination configured (or none).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." $GP/backup-destinations | jq '.items | length'`

**Expected**
HTTP 200, JSON array of restic backup destination Secrets (URL, hasPassword flag, no credentials).

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### events-sse

**Preconditions**
Authenticated as audit018-viewer (servers:read). At least one server exists.

**Resources created**
none (streaming connection)

**Steps**
1. Login cost: 0. Run: `curl -N --http1.1 -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." "$GP/events?namespace=gameplane-games" & sleep 2; kill $!`

**Expected**
HTTP 200, Content-Type: text/event-stream. Stream sends events line-by-line (data: JSON format). Connection is long-lived.

**Cleanup**
Kill the curl process.

**Automatable?**
yes (api-auth) - streaming tests are lightweight.

---

### ws-console

**Preconditions**
Authenticated as audit018-operator (servers:console). Server is running and agent is healthy. WebSocket upgrade supported.

**Resources created**
none (WebSocket stream, proxied to agent)

**Steps**
1. Login cost: 0. Note: WebSocket testing requires a client (e.g., `websocat`). Run: `websocat -H "Cookie: gameplane_session=...;gameplane_csrf=..." -H "X-Gameplane-CSRF: ..." "ws://127.0.0.1:18080/ws/servers/mc-fabric/console?namespace=gameplane-games" <<< 'help' 2>&1 | head -5`

**Expected**
WebSocket upgrade (HTTP 101). Commands are proxied to the game's RCON interface; responses stream back.

**Cleanup**
Close connection.

**Automatable?**
deferred to T031 (requires WebSocket client; browser-based only in most test envs)

---

### ws-logs

**Preconditions**
Authenticated as audit018-viewer (servers:read). Server is running; logs available from agent.

**Resources created**
none (WebSocket stream)

**Steps**
1. Login cost: 0. Run: `websocat "ws://127.0.0.1:18080/ws/servers/mc-fabric/logs?namespace=gameplane-games" 2>&1 | head -5`

**Expected**
WebSocket 101, game's log lines stream as text frames.

**Cleanup**
Close connection.

**Automatable?**
deferred to T031

---

### files-list

**Preconditions**
Authenticated as audit018-viewer (servers:read, or owner via ownership fallback). Server has agent running.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/mc-fabric/files/list?path=/" | jq 'length'`

**Expected**
HTTP 200, JSON array of file entries (each with `name`, `path`, `size`, `mode`, `dir`, `modTime`) — not wrapped in a `files` key.

**Cleanup**
none

**Automatable?**
yes (api-rbac)

---

### files-read

**Preconditions**
Authenticated as audit018-viewer. Server has agent. File exists at path (e.g., config file, README).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/mc-fabric/files/read?path=/readme.txt" | head -c 100`

**Expected**
HTTP 200, body is the raw file bytes served via `http.ServeFile` (not JSON — there is no `content` field).

**Cleanup**
none

**Automatable?**
yes (api-rbac)

---

### files-upload

**Preconditions**
Authenticated as audit018-operator (servers:write, or owner). Server has agent.

**Resources created**
Uploaded file in server's game data directory.

**Steps**
1. Login cost: 0. Create a test file: `echo 'test content' > /tmp/test-upload.txt`
2. Run: `curl -s -o /dev/null -w '%{http_code}\n' -F "file=@/tmp/test-upload.txt" -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/mc-fabric/files/upload?path=/uploads/"`

**Expected**
HTTP 204 (No Content), empty body — confirm the file landed via files-list on `/uploads/`.

**Cleanup**
DELETE /servers/mc-fabric/files/delete?path=/uploads/test-upload.txt

**Automatable?**
yes (api-rbac)

---

### players-list

**Preconditions**
Authenticated as audit018-viewer (servers:read). Server is running; agent reports players.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/mc-fabric/players" | jq '.players | length'`

**Expected**
HTTP 200, JSON with `players` array (may be empty).

**Cleanup**
none

**Automatable?**
yes (api-rbac)

---

### players-kick

**Preconditions**
Authenticated as audit018-operator (servers:write). Server running, player logged in (e.g., "Steve").

**Resources created**
none (player booted)

**Steps**
1. Login cost: 0. Create request: `{"name":"Steve","reason":"Test kick"}`
2. Run: `curl -X POST -H "Content-Type: application/json" -d @request.json -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/mc-fabric/players/kick" | jq`

**Expected**
HTTP 200, JSON response confirms kick.

**Cleanup**
none (player left server)

**Automatable?**
deferred to T031 (requires live player in server)

---

### system-logs-api

**Preconditions**
Authenticated as audit018-viewer (cluster:read).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." "$GP/admin/system-logs/api?tailLines=20" | wc -l`

**Expected**
HTTP 200, Content-Type: text/plain. Body is up to 20 raw, timestamped log lines from the API pod — not JSON, no `logs` field.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### system-logs-operator

**Preconditions**
Authenticated as audit018-viewer (cluster:read).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." "$GP/admin/system-logs/operator?tailLines=20" | wc -l`

**Expected**
HTTP 200, Content-Type: text/plain. Body is up to 20 raw, timestamped log lines from the operator pod — not JSON.

**Cleanup**
none

**Automatable?**
yes (api-auth)
