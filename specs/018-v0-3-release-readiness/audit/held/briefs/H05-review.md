# Review H05: release signing order and signing-environment scope

Held review (OD-019). Never committed.

- **Worktree:** `/tmp/claude-1000/-home-valgul-project-Gameplane/0cbfe030-6e64-40ca-9a18-d291dbf5b40a/scratchpad/wt-H05`
- **Branch:** `fix/018-harden-release-signing`, one commit `16407ee1` on merge-base `61f50265` (= `origin/master`, the brief's scouted base)
- **Verdict:** **pass**

## Scope check

`git diff --stat origin/master...HEAD`: exactly the 7 paths the brief lists (`release.yaml`, `publish-edge.yaml`, `images.yaml`, `republish-modules.yaml`, `CHANGELOG.md`, `docs/contributing.md`, `docs/module-authoring.md`). Nothing under `specs/`, no `SECURITY_AUDIT.md`. `git diff --check` clean. Worktree clean. `release.yaml:15` (`timeout-minutes: 30`) untouched, as the brief requires.

Every AFTER block in the brief (edits 01-19) is present verbatim in the resulting files (release, publish-edge, images and republish-modules read in full; docs and CHANGELOG diffs read).

## Against the findings

F-246 (tags before signature):
- `require signing key` now runs right after checkout, before QEMU/buildx/login/build, in release `images`, publish-edge `images`, images `common-base` and `game-images`. The chart and modules jobs already checked before their push.
- Every image build uses `outputs: type=image,name=<lowercased>,push-by-digest=true,name-canonical=true,push=true` with no `tags:`/`push: true`, so nothing tagged is published at build time.
- Sign and strict verify (unchanged) run against `@${DIGEST}`, and only then does `oras tag` attach the metadata tags. A final loop checks each tag with `oras resolve` against the signed digest. A guard fails closed if a metadata tag doesn't belong to `IMAGE` (both sides are lowercase, because metadata-action lowercases `images:`).
- `release.yaml` images matrix has `fail-fast: false`. The re-run policy is documented in `docs/contributing.md`.
- The expected state in the finding ("push by digest, sign, then attach tags"; key check before any push; fail-fast false with a re-run policy) is met in full.

F-247 (workflow half):
- Every job that reads `COSIGN_*` declares `environment: release-signing`: release `images`, `chart` and `modules`, publish-edge `images`, republish-modules `modules`, and images `common-base`/`game-images` (conditional on non-PR). `grep -l COSIGN_ .github/workflows/*` returns only those four files. `github-release` does not read the key and has no environment, which is correct.
- The repository-settings half (D16: create the environment and its deployment policy, move the secrets, add a `v*` tag ruleset) is the maintainer's job and is listed in the brief's HELD NOTES. F-247 stays open until then, as the plan says.

Chart (`helm push` tags at push time) and module bundles (submodule `build.sh`) are left out on purpose and recorded as residuals in the brief. That is acceptable for this group.

## Correctness notes (non-blocking)

- PR path in `images.yaml`: `push: false`, `outputs: ''`, no login and no environment. That matches the base's behaviour (build only), and the `build-base-pr` OCI-layout path is untouched.
- The master path of `game-images` still pins the base by `needs.common-base.outputs.steamcmd-digest`, so it doesn't depend on the base tag, which is now attached later.
- `oras` gets credentials from `~/.docker/config.json`, written by `docker/login-action` in each job before the attach step.
- Residual assumption, called out in the brief: an empty `environment:` expression result on `pull_request` means "no environment". This PR's own `images.yaml` PR run exercises it. If GitHub rejects it, the brief's fallback applies: a separate gated signing job.
- Side effects to expect: failed or cancelled runs leave untagged digests on GHCR (GC noise only), and each matrix leg creates a deployment record under `release-signing`.
- Until D16 is done, the docs sentence "its deployment policy admits only master and v* tags" describes a setting that doesn't exist yet. That's fine as long as D16 lands with or right after the merge, per the brief's order.

## Tests / E2E

- No tests were added, changed or removed, and the plan calls for none ("No E2E applies. actionlint runs in CI"). The optional structural check needs sign-off and was correctly not added.
- No `buckets.sh` change. No logins, so the budget is unaffected.

## Compile / static checks (run by reviewer)

- `actionlint` on the 4 workflows: rc 0, no output.
- `zizmor --offline --config .github/zizmor.yml` on the 4 workflows: "No findings to report (10 suppressed)". The suppressions come from the unchanged repo config.
- `grep -c '^    environment:'`: release 3, publish-edge 1, images 2, republish-modules 1, as expected.
- No Go, chart or TS changes, so no go build, helm template or tsc was needed.

## CHANGELOG

`### Security hardening` is created at the end of `## [Unreleased]` (line 150), just before `## [0.3.0-rc.1]` (line 154). The line is `- **ci:** hardened the release signing order and the scope of the signing key.`, which matches the required format.

## Wording leak check

- Grepped the full diff and the commit message for `F-NNN`, `Hxx`, `OD-0`, `held`, `exploit`, `attacker`, `repro` and `unsigned window`. The only hits are "held by this environment" in comments, meaning the key is stored there. That's neutral, not a reference to held material.
- The comments, docs, commit message and CHANGELOG describe the control ("a tag only ever names a signed image", "a run on any other ref never receives the key") without any account of how the previous gap could be used.
- The commit is signed off and carries the required `Co-Authored-By` and `Claude-Session` trailers.
- **wordingLeak: false.**

## Issues

None blocking.
