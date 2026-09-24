# Procedures: UPG

Shared conventions: [conventions.md](../conventions.md).

## baseline-beta8

This establishes the upgrade baseline at `v0.2.0-beta.8` as required by FR-015. If kubelab's API database schema is already ahead of beta.8, it uninstalls and reinstalls at beta.8 with a fresh database; otherwise it upgrades in place. The real database is snapshotted before any changes for later restoration.

**Preconditions**

- `kubelab-baseline.md` has been captured
- `kubectl get nodes` lists `kubelab-control`, `kubelab-worker-1`, `kubelab-worker-2` in Ready state
- Current Helm release is recorded in `helm get values gameplane -n gameplane-system`
- Check kubelab's API database migration level: `kubectl exec -n gameplane-system <api-pod-name> -- sqlite3 /app/data/gameplane.db "SELECT MAX(name) FROM migrations;" > ~/gameplane-audit-018/db-level-before.txt`

**Resources created**

None (this section only modifies the existing `gameplane` Helm release).

**Steps**

1. **Snapshot the real database** (off-git):
   ```sh
   mkdir -p ~/gameplane-audit-018/db-snapshots
   kubectl exec -n gameplane-system <api-pod-name> -- sqlite3 /app/data/gameplane.db ".backup /tmp/upg-real.db"
   kubectl cp gameplane-system/<api-pod-name>:/tmp/upg-real.db ~/gameplane-audit-018/db-snapshots/upg-real.db
   kubectl exec -n gameplane-system <api-pod-name> -- rm /tmp/upg-real.db
   ls -lh ~/gameplane-audit-018/db-snapshots/upg-real.db
   ```
   Record: pre-upgrade snapshot size and timestamp in `rounds.md`.

2. **Check migration level** against `v0.2.0-beta.8` (ships `006_share_links.sql`):
   ```sh
   cat ~/gameplane-audit-018/db-level-before.txt
   ```
   If result is `006_share_links.sql` or earlier, go to step 4 (upgrade in place).
   If result is `007_*` or later, go to step 3 (reinstall).

3. **Reinstall at beta.8** (if ahead):
   ```sh
   helm uninstall gameplane -n gameplane-system --keep-history
   # Wait for API pod to terminate
   kubectl wait --for=delete pod -l app.kubernetes.io/name=gameplane-api -n gameplane-system --timeout=60s || true
   
   helm upgrade gameplane oci://ghcr.io/valgulnecron/charts/gameplane \
     --version 0.2.0-beta.8 \
     -n gameplane-system --reuse-values
   
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

No. Requires manual database snapshot via `kubectl exec` (OD-018).

---

## seed

Creates an `audit018-upg` GameServer with a marker file to verify persistence through upgrade and rollback cycles. Also creates the `audit018-admin` account for subsequent API tests.

**Preconditions**

- Gameplane is running at `v0.2.0-beta.8` (from baseline-beta8 step)
- Admin account does not yet exist (OD-015)
- `GP=http://127.0.0.1:18080` (port-forward running)

**Resources created**

- `audit018-upg` GameServer (and its PVC)
- `audit018-admin` user account (via API, not kubectl)

**Steps**

1. **Create admin account** (pending OD-015):
   ```sh
   # Via dashboard or API bootstrap command
   # Option (a): create in dashboard and record password in ~/gameplane-audit-018/admin.env (mode 600)
   # Option (b): kubectl exec -n gameplane-system <api-pod-name> -- gameplane-api bootstrap-admin --username audit018-admin
   ```
   Record: admin password location in `rounds.md`.

2. **Log in as admin**:
   ```sh
   curl -s -D ~/gameplane-audit-018/headers-admin.txt \
     -H "Content-Type: application/json" \
     -d '{"username":"audit018-admin","password":"<password>"}' \
     $GP/auth/login
   
   grep "^set-cookie:" ~/gameplane-audit-018/headers-admin.txt > ~/gameplane-audit-018/session-admin.txt
   chmod 600 ~/gameplane-audit-018/session-admin.txt
   ```
   Login cost: 1 admin.

3. **Create `audit018-upg` GameServer**:
   ```sh
   MARKER=$(openssl rand -hex 16)
   echo "$MARKER" > ~/gameplane-audit-018/marker.txt
   
   cat > /tmp/audit018-upg.yaml <<EOF
   apiVersion: gameplane.io/v1alpha1
   kind: GameServer
   metadata:
     name: audit018-upg
     namespace: gameplane-games
     labels:
       gameplane.io/audit: "018"
   spec:
     template: minecraft-vanilla
     replicas: 1
     storage:
       gamedata:
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

No. Admin account creation is pending OD-015 (manual via dashboard or `bootstrap-admin` command).

---

## upgrade-to-rc

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

## restart

Restarts the API and operator deployments to verify that state persists across pod boundaries. The `audit018-upg` server and marker file must remain intact.

**Preconditions**

- Gameplane is at RC version with `audit018-upg` running
- Marker file is in `/data/audit018-marker`

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
   curl -s -H "Cookie: gameplane_session=$SESS; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     $GP/api/user
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

## rollback

Rolls back the Gameplane Helm release from the RC to `v0.2.0-beta.8`. Verifies that the previous release serves logins, lists the `audit018-upg` server, and the marker is intact. If new database migrations were applied during the upgrade, the SQLite snapshot must be restored before the API restarts.

**Preconditions**

- Gameplane is at RC version
- Pre-upgrade database snapshot is saved at `~/gameplane-audit-018/db-snapshots/upg-real.db`
- Helm release history includes the beta.8 revision (usually revision 1 or earlier)
- Admin credentials from seed step are available

**Resources created**

None.

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

3. **Restore the snapshot** (if RC applied new migrations):
   ```sh
   # Get the PVC name for the API database
   PVC=$(kubectl get pvc -n gameplane-system -o name | grep api-data | head -1)
   
   # Mount the PVC and restore the backup
   kubectl exec -n gameplane-system -it <api-pod-name> -- sqlite3 /app/data/gameplane.db < ~/gameplane-audit-018/db-snapshots/upg-real.db
   # Alternative: restore via file copy if snapshot-diff needs the old schema
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
     $GP/api/gameservers | grep -q audit018-upg && echo "Found audit018-upg"
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

Partial. Steps 1-5 and 7-9 are automatable (bucket: `upgrade`). Step 3 (database restore) is conditional and pending OD-018.

---

## restore-real-db

Restores the pre-upgrade snapshot that was taken at the beginning of baseline-beta8 to verify that real production data can be recovered cleanly. Runs `snapshot-diff` to verify that the baseline state is restored.

**Preconditions**

- Release is at `v0.2.0-beta.8` (from rollback step)
- API is running and responsive
- Pre-upgrade snapshot is saved at `~/gameplane-audit-018/db-snapshots/upg-real.db`
- A baseline snapshot was captured in `audit/evidence/baseline/`

**Resources created**

None.

**Steps**

1. **Scale API to 0**:
   ```sh
   kubectl scale deployment gameplane-api --replicas=0 -n gameplane-system
   kubectl wait --for=delete pod -l app.kubernetes.io/name=gameplane-api -n gameplane-system --timeout=60s || true
   ```

2. **Restore the real database snapshot**:
   ```sh
   # Get API PVC location; depends on chart settings but typically /app/data
   kubectl exec -n gameplane-system <api-pod-name-before-scale> -- \
     sqlite3 /app/data/gameplane.db < ~/gameplane-audit-018/db-snapshots/upg-real.db
   ```

3. **Scale API back up**:
   ```sh
   kubectl scale deployment gameplane-api --replicas=1 -n gameplane-system
   kubectl rollout status deployment/gameplane-api -n gameplane-system --timeout=300s
   ```

4. **Run snapshot-diff against baseline**:
   ```sh
   bash ~/Gameplane/audit/tools/snapshot-diff.sh \
     ~/gameplane-audit-018/db-snapshots/upg-real.db \
     ~/Gameplane/audit/evidence/baseline/api-schema.json
   ```
   Record: diff output in `evidence/INV-UPG-005/`.

5. **Verify API is responsive**:
   ```sh
   curl -s $GP/auth/login -H "Content-Type: application/json" \
     -d '{"username":"audit018-admin","password":"<password>"}' \
     | grep -q "gameplane_session" && echo "API OK"
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

No. Database restore via `sqlite3 < file` is pending OD-018 (manual operation).

