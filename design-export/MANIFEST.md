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
| `rNhll` | Gameplane/Button/Outline | Button composition | Button/Outline/MD variant from HeroUI definitions. |
| `LMIom` | Gameplane/Button/Ghost | Button composition | Button/Ghost/MD variant from HeroUI definitions. |
| `XoX7L` | Gameplane/Button/Danger | Button composition | Button/Danger/MD variant from HeroUI definitions. |
| `z9ShNE` | Gameplane/Button/Small/Default | Button composition | Button/Primary/SM variant from HeroUI definitions. |
| `d5N3W3` | Gameplane/Button/Small/Outline | Button composition | Button/Outline/SM variant from HeroUI definitions. |
| `J09iP` | Gameplane/Button/Small/Ghost | Button composition | Button/Ghost/SM variant from HeroUI definitions. |
| `IU7OG` | Gameplane/Button/Small/Danger | Button composition | Button/Danger/SM variant from HeroUI definitions. |
| `D0cDM` | Gameplane/Input | Form atom | Bare field row (280×40) restyled from HeroUI Input/Primary's field frame (`$field/background`, `$radius/xl`, `$field/border`, `$field/placeholder`), no label/description — matches the original atom's bare-field shape. |
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
| `w4ntSc` | Gameplane/Loading Card | Composition | Card with Spinner shown during async data loads. |
| `zzx8f` | Gameplane/Error Card | Composition | Alert (danger) + Card composition for failed-load states. |
| `igj2U` | Gameplane/Error Banner | Composition | Alert (danger) for inline error messages (failed save, provisioning failure). |

**Export method & validation:**

- **JSON:** `Get(id, {depth: ≥12, includePathGeometry: true})` via the Pencil `execute` tool for each of the 25 components (button components at depth 12; `LtgNm` iterated at depth 10–12 per child, per the section above). All exports executed at depth ≥ 12 (`LtgNm` 10–12) to ensure no `"..."` elision markers appear in structural fields.
- **Validation of truncation markers:** Programmatic check of all 25 JSON files (`python3 json.load()` + a `"..."` scan) confirmed zero `"..."` as Pencil truncation markers in structural fields (`"children": "..."`, `"geometry": "..."`, etc.), and top-level `"id"` matches the filename in every file. `LtgNm` contains two instances of `"..."` as actual component content — the `Pagination/Ellipsis` (`i18Al2`) symbol and `tableEx` (`Q0ilUf`) cell text — confirmed by path (`.content` fields on leaf nodes `EHTqO`, `DpNvd`), not a structural elision. For the six button components (`tpKRk`, `rNhll`, `LMIom`, `XoX7L`, `z9ShNE`, `d5N3W3`), the first child is a `ref` node pointing at a HeroUI Button definition (`cb4rt`, `i6gfu`, `jsrtu`, `CFM8i`, `j9c5W`, `FIB65` respectively).
- **Screenshots:** `export_nodes` PNG export of each component at 2× scale to `/design-export/screenshots/<id>.png`. All 25 PNG files present, verified non-empty and valid PNG via `file`.
- **File inventory:** All 25 components have both `json/<id>.json` and `screenshots/<id>.png` files in design-export/, timestamped 2026-09-04 (git status confirms all 25 ids' json+png as modified).
- **Verification command:** `for id in LtgNm tpKRk rNhll LMIom XoX7L z9ShNE d5N3W3 J09iP IU7OG D0cDM qvQPg Lmaf1 AT7ya hl7R3 rh2QH k38Uta ZWcwn xCDF7 x3beP WwNlX BPEpm FyV6E w4ntSc zzx8f igj2U; do grep -c "$id" design-export/MANIFEST.md; done` — each id appears exactly once in this section (grep returns 1).

**Context:**

These 25 components constitute the Phase 2 Foundation (Slice 0, second half) atom layer. They are the complete set of reusable, redrawn-from-HeroUI definitions (including the foundational HeroUI library frame, `LtgNm`, which all downstream components and screens compose from) that all subsequent user-story slices (1–5) will compose into screens. No screen rebuilds can proceed until this atom layer is complete and exported. The HeroUI library frame (`LtgNm`) was exported as the foundational deliverable; Tasks T011–T014 redrew the 24 Gameplane atom components in design.pen; Task T016 exports all 25 components here and updates MANIFEST.md. Tasks T019–T030 (Phase 2, code wave) implement the TypeScript component wrappers in `web/src/components/hero/`. Task T017 (token mapping in `web/src/styles/globals.css`) parallels the design-wave work.
