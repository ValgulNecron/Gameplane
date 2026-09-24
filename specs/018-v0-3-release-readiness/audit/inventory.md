# Feature Inventory

Enumerated from code (research R2). Columns and allowed values are fixed by [contracts/audit-records.md](../contracts/audit-records.md#inventorymd). Procedures are in [procedures/](procedures/) and evidence in [evidence/](evidence/). Allowed `Outcome` values: `untested`, `pass`, `fail`, `blocked`, `n/a`.

## WEB
| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|
| INV-WEB-001 | Sign in with username and password | web/ | web/src/routes/Login.tsx:69-92 | [p](procedures/web.md#login-username-password) | TestAPI_BootstrapAndLogin | untested |  | [e](evidence/INV-WEB-001/) |  |  |
| INV-WEB-002 | Toggle password visibility on login form | web/ | web/src/routes/Login.tsx:172-186 | [p](procedures/web.md#login-toggle-password-visibility) | none | untested |  | [e](evidence/INV-WEB-002/) |  |  |
| INV-WEB-003 | Sign in with SSO provider | web/ | web/src/routes/Login.tsx:295-312 | [p](procedures/web.md#login-sso-provider) | TestAPI_DynamicAuthProviders, TestAPI_OIDCHelmSeeded_AdminOnFirstLogin | untested |  | [e](evidence/INV-WEB-003/) |  |  |
| INV-WEB-004 | Sign in with safe mode (custom CSS disabled) | web/ | web/src/routes/Login.tsx:69-86 | [p](procedures/web.md#login-safe-mode) | none | untested |  | [e](evidence/INV-WEB-004/) |  |  |
| INV-WEB-005 | Display password reset hint on login | web/ | web/src/routes/Login.tsx:151-159 | [p](procedures/web.md#login-password-forgotten-hint) | TestAPI_PasswordResetInvalidatesSession | untested |  | [e](evidence/INV-WEB-005/) |  |  |
| INV-WEB-006 | View fleet status card (running, stopped, failed servers) | web/ | web/src/routes/Dashboard.tsx:196-249 | [p](procedures/web.md#dashboard-view-fleet-summary) | none | untested |  | [e](evidence/INV-WEB-006/) |  |  |
| INV-WEB-007 | View cluster resources metrics (CPU, memory, storage, nodes) | web/ | web/src/routes/Dashboard.tsx:296-350 | [p](procedures/web.md#dashboard-view-cluster-resources) | none | untested |  | [e](evidence/INV-WEB-007/) |  |  |
| INV-WEB-008 | View recent activity (audit events) | web/ | web/src/routes/Dashboard.tsx:372-410 | [p](procedures/web.md#dashboard-view-recent-activity) | TestAPI_AuditEmitsOnMutation | untested |  | [e](evidence/INV-WEB-008/) |  |  |
| INV-WEB-009 | View recent backups | web/ | web/src/routes/Dashboard.tsx:412-477 | [p](procedures/web.md#dashboard-view-recent-backups) | TestBackup_OperatorMaterializesJob | untested |  | [e](evidence/INV-WEB-009/) |  |  |
| INV-WEB-010 | Create server from dashboard button | web/ | web/src/routes/Dashboard.tsx:113-121 | [p](procedures/web.md#dashboard-create-server-button) | TestAPI_LifecycleClone | untested |  | [e](evidence/INV-WEB-010/) |  |  |
| INV-WEB-011 | List all servers | web/ | web/src/routes/Servers.tsx:38-174 | [p](procedures/web.md#servers-list-view) | none | untested |  | [e](evidence/INV-WEB-011/) |  |  |
| INV-WEB-012 | Filter servers by status (all, running, stopped) | web/ | web/src/routes/Servers.tsx:230-262 | [p](procedures/web.md#servers-filter-by-status) | none | untested |  | [e](evidence/INV-WEB-012/) |  |  |
| INV-WEB-013 | Search servers by name | web/ | web/src/routes/Servers.tsx:263-296 | [p](procedures/web.md#servers-search-by-name) | none | untested |  | [e](evidence/INV-WEB-013/) |  |  |
| INV-WEB-014 | Filter servers by game type | web/ | web/src/routes/Servers.tsx:274-295 | [p](procedures/web.md#servers-filter-by-game) | none | untested |  | [e](evidence/INV-WEB-014/) |  |  |
| INV-WEB-015 | Filter servers by namespace | web/ | web/src/routes/Servers.tsx:274-295 | [p](procedures/web.md#servers-filter-by-namespace) | none | untested |  | [e](evidence/INV-WEB-015/) |  |  |
| INV-WEB-016 | Open server detail page | web/ | web/src/routes/Servers.tsx:405-411 | [p](procedures/web.md#servers-open-detail) | none | untested |  | [e](evidence/INV-WEB-016/) |  |  |
| INV-WEB-017 | Start stopped server from list | web/ | web/src/routes/Servers.tsx:626-633 | [p](procedures/web.md#servers-start-server) | TestAPI_LifecycleStartStop, TestGameServer_HeartbeatReachesRunning | untested |  | [e](evidence/INV-WEB-017/) |  |  |
| INV-WEB-018 | Stop running server from list | web/ | web/src/routes/Servers.tsx:634-644 | [p](procedures/web.md#servers-stop-server) | TestAPI_LifecycleStartStop | untested |  | [e](evidence/INV-WEB-018/) |  |  |
| INV-WEB-019 | Restart running server from list | web/ | web/src/routes/Servers.tsx:645-650 | [p](procedures/web.md#servers-restart-server) | TestAPI_LifecycleRestart | untested |  | [e](evidence/INV-WEB-019/) |  |  |
| INV-WEB-020 | Wake sleeping server from list | web/ | web/src/routes/Servers.tsx:617-625 | [p](procedures/web.md#servers-wake-sleeping-server) | TestGameServer_WakeOnConnect_LoginWakes | untested |  | [e](evidence/INV-WEB-020/) |  |  |
| INV-WEB-021 | Access server actions menu (edit, transfer, delete) | web/ | web/src/routes/Servers.tsx:651 | [p](procedures/web.md#servers-menu-actions) | none | untested |  | [e](evidence/INV-WEB-021/) |  |  |
| INV-WEB-022 | View server overview (metrics, players, endpoints, events) | web/ | web/src/routes/tabs/Overview.tsx:15-200 | [p](procedures/web.md#server-detail-overview-tab) | none | untested |  | [e](evidence/INV-WEB-022/) |  |  |
| INV-WEB-023 | Start server from detail page | web/ | web/src/routes/ServerDetail.tsx:227-235 | [p](procedures/web.md#server-detail-lifecycle-start) | TestAPI_LifecycleStartStop | untested |  | [e](evidence/INV-WEB-023/) |  |  |
| INV-WEB-024 | Stop server from detail page | web/ | web/src/routes/ServerDetail.tsx:209-216 | [p](procedures/web.md#server-detail-lifecycle-stop) | TestAPI_LifecycleStartStop | untested |  | [e](evidence/INV-WEB-024/) |  |  |
| INV-WEB-025 | Restart server from detail page | web/ | web/src/routes/ServerDetail.tsx:201-208 | [p](procedures/web.md#server-detail-lifecycle-restart) | TestAPI_LifecycleStartStop | untested |  | [e](evidence/INV-WEB-025/) |  |  |
| INV-WEB-026 | Wake sleeping server from detail page | web/ | web/src/routes/ServerDetail.tsx:217-226 | [p](procedures/web.md#server-detail-lifecycle-wake) | TestGameServer_WakeOnConnect_LoginWakes | untested |  | [e](evidence/INV-WEB-026/) |  |  |
| INV-WEB-027 | Open console tab from detail page | web/ | web/src/routes/ServerDetail.tsx:237-241 | [p](procedures/web.md#server-detail-open-console) | TestAPI_ConsolePTYRoundTrip | untested |  | [e](evidence/INV-WEB-027/) |  |  |
| INV-WEB-028 | Send command in console (PTY/RCON) | web/ | web/src/routes/tabs/Console.tsx:48-100 | [p](procedures/web.md#server-detail-console-send-command) | TestAPI_ConsolePTYRoundTrip, TestGameServer_Squad_RCON | untested |  | [e](evidence/INV-WEB-028/) |  |  |
| INV-WEB-029 | View pod container logs | web/ | web/src/routes/tabs/Logs.tsx:73-100 | [p](procedures/web.md#server-detail-logs-view-pod-logs) | TestAPI_LogsTailWS | untested |  | [e](evidence/INV-WEB-029/) |  |  |
| INV-WEB-030 | View game log file | web/ | web/src/routes/tabs/Logs.tsx:71 | [p](procedures/web.md#server-detail-logs-view-game-logs) | TestAPI_LogsTailWS | untested |  | [e](evidence/INV-WEB-030/) |  |  |
| INV-WEB-031 | Filter logs by level (INFO, WARN, ERROR, DEBUG) | web/ | web/src/routes/tabs/Logs.tsx:56-58 | [p](procedures/web.md#server-detail-logs-filter-by-level) | none | untested |  | [e](evidence/INV-WEB-031/) |  |  |
| INV-WEB-032 | Download logs | web/ | web/src/routes/tabs/Logs.tsx:56-100 | [p](procedures/web.md#server-detail-logs-download) | none | untested |  | [e](evidence/INV-WEB-032/) |  |  |
| INV-WEB-033 | Browse server files | web/ | web/src/routes/tabs/Files.tsx:42-100 | [p](procedures/web.md#server-detail-files-browse) | TestAPI_AgentFilesRoundTrip | untested |  | [e](evidence/INV-WEB-033/) |  |  |
| INV-WEB-034 | View file content in editor | web/ | web/src/routes/tabs/Files.tsx:73-95 | [p](procedures/web.md#server-detail-files-view-file) | TestAPI_AgentFilesRoundTrip | untested |  | [e](evidence/INV-WEB-034/) |  |  |
| INV-WEB-035 | Edit and save file | web/ | web/src/routes/tabs/Files.tsx:42-100 | [p](procedures/web.md#server-detail-files-edit-file) | TestAPI_AgentFilesRoundTrip | untested |  | [e](evidence/INV-WEB-035/) |  |  |
| INV-WEB-036 | Upload file to server | web/ | web/src/routes/tabs/Files.tsx:62 | [p](procedures/web.md#server-detail-files-upload-file) | TestAPI_AgentFilesRoundTrip | untested |  | [e](evidence/INV-WEB-036/) |  |  |
| INV-WEB-037 | Create new file | web/ | web/src/routes/tabs/Files.tsx:59-60 | [p](procedures/web.md#server-detail-files-create-file) | TestAPI_AgentFilesRoundTrip | untested |  | [e](evidence/INV-WEB-037/) |  |  |
| INV-WEB-038 | Create new directory | web/ | web/src/routes/tabs/Files.tsx:59 | [p](procedures/web.md#server-detail-files-create-directory) | TestAPI_AgentFilesRoundTrip | untested |  | [e](evidence/INV-WEB-038/) |  |  |
| INV-WEB-039 | Delete file or directory | web/ | web/src/routes/tabs/Files.tsx:58 | [p](procedures/web.md#server-detail-files-delete-file) | TestAPI_AgentFilesRoundTrip | untested |  | [e](evidence/INV-WEB-039/) |  |  |
| INV-WEB-040 | Download file from server | web/ | web/src/routes/tabs/Files.tsx:42-100 | [p](procedures/web.md#server-detail-files-download-file) | TestAPI_AgentFilesRoundTrip | untested |  | [e](evidence/INV-WEB-040/) |  |  |
| INV-WEB-041 | View Kubernetes events | web/ | web/src/routes/tabs/Events.tsx:11-40 | [p](procedures/web.md#server-detail-events-view) | none | untested |  | [e](evidence/INV-WEB-041/) |  |  |
| INV-WEB-042 | Filter events by type (all, info, warnings) | web/ | web/src/routes/tabs/Events.tsx:34-39 | [p](procedures/web.md#server-detail-events-filter) | none | untested |  | [e](evidence/INV-WEB-042/) |  |  |
| INV-WEB-043 | List installed mods | web/ | web/src/routes/tabs/Mods.tsx:55-100 | [p](procedures/web.md#server-detail-mods-list) | TestAPI_ModManifestInstallUpgrade | untested |  | [e](evidence/INV-WEB-043/) |  |  |
| INV-WEB-044 | Install mod from URL | web/ | web/src/routes/tabs/Mods.tsx:65-100 | [p](procedures/web.md#server-detail-mods-install-url) | TestAPI_ModManifestInstallUpgrade, TestAPI_ModUpload | untested |  | [e](evidence/INV-WEB-044/) |  |  |
| INV-WEB-045 | Browse mod registry | web/ | web/src/routes/tabs/Mods.tsx:65-100 | [p](procedures/web.md#server-detail-mods-browse-registry) | TestAPI_ModManifestInstallUpgrade | untested |  | [e](evidence/INV-WEB-045/) |  |  |
| INV-WEB-046 | Install mod from registry | web/ | web/src/routes/tabs/Mods.tsx:65-100 | [p](procedures/web.md#server-detail-mods-install-registry) | TestAPI_ModManifestInstallUpgrade, TestAPI_ModUpload | untested |  | [e](evidence/INV-WEB-046/) |  |  |
| INV-WEB-047 | Upload custom mod file | web/ | web/src/routes/tabs/Mods.tsx:65-100 | [p](procedures/web.md#server-detail-mods-upload) | TestAPI_ModUpload | untested |  | [e](evidence/INV-WEB-047/) |  |  |
| INV-WEB-048 | Check for mod updates | web/ | web/src/routes/tabs/Mods.tsx:100 | [p](procedures/web.md#server-detail-mods-check-updates) | none | untested |  | [e](evidence/INV-WEB-048/) |  |  |
| INV-WEB-049 | Remove installed mod | web/ | web/src/routes/tabs/Mods.tsx:65-100 | [p](procedures/web.md#server-detail-mods-remove) | TestAPI_ModManifestInstallUpgrade | untested |  | [e](evidence/INV-WEB-049/) |  |  |
| INV-WEB-050 | Browse modpacks in registry | web/ | web/src/routes/tabs/Modpacks.tsx:29-100 | [p](procedures/web.md#server-detail-modpacks-browse) | none | untested |  | [e](evidence/INV-WEB-050/) |  |  |
| INV-WEB-051 | Install modpack | web/ | web/src/routes/tabs/Modpacks.tsx:29-100 | [p](procedures/web.md#server-detail-modpacks-install) | TestAPI_ModManifestInstallUpgrade, TestAPI_ModUpload | untested |  | [e](evidence/INV-WEB-051/) |  |  |
| INV-WEB-052 | View online players | web/ | web/src/routes/tabs/Players.tsx:28-100 | [p](procedures/web.md#server-detail-players-view) | TestAPI_AgentPlayers | untested |  | [e](evidence/INV-WEB-052/) |  |  |
| INV-WEB-053 | Kick player from server | web/ | web/src/routes/tabs/Players.tsx:28-100 | [p](procedures/web.md#server-detail-players-kick) | TestAPI_AgentPlayers | untested |  | [e](evidence/INV-WEB-053/) |  |  |
| INV-WEB-054 | Ban player | web/ | web/src/routes/tabs/Players.tsx:28-100 | [p](procedures/web.md#server-detail-players-ban) | TestAPI_AgentPlayers | untested |  | [e](evidence/INV-WEB-054/) |  |  |
| INV-WEB-055 | Unban player | web/ | web/src/routes/tabs/Players.tsx:28-100 | [p](procedures/web.md#server-detail-players-unban) | TestAPI_AgentPlayers | untested |  | [e](evidence/INV-WEB-055/) |  |  |
| INV-WEB-056 | Add player to whitelist | web/ | web/src/routes/tabs/Players.tsx:28-100 | [p](procedures/web.md#server-detail-players-whitelist-add) | TestAPI_AgentPlayers | untested |  | [e](evidence/INV-WEB-056/) |  |  |
| INV-WEB-057 | Remove player from whitelist | web/ | web/src/routes/tabs/Players.tsx:28-100 | [p](procedures/web.md#server-detail-players-whitelist-remove) | TestAPI_AgentPlayers | untested |  | [e](evidence/INV-WEB-057/) |  |  |
| INV-WEB-058 | Create backup now (from server detail) | web/ | web/src/routes/tabs/Backups.tsx:42-50 | [p](procedures/web.md#server-detail-backups-create-now) | TestBackup_OperatorMaterializesJob, TestRestore_RoundTrip | untested |  | [e](evidence/INV-WEB-058/) |  |  |
| INV-WEB-059 | View server backups list | web/ | web/src/routes/tabs/Backups.tsx:16-100 | [p](procedures/web.md#server-detail-backups-view-list) | TestBackupSchedule_CreatesBackupCR | untested |  | [e](evidence/INV-WEB-059/) |  |  |
| INV-WEB-060 | Restore from backup | web/ | web/src/routes/tabs/Backups.tsx:16-100 | [p](procedures/web.md#server-detail-backups-restore) | TestRestore_RoundTrip | untested |  | [e](evidence/INV-WEB-060/) |  |  |
| INV-WEB-061 | Create backup schedule | web/ | web/src/routes/tabs/Backups.tsx:16-100 | [p](procedures/web.md#server-detail-backups-schedule-create) | TestBackup_OperatorMaterializesJob, TestRestore_RoundTrip | untested |  | [e](evidence/INV-WEB-061/) |  |  |
| INV-WEB-062 | Delete backup schedule | web/ | web/src/routes/tabs/Backups.tsx:16-100 | [p](procedures/web.md#server-detail-backups-schedule-delete) | TestBackupSchedule_CreatesBackupCR | untested |  | [e](evidence/INV-WEB-062/) |  |  |
| INV-WEB-063 | Edit server general settings (name, description, labels) | web/ | web/src/routes/tabs/settings/General.tsx:7-100 | [p](procedures/web.md#server-detail-settings-general) | none | untested |  | [e](evidence/INV-WEB-063/) |  |  |
| INV-WEB-064 | Change server version | web/ | web/src/routes/tabs/settings/Version.tsx:1-50 | [p](procedures/web.md#server-detail-settings-version) | TestGameServer_VersionSwitch | untested |  | [e](evidence/INV-WEB-064/) |  |  |
| INV-WEB-065 | Edit server resources (CPU, memory, storage) | web/ | web/src/routes/tabs/settings/Resources.tsx:1-50 | [p](procedures/web.md#server-detail-settings-resources) | none | untested |  | [e](evidence/INV-WEB-065/) |  |  |
| INV-WEB-066 | Edit server networking (exposure, hostname, ports, tunnel) | web/ | web/src/routes/tabs/settings/Networking.tsx:1-50 | [p](procedures/web.md#server-detail-settings-networking) | none | untested |  | [e](evidence/INV-WEB-066/) |  |  |
| INV-WEB-067 | Edit server environment variables | web/ | web/src/routes/tabs/settings/EnvVars.tsx:1-50 | [p](procedures/web.md#server-detail-settings-environment) | none | untested |  | [e](evidence/INV-WEB-067/) |  |  |
| INV-WEB-068 | Edit server lifecycle policies (auto-restart, updates, idle sleep) | web/ | web/src/routes/tabs/settings/Lifecycle.tsx:1-50 | [p](procedures/web.md#server-detail-settings-lifecycle) | TestGameServer_IdleNeverSleepsWithoutAPlayerCount | untested |  | [e](evidence/INV-WEB-068/) |  |  |
| INV-WEB-069 | Manage server backup schedules in settings | web/ | web/src/routes/tabs/settings/Backups.tsx:1-50 | [p](procedures/web.md#server-detail-settings-scheduled-backups) | none | untested |  | [e](evidence/INV-WEB-069/) |  |  |
| INV-WEB-070 | Configure network packet capture | web/ | web/src/routes/tabs/settings/NetworkCapture.tsx:1-50 | [p](procedures/web.md#server-detail-settings-network-capture) | TestGameServer_NetworkCaptureStartStopDownload | untested |  | [e](evidence/INV-WEB-070/) |  |  |
| INV-WEB-071 | Configure node placement (affinity, anti-affinity) | web/ | web/src/routes/tabs/settings/Placement.tsx:1-50 | [p](procedures/web.md#server-detail-settings-placement) | none | untested |  | [e](evidence/INV-WEB-071/) |  |  |
| INV-WEB-072 | Manage server RBAC and access (role bindings) | web/ | web/src/routes/tabs/settings/Access.tsx:1-50 | [p](procedures/web.md#server-detail-settings-access) | TestAPI_RBAC_OperatorCanWriteServers_NotUsers | untested |  | [e](evidence/INV-WEB-072/) |  |  |
| INV-WEB-073 | Create share link with expiry | web/ | web/src/routes/tabs/settings/ShareLinks.tsx:1-100 | [p](procedures/web.md#server-detail-settings-sharelinks-create) | none | untested |  | [e](evidence/INV-WEB-073/) |  |  |
| INV-WEB-074 | View share links list | web/ | web/src/routes/tabs/settings/ShareLinks.tsx:1-100 | [p](procedures/web.md#server-detail-settings-sharelinks-view) | none | untested |  | [e](evidence/INV-WEB-074/) |  |  |
| INV-WEB-075 | Revoke share link | web/ | web/src/routes/tabs/settings/ShareLinks.tsx:1-100 | [p](procedures/web.md#server-detail-settings-sharelinks-revoke) | none | untested |  | [e](evidence/INV-WEB-075/) |  |  |
| INV-WEB-076 | Delete server | web/ | web/src/routes/tabs/settings/Danger.tsx:1-50 | [p](procedures/web.md#server-detail-settings-danger-delete) | TestGameServer_CascadingDelete | untested |  | [e](evidence/INV-WEB-076/) |  |  |
| INV-WEB-077 | Transfer server ownership | web/ | web/src/routes/tabs/settings/Danger.tsx:1-50 | [p](procedures/web.md#server-detail-settings-danger-transfer) | TestAPI_OwnerCollaboratorAccess | untested |  | [e](evidence/INV-WEB-077/) |  |  |
| INV-WEB-078 | Browse module catalog | web/ | web/src/routes/Modules.tsx:24-100 | [p](procedures/web.md#modules-catalog-browse) | TestModuleSourceAndModule | untested |  | [e](evidence/INV-WEB-078/) |  |  |
| INV-WEB-079 | Search modules by name | web/ | web/src/routes/Modules.tsx:36 | [p](procedures/web.md#modules-search) | TestModuleSourceAndModule | untested |  | [e](evidence/INV-WEB-079/) |  |  |
| INV-WEB-080 | Filter modules by source | web/ | web/src/routes/Modules.tsx:37 | [p](procedures/web.md#modules-filter-by-source) | TestModuleSourceAndModule | untested |  | [e](evidence/INV-WEB-080/) |  |  |
| INV-WEB-081 | Filter modules by category | web/ | web/src/routes/Modules.tsx:38 | [p](procedures/web.md#modules-filter-by-category) | TestModuleSourceAndModule | untested |  | [e](evidence/INV-WEB-081/) |  |  |
| INV-WEB-082 | Install module | web/ | web/src/routes/Modules.tsx:89-98 | [p](procedures/web.md#modules-install) | TestAPI_ModManifestInstallUpgrade, TestAPI_ModUpload | untested |  | [e](evidence/INV-WEB-082/) |  |  |
| INV-WEB-083 | Upgrade module | web/ | web/src/routes/Modules.tsx:100 | [p](procedures/web.md#modules-upgrade) | TestModuleSourceAndModule | untested |  | [e](evidence/INV-WEB-083/) |  |  |
| INV-WEB-084 | Uninstall module | web/ | web/src/routes/Modules.tsx:100 | [p](procedures/web.md#modules-uninstall) | TestAPI_ModManifestInstallUpgrade, TestAPI_ModUpload | untested |  | [e](evidence/INV-WEB-084/) |  |  |
| INV-WEB-085 | Upload custom module | web/ | web/src/routes/Modules.tsx:42 | [p](procedures/web.md#modules-upload-custom) | TestAPI_ModUpload | untested |  | [e](evidence/INV-WEB-085/) |  |  |
| INV-WEB-086 | Build custom module | web/ | web/src/routes/Modules.tsx:43 | [p](procedures/web.md#modules-build-custom) | none | untested |  | [e](evidence/INV-WEB-086/) |  |  |
| INV-WEB-087 | View cluster nodes | web/ | web/src/routes/Cluster.tsx:25-100 | [p](procedures/web.md#cluster-view-nodes) | none | untested |  | [e](evidence/INV-WEB-087/) |  |  |
| INV-WEB-088 | Download cluster kubeconfig | web/ | web/src/routes/Cluster.tsx:61-76 | [p](procedures/web.md#cluster-download-kubeconfig) | none | untested |  | [e](evidence/INV-WEB-088/) |  |  |
| INV-WEB-089 | Add node to cluster | web/ | web/src/routes/Cluster.tsx:52-59 | [p](procedures/web.md#cluster-add-node) | none | untested |  | [e](evidence/INV-WEB-089/) |  |  |
| INV-WEB-090 | List users | web/ | web/src/routes/Users.tsx:83-100 | [p](procedures/web.md#users-list-view) | TestAPI_RBAC_AdminCanReachAll | untested |  | [e](evidence/INV-WEB-090/) |  |  |
| INV-WEB-091 | Invite user | web/ | web/src/routes/Users.tsx:100 | [p](procedures/web.md#users-invite) | TestAPI_OperatorCannotInviteUsers | untested |  | [e](evidence/INV-WEB-091/) |  |  |
| INV-WEB-092 | Edit user role | web/ | web/src/routes/Users.tsx:100 | [p](procedures/web.md#users-edit-role) | TestAPI_CustomRole_Lifecycle | untested |  | [e](evidence/INV-WEB-092/) |  |  |
| INV-WEB-093 | Reset user password | web/ | web/src/routes/Users.tsx:100 | [p](procedures/web.md#users-reset-password) | none | untested |  | [e](evidence/INV-WEB-093/) |  |  |
| INV-WEB-094 | Delete user | web/ | web/src/routes/Users.tsx:100 | [p](procedures/web.md#users-delete) | none | untested |  | [e](evidence/INV-WEB-094/) |  |  |
| INV-WEB-095 | Manage custom roles | web/ | web/src/routes/Users.tsx:100 | [p](procedures/web.md#users-manage-roles) | none | untested |  | [e](evidence/INV-WEB-095/) |  |  |
| INV-WEB-096 | Manage service accounts | web/ | web/src/routes/Users.tsx:100 | [p](procedures/web.md#users-manage-service-accounts) | none | untested |  | [e](evidence/INV-WEB-096/) |  |  |
| INV-WEB-097 | Manage OIDC providers | web/ | web/src/routes/Users.tsx:100 | [p](procedures/web.md#users-manage-oidc-providers) | none | untested |  | [e](evidence/INV-WEB-097/) |  |  |
| INV-WEB-098 | Edit general admin settings | web/ | web/src/routes/AdminSettings.tsx:89-142 | [p](procedures/web.md#admin-settings-general) | none | untested |  | [e](evidence/INV-WEB-098/) |  |  |
| INV-WEB-099 | Configure authentication (local, SSO, OIDC) | web/ | web/src/routes/AdminSettings.tsx:89-142 | [p](procedures/web.md#admin-settings-auth) | none | untested |  | [e](evidence/INV-WEB-099/) |  |  |
| INV-WEB-100 | Configure backup destinations | web/ | web/src/routes/AdminSettings.tsx:131 | [p](procedures/web.md#admin-settings-backup-destinations) | none | untested |  | [e](evidence/INV-WEB-100/) |  |  |
| INV-WEB-101 | Manage module sources | web/ | web/src/routes/AdminSettings.tsx:132 | [p](procedures/web.md#admin-settings-module-sources) | none | untested |  | [e](evidence/INV-WEB-101/) |  |  |
| INV-WEB-102 | Configure mod registries | web/ | web/src/routes/AdminSettings.tsx:133 | [p](procedures/web.md#admin-settings-mod-registries) | none | untested |  | [e](evidence/INV-WEB-102/) |  |  |
| INV-WEB-103 | Configure notifications (Slack, email, webhook) | web/ | web/src/routes/AdminSettings.tsx:134 | [p](procedures/web.md#admin-settings-notifications) | none | untested |  | [e](evidence/INV-WEB-103/) |  |  |
| INV-WEB-104 | Configure telemetry collection | web/ | web/src/routes/AdminSettings.tsx:135 | [p](procedures/web.md#admin-settings-telemetry) | none | untested |  | [e](evidence/INV-WEB-104/) |  |  |
| INV-WEB-105 | View update availability | web/ | web/src/routes/AdminSettings.tsx:136 | [p](procedures/web.md#admin-settings-updates) | none | untested |  | [e](evidence/INV-WEB-105/) |  |  |
| INV-WEB-106 | View about information (version, build, license) | web/ | web/src/routes/AdminSettings.tsx:137 | [p](procedures/web.md#admin-settings-about) | none | untested |  | [e](evidence/INV-WEB-106/) |  |  |
| INV-WEB-107 | Select theme preset | web/ | web/src/routes/ThemeSettings.tsx:50-59 | [p](procedures/web.md#theme-settings-preset-selection) | none | untested |  | [e](evidence/INV-WEB-107/) |  |  |
| INV-WEB-108 | Select appearance mode (light, dark, system) | web/ | web/src/routes/ThemeSettings.tsx:81-85 | [p](procedures/web.md#theme-settings-appearance-mode) | none | untested |  | [e](evidence/INV-WEB-108/) |  |  |
| INV-WEB-109 | Select custom theme colors | web/ | web/src/routes/ThemeSettings.tsx:64-79 | [p](procedures/web.md#theme-settings-custom-colors) | none | untested |  | [e](evidence/INV-WEB-109/) |  |  |
| INV-WEB-110 | Edit custom CSS | web/ | web/src/routes/ThemeSettings.tsx:87-100 | [p](procedures/web.md#theme-settings-custom-css) | none | untested |  | [e](evidence/INV-WEB-110/) |  |  |
| INV-WEB-111 | Export theme configuration | web/ | web/src/routes/ThemeSettings.tsx:100 | [p](procedures/web.md#theme-settings-export) | none | untested |  | [e](evidence/INV-WEB-111/) |  |  |
| INV-WEB-112 | Import theme configuration | web/ | web/src/routes/ThemeSettings.tsx:100 | [p](procedures/web.md#theme-settings-import) | none | untested |  | [e](evidence/INV-WEB-112/) |  |  |
| INV-WEB-113 | List all backups | web/ | web/src/routes/Backups.tsx:53-100 | [p](procedures/web.md#backups-list-view) | none | untested |  | [e](evidence/INV-WEB-113/) |  |  |
| INV-WEB-114 | Filter backups by server | web/ | web/src/routes/Backups.tsx:91-100 | [p](procedures/web.md#backups-filter-by-server) | none | untested |  | [e](evidence/INV-WEB-114/) |  |  |
| INV-WEB-115 | Filter backups by phase | web/ | web/src/routes/Backups.tsx:91-100 | [p](procedures/web.md#backups-filter-by-phase) | none | untested |  | [e](evidence/INV-WEB-115/) |  |  |
| INV-WEB-116 | Create backup now from backups page | web/ | web/src/routes/Backups.tsx:53-100 | [p](procedures/web.md#backups-backup-now) | TestBackup_OperatorMaterializesJob, TestRestore_RoundTrip | untested |  | [e](evidence/INV-WEB-116/) |  |  |
| INV-WEB-117 | Restore from backup on backups page | web/ | web/src/routes/Backups.tsx:53-100 | [p](procedures/web.md#backups-restore) | TestRestore_RoundTrip | untested |  | [e](evidence/INV-WEB-117/) |  |  |
| INV-WEB-118 | View backup detail | web/ | web/src/routes/Backups.tsx:53-100 | [p](procedures/web.md#backups-view-detail) | none | untested |  | [e](evidence/INV-WEB-118/) |  |  |
| INV-WEB-119 | Create backup schedule on backups page | web/ | web/src/routes/Backups.tsx:53-100 | [p](procedures/web.md#backups-schedules-create) | TestBackup_OperatorMaterializesJob, TestRestore_RoundTrip | untested |  | [e](evidence/INV-WEB-119/) |  |  |
| INV-WEB-120 | Edit backup schedule | web/ | web/src/routes/Backups.tsx:53-100 | [p](procedures/web.md#backups-schedules-edit) | none | untested |  | [e](evidence/INV-WEB-120/) |  |  |
| INV-WEB-121 | Suspend/resume backup schedule | web/ | web/src/routes/Backups.tsx:53-100 | [p](procedures/web.md#backups-schedules-toggle-suspend) | none | untested |  | [e](evidence/INV-WEB-121/) |  |  |
| INV-WEB-122 | Delete backup schedule from backups page | web/ | web/src/routes/Backups.tsx:53-100 | [p](procedures/web.md#backups-schedules-delete) | TestBackupSchedule_CreatesBackupCR | untested |  | [e](evidence/INV-WEB-122/) |  |  |
| INV-WEB-123 | View restore operations | web/ | web/src/routes/Backups.tsx:53-100 | [p](procedures/web.md#backups-restores-view) | none | untested |  | [e](evidence/INV-WEB-123/) |  |  |
| INV-WEB-124 | View audit log events | web/ | web/src/routes/AuditLog.tsx:16-100 | [p](procedures/web.md#audit-log-view) | none | untested |  | [e](evidence/INV-WEB-124/) |  |  |
| INV-WEB-125 | Filter audit events by HTTP status class | web/ | web/src/routes/AuditLog.tsx:16-100 | [p](procedures/web.md#audit-log-filter-by-status-class) | none | untested |  | [e](evidence/INV-WEB-125/) |  |  |
| INV-WEB-126 | Filter audit events by HTTP method | web/ | web/src/routes/AuditLog.tsx:16-100 | [p](procedures/web.md#audit-log-filter-by-method) | none | untested |  | [e](evidence/INV-WEB-126/) |  |  |
| INV-WEB-127 | Filter audit events by actor | web/ | web/src/routes/AuditLog.tsx:16-100 | [p](procedures/web.md#audit-log-filter-by-actor) | none | untested |  | [e](evidence/INV-WEB-127/) |  |  |
| INV-WEB-128 | Paginate audit log | web/ | web/src/routes/AuditLog.tsx:25-33 | [p](procedures/web.md#audit-log-pagination) | none | untested |  | [e](evidence/INV-WEB-128/) |  |  |
| INV-WEB-129 | Export audit events to CSV | web/ | web/src/routes/AuditLog.tsx:35-46 | [p](procedures/web.md#audit-log-export-csv) | none | untested |  | [e](evidence/INV-WEB-129/) |  |  |
| INV-WEB-130 | View audit log integrity | web/ | web/src/routes/AuditLog.tsx:20-24 | [p](procedures/web.md#audit-log-verify-integrity) | none | untested |  | [e](evidence/INV-WEB-130/) |  |  |
| INV-WEB-131 | View API server logs | web/ | web/src/routes/AdminLogs.tsx:78-100 | [p](procedures/web.md#admin-logs-view-api) | none | untested |  | [e](evidence/INV-WEB-131/) |  |  |
| INV-WEB-132 | View operator logs | web/ | web/src/routes/AdminLogs.tsx:78-100 | [p](procedures/web.md#admin-logs-view-operator) | none | untested |  | [e](evidence/INV-WEB-132/) |  |  |
| INV-WEB-133 | Select log tail size | web/ | web/src/routes/AdminLogs.tsx:16-17 | [p](procedures/web.md#admin-logs-tail-option) | none | untested |  | [e](evidence/INV-WEB-133/) |  |  |
| INV-WEB-134 | Enable follow mode for system logs | web/ | web/src/routes/AdminLogs.tsx:81 | [p](procedures/web.md#admin-logs-follow) | none | untested |  | [e](evidence/INV-WEB-134/) |  |  |
| INV-WEB-135 | Download system logs | web/ | web/src/routes/AdminLogs.tsx:78-100 | [p](procedures/web.md#admin-logs-download) | none | untested |  | [e](evidence/INV-WEB-135/) |  |  |
| INV-WEB-136 | Access shared server link | web/ | web/src/routes/Share.tsx:46-100 | [p](procedures/web.md#share-access-link-view) | none | untested |  | [e](evidence/INV-WEB-136/) |  |  |
| INV-WEB-137 | Start server from share link | web/ | web/src/routes/Share.tsx:46-100 | [p](procedures/web.md#share-server-start) | none | untested |  | [e](evidence/INV-WEB-137/) |  |  |
| INV-WEB-138 | View shared server status | web/ | web/src/routes/Share.tsx:46-100 | [p](procedures/web.md#share-server-view-status) | none | untested |  | [e](evidence/INV-WEB-138/) |  |  |
| INV-WEB-139 | Copy server address from share link | web/ | web/src/routes/Share.tsx:46-100 | [p](procedures/web.md#share-link-copy-address) | none | untested |  | [e](evidence/INV-WEB-139/) |  |  |
| INV-WEB-140 | Set theme preference on share link | web/ | web/src/routes/Share.tsx:17-43 | [p](procedures/web.md#share-theme-preference) | none | untested |  | [e](evidence/INV-WEB-140/) |  |  |
| INV-WEB-141 | Select game template in create-server wizard | web/ | web/src/routes/CreateServer.tsx:541-645 | [p](procedures/web.md#create-server-wizard-pick-template) | none | untested |  | [e](evidence/INV-WEB-141/) |  |  |
| INV-WEB-142 | Select template version in create-server wizard | web/ | web/src/routes/CreateServer.tsx:646-684 | [p](procedures/web.md#create-server-wizard-pick-version) | none | untested |  | [e](evidence/INV-WEB-142/) |  |  |
| INV-WEB-143 | Configure server name, resources, and labels in create-server wizard | web/ | web/src/routes/CreateServer.tsx:685-823 | [p](procedures/web.md#create-server-wizard-configure) | none | untested |  | [e](evidence/INV-WEB-143/) |  |  |
| INV-WEB-144 | Configure networking (exposure, ports, tunnel) in create-server wizard | web/ | web/src/routes/CreateServer.tsx:824-1086 | [p](procedures/web.md#create-server-wizard-network) | none | untested |  | [e](evidence/INV-WEB-144/) |  |  |
| INV-WEB-145 | Review and submit new server from create-server wizard | web/ | web/src/routes/CreateServer.tsx:348-500 | [p](procedures/web.md#create-server-wizard-review-create) | TestAPI_LifecycleClone | untested |  | [e](evidence/INV-WEB-145/) |  |  |
| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|

## API
| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|
| INV-API-001 | Health check (read) | api/ | api/cmd/main.go:257 | [p](procedures/api.md#public-healthz-get) | none | untested |  | [e](INV-API-001/) |  |  |
| INV-API-002 | Metrics (read) | api/ | api/cmd/main.go:260 | [p](procedures/api.md#public-metrics-get) | none | untested |  | [e](INV-API-002/) |  |  |
| INV-API-003 | List auth providers (read) | api/ | api/cmd/main.go:267 | [p](procedures/api.md#public-auth-providers-list) | none | untested |  | [e](INV-API-003/) |  |  |
| INV-API-004 | Login (create session) | api/ | api/cmd/main.go:271 | [p](procedures/api.md#public-auth-login) | none | untested |  | [e](INV-API-004/) |  |  |
| INV-API-005 | Logout (destroy session) | api/ | api/cmd/main.go:272 | [p](procedures/api.md#public-auth-logout) | none | untested |  | [e](INV-API-005/) |  |  |
| INV-API-006 | OIDC authorize start | api/ | api/cmd/main.go:277 | [p](procedures/api.md#public-auth-oidc-provider-start) | none | untested |  | [e](INV-API-006/) |  |  |
| INV-API-007 | OIDC callback | api/ | api/cmd/main.go:278-279 | [p](procedures/api.md#public-auth-oidc-callback) | none | untested | blocked: external IdP | [e](INV-API-007/) |  |  |
| INV-API-008 | OIDC authorize start (legacy) | api/ | api/cmd/main.go:283 | [p](procedures/api.md#public-auth-oidc-provider-start) | none | untested |  | [e](INV-API-008/) |  |  |
| INV-API-009 | OIDC callback (legacy) | api/ | api/cmd/main.go:284-285 | [p](procedures/api.md#public-auth-oidc-callback) | none | untested | blocked: external IdP | [e](INV-API-009/) |  |  |
| INV-API-010 | Resolve public share link | api/ | api/internal/handlers/shares.go:40 | [p](procedures/api.md#public-shares-resolve) | none | untested |  | [e](INV-API-010/) |  |  |
| INV-API-011 | Start server from public share | api/ | api/internal/handlers/shares.go:41 | [p](procedures/api.md#public-shares-start) | none | untested | blocked: share creation requires auth | [e](INV-API-011/) |  |  |
| INV-API-012 | List servers | api/ | api/internal/handlers/resources.go:53-54 | [p](procedures/api.md#servers-list) | none | untested |  | [e](INV-API-012/) |  |  |
| INV-API-013 | Create server | api/ | api/internal/handlers/resources.go:55 | [p](procedures/api.md#servers-create) | none | untested |  | [e](INV-API-013/) |  |  |
| INV-API-014 | Get server | api/ | api/internal/handlers/resources.go:56 | [p](procedures/api.md#servers-get) | none | untested |  | [e](INV-API-014/) |  |  |
| INV-API-015 | Update server | api/ | api/internal/handlers/resources.go:57 | [p](procedures/api.md#servers-update) | none | untested |  | [e](INV-API-015/) |  |  |
| INV-API-016 | Delete server | api/ | api/internal/handlers/resources.go:58 | [p](procedures/api.md#servers-delete) | TestGameServer_CascadingDelete | untested |  | [e](INV-API-016/) |  |  |
| INV-API-017 | List templates | api/ | api/internal/handlers/resources.go:53-54 | [p](procedures/api.md#templates-list) | none | untested |  | [e](INV-API-017/) |  |  |
| INV-API-018 | Get template | api/ | api/internal/handlers/resources.go:56 | [p](procedures/api.md#templates-get) | none | untested |  | [e](INV-API-018/) |  |  |
| INV-API-019 | List backups | api/ | api/internal/handlers/resources.go:53-54 | [p](procedures/api.md#backups-list) | none | untested |  | [e](INV-API-019/) |  |  |
| INV-API-020 | Create backup | api/ | api/internal/handlers/resources.go:55 | [p](procedures/api.md#servers-create) | TestBackup_OperatorMaterializesJob, TestRestore_RoundTrip | untested | blocked: requires backup trigger; alternative: list backups | [e](INV-API-020/) |  |  |
| INV-API-021 | Get backup | api/ | api/internal/handlers/resources.go:56 | [p](procedures/api.md#servers-get) | none | untested |  | [e](INV-API-021/) |  |  |
| INV-API-022 | Delete backup | api/ | api/internal/handlers/resources.go:58 | [p](procedures/api.md#servers-delete) | none | untested |  | [e](INV-API-022/) |  |  |
| INV-API-023 | List schedules | api/ | api/internal/handlers/resources.go:53-54 | [p](procedures/api.md#schedules-list) | none | untested |  | [e](INV-API-023/) |  |  |
| INV-API-024 | Create schedule | api/ | api/internal/handlers/resources.go:55 | [p](procedures/api.md#servers-create) | none | untested |  | [e](INV-API-024/) |  |  |
| INV-API-025 | List restores | api/ | api/internal/handlers/resources.go:53-54 | [p](procedures/api.md#restores-list) | none | untested |  | [e](INV-API-025/) |  |  |
| INV-API-026 | Restore from backup | api/ | api/internal/handlers/resources.go:55 | [p](procedures/api.md#servers-create) | TestRestore_RoundTrip | untested | blocked: requires backup; alternative: list restores | [e](INV-API-026/) |  |  |
| INV-API-027 | Read audit log (paginated) | api/ | api/internal/handlers/audit.go:23 | [p](procedures/api.md#audit-read-paginated) | none | untested |  | [e](INV-API-027/) |  |  |
| INV-API-028 | Verify audit chain integrity | api/ | api/internal/handlers/audit.go:42 | [p](procedures/api.md#audit-verify-chain) | none | untested |  | [e](INV-API-028/) |  |  |
| INV-API-029 | Export audit log (CSV) | api/ | api/internal/handlers/audit.go:58 | [p](procedures/api.md#audit-export-csv) | none | untested |  | [e](INV-API-029/) |  |  |
| INV-API-030 | Export audit log (JSON) | api/ | api/internal/handlers/audit.go:58 | [p](procedures/api.md#audit-export-json) | none | untested |  | [e](INV-API-030/) |  |  |
| INV-API-031 | Read admin config | api/ | api/internal/handlers/config.go:53 | [p](procedures/api.md#config-read-all) | none | untested |  | [e](INV-API-031/) |  |  |
| INV-API-032 | Write admin config section | api/ | api/internal/handlers/config.go:54 | [p](procedures/api.md#config-write-section) | none | untested |  | [e](INV-API-032/) |  |  |
| INV-API-033 | Reset role mapping (auth config) | api/ | api/internal/handlers/config.go:55 | [p](procedures/api.md#config-write-section) | none | untested |  | [e](INV-API-033/) |  |  |
| INV-API-034 | Store OIDC provider secret | api/ | api/internal/handlers/auth_provider_secret.go:26 | [p](procedures/api.md#auth-provider-secret-put) | none | untested |  | [e](INV-API-034/) |  |  |
| INV-API-035 | Delete OIDC provider secret | api/ | api/internal/handlers/auth_provider_secret.go:27 | [p](procedures/api.md#auth-provider-secret-delete) | none | untested |  | [e](INV-API-035/) |  |  |
| INV-API-036 | Test notification sink | api/ | api/internal/handlers/notifications.go:33-34 | [p](procedures/api.md#notifications-test-sink) | none | untested | deferred: T031 | [e](INV-API-036/) |  |  |
| INV-API-037 | Store notification sink secret | api/ | api/internal/handlers/notifications.go:50 | [p](procedures/api.md#notifications-secret-put) | none | untested |  | [e](INV-API-037/) |  |  |
| INV-API-038 | Delete notification sink secret | api/ | api/internal/handlers/notifications.go:51 | [p](procedures/api.md#notifications-secret-put) | none | untested |  | [e](INV-API-038/) |  |  |
| INV-API-039 | Store mod registry secret | api/ | api/internal/handlers/registry_secret.go:24 | [p](procedures/api.md#registry-secret-put) | none | untested |  | [e](INV-API-039/) |  |  |
| INV-API-040 | Delete mod registry secret | api/ | api/internal/handlers/registry_secret.go:25 | [p](procedures/api.md#registry-secret-put) | none | untested |  | [e](INV-API-040/) |  |  |
| INV-API-041 | Enable network capture | api/ | api/internal/handlers/capture.go:67 | [p](procedures/api.md#capture-enable) | none | untested | deferred: T031 | [e](INV-API-041/) |  |  |
| INV-API-042 | Disable network capture | api/ | api/internal/handlers/capture.go:68 | [p](procedures/api.md#capture-enable) | none | untested | deferred: T031 | [e](INV-API-042/) |  |  |
| INV-API-043 | Start packet capture | api/ | api/internal/handlers/capture.go:69 | [p](procedures/api.md#capture-start) | none | untested | deferred: T031 | [e](INV-API-043/) |  |  |
| INV-API-044 | Stop packet capture | api/ | api/internal/handlers/capture.go:70 | [p](procedures/api.md#capture-start) | none | untested | deferred: T031 | [e](INV-API-044/) |  |  |
| INV-API-045 | List packet captures | api/ | api/internal/handlers/capture.go:71 | [p](procedures/api.md#capture-list) | none | untested | deferred: T031 | [e](INV-API-045/) |  |  |
| INV-API-046 | Get packet capture | api/ | api/internal/handlers/capture.go:72 | [p](procedures/api.md#capture-list) | none | untested | deferred: T031 | [e](INV-API-046/) |  |  |
| INV-API-047 | Download capture file | api/ | api/internal/handlers/capture.go:73 | [p](procedures/api.md#capture-list) | none | untested | deferred: T031 | [e](INV-API-047/) |  |  |
| INV-API-048 | Delete packet capture | api/ | api/internal/handlers/capture.go:74 | [p](procedures/api.md#capture-list) | none | untested | deferred: T031 | [e](INV-API-048/) |  |  |
| INV-API-049 | Start server | api/ | api/internal/handlers/lifecycle.go:41 | [p](procedures/api.md#servers-start) | none | untested |  | [e](INV-API-049/) |  |  |
| INV-API-050 | Stop server | api/ | api/internal/handlers/lifecycle.go:42 | [p](procedures/api.md#servers-stop) | none | untested |  | [e](INV-API-050/) |  |  |
| INV-API-051 | Restart server | api/ | api/internal/handlers/lifecycle.go:43 | [p](procedures/api.md#servers-restart) | none | untested |  | [e](INV-API-051/) |  |  |
| INV-API-052 | Wake server from idle | api/ | api/internal/handlers/lifecycle.go:44 | [p](procedures/api.md#servers-restart) | none | untested |  | [e](INV-API-052/) |  |  |
| INV-API-053 | Clone server | api/ | api/internal/handlers/lifecycle.go:45 | [p](procedures/api.md#servers-clone) | none | untested |  | [e](INV-API-053/) |  |  |
| INV-API-054 | Wipe server data | api/ | api/internal/handlers/lifecycle.go:46 | [p](procedures/api.md#servers-wipe-data) | none | untested |  | [e](INV-API-054/) |  |  |
| INV-API-055 | Create share link | api/ | api/internal/handlers/shares.go:27-28 | [p](procedures/api.md#shares-create) | none | untested | deferred: T031 (OD-015 blocks) | [e](INV-API-055/) |  |  |
| INV-API-056 | List share links | api/ | api/internal/handlers/shares.go:29 | [p](procedures/api.md#shares-list) | none | untested | deferred: T031 (OD-015 blocks) | [e](INV-API-056/) |  |  |
| INV-API-057 | Revoke share link | api/ | api/internal/handlers/shares.go:31 | [p](procedures/api.md#shares-list) | none | untested | deferred: T031 (OD-015 blocks) | [e](INV-API-057/) |  |  |
| INV-API-058 | Transfer server ownership | api/ | api/internal/handlers/ownership.go:58 | [p](procedures/api.md#servers-transfer) | TestAPI_OwnerCollaboratorAccess | untested | deferred: T031 (OD-015 blocks) | [e](INV-API-058/) |  |  |
| INV-API-059 | Set server collaborators | api/ | api/internal/handlers/ownership.go:59 | [p](procedures/api.md#servers-collaborators-set) | none | untested | deferred: T031 (OD-015 blocks) | [e](INV-API-059/) |  |  |
| INV-API-060 | Get owned servers | api/ | api/internal/handlers/ownership.go:60 | [p](procedures/api.md#servers-transfer) | none | untested |  | [e](INV-API-060/) |  |  |
| INV-API-061 | List users | api/ | api/internal/handlers/users.go:29 | [p](procedures/api.md#users-list) | TestAPI_RBAC_AdminCanReachAll | untested |  | [e](INV-API-061/) |  |  |
| INV-API-062 | Create user | api/ | api/internal/handlers/users.go:30 | [p](procedures/api.md#users-create) | none | untested |  | [e](INV-API-062/) |  |  |
| INV-API-063 | Get current user (/me) | api/ | api/internal/handlers/users.go:31 | [p](procedures/api.md#users-me) | none | untested |  | [e](INV-API-063/) |  |  |
| INV-API-064 | Get user preferences | api/ | api/internal/handlers/users.go:32 | [p](procedures/api.md#users-preferences-get) | none | untested |  | [e](INV-API-064/) |  |  |
| INV-API-065 | Update user preferences | api/ | api/internal/handlers/users.go:33 | [p](procedures/api.md#users-preferences-put) | none | untested |  | [e](INV-API-065/) |  |  |
| INV-API-066 | Reset user preferences | api/ | api/internal/handlers/users.go:34 | [p](procedures/api.md#users-preferences-reset) | none | untested |  | [e](INV-API-066/) |  |  |
| INV-API-067 | Get user | api/ | api/internal/handlers/users.go:35 | [p](procedures/api.md#users-get) | none | n/a | withdrawn at T024: no GET /users/{id} route exists in MountUsers; line 35 is `r.Delete("/{id}", h.del)`, and no `get` method exists on userHandler | [e](INV-API-067/) |  |  |
| INV-API-068 | Update user | api/ | api/internal/handlers/users.go:36 | [p](procedures/api.md#users-update) | none | untested |  | [e](INV-API-068/) |  |  |
| INV-API-069 | Delete user | api/ | api/internal/handlers/users.go:35 | [p](procedures/api.md#users-delete) | none | untested |  | [e](INV-API-069/) |  |  |
| INV-API-070 | Reset user password | api/ | api/internal/handlers/users.go:37 | [p](procedures/api.md#users-reset-password) | none | untested | deferred: T031 | [e](INV-API-070/) |  |  |
| INV-API-071 | List user role bindings | api/ | api/internal/handlers/users.go:40 | [p](procedures/api.md#users-bindings-list) | none | untested |  | [e](INV-API-071/) |  |  |
| INV-API-072 | Add user role binding | api/ | api/internal/handlers/users.go:41 | [p](procedures/api.md#users-bindings-add) | none | untested |  | [e](INV-API-072/) |  |  |
| INV-API-073 | Delete user role binding | api/ | api/internal/handlers/users.go:42 | [p](procedures/api.md#users-bindings-add) | none | untested |  | [e](INV-API-073/) |  |  |
| INV-API-074 | List roles | api/ | api/internal/handlers/roles.go:22-23 | [p](procedures/api.md#roles-list) | none | untested |  | [e](INV-API-074/) |  |  |
| INV-API-075 | Get permission catalog | api/ | api/internal/handlers/roles.go:26 | [p](procedures/api.md#roles-permissions-catalog) | none | untested |  | [e](INV-API-075/) |  |  |
| INV-API-076 | Create custom role | api/ | api/internal/handlers/roles.go:27 | [p](procedures/api.md#roles-list) | none | untested |  | [e](INV-API-076/) |  |  |
| INV-API-077 | Update role | api/ | api/internal/handlers/roles.go:28 | [p](procedures/api.md#roles-list) | none | untested |  | [e](INV-API-077/) |  |  |
| INV-API-078 | Delete role | api/ | api/internal/handlers/roles.go:29 | [p](procedures/api.md#roles-list) | none | untested |  | [e](INV-API-078/) |  |  |
| INV-API-079 | Get cluster info | api/ | api/internal/handlers/cluster.go:29 | [p](procedures/api.md#cluster-view) | none | untested |  | [e](INV-API-079/) |  |  |
| INV-API-080 | Get cluster metadata | api/ | api/internal/handlers/cluster.go:30 | [p](procedures/api.md#cluster-info) | none | untested |  | [e](INV-API-080/) |  |  |
| INV-API-081 | Get cluster stats | api/ | api/internal/handlers/cluster.go:31 | [p](procedures/api.md#cluster-stats) | none | untested |  | [e](INV-API-081/) |  |  |
| INV-API-082 | Join node to cluster | api/ | api/internal/handlers/cluster_actions.go:40 | [p](procedures/api.md#cluster-join-node) | none | untested |  | [e](INV-API-082/) |  |  |
| INV-API-083 | Download kubeconfig | api/ | api/internal/handlers/cluster_actions.go:41 | [p](procedures/api.md#cluster-download-kubeconfig) | none | untested |  | [e](INV-API-083/) |  |  |
| INV-API-084 | List remote clusters | api/ | api/internal/handlers/clusters.go:27-28 | [p](procedures/api.md#clusters-list) | none | untested |  | [e](INV-API-084/) |  |  |
| INV-API-085 | Register remote cluster | api/ | api/internal/handlers/clusters.go:29 | [p](procedures/api.md#clusters-list) | none | untested |  | [e](INV-API-085/) |  |  |
| INV-API-086 | Deregister remote cluster | api/ | api/internal/handlers/clusters.go:30 | [p](procedures/api.md#clusters-list) | none | untested |  | [e](INV-API-086/) |  |  |
| INV-API-087 | List installed modules | api/ | api/internal/handlers/modules.go:38-39 | [p](procedures/api.md#modules-list-installed) | none | untested |  | [e](INV-API-087/) |  |  |
| INV-API-088 | Install module | api/ | api/internal/handlers/modules.go:40 | [p](procedures/api.md#modules-list-installed) | TestAPI_ModManifestInstallUpgrade, TestAPI_ModUpload | untested |  | [e](INV-API-088/) |  |  |
| INV-API-089 | List module sources | api/ | api/internal/handlers/modules.go:41 | [p](procedures/api.md#modules-list-sources) | none | untested |  | [e](INV-API-089/) |  |  |
| INV-API-090 | Create module source | api/ | api/internal/handlers/modules.go:42 | [p](procedures/api.md#modules-list-sources) | none | untested |  | [e](INV-API-090/) |  |  |
| INV-API-091 | Update module source | api/ | api/internal/handlers/modules.go:43 | [p](procedures/api.md#modules-list-sources) | none | untested |  | [e](INV-API-091/) |  |  |
| INV-API-092 | Delete module source | api/ | api/internal/handlers/modules.go:44 | [p](procedures/api.md#modules-list-sources) | none | untested |  | [e](INV-API-092/) |  |  |
| INV-API-093 | Upload module bundle | api/ | api/internal/handlers/modules.go:45 | [p](procedures/api.md#modules-list-sources) | none | untested |  | [e](INV-API-093/) |  |  |
| INV-API-094 | Delete uploaded module | api/ | api/internal/handlers/modules.go:46 | [p](procedures/api.md#modules-list-sources) | none | untested |  | [e](INV-API-094/) |  |  |
| INV-API-095 | Get merged module catalog | api/ | api/internal/handlers/modules.go:47 | [p](procedures/api.md#modules-catalog) | none | untested |  | [e](INV-API-095/) |  |  |
| INV-API-096 | List module archetypes | api/ | api/internal/handlers/modules.go:49 | [p](procedures/api.md#modules-list-installed) | none | untested |  | [e](INV-API-096/) |  |  |
| INV-API-097 | Scaffold module | api/ | api/internal/handlers/modules.go:50 | [p](procedures/api.md#modules-list-installed) | none | untested |  | [e](INV-API-097/) |  |  |
| INV-API-098 | Validate module | api/ | api/internal/handlers/modules.go:51 | [p](procedures/api.md#modules-list-installed) | none | untested |  | [e](INV-API-098/) |  |  |
| INV-API-099 | Preview module | api/ | api/internal/handlers/modules.go:52 | [p](procedures/api.md#modules-list-installed) | none | untested |  | [e](INV-API-099/) |  |  |
| INV-API-100 | Export module | api/ | api/internal/handlers/modules.go:53 | [p](procedures/api.md#modules-list-installed) | none | untested |  | [e](INV-API-100/) |  |  |
| INV-API-101 | Get installed module | api/ | api/internal/handlers/modules.go:55 | [p](procedures/api.md#modules-list-installed) | none | untested |  | [e](INV-API-101/) |  |  |
| INV-API-102 | Upgrade module | api/ | api/internal/handlers/modules.go:56 | [p](procedures/api.md#modules-list-installed) | TestModuleSourceAndModule | untested |  | [e](INV-API-102/) |  |  |
| INV-API-103 | Uninstall module | api/ | api/internal/handlers/modules.go:57 | [p](procedures/api.md#modules-list-installed) | TestAPI_ModManifestInstallUpgrade, TestAPI_ModUpload | untested |  | [e](INV-API-103/) |  |  |
| INV-API-104 | List mod registry providers | api/ | api/internal/handlers/registry.go:43 | [p](procedures/api.md#servers-list) | none | untested |  | [e](INV-API-104/) |  |  |
| INV-API-105 | Search mod registry | api/ | api/internal/handlers/registry.go:44 | [p](procedures/api.md#servers-list) | none | untested |  | [e](INV-API-105/) |  |  |
| INV-API-106 | Get mod versions | api/ | api/internal/handlers/registry.go:45 | [p](procedures/api.md#servers-list) | none | untested |  | [e](INV-API-106/) |  |  |
| INV-API-107 | Get modpack dependencies | api/ | api/internal/handlers/registry.go:46 | [p](procedures/api.md#servers-list) | none | untested |  | [e](INV-API-107/) |  |  |
| INV-API-108 | Install modpack | api/ | api/internal/handlers/registry.go:47 | [p](procedures/api.md#servers-list) | TestAPI_ModManifestInstallUpgrade, TestAPI_ModUpload | untested |  | [e](INV-API-108/) |  |  |
| INV-API-109 | Check mod updates | api/ | api/internal/handlers/mod_updates.go:40 | [p](procedures/api.md#servers-list) | none | untested |  | [e](INV-API-109/) |  |  |
| INV-API-110 | Get server mod IDs | api/ | api/internal/handlers/mod_ids.go:64 | [p](procedures/api.md#servers-list) | none | untested |  | [e](INV-API-110/) |  |  |
| INV-API-111 | Update server mod IDs | api/ | api/internal/handlers/mod_ids.go:65 | [p](procedures/api.md#servers-list) | none | untested |  | [e](INV-API-111/) |  |  |
| INV-API-112 | Get tunnel credentials | api/ | api/internal/handlers/tunnelcreds.go:33 | [p](procedures/api.md#servers-list) | none | untested |  | [e](INV-API-112/) |  |  |
| INV-API-113 | Set tunnel credentials | api/ | api/internal/handlers/tunnelcreds.go:32 | [p](procedures/api.md#servers-list) | none | untested |  | [e](INV-API-113/) |  |  |
| INV-API-114 | Delete tunnel credentials | api/ | api/internal/handlers/tunnelcreds.go:34 | [p](procedures/api.md#servers-list) | none | untested |  | [e](INV-API-114/) |  |  |
| INV-API-115 | List backup destinations | api/ | api/internal/handlers/destinations.go:42-45 | [p](procedures/api.md#backup-destinations-list) | none | untested |  | [e](INV-API-115/) |  |  |
| INV-API-116 | Create backup destination | api/ | api/internal/handlers/destinations.go:43 | [p](procedures/api.md#backup-destinations-list) | TestBackup_OperatorMaterializesJob, TestRestore_RoundTrip | untested |  | [e](INV-API-116/) |  |  |
| INV-API-117 | Get backup destination | api/ | api/internal/handlers/destinations.go:44 | [p](procedures/api.md#backup-destinations-list) | none | untested |  | [e](INV-API-117/) |  |  |
| INV-API-118 | Delete backup destination | api/ | api/internal/handlers/destinations.go:45 | [p](procedures/api.md#backup-destinations-list) | none | untested |  | [e](INV-API-118/) |  |  |
| INV-API-119 | Watch events (SSE) | api/ | api/internal/handlers/events.go:24 | [p](procedures/api.md#events-sse) | none | untested |  | [e](INV-API-119/) |  |  |
| INV-API-120 | Get pod events | api/ | api/internal/handlers/pod_events.go:27 | [p](procedures/api.md#servers-list) | none | untested |  | [e](INV-API-120/) |  |  |
| INV-API-121 | WebSocket console | api/ | api/internal/ws/dialer.go:41 | [p](procedures/api.md#ws-console) | none | untested | deferred: T031 | [e](INV-API-121/) |  |  |
| INV-API-122 | WebSocket logs | api/ | api/internal/ws/dialer.go:42 | [p](procedures/api.md#ws-logs) | none | untested | deferred: T031 | [e](INV-API-122/) |  |  |
| INV-API-123 | Download logs | api/ | api/internal/ws/dialer.go:43 | [p](procedures/api.md#ws-logs) | none | untested |  | [e](INV-API-123/) |  |  |
| INV-API-124 | List server files | api/ | api/internal/ws/dialer.go:53 | [p](procedures/api.md#files-list) | none | untested |  | [e](INV-API-124/) |  |  |
| INV-API-125 | Read server file | api/ | api/internal/ws/dialer.go:54 | [p](procedures/api.md#files-read) | none | untested |  | [e](INV-API-125/) |  |  |
| INV-API-126 | Download server file | api/ | api/internal/ws/dialer.go:55 | [p](procedures/api.md#files-read) | none | untested |  | [e](INV-API-126/) |  |  |
| INV-API-127 | Write server file | api/ | api/internal/ws/dialer.go:56 | [p](procedures/api.md#files-read) | none | untested |  | [e](INV-API-127/) |  |  |
| INV-API-128 | Upload server file | api/ | api/internal/ws/dialer.go:57 | [p](procedures/api.md#files-upload) | none | untested |  | [e](INV-API-128/) |  |  |
| INV-API-129 | Create directory | api/ | api/internal/ws/dialer.go:58 | [p](procedures/api.md#files-read) | none | untested |  | [e](INV-API-129/) |  |  |
| INV-API-130 | Delete file/directory | api/ | api/internal/ws/dialer.go:59 | [p](procedures/api.md#files-read) | TestAPI_AgentFilesRoundTrip | untested |  | [e](INV-API-130/) |  |  |
| INV-API-131 | List server players | api/ | api/internal/ws/dialer.go:62 | [p](procedures/api.md#players-list) | none | untested |  | [e](INV-API-131/) |  |  |
| INV-API-132 | List banned players | api/ | api/internal/ws/dialer.go:63 | [p](procedures/api.md#players-list) | none | untested |  | [e](INV-API-132/) |  |  |
| INV-API-133 | Kick player | api/ | api/internal/ws/dialer.go:64 | [p](procedures/api.md#players-kick) | TestAPI_AgentPlayers | untested | deferred: T031 | [e](INV-API-133/) |  |  |
| INV-API-134 | Ban player | api/ | api/internal/ws/dialer.go:65 | [p](procedures/api.md#players-kick) | TestAPI_AgentPlayers | untested | deferred: T031 | [e](INV-API-134/) |  |  |
| INV-API-135 | Unban player | api/ | api/internal/ws/dialer.go:66 | [p](procedures/api.md#players-kick) | TestAPI_AgentPlayers | untested | deferred: T031 | [e](INV-API-135/) |  |  |
| INV-API-136 | Get whitelist | api/ | api/internal/ws/dialer.go:67 | [p](procedures/api.md#players-list) | none | untested |  | [e](INV-API-136/) |  |  |
| INV-API-137 | Add whitelist entry | api/ | api/internal/ws/dialer.go:68 | [p](procedures/api.md#players-list) | none | untested |  | [e](INV-API-137/) |  |  |
| INV-API-138 | Remove whitelist entry | api/ | api/internal/ws/dialer.go:69 | [p](procedures/api.md#players-list) | none | untested |  | [e](INV-API-138/) |  |  |
| INV-API-139 | Run operator action | api/ | api/internal/ws/dialer.go:78 | [p](procedures/api.md#servers-list) | none | untested |  | [e](INV-API-139/) |  |  |
| INV-API-140 | Get server status | api/ | api/internal/ws/dialer.go:79 | [p](procedures/api.md#servers-list) | none | untested |  | [e](INV-API-140/) |  |  |
| INV-API-141 | List server mods | api/ | api/internal/ws/dialer.go:85 | [p](procedures/api.md#servers-list) | none | untested |  | [e](INV-API-141/) |  |  |
| INV-API-142 | Install mod | api/ | api/internal/ws/dialer.go:86 | [p](procedures/api.md#servers-list) | TestAPI_ModManifestInstallUpgrade, TestAPI_ModUpload | untested |  | [e](INV-API-142/) |  |  |
| INV-API-143 | Upload mod | api/ | api/internal/ws/dialer.go:87 | [p](procedures/api.md#servers-list) | none | untested |  | [e](INV-API-143/) |  |  |
| INV-API-144 | Delete mod | api/ | api/internal/ws/dialer.go:88 | [p](procedures/api.md#servers-list) | none | untested |  | [e](INV-API-144/) |  |  |
| INV-API-145 | Get system logs (API) | api/ | api/internal/handlers/systemlogs.go:24 | [p](procedures/api.md#system-logs-api) | none | untested |  | [e](INV-API-145/) |  |  |
| INV-API-146 | Get system logs (Operator) | api/ | api/internal/handlers/systemlogs.go:24 | [p](procedures/api.md#system-logs-operator) | none | untested |  | [e](INV-API-146/) |  |  |
| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|

## CRD
| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|
| INV-CRD-001 | Create server from template | operator/ | operator/api/v1alpha1/gameserver_types.go:1-100 | [p](procedures/crd.md#gameserver-create-from-template) | none | untested |  | [e](INV-CRD-001/) |  |  |
| INV-CRD-002 | Phase transition Pending → Starting | operator/ | operator/internal/controller/gameserver_status.go:268-293 | [p](procedures/crd.md#gameserver-phase-pending-to-starting) | none | untested |  | [e](INV-CRD-002/) |  |  |
| INV-CRD-003 | Phase transition Starting → Running | operator/ | operator/internal/controller/gameserver_status.go:268-293 | [p](procedures/crd.md#gameserver-phase-starting-to-running) | none | untested |  | [e](INV-CRD-003/) |  |  |
| INV-CRD-004 | Suspend (soft stop) game server | operator/ | operator/internal/controller/gameserver_controller.go:255-470 | [p](procedures/crd.md#gameserver-suspend) | none | untested |  | [e](INV-CRD-004/) |  |  |
| INV-CRD-005 | Unsuspend and wake game server | operator/ | operator/internal/controller/gameserver_controller.go:255-470 | [p](procedures/crd.md#gameserver-unsuspend-wake) | none | untested |  | [e](INV-CRD-005/) |  |  |
| INV-CRD-006 | Restart game server pod | operator/ | operator/internal/controller/gameserver_restart.go:1-50 | [p](procedures/crd.md#gameserver-restart) | none | untested |  | [e](INV-CRD-006/) |  |  |
| INV-CRD-007 | Switch server version | operator/ | operator/internal/controller/gameserver_version.go:1-100 | [p](procedures/crd.md#gameserver-version-switch) | none | untested |  | [e](INV-CRD-007/) |  |  |
| INV-CRD-008 | Wipe server data volume | operator/ | operator/internal/controller/gameserver_wipe.go:1-150 | [p](procedures/crd.md#gameserver-wipe-data-volume) | none | untested |  | [e](INV-CRD-008/) |  |  |
| INV-CRD-009 | Idle auto-sleep (zero players) | operator/ | operator/internal/controller/gameserver_idle.go:1-150 | [p](procedures/crd.md#gameserver-idle-auto-sleep) | none | untested |  | [e](INV-CRD-009/) |  |  |
| INV-CRD-010 | Wake via scheduled window | operator/ | operator/internal/controller/gameserver_idle.go:1-150 | [p](procedures/crd.md#gameserver-idle-wake-window) | none | untested |  | [e](INV-CRD-010/) |  |  |
| INV-CRD-011 | Wake on player connect (sentinel) | operator/ | operator/internal/controller/gameserver_sentinel.go:1-200 | [p](procedures/crd.md#gameserver-idle-wake-on-connect) | none | untested | user-facing; sentinel pod injected | [e](INV-CRD-011/) |  |  |
| INV-CRD-012 | Delete game server with finalizer | operator/ | operator/internal/controller/gameserver_controller.go:255-470 | [p](procedures/crd.md#gameserver-delete-with-finalizer) | none | untested |  | [e](INV-CRD-012/) |  |  |
| INV-CRD-013 | Phase Failed (crash loop) | operator/ | operator/internal/controller/gameserver_status.go:104-135 | [p](procedures/crd.md#gameserver-failed-phase-crash-loop) | none | untested |  | [e](INV-CRD-013/) |  |  |
| INV-CRD-014 | Create game template | operator/ | operator/api/v1alpha1/gametemplate_types.go:1-200 | [p](procedures/crd.md#gametemplate-create) | none | untested |  | [e](INV-CRD-014/) |  |  |
| INV-CRD-015 | Create backup (Pending → Running) | operator/ | operator/internal/controller/backup_controller.go:173-263 | [p](procedures/crd.md#backup-create-and-run) | TestBackup_OperatorMaterializesJob, TestRestore_RoundTrip | untested |  | [e](INV-CRD-015/) |  |  |
| INV-CRD-016 | Backup phase Running → Succeeded | operator/ | operator/internal/controller/backup_controller.go:316-450 | [p](procedures/crd.md#backup-create-and-run) | none | untested |  | [e](INV-CRD-016/) |  |  |
| INV-CRD-017 | Backup phase failure (Failed) | operator/ | operator/internal/controller/backup_controller.go:584-607 | [p](procedures/crd.md#backup-failure-and-phase) | none | untested |  | [e](INV-CRD-017/) |  |  |
| INV-CRD-018 | Restore Pending → Suspending | operator/ | operator/internal/controller/restore_controller.go:40-200 | [p](procedures/crd.md#restore-create-and-suspend-server) | none | untested |  | [e](INV-CRD-018/) |  |  |
| INV-CRD-019 | Restore Suspending → Running | operator/ | operator/internal/controller/restore_controller.go:40-200 | [p](procedures/crd.md#restore-create-and-suspend-server) | none | untested |  | [e](INV-CRD-019/) |  |  |
| INV-CRD-020 | Restore Running → Resuming → Succeeded | operator/ | operator/internal/controller/restore_controller.go:40-200 | [p](procedures/crd.md#restore-complete-and-resume) | none | untested |  | [e](INV-CRD-020/) |  |  |
| INV-CRD-021 | BackupSchedule create and schedule | operator/ | operator/internal/controller/backupschedule_controller.go:49-200 | [p](procedures/crd.md#backup-schedule-create) | none | untested |  | [e](INV-CRD-021/) |  |  |
| INV-CRD-022 | BackupSchedule tick creates Backup | operator/ | operator/internal/controller/backupschedule_controller.go:49-200 | [p](procedures/crd.md#backup-schedule-tick-creates-backup) | none | untested |  | [e](INV-CRD-022/) |  |  |
| INV-CRD-023 | BackupSchedule retention prunes backups | operator/ | operator/internal/controller/backupschedule_controller.go:49-200 | [p](procedures/crd.md#backup-schedule-retention-prunes) | none | untested |  | [e](INV-CRD-023/) |  |  |
| INV-CRD-024 | Module Pending phase (version resolving) | operator/ | operator/internal/controller/module_controller.go:58-150 | [p](procedures/crd.md#module-create-pending) | none | untested |  | [e](INV-CRD-024/) |  |  |
| INV-CRD-025 | Module Pulling → Ready (materialize template) | operator/ | operator/internal/controller/module_controller.go:58-150 | [p](procedures/crd.md#module-pulling-to-ready) | none | untested |  | [e](INV-CRD-025/) |  |  |
| INV-CRD-026 | Module bad signature fails (cosign verify) | operator/ | operator/internal/controller/module_controller.go:58-150 | [p](procedures/crd.md#module-bad-signature-fails) | none | untested | blocked candidate: requires external signed OCI bundle; alternative: test with unsigned bundle in local OCI source | [e](INV-CRD-026/) |  |  |
| INV-CRD-027 | ModuleSource OCI periodic refresh/sync | operator/ | operator/internal/controller/modulesource_controller.go:51-150 | [p](procedures/crd.md#modulesource-oci-sync) | none | untested |  | [e](INV-CRD-027/) |  |  |
| INV-CRD-028 | ModuleSource git sync error | operator/ | operator/internal/controller/modulesource_controller.go:51-150 | [p](procedures/crd.md#modulesource-git-sync-error) | none | untested | blocked candidate: requires intentional git repo failure; alternative: observe sync timeout with slow network | [e](INV-CRD-028/) |  |  |
| INV-CRD-029 | Cluster register and health check | operator/ | operator/internal/controller/cluster_controller.go:31-170 | [p](procedures/crd.md#cluster-register-and-health-check) | none | untested | user-facing; multicluster feature | [e](INV-CRD-029/) |  |  |
| INV-CRD-030 | Cluster health check failure (Unhealthy) | operator/ | operator/internal/controller/cluster_controller.go:130-165 | [p](procedures/crd.md#cluster-health-check-unreachable) | none | untested | user-facing; multicluster feature | [e](INV-CRD-030/) |  |  |
| INV-CRD-031 | NetworkCapture Pending phase | operator/ | operator/internal/controller/networkcapture_controller.go:139-250 | [p](procedures/crd.md#networkcapture-create-pending) | none | untested | user-facing; requires capture sidecar enabled | [e](INV-CRD-031/) |  |  |
| INV-CRD-032 | NetworkCapture Pending → Running | operator/ | operator/internal/controller/networkcapture_controller.go:139-250 | [p](procedures/crd.md#networkcapture-start-running) | none | untested | user-facing; requires capture sidecar enabled | [e](INV-CRD-032/) |  |  |
| INV-CRD-033 | NetworkCapture stop and complete | operator/ | operator/internal/controller/networkcapture_controller.go:139-250 | [p](procedures/crd.md#networkcapture-stop-completed) | none | untested | user-facing; requires capture sidecar enabled | [e](INV-CRD-033/) |  |  |
| INV-CRD-034 | NetworkCapture failure (sidecar crash) | operator/ | operator/internal/controller/networkcapture_controller.go:139-250 | [p](procedures/crd.md#networkcapture-failed-sidecar-crash) | none | untested | user-facing; ephemeral container restart behavior | [e](INV-CRD-034/) |  |  |
| INV-CRD-035 | NetworkCapture auto-delete after TTL | operator/ | operator/internal/controller/networkcapture_controller.go:139-250 | [p](procedures/crd.md#networkcapture-expired-auto-delete) | none | untested | user-facing; garbage collection | [e](INV-CRD-035/) |  |  |
| INV-CRD-036 | Create backup using VolumeSnapshot strategy (spec.strategy=volume-snapshot) | operator/ | operator/internal/controller/backup_volumesnapshot.go:26-68 | [p](procedures/crd.md#backup-volumesnapshot-strategy) | TestBackup_VolumeSnapshotSucceeds | untested |  | [e](INV-CRD-036/) |  |  |
| INV-CRD-037 | Restore from a VolumeSnapshot-strategy backup | operator/ | operator/internal/controller/restore_volumesnapshot.go:24-103 | [p](procedures/crd.md#restore-volumesnapshot-strategy) | TestRestore_VolumeSnapshotProvisionsNewServer | untested |  | [e](INV-CRD-037/) |  |  |
| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|

## AGT
| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|
| INV-AGT-001 | Execute command via source RCON | agent/internal/console | agent/internal/rcon/rcon.go:122-188 | [p](procedures/agent.md#console-source) | none | untested |  | [e](evidence/INV-AGT-001/) |  |  |
| INV-AGT-002 | Execute command via telnet RCON | agent/internal/console | agent/internal/rcon/telnet.go:1 | [p](procedures/agent.md#console-telnet) | none | untested |  | [e](evidence/INV-AGT-002/) |  |  |
| INV-AGT-003 | Execute command via websocket RCON | agent/internal/console | agent/internal/rcon/websocket.go:1 | [p](procedures/agent.md#console-websocket) | none | untested |  | [e](evidence/INV-AGT-003/) |  |  |
| INV-AGT-004 | Execute command via battleye RCON | agent/internal/console | agent/internal/rcon/battleye.go:1 | [p](procedures/agent.md#console-battleye) | none | untested |  | [e](evidence/INV-AGT-004/) |  |  |
| INV-AGT-005 | Execute command via satisfactory RCON | agent/internal/console | agent/internal/rcon/satisfactory.go:1 | [p](procedures/agent.md#console-satisfactory) | none | untested |  | [e](evidence/INV-AGT-005/) |  |  |
| INV-AGT-006 | Execute command via palworld RCON | agent/internal/console | agent/internal/rcon/palworld.go:1 | [p](procedures/agent.md#console-palworld) | none | untested |  | [e](evidence/INV-AGT-006/) |  |  |
| INV-AGT-007 | Execute command via nuclearoption RCON | agent/internal/console | agent/internal/rcon/nuclearoption.go:1 | [p](procedures/agent.md#console-nuclearoption) | none | untested |  | [e](evidence/INV-AGT-007/) |  |  |
| INV-AGT-008 | Execute command via REST RCON | agent/internal/console | agent/internal/rcon/rest.go:1 | [p](procedures/agent.md#console-rest) | none | untested |  | [e](evidence/INV-AGT-008/) |  |  |
| INV-AGT-009 | Execute command via PTY console | agent/internal/console | api/internal/ws/attach.go:41-46 | [p](procedures/agent.md#console-pty) | none | untested |  | [e](evidence/INV-AGT-009/) |  |  |
| INV-AGT-010 | List files and directories | agent/internal/files | agent/internal/files/files.go:37 | [p](procedures/agent.md#files-list) | none | untested |  | [e](evidence/INV-AGT-010/) |  |  |
| INV-AGT-011 | Read file contents | agent/internal/files | agent/internal/files/files.go:38 | [p](procedures/agent.md#files-read) | none | untested |  | [e](evidence/INV-AGT-011/) |  |  |
| INV-AGT-012 | Write or overwrite file | agent/internal/files | agent/internal/files/files.go:40 | [p](procedures/agent.md#files-write) | none | untested |  | [e](evidence/INV-AGT-012/) |  |  |
| INV-AGT-013 | Upload file to server | agent/internal/files | agent/internal/files/files.go:41 | [p](procedures/agent.md#files-upload) | TestAPI_AgentFilesRoundTrip | untested |  | [e](evidence/INV-AGT-013/) |  |  |
| INV-AGT-014 | Download file from server | agent/internal/files | agent/internal/files/files.go:39 | [p](procedures/agent.md#files-download) | TestAPI_AgentFilesRoundTrip | untested |  | [e](evidence/INV-AGT-014/) |  |  |
| INV-AGT-015 | Create directory | agent/internal/files | agent/internal/files/files.go:42 | [p](procedures/agent.md#files-mkdir) | none | untested |  | [e](evidence/INV-AGT-015/) |  |  |
| INV-AGT-016 | Delete file or directory | agent/internal/files | agent/internal/files/files.go:43 | [p](procedures/agent.md#files-delete) | TestAPI_AgentFilesRoundTrip | untested |  | [e](evidence/INV-AGT-016/) |  |  |
| INV-AGT-017 | Stream log file via WebSocket | agent/internal/logs | agent/internal/logs/logs.go:31 | [p](procedures/agent.md#logs-tail) | none | untested |  | [e](evidence/INV-AGT-017/) |  |  |
| INV-AGT-018 | Download complete log file | agent/internal/logs | agent/internal/logs/logs.go:32 | [p](procedures/agent.md#logs-download) | none | untested |  | [e](evidence/INV-AGT-018/) |  |  |
| INV-AGT-019 | List online players | agent/internal/players | agent/internal/players/players.go:97 | [p](procedures/agent.md#players-list) | none | untested |  | [e](evidence/INV-AGT-019/) |  |  |
| INV-AGT-020 | List banned players | agent/internal/players | agent/internal/players/players.go:98 | [p](procedures/agent.md#players-banned) | none | untested |  | [e](evidence/INV-AGT-020/) |  |  |
| INV-AGT-021 | Kick player from server | agent/internal/players | agent/internal/players/players.go:99 | [p](procedures/agent.md#players-kick) | TestAPI_AgentPlayers | untested |  | [e](evidence/INV-AGT-021/) |  |  |
| INV-AGT-022 | Ban player from server | agent/internal/players | agent/internal/players/players.go:100 | [p](procedures/agent.md#players-ban) | TestAPI_AgentPlayers | untested |  | [e](evidence/INV-AGT-022/) |  |  |
| INV-AGT-023 | Unban player from server | agent/internal/players | agent/internal/players/players.go:101 | [p](procedures/agent.md#players-unban) | TestAPI_AgentPlayers | untested |  | [e](evidence/INV-AGT-023/) |  |  |
| INV-AGT-024 | List whitelisted players | agent/internal/players | agent/internal/players/players.go:102 | [p](procedures/agent.md#players-whitelist) | none | untested |  | [e](evidence/INV-AGT-024/) |  |  |
| INV-AGT-025 | Add player to whitelist | agent/internal/players | agent/internal/players/players.go:103 | [p](procedures/agent.md#players-whitelist-add) | TestAPI_AgentPlayers | untested |  | [e](evidence/INV-AGT-025/) |  |  |
| INV-AGT-026 | Remove player from whitelist | agent/internal/players | agent/internal/players/players.go:104 | [p](procedures/agent.md#players-whitelist-remove) | TestAPI_AgentPlayers | untested |  | [e](evidence/INV-AGT-026/) |  |  |
| INV-AGT-027 | Pause game writes (quiesce) | agent/internal/quiesce | agent/internal/quiesce/quiesce.go:62 | [p](procedures/agent.md#quiesce-pause) | none | untested |  | [e](evidence/INV-AGT-027/) |  |  |
| INV-AGT-028 | Resume game writes (unquiesce) | agent/internal/quiesce | agent/internal/quiesce/quiesce.go:74 | [p](procedures/agent.md#quiesce-resume) | none | untested |  | [e](evidence/INV-AGT-028/) |  |  |
| INV-AGT-029 | Execute stop sequence | agent/internal/lifecycle | agent/internal/lifecycle/lifecycle.go:56 | [p](procedures/agent.md#lifecycle-stop) | none | untested |  | [e](evidence/INV-AGT-029/) |  |  |
| INV-AGT-030 | Run module-declared action | agent/internal/actions | agent/internal/actions/actions.go:53 | [p](procedures/agent.md#actions-run) | none | untested |  | [e](evidence/INV-AGT-030/) |  |  |
| INV-AGT-031 | Retrieve live status metrics | agent/internal/status | agent/internal/status/status.go:68 | [p](procedures/agent.md#status-metrics) | none | untested |  | [e](evidence/INV-AGT-031/) |  |  |
| INV-AGT-032 | List installed mods | agent/internal/mods | agent/internal/mods/mods.go:72 | [p](procedures/agent.md#mods-list) | TestAPI_ModManifestInstallUpgrade | untested |  | [e](evidence/INV-AGT-032/) |  |  |
| INV-AGT-033 | Install mod from URL | agent/internal/mods | agent/internal/mods/mods.go:73 | [p](procedures/agent.md#mods-install) | TestAPI_ModManifestInstallUpgrade, TestAPI_ModUpload | untested |  | [e](evidence/INV-AGT-033/) |  |  |
| INV-AGT-034 | Upload mod file | agent/internal/mods | agent/internal/mods/mods.go:74 | [p](procedures/agent.md#mods-upload) | none | untested |  | [e](evidence/INV-AGT-034/) |  |  |
| INV-AGT-035 | Remove installed mod | agent/internal/mods | agent/internal/mods/mods.go:75 | [p](procedures/agent.md#mods-remove) | TestAPI_ModManifestInstallUpgrade | untested |  | [e](evidence/INV-AGT-035/) |  |  |
| INV-AGT-036 | Report metrics via Prometheus | agent/internal/heartbeat | agent/cmd/main.go:199 | [p](procedures/agent.md#heartbeat-metrics) | none | untested |  | [e](evidence/INV-AGT-036/) |  |  |
| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|

## AUX
| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|
| INV-AUX-001 | Detect join vs status on Minecraft (wakeProtocol=minecraft) | sentinel/ | sentinel/main.go:41-58 | [p](procedures/aux.md#sentinel-minecraft-status-ping) | none | untested |  | [e](INV-AUX-001/) |  |  |
| INV-AUX-002 | Detect Minecraft join handshake and patch wake annotation | sentinel/ | sentinel/main.go:41-58 | [p](procedures/aux.md#sentinel-minecraft-join-wake) | none | untested |  | [e](INV-AUX-002/) |  |  |
| INV-AUX-003 | Detect Terraria join attempt via gameproto classifier | sentinel/ | sentinel/main.go:41-58, gameproto/terraria.go | [p](procedures/aux.md#sentinel-terraria-generic-udp) | none | untested |  | [e](INV-AUX-003/) |  |  |
| INV-AUX-004 | UDP packet-counting heuristic for generic wake detection | sentinel/ | sentinel/main.go (UDP heuristic) | [p](procedures/aux.md#sentinel-terraria-generic-udp) | none | untested |  | [e](INV-AUX-004/) |  |  |
| INV-AUX-005 | Start network packet capture with BPF filter | capture-sidecar/ | capture-sidecar/cmd/main.go, capture-sidecar/internal/capture/afpacket.go | [p](procedures/aux.md#capture-sidecar-start-stop) | TestGameServer_NetworkCaptureStartStopDownload | untested |  | [e](INV-AUX-005/) |  |  |
| INV-AUX-006 | Stop capture and finalize PCAPNG file | capture-sidecar/ | capture-sidecar/internal/capture/writer.go | [p](procedures/aux.md#capture-sidecar-start-stop) | none | untested |  | [e](INV-AUX-006/) |  |  |
| INV-AUX-007 | Download completed capture file via mTLS HTTP endpoint | capture-sidecar/ | capture-sidecar/internal/httpserver/handlers.go | [p](procedures/aux.md#capture-sidecar-start-stop) | none | untested |  | [e](INV-AUX-007/) |  |  |
| INV-AUX-008 | Enforce capture size and duration limits | capture-sidecar/ | capture-sidecar/internal/capture/writer.go | [p](procedures/aux.md#capture-sidecar-start-stop) | none | untested |  | [e](INV-AUX-008/) |  |  |
| INV-AUX-009 | Supervise frp (forward proxy) relay client | tunnel/ | tunnel/main.go:1-18, charts/gameplane/values.yaml:85 | [p](procedures/aux.md#tunnel-frp-blocked-candidate) | none | blocked | blocked candidate: needs external frp server; alternative: test operator tunnel injection via envtest (api-agent bucket) | [e](INV-AUX-009/) |  |  |
| INV-AUX-010 | Supervise Tailscale relay client | tunnel/ | tunnel/main.go:1-18, charts/gameplane/values.yaml:86 | [p](procedures/aux.md#tunnel-tailscale-blocked-candidate) | none | blocked | blocked candidate: needs Tailscale account and auth key; alternative: test operator injection | [e](INV-AUX-010/) |  |  |
| INV-AUX-011 | Supervise playit relay client | tunnel/ | tunnel/main.go:1-18, charts/gameplane/values.yaml:87 | [p](procedures/aux.md#tunnel-playit-blocked-candidate) | none | blocked | blocked candidate: needs playit.gg account and secret key; alternative: test operator injection | [e](INV-AUX-011/) |  |  |
| INV-AUX-012 | Accept HTTP audit events and relay to syslog collector | audit-syslog-bridge/ | audit-syslog-bridge/README.md, audit-syslog-bridge/main.go:1-30, charts/gameplane/values.yaml:183 | [p](procedures/aux.md#audit-syslog-bridge-blocked-candidate) | none | blocked | blocked candidate: needs external RFC 5424 syslog receiver; alternative: test with local netcat listener (api-auth bucket) | [e](INV-AUX-012/) |  |  |
| INV-AUX-013 | Format and send RFC 5424 syslog records | audit-syslog-bridge/ | audit-syslog-bridge/main.go | [p](procedures/aux.md#audit-syslog-bridge-blocked-candidate) | none | blocked | blocked candidate: needs external syslog collector | [e](INV-AUX-013/) |  |  |
| INV-AUX-014 | Accept and aggregate anonymous usage telemetry reports | telemetry-receiver/ | telemetry-receiver/main.go, charts/gameplane/values.yaml:235 | [p](procedures/aux.md#telemetry-receiver-opt-in) | none | untested |  | [e](INV-AUX-014/) |  |  |
| INV-AUX-015 | Expose aggregated telemetry metrics on /metrics endpoint | telemetry-receiver/ | telemetry-receiver/main.go | [p](procedures/aux.md#telemetry-receiver-opt-in) | none | untested |  | [e](INV-AUX-015/) |  |  |
| INV-AUX-016 | Provide read-only Model Context Protocol (MCP) interface | mcp-server/ | mcp-server/README.md, mcp-server/main.go, charts/gameplane/values.yaml:400 | [p](procedures/aux.md#mcp-server-read-only-check) | none | untested |  | [e](INV-AUX-016/) |  |  |
| INV-AUX-017 | List Gameplane CRDs (GameServers, GameTemplates, etc.) via MCP | mcp-server/ | mcp-server/tools.go | [p](procedures/aux.md#mcp-server-read-only-check) | none | untested |  | [e](INV-AUX-017/) |  |  |
| INV-AUX-018 | Structurally enforce read-only access (no mutating methods) | mcp-server/ | mcp-server/main.go, mcp-server/internal/kube/client.go | [p](procedures/aux.md#mcp-server-read-only-check) | none | untested |  | [e](INV-AUX-018/) |  |  |
| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|

## HELM
| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|
| INV-HELM-001 | Enable OIDC authentication | api/ | charts/gameplane/values.yaml:240 | [p](procedures/helm.md#oidc-authentication) | none | untested | blocked candidate: needs external OIDC IdP; alternative: test with mock IdP stub | [e](INV-HELM-001/) |  |  |
| INV-HELM-002 | Configure audit webhook sink | api/ | charts/gameplane/values.yaml:160 | [p](procedures/helm.md#audit-webhook-sink) | none | untested |  | [e](INV-HELM-002/) |  |  |
| INV-HELM-003 | Enable syslog bridge for audit events | audit-syslog-bridge/ | charts/gameplane/values.yaml:183 | [p](procedures/helm.md#syslog-bridge-audit) | none | untested |  | [e](INV-HELM-003/) |  |  |
| INV-HELM-004 | Configure S3 audit sink | api/ | charts/gameplane/values.yaml:202 | [p](procedures/helm.md#s3-audit-sink) | none | untested | blocked candidate: needs S3-compatible endpoint; alternative: mock MinIO if available on kubelab | [e](INV-HELM-004/) |  |  |
| INV-HELM-005 | Enable audit event stdout logging | api/ | charts/gameplane/values.yaml:153 | [p](procedures/helm.md#audit-stdout-logging) | none | untested |  | [e](INV-HELM-005/) |  |  |
| INV-HELM-006 | Configure telemetry collection endpoint | api/ | charts/gameplane/values.yaml:222 | [p](procedures/helm.md#telemetry-collection) | none | untested | blocked candidate: telemetry sent daily; requires deterministic triggers or extended wait | [e](INV-HELM-006/) |  |  |
| INV-HELM-007 | Enable bundled telemetry receiver | telemetry-receiver/ | charts/gameplane/values.yaml:235 | [p](procedures/helm.md#telemetry-receiver-bundled) | none | untested |  | [e](INV-HELM-007/) |  |  |
| INV-HELM-008 | Enable cluster operations feature | api/ | charts/gameplane/values.yaml:392 | [p](procedures/helm.md#cluster-operations-feature) | none | untested |  | [e](INV-HELM-008/) |  |  |
| INV-HELM-009 | Enable MCP (Model Context Protocol) server | mcp-server/ | charts/gameplane/values.yaml:400 | [p](procedures/helm.md#mcp-server-deployment) | none | untested |  | [e](INV-HELM-009/) |  |  |
| INV-HELM-010 | Enable default-deny network policies | operator/ | charts/gameplane/values.yaml:316 | [p](procedures/helm.md#network-policies-enforcement) | none | untested |  | [e](INV-HELM-010/) |  |  |
| INV-HELM-011 | Enforce restricted pod security policy | operator/ | charts/gameplane/values.yaml:440 | [p](procedures/helm.md#pod-security-enforcement) | none | untested |  | [e](INV-HELM-011/) |  |  |
| INV-HELM-012 | Enable game pod egress to public internet | operator/ | charts/gameplane/values.yaml:343 | [p](procedures/helm.md#game-egress-policies) | none | untested | blocked candidate: requires GameServer pod startup for network testing | [e](INV-HELM-012/) |  |  |
| INV-HELM-013 | Enable per-GameServer ingress network policies | operator/ | charts/gameplane/values.yaml:371 | [p](procedures/helm.md#game-ingress-policies) | none | untested | blocked candidate: requires GameServer pod startup and external connectivity | [e](INV-HELM-013/) |  |  |
| INV-HELM-014 | Create Prometheus ServiceMonitor objects | charts/gameplane/ | charts/gameplane/values.yaml:418 | [p](procedures/helm.md#service-monitors) | none | untested | blocked candidate: needs Prometheus Operator CRDs (ServiceMonitor); alternative: verify on cluster with CRDs | [e](INV-HELM-014/) |  |  |
| INV-HELM-015 | Create Prometheus alert rules | charts/gameplane/ | charts/gameplane/values.yaml:428 | [p](procedures/helm.md#prometheus-rules) | none | untested | blocked candidate: needs Prometheus Operator CRDs (PrometheusRule); alternative: verify on cluster with CRDs | [e](INV-HELM-015/) |  |  |
| INV-HELM-016 | Create Grafana dashboard ConfigMap | charts/gameplane/ | charts/gameplane/values.yaml:435 | [p](procedures/helm.md#grafana-dashboards) | none | untested | blocked candidate: needs Grafana with sidecar loader; alternative: verify on Grafana-equipped cluster | [e](INV-HELM-016/) |  |  |
| INV-HELM-017 | Enable default module source | operator/ | charts/gameplane/values.yaml:446 | [p](procedures/helm.md#default-module-source) | none | untested |  | [e](INV-HELM-017/) |  |  |
| INV-HELM-018 | Enable module bundle signature verification | operator/ | charts/gameplane/values.yaml:507 | [p](procedures/helm.md#module-signature-verification) | none | untested | blocked candidate: requires pulling and validating module bundles; alternative: verify ModuleSource spec includes verify settings | [e](INV-HELM-018/) |  |  |
| INV-HELM-019 | Enable dashboard module upload source | operator/ | charts/gameplane/values.yaml:518 | [p](procedures/helm.md#upload-module-source) | none | untested |  | [e](INV-HELM-019/) |  |  |
| INV-HELM-020 | Enable packet capture sidecar for GameServers | agent/ | charts/gameplane/values.yaml:527 | [p](procedures/helm.md#packet-capture-sidecar) | none | untested | blocked candidate: requires GameServer creation and sidecar injection verification | [e](INV-HELM-020/) |  |  |
| INV-HELM-021 | Enable/disable web dashboard UI | web/ | charts/gameplane/values.yaml:289 | [p](procedures/helm.md#web-dashboard-ui) | none | untested |  | [e](INV-HELM-021/) |  |  |
| INV-HELM-022 | Configure dashboard ingress | charts/gameplane/ | charts/gameplane/values.yaml:296 | [p](procedures/helm.md#ingress-configuration) | none | untested |  | [e](INV-HELM-022/) |  |  |
| INV-HELM-023 | Use existing storage claim for API database | api/ | charts/gameplane/values.yaml:142 | [p](procedures/helm.md#existing-storage-claim) | none | untested |  | [e](INV-HELM-023/) |  |  |
| INV-HELM-024 | Configure default storage class for GameServer data | operator/ | charts/gameplane/values.yaml:109 | [p](procedures/helm.md#game-storage-class) | none | untested | blocked candidate: requires GameServer creation and PVC provisioning | [e](INV-HELM-024/) |  |  |
| INV-HELM-025 | Enable automatic CRD upgrade hook | charts/gameplane/ | charts/gameplane/values.yaml:34 | [p](procedures/helm.md#crd-auto-apply-hook) | none | untested |  | [e](INV-HELM-025/) |  |  |
| INV-HELM-026 | Override container image registry | charts/gameplane/ | charts/gameplane/values.yaml:17 | [p](procedures/helm.md#image-registry-override) | none | untested | blocked candidate: needs alternative registry accessible from cluster | [e](INV-HELM-026/) |  |  |
| INV-HELM-027 | Override container image tag | charts/gameplane/ | charts/gameplane/values.yaml:20 | [p](procedures/helm.md#image-tag-override) | none | untested |  | [e](INV-HELM-027/) |  |  |
| INV-HELM-028 | Enable operator leader election (HA) | operator/ | charts/gameplane/values.yaml:53 | [p](procedures/helm.md#operator-leader-election) | none | untested |  | [e](INV-HELM-028/) |  |  |
| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|

## MOD
| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|
| INV-MOD-001 | Create and start Gmod server from template | modules/ | audit/evidence/rc.0/module-categories.md:48 | [p](procedures/modules.md#garrys-mod) | none | untested |  | [e](evidence/INV-MOD-001/) |  |  |
| INV-MOD-002 | Join Gmod server via A2S query | modules/ | audit/evidence/rc.0/module-categories.md:48 | [p](procedures/modules.md#garrys-mod) | none | untested |  | [e](evidence/INV-MOD-002/) |  |  |
| INV-MOD-003 | Gmod server console unavailable (none family) | modules/ | audit/evidence/rc.0/module-categories.md:48 | [p](procedures/modules.md#garrys-mod) | none | untested |  | [e](evidence/INV-MOD-003/) |  |  |
| INV-MOD-004 | Backup and restore Gmod server | modules/ | audit/evidence/rc.0/module-categories.md:48 | [p](procedures/modules.md#garrys-mod) | none | untested |  | [e](evidence/INV-MOD-004/) |  |  |
| INV-MOD-005 | Create and start Farming Simulator 25 server from template | modules/ | audit/evidence/rc.0/module-categories.md:49 | [p](procedures/modules.md#farming-simulator-25) | none | untested |  | [e](evidence/INV-MOD-005/) |  |  |
| INV-MOD-006 | Join Farming Simulator 25 server via HTTP REST | modules/ | audit/evidence/rc.0/module-categories.md:49 | [p](procedures/modules.md#farming-simulator-25) | none | untested |  | [e](evidence/INV-MOD-006/) |  |  |
| INV-MOD-007 | Farming Simulator 25 server REST console query | modules/ | audit/evidence/rc.0/module-categories.md:49 | [p](procedures/modules.md#farming-simulator-25) | none | untested |  | [e](evidence/INV-MOD-007/) |  |  |
| INV-MOD-008 | Backup and restore Farming Simulator 25 server | modules/ | audit/evidence/rc.0/module-categories.md:49 | [p](procedures/modules.md#farming-simulator-25) | none | untested |  | [e](evidence/INV-MOD-008/) |  |  |
| INV-MOD-009 | Create and start BeamMP server from template | modules/ | audit/evidence/rc.0/module-categories.md:50 | [p](procedures/modules.md#beammp) | none | untested |  | [e](evidence/INV-MOD-009/) |  |  |
| INV-MOD-010 | Join BeamMP server via TCP | modules/ | audit/evidence/rc.0/module-categories.md:50 | [p](procedures/modules.md#beammp) | none | untested |  | [e](evidence/INV-MOD-010/) |  |  |
| INV-MOD-011 | BeamMP server PTY console access | modules/ | audit/evidence/rc.0/module-categories.md:50 | [p](procedures/modules.md#beammp) | none | untested |  | [e](evidence/INV-MOD-011/) |  |  |
| INV-MOD-012 | Backup and restore BeamMP server | modules/ | audit/evidence/rc.0/module-categories.md:50 | [p](procedures/modules.md#beammp) | none | untested |  | [e](evidence/INV-MOD-012/) |  |  |
| INV-MOD-013 | Create and start Valheim server from template | modules/ | audit/evidence/rc.0/module-categories.md:51 | [p](procedures/modules.md#valheim) | none | untested |  | [e](evidence/INV-MOD-013/) |  |  |
| INV-MOD-014 | Join Valheim server via HTTP REST | modules/ | audit/evidence/rc.0/module-categories.md:51 | [p](procedures/modules.md#valheim) | none | untested |  | [e](evidence/INV-MOD-014/) |  |  |
| INV-MOD-015 | Valheim server PTY console access | modules/ | audit/evidence/rc.0/module-categories.md:51 | [p](procedures/modules.md#valheim) | none | untested |  | [e](evidence/INV-MOD-015/) |  |  |
| INV-MOD-016 | Backup and restore Valheim server | modules/ | audit/evidence/rc.0/module-categories.md:51 | [p](procedures/modules.md#valheim) | none | untested |  | [e](evidence/INV-MOD-016/) |  |  |
| INV-MOD-017 | Create and start Don't Starve Together server from template | modules/ | audit/evidence/rc.0/module-categories.md:52 | [p](procedures/modules.md#dont-starve-together) | none | untested |  | [e](evidence/INV-MOD-017/) |  |  |
| INV-MOD-018 | Join Don't Starve Together server via A2S query | modules/ | audit/evidence/rc.0/module-categories.md:52 | [p](procedures/modules.md#dont-starve-together) | none | untested |  | [e](evidence/INV-MOD-018/) |  |  |
| INV-MOD-019 | Don't Starve Together server PTY console access | modules/ | audit/evidence/rc.0/module-categories.md:52 | [p](procedures/modules.md#dont-starve-together) | none | untested |  | [e](evidence/INV-MOD-019/) |  |  |
| INV-MOD-020 | Backup and restore Don't Starve Together server | modules/ | audit/evidence/rc.0/module-categories.md:52 | [p](procedures/modules.md#dont-starve-together) | none | untested |  | [e](evidence/INV-MOD-020/) |  |  |
| INV-MOD-021 | Create and start tModLoader server from template | modules/ | audit/evidence/rc.0/module-categories.md:53 | [p](procedures/modules.md#tmodloader) | none | untested |  | [e](evidence/INV-MOD-021/) |  |  |
| INV-MOD-022 | Join tModLoader server via Terraria TCP probe | modules/ | audit/evidence/rc.0/module-categories.md:53 | [p](procedures/modules.md#tmodloader) | none | untested |  | [e](evidence/INV-MOD-022/) |  |  |
| INV-MOD-023 | tModLoader server PTY console access | modules/ | audit/evidence/rc.0/module-categories.md:53 | [p](procedures/modules.md#tmodloader) | none | untested |  | [e](evidence/INV-MOD-023/) |  |  |
| INV-MOD-024 | Backup and restore tModLoader server | modules/ | audit/evidence/rc.0/module-categories.md:53 | [p](procedures/modules.md#tmodloader) | none | untested |  | [e](evidence/INV-MOD-024/) |  |  |
| INV-MOD-025 | Create and start Terraria server from template | modules/ | audit/evidence/rc.0/module-categories.md:54 | [p](procedures/modules.md#terraria) | none | untested |  | [e](evidence/INV-MOD-025/) |  |  |
| INV-MOD-026 | Join Terraria server via gameproto wire protocol | modules/ | audit/evidence/rc.0/module-categories.md:54 | [p](procedures/modules.md#terraria) | none | untested |  | [e](evidence/INV-MOD-026/) |  |  |
| INV-MOD-027 | Terraria server PTY console access | modules/ | audit/evidence/rc.0/module-categories.md:54 | [p](procedures/modules.md#terraria) | none | untested |  | [e](evidence/INV-MOD-027/) |  |  |
| INV-MOD-028 | Backup and restore Terraria server | modules/ | audit/evidence/rc.0/module-categories.md:54 | [p](procedures/modules.md#terraria) | none | untested |  | [e](evidence/INV-MOD-028/) |  |  |
| INV-MOD-029 | Create and start Factorio server from template | modules/ | audit/evidence/rc.0/module-categories.md:55 | [p](procedures/modules.md#factorio) | none | untested |  | [e](evidence/INV-MOD-029/) |  |  |
| INV-MOD-030 | Join Factorio server via TCP | modules/ | audit/evidence/rc.0/module-categories.md:55 | [p](procedures/modules.md#factorio) | none | untested |  | [e](evidence/INV-MOD-030/) |  |  |
| INV-MOD-031 | Factorio server PTY + Source RCON console | modules/ | audit/evidence/rc.0/module-categories.md:55 | [p](procedures/modules.md#factorio) | none | untested |  | [e](evidence/INV-MOD-031/) |  |  |
| INV-MOD-032 | Backup and restore Factorio server | modules/ | audit/evidence/rc.0/module-categories.md:55 | [p](procedures/modules.md#factorio) | none | untested |  | [e](evidence/INV-MOD-032/) |  |  |
| INV-MOD-033 | Create and start DayZ server from template | modules/ | audit/evidence/rc.0/module-categories.md:56 | [p](procedures/modules.md#dayz) | none | blocked | blocked candidate: needs node memory ≥6Gi per pod; no lighter alternative available in rcon/battleye category (DayZ is the only module) | [e](evidence/INV-MOD-033/) |  |  |
| INV-MOD-034 | Join DayZ server via A2S query | modules/ | audit/evidence/rc.0/module-categories.md:56 | [p](procedures/modules.md#dayz) | none | blocked | blocked candidate: needs node memory ≥6Gi per pod; no lighter alternative available in rcon/battleye category (DayZ is the only module) | [e](evidence/INV-MOD-034/) |  |  |
| INV-MOD-035 | DayZ server BattlEye RCON console | modules/ | audit/evidence/rc.0/module-categories.md:56 | [p](procedures/modules.md#dayz) | none | blocked | blocked candidate: needs node memory ≥6Gi per pod; no lighter alternative available in rcon/battleye category (DayZ is the only module) | [e](evidence/INV-MOD-035/) |  |  |
| INV-MOD-036 | Backup and restore DayZ server | modules/ | audit/evidence/rc.0/module-categories.md:56 | [p](procedures/modules.md#dayz) | none | blocked | blocked candidate: needs node memory ≥6Gi per pod; no lighter alternative available in rcon/battleye category (DayZ is the only module) | [e](evidence/INV-MOD-036/) |  |  |
| INV-MOD-037 | Create and start Nuclear Option server from template | modules/ | audit/evidence/rc.0/module-categories.md:57 | [p](procedures/modules.md#nuclear-option) | none | untested |  | [e](evidence/INV-MOD-037/) |  |  |
| INV-MOD-038 | Join Nuclear Option server via UDP (manual verification) | modules/ | audit/evidence/rc.0/module-categories.md:57 | [p](procedures/modules.md#nuclear-option) | none | untested | join probe not automated; manual verification required | [e](evidence/INV-MOD-038/) |  |  |
| INV-MOD-039 | Nuclear Option server nuclearoption RCON console | modules/ | audit/evidence/rc.0/module-categories.md:57 | [p](procedures/modules.md#nuclear-option) | none | untested |  | [e](evidence/INV-MOD-039/) |  |  |
| INV-MOD-040 | Backup and restore Nuclear Option server | modules/ | audit/evidence/rc.0/module-categories.md:57 | [p](procedures/modules.md#nuclear-option) | none | untested |  | [e](evidence/INV-MOD-040/) |  |  |
| INV-MOD-041 | Create and start Palworld server from template | modules/ | audit/evidence/rc.0/module-categories.md:58 | [p](procedures/modules.md#palworld) | none | untested |  | [e](evidence/INV-MOD-041/) |  |  |
| INV-MOD-042 | Join Palworld server via A2S query | modules/ | audit/evidence/rc.0/module-categories.md:58 | [p](procedures/modules.md#palworld) | none | untested |  | [e](evidence/INV-MOD-042/) |  |  |
| INV-MOD-043 | Palworld server palworld RCON REST API console | modules/ | audit/evidence/rc.0/module-categories.md:58 | [p](procedures/modules.md#palworld) | none | untested |  | [e](evidence/INV-MOD-043/) |  |  |
| INV-MOD-044 | Backup and restore Palworld server | modules/ | audit/evidence/rc.0/module-categories.md:58 | [p](procedures/modules.md#palworld) | none | untested |  | [e](evidence/INV-MOD-044/) |  |  |
| INV-MOD-045 | Create and start FiveM server from template | modules/ | audit/evidence/rc.0/module-categories.md:59 | [p](procedures/modules.md#fivem) | none | untested |  | [e](evidence/INV-MOD-045/) |  |  |
| INV-MOD-046 | Join FiveM server via HTTP REST | modules/ | audit/evidence/rc.0/module-categories.md:59 | [p](procedures/modules.md#fivem) | none | untested |  | [e](evidence/INV-MOD-046/) |  |  |
| INV-MOD-047 | FiveM server REST RCON console | modules/ | audit/evidence/rc.0/module-categories.md:59 | [p](procedures/modules.md#fivem) | none | untested |  | [e](evidence/INV-MOD-047/) |  |  |
| INV-MOD-048 | Backup and restore FiveM server | modules/ | audit/evidence/rc.0/module-categories.md:59 | [p](procedures/modules.md#fivem) | none | untested |  | [e](evidence/INV-MOD-048/) |  |  |
| INV-MOD-049 | Create and start Satisfactory server from template | modules/ | audit/evidence/rc.0/module-categories.md:60 | [p](procedures/modules.md#satisfactory) | none | blocked | blocked candidate: needs node memory ≥6Gi per pod; alternative: FiveM (rcon/rest + HTTP REST probe, 4Gi) | [e](evidence/INV-MOD-049/) |  |  |
| INV-MOD-050 | Join Satisfactory server via HTTP REST | modules/ | audit/evidence/rc.0/module-categories.md:60 | [p](procedures/modules.md#satisfactory) | none | blocked | blocked candidate: needs node memory ≥6Gi per pod; alternative: FiveM (rcon/rest + HTTP REST probe, 4Gi) | [e](evidence/INV-MOD-050/) |  |  |
| INV-MOD-051 | Satisfactory server satisfactory RCON console | modules/ | audit/evidence/rc.0/module-categories.md:60 | [p](procedures/modules.md#satisfactory) | none | blocked | blocked candidate: needs node memory ≥6Gi per pod; alternative: FiveM (rcon/rest + HTTP REST probe, 4Gi) | [e](evidence/INV-MOD-051/) |  |  |
| INV-MOD-052 | Backup and restore Satisfactory server | modules/ | audit/evidence/rc.0/module-categories.md:60 | [p](procedures/modules.md#satisfactory) | none | blocked | blocked candidate: needs node memory ≥6Gi per pod; alternative: FiveM (rcon/rest + HTTP REST probe, 4Gi) | [e](evidence/INV-MOD-052/) |  |  |
| INV-MOD-053 | Create and start Ark Survival Ascended server from template | modules/ | audit/evidence/rc.0/module-categories.md:61 | [p](procedures/modules.md#ark-survival-ascended) | none | blocked | blocked candidate: needs node memory ≥10Gi per pod; alternative: CS2 (rcon/source + Steam A2S probe, 2Gi) | [e](evidence/INV-MOD-053/) |  |  |
| INV-MOD-054 | Join Ark Survival Ascended server via TCP RCON probe | modules/ | audit/evidence/rc.0/module-categories.md:61 | [p](procedures/modules.md#ark-survival-ascended) | none | blocked | blocked candidate: needs node memory ≥10Gi per pod; alternative: CS2 (rcon/source + Steam A2S probe, 2Gi) | [e](evidence/INV-MOD-054/) |  |  |
| INV-MOD-055 | Ark Survival Ascended server Source RCON console | modules/ | audit/evidence/rc.0/module-categories.md:61 | [p](procedures/modules.md#ark-survival-ascended) | none | blocked | blocked candidate: needs node memory ≥10Gi per pod; alternative: CS2 (rcon/source + Steam A2S probe, 2Gi) | [e](evidence/INV-MOD-055/) |  |  |
| INV-MOD-056 | Backup and restore Ark Survival Ascended server | modules/ | audit/evidence/rc.0/module-categories.md:61 | [p](procedures/modules.md#ark-survival-ascended) | none | blocked | blocked candidate: needs node memory ≥10Gi per pod; alternative: CS2 (rcon/source + Steam A2S probe, 2Gi) | [e](evidence/INV-MOD-056/) |  |  |
| INV-MOD-057 | Create and start CS2 server from template | modules/ | audit/evidence/rc.0/module-categories.md:62 | [p](procedures/modules.md#cs2) | none | untested |  | [e](evidence/INV-MOD-057/) |  |  |
| INV-MOD-058 | Join CS2 server via A2S query | modules/ | audit/evidence/rc.0/module-categories.md:62 | [p](procedures/modules.md#cs2) | none | untested |  | [e](evidence/INV-MOD-058/) |  |  |
| INV-MOD-059 | CS2 server Source RCON console | modules/ | audit/evidence/rc.0/module-categories.md:62 | [p](procedures/modules.md#cs2) | none | untested |  | [e](evidence/INV-MOD-059/) |  |  |
| INV-MOD-060 | Backup and restore CS2 server | modules/ | audit/evidence/rc.0/module-categories.md:62 | [p](procedures/modules.md#cs2) | none | untested |  | [e](evidence/INV-MOD-060/) |  |  |
| INV-MOD-061 | Create and start Minecraft Java server from template | modules/ | audit/evidence/rc.0/module-categories.md:63 | [p](procedures/modules.md#minecraft-java) | none | untested |  | [e](evidence/INV-MOD-061/) |  |  |
| INV-MOD-062 | Join Minecraft Java server via gameproto wire protocol | modules/ | audit/evidence/rc.0/module-categories.md:63 | [p](procedures/modules.md#minecraft-java) | none | untested |  | [e](evidence/INV-MOD-062/) |  |  |
| INV-MOD-063 | Minecraft Java server Source RCON console | modules/ | audit/evidence/rc.0/module-categories.md:63 | [p](procedures/modules.md#minecraft-java) | none | untested |  | [e](evidence/INV-MOD-063/) |  |  |
| INV-MOD-064 | Backup and restore Minecraft Java server | modules/ | audit/evidence/rc.0/module-categories.md:63 | [p](procedures/modules.md#minecraft-java) | none | untested |  | [e](evidence/INV-MOD-064/) |  |  |
| INV-MOD-065 | Create and start Rust server from template | modules/ | audit/evidence/rc.0/module-categories.md:64 | [p](procedures/modules.md#rust) | none | untested |  | [e](evidence/INV-MOD-065/) |  |  |
| INV-MOD-066 | Join Rust server via A2S query | modules/ | audit/evidence/rc.0/module-categories.md:64 | [p](procedures/modules.md#rust) | none | untested |  | [e](evidence/INV-MOD-066/) |  |  |
| INV-MOD-067 | Rust server WebSocket RCON console | modules/ | audit/evidence/rc.0/module-categories.md:64 | [p](procedures/modules.md#rust) | none | untested |  | [e](evidence/INV-MOD-067/) |  |  |
| INV-MOD-068 | Backup and restore Rust server | modules/ | audit/evidence/rc.0/module-categories.md:64 | [p](procedures/modules.md#rust) | none | untested |  | [e](evidence/INV-MOD-068/) |  |  |

## UPG
| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|
| INV-UPG-001 | Upgrade preserves game server state and data across beta.8 to RC transition | api/, operator/ | api/cmd/main.go:1, operator/cmd/main.go:1 | [p](procedures/upgrade.md#upgrade-to-rc) | none | untested |  | [e](evidence/INV-UPG-001/) |  |  |
| INV-UPG-002 | Admin login succeeds and audit Verify chain validates after upgrade | api/internal/auth/, api/internal/audit/ | api/internal/auth/local.go:105-173, api/internal/audit/audit.go:1 | [p](procedures/upgrade.md#upgrade-to-rc) | none | untested |  | [e](evidence/INV-UPG-002/) |  |  |
| INV-UPG-003 | API and operator restarts preserve reconciliation state and server data | operator/internal/controller/, api/internal/handlers/ | operator/internal/controller/gameserver_controller.go:1, api/cmd/main.go:1 | [p](procedures/upgrade.md#restart) | none | untested |  | [e](evidence/INV-UPG-003/) |  |  |
| INV-UPG-004 | Helm rollback from RC to beta.8 serves API requests and lists seeded server | api/internal/handlers/, charts/gameplane/ | api/cmd/main.go:1, charts/gameplane/Chart.yaml:1 | [p](procedures/upgrade.md#rollback) | none | untested |  | [e](evidence/INV-UPG-004/) |  |  |
| INV-UPG-005 | Real production database restores cleanly from snapshot with no schema drift | api/internal/db/ | api/internal/db/db.go:70-117 | [p](procedures/upgrade.md#restore-real-db) | none | untested |  | [e](evidence/INV-UPG-005/) |  |  |

## NODE
| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|
| INV-NODE-001 | Record baseline scheduling: audit018- GameServers and their node assignments | operator/ | operator/internal/controller/gameserver_controller.go:1 | [p](procedures/nodes.md#scheduling) | none | untested |  | [e](evidence/INV-NODE-001/) |  |  |
| INV-NODE-002 | Cordon node, evict audit018- pods via eviction API, operator reschedules on surviving nodes (pre-existing pods untouched) | operator/ | operator/internal/controller/gameserver_controller.go:1 | [p](procedures/nodes.md#drain) | none | untested |  | [e](evidence/INV-NODE-002/) |  |  |
| INV-NODE-003 | Stop k3s-agent on kubelab-worker-2 for 5 min (OD-017), operator recovers evicted audit018- pods and soak-pool-west returns Running with the same UID and PVC | operator/ | operator/internal/controller/gameserver_controller.go:1, operator/api/v1alpha1/gameserver_types.go:1 | [p](procedures/nodes.md#node-loss) | none | untested |  | [e](evidence/INV-NODE-003/) |  |  |

## SEC

The security-control rows (INV-SEC-001 to INV-SEC-016) are held off-git until the audit's security work is complete (OD-019).
