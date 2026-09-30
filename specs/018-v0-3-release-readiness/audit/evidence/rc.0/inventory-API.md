# Draft inventory rows: API (rc.0)

Links are relative to audit/inventory.md.

| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|
| INV-API-001 | Health check (read) | api/ | api/cmd/main.go:257 | [p](procedures/api.md#public-healthz-get) | TBD | untested | | [e](INV-API-001/) | | |
| INV-API-002 | Metrics (read) | api/ | api/cmd/main.go:260 | [p](procedures/api.md#public-metrics-get) | TBD | untested | | [e](INV-API-002/) | | |
| INV-API-003 | List auth providers (read) | api/ | api/cmd/main.go:267 | [p](procedures/api.md#public-auth-providers-list) | TBD | untested | | [e](INV-API-003/) | | |
| INV-API-004 | Login (create session) | api/ | api/cmd/main.go:271 | [p](procedures/api.md#public-auth-login) | TBD | untested | | [e](INV-API-004/) | | |
| INV-API-005 | Logout (destroy session) | api/ | api/cmd/main.go:272 | [p](procedures/api.md#public-auth-logout) | TBD | untested | | [e](INV-API-005/) | | |
| INV-API-006 | OIDC authorize start | api/ | api/cmd/main.go:277 | [p](procedures/api.md#public-auth-oidc-provider-start) | TBD | untested | | [e](INV-API-006/) | | |
| INV-API-007 | OIDC callback | api/ | api/cmd/main.go:278 | [p](procedures/api.md#public-auth-oidc-callback) | TBD | untested | blocked: external IdP | [e](INV-API-007/) | | |
| INV-API-008 | OIDC authorize start (legacy) | api/ | api/cmd/main.go:283 | [p](procedures/api.md#public-auth-oidc-provider-start) | TBD | untested | | [e](INV-API-008/) | | |
| INV-API-009 | OIDC callback (legacy) | api/ | api/cmd/main.go:284 | [p](procedures/api.md#public-auth-oidc-callback) | TBD | untested | blocked: external IdP | [e](INV-API-009/) | | |
| INV-API-010 | Resolve public share link | api/ | api/internal/handlers/shares.go:40 | [p](procedures/api.md#public-shares-resolve) | TBD | untested | | [e](INV-API-010/) | | |
| INV-API-011 | Start server from public share | api/ | api/internal/handlers/shares.go:41 | [p](procedures/api.md#public-shares-start) | TBD | untested | blocked: share creation requires auth | [e](INV-API-011/) | | |
| INV-API-012 | List servers | api/ | api/internal/handlers/resources.go:53-54 | [p](procedures/api.md#servers-list) | TBD | untested | | [e](INV-API-012/) | | |
| INV-API-013 | Create server | api/ | api/internal/handlers/resources.go:55 | [p](procedures/api.md#servers-create) | TBD | untested | | [e](INV-API-013/) | | |
| INV-API-014 | Get server | api/ | api/internal/handlers/resources.go:56 | [p](procedures/api.md#servers-get) | TBD | untested | | [e](INV-API-014/) | | |
| INV-API-015 | Update server | api/ | api/internal/handlers/resources.go:57 | [p](procedures/api.md#servers-update) | TBD | untested | | [e](INV-API-015/) | | |
| INV-API-016 | Delete server | api/ | api/internal/handlers/resources.go:58 | [p](procedures/api.md#servers-delete) | TBD | untested | | [e](INV-API-016/) | | |
| INV-API-017 | List templates | api/ | api/internal/handlers/resources.go:53-54 | [p](procedures/api.md#templates-list) | TBD | untested | | [e](INV-API-017/) | | |
| INV-API-018 | Get template | api/ | api/internal/handlers/resources.go:56 | [p](procedures/api.md#templates-get) | TBD | untested | | [e](INV-API-018/) | | |
| INV-API-019 | List backups | api/ | api/internal/handlers/resources.go:53-54 | [p](procedures/api.md#backups-list) | TBD | untested | | [e](INV-API-019/) | | |
| INV-API-020 | Create backup | api/ | api/internal/handlers/resources.go:55 | [p](procedures/api.md#servers-create) | TBD | untested | blocked: requires backup trigger; alternative: list backups | [e](INV-API-020/) | | |
| INV-API-021 | Get backup | api/ | api/internal/handlers/resources.go:56 | [p](procedures/api.md#servers-get) | TBD | untested | | [e](INV-API-021/) | | |
| INV-API-022 | Delete backup | api/ | api/internal/handlers/resources.go:58 | [p](procedures/api.md#servers-delete) | TBD | untested | | [e](INV-API-022/) | | |
| INV-API-023 | List schedules | api/ | api/internal/handlers/resources.go:53-54 | [p](procedures/api.md#schedules-list) | TBD | untested | | [e](INV-API-023/) | | |
| INV-API-024 | Create schedule | api/ | api/internal/handlers/resources.go:55 | [p](procedures/api.md#servers-create) | TBD | untested | | [e](INV-API-024/) | | |
| INV-API-025 | List restores | api/ | api/internal/handlers/resources.go:53-54 | [p](procedures/api.md#restores-list) | TBD | untested | | [e](INV-API-025/) | | |
| INV-API-026 | Restore from backup | api/ | api/internal/handlers/resources.go:55 | [p](procedures/api.md#servers-create) | TBD | untested | blocked: requires backup; alternative: list restores | [e](INV-API-026/) | | |
| INV-API-027 | Read audit log (paginated) | api/ | api/internal/handlers/audit.go:23 | [p](procedures/api.md#audit-read-paginated) | TBD | untested | | [e](INV-API-027/) | | |
| INV-API-028 | Verify audit chain integrity | api/ | api/internal/handlers/audit.go:42 | [p](procedures/api.md#audit-verify-chain) | TBD | untested | | [e](INV-API-028/) | | |
| INV-API-029 | Export audit log (CSV) | api/ | api/internal/handlers/audit.go:58 | [p](procedures/api.md#audit-export-csv) | TBD | untested | | [e](INV-API-029/) | | |
| INV-API-030 | Export audit log (JSON) | api/ | api/internal/handlers/audit.go:58 | [p](procedures/api.md#audit-export-json) | TBD | untested | | [e](INV-API-030/) | | |
| INV-API-031 | Read admin config | api/ | api/internal/handlers/config.go:53 | [p](procedures/api.md#config-read-all) | TBD | untested | | [e](INV-API-031/) | | |
| INV-API-032 | Write admin config section | api/ | api/internal/handlers/config.go:54 | [p](procedures/api.md#config-write-section) | TBD | untested | | [e](INV-API-032/) | | |
| INV-API-033 | Reset role mapping (auth config) | api/ | api/internal/handlers/config.go:55 | [p](procedures/api.md#config-write-section) | TBD | untested | | [e](INV-API-033/) | | |
| INV-API-034 | Store OIDC provider secret | api/ | api/internal/handlers/auth_provider_secret.go:25 | [p](procedures/api.md#auth-provider-secret-put) | TBD | untested | | [e](INV-API-034/) | | |
| INV-API-035 | Delete OIDC provider secret | api/ | api/internal/handlers/auth_provider_secret.go:26 | [p](procedures/api.md#auth-provider-secret-delete) | TBD | untested | | [e](INV-API-035/) | | |
| INV-API-036 | Test notification sink | api/ | api/internal/handlers/notifications.go:31 | [p](procedures/api.md#notifications-test-sink) | TBD | untested | deferred: T031 | [e](INV-API-036/) | | |
| INV-API-037 | Store notification sink secret | api/ | api/internal/handlers/notifications.go:32 | [p](procedures/api.md#notifications-secret-put) | TBD | untested | | [e](INV-API-037/) | | |
| INV-API-038 | Delete notification sink secret | api/ | api/internal/handlers/notifications.go:33 | [p](procedures/api.md#notifications-secret-put) | TBD | untested | | [e](INV-API-038/) | | |
| INV-API-039 | Store mod registry secret | api/ | api/internal/handlers/registry_secret.go:22 | [p](procedures/api.md#registry-secret-put) | TBD | untested | | [e](INV-API-039/) | | |
| INV-API-040 | Delete mod registry secret | api/ | api/internal/handlers/registry_secret.go:23 | [p](procedures/api.md#registry-secret-put) | TBD | untested | | [e](INV-API-040/) | | |
| INV-API-041 | Enable network capture | api/ | api/internal/handlers/capture.go:57 | [p](procedures/api.md#capture-enable) | TBD | untested | deferred: T031 | [e](INV-API-041/) | | |
| INV-API-042 | Disable network capture | api/ | api/internal/handlers/capture.go:58 | [p](procedures/api.md#capture-enable) | TBD | untested | deferred: T031 | [e](INV-API-042/) | | |
| INV-API-043 | Start packet capture | api/ | api/internal/handlers/capture.go:59 | [p](procedures/api.md#capture-start) | TBD | untested | deferred: T031 | [e](INV-API-043/) | | |
| INV-API-044 | Stop packet capture | api/ | api/internal/handlers/capture.go:60 | [p](procedures/api.md#capture-start) | TBD | untested | deferred: T031 | [e](INV-API-044/) | | |
| INV-API-045 | List packet captures | api/ | api/internal/handlers/capture.go:61 | [p](procedures/api.md#capture-list) | TBD | untested | deferred: T031 | [e](INV-API-045/) | | |
| INV-API-046 | Get packet capture | api/ | api/internal/handlers/capture.go:62 | [p](procedures/api.md#capture-list) | TBD | untested | deferred: T031 | [e](INV-API-046/) | | |
| INV-API-047 | Download capture file | api/ | api/internal/handlers/capture.go:63 | [p](procedures/api.md#capture-list) | TBD | untested | deferred: T031 | [e](INV-API-047/) | | |
| INV-API-048 | Delete packet capture | api/ | api/internal/handlers/capture.go:64 | [p](procedures/api.md#capture-list) | TBD | untested | deferred: T031 | [e](INV-API-048/) | | |
| INV-API-049 | Start server | api/ | api/internal/handlers/lifecycle.go:41 | [p](procedures/api.md#servers-start) | TBD | untested | | [e](INV-API-049/) | | |
| INV-API-050 | Stop server | api/ | api/internal/handlers/lifecycle.go:42 | [p](procedures/api.md#servers-stop) | TBD | untested | | [e](INV-API-050/) | | |
| INV-API-051 | Restart server | api/ | api/internal/handlers/lifecycle.go:43 | [p](procedures/api.md#servers-restart) | TBD | untested | | [e](INV-API-051/) | | |
| INV-API-052 | Wake server from idle | api/ | api/internal/handlers/lifecycle.go:44 | [p](procedures/api.md#servers-restart) | TBD | untested | | [e](INV-API-052/) | | |
| INV-API-053 | Clone server | api/ | api/internal/handlers/lifecycle.go:45 | [p](procedures/api.md#servers-clone) | TBD | untested | | [e](INV-API-053/) | | |
| INV-API-054 | Wipe server data | api/ | api/internal/handlers/lifecycle.go:46 | [p](procedures/api.md#servers-wipe-data) | TBD | untested | | [e](INV-API-054/) | | |
| INV-API-055 | Create share link | api/ | api/internal/handlers/shares.go:27-28 | [p](procedures/api.md#shares-create) | TBD | untested | deferred: T031 (OD-015 blocks) | [e](INV-API-055/) | | |
| INV-API-056 | List share links | api/ | api/internal/handlers/shares.go:29 | [p](procedures/api.md#shares-list) | TBD | untested | deferred: T031 (OD-015 blocks) | [e](INV-API-056/) | | |
| INV-API-057 | Revoke share link | api/ | api/internal/handlers/shares.go:31 | [p](procedures/api.md#shares-list) | TBD | untested | deferred: T031 (OD-015 blocks) | [e](INV-API-057/) | | |
| INV-API-058 | Transfer server ownership | api/ | api/internal/handlers/ownership.go:57 | [p](procedures/api.md#servers-transfer) | TBD | untested | deferred: T031 (OD-015 blocks) | [e](INV-API-058/) | | |
| INV-API-059 | Set server collaborators | api/ | api/internal/handlers/ownership.go:58 | [p](procedures/api.md#servers-collaborators-set) | TBD | untested | deferred: T031 (OD-015 blocks) | [e](INV-API-059/) | | |
| INV-API-060 | Get owned servers | api/ | api/internal/handlers/ownership.go:59 | [p](procedures/api.md#servers-transfer) | TBD | untested | | [e](INV-API-060/) | | |
| INV-API-061 | List users | api/ | api/internal/handlers/users.go:29 | [p](procedures/api.md#users-list) | TBD | untested | | [e](INV-API-061/) | | |
| INV-API-062 | Create user | api/ | api/internal/handlers/users.go:30 | [p](procedures/api.md#users-create) | TBD | untested | | [e](INV-API-062/) | | |
| INV-API-063 | Get current user (/me) | api/ | api/internal/handlers/users.go:31 | [p](procedures/api.md#users-me) | TBD | untested | | [e](INV-API-063/) | | |
| INV-API-064 | Get user preferences | api/ | api/internal/handlers/users.go:32 | [p](procedures/api.md#users-preferences-get) | TBD | untested | | [e](INV-API-064/) | | |
| INV-API-065 | Update user preferences | api/ | api/internal/handlers/users.go:33 | [p](procedures/api.md#users-preferences-put) | TBD | untested | | [e](INV-API-065/) | | |
| INV-API-066 | Reset user preferences | api/ | api/internal/handlers/users.go:34 | [p](procedures/api.md#users-preferences-reset) | TBD | untested | | [e](INV-API-066/) | | |
| INV-API-067 | Get user | api/ | api/internal/handlers/users.go:35 | [p](procedures/api.md#users-get) | TBD | untested | | [e](INV-API-067/) | | |
| INV-API-068 | Update user | api/ | api/internal/handlers/users.go:36 | [p](procedures/api.md#users-update) | TBD | untested | | [e](INV-API-068/) | | |
| INV-API-069 | Delete user | api/ | api/internal/handlers/users.go:35 | [p](procedures/api.md#users-delete) | TBD | untested | | [e](INV-API-069/) | | |
| INV-API-070 | Reset user password | api/ | api/internal/handlers/users.go:37 | [p](procedures/api.md#users-reset-password) | TBD | untested | deferred: T031 | [e](INV-API-070/) | | |
| INV-API-071 | List user role bindings | api/ | api/internal/handlers/users.go:40 | [p](procedures/api.md#users-bindings-list) | TBD | untested | | [e](INV-API-071/) | | |
| INV-API-072 | Add user role binding | api/ | api/internal/handlers/users.go:41 | [p](procedures/api.md#users-bindings-add) | TBD | untested | | [e](INV-API-072/) | | |
| INV-API-073 | Delete user role binding | api/ | api/internal/handlers/users.go:42 | [p](procedures/api.md#users-bindings-add) | TBD | untested | | [e](INV-API-073/) | | |
| INV-API-074 | List roles | api/ | api/internal/handlers/roles.go:22-23 | [p](procedures/api.md#roles-list) | TBD | untested | | [e](INV-API-074/) | | |
| INV-API-075 | Get permission catalog | api/ | api/internal/handlers/roles.go:26 | [p](procedures/api.md#roles-permissions-catalog) | TBD | untested | | [e](INV-API-075/) | | |
| INV-API-076 | Create custom role | api/ | api/internal/handlers/roles.go:27 | [p](procedures/api.md#roles-list) | TBD | untested | | [e](INV-API-076/) | | |
| INV-API-077 | Update role | api/ | api/internal/handlers/roles.go:28 | [p](procedures/api.md#roles-list) | TBD | untested | | [e](INV-API-077/) | | |
| INV-API-078 | Delete role | api/ | api/internal/handlers/roles.go:29 | [p](procedures/api.md#roles-list) | TBD | untested | | [e](INV-API-078/) | | |
| INV-API-079 | Get cluster info | api/ | api/internal/handlers/cluster.go:29 | [p](procedures/api.md#cluster-view) | TBD | untested | | [e](INV-API-079/) | | |
| INV-API-080 | Get cluster metadata | api/ | api/internal/handlers/cluster.go:30 | [p](procedures/api.md#cluster-info) | TBD | untested | | [e](INV-API-080/) | | |
| INV-API-081 | Get cluster stats | api/ | api/internal/handlers/cluster.go:31 | [p](procedures/api.md#cluster-stats) | TBD | untested | | [e](INV-API-081/) | | |
| INV-API-082 | Join node to cluster | api/ | api/internal/handlers/cluster_actions.go:39 | [p](procedures/api.md#cluster-join-node) | TBD | untested | | [e](INV-API-082/) | | |
| INV-API-083 | Download kubeconfig | api/ | api/internal/handlers/cluster_actions.go:40 | [p](procedures/api.md#cluster-download-kubeconfig) | TBD | untested | | [e](INV-API-083/) | | |
| INV-API-084 | List remote clusters | api/ | api/internal/handlers/clusters.go:27-29 | [p](procedures/api.md#clusters-list) | TBD | untested | | [e](INV-API-084/) | | |
| INV-API-085 | Register remote cluster | api/ | api/internal/handlers/clusters.go:28 | [p](procedures/api.md#clusters-list) | TBD | untested | | [e](INV-API-085/) | | |
| INV-API-086 | Deregister remote cluster | api/ | api/internal/handlers/clusters.go:29 | [p](procedures/api.md#clusters-list) | TBD | untested | | [e](INV-API-086/) | | |
| INV-API-087 | List installed modules | api/ | api/internal/handlers/modules.go:38-39 | [p](procedures/api.md#modules-list-installed) | TBD | untested | | [e](INV-API-087/) | | |
| INV-API-088 | Install module | api/ | api/internal/handlers/modules.go:40 | [p](procedures/api.md#modules-list-installed) | TBD | untested | | [e](INV-API-088/) | | |
| INV-API-089 | List module sources | api/ | api/internal/handlers/modules.go:41 | [p](procedures/api.md#modules-list-sources) | TBD | untested | | [e](INV-API-089/) | | |
| INV-API-090 | Create module source | api/ | api/internal/handlers/modules.go:42 | [p](procedures/api.md#modules-list-sources) | TBD | untested | | [e](INV-API-090/) | | |
| INV-API-091 | Update module source | api/ | api/internal/handlers/modules.go:43 | [p](procedures/api.md#modules-list-sources) | TBD | untested | | [e](INV-API-091/) | | |
| INV-API-092 | Delete module source | api/ | api/internal/handlers/modules.go:44 | [p](procedures/api.md#modules-list-sources) | TBD | untested | | [e](INV-API-092/) | | |
| INV-API-093 | Upload module bundle | api/ | api/internal/handlers/modules.go:45 | [p](procedures/api.md#modules-list-sources) | TBD | untested | | [e](INV-API-093/) | | |
| INV-API-094 | Delete uploaded module | api/ | api/internal/handlers/modules.go:46 | [p](procedures/api.md#modules-list-sources) | TBD | untested | | [e](INV-API-094/) | | |
| INV-API-095 | Get merged module catalog | api/ | api/internal/handlers/modules.go:47 | [p](procedures/api.md#modules-catalog) | TBD | untested | | [e](INV-API-095/) | | |
| INV-API-096 | List module archetypes | api/ | api/internal/handlers/modules.go:49 | [p](procedures/api.md#modules-list-installed) | TBD | untested | | [e](INV-API-096/) | | |
| INV-API-097 | Scaffold module | api/ | api/internal/handlers/modules.go:50 | [p](procedures/api.md#modules-list-installed) | TBD | untested | | [e](INV-API-097/) | | |
| INV-API-098 | Validate module | api/ | api/internal/handlers/modules.go:51 | [p](procedures/api.md#modules-list-installed) | TBD | untested | | [e](INV-API-098/) | | |
| INV-API-099 | Preview module | api/ | api/internal/handlers/modules.go:52 | [p](procedures/api.md#modules-list-installed) | TBD | untested | | [e](INV-API-099/) | | |
| INV-API-100 | Export module | api/ | api/internal/handlers/modules.go:53 | [p](procedures/api.md#modules-list-installed) | TBD | untested | | [e](INV-API-100/) | | |
| INV-API-101 | Get installed module | api/ | api/internal/handlers/modules.go:55 | [p](procedures/api.md#modules-list-installed) | TBD | untested | | [e](INV-API-101/) | | |
| INV-API-102 | Upgrade module | api/ | api/internal/handlers/modules.go:56 | [p](procedures/api.md#modules-list-installed) | TBD | untested | | [e](INV-API-102/) | | |
| INV-API-103 | Uninstall module | api/ | api/internal/handlers/modules.go:57 | [p](procedures/api.md#modules-list-installed) | TBD | untested | | [e](INV-API-103/) | | |
| INV-API-104 | List mod registry providers | api/ | api/internal/handlers/registry.go:44 | [p](procedures/api.md#servers-list) | TBD | untested | | [e](INV-API-104/) | | |
| INV-API-105 | Search mod registry | api/ | api/internal/handlers/registry.go:45 | [p](procedures/api.md#servers-list) | TBD | untested | | [e](INV-API-105/) | | |
| INV-API-106 | Get mod versions | api/ | api/internal/handlers/registry.go:46 | [p](procedures/api.md#servers-list) | TBD | untested | | [e](INV-API-106/) | | |
| INV-API-107 | Get modpack dependencies | api/ | api/internal/handlers/registry.go:47 | [p](procedures/api.md#servers-list) | TBD | untested | | [e](INV-API-107/) | | |
| INV-API-108 | Install modpack | api/ | api/internal/handlers/registry.go:48 | [p](procedures/api.md#servers-list) | TBD | untested | | [e](INV-API-108/) | | |
| INV-API-109 | Check mod updates | api/ | api/internal/handlers/mod_updates.go:32 | [p](procedures/api.md#servers-list) | TBD | untested | | [e](INV-API-109/) | | |
| INV-API-110 | Get server mod IDs | api/ | api/internal/handlers/mod_ids.go:63 | [p](procedures/api.md#servers-list) | TBD | untested | | [e](INV-API-110/) | | |
| INV-API-111 | Update server mod IDs | api/ | api/internal/handlers/mod_ids.go:64 | [p](procedures/api.md#servers-list) | TBD | untested | | [e](INV-API-111/) | | |
| INV-API-112 | Get tunnel credentials | api/ | api/internal/handlers/tunnelcreds.go:32 | [p](procedures/api.md#servers-list) | TBD | untested | | [e](INV-API-112/) | | |
| INV-API-113 | Set tunnel credentials | api/ | api/internal/handlers/tunnelcreds.go:31 | [p](procedures/api.md#servers-list) | TBD | untested | | [e](INV-API-113/) | | |
| INV-API-114 | Delete tunnel credentials | api/ | api/internal/handlers/tunnelcreds.go:33 | [p](procedures/api.md#servers-list) | TBD | untested | | [e](INV-API-114/) | | |
| INV-API-115 | List backup destinations | api/ | api/internal/handlers/destinations.go:44-47 | [p](procedures/api.md#backup-destinations-list) | TBD | untested | | [e](INV-API-115/) | | |
| INV-API-116 | Create backup destination | api/ | api/internal/handlers/destinations.go:45 | [p](procedures/api.md#backup-destinations-list) | TBD | untested | | [e](INV-API-116/) | | |
| INV-API-117 | Get backup destination | api/ | api/internal/handlers/destinations.go:46 | [p](procedures/api.md#backup-destinations-list) | TBD | untested | | [e](INV-API-117/) | | |
| INV-API-118 | Delete backup destination | api/ | api/internal/handlers/destinations.go:47 | [p](procedures/api.md#backup-destinations-list) | TBD | untested | | [e](INV-API-118/) | | |
| INV-API-119 | Watch events (SSE) | api/ | api/internal/handlers/events.go:24 | [p](procedures/api.md#events-sse) | TBD | untested | | [e](INV-API-119/) | | |
| INV-API-120 | Get pod events | api/ | api/internal/handlers/pod_events.go:26 | [p](procedures/api.md#servers-list) | TBD | untested | | [e](INV-API-120/) | | |
| INV-API-121 | WebSocket console | api/ | api/internal/ws/dialer.go:41 | [p](procedures/api.md#ws-console) | TBD | untested | deferred: T031 | [e](INV-API-121/) | | |
| INV-API-122 | WebSocket logs | api/ | api/internal/ws/dialer.go:42 | [p](procedures/api.md#ws-logs) | TBD | untested | deferred: T031 | [e](INV-API-122/) | | |
| INV-API-123 | Download logs | api/ | api/internal/ws/dialer.go:43 | [p](procedures/api.md#ws-logs) | TBD | untested | | [e](INV-API-123/) | | |
| INV-API-124 | List server files | api/ | api/internal/ws/dialer.go:53 | [p](procedures/api.md#files-list) | TBD | untested | | [e](INV-API-124/) | | |
| INV-API-125 | Read server file | api/ | api/internal/ws/dialer.go:54 | [p](procedures/api.md#files-read) | TBD | untested | | [e](INV-API-125/) | | |
| INV-API-126 | Download server file | api/ | api/internal/ws/dialer.go:55 | [p](procedures/api.md#files-read) | TBD | untested | | [e](INV-API-126/) | | |
| INV-API-127 | Write server file | api/ | api/internal/ws/dialer.go:56 | [p](procedures/api.md#files-read) | TBD | untested | | [e](INV-API-127/) | | |
| INV-API-128 | Upload server file | api/ | api/internal/ws/dialer.go:57 | [p](procedures/api.md#files-upload) | TBD | untested | | [e](INV-API-128/) | | |
| INV-API-129 | Create directory | api/ | api/internal/ws/dialer.go:58 | [p](procedures/api.md#files-read) | TBD | untested | | [e](INV-API-129/) | | |
| INV-API-130 | Delete file/directory | api/ | api/internal/ws/dialer.go:59 | [p](procedures/api.md#files-read) | TBD | untested | | [e](INV-API-130/) | | |
| INV-API-131 | List server players | api/ | api/internal/ws/dialer.go:62 | [p](procedures/api.md#players-list) | TBD | untested | | [e](INV-API-131/) | | |
| INV-API-132 | List banned players | api/ | api/internal/ws/dialer.go:63 | [p](procedures/api.md#players-list) | TBD | untested | | [e](INV-API-132/) | | |
| INV-API-133 | Kick player | api/ | api/internal/ws/dialer.go:64 | [p](procedures/api.md#players-kick) | TBD | untested | deferred: T031 | [e](INV-API-133/) | | |
| INV-API-134 | Ban player | api/ | api/internal/ws/dialer.go:65 | [p](procedures/api.md#players-kick) | TBD | untested | deferred: T031 | [e](INV-API-134/) | | |
| INV-API-135 | Unban player | api/ | api/internal/ws/dialer.go:66 | [p](procedures/api.md#players-kick) | TBD | untested | deferred: T031 | [e](INV-API-135/) | | |
| INV-API-136 | Get whitelist | api/ | api/internal/ws/dialer.go:67 | [p](procedures/api.md#players-list) | TBD | untested | | [e](INV-API-136/) | | |
| INV-API-137 | Add whitelist entry | api/ | api/internal/ws/dialer.go:68 | [p](procedures/api.md#players-list) | TBD | untested | | [e](INV-API-137/) | | |
| INV-API-138 | Remove whitelist entry | api/ | api/internal/ws/dialer.go:69 | [p](procedures/api.md#players-list) | TBD | untested | | [e](INV-API-138/) | | |
| INV-API-139 | Run operator action | api/ | api/internal/ws/dialer.go:78 | [p](procedures/api.md#servers-list) | TBD | untested | | [e](INV-API-139/) | | |
| INV-API-140 | Get server status | api/ | api/internal/ws/dialer.go:79 | [p](procedures/api.md#servers-list) | TBD | untested | | [e](INV-API-140/) | | |
| INV-API-141 | List server mods | api/ | api/internal/ws/dialer.go:85 | [p](procedures/api.md#servers-list) | TBD | untested | | [e](INV-API-141/) | | |
| INV-API-142 | Install mod | api/ | api/internal/ws/dialer.go:86 | [p](procedures/api.md#servers-list) | TBD | untested | | [e](INV-API-142/) | | |
| INV-API-143 | Upload mod | api/ | api/internal/ws/dialer.go:87 | [p](procedures/api.md#servers-list) | TBD | untested | | [e](INV-API-143/) | | |
| INV-API-144 | Delete mod | api/ | api/internal/ws/dialer.go:88 | [p](procedures/api.md#servers-list) | TBD | untested | | [e](INV-API-144/) | | |
| INV-API-145 | Get system logs (API) | api/ | api/internal/handlers/systemlogs.go:24 | [p](procedures/api.md#system-logs-api) | TBD | untested | | [e](INV-API-145/) | | |
| INV-API-146 | Get system logs (Operator) | api/ | api/internal/handlers/systemlogs.go:24 | [p](procedures/api.md#system-logs-operator) | TBD | untested | | [e](INV-API-146/) | | |

Row count: 146
