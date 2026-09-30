# Draft inventory rows: CRD (rc.0)

Links are relative to audit/inventory.md.

| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|
| INV-CRD-001 | Create server from template | operator/ | operator/api/v1alpha1/gameserver_types.go:1-100 | [p](procedures/crd.md#gameserver-create-from-template) | TBD | untested | | [e](INV-CRD-001/) | | |
| INV-CRD-002 | Phase transition Pending → Starting | operator/ | operator/internal/controller/gameserver_status.go:268-293 | [p](procedures/crd.md#gameserver-phase-pending-to-starting) | TBD | untested | | [e](INV-CRD-002/) | | |
| INV-CRD-003 | Phase transition Starting → Running | operator/ | operator/internal/controller/gameserver_status.go:268-293 | [p](procedures/crd.md#gameserver-phase-starting-to-running) | TBD | untested | | [e](INV-CRD-003/) | | |
| INV-CRD-004 | Suspend (soft stop) game server | operator/ | operator/internal/controller/gameserver_controller.go:255-470 | [p](procedures/crd.md#gameserver-suspend) | TBD | untested | | [e](INV-CRD-004/) | | |
| INV-CRD-005 | Unsuspend and wake game server | operator/ | operator/internal/controller/gameserver_controller.go:255-470 | [p](procedures/crd.md#gameserver-unsuspend-wake) | TBD | untested | | [e](INV-CRD-005/) | | |
| INV-CRD-006 | Restart game server pod | operator/ | operator/internal/controller/gameserver_restart.go:1-50 | [p](procedures/crd.md#gameserver-restart) | TBD | untested | | [e](INV-CRD-006/) | | |
| INV-CRD-007 | Switch server version | operator/ | operator/internal/controller/gameserver_version.go:1-100 | [p](procedures/crd.md#gameserver-version-switch) | TBD | untested | | [e](INV-CRD-007/) | | |
| INV-CRD-008 | Wipe server data volume | operator/ | operator/internal/controller/gameserver_wipe.go:1-150 | [p](procedures/crd.md#gameserver-wipe-data-volume) | TBD | untested | | [e](INV-CRD-008/) | | |
| INV-CRD-009 | Idle auto-sleep (zero players) | operator/ | operator/internal/controller/gameserver_idle.go:1-150 | [p](procedures/crd.md#gameserver-idle-auto-sleep) | TBD | untested | | [e](INV-CRD-009/) | | |
| INV-CRD-010 | Wake via scheduled window | operator/ | operator/internal/controller/gameserver_idle.go:1-150 | [p](procedures/crd.md#gameserver-idle-wake-window) | TBD | untested | | [e](INV-CRD-010/) | | |
| INV-CRD-011 | Wake on player connect (sentinel) | operator/ | operator/internal/controller/gameserver_sentinel.go:1-200 | [p](procedures/crd.md#gameserver-idle-wake-on-connect) | TBD | untested | user-facing; sentinel pod injected | [e](INV-CRD-011/) | | |
| INV-CRD-012 | Delete game server with finalizer | operator/ | operator/internal/controller/gameserver_controller.go:255-470 | [p](procedures/crd.md#gameserver-delete-with-finalizer) | TBD | untested | | [e](INV-CRD-012/) | | |
| INV-CRD-013 | Phase Failed (crash loop) | operator/ | operator/internal/controller/gameserver_status.go:104-135 | [p](procedures/crd.md#gameserver-failed-phase-crash-loop) | TBD | untested | | [e](INV-CRD-013/) | | |
| INV-CRD-014 | Create game template | operator/ | operator/api/v1alpha1/gametemplate_types.go:1-200 | [p](procedures/crd.md#gametemplate-create) | TBD | untested | | [e](INV-CRD-014/) | | |
| INV-CRD-015 | Create backup (Pending → Running) | operator/ | operator/internal/controller/backup_controller.go:173-263 | [p](procedures/crd.md#backup-create-and-run) | TBD | untested | | [e](INV-CRD-015/) | | |
| INV-CRD-016 | Backup phase Running → Succeeded | operator/ | operator/internal/controller/backup_controller.go:316-450 | [p](procedures/crd.md#backup-create-and-run) | TBD | untested | | [e](INV-CRD-016/) | | |
| INV-CRD-017 | Backup phase failure (Failed) | operator/ | operator/internal/controller/backup_controller.go:584-607 | [p](procedures/crd.md#backup-failure-and-phase) | TBD | untested | | [e](INV-CRD-017/) | | |
| INV-CRD-018 | Restore Pending → Suspending | operator/ | operator/internal/controller/restore_controller.go:40-200 | [p](procedures/crd.md#restore-create-and-suspend-server) | TBD | untested | | [e](INV-CRD-018/) | | |
| INV-CRD-019 | Restore Suspending → Running | operator/ | operator/internal/controller/restore_controller.go:40-200 | [p](procedures/crd.md#restore-create-and-suspend-server) | TBD | untested | | [e](INV-CRD-019/) | | |
| INV-CRD-020 | Restore Running → Resuming → Succeeded | operator/ | operator/internal/controller/restore_controller.go:40-200 | [p](procedures/crd.md#restore-complete-and-resume) | TBD | untested | | [e](INV-CRD-020/) | | |
| INV-CRD-021 | BackupSchedule create and schedule | operator/ | operator/internal/controller/backupschedule_controller.go:49-200 | [p](procedures/crd.md#backup-schedule-create) | TBD | untested | | [e](INV-CRD-021/) | | |
| INV-CRD-022 | BackupSchedule tick creates Backup | operator/ | operator/internal/controller/backupschedule_controller.go:49-200 | [p](procedures/crd.md#backup-schedule-tick-creates-backup) | TBD | untested | | [e](INV-CRD-022/) | | |
| INV-CRD-023 | BackupSchedule retention prunes backups | operator/ | operator/internal/controller/backupschedule_controller.go:49-200 | [p](procedures/crd.md#backup-schedule-retention-prunes) | TBD | untested | | [e](INV-CRD-023/) | | |
| INV-CRD-024 | Module Pending phase (version resolving) | operator/ | operator/internal/controller/module_controller.go:58-150 | [p](procedures/crd.md#module-create-pending) | TBD | untested | | [e](INV-CRD-024/) | | |
| INV-CRD-025 | Module Pulling → Ready (materialize template) | operator/ | operator/internal/controller/module_controller.go:58-150 | [p](procedures/crd.md#module-pulling-to-ready) | TBD | untested | | [e](INV-CRD-025/) | | |
| INV-CRD-026 | Module bad signature fails (cosign verify) | operator/ | operator/internal/controller/module_controller.go:58-150 | [p](procedures/crd.md#module-bad-signature-fails) | TBD | untested | blocked candidate: requires external signed OCI bundle; alternative: test with unsigned bundle in local OCI source | [e](INV-CRD-026/) | | |
| INV-CRD-027 | ModuleSource OCI periodic refresh/sync | operator/ | operator/internal/controller/modulesource_controller.go:51-150 | [p](procedures/crd.md#modulesource-oci-sync) | TBD | untested | | [e](INV-CRD-027/) | | |
| INV-CRD-028 | ModuleSource git sync error | operator/ | operator/internal/controller/modulesource_controller.go:51-150 | [p](procedures/crd.md#modulesource-git-sync-error) | TBD | untested | blocked candidate: requires intentional git repo failure; alternative: observe sync timeout with slow network | [e](INV-CRD-028/) | | |
| INV-CRD-029 | Cluster register and health check | operator/ | operator/internal/controller/cluster_controller.go:31-170 | [p](procedures/crd.md#cluster-register-and-health-check) | TBD | untested | user-facing; multicluster feature | [e](INV-CRD-029/) | | |
| INV-CRD-030 | Cluster health check failure (Unhealthy) | operator/ | operator/internal/controller/cluster_controller.go:130-165 | [p](procedures/crd.md#cluster-health-check-unreachable) | TBD | untested | user-facing; multicluster feature | [e](INV-CRD-030/) | | |
| INV-CRD-031 | NetworkCapture Pending phase | operator/ | operator/internal/controller/networkcapture_controller.go:139-250 | [p](procedures/crd.md#networkcapture-create-pending) | TBD | untested | user-facing; requires capture sidecar enabled | [e](INV-CRD-031/) | | |
| INV-CRD-032 | NetworkCapture Pending → Running | operator/ | operator/internal/controller/networkcapture_controller.go:139-250 | [p](procedures/crd.md#networkcapture-start-running) | TBD | untested | user-facing; requires capture sidecar enabled | [e](INV-CRD-032/) | | |
| INV-CRD-033 | NetworkCapture stop and complete | operator/ | operator/internal/controller/networkcapture_controller.go:139-250 | [p](procedures/crd.md#networkcapture-stop-completed) | TBD | untested | user-facing; requires capture sidecar enabled | [e](INV-CRD-033/) | | |
| INV-CRD-034 | NetworkCapture failure (sidecar crash) | operator/ | operator/internal/controller/networkcapture_controller.go:139-250 | [p](procedures/crd.md#networkcapture-failed-sidecar-crash) | TBD | untested | user-facing; ephemeral container restart behavior | [e](INV-CRD-034/) | | |
| INV-CRD-035 | NetworkCapture auto-delete after TTL | operator/ | operator/internal/controller/networkcapture_controller.go:139-250 | [p](procedures/crd.md#networkcapture-expired-auto-delete) | TBD | untested | user-facing; garbage collection | [e](INV-CRD-035/) | | |

Row count: 35

Unsure about:
- Module bad signature (INV-CRD-026): requires production signed OCI bundle; marked as blocked candidate with fallback to test local unsigned bundle rejection.
- ModuleSource git sync error (INV-CRD-028): requires intentional git repo failure for testing; marked as blocked candidate with fallback to timeout simulation.
- NetworkCapture procedures (INV-CRD-031..035): require capture.enabled=true on cluster, capture sidecar injection, and ephemeral container readiness. Automatable if test cluster has these prerequisites; otherwise require manual prerequisites.
- Multicluster (INV-CRD-029..030): Cluster CRD is cluster-scoped and requires a second kubeconfig; kubelab may not have this setup. Check OD-015/OD-016 for permission/network status.
