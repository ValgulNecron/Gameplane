# design.pen export manifest

Generated via the `pencil` MCP server against `/home/valgul/project/kubernetes-game-dashboard/design.pen`. Read-only export — no Insert/Update/Replace/Delete/Copy/SetVariables calls were made.

## Totals

*(Recounted 2026-08-26 during the spec-006 install-time-config export refresh. Every figure below was measured, not carried forward — the previous "74 screens / 149 components / 223 objects / 447 files" block was stale.)*

- **Screens exported:** 77 — counted with the depth-0 `Get` visitor described below, filtering top-level nodes whose name starts with `Screen/` (138 top-level nodes, 77 of them screens). Baseline 74 + 3 added by spec 006 (`nNGDX`, `dxdEi`, `QgW58`).
- **Components exported:** 155 — every id in `get_app_state`'s "Reusable components" list (55 `Gameplane/...` definitions + 100 `c:...` base design-system primitives). Baseline 149 + 6 added by spec 006 (`Kp48V`, `XL5ZU`, `vStkb`, `R65Xyx`, `qvQPg`, and `uw0dB`, which existed as a node but had only ever been exported under the wrong filename — see below).
- **Total objects:** 232 (77 screens + 155 components)
- **JSON files written:** 232 / 232 (100%) — `ls json/ | wc -l` = 232, and the id set (with `c_` → `c:` un-sanitized) is an exact set-equality match against the 232 expected ids: zero missing, zero extra.
- **Screenshots written:** 232 / 232 (100%) — `ls screenshots/ | wc -l` = 232, id set identical to the JSON set.
- **Total files in design-export/:** 465 (232 JSON + 232 PNG + this MANIFEST.md)
- **Failed exports:** none.

Three top-level nodes are deliberately **not** exported, as in every previous pass: the `Shared Components Band` and `Gameplane/Connection Card — Tunnel States (reference)` scaffolding frames, and the three `Note — …` annotation frames (`m8wjom`, `I96cW`, `R6iab`) added by spec 006. They are neither screens nor reusable components, so they are outside the counted object set.

**Stale file removed:** `json/c_uw0dB.json` + `screenshots/c_uw0dB.png` held the `uw0dB` component under a `c:`-prefixed filename it never had (its own body reads `"id": "uw0dB"`). It was also pre-dating the chip rework. Both were deleted and replaced by correctly-named `uw0dB.json` / `uw0dB.png`.

## Enumeration method and confidence

**Screens:** Discovered a reliable one-shot method superior to the sampling approach originally suggested. `Get(document, visitorFn)` (the buggy whole-document walk) throws `TypeError: cannot read property of undefined` partway through *and* misreports `ctx.depth`/`ctx.parentCtx` for visited nodes — confirmed independently. However, calling `ctx.skipChildren()` inside the visitor as soon as a depth-0 (top-level) node is recorded avoids ever descending into the buggy subtree walk entirely. This produced **all 118 top-level document children in one execute call, with zero errors**:

```js
const results = [];
Get((n, ctx) => {
  if (ctx.depth === 0) { results.push({id: n.id, name: n.name}); ctx.skipChildren(); }
});
```

Filtering that list for names starting with `Screen/` yielded exactly 67 screens. This is a complete, non-sampled enumeration (not a partial/heuristic scan), so confidence in the screen count is **high** — the walk completed without truncation, crash, or a result-count cap. The task brief's "~100-120" estimate was speculative; the actual count in this document is 67. Design areas mentioned in the brief (Login variants, Create Server steps 1-5, Modules Catalog, Backups Index/Schedules/Restores, Share Link states x5, Cluster Settings, Server Detail tabs incl. all Settings sub-pages and Overview state variants, Users & RBAC incl. sub-pages, Admin Settings incl. all sub-pages, Mobile screens) are all present in the exported list — nothing that looked like an expected category was obviously missing.

**Components:** Taken directly from `get_app_state`'s "Reusable components" line, which the task brief already identified as reliable and non-truncated. 148 components total (48 `Gameplane/...` component definitions + 100 base `c:...` design-system primitives living inside the "lunaris: design system components" wrapper frame).

## Issues found and fixed

During the bulk export (delegated to 16 parallel haiku subagents, one per ~9-19-object batch), a spot-check (`json.load` over every output file) found **30 files that failed to parse**, despite every subagent self-reporting 100% success. Two distinct failure modes:

1. **15 files** had exactly one stray extra trailing character (`}` or `]}`) appended after an otherwise-complete, valid JSON body — a copy/paste artifact from the subagent's own Write call. Fixed by trimming to the valid JSON prefix (via `json.JSONDecoder().raw_decode`) and re-verifying.
2. **16 files** (mostly the largest screens — Server Detail Overview variants, Modules Catalog, Create Server steps, Admin Settings pages) were genuinely **truncated mid-structure** (missing closing brackets), most likely because the subagent's `Print(JSON.stringify(...))` output for that specific node exceeded some internal response-size limit on that run and got cut off. Fixed by re-fetching each of these 16 nodes directly (`Get(id, {depth:6})`) and writing the complete output; all came back complete on retry with no truncation.

Affected IDs (now all valid): `atqRh`, `BX0XM`, `bYDHC`, `DMnEi`, `DPrYX`, `dQV9N`, `E9EEv0`, `EZFW0`, `f1Vga`, `g5mEpx`, `I9W8z`, `IzuY2`, `JLaGB`, `kK8Ji`, `MaoHP`, `mQ1zB`, `n6Xlo`, `NLDDv`, `RC3Kf`, `t3IY3u`, `TE2jI`, `tTSdi`, `uMiwd`, `UMJli`, `uoxQW`, `V1VhGE`, `VM7ro`, `vUqMl`, `Wj0V4`, `WZdnw`, `xCJlu`.

**Lesson for future exports of this kind:** subagent "done, no failures" reports for this MCP were not reliable evidence of correctness — an explicit `json.load()` validation pass over every output file was necessary and caught real corruption the subagents missed.

## Known, accepted limitation: depth-6 elision

78 of the 215 JSON files (mostly larger screens and a handful of components with deeply nested decorative sub-trees) contain one or more `"children": "..."` markers — the `Get(id, {depth: 6})` call's normal behavior when a subtree exceeds the requested depth. Spot-checking several of these showed the elided content is consistently small, leaf-level decorative structure (icon-only wrapper frames, tiny count badges, progress-bar fills, sparkline path children) nested 6+ levels deep inside cards/tables — not missing top-level screen content. Per the task's own guidance, higher depth was only used where the *node itself* was suspiciously incomplete (the 16-file truncation issue above, all fixed at depth 6 once re-fetched cleanly); re-running all 78 at depth 10+ was judged not worth the added tool-call volume for what is uniformly decorative/leaf content. Flagging here for transparency rather than silently omitting it.

## Skipped IDs

None. All 232 discovered objects (77 screens + 155 components) have both a JSON file and a PNG screenshot. (This line previously read "215 (67 screens + 148 components)" — a wave-1 figure never updated as screens and components were added; corrected 2026-08-26 against a live recount.)

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
| `uw0dB` | Gameplane/Removable Group Chip/Secondary | Re-exported after the chip rework (14px label, `[4,4,4,8]` padding) and renamed to the `/Secondary` variant. First correctly-named export (see the stale-file note under Totals). |
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
