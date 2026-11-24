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
- Done: `relief()` and `contact()` in `winds.js` light Esperit, Cap de vent and Rosa de rellotge over their own top-left gradients: a core shadow toward the bottom right (`wf-shade`), reflected light at that edge (`wf-rim`), a broad sheen (`wf-sheen`, dropped under 28 px) and a crisp specular highlight (`wf-spec`). At large size they stand on a soft shadow (`wf-ground`) over a bounce of their own colour (`wf-bounce`), so it still reads on a dark page. The large Rosa adds a curved reflection on its glass (`wf-glass`); its rim uses the dial's dark tone. The large Esperit and Cap get inset eyes or lit cheeks. Cares keeps its shape and gets a stronger core shadow (`wf-shade`), a stronger rim, a crisp highlight, occlusion over its feet (`wf-ao`) and, at large size, inset eyes and lit cheeks. Every gradient id carries the face serial.
- Screenshots (ignored), every pack at 24, 28 and 96 px on dark and light: `output/ui-review/094/before.png`, `after-1.png` to `after-5.png` (`after-5.png` is final), and `compare-4.png` and `compare-5.png` (before on the left, after on the right). `winds.css` is unchanged, so the mood keyframes and the reduced-motion rules are as before; the new layers live inside the groups that already move.
- Not checked in a running server or the desktop app, only in a static page that loads `winds.js` and `winds.css`.
- Follow-up: Cares' own ground shadow has no colour bounce yet, so on a dark page it fades more than the other packs' shadows.
