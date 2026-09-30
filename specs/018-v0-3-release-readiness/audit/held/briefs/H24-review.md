# H24 review: `fix/018-pin-dev-ingress-manifest`

- Reviewer: Opus 5.5 (tier-up review, read-only)
- Worktree: `/tmp/claude-1000/-home-valgul-project-Gameplane/0cbfe030-6e64-40ca-9a18-d291dbf5b40a/scratchpad/wt-H24`
- Commit: `dda0082bb5dd8ce2dbbfc918750e0ae4c69e621e` on top of `origin/master` `61f50265` (the brief's base)
- Verdict: **pass**

## Diff vs brief

`git diff origin/master...HEAD --stat`: `CHANGELOG.md` (+4) and `deploy/kind/up.sh` (+7/-2). These are exactly the two files in brief section 5. Nothing under `specs/*/audit/held/` and no `SECURITY_AUDIT.md`. The worktree is clean.

- `up.sh` hunk 1: `INGRESS_NGINX_VERSION="controller-v1.15.1"` and its two-line comment sit right after `METALLB_VERSION`. Matches the brief's AFTER block word for word.
- `up.sh` hunk 2: the echo prints the version, and the `kubectl apply -f` URL is quoted and uses `${INGRESS_NGINX_VERSION}` in place of `main`. Matches the brief word for word.
- `CHANGELOG.md`: a new `### Security hardening` heading with a single bullet, placed at the end of `## [Unreleased]` directly above `## [0.3.0-rc.1]`. The base had no heading of that name, so creating one is correct. The line follows the required format `- **<area>:** hardened <neutral phrase>.`

## Correctness

- The change closes the gap in F-235. The finding's "Expected" asks for an immutable `controller-vX.Y.Z` tag held in a variable next to `METALLB_VERSION`, and that is what the diff does. `grep` finds no other reference to `ingress-nginx/main` or to the upstream `deploy.yaml` anywhere in the tree outside `held/`.
- Upstream check (read-only): `git ls-remote` resolves `refs/tags/controller-v1.15.1` to `0a5901f3…`, which matches the brief. `curl -I` on the pinned raw URL returns HTTP 200. The path exists at the tag, so `make dev-up` will not fail on a 404.
- Legitimate callers are unaffected. `make dev-up` still installs ingress-nginx inside the same `if ! kubectl get ns ingress-nginx` guard, and the `wait` selector is unchanged. The brief's held notes show the manifest blob at the tag is identical to `main`'s, so dev clusters get the same content as today. CI is unaffected because `e2e.sh` does not install ingress-nginx.
- Shell: `bash -n deploy/kind/up.sh` passes (I re-ran it). The line continuation and quoting are correct, and the variable is defined unconditionally at the top level before it is used.

## Tests, buckets, login budget

The group adds no automated test, as the fix plan intends ("none automated", dev-only script). No existing test was touched, `buckets.sh` needs no entry, and the login budget is unaffected.

## specs / docs

The fix plan says `specs.md: no` and `design.pen: no`, and neither was touched. No docs reference the unpinned URL.

## Wording leaks

I checked every line that goes into git: the commit subject and body, both `up.sh` comments, the echo text and the CHANGELOG bullet.
- They contain no F-IDs and no Hxx IDs.
- They contain no repro steps and nothing about how the gap could be used. The comment states only the pinning rule ("a controller release tag, never a branch").
- The branch name `fix/018-pin-dev-ingress-manifest` follows the plan.

wordingLeak: none.

## Minor notes (no action required)

- The commit trailer reads `Co-Authored-By: Claude Haiku 4.5`, while the brief's template has `Opus 5.5`. Haiku 4.5 is the model that actually made the commit, and CLAUDE.md rule 11 asks for the current running model, so the trailer is accurate. `Claude-Session` and `Signed-off-by` are present.
- Maintainer follow-up (carried over from the brief's held notes, not part of this branch): upstream ingress-nginx is archived, so `controller-v1.15.1` is the final release. Deciding whether to move the dev cluster to another ingress controller or to Gateway API is a separate question.
- Manual verification is still owed by the maintainer: run `make dev-down && make dev-up` and confirm the controller pod reaches Ready.
