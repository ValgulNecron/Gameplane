# Quickstart: validating a feature 014 slice

This is the validation path, not the implementation. Tests run on GitHub Actions (rule 8); the maintainer's cluster may be used for a manual walk after CI is green.

## Prerequisites

- The slice branch pushed (`014a-…` … `014g-…`), PR open with `type: refactor` (+ `type: feature` for slice 5) and `area: web`.
- Pencil GUI has `/home/valgul/project/Gameplane/design.pen` open and saved after the slice's design wave.
- `gh` authenticated.

## 1. Design side

```sh
# the touched ids are listed in the slice row of plan.md
git show --stat HEAD~N -- design.pen design-export/   # design commit precedes code commits
python3 -c 'import json,sys; [json.load(open(f)) for f in sys.argv[1:]]' design-export/json/<id>.json …
grep -l '"\.\.\."' design-export/json/<id>.json …      # must print nothing
grep -n '<id>' design-export/MANIFEST.md                # one row per touched id
```

Expected: every touched id has a fresh JSON without elision markers and a PNG with real dimensions; MANIFEST rows updated; no `$c:` variable remains in a redrawn screen's JSON (`grep -c '"\$c:' design-export/json/<id>.json` prints 0).

## 2. Compile check (allowed locally)

```sh
cd web && npx tsc --noEmit
```

## 3. CI

```sh
gh pr checks <n> --watch
gh run view <run-id> --log-failed
```

Expected green: `lint`, `web`, `web-e2e-mock`, `e2e-web-live` (both arches), plus the unchanged Go jobs. Download the Playwright artifact and compare screenshots per `contracts/screen-verification.md`.

## 4. Review gates (recorded in the PR description)

- Screen verdict table: one row per screen id in the slice, verdict per the contract.
- Family check: `grep -rl "@/components/ui/\|@radix-ui" web/src --include=*.tsx | grep -v test` lists no file that the slice rebuilt; for slice 5 it lists nothing at all.
- Test counts: `git diff master --stat -- 'web/src/**/*.test.ts*'` shows no deleted file; `grep -c "it(" <file>` per touched test file not lower than on `master`.
- Coverage summary from the `web` job at or above 92/76/82/92.
- `web/specs.md` diff describes the slice's component change.
- Pencil node ids listed in the PR body (rule 8).

## 5. Manual walk on the maintainer's cluster (optional, after green)

Slice 1: sign in (local and SSO-only variant), toggle appearance, open every sidebar entry as admin and as viewer, open the mobile drawer at 390 px.
Slice 2a/2b: servers list filters, each tab, one settings save and discard, one confirm dialog.
Slice 3: full wizard, one module install and upload, one schedule and one restore dialog.
Slice 4: invite, role edit, OIDC override add/remove with the admin confirm, audit verify, logs download.
Slice 5: create a share link, open it signed out in each state, revoke, reopen.

## 6. Close-out per slice

Maintainer approves and merges; delete the branch remote and local (rule 12); cut the next slice's branch from `master`. After slice 5: update `.coderabbit.yaml`/docs mentions of the old primitives if any, then `/speckit-tasks` close-out marks every task `[X]` and the folder is renamed `done_014-heroui-web-rebuild` once merged (rule 16).
