# Security

## Threat model

Kestrel's dashboard is deliberately internet-exposed — that's the whole
point. Assume:

- the login page is enumerable by any scanner,
- cluster-internal attackers may land a pod in `kestrel-games` via a
  compromised game image (Minecraft plugins, Valheim mods, etc.),
- the game pods themselves should be treated as low-trust.

## Authentication

Two modes, configurable independently:

- **Local accounts** — argon2id (64 MiB, t=3, p=4) password hashing.
  Session cookies are HttpOnly, Secure, SameSite=Lax. CSRF protection
  via a double-submit `X-Kestrel-CSRF` header on mutating requests.
- **OIDC** — Keycloak, Google, GitHub, any RFC-7519 compliant IdP.
  State validated through a short-lived cookie; `id_token` signature
  verified against the provider's JWKS.

On first OIDC login for a subject, Kestrel creates a user row with
role `viewer`. Admins must promote new OIDC users manually.

## Authorization

Three cluster-scoped roles: `admin`, `operator`, `viewer`. Role-to-route
mapping lives in `api/internal/rbac/rbac.go`. Per-namespace and
per-GameServer scopes are planned for v1.1.

## API → Agent

mTLS. The Helm chart provisions a self-signed CA via a post-install
hook (or takes an existing `kestrel-agent-ca` Secret). The operator
uses the CA to sign per-pod server certs; the API uses a single client
cert. Agent refuses plain-HTTP traffic when TLS material is present.

Fallback: a shared-secret bearer token via `--api-token-file`. Only
intended for local `kind` development where mTLS is overkill.

## NetworkPolicies

When `networkPolicies.enabled=true` (default) the chart applies:

- `default-deny-ingress` in the games namespace
- `allow-api-to-agent` — API pod can reach agent:8090
- `allow-kubelet-probes` — kubelet can hit probes

Game clients connecting from outside the cluster use the per-GameServer
Service (ClusterIP/NodePort/LoadBalancer). Each Service adds an
allowIngress implicitly.

## Pod security

Every Kestrel-managed pod (operator, api, agent) runs as:

- `runAsNonRoot: true` (uid 65532)
- `readOnlyRootFilesystem: true`
- `seccompProfile.type: RuntimeDefault`
- `capabilities.drop: [ALL]`
- `allowPrivilegeEscalation: false`

Game pods are shaped per-template. For a hostile game module, enable
Pod Security Standards `restricted` on the games namespace via
`podSecurity.enforceRestricted=true`.

## Secrets

Secrets Kestrel reads or creates, by convention:

- `kestrel-<gameserver>-rcon` — per-game RCON password, created by operator
- `kestrel-agent-ca` — CA bundle the API trusts
- `kestrel-agent-client` — API's client cert/key
- `kestrel-oidc` — OIDC client secret (user-supplied)
- `kestrel-backup-repo` — restic repo URL + password (user-supplied)

Rotation: deleting the `-rcon` secret triggers a reconciliation and
generates a fresh password on the next pod restart.

## Pre-auth screens

No internal infrastructure metrics are displayed on the login page or
any other unauthenticated surface. This is a hard requirement — see
`web/src/routes/Login.tsx` for the enforcement.
