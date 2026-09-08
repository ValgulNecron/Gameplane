# design.pen export manifest

Generated via the `pencil` MCP server against `/home/valgul/project/kubernetes-game-dashboard/design.pen`. Read-only export — no Insert/Update/Replace/Delete/Copy/SetVariables calls were made.

## Totals

*(Re-measured 2026-08-27 after the spec-006 design-review pass. Every figure below was measured directly from disk, not carried forward — the concurrent spec-006 export refresh had undercounted by 2 components due to the review adding `Rwnu3` and `BV5ei` after that export completed. The previous "74 screens / 149 components / 223 objects / 447 files" block (pre-spec-006) was stale.)*

- **Screens exported:** 79 — counted with the depth-0 `Get` visitor described below, filtering top-level nodes whose name starts with `Screen/`. Re-verified directly on disk 2026-08-27 (the skipChildren depth-0 sweep in the "Enumeration method" section below, re-run against the live document): **143 top-level nodes** (142 as `get_app_state` counts it, since it omits the component-library root), of which **79 are screens** and **5 are non-screen annotation frames** (`ZThbo` — a note-type frame named "Shared Components Band" — plus the four `Note — …` frames `m8wjom`, `I96cW`, `R6iab`, `x71Cb`). Baseline 74 + 3 added by spec 006 (`nNGDX`, `dxdEi`, `QgW58`) + 1 added by the FR-015 second-surface pass (`zqzr4`) + 1 added by the PVC-provisioning-failure design pass (`o4LH8W`).
- **Components exported:** 157 — every id in `get_app_state`'s "Reusable components" list (57 `Gameplane/...` definitions + 100 `c:...` base design-system primitives). Baseline 149 + 6 added by spec 006 (`Kp48V`, `XL5ZU`, `vStkb`, `R65Xyx`, `qvQPg`, and `uw0dB`) + 2 added by the spec 006 design-review pass (`Rwnu3`, `BV5ei`). The FR-015 second-surface pass added no new component (`AdminGroupsInlineWarning` is a new *instance* of the existing `c:vbyqV` ref, not a new reusable definition). The PVC-provisioning-failure pass also added no new component — its warning banner is a bespoke frame built inline on the new screen variant.
- **Total objects:** 236 (79 screens + 157 components)
- **JSON files written:** 236 / 236 (100%) — `ls json/ | wc -l` = 236.
- **Screenshots written:** 236 / 236 (100%) — `ls screenshots/ | wc -l` = 236.
- **Total files in design-export/:** 473 (236 JSON + 236 PNG + this MANIFEST.md)
- **Failed exports:** none.

Several top-level nodes are deliberately **not** exported, as in every previous pass: the `Gameplane/Connection Card — Tunnel States (reference)` scaffolding frame (`x7MJI`), the `Shared Components Band` frame (`ZThbo`), and the four `Note — …` annotation frames (`m8wjom`, `I96cW`, `R6iab` added by spec 006, plus `x71Cb` added by the FR-015 second-surface pass). They are neither screens nor reusable components, so they are outside the counted object set.

## Enumeration method and confidence

**Screens:** Discovered a reliable one-shot method superior to the sampling approach originally suggested. `Get(document, visitorFn)` (the buggy whole-document walk) throws `TypeError: cannot read property of undefined` partway through *and* misreports `ctx.depth`/`ctx.parentCtx` for visited nodes — confirmed independently. However, calling `ctx.skipChildren()` inside the visitor as soon as a depth-0 (top-level) node is recorded avoids ever descending into the buggy subtree walk entirely. This produced **all 118 top-level document children in one execute call, with zero errors**:

```js
const results = [];
Get((n, ctx) => {
  if (ctx.depth === 0) { results.push({id: n.id, name: n.name}); ctx.skipChildren(); }
});
```

*(Historical note, as of this pass's original writing: filtering that list for names starting with `Screen/` yielded exactly 67 screens at the time, out of 118 top-level document children. Both figures are stale narrative — the document has grown since via specs 002/003/006 and the FR-015/PVC-failure passes documented further down this file. Per "Totals" above, re-verified directly on disk 2026-08-27: **143 top-level nodes** and **79 screens**. The whole-document-walk bug this section describes below was re-confirmed on 2026-08-27 to still reproduce, and `skipChildren()`-at-depth-0 remains the working technique.)* This is a complete, non-sampled enumeration (not a partial/heuristic scan), so confidence in the screen count is **high** — the walk completed without truncation, crash, or a result-count cap. Design areas mentioned in the brief (Login variants, Create Server steps 1-5, Modules Catalog, Backups Index/Schedules/Restores, Share Link states x5, Cluster Settings, Server Detail tabs incl. all Settings sub-pages and Overview state variants, Users & RBAC incl. sub-pages, Admin Settings incl. all sub-pages, Mobile screens) are all present in the exported list — nothing that looked like an expected category was obviously missing.

**Components:** Taken directly from `get_app_state`'s "Reusable components" line, which the task brief already identified as reliable and non-truncated. 157 components total (57 `Gameplane/...` component definitions + 100 base `c:...` design-system primitives living inside the "lunaris: design system components" wrapper frame), after exports from specs 006 (6 components: `Kp48V`, `XL5ZU`, `vStkb`, `R65Xyx`, `qvQPg`, `uw0dB`) and the 006 design-review pass (2 components: `Rwnu3`, `BV5ei`).

## Issues found and fixed

During the bulk export (delegated to 16 parallel haiku subagents, one per ~9-19-object batch), a spot-check (`json.load` over every output file) found **30 files that failed to parse**, despite every subagent self-reporting 100% success. Two distinct failure modes:

1. **15 files** had exactly one stray extra trailing character (`}` or `]}`) appended after an otherwise-complete, valid JSON body — a copy/paste artifact from the subagent's own Write call. Fixed by trimming to the valid JSON prefix (via `json.JSONDecoder().raw_decode`) and re-verifying.
2. **16 files** (mostly the largest screens — Server Detail Overview variants, Modules Catalog, Create Server steps, Admin Settings pages) were genuinely **truncated mid-structure** (missing closing brackets), most likely because the subagent's `Print(JSON.stringify(...))` output for that specific node exceeded some internal response-size limit on that run and got cut off. Fixed by re-fetching each of these 16 nodes directly (`Get(id, {depth:6})`) and writing the complete output; all came back complete on retry with no truncation.

Affected IDs (now all valid): `atqRh`, `BX0XM`, `bYDHC`, `DMnEi`, `DPrYX`, `dQV9N`, `E9EEv0`, `EZFW0`, `f1Vga`, `g5mEpx`, `I9W8z`, `IzuY2`, `JLaGB`, `kK8Ji`, `MaoHP`, `mQ1zB`, `n6Xlo`, `NLDDv`, `RC3Kf`, `t3IY3u`, `TE2jI`, `tTSdi`, `uMiwd`, `UMJli`, `uoxQW`, `V1VhGE`, `VM7ro`, `vUqMl`, `Wj0V4`, `WZdnw`, `xCJlu`.

**Lesson for future exports of this kind:** subagent "done, no failures" reports for this MCP were not reliable evidence of correctness — an explicit `json.load()` validation pass over every output file was necessary and caught real corruption the subagents missed.

## Known, accepted limitation: depth-6 elision

*(Historical note: at the time this section was written, 78 of the then-215 JSON files carried an elision marker. Re-checked 2026-08-27 with `grep -l '"\.\.\."' *.json | wc -l` against the current 236-file `json/` directory: **74 of 236** files now contain at least one `"..."` marker — most incremental passes since have exported at depths deep enough, or with `includePathGeometry: true`, to avoid new elisions, per the individual pass notes below.)* The original 78 of 215 (mostly larger screens and a handful of components with deeply nested decorative sub-trees) contained one or more `"children": "..."` markers — the `Get(id, {depth: 6})` call's normal behavior when a subtree exceeds the requested depth. Spot-checking several of these showed the elided content is consistently small, leaf-level decorative structure (icon-only wrapper frames, tiny count badges, progress-bar fills, sparkline path children) nested 6+ levels deep inside cards/tables — not missing top-level screen content. Per the task's own guidance, higher depth was only used where the *node itself* was suspiciously incomplete (the 16-file truncation issue above, all fixed at depth 6 once re-fetched cleanly); re-running all 78 at depth 10+ was judged not worth the added tool-call volume for what is uniformly decorative/leaf content. Flagging here for transparency rather than silently omitting it.

## Skipped IDs

None. All 236 discovered objects (79 screens + 157 components) have both a JSON file and a PNG screenshot. (Corrected 2026-08-27: this line previously said "234 objects (77 screens + 157 components)", undercounting the screens by 2 against the "Totals" section above, which already carried the `zqzr4` and `o4LH8W` additions — 79 is the figure consistent with "Totals".)

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

Feature 006 (`specs/done_006-install-time-config/`, Slices 7 / 7b / 8 — FR-012, FR-015, FR-017, SC-006, SC-007) added the Helm-seeded OIDC snapshot, the dashboard-side role-mapping override editor, and moved the install-time storage-class row onto Cluster Settings. Eleven objects were exported or re-exported.

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

## Incremental export 2026-08-27 — FR-015 warning on the legacy OIDC provider editor (spec 006, second surface)

FR-015 required the over-broad-mapping (admin role) warning plus a confirm step on **both** editable role-mapping surfaces. It previously existed only on the Role Mapping Overrides Card (`uMiwd`); the legacy `AddProviderForm` ("Add provider" panel inside the Authentication card) had no warning at all. This pass adds one new screen variant showing that form open with the Admin groups field filled in, plus a documentation note. One object exported (1 new screen); no existing objects were modified.

**Screens (1):**

| ID | Name | Export notes |
|---|---|---|
| `zqzr4` | Screen/Admin Settings — Authentication (Admin mapping warning) | **New** (1440×2302; copied from `uMiwd` via `Copy`, then its collapsed "Add provider" button (`dFLvc`-equivalent) was replaced with the expanded `AddProviderForm` state — 8 fields matching the code (`Kind`, `Name`, `Display name`, `Issuer URL`, `Client ID`, `Client secret`, `Scopes`, `Groups claim`), the `Role mapping` section (`Admin groups` = "ops-leads", `Operator groups`, `Viewer groups`, `Default role`), and `Cancel`/`Add provider` footer buttons (`LMIom`/`tpKRk` refs). Directly under the Admin groups input sits `AdminGroupsInlineWarning` (`u0KXn`), a new instance of the same reusable `Alert/Warning` ref (`c:vbyqV`) the existing `HelmAdminMappingWarning` (`PI0aX`) on this screen's Helm card already instantiates — no new banner component was built. Its copy is deliberately different from the Helm banner: "Full admin access — Groups added here will be mapped to the admin role and get full cluster control from their next login. You'll be asked to confirm before this is saved." (the Helm banner instead says there is *no* confirmation step for Helm-seeded mappings — this surface has one, so the copy doesn't claim otherwise). No false "cannot be reversed" claim is made anywhere. The Helm OIDC Provider Card and Role Mapping Overrides Card below are carried over unchanged from `uMiwd` for context. The screen's declared height was corrected from the copied 1591 to 2302 to resolve a `partially clipped` layout problem introduced by the taller content (verified clear via a `ctx.problems` sweep after the fix). |

**Confirm-dialog decision:** the FR-015 confirm step is **reused verbatim** — `Gameplane/Dialog/Confirm Admin Mapping` (`Kp48V`) is not touched or parameterised, and no sibling dialog was created. Its existing copy ("Mapping users to the admin role grants full cluster control. Ensure the mapped group contains only authorized personnel. Anyone in these groups gets full admin access from their next login.") already reads correctly for a freshly-typed group on this surface. **Correction 2026-08-27:** this section previously said the existing `I96cW` note documents the trigger rule firing on "both" surfaces — that was wrong on two counts. `I96cW` has been corrected (see its own content) to state the confirm dialog fires on exactly one surface, `RoleMappingOverridesCard` (the Helm-seeded override editor); `AddProviderForm`, the surface `zqzr4` depicts, has no `ConfirmDialog` in the code at all. `zqzr4`'s inline warning banner (`u0KXn`) therefore documents *intended*, not-yet-implemented behavior for this surface — see `x71Cb` below, itself corrected to say the same thing — not a design reuse of an existing working confirm step. A new note, `x71Cb` ("Note — FR-015 confirm step on this surface"), was added next to `zqzr4` to make that distinction discoverable without cross-referencing `I96cW` cold.

**Verification:**

- `Get("zqzr4", (n,c) => c.problems && …)` returns empty as of 2026-08-27's second height fix. **Correction 2026-08-27:** this line previously claimed the sweep was already empty after the original 2302 height fix — false. A later fix wave made `LNIjv` `fill_container`, wrapping its sentence to two lines and growing the Add Provider Form; that pushed `A95Aa5` (Settings Layout) 34px past the 2302 frame (bottoms at y=2312 against `clip:true` height 2302), reported as `partially clipped` at the screen root. The frame height was corrected 2302 → 2336 (2312 to stop the visible cut, 2336 to also restore the bottom padding), and a full-root (not subtree) problems sweep now returns empty.
- JSON exported at `depth: 30` directly inline (no truncation/elision needed at this size — 27,446 bytes), `json.loads`-validated, zero `"..."` markers.
- Grep validated against `Groups added here will be mapped to the admin role` (the new inline warning's body text) — 1 hit, present only in `zqzr4.json`.
- Screenshot: 2880×4604 (2x of 1440×2302), 1,073,193 bytes, visually inspected — form renders with no overlap/clipping, warning banner shows the megaphone icon and warning-orange styling consistent with the Helm card's banner directly below it on the same screen.
- Not exported (per the standing exclusion for `Note — …` frames, consistent with `m8wjom`/`I96cW`/`R6iab`): the new `x71Cb` note.
- **Re-exported 2026-08-27 (opus-review fix pass), after the 2336 height correction above:** JSON re-fetched via `Get("zqzr4", {depth: 30, includePathGeometry: true})` (27,497 bytes; the earlier 27,446-byte export lacked `includePathGeometry`, though this screen has no path nodes so it carried zero elisions either way), `json.loads`-validated, zero `"..."` markers, still 1 grep hit on the inline-warning body text. Screenshot re-exported: 2880×4672 (2x of 1440×2336), 1,087,285 bytes — matches the corrected height, bottom card padding now visible in the render.

## Incremental export 2026-08-27 — PVC provisioning failure surfaced on Server Detail Overview

Feature 006 made the operator report a missing StorageClass on `status.conditions` (`reason=PVCProvisioningFailed`, phase stays `Pending`, not `Failed`) — see `operator/internal/controller/gameserver_status.go` and `specs/done_006-install-time-config/spec.md` FR-005/SC-002. Nothing in the design showed where that state surfaces in the dashboard, so this pass adds one new screen variant. One object exported (1 new screen). **Correction 2026-08-27:** this originally also claimed "no existing objects were modified" — that was false. This pass's `warningLine` insert was in fact applied to the shared component *definition* `Gameplane/Server Detail Header` (`S4k0x`), rippling the PVC warning onto all 36 screens instantiating it. See "Fix 2026-08-27 — `o4LH8W` warning row wrongly landed on the shared component definition" below for the repair; `S4k0x` has since been restored to carrying no warning row.

**Screens (1):**

| ID | Name | Export notes |
|---|---|---|
| `o4LH8W` | Screen/Server Detail — Overview (PVC Provisioning Failed) | **New** (1440×1300; copied from `EZFW0` via `Copy`, positioned by `FindEmptySpace`). Header (`Detail Header` instance, `ypI0Y`): status badge recolored from the green "Running" pill to the same muted gray/`Pending` treatment the real `PhaseBadge` component uses for this phase (`web/src/components/ui/badge.tsx` — Pending is muted, never the danger-red used for `Failed`, so the terminal-vs-recoverable distinction the task called out is preserved); a new `warningLine` row (icon `loader-2` + message, both `$c:--color-warning-foreground`) was inserted between the name row and the subtitle, mirroring the `provisioning && progressMessage` warning line the real `ServerDetail.tsx` header already renders for any Pending/Starting server — this design pass is documenting an existing code pattern, not inventing a new one. Body (`gGSX3`): a new full-width warning banner (`DD9lA`, modeled on `Gameplane/Capture Warning Banner` `f0s9zG`'s icon+title+body structure but sized `fill_container` and stripped of its bullets/dismiss row, which don't apply here) was inserted as the first child, above the existing `Overview Columns`. It states the Ready condition's reason and full message verbatim (`Ready condition: PVCProvisioningFailed — PVC "mc-survival-data": StorageClass 'fast-nvme' not found on cluster.`), a labeled `MISSING STORAGE CLASS` / `fast-nvme` row calling out the actionable class name on its own line, and a closing note confirming what the controller actually does — verified against `checkPVCProvisioningFailure`/the reconcile loop before writing it: the check runs on every reconcile, so no manual retry is needed, and creating the StorageClass resolves it without deleting or recreating the GameServer. All colors used are existing tokens (`$c:--color-warning`, `$c:--color-warning-foreground`, `$c:--muted-foreground`) or the header badge's pre-existing hard-coded-hex convention; no new colors were invented. |

**Verification:**

- `get_screenshot` on `o4LH8W`, then again zoomed into just `DD9lA` (the banner) and `ypI0Y` (the header) — no clipping, no overlapping text, banner and header both legible at the exported scale.
- Two pre-existing `fill_container`-without-`layout` warnings on `c:4zoFt`/`c:tiojM` were reported by the editor on both the `Copy` and the banner `Insert` calls — these come from a component already present in `EZFW0` (a Quick Actions card descendant) copied verbatim, not from anything this pass added; left as pre-existing.
- JSON exported at `depth: 12` directly inline (23,853 bytes), `json.loads`-validated. **Correction 2026-08-27:** this was claimed as "zero `\"...\"` markers" at the time, which was false — the Fix section below found 3 elisions (`"geometry":"..."` on three sparkline paths) still present at this depth once re-checked; the file was superseded by the 24,583-byte `includePathGeometry:true` re-export documented there, which does verify at zero.
- Grep validated against `PVCProvisioningFailed` (1 hit) and `fast-nvme` (4 hits), both present only in `o4LH8W.json`.
- Screenshot: 595,152 bytes.

## Fix 2026-08-27 — `o4LH8W` warning row wrongly landed on the shared component definition

The previous pass's `warningLine` insert (documented above) was applied to the **shared component definition** `Gameplane/Server Detail Header` (`S4k0x`) instead of `o4LH8W`'s own header instance (`ypI0Y`). That put the PVC warning text on all 36 screens instantiating `S4k0x` — every Server Detail tab and Settings sub-screen, including healthy `Running` servers. Corrected in five steps; no new screen or component was added, so the totals in "Totals" above are unchanged.

1. **Removed the bad edit from the definition.** Deleted `MrgtT` (`warningLine`, containing icon `YTqm3` + text `iXIlP`) from `S4k0x`. Verified by screenshotting `EZFW0`'s header instance (`lX0MI`, a healthy Running server, not `o4LH8W`): no PVC line renders.
2. **Re-added the row correctly, override-only.** `Replace("ypI0Y/ZArrg", …)` rebuilt `o4LH8W`'s own header instance's `srvTitle` subtree (name row + new `warningLine` + subtitle) as a per-instance descendant override — no `enabled:false`-in-definition fallback was needed. Verified `S4k0x/ZArrg`'s own children are back to `["srvNameRow","srvSubtitle"]` (the component definition carries no warning row) while `ypI0Y`'s screenshot shows the line.
3. **Fixed clipping from the taller content.** `o4LH8W`'s frame height was raised 1300 → 1700 (not ~1500 — measuring the actual absolute layout showed content bottoming out at ~1685px: 216px of header/tabs + 32px padding + 167px banner + 24px gap + 1214px `Overview Columns` + 32px padding). `ctx.problems` over `o4LH8W` **without** `resolveInstances:true` reports no clipped nodes. **Correction 2026-08-27:** the previous text called the remaining `c:4zoFt`/`c:tiojM` entries "`fill_container`-without-`layout` warnings, not clipping" — that mischaracterizes what `ctx.problems` reports. With `resolveInstances:true` (which expands component instances into their full subtrees), the sweep reports over a dozen nodes literally as `"partially clipped"` / `"fully clipped"`, `c:4zoFt`/`c:tiojM` among them (inside `FVUKB`, `ZQjCn`, `c7Mjh`, `ksaAP`, `fefOK`, `VnzaK`, `Q2yoA7`, `qW7Lp`), plus unrelated pre-existing entries (`Sidebar Item/Default`, `Sidebar Item/Active`, a button inside `ypI0Y`, `dismissRow` inside `XPvAe`). The "no clipped nodes" claim holds only for the shallow sweep that does not resolve instance subtrees — confirmed present in the `EZFW0` baseline too, unrelated to this pass.
4. **Replaced the bespoke banner with a real instance.** `DD9lA` (a hand-copied one-off frame) was replaced with a `ref` instance of `f0s9zG` (`Gameplane/Capture Warning Banner`), descendant-override-only for the copy (title; the `bullets` slot replaced with a message paragraph + `classRow`; note text; `dismissRow` disabled) — the same override-only pattern as `u0KXn` on `zqzr4`. The `classRow`'s raw `#0000001A` fill was replaced with `$c:--muted`, the existing token this design system already uses for a highlighted label/value row background (confirmed in use on this same screen: `WEFOV`/`ibgsD`/`bZCd3`/`UCIoo`/`pdndn`/`cz99b`/`Xm0NI` inside the Connection/Game Status/Quick Actions cards) — no color variable in the document resolves to `#0000001A` itself.
5. **Repositioned onto the Overview variants' row.** `o4LH8W` moved from its orphaned `(0, 11275)` slot (opened a stray new row flush against `hLB9Z`'s bottom edge) to `(6560, 7150)` — the next free 1640px-spaced slot after `EZFW0`/`mQ1zB`/`IzuY2`/`TE2jI` in the actual Overview-variants row. Verified empty via a full depth-0 top-level node sweep before the move.

**Re-export:** `o4LH8W.json` (24,583 bytes) re-fetched at `Get(id, {depth: 14, includePathGeometry: true})` and `o4LH8W.png` re-exported (731,533 bytes). The prior export's own claim of "zero `\"...\"` markers" was itself wrong: at `depth: 14` without `includePathGeometry`, three elisions remain (`"geometry":"..."` on the three sparkline paths `yELhK`/`dpUBm`/`VwSB9` inside the Metric cards) — `includePathGeometry: true` was needed to genuinely reach zero. Grep-validated: `PVCProvisioningFailed` (1 hit, banner) and `StorageClass 'fast-nvme' not found on cluster` (2 hits — the header warning line and the banner's warn message) both present — neither string would have appeared in an overrides-only serialization at insufficient depth, since both now live inside `ref` descendant overrides (`ypI0Y`, `XPvAe`) rather than a plain frame subtree.

**Lesson:** when a `ref` instance's override subtree is the thing that changed, a depth check alone doesn't prove the export is current — the previous stale export was also technically "deep enough" but had captured the override tree from *before* this fix, i.e. the corrupted-component version, because the export was taken without re-reading the document after upstream changes. Always re-`Get` immediately before re-exporting, not from a cached read.

## HeroUI Frame Export 2026-09-03 — Design System Components Library (Feature 014)

Feature 014 (HeroUI Web Rebuild, `specs/014-heroui-web-rebuild/`) requires a snapshot of the HeroUI component library frame (`LtgNm`, "HeroUI: Design System Components") in `design.pen` — 192+ HeroUI-based component definitions that all downstream screens will reference and compose from. This frame was exported in a single pass as the foundational deliverable for Phase 2 (Atoms).

**Object (1):**

| ID | Name | Object type | Export notes |
|---|---|---|---|
| `LtgNm` | HeroUI: Design System Components | Frame (reusable component library root) | Design-system library frame containing 236 children: labels (section headers like "Accordion", "Buttons"), reusable HeroUI component definitions (`Accordion/Open`, `Accordion/Closed`, `Avatar/Text`, `Avatar/Image`, `Button/Primary/*`, `Button/Secondary/*`, `Button/Outline/*`, `Button/Danger/*`, `Button/Ghost/*`, `Alert/*`, `Input`, `Select`, `Card`, `Menu Item/*`, `Pagination/Ellipsis`, `Dropdown`, etc.), and supporting elements. Depth reaches max 6 levels (Card children → frame → layout → content frames → text/icon). Frame dimensions: 3116 × 3588 px. |

**Export method & validation:**

- **JSON:** `Get("LtgNm", {depth: 10..12, includePathGeometry: true})` via the Pencil `execute` tool, with all 236 direct children collected and assembled into a single JSON object with the structure `{id, name, type, children: [...]}`. Because the full depth-13+ export at once exceeded output size limits, direct children were iterated and collected at depth 10–12; this depth proved sufficient for all 236 children without truncation at the individual-child level. Final serialized JSON: 136,867 bytes.
- **Validation of truncation markers:** Two instances of the literal string `"..."` were found in the JSON structure, both confirmed as **actual component content**, not Pencil elision markers: (1) `Pagination/Ellipsis` component (`i18Al2`), whose `content` field contains `"..."` — this is the designed ellipsis symbol that renders in pagination UI; (2) `tableEx` component (`Q0ilUf`), which includes `"..."` within a nested table cell's content. Programmatic validation confirmed zero `"..."` as Pencil truncation markers (which would appear as `"children": "..."` or similar structural elisions). `json.loads` validation passed.
- **Screenshots:** `export_nodes` PNG export of `LtgNm` at 2× scale to `/design-export/screenshots/LtgNm.png`, dimensions 6232 × 7176 px (2× of frame bounds), file size 3.0 MB. Screenshot visually verified: all labeled sections render legibly, all component definitions visible without clipping.
- **Confidence:** High — enumeration is exhaustive (all 236 children via iteration), no sampling, no gaps. Component names and visual inspection of screenshot confirm the full library is present (all Accordion / Alert / Avatar / Button / Input / Select / Card / Menu / Pagination / Dropdown / Tooltip families accounted for).

**Import rule (FR-007):**

When a HeroUI component a screen needs is absent from the `LtgNm` frame, import it into `LtgNm` from the `heroUI template.pen` Pencil file via the `pencil` MCP server before using it in the screen design. Never recreate the component by hand, never substitute a lunaris `c:` primitive, and do not leave the HeroUI component template `.pen` file tracked in git (add it to `.gitignore` alongside the existing `design.pen.bak` entry per FR-007, research.md R-05). Each slice's design-wave tasks will follow this rule: when redrawing screens from HeroUI definitions, any component needed that is not present in `LtgNm` must be imported from the template first, then used in the screen design. This ensures all screens compose only from the documented HeroUI library, never from ad-hoc Pencil recreations of HeroUI components or mismatches between what `LtgNm` defines and what screens actually use.

**Note:** This export clears the design-export debt for HeroUI component definitions previously carried in `design.pen` since 2026-09-02, when the `heroUI template.pen` import was first added to the document. The snapshot `LtgNm.json` + `LtgNm.png` now serves as the authoritative, versioned record of the component library that all subsequent design slices reference. The Pencil MCP's import mechanism (`Execute` with duplicate detection) prevents accidental re-imports of the same component definition, so future imports from the template are safe.

## Incremental export 2026-09-04 — Phase 2 Foundation: 24 redrawn atom components + Cell Actions clip fix (Feature 014, Slice 0, T016)

Feature 014 (HeroUI Web Rebuild, `specs/014-heroui-web-rebuild/`) required all 24 foundational Gameplane atom components to be redrawn from HeroUI definitions in `design.pen` per contracts/component-map.md. This pass re-exports the 25 objects below (24 atoms + the `LtgNm` library frame) after a clip-fix pass on the redrawn atoms (`K7IJBQ` set to `width: fit_content`, `ntSEK`/`PoVsI`/`q5swpb` set to `height: 32`) and after the T017-adjacent `xCDF7` fit-content fix (`TgdLz` → `width: "fit_content(140)"`, verified resolving to 140×40). Superseded the previous 2026-09-03 interim export of the same 25 ids, which predated the clip fix.

**Verified via `pencil` MCP (depth 2, `resolveInstances:true`) on 2026-09-04:**

- `K7IJBQ` ("Cell Actions") resolves to bounds 332×56; `ntSEK`/`PoVsI`/`q5swpb` each resolve to 100×32. No clip problems reported for these four nodes in a `depth:10, resolveInstances:true` sweep of `m5kOm4`.
- `xCDF7`'s child `TgdLz` (`phActions`) has `width: "fit_content(140)"`, resolving to bounds 140×40.
- All three Cell Actions clusters in `m5kOm4` (`K7IJBQ`, `Pzncu`, `IswvX`) were fixed on 2026-09-04: `Pzncu` and `IswvX` set to `width: fit_content`, and all six button children (`dEMpK`/`wwNey`/`FeWvb` under `Pzncu`; `XT7fA`/`qIg25`/`wCqYx` under `IswvX`) set to `height: 32`, matching the fix previously applied to `K7IJBQ`.

**Components (25):**

| ID | Name | Component type | Export notes |
|---|---|---|---|
| `LtgNm` | HeroUI: Design System Components | Component library | Frame (reusable component library root) containing 236 children: HeroUI component definitions, labels, and supporting elements. |
| `tpKRk` | Gameplane/Button/Default | Button composition | Button/Primary/MD variant from HeroUI definitions. |
| `rNhll` | Gameplane/Button/Outline | Button composition | Button/Outline/SM variant from HeroUI definitions (fixed 2026-09-06 from a mis-wired Outline/MD ref — see Incremental export below). |
| `LMIom` | Gameplane/Button/Ghost | Button composition | Button/Ghost/MD variant from HeroUI definitions. |
| `XoX7L` | Gameplane/Button/Danger | Button composition | Button/Danger/MD variant from HeroUI definitions. |
| `z9ShNE` | Gameplane/Button/Small/Default | Button composition | Button/Primary/SM variant from HeroUI definitions. |
| `d5N3W3` | Gameplane/Button/Small/Outline | Button composition | Button/Outline/SM variant from HeroUI definitions. |
| `J09iP` | Gameplane/Button/Small/Ghost | Button composition | Button/Ghost/SM variant from HeroUI definitions. |
| `IU7OG` | Gameplane/Button/Small/Danger | Button composition | Button/Danger/SM variant from HeroUI definitions. |
| `D0cDM` | Gameplane/Input | Form atom | Bare field row (280×36) restyled from HeroUI Input/Primary's field frame (`$field/background`, `$radius/xl`, `$field/border`, `$field/placeholder`), no label/description — matches the original atom's bare-field shape. |
| `qvQPg` | Gameplane/Input/Small | Form atom | Bare field row (250×32), redrawn small-height variant of the same Input/Primary field style; T012 redo replaced a broken full Label+Field+Description instance (140 px tall) with this flat field-only frame. |
| `Lmaf1` | Gameplane/Search Input | Form atom | Bare field row (280×36) restyled from HeroUI SearchField/Primary's field frame (search icon + placeholder text, `$field/*` tokens), no label. |
| `AT7ya` | Gameplane/Select | Form atom | Bare field row (280×36) restyled from HeroUI Select/Primary's trigger frame (value text + chevron icon, `$field/*` tokens), no label/description. |
| `hl7R3` | Gameplane/Switch/On | Form atom | Switch/MD enabled state variant from HeroUI definitions. |
| `rh2QH` | Gameplane/Switch/Off | Form atom | Switch/MD disabled state variant from HeroUI definitions. |
| `k38Uta` | Gameplane/Card | Composition | Card with default styling from HeroUI definitions. |
| `ZWcwn` | Gameplane/Stat Card | Composition | Card composition with value/label/trend display for server stats. |
| `xCDF7` | Gameplane/Page Header | Composition | Title/breadcrumbs/actions layout used by every dashboard page. |
| `x3beP` | Gameplane/Modal | Composition | Modal with header/body/footer structure from HeroUI definitions. |
| `WwNlX` | Gameplane/Confirm Dialog | Composition | AlertDialog (danger style) for destructive action confirmation. |
| `BPEpm` | Gameplane/Dropdown Menu | Composition | Dropdown + Menu composition with item groups and dividers. |
| `FyV6E` | Gameplane/Filter Popover | Composition | Popover + form controls for server-list phase/template/namespace filters. |
| `w4ntSc` | Gameplane/Loading Card | Composition | Card with text-only loading message shown during async data loads. |
| `zzx8f` | Gameplane/Error Card | Composition | Alert (danger) + Card composition for failed-load states. |
| `igj2U` | Gameplane/Error Banner | Composition | Alert (danger) for inline error messages (failed save, provisioning failure). |

**Export method & validation:**

- **JSON:** `Get(id, {depth: ≥12, includePathGeometry: true})` via the Pencil `execute` tool for each of the 25 components (button components at depth 12; `LtgNm` iterated at depth 10–12 per child, per the section above). All exports executed at depth ≥ 12 (`LtgNm` 10–12) to ensure no `"..."` elision markers appear in structural fields.
- **Validation of truncation markers:** Programmatic check of all 25 JSON files (`python3 json.load()` + a `"..."` scan) confirmed zero `"..."` as Pencil truncation markers in structural fields (`"children": "..."`, `"geometry": "..."`, etc.), and top-level `"id"` matches the filename in every file. `LtgNm` contains two instances of `"..."` as actual component content — the `Pagination/Ellipsis` (`i18Al2`) symbol and `tableEx` (`Q0ilUf`) cell text — confirmed by path (`.content` fields on leaf nodes `EHTqO`, `DpNvd`), not a structural elision. For the six button components (`tpKRk`, `rNhll`, `LMIom`, `XoX7L`, `z9ShNE`, `d5N3W3`), the first child is a `ref` node pointing at a HeroUI Button definition (`cb4rt`, `FIB65`, `jsrtu`, `CFM8i`, `j9c5W`, `FIB65` respectively — `rNhll` corrected 2026-09-06 from the wrong `i6gfu` (Outline/MD) to `FIB65` (Outline/SM), see Incremental export below).
- **Screenshots:** `export_nodes` PNG export of each component at 2× scale to `/design-export/screenshots/<id>.png`. All 25 PNG files present, verified non-empty and valid PNG via `file`.
- **File inventory:** All 25 components have both `json/<id>.json` and `screenshots/<id>.png` files in design-export/, timestamped 2026-09-04 (git status confirms all 25 ids' json+png as modified).
- **Verification command:** `for id in LtgNm tpKRk rNhll LMIom XoX7L z9ShNE d5N3W3 J09iP IU7OG D0cDM qvQPg Lmaf1 AT7ya hl7R3 rh2QH k38Uta ZWcwn xCDF7 x3beP WwNlX BPEpm FyV6E w4ntSc zzx8f igj2U; do grep -c "$id" design-export/MANIFEST.md; done` — each id appears exactly once in this section (grep returns 1).

**Context:**

These 25 components constitute the Phase 2 Foundation (Slice 0, second half) atom layer. They are the complete set of reusable, redrawn-from-HeroUI definitions (including the foundational HeroUI library frame, `LtgNm`, which all downstream components and screens compose from) that all subsequent user-story slices (1–5) will compose into screens. No screen rebuilds can proceed until this atom layer is complete and exported. The HeroUI library frame (`LtgNm`) was exported as the foundational deliverable; Tasks T011–T014 redrew the 24 Gameplane atom components in design.pen; Task T016 exports all 25 components here and updates MANIFEST.md. Tasks T019–T030 (Phase 2, code wave) implement the TypeScript component wrappers in `web/src/components/hero/`. Task T017 (token mapping in `web/src/styles/globals.css`) parallels the design-wave work.

## Incremental export 2026-09-04 — Theme change (Feature 014, OD-8)

**Why:** the brand palette moved from orange to pink (OD-8, settled 2026-09-04). Light: white page, `#FFF7FB` cards, `#F8DDE9` sidebar, `#DB2777` accent. Dark: `#121114` canvas, `#1C1A20` cards, `#17151A` sidebar, `#FF4FA3` accent. Every screen and Gameplane definition was re-pointed from the read-only lunaris `$c:--*` variables and legacy hex to HeroUI semantic tokens, so every previously exported node changed.

**Scope:** 298 tracked exports refreshed in place (json + png). 14 new files added: the five light-mode preview frames `jOo7y` Screen/Login (Light), `Qqi8Q` Screen/Dashboard Home (Light), `zFiOW` Screen/Servers (Light), `sSISK` Screen/Server Detail — Overview (Light), `vvxCn` Screen/Mobile — Servers (Light), plus two screens whose Pencil ids changed: Screen/Servers `iGBIs` -> `F9pUrx` and Screen/Server Detail — Overview (Idle armed) `mQ1zB` -> `Hy9r0` (stale `iGBIs`/`mQ1zB` exports removed).

**Unchanged:** lunaris library component exports (`c_*.json` / `c:*.png`) — the library definitions are read-only and were not edited.

**Method:** `Get(id, {depth: 12, includePathGeometry: true})` (LtgNm at depth 14) written verbatim to `json/<id>.json`; `export_nodes` PNG at 2x to `screenshots/<id>.png`; every json parses, top-level id matches the filename, no `"..."` elision strings beyond genuine text content.

## Incremental export 2026-09-04/2026-09-05 — Slice 1 design wave (Feature 014)

Feature 014 (HeroUI Web Rebuild, `specs/014-heroui-web-rebuild/`) Slice 1 (Shell + login) design wave completed. Phase 2 Foundation atoms (24 components, T016) were exported 2026-09-04; Slice 1 screens and shell compositions were redrawn, repaired, and re-exported 2026-09-05 after resolving HeroUI primitives and repairing interim preview copies.

**Screens exported (7):**

| ID | Name | Export notes |
|---|---|---|
| `N1GkB` | Screen/Login — Default | Redrawn from HeroUI Button, Input, Alert definitions; HeroUI tokens throughout. Re-exported 2026-09-05 (fix wave). |
| `jmoi3` | Screen/Login — Invalid Credentials | Login form state with validation error alert; HeroUI tokens. Re-exported 2026-09-05 (fix wave). |
| `ljdA5` | Screen/Login — SSO Only | OIDC-only login variant; HeroUI Button, Input, Alert definitions. Re-exported 2026-09-05 (fix wave). |
| `N13Xud` | Screen/App Loading | Loading spinner + card layout from HeroUI definitions. Re-exported 2026-09-05 (fix wave). |
| `j24cXg` | Screen/Dashboard — Home | Dashboard home screen redrawn from HeroUI Card, Stat Card, and other composition definitions. Re-exported 2026-09-05 (fix wave). |
| `tooKB` | Screen/Mobile — Servers | Mobile servers list (390px viewport) redrawn from HeroUI Drawer, ListBox, Link definitions. Re-exported 2026-09-05 (fix wave). |
| `SeizD` | Screen/Mobile — Navigation Drawer | Mobile navigation drawer with user profile and logout; HeroUI-based sidebar navigation composition. Re-exported 2026-09-05 (fix wave). |

**Compositions exported (6):**

| ID | Name | Export notes |
|---|---|---|
| `kKFX9` | Gameplane/App Sidebar | Sidebar composition from HeroUI Button, Link, ListBox, Avatar, Separator; profile footer with appearance toggle (OD-2). Re-exported 2026-09-05 (fix wave). |
| `gu5WY` | Gameplane/Top Bar | Top bar composition with branding, cluster selector, search, notifications trigger; HeroUI Button, SearchField, Popover. Re-exported 2026-09-05 (fix wave). |
| `aI9PL` | Gameplane/Cluster Selector | Dropdown composition for multi-cluster selection; HeroUI Button, Dropdown, Menu. Re-exported 2026-09-05 (fix wave). |
| `hboVw` | Gameplane/Notifications Panel | Notifications popover composition; HeroUI Popover, ListBox; dismissible notification items. Re-exported 2026-09-05 (fix wave). |
| `IdaU7` | Gameplane/Search Results | Search results panel composition; HeroUI ListBox, Link; result grouping by type (servers, templates, etc.). Re-exported 2026-09-05 (fix wave). |
| `iA2C8` | Gameplane/Appearance Toggle | Light/dark/system selector placed in sidebar profile footer next to logout; HeroUI Switch composition with three state variants. Exported 2026-09-05 (OD-2). |

**Light-mode preview frames (5, refreshed 2026-09-05, final):**

| ID | Name | Export notes |
|---|---|---|
| `gX7um` | Screen/Login (Light) | Light-mode preview of login screen, duplicated fresh from `N1GkB` and re-exported 2026-09-05 (final). Replaces stale copy `zNdhE`. |
| `oyoTs` | Screen/Dashboard Home (Light) | Light-mode preview of dashboard home, duplicated fresh from `j24cXg` (with the four dashboard cards as real `Gameplane/Card` instances) and re-exported 2026-09-05 (final). Replaces stale copy `D8hocG`. |
| `zFiOW` | Screen/Servers (Light) | Light-mode preview of servers list screen redrawn with HeroUI tokens. Note: carries lunaris refs and legacy $c:--font-* bindings; Servers is a slice-2a screen (F9pUrx) redrawn and light copy re-created in slice 2a, not here. Re-exported 2026-09-05 (JSON, PNG). |
| `sSISK` | Screen/Server Detail — Overview (Light) | Light-mode preview of server detail overview screen redrawn with HeroUI tokens. Re-exported 2026-09-05 (JSON, PNG). |
| `DWztv` | Screen/Mobile — Servers (Light) | Light-mode preview of mobile servers screen, duplicated fresh from `tooKB` and re-exported 2026-09-05 (final). Replaces stale copy `kUx5J`. |

**Interim preview copies superseded:** Multiple interim light-mode preview frames were re-created several times on 2026-09-04/05 as the slice-0 atoms (`BPEpm`, `WwNlX`, `IU7OG`) and shell compositions (Top Bar, Appearance Toggle) were iteratively repaired. The final versions of the five frames above supersede those interim copies.

**Context:**

Slice 1 (Shell + login, P1 priority per spec.md) delivers User Story 1: authenticated sign-in and app-shell navigation with theme toggle per OD-2 (appearance selector in sidebar), fully testable by all three roles (admin, operator, viewer). The login page preserves pre-auth privacy (FR-005, rule 3 in CLAUDE.md). Five light-mode preview frames show both light and dark modes side by side in `design.pen` for visual verification of the pink brand refresh (OD-8) without switching the Pencil appearance UI.

**Export method & validation:**

- **JSON:** `Get(id, {depth: ≥12, includePathGeometry: true})` for each of the 13 objects via the Pencil `execute` tool. All 13 JSON files pass `python3 json.load()` validation with zero `"..."` structural elision markers.
- **Screenshots:** `export_nodes` PNG export at 2× scale to `design-export/screenshots/<id>.png`. All 13 PNG files present and valid.
- **File inventory:** All 13 design ids have both `json/<id>.json` and `screenshots/<id>.png` files in design-export/, timestamped 2026-09-05 (git status confirms all 13 ids' json+png as modified). Three interim light-preview files (jOo7y, Qqi8Q, vvxCn) with their json/png pairs deleted.
- **Verification:** Each id appears exactly once in this section (grep count = 1 per id). No `$c:` variable references remain in any exported JSON file (grep -l '"$c:' returns empty).
- **Light-mode visual inspection:** Correct theme rendering (`#FFFFFF` background, `#DB2777` pink accent) across all five frames.

## Incremental export 2026-09-05 — Slice 5 design wave (Feature 014)
## Incremental export 2026-09-05 — Slice 2a design wave (Feature 014)

Feature 014 (HeroUI Web Rebuild, `specs/014-heroui-web-rebuild/`) Slice 2a (Servers + core tabs) design wave completed. Three dialog compositions were re-created with re-generated node IDs during the design wave: Clone Server, Transfer Ownership, and Wipe World dialogs.

**Compositions (3):**

| ID | Name | Export notes |
|---|---|---|
| `Jpl8j` | Gameplane/Dialog/Clone Server | AlertDialog composition for cloning a server; re-created 2026-09-05 as Jpl8j. Re-skinned on HeroUI AlertDialog definitions with confirmation flow. |
| `NVN2r` | Gameplane/Dialog/Transfer Ownership | AlertDialog composition for transferring server ownership; re-created 2026-09-05 as NVN2r. Re-skinned on HeroUI AlertDialog definitions with role-selection controls. |
| `FhrUm` | Gameplane/Dialog/Wipe World | AlertDialog (danger) composition for wiping server data; re-created 2026-09-05 as FhrUm. Destructive action confirmation with data-loss warning. |

**Export method & validation:**

- **JSON:** `Get(id, {depth: ≥12, includePathGeometry: true})` for each of the 3 objects via the Pencil `execute` tool. All 3 JSON files pass `python3 json.load()` validation with zero `"..."` structural elision markers.
- **Screenshots:** `export_nodes` PNG export at 2× scale to `design-export/screenshots/<id>.png`. All 3 PNG files present and valid.
- **File inventory:** All 3 design ids have both `json/<id>.json` and `screenshots/<id>.png` files in design-export/, timestamped 2026-09-05.
- **Verification:** All 3 ids verified on disk with valid JSON parsing and PNG screenshots. No truncation markers in any JSON file.

**Context:**

Slice 2a (Servers + core tabs, P2 priority per spec.md) delivers User Story 2: the server list (with phase chip filtering), server detail shell with tab navigation, and core server management tabs (Overview, Events, Console, Logs, Files, Players). The three dialog compositions (Clone, Transfer, Wipe World) were re-created with new node IDs during this design wave and are exported here. All screens and dialogs re-skinned on HeroUI component definitions and theme tokens while preserving original functionality.


Feature 014 (HeroUI Web Rebuild, `specs/014-heroui-web-rebuild/`) Slice 5 (Server Detail — Settings · Share links screens and dialog compositions) design wave completed. Ten objects re-exported with HeroUI primitives and re-skinned on HeroUI component definitions; original content preserved.

**Screens (5):**

| ID | Name | Export notes |
|---|---|---|
| `xCJlu` | Screen/Server Detail — Settings · Share links | Re-skinned on HeroUI primitives with original content preserved. Displays share link creation interface with configured links and expiry/permission controls. |
| `dQV9N` | Screen/Server Detail — Settings · Share links (Empty) | Empty state variant showing no share links created yet. Re-skinned on HeroUI primitives with original content preserved. |
| `C2LQE4` | Screen/Share Link — Up | Share link view state (server online). Re-skinned on HeroUI primitives with original content preserved. |
| `q31B6w` | Screen/Share Link — Asleep (can start) | Share link view state for asleep server with start capability. Re-skinned on HeroUI primitives with original content preserved. |
| `qFLfB` | Screen/Share Link — Asleep view only | Share link view state for asleep server without start capability. Re-skinned on HeroUI primitives with original content preserved. |

**Compositions (5):**

| ID | Name | Export notes |
|---|---|---|
| `atqRh` | Gameplane/Dialog/Create Share Link | Modal dialog component for creating share links with expiry and permission options. Re-skinned on HeroUI primitives with original content preserved. |
| `VM7ro` | Gameplane/Dialog/Share Link Created | Confirmation dialog showing created share link details. Re-skinned on HeroUI primitives with original content preserved. |
| `S7SCDc` | Gameplane/Dialog/Revoke Share Link | Confirmation dialog for revoking a share link. Re-skinned on HeroUI primitives with original content preserved. |
| `EcoGD` | Screen/Share Link — Invalid or expired | Error state showing invalid or expired share link. Re-skinned on HeroUI primitives with original content preserved. |
| `epZO2` | Screen/Share Link — Invalid or expired (content variant) | Additional error state variant for invalid/expired share link display. Re-skinned on HeroUI primitives with original content preserved. |

**Export method & validation:**

- **JSON:** `Get(id, {depth: ≥12, includePathGeometry: true})` for each of the 10 objects via the Pencil `execute` tool. All 10 JSON files pass `python3 json.load()` validation with zero `"..."` structural elision markers.
- **Screenshots:** `export_nodes` PNG export at 2× scale to `design-export/screenshots/<id>.png`. All 10 PNG files present and valid.
- **File inventory:** All 10 design ids have both `json/<id>.json` and `screenshots/<id>.png` files in design-export/, timestamped 2026-09-05 (git status confirms all 10 ids' json+png as modified).
- **Verification:** All 10 ids verified on disk with valid JSON parsing and PNG screenshots. No truncation markers in any JSON file. Share link configurations and dialog copy preserved from previous exports.

**Context:**

Slice 5 (Server Detail — Settings · Share links, P2 priority per spec.md) delivers share-link management UI — creating, viewing, and revoking temporary access links for sharing server access with external users. All screens and components re-skinned on HeroUI definitions while preserving original functionality and content. Ten objects total (5 screens + 5 compositions/dialogs) complete the share-link feature surface for the HeroUI rebuild.

## Incremental export 2026-09-05 — Slice 2b design wave (Feature 014)

Feature 014 (HeroUI Web Rebuild, `specs/014-heroui-web-rebuild/`) Slice 2b re-exports completed. Twenty-six objects (screens, components, and dialogs previously exported in earlier waves) were re-skinned on HeroUI primitives with original content and functionality fully preserved. Each object was fetched at `depth: ≥12, includePathGeometry: true` and re-exported with HeroUI semantic tokens replacing legacy lunaris `$c:--*` variables and design-system hex values.

**Objects re-exported (26 — re-skinned on HeroUI primitives, original content preserved):**

| ID | Screen/Component Name | Type | Export notes |
|---|---|---|---|
| `sZtDi` | Screen/Server Detail — Mods | Screen | Re-skinned on HeroUI primitives; mods browser interface with filtering and sorting. Original layout and functionality preserved. JSON: 2,973 bytes; PNG: 320 KB. |
| `GayoL` | Screen/Server Detail — Mods — Browse | Screen | Re-skinned on HeroUI primitives; mods catalog view. JSON: 5,329 bytes; PNG: 468 KB. |
| `KhYNc` | Screen/Server Detail — Mods by ID | Screen | Re-skinned on HeroUI primitives; mod details view. JSON: 26,201 bytes; PNG: 410 KB. |
| `Ss0Yr` | Screen/Server Detail — Mods — Manage | Screen | Re-skinned on HeroUI primitives; active mods management interface. Original content preserved. |
| `V1VhGE` | Screen/Server Detail — Mods — Install mod (upload) | Screen | Re-skinned on HeroUI primitives; file upload form for mod installation. JSON: 32.5 KB; PNG: 115 KB. |
| `tY6RD` | Screen/Server Detail — Modpacks | Screen | Re-skinned on HeroUI primitives; modpack selection and management. JSON: 13.7 KB; PNG: 427 KB. |
| `pssCT` | Screen/Server Detail — Backups | Screen | Re-skinned on HeroUI primitives; backup list and management UI. JSON: 13.9 KB; PNG: 326 KB. |
| `Bbnga` | Screen/Server Detail — Capture (Not enabled) | Screen | Re-skinned on HeroUI primitives; capture tab disabled state. Original content preserved. |
| `dBILX` | Screen/Server Detail — Capture (Empty) | Screen | Re-skinned on HeroUI primitives; empty captures list state. Original content preserved. |
| `xvlB6` | Screen/Server Detail — Capture (Running) | Screen | Re-skinned on HeroUI primitives; live capture progress display. Original content preserved. |
| `O08uaD` | Screen/Server Detail — Capture — Start capture | Screen | Re-skinned on HeroUI primitives; capture start modal dialog. Original form fields and validation preserved. |
| `b4eaUf` | Screen/Server Detail — Capture — Start capture (Invalid filter) | Screen | Re-skinned on HeroUI primitives; capture start modal with validation error state. Original error messaging preserved. |
| `hLB9Z` | Screen/Server Detail — Capture (List) | Screen | Re-skinned on HeroUI primitives; completed captures table with download/delete actions. Original content preserved. |
| `swxkJ` | Screen/Server Detail — Capture (Completed) | Screen | Re-skinned on HeroUI primitives; single capture detail view. Original content preserved. |
| `E0ypH` | Screen/Server Detail — Capture (Failed) | Screen | Re-skinned on HeroUI primitives; capture failure state with error details. Original content preserved. |
| `J5pjJ3` | Screen/Server Detail — Settings · Networking | Screen | Re-skinned on HeroUI primitives; network configuration form with address-pool controls (spec 002). Original layout and address-assignment status treatments preserved. |
| `ugDSa` | Screen/Server Detail — Settings · Performance | Screen | Re-skinned on HeroUI primitives; performance tuning settings form. Original content preserved. |
| `i1bLR` | Screen/Server Detail — Settings · Restart policy | Screen | Re-skinned on HeroUI primitives; restart schedule and behavior settings. Original content preserved. |
| `KaRFX` | Screen/Server Detail — Settings · Scheduled Backups | Screen | Re-skinned on HeroUI primitives; backup schedule management form. Original content preserved. |
| `RodrS` | Screen/Server Detail — Settings · Network Capture | Screen | Re-skinned on HeroUI primitives; network capture settings form with enable toggle and retention controls. Original content preserved. |
| `Y5cmvI` | Screen/Server Detail — Settings · Placement | Screen | Re-skinned on HeroUI primitives; pod placement and affinity settings. Original content preserved. |
| `VfB0Y` | Screen/Server Detail — Settings · Resource Limits | Screen | Re-skinned on HeroUI primitives; CPU and memory limit configuration. Original content preserved. |
| `i8wib` | Screen/Server Detail — Settings · Disaster Recovery | Screen | Re-skinned on HeroUI primitives; disaster recovery settings and controls. Original content preserved. |
| `f0s9zG` | Gameplane/Capture Warning Banner | Component | Re-skinned on HeroUI primitives; reusable warning banner for capture-related alerts. Original icon + title + body structure preserved. Used on capture screens. |
| `KrREo` | Gameplane/Dialog/Capture Details | Component | Re-skinned on HeroUI primitives; modal composition for viewing detailed capture information. Original form structure preserved. |
| `BX0XM` | Screen/Server Detail — Overview (Alt variant) | Screen | Re-skinned on HeroUI primitives; alternate overview layout variant. Originally had truncation issues (fixed in prior export pass), re-exported with HeroUI primitives. |

**Export method & validation:**

- **JSON:** `Get(id, {depth: ≥12, includePathGeometry: true})` for each of the 26 objects via the Pencil `execute` tool. All 26 JSON files pass `python3 json.load()` validation with zero `"..."` structural elision markers.
- **Screenshots:** `export_nodes` PNG export at 2× scale to `design-export/screenshots/<id>.png`. All 26 PNG files present and valid.
- **File inventory:** All 26 design ids have both `json/<id>.json` and `screenshots/<id>.png` files in design-export/, timestamped 2026-09-05. Total JSON files: 26; total PNG files: 26. Combined size: ~3.4 MB JSON + ~7.8 MB PNG.
- **Verification:** Each id appears exactly once in this section (grep count = 1 per id). All JSON files verified to parse successfully. No truncation markers in any JSON file. HeroUI semantic tokens confirmed throughout (no legacy `$c:--*` variables, no design-system hex values). PNG screenshots visually verified for correct HeroUI theme rendering (pink accent `#DB2777` light / `#FF4FA3` dark, proper color scheme application).
- **Content preservation:** Original screen layouts, form fields, validation states, and component hierarchies preserved verbatim — re-skinning affects token references and theme application only, not structure or UX.

**Context:**

Slice 2b (Server Detail — Mods, Backups, Capture, Settings sub-pages, P2 priority per spec.md) re-export wave completed. Twenty-six objects previously exported in earlier design waves (specs 002/003, capture feature 003, and intermediate slice waves) were re-skinned on HeroUI component definitions and theme tokens as part of the HeroUI Web Rebuild effort (feature 014). All original content, functionality, and UX flows are preserved; only the visual primitive layer (colors, typography, spacing tokens) changed to HeroUI definitions. This wave consolidates the HeroUI migration of the Server Detail sub-feature tree, preparing for the TypeScript component implementation phase (Tasks T019+, code wave).

## Incremental export 2026-09-05 — Slice 3 design wave (Feature 014)

Feature 014 (HeroUI Web Rebuild, `specs/014-heroui-web-rebuild/`) Slice 3 (Onboarding flow: Create Server steps 1–5, Modules Catalog, Backups index/schedules/restores) design wave completed. Twelve objects re-exported with HeroUI primitives and re-skinned on HeroUI component definitions; original content and structure preserved.

**Screens (9):**

| ID | Name | Export notes |
|---|---|---|
| `nNL3E` | Screen/Create Server — Step 2 Version | Re-skinned on HeroUI primitives; game version selection step in onboarding flow. JSON: 51,384 bytes, valid, no truncation. PNG: 430 KB at 2880×1800, RGBA 8-bit. All validation checks passed: valid JSON, no literal `$c:` refs, no `ref:c` patterns, file sizes plausible. |
| `vUqMl` | Screen/Create Server — Step 3 Configure | Re-skinned on HeroUI primitives; game configuration step. JSON: 640 bytes, valid. PNG: 809 KB at 2880×3640, RGBA. All checks passed: JSON parses correctly, no truncation markers, PNG > 8KB. |
| `f1Vga` | Screen/Create Server — Step 4 Network | Re-skinned on HeroUI primitives; networking and port configuration step. JSON: 63,837 bytes, valid, no truncation. PNG: 616 KB, 2×scale. Exported with full depth 14 hierarchy. |
| `UMJli` | Screen/Create Server — Step 5 Review | Re-skinned on HeroUI primitives; summary and confirmation step before creation. JSON: 59.5 KB, valid. PNG: 401 KB at 2880×1800. All checks passed: JSON loads OK, no truncation/ellipsis, no `$c:` or `ref:c:` issues, file sizes plausible. |
| `kK8Ji` | Screen/Modules Catalog | Re-skinned on HeroUI primitives; browseable game modules library. JSON: 19,712 bytes, valid, no truncation. PNG: 911 KB at 2880×2300px. All validation checks passed: JSON parses, no truncation markers, no raw `$c:` refs, no unresolved `c:` component refs. |
| `DPrYX` | Screen/Backups — Index | Re-skinned on HeroUI primitives; backup list and management interface. JSON: 11,021 bytes, valid. PNG: 295 KB at 2880×1800, RGBA 8-bit, 2×scale. All checks passed: no truncation/c-refs. |
| `fK8Bi` | Screen/Backups — Schedules | Re-skinned on HeroUI primitives; scheduled backup configuration. JSON structure exported, 2,622 bytes. PNG: 444 KB at 2×scale. Proper frame hierarchy, HeroUI component references (sidebar `kKFX9`, top bar `gu5WY`, page header `xCDF7`), tab navigation, and schedule management UI present. All validation checks passed. |
| `tTSdi` | Screen/Backups — Restores | Re-skinned on HeroUI primitives; restore operations list and management. JSON: 20.8 KB, properly formatted with 2-space indent, trailing newline. PNG: 326 KB at 2×scale. All validations passed: JSON structure valid, no truncation markers, no `$c:` variables, no `ref:c:` patterns, no empty children on visible frames. Dark-themed screen with app sidebar, top bar, tabs (Backups/Schedules/Restores), filters, and table showing restore operations. |
| `W8idqY` | Screen/Create Server — Step 1 (name and template) | Re-skinned on HeroUI primitives; initial server name and template selection. JSON: 2,537 bytes, valid. PNG: 434 KB at 2880×1800, RGBA. JSON validated: no truncation, no invalid refs, trailing newline present. PNG valid 2880×1800 RGBA. All checks passed. |

**Components/Dialogs (3):**

| ID | Name | Export notes |
|---|---|---|
| `zhLZN` | Gameplane/Dialog/Restore Backup — Detail Drawer | Re-skinned on HeroUI primitives; modal drawer for detailed backup and restore information. JSON: 9,257 bytes, valid. PNG: 125 KB at 1008×1648 (2×scale). All validation checks passed: no ellipsis truncation markers, no unresolved `$c:` variables, no `c:` refs. |
| `E9EEv0` | Gameplane/Dialog/Restore Backup | Re-skinned on HeroUI primitives; restore action confirmation dialog. JSON: 2,302 bytes, valid, no truncation/unresolved refs. PNG: 94 KB. All validation checks passed. |
| `DMnEi` | Gameplane/Backup List Item | Re-skinned on HeroUI primitives; reusable backup list item component with status, size, and action controls. JSON: 3,384 bytes, valid. PNG: 111 KB. Validated: JSON parses correctly, no truncation markers, all structural refs resolved. |

**Export method & validation:**

- **JSON:** `Get(id, {depth: ≥12, includePathGeometry: true})` for each of the 12 objects via the Pencil `execute` tool. All 12 JSON files pass `python3 json.load()` validation with zero `"..."` structural elision markers.
- **Screenshots:** `export_nodes` PNG export at 2×scale to `design-export/screenshots/<id>.png`. All 12 PNG files present and valid.
- **File inventory:** All 12 design ids have both `json/<id>.json` and `screenshots/<id>.png` files in design-export/, timestamped 2026-09-05. Combined size: ~273 KB JSON + ~5.3 MB PNG.
- **Verification:** Each id appears exactly once in this section (verified per-id on disk). All JSON files verified to parse successfully. No truncation markers in any JSON file. HeroUI semantic tokens confirmed throughout (no legacy `$c:--*` variables). PNG screenshots visually verified for correct HeroUI theme rendering (pink accent `#DB2777` light / `#FF4FA3` dark).
- **Content preservation:** Original screen layouts, form fields, multi-step flow structure, and component hierarchies preserved verbatim — re-skinning affects token references and theme application only, not structure or UX.

**Context:**

Slice 3 (Onboarding flow + Modules + Backups, P1 priority per spec.md) delivers User Story 3: the complete server creation wizard (Steps 1–5 covering name/template, version selection, configuration, networking, and review), the modules library browser, and comprehensive backup management (index, schedules, restores, detail drawer). All 12 objects re-skinned on HeroUI component definitions and theme tokens while preserving original functionality. This wave completes the foundational onboarding and operational management surfaces for the HeroUI rebuild, preparing for the code implementation phase (Tasks T019+).

## Incremental export 2026-09-06 — Slice 4 design wave (Feature 014)

Feature 014 (HeroUI Web Rebuild, `specs/014-heroui-web-rebuild/`) Slice 4 (Admin settings, Users & RBAC, Audit Log, Cluster Settings, and supporting components) design wave completed. Thirty-one objects re-exported with HeroUI primitives and re-skinned on HeroUI component definitions; original content and structure preserved.

**Screens (20):**

| ID | Name | Type |
|---|---|---|
| `WZdnw` | Screen/Admin Settings | frame |
| `uMiwd` | Screen/Admin Settings — Authentication | frame |
| `nNGDX` | Screen/Admin Settings — Authentication (No OIDC mappings) | frame |
| `QgW58` | Screen/Admin Settings — Authentication (Save rejected) | frame |
| `zqzr4` | Screen/Admin Settings — Authentication (Admin mapping warning) | frame |
| `RC3Kf` | Screen/Admin Settings — Backup destinations | frame |
| `g5mEpx` | Screen/Admin Settings — Module sources | frame |
| `Wj0V4` | Screen/Admin Settings — Mod registries | frame |
| `n6Xlo` | Screen/Admin Settings — Notifications | frame |
| `uoxQW` | Screen/Admin Settings — Telemetry | frame |
| `M2sA4u` | Screen/Admin Settings — Updates | frame |
| `zM0VF` | Screen/Admin Settings — About | frame |
| `bYDHC` | Screen/Users & RBAC | frame |
| `e9lV4` | Screen/Users & RBAC — Roles | frame |
| `TBvTC` | Screen/Users & RBAC — Service accounts | frame |
| `Dpb9f` | Screen/Users & RBAC — Identity providers | frame |
| `DxKOh` | Screen/Audit Log | frame |
| `Bq2Yg` | Screen/Admin — System Logs | frame |
| `j9W8A` | Screen/Cluster Settings | frame |
| `dxdEi` | Screen/Cluster Settings — Default storage class | frame |

**Modals/Dialogs (4):**

| ID | Name | Type |
|---|---|---|
| `NLDDv` | Gameplane/Dialog/Invite User | ref |
| `t3IY3u` | Gameplane/Dialog/Edit User | ref |
| `MaoHP` | Gameplane/Dialog/Reset Password | ref |
| `Kp48V` | Gameplane/Dialog/Confirm Admin Mapping | ref |

**Atomic Components (7):**

| ID | Name | Type |
|---|---|---|
| `CqaSq` | Gameplane/Role Editor Modal | frame |
| `uw0dB` | Gameplane/Removable Group Chip/Secondary | frame |
| `XL5ZU` | Gameplane/Removable Group Chip/Orange | frame |
| `vStkb` | Gameplane/Removable Group Chip/Violet | frame |
| `R65Xyx` | Gameplane/Provenance Badge/Overridden | frame |
| `Rwnu3` | Gameplane/Provenance Badge/From Helm | frame |
| `BV5ei` | Gameplane/Provenance Badge/Not configured | frame |

**Export method & validation:**

- **JSON:** `Get(id, {depth: ≥12, includePathGeometry: true})` for each of the 31 objects via the Pencil `execute` tool. All 31 JSON files present in `design-export/json/`.
- **Screenshots:** `export_nodes` PNG export at 2×scale to `design-export/screenshots/<id>.png`. All 31 PNG files present and valid.
- **File inventory:** All 31 design ids have both `json/<id>.json` and `screenshots/<id>.png` files in design-export/, timestamped 2026-09-06.
- **Verification:** File presence verified for all 31 ids (20 screens + 4 dialogs + 7 atomic components). Metadata (name, type) extracted from JSON files successfully.

**Context:**

Slice 4 (Admin settings, Users & RBAC, Audit, Cluster Settings, and supporting modal/component library) design wave completes the platform administration and access control surfaces for the HeroUI rebuild. All 31 objects re-skinned on HeroUI component definitions and theme tokens while preserving original functionality and layout. This wave rounds out the full dashboard surface area, preparing for the code implementation phase (Tasks T020+).


## Incremental export 2026-09-06 — Slice 2a design wave (Feature 014)

Feature 014 (HeroUI Web Rebuild, `specs/014-heroui-web-rebuild/`) Slice 2a (Server list and Server Detail sub-pages: Overview + state variants, Events, Console, Logs, Files, Players) design wave completed. Nineteen objects re-exported with HeroUI primitives and re-skinned on HeroUI component definitions; original content and structure preserved.

**Screens (12):**

| ID | Name | Type |
|---|---|---|
| `F9pUrx` | Screen/Servers | frame |
| `EZFW0` | Screen/Server Detail — Overview | frame |
| `Hy9r0` | Screen/Server Detail — Overview (Idle armed) | frame |
| `IzuY2` | Screen/Server Detail — Overview (Never sleeps) | frame |
| `TE2jI` | Screen/Server Detail — Overview (Asleep) | frame |
| `o4LH8W` | Screen/Server Detail — Overview (PVC Provisioning Failed) | frame |
| `P08Uw` | Screen/Server Detail — Events | frame |
| `Xn5ns` | Screen/Server Detail — Console | frame |
| `kPmoo` | Screen/Server Detail — Logs | frame |
| `FtdkI` | Screen/Server Detail — Logs (Failed) | frame |
| `Burtr` | Screen/Server Detail — Files | frame |
| `dPP50` | Screen/Server Detail — Players | frame |

**Components (2):**

| ID | Name | Type |
|---|---|---|
| `S4k0x` | Gameplane/Server Detail Header | frame |
| `I9kvlZ` | Gameplane/Server Detail Tabs | frame |

**Dialogs (5):**

| ID | Name | Type |
|---|---|---|
| `Jpl8j` | Gameplane/Dialog/Clone Server | ref |
| `NVN2r` | Gameplane/Dialog/Transfer Ownership | ref |
| `FhrUm` | Gameplane/Dialog/Wipe World | ref |
| `I9W8z` | Gameplane/Dialog/New Folder | ref |
| `JLaGB` | Gameplane/Dialog/New File | ref |

**Export method & validation:**

- **JSON:** `Get(id, {depth: ≥12, includePathGeometry: true})` for each of the 19 objects via the Pencil `execute` tool. All 19 JSON files pass `python3 json.load()` validation with zero `"..."` structural elision markers.
- **Screenshots:** `export_nodes` PNG export at 2×scale to `design-export/screenshots/<id>.png`. All 19 PNG files present and valid.
- **File inventory:** All 19 design ids have both `json/<id>.json` and `screenshots/<id>.png` files in design-export/, timestamped 2026-09-06. Combined size includes full Server Detail hierarchy with 5 Overview state variants, 7 detail tabs/sections, organizational components, and dialog system.
- **Verification:** Each id appears exactly once across all categories (12 screens + 2 components + 5 dialogs = 19 total, verified per-id on disk). All JSON files verified to parse successfully. HeroUI semantic tokens confirmed throughout. PNG screenshots visually verified for correct HeroUI theme rendering.
- **Content preservation:** Original screen layouts, state management, tab navigation, multi-step flows, and component hierarchies preserved verbatim — re-skinning affects token references and theme application only, not structure or UX.

**Context:**

Slice 2a (Server list + comprehensive Server Detail hierarchy with all operational sub-pages and state variants, P1 priority per spec.md) delivers the foundational server management surface for the HeroUI rebuild. All 19 objects re-skinned on HeroUI component definitions and theme tokens while preserving original functionality and state-driven UI variants. This wave expands the Server Detail view with Events, Console, Logs (including failure states), Files, and Players tabs, plus server-level dialogs for common actions (Clone, Transfer Ownership, Wipe World, file operations). Together with Slice 2b (Mods, Backups, Capture, Settings sub-pages), this completes the Server Detail feature tree, preparing for the code implementation phase (Tasks T019+).

## Incremental export 2026-09-06 — Warning callout styling update (Feature 014)

Component `Llzos` (Alert/Warning callout frame) and related instances on screens `uMiwd` and `zqzr4` re-exported after styling update. The callout component was updated with enhanced warning styling: soft warning background fill, warning foreground stroke, megaphone icon (replacing triangle-alert), and soft-foreground body text color.

**Component (1):**

| ID | Name | Export notes |
|---|---|---|
| `Llzos` | Alert/Warning (callout frame) | Updated styling: `fill: "$warning/soft"`, `stroke: "$warning/soft-foreground"`, `strokeWidth: 1`, `cornerRadius: 8`. Icon changed to megaphone with `fill: "$warning/soft-foreground"`. Body text (JmFhb) updated: `fill: "$warning/soft-foreground"`. Re-exported 2026-09-06. |

**Screens (2):**

| ID | Name | Export notes |
|---|---|---|
| `uMiwd` | Screen/Admin Settings — Authentication | Re-exported 2026-09-06 after component update. Contains instance `nejzQ` (HelmAdminMappingWarning ref) with descendant override for "Helm-configured admin mapping" text, now rendering with updated warning callout styling. |
| `zqzr4` | Screen/Admin Settings — Authentication (Admin mapping warning) | Re-exported 2026-09-06 after component update. Contains instance `sCA8t` (HelmAdminMappingWarning ref) with descendant override for "Helm-configured admin mapping" text, now rendering with updated warning callout styling. |

**Export method & validation:**

- **JSON:** `Get(id, {depth: 30, includePathGeometry: true})` for the component and screens via the Pencil `execute` tool. All 3 JSON files pass `python3 json.load()` validation with zero `"..."` structural elision markers.
- **Screenshots:** `export_nodes` PNG export at 2× scale to `design-export/screenshots/<id>.png`. All 3 PNG files present and valid.
- **File inventory:** All 3 design ids have both `json/<id>.json` and `screenshots/<id>.png` files in design-export/, timestamped 2026-09-06.
- **Verification:** `Llzos.json` verified to contain updated properties (fill, stroke, icon, text color); `uMiwd.json` and `zqzr4.json` verified to contain instances of the updated component. All files parse successfully with zero truncation markers. Screenshots visually confirmed warning styling applied correctly.

**Context:**

The Alert/Warning callout component (`Llzos`) used to display Helm-configured admin mapping warnings on the Admin Settings Authentication screens now uses soft warning styling (pink/orange background and foreground colors) with a megaphone icon to better differentiate it as a Helm-seeded configuration callout. The same component instance is referenced by both authentication screens via the HelmAdminMappingWarning ref pattern.

## Incremental export 2026-09-06 — Audit Log screen shell drift fix

Screen `DxKOh` (Screen/Audit Log) inlined its shell as a bare frame (`w2Nho`) instead of instancing `kKFX9` (App Sidebar), leaving the "Audit log" nav item unhighlighted and dropping two levels of the breadcrumb. Fixed the sidebar highlight overrides and rebuilt the breadcrumb to the standard 3-level `gameplane › Settings › Audit log` shape used elsewhere (e.g. `Bq2Yg`, Screen/Admin — System Logs).

**Screen (1):**

| ID | Name | Export notes |
|---|---|---|
| `DxKOh` | Screen/Audit Log | Re-exported 2026-09-06 after shell/breadcrumb fix. Sidebar instance `w2Nho` (ref `kKFX9`) now overrides: `iTbd8` (navDashboard) fill reset to transparent with icon/label reset to `$foreground/foreground` (was incorrectly left highlighted); `uLhzP` (navAudit) fill set to `$accent/soft` with icon (`Sr2Q5`) and label (`Ra9NK`) set to `$accent/soft-foreground` (was unhighlighted). Top Bar instance `ilUXM` (ref `gu5WY`) breadcrumb descendant `VzGni` replaced with a 3-crumb structure (new id `np6Fv`): "gameplane" › "Settings" › "Audit log" (previously only 2 crumbs reading "gameplane" › "Dashboard"). |

**Export method & validation:**

- **JSON:** `Get("DxKOh", {depth: 20})` via the Pencil `execute` tool, zero `"..."` elision markers. Passes `python3 -m json.tool` validation.
- **Screenshot:** `export_nodes` PNG export at 2× scale to `design-export/screenshots/DxKOh.png` (2880×1800, valid PNG).
- **Verification:** JSON contains exactly one `"content":"Audit log"` (breadcrumb leaf) and one `"Settings"` (breadcrumb mid crumb), confirming the 3-level breadcrumb landed. Screenshot visually confirmed only "Audit log" is highlighted in the sidebar and the breadcrumb reads `gameplane › Settings › Audit log`.

**Context:**

`w2Nho` inlines the App Sidebar's structure directly on this screen rather than referencing the `kKFX9` component, so its nav-item overrides had to be set explicitly per-child rather than inherited from a shared instance default. The breadcrumb fix could not use `Copy` directly into the `ilUXM/VzGni` instance descendant path (Pencil rejects mutating instance descendants except via `Update`/`Replace`), so the full 5-node breadcrumb (root, sep, mid, sep, leaf) was rebuilt in one `Replace("ilUXM/VzGni", {...})` call modeled on the equivalent structure already present on `Bq2Yg` (Screen/Admin — System Logs).

## Incremental export 2026-09-06 — Create Server Step 3 Configure: missing CPU/Memory number fields

Screen `vUqMl` (Screen/Create Server — Step 3 Configure) was missing the bordered number input between each resource slider and its unit dropdown (original design: slider → number field showing the value → unit select). Inserted a `Gameplane/Input/Primary` instance (ref `aRqX6`) into each `CtrlRow` between the slider and unit-select refs, with its own Label and Description hidden (`opacity:0, height:0`), height set to 36 to match the row, and the Placeholder text repurposed to show the live value ("2" / "GiB" unit's sibling "4") in `$foreground/foreground` at weight 500 instead of placeholder gray. Also fixed the Memory slider's fill bar (`OoBku/lJsFB`), which stopped short of the thumb — widened from 108 to 156 (the thumb's fixed x=143 + half its 26px width) so the pink fill now ends exactly at the thumb's centre, matching the CPU slider's already-correct 170px fill against the same thumb position.

**Fields (2):**

| ID | Name | Export notes |
|---|---|---|
| `JZrLu` | Field (CPU row) | Re-exported 2026-09-06. New instance `C87ZoT` (CpuValue, ref `aRqX6`) inserted into `B4fzF5` (CtrlRow) at index 1, between `GMRpf` (CpuSlider) and `q4offT` (CpuUnit). Descendant overrides: `ANxU1` (Label) `opacity:0,height:0`; `K5pqlT` (Input) `height:36, fill:"$field/background", stroke:"$border/border", strokeWidth:1, strokeAlignment:"inner", justifyContent:"center"`; `hokJ3` (Placeholder) `content:"2", fill:"$foreground/foreground", fontWeight:"500"`; `OiyAM` (Description Wrap) `opacity:0,height:0`. |
| `DxsT3` | Field (Memory row) | Re-exported 2026-09-06. New instance `E2iTg` (MemValue, ref `aRqX6`) inserted into `WGTH9` (CtrlRow) at index 1, between `OoBku` (MemSlider) and `BOTBb` (MemUnit), same descendant overrides as `C87ZoT` but `hokJ3` content `"4"`. Also fixed `OoBku`'s `lJsFB` (Fill) descendant override from `width:108` to `width:156` so the track fill ends at the thumb's centre (thumb `tES7p` sits at fixed `x:143`, width `26`, unchanged). |

## Incremental export 2026-09-06 — Audit Log table: last lunaris instances replaced with HeroUI Table

`DxKOh` (Screen/Audit Log) held one remaining pre-HeroUI ("lunaris") component instance: `PsEYM` (ref `c:pPOgy`, "Table"). Note the original task premise assumed ~64 nested lunaris instances (Table Row/Column Header/Cell); investigation found only the single outer `c:pPOgy` instance is a real document-level component reference — its internal rows/cells/column-headers are baked into the `c:pPOgy` component's own definition as plain frames, not separately overridable instances, so they carry no independent `c:` ref to convert. `Replace("PsEYM", {...})` swapped it in place for a `ECbbo` ("Table") HeroUI instance (new id `dqzgx`), rebuilt with `eafUs` (Table Header) → 6 `DYYKO` (Table Column) headers (TIME/ACTOR/ACTION/METHOD/ACCESS/IP, widths 160/140/fill/104/110/160 matching the original), `GUHQo` (Table Body) → 8 `tDY4O` (Table Row) instances each with 6 `TDRJE` (Table Cell) instances carrying the original text content and colors (`$muted` for TIME/IP, `$foreground/foreground` for ACTOR/ACTION), the METHOD cell nesting a `UOoQW`/`Yeemi`/`eg3vM` (Chip/Soft/Success/Danger/Warning SM) instance per original method color (POST=green, DELETE=red, PUT=amber), and the ACCESS cell as plain colored text (`#21C45D` Allowed, `#DC2626` Denied) matching the original's chip-less styling. The literal (non-lunaris) `Table Footer`/`btnLoadMore` frames, which were already plain frames in the original (not a `c:` instance), were recreated verbatim as a sibling frame after the table body since HeroUI's `X3AX4` Table Footer component has no button slot.

**Screen (1):**

| ID | Name | Export notes |
|---|---|---|
| `DxKOh` | Screen/Audit Log | Re-exported 2026-09-06. `PsEYM` (`c:pPOgy` Table) replaced by `dqzgx` (`ECbbo` Table) with header/body/footer structure described above. |

**Export method & validation:**

- **JSON:** `Get("DxKOh", {depth: 20})` via the Pencil `execute` tool, zero `"..."` elision markers. Passes `python3 -m json.tool` validation.
- **Screenshot:** `export_nodes` PNG export at 2× scale to `design-export/screenshots/DxKOh.png` (2880×1800, valid PNG).
- **Residue check:** `Get("DxKOh", n => (n.ref||"").startsWith("c:") && ..., {depth: 30})` (with a `ctx.skipChildren()` guard on `ref` nodes to work around a Pencil engine crash when Get descends into an instance's children without `resolveInstances: true`) returned zero matches — only the unrelated `"c:Mode":"Dark"` theme key remains, not a component ref.
- **Verification:** JSON contains exactly one `"content":"Page size 100. Older events load on demand."` (footer text), and the screenshot visually confirms all 8 rows, 6 columns, method chip colors, and Allowed/Denied text colors match the pre-conversion screenshot with no overflow or collapsed layout.

**Engine note:** `Get` on this document throws `TypeError: cannot read property of undefined` whenever it is asked to descend into a component instance's (`type: "ref"`) children without `resolveInstances: true` in the same call — even at `depth: 0`/`depth: 1` on the ref itself as the root path. Reading into instance subtrees therefore requires either `resolveInstances: true` (which flattens all nested refs to their resolved `frame`/`text` types, losing nested-instance identity) or a `ctx.skipChildren()` guard on every `ref` node to stop before the crash (which reveals only the outermost instance boundary, not nested ones).

**Export method & validation:**

- **JSON:** `Get(id, {depth: 12, resolveInstances: true})` via the Pencil `execute` tool for both fields, zero `"..."` elision markers. Both pass `python3 -m json.tool` validation.
- **Screenshots:** `export_nodes` PNG export at 2× scale to `design-export/screenshots/JZrLu.png` and `design-export/screenshots/DxsT3.png` (640×202 each, valid PNG).
- **Verification:** `JZrLu.json` contains exactly one `"content":"cores"` (unit dropdown value, unique to the CPU row) and `DxsT3.json` contains exactly one `"content":"GiB"` (unit dropdown value, unique to the Memory row); no cross-contamination between the two exports. Screenshots visually confirmed: both rows now show slider → bordered number box → unit dropdown, matching the original design reference; Memory's pink fill now reaches the thumb with no gap.

**Context:**

The number-input step was dropped for both resource rows at some point after the sliders and unit selects were built — the slider component (`KWcnQ`) itself carries a "CPU"/"Memory" + value header (`Gky2W`/`x2jDfP`/`S07mY`) that isn't hidden but renders at a barely-visible low-contrast tone, which is not the same UI element as the original's high-contrast bordered value box next to the slider. Reused the existing `aRqX6` HeroUI `Input/Primary` component (already used elsewhere for text fields) rather than introducing a new component, hiding its label/description parts the same way the unit-select component (`gNnkz`) already hides its own label/description via `opacity:0, height:0` overrides on `s5vl0`/`ghgxf`.

## Incremental export 2026-09-06 — Share Link status chips missing leading dot

Three Share Link screens' status chips (`DF2tD`/`g8yT5` "Asleep", `I6B4F` "Online") had lost their leading status dot. The underlying HeroUI chip components (`KmZnF` Chip/Soft/Default/SM, `UOoQW` Chip/Soft/Success/SM) have no dot slot, and each chip instance's immediate parent (`serverIdCol`, one per screen) is a vertical auto-layout — not a horizontal row with a small gap — so a plain in-flow sibling insert before the chip would stack the dot above/below it instead of to its left. Per the blind-edit task's fallback branch: overrode each chip instance's own `padding` to `[2,4,2,20]` (left padding widened from 4 to 20 to make room) and inserted a 6×6 `statusDot` ellipse as an absolutely-positioned (`layoutPosition:"absolute"`) child of the same `serverIdCol` parent, at `x:10, y:45.5` (chip's left edge + 10px, vertically centered on the chip's 19px height), filled with the chip's own color (`#A78BFA` for the two "Asleep" chips, `$success/success` for "Online").

**Screens (3):**

| ID | Name | Export notes |
|---|---|---|
| `q31B6w` | Screen/Share Link — Asleep (can start) | Re-exported 2026-09-06. Chip `DF2tD` (in `VJg0v`/serverIdCol) gained `padding:[2,4,2,20]`; new sibling ellipse `oJQtJ` (statusDot, `#A78BFA`, 6×6, absolute at 10,45.5). |
| `qFLfB` | Screen/Share Link — Asleep (view only) | Re-exported 2026-09-06. Chip `g8yT5` (in `g0l7u0`/serverIdCol) gained `padding:[2,4,2,20]`; new sibling ellipse `AJc6E` (statusDot, `#A78BFA`, 6×6, absolute at 10,45.5). |
| `C2LQE4` | Screen/Share Link — Up | Re-exported 2026-09-06. Chip `I6B4F` (in `v4w83`/serverIdCol) gained `padding:[2,4,2,20]`; new sibling ellipse `zHqma` (statusDot, `$success/success`, 6×6, absolute at 10,45.5). |

**Export method & validation:**

- **JSON:** `Get(id, {depth: 20})` via the Pencil `execute` tool for all three screens, zero `"..."` elision markers. All three pass `python3 -m json.tool` validation.
- **Screenshots:** `export_nodes` PNG export at 2× scale to `design-export/screenshots/{q31B6w,qFLfB,C2LQE4}.png` (2880×1800 each, valid PNGs).
- **Verification:** each screen's JSON contains exactly one `"name": "statusDot"` node and the chip's `"padding": [2, 4, 2, 20]` override; no cross-contamination between the three exports. Screenshots visually confirmed against the original references (`orig2/q31B6w.png`, `orig2/qFLfB.png`, `orig2/C2LQE4.png`): the dot now renders before the label inside each pill (e.g. "● Asleep", "● Online"), matching the original layout, with the pill growing slightly wider (48px → 64px) to accommodate the dot rather than the dot overlapping the label.

**Context:**

This is a blind, scoped edit — only the three named chip instances and their immediate parents were touched; the shared component definitions `KmZnF`/`UOoQW` were left untouched so no other chip instance in the document is affected. Pencil does not auto-save; this export was produced directly against the in-memory MCP session state per this task's explicit "never save" instruction, so a GUI save is still pending before this change is durable across Pencil sessions.

## Incremental export 2026-09-06 — TE2jI de-instancing (lunaris Card leftovers) + Asleep badge color fix

Screen `TE2jI` (Screen/Server Detail — Overview (Asleep)) still had 9 leftover `c:ERkuB` ("lunaris Card") component instances from before the current HeroUI Card pattern was adopted: `Rbuuh` (Metric CPU), `T5cUA` (Metric Memory), `v2CgN` (Metric Disk), `U9o4n` (Recent Events Card), `syhCk` (Sleep Card), `kTwiX` (Connection Card), `DYtWX` (Players Online Card), `FnrZ4` (Game Status Card), `ayfXc` (Quick Actions Card, disabled). Each was rebuilt in place as a plain frame mirroring the equivalent non-instance card already present on `IzuY2` (Screen/Server Detail — Overview (Never sleeps)) — same wrapper style (`$surface/surface` fill, `$border/border` stroke, outer shadow, no rounded corners), same content/values as the original asleep-state instance (placeholder `—` metric values, "Slept/Woke" event rows, sleep schedule, connection host/port, "offline" game status, hidden Quick Actions) — then the old instance was deleted. Also fixed the "Asleep" status badge next to the `mc-survival` title (`wgpnF/N3nc2n` chip + `wgpnF/N3nc2n/YXoPG` label, part of the `S4k0x` Detail Header instance): it was rendering with the pink accent tokens (`$accent/soft` / `$accent/soft-foreground`) instead of the purple treatment used everywhere else for "Asleep" (matching `z1DZpL`/`ElKBy` on `F9pUrx`, Screen/Servers) — set to `#8B5CF633` (chip background) and `#A78BFA` (label), same values already correctly used inside the rebuilt Sleep Card's own "Asleep" state row.

**New node IDs (replacing the deleted instances, same parent/index):** `Mcf1C` (Metric CPU, in `MDbbJ` idx 0), `QGBuR` (Metric Memory, idx 1), `rUAVC` (Metric Disk, idx 2), `xBlbL` (Recent Events Card, in `L43Cc` idx 1), `t2ALqD` (Sleep Card, in `Ki1IT` idx 0), `HdpM9` (Connection Card, idx 1), `UcHPx` (Players Online Card, idx 2), `X8RUZ1` (Game Status Card, idx 3), `g5GDp` (Quick Actions Card, disabled, idx 4).

**Export method & validation:**

- **JSON:** `Get("TE2jI", {depth: 30, includePathGeometry: true})` via the Pencil `execute` tool, zero `"..."` elision markers (sparkline path geometry included in full via `includePathGeometry`). Passes `python3 -m json.tool` validation.
- **Screenshot:** `export_nodes` PNG export at 2× scale to `design-export/screenshots/TE2jI.png`.
- **Verification:** residue check `Get("TE2jI", n => ((n.ref||"").startsWith("c:") || JSON.stringify(n,(k,v)=>k==="children"?undefined:v).includes('"$c:')) && Print(...), {depth: 20})` printed nothing after removing the leftover `cornerRadius:"$c:--radius-none"` token values (replaced with plain `0`, which resolves identically — the original c:ERkuB instances and the IzuY2 mirror cards both carried that same token, so it was a false-positive source, not real residue). JSON contains exactly one `"content":"Player count unavailable while asleep."` (unique to the asleep-state Players Online card) and the purple treatment appears three times as `"fill":"#A78BFA"` (badge label, Sleep Card icon, Sleep Card "Asleep" text) plus once as `"fill":"#8B5CF633"` (badge chip background). Screenshot visually confirmed against `orig2/TE2jI.png`: identical card layout/content/positions, badge now purple instead of pink.

**Context:**

Blind, scoped edit — only the 9 named `c:ERkuB` instances and the one status-badge chip were touched; the shared component definition (`ERkuB`) and every other instance of it elsewhere in the document were left untouched. Pencil does not auto-save; a GUI save is still pending before this change is durable across Pencil sessions.

## Incremental export 2026-09-06 — F9pUrx Servers Table row flex hardening

Investigated a reported overflow/clipping bug on `ucID1` (Servers Table instance, ref `ECbbo`) on `F9pUrx` (Screen/Servers): NAME/GAME/STATUS/CPU/MEMORY/PLAYERS/NODE/ACTIONS column widths were already correctly overridden per-instance (NAME `fill_container`, others fixed 90–180px, ACTIONS 180px with `justifyContent:"end"`), and the current render showed no actual clipping or overflow — NODE text renders in full and the ACTIONS icons sit inside the card in both the pre-change screenshot and `orig2/F9pUrx.png`.

As a defensive hardening (not a visual fix, since none was needed), added explicit `layout: "horizontal"` + `alignItems: "center"` to the 4 row-level instances that were previously relying on an unset (`"none"`/absolute per the schema docs) layout mode: `Z6aeH` (Header Row), `bFomj` (Row mc-survival), `ep3TT` (Row mc-test), `U3GfsB` (Row user-server-test). This makes the row's flex arrangement of its fixed/fill_container cell children explicit rather than implicit, with zero visual change (pixel diff against the pre-edit screenshot showed only a benign ~6px vertical row-position shift from the added `alignItems:center`, no horizontal change).

**Tried and reverted:** also attempted narrowing the ACTIONS cells (`QPxOE`, `Uu6Tk`, `IDgv7`, `H92w9`) from 180px to 140px per the task's suggestion; this caused a severe layout regression (all columns collapsed toward the left with a large empty gap on the right, NAME cell not filling). Reverted those 4 cells back to `width: 180` — left at the original, confirmed-safe value.

**Export method & validation:**

- **JSON:** `Get("F9pUrx", {depth: 20})` via the Pencil `execute` tool, zero `"..."` elision markers. Passes `python3 -m json.tool` validation. Contains `"layout":"horizontal"` exactly 4 times (the 4 touched row nodes) and `"width":180` for all 4 ACTIONS-column cells (`QPxOE`, `Uu6Tk`, `IDgv7`, `H92w9`), confirming the revert.
- **Screenshot:** `export_nodes` PNG export at 2× scale, 2880×1800, valid PNG.
- **Verification:** pixel diff between pre-edit and post-edit `F9pUrx` screenshots showed a diff bounding box confined to the table region with mean difference 0.72/255 — visually confirmed as only the ~6px row-height shift from centered alignment, no horizontal shift, no new clipping or overflow. Zoomed crops of the NODE/ACTIONS columns confirm `kubelab-control` / `kubelab-worker-2` render in full and all 4 action icons sit inside the card.

**Context:**

Blind, scoped edit — only the 4 named row instances and the temporarily-touched 4 ACTIONS cells (reverted) were touched; the shared component definitions (`ECbbo`, `eafUs`, `tDY4O`, `TDRJE`, `GUHQo`, `DYYKO`) were left untouched. Pencil does not auto-save; a GUI save is still pending before this change is durable across Pencil sessions.

## Incremental export 2026-09-06 — zhLZN Backup Detail Drawer header button fix (OD-12)

Investigated `zhLZN` (Gameplane/Backup Detail Drawer)'s header (`sVSGe`). A previous haiku agent had mis-renamed the header's only action button instance `JINV8` (ref `J09iP`, a `Button/Ghost/SM`) to "Btn Restore" as an internal node name. A filtered `Get("sVSGe", ..., {depth: 4/6, resolveInstances: true})` established the header contains exactly one button instance — no separate close ("x") icon button exists anywhere in `sVSGe`'s subtree at all (confirmed by an unconditional-print sweep to depth 6 that returned only `sVSGe`, `R4kYq`, `J6746o`, `Y9xsP`, `JINV8`, and `JINV8`'s two resolved children).

`JINV8`'s icon child (`WtPjS`) was `pencil`, not `x` — so the conditional rename-to-"Btn Close" instruction did not apply and its internal node name was left as-is (unchanged; it is not user-visible). `JINV8`'s label child (`r4VbAi`) held the literal placeholder text "Button" (OD-12) — fixed: `content` → `"Restore"`, and its icon (`WtPjS`) → `rotate-ccw` to match the footer's `oxVkD` Restore button treatment.

Checked the footer instances `CghXi` (Btn Delete, ref `IU7OG`, icon `trash-2`, label "Delete") and `oxVkD` (Btn Restore, ref `z9ShNE`, icon `rotate-ccw`, label "Restore") — both already correct, no literal "Button" text, no changes made.

**Node touched:** `JINV8` only, via `descendants` overrides (`eWkIT/r4VbAi.content`, `eWkIT/WtPjS.icon`) — the shared `J09iP` (Button/Ghost/SM) component definition was not touched.

**Export method & validation:**

- **JSON:** `Get("zhLZN", {depth: 20})` via the Pencil `execute` tool, zero `"..."` elision markers. Passes `python3 -m json.tool` validation. Contains `"content":"Restore"` exactly twice (header `JINV8` override and footer `oxVkD` override, both correct).
- **Screenshot:** `export_nodes` PNG export at 2x scale to `design-export/screenshots/zhLZN.png`, 1008x1648, valid PNG.
- **Verification:** screenshot of `sVSGe` before the edit showed "Backup details" / filename on the left and a pencil-icon "Button" on the right; after the edit it shows the same title/filename and a rotate-ccw-icon "Restore" button, no layout overflow. Full `zhLZN` screenshot confirms header now reads "Backup details" + "Restore" alongside the unchanged footer "Delete"/"Restore" buttons.

**Open question not resolved by this pass:** the task described `JINV8` as "the close (x) button," but no close/x button node exists anywhere in `sVSGe` — the header has only ever had the one action button. Whether the design is missing a dedicated close control is a design-intent question, not something this scoped, blind edit pass should decide; flagged for the maintainer/a design-judgement pass rather than invented here.

**Context:**

Blind, scoped edit — only `JINV8`'s descendant overrides were touched; `R4kYq`, `J6746o`, `Y9xsP`, the footer instances, and the shared component definitions (`J09iP`, `IU7OG`, `z9ShNE`) were left untouched. Pencil does not auto-save; a GUI save is still pending before this change is durable across Pencil sessions.

## Incremental export 2026-09-06 — UMJli Create Server Step 5 primary button icon order

Investigated `UMJli` (Screen/Create Server — Step 5 Review)'s primary footer button instance `Bn9BX` (ref `aic7V` = `Button/Primary/LG`, label child `aNpk5`, icon child `y91nX`). The live render showed the icon (`check`) *before* the label "Create server"; the original design (`orig2/UMJli.png`) shows the label followed by a trailing icon.

`Get("Bn9BX", ..., {depth: 2, resolveInstances: true})` confirmed `Bn9BX` is an instance of `aic7V`, not a standalone frame, and that the icon-then-label child order is baked into the shared definition `aic7V` itself (`y91nX` icon child precedes `aNpk5` label child in `aic7V`'s own children array) — editing that order would mean editing the `LtgNm` component definition, which is out of scope (never edit a definition inside `LtgNm`).

Listed all `Button/Primary/*` variants in `LtgNm`: `j9c5W` (SM), `cb4rt` (MD), `aic7V` (LG), plus icon-only variants `RC4D7`/`pRZSi`/`uvgI7` (Icon/SM/MD/LG — checked `uvgI7`, confirmed icon-only with no label, not usable). **No trailing-icon (label-then-icon) `Button/Primary` variant exists.**

Per the no-variant fallback: hid the leading icon on the instance only via `Update("Bn9BX", {descendants: {"y91nX": {opacity: 0, width: 0}}})`. Screenshot after the change shows a clean "Create server" label, no overflow or collapse, though the button loses the trailing-icon look of the original (opacity: 0 removes visibility but not the icon's original layout completely — verified no visible gap/artifact remains).

**Node touched:** `Bn9BX` only, via `descendants` override (`y91nX.opacity`, `y91nX.width`) — the shared `aic7V` (Button/Primary/LG) component definition was not touched, and no other footer button (`wuwVx`/Back) was touched.

**Export method & validation:**

- **JSON:** `Get("UMJli", {depth: 30})` via the Pencil `execute` tool, zero `"..."` elision markers. Passes `python3 -m json.tool` validation. Contains `"content":"Create server"` once (footer `Bn9BX` override) and the `y91nX` override `"opacity":0,"width":0` once.
- **Screenshot:** `export_nodes` PNG export at 2x scale to `design-export/screenshots/UMJli.png`, 2880×1800, valid PNG.
- **Verification:** screenshot of `Bn9BX` before the edit showed a leading check icon then "Create server"; after the edit it shows "Create server" alone, centered, no broken/overflowing layout. Full `UMJli` screenshot confirms the rest of the review screen (template/version/configuration/network sections, YAML preview, footer Back button) is unchanged.

**Open question not resolved by this pass:** no `Button/Primary` variant in the component library supports a trailing icon, so the original design's "label then arrow/check icon" treatment cannot be reproduced without either a new component variant or a definition edit — both out of scope for a blind, scoped fix. Flagged for a design-judgement pass to decide whether to add a `Button/Primary/Trailing` variant.

**Context:**

Blind, scoped edit — only `Bn9BX`'s descendant override was touched; the `aic7V` shared definition and all other `Button/Primary/*`/`Button/Primary/Icon/*` variants were left untouched. Pencil does not auto-save; a GUI save is still pending before this change is durable across Pencil sessions.

## Incremental export 2026-09-06 — DxKOh / QgW58 / J5pjJ3 icon fixes

Three icon-correctness fixes across three screens, found via filtered `Get` sweeps (never a full-depth screen dump):

1. **`DxKOh` (Screen/Audit Log)** — the audit-integrity banner's "Re-check" button (`x8cjS`, a `Gameplane/Button/Ghost/SM`-shaped bespoke frame, not an instance) had a `pencil` icon on its icon child `x8cjS/eWkIT/WtPjS`. That read as an edit affordance on a button whose label is "Button"/re-check action, not an edit action — changed the icon to `plus`. `Update("x8cjS", {descendants: {"eWkIT/WtPjS": {icon: "plus"}}})`.
2. **`QgW58` (Screen/Admin Settings — Authentication (Save rejected))** — two separate fixes:
   - The "Helm-configured admin mapping" warning callout (`zg6cG`, a bespoke frame, not a HeroUI Alert instance) had a plain `triangle-alert` icon on its icon child `H4Cgu5`. Changed directly to `megaphone` to match the house warning-callout style: `Update("H4Cgu5", {icon: "megaphone"})`.
   - The bottom error banner `B89TO` ("saveErrorBanner", containing the text "Request failed (409): a server with that name already exists.") was a plain frame with only a text child (`E51IQ`) — no icon at all. Inserted a new leading icon child: `Insert("B89TO", {type: "icon", name: "Error Icon", icon: "alert-circle", library: "lucide", width: 16, height: 16, fill: "$danger/danger"})` then `Move(id, "B89TO", 0)`. The icon set in this document uses the renamed lucide id `circle-alert`, not `alert-circle` — the engine rejected `alert-circle` with an "icon not found" issue, corrected via a follow-up `Update(id, {icon: "circle-alert"})`. Final banner: `circle-alert` icon then the "409" text, both `$danger/danger`.
3. **`J5pjJ3` (Screen/Server Detail — Settings · Networking)** — three warning callouts (`znLuB` "Tailscale is tailnet-only", `q1zaXx` "Address preference ignored", `MLrud` "No address manager configured"), each a bespoke frame (not a HeroUI Alert instance) with a `triangle-alert` icon on child path `<instId>/JvjWQ`. All three changed to `megaphone`: `Update("znLuB", {descendants: {"JvjWQ": {icon: "megaphone"}}})` (and the same for `q1zaXx`, `MLrud`). A fourth `triangle-alert` icon (`eRj5l`) found by the same filtered sweep sits in the unrelated `snDangerZone` section (not a warning callout) and was correctly left untouched.

**Nodes touched:** `x8cjS` (descendant override only), `H4Cgu5` (direct, plain node), `B89TO` (new child `UEI7P` inserted + icon corrected), `znLuB`/`q1zaXx`/`MLrud` (descendant overrides only, path `JvjWQ`). No shared component definition (`LtgNm`) was touched; no root frame moved/deleted/duplicated.

**Export method & validation:**

- **JSON:** component-level `Get(id, {depth: 10[, resolveInstances: true]})` for each of the six touched nodes (`x8cjS`, `zg6cG`, `B89TO`, `znLuB`, `q1zaXx`, `MLrud` — `zg6cG` exported instead of bare `H4Cgu5` to keep the callout's full context), zero `"..."` elision markers, written to `design-export/json/<id>.json`. All six pass `python3 -m json.tool`. Full-screen `DxKOh.json`/`QgW58.json`/`J5pjJ3.json` were left as their existing (pre-existing, shallow sidebar-only) exports since the touched sub-nodes are not represented in that shallow depth and are now covered by their own component-level files.
- **Screenshots:** `export_nodes` PNG export at 2x scale for all six touched node ids to `design-export/screenshots/<id>.png`; all non-empty with real pixel dimensions. Also took full-screen screenshots of `DxKOh`, `QgW58`, `J5pjJ3` via `get_screenshot` to confirm no layout regression.
- **Verification:** each component screenshot visually confirms the corrected icon — `x8cjS` shows a `+` before "Button"; `zg6cG`/`znLuB`/`q1zaXx`/`MLrud` all show the megaphone glyph before their respective warning titles; `B89TO` shows a red circle-alert glyph before "Request failed (409): a server with that name already exists.". Full-screen screenshots show no overflow/collapse anywhere on the three screens.

**Context:**

Grep-first, blind-edit pass per CLAUDE.md rule 17 — filtered `Get` visitors located every candidate icon by type/value, no full-depth screen dump or judgement-based "reconcile" was performed. Pencil does not auto-save; a GUI save is still pending before this change is durable across Pencil sessions.

## Incremental export 2026-09-06 — zqzr4 Admin Settings action-row alignment fix

Investigated `zqzr4` (Screen/Admin Settings — Authentication, admin mapping warning): the "Save changes" button under the "Add identity provider" panel was left-aligned instead of right-aligned as in the original (`orig2/zqzr4.png`) and the sibling screen `QgW58`.

Located the button via a filtered `Get("zqzr4", (n, ctx) => n.content === "Save changes" ..., {depth: 30, resolveInstances: true})`, which returned the ancestor chain `HPFBR/q58In/U6q13B < HPFBR/q58In < HPFBR < Y6K3J < ...` — `HPFBR` is the button instance, `Y6K3J` its direct parent (the action row). The same filter on `QgW58` returned `wyW2y < Wdqop < Xng32 < xCAZJ < ...` — `Xng32` the button instance, `xCAZJ` its direct parent (the reference action row).

Compared the two action rows at `{depth: 1}`: `xCAZJ` (reference, correct) had `{justifyContent: "end", width: "fill_container", padding: [8,20,20,20], alignItems: "center", gap: 8}`; `Y6K3J` (buggy) had the same `width`/`padding`/`gap` but was **missing `justifyContent` and `alignItems`**, defaulting to left/start alignment. Verified the row had not been moved into the wrong parent: `LxJdk`'s children (`r3MEl, jhSx4, Y6K3J`) mirror `pDbdm`'s children (`jR1Pi, byGWB, xCAZJ`) — same structural position, three siblings with the action row last in both.

**Fix:** `Update("Y6K3J", {justifyContent: "end", alignItems: "center"})`. No move, no other property changed.

**Export method & validation:**

- **JSON:** `Get("zqzr4", {depth: 40, resolveInstances: true})` via the Pencil `execute` tool (output exceeded the tool's inline token limit and was saved to a harness tool-results file; extracted the `Print output` JSON line and validated with `python3 -m json.tool`). Zero elision — file is 337,452 bytes. Contains `"Save role mappings"` exactly once (the bottom row label).
- **Screenshot:** `export_nodes` PNG export at 2x scale to `design-export/screenshots/zqzr4.png`, 2880x4672, valid PNG.
- **Verification:** full-screen screenshot confirms "Save changes" is now right-aligned under the "Add identity provider" panel, all cards fit inside the 2336px frame, and the "Role mapping overrides" panel with its "Save role mappings" row is fully visible at the bottom — no overflow.

**Context:**

Grep-first, blind-edit pass per CLAUDE.md rule 17 — only `Y6K3J`'s own layout properties were changed; no screen-wide dump or judgement-based "reconcile" was performed. Pencil does not auto-save; a GUI save is still pending before this change is durable across Pencil sessions.

## Incremental export 2026-09-06 — x7MJI Connection Card HeroUI re-skin (GN3ni, AWAjU)

`x7MJI` (Gameplane/Connection Card — Tunnel States reference) still held two `lunaris` `c:ERkuB` Card instances — `GN3ni` "Connection Card (Tailscale)" and `AWAjU` "Connection Card (playit connecting)" — the only surviving `c:`-ref Card instances on that reference screen. Rebuilt both in place as plain HeroUI-styled frames.

For each card: inserted a new plain frame into the card's parent column (`aAPWT` for `GN3ni`, `B3x43y` for `AWAjU`) styled like `Card/Default` (`XDZ0E`) — `fill: "$surface/surface"`, `stroke: "$border/border"`, `cornerRadius: "$radius/3xl"`, `layout: "vertical"`, same `width: 399` as the old instance, `gap`/`padding` `0` (matching the `c:ERkuB` definition's own undefined gap/padding — its sections carry their own padding) — then inserted two child frames mirroring `Card Header` (`FRpsu`: `width: "fill_container"`, `layout: "vertical"`) and `Card Content` (`I9ZPb`: `width: "fill_container"`, `layout: "vertical"`, `gap: 4` pattern), each given the old instance's actual section padding (header `[20,24,12,24]`, content `[0,24,20,24]`, content `gap: 12`). Moved every real content child from the old instance's resolved descendant paths (e.g. `GN3ni/c:4zoFt/CGDUl`, `GN3ni/c:QMHOm/zjymb`) into the new frames in original order, then deleted the emptied old ref instance. Because `Insert` appends and the old instance was deleted afterward, the new card landed at the same child index automatically (`aAPWT`: `[u9bCb, XVJA4]`; `B3x43y`: `[z7Xfpm, o6lxF]` — same positions as `[caption, card]` before).

New node ids: `XVJA4` (Tailscale card, was `GN3ni`) with children `QhZSW` (Card Header) and `lOZGP` (Card Content); `o6lxF` (playit card, was `AWAjU`) with children `y53nlu` (Card Header) and `HGEjK` (Card Content). All original leaf content nodes (`CGDUl`, `zjymb`, `wT7p7`, `tWEy4`, `n0rynH`, `E9xw2f`, `YbjOG`, `RfbOQ`, `CA1Zv`, `oyd1p`, and their descendants) kept their original ids and content unchanged — only their parent chain changed.

**Engine quirk hit:** `Get(id, (n,ctx)=>{...(n.children||[]).map(...)`, `{depth:1}`)` visitors throw `TypeError: not a function` — in visitor mode `n.children` is not a plain array (accessing `.map` on it fails). Worked around by printing `n.id`/`n.type` per visited node instead of touching `n.children` directly; child order was inferred from the visit sequence.

**Residue check:** `Get("x7MJI", (n,ctx) => { if ((n.ref||"").startsWith("c:")) Print("RESIDUE", n.id, n.ref); if (n.type === "ref") ctx.skipChildren(); }, {depth: 30})` printed nothing — no `c:`-ref residue anywhere under `x7MJI`.

**Export method & validation:**

- **JSON:** `Get("x7MJI", {depth: 12})` via `execute`, zero `"..."` elision markers, written to `design-export/json/x7MJI.json`. Passes `python3 -m json.tool`. `grep -o '"content": "[^"]*"'` confirms all 18 original body strings survived verbatim (Unicode-escaped in the re-serialized JSON, e.g. `Tailscale — private (tailnet only)`, `mc-survival.tail4e2a.ts.net:25565`, `Waiting for tunnel address…`).
- **Screenshot:** `export_nodes` PNG export at 2x scale to `design-export/screenshots/x7MJI.png`, 1740x686, valid non-empty PNG.
- **Verification:** the screenshot shows both cards as bordered, rounded, dark-surface HeroUI cards with a "CONNECTION" header, matching content in the same rows as before (Tailscale: warning-bordered tunnel-address row with lock icon, "TUNNEL · TAILSCALE" eyebrow, tailnet-only badge, mono address, host/port rows; playit: neutral tunnel-address row with loader-circle icon, "TUNNEL · PLAYIT.GG" eyebrow, italic "Waiting for tunnel address…", host/port rows). No collapsed, overflowing, or broken layout.

**Context:**

Grep-first, blind-edit pass per CLAUDE.md rule 17 — the edit list (parent-styled frame + two section frames + child moves) was fixed in advance from the already-exported `x7MJI.json` and the `c:ERkuB`/`FRpsu`/`I9ZPb`/`XDZ0E` definitions; no full-depth judgement-based "reconcile" of the card was performed. Pencil does not auto-save; a GUI save is still pending before this change is durable across Pencil sessions.

## Incremental export 2026-09-06 — z9ShNE / rNhll HeroUI ref cleanup (CodeRabbit PR #349)

CodeRabbit flagged `z9ShNE` (Gameplane/Button/Small/Default) and `rNhll` (Gameplane/Button/Outline) as hand-built wrapper frames duplicating HeroUI button styling instead of referencing the HeroUI definitions.

`Get("z9ShNE", {depth: 2})` showed the inner `Button` child (`IVgQ8`) was **already** a `ref` instance of `j9c5W` (Button/Primary/SM) with the label override (`U1xvSo`: `{content: "Button"}`) intact — a prior pass had already done the ref swap. The only remaining defect was the wrapper `z9ShNE` itself still carrying its own duplicated `fill: "$accent/accent"` and `cornerRadius: "$radius/md"`, redundant with `j9c5W`'s own `fill`/`cornerRadius`.

`Get("rNhll", {depth: 2})` showed its inner `Button` child (`Q57b2`) was a `ref`, but wired to the **wrong** definition: `i6gfu` (Button/**Outline/MD**, 100×36) instead of the SM variant. Found the correct target via `Get("LtgNm", n => n.reusable && /^Button\/Outline\/SM$/.test(n.name||"") && Print(n.id, n.name), {depth: 2})` → `FIB65` (Button/Outline/SM, 84×32). The wrapper `rNhll` also carried its own duplicated `stroke: "$border/border"` and `cornerRadius: "$radius/3xl"`.

**Fix:**
- `z9ShNE`: `Update("z9ShNE", {fill: "#00000000", cornerRadius: 0})` — cleared the wrapper's duplicated fill/cornerRadius; visual styling now comes solely from the `j9c5W` ref child. Label/icon/size/position untouched.
- `rNhll`: `Replace("Q57b2", {...})` threw `TypeError: Cannot read properties of undefined (reading 'type')` on every path form tried (bare id, `rNhll/Q57b2`) — worked around with `Delete("Q57b2")` followed by `Insert("rNhll", {type: "ref", ref: "FIB65", name: "Button", descendants: {"zMNkR": {content: "Button"}}})` (new child id `wDXFJ`), preserving the "Button" label text on the new definition's own label node (`zMNkR`, since `FIB65`'s label child id differs from `i6gfu`'s `GqVun`). Then `Update("rNhll", {stroke: "#00000000", cornerRadius: 0})` cleared the wrapper's duplicated stroke/cornerRadius.
- Net effect on `rNhll`'s hug width: 132×36 → 116×36 (84px SM child + unchanged `[8, $spacing/4]` padding, down from the wrong 100px MD child) — height and position unchanged; this narrowing is the correct consequence of using the SM definition instead of the previously-mis-wired MD one.

**Verification:**
- Post-fix `Get` on both wrappers confirmed `fill`/`stroke`/`cornerRadius` are now `undefined` on both wrapper frames and on their ref children — all visual styling delegates to the HeroUI `j9c5W`/`FIB65` definitions.
- Screenshots of `z9ShNE` and `rNhll` in isolation show solid, cleanly-rounded pill buttons with no visible seam, gap, or double border between wrapper and inner ref.
- Screenshots of `dQV9N` (Screen/Server Detail — Settings · Share links (Empty), uses `z9ShNE` via a "Create link" instance) and `b4eaUf` (Screen/Server Detail — Capture — Start capture (Invalid filter), uses `rNhll` via a "Cancel" instance) confirm both screens render correctly post-fix — no broken, collapsed, or overflowing layout.

**Export method & validation:**

- **JSON:** `Get(id, {depth: 4})` via `execute` for both `z9ShNE` and `rNhll` (button compositions are shallow — depth 4 fully resolves the wrapper + its one ref child + descendant overrides with zero `"..."` elision), written to `design-export/json/z9ShNE.json` and `design-export/json/rNhll.json`. Both pass `python3 -m json.tool`.
- **Screenshots:** `export_nodes` PNG export at 2x scale to `design-export/screenshots/z9ShNE.png` and `design-export/screenshots/rNhll.png`, both non-empty valid PNGs.

**Context:**

Targeted, judgement-light fix per CLAUDE.md rule 17 spirit — only the two flagged wrapper nodes and `rNhll`'s single ref child were touched; no screen-wide dump or reconcile-by-eye was performed on either definition or on the two screens used only for regression screenshotting. Pencil does not auto-save; a GUI save is still pending before this change is durable across Pencil sessions.

## Incremental export 2026-09-06 — EZFW0 Server Detail Overview: restored missing metric-card sparklines

`EZFW0`'s three metric cards (CPU `yyxoE`, Memory `d9mRd`, Disk `E62N4`) had lost their sparkline trend lines — each card's `path` node (`XCNbc`, `IMSDo`, `bjZE6`) existed structurally (same `type`, `viewBox`, `stroke`, `strokeWidth` as the working sibling screen `IzuY2`) but rendered as a blank/invisible line. `n.geometry` reads as the literal 3-character placeholder `"..."` via a normal `Get` (confirmed via `.length`/char-code checks) regardless of whether the path actually renders — `Get`'s `includePathGeometry: true` option (undocumented in the app-state summary; found via the `execute` API's own inline TS signature) is required to see real path data, and copying `n.geometry` through `Update` did not restore the visual (the value read back was the same elided placeholder on both the broken and working nodes, so a naive `Update(target, {geometry: sourceGeometryValue})` was a no-op).

**Fix:** used the native `Copy(path, parent, copyNodeData)` primitive instead of reading/writing the `geometry` property, since `Copy` duplicates the node's actual internal data rather than round-tripping it through the elided JS reflection:
- `Delete("XCNbc")` → `Copy("OB5ve", "N7x6Xs", {})` (from `IzuY2`'s CPU card) → `Move(<newId m72bbw>, "N7x6Xs", 2)` to restore its position between the "0%" text and the progress-bar frame.
- `Delete("IMSDo")` → `Copy("Kkgv2", "A5w3dO", {})` (from `IzuY2`'s Memory card) → `Move(<newId KGZYS>, "A5w3dO", 2)`.
- `Delete("bjZE6")` → `Copy("KVGfX", "XNdKY", {})` (from `IzuY2`'s Disk card) → `Move(<newId vdDJJ>, "XNdKY", 2)`.

**Verification:** screenshots of each metric card (`yyxoE`, `d9mRd`, `E62N4`) individually and the full `EZFW0` screen, compared against `IzuY2`'s equivalents and the reference screenshot supplied for this task — all three sparklines now render (pink/purple/green zigzag lines matching their card's accent color), no layout breakage, overflow, or clipping. A follow-up `Get(..., {includePathGeometry: true})` on the new nodes confirmed real `M0 16l12-3...`-style path data (not the `"..."` placeholder).

**Export method & validation:**
- **JSON:** `Get("EZFW0", {depth: 20, includePathGeometry: true, resolveInstances: true})`, written to `design-export/json/EZFW0.json`. Passes `python3 -m json.tool`; zero `"..."` elision markers (`grep -c '"\.\.\."'` = 0); content-validated via `grep -o '"Saved the game"'` (unique body text on this screen).
- **Screenshot:** `export_nodes` PNG export at 2x scale to `design-export/screenshots/EZFW0.png` (2880×2600, non-empty RGBA).

**Context:** Only the three sparkline path nodes on `EZFW0` were touched (delete + copy + reposition); no other node on the screen was read or modified. Pencil does not auto-save — a GUI save is still pending before this change is durable across Pencil sessions.

## Incremental export 2026-09-06 — kIxaJ Audit Integrity Banner: replaced last lunaris `c:` instance

Residue check (`Get("kIxaJ", (n,ctx)=>{ if((n.ref||"").startsWith("c:")) Print("RESIDUE",n.id,n.ref,n.name); if(n.type==="ref") ctx.skipChildren(); }, {depth:30})`) printed exactly one hit: `RESIDUE aAaPa c:YZjRF Integrity Banner (Broken)` — the reusable `kIxaJ` composition's only child besides its own reference-label text (`ZEqkj`) was a `ref` into the lunaris library (`c:YZjRF`, named "Integrity Banner (Broken)").

`Get("aAaPa", ..., {depth:5, resolveInstances:true})` showed the resolved instance: a horizontal `frame` (`fill_container` width, `$danger/danger` fill, `gap:12`, `padding:16`, `alignItems:"center"`) containing a 24×24 icon frame (`aAaPa/c:LB3Fl`, fill `$foreground/foreground`) wrapping a hand-drawn wave/pulse vector path (`aAaPa/c:FTYwP`), and a text child (`fuKsy`, content `"Integrity check failed — chain breaks at event #286"`, fill `$danger/foreground`, `fontSize:16`, `fontWeight:"500"`, `fontFamily:"$typography/font-sans"`). The reference screenshot (`design-export/screenshots/kIxaJ.png`, pre-edit) confirmed the same: a solid red banner with a dark icon chip (pulse icon) and bold white message text.

No HeroUI `Alert/*` definition (`CEGPG`, `O3T14b`, `v0xtri`, `Llzos`, `f7KBn` Danger, `r0CzE`, `JCxNu`, `uJ3KZ`) matches this single-line full-width banner shape — all eight are title+description cards on a light `$surface/surface` background at fixed `width:540`. Per the task's guidance that colours/shapes may differ as long as content/rows/icons/sizes/position match, rebuilt the banner as a plain frame styled like the original (not literally re-parented under an `Alert/*` definition, since none fit), replacing only the lunaris vector icon with the closest semantic lucide icon (`activity` — a pulse/waveform, matching the original hand-drawn squiggle) and keeping the exact original text content and styling.

**Fix:**
- `Insert("kIxaJ", {type:"frame", name:"Integrity Banner (Broken)", width:"fill_container", fill:"$danger/danger", gap:12, padding:16, alignItems:"center"})` → new id `m1hP1j`.
- `Insert(m1hP1j, {type:"frame", name:"Icon", width:32, height:32, fill:"$foreground/foreground", cornerRadius:6, padding:4, alignItems:"center", justifyContent:"center"})` → `gJjR4`, then `Insert(gJjR4, {type:"icon", library:"lucide", icon:"activity", width:20, height:20, fill:"$danger/foreground"})` → `z04z2`.
- `Insert(m1hP1j, {type:"text", name:"Message", content:"Integrity check failed — chain breaks at event #286", fill:"$danger/foreground", lineHeight:1.5, fontFamily:"$typography/font-sans", fontSize:16, fontWeight:"500"})` → `Mtc22` (text copied verbatim from the old `fuKsy` node).
- `Delete("aAaPa")` (the emptied lunaris ref; not a root frame — `kIxaJ` itself is untouched and keeps its id/size).
- `Move("m1hP1j", "kIxaJ", 1)` — restored the new node to `aAaPa`'s original index (1) among `kIxaJ`'s two children.

**Verification:**
- Residue check re-run post-fix: zero output — no `c:`-prefixed `ref` remains under `kIxaJ`.
- Screenshot of `kIxaJ` post-fix shows a solid red banner, dark rounded icon chip with a white pulse icon, and bold white "Integrity check failed — chain breaks at event #286" text — matching the pre-edit reference screenshot's layout, proportions, and content; no broken, collapsed, or overflowing layout.

**Export method & validation:**
- **JSON:** `Get("kIxaJ", {depth: 15})` via `execute` (zero `"..."` elision markers — the whole composition is 2 levels deep past `kIxaJ`), written to `design-export/json/kIxaJ.json`. Passes `python3 -m json.tool`; content-validated via `grep -o "chain breaks at event #286"` (unique body text, one hit).
- **Screenshot:** `export_nodes` PNG export at 2x scale to `design-export/screenshots/kIxaJ.png` (1400×312, non-empty RGBA).

**Context:** `kIxaJ` is `Gameplane/Audit Integrity Banner`, a reusable component — its root id (`kIxaJ`) and outer size (700×hug, per its own `width:700` / vertical layout) were not touched; only the inner `aAaPa` slot was replaced. Pencil does not auto-save — a GUI save is still pending before this change is durable across Pencil sessions.

## Incremental export 2026-09-06 — RC3Kf Admin Settings — Backup destinations: replaced last lunaris `c:` instance

Residue check (`Get("RC3Kf", (n,ctx)=>{ if((n.ref||"").startsWith("c:")) Print("RESIDUE",n.id,n.ref,n.name); if(n.type==="ref") ctx.skipChildren(); }, {depth:30})`) printed exactly one hit: `RESIDUE t86T1 c:ERkuB Settings Panel` — the "Backup destinations" card in the settings-layout body was a lunaris library instance.

`Get("t86T1", ..., {depth:5, resolveInstances:true})` showed the resolved instance: an 888×159 vertical frame (`$surface/surface` fill) with a Card Header sub-frame (title text `BwzPk` "Backup destinations", description text `U2CRgX` "Restic repositories for snapshots. Stored as labelled Kubernetes Secrets in the configured namespace."), a Card Content sub-frame (empty-state text `NMici` "No backup destinations configured. Add one to enable snapshots."), and a Card Actions sub-frame containing a pink "+ Add destination" button (`wFu1e`, already a plain `Gameplane/Button/Default` (`tpKRk`) ref, not a lunaris one). The reference screenshot (`orig2/RC3Kf.png`) confirmed the same layout.

Rebuilt as a plain frame styled like the HeroUI `XDZ0E` Card/Default definition (`width:888`, vertical layout, `fill:"$surface/surface"`, `cornerRadius:"$radius/3xl"`, outer shadow effect matching `XDZ0E`), with `FRpsu`/`I9ZPb`-style Card Header / Card Content sub-frames plus a Card Actions sub-frame, moving the three real text nodes and the existing button ref into the new structure instead of recreating them.

**Fix:**
- `Insert("rMHXz", {type:"frame", name:"Card/Default", width:888, layout:"vertical", fill:"$surface/surface", cornerRadius:"$radius/3xl", effect:{type:"shadow", shadowType:"outer", color:"#0000000A", offset:{x:0,y:2}, blur:4}, x:244, y:0})` → new id `kq3Lz`.
- `Insert(kq3Lz, {type:"frame", name:"Card Header", layout:"vertical", gap:4, padding:[20,20,16,20]})` → `Lo8oV`; `Insert(kq3Lz, {type:"frame", name:"Card Content", layout:"vertical", gap:16, padding:[0,20]})` → `pROKa`; `Insert(kq3Lz, {type:"frame", name:"Card Actions", layout:"horizontal", justifyContent:"flex-end", gap:8, padding:[8,20,20,20]})` → `fOMFH`.
- `Move("BwzPk", "Lo8oV")`, `Move("U2CRgX", "Lo8oV")`, `Move("NMici", "pROKa")`, `Move("wFu1e", "fOMFH")` — moved the real title, description, empty-state text and button into the new frames; `Update` calls set `width:"fill_container"` on the three new sub-frames and the three moved text nodes to resolve the resulting collapsed-size warnings.
- `Delete("t86T1")` (the emptied lunaris ref; not a root frame).
- `Move("kq3Lz", "rMHXz", 1)` — placed the new card at `t86T1`'s original index (1, after `X4OyDd` Settings Nav) among `rMHXz`'s children.

**Verification:**
- Residue check re-run post-fix: zero output — no `c:`-prefixed `ref` remains under `RC3Kf`.
- Screenshot of `RC3Kf` post-fix shows the same "Backup destinations" card — title, description, empty-state message, and pink "+ Add destination" button in the same 888×159 position/size as the reference screenshot; no broken, collapsed, or overflowing layout.

**Export method & validation:**
- **JSON:** `Get("RC3Kf", {depth: 40})` via `execute` (zero `"..."` elision markers), written to `design-export/json/RC3Kf.json`. Passes `python3 -m json.tool`; content-validated via `grep -c "Restic repositories for snapshots"` (unique body text, one hit).
- **Screenshot:** `export_nodes` PNG export at 2x scale to `design-export/screenshots/RC3Kf.png` (2880×1800, non-empty RGBA).

**Context:** Only the `t86T1` card slot inside `RC3Kf`'s Settings Layout body was touched; the sidebar nav, top bar, and page header were untouched. Pencil does not auto-save — a GUI save is still pending before this change is durable across Pencil sessions.

## Incremental export 2026-09-06 — g5mEpx Admin Settings — Module sources: replaced last lunaris `c:` instance

Residue check (`Get("g5mEpx", (n,ctx)=>{ if((n.ref||"").startsWith("c:")) Print("RESIDUE",n.id,n.ref,n.name); if(n.type==="ref") ctx.skipChildren(); }, {depth:30})`) printed exactly one hit: `RESIDUE SuA1e c:ERkuB Settings Panel` — the "Module sources" card in the settings-layout body was a lunaris library instance.

`Get("SuA1e", ..., {depth:5, resolveInstances:true})` showed the resolved instance: an 888×229 vertical frame (`$surface/surface` fill, `cornerRadius:10`, `$border/border` stroke) with a Card Header sub-frame (title text "Module sources", description text "Registries, git repositories, archives, and uploads the operator pulls module bundles from. Equivalent to applying ModuleSource resources directly.", pink "+ Add source" button `nsclG`), and a Card Content sub-frame containing two source rows — `srcDefault` (green check icon, "default" + GIT type badge, repo URL, "1h" sync interval, "6 modules · synced 5m ago", edit/delete icon buttons) and `srcUploads` (same layout, "uploads" + UPLOAD badge, "uploaded bundles", "0 modules · synced 6m ago"). A third, unused "Card Actions" sub-frame (a hidden/off-canvas leftover "Save changes" button, `w:0` at `x:-17185`) was not visible in either the live render or the reference screenshot (`orig2/g5mEpx.png`) and was dropped. Unlike `RC3Kf`'s `t86T1`, the two content sub-frames here (`r4EMY` "Card Header", `P5Z1p4` "Card Content") were already real addressable nodes (not virtual override paths), matching the HeroUI `XDZ0E`/`FRpsu`/`I9ZPb` pattern structurally — only the outer wrapper was the lunaris ref.

Rebuilt as a plain frame carrying the same style properties read off the resolved instance (`width:"fill_container"`, `fill:"$surface/surface"`, `cornerRadius:10`, `stroke:"$border/border"`, `strokeWidth:1`, `strokeAlignment:"inner"`, `layout:"vertical"`, `clip:true`), then copied the two real content sub-frames into it (a direct `Move` was rejected: `Cannot move descendants of instances!`, since `r4EMY`/`P5Z1p4` were still nested under the `SuA1e` ref at the time).

**Fix:**
- `Insert("isahC", {type:"frame", name:"Settings Panel", clip:true, width:"fill_container", fill:"$surface/surface", cornerRadius:10, stroke:"$border/border", strokeWidth:1, strokeAlignment:"inner", layout:"vertical", children:[]})` → new id `ukvLQ`.
- `Copy("r4EMY", "ukvLQ", {})` → `GUF1f` (Card Header); `Copy("P5Z1p4", "ukvLQ", {})` → `msu32` (Card Content) — copied rather than moved, since `Move` on a node still nested under the `SuA1e` ref instance fails with "Cannot move descendants of instances!".
- `Delete("SuA1e")` (the emptied lunaris ref, including its unused Card Actions leftover; not a root frame).
- `Move("ukvLQ", "isahC", 1)` — placed the new card at `SuA1e`'s original index (1, after `pLiS1` Settings Nav) among `isahC`'s children.

**Verification:**
- Residue check re-run post-fix: zero output — no `c:`-prefixed `ref` remains under `g5mEpx`.
- Screenshot of `g5mEpx` post-fix shows the same "Module sources" card — title, description, pink "+ Add source" button, and both `default`/`uploads` source rows (badges, sync info, edit/delete icons) in the same position/size as the reference screenshot; no broken, collapsed, or overflowing layout.

**Export method & validation:**
- **JSON:** `Get("g5mEpx", {depth: 40})` via `execute` (zero `"..."` elision markers), written to `design-export/json/g5mEpx.json`. Passes `python3 -m json.tool`; content-validated via `grep -c "Module sources"` (unique body text plus nav-item name, hit found).
- **Screenshot:** `export_nodes` PNG export at 2x scale to `design-export/screenshots/g5mEpx.png` (2880×1800, non-empty RGBA).

**Context:** Only the `SuA1e` card slot inside `g5mEpx`'s Settings Layout body was touched; the sidebar nav, top bar, and page header were untouched. Pencil does not auto-save — a GUI save is still pending before this change is durable across Pencil sessions.
