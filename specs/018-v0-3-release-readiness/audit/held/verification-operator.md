# T045 operator chunk: independent verification (opus)

Held under OD-019, off-git. This file covers the ten candidates in `held/review-operator.md`. Its wording is defensive. For each control it names the control, where the control lives, the correct behaviour, and how a maintainer confirms that the control holds. It contains no misuse walkthroughs.

Method: I checked each candidate against master `13a859ff`. None of the cited files differ between this branch and master. I read the cited code together with its callers and callees: `operator/internal/oci/client.go`, `operator/internal/modsrc/{oci,git,http}.go`, `operator/internal/verify/verify.go`, `operator/internal/controller/{module,networkcapture,gameserver,gameserver_tunnel,tunnel_rbac,agent_rbac,restore_volumesnapshot}_*.go`, and `operator/config/rbac/*`. I also read the vendored `oras.land/oras-go/v2@v2.6.2` source (`registry/remote/repository.go`, `content/reader.go`), the chart's `templates/{operator,networkpolicies}.yaml`, `docs/{architecture,security,tunnels}.md`, `README.md`, spec `done_003` (FR-003 and `contracts/capture-sidecar.md`), `operator/specs.md` and `audit/findings.md`. None of the kept items is tracked in `findings.md`. F-018 covers the tunnel-credential ownership control itself, which holds. `go build ./...` in `operator/` passes. No cluster was reachable from this session, so every confirmation step below is a code reading or a test a maintainer adds. No test or lint suite was run.

Cross-references from other chunks: the netguard chunk rejected H-netguard-04 as a duplicate of H-operator-03. H-operator-03 is kept here, so that item needs no second look, and the `README.md:146` wording it raised is folded into H-operator-03's location. The edge chunk rejected evidence candidate C-capture-sidecar-01 as a duplicate of H-operator-04. That defect is therefore already described in git-bound files (`evidence/review-capture-sidecar/notes.md` and `verification.md`), and the maintainer should settle whether H-operator-04 still needs to be held.

Severity follows research R3. One change from the reviewer: H-operator-01 goes from S2 to S1. It defeats the control that `verify.go:1-4` and `docs/security.md` say exists to stop a compromised registry from supplying a GameTemplate, which makes it a security-boundary break.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| H-operator-01 | kept | S1 | Confirmed. `Pull` returns the descriptor digest from `FetchReference` by tag (`oci/client.go:98-131`). In oras-go v2.6.2 that digest is the registry's `Docker-Content-Digest` header when one is present, and the body is not hashed (`repository.go:1579-1660`, header taken at `:1646`). Layers are read with `io.ReadAll` (`client.go:134-140`). `blobStore.Fetch` checks only the header and Content-Length (`repository.go:775`, `:1684-1709`). The operator never runs the bytes it consumes through `content.ReadAll`/`NewVerifyReader` (`content/reader.go:112`, `:132`), and cosign checks the signature for `ref@<that digest>` only (`verify.go:85-97`). The bytes used to build the template are therefore not bound to the digest that was verified. No test covers a mismatch (`oci/testregistry_test.go` always sends the correct header). |
| H-operator-02 | kept | S3 | Confirmed. The convergence check (`module_controller.go:100-107`) never looks at `spec.digest`. A pin edited while the version stays the same returns early, before the pin check at `:136-139`. The Module stays `Ready`, and `observedGeneration` is not bumped. |
| H-operator-03 | kept | S3 | Confirmed. The OCI client uses a plain `http.Transport` (`oci/client.go:37`), and the cosign registry client uses go-containerregistry's default transport (`verify.go:100-110`). Only git and http use `netguard.IsAllowed` (`modsrc/git.go:39`, `:55`; `modsrc/http.go:42`). `docs/architecture.md:273-276` and `README.md:146` ("… and OCI module fetches") say OCI is covered. `docs/security.md:308` scopes the guard to git and http only. Destinations are admin-configured, so this is defense in depth (S3, the same as H-netguard-01). |
| H-operator-04 | kept | S3 | Confirmed. The API stores `filter: nil` when the field is empty (`api/internal/handlers/capture.go:255-258`), and the dashboard's filter field defaults to empty (`CaptureWidget.tsx:512`, `:523`). The operator forwards `nc.Spec.Filter` unchanged (`networkcapture_controller.go:373-381`, `sidecar_capture.go:96-100`). The sidecar's 400 is not transient (`sidecar_capture.go:52-61`), so the capture goes `Failed`. Nothing in `operator/` or `api/` builds the FR-003 default. The restriction fails closed, and typing a filter works around it. |
| H-operator-05 | kept | S3 | Confirmed. For playit, the `<gs>-tunnel-egress` rule contains only DNS and the template's advertised ports (`gameserver_tunnel.go:453-463`, `:482-487`). The comment says "we permit all ports". No chart policy selects `app.kubernetes.io/name: gameplane-tunnel` (`networkpolicies.yaml`), so under the default `networkPolicies.enabled: true`, relay and apiserver egress are refused (fails closed). Separately, address reporting is still TODO (`gameserver_tunnel.go:130`, `tunnel/main.go:282-285`), while `docs/tunnels.md:65-67` says the address appears on status. |
| H-operator-06 | kept | S3 | Confirmed. `docs/tunnels.md:112-113`, `:153-154` and `:200-201` create Secrets with `kubectl create secret generic`, which gives them no ownerReference. `reconcileTunnel` refuses them (`gameserver_tunnel.go:316-323`, check at `gameserver_controller.go:2035-2051`), and the error returns at `gameserver_controller.go:382-385`, before the Service, StatefulSet and status steps. The control behaves as designed. The docs and the missing status signal are the defect. (F-031 is a separate defect in the same examples.) |
| H-operator-07 | kept | S3 | Confirmed. `orig.Spec.DeepCopyInto` (`restore_volumesnapshot.go:74`) copies `spec.env` Secret and ConfigMap refs and `tunnel.credentialsSecretRef`. Those objects are owned by the original server's name and UID, so the new server is refused (`validateServerEnvSources` at `gameserver_controller.go:1996-2025`; `gameserver_tunnel.go:321-323`) before its status is written. `awaitRestoredServer` has no deadline (`restore_volumesnapshot.go:103-128`), so the Restore stays `Running`. The ownership check must stay as it is. |
| H-operator-08 | kept | S4 | Confirmed. `operator/specs.md:460` and `:318` describe an exec Role and an agent token that the operator verifies. `agent_rbac.go:43-56` grants only `get`/`patch` on `gameservers/status` for the one server's `resourceNames`. Quiesce runs operator to agent over mTLS (`agent/client.go:60-94`). `docs/architecture.md:272` says "cluster-wide CRUD … + workload primitives", but the chart's ClusterRole is read-only for workloads and Secrets, with writes in a namespaced Role (`charts/gameplane/templates/operator.yaml:10-103`, `:123+`). Documentation only. |
| H-operator-09 | kept | S4 | Confirmed, on the dev path only. `role_binding.yaml` binds `gameplane-operator-manager` and `gameplane-operator-leader-election`, and neither is defined under `operator/config/`. The generated ClusterRole is named `manager-role`. The generated `role.yaml` grants cluster-wide writes on `serviceaccounts`, `roles`, `rolebindings`, `jobs`, `networkpolicies`, `volumesnapshots`, `pods/ephemeralcontainers` and several Gameplane CRDs. That contradicts `role_namespace.yaml:1-3` and the marker comment at `gameserver_controller.go:215-220`. The same comment's "keeps a compromised operator token from reading Secrets cluster-wide" also does not match either ClusterRole, since both grant `get/list/watch` on `secrets` cluster-wide. The chart's own RBAC is what a real install uses. |
| H-operator-10 | kept | S4 | Confirmed. The CRD field doc (`modulesource_types.go:146-148`, rendered at `crds/gameplane.local_modulesources.yaml:162-164`) says `insecure` "skips TLS verification". The code only sets `PlainHTTP` (`oci/client.go:49`), and `grep -rn InsecureSkipVerify operator` finds nothing outside tests. The code is stricter than the doc, and the git/http field's description (`:116-118`) already states the correct behaviour. Documentation only. |

No candidates rejected. Kept: 10.

### H-operator-01

**Location:** `operator/internal/oci/client.go:94-131` (`Pull`), `:134-140` (`readBlob`); `operator/internal/modsrc/oci.go:106-113`; `operator/internal/controller/module_controller.go:118-132`; `operator/internal/verify/verify.go:85-97`; library behaviour at `oras.land/oras-go/v2@v2.6.2/registry/remote/repository.go:1579-1660`, `:775`, `:1684-1709`.

**Control:** `ModuleSource.spec.verify` (cosign). Per `verify.go:1-4` and `docs/security.md` ("Signature verification"), a compromised registry must not be able to supply a GameTemplate.

**Repro / observation (defensive: confirms whether the control holds)**
1. Read `oci/client.go:Pull`. The returned digest is `manifestDesc.Digest` from `FetchReference(ctx, <tag>)`, and the manifest bytes come from `io.ReadAll` on the response body. Nothing computes `digest.FromBytes(manifestBytes)` or compares it.
2. Read `readBlob`: `Blobs().Fetch` followed by `io.ReadAll`, with no digest verification of the body.
3. Read the vendored oras-go: for a tag reference, `generateDescriptor` takes the digest from the response header when one is present (`:1646`) and hashes the body only when the header is absent. `blobStore.Fetch` compares only header values (`verifyContentDigest`). oras-go leaves body verification to the caller (`content.ReadAll` or `content.NewVerifyReader`).
4. Read `module_controller.go:130`: `verifier.Verify(ctx, entry.Reference, bundle.Digest)` checks the signature for that digest. The template is then materialised from the unverified bytes (`:151`).

**Expected:** Every byte the operator consumes is bound to the digest that cosign verified. For example: resolve the tag, fetch the manifest by digest, and check `digest.FromBytes(manifestBytes)` against it; read each layer through `content.ReadAll(rc, layer)` (or `NewVerifyReader`) so a size or digest mismatch fails `Pull`.

**Actual:** The manifest body and the layer bodies are not hashed. The signature therefore guarantees nothing about the content that is applied, which is the exact case the control exists for. To confirm a fix: add an `oci/testregistry_test.go` case in which the served manifest body, and separately a served layer body, do not match their descriptors, and assert that `Pull` returns an error in both cases.

### H-operator-02

**Location:** `operator/internal/controller/module_controller.go:100-107` (converged early return) vs `:134-139` (pin check); control documented at `operator/api/v1alpha1/module_types.go:29-34`.

**Control:** `Module.spec.digest`: "Install fails with a DigestMismatch if the resolved bundle's digest differs".

**Repro / observation (defensive)**
1. Read `:100-102`. The condition compares `AppliedVersion`, `AppliedTemplate`, `Phase` and the catalog digest. `mod.Spec.Digest` does not appear in it.
2. So on a `Ready` Module, a `spec.digest` edit that leaves `spec.version` unchanged returns at `:106`. The pin check at `:136` is never reached, and `status.observedGeneration` keeps the old generation.

**Expected:** When `spec.digest` is set and differs from `status.appliedDigest`, the Module is not treated as converged. It re-resolves and either applies the pinned content or reports `DigestMismatch`.

**Actual:** The Module stays `Ready` on content that does not match the pin. To confirm a fix, write an envtest: a `Ready` Module at digest A, patch `spec.digest` to B with the same version, and assert `Ready=False reason=DigestMismatch`, or `status.appliedDigest == spec.digest`, with `observedGeneration` advanced.

### H-operator-03

**Location:** `operator/internal/oci/client.go:32-39` (plain `http.Transport` at `:37`); `operator/internal/verify/verify.go:100-110` (cosign registry client with no custom transport); doc claims at `docs/architecture.md:273-276` and `README.md:146`; narrower claim at `docs/security.md:308-315`.

**Control:** the netguard dial-time SSRF guard (`netguard.IsAllowed`) for the operator's ModuleSource fetches.

**Repro / observation (defensive)**
1. `grep -n netguard operator/internal/modsrc/*.go operator/internal/oci/*.go operator/internal/verify/*.go` finds hits only in `modsrc/git.go` and `modsrc/http.go`.
2. `oci.New` builds `retry.NewTransport(&http.Transport{})` with no guarded `DialContext`. `baseCheckOpts` passes only auth and context to go-containerregistry, so its default transport is used.
3. `docs/architecture.md` and `README.md` include OCI in the guard's scope, while `docs/security.md` lists git and http only.

**Expected:** Either the OCI client and the cosign registry client dial through netguard (for example `netguard.HTTPClient(..., netguard.IsAllowed)`, or its `DialContext` inside the retry transport and in `ggcrremote.WithTransport`), or `docs/architecture.md` and `README.md:146` are narrowed to match `docs/security.md`.

**Actual:** The OCI and cosign fetch paths have no dial-time guard, and two documents say they do. To confirm a fix: a unit test that points `oci.Client` (and the verifier's transport) at a link-local literal host and expects a dial-time refusal. Or, if the docs are narrowed instead, re-read all three documents and check that they agree.

### H-operator-04

**Location:** `operator/internal/controller/networkcapture_controller.go:373-381`; `operator/api/v1alpha1/networkcapture_types.go:38-41`; peers `api/internal/handlers/capture.go:255-258`, `capture-sidecar/internal/httpserver/handlers.go:378-381`, `web/src/components/CaptureWidget.tsx:512`, `:523`; requirement `specs/done_003-network-capture-sidecar/spec.md:124` (FR-003) and `contracts/capture-sidecar.md:250`.

**Control:** the capture scope restriction: a capture records only the game server's advertised ports unless the requester supplies a narrower or custom filter.

**Repro / observation (defensive)**
1. The dashboard's filter field starts empty and sends `undefined` when it is left blank. The API then stores `spec.filter: nil`.
2. The operator passes `nc.Spec.Filter` (nil) to `StartCapture`, so the sidecar receives an empty filter and answers 400. `IsTransientError` treats a 4xx as permanent, and the capture goes `Failed` with "failed to start capture on sidecar".
3. `grep -rn` across `operator/` and `api/` finds no code that builds a filter from `tmpl.Spec.Ports`.
4. The sidecar's refusal is correct and must stay. The restriction fails closed.

**Expected:** When `spec.filter` is nil, the operator (the control plane that knows the ports) builds the default filter from the GameTemplate's advertised ports and protocols and sends that. The sidecar keeps refusing an empty filter.

**Actual:** A capture started without a filter always fails. To confirm a fix: an envtest with a recording stub `SidecarCaptureClient`, where a NetworkCapture with no `spec.filter` on a template advertising `game` 25565/TCP produces a non-empty filter restricted to that port and protocol. Also a live check that a dashboard start with the field left blank reaches `Running`, and that a direct empty-filter request to the sidecar still returns 400.

### H-operator-05

**Location:** `operator/internal/controller/gameserver_tunnel.go:453-463` (playit case adds no rule), `:482-487` (single egress rule with an explicit `Ports` list); `operator/internal/controller/tunnel_rbac.go:98-111` (playit Role); `charts/gameplane/templates/networkpolicies.yaml:16-37`; address-reporting TODOs at `gameserver_tunnel.go:130` and `tunnel/main.go:282-285`; claim at `docs/tunnels.md:65-67`.

**Control:** the per-server tunnel egress NetworkPolicy `<gs>-tunnel-egress`, layered on the chart's `default-deny-egress`.

**Repro / observation (defensive)**
1. Read the `playit` case. It adds no ports, although its comment says "we permit all ports". The resulting rule lists DNS (53/UDP and 53/TCP) plus the template's advertised container ports.
2. `grep -n "gameplane-tunnel" charts/gameplane/templates/networkpolicies.yaml` finds nothing, so no chart policy adds egress for tunnel pods. The playit Role grants a status patch, but the pod has no egress path to the apiserver.
3. Read the TODOs: nothing writes a playit address to status yet.

**Expected:** The maintainer decides the intended playit egress scope, either the comment's "all ports" or a narrower documented set, plus apiserver access if the pod is meant to report status. The policy, the comment and `docs/tunnels.md` then agree, and the docs describe the actual state of address reporting.

**Actual:** Under the chart defaults, a playit tunnel pod cannot reach its relay (fails closed). The docs also promise an address on status that nothing writes. To confirm a fix: render `<gs>-tunnel-egress` for a playit GameServer in envtest and compare `spec.egress` with the documented decision. Then, on a cluster with the chart's default policies, check that the tunnel pod connects and that its address reaches `status.tunnelEndpoints`.

### H-operator-06

**Location:** `operator/internal/controller/gameserver_tunnel.go:316-323`; `operator/internal/controller/gameserver_controller.go:382-385`, `:2035-2051`; `api/internal/handlers/tunnelcreds.go` (`PUT …/tunnel-credentials` creates owned Secrets); `docs/tunnels.md:109-114`, `:150-155`, `:197-201`.

**Control:** tunnel credential Secrets must carry an ownerReference to the GameServer that references them. The API and the operator both enforce this (see F-018).

**Repro / observation (defensive)**
1. The three setup sections in `docs/tunnels.md` create the Secret with `kubectl create secret generic`, which sets no ownerReference.
2. `reconcileTunnel` returns an error for such a Secret, and `Reconcile` returns at `:382-385`, before the Service, StatefulSet and status steps. The only signal is the operator log line `reconcile tunnel`.

**Expected:** The docs describe the supported way to supply credentials: the dashboard or API endpoint, or a Secret created with an ownerReference to the GameServer. The refusal also appears on the GameServer's status or conditions. The ownership check itself stays as it is.

**Actual:** Following the docs leaves a GameServer that never gets its Service or StatefulSet, and nothing on its status says why. To confirm a fix: follow `docs/tunnels.md` on a test cluster (after F-031's example fix) until the server reaches `Running`. Also an envtest in which a GameServer referencing an unowned Secret gets a condition that names the refusal.

### H-operator-07

**Location:** `operator/internal/controller/restore_volumesnapshot.go:66-89` (`orig.Spec.DeepCopyInto` at `:74`), `:103-128` (`awaitRestoredServer`, no deadline); refusals at `operator/internal/controller/gameserver_controller.go:1365-1367` (`validateServerEnvSources`, `:1996-2025`) and `gameserver_tunnel.go:321-323`.

**Control:** Secrets and ConfigMaps referenced from `spec.env`, and `spec.networking.tunnel.credentialsSecretRef`, must be controller-owned by the referencing GameServer (name and UID).

**Repro / observation (defensive)**
1. A volume-snapshot Restore creates the new server by copying the original's whole spec, including those references.
2. The referenced objects are owned by the original server, so the new server's reconcile is refused before its status is written, and its phase stays empty.
3. `awaitRestoredServer` treats an empty phase as "still starting" and requeues every 10 s with no deadline. The Restore stays `Running`.

**Expected:** The volume-snapshot restore gives the new server objects it owns (copied or re-created with the new owner), or drops or rewrites the inherited references, or fails the Restore with a clear message. The ownership check stays unchanged.

**Actual:** A volume-snapshot restore of any server that uses env Secret or ConfigMap refs, or a tunnel credential Secret, never finishes. To confirm a fix: an envtest with an original server that has an owned env Secret and a volume-snapshot Backup. The Restore must reach a terminal phase, and the new server must not reference any object owned by another server's UID.

### H-operator-08

**Location:** `operator/specs.md:318`, `:460`; `docs/architecture.md:272`; code `operator/internal/controller/agent_rbac.go:43-56`, `operator/internal/agent/client.go:60-94`; chart `charts/gameplane/templates/operator.yaml:10-103` (ClusterRole), `:123+` (namespaced Role).

**Control:** the per-GameServer agent ServiceAccount and Role, operator-to-agent authentication, and the operator's own RBAC boundary.

**Repro / observation (defensive)**
1. `agent_rbac.go` creates a Role `<sa>-heartbeat` with a single rule: `get` and `patch` on `gameservers/status`, restricted to `resourceNames: [<gs>]`. It grants no `exec`.
2. `agent/client.go` builds an mTLS client (client certificate plus CA pool). The operator calls the agent. The operator receives no calls from the agent and verifies no agent token.
3. The chart's ClusterRole is read-only for workloads and Secrets. Workload writes sit in the namespaced Role.

**Expected:** `operator/specs.md` and `docs/architecture.md` state the real grants and the real direction and mechanism of authentication.

**Actual:** Both documents describe a different model. To confirm a fix, re-read the two documents against the three code sites above.

### H-operator-09

**Location:** `operator/config/rbac/role_binding.yaml:1-25`; `operator/config/rbac/role.yaml` (generated as `manager-role` by `Makefile:290-293`); `operator/config/rbac/role_namespace.yaml:1-20`; `operator/internal/controller/gameserver_controller.go:215-220` (marker comment).

**Control:** the operator's least-privilege split: cluster-wide reads, and writes limited to the games namespace.

**Repro / observation (defensive)**
1. `grep -rn "gameplane-operator-manager\|gameplane-operator-leader-election" operator/config` finds only the binding file. Neither role is defined there.
2. List `role.yaml`'s rules. Write verbs appear cluster-wide on `serviceaccounts`, `roles`, `rolebindings`, `jobs`, `networkpolicies`, `volumesnapshots`, `pods/ephemeralcontainers`, `gameservers`, `gametemplates`, `backups`, `backupschedules`, `networkcaptures` and `clusters`/`modules`/`restores`. They come from write verbs in several controllers' kubebuilder markers.
3. The marker comment says the ClusterRole "keeps a compromised operator token from reading Secrets cluster-wide". Both `role.yaml` and the chart's ClusterRole grant `get/list/watch` on `secrets` cluster-wide, so the comment overstates the protection.
4. `role_namespace.yaml` lacks grants the controllers use (for example `apps/deployments` writes, `networkpolicies`, `restores`, `*/status`).

**Expected:** The dev RBAC manifests reference roles that exist. The kubebuilder markers declare only the cluster-wide reads plus the few cluster-scoped writes, and the namespaced Role matches the chart's. The marker comment states what the ClusterRole actually allows.

**Actual:** The dev manifests bind nothing usable, and the generated ClusterRole is broader than the design it documents. A chart install uses its own RBAC and is unaffected. To confirm a fix, run `kubectl apply -f operator/config/rbac/` on a scratch cluster, then `kubectl auth can-i --list --as=system:serviceaccount:gameplane-system:gameplane-operator -n default`. It must show no write verbs on workload or RBAC resources.

### H-operator-10

**Location:** `operator/api/v1alpha1/modulesource_types.go:146-148` (rendered at `charts/gameplane/crds/gameplane.local_modulesources.yaml:162-164`); `operator/internal/oci/client.go:26-31`, `:49`.

**Control:** TLS verification of module registries.

**Repro / observation (defensive)**
1. `oci/client.go` sets only `r.PlainHTTP = c.insecure`, and its doc comment says the client "deliberately does not skip TLS verification".
2. `grep -rn InsecureSkipVerify operator | grep -v _test` returns nothing.
3. The CRD description of `spec.oci.insecure` says it "skips TLS verification". The description of the neighbouring git/http field (`:116-118`) already states the correct behaviour.

**Expected:** The CRD doc, and the regenerated CRD description, say "plain HTTP only; TLS verification is never skipped".

**Actual:** The documentation promises a weaker behaviour than the code implements. To confirm a fix, run `make generate manifests`, then check that the rendered description no longer mentions skipping TLS verification and that step 2 still returns nothing.

## Moved from audit/evidence/review-operator/verification.md (OD-019, 2026-09-24)

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-operator-10 | kept | S4 | Confirmed. Four fixed-name `r.Delete` calls run on every reconcile, with no cache read and no ownership check (`gameserver_config.go:300-302`, `:323-325`, `gameserver_rcon.go:85-88`, `gameserver_controller.go:2210-2216`). The same file already avoids exactly this pattern (`:1036-1051`). Nothing in Gameplane creates colliding names, so deleting a user's object needs a user-made object with one of those names. |

### C-operator-10

**Location:** `operator/internal/controller/gameserver_config.go:300-302`, `:323-325`; `operator/internal/controller/gameserver_rcon.go:85-88`; `operator/internal/controller/gameserver_controller.go:2210-2216`.

**Repro / observation**
1. Reading: when the feature is off, each function calls `r.Delete` on a fixed name (`<gs>-config`, `<gs>-files`, `<gs>-rcon`, BackupSchedule `<gs>-auto`) with no cache read and no `IsControlledBy` check. `client.Delete` always goes to the apiserver.
2. Take a server with no password config fields, no `configFiles`, no `backupPolicy` and no operator-managed RCON. With apiserver audit logging on, every agent heartbeat (a status patch that re-triggers `For(&GameServer{})`) produces four `DELETE` calls that return 404.
3. If an object with one of those names exists and is not controlled by the GameServer, it is deleted.

**Expected:** The pattern already used in `reconcileNetworkPolicy` (`gameserver_controller.go:1036-1051`) and in the tunnel NetworkPolicy, RBAC and direct-Service paths: read from the cache, and delete only when `metav1.IsControlledBy`.

**Actual:** Four live, unowned deletes per reconcile. No Gameplane component creates objects with those names, so the unowned-delete half needs a user-made object with a colliding name.
