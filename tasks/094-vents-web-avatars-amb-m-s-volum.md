---
title: Vents web: avatars amb més volum
status: done
depends_on: []
---

## Why

The user wants the wind avatars to look more three-dimensional. «Cares» already has volume (radial gradients, rim light, sheen, ground shadow); the other packs are flatter.

## Acceptance criteria

- [x] The «Esperit», «Cap de vent» and «Rosa de rellotge» packs (`esperit()`, `cap()`, `rosa()` in `winds.js`) get the same volume language as «Cares»: a radial gradient lit from the top left, a rim light, a specular highlight and a soft contact shadow under the large size. A node test that each pack's SVG has the gradient, highlight and shadow elements.
- [x] «Cares» gets a stronger depth (a second shading layer and a soft ambient occlusion where the face meets the body) without changing its shape. Before/after screenshots of every pack, small and large, dark and light, in `output/ui-review/`.
- [x] Every pack stays readable at the smallest size used in the lists (24-28 px): checked in the screenshots.
- [x] The mood animations still work (`winds.css` keyframes) and respect `prefers-reduced-motion`.

## Out of scope

- Android (095).
- New packs.
- The weather map pack (`mapa`, a canvas).

## Notes

- Files: `winds.js` (`face` 575, packs 367-556, Cares shading 597-618), `winds.css` 275-336. Test file: `wind-face.test.cjs`.
- Web: node tests with `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; check in a sandboxed server in the browser pane, dark and light theme, with before/after screenshots in `output/ui-review/` (ignored).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
- Done: `relief()` and `contact()` in `winds.js` give Esperit, Cap de vent and Rosa de rellotge a rim light (`wf-rim`), a specular highlight (`wf-spec`) and, at large size, a contact shadow (`wf-ground`); their existing top-left radial gradients stay. Cares gets `wf-shade` (a second, darker falloff away from the light) and `wf-ao` (occlusion at the base over the feet), both inside its clip, so its outline is unchanged. Rosa's highlight sits on top of the dial, like glass, with a rim in the dial's own dark tone. Every gradient id carries the face serial.
- Screenshots (ignored): `output/ui-review/094/before.png`, `after-1.png`, `after-2.png`, `after-3.png` (final): every pack at 24, 28 and 96 px on dark and light. `winds.css` is unchanged, so the mood keyframes and the reduced-motion rules are as before; the new layers live inside the groups that already move (Esperit's `wf-drift`, Cares' `wf-figure`).
- Not checked in a running server or the desktop app, only in a static page that loads `winds.js` and `winds.css`.
- Follow-up: on the dark theme the contact shadows are almost invisible (dark on dark), as Cares' ground already was.
