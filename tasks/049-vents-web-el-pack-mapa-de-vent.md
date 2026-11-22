---
title: "Vents (web): el pack Mapa de vent"
status: in-progress
depends_on: ["045"]
---

## Why

The most striking pack in `mockups/vents-avatars.html`: a live weather map inside a circle, with streaks of wind blowing in the wind's direction, faster while the wind works.

## Acceptance criteria

- [x] `mapa` is a valid value for the wind's `avatar` and for the theme. It appears in the preferences and in the editor's picker, and faces use it when chosen. A Go test for the value and a node test for the dispatch.
- [x] The streaks flow in the opposite direction of the wind's direction (045's function), and the same wind always gets the same picture. A node test on the simulation step, without a canvas: positions move along the expected angle.
- [x] Idle, the map is a still frame, unless expressive motion is on and `prefers-reduced-motion` is off. While working, it moves faster. Only faces on screen animate, through one shared animation loop. A node test checks that a still face does not register in the loop.
- [ ] With 30 faces on screen (sidebar, start page, list), a scroll keeps up without dropping frames on the desktop app. A recorded performance profile in the task notes is enough.

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

### Not verified

- Criterion 4 is not met, so `status` is `in-progress` (the code is on main) until someone records the profile or accepts closing without it: no performance profile was recorded on the desktop app. A rough check only: 30 small and 3 large map faces in a scratch page with the web's `winds.js`/`winds.css` in the browser pane rendered as expected (Tramuntana streaks go down the circle) and the script time of the rAF callbacks averaged about 0.5 ms per callback; that pane ran at ~30 fps as a background pane, so it says nothing about dropped frames. Someone should record a profile scrolling Vents with 30 faces in the desktop app.
- Not compared side by side with the mockup, and not seen inside the real Vents page, the sidebar, the start page or the pati scene (`face(w, 'figure')` draws the same plain disc, like Esperit).
- Rebased onto 047 and 051: `mapa` joins `cap` and `rosa` in `ValidAvatar`, the contract, the pack list, the theme choices and the face dispatch. The Preferences and editor picker tests in `winds.test.js` now list all five packs.
- Follow-up (review): a map face removed while off screen, or before it was ever seen, stays in the IntersectionObserver target list; and the `visibilitychange` restart and re-rendering are not covered by a test.
