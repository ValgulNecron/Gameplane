# Git hooks

Shared git hooks for this repo. They are **not** active until you point git at
this directory (git only runs `.git/hooks/` by default):

```sh
git config core.hooksPath .githooks
```

(or copy an individual hook into `.git/hooks/`). Run once per clone.

## `pre-push` — submodule auto-bump

Before you push the superproject, this advances any submodule whose
`.gitmodules` entry declares `branch = <b>` up to that submodule's
`origin/<b>`, records the bump as its **own** commit, and stops the push so the
bump is included — just run your `git push` again.

It is deliberately conservative and will **skip** (never guess) when:

- the advance would not be a **fast-forward** (`origin/<b>` isn't a descendant
  of the currently-pinned commit) — this protects against dropping work when a
  submodule's branch was rebased/reset;
- the submodule worktree has **uncommitted tracked changes**;
- the submodule is sitting on **local work** (HEAD is neither the pinned commit
  nor `origin/<b>`), e.g. you're on a feature branch inside it.

In all those cases it prints why and leaves the pointer alone for you to
reconcile by hand.

### Escape hatches

- `git push --no-verify` — skip all hooks for one push.
- `GAMEPLANE_SUBBUMP_SKIP=1 git push` — skip just this hook.
- `GAMEPLANE_SUBBUMP_DRYRUN=1 git push` — report what it *would* bump, change nothing.
