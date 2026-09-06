# Contract: theme tokens

Binding for `web/src/styles/globals.css` and for the HeroUI variable values in `design.pen`. Brand values are final as of 2026-09-04 (maintainer, in-session); HeroUI names come from the theming page (R-04). The lunaris library variables are read-only in Pencil, so all screens are re-pointed to HeroUI semantic tokens rather than the library being edited. Converting HSL to oklch is mechanical; the implementer records the exact oklch strings in `globals.css` and the same values in Pencil.

## Brand → HeroUI token

| HeroUI token (CSS) | Pencil variable | Dark value (`.dark`) | Light value (`:root`) | Notes |
|---|---|---|---|---|
| `--background` | `$background/background` | `#121114` | `#FFFFFF` | page ground |
| `--foreground` | `$foreground/foreground` | `#F5F3F7` | `#2A0F1E` | text |
| `--surface` | `$surface/surface` | `#1C1A20` | `#FFF7FB` | cards, panels |
| `--surface-secondary` | `$surface/secondary` | `#17151A` | `#F8DDE9` | sidebar, sunken areas |
| `--overlay` | `$overlay/overlay` | `#201E24` | `#FFF7FB` | modals, popovers |
| `--accent` / `--accent-foreground` | `$accent/accent`, `$accent/foreground` | `#FF4FA3` / `#FFFFFF` | `#DB2777` / `#FFFFFF` | brand primary (pink) |
| `--accent-soft` / `--accent-soft-foreground` | `$accent/soft`, `$accent/soft-foreground` | `#331525` / `#FF8AC4` | `#FFFFFF` / `#BE185D` | accent hover/visited |
| `--default` / `--default-foreground` | `$default/default`, `$default/foreground` | `#232128` / `#EDE9F0` | `#F6DCE7` / `#2A0F1E` | secondary buttons, chips |
| `--success` | `$success/success` | hsl 142 71% 45% | same | semantic (unchanged) |
| `--warning` | `$warning/warning` | hsl 38 92% 50% | same | semantic (unchanged) |
| `--danger` | `$danger/danger` | `#E0304F` | `#DC2828` | semantic error |
| `--danger-soft` / `--danger-soft-foreground` | `$danger/soft`, `$danger/soft-foreground` | `#2E1317` / `#F87171` | — | danger hover/visited (dark mode only) |
| `--muted` | `$muted/muted` | `#9E98A6` | `#6E5262` | secondary text |
| `--border`, `--separator` | `$border/border`, `$separator/separator` | `#2C2932`, `#232227` | `#EFC3D6`, `#EFC3D6` | |
| `--field-background` | `$field/background` | `#17151A` | `#FFFFFF` | input backgrounds |
| `--field-border` | `$field/border` | `#2C2932` | `#EFC3D6` | input borders |
| `--field-foreground` | `$field/foreground` | `#FCFCFC` | `#2A0F1E` | input text |
| `--field-placeholder` | `$field/placeholder` | `#7B7584` | `#8A6C7B` | input placeholder text |
| `--focus` | `$focus` | `#A78BFA` | `#7C3AED` | focus ring (purple) |
| `--link` | `$foreground/link` | `#7DB4FF` | `#2563EB` | hyperlink text |
| `--surface-tertiary` | `$surface/tertiary` | `#141317` | `#F8DDE9` | deepest surface variant |
| `--segment` | `$segment/segment` | `#1D1B22` | `#FFFFFF` | segmented control backgrounds |
| `--segment-foreground` | `$segment/foreground` | `#F5F3F7` | `#2A0F1E` | segmented control text |
| `--radius` | `$radius/*` | 10 px base (HeroUI default), 6 px field (component-level) | same | design pass keeps HeroUI's radii per component |
| fonts | `$typography/font-sans`, `font-mono` | Geist / JetBrains Mono | same | Pencil variables typography/font-sans = Geist and typography/font-mono = JetBrains Mono set on 2026-09-05; loaded from `index.html` |

Soft variants (`-soft`, `-soft-foreground`) are now explicitly set above; all other HeroUI-derived variants use HeroUI's defaults unless a design mismatch requires an override.

## History

The original palette (R-01, orange accent `#F97316`) was superseded on 2026-09-04. The maintainer reviewed the slice-0 HeroUI atoms and approved a new brand theme: pink accent (`#FF4FA3` dark, `#DB2777` light), purple and blue secondaries (focus `#7C3AED` / `#A78BFA`, link `#2563EB` / `#7DB4FF`). Because the lunaris library variables are read-only in Pencil, all screens in `design.pen` were re-pointed to HeroUI semantic tokens (via the `$`-prefix notation) rather than the library values being edited. Five light-mode preview copies were added so both modes are visible side by side in the design canvas.

## Gameplane extra token

`--color-violet` (hsl 258 90% 66%) stays in the `@theme` block permanently for the operator-role chips (`vStkb`, `c:rjvI1` today). It is not a HeroUI token.

Stat-card icon color is a hardcoded design value: purple `#8B5CF6` (pending a formal token definition — open question).

## Legacy alias policy (transition only)

Slice 0 rewrites `globals.css` as:

1. `@import "@heroui/styles";` (replaces `@import "tailwindcss";`, which the HeroUI stylesheet already includes).
2. `:root { … }` and `.dark { … }` blocks setting the HeroUI tokens above.
3. Legacy HSL triplets renamed `--gp-bg --gp-surface --gp-card --gp-border --gp-muted --gp-fg --gp-primary --gp-primary-fg` so they no longer collide with HeroUI's `--surface --border --muted`.
4. `@theme` aliases for old screens, each defined from a HeroUI token so colours stay identical on every un-rebuilt screen: `--color-background: var(--background)`, `--color-surface: var(--surface-secondary)`, `--color-card: var(--surface)`, `--color-border: var(--border)`, `--color-muted: var(--muted)`, `--color-fg: var(--foreground)`, `--color-primary: var(--accent)`, `--color-primary-fg: var(--accent-foreground)`, `--color-success/-warning/-danger` likewise.

Rebuilt files use HeroUI's semantic utilities (`bg-surface`, `text-foreground`, `text-muted`, `bg-accent`) and never the legacy aliases. Slice 5 deletes steps 3 and 4 and `web/tailwind.config.ts`.

## OKLCH values

All hex colors in the table above have been converted to OKLCH format (sRGB → linear RGB → OKLab → LCH polar coordinates) and recorded in `web/src/styles/globals.css`. Format: `oklch(L% C H)` with L as percent (2 decimals), C as chroma (4 decimals), H as hue degrees (2 decimals). The conversion is mechanical and deterministic; see `globals.css` for the authoritative recorded values.

## Appearance selection

- `<html class="dark">` stays the shipped default (`web/index.html`).
- Slice 1 adds a toggle (placement per OD-2) wired to HeroUI's `useTheme` (`light` / `dark` / `system`, persisted in `localStorage`); it sets both `class` and `data-theme` on `<html>` so HeroUI and any legacy `.dark` selector agree.
- The login and share-link pages honour the stored preference but expose no toggle of their own (FR-005 keeps them minimal).

## Verification

- A Vitest test in slice 0 renders a probe element and asserts the computed values of `--accent`, `--surface`, `--background`, `--foreground` in both `.dark` and `.light` equal the table above.
- The design pass sets the same values on the HeroUI variables in `design.pen` (both semantic modes); the slice 0 export of `LtgNm` is the design-side evidence.
- Five light-mode preview frames (gX7um Screen/Login (Light), oyoTs Screen/Dashboard Home (Light), zFiOW Screen/Servers (Light), sSISK Screen/Server Detail — Overview (Light), DWztv Screen/Mobile — Servers (Light)) show both light and dark modes side by side in `design.pen`, confirming the colour values across the brand refresh. Previews refreshed 2026-09-05 (final).
