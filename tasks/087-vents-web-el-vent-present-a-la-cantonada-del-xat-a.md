---
title: Vents web: el vent present a la cantonada del xat, animat pel que fa
status: done
depends_on: ["085"]
---

## Why

The wind should feel present next to the person, not only as a small face in each row: a larger avatar that sits partly outside the chat's frame and moves with what the wind is doing.

## Acceptance criteria

- [x] In a wind's chat on a wide screen (at least 900 px), the wind's avatar (the active pack, `face`) sits at the bottom corner of the chat, partly outside the message column's frame, about 96 px; on narrow screens it is hidden. A node test that the element is built for a wind chat and not for a rotllana.
- [x] Its mood follows the wind's state from 085: working moods while a turn runs (the existing `data-mood` keyframes in `winds.css`), a waiting pose when it has an ask, a short done animation when the turn ends. A node test of the state → mood mapping.
- [x] It never covers the composer or a message: the message column keeps a margin on that side. Checked with screenshots at 900, 1280 and 1600 px.
- [x] With `prefers-reduced-motion` it does not animate.

## Out of scope

- Android (088).
- New avatar art (094 adds volume).

## Notes

- Moods: `moodOf()` / `setMood()` in `winds.js` (194, 316), keyframes in `winds.css` 275-336.
- Web: node tests with `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; check in a sandboxed server in the browser pane, dark and light theme, with before/after screenshots in `output/ui-review/` (ignored).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
- Done: the corner wind is `.vw-wind-corner` in `chatView()` (winds-workspace.js), a zero-height row between the thread and the composer; `cornerMood()` reuses 085's `windMark()` and `moodOf()`. «waiting» is not a winds.js mood (the face draws idle): the lean towards the composer is CSS on the corner's `data-mood`. It updates from `paintMark()`, so it follows the same pending/asks repaints as the header label.
- Not shown in Gregal's own thread (its face is already large at the top, B14) nor in groups. Also hidden between 900 and 1200 px while a work pane (089's computer or Gregal Docs) is open, because the chat is then about 300 px wide.
- Screenshots (ignored): `output/ui-review/087/` at 900, 1280 and 1600 px, dark and light, done/working/waiting and with the computer pane; the script also checked by bounding boxes that the face covers no message, ask or composer, and that with `prefers-reduced-motion` no animation runs in the corner for the cares, esperit, cap and rosa packs.
- Follow-up: the mapa pack draws its canvas from JS and keeps moving while the wind works even with reduced motion (winds.js `mapWake`), in the corner too; CSS cannot stop it.
- Follow-up done (fix/vents-web-detalls): the computer pane now floats at `max-width:1000px` like the docs pane; the corner wind stays hidden with a work pane open below 1200 px (the pane floats over it up to 1000 px and sits beside it above).
- Done in `fix/vents-mapa-moviment`: under reduced motion the mapa map is one still frame, working or not, and it follows the setting when it changes (`mapReduce` listens to the media query in winds.js); Android `MapFace` likewise stays still when `windMotionEnabled()` is false. Tests: `wind-face.test.cjs` and `WindMapTest`.
