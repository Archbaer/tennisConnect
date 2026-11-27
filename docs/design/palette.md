# Colour palette — court green, optic yellow, clay

**Status:** implemented. These values are defined once in `web/src/app.css`:
` :root` for light mode and `html[data-bs-theme='dark']` for dark mode.
`web/src/lib/palette.test.ts` parses those blocks and asserts the WCAG pairs
below, so this table and the shipped tokens stay in step.

## Direction

Tennis-club colour, not neutral dashboard grey. Light mode is a warm off-white
with a green tint; cards are warm white. Dark mode is deep navy layered with
green, never flat charcoal. Deep court green carries structure and primary
navigation. Optic yellow carries primary actions, the active-tab indicator,
chart lines and key stat accents. Clay orange is the secondary highlight and
warning colour. Yellow is never body text on a light surface: on yellow buttons
the label is dark green (`--ball-ink`), at 14.08:1 in light and 13.20:1 in dark.

## Core tokens

| Token | Light | Dark | Use | Contrast |
| --- | --- | --- | --- | --- |
| `--court` | `#0f5132` | `#2e9b6e` | Primary structure, secondary buttons, active nav | n/a (surface fill) |
| `--court-deep` | `#0b3d2a` | `#0b2b32` | Footer, reveal, depth | on-court 11.11 / 13.55 |
| `--court-light` | `#1c7a53` | `#7ee2ac` | Links in dark, accent rules, nav hover | dark link 11.22 |
| `--court-bright` | `#2e9b6e` | `#9df0c4` | Decorative green highlights | n/a |
| `--ball` | `#ccff00` | `#d4f000` | Primary CTA fill, active tab, chart line | ball-ink 14.08 / 13.20 |
| `--ball-deep` | `#b8e600` | `#a8c700` | Yellow CTA hover and border | ball-ink 11.31 / 8.80 |
| `--ball-top` | `#efff9e` | `#efff9e` | Top of the yellow button/ball gradient | with ball-ink >= 11 |
| `--ball-shine` | `#f7ffcf` | `#f7ffcf` | Ball radial highlight | n/a |
| `--ball-ink` | `#0b231a` | `#08201a` | Text and icons on optic yellow | on ball 14.08 / 13.20 |
| `--clay` | `#c2511d` | `#c65a28` | Clay fill, secondary chart series | n/a |
| `--clay-deep` | `#9e3f12` | `#9e3f12` | Clay depth and hover | n/a |
| `--clay-ink` | `#98400f` | `#ffb48a` | Warning text on clay tint | on clay-bg 5.86 / 8.30 |
| `--clay-bg` | `#fbebdd` | `#3b2518` | Warning badge background | see above |
| `--clay-line` | `#efc9ad` | `#5c3a25` | Warning badge border | decorative |
| `--sky` | `#0e7490` | `#7dd3e8` | Focus ring, info accent | on paper 4.94 / 10.37 |
| `--grass` | `#4f8f3a` | `#5f9a44` | Decorative court fill only | n/a |

## Surfaces and text

| Token | Light | Dark | Use | Contrast |
| --- | --- | --- | --- | --- |
| `--paper` | `#f3f7ee` | `#071b24` | Page background | ink 15.24 / 16.00 |
| `--surface` | `#fcfdf9` | `#0e2832` | Cards, sheets, inputs | ink 16.19 / 13.93 |
| `--surface-top` | `#ffffff` | `#133540` | Card gradient top | n/a |
| `--surface-2` | `#f7faf1` | `#0a212b` | Secondary panel | n/a |
| `--surface-hover` | `#eaf2e2` | `#173a3c` | Row and nav hover | n/a |
| `--ink` | `#0b231a` | `#eaf7f0` | Body text, headings | on paper 15.24 / 16.00 |
| `--muted` | `#4c6257` | `#a6c3b8` | Secondary text | on paper 6.06 / 9.33 |
| `--line` | `#d6e0cf` | `#21423f` | Decorative rules and card borders | decorative |
| `--line-strong` | `#bccdb3` | `#335853` | Hover borders | decorative |
| `--control-border` | `#6b7f72` | `#5e8a7c` | Input borders, UI boundary | on paper 3.95 / 3.95 |
| `--link` | `#0f5132` | `#7ee2ac` | Link text | on paper 8.63 / 11.22 |
| `--link-hover` | `#0b3d2a` | `#a5f2c6` | Link hover | >= 8:1 |
| `--chip-bg` | `#e7efe0` | `#16342f` | Neutral badge background | chip-ink 11.67 / 12.11 |
| `--chip-ink` | `#123326` | `#e8f7ee` | Neutral badge text | see above |
| `--chip-line` | `#d0dfc6` | `#2c5049` | Neutral badge border | decorative |
| `--media-bg` | `#e3ecdc` | `#16342f` | Photo and skeleton placeholder | decorative |

## Status

Status stays semantic. Badges always pair text with an icon or word, never
colour alone.

| Token | Light | Dark | Use | Contrast |
| --- | --- | --- | --- | --- |
| `--success` | `#0f6b3f` | `#7fe3ae` | Success text, win badge | on success-bg 5.62 / 8.31 |
| `--success-bg` | `#dff2e5` | `#12382a` | Success badge background | see above |
| `--success-line` | `#b4dcc4` | `#245641` | Success badge border | decorative |
| `--danger` | `#a2261b` | `#ffb4a2` | Error and loss text, outline danger | on danger-bg 6.06 / 8.67 |
| `--danger-bg` | `#fbe3df` | `#3c2020` | Error badge background | see above |
| `--danger-line` | `#efbdb5` | `#5f3230` | Error badge border | decorative |
| `--danger-surface` | `#fcf0ec` | `#2b1717` | Alert panel surface | n/a |
| `--danger-fill` | `#a2261b` | `#a2261b` | Destructive button fill | on-danger 7.02 / 7.02 |
| `--danger-fill-deep` | `#841e15` | `#841e15` | Destructive hover | n/a |
| `--warning` | `#98400f` | `#ffb48a` | Warning text | on warning-bg 5.86 / 8.30 |
| `--warning-bg` | `#fbebdd` | `#3b2518` | Warning background | see above |
| `--warning-line` | `#efc9ad` | `#5c3a25` | Warning border | decorative |
| `--info-surface` | `#e2f2f7` | `#0e2f3b` | Info notice background | info-ink 6.61 / 9.35 |
| `--info-ink` | `#0a5b75` | `#9bdcf2` | Info notice text | see above |
| `--info-line` | `#b8dce8` | `#23505f` | Info notice border | decorative |

## Text and lines on deep fills

| Token | Light | Dark | Use |
| --- | --- | --- | --- |
| `--on-court` | `#eaf7ef` | `#eaf7ef` | Text on court green and the footer |
| `--on-court-muted` | `#cfe6da` | `#c6e2d4` | Secondary text on court green |
| `--on-court-line` | `rgb(255 255 255 / 28%)` | `rgb(255 255 255 / 24%)` | Chalk rules on court green |
| `--on-court-line-strong` | `rgb(255 255 255 / 55%)` | `rgb(255 255 255 / 48%)` | Icon-button border on court green |
| `--on-danger` | `#fff7f4` | `#fff7f4` | Text on destructive fills |

## Charts, data and motion accents

| Token | Light | Dark | Use |
| --- | --- | --- | --- |
| `--chart-fill-start` | `#1c7a53` | `#2e9b6e` | Top of the green-to-transparent area fill |
| `--chart-line-start` | `#ccff00` | `#d4f000` | Yellow line, left stop |
| `--chart-line-end` | `#d4f000` | `#ccff00` | Yellow line, right stop |
| `--chart-line-halo` | `#0f5132` | `#0b2b32` | Darker outline under the yellow line |
| `--chart-line` | `#0f5132` | `#d4f000` | Inactive point stroke |
| `--chart-glow` | `rgb(15 81 50 / 22%)` | `rgb(212 240 0 / 38%)` | Line glow |
| `--chart-grid` | `#dce5d6` | `#1d3a3d` | Grid rules |
| `--chart-secondary` | `#c2511d` | `#f08a4b` | Reserved clay colour for a second series |
| `--tracker-track` | `#dfe8d8` | `#1b3c3b` | Empty progress segments |
| `--tracker-from` | `#1c7a53` | `#2e9b6e` | Filled segments, green stop |
| `--tracker-to` | `#ccff00` | `#d4f000` | Filled segments, yellow stop |
| `--kpi-accent` | green → yellow → clay | green → yellow → clay | KPI top accent rule |
| `--tab-indicator` | green → yellow | green → yellow | Active mobile tab bar |

### Light-mode chart line

The main series is optic yellow, which does not reach 3:1 by itself on a light
surface. The dashboard and landing preview therefore draw a `5.5px` /
`5px` `--chart-line-halo` stroke behind the `2.5px` yellow line. The visible
outline is deep court green, so the chart remains legible without darkening the
yellow brand accent. In dark mode the same halo adds depth against the navy
surface. `--chart-secondary` is clay for a future second series; no second
series is rendered today.

## Bootstrap mapping

`app.css` maps `--bs-primary`, `--bs-primary-rgb`, `--bs-link-color`,
`--bs-link-hover-color`, `--bs-body-color`, `--bs-body-bg`,
`--bs-border-color`, `--bs-border-color-translucent`, `--bs-secondary-color`,
`--bs-secondary-bg`, `--bs-tertiary-bg`, `--bs-emphasis-color`,
`--bs-heading-color`, `--bs-focus-ring-color`, the status emphasis pairs and
the alert subtle pairs to the tokens above. Component styles use those tokens
or the shared `--rgb-*` channels for alpha compositing; no raw hex or `rgb()`
colour literals remain in component styles.

## Accessibility rules

- Body text is at least 4.5:1; large text and UI boundaries at least 3:1, in
  both themes. `palette.test.ts` checks the pairs listed above.
- Optic yellow is never text on a light background. Yellow buttons use
  `--ball-ink`.
- Status colours are not the only signal: badges keep text plus an icon, and
  destructive controls spell out the action.
- Focus rings stay `--sky` blue, with `--focus-halo` for the soft outer ring.
- `--on-court` and `--on-court-muted` keep footer and reveal text legible on
  deep green and navy.
