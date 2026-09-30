# H26 review

Verdict: **pass**

Branch `fix/018-harden-security-docs-accuracy`, commit `7d823aac`, base `61f50265` (= current `origin/master`, merge-base matches). Worktree `wt-H26`.

## Scope
- Diff: exactly `CHANGELOG.md`, `docs/install.md`, `docs/security.md` (+32/-10). No code, no tests, nothing under `specs/*/audit/held/`, no `SECURITY_AUDIT.md`. Worktree clean.
- All five edits match the brief's AFTER blocks verbatim. `security.md:236-238` (podSecurity paragraph) and the `capture.enabled` default line are untouched, as required.
- D4 (docs now, code later) respected: no operator change.

## Accuracy vs code (checked in worktree)
- Agent sidecar hardening: `gameserver_controller.go:2182-2189` sets RunAsNonRoot, RunAsUser, ReadOnlyRootFilesystem, AllowPrivilegeEscalation=false, drop ALL, seccomp RuntimeDefault. "Same fixed hardening" is correct.
- Game container: `gameContainerSecurityContext` (`:1981-1990`) returns nil or only RunAsUser/RunAsGroup; `gamePodSecurityContext` only fsGroup. New paragraph and capture-exception paragraph are accurate; `NET_RAW`/`allowPrivilegeEscalation: true` appear only on the capture sidecar (`:1580-1611`).
- Anchors: `module-authoring.md:1008 ### Security context` -> `#security-context`; `security.md:225 ## Pod security` -> `#pod-security`. Both resolve.
- OIDC: `defaultGroupsClaim` (`oidc.go:111-116`) maps empty to "groups"; mapping active when any Helm role list non-empty (`api/cmd/main.go:169-176`) or a DB override exists (`effectiveHelmPolicy` builds RoleMappings from the override even when the Helm seed is nil). "New OIDC users get viewer, existing roles never re-evaluated" matches `values.yaml:254-257` and the `computeRole` comment. install.md now agrees with values.yaml and security.md.
- F-250 Expected and F-229 Expected are both met.

## Wording / hygiene
- No finding IDs, group IDs, or exploit description in any added line or the commit message (the only "exploit" match is a removed line). "which may be root" states posture, not a use.
- Commit signed-off, carries Co-Authored-By and Claude-Session trailers, neutral message.
- CHANGELOG: new `### Security hardening` heading at end of `[Unreleased]`, directly before `## [0.3.0-rc.1]`, one line in the required `- **<area>:** hardened ...` form.
- `git diff --check` clean. There are no compile checks to run because the change is Markdown only. CI doc gates (check-links, check-specs, check-doc-versions) will verify it.

## Notes for orchestrator (not blocking)
- The hot-spot order says H26 merges last among held docs edits to `security.md`/`install.md`. Rebase before push if other held groups land first.
- Still open, outside this PR: the maintainer check on whether `podSecurity.enforceRestricted=true` admits game pods (F-250 step 6), and the later D4 operator code change.
