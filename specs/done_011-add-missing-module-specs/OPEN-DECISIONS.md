# Open Decisions

**Status**: None.

All unknowns from the feature discovery phase have been resolved by maintainer rulings D1–D6 (issued 2026-09-01, with D5–D6 issued post-review) and research findings. The research.md document captures the complete decision set, including:

- **D1**: Initial scoping of check location, implementation language, module list source, output format, and CI integration approach.
- **D2**: Exclusion of `modules/*` game directories from the compliance check; game module specs.md is the responsibility of the separate `gameplane-module` repo's CI.
- **D3**: Definition of "missing" specs.md: file does not exist, is empty, or contains only whitespace.
- **D4**: Placement of the modules/<game>/specs.md guideline in `docs/module-authoring.md` as a best-practice recommendation (not CI-enforced in this repo).
- **D5** (post-review, 2026-09-01): Clarification and authorization of CI wiring; **supersedes D1's CI half**. The check is a dedicated step in the existing lint job (gated `if: matrix.module == 'netguard'`), not a separate CI job, since CI does not invoke `make lint`.
- **D6** (post-review, 2026-09-01): Authorization for local execution of `hack/check-specs.sh` as a read-only compile-check exception under CLAUDE.md rule 8.

Supporting research findings:
- Correction of svcutil's coverage gate from the spec's stated 70% to the authoritative 90%
- Exported contract and invariants for svcutil and tunnel
- Canonical specs.md structure and its alignment with FR-005
- Implementation approach for the compliance check (shell script, go.work parsing, whitespace detection)
- Audit results confirming 12 of 14 go.work modules + web are currently compliant; svcutil and tunnel are the sole gaps

No genuine ambiguities remain that would require maintainer judgment. The path to implementation is clear: write svcutil/specs.md and tunnel/specs.md per the canonical structure and coverage gates, implement the shell script, integrate it into make lint, and update docs/module-authoring.md with the guideline note.
