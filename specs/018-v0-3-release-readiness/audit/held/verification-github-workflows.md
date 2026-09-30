# T045 ci chunk: independent verification (opus)

Held (OD-019, off-git). Verifier input: `audit/held/review-github-workflows.md` (opus reviewer), candidates H-github-workflows-01 and H-github-workflows-02.

**Method.** I checked each control against master `13a859ff`. This branch has no diff from master under `.github/`.

What I read:
- the image jobs of `release.yaml:11-125` and its chart job at `:150-205`
- `publish-edge.yaml`, `images.yaml`, `republish-modules.yaml`, and the key check in `modules/build.sh:100-102`
- the control statements: `release.yaml:68-69`, `docs/contributing.md:140-148` and spec 008 `spec.md:176` (Assumptions)
- a grep of every workflow for `environment:` and `github.ref` guards

Read-only `gh api` calls against the repository settings:
- `actions/secrets` (names only)
- `environments`
- `rulesets`
- `rules/branches/master`

I did not trigger or re-run any workflow and did not contact the registry for these items. I ran no test or lint suite. Severity follows research R3. Both items need a trusted role or a failed run to show, and neither exposes key material, so neither is S1.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| H-github-workflows-01 | kept | S3 | Confirmed by step order. In `release.yaml` the image is pushed with its full tag list (`:57-68`) before the key-presence check (`:70-77`) and before signing (`:90-107`). The images matrix (`:19-36`) doesn't set `fail-fast: false`, so a failed leg cancels siblings that may have pushed but not yet signed. `images.yaml` has the same order (`:61-84`, `:224-250`). `publish-edge.yaml` does too (`:87-111`), but it sets `fail-fast: false` (`:51`), so there only a failed signing attempt applies. The chart job checks the key before its push (`release.yaml:155-162` before the `helm push` step at `:169-180`), and so does `modules/build.sh:100-102`. |
| H-github-workflows-02 | kept | S3 | Confirmed from repository settings. `workflow_dispatch` is enabled on `publish-edge.yaml:30`, `republish-modules.yaml:10-11` and `images.yaml:4`. No workflow has an `environment:` key or a `github.ref` job guard. `COSIGN_PRIVATE_KEY` and `COSIGN_PASSWORD` are repository-level Actions secrets. The only environment is `copilot`, and the only ruleset (`18692396`) targets branches, with no tag rule. So the confinement stated in spec 008 (`spec.md:176`) is a convention, not an enforced setting. Using it takes write access, and fork PRs never receive these secrets, so this is a weakened control rather than a boundary open to outsiders. |

### H-github-workflows-01

**Location:**
- `.github/workflows/release.yaml:57-68` (build-push with `push: true` and every metadata tag), `:70-77` (key-presence check) and `:90-107` (keyed sign with 3 attempts). `:19-36` is the images `strategy`, which has no `fail-fast: false`.
- The same push-then-check order appears in `.github/workflows/images.yaml:61-75` and `:76-84` (common base, sign at `:94`), in `images.yaml:224-240` and `:242-250` (game images, sign at `:260`), and in `.github/workflows/publish-edge.yaml:87-99` and `:104-111` (sign at `:144`).

**Control:** Official tags point only at signed artifacts. It is stated at `release.yaml:68-69` ("Signing is mandatory for an official release: fail rather than publish unsigned images") and at `docs/contributing.md:146-148` ("Signing is **mandatory and fail-closed**").

**Repro / observation** (how to confirm the control holds):
1. Read `release.yaml:46-68`. The build step pushes every tag `docker/metadata-action` produced (`:54-56`). For a final tag such as `v0.3.0` that is `v0.3.0`, `0.3.0`, `0.3` and `latest`. The push happens before the "require signing key" step (`:70`) and the "sign image" step (`:90`) run.
2. Read `release.yaml:19-20`. The `strategy` block sets only `matrix`, so GitHub's default `fail-fast: true` applies: when one leg fails, the in-progress legs are cancelled, whatever step they are on.
3. From steps 1 and 2: a run that stops between push and sign leaves the tags on a digest with no signature until the job is re-run. That happens when signing exhausts its retries (`:97-107`) or when a sibling leg fails. The job still fails, so the gap is not silent, but publication comes before the check.
4. After every release run, including failed or cancelled ones, confirm with `cosign verify --key cosign.pub ghcr.io/valgulnecron/gameplane/<component>:<tag>` for all 12 components (`release.yaml:21-36`) and every tag that run created. Also confirm `cosign verify --key cosign.pub ghcr.io/valgulnecron/charts/gameplane:<version>` for the chart. Do the same for `:edge` after a failed `publish-edge` run.
5. After a fix, confirm by reading the order: the key check runs before any push step, and tags are attached only to a digest that has already been signed.

**Expected:** An official tag only ever resolves to a digest whose signature already exists. One approach is to push by digest, sign, and then attach tags. At minimum, the key check should run before any push, and the matrix should use `fail-fast: false` with a documented re-run policy. `docs/contributing.md:146-148` then holds at every point of a run, not just at its end.

**Actual:** Every image job fails closed, but only after it has published. The unsigned window lasts until someone re-runs the job. I did not observe this live, since that needs a failing release run. The digest in the window is still the project's own CI build, and a consumer who verifies gets a verification failure, not a false pass.

### H-github-workflows-02

**Location:**
- `.github/workflows/publish-edge.yaml:30` (`workflow_dispatch: {}`), `.github/workflows/republish-modules.yaml:10-11` (`on: workflow_dispatch`) and `.github/workflows/images.yaml:4` (`workflow_dispatch:`). None of their jobs has an `environment:` or an `if: github.ref == …` guard (grep over `.github/workflows/`).
- Repository settings: Actions secrets `COSIGN_PRIVATE_KEY` and `COSIGN_PASSWORD` are repository-scoped, the only environment is `copilot`, and the only ruleset is `18692396` ("protect main", target `branch`).

**Control:** Signing-secret confinement. Spec 008 `spec.md:176` (Assumptions) says: "Production signing secrets (`COSIGN_PRIVATE_KEY`) remain strictly restricted to master branch and release tag workflows (`publish-edge.yaml`, `release.yaml`, `images.yaml`, `republish-modules.yaml`)". This complements the protected-master ruleset (one human approval, no self-approval).

**Repro / observation** (how to confirm the control holds):
1. Read the `on:` blocks of the three files. Each accepts `workflow_dispatch`, and GitHub lets the person dispatching choose the ref.
2. Run `grep -n 'environment:\|github.ref ==' .github/workflows/*.yaml`. It finds no job-level ref guard or environment on any signing job.
3. `gh api repos/ValgulNecron/Gameplane/actions/secrets --jq '.secrets[].name'` lists `COSIGN_PASSWORD` and `COSIGN_PRIVATE_KEY` at repository scope. `gh api repos/ValgulNecron/Gameplane/environments` lists only `copilot`. Repository-scoped secrets are not tied to a ref, so adding a ref guard to these three files is necessary but not sufficient. The setting that actually confines the key is environment scoping with a deployment policy.
4. `gh api repos/ValgulNecron/Gameplane/rulesets` returns a single branch-target ruleset. Nothing restricts who can create `v*` tags, and `release.yaml` signs whatever commit such a tag points at.
5. After a fix, confirm the following:
   - The `COSIGN_*` secrets are gone from the repository list and exist only in an environment (for example `release-signing`) whose deployment policy allows `refs/heads/master` and `refs/tags/v*` only (Settings → Environments).
   - Every signing job declares that environment.
   - A tag ruleset limits who can create `v*` tags.

**Expected:** The signing key is available only to runs on `master` or on a `v*` release tag, and this is enforced by repository configuration, as spec 008's assumption states.

**Actual:** Confinement relies on convention. Any account with write access can run the signing workflows against a ref other than master, and those runs get the key. Fork pull requests are unaffected, because they receive no secrets.
