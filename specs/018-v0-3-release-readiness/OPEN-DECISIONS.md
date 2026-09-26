# Open Decisions: v0.3 Release Readiness Audit

These values are unsettled. They are not contracts; resolve them during `/speckit-clarify` or `/speckit-plan`.

## OD-001: How the release candidate is deployed onto kubelab — RESOLVED 2026-09-23

Decision: public pre-release tag (option b). Recorded in `spec.md` Clarifications, FR-018, FR-019. Each publication needs explicit maintainer approval; kubelab's original image settings are noted for restoration.

Original question:

kubelab's live Helm release uses side-loaded images and a git module source that differ from the repository's local-development defaults. Deploying without care could repoint the registry, tag, or module source. Options: (a) side-load release-candidate images and `helm upgrade --reuse-values` with only intended overrides; (b) publish candidate images to the public registry under a pre-release tag. Needs the maintainer's choice.

## OD-002: Release-blocking severity threshold — RESOLVED 2026-09-23

Decision: every finding of any severity blocks the release; only "not a defect" or roadmap-cited out-of-scope closures are allowed. Recorded in `spec.md` Clarifications, FR-005, FR-007, SC-003, SC-009. Severity only orders the fix work.

## OD-003: Meaning of "no longer beta" — RESOLVED 2026-09-23

Decision: drop the beta suffix (v0.3.0), status wording becomes pre-v1 release, live upgrade from last beta, remaining caveats stay in the roadmap. Recorded in `spec.md` Clarifications, FR-015, FR-016. Exact README/roadmap wording is a planning detail.

## OD-004: Where findings and the report live — RESOLVED 2026-09-23

Decision: files inside this spec folder. Recorded in `spec.md` Clarifications and FR-020.
