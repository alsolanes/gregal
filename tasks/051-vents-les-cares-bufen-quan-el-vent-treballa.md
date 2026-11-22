---
title: "Vents: les cares bufen quan el vent treballa"
status: done
depends_on: ["045", "046"]
---

## Why

The Cares pack (the current faces) gets the one thing only Gregal has: while a wind works, it blows a few breath lines toward the wind's direction, as in «Cares que bufen» in `mockups/vents-avatars.html`. Today the working state is the same as everyone's.

## Acceptance criteria

- [x] Web: a Cares face in the `working`, `thinking`, `searching`, `reading` or `writing` mood shows three breath lines leaving the face toward the opposite of the wind's direction (045's function), and two short lines below 28 px. Idle shows none. A node test with the fake DOM.
- [x] Android: `WindFace` with the Cares pack does the same, in the same direction as the web. A Compose test.
- [x] The lines stay inside the face's own box plus a margin that doesn't cover the wind's name in list rows. A node test checks the overlay's box, and a screenshot of the sidebar is enough on Android.
- [x] The lines move only while the wind works; when the mood goes back to idle, they disappear. A test on each platform.

## Out of scope

- Changing the face drawing, the kits or the other moods.

## Notes

- Web: `face()`/`setMood()` in `internal/web/app/winds.js`. Android: `WindFace.kt`, `drawKit`/mood handling.
- Reference: `bufen()` in `mockups/vents-avatars.html`. The `working` mood's cog stays for packs other than Cares; in Cares the breath replaces it.
- This touches both clients in one diff because each side is small (well under 200 lines). If it grows, split it into web and Android.

### What was built (2026-11-22)

- Same lines on both clients, in the face's 80 box around its centre, drawn along +x and turned `bearing + 90` (Tramuntana blows south, Ponent east): three long lines (`M35 -7C39 -9 43 -5 47 -7.5`, `M36 0C40 -2 44 2 48 0`, `M35 7C39 9 43 5 47 7.5`, width 3.2) or, under 28 px, two short bolder ones (width 5). They reach 48 from the centre: the face box plus 8 units (10 %) on each side, i.e. 1.8 px on an 18 px sidebar face and 3.8 px on a 38 px chat avatar, less than the 10–13 px gap before the name in every list row. Stroke: the mockup's gradient (wind colour lightened 15 % → transparent). The long lines are dashes 3/5 flowing outward every 0.75 s; the gap is wider than the 3.2 stroke because round caps grow each dash by the stroke width (a first 5/2.5 drew solid lines). The short pair is solid, as the mockup's `40 0`, so under 28 px the breath shows but does not stream.
- Moods: thinking, searching, reading, writing, working (not discussing, done, failed, news, idle). In Cares the breath replaces the working cog; the other work marks stay. Other packs keep the cog and never blow.
- Web: `face()` records `{ deg, fill }` for Cares faces only (`faceBreath`), and `setMood()` removes and redraws the breath (`blow()`), so `gregalWinds.mood(svg, 'idle')` takes it away. The breath is a nested `<svg class="wf-breath">` with `x/y -8`, `96×96`, `overflow:hidden`: the box is the clip. `face()` has no pixel size, so the short pair is chosen in `winds.css` by the list contexts whose faces are under 28 px (`.vw-nav`, `.vw-pick-card`, `.vw-task-faces`, `.vw-crew-chip`, `.vw-owner`, `.vw-owner-faces`, `.vw-address-wind`, `.vw-thread-solo`, `.vw-wx-by`).
- Android: `WindFace` draws the breath after the body for Cares (`drawBreath`, `windBreathLines(size)`); `EsperitFace` is untouched.
- Motion: unlike Esperit's tails, the breath follows the Cares motion rule. On the web it stops under `prefers-reduced-motion` (the existing `.winds-face *` rule). On Android it streams whenever `windMotionEnabled()` (also in small list faces, where the rest of Cares stays still), and it is drawn still when motion is off, because `WindAuraTest` («sense moviment l'aura queda quieta») requires a working Cares face to be completely still with motion off.

### Tests

- Node (`internal/web/app/wind-face.test.cjs`): five moods blow, the others do not, direction (Tramuntana 90, Ponent 0), cog replaced only in Cares, short pair in the small contexts' CSS; the overlay box is −8..88 and every point of every line stays in it and outside the face for the eight directions; redraw on mood change, gone on idle, animation tied to the work moods, dash gap wider than the stroke.
- Android (`WindBreathTest`, xxhdpi): line counts by size and the same box check; rendered on black, Tramuntana blows only below its box and Ponent only to its right, nothing beyond the 4 dp margin, reading blows, idle and discussing do not, the cog is gone and the book stays; along the middle line there are gaps between dashes and they move with motion on in a 40 dp face, the breath is still with motion off, and it disappears on idle.

### Not verified

- Not seen in a browser. On Android it was rendered in Robolectric (eight directions, 24 dp and 96 dp faces) and looked like the mockup; the sidebar screenshot was replaced by the pixel test for the margin.
- Follow-up: narrow-screen chat avatars shrink to 26 px inside media queries (`winds-workspace.css` lines ~476 and ~753); they keep the three long lines because the small-face list in `winds.css` does not cover media queries.
- Follow-up: on light backgrounds the pastel breath is faint, as in the mockup.
- Fix after merge: the breath streamed in list faces (under 64 dp, not tappable), which broke `MascotMinimalTest::compactAvatarKeepsItsStatusWithoutAnimating` (an avatar in a list stays still while work continues). Now it is shown but still there, like the rest of Cares, and `WindBreathTest` checks a list face keeps still and a tappable one streams. The test name changed from «even in a list face» to «still in a list face» because the older requirement wins.
