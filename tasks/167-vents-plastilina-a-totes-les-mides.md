---
title: Vents: plastilina a totes les mides
status: done
depends_on: []
---

## Why

A wind in the Cares pack is clay where it is large and a code drawing in a list row or the chat header, so the same wind looks like two different characters. The pelut pack already solved this (155): at small sizes it holds its clay sheet's still frame. Cares should do the same, on the web and on Android.

## Acceptance criteria

- [x] Web: a small Cares face (list rows and the chat header; the home start faces and the Avui card stay animated, as in 113) is a clay canvas holding the still frame of its mood's sheet. It is tinted with the wind's colour and carries the trade badge if it has a trade. Until the sheet loads, or if it fails, the drawn face stays. `characters.test.cjs` / `wind-face.test.cjs`.
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
- Web: `face()` in `winds.js` hands every non-large Cares face (`large` falsy) to `gregalCharacters.play(drawn, w, true)`; large faces are still played by their callers, as before. A caller that plays a face `face()` handed out still (`play(x, w, false)`, as `clay(w, face(w))` does for the start faces and the Avui card) makes it animate: `characters.js` keeps the player of each handed face and moves it into the loop on its whole sheet, whether its still frame had loaded yet or not. `change()` drops a still frame that lands after the face was set moving, so a late still cannot take back the whole sheet (`characters.test.cjs` "a still face played while its new mood's still frame loads keeps the whole sheet"). Below 28 px `data-kit` is not set, so the player draws no badge. `packFace` now only hands the minairó itself, since `face()` already hands Cares and the pelut. One tinted still per wind colour, silhouette and mood is shared by every row through the existing `sheet()` cache.
- Android: "below the large size" is read as `compact` (under 64 dp) and not `clay`; a 64-96 dp face without `clay` (the creator preview, `WindVidaTest`) keeps the Canvas face. The small clay face always holds still, even when tappable, so it loses the Canvas face's tap wiggle, the breath lines and the idle glance; the turn cue (`windTurnCue`) stays on the modifier.
- Tests of the small Canvas Cares face now point at a missing sheet folder (`LocalWindClay provides "no-such-folder"`), so they keep testing it as the fallback: `WindClayTest.withoutTheSheetsASmallAvatarKeepsTheCanvasFace` (renamed from `smallAvatarsAndMissingSheetsKeepTheCanvasFace`), `MascotMinimalTest.smallSpecialistKeepsItsUnclutteredSilhouette`, `WindCaresTest.smallFacesKeepTwoLargeCrispEyes` and the two breath tests in `WindBreathTest`. With the sheets, those faces are clay by this task. Their assertions are unchanged.
- `ChatHeaderFadeLookTest` failed once in a full run ("Missatge 7" not found, though the screenshot shows it) and passed alone and in the next full run: it looks order or timing dependent.
- Follow-up: the comment and name of `winds-start.test.cjs` "large faces play the clay winds ... list avatars stay drawn" describe the workspace layer only (it stubs `face()`); small Cares faces are now clay through `winds.js`.
- The review found the start faces and the Avui card held still; the coordinator decided they stay animated (113). `characters.test.cjs` "a face from face() that its caller plays animates..." runs the real `winds.js` with the player, the case the stubbed `winds-start.test.cjs` misses. The pelut test now hands its small face twice as still, since a non-still second call now animates it.
- Intended change to 155: the pelut's start faces, which held still since 155, animate again through the same path. The start strip animates for every wind (coordinator's decision).
