---
title: Vents: plastilina a totes les mides
status: todo
depends_on: []
---

## Why

A wind in the Cares pack is clay where it is large and a code drawing in a list row or the chat header, so the same wind looks like two different characters. The pelut pack already solved this (155): at small sizes it holds its clay sheet's still frame. Cares should do the same, on the web and on Android.

## Acceptance criteria

- [ ] Web: a small Cares face (list rows, the chat header, the home strip's small faces) is a clay canvas holding the still frame of its mood's sheet. It is tinted with the wind's colour and carries the trade badge if it has a trade. Until the sheet loads, or if it fails, the drawn face stays. `characters.test.cjs` / `wind-face.test.cjs`.
- [ ] Web: a mood change on a small face swaps to that mood's still frame without starting an animation loop (nothing is added to `running`). Node test.
- [ ] Android: `WindFace` below the large size draws the Cares wind with `WindClayFace` holding a still frame, with the badge for a trade. The chat header and the list use it. Without the sheets it falls back to the Canvas face. `WindClayTest`.
- [ ] The other packs (cap de vent, rosa, mapa, esperit) stay code-drawn at every size. Web and Android tests.
- [ ] Below 28 px/dp the badge is not drawn: it would be a dot. Web and Android tests.

## Out of scope

New sheets or re-renders. The large faces, which are already clay. The minairó's small faces, unless they come for free through the same path.

## Notes

- Web: `winds.js` around the pelut branch (`large === 'figure' || !root.gregalCharacters ? drawn : root.gregalCharacters.play(drawn, w, large !== true)`), `characters.js` (`play` with `still`, the `kept` cache of 24 tinted sheets). Check that a long list does not tint more than one still per wind and mood.
- Android: `WindFace.kt` (the `clay` flag and the pelut path at every size), `WindClay.kt`.
- Check it in the browser and on emulator-5554 with several winds in the list.
