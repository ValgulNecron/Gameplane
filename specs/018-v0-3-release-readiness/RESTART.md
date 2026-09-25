# Restart Guide: spec 018 (v0.3 release readiness)

Live hand-off, last updated during session 5 (a cloud session, 2026-09-25 ~00:00 UTC). **After every context compaction, re-read this file first**, then [tasks.md](tasks.md) and [OPEN-DECISIONS.md](OPEN-DECISIONS.md). Keep this file current before context runs out.

## 0. Setup

1. `git fetch origin && git checkout 018-v0-3-release-readiness` (draft PR #424 holds the audit records).
2. The held security material (`audit/held/`, `SECURITY_AUDIT.md` rewording, `~/gameplane-audit-018/`, `~/kubelab.yaml`) exists **only on the devbox** and is git-ignored (OD-019). Never commit it or paste it anywhere public.
3. `git submodule update --init modules`.

## 1. Working rules

All of these still apply.

- **Context:** use `Workflow` and `Agent` for triage, fixes and reviews, so the main loop's context isn't used up (maintainer, session 5). Work only in scratchpad worktrees, never by switching the main checkout's branch.
- **OD-019:** held fixes read as neutral "hardening": no held IDs or repros in branches, commits, comments or PRs.
- **CLAUDE.md 8:** compile checks only locally. CI is the verifier.
- **Branches and PRs:**
  - One `fix/018-<group>` branch and PR per fix group. The maintainer approved this.
  - PR labels are set with the GitHub API (issue labels).
  - Only the maintainer merges.
- **Branch deletion:** the cloud session can't delete remote branches (403). Ask the maintainer, or enable GitHub's "Automatically delete head branches".
- **`github-advanced-security`:** ignore this check. It is Copilot autofind and always fails with "model not supported", per the maintainer.
- **Open PRs:** don't leave them to rot. Watch every open PR, fix red CI immediately, and say what is ready to merge. Session 5 is subscribed to #427–#447 and has an hourly `send_later` check-in.
- **Scope:** no design work (OD-025 groups 3, 7, 12, 13 and 36, the H31b dashboard, #434) and no held security fixes in cloud sessions. The held fixes (H31a onward) are done on the devbox.
- **Questions:** ask with the question tool, one issue plus its proposed fix per question. Never ask a blanket "go ahead or wait".
- **Model tiers (CLAUDE.md 13):** fable is banned. Opus work gets an independent opus review.
- **Helm:** CI uses Helm 4 (`azure/setup-helm` latest). For local `helm template` checks, download a helm binary into the scratchpad.

## 2. Decisions

- **Session 4 records (applied in `3990fa18`):**
  - `audit/findings.md` Status/Fix PR updates.
  - OD-021 and OD-026 RESOLVED 2026-09-24. Their resolutions are in OPEN-DECISIONS.md, and the live rounds use them.
  - **rc.1:** don't re-run or move the tag; `v0.3.0-rc.1` predates #435. Cut **rc.2** from master once the current fixes land (T057).
- **#430:** remote-cluster requests to home-cluster-only routes answer **501**, with the body `httperr.RemoteClusterNotImplemented`. The maintainer asked for this; the cross-cluster agent comes later.
- **#434** (a design PR from session 3) is ignored for now.

## 3. PR state

### Merged (findings already `fixed-unverified`)

- #432: F-215.
- #435: F-257. Confirm the operator, api and web images finished on `publish-edge` (run 36069312826).
- #438: F-121.
- #431: held H03. Update `held/findings.md` on the devbox.
- #428: group 8, F-172/F-052/F-173 (merged 2026-09-25 11:08 UTC; findings `fixed-unverified`). #447 retargeted to master.
- #429: group 4, F-116/F-130 (merged 2026-09-25 11:08 UTC; findings `fixed-unverified`). Group 22 can start now (it waited on #429).
- #436: group 11, F-054 (merged; findings `fixed-unverified`).
- #427: held H02 (merged 2026-09-25 11:05 UTC). Update `held/findings.md` on the devbox.

### Green at the end of session 4; ready to merge

| PR | Content |
|---|---|
| #433 | group 2: F-102, F-108 |
| #437 | group 15: F-103, F-104 |
| #439 | group 26: F-204 |
| #444 | group 16: F-074, F-075 (green in session 5) |
| #445 | F-258 (green in session 5; follow-up: registry errors whose message differs on every try can still cause churn) |

### Opened or fixed in session 5 (all green as of 2026-09-25 01:12 UTC)

| PR | Content | State |
|---|---|---|
| #430 | held H01 plus the 501 change | CI never ran on `94e79456` because of a CHANGELOG merge conflict. Master merged in as `258d8a27` (both CHANGELOG bullets kept); the PR body now says 501. Master merged again as `51e069a5` after #427 (CHANGELOG conflict); CI re-running. |
| #440 | group 21: F-125 | Re-implemented and pushed as `a8ce6395` after an independent opus review approved it. A module-level flag keeps `?safe-mode=1` across client-side navigation only; a full reload clears it, per theme-ui.md §4. The e2e spec is unchanged, and only the test this PR added was modified. Master was merged in. **Green** on `a8ce6395`. |
| #441 | group 23: F-159..F-162 | **Green** on `0360b22c` (the capture failure passed on its one re-run). `e2e operator / arm64` failed: `TestGameServer_NetworkCaptureStartStopDownload` got a 409 on the capture download after Completed. That is unrelated to the diff; a re-run is scheduled. It is the second sporadic capture failure (after #436), it was root-caused as F-259 and fixed in #449. The stricter validator rejected api module-builder fixtures that lack the CRD-required `spec.displayName`/`spec.version`. `86beb6c4` and `0360b22c` fixed them all (no production change was needed). **Test edits need maintainer sign-off** (PR comment posted). |
| #443 | group 9: F-213, F-214, F-218 | **Green** (17/17) on `0bd5c776`. `eac8389a`: master merged in, and the `backupEgress` template falls back to defaults under `--reuse-values`. `0bd5c776`: the CRD hook applies with `--field-manager=helm`. Without that, a Helm 4 reinstall over leftover CRDs failed with a `.spec.versions` conflict (a real user bug). The upgrade e2e now builds realistic leftover CRDs, and `docs/install.md` documents `--force-conflicts` for leftovers from beta.8 or earlier. **Sign-off needed:** the expected F-218 hook events now depend on the Helm version (PR comment posted). |
| #446 | group 24: F-179, F-180 (sentinel drain) | **Green** on `d23f2472`. Ask the maintainer whether the 4h drain ceiling is right. Nits: the parse error isn't wrapped with %w, and the 4h constant is duplicated. |
| #449 | F-259 (new): capture download 409 right after a user stop | **Green** on `357cddbd`. Lint (SA4006) fixed in `357cddbd`. The api download waits up to 15s for `SidecarStopped` before proxying. An independent opus review approved option A (polling in the api); the operator-set-Completed alternative was rejected because it would break two existing test assertions and research.md's lifecycle decision. Also watch for a repeat of the separate amd64 `NetworkCaptureEphemeralContainer` "ready still false" failure (cause unknown). |
| #447 | F-174 (playit address, OD-026 (a)) | Opened in session 5, originally stacked on #428; retargeted to master after #428 merged. **Green** on `639c765f` (after one re-run of an infra setup failure). `lint (tunnel)` is fixed in `639c765f` (gosec G304, noctx and an unused parameter, all fixed structurally with no suppressions). `TestBuildCommandPlayit` was edited for `--socket-path`, which needs sign-off. |
| #448 | group 18: F-105, F-106 | **Green** on `e53ab942`. Overview PlayersCard now shows "—" for -1, and the agent's `parseListWithRegex` returns Max:-1. PR opened in session 5. The `web` job failed because the Players.test `getByText("—")` added by this PR was ambiguous; it is now scoped with `within()` (`e53ab942`). Earlier head `f903a78b` with the Overview test scoped to the Players card heading); F-105/F-106 are `fixing`. **Sign-off needed** on the existing agent test edits (`players_test.go` 0/0→-1/-1, `heartbeat_test.go` dropping the gameVersion check). |

Local `npm ci` fails with ERESOLVE (`@eslint/js` 10 vs `eslint` 9, from merged #387). Use `--legacy-peer-deps` for the compile check only.

## 4. Next (in order)

1. Finish the in-flight items above. Review each agent's result (one tier up), push, and watch until green.
2. Give the maintainer the merge-ready list. After each merge, set its findings to `fixed-unverified`.
3. The remaining public non-design fix groups from `audit/evidence/rc.1/fix-plan.md`:
   - done or in flight: groups 1, 2, 4, 5, 6, 8, 9, 11, 15, 16, 18, 20, 21, 23, 24, 26;
   - not started: 10, 14, 17, 22 (land after #429), 25, 27, 29–31, 33–35, 37–52;
   - 28 is blocked on the module tag (T054), 32 is T063, and 19 may need design.
4. The OD-021 follow-ups (see OPEN-DECISIONS.md OD-021):
   - the new Go e2e bucket (item 12);
   - fix the Failed nuclear-option, terraria and minecraft-java Modules (items 3 and 23a);
   - the S4 finding for the unused phases (item 22);
   - procedure edits for every item.
5. rc.2 (T057): a CHANGELOG PR, then RC-TAG-2 approval, the tag, the release run (check it against contracts/rc-deploy.md §1) and a deploy to kubelab.
6. The devbox-only work:
   - held fixes H31a onward;
   - the T012 user list;
   - T046 and the live rounds T025–T034 and T047–T049, using the OD-021 resolutions in OPEN-DECISIONS.md.

## 5. kubelab facts

Unchanged from session 3.

- `KUBECONFIG=~/kubelab.yaml`. Nodes: `kubelab-control` and `kubelab-worker-{1,2}`, on k3s v1.36.2.
- Helm release `gameplane` in `gameplane-system`, revision 6, chart 0.2.0-beta.8.
- Site settings to preserve: `image.registry=gameplane-test`, `image.tag=016`, `operator.addressManager=metallb`, `ingress.host=gameplane.local`, `defaultModuleSource` git `ref: main`, `crds.autoApply.enabled=false`.
- Pre-existing GameServers (never write to them): `mc-fabric`, `soak-bogus-pool`, `soak-no-preference`, `squad` and `soak-pool-west`. ModuleSources `default` and `uploads` are also off limits.
- API access: `kubectl port-forward -n gameplane-system svc/gameplane-web 18080:80`, then `GP=http://127.0.0.1:18080`. The DB is at migration 011.
