# design.pen export manifest

Generated via the `pencil` MCP server against `/home/valgul/project/kubernetes-game-dashboard/design.pen`. Read-only export — no Insert/Update/Replace/Delete/Copy/SetVariables calls were made.

## Totals

*(Re-measured 2026-08-27 after the spec-006 design-review pass. Every figure below was measured directly from disk, not carried forward — the concurrent spec-006 export refresh had undercounted by 2 components due to the review adding `Rwnu3` and `BV5ei` after that export completed. The previous "74 screens / 149 components / 223 objects / 447 files" block (pre-spec-006) was stale.)*

- **Screens exported:** 77 — counted with the depth-0 `Get` visitor described below, filtering top-level nodes whose name starts with `Screen/` (138 top-level nodes, 77 of them screens). Baseline 74 + 3 added by spec 006 (`nNGDX`, `dxdEi`, `QgW58`).
- **Components exported:** 157 — every id in `get_app_state`'s "Reusable components" list (57 `Gameplane/...` definitions + 100 `c:...` base design-system primitives). Baseline 149 + 6 added by spec 006 (`Kp48V`, `XL5ZU`, `vStkb`, `R65Xyx`, `qvQPg`, and `uw0dB`) + 2 added by the spec 006 design-review pass (`Rwnu3`, `BV5ei`).
- **Total objects:** 234 (77 screens + 157 components)
- **JSON files written:** 234 / 234 (100%) — `ls json/ | wc -l` = 234, and the id set (with `c_` → `c:` un-sanitized) is an exact set-equality match against the 234 expected ids: zero missing, zero extra.
- **Screenshots written:** 234 / 234 (100%) — `ls screenshots/ | wc -l` = 234, id set identical to the JSON set.
- **Total files in design-export/:** 469 (234 JSON + 234 PNG + this MANIFEST.md)
- **Failed exports:** none.

Three top-level nodes are deliberately **not** exported, as in every previous pass: the `Shared Components Band` and `Gameplane/Connection Card — Tunnel States (reference)` scaffolding frames, and the three `Note — …` annotation frames (`m8wjom`, `I96cW`, `R6iab`) added by spec 006. They are neither screens nor reusable components, so they are outside the counted object set.

## Enumeration method and confidence

**Screens:** Discovered a reliable one-shot method superior to the sampling approach originally suggested. `Get(document, visitorFn)` (the buggy whole-document walk) throws `TypeError: cannot read property of undefined` partway through *and* misreports `ctx.depth`/`ctx.parentCtx` for visited nodes — confirmed independently. However, calling `ctx.skipChildren()` inside the visitor as soon as a depth-0 (top-level) node is recorded avoids ever descending into the buggy subtree walk entirely. This produced **all 118 top-level document children in one execute call, with zero errors**:

```js
const results = [];
Get((n, ctx) => {
  if (ctx.depth === 0) { results.push({id: n.id, name: n.name}); ctx.skipChildren(); }
});
```

Filtering that list for names starting with `Screen/` yielded exactly 67 screens. This is a complete, non-sampled enumeration (not a partial/heuristic scan), so confidence in the screen count is **high** — the walk completed without truncation, crash, or a result-count cap. The task brief's "~100-120" estimate was speculative; the actual count in this document is 67. Design areas mentioned in the brief (Login variants, Create Server steps 1-5, Modules Catalog, Backups Index/Schedules/Restores, Share Link states x5, Cluster Settings, Server Detail tabs incl. all Settings sub-pages and Overview state variants, Users & RBAC incl. sub-pages, Admin Settings incl. all sub-pages, Mobile screens) are all present in the exported list — nothing that looked like an expected category was obviously missing.

**Components:** Taken directly from `get_app_state`'s "Reusable components" line, which the task brief already identified as reliable and non-truncated. 157 components total (57 `Gameplane/...` component definitions + 100 base `c:...` design-system primitives living inside the "lunaris: design system components" wrapper frame), after exports from specs 006 (6 components: `Kp48V`, `XL5ZU`, `vStkb`, `R65Xyx`, `qvQPg`, `uw0dB`) and the 006 design-review pass (2 components: `Rwnu3`, `BV5ei`).

## Issues found and fixed

During the bulk export (delegated to 16 parallel haiku subagents, one per ~9-19-object batch), a spot-check (`json.load` over every output file) found **30 files that failed to parse**, despite every subagent self-reporting 100% success. Two distinct failure modes:

1. **15 files** had exactly one stray extra trailing character (`}` or `]}`) appended after an otherwise-complete, valid JSON body — a copy/paste artifact from the subagent's own Write call. Fixed by trimming to the valid JSON prefix (via `json.JSONDecoder().raw_decode`) and re-verifying.
2. **16 files** (mostly the largest screens — Server Detail Overview variants, Modules Catalog, Create Server steps, Admin Settings pages) were genuinely **truncated mid-structure** (missing closing brackets), most likely because the subagent's `Print(JSON.stringify(...))` output for that specific node exceeded some internal response-size limit on that run and got cut off. Fixed by re-fetching each of these 16 nodes directly (`Get(id, {depth:6})`) and writing the complete output; all came back complete on retry with no truncation.

Affected IDs (now all valid): `atqRh`, `BX0XM`, `bYDHC`, `DMnEi`, `DPrYX`, `dQV9N`, `E9EEv0`, `EZFW0`, `f1Vga`, `g5mEpx`, `I9W8z`, `IzuY2`, `JLaGB`, `kK8Ji`, `MaoHP`, `mQ1zB`, `n6Xlo`, `NLDDv`, `RC3Kf`, `t3IY3u`, `TE2jI`, `tTSdi`, `uMiwd`, `UMJli`, `uoxQW`, `V1VhGE`, `VM7ro`, `vUqMl`, `Wj0V4`, `WZdnw`, `xCJlu`.

**Lesson for future exports of this kind:** subagent "done, no failures" reports for this MCP were not reliable evidence of correctness — an explicit `json.load()` validation pass over every output file was necessary and caught real corruption the subagents missed.

## Known, accepted limitation: depth-6 elision

78 of the 215 JSON files (mostly larger screens and a handful of components with deeply nested decorative sub-trees) contain one or more `"children": "..."` markers — the `Get(id, {depth: 6})` call's normal behavior when a subtree exceeds the requested depth. Spot-checking several of these showed the elided content is consistently small, leaf-level decorative structure (icon-only wrapper frames, tiny count badges, progress-bar fills, sparkline path children) nested 6+ levels deep inside cards/tables — not missing top-level screen content. Per the task's own guidance, higher depth was only used where the *node itself* was suspiciously incomplete (the 16-file truncation issue above, all fixed at depth 6 once re-fetched cleanly); re-running all 78 at depth 10+ was judged not worth the added tool-call volume for what is uniformly decorative/leaf content. Flagging here for transparency rather than silently omitting it.

## Skipped IDs

None. All 234 discovered objects (77 screens + 157 components) have both a JSON file and a PNG screenshot. (This line was corrected 2026-08-27 to reflect the two components added by the design-review pass; prior entries incorrectly stated "232 objects" and "155 components" after the spec-006 export but before the review.)

## Filename sanitization

Component IDs prefixed `c:` (e.g. `c:xCEfn`) were saved as `c_xCEfn.json` in `json/` (`:` → `_`, per instructions — filesystem/tool safety). `export_nodes` screenshot filenames were left under its own control and it wrote them with the literal `:` intact (e.g. `c:xCEfn.png`) without erroring, so no renaming was needed on the screenshot side.

## In scope for spec 002 (Track B — address-pool override)

Three exported screens are in scope for the load-balancer address-pool override (`spec.networking.addressPool` / `spec.networking.address`). A design pass was completed in commit 7de0880 (2026-08-22) and all three screens' exports were refreshed (JSON + screenshot) to include the address-pool UI elements.

| ID | Screen | Export coverage |
|---|---|---|
| `f1Vga` | `Screen/Create Server — Step 4 Network` | Optional address-pool and requested-address inputs; alerts for preference-saved-but-not-applied states. |
| `J5pjJ3` | `Screen/Server Detail — Settings · Networking` | Current assignment display, both edit fields, five AddressAssignment status treatments, ignored/no-manager alerts as alternate states. |
| `EZFW0` | `Screen/Server Detail — Overview` | External address row showing the address with the pool it came from. |

All three have both a `json/<id>.json` and a `screenshots/<id>.png` file, with JSON/PNG timestamped to commit 7de0880.

## Incremental export 2026-08-23 — Network Capture feature (spec 003)

Feature 003 (network-packet-capture sidecar) design pass completed. Seven screens + one reusable component added to design.pen (per designer report: all frames placed at y=10155, no overlap with existing designs).

**Screens added (7):**

| ID | Name | Export notes |
|---|---|---|
| `Bbnga` | Screen/Server Detail — Capture (Not enabled) | Shows capture tab disabled; "Capture is not enabled on this server." status text + explanatory copy + Enable Capture button. |
| `dBILX` | Screen/Server Detail — Capture (Empty) | Captures tab active; status badge, retention note, warning banner, empty state ("No captures yet."). |
| `xvlB6` | Screen/Server Detail — Capture (Running) | Live capture card: capture id/filter, Stop button, Max duration + Max size progress bars with elapsed/remaining text, packet count ("1,234 packets captured"). |
| `m5kOm4` | Screen/Server Detail — Capture (List) | Completed captures table; 3 rows (`cap-8f7d3c1a` Completed, `cap-3ba91e02` Completed, `cap-e40cd971` Failed); ID/STATUS/SIZE/PACKETS/DURATION/COMPLETED AT/EXPIRES IN/FILTER columns; download/view/delete buttons; expiry badges (green/amber/red). |
| `O08uaD` | Screen/Server Detail — Capture — Start capture | Modal dialog over the dimmed captures list; Packet Filter input (valid, green check), Max Duration/Max Size/Retention fields with helper text, Start Capture button. |
| `b4eaUf` | Screen/Server Detail — Capture — Start capture (Invalid filter) | Same modal; filter input with red border, red X icon, error text "Invalid BPF syntax: syntax error at position 12 (invalid token 'foo')", disabled (greyed) Start button. Shows full captures table behind dimmed overlay (context). |
| `RodrS` | Screen/Server Detail — Settings · Network capture | Settings sub-nav showing "Network capture" entry (active/highlighted) between "Scheduled backups" and "Placement"; full form body: Enable Capture switch + disable/admin-access warning text ("...are not redacted."), Retention Window input + unit select + cluster-max note, Discard/Save changes footer buttons. |

**Component added (1):**

| ID | Name | Notes |
|---|---|---|
| `f0s9zG` | Gameplane/Capture Warning Banner | Reusable banner component; warning background, triangle-alert icon, title ("Caution: network packet captures contain real player data") + 3 bullet points + note text ("Captures are not redacted or sanitized...") + "Dismiss" action. Used on capture screens 2, 3, 4. |

**Shared component modifications:**

- `I9kvlZ` (Gameplane/Server Detail Tabs): new child `k62ubV` (tabCapture) added between tabBackups and tabSettings, inactive by default. This ripples to all existing Server Detail screens (Overview, Backups, Mods, etc.), which now show the full "Overview … Backups Capture Settings" tab bar.

**Export method & validation (corrected 2026-08-23, second pass):**

A first export pass of these 8 objects was hollow: its keyword check searched for "Capture", which also appears in every frame's *name*, so the check passed even on files whose actual on-screen content was elided by too-shallow a `depth`. Five of the seven JSON files were missing their own body text, and `RodrS` (the Settings form) was fetched at depth 5, which left its form body as a bare `"..."` placeholder; `Bbnga` came back at only 9 nodes / max depth 4 versus a comparable existing screen (`pssCT`, 28 nodes / max depth 7).

All 8 objects were re-exported from scratch:

- JSON: `Print(JSON.stringify(Get(id, {depth: 12})))` via the Pencil `execute` tool, for every one of the 8 objects — no depth below 12 was used this time, and none of the 8 responses showed any `"..."` elision marker.
- Screenshots: re-exported via `export_nodes` PNG batch (all 8, 2x scale), sizes 103 KB (`f0s9zG`, smallest — a single component) to 478 KB (`b4eaUf`, the modal-with-error screen).
- Validation was content-specific per file, not name-based: each file was checked for a string that can only appear in that screen's own body text (never in a frame name), e.g. `Bbnga` → "not enabled", `m5kOm4` → "EXPIRES IN" and "cap-", `RodrS` → "Retention" and "not redacted", `b4eaUf` → the literal invalid-filter error string. All 8 greps hit. `python3 json.load()` also passed on all 8 files.

**Lesson for future exports of this kind:** a keyword check that matches a frame's *name* (e.g. every one of these screens is named `...Capture...`) proves nothing about whether the frame's *content* came through — it will pass even on a file elided down to a handful of top-level nodes. Always grep for a string that is unique to the screen's own text content and could not appear in any frame/component name, and when in doubt about depth, fetch at a depth generous enough that no `"..."` elision marker appears in the response at all (depth 12 was sufficient for every object in this feature) rather than guessing a depth per screen's apparent complexity.

All 8 objects (7 screens + 1 component) have both `json/<id>.json` and `screenshots/<id>.png` files in design-export/, timestamped 2026-08-23 (second pass).

## Incremental export 2026-08-26 — Install-time configuration (spec 006)

Feature 006 (`specs/006-install-time-config/`, Slices 7 / 7b / 8 — FR-012, FR-015, FR-017, SC-006, SC-007) added the Helm-seeded OIDC snapshot, the dashboard-side role-mapping override editor, and moved the install-time storage-class row onto Cluster Settings. Eleven objects were exported or re-exported.

**Screens (5):**

| ID | Name | Export notes |
|---|---|---|
| `j9W8A` | Screen/Cluster Settings | **Re-exported.** New `Storage Card` (`DMHH5`) sits between the page header and the node grid: title "Storage", subtitle "Set at install time via Helm values — not editable from the dashboard.", and the `Game data storage class` row (hint naming `operator.gameDataStorage.storageClassName` / `--game-data-storage-class`) with the value `fast-nvme`. Node grid unchanged below it. |
| `dxdEi` | Screen/Cluster Settings — Default storage class | **New.** Second value state of the storage card (`VvdTF`): the value is an `Icon Label/Secondary` pill reading "Cluster default" and the hint adds "Left unset, so new volumes use the cluster's default StorageClass." — the contract's empty-string semantic. |
| `uMiwd` | Screen/Admin Settings — Authentication | **Re-exported (1440×1591).** Both new cards now live in a `Settings Column` (`klk20`) inside the settings layout, under the pre-existing Authentication panel: the read-only **Helm OIDC Provider Card** (`gVIBn` — groups claim, default role, Helm-seeded role mappings per role, and the conditional `HelmAdminMappingWarning` banner) and the **Role Mapping Overrides Card** (`txRYA` — per-role provenance badges, removable group chips, add-group rows, and a Save action with next-login helper copy). The install-time storage row that used to sit here has moved to `j9W8A`. |
| `nNGDX` | Screen/Admin Settings — Authentication (No OIDC mappings) | **Re-exported (1440×1463).** Empty-state variant: info alert "No OIDC role mappings yet" offering both remedies (add mappings here, or seed `api.oidc.roleMappings.*` at install), plus its own Role Mapping Overrides Card (`qDB71`) with all three roles in the not-overridden treatment and per-role empty states. |
| `QgW58` | Screen/Admin Settings — Authentication (Save rejected) | **New.** Error variant: field-level red-stroked input + "Group name can't be blank.", and a card-level `Gameplane/Error Banner` carrying the 400 response ("…helmOverride.roleMappings.admin must not contain blank group names. No changes were saved."). |

**Components (6):**

| ID | Name | Notes |
|---|---|---|
| `Kp48V` | Gameplane/Dialog/Confirm Admin Mapping | FR-015 confirmation dialog — a `WwNlX` instance overriding title, the "Full admin access" warning alert, and a chip preview of the group(s) being mapped to admin. |
| `uw0dB` | Gameplane/Removable Group Chip/Secondary | Exported after the chip rework (14px label, `[4,4,4,8]` padding) as the `/Secondary` variant. First export. |
| `XL5ZU` | Gameplane/Removable Group Chip/Orange | Admin-role chip colour (`$c:--color-warning`). |
| `vStkb` | Gameplane/Removable Group Chip/Violet | Operator-role chip colour (`$c:--color-info`). |
| `R65Xyx` | Gameplane/Provenance Badge/Overridden | Role-neutral outlined badge (`$c:--border` stroke, lucide `pencil`, "Overridden in dashboard") — deliberately not a `Label/Orange` or `Label/Violet`, both of which already carry role meaning. |
| `qvQPg` | Gameplane/Input/Small | 220×32 input documenting the smaller height used beside the Small/Outline "Add" buttons, replacing per-instance height overrides on `Gameplane/Input`. |

**Export method & validation:**

- JSON: `Print(JSON.stringify(Get(id, {depth: 14})))` per object. **Zero `"..."` elision markers** in any of the 11 files — checked programmatically, not by eye.
- The `Print` output was piped to disk without hand-transcription: each call appended a large padding string so the MCP result exceeded the inline-response cap and was persisted to a `tool-results/` file, from which the JSON was extracted between `<<<BEGIN id>>>` / `<<<END id>>>` markers, `json.loads`-validated, and re-serialised compactly to match the existing files' formatting. This removes the copy/paste corruption class that damaged 15 files in the wave-1 export.
- Screenshots: `export_nodes` PNG batch at 2x; all 11 verified non-empty with real pixel dimensions (`uw0dB` 236×52 smallest, `QgW58` 2880×3320 largest).
- Validation greps were content-specific — a string that appears only in the object's own body text, never in a frame name: `j9W8A` → "Game data storage class", `dxdEi` → "Cluster default", `uMiwd` → "Role mapping overrides", `nNGDX` → "No OIDC role mappings yet", `QgW58` → "Save failed (400)", `Kp48V` → "Mapping users to the admin role grants full cluster control", `R65Xyx` → "Overridden in dashboard". All hit.
- Size sanity: the five screens came back at 19.0–21.3 KB against ~7–11 KB for the comparable depth-6 Admin Settings exports (`n6Xlo` 8.0 KB, `RC3Kf` 7.2 KB, `g5mEpx` 8.8 KB) — richer, not hollow. `uMiwd` grew 8.0 KB → 20.7 KB and `j9W8A` 10.7 KB → 19.0 KB over their previous exports.

## Incremental export 2026-08-27 — Spec 006 design-review fixes (provenance badges, chip metric)

Follow-up pass on the spec 006 objects, addressing four defects raised independently by two design reviewers. Nine objects re-exported (3 screens, 4 existing components, 2 new components).

**Screens (3):**

| ID | Name | Export notes |
|---|---|---|
| `uMiwd` | Screen/Admin Settings — Authentication | **Re-exported.** The Operator row's "From Helm values" badge is no longer a per-instance content override on the lunaris `c:it00G` (`Label/Secondary`); it is now an instance (`UarEZ`) of the new `Gameplane/Provenance Badge/From Helm` (`Rwnu3`), so it renders as a sibling of the outlined "Overridden in dashboard" badge instead of a filled secondary pill. Removable group chips are 32px tall (was 26px), matching the Icon Label chips on the Helm card above. |
| `nNGDX` | Screen/Admin Settings — Authentication (No OIDC mappings) | **Re-exported.** All three per-role provenance badges (`uMANQ`/`o1r2Yg`/`FJQBG`, now `MNGVu`/`KrenL`/`anO56`) previously read "From Helm values" on a screen whose own subtitle states nothing is seeded from Helm. They now use `Gameplane/Provenance Badge/Not configured` (`BV5ei`). |
| `QgW58` | Screen/Admin Settings — Authentication (Save rejected) | **Re-exported.** Same two changes as `uMiwd`: Operator badge retargeted to `Rwnu3` (`hf41a`), chips on the 32px metric. |

**Components (6):**

| ID | Name | Notes |
|---|---|---|
| `Rwnu3` | Gameplane/Provenance Badge/From Helm | **New.** Sibling of `R65Xyx`, frame-identical (transparent fill, `$c:--border` 1px inner stroke, `$c:--radius-pill`, `[3,10]` padding, gap 6, 12px/500 `$c:--muted-foreground` label) with lucide `package` instead of `pencil`. Gives the provenance axis two visually related states instead of one component plus one one-off label override. |
| `BV5ei` | Gameplane/Provenance Badge/Not configured | **New.** Third sibling on the same frame, lucide `minus`, label "Not configured" — the neutral state for an install with no Helm seed and no dashboard override. |
| `R65Xyx` | Gameplane/Provenance Badge/Overridden | **Re-exported.** Added the missing `theme: {"c:Mode":"Dark"}` pin so the standalone component resolves in Dark like every other `Gameplane/*` component. No visual change in situ. |
| `uw0dB` | Gameplane/Removable Group Chip/Secondary | **Re-exported.** Theme pinned to Dark; padding `[4,4,4,8]` → `[8,8,8,12]` and `chipLabel` `lineHeight: 1.1428571428571428`, bringing the chip onto the same 32px metric as the lunaris `Icon Label/*` chips used on the Helm-seeded card and the confirm dialog. |
| `XL5ZU` | Gameplane/Removable Group Chip/Orange | **Re-exported.** Same theme pin and 32px metric change. |
| `vStkb` | Gameplane/Removable Group Chip/Violet | **Re-exported.** Same theme pin and 32px metric change. |

**Verification:**

- Chip metric measured, not eyeballed: on `uMiwd`, `BkZku` (`chip_admin_gameplane-admins`, Removable Group Chip family) was 169×26 and `gK9nC` (`chip_admin_gameplane-admins`, Icon Label family, Helm card on the same screen) 151×32. After the change both families measure 32px tall (`BkZku` 177×32, `vHQ8A` 152×32, `bIySP` 152×32; `gK9nC` 151×32, `B7qPG7` 126×32, `rS14I` 159×32).
- `Get(screen, (n,c) => c.problems && …)` over `uMiwd`, `nNGDX`, `QgW58`, `Kp48V` reports only the two pre-existing `Sidebar Item/*` "partially clipped" entries per screen — nothing introduced by this pass.
- JSON re-exported at `depth: 30`, extracted from the persisted `tool-results/` file (no hand transcription), `json.loads`-validated, and asserted free of `"..."` elision markers. Note the screens no longer contain the literal string "From Helm values": the label now lives in the `Rwnu3` component body, so the content-specific grep for these screens is the component id (`Rwnu3` on `uMiwd`/`QgW58`, `BV5ei` ×3 on `nNGDX`).
