---
description: "Task list for 008-hardened-github-actions"
---

# Tasks: Hardened GitHub Actions CI/CD, AI Automation & Multi-Module Dependabot

**Input**: Design documents from `/specs/008-hardened-github-actions/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: This feature has no Go/TS test suite. A `workflow-lint` job in ci.yaml (ruling D-H, amended by D-J) runs `actionlint` and `zizmor` to validate expression-injection safety, YAML schema, shellcheck findings, and SHA pinning. Permissions presence/values, timeout presence/values, concurrency gating, and Dependabot parity are code-review-only. Falsification is via the quickstart scenarios: each task is validated by running a scenario that exercises the hardening after the fix lands.

**Organization**: Grouped by user story. US1 and US2 are both P1; US1 is the MVP.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: parallelizable — different file, no dependency on an incomplete task
- **[Story]**: US1–US4, mapping to spec.md's user stories

## Path Conventions

Everything lives under `.github/` and `docs/`. No product code (Go, TypeScript, CRDs, charts) is touched
by any task in this file. Hardening is applied directly to workflow and action files. The workflow-lint gate (actionlint + zizmor) validates expression-injection safety and SHA pinning; permissions, timeouts, concurrency, and Dependabot parity are enforced by code review.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Resolve action SHAs used throughout the hardening tasks.

- [X] T003 [P] Re-resolve every SHA in [contracts/action-pins.md](./contracts/action-pins.md) with `git ls-remote --tags --refs` and correct any drift since 2026-08-29; additionally resolve `anthropics/claude-code-action` (row 19) and record it in the same table.

**Checkpoint**: Action pins in contracts/action-pins.md are current.

---

## Phase 2: Foundational (No Blocking Prerequisites)

**Purpose**: T004 (the rule-module interface contract) was deleted per ruling D-H — it was an agent invention, not requested in the spec. Phase 1 is now sufficient to unblock all user stories.

**Checkpoint**: Ready to begin user stories US1–US4 in parallel.

---

## Phase 3: User Story 1 — Hardened Static Quality & Multi-Module Lint Gates (Priority: P1) 🎯 MVP

**Goal**: Every workflow and composite action is SHA-pinned (enforced by `zizmor`) and free of script-injection surface (enforced by `actionlint`); all jobs are least-privileged with explicit minimal permissions, bounded by explicit timeouts, and concurrency-gated (enforced by code review).

**Independent Test**: quickstart.md scenarios 1, 2, 4, and 6 validate the hardening. Scenario 6 deliberately introduces a regression and confirms a non-zero exit.

### Implementation for User Story 1

> One task per file so all of these run in parallel. Each applies **all** of US1's changes to its file: SHA pins, top-level permissions, per-job permissions, timeouts, concurrency.

- [X] T010 [US1] Harden `.github/workflows/ci.yaml` — pin all 10 external actions; **drop `statuses: write` from the top-level `permissions`** (leaving `contents: read`) and add it to the `web` job only; add an explicit `permissions` block to all 17 jobs currently lacking one; verify `changes` and `report` are not broader than needed; set `job_timeout` to 60 for all five E2E jobs (`e2e-go`, `e2e-multicluster`, `e2e-upgrade`, `e2e-web-live`, `e2e-game-bot`); add an inline justification comment on each since 60 exceeds FR-004's ≤30 default. **Not [P]** — every other ci.yaml task depends on this landing first.
- [X] T011 [P] [US1] Harden `.github/workflows/images.yaml` — pin 7 actions; add `permissions` to `common-base` and `game-images`; add `timeout-minutes: 10` to both `common-base` and `game-images` (OPEN-DECISIONS.md D-A — `common-base` marked [EXTENSION] pending maintainer approval); add the `concurrency` block from [contracts/permissions-matrix.md](./contracts/permissions-matrix.md) (`cancel-in-progress` for PRs only, never for a master publish).
- [X] T012 [P] [US1] Harden `.github/workflows/publish-edge.yaml` — pin 8 actions; add `permissions` to `images`; add `timeout-minutes: 35` to `images` with an inline justification comment citing the measured 31-minute max (exceeds FR-004's ≤30 default). Confirm whether keyless signing is in use before adding `id-token: write` — this repo signs keyed/offline, so **omit it** unless verified otherwise.
- [X] T013 [P] [US1] Harden `.github/workflows/release.yaml` — pin 9 actions; **narrow top-level `contents: write` to `contents: read`** and grant `contents: write` to the `github-release` job only (the other three jobs merely read the tree and push to a registry); add `permissions` and `timeout-minutes: 30` to the `images` job (exactly at FR-004's ≤30 default); add `permissions` and `timeout-minutes: 15` to the `chart`, `github-release`, and `modules` jobs (OPEN-DECISIONS.md D-A — marked [EXTENSION] pending maintainer approval).
- [X] T014 [P] [US1] Harden `.github/workflows/republish-modules.yaml` — pin 4 actions; add `permissions` and `timeout-minutes: 15` to `modules` (OPEN-DECISIONS.md D-A, marked [EXTENSION] pending maintainer approval).
- [X] T015 [P] [US1] Pin the 3 actions in `.github/actions/build-e2e-images/action.yml` (setup-buildx, bake-action, upload-artifact).
- [X] T016 [P] [US1] Pin `actions/download-artifact` in `.github/actions/e2e-images/action.yml`.
- [X] T017 [P] [US1] Pin `actions/setup-go` and `actions/cache` in `.github/actions/go-cache/action.yml`.
- [X] T018 [US1] Add `zizmor` to the `workflow-lint` job in `.github/workflows/ci.yaml` — pin the zizmor action to a SHA (resolved in T003 or via lookup), configure it to enforce unpinned `uses:` refs, permissions misuse, and `pull_request_target` misuse. Wire the job into the `report` job's `needs:`, `NEEDS_ORDER`, and `JOB_MATCHERS` so failures propagate to PR status. Job layout (whether zizmor runs in the same `workflow-lint` job as actionlint or as its own parallel job) and ruleset/severity are open questions — settle these during implementation.


### Validation for User Story 1

- [X] T021 [US1] Run quickstart.md scenario 6 — deliberately regress a zizmor-enforced control by removing a SHA pin from an action `uses:`, run `zizmor` locally on the modified file to confirm it exits non-zero (as the scenario demonstrates), then restore the tree. Paste the zizmor output into the PR description. Separately, optionally regress an actionlint-enforced control (e.g., inject a bash expression into a run: body) to demonstrate actionlint's catch. Manual review confirms that removing a permissions block or timeout would be caught in code review.

**Checkpoint**: US1 is complete and independently shippable. SC-001 and FR-003 are verified by `zizmor` (SHA pinning); FR-006 is verified by `actionlint` (expression-injection safety). SC-002, FR-001, FR-002, FR-004, FR-005, and FR-007–FR-011 are satisfied and enforced by code review (permissions presence/values, timeouts, concurrency, and static test suite).

---

## Phase 4: User Story 2 — Resilient & Hardened Kind E2E Pipeline (Priority: P1)

**Goal**: E2E failure diagnostics are comprehensive **and** sanitized — no token, password, or key ever reaches a step summary or an artifact.

**Independent Test**: quickstart.md scenario 5 — seed a sentinel value into a game pod's environment, force an e2e failure, and confirm the sentinel appears nowhere in the artifact or the run log while the surrounding structure remains readable.

**Depends on**: Phase 2 only. Runs fully in parallel with US1, US3 and US4 — it touches one file none of them touch.

**Coverage gap (D-F)**: R10's dump-cluster-state redaction wiring check is no longer enforced. The `redact()` filter itself must be hand-audited: ensure it is applied at every emit boundary (steps writing to `$GITHUB_STEP_SUMMARY` or uploading artifacts).

- [X] T023 [US2] Add the redaction filter to `.github/actions/dump-cluster-state/action.yml` as a shared shell function applied at the **emit** boundary — before anything reaches `$GITHUB_STEP_SUMMARY` or an uploaded artifact. Patterns per [data-model.md](./data-model.md) E5: labelled key/value pairs (`password|passwd|token|secret|api[-_]?key|bearer|authorization`), bare JWTs (`eyJ...`), and PEM private-key blocks. **Redact values, preserve keys** — the dump must stay useful for debugging.
- [X] T024 [US2] Apply the filter to every collection step in `.github/actions/dump-cluster-state/action.yml`: `describe pods`, operator logs, API logs, game-server container logs, ephemeral capture-container logs, events, and the optional `helm history`. Audit all 178 lines — a single unfiltered stream defeats the whole control. Depends on T023.
- [X] T025 [US2] Confirm the artifact-reuse and cancellation invariants FR-012 and FR-005 assert already hold in `.github/workflows/ci.yaml` — `build-images`/`build-images-arm64` produce the tarball once and every e2e matrix job consumes it via `./.github/actions/e2e-images` with no rebuild, and the `concurrency` group cancels superseded PR runs. Record the finding; open a task only if something is actually broken. Depends on T010.
- [ ] T026 [US2] Execute quickstart.md scenario 5 on a throwaway branch: seed `GAMEPLANE_LEAK_CANARY`, force a test failure, download the artifact, assert `clean` for both the artifact and the run log, then revert. Record the run URL in the PR — this is the only evidence that satisfies SC-005's "100% of failure runs" claim. Depends on T024.

**Checkpoint**: SC-005 and FR-012…FR-016 satisfied, with a live CI run as evidence.

---

## Phase 5: User Story 3 — Comprehensive Multi-Module Dependabot Automation (Priority: P2)

**Goal**: All 14 Go modules, 12 Dockerfiles, npm, and GitHub Actions are monitored, grouped, and rate-limited — replacing a config in which the Go and Docker entries have never matched a manifest.

**Independent Test**: quickstart.md scenario 3 — both `diff`s empty, and the DEAD-entry loop produces no output (run it against `master` first to see the two current dead entries).

**Depends on**: Phase 2 only. Fully parallel with US1, US2 and US4 — one file, touched by nothing else.

**Coverage gap (D-F)**: R7's Dependabot<->tree parity check is no longer enforced. When a new Go module or Dockerfile is added, the corresponding Dependabot entry must be added in the same change — but CI will not fail if it is forgotten. Manual review required.

- [X] T028 [US3] Rewrite `.github/dependabot.yml` to the 28-entry matrix in [contracts/dependabot-matrix.md](./contracts/dependabot-matrix.md): 14 `gomod` + 1 `npm` + 12 `docker` + 1 `github-actions`. Drop the dead `gomod: /` and `docker: /` entries. Apply the FR-019 corrections from research.md D-08 — remove `/` and `/tunnel` (no Dockerfile), add `/test/e2e` and `/web` (they have one).
- [X] T029 [US3] Add the group definitions to `.github/dependabot.yml` (OPEN-DECISIONS.md D-C): one group per module, each batching `["minor", "patch"]` across all dependencies, with no k8s carve-out. Modules with no k8s dependency (`netguard`, `gameaction`, `gameproto`, `svcutil`) follow the same pattern as modules with k8s dependencies. **Accepted consequence**: a grouped PR moving only some version-locked k8s libraries may not compile; those PRs get closed by hand. This simpler config was preferred over the declaration-order rule and the k8s carve-out. Depends on T028.
- [X] T030 [US3] Set limits and schedules in `.github/dependabot.yml` (OPEN-DECISIONS.md D-B, D-D): `open-pull-requests-limit` set to 5 for each gomod entry (x14), 10 for npm, 5 for each docker entry (x12), and 5 for github-actions. Schedule all entries to weekly, Monday at 03:00 UTC — no stagger. `commit-message.prefix: "chore(deps)"` with `include: "scope"`, fixing the current `"chore: "` which yields a malformed `chore: (deps):` subject. Depends on T029.

**Checkpoint**: SC-003 and FR-017…FR-021 satisfied. Adding a 15th Go module now reddens CI until Dependabot is updated in the same change.

---

## Phase 6: User Story 4 — Secure GitHub AI Workflow for PR Review (Priority: P2)

**Goal**: AI review on every PR, including forks, with no job ever holding both attacker-controlled code and a privileged token.

**Independent Test**: quickstart.md scenario 7 — all five sub-checks: same-repo comment, stickiness across a second push, fork PR green with no comment and a step-summary report, `collect` provably without the API key, and an injection attempt in the PR body that changes nothing.

**Depends on**: Phase 2, plus T003 (the resolved `claude-code-action` pin) and T010-T017 (action SHA pinning, which the new file must satisfy on arrival).

- [X] T033 [US4] Create `.github/workflows/ai-review.yaml` with the `collect` job — `on: pull_request: types: [opened, synchronize, reopened]`, top-level `permissions: {}`, job `permissions: contents: read`, `timeout-minutes: 10`, **no secrets**. Declare named `env:` constants per OPEN-DECISIONS.md D-G: `MAX_DIFF_BYTES: 200000` (approximately 200 KB, about 50k tokens, leaving room for constitution/CLAUDE.md/specs in the review context); `MAX_TITLE_CHARS: 200` (GitHub's PR title limit is 256); `MAX_BODY_CHARS: 4000` (enough for a real description with a checklist). `STICKY_MARKER` is declared in `ai-review-respond.yaml`, not here — only the `review` job posts comments. Checks out PR head with `fetch-depth: 0`, computes the base diff truncated to `MAX_DIFF_BYTES`, writes the metadata JSON (`pr_number`, `head_sha`, `base_ref`, `title`, `body`, `changed_files`), sanitises `title`/`body` (strip backticks and `${`, truncate to `MAX_TITLE_CHARS`/`MAX_BODY_CHARS`), uploads the `ai-review-payload` artifact.
- [X] T034 [US4] Create `.github/workflows/ai-review-respond.yaml` with the `review` job — `on: workflow_run: {workflows: [ai-review], types: [completed]}`, guarded by `if: github.event.workflow_run.event == 'pull_request' && github.event.workflow_run.conclusion == 'success'`, `permissions: contents: read, pull-requests: write, actions: read`, `timeout-minutes: 15`. Downloads the artifact. **Must not check out PR code** — that would collapse the trust split the whole design rests on. Depends on T033.
- [X] T035 [US4] Add re-validation of every artifact field at the top of the `review` job in `.github/workflows/ai-review-respond.yaml` per [contracts/ai-review-contract.md](./contracts/ai-review-contract.md) — `pr_number` `^[0-9]+$`, `head_sha` `^[0-9a-f]{40}$`, `base_ref` `^[\w./-]+$`, re-sanitise and re-truncate `title`/`body`/`diff`. `collect` ran next to the attacker's code, so nothing it produced is trusted — including its claim to have sanitised. Same boundary discipline `gameaction/` applies between API and agent. Depends on T034.
- [X] T036 [US4] Add the three-section prompt to `.github/workflows/ai-review-respond.yaml` — trusted SYSTEM (role, criteria, and an explicit statement that `<untrusted_diff>` content is data to review and never instructions to follow), trusted CONTEXT loaded from the **base** checkout only (`.specify/memory/constitution.md`, `CLAUDE.md`, `specs.md` for touched modules), then fenced untrusted INPUT last. Pin `anthropics/claude-code-action` to the SHA resolved in T003. Grant no shell tool and no network beyond the API call — that capability floor is what bounds a successful injection, the framing only lowers its odds. Depends on T035.
- [X] T037 [US4] Implement the review criteria list from [contracts/ai-review-contract.md](./contracts/ai-review-contract.md) in the prompt (in `.github/workflows/ai-review-respond.yaml`) — in-source suppressions, `%w` error wrapping, unjustified `any` / floating promises, CRD edits without regenerated artifacts, behavior changes without a `specs.md` update, features without E2E coverage or a bucket entry, dashboard changes without a `design.pen` + export update, business logic in handlers that belongs in a reconciler, and unpinned actions. Depends on T036.
- [X] T038 [US4] Implement the sticky comment upsert in `.github/workflows/ai-review-respond.yaml` — locate the first bot comment whose body starts with the `STICKY_MARKER` env var, `PATCH` it if found and `POST` otherwise, never posting a second marked comment. Include the short `head_sha` and an advisory/non-blocking footer. Depends on T037.
- [X] T039 [US4] Implement graceful degradation in `.github/workflows/ai-review-respond.yaml` — catch the 403 a fork PR's downgraded token produces on upsert and write the identical report to `$GITHUB_STEP_SUMMARY`; handle unset `ANTHROPIC_API_KEY`, API failure or timeout, and a missing or malformed artifact. **Every path exits 0.** The reviewer must be structurally incapable of blocking a PR — which is also what makes it safe to run on untrusted input. Depends on T038.
- [ ] T040 [US4] Execute quickstart.md scenario 7's five sub-checks and record the results in the PR. A red `review` job on a fork PR is a bug, not an expected outcome — the write token was never going to be granted. Depends on T039.

**Checkpoint**: SC-006 and FR-022…FR-025 satisfied.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T043 Run the full local pre-push check from quickstart.md ("Full local pre-push check") and confirm `PRE-PUSH OK`.
- [ ] T044 Push the branch and confirm a **fully green** CI run — including every pre-existing e2e bucket. Not a formality: the hardening narrows every job's token and repins every action, and a green e2e tier is what proves nothing was quietly relying on the over-broad `statuses: write`. Per Constitution Principle VI, nothing here is validated until this run is green.
- [ ] T045 Complete quickstart.md's Definition of Done table — all 8 evidence rows, with the scenario 5 and 7 run URLs and the scenario 6 falsification output in the PR description. Depends on T044.
- [ ] T041 **[Last]** — after all other tasks are complete, update `docs/contributing.md` and `docs/security.md` to describe the final CI: the SHA-pinning policy and how Dependabot maintains the pins, the lowest-privilege per-job permission model, the workflow-lint gate (`actionlint` + `zizmor`), the timeout budgets from D-A, secret confinement, and the AI reviewer's trust split. Written against the merged state — no forward references to work not yet done. Depends on T045.
- [ ] T046 Merge to `master` and delete the branch, remote and local, per CLAUDE.md rule 12. Depends on T041.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: no dependencies.
- **Phase 2 (Foundational)**: depends on Phase 1. Unblocks all user stories.
- **Phase 3–6 (US1–US4)**: all depend only on Phase 2. **All four can run concurrently.**
- **Phase 7 (Polish)**: depends on the stories being merged.

### User Story Dependencies

| Story | Depends on | Notes |
|---|---|---|
| **US1** (P1) | Phase 2 | The MVP. SHA pinning verified by `zizmor`; expression-injection safety verified by `actionlint`; permissions, timeouts, and concurrency enforced by code review. |
| **US2** (P1) | Phase 2 | Touches only `dump-cluster-state/action.yml` — zero file overlap with US1/US3/US4. T025 alone waits on T010. |
| **US3** (P2) | Phase 2 | Touches only `dependabot.yml`. Fully independent. Note R7 gap: no CI enforcement of parity. |
| **US4** (P2) | Phase 2, T003, T010–T017 | New file must arrive already satisfying action SHA pins. |

### File-Conflict Map

The constraint that actually governs parallelism here — one file, one writer at a time:

| File | Tasks | Concurrency |
|---|---|---|
| `.github/workflows/ci.yaml` | T010, T018 | **Critical path**, T010 must land first; T018 depends on T010. |
| `.github/workflows/images.yaml` | T011 | free |
| `.github/workflows/publish-edge.yaml` | T012 | free |
| `.github/workflows/release.yaml` | T013 | free |
| `.github/workflows/republish-modules.yaml` | T014 | free |
| `.github/actions/*/action.yml` (3 files) | T015, T016, T017 | free, one each |
| `.github/actions/dump-cluster-state/action.yml` | T023 → T024 | serial pair |
| `.github/dependabot.yml` | T028 → T029 → T030 | serial chain |
| `.github/workflows/ai-review.yaml` | T033 | free (creates `collect` job) |
| `.github/workflows/ai-review-respond.yaml` | T034 → T035 → … → T039 | serial chain, 6 deep (creates `review` job) |

### Parallel Opportunities

- **T011–T017** — seven independent config files, launchable together with T010 (but not T018, which depends on T010).
- **T018** — wired into ci.yaml by T010, depends on T010 landing first.
- **US1, US2, US3, US4** — four fully independent streams after Phase 2.

---

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

The config wave (seven files) and the `ai-review.yaml`/`dependabot.yml` chains are the places where fan-out and serialism constrain parallelism. T010 is the critical path since every parallel task in T011–T017 depends on it landing first.

---

## Notes

- Hardening is validated by quickstart scenarios: each task tests its changes without relying on CI gates.
- Commit per task or per logical group, signed (`git commit -s`), conventional prefixes — `ci:` for workflow changes, `chore:` for dependabot, `docs:` for Phase 7.
- No product code is touched. A diff outside `.github/` and `docs/` means scope has drifted.
- Do not run test or lint suites locally (Principle VI). A YAML parse check is fine.
- The workflow-lint gate (actionlint + zizmor) validates expression injection, YAML schema, and shellcheck findings (actionlint) and SHA pinning (zizmor) via a new `workflow-lint` job in ci.yaml (ruling D-H, amended by D-J). Permissions, timeouts, concurrency, and Dependabot parity are code-review-enforced. The old verifier (workflows-verify.sh + .github/verify-rules/) is deleted.
- Out of scope, named in plan.md so they don't creep in: reusable-workflow refactoring, splitting the 1400-line `ci.yaml`, additional linters beyond `actionlint`, bucket restructuring, branch protection rules.
