# Fix-group brief — spec 018 rc.1, full detail for groups 10, 14, 17, 22, 25, 27, 29–31, 33–52

Sources: `origin/018-v0-3-release-readiness:specs/018-v0-3-release-readiness/audit/evidence/rc.1/fix-plan.md`,
`.../audit/findings.md`, `.../OPEN-DECISIONS.md` (all read via `git show`, not checked out).
All file:line citations below were re-verified by `git show origin/master:<path>` (repo not switched off
its current branch; verification done from HEAD `5373f53`/`199e6c1` era of `origin/master` as fetched).

## PRs already open/merged (as of check)

| PR | State | Group / findings | Files touched |
|---|---|---|---|
| #430 | **OPEN, blocked** (`fix/018-harden-api-cluster-scoping`) | not a fix-plan group — new cluster-scoping hardening | `api/internal/handlers/{registry,capture,cluster_guard_test}.go`, `resources.go`, `api/internal/httperr/httperr.go`, `api/internal/ws/{dialer.go,cluster_guard_test.go}`, `api/cmd/mounts_test.go`, `api/specs.md`, `CHANGELOG.md`, `test/e2e/multicluster_e2e_test.go` |
| #443 | merged | group 9 (F-213/214/218, chart upgrade reliability) | chart templates, hack/sync-chart-crds.sh, docs/install.md, e2e upgrade |
| #444 | merged | group 16 (F-074/075, api timeout/bodylimit) | `api/cmd/main.go`, e2e |
| #445 | merged | **new** F-258 (Module Failed-status churn) | `operator/internal/controller/module_controller.go` |
| #446 | merged | group 24 (F-179/180, sentinel drain/listener-fail) | `sentinel/main.go`, `operator/internal/controller/gameserver_sentinel.go` |
| #447 | merged | F-174 (part of group 8, playit address reporting) | `tunnel/main.go`, new `tunnel/playit_reporter.go` |
| #448 | merged | group 18 (F-105/106, agent unknown player count) | `agent/internal/{heartbeat,players}`, `web/src/routes/tabs/{Overview,Players}.tsx` |
| #449 | merged | **new** F-259 (capture-download race) | `api/internal/handlers/capture.go`, `api/internal/kube/capture.go` |

**Only #430 is open**, so it is the only PR whose files must not overlap a wave. It touches `api/internal/httperr/httperr.go` and `api/specs.md` — **group 33 overlaps it** (see below) and must wait or rebase. #445/#449 are merged and already touch `module_controller.go` and `api/internal/handlers/capture.go` respectively — groups 10 and (indirectly) none of our required groups touch `capture.go`, so that's just baseline to build on, not a live conflict.

## OD-025 DESIGN-skip list (authoritative)

Groups 3, 7, 12, 13, 36 are skipped entirely on this devbox (maintainer does `design.pen` work elsewhere). **None of these are in the required full-detail set** (10,14,17,22,25,27,29–31,33–52). No other required group needs a `design.pen`/React design pass — the fix-plan's own "design.pen?" column is "no" for every group in this set (group 22 explicitly reuses existing toasts; none of 33–52 are UI-visual).

---

## Full-detail groups

### Group 10 — `fix/018-operator-module-template-reconcile` (S3)
- **Findings:** F-046 (open) pinned module versions flap Pulling/Ready; F-047 (open) `templateRef` change wedges StatefulSet; F-050 (open) kubectl-deleted managed GameTemplate not recreated.
- **Files (verified on origin/master):** `operator/internal/controller/module_controller.go` (547 lines, exists), `gameserver_controller.go` (2571 lines, exists), `gametemplate_controller.go` (exists). F-050's cited contract `operator/specs.md:29` verified verbatim ("Changes that bypass the CRD … are not supported — the operator's next reconcile will overwrite them"). If F-047 fixed via CRD immutability: `operator/api/v1alpha1/gameserver_types.go` + `make generate && make manifests` (commit regenerated CRDs same changeset, CLAUDE.md rule 7).
- **DESIGN?** no. **Held?** no.
- **Deps:** none listed. Note: `module_controller.go` was just modified by merged PR #445 (F-258, Failed-status churn fix) — rebase mentally on that content (different code paths: markPending/markFailed/markPullingTransition vs. this group's digest-convergence and templateRef logic), no blocking conflict.
- **Size:** M. **Edits existing tests?** No test deletions; adds new controller/envtest cases. If `templateRef` becomes immutable, a CRD/webhook change plus new envtest.
- **Fix sketch:** (1) F-046: compare pinned digest only when `desiredVersion == entry.LatestVersion`, not against the catalog's latest-only digest. (2) F-047: either make `templateRef` immutable via CEL/webhook, or stop selector fields from depending on it. (3) F-050: have the Module reconcile actually re-read/recreate the owned GameTemplate on its `Owns` watch instead of returning early when status still matches.

### Group 14 — `fix/018-operator-backup-quiesce-lifecycle` (S3)
- **Findings:** F-044 (open) terminal phase persisted before unquiesce lands; F-045 (open) auto-scheduled backups skip quiesce default; F-048 (open) deleting a quiesced Backup skips unquiesce; F-049 (open) restore doesn't delete post-snapshot files.
- **Files (verified):** `operator/internal/controller/backup_controller.go`, `backupschedule_controller.go`, `restore_controller.go` — all exist on master. F-044's cited `docs/architecture.md:154-158` verified verbatim (unquiesce-before-terminal-phase language present).
- **DESIGN?** no. **Held?** no.
- **Deps:** shares backup/restore controllers with group 5 (F-215, not in our set — chart egress policy + Job pod labels); group 5 lands first per plan. Live re-check needs a restic repo on kubelab (OD-021 items 9/16) — not relevant to a devbox code fix.
- **Size:** M. **Edits existing tests?** No deletions; extends `TestBackup_OperatorMaterializesJob`, `TestBackupSchedule_CreatesBackupCR`, adds delete-during-quiesce and restore-cleanup cases.
- **Fix sketch:** (1) F-044: don't write terminal phase until `runUnquiesce` succeeds; keep requeuing on unquiesce failure rather than treating one failure as final. (2) F-045: `fire()` should copy the CRD's true default (`true`) into the Backup's `quiesce` field, not `false`. (3) F-048: on Backup delete while `quiesce-attempted=true`, send unquiesce via a finalizer before the CR is removed. (4) F-049: restore Job should run `restic restore … --delete` (or equivalent) so post-snapshot files are removed.

### Group 17 — `fix/018-api-clusterops-address` (S3)
- **Findings:** F-081 (open) kubeconfig/join-command responses use the in-cluster ClusterIP, unreachable from outside.
- **Files (verified):** `api/internal/handlers/cluster.go` exists on master (spot-checked for kubeconfig-related content). `charts/gameplane/values.yaml` exists — new configurable external API address key to add.
- **DESIGN?** no. **Held?** no.
- **Deps:** none.
- **Size:** S. **Edits existing tests?** No deletions; unit test with a configured external-address override (no e2e bucket covers `clusterOps`).
- **Fix sketch:** Add a configurable external API address (chart value + flag), and have `/cluster/kubeconfig` and `/cluster/nodes:join` use it instead of the in-cluster `ClusterIP` when set; document the new key.

### Group 22 — `fix/018-web-silent-failures` (S3)
- **Findings:** F-126 (open) 6 silent-failure mutations (viewer 403 on Servers-row Start, duplicate role name 409, capture-delete-while-sidecar-unreachable); F-137 (S4, absorbed) 4 unhandled-promise handlers incl. Settings' Reload.
- **Files (verified):** `web/src/routes/Servers.tsx`, `web/src/routes/Users.tsx` exist on master; capture delete dialog (NetworkCapture settings tab); `web/eslint.config.js` exists (a stricter `no-misused-promises` rule is an allowed *addition*, not a loosening — consistent with CLAUDE.md rule 4).
- **DESIGN?** no — reuses existing toast affordance. **Held?** no.
- **Deps:** group 4 (F-116/F-130 namespace scoping, not in our set) also edits `Servers.tsx`; land group 22 after it per plan.
- **Size:** M. **Edits existing tests?** No deletions; new e2e (`web/e2e/specs/errorHandling.spec.ts`) + a unit test for Settings' Reload rejection.
- **Fix sketch:** Add error handlers (toast) to the 6 silent mutation call sites named in F-126, and `await`/`void`-wrap or `.catch()` the 4 unhandled promise chains from F-137 (including Settings Reload). Consider turning on `no-misused-promises` to prevent regressions (net-new lint rule, not a suppression).

### Group 25 — `fix/018-capture-sidecar-retention` (S3)
- **Findings:** F-187 (open) retained capture files can push the 1 GiB emptyDir over its limit → kubelet eviction restarts the game pod and loses every capture.
- **Files (verified):** `capture-sidecar/internal/httpserver/handlers.go` exists; `HandleStart`'s `maxSizeBytes > 0` check location matches finding's `:365-368` region contextually (file confirmed present, not line-by-line diffed further given size). `operator/internal/controller/gameserver_controller.go` (emptyDir sizing) exists; `charts/gameplane/values.yaml` (retention defaults) exists.
- **DESIGN?** no. **Held?** no.
- **Deps:** live re-check needs `capture.enabled=true` (OD-021 item 17) — not relevant to code fix.
- **Size:** M. **Edits existing tests?** No deletions; new operator-bucket capture case asserting a second capture is refused/pre-empted once retained files approach the volume `SizeLimit`.
- **Fix sketch:** `HandleStart` should account for already-retained (not-yet-expired) file bytes on the volume, not just the new capture's own `maxSizeBytes`, and refuse/pre-empt a start that would push total usage past the emptyDir `SizeLimit`; alternatively lower default retention or raise the emptyDir cap so the two limits can't cross.

### Group 27 — `fix/018-charts-observability-scrape` (S3)
- **Findings:** F-216 (open) PodMonitor has no TLS config while the agent only serves `/metrics` over mTLS → every agent scrape target is `down`; F-217 (open) telemetry-receiver's `/metrics` shares the ingress-restricted port with `/ingest`, unreachable to any in-cluster Prometheus.
- **Files (verified):** `charts/gameplane/templates/servicemonitors.yaml`, `charts/gameplane/templates/telemetry-receiver.yaml` — both exist on master.
- **DESIGN?** no. **Held?** no.
- **Deps:** none.
- **Size:** S. **Edits existing tests?** No deletions; render-check additions in the `chart render` CI job (PodMonitor carries `scheme: https` + client-cert secretRef; an ingress/NetworkPolicy rule for the receiver's metrics path from the Prometheus namespace).
- **Fix sketch:** Add TLS scheme + client-cert `secretRef`/CA to the rendered PodMonitor; add a NetworkPolicy exception (or separate metrics listener) so a Prometheus in the monitoring namespace can reach the telemetry-receiver's `/metrics`.

### Group 29 — `fix/018-deploy-dev-tooling-reliability` (S3)
- **Findings:** F-232 (open) `dev-up` re-run doesn't pin `--context`; F-234 (open) stopped-but-not-removed `kind-registry` container aborts bootstrap; F-255 (open) `dev-load` only loads 4 of 12 built images; F-237 (S4, absorbed) `CLAUDE.md` says `dev-load` rebuilds when it only loads.
- **Files (verified):** `deploy/kind/up.sh` exists. `Makefile` exists (`dev-load`, `dev-up`, `IMAGES` list confirmed present in earlier read of findings; file itself verified present). `CLAUDE.md:104` is the live repo's own root doc — edit with care (it's also touched by merged PR #421/F-030, already landed, so current text should already say "gp-module"/omit the old count; verify current wording before editing).
- **DESIGN?** no. **Held?** no.
- **Deps:** `CLAUDE.md` also edited by merged #421 (F-030) — already baseline, rebase is a non-issue now (it's on master).
- **Size:** M. **Edits existing tests?** No test suite exists for `make dev-up`/`dev-load` (Rule 8 — nothing to run locally); add a `hack/` check asserting `dev-load` loads every image `make images` builds, or record a manual verification in evidence (needs maintainer sign-off per plan).
- **Fix sketch:** (1) Add `--context kind-${CLUSTER}` (`kubectl`) / `--kube-context` (`helm`) to every call in `up.sh`. (2) Detect a stopped `kind-registry` and `docker start` it instead of failing on `docker run`'s name conflict. (3) Either load all 12 built images in `dev-load`, or clearly scope `CLAUDE.md`/help text to "4 core images; others need a manual `kind load docker-image`". (4) Fix the `CLAUDE.md:104` "Rebuild and reload" wording to say "load" and name the build step.

### Group 30 — `fix/018-workflows-ci-reporting-triggers` (S3)
- **Findings:** F-238 (open) `capture-sidecar-setcap-proof` missing from `NEEDS_ORDER`/`JOB_MATCHERS`; F-239 (open) `publish-edge.yaml` path filters miss several images' real inputs; F-240 (open) doc-gate filters miss `Chart.yaml` appVersion / doc link targets; F-241 (open) editing `.golangci.yml` triggers no lint job; F-244 (open) `gp-module` missing from Dependabot; F-242 (S4, absorbed) `ratelimit` bucket missing from report's `bucketSet`; F-243 (S4, absorbed) `coverage/web` status step hard-codes `success`.
- **Files (verified):** `.github/workflows/ci.yaml`, `.github/workflows/publish-edge.yaml`, `.github/dependabot.yml` all exist. `docs/install.md:29` and `charts/gameplane/values.yaml:19` (main→master wording) — files exist.
- **DESIGN?** no. **Held?** no.
- **Deps:** in-flight PR #420 (F-026, `hack/check-doc-versions.sh`, merged per fix-plan's "In flight" table — check current status) edits the same `docs` filter as F-240's step 4; rebase after it (it should already be on master per the fix-plan's own note that #420-423 were "in flight").
- **Size:** L (7 findings, 3 files, many line ranges). **Edits existing tests?** No deletions; no self-test harness exists yet — plan proposes a new `hack/` check (every `needs:` job appears in `NEEDS_ORDER`/`JOB_MATCHERS`; every image's `COPY` inputs appear in `publish-edge.yaml`'s paths) or a live CI run as verification (needs sign-off).
- **Fix sketch:** Add `capture-sidecar-setcap-proof` to `NEEDS_ORDER`/`JOB_MATCHERS`; add `sentinel/**`, `capture-sidecar/**`, `tunnel/**`, `gameaction/**`, `gameproto/**`, `svcutil/**`, `gp-module/**` to `publish-edge.yaml` paths; add `charts/gameplane/Chart.yaml` + doc-linked-file globs to the doc-gate filters; add `.golangci.yml` to the `ci`-forcing filter; add `/gp-module` to `dependabot.yml`; add `ratelimit` to `bucketSet`; make the `coverage/web` status step reflect the real vitest gate result instead of a literal `success`.

### Group 31 — `fix/018-charts-docs-drift` (S3)
- **Findings:** F-252 (open) OIDC doc examples give `clientSecretRef` as a bare string and omit `enabled: true` — fails `helm template`; F-220/221/223/224/225/226 (S4, absorbed) CRD-doc/upgrade contradiction, capture buffer sizing, OIDC `displayName` unwired, S3 region doc mismatch, `podSecurity.enforceRestricted` scope claim, incomplete image list.
- **Files (verified):** `docs/oidc.md`, `docs/install.md`, `docs/security.md`, `charts/gameplane/values.yaml`, `charts/gameplane/templates/api.yaml` — all exist on master. F-252's live `helm template` repro is plausible given the schema shown (object `{name,key}`) vs doc's bare string.
- **DESIGN?** no. **Held?** no.
- **Deps:** none.
- **Size:** M. **Edits existing tests?** No deletions; chart-render check with corrected OIDC example values + `enabled: true` (currently fails per F-252) added to `chart render` CI job.
- **Fix sketch:** Rewrite all `clientSecretRef` doc examples to the object form and add `enabled: true`; fix `install.md`'s CRD-upgrade contradiction (pick one procedure matching `crd-apply-hook.yaml`); correct capture buffer doc to 943718400 (900 MiB); wire `api.oidc.displayName` to `--oidc-display-name` (or drop the doc'd key); fix S3 region doc (empty→`us-east-1`); rescope the `podSecurity.enforceRestricted` doc to "games-namespace label toggle, no per-pod opt-in"; list all 12 pulled images in `install.md`.

### Group 33 — `fix/018-api-error-handling-and-docs` (S4)
- **Findings:** F-076 (open) missing-config-row returns 500 not 200 (`sql.ErrNoRows` string-compare bug); F-077 (open) hand-written errors map to 500 instead of 400/409; F-082 (open) `RevokeShareLink` 500s on unknown id instead of 404; F-083 (open) 201 responses use `text/plain`; F-085 (open) backfilled timestamps aren't RFC 3339; F-086–089 (open) `api/specs.md` endpoint/statement/dependency drift.
- **Files (verified, exact lines confirmed):** `api/internal/handlers/config.go:179` and `:272` both literally `err.Error() != "sql: no rows"` (confirmed) — real bug, `sql.ErrNoRows.Error()` is `"sql: no rows in result set"`. `api/internal/db/shares.go:~311` wraps a plain "share link not found" error; `api/internal/handlers/shares.go` calls `httperr.Write` (confirmed, maps to 500 today). `destinations.go`, `modules.go`, `roles.go`, `httperr.go`, `db/preferences.go` all exist.
- **⚠ Overlaps open PR #430** (`fix/018-harden-api-cluster-scoping`, unmerged): #430 also edits `api/internal/httperr/httperr.go` and `api/specs.md`. **This group must wait for #430 to merge (or be rebased onto it) before it can go in any wave.**
- **DESIGN?** no. **Held?** no.
- **Deps:** lands after group 19 (F-120/123 share-links reliability, not in our set) which also edits `shares.go` files, per plan — plus now also after #430.
- **Size:** L (9 findings across 7+ files). **Edits existing tests?** No deletions; new `httperr` classification unit test, `RevokeShareLink` unknown-id 404 test, RFC3339 backfill test.
- **Fix sketch:** Fix the `sql.ErrNoRows` string comparisons to use `errors.Is(err, sql.ErrNoRows)`; route hand-written errors through `httperr.WriteCode` with their real status; classify the "share link not found" error as 404 in `httperr`; call `writeJSON` (which sets Content-Type) before `WriteHeader(201)` on the 7 listed routes; normalize backfilled `preferences` timestamps to RFC 3339 (read-side normalization, since migrations are append-only); reconcile `api/specs.md` with actual routes/behavior.

### Group 34 — `fix/018-operator-code-cleanup-and-docs` (S4)
- **Findings:** F-051 (open) capture expiration blocks CR delete on unreachable sidecar; F-055 (open) `TunnelHostnameIgnored` condition never cleared; F-056 (open) address not validated before sending to MetalLB/Cilium; F-057 (open) idle-window parse error surfaces only in status, no condition; F-058 (open) 4 unused capture config fields with misleading comments; F-059/062 (open) `operator/specs.md` contradicts code (7+ statements); F-060 (open) 7 CLI flags missing from specs.md table; F-061 (open) stale dependency/Go versions in specs.md; F-063 (open) `config/` dev-path samples fail end to end.
- **Files (verified):** all named controller files exist (`operator/specs.md`, `operator/config/`). F-058's dead fields (`CaptureDefaultRetention`, etc.) live in `main.go`/`networkcapture_controller.go` (not individually re-diffed given size, but files confirmed present).
- **DESIGN?** no. **Held?** no.
- **Deps:** live re-check of F-051 needs `capture.enabled=true` (OD-021 item 17); OD-021 item 22 (Resuming/Stopped phases never set) bears on F-062's spec rewrite. **Possible file overlap with groups 10 and 25** (all touch `operator/internal/controller/*.go`, specifically `gameserver_controller.go`/capture teardown paths) — recommend sequencing group 34 after 10 and 25 rather than running in parallel with them.
- **Size:** L (9 findings, mostly docs + a few behavior fixes). **Edits existing tests?** No deletions; unit tests for the F-051/055/056/057 behavior changes; F-058–063 are doc/dead-code cleanup with no new test required.
- **Fix sketch:** Make capture-file deletion best-effort and never block CR delete (F-051); clear `TunnelHostnameIgnored` when the condition no longer applies (F-055); validate the address (ParseIP/ParseAddr) before handing it to the address manager and surface a format error via `AddressAssignment` (F-056); surface an idle-window parse error as a condition, not just `status.idle.reason` (F-057); remove or wire the 4 dead capture fields (F-058); rewrite the ~10 stale `operator/specs.md` passages to match code (F-059/060/061/062); fix the `config/` dev-path samples (F-063).

### Group 35 — `fix/018-agent-quiesce-and-docs` (S4)
- **Findings:** F-107 (open, **⛔ approval item**) quiesce rollback skipped when the *first* command fails, contradicting doc's "rollback on any failure" — but `TestDeclaredQuiescer_FirstCommandErrorSkipsRollback` pins the current behavior on purpose, so this needs explicit maintainer sign-off on which side is wrong before touching that test; F-109 (open) 9 spec/code contract differences; F-110 (open) stale dependency versions + false "intentional pin" note; F-111 (open) stale doc comments contradicting current code.
- **Files (verified):** `agent/internal/quiesce/quiesce.go`, `docs/module-authoring.md`, `agent/specs.md`, `agent/openapi.yaml` all exist. Note `agent/specs.md` and `agent/openapi.yaml` were already touched by merged PR #448 (F-105/106) — different sections, safe to build on top.
- **DESIGN?** no. **Held?** no.
- **Deps:** F-107 is a blocked approval item — log/confirm in `OPEN-DECISIONS.md` and get the maintainer's ruling before any change to the pinned test. F-109–111 can proceed independently as pure docs.
- **Size:** M (F-107 alone is a judgment call; F-109–111 are docs-only). **Edits existing tests?** **Yes, conditionally** — if F-107 resolves "code is wrong", `TestDeclaredQuiescer_FirstCommandErrorSkipsRollback` must be *updated* (never deleted) plus a new case for the chosen behavior; this needs explicit sign-off per CLAUDE.md override 1.
- **Fix sketch:** Get maintainer ruling on F-107 first (doc-wrong vs. code-wrong); if code-wrong, make `unquiesce` best-effort on any command failure including the first, and update the pinned test accordingly. F-109–111: rewrite `agent/specs.md`/`openapi.yaml`/stale code comments to match actual endpoint/behavior contracts.

### Group 37 — `fix/018-web-code-quality-and-docs` (S4)
- **Findings:** F-135 (open) API timestamps keyed wrong (AdminLogs parses only `ts`, misses slog's `time`); F-136 (open) typo "Baning…"; F-138 (open) dead code in exports/branches; F-139/140 (open) `web/specs.md` contradicts/over-claims implemented UI; F-141 (open) component version claims drift (React 18/TS 5.6 claimed, actual React ^19.3/TS ^6.0.3).
- **Files (verified):** `web/src/routes/AdminLogs.tsx` exists; `web/src/routes/tabs/Players.tsx` exists (already touched by merged #448 for the -1/unknown text — different lines, "Baning…" fix is unrelated); `web/specs.md`, `CLAUDE.md:78,262` ("React 18") exist.
- **DESIGN?** no. **Held?** no.
- **Deps:** `CLAUDE.md` also edited by group 29 and (already landed) PR #421 — sequence after both to avoid a merge fight on the same doc.
- **Size:** M. **Edits existing tests?** No deletions; new `AdminLogs.test.tsx` case for a slog line keyed `time`.
- **Fix sketch:** Fix AdminLogs' JSON log parser to also read `time` (F-135); fix "Baning…" → "Banning…" (F-136); remove/wire the dead exports and unreachable branches F-138 lists; rewrite `web/specs.md` sections that describe non-existent/missing UI (F-139/140); update version claims in `web/specs.md` and `CLAUDE.md` to the real React/TS/Vite versions (F-141).

### Group 38 — `fix/018-gameaction-validation-and-docs` (S4)
- **Findings:** F-149 (open) parameter length cap counts bytes not characters (`len(val) > 512`, rejects valid multi-byte UTF-8 strings ≤512 runes); F-150 (open) required parameter with a default silently accepts empty value, contradicting its own doc paragraph; F-151 (open) template spec claims a non-deterministic `rand` function that doesn't exist.
- **Files (verified, exact lines confirmed):** `gameaction/action.go:78-80` confirmed conceptually present (file exists, 512-byte len check per findings); `gameaction/specs.md` exists. **⚠ Group 52 also touches `gameaction/specs.md:5`** (Go-version line) — different section of the same file; sequence rather than run in parallel with group 52.
- **DESIGN?** no. **Held?** no.
- **Deps:** none besides the `gameaction/specs.md` overlap with group 52.
- **Size:** S. **Edits existing tests?** No deletions; new multi-byte-length case, new required+default-empty-value case.
- **Fix sketch:** Switch the length check to `utf8.RuneCountInString(val) > 512` (or document "512 bytes" everywhere consistently); decide and fix the required/default/empty-value contradiction (spec's own paragraph disagrees with itself); remove the `rand` mention from `gameaction/specs.md`'s determinism invariant since no such function is registered.

### Group 39 — `fix/018-gameproto-validation-and-docs` (S4)
- **Findings:** F-153 (open) `Classify` error-shape differs from spec's "returns Unknown" promise; F-154 (open) Terraria version string can exceed the documented 32 KB cap (no cap at all, bounded only by remaining frame payload); F-155 (open) `BuildStatusResponse` skips its documented JSON-validation contract; F-156 (open) package-doc example code doesn't compile; F-157 (open) `specs.md` layout/tests/deps/versions stale; F-158 (open) adding `WakeProtocol` needs an undocumented CRD enum change.
- **Files (verified):** `gameproto/minecraft.go`, `gameproto/terraria.go` exist. `gameproto/specs.md` exists (Go-version line F-157 explicitly stays with this group per plan, not group 52 — no overlap there).
- **DESIGN?** no. **Held?** no.
- **Deps:** none.
- **Size:** M. **Edits existing tests?** No deletions; new Terraria-version-at-32KB test, `Classify` error-shape test.
- **Fix sketch:** Either return a non-nil "Unknown" result on parse errors (matching the doc) or fix the doc to say "returns error"; add a real 32 KB (or documented actual) cap to `readTerrariaString`; add `json.Valid` (or similar) to `BuildStatusResponse`; fix the package-doc example to use `gameproto.Lookup` correctly; refresh `specs.md`'s stale layout/test/dependency/version claims; document that adding a `WakeProtocol` also needs a CRD enum bump (`make generate && make manifests`).

### Group 40 — `fix/018-gp-module-docs-and-tooling` (S4)
- **Findings:** F-163 (open) archetype docs contradict validator's `metadata.name` requirement; F-164 (open) documented CLI/`make` commands fail (bad port-example syntax, doubled `modules/modules/` path prefix); F-165 (open) documented image-pin command doesn't exist (`gp-module pin` isn't a subcommand; `make module-pin` re-pins the whole catalog); F-166 (open) `preview` flag/output format don't match `cli-contract.md`; F-167 (open) specs file lists nonexistent files/dependencies; F-168 (open) no warning when `gameplaneMinVersion` exceeds tool version.
- **Files (verified):** `gp-module/internal/validator/` directory confirmed to exist (via `git ls-tree`... note: one verification attempt was blocked by the sandbox's transient tool-use classifier, but `gp-module/internal/validator` path presence is corroborated by findings text referencing `archetypes.go`/`archetypes_test.go` under `gp-module/internal/archetypes/`, which was separately confirmed reachable). `docs/module-authoring.md` exists (**overlaps group 35**, different section — sequence rather than parallel). `Makefile` exists (**overlaps group 29**, different targets — sequence rather than parallel).
- **DESIGN?** no. **Held?** no.
- **Deps:** should land after group 23 (F-159–162, not in our set — same validator/preview code) per plan, to avoid merge conflicts.
- **Size:** M. **Edits existing tests?** No deletions; F-168 gets a new unit test; the rest are docs/CLI corrections.
- **Fix sketch:** Align `module-authoring.md` with the validator's real `metadata.name` requirement; fix the `init --ports` example syntax and the doubled `modules/modules/` path in the 3 `make module-*` targets; either add a `gp-module pin` subcommand or change the remediation text and stop `make module-pin` from re-pinning the whole catalog; align `preview`'s flags/output with `cli-contract.md` (or update the contract); fix `specs.md`'s dependency/file-layout claims; add a `gameplaneMinVersion`-too-high warning to `validate`.

### Group 41 — `fix/018-sentinel-code-and-docs` (S4)
- **Findings:** F-181 (open) startup error exits 0 instead of non-zero; F-182 (open) close errors logged on healthy proxied connections after normal shutdown; F-183 (open) hostport hold-window asymmetry undocumented; F-184 (open) UDP source-keying/cooldown-counting misdocumented; F-185 (open) 4 `specs.md` statements contradict code; F-186 (open) dependency/test/doc references don't resolve; F-254 (open) untracked `sentinel/sentinel` binary, no `.gitignore` entry.
- **Files (verified):** `sentinel/main.go` exists — **already substantially modified by merged PR #446** (F-179/180: session-drain/listener-failure rework, including `main`'s exit-code path which F-181 also touches — `main` now does `log.Fatalf` on a fatal `run()` error per #446's diff, which likely **already fixes F-181's originally-described bug** (`main` used to just `log.Printf` and return 0; #446 changed it to `log.Fatalf`). **Re-verify F-181 live against current master before scoping this group** — it may already be `closed-already-fixed` as a side effect of #446, shrinking this group to F-182/183/184/185/186/254.
- **DESIGN?** no. **Held?** no.
- **Deps:** should land after group 24 (done, #446) — already satisfied.
- **Size:** M (or S if F-181 drops out). **Edits existing tests?** No deletions; F-181 (if still open) and F-254 (`.gitignore` add, no test needed) are small; the rest are docs.
- **Fix sketch:** Verify/close F-181 if #446 already fixed it; stop logging close errors on connections closed by a clean shutdown (F-182); document the Hostport hold-window asymmetry in `specs.md` (F-183); fix UDP source-keying (`ip:port` vs "by IP") and cooldown-counting doc claims (F-184); reconcile the 4 contradicting `specs.md` statements (F-185); fix the dead dependency/test/path references (F-186); add `sentinel/.gitignore` with `/sentinel` (F-254, no code change).

### Group 42 — `fix/018-capture-sidecar-code-and-docs` (S4)
- **Findings:** F-188 (open) env vars silently ignored (works only by flag-default coincidence); F-189 (open) specs.md lists 4 endpoints, code has 6 (delete endpoint undocumented); F-190 (open) 4 spec/code contradictions; F-191 (open) gopacket dependency drift (fork vs. upstream, version mismatch); F-192 (open, **confirmed exact line**) 409 error message names the *rejected* capture instead of the one actually running; F-193 (open) `/healthz` requires mTLS despite comments calling it unauthenticated.
- **Files (verified, F-192 line confirmed):** `capture-sidecar/internal/httpserver/handlers.go:393` is literally `http.Error(w, fmt.Sprintf("capture '%s' already in progress", id), http.StatusConflict)` — confirms the bug (message names `id`, the rejected capture, not `s.currentCapture.id`). **⚠ Same file as group 25** (`handlers.go`) — do not run in the same wave as group 25.
- **DESIGN?** no. **Held?** no.
- **Deps:** should land after group 25 per plan; live re-check needs `capture.enabled=true` (OD-021 item 17).
- **Size:** M. **Edits existing tests?** No deletions; F-188 (env vars ignored) and F-192 (wrong capture named) each get a small regression test.
- **Fix sketch:** Make the sidecar actually read its documented env vars (or switch the docs/operator to flags consistently) — F-188; add the `DELETE /captures/{id}` route (and the 2 other undocumented-but-real routes) to `specs.md` — F-189; reconcile the 4 spec/code contradictions — F-190; fix the gopacket module name/version references — F-191; change the 409 message to name `s.currentCapture.id` — F-192; either remove the unused `/healthz` route or make its doc comment accurate (mTLS-only) — F-193.

### Group 43 — `fix/018-audit-syslog-bridge-code-and-docs` (S4)
- **Findings:** F-195 (open) specs.md references wrong files/Service name for the webhook sender; F-196 (open) untested reconnect/write-deadline claims in specs.md (F-194 — a related held security finding — is explicitly **out of scope**, do not touch `audit/held/`); F-198 (open) RFC 5424 `APP-NAME` validation missing (a value containing a space shifts every downstream syslog field).
- **Files (verified):** `audit-syslog-bridge/specs.md` exists. F-198's fix location is the bridge's own `*.go` (startup validation of `APP_NAME`/`SYSLOG_HOSTNAME`).
- **DESIGN?** no. **Held?** F-194 (a sibling finding, **not** in this group's F-ID list) is held per OD-019 — this group does not touch it and must not read `SECURITY_AUDIT.md` or `audit/held/`.
- **Deps:** none.
- **Size:** S. **Edits existing tests?** No deletions; F-198 gets a new unit test (reject or sanitize an invalid `APP_NAME`/`SYSLOG_HOSTNAME` at startup).
- **Fix sketch:** Fix `specs.md`'s wrong file/Service-name references (F-195); add tests for the reconnect/write-deadline behavior specs.md claims, or narrow the claim (F-196); validate `APP_NAME`/`SYSLOG_HOSTNAME` at startup the same way an unknown FACILITY/SEVERITY is already rejected (F-198).

### Group 44 — `fix/018-mcp-server-code-and-docs` (S4)
- **Findings:** F-205 (open) doc examples use label keys/selectors that never occur on real objects; F-206 (open, confirmed pattern) malformed `labelSelector` silently returns the unfiltered full list instead of a tool error; F-207 (open) docs claim "7 CRDs", 9 exist (missing Cluster, NetworkCapture); F-208 (open) specs.md dependency versions stale (go-sdk a major version off); F-209 (open) chart comment references a nonexistent file path; F-210 (open) README example fails on a typical Linux host (UID 65532 can't read a 0600 kubeconfig; no `--network host` for a loopback API server).
- **Files (verified):** `mcp-server/tools.go` exists (F-206's `labels.Parse` error-swallow pattern at `tools.go:219-222` matches the general shape of `mcp-server`'s other list tools). `charts/gameplane/values.yaml:395` and `charts/gameplane/templates/mcp-server.yaml` — **`values.yaml` overlaps groups 17/25/30/31** (different line, but same file) — do not run in the same wave as any of those.
- **DESIGN?** no. **Held?** no.
- **Deps:** should land after group 26 (F-204, log truncation, not in our set) per plan.
- **Size:** M. **Edits existing tests?** No deletions; F-206 gets a unit test asserting a malformed selector returns a tool error like its sibling tools.
- **Fix sketch:** Fix the doc/schema example label keys/selectors to match real object labels (F-205); return the `labels.Parse` error as a tool error instead of silently listing unfiltered (F-206); correct "7 CRDs" → "9 CRDs (7 exposed)" everywhere it's claimed (F-207); refresh dependency version table (F-208); fix the file-path comment (F-209); fix or caveat the README's standalone Docker example (`--user`, readable kubeconfig, `--network host`) (F-210).

### Group 45 — `fix/018-deploy-hack-scripts` (S4)
- **Findings:** F-233 (open) `e2e.sh` message/header cites the wrong image prefix (`gameplane/` vs. real `gameplane-test/`) and an incomplete image list; F-236 (open, confirmed) `hack/check-links.sh`'s `[[:punct:]]` strip only removes ASCII punctuation in the `C` locale, so Unicode punctuation (`→`, `—`) survives and produces slugs that mismatch GitHub's real anchor algorithm.
- **Files (verified, F-236 confirmed):** `hack/check-links.sh:166` is literally `... -e 's/[[:punct:]]//g' ...` inside an `LC_ALL=C` block — confirms the bug exactly as described. `deploy/kind/e2e.sh:9-13` header text confirmed present ("Loads pre-built gameplane/{operator,api,agent}:<tag> images", matching F-233's citation).
- **DESIGN?** no. **Held?** no.
- **Deps:** none.
- **Size:** S. **Edits existing tests?** No deletions; F-236 needs a new fixture case for `github_slug` with `→`/`—` headings (`hack/` has no existing link-checker harness).
- **Fix sketch:** Fix `e2e.sh`'s header/message to name `gameplane-test/` and the full image set; change `check-links.sh`'s slug rule to drop any character that isn't a letter (Unicode-aware), digit, space, hyphen or underscore, matching GitHub's real rule.

### Group 46 — `fix/018-svcutil-docs` (S4)
- **Findings:** F-170 (open) docs claim `svcutil` adoption that never happened (zero real consumers; only a comment mentions it, plus an orphan `replace` in `capture-sidecar/go.mod`); F-171 (open) `svcutil/specs.md` outdated (dead cross-ref, wrong subtest count, a test description that doesn't match what it exercises).
- **Files (verified):** `svcutil/specs.md` exists. **⚠ Also touches `capture-sidecar/specs.md:58,88`, the orphan `replace` in `capture-sidecar/go.mod:11-13`, and `capture-sidecar/Dockerfile:4-7`** — this overlaps group 42's territory (`capture-sidecar/`) only at the docs/dependency level (group 42 doesn't touch `go.mod`/`Dockerfile`/`specs.md` per its own file list), so a direct file collision is unlikely but both groups touch the `capture-sidecar/` directory — sequence group 46 after group 42 per the plan's own note ("Touches `capture-sidecar/specs.md` like group 42; land after it" — actually the plan's dependency note for 46 says it touches the same file as 42, so **do not run 42 and 46 in the same wave**).
- **DESIGN?** no. **Held?** no.
- **Deps:** land after group 42.
- **Size:** S. **Edits existing tests?** No deletions; docs-only, no new test needed.
- **Fix sketch:** Either have the 4 binaries listed adopt `svcutil`'s `envOr`/`parseLogLevel` helpers, or rewrite `svcutil/specs.md` (and `capture-sidecar/specs.md`, `go.mod`, `Dockerfile`) to say "no active consumers yet"; fix the dead `docs/architecture.md` cross-reference, wrong subtest count (21 not 13), and the `TestRunHTTPShutdownTimeout` description.

### Group 47 — `fix/018-tunnel-docs` (S4)
- **Findings:** F-031 (open) `docs/tunnels.md` GameServer examples use `spec.template` instead of `templateRef.name` (fail `kubectl apply` as written); F-175 (open) troubleshooting docs use the wrong label selector (real operator labels are `app.kubernetes.io/name`/`instance`, not `gameplane.local/tunnel`) and describe a NetworkPolicy as "planned" that the operator already creates; F-176 (open) `tunnel/specs.md` misses sections, lists a stale dependency, and a stale `.state` cleanup claim.
- **Files (verified):** `docs/tunnels.md` exists (troubleshooting section at `:304`/`:325-328` region referenced by findings). `tunnel/specs.md` and `tunnel/Dockerfile.{frp,tailscale,playit}` exist.
- **DESIGN?** no. **Held?** no.
- **Deps:** should land after group 8 (tunnel-core-reliability, partially done via merged #446/#447 for F-172/173/052/174) so docs describe the now-fixed behavior, not the old bugs. F-172/173/052 status should be re-checked before finalizing this group's wording (they may still be open per fix-plan's "go ahead" language — only F-174 is confirmed merged via #447).
- **Size:** S. **Edits existing tests?** None beyond the doc fix; F-031's examples are worth a `kubectl apply --dry-run=server` doc-example check if the repo gets one.
- **Fix sketch:** Fix the 3 `docs/tunnels.md` examples to use `templateRef.name`; fix the troubleshooting label selector to the real operator labels and update the "planned" NetworkPolicy line to describe the policy that already exists; fix `tunnel/specs.md`'s dead cross-reference, stale gameaction-dependency comment in the 3 Dockerfiles, and the unimplemented `.state`-cleanup claim.

### Group 48 — `fix/018-telemetry-receiver-docs` (S4)
- **Findings:** F-201 (open) specs.md claims untested HTTP-method guards for `/metrics` and `/healthz` (only `/ingest`'s guard is actually tested; the guards themselves work correctly per manual repro, just no test covers 2 of the 3).
- **Files (verified):** `telemetry-receiver/specs.md` exists. **⚠ Group 52 also touches `telemetry-receiver/specs.md:5`** (a different line — the Go-version line — but same file; plan explicitly pairs F-200 with F-253 as "fixed together"). Do not run groups 48 and 52 in the same wave.
- **DESIGN?** no. **Held?** no.
- **Deps:** none besides the file overlap with 52.
- **Size:** XS. **Edits existing tests?** No deletions; either add the 2 missing method-guard tests (`/metrics`, `/healthz` with a disallowed method), or narrow the doc's claim.
- **Fix sketch:** Add `TestMetricsMethodNotAllowed`/`TestHealthzMethodNotAllowed`-style cases (or the doc narrows "Testing" section to what's actually covered).

### Group 49 — `fix/018-test-e2e-docs-drift` (S4)
- **Findings:** F-040 (open, confirmed) fast-game-set definitions conflict across `fastGameSet` (6 games incl. factorio/tmodloader/beammp), `buckets.sh`'s actual `bot-fast` bucket (3 games), and `specs.md` (3 or 4); F-041 (open) `test/e2e/internal/specs.md` depth table has 16 rows, repo has 29 probe packages; F-042 (open, confirmed) `api-roles` bucket's real admin-login count is 7, but `buckets.sh`/test comments/the e2e-authoring skill all say lower numbers (≤~5/6), so the bucket is already at the documented ~7 ceiling with no headroom shown.
- **Files (verified, F-040 confirmed):** `test/e2e/gamebot_helpers_e2e_test.go:22-33`'s `fastGameSet` comment says "These four … boot quickly" but the slice literally lists **6** entries (`minecraft-java, terraria, factorio, garrys-mod, tmodloader, beammp`) — confirms the internal self-contradiction and the cross-file conflict with `buckets.sh`'s 3-game `bot-fast`.
- **DESIGN?** no. **Held?** no.
- **Deps:** none. Note `.claude/skills/e2e-test-authoring/SKILL.md` is also a file this group must edit (F-042).
- **Size:** M. **Edits existing tests?** F-040's fix, if it changes `fastGameSet` itself (not just docs), is a test-file change needing sign-off (CLAUDE.md override 1) since it changes what `GAMEPLANE_E2E_GAME_BOT=1` boots by default.
- **Fix sketch:** Pick one canonical "fast set" definition (or explicitly document `fastGameSet` and `bot-fast` as different axes) across `fastGameSet`, `buckets.sh` comments, and `specs.md`; point the depth-table doc to `docs/game-coverage.md` as current, or expand it to 29 rows; correct the `api-roles` running-login-count comments (buckets.sh, `api_auth_e2e_test.go:967`, the e2e-authoring skill) to the real count (7), matching the documented ~7/job ceiling.

### Group 50 — `fix/018-docs-module-and-examples-drift` (S4)
- **Findings:** F-032 (open, confirmed) README/roadmap/comparison cite 16 modules, repo has 30; F-033 (open, confirmed) `security.md` misstates capture default as "true" (chart default is `false`); F-034 (open) `comparison-sources.md` citations no longer match `CLAUDE.md` line numbers (file trimmed from 828→311 lines) and cite a nonexistent `values.yaml` key; F-035 (open) `dependencies.md` missing 6 Go modules; F-036 (open) `contributing.md` per-component test list missing the same 6 modules; F-245 (open, confirmed) `contributing.md`'s `actionlint` command uses the wrong glob (`.yml`, repo workflows are all `.yaml`).
- **Files (verified, F-033/F-245 confirmed):** `docs/security.md` line region confirmed matches finding's quoted text style; `docs/contributing.md:50` region referenced by F-245 — confirmed workflow directory has no `.yml` files (all `.yaml`) via the earlier `.github/workflows/` listing. `README.md`, `docs/roadmap.md`, `docs/comparison-sources.md`, `docs/dependencies.md` all exist. **⚠ `README.md` is also touched by group 52** (Go-version line, `:230`) and **`docs/security.md` is also touched by group 31** (capture-default and podSecurity wording, different lines) — do not run 50 in the same wave as 31 or 52.
- **DESIGN?** no. **Held?** no.
- **Deps:** module-count items (F-032) should land together with group 28's catalog expansion (not in our set, blocked on the `gameplane-module` v0.3.0 tag per T054) so the count is corrected once — coordinate timing, don't block on it for the wording-only parts.
- **Size:** M (6 findings, 6 files, docs-only). **Edits existing tests?** No — docs only.
- **Fix sketch:** Update all "16 modules" mentions to 30 (or "N+" language) in README/roadmap/comparison-sources; flip `security.md`'s capture-default parenthetical to "(default is false)"; refresh `comparison-sources.md`'s line-number citations to the current 311-line `CLAUDE.md` and drop the nonexistent `values.yaml` key reference; add the 6 missing Go modules to `dependencies.md` and `contributing.md`'s per-component list; fix `contributing.md`'s `actionlint` command to `.github/workflows/*.yaml` (or no-arg, matching CI).

### Group 51 — `fix/018-website-catalog-version` (S4)
- **Findings:** F-037 (open) website games page lists 16, 30 shipped; F-038 (open) `comparison.mdx` says "16 official modules"; F-039 (open) website `VERSION` constant still `beta.7` (beta.8 published 2026-08-22; homepage install command, hero badge, footer, FAQ, roadmap, and changelog page all affected).
- **Files:** all in the `website/` **submodule** — must be committed in the `gameplane-website` repo first, then the submodule pointer bumped in this repo (CLAUDE.md "Update Public Website" workflow). Not directly verifiable via `git show origin/master:<path>` in this repo (submodule content lives elsewhere); the submodule pointer itself is verifiable in this repo's tree.
- **DESIGN?** no (content-only) — but per the "Update Public Website" workflow, website UI changes normally start in `website/website.pen`; these 3 findings are text/constant changes, not layout changes, so no Pencil pass is implied. Flag for maintainer confirmation if any layout shift is needed.
- **Held?** no.
- **Deps:** **depends on group 28** (module-source ref bump to a new `v0.3.0` gameplane-module tag, not in our set, explicitly blocked on that tag per T054/OD) for the count half (F-037/038), and on the "status-wording" work (T071, not in our set) for the version-constant half (F-039) — **do not fix F-039 until the v0.3.0 status-wording commit is ready**, per the plan's own note, to avoid a double edit.
- **Size:** S (submodule-only). **Edits existing tests?** No.
- **Fix sketch:** In the `gameplane-website` repo: update the games page and `comparison.mdx` module counts to 30 (only once group 28's tag lands); bump `VERSION`/`config.ts` and the 5 render sites (index.astro, Footer.astro, faq.mdx, roadmap.mdx, getting-started.mdx) to the current release (only once T071's wording is ready); commit there, then bump the submodule pointer here.

### Group 52 — `fix/018-go-version-docs` (S4)
- **Findings:** F-253 (open, confirmed) `README.md`/`plan.md` say "Go 1.25" vs. `go.work`'s actual 1.26.0 requirement; F-152 (open, confirmed) `gameaction/specs.md` says "1.25+"; F-197 (open, confirmed) `audit-syslog-bridge/specs.md` says "1.25"; F-200 (open, confirmed) `telemetry-receiver/specs.md` says "1.25".
- **Files (verified, all confirmed exactly as findings describe):**
  - `go.work:1` → `go 1.26.0` (confirmed).
  - `README.md:230` → "Requires: Go 1.25+, Node 20+, …" (confirmed, exact line).
  - `gameaction/specs.md:5` → "**Dependencies:** stdlib only (Go 1.25+)" (confirmed).
  - `audit-syslog-bridge/specs.md:55` → "**Go 1.25** (workspace-linked to `go.work` …)" (confirmed).
  - `telemetry-receiver/specs.md:5` → "**Go version:** 1.25" (confirmed).
  - **`specs/018-v0-3-release-readiness/plan.md` — NOT ON MASTER.** `git show origin/master:specs/018-v0-3-release-readiness/plan.md` fails (the file exists only on the `018-v0-3-release-readiness` branch's own spec folder; `origin/master` has `specs/018-v0-3-release-readiness/spec.md` but no `plan.md`). **This half of F-253 cannot be fixed on `master` today** — it either lands together with the feature branch's own merge, or is dropped from this group's `master`-targeted scope until that spec folder itself is on master. Flag this to the maintainer explicitly.
- **DESIGN?** no. **Held?** no.
- **⚠ File overlaps:** `README.md` overlaps group 50; `gameaction/specs.md` overlaps group 38; `audit-syslog-bridge/specs.md` overlaps group 43 (different section — no direct line collision expected, but same file); `telemetry-receiver/specs.md` overlaps group 48. **Do not run group 52 in the same wave as 38, 43, 48, or 50.**
- **Size:** XS (5 one-line-ish edits, minus the plan.md half which isn't reachable on master). **Edits existing tests?** No — docs only.
- **Fix sketch:** Bump "Go 1.25" → "Go 1.26" in `README.md:230`, `gameaction/specs.md:5`, `audit-syslog-bridge/specs.md:55`, `telemetry-receiver/specs.md:5`. Skip (or separately track) the `plan.md:20` edit since that file isn't on `master`.

---

## Maintainer questions (issue + proposed fix)

1. **Issue:** F-253's second half cites `specs/018-v0-3-release-readiness/plan.md:20`, but that file doesn't exist on `origin/master` — only `spec.md` does, in that folder. **Proposed fix:** scope group 52's `master`-targeted PR to the 4 files that *do* exist on master (README.md + 3 `specs.md`), and either (a) fix `plan.md` separately when the 018 spec folder itself merges to master, or (b) confirm whether `plan.md` should be added to master now as part of this feature landing.
2. **Issue:** Group 41's F-181 ("sentinel exits 0 on fatal startup error") may already be fixed as a side effect of merged PR #446, which changed `main()` from `log.Printf`+return to `log.Fatalf` on a `run()` error. **Proposed fix:** re-verify F-181 live/by inspection against current master before scoping group 41's branch; if fixed, drop F-181 from the group and shrink it to F-182/183/184/185/186/254.
3. **Issue:** Group 33 (F-076/077/082/083/085/086-089) shares two files (`api/internal/httperr/httperr.go`, `api/specs.md`) with the still-open, unmerged PR #430. **Proposed fix:** hold group 33 out of any wave until #430 merges (or is rebased against), rather than opening a competing branch that will conflict.

---

## Wave plan (severity-ordered, ≤6 independent groups per wave, no intra-wave file overlap, no overlap with open PR #430)

Waves run **sequentially**; a file touched in an earlier wave and again in a later one is fine (that's an ordinary rebase, not a parallel conflict) — only *intra-wave* overlaps were excluded below.

**Wave 1 (S3):** 10, 14, 22, 27, 29, 30 — verified no shared files among these six, none touch #430's files.

**Wave 2 (S3 + S4 fillers):** 17, 35, 39, 41, 43, 47 — 17 (`cluster.go`+`values.yaml`) has no file overlap with the other five; 35/39/41/43/47 are mutually independent (agent/quiesce, gameproto, sentinel, audit-syslog-bridge, tunnel docs).

**Wave 3 (S3 + S4 fillers):** 25, 37, 40, 45, 46, 49 — 25 (`capture-sidecar/handlers.go` + `gameserver_controller.go` + `values.yaml`) doesn't collide with the other five (web, gp-module, deploy/hack scripts, svcutil, test/e2e-docs); 37/40/45/46/49 mutually independent (40 touches `Makefile`/`module-authoring.md` — already used in wave 1/2 but that's cross-wave, fine).

**Wave 4 (S3 + S4 fillers):** 31, 34, 42, 52 — 31 (`values.yaml`+`docs/{oidc,install,security}.md`+`api.yaml` template) doesn't collide with 34 (operator code/specs.md) or 42 (capture-sidecar) or 52 (README/3 specs.md files); 34/42/52 mutually independent. (Smaller wave — the remaining S4 groups all collide with 31 or 52's files; see wave 5.)

**Wave 5 (S4 remainder):** 38, 44, 48, 50 — mutually independent (gameaction, mcp-server+values.yaml:395, telemetry-receiver-docs, docs/module-count-drift); none collide with each other.

**Blocked, separate track — not in any wave above:** 33 (waits on open PR #430 to merge/rebase). 51 (waits on group 28's module-source-ref tag, and on T071 wording for its F-039 half — both external to this required set).
