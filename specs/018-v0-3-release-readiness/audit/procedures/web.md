# Procedures: WEB

Shared conventions: [conventions.md](conventions.md).

### login-username-password

**Preconditions:** User has local credentials enabled. GET $GP/auth/providers returns a provider with kind="local".

**Resources created:** none

**Steps:**
1. Navigate to $GP/login
2. Enter username in "Email or username" field
3. Enter password in "Password" field
4. Click "Sign in →" button

**Expected:** Dashboard loads; gameplane_session and gameplane_csrf cookies are set.

**Cleanup:** Session ends at logout or manual cookie deletion.

**Automatable?** yes (api-auth)

---

### login-toggle-password-visibility

**Preconditions:** Login page is visible with password form.

**Resources created:** none

**Steps:**
1. Navigate to $GP/login
2. Click the eye/eye-off icon on the password field
3. Observe the password field

**Expected:** Password visibility toggles between hidden (bullets) and plaintext.

**Cleanup:** None needed.

**Automatable?** no (visual UI behavior only)

---

### login-sso-provider

**Preconditions:** GET $GP/auth/providers returns at least one provider with kind != "local".

**Resources created:** none

**Steps:**
1. Navigate to $GP/login
2. Click "Continue with [provider name]" button
3. Follow the SSO flow (details vary by provider)

**Expected:** User is authenticated and redirected to dashboard (or provider-specific finish page).

**Cleanup:** SSO session cleaned up by provider logout (out of scope).

**Automatable?** no (requires external SSO interaction)

---

### login-safe-mode

**Preconditions:** Login form is visible.

**Resources created:** none

**Steps:**
1. Navigate to $GP/login
2. Enter credentials
3. Click "Sign in with safe mode (custom styling disabled)" link

**Expected:** User logs in; sessionStorage sets SAFE_MODE_SESSION_KEY; custom CSS overlay is not applied on the authenticated session.

**Cleanup:** Clear sessionStorage or log out.

**Automatable?** yes (api-auth)

---

### login-password-forgotten-hint

**Preconditions:** Login page with password form visible.

**Resources created:** none

**Steps:**
1. Navigate to $GP/login
2. Click "Forgot?" button next to password field

**Expected:** Text reveals: "Contact your administrator to reset your password."

**Cleanup:** None needed.

**Automatable?** no (static UI reveal)

---

### dashboard-view-fleet-summary

**Preconditions:** User is authenticated and on dashboard.

**Resources created:** none

**Steps:**
1. Navigate to $GP/
2. Observe the Fleet Status card

**Expected:** Card shows total servers, running count, stopped count, failed count, and list of servers needing attention.

**Cleanup:** None needed.

**Automatable?** yes (operator)

---

### dashboard-view-cluster-resources

**Preconditions:** User is authenticated and has servers:write permission (or dashboard still shows general metrics).

**Resources created:** none

**Steps:**
1. Navigate to $GP/
2. Observe the Cluster Resources card

**Expected:** Card shows CPU, Memory, Storage usage bars and node list.

**Cleanup:** None needed.

**Automatable?** yes (operator)

---

### dashboard-view-recent-activity

**Preconditions:** User has audit:read permission.

**Resources created:** none

**Steps:**
1. Navigate to $GP/
2. Observe Recent Activity card (if visible)

**Expected:** Card shows recent audit events with icons for action types. "View all" link navigates to /admin/audit.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### dashboard-view-recent-backups

**Preconditions:** Backups are configured and some have completed.

**Resources created:** none

**Steps:**
1. Navigate to $GP/
2. Observe Recent Backups card

**Expected:** Card shows latest backups by server and completion time. "View all" link navigates to /backups.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### dashboard-create-server-button

**Preconditions:** User is authenticated.

**Resources created:** none

**Steps:**
1. Navigate to $GP/
2. Click "Create server" button in page header

**Expected:** Navigate to /servers/new wizard.

**Cleanup:** Cancel the wizard without creating a server.

**Automatable?** no (navigation only)

---

### servers-list-view

**Preconditions:** User is authenticated.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers
2. Observe the server table/card list

**Expected:** All servers in the cluster are listed with status, CPU, memory, player count, node, and action buttons. Mobile uses cards, desktop uses table.

**Cleanup:** None needed.

**Automatable?** yes (operator)

---

### servers-filter-by-status

**Preconditions:** Multiple servers exist in different states.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers
2. Click the "Running" or "Stopped" tab in the status filter
3. Observe the list

**Expected:** List filters to only running or stopped servers. Counter updates.

**Cleanup:** Click "All" to reset.

**Automatable?** yes (operator)

---

### servers-search-by-name

**Preconditions:** Multiple servers with distinguishable names.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers
2. Type a server name prefix in the "Search servers…" input
3. Observe the list

**Expected:** List filters to servers matching the query (case-insensitive substring).

**Cleanup:** Clear input.

**Automatable?** yes (operator)

---

### servers-filter-by-game

**Preconditions:** Servers of different game types exist.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers
2. Click "Filter" button
3. Toggle one or more game checkboxes
4. Click "Apply"

**Expected:** List shows only servers of selected games. "Filter" button shows a badge with count.

**Cleanup:** Click "Filter", then "Clear", then "Apply".

**Automatable?** yes (operator)

---

### servers-filter-by-namespace

**Preconditions:** Servers exist in non-default namespaces.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers
2. Click "Filter" button
3. Toggle one or more namespace checkboxes
4. Click "Apply"

**Expected:** List shows only servers in selected namespaces.

**Cleanup:** Clear and reapply filter.

**Automatable?** yes (operator)

---

### servers-open-detail

**Preconditions:** At least one server exists.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers
2. Click on a server name in the table/card

**Expected:** Navigate to /servers/[name] detail page.

**Cleanup:** Navigate back to /servers.

**Automatable?** no (navigation only)

---

### servers-start-server

**Preconditions:** Server exists and is in Stopped/Suspended/Failed state.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers
2. Locate a stopped server
3. Click the play icon button in the Actions column (or card) for that server

**Expected:** Server transitions to Starting phase. Status badge updates in real time.

**Cleanup:** Server reaches Running state or fails; no manual cleanup needed.

**Automatable?** yes (api-agent)

---

### servers-stop-server

**Preconditions:** Server is Running or Asleep.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers
2. Locate a running server
3. Click the stop icon button in the Actions column

**Expected:** Server transitions to Stopping then Stopped. Badge updates.

**Cleanup:** Server halts; no manual cleanup.

**Automatable?** yes (api-agent)

---

### servers-restart-server

**Preconditions:** Server is Running.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers
2. Locate a running server
3. Click the restart icon button

**Expected:** Server restarts. Badge shows Stopping briefly, then Running again.

**Cleanup:** Server returns to Running.

**Automatable?** yes (api-agent)

---

### servers-wake-sleeping-server

**Preconditions:** Server is Suspended/asleep (indicated by "Wake" button visible).

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers
2. Locate an asleep server (has Sunrise icon button)
3. Click the wake button

**Expected:** Server wakes and transitions to Running.

**Cleanup:** Server reaches Running.

**Automatable?** yes (api-agent)

---

### servers-menu-actions

**Preconditions:** Server exists and is not shared read-only.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers
2. Click the "⋯" menu button in the Actions column for a server
3. Observe menu items (Clone server, Transfer ownership, Wipe world data, Delete server)

**Expected:** Dropdown menu shows available actions.

**Cleanup:** Close menu without action.

**Automatable?** no (menu inspection only)

---

### server-detail-overview-tab

**Preconditions:** Server exists and is viewed in detail page.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name]
2. Click "Overview" tab (default)

**Expected:** Tab shows resource metrics (CPU, memory, disk), player count, endpoint(s), tunnel status, recent events.

**Cleanup:** None needed.

**Automatable?** yes (operator)

---

### server-detail-lifecycle-start

**Preconditions:** Server is in Stopped/Suspended/Failed state.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name]
2. Click "Start" button in header

**Expected:** Server transitions to Starting.

**Cleanup:** Server reaches Running or fails.

**Automatable?** yes (api-agent)

---

### server-detail-lifecycle-stop

**Preconditions:** Server is Running or Asleep.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name]
2. Click "Stop" button in header

**Expected:** Server transitions to Stopping then Stopped.

**Cleanup:** None needed.

**Automatable?** yes (api-agent)

---

### server-detail-lifecycle-restart

**Preconditions:** Server is Running.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name]
2. Click "Restart" button in header

**Expected:** Server restarts (Stopping → Running).

**Cleanup:** Server reaches Running.

**Automatable?** yes (api-agent)

---

### server-detail-lifecycle-wake

**Preconditions:** Server is Suspended/asleep.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name]
2. Click "Wake" button in header

**Expected:** Server transitions to Running.

**Cleanup:** None needed.

**Automatable?** yes (api-agent)

---

### server-detail-open-console

**Preconditions:** Server supports a console (consoleMode != "none").

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name]
2. Click "Open console" button in header

**Expected:** Navigate to Console tab; terminal emulator loads (xterm).

**Cleanup:** Navigate away.

**Automatable?** no (terminal interaction)

---

### server-detail-console-send-command

**Preconditions:** Server is Running and console is connected.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Console" tab
2. Type a command in the terminal
3. Press Enter

**Expected:** Command is sent to the server; output appears in terminal.

**Cleanup:** None needed (command executes on server).

**Automatable?** yes (api-agent) with caveats (game-specific)

---

### server-detail-logs-view-pod-logs

**Preconditions:** Server exists.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name]
2. Click "Logs" tab
3. Observe pod log stream

**Expected:** Container's stdout is streamed and displayed. Defaults to "Container output" source.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### server-detail-logs-view-game-logs

**Preconditions:** Server template specifies a logPath.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Logs" tab
2. Switch to "Game logs" source tab (if available)

**Expected:** Agent tails the game log file. Stream may fail with actionable error.

**Cleanup:** None needed.

**Automatable?** yes (api-agent) with caveats

---

### server-detail-logs-filter-by-level

**Preconditions:** Logs are visible.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Logs" tab
2. Click a log level filter button (INFO, WARN, ERROR, DEBUG)

**Expected:** Log lines are filtered by detected level.

**Cleanup:** Click "All" to show all levels.

**Automatable?** no (client-side filter only)

---

### server-detail-logs-download

**Preconditions:** Logs are loaded.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Logs" tab
2. Click "Download" button

**Expected:** Current log buffer is downloaded as a .txt file.

**Cleanup:** None needed.

**Automatable?** no (download to client)

---

### server-detail-files-browse

**Preconditions:** Server is Running or has been started (agent is available).

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name]
2. Click "Files" tab
3. Navigate through directory tree

**Expected:** Files and folders are listed. Clicking a folder navigates into it.

**Cleanup:** None needed.

**Automatable?** yes (api-agent)

---

### server-detail-files-view-file

**Preconditions:** Files are browsable.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Files" tab
2. Click a file in the tree
3. Observe the file content in the editor pane

**Expected:** File is loaded in Monaco editor (or read-only view). Line count, language syntax highlighting shown.

**Cleanup:** None needed.

**Automatable?** yes (api-agent)

---

### server-detail-files-edit-file

**Preconditions:** File is loaded in editor; changes are made.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Files" tab
2. Select a file
3. Edit content in the editor
4. Click "Save" button

**Expected:** File is written back to the server. The "modified" indicator next to the filename clears.

**Cleanup:** Server writes file; no manual cleanup.

**Automatable?** yes (api-agent)

---

### server-detail-files-upload-file

**Preconditions:** Server is browsable and user is in a directory.

**Resources created:** one file (audit018-<purpose>-uploaded)

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Files" tab
2. Click "Upload" button (upload icon)
3. Select a local file
4. Wait for upload to complete

**Expected:** File is uploaded to the current directory. File list updates.

**Cleanup:** Delete the uploaded file via the Files UI.

**Automatable?** yes (api-agent)

---

### server-detail-files-create-file

**Preconditions:** Directory is selected.

**Resources created:** one empty file (audit018-<purpose>)

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Files" tab
2. Click "New file" button
3. Enter a filename
4. Click create

**Expected:** Empty file is created in the current directory.

**Cleanup:** Delete via Files UI.

**Automatable?** yes (api-agent)

---

### server-detail-files-create-directory

**Preconditions:** Directory is selected.

**Resources created:** one directory (audit018-<purpose>)

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Files" tab
2. Click "New folder" button
3. Enter a directory name
4. Click create

**Expected:** Directory is created; navigation updates.

**Cleanup:** Delete via Files UI.

**Automatable?** yes (api-agent)

---

### server-detail-files-delete-file

**Preconditions:** File exists and is selected.

**Resources created:** none (file is deleted)

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Files" tab
2. Click the delete icon (trash) on a file
3. Confirm in the dialog

**Expected:** File is deleted. List updates.

**Cleanup:** None needed.

**Automatable?** yes (api-agent)

---

### server-detail-files-download-file

**Preconditions:** File is selected.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Files" tab
2. Click the download icon on a file

**Expected:** File is downloaded to the client's Downloads folder.

**Cleanup:** None needed.

**Automatable?** no (download to client)

---

### server-detail-events-view

**Preconditions:** Server exists and has Kubernetes events.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name]
2. Click "Events" tab

**Expected:** Kubernetes events (pod, StatefulSet, GameServer) are listed with timestamps and messages.

**Cleanup:** None needed.

**Automatable?** yes (operator)

---

### server-detail-events-filter

**Preconditions:** Events are displayed.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Events" tab
2. Click filter buttons (all, info, warnings)

**Expected:** Event list is filtered by kind.

**Cleanup:** Click "All" to reset.

**Automatable?** no (client-side filter)

---

### server-detail-mods-list

**Preconditions:** Server template declares mod support.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name]
2. Click "Mods" tab (if visible)

**Expected:** List of installed mods with versions and removal buttons.

**Cleanup:** None needed.

**Automatable?** yes (api-agent)

---

### server-detail-mods-install-url

**Preconditions:** Mod list is visible and template supports URL installs.

**Resources created:** one mod (audit018-<name>)

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Mods" tab
2. Click "Install mod", then select "From URL"
3. Enter a .jar URL and optional name
4. Click install

**Expected:** Mod is downloaded and installed. List updates.

**Cleanup:** Remove mod via remove button.

**Automatable?** yes (api-agent)

---

### server-detail-mods-browse-registry

**Preconditions:** Template declares a registry.

**Resources created:** none (browse only)

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Mods" tab
2. Click "Install mod" (opens on "Browse registry" by default when the template declares one)
3. Search/filter for a mod
4. View mod details

**Expected:** Registry browser opens with search and filters (CurseForge, Modrinth, etc.).

**Cleanup:** Close without installing.

**Automatable?** yes (api) but game-specific

---

### server-detail-mods-install-registry

**Preconditions:** Registry browser is open and mod is selected.

**Resources created:** one mod (audit018-<game>-<name>)

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Mods" tab
2. Open registry browser
3. Search and click "Install" on a mod

**Expected:** Mod is installed. List updates.

**Cleanup:** Remove mod.

**Automatable?** yes (api-agent)

---

### server-detail-mods-upload

**Preconditions:** Mods tab is visible.

**Resources created:** one mod (audit018-<purpose>)

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Mods" tab
2. Click "Install mod", then select "Upload file"
3. Select a .jar file
4. Upload

**Expected:** Mod is uploaded and installed.

**Cleanup:** Remove mod.

**Automatable?** yes (api-agent)

---

### server-detail-mods-check-updates

**Preconditions:** Installed mods exist.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Mods" tab
2. Click "Check updates"

**Expected:** Mods are checked against registry for newer versions. Results shown.

**Cleanup:** None needed.

**Automatable?** yes (api) but registry-dependent

---

### server-detail-mods-remove

**Preconditions:** Mod is installed.

**Resources created:** none (mod deleted)

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Mods" tab
2. Click remove icon on a mod
3. Confirm

**Expected:** Mod is deleted from the server.

**Cleanup:** None needed.

**Automatable?** yes (api-agent)

---

### server-detail-modpacks-browse

**Preconditions:** Server template supports modpacks.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name]
2. Click "Modpacks" tab (if visible)
3. Search/filter for a modpack

**Expected:** Modpack registry is displayed (Modrinth, Thunderstore, etc.). Search and category filters work.

**Cleanup:** Close without installing.

**Automatable?** yes (api)

---

### server-detail-modpacks-install

**Preconditions:** Modpack is found in registry.

**Resources created:** server state change (env variable or mod dependencies)

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Modpacks" tab
2. Find a modpack
3. Click "Install"

**Expected:** Modpack is installed (either env-based or deps-based). Server may restart.

**Cleanup:** Uninstall by setting env to empty or removing mods.

**Automatable?** yes (api-agent)

---

### server-detail-players-view

**Preconditions:** Server is Running and game supports player roster.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name]
2. Click "Players" tab

**Expected:** List of connected players, player count, ban/whitelist lists (if applicable).

**Cleanup:** None needed.

**Automatable?** yes (api-agent)

---

### server-detail-players-kick

**Preconditions:** Players are online and game supports kicking.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Players" tab
2. Locate a player
3. Click "Kick" button
4. Optionally enter a reason

**Expected:** Player is kicked from the server.

**Cleanup:** None needed (player is disconnected).

**Automatable?** yes (api-agent) with game-specific caveats

---

### server-detail-players-ban

**Preconditions:** Players are online and game supports banning.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Players" tab
2. Locate a player
3. Click "Ban" button
4. Optionally enter a reason

**Expected:** Player is banned and kicked.

**Cleanup:** Unban from the banned player list.

**Automatable?** yes (api-agent)

---

### server-detail-players-unban

**Preconditions:** Banned player list is visible.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Players" tab
2. Show "Banned" section
3. Click unban button on a banned player

**Expected:** Player is removed from ban list.

**Cleanup:** None needed.

**Automatable?** yes (api-agent)

---

### server-detail-players-whitelist-add

**Preconditions:** Whitelist feature is available.

**Resources created:** one whitelist entry (audit018-<player>)

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Players" tab
2. Show "Whitelist" section
3. Enter a player name
4. Click "Add"

**Expected:** Player is added to whitelist.

**Cleanup:** Remove via whitelist remove button.

**Automatable?** yes (api-agent)

---

### server-detail-players-whitelist-remove

**Preconditions:** Whitelist contains entries.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Players" tab
2. Show "Whitelist" section
3. Click remove button on a whitelist entry

**Expected:** Entry is removed.

**Cleanup:** None needed.

**Automatable?** yes (api-agent)

---

### server-detail-backups-create-now

**Preconditions:** Backup destination is configured (single).

**Resources created:** one backup (audit018-<server>-manual)

**Steps:**
1. Navigate to $GP/servers/[name]
2. Click "Backups" tab
3. Click "Back up now" button

**Expected:** Backup job starts. Tab shows "Running" status and progress.

**Cleanup:** Backup completes or fails naturally.

**Automatable?** yes (api)

---

### server-detail-backups-view-list

**Preconditions:** Backups exist for this server.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Backups" tab
2. Observe backup table

**Expected:** Table shows completed and in-progress backups with timestamps, sizes, and status.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### server-detail-backups-restore

**Preconditions:** Completed backup exists.

**Resources created:** none (restore operation)

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Backups" tab
2. Click restore button on a backup
3. Confirm in dialog

**Expected:** Restore job starts. Server transitions to Suspending → Resuming → Running.

**Cleanup:** Restore completes naturally.

**Automatable?** yes (api)

---

### server-detail-backups-schedule-create

**Preconditions:** Backup destination exists.

**Resources created:** one schedule (audit018-<server>-schedule)

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Backups" tab
2. Click "Create schedule" button
3. Enter cron or preset schedule
4. Click save

**Expected:** Schedule is created. Next run time is calculated and shown.

**Cleanup:** Delete schedule.

**Automatable?** yes (api)

---

### server-detail-backups-schedule-delete

**Preconditions:** Schedule exists.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Backups" tab
2. Click delete button on a schedule
3. Confirm

**Expected:** Schedule is deleted. Future backups won't run.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### server-detail-settings-general

**Preconditions:** Server exists.

**Resources created:** none (changes saved)

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Settings" tab
2. Click "General" section (default)
3. Edit description, image override, or add labels
4. Click "Save"

**Expected:** Changes are saved. Conflict detection works if server was modified elsewhere.

**Cleanup:** None needed (changes are persisted).

**Automatable?** yes (api)

---

### server-detail-settings-version

**Preconditions:** Template has multiple versions.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Settings" tab
2. Click "Version" section
3. Select a new version from dropdown
4. Click "Save"

**Expected:** Server's active version is updated. Server may restart.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### server-detail-settings-resources

**Preconditions:** Server exists.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Settings" tab
2. Click "Resources" section
3. Edit CPU, memory, or storage limits
4. Click "Save"

**Expected:** Limits are updated. Pod is rescheduled if needed.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### server-detail-settings-networking

**Preconditions:** Server exists.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Settings" tab
2. Click "Networking" section
3. Edit exposure type (NodePort/LoadBalancer/ClusterIP), hostname, source ranges, or port overrides
4. Click "Save"

**Expected:** Network configuration is updated. Service is recreated if needed.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### server-detail-settings-environment

**Preconditions:** Server exists.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Settings" tab
2. Click "Environment" section
3. Add, edit, or remove environment variables
4. Click "Save"

**Expected:** Env vars are persisted. Server may need restart for changes to take effect.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### server-detail-settings-lifecycle

**Preconditions:** Server exists.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Settings" tab
2. Click "Lifecycle" section
3. Configure auto-restart, update policy, or idle sleep settings
4. Click "Save"

**Expected:** Lifecycle policies are updated.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### server-detail-settings-scheduled-backups

**Preconditions:** Backup destination exists.

**Resources created:** one schedule (audit018-<server>-schedule)

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Settings" tab
2. Click "Scheduled backups" section
3. Create, edit, or delete a backup schedule
4. Click "Save"

**Expected:** Schedule is created or updated.

**Cleanup:** Delete schedule.

**Automatable?** yes (api)

---

### server-detail-settings-network-capture

**Preconditions:** Server exists (capture requires running pod).

**Resources created:** none (capture is ephemeral)

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Settings" tab
2. Click "Network capture" section
3. Enable capture and configure BPF filter (optional)
4. Click "Save"

**Expected:** Capture pod is deployed. Packet capture runs for configured duration.

**Cleanup:** Capture pod is cleaned up after duration expires.

**Automatable?** yes (api)

---

### server-detail-settings-placement

**Preconditions:** Server exists.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Settings" tab
2. Click "Placement" section
3. Configure node affinity or anti-affinity
4. Click "Save"

**Expected:** Pod scheduling constraints are updated.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### server-detail-settings-access

**Preconditions:** User has servers:write or higher.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Settings" tab
2. Click "RBAC & access" section
3. Add or modify role bindings (e.g., operator role for a user)
4. Click "Save"

**Expected:** Role bindings are updated. Access is granted immediately.

**Cleanup:** Remove role binding.

**Automatable?** yes (api)

---

### server-detail-settings-sharelinks-create

**Preconditions:** Server exists.

**Resources created:** one share link (audit018-<server>-share)

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Settings" tab
2. Click "Share links" section
3. Click "Create share link"
4. Choose expiry (preset or custom date)
5. Click "Create"

**Expected:** Share link is generated with token. Link is displayable and copyable.

**Cleanup:** Revoke link.

**Automatable?** yes (api)

---

### server-detail-settings-sharelinks-view

**Preconditions:** Share links exist.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Settings" tab
2. Click "Share links" section
3. Observe table of active links

**Expected:** Table shows token (masked), expiry date, and access level.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### server-detail-settings-sharelinks-revoke

**Preconditions:** Share link exists.

**Resources created:** none (link is deleted)

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Settings" tab
2. Click "Share links" section
3. Click revoke button on a link
4. Confirm

**Expected:** Link is deleted. Token is no longer valid.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### server-detail-settings-danger-delete

**Preconditions:** User has servers:write or higher.

**Resources created:** none (server deleted)

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Settings" tab
2. Click "Danger zone" section
3. Click "Delete server"
4. Confirm by typing server name
5. Confirm final dialog

**Expected:** GameServer CR is deleted. Pod and PVC are cleaned up. Redirect to /servers.

**Cleanup:** None needed (server is deleted).

**Automatable?** yes (api)

---

### server-detail-settings-danger-transfer

**Preconditions:** User has servers:write.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/[name] and click the "Settings" tab
2. Click "Danger zone" section
3. Click "Transfer ownership"
4. Select new owner from dropdown
5. Confirm

**Expected:** Server ownership is transferred. New owner can manage it.

**Cleanup:** Transfer back.

**Automatable?** yes (api)

---

### modules-catalog-browse

**Preconditions:** User is authenticated.

**Resources created:** none

**Steps:**
1. Navigate to $GP/modules
2. Observe the module catalog grid/list

**Expected:** All available modules are displayed with install/upgrade/uninstall buttons.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### modules-search

**Preconditions:** Multiple modules exist.

**Resources created:** none

**Steps:**
1. Navigate to $GP/modules
2. Type in the search input

**Expected:** Catalog filters by module name.

**Cleanup:** Clear search.

**Automatable?** yes (api)

---

### modules-filter-by-source

**Preconditions:** Multiple module sources exist.

**Resources created:** none

**Steps:**
1. Navigate to $GP/modules
2. Click "Source" dropdown
3. Select a source

**Expected:** Catalog filters to modules from that source.

**Cleanup:** Select "All sources".

**Automatable?** yes (api)

---

### modules-filter-by-category

**Preconditions:** Modules have categories.

**Resources created:** none

**Steps:**
1. Navigate to $GP/modules
2. Click category chips to toggle them

**Expected:** Catalog filters by selected categories.

**Cleanup:** Click "All".

**Automatable?** yes (api)

---

### modules-install

**Preconditions:** Module is not installed.

**Resources created:** one Module CR (audit018-<game>-module)

**Steps:**
1. Navigate to $GP/modules
2. Find a module
3. Click "Install"
4. Select version (if multiple)
5. Confirm

**Expected:** Module CR is created. Operator pulls and materializes the template. Status transitions from Pending → Running.

**Cleanup:** Uninstall module.

**Automatable?** yes (api)

---

### modules-upgrade

**Preconditions:** Module is installed and newer version available.

**Resources created:** none (in-place upgrade)

**Steps:**
1. Navigate to $GP/modules
2. Find an installed module showing the "Upgrade" button
3. Click "Upgrade"
4. Confirm

**Expected:** Module version is updated. Template is re-materialized.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### modules-uninstall

**Preconditions:** Module is installed.

**Resources created:** none (module deleted)

**Steps:**
1. Navigate to $GP/modules
2. Find an installed module
3. Click "Uninstall"
4. Confirm

**Expected:** Module CR is deleted. Template is removed.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### modules-upload-custom

**Preconditions:** User has config:manage (admin). Upload source exists.

**Resources created:** one Module CR (audit018-custom-<name>)

**Steps:**
1. Navigate to $GP/modules
2. Click "Upload module" button
3. Select a local OCI bundle file or image archive
4. Click upload

**Expected:** Module is uploaded to the uploads source. CR is created.

**Cleanup:** Uninstall module.

**Automatable?** yes (api)

---

### modules-build-custom

**Preconditions:** User has config:manage. Build tool is available.

**Resources created:** one Module CR (audit018-custom-<name>)

**Steps:**
1. Navigate to $GP/modules
2. Click "Build module" button
3. Fill in module metadata (name, description, icon, etc.)
4. Submit

**Expected:** Build dialog walks user through creating a module. Module is created and installed.

**Cleanup:** Uninstall.

**Automatable?** no (manual interactive builder)

---

### cluster-view-nodes

**Preconditions:** User has servers:write. Cluster info is available.

**Resources created:** none

**Steps:**
1. Navigate to $GP/cluster
2. Observe the node list

**Expected:** Each node shows name, status (Ready/NotReady), CPU/memory usage, pod count.

**Cleanup:** None needed.

**Automatable?** yes (operator)

---

### cluster-download-kubeconfig

**Preconditions:** User has servers:write. Cluster ops are enabled.

**Resources created:** none

**Steps:**
1. Navigate to $GP/cluster
2. Click "Download kubeconfig" button

**Expected:** gameplane-kubeconfig.yaml is downloaded. Contains cluster API credentials.

**Cleanup:** None needed (file is on client).

**Automatable?** no (download to client)

---

### cluster-add-node

**Preconditions:** User has servers:write. Cluster ops are enabled.

**Resources created:** one node (to be joined manually)

**Steps:**
1. Navigate to $GP/cluster
2. Click "Add node" button
3. Follow join instructions (copies join command)

**Expected:** Node join info is displayed (command to run on new node).

**Cleanup:** Node must be joined manually; no cleanup needed in UI.

**Automatable?** no (requires external node setup)

---

### users-list-view

**Preconditions:** User has users:manage.

**Resources created:** none

**Steps:**
1. Navigate to $GP/users
2. Click "Users" tab (default)

**Expected:** Table shows all users with email, roles, and action buttons (edit, reset password, delete).

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### users-invite

**Preconditions:** User has users:manage.

**Resources created:** one user (audit018-<email>)

**Steps:**
1. Navigate to $GP/users
2. Click "Invite user" button
3. Enter email address
4. Select role
5. Click invite

**Expected:** User is created with temporary credentials. Email is sent (or link shown).

**Cleanup:** Delete user.

**Automatable?** yes (api)

---

### users-edit-role

**Preconditions:** User exists and has users:manage.

**Resources created:** none (role updated)

**Steps:**
1. Navigate to $GP/users
2. Click edit icon on a user
3. Change role dropdown
4. Click save

**Expected:** User's role is updated. Permissions change immediately.

**Cleanup:** Revert role.

**Automatable?** yes (api)

---

### users-reset-password

**Preconditions:** User exists and has users:manage.

**Resources created:** none

**Steps:**
1. Navigate to $GP/users
2. Click reset password icon on a user
3. Confirm

**Expected:** User's password is reset. New temporary password is shown (or link displayed).

**Cleanup:** None needed (user can set new password).

**Automatable?** yes (api)

---

### users-delete

**Preconditions:** User exists and has users:manage.

**Resources created:** none (user deleted)

**Steps:**
1. Navigate to $GP/users
2. Click delete icon on a user
3. Confirm

**Expected:** User is deleted. Sessions are invalidated.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### users-manage-roles

**Preconditions:** User has users:manage.

**Resources created:** none (role created or modified)

**Steps:**
1. Navigate to $GP/users
2. Click "Roles" tab
3. Click "Create role" or edit existing role
4. Assign permissions
5. Click save

**Expected:** Role is created or updated. Users can be assigned to it.

**Cleanup:** Delete role (if unused).

**Automatable?** yes (api)

---

### users-manage-service-accounts

**Preconditions:** User has users:manage.

**Resources created:** one service account (audit018-<purpose>)

**Steps:**
1. Navigate to $GP/users
2. Click "Service accounts" tab
3. Click "Create service account"
4. Enter name and scope
5. Click create

**Expected:** Service account is created. API token is displayed (one-time).

**Cleanup:** Delete service account.

**Automatable?** yes (api)

---

### users-manage-oidc-providers

**Preconditions:** User has users:manage and config:manage.

**Resources created:** none

**Steps:**
1. Navigate to $GP/users
2. Click "OIDC providers" tab (if available)
3. Add or edit OIDC provider configuration
4. Click save

**Expected:** OIDC configuration is persisted. Login options may update.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### admin-settings-general

**Preconditions:** User has config:manage.

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin
2. Click "General" section (default)
3. Edit cluster name, description, or other general settings
4. Click "Save"

**Expected:** Settings are persisted.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### admin-settings-auth

**Preconditions:** User has config:manage.

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin
2. Click "Authentication" section
3. Configure local auth, SSO providers, or OIDC
4. Click "Save"

**Expected:** Auth configuration is updated.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### admin-settings-backup-destinations

**Preconditions:** User has config:manage.

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin
2. Click "Backup destinations" section
3. Add, edit, or remove backup destination (S3, local, etc.)
4. Click "Save"

**Expected:** Backup destination is configured. Backups can use it.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### admin-settings-module-sources

**Preconditions:** User has config:manage.

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin
2. Click "Module sources" section
3. Add, edit, or remove module source
4. Click "Save"

**Expected:** Module source is configured. Modules from that source appear in catalog.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### admin-settings-mod-registries

**Preconditions:** User has config:manage.

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin
2. Click "Mod registries" section
3. Add or edit a mod registry (CurseForge, Modrinth, etc.)
4. Click "Save"

**Expected:** Registry is configured. Games can browse mods from it.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### admin-settings-notifications

**Preconditions:** User has config:manage.

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin
2. Click "Notifications" section
3. Configure notification sinks (Slack, email, webhook, etc.)
4. Click "Save"

**Expected:** Notification config is saved. Events are sent to configured endpoints.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### admin-settings-telemetry

**Preconditions:** User has config:manage.

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin
2. Click "Telemetry" section
3. Enable/disable telemetry collection
4. Click "Save"

**Expected:** Telemetry setting is persisted.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### admin-settings-updates

**Preconditions:** User has config:manage.

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin
2. Click "Updates" section
3. View update availability and status

**Expected:** Current version and available updates are shown.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### admin-settings-about

**Preconditions:** User has config:manage (or any authenticated user for read-only view).

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin
2. Click "About" section

**Expected:** Version, build info, license, and links are displayed.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### theme-settings-preset-selection

**Preconditions:** User is authenticated.

**Resources created:** none

**Steps:**
1. Navigate to $GP/settings/theme
2. Click a preset theme option (Modern Pink, Legacy Orange)

**Expected:** Theme preset is applied live to the dashboard. Setting is saved.

**Cleanup:** Switch back to default.

**Automatable?** no (visual preference, no behavioral change)

---

### theme-settings-appearance-mode

**Preconditions:** User is authenticated.

**Resources created:** none

**Steps:**
1. Navigate to $GP/settings/theme
2. Select appearance mode (Light, Dark, System)

**Expected:** UI switches to selected mode. Preference is saved.

**Cleanup:** Reset to System.

**Automatable?** no (visual preference)

---

### theme-settings-custom-colors

**Preconditions:** User is authenticated.

**Resources created:** none

**Steps:**
1. Navigate to $GP/settings/theme
2. Click "Custom colors" section
3. Select custom accent and surface colors from swatches
4. Click "Save"

**Expected:** Custom colors are applied and saved to localStorage/preferences.

**Cleanup:** Click "Reset to Defaults".

**Automatable?** no (visual preference)

---

### theme-settings-custom-css

**Preconditions:** User is authenticated.

**Resources created:** none

**Steps:**
1. Navigate to $GP/settings/theme
2. Click "Custom CSS" section
3. Enter CSS rules in the editor
4. Click "Save"

**Expected:** Custom CSS is injected into the page. Preference is saved (max 64 KB).

**Cleanup:** Clear editor and save.

**Automatable?** no (visual preference)

---

### theme-settings-export

**Preconditions:** User has custom theme settings.

**Resources created:** none

**Steps:**
1. Navigate to $GP/settings/theme
2. Click "Download gameplane-theme.json" button

**Expected:** Theme config is downloaded as a JSON file.

**Cleanup:** None needed.

**Automatable?** no (download to client)

---

### theme-settings-import

**Preconditions:** User has a theme export file.

**Resources created:** none

**Steps:**
1. Navigate to $GP/settings/theme
2. Click "Choose file…" button
3. Select exported JSON file

**Expected:** Theme settings are loaded from file and applied.

**Cleanup:** Import a different theme.

**Automatable?** yes (api)

---

### backups-list-view

**Preconditions:** Backups exist.

**Resources created:** none

**Steps:**
1. Navigate to $GP/backups
2. Click "Backups" tab (default)
3. Observe backup list

**Expected:** Table shows all backups with server, phase, size, timestamp, and actions.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### backups-filter-by-server

**Preconditions:** Multiple servers have backups.

**Resources created:** none

**Steps:**
1. Navigate to $GP/backups
2. Use server filter dropdown
3. Select a server

**Expected:** List shows only backups for that server.

**Cleanup:** Select "All servers".

**Automatable?** yes (api)

---

### backups-filter-by-phase

**Preconditions:** Backups in different states exist.

**Resources created:** none

**Steps:**
1. Navigate to $GP/backups
2. Use phase filter dropdown
3. Select phases (Pending, Running, Succeeded, Failed)

**Expected:** List filters by phase.

**Cleanup:** Select "All".

**Automatable?** yes (api)

---

### backups-backup-now

**Preconditions:** Backup destination is configured.

**Resources created:** one backup (audit018-<server>-manual)

**Steps:**
1. Navigate to $GP/backups
2. Click "Back up now" button
3. Select server and destination
4. Click start

**Expected:** Backup job starts. List updates.

**Cleanup:** Backup completes.

**Automatable?** yes (api)

---

### backups-restore

**Preconditions:** Completed backup exists.

**Resources created:** none (restore operation)

**Steps:**
1. Navigate to $GP/backups
2. Find a backup
3. Click restore button
4. Confirm

**Expected:** Restore job starts. Server state transitions.

**Cleanup:** Restore completes.

**Automatable?** yes (api)

---

### backups-view-detail

**Preconditions:** Backup exists.

**Resources created:** none

**Steps:**
1. Navigate to $GP/backups
2. Click a backup row

**Expected:** Detail drawer opens showing backup info (server, size, duration, repo, etc.).

**Cleanup:** Close drawer.

**Automatable?** yes (api)

---

### backups-schedules-create

**Preconditions:** Backup destination exists.

**Resources created:** one schedule (audit018-<server>-schedule)

**Steps:**
1. Navigate to $GP/backups
2. Click "Schedules" tab
3. Click "Create schedule"
4. Configure cron/preset schedule
5. Click save

**Expected:** Schedule is created.

**Cleanup:** Delete schedule.

**Automatable?** yes (api)

---

### backups-schedules-edit

**Preconditions:** Schedule exists.

**Resources created:** none

**Steps:**
1. Navigate to $GP/backups
2. Click "Schedules" tab
3. Click edit on a schedule
4. Modify settings
5. Click save

**Expected:** Schedule is updated.

**Cleanup:** Revert to original.

**Automatable?** yes (api)

---

### backups-schedules-toggle-suspend

**Preconditions:** Schedule exists.

**Resources created:** none

**Steps:**
1. Navigate to $GP/backups
2. Click "Schedules" tab
3. Toggle the "Suspend" switch on a schedule

**Expected:** Schedule is suspended (no future runs) or resumed.

**Cleanup:** Toggle back.

**Automatable?** yes (api)

---

### backups-schedules-delete

**Preconditions:** Schedule exists.

**Resources created:** none

**Steps:**
1. Navigate to $GP/backups
2. Click "Schedules" tab
3. Click delete button on a schedule
4. Confirm

**Expected:** Schedule is deleted.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### backups-restores-view

**Preconditions:** Restore jobs have run.

**Resources created:** none

**Steps:**
1. Navigate to $GP/backups
2. Click "Restores" tab

**Expected:** Table shows all restore jobs with server, backup source, phase, and duration.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### audit-log-view

**Preconditions:** User has audit:read.

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin/audit
2. Observe the audit event table

**Expected:** Events are listed newest-first. Each row shows timestamp, actor, method, path, status, target.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### audit-log-filter-by-status-class

**Preconditions:** Events with different HTTP status classes exist.

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin/audit
2. Click status filter tabs (2xx, 4xx, 5xx)

**Expected:** List filters to events with matching status codes.

**Cleanup:** Click "All".

**Automatable?** yes (api)

---

### audit-log-filter-by-method

**Preconditions:** Events with different HTTP methods exist.

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin/audit
2. Click method filter (GET, POST, PUT, PATCH, DELETE)

**Expected:** List filters by method.

**Cleanup:** Select "All".

**Automatable?** yes (api)

---

### audit-log-filter-by-actor

**Preconditions:** Events from different actors exist.

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin/audit
2. Type in the actor search field

**Expected:** List filters to events where actor matches (case-insensitive substring).

**Cleanup:** Clear input.

**Automatable?** yes (api)

---

### audit-log-pagination

**Preconditions:** More than 100 audit events exist.

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin/audit
2. Scroll to bottom
3. Click "Load more" button

**Expected:** Next page of events is loaded and appended.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### audit-log-export-csv

**Preconditions:** Audit events exist and filters are optionally applied.

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin/audit
2. Apply any filters (optional)
3. Click "Export CSV" button

**Expected:** CSV file is downloaded with filtered audit events.

**Cleanup:** None needed.

**Automatable?** no (download to client)

---

### audit-log-verify-integrity

**Preconditions:** User has audit:read.

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin/audit
2. Observe integrity banner (if verification data exists)

**Expected:** Banner shows integrity status (verified, unverified, or warning) with details.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### admin-logs-view-api

**Preconditions:** User has * permission (admin).

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin/logs
2. Component dropdown shows "API server"
3. Observe log stream

**Expected:** API server logs are streamed (pod is identified in header).

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### admin-logs-view-operator

**Preconditions:** User has * permission (admin).

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin/logs
2. Change component dropdown to "Operator"
3. Observe log stream

**Expected:** Operator logs are displayed.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### admin-logs-tail-option

**Preconditions:** Admin logs page is open.

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin/logs
2. Select a tail size from dropdown (100, 500, 1000, 5000)

**Expected:** Log stream starts with selected number of tail lines.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### admin-logs-follow

**Preconditions:** Admin logs page is open.

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin/logs
2. Toggle "Follow" switch on

**Expected:** Log stream continues to update in real-time as new lines arrive. Auto-reconnects on disconnect.

**Cleanup:** Toggle "Follow" off.

**Automatable?** yes (api)

---

### admin-logs-download

**Preconditions:** Log stream has data.

**Resources created:** none

**Steps:**
1. Navigate to $GP/admin/logs
2. Click "Download" button

**Expected:** Current log buffer is downloaded as a .txt file.

**Cleanup:** None needed.

**Automatable?** no (download to client)

---

### share-access-link-view

**Preconditions:** User has a valid share token from a server owner.

**Resources created:** none

**Steps:**
1. Navigate to $GP/share/[token]
2. Observe the share page

**Expected:** Server info is displayed (name, status, address). Actions available depend on server state.

**Cleanup:** None needed.

**Automatable?** no (token-based access, no auth required)

---

### share-server-start

**Preconditions:** Server is Stopped/Suspended and token is valid.

**Resources created:** none

**Steps:**
1. Navigate to $GP/share/[token]
2. Click "Start server" button

**Expected:** Server transitions to Starting → Running. Page updates to show address.

**Cleanup:** Token owner can stop server.

**Automatable?** yes (api)

---

### share-server-view-status

**Preconditions:** Token is valid and points to a running server.

**Resources created:** none

**Steps:**
1. Navigate to $GP/share/[token]
2. Observe the server status

**Expected:** Server address is displayed. Status shows current phase.

**Cleanup:** None needed.

**Automatable?** yes (api)

---

### share-link-copy-address

**Preconditions:** Server is Running and address is visible.

**Resources created:** none

**Steps:**
1. Navigate to $GP/share/[token]
2. Click copy icon next to server address

**Expected:** Address is copied to clipboard. Feedback is shown (checkmark appears).

**Cleanup:** None needed.

**Automatable?** no (clipboard operation)

---

### share-theme-preference

**Preconditions:** Share link is accessed.

**Resources created:** none

**Steps:**
1. Navigate to $GP/share/[token]
2. Appearance mode selector may be visible
3. Change light/dark mode if available

**Expected:** Theme persists in localStorage for this share page.

**Cleanup:** None needed.

**Automatable?** no (localStorage preference)

---

### create-server-wizard-pick-template

**Preconditions:** User is authenticated. At least one GameTemplate exists.

**Resources created:** none

**Steps:**
1. Navigate to $GP/servers/new
2. On the "Template" step, search/filter the template list
3. Select a template card

**Expected:** The template is selected, its game icon/name is shown, and the wizard advances (or enables "Next") to the version step (if the template has versions) or configure step.

**Cleanup:** Cancel the wizard without creating a server.

**Automatable?** yes; bucket: `api-agent`.

---

### create-server-wizard-pick-version

**Preconditions:** Selected template (from `create-server-wizard-pick-template`) declares one or more `spec.versions` entries.

**Resources created:** none

**Steps:**
1. From the "Template" step, proceed to the "Version" step
2. Select a version from the list

**Expected:** The version step only appears for templates with declared versions; selecting a version carries it forward to the review step.

**Cleanup:** Cancel the wizard without creating a server.

**Automatable?** no (conditional step; covered end-to-end by `create-server-wizard-review-create`)

---

### create-server-wizard-configure

**Preconditions:** Template selected (and version, if applicable).

**Resources created:** none

**Steps:**
1. On the "Configure" step, enter a server name
2. Adjust CPU/memory/storage resource fields
3. Add or edit labels if the field is present

**Expected:** Name validation rejects names that collide with an existing server or fail Kubernetes naming rules; resource fields default from the template and accept overrides within node capacity.

**Cleanup:** Cancel the wizard without creating a server.

**Automatable?** yes; bucket: `api-agent`.

---

### create-server-wizard-network

**Preconditions:** Template selected; on the "Network" step.

**Resources created:** none

**Steps:**
1. Choose exposure mode (e.g., ClusterIP/NodePort/tunnel) if offered by the template
2. Set port overrides and/or tunnel port mappings
3. Enter allowed source ranges if the field is present

**Expected:** Network fields validate (e.g., malformed CIDRs in source ranges are rejected) and carry forward to the review step summary.

**Cleanup:** Cancel the wizard without creating a server.

**Automatable?** yes; bucket: `api-agent`.

---

### create-server-wizard-review-create

**Preconditions:** All prior wizard steps completed.

**Resources created:** GameServer named per the wizard's name field (use `audit018-` prefix).

**Steps:**
1. On the "Review" step, confirm the summarized template/version/configuration/network values
2. Click "Create"
   - Login cost: 1 (reuse admin/operator session)
3. Verify the API call: `kubectl get gameserver <name> -o yaml`

**Expected:** POST to `/servers` succeeds and the dashboard navigates to the new server's detail page; a name collision or RBAC denial surfaces the error inline (see `errorMessage` in CreateServer.tsx) without leaving the wizard.

**Cleanup:** `kubectl delete gameserver <name> -n gameplane-games`.

**Automatable?** yes; bucket: `api-agent`.

---
