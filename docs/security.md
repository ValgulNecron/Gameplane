# Security

## Threat model

Gameplane's dashboard is deliberately internet-exposed — that's the whole
point. Assume:

- the login page is enumerable by any scanner,
- cluster-internal attackers may land a pod in `gameplane-games` via a
  compromised game image (Minecraft plugins, Valheim mods, etc.),
- the game pods themselves should be treated as low-trust.

## Authentication

Two modes, configurable independently:

- **Local accounts** — argon2id (64 MiB, t=3, p=4) password hashing.
  Session cookies are HttpOnly, Secure, SameSite=Lax. CSRF protection
  via a double-submit `X-Gameplane-CSRF` header on mutating requests.
- **OIDC** — Keycloak, Google, GitHub, any RFC-7519 compliant IdP.
  State validated through a short-lived cookie; `id_token` signature
  verified against the provider's JWKS.

On first OIDC login for a subject, Gameplane creates a user row with
role `viewer`. Admins must promote new OIDC users manually.

## Authorization

RBAC is **permission-based**. A *permission* is a fixed `resource:action`
string from the server-defined catalog (`api/internal/rbac/catalog.go`,
e.g. `servers:write`, `backups:restore`, `users:manage`). A *role* is a
named set of permissions, and a user is bound to roles **per namespace**.

- **Roles** live in the API database (`roles` / `role_permissions`). The
  built-in `admin`, `operator`, and `viewer` roles are seeded so their
  cluster-wide grants reproduce the historical role matrix exactly. `admin`
  holds the `*` wildcard and is immutable; `operator`/`viewer` are editable
  templates; custom roles can be created with any subset of the catalog (the
  `*` wildcard is never grantable through the API). Built-in roles and roles
  still assigned to a user cannot be deleted.
- **Bindings** (`user_role_bindings`) grant a role in a namespace; `*` means
  cluster-wide. A user's primary role (`PATCH /users/{id}`) is their
  cluster-wide binding; additional per-namespace grants are managed via
  `…/users/{id}/bindings`. Allowed namespaces are the `GAMEPLANE_EXTRA_NAMESPACES`
  allow-list plus the default `gameplane-games`.
- **Enforcement** (`api/internal/rbac/rbac.go`): each route maps to one
  required permission; the middleware resolves the request's target namespace
  and checks the caller's resolved permission set. A *namespaced* permission
  is granted by a cluster-wide binding **or** a binding in the target
  namespace; a *cluster-scoped* permission requires a cluster-wide binding —
  the same Role vs ClusterRole split Kubernetes uses. Unmatched routes fail
  closed.
- **Lockout guards.** The API refuses to demote or delete the last user who
  can manage users, and refuses self-demotion below `users:manage`.

Per-GameServer (owner-based) authorization remains future work; server
ownership today is informational only.

## API → Agent

mTLS. The Helm chart provisions a self-signed CA via a post-install
hook (or takes an existing `gameplane-agent-ca` Secret). The operator
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

Every Gameplane-managed pod (operator, api, agent) runs as:

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
  source you don't fully control. The official `modules/*` bundles are
  keyed-signed by the release pipeline and verify **offline** (no Rekor/Fulcio
  reachability needed). Signing is an OCI concept, so switch the default source
  to `type: oci` and enable `defaultModuleSource.oci.verify.enabled`.
- **Digest pinning.** `Module.spec.digest` pins exact bundle content; a moved
  tag fails the install with `DigestMismatch`.

Verification and pinning are opt-in. A source with neither is trusted to
serve a `GameTemplate` whose image/command runs in your cluster — only point
Gameplane at module sources you trust, and prefer signed, pinned installs for
third-party games. Authoring details: [`module-authoring.md`](module-authoring.md).

## Secrets

Secrets Gameplane reads or creates, by convention:

- `gameplane-<gameserver>-rcon` — per-game RCON password, created by operator
- `gameplane-agent-ca` — CA bundle the API trusts
- `gameplane-agent-client` — API's client cert/key
- `gameplane-oidc` — OIDC client secret (user-supplied)
- `gameplane-backup-repo` — restic repo URL + password (user-supplied)
- audit-webhook auth — any Secret you reference via
  `api.audit.webhook.authSecretRef` (user-supplied). The token is injected as an
  env var, never a flag, so it does not appear in the pod spec or `ps` output.

Rotation: deleting the `-rcon` secret triggers a reconciliation and
generates a fresh password on the next pod restart.

## Pre-auth screens

No internal infrastructure metrics are displayed on the login page or
any other unauthenticated surface. This is a hard requirement — see
`web/src/routes/Login.tsx` for the enforcement.
