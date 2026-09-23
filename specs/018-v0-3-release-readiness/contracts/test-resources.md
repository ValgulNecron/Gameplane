# Contract: Live Test Resources on kubelab

Enforces FR-009 and SC-006. kubelab is long-lived and holds real servers, accounts and data.

## Identification

- Every resource the audit creates is named `audit018-<purpose>[-<n>]`. This covers GameServers, Backups, Restores, BackupSchedules, NetworkCaptures, ModuleSources, dashboard users, custom roles, share links, and notification sinks. Keep names within Kubernetes' 63-character label limit.
- Resources created with `kubectl` also carry the label `gameplane.io/audit: "018"`.
- Pre-existing resources are **never** targeted by a write. That includes restore targets, ownership transfers, and role changes. A procedure that needs an "existing" object creates its own `audit018-` copy first.

## Baseline snapshot (before round 0, and again after final cleanup)

Save the snapshot as `audit/kubelab-baseline.md`, redacted, together with the machine-readable `audit/evidence/baseline/*.json`:

| Scope | Captured fields |
|---|---|
| GameServer, GameTemplate, Backup, BackupSchedule, Restore, Module, ModuleSource, NetworkCapture, Cluster (all namespaces) | name, namespace, UID, `metadata.generation`, `status.phase` |
| PVCs in `gamesNamespace` and the API namespace | name, UID, capacity, bound volume |
| Nodes | name, roles, schedulable, kubelet version |
| Helm release | chart version, `helm get values` (secret-looking keys redacted), image refs |
| API | user list (username, role), role list, module-source list, auth-provider names, notification sinks (names only) |

**SC-006 comparison**: leave out `audit018-` objects and status-only fields. Every remaining UID and `generation` must match. The one exception is the Gameplane Helm release and its own Deployments, which change on purpose during RC upgrades (FR-018). A mismatch is an S1 finding.

## Cleanup

After each round:
1. Delete every `audit018-` resource through the same path that created it: API for API-created, `kubectl` for `kubectl`-created.
2. Check that none remain. Kubernetes: `kubectl get <kinds> -A -o name | grep audit018-` must be empty. API: the user, role and share lists hold no `audit018-` entries.
3. Record the result in `rounds.md`.

A resource that can't be removed is itself a finding.

## Disruptive operations

| Operation | Allowed form |
|---|---|
| Node drain | `kubectl cordon` + Eviction of the `audit018-` pod only + `kubectl uncordon`. A full `kubectl drain` is not allowed. |
| Node loss | Approved (OD-006): stop `k3s-agent` for a few minutes on a worker that holds no pre-existing stateful game server, then restart it. Record the node chosen, the window, and that pre-existing pods recovered. |
| API restart | Allowed: the API is the system under test. Record it in `rounds.md`. |
| Operator restart | Allowed. Record it in `rounds.md`. |
| Audit-chain tamper test | Live, on `audit018-` rows only (OD-008). Take a database snapshot first. Chain recovery follows OD-009. The test must not run until OD-009 is resolved. |
| Login brute force | Only against `audit018-` users, and last in the round. |
