# Draft inventory rows: WEB (rc.0)

Links are relative to audit/inventory.md.

| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|
| INV-WEB-001 | Sign in with username and password | web/ | web/src/routes/Login.tsx:69-92 | [p](procedures/web.md#login-username-password) | TBD | untested | | [e](evidence/INV-WEB-001/) | | |
| INV-WEB-002 | Toggle password visibility on login form | web/ | web/src/routes/Login.tsx:172-186 | [p](procedures/web.md#login-toggle-password-visibility) | TBD | untested | | [e](evidence/INV-WEB-002/) | | |
| INV-WEB-003 | Sign in with SSO provider | web/ | web/src/routes/Login.tsx:295-312 | [p](procedures/web.md#login-sso-provider) | TBD | untested | | [e](evidence/INV-WEB-003/) | | |
| INV-WEB-004 | Sign in with safe mode (custom CSS disabled) | web/ | web/src/routes/Login.tsx:69-86 | [p](procedures/web.md#login-safe-mode) | TBD | untested | | [e](evidence/INV-WEB-004/) | | |
| INV-WEB-005 | Display password reset hint on login | web/ | web/src/routes/Login.tsx:151-159 | [p](procedures/web.md#login-password-forgotten-hint) | TBD | untested | | [e](evidence/INV-WEB-005/) | | |
| INV-WEB-006 | View fleet status card (running, stopped, failed servers) | web/ | web/src/routes/Dashboard.tsx:196-249 | [p](procedures/web.md#dashboard-view-fleet-summary) | TBD | untested | | [e](evidence/INV-WEB-006/) | | |
| INV-WEB-007 | View cluster resources metrics (CPU, memory, storage, nodes) | web/ | web/src/routes/Dashboard.tsx:296-350 | [p](procedures/web.md#dashboard-view-cluster-resources) | TBD | untested | | [e](evidence/INV-WEB-007/) | | |
| INV-WEB-008 | View recent activity (audit events) | web/ | web/src/routes/Dashboard.tsx:372-410 | [p](procedures/web.md#dashboard-view-recent-activity) | TBD | untested | | [e](evidence/INV-WEB-008/) | | |
| INV-WEB-009 | View recent backups | web/ | web/src/routes/Dashboard.tsx:412-477 | [p](procedures/web.md#dashboard-view-recent-backups) | TBD | untested | | [e](evidence/INV-WEB-009/) | | |
| INV-WEB-010 | Create server from dashboard button | web/ | web/src/routes/Dashboard.tsx:113-121 | [p](procedures/web.md#dashboard-create-server-button) | TBD | untested | | [e](evidence/INV-WEB-010/) | | |
| INV-WEB-011 | List all servers | web/ | web/src/routes/Servers.tsx:38-174 | [p](procedures/web.md#servers-list-view) | TBD | untested | | [e](evidence/INV-WEB-011/) | | |
| INV-WEB-012 | Filter servers by status (all, running, stopped) | web/ | web/src/routes/Servers.tsx:230-262 | [p](procedures/web.md#servers-filter-by-status) | TBD | untested | | [e](evidence/INV-WEB-012/) | | |
| INV-WEB-013 | Search servers by name | web/ | web/src/routes/Servers.tsx:263-296 | [p](procedures/web.md#servers-search-by-name) | TBD | untested | | [e](evidence/INV-WEB-013/) | | |
| INV-WEB-014 | Filter servers by game type | web/ | web/src/routes/Servers.tsx:274-295 | [p](procedures/web.md#servers-filter-by-game) | TBD | untested | | [e](evidence/INV-WEB-014/) | | |
| INV-WEB-015 | Filter servers by namespace | web/ | web/src/routes/Servers.tsx:274-295 | [p](procedures/web.md#servers-filter-by-namespace) | TBD | untested | | [e](evidence/INV-WEB-015/) | | |
| INV-WEB-016 | Open server detail page | web/ | web/src/routes/Servers.tsx:405-411 | [p](procedures/web.md#servers-open-detail) | TBD | untested | | [e](evidence/INV-WEB-016/) | | |
| INV-WEB-017 | Start stopped server from list | web/ | web/src/routes/Servers.tsx:626-633 | [p](procedures/web.md#servers-start-server) | TBD | untested | | [e](evidence/INV-WEB-017/) | | |
| INV-WEB-018 | Stop running server from list | web/ | web/src/routes/Servers.tsx:634-644 | [p](procedures/web.md#servers-stop-server) | TBD | untested | | [e](evidence/INV-WEB-018/) | | |
| INV-WEB-019 | Restart running server from list | web/ | web/src/routes/Servers.tsx:645-650 | [p](procedures/web.md#servers-restart-server) | TBD | untested | | [e](evidence/INV-WEB-019/) | | |
| INV-WEB-020 | Wake sleeping server from list | web/ | web/src/routes/Servers.tsx:617-625 | [p](procedures/web.md#servers-wake-sleeping-server) | TBD | untested | | [e](evidence/INV-WEB-020/) | | |
| INV-WEB-021 | Access server actions menu (edit, transfer, delete) | web/ | web/src/routes/Servers.tsx:651 | [p](procedures/web.md#servers-menu-actions) | TBD | untested | | [e](evidence/INV-WEB-021/) | | |
| INV-WEB-022 | View server overview (metrics, players, endpoints, events) | web/ | web/src/routes/tabs/Overview.tsx:15-200 | [p](procedures/web.md#server-detail-overview-tab) | TBD | untested | | [e](evidence/INV-WEB-022/) | | |
| INV-WEB-023 | Start server from detail page | web/ | web/src/routes/ServerDetail.tsx:227-235 | [p](procedures/web.md#server-detail-lifecycle-start) | TBD | untested | | [e](evidence/INV-WEB-023/) | | |
| INV-WEB-024 | Stop server from detail page | web/ | web/src/routes/ServerDetail.tsx:209-216 | [p](procedures/web.md#server-detail-lifecycle-stop) | TBD | untested | | [e](evidence/INV-WEB-024/) | | |
| INV-WEB-025 | Restart server from detail page | web/ | web/src/routes/ServerDetail.tsx:201-208 | [p](procedures/web.md#server-detail-lifecycle-restart) | TBD | untested | | [e](evidence/INV-WEB-025/) | | |
| INV-WEB-026 | Wake sleeping server from detail page | web/ | web/src/routes/ServerDetail.tsx:217-226 | [p](procedures/web.md#server-detail-lifecycle-wake) | TBD | untested | | [e](evidence/INV-WEB-026/) | | |
| INV-WEB-027 | Open console tab from detail page | web/ | web/src/routes/ServerDetail.tsx:237-241 | [p](procedures/web.md#server-detail-open-console) | TBD | untested | | [e](evidence/INV-WEB-027/) | | |
| INV-WEB-028 | Send command in console (PTY/RCON) | web/ | web/src/routes/tabs/Console.tsx:48-100 | [p](procedures/web.md#server-detail-console-send-command) | TBD | untested | | [e](evidence/INV-WEB-028/) | | |
| INV-WEB-029 | View pod container logs | web/ | web/src/routes/tabs/Logs.tsx:73-100 | [p](procedures/web.md#server-detail-logs-view-pod-logs) | TBD | untested | | [e](evidence/INV-WEB-029/) | | |
| INV-WEB-030 | View game log file | web/ | web/src/routes/tabs/Logs.tsx:71 | [p](procedures/web.md#server-detail-logs-view-game-logs) | TBD | untested | | [e](evidence/INV-WEB-030/) | | |
| INV-WEB-031 | Filter logs by level (INFO, WARN, ERROR, DEBUG) | web/ | web/src/routes/tabs/Logs.tsx:56-58 | [p](procedures/web.md#server-detail-logs-filter-by-level) | TBD | untested | | [e](evidence/INV-WEB-031/) | | |
| INV-WEB-032 | Download logs | web/ | web/src/routes/tabs/Logs.tsx:56-100 | [p](procedures/web.md#server-detail-logs-download) | TBD | untested | | [e](evidence/INV-WEB-032/) | | |
| INV-WEB-033 | Browse server files | web/ | web/src/routes/tabs/Files.tsx:42-100 | [p](procedures/web.md#server-detail-files-browse) | TBD | untested | | [e](evidence/INV-WEB-033/) | | |
| INV-WEB-034 | View file content in editor | web/ | web/src/routes/tabs/Files.tsx:73-95 | [p](procedures/web.md#server-detail-files-view-file) | TBD | untested | | [e](evidence/INV-WEB-034/) | | |
| INV-WEB-035 | Edit and save file | web/ | web/src/routes/tabs/Files.tsx:42-100 | [p](procedures/web.md#server-detail-files-edit-file) | TBD | untested | | [e](evidence/INV-WEB-035/) | | |
| INV-WEB-036 | Upload file to server | web/ | web/src/routes/tabs/Files.tsx:62 | [p](procedures/web.md#server-detail-files-upload-file) | TBD | untested | | [e](evidence/INV-WEB-036/) | | |
| INV-WEB-037 | Create new file | web/ | web/src/routes/tabs/Files.tsx:59-60 | [p](procedures/web.md#server-detail-files-create-file) | TBD | untested | | [e](evidence/INV-WEB-037/) | | |
| INV-WEB-038 | Create new directory | web/ | web/src/routes/tabs/Files.tsx:59 | [p](procedures/web.md#server-detail-files-create-directory) | TBD | untested | | [e](evidence/INV-WEB-038/) | | |
| INV-WEB-039 | Delete file or directory | web/ | web/src/routes/tabs/Files.tsx:58 | [p](procedures/web.md#server-detail-files-delete-file) | TBD | untested | | [e](evidence/INV-WEB-039/) | | |
| INV-WEB-040 | Download file from server | web/ | web/src/routes/tabs/Files.tsx:42-100 | [p](procedures/web.md#server-detail-files-download-file) | TBD | untested | | [e](evidence/INV-WEB-040/) | | |
| INV-WEB-041 | View Kubernetes events | web/ | web/src/routes/tabs/Events.tsx:11-40 | [p](procedures/web.md#server-detail-events-view) | TBD | untested | | [e](evidence/INV-WEB-041/) | | |
| INV-WEB-042 | Filter events by type (all, info, warnings) | web/ | web/src/routes/tabs/Events.tsx:34-39 | [p](procedures/web.md#server-detail-events-filter) | TBD | untested | | [e](evidence/INV-WEB-042/) | | |
| INV-WEB-043 | List installed mods | web/ | web/src/routes/tabs/Mods.tsx:55-100 | [p](procedures/web.md#server-detail-mods-list) | TBD | untested | | [e](evidence/INV-WEB-043/) | | |
| INV-WEB-044 | Install mod from URL | web/ | web/src/routes/tabs/Mods.tsx:65-100 | [p](procedures/web.md#server-detail-mods-install-url) | TBD | untested | | [e](evidence/INV-WEB-044/) | | |
| INV-WEB-045 | Browse mod registry | web/ | web/src/routes/tabs/Mods.tsx:65-100 | [p](procedures/web.md#server-detail-mods-browse-registry) | TBD | untested | | [e](evidence/INV-WEB-045/) | | |
| INV-WEB-046 | Install mod from registry | web/ | web/src/routes/tabs/Mods.tsx:65-100 | [p](procedures/web.md#server-detail-mods-install-registry) | TBD | untested | | [e](evidence/INV-WEB-046/) | | |
| INV-WEB-047 | Upload custom mod file | web/ | web/src/routes/tabs/Mods.tsx:65-100 | [p](procedures/web.md#server-detail-mods-upload) | TBD | untested | | [e](evidence/INV-WEB-047/) | | |
| INV-WEB-048 | Check for mod updates | web/ | web/src/routes/tabs/Mods.tsx:100 | [p](procedures/web.md#server-detail-mods-check-updates) | TBD | untested | | [e](evidence/INV-WEB-048/) | | |
| INV-WEB-049 | Remove installed mod | web/ | web/src/routes/tabs/Mods.tsx:65-100 | [p](procedures/web.md#server-detail-mods-remove) | TBD | untested | | [e](evidence/INV-WEB-049/) | | |
| INV-WEB-050 | Browse modpacks in registry | web/ | web/src/routes/tabs/Modpacks.tsx:29-100 | [p](procedures/web.md#server-detail-modpacks-browse) | TBD | untested | | [e](evidence/INV-WEB-050/) | | |
| INV-WEB-051 | Install modpack | web/ | web/src/routes/tabs/Modpacks.tsx:29-100 | [p](procedures/web.md#server-detail-modpacks-install) | TBD | untested | | [e](evidence/INV-WEB-051/) | | |
| INV-WEB-052 | View online players | web/ | web/src/routes/tabs/Players.tsx:28-100 | [p](procedures/web.md#server-detail-players-view) | TBD | untested | | [e](evidence/INV-WEB-052/) | | |
| INV-WEB-053 | Kick player from server | web/ | web/src/routes/tabs/Players.tsx:28-100 | [p](procedures/web.md#server-detail-players-kick) | TBD | untested | | [e](evidence/INV-WEB-053/) | | |
| INV-WEB-054 | Ban player | web/ | web/src/routes/tabs/Players.tsx:28-100 | [p](procedures/web.md#server-detail-players-ban) | TBD | untested | | [e](evidence/INV-WEB-054/) | | |
| INV-WEB-055 | Unban player | web/ | web/src/routes/tabs/Players.tsx:28-100 | [p](procedures/web.md#server-detail-players-unban) | TBD | untested | | [e](evidence/INV-WEB-055/) | | |
| INV-WEB-056 | Add player to whitelist | web/ | web/src/routes/tabs/Players.tsx:28-100 | [p](procedures/web.md#server-detail-players-whitelist-add) | TBD | untested | | [e](evidence/INV-WEB-056/) | | |
| INV-WEB-057 | Remove player from whitelist | web/ | web/src/routes/tabs/Players.tsx:28-100 | [p](procedures/web.md#server-detail-players-whitelist-remove) | TBD | untested | | [e](evidence/INV-WEB-057/) | | |
| INV-WEB-058 | Create backup now (from server detail) | web/ | web/src/routes/tabs/Backups.tsx:42-50 | [p](procedures/web.md#server-detail-backups-create-now) | TBD | untested | | [e](evidence/INV-WEB-058/) | | |
| INV-WEB-059 | View server backups list | web/ | web/src/routes/tabs/Backups.tsx:16-100 | [p](procedures/web.md#server-detail-backups-view-list) | TBD | untested | | [e](evidence/INV-WEB-059/) | | |
| INV-WEB-060 | Restore from backup | web/ | web/src/routes/tabs/Backups.tsx:16-100 | [p](procedures/web.md#server-detail-backups-restore) | TBD | untested | | [e](evidence/INV-WEB-060/) | | |
| INV-WEB-061 | Create backup schedule | web/ | web/src/routes/tabs/Backups.tsx:16-100 | [p](procedures/web.md#server-detail-backups-schedule-create) | TBD | untested | | [e](evidence/INV-WEB-061/) | | |
| INV-WEB-062 | Delete backup schedule | web/ | web/src/routes/tabs/Backups.tsx:16-100 | [p](procedures/web.md#server-detail-backups-schedule-delete) | TBD | untested | | [e](evidence/INV-WEB-062/) | | |
| INV-WEB-063 | Edit server general settings (name, description, labels) | web/ | web/src/routes/tabs/settings/General.tsx:7-100 | [p](procedures/web.md#server-detail-settings-general) | TBD | untested | | [e](evidence/INV-WEB-063/) | | |
| INV-WEB-064 | Change server version | web/ | web/src/routes/tabs/settings/Version.tsx:1-50 | [p](procedures/web.md#server-detail-settings-version) | TBD | untested | | [e](evidence/INV-WEB-064/) | | |
| INV-WEB-065 | Edit server resources (CPU, memory, storage) | web/ | web/src/routes/tabs/settings/Resources.tsx:1-50 | [p](procedures/web.md#server-detail-settings-resources) | TBD | untested | | [e](evidence/INV-WEB-065/) | | |
| INV-WEB-066 | Edit server networking (exposure, hostname, ports, tunnel) | web/ | web/src/routes/tabs/settings/Networking.tsx:1-50 | [p](procedures/web.md#server-detail-settings-networking) | TBD | untested | | [e](evidence/INV-WEB-066/) | | |
| INV-WEB-067 | Edit server environment variables | web/ | web/src/routes/tabs/settings/EnvVars.tsx:1-50 | [p](procedures/web.md#server-detail-settings-environment) | TBD | untested | | [e](evidence/INV-WEB-067/) | | |
| INV-WEB-068 | Edit server lifecycle policies (auto-restart, updates, idle sleep) | web/ | web/src/routes/tabs/settings/Lifecycle.tsx:1-50 | [p](procedures/web.md#server-detail-settings-lifecycle) | TBD | untested | | [e](evidence/INV-WEB-068/) | | |
| INV-WEB-069 | Manage server backup schedules in settings | web/ | web/src/routes/tabs/settings/Backups.tsx:1-50 | [p](procedures/web.md#server-detail-settings-scheduled-backups) | TBD | untested | | [e](evidence/INV-WEB-069/) | | |
| INV-WEB-070 | Configure network packet capture | web/ | web/src/routes/tabs/settings/NetworkCapture.tsx:1-50 | [p](procedures/web.md#server-detail-settings-network-capture) | TBD | untested | | [e](evidence/INV-WEB-070/) | | |
| INV-WEB-071 | Configure node placement (affinity, anti-affinity) | web/ | web/src/routes/tabs/settings/Placement.tsx:1-50 | [p](procedures/web.md#server-detail-settings-placement) | TBD | untested | | [e](evidence/INV-WEB-071/) | | |
| INV-WEB-072 | Manage server RBAC and access (role bindings) | web/ | web/src/routes/tabs/settings/Access.tsx:1-50 | [p](procedures/web.md#server-detail-settings-access) | TBD | untested | | [e](evidence/INV-WEB-072/) | | |
| INV-WEB-073 | Create share link with expiry | web/ | web/src/routes/tabs/settings/ShareLinks.tsx:1-100 | [p](procedures/web.md#server-detail-settings-sharelinks-create) | TBD | untested | | [e](evidence/INV-WEB-073/) | | |
| INV-WEB-074 | View share links list | web/ | web/src/routes/tabs/settings/ShareLinks.tsx:1-100 | [p](procedures/web.md#server-detail-settings-sharelinks-view) | TBD | untested | | [e](evidence/INV-WEB-074/) | | |
| INV-WEB-075 | Revoke share link | web/ | web/src/routes/tabs/settings/ShareLinks.tsx:1-100 | [p](procedures/web.md#server-detail-settings-sharelinks-revoke) | TBD | untested | | [e](evidence/INV-WEB-075/) | | |
| INV-WEB-076 | Delete server | web/ | web/src/routes/tabs/settings/Danger.tsx:1-50 | [p](procedures/web.md#server-detail-settings-danger-delete) | TBD | untested | | [e](evidence/INV-WEB-076/) | | |
| INV-WEB-077 | Transfer server ownership | web/ | web/src/routes/tabs/settings/Danger.tsx:1-50 | [p](procedures/web.md#server-detail-settings-danger-transfer) | TBD | untested | | [e](evidence/INV-WEB-077/) | | |
| INV-WEB-078 | Browse module catalog | web/ | web/src/routes/Modules.tsx:24-100 | [p](procedures/web.md#modules-catalog-browse) | TBD | untested | | [e](evidence/INV-WEB-078/) | | |
| INV-WEB-079 | Search modules by name | web/ | web/src/routes/Modules.tsx:36 | [p](procedures/web.md#modules-search) | TBD | untested | | [e](evidence/INV-WEB-079/) | | |
| INV-WEB-080 | Filter modules by source | web/ | web/src/routes/Modules.tsx:37 | [p](procedures/web.md#modules-filter-by-source) | TBD | untested | | [e](evidence/INV-WEB-080/) | | |
| INV-WEB-081 | Filter modules by category | web/ | web/src/routes/Modules.tsx:38 | [p](procedures/web.md#modules-filter-by-category) | TBD | untested | | [e](evidence/INV-WEB-081/) | | |
| INV-WEB-082 | Install module | web/ | web/src/routes/Modules.tsx:89-98 | [p](procedures/web.md#modules-install) | TBD | untested | | [e](evidence/INV-WEB-082/) | | |
| INV-WEB-083 | Upgrade module | web/ | web/src/routes/Modules.tsx:100 | [p](procedures/web.md#modules-upgrade) | TBD | untested | | [e](evidence/INV-WEB-083/) | | |
| INV-WEB-084 | Uninstall module | web/ | web/src/routes/Modules.tsx:100 | [p](procedures/web.md#modules-uninstall) | TBD | untested | | [e](evidence/INV-WEB-084/) | | |
| INV-WEB-085 | Upload custom module | web/ | web/src/routes/Modules.tsx:42 | [p](procedures/web.md#modules-upload-custom) | TBD | untested | | [e](evidence/INV-WEB-085/) | | |
| INV-WEB-086 | Build custom module | web/ | web/src/routes/Modules.tsx:43 | [p](procedures/web.md#modules-build-custom) | TBD | untested | | [e](evidence/INV-WEB-086/) | | |
| INV-WEB-087 | View cluster nodes | web/ | web/src/routes/Cluster.tsx:25-100 | [p](procedures/web.md#cluster-view-nodes) | TBD | untested | | [e](evidence/INV-WEB-087/) | | |
| INV-WEB-088 | Download cluster kubeconfig | web/ | web/src/routes/Cluster.tsx:61-76 | [p](procedures/web.md#cluster-download-kubeconfig) | TBD | untested | | [e](evidence/INV-WEB-088/) | | |
| INV-WEB-089 | Add node to cluster | web/ | web/src/routes/Cluster.tsx:52-59 | [p](procedures/web.md#cluster-add-node) | TBD | untested | | [e](evidence/INV-WEB-089/) | | |
| INV-WEB-090 | List users | web/ | web/src/routes/Users.tsx:83-100 | [p](procedures/web.md#users-list-view) | TBD | untested | | [e](evidence/INV-WEB-090/) | | |
| INV-WEB-091 | Invite user | web/ | web/src/routes/Users.tsx:100 | [p](procedures/web.md#users-invite) | TBD | untested | | [e](evidence/INV-WEB-091/) | | |
| INV-WEB-092 | Edit user role | web/ | web/src/routes/Users.tsx:100 | [p](procedures/web.md#users-edit-role) | TBD | untested | | [e](evidence/INV-WEB-092/) | | |
| INV-WEB-093 | Reset user password | web/ | web/src/routes/Users.tsx:100 | [p](procedures/web.md#users-reset-password) | TBD | untested | | [e](evidence/INV-WEB-093/) | | |
| INV-WEB-094 | Delete user | web/ | web/src/routes/Users.tsx:100 | [p](procedures/web.md#users-delete) | TBD | untested | | [e](evidence/INV-WEB-094/) | | |
| INV-WEB-095 | Manage custom roles | web/ | web/src/routes/Users.tsx:100 | [p](procedures/web.md#users-manage-roles) | TBD | untested | | [e](evidence/INV-WEB-095/) | | |
| INV-WEB-096 | Manage service accounts | web/ | web/src/routes/Users.tsx:100 | [p](procedures/web.md#users-manage-service-accounts) | TBD | untested | | [e](evidence/INV-WEB-096/) | | |
| INV-WEB-097 | Manage OIDC providers | web/ | web/src/routes/Users.tsx:100 | [p](procedures/web.md#users-manage-oidc-providers) | TBD | untested | | [e](evidence/INV-WEB-097/) | | |
| INV-WEB-098 | Edit general admin settings | web/ | web/src/routes/AdminSettings.tsx:89-142 | [p](procedures/web.md#admin-settings-general) | TBD | untested | | [e](evidence/INV-WEB-098/) | | |
| INV-WEB-099 | Configure authentication (local, SSO, OIDC) | web/ | web/src/routes/AdminSettings.tsx:89-142 | [p](procedures/web.md#admin-settings-auth) | TBD | untested | | [e](evidence/INV-WEB-099/) | | |
| INV-WEB-100 | Configure backup destinations | web/ | web/src/routes/AdminSettings.tsx:131 | [p](procedures/web.md#admin-settings-backup-destinations) | TBD | untested | | [e](evidence/INV-WEB-100/) | | |
| INV-WEB-101 | Manage module sources | web/ | web/src/routes/AdminSettings.tsx:132 | [p](procedures/web.md#admin-settings-module-sources) | TBD | untested | | [e](evidence/INV-WEB-101/) | | |
| INV-WEB-102 | Configure mod registries | web/ | web/src/routes/AdminSettings.tsx:133 | [p](procedures/web.md#admin-settings-mod-registries) | TBD | untested | | [e](evidence/INV-WEB-102/) | | |
| INV-WEB-103 | Configure notifications (Slack, email, webhook) | web/ | web/src/routes/AdminSettings.tsx:134 | [p](procedures/web.md#admin-settings-notifications) | TBD | untested | | [e](evidence/INV-WEB-103/) | | |
| INV-WEB-104 | Configure telemetry collection | web/ | web/src/routes/AdminSettings.tsx:135 | [p](procedures/web.md#admin-settings-telemetry) | TBD | untested | | [e](evidence/INV-WEB-104/) | | |
| INV-WEB-105 | View update availability | web/ | web/src/routes/AdminSettings.tsx:136 | [p](procedures/web.md#admin-settings-updates) | TBD | untested | | [e](evidence/INV-WEB-105/) | | |
| INV-WEB-106 | View about information (version, build, license) | web/ | web/src/routes/AdminSettings.tsx:137 | [p](procedures/web.md#admin-settings-about) | TBD | untested | | [e](evidence/INV-WEB-106/) | | |
| INV-WEB-107 | Select theme preset | web/ | web/src/routes/ThemeSettings.tsx:50-59 | [p](procedures/web.md#theme-settings-preset-selection) | TBD | untested | | [e](evidence/INV-WEB-107/) | | |
| INV-WEB-108 | Select appearance mode (light, dark, system) | web/ | web/src/routes/ThemeSettings.tsx:81-85 | [p](procedures/web.md#theme-settings-appearance-mode) | TBD | untested | | [e](evidence/INV-WEB-108/) | | |
| INV-WEB-109 | Select custom theme colors | web/ | web/src/routes/ThemeSettings.tsx:64-79 | [p](procedures/web.md#theme-settings-custom-colors) | TBD | untested | | [e](evidence/INV-WEB-109/) | | |
| INV-WEB-110 | Edit custom CSS | web/ | web/src/routes/ThemeSettings.tsx:87-100 | [p](procedures/web.md#theme-settings-custom-css) | TBD | untested | | [e](evidence/INV-WEB-110/) | | |
| INV-WEB-111 | Export theme configuration | web/ | web/src/routes/ThemeSettings.tsx:100 | [p](procedures/web.md#theme-settings-export) | TBD | untested | | [e](evidence/INV-WEB-111/) | | |
| INV-WEB-112 | Import theme configuration | web/ | web/src/routes/ThemeSettings.tsx:100 | [p](procedures/web.md#theme-settings-import) | TBD | untested | | [e](evidence/INV-WEB-112/) | | |
| INV-WEB-113 | List all backups | web/ | web/src/routes/Backups.tsx:53-100 | [p](procedures/web.md#backups-list-view) | TBD | untested | | [e](evidence/INV-WEB-113/) | | |
| INV-WEB-114 | Filter backups by server | web/ | web/src/routes/Backups.tsx:91-100 | [p](procedures/web.md#backups-filter-by-server) | TBD | untested | | [e](evidence/INV-WEB-114/) | | |
| INV-WEB-115 | Filter backups by phase | web/ | web/src/routes/Backups.tsx:91-100 | [p](procedures/web.md#backups-filter-by-phase) | TBD | untested | | [e](evidence/INV-WEB-115/) | | |
| INV-WEB-116 | Create backup now from backups page | web/ | web/src/routes/Backups.tsx:53-100 | [p](procedures/web.md#backups-backup-now) | TBD | untested | | [e](evidence/INV-WEB-116/) | | |
| INV-WEB-117 | Restore from backup on backups page | web/ | web/src/routes/Backups.tsx:53-100 | [p](procedures/web.md#backups-restore) | TBD | untested | | [e](evidence/INV-WEB-117/) | | |
| INV-WEB-118 | View backup detail | web/ | web/src/routes/Backups.tsx:53-100 | [p](procedures/web.md#backups-view-detail) | TBD | untested | | [e](evidence/INV-WEB-118/) | | |
| INV-WEB-119 | Create backup schedule on backups page | web/ | web/src/routes/Backups.tsx:53-100 | [p](procedures/web.md#backups-schedules-create) | TBD | untested | | [e](evidence/INV-WEB-119/) | | |
| INV-WEB-120 | Edit backup schedule | web/ | web/src/routes/Backups.tsx:53-100 | [p](procedures/web.md#backups-schedules-edit) | TBD | untested | | [e](evidence/INV-WEB-120/) | | |
| INV-WEB-121 | Suspend/resume backup schedule | web/ | web/src/routes/Backups.tsx:53-100 | [p](procedures/web.md#backups-schedules-toggle-suspend) | TBD | untested | | [e](evidence/INV-WEB-121/) | | |
| INV-WEB-122 | Delete backup schedule from backups page | web/ | web/src/routes/Backups.tsx:53-100 | [p](procedures/web.md#backups-schedules-delete) | TBD | untested | | [e](evidence/INV-WEB-122/) | | |
| INV-WEB-123 | View restore operations | web/ | web/src/routes/Backups.tsx:53-100 | [p](procedures/web.md#backups-restores-view) | TBD | untested | | [e](evidence/INV-WEB-123/) | | |
| INV-WEB-124 | View audit log events | web/ | web/src/routes/AuditLog.tsx:16-100 | [p](procedures/web.md#audit-log-view) | TBD | untested | | [e](evidence/INV-WEB-124/) | | |
| INV-WEB-125 | Filter audit events by HTTP status class | web/ | web/src/routes/AuditLog.tsx:16-100 | [p](procedures/web.md#audit-log-filter-by-status-class) | TBD | untested | | [e](evidence/INV-WEB-125/) | | |
| INV-WEB-126 | Filter audit events by HTTP method | web/ | web/src/routes/AuditLog.tsx:16-100 | [p](procedures/web.md#audit-log-filter-by-method) | TBD | untested | | [e](evidence/INV-WEB-126/) | | |
| INV-WEB-127 | Filter audit events by actor | web/ | web/src/routes/AuditLog.tsx:16-100 | [p](procedures/web.md#audit-log-filter-by-actor) | TBD | untested | | [e](evidence/INV-WEB-127/) | | |
| INV-WEB-128 | Paginate audit log | web/ | web/src/routes/AuditLog.tsx:25-33 | [p](procedures/web.md#audit-log-pagination) | TBD | untested | | [e](evidence/INV-WEB-128/) | | |
| INV-WEB-129 | Export audit events to CSV | web/ | web/src/routes/AuditLog.tsx:35-46 | [p](procedures/web.md#audit-log-export-csv) | TBD | untested | | [e](evidence/INV-WEB-129/) | | |
| INV-WEB-130 | View audit log integrity | web/ | web/src/routes/AuditLog.tsx:20-24 | [p](procedures/web.md#audit-log-verify-integrity) | TBD | untested | | [e](evidence/INV-WEB-130/) | | |
| INV-WEB-131 | View API server logs | web/ | web/src/routes/AdminLogs.tsx:78-100 | [p](procedures/web.md#admin-logs-view-api) | TBD | untested | | [e](evidence/INV-WEB-131/) | | |
| INV-WEB-132 | View operator logs | web/ | web/src/routes/AdminLogs.tsx:78-100 | [p](procedures/web.md#admin-logs-view-operator) | TBD | untested | | [e](evidence/INV-WEB-132/) | | |
| INV-WEB-133 | Select log tail size | web/ | web/src/routes/AdminLogs.tsx:16-17 | [p](procedures/web.md#admin-logs-tail-option) | TBD | untested | | [e](evidence/INV-WEB-133/) | | |
| INV-WEB-134 | Enable follow mode for system logs | web/ | web/src/routes/AdminLogs.tsx:81 | [p](procedures/web.md#admin-logs-follow) | TBD | untested | | [e](evidence/INV-WEB-134/) | | |
| INV-WEB-135 | Download system logs | web/ | web/src/routes/AdminLogs.tsx:78-100 | [p](procedures/web.md#admin-logs-download) | TBD | untested | | [e](evidence/INV-WEB-135/) | | |
| INV-WEB-136 | Access shared server link | web/ | web/src/routes/Share.tsx:46-100 | [p](procedures/web.md#share-access-link-view) | TBD | untested | | [e](evidence/INV-WEB-136/) | | |
| INV-WEB-137 | Start server from share link | web/ | web/src/routes/Share.tsx:46-100 | [p](procedures/web.md#share-server-start) | TBD | untested | | [e](evidence/INV-WEB-137/) | | |
| INV-WEB-138 | View shared server status | web/ | web/src/routes/Share.tsx:46-100 | [p](procedures/web.md#share-server-view-status) | TBD | untested | | [e](evidence/INV-WEB-138/) | | |
| INV-WEB-139 | Copy server address from share link | web/ | web/src/routes/Share.tsx:46-100 | [p](procedures/web.md#share-link-copy-address) | TBD | untested | | [e](evidence/INV-WEB-139/) | | |
| INV-WEB-140 | Set theme preference on share link | web/ | web/src/routes/Share.tsx:17-43 | [p](procedures/web.md#share-theme-preference) | TBD | untested | | [e](evidence/INV-WEB-140/) | | |

Row count: 140
