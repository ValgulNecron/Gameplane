# Procedures: CRD

Shared conventions: [conventions.md](conventions.md).

API steps reuse the `audit018-admin` session (login cost 0 once it exists): set `SESS` and `CSRF` from `~/gameplane-audit-018/session-admin.txt` as in the Sessions convention. Namespaced objects (GameServer, Backup, BackupSchedule, Restore, NetworkCapture, pods, PVCs, Jobs) live in `gameplane-games`; GameTemplate, Module, ModuleSource and Cluster are cluster-scoped.

Evidence: save the final `kubectl get … -o yaml` of every object a procedure creates or observes, and every API response, under `specs/018-v0-3-release-readiness/audit/evidence/<INV-ID>/` using the procedure's `INV-CRD-…` ID from `audit/inventory.md` (for example `kubectl get gameserver audit018-server-create -n gameplane-games -o yaml > specs/018-v0-3-release-readiness/audit/evidence/INV-CRD-001/gameserver.yaml`). Never save a Secret or a kubeconfig.

### gameserver-create-from-template

**Preconditions:** GameTemplate `audit018-test-template` exists (run `gametemplate-create` first). Admin or operator role.

**Resources created:** GameServer named `audit018-server-create`.

**Steps:**
1. Export session and port-forward (per conventions).
2. Query the template to confirm it exists: `kubectl get gametemplate audit018-test-template -o yaml`.
3. Create GameServer via API: `curl -s -w '\nHTTP %{http_code}\n' -X POST -H "Cookie: gameplane_session=$SESS; gameplane_csrf=$CSRF" -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-server-create","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"audit018-test-template"}}}' "$GP/servers?namespace=gameplane-games"` (expect HTTP 201).
   - Login cost: 0 (reuse admin session)
4. Verify creation: `kubectl get gameserver audit018-server-create -n gameplane-games -o yaml`.

**Expected:** GameServer phase transitions from Pending → Starting → Running within 2 minutes. Agent reports heartbeat in status.agent.lastHeartbeat.

**Cleanup:** none (used by the procedures below; deleted by `gameserver-delete-with-finalizer`).

**Automatable?** yes; bucket: `api-agent`.

---

### gameserver-phase-pending-to-starting

**Preconditions:** GameServer `audit018-server-create` just created by `gameserver-create-from-template` (run these steps right after its step 3; Pending and Starting last only until the pod is Ready).

**Resources created:** none (observing existing).

**Steps:**
1. Watch pod provisioning: `kubectl get pod -n gameplane-games -l app.kubernetes.io/name=gameplane-game,app.kubernetes.io/instance=audit018-server-create -w`.
2. Check storage status: `kubectl get pvc -n gameplane-games | grep audit018-server-create`.
3. Query GameServer status: `kubectl get gameserver audit018-server-create -n gameplane-games -o jsonpath='{.status.phase}{"\n"}{.status.conditions}{"\n"}'`.
   - Look for `phase: Starting`.

**Expected:** Pod scheduled, image pull in progress, PVC bound. GameServer.status.phase = Starting.

**Cleanup:** none (keep for next test).

**Automatable?** yes; bucket: `operator`.

---

### gameserver-phase-starting-to-running

**Preconditions:** GameServer `audit018-server-create` in Starting phase from above.

**Resources created:** none (observing existing).

**Steps:**
1. Watch the pod become Ready: `kubectl wait --for=condition=Ready pod/audit018-server-create-0 -n gameplane-games --timeout=300s`.
2. Check agent heartbeat: `kubectl get gameserver audit018-server-create -n gameplane-games -o jsonpath='{.status.agent.lastHeartbeat}'`.
3. Query full GameServer status: `kubectl get gameserver audit018-server-create -n gameplane-games -o yaml`.
   - Verify `phase: Running` and Ready condition is True.

**Expected:** Pod Ready, agent heartbeats present, phase = Running, conditions show Ready=True, Progressing=False (reason Stable) and Healthy=True.

**Cleanup:** none (keep for next test).

**Automatable?** yes; bucket: `operator`.

---

### gameserver-suspend

**Preconditions:** GameServer `audit018-server-create` in Running state.

**Resources created:** none (modifying existing).

**Steps:**
1. Issue soft stop (graceful): `kubectl patch gameserver audit018-server-create -p '{"spec":{"suspend":true}}' --type merge -n gameplane-games`.
2. Watch phase transition (Ctrl-C once Suspended): `kubectl get gameserver audit018-server-create -n gameplane-games -w -o jsonpath='{.status.phase}{"\n"}'`.
3. After ~30s (default grace period), confirm phase: `kubectl get gameserver audit018-server-create -n gameplane-games -o yaml | grep phase:`.

**Expected:** Phase transitions Stopping → Suspended (the operator never reports Stopped for a suspend). StatefulSet replicas scale to 0. Pod deleted.

**Cleanup:** none (resume in next test).

**Automatable?** yes; bucket: `operator`.

---

### gameserver-unsuspend-wake

**Preconditions:** GameServer suspended from previous step.

**Resources created:** none (modifying existing).

**Steps:**
1. Resume: `kubectl patch gameserver audit018-server-create -p '{"spec":{"suspend":false}}' --type merge -n gameplane-games`.
2. Watch pod creation: `kubectl get pod -n gameplane-games -l app.kubernetes.io/name=gameplane-game,app.kubernetes.io/instance=audit018-server-create -w`.
3. Wait for Running: `kubectl wait --for=condition=Ready pod/audit018-server-create-0 -n gameplane-games --timeout=300s`.
4. Confirm phase: `kubectl get gameserver audit018-server-create -n gameplane-games -o yaml | grep phase:`.

**Expected:** Pod scheduled, StatefulSet replicas scale to 1, phase transitions back to Starting → Running.

**Cleanup:** none (delete in final step).

**Automatable?** yes; bucket: `operator`.

---

### gameserver-restart

**Preconditions:** GameServer `audit018-server-create` running.

**Resources created:** none (modifying existing).

**Steps:**
1. Record the old pod UID (the pod name `audit018-server-create-0` does not change): `kubectl get pod audit018-server-create-0 -n gameplane-games -o jsonpath='{.metadata.uid}'`.
2. Restart via API: `curl -s -o /dev/null -w '%{http_code}\n' -X POST -H "Cookie: gameplane_session=$SESS; gameplane_csrf=$CSRF" -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-server-create:restart?namespace=gameplane-games"` (expect 202; login cost: 0, reuse session).
3. Watch pod deletion and new pod creation: `kubectl get pod -n gameplane-games -l app.kubernetes.io/name=gameplane-game,app.kubernetes.io/instance=audit018-server-create -w`.
4. Confirm new pod is Running: `kubectl wait --for=condition=Ready pod/audit018-server-create-0 -n gameplane-games --timeout=300s`, then re-run step 1 and compare the UID.

**Expected:** Old pod deleted, new pod scheduled and becomes Ready. Same StatefulSet and pod name `audit018-server-create-0`, new pod UID. Annotation `gameplane.local/restart-completed` equals `gameplane.local/restart-requested`.

**Cleanup:** none (delete in final step).

**Automatable?** yes; bucket: `api-agent`.

---

### gameserver-version-switch

**Preconditions:** GameTemplate `audit018-test-template` has at least 2 version entries in `spec.versions[]` (the `minecraft-java` copy from `gametemplate-create` does). GameServer `audit018-server-create` running.

**Resources created:** none (modifying existing).

**Steps:**
1. List available versions: `kubectl get gametemplate audit018-test-template -o jsonpath='{.spec.versions[*].id}'`.
2. Record current version from GameServer (empty means the template's default version): `kubectl get gameserver audit018-server-create -n gameplane-games -o jsonpath='{.spec.version}'`.
3. Patch to a different version: `kubectl patch gameserver audit018-server-create -p '{"spec":{"version":"<new-version-id>"}}' --type merge -n gameplane-games`.
4. Watch pod recreation: `kubectl get pod -n gameplane-games -l app.kubernetes.io/name=gameplane-game,app.kubernetes.io/instance=audit018-server-create -w`.
5. Confirm new pod is Running: `kubectl wait --for=condition=Ready pod/audit018-server-create-0 -n gameplane-games --timeout=300s`.

**Expected:** Pod recreates with new image (version). Spec.version matches new-version-id.

**Cleanup:** none (delete in final step).

**Automatable?** yes; bucket: `operator`.

---

### gameserver-wipe-data-volume

**Preconditions:** GameServer `audit018-server-create` running (the marker file is written through its agent).

**Resources created:** none (modifying existing).

**Steps:**
1. Create a marker file on the data volume through the files API: `curl -s -o /dev/null -w '%{http_code}\n' -X POST -H "Cookie: gameplane_session=$SESS; gameplane_csrf=$CSRF" -H "X-Gameplane-CSRF: $CSRF" --data-binary "audit018" "$GP/servers/audit018-server-create/files/write?path=/audit018-wipe-marker.txt"` (expect 204).
2. Verify file exists: `curl -s -H "Cookie: gameplane_session=$SESS; gameplane_csrf=$CSRF" "$GP/servers/audit018-server-create/files/read?path=/audit018-wipe-marker.txt"`.
3. Trigger wipe: `curl -s -o /dev/null -w '%{http_code}\n' -X POST -H "Cookie: gameplane_session=$SESS; gameplane_csrf=$CSRF" -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" -d '{"confirm":"audit018-server-create"}' "$GP/servers/audit018-server-create:wipe-data?namespace=gameplane-games"` (expect 202; login cost: 0).
4. Watch the server suspend and the wipe Job `audit018-server-create-wipe` run (it is created only once the server is Suspended): `kubectl get job -n gameplane-games -w`, then confirm `gameplane.local/wipe-data-completed` equals `gameplane.local/wipe-data-requested`: `kubectl get gameserver audit018-server-create -n gameplane-games -o jsonpath='{.metadata.annotations}'`.
5. The wipe leaves the server suspended. Start it: `curl -s -o /dev/null -w '%{http_code}\n' -X POST -H "Cookie: gameplane_session=$SESS; gameplane_csrf=$CSRF" -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-server-create:start?namespace=gameplane-games"`, then `kubectl wait gameserver/audit018-server-create -n gameplane-games --for=jsonpath='{.status.phase}'=Running --timeout=600s`.
6. Check that file is gone: re-run step 2.

**Expected:** Data volume cleared. Marker file gone. Pod restarts cleanly after the explicit start.

**Cleanup:** none (delete in final step).

**Automatable?** yes; bucket: `api-agent`.

---

### gameserver-idle-auto-sleep

**Preconditions:** GameServer `audit018-server-create` Running with a fresh agent heartbeat. No players online.

**Resources created:** none (modifying existing).

**Steps:**
1. Enable idle with the shortest allowed delay (the CRD minimum is 5): `kubectl patch gameserver audit018-server-create -n gameplane-games --type merge -p '{"spec":{"idle":{"enabled":true,"afterMinutes":5}}}'`.
2. Ensure players are 0 (unknown never sleeps; `status.idle.reason` then says why): check `kubectl get gameserver audit018-server-create -n gameplane-games -o jsonpath='{.status.agent.playersOnline}'`.
3. Wait for idle threshold: watch `kubectl get gameserver audit018-server-create -n gameplane-games -w -o jsonpath='{.status.idle.asleep}{"\n"}'` for 6+ minutes.
4. Verify phase transitions to Suspended: `kubectl get gameserver audit018-server-create -n gameplane-games -o jsonpath='{.status.phase}'`.
5. Confirm pod scaled down: `kubectl get sts audit018-server-create -n gameplane-games -o jsonpath='{.spec.replicas}'` prints `0`, and `kubectl get pod audit018-server-create-0 -n gameplane-games` returns NotFound.

**Expected:** status.idle.asleep = true, status.idle.asleepSince set (status.idle.emptySince is cleared once asleep), phase = Suspended with Ready condition reason IdleAsleep, pod deleted.

**Cleanup:** none (the server stays asleep for `gameserver-idle-wake-window`).

**Automatable?** yes; bucket: `operator`.

---

### gameserver-idle-wake-window

**Preconditions:** GameServer `audit018-server-create` sleeping from `gameserver-idle-auto-sleep`. Step 1 adds a `spec.idle.wakeWindows` cron entry that fires within the next minute.

**Resources created:** none (modifying existing).

**Steps:**
1. Set a wake window cron (e.g., `* * * * *` for every minute): `kubectl patch gameserver audit018-server-create -n gameplane-games --type merge -p '{"spec":{"idle":{"wakeWindows":["* * * * *"]}}}'`.
2. Wait for the next minute boundary: observe via `kubectl get gameserver audit018-server-create -n gameplane-games -w -o jsonpath='{.status.idle.asleep}{"\n"}'`.
3. Within 2 minutes, phase should transition back to Starting/Running.
4. Confirm pod is created: `kubectl get pod -n gameplane-games -l app.kubernetes.io/name=gameplane-game,app.kubernetes.io/instance=audit018-server-create`.

**Expected:** Pod scheduled, phase = Starting → Running, status.idle.asleep = false, status.idle.lastWakeTime updated.

**Cleanup:** remove the wake window, keeping idle on for `gameserver-idle-wake-on-connect`: `kubectl patch gameserver audit018-server-create -n gameplane-games --type merge -p '{"spec":{"idle":{"wakeWindows":null}}}'`.

**Automatable?** yes; bucket: `operator`.

---

### gameserver-idle-wake-on-connect

**Preconditions:** GameServer `audit018-server-create` with idle enabled and no wake window (from `gameserver-idle-wake-window`).

**Resources created:** Sentinel Deployment `audit018-server-create-waker` and its pod (auto-created by operator).

**Steps:**
1. Arm wake-on-connect: `kubectl patch gameserver audit018-server-create -n gameplane-games --type merge -p '{"spec":{"idle":{"wakeOnConnect":true}}}'`, then wait (6+ minutes, no players) until `kubectl get gameserver audit018-server-create -n gameplane-games -o jsonpath='{.status.idle.asleep}'` prints `true`.
2. Locate the sentinel pod: `kubectl get pod -n gameplane-games -l app.kubernetes.io/name=gameplane-waker,app.kubernetes.io/instance=audit018-server-create`.
3. From outside the cluster, make a real game connection to the game port (e.g. `kubectl port-forward -n gameplane-games svc/audit018-server-create 25565:25565`, then join or server-list-ping `127.0.0.1:25565` with a Minecraft client). This simulates a player joining.
4. Watch sentinel detect the connection and trigger wake (may trigger via webhook or controller event).
5. Confirm game pod is created: `kubectl get pod -n gameplane-games -l app.kubernetes.io/name=gameplane-game,app.kubernetes.io/instance=audit018-server-create`.
6. Verify phase: `kubectl get gameserver audit018-server-create -n gameplane-games -o jsonpath='{.status.phase}'`.

**Expected:** Sentinel pod exists while asleep. On connection, game pod is created, phase transitions to Starting → Running.

**Cleanup:** disable idle (the operator wakes the server if it is still asleep): `kubectl patch gameserver audit018-server-create -n gameplane-games --type merge -p '{"spec":{"idle":null}}'`.

**Automatable?** no (requires external client connection to sentinel port).

---

### gameserver-delete-with-finalizer

**Preconditions:** GameServer `audit018-server-create` exists (may be running or stopped). Run this after every other procedure in this file that uses `audit018-server-create` (the backup, schedule, capture and volume-snapshot procedures below).

**Resources created:** none (deleting existing).

**Steps:**
1. Record finalizers before deleting: `kubectl get gameserver audit018-server-create -n gameplane-games -o jsonpath='{.metadata.finalizers}'`.
2. Initiate delete: `kubectl delete gameserver audit018-server-create -n gameplane-games --wait=true --timeout=120s`.
3. Observe StatefulSet, Service and PVC cleanup: `kubectl get sts,svc,pvc -n gameplane-games | grep audit018-server-create` (repeat until empty).
4. Confirm the GameServer is gone: `kubectl get gameserver audit018-server-create -n gameplane-games` returns NotFound.

**Expected:** `metadata.finalizers` is empty: the operator sets no GameServer finalizer, and cleanup is by ownerReference garbage collection. The GameServer is deleted at once; its StatefulSet, Services and `audit018-server-create-data` PVC are garbage-collected shortly after.

**Cleanup:** none (object deleted).

**Automatable?** yes; bucket: `operator`.

---

### gameserver-failed-phase-crash-loop

**Preconditions:** GameTemplate `audit018-test-template` exists (from `gametemplate-create`).

**Resources created:** GameServer named `audit018-server-failed`.

**Steps:**
1. Create a GameServer from the template whose `spec.image` override exits at once (busybox's default `sh` exits with no stdin, so the container restart-loops):
   ```sh
   kubectl apply -f - <<'EOF'
   apiVersion: gameplane.local/v1alpha1
   kind: GameServer
   metadata:
     name: audit018-server-failed
     namespace: gameplane-games
     labels:
       gameplane.io/audit: "018"
   spec:
     templateRef:
       name: audit018-test-template
     image: busybox:1.36
   EOF
   ```
2. Watch pod crash: `kubectl get pod -n gameplane-games -l app.kubernetes.io/name=gameplane-game,app.kubernetes.io/instance=audit018-server-failed -w`.
3. Allow ~2 minutes for the game container to reach 3 restarts.
4. Check GameServer status: `kubectl get gameserver audit018-server-failed -n gameplane-games -o jsonpath='{.status.conditions}'`.
5. Verify phase = Failed: `kubectl get gameserver audit018-server-failed -n gameplane-games -o jsonpath='{.status.phase}'`.

**Expected:** Pod crashes repeatedly. CrashLoopBackOff observed. GameServer phase escalates to Failed once the game container has restarted 3 times. The Ready and Progressing conditions carry reason `CrashLoopBackOff` (or `ContainerExited` for a non-zero exit) with the crash explanation.

**Cleanup:** `kubectl delete gameserver audit018-server-failed -n gameplane-games`.

**Automatable?** yes; bucket: `operator`.

---

### gametemplate-create

**Preconditions:** User or admin role. The pre-existing GameTemplate `minecraft-java` exists (it is only read, never written).

**Resources created:** GameTemplate named `audit018-test-template`.

**Steps:**
1. Define the template by copying the spec of `minecraft-java` (it has several `spec.versions` entries and reports a player count over RCON, which later procedures need): `kubectl get gametemplate minecraft-java -o json | jq '{apiVersion, kind, metadata: {name: "audit018-test-template", labels: {"gameplane.io/audit": "018"}}, spec}' > audit018-template.json`.
2. Apply: `kubectl apply -f audit018-template.json`.
3. List templates: `kubectl get gametemplate audit018-test-template -o yaml`.
4. Verify fields are persisted: displayName, game, image, ports, and at least 2 `spec.versions` entries.

**Expected:** GameTemplate exists in apiserver. Can be referenced by GameServers via templateRef.

**Cleanup:** `kubectl delete gametemplate audit018-test-template` (after every GameServer that references it is deleted).

**Automatable?** yes; bucket: `operator`.

---

### backup-create-and-run

**Preconditions:** GameServer `audit018-server-create` running. A restic repository URL reachable from `gameplane-games` (the Backup Job runs `restic init` if the repository does not exist yet).

**Resources created:** Secret `audit018-repo-secret`, Backup named `audit018-backup-1`, marker file `audit018-backup-marker.txt` on the server's data volume.

**Steps:**
1. Create the repo Secret (keys `repo` and `password`; the password is generated and never printed or saved): `kubectl create secret generic audit018-repo-secret -n gameplane-games --from-literal=repo='<restic-repo-url>' --from-literal=password="$(openssl rand -hex 24)"`, then `kubectl label secret audit018-repo-secret -n gameplane-games gameplane.io/audit=018`.
2. Write a marker file for `restore-complete-and-resume` to find later: `curl -s -o /dev/null -w '%{http_code}\n' -X POST -H "Cookie: gameplane_session=$SESS; gameplane_csrf=$CSRF" -H "X-Gameplane-CSRF: $CSRF" --data-binary "audit018-backup-marker" "$GP/servers/audit018-server-create/files/write?path=/audit018-backup-marker.txt"` (expect 204).
3. Create Backup:
   ```sh
   kubectl apply -f - <<'EOF'
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-backup-1
     namespace: gameplane-games
     labels:
       gameplane.io/audit: "018"
   spec:
     serverRef:
       name: audit018-server-create
     repoRef:
       name: audit018-repo-secret
       key: repo
   EOF
   ```
4. Watch phase: `kubectl get backup audit018-backup-1 -n gameplane-games -w -o jsonpath='{.status.phase}{"\n"}'`.
5. Confirm phase transitions: Pending → Running.
6. Wait for completion: `kubectl wait --for=jsonpath='{.status.phase}'=Succeeded backup/audit018-backup-1 -n gameplane-games --timeout=600s`.
7. Record completion time: `kubectl get backup audit018-backup-1 -n gameplane-games -o jsonpath='{.status.completionTime}'`.

**Expected:** Backup phase = Running. Backup Job pod created and runs restic. On success, phase = Succeeded, status.completionTime set, status.snapshotID populated.

**Cleanup:** none yet (`restore-create-and-suspend-server` needs the Backup and the schedule procedures need the Secret). After `restore-complete-and-resume`: `kubectl delete backup audit018-backup-1 -n gameplane-games`.

**Automatable?** yes; bucket: `api-mods`.

---

### backup-failure-and-phase

**Preconditions:** The restic repository was initialized by `backup-create-and-run` (Secret `audit018-repo-secret` exists).

**Resources created:** Secret `audit018-repo-secret-bad`, Backup named `audit018-backup-fail`.

**Steps:**
1. Create a Secret for the same repository with a wrong password: `kubectl create secret generic audit018-repo-secret-bad -n gameplane-games --from-literal=repo="$(kubectl get secret audit018-repo-secret -n gameplane-games -o jsonpath='{.data.repo}' | base64 -d)" --from-literal=password="$(openssl rand -hex 24)"`, then `kubectl label secret audit018-repo-secret-bad -n gameplane-games gameplane.io/audit=018`.
2. Create Backup with bad repo credentials: the manifest from `backup-create-and-run` step 3 with `metadata.name: audit018-backup-fail` and `spec.repoRef.name: audit018-repo-secret-bad`, applied with `kubectl apply -f -`.
3. Watch phase: `kubectl get backup audit018-backup-fail -n gameplane-games -w -o jsonpath='{.status.phase}{"\n"}'`.
4. Observe Job failure: `kubectl get job -n gameplane-games | grep audit018-backup-fail`.
5. Check backup status message: `kubectl get backup audit018-backup-fail -n gameplane-games -o jsonpath='{.status.message}'`.

**Expected:** Backup phase transitions Pending → Running → Failed. Job Pod exits non-zero. status.message explains the error.

**Cleanup:** `kubectl delete backup audit018-backup-fail -n gameplane-games` and `kubectl delete secret audit018-repo-secret-bad -n gameplane-games`.

**Automatable?** yes; bucket: `api-mods`.

---

### restore-create-and-suspend-server

**Preconditions:** Backup `audit018-backup-1` in Succeeded state (from `backup-create-and-run`). GameTemplate `audit018-test-template` exists.

**Resources created:** GameServer `audit018-restore-target` (from `audit018-test-template`), Restore named `audit018-restore-1`.

**Steps:**
1. Create the target GameServer and wait for it to run:
   ```sh
   kubectl apply -f - <<'EOF'
   apiVersion: gameplane.local/v1alpha1
   kind: GameServer
   metadata:
     name: audit018-restore-target
     namespace: gameplane-games
     labels:
       gameplane.io/audit: "018"
   spec:
     templateRef:
       name: audit018-test-template
   EOF
   kubectl wait gameserver/audit018-restore-target -n gameplane-games --for=jsonpath='{.status.phase}'=Running --timeout=600s
   ```
2. Create Restore:
   ```sh
   kubectl apply -f - <<'EOF'
   apiVersion: gameplane.local/v1alpha1
   kind: Restore
   metadata:
     name: audit018-restore-1
     namespace: gameplane-games
     labels:
       gameplane.io/audit: "018"
   spec:
     backupRef:
       name: audit018-backup-1
     serverRef:
       name: audit018-restore-target
   EOF
   ```
3. Watch phase: `kubectl get restore audit018-restore-1 -n gameplane-games -w -o jsonpath='{.status.phase}{"\n"}'`.
4. Verify target server is suspended: `kubectl get gameserver audit018-restore-target -n gameplane-games -o jsonpath='{.status.phase}'` should be Suspended or Stopping.
5. Observe Restore phases: Pending → Suspending → Running.

**Expected:** Phase transitions correctly. Target GameServer scales to 0 (suspended). Restore Job `restore-audit018-restore-1` created. PVC mount point prepared.

**Cleanup:** none (observe Running phase for now).

**Automatable?** yes; bucket: `api-mods`.

---

### restore-complete-and-resume

**Preconditions:** Restore `audit018-restore-1` in Running phase from above.

**Resources created:** none (completing existing).

**Steps:**
1. Wait for restore Job completion: `kubectl wait --for=jsonpath='{.status.phase}'=Succeeded restore/audit018-restore-1 -n gameplane-games --timeout=600s` (or Failed if an error).
2. On Job success the phase goes straight from Running to Succeeded (the controller never sets Resuming) and the controller sets the target's `spec.suspend` back to false.
3. Watch target GameServer come back up: `kubectl get gameserver audit018-restore-target -n gameplane-games -w -o jsonpath='{.status.phase}{"\n"}'` should transition to Starting → Running.
4. Verify data was restored: the marker written in `backup-create-and-run` step 2 is present: `curl -s -H "Cookie: gameplane_session=$SESS; gameplane_csrf=$CSRF" "$GP/servers/audit018-restore-target/files/read?path=/audit018-backup-marker.txt"`.

**Expected:** Restore phase = Succeeded or Failed (check condition for details). Target server resumes at Running. Pod recreated with restored data.

**Cleanup:** `kubectl delete restore audit018-restore-1 -n gameplane-games`, `kubectl delete gameserver audit018-restore-target -n gameplane-games` and `kubectl delete backup audit018-backup-1 -n gameplane-games`.

**Automatable?** yes; bucket: `api-mods`.

---

### backup-schedule-create

**Preconditions:** GameServer `audit018-server-create` running. Secret `audit018-repo-secret` from `backup-create-and-run`. User/admin role.

**Resources created:** BackupSchedule named `audit018-schedule-1`.

**Steps:**
1. Create BackupSchedule (every 5 minutes):
   ```sh
   kubectl apply -f - <<'EOF'
   apiVersion: gameplane.local/v1alpha1
   kind: BackupSchedule
   metadata:
     name: audit018-schedule-1
     namespace: gameplane-games
     labels:
       gameplane.io/audit: "018"
   spec:
     serverRef:
       name: audit018-server-create
     schedule: "*/5 * * * *"
     repoRef:
       name: audit018-repo-secret
       key: repo
   EOF
   ```
2. List schedules: `kubectl get backupschedule audit018-schedule-1 -n gameplane-games -o yaml`.
3. Record status.nextScheduleTime: `kubectl get backupschedule audit018-schedule-1 -n gameplane-games -o jsonpath='{.status.nextScheduleTime}'`.

**Expected:** BackupSchedule exists. nextScheduleTime is within 5 minutes. No Backup objects yet (wait for first tick).

**Cleanup:** none (used by the next two procedures; deleted in `backup-schedule-retention-prunes`).

**Automatable?** yes; bucket: `api-mods`.

---

### backup-schedule-tick-creates-backup

**Preconditions:** BackupSchedule `audit018-schedule-1` with schedule firing within the next 5 minutes.

**Resources created:** Backup `audit018-schedule-1-<YYYYMMDD-HHMMSS>` (auto-created by controller).

**Steps:**
1. Wait for schedule tick (up to 5 min): watch `kubectl get backup -n gameplane-games | grep audit018-schedule-1`.
2. Once Backup created, observe its phase: `kubectl get backup -n gameplane-games -l gameplane.local/backup-schedule=audit018-schedule-1 -w -o jsonpath='{.metadata.name}{" "}{.status.phase}{"\n"}'`.
3. Wait for completion: `kubectl wait --for=jsonpath='{.status.phase}'=Succeeded backup/<name> -n gameplane-games --timeout=600s`.
4. Update BackupSchedule status: `kubectl get backupschedule audit018-schedule-1 -n gameplane-games -o jsonpath='{.status.lastSuccessfulTime}'` should be recent.

**Expected:** On schedule tick, a new Backup is created owned by the BackupSchedule. Backup runs and succeeds. lastSuccessfulTime updated.

**Cleanup:** none (delete schedule in final step).

**Automatable?** yes; bucket: `api-mods`.

---

### backup-schedule-retention-prunes

**Preconditions:** BackupSchedule `audit018-schedule-1` from `backup-schedule-create`, with at least one succeeded Backup.

**Resources created:** none (observing deletion).

**Steps:**
1. Set the retention policy: `kubectl patch backupschedule audit018-schedule-1 -n gameplane-games --type merge -p '{"spec":{"retention":{"keepLast":2}}}'`.
2. Wait for at least 3 schedule ticks (about 15 minutes) and note the older Backup names: `kubectl get backup -n gameplane-games -l gameplane.local/backup-schedule=audit018-schedule-1`.
3. After the next tick, list the schedule's Backups with their phases: `kubectl get backup -n gameplane-games -l gameplane.local/backup-schedule=audit018-schedule-1 -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.phase}{"\n"}{end}'`.

**Expected:** When retention.keepLast=2, at most 2 Succeeded Backups remain; older Succeeded ones are deleted by the controller. In-flight (Pending/Running) Backups are never deleted.

**Cleanup:** `kubectl delete backupschedule audit018-schedule-1 -n gameplane-games` (its Backups are garbage-collected with it) and `kubectl delete secret audit018-repo-secret -n gameplane-games`.

**Automatable?** yes; bucket: `api-mods`.

---

### module-create-pending

**Preconditions:** ModuleSource `default` exists and is indexed (it is only referenced, never written). Module bundle `valheim` exists in the source.

**Resources created:** Module named `audit018-module-1`.

**Steps:**
1. Create Module:
   ```sh
   kubectl apply -f - <<'EOF'
   apiVersion: gameplane.local/v1alpha1
   kind: Module
   metadata:
     name: audit018-module-1
     labels:
       gameplane.io/audit: "018"
   spec:
     source:
       name: default
     name: valheim
   EOF
   ```
2. Check phase: `kubectl get module audit018-module-1 -o jsonpath='{.status.phase}'` should be Pending or Pulling.
3. Record appliedVersion: `kubectl get module audit018-module-1 -o jsonpath='{.status.appliedVersion}'` (may be empty while Pending).

**Expected:** Module created. Phase = Pending or Pulling. Controller is resolving the version.

**Cleanup:** none (observe next step).

**Automatable?** yes; bucket: `api-mods`.

---

### module-pulling-to-ready

**Preconditions:** Module `audit018-module-1` in Pulling phase from above.

**Resources created:** none (observing existing).

**Steps:**
1. Watch phase: `kubectl get module audit018-module-1 -w -o jsonpath='{.status.phase}{"\n"}'`.
2. Wait for phase = Ready: `kubectl wait --for=jsonpath='{.status.phase}'=Ready module/audit018-module-1 --timeout=300s`.
3. Verify GameTemplate created: `kubectl get gametemplate audit018-module-1 -o yaml` (named same as Module).
4. Check labels: `kubectl get gametemplate audit018-module-1 -o jsonpath='{.metadata.labels.gameplane\.local/managed-by}'` should be "Module".

**Expected:** Phase transitions Pulling → Ready. GameTemplate created with labels indicating Module ownership. appliedVersion and appliedDigest recorded.

**Cleanup:** `kubectl delete module audit018-module-1` (its GameTemplate `audit018-module-1` is garbage-collected with it).

**Automatable?** yes; bucket: `api-mods`.

---

### module-bad-signature-fails

**Preconditions:** An `audit018-` ModuleSource of type `oci` with `spec.verify` configured (never `default` or `uploads`), pointing at a registry that holds an unsigned or incorrectly signed module bundle.

**Resources created:** that `audit018-` ModuleSource, Module named `audit018-module-bad-sig`.

**Steps:**
1. Create Module referencing an unsigned bundle: `kubectl apply -f audit018-module-unsigned.yaml`.
2. Watch phase: `kubectl get module audit018-module-bad-sig -w -o jsonpath='{.status.phase}{"\n"}'`.
3. Phase should transition to Failed: `kubectl wait --for=jsonpath='{.status.phase}'=Failed module/audit018-module-bad-sig --timeout=300s`.
4. Check lastError: `kubectl get module audit018-module-bad-sig -o jsonpath='{.status.lastError}'` should mention signature verification failure.

**Expected:** Phase = Failed. lastError explains signature mismatch or verification failure. No GameTemplate created.

**Cleanup:** `kubectl delete module audit018-module-bad-sig`, then delete the `audit018-` ModuleSource.

**Automatable?** no (requires external bundle management; defer to T026).

---

### modulesource-oci-sync

**Preconditions:** OCI registry `ghcr.io/valgulnecron/gameplane-modules` reachable from the operator. The pre-existing ModuleSource `default` is git-based on kubelab and is not written to, so this procedure creates its own oci source.

**Resources created:** ModuleSource named `audit018-oci-source`.

**Steps:**
1. Create the source with a 1-minute refresh:
   ```sh
   kubectl apply -f - <<'EOF'
   apiVersion: gameplane.local/v1alpha1
   kind: ModuleSource
   metadata:
     name: audit018-oci-source
     labels:
       gameplane.io/audit: "018"
   spec:
     type: oci
     oci:
       url: ghcr.io/valgulnecron/gameplane-modules
       modules:
         - name: valheim
     refreshInterval: 1m
   EOF
   ```
2. Get ModuleSource status: `kubectl get modulesource audit018-oci-source -o jsonpath='{.status.conditions}'`.
3. Record lastSync: `kubectl get modulesource audit018-oci-source -o jsonpath='{.status.lastSync}'`.
4. Watch for the next refresh (about 1 minute): `kubectl get modulesource audit018-oci-source -w -o jsonpath='{.status.lastSync}{"\n"}'` (should update).
5. List discovered modules: `kubectl get modulesource audit018-oci-source -o jsonpath='{.status.modules[*].name}'`.

**Expected:** ModuleSource periodically syncs with OCI registry. status.lastSync updates. Condition Synced=True (reason Indexed). status.modules contains `valheim` with its versions.

**Cleanup:** `kubectl delete modulesource audit018-oci-source`.

**Automatable?** yes; bucket: `api-mods`.

---

### modulesource-git-sync-error

**Preconditions:** None. The procedure creates its own git ModuleSource with a repo URL that cannot resolve.

**Resources created:** ModuleSource named `audit018-git-bad`.

**Steps:**
1. Create the source:
   ```sh
   kubectl apply -f - <<'EOF'
   apiVersion: gameplane.local/v1alpha1
   kind: ModuleSource
   metadata:
     name: audit018-git-bad
     labels:
       gameplane.io/audit: "018"
   spec:
     type: git
     git:
       url: https://audit018.invalid/gameplane-module.git
   EOF
   ```
2. Check conditions for errors: `kubectl get modulesource audit018-git-bad -o jsonpath='{.status.conditions}'`.
3. Record the Synced condition message: `kubectl get modulesource audit018-git-bad -o jsonpath='{.status.conditions[?(@.type=="Synced")].message}'`.

**Expected:** The git clone fails: condition Synced=False with reason IndexFailed and a message naming the error, condition Ready=False (reason SourceUnreachable), and `status.lastSync` stays empty.

**Cleanup:** `kubectl delete modulesource audit018-git-bad`.

**Automatable?** yes; bucket: `api-mods`.

---

### cluster-register-and-health-check

**Preconditions:** User/admin role. Target remote Kubernetes cluster kubeconfig accessible as a local file (off-git).

**Resources created:** Cluster named `audit018-cluster-1`. Secret `audit018-kubeconfig-secret` in `gameplane-system` holding the kubeconfig.

**Steps:**
1. Create the kubeconfig Secret in the control-plane namespace (key `kubeconfig`, label `gameplane.local/cluster-kubeconfig=true`): `kubectl create secret generic audit018-kubeconfig-secret -n gameplane-system --from-file=kubeconfig=<remote-kubeconfig-file>`, then `kubectl label secret audit018-kubeconfig-secret -n gameplane-system gameplane.local/cluster-kubeconfig=true gameplane.io/audit=018`.
2. Create Cluster:
   ```sh
   kubectl apply -f - <<'EOF'
   apiVersion: gameplane.local/v1alpha1
   kind: Cluster
   metadata:
     name: audit018-cluster-1
     labels:
       gameplane.io/audit: "018"
   spec:
     kubeconfigSecret:
       name: audit018-kubeconfig-secret
   EOF
   ```
3. Watch phase: `kubectl get cluster audit018-cluster-1 -w -o jsonpath='{.status.phase}{"\n"}'`.
4. Phase should transition: Unknown → Healthy (if reachable) or Unknown → Unhealthy (if not).
5. Check serverVersion: `kubectl get cluster audit018-cluster-1 -o jsonpath='{.status.serverVersion}'`.

**Expected:** Cluster phase = Healthy or Unhealthy. If Healthy, serverVersion is populated. Conditions show health check result.

**Cleanup:** none (used by `cluster-health-check-unreachable`).

**Automatable?** yes; bucket: `multicluster`.

---

### cluster-health-check-unreachable

**Preconditions:** Cluster `audit018-cluster-1` registered from above.

**Resources created:** none (observing existing).

**Steps:**
1. Make the target unreachable without touching any real cluster: replace the Secret's kubeconfig with one whose `server:` is `https://192.0.2.1:6443` (TEST-NET-1, never routable): `kubectl create secret generic audit018-kubeconfig-secret -n gameplane-system --from-file=kubeconfig=<unreachable-kubeconfig-file> --dry-run=client -o yaml | kubectl apply -f -`, then re-run the `kubectl label` command from `cluster-register-and-health-check` step 1 with `--overwrite`. Never stop or firewall kubelab's own apiserver.
2. Watch Cluster status: `kubectl get cluster audit018-cluster-1 -w -o jsonpath='{.status.phase}{"\n"}'`.
3. Within the next health check (the controller re-checks every 2 minutes), phase should become Unhealthy.
4. Check message: `kubectl get cluster audit018-cluster-1 -o jsonpath='{.status.message}'`.

**Expected:** Phase = Unhealthy. message explains the failure (e.g., "connection refused", "timeout"). serverVersion may be stale.

**Cleanup:** `kubectl delete cluster audit018-cluster-1` and `kubectl delete secret audit018-kubeconfig-secret -n gameplane-system`.

**Automatable?** yes; bucket: `multicluster`.

---

### networkcapture-create-pending

**Preconditions:** Helm value `capture.enabled=true` for this round (it is `false` in the kubelab baseline). GameServer `audit018-server-create` running. Admin role (capture needs `captures:manage`).

**Resources created:** NetworkCapture named `audit018-capture-1`.

**Steps:**
1. Enable capture on the server: `curl -s -w '\nHTTP %{http_code}\n' -X POST -H "Cookie: gameplane_session=$SESS; gameplane_csrf=$CSRF" -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-server-create:capture-enable?namespace=gameplane-games"` (expect 200; 501 means the feature is off cluster-wide). Then verify the capture sidecar is ready: `kubectl get pod audit018-server-create-0 -n gameplane-games -o jsonpath='{.status.ephemeralContainerStatuses[*].name}'` should include `capture`, and `kubectl get gameserver audit018-server-create -n gameplane-games -o jsonpath='{.status.capture.ready}'` prints `true`.
2. Create NetworkCapture:
   ```sh
   kubectl apply -f - <<'EOF'
   apiVersion: gameplane.local/v1alpha1
   kind: NetworkCapture
   metadata:
     name: audit018-capture-1
     namespace: gameplane-games
     labels:
       gameplane.io/audit: "018"
   spec:
     serverRef:
       name: audit018-server-create
     maxDuration: 5m
     maxSize: 100Mi
     ttlSecondsAfterFinished: 60
   EOF
   ```
3. Check phase: `kubectl get networkcapture audit018-capture-1 -n gameplane-games -o jsonpath='{.status.phase}'` should be Pending (it may already read Running if the operator reconciled first).
4. Record creationTimestamp: `kubectl get networkcapture audit018-capture-1 -n gameplane-games -o jsonpath='{.metadata.creationTimestamp}'`.

**Expected:** NetworkCapture phase = Pending. status.startTime empty (capture not yet started).

**Cleanup:** none (observe next step).

**Automatable?** yes; bucket: `operator`.

---

### networkcapture-start-running

**Preconditions:** NetworkCapture `audit018-capture-1` in Pending state from above.

**Resources created:** none (observing existing).

**Steps:**
1. Watch phase: `kubectl get networkcapture audit018-capture-1 -n gameplane-games -w -o jsonpath='{.status.phase}{"\n"}'`.
2. Wait for phase = Running: `kubectl wait --for=jsonpath='{.status.phase}'=Running networkcapture/audit018-capture-1 -n gameplane-games --timeout=30s`.
3. Record startTime: `kubectl get networkcapture audit018-capture-1 -n gameplane-games -o jsonpath='{.status.startTime}'`.
4. Observe packets captured: `kubectl get networkcapture audit018-capture-1 -n gameplane-games -o jsonpath='{.status.packetsWritten}'`.

**Expected:** Phase transitions Pending → Running. startTime populated. Packets and bytes counters start incrementing (if traffic is flowing).

**Cleanup:** none (stop capture in next step).

**Automatable?** yes; bucket: `operator`.

---

### networkcapture-stop-completed

**Preconditions:** NetworkCapture `audit018-capture-1` in Running state from above.

**Resources created:** none (stopping existing).

**Steps:**
1. Stop capture manually through the API (the sidecar already holds `maxDuration`, so patching the spec does not stop it): `curl -s -w '\nHTTP %{http_code}\n' -X POST -H "Cookie: gameplane_session=$SESS; gameplane_csrf=$CSRF" -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" -d '{"captureId":"audit018-capture-1"}' "$GP/servers/audit018-server-create:capture-stop?namespace=gameplane-games"` (login cost: 0).
2. Or wait for maxDuration to elapse (e.g., 5m from startTime).
3. Watch phase: `kubectl get networkcapture audit018-capture-1 -n gameplane-games -w -o jsonpath='{.status.phase}{"\n"}'`.
4. Phase should transition to Completed: `kubectl wait --for=jsonpath='{.status.phase}'=Completed networkcapture/audit018-capture-1 -n gameplane-games --timeout=30s`.
5. Record completionTime and final stats: `kubectl get networkcapture audit018-capture-1 -n gameplane-games -o yaml | grep -E 'completionTime|packetsWritten|bytesWritten'`.

**Expected:** Phase = Completed. completionTime populated. packetsWritten and bytesWritten show final counts.

**Cleanup:** none (capture will auto-delete after TTL).

**Automatable?** yes; bucket: `operator`.

---

### networkcapture-failed-sidecar-crash

**Preconditions:** GameServer `audit018-server-create` with capture enabled (from `networkcapture-create-pending`). No other capture of this server is Pending or Running.

**Resources created:** NetworkCapture named `audit018-capture-fail`.

**Steps:**
1. Start a capture: the manifest from `networkcapture-create-pending` step 2 with `metadata.name: audit018-capture-fail`, applied with `kubectl apply -f -`, then wait for Running: `kubectl wait --for=jsonpath='{.status.phase}'=Running networkcapture/audit018-capture-fail -n gameplane-games --timeout=30s`.
2. Take the sidecar down mid-capture. The capture image is distroless (no shell or `kill`), so `kubectl exec` cannot kill it; delete the game pod instead, which kills the ephemeral sidecar with it: `kubectl delete pod audit018-server-create-0 -n gameplane-games`.
3. Watch capture phase: `kubectl get networkcapture audit018-capture-fail -n gameplane-games -w -o jsonpath='{.status.phase}{"\n"}'`.
4. Phase should transition to Failed within ~30s.
5. Check message: `kubectl get networkcapture audit018-capture-fail -n gameplane-games -o jsonpath='{.status.message}'`.

**Expected:** Phase = Failed. message explains the loss of the sidecar (e.g., "game pod was deleted while the capture was running").

**Cleanup:** `kubectl delete networkcapture audit018-capture-fail -n gameplane-games`.

**Automatable?** yes; bucket: `operator`.

---

### networkcapture-expired-auto-delete

**Preconditions:** Completed NetworkCapture `audit018-capture-1` with a short TTL (`spec.ttlSecondsAfterFinished: 60`, set in `networkcapture-create-pending`). Start right after `networkcapture-stop-completed`.

**Resources created:** none (observing deletion).

**Steps:**
1. Wait for TTL to elapse: monitor `kubectl get networkcapture audit018-capture-1 -n gameplane-games -w`.
2. After ttlSecondsAfterFinished seconds pass since completionTime, the operator should delete the NetworkCapture.
3. Verify deletion: `kubectl get networkcapture audit018-capture-1 -n gameplane-games` (should not exist, or phase = Expired if recorded before deletion).

**Expected:** After TTL elapses, NetworkCapture is deleted by operator. Phase may show Expired briefly.

**Cleanup:** none (object auto-deleted).

**Automatable?** yes; bucket: `operator`.

---

### backup-volumesnapshot-strategy

**Preconditions:** GameServer `audit018-server-create` running. Cluster CSI driver supports VolumeSnapshot (a default VolumeSnapshotClass exists, or one is named via `spec.volumeSnapshotClassName`; check with `kubectl get volumesnapshotclass`, and record the row `blocked` if there is none).

**Resources created:** Backup named `audit018-backup-vs` with `spec.strategy: volume-snapshot` (no `repoRef`).

**Steps:**
1. Create Backup: `kubectl apply -f audit018-backup-vs.yaml` with `metadata.name: audit018-backup-vs`, `metadata.namespace: gameplane-games`, label `gameplane.io/audit: "018"`, `spec.serverRef.name: audit018-server-create`, `spec.strategy: volume-snapshot`, `spec.repoRef` omitted.
2. Watch phase: `kubectl get backup audit018-backup-vs -n gameplane-games -w -o jsonpath='{.status.phase}{"\n"}'`.
3. Confirm a VolumeSnapshot is created: `kubectl get volumesnapshot -n gameplane-games | grep audit018-backup-vs`.
4. Wait for completion: `kubectl wait --for=jsonpath='{.status.phase}'=Succeeded backup/audit018-backup-vs -n gameplane-games --timeout=600s`.
5. Record the bound snapshot: `kubectl get backup audit018-backup-vs -n gameplane-games -o jsonpath='{.status.volumeSnapshotContentName}'`.

**Expected:** Backup skips restic entirely; a VolumeSnapshot of the server's data PVC is created, quiesce/unquiesce brackets it the same way the restic path does, and once the VolumeSnapshot reports readyToUse the Backup phase reaches Succeeded with `status.volumeSnapshotContentName` set.

**Cleanup:** after `restore-volumesnapshot-strategy`: `kubectl delete backup audit018-backup-vs -n gameplane-games` (the owned VolumeSnapshot is garbage-collected with it).

**Automatable?** yes; bucket: `api-mods`.

---

### restore-volumesnapshot-strategy

**Preconditions:** A Succeeded `volume-snapshot`-strategy Backup exists (`audit018-backup-vs` from `backup-volumesnapshot-strategy`).

**Resources created:** Restore named `audit018-restore-vs`, which provisions a new GameServer `audit018-restore-vs-server` from the snapshot; Restore `audit018-restore-vs-dup` (step 5).

**Steps:**
1. Create Restore: `kubectl apply -f audit018-restore-vs.yaml` with `metadata.name: audit018-restore-vs`, `metadata.namespace: gameplane-games`, label `gameplane.io/audit: "018"`, `spec.backupRef.name: audit018-backup-vs`, `spec.serverRef.name: audit018-restore-vs-server` (a server that does not exist yet).
2. Watch phase: `kubectl get restore audit018-restore-vs -n gameplane-games -w -o jsonpath='{.status.phase}{"\n"}'`.
3. Confirm the new GameServer's PVC is provisioned with the VolumeSnapshot as its `dataSource`: `kubectl get pvc -n gameplane-games audit018-restore-vs-server-data -o jsonpath='{.spec.dataSource}'`.
4. Wait for completion: `kubectl wait --for=jsonpath='{.status.phase}'=Succeeded restore/audit018-restore-vs -n gameplane-games --timeout=600s`.
5. Repeat step 1 as Restore `audit018-restore-vs-dup` with `spec.serverRef.name: audit018-server-create` (an existing audit server) and confirm it is rejected: `kubectl get restore audit018-restore-vs-dup -n gameplane-games -o jsonpath='{.status.phase}{" "}{.status.message}'`.

**Expected:** Restore reconstructs a new PVC using the bound VolumeSnapshot as `dataSource`, the target GameServer reaches Running, and re-targeting an existing GameServer name is rejected rather than overwritten (Restore phase Failed, message says the target server already exists; `audit018-server-create` is untouched).

**Cleanup:** `kubectl delete restore audit018-restore-vs audit018-restore-vs-dup -n gameplane-games` and `kubectl delete gameserver audit018-restore-vs-server -n gameplane-games`.

**Automatable?** yes; bucket: `api-mods`.

