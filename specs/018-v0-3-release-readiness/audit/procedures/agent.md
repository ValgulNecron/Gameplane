# Procedures: AGT

Shared conventions: [conventions.md](conventions.md).

### console-source

Execute a command on a game server via source RCON protocol (Minecraft, Valve game servers). The server must have RCON enabled with `consoleMode: rcon` and `rcon.protocol: source`.

**Preconditions**

- `audit018-operator` or `audit018-admin` account exists
- `minecraft-java` module is deployed in the cluster
- No pre-existing GameServer is required: this procedure creates its own `audit018-mc-source` GameServer from the `minecraft-java` template (Step 2), which sets `rcon.protocol: source` (modules/minecraft-java/template.yaml:83-85) and so defaults implicitly to `consoleMode: rcon` (operator/api/v1alpha1/gametemplate_types.go:115-116) (OD-021 item 1)
- `GP=http://127.0.0.1:18080` environment variable is set
- Session cookie and CSRF token for the operator account are saved in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-mc-source` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)

**Steps**

1. Launch the operator session and set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2 | cut -d';' -f1)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2 | cut -d';' -f1)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-mc-source","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-mc-source:start"
   ```
   Poll `kubectl get gameserver audit018-mc-source -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Send a simple RCON command via the agent console WebSocket endpoint:
   ```sh
   (echo -n '{"kind":"cmd","body":"list"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-mc-source/console"
   ```
   (Cost: 0 additional logins)

4. Capture the response, which should echo an RCON response frame. Save to evidence file:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-001
   (echo -n '{"kind":"cmd","body":"list"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-mc-source/console" \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-001/console-source.txt
   ```

**Expected**

- WebSocket connection succeeds (101 Switching Protocols)
- RCON response is received as a JSON frame with `kind="out"` and `body` containing the command output
- Response indicates online player count or similar RCON data

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-mc-source"
   ```
- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### console-telnet

Execute a command on a game server via telnet RCON protocol (7 Days to Die, Line-based RCon). The server must have telnet RCON configured.

**Preconditions**

- `audit018-operator` or `audit018-admin` account exists
- `7-days-to-die` module is available (telnet RCON on port 8081)
- No pre-existing GameServer is required: this procedure creates its own `audit018-telnet-rcon` GameServer from the `7-days-to-die` template (Step 2) (OD-021 item 1)
- `GP=http://127.0.0.1:18080` environment variable is set
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-telnet-rcon` GameServer (created from the `7-days-to-die` template by this procedure; deleted at cleanup)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `7-days-to-die` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-telnet-rcon","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"7-days-to-die"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-telnet-rcon:start"
   ```
   Poll `kubectl get gameserver audit018-telnet-rcon -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Send a telnet RCON command:
   ```sh
   (echo -n '{"kind":"cmd","body":"help"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-telnet-rcon/console"
   ```
   (Cost: 0 additional logins)

4. Save response to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-002
   (echo -n '{"kind":"cmd","body":"help"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-telnet-rcon/console" \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-002/console-telnet.txt
   ```

**Expected**

- WebSocket connection succeeds
- Telnet RCON response received with command output
- Response shows help text or acknowledgment

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-telnet-rcon"
   ```
- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### console-websocket

Execute a command on a game server via websocket RCON protocol (Rust WebRcon). The server must have `consoleMode: rcon` and `rcon.protocol: websocket`.

**Preconditions**

- `audit018-operator` account exists
- `rust` module is available
- No pre-existing GameServer is required: this procedure creates its own `audit018-rust-ws` GameServer from the `rust` template (Step 2), which defaults to `consoleMode: rcon` / `rcon.protocol: websocket` (modules/rust/template.yaml:62-69) (OD-021 item 1)
- `GP=http://127.0.0.1:18080` environment variable is set
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-rust-ws` GameServer (created from the `rust` template by this procedure; deleted at cleanup)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `rust` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-rust-ws","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"rust"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-rust-ws:start"
   ```
   Poll `kubectl get gameserver audit018-rust-ws -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Send a websocket RCON command:
   ```sh
   (echo -n '{"kind":"cmd","body":"status"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-rust-ws/console"
   ```
   (Cost: 0 additional logins)

4. Save response to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-003
   (echo -n '{"kind":"cmd","body":"status"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-rust-ws/console" \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-003/console-websocket.txt
   ```

**Expected**

- WebSocket connection succeeds
- Websocket RCON response received
- Response contains server status data

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-rust-ws"
   ```
- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### console-battleye

Execute a command on a game server via BattlEye RCON protocol (DayZ, Arma). The server must have `rcon.protocol: battleye`.

**Preconditions**

- `audit018-operator` account exists
- `dayz` module is available
- No pre-existing GameServer is required: this procedure creates its own `audit018-dayz-be` GameServer from the `dayz` template (Step 2), which defaults to `consoleMode: rcon` / `rcon.protocol: battleye` (modules/dayz/template.yaml:107-113) (OD-021 item 1)
- `GP=http://127.0.0.1:18080` environment variable is set
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-dayz-be` GameServer (created from the `dayz` template by this procedure; deleted at cleanup)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `dayz` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-dayz-be","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"dayz"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-dayz-be:start"
   ```
   Poll `kubectl get gameserver audit018-dayz-be -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Send a BattlEye RCON command:
   ```sh
   (echo -n '{"kind":"cmd","body":"players"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-dayz-be/console"
   ```
   (Cost: 0 additional logins)

4. Save response to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-004
   (echo -n '{"kind":"cmd","body":"players"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-dayz-be/console" \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-004/console-battleye.txt
   ```

**Expected**

- WebSocket connection succeeds
- BattlEye RCON response received with player list or acknowledgment

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-dayz-be"
   ```
- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### console-satisfactory

Execute a command on a game server via Satisfactory HTTPS admin API RCON protocol.

**Preconditions**

- `audit018-operator` account exists
- `satisfactory` module is available
- No pre-existing GameServer is required: this procedure creates its own `audit018-satisfactory-api` GameServer from the `satisfactory` template (Step 2), which defaults to `consoleMode: rcon` / `rcon.protocol: satisfactory` (modules/satisfactory/template.yaml:116-123) (OD-021 item 1)
- `GP=http://127.0.0.1:18080` environment variable is set
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-satisfactory-api` GameServer (created from the `satisfactory` template by this procedure; deleted at cleanup)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `satisfactory` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-satisfactory-api","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"satisfactory"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-satisfactory-api:start"
   ```
   Poll `kubectl get gameserver audit018-satisfactory-api -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Send a Satisfactory RCON command:
   ```sh
   (echo -n '{"kind":"cmd","body":"GetGamePhase"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-satisfactory-api/console"
   ```
   (Cost: 0 additional logins)

4. Save response to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-005
   (echo -n '{"kind":"cmd","body":"GetGamePhase"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-satisfactory-api/console" \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-005/console-satisfactory.txt
   ```

**Expected**

- WebSocket connection succeeds
- Satisfactory HTTPS API response received

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-satisfactory-api"
   ```
- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### console-palworld

Execute a command on a game server via Palworld REST admin API RCON protocol.

**Preconditions**

- `audit018-operator` account exists
- `palworld` module is available
- No pre-existing GameServer is required: this procedure creates its own `audit018-palworld-api` GameServer from the `palworld` template (Step 2), which defaults to `rcon.protocol: palworld` (modules/palworld/template.yaml:109-111) (OD-021 item 1)
- `GP=http://127.0.0.1:18080` environment variable is set
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-palworld-api` GameServer (created from the `palworld` template by this procedure; deleted at cleanup)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `palworld` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-palworld-api","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"palworld"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-palworld-api:start"
   ```
   Poll `kubectl get gameserver audit018-palworld-api -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Send a Palworld RCON command:
   ```sh
   (echo -n '{"kind":"cmd","body":"Info"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-palworld-api/console"
   ```
   (Cost: 0 additional logins)

4. Save response to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-006
   (echo -n '{"kind":"cmd","body":"Info"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-palworld-api/console" \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-006/console-palworld.txt
   ```

**Expected**

- WebSocket connection succeeds
- Palworld REST API response received with server info

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-palworld-api"
   ```
- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### console-nuclearoption

Execute a command on a game server via Nuclear Option remote-command RCON protocol.

The baseline recorded the `nuclear-option` Module as `Failed`. Per OD-021 item 3, re-check its health right before this round; if it is still `Failed`, root-cause and fix it and file a finding rather than substituting another module.

**Preconditions**

- `audit018-operator` account exists
- `nuclear-option` module is available and its Module CR reports `status.phase: Ready` as of this round (Step 1) — if still `Failed`, do not proceed with a substitute module (OD-021 item 3)
- No pre-existing GameServer is required: this procedure creates its own `audit018-nuclear-api` GameServer from the `nuclear-option` template (Step 3), which defaults to `rcon.protocol: nuclearoption` (modules/nuclear-option/template.yaml:84-86) (OD-021 item 1)
- `GP=http://127.0.0.1:18080` environment variable is set
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-nuclear-api` GameServer (created from the `nuclear-option` template by this procedure; deleted at cleanup)

**Steps**

1. Re-check the `nuclear-option` Module's health for this round (OD-021 item 3). Module status has no `message` field — the error field is `status.lastError`, with `status.conditions` alongside it (operator/api/v1alpha1/module_types.go:82,88):
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-007
   kubectl get module nuclear-option -o jsonpath='{.status.phase}{"\n"}{.status.lastError}{"\n"}' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-007/module-health.txt
   kubectl get module nuclear-option -o jsonpath='{.status.conditions}' \
     >> specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-007/module-health.txt
   ```
   If `status.phase` is `Failed`: root-cause it (check `status.lastError` and `status.conditions` above, the ModuleSource sync logs, and the referenced OCI/git artifact), fix the underlying issue, wait for `status.phase: Ready`, and file a finding documenting the root cause and fix, backed by the saved `module-health.txt`. Do not switch to a different module to route around a `Failed` `nuclear-option`.

2. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

3. Create and start the test GameServer from the `nuclear-option` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-nuclear-api","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"nuclear-option"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-nuclear-api:start"
   ```
   Poll `kubectl get gameserver audit018-nuclear-api -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

4. Send a Nuclear Option RCON command:
   ```sh
   (echo -n '{"kind":"cmd","body":"status"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-nuclear-api/console"
   ```
   (Cost: 0 additional logins)

5. Save response to evidence:
   ```sh
   (echo -n '{"kind":"cmd","body":"status"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-nuclear-api/console" \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-007/console-nuclearoption.txt
   ```

**Expected**

- WebSocket connection succeeds
- Nuclear Option RCON response received

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-nuclear-api"
   ```
- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### console-rest

Execute a command on a game server via generic HTTP/JSON REST admin API RCON protocol (FiveM, Farming Simulator 25).

**Preconditions**

- `audit018-operator` account exists
- `fivem` module is available
- No pre-existing GameServer is required: this procedure creates its own `audit018-fivem-rest` GameServer from the `fivem` template (Step 2), which defaults to `consoleMode: rcon` / `rcon.protocol: rest` (modules/fivem/template.yaml:61-66) (OD-021 item 1)
- `GP=http://127.0.0.1:18080` environment variable is set
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-fivem-rest` GameServer (created from the `fivem` template by this procedure; deleted at cleanup)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `fivem` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-fivem-rest","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"fivem"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-fivem-rest:start"
   ```
   Poll `kubectl get gameserver audit018-fivem-rest -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Send a REST RCON command:
   ```sh
   (echo -n '{"kind":"cmd","body":"status"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-fivem-rest/console"
   ```
   (Cost: 0 additional logins)

4. Save response to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-008
   (echo -n '{"kind":"cmd","body":"status"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-fivem-rest/console" \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-008/console-rest.txt
   ```

**Expected**

- WebSocket connection succeeds
- REST API response received with server status

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-fivem-rest"
   ```
- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### console-pty

Execute a command on a game server via PTY console (pod attach, for games without RCON like Terraria, Valheim, Arma Reforger).

The baseline recorded the `terraria` Module as `Failed`. Per OD-021 item 3, re-check its health right before this round; if it is still `Failed`, root-cause and fix it and file a finding rather than substituting another `consoleMode: pty` module.

**Preconditions**

- `audit018-operator` account exists
- A game with `consoleMode: pty` is available (`terraria`), whose Module CR reports `status.phase: Ready` as of this round (Step 1) — if still `Failed`, do not proceed with a substitute module (OD-021 item 3)
- No pre-existing GameServer is required: this procedure creates its own `audit018-terraria-pty` GameServer from the `terraria` template (Step 3), which defaults to `consoleMode: pty` (modules/terraria/template.yaml:63-65) (OD-021 item 1)
- `GP=http://127.0.0.1:18080` environment variable is set
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-terraria-pty` GameServer (created from the `terraria` template by this procedure; deleted at cleanup)

**Steps**

1. Re-check the `terraria` Module's health for this round (OD-021 item 3). Module status has no `message` field — the error field is `status.lastError`, with `status.conditions` alongside it (operator/api/v1alpha1/module_types.go:82,88):
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-009
   kubectl get module terraria -o jsonpath='{.status.phase}{"\n"}{.status.lastError}{"\n"}' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-009/module-health.txt
   kubectl get module terraria -o jsonpath='{.status.conditions}' \
     >> specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-009/module-health.txt
   ```
   If `status.phase` is `Failed`: root-cause it (check `status.lastError` and `status.conditions` above, the ModuleSource sync logs, and the referenced OCI/git artifact), fix the underlying issue, wait for `status.phase: Ready`, and file a finding documenting the root cause and fix, backed by the saved `module-health.txt`. Do not switch to a different `consoleMode: pty` module to route around a `Failed` `terraria`.

2. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

3. Create and start the test GameServer from the `terraria` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-terraria-pty","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"terraria"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-terraria-pty:start"
   ```
   Poll `kubectl get gameserver audit018-terraria-pty -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

4. Send PTY stdin via WebSocket (pod attach path):
   ```sh
   (echo -n '{"kind":"stdin","body":"aGVscAo="}'; sleep 1) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-terraria-pty/console-pty"
   ```
   (The base64 blob is "help\n" encoded)
   (Cost: 0 additional logins)

5. Save response to evidence:
   ```sh
   (echo -n '{"kind":"stdin","body":"aGVscAo="}'; sleep 1) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-terraria-pty/console-pty" \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-009/console-pty.txt
   ```

**Expected**

- WebSocket connection succeeds (pod attach via Kubernetes API)
- PTY stdout is received as base64-encoded frames with `kind="stdout"`
- Terminal output is visible when decoded

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-terraria-pty"
   ```
- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### files-list

List files and directories in the server's data volume.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own `audit018-agt-files-list` GameServer from the `minecraft-java` template (Step 2) instead of addressing the template name as a server (OD-021 item 1)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-files-list` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-files-list","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-files-list:start"
   ```
   Poll `kubectl get gameserver audit018-agt-files-list -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. List the root directory:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-files-list/files/list?path=/" | jq '.'
   ```
   (Cost: 0 additional logins)

4. Save response to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-010
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-files-list/files/list?path=/" \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-010/files-list.json
   ```

**Expected**

- HTTP 200 response
- JSON array of file/directory entries with name, path, size, isDir, modTime

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-files-list"
   ```

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### files-read

Read the contents of a text file from the server's data volume.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own GameServer from the `minecraft-java` template (Step 2) (OD-021 item 1)
- A known text file exists (e.g., server.properties in Minecraft)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-files-read` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-files-read","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-files-read:start"
   ```
   Poll `kubectl get gameserver audit018-agt-files-read -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Read a file:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-files-read/files/read?path=/server.properties"
   ```
   (Cost: 0 additional logins)

4. Save response to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-011
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-files-read/files/read?path=/server.properties" \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-011/files-read.json
   ```

**Expected**

- HTTP 200 response
- File contents returned as JSON with data field

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-files-read"
   ```

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### files-write

Write or overwrite a file in the server's data volume.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own GameServer from the `minecraft-java` template (Step 2) (OD-021 item 1)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-files-write` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)
- `audit018-test-file.txt` in the server's data volume

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-files-write","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-files-write:start"
   ```
   Poll `kubectl get gameserver audit018-agt-files-write -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Write a test file:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     --data-binary "Hello, audit!" \
     "$GP/servers/audit018-agt-files-write/files/write?path=/audit018-test-file.txt"
   ```
   (Cost: 0 additional logins)

4. Verify by reading the file back:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-files-write/files/read?path=/audit018-test-file.txt"
   ```

5. Save responses to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-012
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     --data-binary "Hello, audit!" \
     "$GP/servers/audit018-agt-files-write/files/write?path=/audit018-test-file.txt" \
     -w '\nHTTP %{http_code}\n' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-012/write.txt
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-files-write/files/read?path=/audit018-test-file.txt" \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-012/read-back.json
   ```

**Expected**

- HTTP 204 No Content response from write
- File contents are readable via the read endpoint
- Contents match what was written

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-files-write"
   ```
- Delete the test file:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-files-write/files/delete?path=/audit018-test-file.txt"
   ```

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### files-upload

Upload a file to the server's data volume.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own GameServer from the `minecraft-java` template (Step 2) (OD-021 item 1)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-files-upload` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)
- `audit018-uploaded.txt` in the server's data volume

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-files-upload","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-files-upload:start"
   ```
   Poll `kubectl get gameserver audit018-agt-files-upload -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Create a test file locally:
   ```sh
   echo "Uploaded test file" > /tmp/audit018-upload.txt
   ```

4. Upload the file:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -F "file=@/tmp/audit018-upload.txt" \
     "$GP/servers/audit018-agt-files-upload/files/upload?path=/"
   ```
   (Cost: 0 additional logins)

5. Verify by listing the directory:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-files-upload/files/list?path=/" | jq '.[] | select(.name == "audit018-upload.txt")'
   ```

6. Save responses to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-013
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -F "file=@/tmp/audit018-upload.txt" \
     "$GP/servers/audit018-agt-files-upload/files/upload?path=/" \
     -w '\nHTTP %{http_code}\n' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-013/upload.txt
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-files-upload/files/list?path=/" | jq '.[] | select(.name == "audit018-upload.txt")' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-013/list-after.json
   ```

**Expected**

- HTTP 204 No Content response from upload
- File appears in directory listing
- File is readable

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-files-upload"
   ```
- No separate file cleanup: deleting the test GameServer above removes its data volume, including the uploaded file

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### files-download

Download a file from the server's data volume.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own GameServer from the `minecraft-java` template (Step 2) (OD-021 item 1)
- A known file exists in the server
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-files-download` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-files-download","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-files-download:start"
   ```
   Poll `kubectl get gameserver audit018-agt-files-download -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Download a file:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-files-download/files/download?path=/server.properties" \
     -o /tmp/audit018-downloaded.properties
   file /tmp/audit018-downloaded.properties
   ```
   (Cost: 0 additional logins)

4. Save metadata to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-014
   ls -lh /tmp/audit018-downloaded.properties \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-014/download-metadata.txt
   file /tmp/audit018-downloaded.properties \
     >> specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-014/download-metadata.txt
   ```

**Expected**

- HTTP 200 response
- File binary is downloaded with correct content-disposition header
- Downloaded file is readable and matches original

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-files-download"
   ```
- Remove the downloaded file

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### files-mkdir

Create a directory in the server's data volume.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own GameServer from the `minecraft-java` template (Step 2) (OD-021 item 1)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-files-mkdir` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)
- `audit018-testdir` directory in the server's data volume

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-files-mkdir","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-files-mkdir:start"
   ```
   Poll `kubectl get gameserver audit018-agt-files-mkdir -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Create a directory:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-files-mkdir/files/mkdir?path=/audit018-testdir"
   ```
   (Cost: 0 additional logins)

4. Verify by listing:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-files-mkdir/files/list?path=/" | jq '.[] | select(.name == "audit018-testdir")'
   ```

5. Save responses to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-015
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-files-mkdir/files/mkdir?path=/audit018-testdir" \
     -w '\nHTTP %{http_code}\n' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-015/mkdir.txt
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-files-mkdir/files/list?path=/" | jq '.[] | select(.name == "audit018-testdir")' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-015/list-after.json
   ```

**Expected**

- HTTP 204 No Content response
- Directory appears in listing with isDir=true

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-files-mkdir"
   ```
- No separate directory cleanup: deleting the test GameServer above removes its data volume, including the directory

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### files-delete

Delete a file or directory from the server's data volume.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own GameServer from the `minecraft-java` template (Step 2) (OD-021 item 1)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-files-delete` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)
- `audit018-test-file.txt` in the server's data volume, written by this procedure in Step 3 so the test is self-contained (OD-021 item 1), then deleted in Step 4

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-files-delete","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-files-delete:start"
   ```
   Poll `kubectl get gameserver audit018-agt-files-delete -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Write the test file so there is something to delete:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     --data-binary "Hello, audit!" \
     "$GP/servers/audit018-agt-files-delete/files/write?path=/audit018-test-file.txt"
   ```

4. Delete the test file:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-files-delete/files/delete?path=/audit018-test-file.txt"
   ```
   (Cost: 0 additional logins)

5. Verify by attempting to read the deleted file (should fail):
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-files-delete/files/read?path=/audit018-test-file.txt" | jq .
   ```

6. Save responses to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-016
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-files-delete/files/delete?path=/audit018-test-file.txt" \
     -w '\nHTTP %{http_code}\n' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-016/delete.txt
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-files-delete/files/read?path=/audit018-test-file.txt" | jq . \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-016/read-after-delete.json
   ```

**Expected**

- HTTP 204 No Content response from delete
- Subsequent read returns 404 or error
- File no longer appears in directory listing

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-files-delete"
   ```

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### logs-tail

Stream the server's log file via WebSocket in real-time.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own GameServer from the `minecraft-java` template (Step 2), which has a log file configured (OD-021 item 1)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-logs-tail` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-logs-tail","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-logs-tail:start"
   ```
   Poll `kubectl get gameserver audit018-agt-logs-tail -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Open the log tail WebSocket and read 5 seconds of logs:
   ```sh
   timeout 5 curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-agt-logs-tail/logs" 2>&1 | head -50
   ```
   (Cost: 0 additional logins)

4. Save output to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-017
   timeout 5 curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://${GP#http://}/ws/servers/audit018-agt-logs-tail/logs" 2>&1 | head -50 \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-017/logs-tail.txt
   ```

**Expected**

- WebSocket connection succeeds (101 Switching Protocols)
- Each log line is streamed as a plain-text WS frame (agent/internal/logs/logs.go `tailLoop` calls `conn.Write(ctx, websocket.MessageText, []byte(line))`) — not a JSON envelope, and not base64-encoded

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-logs-tail"
   ```
- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### logs-download

Download the complete log file from the server.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own GameServer from the `minecraft-java` template (Step 2), which has a log file (OD-021 item 1)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-logs-download` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-logs-download","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-logs-download:start"
   ```
   Poll `kubectl get gameserver audit018-agt-logs-download -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Download the log file:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-logs-download/logs/download" \
     -o /tmp/audit018-game.log
   wc -l /tmp/audit018-game.log
   file /tmp/audit018-game.log
   ```
   (Cost: 0 additional logins)

4. Save metadata to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-018
   ls -lh /tmp/audit018-game.log \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-018/download-metadata.txt
   head -20 /tmp/audit018-game.log \
     >> specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-018/download-metadata.txt
   ```

**Expected**

- HTTP 200 response
- Log file is downloaded as plain text
- File contains readable log entries

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-logs-download"
   ```
- Remove the downloaded file

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### players-list

List online players on the server.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own `audit018-agt-players-list` GameServer from the `minecraft-java` template (Step 2), which reports player counts over RCON (OD-021 item 1)
- At least one player is online (or procedure documents empty list)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-players-list` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-players-list","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-players-list:start"
   ```
   Poll `kubectl get gameserver audit018-agt-players-list -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Get the player list:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-players-list/players/" | jq '.'
   ```
   (Cost: 0 additional logins)

4. Save response to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-019
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-players-list/players/" \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-019/players-list.json
   ```

**Expected**

- HTTP 200 response
- JSON body `{"online":N,"max":M,"players":["name1",...],"asOf":"...","capabilities":{...}}` (agent/internal/players/players.go `Snapshot`)
- `players` is a flat array of player-name strings — there is no per-player UUID or joinTime field

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-players-list"
   ```

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### players-banned

List banned players on the server.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own GameServer from the `minecraft-java` template (Step 2), which has ban support (OD-021 item 1)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-players-banned` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-players-banned","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-players-banned:start"
   ```
   Poll `kubectl get gameserver audit018-agt-players-banned -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Get the banned players list:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-players-banned/players/banned" | jq '.'
   ```
   (Cost: 0 additional logins)

4. Save response to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-020
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-players-banned/players/banned" \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-020/players-banned.json
   ```

**Expected**

- HTTP 200 response
- JSON response with banned list (may be empty)

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-players-banned"
   ```

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### players-kick

Kick a player from the server.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own GameServer from the `minecraft-java` template (Step 2) (OD-021 item 1)
- At least one player is online to kick
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-players-kick` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)
- None (removes a player session)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-players-kick","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-players-kick:start"
   ```
   Poll `kubectl get gameserver audit018-agt-players-kick -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. First, list players to get a valid name:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-players-kick/players/" | jq -r '.players[0]'
   export PLAYER_NAME=<from-above>
   ```

4. Kick the player:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d "{\"name\":\"$PLAYER_NAME\",\"reason\":\"audit018\"}" \
     "$GP/servers/audit018-agt-players-kick/players/kick"
   ```
   (Cost: 0 additional logins)

5. Save responses to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-021
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d "{\"name\":\"$PLAYER_NAME\",\"reason\":\"audit018\"}" \
     "$GP/servers/audit018-agt-players-kick/players/kick" \
     -w '\nHTTP %{http_code}\n' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-021/kick.txt
   ```

**Expected**

- HTTP 200 response
- Player is disconnected from the server
- Player does not appear in subsequent player list

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-players-kick"
   ```
- None (the player was already removed)

**Automatable?**

Yes. Proposed bucket: `api-agent`. (Requires test player bot for consistent automation.)

---

### players-ban

Ban a player from the server.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own GameServer from the `minecraft-java` template (Step 2) (OD-021 item 1)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-players-ban` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)
- Ban entry for the test player

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-players-ban","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-players-ban:start"
   ```
   Poll `kubectl get gameserver audit018-agt-players-ban -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Ban a player (using a known or test player name):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"name":"audit018testplayer","reason":"audit"}' \
     "$GP/servers/audit018-agt-players-ban/players/ban"
   ```
   (Cost: 0 additional logins)

4. Verify the player is in the banned list:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-players-ban/players/banned" | jq '.[] | select(.name == "audit018testplayer")'
   ```

5. Save responses to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-022
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"name":"audit018testplayer","reason":"audit"}' \
     "$GP/servers/audit018-agt-players-ban/players/ban" \
     -w '\nHTTP %{http_code}\n' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-022/ban.txt
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-players-ban/players/banned" | jq '.[] | select(.name == "audit018testplayer")' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-022/banned-after.json
   ```

**Expected**

- HTTP 200 response
- Player appears in the banned list
- Player cannot rejoin the server

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-players-ban"
   ```
- No separate unban step needed: deleting the test GameServer above removes its ban list

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### players-unban

Unban a player from the server.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own GameServer from the `minecraft-java` template (Step 2) (OD-021 item 1)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-players-unban` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)
- A ban entry for `audit018testplayer`, created by this procedure in Step 3 so the test is self-contained (OD-021 item 1), then removed in Step 4

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-players-unban","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-players-unban:start"
   ```
   Poll `kubectl get gameserver audit018-agt-players-unban -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Ban the player first so there is a ban entry to remove:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"name":"audit018testplayer","reason":"audit"}' \
     "$GP/servers/audit018-agt-players-unban/players/ban"
   ```

4. Unban the player:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"name":"audit018testplayer"}' \
     "$GP/servers/audit018-agt-players-unban/players/unban"
   ```
   (Cost: 0 additional logins)

5. Verify the player is no longer banned:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-players-unban/players/banned" | jq '.[] | select(.name == "audit018testplayer")'
   ```
   (Should return nothing)

6. Save responses to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-023
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"name":"audit018testplayer"}' \
     "$GP/servers/audit018-agt-players-unban/players/unban" \
     -w '\nHTTP %{http_code}\n' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-023/unban.txt
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-players-unban/players/banned" | jq '.[] | select(.name == "audit018testplayer")' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-023/banned-after.json
   ```

**Expected**

- HTTP 200 response
- Player no longer appears in the banned list
- Player can rejoin the server

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-players-unban"
   ```

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### players-whitelist

List whitelisted players on the server.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own GameServer from the `minecraft-java` template (Step 2), which has whitelist support (OD-021 item 1)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-players-wl` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-players-wl","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-players-wl:start"
   ```
   Poll `kubectl get gameserver audit018-agt-players-wl -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Get the whitelist:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-players-wl/players/whitelist" | jq '.'
   ```
   (Cost: 0 additional logins)

4. Save response to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-024
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-players-wl/players/whitelist" \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-024/whitelist.json
   ```

**Expected**

- HTTP 200 response
- JSON response with whitelist (array of players)

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-players-wl"
   ```

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### players-whitelist-add

Add a player to the server's whitelist.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own GameServer from the `minecraft-java` template (Step 2), which has whitelist support (OD-021 item 1)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-wl-add` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)
- Whitelist entry for the test player

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-wl-add","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-wl-add:start"
   ```
   Poll `kubectl get gameserver audit018-agt-wl-add -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Add a player to the whitelist:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"name":"audit018whitelisted"}' \
     "$GP/servers/audit018-agt-wl-add/players/whitelist/add"
   ```
   (Cost: 0 additional logins)

4. Verify the player is in the whitelist:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-wl-add/players/whitelist" | jq '.[] | select(. == "audit018whitelisted")'
   ```

5. Save responses to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-025
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"name":"audit018whitelisted"}' \
     "$GP/servers/audit018-agt-wl-add/players/whitelist/add" \
     -w '\nHTTP %{http_code}\n' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-025/whitelist-add.txt
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-wl-add/players/whitelist" | jq '.[] | select(. == "audit018whitelisted")' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-025/whitelist-after.json
   ```

**Expected**

- HTTP 200 response
- Player appears in the whitelist

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-wl-add"
   ```
- No separate whitelist-removal step needed: deleting the test GameServer above removes its whitelist

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### players-whitelist-remove

Remove a player from the server's whitelist.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own GameServer from the `minecraft-java` template (Step 2) (OD-021 item 1)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-wl-rm` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)
- A whitelist entry for `audit018whitelisted`, added by this procedure in Step 3 so the test is self-contained (OD-021 item 1), then removed in Step 4

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-wl-rm","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-wl-rm:start"
   ```
   Poll `kubectl get gameserver audit018-agt-wl-rm -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Add the player to the whitelist first so there is an entry to remove:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"name":"audit018whitelisted"}' \
     "$GP/servers/audit018-agt-wl-rm/players/whitelist/add"
   ```

4. Remove the player from the whitelist:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"name":"audit018whitelisted"}' \
     "$GP/servers/audit018-agt-wl-rm/players/whitelist/remove"
   ```
   (Cost: 0 additional logins)

5. Verify the player is no longer in the whitelist:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-wl-rm/players/whitelist" | jq '.[] | select(. == "audit018whitelisted")'
   ```
   (Should return nothing)

6. Save responses to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-026
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"name":"audit018whitelisted"}' \
     "$GP/servers/audit018-agt-wl-rm/players/whitelist/remove" \
     -w '\nHTTP %{http_code}\n' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-026/whitelist-remove.txt
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-wl-rm/players/whitelist" | jq '.[] | select(. == "audit018whitelisted")' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-026/whitelist-after.json
   ```

**Expected**

- HTTP 200 response
- Player no longer appears in the whitelist

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-wl-rm"
   ```

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### quiesce-pause

Verify the agent's pause (quiesce) sequence, which the operator triggers over RCON before a backup snapshot.

The API gateway does not proxy `/quiesce` or `/unquiesce` (agent/internal/quiesce/quiesce.go `Mount` registers them on the agent's own router; only the operator's Backup path reaches them). Test it the way the operator does: create a Backup CR with `spec.quiesce: true` against an `audit018-` server and read the pause sequence back from the agent-exposed game log (OD-021 item 2), instead of calling `/quiesce` directly through `$GP`.

**Preconditions**

- `audit018-operator` or `audit018-admin` account exists
- No pre-existing GameServer is required: this procedure creates its own `audit018-agt-quiesce-p` GameServer from the `minecraft-java` template (Step 2) (OD-021 item 1)
- The `audit018-restic` restic-server Deployment/Service exist in `gameplane-system`, and the per-round `audit018-restic` repo Secret exists in `gameplane-games` (OD-021 items 9/16, set up by the modules.md/crd.md backup procedures for this round)
- `minecraft-java`'s GameTemplate declares `capabilities.quiesce.quiesce: ["save-off", "save-all flush"]` (modules/minecraft-java/template.yaml:329-332), so the pause sequence is these two RCON commands, in order

**Resources created**

- `audit018-agt-quiesce-p` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)
- Backup `audit018-agt-quiesce-p-backup` with `spec.quiesce: true` (created by this procedure; deleted at cleanup)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-quiesce-p","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-quiesce-p:start"
   ```
   Poll `kubectl get gameserver audit018-agt-quiesce-p -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Create the Backup CR with `quiesce: true` (the CRD default; setting it explicitly documents intent — operator/api/v1alpha1/backup_types.go:64, field at :66):
   ```sh
   kubectl apply -f - <<'YAML'
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-agt-quiesce-p-backup
     namespace: gameplane-games
     labels:
       gameplane.io/audit: "018"
   spec:
     serverRef:
       name: audit018-agt-quiesce-p
     repoRef:
       name: audit018-restic
       key: repo
     strategy: restic-snapshot
     quiesce: true
   YAML
   ```
   (Cost: 0 additional logins)

4. Immediately watch the Backup's quiesce annotations, which the controller sets before the restic Job runs (operator/internal/controller/backup_controller.go: `annoQuiescedAt`/`annoUnquiescedAt`), saving the watch output to evidence (the Expected claim about annotation ordering is backed by this file, not by a terminal-only watch):
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-027
   timeout 60 kubectl get backup audit018-agt-quiesce-p-backup -n gameplane-games -w -o jsonpath='{.metadata.annotations.backup\.gameplane\.local/quiesced-at}{"\n"}' \
     | tee specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-027/quiesce-watch.txt
   ```
   Stop watching once `quiesced-at` is set but before the Backup reaches `Succeeded` (poll `status.phase`); this is the paused window the snapshot runs in.

5. While paused, download the game log via the agent and confirm the pause sequence ran (save-off then save-all flush), before any post-backup resume line appears:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-quiesce-p/logs/download" \
     -o specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-027/game-log-during-pause.log
   grep -n -i -E "automatic saving is now disabled|saved the game|saving the game" \
     specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-027/game-log-during-pause.log \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-027/save-sequence.txt
   kubectl get backup audit018-agt-quiesce-p-backup -n gameplane-games -o json \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-027/backup-object.json
   ```

**Expected**

- The Backup's `backup.gameplane.local/quiesce-attempted` and `backup.gameplane.local/quiesced-at` annotations are set before `status.phase` reaches `Succeeded`
- The game log shows the auto-save-disabled line (from `save-off`) followed by a world-saved line (from `save-all flush`), in that order, before the matching resume line from `unquiesce`
- No `saving failed` text appears in the output (the module's `failurePattern` — modules/minecraft-java/template.yaml:332)

**Cleanup**

- Wait for the Backup to reach a terminal phase, then delete it:
   ```sh
   kubectl delete backup audit018-agt-quiesce-p-backup -n gameplane-games
   ```
- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-quiesce-p"
   ```

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### quiesce-resume

Verify the agent's resume (unquiesce) sequence, which the operator triggers over RCON once a backup snapshot completes.

Like quiesce-pause, this is tested through a Backup CR (`spec.quiesce: true`) against its own `audit018-` server rather than by calling `/unquiesce` through `$GP`, since the API gateway does not proxy it (OD-021 item 2).

**Preconditions**

- `audit018-operator` or `audit018-admin` account exists
- No pre-existing GameServer is required: this procedure creates its own `audit018-agt-quiesce-r` GameServer from the `minecraft-java` template (Step 2) (OD-021 item 1)
- The `audit018-restic` restic-server Deployment/Service exist in `gameplane-system`, and the per-round `audit018-restic` repo Secret exists in `gameplane-games` (OD-021 items 9/16)
- `minecraft-java`'s GameTemplate declares `capabilities.quiesce.unquiesce: ["save-on"]` (modules/minecraft-java/template.yaml:329-332)

**Resources created**

- `audit018-agt-quiesce-r` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)
- Backup `audit018-agt-quiesce-r-backup` with `spec.quiesce: true` (created by this procedure; deleted at cleanup)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-quiesce-r","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-quiesce-r:start"
   ```
   Poll `kubectl get gameserver audit018-agt-quiesce-r -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Create the Backup CR:
   ```sh
   kubectl apply -f - <<'YAML'
   apiVersion: gameplane.local/v1alpha1
   kind: Backup
   metadata:
     name: audit018-agt-quiesce-r-backup
     namespace: gameplane-games
     labels:
       gameplane.io/audit: "018"
   spec:
     serverRef:
       name: audit018-agt-quiesce-r
     repoRef:
       name: audit018-restic
       key: repo
     strategy: restic-snapshot
     quiesce: true
   YAML
   ```
   (Cost: 0 additional logins)

4. Wait for the Backup to reach `status.phase = Succeeded` (the controller runs the matching unquiesce once the restic Job finishes, per backup_controller.go's retry-until-it-lands unquiesce logic), saving the watch output to evidence (the Expected claim about the completion timing is backed by this file, not by a terminal-only watch):
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-028
   timeout 300 kubectl get backup audit018-agt-quiesce-r-backup -n gameplane-games -w -o jsonpath='{.status.phase}{"\n"}' \
     | tee specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-028/phase-watch.txt
   ```

5. Download the game log via the agent and confirm the resume line follows the Backup's completion time:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-quiesce-r/logs/download" \
     -o specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-028/game-log-after-backup.log
   grep -n -i "automatic saving is now enabled" \
     specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-028/game-log-after-backup.log \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-028/resume-line.txt
   kubectl get backup audit018-agt-quiesce-r-backup -n gameplane-games -o json \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-028/backup-object.json
   ```

**Expected**

- The Backup reaches `status.phase = Succeeded` with `status.completionTime` set
- The `backup.gameplane.local/unquiesced-at` annotation is set once the restic Job finishes
- The game log shows the auto-save-enabled line (from `save-on`) after the pause sequence and after the restic snapshot completes
- The game world resumes saving normally afterward

**Cleanup**

- Delete the Backup:
   ```sh
   kubectl delete backup audit018-agt-quiesce-r-backup -n gameplane-games
   ```
- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-quiesce-r"
   ```

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### lifecycle-stop

Execute the server's stop sequence to cleanly shut down before scaling to zero.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own `audit018-agt-lifecycle` GameServer from the `minecraft-java` template (Step 2), which supports lifecycle stop (OD-021 item 1)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-lifecycle` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)
- None (triggers shutdown)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-lifecycle","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-lifecycle:start"
   ```
   Poll `kubectl get gameserver audit018-agt-lifecycle -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Trigger the stop sequence:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     "$GP/servers/audit018-agt-lifecycle:stop"
   ```
   (Cost: 0 additional logins)

4. Save response to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-029
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     "$GP/servers/audit018-agt-lifecycle:stop" \
     -w '\nHTTP %{http_code}\n' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-029/stop.txt
   ```

5. Verify the server is shut down, watching the pod transition and saving it to evidence (the Expected claim below is backed by this file, not by a terminal-only check):
   ```sh
   timeout 60 kubectl get pod -n gameplane-games -w | grep audit018-agt-lifecycle \
     | tee specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-029/pod-transition.txt
   ```

**Expected**

- HTTP 202 Accepted with no response body (patchSuspend in api/internal/handlers/lifecycle.go just does `w.WriteHeader(http.StatusAccepted)`; spec.suspend is patched to true and the operator performs the actual shutdown asynchronously)
- Server pod transitions to Terminating state
- Server eventually exits (pod deleted if suspend=true)

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-lifecycle"
   ```
- The server is shut down; restart it if needed for later tests

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### actions-run

Run a module-declared action (custom button) on the server.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own `audit018-agt-actions` GameServer from the `minecraft-java` template (Step 2), which declares the save-all action (OD-021 item 1)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-actions` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)
- None (executes an action)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-actions","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-actions:start"
   ```
   Poll `kubectl get gameserver audit018-agt-actions -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. List available actions for the server:
   ```sh
   TEMPLATE=$(kubectl get gameserver audit018-agt-actions -n gameplane-games -o jsonpath='{.spec.templateRef.name}')
   kubectl get gametemplate "$TEMPLATE" -o jsonpath='{.spec.capabilities.actions}' | jq '.[].id'
   ```

4. Run an action (save-all for Minecraft):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"id":"save-all","params":{}}' \
     "$GP/servers/audit018-agt-actions/actions/run"
   ```
   (Cost: 0 additional logins)

5. Save response to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-030
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"id":"save-all","params":{}}' \
     "$GP/servers/audit018-agt-actions/actions/run" \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-030/actions-run.json
   ```

**Expected**

- HTTP 200 response
- Action executes on the server (visible in logs or game behavior)
- Response indicates success or failure

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-actions"
   ```

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### status-metrics

Retrieve live status metrics (TPS, uptime, etc.) from the server.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own GameServer from the `minecraft-java` template (Step 2), which has declared status metrics (OD-021 item 1)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-status` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-status","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-status:start"
   ```
   Poll `kubectl get gameserver audit018-agt-status -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Get status metrics:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-status/status" | jq '.'
   ```
   (Cost: 0 additional logins)

4. Save response to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-031
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-status/status" \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-031/status.json
   ```

**Expected**

- HTTP 200 response
- JSON array of metric objects with id, displayName, value, unit
- Values are populated if metrics are declared

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-status"
   ```

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### mods-list

List installed mods/plugins on the server.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own `audit018-agt-mods-list` GameServer from the `minecraft-java` template (Step 2), which supports a mods directory (OD-021 item 1)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-mods-list` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-mods-list","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-mods-list:start"
   ```
   Poll `kubectl get gameserver audit018-agt-mods-list -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. List mods:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-mods-list/mods" | jq '.'
   ```
   (Cost: 0 additional logins)

4. Save response to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-032
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-mods-list/mods" \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-032/mods-list.json
   ```

**Expected**

- HTTP 200 response
- JSON array of mod objects with name, size, modTime
- List may be empty if no mods are installed

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-mods-list"
   ```

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### mods-install

Install a mod from a URL.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own GameServer from the `minecraft-java` template (Step 2), which has mods support (OD-021 item 1)
- A valid mod URL is available for the game
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-mods-install` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)
- Installed mod file in the server's mods directory

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-mods-install","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-mods-install:start"
   ```
   Poll `kubectl get gameserver audit018-agt-mods-install -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Install a mod (using a test mod or known good URL):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"url":"https://example.com/audit018-test-mod.jar"}' \
     "$GP/servers/audit018-agt-mods-install/mods/install"
   ```
   (Cost: 0 additional logins)

4. Verify the mod is installed:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-mods-install/mods" | jq '.[] | select(.name | contains("audit018"))'
   ```

5. Save responses to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-033
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"url":"https://example.com/audit018-test-mod.jar"}' \
     "$GP/servers/audit018-agt-mods-install/mods/install" \
     -w '\nHTTP %{http_code}\n' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-033/install.txt
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-mods-install/mods" | jq '.[] | select(.name | contains("audit018"))' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-033/mods-after.json
   ```

**Expected**

- HTTP 200 response
- Mod appears in the mods list
- File exists in the server's mods directory

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-mods-install"
   ```
- No separate mod cleanup: deleting the test GameServer above removes its data volume, including the installed mod

**Automatable?**

Yes. Proposed bucket: `api-agent`. (Requires a stable test mod URL or local file server.)

---

### mods-upload

Upload a mod file directly to the server.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own GameServer from the `minecraft-java` template (Step 2), which has mods support (OD-021 item 1)
- A mod file is available locally
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-mods-upload` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)
- Uploaded mod file in the server's mods directory

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-mods-upload","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-mods-upload:start"
   ```
   Poll `kubectl get gameserver audit018-agt-mods-upload -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Create a test mod file (or use an existing one):
   ```sh
   echo "audit018 test mod" > /tmp/audit018-test-mod.jar
   ```

4. Upload the mod:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -F "file=@/tmp/audit018-test-mod.jar" \
     "$GP/servers/audit018-agt-mods-upload/mods/upload"
   ```
   (Cost: 0 additional logins)

5. Verify the mod is uploaded:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-mods-upload/mods" | jq '.[] | select(.name | contains("audit018"))'
   ```

6. Save responses to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-034
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -F "file=@/tmp/audit018-test-mod.jar" \
     "$GP/servers/audit018-agt-mods-upload/mods/upload" \
     -w '\nHTTP %{http_code}\n' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-034/upload.txt
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-mods-upload/mods" | jq '.[] | select(.name | contains("audit018"))' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-034/mods-after.json
   ```

**Expected**

- HTTP 200 response
- Uploaded mod appears in the mods list
- File is readable

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-mods-upload"
   ```
- No separate mod cleanup: deleting the test GameServer above removes its data volume, including the uploaded mod

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### mods-remove

Remove an installed mod from the server.

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own GameServer from the `minecraft-java` template (Step 2) (OD-021 item 1)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-agt-mods-remove` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)
- `audit018-test-mod.jar`, uploaded by this procedure in Step 3 so the test is self-contained (OD-021 item 1), then removed in Step 5

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-mods-remove","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-mods-remove:start"
   ```
   Poll `kubectl get gameserver audit018-agt-mods-remove -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Upload a test mod first so there is a mod to remove:
   ```sh
   echo "audit018 test mod" > /tmp/audit018-test-mod.jar
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -F "file=@/tmp/audit018-test-mod.jar" \
     "$GP/servers/audit018-agt-mods-remove/mods/upload"
   ```

4. List mods to get the mod name:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-mods-remove/mods" | jq -r '.[] | select(.name | contains("audit018")) | .name' | head -1
   export MOD_NAME=<from-above>
   ```

5. Remove the mod:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-mods-remove/mods?name=$MOD_NAME"
   ```
   (Cost: 0 additional logins)

6. Verify the mod is removed:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-mods-remove/mods" | jq '.[] | select(.name == "'$MOD_NAME'")'
   ```
   (Should return nothing)

7. Save responses to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-035
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-mods-remove/mods?name=$MOD_NAME" \
     -w '\nHTTP %{http_code}\n' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-035/remove.txt
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/audit018-agt-mods-remove/mods" | jq '.[] | select(.name == "'$MOD_NAME'")' \
     > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-035/mods-after.json
   ```

**Expected**

- HTTP 204 No Content response
- Mod no longer appears in the mods list

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-mods-remove"
   ```

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

### heartbeat-metrics

Verify the server reports metrics via Prometheus (heartbeat, player count, resource usage).

**Preconditions**

- `audit018-operator` account exists
- No pre-existing GameServer is required: this procedure creates its own `audit018-agt-heartbeat` GameServer from the `minecraft-java` template (Step 2) (OD-021 item 1)
- `GP=http://127.0.0.1:18080` environment variable is set
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`
- Prometheus endpoint is accessible on the agent pod
- Kubernetes API access available

**Resources created**

- `audit018-agt-heartbeat` GameServer (created from the `minecraft-java` template by this procedure; deleted at cleanup)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create and start the test GameServer from the `minecraft-java` template (OD-021 item 1: each agent procedure creates and deletes its own `audit018-` server instead of addressing the template name as a server):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" -H "Content-Type: application/json" \
     -d '{"apiVersion":"gameplane.local/v1alpha1","kind":"GameServer","metadata":{"name":"audit018-agt-heartbeat","labels":{"gameplane.io/audit":"018"}},"spec":{"templateRef":{"name":"minecraft-java"}}}' \
     "$GP/servers?namespace=gameplane-games"
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     "$GP/servers/audit018-agt-heartbeat:start"
   ```
   Poll `kubectl get gameserver audit018-agt-heartbeat -n gameplane-games -o jsonpath='{.status.phase}'` until `Running` (10m timeout — OD-021 item 24, cold first boot; record the actual boot time).

3. Now that the pod exists, identify the agent pod and export `AGENT_POD` (the agent runs as a sidecar container inside the GameServer's own pod, which the operator labels `app.kubernetes.io/instance=<server-name>` alongside `app.kubernetes.io/name=gameplane-game` in the StatefulSet pod template — operator/internal/controller/gameserver_controller.go:1371-1373, applied at :1379; the game Service selector carries the same pair at :572-575):
   ```sh
   export AGENT_POD=$(kubectl get pod -n gameplane-games -l app.kubernetes.io/instance=audit018-agt-heartbeat -o jsonpath='{.items[0].metadata.name}')
   ```

4. Port-forward to the agent's plain-HTTP metrics listener (agent/cmd/main.go:64, separate from the mTLS control listener on :8090 at :63):
   ```sh
   kubectl port-forward -n gameplane-games "$AGENT_POD" 18090:9090 &
   sleep 2
   ```

5. Fetch the Prometheus metrics:
   ```sh
   curl -s http://localhost:18090/metrics | grep gameplane_
   ```

6. Save metrics to evidence:
   ```sh
   mkdir -p specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-036
   curl -s http://localhost:18090/metrics | grep -E "gameplane_|HELP" > specs/018-v0-3-release-readiness/audit/evidence/INV-AGT-036/metrics.txt
   ```

7. Close the port-forward:
   ```sh
   pkill -f "kubectl port-forward"
   ```

**Expected**

- HTTP 200 response from the metrics endpoint
- Metrics include (agent/internal/metrics/metrics.go:132-167), each labeled `server`, `namespace`, `template`, `game`:
  - `gameplane_agent_cpu_millicores` / `gameplane_agent_cpu_limit_millicores` (CPU used / limit)
  - `gameplane_agent_memory_bytes` / `gameplane_agent_memory_limit_bytes` (memory used / limit)
  - `gameplane_agent_disk_used_bytes` / `gameplane_agent_disk_total_bytes` (disk used / total)
  - `gameplane_agent_players_online` / `gameplane_agent_players_max` (player count / capacity)

**Cleanup**

- Delete the test GameServer:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" "$GP/servers/audit018-agt-heartbeat"
   ```
- Close port-forward
- Delete generated evidence files if needed

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

End of AGT procedures. Total procedures: 36.
