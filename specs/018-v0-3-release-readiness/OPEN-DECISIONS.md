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

## OD-005: Fix group 37 residuals (F-138, F-141) — recorded 2026-09-26, worktree `wtg37`

This worktree's checkout of `specs/018-v0-3-release-readiness/audit/findings.md` does not exist at the branch point used for fix group 37 (`fix/018-web-code-quality-and-docs`, based on current master), so these residuals are recorded here per CLAUDE.md rule 10 instead; they should be merged into `findings.md`'s F-138/F-141 entries by whoever next has write access to that file.

**F-138 (dead-export removal) — left partly open.** The following exports are used only by their own tests, not by any app code, but were kept because removing a symbol that is still test-covered needs explicit sign-off (group rule): `RequireRole`, `hasRole`, `themeExportToUpdate`, `Modules.get`, `Schedules.get`, `BackupDestinations.get`, `Restores.remove`. Everything else provably dead (grepped across `web/src` including tests) was removed in this pass: `Users.getPreferences` and path-only `Shares` endpoint helpers (app and tests use `api.ts`'s `Shares` instead), the always-true tunnel guard in `CreateServer.tsx`, and `ServerCard`'s unused `onAct` prop (`LifecycleVerb` itself is still used).

**F-141 (React/TS/Vite version claims) — fixed only the version facts, per group scope.** `web/specs.md` still has stale unrelated details nearby that were not touched: line references into `api.ts`, `ServerDetail.tsx`, and `Login.tsx`, plus the "nine sub-views" and "11 sections" counts. These were left alone since F-141's scope was the version numbers only (React 18→19, TypeScript 5.6→6.0, Vite 5.4→8.3 in `web/specs.md` and `CLAUDE.md`'s architecture table, matching `web/package.json`'s `react ^19.3.0`, `typescript ^6.0.3`, `vite ^8.3.0`); no other CLAUDE.md rule text was touched.
