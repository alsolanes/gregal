---
title: Vents: l'espera es veu sobre la figura
status: done
depends_on: []
---

## Why

When a wind is waiting for you (a pending question or approval, the asks in `WindAsks.kt` and winds-workspace.js), you only find out from text. The clay `waiting` sheet exists, but it has no mark of its own and is not used everywhere the wind shows. OpenBot's figures show "waiting" and "blocked" on the figure itself. We want the same with our clay: wherever the wind's face shows, you can tell at a glance that it needs you.

## Acceptance criteria

- [x] Web: a wind with a pending ask shows the `waiting` mood on its faces on the home strip, the crew list and the chat header, not only in its open chat. When the ask is answered or expires, it goes back to its previous mood. Node test with a stubbed asks list.
- [x] Web: a clay face in `waiting` carries a mark drawn over every frame, a small speech bubble with «?» at the top-right corner, tinted with the wind's colour like the trade badge (159). The mark does not overlap the badge. `characters.test.cjs` checks the mark is drawn in `waiting` and not in other moods.
- [x] Android: the same rule and the same mark in `WindClayFace` for winds with a pending ask, on the home and the chat header. `WindClayTest` and `VentsExperienceTest`.
- [x] The face's accessible label says the wind is waiting for you, in Catalan and English. Web and Android tests.

## Out of scope

New Blender renders. Gaze (169). The `done` and `failed` sheets, which already have their own animation.

## Notes

- Web: `winds-workspace.js:179` already maps asks to `'waiting'` for one place; `winds.js` `moodOf`; `characters.js` `draw` / `badge`.
- Android: `WindAsks.kt`, `WindClay.kt` (`drawBadge`), `WindFace.kt`.
- Web: `waitMood` / `paintWaiting` in `winds-workspace.js` give the waiting mood to the home strip, the crew list (`.vw-member`) and a group's header faces; `paintAsks` repaints them, so an answered or expired ask (gone on the next `loadAsks`) puts back idle or news. Home and the crew view now call `loadAsks()`. The solo chat header already had it (`windMark` / `cornerMood`).
- Web: the mark is drawn in `characters.js` (`waiting`) for clay faces only; a code-drawn face (other packs, or before the sheet loads) has the pose and the label but no bubble. The label is `winds.waitingForYou` in `i18n.js`.
- Android: `drawWaiting` in `WindClay.kt` reaches the chat header through the existing `windPresenceMood`; the home strip gets `waiting` from `vm.windAsks` in `CompanyScreen`, which now also reloads the asks in its `LifecyclePoll`. The face's label on Android is the start tile's state description (`winds.state.waiting`); the chat header keeps its visible «T'espera» text.
- Follow-up: on the home strip a wind with news and a pending ask gets its news mark over the «?» bubble, both at the top-right (`NewsBadge` on Android, `.vw-start-face > .vw-news-dot` on the web).
- Follow-up: the web sidebar faces and the Android group header and crew list do not show the waiting mood yet.
- Not verified in a browser or on emulator-5554; checked with the Node and Robolectric tests only.
