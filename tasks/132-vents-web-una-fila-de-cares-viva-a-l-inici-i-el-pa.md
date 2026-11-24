---
title: Vents web: una fila de cares viva a l'inici i el pati a primera fila
status: todo
depends_on: ["125", "126", "127"]
---

## Why

The home shows only some winds with no cue, and the fun (El pati, El Racó) is buried under settings.

## Acceptance criteria

- [ ] The home faces row scroll-snaps with a fading edge on mobile so every wind is reachable; faces breathe out of sync and show a dot when busy; tapping a face zooms it into the chat header (View Transitions where supported, plain navigation otherwise). Motion respects reduced motion (web: prefers-reduced-motion; Android: LocalWindMotion). A node test for the busy dot and that every wind is in the row.
- [ ] El pati is in the bottom bar and the sidebar (with El Racó folded into it), not under «Configuració i més»; the home has a highlighted tab in the bottom bar; Mestral is no longer cut at the pati's left edge. A node test. Before/after screenshots (light, dark; 1280 and 390 px) in `output/ui-review/132/`.

## Out of scope

- Android (133).

## Notes

- Files: winds-workspace.js (~132-138 nav), winds-workspace.css.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
