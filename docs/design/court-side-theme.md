# Court side theme

Active visual direction for the SvelteKit client. The implemented layout,
palette, radius and app shell are frozen in `final-ui.md`. This file keeps the
original direction; `final-ui.md` wins where they differ.

## Direction

Vibrant club tennis, not a generic dashboard. Court green, ball yellow, clay
accents, chalk geometry. Security stays louder than decoration: no private data
in the browser, no third-party assets, no client-side authorization.

## Stack

- SvelteKit 3 + Svelte 5 + TypeScript.
- Bootstrap 5.3.9 CSS, self-hosted.
- Bootstrap Icons, self-hosted.
- No Bootstrap JS, no jQuery, no CDN, no analytics.
- Svelte `dialog` or inline state for modals.

## Tokens

Current shipped values, including dark mode, are in
[`palette.md`](./palette.md). The original light values are kept below for
history.

| Token | Value | Use |
| --- | --- | --- |
| `--court` | `#0b3b2e` | Primary actions, nav |
| `--court-deep` | `#06281f` | Match reveal, footer |
| `--court-light` | `#146b4a` | Notices, links |
| `--ball` | `#d8ff3e` | Match accent, active markers |
| `--ball-deep` | `#b8df22` | Ball shading |
| `--clay` | `#b94e2f` | Destructive fill |
| `--clay-deep` | `#8f3b24` | Destructive hover/text |
| `--sky` | `#2e9cc4` | Focus ring |
| `--paper` | `#f7f9f4` | Page background |
| `--ink` | `#0a1714` | Body text |
| `--line` | `#d7e0d8` | Chalk rules |

Rules:

- Ball yellow never carries meaning alone; always paired with words.
- Destructive actions always use text plus clay/red.
- Body text keeps 4.5:1; control boundaries keep 3:1.
- Faint court grid sits behind content at low opacity.

## Motion

- Landing ball bounce, 2.8 s, transform only.
- Match reveal flood and ball drop, under 500 ms, once.
- Page fade-up, 280 ms.
- Card lift on hover, 1–2 px.
- `prefers-reduced-motion` removes all of it.

## Screen treatment

- Landing: court perspective, bouncing ball, scoreboard facts.
- Discover: one scorecard, 4:5 photo, week strip, ball-yellow primary action.
- Match reveal: court-deep surface, ball-yellow flood.
- Profile/photos: club registration card, 3-up photo grid, status banner.
- Settings/account: icon rows, clear destructive separation.
- Empty/offline/error: ball mark, one sentence, one clear action.

## Bootstrap usage

- Grid, forms, buttons, alerts, badges, progress, utilities.
- Custom classes layer over Bootstrap, not replace it.
- `app.css` loads after Bootstrap and owns tokens and component overrides.
- Desktop uses Bootstrap nav pills; mobile uses a fixed bottom tab bar.
- The footer is pushed to the bottom by the app shell.
- No Bootstrap JS components.

## Implementation

See `final-ui.md` for the implemented card layout, palette, radius scale,
safety dialogs and court background.

## Security constraints

- No profile, match, photo, report or token data in `localStorage`/`sessionStorage`.
- No `{@html}` for user content.
- No external fonts, icons, scripts or images.
- Same-origin cookies only.
- Server sends only fields a visible frame needs.
- Presigned S3 URLs remain five-minute bearer credentials; bucket host is visible but not credentials.
- CSP still open; see `docs/security.md`.
