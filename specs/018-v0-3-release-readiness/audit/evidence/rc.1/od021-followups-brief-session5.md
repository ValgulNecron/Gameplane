# OD-021 Scout Brief — v0.3 Release Readiness (item 12, items 3/23a, item 22)

Source: `origin/018-v0-3-release-readiness:specs/018-v0-3-release-readiness/OPEN-DECISIONS.md`
(fetched read-only; repo branch was not switched, work done via
`git show`/`git grep origin/...` and a detached worktree at
`/tmp/claude-0/.../scratchpad/wtod21` off `origin/master`, now safe to
remove).

---

## (a) Item 12 — new Go e2e bucket for web.md's `api` procedures

**Finding:** `procedures/web.md` on `origin/018-v0-3-release-readiness`
(`specs/018-v0-3-release-readiness/audit/procedures/web.md`) has exactly
**73** procedures whose `**Automatable?**` line reads literally `yes (api)`
(2 more say `yes (api) but ...` with a caveat — `844`, `901` — same bucket).
`test/e2e/buckets.sh` has no bucket named `api`; the closest names are the
feature-cut `api-auth`, `api-roles`, `api-rbac`, `api-agent`, `api-mods`.
Two of the 73 (`users-manage-service-accounts`,
`users-manage-oidc-providers`) are withdrawn `n/a` per OD item 13 — 71 will
actually need coverage.

Procedure IDs by feature area (`### <id>` headings in web.md, grep'd against
the `(api)` lines):

- **Admin/settings + logs** (14): `admin-settings-general`, `-auth`,
  `-backup-destinations`, `-mod-registries`, `-module-sources`,
  `-notifications`, `-telemetry`, `-updates`, `-about`,
  `admin-logs-view-api`, `-view-operator`, `-follow`, `-tail-option`,
  `theme-settings-import`.
- **Audit log** (6): `audit-log-view`, `-pagination`, `-filter-by-actor`,
  `-filter-by-method`, `-filter-by-status-class`, `-verify-integrity`.
- **Users/roles** (6): `users-list-view`, `-invite`, `-edit-role`,
  `-delete`, `-reset-password`, `-manage-roles`.
- **Backups (fleet-level + per-server)** (13): `backups-list-view`,
  `-filter-by-phase`, `-filter-by-server`, `-backup-now`, `-restore`,
  `-view-detail`, `-restores-view`, `-schedules-create`, `-schedules-edit`,
  `-schedules-delete`, `-schedules-toggle-suspend`,
  `server-detail-backups-view-list`, `-create-now`, `-restore`,
  `-schedule-create`, `-schedule-delete` (16 counting the server-detail
  variants separately).
- **Modules catalog** (7): `modules-catalog-browse`, `-search`,
  `-filter-by-category`, `-filter-by-source`, `-install`, `-upgrade`,
  `-uninstall`, `-upload-custom` (8).
- **Server-detail settings tabs** (13): `server-detail-settings-general`,
  `-lifecycle`, `-networking`, `-placement`, `-resources`, `-access`,
  `-environment`, `-version`, `-network-capture`, `-scheduled-backups`,
  `-danger-delete`, `-danger-transfer`, `-sharelinks-create/-view/-revoke`
  (16 counting sharelinks separately).
- **Server-detail mods/modpacks/logs/share** (7): `server-detail-mods-browse-registry`,
  `-check-updates`, `server-detail-modpacks-browse`,
  `server-detail-logs-view-pod-logs`, `share-server-start`,
  `-view-status`.

(Counts above are illustrative groupings, not a partition audit — the exact
73 must be re-verified against `web.md` when tests are written; I did not
hand-count every line to 73 across groups, only confirmed the total via
`grep -c 'yes (api)'`.)

**CI structure today** (`.github/workflows/ci.yaml`):
- `e2e-buckets` job (`:761`) runs `./test/e2e/buckets.sh verify` (`:774`) —
  the single source of truth that every `Test*` in `test/e2e/*_test.go` is
  in exactly one bucket.
- `e2e-go` job (`:787-878`) matrix `bucket: [operator, api-auth, api-roles,
  api-rbac, api-agent, api-mods]` (`:805`), each with its own `include:`
  block (`parallel`, `test_timeout`, `job_timeout`, optional `tail:
  ratelimit`) at `:807-832`.
- `e2e-web-live` job (`:1002-1071`) is a **separate**, pre-existing
  Playwright job (`web/e2e`, `npm run test:e2e:live`) gated on
  `needs.changes.outputs.weblive` — it drives the real browser against a
  live cluster and is **not** the Go suite; it does not satisfy item 12 (the
  73 procedures are meant to be scripted directly against the API the way
  `api-agent`/`api-rbac` tests are, not via a browser).
- The bucket name also appears in the dashboard-summary regexes at
  `:1270` (`'e2e-go': (n) => /^e2e (operator|api-auth|api-roles|api-rbac|api-agent|api-mods) \//.test(n)`)
  and `:1413` (same alternation, used to populate `bucketSet` for the
  "### e2e buckets run" markdown section built around `:1482`).

**Design — exactly what item 12 asks for:**

1. **Bucket name:** `api-web` (keeps the existing `api-<area>` naming
   convention; distinguishes it from the browser-driven `e2e-web-live` job
   name, and from `web/e2e` Playwright specs, without reusing the taken
   name `web`).
2. **Test functions:**
   - **Move:** none. No existing Go e2e test currently covers these 71
     procedures — `TestAPI_AuditEmitsOnMutation` /
     `TestAPI_AuditPaginationAndFilter` (currently in `api-auth`,
     `buckets.sh:64-73`) are adjacent to `audit-log-view`/`-pagination` but
     assert on the audit *mutation* trail, not the log-viewer's actor/
     method/status-class filters or hash-chain integrity check that
     `audit-log-filter-by-*` and `audit-log-verify-integrity` need — leave
     those two where they are and add new, more targeted tests to
     `api-web` rather than re-scoping them.
   - **New (grouped to respect the login-pressure convention in
     `buckets.sh:24-31` — one admin login shared by table-driven subtests
     per function, not one login per procedure):**
     - `TestAPI_AdminSettingsSections` — `admin-settings-*` (9 procedures,
       1 login, table-driven per settings tab).
     - `TestAPI_AdminLogsView` — `admin-logs-*` (4 procedures).
     - `TestAPI_AuditLogFilters` — `audit-log-filter-by-actor/-method/-status-class`
       + `-pagination` (reuses `TestAPI_AuditPaginationAndFilter`'s fixture
       pattern but as a new function so `api-auth`'s login budget is
       untouched).
     - `TestAPI_AuditLogIntegrity` — `audit-log-verify-integrity` (separate
       function: hash-chain verification is a distinct code path from
       filtering and may need its own fixture size).
     - `TestAPI_UsersManagement` — `users-list-view/-invite/-edit-role/-delete/-reset-password/-manage-roles`
       (6 procedures, 1 admin login + N target-user logins as needed).
     - `TestAPI_BackupsFleetView` — `backups-list-view/-filter-by-phase/-filter-by-server/-view-detail/-restores-view`.
     - `TestAPI_BackupsLifecycle` — `backups-backup-now/-restore/-schedules-create/-edit/-delete/-toggle-suspend`
       + the `server-detail-backups-*` mirrors (same underlying
       `/servers/{name}/backups*` and `/backupschedules` routes exercised
       from the server-detail path instead of the fleet path).
     - `TestAPI_ModulesCatalog` — `modules-catalog-browse/-search/-filter-by-category/-filter-by-source/-install/-upgrade/-uninstall/-upload-custom`.
     - `TestAPI_ServerDetailSettings` — `server-detail-settings-general/-lifecycle/-networking/-placement/-resources/-access/-environment/-version/-network-capture/-scheduled-backups`.
     - `TestAPI_ServerDetailDangerZone` — `server-detail-settings-danger-delete/-danger-transfer` (kept
       separate: destructive, wants its own server fixture per subtest,
       shouldn't share state with the settings-read tests above).
     - `TestAPI_ShareLinks` — `server-detail-settings-sharelinks-create/-view/-revoke` +
       `share-server-start/-view-status` (both are the same share-link
       feature, producer and consumer side).
     - `TestAPI_ServerDetailModsAndLogs` — `server-detail-mods-browse-registry/-check-updates`,
       `server-detail-modpacks-browse`, `server-detail-logs-view-pod-logs`.
     - `TestAPI_ThemeSettingsImport` — `theme-settings-import` (pairs with
       the existing `TestAPI_ThemePreferences` in `api-roles`, but as its
       own function/login since it's a distinct import-file code path).
   - This is 13 new test functions for 71 procedures — each grouped
     function should hold its 1 shared login under `buckets.sh`'s ~7
     admin-logins-per-job ceiling; **13 functions in one job would still
     need review against that ceiling** once real login counts are known
     (some groups above, e.g. `TestAPI_UsersManagement`, likely need more
     than 1 login for delete/reset-password paths) — may need splitting
     into two jobs/buckets (e.g. `api-web` and `api-web-2`) the same way
     `api-auth`'s comment block documents budget trade-offs
     (`buckets.sh:76-99`). Flagged as an open sizing question for whoever
     implements this, not resolved here.
3. **`test/e2e/buckets.sh` diff:**
   - Add a new `bucket_api_web() { cat <<'EOF' ... EOF }` function, placed
     after `bucket_api_mods()` (currently `:173-180`) and before
     `bucket_ratelimit()` (currently `:182-184`), listing the 13 `Test*`
     names above.
   - `bucket_names()` (`:301-303`): change
     `printf '%s\n' operator api-auth api-roles api-rbac api-agent api-mods ratelimit bot-fast bot-heavy multicluster upgrade`
     to insert `api-web` after `api-mods` and before `ratelimit`.
   - `list_bucket()` (`:305-320`): add `api-web) bucket_api_web ;;` after
     the `api-mods) bucket_api_mods ;;` line (currently `:312`).
4. **`.github/workflows/ci.yaml` diff:**
   - `:805` — matrix `bucket:` list: add `api-web` after `api-mods`.
   - `:807-832` — add a new `include:` block for `api-web` (parallel/
     timeout values TBD from real login-cost measurement; start
     conservative, e.g. `parallel: 3, test_timeout: 15m, job_timeout: 60`).
   - `:1270` and `:1413` — extend the bucket-name alternation regex
     `(operator|api-auth|api-roles|api-rbac|api-agent|api-mods)` to include
     `|api-web` in both places (dashboard-summary job-name matching).
   - No change needed to `e2e-buckets` (`:761-774`) or `e2e-web-live`
     (`:1002-1071`) — `buckets.sh verify` picks up the new bucket
     automatically once the test functions exist in `test/e2e/*_test.go`,
     and `e2e-web-live` is an unrelated Playwright job.

---

## (b) Items 3/23a — nuclear-option / terraria / minecraft-java Failed/Pulling on kubelab

`modules/` was already populated (`ff19983c` fetched, not empty) — no
`submodule update --init` needed. Built `gp-module` from `origin/master`
(worktree `bd32964f`) and ran `validate` on all three:

```
nuclear-option: OK (no findings)
terraria:       OK (no findings)
minecraft-java: OK (no findings)
```

So the **local bundle content in `modules/<name>/` is not malformed** —
`module.yaml`/`template.yaml` parse, schema-validate, and all three
`template.yaml` image refs are syntactically well-formed digest pins (64
hex chars, confirmed for all three: nuclear-option
`ghcr.io/valgulnecron/gameplane/nuclear-option@sha256:1be11d39...`,
terraria `passivelemon/terraria-docker:terraria-latest@sha256:d60f2805...`,
minecraft-java `itzg/minecraft-server:java21@sha256:f7155587...`). This
rules out a `gp-module validate`-catchable defect as the cause; the "Failed/
stuck Pulling" symptom lives in the **Module CR reconcile path**
(`operator/internal/controller/module_controller.go`) and/or kubelab's live
`ModuleSource`/registry reachability, not in the checked-in fixtures.

**Reconcile() failure paths** (`module_controller.go:76-153`):
`SourceNotFound` (77-79) → `WaitingForCatalog`/`NoVersionAvailable`
(pending, not failed) → `VersionUnavailable` (94-98) → `SourceConfig`
(110-112) → `markPullingTransition` (115, phase→`Pulling`) →
`fetcher.Pull` → **`PullFailed`** (118-121, registry/git fetch error) →
`VerifyConfig`/`SignatureInvalid` (126-132, cosign) →
**`DigestMismatch`** (134-139, `mod.Spec.Digest` pin vs. resolved
`bundle.Digest` — this is the **module bundle's** digest, i.e. the OCI/git
artifact the `ModuleSource` serves, unrelated to the container-image
digests inside `template.yaml` checked above) → `IncompatibleOperator`
(144-148) → `ApplyTemplate` (151-153).

**F-258 is the most likely direct cause of the "stuck Pulling" symptom**
(open, PR #445, `audit/findings.md:2915-2927` on the 018 branch): a Failed
Module's every reconcile calls `markPullingTransition` (phase
`Failed`→`Pulling`, `Pulling` condition flips `True` with a fresh
`LastTransitionTime`) then immediately `markFailed` (phase back to
`Failed`) — two status writes per reconcile, and because
`module_controller.go`'s `For(&Module{})` watch has no
generation-changed predicate, each write re-queues itself, producing a
continuous `Failed`→`Pulling`→`Failed` churn for as long as the Module
stays broken. A dashboard/`kubectl get module` snapshot taken mid-churn
would read `Pulling` even though the Module is really wedged `Failed` —
this matches "stuck Pulling" far better than a genuine slow pull. F-159/
F-160/F-161/F-162 (gp-module config-enum/CRD-enum/schema-validation/
preview fixes, `audit/findings.md` gp-module section, already merged
#441) are validator-side fixes and don't touch this reconcile loop, so
they don't explain the symptom by themselves — but F-161 in particular
("schema validation skips required CRD fields") means a bundle that used
to silently pass `gp-module validate`/the controller's own schema check
pre-#441 could now legitimately fail post-#441 if kubelab's currently
*applied* Module version predates that fix and has a real schema gap that
was never caught. That's a second, independent candidate cause worth
checking live.

**Per-module assessment (bundle content is clean; root cause needs live
confirmation):**

| Module | Likely cause | Evidence (repo-only) | Live kubelab confirmation needed |
|---|---|---|---|
| **minecraft-java** | Matches F-258's churn signature exactly — OD-021's own item 23(a) resolution says "check it against F-258/#445" before copying it for the audit's `audit018-test-template` fixture. | `module_controller.go:115,378-397` (`markPullingTransition`, no generation predicate on the watch — verify against `SetupWithManager`, not re-checked here); `findings.md:2915-2927`. | `kubectl get module minecraft-java -o yaml` watched over ~30s: if `status.phase` and the `Pulling` condition's `lastTransitionTime` are flapping every reconcile with `status.conditions[Failed].reason` non-empty underneath, it's F-258, not a real pull failure — the fix is PR #445 merging, not a modules-repo change. |
| **nuclear-option** | `ghcr.io/valgulnecron/...@sha256:1be11d39...` is an org-owned GHCR image; a genuine `PullFailed` on kubelab (vs. the F-258 churn) would mean either the digest was never pushed / was pushed under a different tag+digest since the module bundle's `module.yaml` version (`1.0.0`) was last cut, or GHCR auth/anonymous-pull is failing from kubelab's nodes specifically. | Digest is well-formed (64 hex chars) but its *existence in the registry* can't be checked without registry/network access from this scout session. | `kubectl describe module nuclear-option -n gameplane-system` for the `Reason`/`Message` on the `Ready`/`Pulling` conditions (distinguishes `PullFailed` "manifest unknown" from `DigestMismatch` from F-258 churn); if `PullFailed`, `crane manifest ghcr.io/valgulnecron/gameplane/nuclear-option@sha256:1be11d39...` (or equivalent) from a node/pod with kubelab's network path to confirm the digest actually resolves. |
| **terraria** | `passivelemon/terraria-docker:terraria-latest@sha256:d60f2805...` is a third-party Docker Hub image; Docker Hub's anonymous-pull rate limit (100 pulls/6h per source IP) is a standing hazard for a shared-egress cluster like kubelab and is a very plausible non-code cause distinct from F-258 — this would show as `ImagePullBackOff` on the **GameServer pod**, not the Module CR, so it may be a *different* failure than what "Module Failed" implies; needs disambiguating live whether "Failed" here means the Module CR or a resulting GameServer's pod. | None in-repo; this is an operational/registry-availability question. | Check whether `kubectl get module terraria` itself is `Failed` (Module-bundle pull, i.e. the git/OCI `ModuleSource` fetch of `modules/terraria/`) vs. a `GameServer` using the `terraria` `GameTemplate` sitting in pod `ImagePullBackOff` (image pull of `passivelemon/terraria-docker`) — these are two different Kubernetes objects and OD-021 items 3/23(a) don't disambiguate which is actually broken on kubelab. If it's the pod, check `kubectl describe pod` for `429 Too Many Requests` from Docker Hub. |
| **modules submodule vs. operator** | Given F-258 as the leading hypothesis, the fix is **operator-side** (PR #445, already open) — no `modules/` submodule change needed for minecraft-java. `nuclear-option`/`terraria` may turn out to be operator-side (F-258 churn again) or `modules/`-side (a stale/rotated digest pin needing a `module.yaml`/`template.yaml` version bump + re-push), but that split can't be determined without the live `describe`/`get -o yaml` output above. | — | Live `describe` output for all three, cross-referenced against whether `#445` alone (once merged) makes the "stuck Pulling" symptom disappear without any `modules/` change. |

---

## (c) Item 22 — GameServer `Resuming`/`Stopped` phase S4 finding

Verified in `operator/api/v1alpha1/`:

- `GameServerPhase` (`gameserver_types.go:8-22`) declares
  `Pending;Starting;Running;Stopping;Stopped;Suspended;Failed` — **there is
  no `Resuming` value in this enum at all.** `RestorePhase`
  (`restore_types.go:8-16`) is the type that has `Resuming` (alongside
  `Pending;Suspending;Running;Resuming;Succeeded;Failed`).
- `operator/internal/controller/gameserver_status.go`'s `derivePhase()`
  (`:268-294`) is the sole place `GameServerPhase` values are computed and
  assigned (`gameserver_status.go:255-256` is the only `Status().Patch`
  call that writes `gs.Status.Phase`, sourced from this function's return).
  It returns only `Pending` (282), `Starting` (285, 291), `Stopping` (277),
  `Suspended` (279), or `Running` (293) — **`GameServerPhaseStopped` is
  never returned**, even though it's referenced as a *read* in
  `gameserver_status.go:251` (clearing `StartedAt`), in `metrics.go:23`
  (enumerated for the phase-count metric, so the metric has a permanently-
  zero `Stopped` series), and in `restore_controller.go:127` (accepted as a
  valid pre-restore phase that in practice can never occur).
- `RestorePhaseResuming` (`restore_types.go:16`) is likewise declared but
  never assigned: `restore_controller.go` only ever sets `Pending` (70),
  `Running` (83, 130), `Suspending` (85), `Succeeded` (161), or `Failed`
  (200) — grep of the whole file for `RestorePhaseResuming` finds zero
  writes.
- `inventory-CRD.md` (018 branch) row `INV-CRD-020` is titled "Restore
  Running → Resuming → Succeeded" (`procedures/crd.md#restore-complete-and-resume`,
  pointing at `restore_controller.go:40-200`) — so OD-021's "Resuming...
  never set" is about `RestorePhaseResuming`, not a `GameServerPhase` value
  (that enum has no such member to begin with); "Stopped" is the
  `GameServerPhase` half. Two different CRDs, one shared defect pattern
  (declared enum value, never assigned).

**`findings.md` style** (matched against `### F-258`,
`audit/findings.md:2915-2927` on the 018 branch — numbered
Repro/observation, **Expected**, **Actual**, **Evidence**). Next free ID
per the file's current tail (`### F-259` is the last entry) is **F-260**.

Proposed row for the findings table (style of the existing rows,
e.g. `findings.md`'s `operator` section):

```
| F-260 | GameServer.Stopped and Restore.Resuming phases declared but never assigned | operator | review:operator | S4 | open | | | |
```

Proposed `### F-260` subsection, appended after `### F-259`:

```markdown
### F-260

**Repro / observation**
1. `GameServerPhase` (`operator/api/v1alpha1/gameserver_types.go:8-22`)
   declares `Pending;Starting;Running;Stopping;Stopped;Suspended;Failed`.
   `derivePhase` (`operator/internal/controller/gameserver_status.go:268-294`),
   the only function that assigns `gs.Status.Phase`, returns `Pending`,
   `Starting`, `Stopping`, `Suspended`, or `Running` — never `Stopped`.
2. `GameServerPhaseStopped` is still referenced downstream as if it were
   reachable: `gameserver_status.go:251` clears `StartedAt` on it,
   `metrics.go:23` includes it in the phase-count gauge's label set (so
   that series is permanently zero), and `restore_controller.go:127`
   accepts it as a valid pre-restore precondition that can never actually
   be observed.
3. `RestorePhase` (`operator/api/v1alpha1/restore_types.go:8-16`) declares
   `Pending;Suspending;Running;Resuming;Succeeded;Failed`.
   `restore_controller.go` only ever assigns `Pending` (:70), `Running`
   (:83, :130), `Suspending` (:85), `Succeeded` (:161), or `Failed` (:200)
   — `RestorePhaseResuming` is never written.
4. `inventory-CRD.md` row `INV-CRD-020` ("Restore Running → Resuming →
   Succeeded") documents the never-observed `Resuming` transition as if it
   happens.

**Expected:** Every declared enum value is either reachable by some code
path, or removed from the CRD/type if the transition it names was never
implemented (e.g. a suspend-then-resume restore, or an explicit stopped-
vs-suspended GameServer distinction).

**Actual:** `GameServerPhase.Stopped` and `RestorePhase.Resuming` are both
permanently dead values — present in the OpenAPI schema (`+kubebuilder:
validation:Enum` on both types), documented, and even branched on
elsewhere in the operator, but no reconciler ever sets either one.

**Evidence:** `operator/api/v1alpha1/gameserver_types.go:10,19`;
`operator/api/v1alpha1/restore_types.go:8,16`;
`operator/internal/controller/gameserver_status.go:251,268-294`;
`operator/internal/controller/metrics.go:23`;
`operator/internal/controller/restore_controller.go:70,83,85,127,130,161,200`;
`inventory-CRD.md` (018 branch) row `INV-CRD-020`.
```

---

## Assumptions

- Treated "web.md procedures that currently say `api`" as exactly the
  `**Automatable?** yes (api)` / `yes (api) but ...` lines (75 total, 73
  distinct after the 2 `n/a` withdrawals under OD item 13) — did not
  additionally pull in `api-agent`-tagged web.md rows (already correctly
  bucketed) or the `operator`-tagged ones.
- Grouped the 71 procedures into 13 proposed Go test functions by feature
  area to respect the login-budget convention documented in
  `buckets.sh:24-31`; did not attempt to size real login counts per group
  (would need the actual API routes/handlers per procedure, out of scope
  for a read-only scout pass) — flagged as an open sizing question rather
  than a settled bucket split.
- For (b), could not reach kubelab from this scout session (read-only,
  no live-cluster tooling used), so per-module root cause is evidence-based
  hypothesis (F-258 churn as leading cause, registry-reachability as
  secondary) rather than a confirmed diagnosis; the "what needs live
  confirmation" column is the deliverable for that gap, as scoped.
- Left the `wtod21` worktree and built `gp-module` binary in place under
  the scratchpad directory (`/tmp/claude-0/.../scratchpad/wtod21`,
  `/tmp/claude-0/.../scratchpad/gp-module`) since the task description's
  own instructions produced them there and cleanup wasn't requested;
  both are scratchpad-scoped and outside the repo working tree, so no
  `/home/user/Gameplane` branch state was touched.
