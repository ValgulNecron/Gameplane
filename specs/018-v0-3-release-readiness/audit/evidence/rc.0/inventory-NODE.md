# Draft inventory rows: NODE (rc.0)

Links are relative to audit/inventory.md.

| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|
| INV-NODE-001 | Record baseline scheduling: audit018- GameServers and their node assignments | operator/ | operator/internal/controller/gameserver_controller.go:1 | [p](procedures/nodes.md#scheduling) | TBD | untested | | [e](INV-NODE-001/) | | |
| INV-NODE-002 | Cordon node, evict audit018- pods via eviction API, operator reschedules on surviving nodes (pre-existing pods untouched) | operator/ | operator/internal/controller/gameserver_controller.go:1 | [p](procedures/nodes.md#drain) | TBD | untested | | [e](INV-NODE-002/) | | |
| INV-NODE-003 | Stop k3s-agent on a worker for 5 min, operator recovers evicted audit018- pods (pre-existing servers stay running) | operator/ | operator/internal/controller/gameserver_controller.go:1, operator/api/v1alpha1/gameserver_types.go:1 | [p](procedures/nodes.md#node-loss) | TBD | blocked | blocked candidate: OD-017 — no worker holds zero pre-existing stateful game servers on 2026-09-23; alternative = drain test (INV-NODE-002) | [e](INV-NODE-003/) | | |

Row count: 3
