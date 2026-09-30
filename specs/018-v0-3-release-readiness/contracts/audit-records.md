# Contract: Audit Record Files

All files live in `specs/018-v0-3-release-readiness/audit/`. Field semantics are defined in [../data-model.md](../data-model.md). This contract fixes the file layout so reviewers and later agents can parse the records without guessing.

## General rules

- Markdown only. One table per record type, with columns in the order given here. Don't reorder or rename columns.
- IDs are stable. An ID is never renumbered or reused. A withdrawn row stays in place with outcome `n/a` or status `not-a-defect`, plus a reason.
- Every link is relative, so `hack/check-links.sh` can check it.
- **No secrets in git.** Session cookies, CSRF tokens, share tokens, client secrets, kubeconfigs, API keys, and the database snapshot never go into `audit/`. Evidence logs are redacted with the same patterns as `hack/` dump redaction (#306 / #327). A share token appears as `<token>`.
- Evidence goes in `audit/evidence/<ID>/`: `.txt`/`.log` (trimmed to the relevant part), `.json`, and `.png` screenshots. Keep each file under 1 MB.
- Every change to an outcome or status names its round (`rc.N`).

## `release-criteria.md`

```markdown
| ID | Criterion | Source | Evidence | Met |
|----|-----------|--------|----------|-----|
| RC-01 | … | SC-001 | report.md#inventory | pending |
```
Committed before round 0 (FR-015). An edit after that needs a line under `## Change log`, dated and citing the maintainer's decision.

## `inventory.md`

One `## <Area>` section per area code (`WEB`, `API`, `CRD`, `AGT`, `AUX`, `HELM`, `MOD`, `UPG`, `NODE`, `SEC`):
```markdown
| ID | Capability | Component | Source | Procedure | CI test | Outcome | Reason | Evidence | Round | Findings |
|----|------------|-----------|--------|-----------|---------|---------|--------|----------|-------|----------|
| INV-API-001 | Log in with local account | api/ | api/cmd/main.go:271 | [p](procedures/api.md#login) | TestAPI_LoginPrivacy | pass | | [e](evidence/INV-API-001/) | rc.1 | |
```
Allowed `Outcome` values: `untested`, `pass`, `fail`, `blocked`, `n/a`. `Reason` is required when the outcome is `blocked` (it names the missing prerequisite and the closest alternative that was tested) and when it is `n/a`.

## `coverage.md`

```markdown
| Component | Scope reviewed | Method | Reviewer tier | Date | Status | Findings |
|-----------|----------------|--------|---------------|------|--------|----------|
| netguard/ | netguard.go, specs.md | code-review + live-violation | sonnet → opus | 2026-09-24 | complete | F-012 |
```
Required rows: every `go.work` entry (`agent`, `api`, `audit-syslog-bridge`, `capture-sidecar`, `gameaction`, `gameproto`, `gp-module`, `mcp-server`, `netguard`, `operator`, `sentinel`, `svcutil`, `telemetry-receiver`, `test/e2e`, `tunnel`). Also `web/`, `charts/gameplane/`, `deploy/`, `hack/`, `.github/workflows/`, `docs/`, root docs, `design-export/`, and `modules/` and `website/` (both `consistency-only`).

## `findings.md`

```markdown
| ID | Title | Component | Origin | Severity | Status | Fix | Re-verified | Note |
|----|-------|-----------|--------|----------|--------|-----|-------------|------|
| F-001 | … | web/ | imported:#373 | S3 | open | | | |
```
Each finding also gets a `### F-NNN` subsection under the table. It holds **Repro / observation** (numbered steps), **Expected**, **Actual**, **Evidence** (link), and for `not-a-defect` / `out-of-scope` a **Justification** that cites a `docs/roadmap.md` line for out-of-scope. Allowed `Status` values: `imported`, `open`, `fixing`, `fixed-unverified`, `verified`, `closed-already-fixed`, `not-a-defect`, `out-of-scope`.

## `procedures/<area>.md`

One anchor-addressable `### <slug>` per procedure, with:
**Preconditions** · **Resources created** (all `audit018-`) · **Steps** (exact commands or dashboard clicks; login-budget cost noted) · **Expected** · **Cleanup** · **Automatable?** (yes/no + proposed E2E bucket, FR-012).

## `rounds.md`

One `## rc.N` section per round, with: tag + approval reference, Helm overrides vs baseline, DB snapshot location (off-git path only), rows run, new findings, test-resource table (name, kind, created_by, removed), and the cleanup snapshot-diff result.

## `report.md`

Sections in this order: **Decision** (go/no-go, maintainer, date) · **Release criteria** (table with Met) · **Totals** · **Open findings** (must be empty for a go) · **Not live-verified** (every `blocked` row; copied verbatim into the release notes) · **By component** · **By area** · **Proposed E2E additions** · **Status-wording change set**. The report must be readable cold in under 15 minutes (SC-008).
