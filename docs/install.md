# Install

## Prerequisites

- Kubernetes 1.28+
- Helm 3.13+
- A default StorageClass (any RWO CSI driver works)
- Optional: an ingress controller (nginx-ingress by default) + cert-manager for TLS

## One-shot install

```sh
helm repo add gameplane https://charts.gameplane.gg     # (once the chart is published)
helm upgrade --install gameplane gameplane/gameplane \
  --namespace gameplane-system --create-namespace \
  --set ingress.host=gameplane.your-domain.test
```

From source (during development):

```sh
helm upgrade --install gameplane ./charts/gameplane \
  --namespace gameplane-system --create-namespace
```

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
- `defaultModuleSource.*` — the OCI registry catalog shipped by default
- `uploadModuleSource.enabled` — the `uploads` source backing dashboard bundle uploads (default on)
- `operator.localModules.{enabled,hostPath,existingClaim,mountPath}` — mount a
  directory of module bundles into the operator for `local`-type sources

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
