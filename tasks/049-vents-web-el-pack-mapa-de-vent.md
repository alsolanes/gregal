---
title: "Vents (web): el pack Mapa de vent"
status: done
depends_on: ["045"]
---

## Why

The most striking pack in `mockups/vents-avatars.html`: a live weather map inside a circle, with streaks of wind blowing in the wind's direction, faster while the wind works.

## Acceptance criteria

- [x] `mapa` is a valid value for the wind's `avatar` and for the theme. It appears in the preferences and in the editor's picker, and faces use it when chosen. A Go test for the value and a node test for the dispatch.
- [x] The streaks flow in the opposite direction of the wind's direction (045's function), and the same wind always gets the same picture. A node test on the simulation step, without a canvas: positions move along the expected angle.
- [x] Idle, the map is a still frame, unless expressive motion is on and `prefers-reduced-motion` is off. While working, it moves faster. Only faces on screen animate, through one shared animation loop. A node test checks that a still face does not register in the loop.
- [x] With 30 faces on screen (sidebar, start page, list), a scroll keeps up without dropping frames on the desktop app. A recorded performance profile in the task notes is enough.

## Out of scope

- Android: 050.
- Two flows mixing in one circle for rotllanes (they keep the stack).

## Notes

- Reference: `mapa()` and `step()` in `mockups/vents-avatars.html` (canvas, `lighter` compositing, fade per frame). Use a `canvas` sized ×2 for sharpness, and pause the loop when the tab is hidden.
- Update `docs/api-contract.md` with the new value.

### What was built (2026-11-22)

- Server: `companion.ValidAvatar` accepts `mapa`, so it is valid for a wind's `avatar` and for the theme. Contract updated (`/api/winds` GET and POST, `/api/winds/avatar-theme`).
- Web: `face()` dispatches `mapa` to `mapa()` in `winds.js`. The face stays an `svg` (so `setMood`, badges and every caller keep working): a `foreignObject` holds a `canvas.wf-map` (rounded in `winds.css`), with the mockup's gloss and rim drawn on top in SVG. No trade kit, like Esperit. Picker strings: «Mapa de vent» / «Wind map» (`winds.avatarMapa`).
- Simulation: `mapStart(w)` and `mapStep(st, busy)` (exported on `gregalWinds`) are pure. The particles are seeded from the wind's id (an unsaved wind in the editor preview uses the id its name will get, the same `windId` rule `direction()` uses) with a small PRNG instead of `Math.random`, so the same wind always gets the same picture. 90 particles whatever the size. The flow angle is `direction(w) + 180°`, with the mockup's ±0.6 rad swirl; working steps 0.013 instead of 0.0055 and fades faster.
- Loop: one `requestAnimationFrame` for every map (`mapRun`/`mapTick`), one shared `IntersectionObserver`. `setMood` marks a map as moving only when it works or when motion is allowed (no `prefers-reduced-motion`; the web has no «Moviment expressiu» preference, see 045). The loop paints the moving maps among those the observer has on screen (a face removed from the page leaves that set and is unobserved), and it stops when nothing on screen moves or `document.hidden` (restarts on `visibilitychange`). The canvas is sized ×2 its on-screen width on first sight (and restarts the same picture if the size changes); the still frame is 50 silent steps plus 20 painted, done only when the face first appears on screen.
- Tests: Go `TestMapaIsAnAvatarPack` (`internal/web/winds_avatar_test.go`); node `wind-face.test.cjs` (dispatch, the step's angle and speed and determinism, the loop), `winds.test.js` (Preferences row and editor picker list «Mapa de vent»).

### Performance (2026-11-22)

A scratch page with `winds.js`/`winds.css` from main and 75 map faces (12 in a fixed sidebar, 3 large, 60 in a list; every fourth working, the rest drifting, so all on screen move): 28 on screen at once. Playwright drove a windowed Chrome 1280×800, scrolling the whole page top to bottom and back over 6 s, and timed every `requestAnimationFrame` gap. Two passes per run (the first draws each map's still frame as it appears).

| Run | fps | p50 | p95 | p99 | max | frames > 20 ms |
| --- | --- | --- | --- | --- | --- | --- |
| Mapa, first pass | 116 | 7.6 ms | 15.5 ms | 16.0 ms | 16.5 ms | 0 |
| Mapa, second pass | 120 | 7.6 ms | 15.1 ms | 15.7 ms | 16.1 ms | 0 |
| Cares (same page) | 112–115 | 7.6 ms | 15.5 ms | 16.0 ms | 16.7 ms | 0 |

No long tasks and no frame over 20 ms, the same as Cares. A jump instead of a scroll (an anchor 1,300 px down revealing ~14 new maps at once) does drop a frame (~55 ms): each map's first picture is about 10 ms of canvas work (20 painted steps), done in the IntersectionObserver callback. Spreading them over tasks brought that to ~38 ms but cost fps on normal scrolls, so it was left out.

### Not verified

- Not measured in the desktop app itself, but in Chrome (the same Chromium engine as Electron), on a 120 Hz screen.
- Not compared side by side with the mockup, and not seen inside the real Vents page, the sidebar, the start page or the pati scene (`face(w, 'figure')` draws the same plain disc, like Esperit).
- Rebased onto 047 and 051: `mapa` joins `cap` and `rosa` in `ValidAvatar`, the contract, the pack list, the theme choices and the face dispatch. The Preferences and editor picker tests in `winds.test.js` now list all five packs.
- Follow-up (review): a map face removed while off screen, or before it was ever seen, stays in the IntersectionObserver target list; and the `visibilitychange` restart and re-rendering are not covered by a test.
