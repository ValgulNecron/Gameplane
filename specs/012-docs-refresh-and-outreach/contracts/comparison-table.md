# Contract: README Comparison Table and Source Documentation

**Status**: Specification Draft  
**Feature Branch**: `012-docs-refresh-and-outreach`  
**Applies To**: README.md placement and `docs/comparison-sources.md` schema

---

## 1. README.md Placement (FR-001, D-H)

The comparison table MUST be placed in README.md immediately after the "## Why Gameplane?" section and before the "## Features" section.

**Current line references**:
- "## Why Gameplane?" starts at line 42
- "## Features" starts at line 48
- **Insertion point**: between lines 47 and 48

**Placement specification**:
```
## Why Gameplane?

[existing text about Kubernetes-native architecture...]

[FR-004 STATUS LINE appears here]

[COMPARISON TABLE appears here]

## Features

[existing feature list...]
```

---

## 2. FR-004 Status Line (Verbatim)

Directly above the comparison table, include this status line without modification:

```markdown
Status: **beta** (`v0.2.0-beta.8`). The operator, API, agent, and dashboard
are feature-complete for the v1 scope and stabilized for external testing.
```

**Evidence**: README.md:8–9  
**Requirement**: Exact verbatim text per spec FR-004; this replaces any inline "beta" qualifiers in the table header.

---

## 3. Table Structure: Header Row and Column Order

The comparison table MUST use GitHub-flavored Markdown table syntax with this exact column order and header:

```markdown
| Dimension | Gameplane | Pterodactyl | CubeCoders AMP | Agones |
|-----------|-----------|-------------|----------------|--------|
```

Column order is fixed per spec FR-002 (not alphabetical): Gameplane, Pterodactyl, CubeCoders AMP, Agones.

---

## 4. Comparison Table: Row Labels and Order (FR-002)

The table MUST contain exactly nine rows in this order, with exact label text per FR-002 (a) through (i):

| Row # | Exact Label Text | FR-002 Reference |
|-------|------------------|------------------|
| 1 | Deployment/runtime model | (a) |
| 2 | Scaling & auto-sleep | (b) |
| 3 | Inbound connectivity (NAT traversal, relay) | (c) |
| 4 | Backup and restore | (d) |
| 5 | Access control & authentication | (e) |
| 6 | Game template distribution | (f) |
| 7 | Multi-tenancy & multi-cluster | (g) |
| 8 | Licensing | (h) |
| 9 | Target operator scope (self-hosted vs. managed SaaS) | (i) |

**Label precision requirement**: Each row label MUST match the exact text above, including punctuation, parenthetical qualifications, and capitalization. No rewording or abbreviation.

---

## 5. Gameplane Column (FR-003, D-A)

Each Gameplane cell MUST:

1. **State a factual capability** (max ~25 words), not a judgment
2. **Inline all qualifiers** using D-C vocabulary: `[BETA]`, `[optional]`, `[experimental]`, `[disabled by default]`, `[local cluster only]`, or similar
3. **Include a bracketed marker** `[G-x]` that links to `docs/comparison-sources.md#gameplane-row-x`, where `x` is the dimension letter (a, b, c, etc.)
4. **Evidence citation**: Each cell's claim must be traceable to one or more source paths: CRD types (`operator/api/v1alpha1/*_types.go`), README.md, docs/, CLAUDE.md, or values.yaml

### Gameplane Cell Template

```markdown
[<factual claim with inline qualifiers>] [G-a]
```

### Gameplane Column Content (R5-verified facts, with qualifiers)

| Row | Gameplane Cell Content |
|-----|------------------------|
| (a) Deployment/runtime model | Kubernetes-native CRDs and controller-runtime operator; scales from k3s homelab to multi-node clusters. [BETA] [G-a] |
| (b) Scaling & auto-sleep | Opt-in idle auto-sleep with configurable wake windows, manual wake button, or wake-on-connect. Minecraft/Terraria full protocol support; others use packet heuristics. [optional] [G-b] |
| (c) Inbound connectivity (NAT traversal, relay) | Integrated frp, Tailscale, playit relay sidecars; playit mappings user-managed via playit.gg account. [optional; disabled by default] [G-c] |
| (d) Backup and restore | Restic snapshots to S3-compatible storage; on-demand or cron-scheduled via BackupSchedule; one-click restore. [G-d] |
| (e) Access control & authentication | Local argon2id + OIDC (Keycloak/Google/GitHub); three built-in roles (admin/operator/viewer); custom roles supported. [G-e] |
| (f) Game template distribution | OCI bundles via ModuleSource (git/http/oci/local/upload); optional cosign signature verification per source. 16 ready-to-use templates shipped. [G-f] |
| (g) Multi-tenancy & multi-cluster | Cluster CRD for remote registration/monitoring; console/log streaming local-cluster only. [local cluster only for streaming] [G-g] |
| (h) Licensing | GNU Affero General Public License v3.0 or later (AGPL-3.0-or-later). [G-h] |
| (i) Target operator scope (self-hosted vs. managed SaaS) | Self-hosted only; runs on Kubernetes (k3s, kubeadm, managed services); no managed SaaS offering. [G-i] |

---

## 6. Competitor Columns (FR-005, FR-006)

Each competitor cell MUST:

1. **State only factual, verifiable features** — no speculation, judgment, or unverifiable claims
2. **Include a bracketed source marker** `[<ABBREV>-<row>]` that links to `docs/comparison-sources.md#<anchor>`
3. **Leave placeholders for implementation**: `<to be researched during implementation; source required>`
4. **Qualify with uncertainty**: If information cannot be verified, note it explicitly (e.g., "Not publicly documented")

### Source Marker Syntax

The marker format is: `[<ABBREV>-<dimension-letter>]`

Where:
- `<ABBREV>` is the competitor abbreviation:
  - `P` = Pterodactyl
  - `C` = CubeCoders AMP
  - `A` = Agones
- `<dimension-letter>` is the row identifier (a, b, c, d, e, f, g, h, i)

Example marker: `[P-a]` links to `docs/comparison-sources.md#pterodactyl-row-a`

### Competitor Column Template

```markdown
<factual claim describing feature or absence> [<ABBREV>-<dimension>]
```

If the cell cannot be verified, use:

```markdown
<to be researched during implementation; source required> [<ABBREV>-<dimension>]
```

### Competitor Column Placeholder Content

Each cell MUST contain exactly this placeholder (per FR-005, competitors are researched during implementation):

```markdown
<to be researched during implementation; source required> [<ABBREV>-x]
```

Where `x` is the dimension letter (a–i). The placeholder is a temporary token that will be replaced during implementation with actual findings and a dated source marker.

---

## 7. Table Cell Grammar Rules (FR-006, SC-001)

Every table cell MUST conform to these grammar and content rules:

1. **Factual, not judgmental**: Describe feature presence/absence and design choices, never state value judgments.
   - ✓ Good: "Local argon2id + OIDC support; three built-in roles"
   - ✗ Bad: "Superior authentication with flexible roles"

2. **Verifiable, not speculative**: State only what is publicly documented or observable in code.
   - ✓ Good: "Opt-in idle auto-sleep with configurable windows"
   - ✗ Bad: "Might support resource scaling in the future"

3. **Concise** (max ~25 words per cell): Long descriptions are moved to linked documentation.

4. **Qualifier tags inline**: Tag the feature state inline within the cell, not in a separate column.
   - ✓ Good: "Sentinel [optional] wake-on-connect for Minecraft/Terraria"
   - ✗ Bad: (cell) "Sentinel wake-on-connect" / (separate column) "Optional (Minecraft/Terraria)"

5. **"Not applicable" allowed**: If a feature dimension genuinely does not apply to a product (e.g., Agones does not have backup features per traditional control panel design), state `Not applicable` and cite why (if relevant).

---

## 8. docs/comparison-sources.md Schema and Structure

A new file `docs/comparison-sources.md` MUST be created with the following structure:

### File Header

```markdown
# Comparison Table Sources

This file documents the sources and verification dates for every claim in the
README comparison table. Each product has a section below. Entries are organized
by row (dimension a–i) and include the source URL, date checked, and what was
verified.

**Last Updated**: [YYYY-MM-DD during implementation]

---
```

### Per-Product Section Template

Each product has its own top-level section with subsections for each dimension:

```markdown
## <Product Name>

**License**: [License name and link]  
**Documentation Root**: [URL]  
**Repository**: [URL or "proprietary/unavailable"]

### Row (a): Deployment/runtime model

**Source ID**: [ABBREV]-a  
**URL**: [Canonical source for this dimension]  
**Checked on**: YYYY-MM-DD  
**What was verified**: [Brief description of what the source stated]  
**Last-known URL**: [Fallback URL if primary goes down]

[Repeat for rows b–i]
```

### Section Anchors

Each dimension row MUST have an anchor derivable from the source ID. Anchor format:

```markdown
### Row (<letter>): <dimension-label>
```

GitHub automatically generates anchors from heading text as `#row-<letter>-<dimension-label-slug>`.

Example: Row (a) heading generates anchor `#row-a-deployment-runtime-model`

Marker `[P-a]` in the table links to this anchor explicitly as `docs/comparison-sources.md#pterodactyl-row-a` (custom anchor).

**Custom anchor requirement**: Add explicit HTML anchors for clarity:

```markdown
### Row (a): Deployment/runtime model {#pterodactyl-row-a}
```

This ensures `[P-a]` link resolves precisely.

### Canonical Source Domains (R7)

Competitor sources MUST come from these official roots only:

| Product | Allowed Domains |
|---------|-----------------|
| Pterodactyl | https://pterodactyl.io/ (docs), https://github.com/pterodactyl/ (repo, license) |
| CubeCoders AMP | https://www.cubecoders.com/ (attempts to verify; if inaccessible, note as "proprietary, unverifiable") |
| Agones | https://agones.dev/site/docs/ (docs), https://github.com/agones-dev/ (repo, license) |

**CubeCoders exception** (R7, OD-9): If the website is not directly accessible via WebFetch or browsable by standard tools, document this clearly with the status "Proprietary; official documentation not directly verifiable via automated means. Recommend manual browser research during implementation or deferral with justification."

**404 fallback sourcing** (R7, OD-10): When dimension-specific documentation URLs return HTTP 404 (for Pterodactyl or Agones), fall back to the base documentation root with a dimension note (e.g., "Verified against Pterodactyl docs root at pterodactyl.io/docs, deployment section"). See OPEN-DECISIONS.md OD-10 for the fallback-sourcing decision and recommended strategy.

---

## 9. Table Rendering on GitHub (FR-007)

When committed and viewed on GitHub, the comparison table MUST:

1. Render as a readable Markdown table with aligned columns
2. Have clickable links in the bracket markers `[G-a]`, `[P-a]`, etc. pointing to `docs/comparison-sources.md`
3. Display inline qualifiers `[BETA]`, `[optional]`, etc. in monospace or as-is
4. Preserve line breaks and formatting without spurious HTML escaping

**Test** (before merge): View the README on GitHub and confirm the table renders as intended.

---

## 10. Acceptance Checklist

Implementation MUST verify all of these before marking the contract complete:

### Structure & Placement
- [ ] SC-001: Table is placed after "## Why Gameplane?" section and before "## Features"
- [ ] Comparison table has exactly 9 rows (a–i) in FR-002 order
- [ ] Column order is exactly: Gameplane | Pterodactyl | CubeCoders AMP | Agones
- [ ] All row labels match FR-002 exact text (including punctuation)

### Status Line (FR-004)
- [ ] Status line appears directly above the table, verbatim from README.md:8–9
- [ ] Status line is not repeated elsewhere in the table

### Gameplane Column (FR-003)
- [ ] SC-002: Every Gameplane cell is traceable to codebase, README, or docs/
- [ ] Every cell has exactly one evidence path (or paths joined with semicolon)
- [ ] FR-012: All optional/experimental features carry a tag on first mention ([optional], [experimental], [BETA], [local cluster only], [disabled by default])
- [ ] Every cell includes inline qualifier tags where applicable
- [ ] Every cell includes a source marker `[G-<letter>]`

### Competitor Columns (FR-005)
- [ ] Every competitor cell has a source marker `[<ABBREV>-<letter>]`
- [ ] Placeholders match exactly: `<to be researched during implementation; source required>`
- [ ] FR-006: No disparaging, speculative, or unverifiable language in cells (when filled)
- [ ] Source domains match R7 canonical roots

### Cell Grammar (FR-006)
- [ ] All cells use factual language, not judgments
- [ ] All cells are concise (~25 words max)
- [ ] No cell contains unverifiable claims

### docs/comparison-sources.md
- [ ] File is created at `docs/comparison-sources.md`
- [ ] File has one top-level section per product (Gameplane, Pterodactyl, CubeCoders AMP, Agones)
- [ ] Each product section has a subsection for each row (a–i)
- [ ] Each subsection has: source ID, URL, "Checked on [DATE]", what was verified, last-known-URL
- [ ] Custom HTML anchors are present for each row (e.g., `{#pterodactyl-row-a}`)
- [ ] Table markers in README link correctly to these anchors

### FR-007 & FR-008 (Table Rendering)
- [ ] Table renders as readable GitHub Markdown without escaping errors
- [ ] All links `[G-*]`, `[P-*]`, `[C-*]`, `[A-*]` are clickable and resolve to `docs/comparison-sources.md`
- [ ] FR-008: Multi-cluster console/log streaming correctly marked `[local cluster only]`
- [ ] All beta features marked with `[BETA]`
- [ ] All optional features marked with `[optional]` or `[disabled by default]`

### Evidence & Citations
- [ ] Every Gameplane cell evidence path is verifiable (file:line or file exists)
- [ ] Every competitor source is dated (YYYY-MM-DD format in docs/comparison-sources.md)
- [ ] CubeCoders AMP is either verified or explicitly noted as "proprietary, unverifiable"

---

## 11. Open Questions & Recommendations (to be addressed during implementation)

The following open questions (OD) from the planning phase MUST be resolved during implementation:

### OD-4: Tunnel in FR-012 Label Set

**Question**: Should the tunnel (relay supervisor) component be included in the five optional components labelled per FR-012?

**Evidence**: CLAUDE.md:372 marks tunnel as `[optional] relay client supervisor`; SC-007 requires every optional feature in CLAUDE.md to be labelled.

**Recommendation**: Include tunnel in the optional components list per FR-012. Evidence is SC-007 and CLAUDE.md rule.

### OD-9: CubeCoders AMP Verifiability

**Question**: If CubeCoders AMP's official website is not accessible via automated tools (WebFetch), how should the table handle it?

**Options**:
- (a) Defer CubeCoders column to implementation with a note: "Proprietary; official documentation not directly verifiable"
- (b) Attempt manual browser research during implementation
- (c) Use public secondary sources (third-party comparisons, forum posts) with caveats

**Recommendation**: Document the site access status clearly. If the site remains inaccessible, either defer with a note or cite secondary sources with a caveat "verified against secondary sources, not official documentation" in docs/comparison-sources.md.

### OD-10: 404 Dimension URLs — Fallback Sourcing

**Question**: R7-competitor-sources.md identified three Pterodactyl URLs and three Agones URLs that return HTTP 404 when fetched during research. Should implementation cite the base documentation root (fallback) as the source, or attempt to track down the correct dimension-specific URL?

**Evidence**: spec.md:104–105 FR-005 requires sources from official documentation. R7-competitor-sources.md notes fallback sources (Pterodactyl docs root: https://pterodactyl.io/, Agones docs root: https://agones.dev/site/docs/).

**Recommendation**: Use base documentation root + dimension notes as fallback. Per OPEN-DECISIONS.md OD-10: if a specific dimension URL is 404, cite the base docs root and note the section/feature name. This maintains verifiability without false precision.

### OD-11: Agones Scope and N/A Cells

**Question**: Agones is a Kubernetes library/operator, not a control panel (no dashboard, no user auth, no template distribution). Many comparison table dimensions do not map to Agones (e.g., "Access Control", "Template Distribution"). How should non-applicable dimensions be handled?

**Evidence**: spec.md:96 names Agones as a comparison product; spec.md:98 includes dimensions like "Access control & authentication" and "Game template distribution" which do not apply to a library. R7-competitor-sources.md flags this scope mismatch.

**Recommendation**: Keep Agones in the table and mark "Not Applicable" cells for non-applicable dimensions. Per OPEN-DECISIONS.md OD-11(b): "N/A — Agones is a library, not a control panel" educates evaluators about the different category without removing the comparison.

### OD-12: Agones Notation

**Question**: Should the comparison table include a notation clarifying that Agones is a Kubernetes library/operator, not a control panel? This would prevent evaluators from assuming parity with Pterodactyl and CubeCoders.

**Evidence**: spec.md:98 SC-001 states evaluators should "identify at least three key architectural or operational differences." Understanding Agones's role (library vs. panel) is a key difference. R7-competitor-sources.md and OD-11 establish the category distinction.

**Recommendation**: Include intro text above the table per OPEN-DECISIONS.md OD-12(b). A short paragraph noting "Gameplane is compared to Pterodactyl and CubeCoders (both control panels) and Agones (a Kubernetes library)" immediately sets expectations and prevents misreading.

---

## 12. References & Evidence

**Spec references**:
- Feature Specification: `/home/user/Gameplane/specs/012-docs-refresh-and-outreach/spec.md` (FR-001 through FR-008, SC-001 through SC-003)
- Orchestrator decisions: D-A (marker syntax), D-B (version truth), D-C (label vocabulary), D-H (placement)

**Research references**:
- R5 (Gameplane-column facts): `/tmp/claude-0/-home-user-Gameplane/.../R5-gameplane-facts.md`
- R7 (Competitor sources): `/tmp/claude-0/-home-user-Gameplane/.../R7-competitor-sources.md`

**Code sources** (evidence anchors):
- `charts/gameplane/Chart.yaml:6` — appVersion 0.2.0-beta.8
- `operator/api/v1alpha1/gameserver_types.go` — GameServer CRD with idle/wake/capture/relay specs
- `charts/gameplane/values.yaml` — feature toggles (tunnel, sentinel, capture, mcp-server, telemetry)
- `LICENSE` — AGPL-3.0-or-later text
- `README.md:8–9` — current status line (exact verbatim text)
- `docs/architecture.md` — architecture reference and multi-cluster scoping
- `CLAUDE.md` — feature documentation (optional components, K8s-native design)

---

## 13. Notes for Implementation

1. **Placeholder filling**: During implementation, each `<to be researched during implementation; source required>` placeholder MUST be replaced with actual verified content from official sources, dated and linked.

2. **Link validation**: Before merging, verify that all `[G-*]`, `[P-*]`, `[C-*]`, `[A-*]` markers in README resolve correctly to `docs/comparison-sources.md` anchors.

3. **Version freshness**: The Gameplane column uses v0.2.0-beta.8 facts verified on 2026-09-01. If a new release ships during implementation, re-verify the Gameplane column against the new appVersion.

4. **CubeCoders research**: R7 notes that CubeCoders AMP's website requires JavaScript rendering and WebFetch cannot access it. Implementation should either attempt manual browser research or document the limitation explicitly in docs/comparison-sources.md.

5. **Competitor sources update frequency**: Competitor documentation may change. If a source URL returns 404 during implementation, fall back to the canonical documentation root (e.g., https://pterodactyl.io/ or https://agones.dev/site/docs/) and note the fallback in docs/comparison-sources.md. See OPEN-DECISIONS.md OD-10 for the fallback-sourcing decision and recommended approach.

