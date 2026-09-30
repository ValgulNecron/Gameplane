# Held review candidates: operator (OD-019)

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `operator/specs.md`, `docs/architecture.md` ("Security boundaries"), `docs/security.md` ("Module supply chain"), `docs/tunnels.md`, `specs/done_003-network-capture-sidecar/` (spec.md FR-003, contracts/capture-sidecar.md), CRD doc comments in `operator/api/v1alpha1/`
- **Companion notes**: `audit/evidence/review-operator/notes.md` (held candidates: 10)

Each entry below concerns a security control: supply-chain integrity, secret handling, network exposure, authorization/RBAC or TLS. Each is described as the control, where it lives, the correct behaviour, and how a maintainer confirms that the control holds. None has reproduction steps.

## Candidate findings

### H-operator-01: Module bundle content is not bound to the digest that signature verification checks
- **Location**: `operator/internal/oci/client.go:94-131` (`Pull`); `operator/internal/modsrc/oci.go:106-113`; `operator/internal/controller/module_controller.go:118-132`; `operator/internal/verify/verify.go:85-97`
- **Category**: correctness (supply-chain control)
- **Suggested severity**: S2
- **Control**: `ModuleSource.spec.verify` (cosign). `docs/security.md` ("Signature verification") and the package doc at `verify.go:1-4` say the control exists "so a compromised registry can't serve a forged GameTemplate".
- **Observation**: `Pull` returns `manifestDesc.Digest` from oras `FetchReference` by tag. For a tag reference, oras v2.6.2 takes that digest from the registry's `Docker-Content-Digest` response header when present, and does not hash the body (`registry/remote/repository.go` `generateDescriptor`, around lines 1606-1647). The operator never hashes `manifestBytes`, and `readBlob` never hashes layer bytes against `layer.Digest`: oras `blobStore.Fetch` only compares header values (`verifyContentDigest`, around lines 1684-1709). The module controller then verifies the signature for `ref@<that digest>` and materializes the template from the bytes it received.
- **Correct behaviour**: The manifest and every layer consumed by `FromFiles` are cryptographically bound to the digest passed to `verifier.Verify`. For example: compute `digest.FromBytes(manifestBytes)` and require it to equal the resolved digest (or resolve first, then fetch by digest and check); verify each blob with a digest verifier before use.
- **How a maintainer confirms it holds**: Read `oci/client.go:Pull` and check that both the manifest body and each layer body are hashed and compared to their descriptors before return. Add a unit test with an in-process registry (`testregistry_test.go`) whose manifest response carries a mismatching `Docker-Content-Digest`, and whose blob response body does not match the descriptor digest. `Pull` must fail in both cases.

### H-operator-02: A changed `Module.spec.digest` pin is not re-checked while the Module is converged
- **Location**: `operator/internal/controller/module_controller.go:100-107` (converged early return) vs `:136-139` (digest pin check)
- **Category**: correctness (supply-chain control)
- **Suggested severity**: S3
- **Control**: `Module.spec.digest`. `module_types.go`: "Install fails with a DigestMismatch if the resolved bundle's digest differs".
- **Observation**: The converged check compares AppliedVersion, AppliedTemplate, Phase and the catalog digest, but not `mod.Spec.Digest`. A pin edited to a value different from `status.appliedDigest`, with the version unchanged, returns early. The Module stays `Ready` and does not report `DigestMismatch`; `observedGeneration` is not bumped.
- **Correct behaviour**: When `spec.digest` is set and differs from `status.appliedDigest`, the Module is not considered converged. It either re-pulls and fails with `DigestMismatch`, or reports the mismatch.
- **How a maintainer confirms it holds**: envtest: a Ready Module at digest A, patch `spec.digest` to B with the same version, and assert `Ready=False reason=DigestMismatch`, or that `status.appliedDigest == spec.digest`.

### H-operator-03: OCI module fetches are not routed through the netguard dial guard
- **Location**: `operator/internal/oci/client.go:32-39` (plain `http.Transport`); `operator/internal/verify/verify.go:100-110` (cosign registry client with default transport)
- **Category**: docs-drift (network-exposure control)
- **Suggested severity**: S3
- **Control**: The netguard SSRF dial guard. `docs/architecture.md:273-276`: "a shared dial-time SSRF guard (`netguard/`) refuses cloud-metadata and other unroutable-for-the-caller addresses — permissive for the operator's admin-configured ModuleSource fetches". `docs/security.md:308` scopes the guard to "The operator's `git`/`http` source fetchers", so the two documents disagree.
- **Observation**: git (`modsrc/git.go:38-40`, `:55`) and http (`modsrc/http.go:42`, `:92-114`) use `netguard.IsAllowed`. The OCI client and the cosign signature fetch do not.
- **Correct behaviour**: Either the OCI and cosign registry clients use `netguard.HTTPClient(..., netguard.IsAllowed)` (or an equivalent `DialContext`), or `docs/architecture.md` is narrowed to git/http to match `docs/security.md`.
- **How a maintainer confirms it holds**: Check that `oci.New` and `verify.baseCheckOpts` build their transports from netguard, and that a unit test with a link-local registry host is refused at dial time.

### H-operator-04: The default capture filter (FR-003) is never computed; captures started without a filter always fail
- **Location**: `operator/internal/controller/networkcapture_controller.go:373-381` (passes `nc.Spec.Filter` through); `operator/api/v1alpha1/networkcapture_types.go:40`; peers: `api/internal/handlers/capture.go:255-263`, `capture-sidecar/internal/httpserver/handlers.go:378-381`, `web/src/components/CaptureWidget.tsx:523`
- **Category**: correctness (data-capture restriction control)
- **Suggested severity**: S3 (fails closed; the dashboard's default "no filter" start fails)
- **Control**: The capture scope restriction. FR-003 (`specs/done_003-network-capture-sidecar/spec.md:124`): "The filter is OPTIONAL; when omitted, a default filter is applied that restricts the capture to the game server's own advertised ports." `contracts/capture-sidecar.md:250` puts the duty on the control plane: "only the control plane knows those ports, so it must materialise that default before calling the sidecar", and says the sidecar rejects an absent filter with 400.
- **Observation**: The API passes `nil` when the filter is empty, and so does the dashboard (`filter.trim() || undefined`). The operator forwards `nil`, so the sidecar returns 400 and the capture goes Failed ("failed to start capture on sidecar: … filter is required"). The sidecar's refusal is correct and the restriction holds (fails closed). What is missing is the operator-side default.
- **Correct behaviour**: When `spec.filter` is nil, the operator builds the default filter from the GameTemplate's advertised ports and protocols and sends that. The sidecar keeps refusing an empty filter.
- **How a maintainer confirms it holds**: envtest with a stub sidecar client that records the `filter` argument: a NetworkCapture without `spec.filter` on a template advertising `game/TCP 25565` gets a non-empty filter restricted to that port. An e2e capture start without a filter reaches Running.

### H-operator-05: The playit tunnel egress NetworkPolicy does not match its documented intent
- **Location**: `operator/internal/controller/gameserver_tunnel.go:453-463` (playit case adds no rule); `:482-487` (single egress rule with an explicit `Ports` list)
- **Category**: correctness (network-exposure control)
- **Suggested severity**: S3 (fails closed; playit tunnels cannot work under the chart's default `networkPolicies.enabled: true`)
- **Control**: The per-server tunnel egress NetworkPolicy (`<gs>-tunnel-egress`), with the chart's `default-deny-egress` (`charts/gameplane/templates/networkpolicies.yaml:24-37`, `podSelector: {}`).
- **Observation**: The playit comment says "we permit all ports". The generated rule lists only DNS (53/UDP and TCP) and the template's advertised container ports. Egress to the relay's control plane, and to the apiserver (which the playit Role at `tunnel_rbac.go:98-111` exists to allow), is not admitted. Separately, `tunnel/main.go:282-285` records playit address reporting as "TBD". The operator's `gameserver_tunnel.go:130` has a matching TODO, while `docs/tunnels.md:64-67` says "The address appears in `status.endpoints` once the tunnel pod reports it back".
- **Correct behaviour**: A maintainer decides the intended playit egress scope (the comment's "all ports", or a narrower documented set). The policy then matches that decision, the comment matches the policy, and the docs describe the actual state of address reporting.
- **How a maintainer confirms it holds**: Render `<gs>-tunnel-egress` for a playit GameServer in envtest and compare `spec.egress` with the documented intent. Then, on a cluster with the chart's default policies, check that a playit tunnel pod reaches its relay and that its assigned address reaches `status.tunnelEndpoints`.

### H-operator-06: `docs/tunnels.md` tells users to create credential Secrets that the operator will refuse
- **Location**: `operator/internal/controller/gameserver_tunnel.go:316-323`; `api/internal/handlers/resources.go:544-554`; `docs/tunnels.md:109-114`, `:150-155`, `:197-201`
- **Category**: docs-drift (secret-handling control)
- **Suggested severity**: S3
- **Control**: The server-owned Secret requirement for `spec.networking.tunnel.credentialsSecretRef`: the Secret must carry an ownerReference to the GameServer, enforced by both the API and the operator.
- **Observation**: `docs/tunnels.md` tells users to `kubectl -n gameplane-games create secret generic frp-creds …` (and likewise for tailscale and playit), which gives a Secret with no ownerReference. The operator then returns an error from `reconcileTunnel`. That aborts the whole GameServer reconcile before the Service, StatefulSet and status steps run, and the cause appears only in operator logs. The API's `PUT …/tunnel-credentials` (`tunnelcreds.go:96-111`) creates correctly owned Secrets. The control itself behaves as designed.
- **Correct behaviour**: The docs describe the supported way to supply credentials (the API endpoint, or a Secret with the required ownerReference). The refusal is also surfaced on the GameServer's status or conditions, not only in logs.
- **How a maintainer confirms it holds**: Follow `docs/tunnels.md` step by step on a test cluster. The GameServer must reach Running with a working tunnel, or the docs are corrected. envtest: a GameServer referencing an unowned Secret gets a status condition that names the refusal.

### H-operator-07: A volume-snapshot restore copies Secret references the new server is not allowed to use, and the Restore hangs
- **Location**: `operator/internal/controller/restore_volumesnapshot.go:67-89` (`orig.Spec.DeepCopyInto`); refusals at `gameserver_controller.go:1365-1367` (`validateServerEnvSources`) and `gameserver_tunnel.go:321-323`; wait loop at `restore_volumesnapshot.go:103-128`
- **Category**: correctness (secret-handling control)
- **Suggested severity**: S3
- **Control**: Secrets referenced from `spec.env` and `spec.networking.tunnel.credentialsSecretRef` must be owned by the referencing GameServer (same name and UID).
- **Observation**: The restored GameServer inherits the original's Secret references. Those Secrets are owned by the original server's UID, so the new server's reconcile is refused before status is written. `awaitRestoredServer` has no deadline and requeues every 10 s, so the Restore stays `Running` indefinitely. The control holds; the Restore flow does not account for it.
- **Correct behaviour**: The volume-snapshot restore gives the new server Secrets it owns (copied or re-created), or drops or rewrites the inherited references, or fails the Restore with a clear message. The ownership check must stay as it is.
- **How a maintainer confirms it holds**: envtest: an original server with an owned env Secret and a volume-snapshot Backup. The Restore reaches a terminal phase, and the new server does not reference a Secret owned by another server's UID.

### H-operator-08: `operator/specs.md` misdescribes the agent's RBAC and how quiesce calls are authenticated
- **Location**: `operator/specs.md:318`, `:460`; `docs/architecture.md:272`; code: `operator/internal/controller/agent_rbac.go:43-56`, `operator/internal/agent/client.go:58-94`
- **Category**: docs-drift (authorization/authentication control)
- **Suggested severity**: S4
- **Control**: The per-GameServer agent ServiceAccount and Role, and operator-to-agent authentication.
- **Observation**: `specs.md:460` says: "Each GameServer gets a unique ServiceAccount + Role (verb:exec on that Pod only). Agent token is bound to that SA and verified by the operator before accepting quiesce/unquiesce calls." The code's Role grants only `get`/`patch` on `gameservers/status` for that one GameServer (`resourceNames`). Quiesce and unquiesce are calls from the operator to the agent over mTLS (client certificate). The operator accepts no calls from the agent and verifies no agent token. `docs/architecture.md:272` says "Operator → K8s: cluster-wide CRUD on Gameplane CRDs + workload primitives". The chart grants workload writes per namespace (`charts/gameplane/templates/operator.yaml:123-183`).
- **Correct behaviour**: The docs state the actual grants and the actual authentication direction and mechanism.
- **How a maintainer confirms it holds**: Compare the specs.md "Security considerations" section with `agent_rbac.go`, `agent/client.go` (mTLS `tls.Config`) and the agent's `--tls-client-ca` handling. Compare `docs/architecture.md`'s boundary line with the chart's ClusterRole and Role split.

### H-operator-09: The `operator/config/rbac` dev manifests disagree with each other and with the stated RBAC design
- **Location**: `operator/config/rbac/role_binding.yaml:1-11` (roleRef `gameplane-operator-manager`), `:13-25` (roleRef `gameplane-operator-leader-election`); `operator/config/rbac/role.yaml` (generated as `manager-role`); `operator/config/rbac/role_namespace.yaml:1-3`; `operator/internal/controller/gameserver_controller.go:215-220`
- **Category**: correctness (authorization/RBAC control)
- **Suggested severity**: S4 (dev path only; the chart has its own RBAC)
- **Control**: The operator's least-privilege split: cluster-wide reads, and writes scoped to the games namespace.
- **Observation**:
  - `role_binding.yaml` binds ClusterRole `gameplane-operator-manager` and Role `gameplane-operator-leader-election`. Neither is defined under `operator/config/`. `make manifests` generates the ClusterRole as `manager-role` (`Makefile:290`).
  - `role_namespace.yaml:1-3` and the marker comment at `gameserver_controller.go:215-220` say the ClusterRole holds only reads and that writes are namespace-scoped. The generated `role.yaml` grants cluster-wide `create/update/patch` on `serviceaccounts`, `roles`, `rolebindings`, `jobs` (plus delete), `networkpolicies` (plus delete), `gameservers`, `gametemplates`, `backups`, `backupschedules`, `networkcaptures` and `pods/ephemeralcontainers`, because several controllers' kubebuilder markers declare write verbs.
  - `role_namespace.yaml` also lacks grants the controllers use: `apps/deployments` writes (sentinel and tunnel), `networkpolicies`, `restores`, `networkcaptures`, `*/status` and `pods/ephemeralcontainers`.
- **Correct behaviour**: The dev RBAC manifests reference roles that exist. The kubebuilder markers declare only cluster-wide reads (plus the few cluster-scoped writes), as the comments state. The namespaced Role matches the chart's Role.
- **How a maintainer confirms it holds**: `kubectl apply -f operator/config/rbac/` on a scratch cluster, then `kubectl auth can-i --list --as=system:serviceaccount:gameplane-system:gameplane-operator` in a non-games namespace. It must show no write verbs on workload or RBAC resources. Grep the markers for write verbs on namespaced kinds.

### H-operator-10: `OCISourceSpec.Insecure` is documented as skipping TLS verification, but the client only switches to plain HTTP
- **Location**: `operator/api/v1alpha1/modulesource_types.go:146-150`; `operator/internal/oci/client.go:26-31`, `:49`
- **Category**: docs-drift (TLS control)
- **Suggested severity**: S4
- **Control**: TLS verification of module registries.
- **Observation**: The CRD field doc says "Insecure allows plain HTTP and skips TLS verification". The client comment says "this client deliberately does not skip TLS verification", and the code only sets `PlainHTTP`. The code is stricter than the CRD doc. A user who sets `insecure: true` for a self-signed HTTPS registry gets plain-HTTP requests against a TLS endpoint, not a skipped check.
- **Correct behaviour**: The CRD doc (and the generated CRD description) matches the implemented behaviour, which is plain HTTP only with TLS verification kept.
- **How a maintainer confirms it holds**: Read `oci/client.go:repo()` and check that no `InsecureSkipVerify` is set anywhere in `operator/internal/oci` or `operator/internal/verify`. Check that the CRD description of `spec.oci.insecure` no longer says TLS verification is skipped.

## Moved from audit/evidence/review-operator/notes.md (OD-019, 2026-09-24)

### C-operator-10: Fixed-name Secrets and the `-auto` BackupSchedule are deleted without an ownership check, on every reconcile
- **Location**: `operator/internal/controller/gameserver_config.go:301-302`, `:324-325`; `operator/internal/controller/gameserver_rcon.go:87-88`; `operator/internal/controller/gameserver_controller.go:2210-2216`
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. GameServer `mc` uses a template with no password config fields and no configFiles, has no backupPolicy, and has RCON either disabled or backed by an external Secret.
  2. Every reconcile issues a live `DELETE` for `mc-config`, `mc-files`, `mc-rcon` and BackupSchedule `mc-auto`. Status-only updates, including every agent heartbeat, trigger a reconcile (`For(&GameServer{})` has no predicate).
  3. If an object with one of those names exists and is not owned by `mc` (for example, a user's own Secret `mc-config`), it is deleted.
- **Expected**: The same file already defines the pattern for this: `reconcileNetworkPolicy` (`gameserver_controller.go:1036-1051`) reads from the cache and deletes only when `metav1.IsControlledBy`. Its comment explains that an unconditional Delete "would fire a DELETE -> 404 for every server on every heartbeat". `reconcileTunnelNetworkPolicy`, `reconcileGameDirectServiceFromTemplate` and `reconcileTunnelRBAC` also follow it.
- **Actual**: Four unconditional, unowned deletes per reconcile.

### Q-operator-capture-immutability

- **Immutability claims**: `networkcapture_types.go:34` and `:41` say `serverRef` and `filter` are "Immutable once created", but no CRD has an `oldSelf` transition rule. Should these be enforced?
