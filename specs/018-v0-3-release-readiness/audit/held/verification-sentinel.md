# T045 edge chunk: independent verification (opus)

## Method

I looked for held security-control candidates for `sentinel/`. There is no `audit/held/review-sentinel.md`, and `audit/evidence/review-sentinel/notes.md` records "held candidates: 0" (OD-019). None of the 8 candidates in the evidence notes touches a security boundary or control, so none should have been held instead. I checked the controls the notes list as holding, on branch `018-v0-3-release-readiness` at `3de03ab0`, where nothing differs from master:
- The sentinel's Role grants only `get` and `patch` on its one named GameServer (`operator/internal/controller/gameserver_sentinel.go:400-405`).
- The pod runs non-root with all capabilities dropped and `allowPrivilegeEscalation: false` (`:215-252`).
- The wake patch is a JSON merge patch that sets only the wake annotation (`sentinel/main.go:359-372`).
- The per-source UDP map and the TCP handler semaphore are bounded (`main.go:387`, `:454-464`, `:802-813`).

All four hold as described.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| (none) | n/a | n/a | No held candidates for this component. |
