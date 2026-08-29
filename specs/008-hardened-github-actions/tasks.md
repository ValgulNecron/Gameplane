---
description: "Task list for 008-hardened-github-actions"
---

# Tasks: Hardened GitHub Actions CI/CD, AI Automation & Multi-Module Dependabot

**Input**: Design documents from `/specs/008-hardened-github-actions/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: This feature has no Go/TS test suite. The plan proposed `.github/workflows-verify.sh`
as its executable proof — but that verifier is an agent invention `spec.md` never asked for,
and Principle I's status is unresolved pending a maintainer ruling (OPEN-DECISIONS.md D-F).
The falsification discipline below is applied because it is good practice, not because the
constitution demands it of a config gate. The verifier rules are therefore written in Phase 2 and each
user story's rules land **before** the config they check — the rule fails first, the fix
makes it pass. That ordering is deliberate, not incidental; do not reorder it.

**Organization**: Grouped by user story. US1 and US2 are both P1; US1 is the MVP.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: parallelizable — different file, no dependency on an incomplete task
- **[Story]**: US1–US4, mapping to spec.md's user stories

## Path Conventions

Everything lives under `.github/`. No product code (Go, TypeScript, CRDs, charts) is touched
by any task in this file. Two new trees:

- `.github/workflows-verify.sh` — dispatcher
- `.github/verify-rules/` — one Python module per rule, so rules can be authored in parallel

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Scaffolding the rest of the feature builds on.

- [X] T001 Create `.github/verify-rules/_common.py` — YAML loading via `python3` + PyYAML with a `yq`-free fallback, a `Violation(file, line, rule, message)` dataclass, repo-root resolution, and iterators `iter_workflows()`, `iter_jobs()`, `iter_uses()`, `iter_run_blocks()` that every rule consumes. Must preserve line numbers so violations are clickable.
- [X] T002 Create `.github/workflows-verify.sh` — dispatcher with a `verify` subcommand (`set -euo pipefail`), discovering and running every `.github/verify-rules/r*.py` module, printing `R<n> pass: <summary>` or `R<n> FAIL` with `file:line` per violation, exiting non-zero if any rule fails. Mirrors the `verify` subcommand convention of `test/e2e/buckets.sh`. `chmod +x`.
- [X] T003 [P] Re-resolve every SHA in [contracts/action-pins.md](./contracts/action-pins.md) with `git ls-remote --tags --refs` and correct any drift since 2026-08-29; additionally resolve `anthropics/claude-code-action` (row 19) and record it in the same table.

**Checkpoint**: `.github/workflows-verify.sh verify` runs and reports zero rules registered.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The rule-authoring contract every story's rules must satisfy.

**⚠️ CRITICAL**: No user story may begin until T004 fixes the rule interface — rules written against a moving interface get rewritten.

- [X] T004 Define and document the rule-module contract at the top of `.github/verify-rules/_common.py`: each `r*.py` exposes `RULE_ID`, `DESCRIPTION`, and `check(ctx) -> list[Violation]`; `ctx` carries parsed workflows, parsed composite actions, the parsed `dependabot.yml`, the `go.work` module list, and the `find`-derived Dockerfile list, each computed once and shared. Add a `--rule R<n>` flag to `.github/workflows-verify.sh` so a single rule can be exercised in isolation during development.

**Checkpoint**: Foundation ready — US1–US4 rule modules can now be authored in parallel.

---

## Phase 3: User Story 1 — Hardened Static Quality & Multi-Module Lint Gates (Priority: P1) 🎯 MVP

**Goal**: Every workflow and composite action is SHA-pinned, least-privileged, bounded by a timeout, concurrency-gated, and free of script-injection surface — with a CI gate that keeps it that way.

**Independent Test**: `.github/workflows-verify.sh verify` exits 0 on rules R1–R6; quickstart.md scenarios 1, 2 and 4 all produce their expected empty output; a deliberate regression of any one rule (quickstart scenario 6) exits non-zero.

### Rules for User Story 1

> Written first. Each MUST fail against the current tree before the config tasks below fix it.

- [X] T005 [P] [US1] Implement R1 (action SHA pinning) in `.github/verify-rules/r1_action_pins.py` — every `uses:` naming `owner/repo` matches `@[0-9a-f]{40}` followed by exactly `# vX.Y.Z`; local `./.github/` refs exempt; additionally assert no version skew (one `owner/repo` never pins two different SHAs). Per [contracts/action-pins.md](./contracts/action-pins.md).
- [X] T006 [P] [US1] Implement R2 + R3 (permissions) in `.github/verify-rules/r2_permissions.py` — every workflow declares top-level `permissions`; every job declares its own `permissions` block, no exceptions and no allowlist. Per [contracts/permissions-matrix.md](./contracts/permissions-matrix.md).
- [X] T007 [P] [US1] Implement R4 (timeouts) in `.github/verify-rules/r4_timeouts.py` — every job declares `timeout-minutes` ≤ 30, except the four documented exceptions which must additionally carry an inline justification comment. **Must resolve `timeout-minutes: ${{ matrix.job_timeout }}` back to the matrix's literal values and check each**, or the rule is bypassable by any job switching to an expression.
- [X] T008 [P] [US1] Implement R5 (concurrency) in `.github/verify-rules/r5_concurrency.py` — any workflow whose `on:` includes `push` or `pull_request` declares `concurrency` with both `group` and `cancel-in-progress`. Tag-only and `workflow_dispatch`-only workflows are exempt.
- [X] T009 [P] [US1] Implement R6 (script injection) in `.github/verify-rules/r6_injection.py` — no `${{ github.head_ref }}` or `${{ github.event.*.{title,body,ref,name,label} }}` inside a `run:` block. Must distinguish `run:` bodies from `env:` bindings by parsing, not grepping — the safe pattern (`env:` binding, `"$VAR"` in the script) must not trip the rule.

### Implementation for User Story 1

> One task per file so all of these run in parallel. Each applies **all** of US1's changes to its file: SHA pins, top-level permissions, per-job permissions, timeouts, concurrency.

- [ ] T010 [US1] Harden `.github/workflows/ci.yaml` — pin all 10 external actions; **drop `statuses: write` from the top-level `permissions`** (leaving `contents: read`) and add it to the `web` job only; add an explicit `permissions` block to all 17 jobs currently lacking one; verify `changes` and `report` are not broader than needed; confirm all `e2e-go` matrix `job_timeout` values are ≤ 30; add the inline justification comment on `e2e-game-bot`'s 50. **Not [P]** — every other ci.yaml task depends on this landing first.
- [ ] T011 [P] [US1] Harden `.github/workflows/images.yaml` — pin 7 actions; add `permissions` to `common-base` and `game-images`; add `timeout-minutes` (both currently unbounded) — **30/60 are unmeasured guesses, OPEN-DECISIONS.md D-A**; add the `concurrency` block from [contracts/permissions-matrix.md](./contracts/permissions-matrix.md) (`cancel-in-progress` for PRs only, never for a master publish).
- [ ] T012 [P] [US1] Harden `.github/workflows/publish-edge.yaml` — pin 8 actions; add `permissions` to `images`; add a `timeout-minutes` — **45 is an unmeasured guess, OPEN-DECISIONS.md D-A**. Confirm whether keyless signing is in use before adding `id-token: write` — this repo signs keyed/offline, so **omit it** unless verified otherwise.
- [ ] T013 [P] [US1] Harden `.github/workflows/release.yaml` — pin 9 actions; **narrow top-level `contents: write` to `contents: read`** and grant `contents: write` to the `github-release` job only (the other three jobs merely read the tree and push to a registry); add `permissions` and `timeout-minutes` to all four jobs (all currently unbounded) — **45/20/15/30 are unmeasured guesses, OPEN-DECISIONS.md D-A**.
- [ ] T014 [P] [US1] Harden `.github/workflows/republish-modules.yaml` — pin 4 actions; add `permissions` and `timeout-minutes: 30` to `modules`.
- [ ] T015 [P] [US1] Pin the 3 actions in `.github/actions/build-e2e-images/action.yml` (setup-buildx, bake-action, upload-artifact).
- [ ] T016 [P] [US1] Pin `actions/download-artifact` in `.github/actions/e2e-images/action.yml`.
- [ ] T017 [P] [US1] Pin `actions/setup-go` and `actions/cache` in `.github/actions/go-cache/action.yml`.

### CI wiring for User Story 1

- [ ] T018 [US1] Add a `github` path filter output to the `changes` job in `.github/workflows/ci.yaml` covering `.github/**`, `go.work`, and `**/Dockerfile` — the last two so R7 re-runs when a module or image is added. Depends on T010.
- [ ] T019 [US1] Add the `workflows-verify` job to `.github/workflows/ci.yaml` — `needs: [changes]`, `if: needs.changes.outputs.github == 'true'`, `runs-on: ubuntu-latest`, `timeout-minutes: 5`, `permissions: contents: read`, running `.github/workflows-verify.sh verify`. Depends on T018.
- [ ] T020 [US1] Wire `workflows-verify` into the `report` job in `.github/workflows/ci.yaml` — all three places: the `needs:` list, the `NEEDS_ORDER` array, and the `JOB_MATCHERS` map. Missing any one leaves the job silently absent from the PR comment. Depends on T019.

### Falsification for User Story 1

- [ ] T021 [US1] Run quickstart.md scenario 6 against R1, R3, R4 and R5 — regress each rule one at a time, confirm a non-zero exit naming the file, line and rule, then restore the tree. Paste the output into the PR description. A gate that has only ever passed is indistinguishable from a gate that does nothing.

**Checkpoint**: US1 is complete and independently shippable. SC-001, SC-002 and FR-001…FR-011 are satisfied and mechanically enforced.

---

## Phase 4: User Story 2 — Resilient & Hardened Kind E2E Pipeline (Priority: P1)

**Goal**: E2E failure diagnostics are comprehensive **and** sanitized — no token, password, or key ever reaches a step summary or an artifact.

**Independent Test**: quickstart.md scenario 5 — seed a sentinel value into a game pod's environment, force an e2e failure, and confirm the sentinel appears nowhere in the artifact or the run log while the surrounding structure remains readable.

**Depends on**: Phase 2 only. Runs fully in parallel with US1, US3 and US4 — it touches one file none of them touch.

- [X] T022 [P] [US2] Implement R10 (diagnostics safety) in `.github/verify-rules/r10_diagnostics.py` — assert `.github/actions/dump-cluster-state/action.yml` contains no `kubectl get secret` / `describe secret` in any form, and that every log/describe collection step pipes through the redaction filter. Fails against the current tree, which has neither.
- [ ] T023 [US2] Add the redaction filter to `.github/actions/dump-cluster-state/action.yml` as a shared shell function applied at the **emit** boundary — before anything reaches `$GITHUB_STEP_SUMMARY` or an uploaded artifact. Patterns per [data-model.md](./data-model.md) E5: labelled key/value pairs (`password|passwd|token|secret|api[-_]?key|bearer|authorization`), bare JWTs (`eyJ...`), and PEM private-key blocks. **Redact values, preserve keys** — the dump must stay useful for debugging.
- [ ] T024 [US2] Apply the filter to every collection step in `.github/actions/dump-cluster-state/action.yml`: `describe pods`, operator logs, API logs, game-server container logs, ephemeral capture-container logs, events, and the optional `helm history`. Audit all 178 lines — a single unfiltered stream defeats the whole control. Depends on T023.
- [ ] T025 [US2] Confirm the artifact-reuse and cancellation invariants FR-012 and FR-005 assert already hold in `.github/workflows/ci.yaml` — `build-images`/`build-images-arm64` produce the tarball once and every e2e matrix job consumes it via `./.github/actions/e2e-images` with no rebuild, and the `concurrency` group cancels superseded PR runs. Record the finding; open a task only if something is actually broken. Depends on T010.
- [ ] T026 [US2] Execute quickstart.md scenario 5 on a throwaway branch: seed `GAMEPLANE_LEAK_CANARY`, force a test failure, download the artifact, assert `clean` for both the artifact and the run log, then revert. Record the run URL in the PR — this is the only evidence that satisfies SC-005's "100% of failure runs" claim. Depends on T024.

**Checkpoint**: SC-005 and FR-012…FR-016 satisfied, with a live CI run as evidence.

---

## Phase 5: User Story 3 — Comprehensive Multi-Module Dependabot Automation (Priority: P2)

**Goal**: All 14 Go modules, 12 Dockerfiles, npm, and GitHub Actions are monitored, grouped, and rate-limited — replacing a config in which the Go and Docker entries have never matched a manifest.

**Independent Test**: quickstart.md scenario 3 — both `diff`s empty, and the DEAD-entry loop produces no output (run it against `master` first to see the two current dead entries).

**Depends on**: Phase 2 only. Fully parallel with US1, US2 and US4 — one file, touched by nothing else.

- [X] T027 [P] [US3] Implement R7 (Dependabot parity) in `.github/verify-rules/r7_dependabot.py` — gomod directories must equal the `go.work` module list exactly; docker directories must equal `find . -name Dockerfile -not -path './website/*'` exactly; every entry must declare `groups` (≥ 1), `open-pull-requests-limit`, and `commit-message.prefix: chore(deps)`; **no entry may name a directory lacking its ecosystem's manifest** — the exact failure that silently disabled Go and Docker updates.
- [ ] T028 [US3] Rewrite `.github/dependabot.yml` to the 28-entry matrix in [contracts/dependabot-matrix.md](./contracts/dependabot-matrix.md): 14 `gomod` + 1 `npm` + 12 `docker` + 1 `github-actions`. Drop the dead `gomod: /` and `docker: /` entries. Apply the FR-019 corrections from research.md D-08 — remove `/` and `/tunnel` (no Dockerfile), add `/test/e2e` and `/web` (they have one).
- [ ] T029 [US3] Add the group definitions to `.github/dependabot.yml` — **group names and shapes are unratified (OPEN-DECISIONS.md D-C).** Proposed: per Go module, the `k8s` group (`k8s.io/*`, `sigs.k8s.io/*`) declared **before** the `<module>-minor-patch` catch-all, since Dependabot matches groups in declaration order and a catch-all listed first swallows everything. The k8s carve-out is not optional: those libraries are version-locked and a PR bumping one alone does not compile. Modules with no k8s dependency (`netguard`, `gameaction`, `gameproto`, `svcutil`) get only the minor-patch group. Depends on T028.
- [ ] T030 [US3] Set limits and schedules in `.github/dependabot.yml` — **limit values and the npm stagger are unratified (OPEN-DECISIONS.md D-B, D-D); confirm before applying.** Proposed: gomod 3 / npm 10 / docker 5 / actions 5, weekly Monday 03:00 UTC with npm at 04:00. Note gomod 3 sits below the spec's own "max 5–10". `commit-message.prefix: "chore(deps)"` with `include: "scope"`, fixing the current `"chore: "` which yields a malformed `chore: (deps):` subject. Depends on T029.

**Checkpoint**: SC-003 and FR-017…FR-021 satisfied. Adding a 15th Go module now reddens CI until Dependabot is updated in the same change.

---

## Phase 6: User Story 4 — Secure GitHub AI Workflow for PR Review (Priority: P2)

**Goal**: AI review on every PR, including forks, with no job ever holding both attacker-controlled code and a privileged token.

**Independent Test**: quickstart.md scenario 7 — all five sub-checks: same-repo comment, stickiness across a second push, fork PR green with no comment and a step-summary report, `collect` provably without the API key, and an injection attempt in the PR body that changes nothing.

**Depends on**: Phase 2, plus T003 (the resolved `claude-code-action` pin) and T005 (R1, which the new file must satisfy on arrival).

- [X] T031 [P] [US4] Implement R8 (secret confinement) in `.github/verify-rules/r8_secrets.py` — `COSIGN_PRIVATE_KEY`, `COSIGN_PASSWORD` and registry credentials appear only in `images.yaml`, `publish-edge.yaml`, `release.yaml`, `republish-modules.yaml`; `ANTHROPIC_API_KEY` appears only in `ai-review.yaml`'s `review` job and **never** in its `collect` job.
- [X] T032 [P] [US4] Implement R9 (no `pull_request_target`) in `.github/verify-rules/r9_pr_target.py` — reject the trigger repo-wide. It runs with full secrets *and* a write token; one added checkout of `head.sha` turns it into RCE against repository secrets.
- [ ] T033 [US4] Create `.github/workflows/ai-review.yaml` with the `collect` job — `on: pull_request: types: [opened, synchronize, reopened]`, top-level `permissions: {}`, job `permissions: contents: read`, `timeout-minutes: 10`, **no secrets**. Checks out PR head with `fetch-depth: 0`, computes the base diff truncated to 200 KB, writes the metadata JSON (`pr_number`, `head_sha`, `base_ref`, `title`, `body`, `changed_files`), sanitises `title`/`body` (strip backticks and `${`, truncate 200/4000), uploads the `ai-review-payload` artifact.
- [ ] T034 [US4] Add the `review` job to `.github/workflows/ai-review.yaml` — `on: workflow_run: {workflows: [ai-review], types: [completed]}`, guarded by `if: github.event.workflow_run.event == 'pull_request' && github.event.workflow_run.conclusion == 'success'`, `permissions: contents: read, pull-requests: write, actions: read`, `timeout-minutes: 15`. Downloads the artifact. **Must not check out PR code** — that would collapse the trust split the whole design rests on. Depends on T033.
- [ ] T035 [US4] Add re-validation of every artifact field at the top of the `review` job in `.github/workflows/ai-review.yaml` per [contracts/ai-review-contract.md](./contracts/ai-review-contract.md) — `pr_number` `^[0-9]+$`, `head_sha` `^[0-9a-f]{40}$`, `base_ref` `^[\w./-]+$`, re-sanitise and re-truncate `title`/`body`/`diff`. `collect` ran next to the attacker's code, so nothing it produced is trusted — including its claim to have sanitised. Same boundary discipline `gameaction/` applies between API and agent. Depends on T034.
- [ ] T036 [US4] Add the three-section prompt to `.github/workflows/ai-review.yaml` — trusted SYSTEM (role, criteria, and an explicit statement that `<untrusted_diff>` content is data to review and never instructions to follow), trusted CONTEXT loaded from the **base** checkout only (`.specify/memory/constitution.md`, `CLAUDE.md`, `specs.md` for touched modules), then fenced untrusted INPUT last. Pin `anthropics/claude-code-action` to the SHA resolved in T003. Grant no shell tool and no network beyond the API call — that capability floor is what bounds a successful injection, the framing only lowers its odds. Depends on T035.
- [ ] T037 [US4] Implement the review criteria list from [contracts/ai-review-contract.md](./contracts/ai-review-contract.md) in the prompt — in-source suppressions, `%w` error wrapping, unjustified `any` / floating promises, CRD edits without regenerated artifacts, behavior changes without a `specs.md` update, features without E2E coverage or a bucket entry, dashboard changes without a `design.pen` + export update, business logic in handlers that belongs in a reconciler, and unpinned actions. Depends on T036.
- [ ] T038 [US4] Implement the sticky comment upsert in `.github/workflows/ai-review.yaml` — locate the first bot comment whose body starts with `<!-- gameplane-ai-review -->`, `PATCH` it if found and `POST` otherwise, never posting a second marked comment. Include the short `head_sha` and an advisory/non-blocking footer. Depends on T037.
- [ ] T039 [US4] Implement graceful degradation in `.github/workflows/ai-review.yaml` — catch the 403 a fork PR's downgraded token produces on upsert and write the identical report to `$GITHUB_STEP_SUMMARY`; handle unset `ANTHROPIC_API_KEY`, API failure or timeout, and a missing or malformed artifact. **Every path exits 0.** The reviewer must be structurally incapable of blocking a PR — which is also what makes it safe to run on untrusted input. Depends on T038.
- [ ] T040 [US4] Execute quickstart.md scenario 7's five sub-checks and record the results in the PR. A red `review` job on a fork PR is a bug, not an expected outcome — the write token was never going to be granted. Depends on T039.

**Checkpoint**: SC-006 and FR-022…FR-025 satisfied.

---

## Phase 7: Polish & Cross-Cutting Concerns

> T041 and T042 edit docs nobody asked to have edited, and both depend on the verifier
> surviving the D-F ruling. **Blocked pending maintainer review** — see OPEN-DECISIONS.md
> D-F and D-I.

- [ ] T041 [P] **[BLOCKED — D-I]** Document the verifier in `docs/contributing.md` — what `.github/workflows-verify.sh verify` checks, how to run it before pushing, and how to add a rule.
- [ ] T042 [P] **[BLOCKED — D-I]** Add a short "CI hardening" section to `docs/security.md` covering the SHA-pinning policy, the least-privilege permission model, secret confinement, and the AI reviewer's trust split.
- [ ] T043 Run the full local pre-push check from quickstart.md ("Full local pre-push check") and confirm `PRE-PUSH OK`.
- [ ] T044 Push the branch and confirm a **fully green** CI run — including every pre-existing e2e bucket. Not a formality: the hardening narrows every job's token and repins every action, and a green e2e tier is what proves nothing was quietly relying on the over-broad `statuses: write`. Per Constitution Principle VI, nothing here is validated until this run is green.
- [ ] T045 Complete quickstart.md's Definition of Done table — all 8 evidence rows, with the scenario 5 and 7 run URLs and the scenario 6 falsification output in the PR description. Depends on T044.
- [ ] T046 Merge to `master` and delete the branch, remote and local, per CLAUDE.md rule 12. Depends on T045.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: no dependencies.
- **Phase 2 (Foundational)**: depends on T001–T002. **Blocks every user story** — the rule-module interface must be fixed before rules are written against it.
- **Phase 3–6 (US1–US4)**: all depend only on Phase 2. **All four can run concurrently.**
- **Phase 7 (Polish)**: depends on the stories being merged.

### User Story Dependencies

| Story | Depends on | Notes |
|---|---|---|
| **US1** (P1) | Phase 2 | The MVP. Nothing depends on it except US4's pin convention (T005). |
| **US2** (P1) | Phase 2 | Touches only `dump-cluster-state/action.yml` — zero file overlap with US1/US3/US4. T025 alone waits on T010. |
| **US3** (P2) | Phase 2 | Touches only `dependabot.yml`. Fully independent. |
| **US4** (P2) | Phase 2, T003, T005 | New file must arrive already satisfying R1. |

### File-Conflict Map

The constraint that actually governs parallelism here — one file, one writer at a time:

| File | Tasks | Concurrency |
|---|---|---|
| `.github/workflows/ci.yaml` | T010 → T018 → T019 → T020 | **Strictly serial.** The single longest chain. |
| `.github/workflows/images.yaml` | T011 | free |
| `.github/workflows/publish-edge.yaml` | T012 | free |
| `.github/workflows/release.yaml` | T013 | free |
| `.github/workflows/republish-modules.yaml` | T014 | free |
| `.github/actions/*/action.yml` (3 files) | T015, T016, T017 | free, one each |
| `.github/actions/dump-cluster-state/action.yml` | T023 → T024 | serial pair |
| `.github/dependabot.yml` | T028 → T029 → T030 | serial chain |
| `.github/workflows/ai-review.yaml` | T033 → T034 → … → T039 | serial chain, 7 deep |
| `.github/verify-rules/r*.py` | T005–T009, T022, T027, T031, T032 | **9 files, 9 writers, all free** |

### Parallel Opportunities

- **T005–T009, T022, T027, T031, T032** — nine rule modules, nine separate files, no shared state. The single largest fan-out in the feature; launch all nine at once once T004 lands.
- **T011–T017** — seven independent config files, launchable together with T010.
- **US1, US2, US3, US4** — four fully independent streams after Phase 2.
- **T041, T042** — two different docs files.

---

## Parallel Example: the rule wave

```bash
# After T004 fixes the rule interface — nine agents, nine files, zero contention:
Task: "Implement R1 action SHA pinning in .github/verify-rules/r1_action_pins.py"
Task: "Implement R2+R3 permissions in .github/verify-rules/r2_permissions.py"
Task: "Implement R4 timeouts in .github/verify-rules/r4_timeouts.py"
Task: "Implement R5 concurrency in .github/verify-rules/r5_concurrency.py"
Task: "Implement R6 script injection in .github/verify-rules/r6_injection.py"
Task: "Implement R7 dependabot parity in .github/verify-rules/r7_dependabot.py"
Task: "Implement R8 secret confinement in .github/verify-rules/r8_secrets.py"
Task: "Implement R9 no pull_request_target in .github/verify-rules/r9_pr_target.py"
Task: "Implement R10 diagnostics safety in .github/verify-rules/r10_diagnostics.py"
```

## Parallel Example: the config wave

```bash
# T010 runs alongside these; it owns ci.yaml exclusively for the whole wave:
Task: "Harden .github/workflows/images.yaml"
Task: "Harden .github/workflows/publish-edge.yaml"
Task: "Harden .github/workflows/release.yaml"
Task: "Harden .github/workflows/republish-modules.yaml"
Task: "Pin actions in .github/actions/build-e2e-images/action.yml"
Task: "Pin actions in .github/actions/e2e-images/action.yml"
Task: "Pin actions in .github/actions/go-cache/action.yml"
```

---

## Implementation Strategy

### MVP (US1 only)

1. Phase 1 → Phase 2 → Phase 3.
2. **Stop and validate**: quickstart scenarios 1, 2, 4, 6.
3. Shippable on its own. SC-001 and SC-002 are met, the supply-chain and privilege risks are closed, and the gate prevents regression. US2–US4 add coverage on top of a foundation that already holds.

### Incremental Delivery

Setup + Foundational → **US1 (MVP, ship)** → US2 (diagnostics, ship) → US3 (dependabot, ship) → US4 (AI review, ship). Each is independently mergeable; none breaks the previous.

### Parallel Strategy

After Phase 2, fan out four streams. Per Constitution Principle V and CLAUDE.md rule 13, `/speckit-implement` delegates each wave through a Workflow with `model` set explicitly on every `agent()` call, starting at `haiku` and escalating one tier only on demonstrated failure — then reviews the combined output one tier up before accepting it.

The rule wave (nine files) and the config wave (seven files) are the two places where fan-out actually pays; the `ci.yaml`, `ai-review.yaml`, and `dependabot.yml` chains are serial no matter how many agents are available, and `ci.yaml` (T010 → T020) is the critical path.

---

## Notes

- Rules land **before** the config they check. Each must be observed failing against the pre-fix tree — that observation is the feature's substitute for an E2E test (plan.md Complexity Tracking), and T021 is where it gets recorded.
- Commit per task or per logical group, signed (`git commit -s`), conventional prefixes — `ci:` for workflow changes, `chore:` for dependabot, `docs:` for Phase 7.
- No product code is touched. A diff outside `.github/`, `docs/`, and `specs/` means scope has drifted.
- Do not run test or lint suites locally (Principle VI). `.github/workflows-verify.sh verify` is a static parser, not a suite, and is permitted — as is a YAML parse check.
- Out of scope, named in plan.md so they don't creep in: reusable-workflow refactoring, splitting the 1400-line `ci.yaml`, `actionlint`/`zizmor` adoption, bucket restructuring, branch protection rules.
