# Gameplane-Sec Security Audit

## Scope

This report records confirmed repository-level security findings from an in-depth audit of:

- All Go modules and executable services (`api`, `operator`, `agent`, `audit-syslog-bridge`, `telemetry-receiver`, `sentinel`, `tunnel`, `capture-sidecar`, `mcp-server`, `svcutil`, `netguard`, `gameaction`, `gameproto`)
- API routes, WebSockets, SSE, authentication, RBAC authorization, and share links
- Kubernetes CRDs, RBAC roles, NetworkPolicies, Services, Deployments, and Helm defaults
- Operator controllers, reconcilers, and sidecar injection logic
- Filesystem, process, shell, network, URL, and secret-handling inputs
- CI workflows, Dockerfiles, image sources, and build configuration
- Default Helm rendering and deployment exposure

---

## Findings & Remediations

### 1. Medium — Share links are not cluster-bound

**Description:**
Share creation originally stored only the namespace and server name in the database, while token resolution evaluated against the caller's active cluster context. In multi-cluster configurations, a valid share token could resolve or wake/start a same-named server in another cluster. Furthermore, `ListShareLinks` and `RevokeShareLink` checked server ownership in the caller's cluster context but queried/mutated share links purely by `(namespace, server_name)` or `id`, permitting an owner of a same-named server in cluster B to inspect or revoke shares created in cluster A.

**References:**
- `api/internal/handlers/shares.go`
- `api/internal/db/shares.go`
- `api/internal/db/migrations/009_share_links_cluster.sql`

**Remediation:**
- Added `cluster` column to `share_links` table with migration `009_share_links_cluster.sql`.
- Updated `CreateShareLink`, `ListShareLinks`, and `RevokeShareLink` to require and enforce `cluster` matching.
- Updated `resolveShareHandler` and `startShareHandler` to resolve strictly to the recorded originating cluster from the database record rather than user request context.
- Added comprehensive unit tests in `api/internal/db/shares_test.go` and `api/internal/handlers/shares_test.go` (`TestShareClusterScoping`).

**Status:** Fixed

---

### 2. Medium/Low — Foreign same-named Secrets may be trusted & Env Exfiltration

**Description:**
`GameServer` specifications accept Secret and ConfigMap references via `spec.env[*].valueFrom.secretKeyRef`. Previously, non-admin users could specify arbitrary Secret names (e.g. `restic-repo-backups`, auth tokens, or other servers' secrets) in `spec.env`, which would then be mounted into game server containers as environment variables, allowing secret exfiltration. Additionally, canonical secret name suffixes and user-controlled labels could be trusted as proof of ownership.

**References:**
- `api/internal/handlers/resources.go`
- `operator/internal/controller/gameserver_controller.go`

**Remediation:**
- In `api/internal/handlers/resources.go`, `validateAndProtectGameServer` enforces that any Secret or ConfigMap referenced in `spec.env` must have a controller `OwnerReference` matching the `GameServer`'s name and UID. Suffixes or labels alone are rejected.
- In `operator/internal/controller/gameserver_controller.go`, `validateServerEnvSources` validates all `spec.env` `SecretKeyRef` and `ConfigMapKeyRef` targets against the cluster before StatefulSet reconciliation, preventing unowned secret or configmap mounts even if bypassed via direct CR creation.
- Added unit tests in `api/internal/handlers/resources_security_test.go` and `operator/internal/controller/gameserver_security_test.go`.

**Status:** Fixed

---

### 3. Low — Optional audit syslog bridge may accept forged events

**Description:**
When the optional `gameplane-audit-syslog-bridge` is enabled without `api.audit.webhook.authSecretRef`, it accepts unauthenticated HTTP POST requests. Its ClusterIP Service previously had no dedicated Ingress NetworkPolicy, meaning any workload in the cluster could reach port 8514 and inject forged audit log records into the upstream syslog collector.

**References:**
- `audit-syslog-bridge/main.go:143`
- `charts/gameplane/templates/audit-syslog-bridge.yaml`

**Remediation:**
- Added a dedicated Ingress `NetworkPolicy` to `charts/gameplane/templates/audit-syslog-bridge.yaml` that admits TCP port 8514 ingress exclusively from pods matching `app.kubernetes.io/name: gameplane-api` (requires `networkPolicies.enabled=true` in Helm values or equivalent cluster CNI policies).
- Maintained existing token authentication requirement via `AUTH_HEADER` when configured.

**Status:** Fixed

---

### 4. Low — Mutable container base images

**Description:**
`images/common/steamcmd/Dockerfile` previously declared `FROM steamcmd/steamcmd:latest`. Mutable tags reduce build reproducibility and permit upstream tag rewrites to alter release image contents without repository changes.

**References:**
- `images/common/steamcmd/Dockerfile:42`

**Remediation:**
- Pinned `steamcmd/steamcmd` to immutable digest `steamcmd/steamcmd@sha256:7178bc460bf7f9658135f57760bf7f486638a2b8855ca65dbae55d688abfff50`.

**Status:** Fixed

---

### 5. High/Medium — Arbitrary Secret Deletion & Exfiltration via Tunnel Credentials

**Description:**
Multiple vulnerabilities existed around tunnel credential handling:
1. `DELETE /servers/{name}:tunnel-credentials` in `api/internal/handlers/tunnelcreds.go` read `gs.Spec.Networking.Tunnel.CredentialsSecretRef.Name` and called `k.Typed.CoreV1().Secrets(ns).Delete` without verifying that the secret was owned by the GameServer. An attacker with server update permissions could set `credentialsSecretRef.name` to sensitive secrets (such as `restic-repo-backups` or other server secrets) and issue a DELETE request to delete arbitrary secrets in the namespace.
2. `PUT /servers/{name}:tunnel-credentials` patched `<server>-tunnel-auth` if it already existed without checking whether the existing secret was owned by the calling GameServer UID, allowing overwriting of pre-existing secrets.
3. `reconcileTunnel` in `operator/internal/controller/gameserver_tunnel.go` mounted `tunnel.CredentialsSecretRef.Name` into the tunnel relay Deployment without verifying ownership, permitting exfiltration of arbitrary secrets into tunnel pods.
4. `validateAndProtectGameServer` in `api/internal/handlers/resources.go` omitted checks for `spec.networking.tunnel.credentialsSecretRef`.

**References:**
- `api/internal/handlers/tunnelcreds.go`
- `api/internal/handlers/resources.go`
- `operator/internal/controller/gameserver_tunnel.go`

**Remediation:**
- Enforced `isServerOwnedSecretObject` in `tunnelcredsHandler.delete`: fetches the Secret and verifies an `OwnerReference` matching the `GameServer` name and UID before deletion. Refuses unowned secrets with `403 Forbidden`.
- Enforced `isServerOwnedSecretObject` in `tunnelcredsHandler.put`: checks existing secrets before patching and returns `409 Conflict` if not owned by the GameServer UID.
- Added validation in `api/internal/handlers/resources.go`: `validateAndProtectGameServer` restricts non-admin updates to `spec.networking.tunnel.credentialsSecretRef` to server-owned Secrets.
- Added validation in `operator/internal/controller/gameserver_tunnel.go`: `reconcileTunnel` validates that `tunnel.CredentialsSecretRef` is owned by `gs` via `isServerOwnedSecret` before adding the Secret volume mount.
- Added tests in `api/internal/handlers/tunnelcreds_test.go`, `api/internal/handlers/resources_security_test.go`, and `operator/internal/controller/gameserver_security_test.go`.

**Status:** Fixed

---

### 6. Medium/Low — Telemetry receiver unauthenticated in-cluster access

**Description:**
The optional anonymous usage telemetry receiver (`charts/gameplane/templates/telemetry-receiver.yaml`) deployed a ClusterIP Service without an Ingress NetworkPolicy. Workloads across the cluster could access port 8080 and post arbitrary telemetry data when `authSecretRef` is not configured.

**References:**
- `charts/gameplane/templates/telemetry-receiver.yaml`

**Remediation:**
- Added a dedicated Ingress `NetworkPolicy` to `charts/gameplane/templates/telemetry-receiver.yaml` restricting TCP port 8080 ingress to pods matching `app.kubernetes.io/name: gameplane-api` (requires `networkPolicies.enabled=true` in Helm values or equivalent cluster CNI policies).

**Status:** Fixed

---

## Items reviewed but not retained as findings

- **Backup destination secrets (`handlers/destinations.go`):** The API explicitly filters and manages secrets with `gameplane.local/backup-destination=true` label, never returns raw passwords, and requires `destinations:manage` / `backups:write` permissions.
- **Operator Role/RoleBinding permissions:** The operator's RBAC grants remain scoped and adhere to controller-runtime least privilege, bound by Kubernetes authorization and API server validation.
- **Template mount-path command construction:** Game templates already define container images and entrypoints (same trust boundary as template creators).
- **Notification HTTP/SMTP egress:** Outbound webhook and email delivery are guarded at dial time and restricted to the `config:manage` administrative boundary.
- **Agent in-pod sidecar communication:** Secured via token authentication (mTLS or shared secret) and restricted to localhost and cluster network policies.
- **Agent filesystem & mod confinement (`agent/internal/files`, `agent/internal/mods`):** Verified strict path confinement (`ConfinePath`, `ConfineRelPath`), symlink traversal prevention, zip-slip defense, and file size/count bounds on multipart uploads and archive extractions.
- **Agent console & WebSocket multiplexing (`agent/internal/console`, `api/internal/ws`):** Evaluated RCON and PTY console routes; access is gated behind `servers:console` RBAC, and stdin/RCON command parameters are protected against control-character injection via `gameaction.Resolve`.
- **Sentinel & gameproto packet parsers (`sentinel`, `gameproto`):** Handshake parsers for Minecraft (`minecraft.go`) and Terraria (`terraria.go`) strictly bound frame lengths (512B for Minecraft, 64KB for Terraria), validate VarInt / 7-bit encoded integer ranges, and prevent unbounded memory allocation or panic triggers on untrusted network packets.
- **MCP server (`mcp-server`):** Verified that all exposed tools explicitly declare `readOnlyTool` metadata and are backed by a restricted Kubernetes client that only implements `Get` and `List` methods, structurally preventing any mutating operations.
- **Capture sidecar (`capture-sidecar`):** Evaluated packet capture control plane; capture IDs are strictly validated against `^[A-Za-z0-9_-]{1,64}$` to prevent filesystem traversal when writing PCAPNG files.
- **Outbound network guard (`netguard`):** Audited `IsAllowed` (operator) and `IsPublic` (agent) dial-time address controls; confirmed all cloud metadata ranges (including 169.254.0.0/16, IPv6 translation prefixes, and Kubernetes pod/node CGNAT blocks) are blocked against SSRF and DNS rebinding.
- **Web UI (`web/`):** Confirmed complete absence of `dangerouslySetInnerHTML` or unsanitized DOM manipulation; session cookies enforce `HttpOnly`, `Secure`, and `SameSite=Lax`, with double-submit CSRF token protection on mutations.

---

## Verification

All fixes were verified on the remote development environment (`dev@127.0.0.1`):
- All 13 Go binaries compiled cleanly via `make build-go`.
- Unit and security test suites in `api/internal/db`, `api/internal/handlers`, and `operator/internal/controller` passed without regressions.
- All 14 Go modules passed tests via `make test-go` (0 failures).

