---
title: Vents web: el vent present a la cantonada del xat, animat pel que fa
status: todo
depends_on: ["085"]
---

## Why

The wind should feel present next to the person, not only as a small face in each row: a larger avatar that sits partly outside the chat's frame and moves with what the wind is doing.

## Acceptance criteria

- [ ] In a wind's chat on a wide screen (at least 900 px), the wind's avatar (the active pack, `face`) sits at the bottom corner of the chat, partly outside the message column's frame, about 96 px; on narrow screens it is hidden. A node test that the element is built for a wind chat and not for a rotllana.
- [ ] Its mood follows the wind's state from 085: working moods while a turn runs (the existing `data-mood` keyframes in `winds.css`), a waiting pose when it has an ask, a short done animation when the turn ends. A node test of the state → mood mapping.
- [ ] It never covers the composer or a message: the message column keeps a margin on that side. Checked with screenshots at 900, 1280 and 1600 px.
- [ ] With `prefers-reduced-motion` it does not animate.

## Out of scope

- Android (088).
- New avatar art (094 adds volume).

## Notes

- Moods: `moodOf()` / `setMood()` in `winds.js` (194, 316), keyframes in `winds.css` 275-336.
- Web: node tests with `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; check in a sandboxed server in the browser pane, dark and light theme, with before/after screenshots in `output/ui-review/` (ignored).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
