# T045 ci chunk: independent verification (opus)

**Method.** I tried to refute each candidate in `notes.md` (written by another opus reviewer) against master `13a859ff`. This branch has no diff from master under `.github/`, `Makefile`, `hack/` or `docs/`.

What I read in `.github/workflows/ci.yaml`:
- the workflow-level `paths` (`:3-43`)
- the `changes` filters and the combine step (`:94-209`)
- the setcap proof (`:271-373`), the `lint` job (`:378-460`), the `web` job (`:550-590`), the `e2e-go` matrix (`:790-870`) and the `report` job (`:1125-1470`)

Other files read:
- `publish-edge.yaml`, `release.yaml:1-215` and `images.yaml`
- `.github/dependabot.yml` (every `directory:` line) and `go.work`
- the `COPY` lines of all 12 component Dockerfiles, and `.github/actions/dump-cluster-state/action.yml`
- spec 008: `spec.md`, `data-model.md` (E4 and E5) and `contracts/permissions-matrix.md`
- `docs/contributing.md`, `docs/install.md:25-32`, `docs/security.md:440-512` and `audit/findings.md`

Read-only `gh` calls:
- the `publish-edge.yaml` run history against the master merge history
- the master branch rules
- the diff of open PR #420, which edits the `docs` filter

I ran no workflow, test or lint. The two held candidates for this component are verified separately, off-git.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-github-workflows-01 | kept | S3 | Confirmed. `report.needs` has 18 entries, including `capture-sidecar-setcap-proof` (`:1144`). `NEEDS_ORDER` (`:1247-1252`) has 17, and `JOB_MATCHERS` (`:1257-1275`) has no matcher for "capture-sidecar CAP_NET_RAW survival". The tally (`:1277-1299`) loops only over `NEEDS_ORDER`, so a run where only the setcap proof fails reports "0 failed" and shows no Failing table. The PR checks list is still red, which is the workaround. |
| C-github-workflows-02 | kept | S3 | Confirmed, including live. No `publish-edge` run exists for master merge `9e1c7bc3` (#399, a capture-sidecar `go.mod` bump) or `b21684ed` (#402, a bump of the tunnel Dockerfiles' base images). The next run came from `26e7d444` (#410, a web change). Count corrected: 7 of 12 images have at least one input the filter doesn't list. For 5 of them (sentinel, capture-sidecar and the three tunnels), that includes the component's own directory. Every run rebuilds all 12 images, so the stale window ends at the next push to a listed path or at a manual dispatch. |
| C-github-workflows-03 | kept | S3 | Confirmed. A PR that only changes `charts/gameplane/Chart.yaml` sets `charts=true` and nothing else, so `lint` (`:381`) is skipped and `check-doc-versions` never compares the docs against the new appVersion. A rename of `docs/agent-architecture.md` (linked at `docs/architecture.md:7`) starts the workflow, but no filter matches it. A PR that only `git mv`s a spec folder never starts CI (`!**.md`, `:18`, `:33`). Open PR #420 adds its fixture paths to the `docs` filter but not `Chart.yaml`. This is relevant to the rc.1 appVersion bump. |
| C-github-workflows-04 | kept | S3 | Confirmed. `.golangci.yml` is the only golangci-lint config (`git ls-files`; the others are test fixtures), and no filter at `:101-173` lists it. A PR that only edits it sets `go`, `specs` and `docs` false, so all 15 `lint` legs are skipped. |
| C-github-workflows-05 | rejected | n/a | Spec 008 `contracts/permissions-matrix.md:17` records that exactly one step, `web` → "report coverage (commit status)", posts a status. So a report whose Coverage table has no Go rows is the specified state. Every Go gate is still enforced in its own `go` leg by `make cover-go-check` (FR-010). What's left is an `else` branch that only runs if parsing `vitest.config.ts` fails, and a stale line reference in a comment (`web:287`, now `:568`). Both are style. The "will reinstate" note in `a9e2f79e` is a maintainer TODO, not a requirement. |
| C-github-workflows-06 | kept | S4 | Confirmed. The `api-auth` leg runs the `ratelimit` bucket as a tail step (`:816`, `:860-870`), and `buckets.sh:302` lists `ratelimit` as its own bucket. `bucketSet` (`:1410-1420`) never adds it, so the report is incomplete on every green run. It has no effect on CI results. |
| C-github-workflows-07 | kept | S4 | Confirmed. The step at `:568-584` runs `if: always()` and posts `"state":"success"` hard-coded, whatever the threshold result. Master has no required status checks (`gh api …/rules/branches/master` returns only `deletion`, `non_fast_forward` and `pull_request`), so a green `coverage/web` misleads but can't let a PR through. The red `web` job still shows the failure. |
| C-github-workflows-08 | rejected | n/a | Spec 008 `data-model.md` E5 says the dumper's lifecycle is "collect → redact → write to stdout → captured as the job log". It adds that `action.yml` "has no `upload-artifact` step and no `$GITHUB_STEP_SUMMARY` write", and gives a forward-looking rule for if a sink is ever added. Under CLAUDE.md rule 15 the spec folder includes `data-model.md`, so stdout-only is the specified behaviour, and it is more specific than the US2 acceptance wording. `docs/security.md:509-510` states an ordering ("before any data reaches…"), which holds. |
| C-github-workflows-09 | kept | S3 | Confirmed. There are 14 `gomod` directories, and `/gp-module` isn't among them, while `go.work` has 15 modules. This breaks spec 008 SC-003/FR-017 and the E4 invariant `count(gomod entries) == count(module lines in go.work)` (`data-model.md:125-133`). The same section's "COVERAGE GAP" note predicted this drift. The practical effect is limited: the api image builds with api's own `go.mod` versions, so what drifts is `gp-module/go.mod` itself. |
| C-github-workflows-10 | kept | S4 | Confirmed. All 7 workflows are `*.yaml`, so `actionlint .github/workflows/*.yml` (`docs/contributing.md:50`) errors in zsh ("no matches found") and in bash (actionlint can't open the literal pattern). CI runs `actionlint` with no arguments (`ci.yaml:711-715`), pinned to v1.7.9 (`:705-709`), while the doc installs `@latest`. |

### C-github-workflows-01

**Location:** `.github/workflows/ci.yaml:1247-1252` (`NEEDS_ORDER`), `:1257-1275` (`JOB_MATCHERS`) and `:1277-1299` (the tally), compared with `report.needs` at `:1139-1159` (the entry is at `:1144`). The job is defined at `:271-274`, and its display name is "capture-sidecar CAP_NET_RAW survival".

**Repro / observation:**
1. Read `:1139-1159`. `report` needs 18 jobs, one of them `capture-sidecar-setcap-proof`.
2. Read `:1247-1252`. `NEEDS_ORDER` lists 17 keys and leaves out `capture-sidecar-setcap-proof`. Read `:1257-1275`: no matcher recognises "capture-sidecar CAP_NET_RAW survival".
3. Read `:1277-1299`. `passed`, `failed`, `skipped`, `cancelled` and `failingRows` are computed only from `NEEDS_ORDER`. The headline (`:1437`) prints `failed`, and the "Failing" table (`:1439-1445`) is rendered only when `failed > 0`.
4. Take a PR with `go=true` where every job passes except the setcap proof. For example, a `capture-sidecar/Dockerfile` change that loses the file capability makes the proof `exit 1` at `:362-373`. On that PR the job summary and the sticky comment say `0 failed`, with no Failing table, while the workflow run is red.

**Expected:** Spec 008 FR-016 says the reporter consolidates status across all jobs. `contracts/permissions-matrix.md:58-60` says a new job needs "the `needs:` list, the `NEEDS_ORDER` array, and the `JOB_MATCHERS` map. Miss any one and the new job is silently absent from the PR comment."

**Actual:** Only the `needs:` edit was made, so the setcap proof is missing from the tally and from the failing-jobs table.

### C-github-workflows-02

**Location:** `.github/workflows/publish-edge.yaml:17-29` (trigger `paths`), `:53-68` (matrix) and `:3-8`, `:15-16` (comments). The same wording appears at `docs/install.md:29` and `charts/gameplane/values.yaml:18-19`.

**Repro / observation:**
1. Read `publish-edge.yaml:17-29`. The listed paths are `operator/`, `api/`, `web/`, `agent/`, `netguard/`, `audit-syslog-bridge/`, `telemetry-receiver/`, `mcp-server/`, `go.work`, `go.work.sum`, `**/Dockerfile` and the workflow file. `**/Dockerfile` matches `sentinel/Dockerfile` and `capture-sidecar/Dockerfile` only when that file itself changes, and it never matches `tunnel/Dockerfile.{frp,tailscale,playit}`.
2. Compare the `COPY` lines: `sentinel/Dockerfile:7-13` (`gameproto/`, `sentinel/`), `capture-sidecar/Dockerfile:7-13` (`svcutil/`, `capture-sidecar/`), `tunnel/Dockerfile.*:8-14` (`gameaction/`, `tunnel/`), `api/Dockerfile:7-15` (`netguard/`, `gameaction/`, `gp-module/`, `api/`) and `agent/Dockerfile:7-14` (`netguard/`, `gameaction/`, `agent/`). The paths that land in an image but are not listed are `sentinel/**`, `capture-sidecar/**`, `tunnel/**`, `gameaction/**`, `gameproto/**`, `svcutil/**` and `gp-module/**`. They feed 7 images: sentinel, capture-sidecar, the three tunnels, api and agent.
3. Live check: `gh run list --workflow publish-edge.yaml` has no run for master merge `9e1c7bc3` (#399, `capture-sidecar/go.mod` bump, 2026-09-21 23:28 +02:00) or for `b21684ed` (#402, tunnel Dockerfile base bump, 23:30). The next run is for `26e7d444` (#410, a web change, 2026-09-22 00:01).
4. Each run rebuilds all 12 matrix images, so a change under an unlisted path reaches `:edge` only when a later push touches a listed path, or when someone dispatches the workflow by hand. A chart installed with `image.tag=edge` runs the older build until then.

**Expected:** `publish-edge.yaml:15-16` says "Only rebuild when something that lands in an image changes", and `:3` says "on every push to master". `docs/install.md:29` says "Every push to `main` publishes rolling `:edge` images."

**Actual:** A push that changes only an unlisted input publishes nothing. There is also wording drift. The header comment (`:8`) names 6 of the 12 images. `docs/install.md:29` and `values.yaml:19` call the branch `main`, but it is `master` (`publish-edge.yaml:14`).

### C-github-workflows-03

**Location:** `.github/workflows/ci.yaml:150-152` (`specs` filter), `:153-173` (`docs` filter), `:381` (`lint` job `if:`) and `:396-406` (the gate steps). The workflow-level `paths` are at `:16-29` and `:30-43` (`!**.md`, with `docs/**` and three READMEs re-included). The checkers' inputs are `hack/check-doc-versions.sh:44` (line 6 of `charts/gameplane/Chart.yaml`) and `hack/check-links.sh:302-311` (any file in the repo).

**Repro / observation:**
1. Case A (appVersion bump): a PR changes only `charts/gameplane/Chart.yaml` `appVersion` (for example to `0.3.0-rc.1`). The workflow starts, but only the `charts` filter matches, so the combine step (`:186-209`) emits `go=false`, `specs=false` and `docs=false`. `lint` is skipped, and `make check-doc-versions` does not run. The next PR that touches one of the 18 audited docs runs `lint` and fails on every unmarked `0.2.0-beta.8` in those docs.
2. Case B1 (renamed link target): a PR renames `docs/agent-architecture.md`, which `docs/architecture.md:7` links to. `docs/**` starts the workflow, but the `docs` filter lists only the 18 audited files and the two scripts, so `lint` is skipped and the broken link merges green.
3. Case B2 (archived spec): a PR that only `git mv`s `specs/012-docs-refresh-and-outreach` to `specs/done_012-…` (CLAUDE.md rule 16) and misses the link at `docs/contributing.md:155` changes only `.md` files under `specs/`. Those are excluded by `!**.md` (`:18`, `:33`), so CI doesn't start.
4. Open PR #420 adds `hack/test-check-doc-versions.sh` and its testdata to the `docs` filter but not `Chart.yaml`, so Case A stays open after it merges.

**Expected:** The two doc gates run whenever an input they read changes: `charts/gameplane/Chart.yaml`, and the files the audited docs link to (`docs/**`, `CHANGELOG.md`, `CLAUDE.md`, `LICENSE`, `cosign*.pub`, `specs/012-*/outreach.md`).

**Actual:** They run only when one of the 18 audited docs, or one of the two checker scripts, changes. A stale version or broken link then fails a later, unrelated PR.

### C-github-workflows-04

**Location:** `.github/workflows/ci.yaml:101-173` (no filter lists `.golangci.yml`) and `:381` (`lint` job `if:`). The config is the root `/.golangci.yml`, the only non-fixture golangci config in the repo.

**Repro / observation:**
1. Open a PR that edits only `.golangci.yml`, for example to enable a linter that flags existing code. The file is not `.md`, so the workflow starts.
2. The `go`, `ci`, `specs` and `docs` filters (`:101-173`) don't list the file, so the combine step emits `go=false`, `specs=false` and `docs=false`. All 15 `lint (…)` legs and all `go (…)` legs are skipped, and the PR merges green.
3. The lint steps run `golangci-lint-action` with `working-directory: <module>` (`:432-460`). golangci-lint finds the root config by walking up parent directories, so the change first takes effect on the next Go PR, which then fails lint on code it didn't change.

**Expected:** A lint-config change forces the lint job to run, the same way the `ci` filter (`:138-144`) forces everything on for `Makefile`, `go.work` and the workflow itself.

**Actual:** `.golangci.yml` is in no filter.

### C-github-workflows-06

**Location:** `.github/workflows/ci.yaml:1410-1420` (`bucketSet`), compared with `:812-816` (the `api-auth` include with `tail: ratelimit`) and `:860-870` (the ratelimit tail step).

**Repro / observation:**
1. Read `:812-816` and `:860-870`. Every `api-auth` leg (amd64 and arm64) runs `buckets.sh regex ratelimit` as a last step, after its own bucket passes.
2. `test/e2e/buckets.sh:302` lists `ratelimit` as a bucket in its own right.
3. Read `:1410-1420`. `bucketSet` adds names only from job names (`^e2e (operator|api-auth|api-roles|api-rbac|api-agent|api-mods) \/`, multicluster and upgrade), plus `bot-fast` from `needs['e2e-game-bot']`. Nothing adds `ratelimit`.
4. On any green run where the `api-auth` legs ran, the report lists `api-auth` but not `ratelimit`.

**Expected:** `ci.yaml:1127-1128`: the report shows "which e2e buckets actually ran".

**Actual:** `ratelimit` never appears. A correct fix needs step-level information, not just the job name, because the tail is skipped when the `api-auth` bucket step fails.

### C-github-workflows-07

**Location:** `.github/workflows/ci.yaml:568-584` (the "report coverage (commit status)" step of the `web` job; the test run is at `:567`).

**Repro / observation:**
1. Read `:567`. `npm run test:cover` runs vitest with `thresholds` (`web/vitest.config.ts:37-42`) and the `json-summary` reporter (`:23`).
2. Read `:568-584`. The step runs `if: always()`. It exits early only if `coverage/coverage-summary.json` is missing, and otherwise POSTs `"state":"success"`, a literal, for context `coverage/web`.
3. When a threshold fails, vitest writes its reports and then sets a failing exit code, so the summary exists and the step posts `success`. I did not reproduce this with a run (Rule 8), and `web/node_modules` is not installed here, so the order comes from vitest's coverage-provider behaviour, not from reading the installed source. Whatever the order, the posted state never depends on the gate result.
4. `gh api repos/ValgulNecron/Gameplane/rules/branches/master` returns only `deletion`, `non_fast_forward` and `pull_request` rules, so `coverage/web` is not a required check.

**Expected:** The status state reflects the gate (`failure` below threshold). `docs/security.md:448-449` says the `web` job's `statuses: write` is there "to mark PR checks".

**Actual:** A green `coverage/web` check appears next to a red `web` job. It can't let a PR through, but it misreports the coverage gate.

### C-github-workflows-09

**Location:** `.github/dependabot.yml:7-190` (14 `gomod` entries), compared with `go.work:3-19` (15 modules).

**Repro / observation:**
1. `grep -n 'directory' .github/dependabot.yml` gives `gomod` directories `/agent`, `/api`, `/audit-syslog-bridge`, `/capture-sidecar`, `/gameaction`, `/gameproto`, `/mcp-server`, `/netguard`, `/operator`, `/sentinel`, `/svcutil`, `/telemetry-receiver`, `/test/e2e` and `/tunnel`. There is no `/gp-module`.
2. `go.work` lists `./gp-module`. `gp-module/go.mod` requires `gopkg.in/yaml.v3 v3.0.1` and `k8s.io/apimachinery v0.37.0`.
3. Spec 008 `data-model.md:125-133` states the invariant `count(gomod entries) == count(module lines in go.work)`, which is now 14 against 15. Its "COVERAGE GAP" note says a 15th module added without an entry won't be caught automatically, and that is what happened.

**Expected:** Spec 008 SC-003 ("Dependabot monitors all … Go submodules … without omitting any repository component") and FR-017: every `go.work` module has a `gomod` entry.

**Actual:** `gp-module` has no entry, so Dependabot opens no version-update PRs for its `go.mod`. Shipped images are not affected, because the api image builds with api's own required versions.

### C-github-workflows-10

**Location:** `docs/contributing.md:49-50`.

**Repro / observation:**
1. Read `docs/contributing.md:50`: `actionlint .github/workflows/*.yml`.
2. `ls .github/workflows/` gives `ci.yaml`, `images.yaml`, `publish-edge.yaml`, `release.yaml`, `republish-modules.yaml`, `screenshot-refresh.yaml` and `visual-diff.yaml`. None ends in `.yml`.
3. In zsh the glob fails with "no matches found" before actionlint starts. In bash the literal `.github/workflows/*.yml` is passed, and actionlint fails to open it. Either way, nothing is linted.
4. CI runs `actionlint` with no arguments from the checkout root (`ci.yaml:711-715`) and pins v1.7.9 by sha256 (`:705-709`). Line 49 of the doc installs `@latest`.

**Expected:** A command that lints the workflows, such as `actionlint` with no arguments (as CI does) or `.github/workflows/*.yaml`, ideally with the version CI pins.

**Actual:** The documented pre-push check fails without linting anything.
