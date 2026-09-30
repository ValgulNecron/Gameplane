# Contract: v0.3.0 Status Wording and Version Markers

Covers FR-015 and FR-016. Applied as one commit, only after the go decision. The locations were found with `git grep` on 2026-09-23. Re-run the grep before applying (see the end of this file).

## Wording

| Replace | With |
|---|---|
| `Status: **beta** (\`v0.2.0-beta.8\`)` and variants | `Status: **pre-v1 release** (\`v0.3.0\`)` |
| `## Beta Status & Limitations` | `## Pre-v1 Status & Limitations` (update the `#beta-status--limitations` anchor at `README.md:10`) |
| `Gameplane is currently in **beta**` | `Gameplane is a **pre-v1 release**` |
| `**Status:** Beta (\`v0.2.0-beta.8\`)` in CLAUDE.md | `**Status:** Pre-v1 release (\`v0.3.0\`)` |
| `**Status:** beta (v0.2.0-beta.8)` in module `specs.md` | `**Status:** pre-v1 (v0.3.0)` |
| roadmap "between beta and a v1 GA" | "between v0.3 and a v1 GA" |

The new wording must not claim v1, "stable" or production support (OD-003). The remaining caveats stay in `docs/roadmap.md` under "Wanted for v1, not blocking".

## Version markers → `0.3.0`

- `charts/gameplane/Chart.yaml:5-6`: `version` and `appVersion`
- `web/package.json:4`, and the regenerated `web/package-lock.json` top-level version
- `README.md:8,39,61`
- `CLAUDE.md:6`
- `docs/roadmap.md:3,6,211`
- `docs/install.md:14` (example version), `docs/install.md:27` (edge-channel heading wording)
- `docs/dependencies.md:26,28,226`
- `telemetry-receiver/README.md:9,28`
- `web/specs.md:740`
- Status lines in `*/specs.md`: `agent`, `api`, `audit-syslog-bridge`, `capture-sidecar`, `gameaction`, `gameproto`, `mcp-server`, `netguard`, `operator`, `sentinel`, `telemetry-receiver`, `tunnel`, `web`, `test/e2e`, and `test/e2e/internal/**/spec.md`. Keep each file's existing qualifiers, such as "probe depth measured" and "in-progress".
- `.github/workflows/publish-edge.yaml:3`: the comment says "beta images". Change it to "edge images".

## Coupled changes, in the same release but not wording

- The upgrade baseline goes from `0.2.0-beta.5` to `0.2.0-beta.8`: `deploy/kind/upgrade.sh:36`, `.github/workflows/ci.yaml:968-970`, `.claude/agents/ci-triager.md:65`. This can land earlier, in a fix round.
- The default git module source `ref: v0.2.0-beta.6` (`charts/gameplane/values.yaml:473`) moves to the `gameplane-module` tag tested with v0.3.0.
- `hack/check-doc-versions.sh` only recognises `-beta.N` versions: see lines 19, 94, 103 and 151, where the single-digit `[0-9]` pattern disagrees with the `[0-9]+` in the header. Without a fix it can't catch stale `0.3.0` or `-rc.N` strings. It has to accept `X.Y.Z` and `X.Y.Z-(beta|rc).N` before the wording commit. This is seeded as a finding.
- `CHANGELOG.md` gets a `## [0.3.0]` section. The "Unreleased" entries, including the OIDC Helm role mappings flagged in 012 OD-8, move into it or into the right RC sections.

## Left as-is (historical or unrelated)

- Lines marked `<!-- doc-versions: historical -->`, "shipped v0.2.0-beta.X" roadmap headings, and past CHANGELOG sections.
- Chart comments about `<= 0.2.0-beta.5` behaviour (`charts/gameplane/templates/api.yaml:219,315`, `crd-apply-hook.yaml:71`).
- Kubernetes "This is a beta field" text in generated CRDs, `service.beta.kubernetes.io` annotations, and test fixtures that use "beta" as a server name.
- Web tests that assert no version or "beta" text leaks onto public pages (`web/src/routes/Share.test.tsx:137,559`, `web/e2e/specs/slice5.spec.ts:124`). These are login-privacy guards and must stay.
- `reddit.md` and `IDEA.md`. They're maintainer notes. Flag them to the maintainer, but they're not part of the product docs.

## Re-check command

```sh
git grep -n -E '0\.2\.0-beta\.[0-9]+|\b[Bb]eta\b' -- ':!specs/**' ':!CHANGELOG.md' ':!**/package-lock.json' ':!*.pen' ':!design-export/**'
```
Every remaining hit must fall under "Left as-is".
