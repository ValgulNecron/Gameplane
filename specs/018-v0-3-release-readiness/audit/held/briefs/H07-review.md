# Review H07: auth identity lifecycle hardening

- Branch: `fix/018-harden-auth-identity-lifecycle`, commit `c9c04f5f`, base `origin/master` @ `61f50265`
- Worktree: `/tmp/claude-1000/-home-valgul-project-Gameplane/0cbfe030-6e64-40ca-9a18-d291dbf5b40a/scratchpad/wt-H07`
- Reviewer: opus (tier above the implementer)
- **Verdict: pass.** One caveat: I could not re-run the compile checks myself (see section 6).

## 1. Diff against the brief

- 13 files changed, matching brief section 5 exactly: `CHANGELOG.md`, `api/cmd/{bootstrap.go,bootstrap_test.go,main.go}`, `api/internal/auth/{oidc.go,oidc_rolemap_test.go,registry.go,registry_test.go}`, `api/specs.md`, `docs/{install.md,security.md}`, `test/e2e/{api_breakglass_e2e_test.go,buckets.sh}`.
- I checked mechanically that every non-blank added line in `git diff origin/master...HEAD` appears verbatim in the brief. None is missing. The implementer's "no deviations" claim holds.
- No path under `specs/*/audit/held/` and no `SECURITY_AUDIT.md` is in the commit.
- The commit is signed off and carries the `Co-Authored-By` and `Claude-Session` trailers. The subject and body are neutral.

## 2. Each finding, checked against the code

### Role re-evaluation keyed to the effective policy (`oidc.go:468-471`)
- `syncRole` now reads `resolvePolicy`, the same policy `computeRole` and `getMatchedGroup` use.
- The Helm provider's base policy is always non-nil when OIDC is configured (`main.go:177`, `&ProviderPolicy{... RoleMappings: nil}`). `effectiveHelmPolicy` returns a non-nil `RoleMappings` whenever an override is present. So an override alone now turns re-evaluation on, which matches the finding's "Expected" line.
- Dashboard providers are unchanged. The override merge is gated on `providerName == HelmProviderName` (`oidc.go:448`), so for them `resolvePolicy == o.policy`, as before.
- Regression test `TestHandleCallback_HelmOverrideMappingsReEvaluateWithoutHelmMappings`:
  - It uses the real production base shape (`&ProviderPolicy{}`).
  - It covers the three cases the finding asks for: first login gives admin, the last-user-manager guard skips the demotion with no audit event, and a demotion applies once a second manager exists.
  - The guard counts `users.role` (`db/rbac.go:95`), so the `seedUser(..., "admin")` row with no binding counts correctly.
  - It fails before the fix at login 3: the role stays `admin` and no second audit event is written. It is a real regression test.

### Dashboard-provider role-assignment audit (`registry.go`, `main.go:225-234`)
- `build` attaches the registry's audit func and `SetProviderName(p.Name)`.
- Setting the name is safe: the override gate needs the name `"helm"`, and `validateAuth` rejects that name (`handlers/config.go:630`). `OIDCFor("helm")` always returns the legacy provider and never builds from the row (`registry.go:284`), so even a hand-edited row can't pull a dashboard provider into the override merge.
- `AttachAuditWriteSyncFunc` clears the build cache, so a provider built before the call is rebuilt.
- `main.go` calls it once, right after `audit.New` and before any route is mounted (routes start at line ~270). Minor, not blocking: `build` reads `auditWriteSync` outside the lock that `OIDCFor` later takes to store the entry. A build running during an attach could therefore cache a provider without the func. The only call is at startup, before serving, so nothing can hit this.
- Tests:
  - `TestRegistry_DashboardProviderAuditsRoleAssignment` checks the exact reason string with `provider=corp`. It can't compile without the new method, so it fails before the fix.
  - `TestRegistry_AttachAuditWriteSyncFuncRebuildsCachedProviders` checks cache invalidation and the provider name.
- The E2E waiver is justified. The fake IdP serves only the Helm client and redirect URI (`deploy/kind/e2e.sh:263-264`). Adding a second client is a harness change that needs sign-off, as the fix plan says.

### `--enable-local-login` keeps the rest of the auth row (`bootstrap.go:171-209`)
- The row is now decoded top-level as `map[string]json.RawMessage`. Only `providers` is rewritten, and every other key is written back byte-for-byte.
- Edge cases are handled: a JSON-`null` row, a missing `providers` key, and `null` provider entries.
- Bare `return err` calls now wrap their errors with `%w` (CLAUDE.md rule 6).
- `TestBootstrap_EnableLocalLoginKeepsRestOfAuthConfig` fails before the fix, because `helmOverride` was dropped. It also covers the explicit-empty-list case that mattered for the per-role semantics.

### `--force` ends the account's sessions (`bootstrap.go:125-129`)
- It uses the same `SessionStore.DeleteForUser` as the dashboard reset (`handlers/users.go:655`).
- Here a failure returns an error, where the dashboard only logs. That is the stricter and better choice for a break-glass command: it runs after the password update, and rerunning it is idempotent.
- The first-time create path has no sessions to end, so it correctly skips the delete.
- `TestBootstrap_ForceEndsExistingSessions` checks that the reset account's session count is 0 and another account's is 1. It fails before the fix.
- E2E `TestAPI_BootstrapAdminForceEndsExistingSessions`:
  - It uses its own account and both `--password-stdin` and `--force`, so it survives a rerun.
  - It calls `BootstrapAdmin` first, which serializes behind the process-wide `sync.Once`.
  - It checks for a 401 on `/users/me` with the old cookie. `APIClient.Do` does not log in again on a 401, so the check is real.
  - The path and the helpers match the existing `TestAPI_BootstrapAndLogin`.

## 3. Legitimate callers and side effects

- The E2E `BootstrapAdmin` helper passes `--force`, so from now on it also ends `e2e-admin` sessions. This is safe:
  - It runs once per `go test` process, and `Once.Do` blocks concurrent callers until it finishes.
  - `deploy/kind/e2e.sh` does not pre-create `e2e-admin`, so no `e2e-admin` session can exist in a process before its one bootstrap completes.
  - The `ratelimit` process that follows the main bucket in the same job runs sequentially, so it can't end live sessions.
- Argon2 memory: the new E2E adds two in-container hashes plus one login in `api-mods`. The API limit is 1Gi in e2e (`e2e.sh:310`, sized for about 4 concurrent hashes), so this is acceptable. If `api-mods` ever shows OOM kills, look here first.
- Behaviour change for installs that set only a dashboard `helmOverride` and no Helm mappings: the IdP becomes authoritative for existing users, so manual promotions are re-evaluated at the next login. The finding intends this, and the brief's release-note text covers it.
- A hand-edited all-absent override (`{}`) now also enables re-evaluation. The brief already notes this: the dashboard reset route collapses `{}` to nil, so it is only reachable by hand-editing the row.

## 4. Tests, buckets, login budget

- No existing test was changed or weakened. Five unit tests and one E2E were added, each named after the control it checks.
- `buckets.sh`: the new E2E is listed once, in `api-mods`, with a neutral comment.
- Login budget (fix-plan rule 7): `api-mods` still has 5 `e2e-admin` logins. It gains one login under a fresh username on the shared per-IP limiter (burst 10), for 6 in total. No `e2e-admin` login is added. The test does not write `helmOverride.roleMappings` and is not in `api-auth`. Compliant.

## 5. Docs, specs, CHANGELOG

- `docs/security.md`:
  - The break-glass paragraph and the new `--force` paragraph are accurate.
  - The re-evaluation paragraph now lists all three sources of mappings and is accurate. For the Helm provider, the override "counts even when the Helm values set no mappings", which holds because the base policy is non-nil whenever Helm OIDC is configured.
  - The audit provider-name bullet is accurate.
- `docs/install.md`: accurate.
- `api/specs.md`:
  - The `bootstrap-admin` bullets are accurate.
  - The new "Re-evaluation trigger" and "Audit (FR-014)" bullets match the code. First logins and role changes are audited. Unchanged roles are not. Guard skips are logged but not audited.
- `CHANGELOG.md`: adds a `### Security hardening` heading at the end of `## [Unreleased]`, directly before `## [0.3.0-rc.1]`, with one bullet in the required `- **api:** hardened ...` form. Master had no such heading, so creating it was correct.

## 6. Compile checks

- **I did not re-run them.** Every shell command in this review session was refused by the session's auto-mode safety check, including read-only `git` and `echo`. The check said the refusal came from earlier conversation content, not from the command. I carried out all the checks above with file reads only, plus two `git diff` outputs captured before the block.
- The implementer reports that these passed: `go build ./... && go vet ./...` in `api/`, `go vet -tags e2e ./...` in `test/e2e/`, `buckets.sh verify` (133 tests), and `gofmt -l` with no output.
- A static read agrees:
  - The imports that were added (`encoding/json`, `reflect`) are both used.
  - The helpers the new tests use exist with matching signatures: `dsnIn`, `mustOpen`, `runBootstrap`, `newAuthDB`, `seedUser`, `seedConfigRow`, `authRow`, `providerRow`, `staticSecrets`, `secretFor`, `newFakeIDP`, `callbackViaIDP`, `auditWriteRecorder`, `KubectlWithStdin`, `APIClient`, `BootstrapAdmin`.
  - `authRegistry` is in scope at `main.go:234`.
- **Action for the orchestrator:** re-run the four compile checks from brief section 4 in a session where the shell works, before the PR is opened.

## 7. Wording leaks

- I read every git-bound line: code, comments, test names, the E2E doc comment, the `buckets.sh` comment, docs, `specs.md`, the CHANGELOG and the commit message.
- None contains a finding ID, a group ID, "held", or any repro or description of misuse.
- `FR-014` is a pre-existing spec requirement ID that is already used in the repo, not a finding ID.
- **wordingLeak: false.**

## 8. Follow-ups (off-git)

- The orchestrator re-runs the compile checks (section 6).
- H18 lands after this PR. Whichever of H07 and H18 merges second keeps a single `### Security hardening` heading.
- The maintainer updates the Fix column in `held/findings.md` once the PR number exists.
