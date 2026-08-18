# Feature Specification: Lint Backlog Wave 2 — Static Analysis Gate for api, agent, test/e2e

**Feature Branch**: `004-lint-backlog-wave2`

**Created**: 2026-08-17

**Status**: Draft

**Input**: User description: "do spec 004 for https://github.com/ValgulNecron/Gameplane/pull/216 this will be done BEFORE 002 and 003"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Three modules brought under the uniform lint gate (Priority: P1)

Today three of the project's largest and most security-sensitive Go modules—the public
API gateway (`api`), the in-pod agent (`agent`), and the E2E test suite (`test/e2e`)—are
exempt from the static-analysis gate that every other module in `go.work` must pass.
This creates an unequal quality floor: defects in exactly the code that handles
authentication, RCON, file access, network I/O, and cluster scripting go uncaught by
linters, while smaller, less critical modules are held to a stricter standard. A
developer working on any other module knows their code has passed 14 linters; a developer
working on the API, agent, or E2E suite has no such guarantee. The feature brings all
three under the same gate, so the standard applies uniformly across the workspace.

**Why this priority**: Static analysis is a first-class gate in Gameplane's CI. Skipping
it in three of the largest modules undermines the entire premise: either lint coverage
matters or it does not. If it matters, it matters for the authentication server, the
in-pod binary, and the test suite equally. This is not a nice-to-have; it is correctness
infrastructure.

**Independent Test**: Can be tested independently by verifying that the CI matrix
includes `api`, `agent`, and `test/e2e` in the golangci-lint job with the correct build
tags (`--build-tags=envtest` for api, `--build-tags=e2e` for test/e2e), and that a
fresh `git push` to any branch causes CI to fail if any of these three modules has a
golangci-lint finding that was not present in the base branch.

**Acceptance Scenarios**:

1. **Given** the current `.golangci.yml` config and the 14 enabled linters, **When**
   golangci-lint runs against `api` with `--build-tags=envtest`, **Then** it reports
   all findings that exist in the module (no exemptions by build tag, and envtest-tagged
   files are actually analyzed).
2. **Given** the same setup for `agent` and `test/e2e` (with `--build-tags=e2e` for the
   latter), **When** CI runs, **Then** all three modules are included in the lint check,
   and CI fails if findings exist and have not been fixed.
3. **Given** a developer's code change that introduces a new linting finding in `api`,
   `agent`, or `test/e2e`, **When** that change is pushed, **Then** the same CI job
   that would reject a finding in `operator` also rejects it in these three modules.

---

### User Story 2 - Findings are fixed, not suppressed (Priority: P2)

Every linting finding in the tree today is fixed with real code changes: there are no
`//nolint`, `// nosec`, `// eslint-disable-next-line`, or equivalent suppression
directives anywhere in the project (with one authorized exception: a single gosec G115
exclusion in the Minecraft VarInt codec, documented and reviewed). This zero-suppression
property is a quality signal—it means linters are trusted and findings are addressable.
When the three modules are brought under the gate, this property must be preserved: the
~488 findings across `api`, `agent`, and `test/e2e` will be resolved by fixing code, not
by adding directives. No suppression directive can be introduced to clear the gate.

**Why this priority**: Suppression directives are technical debt masquerading as
simplification. Once a codebase permits them, they accumulate: "I'll fix this linter
warning later" becomes "this is a known false positive" becomes "why are we running this
linter at all?" Gameplane chose a different path: linters exist to find real problems, and
real problems get fixed. The zero-suppression property is both a constraint and a promise.
Bringing three modules under the gate only preserves that promise if the resolution method
is the same as it has been for every other module.

**Independent Test**: Can be tested independently by searching the tree for all
suppression-directive patterns (`//nolint:`, `//#nosec`, `//`, `:ignore`, etc.) across
all `.go` files, excluding the one authorized G115 exclusion, and confirming zero matches.
This test MUST be repeatable: the pattern search can be run before, during, and after the
feature work, and the count must never increase.

**Acceptance Scenarios**:

1. **Given** the completed work on `api`, `agent`, and `test/e2e`, **When** a search for
   linting suppression directives runs across the workspace, **Then** zero directives are
   found outside the single authorized G115 exclusion in the Minecraft codec.
2. **Given** a code reviewer examining a merged PR that resolves Wave 2 findings, **When**
   they read the diff, **Then** they see only code fixes (added context parameters,
   improved error handling, variable renames to avoid collisions, etc.), never
   suppression directives added as a shortcut.
3. **Given** the same zero-suppression check at release time, **When** it runs, **Then**
   it reports the same result: zero suppression directives outside the one Minecraft
   exclusion. (This scenario is really a regression check: the property is maintained,
   not just temporarily achieved.)

---

### User Story 3 - The gate cannot silently regress (Priority: P3)

A module can be excluded from CI review by mistake, oversight, or future cost-cutting, and
no one notices until weeks later when findings silently accumulate. There must be a way
for a maintainer to inspect the current state—which modules are gated, which are not, why—
without reading CI logs or historical comments. The CI configuration itself, and the logic
that maintains it, must make regressions visible.

**Why this priority**: This is a lower priority than actually fixing findings or preserving
the zero-suppression property, but it is necessary so that future exclusions are deliberate
and reviewable rather than accidents. It also prevents the pattern of "api was accidentally
dropped from the matrix in PR #xyz and no one noticed until #abc."

**Independent Test**: Can be tested independently by confirming that a maintainer reading
`.golangci.yml` (or a clear comment that cross-references it) can immediately identify which
Go modules are subject to the lint gate and which, if any, are excluded and why. Any exclusion
must be justified and must be something that, if removed, makes the configuration invalid (so
removing the exclusion is a deliberate choice with visible consequences).

**Acceptance Scenarios**:

1. **Given** a maintainer inspecting `.golangci.yml` and any CI configuration files, **When**
   they search for the list of linted modules, **Then** they find a clear, maintained list or
   matrix that includes all modules in the `go.work` workspace, organized in a way that makes
   omissions obvious (e.g., an explicit module matrix in CI YAML).
2. **Given** a hypothetical future scenario where a module is incorrectly dropped from the
   matrix, **When** CI runs without that module being checked, **Then** the configuration
   itself fails a validation check (e.g., a script that verifies all `go.work` members are
   covered), or it is visually obvious in the matrix that a module is missing.
3. **Given** a code review of any change to the CI configuration or `.golangci.yml` that
   affects which modules are gated, **When** the change is merged, **Then** the PR diff
   clearly shows which modules are being added, removed, or exempted, so the human reviewer
   can verify the change is intentional.

---

### Edge Cases

- What happens when a finding is truly a false positive? The authorized-exception process
  (which has produced exactly one exception in the Minecraft codec to date) applies: the
  finding must be reviewed and documented at a scope level (per-file, per-function, or
  similar) rather than globally ignored. The single authorized G115 exclusion serves as a
  model; new exceptions follow the same rigor, or they are fixed as real code changes.
- How does the feature handle build-tag-conditional compilation? Files behind `//go:build
  envtest` (for `api`) or `//go:build e2e` (for `test/e2e`) must be analyzed by the linter
  when the corresponding build tag is passed; this is non-negotiable so tag-gated call sites
  do not become a hiding place for broken code. The CI configuration must explicitly pass
  `--build-tags=envtest` for `api` and `--build-tags=e2e` for `test/e2e`.
- What if a finding appears only on some platforms or Go versions? The CI matrix runs on
  Linux with Go 1.26 (current project version); findings detected there are considered
  actionable. If a finding manifests only under conditions not tested in CI (e.g., Windows,
  Go 1.27+), that is a gap for future discovery; this feature assumes a single platform and
  Go version, per project convention.
- What about third-party dependencies that have unfixed linting issues? Dependencies are
  managed as ordinary Go modules; findings originating in dependency code, rather than
  workspace code, are out of scope for this feature. If a dependency has issues, upgrading or
  replacing it is a separate concern from Wave 2.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The CI golangci-lint job MUST include `api`, `agent`, and `test/e2e` in its
  module matrix, with the correct build tags (`--build-tags=envtest` for `api`,
  `--build-tags=e2e` for `test/e2e`), so that tag-gated source files are actually analyzed.
- **FR-002**: Every linting finding reported by golangci-lint in `api`, `agent`, and
  `test/e2e` MUST be resolved via code changes that fix the underlying issue (improved error
  handling, added context parameters, variable renames, refactoring for clarity, etc.), not
  via suppression directives. Suppression directives (`//nolint`, `//#nosec`, `//lint:ignore`,
  or equivalent) MUST NOT be introduced anywhere in the tree — this remains absolute and
  admits no exception. Config-level, scoped exclusions in `.golangci.yml` (narrow path
  pattern, single linter/rule, commented and maintainer-authorized) are a separate mechanism
  and remain permitted; several are authorized as of this work (see
  `contracts/exclusion-policy.md` for the current inventory), none of them inline.
- **FR-003**: Once `api`, `agent`, and `test/e2e` are brought into the lint gate, any future
  finding in those modules that is introduced by a new commit MUST cause CI to fail on the
  branch that introduced it, just as findings in other modules do.
- **FR-004**: The CI configuration that defines which modules are subject to the lint gate
  MUST be explicit and maintainable (e.g., a clear module matrix or list), so that any
  accidental omission of a module from the gate is detectable by code review and/or
  configuration validation.
- **FR-005**: The feature MUST preserve the existing zero-inline-suppression property of the
  tree: no new in-source suppression directives (`//nolint`, `//#nosec`, `//lint:ignore`, or
  equivalent) are introduced at any point during Wave 2 work, and none exist anywhere in the
  tree at completion. This is distinct from config-level exclusions in `.golangci.yml`, which
  are narrow, per-file, documented, and maintainer-authorized; several such exclusions were
  authorized during Wave 2 work (see `contracts/exclusion-policy.md`), and adding them does
  not weaken the inline-suppression prohibition. (prohibitive corollary to FR-002; keep the
  two in sync)
- **FR-006**: Frozen surfaces (audit field names, chained-hash business logic,
  `api/internal/db/migrations/` append-only structure, e2e test names in
  `test/e2e/buckets.sh`, reverse-engineered game protocol byte layouts, rate-limit
  thresholds, Prometheus metric names and labels) MUST remain unchanged during Wave 2 work,
  so that existing production systems and monitoring do not break. If a finding requires
  touching one of these frozen surfaces, the finding MAY be fixed via refactoring outside
  the frozen API (e.g., extracting logic to a helper function rather than renaming an
  exported field).
- **FR-007**: The gate MUST NOT be satisfied by removing code from analysis. Findings MUST
  NOT be cleared by moving code behind a build tag that CI does not pass, by deleting or
  renaming files to evade a linter, or by narrowing the CI invocation's package scope. Every
  source file that compiles into `api`, `agent`, or `test/e2e` under the tags CI passes MUST
  be analysed.

### Key Entities

- **Lint Gate / Static-Analysis Gate**: The CI job (golangci-lint with v2 config) that runs
  against all modules in the `go.work` workspace and fails CI if findings exist. All modules
  MUST be equally subject to the gate; no module may be exempted without explicit discussion.
- **Linting Finding**: A single reported issue from any of the 14 enabled linters (bodyclose,
  errcheck, gosec, govet, ineffassign, staticcheck, unused, misspell, revive, unparam,
  nilerr, noctx, errorlint, contextcheck) that indicates a code quality, correctness, or
  security concern.
- **Suppression Directive**: A source code annotation (`//nolint:`, `// nosec`, etc.) that
  tells a linter to skip a finding on a specific line or block. The project policy is
  zero in-source suppression directives, full stop — no exception. This is distinct from
  the Authorized-Exclusion List below, which operates at the config level, not in source.
- **Authorized-Exclusion List**: The set of linting findings that have been explicitly
  reviewed and accepted as necessary or acceptable, each as a narrow, per-file, documented,
  maintainer-authorized rule in `.golangci.yml` (never an in-source directive). As of this
  work it contains eight items across six gosec rules and two other linter scopes — the
  original gosec G115 finding in the Minecraft VarInt codec plus gosec G302 (agent mod
  extraction), G704 (api ws dialer), G124 (api CSRF cookie), G204 (test/e2e kubectl helper),
  and G402 (test/e2e Satisfactory probe), alongside the pre-existing `_test.go` and
  `internal/controller/` scoped exclusions (documented in `contracts/exclusion-policy.md`).
  Future additions follow the same rigor as the existing ones.
- **Build-Tag-Conditional Code**: Source files or blocks gated by `//go:build` directives
  (e.g., `//go:build envtest` for `api/internal/handlers/*_envtest_test.go`,
  `//go:build e2e` for `test/e2e/**/*_test.go`). These files MUST be analyzed by golangci-lint
  when the corresponding build tag is passed.
- **Frozen Surface**: A data structure, field name, API contract, or behavioral constant that
  is part of the production system's observable interface and cannot be changed without
  breaking existing deployments, tests, or monitoring. Examples: audit event field names
  (exported by the API), migration file names and structure (append-only), e2e test names
  (mapped by bucket script), protocol byte layouts (shared with game servers), rate-limit
  thresholds (exposed in pod logs), metric names and labels (scraped by Prometheus).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of modules in the `go.work` workspace (13 members: the 10 already gated
  plus the three brought in by this feature) are included in the CI golangci-lint job and are
  checked on every push to a branch and on every merge to main. No module is exempted,
  excluded, or listed as "pending future cleanup" in the CI configuration.
- **SC-002**: Zero in-source suppression directives (`//nolint`, `//#nosec`, `//lint:ignore`,
  or equivalent) exist anywhere in the tree, verified before, during, and after Wave 2 work.
  This is a regression test: the zero-inline-suppression property is maintained, not just
  temporarily achieved. It is independent of `.golangci.yml`'s config-level exclusion count
  (several narrow, documented, maintainer-authorized exclusions exist there; see
  `contracts/exclusion-policy.md`) — those are a config-level mechanism, not in-source
  directives, and do not count against this criterion.
- **SC-003**: A full golangci-lint run over `api`, `agent`, and `test/e2e`, using the same
  configuration and build tags CI passes, reports zero findings. The count of findings
  outstanding at the start of the work is incidental; the completion condition is an empty
  result, not a number reduced.
- **SC-004**: A code reviewer reading the CI configuration (`.golangci.yml`, GitHub Actions
  YAML, or similar) can identify all linted modules and any exemptions within 2 minutes of
  opening the file. The configuration does not require external documentation or historical
  knowledge to understand what is and is not gated.
- **SC-005**: Frozen surfaces (audit fields, migrations, e2e test names, protocol layouts,
  thresholds, metric names) remain semantically identical before and after Wave 2 work. They
  may be refactored internally or wrapped by helpers, but their external observable behavior
  and names do not change.

## Assumptions

- This feature is sequenced BEFORE features 002 and 003 at the user's explicit direction.
  The user's stated constraint ("this will be done BEFORE 002 and 003") sets the ordering; no
  other feature or task is assumed to depend on Wave 2 being complete.
- The module set subject to the lint gate is the current `go.work` membership (13 members):
  `netguard`, `gameaction`, `gameproto`, `operator`, `api`, `agent`, `audit-syslog-bridge`,
  `telemetry-receiver`, `sentinel`, `mcp-server`, `svcutil`, `test/e2e`, and `tunnel`. Modules added or
  removed from `go.work` after this spec is written are outside the scope of Wave 2 but will
  become subject to the gate by FR-004's maintenance requirement.
- The existing `.golangci.yml` linter selection (14 enabled linters: bodyclose, errcheck,
  gosec, govet, ineffassign, staticcheck, unused, misspell, revive, unparam, nilerr, noctx,
  errorlint, contextcheck) and its v2 schema are taken as given and will not be re-litigated
  during Wave 2. If a linter is deemed unsuitable for a module, that is a separate feature
  request, not part of this work.
- The single authorized gosec G115 exclusion (in the Minecraft VarInt codec) is accepted and
  will remain in place. No attempt to remove it or to add it to other modules is made during
  Wave 2. Future authorization of new exceptions follows the same review rigor and is
  documented with equal clarity.
- PR #216's current state is partially complete with some fix commits already authored but
  now conflicting against main. The decision to salvage, rebase, or redo that work is
  deferred to the `/speckit-plan` phase and is not made in this spec. The spec's success
  criteria do not depend on any specific implementation strategy.
- "Bringing a module under the gate" means adding it to the CI matrix and ensuring all its
  findings are fixed before the PR merges. It is a one-time event per module and is not
  reversible (once a module is gated, it stays gated per FR-003 and FR-004).
- Build-tag-conditional files must be analyzed by golangci-lint for the gate to be effective.
  The CI configuration MUST pass `--build-tags=envtest` when linting `api` and
  `--build-tags=e2e` when linting `test/e2e` to ensure these files are not silently skipped.
- Findings span both production and test code. `test/e2e` is entirely test code, and `api`
  and `agent` contain their own test files. Note that `.golangci.yml` already exempts
  `_test.go` files from errcheck, gosec, and unparam, so the findings surfaced in test code
  come from the remaining linters; that existing exemption is taken as given and is not
  revisited by this feature.
- No local machine execution is performed during Wave 2 work. All build, test, and lint
  execution happens in CI. Verification of fixes is done by reading diffs and reviewing CI
  logs, not by running commands locally.
