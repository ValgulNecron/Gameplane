# Release Criteria: v0.3.0

Written before round 0 (FR-015). Columns are fixed by [contracts/audit-records.md](../contracts/audit-records.md#release-criteriamd). Every `Met` stays `pending` until the go/no-go decision (T070). Changing a criterion after this file is first committed needs a dated line under [Change log](#change-log) that cites the maintainer's decision.

| ID | Criterion | Source | Evidence | Met |
|----|-----------|--------|----------|-----|
| RC-01 | Every inventory row has an outcome, zero rows `fail`, and every `blocked` row names its prerequisite and is listed as not live-verified | SC-001 | [report.md#totals](report.md#totals), [report.md#not-live-verified](report.md#not-live-verified) | pending |
| RC-02 | Every component has a `complete` review record | SC-002 | [coverage.md](coverage.md), [report.md#by-component](report.md#by-component) | pending |
| RC-03 | Zero findings in `open`, `fixing` or `fixed-unverified` | SC-003, SC-004 | [report.md#open-findings](report.md#open-findings), [findings.md](findings.md) | pending |
| RC-04 | Each FR-008 boundary has at least one active violation attempt recorded | SC-007 | [inventory.md#sec](inventory.md#sec) | pending |
| RC-05 | Live beta.8 → RC upgrade with zero data loss and zero lost accounts | SC-005, OD-005 | [inventory.md#upg](inventory.md#upg) | pending |
| RC-06 | The baseline snapshot matches the post-cleanup snapshot, with zero `audit018-` resources remaining | SC-006 | [rounds.md](rounds.md), [kubelab-baseline.md](kubelab-baseline.md) | pending |
| RC-07 | Every `not-a-defect` and `out-of-scope` closure is justified, and each out-of-scope one cites a roadmap line | SC-009 | [findings.md](findings.md) | pending |
| RC-08 | CI is green on the tagged commit | FR-017 | [rounds.md#v030](rounds.md#v030) | pending |

## Change log
