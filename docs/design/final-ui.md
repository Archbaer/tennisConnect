# Final UI — Court side v2

**Status:** implemented. Supersedes the visual decisions in `court-side-theme.md`
and the layout assumptions in `v1-ux-spec.md`.

## Direction

Mobile-first club tennis. The structure follows the supplied mobile mockup:
a full-screen player card, a persistent bottom tab bar, and a short match list.
The palette follows the supplied court illustration: grass, chalk, clay and a
tennis ball.

No third-party assets, fonts, scripts or analytics. Bootstrap 5.3.9 CSS and
Bootstrap Icons are self-hosted. No Bootstrap JS.

## App shell

- `min-height: 100dvh` flex column.
- `main` grows; the footer sits at the bottom on short pages.
- Mobile, signed in: fixed bottom tab bar for Discover, Matches, You and
  Settings. 64 px plus `env(safe-area-inset-bottom)`. The marketing footer is
  hidden and `main` reserves bottom padding.
- Desktop: sticky top header with nav pills. The bottom tab bar is hidden.
- Signed out: top header shows the sign-in action; the footer stays at the
  bottom of the page.

## Palette

| Token | Value | Use |
| --- | --- | --- |
| `--ink` | `#0A1714` | Body text |
| `--paper` | `#F7F9F4` | Page background |
| `--surface` | `#FFFFFF` | Cards, inputs, sheets |
| `--court-deep` | `#06281F` | Depth, footer, reveal |
| `--court` | `#0B3B2E` | Primary actions, active nav |
| `--court-light` | `#146B4A` | Links, secondary structure |
| `--grass` | `#6FA84A` | Decorative court fill only |
| `--ball` | `#D8FF3E` | Match accent, active marker |
| `--clay` | `#B94E2F` | Destructive fill (5:1 with white) |
| `--clay-deep` | `#8F3B24` | Destructive text |
| `--muted` | `#4F625B` | Secondary text |
| `--line` | `#D7E0D8` | Decorative rules |
| `--sky` | `#2E9CC4` | Focus ring |

Usage: 60% paper/surface, 30% court greens, 10% ball/clay. Ball yellow always
sits with words. Clay is only destructive. Grass is only scenery.

## Radius and spacing

- Cards and sheets: `28px`.
- Inputs and secondary controls: `14px`.
- Chips: `10px`.
- Buttons and nav pills: `999px`.
- One 1px border; no mixed radii or double borders.
- Discover/detail column: `30rem`. Match list: `42rem`. Forms/settings: `36rem`.
- Page gutters: `1.5rem` mobile, `2rem` desktop.

## Discover card

Vertical, Tinder-like card:

1. 4:3 full-bleed photo (max 20rem tall on mobile), with a distance chip and a
   safety overflow button.
2. Name, age and skill.
3. Seven-tick week strip.
4. Plays and distance meta rows.
5. Two-line bio.
6. Pinned `I'd play` / `Not for me` actions.

No horizontal split at any breakpoint. On mobile the card fits above the bottom
tab bar; on desktop it stays in a centered narrow column. A failed photo URL
falls back to a court-green initials tile.

## Area and matching sheets

- Area editing is a focused native dialog opened from Settings and the profile.
  It contains the coarse area form, geolocation fallback and the discovery
  radius. It never routes an existing player through onboarding.
- Matching is a focused dialog for gender preference, age range and skill
  tolerance. It fetches the existing profile, merges the changed fields and
  saves the full profile.
- Onboarding keeps only first-run setup. Area editing after setup stays in the
  sheets.

## Settings

- Centered page header and section headings; centered row actions.
- Sections: card, matching, privacy/safety, data, about and session.
- Long privacy prose stays left-aligned for readability.
- No targetless report action.

## Profile

- A visual player card at the top: full-width photo fallback, name/age, skill,
  area chip and primary actions.
- Preview as another player opens the same card in a native dialog.
- Area changes use the area sheet.
- Photo grid, play times, area and settings remain reachable below.

## Player detail and match reveal

- Discover cards link to `/discover/[id]?radius=...`; the detail endpoint reuses
  the same server-side visibility predicates and returns full bio and play-time
  slots only while the player is visible.
- The detail screen keeps the vertical card, pinned reactions and target-aware
  safety button.
- A new match goes to `/matches/[id]/new`; the reveal is a one-visit
  court-deep/ball-yellow screen with the heading focused and a polite match
  announcement. Instagram stays off this screen.
- Match rows show a ball-yellow edge and “New match” until the viewer has seen
  the reveal. The Matches tab accessible name gains “, new match” from an
  in-memory store; no extra API polling.

## Safety actions

Every report or block action has a target and a visible name.

- Discover card: safety overflow button.
- Player detail: same button.
- Match row: safety button beside the row link.
- Match detail: Report and Block buttons with the player's name.

The native `<dialog>` repeats the target name, traps focus, closes on Escape and
returns focus to the invoking button. Destructive actions put initial focus on
Cancel. The target UUID never appears in a URL. There is no targetless report
form.

## Court background

`web/static/court-hero.svg` is a flat vector court with chalk lines, a clay
racket and a yellow ball. It is used in the landing hero and as a 3.5% opacity
fixed page texture. It is decorative, cached as a local asset, and has no text.

## Motion

CSS only: landing ball bounce, match flood, 280 ms page fade-up, 1–2 px card
lift. All removed under `prefers-reduced-motion`.

## Accessibility

- One `<h1>` per screen; skip link first.
- Bottom tab bar uses text labels and 44 px targets.
- Focus ring `3px` sky; inner ring on photos.
- `scroll-margin` for sticky header, tab bar and pinned actions.
- Axe: zero serious/critical on every route.
- Keyboard, 320 px, 200% zoom, VoiceOver and TalkBack passes remain launch
  gates.
