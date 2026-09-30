# H14 design brief: Capture tab no-access state

Group: H14 (`fix/018-harden-dashboard-permission-gates`). Design scouting only. design.pen was not edited.

## Verdict

**needsDesign = true.** One new frame: the Capture tab as seen by a user who can't manage captures on this server. Mods and server actions need no design work.

## What the scout checked (read-only)

- **Capture tab frames in design.pen:**
  - `Bbnga`: Capture (Not enabled)
  - `dBILX`: Capture (Empty)
  - `xvlB6`: Capture (Running)
  - `m5kOm4`: Capture (List)
  - `O08uaD` and `b4eaUf`: Start capture dialogs
  - None of them has a no-access state.
- **Settings → Network capture (`RodrS`):**
  - The only permission-related text is `BG3te`: "Network packet capture requires admin access…".
  - The React warning line "You don't have permission to change capture settings for this server." (`web/src/routes/tabs/settings/NetworkCapture.tsx:143-147`) is **not** in design.pen either.
  - That line is outside this group's scope. It's noted here only so the orchestrator knows the design and the code already differ there.
- **API** (`api/internal/rbac/rbac.go:202-209`): every capture route needs `captures:manage` in the server's namespace, including the list, get and file GETs. So a user without it can't see the capture list at all. The whole tab body has to be replaced by one notice. Disabling individual buttons isn't enough.
- **Mods** (`web/src/routes/tabs/Mods.tsx:68`) and `ServerActionsMenu`: the fix only adds the namespace argument to `can()`. The existing enabled and disabled visuals stay as they are, so no design is needed.

## State to design: "Capture (No access)"

The source frame is `Bbnga`: Screen/Server Detail — Capture (Not enabled). It already shows the correct Capture tab selected and a centred notice card.

Run this as one `execute` on `/home/valgul/project/Gameplane/design.pen`:

```js
pos = FindEmptySpace({width: 1440, height: 900, nodeId: "Bbnga", direction: "bottom", padding: 100})
newId = Copy("Bbnga", document, {
  name: "Screen/Server Detail — Capture (No access)",
  x: pos.x, y: pos.y,
  descendants: {
    "A3bwvs": {name: "No Access Card"},
    "xNgDJ":  {name: "Lock Icon", icon: "lock"},
    "HHXSt":  {content: "You don't have access to packet capture on this server."},
    "OJNGw":  {content: "Viewing, starting and downloading captures requires capture access for this server. Ask an administrator if you need it."},
    "rzJ4W":  {enabled: false},
    "faDKK":  {enabled: false},
    "nLDsn":  {enabled: false}
  }
})
Print(newId)
```

The per-node changes, in the same form (source node id → change):

| Source id | Node | Change |
|---|---|---|
| `Bbnga` | frame | Copy it to document root. Name it `Screen/Server Detail — Capture (No access)`. Place it with FindEmptySpace below `Bbnga`. |
| `A3bwvs` | Not Enabled Card | Rename to `No Access Card`. Keep the style (480 wide, `$surface/surface`, 10 radius, 32 padding). |
| `m2GJH` | Icon Wrap | No change. |
| `xNgDJ` | Capture Icon | Set `icon: "lock"` (lucide) and rename to `Lock Icon`. Keep fill `$muted` and size 28. |
| `HHXSt` | Status text | Set content to: `You don't have access to packet capture on this server.` |
| `OJNGw` | Info text | Set content to: `Viewing, starting and downloading captures requires capture access for this server. Ask an administrator if you need it.` |
| `rzJ4W` | Btn Row (Enable Capture button `Vngv8`) | `enabled: false` (hidden). |
| `faDKK` | Error Banner instance (`igj2U`) | `enabled: false` (hidden). |
| `nLDsn` | Admin Hint text | `enabled: false` (hidden). |

- No components are inserted. The sidebar, top bar, detail header and tabs bar are kept as copied.
- If the Copy `descendants` keys by source id are rejected, apply the same changes to the copied frame's children, found by name ("Not Enabled Card", "Capture Icon", "Status", "Info", "Btn Row", "Cluster Disabled Error", "Admin Hint").

**Verify:**
- Take `get_screenshot` of the new frame. The card should show only the lock icon, the heading and the one paragraph, centred, with no button.
- The card should shrink to fit its content. It's a vertical auto-layout, so no manual resize should be needed.

**Export (rule 1):**
- Run the `design-export` skill for the new frame id.
- That writes `design-export/json/<newId>.json` and `design-export/screenshots/<newId>.png`.
- Commit both with the design change.

## Behaviour the React fix should match (for the code brief, not the design)

- `CaptureWidget` renders this card when `/users/me` has resolved and `!can(me, "captures:manage", ns)`.
- This takes priority over the not-enabled, empty, running and list states.
- While `/users/me` is loading, render no capture controls (fail-closed, the same rule as `NetworkCapture.tsx:101-104`).
- Don't show the no-access card during loading.
- Disable the captures list query when the user lacks the permission.
- The Capture tab itself stays visible, so the tab bar doesn't shift between users.

Copy tone: this matches the existing "Capture is not enabled on this server." card. It uses sentence case, has no permission keys, and uses the generic "capture access" wording.
