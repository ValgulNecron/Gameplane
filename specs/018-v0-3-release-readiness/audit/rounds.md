# Audit Rounds

One section per round (contracts/audit-records.md). Database snapshots are stored off-git; only their path is recorded here.

## Round setup / teardown checklist (OD-021)

Every round that runs `agent.md`/`api.md`/`modules.md`/`web.md`/`crd.md` rows needs some or all of these fixtures, per `procedures/conventions.md`. Record what was actually created/removed under that round's **Cleanup** field. Not every round needs every item — only create what that round's rows require.

Setup (before running rows):
1. `audit018-restic`: derive a Deployment + Service named `audit018-restic` from `test/e2e/fixtures/restic-server.yaml` (renamed from `gameplane-test-restic`, labeled `gameplane.io/audit: "018"`), then create the `audit018-restic` Secret in `gameplane-games` pointing `repo` at `rest:http://audit018-restic.gameplane-system.svc:8000/`, and an `audit018-restic` NetworkPolicy in `gameplane-games` (labeled `gameplane.io/audit: "018"`) allowing egress to pods labeled `app.kubernetes.io/name: audit018-restic` in `gameplane-system` on TCP 8000, since `default-deny-egress` (networkPolicies enabled) would otherwise block backup Job pods from reaching it (OD-021 items 9/16).
2. `audit018-games2` namespace plus one small `audit018-` GameServer in it (item 14).
3. `capture.enabled=true`: `helm upgrade` override, previous value recorded here (item 17).
4. `audit018-registry` (in-cluster OCI registry) plus an `audit018-` OCI ModuleSource; push one signed and one unsigned bundle (items 6/21).
5. CSI snapshot support: install `csi-driver-host-path` and the snapshot controller/CRDs (item 20).
6. `audit018-collab` account: role `audit018-norole` (no permissions), user `audit018-collab`, collaborator grant on one `audit018-` server; credentials in `~/gameplane-audit-018/collab.env` (item 15).

Teardown (after running rows, reverse order):
1. Remove the `audit018-collab` collaborator grant, delete the `audit018-collab` user, delete the `audit018-norole` role, delete `~/gameplane-audit-018/collab.env`.
2. Uninstall the CSI snapshot controller/CRDs and `csi-driver-host-path`.
3. Delete the `audit018-` OCI ModuleSource, then the `audit018-registry` Deployment/Service.
4. Restore `capture.enabled` to its prior value (`helm upgrade --set capture.enabled=false`, or the round's recorded prior value); snapshot-diff the config it affects.
5. Delete the `audit018-games2` GameServer, then the `audit018-games2` namespace.
6. Delete the `audit018-restic` Secret, then the `audit018-restic` NetworkPolicy, then the `audit018-restic` Deployment/Service.

## rc.0

Pre-RC round: component reviews, inventory enumeration and the known-bug import. No release candidate is deployed.

- **Tag**: none (pre-RC)
- **Helm overrides vs baseline**: none
- **DB snapshot location (off-git)**: `~/gameplane-audit-018/db-snapshots/`
- **kubelab connectivity (T004, OD-007)**: ok on 2026-09-23. `getent hosts kubelab-api` → `10.43.153.36 kubelab-control.kubelab.svc.cluster.local`. `kubectl get nodes -o wide` → `kubelab-control` (control-plane), `kubelab-worker-1`, `kubelab-worker-2`, all Ready, k3s `v1.36.2+k3s1`. Helm release `gameplane` in namespace `gameplane-system`, revision 6, chart `gameplane-0.2.0-beta.8`.
- **Rows run**: 0
- **New findings**: see [findings.md](findings.md)
- **Cleanup**: Delete `audit018-admin` account (see test resources table).
- **Admin bootstrap (T012, OD-015)**: `audit018-admin` was created via `bootstrap-admin` on 2026-09-24 15:15 UTC (`kubectl exec -n gameplane-system deploy/gameplane-api -- /api bootstrap-admin --username audit018-admin --password-stdin`, no `--force`, no existing user touched). Written straight into the live DB, so there is no API audit event for its creation. Password kept off-git in `~/gameplane-audit-018/admin.env` (mode 600). To be deleted at cleanup.

### Test resources

| Name | Kind | Created by | Round | Removed |
|------|------|------------|-------|---------|
| audit018-admin | User | bootstrap-admin cmd | rc.0 | pending |
