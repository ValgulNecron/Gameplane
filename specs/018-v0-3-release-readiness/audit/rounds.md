# Audit Rounds

One section per round (contracts/audit-records.md). Database snapshots are stored off-git; only their path is recorded here.

## rc.0

Pre-RC round: component reviews, inventory enumeration and the known-bug import. No release candidate is deployed.

- **Tag**: none (pre-RC)
- **Helm overrides vs baseline**: none
- **DB snapshot location (off-git)**: `~/gameplane-audit-018/db-snapshots/`
- **kubelab connectivity (T004, OD-007)**: ok on 2026-09-23. `getent hosts kubelab-api` → `10.43.153.36 kubelab-control.kubelab.svc.cluster.local`. `kubectl get nodes -o wide` → `kubelab-control` (control-plane), `kubelab-worker-1`, `kubelab-worker-2`, all Ready, k3s `v1.36.2+k3s1`. Helm release `gameplane` in namespace `gameplane-system`, revision 6, chart `gameplane-0.2.0-beta.8`.
- **Rows run**: 0
- **New findings**: see [findings.md](findings.md)
- **Cleanup**: n/a (no test resources created)

### Test resources

| Name | Kind | Created by | Round | Removed |
|------|------|------------|-------|---------|
