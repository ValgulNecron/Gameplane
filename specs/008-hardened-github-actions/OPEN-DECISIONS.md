# Open Decisions — values the agent invented, awaiting a maintainer ruling

**Feature**: 008-hardened-github-actions | **Raised**: 2026-08-30

Everything below was invented during `/speckit-plan` and `/speckit-implement` and written
into the plan, the contracts, or the verifier **as though it were a settled decision**. None
of it comes from `spec.md`, `CLAUDE.md`, or the constitution. It is collected here so it can
be ruled on rather than absorbed.

Until an item is marked RESOLVED, the verifier does **not** enforce it. Rules enforce only
what `spec.md` actually states; invented values are recorded as defaults with a marker, not
as gates.

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

**Status**: OPEN. R4 enforces the ≤ 30 ceiling (spec-stated) and requires a declared
timeout (spec-stated). It does **not** fail a job for exceeding an invented allowance.

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

**Status**: OPEN. R7 requires that a limit be declared (spec-stated). It does **not**
enforce a particular value.

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

**Status**: OPEN. R7 requires ≥ 1 group per entry (spec-stated). It does **not** enforce
group names or shapes.

---

## D-D: Dependabot schedule stagger

**Spec says** (FR-021): "update schedules (e.g., weekly on Mondays)".

**Invented**: 03:00 UTC for gomod/docker/actions, **04:00 for npm** to "stagger the webhook
burst".

**Question**: is the stagger wanted, or should everything run at one time?

**Status**: OPEN, low stakes. Not enforced.

---

## D-E: Top-level `packages: write`

**Spec says** (FR-001): every workflow's top-level permissions set to "`contents: read` or
`{}` (least privilege)".

**Invented**: R2 originally permitted `packages` at the top level alongside `contents`,
because all jobs in the four publish workflows push to ghcr.io and per-job duplication
seemed noisy. **This directly contradicts FR-001**, which allows only two values.

**Question**: hold the strict FR-001 line (top level is `contents: read` or `{}`, and
`packages: write` is declared per job), or amend FR-001 to permit it?

**Status**: OPEN. R2 currently reports top-level `packages` as an **advisory note**, not a
failure, pending the ruling. Strict FR-001 enforcement is one line away.

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

**Status**: OPEN — the largest open item. Built and working; deletable in one commit.

---

## D-G: AI review magic numbers

**Spec says** (FR-024, FR-025): a sticky comment, and sanitised input.

**Invented**: 200 KB diff cap, 200-char title cap, 4000-char body cap, and the
`<!-- gameplane-ai-review -->` marker string.

**Status**: OPEN, low stakes. Defaults, not requirements.

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

**Status**: OPEN.

---

## D-I: Documentation tasks

**Invented**: T041 (`docs/contributing.md`) and T042 (`docs/security.md`) — tasks to edit
docs nobody asked to have edited.

**Status**: OPEN. Both are still unstarted.

---

## Already corrected

- **Fabricated constitution citation.** `plan.md` claimed the Governance section "defers to
  explicit human direction" to justify skipping Principle V. No such clause exists. The
  citation is removed and the violation is now recorded as unjustified.
- **`.gitignore`.** Two Python patterns were appended for `.github/verify-rules/`, outside
  the scope the plan itself declared. Kept only if D-F is resolved as "keep".
