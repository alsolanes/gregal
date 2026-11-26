---
title: Vents: plastilina a totes les mides
status: blocked
depends_on: []
---

## Why

A wind in the Cares pack is clay where it is large and a code drawing in a list row or the chat header, so the same wind looks like two different characters. The pelut pack already solved this (155): at small sizes it holds its clay sheet's still frame. Cares should do the same, on the web and on Android.

## Acceptance criteria

- [ ] Web: a small Cares face (list rows, the chat header, the home strip's small faces) is a clay canvas holding the still frame of its mood's sheet. It is tinted with the wind's colour and carries the trade badge if it has a trade. Until the sheet loads, or if it fails, the drawn face stays. `characters.test.cjs` / `wind-face.test.cjs`.
- [x] Web: a mood change on a small face swaps to that mood's still frame without starting an animation loop (nothing is added to `running`). Node test.
- [x] Android: `WindFace` below the large size draws the Cares wind with `WindClayFace` holding a still frame, with the badge for a trade. The chat header and the list use it. Without the sheets it falls back to the Canvas face. `WindClayTest`.
- [x] The other packs (cap de vent, rosa, mapa, esperit) stay code-drawn at every size. Web and Android tests.
- [x] Below 28 px/dp the badge is not drawn: it would be a dot. Web and Android tests.

## Out of scope

New sheets or re-renders. The large faces, which are already clay. The minairó's small faces, unless they come for free through the same path.

## Notes

- Web: `winds.js` around the pelut branch (`large === 'figure' || !root.gregalCharacters ? drawn : root.gregalCharacters.play(drawn, w, large !== true)`), `characters.js` (`play` with `still`, the `kept` cache of 24 tinted sheets). Check that a long list does not tint more than one still per wind and mood.
- Android: `WindFace.kt` (the `clay` flag and the pelut path at every size), `WindClay.kt`.
- Check it in the browser and on emulator-5554 with several winds in the list.
- Web: `face()` in `winds.js` hands every non-large Cares face (`large` falsy) to `gregalCharacters.play(drawn, w, true)`; large faces are still played by their callers, as before. Below 28 px `data-kit` is not set, so the player draws no badge. `packFace` now only hands the minairó itself, since `face()` already hands Cares and the pelut. One tinted still per wind colour, silhouette and mood is shared by every row through the existing `sheet()` cache.
- Android: "below the large size" is read as `compact` (under 64 dp) and not `clay`; a 64-96 dp face without `clay` (the creator preview, `WindVidaTest`) keeps the Canvas face. The small clay face always holds still, even when tappable, so it loses the Canvas face's tap wiggle, the breath lines and the idle glance; the turn cue (`windTurnCue`) stays on the modifier.
- Tests of the small Canvas Cares face now point at a missing sheet folder (`LocalWindClay provides "no-such-folder"`), so they keep testing it as the fallback: `WindClayTest.smallAvatarsAndMissingSheetsKeepTheCanvasFace`, `MascotMinimalTest.smallSpecialistKeepsItsUnclutteredSilhouette`, `WindCaresTest.smallFacesKeepTwoLargeCrispEyes` and the two breath tests in `WindBreathTest`. With the sheets, those faces are clay by this task. Their assertions are unchanged.
- `ChatHeaderFadeLookTest` failed once in a full run ("Missatge 7" not found, though the screenshot shows it) and passed alone and in the next full run: it looks order or timing dependent.
- Follow-up: the comment and name of `winds-start.test.cjs` "large faces play the clay winds ... list avatars stay drawn" describe the workspace layer only (it stubs `face()`); small Cares faces are now clay through `winds.js`.
- BLOCKED (review): on the web, `face()` now hands every Cares face with a falsy `large` to the player as still. The home start faces (`winds-workspace.js` `clay(w,face(w))` in the start strip) and the Avui card (`clay(w,face(w))`) call `face(w)` and then play it through their own `clay()`, which `play()` ignores for a face already handed. So they now hold still instead of animating, which reverses task 113. `winds-start.test.cjs` stubs `face()` and does not catch it. The criterion's "the home strip's small faces" could mean those start faces (still, reversing 113) or not (they keep animating). To decide: (a) keep them animated, by having those two callers ask `face()` for a large-but-plain face, which needs `winds-workspace.js` (task 165 edits it at the same time), or by letting `play(svg, w, false)` promote a face handed as still in `characters.js`; or (b) make them still and update the name and comment of the 113 test in `winds-start.test.cjs`. The pelut already holds still there since 155.
