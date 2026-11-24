---
title: Vents web: avatars amb més volum
status: todo
depends_on: []
---

## Why

The user wants the wind avatars to look more three-dimensional. «Cares» already has volume (radial gradients, rim light, sheen, ground shadow); the other packs are flatter.

## Acceptance criteria

- [ ] The «Esperit», «Cap de vent» and «Rosa de rellotge» packs (`esperit()`, `cap()`, `rosa()` in `winds.js`) get the same volume language as «Cares»: a radial gradient lit from the top left, a rim light, a specular highlight and a soft contact shadow under the large size. A node test that each pack's SVG has the gradient, highlight and shadow elements.
- [ ] «Cares» gets a stronger depth (a second shading layer and a soft ambient occlusion where the face meets the body) without changing its shape. Before/after screenshots of every pack, small and large, dark and light, in `output/ui-review/`.
- [ ] Every pack stays readable at the smallest size used in the lists (24-28 px): checked in the screenshots.
- [ ] The mood animations still work (`winds.css` keyframes) and respect `prefers-reduced-motion`.

## Out of scope

- Android (095).
- New packs.
- The weather map pack (`mapa`, a canvas).

## Notes

- Files: `winds.js` (`face` 575, packs 367-556, Cares shading 597-618), `winds.css` 275-336. Test file: `wind-face.test.cjs`.
- Web: node tests with `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; check in a sandboxed server in the browser pane, dark and light theme, with before/after screenshots in `output/ui-review/` (ignored).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
