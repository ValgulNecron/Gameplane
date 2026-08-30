# Open Decisions — values the agent invented, awaiting a maintainer ruling

**Feature**: 008-hardened-github-actions | **Raised**: 2026-08-30

Everything below was invented during `/speckit-plan` and `/speckit-implement` and written
into the plan, the contracts, or the verifier **as though it were a settled decision**. None
of it comes from `spec.md`, `CLAUDE.md`, or the constitution. It is collected here so it can
be ruled on rather than absorbed.

**All nine items were ruled on by the maintainer on 2026-08-30.** Each carries the verbatim
response, followed by what it means for the implementation.

---

## Execution order for the next session

The rulings reshape the remaining work substantially — the verifier is deleted and
`actionlint` replaces it. In dependency order:

| # | Work | Ruling |
|---|---|---|
| 1 | Delete `.github/workflows-verify.sh`, `.github/verify-rules/`, the `.gitignore` Python block, `baseline-violations.txt` | D-F |
| 2 | Strip verifier references from `plan.md`, `research.md`, `tasks.md`, `quickstart.md`, `contracts/*.md` | D-F |
| 3 | Add `timeout-minutes` to every job per the D-A table | D-A |
| 4 | Rewrite `.github/dependabot.yml`: 28 entries, one group per module, limits 5/10/5/5, all Monday 03:00 | D-B, D-C, D-D |
| 5 | Add the workflow-lint gate (`actionlint` + `zizmor`) to `ci.yaml`, wired into `report` (needs + `NEEDS_ORDER` + `JOB_MATCHERS`) | D-H, D-J |
| 6 | Write `.github/workflows/ai-review.yaml` with named, documented constants | D-G |
| 7 | Add the `concurrency` block to `images.yaml` | FR-005 |
| 8 | **Last:** update `docs/contributing.md` and `docs/security.md` to the final CI state | D-I |

Already landed in `01af5953`: all 96 SHA pins and every per-job permissions block (D-E).
Already landed uncommitted: the `redact()` filter in `dump-cluster-state/action.yml`.

**Measurement completed**: D-A's original instruction asked for the actual duration of the release and image build steps. On 2026-08-30, measurements were taken from successful runs only, using job duration (completedAt - startedAt) rather than workflow wall-clock time. The following data was collected:

| Workflow | Job | Median | Max | Samples | Assessment |
|---|---|---|---|---|---|
| `ci.yaml` | `e2e-go` | 3m | 6m | 192 | SAFE |
| `ci.yaml` | `e2e-multicluster` | 3m | 4m | multiple | SAFE |
| `ci.yaml` | `e2e-upgrade` | 2m | 3m | multiple | SAFE |
| `ci.yaml` | `e2e-web-live` | 4m | 5m | multiple | SAFE |
| `ci.yaml` | `e2e-game-bot` | 19m | 20m | multiple | SAFE |
| `images.yaml` | `game-images` | 1m | 1m | 2 | SAFE (thin evidence) |
| `images.yaml` | `common-base` | 1m | 1m | 2 | SAFE (thin evidence) |
| `release.yaml` | `images` | 8m | 26m | 29 | EXCEEDS |
| `publish-edge.yaml` | `images` | 5m | 31m | 142 | EXCEEDS |
| `republish-modules.yaml` | `modules` | 1m | 1m | 2 | SAFE |

Note: `images.yaml` budgets rest on thin evidence (2 successful runs only). Every other job's budget is well-supported.

---

## D-A: Timeout exception values

**Spec says** (FR-004): `timeout-minutes` on every job, "defaulting to <= 30 minutes unless
specifically justified, e.g. heavy game bot suites".

**Invented**: the specific ceilings for four jobs. Only `e2e-game-bot` is traceable to the
spec's own example, and its 50 was already in the tree.

| Job | Value | Source |
|---|---|---|
| `ci.yaml` / `e2e-game-bot` | 50 | pre-existing in the tree; spec names game-bot as the example |
| `ci.yaml` / `e2e-go` | 35 | pre-existing in the tree (`operator` matrix leg) |
| `images.yaml` / `game-images` | 60 | **invented** |
| `release.yaml` / `images` | 45 | **invented** |
| `publish-edge.yaml` / `images` | 45 | **invented** |

**Question**: are 60/45/45 acceptable, or should these jobs be measured first and set from
observed p95 duration? The three invented values were guesses at what a multi-arch buildx
plus cosign run costs; nobody timed them.

**RESPONSE**: if the whole E2E need to run on 35 - 50 minute do you really thing the image build should be 45-60minute? no it should not. here the two thing that will be made all E2E get a new timeout of 1h (60minute) game-images get 10minute PER image they should run in parralel but n image should take more than 10minute. for the release budget will be at 15minute and same for publish edge. (if the CI timeout increase in 5minute increment) also check how long these specific step usually take to conclude if it not enought and propose new value.

**Status**: **RESOLVED** 2026-08-30. The maintainer's reasoning governs: an image build has
no business taking longer than the entire E2E suite it feeds. Values below are the ruling,
not a measurement — `gh run list` was unavailable when this was recorded (the session's
Bash tool was blocked), so the "check how long these steps actually take" half of the
instruction is **outstanding**. Re-measure when `gh` is reachable and propose adjustments
if any job runs close to its budget.

All timeouts are multiples of 5 minutes, per the ruling.

**Maintainer's ruling on measurement**: "I did say that IF the real time take longer you can move IF REAL DATA back it up always 5min increment so 30 and 35 in this case." The original reasoning—that an image build should not outlast the E2E suite it feeds—rested on an assumption the data contradicts: E2E jobs actually run 2–20 minutes, not the assumed 35–50, while the image jobs run up to 31 minutes. This discrepancy is why two budgets moved: `release.yaml` `images` from 15 to 30, and `publish-edge.yaml` `images` from 15 to 35.

| Workflow | Job | Timeout | Basis |
|---|---|---|---|
| `ci.yaml` | `e2e-go` (all matrix legs) | **60** | "all E2E get a new timeout of 1h" |
| `ci.yaml` | `e2e-multicluster` | **60** | same |
| `ci.yaml` | `e2e-upgrade` | **60** | same |
| `ci.yaml` | `e2e-web-live` | **60** | same |
| `ci.yaml` | `e2e-game-bot` | **60** | same — lowered from the pre-existing 50 |
| `images.yaml` | `game-images` | **10** | "10 minutes PER image"; the job is a matrix, one leg per image, legs run in parallel |
| `images.yaml` | `common-base` | **10** | ⚠️ **EXTENSION, not ruled on.** Same shape as `game-images` — a per-image matrix — so the same per-image budget is applied. Veto here if the steamcmd base build legitimately needs longer. |
| `release.yaml` | `images` | **30** | measured median 8m, max 26m across 29 samples; 30 provides buffer above observed max |
| `release.yaml` | `chart`, `github-release`, `modules` | **15** | ⚠️ **EXTENSION.** The ruling named a release budget of 15 without splitting it per job; applied uniformly across the release workflow's jobs. |
| `publish-edge.yaml` | `images` | **35** | measured median 5m, max 31m across 142 samples; 35 provides buffer above observed max |
| `republish-modules.yaml` | `modules` | **15** | ⚠️ **EXTENSION.** Not named in the ruling; it is a module push like `release.yaml`'s `modules`, so it inherits the same 15. |

Note this raises the E2E jobs above the ≤ 30 default in FR-004. That is the maintainer's
call and supersedes the FR's default; FR-004's "unless specifically justified" clause covers
it, and the justification is recorded here.

**Over-30 exception set**: `release.yaml` `images` at 30 minutes sits exactly at FR-004's ceiling and is therefore NOT an over-30 exception. `publish-edge.yaml` `images` at 35 minutes IS an exception (above the ceiling) and requires an inline justification comment in the workflow. The complete exception set is now six jobs: the five E2E jobs at 60, plus `publish-edge.yaml` `images` at 35.

Every remaining non-E2E job keeps whatever it has today; nothing else changes.

---

## D-B: Dependabot pull-request limits

**Spec says** (FR-021): configure "open pull request limits". The Edge Cases section says
"strict open PR limits (e.g., max 5–10)".

**Invented**: `gomod: 3`, `npm: 10`, `docker: 5`, `github-actions: 5`.

The `gomod: 3` is **below the spec's own stated floor of 5**. It was chosen to keep the
worst case (14 × N) bounded, which is a real concern, but it contradicts the spec rather
than implementing it.

**Question**: 3 per Go module, or 5 as the spec implies? At 5 the worst case is 70 open PRs;
at 3 it is 42. Grouping should collapse both to ~14 in practice, so the ceiling only matters
in a bad week.

**RESPONSE**:  respect the 5-10

**Status**: **RESOLVED** 2026-08-30. Every `open-pull-requests-limit` sits inside the spec's
own 5–10 band. The invented `gomod: 3` is discarded.

| Ecosystem | Limit |
|---|---|
| `gomod` (×14) | **5** |
| `npm` | **10** |
| `docker` (×12) | **5** |
| `github-actions` | **5** |

Worst case is 14 × 5 = 70 open Go PRs; grouping (D-C) collapses the normal week to ~14.

---

## D-C: Dependabot grouping scheme

**Spec says** (FR-021): "dependency groups to batch minor and patch version bumps together".

**Invented**: the group names (`<module>-minor-patch`, `k8s`, `react`, `types`), the
k8s-libraries carve-out, the npm `react`/`types` split, and the declaration-order rule.

The k8s carve-out has a real technical basis — `k8s.io/*` and `sigs.k8s.io/*` are
version-locked and a PR bumping one alone does not compile — but the spec does not ask for
it and nobody confirmed it.

**Question**: keep the per-module k8s group, or accept simpler one-group-per-module and let
the occasional broken PR be closed by hand?

**RESPONSE**: per module group

*(Clarified on follow-up: "one group per module.")*

**Status**: **RESOLVED** 2026-08-30. **One group per module, no k8s carve-out.** Each `gomod`
entry declares exactly one group batching `update-types: ["minor", "patch"]` across all its
dependencies. Major bumps still arrive as individual PRs.

Consequence, accepted knowingly: `k8s.io/*` and `sigs.k8s.io/*` are version-locked to each
other, so a grouped PR that moves only some of them may not compile. Those get closed by
hand. The simpler config was preferred over pre-empting that case.

Same shape for the other ecosystems: one group per entry, batching minor and patch.
---

## D-D: Dependabot schedule stagger

**Spec says** (FR-021): "update schedules (e.g., weekly on Mondays)".

**Invented**: 03:00 UTC for gomod/docker/actions, **04:00 for npm** to "stagger the webhook
burst".

**Question**: is the stagger wanted, or should everything run at one time?

**RESPONSE**: at the same time

**Status**: **RESOLVED** 2026-08-30. No stagger. Every entry across all four ecosystems runs
`weekly`, `monday`, `03:00` UTC. The invented 04:00 npm offset is discarded.

---

## D-E: Top-level `packages: write`

**Spec says** (FR-001): every workflow's top-level permissions set to "`contents: read` or
`{}` (least privilege)".

**Invented**: R2 originally permitted `packages` at the top level alongside `contents`,
because all jobs in the four publish workflows push to ghcr.io and per-job duplication
seemed noisy. **This directly contradicts FR-001**, which allows only two values.

**Question**: hold the strict FR-001 line (top level is `contents: read` or `{}`, and
`packages: write` is declared per job), or amend FR-001 to permit it?

**RESPONSE**: per job packages write ALwAYS do lowest priv

**Status**: **RESOLVED** 2026-08-30, and **already implemented** in commit `01af5953`.
FR-001 holds strictly: top level is `contents: read` or `{}`, never more. `packages: write`
is declared only on the jobs that actually push to ghcr.io, in all four publish workflows.
The invented top-level carve-out is gone.

Standing principle recorded for future work: **always lowest privilege.** When a scope is
needed by one job, it goes on that job — never at the top level "for convenience".

---

## D-F: The verifier itself

**Spec says**: nothing. `spec.md` describes target state (FR-001…FR-025) and success
criteria (SC-001…SC-006). It never asks for an enforcement gate.

**Invented**: `.github/workflows-verify.sh` plus nine rule modules — roughly 1,000 lines,
now the largest artifact in the feature. It was then cited in the plan's Constitution Check
as the thing satisfying Principle I, so an unrequested invention became the justification
for passing a gate.

**Argument for keeping it**: SC-001 through SC-004 are "100%" claims. A 100% claim checked
only by code review decays. R7 in particular is what makes adding a 15th Go module fail CI
until Dependabot is updated — the failure mode that left this repo's Go modules unmonitored
for its whole life.

**Argument against**: it is a substantial new subsystem nobody asked for, with its own
maintenance cost and its own bugs, added during a feature that was scoped as configuration
hardening.

**Question**: keep it, cut it to a smaller subset (R1 and R7 carry most of the value), or
drop it entirely and rely on review?

**RESPONSE**: never asked for it so removed

**Status**: **RESOLVED — DELETE.** 2026-08-30. Not requested, so it goes. Enforcement moves
to `actionlint` per D-H, which covers the pin/permission/injection ground better than
hand-written rules and is maintained by someone else.

Delete, in one commit:

- `.github/workflows-verify.sh`
- `.github/verify-rules/` (all 10 modules: `_common.py`, `r1`, `r2`, `r4`, `r5`, `r6`, `r7`,
  `r8`, `r9`, `r10`)
- the `__pycache__/` + `*.py[cod]` block appended to `.gitignore` — it existed only for
  these modules
- `specs/008-hardened-github-actions/baseline-violations.txt` — the verifier's own output

Then strip every reference to it from the spec artifacts: `plan.md` (Summary, Technical
Context, Project Structure, and the Principle I row of the Constitution Check),
`research.md` (D-10 and the D-04 rejection of actionlint, now reversed), `tasks.md`
(T001–T002, T004, and every rule task: T005–T009, T018–T020, T022, T027, T031–T032),
`quickstart.md` (scenarios 1–4 and 6 are written around `workflows-verify.sh`), and the
"Enforced by" headers in all four `contracts/*.md`.

**What is lost, stated plainly** so it is a known gap rather than a silent one:

- **R7's parity check.** Nothing will now fail CI when a 15th Go module or 13th Dockerfile
  is added without a matching Dependabot entry. That is precisely the failure that left all
  14 Go modules unmonitored for the life of this repo, and `actionlint` does not cover it.
  If a cheap replacement is wanted later, this one check is ~40 lines and is the single
  highest-value piece of what is being deleted.
- **R10's redaction check.** Nothing will verify that `dump-cluster-state` keeps piping
  through `redact`. The filter itself stays — only the check that it is still wired up goes.

Also resolved by this deletion: the R10 line-continuation and subcommand-indirection
bypasses found by adversarial review are moot, along with the `.gitignore` question in
"Already corrected" below.

---

## D-G: AI review magic numbers

**Spec says** (FR-024, FR-025): a sticky comment, and sanitised input.

**Invented**: 200 KB diff cap, 200-char title cap, 4000-char body cap, and the
`<!-- gameplane-ai-review -->` marker string.

**RESPONSE**: why cap on these? magic number should not exist. they should be documented named and explained

**Status**: **RESOLVED** 2026-08-30. The objection is to *unexplained* numbers, not to
limits as such — a diff cap is genuinely needed, since an unbounded diff would blow the
model's context and cost. So: no bare literals anywhere in `ai-review.yaml`. Every limit
becomes a named `env:` constant carrying a comment that states **why it has that value**,
and each is justified below rather than asserted.

| Constant | Value | Why this number |
|---|---|---|
| `MAX_DIFF_BYTES` | 200000 | ~200 KB ≈ 50k tokens, roughly a quarter of the review context, leaving room for the constitution, CLAUDE.md and the touched `specs.md`. Larger diffs are truncated with an explicit marker so the reviewer knows it is seeing a partial change. |
| `MAX_TITLE_CHARS` | 200 | GitHub's own PR title limit is 256; 200 keeps a margin and a title is a headline, not prose. |
| `MAX_BODY_CHARS` | 4000 | Enough for a real PR description with a checklist; bounds how much attacker-controlled text enters the prompt at all. |
| `STICKY_MARKER` | `<!-- gameplane-ai-review -->` | An HTML comment is invisible in rendered Markdown, so it identifies the bot's comment for in-place updates without showing up in the thread. |

If a value later proves wrong, the fix is to change the constant and its comment together —
never to leave the comment describing an old value.

---

## D-H: Scope exclusions

**Invented**: `plan.md`'s "Out of Scope" section rules out reusable-workflow refactoring,
splitting `ci.yaml`, adopting `actionlint`/`zizmor`, bucket restructuring, and branch
protection rules. These are maintainer calls that were made unilaterally and written as
settled.

`actionlint` in particular deserves a real answer: it covers R1, R4 and R6 better than the
hand-written rules do, and adopting it would shrink D-F considerably. It was rejected in
research.md D-04/D-10 on the grounds of "adding a third-party binary to a supply-chain
hardening feature" — a defensible argument, but not one anybody asked for.

**RESPONSE**: adding new third party is not an issue if it would really improve CI you have my go ahead. 

**Status**: **RESOLVED** 2026-08-30. **Adopt `actionlint`.** It replaces the deleted
verifier (D-F) and does the job better: it is purpose-built, externally maintained, and
already knows GitHub's workflow schema, expression syntax, and `shellcheck`-level analysis
of `run:` bodies.

Add a `workflow-lint` job to `ci.yaml`, gated on a new `github` path filter, `contents: read`,
running `actionlint` over `.github/workflows/`. Pin the action or the binary by SHA like
every other dependency, and add it to the `report` job's `needs`, `NEEDS_ORDER` and
`JOB_MATCHERS` (all three, or it is silently missing from the PR comment).

What it covers that the deleted rules did: unpinned/malformed action refs, expression-injection
into `run:` bodies (the old R6), schema and type errors, shellcheck findings, deprecated
syntax. What it does **not** cover, and stays uncovered per D-F: Dependabot↔tree parity,
`dump-cluster-state` redaction wiring, and repo-specific secret confinement.

`zizmor` (a workflow security auditor covering permissions and `pull_request_target` misuse)
would close part of that remaining gap. Not adopted here — flagged as a candidate once
`actionlint` is settled, so this feature does not grow a second new dependency mid-flight.

---

## D-I: Documentation tasks

**Invented**: T041 (`docs/contributing.md`) and T042 (`docs/security.md`) — tasks to edit
docs nobody asked to have edited.

**RESPONSE**: revert and once the whole thing is done update the docs to reflect the LAST state of the new ci

**Status**: **RESOLVED** 2026-08-30. Drop T041/T042 as written — they were speculative and,
worse, they described the verifier that D-F now deletes. Replace them with a single task
that runs **last**, after every other change has landed, documenting the CI as it finally
is rather than as it was planned:

> **T041 (revised)** — after all other tasks are complete, update `docs/contributing.md` and
> `docs/security.md` to describe the final CI: the SHA-pinning policy and how Dependabot
> maintains the pins, the lowest-privilege per-job permission model, the `actionlint` gate,
> the timeout budgets from D-A, secret confinement, and the AI reviewer's trust split.
> Written against the merged state — no forward references to work not yet done.

Sequencing matters here: documentation written mid-flight describes a CI that never shipped.

---

## Already corrected

- **Fabricated constitution citation.** `plan.md` claimed the Governance section "defers to
  explicit human direction" to justify skipping Principle V. No such clause exists. The
  citation is removed and the violation is now recorded as unjustified.
- **`.gitignore`.** Two Python patterns were appended for `.github/verify-rules/`, outside
  the scope the plan itself declared. Kept only if D-F is resolved as "keep".

---

## D-J: actionlint does not enforce SHA pinning

**Spec says** (cited in D-H's coverage list): `actionlint` covers "unpinned/malformed action refs" as part of replacing the deleted verifier's R1.

**FINDING**: Actionlint's own documentation (`docs/checks.md`) states its "Action format in uses:" check validates only the syntactic shape `owner/repo@ref`. It accepts `actions/checkout@v4` (a floating ref) and does not reject it. Actionlint has no SHA-pinning rule. Therefore actionlint cannot enforce SC-001 ("100% of actions SHA-pinned") on its own — the deleted verifier's R1 was the actual pin check, and D-H's replacement claim is false. With R1 deleted and actionlint unable to replace it, SC-001 would have no automated enforcement at all.

**RESPONSE**: Adopt `zizmor` alongside `actionlint`.

**Status**: **RESOLVED** 2026-08-30. Zizmor is a workflow security auditor that covers unpinned `uses:` refs (closing the gap left by deleting R1), plus `permissions` misuse and `pull_request_target` misuse—three control gaps with one dependency. D-H itself identified zizmor as a candidate; this ruling authorizes it.

**Enforcement reality after this ruling:**

*Actionlint enforces (in the `workflow-lint` gate):*
- Schema and type errors (including invalid event names, malformed keys)
- Expression-injection into `run:` bodies (the old R6)
- Shellcheck findings in `run:` bodies
- Deprecated syntax
- **MALFORMED** `timeout-minutes`/`permissions` keys (schema only — never presence, never value)

*Zizmor enforces (in the `workflow-lint` gate):*
- **Unpinned `uses:` refs** (the old R1) — this is what makes SC-001 mechanically true
- Permissions misuse
- `pull_request_target` misuse

*Nothing automatically enforces (code review only, each must be stated as a gap):*
- Dependabot ↔ tree parity (the old R7)
- `dump-cluster-state` redaction wiring (the old R10)
- Secret confinement
- `timeout-minutes` **PRESENCE** and **VALUES** (D-A sets the values; nothing checks them)
- `concurrency` **PRESENCE**
- The "# vX.Y.Z" comment convention beside each SHA pin

D-J **corrects** the "unpinned/malformed action refs" clause in D-H's coverage list. D-H's own text remains as the historical record; D-J supersedes that one claim.

**Implementation detail, not decided by this ruling:** Job layout (whether zizmor runs inside the same `workflow-lint` job as actionlint or as its own job) and zizmor's ruleset/severity are deferred to execution-order step 5 in the next session.

---

## D-K: Zizmor scoping and gating

**Finding:**

(i) Zizmor's first real run found a broken pin: `claude-code-action` was pinned to an
annotated tag's TAG OBJECT (`50b26a71...`) rather than its commit (`a874e9ec...`), because
`research.md` D-01's documented method used `git ls-remote --tags --refs` and `--refs` strips
the peeled `^{}` lines that carry the commit SHA for annotated tags. One pin affected;
the method has been corrected; every other pinned action uses a lightweight tag, where
`refs/tags/X` IS the commit, so those were unaffected. The audit covered the 20 distinct
pins present in `.github/` — note that `zizmorcore/zizmor-action` is pinned in `ci.yaml`
but was never added to `contracts/action-pins.md`'s registry table, so that table lists 19.
It resolves correctly; the gap is documentation, not a bad pin.

(ii) The zizmor step exited 0 and the job went green, because `zizmor-action` defaults to
`advanced-security: true` — uploading SARIF to code scanning instead of failing the job.
D-J's "fail on any finding" was therefore not implemented: the zizmor step did not gate.

**Response:** Scope zizmor to `.github/` only (the 171 findings in `test/e2e/testdata/lint-gate/*`
are deliberately-broken fixture workflows whose purpose is to prove the lint gate can fail;
they cannot be fixed without destroying the tests). Make the job fail. Gate now on
`unpinned-uses` (the rule D-J adopted zizmor for — it is what makes SC-001's "100% SHA-pinned"
mechanically true) and `ref-version-mismatch` (which just caught the pin in finding (i) and
would have caught it earlier). Defer the remaining real findings as a recorded backlog.

**Status**: **RESOLVED** 2026-08-30. Zizmor is scoped to `.github/` only and configured with
`advanced-security: false`, so its own exit code fails the job rather than being absorbed by
a SARIF upload.

Stated precisely, because the shorthand is misleading: `.github/zizmor.yml` DISABLES the four
backlogged rules below. Every other zizmor rule remains enabled and gates the build — that
includes the two the tool was adopted for, `unpinned-uses` and `ref-version-mismatch`, and
also the ~35 rules that produced zero findings on 2026-08-30. Leaving those on costs nothing
today and catches regressions for free, so the gate is broader than "two rules".

The remaining zizmor findings—those outside the two gated rules—are deferred as backlog, not
silently accepted. This inventory is as of 2026-08-30:

| Rule | Count | Deferred to |
|---|---|---|
| `artipacked` | 58 | follow-up feature |
| `dependabot-cooldown` | 56 | follow-up feature |
| `template-injection` | 33 | follow-up feature |
| `dangerous-triggers` | 2 | see note below |

**Note on `dangerous-triggers`:** This rule fires on `workflow_run`, which in this design IS
the security control — the `workflow_run` job runs the base branch's definition, which stops
a PR from granting itself the API key. This is the rule being wrong for a justified case
(CLAUDE.md rule 4), not a defect to fix. It is recorded here as accepted-but-misgated rather
than as a bug to resolve.

D-K implements D-J's "fail on any finding" with an explicitly bounded and recorded starting
rule set. D-K **supersedes nothing** in D-J; it completes D-J's implementation detail.
