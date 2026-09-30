# T045 ci chunk: independent verification (opus)

## Method

I looked for held security-control candidates for `hack/`. There is no `audit/held/review-hack.md`, and `audit/evidence/review-hack/notes.md` records "held candidates: 0" (OD-019). None of the 5 candidates in the notes concerns a security boundary or control, so none had to move here:

- 3 of them are documentation-check tooling.
- 2 of them are CLAUDE.md wording.

The four `hack/` scripts run only in CI and locally. They read repository files, and none of them takes a secret, uses the network or writes outside the working tree:

- `check-links.sh` and `check-doc-versions.sh` only read the repository.
- `check-specs.sh` only reads `go.work` and each module's `specs.md`.
- `gen-module-schema.py` writes only `modules/.schema/`.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| (none) | n/a | n/a | No held candidates for this component. |
