# Data Model: v0.3 Release Readiness Audit

The entities below are records in Markdown files under `audit/`, not database tables. Field formats and table layouts are fixed in [contracts/audit-records.md](contracts/audit-records.md).

## Release Criteria (`audit/release-criteria.md`)

Written and committed **before** round 0 (FR-015). Changing it later needs a recorded maintainer decision.

| Field | Rule |
|---|---|
| `id` | `RC-01`…`RC-nn` |
| `statement` | One testable condition |
| `source` | Spec requirement or success criterion it enforces (e.g. `FR-005`, `SC-003`) |
| `evidence` | Where the report proves it (file/section) |
| `met` | `yes` / `no` / `pending`, filled at go/no-go only |

Required criteria (minimum set):
- Every inventory row has an outcome. Zero rows are `fail`. Every `blocked` row names its prerequisite and is listed as not live-verified (SC-001).
- Every component has a `complete` review record (SC-002).
- There are zero findings in `open`, `fixing` or `fixed-unverified` (SC-003, SC-004).
- Each FR-008 boundary has at least one active violation attempt recorded (SC-007).
- Live beta.8 → RC upgrade shows zero data loss and zero lost accounts (SC-005, subject to OD-005).
- Kubelab baseline snapshot matches the post-cleanup snapshot, and zero `audit018-` resources remain (SC-006).
- Every `not-a-defect` and `out-of-scope` closure has a justification, and each out-of-scope one cites a roadmap line (SC-009).
- CI is green on the tagged commit (FR-017).

## Feature Inventory Item (`audit/inventory.md`)

| Field | Rule |
|---|---|
| `id` | `INV-<area>-NNN`; area ∈ `WEB`, `API`, `CRD`, `AGT`, `AUX`, `HELM`, `MOD`, `UPG`, `NODE`, `SEC` |
| `name` | User-facing capability, verb-first ("Create server from template") |
| `component` | Owning component from the coverage table |
| `source` | Code location it was enumerated from (`file:line`) |
| `procedure` | Link to `procedures/<area>.md#<anchor>` |
| `ci_test` | E2E test name(s) covering it, or `none` |
| `outcome` | `untested` → `pass` / `fail` / `blocked` / `n/a` |
| `reason` | Required when `blocked` (named missing prerequisite + closest alternative tested) or `n/a` |
| `evidence` | Link to `evidence/<id>/` |
| `round` | RC round of the latest outcome (`rc.N`) |
| `findings` | Linked `F-` IDs |
| `flaky` | Optional: `k/n` failure rate when intermittent (spec edge case) |

State transitions:
```
untested ──run──▶ pass | fail | blocked | n/a
fail ──fix merged + rc.N+1──▶ untested ──run──▶ …
pass ──a fix touched same component──▶ untested   (regression sweep)
```
If `outcome` differs from the latest CI result for `ci_test`, a finding is raised (US1 AS-2).

## Component Review Record (`audit/coverage.md`)

| Field | Rule |
|---|---|
| `component` | Path (e.g. `api/`, `charts/gameplane/`, `docs/`) |
| `review_scope` | What was examined (files/areas, spec it was checked against) |
| `method` | `code-review`, `live-violation` (security boundaries), `consistency-only` (submodules) |
| `reviewer_tier` | Model tier of the reviewer and verifier (e.g. `sonnet → opus`) |
| `date` | ISO date |
| `status` | `not-started` → `in-review` → `complete` |
| `findings` | Linked `F-` IDs, or `none` |

## Finding (`audit/findings.md`)

| Field | Rule |
|---|---|
| `id` | `F-NNN`, sequential, never reused |
| `title` | One line |
| `component` | From coverage table |
| `origin` | `imported:<source>` (issue/PR/spec) or `live:<INV id>` or `review:<component>` |
| `repro` | Steps or observation, enough for another maintainer to repeat |
| `severity` | `S1` / `S2` / `S3` / `S4` (research R3); orders fix work only |
| `status` | see transitions |
| `disposition_note` | Required for `not-a-defect` / `out-of-scope`; out-of-scope MUST cite a `docs/roadmap.md` line placing the capability after v0.3 |
| `fix_ref` | PR number(s) |
| `reverify` | `rc.N` + evidence link of the passing live repeat |

State transitions:
```
imported ──live repro──▶ open | closed-already-fixed (repro passes on rc.N; evidence required)
open ──▶ fixing ──PR merged──▶ fixed-unverified ──live repeat passes on rc.N──▶ verified
open ──▶ not-a-defect        (justification)
open ──▶ out-of-scope        (roadmap citation; only capabilities the roadmap places after v0.3)
fixed-unverified ──live repeat fails──▶ open
```
Blocking statuses at go/no-go: `imported`, `open`, `fixing`, `fixed-unverified`. No other deferral state exists (SC-009).

## Test Resource

A record in `audit/rounds.md` (per round), not its own file.

| Field | Rule |
|---|---|
| `name` | MUST start with `audit018-` |
| `kind` | GameServer, Backup, Restore, BackupSchedule, NetworkCapture, ModuleSource, user, role, share link, notification sink, … |
| `created_by` | `api`, `dashboard`, `kubectl` |
| `round` | `rc.N` |
| `removed` | `yes` + cleanup evidence |

## Audit Round (`audit/rounds.md`)

| Field | Rule |
|---|---|
| `round` | `rc.0` (baseline/pre-RC reviews), `rc.1`, `rc.2`, … |
| `tag` | Git tag, and the approval reference for its publication (FR-019) |
| `helm_overrides` | Exact `--set` keys changed vs `kubelab-baseline.md` |
| `db_snapshot` | Location of the pre-upgrade SQLite snapshot (off-git) |
| `rows_run` | Count of inventory rows executed |
| `new_findings` | `F-` IDs |
| `cleanup` | Post-round cleanup + snapshot diff result |

## Audit Report (`audit/report.md`)

Built from the records above. It has no data of its own except the decision.
- Totals: inventory outcome counts, findings by severity and status, and component coverage percentage.
- A list of findings still open, which must be empty for a go.
- A not-live-verified list (every `blocked` row), copied into the release notes.
- A release-criteria table with `met` filled.
- A go/no-go statement, with the maintainer's name and date.
- On a go: the status-wording change set, from [contracts/status-wording.md](contracts/status-wording.md).
