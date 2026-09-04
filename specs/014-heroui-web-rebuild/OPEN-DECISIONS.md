# Open decisions — feature 014

Values here are not traceable to `spec.md`, `CLAUDE.md` or the constitution. Each was a question for the maintainer, not a decision, until ruled on. All seven rows below were settled by the maintainer on 2026-09-03; each is now enforced in CI and written into contracts/tasks as settled, per the ruling recorded in its row.

| ID | Question | Ruling | Status |
|---|---|---|---|
| OD-1 | Public share-link route path in the dashboard. The API serves `GET /shares/{token}` and `POST /shares/{token}/start`; the web route is undefined. | Public share route path is `/share/$token`. | Settled 2026-09-03 |
| OD-2 | Where the light/dark/system appearance toggle lives (FR-006). No design frame shows one. | The appearance toggle lives in the sidebar profile footer next to logout, mirrored in the mobile navigation drawer; slice 1 adds a design frame for it. | Settled 2026-09-03 |
| OD-3 | Constitution Principle I says every feature's E2E test lives in `test/e2e/` with a bucket. The dashboard's existing browser E2E is Playwright in `web/e2e/` (CI jobs `web-e2e-mock`, `e2e-web-live`), and the Go suite never drives a browser. | `web/e2e/` Playwright live specs are the E2E tier for this feature; no Go `test/e2e/` test is added. The constitution wording will be amended separately to name `web/e2e/`. | Settled 2026-09-03 |
| OD-4 | The imported HeroUI sample frame `dashboard-utility` (`bJ2cg`) and its prompt node `OFfAu` sit in `design.pen` beside real screens. | Delete the HeroUI sample frame `dashboard-utility` (`bJ2cg`) and its prompt node `OFfAu` from `design.pen` via the Pencil MCP, in slice 0; the maintainer saves in the GUI. | Settled 2026-09-03 |
| OD-5 | Whether `heroUI template.pen` (repo root, untracked, 391 KB) is committed. | `heroUI template.pen` is NOT committed; add it to `.gitignore` next to `design.pen.bak`. | Settled 2026-09-03 |
| OD-6 | CSS bundle budget after `@import "@heroui/styles"` (root import pulls every component's styles). | No CSS bundle budget; the root `@import "@heroui/styles"` stays regardless of size. The measure-and-switch task is dropped. | Settled 2026-09-03 |
| OD-7 | Name of the directory holding the rebuilt Gameplane compositions while `components/ui/` still exists. | Rebuilt compositions live in `web/src/components/hero/` during the transition and are renamed to `web/src/components/ui/` in slice 5, after the old primitives are deleted. | Settled 2026-09-03 |
