# H09 review: harden account removal and share-link revocation

- **Worktree:** `/tmp/claude-1000/-home-valgul-project-Gameplane/0cbfe030-6e64-40ca-9a18-d291dbf5b40a/scratchpad/wt-H09`
- **Branch:** `fix/018-harden-account-removal-cleanup`, one commit `a4868695` on `origin/master` = `61f50265` (the brief's base; merge-base matches)
- **Reviewer tier:** opus (tier-up for held work, fix plan rule 6)
- **Verdict: PASS**

## 1. Diff vs brief

- The committed diff's +/- lines are byte-identical to `briefs/H09.patch`. I checked this by comparing the change lines of `git diff origin/master HEAD` with the patch.
- 15 files: 11 modified and 4 new, exactly the list the brief expects. The worktree is clean.
- Nothing under `specs/*/audit/held/` is touched, and neither is `SECURITY_AUDIT.md`.
- Commit: `fix(api): ...`, signed off, with the Co-Authored-By and Claude-Session trailers.

## 2. Findings coverage

| Finding | Expected | Delivered | OK |
|---|---|---|---|
| F-079: SSO re-provisioning after delete | A deleted SSO user is provisioned again. users.go deletes explicitly. | `db.Store.DeleteUser` deletes `oidc_links` in the same transaction. Migration 012 purges existing orphans. Unit test `TestResolveOrLinkUser_DeletedUserIsProvisionedAgain`, plus the E2E SSO half. | yes |
| F-095: creator's share links stay valid | Revoke or delete the creator's links on both drivers. `GET /shares/{token}` gets the uniform 404. | `DeleteUser` revokes the links (`revoked_at`). Migration 012 revokes links whose creator is already gone. Handler test `TestShareLinkStopsResolvingWhenCreatorDeleted` asserts the 404 and the exact uniform body on resolve and on start. The E2E share half checks the same. | yes |
| F-094: revoke matches id and cluster only | Match cluster, namespace, server_name and id. A not-found sentinel; the handler answers 404 and leaves the row unchanged. | The SQL matches all four columns. Empty cluster now defaults to `"local"`, the same as Create and List. `ErrShareLinkNotFound` maps to 404 in `revokeShareHandler`. Store test `TestRevokeShareLink_ScopedToServer` asserts the sentinel for each of the 3 mismatched dimensions and checks that `revoked_at` stays NULL. Handler test `TestShareRevokeScopedToPathServer` covers the 404 and the link still resolving. | yes |
| Public F-082 (absorbed): 500 → 404 on an unknown id | 404 | `TestShareRevokeUnknownIDReturns404` and `TestRevokeShareLink_UnknownIDReturnsNotFound` | yes |

**D10** ("explicit deletes plus cleanup migration 012") is followed. The DSN and FK settings are unchanged, and the migration is numbered 012 (the next free number; the last on master is `011_user_theme_preferences.sql`).

## 3. Code correctness

**Tables covered.** Every table with a `REFERENCES users(id)` is handled:

- deleted in `DeleteUser` and in 012: `sessions`, `oidc_links`, `api_tokens`, `user_role_bindings`, `user_preferences`
- revoked in both: `share_links`

`audit_events` carries no FK and is correctly left alone.

**Transaction.** `DeleteUser` runs inside `BeginTx`. `DeleteUserBindings(ctx, tx, id)` accepts the `Execer` (`rbac.go:40`), so the bindings delete is inside the transaction too. The old code deleted the bindings outside any transaction and only logged a warning on failure. Every error is wrapped with `%w`.

**User ids are not reused.** `users.id` is `INTEGER PRIMARY KEY AUTOINCREMENT` (`001_init.sql`), so the "fresh user id" assertions (unit and E2E) are sound: SQLite won't hand the max rowid out again.

**Migration runner compatibility.** `splitStatements` splits on `";\n"`. The only `;` inside the new file's header comment is mid-line (`...as the user; this migration...`), so the split is unaffected. The `strftime('%Y-%m-%dT%H:%M:%SZ','now')` output matches the `time.RFC3339` UTC format that `shares.go` writes.

**Handler callers.**

- `revokeShareHandler` already had `ns` from `resolveNS` and `name` from the path, and it checks ownership on `{name}` before the revoke. Scoping the SQL to that same `(ns, name)` closes the gap without affecting a legitimate owner who revokes through the right server.
- `RevokeShareLink` has no other production caller (grep).
- `users.del` keeps its self-delete and last-manager guards unchanged ahead of `DeleteUser`.
- `slog` is still used elsewhere in users.go, so no import goes unused.

**Would the new tests fail before the fix?** Yes. With the old code:

- The scope tests would get 204 (or nil), not 404 (or the sentinel).
- The unknown-id tests would get 500 (or an unwrapped error).
- The DeleteUser, migration and SSO tests exercise rows that the old handler left behind, and would observe them.

## 4. Existing tests changed

10 existing `RevokeShareLink` call sites had to change for the new signature:

- 7 in `db/shares_test.go`
- 3 in `handlers/shares_test.go`

Each one passes the link's own `Namespace` and `ServerName`, or fixed values for the nonexistent-id case, so every assertion keeps its intent. That includes the cluster-scoping tests, which still expect the wrong-cluster revoke to fail and the right-cluster revoke to succeed. Nothing was weakened or deleted. This matches the implementer's flag.

## 5. E2E, buckets and login budget

**Registration.** `TestAPI_AccountRemoval_RevokesSharesAndAllowsSSOReprovision` is added to `bucket_api_mods`. `buckets.sh verify` reports: "133 tests, all in exactly one bucket".

**Login budget (rule 7).**

- One `e2e-admin` `APIClient` login takes api-mods from 5 to 6, within the ceiling of about 7.
- Two OIDC callbacks count against `OIDCCallbackLimiter` (burst 10), which is separate from `LoginLimiter`. No other api-mods test uses OIDC.
- The test writes no `roleMappings` and no auth config, and it is not in `api-auth`.

**Fixture.** The shapes match existing tests: the busybox template is the same as in `api_mods_e2e_test.go`, and the `gameplane.local/owner-id` annotation is the one `isServerOwner` reads. The resources carry `t.Parallel()` and UnixNano-unique names. Cleanups cover both user ids, the template and the GameServer.

**Lockout guard.** The SSO user is in the admin group, so it counts as a user manager. `e2e-admin` stays a manager too, so the delete passes the guard.

## 6. Specs, docs and CHANGELOG

**`api/specs.md`.** It documents the DELETE `/users/{id}` semantics, the scope and 404 behavior of share revocation, and the 012 entry, including the forward-only rollback note that OD-023 and R6 ask for. All of it is accurate against the code.

**`docs/security.md`.** The new "Account removal" bullet and the revocation sentence are accurate.

**CHANGELOG.** `### Security hardening` is created at the end of `## [Unreleased]`, directly before `## [0.3.0-rc.1]`. The bullet reads `- **api:** hardened account removal and share-link revocation.`, which is the required format.

## 7. Wording leak check

I scanned the commit message and every added line for:

- finding and group IDs (`F-NNN`, `Hxx`, `OD-`)
- "attack", "exploit", "bypass", "vulnerab", "orphan", "repro", "held" and "audit/"

**Result: no leaks.** The only hits are "lockout guard" (the existing docs term for the last-manager guard) and "Reprovision" in a test name. Both are neutral. The pre-existing "an attacker probing tokens" comment in `shares.go` is unchanged context, not a new line.

Test names state controls: `TestRevokeShareLink_ScopedToServer`, `TestDeleteUser_RemovesAccountRowsAndRevokesShareLinks`, `TestMigrate_AccountCleanupClearsRowsOfRemovedUsers`, and so on.

## 8. Compile checks (run by reviewer)

- `api`: `go build ./...` OK, `go vet ./...` OK
- `api`: `go vet -tags postgres ./internal/db/` OK
- `test/e2e`: `go vet -tags e2e ./...` OK
- `gofmt -l api test/e2e`: clean
- `bash test/e2e/buckets.sh verify`: OK (133)

No tests were run (repo rule 8).

## 9. Non-blocking notes (no change required)

1. **Postgres dialect.** Migration 012 uses the SQLite-only `strftime`. Its header comment says that on Postgres "every statement matches nothing". In fact the statement would not parse there. This is not a regression:
   - earlier migrations already use `datetime('now')`
   - `runMigration` inserts with `datetime('now')`
   - all migrations use `?` placeholders

   So the experimental Postgres path can't run this migration set today. If a Postgres migration path is added later, 012 needs a dialect-neutral timestamp. Consider rewording the comment at merge time; it doesn't block.
2. **Postgres share links are deleted, not revoked.** On Postgres, `DELETE FROM users` still cascades to `share_links`, so the creator's links end up deleted rather than kept as revoked. The result is secure either way (the token stops resolving). The docs' "revokes" wording is exact on SQLite, the shipped driver.
3. **No budget comment in `buckets.sh`.** Unlike earlier additions, the api-mods entry has no running-tally comment in `buckets.sh`. The test's doc comment does record the budget. This is optional.
