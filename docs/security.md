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

- **Local accounts** — argon2id (64 MiB, t=3, p=2) password hashing.
  Session cookies are HttpOnly, Secure, SameSite=Lax. CSRF protection
  via a double-submit `X-Gameplane-CSRF` header on mutating requests.
- **OIDC** — Keycloak, Google, GitHub, any RFC-7519 compliant IdP.
  State validated through a short-lived cookie; `id_token` signature
  verified against the provider's JWKS.

On first OIDC login for a subject, Gameplane creates a user row with
role `viewer`. Admins must promote new OIDC users manually.

### Dashboard-managed providers

OIDC providers can be added at runtime under **Admin Settings →
Authentication**: issuer + client id live in the auth config row (they
are public OAuth identifiers), and the client secret is stored as an
API-managed Secret `gameplane-auth-<name>` in the control-plane
namespace. Two labels bound the API's reach: it only *reads* Secrets
labelled `gameplane.local/auth-provider=true`, and only *deletes* ones
additionally labelled `gameplane.local/managed-by=gameplane-api` — so a
`config:manage` user can neither exfiltrate arbitrary control-plane
Secrets through a provider's `configRef` nor delete kubectl-/GitOps-
created ones over HTTP. Provider changes apply on save, no restart: the
registry re-reads the config row per auth request and rebuilds OIDC
clients lazily (issuer discovery cached, failures back off).

A provider configured through Helm flags (`api.oidc.*`) appears as the
read-only `helm` provider; it is owned by values.yaml and cannot be
edited, disabled, or deleted from the dashboard.

### Lockout guard and break-glass

Saving an auth config with zero enabled providers is rejected (the Helm
provider counts as always-enabled). If you still lock yourself out —
local login disabled while the only OIDC provider is broken — run the
break-glass inside the API pod:

```sh
kubectl -n gameplane-system exec deploy/gameplane-api -- \
  /api bootstrap-admin --enable-local-login
```

It force-enables the local provider in the auth config row (preserving
everything else) and takes effect on the next login attempt.

### Client IP extraction from forwarded headers

The API determines the real client IP from the `X-Forwarded-For` header
to power login rate limiting and audit records. This is configurable via
`api.trustedProxies` (default: private/loopback ranges `127.0.0.0/8`,
`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `169.254.0.0/16`,
`::1/128`, `fc00::/7`, `fe80::/10`).

**In a normal Kubernetes install** (API behind nginx-ingress, ALB, etc.),
the default works out-of-the-box: the ingress sits in one of the default
ranges and sets `X-Forwarded-For` unconditionally, so the API extracts the
true client IP safely. The API **only** trusts `X-Forwarded-For` from
requests originating within the configured CIDR blocks, defeating IP
spoofing.

**When the API is directly exposed** (no proxy), the default is correct:
`X-Forwarded-For` is ignored, and rate limiting uses the TCP peer's
address as the true client IP — which is already authoritative. If you
place a proxy in front of the API, add that proxy's address(es) to
`api.trustedProxies` so the API can extract the real client IP from
`X-Forwarded-For`.

Example for direct exposure behind a specific proxy at `203.0.113.1`:

```yaml
api:
  trustedProxies: "203.0.113.1/32"
```

The client IP is used for login rate limiting (per-IP caps) and audit
records, so misconfigurating this can either hide the real attacker's IP
in logs or prevent legitimate users from logging in if they're grouped
behind a proxy the API doesn't trust.

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

### Per-GameServer access (owner + collaborators)

In addition to namespace-based RBAC, GameServers support ownership and
collaboration: the **owner** (who created the server) and any **collaborators**
(managed via `PUT /servers/{name}:collaborators`) gain operational control over
that specific server, regardless of their namespace role. This is purely additive
— it does not override namespace bindings. Collaborators retain: read, console,
WebSocket access, start/stop/restart/clone operations, and files/players/config
subroutes. Destructive operations are owner-only: delete, wipe-data, ownership
transfer, and collaborator list edits. Only the owner and users holding the
namespace `servers:write` permission can perform owner-only operations. Backups,
restore jobs, schedules, and events remain namespace-gated in this release.

## API → Agent

mTLS. The Helm chart provisions a self-signed CA via a post-install
hook (or takes an existing `gameplane-agent-ca` Secret). The operator
uses the CA to sign per-pod server certs; the API uses a single client
cert. Agent refuses plain-HTTP traffic when TLS material is present.

Fallback: a shared-secret bearer token via `--api-token-file`. Only
intended for local `kind` development where mTLS is overkill.

## NetworkPolicies

When `networkPolicies.enabled=true` (default) the chart applies:

- `default-deny-ingress` — denies all ingress to all pods in the games
  namespace. A Kubernetes Service does not create any NetworkPolicy
  allowance; ingress traffic is fully isolated unless an allow-policy
  explicitly permits it.
- `default-deny-egress` — denies all egress from all pods in the games
  namespace except DNS (UDP/TCP port 53 to `kube-system`). This is the
  most restrictive policy; outbound downloads (binaries, assets, mods) are
  gated by the `allow-game-public-egress` policy below, and apiserver
  access by `allow-agent-to-apiserver`.
- `allow-agent-to-apiserver` — allows game pods to reach the kube-apiserver
  for status heartbeat (GameServer status patches). Permits TCP 443 and 6443
  to apiserver endpoints; by default targets RFC1918 + link-local ranges, or
  customizable via `networkPolicies.apiServerCIDRs`.
- `allow-api-to-agent` — allows the API and operator pods (in the
  control-plane namespace) to reach every game pod's agent on TCP port 8090.
  The API proxies console/files/logs/players; the operator calls `/quiesce`
  before backups.
- `<gameserver-name>-game-ingress` — created by the **operator** for each
  GameServer, allows external traffic to reach only the advertised ports
  declared in the GameTemplate. Selects traffic from `networkPolicies.gameIngress.fromCIDRs`
  (default `0.0.0.0/0`, the internet) to only the container ports marked
  `Advertise: true` at their declared protocol (UDP preserved). This policy
  does not itself open RCON or the agent's port 8090; however, those ports
  remain reachable from `kubeletCIDRs` via the `allow-kubelet-probes` policy
  (see below), which has no port restrictions by default. To close that hole,
  narrow `networkPolicies.probePorts` — previously unsafe to do, but now
  safe since this policy protects advertised player traffic separately.
  Operators may also narrow `fromCIDRs` to a private range (e.g. a LAN, a VPN
  CIDR) to gate player access. If a template declares no advertised ports or
  `networkPolicies.gameIngress.enabled: false`, the operator ensures this
  policy does not exist. The policy is owned by the GameServer and cascade-deletes
  with it.
- `allow-kubelet-probes` — allows kubelet to reach game pods for
  liveness/readiness probes. By default targets RFC1918 + link-local ranges,
  or customizable via `networkPolicies.kubeletCIDRs`; probe ports via
  `networkPolicies.probePorts`.
- `allow-game-public-egress` — (enabled by default, gated by
  `networkPolicies.gameEgress.enabled`) allows game pods to reach the public
  internet for binary/asset/mod downloads. Set `networkPolicies.gameEgress.enabled: false`
  to withhold public egress. Permits egress to 0.0.0.0/0 except private ranges
  defined in `networkPolicies.gameEgress.privateCIDRs` (by default: 10.0.0.0/8,
  172.16.0.0/12, 192.168.0.0/16, 169.254.0.0/16 to block in-cluster and
  cloud-metadata access). Ports customizable via `networkPolicies.gameEgress.ports`.

**Note:** `default-deny-egress` applies to all pods in the games namespace
(`podSelector: {}`), while `allow-agent-to-apiserver` and
`allow-game-public-egress` only select game pods labelled
`app.kubernetes.io/name: gameplane-game`. Any pod in the games namespace
without that label receives DNS-only egress (connecting to kube-dns on port
53). Do not place helper or debug pods in the games namespace expecting them
to route traffic—place them in a different namespace instead.

## Pod security

Every Gameplane-managed pod (operator, api, agent, and the optional
audit-syslog-bridge) runs as:

- `runAsNonRoot: true` (uid 65532)
- `readOnlyRootFilesystem: true`
- `seccompProfile.type: RuntimeDefault`
- `capabilities.drop: [ALL]`
- `allowPrivilegeEscalation: false`

Game pods are shaped per-template. For a hostile game module, enable
Pod Security Standards `restricted` on the games namespace via
`podSecurity.enforceRestricted=true`.

### Network Capture Security Exception

The optional network capture feature (see
[`docs/roadmap.md`](roadmap.md)) adds an ephemeral sidecar container to game pods when
capture is enabled. This sidecar requires **`allowPrivilegeEscalation: true`** in
its securityContext, which violates the Pod Security Standards `restricted`
profile.

**Why the exception is necessary**: The sidecar acquires `CAP_NET_RAW` capability
via file capabilities (`setcap cap_net_raw+ep`), applied at container image build
time. This mechanism is necessary because Kubernetes does not set ambient
capabilities on a container's process by default: declaring
`securityContext.capabilities.add: ["NET_RAW"]` alone on a non-root user grants
nothing — the kernel clears the effective capability set when the entrypoint
binary is executed (via `execve`), leaving the non-root process with no
capabilities. File capabilities survive this exec because they are consulted
independently by the kernel at exec time, independent of ambient state.

The container's `securityContext.capabilities` still lists
`Drop: ["ALL"], Add: ["NET_RAW"]`, but that `Add` is not what grants the
capability — it exists only because `Drop: ["ALL"]` on its own would also empty
the process's *bounding* capability set, and the kernel refuses to grant a file
capability at `execve` that isn't in the bounding set (EPERM). Re-adding NET_RAW
keeps it available in the bounding set so the file capability grant can proceed;
the process's own *effective* set is still empty at start, and the actual grant
comes from the setcap'd binary at exec, not from `capabilities.add`. Separately,
file capabilities are ignored by the kernel when `no_new_privs` is set, and
Kubernetes sets `no_new_privs` whenever `allowPrivilegeEscalation: false`.
Therefore, the container must also set `allowPrivilegeEscalation: true` for the
file capability to function.

The game container retains its unprivileged posture: `runAsNonRoot: true`,
`allowPrivilegeEscalation: false`, and no elevated capabilities. Only the capture
sidecar holds `CAP_NET_RAW`; exploit of game code cannot grant packet-capture
ability.

**Trade-off with PodSecurity `restricted`**: A cluster enforcing the `restricted`
Pod Security Standards profile on the games namespace will reject any pod with
`allowPrivilegeEscalation: true`. If you require `restricted` admission on your
games namespace, you have three options:

1. **Disable capture** — leave the cluster's capture feature disabled via Helm
   value `capture.enabled: false` (default is true). Captures are not required
   for normal operation; this is the safest option if you cannot or prefer not to
   relax the `restricted` profile.
2. **Exempt the games namespace** — remove or relax the Pod Security Standards
   `restricted` enforcement for the games namespace, using `baseline` or
   `privileged` instead. The games namespace remains an untrusted environment
   (game code can run arbitrary containers), but the admission level permits the
   capture sidecar to be injected when needed.
3. **Disable `restricted` cluster-wide** — if the games namespace is managed by
   your deployment and you accept the operational trade-off, set `podSecurity.enforceRestricted=false`
   in the Helm values. Captures will work, and other pods are not forced into
   `restricted` mode (they can still opt in per-pod via labels).

**Data sensitivity**: Captures contain binary game protocols, player IP addresses,
and may include sensitive data like in-game chat or credentials. An admin with
`captures:manage` permission can start a capture and read all traffic reaching
that pod, including other players' packets. This access is admin-only and fully
audited (every capture operation is logged). Default capture retention is 24 hours,
reducing the exposure window for captured data.

## Module supply chain

A `GameTemplate` materialized from a module chooses the container image,
command, and config a game pod runs — so a module source is a trust
boundary. Three controls protect it:

- **Fetch SSRF guard.** The operator's `git`/`http` source fetchers
  (`netguard.IsAllowed`) refuse link-local, cloud-metadata
  (`169.254.169.254`), unspecified, and multicast destinations, at dial time
  (so a DNS name rebinding to one is caught). This blocks a source from being
  aimed at the instance-metadata endpoint to steal the operator's IAM
  credentials. Private/loopback addresses stay reachable for self-hosted
  registries. `ModuleSource` mutation is admin-only, so this is
  defense-in-depth.
- **Signature verification.** `ModuleSource.spec.verify` (OCI sources) makes
  the operator refuse any bundle without a valid cosign signature — keyed (a
  public key) or keyless (a pinned Fulcio issuer + identity). Use it for any
  source you don't fully control. The official `modules/*` bundles are
  keyed-signed by the release pipeline (and now also recorded in the public
  Sigstore Rekor log), and verify **offline** (no Rekor/Fulcio reachability
  needed). The operator's verification is intentionally offline/keyed, keeping
  air-gapped and self-hosted clusters functional. Opt-in enforcement of
  transparency log inclusion is future work. Signing is an OCI concept, so
  switch the default source to `type: oci` and enable
  `defaultModuleSource.oci.verify.enabled`.
- **Digest pinning.** `Module.spec.digest` pins exact bundle content; a moved
  tag fails the install with `DigestMismatch`.

Verification and pinning are opt-in. A source with neither is trusted to
serve a `GameTemplate` whose image/command runs in your cluster — only point
Gameplane at module sources you trust, and prefer signed, pinned installs for
third-party games. Authoring details: [`module-authoring.md`](module-authoring.md).

## Runtime mod installs (agent)

Separately from the module supply chain above, a running server can install
mods/plugins at runtime if its template declares
`capabilities.mods.install` — a user-supplied URL the agent downloads into
the server's data volume (see
[`module-authoring.md`](module-authoring.md#mods)). This is a distinct trust
boundary: the target is whatever host the logged-in user types, not an
admin-configured `ModuleSource`, so it is guarded more strictly
(`netguard.IsPublic`): only globally routable addresses are allowed —
loopback, private/ULA, link-local, and CGNAT/reserved ranges are all
refused, not just the cloud-metadata range. An `allowedHosts` allow-list is
also required before installs are enabled at all, and redirects are
re-checked against both the host allow-list and the address guard. This
guard and the operator's fetch guard above share their dial-time enforcement
machinery in the `netguard/` module; its package doc explains why the two
policies (`IsAllowed` vs `IsPublic`) stay separately selectable rather than
being collapsed into one.

## Notifications

Notification sinks ([docs](notifications.md)) are a third outbound-dial
surface, between the two above in trust: the URLs are configured at runtime
through the dashboard (unlike the deploy-time audit webhook flag), but only
by users holding `config:manage` — admin-tier, the same trust class as the
operator's `ModuleSource`s. They get the same guard: every sink dial
(HTTP and SMTP) goes through `netguard.IsAllowed`, so LAN/in-cluster
receivers (ntfy, a syslog bridge, an SMTP smarthost) keep working while
link-local (cloud metadata), unspecified, multicast, and NAT64/6to4
destinations are refused at dial time — DNS rebinding can't slip past.
Two further containments: sink credentials resolve only from Secrets
labelled `gameplane.local/notification-sink=true` in the control-plane
namespace (so a sink `configRef` can't be aimed at an arbitrary Secret),
and delivery errors are sanitized to never echo the sink URL, whose path
often embeds a capability token.

## Audit log integrity

`audit_events` is a hash chain (migration `005_audit_chain.sql`): every row
inserted after that migration stores `hash = SHA-256(prev_hash ||
canonical(row))`, and `GET /admin/audit/verify` re-walks the chain to report
the first broken link. Two config-table entries bound the walk: a
`Prune`-written checkpoint anchors the oldest surviving row after a
retention sweep, and a per-insert head anchors the newest row, so a
`DELETE FROM audit_events WHERE id > N` — truncating only the tail, which
would otherwise leave every surviving link internally consistent — is
detected too.

**Be precise about what this catches.** The chain is unkeyed: it recomputes
hashes from row content and two config-table entries, and `config` is
writable by anyone with the same database access an attacker would need to
tamper with `audit_events` in the first place. This mechanism reliably
detects:

- naive in-DB tampering — `UPDATE`/`DELETE` (including tail truncation)
  against `audit_events` alone, without also touching `config`; and
- accidental corruption (a bad migration, a restore from an inconsistent
  backup, etc).

It does **not** detect a sophisticated attacker who has DB write access and
also recomputes and rewrites the checkpoint and head to match — that
attacker can forge an internally-consistent chain from any starting point.
Nothing server-side can close that gap while the verification data lives in
the same database the attacker can already write to.

**The real append-only record of last resort is the external sinks** —
stdout (cluster log aggregation), the audit webhook, and the S3 batch sink
(see [Secrets](#secrets) below for how their credentials are contained).
Because delivery is push-based and decoupled from the request path, an
attacker who compromises the database after the fact cannot retroactively
alter what was already shipped to those destinations. Treat the hash chain
as tamper-*evidence* for common-case tampering and corruption, and the
external sinks as the actual tamper-*proof* trail.

A documented future hardening is HMAC-keyed chaining (`hash =
HMAC-SHA256(key, prev_hash || canonical(row))` with the key held outside the
database — e.g. a K8s Secret the API process reads but never writes back),
which would raise the bar to compromising that external key as well. Not
implemented today; tracked in [`roadmap.md`](roadmap.md).

## GitHub Actions supply chain and CI security

Gameplane's release binaries and container images are built via GitHub Actions.
The CI pipeline is a trust boundary: compromised build steps can inject malicious
code into what users deploy. Several controls harden the pipeline:

### Action pinning and mutable-tag defense

External GitHub Actions in `.github/workflows/` and `.github/actions/` are pinned
to a 40-character commit SHA with an inline `# vX.Y.Z` semver comment, never to
a mutable tag like `@v4`. The threat this closes: a compromised or malicious
action maintainer can repoint the tag at new code, giving arbitrary code execution
inside CI with that workflow's `GITHUB_TOKEN` privileges — a supply-chain attack
on the entire user base.

Pinning to an immutable commit SHA is enforced mechanically by the `zizmor` linter
in the `workflow-lint` job (`.github/zizmor.yml` config). Dependabot's
`github-actions` ecosystem entry maintains pins through reviewable pull requests,
so the operational trade-off — pins grow stale and miss security updates — is
managed rather than ignored: every update lands as a visible, auditable PR before
it ships.

### Least-privilege token grants and scope confinement

The threat: a compromised or buggy build step can use the full scope of the
`GITHUB_TOKEN`. An over-broad top-level `permissions` grants those scopes to
every job and step, multiplying the blast radius.

The defense: `.github/workflows/ci.yaml` and `release.yaml` now grant
`permissions: {contents: read}` at the top level — the minimum viable scope —
and elevate scopes only on the specific job(s) that need them, in an explicit
per-job `permissions` block. Concrete reductions from earlier config:

- **ci.yaml**: `statuses: write` (needed only for the `web` job to mark PR
  checks) was inherited by every job; it now lives on `web` alone.
- **release.yaml**: top-level `permissions` was `{contents: write}`, granting
  write access to every release job; it is now `{contents: read}` at the top,
  with `contents: write` elevated only to the `github-release` job.

Every job in every workflow has an explicit `timeout-minutes` to bound the
duration a compromised job can run (see below).

### Expression injection and shell-escaping discipline

The threat: attacker-controlled PR text — `github.event.pull_request.title`,
`.body`, `github.head_ref` — can be interpolated directly into a `run:` shell
body without quoting, giving arbitrary shell execution. A malicious PR title
like `"; rm -rf /; #"` then becomes a command.

The safe pattern: pass attacker-controlled values through the `env:` block and
reference them as quoted shell variables (`"$VAR"`, not `$VAR`), so they are
treated as data, not code. Example:

```yaml
env:
  PR_TITLE: ${{ github.event.pull_request.title }}
run: echo "Title is: \"$PR_TITLE\""
```

The `actionlint` linter (run in the `workflow-lint` job) detects the unsafe form
— direct interpolation of `github.event.*` or `github.head_ref` into `run:` —
and rejects it. This repo does not use `pull_request_target` (the trigger that
runs workflows on untrusted fork code with the base repo's `GITHUB_TOKEN`),
closing the attack surface entirely: `pull_request_target` was designed for
workflows that *must* access repo secrets (e.g., automated releases), but it
introduces risk if any step trusts PR body content as code.

### Timeouts as denial-of-service and cost bounds

Every job carries an explicit `timeout-minutes` to bound job duration. The threat
is twofold: a compromised job could hang indefinitely (DoS on CI capacity and
cost), and a build failure could leave a job in a partially-modified state if the
termination is not clean.

Default timeout budget is ≤30 minutes. Documented exceptions with inline
justification comments:

- The five e2e jobs in ci.yaml run at 60 minutes (the `e2e-go` job uses a
  `job_timeout` matrix value; `e2e-multicluster`, `e2e-upgrade`, `e2e-web-live`,
  `e2e-game-bot` set it directly). E2E test suites on kind clusters are
  inherently slow; 60 minutes is measured from prior runs.
- **publish-edge.yaml**: the `images` job runs at 35 minutes (measured 31-minute
  historical max for full image build and push across all components).

### Diagnostics redaction and secret confinement

The threat: CI failures on a public repository produce world-readable artifacts
(downloaded test logs, pod state dumps, `$GITHUB_STEP_SUMMARY` markdown). Any
unredacted secret — a pod env var, a log line, a manifest dump — becomes public
and compromised immediately. Multi-cluster environments and complex setups make
this harder to spot: a cluster dump is hundreds of lines and secrets can hide in
labels, annotation values, or environment variable lists.

The defense: `.github/actions/dump-cluster-state/action.yml` applies a `redact()`
filter at **every** emit boundary — before any data reaches `$GITHUB_STEP_SUMMARY`,
before any artifact is uploaded, before logs are written. The filter is
**conservative**: it redacts *values* (preserving *keys* so dumps stay debuggable)
by matching known patterns:

- Labelled key/value pairs: any value for keys containing `password`, `passwd`,
  `token`, `secret`, `api`, `key`, `bearer`, or `authorization`
  (case-insensitive, with optional dashes/underscores).
- Bare tokens: JWT-shaped values (`eyJ...`), PEM private-key blocks.

An important limitation: **redaction is pattern-based and therefore best-effort.**
A credential in an unrecognised shape — a long hex string, a custom token format,
or a value buried in a JSON log without a recognisable key — may not be caught.
No Kubernetes Secret object is ever collected into dumps (`for obj in deployments
statefulsets daemonsets jobs configmaps`), so high-entropy database passwords and
OIDC secrets bound to the pod via Secrets are outside the dump scope entirely.

Operators should: treat CI artifacts as sensitive (not suitable for sharing with
untrusted parties without review), verify no live credentials appear in failures,
and rotate any that do immediately. This is not a substitute for not logging
credentials in the first place.

### Automated code review and trust model

Optional feature: the repository uses the **CodeRabbit GitHub App** for automated
code review (configured in `.coderabbit.yaml`). The app is structurally safer than
a self-hosted API-key reviewer:

- **No repository secret in untrusted job**: A self-hosted API-key reviewer would
  require placing a repository secret (e.g., `ANTHROPIC_API_KEY`) inside a job
  that checks out untrusted fork code. If the fork is malicious, it can
  exfiltrate the secret from `$GITHUB_TOKEN`, env vars, or the runner's
  filesystem. The CodeRabbit app is a GitHub App, not a personal API key — it
  integrates via GitHub's OAuth flow and never places a repository secret
  alongside untrusted code.
- **No trust in PR content**: Review is advisory (cannot block a merge) and
  treats PR content as data to review, not as instructions to follow or commands
  to execute. The app's configuration (`path_instructions`, `labeling_instructions`)
  is repository-local and not influenced by PR branches.
- **No implicit privilege escalation**: The app cannot request scopes it was not
  granted at install time, and it cannot modify its own permissions.

The review is advisory — a human reviewer must still validate changes before merge
— and the tool can be disabled at any time via GitHub's app management UI.

## mcp-server (optional)

The optional MCP server (`mcpServer.enabled`, see [`mcp-server/README.md`](../mcp-server/README.md))
is strictly read-only — no tool it exposes can create, update, patch,
delete, or apply anything, enforced structurally (its tool handlers only
ever hold a client whose exported methods are List/Get-shaped) and by RBAC
(a ClusterRole granting only `get`/`list`/`watch`, plus `get` on
`pods/log`).

That RBAC grant is **cluster-wide**, not scoped to `gameplane-games` or any
other single namespace: the server can list/read Pods, Events, and pod logs
in every namespace, including `kube-system` and any other workload's
namespace sharing the cluster. Pod logs in particular can surface secrets
an application logs at startup or during errors (API keys, connection
strings, stack traces) — Kubernetes has no mechanism to redact those.
Combined with write-freedom and opt-in, admin-only installation
(`mcpServer.enabled` plus whatever gates `kubectl exec` access to the
`gameplane-mcp-server` pod), this is an accepted tradeoff, not an oversight
— but install it knowing that anyone who can reach a `serve` session gets
read access to cluster-wide pod state and logs, not just Gameplane-managed
namespaces. If that blast radius is wider than acceptable for a given
cluster, don't enable `mcpServer` there.

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
- audit S3 credentials — any Secret you reference via
  `api.audit.s3.credentialsSecretRef` (user-supplied). The access key and secret
  key are injected as env vars (`GAMEPLANE_AUDIT_S3_ACCESS_KEY`,
  `GAMEPLANE_AUDIT_S3_SECRET_KEY`), never flags, so they do not appear in the pod
  spec or `ps` output.
- notification sinks — any Secret labelled
  `gameplane.local/notification-sink=true` in the control-plane namespace
  (user-supplied; referenced by name from Admin Settings → Notifications, read
  by the API at delivery time — see [notifications](notifications.md)).

Rotation: deleting the `-rcon` secret triggers a reconciliation and
generates a fresh password on the next pod restart.

## Kubeconfig Secret handling

In a multi-cluster setup, each target cluster is referenced by a Secret
containing its kubeconfig. Access to cluster credentials is protected
by several layers:

- **Label guard.** The API only reads Secrets labelled
  `gameplane.local/cluster-kubeconfig=true` when registering a cluster
  via the dashboard or API. This prevents a user from pointing at an
  arbitrary control-plane Secret (e.g., the OIDC client secret or
  backup credentials) and using it as a kubeconfig.
- **Never logged or returned.** The kubeconfig is never logged by the
  API, never echoed in responses, never visible in audit trails. It
  exists only to bootstrap the Kubernetes client for that cluster.
- **Permission gating.** Only users holding the `cluster:manage`
  permission (admin-only) can register, list, or delete clusters via
  the API. Dashboard access to `/clusters` is similarly gated.
- **No implicit RBAC.** Registering a cluster does not grant any user
  access to resources on that cluster. Access is determined by role
  bindings created independently on the target cluster, not by
  federation. See [install.md](install.md#rbac-and-permissions).

## Install-Time OIDC Role Mappings

When OIDC authentication is configured at install time via Helm values
(`api.oidc.groupsClaim`, `api.oidc.roleMappings`, `api.oidc.defaultRole`),
Gameplane automatically assigns roles to users based on their OIDC provider's
group/role claims on every login. This eliminates the need for a bootstrap-admin
account in OIDC-only deployments — an operator can configure group mappings at
install time and the first user to log in receives the correct role immediately.
The security model consists of:

### Trust Chain: IdP Group Membership → Gameplane Roles

**Core risk**: Gameplane trusts the IdP's group claim unconditionally. Whoever
controls IdP group membership effectively controls Gameplane role assignments.
If an attacker compromises the IdP or its group directory (LDAP, Active Directory,
cloud identity service), they can add themselves to a mapped group and gain that
group's Gameplane role on their next login — up to and including admin access.

This is an accepted architectural boundary: the IdP is a trust root. If the IdP
is compromised, Gameplane cannot defend against that. Mitigation is at the IdP
level: strong authentication to the IdP, audit logging of group membership changes,
and monitoring for suspicious group additions.

### Helm-Seeded Values vs. Database Overrides (Hybrid Model)

Install-time values (`api.oidc.roleMappings.*`) seed the role-mapping policy in
the database when the API starts. An admin can then override one or more roles'
group lists through the dashboard (`PUT /admin/config/auth` with `helmOverride`)
at runtime, without restarting the API or re-running Helm. Each role's effective
mapping is determined independently:

- **Database override present** (even if an empty list `[]`): That list is the
  effective mapping for that role, used on every login. An empty list means
  "nobody maps to this role from any group" — a valid and meaningful override
  distinct from "no override set."
- **Database override absent**: The Helm-seeded value is the effective mapping for
  that role.

When an operator runs `helm upgrade` and changes a Helm value (`api.oidc.roleMappings.*`),
the new value updates the seed — but it does NOT overwrite a database override that
has already been set for that role. The override persists until explicitly reset via
the dashboard (`DELETE /admin/config/auth/role-mappings/{role}`). This is deliberate:
Helm upgrades should not silently undo admin customizations made through the UI.

Consequences for operators: the effective role mappings in the dashboard may not
match what is in `values.yaml` after one or more roles have been dashboard-overridden.
To audit what is actually configured, consult the dashboard's `/admin/config` view
(`installTimeSettings.oidcHelmProvider` shows the Helm seed, `auth.helmOverride`
shows any database overrides) rather than relying on Helm values alone.

### Most-Privileged-Match Rule Across Sources

When resolving a user's role, Gameplane matches the user's groups against every
role's effective mapping (seed + overrides merged) and assigns the **highest
privilege match**: `admin` > `operator` > `viewer`. This is applied *after* the
per-role merge of Helm seed and database override, so a user matching both an
overridden (database-managed) viewer group *and* a Helm-seeded admin group still
resolves to `admin`.

Overriding a lower role does **not** revoke a higher one. Example: if the database
overrides the `viewer` list to `[]` (nobody maps to viewer), but the Helm-seeded
`admin` list is `["admins"]`, a user in the "admins" group still resolves to `admin`
on the next login — the admin mapping was never overridden.

### Re-evaluation on Every Login

On each OIDC login, Gameplane:

1. Extracts the user's group membership from the OIDC token's group claim (configured
   via `api.oidc.groupsClaim`; defaults to `"groups"`).
2. Reads the effective role mappings: the Helm-seeded values, merged with any
   database overrides for each role independently.
3. Matches the user's groups against the effective mappings to compute their role.
4. If no role matches, assigns the default role (configured via `api.oidc.defaultRole`;
   defaults to `viewer`; can be set to `deny` to reject login).

This re-evaluation runs only when Helm OIDC role mappings are configured
(i.e., `api.oidc.roleMappings` has at least one non-empty role array). If role
mappings are not configured, new OIDC users receive the fixed `viewer` role and
existing users' roles are never re-evaluated.

Two guards prevent lockout during re-evaluation:

- **No lockout rule at login**: If re-evaluating a user's role would remove the
  last user able to manage users (hold the `users:manage` permission), the
  re-evaluation is **not applied** — that user retains their old role and can
  still manage other users. This prevents an operator from accidentally creating
  an unrecoverable lockout via an override change.
- **Break-glass mechanism**: If role mappings are misconfigured such that nobody
  can reach admin, an operator can run the `bootstrap-admin` break-glass command
  to create a local admin account and fix the mappings. Bootstrap-admin and
  OIDC-mapped admin accounts coexist peacefully.

### Admin-Mapping Warning (FR-015)

Gameplane **always warns** when an operator configures or changes a role mapping
that includes a group. The warning text states: "Be aware that an OIDC group may
include a large number of users, and assigning it to the admin role grants admin
access to all members of that group." This warning is unconditional — it appears
on every such configuration change, not just when the operator first enables role
mappings.

**Why unconditional**: Gameplane cannot enumerate OIDC group membership — it cannot
tell whether a group name refers to a 3-person team or a 3000-person organization.
An attacker with dashboard access who knows the group structure could configure a
mapping for an unexpectedly large group to gain access. Operators must be aware of
this risk at every configuration step. The warning does not prevent the change, but
it ensures operators cannot claim they were not aware of the risk.

### Audit Trail: Every Assignment, Override, and Reset

Every role assignment driven by OIDC group mappings (on initial login or re-evaluation)
is recorded in `audit_events` with the matched group name and role transition:

- **Action**: OIDC login with role assignment
- **Target**: the user (subject of the OIDC token)
- **Details recorded**:
  - Which OIDC provider performed the assignment (always `"helm"` for Helm-seeded mappings)
  - Which group matched a mapping rule (or `"none"` if no mapping matched)
  - The user's old role (`"new_user"` on first login, or the previous role)
  - The assigned role (`"viewer"`, `"operator"`, `"admin"`, or `"denied"` if rejected)

Examples of what gets logged:
- First login, matched admin group: user created with admin role from group membership
- Re-evaluation, no mapping match: role re-evaluated to default role on next login
- Subsequent login, role upgraded: user's role changed from viewer to operator based on new group membership

Every dashboard change to a role's override — writing a new group list via
`PUT /admin/config/auth` or resetting it via `DELETE /admin/config/auth/role-mappings/{role}`
— is also audited as a configuration change:

- **Action**: Role mapping override write or reset
- **Target**: the affected role (`"admin"`, `"operator"`, or `"viewer"`)
- **Details recorded**:
  - Which admin made the change (from session)
  - The new or reset value (the group list, if changed)

This allows operators to track who changed what mappings and when, and to
correlate unexpected role assignments with dashboard configuration changes.

### `groupsClaim` and `defaultRole` Are Helm-Only

`api.oidc.groupsClaim` (the claim name from the OIDC token that holds group
information) and `api.oidc.defaultRole` (the fallback role when no group matches)
are configured via Helm values only — there is no dashboard write path for either
in v1.

**Why this matters for security**: An attacker with dashboard admin access can
override individual role mappings but **cannot** repoint the group claim to a
different OIDC token field (e.g., changing from `"groups"` to a field they control),
nor can they change the default role fallback to `"deny"` to lock everyone out. These
two settings remain under the operator's full control, via Helm values only, and
require a `helm upgrade` to change — an out-of-band action that can be audited and
gated by access controls on the cluster itself (e.g., who can run Helm in production).

## Pre-auth screens

No internal infrastructure metrics are displayed on the login page or
any other unauthenticated surface. This is a hard requirement — see
`web/src/routes/Login.tsx` for the enforcement.
