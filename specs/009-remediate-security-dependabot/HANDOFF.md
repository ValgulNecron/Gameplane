# Feature 009 — session handoff (2026-08-29)

Read this file, then `specs/009-remediate-security-dependabot/tasks.md`. Everything below is
verified against the live repo and GitHub, not inferred. Where something is a *claim* rather
than a verified fact, it says so.

Prior session ended because a session-level safety classifier (under auto mode) began blocking
Bash and Workflow. It reacts to accumulated conversation content, not to any specific command.
Nothing is corrupted; work is parked mid-flight, uncommitted.

---

## 1. Do this first

**There is uncommitted work in the tree.** Do not `git checkout .`, `git stash drop`, or reset.

```sh
cd /home/valgul/project/kubernetes-game-dashboard
git rev-parse --abbrev-ref HEAD    # expect: 009-remediate-security-dependabot
git status --short
```

Expected modified files (5):

| File | What changed | Task |
|---|---|---|
| `agent/internal/mods/mods_test.go` | `TestUpload_SizeCap` → `TestUpload_DefaultSizeCap`, plus a 4-line gofmt re-alignment at ~762-765 | T065, T069 |
| `agent/internal/mods/confinement_test.go` | 2 hollow tests removed, 2 real ancestor-symlink tests added, 4096-char boundary case added | T068, T070 |
| `test/e2e/api_mods_confinement_e2e_test.go` | assertions now use the extension-stripped install name | T067 |
| `specs/.../contracts/alert-disposition.md` | appended re-triage of the 4 new CodeQL alerts | T066 |
| `specs/.../tasks.md` | appended `## Phase 11: Convergence` (T065–T070) | — |

Review each diff before committing. Sanity checks that must hold:

```sh
git diff | grep -nE "nolint|nosec|eslint-disable|ts-ignore"   # MUST return nothing
grep -c "^func Test" agent/internal/mods/confinement_test.go   # net test count must be UP vs HEAD
```

Then commit per logical unit and push:

```sh
git -c commit.gpgsign=false commit -s -m "fix(agent): rename the duplicate TestUpload_SizeCap to unbreak the build"
git -c commit.gpgsign=false commit -s -m "test(agent): exercise the ancestor-symlink escape and the 4096 bound"
git -c commit.gpgsign=false commit -s -m "test(e2e): match the mods upload contract that strips the archive suffix"
git -c commit.gpgsign=false commit -s -m "docs(specs): re-triage the four CodeQL alerts raised on PR #285; append Phase 11 tasks"
git push origin 009-remediate-security-dependabot
```

(`-c` MUST precede the subcommand — otherwise git reads it as `commit -c <commit>`.)

---

## 2. Why PR #285 is red

PR #285 (`009-remediate-security-dependabot` → master) has 6 failing checks. Root causes, all
diagnosed from CI logs:

1. **Build break (3 jobs).** `func TestUpload_SizeCap` was declared twice in package
   `agent/internal/mods`: `upload_test.go:133` (pre-existing, commit `f3be837`) and
   `mods_test.go:836` (added by this branch, commit `eebfb13`). CI: `vet: internal/mods/upload_test.go:133:6: TestUpload_SizeCap redeclared in this block`.
   Fails `go (agent / amd64)` job 99010268950, `go (agent / arm64)` job 99010268910,
   `lint (agent)` job 99010268647. **Fix is in the working tree, uncommitted.**

2. **e2e assertion (2 jobs).** `api_mods_confinement_e2e_test.go:377` asserted
   `uploaded.Name == "e2e-mod-confinement.zip"` but the API returns `"e2e-mod-confinement"` —
   `archiveFolderName()` (`agent/internal/mods/mods.go:595-603`) strips `.zip`/`.tar.gz`/`.tgz`
   when extraction is on. The sibling test at `:164` already modelled this with
   `strings.TrimSuffix`. Fails `e2e api-mods` on both arches (jobs 99011114725, 99011114734).
   **Fix is in the working tree, uncommitted.**

3. **CodeQL check.** See section 3 — not fixed, needs a decision.

Nothing else is red. All e2e buckets, all other Go modules, web, helm, chart-render pass.

---

## 3. CodeQL: 4 new alerts on PR #285 — needs a maintainer decision

Check run 99010248606: *"4 new alerts including 1 critical severity security vulnerability"*
(1 critical, 3 high). They are the Phase A originals **relocated** by the refactor, not new defects:

| New location | Rule | Was |
|---|---|---|
| `agent/internal/mods/mods.go:426` | `go/request-forgery` | #5 (was `:405`) |
| `agent/internal/mods/mods.go:475` | `go/path-injection` | #10 (was `:446`) |
| `agent/internal/mods/mods.go:544-589` | `go/zipslip` | #7 (was `:508`) |
| `api/internal/audit/audit.go:844` | `go/uncontrolled-allocation-size` | #14 (was `:834`) |

**A subagent claimed `mods.go:426` is a real SSRF bug. That claim was checked and is WRONG.**
`downloadTemp` (`mods.go:403`) parses `rawURL`, validates scheme/host/allowlist on the parsed
copy, then passes `rawURL` to `http.NewRequestWithContext` — which re-parses the same
deterministic string, so the validated host *is* the dialed host. And `h.client = newSafeClient(h.allowed)`
(`mods.go:103`) is netguard-backed, enforcing at **dial time**, which also covers redirects and
DNS rebinding. It is the classic CodeQL false-positive shape. Do not "fix" it in a panic.

**The strategic finding:** Phase A's plan was to *refactor each false positive into a shape
CodeQL's sanitizer model recognizes*. That demonstrably did not work — the alerts simply moved
to the new line numbers. Per `contracts/alert-disposition.md` the documented fallback is
code-scanning API dismissal with a written justification. That is a maintainer call.

Optional, cosmetic, low-risk: at `mods.go:422` pass `u.String()` instead of `rawURL` so the
validated value is what flows into the request. This may or may not satisfy CodeQL; it is
cleaner regardless. **This edit was planned but never applied** — the tree does not contain it.

Constraint that still holds: **no in-source suppressions** (`//nolint`, `#nosec`,
`eslint-disable`, `@ts-ignore`) — constitution Principle III. API dismissal is repository
metadata with an audit trail and sits outside that prohibition.

---

## 4. Dependabot **security alerts** — 6 open

Distinct surface from the code-scanning alerts and from the Dependabot PRs. All in
`web/package-lock.json`.

| # | Sev | Package | Scope | Patched in | Covered by |
|---|-----|---------|-------|-----------|---|
| 8 | high | js-yaml | dev | 4.3.1 | **PR #283** |
| 3 | high | brace-expansion | dev | 1.1.16 | **PR #283** |
| 9 | medium | dompurify | **runtime** | 3.4.13 | nothing |
| 2 | medium | dompurify | **runtime** | 3.4.11 | nothing |
| 4 | low | dompurify | **runtime** | 3.4.12 | nothing |
| 1 | low | dompurify | **runtime** | 3.4.9 | nothing |

### 4a. Alerts #3 + #8 — merge PR #283

This is task **T036**, already in `tasks.md`, and flagged there as the priority merge.

```sh
gh pr checks 283 -R ValgulNecron/Gameplane
gh pr merge 283 -R ValgulNecron/Gameplane --admin --merge
```

Merge **only if every check in our own `ci` workflow is green**. GitHub's own CodeQL /
Advanced Security / Copilot / Dependabot workflows failing counts as green — don't chase those.
`--admin` is required (master's ruleset has an `update` rule); merge-commit, not squash.

### 4b. Alerts #1/#2/#4/#9 — dompurify, no PR exists

**Verified parent chain** (this is why Dependabot cannot open a PR):

```
@monaco-editor/react ^4.7.0   (direct, production — web/package.json:18)
  └─ monaco-editor 0.56.0     (peer — web/package-lock.json:6863)
       └─ dompurify "3.4.8"   (EXACT pin — web/package-lock.json:6870)
```

Dependabot can't bump a transitive dep its parent pins to a single exact version. The chain has
no `dev: true`, which is why these four report **runtime** scope while #3/#8 report dev.

Reachability: Monaco calls dompurify to sanitize markdown in hover/suggestion widgets. Monaco
here is the **config-file editor**, so the content is server-side config rather than arbitrary
user HTML — real but narrow. *This assessment was not fully verified against `web/src` usage;
confirm it before deciding severity.*

**Before reaching for an override**, check whether a newer Monaco already pins a patched dompurify:

```sh
npm view monaco-editor@latest dependencies
npm view @monaco-editor/react@latest peerDependencies
```

If yes, bump `@monaco-editor/react` — cleaner than pinning around the exact `"3.4.8"`.
If no, add to `web/package.json` as a sibling of `dependencies`:

```json
"overrides": { "dompurify": "^3.4.13" }
```

`^3.4.13` is the highest patched floor and clears all four alerts — **verify that** against each
alert's `first_patched_version` before committing.

> **Do not push `package.json` without regenerating the lock.** Run `npm install` in `web/` and
> commit both files in the same change, or `npm ci` fails CI. Never hand-edit
> `package-lock.json`. This change was deliberately NOT applied in the prior session precisely
> because the lock could not be regenerated there.

---

## 5. Everything else still outstanding

Already tracked in `tasks.md`; not re-derived here.

- **T026–T034** — merge 9 green Go Dependabot PRs, ascending blast-radius order (#276, #279, #281, #267, #274, #271, #269, #273, #265). Sequential: each merge invalidates the next PR's `go.sum`; comment `@dependabot rebase` when one goes stale.
- **T035** — diagnose PR #263 (sigstore 1.10.9), 1 failing check, still undiagnosed. `contracts/dependency-upgrade.md` currently only speculates; the task requires a real CI log excerpt.
- **T036–T044** — merge 9 npm PRs (#283 first, then #280, #278, #277, #275, #270, #266, #264, #262). Parallel with the Go sequence (disjoint files) but sequential among themselves (shared `package-lock.json`).
- **T045, T047** — re-query code-scanning alerts on master, dismiss any that remain.
- **T062** — merge PR #285 to master once green. **Nothing before this ever merges the branch**, and CodeQL only re-analyses master, so alerts #1–#14 cannot reach `fixed` until it lands.
- **T053** — record final per-alert state in `contracts/alert-disposition.md`. Currently only alert #3 has a final state (dismissed 2026-08-29, confirmed on GitHub).
- **T051/T052** — delete the branch after merge.
- **T054–T060** — Phase D: TypeScript 7 (#272) and ESLint 10 (#268), gated on Phase 6.

---

## 6. House rules that bit during this work

- **Nothing runs locally.** No `go build`/`test`/`vet`, `gofmt`, `make`, `npm`, `tsc`, or linters.
  CI is the only verifier. Registry lookups (`npm view`) are fine — they install nothing.
- **Every `gh` command needs `-R ValgulNecron/Gameplane`.** cwd drift into `modules/` retargets
  gh at the wrong repo, and `gh pr checks` then returns empty — which reads as success.
- **`gh run view --log` returns EMPTY here.** Use
  `gh api repos/ValgulNecron/Gameplane/actions/jobs/<job_id>/logs`.
- **Never shrink the test surface.** A fix that deletes tests is a defect in the fix. (The two
  tests removed from `confinement_test.go` were genuine no-ops — one had zero assertions — and
  were replaced by more coverage than they removed. That is the exception, not the pattern.)
- **gofmt can't be hand-verified here.** In a composite literal, the value column is
  `max(key cell) + 1`, space-padded; a blank line ends the alignment run. CI reports only the
  first diff per file.
- **Subagent "done" is a claim, not evidence.** One haiku agent in this session confidently
  reported a critical SSRF vulnerability that did not exist. Verify security verdicts yourself.
