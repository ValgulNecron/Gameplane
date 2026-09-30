# Held review candidates: .github/workflows/ (OD-019, off-git)

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `specs/done_008-hardened-github-actions/spec.md` (Assumptions, FR-014), `specs/done_008-hardened-github-actions/contracts/permissions-matrix.md` (Secret confinement), `docs/contributing.md` "Release process", in-file comments of the release workflows

These concern supply-chain controls (artifact signing and signing-key confinement), so they stay out of `evidence/review-github-workflows/notes.md` until fixed. Each entry names the control, where it lives, the behaviour it should have, and how a maintainer confirms it holds. No reproduction against the live registry was attempted.

## Candidate findings

### H-github-workflows-01: A published tag can exist before its signature does

- **Control**: "only signed artifacts are published under official tags" (signing is mandatory and fail-closed).
- **Location**: `.github/workflows/release.yaml:57-77` (push at 57-67, key-presence check at 70-77, signing at 90-107), `release.yaml:19-36` (images matrix without `fail-fast: false`); same order in `.github/workflows/publish-edge.yaml:87-111` and `.github/workflows/images.yaml:61-84` and `224-250`. The stated contract is `release.yaml:68-69` ("Signing is mandatory for an official release: fail rather than publish unsigned images") and `docs/contributing.md:147-148` ("Signing is **mandatory and fail-closed**: if `COSIGN_PRIVATE_KEY` is not configured, the release job fails.").
- **Category**: correctness
- **Suggested severity**: S3
- **Observation**:
  1. In each image job, `docker/build-push-action` runs with `push: true` and the full tag list (for a final tag that includes the moving `0.3` and `latest`) before the "require signing key" step and before `cosign sign`.
  2. So if the key secret is absent, or all three signing attempts fail, the job fails, but only after the tags already point at an unsigned image.
  3. The release `images` matrix uses the default `fail-fast: true`. When one leg fails, GitHub cancels the others, including any leg that has pushed but not yet signed.
  4. The chart job (`release.yaml:155-203`) and `modules/build.sh:100-102` check the key before pushing, so only the missing-signature-after-failed-signing case applies to them.
- **Expected**: The key check runs before any push. Official tags only point at a digest after its signature exists, for example by pushing by digest, signing, then tagging, or by `fail-fast: false` plus a re-run policy. Either way, `docs/contributing.md:147-148` stays true.
- **Actual**: The job fails, but only after publishing. The unsigned window lasts until someone re-runs the job.
- **How a maintainer confirms the control holds**: read the step order in the three workflows. After every release run, including failed or cancelled ones, run `cosign verify --key cosign.pub ghcr.io/valgulnecron/gameplane/<component>:<tag>` for all 12 components and every tag the run created (`vX.Y.Z[-rc.N]`, `X.Y.Z[-rc.N]`, and for finals also `X.Y` and `latest`).

### H-github-workflows-02: The signing key can be used from refs that are not master and not a release tag

- **Control**: signing-secret confinement. Spec 008 Assumptions (`specs/done_008-hardened-github-actions/spec.md:176`): "Production signing secrets (`COSIGN_PRIVATE_KEY`) remain strictly restricted to master branch and release tag workflows (`publish-edge.yaml`, `release.yaml`, `images.yaml`, `republish-modules.yaml`)".
- **Location**: `.github/workflows/publish-edge.yaml:30` (`workflow_dispatch: {}`), `.github/workflows/republish-modules.yaml:11`, `.github/workflows/images.yaml:4`. No job in these files has an `environment:` or an `if: github.ref == 'refs/heads/master'` guard (checked with grep).
- **Category**: correctness
- **Suggested severity**: S3
- **Observation**:
  1. `workflow_dispatch` can be started on any branch that contains the workflow file. Every branch cut from master contains it.
  2. `publish-edge.yaml` run that way builds the branch's code, pushes it as `:edge` and `:sha-<short>`, and signs it with the project key. `republish-modules.yaml` pushes and signs the bundles pinned by that branch's `modules/` pointer. `images.yaml` pushes and signs game images tagged with the branch name.
  3. This path skips the review that the protected-master ruleset (`18692396`, one human approval) enforces for master.
- **Expected**: Signing jobs run only for `refs/heads/master` or `refs/tags/v*`. Enforce this with a ref guard on the job, or put the secrets in an environment whose deployment-branch policy allows only master and `v*` tags.
- **Actual**: Anyone with write access can start them from any branch, and the key is available there.
- **How a maintainer confirms the control holds**: check each signing workflow for a ref guard or an `environment:` with a branch policy (Settings → Environments). Also check that the `COSIGN_*` secrets are environment-scoped rather than repository-scoped. Separately, confirm whether a tag ruleset limits who can create `v*` tags, since `release.yaml` signs whatever commit a `v*` tag points at.

## Questions (not findings)

- `release.yaml:177` passes the job token on the command line (`helm registry login ghcr.io -u … -p ${{ secrets.GITHUB_TOKEN }}`). `screenshot-refresh.yaml:103-105` avoids exactly this ("the secret never appears in argv (the runner's process table is not masked)"). The token is job-scoped and short-lived, and no spec sentence requires stdin. Should the chart job use `--password-stdin` for consistency?
- `ci.yaml:524` and `Makefile:163` install `setup-envtest@release-0.24`, which is a moving branch ref. By contrast, `Makefile:48-51` pins `gocovmerge` on purpose ("Pinned, not @latest…"). The checksum database still verifies whatever pseudo-version resolves, and the binary is cached forever under a fixed key (`ci.yaml:513`). Is a pinned pseudo-version wanted here?
