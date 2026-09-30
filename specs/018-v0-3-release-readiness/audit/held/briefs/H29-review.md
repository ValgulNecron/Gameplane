# H29 review: keep one tunnel credential per provider

- Worktree: `/tmp/claude-1000/-home-valgul-project-Gameplane/0cbfe030-6e64-40ca-9a18-d291dbf5b40a/scratchpad/wt-H29`
- Branch: `fix/018-harden-tunnel-credential-switch`, commit `614be90b` on base `61f50265` (= current `origin/master`)
- Reviewer tier: opus (implementer: haiku)
- **Verdict: pass** (one non-blocking note)

## Diff vs brief

Exactly 4 files: `CHANGELOG.md`, `api/internal/handlers/tunnelcreds.go`, `api/internal/handlers/tunnelcreds_test.go`, `api/specs.md`. Every AFTER block in brief sections 2.1 to 2.7 is in the commit, character for character. Nothing under `specs/*/audit/held/` and no `SECURITY_AUDIT.md` in the commit. Worktree is clean.

## Finding coverage (F-084)

- **Leftover key on provider switch.** The PUT rotation path (Secret already exists and is server-owned) now builds its merge patch with `tunnelCredentialPatch`. That sets every other provider's key to `null` in `data` and `stringData`. On a real apiserver, `data.<key>: null` deletes the key. `stringData.<key>: null` for a key that isn't stored is a no-op under RFC 7386, and the new `stringData` value is merged into `data` as before. Keys that no provider uses are not touched, so the "preserve admin fields" intent of the existing code still holds. The Create path (new Secret) only ever held `body.Values`, so it was never affected.
- **Nondeterministic GET.** `tunnelKeysForSecret` prefers the GameServer's `spec.networking.tunnel.provider` when that provider's keys are present, and otherwise walks the fixed `tunnelProviderOrder`. It no longer ranges over the map. It returns nil when nothing matches, as before. `getNestedString` returns `("", false, nil)` for a missing or non-string provider, so the fallback is taken and there's no new error path in practice. The only error comes from a non-map at an intermediate key, and the preceding `credentialsSecretRef` lookup already hit that same case.
- **Legitimate callers.** `validateTunnelValues` limits `body.Values` to the chosen provider's keys, so the `keep` guard in the patch builder is only a safety net. `tunnel/main.go` reads one key by provider name. The operator mounts the Secret as a volume. `web/src/routes/CreateServer.tsx` uses only the PUT API. The new behaviour breaks none of them. Legacy Secrets that already hold two keys get a deterministic GET answer and are cleaned up on the next PUT, which matches the held notes.

## Tests

- `TestTunnelCreds_ProviderSwitchLeavesOnlyNewProviderKey`: runs frp PUT, then an admin key added out-of-band, then tailscale PUT. It asserts `token` is gone from `Data` and `StringData`, `authKey` is correct, `admin-note` is kept, and 20 GETs all return `[authKey]`. Before the fix, `Data["token"]` survives, so the test fails deterministically. The fake client applies merge-patch nulls to both maps, and `syncSecretData` only copies `StringData` into `Data`, so the assertions show real patch semantics. The fixture GameServer has no spec provider, so the GET goes through the fixed-order fallback, and `authKey` is the only key left.
- `TestTunnelCreds_GetReportsKeysDeterministically`: a Secret holding all three keys, with and without a spec provider, over 20 GETs. Before the fix, each GET is a random 1-in-3 pick, so the test fails with near certainty. It checks both the spec-provider-wins rule and the fixed-order fallback.
- `TestTunnelProviderOrder_CoversEveryProvider` guards against the order slice and the key map drifting apart.
- No existing test was modified. `TestTunnelCreds_RotationPreservesExtraFields` should still pass: same-provider rotation nulls only other providers' keys, and its extra field is not a provider key. That's CI's job to confirm.

## E2E, buckets, login budget

No E2E was added. `buckets.sh` is unchanged and the login budget is unaffected. The brief's held notes ask the maintainer for an S4 waiver, and the fix plan allows either a waiver or one `operator` login. That's the maintainer's call and doesn't block this review.

## Docs and CHANGELOG

- The `api/specs.md` route line is accurate: PUT, GET and DELETE exist, the key names match `tunnelProviderKeys`, and the GET order and the "never the values" statement match the code.
- The CHANGELOG adds a `### Security hardening` heading with one bullet at the end of `## [Unreleased]`, just before `## [0.3.0-rc.1]`, in the required format. No other hardening heading exists on master yet. If H22 lands first, the merge-second branch must keep a single heading (already in the brief's held notes).

## Wording leaks

A scan of all added lines and the commit message found no finding IDs, no group IDs, and no wording about randomness, exploits or stale material. Test names and comments state the control they check. The commit is signed off.

## Compile checks (run by reviewer)

`cd api && go build ./... && go vet ./... && go vet -tags envtest ./...`: all passed. `gofmt -l api/internal/handlers/` is clean.

## Note (non-blocking)

- The commit trailer reads `Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>`, while brief section 6 gave the Opus 5.5 line. CLAUDE.md rule 11 asks for the current running model, and haiku made the commit, so the trailer is defensible. The implementer's "no deviations" report missed it. No change needed unless the maintainer wants the brief's exact text. Changing it would mean amending before the push, which is allowed because the commit isn't pushed.
