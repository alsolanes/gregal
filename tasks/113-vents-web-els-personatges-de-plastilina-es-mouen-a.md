---
title: Vents web: els personatges de plastilina es mouen a la web
status: done
depends_on: ["112"]
---

## Why

Play the clay characters on the web and desktop, with the code-drawn face as fallback.

## Acceptance criteria

- [x] A small player draws the right mood sheet on a canvas, tinting the body with the wind's colour through the mask, switching moods with a short crossfade; it is used wherever a wind face is large (corner/flying face, profile, start faces); tiny list avatars keep the SVG. A node test of mood switching and the fallback when a sheet is missing.
- [x] Celebration, nod/shake and the flight (101, 102, 108) still play on top. Reduced motion shows the first frame.
- [x] No new dependency.

## Out of scope

- Android (114).

## Notes

- Files: `winds.js` face entry points, a new `characters.js`.
- User: «l'estètica ha de ser millor del que tenim, dots és molt millor, grok bot també» and chose both steps: redesign in code first, then a Blender-rendered clay version (free, GPL; renders are ours to redistribute).
- Done 2026-11-24. `internal/web/app/characters.js` (`window.gregalCharacters`: `play(svg, w)`, `mood(el, mood)`, `tint`) takes the face winds.js drew and returns the clay canvas when a sheet is already loaded, or the svg, swapped in place once the sheet loads. The canvas copies the svg's classes (`winds-face …` plus `winds-clay`), `data-wind`, `data-mood` and aria attributes, so the corner CSS of 101/102/108 (`.vw-wind-corner > .winds-face`, `.vw-cheer`, `[data-react]`, `[data-mood]`) applies unchanged.
- Tint: the mask is drawn scaled up to the sheet, then each pixel is `clay * (1 - a + a * colour)` with `a` the mask alpha, sheet alpha kept (the README's multiply). Done once per silhouette, mood and colour into an offscreen canvas shared by every face; frames are then plain `drawImage`s.
- Crossfade 180 ms: old frame at `1 - t`, new one at `t` with `lighter`, so the figure never thins half way. A face drawn again in the same spot (the chat redraws its corner after a turn) starts from the sheet that spot showed last, so it crossfades instead of popping back to the svg. Reduced motion: frame 0, no crossfade, no animation loop.
- One shared `requestAnimationFrame` loop over the players on the page; an `IntersectionObserver` pauses faces off screen and the loop stops while `document.hidden`. A face that left the page is dropped at the next tick.
- Where: chat corner (`paintCorner` changes mood in place on the clay instead of redrawing), start faces, Today cards (34 px, asked for in the brief), the profile dialog head. List avatars (sidebar, recent rows, messages, crew roster rows) keep the svg. Trade-kit winds (`data-kit`) and the other avatar packs (`data-avatar`) stay drawn: the clay has no props or pack looks. `winds.js` news refresh now also moves clay faces between idle and news.
- Server: `/app/characters/*` was already embedded and served (WebP sniffed as `image/webp`); `.json` now goes out as `application/json`. `index.html` loads `characters.js` before `winds-workspace.js`. Tests: `internal/web/app/characters.test.cjs`, `winds-start.test.cjs` (two clay tests), `wind-face.test.cjs` (news on clay), `TestCharactersServedToTheWeb`.
- Visual check (headless Chrome, sandbox server on 18913, fake provider): `output/ui-review/113/` (ignored by git): start faces, chat corner idle frames, flight up/down, crossfade to working, nod, a turn that comes back done with the celebration, dark, light and dark reduced motion; `strip-*.png` put each sequence side by side.
- Not verified: high-DPI sharpness (the canvas is 128 px, shown at up to 96 CSS px, so 2x screens upscale it a little); desktop (Electron) and a real provider turn (the turn was faked at `/api/winds/chat/stream`); the profile and Today in the browser (unit tests only).
- Follow-up: under reduced motion `done` shows its first frame, which is the anticipation pose, not the happy arcs; a chosen still per mood would read better.
- Follow-up: `characters.js` repeats winds.js `color()` and the style-letter fallback; exporting them from `gregalWinds` would keep the two in step.
- Follow-up: players removed while off screen are only pruned on the next tick with a visible face; pruning in the observer callback would close that.
- Follow-up: the clay is drawn smaller than the svg in the same box (the sheet leaves room above for hops and the curl); 40 px start faces could use a little more size.
