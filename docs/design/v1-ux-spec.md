# Tennis Connect — V1 UX specification

**Task:** plan.md Task 4 (design track). **Status:** draft for user review before Task 8.
**Scope:** mobile web, 390 × 844 baseline (safe at 320 px reflow), UK English, adults only.
**This is a design artifact.** It contains no code, no routes to build in this task, and no HTML.
**Consumes:** auth + profile + photos contracts already implemented (Tasks 1–2, README).
**Assumes:** Task 3 contract as planned, and Tasks 5, 6, 7 contracts listed in §8. Assumptions are flagged, not invented as fact.
**Repo note:** `.gitignore` ignores `*.md` except `README.md` and `docs/design/*.md`, so this file is trackable while other working notes stay local.

---

## 0. Decisions to confirm before Task 8

| # | Decision | Why it matters |
|---|---|---|
| D1 | **No map, ever, in V1 discovery.** Distance is a number (`Under 1 mile`, `3 miles`), radius is a slider. | A map implies precision we do not have and must not expose. Costs nothing to drop. |
| D2 | **"Not for me" is permanent and confirmed inline.** The button says what it does; V1 has no undo endpoint. | Task 6 reactions exclude permanently; calling it "Skip" would be a lie the user only learns later. |
| D3 | **Optical yellow appears only when a match exists.** Every other state is ink/paper/court-blue. | The one loud colour in the product means "mutual". Nothing else can borrow it. |
| D4 | **Email address is never echoed in the UI.** "Check your inbox", not "We sent a link to alex@…". | Hard rule: no email in any frame; also shoulder-surfing and screen-share safety. |
| D5 | **Instagram stays behind a "Show Instagram" action inside a match**, not in the match payload or list. | Leaks a private handle into list caches, screenshots and logs otherwise. Adds one Task 6 dependency. |
| D6 | **Thumbnails are real for approved and pending photos** (Task 3 owner presigned GET); rejected photos show a status tile because the URL 404s. States are rediscovered on load from `GET /api/v1/photos`. | Removes the "no thumbnail after reload" gap without ever re-fetching a rejected object. |

D5 remains an API ask. D6 is now settled by the Task 3 contract; if that changes, the status-tile fallback still covers every state.

---

## 1. Principles and visual direction

### 1.1 Product character → design consequence

Tennis Connect is a stranger-meeting product for adults, with photos that are moderated before anyone sees them, a date-of-birth gate, and one piece of contact information released only after two people both say yes. The character is **clinical about safety, warm about tennis**. That rules out most of the visual vocabulary of a dating app and most of the vocabulary of a startup landing page.

| Trait | Design consequence | Deliberately avoided |
|---|---|---|
| Safety is a claim the interface has to keep | State is always written in words. Nothing important is communicated by colour alone. Every async state has a sentence and a next action. | Colour-coded dots, green "success" ticks as the only signal, spinner-only feedback. |
| Two adults must both consent | The mutual moment is the only loud moment in the product. Before it, the interface is quiet. | Confetti, streaks, "someone liked you" teasers, urgency timers, red badges. |
| Real courts, real evenings, a real city | Time and distance are set in tabular numerals like a scorecard or a club ladder sheet. Weekday/time is treated as the headline fact about a player. | Full-bleed glossy hero photography, gradients, glass panels. |
| Adults, UK, plain dealing | Sentence case, second person, no exclamation marks, no "Oops". Errors state what happened and what to do. | Mascots, jokes in error states, "Yay!", emoji as icons. |
| Photos are the risk surface | The photo area is framed like a document: a thin rule, an explicit status chip, and copy that says who can see it. | Blurred teasers, "premium to see", hover-to-reveal. |

**Visual direction name: *Match ink*.** Paper and ink base, court blue for anything actionable, chalk rules for structure, and one reserve ink — optical yellow — that is only ever mixed when a mutual match exists.

Why this is not a templated default: the palette is derived from the thing itself (painted hard courts, chalk lines, the ball), not from a genre. The typography is a single variable grotesque used at three widths so that the scorecard voice and the body voice are the same face. The one aesthetic risk — a fluorescent yellow that fails every text-contrast test — is confined to a single non-text flood fill on one screen, which is exactly what safety-critical restraint allows.

### 1.2 Colour tokens

All values below are measured WCAG 2.2 ratios (calculate again in Task 8; do not eyeball).

| Token | Hex | Use | Measured contrast |
|---|---|---|---|
| `ink` | `#111A17` | Body text, headings, 1 px borders | 17.7:1 on white, 16.5:1 on paper |
| `paper` | `#F6F7F5` | Page background | — |
| `surface` | `#FFFFFF` | Cards, sheets, inputs | — |
| `court` | `#0C4A5E` | Primary buttons, links, focus ring, active tab | 9.7:1 on white, 9.1:1 on paper |
| `court-deep` | `#08323F` | Pressed/hover primary, match-reveal background | 13.7:1 white text |
| `optic` | `#DCF23F` | **Only** match flood fill: reveal background, match badge, match tab dot | 14.2:1 with `ink` text; 1.25:1 vs white → **fill only, never text, never an icon stroke on paper** |
| `alert` | `#A62B1F` | Rejected, blocked, delete, destructive text | 7.0:1 on white |
| `muted` | `#55645E` | Secondary text, metadata | 6.2:1 on white, 5.8:1 on paper |
| `rule` | `#D5DCD8` | Decorative chalk rules and dividers only (never a control boundary) | 1.4:1 — non-text, decorative |
| `control-border` | `#6B7A74` | Input and secondary-button boundaries | 4.5:1 |

Rules: `optic` never carries meaning alone (it always sits with the words "It's a match" or "Matched"); `alert` never carries meaning alone (it always sits with an icon and a sentence); `rule` never outlines an interactive control.

### 1.3 Type

One variable family, **Archivo** (self-hosted WOFF2, subset Latin, `font-display: swap`, preloaded). Three roles, chosen so the club-scorecard voice and the body voice are the same face:

| Role | Spec | Use |
|---|---|---|
| Display | Archivo 700, width 92, tracking −0.01em, 32/36 mobile | Screen titles, match headline |
| Body | Archivo 400/500, width 100, 17/25 | All prose, labels, buttons |
| Score | Archivo 500, width 88, uppercase, 12/16, tracking +0.06em, `tabular-nums` | Metadata rows: `3 MILES`, `TUESDAY EVENINGS`, `2 OF 3 PHOTOS`, timers |

No third family. Numerals are always `tabular-nums` where they compare or count (distance, ages, photo counts, expiry times). Minimum rendered text size is 12 px and 12 px is reserved for the uppercase score role; body copy is never below 15 px.

Type scale: 32/36, 26/30, 20/26, 17/25, 15/22, 12/16. Headings use sentence case (UK convention, and it keeps a plain-dealing register — this is a deliberate departure from US Title Case).

### 1.4 Layout, spacing, surface

- 390 px baseline. Page padding 20 px, gutters 20 px, content column max 62ch. Reflows cleanly at 320 px with no horizontal scroll.
- 8 px rhythm: 4, 8, 12, 16, 20, 24, 32, 40, 48. Section gap 32 px, block gap 16 px, label-to-field 6 px.
- Radius: 8 px inputs/secondary buttons, 12 px cards and photos, 24 px top of bottom sheets, 4 px status chips. Nothing is a pill except an avatar.
- Elevation: exactly one shadow, on bottom sheets and the tab bar. Everything else is a 1 px `rule`/`control-border` edge on `surface`. Cards read as sheets of paper, not as floating tiles.
- Grid: single column everywhere. The only two-column moments are the metadata row (`label` left, `score` right, rule between) and the two reaction buttons.
- Safe areas: bottom bar height 64 px + `env(safe-area-inset-bottom)`; sheets respect the inset; no content under the notch.
- Icon set: 20 px stroke icons (Lucide or equivalent), 1.75 px stroke, always paired with a text label or `aria-label`. No emoji as icons.

### 1.5 Global navigation

Three tabs: **Discover**, **Matches**, **You**. A three-tab bar is the smallest structure that covers V1 and keeps Settings inside `You` rather than in a fourth slot.

- 64 px bar, 1 px top rule, `surface` background, `aria-label="Main"`, `aria-current="page"` on the active tab.
- Active state = `ink` label + 2 px `court` underline + filled icon. Not colour-only.
- Tab targets are 44 × 44 minimum, full-height hit area, `touch-action: manipulation`.
- Sub-pages get a 56 px top bar: back arrow (44 px, `aria-label="Back"`), title, optional single text action.
- Skip link ("Skip to content") is first in the tab order on every screen and visible on focus.
- Matches tab carries a dot (optic) when an unrevealed match exists. The dot is accompanied by the tab's accessible name gaining ", new match" — a dot alone is not announced.

### 1.6 Motion

- Durations 120 ms (state change), 180 ms (sheet, reveal). Easing `cubic-bezier(0.2, 0, 0, 1)`. Transform/opacity only.
- Two orchestrated moments only: the match reveal (once, 240 ms) and bottom-sheet entry. Everything else is instant.
- `prefers-reduced-motion: reduce` → both moments render their final frame immediately, no transitions, no shimmer in skeletons.
- No autoplay, no sound, no looping background motion, no parallax, no scroll-jacking.

### 1.7 Signature element: the week strip

The product's real differentiator is not a face, it is a calendar. So the recurring visual device is a **seven-tick week strip**: one 12 × 18 px tick per ISO weekday, 3 px apart, filled `court` on days the player has a slot and outlined `rule` on days they do not, with `M T W T F S S` beneath in the 12 px score role.

- It encodes true data (their availability), not decoration. Two players' strips stacked is the fastest read of "can we actually play".
- It is `aria-hidden="true"`; the adjacent text ("Tue, Thu evenings") is the accessible value, so nothing depends on the graphic.
- It appears in exactly three places: discovery card, player detail, and the match reveal (stacked, with shared days marked by an `optic` tick — inside the match screen, where `optic` is allowed).
- No other screen uses it, so it stays a signature rather than wallpaper.

### 1.8 What we deliberately do not build

No swipe-only deck (visible buttons always). No map. No chat. No read receipts. No "who liked you". No public profile pages, no shareable profile URL, no OG preview of a member. No dark mode in V1 (single carefully tuned light theme beats two mediocre ones); `color-scheme: light` declared explicitly.

---

## 2. Flow map

```
 / (landing)
   └── email input ──► /check-inbox ──(mail)──► /auth/verify?token
                                                    ├── existing ──► /discover
                                                    └── new ──► DOB gate (step 1) ──► /onboarding
                                                                            2 name/gender/level
                                                                            3 play times
                                                                            4 photos
                                                                            5 preferences
                                                                            6 area + radius
                                                                            ──► ready check ──► /discover

 /discover ──card──► pass | ask to play ──► (both said yes) ──► /match/new (reveal) ──► /match/{id}
 /matches ──► /match/{id} (Instagram behind "Show Instagram")
 /you ──► edit profile | play times | photos | area | settings | download data | delete
 /safety/report, /safety/block (sheets over any screen)
```

Proposed routes (Task 8 decides final paths; keep them deep-linkable so refresh and back behave):

| Screen | Proposed route | Deep-linkable state |
|---|---|---|
| Landing | `/` | — |
| Check inbox | `/check-inbox` | — |
| Sign-in link landing | `/auth/verify?token=…` | token in query (never rendered) |
| DOB gate | `/welcome/age` | — |
| Onboarding steps | `/welcome/age`, `/welcome/name`, `/welcome/times`, `/welcome/photos`, `/welcome/preferences`, `/welcome/area` | step in path, resume supported |
| Discover | `/discover?radius=&cursor=` | radius + cursor |
| Match list | `/matches` | — |
| Match detail | `/match/{id}` | id |
| Own profile | `/you` | — |
| Photo manager | `/you/photos` | — |
| Settings | `/you/settings` | — |
| Delete / recovery | `/you/delete` | — |

Back behaviour: hardware/browser back must always undo the last screen, never trap inside a sheet (sheet = `dialog` with history entry) and never re-submit a reaction. Onboarding back preserves entered data and never re-asks the date of birth (WCAG 3.3.7).

---

## 3. Screens

Notation: **P** primary action, **S** secondary, **T** tertiary/quiet. State tables list only states that actually occur; a screen with no empty state (S5, S25) simply has none. Three states apply to every authenticated screen and are not repeated below:

| Cross-cutting state | Behaviour |
|---|---|
| Loading | Fixed-height skeletons matching final layout (no CLS); no spinner over content that already rendered. |
| Offline | 40 px banner under the top bar: "You're offline. Some things won't load until you reconnect." Actions that need the network are disabled with the reason inline. A `polite` region announces "Back online." on recovery. |
| Session expired (401) | Full-screen: "You've been signed out." + **P** Send me a sign-in link. Never a silent redirect that loses typed data; if a form was dirty, keep its values in memory for one retry after sign-in. |

Server error strings are never surfaced. Each screen maps 4xx/5xx/timeout to its own sentence from the tables below.


### S1 · Landing (signed out)

**Purpose:** one job — start sign-in. **Route:** `/`.

```
┌──────────────────────────────┐
│  TENNIS CONNECT              │  score role, 12px, tracking
│                              │
│  Find someone to              │  display, 32/36
│  play this week.              │
│                              │
│  Adults only. Photos are      │  body 17/25, muted
│  checked before anyone        │
│  sees them. Contact stays     │
│  private until you both       │
│  say yes.                     │
│                              │
│  ┌────────────────────────┐  │
│  │ Email address          │  │  label above field
│  └────────────────────────┘  │
│  ┌────────────────────────┐  │
│  │ Send me a sign-in link │  │  P, court, 52px
│  └────────────────────────┘  │
│  We'll email a link. No       │  helper, muted
│  password to remember.        │
│                              │
│  ─────────────────────────   │  chalk rule
│  How we keep you safe        │  S, text button
│  Terms · Privacy             │  T, 44px each
└──────────────────────────────┘
```

- **P** Send me a sign-in link. **S** How we keep you safe. **T** Terms, Privacy.
- The three trust sentences are not decorative: they are the reason an adult hands over a photo. Keep them above the fold at 390 px.

| State | What shows | Action available |
|---|---|---|
| Idle | Field, enabled button | Submit |
| Invalid email (client) | Inline error under field, `aria-invalid="true"`, focus stays | Fix and resubmit |
| Sending | Button label "Sending…" + spinner, button disabled, field readonly | None (no double submit) |
| Sent | Navigate to S2 | — |
| Rate limited (429) | Inline error with the wait time from `Retry-After`, formatted in minutes | Wait, retry |
| Provider/database down (503) | "We couldn't send the link. Try again in a minute." | Retry |
| Offline | Banner + "You're offline. Connect to get a sign-in link." Field disabled | Retry when back online |

**Copy:** "Send me a sign-in link" (not "Continue", not "Submit"). Error: "Enter an email address like name@example.com." Rate limit: "Too many attempts. Try again in 2 minutes." Never echo the address back (D4).

**A11y:** single `<h1>`; label is visible and associated (no placeholder-as-label); `autocomplete="email"`, `inputmode="email"`, `spellcheck="false"`; submit disabled only after the request starts, never before; error announced `role="alert"`; focus moves to the error summary if the error is above the field.

### S2 · Check your inbox

**Purpose:** tell the user what happens next without leaking the address.

- **Heading:** "Check your inbox." **Body:** "We sent a sign-in link. It works once and expires in 15 minutes."
- **P** Open my email app (`mailto:` — optional; if the platform blocks it, omit). **S** Use a different address (returns to S1 with the field focused, value cleared).
- **No address is displayed** (D4). No resend before 60 s; after that "Send a new link" appears with a countdown in `aria-live="polite"`. The server also caps sign-in links at 5 per address per hour, so a 429 is a first-class state with the `Retry-After` wait spelled out — never a button that silently does nothing.
- States: sent (default) · resend-throttled (60 s countdown) · link quota reached (429, per-address 5/hour, shows the wait) · offline (banner, no auto-retry).

### S3 · Sign-in link landing

**Purpose:** convert a click into a session with one honest confirmation tap. Server page exists today (`GET /auth/verify`) and renders a bare form; Task 8 restyles it, contract unchanged.

- **Heading:** "Continue sign-in." **Body:** "This link signs you in on this device."
- **P** Continue. **S** Not me — use a different link (returns to S1).
- **States:** valid · expired/invalid (400) → "This link has expired or was already used. Get a new one." + **P** Get a new link · consumed by an existing session → straight to Discover.
- **Privacy:** the token is a bearer credential. It is never rendered as visible text, never in a heading, never logged, never in a share sheet. `Cache-Control: no-store`, `Referrer-Policy: no-referrer`, CSP as already implemented. Do not fire analytics on this route.

### S4 · Date of birth gate

**Purpose:** meet the adults-only rule with the least data possible, once.

```
┌──────────────────────────────┐
│ ←                            │
│ Step 1 of 6                  │  score role, "1 OF 6"
│                              │
│ When were you born?          │
│                              │
│ You need to be 18 or over.   │
│ We check it once and show    │
│ your age to other players.   │
│ We never show your date of   │
│ birth.                       │
│                              │
│  Day   Month   Year          │  three selects, or one date input
│  ┌───┐ ┌─────┐ ┌────┐        │
│  └───┘ └─────┘ └────┘        │
│                              │
│ ┌──────────────────────────┐ │
│ │ Continue                 │ │  P
│ └──────────────────────────┘ │
└──────────────────────────────┘
```

- **P** Continue. **S** Back arrow (returns to sign-in link page). No skip.
- Native date input is acceptable but must be tested on iOS/Android; three selects are the fallback when the native picker is unreliable. Either way the field has a visible label, `autocomplete="bday"`, and a format hint "Day, month, year" — never a placeholder-only label.
- The stored date is **never displayed again** anywhere, including the profile screen. The profile shows a server-computed integer `age` only (`{"age": 34}` — §8.2), never the date itself.

| State | What shows |
|---|---|
| Idle | Empty selects, Continue enabled |
| Incomplete | Continue enabled; on submit, inline error + focus first empty part |
| Malformed (400) | "Enter a date like 29 October 1990." |
| Under 18 (400) | Navigate to S5 |
| Token invalid/expired (401) | "This link has expired. Get a new one." → S1 |
| Offline | Banner, submit disabled until back online |

**Copy rule:** the sentence "We never show your date of birth." is mandatory on this screen. It is the only reason a reasonable adult types a real DOB.

### S5 · Under-18 stop

**Purpose:** refuse clearly, retain nothing, offer no workaround.

- **Heading:** "You need to be 18 or over." **Body:** "Tennis Connect is for adults only, so we can't create an account. We didn't save anything." **P** Close. No email capture, no "notify me", no retry loop.
- The server already returns 400 for underage registration; the screen is a view of that decision, never a client-side gate.
- **A11y:** focus moves to the `<h1>`; `role="alert"` on the body sentence so the outcome is announced, not just displayed.

### S6 · Onboarding shell

**Purpose:** make a six-step setup feel finite and resumable.

- Progress is a **text** fact, not only a bar: "Step 3 of 6 — When you play". A 2 px court progress rule sits under the top bar; it is `aria-hidden` because the text already carries it.
- **P** Continue (label names the next step: "Add play times", "Add photos"). **S** Back. **T** "Finish later" — appears only after step 3, because steps 1–3 are the minimum for a usable profile. "Finish later" states plainly: "You can finish later. You won't appear in Discover until you have a profile, play times and one approved photo."
- Resume: reopening `/welcome/…` returns to the last incomplete step. Previously entered values are pre-filled (WCAG 3.3.7).
- Exit mid-flow leaves the user signed in but **not discovery-eligible**; `/you` shows a checklist (S18) explaining exactly what is missing.

### S7 · Name, gender, level

**Purpose:** the three facts other players will actually read.

- Fields: Display name (1–50 chars, single line), Gender (`Woman` / `Man` — radio cards, not a dropdown), Skill level (`Beginner` / `Improver` / `Intermediate` / `Advanced` — radio list with a one-line description each), Ranking (optional, 50 chars, "LTA rating or club ranking — optional").
- **P** Continue. **S** Back. No "preview" here.
- Live counter "12 of 50" only appears after 40 characters; it is not a character-count nag.
- **Validation copy (inline, under field):**
  - Contact details in name: "Remove contact details. Names can't include links, handles, email addresses or phone numbers." (mirrors the server's fail-closed rule)
  - Line breaks: rejected silently by the input (single-line).
  - Empty: "Enter a display name." 
  - Invalid character (NUL): "Remove characters we can't display."
- Skill descriptions (mirrors skill semantics, no ladder inflation): Beginner "New to tennis or returning after a long break." · Improver "Rallying, learning to serve, playing socially." · Intermediate "Consistent rallies, plays weekly, happy to compete." · Advanced "Competes regularly; strong serve and match play."
- **A11y:** radio groups use `fieldset` + `legend`; the whole radio card is one 44 px+ target; focus ring on the card, not an inner dot.

### S8 · When you play

**Purpose:** collect 1–20 weekly slots, in `Europe/London`, same day, no overnight. This is the highest-value field in the product — schedule overlap is the reason matches work.

```
┌──────────────────────────────┐
│ ←  Step 3 of 6               │
│ When do you play?            │
│ Add the times you can        │
│ usually play. Others see     │
│ the days and times.          │
│                              │
│ MONDAY            + Add time │
│  ┌ 18:00 – 21:00       ✕ ┐   │
│  └───────────────────────┘   │
│ TUESDAY           + Add time │
│  ┌ 18:00 – 21:00       ✕ ┐   │
│  └───────────────────────┘   │
│ WEDNESDAY         + Add time │
│ THURSDAY          + Add time │
│ …                            │
│ 2 of 20 slots used           │  score role
│ ┌──────────────────────────┐ │
│ │ Continue                 │ │
│ └──────────────────────────┘ │
└──────────────────────────────┘
```

- Days are always all seven, in ISO order, each with its own **Add time** (44 px). Empty days say "No times yet" rather than being omitted.
- Adding a slot opens a sheet: two native time inputs (15-minute steps are acceptable; free entry must still validate), **Add**, **Cancel**. Slot removal is a 44 px ✕ with `aria-label="Remove Monday 18:00 to 21:00"`.
- Validation mirrors the server: end must be after start (same day), no duplicate identical slots, max 20. A slot crossing midnight returns "Times must end on the same day. Add a second slot for after midnight."
- **P** Continue. **S** Back. **T** "Copy from Monday" appears once ≥2 days exist (cheap, high-value; no API change — client-side fill only).
- **Empty/zero-slot state:** Continue disabled with `aria-disabled` **and** an inline reason: "Add at least one time so players can find you." Never a disabled button with no explanation.
- **A11y:** each day is a `fieldset` with `legend` = weekday; each slot is a list item with an accessible name including both times; adding/removing announces via `aria-live="polite"` ("Removed Monday 18:00 to 21:00. 1 slot used."). Two time inputs inside one logical range must be grouped with a shared label.

### S9 · Photos

**Purpose:** get 1–3 photos through automated checking and be honest about what that means.

```
┌──────────────────────────────┐
│ ←  Step 4 of 6               │
│ Add your photos              │
│ Add 1 to 3. One clear photo  │
│ of your face works best.     │
│ Every photo is checked       │
│ before anyone sees it.       │
│                              │
│ ┌────────┐ ┌────────┐ ┌────┐ │
│ │ photo  │ │ + Add  │ │    │ │
│ │ Check- │ │        │ │    │ │
│ │ ing…   │ │        │ │    │ │
│ └────────┘ └────────┘ └────┘ │
│ 1 of 3 added · 0 approved    │  score role
│                              │
│ Photo rules                  │  S, opens rules sheet
│ ┌──────────────────────────┐ │
│ │ Continue                 │ │
│ └──────────────────────────┘ │
└──────────────────────────────┘
```

- Tiles are 3-up, square, 12 px radius, 1 px rule. Each tile is a button opening a tile sheet with only the actions that state allows (§4). No tile offers edit, crop or replace: those do not exist in V1.
- **P** Continue — enabled once `/process` has answered `pending` or `approved` for at least one photo (a row alone is not enough: `upload-url` creates the row before any bytes exist), with a persistent, honest note: "You won't appear in Discover until one photo is approved."
- **S** Photo rules (sheet: face clearly visible, no other people without consent, no contact details anywhere in the image, no screenshots, no minors, no swimwear/underwear-only shots). Stated up front so a rejection is never the first time the user hears them.
- Checking is synchronous but must not flash: hold the "Checking…" state for a **minimum of 1.2 s** even when the response arrives sooner, and never show a spinner for less than 400 ms. A 503 (check service unreachable) is retryable by an explicit **Try again**; nothing auto-retries.
- `200 "pending"` means the check was inconclusive, not that a person is looking at it. Pending copy says exactly that and adds the reassurance that matters: "It isn't visible to anyone." There is no moderation queue and no human-review promise anywhere in the product.
- Upload guidance: JPEG, PNG or WebP, up to 5 MB. Show the accepted list **before** the file picker, not only in the error.
- **File input:** `accept="image/jpeg,image/png,image/webp"`, `capture` not forced (gallery is the common path). Uploads may run in parallel up to the slot limit; each tile owns its own state so one failure never blocks another.
- Full state matrix: **§4**.
- **A11y:** tile status is text inside the tile plus a compact `aria-live` line ("Photo 2 is approved"); never colour-only. The upload progress is a determinate bar with `role="progressbar"`, `aria-valuenow`, and a text percentage; when indeterminate, `aria-valuetext="Uploading"`. Focus returns to the tile after the picker closes.

### S10 · Who you'd like to meet

**Purpose:** preferences that narrow discovery without becoming a wish list.

- Gender preference: `Women` / `Men` / `Everyone` (radio cards).
- Age range: two numeric inputs (min 18, max 120) or a dual-thumb control with **explicit numeric inputs as the accessible alternative** (dragging must never be the only way — WCAG 2.5.7). Show as "25 to 45".
- Skill match: `Any nearby level` (= tolerance 1) / `Exactly my level` (= tolerance 0), each with one line: "You'll see the level above and below yours." / "You'll only see your own level."
- **P** Continue. **S** Back. Copy notes this is editable later: "You can change this any time."
- Validation: min > max → "The youngest age must be lower than the oldest." Focus moves to the min field.

### S11 · Where you play

**Purpose:** get an approximate area, never a fix.

```
┌──────────────────────────────┐
│ ←  Step 6 of 6               │
│ Where do you play?           │
│ We use your area to find     │
│ players nearby. Other        │
│ players only ever see a      │
│ distance like "3 miles".     │
│ Your exact location is       │
│ never shown.                 │
│ ┌──────────────────────────┐ │
│ │ Use my location          │ │  P
│ └──────────────────────────┘ │
│ Set my area instead          │  S
│                              │
│ Within 10 miles              │  slider + numeric
│ ──────●──────────────        │
│ 10 miles          30 miles   │
└──────────────────────────────┘
```

- **P** Use my location (browser geolocation, requested **only** after this button is pressed — never on load, never on an earlier step).
- **S** Set my area instead (postcode or neighbourhood search; see open question Q1).
- Radius: default 10 miles, min 5, max 30, whole miles, 44 px thumb, plus a numeric stepper as the accessible path. Displayed as "Within 10 miles".
- Distances shown to *other* players are computed server-side as `<1 mile` or whole miles rounded up. The client never receives coordinates, never formats a decimal, and never renders a map (D1).
- The area label is a **neighbourhood name** resolved server-side. A postcode may be accepted as input (Q1) but is never displayed afterwards: a neighbourhood label is the finest granularity that may appear on screen.

| State | What shows | Action |
|---|---|---|
| Not yet asked | Explanation + button | Use my location |
| Prompting | Button "Finding your area…" disabled | Wait |
| Granted | "Area set: Battersea · within 10 miles" + **Change** | Continue |
| Denied (permission) | "Location is off." + per-platform steps (iOS: Settings › Privacy › Location; Android: Chrome › Site settings) + **Set my area instead** | Manual area |
| Unavailable (no geolocation API) | Same as denied, without OS steps | Manual area |
| Timeout | "We couldn't get your location. Try again, or set your area." | Retry / manual |
| Out of UK (Task 5 rule) | "Tennis Connect is UK-only for now." | Set a UK area |
| Offline | Banner; both actions disabled | Wait |

### S12 · Ready check

**Purpose:** one honest summary before the first card, so nobody wonders why they see nothing.

- Checklist with ticks in words: "Profile ✓" / "Play times ✓" / "1 photo approved — 1 photo not confirmed" / "Area set ✓". A pending row offers **Check again** inline rather than making the user hunt for it.
- **P** Start discovering — enabled when `GET /api/v1/photos` contains at least one `approved` row. There is no eligibility endpoint (the Task 3 check is internal), so the client derives eligibility from that list and must not invent a second source of truth. If not eligible: the button is replaced by "One photo needs to be approved before you appear" + **Check again**. Never imply a pending photo will be approved on its own, and never promise an email — V1 has no notification.
- **S** Review my profile.

### S13 · Discover

**Purpose:** one card at a time, with a decision the user can take in five seconds and cannot accidentally make irreversible.

```
┌──────────────────────────────┐
│ Discover        Battersea ▾  │  neighbourhood label, never a postcode
│ ┌──────────────────────────┐ │
│ │                          │ │
│ │        PHOTO 4:5         │ │  approved photo, presigned GET
│ │                          │ │
│ └──────────────────────────┘ │
│ Priya, 34                    │  title 20/26
│ ▫▪▫▪▫▫▫  M T W T F S S       │  week strip (aria-hidden)
│ ──────────────────────────── │  chalk rule
│ SKILL      Intermediate      │  score label / body value
│ PLAYS      Tue, Thu evenings │  accessible value for the strip
│ DISTANCE   3 miles           │
│ ──────────────────────────── │
│ Up for a game after work.    │  bio, 2 lines clamp
│                              │
│ ┌───────────┐ ┌────────────┐ │
│ │ Not for me│ │ I'd play   │ │  equal width, 52px
│ └───────────┘ │            │ │  primary = court
│               └────────────┘ │
└──────────────────────────────┘
```

- **P** "I'd play" — expresses interest. **S** "Not for me" — permanently excludes, opens a one-line confirmation: "You won't see each other again." with **Not for me** / **Cancel**. Honest label, no undo path promised (D2).
- **T** Overflow (44 px) → **Report** / **Block**. These are per-card and also available in the detail view.
- Card is one controlled region: it scrolls the page normally (no horizontal swipe required). A swipe gesture may be added in V2 only with the buttons retained.
- Name, age, skill, derived play summary (weekday + part of day, max two), whole-mile distance, bio clamped to two lines with "Read more" expanding in place. No ranking on the card (Q3). No gendered copy ("she plays…") — use name.
- Advanced controls: area label opens a sheet with radius + "Change area" (same content as S11's manual path).

| State | What shows | Action |
|---|---|---|
| First load | Skeleton card, fixed 4:5 ratio, no shimmer under reduced motion | — |
| Loaded | Card | Reactions |
| Empty (none in range) | "No players in range right now." Body explains the three fixes: widen radius, add play times, check area. | Widen radius · Change area |
| Not eligible (no approved photo) | "One photo needs to be approved before you appear in Discover." Pending photos are described as "not confirmed", never "almost there". If a photo was never confirmed, add **Check again**. | Review photos |
| Error | "We couldn't load players." + "Check your connection and try again." | Retry |
| Offline | "You're offline." Last card is **not** shown stale by default; show the offline empty state (safety: stale profiles may already be blocked) | Retry |
| Rate limited | "You're going a bit fast. Take a moment, then try again." | Retry after countdown |
| Submitting reaction | Button shows "Sending…" (reaction buttons only), card stays | — |
| Reaction failed | Inline error above buttons: "That didn't send. Try again." Card stays, no advance | Retry |
| Match created | Go to S15 | — |

**Privacy:** the client receives no `birth_date`, no email, no coordinates, no exact times beyond the published slot summary, no other user's moderation state or report status. Card data is held in memory per page only; no `localStorage`/`sessionStorage` cache of profiles.

### S14 · Player detail (expanded card)

Same content as S13 plus full bio, full play-time list ("Tuesday 18:00–21:00, Thursday 18:00–21:00"), skill description, and the same two actions pinned to the bottom above the nav bar. Reasons: long bios must not truncate the decision, and the detail view is where Report/Block are easiest to find. The pinned action bar must not obscure focused content (WCAG 2.4.11): `scroll-margin-bottom` equal to bar height + 8 px.

### S15 · Mutual match reveal

**Purpose:** the single loud moment, and the point where both people learn that consent is mutual.

- **Route:** `/match/{id}/new` (own route so refresh and deep links work; a sheet is not deep-linkable). One visit only: after first view, the same route renders S17.
- Visual: full-screen `court-deep` → the centre line is drawn, then the surface floods `optic` and the two names land side by side under the headline. 240 ms total, one time. Under reduced motion the final frame is shown instantly.
- **Heading:** "You're both in." **Sub:** "You and Priya both said yes to a game."
- **P** See match. **S** Keep looking (returns to Discover; the match stays in Matches).
- No confetti, no sound, no streak, no "you matched 3 times today".
- **Screen-reader expectation (explicit script for Task 8):** on arrival, focus moves to the `<h1>` ("You're both in"); the match is announced via a `polite` live region with exactly: "It's a match. You and Priya both said yes. Instagram is now visible in this match." No decorative elements are announced; the flood is `aria-hidden`. Do **not** use the whole screen as `role="alert"`. If the match arrives while the user is on another screen, a toast appears in a `polite` region ("New match with Priya") and the Matches tab gains ", new match"; the toast never replaces focus.
- Instagram: **not** shown on this screen. The reveal is about consent, not contact. The handle is one tap away in S17 (D5).
- **Depends on:** the matched-only signed photo URL (§8.2, Task 6). Both photos on this screen — and the thumbs on S16/S17 — come from that contract; without it the reveal shows names and the week strips with no imagery.

### S16 · Match list

**Purpose:** the durable record; every match stays reachable.

- Rows: 56 px photo thumb (matched-only signed URL, §8.2), name, "Matched 3 days ago" (relative, via `Intl.RelativeTimeFormat`), and a state marker in words: "Instagram shared" or "No Instagram".
- **Empty:** "No matches yet." Body: "A match happens when you and another player both say yes. Keep discovering." **P** Discover.
- **Unrevealed match:** row carries a 1 px optic bar at the left plus the words "New match".
- **Loading:** 3 skeleton rows (fixed heights, no layout shift). **Error/offline:** inline retry, existing list is not cleared (a stale list is safer here than an empty one — unlike Discover, a match is already mutual and cannot be un-met by a block the user just made on their own side).

### S17 · Match detail and Instagram switch

**Purpose:** the only place contact can be exchanged, and only because both agreed.

```
┌──────────────────────────────┐
│ ←  Match                     │
│ ┌──────────────────────────┐ │
│ │ photo (signed, §8.2)     │ │
│ └──────────────────────────┘ │
│ Priya, 34                    │
│ Matched 3 days ago           │
│ ──────────────────────────── │
│ YOU BOTH PLAY                │
│ Tue, Thu evenings            │
│ 3 miles apart                │
│ ──────────────────────────── │
│ Instagram                    │
│ Shared because you both      │
│ matched.                     │
│ ┌──────────────────────────┐ │
│ │ Show Instagram           │ │  P
│ └──────────────────────────┘ │
│ ──────────────────────────── │
│ Report                       │  T
│ Block                        │  T, alert colour
└──────────────────────────────┘
```

- **P** "Show Instagram" → fetches the match-only read and replaces itself with the handle (`@priya.plays`), a **Copy** button, and the line "Copied" in a `polite` live region. The handle is fetched on demand and kept in memory for the session only; it is never put in `localStorage`, analytics, page title, or a link preview. An `instagram.com` link opens in a new tab with `rel="noopener noreferrer"` and no referrer.
- If the other player has no Instagram: no button. Copy: "Priya hasn't added Instagram. Tennis Connect doesn't have chat yet, so you can't message here. You can still meet on court at the times above." **This is a real V1 dead end and is flagged in Q2.**
- If the match was ended by a block (either direction): the route returns a terminal state "This match is closed." with no photo, no handle, no history. Never a partial view. Photo URLs are matched-only and stop being issued the moment a block exists, so the terminal state never depends on the client hiding an image.
- **T** Report, Block — same sheets as S22.

**A11y:** the handle is text, not an image; the Copy button has `aria-label="Copy Priya's Instagram handle"`; the live region announces "Copied @priya.plays"; focus stays on the button after copy.

### S18 · Your profile

**Purpose:** show the owner how they appear, plus the one thing blocking discovery if any.

- Top: own photo carousel (3-up grid, 1:1), name, **age** (server-computed `age` field, §8.2 — the profile API does not return `birth_date`), skill.
- **Discovery status banner (only when blocked):** "You're not in Discover yet — one approved photo is needed." + **Review photos**. Pending and rejected photos do not count and the banner never suggests they will. State is rediscovered on load from `GET /api/v1/photos`, so a reload never loses it.
- Sections, each with an **Edit** action: Profile (S19), Play times (S20), Photos (S21), Area (S11 content, in a sheet), Settings (S24).
- **T** "Preview as another player" — opens the same card component with own data, read-only. Cheap, and it is how a user understands what is public. Recommended, not required.
- **Never** displayed here or anywhere: date of birth, email address, user id, session info, moderation provider reference, rejected-photo provider reason.

### S19 · Edit profile

Same fields as S7, plus bio (0–200 chars). Matching preferences are **not** here — they live in Settings → Matching (S24) so this screen only edits what others read.

- Bio field: multiline 4 rows, counter after 160 chars, helper "200 characters. No links, handles, email addresses or phone numbers — we remove profiles that include them."
- **P** Save changes (52 px, pinned above the tab bar). **S** Cancel (with unsaved-changes guard: "Discard your changes?"). Dirty-state guard on back/hardware back.
- Saving: "Saving…", then a `polite` announcement "Profile saved." and focus returns to the top bar title. Server errors map to field errors where possible; otherwise a form-level `role="alert"`: "We couldn't save your profile. Try again."
- Contact-detail rejection copy: "That looks like contact details. Remove links, handles, email addresses and phone numbers, then save again." — mirrors the fail-closed regex, and must not hint at what specifically triggered.

### S20 · Play-time editor

Reuses S8's component in edit mode. Differences: **P** Save changes, and a warning when removing the last slot: "You need at least one time. Removing this one won't save." Save failure keeps the sheet open with the entered data intact (never silently discard typed times).

### S21 · Photo manager

**Purpose:** the owner's view of the moderation pipeline — the highest-anxiety screen in the product.

- 3 slots in a fixed column, each showing one of the §4 states. Slot 1 is labelled "Main photo" (lowest `display_order`).
- Actions per state are listed in §4; the manager never offers an action the API cannot perform. There is no delete endpoint in V1 (Q10), so approved and pending tiles carry no remove control; a rejected tile is replaced by **Upload another photo**, which reuses the freed slot on the next upload.
- States survive a reload: the manager reads `GET /api/v1/photos` on mount and fetches a thumbnail only for approved or pending rows (rejected rows 404 by design).
- **Rejection copy never states a cause.** A rejected photo says it wasn't accepted, shows the photo rules, and offers **Upload another photo** — no provider category, score, threshold, classifier label, or invented reason.
- **No moderation queue:** never say "a moderator is reviewing this", never promise a review time, and never describe an inconclusive photo as "in review". Pending means "we couldn't confirm it", plus "it isn't visible to anyone".
- **Reordering:** only if a reorder contract exists (Q4). Until then no drag handles and no "Set as main" — a control that cannot persist is worse than no control. If it lands, use up/down buttons (not drag-only) with `aria-label="Move photo 2 up"`.
- **Live updates:** while the screen is open, poll `GET /api/v1/photos` every 15 s (no websockets in V1) and announce genuine changes in a `polite` region: "Photo 1 was approved." / "Photo 1 wasn't accepted." Inconclusive rows do not change without a **Check again**, so polling is a safety net, not the primary mechanism.
- **Empty (0 photos):** "No photos yet." + "Add 1 to 3 photos. One approved photo puts you in Discover." **P** Add a photo.

### S22 · Block and report

**Purpose:** make the safest action the easiest action, and never let the other person infer it.

- Entry points: card overflow (S13), player detail (S14), match detail (S17), and the overflow on a match row.
- **Report sheet:** heading "Report Priya". Radio list of categories: "Fake profile or photos", "Inappropriate photos", "Harassment or abuse", "Contact details in the profile", "Looks like someone under 18", "Something else". Optional details field (max 500 chars, helper "Don't include your own contact details."). **P** Submit report. **S** Cancel.
- **Block sheet:** heading "Block Priya?" Body: "You won't see each other in Discover. Any match between you closes and Instagram access is removed. Priya won't be told." **P** Block (alert-coloured, not primary court). **S** Cancel.
- Report implies "hide from you now, review within 72 hours" — state that plainly: "We'll review this within 72 hours. Priya is hidden from your Discover straight away."
- **Blocking a match** additionally closes the match: after confirming, return to the matches list with a `polite` confirmation "Blocked. The match is closed." and no dialog stack.
- **Never** show the other person's report count, moderation state, ban status, or whether they were reported before.
- **A11y:** sheets are `dialog` with `aria-modal`, labelled by the heading, focus trapped, Escape closes, focus returns to the invoking control; destructive confirm gets initial focus on **Cancel**, not on the destructive button.

### S23 · Report received / appeal

- **Report sent:** "Thanks — we've got it." Body: "We'll review within 72 hours. We can't tell you the outcome, but we'll act on the account if needed. Priya is hidden from your Discover." **P** Back to Discover.
- **Appeal (reported user, if banned):** `/account/appeal` — "Your account is suspended." Body explains the category in plain terms, the 14-day appeal window with the exact date (via `Intl.DateTimeFormat`, `Europe/London`), and one textarea. **P** Send appeal. **S** Sign out. One appeal only; a second submission returns a terminal state.
- No details of the reporter, ever, in any state.

### S24 · Settings

Grouped list, 56 px rows, no fiddly toggles in V1:

- **Account**: Email (row reads "Email address — sign-in only", no value), Sign out.
- **Matching**: gender preference, age range, skill match (the S10 fields in a sheet).
- **Area and distance**: radius, change area.
- **Photos**: review rules (static sheet).
- **Your data**: Download my data (S27), Delete account (S25).
- **About**: How we keep you safe, Terms, Privacy.
- No marketing toggles, no notification settings (nothing to notify with in V1), no theme switch.

### S25 · Delete account

**Purpose:** irreversible, slow, and clear about the recovery window.

```
┌──────────────────────────────┐
│ ←  Delete account            │
│ Deleting removes your        │
│ profile, photos, play times, │
│ matches and likes.           │
│                              │
│ You have 30 days to change   │
│ your mind. Sign in with your │
│ email link before 5 December │
│ 2026 and we'll restore your  │
│ account.                     │
│                              │
│ Your profile is hidden from  │
│ Discover straight away.      │
│ ──────────────────────────── │
│ Download my data first       │  S (recommended order)
│ ──────────────────────────── │
│ ┌──────────────────────────┐ │
│ │ Delete my account        │ │  alert, 52px
│ └──────────────────────────┘ │
│ Keep my account              │  T
└──────────────────────────────┘
```

- Two steps: this screen, then a confirm dialog "Delete your account? This can't be undone after 5 December 2026." **P** Delete my account (alert fill). **S** Keep my account (initial focus).
- The exact date is computed server-side (the 30-day boundary) and formatted client-side with `Intl.DateTimeFormat('en-GB', { dateStyle: 'long' })` in `Europe/London`. Never a hardcoded date string.
- After deletion: signed out, returned to S1 with a `polite` line "Your account is scheduled for deletion. Sign in within 30 days to recover it."
- **Export before delete** is offered here because it is the only moment it matters.
- **A11y:** the confirm dialog is labelled, described by the consequences sentence, traps focus, Escape = cancel.

### S26 · Account scheduled for deletion (recovery)

- Signing in within the window shows a blocking recovery screen before anything else: "Your account is scheduled for deletion on 5 December 2026." **P** Recover my account. **S** Continue deleting (signs out).
- Recovery restores the account as it was; if a photo was rejected before deletion it stays rejected (state is not rewound).
- After recovery, land on Discover with a `polite` "Account recovered."
- Never show the countdown as a pressure tactic: one date, one sentence, no timer.

### S27 · Download your data (JSON export)

- Entry: Settings → Download my data, and the delete screen.
- **What it includes (say it up front):** "Your profile, play times, your photo list, your likes and matches, and reports you've made."
- **What it excludes (say it too):** "Other players' private details, including their Instagram handles unless you matched, and anything about reports other people made."
- States: **Preparing…** (determinate if the server gives progress, otherwise indeterminate with `aria-valuetext="Preparing your file"`; disable repeat taps) → **Ready**: "Your file is ready." **P** Download (filename `tennis-connect-data-2026-11-05.json`) with a note "The link expires in 10 minutes." → **Expired**: "That link expired. Prepare a new file." → **Error**: "We couldn't prepare your file. Try again." The 10-minute TTL and one-time link are **assumptions** pending the Task 7 contract (§8.2); if the export is a direct authenticated download instead, drop the ready/expired states and show only Preparing → Error.
- The download is a normal link (`<a download>`), keyboard operable, announced in a `polite` region when ready.

---

## 4. Photo state matrix

Server truth (Task 3, approved): `POST /api/v1/photos/{id}/process` returns `200` with `status: "approved" | "rejected" | "pending"` — `pending` means the provider was inconclusive, not an error. `503` means the check could not complete (provider timeout or transport error, malformed response, or a database failure); the row's status and provider reference are unchanged and the photo stays hidden. `400` means bad bytes. `GET /api/v1/photos` returns `{"photos":[{"photo_id","display_order","status"}]}` — an object, no URLs; `GET /api/v1/photos/{id}/url` returns a 5-minute presigned GET for **approved and pending** photos and `404` for rejected ones (the object is deleted). Client copy is fixed here so Task 8 has one source.

| State | How the client knows | Tile shows | What the user can do | Copy |
|---|---|---|---|---|
| **Empty slot** | fewer than 3 photos | Dashed 1 px `rule` tile, centred plus icon, `aria-label="Add photo"` | Add a photo | "Add photo" |
| **Uploading** | PUT in flight | Determinate progress bar + percentage, preview from `URL.createObjectURL(file)` | Cancel | "Uploading 62%" |
| **Processing** | upload done, `/process` in flight | Same preview, dimmed, "Checking…" | Wait (no cancel) | "Checking your photo…" |
| **Pending (inconclusive)** | `/process` 200 `"pending"`, or a list read returns pending | Thumbnail from the owner presigned GET, dimmed, chip "Not confirmed" | **Check again** (re-POST `/process` while the row is under 1 hour old) | "We couldn't confirm this photo is ready yet. It isn't visible to anyone." |
| **Check unavailable** | `/process` 503 | Preview stays, chip "Check failed" | **Try again** (manual only — never auto-retry) | "We couldn't check this photo just now. Try again." |
| **Approved** | `/process` 200 `"approved"`, or a list read | Photo, chip "Approved"; slot 1 labelled "Main photo" | None in V1 — no delete endpoint (Q10); reorder only if Q4 lands | "Approved. You're in Discover." |
| **Rejected** | `/process` 200 `"rejected"`, or a list read; `/photos/{id}/url` returns 404 | Status tile only — **never request the image** (the object is gone); alert rule icon, chip "Not accepted" | **Upload another photo** (the freed slot is reused on the next upload) | "This photo wasn't accepted. Photos need your face clearly visible, no other people, and no contact details." |
| **Upload failed (network/CORS/expired URL)** | PUT rejected or fetch failed | Preview stays, chip "Upload failed", retry button | Try again (re-requests `upload-url`; a stale slot is reclaimed) | "The upload didn't finish. Check your connection and try again." |
| **Photo too large** | `/process` 400 and the local file is > 5 MB | No preview persists; chip "Too large" | Choose another | "That photo is 12.4 MB. The limit is 5 MB. Choose a smaller one." |
| **Too many pixels** | `/process` 400 and local dimensions exceed 20 MP | chip "Too large" | Choose another | "That photo is 34 megapixels. The limit is 20. Choose a smaller version." |
| **Wrong format** | `/process` 400 and local type not jpeg/png/webp (e.g. HEIC) | chip "Wrong file type" | Choose another | "We can't read that file. Use a JPEG, PNG or WebP photo." + iOS hint: "On iPhone, Settings › Camera › Formats › Most Compatible." |
| **Slot expired (row older than 1 hour)** | `/process` 409 `upload expired` (also applies to a row that stayed pending past an hour), or `upload-url` reclaimed the slot | Preview cleared, chip "Expired" | Try again | "That upload took too long, so we cleared it. Add the photo again." |
| **Storage unavailable** | `/upload-url` 503 | Add-tile disabled with inline reason | Retry | "Photo uploads aren't available right now. Try again in a minute." |
| **Slot limit reached (3 photos)** | `/upload-url` 409 | Add-tile replaced by helper text | — | "You already have 3 photos." |

Never: a fake progress bar that finishes before the PUT does; a rejected photo fetched or shown to anyone (including its owner after the object is deleted); provider labels ("nudity-2.1", scores, thresholds, categories) in any frame; a stated *reason* for a rejection we were not given; a human-review queue or review-time promise; a remove action for a slot the API cannot free.

The server returns the same `400 invalid photo` for oversized, over-pixel, undecodable and missing bytes, so the client distinguishes the first three **locally** (it still holds the `File`) and falls back to the generic sentence when it does not. It must never claim a specific cause it cannot establish.

---

## 5. Accessibility contract (WCAG 2.2 AA)

### 5.1 Global

| Requirement | Implementation rule |
|---|---|
| 2.4.1 skip link | "Skip to content" is the first focusable element on every screen, visible on focus, targets `<main id="content" tabindex="-1">`. |
| 1.3.1 structure | One `<h1>` per screen, no skipped levels, `<main>`/`<nav>`/`<header>` landmarks, every landmark labelled when repeated. |
| 3.1.1 language | `<html lang="en-GB">` on every route (server-rendered, so it is present before hydration); UK spelling in all copy ("organise", "colour"). |
| 2.4.2 page titles | A unique per-route `<title>`, name first then context: "Discover · Tennis Connect", "Priya · Tennis Connect". Never a handle, distance or moderation state in a title. |
| 2.4.4 link purpose | "Read more" carries `aria-label="Read more about Priya"`; match rows and cards are links whose accessible name includes the player's name, so a list of links is never ambiguous out of context. |
| 1.1.1 photo alternatives | Player photos: `alt="{Name}'s photo {n} of {total}"`, own photos "Your photo {n} of {total}". Decorative layers (court rules, week strip) are `aria-hidden`. No screen relies on a photo to convey a name. |
| 2.4.7 / 1.4.11 focus | 2 px `court` ring, 2 px offset, `:focus-visible` only. On photos and `optic` surfaces, a 2 px `ink` inner ring + 2 px `surface` outer ring so the indicator never disappears against the image. |
| 2.4.11 focus not obscured | Sticky top bar (56 px), pinned action bar (72 px) and tab bar (64 px) all get matching `scroll-margin-top/bottom`; no focused element can sit under them. |
| 2.5.8 target size | 44 × 44 minimum for every control, including the tile ✕ and the overflow. Adjacent destructive actions separated by ≥ 8 px. |
| 2.5.7 dragging | Radius control and any future reorder are numeric/buttons first; drag is at most an enhancement. |
| 1.4.3 / 1.4.11 contrast | All text ≥ 4.5:1 (values in §1.2); control boundaries ≥ 3:1; `optic` never text. |
| 1.4.4 resize / 1.4.10 reflow | Usable at 200 % zoom and at 320 px wide with no two-dimensional scrolling. |
| 1.4.12 text spacing | No clipped text at 1.5 line-height, 0.12 em letter, 0.16 em word, 2 em paragraph spacing. |
| 2.2.2 | No auto-advancing carousels, no autoplay, no session timeout inside a form (the 14-day session is not a UI timer). |
| 2.3.3 / reduced motion | All reveal and sheet motion respects `prefers-reduced-motion`; nothing conveys meaning by motion alone. |
| 3.2.3 / 3.2.6 | Tab order and position never change between screens; help ("How we keep you safe") sits in the same place on Landing, Settings and every safety sheet. |
| 3.3.7 redundant entry | Onboarding resumes with values filled; DOB never re-asked within a session; export does not ask for the email again. |
| 3.3.8 accessible auth | Magic link only, paste allowed, no puzzle, no memorised secret; the link page states the 15-minute expiry up front. |
| 4.1.3 status messages | Every async outcome is announced: photo state changes, saved profile, copied handle, match created, blocked, offline, export ready. |
| Forms | Visible labels always (never placeholder-as-label); `autocomplete` on all identity fields; errors inline next to the field plus a form-level summary when more than one; first invalid field focused on submit; `aria-invalid` + `aria-describedby`. |
| Errors | Never "Something went wrong" alone. Never expose a server string. Never block paste. |

### 5.2 Live regions

- One `aria-live="polite"` region per screen, mounted empty, updated with short sentences ("Photo 1 was approved."). Never two competing regions on one screen; a form-level error uses `role="alert"` and lives next to the summary.
- Announcements are ≤ 1 sentence and never repeat on re-render (deduplicate by message text).
- The offline banner is a `polite` region that also states recovery ("Back online.").

### 5.3 Screen-reader expectations: match reveal

This is the highest-risk announcement in the product — it arrives unprompted and changes what data is available.

1. Arrival on `/match/{id}/new`: focus moves to the `<h1>`. Reading order: heading → sub-line with the other player's name → the one action.
2. Live region message, verbatim: **"It's a match. You and Priya both said yes. Instagram is now visible in this match."**
3. Decorative layers (the drawn line, the flood) are `aria-hidden="true"` and contribute no text.
4. The `.name` pairing is read as text, not as two unlabelled images; both photos carry `alt="Priya's photo 1 of 1"` / `alt="Your photo 1 of 2"`.
5. Deferred match (arrives while elsewhere): toast in a `polite` region, no focus move, Matches tab accessible name becomes "Matches, new match". The user is never interrupted mid-form.
6. Reduced motion: identical text and focus behaviour; only the animation is skipped.
7. Do not announce the Instagram handle at reveal; it is not yet disclosed by an explicit tap (D5).

### 5.4 Focus order (key screens)

Focus follows visual order, top to bottom, and never jumps to a control the user has not reached. Headings are not focusable except the page `<h1>` on route change.

| Screen | Tab order |
|---|---|
| S4 DOB | skip link → back → day → month → year → Continue |
| S9 Photos | skip link → back → tile 1 (or its state action) → tile 2 → tile 3 → Add photo → Photo rules → Continue |
| S13 Discover | skip link → area control → card (photo link, then "Read more") → Not for me → I'd play → overflow → tab bar |
| S15 Match reveal | skip link → `<h1>` (programmatic focus, not tabbable) → See match → Keep looking → tab bar |
| S17 Match | skip link → back → photo → Show Instagram (or its absent-state text) → Copy (once revealed) → Report → Block → tab bar |
| S21 Photos manager | skip link → back → slot 1 → slot 1 actions → slot 2 → … → Add photo |
| S22 Block/report | sheet label (programmatic) → first radio → … → last radio → details field → Submit report → Cancel; Escape closes; on close, focus returns to the invoking control |

Rules: after any async action that removes the focused element, move focus to the nearest surviving container (the tile, the card, the sheet heading) — never to `<body>`. Sheets trap focus; pages do not.

### 5.5 Testing gates for Task 8

Keyboard-only pass on every flow · VoiceOver (iOS Safari) and TalkBack (Android Chrome) on S1, S4, S8, S9, S13, S15, S17, S22, S25 · 200 % zoom and 320 px · `prefers-reduced-motion` · axe clean with zero serious/critical · iOS camera-format (HEIC) path exercised on a real device.

---

## 6. Privacy guardrails

### 6.1 Never rendered, in any frame, state, skeleton, toast, or empty state

- Exact coordinates, or anything from which they can be derived (no map, no pin, no "lat, lng", no decimal distance).
- A displayed date of birth, or any string that reconstructs it (age only).
- Email addresses — including the signed-in user's own; "Check your inbox" replaces echo.
- Session tokens, link tokens, presigned URLs, object keys, bucket names, provider request ids, `provider_reference`, API keys.
- Another player's moderation state, report status, ban status, report count, or the fact that they were reported.
- Any other player's exact play-time rows beyond the published slot summary; any other player's full profile id in a visible URL.

### 6.2 Handling rules

- **Presigned PUT URL:** memory only, single use, never in the DOM attribute after the request resolves, never logged, never in a share sheet. If it must be held across a retry, hold it for the retry only and discard after 5 minutes.
- **Presigned GET (photo display):** `GET /api/v1/photos/{id}/url` returns a 5-minute URL with `Cache-Control: no-store`; fetch per tile, hold in memory, never request it for a rejected photo (it 404s), never write it to `localStorage`/`CacheStorage`, never log it.
- **Instagram handle:** fetched on explicit tap, held in memory for the session, never cached, never prefetched, never in the page title or meta description, never in a link preview.
- **Analytics:** no third-party analytics or pixels in V1. If added later, no per-profile events, no reaction targets, no photo URLs.
- **Client storage:** session is an `HttpOnly` cookie. No tokens, photos, profiles or handles in `localStorage`. Theme/UI prefs are the only local keys permitted.
- **Sharing:** no public profile page, no "share this player", `Referrer-Policy: no-referrer` on all auth and match routes, and no OG tags populated with member data anywhere.
- **Moderation state is owner-only:** the photo list endpoint returns the caller's photos only; a rejected photo is never served to anyone else, in any size, at any status.
- **Visual hiding is not authorization.** Every hidden element (Instagram panel, delete controls) is also absent from the response. Copy for developers: if a field is not needed for the frame, do not fetch it.

### 6.3 Age and identity handling

- DOB is the **one** place a date of birth exists in a frame: the empty entry field on S4. It is never populated from stored data — the profile API returns no `birth_date`, so no reload, resume or retry can display it. It returns a server-computed integer `age` instead (§8.2), and that integer is the only age value the client ever holds.
- The DOB gate is a server decision (registration). The client renders the server's verdict and never computes eligibility for gating purposes.
- No identity documents, no selfie verification, no "verified" badge in V1; do not imply verification exists.
- Because a DOB is sensitive, S4 carries the promise "We never show your date of birth." — removing that sentence requires a product decision, not a copy tweak.

---

## 7. Component inventory for Task 8

| Component | Variants / states |
|---|---|
| `Button` | primary (court), secondary (ink outline), quiet (text), destructive (alert); enabled/disabled-with-reason/loading; 44/52 px heights |
| `Field` | text, textarea, select, date parts, time; label, helper, error, counter; `aria-invalid` |
| `RadioCard` | unchecked/checked/focus/disabled; fieldset wrapper |
| `StatusChip` | pending, approved, rejected, failed, expired; text + icon, 4 px radius, never pill |
| `MetaRow` | score label left, value right, 1 px rule, tabular numerals |
| `PhotoTile` | empty/uploading/processing/pending/approved/rejected/failed/too-large/wrong-format/expired |
| `ProgressBar` | determinate + indeterminate, `role="progressbar"` |
| `Sheet` | bottom sheet dialog, focus trap, Escape, drag handle `aria-hidden`, body scroll lock |
| `Toast` | polite live region, one at a time, 6 s, dismissible |
| `Banner` | offline, discovery-blocked, scheduled-deletion |
| `Deck` | skeleton/loaded/empty/error/offline |
| `PlayerCard` | photo, name/age, meta rows, bio clamp, reactions, overflow |
| `MatchRow`, `MatchReveal` | new/seen; reduced-motion variant |
| `EmptyState` | title, one-sentence reason, one or two actions |
| `Checklist` | ready-check rows with text status |

---

## 8. Open questions and assumed contracts

### 8.1 Open questions (recommended default in brackets)

| # | Question | Impact if unanswered |
|---|---|---|
| Q1 | Does Task 5 accept a **manual area** (postcode/neighbourhood) when geolocation is denied? | The denied state has no fallback and the user is dead-ended. [Default: accept UK postcode → area, no coordinates returned to the client.] |
| Q2 | How do two matched players with **no Instagram** actually arrange a game? V1 has no chat and Instagram is the only contact channel. | A match can be a dead end. [Options: require Instagram before discovery eligibility; add a minimal in-app "propose a court and time" after match; or state the limitation clearly. Default: state it clearly + prompt to add Instagram in `You`.] |
| Q3 | Should `ranking` (free text, up to 50 chars) appear on a discovery card? | Unvalidated free text on a stranger's card can carry contact details that the bio filter caught, or confusing claims. [Default: hide on the card, show on the detail view labelled "Ranking (self-reported)".] |
| Q4 | Does a **photo reorder** endpoint exist (display_order 1–3)? None in Tasks 1–3. | If not, no reorder UI at all rather than a control that cannot persist. [Default: defer to V2.] |
| Q5 | Exact **report categories** and appeal states from Task 7. | The report sheet's radio list is a guess at the enum. [Default: the six categories in S22; align before Task 8.] |
| Q6 | ~~Is `GET /api/v1/photos` the agreed contract?~~ **Resolved and implemented**: owner-only `{"photos":[{"photo_id","display_order","status"}]}`, no URLs. | — |
| Q7 | Match **notification** on app open only (no email/push in V1)? | If a match email is expected, S15's "deferred match" path changes. [Default: in-app only.] |
| Q8 | Minimum browser targets (Safari 16+/iOS 16, Chrome 110+)? | Affects native date/time input behaviour and `:has()` usage. [Default: iOS 16+, Chrome 110+, no IE-era fallbacks.] |
| Q9 | Is client-side **HEIC conversion** in scope for iPhone uploads? | Today the server rejects HEIC, and iOS camera output defaults to HEIC in some configurations. [Default: no conversion; clear error + settings hint.] |
| Q10 | Is there a **photo delete/replace** endpoint? None in Tasks 1–3, and `/photos/{id}/url` 404s for rejected rows. | With 3 approved photos the user cannot change one, and approved/pending tiles have no remove action. [Default: request `DELETE /api/v1/photos/{id}` in Task 3 or 5; until then, no remove control is shown.] |

### 8.2 Contracts this spec assumes (not yet built)

**Task 3 — moderation (approved plan, implemented)**
- `POST /api/v1/photos/{id}/process` returns **synchronously** `{photo_id, status: "approved" | "rejected" | "pending"}`. `pending` = the provider was inconclusive (a `200`, not an error) and the photo is visible to nobody. `503` = the check could not complete: provider timeout, transport error, malformed response, or a database failure — status and provider reference unchanged.
- Owner-only `GET /api/v1/photos` → `{"photos":[{"photo_id","display_order","status"}]}`, no URLs. This is how every state is rediscovered after a reload.
- Owner-only `GET /api/v1/photos/{id}/url` → `{photo_id, status, url, expires_in_seconds: 300}`, 5-minute presigned GET, `Cache-Control: no-store`; `404` for unknown, foreign, or **rejected** photos (object deleted).
- Rejected rows free their slot on the next upload, so **Upload another photo** is the only replacement path — no edit, crop or replace verb exists.
- Moderation is synchronous and never auto-retries; the client shows a minimum 1.2 s checking state and offers manual retry on `503`.
- Approved photos still need a read path for *other* viewers: a short-lived signed URL issued with the discovery card (see Task 5). **If that does not exist, discovery cards are text-only — a design change, not a cosmetic one.**

**Profile contract addition (D1 — required by S18 and the preview view)**
- `GET /api/v1/profile` must gain a server-computed integer `age` (18–120) derived from `users.birth_date`. `birth_date` itself is never returned, and `PUT` ignores any `age` the client sends.
- This is the smallest change that lets S18 and "preview as another player" show the same age that discovery cards already show. The alternative — fetching the DOB and computing age client-side — is rejected: a date of birth must not leave the server.
- If this is refused, remove the age from S18 and from the preview; do not substitute "age unknown", which reads as a broken profile.

**Task 5 — location and discovery**
- `GET /api/v1/discovery?radius=<miles>&cursor=…` → 20–50 cards: `{player: {display_name, age, gender, skill_level, ranking?, bio?, play_summary, photo_urls[], distance_miles (integer ≥ 0)}, next_cursor}`. **`distance_miles` is already rounded server-side**; the client only maps `0 → "Under 1 mile"` and `n → "n miles"`.
- `PUT /api/v1/location` accepting a coarse area (or coordinates server-side only) and never returning them.
- Travel mode: destination city name only; no coordinates in any response.
- Approved photo delivery for cards uses the same short-lived signed-URL pattern, issued per card and never cached client-side.

**Task 6 — reactions, matches, blocks**
- `POST /api/v1/reactions` `{target_id, action: "like" | "pass"}` → `{matched: bool, match_id?: string}`; `pass` is permanent (drives D2's confirmation).
- `GET /api/v1/matches` → rows with `matched_at`, `has_instagram` (boolean only — no handle) and `photo_urls[]`.
- `GET /api/v1/matches/{id}` → detail with `photo_urls[]`; **handle only via an explicit disclosure call**, e.g. `GET /api/v1/matches/{id}/instagram` (D5).
- **Matched-only signed photo URLs (new requirement, D2):** `photo_urls[]` carries short-lived signed URLs for the other player's **approved** photos only, issued only while a match exists, never cached, and not issued at all once a block exists in either direction. The owner-only `GET /api/v1/photos/{id}/url` cannot serve this — it 404s foreign photos by design. Without this field, S15/S16/S17 are text-only.
- `POST /api/v1/blocks` `{target_id}`; blocking revokes the match, the photo URLs and Instagram access in both directions.

**Task 7 — safety and account**
- Report categories enum (Q5), 72-hour review, one appeal within 14 days.
- `DELETE /api/v1/account` → immediate hiding + 30-day recovery boundary timestamp; recovery on next successful sign-in.
- `GET /api/v1/account/export` → JSON (owner-scoped). **Assumed:** either a direct authenticated download (Preparing → Error only) or a one-time 10-minute link (Preparing → Ready → Expired). S27 supports both; the TTL is not yet contractual.

If any assumption proves wrong, the affected screen is marked in §3 and re-reviewed before Task 8 — none of them change the visual direction.

---

## 9. Review checklist for this document

- [ ] Visual direction accepted (paper/ink + court blue, optic reserved for mutual match).
- [ ] D1–D6 confirmed or revised.
- [ ] Q1–Q10 answered or explicitly deferred.
- [ ] The two API additions in §8.2 accepted: profile `age` (D1) and matched-only `photo_urls[]` (D2).
- [ ] Copy register accepted (UK English, sentence case, no exclamation marks, no email echo).
- [ ] Photo state matrix agreed as the single source of truth for Task 3 + 8.
- [ ] Screen inventory agreed as V1 scope — anything to cut before Task 8.
