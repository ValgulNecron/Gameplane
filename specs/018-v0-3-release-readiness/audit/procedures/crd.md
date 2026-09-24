# Procedures: CRD

Shared conventions: [conventions.md](conventions.md).

## gameserver-create-from-template

**Preconditions:** GameTemplate `audit018-test-template` exists (create if needed via `kubectl apply`). Admin or operator role.

**Resources created:** GameServer named `audit018-server-create`.

**Steps:**
1. Export session and port-forward (per conventions).
2. Query the template to confirm it exists: `kubectl get gametemplate audit018-test-template -o yaml`.
3. Create GameServer via API: `POST $GP/api/gameservers` with payload naming the template.
   - Login cost: 1 (reuse admin session)
4. Verify creation: `kubectl get gameserver audit018-server-create -o yaml`.

**Expected:** GameServer phase transitions from Pending → Starting → Running within 2 minutes. Agent reports heartbeat in status.agent.lastHeartbeat.

**Cleanup:** `kubectl delete gameserver audit018-server-create -n gameplane-games`.

**Automatable?** yes; bucket: `api-agent`.

---

## gameserver-phase-pending-to-starting

**Preconditions:** GameServer `audit018-server-create` from above still Pending.

**Resources created:** none (observing existing).

**Steps:**
1. Watch pod provisioning: `kubectl get pod -n gameplane-games -l gameserver.gameplane.local/name=audit018-server-create -w`.
2. Check storage status: `kubectl get pvc -n gameplane-games | grep audit018-server-create`.
3. Query GameServer status: `kubectl get gameserver audit018-server-create -o yaml | grep -A 10 status:`.
   - Look for `phase: Starting`.

**Expected:** Pod scheduled, image pull in progress, PVC bound. GameServer.status.phase = Starting.

**Cleanup:** none (keep for next test).

**Automatable?** yes; bucket: `operator`.

---

## gameserver-phase-starting-to-running

**Preconditions:** GameServer in Starting phase from above.

**Resources created:** none (observing existing).

**Steps:**
1. Watch the pod become Ready: `kubectl wait --for=condition=Ready pod/<pod-name> -n gameplane-games --timeout=300s` (find pod name from label selector).
2. Check agent heartbeat: `kubectl get gameserver audit018-server-create -o jsonpath='{.status.agent.lastHeartbeat}'`.
3. Query full GameServer status: `kubectl get gameserver audit018-server-create -o yaml`.
   - Verify `phase: Running` and Ready condition is True.

**Expected:** Pod Ready, agent heartbeats present, phase = Running, conditions show Ready=True and Progressing=True.

**Cleanup:** none (keep for next test).

**Automatable?** yes; bucket: `operator`.

---

## gameserver-suspend

**Preconditions:** GameServer `audit018-server-create` in Running state.

**Resources created:** none (modifying existing).

**Steps:**
1. Issue soft stop (graceful): `kubectl patch gameserver audit018-server-create -p '{"spec":{"suspend":true}}' --type merge -n gameplane-games`.
2. Watch phase transition: `kubectl get gameserver audit018-server-create -w -o jsonpath='{.status.phase}' 2>&1 | grep -E 'Stopping|Stopped|Suspended'`.
3. After ~30s (default grace period), confirm phase: `kubectl get gameserver audit018-server-create -o yaml | grep phase:`.

**Expected:** Phase transitions Stopping → Suspended (or Stopped if no stop sequence). StatefulSet replicas scale to 0. Pod deleted.

**Cleanup:** none (resume in next test).

**Automatable?** yes; bucket: `operator`.

---

## gameserver-unsuspend-wake

**Preconditions:** GameServer suspended from previous step.

**Resources created:** none (modifying existing).

**Steps:**
1. Resume: `kubectl patch gameserver audit018-server-create -p '{"spec":{"suspend":false}}' --type merge -n gameplane-games`.
2. Watch pod creation: `kubectl get pod -n gameplane-games -l gameserver.gameplane.local/name=audit018-server-create -w`.
3. Wait for Running: `kubectl wait --for=condition=Ready pod/<pod-name> -n gameplane-games --timeout=300s`.
4. Confirm phase: `kubectl get gameserver audit018-server-create -o yaml | grep phase:`.

**Expected:** Pod scheduled, StatefulSet replicas scale to 1, phase transitions back to Starting → Running.

**Cleanup:** none (delete in final step).

**Automatable?** yes; bucket: `operator`.

---

## gameserver-restart

**Preconditions:** GameServer running.

**Resources created:** none (modifying existing).

**Steps:**
1. Record the old pod name: `kubectl get pod -n gameplane-games -l gameserver.gameplane.local/name=audit018-server-create -o name`.
2. Restart via API: `POST $GP/api/gameservers/audit018-server-create:restart` (login cost: 1, reuse session).
3. Watch pod deletion and new pod creation: `kubectl get pod -n gameplane-games -l gameserver.gameplane.local/name=audit018-server-create -w`.
4. Confirm new pod is Running: `kubectl wait --for=condition=Ready pod/<new-pod-name> -n gameplane-games --timeout=300s`.

**Expected:** Old pod deleted, new pod scheduled and becomes Ready. StatefulSet ordinal unchanged (same StatefulSet, new pod index).

**Cleanup:** none (delete in final step).

**Automatable?** yes; bucket: `api-agent`.

---

## gameserver-version-switch

**Preconditions:** GameTemplate `audit018-test-template` has at least 2 version entries in `spec.versions[]`. GameServer running.

**Resources created:** none (modifying existing).

**Steps:**
1. List available versions: `kubectl get gametemplate audit018-test-template -o jsonpath='{.spec.versions[*].id}'`.
2. Record current version from GameServer: `kubectl get gameserver audit018-server-create -o jsonpath='{.spec.version}'`.
3. Patch to a different version: `kubectl patch gameserver audit018-server-create -p '{"spec":{"version":"<new-version-id>"}}' --type merge -n gameplane-games`.
4. Watch pod recreation: `kubectl get pod -n gameplane-games -l gameserver.gameplane.local/name=audit018-server-create -w`.
5. Confirm new pod is Running: `kubectl wait --for=condition=Ready pod/<new-pod-name> -n gameplane-games --timeout=300s`.

**Expected:** Pod recreates with new image (version). Spec.version matches new-version-id.

**Cleanup:** none (delete in final step).

**Automatable?** yes; bucket: `operator`.

---

## gameserver-wipe-data-volume

**Preconditions:** GameServer with data volume (PVC). Running or stopped.

**Resources created:** none (modifying existing).

**Steps:**
1. Create a test file on the data volume (attach an init pod or use console):
   - Option A: via console (if available): `POST $GP/api/gameservers/audit018-server-create/console:send` with `echo test > /data/testfile.txt`.
   - Option B: via kubectl exec into a debug pod attached to the PVC.
2. Verify file exists: `ls /data/testfile.txt` (via console or debug pod).
3. Trigger wipe: `PATCH /api/gameservers/audit018-server-create:wipe` (login cost: 1).
4. Watch pod scale-down and back up: `kubectl get pod -n gameplane-games -l gameserver.gameplane.local/name=audit018-server-create -w`.
5. Check that file is gone: try to read it again via console or debug pod.

**Expected:** Data volume cleared. Testfile gone. Pod restarts cleanly.

**Cleanup:** none (delete in final step).

**Automatable?** yes; bucket: `api-agent`.

---

## gameserver-idle-auto-sleep

**Preconditions:** GameServer with `spec.idle.enabled=true` and `spec.idle.afterMinutes=1`. Running. No players online.

**Resources created:** none (modifying existing).

**Steps:**
1. Ensure players are 0 or unknown: check `kubectl get gameserver audit018-server-create -o jsonpath='{.status.agent.playersOnline}'`.
2. Wait for idle threshold: watch `kubectl get gameserver audit018-server-create -w -o jsonpath='{.status.idle.asleep}'` for 2+ minutes.
3. Verify phase transitions to Suspended: `kubectl get gameserver audit018-server-create -o jsonpath='{.status.phase}'`.
4. Confirm pod scaled down: `kubectl get pod -n gameplane-games -l gameserver.gameplane.local/name=audit018-server-create | grep -i no\ resources`.

**Expected:** status.idle.asleep = true, phase = Suspended, pod deleted, status.idle.emptySince reflects the time idle started.

**Cleanup:** none (clean up manually or set idle.enabled=false).

**Automatable?** yes; bucket: `operator`.

---

## gameserver-idle-wake-window

**Preconditions:** GameServer sleeping from above. Spec has `spec.idle.wakeWindows` with a cron entry that fires within the next minute.

**Resources created:** none (modifying existing).

**Steps:**
1. Set a wake window cron (e.g., `* * * * *` for every minute): `kubectl patch gameserver audit018-server-create --type merge -p '{"spec":{"idle":{"wakeWindows":["* * * * *"]}}}'`.
2. Wait for the next minute boundary: observe via `kubectl get gameserver audit018-server-create -w -o jsonpath='{.status.idle.asleep}'`.
3. Within 2 minutes, phase should transition back to Starting/Running.
4. Confirm pod is created: `kubectl get pod -n gameplane-games -l gameserver.gameplane.local/name=audit018-server-create`.

**Expected:** Pod scheduled, phase = Starting → Running, status.idle.asleep = false, status.idle.lastWakeTime updated.

**Cleanup:** disable idle or delete server.

**Automatable?** yes; bucket: `operator`.

---

## gameserver-idle-wake-on-connect

**Preconditions:** GameServer with `spec.idle.wakeOnConnect=true` and sleeping. Sentinel pod must be injected.

**Resources created:** Sentinel pod (auto-created by operator).

**Steps:**
1. Locate the sentinel pod: `kubectl get pod -n gameplane-games -l gameserver.gameplane.local/name=audit018-server-create,gameplane.local/role=sentinel`.
2. From outside the cluster, attempt a TCP connection to the game port (via NodePort or tunnel). This simulates a player joining.
3. Watch sentinel detect the connection and trigger wake (may trigger via webhook or controller event).
4. Confirm game pod is created: `kubectl get pod -n gameplane-games -l gameserver.gameplane.local/name=audit018-server-create | grep -v sentinel`.
5. Verify phase: `kubectl get gameserver audit018-server-create -o jsonpath='{.status.phase}'`.

**Expected:** Sentinel pod exists while asleep. On connection, game pod is created, phase transitions to Starting → Running.

**Cleanup:** disable wake-on-connect or delete server.

**Automatable?** no (requires external client connection to sentinel port).

---

## gameserver-delete-with-finalizer

**Preconditions:** GameServer exists (may be running or stopped).

**Resources created:** none (deleting existing).

**Steps:**
1. Initiate delete: `kubectl delete gameserver audit018-server-create -n gameplane-games --grace-period=30`.
2. Watch deletion: `kubectl get gameserver audit018-server-create -w -o yaml` (may hang if finalizer is present).
3. Observe StatefulSet and PVC cleanup: `kubectl get sts,pvc -n gameplane-games | grep audit018-server-create`.
4. If hung, verify finalizers are present: `kubectl get gameserver audit018-server-create -o jsonpath='{.metadata.finalizers}'`.
5. Once finalizers clear, GameServer should be deleted.

**Expected:** Finalizers present during deletion (e.g., `gameplane.local/gameserver-finalizer`). StatefulSet, Service, PVC deleted. GameServer eventually absent.

**Cleanup:** none (object deleted).

**Automatable?** yes; bucket: `operator`.

---

## gameserver-failed-phase-crash-loop

**Preconditions:** GameServer with an image that fails to start (e.g., a bad entrypoint or crash immediately).

**Resources created:** GameServer named `audit018-server-failed`.

**Steps:**
1. Create a GameServer using a broken image: `kubectl apply -f audit018-broken-gs.yaml` (Create the YAML manually or via API with bad image).
2. Watch pod crash: `kubectl get pod -n gameplane-games -l gameserver.gameplane.local/name=audit018-server-failed -w`.
3. Allow ~60s for Kubelet to detect crash loop.
4. Check GameServer status: `kubectl get gameserver audit018-server-failed -o yaml | grep -A 10 status:`.
5. Verify phase = Failed: `kubectl get gameserver audit018-server-failed -o jsonpath='{.status.phase}'`.

**Expected:** Pod crashes with non-zero exit. CrashLoopBackOff observed. GameServer phase escalates to Failed. Provisioning condition shows the crash reason.

**Cleanup:** `kubectl delete gameserver audit018-server-failed -n gameplane-games`.

**Automatable?** yes; bucket: `operator`.

---

## gametemplate-create

**Preconditions:** User or admin role. Template YAML prepared or generated from module.

**Resources created:** GameTemplate named `audit018-test-template`.

**Steps:**
1. Define a template (or use one from a module install): Create `audit018-template.yaml` with `kind: GameTemplate`, a game identifier, image, ports, etc.
2. Apply: `kubectl apply -f audit018-template.yaml -l gameplane.io/audit=018`.
3. List templates: `kubectl get gametemplate audit018-test-template -o yaml`.
4. Verify fields are persisted: displayName, game, image, ports, etc.

**Expected:** GameTemplate exists in apiserver. Can be referenced by GameServers via templateRef.

**Cleanup:** `kubectl delete gametemplate audit018-test-template`.

**Automatable?** yes; bucket: `operator`.

---

## backup-create-and-run

**Preconditions:** GameServer `audit018-server-create` running. Restic repo Secret exists in namespace (e.g., `audit018-repo-secret`) OR CSI VolumeSnapshotClass available for volume-snapshot strategy.

**Resources created:** Backup named `audit018-backup-1`.

**Steps:**
1. Create Backup: `kubectl apply -f audit018-backup.yaml` where spec.serverRef.name = audit018-server-create, spec.repoRef points to the repo Secret (or omit for volume-snapshot).
2. Watch phase: `kubectl get backup audit018-backup-1 -w -o jsonpath='{.status.phase}'`.
3. Confirm phase transitions: Pending → Running.
4. Wait for completion: `kubectl wait --for=jsonpath='{.status.phase}'=Succeeded backup/audit018-backup-1 --timeout=600s`.
5. Record completion time: `kubectl get backup audit018-backup-1 -o jsonpath='{.status.completionTime}'`.

**Expected:** Backup phase = Running. Backup Job pod created and runs restic or triggers CSI snapshot. On success, phase = Succeeded, status.completionTime set, status.snapshotID populated.

**Cleanup:** `kubectl delete backup audit018-backup-1 -n gameplane-games`.

**Automatable?** yes; bucket: `api-mods`.

---

## backup-failure-and-phase

**Preconditions:** Restic repo Secret with wrong credentials, OR CSI snapshot fails.

**Resources created:** Backup named `audit018-backup-fail`.

**Steps:**
1. Create Backup with bad repo credentials: `kubectl apply -f audit018-backup-bad-repo.yaml`.
2. Watch phase: `kubectl get backup audit018-backup-fail -w -o jsonpath='{.status.phase}'`.
3. Observe Job failure: `kubectl get job -n gameplane-games | grep audit018-backup-fail`.
4. Check backup status message: `kubectl get backup audit018-backup-fail -o jsonpath='{.status.message}'`.

**Expected:** Backup phase transitions Pending → Running → Failed. Job Pod exits non-zero. status.message explains the error.

**Cleanup:** `kubectl delete backup audit018-backup-fail -n gameplane-games`.

**Automatable?** yes; bucket: `api-mods`.

---

## restore-create-and-suspend-server

**Preconditions:** Backup `audit018-backup-1` in Succeeded state. Target GameServer `audit018-restore-target` exists (or will be created for volume-snapshot restore).

**Resources created:** Restore named `audit018-restore-1`.

**Steps:**
1. Create Restore: `kubectl apply -f audit018-restore.yaml` where spec.backupRef.name = audit018-backup-1, spec.serverRef.name = audit018-restore-target.
2. Watch phase: `kubectl get restore audit018-restore-1 -w -o jsonpath='{.status.phase}'`.
3. Verify target server is suspended: `kubectl get gameserver audit018-restore-target -o jsonpath='{.status.phase}'` should be Suspended or Stopping.
4. Observe Restore phases: Pending → Suspending → Running.

**Expected:** Phase transitions correctly. Target GameServer scales to 0 (suspended). Restore Job created. PVC mount point prepared.

**Cleanup:** none (observe Running phase for now).

**Automatable?** yes; bucket: `api-mods`.

---

## restore-complete-and-resume

**Preconditions:** Restore `audit018-restore-1` in Running phase from above.

**Resources created:** none (completing existing).

**Steps:**
1. Wait for restore Job completion: `kubectl wait --for=jsonpath='{.status.phase}'=Succeeded restore/audit018-restore-1 --timeout=600s` (or Failed if an error).
2. If Succeeded, phase transitions to Resuming.
3. Watch target GameServer come back up: `kubectl get gameserver audit018-restore-target -w -o jsonpath='{.status.phase}'` should transition to Starting → Running.
4. Verify data was restored: Check target pod's data volume for restored content (e.g., a marker file created before backup).

**Expected:** Restore phase = Succeeded or Failed (check condition for details). Target server resumesat Running. Pod recreated with restored data.

**Cleanup:** `kubectl delete restore audit018-restore-1 -n gameplane-games`.

**Automatable?** yes; bucket: `api-mods`.

---

## backup-schedule-create

**Preconditions:** GameServer running. User/admin role.

**Resources created:** BackupSchedule named `audit018-schedule-1`.

**Steps:**
1. Create BackupSchedule: `kubectl apply -f audit018-schedule.yaml` where spec.serverRef.name = audit018-server-create, spec.schedule = "*/5 * * * *" (every 5 minutes).
2. List schedules: `kubectl get backupschedule audit018-schedule-1 -o yaml`.
3. Record status.nextScheduleTime: `kubectl get backupschedule audit018-schedule-1 -o jsonpath='{.status.nextScheduleTime}'`.

**Expected:** BackupSchedule exists. nextScheduleTime is within 5 minutes. No Backup objects yet (wait for first tick).

**Cleanup:** `kubectl delete backupschedule audit018-schedule-1 -n gameplane-games`.

**Automatable?** yes; bucket: `api-mods`.

---

## backup-schedule-tick-creates-backup

**Preconditions:** BackupSchedule `audit018-schedule-1` with schedule firing within the next 5 minutes.

**Resources created:** Backup (auto-created by controller).

**Steps:**
1. Wait for schedule tick (up to 5 min): watch `kubectl get backup -n gameplane-games | grep audit018-schedule-1`.
2. Once Backup created, observe its phase: `kubectl get backup -l backupschedule=audit018-schedule-1 -w -o jsonpath='{.status.phase}'`.
3. Wait for completion: `kubectl wait --for=jsonpath='{.status.phase}'=Succeeded backup/<name> --timeout=600s`.
4. Update BackupSchedule status: `kubectl get backupschedule audit018-schedule-1 -o jsonpath='{.status.lastSuccessfulTime}'` should be recent.

**Expected:** On schedule tick, a new Backup is created owned by the BackupSchedule. Backup runs and succeeds. lastSuccessfulTime updated.

**Cleanup:** none (delete schedule in final step).

**Automatable?** yes; bucket: `api-mods`.

---

## backup-schedule-retention-prunes

**Preconditions:** BackupSchedule with retention policy (e.g., `retention.keepLast=2`). Multiple Backups already exist.

**Resources created:** none (observing deletion).

**Steps:**
1. List all Backups for the schedule: `kubectl get backup -l backupschedule=audit018-schedule-1 -o name | wc -l`.
2. Note older Backup names.
3. Wait for schedule tick and new Backup creation.
4. Immediately check that old Backups are deleted: `kubectl get backup -l backupschedule=audit018-schedule-1 | tail -2` (only the 2 most recent remain).

**Expected:** When retention.keepLast=2, only 2 Backups remain. Older ones are deleted by the controller on the next schedule tick.

**Cleanup:** delete BackupSchedule.

**Automatable?** yes; bucket: `api-mods`.

---

## module-create-pending

**Preconditions:** ModuleSource `default` or `uploads` exists and indexed. Module bundle exists in the source.

**Resources created:** Module named `audit018-module-1`.

**Steps:**
1. Create Module: `kubectl apply -f audit018-module.yaml` where spec.source.name = default, spec.name = minecraft (or another existing module name in the source).
2. Check phase: `kubectl get module audit018-module-1 -o jsonpath='{.status.phase}'` should be Pending or Pulling.
3. Record appliedVersion: `kubectl get module audit018-module-1 -o jsonpath='{.status.appliedVersion}'` (may be empty while Pending).

**Expected:** Module created. Phase = Pending or Pulling. Controller is resolving the version.

**Cleanup:** none (observe next step).

**Automatable?** yes; bucket: `api-mods`.

---

## module-pulling-to-ready

**Preconditions:** Module `audit018-module-1` in Pulling phase from above.

**Resources created:** none (observing existing).

**Steps:**
1. Watch phase: `kubectl get module audit018-module-1 -w -o jsonpath='{.status.phase}'`.
2. Wait for phase = Ready: `kubectl wait --for=jsonpath='{.status.phase}'=Ready module/audit018-module-1 --timeout=300s`.
3. Verify GameTemplate created: `kubectl get gametemplate audit018-module-1 -o yaml` (named same as Module).
4. Check labels: `kubectl get gametemplate audit018-module-1 -o jsonpath='{.metadata.labels.gameplane\.local/managed-by}'` should be "Module".

**Expected:** Phase transitions Pulling → Ready. GameTemplate created with labels indicating Module ownership. appliedVersion and appliedDigest recorded.

**Cleanup:** none (delete Module in final step).

**Automatable?** yes; bucket: `api-mods`.

---

## module-bad-signature-fails

**Preconditions:** ModuleSource with `spec.verify` configured. OCI source with unsigned or incorrectly signed module bundle.

**Resources created:** Module named `audit018-module-bad-sig`.

**Steps:**
1. Create Module referencing an unsigned bundle: `kubectl apply -f audit018-module-unsigned.yaml`.
2. Watch phase: `kubectl get module audit018-module-bad-sig -w -o jsonpath='{.status.phase}'`.
3. Phase should transition to Failed: `kubectl wait --for=jsonpath='{.status.phase}'=Failed module/audit018-module-bad-sig --timeout=300s`.
4. Check lastError: `kubectl get module audit018-module-bad-sig -o jsonpath='{.status.lastError}'` should mention signature verification failure.

**Expected:** Phase = Failed. lastError explains signature mismatch or verification failure. No GameTemplate created.

**Cleanup:** `kubectl delete module audit018-module-bad-sig`.

**Automatable?** no (requires external bundle management; defer to T026).

---

## modulesource-oci-sync

**Preconditions:** ModuleSource of type `oci` (e.g., `default` if it is OCI-based). OCI registry accessible.

**Resources created:** none (observing existing source).

**Steps:**
1. Get ModuleSource status: `kubectl get modulesource default -o yaml | grep -A 20 status:`.
2. Record lastRefreshTime: `kubectl get modulesource default -o jsonpath='{.status.lastRefreshTime}'`.
3. Wait for next refresh (default 1h, or trigger manually): `kubectl annotate modulesource default gameplane.local/force-refresh=true --overwrite`.
4. Watch for refresh: `kubectl get modulesource default -w -o jsonpath='{.status.lastRefreshTime}'` (should update).
5. List discovered modules: `kubectl get modulesource default -o jsonpath='{.status.modules[*].name}'`.

**Expected:** ModuleSource periodically syncs with OCI registry. status.lastRefreshTime updates. status.modules contains discovered module names.

**Cleanup:** none (leave source intact).

**Automatable?** yes; bucket: `api-mods`.

---

## modulesource-git-sync-error

**Preconditions:** ModuleSource of type `git` with invalid repo URL or broken clone.

**Resources created:** none (observing existing source).

**Steps:**
1. Get ModuleSource status: `kubectl get modulesource <git-source> -o yaml | grep -A 30 status:`.
2. Check conditions for errors: `kubectl get modulesource <git-source> -o jsonpath='{.status.conditions[*]}'`.
3. Record message: `kubectl get modulesource <git-source> -o jsonpath='{.status.message}'`.

**Expected:** If git clone fails, status.message explains the error (e.g., "repo not found", "authentication failed"). Phase indicates unhealthy state (if implemented).

**Cleanup:** none (observing existing source).

**Automatable?** yes; bucket: `api-mods`.

---

## cluster-register-and-health-check

**Preconditions:** User/admin role. Target remote Kubernetes cluster kubeconfig accessible (e.g., from a Secret or file).

**Resources created:** Cluster named `audit018-cluster-1`. Secret holding kubeconfig.

**Steps:**
1. Create Secret with kubeconfig: `kubectl apply -f audit018-kubeconfig-secret.yaml` (must have label `gameplane.local/cluster-kubeconfig=true`).
2. Create Cluster: `kubectl apply -f audit018-cluster.yaml` where spec.kubeconfigSecret.name = audit018-kubeconfig-secret.
3. Watch phase: `kubectl get cluster audit018-cluster-1 -w -o jsonpath='{.status.phase}'`.
4. Phase should transition: Unknown → Healthy (if reachable) or Unknown → Unhealthy (if not).
5. Check serverVersion: `kubectl get cluster audit018-cluster-1 -o jsonpath='{.status.serverVersion}'`.

**Expected:** Cluster phase = Healthy or Unhealthy. If Healthy, serverVersion is populated. Conditions show health check result.

**Cleanup:** `kubectl delete cluster audit018-cluster-1` and delete Secret.

**Automatable?** yes; bucket: `multicluster`.

---

## cluster-health-check-unreachable

**Preconditions:** Cluster registered from above with a kubeconfig pointing to an unreachable or stopped remote cluster.

**Resources created:** none (observing existing).

**Steps:**
1. Ensure the target cluster is unreachable (e.g., stop kube-apiserver or firewall the connection).
2. Watch Cluster status: `kubectl get cluster audit018-cluster-1 -w -o yaml | grep -A 20 status:`.
3. Within the next reconciliation (typically <30s), phase should become Unhealthy.
4. Check message: `kubectl get cluster audit018-cluster-1 -o jsonpath='{.status.message}'`.

**Expected:** Phase = Unhealthy. message explains the failure (e.g., "connection refused", "timeout"). serverVersion may be stale.

**Cleanup:** none (observe for now).

**Automatable?** yes; bucket: `multicluster`.

---

## networkcapture-create-pending

**Preconditions:** GameServer `audit018-server-create` running with `spec.capture.enabled=true`. Capture sidecar injected and Ready.

**Resources created:** NetworkCapture named `audit018-capture-1`.

**Steps:**
1. Verify capture sidecar is ready: `kubectl get pod -n gameplane-games <server-pod> -o jsonpath='{.status.ephemeralContainers[*].name}'` should include capture sidecar.
2. Create NetworkCapture: `kubectl apply -f audit018-capture.yaml` where spec.serverRef.name = audit018-server-create, spec.maxDuration = 5m, spec.maxSize = 100Mi.
3. Check phase: `kubectl get networkcapture audit018-capture-1 -o jsonpath='{.status.phase}'` should be Pending.
4. Record creationTimestamp: `kubectl get networkcapture audit018-capture-1 -o jsonpath='{.metadata.creationTimestamp}'`.

**Expected:** NetworkCapture phase = Pending. status.startTime empty (capture not yet started).

**Cleanup:** none (observe next step).

**Automatable?** yes; bucket: `operator`.

---

## networkcapture-start-running

**Preconditions:** NetworkCapture `audit018-capture-1` in Pending state from above.

**Resources created:** none (observing existing).

**Steps:**
1. Watch phase: `kubectl get networkcapture audit018-capture-1 -w -o jsonpath='{.status.phase}'`.
2. Wait for phase = Running: `kubectl wait --for=jsonpath='{.status.phase}'=Running networkcapture/audit018-capture-1 --timeout=30s`.
3. Record startTime: `kubectl get networkcapture audit018-capture-1 -o jsonpath='{.status.startTime}'`.
4. Observe packets captured: `kubectl get networkcapture audit018-capture-1 -o jsonpath='{.status.packetsWritten}'`.

**Expected:** Phase transitions Pending → Running. startTime populated. Packets and bytes counters start incrementing (if traffic is flowing).

**Cleanup:** none (stop capture in next step).

**Automatable?** yes; bucket: `operator`.

---

## networkcapture-stop-completed

**Preconditions:** NetworkCapture `audit018-capture-1` in Running state from above.

**Resources created:** none (stopping existing).

**Steps:**
1. Stop capture manually: `kubectl patch networkcapture audit018-capture-1 --type merge -p '{"spec":{"maxDuration":"1s"}}'` (force timeout immediately).
2. Or wait for maxDuration to elapse (e.g., 5m from creation).
3. Watch phase: `kubectl get networkcapture audit018-capture-1 -w -o jsonpath='{.status.phase}'`.
4. Phase should transition to Completed: `kubectl wait --for=jsonpath='{.status.phase}'=Completed networkcapture/audit018-capture-1 --timeout=30s`.
5. Record completionTime and final stats: `kubectl get networkcapture audit018-capture-1 -o yaml | grep -E 'completionTime|packetsWritten|bytesWritten'`.

**Expected:** Phase = Completed. completionTime populated. packetsWritten and bytesWritten show final counts.

**Cleanup:** none (capture will auto-delete after TTL).

**Automatable?** yes; bucket: `operator`.

---

## networkcapture-failed-sidecar-crash

**Preconditions:** GameServer with capture enabled. Capture sidecar container intentionally crashed or made unavailable.

**Resources created:** NetworkCapture named `audit018-capture-fail`.

**Steps:**
1. Start a capture: `kubectl apply -f audit018-capture-fail.yaml`.
2. Immediately crash the sidecar container: `kubectl exec -it <pod> -c capture -- kill 1` (forces container restart, but ephemeral containers cannot restart in Kubernetes, so it terminates).
3. Watch capture phase: `kubectl get networkcapture audit018-capture-fail -w -o jsonpath='{.status.phase}'`.
4. Phase should transition to Failed within ~30s.
5. Check message: `kubectl get networkcapture audit018-capture-fail -o jsonpath='{.status.message}'`.

**Expected:** Phase = Failed. message explains sidecar crash or connection loss.

**Cleanup:** none (will auto-expire).

**Automatable?** yes; bucket: `operator`.

---

## networkcapture-expired-auto-delete

**Preconditions:** Completed NetworkCapture with short TTL (e.g., TTLSecondsAfterFinished=60).

**Resources created:** none (observing deletion).

**Steps:**
1. Wait for TTL to elapse: monitor `kubectl get networkcapture audit018-capture-1 -w`.
2. After TTLSecondsAfterFinished seconds pass since completionTime, the operator should delete the NetworkCapture.
3. Verify deletion: `kubectl get networkcapture audit018-capture-1` (should not exist, or phase = Expired if recorded before deletion).

**Expected:** After TTL elapses, NetworkCapture is deleted by operator. Phase may show Expired briefly.

**Cleanup:** none (object auto-deleted).

**Automatable?** yes; bucket: `operator`.

---

### backup-volumesnapshot-strategy

**Preconditions:** GameServer `audit018-server-create` running. Cluster CSI driver supports VolumeSnapshot (a default VolumeSnapshotClass exists, or one is named via `spec.volumeSnapshotClassName`).

**Resources created:** Backup named `audit018-backup-vs` with `spec.strategy: volume-snapshot` (no `repoRef`).

**Steps:**
1. Create Backup: `kubectl apply -f audit018-backup-vs.yaml` with `spec.serverRef.name: audit018-server-create`, `spec.strategy: volume-snapshot`, `spec.repoRef` omitted.
2. Watch phase: `kubectl get backup audit018-backup-vs -w -o jsonpath='{.status.phase}'`.
3. Confirm a VolumeSnapshot is created: `kubectl get volumesnapshot -n gameplane-games | grep audit018-backup-vs`.
4. Wait for completion: `kubectl wait --for=jsonpath='{.status.phase}'=Succeeded backup/audit018-backup-vs --timeout=600s`.
5. Record the bound snapshot: `kubectl get backup audit018-backup-vs -o jsonpath='{.status.volumeSnapshotContentName}'`.

**Expected:** Backup skips restic entirely; a VolumeSnapshot of the server's data PVC is created, quiesce/unquiesce brackets it the same way the restic path does, and once the VolumeSnapshot reports readyToUse the Backup phase reaches Succeeded with `status.volumeSnapshotContentName` set.

**Cleanup:** `kubectl delete backup audit018-backup-vs -n gameplane-games` (the owned VolumeSnapshot is garbage-collected with it).

**Automatable?** yes; bucket: `api-mods`.

---

### restore-volumesnapshot-strategy

**Preconditions:** A Succeeded `volume-snapshot`-strategy Backup exists (`audit018-backup-vs` from `backup-volumesnapshot-strategy`).

**Resources created:** Restore named `audit018-restore-vs`, which provisions a new GameServer from the snapshot.

**Steps:**
1. Create Restore: `kubectl apply -f audit018-restore-vs.yaml` with `spec.backupRef.name: audit018-backup-vs`, targeting a new server name.
2. Watch phase: `kubectl get restore audit018-restore-vs -w -o jsonpath='{.status.phase}'`.
3. Confirm the new GameServer's PVC is provisioned with the VolumeSnapshot as its `dataSource`: `kubectl get pvc -n gameplane-games <new-server>-data -o jsonpath='{.spec.dataSource}'`.
4. Wait for completion: `kubectl wait --for=jsonpath='{.status.phase}'=Succeeded restore/audit018-restore-vs --timeout=600s`.
5. Re-run step 1 targeting an already-existing GameServer name and confirm it is rejected.

**Expected:** Restore reconstructs a new PVC using the bound VolumeSnapshot as `dataSource`, the target GameServer reaches Running, and re-targeting an existing GameServer name is rejected rather than overwritten.

**Cleanup:** `kubectl delete restore audit018-restore-vs -n gameplane-games`; delete the provisioned GameServer.

**Automatable?** yes; bucket: `api-mods`.

