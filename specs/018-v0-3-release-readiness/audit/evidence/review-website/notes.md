# Review: website/ (consistency-only)

- **Date**: 2026-09-23
- **Reviewer tier**: sonnet (verified by opus)
- **Checked against**: product behaviour (modules/ catalog, CHANGELOG.md, README.md), README.md, docs/

## Scope reviewed

- `website/src/config.ts` (site-wide displayed version)
- `website/src/pages/games.astro` (full game list array)
- `website/src/pages/index.astro`, `website/src/pages/features.astro`, `website/src/pages/compare.astro`, `website/src/pages/showcase.astro` (grepped for counts/claims; not read line-by-line in full)
- `website/src/content/docs/getting-started.mdx` (install command, version pin)
- `website/src/content/docs/comparison.mdx` (game-catalog row)
- Grepped all of `website/src/content/docs/*.mdx` and `website/src/pages/*.astro` for stale-count/version patterns
- Did not read: `website/src/components/`, `website/src/layouts/`, the remaining prose of every `.mdx` doc page not flagged by the grep pass (`architecture.mdx`, `console-protocols.mdx`, `contributing.mdx`, `crd-catalog.mdx`, `dashboard-tour.mdx`, `extension-services.mdx`, `faq.mdx`, `get-involved.mdx`, `module-authoring.mdx`, `multi-cluster.mdx`, `roadmap.mdx`, `security.mdx`, `changelog.mdx`)

## Method

Cross-checked every version string and game-count claim found in the website submodule against the actual `modules/` directory listing (30 modules) and the root repo's `CHANGELOG.md`/`README.md` current version (`v0.2.0-beta.8`).

## Observations (no finding)

- `website/CLAUDE.md`'s architecture rules (semantic tokens, `withBase()`, content-collection docs nav) are conventions, not falsifiable product claims — out of scope for this pass.
- No install commands or Helm keys in `getting-started.mdx` other than the version pin (flagged below) contradict `docs/install.md`.

## Candidate findings

### C-website-01: Public games page (`games.astro`) lists exactly 16 games; the actual shipped catalog is 30 — 14 real modules are missing from the public site

- **Location**: `website/src/pages/games.astro:9-108` (the `games` array; verified via `grep -c "title:"` = 16)
- **Category**: docs-drift (consistency: website vs. product)
- **Suggested severity**: S3
- **Observation / repro**: `games.astro`'s hardcoded `games` array lists: Minecraft (Java Edition), Valheim, Terraria, Factorio, Palworld, Rust, DayZ, Satisfactory, Counter-Strike 2, ARK: Survival Ascended, Project Zomboid, 7 Days to Die, Garry's Mod, Enshrouded, Don't Starve Together, V Rising — 16 entries. The root repo's `modules/` directory (source of truth for what's actually shippable, confirmed each has a `module.yaml`) has 30 entries. The 14 missing from the website page: `arma-reforger`, `beammp`, `euro-truck-simulator-2`, `farming-simulator-25`, `fivem`, `hell-let-loose`, `left-4-dead-2`, `mount-and-blade-2-bannerlord`, `nuclear-option`, `squad`, `team-fortress-2`, `the-isle`, `tmodloader`, `ark-survival-evolved`.
- **Expected**: The public "Games" page reflecting all 30 shipped game modules, or an explicit "and N more" framing if the page is deliberately curated.
- **Actual**: A visitor to the public marketing site's dedicated games page sees fewer than half of the games Gameplane actually ships templates for — this is the exact "product behaviour vs. website claim" gap the review brief calls out, and it's the same root staleness as the README's "16 templates" claim (see review-docs notes, C-docs-01), just independently hardcoded in the website submodule rather than shared.

### C-website-02: `comparison.mdx` repeats the same stale "16 official modules" figure

- **Location**: `website/src/content/docs/comparison.mdx:14`
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**: The comparison table row reads: `| Game catalog | 16 official modules; open template format | ... |`. Same discrepancy as C-website-01 / review-docs C-docs-01 (actual count is 30).
- **Expected**: A count matching the real `modules/` catalog.
- **Actual**: Same stale figure repeated a third time (README, games.astro, and here), reinforcing that this is a real, multi-file drift rather than a one-off typo.

### C-website-03: Website's displayed version and install-command example are pinned one release behind the current release

- **Location**: `website/src/config.ts:13` (`export const VERSION = "v0.2.0-beta.7";`); `website/src/content/docs/getting-started.mdx:24` (`--version 0.2.0-beta.7`)
- **Category**: docs-drift
- **Suggested severity**: S4
- **Observation / repro**: The root repo's `CHANGELOG.md` has a dated `## [0.2.0-beta.8] — 2026-08-22` entry, and `README.md`'s status line reads "Status: **beta** (`v0.2.0-beta.8`)". `website/src/config.ts`, which per `website/CLAUDE.md` is where "the displayed version" lives site-wide, is still set to `v0.2.0-beta.7`, and the getting-started guide's copy-pasteable install command pins the same older version.
- **Expected**: `v0.2.0-beta.8` (or a version-agnostic placeholder, as `README.md`'s own one-shot install command uses `--version <version>`).
- **Actual**: The site's own copy-paste install example, and every place `VERSION` is surfaced (this review didn't enumerate every render site of the constant), point at a release one behind the current one. Not functionally broken — beta.7 is a real, presumably still-available tag — but it's an easy one-line staleness that undercuts a "getting started" page's job of onboarding onto the current release.

## Questions (not findings)

- Whether `games.astro`'s per-game `version:` fields (e.g. Minecraft "v2.8.1", Valheim "v2.3.3") track the actual module `spec.versions`/image tags in `modules/*/template.yaml` was not checked — would require a per-module diff against the website's per-card version string, which this pass didn't have budget for. Flagging as a question in case a future pass wants to spot-check it.
