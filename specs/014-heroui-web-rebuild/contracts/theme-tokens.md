# Contract: theme tokens

Binding for `web/src/styles/globals.css` and for the HeroUI variable values in `design.pen`. Brand values come from today's `globals.css` (R-01); HeroUI names come from the theming page (R-04). Converting HSL to oklch is mechanical; the implementer records the exact oklch strings in `globals.css` and the same values in Pencil.

## Brand → HeroUI token

| HeroUI token (CSS) | Pencil variable | Dark value (today's `.dark`) | Light value (today's `:root`) | Notes |
|---|---|---|---|---|
| `--background` | `$background/background` | `#0F0F0F` (hsl 0 0% 6%) | `#FFFFFF` | page ground |
| `--foreground` | `$foreground/foreground` | `#F5F5F5` | hsl 224 71% 4% | text |
| `--surface` | `$surface/surface` | `#1C1C1C` (today's `card`) | `#FFFFFF` (today's `card`) | cards, panels |
| `--surface-secondary` | `$surface/secondary` | `#171717` (today's `surface`) | hsl 220 14% 96% | sidebar, sunken areas |
| `--overlay` | `$overlay/overlay` | `#1C1C1C` | `#FFFFFF` | modals, popovers |
| `--accent` / `--accent-foreground` | `$accent/accent`, `$accent/foreground` | `#F97316` / `#FFFFFF` | hsl 22 95% 53% / `#FFFFFF` | orange brand primary |
| `--default` / `--default-foreground` | `$default/default` | `#292929` / `#F5F5F5` | hsl 220 13% 91% / fg | secondary buttons, chips |
| `--success` | `$success/success` | hsl 142 71% 45% | same | |
| `--warning` | `$warning/warning` | hsl 38 92% 50% | same | |
| `--danger` | `$danger/danger` | hsl 0 72% 51% | same | |
| `--muted` | `$muted/muted` | `#949494` | hsl 220 9% 46% | secondary text |
| `--border`, `--separator` | `$border/border` | `#292929` | hsl 220 13% 91% | |
| `--field-background`, `--field-border`, `--field-placeholder` | `$field/*` | `#0F0F0F`, `#292929`, `#949494` | `#FFFFFF`, border, muted | inputs |
| `--focus`, `--link` | `$focus`, `$link` | accent | accent | |
| `--radius` | `$radius/*` | 10 px base (today's `--radius-lg`), `--field-radius` 6 px (today's `--radius-md`) | same | HeroUI defaults use pill buttons (`$radius/3xl`); the design pass keeps HeroUI's radii per component and the code does not override them |
| fonts | `$typography/font-sans`, `font-mono` | Geist / JetBrains Mono | same | loaded as today from `index.html` |

`-hover`, `-soft` and `-soft-foreground` variants are left to HeroUI's derived defaults unless the design pass shows a mismatch, in which case the override is added to this table first.

## Gameplane extra token

`--color-violet` (hsl 258 90% 66%) stays in the `@theme` block permanently for the operator-role chips (`vStkb`, `c:rjvI1` today). It is not a HeroUI token.

## Legacy alias policy (transition only)

Slice 0 rewrites `globals.css` as:

1. `@import "@heroui/styles";` (replaces `@import "tailwindcss";`, which the HeroUI stylesheet already includes).
2. `:root { … }` and `.dark { … }` blocks setting the HeroUI tokens above.
3. Legacy HSL triplets renamed `--gp-bg --gp-surface --gp-card --gp-border --gp-muted --gp-fg --gp-primary --gp-primary-fg` so they no longer collide with HeroUI's `--surface --border --muted`.
4. `@theme` aliases for old screens, each defined from a HeroUI token so colours stay identical on every un-rebuilt screen: `--color-background: var(--background)`, `--color-surface: var(--surface-secondary)`, `--color-card: var(--surface)`, `--color-border: var(--border)`, `--color-muted: var(--muted)`, `--color-fg: var(--foreground)`, `--color-primary: var(--accent)`, `--color-primary-fg: var(--accent-foreground)`, `--color-success/-warning/-danger` likewise.

Rebuilt files use HeroUI's semantic utilities (`bg-surface`, `text-foreground`, `text-muted`, `bg-accent`) and never the legacy aliases. Slice 5 deletes steps 3 and 4 and `web/tailwind.config.ts`.

## Appearance selection

- `<html class="dark">` stays the shipped default (`web/index.html`).
- Slice 1 adds a toggle (placement per OD-2) wired to HeroUI's `useTheme` (`light` / `dark` / `system`, persisted in `localStorage`); it sets both `class` and `data-theme` on `<html>` so HeroUI and any legacy `.dark` selector agree.
- The login and share-link pages honour the stored preference but expose no toggle of their own (FR-005 keeps them minimal).

## Verification

- A Vitest test in slice 0 renders a probe element and asserts the computed values of `--accent`, `--surface`, `--background`, `--foreground` in both `.dark` and `.light` equal the table above.
- The design pass sets the same values on the HeroUI variables in `design.pen` (both semantic modes); the slice 0 export of `LtgNm` is the design-side evidence.
