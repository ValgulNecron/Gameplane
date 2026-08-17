# Research: Lint Backlog Wave 2 — Static Analysis Gate for api, agent, test/e2e — Phase 0 Decisions

**Date**: 2026-08-17 | **Branch**: `004-lint-backlog-wave2` | **Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

This document records decisions made during planning for bringing three large Go modules (api, agent, test/e2e) under the uniform static-analysis gate already applied to the other 10 modules in the `go.work` workspace. Each decision is presented with rationale and rejected alternatives, backed by evidence from the codebase.

---

## Unknowns Resolved

| Planner's Question | Resolution Decision | Evidence |
|---|---|---|
| What is the current state of PR #216, and can it be salvaged? | PARTIAL SALVAGE: api and agent fix commits are clean; test/e2e fixes are stale against master rewrites. | **Decision 1** below; branch `chore/lint-backlog-wave2` has 11 commits; fork-base b3d5b38 is ancestor of all branch commits; api/agent fixes have zero overlap with master changes in those modules since fork; test/e2e has no textual merge conflicts but is stale against restructured code. |
| Is the starting finding count measurable without running the linter locally? | No; Constitution VI forbids local linter runs, so the count is measured by CI on the enumeration commit. | **Decision 2** below; CI run 31971653399 on the branch reported 91 residual findings (after fixes), not the initial ~488; the measurement must come from CI, not assertion. |
| How should sequencing balance the tension between "fix-first" and "enable-last"? | Fixes first, then early enablement (red CI acceptable for measurement), then enable at merge (CI green). | **Decision 3** below; wave 1 (PR #215) landed fixes before enablement commit ab49814; measurement and verification must happen before master sees the change. |
| How can parallelism scale to multiple workers without file conflicts? | Partition by package directory, not by linter or by finding count. | **Decision 4** below; api has 13 packages with 199 files; agent has 17 packages with 64 files; test/e2e has 23 packages but monolithic at 51 root files. Each worker takes one package. |
| Can new suppression directives be introduced? | No; Constitution III forbids silencing, and the feature preserves zero-suppression property. | **Decision 5** below; `.golangci.yml` has exactly three exemptions today (_test.go, internal/controller revive, gameproto minecraft.go G115); no inline directives exist anywhere. |
| What about build-tag-conditional files? | Build tags are mandatory for api and test/e2e, and CI must pass them explicitly. | **Decision 6** below; 7 api files carry `//go:build envtest`; 51 of 79 test/e2e files carry `//go:build e2e`; CI action args: `--build-tags=envtest` for api, `--build-tags=e2e` for test/e2e. |
| What surfaces cannot be changed, even to fix findings? | Frozen surfaces are refactored around via extraction/wrapping, never renamed. | **Decision 7** below; audit field names, migration structure, e2e bucket mapping, protocol byte layouts, rate-limit thresholds, Prometheus metric names are production contracts. |

---

## Decision 1: Salvage Strategy for PR #216 — PARTIAL SALVAGE

**Statement**: The branch `chore/lint-backlog-wave2` has 11 commits split into three logical parts: (a) CI enablement, (b) api fix commit ba32d0b fixing 68 api files across 13 packages, (c) agent fix commit f5b9ede fixing ~45 agent files across 17 packages, (d) test/e2e fix commit e7b99b6 fixing ~51 test/e2e files. Commits (b) and (c) have zero overlap with changes master has made since the fork (ba32d0b changes 68 api files; master has changed 0 api files and 0 agent files since the fork, so clean rebase). Commit (d) is stale against feature 001 (the game protocol infrastructure that landed after the fork), which rewrote 108 test/e2e files (54 new, 4,279 insertions). Decision: keep and rebase commits (b) and (c) onto master; redo commit (d) against the current test/e2e structure.

**Rationale**:

The cost of full redo (throwing away api and agent work) is ~336 findings × 2 modules = 672 LOC rewritten for no benefit, since the work is already clean and conflict-free. The branch's api fix commit ba32d0b touches 68 api files; master has changed 0 api files since the fork, so a clean rebase applies with zero conflicts. The test/e2e situation is distinct: feature 001 (game protocol abstractions) restructured that directory (108 files changed, 54 new, 4,279 insertions), and 51 of the old e2e fixes are moot (the files they touched either no longer exist in their original form or have been refactored).

The rebase path for (b) and (c) is straightforward because each fix commit is self-contained (no inter-module dependencies between api and agent fixes) and the changes they make (adding context parameters, improving error handling, renaming locals to avoid shadowing) are mechanical and unlikely to collide with master's own changes in those modules.

For (d), the redo cost is lower than the salvage cost because: (1) test/e2e's git history from feature 001 and the branch diverge structurally (file paths, package layout changed), so a three-way merge is error-prone; (2) the branch's findings predate the new structure, so the linter will re-enumerate anyway; (3) a fresh run against current code will catch findings the old fixes didn't anticipate.

**Alternatives considered and rejected**:

1. **Full salvage (rebase all three commits)** — rejected because the underlying code structure changed. While a `git merge-tree` produces no textual conflicts, the branch's test/e2e fixes were written against code that has since been restructured, and re-linting the resolved code would need to happen anyway. The branch's test/e2e fixes target an outdated code shape, so a fresh lint run against current code is more efficient than salvage and re-validation.

2. **Full redo (discard entire branch)** — rejected because api and agent fix commits (ba32d0b, f5b9ede) are conflict-free and internally sound. Redoing them costs ~336 findings' worth of work that is already complete and clean. The api fix alone touches 68 files; the agent fix touches 18 files. That work is salvageable and should be kept.

3. **Cherry-pick individual files from the branch** — rejected because each commit spans multiple packages and relies on internal consistency within its scope (e.g., api commit ba32d0b refactors registry, telemetry, ws, and scope packages together; splitting by file breaks that cohesion). Commit-level salvage is more reliable than file-level.

**Evidence from codebase**:

- `chore/lint-backlog-wave2` branch log (lines 1-11 of the log output above): ci enablement commit 9967218, then ba32d0b (api), f5b9ede (agent), e7b99b6 (test/e2e), then later 3 more fix rounds and a compile-error repair.
- Merge-base: `git merge-base chore/lint-backlog-wave2 master` = b3d5b38; this is the fork point where the branch diverged from master.
- API file count: ba32d0b changes 68 api files; master has changed 0 api files since the fork b3d5b38, so a clean rebase applies with zero textual conflicts.
- Agent file count: f5b9ede changes 18 files; master has changed 0 agent files since the fork b3d5b38, so a clean rebase applies.
- Test/e2e file count: e7b99b6 fixes 51 files; feature 001 (merged 2026-08-16) changed 108 files total (54 new, 4,279 insertions), restructuring the directory layout. CI.yaml line 177: comment says "api, agent, and test/e2e deliberately excluded pending lint backlog cleanup (wave 2)."

---

## Decision 2: The Starting Finding Count Is Measured by CI, Not Asserted

**Statement**: The initial finding count (~488 across api 188, agent 148, test/e2e 152) cannot be verified locally per Constitution VI (no local build/test/lint execution). The measurement comes from a CI run on the enumeration commit (the commit that adds the three modules to the lint matrix with no fixes yet applied). That CI run's reported number is the baseline; no prior count is trusted. The feature's completion condition (SC-003) is "zero findings," not "N findings fixed," to decouple success from the measured starting count.

**Rationale**:

A stale enumeration from a branch (the 488 and later 91 from run 31971653399) is not authoritative because: (1) the branch may be out of sync with master, and the true current count on master is unknown; (2) run 31971653399 ran AFTER the branch's fix commits, so 91 is a residual count, not the initial count; (3) constitutionally, linter runs happen in CI, not locally.

The only trustworthy measurement is: (a) enumerate on current master with the three modules added to the matrix and no fixes applied, (b) record that CI result as the baseline, (c) apply fixes and measure reduction over that baseline. This makes the starting count a CI artifact, not an assertion. If a maintainer later asks "how many findings were fixed?" the answer is recoverable from CI logs, not from stale notes.

**Alternatives considered and rejected**:

1. **Assert a specific number based on prior runs** — rejected because the branch enumeration and current master may differ (new code = new findings; refactored code = different findings). Without running CI on today's master, the actual count is unknowable.

2. **Run the linter locally to get a number** — rejected because Constitution VI forbids this. The project's no-local-execution rule makes CI the only authoritative source. A local run on a developer's machine can use different Go versions, linter versions, or dependency versions, and produces an unverified result.

3. **Use the stale 488 or residual 91 as the target** — rejected because 91 is measured AFTER fixes, so it's a residual not a baseline. The 488 is from an old run on a different commit state. Neither is trustworthy for a completion metric.

**Evidence from codebase**:

- Constitution VI (in CLAUDE.md, rule 8): "Do NOT run the test or lint suites locally — CI is the source of truth."
- Spec SC-003 (spec.md line 229-232): "A full golangci-lint run over `api`, `agent`, and `test/e2e`... reports zero findings. The count of findings outstanding at the start of the work is incidental; the completion condition is an empty result, not a number reduced."
- CI.yaml line 177-180: matrix currently excludes api, agent, test/e2e with comment "deliberately excluded pending lint backlog cleanup."
- Plan §2 (plan.md, implied): enumeration is a separate step from fixing, and enumeration should happen on the enumeration commit before any fixes.

---

## Decision 3: Sequencing — Fixes Before Matrix Enablement (On Branch), Then Enable at Merge

**Statement**: The working branch will contain: (1) an early enumeration commit that adds the three modules to the CI matrix with NO fixes, so CI reports the baseline finding count; (2) logical fix commits partitioned by module+package, applying fixes and addressing findings; (3) a final merge commit that brings the three modules into the lint matrix on master. Intermediate CI runs on the branch will be RED (because findings exist), which is expected and acceptable for measurement. The merge to master happens only when the tree is GREEN (all findings fixed). This balances the tension between "measure first" (needs enumeration to be early) and "only push green" (never push broken code to master).

**Rationale**:

Wave 1 (PR #215, merged at 16c2625) established this pattern: merge commit ab49814 added the first 8 modules to the CI matrix, and that commit came AFTER all their fixes. But the plan for wave 2 must account for an operational reality: during the work, CI needs to report baseline findings so the team knows how many to fix. If the enumeration happens only at the end (when the matrix is enabled on master), the in-progress measurement is invisible — the working branch has no CI signal until the very last commit.

Solution: put the enumeration commit EARLY on the branch (where red CI is acceptable because the branch is work-in-progress), apply fixes, and then enable on master (when CI is green). This gives the team measurement mid-work and keeps master green always.

The enumeration commit that adds the matrix entries but applies no fixes is a deliberate "red CI is OK here" moment — it's not a gate violation, it's the measurement step. Once findings are fixed, CI goes green, and the merge happens.

**Alternatives considered and rejected**:

1. **Enabling on master first, then fixing** — rejected because it turns master red (violates the never-push-broken rule and would fail any branch protection). A red master is worse than a red working branch.

2. **Measuring blind and enabling at the very end** — rejected because the team works blind during the fix phase. No one knows if they're 10% done or 90% done with findings. Measurement mid-work is operationally valuable.

3. **Splitting enumeration and enabling into two separate PRs** — rejected because it fragments the work. A single PR (or stacked series) that enumerates, fixes, and then enables is clearer and easier to review as a logical unit.

**Evidence from codebase**:

- Wave 1 pattern: PR #215 (merge 16c2625) added 8 modules to CI; the enabling commit ab49814 is one of 21 commits in that PR. The fixes came before enablement on the merge.
- CI.yaml line 177: current matrix excludes api, agent, test/e2e with no build-tag configs yet defined. The decision requires adding them with explicit tags.
- Spec (lines 267-269): "Bringing a module under the gate is a one-time event per module and is not reversible (once a module is gated, it stays gated per FR-003 and FR-004)." This means the matrix change is not reverted; once enabled, it stays.

---

## Decision 4: Partition Work by Package Directory to Avoid File Conflicts

**Statement**: Parallel workers are assigned by package directory, so no two workers modify the same file. API is split into 13 packages (handlers dominates at 84 files; then registry 24, auth 22, ws 12, db 11, kube 11, notify 11, audit 5, rbac 5, cmd 5, scope 4, telemetry 3, httperr 2), so 13 sequential or parallel workers, one per package. Agent is split into 17 packages (rcon dominates at 14 files; then mods 6, logs 3, console 2, files 2, players 2, usage 2, quiesce 1, auth 1, heartbeat 1, rcon-protocols 1, ssh-auth 1, etc.), so 17 workers. Test/e2e has 23 packages but is monolithic at 51 root-level `*_test.go` files, limiting parallelism — those 51 files can be partitioned by game or by test suite, but true parallel scaling is harder. This is documented honestly, not worked around with hasty refactoring.

**Rationale**:

Partitioning by linter (all errcheck findings in one worker, all govet findings in another) would result in multiple workers touching the same file from different linters, causing merge conflicts and requiring sequential resolution. Partitioning by package avoids file overlap: each package's code is touched by exactly one worker, and that worker applies all applicable linter fixes to that package. This scales to the number of packages, not the number of linters or findings.

File-level partitioning (e.g., worker A fixes a.go, worker B fixes b.go) is fine in principle but breaks the contract of logical code review: related changes within a package (e.g., adding context parameters to a cluster of functions in registry.go) should be reviewed together, not split across two PRs or two workers.

Test/e2e's case is special: the 51 root-level test files are not in any package subdirectory; they are in the root `test/e2e/` directory. Partitioning them requires either (a) moving test files into subdirectories by game (introduces file-path changes), (b) assigning them to one worker (bottleneck), or (c) accepting some sequential work. Option (c) is honest and documented, not worked around.

**Alternatives considered and rejected**:

1. **Partition by linter** — rejected because multiple linters can flag the same file (e.g., errcheck and contextcheck both flag a function in ws/dialer.go). Workers would conflict on the same file, requiring sequential merging and review.

2. **Partition by finding count** — rejected because findings cross package boundaries (e.g., a single function might have 3 errcheck findings and 2 contextcheck findings; splitting by finding type splits the function's fix across two workers and two reviews).

3. **Fully parallelize test/e2e without structural change** — rejected because 51 root files cannot be safely partitioned without renaming or moving them. That would introduce structural changes to a test-critical directory, adding risk to the fix work.

**Evidence from codebase**:

- API package structure: `api/internal/handlers` (84 .go files), `api/internal/registry` (24 files), `api/internal/auth` (22 files), `api/internal/ws` (12 files), `api/internal/db` (11 files), `api/internal/kube` (11 files), `api/internal/notify` (11 files), others (10 files). Full list from `ls -R api/internal/` shows 13 subdirectories.
- Agent package structure: `agent/internal/rcon` (14 files), `agent/internal/mods` (6 files), others (44 files across 15 packages). Full breakdown: `ls -R agent/internal/` shows 17 directories.
- Test/e2e monolithic structure: 51 `*_test.go` files at root `test/e2e/`, plus 23 package subdirectories under `test/e2e/internal/`. The root-level tests cannot be split by package membership; they are the root package.

---

## Decision 5: No New Suppression Directives; the Zero-Suppression Property Is Preserved

**Statement**: The `.golangci.yml` configuration today has exactly three exemptions: (1) `_test.go` files are exempt from errcheck, gosec, and unparam (because test setup often ignores errors deliberately); (2) `internal/controller/` is exempt from revive's `exported:` rule (because reconciler builders have a lot of repeated patterns); (3) `gameproto/minecraft.go` is exempt from gosec's G115 (because VarInt encoding requires lossless reinterpretation between uint32 and int32, with explicit overflow checks in code). No new `//nolint`, `// nosec`, `// lint:ignore`, or equivalent suppression directives will be introduced during wave 2 work. The zero-suppression property — no inline directives in source code — is preserved. Every finding is fixed with real code changes.

**Rationale**:

Suppression directives are technical debt masquerading as simplification. Once a codebase permits them, they accumulate: "I'll add a linter suppression" becomes easier than "I'll refactor to fix this," and suddenly the linter is no longer trusted — "this rule has too many false positives" is the usual defense. Gameplane chose a different path: linters exist to find real problems, and real problems get fixed via code changes (adding context, improving error handling, renaming shadowed variables, etc.).

Wave 1 cleared 582 findings across 8 modules and added exactly one new suppression (the G115 exclusion in gameproto's minecraft.go). This is the authoritative precedent: when a finding is real but unavoidable (like VarInt's two's complement reinterpretation), the suppression is documented with a comment (lines 47-52 in .golangci.yml) explaining why the finding is safe and what explicit checking is in place. No other exceptions exist.

Wave 2 must follow the same discipline. If a finding is a genuine false positive (e.g., a linter flags something that is actually safe), the finding gets raised with the maintainer and reviewed with the same rigor as the Minecraft G115 case — not silenced inline.

**Alternatives considered and rejected**:

1. **Allowing `//nolint` for "obviously false" findings** — rejected because "obvious" is subjective, and each suppression is a debt token. What seems obvious today (a variable that looks unused but is actually used by reflection) is a trap for future refactors. The project's experience is that suppressing a finding and moving on costs more in the long run than fixing it.

2. **Broadening the `_test.go` exemption to cover test/e2e** — rejected because test/e2e is entirely tests, and broadening the exemption would gut the gate for that module. The whole point is to bring test/e2e under the gate; exempting it defeats that. The existing `_test.go` exemption is fine because ordinary unit tests can deliberately ignore errors in setup; e2e tests should not.

3. **Adding per-module config files** — rejected because it fragments the policy. A single global `.golangci.yml` is easier to audit and maintain than 13 module-local configs. Exceptions should be rare and centralized.

**Evidence from codebase**:

- `.golangci.yml` lines 35-52: the three exemptions (only). No other directives, no per-module configs.
- `.golangci.yml` lines 47-52: Minecraft G115 exclusion with a detailed comment explaining the two's complement reinterpretation and why explicit checks are in place.
- Spec FR-002 (spec.md line 162-165): "Every linting finding reported by golangci-lint in `api`, `agent`, and `test/e2e` MUST be resolved via code changes... Suppression directives (`//nolint`, `//#nosec`, etc.) MUST NOT be introduced in these three modules."
- Spec FR-005 (spec.md line 172-175): "The feature MUST preserve the existing zero-suppression property... no new suppression directives are introduced at any point during Wave 2 work. The single authorized gosec G115 exclusion... remains the only exception."

---

## Decision 6: Build Tags Are Mandatory for api and test/e2e; CI Must Pass Them Explicitly

**Statement**: API contains 7 files with `//go:build envtest`, and test/e2e contains 51 of 79 files with `//go:build e2e`. Without the build tag, these files are invisible to `go vet` and to `golangci-lint` (both tools skip tagged files by default unless the tag is passed). The CI configuration MUST explicitly pass `--build-tags=envtest` when linting the `api` module and `--build-tags=e2e` when linting the `test/e2e` module. This is non-negotiable so tag-gated call sites do not become a hiding place for broken code.

**Rationale**:

A finding in an envtest-tagged file will never be caught by a lint run that doesn't pass the tag. This creates a risk: code that is only compiled into integration tests (or e2e-only paths) can have issues that are invisible to ordinary `go build ./...` or `go vet ./...` commands. If the CI configuration doesn't pass the tags, the gate is incomplete and useless.

Wave 1's `operator` module already passes `--build-tags=envtest` in the lint job (CI.yaml line 195), so the precedent exists. The same pattern applies to `api` and `test/e2e`.

The alternative of NOT linting tagged files is explicitly forbidden by spec FR-007: "The gate MUST NOT be satisfied by removing code from analysis... every source file that compiles into `api`, `agent`, or `test/e2e` under the tags CI passes MUST be analysed."

**Alternatives considered and rejected**:

1. **Linting without build tags** — rejected because 51 of 79 test/e2e files and 7 api files are then silently skipped. The gate window is incomplete and unverified code ships.

2. **Moving tagged files into untagged locations** — rejected because it changes the code structure and introduces unnecessary refactoring risk. The tags exist for a reason (to segregate integration tests from unit tests, to isolate envtest dependencies). Removing them makes the code harder to understand, not simpler.

3. **Accepting that tagged code is "trusted" without linting** — rejected because there is no good reason to trust tagged code differently. A bug in an envtest file is still a bug; a security issue in an e2e file is still a security issue. The linter should run over all code paths that end up in the binary.

**Evidence from codebase**:

- API envtest files: `ls api/internal/handlers/*_envtest_test.go` shows 3 files (modules, resources_cluster, dispatch_isolation), and a search finds 7 total across api.
- Test/e2e tagged files: `grep -l "//go:build e2e" test/e2e/*.go | wc -l` shows 51 files.
- CI.yaml line 189-195: operator lint job already passes `--build-tags=envtest` as a precedent.
- Spec FR-006 (spec.md line 176-188): "Frozen surfaces... MUST remain unchanged... Build-tag-conditional code... files behind `//go:build envtest`... MUST be analyzed by golangci-lint when the corresponding build tag is passed."
- Spec FR-007 (spec.md line 184-188): "The gate MUST NOT be satisfied by removing code from analysis... Every source file that compiles into `api`, `agent`, or `test/e2e` under the tags CI passes MUST be analysed."

---

## Decision 7: Frozen Surfaces Are Refactored Around, Never Renamed

**Statement**: The following data structures, API contracts, and constants are part of the production observable interface and cannot be changed without breaking existing deployments, monitoring, or tests: (1) audit event field names exported by the API (`api/internal/audit/audit.go`, fields like `Action`, `UserID`, etc.); (2) append-only database migration files (`api/internal/db/migrations/001_init.sql` through `006_share_links.sql`); (3) e2e test names as mapped in `test/e2e/buckets.sh` (the CI coverage verifier depends on exact name strings); (4) reverse-engineered game protocol byte layouts (`gameproto/minecraft.go`, `gameproto/terraria.go`, `test/e2e/internal/*/proto/*` files); (5) rate-limit thresholds in `api/internal/auth/ratelimit.go` (e2e login budget depends on these); (6) Prometheus metric names and labels (e.g., `gameplane_audit_webhook_events_total` from `api/internal/audit/audit.go`). When a linting finding requires touching one of these frozen surfaces, the fix is applied via extraction or wrapping (e.g., creating a helper function) rather than renaming the frozen interface itself.

**Rationale**:

Frozen surfaces are boundaries between this codebase and the outside world. Renaming an audit field breaks external systems that parse audit logs. Renaming a migration file breaks the append-only contract and database consistency. Renaming an e2e test breaks the CI coverage verifier. Renaming a Prometheus metric breaks monitoring dashboards and alerts.

A linting finding that "suggests" renaming a variable can be addressed by extracting that variable into a helper function or wrapping it in a type alias, rather than renaming it in place. This is more work but preserves the contract.

The linters (revive for `var-naming`, gosec for `G101` "hardcoded credentials") sometimes push toward renaming identifiers to follow conventions or avoid perceived security issues. These findings are real but must be deflected away from frozen surfaces.

**Alternatives considered and rejected**:

1. **Renaming the frozen surface and updating all references** — rejected because "all references" extends beyond the codebase (external audit consumers, Prometheus scrapers, the bucket coverage script). The coupling is not just internal; it is external.

2. **Splitting the frozen surface into an internal name and an external name** — rejected because this introduces a two-name problem (which is correct? which is canonical?). The existing name is already locked in by external consumers; renaming it is just a rename, not a fix.

3. **Creating a new function with a better-named parameter and deprecating the old one** — rejected as unnecessary boilerplate if the current name is working. Refactoring around the name (e.g., extracting to a const or a named type) is simpler.

**Evidence from codebase**:

- Audit field names: `api/internal/audit/audit.go` defines an `Event` struct (not shown in excerpt but referenced by metric line 40 "result" label). The field names are part of the audit event JSON and are versioned as part of the database schema.
- Migration files: `api/internal/db/migrations/` contains 001 through 006, each adding tables or columns (e.g., 006_share_links.sql). Renaming or reordering these files breaks the migration sequence.
- E2e test names: `test/e2e/buckets.sh` maps test names by exact string (lines 143-157 list games like `TestGameServer_MinecraftJavaBot_Joined`); the coverage verifier (not yet implemented but specified) will search for these exact names.
- Protocol byte layouts: `gameproto/minecraft.go` defines VarInt encoding (constants and functions like `readMinecraftVarIntWithCapture`); changing these breaks the Minecraft handshake parser.
- Rate-limit thresholds: `api/internal/auth/ratelimit.go` line 22-23 defines `rate` and `burst` fields in the TokenBucket struct. These thresholds are baked into e2e tests (CI login budget depends on them).
- Prometheus metric name: `api/internal/audit/audit.go` line 37-40 defines `gameplane_audit_webhook_events_total` with label `result`. Renaming the metric breaks any Prometheus query or alert that scrapes this metric.

---

## Known Traps Carried Forward from Wave 1

The following gotchas emerged during wave 1 and are documented here so wave 2 avoids them:

1. **net.DialTimeout → DialContext silently drops the timeout unless Dialer.Timeout is set.** When refactoring a dial call from `net.DialTimeout(...)` to `net.Dialer{}.DialContext(...)`, ensure the timeout is mirrored into `Dialer.Timeout`; the Timeout field is NOT deprecated but is NOT automatic when DialContext is used. Check wave 1's websocket/rcon refactors for examples.

2. **contextcheck on shutdown paths requires explicit WithTimeout wrapping.** A handler that calls `<-ctx.Done()` (blocking on context cancellation) will fail contextcheck if ctx has no timeout. The fix is `context.WithTimeout(context.WithoutCancel(ctx), d)` — note the `WithoutCancel` to preserve cancellation semantics while adding a deadline. This is subtle and easy to miss.

3. **gosec G101 renames must dodge every substring in the password/secret/token wordlist.** The regex is `(?i)passwd|pass|password|pwd|secret|token|pw|apiKey|bearer|cred`. A variable declared as `s` can be renamed to `secret` (sounds safe) but will match "secret" in the regex and redline. Every rename MUST be validated against this regex, and every reference must be updated together (declaration + all call sites in the same commit, else intermediate commits fail linting).

4. **Naive nilerr fixes can turn deliberate fallbacks into hard failures.** The nilerr linter flags `if err != nil { return nil, err }` but sometimes an error is genuinely non-fatal and a nil return is intentional. Before removing the nil return, verify the caller's contract: does it expect nil or an error? Changing nil to err can break callers that rely on the nil.

5. **Signature changes in functions behind //go:build envtest and //go:build e2e are invisible to ordinary tooling.** A function signature change (e.g., adding a parameter) in a tagged file will not be caught by `go vet ./...` (which skips tagged files by default). Ordinary tooling cannot verify these breakages — only tag-scoped CI builds can. Ensure CI's build-tag args (--build-tags=envtest for api, --build-tags=e2e for test/e2e) catch these changes; verify failures via CI logs, not local execution.

---

## Summary

These seven decisions establish the infrastructure and sequencing for wave 2. Each decision is backed by evidence from files already in the codebase and requirements already in the spec. No decision contradicts Constitution IV, the project's coding standards, or the zero-suppression property. All seven support the spec's mandatory requirements (FR-001 through FR-007, SC-001 through SC-005).

The next phase (Phase 1) will translate these decisions into concrete implementation tasks: creating the CI matrix entries, partitioning worker assignments, and preparing the branch structure for fix commits.
