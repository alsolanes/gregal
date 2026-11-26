---
title: Vents: l'espera es veu sobre la figura
status: todo
depends_on: []
---

## Why

When a wind is waiting for you (a pending question or approval, the asks in `WindAsks.kt` and winds-workspace.js), you only find out from text. The clay `waiting` sheet exists, but it has no mark of its own and is not used everywhere the wind shows. OpenBot's figures show "waiting" and "blocked" on the figure itself. We want the same with our clay: wherever the wind's face shows, you can tell at a glance that it needs you.

## Acceptance criteria

- [ ] Web: a wind with a pending ask shows the `waiting` mood on its faces on the home strip, the crew list and the chat header, not only in its open chat. When the ask is answered or expires, it goes back to its previous mood. Node test with a stubbed asks list.
- [ ] Web: a clay face in `waiting` carries a mark drawn over every frame, a small speech bubble with «?» at the top-right corner, tinted with the wind's colour like the trade badge (159). The mark does not overlap the badge. `characters.test.cjs` checks the mark is drawn in `waiting` and not in other moods.
- [ ] Android: the same rule and the same mark in `WindClayFace` for winds with a pending ask, on the home and the chat header. `WindClayTest` and `VentsExperienceTest`.
- [ ] The face's accessible label says the wind is waiting for you, in Catalan and English. Web and Android tests.

## Out of scope

New Blender renders. Gaze (169). The `done` and `failed` sheets, which already have their own animation.

## Notes

- Web: `winds-workspace.js:179` already maps asks to `'waiting'` for one place; `winds.js` `moodOf`; `characters.js` `draw` / `badge`.
- Android: `WindAsks.kt`, `WindClay.kt` (`drawBadge`), `WindFace.kt`.
