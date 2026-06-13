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

## Module supply chain

A `GameTemplate` materialized from a module chooses the container image,
command, and config a game pod runs — so a module source is a trust
boundary. Three controls protect it:

- **Fetch SSRF guard.** The operator's `git`/`http` source fetchers refuse
  link-local, cloud-metadata (`169.254.169.254`), unspecified, and multicast
  destinations, at dial time (so a DNS name rebinding to one is caught). This
  blocks a source from being aimed at the instance-metadata endpoint to steal
  the operator's IAM credentials. Private/loopback addresses stay reachable
  for self-hosted registries. `ModuleSource` mutation is admin-only, so this
  is defense-in-depth.
- **Signature verification.** `ModuleSource.spec.verify` (OCI sources) makes
  the operator refuse any bundle without a valid cosign signature — keyed (a
  public key) or keyless (a pinned Fulcio issuer + identity). Use it for any
  source you don't fully control.
- **Digest pinning.** `Module.spec.digest` pins exact bundle content; a moved
  tag fails the install with `DigestMismatch`.

Verification and pinning are opt-in. A source with neither is trusted to
serve a `GameTemplate` whose image/command runs in your cluster — only point
Kestrel at module sources you trust, and prefer signed, pinned installs for
third-party games. Authoring details: [`module-authoring.md`](module-authoring.md).

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
