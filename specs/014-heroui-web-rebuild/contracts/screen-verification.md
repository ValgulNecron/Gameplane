# Contract: screen verification (FR-009, SC-004)

Every rebuilt screen is accepted only through this procedure. It runs on CI and in the sonnet review; nothing here runs on a developer machine (rule 8).

## Inputs

- `design-export/screenshots/<id>.png` — the redrawn frame, exported at 2× (2880×1800 desktop, 780×1688 mobile) in the same commit as the design change.
- A Playwright screenshot of the shipped surface in the same state, taken by `web/e2e/screenshots/<slice>.spec.ts` in mock mode at viewport 1440×900 (or 390×844), `deviceScaleFactor: 2`, dark appearance, with MSW fixtures that reproduce the frame's sample data (server names, counts, phases as drawn).

## Procedure

1. The slice's screenshot spec lists each screen id it covers and the route, query and fixture that reach that state. A screen with no entry is not verified, and the PR says so.
2. The `web-e2e-mock` job uploads `web/playwright-report` including the screenshots as an artifact on every run (today only on failure; the spec adds `always()` for the screenshot project).
3. The reviewer (sonnet in the review Workflow, then the maintainer) compares each pair and records one verdict per screen in the PR description: **match**, **layout mismatch**, **colour mismatch**, **component-family mismatch**, **content mismatch** (fixture, not design). Only **match** or **content mismatch** is acceptable.
4. A **component-family mismatch** (a lunaris-looking control on a rebuilt screen, or an old primitive import) blocks the PR regardless of visual closeness.
5. Pixel-diff tooling is not required; the comparison is visual, at reference width, same appearance. Anti-aliasing and font-hinting differences are not mismatches.

## Per-slice coverage

| Slice | Screens verified | Viewport |
|---|---|---|
| 1 | `N1GkB jmoi3 ljdA5 N13Xud j24cXg` at 1440; `tooKB SeizD` at 390 | both |
| 2a | `iGBIs` + all Overview states + `P08Uw Xn5ns kPmoo FtdkI Burtr dPP50` | 1440 |
| 2b | Mods/Modpacks/Backups-tab/Capture states + every Settings section | 1440 |
| 3 | five wizard steps, `kK8Ji`, three Backups sub-pages | 1440 |
| 4 | every Admin, Users, Audit, System logs, Cluster frame | 1440 |
| 5 | seven share-link frames | 1440 |

## Other gates that must be green on the same PR

- `web` job: ESLint, `tsc`, Vitest with coverage thresholds 92/76/82/92.
- `web-e2e-mock`: existing 17 specs plus the slice's screenshot spec.
- `e2e-web-live`: existing 4 live specs plus the slice's additions (FR-011 flows for slice 1 and 2a).
- Test-file count and per-file `it()` count not lower than on `master` (checked by the reviewer with `git diff --stat` and a grep; SC-005).
- FR-012 import check (see `component-map.md`).
- `design-export/MANIFEST.md` row for every touched id; JSON has no `"..."` marker; PNG non-empty.
- Keyboard walk (SC-007) recorded in the PR for slices 1 and 2a/2b: Tab order through login, servers list, one settings section; no focus trap.
