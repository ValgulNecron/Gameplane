# Open Decisions: v0.3 Release Readiness Audit

These values are unsettled. They are not contracts; resolve them during `/speckit-clarify` or `/speckit-plan`.

## OD-001: How the release candidate is deployed onto kubelab — RESOLVED 2026-09-23

Decision: public pre-release tag (option b). Recorded in `spec.md` Clarifications, FR-018, FR-019. Each publication needs explicit maintainer approval; kubelab's original image settings are noted for restoration.

Original question:

kubelab's live Helm release uses side-loaded images and a git module source that differ from the repository's local-development defaults. Deploying without care could repoint the registry, tag, or module source. Options: (a) side-load release-candidate images and `helm upgrade --reuse-values` with only intended overrides; (b) publish candidate images to the public registry under a pre-release tag. Needs the maintainer's choice.

## OD-002: Release-blocking severity threshold — RESOLVED 2026-09-23

Decision: every finding of any severity blocks the release; only "not a defect" or roadmap-cited out-of-scope closures are allowed. Recorded in `spec.md` Clarifications, FR-005, FR-007, SC-003, SC-009. Severity only orders the fix work.

## OD-003: Meaning of "no longer beta" — RESOLVED 2026-09-23

Decision: drop the beta suffix (v0.3.0), status wording becomes pre-v1 release, live upgrade from last beta, remaining caveats stay in the roadmap. Recorded in `spec.md` Clarifications, FR-015, FR-016. Exact README/roadmap wording is a planning detail.

## OD-004: Where findings and the report live — RESOLVED 2026-09-23

Decision: files inside this spec folder. Recorded in `spec.md` Clarifications and FR-020.

## OD-005: Upgrade baseline if kubelab runs a build newer than beta.8 — RESOLVED 2026-09-23

Decision: option (b). Snapshot the real database, reinstall kubelab's Gameplane release at public `v0.2.0-beta.8` with a fresh database, seed `audit018-` state, upgrade to the RC, verify, then restore the real database. Applies only if round 0 finds kubelab's schema is ahead of beta.8. Otherwise kubelab is simply moved to public beta.8 first. Recorded in research R6 and contracts/rc-deploy.md §2a.

Original question:

FR-015 requires the upgrade from the last beta (`v0.2.0-beta.8`) to be verified live. kubelab currently runs side-loaded images from a private tag. If that build already carries API database migrations newer than beta.8, installing public beta.8 first would be a **downgrade** over a newer schema. The migrations are forward-only (`api/internal/db/migrations/`), so this could break the API or lose data.

Round 0 checks this: compare the migration level in kubelab's API database with the highest migration shipped in `v0.2.0-beta.8`.

Options if kubelab is ahead:
- (a) Run the live upgrade from kubelab's current build to the RC, and rely on CI `e2e-upgrade` (baseline bumped to beta.8) for beta.8 → RC. This technically falls short of FR-015's "verified live".
- (b) Snapshot the database, reinstall kubelab's Gameplane release at beta.8 with a fresh database, seed it, then upgrade. Restore the real database afterwards. This is invasive to a long-lived cluster.
- (c) Temporarily provision a separate single-node cluster for the beta.8 → RC step only.

Needs the maintainer's choice. No option is assumed.

## OD-006: Real node-loss test on kubelab — RESOLVED 2026-09-23

Decision: yes, go ahead without asking again. Stop `k3s-agent` for a few minutes on a worker that holds no pre-existing stateful game server, then restart it. Eviction on a cordoned node is still tested as the drain case. Recorded in research R8 and contracts/test-resources.md.

Original question:

FR-011 asks for node-loss behaviour. Actually taking a kubelab node down affects every pre-existing workload on it, which conflicts with FR-009. The plan's default (research R8) is to test drain behaviour by cordoning a node and evicting only the `audit018-` pod. A real node loss would run only with explicit run-time approval, on a worker that holds no pre-existing stateful game server. Otherwise the row is recorded as `blocked`, with the eviction test as its alternative.

Question: may a real node loss be simulated (e.g. stopping `k3s-agent` on one worker for a few minutes)? If so, on which node and in what time window?

## OD-007: kubelab not reachable from the audit machine — RESOLVED 2026-09-23

Decision: the maintainer will fix networking on the devbox. Until `kubectl get nodes` works, work that needs no cluster goes ahead: reviews, inventory, known-bug import. Live rows wait.

Update 2026-09-23: networking fixed. `kubelab-api` now resolves to `10.43.153.36` (`kubelab-control.kubelab.svc.cluster.local`), and `kubectl get nodes` lists `kubelab-control`, `kubelab-worker-1` and `kubelab-worker-2`, all Ready, on k3s `v1.36.2+k3s1`. Live work is unblocked.

Original question:

`~/kubelab.yaml` points at `https://kubelab-api:6443`, but `kubelab-api` doesn't resolve from the devbox ("no such host"). Every live step is blocked until this is fixed. The component reviews, the inventory and the known-bug import can go ahead in the meantime.

Question: is a VPN, `/etc/hosts` entry, or different kubeconfig needed?

## OD-008: Scope of the live tamper test for audit-chain integrity — RESOLVED 2026-09-23

Decision: live tamper on `audit018-` rows in kubelab's real `audit_events` table. How to recover the chain afterwards is still open: see OD-009.

Original question:

FR-008 needs an active attempt to tamper with the audit log. The plan's default (`contracts/test-resources.md`) runs the tamper test against a **copy** of the API database snapshot, never the live database. Doing it live would mean editing rows in kubelab's real `audit_events` table, even `audit018-` ones, and that permanently breaks the real chain from that point on.

Question: is the copy-based test acceptable as the "active violation attempt"? Or do you want a live tamper, in which case how should the broken chain be recovered afterwards?

## OD-009: Recovering the audit chain after the live tamper test — RESOLVED 2026-09-23

Decision: option (a). Take a database snapshot right before each tamper and restore it straight after, then confirm `Verify` is ok again. Scope: all three tamper types, each on `audit018-` rows: UPDATE a row, DELETE a middle row, and truncate the tail. Audit events written between the snapshot and the restore are lost, so run the test in a quiet window with no other audit activity going on. Recorded in contracts/test-resources.md.

Original question:

OD-008 chose a live tamper. Once an `audit018-` row in kubelab's real `audit_events` table is edited, `Verify` reports a break from that row onwards. The chain design keeps a checkpoint and a head anchor (`api/internal/audit/audit.go:263-296`), but no documented re-anchoring procedure exists.

Options, for the maintainer to choose:
- (a) Take a database snapshot right before the tamper and restore it right after. Audit events written in between are lost.
- (b) Undo the edit by restoring the exact original row values, so the hashes match again. This works for an UPDATE but not a DELETE.
- (c) Leave the break in place and record it as a known, intentional break in `rounds.md`. The dashboard's tamper banner will then show permanently.
- (d) Add or use an admin re-anchor or checkpoint operation. That is product work and would need its own spec.

Also: is DELETE / tail-truncation in scope for the live test, or UPDATE only?

