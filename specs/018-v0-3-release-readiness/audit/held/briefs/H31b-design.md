# H31b design brief: owner-only server operations, disabled states

Group: H31b, the dashboard half of H31 (`fix/018-harden-server-owner-operations`). Design scouting only. design.pen was not edited.

Waits for OD-025: the maintainer does the design.pen pass on the other machine. The design edits below still need the user's OK before anyone runs them (rule 1; Pencil screenshots are blind).

## Verdict

**needsDesign = true.** design.pen has no disabled state for any of the three surfaces. Three new frames:

1. **Server actions menu, owner-only items disabled**, with the hint tooltip.
2. **Settings > Danger zone, owner-only actions disabled**, with an explanatory notice.
3. **Settings > Access, collaborator editing unavailable**, with an explanatory line.

The rule (D18) that the copy has to express: only the server's owner or a full admin can transfer ownership, change collaborators, wipe world data or delete the server. Everything that goes into design.pen below is neutral UI copy.

## What the scout checked (read-only)

- **React (what the fix will change):**
  - `web/src/components/server/ServerActionsMenu.tsx:37-39`: `canManage` = owner, or `servers:write` in the namespace. Transfer (`:63-69`) and Wipe (`:70-77`) show the hint "Requires owner or operator role". Delete (`:79-85`) is disabled with **no** hint.
  - `web/src/components/ui/DropdownMenu.tsx:43-74`: `hint` shows only as a `title` tooltip and in the `aria-label`. Disabled items get `opacity-50`.
  - `web/src/routes/tabs/settings/Danger.tsx`: no gate at all. The three buttons are always enabled. Button labels: "Wipe world…" (ghost), "Transfer…" (ghost), "Delete" (danger).
  - `web/src/routes/tabs/settings/Access.tsx:54-61`: same `canManage` rule. For non-managers, the remove "x" on each chip and the add row are **hidden**, with no explanation.
  - Tone precedent: "Requires operator role" (`ServerActionsMenu.tsx:61`, `Mods.tsx:259`, `Modpacks.tsx:128`, `ServerActionsCard.tsx:258`) and "Only the owner can create or revoke links." (`ShareLinks.tsx:534`, design node in `dQV9N`).
- **design.pen nodes (live file, confirmed with `Get`):**
  - `BPEpm` Gameplane/Dropdown Menu (reusable, 220 wide, no theme pin). Items: `Cii7d` Clone (ref `W8XiN`), `NhChV` Transfer ownership (ref `W8XiN`), `V7Slhc` Wipe world data (ref `W8XiN`, danger fills), `s5z5i` divider (ref `z1N6Y`), `kdfaC` Delete server (ref `xjWNb`). No disabled item exists anywhere in the menu components (`W8XiN`, `xjWNb`, `B5sYje`).
  - `hIoOw` Tooltip (reusable). Text child `rWONa`, 12px.
  - `CEGPG` Alert/Default (reusable, 540 wide). Icon `upZAA` (lucide `info`), title `nJnfr`, description `zD5GP`.
  - `XR0f9` Screen/Server Detail — Settings · Danger zone (canonical per `design-export/MANIFEST.md:446`, at x=37000 y=10375, dark pin). Form Body `jsVZP` (vertical, gap 20, padding 24) holds:
    - `yvjKT` Wipe World Card: button `vPgdl` (ref `J09iP` Small/Ghost, inner `eWkIT` ref `rkF0p`, label `r4VbAi`).
    - `a11jT8` Transfer Ownership Card: button `GytYJ` (ref `J09iP`).
    - `yCKdP` Delete Server Card: button `Xctmb` (ref `XoX7L` Danger, inner `C8QRt`, label `t8p1M2`).
    - `i8wib` is an older duplicate with the same title (MANIFEST lists it under a different name). Don't use it.
  - `QpEvu` Screen/Server Detail — Settings · Access (x=35360 y=10375). Owner Card `htsxm` (owner value text `aKIDz` = "—"). Collaborators Card `kRdsz` (vertical, gap 12): `Kq2KY` title, `P4rOBk` description, `Slq9d` "None yet" (`$muted`, 14px, `$typography/font-sans`), `azK6C` add row (input `l2FPN6`, Add button `hrSpe`).
- **A difference already in the design, outside this group's scope:** in `XR0f9` the Wipe and Transfer buttons (`vPgdl`, `GytYJ`) have no label override, so they render "Discard" (the `J09iP` default). The new Danger frame below sets the right labels. Whether `XR0f9` itself should be fixed is the maintainer's call. It's a one-line `Update` per button, using the same keys as the Copy below.

## Final copy (single source for design and React)

| Where | Text |
|---|---|
| Menu hint on Transfer ownership, Wipe world data and Delete server when disabled | `Requires server owner or admin` |
| Danger zone notice, title | `Owner-only actions` |
| Danger zone notice, description | `Only the server owner or an admin can wipe, transfer or delete this server.` |
| Access tab, Collaborators card, in place of the add row | `Only the server owner or an admin can change collaborators.` |

The Clone server hint ("Requires operator role") is unchanged.

## State 1: Server actions menu, owner-only items disabled

Source: component `BPEpm`, used as an instance, plus a `hIoOw` Tooltip instance. Run as **one** `execute` on `/home/valgul/project/Gameplane/design.pen`:

```js
pos = FindEmptySpace({width: 520, height: 220, nodeId: "BPEpm", direction: "right", padding: 60})
st = Insert(document, {type: "frame", name: "State/Server Actions Menu — Owner-only disabled",
  x: pos.x, y: pos.y, width: 520, height: 220, layout: "none",
  theme: {semantic: "dark"}, fill: "$background/background", clip: true})
Insert(st, {type: "ref", ref: "BPEpm", name: "Menu", x: 16, y: 16,
  descendants: {
    "NhChV":  {opacity: 0.5},
    "V7Slhc": {opacity: 0.5},
    "kdfaC":  {opacity: 0.5}
  }})
Insert(st, {type: "ref", ref: "hIoOw", name: "Hint Tooltip", x: 244, y: 61,
  descendants: {"rWONa": {content: "Requires server owner or admin"}}})
Print(st)
```

| Node | Change |
|---|---|
| new frame | Name `State/Server Actions Menu — Owner-only disabled`, 520×220, dark pin, `$background/background` fill, placed right of `BPEpm`. |
| `BPEpm` instance | At (16,16). `NhChV` (Transfer ownership), `V7Slhc` (Wipe world data) and `kdfaC` (Delete server): `opacity: 0.5`. `Cii7d` (Clone) unchanged. |
| `hIoOw` instance | At (244,61), next to the Transfer row (menu x 16 + 220 + 8; Transfer row centre ≈ y 74). `rWONa` content: `Requires server owner or admin`. |

Check with one `get_screenshot` of the new frame: three dimmed items, Clone at full opacity, tooltip beside Transfer and not clipped. If the tooltip clips, widen the frame to 560.

## State 2: Settings > Danger zone, owner-only actions disabled

Source frame `XR0f9`. Run as **one** `execute`:

```js
pos = FindEmptySpace({width: 1440, height: 900, nodeId: "XR0f9", direction: "right", padding: 100})
d = Copy("XR0f9", document, {
  name: "Screen/Server Detail — Settings · Danger zone (Owner-only)",
  x: pos.x, y: pos.y,
  descendants: {
    "vPgdl":             {opacity: 0.5},
    "vPgdl/eWkIT/r4VbAi": {content: "Wipe world…"},
    "GytYJ":             {opacity: 0.5},
    "GytYJ/eWkIT/r4VbAi": {content: "Transfer…"},
    "Xctmb":             {opacity: 0.5},
    "Xctmb/C8QRt/t8p1M2": {content: "Delete"}
  }
})
body = Get(d, n => n.name === "Form Body" ? n.id : undefined)[0]
note = Insert(body, {type: "ref", ref: "CEGPG", name: "Owner-only Notice", width: "fill_container",
  descendants: {
    "upZAA": {icon: "lock"},
    "nJnfr": {content: "Owner-only actions"},
    "zD5GP": {content: "Only the server owner or an admin can wipe, transfer or delete this server."}
  }})
Move(note, body, 0)
Print(d, body, note)
```

| Source id | Node | Change |
|---|---|---|
| `XR0f9` | screen | Copy to document root. Name it `Screen/Server Detail — Settings · Danger zone (Owner-only)`. Place it with FindEmptySpace right of `XR0f9` (currently x=38540, y=10375). |
| `vPgdl` | Wipe button | `opacity: 0.5`. Label `vPgdl/eWkIT/r4VbAi` → `Wipe world…`. |
| `GytYJ` | Transfer button | `opacity: 0.5`. Label `GytYJ/eWkIT/r4VbAi` → `Transfer…`. |
| `Xctmb` | Delete button | `opacity: 0.5`. Label `Xctmb/C8QRt/t8p1M2` → `Delete` (unchanged text, stated so the override is explicit). |
| copy of `jsVZP` | Form Body | Insert a `CEGPG` (Alert/Default) instance named `Owner-only Notice`, `width: fill_container`, as the **first** child. Icon `upZAA` → `lock`. Title `nJnfr` → `Owner-only actions`. Description `zD5GP` → `Only the server owner or an admin can wipe, transfer or delete this server.` |

The card texts, the danger border on the Delete card, and the Form Footer (`vlNFQ`) all stay as they are in the source.

Check with one `get_screenshot` of the copied Settings Panel Wrap (find it by name `Settings Panel Wrap` under `d`): the notice sits above the three cards, all three buttons are dimmed with the right labels, and nothing is clipped. The body grows by about 90px. If the Form Footer clips at the 900 frame height, set the copied screen's `height` to 1000.

## State 3: Settings > Access, collaborator editing unavailable

Source frame `QpEvu`. Run as **one** `execute`, **after** State 2 so FindEmptySpace skips the new Danger frame:

```js
pos = FindEmptySpace({width: 1440, height: 900, nodeId: "QpEvu", direction: "right", padding: 100})
a = Copy("QpEvu", document, {
  name: "Screen/Server Detail — Settings · Access (Read-only)",
  x: pos.x, y: pos.y,
  descendants: {
    "aKIDz": {content: "admin"},
    "azK6C": {enabled: false}
  }
})
card = Get(a, n => n.name === "Collaborators Card" ? n.id : undefined)[0]
Insert(card, {type: "text", name: "Read-only Note", fill: "$muted", textGrowth: "fixed-width",
  width: "fill_container", content: "Only the server owner or an admin can change collaborators.",
  fontFamily: "$typography/font-sans", fontSize: 14, fontWeight: "normal"})
Print(a, card)
```

| Source id | Node | Change |
|---|---|---|
| `QpEvu` | screen | Copy to document root. Name it `Screen/Server Detail — Settings · Access (Read-only)`. Place it with FindEmptySpace right of `QpEvu`. It must not overlap the State 2 frame, which is why State 3 runs second. |
| `aKIDz` | owner value | `admin` (someone other than the viewer; matches the Transfer dialog's "Current owner: admin."). |
| `Slq9d` | "None yet" | No change. React shows collaborators as chips; for non-managers the chips just lose their remove "x". The design has no chip row to adapt, so the empty list stands in for it. |
| `azK6C` | add row | `enabled: false` (hidden, as in React). |
| copy of `kRdsz` | Collaborators Card | Append text `Read-only Note`: `Only the server owner or an admin can change collaborators.`, `$muted`, 14px, `$typography/font-sans`, fill width. |

`Kq2KY` and `P4rOBk` stay unchanged. Check with one `get_screenshot` of the copied Settings Panel Wrap.

## After the edits

- Run the `design-export` skill for the three new frame ids (JSON with `Get(id, {depth: 20, includePathGeometry: true})` and PNG with `export_nodes` at 2×). Add them to `design-export/MANIFEST.md` in the same commit as the design change. If `XR0f9` is also fixed, re-export it too.
- Ask the user to save design.pen in the Pencil UI.

## Notes for the React half (not design work)

- A shared helper, e.g. `canOwnerOperate(me, gs)` in `web/src/lib/auth.ts`: owner (`OWNER_ID_ANNOTATION === String(me.id)`), or `*` in `perms["*"]` or `perms[ns]`. `can()` (`auth.ts:34-42`) treats `*` like any permission, so the helper needs its own `*` check. Plain `servers:write` must no longer count.
- `ServerActionsMenu.tsx`: gate Transfer, Wipe and Delete with the helper. Set `hint="Requires server owner or admin"` on all three, including Delete, which has no hint today. Clone stays on `servers:write`.
- `Danger.tsx`: it only receives `name`/`ns`, so it needs the `GameServer` (or its owner id) passed in to gate. When gated: `isDisabled` on the three buttons, plus the notice above the cards (HeroUI `Alert`, lock icon, the copy above).
- `Access.tsx:56`: switch `canManage` to the helper. When gated, render the "Only the server owner or an admin can change collaborators." line where the add row would be.
