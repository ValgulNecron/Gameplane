# Install

## Prerequisites

- Kubernetes 1.28+
- Helm 3.13+
- A default StorageClass (any RWO CSI driver works)
- Optional: an ingress controller (nginx-ingress by default) + cert-manager for TLS

## One-shot install

```sh
helm repo add kestrel https://charts.kestrel.gg     # (once the chart is published)
helm upgrade --install kestrel kestrel/kestrel \
  --namespace kestrel-system --create-namespace \
  --set ingress.host=kestrel.your-domain.test
```

From source (during development):

```sh
helm upgrade --install kestrel ./charts/kestrel \
  --namespace kestrel-system --create-namespace
```

## First-time setup

Seed an initial admin user. Passwords must be at least 12 characters.

```sh
kubectl -n kestrel-system exec deploy/kestrel-api -- \
  /api bootstrap-admin --username admin --password "<choose>"
```

To avoid the password landing in your shell history, pipe it on stdin:

```sh
printf '%s' "$ADMIN_PASSWORD" | kubectl -n kestrel-system exec -i deploy/kestrel-api -- \
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

## Installing a module

Modules ship as CRDs. Apply whichever games you want:

```sh
kubectl apply -f modules/minecraft-java/template.yaml
kubectl apply -f modules/valheim/template.yaml
```

## Upgrading

```sh
helm upgrade kestrel ./charts/kestrel \
  --namespace kestrel-system \
  --reuse-values
```

CRDs are installed once by Helm and not updated on upgrade (by design).
For CRD schema changes, run:

```sh
kubectl apply -f charts/kestrel/crds/
```
