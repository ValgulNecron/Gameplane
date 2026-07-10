# Install

## Prerequisites

- Kubernetes 1.28+
- Helm 3.13+
- A default StorageClass (any RWO CSI driver works)
- Optional: an ingress controller (nginx-ingress by default) + cert-manager for TLS

## One-shot install

The chart and its images are published to the GitHub Container Registry (GHCR)
as OCI artifacts — no `helm repo add` needed. Install a tagged release straight
from the registry (replace `<version>` with a release, e.g. `0.2.0-beta.5`):

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

Every published image (tagged releases and `:edge`) is signed with the
project's cosign key, [`cosign.pub`](../cosign.pub) at the repo root. The
signatures are offline/keyed (no transparency log), so pass the matching
flag:

```sh
cosign verify --key cosign.pub --insecure-ignore-tlog=true \
  ghcr.io/valgulnecron/gameplane/operator:<version>
```

The official module bundles are signed with the same key; the chart already
carries it, so bundle verification is just a values flip — see
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
- `api.db.driver` — `sqlite` (default) or `postgres`
- `api.db.dsn` — connection string; SQLite default persists to a PVC
- `api.oidc.enabled` + `issuer` / `clientID` / `clientSecretRef` — wire OIDC login
  from Helm (shows up as the read-only `helm` provider). Providers can also be
  added at runtime under **Admin Settings → Authentication** — no Helm values
  or restart needed; see [security](security.md#dashboard-managed-providers).
  Per-IdP walkthroughs (Keycloak, Authentik, Google) live in [oidc.md](oidc.md)
- `ingress.host` — dashboard hostname
- `networkPolicies.enabled` — default-deny in games namespace (recommended on)
- `podSecurity.enforceRestricted` — label games namespace for Pod Security Standards
- `defaultModuleSource.*` — the official game catalog shipped by default;
  `type: git` (default) indexes the public `gameplane-module` repo directly, or
  `type: oci` pulls signed bundles from a registry (`defaultModuleSource.oci.*`)
- `uploadModuleSource.enabled` — the `uploads` source backing dashboard bundle uploads (default on)
- `operator.localModules.{enabled,hostPath,existingClaim,mountPath}` — mount a
  directory of module bundles into the operator for `local`-type sources
- `serviceMonitors.enabled` / `prometheusRules.enabled` / `grafanaDashboards.enabled`
  — opt-in Prometheus Operator integration (see [Observability](#observability))

## Observability

The operator and API expose Prometheus metrics on `/metrics` (operator `:8080`,
API `:8000`). Three **off-by-default** chart toggles wire them into a
Prometheus-Operator stack (e.g. kube-prometheus-stack):

- `serviceMonitors.enabled` — `ServiceMonitor`s so Prometheus scrapes the
  operator and API.
- `prometheusRules.enabled` — a `PrometheusRule` of operator alerts.
- `grafanaDashboards.enabled` — a Grafana dashboard `ConfigMap` the Grafana
  sidecar auto-imports (relabel via `grafanaDashboards.labels` if your sidecar
  watches a different label).

All three add `labels:` you can set so a Prometheus/Grafana selector picks the
objects up.

### Metrics

Beyond the standard `controller-runtime_*` / `workqueue_*` series, the operator
exports **fleet gauges** computed from its cache at scrape time:

| Metric | Labels | Meaning |
|---|---|---|
| `gameplane_gameservers` | `phase` | GameServers per lifecycle phase (Pending/Starting/Running/Stopping/Stopped/Suspended/Failed) |
| `gameplane_backups` | `phase` | Backups per phase (Pending/Running/Succeeded/Failed) |

Every phase is always present (0 when empty). With 2+ operator replicas each
replica reports the same cache-derived counts, so aggregate with
`max by (phase) (...)` (the bundled dashboard and alerts already do).

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

The chart ships two `ModuleSource`s: `default` (the official OCI
registry catalog) and `uploads` (dashboard bundle uploads). Install
games from the dashboard's **Modules** page, or add more sources —
git repositories, http archives, a local directory — under
**Modules → Manage sources** (admin) or by applying `ModuleSource`
CRs. See `docs/module-authoring.md` for the source types and the
bundle format.

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

The `Cluster` CRD is installed by Helm on first deploy. **`helm upgrade`
never updates CRDs** — if you upgrade the chart and the CRD schema has
changed, manually apply the updated CRD:

```sh
kubectl apply -f charts/gameplane/crds/gameplane.local_clusters.yaml
```

After that, you can proceed with your `helm upgrade`.

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
helm upgrade gameplane ./charts/gameplane \
  --namespace gameplane-system \
  --reuse-values
```

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
an upgrade — this is expected and deliberate. Postgres-backed installs use
rolling updates with no downtime, since the database is external and
shared.
