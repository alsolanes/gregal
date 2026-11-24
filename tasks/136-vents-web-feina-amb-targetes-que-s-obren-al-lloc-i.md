---
title: Vents web: Feina amb targetes que s'obren al lloc i una guia més lleugera
status: todo
depends_on: ["125", "126", "127"]
---

## Why

Feina's running items are plain rows, «El que han fet els vents» overlaps Feina › Fet, and the guide is slow and oversized.

## Acceptance criteria

- [ ] Running items in Feina are cards with the owner's face that expand in place to show steps and links (animated; Motion respects reduced motion (web: prefers-reduced-motion; Android: LocalWindMotion).); «El que han fet els vents» merges into Fet with no duplicate entry point. A node test.
- [ ] The «Què pot fer» guide is a sheet with 3 cards and «Més», cards staggered about 40 ms (about 250 ms total). A node test for the 3-card default. Before/after screenshots (light, dark; 1280 and 390 px) in `output/ui-review/136/`.

## Out of scope

- Android Feina (note gaps).

## Notes

- Files: winds-workspace.js (Feina, guide `.vw-dialog.vw-guide`), winds-workspace.css.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
