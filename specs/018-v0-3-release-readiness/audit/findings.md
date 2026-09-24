# Findings

Every finding blocks the release until it is `verified`, `closed-already-fixed`, `not-a-defect` or `out-of-scope` (FR-005, OD-002). Severity (research R3) only orders the fix work: S1 data loss, security-boundary break, or upgrade/rollback corruption; S2 core path broken with no workaround; S3 degraded behaviour or a workaround exists; S4 cosmetic, documentation or wording. Columns are fixed by [contracts/audit-records.md](../contracts/audit-records.md#findingsmd).

Security findings that are not yet fixed are held off-git until their fix merges ([OD-019](../OPEN-DECISIONS.md#od-019-security-findings-stay-unpushed-until-fixed--resolved-2026-09-24)), so the ID sequence below can have gaps.

| ID | Title | Component | Origin | Severity | Status | Fix | Re-verified | Note |
|----|-------|-----------|--------|----------|--------|-----|-------------|------|

## Details
