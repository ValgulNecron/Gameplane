# Restart Guide: spec 018 (v0.3 release readiness)

Hand-off as of 2026-09-24 ~23:30 UTC (end of the fourth session, a cloud session). Read this first, then [tasks.md](tasks.md) and [OPEN-DECISIONS.md](OPEN-DECISIONS.md).

## 0. Setup

1. `git fetch origin && git checkout 018-v0-3-release-readiness` (draft PR #424 holds the audit records).
2. The held security material (`audit/held/`, `SECURITY_AUDIT.md` rewording, `~/gameplane-audit-018/`, `~/kubelab.yaml`) exists **only on the devbox** and is git-ignored (OD-019). Never commit it or paste it anywhere public.
3. `git submodule update --init modules`.

## 1. Working rules

All of these still apply.

- **OD-019:** held fixes read as neutral "hardening": no held IDs or repros in branches, commits, comments or PRs.
- **CLAUDE.md 8:** compile checks only locally. CI is the verifier.
- **Branches and PRs:**
  - One `fix/018-<group>` branch and PR per fix group. The maintainer approved this.
  - PR labels are set with the GitHub API (issue labels).
  - Only the maintainer merges.
- **Branch deletion:** the cloud session can't delete remote branches (403). Ask the maintainer, or enable GitHub's "Automatically delete head branches".
- **`github-advanced-security`:** ignore this check. It is Copilot autofind and always fails with "model not supported", per the maintainer.
- **Open PRs:** don't leave them to rot. Watch every open PR, fix red CI immediately, and say what is ready to merge.
- **Scope:** no design work (OD-025 groups 3, 7, 12, 13 and 36, the H31b dashboard, #434) and no held security fixes in cloud sessions. The held fixes (H31a onward) are done on the devbox.
- **Questions:** ask with the question tool, one issue plus its proposed fix per question. Never ask a blanket "go ahead or wait".
- **Model tiers (CLAUDE.md 13):** fable is banned. Opus work gets an independent opus review.
- **Worktrees:** an agent once left the main checkout on a fix branch. Check `git branch --show-current` before committing records.

## 2. Record edits from session 4

Applied: `audit/findings.md` Status/Fix PR updates (F-213/F-214/F-218 → #443, F-074/F-075 → #444, F-258 → #445, F-125 → #440, F-159..F-162 → #441, F-179/F-180 → #446, F-174 → #447; F-105/F-106 left alone), and `OPEN-DECISIONS.md` OD-021 and OD-026 marked RESOLVED 2026-09-24 with their resolutions recorded there.

### Decisions from session 4

- **rc.1:** don't re-run or move the tag; `v0.3.0-rc.1` predates #435. Cut **rc.2** from master once the current fixes land (T057).
- **#430:** remote-cluster requests to home-cluster-only routes answer **501**, with the body `httperr.RemoteClusterNotImplemented`. The maintainer asked for this; the cross-cluster agent comes later. It is done, pushed as `94e79456`.
- **#434** (a design PR from session 3) is ignored for now.

## 3. PR state

At the end of session 4, every open PR is green on its head, except where noted below.

### Merged

- #432: F-215, backup/restore egress.
- #435: F-257, Go images cross-compile. `publish-edge` on master confirmed that 9 of the 12 images build and sign in 1–4 minutes. Confirm operator, api and web finished (run 36069312826).
- #438: F-121.
- #431: held H03. Update `held/findings.md` on the devbox.
- F-215, F-257 and F-121 are already `fixed-unverified` in findings.md.

### Open, green, ready to merge

| PR | Content |
|---|---|
| #427 | held H02 hardening |
| #428 | group 8: F-172, F-052, F-173 |
| #429 | group 4: F-116, F-130 |
| #433 | group 2: F-102, F-108 |
| #437 | group 15: F-103, F-104 |
| #439 | group 26: F-204 |

### Open, CI pending or re-running

| PR | Content | State |
|---|---|---|
| #430 | held H01 plus the 501 change | CI ran on `94e79456` |
| #436 | group 11: F-054 | The wipe test passes on both arches. The amd64 failure was `TestGameServer_NetworkCaptureEphemeralContainer` (passed on arm64, not touched by this PR). Failed jobs of run 36067327578 were re-run once. If it fails again, investigate it as a real capture bug (new finding), not in #436. |
| #440 | group 21: F-125 | |
| #441 | group 23: F-159..F-162 | |
| #443 | group 9: F-213, F-214, F-218 | |
| #444 | group 16: F-074, F-075 | |
| #445 | F-258 | Follow-up noted: registry errors whose message differs on every try can still cause churn. |

### Red at hand-off (fix first)

- **#441:** `go (api)` fails on amd64 and arm64. That is odd for a gp-module-only diff, so compare with master first.
- **#440:** `e2e web live` fails on amd64 and arm64.
- **#427:** `e2e web live` arm64 fails on a new head, `b5c3a88d`, which session 4 did not push.
- **#443:** `chart render` fails.
- **#436:** now green, 17/17 after the re-run.

### Wave 3 results: local commits, NOT pushed

Auto mode blocked all writes at the end of session 4.

- **`fix/018-sentinel-core-reliability` at `d23f2472`, group 24, opus review APPROVED.** Push it and open a PR.
  - Each session now runs under its own context and drains on SIGTERM (`SHUTDOWN_DRAIN_TIMEOUT`, default 4h).
  - The operator sets the waker's terminationGracePeriod to the drain time plus 30s, except in Hostport mode, which gets 0s and 30s.
  - F-180 surfaces the first fatal listener error.
  - `TestProxyBidirectionalStopsOnContextCancel` is replaced as the maintainer approved.
  - Non-blocking nits:
    - `SHUTDOWN_DRAIN_TIMEOUT` parse error is not wrapped with %w;
    - a stray test goroutine;
    - the 4h constant is duplicated and only a comment keeps the two in sync.
  - Ask the maintainer about the 4h drain choice.
- **`fix/018-tunnel-playit-address` at `6063e07c`, F-174, stacked on #428, opus review APPROVED.** Push it and open a PR against master after #428 merges, or against #428's branch.
- **`fix/018-agent-status-reporting` at `83897242`, group 18, review NOT approved.**
  - The Players tab now renders "—", but **`web/src/routes/tabs/Overview.tsx:379` (PlayersCard)** also prints -1. Apply the same "—" / "unknown" handling there and add an `Overview.test.tsx` case.
  - Optional: `parseListWithRegex` returns `Max:0` on zero matches; make it -1 for consistency.
  - **Ask the maintainer** to sign off the edits to the existing agent tests (`players_test.go` 0/0→-1/-1, and `heartbeat_test.go` dropping the gameVersion check).
- Local `npm ci` fails with ERESOLVE (`@eslint/js` 10 vs `eslint` 9, from merged #387). Agents used `--legacy-peer-deps` for the compile check only.

### Earlier notes on these branches

- **`fix/018-agent-status-reporting` (group 18: F-105, F-106).** The agent fix is pushed as `c3484c43`. The maintainer approved a text-only Players tab change: an unknown count shows "—" / "Player count unknown". The wave-3 agent adds it. Then open the PR (`type: fix`, `area: agent`, `area: web`).
- **`fix/018-sentinel-core-reliability` (group 24: F-179, F-180).** The maintainer **approved rewriting** `TestProxyBidirectionalStopsOnContextCancel`, which asserts the buggy behaviour. Wave 3 implements it. Labels: `type: fix`, `area: optional-components`, `area: operator`.
- **`fix/018-tunnel-playit-address` (F-174, OD-026 (a)).** Stacked on #428. If the playitd socket protocol can't be established, report back to the maintainer.

## 4. Next (in order)

1. Apply section 2. Collect the wave-3 results, review and push them, open the PRs and watch them.
2. Watch every open PR to green, and give the maintainer the merge-ready list. After each merge, set its findings to `fixed-unverified`.
3. The remaining public non-design fix groups from `audit/evidence/rc.1/fix-plan.md`:
   - done or in flight: groups 1, 2, 4, 5, 6, 8, 9, 11, 15, 16, 18, 20, 21, 23, 24, 26;
   - not started: 10, 14, 17, 22 (land after #429), 25, 27, 29–31, 33–35, 37–52;
   - 28 is blocked on the module tag (T054), 32 is T063, and 19 may need design.
4. The OD-021 follow-ups:
   - the new Go e2e bucket (item 12);
   - fix the Failed nuclear-option, terraria and minecraft-java Modules (items 3 and 23a);
   - the S4 finding for the unused phases (item 22);
   - procedure edits for every item.
5. rc.2 (T057): a CHANGELOG PR, then RC-TAG-2 approval, the tag, the release run (check it against contracts/rc-deploy.md §1) and a deploy to kubelab.
6. The devbox-only work:
   - held fixes H31a onward;
   - the T012 user list;
   - T046 and the live rounds T025–T034 and T047–T049, using the OD-021 decisions above.

## 5. kubelab facts

Unchanged from session 3.

- `KUBECONFIG=~/kubelab.yaml`. Nodes: `kubelab-control` and `kubelab-worker-{1,2}`, on k3s v1.36.2.
- Helm release `gameplane` in `gameplane-system`, revision 6, chart 0.2.0-beta.8.
- Site settings to preserve: `image.registry=gameplane-test`, `image.tag=016`, `operator.addressManager=metallb`, `ingress.host=gameplane.local`, `defaultModuleSource` git `ref: main`, `crds.autoApply.enabled=false`.
- Pre-existing GameServers (never write to them): `mc-fabric`, `soak-bogus-pool`, `soak-no-preference`, `squad` and `soak-pool-west`. ModuleSources `default` and `uploads` are also off limits.
- API access: `kubectl port-forward -n gameplane-system svc/gameplane-web 18080:80`, then `GP=http://127.0.0.1:18080`. The DB is at migration 011.
