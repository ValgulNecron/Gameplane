# Review: .github/workflows/

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `specs/done_008-hardened-github-actions/spec.md` (FR-001 to FR-021, SC-001 to SC-006), `specs/done_008-hardened-github-actions/contracts/permissions-matrix.md`, `specs/018-v0-3-release-readiness/research.md` R4, `specs/018-v0-3-release-readiness/contracts/rc-deploy.md` §1, `docs/contributing.md` ("CI and workflows", "Release process"), `docs/install.md:10-60`, `docs/security.md:435-512`, `docs/module-authoring.md:200-215,425-445`

## Scope reviewed

Read in full:
- `.github/workflows/ci.yaml` (all 1541 lines), `release.yaml`, `publish-edge.yaml`, `images.yaml`, `republish-modules.yaml`, `screenshot-refresh.yaml`, `visual-diff.yaml`
- the composite actions they call: `.github/actions/{build-e2e-images,e2e-images,go-cache,dump-cluster-state}/action.yml`
- `.github/zizmor.yml`, and `modules/build.sh`, which the release and republish jobs call
- `docs/contributing.md`

Read in part, or to cross-check a claim:
- `.github/dependabot.yml` (ecosystem and directory entries only)
- `Makefile` (the `cover-go-merge`, `cover-go-check`, `check-*` and `images` targets the workflows call)
- `charts/gameplane/Chart.yaml`
- `charts/gameplane/values.yaml` (`image`, `defaultModuleSource`, `updates`, `capture.image`)
- `charts/gameplane/templates/_helpers.tpl` (image helpers), `charts/gameplane/templates/api.yaml:295-325`
- `operator/internal/oci/client.go:63-80`
- `test/e2e/buckets.sh` (header, bot-heavy block, bucket names), `test/e2e/lint-gate-verify.sh` (header)
- `web/vitest.config.ts`, `web/package.json` scripts
- the component Dockerfiles' `COPY` and `VERSION` lines
- `CHANGELOG.md` headings
- `git show a9e2f79e`

Not reviewed: `deploy/kind/*.sh` (the chart/deploy chunk). Workflows were not run and no registry was queried. The tag behaviour described below comes from reading `docker/metadata-action` semantics, not from observing a run. T014 checks it live on rc.1.

## Method

1. Read each workflow top to bottom, then traced the tag and prerelease path for `v0.3.0-rc.N` and `v0.3.0` through `release.yaml`: metadata-action tag rules, the chart `sed`, `helm push`, the `gh release create` arguments and the modules job.
2. Compared every `ci.yaml` path filter against the files each gated job reads, and compared the `publish-edge.yaml` path filter against each matrix image's Dockerfile `COPY` lines.
3. Diffed the `report` job's `needs:` list against `NEEDS_ORDER` and `JOB_MATCHERS`.
4. Tabulated `timeout-minutes`, `permissions` and `concurrency` for every job with a short Python YAML read, and checked them against spec 008 FR-001/002/004/005 and the permissions matrix.
5. Used `git log -S` to confirm when the Go coverage status poster was removed.

## Observations (no finding)

- **RC tags.** For `v0.3.0-rc.N`, `release.yaml:53-56` should yield `v0.3.0-rc.N` (`type=ref,event=tag`) and `0.3.0-rc.N` (`type=semver,pattern={{version}}`). metadata-action handles a prerelease with a non-`{{version}}` pattern by falling back to `{{version}}`, so `{{major}}.{{minor}}` creates no `0.3` tag. Its `latest=auto` is decided by the highest-priority semver entry, which is false for a prerelease. This matches research R4. It is still to be confirmed live in T014.
- **Final tags.** For `v0.3.0` the same lines yield `v0.3.0`, `0.3.0`, `0.3` and `latest`.
- **Chart version.** `release.yaml:148-149` sets the chart `version`/`appVersion` to the tag without the `v`. `_helpers.tpl:8` falls back from `image.tag` to `.Chart.AppVersion`, so the chart pulls `:0.3.0-rc.N`, which the images job creates.
- **Image coverage.** All 12 images the chart references are in the release matrix (`release.yaml:21-36`): operator, api, web, agent, sentinel, capture-sidecar, mcp-server, telemetry-receiver, audit-syslog-bridge and the three tunnels.
- **Release page.** `release.yaml:248-250` marks any tag containing `-` as `--prerelease`. The CHANGELOG awk (`release.yaml:228-232`) matches `## [0.3.0-rc.N] — date` headings in the existing format. `gh release view` makes a re-run idempotent.
- **rc-deploy.md line references.** The line numbers `contracts/rc-deploy.md` cites (`release.yaml:226-236`, `248-250`, `148-149`, `180`) still point at the right code.
- **Permissions, timeouts and concurrency.** Every job in all 7 workflows has an explicit `timeout-minutes` and a job-level `permissions` block, and every top-level block is `contents: read` (FR-001/002/004). Concurrency groups exist on every push/PR workflow (FR-005). Timeouts above 30 are the five e2e jobs (60) and `publish-edge.images` (35), matching `docs/contributing.md:62` and the permissions matrix.
- **`ci.yaml` `changes` permissions.** The job declares only `pull-requests: read` (`ci.yaml:84-85`), which leaves `contents` at none. The matrix contract row says `contents: read`. This works on a public repo and is narrower, not broader, than the contract. Noted only as contract drift.
- **Module bundles.** `release.yaml:311-313` and `republish-modules.yaml:63-65` run `modules/build.sh push --tag-latest` on every `v*` tag, including `-rc.N`. That moves each bundle's `:latest`. The operator drops non-semver tags (`operator/internal/oci/client.go:63-65`), so in-cluster catalogs are unaffected. `docs/module-authoring.md:210-211` asks for exactly this dual tagging.
- **Module signing.** The key check happens before any push for module bundles (`release.yaml:279-286`, `modules/build.sh:100-102`), so `docs/module-authoring.md:441-442` holds for the missing-key case.
- **actionlint download.** `workflow-lint` pins it by sha256 (`ci.yaml:705-709`).
- **Coverage gates.** `make cover-go-check` enforces a threshold for all 14 `GO_MODULES`, including `gp-module` (80%). Each `go` matrix leg only gates the module it just profiled, because the others have no profile and are skipped (`Makefile:126-140`).
- **Cancelled master runs.** `ci.yaml:47-49` cancels in-flight runs per ref, including on `master`. A master commit superseded by another merge within the run's duration therefore never gets a completed CI run. This is spec-compliant (FR-005). For RC-08, tag only a commit whose CI run finished.
- **`bot-heavy`.** No workflow runs the `bot-heavy` bucket. That's deliberate (`test/e2e/buckets.sh` comment above `bucket_bot_heavy`).

## Candidate findings

### C-github-workflows-01: The CI report never counts or lists a failed `capture-sidecar-setcap-proof`

- **Location**: `.github/workflows/ci.yaml:1247-1252` (`NEEDS_ORDER`), `ci.yaml:1257-1275` (`JOB_MATCHERS`), compared with `needs:` at `ci.yaml:1139-1159`
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. `report.needs` lists 18 jobs, including `capture-sidecar-setcap-proof` (`ci.yaml:1144`).
  2. `NEEDS_ORDER` lists 17 and leaves it out. `JOB_MATCHERS` has no entry for its name ("capture-sidecar CAP_NET_RAW survival").
  3. The pass/fail tally loops only over `NEEDS_ORDER` (`ci.yaml:1279-1299`).
  4. So a PR where only the setcap proof fails, for example a Dockerfile change that drops the file capability, gets a sticky comment and job summary saying "0 failed", with no "Failing" table, while the run is red.
- **Expected**: Spec 008 FR-016: the reporter is "consolidating status across all matrix legs". `contracts/permissions-matrix.md:58-60`: a new job needs "the `needs:` list, the `NEEDS_ORDER` array, and the `JOB_MATCHERS` map. Miss any one and the new job is silently absent from the PR comment."
- **Actual**: One of the three edits was made. The job is missing from the tally and the failing-jobs table.

### C-github-workflows-02: `publish-edge` path filter leaves out the sources of 6 of the 12 images it publishes

- **Location**: `.github/workflows/publish-edge.yaml:17-29` (paths), `publish-edge.yaml:53-68` (matrix), `publish-edge.yaml:3-4,8,15-16` (comments). Related wording in `docs/install.md:29` and `charts/gameplane/values.yaml:18-19`.
- **Category**: correctness / docs-drift
- **Suggested severity**: S3
- **Observation / repro**:
  1. The trigger paths are `operator/ api/ web/ agent/ netguard/ audit-syslog-bridge/ telemetry-receiver/ mcp-server/ go.work go.work.sum **/Dockerfile` plus the workflow file.
  2. The matrix also builds `sentinel`, `capture-sidecar` and the three `tunnel-*` images. Their sources, and the shared modules copied into images, are not in the list:
     - `sentinel/**`, `capture-sidecar/**` and `tunnel/**`. The tunnel Dockerfiles are named `tunnel/Dockerfile.{frp,tailscale,playit}`, which `**/Dockerfile` does not match.
     - `gameaction/**`: copied by `api/Dockerfile:8` and `agent/Dockerfile:8`.
     - `gameproto/**`: `sentinel/Dockerfile:7`.
     - `svcutil/**`: `capture-sidecar/Dockerfile:7`.
     - `gp-module/**`: `api/Dockerfile:9`.
  3. A master merge touching only `sentinel/` or only `gameaction/` therefore publishes nothing. `:edge` for sentinel, or for api and agent, stays on the older build.
  4. A chart installed with `image.tag=edge` resolves every component image through `gameplane.imageTag` (`_helpers.tpl:8,32-79`), so edge users run the stale images.
- **Expected**: `publish-edge.yaml:15-16`: "Only rebuild when something that lands in an image changes." `publish-edge.yaml:3`: "on every push to master". `docs/install.md:29`: "Every push to `main` publishes rolling `:edge` images."
- **Actual**: Changes that land in 6 of the 12 images don't trigger a publish. The header comment at line 8 also lists only 6 images: `{operator,api,agent,audit-syslog-bridge,telemetry-receiver,mcp-server}`. Two docs say "every push", and `docs/install.md:29` and `values.yaml:19` name the branch `main`, but it is `master` (`publish-edge.yaml:14`). The workaround is a manual `workflow_dispatch`.

### C-github-workflows-03: The doc gates don't run when the files they read change

- **Location**: `.github/workflows/ci.yaml:153-173` (`docs` filter), `ci.yaml:150-152` (`specs` filter), `ci.yaml:381` (lint job `if:`), `ci.yaml:396-406` (the gate steps). Inputs: `hack/check-doc-versions.sh:44` (reads `charts/gameplane/Chart.yaml` line 6), `hack/check-links.sh:311` (target files anywhere in the repo).
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. `check-links` and `check-doc-versions` run only in the `lint` job. That job runs only when the `go`, `specs` or `docs` filter matches. The `docs` filter is the 18 audited files plus the two scripts.
  2. Case A: a PR that bumps only `charts/gameplane/Chart.yaml` `appVersion` (for example to `0.3.0-rc.1`) sets only `charts=true`. `lint` is skipped, so the version checker never compares the docs with the new appVersion. The bump PR is the one change it exists to guard.
  3. Case B: a PR that archives spec 012 per CLAUDE.md rule 16 (`git mv specs/012-docs-refresh-and-outreach specs/done_012-…`) and misses the link at `docs/contributing.md:155` has only `.md` files in its diff. The workflow-level `!**.md` (`ci.yaml:18`) means CI doesn't start at all, and the filters would skip `lint` anyway. A rename of `docs/agent-architecture.md`, linked from `docs/architecture.md`, triggers the workflow, but `lint` is still skipped because that file isn't in the `docs` list. Both merge green.
  4. The broken link or stale version then fails the next unrelated PR that touches a docs file.
- **Expected**: The gates run whenever an input they read changes: `charts/gameplane/Chart.yaml`, and the files linked from the audited docs (`docs/**`, `specs/012-*/outreach.md`, `CHANGELOG.md`, `LICENSE`, `CLAUDE.md`, `cosign*.pub`, `docs/img/*`).
- **Actual**: They run only when a checked document or a checker script changes.

### C-github-workflows-04: A change to `.golangci.yml` alone doesn't run golangci-lint

- **Location**: `.github/workflows/ci.yaml:101-144` (no filter lists `.golangci.yml`), `ci.yaml:381`
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. The root `.golangci.yml` is the config every `golangci-lint-action` leg uses. golangci-lint looks for its config in parent directories of each module's `working-directory`.
  2. A PR that edits only `.golangci.yml`, for example to enable a new linter that flags existing code, produces `go=false`, `ci=false`, `specs=false` and `docs=false`.
  3. `lint (…)` is skipped for all 15 modules, and the PR merges green.
  4. The next PR that touches any Go module then fails lint on code it didn't change. A loosening edit, which CLAUDE.md rule 4 forbids, also merges with no lint run to show its effect.
- **Expected**: CI config paths force the lint job on. The `ci` filter already does this for `Makefile`, `go.work` and the workflow itself.
- **Actual**: `.golangci.yml` is in no filter.

### C-github-workflows-05: The report's Go coverage path is dead because no Go job posts a `coverage/<module>` status

- **Location**: `.github/workflows/ci.yaml:1334-1338` (`parseGoGate`), `ci.yaml:1370-1385` (non-web branch), `ci.yaml:1126-1127`, `ci.yaml:1165`
- **Category**: dead-code / docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. The only commit-status poster in any workflow is the web job, with context `coverage/web` (`ci.yaml:568-584`). A grep for `statuses` across `.github/` confirms this.
  2. Commit `a9e2f79e` (2026-06-14) removed the Go job's "report coverage (commit status)" step "to isolate the go failure" and said it "Will reinstate go coverage reporting via a safer mechanism". It was never reinstated.
  3. So the `else` branch that pairs a Go module's status with `<module>/.testcoverage.yml` never sees a Go context. It runs only for `coverage/web` if parsing `vitest.config.ts` fails.
  4. The "Coverage" table shows only `web`, or nothing on a Go-only PR.
- **Expected**: The comment at `ci.yaml:1126` says it publishes "measured coverage against each gate".
- **Actual**: No Go gate is ever shown. The comment at `ci.yaml:1165` ("published near web:287") also points at a stale line. The status step is now at `ci.yaml:568`.

### C-github-workflows-06: The report's "e2e buckets run" list leaves out `ratelimit`

- **Location**: `.github/workflows/ci.yaml:1410-1420`, compared with `ci.yaml:816` and `ci.yaml:860-870`
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. The `api-auth` leg of `e2e-go` runs the `ratelimit` bucket as a tail step (`tail: ratelimit`, `ci.yaml:816,864`).
  2. `bucketSet` is built only from job names (`^e2e (operator|api-auth|…) \/`, multicluster, upgrade) and from the game-bot result.
  3. A run where `api-auth` executed and its ratelimit tail ran lists `api-auth` but never `ratelimit`.
- **Expected**: `ci.yaml:1127-1128` says the report shows "which e2e buckets actually ran" (the phrase wraps across both lines).
- **Actual**: `ratelimit` never appears.

### C-github-workflows-07: `coverage/web` is posted as `success` even when the coverage threshold fails

- **Location**: `.github/workflows/ci.yaml:567-584`
- **Category**: correctness
- **Suggested severity**: S4
- **Observation / repro**:
  1. `npm run test:cover` runs vitest with `thresholds` (`web/vitest.config.ts:37-42`) and the `json-summary` reporter (`vitest.config.ts:23`). Vitest writes the summary before it applies the thresholds.
  2. If lines drop below 92%, the step fails, but `coverage/coverage-summary.json` exists.
  3. The next step runs `if: always()` and POSTs `"state":"success"`, hard-coded at `ci.yaml:584`.
- **Expected**: The status state reflects whether the gate passed (`failure` below threshold). `docs/security.md:448-449` says the `web` job uses `statuses: write` "to mark PR checks".
- **Actual**: A green `coverage/web` check sits next to a red `web` job. The `report` table shows the negative margin, but the status itself is always green.

### C-github-workflows-08: The failure dumper writes only to the job log, not to step summaries or artifacts

- **Location**: `.github/actions/dump-cluster-state/action.yml:50,56,71,89,103,117,184-212` (all output goes to stdout)
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. Every dump command pipes through `redact` to stdout.
  2. The action never writes `$GITHUB_STEP_SUMMARY` and never calls `upload-artifact`. None of the calling jobs in `ci.yaml` upload the dump either.
  3. A failed e2e job's diagnostics therefore exist only in the collapsed step log.
- **Expected**: Spec 008 US2 acceptance scenario 2 (`spec.md:49`) says the dumper "exports pod descriptions, controller logs, and game server container logs to GitHub step summaries and artifacts". `docs/security.md:509-510` ("Diagnostics redaction and secret confinement") says redaction happens "before any data reaches `$GITHUB_STEP_SUMMARY`, before any artifact is uploaded".
- **Actual**: Nothing reaches either destination. Redaction itself is applied on every emit path. This finding is only about where the output goes.

### C-github-workflows-09: `dependabot.yml` has no `gomod` entry for `gp-module`

- **Location**: `.github/dependabot.yml:7-190` (14 `gomod` entries), compared with `go.work:4-19` (15 modules)
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. `go.work` lists `./gp-module`, a module with its own `go.mod` that requires `gopkg.in/yaml.v3` and `k8s.io/apimachinery v0.37.0`. `api` also depends on it through `replace` (`api/go.mod:18-23`).
  2. `dependabot.yml` has `gomod` entries for agent, api, audit-syslog-bridge, capture-sidecar, gameaction, gameproto, mcp-server, netguard, operator, sentinel, svcutil, telemetry-receiver, test/e2e and tunnel. It has none for `/gp-module`.
  3. Dependabot never opens version-update PRs for gp-module's dependencies, so they drift from the rest of the workspace.
- **Expected**: Spec 008 SC-003: "Dependabot monitors all 14 Go submodules … without omitting any repository component". FR-017 asks for "all … Go modules". The count of 14 predates gp-module.
- **Actual**: One of the 15 workspace modules isn't covered.

### C-github-workflows-10: The local workflow-lint command in `docs/contributing.md` matches no files

- **Location**: `docs/contributing.md:49-50`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**:
  1. `docs/contributing.md:50` says to run `actionlint .github/workflows/*.yml`.
  2. Every workflow is `*.yaml`. `ls .github/workflows/*.yml` returns "no matches found".
  3. In bash, actionlint gets the literal pattern and errors. In zsh, the shell errors before actionlint runs.
  4. Line 49 also installs `actionlint@latest`, while CI pins v1.7.9 (`ci.yaml:706`).
- **Expected**: A command that lints the workflows, for example `actionlint` with no arguments, as `ci.yaml:711-715` does, or `*.yaml`.
- **Actual**: The documented pre-push check never lints anything.

## Questions (not findings)

- **Go versions.** CI unit, envtest and lint jobs run Go `1.26` (`.github/actions/go-cache/action.yml:18-19`, `go.work:1`), but all 13 component Dockerfiles build with `golang:1.27-alpine`. Released binaries come from a toolchain only the e2e jobs exercise. Is that intended?
- **CRD sync check.** No workflow compares `operator/config/crd/*.yaml` with `charts/gameplane/crds/`. `ci.yaml:662` only diffs the chart's two copies with each other. Nothing checks either that `make manifests`/`make generate` output is committed. Today all 9 CRDs are byte-identical across the three directories (checked with `cmp`). Is a CI gate wanted? It is not required by spec 008 FR-009.
- **Module version tags.** `release.yaml:311-313` re-pushes every bundle at `<name>:<module.yaml version>` on every `v*` tag. If a module's content changed in the submodule without a version bump, the existing version tag is silently overwritten. `values.yaml:451-453` says "a new release ADDS a version rather than replacing one". Does `gameplane-module` CI enforce version bumps?
- **Release page vs modules.** `github-release` (`release.yaml:223`) needs `images` and `chart` but not `modules`, so the release page is published even if bundle publishing fails. contracts/rc-deploy.md §1 expects "Module bundles were republished". Should `modules` be in `needs`?
- **Moving `latest` tag.** Tagging a patch on an older line after `v0.3.0`, for example `v0.2.1`, would move image `latest` back to it, because `latest=auto` sets it for any non-prerelease semver tag. Is that intended?
- **`images.yaml` PR path.** On PRs, the game-image job relies on `steps.build.outputs.digest` being empty (`images.yaml:199-212`), not on the event name. If build-push-action ever reports a digest for a non-pushed build, the PR build would try to pull a non-existent registry digest.
- **Dockerfile vs go.mod.** The tunnel Dockerfiles copy `gameaction/` and say tunnel "depends on [it] via a local replace (../gameaction)" (`tunnel/Dockerfile.frp:4-8`), but `tunnel/go.mod` has no requires and no replace. This is for the tunnel reviewer.

## Held

held candidates: 2 (see OD-019)
