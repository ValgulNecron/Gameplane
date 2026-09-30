# H22 review: `fix/018-harden-api-input-limits`

- Worktree: `/tmp/claude-1000/-home-valgul-project-Gameplane/0cbfe030-6e64-40ca-9a18-d291dbf5b40a/scratchpad/wt-H22`
- Commit: `e79938fb` on base `61f50265` (merge-base with `origin/master` confirmed)
- Reviewer: opus (tier-up over the haiku implementer)
- Verdict: **pass**

## Scope check

- There are exactly 8 paths in the diff, and they match brief section 6: `CHANGELOG.md`, `api/internal/handlers/module_upload.go`, `module_upload_test.go`, `api/internal/registry/registry.go`, `registry_dialguard_test.go`, `api/specs.md`, `test/e2e/api_module_upload_budget_e2e_test.go` and `test/e2e/buckets.sh`.
- The diff has no `SECURITY_AUDIT.md` and nothing under `specs/*/audit/held/`. The worktree is clean.
- Every edit (2.1 to 2.15) matches the brief's AFTER blocks. I diffed the E2E file byte for byte against brief 2.13, and it is identical.

## Findings coverage

- **F-100 (no total decompressed budget):** closed.
  - `extractUploadArchive` keeps `remaining` (4 MiB) across members, repeated names included. It caps each read at `min(900 KiB, remaining)+1`, so it never buffers more than the budget plus 1 byte, and it fails at the first member that crosses the budget.
  - Both paths are covered: tar.gz and zip call the same `add`.
  - Handler errors map to 400 through `parseUploadedBundle` and then `httperr.WriteCode(400)`.
- **F-099 (spec vs code on registry dialling):** closed by wiring, per D6 ("Wire"; questions.md line 25 says "D5 and D6 wire netguard").
  - `NewSet` now uses `netguard.HTTPClient(15s, netguard.IsPublic)`.
  - Keyed engines built lazily get the same `s.client` (curseforgeLazy calls `newCurseforge(s.client, …)`).
  - The error chain survives: `sanitizeUpstreamErr` wraps a copy of `*url.Error` with `%w`, `url.Error` unwraps to `net.OpError`, and that unwraps to the Control hook's `ErrBlockedAddr`. So `errors.Is` holds for both `httpGetJSON` and `Curseforge.get` (`%w`).

## Legitimate callers

- **Registry:** production calls go only to fixed public provider hosts, which `IsPublic` allows. No existing test drives a network fetch through `NewSet`:
  - `registry_test.go`, `keys_test.go:161`, `nexus_test.go:179-187` and `steam_test.go:305-313` only call `For`, `Available` or `curseforgeLazy`.
  - The per-engine tests build engines with their own client and override `baseURL` on those engines, not on a Set.
  - The handler tests use a `fakeProvider`.
  - So no existing test changes behaviour.
- **Uploads:** a real bundle is at most 900 KiB compressed, and its kept files must total 900 KiB or less. Only highly compressible padding can exceed 4 MiB extracted, so no legitimate bundle is affected. The existing `TestUploadBundle` and `TestDeleteUpload` fixtures are a few hundred bytes.
- **Behaviour change the maintainer should note:** the registry client now ignores `HTTP(S)_PROXY` and no longer negotiates HTTP/2 (custom Transport). The brief's held notes already cover this. The code and `api/specs.md` state the proxy part accurately.

## Tests: would each fail before the fix?

| Test | Before the fix | After the fix |
|---|---|---|
| `TestExtractUploadArchive_EnforcesTotalExtractedBudget` | 6 × 920,576 B members, each under the cap; extraction returned nil error, so it fails | "total limit" error at member 5. Zip uses `zw.Create` (Deflate), so both fixtures compress to a few KB, well under the 900 KiB guard |
| `TestExtractUploadArchive_RepeatedMemberNamesCountTowardBudget` | 6 × `pad.bin`; `out` has 1 entry and no cap trips, so it fails | Errors on the 5th read |
| `TestUploadBundle_RejectsArchiveOverTotalExtractedBudget` | `parseUploadedBundle` kept 3 files, so it returned 201 and fails | 400, and zero ConfigMaps in `gameplane-system` (the namespace `mountModulesRouter` uses, so the check is not vacuous) |
| `TestSet_RegistryFetchesDialOnlyPublicAddresses` | Loopback httptest returned 200, so err was nil and it fails. Link-local gave a timeout or unreachable error, not `ErrBlockedAddr`, so it fails | Blocked at dial for both |
| `TestSet_KeyedEnginesShareTheGuardedClient` | Pointer equality held, but the loopback GET succeeded, so it fails | Blocked |
| `TestAPI_ModuleUpload_ExtractionStaysWithinBudget` (E2E) | 6 × 800 KiB = 4800 KiB > 4096 KiB, each under the 900 KiB cap; got 201, so it fails | 400, and the ConfigMap is NotFound |

E2E test facts, checked against the tree:
- The helper APIs exist and match their use: `Env.BootstrapAdmin`, `Env.APIClient`, `APIClient.{BaseURL,CSRF,HTTP,Delete,Close}`, `Env.K8s` and `Env.Dyn`.
- `BaseURL` is the API root, as other tests use it.
- ModuleSource is cluster-scoped (`scope: Cluster`), so the Create call without a namespace is correct.
- The CSRF header name matches `api_mods_confinement_e2e_test.go`.
- It calls `t.Parallel()`, uses unique suffixed names, and registers cleanup.

## Buckets and login budget

- `TestAPI_ModuleUpload_ExtractionStaysWithinBudget` is listed in `bucket_api_mods`, and `buckets.sh verify` reports "133 tests, all in exactly one bucket".
- I counted `APIClient(` calls in the five existing api-mods tests: one each, so 5. This test adds 1, making 6. That fits the ~7 ceiling (rule 7), and the buckets.sh comment ("6 of its ~7 ceiling") is accurate.
- H09's planned +1 would bring the bucket to 7. The held notes already record this.

## Specs and docs

- `api/specs.md` Dependencies: the claims are correct.
  - The Steam resolver uses `IsPublic` (`api/internal/steam/resolver.go:45`).
  - Notification sinks use `IsAllowed` (`notify.go:86`, `deliver.go:172`).
- The Outbound safety line matches the code.
- The new "Upload limits" subsection is accurate:
  - The 413 above 900 KiB compressed is correct.
  - The per-member 900 KiB cap, 256 distinct paths and 4 MiB running total match the code.
  - The request gets 400.
  - "The four kept bundle files" matches `bundleFileNames` (4 entries).
- Registry package doc and `httpGetJSON` doc now match the code.

## CHANGELOG

- The `### Security hardening` heading and one bullet sit at the end of `## [Unreleased]`, directly before `## [0.3.0-rc.1]`. It is the only heading of that name in the file.
- Format: `- **api:** hardened module archive extraction limits and mod-registry outbound connections.` This is correct.

## Wording-leak scan (every added line plus the commit message)

- I scanned for F-NNN, Hxx, OD-0xx, "finding", "audit", "SSRF", "bomb", "attack", "exploit", "malicious", "bypass" and "rebind". The only hit is the unrelated `"metadata":` key in a ModuleSource object.
- The test names and comments state controls. `169.254.169.254` appears only as the "link-local" table row, with no account of how it could be used.
- The commit message is neutral.
- `api/specs.md` drops the old "SSRF" wording.

## Compile checks (run by the reviewer)

- `cd api && go build ./... && go vet ./... && go vet -tags envtest ./...`: OK
- `cd test/e2e && go vet -tags e2e ./...`: OK
- `bash test/e2e/buckets.sh verify`: OK (133 tests)
- `gofmt -l` on the touched Go files: clean

## Informational (non-blocking)

1. **Commit trailer:** the commit says `Co-Authored-By: Claude Haiku 4.5`, while brief section 6 says `Claude Opus 5.5 (1M context)`. CLAUDE.md rule 11 asks for `<current-running-model>`, and the implementer ran on haiku, so the trailer is defensible. The commit is unpushed. If the maintainer wants the brief's exact text, amending is allowed, but it is not required.
2. **Carried follow-up:** the non-regular tar entry data that `tar.Reader.Next` skips is still not counted toward the budget. This is pre-existing, and memory stays bounded. The brief's held notes already list it as a later pass, so it is not a defect of this change.
