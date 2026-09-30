# Review: operator

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `operator/specs.md`, `docs/architecture.md` (operator sections: "Data flow examples", "Module system", "Multi-cluster", "Security boundaries"), `docs/tunnels.md` (frp/playit sections), `specs/done_003-network-capture-sidecar/` (spec.md FR-003/FR-007, contracts/capture-sidecar.md), `specs/002-nuclear-option-ip-pool/` (contracts/address-pool-api.md, data-model.md), CRD Go doc comments in `operator/api/v1alpha1/`

held candidates: 10 (see OD-019)

## Scope reviewed

Read in full:
- `operator/cmd/main.go`
- `operator/internal/agent/client.go`, `sidecar_capture.go`
- `operator/internal/controller/`: every non-test file (agent_certs, agent_rbac, backup_controller, backup_volumesnapshot, backupschedule_controller, cluster_controller, gameserver_config, gameserver_controller, gameserver_extravolumes, gameserver_idle, gameserver_modcreds, gameserver_node, gameserver_rcon, gameserver_restart, gameserver_sentinel, gameserver_status, gameserver_stop_attach, gameserver_tunnel, gameserver_version, gameserver_wipe, gametemplate_controller, helpers, metrics, module_controller, modulesource_controller, networkcapture_controller, restic_summary, restore_controller, restore_volumesnapshot, retention, semver, tunnel_rbac)
- `operator/internal/modsrc/` (bundle, dir, fetcher, git, http, local, oci, upload), `operator/internal/oci/` (auth, base64, bundle, client), `operator/internal/verify/verify.go`
- `operator/api/v1alpha1/`: gameserver_types, backup_types, backupschedule_types, restore_types, networkcapture_types, cluster_types, groupversion_info
- `operator/config/`: crd/kustomization.yaml, manager/*, rbac/* (role.yaml, role_binding.yaml, role_namespace.yaml, service_account.yaml), samples/* (all 7 samples + kustomization)
- `operator/specs.md`, `operator/go.mod`, `operator/.testcoverage.yml`

Read in part: `gametemplate_types.go` (lines 500-580, 820-1080, 1160-1180 plus a grep of every "operator"/"reconcile" doc claim), `module_types.go` (spec/status/constants), `modulesource_types.go` (lines 15-160).

Cross-referenced (partial reads, only to confirm an operator-side behaviour): `capture-sidecar/internal/httpserver/handlers.go` (routes, HandleStart, HandleDelete), `tunnel/main.go` (frp config render), `api/internal/handlers/capture.go`, `api/internal/handlers/resources.go` (update validation), `api/internal/handlers/tunnelcreds.go`, `api/internal/kube/capture.go`, `web/src/components/CaptureWidget.tsx`, `web/src/routes/tabs/settings/Backups.tsx`, `charts/gameplane/templates/operator.yaml`, `charts/gameplane/templates/networkpolicies.yaml`, `agent/internal/quiesce/quiesce.go` (grep only), oras-go v2.6.2 `registry/remote/repository.go` (module cache).

Not read: `zz_generated.deepcopy.go` and the 9 CRD YAMLs line by line (checked by regeneration and diff instead, see Method), `operator/Dockerfile`, `operator/test/crds/snapshot/*`, all `*_test.go` files (except `module_upgrade_envtest_test.go:40-50` and `module_envtest_test.go:55-100` to confirm a test fixture).

## Method

1. `go build ./...` in `operator/` (passes).
2. Copied `operator/` and `netguard/` to the scratchpad and ran `controller-gen object` and `controller-gen crd rbac` there with the Makefile's flags (`make generate` / `make manifests` equivalents, with no writes to the working tree). Diffed the output against the committed files.
3. `cmp` of `operator/config/crd/*.yaml` against `charts/gameplane/crds/` and `charts/gameplane/crd-manifests/`.
4. Compared the flags the chart passes (`charts/gameplane/templates/operator.yaml`) with the flags `cmd/main.go` defines.
5. Read each reconciler top to bottom, then checked each `operator/specs.md` and `docs/architecture.md` statement about it against the code.
6. Followed operator-to-component contracts (capture sidecar HTTP routes, tunnel env vars, dashboard fields) far enough to confirm what the operator sends and what the peer expects.

No test or lint suite was run.

## Observations (no finding)

- `go build ./...` succeeds on the current tree. That contradicts `operator/specs.md`'s "production build does not compile" note (see C-operator-18).
- Regenerated `zz_generated.deepcopy.go`, all 9 CRD YAMLs and `config/rbac/role.yaml` are byte-identical to the committed files. `charts/gameplane/crds/` and `charts/gameplane/crd-manifests/` are byte-identical to `operator/config/crd/` (9 of 9). Codegen is in sync.
- Every `--flag` in the chart's operator args is defined in `cmd/main.go`. `--zap-log-level` comes from `zap.Options.BindFlags`.
- The capture retention flags are range-checked and wired into `NetworkCaptureReconciler` (`main.go:268-304`, `:439-440`). The capture sidecar has a `DELETE /captures/{id}` route (`capture-sidecar/internal/httpserver/handlers.go:197`), and `CaptureClient.DeleteCaptureFile` calls it (`sidecar_capture.go:231-268`).
- `reconcileNetworkPolicy` never emits an ingress rule with an empty `Ports` list. It deletes its own policy instead (`gameserver_controller.go:1034-1052`), as its doc comment promises.
- `findAddressConflict`/`addressConflictLess` (`gameserver_controller.go:2427-2571`) implements the ordering `operator/specs.md:186` describes, including the "at least one, not exactly one" caveat.
- The Module finalizer blocks uninstall while any GameServer references the template (`module_controller.go:258-313`), matching `docs/architecture.md:207-208`.
- When indexing fails, the ModuleSource keeps its last-good catalog and leaves `LastSync` unchanged (`modulesource_controller.go:71-107`), as the code comments describe.
- The `ErrUnsupported` sentinel is returned unwrapped by `agent.callQuiesce` (`client.go:153-155`), so `errors.Is` in `backup_controller.go:282` works. Other operator→agent errors that callers inspect use `%w`.
- `derivePhase` (`gameserver_status.go:268-294`) never returns `Stopped`, although the CRD enum, `metrics.go:23`, `gameserver_status.go:251` and `restore_controller.go:127` all handle it. This is harmless reserved-value handling (see Questions).

## Candidate findings

### C-operator-01: A failed post-backup unquiesce is never retried, which leaves the game with auto-save off
- **Location**: `operator/internal/controller/backup_controller.go:186-190` (terminal-phase guard), `:411-439` (status persisted before `runUnquiesce`); `operator/internal/controller/backup_volumesnapshot.go:101-133`, `:138-145`
- **Category**: correctness
- **Suggested severity**: S2 (the game silently stops auto-saving; the only signal is a Backup condition)
- **Observation / repro**:
  1. Create a Backup with `spec.quiesce: true` (the CRD default). `maybeQuiesce` succeeds and stamps `backup.gameplane.local/quiesce-attempted=true`. For Minecraft this means `save-off` has run.
  2. The restic Job succeeds. `mirrorJobStatus` writes `phase: Succeeded` (with `snapshotID`) through `r.Status().Update` at line 412, and only then calls `runUnquiesce` at line 428.
  3. The agent's `/unquiesce` call fails once (agent restarting, RCON reconnecting, network blip). `runUnquiesce` records `Unquiesced=False` and returns `RequeueAfter: 30s` (lines 481-487).
  4. On the next reconcile, lines 187-190 return early, because a `Succeeded` Backup with a snapshot id "needs no further reconciliation". `maybeUnquiesce` never runs again.
  5. The same ordering applies to the `Failed` path (line 427), to `completeVolumeSnapshot` (phase persisted at `:129`, unquiesce at `:133`) and to `failVolumeSnapshot` (`:141-144`).
- **Expected**: `docs/architecture.md:154-158`: "Before going terminal it always releases the game world: a Backup with `spec.quiesce` left the game with auto-save off waiting for the post-backup unquiesce, and a `Failed` Backup is never reconciled again — so the unquiesce runs first, and while the agent is unreachable the operator requeues rather than failing." The `runUnquiesce` doc comment (`:474-479`) also says the unquiesce is "retried — the unquiesced-at annotation makes the retry idempotent and stops the loop once it lands". Only `failUnrestorable` (`:140-148`) follows this rule; its own comment explains why.
- **Actual**: On the normal success and failure paths, the terminal phase is persisted first. One failed unquiesce is then never retried, and the game stays with auto-save disabled until someone runs `save-on` or restarts the server.

### C-operator-02: The BackupSchedule the operator creates from `spec.backupPolicy` always has `quiesce: false`
- **Location**: `operator/internal/controller/gameserver_controller.go:2218-2225`; `operator/api/v1alpha1/backupschedule_types.go:37-48`
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. Set a backup policy from the dashboard (Server, Settings, Backups). This writes `spec.backupPolicy` (`web/src/routes/tabs/settings/Backups.tsx:20-29`), or you can set it with kubectl.
  2. `reconcileBackupSchedule` creates `<gs>-auto` with `CreateOrUpdate`. The mutate function sets ServerRef, Schedule, RepoRef, Retention and Suspend, and never sets `Quiesce`.
  3. `BackupScheduleSpec.Quiesce` is `json:"quiesce"` without `omitempty`, so the typed client sends an explicit `quiesce: false`. The apiserver does not apply the `+kubebuilder:default=true`.
  4. `fire()` copies `Quiesce` into every Backup (`backupschedule_controller.go:184`). Every scheduled backup of a dashboard-configured server therefore runs without `save-off`/`save-all`.
- **Expected**: The type's own comment (`backupschedule_types.go:39-45`) warns about exactly this: dropping an explicit `false` would "silently re-enable" quiesce. The mirror case, silently disabling it, is not handled. `InlineBackupPolicy` has no `quiesce` field, so the user cannot opt back in. The CRD default for both Backup and BackupSchedule is `true`.
- **Actual**: Auto-managed schedules always produce `quiesce: false` backups, which are only crash-consistent for games that need a flush.

### C-operator-03: A Module pinned to an older version from an OCI source never converges and re-pulls in a loop
- **Location**: `operator/internal/controller/module_controller.go:100-107`; `operator/internal/modsrc/oci.go:93-97`
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. An OCI ModuleSource indexes a module with tags `1.1.0` and `1.0.0`. `indexModule` pulls only the latest tag and stores its digest (D2) as `entry.Digest` (`oci.go:97`).
  2. Install or roll back a Module with `spec.version: 1.0.0`. The documented rollback is "re-pinning spec.version" (`module_types.go`, PreviousVersion doc). It applies successfully with `AppliedDigest = D1`.
  3. Next reconcile: the convergence check requires `entry.Digest == "" || mod.Status.AppliedDigest == entry.Digest`. D1 never equals D2, so the check fails.
  4. `markPullingTransition` flips the phase Ready→Pulling (status write, which triggers a watch event). The reconciler pulls 1.0.0 again, cosign-verifies it, re-applies it, and flips back to Ready (another status write and event). The loop repeats.
  5. The envtest fixture avoids this on purpose: `module_upgrade_envtest_test.go:47-48` says "No digest: convergence keys on version alone, so a pinned-older install doesn't re-pull against the latest version's digest". The production OCI fetcher does set the digest, though.
- **Expected**: A pinned install converges once its bundle is applied. The catalog digest describes `LatestVersion` only, so it should only be compared when `desiredVersion == entry.LatestVersion`.
- **Actual**: Continuous registry pulls and cosign verifications, and a Module that flaps between Pulling and Ready. With registry rate limits (GHCR, Docker Hub) this eventually turns into `PullFailed`.

### C-operator-04: Changing `spec.templateRef` on an existing GameServer blocks reconciliation for good
- **Location**: `operator/internal/controller/gameserver_controller.go:1373-1380`; `operator/api/v1alpha1/gameserver_types.go:37` (no immutability rule; the CRD has no `oldSelf` transition rule at all)
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. Create GameServer `mc` with `templateRef.name: minecraft-java`. Its StatefulSet selector includes `gameplane.local/template: minecraft-java`.
  2. `kubectl patch gs mc --type merge -p '{"spec":{"templateRef":{"name":"other"}}}'`. The CRD accepts it, and so does the API update handler (`api/internal/handlers/resources.go`, which has no templateRef check).
  3. `reconcileStatefulSet` sets `ss.Spec.Selector` to a label set containing the new template name. The apiserver rejects the update because a StatefulSet `spec.selector` is immutable.
  4. Every reconcile returns that error before `reconcileStatus` runs, so status is never refreshed and no condition explains the failure.
- **Expected**: Either `templateRef` is immutable at admission (a CEL `self == oldSelf` rule), or the selector does not include a mutable field.
- **Actual**: The server is wedged. Only operator logs show why.

### C-operator-05: Deleting an in-flight quiesced Backup (by hand, or through concurrencyPolicy Replace) never unquiesces
- **Location**: `operator/internal/controller/backupschedule_controller.go:266-271`; `operator/internal/controller/backup_controller.go:173-177` (no finalizer; a deleted Backup is `IgnoreNotFound`)
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. A Backup with `quiesce: true` is Running: the quiesce has been applied and the Job is active.
  2. It is deleted, either by a user (dashboard or kubectl) or by the BackupSchedule controller itself when `concurrencyPolicy: Replace` fires (line 268).
  3. The Backup has no finalizer (only Module uses one; `grep AddFinalizer` shows only `module_controller.go:68`). Reconcile then returns `IgnoreNotFound`, and `maybeUnquiesce` never runs for that Backup. The agent's quiesce has no auto-release timeout (`agent/internal/quiesce/quiesce.go`).
  4. With Replace, the successor Backup's validation runs before its own quiesce (`backup_controller.go:195-243`). If it fails on a missing repo Secret or key, nothing unquiesces the world.
- **Expected**: Deleting an operator-quiesced Backup releases the game world, the same guarantee `docs/architecture.md:154-158` gives for terminal phases.
- **Actual**: Auto-save can stay disabled indefinitely after such a delete.

### C-operator-06: A restic restore leaves files that were created after the snapshot, so the data volume is not "overwritten in place"
- **Location**: `operator/internal/controller/restore_controller.go:249`
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. Take a restic Backup of server `mc` at T0.
  2. Players create new files after T0 (for example new region files, or a newer timestamped save for games that load the newest save).
  3. Restore that Backup into `mc`. The Job runs `restic restore <id> --target /` with no `--delete`. restic restores the snapshot's files and never removes files that are missing from the snapshot. restic 0.17 added `restore --delete`; this needs checking against the pinned `restic/restic:0.17.1`.
- **Expected**: `restore_types.go:30-31`: "it is suspended and its data volume is overwritten in place".
- **Actual**: The restored volume is the union of the snapshot and every post-snapshot file. Games that pick the newest save file can come up on post-backup state.

### C-operator-07: A deleted or edited module-managed GameTemplate is not restored while its Module looks converged
- **Location**: `operator/internal/controller/module_controller.go:100-107` (early return), `:459` (`Owns(&GameTemplate{})`)
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. Module `minecraft-java` is Ready. Its status has AppliedVersion, AppliedTemplate, AppliedDigest and Phase Ready.
  2. `kubectl delete gametemplate minecraft-java`, or `kubectl edit` its spec. The API blocks this for managed templates, kubectl does not.
  3. The `Owns` watch enqueues the Module. Every field in the convergence check still matches, so the reconciler returns without looking at the template.
  4. GameServers referencing it go Failed with `GameTemplate "minecraft-java" not found` (`gameserver_controller.go:266-270`), or keep running an edited spec, until the Module's version or digest changes.
- **Expected**: `operator/specs.md:29`: "Changes that bypass the CRD ... are not supported — the operator's next reconcile will overwrite them". `docs/architecture.md:204-206`: the Module "materializes an owned GameTemplate".
- **Actual**: The owned child is never recreated or reverted while the status fields match.

### C-operator-08: An expired NetworkCapture is never deleted when the capture sidecar is unreachable (for example, the server is stopped)
- **Location**: `operator/internal/controller/networkcapture_controller.go:693-711` (and its doc comment at `:673-679`)
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. A capture completes on GameServer `mc`. The user stops `mc` (`spec.suspend: true`), which removes the pod and its `captures` emptyDir.
  2. When the TTL elapses, `expireCapture` marks the capture Expired and calls `DeleteCaptureFile`. The `<gs>-agent` Service has no endpoints, so the call fails.
  3. The code records `FileCleanupFailed`, returns `RequeueAfter: 30s` and never reaches `r.Delete`. Each retry repeats this, with up to the client's 30 s timeout on the single-worker controller if the CNI drops rather than rejects the connection.
- **Expected**: `operator/specs.md:293`: "File deletion is best-effort: an error is recorded as a `FileCleanupFailed` condition but never blocks the CR delete, since a pod that's already gone by the time retention fires has no file left to remove anyway."
- **Actual**: The CR is kept until the sidecar answers, which may never happen for a server that stays stopped. The function's own doc comment contradicts itself: it says file deletion is "best-effort", then says the CR "is never deleted until cleanup succeeds or a bounded budget expires" (no budget exists) and "no logging is recorded" (line 694 logs at Error level).

### C-operator-09: frp tunnel: the operator sends only `name:remotePort`, and the tunnel uses it as both the local port and a TCP-only proxy
- **Location**: `operator/internal/controller/gameserver_tunnel.go:249`, `:524-543`; peer: `tunnel/main.go:311-332`
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. The GameTemplate has port `game` (containerPort 25565, protocol UDP for a UDP game, or TCP) and the GameServer sets `tunnel.frp.remotePorts: [{name: game, remotePort: 30000}]`.
  2. The operator sets `BACKING_SERVICE_PORT=game:30000`. The container port, Service port and protocol are never sent.
  3. `renderFrpConfig` writes `type = "tcp"`, `localIP = <gs>.<ns>.svc`, `localPort = 30000`, `remotePort = 30000`. frpc then forwards to backing Service port 30000, where nothing listens. A UDP port is always proxied as TCP.
- **Expected**: `docs/tunnels.md:39`: "For frp: the protocol (TCP or UDP) for each forwarded port is determined by the port's `protocol` field in the GameTemplate". `docs/tunnels.md:48`: the public address is `<serverAddr>:<remotePort>`, which implies the local side is the game's own port.
- **Actual**: frp tunnels only work when `remotePort` equals the Service port and the port is TCP. The docs example (25565→25565, TCP) happens to satisfy both conditions.

### C-operator-10: held (OD-019)

### C-operator-11: The data-wipe Job swallows every `rm` error and acks the wipe
- **Location**: `operator/internal/controller/gameserver_wipe.go:114-115`, `:65-70`
- **Category**: error-handling
- **Suggested severity**: S3
- **Observation / repro**:
  1. Request a wipe. The Job runs `rm -rf <mount>/..?* <mount>/.[!.]* <mount>/* 2>/dev/null; true` as uid 65532.
  2. Any failure (EACCES on files a different game uid owns on a volume where the kubelet does not apply fsGroup, such as hostPath or local-path PVs; EROFS; EBUSY) is discarded, and the script exits 0.
  3. `job.Status.Succeeded > 0` leads to `ackWipe`, so `gameplane.local/wipe-data-completed` is set and the user is told the wipe finished.
- **Expected**: A wipe that did not remove the data is reported as failed.
- **Actual**: The wipe is always acked as done, whatever `rm` actually did.

### C-operator-12: The `TunnelHostnameIgnored` and `TunnelAddressInvalid` conditions are never cleared
- **Location**: `operator/internal/controller/gameserver_status.go:1113-1135`, `:197-231`, `:989-993`
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. Enable a tunnel and set `spec.networking.hostname`. `TunnelHostnameIgnored=True` is upserted.
  2. Remove the hostname, or disable the tunnel. `computeTunnelConditions` removes only `TunnelReady` (line 991). Nothing ever removes `TunnelHostnameIgnored`: it is upserted once, and `grep` finds no removal.
  3. Similarly, `TunnelAddressInvalid` is removed only when a playit endpoint batch validates (line 225). It stays if the provider changes or the tunnel is disabled.
- **Expected**: Conditions follow the current spec, as `TunnelReady` and `AddressAssignment` do (for example `addressAssignmentCondition` removes itself when nothing is requested, `:695-697`).
- **Actual**: A stale True condition with an outdated message stays on status.

### C-operator-13: `spec.networking.address` is never parsed, although the CRD doc and spec 002 say the operator validates it
- **Location**: `operator/internal/controller/gameserver_controller.go:871-894` (the request is used verbatim), `:911-923`; `operator/api/v1alpha1/gameserver_types.go:336-341`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. Set `expose: LoadBalancer`, `address: "not-an-ip"` (45 characters or fewer, so the CRD accepts it) on a `metallb` or `cilium` cluster.
  2. The value is written as-is into `metallb.io/loadBalancerIPs` or `lbipam.cilium.io/ips`. The operator's only `net.ParseIP` is in playit host validation (`gameserver_status.go:1321`).
  3. The condition reads `AssignmentPending` (or whatever the address manager emits), not an operator-side format error.
- **Expected**: `gameserver_types.go:337-339`: "The operator parses the value during reconciliation and reports a bad one through the AddressAssignment condition instead." `specs/002-nuclear-option-ip-pool/contracts/address-pool-api.md:54`: "invalid formats are reported as status conditions".
- **Actual**: No format check exists.

### C-operator-14: An invalid wake window is documented as a failed condition, but is only a reason string, and `idleOutcome.err` is dead
- **Location**: `operator/internal/controller/gameserver_idle.go:104-107`, `:149-152`, `:291-297`; `operator/api/v1alpha1/gameserver_types.go:181-183`
- **Category**: dead-code
- **Suggested severity**: S4
- **Observation / repro**:
  1. An asleep server has `wakeWindows: ["61 * * * *"]`, which the CRD pattern admits.
  2. `idleDecide` sets `out.err` and `st.Reason = "asleep; wake window invalid: …"`. `reconcileIdle` never reads `out.err`, and no condition is written.
- **Expected**: `gameserver_types.go:181-183`: "the controller parses with robfig/cron/v3 and reports an unparseable entry as a failed condition". `gameserver_idle.go:104-107` says the error is "surfaced as a condition".
- **Actual**: The error only appears in `status.idle.reason`, and it is only evaluated while the server is asleep.

### C-operator-15: Without mTLS, Backups record quiesce and unquiesce as done even though no call was made
- **Location**: `operator/cmd/main.go:403-409`; `operator/internal/controller/backup_controller.go:263-299`, `:542-560`; `operator/internal/agent/client.go:126-128`
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. Start the operator without `--agent-ca-bundle/--agent-client-cert/--agent-client-key`. `agent.New` returns a non-nil `*Client{Disabled: true}`, which is passed as `AgentClient`.
  2. A Backup with `quiesce: true` passes the `r.AgentClient == nil` check. `Quiesce` silently returns nil, and the reconciler stamps `quiesce-attempted=true` and `quiesced-at`, later `unquiesced-at` and `Unquiesced=True`.
- **Expected**: `operator/specs.md:397`: "If agent unavailable, backup proceeds raw (no pause)". The GameServer reconciler handles the same pitfall explicitly (`AgentStopper.Enabled()`, `gameserver_controller.go:194-201`).
- **Actual**: The Backup's annotations and condition claim a quiesce that never happened.

### C-operator-16: Dead code: four GameServerReconciler capture fields and a test-only wrapper
- **Location**: `operator/internal/controller/gameserver_controller.go:151-173` (`CaptureDefaultRetention`, `CaptureMaxRetention`, `CaptureDefaultMaxDurationSeconds`, `CaptureDefaultMaxSizeBytes`); `:2027-2030` (`validateServerEnvSecrets`)
- **Category**: dead-code
- **Suggested severity**: S4
- **Observation / repro**:
  1. `main.go:385-388` sets the four fields. `grep` finds no read of them anywhere in `GameServerReconciler`; only `NetworkCaptureReconciler`'s own copies are used. `main.go:78-79` admits they are "unused elsewhere".
  2. Their doc comments claim behaviour, for example `:153-154` "Used when a GameServer's spec.capture.retentionSeconds is not set".
  3. `validateServerEnvSecrets`, which is "retained for backward compatibility" although it is unexported, is called only from `gameserver_security_test.go`.
- **Expected**: No unread configuration fields with behavioural doc comments.
- **Actual**: They are dead, and their comments describe behaviour that does not exist.

### C-operator-17: `DeleteCaptureFile`'s "still running" error cannot be told apart from other errors
- **Location**: `operator/internal/agent/sidecar_capture.go:228-229`, `:261-263`
- **Category**: error-handling
- **Suggested severity**: S4
- **Observation / repro**:
  1. The sidecar answers 409. The client returns `fmt.Errorf("capture sidecar: delete capture %s: capture still running", id)`, with no sentinel and no `*HTTPError`.
  2. `errors.Is/As` cannot detect it, and `IsTransientError` (`:52-61`) classifies it as transient because it is not an `*HTTPError`.
- **Expected**: Doc comment line 229: "Returns a distinguishable error if the capture is still running (409)."
- **Actual**: It can only be distinguished by string matching.

### C-operator-18: `operator/specs.md`'s capture "known gap" sections are stale
- **Location**: `operator/specs.md:138`, `:289`, `:294`, `:305-308`, `:311-312`, `:405`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `:294` and `:405` say "**so the production build does not compile until both are added**" and that `handlers.go` "defines no delete route". But `go build ./...` passes, `handlers.go:197` registers `DELETE /captures/{id}`, and `sidecar_capture.go:231` implements `DeleteCaptureFile`.
  2. `:289` says the `NetworkCaptureReconciler{...}` literal "does not yet set `CaptureDefaultRetentionSeconds`/`CaptureMaxRetentionSeconds`". `main.go:439-440` sets them.
  3. `:305-306` and `:312` give the routes as `/captures/{id}:start` / `:stop`. The code uses `/captures/%s/start` and `/stop` (`sidecar_capture.go:107`, `:156`), matching the sidecar.
  4. `:311` says "all methods return nil on Disabled=true". `GetCaptureStatus` returns an error when disabled (`sidecar_capture.go:195-197`), and `Reconcile` relies on that (`networkcapture_controller.go:368-371`).
  5. `:138` says the ephemeral-container injection is "planned for Phase 2 Implementation (T030+)", while `:163` lists it as Built. It is implemented (`gameserver_controller.go:1730-1744`).
- **Expected**: specs.md describes the current code.
- **Actual**: specs.md claims the build is broken and a feature is unwired, when both work.

### C-operator-19: The `operator/specs.md` flag table misses 7 flags and 2 environment defaults
- **Location**: `operator/specs.md:336-361`; `operator/cmd/main.go:169-180`, `:193-200`, `:224-235`, `:247`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**: The table omits `--sentinel-image`, `--tunnel-frp-image`, `--tunnel-tailscale-image`, `--tunnel-playit-image`, `--metallb-namespace`, `--capture-default-max-duration-seconds` and `--capture-default-max-size-bytes`. It also does not document that `--address-manager` defaults from `GAMEPLANE_ADDRESS_MANAGER` and `--game-data-storage-class` from `GAMEPLANE_GAME_DATA_STORAGE_CLASS`.
- **Expected**: The "CLI flags" table lists every flag.
- **Actual**: 7 of 31 flags are missing.

### C-operator-20: The `operator/specs.md` Go version and dependency table are stale
- **Location**: `operator/specs.md:5`, `:413-431`; `operator/go.mod:3`, `:12-31`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**: specs.md says `Go version: 1.25.0`; go.mod has `go 1.26.0`. The table has `k8s.io/api|apimachinery|client-go v0.35.0` (go.mod: v0.37.0), `controller-runtime v0.19.0` (v0.25.1), `go-containerregistry v0.20.7` (v0.22.1), `external-snapshotter/client/v8 v8.0.0` (v8.6.0), `prometheus/client_golang v1.23.2` (v1.24.1), `cosign/v2 v2.6.3` (v2.6.5), `sigstore v1.10.8` (v1.10.9, now `// indirect`), `x/crypto v0.50.0` (v0.57.0), `x/mod v0.35.0` (v0.41.0) and `oras-go/v2 v2.6.0` (v2.6.2). `hack/check-doc-versions.sh` only checks Gameplane version strings in 18 other files, so this is not the tracked checker-pattern issue.
- **Expected**: The versions match go.mod.
- **Actual**: 12 dependency rows, plus the Go version line, are stale.

### C-operator-21: `operator/specs.md` states behaviours the code does not have (wipe, finalizers, Restore resume, conditions, counts)
- **Location**: `operator/specs.md:29`, `:46`, `:53`, `:95`, `:110`, `:113`, `:144`, `:218-219`, `:227`, `:239`, `:464`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `:144` "`gameserver_wipe.go`: data wipe sequence (delete PVC, recreate)". The code runs an `rm -rf` Job against the existing PVC (`gameserver_wipe.go:84-135`).
  2. `:29` says kubectl delete triggers "the operator's cleanup finalizers", and `:464` says "Controllers use ownership and finalizers ... Backup deletion removes associated Jobs". The only finalizer is on Module (`module_controller.go:68`). GameServer and Backup cleanup relies solely on ownerReference GC.
  3. `:113` lists a `Resuming` phase, and `:218-219` says "Resume: clear suspend flag, wait for GameServer to reach Running phase". `restore_controller.go:153-181` clears `suspend` and sets `Succeeded` in the same pass. `RestorePhaseResuming` is never set.
  4. `:239` says "Report InstallFailed condition". There is no such condition; failures are `Ready=False` with reasons such as `PullFailed`, `SignatureInvalid` or `IncompatibleOperator` (`module_controller.go:79-152`).
  5. `:227` says "read optional secret referenced in spec.auth". There is no `spec.auth`; the fields are `spec.oci.pullSecretRef`, `spec.git.secretRef` and `spec.http.secretRef`.
  6. `:46`, `:53` and `:95` say "8 kinds", "8 controllers" and "Eight CRD kinds", while `:116` says 9. There are 9 CRDs and 9 reconcilers.
  7. `:110` says the GameServer "Spec declares desired replica count". `GameServerSpec` has no replicas field.
- **Expected**: specs.md matches the code.
- **Actual**: At least 7 contradicting statements.

### C-operator-22: The `operator/config` dev manifests and samples do not produce a working install
- **Location**: `operator/config/crd/kustomization.yaml:3-10`; `operator/config/samples/gameplane_v1alpha1_gameserver.yaml:8-12`; `operator/config/samples/gameplane_v1alpha1_modulesource.yaml:9-15`; `operator/config/samples/gameplane_v1alpha1_backup.yaml:9-11`; `operator/config/manager/manager.yaml:30-34`
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. `config/crd/kustomization.yaml` lists 7 CRDs and omits `gameplane.local_clusters.yaml` and `gameplane.local_networkcaptures.yaml`. With `kubectl apply -k`, the operator's Cluster and NetworkCapture informers can never sync.
  2. The sample GameServer sets `config: {TYPE, VERSION, DIFFICULTY, MAX_PLAYERS}`. The sample GameTemplate has no `configSchema`, and the real `minecraft-java` module schema has no `TYPE` or `VERSION`. `materializeConfig` (`gameserver_config.go:97-106`) fails the server with "unknown config keys".
  3. The sample ModuleSource uses the old flat `spec.url` / `spec.modules`. The current schema is `spec.type` plus `spec.oci.{url,modules}`, so the CEL rule `self.type != 'oci' || has(self.oci)` rejects it.
  4. The sample Backup sets `repoRef.key: url`, but the operator ignores `repoRef.key` and always reads keys `repo` and `password` (`backup_controller.go:236-239`, `:653-665`), although `SecretKeySelector.Key` is required by the CRD (`gametemplate_types.go:1163-1168`). The dashboard sends `key: "url"` in one place and `"repo"` in another (`web/src/routes/tabs/Backups.tsx:46`, `web/src/routes/Backups.tsx:216`).
  5. `manager.yaml` passes no agent CA or mTLS flags and mounts nothing. `reconcileAgentTLS` then fails on the default `gameplane-system/gameplane-agent-ca` Secret, and every GameServer reconcile errors (`agent_certs.go:69-74`).
- **Expected**: `operator/specs.md:45-50` describes `config/` as the generated manifests, "Manager deployment" and "Sample ... CRs for testing". `config/rbac/role_namespace.yaml:6-8` says the files exist "so `kubectl apply -f operator/config/rbac/` works for local dev".
- **Actual**: No part of the `config/` dev path works end to end against the current CRDs and controllers. The RBAC half of this is in the held file.

## Questions (not findings)

- **BackupSchedule first fire**: `backupschedule_controller.go:100-105` seeds `prev := now.Add(-time.Hour)` when `LastScheduleTime` is nil. A schedule created at 10:30 with `0 * * * *` fires at creation, while a daily `0 3 * * *` does not. This also applies to the `<gs>-auto` schedule created with a brand-new server. Is the one-hour catch-up window intentional? It differs from CronJob semantics and has no comment.
- **Multi-node RWO**: the backup, restore and wipe Jobs mount the game's RWO data PVC with no node affinity to the running game pod, and without `activeDeadlineSeconds`. With network-attached RWO volumes (Longhorn, EBS, RBD) and the game pod running, can the backup pod be scheduled to another node and hang in Multi-Attach forever, leaving the Backup Running? This could not be checked without a multi-node cluster.
- **Wipe or restore as uid 65532**: related to C-operator-11. On k3s local-path (hostPath) PVs, which do not get fsGroup ownership changes, can the uid 65532 Jobs delete or write files the game image created as another uid (for example 1000)?
- **Restore resumes a user-stopped server**: `restore_controller.go:153-159` always sets `suspend: false` on success, even when the user had stopped the server before the restore. Is that intended? It matches `restore_types.go:79-81`, but may surprise users.
- **Volume-snapshot restore copies hostname and address**: `restore_volumesnapshot.go:74` copies the whole original spec, including `networking.hostname` (the external-dns annotation) and `networking.address`, so the original and the restored server end up with the same DNS name and requested LB IP. `docs/architecture.md:174` says "preserving the original server's spec". Is duplicating these fields intended?
- **kubectl-applied NetworkCapture retention**: `spec.capture.retentionSeconds` on the GameServer is applied only by the API (`api/internal/handlers/capture.go:204-213`). A NetworkCapture applied with kubectl and without `ttlSecondsAfterFinished` gets the cluster default in the operator (`networkcapture_controller.go:574-583`), which differs from the "kubectl apply equals dashboard" house rule (`operator/specs.md:9`).
- Q-operator-capture-immutability: held (OD-019)
- **Reconcile errors without status**: any error before `reconcileStatus` (agent CA Secret missing, env or tunnel Secret refusal, StatefulSet update rejected) leaves the GameServer status unchanged, and the cause shows only in operator logs. Only the missing-template, invalid-config and unknown-version paths call `setPhase`. Is this intended?
- **`Stopped` phase**: is it reserved for future use? `derivePhase` never produces it.
