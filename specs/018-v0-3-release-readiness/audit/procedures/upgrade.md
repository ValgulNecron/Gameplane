# Procedures: UPG

Shared conventions: [conventions.md](../conventions.md).

### baseline-beta8

> **DO NOT RUN the reinstall branch (step 3) — OD-023.** Finding F-212 (verified, S1): `helm uninstall gameplane` deletes the Helm-owned `gameplane-games` namespace, and with it every GameServer, StatefulSet and data PVC in it, including the pre-existing `mc-fabric`, `soak-*` and `squad`. kubelab is on migration 011, so OD-005 path (b) would take this branch. Wait for the maintainer's OD-023 decision.

This establishes the upgrade baseline at `v0.2.0-beta.8` as required by FR-015. If kubelab's API database schema is already ahead of beta.8, it uninstalls and reinstalls at beta.8 with a fresh database; otherwise it upgrades in place. The real database is snapshotted before any changes for later restoration.

**Preconditions**

- `kubelab-baseline.md` has been captured
- `kubectl get nodes` lists `kubelab-control`, `kubelab-worker-1`, `kubelab-worker-2` in Ready state
- Current Helm release is recorded in `helm get values gameplane -n gameplane-system`
- Check kubelab's API database migration level and take the pre-upgrade snapshot as described in Step 1. The API container is `gcr.io/distroless/static:nonroot` (`api/Dockerfile`) with no shell and no `sqlite3` binary, so both run through a temporary `audit018-db-tool` pod that mounts the `gameplane-api-data` PVC directly, not a `kubectl exec` into the API pod.

**Resources created**

- `audit018-db-tool` Pod (ephemeral; created and deleted within Step 1) — mounts the `gameplane-api-data` PVC directly while `gameplane-api` is scaled to 0, since the API container has no `sqlite3` binary to exec into.

Otherwise this section only modifies the existing `gameplane` Helm release.

**Steps**

1. **Scale down, then snapshot the real database through a temporary tool pod** (off-git). The API container is distroless with no `sqlite3` binary or shell, so a throwaway pod mounts the same PVC instead of exec'ing into the API pod:
   ```sh
   mkdir -p ~/gameplane-audit-018/db-snapshots

   kubectl scale deployment gameplane-api --replicas=0 -n gameplane-system
   kubectl wait --for=delete pod -l app.kubernetes.io/name=gameplane-api -n gameplane-system --timeout=60s || true

   cat > /tmp/audit018-db-tool.yaml <<EOF
   apiVersion: v1
   kind: Pod
   metadata:
     name: audit018-db-tool
     namespace: gameplane-system
     labels:
       gameplane.io/audit: "018"
   spec:
     restartPolicy: Never
     containers:
       - name: tool
         image: alpine:3.20
         command: ["sleep", "600"]
         volumeMounts:
           - name: data
             mountPath: /data
     volumes:
       - name: data
         persistentVolumeClaim:
           claimName: gameplane-api-data
   EOF
   kubectl apply -f /tmp/audit018-db-tool.yaml
   kubectl wait --for=condition=Ready pod/audit018-db-tool -n gameplane-system --timeout=60s
   kubectl exec -n gameplane-system audit018-db-tool -- apk add --no-cache sqlite >/dev/null

   kubectl exec -n gameplane-system audit018-db-tool -- sqlite3 /data/gameplane.db "SELECT MAX(version) FROM schema_migrations;" > ~/gameplane-audit-018/db-level-before.txt
   kubectl exec -n gameplane-system audit018-db-tool -- sqlite3 /data/gameplane.db ".backup /tmp/upg-real.db"
   kubectl cp gameplane-system/audit018-db-tool:/tmp/upg-real.db ~/gameplane-audit-018/db-snapshots/upg-real.db

   kubectl delete pod audit018-db-tool -n gameplane-system --wait=true
   kubectl scale deployment gameplane-api --replicas=1 -n gameplane-system
   kubectl rollout status deployment/gameplane-api -n gameplane-system --timeout=300s

   ls -lh ~/gameplane-audit-018/db-snapshots/upg-real.db
   ```
   Record: pre-upgrade snapshot size and timestamp, and the migration level read from `~/gameplane-audit-018/db-level-before.txt`, in `rounds.md`.

2. **Check migration level** against `v0.2.0-beta.8` (ships `006_share_links.sql`):
   ```sh
   cat ~/gameplane-audit-018/db-level-before.txt
   ```
   If result is `006_share_links.sql` or earlier, go to step 4 (upgrade in place).
   If result is `007_*` or later, go to step 3 (reinstall).

3. **Reinstall at beta.8** (if ahead). After `--keep-history`, the release's last revision is uninstalled, so a plain `helm upgrade` fails with `has no deployed releases`; reinstall with `helm install --replace` instead, using values captured just before the uninstall, since `helm install` has no `--reuse-values`. Helm client on this devbox is v3.19.0 (`helm version`), which supports `--replace` (Helm calls it "unsafe in production" — kubelab here is the audit's test cluster, not production):
   ```sh
   helm get values gameplane -n gameplane-system -o yaml > ~/gameplane-audit-018/gameplane-values-before-uninstall.yaml
   
   helm uninstall gameplane -n gameplane-system --keep-history
   # Wait for API pod to terminate
   kubectl wait --for=delete pod -l app.kubernetes.io/name=gameplane-api -n gameplane-system --timeout=60s || true
   
   helm install gameplane oci://ghcr.io/valgulnecron/charts/gameplane \
     --version 0.2.0-beta.8 \
     -n gameplane-system --replace \
     -f ~/gameplane-audit-018/gameplane-values-before-uninstall.yaml
   
   # Wait for API to be ready
   kubectl rollout status deployment/gameplane-api -n gameplane-system --timeout=300s
   ```
   Record: reinstall path (a) used in `rounds.md`. Verify pre-existing GameServers are still running:
   ```sh
   kubectl get gameserver -A | grep -E "mc-fabric|soak-bogus-pool|soak-no-preference|soak-pool-west|squad"
   ```

4. **Upgrade in place** (if at or below beta.8):
   ```sh
   helm upgrade gameplane oci://ghcr.io/valgulnecron/charts/gameplane \
     --version 0.2.0-beta.8 \
     -n gameplane-system --reuse-values
   
   kubectl rollout status deployment/gameplane-api -n gameplane-system --timeout=300s
   ```
   Record: upgrade path (b) used in `rounds.md`.

5. **Verify beta.8 is running**:
   ```sh
   helm list -n gameplane-system | grep gameplane
   kubectl get deployment gameplane-api -n gameplane-system -o jsonpath='{.spec.template.spec.containers[0].image}'
   ```

**Expected**

- Pre-existing GameServers stay running (phase Running)
- API pod is ready and responding to requests
- Migration level is confirmed as `006_share_links.sql` or earlier in the database
- Real database snapshot is saved at `~/gameplane-audit-018/db-snapshots/upg-real.db`

**Cleanup**

- Database snapshot is kept off-git for the rollback test
- Helm release is left at beta.8 for the seed step

**Automatable?**

No. Requires a manual multi-step `kubectl` sequence (scale down, stand up a temporary pod, `kubectl exec`/`kubectl cp`, scale back up) since the API container has no `sqlite3` binary (OD-018).

---

### seed

Creates an `audit018-upg` GameServer with a marker file to verify persistence through upgrade and rollback cycles. Also creates the `audit018-admin` account for subsequent API tests.

**Preconditions**

- Gameplane is running at `v0.2.0-beta.8` (from baseline-beta8 step)
- Admin account does not yet exist (OD-015)
- `GP=http://127.0.0.1:18080` (port-forward running)

**Resources created**

- `audit018-upg` GameServer (and its PVC)
- `audit018-admin` user account (via API, not kubectl)

**Steps**

1. **Create admin account** (OD-015, decision: option b):
   ```sh
   PW=$(openssl rand -base64 24)
   echo "GAMEPLANE_ADMIN_PASSWORD=$PW" > ~/gameplane-audit-018/admin.env
   chmod 600 ~/gameplane-audit-018/admin.env

   echo "$PW" | kubectl exec -i -n gameplane-system <api-pod-name> -- \
     /api bootstrap-admin --username audit018-admin --password-stdin
   ```
   The API's entrypoint binary is `/api` (`api/Dockerfile`), not `gameplane-api`, and the image has no shell, so the password goes over stdin (`kubectl exec -i`, `--password-stdin`) rather than a shell env-var trick.
   Record: admin password location in `rounds.md`.

2. **Log in as admin**:
   ```sh
   curl -s -D ~/gameplane-audit-018/headers-admin.txt \
     -H "Content-Type: application/json" \
     -d '{"username":"audit018-admin","password":"<password>"}' \
     $GP/auth/login
   
   grep -i "^set-cookie:" ~/gameplane-audit-018/headers-admin.txt > ~/gameplane-audit-018/session-admin.txt
   chmod 600 ~/gameplane-audit-018/session-admin.txt
   ```
   Login cost: 1 admin.

3. **Create `audit018-upg` GameServer**:
   ```sh
   MARKER=$(openssl rand -hex 16)
   echo "$MARKER" > ~/gameplane-audit-018/marker.txt
   mkdir -p ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-UPG-001
   cp ~/gameplane-audit-018/marker.txt ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-UPG-001/marker.txt
   
   cat > /tmp/audit018-upg.yaml <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: GameServer
   metadata:
     name: audit018-upg
     namespace: gameplane-games
     labels:
       gameplane.io/audit: "018"
   spec:
     templateRef:
       name: minecraft-java
     storage:
       size: 5Gi
   EOF
   
   kubectl apply -f /tmp/audit018-upg.yaml
   kubectl wait --for=condition=Ready gameserver/audit018-upg -n gameplane-games --timeout=300s
   ```
   Record: marker string in `evidence/INV-UPG-001/marker.txt`.

4. **Create marker file in pod**:
   ```sh
   kubectl exec -n gameplane-games audit018-upg-0 -- \
     sh -c "echo '$MARKER' > /data/audit018-marker"
   kubectl exec -n gameplane-games audit018-upg-0 -- cat /data/audit018-marker
   ```

5. **Verify server is accessible**:
   ```sh
   kubectl get gameserver audit018-upg -n gameplane-games -o jsonpath='{.status.phase}'
   kubectl get pod audit018-upg-0 -n gameplane-games -o jsonpath='{.status.phase}'
   ```

**Expected**

- `audit018-admin` user is created and can log in
- `audit018-upg` GameServer reaches Running phase
- Pod `audit018-upg-0` is Running with the marker file present

**Cleanup**

- GameServer and admin account are kept for subsequent tests (T061 onwards)

**Automatable?**

No. Admin account creation requires a manual `kubectl exec` of `bootstrap-admin` inside the API pod (OD-015).

---

### upgrade-to-rc

Upgrades the Gameplane Helm release from `v0.2.0-beta.8` to the release candidate version (e.g., `v0.3.0-rc.1`). Verifies that `audit018-upg` stays running and the marker file survives.

**Preconditions**

- Gameplane is at `v0.2.0-beta.8` with `audit018-upg` server running and marker file in place
- Release candidate image is published to `ghcr.io/valgulnecron/charts/gameplane:0.3.0-rc.N` and is cosign-signed
- `RC_VERSION=0.3.0-rc.1` (set as environment variable)

**Resources created**

None (modifies existing release).

**Steps**

1. **Verify current release state**:
   ```sh
   helm list -n gameplane-system | grep gameplane
   kubectl get gameserver audit018-upg -n gameplane-games -o jsonpath='{.status.phase}'
   kubectl exec -n gameplane-games audit018-upg-0 -- cat /data/audit018-marker > /tmp/marker-before.txt
   cat /tmp/marker-before.txt
   ```

2. **Verify image signature** (if available):
   ```sh
   cosign verify --key ~/Gameplane/cosign.pub \
     ghcr.io/valgulnecron/charts/gameplane:$RC_VERSION
   ```

3. **Upgrade to RC**:
   ```sh
   helm upgrade gameplane oci://ghcr.io/valgulnecron/charts/gameplane \
     --version $RC_VERSION \
     -n gameplane-system --reuse-values
   
   kubectl rollout status deployment/gameplane-api -n gameplane-system --timeout=300s
   kubectl rollout status deployment/gameplane-operator -n gameplane-system --timeout=300s
   ```

4. **Verify upgrade completed**:
   ```sh
   helm list -n gameplane-system | grep gameplane
   kubectl get deployment gameplane-api -n gameplane-system -o jsonpath='{.spec.template.spec.containers[0].image}'
   kubectl get gameserver audit018-upg -n gameplane-games -o jsonpath='{.status.phase}'
   ```

5. **Verify marker file integrity**:
   ```sh
   kubectl exec -n gameplane-games audit018-upg-0 -- cat /data/audit018-marker > /tmp/marker-after.txt
   diff /tmp/marker-before.txt /tmp/marker-after.txt
   ```

**Expected**

- Helm upgrade completes without errors
- API and operator deployments reach ready state
- `audit018-upg` server remains Running
- Marker file is byte-identical (diff shows no differences)

**Cleanup**

- Release stays at RC version for the restart test

**Automatable?**

Yes. Bucket: `upgrade`.

---

### restart

Restarts the API and operator deployments to verify that state persists across pod boundaries. The `audit018-upg` server and marker file must remain intact.

**Preconditions**

- Gameplane is at RC version with `audit018-upg` running
- Marker file is in `/data/audit018-marker`
- `~/gameplane-audit-018/session-admin.txt` holds the admin session from the `seed` step (sessions are DB-backed, so they survive this restart)

**Resources created**

None.

**Steps**

1. **Record pre-restart marker**:
   ```sh
   kubectl exec -n gameplane-games audit018-upg-0 -- cat /data/audit018-marker > /tmp/marker-pre-restart.txt
   ```

2. **Restart API deployment**:
   ```sh
   kubectl rollout restart deployment/gameplane-api -n gameplane-system
   kubectl rollout status deployment/gameplane-api -n gameplane-system --timeout=300s
   ```

3. **Restart operator deployment**:
   ```sh
   kubectl rollout restart deployment/gameplane-operator -n gameplane-system
   kubectl rollout status deployment/gameplane-operator -n gameplane-system --timeout=300s
   ```

4. **Verify state survived restart**:
   ```sh
   kubectl get gameserver audit018-upg -n gameplane-games -o jsonpath='{.status.phase}'
   kubectl exec -n gameplane-games audit018-upg-0 -- cat /data/audit018-marker > /tmp/marker-post-restart.txt
   diff /tmp/marker-pre-restart.txt /tmp/marker-post-restart.txt
   ```

5. **Verify API is responsive**:
   ```sh
   SESS=$(grep gameplane_session ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   curl -s -H "Cookie: gameplane_session=$SESS; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     $GP/users/me
   ```

**Expected**

- Deployments reach ready state within timeout
- `audit018-upg` remains Running
- Marker file is byte-identical after restart

**Cleanup**

- Release stays at RC version for the rollback test

**Automatable?**

Yes. Bucket: `upgrade`.

---

### rollback

Rolls back the Gameplane Helm release from the RC to `v0.2.0-beta.8`. Verifies that the previous release serves logins, lists the `audit018-upg` server, and the marker is intact. If new database migrations were applied during the upgrade, the SQLite snapshot must be restored before the API restarts.

**Preconditions**

- Gameplane is at RC version
- Pre-upgrade database snapshot is saved at `~/gameplane-audit-018/db-snapshots/upg-real.db`
- Helm release history includes the beta.8 revision (usually revision 1 or earlier)
- Admin credentials from seed step are available

**Resources created**

- `audit018-db-tool` Pod (ephemeral; created and deleted within Step 3, only when the snapshot restore runs) — mounts the `gameplane-api-data` PVC directly, since the API container has no `sqlite3` binary or shell to exec into.

**Steps**

1. **Check Helm release history**:
   ```sh
   helm history gameplane -n gameplane-system
   ```
   Note the REVISION number for the `v0.2.0-beta.8` deployment (typically the lowest revision).

2. **Scale API to 0** (in case we need to restore the snapshot):
   ```sh
   kubectl scale deployment gameplane-api --replicas=0 -n gameplane-system
   kubectl wait --for=delete pod -l app.kubernetes.io/name=gameplane-api -n gameplane-system --timeout=60s || true
   ```

3. **Restore the snapshot** (if RC applied new migrations). The API container has no `sqlite3` binary or shell, so a temporary pod mounts the `gameplane-api-data` PVC directly and the raw `.backup` file is copied straight onto it — no SQL replay involved:
   ```sh
   cat > /tmp/audit018-db-tool.yaml <<EOF
   apiVersion: v1
   kind: Pod
   metadata:
     name: audit018-db-tool
     namespace: gameplane-system
     labels:
       gameplane.io/audit: "018"
   spec:
     restartPolicy: Never
     containers:
       - name: tool
         image: alpine:3.20
         command: ["sleep", "300"]
         volumeMounts:
           - name: data
             mountPath: /data
     volumes:
       - name: data
         persistentVolumeClaim:
           claimName: gameplane-api-data
   EOF
   kubectl apply -f /tmp/audit018-db-tool.yaml
   kubectl wait --for=condition=Ready pod/audit018-db-tool -n gameplane-system --timeout=60s

   # Drop stale WAL/SHM files so they don't shadow the restored file
   kubectl exec -n gameplane-system audit018-db-tool -- sh -c "rm -f /data/gameplane.db-wal /data/gameplane.db-shm"
   kubectl cp ~/gameplane-audit-018/db-snapshots/upg-real.db gameplane-system/audit018-db-tool:/data/gameplane.db

   kubectl delete pod audit018-db-tool -n gameplane-system --wait=true
   ```

4. **Perform rollback**:
   ```sh
   helm rollback gameplane <REVISION> -n gameplane-system
   ```
   Where `<REVISION>` is the beta.8 revision from step 1.

5. **Scale API back up**:
   ```sh
   kubectl scale deployment gameplane-api --replicas=1 -n gameplane-system
   kubectl rollout status deployment/gameplane-api -n gameplane-system --timeout=300s
   ```

6. **Verify beta.8 is serving**:
   ```sh
   helm list -n gameplane-system | grep gameplane
   kubectl get deployment gameplane-api -n gameplane-system -o jsonpath='{.spec.template.spec.containers[0].image}'
   ```

7. **Test login as admin**:
   ```sh
   curl -s -D ~/gameplane-audit-018/headers-admin-rollback.txt \
     -H "Content-Type: application/json" \
     -d '{"username":"audit018-admin","password":"<password>"}' \
     $GP/auth/login
   
   # Extract session tokens if login succeeds
   SESS=$(grep gameplane_session ~/gameplane-audit-018/headers-admin-rollback.txt | cut -d'=' -f2 | cut -d';' -f1)
   CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/headers-admin-rollback.txt | cut -d'=' -f2 | cut -d';' -f1)
   ```
   Login cost: 1 admin.

8. **Verify `audit018-upg` is listed**:
   ```sh
   curl -s -H "Cookie: gameplane_session=$SESS; gameplane_csrf=$CSRF" \
     $GP/servers | grep -q audit018-upg && echo "Found audit018-upg"
   ```

9. **Verify marker file**:
   ```sh
   kubectl exec -n gameplane-games audit018-upg-0 -- cat /data/audit018-marker > /tmp/marker-after-rollback.txt
   diff /tmp/marker-before.txt /tmp/marker-after-rollback.txt
   ```

**Expected**

- Rollback completes without errors
- API pod reaches ready state
- Admin login succeeds
- `audit018-upg` server is listed in API response
- Marker file is byte-identical to pre-upgrade version

**Cleanup**

- Release is now at beta.8 version
- Marker file is preserved for restore-real-db test

**Automatable?**

No. Steps 1-2 and 4-9 are plain kubectl/API calls; step 3 (database restore) needs the manual `kubectl apply`/`kubectl cp` sequence against a temporary pod, since the API container has no `sqlite3` binary (OD-018). Proposed bucket once step 3 is scripted: `upgrade`.

---

### restore-real-db

Restores the pre-upgrade snapshot that was taken at the beginning of baseline-beta8 to verify that real production data can be recovered cleanly. Runs `snapshot-diff` to verify that the baseline state is restored.

**Preconditions**

- Release is at `v0.2.0-beta.8` (from rollback step)
- API is running and responsive
- Pre-upgrade snapshot is saved at `~/gameplane-audit-018/db-snapshots/upg-real.db`
- A baseline snapshot was captured in `audit/evidence/baseline/`

**Resources created**

- `audit018-db-tool` Pod (ephemeral; created and deleted within Step 2) — mounts the `gameplane-api-data` PVC directly, since the API container has no `sqlite3` binary or shell to exec into.
- `~/gameplane-audit-018/snapshot-after-restore/` snapshot directory (off-git, for the `snapshot-diff` comparison in Step 4).

**Steps**

1. **Scale API to 0**:
   ```sh
   kubectl scale deployment gameplane-api --replicas=0 -n gameplane-system
   kubectl wait --for=delete pod -l app.kubernetes.io/name=gameplane-api -n gameplane-system --timeout=60s || true
   ```

2. **Restore the real database snapshot**. The pod scaled down in Step 1 no longer exists to `exec` into (and the API container has no `sqlite3` binary or shell anyway), so a temporary pod mounts the `gameplane-api-data` PVC (`/data`, per `values.yaml`'s `api.db.dsn`) directly and the raw `.backup` file is copied straight onto it:
   ```sh
   cat > /tmp/audit018-db-tool.yaml <<EOF
   apiVersion: v1
   kind: Pod
   metadata:
     name: audit018-db-tool
     namespace: gameplane-system
     labels:
       gameplane.io/audit: "018"
   spec:
     restartPolicy: Never
     containers:
       - name: tool
         image: alpine:3.20
         command: ["sleep", "300"]
         volumeMounts:
           - name: data
             mountPath: /data
     volumes:
       - name: data
         persistentVolumeClaim:
           claimName: gameplane-api-data
   EOF
   kubectl apply -f /tmp/audit018-db-tool.yaml
   kubectl wait --for=condition=Ready pod/audit018-db-tool -n gameplane-system --timeout=60s

   kubectl exec -n gameplane-system audit018-db-tool -- sh -c "rm -f /data/gameplane.db-wal /data/gameplane.db-shm"
   kubectl cp ~/gameplane-audit-018/db-snapshots/upg-real.db gameplane-system/audit018-db-tool:/data/gameplane.db

   kubectl delete pod audit018-db-tool -n gameplane-system --wait=true
   ```

3. **Scale API back up**:
   ```sh
   kubectl scale deployment gameplane-api --replicas=1 -n gameplane-system
   kubectl rollout status deployment/gameplane-api -n gameplane-system --timeout=300s
   ```

4. **Run snapshot-diff against baseline**. `snapshot-diff.sh` compares two snapshot *directories* (each produced by `snapshot.sh`), not the sqlite file itself, and both tools live under `specs/018-v0-3-release-readiness/audit/tools/`:
   ```sh
   mkdir -p ~/gameplane-audit-018/snapshot-after-restore
   bash ~/Gameplane/specs/018-v0-3-release-readiness/audit/tools/snapshot.sh \
     ~/gameplane-audit-018/snapshot-after-restore

   mkdir -p ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-UPG-005
   bash ~/Gameplane/specs/018-v0-3-release-readiness/audit/tools/snapshot-diff.sh \
     ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/baseline \
     ~/gameplane-audit-018/snapshot-after-restore \
     | tee ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-UPG-005/snapshot-diff.txt
   ```
   Record: diff output in `evidence/INV-UPG-005/`.

5. **Verify API is responsive**. The session cookie is set in the response *headers*, not the JSON body, so check the HTTP status instead of grepping the body for a cookie name:
   ```sh
   STATUS=$(curl -s -o /dev/null -w '%{http_code}' $GP/auth/login -H "Content-Type: application/json" \
     -d '{"username":"audit018-admin","password":"<password>"}')
   [ "$STATUS" = "200" ] && echo "API OK"
   ```

**Expected**

- Real database snapshot is restored without corruption
- API pod reaches ready state
- `snapshot-diff` shows no unexpected schema or state differences from the baseline
- API is responsive to login requests

**Cleanup**

- Database snapshot can now be deleted: `rm ~/gameplane-audit-018/db-snapshots/upg-real.db`
- `audit018-upg` GameServer and admin account are cleaned up by the next test round or by audit cleanup

**Automatable?**

No. Database restore requires a manual `kubectl apply`/`kubectl cp` sequence against a temporary pod, since the API container has no `sqlite3` binary (OD-018).

