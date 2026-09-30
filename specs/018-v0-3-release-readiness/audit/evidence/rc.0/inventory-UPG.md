# Draft inventory rows: UPG (rc.0)

Links are relative to audit/inventory.md.

| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|
| INV-UPG-001 | Upgrade preserves game server state and data across beta.8 to RC transition | api/, operator/ | api/cmd/main.go:1, operator/cmd/main.go:1 | [p](procedures/upgrade.md#upgrade-to-rc) | TBD | untested | | [e](INV-UPG-001/) | | |
| INV-UPG-002 | Admin login succeeds and audit Verify chain validates after upgrade | api/internal/handlers/, api/internal/audit/ | api/internal/handlers/auth.go:1, api/internal/audit/audit.go:1 | [p](procedures/upgrade.md#upgrade-to-rc) | TBD | untested | | [e](INV-UPG-002/) | | |
| INV-UPG-003 | API and operator restarts preserve reconciliation state and server data | operator/internal/controller/, api/internal/handlers/ | operator/internal/controller/gameserver_controller.go:1, api/cmd/main.go:1 | [p](procedures/upgrade.md#restart) | TBD | untested | | [e](INV-UPG-003/) | | |
| INV-UPG-004 | Helm rollback from RC to beta.8 serves API requests and lists seeded server | api/internal/handlers/, charts/gameplane/ | api/cmd/main.go:1, charts/gameplane/Chart.yaml:1 | [p](procedures/upgrade.md#rollback) | TBD | untested | | [e](INV-UPG-004/) | | |
| INV-UPG-005 | Real production database restores cleanly from snapshot with no schema drift | api/internal/db/ | api/internal/db/migrations/:1 | [p](procedures/upgrade.md#restore-real-db) | TBD | untested | | [e](INV-UPG-005/) | | |

Row count: 5
