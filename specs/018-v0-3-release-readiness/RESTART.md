# Restart Guide: spec 018 (v0.3 release readiness)

Complete hand-off as of 2026-09-24, end of the third session. Read this first, then [tasks.md](tasks.md) and [OPEN-DECISIONS.md](OPEN-DECISIONS.md).

## 0. Setting up on another machine

1. `git fetch origin && git checkout 018-v0-3-release-readiness` (draft PR #424). Every committable audit record is on this branch.
2. **Copy by hand from the devbox**; these are deliberately not in git (OD-019):
   - `specs/018-v0-3-release-readiness/audit/held/`: the whole folder. It is git-ignored and holds everything about the security findings: `findings.md`, `fix-plan-held.md`, `questions.md`, `review-*.md`, `verification-*.md`, `procedures-security.md`, `inventory-SEC.md` and `evidence/`.
   - `SECURITY_AUDIT.md`: the reworded version, an uncommitted change in the devbox working tree. Don't commit it until the maintainer says so.
   - `~/gameplane-audit-018/` (mode 700; `admin.env`, session files, `db-snapshots/`) and `~/kubelab.yaml`, only if this machine will run the live rounds on kubelab.

   Never commit any of these, and never paste their contents into a PR, issue or commit message.
3. `git submodule update --init modules`.

## 1. Working rules (all still apply)

- **OD-019, security findings:** they stay off GitHub until fixed.
  - Public files carry at most a stub such as "held (OD-019)".
  - Fix branches, commits, code comments, test names and PR text for held findings read as neutral "hardening". They cite no held ID and give no repro.
  - After a held fix merges, its row and subsection move from `held/findings.md` into `audit/findings.md` unchanged.
- **SECURITY_AUDIT.md:** never read an older version from git (`git show <rev>:SECURITY_AUDIT.md`, or `git log -p`, `git diff` or `git blame` on it).
- **Model tiers (CLAUDE.md 13):** fable is banned (OD-020, cost). Tier+1 review of opus work is done by an independent opus agent. Sonnet's cyber safeguard stops it on security-adjacent work (`procedures/api.md`, `procedures/crd.md`, anything in `held/`), so use opus there.
- **Design (OD-025):** groups that need a `design.pen` pass are done on a machine where Pencil screenshots work.
  - Public groups: 3 `fix/018-web-file-overwrite-guard`, 7 `fix/018-web-tunnel-wizard-flow`, 12 `fix/018-web-settings-lifecycle`, 13 `fix/018-web-server-create-placement`, 36 `fix/018-web-ui-polish`.
  - Held: the dashboard half of H31 (H31b).
  - Design first, then the maintainer's OK, then React.
- **Verification (CLAUDE.md 8):** no test or lint suites locally; compile checks only (`go build ./...`, `helm template`, `npx tsc --noEmit`, `go vet -tags e2e` for e2e compile). CI is the verifier.
- **Commits and PRs:** `git commit -s` with trailers. PR labels go through the REST API (`gh pr edit` is broken). Only the maintainer merges; PRs from the maintainer's account need the ruleset bypass.
- **Editing records:** apply record edit lists with an exact, unique-match replacement script (the devbox copy is `/tmp/claude-1000/-home-dev-Gameplane/e0cf04a8-b80e-4fc8-92df-ee2396f7dbe8/scratchpad/apply_edits.py`; it's short to rewrite). Model `Edit` calls fed with JSON-escaped text corrupted shell quoting.
- **Auto mode:** its safety check blocked shell commands several times. Run the live security checks, T012 and T046 in the default permission mode.
- **Current focus:** the maintainer asked for the held security fixes (S1 → S4) first. Public fix groups continue only when asked.

## 2. Done

### Git, PRs, release
- **Branch history:** `018-v0-3-release-readiness` was rebuilt without the security material and force-pushed (OD-019). The devbox keeps the old tip as `local/018-pre-rewrite`; never push it.
- **Merged to master:**
  - #420: doc-version checker handles bare versions, with fixture tests (OD-011, OD-012).
  - #421: CLAUDE.md repo map.
  - #422: e2e upgrade baseline beta.8.
  - #423: CHANGELOG rc.1.
  - #425: F-212, the games Namespace gets `helm.sh/resource-policy: keep`.
  - #426: F-251, nginx `client_max_body_size`.
  - Their findings are `fixed-unverified`, and their branches and worktrees are deleted.
- **Tag `v0.3.0-rc.1`:** on `c44cb179`, annotated (OD-013), with the maintainer's approval. CI on it is green (74/74 after one re-run of an arm64 Go-proxy flake).
- **Release run 36051057887** (`release.yaml` for the tag):
  - The first attempt was cancelled: the operator image build hit the 30-minute `timeout-minutes` (`release.yaml:15`), so the chart and GitHub-release jobs were skipped.
  - The re-run of the failed jobs was **cancelled the same way**. rc.1 therefore has images for every component except the operator, and no chart and no GitHub release.
  - This is a real pipeline defect. Record it as a finding (release pipeline, S2: an RC can't be published), then fix it in a PR: raise `timeout-minutes` for the image job (`release.yaml:15`), or speed up the operator multi-arch build.
  - After that merges, re-run the release for the same tag, or cut rc.1 again with the maintainer's approval.

### Records (all in `audit/`)
- **Findings:** 256 in total. `findings.md` holds the 199 public ones; the rest are in `held/findings.md`. Each batch was checked one tier up and cross-chunk duplicates were reconciled.
- **Coverage:** every row in `coverage.md` is complete (code reviews on opus, verified by an independent opus agent). Evidence is in `evidence/review-*/`; held candidates are in `held/`.
- **Inventory:**
  - `inventory.md`: UPG and NODE rows sit in their own sections, SEC rows are held, and evidence links point under `evidence/`.
  - Procedures: every file except the held security one had a correctness pass, factual follow-ups and a quoting sweep.
  - `procedures/upgrade.md` baseline-beta8 step 3 carries a DO NOT RUN banner (OD-023).
- **T012 (kubelab):**
  - `audit018-admin` was created via `kubectl exec -n gameplane-system deploy/gameplane-api -- /api bootstrap-admin --username audit018-admin`; its password is in `~/gameplane-audit-018/admin.env`.
  - API state names and DB migration level 011 are recorded in `kubelab-baseline.md`.
  - Still missing: the API user list.
- **T050 fix plans:** `evidence/rc.1/fix-plan.md` (52 public groups) and `held/fix-plan-held.md` (held groups). The maintainer approved all of them (OD-024).
- **Decisions resolved:** OD-001..OD-020, OD-022..OD-025, and RC-TAG-1. The maintainer's answers on the held questions are in `held/questions.md`.

### Tasks
- **Done (`[X]`):** T001–T011, T013, T024, T035–T045 and T050. T014 has started (tag pushed, release still running).

## 3. In flight

### Open PRs (waiting for the maintainer to review and merge)
| PR | Branch | Content |
|----|--------|---------|
| #428 | `fix/018-tunnel-core-reliability` | group 8: F-172, F-052, F-173 (F-174 waits on OD-026) |
| #429 | `fix/018-web-namespace-scoping` | group 4: F-116, F-130 |
| #427 | `fix/018-harden-module-bundle-integrity` | held H02 (hardening) |
| #430 | `fix/018-harden-api-cluster-scoping` | held H01 (hardening) |
| #431 | `fix/018-harden-admin-settings-state` | held H03 (hardening) |
| #424 | `018-v0-3-release-readiness` | draft, the audit records |

When a fix PR merges: set its findings to `fixed-unverified` (held ones in `held/findings.md`, moved into `findings.md` once merged), and delete the branch and worktree (CLAUDE.md 12).

### Pushed work-in-progress branches (no PR yet, review not passed)
- `fix/018-agent-file-write-safety` (group 2: F-102, F-108). The atomic write now creates files with `os.CreateTemp` (0600), which regresses the mode game containers need. Fix the code, not the linter config:
  - for an existing target, copy its mode onto the temp file from `os.Stat` (a runtime mode, not a constant, so gosec G302 doesn't flag it);
  - for a new file, create the temp file with `os.Create` semantics (0666 & ~umask) under a unique name;
  - assert the mode in `files_test.go` and add one line to `agent/specs.md`;
  - then run a sonnet review and open a PR (`type: fix`, `area: agent`).
- `fix/018-charts-backup-restore-egress` (group 5: F-215). `test/e2e/restore_e2e_test.go` ~135-155 declares `restoreJob` and never reads it, a compile error under the `e2e` tag. Remove it or assert on it, compile-check with `go vet -tags e2e ./...` in `test/e2e/`, re-review, then open a PR (`type: fix`, `area: chart`, `area: operator`).

### Held security fixes (current focus)
- S1: done (#430, #427).
- S2: H03 is done (#431). Next is **H31a**, the API half of H31.
  - The maintainer approved the split and the scope. Details are in `held/questions.md` and the H31 section of `held/fix-plan-held.md`.
  - H31b (dashboard) goes to the design queue.
- Then the remaining held S2 groups, then S3 and S4, from `held/fix-plan-held.md`.

## 4. Left to do (in order)

1. Held security fixes: H31a, then the rest of the held plan S2 → S4.
2. Finish the two WIP public branches (groups 2 and 5).
3. T014: once the release run finishes, check it against contracts/rc-deploy.md §1:
   - the run is green;
   - `ghcr.io/valgulnecron/gameplane/<component>:v0.3.0-rc.1` images exist and pass `cosign verify --key cosign.pub`;
   - the chart `oci://ghcr.io/valgulnecron/charts/gameplane:0.3.0-rc.1` exists and is signed;
   - the GitHub release is a prerelease;
   - no `0.3` image tag was created or moved.

   Record the results under a new `## rc.1` in `audit/rounds.md`.
4. T015: deploy rc.1 to kubelab. Take a DB snapshot first, pass the `capture` and `operator.gameDataStorage` keys explicitly (F-214), record the overrides in `rounds.md`, and compare `helm get values` with the baseline.
5. OD-022: add `.github/actions/` and `images/` rows to `coverage.md` and contracts/audit-records.md, and review both.
6. The remaining public fix groups (S2 → S4) from `evidence/rc.1/fix-plan.md`, when the maintainer asks. Skip the OD-025 design groups and anything blocked by OD-021 or OD-026.
7. T012 user list and T046 (rewrite `held/procedures-security.md` as control checks), both in the default permission mode.
8. Live rounds on rc.1: T025–T034 (these need the OD-021 answers), T047/T048 (the security checks, default permission mode), and T049 (re-check the 25 imported findings).
9. rc.2 (T057): a CHANGELOG PR, RC-TAG-2 approval, tag and deploy.
   - rc.2 contains #425, so OD-023 path (e) applies: reinstall beta.8 on kubelab with the rc.2 chart (the games namespace now survives `helm uninstall`), then run the US4 upgrade and node rounds (T060–T065).
   - Remove the DO NOT RUN banner only once the keep policy is confirmed on kubelab's namespace.
10. Loop T055–T059 until no finding blocks the release, then the report and go/no-go (T066–T070), then the final release (T071–T076).

## 5. Pending with the maintainer

- **OD-021:** 24 procedure design questions. Proposed defaults were sent in chat on 2026-09-24:
  1. one shared `audit018-agent-mc`;
  2. quiesce tested via a Backup;
  3. re-check the failed Modules before the round;
  4. users-get is n/a;
  5. module-source toggles are blocked;
  6. signature check with an `audit018-` OCI ModuleSource;
  7. `web.enabled` runs last, via an api port-forward;
  8. existingClaim is blocked;
  9. and 16. one `audit018-restic` per round;
  10. nuclear-option `Automatable?` is `no`;
  11. drop the different-node assertion;
  12. map each procedure to a real bucket;
  13. the placeholder tabs are n/a with a roadmap cite;
  14. the namespace filter is blocked;
  15. `audit018-collab` gets an `audit018-norole` role plus collaborator;
  17. a per-round `capture.enabled` override;
  18. pod delete as the closest alternative;
  19. kubelab's own kubeconfig, else blocked;
  20. VolumeSnapshot is blocked;
  21. an in-cluster `audit018-registry`;
  22. retitle the rows and add an S4 finding for the unused phases;
  23. fixtures OK;
  24. a 10-minute boot bound.
- **OD-026:** how the tunnel learns playit's address (F-174).
- **Merges:** the PRs in section 3.

## 6. kubelab facts (captured 2026-09-23)

- `KUBECONFIG=~/kubelab.yaml`, context `default`. Nodes `kubelab-control`, `kubelab-worker-1` and `kubelab-worker-2`, on k3s `v1.36.2+k3s1`.
- Helm release `gameplane` in `gameplane-system`, revision 6, labelled chart `0.2.0-beta.8`. It runs side-loaded images `gameplane-test/{api,operator,web,agent,sentinel}:016`.
- Site settings to preserve: `image.registry=gameplane-test`, `image.tag=016`, `operator.addressManager=metallb`, `ingress.host=gameplane.local`, `defaultModuleSource` git `ref: main`, `crds.autoApply.enabled=false`.
- Pre-existing GameServers (never write to them): `mc-fabric` and `soak-bogus-pool` (control), `soak-no-preference` and `squad` (worker-1; `squad` is Failed), `soak-pool-west` (worker-2). ModuleSources `default` and `uploads`.
- API access: `kubectl port-forward -n gameplane-system svc/gameplane-web 18080:80` and `GP=http://127.0.0.1:18080` (OD-016). The API DB is at migration 011, so OD-005 path (b), refined by OD-023.
- Baseline JSON: `audit/evidence/baseline/`. Tools: `audit/tools/snapshot.sh`, `snapshot-diff.sh`, `cleanup-check.sh`.
