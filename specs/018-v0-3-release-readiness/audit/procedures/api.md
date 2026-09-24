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
1. Login cost: 1 (to audit018-admin). Run: `curl -c /tmp/audit-cookies.txt -b /tmp/audit-cookies.txt -H "Content-Type: application/json" -d '{"username":"audit018-admin","password":"password"}' $GP/auth/login | jq`
2. Extract session and CSRF from cookies: `grep gameplane_session /tmp/audit-cookies.txt` and `grep gameplane_csrf /tmp/audit-cookies.txt`

**Expected**
HTTP 200, JSON with user object (id, username, role). Response Set-Cookie headers include `gameplane_session` and `gameplane_csrf` (both HttpOnly, Secure).

**Cleanup**
Remove `/tmp/audit-cookies.txt`.

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
Share link exists for a server (created via authenticated route, then exported here). Share token is the 32-char random token from the share link creation response. Share link is not expired.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s $GP/shares/TOKEN_HERE/ | jq` (replace TOKEN_HERE with actual token)

**Expected**
HTTP 200, JSON object with server info (name, display name, description, etc.). No owner/collaborator status exposed at this level.

**Cleanup**
none

**Automatable?**
deferred to T031 (requires creating a share link first, which requires auth + owned server).

---

### public-shares-start

**Preconditions**
Share link exists and canStart=true. Share token is valid and not expired.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -X POST -s $GP/shares/TOKEN_HERE/start | jq` (replace TOKEN_HERE with actual token)

**Expected**
HTTP 200, JSON with `consoleURL` or `webURL` — a signed redirect to the server's console or dashboard view without requiring login.

**Cleanup**
none

**Automatable?**
deferred to T031 (requires a share link with canStart=true).

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
     "apiVersion": "gameplane.valgul.moe/v1alpha1",
     "kind": "GameServer",
     "metadata": {"name": "audit018-server-from-template", "namespace": "gameplane-games"},
     "spec": {
       "templateRef": "mc-fabric",
       "suspend": false,
       "replicas": 1,
       "gameDataStorage": {"size": "5Gi"},
       "mods": {"items": []}
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

### servers-delete

**Preconditions**
Authenticated as audit018-operator. audit018-server-from-template exists. User has servers:write permission (or is owner).

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
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." "$GP/admin/audit?limit=10" | jq '.items | length'`

**Expected**
HTTP 200, JSON with `items` array of audit events and pagination cursor `before`.

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
HTTP 200, JSON with `ok: true` if chain is intact, or `broken: true` + `brokenAt` if tampering detected.

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
2. Run: `curl -X PUT -H "Content-Type: application/json" -d '{"sinks":[]}' -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/admin/config/notifications | jq'.sinks | length'`

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
Secret in gameplane-system namespace: `gameplane-oidc-test-oidc` (labeled).

**Steps**
1. Login cost: 0. Run: `curl -X PUT -H "Content-Type: application/json" -d '{"clientSecret":"test-secret-value"}' -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/admin/auth/providers/test-oidc/secret | jq '.name'`

**Expected**
HTTP 200, JSON response with `name: gameplane-oidc-test-oidc` and `keys: ["clientSecret"]`.

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
Secret: `gameplane-sink-webhook` (or equivalent) in gameplane-system.

**Steps**
1. Login cost: 0. Run: `curl -X PUT -H "Content-Type: application/json" -d '{"secret":"test-webhook-auth"}' -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/admin/notifications/sinks/webhook/secret | jq '.name'`

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
Secret: `gameplane-registry-curseforge` in gameplane-system.

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
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/mc-fabric:captures?namespace=gameplane-games" | jq '.items | length'`

**Expected**
HTTP 200, JSON array of packet captures for the server.

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
1. Login cost: 0. Run: `curl -X POST -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/audit018-server-from-template:start?namespace=gameplane-games" | jq '.spec.suspend'`

**Expected**
HTTP 200, JSON shows spec.suspend=false (server will start).

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
1. Login cost: 0. Run: `curl -X POST -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/audit018-server-from-template:stop?namespace=gameplane-games" | jq '.spec.suspend'`

**Expected**
HTTP 200, JSON shows spec.suspend=true.

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
1. Login cost: 0. Run: `curl -X POST -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/audit018-server-from-template:restart?namespace=gameplane-games" | jq`

**Expected**
HTTP 200, JSON response. Operator will restart the pod.

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
1. Login cost: 0. Create request: `{"name":"audit018-cloned-server"}`
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
2. Run: `curl -X POST -H "Content-Type: application/json" -d @request.json -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/audit018-server-from-template:wipe-data?namespace=gameplane-games" | jq`

**Expected**
HTTP 200, JSON response. Operator will empty the server's data.

**Cleanup**
none

**Automatable?**
yes (api-rbac)

---

### shares-create

**Preconditions**
Authenticated as audit018-operator, and is the owner of mc-fabric (or audit018-server-from-template). Ownership is verified server-side.

**Resources created**
ServerShare CRD: audit018-share-link-NNN.

**Steps**
1. Login cost: 0. Create request: `{"expiresAt":"2026-10-01T00:00:00Z","canStart":true}`
2. Run: `curl -X POST -H "Content-Type: application/json" -d @request.json -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/mc-fabric:shares?namespace=gameplane-games" | jq '.token'`

**Expected**
HTTP 201, JSON with `id`, `token`, `expiresAt`, `canStart`. Token is a 32-char string (only returned once).

**Cleanup**
DELETE /servers/mc-fabric/shares/{id}

**Automatable?**
deferred to T031 (requires server ownership; blocked by OD-015 admin access)

---

### shares-list

**Preconditions**
Authenticated as owner of mc-fabric. Share links exist.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/mc-fabric:shares?namespace=gameplane-games" | jq '.items | length'`

**Expected**
HTTP 200, JSON array of ServerShare objects (token NOT included in list response).

**Cleanup**
none

**Automatable?**
deferred to T031

---

### servers-transfer

**Preconditions**
Authenticated as owner of audit018-server-from-template (operator role, owner-only). Recipient user exists (e.g., audit018-admin).

**Resources created**
none (ownership annotation mutation)

**Steps**
1. Login cost: 0. Create request: `{"userId": <audit018-admin user id>}`
2. Run: `curl -X POST -H "Content-Type: application/json" -d @request.json -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/audit018-server-from-template:transfer?namespace=gameplane-games" | jq '.metadata.annotations."gameplane.local/owner"'`

**Expected**
HTTP 200, JSON shows ownership transferred (annotation updated).

**Cleanup**
Transfer back to original owner.

**Automatable?**
deferred to T031 (requires server ownership; blocked by OD-015)

---

### servers-collaborators-set

**Preconditions**
Authenticated as owner of audit018-server-from-template. Collaborator users exist.

**Resources created**
none (collaborators annotation mutation)

**Steps**
1. Login cost: 0. Create request: `{"userIds":[<user-id>]}`
2. Run: `curl -X PUT -H "Content-Type: application/json" -d @request.json -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/audit018-server-from-template:collaborators?namespace=gameplane-games" | jq '.metadata.annotations."gameplane.local/collaborators"'`

**Expected**
HTTP 200, JSON shows collaborators list updated.

**Cleanup**
Set collaborators to empty array.

**Automatable?**
deferred to T031 (blocked by OD-015)

---

### users-list

**Preconditions**
Authenticated as audit018-admin (users:read).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/users | jq '.items | length'`

**Expected**
HTTP 200, JSON array of user objects.

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### users-create

**Preconditions**
Authenticated as audit018-admin (users:manage). New username does not exist (e.g., audit018-collab).

**Resources created**
User: audit018-collab in database.

**Steps**
1. Login cost: 0. Create request: `{"username":"audit018-collab","displayName":"Collaborator","email":"collab@audit.local","role":"viewer"}`
2. Run: `curl -X POST -H "Content-Type: application/json" -d @request.json -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/users | jq '.username'`

**Expected**
HTTP 201, JSON with new user object.

**Cleanup**
DELETE /users/{id}

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
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." $GP/users/me/preferences | jq '.theme'`

**Expected**
HTTP 200, JSON with theme, accent, etc. (may be null/default if never set).

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
1. Login cost: 0. Create request: `{"theme":"dark","accent":"blue"}`
2. Run: `curl -X PUT -H "Content-Type: application/json" -d @request.json -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." $GP/users/me/preferences | jq '.theme'`

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
Authenticated as audit018-admin (users:read). audit018-collab user exists.

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
Authenticated as audit018-admin (users:manage). audit018-collab exists with role=viewer.

**Resources created**
none (mutation)

**Steps**
1. Login cost: 0. Create request: `{"displayName":"Updated Collab","role":"operator"}`
2. Run: `curl -X PATCH -H "Content-Type: application/json" -d @request.json -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/users/<collab-id> | jq '.role'`

**Expected**
HTTP 200, JSON shows role=operator.

**Cleanup**
PATCH back to role=viewer.

**Automatable?**
yes (api-auth)

---

### users-delete

**Preconditions**
Authenticated as audit018-admin (users:manage). audit018-collab exists.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -X DELETE -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/users/<collab-id>`

**Expected**
HTTP 204.

**Cleanup**
none (user is deleted)

**Automatable?**
yes (api-auth)

---

### users-reset-password

**Preconditions**
Authenticated as audit018-admin (users:manage). audit018-collab user exists.

**Resources created**
none (password reset in DB, temporary token returned once)

**Steps**
1. Login cost: 0. Run: `curl -X POST -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/users/<collab-id>/reset-password | jq '.temporaryPassword'`

**Expected**
HTTP 200, JSON with temporary password string.

**Cleanup**
none

**Automatable?**
deferred to T031 (password reset flow requires multi-step verification)

---

### users-bindings-list

**Preconditions**
Authenticated as audit018-admin (users:manage). audit018-collab user exists.

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/users/<collab-id>/bindings | jq '.items | length'`

**Expected**
HTTP 200, JSON array of role bindings (may be empty; user has primary role).

**Cleanup**
none

**Automatable?**
yes (api-auth)

---

### users-bindings-add

**Preconditions**
Authenticated as audit018-admin (users:manage). audit018-collab user exists. Namespace `gameplane-games` exists. Role `operator` exists.

**Resources created**
RoleBinding: audit018-collab → operator role in gameplane-games namespace.

**Steps**
1. Login cost: 0. Create request: `{"role":"operator","namespace":"gameplane-games"}`
2. Run: `curl -X POST -H "Content-Type: application/json" -d @request.json -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/users/<collab-id>/bindings | jq '.role'`

**Expected**
HTTP 201, JSON with binding.

**Cleanup**
DELETE /users/{id}/bindings/{role}/{namespace}

**Automatable?**
yes (api-auth)

---

### roles-list

**Preconditions**
Authenticated as audit018-admin (roles:read).

**Resources created**
none

**Steps**
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/roles | jq '.items | length'`

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
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." $GP/cluster/stats | jq '.cpu'`

**Expected**
HTTP 200, JSON with aggregate cluster stats (CPU, memory, storage).

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
1. Login cost: 0. Run: `curl -X POST -b ~/gameplane-audit-018/session-admin.txt -H "X-Gameplane-CSRF: ..." $GP/cluster/kubeconfig | jq '.kubeconfig' | head -c 100`

**Expected**
HTTP 200, JSON with kubeconfig string (base64 or plaintext YAML).

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
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/mc-fabric/files/list?path=/" | jq '.files | length'`

**Expected**
HTTP 200, JSON with `files` array (server's filesystem listing).

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
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/mc-fabric/files/read?path=/readme.txt" | jq '.content' | head -c 100`

**Expected**
HTTP 200, JSON with `content` (file bytes as base64 or text).

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
2. Run: `curl -F "file=@/tmp/test-upload.txt" -b ~/gameplane-audit-018/session-operator.txt -H "X-Gameplane-CSRF: ..." "$GP/servers/mc-fabric/files/upload?path=/uploads/" | jq '.path'`

**Expected**
HTTP 200, JSON with `path` of uploaded file.

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
1. Login cost: 0. Create request: `{"player":"Steve","reason":"Test kick"}`
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
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." "$GP/admin/system-logs/api?tailLines=20" | jq '.logs | length'`

**Expected**
HTTP 200, JSON with `logs` array (last 20 lines of API container logs).

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
1. Login cost: 0. Run: `curl -s -b ~/gameplane-audit-018/session-viewer.txt -H "X-Gameplane-CSRF: ..." "$GP/admin/system-logs/operator?tailLines=20" | jq '.logs | length'`

**Expected**
HTTP 200, JSON with operator container logs.

**Cleanup**
none

**Automatable?**
yes (api-auth)
