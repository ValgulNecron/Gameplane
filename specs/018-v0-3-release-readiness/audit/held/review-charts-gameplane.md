# Held review candidates: charts/gameplane/ (OD-019, off-git)

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Source review**: `audit/evidence/review-charts-gameplane/notes.md` (T042, chunk "chart")
- **Why held**: each candidate concerns a security control (network exposure, transport security for a secret-bearing sink, authorization mapping, key material handling). Each is written defensively: the control, where it lives, the correct behaviour, and how a maintainer confirms it holds. There are no reproduction recipes.

## Candidate findings

### C-charts-gameplane-H01: the API's unauthenticated `/metrics` is reachable through the default public ingress, contradicting the pre-auth "no internal metrics" rule
- **Location**: `api/cmd/main.go:260` (`r.Handle("/metrics", promhttp.Handler())` on the root router, outside any auth group); `web/nginx.conf.template:94-115` (`location /` → `@backend`, which proxies every request that isn't an HTML navigation to the API); `charts/gameplane/templates/ingress.yaml:20-25` (default `/` → `gameplane-web`, or → `gameplane-api` when web is disabled); defaults `ingress.enabled: true`, `web.enabled: true`
- **Category**: correctness
- **Suggested severity**: S3
- **Control**: pre-auth information exposure. `docs/security.md:786-790`: "No internal infrastructure metrics are displayed on the login page or any other unauthenticated surface. This is a hard requirement". CLAUDE.md rule 3 (Login Privacy) says the same.
- **Expected (correct behaviour)**: Prometheus metrics (Go runtime and process series, `gameplane_audit_webhook_events_total`, `gameplane_audit_s3_events_total`, `gameplane_notify_deliveries_total{kind,result}`) are not served to unauthenticated clients through the public ingress. Monitoring still reaches them in-cluster (the chart's API ServiceMonitor scrapes the Service directly).
- **Actual**: Nothing in the chart or the nginx config keeps `/metrics` off the public host. It is proxied to the API like any other non-HTML path.
- **How a maintainer confirms it holds**: on a default install, request `/metrics` on the ingress host without a session, from a non-browser client. With the control in place, the response is not the Prometheus exposition format (404/401/403). Also check `web/nginx.conf.template` and `ingress.yaml` for an explicit block or allow-list of that path.

### C-charts-gameplane-H02: `api.audit.s3.insecure: true` turns TLS off for the S3 audit sink; the docs say it only skips certificate verification
- **Location**: `api/internal/audit/s3.go:71-75` (`minio.Options{Secure: !cfg.Insecure, ...}` with no custom transport); documented at `charts/gameplane/values.yaml:206`, `docs/install.md:359-360` and the flag help at `api/cmd/main.go:498`
- **Category**: correctness
- **Suggested severity**: S3
- **Control**: transport security for the audit mirror, which carries the audit trail and SigV4-signed requests made with the credentials from `api.audit.s3.credentialsSecretRef`.
- **Expected (correct behaviour)**: As documented, "disable TLS certificate verification (for self-signed certs)": the connection still uses TLS, and only the certificate chain check is skipped. Alternatively, the docs say plainly that the option switches to plain HTTP.
- **Actual**: In minio-go, `Secure: false` selects the `http` scheme. An admin who sets `insecure: true` for a self-signed HTTPS endpoint gets either failed uploads (HTTP to an HTTPS port) or, where the endpoint also serves HTTP, an unencrypted audit mirror.
- **How a maintainer confirms it holds**: read `NewS3Sink`. The documented behaviour needs `Secure: true` plus a transport with `InsecureSkipVerify` when `Insecure` is set. Check that a unit test asserts the client's endpoint scheme is `https` when `insecure=true`.

### C-charts-gameplane-H03: install.md says an empty `api.oidc.groupsClaim` disables group-based role mapping; the code defaults the claim to `groups`, so mapping stays active
- **Location**: `docs/install.md:131-134` vs `api/internal/auth/oidc.go:109-116,122` (`defaultGroupsClaim`, `extractGroups`) and `:439-454`
- **Category**: docs-drift
- **Suggested severity**: S3
- **Control**: OIDC group → role authorization mapping (spec `done_006` FR-007..FR-012).
- **Expected (correct behaviour)**: One consistent statement of when mapping is active. values.yaml:247-248 ("Default: "groups" if empty or omitted") and security.md:699-700 ("defaults to `"groups"`") agree with the code.
- **Actual**: install.md:131-134 says: "`groupsClaim` ... (default `""`) ... Empty/omitted = group-based role mapping disabled; new OIDC users default to `defaultRole`". install.md:147-148 repeats that `defaultRole` is "Meaningful only if `groupsClaim` and `roleMappings` are configured". An admin who follows install.md and leaves `groupsClaim` empty expects every OIDC user to get `defaultRole`, but roles are still assigned from the IdP's `groups` claim whenever `roleMappings` is set.
- **How a maintainer confirms it holds**: compare the three docs sentences with `extractGroups`/`computeRole`. Once install.md is aligned, all three agree, and a unit test covers `GroupsClaim: ""` with non-empty `RoleMappings`.

### C-charts-gameplane-H04: the documented "bring your own agent CA" path doesn't work, and the `api.agentMTLS.*.key` / `clientKeyRef` values are never read
- **Location**: `charts/gameplane/templates/mtls.yaml:8-10,12-35,37-65`; `charts/gameplane/values.yaml:276-281`; `charts/gameplane/templates/api.yaml:400-403`; `charts/gameplane/templates/operator.yaml:280,359-362`; `operator/internal/controller/agent_certs.go:75-81`; claim at `docs/security.md:161-162`
- **Category**: correctness
- **Suggested severity**: S3
- **Control**: API↔agent mTLS key material (CA and client certificate provisioning).
- **Expected (correct behaviour)**: security.md:161-162 says the chart "provisions a self-signed CA via a post-install hook (or takes an existing `gameplane-agent-ca` Secret)". mtls.yaml:8-10 says users "can pre-create `gameplane-agent-ca` and `gameplane-agent-client`; the lookup path then reuses them". values.yaml:276-277 says "When unset, the chart provisions a self-signed CA + cert via a post-install hook." A user-supplied CA should therefore be honoured and no unused CA key generated.
- **Actual**:
  1. The Secrets are ordinary templates, not a hook.
  2. The chart renders Secrets named `gameplane-agent-ca` and `gameplane-agent-client` unconditionally. Helm refuses to adopt a pre-created Secret that lacks Helm ownership metadata, so the documented pre-create path fails at `helm install` unless the user adds that metadata.
  3. If the refs are pointed at other Secret names, the chart still generates a fresh CA and private key under the default names, which nothing uses.
  4. `caSecretRef.key`, `clientCertRef.key`, `clientKeyRef.key` and `clientKeyRef.name` appear in no template. The API mounts whole Secrets at fixed filenames (`ca.crt`, `tls.crt`, `tls.key`), and the operator reads `ca.crt`/`ca.key` by fixed key.
- **How a maintainer confirms it holds**: in a scratch namespace, pre-create a CA Secret as the docs describe and run `helm install`. With the control documented correctly, the install succeeds and the Secret's content is unchanged. `helm template` with non-default ref names should render no generated CA Secret, and every `api.agentMTLS` field should be consumed by a template or removed from values.yaml.

### C-charts-gameplane-H05: the default `allow-agent-to-apiserver` egress rule opens every RFC1918 and link-local address on TCP 443/6443 to game pods, contradicting the anti-SSRF claim for game egress
- **Location**: `charts/gameplane/templates/networkpolicies.yaml:45-71` (fallback ipBlocks 10/8, 172.16/12, 192.168/16, 169.254/16 on 443 and 6443, `podSelector: gameplane-game`); the sentinel's equivalent at `:204-230`; claims at `charts/gameplane/values.yaml:336-341`, `networkpolicies.yaml:141-145` and `docs/security.md:212-215`
- **Category**: correctness
- **Suggested severity**: S3
- **Control**: game-pod egress isolation (network exposure / SSRF surface).
- **Expected (correct behaviour)**: values.yaml:340-341: "Private ranges are excepted so game pods still cannot reach in-cluster services or link-local metadata (anti-SSRF)". security.md:212-215 lists the excepted ranges "to block in-cluster and cloud-metadata access". Game containers should reach only the apiserver endpoint(s) among private addresses.
- **Actual**: NetworkPolicy allows are additive and apply to the whole pod, including the game container, not only the agent sidecar. With `apiServerCIDRs` unset (the default), the heartbeat policy admits any private or link-local destination on 443 and 6443. That covers in-cluster pods serving on those ports and, on homelab installs, LAN hosts' HTTPS endpoints. security.md:182-185 documents that the default targets RFC1918, but no doc connects this to the anti-SSRF promise.
- **How a maintainer confirms it holds**: render the chart with defaults and take the union of egress rules whose podSelector matches `app.kubernetes.io/name: gameplane-game`. The control holds when no rule admits a private-range destination other than the apiserver endpoint(s): either `apiServerCIDRs` defaults to something narrower, or the docs state the residual allowance explicitly.

## Questions (not findings)

- `api.db.dsn` goes to the API as a CLI flag (`charts/gameplane/templates/api.yaml:248`). For `db.driver=postgres` the example DSN embeds a password (values.yaml:131-132), which then appears in the pod spec and the process arguments. Webhook and S3 credentials, by contrast, come from Secrets as env vars (security.md:593-600). Should the DSN get a `secretRef` before postgres leaves experimental status?
- The `api.trustedProxies` default trusts all of RFC1918 (values.yaml:125). No NetworkPolicy restricts ingress to the API in the release namespace, so any in-cluster workload is a "trusted proxy" for `X-Forwarded-For`. That affects the per-IP login limiter and audit IPs for in-cluster callers. security.md:72-74 ("defeating IP spoofing") is accurate only for callers outside those ranges. Is this the intended trust boundary?
- `gameplane-web`, `gameplane-audit-syslog-bridge` and `gameplane-telemetry-receiver` run under the namespace's default ServiceAccount with the token automounted (no `automountServiceAccountToken: false`), though none of them calls the Kubernetes API (web.yaml, audit-syslog-bridge.yaml:10-13, telemetry-receiver.yaml:8-11). Hardening question only.
- The operator-injected images `busybox:1.38.0` and `restic/restic:0.19.1` (values.yaml:90-91) and the hook image `registry.k8s.io/kubectl:v1.36.3` (values.yaml:38) are pinned by tag, not digest. SECURITY_AUDIT.md §4 covers base-image digest pinning of Gameplane's own images, not these runtime images. Should they be digest-pinned?
- `allow-kubelet-probes` (networkpolicies.yaml:109-138), with default `kubeletCIDRs: []` and `probePorts: []`, admits any source in RFC1918 or link-local to any port on game pods. Pod CIDRs are usually RFC1918, so this covers every in-cluster pod, not only kubelets. security.md:195-199 documents the port side ("no port restrictions by default") but not that the source side covers ordinary pods. Should the docs say so?
