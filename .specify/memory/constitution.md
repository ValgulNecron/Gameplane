<!--
Sync Impact Report
- Version change: 1.5.0 → 1.6.0
- Modified principles: III. Language & Ecosystem Best Practice — clarified that while
  in-source suppressions (//nolint, // eslint-disable-next-line, etc.) remain absolutely
  forbidden, narrowly-scoped, maintainer-authorized config-level exclusions in
  `.golangci.yml` are permitted in two categories: (1) path-scoped false-positive
  exclusions targeting a single path pattern and single linter/rule, or (2) global
  redundant-rule disabling where a linter rule is wholly subsumed by a stricter enabled
  rule. All exclusions carry inline justification and are inventoried in an exclusion
  policy document. Explicitly preserved the ban on broad/global rule-weakening that
  hides actual defect classes (disabling linters, repo-wide exclusions).
- Version history: 1.3.1 → 1.4.0 (VI exception) → 1.4.1 (II encryption fix) → 1.5.0
  (II re-export requirement) → 1.6.0 (III config-level exclusion carve-out)
- Added sections: none
- Removed sections: none
- Deferred / TODO placeholders: none — all template tokens resolved
-->

# Gameplane Constitution

## Core Principles

### I. E2E-Tested Delivery (NON-NEGOTIABLE)
Every feature MUST be verifiable end-to-end before it is considered done. A change is
incomplete — regardless of unit or integration coverage — until it has a corresponding
E2E test exercising the real user- or operator-facing path (dashboard flow, API contract,
CRD reconciliation loop, or agent behavior) in `test/e2e/`, added to a bucket in
`test/e2e/buckets.sh` per the project's e2e conventions. Unit tests (`make test-go`,
`make test-web`) and envtest integration tests (`make test-integration`) are required
supporting layers, not substitutes. New E2E tests MUST call `t.Parallel()`, use
per-test unique resource names, and respect existing shared-state guards
(`ociPushMu`, `ensureResticRepo`, per-bucket login budgets).

Every game module's E2E coverage MUST include a real join: a hand-rolled client that
speaks that game's actual wire protocol to connect to a booted GameServer, not a bare
TCP/UDP dial or a mocked handshake — a probe MUST be proven to both fail against a dead
address and succeed against a real listener before it is trusted. When a game is too
resource-heavy to run inside a GitHub Actions runner (memory/CPU/disk), its E2E test
MUST still be written and committed to `test/e2e/` — it is simply excluded from every
CI bucket in `test/e2e/buckets.sh` rather than skipped or deleted, and the bucket
exclusion MUST be commented with why. A heavy test that exists but never runs in CI is
still runnable on demand (locally against a real cluster, or in a manually triggered
job) and stays exempt from the "e2e bucket coverage" CI gate that would otherwise fail
on an unbucketed test.
Rationale: Gameplane's promise is that the same operational model works identically on
a single-node k3s homelab and a multi-node production cluster. Only a real E2E run
against a live control plane catches drift between CRD, operator, API, agent, and
dashboard — unit tests in isolation cannot. A protocol-level join is the only proof a
server is actually reachable and playable, not merely "a port is open"; and heavy game
servers (multi-GB images, sustained CPU) would blow CI runner budgets, so the tradeoff
is deferred execution, never deferred authorship.

### II. Design-First for User-Facing Change
Any change to the web dashboard's visual surface, and any change to the public website's
screens, MUST be designed first in the relevant Pencil source (`design.pen` for the
dashboard, `website.pen` for the public website's screens) via the `pencil` MCP server,
and translated to React only after the design is committed. Backend-only, API-only, and operator-only changes are exempt. The `.pen`
files are Pencil-owned design sources and MUST NOT be read, hand-edited, or deleted with
generic file tools (`Read`, `Grep`, `sed`, `cat`, `rm`) — they MUST be accessed and
modified through the `pencil` MCP server only. They are multi-megabyte machine-generated
JSON documents: hand-editing risks structural corruption Pencil cannot recover, and
reading one floods an agent's context while revealing nothing useful about the design,
for which `get_screenshot` and `export_nodes` are the correct tools.
Every design edit — a new screen or a change to an existing one, in either `.pen` file —
MUST be followed, in the same change, by re-exporting the touched object(s) to the
matching plain-file snapshot: `design-export/{json,screenshots}/` for `design.pen`,
`website/website-export/{json,screenshots}/` for `website.pen`. The export is produced
via the `pencil` MCP server (a JSON dump per touched node and a screenshot per touched
node), scoped to what changed, not a full re-run of every screen. A design change without
a matching export update is incomplete.
Rationale: the `.pen` files are the source of truth for the product's designed screens.
Code-led redesigns bypass that source of truth, drift from it silently, and have been
reverted before. They are large, complex machine-generated JSON structures where
uncontrolled hand-edits risk corruption that Pencil's recovery mechanisms cannot undo.
The export snapshot exists because the `.pen` format cannot be read directly (Principle
II above) and the Pencil MCP server's own whole-document read tools have proven
unreliable for bulk enumeration — a git-tracked, plain-file mirror is what lets any
agent or human inspect the current design state without a live Pencil session, and a
stale mirror actively misleads, so it cannot be allowed to drift from the source.

### III. Language & Ecosystem Best Practice
Code MUST follow the idioms of its language and the project's established tooling
rather than working around them. Go code wraps errors with `%w` so `errors.Is`/`errors.As`
keep working up the call stack; TypeScript runs under `strict` mode with
`@typescript-eslint/no-explicit-any` and `@typescript-eslint/no-floating-promises`
enforced — an unavoidable `any` gets a one-line comment explaining why, and a
fire-and-forget `Promise` is resolved with `await` or explicit `void`, never suppressed.
When `golangci-lint` or ESLint flags something, the code MUST be fixed, not silenced.
In-source suppression directives are absolutely forbidden — no `//nolint`, no `//#nosec`,
no `//lint:ignore`, no `// eslint-disable-next-line`, no `// @ts-ignore`. Narrowly-scoped,
maintainer-authorized config-level exclusions in `.golangci.yml` are permitted in two
categories:

(1) **Path-scoped false-positive exclusions**: Single path pattern, single linter or rule,
    with inline justification, targeting a false positive that the code handles correctly
    despite what the linter reports.

(2) **Redundant-rule disabling**: Disabling a linter rule globally when that rule is a
    duplicate or strict subset of another enabled linter rule, such that no class of actual
    defect goes unchecked. The disabled rule must be wholly subsumed by a stricter, more
    configurable enabled linter such that the underlying requirement (e.g., error-checking)
    remains enforced. Exclusions of this type are rare and require maintainer sign-off;
    specific instances are documented in `specs/004-lint-backlog-wave2/contracts/exclusion-policy.md`.

Global or broad rule-weakening — removing a linter from the enabled set, or repo-wide
exclusions for findings that would otherwise go unchecked — remains forbidden.

CRD Go type edits MUST be followed by `make generate && make manifests` in the same
commit, with the regenerated artifacts included.
Rationale: suppression directives and workarounds hide defects instead of fixing them,
and this project has already accumulated cases where that erased real signal. Fixing
the underlying issue is the only change that survives the next refactor.

### IV. Spec-Driven Development
Non-trivial work MUST move through the spec-kit lifecycle before code is written:
`/speckit-constitution` (this document) → `/speckit-specify` (what and why) →
`/speckit-plan` (how) → `/speckit-tasks` (breakdown) → `/speckit-implement` (execution).
A feature branch's spec and plan artifacts are the source of truth for intent; code
review checks the diff against them, not against an implementer's memory of a
conversation. Trivial fixes (typos, one-line corrections, config tweaks with no
behavioral ambiguity) are exempt from the full lifecycle but still get a clear commit
message stating intent.

Every module folder (each Go module directory such as `netguard/`, `gameaction/`,
`gameproto/`, `operator/`, `api/`, `agent/`, `audit-syslog-bridge/`,
`telemetry-receiver/`, `sentinel/`, `mcp-server/`; the `web/` tree; and each game
directory under `modules/<name>/`) MUST maintain a `specs.md` describing in detail how
that module actually works: its responsibilities and boundaries, the protocols or
contracts it implements (e.g. the wire-protocol handshake a `modules/<game>/` directory
speaks, or the CRDs a controller reconciles), its inputs/outputs, and the invariants
other modules depend on. `specs.md` MUST be updated in the same change that alters the
behavior it documents — a behavior change without a matching `specs.md` update is
incomplete.
Rationale: this repo has repeatedly lost work to agents re-deriving intent from scratch
mid-session or drifting from what was actually agreed. A written spec is the artifact
that survives context resets and hand-offs between sessions and agents. Per-module
`specs.md` files extend that same guarantee to a module's ongoing behavior, not just its
initial feature spec — critical for game protocols in particular, which are often
undocumented upstream and were previously reconstructed from scratch each time.

### V. Delegate to Workflows & Subagents
The main agent loop's job is decomposition, orchestration, judgment, and verification —
not the legwork. Work MUST be split into the largest number of genuinely independent
tasks it supports and fanned out to subagents concurrently. Delegated tasks start at
the smallest capable model tier and escalate one tier at a time only on demonstrated
failure of the smaller tier, never pre-emptively. Once a wave of subagents completes,
its combined output MUST be reviewed by an agent one tier above the tier the work ran
at before being accepted; fixes from that review are applied by relaunching small
agents, not by fixing in the main loop. Using the highest-capability model tier for
review or execution requires explicit human authorization first.

Delegation MUST prefer non-blocking orchestration over blocking calls. In Claude Code,
a single ad hoc subagent call (e.g. the `Agent` tool) blocks the calling turn until
that subagent finishes, which serializes the session behind the slowest task. The
`Workflow` tool runs in the background and returns immediately, notifying on
completion, and is what actually enforces this principle's own fan-out, tiered
escalation, and tier-up review requirements as deterministic script logic rather than
ad hoc prompting. A single blocking call to one subagent for a narrow, already-scoped
lookup is acceptable; any task that decomposes into multiple independent units, or
that needs the review-then-fix cycle, MUST run through a workflow rather than a chain
of blocking subagent calls.
Rationale: single-threaded, single-model execution is slower and costlier than
parallel cheap agents for well-scoped work, and a subagent's own "done" report is a
claim, not evidence — the tier-up review step is what catches the gap between the two.
A blocking subagent call still ties up the session for its full duration even when run
concurrently with others in one turn; non-blocking workflow orchestration is what lets
the main loop keep making progress instead of idling on the slowest branch.

### VI. CI Bears the Heavy Lifting
Builds, tests, lint, coverage, envtest, and E2E suites run on GitHub Actions CI, not on
a developer's or agent's local machine. A quick compilation check (`go build ./...`,
`tsc --noEmit`) to catch obviously broken code before pushing is permitted; running the
actual test or lint suites locally is not. Work is verified by pushing to a branch,
watching the CI run (`gh` CLI / the Actions UI), and fixing failures with follow-up
commits. A change MUST NOT be reported as validated, working, or ready to merge until
CI has actually run it green — a local build succeeding is not evidence a test suite
would pass.

The one exception: if the operator (the human directing the work) explicitly provides
a remote host or cluster for running builds/tests — e.g. a dedicated test VM or a live
Kubernetes cluster reachable over SSH/kubeconfig — tests and builds MAY be run there.
This is not a loophole for running suites on the agent's or developer's own local
machine; it applies only to infrastructure the operator has named and handed over for
that purpose, and CI remains the system of record for merge readiness regardless.
Rationale: CI is the only environment guaranteed to match production's toolchain,
cluster access, and resource limits; local runs on heterogeneous developer machines
have produced false confidence in the past, and heavy suites (kind/e2e) need CI-scale
resources this project's local dev environments don't reliably have. An operator-provided
remote host is a deliberate, known-good exception to that unreliability, not an
erosion of it — it exists for cases CI structurally cannot cover, such as measurements
against a real live cluster or exercising CI-down fallback paths.

## Additional Constraints

Technology choices, coverage thresholds, module boundaries, and generated-artifact
rules are governed in detail by `CLAUDE.md` at the repo root, which this constitution
takes precedence over in case of conflict. In particular: Go 1.25 across the
`go.work` modules, React 18 + TypeScript strict + Vite for the dashboard, the
`controller-runtime`/`client-go` operator stack, `chi` + `coder/websocket` for the API,
per-module coverage gates defined in each module's `.testcoverage.yml` (and
`web/vitest.config.ts` for the frontend), and the CRD/RBAC/Helm codegen pipeline
(`make generate`, `make manifests`) that MUST stay in sync with any CRD type change.

## Development Workflow

1. Constitution and spec artifacts govern before implementation begins (Principle IV).
2. UI/design work is designed in Pencil before code is written (Principle II).
3. Implementation is delegated and fanned out to subagents per Principle V, with
   tier-appropriate model escalation and mandatory tier-up review before acceptance.
4. Every logical unit of work is committed separately and signed (`git commit -s`),
   using conventional-commit prefixes, on its own branch; the branch is deleted
   (remote + local) once merged into `main`.
5. Verification happens on CI, not locally (Principle VI); a change is not "done"
   until its E2E coverage (Principle I) is green on a pushed branch.
6. Merges to `main` require all CI checks green, including the E2E tier — never
   merge on a partial-green or "flaky, ignore it" basis.

## Governance

This constitution supersedes ad hoc practice and prior undocumented conventions for
any Gameplane repository work. `CLAUDE.md` remains the detailed, evolving operational
reference (commands, file paths, workflows); where the two conflict, this constitution's
principles win and `CLAUDE.md` MUST be updated to match.

Amendments are made by editing this file directly, incrementing `CONSTITUTION_VERSION`
per semantic versioning (MAJOR: a principle is removed or redefined incompatibly;
MINOR: a principle or section is added or materially expanded; PATCH: wording or
clarification only), updating `Last Amended`, and recording the change in a Sync
Impact Report comment at the top of the file. Every PR and every spec/plan review MUST
check the proposed change against these principles; a violation MUST be justified
explicitly in the plan's Complexity Tracking section or the change MUST be redesigned
to comply. Use `CLAUDE.md` for the day-to-day runtime guidance this constitution
intentionally leaves at a higher level of abstraction.

**Version**: 1.6.0 | **Ratified**: 2026-08-11 | **Last Amended**: 2026-08-19
