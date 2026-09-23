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

---

## Pending maintainer input from `/speckit-implement` (opened 2026-09-23)

The items below are open. Work that doesn't depend on them carries on.

### RC-TAG-1: publish `v0.3.0-rc.1` — pending

- **Waiting on**: the rc.1 CHANGELOG PR (T013, branch `chore/018-rc1-changelog`) being merged, CI green on the merge commit, then your approval to tag (FR-019).
- **SHA to tag**: the merge commit of that PR on `master`. It's recorded here once known.
- **Command after approval**: `git tag -s v0.3.0-rc.1 <sha> -m "v0.3.0-rc.1" && git push origin v0.3.0-rc.1`. See OD-013 on signing.

### OD-010: RC-03 leaves out the `imported` status

`release-criteria.md` RC-03 is copied word for word from tasks.md T005: "zero findings in `open`, `fixing` or `fixed-unverified`". But data-model.md lists `imported` as a blocking status at go/no-go too, and the quickstart final-validation grep counts it. As written, a finding left `imported` would not fail RC-03.
- Question: add `imported` to RC-03? RC-03 was committed as written. Changing it now needs a dated `## Change log` line citing your decision.

### OD-011: should `hack/check-doc-versions.sh` match bare `X.Y.Z`?

T052 asks the checker to recognise `X.Y.Z` as well as `X.Y.Z-(beta|rc).N`. The 18 audited docs contain about 37 bare `v0.x.y` strings that aren't Gameplane versions. Most are dependency versions in `docs/dependencies.md` (controller-runtime `v0.24.1`, `k8s.io/*` `v0.37.0`, `golang.org/x/*`). A bare pattern would flag all of them.
- What was done: PR for T052 (branch `fix/018-doc-versions-checker`) adds multi-digit `[0-9]+` and `-rc.N` only. On current master that selects exactly the same strings as before. That's enough to catch stale `0.2.0-beta.N` / `0.3.0-rc.N` strings once `appVersion` becomes `0.3.0`.
- Options for bare versions: (a) leave them unmatched and document it (current PR); (b) match bare `X.Y.Z` only in a Gameplane context (`--version`, `gameplane:`, `/gameplane/<component>:v`, `Status:` lines); (c) match all bare `0.x.y` and add a `<!-- doc-versions: dependency -->` marker to dependency lines.
- Finding F-026 stays `fixing` until this is settled.

### OD-012: no test harness for `hack/check-doc-versions.sh` (test sign-off)

T052 says to add a fixture case to the checker's existing harness, or else log the gap. No harness exists: `hack/` holds only the four scripts, and nothing under `test/` or `.github/` exercises the checker with fixtures. Adding one is a new test, which needs your sign-off (CLAUDE.md override 1).
- Question: approve a small fixture harness (e.g. `hack/testdata/check-doc-versions/` plus a `make` target run in CI)?

### OD-013: tag signing on the devbox

Tasks T014, T057 and T073 use `git tag -s`. The devbox has no GPG secret key, and neither `user.signingkey` nor `tag.gpgsign` is set, so a signed tag can't be made here.
- Options: (a) you create and push each tag yourself; (b) you set up a signing key (GPG or SSH `gpg.format=ssh`) on the devbox; (c) use an annotated, unsigned tag (`git tag -a`). Images and chart are still cosign-signed by `release.yaml` either way.

### OD-014: CHANGELOG convention for release-candidate sections

T013 says the `## [0.3.0-rc.1]` section "summarises the unreleased entries". The rc.1 PR reads that as follows:
- the rc.1 section is a short summary with highlights;
- `## [Unreleased]` keeps the full list, which grows until T072 moves it into `## [0.3.0]`.

`## [Unreleased]` was also missing most user-facing changes merged since beta.8 (about 60 PRs). The rc.1 PR backfills it from the merged PR bodies, and this is recorded as a docs finding.
- Question: is "summary in rc sections, full list stays in Unreleased" the convention you want? The alternative is to move the entries out of Unreleased into each rc section.

### OD-015: admin access for the live API work (T012 and every live API row)

T012 needs "a single admin login". Later rows need admin, operator and viewer sessions. The devbox has no admin credentials, and kubelab stores none in a Secret. Only the Helm release, the agent CA and the agent client cert Secrets are in `gameplane-system`.
- Options: (a) you create an `audit018-admin` account in the dashboard, then put its password in `~/gameplane-audit-018/admin.env` (mode 600, off-git); (b) the audit runs `gameplane-api bootstrap-admin --username audit018-admin` inside the API pod, which writes a user row straight into the live DB without an API audit event; (c) you share an existing admin login (not recommended).
- Recommendation: (a).

### OD-016: reaching the API and dashboard on kubelab

The kubelab ingress host is `gameplane.local` (Ingress `gameplane-system/gameplane`), and it doesn't resolve from the devbox. The API and web Services are ClusterIP only.
- Options for API rows: (a) `kubectl port-forward -n gameplane-system svc/gameplane-api` from the devbox, which skips the ingress path; (b) an `/etc/hosts` entry for `gameplane.local` pointing at the Traefik address, which keeps the real user path.
- Also: Chrome MCP runs in your browser. Which URL should dashboard rows (T030) use?

### OD-017: no worker node is free of pre-existing game servers (node-loss test)

OD-006 approved stopping `k3s-agent` on "a worker that holds no pre-existing stateful game server". On 2026-09-23 both workers hold one:
- `kubelab-worker-1`: `soak-no-preference-0` and `squad-0` (`squad` is `Failed`, `ImagePullBackOff`)
- `kubelab-worker-2`: `soak-pool-west-0`
- `kubelab-control` hosts `mc-fabric-0` and `soak-bogus-pool-0`

Question: which node and time window may be used? Or should the `soak-*` servers count as non-stateful for this test? Until you answer, `INV-NODE-003` can only be `blocked`, with the cordon + eviction test as the alternative.

### OD-018: live steps blocked by this session's auto-mode safety check

On 2026-09-23, the harness's auto-mode safety check blocked a read-only `kubectl exec` into the kubelab API pod. The block holds for the rest of that session. Steps that need `kubectl exec` or `kubectl cp` into the API pod wait for a session run outside auto mode, or for you to run them:
- the DB migration level (T012)
- the `sqlite3 .backup` snapshots (T015, T048, T061)
- the tamper test (T048)

Read-only `kubectl get` / `helm get` steps still work.

Facts gathered for T012 in the meantime:
- kubelab runs side-loaded images `gameplane-test/api:016`, `gameplane-test/operator:016` and `gameplane-test/web:016`, under a Helm release labelled chart `gameplane-0.2.0-beta.8`, revision 6.
- `v0.2.0-beta.8` ships API migrations up to `006_share_links.sql`, and `master` has up to `011_user_theme_preferences.sql`.
- So kubelab's DB is very likely ahead of beta.8 (OD-005 path (b)). The DB itself must still confirm this.

### T054: `gameplane-module` tag for the v0.3.0 chart default

`charts/gameplane/values.yaml:473` pins the default git module source to `ref: v0.2.0-beta.6` (finding F-028).
- Question: which `gameplane-module` tag should v0.3.0 point to? Should a new tag (e.g. `v0.3.0`) be cut in that repo after the live module rows pass?

