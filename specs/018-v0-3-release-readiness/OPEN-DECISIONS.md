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

## OD-005: Upgrade baseline if kubelab runs a build newer than beta.8 (OPEN, raised by `/speckit-plan` 2026-09-23)

FR-015 requires the upgrade from the last beta (`v0.2.0-beta.8`) to be verified live. kubelab currently runs side-loaded images from a private tag. If that build already carries API database migrations newer than beta.8, installing public beta.8 first would be a **downgrade** over a newer schema. The migrations are forward-only (`api/internal/db/migrations/`), so this could break the API or lose data.

Round 0 checks this: compare the migration level in kubelab's API database with the highest migration shipped in `v0.2.0-beta.8`.

Options if kubelab is ahead:
- (a) Run the live upgrade from kubelab's current build to the RC, and rely on CI `e2e-upgrade` (baseline bumped to beta.8) for beta.8 → RC. This technically falls short of FR-015's "verified live".
- (b) Snapshot the database, reinstall kubelab's Gameplane release at beta.8 with a fresh database, seed it, then upgrade. Restore the real database afterwards. This is invasive to a long-lived cluster.
- (c) Temporarily provision a separate single-node cluster for the beta.8 → RC step only.

Needs the maintainer's choice. No option is assumed.

## OD-006: Real node-loss test on kubelab (OPEN, raised by `/speckit-plan` 2026-09-23)

FR-011 asks for node-loss behaviour. Actually taking a kubelab node down affects every pre-existing workload on it, which conflicts with FR-009. The plan's default (research R8) is to test drain behaviour by cordoning a node and evicting only the `audit018-` pod. A real node loss would run only with explicit run-time approval, on a worker that holds no pre-existing stateful game server. Otherwise the row is recorded as `blocked`, with the eviction test as its alternative.

Question: may a real node loss be simulated (e.g. stopping `k3s-agent` on one worker for a few minutes)? If so, on which node and in what time window?

## OD-007: kubelab not reachable from the audit machine (OPEN, 2026-09-23)

`~/kubelab.yaml` points at `https://kubelab-api:6443`, but `kubelab-api` doesn't resolve from the devbox ("no such host"). Every live step is blocked until this is fixed. The component reviews, the inventory and the known-bug import can go ahead in the meantime.

Question: is a VPN, `/etc/hosts` entry, or different kubeconfig needed?

## OD-008: Scope of the live tamper test for audit-chain integrity (OPEN, 2026-09-23)

FR-008 needs an active attempt to tamper with the audit log. The plan's default (`contracts/test-resources.md`) runs the tamper test against a **copy** of the API database snapshot, never the live database. Doing it live would mean editing rows in kubelab's real `audit_events` table, even `audit018-` ones, and that permanently breaks the real chain from that point on.

Question: is the copy-based test acceptable as the "active violation attempt"? Or do you want a live tamper, in which case how should the broken chain be recovered afterwards?
