---
title: Vents web: Avui com un resum del dia, una targeta per vent
status: todo
depends_on: ["125", "126", "127"]
---

## Why

Avui should feel like a daily brief, like Duet's, not a set of empty boxes.

## Acceptance criteria

- [ ] Avui leads with one swipeable card per wind that did or proposes something today (what it did, what it proposes, a button to act), reusing the comparison pager; winds with nothing are skipped. A node test with two winds, one idle.
- [ ] Weather and match cards get small illustrated art (inline SVG in the wind's or theme colour, a few paths, no new image assets) instead of the empty smudge. Before/after screenshots (light, dark; 1280 and 390 px) in `output/ui-review/134/`.

## Out of scope

- Android (135).
- New backend data: use what /api/winds/avui and novetats return; if a field is missing, note it.

## Notes

- Files: vents-avui-glance.js, the comparison pager module, winds-workspace.css.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
