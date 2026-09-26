# Procedures: MOD

Shared conventions: [conventions.md](conventions.md).

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

3. **Protocol join.** Run the e2e probe (test/e2e/internal/garrys-mod/app.go) against the in-pod port 27015 via kubectl port-forward or via the agent's console API.
   ```bash
   kubectl get svc -n gameplane-games audit018-garrys-mod -o jsonpath='{.spec.clusterIP}'
   # Use the service IP:27015 to run the probe
   ```

4. **Console command (none family).** This category has consoleMode=none, so no console is available. Skip this step.

5. **Backup.** Create a Backup CR. First, ensure a backup repository is available (test/e2e/fixtures/backup-restic-secret.yaml or equivalent).
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
       name: e2e-restic-creds
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

**Cleanup:** `kubectl delete gameserver -n gameplane-games audit018-garrys-mod; kubectl delete backup,restore -n gameplane-games -l audit018-garrys-mod` (if backups/restores were created).

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

4. **Console command (none/rest family).** Use REST API console endpoint to query server status (endpoint pattern: POST /servers/{name}/actions/run with action type depending on game).

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
       name: e2e-restic-creds
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

**Expected:** Server created, started, HTTP probe succeeds, REST API query succeeds, backup and restore complete, deletion removes resource.

**Cleanup:** `kubectl delete gameserver,backup,restore -n gameplane-games audit018-farming-simulator-25`.

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
       name: e2e-restic-creds
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

**Cleanup:** `kubectl delete gameserver,backup,restore -n gameplane-games audit018-beammp`.

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
       name: e2e-restic-creds
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

**Cleanup:** `kubectl delete gameserver,backup,restore -n gameplane-games audit018-valheim`.

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
       name: e2e-restic-creds
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

**Cleanup:** `kubectl delete gameserver,backup,restore -n gameplane-games audit018-dont-starve-together`.

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
       name: e2e-restic-creds
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

**Cleanup:** `kubectl delete gameserver,backup,restore -n gameplane-games audit018-tmodloader`.

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
       name: e2e-restic-creds
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

**Cleanup:** `kubectl delete gameserver,backup,restore -n gameplane-games audit018-terraria`.

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
       name: e2e-restic-creds
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

**Cleanup:** `kubectl delete gameserver,backup,restore -n gameplane-games audit018-factorio`.

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
       name: e2e-restic-creds
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

**Cleanup:** `kubectl delete gameserver,backup,restore -n gameplane-games audit018-dayz`.

**Automatable?** Yes (if memory available). Propose bucket: `api-mods`.

---

### nuclear-option

**Preconditions:** The `gameplane-games` namespace exists. The `audit018-admin` role has session cookie set. The module `nuclear-option` is installed.

**Resources created:** `audit018-nuclear-option` (GameServer).

**Note:** This module uses UDP with no automated probe (join family: "udp (no probe)"). The test will create and start the server, but the protocol join step cannot be automated via a standard e2e probe. Manual verification via `nc -u` or inspection of pod logs is required.

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
       name: e2e-restic-creds
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

**Cleanup:** `kubectl delete gameserver,backup,restore -n gameplane-games audit018-nuclear-option`.

**Automatable?** Partial (no automated probe join). Propose bucket: `api-mods` with manual verification required for join step.

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
       name: e2e-restic-creds
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

**Cleanup:** `kubectl delete gameserver,backup,restore -n gameplane-games audit018-palworld`.

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
       name: e2e-restic-creds
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

**Cleanup:** `kubectl delete gameserver,backup,restore -n gameplane-games audit018-fivem`.

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
       name: e2e-restic-creds
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

**Cleanup:** `kubectl delete gameserver,backup,restore -n gameplane-games audit018-satisfactory`.

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
       name: e2e-restic-creds
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

**Cleanup:** `kubectl delete gameserver,backup,restore -n gameplane-games audit018-ark-survival-ascended`.

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
       name: e2e-restic-creds
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

**Cleanup:** `kubectl delete gameserver,backup,restore -n gameplane-games audit018-cs2`.

**Automatable?** Yes. Propose bucket: `api-mods`.

---

### minecraft-java

**Preconditions:** The `gameplane-games` namespace exists. The `audit018-admin` role has session cookie set. The module `minecraft-java` is installed.

**Resources created:** `audit018-minecraft-java` (GameServer).

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

2. **Start server.** Poll until phase=Running, 2m timeout.

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
       name: e2e-restic-creds
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

**Expected:** Server created, started, gameproto Minecraft Java handshake succeeds, Source RCON responds, backup and restore complete, deletion removes resource.

**Cleanup:** `kubectl delete gameserver,backup,restore -n gameplane-games audit018-minecraft-java`.

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
       name: e2e-restic-creds
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

**Cleanup:** `kubectl delete gameserver,backup,restore -n gameplane-games audit018-rust`.

**Automatable?** Yes. Propose bucket: `api-mods`.
