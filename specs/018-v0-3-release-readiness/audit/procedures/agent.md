# Procedures: AGT

Shared conventions: [conventions.md](conventions.md).

## console-source

Execute a command on a game server via source RCON protocol (Minecraft, Valve game servers). The server must have RCON enabled with `consoleMode: rcon` and `rcon-protocol: source`.

**Preconditions**

- `audit018-operator` or `audit018-admin` account exists
- `minecraft-java` module is deployed in the cluster
- A running GameServer `audit018-mc-source` with source RCON protocol is available
- RCON password is configured in the GameServer spec
- `GP=http://127.0.0.1:18080` environment variable is set
- Session cookie and CSRF token for the operator account are saved in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-mc-source` GameServer (copied from pre-existing minecraft-java if needed)

**Steps**

1. Launch the operator session and set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Send a simple RCON command via the agent console WebSocket endpoint:
   ```sh
   (echo -n '{"kind":"cmd","body":"list"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://$GP/ws/servers/audit018-mc-source/console"
   ```
   (Cost: 1 login)

3. Capture the response, which should echo an RCON response frame. Save to evidence file:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-001
   # Repeat step 2 with output redirection
   ```

**Expected**

- WebSocket connection succeeds (101 Switching Protocols)
- RCON response is received as a JSON frame with `kind="out"` and `body` containing the command output
- Response indicates online player count or similar RCON data

**Cleanup**

- Leave the GameServer running for use in other procedures
- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## console-telnet

Execute a command on a game server via telnet RCON protocol (7 Days to Die, Line-based RCon). The server must have telnet RCON configured.

**Preconditions**

- `audit018-operator` or `audit018-admin` account exists
- `7-days-to-die` module is available (telnet RCON on port 8081)
- A running GameServer `audit018-telnet-rcon` with telnet RCON is available
- `GP=http://127.0.0.1:18080` environment variable is set
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-telnet-rcon` GameServer (copied from pre-existing 7-days-to-die if needed)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Send a telnet RCON command:
   ```sh
   (echo -n '{"kind":"cmd","body":"help"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://$GP/ws/servers/audit018-telnet-rcon/console"
   ```
   (Cost: 1 login)

3. Save response to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-002
   ```

**Expected**

- WebSocket connection succeeds
- Telnet RCON response received with command output
- Response shows help text or acknowledgment

**Cleanup**

- Leave the GameServer running
- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## console-websocket

Execute a command on a game server via websocket RCON protocol (Rust WebRcon). The server must have `consoleMode: rcon` and `rcon-protocol: websocket`.

**Preconditions**

- `audit018-operator` account exists
- `rust` module is available
- A running GameServer `audit018-rust-ws` with websocket RCON is available
- `GP=http://127.0.0.1:18080` environment variable is set
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-rust-ws` GameServer

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Send a websocket RCON command:
   ```sh
   (echo -n '{"kind":"cmd","body":"status"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://$GP/ws/servers/audit018-rust-ws/console"
   ```
   (Cost: 1 login)

3. Save response to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-003
   ```

**Expected**

- WebSocket connection succeeds
- Websocket RCON response received
- Response contains server status data

**Cleanup**

- Leave the GameServer running
- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## console-battleye

Execute a command on a game server via BattlEye RCON protocol (DayZ, Arma). The server must have `rcon-protocol: battleye`.

**Preconditions**

- `audit018-operator` account exists
- `dayz` module is available
- A running GameServer `audit018-dayz-be` with BattlEye RCON is available
- `GP=http://127.0.0.1:18080` environment variable is set
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-dayz-be` GameServer

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Send a BattlEye RCON command:
   ```sh
   (echo -n '{"kind":"cmd","body":"players"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://$GP/ws/servers/audit018-dayz-be/console"
   ```
   (Cost: 1 login)

3. Save response to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-004
   ```

**Expected**

- WebSocket connection succeeds
- BattlEye RCON response received with player list or acknowledgment

**Cleanup**

- Leave the GameServer running
- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## console-satisfactory

Execute a command on a game server via Satisfactory HTTPS admin API RCON protocol.

**Preconditions**

- `audit018-operator` account exists
- `satisfactory` module is available
- A running GameServer `audit018-satisfactory-api` with satisfactory RCON is available
- `GP=http://127.0.0.1:18080` environment variable is set
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-satisfactory-api` GameServer

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Send a Satisfactory RCON command:
   ```sh
   (echo -n '{"kind":"cmd","body":"GetGamePhase"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://$GP/ws/servers/audit018-satisfactory-api/console"
   ```
   (Cost: 1 login)

3. Save response to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-005
   ```

**Expected**

- WebSocket connection succeeds
- Satisfactory HTTPS API response received

**Cleanup**

- Leave the GameServer running
- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## console-palworld

Execute a command on a game server via Palworld REST admin API RCON protocol.

**Preconditions**

- `audit018-operator` account exists
- `palworld` module is available
- A running GameServer `audit018-palworld-api` with palworld RCON is available
- `GP=http://127.0.0.1:18080` environment variable is set
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-palworld-api` GameServer

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Send a Palworld RCON command:
   ```sh
   (echo -n '{"kind":"cmd","body":"Info"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://$GP/ws/servers/audit018-palworld-api/console"
   ```
   (Cost: 1 login)

3. Save response to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-006
   ```

**Expected**

- WebSocket connection succeeds
- Palworld REST API response received with server info

**Cleanup**

- Leave the GameServer running
- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## console-nuclearoption

Execute a command on a game server via Nuclear Option remote-command RCON protocol.

**Preconditions**

- `audit018-operator` account exists
- `nuclear-option` module is available
- A running GameServer `audit018-nuclear-api` with nuclearoption RCON is available
- `GP=http://127.0.0.1:18080` environment variable is set
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-nuclear-api` GameServer

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Send a Nuclear Option RCON command:
   ```sh
   (echo -n '{"kind":"cmd","body":"status"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://$GP/ws/servers/audit018-nuclear-api/console"
   ```
   (Cost: 1 login)

3. Save response to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-007
   ```

**Expected**

- WebSocket connection succeeds
- Nuclear Option RCON response received

**Cleanup**

- Leave the GameServer running
- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## console-rest

Execute a command on a game server via generic HTTP/JSON REST admin API RCON protocol (FiveM, Farming Simulator 25).

**Preconditions**

- `audit018-operator` account exists
- `fivem` module is available
- A running GameServer `audit018-fivem-rest` with rest RCON is available
- `GP=http://127.0.0.1:18080` environment variable is set
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-fivem-rest` GameServer

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Send a REST RCON command:
   ```sh
   (echo -n '{"kind":"cmd","body":"status"}'; sleep 0.5) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://$GP/ws/servers/audit018-fivem-rest/console"
   ```
   (Cost: 1 login)

3. Save response to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-008
   ```

**Expected**

- WebSocket connection succeeds
- REST API response received with server status

**Cleanup**

- Leave the GameServer running
- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## console-pty

Execute a command on a game server via PTY console (pod attach, for games without RCON like Terraria, Valheim, Arma Reforger).

**Preconditions**

- `audit018-operator` account exists
- A game with `consoleMode: pty` is available (e.g., `terraria`)
- A running GameServer `audit018-terraria-pty` is available
- `GP=http://127.0.0.1:18080` environment variable is set
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-terraria-pty` GameServer

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Send PTY stdin via WebSocket (pod attach path):
   ```sh
   (echo -n '{"kind":"stdin","body":"aGVscApg"}'; sleep 1) | \
     curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://$GP/ws/servers/audit018-terraria-pty/console-pty"
   ```
   (The base64 blob is "help\n" encoded)
   (Cost: 1 login)

3. Save response to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-009
   ```

**Expected**

- WebSocket connection succeeds (pod attach via Kubernetes API)
- PTY stdout is received as base64-encoded frames with `kind="stdout"`
- Terminal output is visible when decoded

**Cleanup**

- Leave the GameServer running
- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## files-list

List files and directories in the server's data volume.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer with files volume is available (use `minecraft-java`)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- None

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. List the root directory:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/files/list?path=/" | jq '.'
   ```
   (Cost: 0 additional logins)

3. Save response to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-010
   # Redirect output to JSON file
   ```

**Expected**

- HTTP 200 response
- JSON array of file/directory entries with name, path, size, isDir, modTime

**Cleanup**

- None

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## files-read

Read the contents of a text file from the server's data volume.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available
- A known text file exists (e.g., server.properties in Minecraft)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- None

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Read a file:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/files/read?path=/server.properties"
   ```
   (Cost: 0 additional logins)

3. Save response to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-011
   ```

**Expected**

- HTTP 200 response
- File contents returned as JSON with data field

**Cleanup**

- None

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## files-write

Write or overwrite a file in the server's data volume.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-test-file.txt` in the server's data volume

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Write a test file:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"path":"/audit018-test-file.txt","data":"Hello, audit!"}' \
     "$GP/servers/minecraft-java/files/write"
   ```
   (Cost: 0 additional logins)

3. Verify by reading the file back:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/files/read?path=/audit018-test-file.txt"
   ```

4. Save responses to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-012
   ```

**Expected**

- HTTP 200 response from write
- File contents are readable via the read endpoint
- Contents match what was written

**Cleanup**

- Delete the test file:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"path":"/audit018-test-file.txt"}' \
     "$GP/servers/minecraft-java/files/delete"
   ```

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## files-upload

Upload a file to the server's data volume.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-uploaded.txt` in the server's data volume

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create a test file locally:
   ```sh
   echo "Uploaded test file" > /tmp/audit018-upload.txt
   ```

3. Upload the file:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -F "path=/" \
     -F "file=@/tmp/audit018-upload.txt" \
     "$GP/servers/minecraft-java/files/upload"
   ```
   (Cost: 0 additional logins)

4. Verify by listing the directory:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/files/list?path=/" | jq '.[] | select(.name == "audit018-upload.txt")'
   ```

5. Save responses to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-013
   ```

**Expected**

- HTTP 200 response from upload
- File appears in directory listing
- File is readable

**Cleanup**

- Delete the uploaded file via files-delete

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## files-download

Download a file from the server's data volume.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available
- A known file exists in the server
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- None

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Download a file:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/files/download?path=/server.properties" \
     -o /tmp/audit018-downloaded.properties
   file /tmp/audit018-downloaded.properties
   ```
   (Cost: 0 additional logins)

3. Save metadata to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-014
   ls -lh /tmp/audit018-downloaded.properties
   ```

**Expected**

- HTTP 200 response
- File binary is downloaded with correct content-disposition header
- Downloaded file is readable and matches original

**Cleanup**

- Remove the downloaded file

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## files-mkdir

Create a directory in the server's data volume.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- `audit018-testdir` directory in the server's data volume

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create a directory:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"path":"/audit018-testdir"}' \
     "$GP/servers/minecraft-java/files/mkdir"
   ```
   (Cost: 0 additional logins)

3. Verify by listing:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/files/list?path=/" | jq '.[] | select(.name == "audit018-testdir")'
   ```

4. Save responses to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-015
   ```

**Expected**

- HTTP 200 response
- Directory appears in listing with isDir=true

**Cleanup**

- Delete the directory via files-delete

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## files-delete

Delete a file or directory from the server's data volume.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available
- A test file or directory exists (from other procedures)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- None (deletes existing resources)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Delete the test file:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"path":"/audit018-test-file.txt"}' \
     "$GP/servers/minecraft-java/files/delete"
   ```
   (Cost: 0 additional logins)

3. Verify by attempting to read the deleted file (should fail):
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/files/read?path=/audit018-test-file.txt" | jq .
   ```

4. Save responses to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-016
   ```

**Expected**

- HTTP 200 response from delete
- Subsequent read returns 404 or error
- File no longer appears in directory listing

**Cleanup**

- None

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## logs-tail

Stream the server's log file via WebSocket in real-time.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available with a log file configured
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- None

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Open the log tail WebSocket and read 5 seconds of logs:
   ```sh
   timeout 5 curl -i -N --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "ws://$GP/ws/servers/minecraft-java/logs" 2>&1 | head -50
   ```
   (Cost: 1 login)

3. Save output to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-017
   ```

**Expected**

- WebSocket connection succeeds (101 Switching Protocols)
- Log entries are streamed as JSON frames
- Log data is base64-encoded in the frames

**Cleanup**

- Close the WebSocket connection

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## logs-download

Download the complete log file from the server.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available with a log file
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- None

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Download the log file:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/logs/download" \
     -o /tmp/audit018-game.log
   wc -l /tmp/audit018-game.log
   file /tmp/audit018-game.log
   ```
   (Cost: 0 additional logins)

3. Save metadata to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-018
   ls -lh /tmp/audit018-game.log
   head -20 /tmp/audit018-game.log
   ```

**Expected**

- HTTP 200 response
- Log file is downloaded as plain text
- File contains readable log entries

**Cleanup**

- Remove the downloaded file

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## players-list

List online players on the server.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer with player list support is available (minecraft-java)
- At least one player is online (or procedure documents empty list)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- None

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Get the player list:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/players/" | jq '.'
   ```
   (Cost: 0 additional logins)

3. Save response to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-019
   ```

**Expected**

- HTTP 200 response
- JSON response with player list (players array)
- Each player has UUID, name, joinTime

**Cleanup**

- None

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## players-banned

List banned players on the server.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer with ban support is available
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- None

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Get the banned players list:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/players/banned" | jq '.'
   ```
   (Cost: 0 additional logins)

3. Save response to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-020
   ```

**Expected**

- HTTP 200 response
- JSON response with banned list (may be empty)

**Cleanup**

- None

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## players-kick

Kick a player from the server.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available
- At least one player is online to kick
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- None (removes a player session)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. First, list players to get a valid UUID:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/players/" | jq -r '.players[0].uuid'
   export PLAYER_UUID=<from-above>
   ```

3. Kick the player:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d "{\"uuid\":\"$PLAYER_UUID\",\"reason\":\"audit018\"}" \
     "$GP/servers/minecraft-java/players/kick"
   ```
   (Cost: 0 additional logins)

4. Save responses to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-021
   ```

**Expected**

- HTTP 200 response
- Player is disconnected from the server
- Player does not appear in subsequent player list

**Cleanup**

- None (the player was already removed)

**Automatable?**

Yes. Proposed bucket: `api-agent`. (Requires test player bot for consistent automation.)

---

## players-ban

Ban a player from the server.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- Ban entry for the test player

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Ban a player (using a known or test player UUID):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"uuid":"00000000-0000-0000-0000-000000000001","name":"audit018-test-player","reason":"audit"}' \
     "$GP/servers/minecraft-java/players/ban"
   ```
   (Cost: 0 additional logins)

3. Verify the player is in the banned list:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/players/banned" | jq '.[] | select(.uuid == "00000000-0000-0000-0000-000000000001")'
   ```

4. Save responses to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-022
   ```

**Expected**

- HTTP 200 response
- Player appears in the banned list
- Player cannot rejoin the server

**Cleanup**

- Unban the player via players-unban

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## players-unban

Unban a player from the server.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available
- A player is currently banned
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- None (removes a ban entry)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Unban the player:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"uuid":"00000000-0000-0000-0000-000000000001"}' \
     "$GP/servers/minecraft-java/players/unban"
   ```
   (Cost: 0 additional logins)

3. Verify the player is no longer banned:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/players/banned" | jq '.[] | select(.uuid == "00000000-0000-0000-0000-000000000001")'
   ```
   (Should return nothing)

4. Save responses to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-023
   ```

**Expected**

- HTTP 200 response
- Player no longer appears in the banned list
- Player can rejoin the server

**Cleanup**

- None

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## players-whitelist

List whitelisted players on the server.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer with whitelist support is available
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- None

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Get the whitelist:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/players/whitelist" | jq '.'
   ```
   (Cost: 0 additional logins)

3. Save response to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-024
   ```

**Expected**

- HTTP 200 response
- JSON response with whitelist (array of players)

**Cleanup**

- None

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## players-whitelist-add

Add a player to the server's whitelist.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer with whitelist support is available
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- Whitelist entry for the test player

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Add a player to the whitelist:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"uuid":"00000000-0000-0000-0000-000000000002","name":"audit018-whitelisted"}' \
     "$GP/servers/minecraft-java/players/whitelist/add"
   ```
   (Cost: 0 additional logins)

3. Verify the player is in the whitelist:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/players/whitelist" | jq '.[] | select(.uuid == "00000000-0000-0000-0000-000000000002")'
   ```

4. Save responses to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-025
   ```

**Expected**

- HTTP 200 response
- Player appears in the whitelist

**Cleanup**

- Remove the player via players-whitelist-remove

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## players-whitelist-remove

Remove a player from the server's whitelist.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available
- A player is currently whitelisted
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- None (removes a whitelist entry)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Remove the player from the whitelist:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"uuid":"00000000-0000-0000-0000-000000000002"}' \
     "$GP/servers/minecraft-java/players/whitelist/remove"
   ```
   (Cost: 0 additional logins)

3. Verify the player is no longer in the whitelist:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/players/whitelist" | jq '.[] | select(.uuid == "00000000-0000-0000-0000-000000000002")'
   ```
   (Should return nothing)

4. Save responses to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-026
   ```

**Expected**

- HTTP 200 response
- Player no longer appears in the whitelist

**Cleanup**

- None

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## quiesce-pause

Pause game writes (quiesce) before taking a backup snapshot.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available (supports quiesce, e.g., minecraft-java)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- None (temporary pause state)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Pause writes:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     "$GP/servers/minecraft-java/quiesce"
   ```
   (Cost: 0 additional logins)

3. Save response to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-027
   ```

**Expected**

- HTTP 200 response
- Response body contains `{"quiesced":true}`
- Game world stops saving to disk

**Cleanup**

- Resume writes via quiesce-resume

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## quiesce-resume

Resume game writes (unquiesce) after a backup snapshot completes.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available
- The server is currently quiesced
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- None (resumes normal state)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Resume writes:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     "$GP/servers/minecraft-java/unquiesce"
   ```
   (Cost: 0 additional logins)

3. Save response to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-028
   ```

**Expected**

- HTTP 200 response
- Response body contains `{"quiesced":false}`
- Game world resumes saving

**Cleanup**

- None

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## lifecycle-stop

Execute the server's stop sequence to cleanly shut down before scaling to zero.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available (supports lifecycle stop, e.g., minecraft-java)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- None (triggers shutdown)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Trigger the stop sequence:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     "$GP/servers/minecraft-java/lifecycle/stop"
   ```
   (Cost: 0 additional logins)

3. Save response to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-029
   ```

4. Verify the server is shut down:
   ```sh
   kubectl get pod -n gameplane-games | grep minecraft-java
   ```

**Expected**

- HTTP 200 response
- Response body contains `{"stopped":true}`
- Server pod transitions to Terminating state
- Server eventually exits (pod deleted if suspend=true)

**Cleanup**

- The server is shut down; restart it if needed for later tests

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## actions-run

Run a module-declared action (custom button) on the server.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available with declared actions (e.g., minecraft-java has save-all action)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- None (executes an action)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. List available actions for the server:
   ```sh
   kubectl get gameserver minecraft-java -n gameplane-games -o jsonpath='{.spec.template.spec.gameTemplate}' | \
     kubectl get gametemplate -n gameplane-games -o jsonpath='{.spec.capabilities.actions}' | jq '.[].id'
   ```

3. Run an action (save-all for Minecraft):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"actionId":"save-all","params":{}}' \
     "$GP/servers/minecraft-java/actions/run"
   ```
   (Cost: 0 additional logins)

4. Save response to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-030
   ```

**Expected**

- HTTP 200 response
- Action executes on the server (visible in logs or game behavior)
- Response indicates success or failure

**Cleanup**

- None

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## status-metrics

Retrieve live status metrics (TPS, uptime, etc.) from the server.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available with declared status metrics
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- None

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Get status metrics:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/status" | jq '.'
   ```
   (Cost: 0 additional logins)

3. Save response to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-031
   ```

**Expected**

- HTTP 200 response
- JSON array of metric objects with id, displayName, value, unit
- Values are populated if metrics are declared

**Cleanup**

- None

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## mods-list

List installed mods/plugins on the server.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available with mods support (e.g., minecraft-java with mods directory)
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- None

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. List mods:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/mods" | jq '.'
   ```
   (Cost: 0 additional logins)

3. Save response to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-032
   ```

**Expected**

- HTTP 200 response
- JSON array of mod objects with name, size, modTime
- List may be empty if no mods are installed

**Cleanup**

- None

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## mods-install

Install a mod from a URL.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available with mods support
- A valid mod URL is available for the game
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- Installed mod file in the server's mods directory

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Install a mod (using a test mod or known good URL):
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d '{"url":"https://example.com/audit018-test-mod.jar"}' \
     "$GP/servers/minecraft-java/mods/install"
   ```
   (Cost: 0 additional logins)

3. Verify the mod is installed:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/mods" | jq '.[] | select(.name | contains("audit018"))'
   ```

4. Save responses to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-033
   ```

**Expected**

- HTTP 200 response
- Mod appears in the mods list
- File exists in the server's mods directory

**Cleanup**

- Delete the installed mod via mods-remove

**Automatable?**

Yes. Proposed bucket: `api-agent`. (Requires a stable test mod URL or local file server.)

---

## mods-upload

Upload a mod file directly to the server.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available with mods support
- A mod file is available locally
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- Uploaded mod file in the server's mods directory

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. Create a test mod file (or use an existing one):
   ```sh
   echo "audit018 test mod" > /tmp/audit018-test-mod.jar
   ```

3. Upload the mod:
   ```sh
   curl -s -X POST --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -F "file=@/tmp/audit018-test-mod.jar" \
     "$GP/servers/minecraft-java/mods/upload"
   ```
   (Cost: 0 additional logins)

4. Verify the mod is uploaded:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/mods" | jq '.[] | select(.name | contains("audit018"))'
   ```

5. Save responses to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-034
   ```

**Expected**

- HTTP 200 response
- Uploaded mod appears in the mods list
- File is readable

**Cleanup**

- Delete the uploaded mod via mods-remove

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## mods-remove

Remove an installed mod from the server.

**Preconditions**

- `audit018-operator` account exists
- A running GameServer is available
- At least one test mod is installed
- Session cookie and CSRF token are in `~/gameplane-audit-018/session-operator.txt`

**Resources created**

- None (removes a mod)

**Steps**

1. Set environment variables:
   ```sh
   export GP=http://127.0.0.1:18080
   export SESSION=$(grep gameplane_session ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   export CSRF=$(grep gameplane_csrf ~/gameplane-audit-018/session-operator.txt | cut -d= -f2)
   ```

2. First, list mods to get the mod name:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/mods" | jq -r '.[] | select(.name | contains("audit018")) | .name' | head -1
   export MOD_NAME=<from-above>
   ```

3. Remove the mod:
   ```sh
   curl -s -X DELETE --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     -H "X-Gameplane-CSRF: $CSRF" \
     -H "Content-Type: application/json" \
     -d "{\"name\":\"$MOD_NAME\"}" \
     "$GP/servers/minecraft-java/mods"
   ```
   (Cost: 0 additional logins)

4. Verify the mod is removed:
   ```sh
   curl -s --cookie "gameplane_session=$SESSION; gameplane_csrf=$CSRF" \
     "$GP/servers/minecraft-java/mods" | jq '.[] | select(.name == "'$MOD_NAME'")'
   ```
   (Should return nothing)

5. Save responses to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-035
   ```

**Expected**

- HTTP 200 response
- Mod no longer appears in the mods list

**Cleanup**

- None

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

## heartbeat-metrics

Verify the server reports metrics via Prometheus (heartbeat, player count, resource usage).

**Preconditions**

- A running GameServer is available (e.g., minecraft-java)
- Prometheus endpoint is accessible on the agent pod
- Kubernetes API access available

**Resources created**

- None

**Steps**

1. Identify the agent pod:
   ```sh
   kubectl get pod -n gameplane-games -l gameplane.io/component=agent | grep minecraft-java
   export AGENT_POD=<pod-name>
   ```

2. Port-forward to the agent metrics endpoint:
   ```sh
   kubectl port-forward -n gameplane-games "$AGENT_POD" 18090:8090 &
   sleep 2
   ```

3. Fetch the Prometheus metrics:
   ```sh
   curl -s http://localhost:18090/metrics | grep gameplane_
   ```

4. Save metrics to evidence:
   ```sh
   mkdir -p ~/gameplane-audit-018/evidence/INV-AGT-036
   curl -s http://localhost:18090/metrics | grep -E "gameplane_|HELP" > ~/gameplane-audit-018/evidence/INV-AGT-036/metrics.txt
   ```

5. Close the port-forward:
   ```sh
   pkill -f "kubectl port-forward"
   ```

**Expected**

- HTTP 200 response from the metrics endpoint
- Metrics include:
  - `gameplane_agent_info` (server name, template, game, version)
  - `gameplane_players` (online player count)
  - `gameplane_cpu_usage_percent` (CPU usage)
  - `gameplane_memory_usage_bytes` (memory usage)

**Cleanup**

- Close port-forward
- Delete generated evidence files if needed

**Automatable?**

Yes. Proposed bucket: `api-agent`.

---

End of AGT procedures. Total procedures: 36.
