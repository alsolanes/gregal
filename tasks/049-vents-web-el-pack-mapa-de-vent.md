---
title: "Vents (web): el pack Mapa de vent"
status: todo
depends_on: ["045"]
---

## Why

The most striking pack in `mockups/vents-avatars.html`: a live weather map inside a circle, with streaks of wind blowing in the wind's direction, faster while the wind works.

## Acceptance criteria

- [ ] `mapa` is a valid value for the wind's `avatar` and for the theme. It appears in the preferences and in the editor's picker, and faces use it when chosen. A Go test for the value and a node test for the dispatch.
- [ ] The streaks flow in the opposite direction of the wind's direction (045's function), and the same wind always gets the same picture. A node test on the simulation step, without a canvas: positions move along the expected angle.
- [ ] Idle, the map is a still frame, unless expressive motion is on and `prefers-reduced-motion` is off. While working, it moves faster. Only faces on screen animate, through one shared animation loop. A node test checks that a still face does not register in the loop.
- [ ] With 30 faces on screen (sidebar, start page, list), a scroll keeps up without dropping frames on the desktop app. A recorded performance profile in the task notes is enough.

## Out of scope

- Android: 050.
- Two flows mixing in one circle for rotllanes (they keep the stack).

## Notes

- Reference: `mapa()` and `step()` in `mockups/vents-avatars.html` (canvas, `lighter` compositing, fade per frame). Use a `canvas` sized ×2 for sharpness, and pause the loop when the tab is hidden.
- Update `docs/api-contract.md` with the new value.
