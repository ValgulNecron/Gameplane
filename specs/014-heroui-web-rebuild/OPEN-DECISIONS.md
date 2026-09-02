# Open decisions — feature 014

Values here are not traceable to `spec.md`, `CLAUDE.md` or the constitution. Each is a question for the maintainer, not a decision. Nothing listed here is enforced in CI or written into a contract as settled until the row says **Settled**.

| ID | Question | Proposed default (not binding) | Status |
|---|---|---|---|
| OD-1 | Public share-link route path in the dashboard. The API serves `GET /shares/{token}` and `POST /shares/{token}/start`; the web route is undefined. | `/s/$token` (short, shareable). Alternative `/share/$token`. | Open |
| OD-2 | Where the light/dark/system appearance toggle lives (FR-006). No design frame shows one. | In the profile footer of the sidebar next to logout, plus the mobile drawer. Needs a design frame in slice 1. | Open |
| OD-3 | Constitution Principle I says every feature's E2E test lives in `test/e2e/` with a bucket. The dashboard's existing browser E2E is Playwright in `web/e2e/` (CI jobs `web-e2e-mock`, `e2e-web-live`), and the Go suite never drives a browser. | Treat `web/e2e/` live specs as the E2E tier for this feature (existing precedent since the Playwright suite was added) and add no Go e2e test. If the maintainer wants Principle I read literally, the constitution wording needs an amendment naming `web/e2e/`. | Open |
| OD-4 | The imported HeroUI sample frame `dashboard-utility` (`bJ2cg`) and its prompt node `OFfAu` sit in `design.pen` beside real screens. | Leave in place, excluded from `design-export/` like the other reference frames. Deleting is a design edit the maintainer must approve. | Open |
| OD-5 | Whether `heroUI template.pen` (repo root, untracked, 391 KB) is committed. | Not committed; `design.pen` already contains everything it has (R-05). Add to `.gitignore` alongside `design.pen.bak`. | Open |
| OD-6 | CSS bundle budget after `@import "@heroui/styles"` (root import pulls every component's styles). | Measure in slice 1; switch to per-component `layer(components)` imports if the gzipped CSS exceeds twice today's size. The threshold itself needs the maintainer's sign-off. | Open |
| OD-7 | Name of the directory holding the rebuilt Gameplane compositions while `components/ui/` still exists. | `web/src/components/hero/`, deleted-and-renamed to `components/ui/` in slice 5 when the old primitives go. | Open |
