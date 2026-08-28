# Install

## Prerequisites

- Kubernetes 1.28+
- Helm 3.13+
- A default StorageClass (any RWO CSI driver works)
- Optional: an ingress controller (nginx-ingress by default) + cert-manager for TLS

## One-shot install

The chart and its images are published to the GitHub Container Registry (GHCR)
as OCI artifacts — no `helm repo add` needed. Install a tagged release straight
from the registry (replace `<version>` with a release, e.g. `0.2.0-beta.7`):

```sh
helm upgrade --install gameplane oci://ghcr.io/valgulnecron/charts/gameplane \
  --version <version> \
  --namespace gameplane-system --create-namespace \
  --set ingress.host=gameplane.your-domain.test
```

The chart's `appVersion` pins matching component images
(`ghcr.io/valgulnecron/gameplane/{operator,api,agent}:<version>`), so no image
overrides are needed for a released version.

### Edge channel (latest beta)

Every push to `main` publishes rolling `:edge` images. To track them, install
the chart and point images at the edge tag:

```sh
helm upgrade --install gameplane oci://ghcr.io/valgulnecron/charts/gameplane \
  --version <version> --set image.tag=edge \
  --namespace gameplane-system --create-namespace \
  --set ingress.host=gameplane.your-domain.test
```

`:edge` moves with `main`; pin a specific commit with `image.tag=sha-<short>`
when you need reproducibility.

### Verifying image signatures

Every published image (tagged releases and `:edge`), the Helm chart, and the
official module bundles are signed with the project's cosign key,
[`cosign.pub`](../cosign.pub) at the repo root, and recorded in the public
Sigstore Rekor transparency log:

```sh
cosign verify --key cosign.pub \
  ghcr.io/valgulnecron/gameplane/operator:<version>
```

Pre-rotation releases (v0.2.0-beta.7 and earlier) used the retired Ed25519 key
and lack transparency log entries — verify those with `cosign-legacy.pub` and
`--insecure-ignore-tlog=true`. See [`key-rotation.md`](key-rotation.md) for the
trust continuity proof. Module bundles are verified the same way; the chart
carries the key, so bundle verification is just a values flip — see
[`module-authoring.md`](module-authoring.md#signing-official-bundles).

From source (during development), the chart in this repo always renders against
the local default image path:

```sh
helm upgrade --install gameplane ./charts/gameplane \
  --namespace gameplane-system --create-namespace
```

> **Note:** GHCR packages are private on first publish. The maintainer makes the
> `gameplane/operator`, `gameplane/api`, `gameplane/agent`, and `charts/gameplane`
> packages public once (GHCR → package → *Package settings* → *Change visibility*)
> so anonymous `helm install` / `docker pull` works. For a private install,
> create a `kubernetes.io/dockerconfigjson` pull secret and set
> `image.pullSecrets`.

## First-time setup

Seed an initial admin user. Passwords must be at least 12 characters.

```sh
kubectl -n gameplane-system exec deploy/gameplane-api -- \
  /api bootstrap-admin --username admin --password "<choose>"
```

To avoid the password landing in your shell history, pipe it on stdin:

```sh
printf '%s' "$ADMIN_PASSWORD" | kubectl -n gameplane-system exec -i deploy/gameplane-api -- \
  /api bootstrap-admin --username admin --password-stdin
```

If a user with that name already exists, pass `--force` to rotate the
password and promote them to `admin`.

Open `https://<ingress.host>` and log in.

## Values reference

Top-level knobs (see `values.yaml` for the full list):

- `image.registry` / `image.tag` — container image pinning
- `operator.replicas` — leader-elected, safe at 2+
- `operator.configInitImage` / `operator.resticImage` — the two images the operator
  injects into workloads it creates: the config-init container on game pods
  (`busybox`) and the backup/restore Jobs (`restic/restic`). Both default to the
  upstream pins; retag them to a private registry mirror for air-gapped clusters
  where Docker Hub is unreachable. They map to the operator's
  `--config-init-image` / `--restic-image` flags, mirroring `operator.agentImage`
- `operator.gameDataStorage.storageClassName` — install-time default storage class
  for game server data volumes (default `""`). Empty string uses the cluster's
  default StorageClass. Applies to all GameServers where neither the GameTemplate
  nor GameServer-level override specifies a class. **Precedence**: GameServer
  override > GameTemplate default > install-time default > cluster default.
  **Immutability**: Changing this value affects only new PVCs — existing volumes
  persist unchanged (PVCs are immutable by Kubernetes design). **Error handling**:
  If the named StorageClass doesn't exist, PVC provisioning fails and the
  GameServer enters Pending with a `PVCProvisioningFailed` condition (visible in
  the dashboard); no pod starts until resolved. Example:
  `--set operator.gameDataStorage.storageClassName=fast-nvme`
- `api.db.driver` — `sqlite` (default, production-tested) or `postgres` (experimental, work-in-progress)
- `api.db.dsn` — connection string; SQLite default persists to a PVC
- `api.oidc.enabled` + the following settings — wire OIDC login from Helm (shows
  up as the read-only `helm` provider). Providers can also be added at runtime
  under **Admin Settings → Authentication** — no Helm values or restart needed;
  see [security](security.md#dashboard-managed-providers). Settings:
  - Core connection: `issuer` / `clientID` / `clientSecretRef` / `redirectURL` /
    `displayName` — OIDC provider credentials and endpoints. Per-IdP walkthroughs
    (Keycloak, Authentik, Google) live in [oidc.md](oidc.md)
  - Role mapping (new, seeded at install time):
    - `groupsClaim` — OIDC claim name containing group memberships (default `""`).
      Typically `"groups"` or `"roles"` depending on your IdP. Empty/omitted =
      group-based role mapping disabled; new OIDC users default to `defaultRole`
      (see below). Example: `--set api.oidc.groupsClaim=groups`
    - `roleMappings.admin`, `roleMappings.operator`, `roleMappings.viewer` — arrays
      of IdP group names mapping to each dashboard role (default `[]`). Example:
      `roleMappings.admin: ["gameplane-admins", "ops-team"]` means users in either
      group receive the `admin` role. **These Helm values seed the mappings at
      install/upgrade time.** After install, admins can override any role's groups
      from the dashboard (**Admin Settings → Authentication**) without restarting;
      a dashboard override survives future `helm upgrade`s that change the Helm
      value. Per-role precedence: dashboard override (if present) > Helm seed. No
      admin account or `bootstrap-admin` run needed for OIDC-only installs.
    - `defaultRole` — Helm-only (no dashboard override in v1). Default role when a
      user's IdP groups don't match any `roleMappings` entry (default `""`).
      Accepted values: `""` (treat as `"viewer"`), `"viewer"`, `"operator"`,
      `"admin"`, or `"deny"` (reject login). Meaningful only if `groupsClaim` and
      `roleMappings` are configured. Example: `--set api.oidc.defaultRole=viewer`
  **Backward compatibility**: Omitting `groupsClaim` and `roleMappings` disables
  group-based mapping — existing OIDC setups continue unchanged
- `ingress.host` — dashboard hostname
- `gamesNamespace` — namespace where GameServers are created (default `gameplane-games`)
- `networkPolicies.enabled` — default-deny in games namespace (recommended on)
  - `networkPolicies.kubeletCIDRs` — CIDRs for kubelet liveness/readiness probes (defaults to RFC1918 + link-local)
  - `networkPolicies.probePorts` — specific ports to allow on game pods for kubelet probes (empty = any port)
  - `networkPolicies.gameIngress` — control ingress to game pods from external sources (players)
    - `gameIngress.enabled` — toggle player-ingress allowance (default `true`)
    - `gameIngress.fromCIDRs` — CIDRs from which player traffic is allowed (default `[0.0.0.0/0]`)
  - `networkPolicies.apiServerCIDRs` — CIDRs for agent heartbeat to kube-apiserver (defaults to RFC1918 + link-local on 443/6443)
  - `networkPolicies.gameEgress` — control public-internet egress for game pods (downloads, mod registries)
    - `gameEgress.enabled` — toggle public-egress allowance (default `true`)
    - `gameEgress.ports` — TCP ports for downloads (default 80, 443)
    - `gameEgress.privateCIDRs` — exclude private ranges from public-egress (anti-SSRF)
- `clusterOps.enabled` — credential-minting cluster operations (Add node, Download kubeconfig) in the dashboard's Cluster page (default off; grants powerful kube-system + CSR-approval RBAC)
- `mcpServer.enabled` — optional strictly read-only MCP (Model Context Protocol) server for AI assistants to read cluster state and propose fixes (default off); see [mcp-server/README.md](../mcp-server/README.md)
  - `mcpServer.replicas` — MCP server replicas (default 1)
- `updates.channel` — informational release-channel label (e.g., `stable`, `edge`) shown read-only in the dashboard's Admin Settings → Updates section; purely informational (Gameplane upgrades via Helm, not auto-update)
- `podSecurity.enforceRestricted` — label games namespace for Pod Security Standards
- `defaultModuleSource.*` — the official game catalog shipped with the chart (enabled by default)
  - `defaultModuleSource.enabled` — whether to create the default `ModuleSource` (default `true`; disable when managing sources via GitOps)
  - `defaultModuleSource.name` — name of the `ModuleSource` resource (default `default`)
  - `defaultModuleSource.refreshInterval` — how often the catalog is re-indexed (default `1h`)
  - `defaultModuleSource.type` — source type: `oci` (default, pulls pre-built bundles from a registry) or `git` (index the public `gameplane-module` repo)
  - `defaultModuleSource.git.*` — git configuration (when `type: git`)
    - `git.url` — repository URL (default `https://github.com/ValgulNecron/gameplane-module.git`)
    - `git.ref` — git branch/tag (default `main`)
    - `git.subPath` — module subdirectory within the repository (default `""`, empty means root)
  - `defaultModuleSource.oci.*` — OCI registry configuration (when `type: oci`)
    - `oci.url` — OCI registry URL (e.g., `ghcr.io/valgulnecron/gameplane-modules`)
    - `oci.insecure` — skip TLS verification for plain-HTTP registries (e.g., local development)
    - `oci.modules` — which modules to pull from the registry
    - `oci.pullSecretName` — optional kubernetes.io/dockerconfigjson Secret for private registries
    - `oci.verify.enabled` — enable cosign signature verification for official bundles (default off)
    - `oci.verify.cosignPublicKey` — cosign public key for verification (official key shipped in values.yaml)
- `uploadModuleSource.enabled` — the `uploads` source backing dashboard bundle uploads (default on)
  - `uploadModuleSource.name` — name of the upload `ModuleSource` (default `uploads`)
  - `uploadModuleSource.refreshInterval` — re-index interval for uploaded modules (default `1h`)
- `operator.localModules.{enabled,hostPath,existingClaim,mountPath}` — mount a
  directory of module bundles into the operator for `local`-type sources
- `serviceMonitors.enabled` / `prometheusRules.enabled` / `grafanaDashboards.enabled`
  — opt-in Prometheus Operator integration (see [Observability](#observability))
- `operator.sentinelImage` — the optional sentinel component for wake-on-connect (default
  `ghcr.io/valgulnecron/gameplane/sentinel:<version>`). The sentinel holds
  advertised ports while a GameServer is asleep and wakes it on a genuine
  connection attempt; opt-in per server via `spec.idle.wakeOnConnect` (default
  false). Runs as a small 1-replica Deployment per armed server; costs one pod
  per sleeping server, so disabled by default. See `docs/roadmap.md` for design
  details, caveats (Hostport asymmetry), and why only Minecraft and Terraria get
  real handshake parsing (other games use a generic packets-in-window heuristic).
- `operator.addressManager` — which load-balancer address manager runs in this
  cluster, so the operator knows how to express a GameServer's
  `spec.networking.addressPool` / `spec.networking.address` preference. One of:
  - `metallb` — Service annotations `metallb.io/address-pool` and
    `metallb.io/loadBalancerIPs`.
  - `cilium` — Service label `gameplane.local/lb-pool` plus annotation
    `lbipam.cilium.io/ips`. The label is a Gameplane convention, not something
    Cilium recognises on its own: mirror it in your
    `CiliumLoadBalancerIPPool`'s `spec.serviceSelector` or the pool preference
    selects nothing.
  - `none` (default) — no Service is mutated. A pool/address preference is
    reported on the GameServer's `AddressAssignment` condition as unhonored
    rather than silently falling back to the cluster's default pool.

  Any other value fails the operator at startup. The operator never writes the
  deprecated `service.spec.loadBalancerIP`.
- `capture.enabled` — the optional network packet capture sidecar for GameServers
  (default `false`). When enabled cluster-wide, admins can opt individual GameServers
  into live AF_PACKET capture with BPF filtering and download PCAPNG files. Captures
  are always opt-in per server via `spec.capture.enabled` and admin-only (`captures:manage`
  permission). Key sub-values:
  - `capture.defaultRetentionSeconds` — how long a completed capture is kept before
    automatic deletion (default `86400` = 24 hours).
  - `capture.maxRetentionSeconds` — cluster-wide maximum retention, clamping per-server
    overrides (default `604800` = 7 days). Reflects a GDPR Art. 5(1)(e) storage-limitation
    engineering default, not a legal requirement.
  - `capture.defaultMaxDurationSeconds` — default maximum runtime per capture in seconds
    (default `300` = 5 minutes); captures stop automatically when the duration is reached.
  - `capture.defaultMaxSizeBytes` — default maximum file size per capture in bytes
    (default `5368709120` = 5 GiB); captures stop automatically when the size limit is reached.
  - `capture.image` — sidecar container image (defaults to `{image.registry}/capture-sidecar:{image.tag}`).

## Observability

The operator, API, and in-pod agent sidecars expose Prometheus metrics on
`/metrics` (operator `:8080`, API `:8000`, agent `:8090`). Three
**off-by-default** chart toggles wire them into a Prometheus-Operator stack
(e.g. kube-prometheus-stack):

- `serviceMonitors.enabled` — `ServiceMonitor`s so Prometheus scrapes the
  operator and API, plus a `PodMonitor` that scrapes per-GameServer agent
  metrics from game pods in `gamesNamespace`.
- `prometheusRules.enabled` — a `PrometheusRule` of operator alerts.
- `grafanaDashboards.enabled` — a Grafana dashboard `ConfigMap` the Grafana
  sidecar auto-imports (relabel via `grafanaDashboards.labels` if your sidecar
  watches a different label).

All three add `labels:` you can set so a Prometheus/Grafana selector picks the
objects up.

### Metrics

**Operator fleet gauges** (computed at scrape time from the operator's cache):

| Metric | Labels | Meaning |
|---|---|---|
| `gameplane_gameservers` | `phase` | GameServers per lifecycle phase (Pending/Starting/Running/Stopping/Stopped/Suspended/Failed) |
| `gameplane_backups` | `phase` | Backups per phase (Pending/Running/Succeeded/Failed) |

Every phase is always present (0 when empty). With 2+ operator replicas each
replica reports the same cache-derived counts, so aggregate with
`max by (phase) (...)` (the bundled dashboard and alerts already do).

**Agent per-server metrics** (scraped from port 8090 in each game pod when
`serviceMonitors.enabled: true`):

| Metric | Labels | Meaning |
|---|---|---|
| `gameplane_agent_cpu_millicores` | `server`, `namespace`, `template`, `game` | Game process CPU usage (millicores, read from `/proc`). Emitted only when readable. |
| `gameplane_agent_cpu_limit_millicores` | `server`, `namespace`, `template`, `game` | CPU limit for the game container (millicores). Emitted only when readable. |
| `gameplane_agent_memory_bytes` | `server`, `namespace`, `template`, `game` | Game process memory usage (bytes, read from `/proc`). Emitted only when readable. |
| `gameplane_agent_memory_limit_bytes` | `server`, `namespace`, `template`, `game` | Memory limit for the game container (bytes). Emitted only when readable. |
| `gameplane_agent_disk_used_bytes` | `server`, `namespace`, `template`, `game` | Disk used in the server's data directory (bytes). Emitted only when readable. |
| `gameplane_agent_disk_total_bytes` | `server`, `namespace`, `template`, `game` | Total disk capacity of the server's data directory (bytes). Emitted only when readable. |
| `gameplane_agent_players_online` | `server`, `namespace`, `template`, `game` | Number of players currently online. Emitted only when the game RCON succeeded. |
| `gameplane_agent_players_max` | `server`, `namespace`, `template`, `game` | Maximum player capacity: -1 for unlimited, positive value for a known limit. Emitted only when the capacity is known; absent when unknown or RCON fails. |

**Example PromQL queries for right-sizing servers:**

```
# Peak CPU over the past week per server
max_over_time(gameplane_agent_cpu_millicores[7d])

# Average memory utilization (as a percentage of limit) over 7 days
avg_over_time(gameplane_agent_memory_bytes[7d]) / gameplane_agent_memory_limit_bytes * 100

# 95th percentile disk usage per template
quantile_over_time(0.95, gameplane_agent_disk_used_bytes[7d])
```

### Alerts

`prometheusRules.enabled` ships (group `gameplane.operator` unless noted):

- `GameplaneOperatorReconcileErrors` — a controller failing reconciles for 10m.
- `GameplaneOperatorWorkqueueBacklog` — a workqueue over 50 items for 15m.
- `GameplaneOperatorReconcileStuck` — a single reconcile running over 5m.
- `GameplaneGameServerFailed` *(group `gameplane.fleet`)* — any GameServer in
  the Failed phase for 10m.
- `GameplaneBackupFailed` *(group `gameplane.fleet`)* — any Backup in the Failed
  phase for 15m (a failed backup is a data-loss risk until superseded or pruned).

### Notifications

Prometheus alerts cover operators watching a dashboard; for pushing events to
where a game-server admin actually lives — Discord, Slack, email, or any
webhook receiver — configure notification sinks under **Admin Settings →
Notifications**. No Helm values are involved: sinks are runtime config, with
credentials in labelled Secrets. Event types, Secret shapes, and the
test-send endpoint are documented in [notifications.md](notifications.md);
delivery health is visible at `/metrics` as
`gameplane_notify_deliveries_total`.

### Audit log

Every mutating API request is recorded to the `audit_events` table and served at
`GET /admin/audit` (and `GET /admin/audit/export` for a full CSV/JSON dump).
Beyond the database, the trail can be fanned out to external systems — each sink
**mirrors**, it never gates: events always land in the database regardless, and
a slow or down sink never blocks or fails a request.

- `api.audit.retentionDays` — prune events older than N days (`0` = keep
  forever, the default).
- `api.audit.stdout` — also emit each event as a structured JSON log line, for a
  cluster log aggregator (Loki/ELK/CloudWatch) scraping the pod's stdout.
- `api.audit.webhook.url` — POST each event as JSON to an HTTP receiver (a log
  aggregator's push endpoint, a SIEM, or your own collector). Delivery is
  best-effort from a bounded in-memory buffer; if the endpoint stalls, events
  are dropped rather than queued unboundedly. Watch
  `gameplane_audit_webhook_events_total{result="sent|failed|dropped"}` on
  `/metrics` to confirm the mirror is healthy.
- `api.audit.webhook.authSecretRef` — optional `Authorization` header for the
  webhook, sourced from a Secret (never a flag — see [security](security.md)).
- `api.audit.webhook.syslogBridge.enabled` — deploy the bundled
  [audit-syslog bridge](../audit-syslog-bridge/README.md) and point the webhook
  at it automatically, so events are forwarded to a **syslog** collector. Set
  `syslogBridge.syslog.addr` to your collector `host:port` (required when
  enabled), and optionally `network` (`tcp`/`udp`), `tls`, `facility`, and
  `severity`. Setting `webhook.url` explicitly overrides the auto-wiring.

  ```sh
  helm upgrade ... \
    --set api.audit.webhook.syslogBridge.enabled=true \
    --set api.audit.webhook.syslogBridge.syslog.addr=syslog.example:514
  ```

- `api.audit.s3.*` — native S3-compatible sink for batching audit events as
  NDJSON objects. Events are buffered in memory and flushed when ANY of three
  thresholds are hit: 100 events, 1 MiB, or 5 seconds. Upload uses S3 `PutObject`
  with retries (immediate/+2s/+8s); watch `gameplane_audit_s3_events_total`
  for delivery health. Works with AWS S3, MinIO, Backblaze, Wasabi, or any
  S3-compatible endpoint.
  - `api.audit.s3.endpoint` — S3 endpoint `host:port` (e.g.,
    `minio:9000` for a local MinIO, `s3.amazonaws.com` for AWS).
  - `api.audit.s3.bucket` — bucket name (required when endpoint is set).
  - `api.audit.s3.prefix` — optional object key prefix (e.g.,
    `gameplane-audit`; empty = root).
  - `api.audit.s3.region` — S3 region (e.g., `us-east-1`; empty = path-style
    requests).
  - `api.audit.s3.insecure` — `true` to skip TLS certificate verification
    (for self-signed certs on dev/homelab clusters).
  - `api.audit.s3.credentialsSecretRef` — reference to a Secret holding S3
    credentials (see [security](security.md)); leave `name` empty to disable S3.

  **MinIO homelab example**:

  ```sh
  # Create a Secret with MinIO credentials (user must have read/write on the bucket).
  kubectl create secret generic gameplane-s3-creds \
    -n gameplane-system \
    --from-literal=access-key=minioadmin \
    --from-literal=secret-key=minioadmin

  # Enable S3 sink pointing at local MinIO.
  helm upgrade ... \
    --set api.audit.s3.endpoint="minio.gameplane-system:9000" \
    --set api.audit.s3.bucket="gameplane-audit" \
    --set api.audit.s3.prefix="events" \
    --set api.audit.s3.insecure=true \
    --set api.audit.s3.credentialsSecretRef.name=gameplane-s3-creds \
    --set api.audit.s3.credentialsSecretRef.accessKeyKey=access-key \
    --set api.audit.s3.credentialsSecretRef.secretKeyKey=secret-key
  ```

### Telemetry

Gameplane can report anonymous usage once a day: `{version, servers,
templates}` — no names, namespaces, hostnames, or identifiers. Two
independent gates must both open before anything is sent: the admin
toggle (**Admin Settings → Telemetry → Send anonymous usage metrics**,
off by default) decides *whether*, and the chart decides *where*. With no
destination configured (the default), the reporter never runs.

- `api.telemetry.receiver.enabled` — deploy the bundled
  [telemetry-receiver](../telemetry-receiver/README.md) next to the API
  and point the API at it automatically. It logs each report and exposes
  aggregate Prometheus metrics (`gameplane_telemetry_reports_total` by
  version, fleet-size histograms) on its `/metrics`.
- `api.telemetry.endpoint` — send reports to an external receiver URL
  instead (e.g. `https://telemetry.example.com/ingest`); setting it
  overrides the receiver auto-wiring.
- `api.telemetry.authSecretRef` — optional shared ingest token, sourced
  from a Secret. The API sends it verbatim as the `Authorization` header
  and the bundled receiver requires it — recommended when the receiver is
  enabled, since its Service is reachable by other in-cluster pods.

  ```sh
  kubectl -n gameplane-system create secret generic telemetry-ingest \
    --from-literal=token='Bearer some-long-random-string'
  helm upgrade ... \
    --set api.telemetry.receiver.enabled=true \
    --set api.telemetry.authSecretRef.name=telemetry-ingest
  ```

## Installing a module

The chart ships two `ModuleSource`s: `default` (pulls pre-built bundles from the
official registry) and `uploads` (dashboard bundle uploads). The default uses
`type: oci` for zero-configuration access to versioned, optionally signed bundles;
`type: git` is available to track an unreleased branch directly from the
`gameplane-module` repository. Install games from the dashboard's **Modules** page,
or add more sources — git repositories, http archives, a local directory — under
**Modules → Manage sources** (admin) or by applying `ModuleSource` CRs. See
`docs/module-authoring.md` for the source types and the bundle format.

## Registering an additional cluster

Gameplane can manage game servers across multiple Kubernetes clusters
through a federation model. Each target cluster runs its own operator
instance; the control-plane cluster's API dispatches requests to the
target cluster via a `?cluster=<name>` parameter. See
[architecture.md](architecture.md#multi-cluster-federation) for the
design details.

### Prerequisites

Before registering a target cluster, ensure it has:

- Kubernetes 1.28+
- Gameplane operator and agent images accessible (same registry as the control-plane)
- A valid kubeconfig with admin credentials to manage Gameplane CRDs on that cluster

### Path 1: kubectl apply

1. Create a `kubeconfig` Secret in the control-plane's `gameplane-system` namespace.
   The Secret **must** be labelled `gameplane.local/cluster-kubeconfig=true`:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: my-cluster-kubeconfig
     namespace: gameplane-system
     labels:
       gameplane.local/cluster-kubeconfig: "true"
   type: Opaque
   data:
     kubeconfig: <base64-encoded kubeconfig for the target cluster>
   ```

   To base64-encode your kubeconfig:

   ```sh
   cat /path/to/target-cluster-kubeconfig.yaml | base64 -w0
   ```

2. Create the `Cluster` CRD referencing the Secret:

   ```yaml
   apiVersion: gameplane.local/v1alpha1
   kind: Cluster
   metadata:
     name: my-cluster
   spec:
     displayName: My Cluster
     kubeconfigSecret:
       name: my-cluster-kubeconfig
       # key is optional; defaults to "kubeconfig" if omitted
       key: kubeconfig
   ```

   **Cluster spec fields:**
   - `displayName` (optional): Human-readable name shown in the dashboard
   - `kubeconfigSecret.name` (required): Name of the Secret containing the kubeconfig
   - `kubeconfigSecret.key` (optional): Data key within the Secret; defaults to `"kubeconfig"`

3. Apply both to the control-plane cluster:

   ```sh
   kubectl apply -f secret.yaml -f cluster.yaml
   ```

4. Verify the cluster status:

   ```sh
   kubectl get clusters
   kubectl describe cluster my-cluster
   ```

The operator on the control-plane will reconcile the `Cluster` and
update `status.phase` (Unknown → Healthy/Unhealthy). When `Healthy`,
the API can dispatch requests to that cluster.

### Path 2: Dashboard API

POST to `/clusters` with permission `cluster:manage` (admin-only):

```sh
curl -X POST https://<dashboard>/api/clusters \
  -H "Content-Type: application/json" \
  -H "X-Gameplane-CSRF: <csrf-token>" \
  --cookie "session=<session-cookie>" \
  -d '{
    "name": "my-cluster",
    "kubeconfig": "<base64-encoded kubeconfig>"
  }'
```

The API stores the kubeconfig as a labelled Secret and creates the
`Cluster` CRD. The kubeconfig is never returned by the API and never
logged.

### Helm CRD caveat

**`helm upgrade` never updates CRDs.** That is a documented Helm limitation,
not a Gameplane one: files under a chart's `crds/` directory are installed on
first install and ignored on every upgrade thereafter.

The chart works around this for you. `crds.autoApply` (enabled by default)
ships a **pre-upgrade hook** that runs `kubectl apply --server-side` over the
current CRDs on every `helm upgrade`, so the `Cluster` CRD — and every other
Gameplane CRD — stays in step with the chart automatically. **No manual
`kubectl apply` step is needed.**

You only need to apply CRDs by hand if you have deliberately disabled the
hook:

```sh
# only when crds.autoApply.enabled=false
kubectl apply --server-side -f charts/gameplane/crds/
```

The hook is pre-upgrade *only*. A fresh install gets its CRDs from Helm's
native `crds/` handling, which needs no pod — so first installs, including
air-gapped ones, never depend on pulling the hook's `kubectl` image. CRDs are
never owned or deleted by Helm here, so `helm uninstall` leaves your
GameServers intact.

### RBAC and permissions

Registering a cluster grants **no implicit RBAC** on it — a user who
can start servers on the "local" cluster will not automatically be able
to do so on a newly registered cluster. Each cluster maintains its own
role bindings. To grant a user access to resources on the target
cluster, create matching role bindings there, or use the dashboard to
add cluster-scoped permissions if the target cluster's API also
supports the same RBAC model.

## Upgrading

```sh
helm upgrade gameplane oci://ghcr.io/valgulnecron/charts/gameplane \
  --version <new-version> \
  --namespace gameplane-system \
  --reuse-values
```

CRDs are brought up to date automatically by the chart's pre-upgrade hook —
see [Helm CRD caveat](#helm-crd-caveat).

**A caution on `--reuse-values`:** it replays your previous values *and skips
the new chart's defaults*, so any value key introduced since your last upgrade
arrives unset rather than defaulted. If an upgrade fails with a nil/missing
value, re-run it with `--reset-then-reuse-values`, or pass your values file
explicitly with `-f`. Keeping a values file under version control and using
`-f` is the sturdier habit.

### What upgrades are tested

CI runs an end-to-end upgrade on every PR (`e2e upgrade`, on both amd64 and
arm64): it installs the **previous release's published chart and published
GHCR images** into a fresh cluster, seeds a running GameServer with data on
its PVC and an admin user in the database, then upgrades to the chart under
test and asserts that

- the CRD schema was actually updated by the pre-upgrade hook,
- the GameServer survives and the bytes on its volume are unchanged,
- the pre-upgrade admin can still log in — i.e. the new binary's database
  migrations ran against the populated SQLite volume without data loss.

See `test/e2e/upgrade_e2e_test.go`. What this does **not** yet cover: upgrades
that skip several releases at once, and Postgres (still an experimental
driver — see [`roadmap.md`](roadmap.md)). Take a backup before upgrading
production either way.

CRDs are installed once by Helm and not updated on upgrade (by design).
For CRD schema changes, run:

```sh
kubectl apply -f charts/gameplane/crds/
```

### SQLite database adoption (Kestrel → Gameplane)

Installations that predate the Kestrel → Gameplane rename (v0.2.0-beta.2, July 2026)
and use the SQLite database driver will have their legacy `kestrel.db` file
automatically adopted on the first start of the new API. The adoption is
one-time and atomic: the file is renamed to `gameplane.db` in place, and a
WARN-level log entry records the event. If a `gameplane.db` already exists
(e.g., if this is not a fresh upgrade), the legacy file is left untouched
and the existing database is used instead — no data loss.

Nothing else needs to happen; the upgrade proceeds normally.

### SQLite upgrades (brief downtime)

When using the SQLite database driver, the API Deployment uses a `Recreate`
upgrade strategy: the old pod is fully terminated before the new one starts.
This ensures no two API processes try to write the same SQLite database file
(which is a single-writer store on a ReadWriteOnce PVC). As a result,
SQLite-backed installs experience a few seconds of dashboard downtime during
an upgrade — this is expected and deliberate. Postgres-backed installs (experimental)
would use rolling updates with no downtime, since the database is external and
shared, but full Postgres support remains a work-in-progress.
