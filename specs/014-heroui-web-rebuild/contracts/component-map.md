# Contract: component mapping

Binding for both the Pencil redraw and the React translation. Left column is what exists today; right column is what replaces it. "Composition" means a `Gameplane/…` definition redrawn from HeroUI definitions and implemented in `web/src/components/hero/`.

## Lunaris primitive → HeroUI (design) / `@heroui/react` (code)

| Lunaris `c:` family | HeroUI definition in `LtgNm` | React component |
|---|---|---|
| Button/Default, Large/Default | Button/Primary/{MD,LG} | `Button variant="primary"` |
| Button/Secondary, Large/Secondary | Button/Secondary/* | `Button variant="secondary"` |
| Button/Outline, Large/Outline | Button/Outline/* | `Button variant="outline"` |
| Button/Ghost, Large/Ghost | Button/Ghost/* | `Button variant="ghost"` |
| Button/Destructive, Large/Destructive | Button/Danger/* (soft: Button/Danger Soft/*) | `Button variant="danger"` |
| Icon Button/* | Button/*/Icon/* | `Button isIconOnly` |
| Input/Default, Input/Filled, Input Group/* | Input/Primary, Input/Secondary | `TextField` + `Input` (`Label`, `Description`, `FieldError`) |
| Search Box/* | SearchField/Primary | `SearchField` |
| Textarea/*, Textarea Group | Textarea/Primary | `TextField` + `Textarea` |
| Select Group/* | Select/Primary | `Select` + `Select.Item` |
| Input OTP Group/* | InputOTP/Primary | `InputOTP` |
| Checkbox/*, Checkbox Description/* | Checkbox/Checked, Unchecked, Indeterminate | `Checkbox` |
| Radio/*, Radio Description/* | Radio/Selected, Unselected | `RadioGroup` + `Radio` |
| Switch/*, Switch/Checked | Switch/On, Off, SM, MD, LG | `Switch` |
| Card, Card Plain, Card Image, Card Action | Card/Default (+ Card Header/Content/Footer) | `Card`, `Card.Header`, `Card.Content`, `Card.Footer` |
| Alert/Error, Success, Warning, Info | Alert/Danger, Success, Warning, Accent | `Alert status=…` |
| Label/Success, Orange, Violet, Secondary; Icon Label/* | Chip/Soft/{Success,Warning,…}/SM; violet → Chip with the Gameplane `violet` token | `Chip` (composition `PhaseChip`, `RoleChip`) |
| Tabs, Tab Item/Active, Inactive | Tabs, Tab/Active, Tab/Inactive | `Tabs`, `Tabs.Tab`, `Tabs.Panel` |
| Dropdown, List Item/*, List Item Title, List Divider | Dropdown, Menu Item/*, Menu Title, Menu Divider | `Dropdown`, `Menu`, `Menu.Item`, `Separator` |
| Dialog, Modal/Center, Modal/Center Icon | Modal + Modal Header/Body/Footer, AlertDialog/Danger, AlertDialog/Accent | `Modal`, `AlertDialog` |
| Modal/Left | composition `Gameplane/Drawer` from Modal parts, left-anchored | `Drawer` |
| Table, Data Table, Table Row/Cell/Column Header, Data Table Header/Footer | Table + Table Column/Cell/Row/Header/Body/Footer | `Table` compound (sorting via column `allowsSorting`) |
| Pagination, Pagination Item/* | Pagination/* | `Pagination` |
| Progress | composition `Gameplane/Progress` (track + fill from frames) | `ProgressBar` |
| Breadcrumb Item/* | composition `Gameplane/Breadcrumbs` from Link + text + chevron icon | `Breadcrumbs` |
| Sidebar, Sidebar Item/Active, Default, Section Title | composition `Gameplane/App Sidebar` from Link, Menu Item/Active, Menu Item/Default, Menu Title, Separator, Avatar | `hero/Sidebar` on `Link`, `ListBox`, `Separator`, `Avatar`; mobile in `Drawer` |
| Accordion/* | Accordion/Open, Closed | `Accordion` |
| Avatar/Text, Image | Avatar/Text, Image, Small, Large | `Avatar` |
| Tooltip | Tooltip | `Tooltip` |

Rule: a lunaris primitive with no row above is a defect in this contract, not a licence to keep it. Add the row.

## Radix / hand-rolled → `@heroui/react`

| Today (`web/src/components/ui/` or Radix) | Replacement |
|---|---|
| `button.tsx` (cva variants default/secondary/ghost/danger/outline; sizes) | `Button` |
| `card.tsx` | `Card` compound |
| `input.tsx`, `password-input.tsx` | `TextField`/`Input`; password visibility toggle as an `InputGroup` suffix button |
| `select.tsx` (native select) | `Select` |
| `textarea.tsx` | `Textarea` |
| `switch.tsx` | `Switch` |
| `slider.tsx`, `resource-input.tsx` | `Slider`, `NumberField`; `ResourceInput` becomes a composition |
| `tabs.tsx` (`TabBar`) | `Tabs` |
| `badge.tsx` (`Badge`, `PhaseBadge`) | `Chip`; composition `PhaseChip` keeps the phase→colour map |
| `dropdown-menu.tsx` + `@radix-ui/react-dropdown-menu` | `Dropdown` + `Menu` |
| `confirm-dialog.tsx` + `@radix-ui/react-dialog` | `AlertDialog` (danger for destructive, accent otherwise) |
| Radix `Dialog` in feature dialogs and `BackupDetailDrawer` | `Modal`; drawer → `Drawer` |
| `field.tsx` (`FieldLabel`) | `Label` / `Description` / `FieldError` inside `TextField` |
| `stat.tsx` | composition `StatCard` on `Card` |
| `meter.tsx`, `sparkline.tsx`, `game-icon.tsx`, `slack-icon.tsx` | compositions kept as-is in `hero/` (no HeroUI counterpart; pure SVG/markup) |
| `@radix-ui/react-slot` (asChild) | not needed; HeroUI `Button` renders `<a>` via `Link`-style props |
| `@radix-ui/react-label`, `react-tabs`, `react-toast` (unused) | removed |
| absolutely-positioned Notifications panel and GlobalSearch results | `Popover` |
| `class-variance-authority`, `tailwind-merge`, `clsx` | cva removed; `clsx`/`tailwind-merge` may stay for the compositions |

## Gameplane compositions (57) — node ids and implementation file

Slice 0 atoms: `tpKRk rNhll LMIom XoX7L z9ShNE d5N3W3 J09iP IU7OG` (buttons → no wrapper, use `Button` directly; the definitions are redrawn as instances of HeroUI buttons), `D0cDM qvQPg Lmaf1 AT7ya hl7R3 rh2QH` (form atoms), `k38Uta` Card, `ZWcwn` StatCard, `xCDF7` PageHeader, `x3beP` Modal, `WwNlX` ConfirmDialog, `BPEpm` DropdownMenu, `FyV6E` FilterPopover, `w4ntSc` LoadingCard, `zzx8f` ErrorCard, `igj2U` ErrorBanner.

Slice 1: `kKFX9` App Sidebar, `gu5WY` Top Bar, `aI9PL` Cluster Selector, `hboVw` Notifications Panel, `IdaU7` Search Results.

Slice 2a: `S4k0x` Server Detail Header, `I9kvlZ` Server Detail Tabs, dialogs `T1LzpU` Clone, `rdlrx` Transfer, `t9irnv` Wipe World, `I9W8z` New Folder, `JLaGB` New File.

Slice 2b: `f0s9zG` Capture Warning Banner, `KrREo` Install Module, `BX0XM` Upload Module.

Slice 3: `zhLZN` Backup Detail Drawer, `E9EEv0` Restore Backup, `DMnEi` Add Module Source.

Slice 4: `kIxaJ` Audit Integrity Banner, `CqaSq` Role Editor Modal, `NLDDv` Invite User, `t3IY3u` Edit User, `MaoHP` Reset Password, `Kp48V` Confirm Admin Mapping, `uw0dB XL5ZU vStkb` Removable Group Chips, `R65Xyx Rwnu3 BV5ei` Provenance Badges.

Slice 5: `atqRh` Create Share Link, `VM7ro` Share Link Created, `S7SCDc` Revoke Share Link.

`x7MJI` (Connection Card tunnel-states reference) is a reference frame, redrawn with slice 2a's Overview and still not exported.

## Import rule (FR-012, mechanically checked in review)

A file under `web/src` is `hero` when it imports from `@heroui/react` or `@/components/hero/`, `old` when it imports from `@/components/ui/` or `@radix-ui/*`. No file may be both. A route file and the tab/section files it renders must all be the same family before the slice's PR is opened. Slice 5 ends with zero `old` files.
