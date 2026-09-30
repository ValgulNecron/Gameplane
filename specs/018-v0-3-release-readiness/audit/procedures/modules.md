# Procedures: MOD

Shared conventions: [conventions.md](conventions.md).

**Restic fixture (OD-021 items 9/16, shared by every Backup/Restore step below):** kubelab has no restic repository, and the e2e fixture Secret `e2e-restic-creds` doesn't exist here. `test/e2e/fixtures/restic-server.yaml` has three objects: the Deployment `gameplane-test-restic` and Service `gameplane-test-restic` (both in `gameplane-system`), plus a NetworkPolicy `allow-egress-to-restic` in `gameplane-games` whose top-level `spec.podSelector.matchLabels` is `app.kubernetes.io/name: gameplane-backup-restore` (the label the operator stamps on Backup/Restore Job pods, per `backup_controller.go`'s `backupRestoreJobValue` constant), scoping the policy to those Job pods rather than every pod in the namespace; the label `app.kubernetes.io/name: gameplane-test-restic` appears only in `spec.egress[0].to[0].podSelector.matchLabels`, alongside a `namespaceSelector` matching `gameplane-system`, i.e. it scopes the *egress destination* (the restic pod being dialed), not which pods the policy applies to. Do not apply the file as-is: besides renaming `metadata.name`, the Deployment's `spec.selector`/pod-template labels, the Service's `spec.selector`, and the NetworkPolicy's egress peer `podSelector.matchLabels` all carry `app.kubernetes.io/name: gameplane-test-restic`, which must become `app.kubernetes.io/name: audit018-restic` everywhere it appears, or the Deployment's pods and the Service/NetworkPolicy egress selectors stop matching each other. Apply it as follows:
- Deployment and Service, both in `gameplane-system`, `metadata.name: audit018-restic`, `app.kubernetes.io/name: audit018-restic` in the Deployment's `spec.selector.matchLabels`, pod template `metadata.labels`, and the Service's `spec.selector`. Both labelled `gameplane.io/audit: "018"`.
- NetworkPolicy: only create it if egress is enforced on kubelab. Name it `audit018-restic` (not `allow-egress-to-restic`, which is the pre-existing e2e fixture's name and must never be reused or written to), label it `gameplane.io/audit: "018"`. Keep `spec.podSelector.matchLabels` as `app.kubernetes.io/name: gameplane-backup-restore` unchanged — do not rename it, since it is not the fixture's `gameplane-test-restic` label; it matches the label the operator stamps on Backup/Restore Job pods (`backup_controller.go`'s `backupRestoreJobValue`), the same label the chart's own `allow-backup-restore-egress` policy selects, and repointing it would scope the policy away from the Backup pods it must cover. Rename only the egress peer selector, `spec.egress[0].to[0].podSelector.matchLabels`, to `app.kubernetes.io/name: audit018-restic`, so the allowed egress destination is the audit018-restic pod, not the e2e fixture's.

Then create a matching `audit018-restic` Secret in `gameplane-games` (based on `test/e2e/fixtures/backup-restic-secret.yaml`, same `gameplane.io/audit: "018"` label) with:
```yaml
stringData:
  repo: "rest:http://audit018-restic.gameplane-system.svc:8000/"
  password: "<round-local value, not committed>"
```
Every `repoRef.name` below is `audit018-restic`, not the e2e fixture's `e2e-restic-creds`. The Deployment/Service, the (if created) NetworkPolicy `audit018-restic`, and the Secret are all per-round: remove them at round teardown, alongside the last module procedure's cleanup.

### garrys-mod

**Preconditions:** The `gameplane-games` namespace exists. The `audit018-admin` role has session cookie set in `~/gameplane-audit-018/session-admin.txt`. The module `garrys-mod` is installed in the cluster (via ModuleSource `default`).

**Resources created:** `audit018-garrys-mod` (GameServer).

**Steps:**

1. **Create server from template.** Login cost: 0 (reuse session).
   ```bash
   export GP=http://127.0.0.1:18080
   SESS=$(grep gameplane_session ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   COOKIE="gameplane_session=$SESS; gameplane_csrf=$CSRF"
   curl -X POST "$GP/servers/" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)" \
     -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-garrys-mod"},"spec":{"templateRef":{"name":"garrys-mod"}}}'
   ```

2. **Start server.** Poll until status.phase = "Running" with 2m timeout.
   ```bash
   curl -X POST "$GP/servers/audit018-garrys-mod:start" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)"
   ```

3. **Protocol join.** Run test/e2e/internal/garrys-mod/app.go (e2e-probe:steam-a2s-udp) against service IP:27015. `kubectl port-forward` cannot carry the advertised UDP `game` port (`modules/garrys-mod/template.yaml`), and this category has no console to route through instead (`rcon.protocol: none`, `consoleMode` unset), so the probe has to run in-cluster and dial the Service directly — the same way `test/e2e/gameprobe_job.go`'s `RunGameProbe` runs every A2S-family probe as an in-cluster Job, never through port-forward.
   ```bash
   kubectl get svc -n gameplane-games audit018-garrys-mod -o jsonpath='{.spec.clusterIP}'
   # Run the probe from inside the cluster (a short-lived audit018- Job/Pod, deleted afterwards) against <clusterIP>:27015, not via kubectl port-forward.
   # Run it outside gameplane-games (RunGameProbe uses the `default` namespace, test/e2e/gameprobe_job.go):
   # the chart's default-deny-egress policy in gameplane-games allows only DNS egress to kube-system, so a probe pod there cannot reach UDP 27015.
   ```

4. **Console command (none family).** This category has consoleMode=none, so no console is available. Skip this step.

5. **Backup.** Create a Backup CR. The `audit018-restic` repository (see the Restic fixture note above) must already be up for the round.
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-garrys-mod-bk
     namespace: gameplane-games
   spec:
     serverRef:
       name: audit018-garrys-mod
     repoRef:
       name: audit018-restic
       key: repo
     strategy: restic-snapshot
     quiesce: false
   EOF
   ```

6. **Restore.** Create a Restore CR pointing to the backup.
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Restore
   metadata:
     name: audit018-garrys-mod-rs
     namespace: gameplane-games
   spec:
     backupRef:
       name: audit018-garrys-mod-bk
     serverRef:
       name: audit018-garrys-mod
   EOF
   ```

7. **Delete.** Via API.
   ```bash
   curl -X DELETE "$GP/servers/audit018-garrys-mod" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)"
   ```

**Expected:** Server created, started, probe reaches QUERY depth (server is visible and queryable), backup succeeds with phase=Succeeded, restore completes, deletion removes the resource.

**Cleanup:** `kubectl delete gameserver -n gameplane-games audit018-garrys-mod; kubectl delete backup,restore -n gameplane-games audit018-garrys-mod-bk audit018-garrys-mod-rs` (if backups/restores were created).

**Automatable?** Yes. Propose bucket: `api-mods`.

---

### farming-simulator-25

**Preconditions:** The `gameplane-games` namespace exists. The `audit018-admin` role has session cookie set. The module `farming-simulator-25` is installed.

**Resources created:** `audit018-farming-simulator-25` (GameServer).

**Steps:**

1. **Create server from template.**
   ```bash
   export GP=http://127.0.0.1:18080
   SESS=$(grep gameplane_session ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   COOKIE="gameplane_session=$SESS; gameplane_csrf=$CSRF"
   curl -X POST "$GP/servers/" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)" \
     -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-farming-simulator-25"},"spec":{"templateRef":{"name":"farming-simulator-25"}}}'
   ```

2. **Start server.** Poll until phase=Running, 3m timeout.

3. **Protocol join.** Run test/e2e/internal/farming-simulator-25/app.go (e2e-probe:http-rest) against the service IP:8080.

4. **Console command (none/rest family).** `consoleMode` is `none`, so there is no free-form console; `farming-simulator-25`'s only declared action is `save-world` (`modules/farming-simulator-25/template.yaml`'s `capabilities.actions` — `/servers/{name}/actions/run` rejects any other `id` as `unknown action`, `api/internal/ws/actions.go`). Run that action and check its effect rather than querying a status action that doesn't exist:
   ```bash
   curl -X POST "$GP/servers/audit018-farming-simulator-25/actions/run" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)" \
     -H "Content-Type: application/json" \
     -d '{"id":"save-world"}'
   ```
   Expect `{"ok":true,...}`; confirm the save actually happened (a new/updated save file under the server's storage, or the action's `raw` response).

5. **Backup.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-farming-simulator-25-bk
     namespace: gameplane-games
   spec:
     serverRef:
       name: audit018-farming-simulator-25
     repoRef:
       name: audit018-restic
       key: repo
     strategy: restic-snapshot
     quiesce: false
   EOF
   ```

6. **Restore.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Restore
   metadata:
     name: audit018-farming-simulator-25-rs
     namespace: gameplane-games
   spec:
     backupRef:
       name: audit018-farming-simulator-25-bk
     serverRef:
       name: audit018-farming-simulator-25
   EOF
   ```

7. **Delete.**
   ```bash
   curl -X DELETE "$GP/servers/audit018-farming-simulator-25" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)"
   ```

**Expected:** Server created, started, HTTP probe succeeds, `save-world` action returns `ok: true` and the save file updates, backup and restore complete, deletion removes resource.

**Cleanup:** `kubectl delete gameserver -n gameplane-games audit018-farming-simulator-25; kubectl delete backup,restore -n gameplane-games audit018-farming-simulator-25-bk audit018-farming-simulator-25-rs`.

**Automatable?** Yes. Propose bucket: `api-mods`.

---

### beammp

**Preconditions:** The `gameplane-games` namespace exists. The `audit018-admin` role has session cookie set. The module `beammp` is installed.

**Resources created:** `audit018-beammp` (GameServer).

**Steps:**

1. **Create server from template.**
   ```bash
   export GP=http://127.0.0.1:18080
   SESS=$(grep gameplane_session ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   COOKIE="gameplane_session=$SESS; gameplane_csrf=$CSRF"
   curl -X POST "$GP/servers/" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)" \
     -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-beammp"},"spec":{"templateRef":{"name":"beammp"}}}'
   ```

2. **Start server.** Poll until phase=Running, 2m timeout.

3. **Protocol join.** Run test/e2e/internal/beammp/app.go (e2e-probe:beammp-tcp) against service IP:30814.

4. **Console command (pty/none family).** PTY console has no agent involvement and no service port: connect via the API's pod-attach WebSocket instead (`ws://127.0.0.1:18080/ws/servers/audit018-beammp/console-pty`), which bridges directly to the container's stdin/stdout through the Kubernetes API (see agent/internal/console/console.go's package comment and api/internal/ws/attach.go).

5. **Backup.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-beammp-bk
     namespace: gameplane-games
   spec:
     serverRef:
       name: audit018-beammp
     repoRef:
       name: audit018-restic
       key: repo
     strategy: restic-snapshot
     quiesce: false
   EOF
   ```

6. **Restore.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Restore
   metadata:
     name: audit018-beammp-rs
     namespace: gameplane-games
   spec:
     backupRef:
       name: audit018-beammp-bk
     serverRef:
       name: audit018-beammp
   EOF
   ```

7. **Delete.**
   ```bash
   curl -X DELETE "$GP/servers/audit018-beammp" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)"
   ```

**Expected:** Server created, started, TCP probe succeeds, PTY console accessible, backup and restore complete, deletion removes resource.

**Cleanup:** `kubectl delete gameserver -n gameplane-games audit018-beammp; kubectl delete backup,restore -n gameplane-games audit018-beammp-bk audit018-beammp-rs`.

**Automatable?** Yes. Propose bucket: `api-mods`.

---

### valheim

**Preconditions:** The `gameplane-games` namespace exists. The `audit018-admin` role has session cookie set. The module `valheim` is installed.

**Resources created:** `audit018-valheim` (GameServer).

**Steps:**

1. **Create server from template.**
   ```bash
   export GP=http://127.0.0.1:18080
   SESS=$(grep gameplane_session ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   COOKIE="gameplane_session=$SESS; gameplane_csrf=$CSRF"
   curl -X POST "$GP/servers/" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)" \
     -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-valheim"},"spec":{"templateRef":{"name":"valheim"}}}'
   ```

2. **Start server.** Poll until phase=Running, 2m timeout.

3. **Protocol join.** Run test/e2e/internal/valheim/app.go (e2e-probe:http-rest) against service IP:80.

4. **Console command (pty/none family).** Connect via PTY.

5. **Backup.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-valheim-bk
     namespace: gameplane-games
   spec:
     serverRef:
       name: audit018-valheim
     repoRef:
       name: audit018-restic
       key: repo
     strategy: restic-snapshot
     quiesce: false
   EOF
   ```

6. **Restore.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Restore
   metadata:
     name: audit018-valheim-rs
     namespace: gameplane-games
   spec:
     backupRef:
       name: audit018-valheim-bk
     serverRef:
       name: audit018-valheim
   EOF
   ```

7. **Delete.**
   ```bash
   curl -X DELETE "$GP/servers/audit018-valheim" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)"
   ```

**Expected:** Server created, started, HTTP probe succeeds, PTY console accessible, backup and restore complete, deletion removes resource.

**Cleanup:** `kubectl delete gameserver -n gameplane-games audit018-valheim; kubectl delete backup,restore -n gameplane-games audit018-valheim-bk audit018-valheim-rs`.

**Automatable?** Yes. Propose bucket: `api-mods`.

---

### dont-starve-together

**Preconditions:** The `gameplane-games` namespace exists. The `audit018-admin` role has session cookie set. The module `dont-starve-together` is installed.

**Resources created:** `audit018-dont-starve-together` (GameServer).

**Steps:**

1. **Create server from template.**
   ```bash
   export GP=http://127.0.0.1:18080
   SESS=$(grep gameplane_session ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   COOKIE="gameplane_session=$SESS; gameplane_csrf=$CSRF"
   curl -X POST "$GP/servers/" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)" \
     -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-dont-starve-together"},"spec":{"templateRef":{"name":"dont-starve-together"}}}'
   ```

2. **Start server.** Poll until phase=Running, 2m timeout.

3. **Protocol join.** Run test/e2e/internal/dont-starve-together/app.go (e2e-probe:steam-a2s-udp) against service IP:27018.

4. **Console command (pty/none family).** Connect via PTY.

5. **Backup.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-dont-starve-together-bk
     namespace: gameplane-games
   spec:
     serverRef:
       name: audit018-dont-starve-together
     repoRef:
       name: audit018-restic
       key: repo
     strategy: restic-snapshot
     quiesce: false
   EOF
   ```

6. **Restore.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Restore
   metadata:
     name: audit018-dont-starve-together-rs
     namespace: gameplane-games
   spec:
     backupRef:
       name: audit018-dont-starve-together-bk
     serverRef:
       name: audit018-dont-starve-together
   EOF
   ```

7. **Delete.**
   ```bash
   curl -X DELETE "$GP/servers/audit018-dont-starve-together" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)"
   ```

**Expected:** Server created, started, A2S probe succeeds, PTY console accessible, backup and restore complete, deletion removes resource.

**Cleanup:** `kubectl delete gameserver -n gameplane-games audit018-dont-starve-together; kubectl delete backup,restore -n gameplane-games audit018-dont-starve-together-bk audit018-dont-starve-together-rs`.

**Automatable?** Yes. Propose bucket: `api-mods`.

---

### tmodloader

**Preconditions:** The `gameplane-games` namespace exists. The `audit018-admin` role has session cookie set. The module `tmodloader` is installed.

**Resources created:** `audit018-tmodloader` (GameServer).

**Steps:**

1. **Create server from template.**
   ```bash
   export GP=http://127.0.0.1:18080
   SESS=$(grep gameplane_session ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   COOKIE="gameplane_session=$SESS; gameplane_csrf=$CSRF"
   curl -X POST "$GP/servers/" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)" \
     -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-tmodloader"},"spec":{"templateRef":{"name":"tmodloader"}}}'
   ```

2. **Start server.** Poll until phase=Running, 2m timeout.

3. **Protocol join.** Run test/e2e/internal/tmodloader/app.go (e2e-probe:terraria-tcp) against service IP:7777.

4. **Console command (pty/none family).** Connect via PTY.

5. **Backup.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-tmodloader-bk
     namespace: gameplane-games
   spec:
     serverRef:
       name: audit018-tmodloader
     repoRef:
       name: audit018-restic
       key: repo
     strategy: restic-snapshot
     quiesce: false
   EOF
   ```

6. **Restore.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Restore
   metadata:
     name: audit018-tmodloader-rs
     namespace: gameplane-games
   spec:
     backupRef:
       name: audit018-tmodloader-bk
     serverRef:
       name: audit018-tmodloader
   EOF
   ```

7. **Delete.**
   ```bash
   curl -X DELETE "$GP/servers/audit018-tmodloader" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)"
   ```

**Expected:** Server created, started, Terraria TCP probe succeeds, PTY console accessible, backup and restore complete, deletion removes resource.

**Cleanup:** `kubectl delete gameserver -n gameplane-games audit018-tmodloader; kubectl delete backup,restore -n gameplane-games audit018-tmodloader-bk audit018-tmodloader-rs`.

**Automatable?** Yes. Propose bucket: `api-mods`.

---

### terraria

**Preconditions:** The `gameplane-games` namespace exists. The `audit018-admin` role has session cookie set. The module `terraria` is installed.

**Resources created:** `audit018-terraria` (GameServer).

**Steps:**

1. **Create server from template.**
   ```bash
   export GP=http://127.0.0.1:18080
   SESS=$(grep gameplane_session ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   COOKIE="gameplane_session=$SESS; gameplane_csrf=$CSRF"
   curl -X POST "$GP/servers/" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)" \
     -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-terraria"},"spec":{"templateRef":{"name":"terraria"}}}'
   ```

2. **Start server.** Poll until phase=Running, 1m timeout.

3. **Protocol join.** Run test/e2e/internal/terraria/app.go (gameproto:terraria wire protocol) against service IP:7777. Use the gameproto Terraria handshake parser in gameproto/terraria.go.

4. **Console command (pty/none family).** Connect via PTY.

5. **Backup.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-terraria-bk
     namespace: gameplane-games
   spec:
     serverRef:
       name: audit018-terraria
     repoRef:
       name: audit018-restic
       key: repo
     strategy: restic-snapshot
     quiesce: false
   EOF
   ```

6. **Restore.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Restore
   metadata:
     name: audit018-terraria-rs
     namespace: gameplane-games
   spec:
     backupRef:
       name: audit018-terraria-bk
     serverRef:
       name: audit018-terraria
   EOF
   ```

7. **Delete.**
   ```bash
   curl -X DELETE "$GP/servers/audit018-terraria" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)"
   ```

**Expected:** Server created, started, gameproto Terraria handshake succeeds, PTY console accessible, backup and restore complete, deletion removes resource.

**Cleanup:** `kubectl delete gameserver -n gameplane-games audit018-terraria; kubectl delete backup,restore -n gameplane-games audit018-terraria-bk audit018-terraria-rs`.

**Automatable?** Yes. Propose bucket: `api-mods`.

---

### factorio

**Preconditions:** The `gameplane-games` namespace exists. The `audit018-admin` role has session cookie set. The module `factorio` is installed.

**Resources created:** `audit018-factorio` (GameServer).

**Steps:**

1. **Create server from template.**
   ```bash
   export GP=http://127.0.0.1:18080
   SESS=$(grep gameplane_session ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   COOKIE="gameplane_session=$SESS; gameplane_csrf=$CSRF"
   curl -X POST "$GP/servers/" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)" \
     -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-factorio"},"spec":{"templateRef":{"name":"factorio"}}}'
   ```

2. **Start server.** Poll until phase=Running, 2m timeout.

3. **Protocol join.** Run test/e2e/internal/factorio/app.go (e2e-probe:factorio-tcp) against service IP:34197.

4. **Console command (pty/source family).** The interactive console is PTY (pod-attach via `ws://127.0.0.1:18080/ws/servers/audit018-factorio/console-pty`), not RCON: RCON (source protocol) is agent-internal, backs the Players/Actions tabs only, and its port (27015) is declared `advertise: false` in modules/factorio/template.yaml, so it has no Service port to connect to.

5. **Backup.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-factorio-bk
     namespace: gameplane-games
   spec:
     serverRef:
       name: audit018-factorio
     repoRef:
       name: audit018-restic
       key: repo
     strategy: restic-snapshot
     quiesce: false
   EOF
   ```

6. **Restore.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Restore
   metadata:
     name: audit018-factorio-rs
     namespace: gameplane-games
   spec:
     backupRef:
       name: audit018-factorio-bk
     serverRef:
       name: audit018-factorio
   EOF
   ```

7. **Delete.**
   ```bash
   curl -X DELETE "$GP/servers/audit018-factorio" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)"
   ```

**Expected:** Server created, started, TCP probe succeeds, PTY + RCON console accessible, backup and restore complete, deletion removes resource.

**Cleanup:** `kubectl delete gameserver -n gameplane-games audit018-factorio; kubectl delete backup,restore -n gameplane-games audit018-factorio-bk audit018-factorio-rs`.

**Automatable?** Yes. Propose bucket: `api-mods`.

---

### dayz

**Preconditions:** The `gameplane-games` namespace exists. The `audit018-admin` role has session cookie set. The module `dayz` is installed.

**Resources created:** `audit018-dayz` (GameServer).

**Note:** This module requires 6Gi memory. It is a **blocked candidate** if kubelab nodes have insufficient capacity (max ~4Gi per node recommended). The lightest alternative in the rcon/battleye + e2e-probe:steam-a2s-udp category is this module itself (only one in category); no lighter alternative available. If memory is insufficient, mark outcome as `blocked` with reason: "blocked candidate: needs node memory ≥6Gi per pod; no lighter alternative in rcon/battleye category (dayz is the only module)".

**Steps:**

1. **Create server from template.**
   ```bash
   export GP=http://127.0.0.1:18080
   SESS=$(grep gameplane_session ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   COOKIE="gameplane_session=$SESS; gameplane_csrf=$CSRF"
   curl -X POST "$GP/servers/" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)" \
     -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-dayz"},"spec":{"templateRef":{"name":"dayz"}}}'
   ```

2. **Start server.** Poll until phase=Running, 3m timeout (memory-heavy).

3. **Protocol join.** Run test/e2e/internal/dayz/app.go (e2e-probe:steam-a2s-udp) against service IP:27015.

4. **Console command (rcon/battleye family).** RCON (UDP:2305) is declared `advertise: false` in modules/dayz/template.yaml, so it has no Service port; send a command over the agent's console WebSocket instead: `ws://127.0.0.1:18080/ws/servers/audit018-dayz/console` with a `{"kind":"cmd","body":"<command>"}` frame (matches procedures/agent.md#console-battleye).

5. **Backup.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-dayz-bk
     namespace: gameplane-games
   spec:
     serverRef:
       name: audit018-dayz
     repoRef:
       name: audit018-restic
       key: repo
     strategy: restic-snapshot
     quiesce: false
   EOF
   ```

6. **Restore.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Restore
   metadata:
     name: audit018-dayz-rs
     namespace: gameplane-games
   spec:
     backupRef:
       name: audit018-dayz-bk
     serverRef:
       name: audit018-dayz
   EOF
   ```

7. **Delete.**
   ```bash
   curl -X DELETE "$GP/servers/audit018-dayz" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)"
   ```

**Expected:** Server created, started, A2S probe succeeds, BattlEye RCON responds, backup and restore complete, deletion removes resource.

**Cleanup:** `kubectl delete gameserver -n gameplane-games audit018-dayz; kubectl delete backup,restore -n gameplane-games audit018-dayz-bk audit018-dayz-rs`.

**Automatable?** Yes (if memory available). Propose bucket: `api-mods`.

---

### nuclear-option

**Preconditions:** The `gameplane-games` namespace exists. The `audit018-admin` role has session cookie set. The module `nuclear-option` is installed.

**Resources created:** `audit018-nuclear-option` (GameServer).

**Note:** This module uses UDP with no automated probe (join family: "udp (no probe)"). The create/start/console/backup/restore/delete steps are automatable (OD-021 item 10); only the protocol join step (step 3) needs a manual client, since there is no e2e bot or probe for `nuclear-option` under `test/e2e/`. Manual verification via `nc -u` or inspection of pod logs is required for that step.

**Steps:**

1. **Create server from template.**
   ```bash
   export GP=http://127.0.0.1:18080
   SESS=$(grep gameplane_session ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   COOKIE="gameplane_session=$SESS; gameplane_csrf=$CSRF"
   curl -X POST "$GP/servers/" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)" \
     -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-nuclear-option"},"spec":{"templateRef":{"name":"nuclear-option"}}}'
   ```

2. **Start server.** Poll until phase=Running, 2m timeout.

3. **Protocol join.** No automated probe available. Verify server is listening: `nc -u -z <service-ip> 7778` should timeout (UDP has no handshake), but netstat on the pod should show the port open.

4. **Console command (rcon/nuclearoption family).** Port 7779 binds loopback-only inside the pod (modules/nuclear-option/template.yaml: '127.0.0.1:7779... access is pod-local only') and is never advertised on the Service; send a command over the agent's console WebSocket instead: `ws://127.0.0.1:18080/ws/servers/audit018-nuclear-option/console` with a `{"kind":"cmd","body":"<command>"}` frame.

5. **Backup.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-nuclear-option-bk
     namespace: gameplane-games
   spec:
     serverRef:
       name: audit018-nuclear-option
     repoRef:
       name: audit018-restic
       key: repo
     strategy: restic-snapshot
     quiesce: false
   EOF
   ```

6. **Restore.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Restore
   metadata:
     name: audit018-nuclear-option-rs
     namespace: gameplane-games
   spec:
     backupRef:
       name: audit018-nuclear-option-bk
     serverRef:
       name: audit018-nuclear-option
   EOF
   ```

7. **Delete.**
   ```bash
   curl -X DELETE "$GP/servers/audit018-nuclear-option" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)"
   ```

**Expected:** Server created, started, port verified open on pod, RCON responds, backup and restore complete, deletion removes resource.

**Cleanup:** `kubectl delete gameserver -n gameplane-games audit018-nuclear-option; kubectl delete backup,restore -n gameplane-games audit018-nuclear-option-bk audit018-nuclear-option-rs`.

**Automatable?** Yes (OD-021 item 10). Propose bucket: `bot-heavy` (not `api-mods`; the client join, step 3, stays manual).

---

### palworld

**Preconditions:** The `gameplane-games` namespace exists. The `audit018-admin` role has session cookie set. The module `palworld` is installed.

**Resources created:** `audit018-palworld` (GameServer).

**Steps:**

1. **Create server from template.**
   ```bash
   export GP=http://127.0.0.1:18080
   SESS=$(grep gameplane_session ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   COOKIE="gameplane_session=$SESS; gameplane_csrf=$CSRF"
   curl -X POST "$GP/servers/" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)" \
     -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-palworld"},"spec":{"templateRef":{"name":"palworld"}}}'
   ```

2. **Start server.** Poll until phase=Running, 2m timeout.

3. **Protocol join.** Run test/e2e/internal/palworld/app.go (e2e-probe:steam-a2s-udp) against service IP:27015.

4. **Console command (rcon/palworld family).** The REST API port (8212) is declared `advertise: false` in modules/palworld/template.yaml ('An admin API must never be exposed publicly'); send a command over the agent's console WebSocket instead: `ws://127.0.0.1:18080/ws/servers/audit018-palworld/console` with a `{"kind":"cmd","body":"<command>"}` frame.

5. **Backup.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-palworld-bk
     namespace: gameplane-games
   spec:
     serverRef:
       name: audit018-palworld
     repoRef:
       name: audit018-restic
       key: repo
     strategy: restic-snapshot
     quiesce: false
   EOF
   ```

6. **Restore.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Restore
   metadata:
     name: audit018-palworld-rs
     namespace: gameplane-games
   spec:
     backupRef:
       name: audit018-palworld-bk
     serverRef:
       name: audit018-palworld
   EOF
   ```

7. **Delete.**
   ```bash
   curl -X DELETE "$GP/servers/audit018-palworld" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)"
   ```

**Expected:** Server created, started, A2S probe succeeds, REST API RCON responds, backup and restore complete, deletion removes resource.

**Cleanup:** `kubectl delete gameserver -n gameplane-games audit018-palworld; kubectl delete backup,restore -n gameplane-games audit018-palworld-bk audit018-palworld-rs`.

**Automatable?** Yes. Propose bucket: `api-mods`.

---

### fivem

**Preconditions:** The `gameplane-games` namespace exists. The `audit018-admin` role has session cookie set. The module `fivem` is installed.

**Resources created:** `audit018-fivem` (GameServer).

**Steps:**

1. **Create server from template.**
   ```bash
   export GP=http://127.0.0.1:18080
   SESS=$(grep gameplane_session ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   COOKIE="gameplane_session=$SESS; gameplane_csrf=$CSRF"
   curl -X POST "$GP/servers/" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)" \
     -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-fivem"},"spec":{"templateRef":{"name":"fivem"}}}'
   ```

2. **Start server.** Poll until phase=Running, 2m timeout.

3. **Protocol join.** Run test/e2e/internal/fivem/app.go (e2e-probe:http-rest) against service IP:30120.

4. **Console command (rcon/rest family).** RCON actually runs on the txadmin port (40120, per modules/fivem/template.yaml's spec.rcon), which is declared `advertise: false`; send a command over the agent's console WebSocket instead: `ws://127.0.0.1:18080/ws/servers/audit018-fivem/console` with a `{"kind":"cmd","body":"<command>"}` frame.

5. **Backup.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-fivem-bk
     namespace: gameplane-games
   spec:
     serverRef:
       name: audit018-fivem
     repoRef:
       name: audit018-restic
       key: repo
     strategy: restic-snapshot
     quiesce: false
   EOF
   ```

6. **Restore.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Restore
   metadata:
     name: audit018-fivem-rs
     namespace: gameplane-games
   spec:
     backupRef:
       name: audit018-fivem-bk
     serverRef:
       name: audit018-fivem
   EOF
   ```

7. **Delete.**
   ```bash
   curl -X DELETE "$GP/servers/audit018-fivem" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)"
   ```

**Expected:** Server created, started, HTTP probe succeeds, REST API RCON responds, backup and restore complete, deletion removes resource.

**Cleanup:** `kubectl delete gameserver -n gameplane-games audit018-fivem; kubectl delete backup,restore -n gameplane-games audit018-fivem-bk audit018-fivem-rs`.

**Automatable?** Yes. Propose bucket: `api-mods`.

---

### satisfactory

**Preconditions:** The `gameplane-games` namespace exists. The `audit018-admin` role has session cookie set. The module `satisfactory` is installed.

**Resources created:** `audit018-satisfactory` (GameServer).

**Note:** This module requires 6Gi memory. It is a **blocked candidate** if kubelab nodes have insufficient capacity. The lightest alternative in the rcon/satisfactory + e2e-probe:http-rest category is this module itself (only one in category). However, a lighter alternative in nearby categories: `fivem` (rcon/rest + e2e-probe:http-rest, 4Gi).

**Steps:**

1. **Create server from template.**
   ```bash
   export GP=http://127.0.0.1:18080
   SESS=$(grep gameplane_session ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   COOKIE="gameplane_session=$SESS; gameplane_csrf=$CSRF"
   curl -X POST "$GP/servers/" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)" \
     -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-satisfactory"},"spec":{"templateRef":{"name":"satisfactory"}}}'
   ```

2. **Start server.** Poll until phase=Running, 3m timeout (memory-heavy).

3. **Protocol join.** Run test/e2e/internal/satisfactory/app.go (e2e-probe:http-rest) against service IP:8888.

4. **Console command (rcon/satisfactory family).** Send a command over the agent's console WebSocket: `ws://127.0.0.1:18080/ws/servers/audit018-satisfactory/console` with a `{"kind":"cmd","body":"<command>"}` frame; RCON itself runs on the game port (7777, per modules/satisfactory/template.yaml's spec.rcon), not the messaging port (8888).

5. **Backup.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-satisfactory-bk
     namespace: gameplane-games
   spec:
     serverRef:
       name: audit018-satisfactory
     repoRef:
       name: audit018-restic
       key: repo
     strategy: restic-snapshot
     quiesce: false
   EOF
   ```

6. **Restore.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Restore
   metadata:
     name: audit018-satisfactory-rs
     namespace: gameplane-games
   spec:
     backupRef:
       name: audit018-satisfactory-bk
     serverRef:
       name: audit018-satisfactory
   EOF
   ```

7. **Delete.**
   ```bash
   curl -X DELETE "$GP/servers/audit018-satisfactory" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)"
   ```

**Expected:** Server created, started, HTTP probe succeeds, satisfactory RCON responds, backup and restore complete, deletion removes resource.

**Cleanup:** `kubectl delete gameserver -n gameplane-games audit018-satisfactory; kubectl delete backup,restore -n gameplane-games audit018-satisfactory-bk audit018-satisfactory-rs`.

**Automatable?** Yes (if memory available). Propose bucket: `api-mods`.

---

### ark-survival-ascended

**Preconditions:** The `gameplane-games` namespace exists. The `audit018-admin` role has session cookie set. The module `ark-survival-ascended` is installed.

**Resources created:** `audit018-ark-survival-ascended` (GameServer).

**Note:** This module requires 10Gi memory. It is a **blocked candidate** if kubelab nodes have insufficient capacity. The lightest alternative in the rcon/source + e2e-probe:ark-ascended-tcp category is this module itself. However, a lighter alternative in the rcon/source category with steam-a2s-udp probe: `cs2` (rcon/source + e2e-probe:steam-a2s-udp, 2Gi).

**Steps:**

1. **Create server from template.**
   ```bash
   export GP=http://127.0.0.1:18080
   SESS=$(grep gameplane_session ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   COOKIE="gameplane_session=$SESS; gameplane_csrf=$CSRF"
   curl -X POST "$GP/servers/" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)" \
     -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-ark-survival-ascended"},"spec":{"templateRef":{"name":"ark-survival-ascended"}}}'
   ```

2. **Start server.** Poll until phase=Running, 4m timeout (memory-heavy).

3. **Protocol join.** Run test/e2e/internal/ark-survival-ascended/app.go (e2e-probe:ark-ascended-tcp — raw TCP dial to RCON port 27020).

4. **Console command (rcon/source family).** RCON (27020) is declared `advertise: false` in modules/ark-survival-ascended/template.yaml, so it has no Service port; send a command over the agent's console WebSocket instead: `ws://127.0.0.1:18080/ws/servers/audit018-ark-survival-ascended/console` with a `{"kind":"cmd","body":"<command>"}` frame.

5. **Backup.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-ark-survival-ascended-bk
     namespace: gameplane-games
   spec:
     serverRef:
       name: audit018-ark-survival-ascended
     repoRef:
       name: audit018-restic
       key: repo
     strategy: restic-snapshot
     quiesce: false
   EOF
   ```

6. **Restore.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Restore
   metadata:
     name: audit018-ark-survival-ascended-rs
     namespace: gameplane-games
   spec:
     backupRef:
       name: audit018-ark-survival-ascended-bk
     serverRef:
       name: audit018-ark-survival-ascended
   EOF
   ```

7. **Delete.**
   ```bash
   curl -X DELETE "$GP/servers/audit018-ark-survival-ascended" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)"
   ```

**Expected:** Server created, started, TCP probe succeeds, Source RCON responds, backup and restore complete, deletion removes resource.

**Cleanup:** `kubectl delete gameserver -n gameplane-games audit018-ark-survival-ascended; kubectl delete backup,restore -n gameplane-games audit018-ark-survival-ascended-bk audit018-ark-survival-ascended-rs`.

**Automatable?** Yes (if memory available). Propose bucket: `api-mods`.

---

### cs2

**Preconditions:** The `gameplane-games` namespace exists. The `audit018-admin` role has session cookie set. The module `cs2` is installed.

**Resources created:** `audit018-cs2` (GameServer).

**Steps:**

1. **Create server from template.**
   ```bash
   export GP=http://127.0.0.1:18080
   SESS=$(grep gameplane_session ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   COOKIE="gameplane_session=$SESS; gameplane_csrf=$CSRF"
   curl -X POST "$GP/servers/" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)" \
     -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-cs2"},"spec":{"templateRef":{"name":"cs2"}}}'
   ```

2. **Start server.** Poll until phase=Running, 2m timeout.

3. **Protocol join.** Run test/e2e/internal/cs2/app.go (e2e-probe:steam-a2s-udp) against service IP:27015.

4. **Console command (rcon/source family).** RCON is a separate TCP listener from the (advertised) UDP game port and is itself declared `advertise: false` in modules/cs2/template.yaml; send a command over the agent's console WebSocket instead: `ws://127.0.0.1:18080/ws/servers/audit018-cs2/console` with a `{"kind":"cmd","body":"<command>"}` frame.

5. **Backup.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-cs2-bk
     namespace: gameplane-games
   spec:
     serverRef:
       name: audit018-cs2
     repoRef:
       name: audit018-restic
       key: repo
     strategy: restic-snapshot
     quiesce: false
   EOF
   ```

6. **Restore.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Restore
   metadata:
     name: audit018-cs2-rs
     namespace: gameplane-games
   spec:
     backupRef:
       name: audit018-cs2-bk
     serverRef:
       name: audit018-cs2
   EOF
   ```

7. **Delete.**
   ```bash
   curl -X DELETE "$GP/servers/audit018-cs2" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)"
   ```

**Expected:** Server created, started, A2S probe succeeds, Source RCON responds, backup and restore complete, deletion removes resource.

**Cleanup:** `kubectl delete gameserver -n gameplane-games audit018-cs2; kubectl delete backup,restore -n gameplane-games audit018-cs2-bk audit018-cs2-rs`.

**Automatable?** Yes. Propose bucket: `api-mods`.

---

### minecraft-java

**Preconditions:** The `gameplane-games` namespace exists. The `audit018-admin` role has session cookie set. The module `minecraft-java` is installed.

**Resources created:** `audit018-minecraft-java` (GameServer).

**Note (OD-021 item 23a):** the baseline showed the `minecraft-java` Module stuck in `Pulling`. Before step 1, run `kubectl get module minecraft-java -o yaml` watched over ~30s, saving the watched output to this procedure's evidence file (e.g. `audit/evidence/INV-MOD-061/module-watch.yaml`, per `inventory.md`'s mapping of the minecraft-java create step to INV-MOD-061) rather than only printing it to the terminal. Check it first against F-258/#445: if `status.phase` and the `Pulling` condition's `lastTransitionTime` are flapping on every reconcile with a non-empty `status.conditions[Failed].reason` underneath, that is F-258 (`operator/internal/controller/module_controller.go`'s `markPullingTransition` re-triggering itself through its own watch, `audit/findings.md:2918` (F-258), tracked in `#445`) — confirm `#445` is merged and the churn stops; if it isn't merged or the churn continues, file a finding.

If the watched output does not match F-258's flapping signature, this is a real pull failure, not F-258. In every case — F-258 or a real failure — root-cause the stuck `Pulling` state (check `status.conditions[Failed].reason`/`.message`, the ModuleSource, and the pulled artifact/tag) and fix the underlying cause: this may mean a fix in `modules/minecraft-java/` (e.g. a bad `module.yaml`/`template.yaml` reference) as well as, or instead of, an operator fix, and file a finding either way. Only once the Module is genuinely `Ready` should step 1 run.

**Steps:**

1. **Create server from template.**
   ```bash
   export GP=http://127.0.0.1:18080
   SESS=$(grep gameplane_session ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   COOKIE="gameplane_session=$SESS; gameplane_csrf=$CSRF"
   curl -X POST "$GP/servers/" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)" \
     -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-minecraft-java"},"spec":{"templateRef":{"name":"minecraft-java"}}}'
   ```

2. **Start server.** Poll until phase=Running. This is a cold first boot (image pull plus world generation), so the bound is 10 minutes, not 2 (OD-021 item 24). Record the actual wall-clock time to `Running` in the evidence file.

3. **Protocol join.** Run test/e2e/internal/minecraft-java/app.go (gameproto:minecraft-java wire protocol). Use the gameproto Minecraft Java handshake parser in gameproto/minecraft.go to connect to service IP:25565.

4. **Console command (rcon/source family).** RCON (25575) is declared `advertise: false` in modules/minecraft-java/template.yaml, so it has no Service port; send a command over the agent's console WebSocket instead: `ws://127.0.0.1:18080/ws/servers/audit018-minecraft-java/console` with a `{"kind":"cmd","body":"<command>"}` frame.

5. **Backup.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-minecraft-java-bk
     namespace: gameplane-games
   spec:
     serverRef:
       name: audit018-minecraft-java
     repoRef:
       name: audit018-restic
       key: repo
     strategy: restic-snapshot
     quiesce: false
   EOF
   ```

6. **Restore.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Restore
   metadata:
     name: audit018-minecraft-java-rs
     namespace: gameplane-games
   spec:
     backupRef:
       name: audit018-minecraft-java-bk
     serverRef:
       name: audit018-minecraft-java
   EOF
   ```

7. **Delete.**
   ```bash
   curl -X DELETE "$GP/servers/audit018-minecraft-java" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)"
   ```

**Expected:** Server created, reaches Running within 10 minutes (actual boot time recorded), gameproto Minecraft Java handshake succeeds, Source RCON responds, backup and restore complete, deletion removes resource.

**Cleanup:** `kubectl delete gameserver -n gameplane-games audit018-minecraft-java; kubectl delete backup,restore -n gameplane-games audit018-minecraft-java-bk audit018-minecraft-java-rs`.

**Automatable?** Yes. Propose bucket: `api-mods`.

---

### rust

**Preconditions:** The `gameplane-games` namespace exists. The `audit018-admin` role has session cookie set. The module `rust` is installed.

**Resources created:** `audit018-rust` (GameServer).

**Steps:**

1. **Create server from template.**
   ```bash
   export GP=http://127.0.0.1:18080
   SESS=$(grep gameplane_session ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-admin.txt | cut -d'=' -f2 | cut -d';' -f1)
   COOKIE="gameplane_session=$SESS; gameplane_csrf=$CSRF"
   curl -X POST "$GP/servers/" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)" \
     -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-rust"},"spec":{"templateRef":{"name":"rust"}}}'
   ```

2. **Start server.** Poll until phase=Running, 2m timeout.

3. **Protocol join.** Run test/e2e/internal/rust/app.go (e2e-probe:steam-a2s-udp) against service IP:28015.

4. **Console command (rcon/websocket family).** RCON (28016) is declared `advertise: false` in modules/rust/template.yaml ('exposing it publicly would put the RCON password on the internet'); send a command over the agent's console WebSocket instead: `ws://127.0.0.1:18080/ws/servers/audit018-rust/console` with a `{"kind":"cmd","body":"<command>"}` frame.

5. **Backup.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-rust-bk
     namespace: gameplane-games
   spec:
     serverRef:
       name: audit018-rust
     repoRef:
       name: audit018-restic
       key: repo
     strategy: restic-snapshot
     quiesce: false
   EOF
   ```

6. **Restore.**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: gameplane.local/v1alpha1
   kind: Restore
   metadata:
     name: audit018-rust-rs
     namespace: gameplane-games
   spec:
     backupRef:
       name: audit018-rust-bk
     serverRef:
       name: audit018-rust
   EOF
   ```

7. **Delete.**
   ```bash
   curl -X DELETE "$GP/servers/audit018-rust" \
     -H "Cookie: $COOKIE" \
     -H "X-Gameplane-CSRF: $(echo $COOKIE | grep -o 'gameplane_csrf=[^;]*' | cut -d= -f2)"
   ```

**Expected:** Server created, started, A2S probe succeeds, WebSocket RCON responds, backup and restore complete, deletion removes resource.

**Cleanup:** `kubectl delete gameserver -n gameplane-games audit018-rust; kubectl delete backup,restore -n gameplane-games audit018-rust-bk audit018-rust-rs`.

**Automatable?** Yes. Propose bucket: `api-mods`.
