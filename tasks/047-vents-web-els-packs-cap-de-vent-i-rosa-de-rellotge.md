---
title: "Vents (web): els packs Cap de vent i Rosa de rellotge"
status: done
depends_on: ["045"]
---

## Why

Two more avatar packs from `mockups/vents-avatars.html`: Cap de vent (the blowing heads of old maps: full cheeks, closed eyes, breath toward where the wind goes) and Rosa de rellotge (the wind rose as a dial, with the wind's point long and in its colour).

## Acceptance criteria

- [x] `cap` and `rosa` are valid values for the wind's `avatar` and for the account theme; anything else is still 400. A Go test.
- [x] Both packs appear in the preferences and in the wind editor's picker (Free mode), and every face in Vents uses them when chosen. A node test with the fake DOM.
- [x] Cap de vent blows toward the opposite of the wind's direction (045's function). Rosa de rellotge points its long tip in the wind's direction. Below 28 px both drop the details (breath to two short lines; only the wind's point), as in the mockup. A node test checks the angle and the small variant.
- [x] While working, the breath flows (Cap de vent) and the point swings (Rosa); idle, they are still unless expressive motion is on. A node test.

## Out of scope

- Android: 048.
- A combined rose for rotllanes.

## Notes

- Drawing reference: `bufador()` and `rosa()` in `mockups/vents-avatars.html`.
- Update `docs/api-contract.md` with the new values.

### What was built (2026-11-22)

- Server: `companion.ValidAvatar` accepts `cap` and `rosa`, so both the wind's `avatar` and the theme take them (the theme check reuses it). Contract updated.
- Web: `packs` and `themeChoices()` in `winds.js` list them, so Preferences › Avatars and the Free picker offer them with no other change. Names: «Cap de vent» / "Wind head", «Rosa de rellotge» / "Compass dial" (`winds.avatarCap`, `winds.avatarRosa` in `i18n.js`).
- `cap()` and `rosa()` in `winds.js` port the mockup to `svgNode`, in a 100 box scaled to 0.8 like `esperit()`. No kits. Cap de vent: head on the side the wind comes from, breath group `.wf-breath` turned by `(direction + 90) % 360` (drawn along +x, so it blows to direction + 180). Rosa de rellotge: dark dial from the wind's colour, eight cream points, the wind's point in `.wf-point` inside a group turned by `direction`. The ring is drawn unless small; the 32 minute ticks only on the large figure (`large === true`), instead of the mockup's 56 px threshold.
- Small variant: `face()` has a new fourth argument `size` (CSS px). Under 28 the packs draw the small variant (Cap: two thick breath lines and only the mouth; Rosa: only the wider wind point). Callers that know they are small pass it: in `winds-workspace.js` the local `face(w, work, size)` helper, used by the sidebar thread and channel rows (20), the conversation owner stack (24) and the weather "by" line (18). Every other caller passes nothing and gets the full drawing.
- Motion: same rule as 045. `wf-idle` only when `prefers-reduced-motion` is not set; in `winds.css` the breath dash flows (1.8 s) and the point sways ±5° (4 s) only under `wf-idle`; while working the breath flows at 0.7 s and the point swings ±10° at 1.4 s with `!important`.
- The existing Preferences test in `internal/web/winds.test.js` now expects five buttons instead of three, because the requirement added two packs.

### Tests

- Go: `TestCapAndRosaAreAvatarPacks` in `internal/web/winds_avatar_test.go`.
- Node: `internal/web/app/wind-face.test.cjs` (Cap direction and small variant, Rosa direction and small variant, motion gate); `internal/web/winds.test.js` (Preferences and Free picker with both packs, every face redrawn); `internal/web/app/winds-start.test.cjs` (the sidebar asks for the small variant, the 40 px start faces do not).

### Not verified

- Not seen in a browser: the drawings were not compared with the mockup, and the CSS `transform-origin:0 0` on `.wf-point` (inside a translated and rotated group) was not checked rendering.
- After review, the other faces fixed at 26 px also pass `size`: the conversation owner, the address bar, the crew chips, the task faces and the pick cards (`winds-shared.js`). Follow-up: faces that shrink only under a media query (mobile messages at 26 px, the `refine.css` nav at 18 px) still draw the full figure, because `face()` cannot know the viewport.
- Follow-up: Esperit de vent could use the same `size` argument for its mockup compact variant (045 left it out because there was no size).
- Follow-up: the mood badges in the top-right corner sit over the Rosa dial, which fills the whole box.
- Rebased onto 051 (Cares que bufen): Cap de vent's breath group is `wf-cap-breath`, not `wf-breath`, because 051's `setMood()` removes `.wf-breath` on every mood change and its CSS animates `.wf-breath path`. A test checks a mood change keeps the head blowing.
- 051 picks the small Cares breath by CSS row classes, this task by the `size` argument of `face()`. Follow-up: use one of the two.
