# T045 edge chunk: independent verification (opus)

## Method

I looked for held security-control candidates for `capture-sidecar/`. There is no `audit/held/review-capture-sidecar.md`, and `audit/evidence/review-capture-sidecar/notes.md` records "held candidates: 0" (OD-019). I checked the evidence candidates for security-control content on branch `018-v0-3-release-readiness` at `3de03ab0`, where nothing differs from master.

One cross-reference needs a maintainer's decision. Evidence candidate C-capture-sidecar-01, the missing FR-003 default filter, describes the same defect as the operator chunk's held candidate H-operator-04. The operator reviewer classified that defect as a data-capture restriction control. I rejected the evidence copy as a duplicate, so the operator chunk carries it. The git-bound notes file still contains the evidence copy's text, so the maintainer should settle which classification applies before that file is committed.

The control that defect concerns does hold:
- The sidecar refuses an empty or missing filter with 400 (`capture-sidecar/internal/httpserver/handlers.go:378-381`).
- It refuses filters that compile to no packet test (`capture-sidecar/internal/capture/filter.go:69-134`).
- A capture without a filter therefore fails closed and never records unfiltered traffic.

To confirm the control still holds after the operator-side fix:
1. Start a capture with no filter against a template that advertises one game port.
2. Check that the request reaching the sidecar carries a non-empty filter restricted to that port.
3. Check that an empty filter sent straight to the sidecar is still rejected with 400.

The sidecar's authentication boundary also holds, and it is stricter than its comments say: the listener requires and verifies a client certificate for every route, `/healthz` included (`capture-sidecar/internal/auth/tls.go:34-39`). Evidence candidate C-capture-sidecar-08 covers only the comment wording, so it stays in evidence.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| (none) | n/a | n/a | No held candidates for this component. |

## Moved from audit/evidence/review-capture-sidecar/verification.md (OD-019, 2026-09-24)

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-capture-sidecar-01 | rejected | n/a | Duplicate. The operator chunk raises the same defect, with the operator reconciler as its primary location, and carries it there. No code in `operator/` or `api/` builds a default filter. The sidecar has no default filter of its own and rejects an empty filter. Not re-filed here. |

### Count line replaced

Rejected: 2 (C-capture-sidecar-01 as a duplicate, C-capture-sidecar-09 as a style preference).
