# Install

## Prerequisites

- Kubernetes 1.28+
- Helm 3.13+
- A default StorageClass (any RWO CSI driver works)
- Optional: an ingress controller (nginx-ingress by default) + cert-manager for TLS

## One-shot install

The chart and its images are published to the GitHub Container Registry (GHCR)
as OCI artifacts — no `helm repo add` needed. Install a tagged release straight
from the registry (replace `<version>` with a release, e.g. `0.2.0-beta.4`):

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
- `api.db.driver` — `sqlite` (default) or `postgres`
- `api.db.dsn` — connection string; SQLite default persists to a PVC
- `api.oidc.enabled` + `issuer` / `clientID` / `clientSecretRef` — wire OIDC login
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

## Installing a module

The chart ships two `ModuleSource`s: `default` (the official OCI
registry catalog) and `uploads` (dashboard bundle uploads). Install
games from the dashboard's **Modules** page, or add more sources —
git repositories, http archives, a local directory — under
**Modules → Manage sources** (admin) or by applying `ModuleSource`
CRs. See `docs/module-authoring.md` for the source types and the
bundle format.

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
