# T045 chart chunk: independent verification (opus)

Held under OD-019, off-git. This file covers the five candidates in `held/review-charts-gameplane.md`. It is worded defensively: for each item it names the control, where it lives, the correct behaviour, and how a maintainer confirms the control holds. It contains no misuse walkthroughs.

Method: I checked each candidate against master `13a859ff`. None of the cited files differ between this branch and master. I read `charts/gameplane/templates/{ingress,mtls,api,operator,networkpolicies}.yaml`, `charts/gameplane/values.yaml`, `web/nginx.conf.template`, `api/cmd/main.go:245-262,494-500`, `api/specs.md:96-102,556-560`, `api/internal/audit/s3.go` and `s3_test.go`, the `getEndpointURL` function in minio-go v7.3.0 (the version in `api/go.mod:31`), `api/internal/auth/oidc.go:103-130,430-470`, `operator/internal/controller/agent_certs.go:60-90`, and the relevant parts of `docs/install.md` and `docs/security.md`. I rendered the chart with `helm template` using default values. No cluster was reachable from this session, so nothing was checked live. No test or lint suite was run. No repo file was changed; this file is off-git. I checked `audit/findings.md` and the other held files for duplicates and found none.

Severity follows research R3.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-charts-gameplane-H01 | kept | S3 | Confirmed. The API registers `/metrics` on its root router with no auth (`api/cmd/main.go:260`). `web/nginx.conf.template:94-115` hands any request that is not an HTML navigation to the API. The default ingress routes `/` to `gameplane-web`, or to `gameplane-api` when web is off (`ingress.yaml:19-25`). Nothing blocks the path. The metrics are low-sensitivity: Go/process series plus the audit-webhook, audit-S3 and notification delivery counters, which show whether those sinks are configured and failing. Even so, the stated rule is broken (`docs/security.md:786-790`, `api/specs.md:560`, CLAUDE.md rule 3). S3, not S1, because nothing it exposes is a credential or grants access. `api/specs.md:102` lists `/metrics` as a public route on the API listener. That describes in-cluster scraping and does not settle exposure on the public host. |
| C-charts-gameplane-H02 | kept | S3 | Confirmed. `NewS3Sink` sets `Secure: !cfg.Insecure` and installs no custom transport (`s3.go:71-75`). In minio-go v7.3.0, `getEndpointURL` picks the `http` scheme whenever `secure` is false. `s3_test.go` runs every case with `Insecure: true` against a plain-HTTP `httptest` server, so the tests pin "insecure means plain HTTP". The docs (`values.yaml:206`, `install.md:359-360`, flag help `main.go:498`) say the option only skips certificate verification. The admin opts in, and the result is either failed uploads or an unencrypted mirror, so S3. |
| C-charts-gameplane-H03 | kept | S4 | Confirmed, but the severity is lowered to S4. `install.md:131-134` says an empty `groupsClaim` disables group mapping, and `:147-148` repeats the premise. The code reads the `groups` claim when the setting is empty (`defaultGroupsClaim`, `oidc.go:110-116`, used by `extractGroups` at `:122` and at `:439-444`), so mapping is active whenever Helm or dashboard role mappings exist. `values.yaml:247-248` and `security.md:699-700` state the real behaviour. Mapping only grants roles when an admin has configured mappings, so no role is assigned that an admin did not map. The defect is a wrong statement in one doc, and S4 fits. |
| C-charts-gameplane-H04 | kept | S3 | Confirmed. The CA and client Secrets are ordinary templates, not hooks (`mtls.yaml:24-35`, `:54-65`), and they always render under the fixed names `gameplane-agent-ca` and `gameplane-agent-client`. Helm will not adopt a pre-created Secret that lacks its ownership metadata, so the documented bring-your-own path (`mtls.yaml:8-10`, `security.md:161-162`, `values.yaml:276-277`) fails at install. Only the `.name` of `caSecretRef` and `clientCertRef` is read (`api.yaml:401,403`, `operator.yaml:280,360,362`). `caSecretRef.key`, `clientCertRef.key` and both `clientKeyRef` fields are never read, and the operator reads fixed keys `ca.crt`/`ca.key` (`agent_certs.go:75-82`). The generated CA works, and adding Helm ownership metadata to a pre-created Secret is a workaround, so S3. |
| C-charts-gameplane-H05 | kept | S3 | Confirmed by render. With `apiServerCIDRs` unset (the default), `allow-agent-to-apiserver` (`networkpolicies.yaml:45-71`) selects every `gameplane-game` pod and admits egress to 10/8, 172.16/12, 192.168/16 and 169.254/16 on TCP 443 and 6443. NetworkPolicy allows are additive and cover the whole pod, including the game container. That undoes, for those two ports, the private-range exception that `allow-game-public-egress` and its docs present as the anti-SSRF control (`values.yaml:336-341`, `networkpolicies.yaml:141-145`, `security.md:212-215`). `security.md:182-185` does say the default targets RFC1918 and link-local, but no doc links that to the anti-SSRF promise. The documented `apiServerCIDRs` knob narrows the rule, so S3. |

### C-charts-gameplane-H01

**Location:** `api/cmd/main.go:260` (`r.Handle("/metrics", promhttp.Handler())` on the root router, outside every auth group); `web/nginx.conf.template:94-115` (`location /` → `@backend`, which proxies every request that is not an HTML navigation); `charts/gameplane/templates/ingress.yaml:19-25` (the default `/` backend). Defaults: `ingress.enabled: true`, `web.enabled: true`.

**Control:** pre-authentication information exposure. `docs/security.md:786-790` says no internal infrastructure metrics appear on any unauthenticated surface, and calls this a hard requirement. `api/specs.md:560` ("No internal metrics visible pre-auth") and CLAUDE.md rule 3 say the same.

**Repro / observation** (defensive; confirms whether the control is in place):
1. Read `web/nginx.conf.template:54-66` and `:94-115`. The only paths nginx serves locally are real files, `/assets/` and `/nginx-health`. `$force_proxy` lists `/auth/` and download paths. There is no rule that keeps `/metrics` local or refuses it.
2. Read `api/cmd/main.go:245-262`. `/metrics` and `/healthz` sit on the root router before the auth groups.
3. On a default install, request `/metrics` on the ingress host with no session and a non-HTML `Accept` header. With the control in place the response is 401, 403 or 404, not the Prometheus text format.

**Expected (correct behaviour):** The public host does not serve Prometheus metrics to unauthenticated clients. In-cluster scraping still works: the chart's API ServiceMonitor scrapes the Service directly (`servicemonitors.yaml:42-45`), which an nginx `location = /metrics { return 404; }` or a separate metrics listener would leave untouched.

**Actual:** Nothing in the chart or the nginx config keeps `/metrics` off the public host, so it is proxied to the API like any other non-HTML request.

**How a maintainer confirms it holds:** add a check that the web image's nginx config answers `/metrics` locally (404) and never proxies it, for example a CI step that runs the image and requests the path. Also check live on the release candidate's ingress host, as in step 3.

### C-charts-gameplane-H02

**Location:** `api/internal/audit/s3.go:71-75` (`minio.Options{Secure: !cfg.Insecure, ...}`, no `Transport`). Documented at `charts/gameplane/values.yaml:206`, `docs/install.md:359-360` and `api/cmd/main.go:498`. Tests: `api/internal/audit/s3_test.go:70-78` and the other `Insecure: true` cases.

**Control:** transport security for the audit mirror, which carries the audit trail and SigV4-signed requests made with the credentials from `api.audit.s3.credentialsSecretRef`.

**Repro / observation** (by reading code):
1. `minio-go/v7@v7.3.0/utils.go` `getEndpointURL`: for an endpoint without a scheme, it uses `https` if `secure` is true and `http` otherwise.
2. `NewS3Sink` passes `Secure: !cfg.Insecure` and no custom `http.RoundTripper`, so `insecure: true` switches to plain HTTP. It does not keep TLS and skip verification.
3. Every `s3_test.go` case sets `Insecure: true` and points at a plain-HTTP `httptest.NewServer`, so the suite depends on the current meaning.

**Expected (correct behaviour):** One of two things. The option does what the docs say: it keeps `https` and sets `InsecureSkipVerify` on a custom transport, with the plain-HTTP test cases moved to a separate, explicitly named option. Or the docs and flag help say plainly that `insecure: true` means plain HTTP with no TLS.

**Actual:** The option disables TLS entirely, while all three docs describe it as skipping certificate verification only.

**How a maintainer confirms it holds:** a unit test asserts that the sink's client endpoint scheme is `https` when only certificate verification is skipped, and that a self-signed `httptest.NewTLSServer` is reachable with that option. If the chosen fix is the documentation, then `values.yaml:206`, `install.md:359-360` and `main.go:498` all say "plain HTTP".

### C-charts-gameplane-H03

**Location:** `docs/install.md:131-134` and `:147-148`, compared with `api/internal/auth/oidc.go:110-116` (`defaultGroupsClaim`), `:122` (`extractGroups`) and `:439-444` (the callback reads groups with the configured or defaulted claim).

**Control:** OIDC group-to-role authorization mapping (spec `done_006` FR-007..FR-012).

**Repro / observation:**
1. `install.md:131-134`: "`groupsClaim` ... (default `""`) ... Empty/omitted = group-based role mapping disabled; new OIDC users default to `defaultRole`".
2. `oidc.go:110-116`: an empty claim name becomes `"groups"`. `computeRole` then applies any configured role mappings to those groups.
3. `values.yaml:247-248` ("Default: "groups" if empty or omitted") and `security.md:699-700` agree with the code. `install.md:149-150` ("Omitting `groupsClaim` and `roleMappings` disables ...") is correct, because it is the empty mappings that disable mapping.

**Expected (correct behaviour):** All docs say the same thing: an empty `groupsClaim` means the `groups` claim is read, and mapping is active whenever role mappings exist (Helm-seeded or set from the dashboard).

**Actual:** install.md alone says an empty `groupsClaim` disables mapping.

**How a maintainer confirms it holds:** after aligning install.md, the three docs agree. A unit test covers `GroupsClaim: ""` together with non-empty `RoleMappings` and asserts that the mapped role is assigned from the `groups` claim.

### C-charts-gameplane-H04

**Location:** `charts/gameplane/templates/mtls.yaml:1-65`; `charts/gameplane/values.yaml:276-281`; `charts/gameplane/templates/api.yaml:400-403`; `charts/gameplane/templates/operator.yaml:280`, `:359-362`; `operator/internal/controller/agent_certs.go:69-82`. Claims: `docs/security.md:161-162`, `mtls.yaml:8-10`, `values.yaml:276-277`.

**Control:** provisioning of the API↔agent mTLS key material (CA and API client certificate).

**Repro / observation:**
1. `mtls.yaml` has no `helm.sh/hook` annotation. Both Secrets are plain release resources, which contradicts "post-install hook" in `security.md:161` and `values.yaml:277`.
2. Both Secrets always render, under the names `gameplane-agent-ca` and `gameplane-agent-client`, whatever `api.agentMTLS.*.name` says. `lookup` reuses existing data, but Helm still has to own the object. A pre-created Secret without Helm's release annotations and managed-by label is refused at `helm install` with an ownership error.
3. `grep -n agentMTLS charts/gameplane/templates/*.yaml` finds only `caSecretRef.name` and `clientCertRef.name`. The `.key` fields and `clientKeyRef` are never read. The API mounts whole Secrets at fixed filenames, and the operator reads fixed keys `ca.crt`/`ca.key`.
4. If the refs name other Secrets, the chart still generates and stores a CA private key under the default name that nothing uses.

**Expected (correct behaviour):** A user-supplied CA and client certificate are honoured as documented. Either the chart skips generating Secrets whose names were overridden (and documents the key names it expects), or the docs describe the real procedure, including the Helm ownership metadata. Every `api.agentMTLS` field is either read by a template or removed from `values.yaml`.

**Actual:** The documented bring-your-own path fails at install, and four of the six `api.agentMTLS` fields do nothing.

**How a maintainer confirms it holds:** in a scratch namespace, pre-create a CA Secret as the docs describe and run `helm install`. The install succeeds and the Secret's data is unchanged. `helm template --set api.agentMTLS.caSecretRef.name=my-ca` renders no generated `gameplane-agent-ca`, or the docs state that it does.

### C-charts-gameplane-H05

**Location:** `charts/gameplane/templates/networkpolicies.yaml:45-71` (`allow-agent-to-apiserver`, fallback ipBlocks on TCP 443/6443 for `gameplane-game` pods) and the sentinel equivalent at `:204-230`. Claims: `charts/gameplane/values.yaml:332-341`, `networkpolicies.yaml:141-145`, `docs/security.md:182-185` and `:212-215`.

**Control:** egress isolation of game pods from in-cluster and private-network endpoints (the anti-SSRF / tenant-isolation statement for game egress).

**Repro / observation** (by rendering):
1. `helm template gameplane charts/gameplane -n gameplane-system` with default values. Collect every NetworkPolicy in `gameplane-games` whose `podSelector` matches `app.kubernetes.io/name: gameplane-game` and has `Egress` in its `policyTypes`.
2. The union is: DNS to kube-system (`default-deny-egress`); `0.0.0.0/0` minus the four private ranges on TCP 80/443 (`allow-game-public-egress`); and the four private ranges themselves on TCP 443/6443 (`allow-agent-to-apiserver`, because `apiServerCIDRs` is empty).
3. So for TCP 443 and 6443, the private-range exception in rule 2 is cancelled by rule 3, for every container in the pod, not only the agent.

**Expected (correct behaviour):** Among private addresses, game pods reach only the apiserver endpoint(s). Either `apiServerCIDRs` gets a narrower default (for example resolved from the `kubernetes` Endpoints at install), or the docs say plainly that with the default the game container can reach any private or link-local address on 443/6443, and tell operators to set `apiServerCIDRs`.

**Actual:** With the default values, the anti-SSRF exception does not hold for ports 443 and 6443.

**How a maintainer confirms it holds:** render with defaults and check that no egress rule selecting `gameplane-game` admits a private-range destination wider than the apiserver endpoint(s). If the fix is in the docs instead, check that `values.yaml:336-341` and `security.md:212-215` name the 443/6443 allowance and the `apiServerCIDRs` setting. On a live cluster with `apiServerCIDRs` set to the apiserver endpoint's /32, check that game pods can still heartbeat.
