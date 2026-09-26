# T045 operator chunk: independent verification (opus)

Verifier input: `notes.md` in this directory (opus reviewer), candidates C-operator-01..22. Held candidates for this component are verified separately under OD-019, and none of their details appear here.

Method: I checked every candidate against master `13a859ff`. `git diff master` shows no change on this branch under `operator/`, `api/`, `web/src`, `charts/`, `tunnel/`, `capture-sidecar/` or `docs/`. For each candidate I read the cited lines, their callers and callees, the tests that touch the path, `operator/specs.md`, `docs/architecture.md`, `docs/tunnels.md`, spec 002 (`contracts/address-pool-api.md`, `data-model.md`), `specs/done_003-network-capture-sidecar/`, and `audit/findings.md`. None of the kept items is tracked there. F-031 is also about `docs/tunnels.md`, but it covers a different defect. I also read the peer code the operator talks to: `agent/internal/quiesce/quiesce.go`, `tunnel/main.go`, `capture-sidecar/internal/httpserver/handlers.go`, `api/internal/handlers/resources.go`, `api/internal/kube/capture.go`, the dashboard files named below, `charts/gameplane/templates/operator.yaml` and `modules/*/template.yaml`. `go build ./...` in `operator/` passes. No cluster was reachable from this session (kubectl has no context), so every observation comes from reading master. The live steps are written so a maintainer can replay them on kubelab. No test or lint suite was run, and no repo file other than this one was written.

Severity follows research R3. Three changes from the reviewer: C-operator-01 goes from S2 to S3, because a workaround exists and the failure is visible on the Backup. C-operator-08 goes from S3 to S4, because it clears itself and keeps no data. C-operator-15 and C-operator-17 are rejected.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-operator-01 | kept | S3 | Confirmed. `mirrorJobStatus` writes the terminal phase (`backup_controller.go:411-415`) before `runUnquiesce` (`:427-428`). A failed unquiesce requeues (`:480-487`), but the next pass returns at `:187-190`, so the retry never happens. The same order appears in `backup_volumesnapshot.go:129/133` and `:141/144`. Only `failUnrestorable` (`:140-148`) releases the world first. The agent has no timeout that re-enables saving. Downgraded from S2: the `Unquiesced=False` condition is visible, and restarting the server or taking a later quiesced Backup of the same server restores saving. |
| C-operator-02 | kept | S3 | Confirmed. The `<gs>-auto` mutate function (`gameserver_controller.go:2218-2225`) never sets `Quiesce`. The field has no `omitempty` (`backupschedule_types.go:48`), so the create sends an explicit `false`, and `fire()` copies it into every Backup (`backupschedule_controller.go:184`). `InlineBackupPolicy` (`gameserver_types.go:467-485`) has no quiesce field. Workaround: patching `<gs>-auto` to `quiesce: true` sticks, because the mutate function never resets it. |
| C-operator-03 | kept | S3 | Confirmed. `entry.Digest` is the digest of the latest tag only (`modsrc/oci.go:81`, `:97`), and the convergence check always compares it (`module_controller.go:100-102`). A Module pinned to an older OCI version therefore never converges. Each pass writes Pulling, then Ready, and each write re-triggers the unfiltered `For(&Module{})` watch, so the loop never stops. The envtest fixture leaves the digest out on purpose (`module_upgrade_envtest_test.go:47-48`). |
| C-operator-04 | kept | S3 | Confirmed. The StatefulSet selector includes `gameplane.local/template: <tmpl.Name>` (`gameserver_controller.go:1373-1380`), and a StatefulSet selector is immutable. No CRD has an `oldSelf` rule, and the API PUT path (`resources.go:211-275`) does not check `templateRef`. The error returns at `:400-403`, before status is written. The dashboard disables the field (`General.tsx:51`), so this only happens through kubectl or the raw API. Reverting the field is the workaround. |
| C-operator-05 | kept | S3 | Confirmed. Backup has no finalizer (the only `AddFinalizer` is `module_controller.go:68`), and a deleted Backup ends at `IgnoreNotFound` (`backup_controller.go:175-177`). The dashboard Delete button is enabled in every phase (`BackupDetailDrawer.tsx:100-106`), so a Running quiesced Backup can be deleted from the UI. The `Replace` path (`backupschedule_controller.go:265-270`) usually heals itself, because the successor quiesces and unquiesces again. |
| C-operator-06 | kept | S3 | Confirmed. The restore runs `restic restore <id> --target /` (`restore_controller.go:249`) without `--delete`, so files created after the snapshot survive, which contradicts `restore_types.go:27-30`. Concrete case: `modules/factorio/template.yaml:49-52` sets `LOAD_LATEST_SAVE=true`, so a save written after the snapshot is the one that loads. Workaround: wipe, then restore. |
| C-operator-07 | kept | S3 | Confirmed. The `Owns(&GameTemplate{})` event (`module_controller.go:459`) lands on the converged early return (`:100-107`), which never reads the template. A deleted or edited managed template stays deleted or edited. The finalizer blocks uninstall and reinstall while servers still reference it (`:258-295`). Workaround: re-pin the Module to another version and back. |
| C-operator-08 | kept | S4 | Confirmed. `expireCapture` returns at `networkcapture_controller.go:710` on any delete error, so `r.Delete` (`:713`) waits until the sidecar answers. That contradicts `operator/specs.md:293` and the function's own doc comment (`:673-679`). Downgraded from S3: no file is kept (the emptyDir went with the pod), the next start of the server gets a 404, which counts as success (`sidecar_capture.go:257-260`), and deleting the GameServer garbage-collects the capture through its ownerReference (`api/internal/kube/capture.go:136-180`). |
| C-operator-09 | kept | S3 | Confirmed. The operator sends only `name:remotePort` (`gameserver_tunnel.go:249`, `:524-543`). `renderFrpConfig` then writes `type = "tcp"` and `localPort = remotePort` (`tunnel/main.go:320-332`). The backing Service listens on the template port (`gameserver_controller.go:1114-1126`). This contradicts `docs/tunnels.md:39`. TCP games have a workaround (set `remotePort` to the Service port). UDP games do not. |
| C-operator-10 | kept | held (OD-019) | held (OD-019) |
| C-operator-11 | kept | S3 | Confirmed from the code: `... 2>/dev/null; true` (`gameserver_wipe.go:114-115`) always exits 0, and `Succeeded > 0` acks the wipe (`:64-70`). The failure trigger (volumes without fsGroup, such as hostPath PVs, with game files owned by uid 1000/25000/root) is plausible but could not be run here. |
| C-operator-12 | kept | S4 | Confirmed. `TunnelHostnameIgnored` is only ever upserted (`gameserver_status.go:1126-1135`). `computeTunnelConditions` removes only `TunnelReady` (`:989-993`). `TunnelAddressInvalid` is removed only when a playit batch validates cleanly (`:224-225`). |
| C-operator-13 | kept | S4 | Confirmed. `spec.networking.address` is copied straight into the MetalLB or Cilium annotation (`gameserver_controller.go:871-876`, `:911-923`). The only IP parse in the controller is for playit (`gameserver_status.go:1321`). This contradicts `gameserver_types.go:336-339`, `specs/002-…/contracts/address-pool-api.md:54` and `data-model.md:63`. |
| C-operator-14 | kept | S4 | Confirmed. `idleOutcome.err` is set (`gameserver_idle.go:149-152`), but `reconcileIdle` returns `out.state, out.status, out.requeue` and never reads it (`:291-297`). No condition is written, which contradicts `gameserver_types.go:181-183` and `gameserver_idle.go:104-107`. |
| C-operator-15 | rejected | n/a | Not reachable in a real install. The chart always passes `--agent-ca-bundle/--agent-client-cert/--agent-client-key` (`charts/gameplane/templates/operator.yaml:277-279`). The disabled client (`agent/client.go:61-63`) exists only when the operator runs outside the chart, for example `go run`, or `config/manager/manager.yaml`, which C-operator-22 already covers. The mismatch it describes is real, but only on a dev path. |
| C-operator-16 | kept | S4 | Confirmed. Nothing reads the four `GameServerReconciler.Capture*` fields (`gameserver_controller.go:151-173`). They are only assigned in `main.go:385-388`, and `main.go:78-79` admits they are unused. Their doc comments describe behaviour that does not exist. `validateServerEnvSecrets` (`:2027-2030`) is called only from `gameserver_security_test.go`. |
| C-operator-17 | rejected | n/a | No behavioural effect, only wording. No caller distinguishes or classifies `DeleteCaptureFile` errors. `expireCapture` treats every error the same way (`networkcapture_controller.go:693-710`), and `IsTransientError` is called only for `StartCapture` (`:382`). The 409 message is also a unique string, so the comment is loosely accurate. |
| C-operator-18 | kept | S4 | Confirmed. `go build ./...` passes. `handlers.go:197` registers `DELETE /captures/{id}`. `sidecar_capture.go:231-268` implements the delete. `main.go:439-440` wires retention. The client uses `/start` and `/stop` paths (`sidecar_capture.go:107`, `:156`). `GetCaptureStatus` returns an error when disabled (`:195-197`). Injection is built (`:163`). specs.md `:138`, `:289`, `:294`, `:305-306`, `:311-312` and `:405` say otherwise. |
| C-operator-19 | kept | S4 | Confirmed by diffing the flag names: `main.go` defines 31 and the table lists 24. Missing: `--sentinel-image`, `--tunnel-{frp,tailscale,playit}-image`, `--metallb-namespace`, `--capture-default-max-duration-seconds` and `--capture-default-max-size-bytes`. The env defaults (`main.go:224`, `:247`) are not documented. |
| C-operator-20 | kept | S4 | Confirmed. 12 dependency rows and the Go line are stale against `go.mod` (the values are in the section below). The Go-version line is also named, as a related site, in C-gameaction-04, and the two can be fixed together. |
| C-operator-21 | kept | S4 | All 7 statements confirmed against code (see the section below). |
| C-operator-22 | kept | S4 | Confirmed on the dev path only; the chart install is unaffected. `config/crd/kustomization.yaml` lists 7 of the 9 CRDs. The sample GameServer's `TYPE`/`VERSION` are not in the `minecraft-java` `configSchema`. The sample ModuleSource uses the removed flat schema, and `spec.type` defaults to `oci` (so the CEL rule rejects it). The sample Backup's `repoRef.key` is ignored. `manager.yaml` passes no agent CA flags. |

Rejected: 2 (C-operator-15, C-operator-17). Kept: 20.

C-operator-03 and C-operator-07 share one code site, the converged early return at `module_controller.go:100-107`, so one rework of that check can fix both. They stay separate items because their triggers and their tests differ.

### C-operator-01

**Location:** `operator/internal/controller/backup_controller.go:186-190` (terminal guard), `:411-415` then `:427-439` (status persisted before `runUnquiesce`), `:480-487` (`runUnquiesce` requeue that is never honoured); `operator/internal/controller/backup_volumesnapshot.go:129` then `:133`, and `:141` then `:144`.

**Repro / observation**
1. On a test cluster, create a GameServer from a template that declares quiesce (for example `minecraft-java`), and a restic Backup with `spec.quiesce: true` (the CRD default). `maybeQuiesce` (`backup_controller.go:263-299`) calls the agent and stamps `backup.gameplane.local/quiesce-attempted=true`. The agent has now run the template's quiesce commands (auto-save off).
2. While the Backup is `Running`, cut the operator off from the agent: `kubectl -n gameplane-games delete networkpolicy allow-api-to-agent`. The chart's `default-deny-ingress` then drops operator→agent `:8090`. `helm upgrade` restores the policy.
3. When the restic Job succeeds, `mirrorJobStatus` writes `phase: Succeeded` (`:411-415`), then `runUnquiesce` fails, sets `Unquiesced=False` and returns `RequeueAfter: 30s`.
4. Restore the NetworkPolicy. The requeued pass returns at `:187-190`. `kubectl -n gameplane-games get backup <name> -o jsonpath='{.metadata.annotations}'` never gains `backup.gameplane.local/unquiesced-at`, and `Unquiesced=False` stays.
5. The game keeps auto-save off. `agent/internal/quiesce/quiesce.go` has no timer that re-enables it.
6. Reading only: `completeVolumeSnapshot` and `failVolumeSnapshot`, and the `Failed` branch of `mirrorJobStatus`, persist the terminal phase first in the same way. `failUnrestorable` (`:132-148`) is the only path that unquiesces before going terminal, and its comment explains why.

**Expected:** The terminal phase is persisted only after the unquiesce lands, as `failUnrestorable` does. `docs/architecture.md:154-158` says "the unquiesce runs first, and while the agent is unreachable the operator requeues rather than failing". The `runUnquiesce` doc comment (`:474-479`) says it is "retried".

**Actual:** One failed unquiesce is final, and auto-save stays off until someone runs the game's resume command or restarts the server. A later quiesced Backup of the same server also fixes it, but auto-managed schedules never quiesce (C-operator-02), so they never do. A crash while auto-save is off loses everything since the last save.

### C-operator-02

**Location:** `operator/internal/controller/gameserver_controller.go:2218-2225`; `operator/api/v1alpha1/backupschedule_types.go:37-48`; `operator/internal/controller/backupschedule_controller.go:184`; `operator/api/v1alpha1/gameserver_types.go:467-485`.

**Repro / observation**
1. Dashboard: Server, then Settings, then Backups. Set a schedule and destination and save. `web/src/routes/tabs/settings/Backups.tsx:19-31` writes `spec.backupPolicy`.
2. `kubectl -n gameplane-games get backupschedule <gs>-auto -o jsonpath='{.spec.quiesce}'` prints `false`.
3. Reading: the `CreateOrUpdate` mutate function sets ServerRef, Schedule, RepoRef, Retention and Suspend. `Quiesce` keeps its Go zero value. It has no `omitempty`, so the typed client sends `"quiesce": false`, and the apiserver's `default=true` applies only to absent fields.
4. When the schedule fires, `fire()` copies `quiesce: false` into the Backup, and `maybeQuiesce` returns at `backup_controller.go:264-266`.

**Expected:** The auto-managed schedule gets the CRD default (`true`), or `InlineBackupPolicy` exposes the choice. The type comment at `backupschedule_types.go:39-45` guards the opposite direction only.

**Actual:** Every scheduled backup of a dashboard-configured server runs without quiesce. Workaround: `kubectl patch backupschedule <gs>-auto --type merge -p '{"spec":{"quiesce":true}}'`. The patch persists because the mutate function never touches the field.

### C-operator-03

**Location:** `operator/internal/controller/module_controller.go:100-107`; `operator/internal/modsrc/oci.go:81-97`.

**Repro / observation**
1. Push two versions of one module (for example `1.0.0` and `1.1.0`) to an OCI registry, and point an OCI ModuleSource at it. After indexing, the ModuleSource's `status.modules[].digest` holds the manifest digest of `1.1.0` (`oci.go:81` pulls only `LatestVersion`, and `:97` records its digest).
2. Install a Module, or roll one back, with `spec.version: "1.0.0"`. This is the documented rollback: "re-pinning spec.version" (`module_types.go:67-73`). It applies, and `status.appliedDigest` is the digest of `1.0.0`.
3. On the next reconcile, `:100-102` requires `appliedDigest == entry.Digest`, which can never hold. `markPullingTransition` writes `Phase=Pulling`, and the reconciler pulls, cosign-verifies and re-applies, then writes `Phase=Ready`. Both status writes re-enqueue the Module (`SetupWithManager` at `:456-461` has no predicate).
4. `kubectl get module <name> -w` flips between Pulling and Ready without stopping. The registry's access log shows repeated manifest and blob GETs.
5. The envtest fixture avoids this on purpose: `module_upgrade_envtest_test.go:47-48` says "No digest: convergence keys on version alone". So no test covers the production OCI shape.

**Expected:** A pinned install converges once it is applied. The catalog digest describes `LatestVersion` only, so compare it only when `desiredVersion == entry.LatestVersion`.

**Actual:** Continuous pulls and signature checks, and a Module that flaps between Pulling and Ready. On a rate-limited registry this ends in `PullFailed`. The applied GameTemplate content stays correct.

### C-operator-04

**Location:** `operator/internal/controller/gameserver_controller.go:1373-1380` (selector includes the template name), `:400-403` (error return before `reconcileStatus`); `operator/api/v1alpha1/gameserver_types.go:35-37` (no transition rule); `api/internal/handlers/resources.go:211-275` (no `templateRef` check on PUT).

**Repro / observation**
1. Create GameServer `mc` from `minecraft-java` and wait for `Running`.
2. `kubectl -n gameplane-games patch gameserver mc --type merge -p '{"spec":{"templateRef":{"name":"<another installed template>"}}}'`. It is admitted: `grep -c oldSelf charts/gameplane/crds/*.yaml` is 0 for every CRD.
3. Operator logs show `reconcile StatefulSet` failing with the apiserver's `spec: Forbidden: updates to statefulset spec for fields other than …`.
4. `kubectl -n gameplane-games get gameserver mc -o yaml`: `status.observedGeneration` stays at the old generation, and no condition names the cause. Later suspend, config or port changes do not apply.
5. The dashboard shows the template field disabled (`web/src/routes/tabs/settings/General.tsx:51`), so only kubectl or a direct API PUT reaches this.

**Expected:** `templateRef` is immutable at admission (for example a CEL `self == oldSelf` rule on `spec.templateRef`), or the selector does not depend on it.

**Actual:** The server is wedged, and only operator logs explain why. Reverting `templateRef` recovers it.

### C-operator-05

**Location:** `operator/internal/controller/backup_controller.go:173-177`; `operator/internal/controller/backupschedule_controller.go:265-270`; `web/src/components/backups/BackupDetailDrawer.tsx:100-106`.

**Repro / observation**
1. Start a quiesced restic Backup on a server with enough data that the Job runs for a while.
2. While it is `Running` with `quiesce-attempted=true`, open it in the dashboard's Backups list and press Delete (the button is enabled in every phase), or `kubectl delete backup <name>`.
3. The Backup disappears at once. `grep -rn AddFinalizer operator/internal` finds only `module_controller.go:68`. The Job is garbage-collected, and the next reconcile returns `IgnoreNotFound`. No unquiesce is sent, and the game stays with auto-save off.
4. `concurrencyPolicy: Replace` deletes in-flight Backups the same way. There the successor normally quiesces and later unquiesces again, which releases the world, unless the successor fails its own validation (`:195-239`) before it quiesces.

**Expected:** Deleting a Backup that the operator quiesced releases the game world. For example, a finalizer runs `maybeUnquiesce` before letting go.

**Actual:** Auto-save can stay off indefinitely. Restarting the server is the workaround.

### C-operator-06

**Location:** `operator/internal/controller/restore_controller.go:249`; claim at `operator/api/v1alpha1/restore_types.go:27-30`; pinned image `backup_controller.go:153` (`restic/restic:0.17.1`).

**Repro / observation**
1. Create server `fac` from the `factorio` module. `modules/factorio/template.yaml:49-52` sets `LOAD_LATEST_SAVE=true`, so the image boots the newest save. Take a restic Backup at T0.
2. Keep the server running until a save file appears that is not in the T0 snapshot (a new autosave slot, or a named save from the `save-world` action).
3. Create a Restore of the T0 Backup into `fac`. The Job runs `restic restore <id> --target /`, with no `--delete`.
4. After the resume, the Files tab still lists the post-T0 save with its newer mtime, and the server loads it. The same applies to Minecraft region files for chunks first generated after T0, and to new `playerdata/*.dat` files.
5. restic 0.17.0 added `restore --delete`. Confirm it exists in the pinned image: `kubectl run restic-help --rm -it --restart=Never --image=restic/restic:0.17.1 -- restore --help`.

**Expected:** After a restore, the data volume matches the snapshot ("its data volume is overwritten in place").

**Actual:** The volume is the snapshot plus every file created after it. Workaround: wipe the server first (see C-operator-11), then restore.

### C-operator-07

**Location:** `operator/internal/controller/module_controller.go:100-107` (early return), `:456-461` (`Owns(&GameTemplate{})`).

**Repro / observation**
1. Module `minecraft-java` is `Ready`.
2. `kubectl delete gametemplate minecraft-java`. The API refuses changes to managed templates, but kubectl does not.
3. The `Owns` watch enqueues the Module. Every field of the convergence check still matches, so it returns without reading the template. `kubectl get gametemplate minecraft-java` stays `NotFound`.
4. GameServers that reference it go `Failed` with `GameTemplate "minecraft-java" not found` (`gameserver_controller.go:266-270`).
5. `kubectl edit gametemplate minecraft-java` behaves the same way: the edit persists.
6. Uninstalling and reinstalling is blocked by the finalizer while servers still reference the template (`:258-295`). What works is re-pinning the Module to another version and back.

**Expected:** `operator/specs.md:29`: changes that bypass the CRD "are not supported — the operator's next reconcile will overwrite them". `docs/architecture.md:204-206`: the Module "materializes an owned GameTemplate".

**Actual:** The owned template is not recreated or reverted while the Module's status fields still match.

### C-operator-08

**Location:** `operator/internal/controller/networkcapture_controller.go:693-713`; doc comment `:673-679`.

**Repro / observation**
1. With the chart's `capture.enabled=true`, run a capture on server `mc` until it is `Completed`, with a short retention.
2. Stop `mc` (`spec.suspend: true`). The pod and its `captures` emptyDir are removed.
3. When the TTL elapses, `expireCapture` marks the capture `Expired` and calls `DeleteCaptureFile`. The call fails because `<gs>-agent` has no endpoints. The function records `FileCleanupFailed` and returns `RequeueAfter: 30s` at `:710`, so it never reaches `r.Delete` at `:713`. Operator logs show the error every 30 s.
4. `kubectl -n gameplane-games get networkcapture` shows it `Expired` for as long as `mc` stays stopped. When `mc` starts again, the new sidecar answers 404, `DeleteCaptureFile` treats that as success (`operator/internal/agent/sidecar_capture.go:257-260`), and the CR is deleted.

**Expected:** `operator/specs.md:293`: file deletion is best-effort, "never blocks the CR delete". The function's own first sentence says the same.

**Actual:** The CR delete waits on the sidecar. The doc comment contradicts itself: "best-effort", then "never deleted until cleanup succeeds or a bounded budget expires" (no budget exists), and "no logging is recorded" (`:694` logs at Error).

### C-operator-09

**Location:** `operator/internal/controller/gameserver_tunnel.go:241-249`, `:522-543`; peer `tunnel/main.go:304-332`; Service ports `operator/internal/controller/gameserver_controller.go:1102-1131`.

**Repro / observation**
1. Create a GameServer from a UDP game (for example `factorio`, port `game` 34197/UDP) with `networking.tunnel: {enabled: true, provider: frp, frp: {serverAddr: <frps>, remotePorts: [{name: game, remotePort: 30000}]}}`.
2. `kubectl -n gameplane-games get deploy <gs>-tunnel -o jsonpath='{.spec.template.spec.containers[0].env}'` shows `BACKING_SERVICE_PORT=game:30000`. No container port, Service port or protocol is passed.
3. Reading `renderFrpConfig`: every entry becomes `type = "tcp"`, `localPort = 30000`, `remotePort = 30000`, and is written to `/tmp/gameplane-tunnel-frpc.toml`.
4. The backing Service `<gs>` exposes 34197/UDP (`svcPortsFromTemplate`), so frpc forwards TCP to `<gs>.<ns>.svc:30000`, where nothing listens. A player connecting to `<serverAddr>:30000` gets nothing.

**Expected:** `docs/tunnels.md:39`: the protocol comes from the template port's `protocol`. `:48`: the public address is `<serverAddr>:<remotePort>`, which forwards to the game's own port.

**Actual:** frp works only when `remotePort` equals the Service port and the port is TCP. The docs example (25565/TCP to 25565) happens to meet both conditions.

### C-operator-10: held (OD-019)

### C-operator-11

**Location:** `operator/internal/controller/gameserver_wipe.go:114-115` (`… 2>/dev/null; true`), `:64-70` (ack on `Succeeded`).

**Repro / observation**
1. Reading: the wipe script always exits 0, so the Job always succeeds, and `ackWipe` sets `gameplane.local/wipe-data-completed` whatever `rm` did.
2. Live (not run here, because no cluster was available): use a StorageClass where the kubelet does not apply `fsGroup` ownership (hostPath-backed PVs, such as k3s `local-path`; check with `kubectl get pv <pv> -o jsonpath='{.spec.hostPath}'`) and a game that writes as another uid (most templates set `runAsUser: 1000`, for example `modules/cs2/template.yaml:66`; `ark-survival-ascended` uses 25000). Its subdirectories are mode 0755. From Server, then Settings, then Danger, choose "Wipe world…".
3. The Job runs as uid 65532, cannot empty those directories, and discards the `EACCES`. The server is acked as wiped, but the Files tab still lists the world.

**Expected:** A wipe that did not remove the data is reported as failed, and the request is not acked.

**Actual:** Every wipe is acked as done, whatever `rm` actually did.

### C-operator-12

**Location:** `operator/internal/controller/gameserver_status.go:1113-1135` (upsert only), `:989-993` (only `TunnelReady` removed), `:197-226` (`TunnelAddressInvalid` removed only on a clean playit batch).

**Repro / observation**
1. Enable an frp tunnel on `mc` and set `spec.networking.hostname`. `TunnelHostnameIgnored=True` appears.
2. Remove the hostname, or disable the tunnel. `kubectl -n gameplane-games get gameserver mc -o jsonpath='{.status.conditions[?(@.type=="TunnelHostnameIgnored")]}'` still returns the condition.
3. `grep -n '"TunnelHostnameIgnored"' operator/internal/controller/*.go` finds only the upsert.

**Expected:** Conditions follow the current spec, as `TunnelReady` and `AddressAssignment` (`:695-697`) do.

**Actual:** A stale `True` condition with an outdated message stays on status.

### C-operator-13

**Location:** `operator/internal/controller/gameserver_controller.go:871-876`, `:911-923`; claims at `operator/api/v1alpha1/gameserver_types.go:336-339`, `specs/002-nuclear-option-ip-pool/contracts/address-pool-api.md:54`, `specs/002-nuclear-option-ip-pool/data-model.md:63`.

**Repro / observation**
1. With `--address-manager=metallb`, set `expose: LoadBalancer` and `address: "not-an-ip"`. The CRD admits it (only `MaxLength=45`).
2. The Service gets `metallb.io/loadBalancerIPs: not-an-ip`, and `AddressAssignment` reports a pending reason, not a format error.
3. `grep -rn "ParseIP\|ParseAddr" operator/internal/controller/*.go` finds only `gameserver_status.go:1321` (playit host validation).

**Expected:** "The operator parses the value during reconciliation and reports a bad one through the AddressAssignment condition".

**Actual:** No format check exists. The bad value goes straight to the address manager.

### C-operator-14

**Location:** `operator/internal/controller/gameserver_idle.go:104-107`, `:149-152`, `:291-297`; claim at `operator/api/v1alpha1/gameserver_types.go:181-183`.

**Repro / observation**
1. Set `spec.idle.wakeWindows: ["61 * * * *"]`. The CRD pattern only checks for five fields.
2. Let the server fall asleep. `status.idle.reason` reads `asleep; wake window invalid: …`.
3. No condition is added. `reconcileIdle` returns `out.state, out.status, out.requeue, nil` and never reads `out.err`.

**Expected:** "reports an unparseable entry as a failed condition" (`gameserver_types.go:181-183`), and "surfaced as a condition" (`gameserver_idle.go:104-107`).

**Actual:** The error appears only in `status.idle.reason`, and only while the server is asleep. `idleOutcome.err` is dead in production code.

### C-operator-16

**Location:** `operator/internal/controller/gameserver_controller.go:151-173`, `:2027-2030`; `operator/cmd/main.go:78-79`, `:385-388`.

**Repro / observation**
1. `grep -rn "CaptureDefaultRetention\b\|CaptureMaxRetention\b\|CaptureDefaultMaxDurationSeconds\|CaptureDefaultMaxSizeBytes" operator` finds, for `GameServerReconciler`, only the field declarations and the assignments in `main.go:385-388`. The reads at `networkcapture_controller.go:264-268` are `NetworkCaptureReconciler`'s own fields.
2. The doc comments claim behaviour, for example `:153-154` "Used when a GameServer's spec.capture.retentionSeconds is not set". `main.go:78-79` says these fields are "unused elsewhere".
3. `grep -rn validateServerEnvSecrets operator` finds only the definition and `gameserver_security_test.go`.

**Expected:** No unread configuration fields that carry behavioural doc comments.

**Actual:** Four dead fields with misleading comments, and a test-only wrapper described as "retained for backward compatibility" although it is unexported.

### C-operator-18

**Location:** `operator/specs.md:138`, `:289`, `:294`, `:305-306`, `:311-312`, `:405`.

**Repro / observation**
1. `cd operator && go build ./...` succeeds. `capture-sidecar/internal/httpserver/handlers.go:197` registers `DELETE /captures/{id}`, and `operator/internal/agent/sidecar_capture.go:231-268` implements `DeleteCaptureFile`. specs.md `:294` and `:405` say "the production build does not compile" and that no delete route exists.
2. `operator/cmd/main.go:439-440` sets the retention fields, which `:289` says are not wired.
3. `sidecar_capture.go:107` and `:156` use `/captures/%s/start` and `/stop`. `:305-306` and `:312` give `:start` and `:stop`.
4. `GetCaptureStatus` returns an error when disabled (`sidecar_capture.go:195-197`). `:311` says all methods return nil.
5. `:138` says ephemeral-container injection is "planned". `:163` lists it as built, and `gameserver_controller.go:1730-1744` implements it.

**Expected:** `operator/specs.md` describes the current code.

**Actual:** Six passages describe a broken build and unwired features that in fact work.

### C-operator-19

**Location:** `operator/specs.md:336-361`; `operator/cmd/main.go:169-180`, `:193-200`, `:224-235`, `:247`.

**Repro / observation**
1. Extract the `flag.*Var` names from `main.go` (31) and the `` | `--…` `` rows from the table (24), then `comm` the two lists.
2. Missing from the table: `--sentinel-image`, `--tunnel-frp-image`, `--tunnel-tailscale-image`, `--tunnel-playit-image`, `--metallb-namespace`, `--capture-default-max-duration-seconds`, `--capture-default-max-size-bytes`. No row lists a flag that does not exist.
3. `main.go:224` defaults `--address-manager` from `GAMEPLANE_ADDRESS_MANAGER`, and `:247` defaults `--game-data-storage-class` from `GAMEPLANE_GAME_DATA_STORAGE_CLASS`. The table mentions neither.

**Expected:** The "CLI flags" table lists every flag and its env default.

**Actual:** 7 of 31 flags are missing, and so are both env defaults.

### C-operator-20

**Location:** `operator/specs.md:5`, `:413-431`; `operator/go.mod:3`, `:10-30`.

**Repro / observation**
1. specs.md says `Go version: 1.25.0`. `go.mod` says `go 1.26.0`.
2. Table vs `go.mod`: `k8s.io/api`, `apimachinery` and `client-go` are v0.35.0 vs v0.37.0. `controller-runtime` is v0.19.0 vs v0.25.1. `go-containerregistry` is v0.20.7 vs v0.22.1. `external-snapshotter/client/v8` is v8.0.0 vs v8.6.0. `prometheus/client_golang` is v1.23.2 vs v1.24.1. `cosign/v2` is v2.6.3 vs v2.6.5. `sigstore` is v1.10.8 vs v1.10.9, now `// indirect`. `x/crypto` is v0.50.0 vs v0.57.0. `x/mod` is v0.35.0 vs v0.41.0. `oras-go/v2` is v2.6.0 vs v2.6.2.
3. `hack/check-doc-versions.sh` checks only Gameplane version strings, so nothing catches this.

**Expected:** The versions match `go.mod`.

**Actual:** 12 dependency rows and the Go line are stale. The Go line is also listed as a related site under C-gameaction-04, so fix both together.

### C-operator-21

**Location:** `operator/specs.md:29`, `:46`, `:53`, `:95`, `:110`, `:113`, `:144`, `:218-219`, `:227`, `:239`, `:464`.

**Repro / observation**
1. `:144` says wipe is "delete PVC, recreate". `gameserver_wipe.go:73-135` runs an `rm` Job against the existing PVC.
2. `:29` and `:464` describe cleanup finalizers and "Backup deletion removes associated Jobs" via finalizers. The only finalizer is on Module (`module_controller.go:68`). Every other cleanup relies on ownerReference garbage collection.
3. `:113` and `:218-219` describe a `Resuming` phase and "wait for GameServer to reach Running". `restore_controller.go:150-181` clears `suspend` and sets `Succeeded` in the same pass. `grep -rn RestorePhaseResuming operator` finds only its declaration (`restore_types.go:16`).
4. `:239` says "Report InstallFailed condition". `grep -rn InstallFailed operator api web/src` finds only `operator/specs.md`. Failures are reported as `Ready=False` with reasons such as `PullFailed`, `SignatureInvalid` and `IncompatibleOperator`.
5. `:227` says "spec.auth". The fields are `spec.oci.pullSecretRef`, `spec.git.secretRef` and `spec.http.secretRef`.
6. `:46`, `:53` and `:95` say "8" kinds or controllers, while `:116` and the code have 9.
7. `:110` says the GameServer "Spec declares desired replica count". `GameServerSpec` has no replicas field.

**Expected:** specs.md matches the code.

**Actual:** At least seven statements contradict it.

### C-operator-22

**Location:** `operator/config/crd/kustomization.yaml:3-10`; `operator/config/samples/gameplane_v1alpha1_gameserver.yaml:9-13`; `operator/config/samples/gameplane_v1alpha1_modulesource.yaml:9-15`; `operator/config/samples/gameplane_v1alpha1_backup.yaml:9-11`; `operator/config/manager/manager.yaml:30-34`.

**Repro / observation**
1. `config/crd/kustomization.yaml` lists 7 of the 9 CRD files in that directory. `gameplane.local_clusters.yaml` and `gameplane.local_networkcaptures.yaml` are missing, so `kubectl apply -k operator/config/crd` leaves the Cluster and NetworkCapture informers unable to sync.
2. The sample GameServer sets `config.TYPE` and `config.VERSION`. The `minecraft-java` `configSchema` (`modules/minecraft-java/template.yaml:555+`) has neither (they are per-version env), so `materializeConfig` fails the server with "unknown config keys" (`gameserver_config.go:96-106`).
3. The sample ModuleSource uses top-level `url` and `modules`. `spec.type` defaults to `oci`, and the CRD rule `self.type != 'oci' || has(self.oci)` rejects the object.
4. The sample Backup sets `repoRef.key: url`, but the operator always reads keys `repo` and `password` (`backup_controller.go:236-239`, `:653-665`). The dashboard also sends `key: "url"` in one place and `key: "repo"` in another (`web/src/routes/tabs/Backups.tsx:46`, `web/src/routes/Backups.tsx:216`). That is harmless because `key` is ignored.
5. `manager.yaml` passes no agent CA or mTLS flags and mounts nothing. `reconcileAgentTLS` then fails on the default `gameplane-system/gameplane-agent-ca` Secret unless it already exists (`agent_certs.go:69-74`).

**Expected:** `operator/specs.md:45-50` presents `config/` as the generated manifests, the manager Deployment and "Sample … CRs for testing". These should apply cleanly against the current CRDs and controllers.

**Actual:** No part of the `config/` dev path works end to end. The Helm chart install does not use these files and is unaffected, hence S4.
